package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	chaincodeName    = "playsmith-assets"
	stateKey         = "playsmith-events"
	retryMax         = 3
	resubscribeDelay = 5 * time.Second

	schema = `
CREATE TABLE IF NOT EXISTS chaincode_events (
    block_number NUMERIC NOT NULL,
    transaction_id TEXT NOT NULL,
    event_name TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (block_number, transaction_id, event_name)
);
CREATE TABLE IF NOT EXISTS balances (
    token_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    balance NUMERIC NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (token_id, owner)
);
CREATE TABLE IF NOT EXISTS listings (
    listing_id TEXT PRIMARY KEY,
    token_id TEXT NOT NULL,
    seller TEXT NOT NULL,
    amount NUMERIC NOT NULL DEFAULT 0,
    price NUMERIC NOT NULL DEFAULT 0,
    price_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS trades (
    trade_id TEXT PRIMARY KEY,
    token_id TEXT NOT NULL,
    seller TEXT NOT NULL,
    buyer TEXT NOT NULL,
    listing_id TEXT NOT NULL,
    amount NUMERIC NOT NULL DEFAULT 0,
    price NUMERIC NOT NULL DEFAULT 0,
    price_hash TEXT NOT NULL DEFAULT '',
    block_number NUMERIC NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS indexer_state (
    key TEXT PRIMARY KEY,
    last_block NUMERIC NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_listings_token_seller ON listings(token_id, seller, status);
CREATE INDEX IF NOT EXISTS idx_trades_token ON trades(token_id, created_at DESC);
-- startup migration for databases created before these changes. Every
-- statement is safe to re-run.
ALTER TABLE balances ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE balances ALTER COLUMN balance TYPE NUMERIC;
ALTER TABLE listings ALTER COLUMN amount TYPE NUMERIC;
ALTER TABLE listings ALTER COLUMN price TYPE NUMERIC;
ALTER TABLE trades ALTER COLUMN amount TYPE NUMERIC;
ALTER TABLE trades ALTER COLUMN price TYPE NUMERIC;
ALTER TABLE chaincode_events ALTER COLUMN block_number TYPE NUMERIC;
ALTER TABLE trades ALTER COLUMN block_number TYPE NUMERIC;
`
)

// execer abstracts a single SQL execution. Real use wraps pgx.Tx; tests inject a recorder.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type pgxExecer struct{ tx pgx.Tx }

func (e pgxExecer) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return e.tx.Exec(ctx, sql, arguments...)
}

// txer abstracts transaction begin for the real DB; tests may inject a no-op begin.
type txer interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type pgxTxer struct{ conn *pgx.Conn }

func (t pgxTxer) Begin(ctx context.Context) (pgx.Tx, error) { return t.conn.Begin(ctx) }

// Indexer consumes Fabric chaincode events and upserts Postgres state.
type Indexer struct {
	begin txer
	exec  execer
}

func NewIndexer(conn *pgx.Conn) *Indexer {
	return &Indexer{begin: pgxTxer{conn: conn}}
}

func NewIndexerWithExec(exec execer) *Indexer {
	return &Indexer{exec: exec}
}

