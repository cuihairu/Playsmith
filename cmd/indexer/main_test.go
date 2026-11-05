package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/jackc/pgx/v5/pgconn"
)

type rec struct {
	sql  string
	args []any
}
type mockExec struct{ recs []rec }

func (m *mockExec) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.recs = append(m.recs, rec{sql, args})
	return pgconn.CommandTag{}, nil
}

func newIdx() (*Indexer, *mockExec) {
	m := &mockExec{}
	return NewIndexerWithExec(m), m
}

func ev(name string, payload map[string]any) *client.ChaincodeEvent {
	b, _ := json.Marshal(payload)
	return &client.ChaincodeEvent{EventName: name, Payload: b, BlockNumber: 5, TransactionID: "tx1"}
}

func TestTransferSingleMintUpsert(t *testing.T) {
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("TransferSingle", map[string]any{"id": "t1", "from": "0x0", "to": "alice", "value": 10.0})); err != nil {
		t.Fatal(err)
	}
	// chaincode_events + balances insert
	if len(m.recs) != 2 {
		t.Fatalf("expected 2 execs, got %d", len(m.recs))
	}
	if m.recs[1].args[1] != "alice" || m.recs[1].args[2] != int64(10) {
		t.Fatalf("bad balance args: %v", m.recs[1].args)
	}
}

func TestTransferSingleDebitsFrom(t *testing.T) {
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("TransferSingle", map[string]any{"id": "t1", "from": "bob", "to": "alice", "value": 4.0})); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 3 {
		t.Fatalf("expected 3 execs (event + debit + credit), got %d", len(m.recs))
	}
	if m.recs[1].args[1] != "bob" || m.recs[1].args[2] != int64(4) {
		t.Fatalf("bad debit args: %v", m.recs[1].args)
	}
}

func TestAskCreatedUpsert(t *testing.T) {
	idx, m := newIdx()
	p := map[string]any{"id": "t1", "seller": "s", "listingId": "L1", "amount": 3.0, "price": 100.0, "priceHash": "abc"}
	if err := idx.HandleEvent(context.Background(), ev("AskCreated", p)); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 2 {
		t.Fatalf("expected 2 execs")
	}
	if m.recs[1].args[0] != "L1" || m.recs[1].args[4] != int64(100) {
		t.Fatalf("bad args: %v", m.recs[1].args)
	}
}

func TestTradeUpdatesBalancesAndListing(t *testing.T) {
	idx, m := newIdx()
	p := map[string]any{"id": "t1", "seller": "s", "buyer": "b", "listingId": "L1", "amount": 2.0, "price": 7.0, "priceHash": ""}
	if err := idx.HandleEvent(context.Background(), ev("Trade", p)); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 5 {
		t.Fatalf("expected 5 execs (event + debit + credit + trade + sold), got %d", len(m.recs))
	}
	if m.recs[4].args[0] != "L1" {
		t.Fatalf("listing sold arg = %v", m.recs[4].args)
	}
}

func TestUnknownEventOnlyLogged(t *testing.T) {
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("URI", map[string]any{"id": "t1", "uri": "u"})); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 1 {
		t.Fatalf("expected 1 exec (raw event only), got %d", len(m.recs))
	}
}

func TestTransferBatch(t *testing.T) {
	idx, m := newIdx()
	p := map[string]any{
		"ids":    []any{"t1", "t2"},
		"from":   "0x0",
		"to":     "batch",
		"values": []any{1.0, 2.0},
	}
	if err := idx.HandleEvent(context.Background(), ev("TransferBatch", p)); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 3 {
		t.Fatalf("expected 3 execs (event + 2 credit), got %d", len(m.recs))
	}
}
