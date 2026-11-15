package main

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shimtest"
)

// otherCtxOn binds a *different* client identity to the same ledger state.
// It is how these tests express "a third party trying to act for someone else":
// the state is shared, the cert (and therefore the invoker id) is not.
func otherCtxOn(t *testing.T, stub *shimtest.MockStub, cn string) *testCtx {
	t.Helper()
	pem, err := makeCertCN(map[string]string{"role": "PLAYER"}, cn)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := serializeCreator(pem)
	if err != nil {
		t.Fatal(err)
	}
	other := shimtest.NewMockStub("playsmith-assets", nil)
	*other = *stub // share the committed state
	other.Creator = sid
	other.MockTransactionStart("tx-" + cn)
	return newCtx(other)
}

func TestBalanceOverflowIsRejected(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "alice", "u", math.MaxUint64-5); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	// 10 more would wrap past MaxUint64; the transaction must fail rather than
	// commit a wrapped value.
	if err := c.Mint(ctx, "t1", "alice", "u", 10); err == nil {
		t.Fatal("expected balance overflow to be rejected")
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "alice"); bal != math.MaxUint64-5 {
		t.Fatalf("balance = %d, want %d (no wrap)", bal, uint64(math.MaxUint64-5))
	}
	if sup, _ := c.TotalSupply(ctx, "t1"); sup != math.MaxUint64-5 {
		t.Fatalf("supply = %d, want %d", sup, uint64(math.MaxUint64-5))
	}
}

func TestSupplyOverflowIsRejected(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	if err := c.Mint(ctx, "t1", "alice", "u", math.MaxUint64-5); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	// Supply is per token, so the mint that overflows it must credit a *second*
	// recipient: alice's balance stays small while the token's supply would wrap.
	if err := c.Mint(ctx, "t1", "bob", "u", 10); err == nil {
		t.Fatal("expected supply overflow to be rejected")
	}
	if sup, _ := c.TotalSupply(ctx, "t1"); sup != math.MaxUint64-5 {
		t.Fatalf("supply = %d, want %d (no wrap)", sup, uint64(math.MaxUint64-5))
	}
}

func TestSelfTransferKeepsBalance(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	self := testInvokerID(t, ctx)
	if err := c.Mint(ctx, "t1", self, "u", 100); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if err := c.Transfer(ctx, "t1", self, self, 10); err != nil {
		t.Fatal(err)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", self); bal != 100 {
		t.Fatalf("self-transfer balance = %d, want 100", bal)
	}
	if sup, _ := c.TotalSupply(ctx, "t1"); sup != 100 {
		t.Fatalf("supply = %d, want 100", sup)
	}
}

func TestBatchMintAggregatesDuplicateKeys(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	// Two mints to the same token+recipient, one to another recipient. The first
	// two touch one balance key and one supply key each; a per-item loop would
	// read the same committed base twice and keep only the last write.
	err := c.BatchMint(ctx, `["t1","t1","t1"]`, `["alice","alice","bob"]`, `[7,8,4]`, `["u","u","u"]`)
	if err != nil {
		t.Fatal(err)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "alice"); bal != 15 {
		t.Fatalf("alice = %d, want 15", bal)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "bob"); bal != 4 {
		t.Fatalf("bob = %d, want 4", bal)
	}
	if sup, _ := c.TotalSupply(ctx, "t1"); sup != 19 {
		t.Fatalf("supply = %d, want 19", sup)
	}
	payload := lastEventPayload(t, ctx, "TransferBatch")
	// Real Fabric keeps only the last SetEvent of a transaction, so the batch
	// event must carry every recipient itself.
	tos, ok := payload["tos"].([]any)
	if !ok || len(tos) != 3 || tos[0] != "alice" || tos[1] != "alice" || tos[2] != "bob" {
		t.Fatalf("TransferBatch tos = %v", payload["tos"])
	}
	if _, stale := payload["to"]; stale {
		t.Fatalf("TransferBatch should not carry a single %q field: %v", "to", payload)
	}
	if uri, _ := c.URI(ctx, "t1"); uri != "u" {
		t.Fatalf("URI = %q, want u", uri)
	}
}

