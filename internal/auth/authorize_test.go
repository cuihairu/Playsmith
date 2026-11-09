package auth

import (
	"errors"
	"testing"
)

func TestAuthorizeRequiresIdentity(t *testing.T) {
	c := &Claims{MSPID: "", Role: ""}
	if err := c.Authorize("Mint"); !errors.Is(err, ErrMissingIdentity) {
		t.Fatalf("empty identity: err=%v, want ErrMissingIdentity", err)
	}
	c = &Claims{MSPID: "Org1MSP", Role: ""}
	if err := c.Authorize("Mint"); !errors.Is(err, ErrMissingIdentity) {
		t.Fatalf("empty role: err=%v, want ErrMissingIdentity", err)
	}
}

func TestAuthorizeMinterGate(t *testing.T) {
	// Non-minter must be rejected for role-gated operations.
	c := &Claims{MSPID: "Org1MSP", Role: "PLAYER"}
	if err := c.Authorize("Mint"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("PLAYER on Mint: err=%v, want ErrNotAuthorized", err)
	}
	if err := c.Authorize("BatchMint"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("PLAYER on BatchMint: err=%v, want ErrNotAuthorized", err)
	}
	// Minter passes for role-gated operations.
	m := &Claims{MSPID: "Org1MSP", Role: "MINTER"}
	if err := m.Authorize("Mint"); err != nil {
		t.Fatalf("MINTER on Mint: %v", err)
	}
	if err := m.Authorize("BatchMint"); err != nil {
		t.Fatalf("MINTER on BatchMint: %v", err)
	}
}

func TestAuthorizeNonGatedOperationsAnyRole(t *testing.T) {
	// Operations without a role gate only require an identity.
	for _, op := range []string{"Transfer", "Buy", "ListForSale", "BalanceOf"} {
		c := &Claims{MSPID: "Org1MSP", Role: "PLAYER"}
		if err := c.Authorize(op); err != nil {
			t.Fatalf("%s with identity: %v", op, err)
		}
	}
}
