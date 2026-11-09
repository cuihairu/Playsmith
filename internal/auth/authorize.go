package auth

import (
	"errors"
	"fmt"
)

var (
	// ErrMissingIdentity is returned when the JWT claims lack the caller's
	// Fabric identity (mspId/role).
	ErrMissingIdentity = errors.New("missing Fabric identity (mspId/role)")
	// ErrNotAuthorized is returned when the claims do not permit the operation.
	ErrNotAuthorized = errors.New("not authorized")
)

// Authorize binds the JWT identity (mspId/role) to per-operation authorization.
// All authenticated operations require a non-empty mspId and role so the caller
// is identifiable; role-gated operations (Mint/BatchMint) additionally require
// role=MINTER, mirroring the chaincode's requireMinter and the SPEC
// Authorization model (docs/SPEC.md).
//
// This enforces authorization at the gateway based on the caller's JWT role,
// independent of the shared Fabric certificate the gateway uses to submit
// transactions, so a low-privilege token cannot reach a privileged on-chain
// operation.
func (c *Claims) Authorize(operation string) error {
	if c.MSPID == "" || c.Role == "" {
		return ErrMissingIdentity
	}
	switch operation {
	case "Mint", "BatchMint":
		if c.Role != "MINTER" {
			return fmt.Errorf("%w: %s requires role=MINTER", ErrNotAuthorized, operation)
		}
	}
	return nil
}
