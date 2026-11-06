package sdk

import (
	"fmt"

	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
)

// CommitError reports a submitted transaction that failed to commit,
// carrying the Fabric transaction ID for client-side tracking.
type CommitError struct {
	TransactionID string
	Code          int32
}

func (e *CommitError) Error() string {
	return fmt.Sprintf("transaction %s failed to commit with status code %d", e.TransactionID, e.Code)
}

func newCommitError(transactionID string, code peer.TxValidationCode) error {
	return &CommitError{TransactionID: transactionID, Code: int32(code)}
}
