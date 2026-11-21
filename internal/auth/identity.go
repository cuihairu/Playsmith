package auth

import (
	"errors"
	"fmt"
	"strings"
)

// ErrIdentityUnbound is returned when claims do not match any configured
// identity binding.
var ErrIdentityUnbound = errors.New("identity is not accepted by JWT_IDENTITY_MAP")

// IdentityBinding pins one JWT subject to the Fabric organization identity it
// may act under, optionally pinning the role as well. It is one entry of the
// trusted subject-to-identity mapping configured by JWT_IDENTITY_MAP.
type IdentityBinding struct {
	Subject string
	MSPID   string
	Role    string // optional: when set, claims.Role must equal it
}

// IdentityRegistry is the trusted subject-to-identity mapping checked after
// token validation and per-operation authorization. It gates which JWT
// subjects may act through the gateway and under which Fabric organization,
// so a stolen or over-broadly-issued token for an unbound subject is rejected
// before any Fabric call. It does not select signing identities: the gateway
// still submits under one configured X.509 identity until per-game-server
// certificates exist (todo.md, Codex deep review, JWT identity isolation).
type IdentityRegistry struct {
	bindings []IdentityBinding
}

// ParseIdentityMap parses a JWT_IDENTITY_MAP specification: bindings
// separated by ";", each "subject:mspId" or "subject:mspId:role". An empty
// spec yields a nil registry, meaning no subject binding is enforced.
func ParseIdentityMap(spec string) (*IdentityRegistry, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	r := &IdentityRegistry{}
	for _, entry := range strings.Split(spec, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, fmt.Errorf("JWT_IDENTITY_MAP has an empty binding")
		}
		parts := strings.Split(entry, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("JWT_IDENTITY_MAP entry %q must be subject:mspId[:role]", entry)
		}
		b := IdentityBinding{Subject: strings.TrimSpace(parts[0]), MSPID: strings.TrimSpace(parts[1])}
		if b.Subject == "" || b.MSPID == "" {
			return nil, fmt.Errorf("JWT_IDENTITY_MAP entry %q has an empty subject or mspId", entry)
		}
		if len(parts) == 3 {
			b.Role = strings.TrimSpace(parts[2])
			if b.Role == "" {
				return nil, fmt.Errorf("JWT_IDENTITY_MAP entry %q has an empty role", entry)
			}
		}
		r.bindings = append(r.bindings, b)
	}
	return r, nil
}

// Check verifies claims against the configured bindings: subject and mspId
// must match one entry, and if that entry pins a role, claims.Role must equal
// it. A nil registry enforces nothing, preserving the pre-mapping behavior.
func (r *IdentityRegistry) Check(c *Claims) error {
	if r == nil {
		return nil
	}
	for _, b := range r.bindings {
		if b.Subject != c.Subject || b.MSPID != c.MSPID {
			continue
		}
		if b.Role != "" && b.Role != c.Role {
			return fmt.Errorf("%w: role mismatch for subject %q", ErrIdentityUnbound, c.Subject)
		}
		return nil
	}
	return fmt.Errorf("%w: subject %q under mspId %q", ErrIdentityUnbound, c.Subject, c.MSPID)
}
