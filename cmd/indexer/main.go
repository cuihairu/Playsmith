package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	chaincodeName = "playsmith-assets"
	schema        = `
CREATE TABLE IF NOT EXISTS chaincode_events (
    block_number BIGINT NOT NULL,
    transaction_id TEXT NOT NULL,
    event_name TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (block_number, transaction_id, event_name)
);
CREATE TABLE IF NOT EXISTS balances (
    token_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (token_id, owner)
);
CREATE TABLE IF NOT EXISTS listings (
    listing_id TEXT PRIMARY KEY,
    token_id TEXT NOT NULL,
    seller TEXT NOT NULL,
    amount BIGINT NOT NULL,
    price BIGINT NOT NULL DEFAULT 0,
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
    amount BIGINT NOT NULL,
    price BIGINT NOT NULL DEFAULT 0,
    price_hash TEXT NOT NULL DEFAULT '',
    block_number BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_listings_token_seller ON listings(token_id, seller, status);
CREATE INDEX IF NOT EXISTS idx_trades_token ON trades(token_id, created_at DESC);
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

// HandleEvent processes one chaincode event in a single transaction.
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
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal %s payload: %w", ev.EventName, err)
	}
	// record the raw event (dedupe by block/txid/event)
	if _, err := idx.exec.Exec(ctx, `INSERT INTO chaincode_events(block_number, transaction_id, event_name, payload)
VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
		int64(ev.BlockNumber), ev.TransactionID, ev.EventName, ev.Payload); err != nil {
		return err
	}

	switch ev.EventName {
	case "TransferSingle":
		return idx.handleTransfer(ctx, payload)
	case "TransferBatch":
		return idx.handleTransferBatch(ctx, payload)
	case "AskCreated":
		return idx.handleAskCreated(ctx, payload)
	case "AskCanceled":
		return idx.handleAskCanceled(ctx, payload)
	case "Trade":
		return idx.handleTrade(ctx, payload, ev.BlockNumber)
	default:
		return nil
	}
}

func (idx *Indexer) handleTransfer(ctx context.Context, p map[string]any) error {
	tokenID, _ := p["id"].(string)
	from, _ := p["from"].(string)
	to, _ := p["to"].(string)
	value, _ := p["value"].(float64)
	if tokenID == "" || to == "" || value <= 0 {
		return errors.New("invalid TransferSingle payload")
	}
	if from != "" && from != "0x0" {
		if _, err := idx.exec.Exec(ctx, `UPDATE balances SET balance = balance - $3, updated_at = now() WHERE token_id=$1 AND owner=$2`, tokenID, from, int64(value)); err != nil {
			return err
		}
	}
	_, err := idx.exec.Exec(ctx, `INSERT INTO balances(token_id, owner, balance) VALUES($1,$2,$3)
ON CONFLICT(token_id,owner) DO UPDATE SET balance = balances.balance + EXCLUDED.balance`, tokenID, to, int64(value))
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
	to, _ := p["to"].(string)
	if from == "0x0" {
		from = ""
	}
	for i := range idsAny {
		id, _ := idsAny[i].(string)
		val, _ := valuesAny[i].(float64)
		sub := map[string]any{"id": id, "from": from, "to": to, "value": val}
		if err := idx.handleTransfer(ctx, sub); err != nil {
			return err
		}
	}
	return nil
}

func (idx *Indexer) handleAskCreated(ctx context.Context, p map[string]any) error {
	tokenID, _ := p["id"].(string)
	seller, _ := p["seller"].(string)
	listingID, _ := p["listingId"].(string)
	amount, _ := p["amount"].(float64)
	price, _ := p["price"].(float64)
	priceHash, _ := p["priceHash"].(string)
	if tokenID == "" || seller == "" || listingID == "" || amount <= 0 {
		return errors.New("invalid AskCreated payload")
	}
	_, err := idx.exec.Exec(ctx, `INSERT INTO listings(listing_id, token_id, seller, amount, price, price_hash, status)
VALUES($1,$2,$3,$4,$5,$6,'active')
ON CONFLICT(listing_id) DO UPDATE SET amount=EXCLUDED.amount, price=EXCLUDED.price, price_hash=EXCLUDED.price_hash, status='active', updated_at=now()`,
		listingID, tokenID, seller, int64(amount), int64(price), priceHash)
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
	amount, _ := p["amount"].(float64)
	price, _ := p["price"].(float64)
	priceHash, _ := p["priceHash"].(string)
	if tokenID == "" || seller == "" || buyer == "" || listingID == "" || amount <= 0 {
		return errors.New("invalid Trade payload")
	}
	if _, err := idx.exec.Exec(ctx, `UPDATE balances SET balance = balance - $3, updated_at=now() WHERE token_id=$1 AND owner=$2`, tokenID, seller, int64(amount)); err != nil {
		return err
	}
	if _, err := idx.exec.Exec(ctx, `INSERT INTO balances(token_id, owner, balance) VALUES($1,$2,$3)
ON CONFLICT(token_id,owner) DO UPDATE SET balance = balances.balance + EXCLUDED.balance`, tokenID, buyer, int64(amount)); err != nil {
		return err
	}
	if _, err := idx.exec.Exec(ctx, `INSERT INTO trades(trade_id, token_id, seller, buyer, listing_id, amount, price, price_hash, block_number)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(trade_id) DO NOTHING`,
		listingID, tokenID, seller, buyer, listingID, int64(amount), int64(price), priceHash, int64(blockNumber)); err != nil {
		return err
	}
	_, err := idx.exec.Exec(ctx, `UPDATE listings SET status='sold', updated_at=now() WHERE listing_id=$1`, listingID)
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

	gw, events, err := connectFabric(ctx)
	if err != nil {
		log.Fatalf("connect Fabric: %v", err)
	}
	defer gw.Close()

	idx := NewIndexer(conn)
	log.Printf("Indexer started, subscribed to chaincode %s", chaincodeName)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := idx.HandleEvent(ctx, ev); err != nil {
				log.Printf("event %s block %d tx %s: %v", ev.EventName, ev.BlockNumber, ev.TransactionID, err)
			} else {
				log.Printf("indexed %s block=%d tx=%s", ev.EventName, ev.BlockNumber, ev.TransactionID)
			}
		}
	}
}

func connectFabric(ctx context.Context) (*client.Gateway, <-chan *client.ChaincodeEvent, error) {
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
	network := gw.GetNetwork(channel)
	events, err := network.ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		_ = gw.Close()
		return nil, nil, err
	}
	return gw, events, nil
}