// HandleEvent processes one chaincode event in a single transaction. The raw
// event and the checkpoint advance with it, so a failure rolls both back.
func (idx *Indexer) HandleEvent(ctx context.Context, ev *client.ChaincodeEvent) error {
	if ev == nil {
		return nil
	}
	if idx.begin == nil {
		// test path: exec already injected
		return idx.route(ctx, ev)
	}
	tx, err := idx.begin.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	idx.exec = pgxExecer{tx: tx}
	if err := idx.route(ctx, ev); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (idx *Indexer) route(ctx context.Context, ev *client.ChaincodeEvent) error {
	payload, err := decodePayload(ev.Payload)
	if err != nil {
		return err
	}
	// record the raw event (dedupe by block/txid/event). Only the first insert
	// may project: a replayed event would otherwise add its balance deltas a
	// second time to a row that ON CONFLICT refused to rewrite.
	tag, err := idx.exec.Exec(ctx, `INSERT INTO chaincode_events(block_number, transaction_id, event_name, payload)
VALUES($1::numeric,$2,$3,$4) ON CONFLICT DO NOTHING`,
		strconv.FormatUint(ev.BlockNumber, 10), ev.TransactionID, ev.EventName, ev.Payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	var err2 error
	switch ev.EventName {
	case "TransferSingle":
		err2 = idx.handleTransfer(ctx, payload)
	case "TransferBatch":
		err2 = idx.handleTransferBatch(ctx, payload)
	case "AskCreated":
		err2 = idx.handleAskCreated(ctx, payload)
	case "AskCanceled":
		err2 = idx.handleAskCanceled(ctx, payload)
	case "Trade":
		err2 = idx.handleTrade(ctx, payload, ev.BlockNumber)
	}
	if err2 != nil {
		return err2
	}

	// remember how far the projection has caught up; a restart resumes here
	if _, err := idx.exec.Exec(ctx, `INSERT INTO indexer_state(key, last_block) VALUES($1,$2::numeric)
ON CONFLICT(key) DO UPDATE SET last_block = GREATEST(indexer_state.last_block, EXCLUDED.last_block)`,
		stateKey, strconv.FormatUint(ev.BlockNumber, 10)); err != nil {
		return err
	}
	return nil
}

// decodePayload decodes into json.Number so amounts keep every decimal digit.
// json.Unmarshal into map[string]any turns numbers into float64, which cannot
// hold uint64 exactly: 9007199254740993 becomes 9007199254740992.
func decodePayload(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var payload map[string]any
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return payload, nil
}

// uintField reads an unsigned quantity exactly, rejecting anything that does
// not name a whole uint64.
func uintField(p map[string]any, key, what string) (uint64, error) {
	n, ok := p[key].(json.Number)
	if !ok {
		return 0, fmt.Errorf("invalid %s %s", what, key)
	}
	v, err := strconv.ParseUint(n.String(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %s: %v", what, key, err)
	}
	return v, nil
}

func numArg(v uint64) string { return strconv.FormatUint(v, 10) }

func (idx *Indexer) handleTransfer(ctx context.Context, p map[string]any) error {
	tokenID, _ := p["id"].(string)
	from, _ := p["from"].(string)
	to, _ := p["to"].(string)
	value, err := uintField(p, "value", "TransferSingle")
	if err != nil {
		return err
	}
	if tokenID == "" || to == "" || value == 0 {
		return errors.New("invalid TransferSingle payload")
	}
	if from != "" && from != "0x0" {
		if _, err := idx.exec.Exec(ctx, `UPDATE balances SET balance = balance - $3::numeric, updated_at = now() WHERE token_id=$1 AND owner=$2`, tokenID, from, numArg(value)); err != nil {
			return err
		}
	}
	_, err = idx.exec.Exec(ctx, `INSERT INTO balances(token_id, owner, balance) VALUES($1,$2,$3::numeric)
ON CONFLICT(token_id,owner) DO UPDATE SET balance = balances.balance + EXCLUDED.balance, updated_at = now()`, tokenID, to, numArg(value))
	return err
}

func (idx *Indexer) handleTransferBatch(ctx context.Context, p map[string]any) error {
	idsAny, ok := p["ids"].([]any)
	if !ok {
		return errors.New("invalid TransferBatch ids")
	}
	valuesAny, ok := p["values"].([]any)
	if !ok || len(idsAny) != len(valuesAny) {
		return errors.New("invalid TransferBatch values")
	}
	from, _ := p["from"].(string)
	if from == "0x0" {
		from = ""
	}
	// BatchMint addresses many recipients, so the event carries a tos array
	// parallel to ids; a single "to" field is a plain batch transfer.
	tosAny, _ := p["tos"].([]any)
	to, _ := p["to"].(string)
	for i := range idsAny {
		id, _ := idsAny[i].(string)
		val, err := uintField(map[string]any{"value": valuesAny[i]}, "value", "TransferBatch")
		if err != nil {
			return err
		}
		dest := to
		if i < len(tosAny) {
			dest, _ = tosAny[i].(string)
		}
		if id == "" || dest == "" {
			return errors.New("invalid TransferBatch recipient")
		}
		if err := idx.handleTransfer(ctx, map[string]any{"id": id, "from": from, "to": dest, "value": json.Number(numArg(val))}); err != nil {
			return err
		}
	}
	return nil
}

func (idx *Indexer) handleAskCreated(ctx context.Context, p map[string]any) error {
	tokenID, _ := p["id"].(string)
	seller, _ := p["seller"].(string)
	listingID, _ := p["listingId"].(string)
	amount, err := uintField(p, "amount", "AskCreated")
	if err != nil {
		return err
	}
	var price uint64
	if _, ok := p["price"]; ok {
		if price, err = uintField(p, "price", "AskCreated"); err != nil {
			return err
		}
	}
	priceHash, _ := p["priceHash"].(string)
	if tokenID == "" || seller == "" || listingID == "" || amount == 0 {
		return errors.New("invalid AskCreated payload")
	}
	_, err = idx.exec.Exec(ctx, `INSERT INTO listings(listing_id, token_id, seller, amount, price, price_hash, status)
VALUES($1,$2,$3,$4::numeric,$5::numeric,$6,'active')
ON CONFLICT(listing_id) DO UPDATE SET amount=EXCLUDED.amount, price=EXCLUDED.price, price_hash=EXCLUDED.price_hash, status='active', updated_at=now()`,
		listingID, tokenID, seller, numArg(amount), numArg(price), priceHash)
	return err
}

func (idx *Indexer) handleAskCanceled(ctx context.Context, p map[string]any) error {
	listingID, _ := p["listingId"].(string)
	if listingID == "" {
		return errors.New("invalid AskCanceled payload")
	}
	_, err := idx.exec.Exec(ctx, `UPDATE listings SET status='canceled', updated_at=now() WHERE listing_id=$1`, listingID)
	return err
}

func (idx *Indexer) handleTrade(ctx context.Context, p map[string]any, blockNumber uint64) error {
	tokenID, _ := p["id"].(string)
	seller, _ := p["seller"].(string)
	buyer, _ := p["buyer"].(string)
	listingID, _ := p["listingId"].(string)
	amount, err := uintField(p, "amount", "Trade")
	if err != nil {
		return err
	}
	var price uint64
	if _, ok := p["price"]; ok {
		if price, err = uintField(p, "price", "Trade"); err != nil {
			return err
		}
	}
	priceHash, _ := p["priceHash"].(string)
	if tokenID == "" || seller == "" || buyer == "" || listingID == "" || amount == 0 {
		return errors.New("invalid Trade payload")
	}
	if _, err := idx.exec.Exec(ctx, `UPDATE balances SET balance = balance - $3::numeric, updated_at=now() WHERE token_id=$1 AND owner=$2`, tokenID, seller, numArg(amount)); err != nil {
		return err
	}
	if _, err := idx.exec.Exec(ctx, `INSERT INTO balances(token_id, owner, balance) VALUES($1,$2,$3::numeric)
ON CONFLICT(token_id,owner) DO UPDATE SET balance = balances.balance + EXCLUDED.balance, updated_at = now()`, tokenID, buyer, numArg(amount)); err != nil {
		return err
	}
	if _, err := idx.exec.Exec(ctx, `INSERT INTO trades(trade_id, token_id, seller, buyer, listing_id, amount, price, price_hash, block_number)
VALUES($1,$2,$3,$4,$5,$6::numeric,$7::numeric,$8,$9::numeric) ON CONFLICT(trade_id) DO NOTHING`,
		listingID, tokenID, seller, buyer, listingID, numArg(amount), numArg(price), priceHash, strconv.FormatUint(blockNumber, 10)); err != nil {
		return err
	}
	_, err = idx.exec.Exec(ctx, `UPDATE listings SET status='sold', updated_at=now() WHERE listing_id=$1`, listingID)
	return err
}

// querier can read one row; main passes the connection, tests pass a stub.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// resumeBlock reports how far an earlier run committed, so a restart picks up
// where it stopped instead of silently dropping every transaction it was down
// for.
func resumeBlock(ctx context.Context, q querier) (uint64, error) {
	var block uint64
	if err := q.QueryRow(ctx, `SELECT last_block FROM indexer_state WHERE key=$1`, stateKey).Scan(&block); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("read checkpoint: %w", err)
	}
	return block, nil
}

// retry repeats a transient failure instead of dropping the event.
func retry(f func() error) error {
	var err error
	for attempt := 0; attempt < retryMax; attempt++ {
		if err = f(); err == nil {
			return nil
		}
	}
	return err
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pgDSN := os.Getenv("POSTGRES_DSN")
	if pgDSN == "" {
		log.Fatal("POSTGRES_DSN is required")
	}
	conn, err := pgx.Connect(ctx, pgDSN)
	if err != nil {
		log.Fatalf("connect Postgres: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, schema); err != nil {
		log.Fatalf("create schema: %v", err)
	}

	gw, network, err := connectFabric(ctx)
	if err != nil {
		log.Fatalf("connect Fabric: %v", err)
	}
	defer gw.Close()

	idx := NewIndexer(conn)
	from, err := resumeBlock(ctx, conn)
	if err != nil {
		log.Fatalf("checkpoint: %v", err)
	}
	log.Printf("Indexer started, subscribed to chaincode %s from block %d", chaincodeName, from)
	events, err := network.ChaincodeEvents(ctx, chaincodeName, client.WithStartBlock(from))
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	// committed is the last block whose projection is durable. It is the resume
	// point if the stream closes: the events table rejects a duplicate
	// (block, tx, event) row, so replaying an already committed event is a
	// no-op instead of double counting it.
	committed := from
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				log.Printf("event stream closed; restarting from block %d", committed)
				select {
				case <-ctx.Done():
					return
				case <-time.After(resubscribeDelay):
				}
				ev2, err2 := network.ChaincodeEvents(ctx, chaincodeName, client.WithStartBlock(committed))
				if err2 != nil {
					log.Printf("resubscribe: %v", err2)
					select {
					case <-ctx.Done():
						return
					case <-time.After(resubscribeDelay):
					}
					continue
				}
				events = ev2
			}
			if err := retry(func() error { return idx.HandleEvent(ctx, ev) }); err != nil {
				log.Printf("dropping %s block=%d tx=%s after %d attempts: %v", ev.EventName, ev.BlockNumber, ev.TransactionID, retryMax, err)
				continue
			}
			if ev.BlockNumber >= committed {
				committed = ev.BlockNumber
			}
			log.Printf("indexed %s block=%d tx=%s", ev.EventName, ev.BlockNumber, ev.TransactionID)
		}
	}
}

