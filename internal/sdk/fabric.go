package sdk

// Minimal interfaces to decouple API from Fabric SDK implementation.
// Replace NoopClient with a real Fabric Gateway client later.

type Client interface {
    Submit(contract, txn string, args ...string) (string, error)
    SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, error)
    Evaluate(contract, txn string, args ...string) (string, error)
}

type NoopClient struct{}

func (n *NoopClient) Submit(contract, txn string, args ...string) (string, error) {
    // TODO: implement with github.com/hyperledger/fabric-gateway/pkg/client
    return "TX_NOOP", nil
}

func (n *NoopClient) SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, error) {
    return "TX_NOOP", nil
}

func (n *NoopClient) Evaluate(contract, txn string, args ...string) (string, error) {
	return "{}", nil
}

// Placeholder to show how a real client might be constructed
// func NewGatewayClient(cfg Config) (Client, error) { ... }