func TestBatchTransferAggregatesDuplicateKeys(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	from := testInvokerID(t, ctx)
	for _, id := range []string{"t1", "t2", "t3"} {
		if err := c.Mint(ctx, id, from, "u", 10); err != nil {
			t.Fatal(err)
		}
	}
	drainEvents(t, ctx)
	if err := c.BatchTransfer(ctx, `["t1","t2","t3"]`, from, "bob", `[4,5,6]`); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if bal, _ := c.BalanceOf(ctx, "t3", from); bal != 4 {
		t.Fatalf("from t3 = %d, want 4", bal)
	}
	if bal, _ := c.BalanceOf(ctx, "t3", "bob"); bal != 6 {
		t.Fatalf("bob t3 = %d, want 6", bal)
	}
	// Two rows hitting the same token: one debit key and one credit key each, so
	// the batch must net them to a single 2 rather than apply both from the
	// same committed base.
	if err := c.BatchTransfer(ctx, `["t1","t1"]`, from, "carol", `[1,1]`); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if bal, _ := c.BalanceOf(ctx, "t1", "carol"); bal != 2 {
		t.Fatalf("carol = %d, want 2", bal)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", from); bal != 4 {
		t.Fatalf("from t1 = %d, want 4", bal)
	}
	if err := c.BatchTransfer(ctx, `["t1"]`, from, from, `[1]`); err != nil {
		t.Fatal(err)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", from); bal != 4 {
		t.Fatalf("self batch transfer changed balance to %d, want 4", bal)
	}
}

func TestListForSaleRejectsForeignIdentity(t *testing.T) {
	c := &GameContract{}
	stub, err := newStub(map[string]string{"role": "MINTER"})
	if err != nil {
		t.Fatal(err)
	}
	owner := newCtx(stub)
	seller := testInvokerID(t, owner)
	if err := c.Mint(owner, "t1", seller, "u", 5); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, owner)
	third := otherCtxOn(t, stub, "third")
	if _, err := c.ListForSale(third, "t1", seller, 3, 9); err == nil {
		t.Fatal("expected a non-owner listing to be rejected")
	}
	// The third party cannot cancel someone else's listing either.
	if _, err := c.ListForSale(owner, "t1", seller, 3, 9); err != nil {
		t.Fatal(err)
	}
	key, _ := stub.CreateCompositeKey("ask", []string{"t1", seller, "tx-test"})
	if raw, _ := stub.GetState(key); len(raw) == 0 {
		t.Fatal("listing disappeared")
	}
	if err := c.CancelListing(third, "t1", seller, "tx-test"); err == nil {
		t.Fatal("expected a non-owner cancel to be rejected")
	}
	if raw, _ := stub.GetState(key); len(raw) == 0 {
		t.Fatal("rejected cancel deleted the listing")
	}
	if err := c.CancelListing(owner, "t1", seller, "tx-test"); err != nil {
		t.Fatalf("owner cancel failed: %v", err)
	}
	if raw, _ := stub.GetState(key); len(raw) != 0 {
		t.Fatal("owner cancel left the listing")
	}
	// An unknown listing id is not the same as a permission failure.
	if err := c.CancelListing(owner, "t1", seller, "no-such-listing"); err == nil {
		t.Fatal("expected a missing listing to be rejected")
	}
}

func TestListForSalePrivateRejectsForeignIdentity(t *testing.T) {
	c := &GameContract{}
	stub, err := newStub(map[string]string{"role": "MINTER"})
	if err != nil {
		t.Fatal(err)
	}
	owner := newCtx(stub)
	seller := testInvokerID(t, owner)
	if err := c.Mint(owner, "t1", seller, "u", 5); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, owner)
	if err := stub.SetTransient(map[string][]byte{"price": []byte("42")}); err != nil {
		t.Fatal(err)
	}
	third := otherCtxOn(t, stub, "third")
	if _, err := c.ListForSalePrivate(third, "t1", seller, 3); err == nil {
		t.Fatal("expected a non-owner private listing to be rejected")
	}
	if _, err := c.ListForSalePrivate(owner, "t1", seller, 3); err != nil {
		t.Fatalf("owner private listing failed: %v", err)
	}
}