func connectFabric(ctx context.Context) (*client.Gateway, *client.Network, error) {
	endpoint := os.Getenv("FABRIC_GATEWAY_ENDPOINT")
	certPath := os.Getenv("FABRIC_CERT_PATH")
	keyPath := os.Getenv("FABRIC_KEY_PATH")
	tlsPath := os.Getenv("FABRIC_TLS_PATH")
	channel := os.Getenv("FABRIC_CHANNEL_NAME")
	mspID := os.Getenv("FABRIC_MSP_ID")
	if endpoint == "" || certPath == "" || keyPath == "" || tlsPath == "" || channel == "" || mspID == "" {
		return nil, nil, errors.New("FABRIC_GATEWAY_ENDPOINT, FABRIC_CERT_PATH, FABRIC_KEY_PATH, FABRIC_TLS_PATH, FABRIC_CHANNEL_NAME, FABRIC_MSP_ID are required")
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	tlsPEM, err := os.ReadFile(tlsPath)
	if err != nil {
		return nil, nil, err
	}
	cert, err := identity.CertificateFromPEM(certPEM)
	if err != nil {
		return nil, nil, err
	}
	key, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, nil, err
	}
	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		return nil, nil, err
	}
	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		return nil, nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(tlsPEM) {
		return nil, nil, errors.New("invalid TLS certificate")
	}
	grpcConn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(pool, "")))
	if err != nil {
		return nil, nil, err
	}
	gw, err := client.Connect(id, client.WithSign(sign), client.WithClientConnection(grpcConn))
	if err != nil {
		return nil, nil, err
	}
	return gw, gw.GetNetwork(channel), nil
}
