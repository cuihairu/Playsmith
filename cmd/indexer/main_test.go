package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
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
	return pgconn.NewCommandTag("INSERT 0 1"), nil
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
	if len(m.recs) != 3 {
		t.Fatalf("expected 3 execs (event + credit + checkpoint), got %d", len(m.recs))
	}
	if m.recs[1].args[1] != "alice" || m.recs[1].args[2] != "10" {
		t.Fatalf("bad balance args: %v", m.recs[1].args)
	}
}

func TestTransferSingleDebitsFrom(t *testing.T) {
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("TransferSingle", map[string]any{"id": "t1", "from": "bob", "to": "alice", "value": 4.0})); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 4 {
		t.Fatalf("expected 4 execs (event + debit + credit + checkpoint), got %d", len(m.recs))
	}
	if m.recs[1].args[1] != "bob" || m.recs[1].args[2] != "4" {
		t.Fatalf("bad debit args: %v", m.recs[1].args)
	}
}

func TestAskCreatedUpsert(t *testing.T) {
	idx, m := newIdx()
	p := map[string]any{"id": "t1", "seller": "s", "listingId": "L1", "amount": 3.0, "price": 100.0, "priceHash": "abc"}
	if err := idx.HandleEvent(context.Background(), ev("AskCreated", p)); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 3 {
		t.Fatalf("expected 3 execs, got %d", len(m.recs))
	}
	if m.recs[1].args[0] != "L1" || m.recs[1].args[4] != "100" {
		t.Fatalf("bad args: %v", m.recs[1].args)
	}
}

func TestTradeUpdatesBalancesAndListing(t *testing.T) {
	idx, m := newIdx()
	p := map[string]any{"id": "t1", "seller": "s", "buyer": "b", "listingId": "L1", "amount": 2.0, "price": 7.0, "priceHash": ""}
	if err := idx.HandleEvent(context.Background(), ev("Trade", p)); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 6 {
		t.Fatalf("expected 6 execs (event + debit + credit + trade + sold + checkpoint), got %d", len(m.recs))
	}
	if m.recs[4].args[0] != "L1" {
		t.Fatalf("listing sold arg = %v", m.recs[5].args)
	}
}

func TestUnknownEventOnlyLogged(t *testing.T) {
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("URI", map[string]any{"id": "t1", "uri": "u"})); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 2 {
		t.Fatalf("expected 2 execs (raw event + checkpoint), got %d", len(m.recs))
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
	if len(m.recs) != 4 {
		t.Fatalf("expected 4 execs (event + 2 credits + checkpoint), got %d", len(m.recs))
	}
}

// duplicateExec records the raw-event insert as having matched the primary key
// (RowsAffected == 0), i.e. this event was already projected.
type duplicateExec struct{ recs []rec }

func (m *duplicateExec) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.recs = append(m.recs, rec{sql, args})
	return pgconn.NewCommandTag("INSERT 0 0"), nil
}

func TestDuplicateEventIsNotProjectedTwice(t *testing.T) {
	m := &duplicateExec{}
	idx := NewIndexerWithExec(m)
	if err := idx.HandleEvent(context.Background(), ev("TransferSingle", map[string]any{"id": "t1", "from": "bob", "to": "alice", "value": 4})); err != nil {
		t.Fatal(err)
	}
	if len(m.recs) != 1 {
		t.Fatalf("duplicate event ran %d execs, want raw insert only", len(m.recs))
	}
}

func TestLargeAmountKeepsEveryDigit(t *testing.T) {
	idx, m := newIdx()
	raw := []byte(`{"id":"t1","from":"0x0","to":"alice","value":9007199254740993}`)
	if err := idx.HandleEvent(context.Background(), &client.ChaincodeEvent{EventName: "TransferSingle", Payload: raw, BlockNumber: 5, TransactionID: "tx1"}); err != nil {
		t.Fatal(err)
	}
	got := m.recs[1].args[2]
	if fmt.Sprintf("%v", got) != "9007199254740993" {
		t.Fatalf("amount = %v, want 9007199254740993 (float64 rounds it)", got)
	}
}

