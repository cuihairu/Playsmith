package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-chaincode-go/pkg/attrmgr"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-protos-go/msp"
)

// makeCert returns a self-signed X.509 cert (PEM) with optional fabric attributes.
// commonName differentiates identities (cid.GetID is derived from Subject/Issuer DN).
func makeCert(attrs map[string]string) ([]byte, error) {
	return makeCertCN(attrs, "test")
}

func makeCertCN(attrs map[string]string, cn string) ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn, OrganizationalUnit: []string{"Fabric"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	if len(attrs) > 0 {
		mgr := attrmgr.New()
		attrList := make([]attrmgr.Attribute, 0, len(attrs))
		for k, v := range attrs {
			attrList = append(attrList, &testAttr{Name: k, Value: v})
		}
		reqs := make([]attrmgr.AttributeRequest, 0, len(attrs))
		for k := range attrs {
			reqs = append(reqs, &testAttrReq{Name: k, Require: true})
		}
		if err := mgr.ProcessAttributeRequestsForCert(reqs, attrList, tmpl); err != nil {
			return nil, err
		}
		// ProcessAttributeRequestsForCert appends to Extensions, but
		// x509.CreateCertificate serializes ExtraExtensions.
		tmpl.ExtraExtensions = tmpl.Extensions
		tmpl.Extensions = nil
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

type testAttr struct{ Name, Value string }

func (a *testAttr) GetName() string  { return a.Name }
func (a *testAttr) GetValue() string { return a.Value }

type testAttrReq struct{ Name string; Require bool }

func (a *testAttrReq) GetName() string   { return a.Name }
func (a *testAttrReq) IsRequired() bool  { return a.Require }

// newStub creates a MockStub with a creator identity carrying the given cert attributes.
func newStub(attrs map[string]string) (*shimtest.MockStub, error) {
	return newStubCN(attrs, "test")
}

func newStubCN(attrs map[string]string, cn string) (*shimtest.MockStub, error) {
	stub := shimtest.NewMockStub("playsmith-assets", nil)
	certPEM, err := makeCertCN(attrs, cn)
	if err != nil {
		return nil, err
	}
	return stubWithCert(stub, certPEM)
}

func serializeCreator(certPEM []byte) ([]byte, error) {
	sid := &msp.SerializedIdentity{Mspid: "Org1", IdBytes: certPEM}
	creator, err := proto.Marshal(sid)
	if err != nil {
		return nil, err
	}
	return creator, nil
}

func stubWithCert(stub *shimtest.MockStub, certPEM []byte) (*shimtest.MockStub, error) {
	creator, err := serializeCreator(certPEM)
	if err != nil {
		return nil, err
	}
	stub.Creator = creator
	stub.MockTransactionStart("tx-test")
	return stub, nil
}

// newCtx wraps a MockStub into a contractapi TransactionContext (via helper).
func newCtx(stub *shimtest.MockStub) *testCtx {
	t := &testCtx{stub: stub}
	_ = t.setCtx()
	return t
}
