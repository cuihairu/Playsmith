package sdk

// Minimal interfaces to decouple API from Fabric SDK implementation.
// Submit returns the transaction result and the Fabric transaction ID so
// callers (REST/gRPC) can expose tx_id for client-side tracking.

type Client interface {
	Submit(contract, txn string, args ...string) (string, string, error)
	SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, string, error)
	Evaluate(contract, txn string, args ...string) (string, error)
}

// NoopClient is the intentional dev-mode stand-in, wired in by cmd/api when
// FABRIC_GATEWAY_ENDPOINT is unset so the API still runs without a network.
// It is not an unfinished implementation: the real GatewayClient lives in
// gateway.go.
type NoopClient struct{}

func (n *NoopClient) Submit(contract, txn string, args ...string) (string, string, error) {
	return "TX_NOOP", "", nil
}

func (n *NoopClient) SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, string, error) {
	return "TX_NOOP", "", nil
}

func (n *NoopClient) Evaluate(contract, txn string, args ...string) (string, error) {
	return "{}", nil
}
