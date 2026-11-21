package auth

import (
	"errors"
	"testing"
)

func TestParseIdentityMapEmpty(t *testing.T) {
	for _, spec := range []string{"", "  "} {
		reg, err := ParseIdentityMap(spec)
		if reg != nil || err != nil {
			t.Fatalf("spec %q: reg=%v err=%v, want nil/nil", spec, reg, err)
		}
	}
}

func TestParseIdentityMapBindings(t *testing.T) {
	reg, err := ParseIdentityMap("server-1:Org1MSP; server-2:Org2MSP:PLAYER")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if reg == nil || len(reg.bindings) != 2 {
		t.Fatalf("bindings = %+v, want 2", reg)
	}
	if reg.bindings[0].Subject != "server-1" || reg.bindings[0].MSPID != "Org1MSP" || reg.bindings[0].Role != "" {
		t.Fatalf("binding 0 = %+v", reg.bindings[0])
	}
	if reg.bindings[1].Subject != "server-2" || reg.bindings[1].MSPID != "Org2MSP" || reg.bindings[1].Role != "PLAYER" {
		t.Fatalf("binding 1 = %+v", reg.bindings[1])
	}
}

func TestParseIdentityMapRejectsMalformed(t *testing.T) {
	for _, spec := range []string{
		"a",           // no mspId part
		"a:Org1:R:x",  // too many parts
		":Org1",       // empty subject
		"a:",          // empty mspId
		"a:Org1:",     // empty role
		"a:Org1;;b:O", // empty entry between separators
	} {
		if _, err := ParseIdentityMap(spec); err == nil {
			t.Fatalf("spec %q accepted; want error", spec)
		}
	}
}

func TestCheckIdentityMap(t *testing.T) {
	reg, err := ParseIdentityMap("server-1:Org1MSP;server-2:Org2MSP:MINTER")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Bound subject under its bound mspId passes without a role pin.
	claims := &Claims{MSPID: "Org1MSP", Role: "PLAYER"}
	claims.Subject = "server-1"
	if err := reg.Check(claims); err != nil {
		t.Fatalf("bound subject rejected: %v", err)
	}
	// Role pin satisfied.
	claims = &Claims{MSPID: "Org2MSP", Role: "MINTER"}
	claims.Subject = "server-2"
	if err := reg.Check(claims); err != nil {
		t.Fatalf("role-pinned subject rejected: %v", err)
	}
	// Role pin violated.
	claims = &Claims{MSPID: "Org2MSP", Role: "PLAYER"}
	claims.Subject = "server-2"
	if err := reg.Check(claims); !errors.Is(err, ErrIdentityUnbound) {
		t.Fatalf("role mismatch: err=%v, want ErrIdentityUnbound", err)
	}
	// Unbound subject.
	claims = &Claims{MSPID: "Org1MSP", Role: "MINTER"}
	claims.Subject = "intruder"
	if err := reg.Check(claims); !errors.Is(err, ErrIdentityUnbound) {
		t.Fatalf("unbound subject: err=%v, want ErrIdentityUnbound", err)
	}
	// Bound subject under the wrong mspId.
	claims = &Claims{MSPID: "Org9MSP", Role: "MINTER"}
	claims.Subject = "server-1"
	if err := reg.Check(claims); !errors.Is(err, ErrIdentityUnbound) {
		t.Fatalf("wrong mspId: err=%v, want ErrIdentityUnbound", err)
	}
	// Nil registry enforces nothing.
	if err := (*IdentityRegistry)(nil).Check(&Claims{MSPID: "Org1MSP", Role: "PLAYER"}); err != nil {
		t.Fatalf("nil registry rejected claims: %v", err)
	}
}
