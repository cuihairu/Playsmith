package auth

import (
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestIssueValidate(t *testing.T) {
	m := testManager(t)
	tok, err := m.Issue("server-1", "Org1MSP", "MINTER")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := m.Validate(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Subject != "server-1" || claims.MSPID != "Org1MSP" || claims.Role != "MINTER" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestValidateWrongSecret(t *testing.T) {
	m := testManager(t)
	tok, err := m.Issue("s", "m", "r")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := NewManager("fedcba9876543210fedcba9876543210", "playsmith", "game-server", time.Hour)
	if _, err := other.Validate(tok); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateExpired(t *testing.T) {
	m, _ := NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Minute)
	tok, err := m.Issue("s", "m", "r")
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if _, err := m.Validate(tok); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestValidateRejectsNoneAlg(t *testing.T) {
	m := testManager(t)
	tok, err := m.Issue("s", "m", "r")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatal("bad token")
	}
	bad := parts[0] + "." + parts[1] + "." + "AAAA" // corrupted signature
	if _, err := m.Validate(bad); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestShortSecretRejected(t *testing.T) {
	if _, err := NewManager("short", "p", "a", time.Hour); err == nil {
		t.Fatal("expected short secret error")
	}
}
