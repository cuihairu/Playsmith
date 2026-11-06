package sdk

import (
	"context"
	"crypto/x509"
	"errors"
	"os"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config for a real Fabric Gateway client.
type Config struct {
	MSPID          string
	CertPath       string
	KeyPath        string
	TLSCertPath    string
	GatewayAddress string
	ChannelName    string
	ChaincodeName  string
}

// GatewayClient implements Client using Fabric Gateway.
type GatewayClient struct {
	gateway *client.Gateway
	network *client.Network
}

// NewGatewayClient connects to a Fabric Gateway peer.
func NewGatewayClient(ctx context.Context, cfg Config) (*GatewayClient, error) {
	certPEM, err := os.ReadFile(cfg.CertPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	tlsPEM, err := os.ReadFile(cfg.TLSCertPath)
	if err != nil {
		return nil, err
	}
	cert, err := identity.CertificateFromPEM(certPEM)
	if err != nil {
		return nil, err
	}
	key, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, err
	}
	id, err := identity.NewX509Identity(cfg.MSPID, cert)
	if err != nil {
		return nil, err
	}
	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(tlsPEM) {
		return nil, errors.New("invalid TLS certificate")
	}
	grpcConn, err := grpc.NewClient(cfg.GatewayAddress, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(pool, "")))
	if err != nil {
		return nil, err
	}
	gateway, err := client.Connect(id, client.WithSign(sign), client.WithClientConnection(grpcConn))
	if err != nil {
		return nil, err
	}
	network := gateway.GetNetwork(cfg.ChannelName)
	return &GatewayClient{gateway: gateway, network: network}, nil
}

func (c *GatewayClient) Submit(contract, txn string, args ...string) (string, string, error) {
	cc := c.network.GetContract(contract)
	res, commit, err := cc.SubmitAsync(txn, client.WithArguments(args...))
	if err != nil {
		return "", "", err
	}
	status, err := commit.Status()
	if err != nil {
		return "", "", err
	}
	if !status.Successful {
		return "", "", newCommitError(status.TransactionID, status.Code)
	}
	return string(res), status.TransactionID, nil
}

func (c *GatewayClient) SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, string, error) {
	cc := c.network.GetContract(contract)
	res, commit, err := cc.SubmitAsync(txn, client.WithArguments(args...), client.WithTransient(transient))
	if err != nil {
		return "", "", err
	}
	status, err := commit.Status()
	if err != nil {
		return "", "", err
	}
	if !status.Successful {
		return "", "", newCommitError(status.TransactionID, status.Code)
	}
	return string(res), status.TransactionID, nil
}

func (c *GatewayClient) Evaluate(contract, txn string, args ...string) (string, error) {
	cc := c.network.GetContract(contract)
	res, err := cc.EvaluateTransaction(txn, args...)
	if err != nil {
		return "", err
	}
	return string(res), nil
}