func TestSetApprovalForAllIsOwnerOnly(t *testing.T) {
	c := &GameContract{}
	stub, err := newStub(map[string]string{"role": "MINTER"})
	if err != nil {
		t.Fatal(err)
	}
	owner := newCtx(stub)
	ownerID := testInvokerID(t, owner)
	if err := c.Mint(owner, "t1", ownerID, "u", 5); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, owner)
	third := otherCtxOn(t, stub, "third")
	thirdID := testInvokerID(t, third)

	// A stranger self-authorising as operator for the owner is the theft
	// primitive the old code allowed.
	if err := c.SetApprovalForAll(third, ownerID, thirdID, "true"); err == nil {
		t.Fatal("expected a foreign approval grant to be rejected")
	}
	if ok, _ := c.IsApprovedForAll(owner, ownerID, thirdID); ok {
		t.Fatal("rejected grant still wrote state")
	}
	if err := c.SetApprovalForAll(owner, ownerID, thirdID, "true"); err != nil {
		t.Fatalf("owner grant failed: %v", err)
	}
	drainEvents(t, owner)
	if ok, _ := c.IsApprovedForAll(owner, ownerID, thirdID); !ok {
		t.Fatal("owner grant did not take effect")
	}
}

func TestBuyRejectsExcessAmount(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	seller := testInvokerID(t, ctx)
	if err := c.Mint(ctx, "t1", seller, "u", 100); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	listingID, err := c.ListForSale(ctx, "t1", seller, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if err := c.Buy(ctx, "t1", seller, listingID, "buyer", 100); err == nil {
		t.Fatal("expected an over-large buy to be rejected")
	}
	if bal, _ := c.BalanceOf(ctx, "t1", seller); bal != 100 {
		t.Fatalf("seller balance = %d, want 100", bal)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "buyer"); bal != 0 {
		t.Fatalf("buyer balance = %d, want 0", bal)
	}
	stub := ctx.GetStub().(*shimtest.MockStub)
	key, _ := stub.CreateCompositeKey("ask", []string{"t1", seller, listingID})
	raw, _ := stub.GetState(key)
	var ask map[string]any
	if len(raw) == 0 {
		t.Fatal("rejected buy consumed the listing")
	}
	if err := json.Unmarshal(raw, &ask); err != nil {
		t.Fatal(err)
	}
	if ask["amount"] != float64(2) {
		t.Fatalf("listing amount = %v, want 2", ask["amount"])
	}
}

func TestBuyPartialFillRetainsRemainder(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	seller := testInvokerID(t, ctx)
	if err := c.Mint(ctx, "t1", seller, "u", 10); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	listingID, err := c.ListForSale(ctx, "t1", seller, 5, 9)
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if err := c.Buy(ctx, "t1", seller, listingID, "buyer", 2); err != nil {
		t.Fatal(err)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "buyer"); bal != 2 {
		t.Fatalf("buyer = %d, want 2", bal)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", seller); bal != 8 {
		t.Fatalf("seller = %d, want 8", bal)
	}
	stub := ctx.GetStub().(*shimtest.MockStub)
	key, _ := stub.CreateCompositeKey("ask", []string{"t1", seller, listingID})
	raw, _ := stub.GetState(key)
	if len(raw) == 0 {
		t.Fatal("partial fill deleted the remainder")
	}
	var ask map[string]any
	if err := json.Unmarshal(raw, &ask); err != nil {
		t.Fatal(err)
	}
	if ask["amount"] != float64(3) || ask["price"] != float64(9) {
		t.Fatalf("remaining listing = %v", ask)
	}
	if err := c.Buy(ctx, "t1", seller, listingID, "buyer", 3); err != nil {
		t.Fatal(err)
	}
	if raw, _ := stub.GetState(key); len(raw) != 0 {
		t.Fatal("full fill left the listing")
	}
	if bal, _ := c.BalanceOf(ctx, "t1", "buyer"); bal != 5 {
		t.Fatalf("buyer = %d, want 5", bal)
	}
}

func TestBuyFromSelf(t *testing.T) {
	c := &GameContract{}
	ctx := getCtx(t, map[string]string{"role": "MINTER"})
	self := testInvokerID(t, ctx)
	if err := c.Mint(ctx, "t1", self, "u", 10); err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	listingID, err := c.ListForSale(ctx, "t1", self, 10, 9)
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(t, ctx)
	if err := c.Buy(ctx, "t1", self, listingID, self, 4); err != nil {
		t.Fatal(err)
	}
	if bal, _ := c.BalanceOf(ctx, "t1", self); bal != 10 {
		t.Fatalf("self-buy balance = %d, want 10", bal)
	}
	stub := ctx.GetStub().(*shimtest.MockStub)
	key, _ := stub.CreateCompositeKey("ask", []string{"t1", self, listingID})
	raw, _ := stub.GetState(key)
	if len(raw) == 0 {
		t.Fatal("self-buy deleted the remainder")
	}
}