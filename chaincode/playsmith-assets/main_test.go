package main

import (
	"encoding/json"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-protos-go/peer"
)

func getCtx(t *testing.T, attrs map[string]string) *testCtx {
	t.Helper()
	stub, err := newStub(attrs)
	if err != nil {
		t.Fatalf("newStub: %v", err)
	}
	return newCtx(stub)
}

func drainEvents(t *testing.T, ctx *testCtx) []*peer.ChaincodeEvent {
	t.Helper()
	stub := ctx.GetStub().(*shimtest.MockStub)
	var evs []*peer.ChaincodeEvent
	for {
		select {
		case ev := <-stub.ChaincodeEventsChannel:
			evs = append(evs, ev)
		default:
			return evs
		}
	}
}

func lastEventPayload(t *testing.T, ctx *testCtx, name string) map[string]any {
	t.Helper()
	for _, ev := range drainEvents(t, ctx) {
		if ev.GetEventName() == name {
			var m map[string]any
			if err := json.Unmarshal(ev.GetPayload(), &m); err != nil {
				t.Fatalf("unmarshal %s event: %v", name, err)
			}
			return m
		}
	}
	t.Fatalf("event %s not found", name)
	return nil
}

// invokerID returns the client identity id for a context.
func testInvokerID(t *testing.T, ctx *testCtx) string {
	t.Helper()
	id, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		t.Fatalf("GetID: %v", err)
	}
	return id
}

func TestMintRequiresMinterRole(t *testing.T) {
	c := &GameContract{}
	if err := c.Mint(getCtx(t, map[string]string{"role": "PLAYER"}), "t1", "alice", "u", 10); err == nil {
		t.Fatal("expected non-minter to be rejected")
	}
	if err := c.Mint(getCtx(t, nil), "t1", "alice", "u", 10); err == nil {
		t.Fatal("expected missing role to be rejected")
	}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "alice", "u", 10); err != nil {
		t.Fatalf("expected minter success: %v", err)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "alice"); bal != 10 {
		t.Fatalf("balance after mint = %d, want 10", bal)
	}
}

func TestTransferOwnerAndOperator(t *testing.T) {
	c := &GameContract{}
	stub, err := newStub(map[string]string{"role": "MINTER"})
	if err != nil {
		t.Fatal(err)
	}
	ownerCtx := newCtx(stub)
	owner := testInvokerID(t, ownerCtx)
	if err := c.Mint(ownerCtx, "t1", owner, "u", 10); err != nil {
		t.Fatal(err)
	}

	// owner (same invoker) can transfer
	if err := c.Transfer(ownerCtx, "t1", owner, "bob", 5); err != nil {
		t.Fatalf("owner transfer failed: %v", err)
	}
	if bal, _ := c.BalanceOf(ownerCtx, "t1", "bob"); bal != 5 {
		t.Fatalf("bob balance = %d, want 5", bal)
	}

	// A different invoker on the SAME ledger stub.
	otherPEM, err := makeCertCN(map[string]string{"role": "PLAYER"}, "other")
	if err != nil {
		t.Fatal(err)
	}
	otherStub := shimtest.NewMockStub("playsmith-assets", nil)
	*otherStub = *stub // share state
	otherStub.Creator = nil
	sidBytes, err := serializeCreator(otherPEM)
	if err != nil {
		t.Fatal(err)
	}
	otherStub.Creator = sidBytes
	otherCtx := newCtx(otherStub)
	otherID := testInvokerID(t, otherCtx)

	// unapproved third party cannot transfer owner balance
	if err := c.Transfer(otherCtx, "t1", owner, otherID, 1); err == nil {
		t.Fatal("expected unauthorized transfer to fail")
	}

	// approve third party as operator, then transfer succeeds
	if err := c.SetApprovalForAll(ownerCtx, owner, otherID, "true"); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ownerCtx)
	if err := c.Transfer(otherCtx, "t1", owner, otherID, 1); err != nil {
		t.Fatalf("operator transfer failed: %v", err)
	}
}

func TestListForSaleRequiresBalance(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "seller", "u", 3); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if _, err := c.ListForSale(ctx, "t1", "seller", 5, 100); err == nil {
		t.Fatal("expected insufficient balance to be rejected")
	}
	listingID, err := c.ListForSale(ctx, "t1", "seller", 3, 100)
	if err != nil {
		t.Fatalf("expected successful listing: %v", err)
	}
	if listingID == "" {
		t.Fatal("expected listing id")
	}
}

func TestPublicAskIsJSONWithTS(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "seller", "u", 5); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	listingID, err := c.ListForSale(ctx, "t1", "seller", 5, 42)
	if err != nil {
		t.Fatal(err)
	}
	stub := ctx.GetStub().(*shimtest.MockStub)
	key, err := stub.CreateCompositeKey("ask", []string{"t1", "seller", listingID})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := stub.GetState(key)
	if err != nil || len(raw) == 0 {
		t.Fatalf("ask state missing: %v %q", err, raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("ask value not JSON: %v %q", err, raw)
	}
	if _, ok := m["ts"]; !ok {
		t.Fatalf("ask JSON missing ts: %q", raw)
	}
	if m["price"] != float64(42) || m["amount"] != float64(5) {
		t.Fatalf("ask fields wrong: %q", raw)
	}
}

func TestTradeEventPriceFields(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "seller", "u", 2); err != nil {
		t.Fatal(err)
	}
	listingID, err := c.ListForSale(ctx, "t1", "seller", 2, 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Buy(ctx, "t1", "seller", listingID, "buyer", 2); err != nil {
		t.Fatal(err)
	}
	payload := lastEventPayload(t, ctx, "Trade")
	if payload["price"] != float64(7) {
		t.Fatalf("Trade price = %v, want 7", payload["price"])
	}
	if _, hasHash := payload["priceHash"]; hasHash {
		t.Fatal("public Trade should not carry priceHash")
	}
}

func TestPrivateTradeEventPriceHash(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "seller", "u", 2); err != nil {
		t.Fatal(err)
	}
	stub := ctx.GetStub().(*shimtest.MockStub)
	if err := stub.SetTransient(map[string][]byte{"price": []byte("99")}); err != nil {
		t.Fatal(err)
	}
	listingID, err := c.ListForSalePrivate(ctx, "t1", "seller", 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Buy(ctx, "t1", "seller", listingID, "buyer", 2); err != nil {
		t.Fatal(err)
	}
	payload := lastEventPayload(t, ctx, "Trade")
	if payload["price"] != float64(0) {
		t.Fatalf("private Trade price = %v, want 0", payload["price"])
	}
	hash, ok := payload["priceHash"].(string)
	if !ok || hash == "" {
		t.Fatalf("private Trade missing priceHash: %v", payload)
	}
}

func TestBuySupportsLegacyAskFormat(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "seller", "u", 2); err != nil {
		t.Fatal(err)
	}
	stub := ctx.GetStub().(*shimtest.MockStub)
	stub.MockTransactionStart("legacy-listing")
	key, err := stub.CreateCompositeKey("ask", []string{"t1", "seller", "legacy-listing"})
	if err != nil {
		t.Fatal(err)
	}
	if err := stub.PutState(key, []byte("77:2")); err != nil {
		t.Fatal(err)
	}
	if err := c.Buy(ctx, "t1", "seller", "legacy-listing", "buyer", 2); err != nil {
		t.Fatalf("legacy ask should remain buyable: %v", err)
	}
	payload := lastEventPayload(t, ctx, "Trade")
	if payload["price"] != float64(77) {
		t.Fatalf("legacy Trade price = %v, want 77", payload["price"])
	}
}