func TestAmountOutOfRangeIsRejected(t *testing.T) {
	idx, _ := newIdx()
	raw := []byte(`{"id":"t1","from":"0x0","to":"alice","value":18446744073709551616}`)
	if err := idx.HandleEvent(context.Background(), &client.ChaincodeEvent{EventName: "TransferSingle", Payload: raw, BlockNumber: 5, TransactionID: "tx1"}); err == nil {
		t.Fatal("expected an out-of-range amount to be rejected")
	}
}

func TestTransferBatchCarriesRecipients(t *testing.T) {
	idx, m := newIdx()
	p := map[string]any{
		"ids":    []any{"t1", "t2"},
		"from":   "0x0",
		"tos":    []any{"alice", "bob"},
		"values": []any{1, 2},
	}
	if err := idx.HandleEvent(context.Background(), ev("TransferBatch", p)); err != nil {
		t.Fatal(err)
	}
	// raw insert + 2 credits + checkpoint
	if len(m.recs) != 4 {
		t.Fatalf("expected 4 execs (event + 2 credits + checkpoint), got %d", len(m.recs))
	}
	if m.recs[1].args[1] != "alice" || fmt.Sprintf("%v", m.recs[1].args[2]) != "1" {
		t.Fatalf("first credit: %v", m.recs[1].args)
	}
	if m.recs[2].args[1] != "bob" || fmt.Sprintf("%v", m.recs[2].args[2]) != "2" {
		t.Fatalf("second credit: %v", m.recs[2].args)
	}
}

func TestBalancesUpsertRefreshesUpdatedAt(t *testing.T) {
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("TransferSingle", map[string]any{"id": "t1", "from": "bob", "to": "alice", "value": 4})); err != nil {
		t.Fatal(err)
	}
	if got := m.recs[1].sql; !strings.Contains(got, "updated_at") {
		t.Fatalf("debit stopped touching updated_at: %q", got)
	}
	if got := m.recs[2].sql; !strings.Contains(got, "updated_at") {
		t.Fatalf("credit stopped touching updated_at: %q", got)
	}
}

// queryStub answers the checkpoint read so resumeBlock is testable without a
// database.
type queryStub struct {
	row pgx.Row
}

func (q queryStub) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return q.row }

type rowStub struct{ n uint64 }

func (r rowStub) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("rowStub scans one column")
	}
	*(dest[0].(*uint64)) = r.n
	return nil
}

type rowStubMissing struct{}

func (rowStubMissing) Scan(dest ...any) error { return pgx.ErrNoRows }

func TestResumeBlockFromCheckpoint(t *testing.T) {
	if got, err := resumeBlock(context.Background(), queryStub{row: rowStub{n: 42}}); err != nil || got != 42 {
		t.Fatalf("resumeBlock = %d, %v; want 42", got, err)
	}
	if got, err := resumeBlock(context.Background(), queryStub{row: rowStubMissing{}}); err != nil || got != 0 {
		t.Fatalf("resumeBlock(missing) = %d, %v; want 0, nil", got, err)
	}
}

// TestSchemaAndSQLAgree is the regression guard for the "SQL touched a column
// the schema never created" class of defect: whatever column the balance
// updates stamp must exist in the DDL they run against.
func TestSchemaAndSQLAgree(t *testing.T) {
	if !strings.Contains(schema, "ALTER TABLE balances ADD COLUMN IF NOT EXISTS updated_at") {
		t.Fatal("schema does not migrate an existing balances table to add updated_at")
	}
	idx, m := newIdx()
	if err := idx.HandleEvent(context.Background(), ev("TransferSingle", map[string]any{"id": "t1", "from": "bob", "to": "alice", "value": 4})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.recs[1].sql, "updated_at = now()") {
		t.Fatalf("balance debit does not stamp updated_at: %q", m.recs[1].sql)
	}
	if !strings.Contains(schema, "updated_at TIMESTAMPTZ") {
		t.Fatal("balances schema has no updated_at column")
	}
}
