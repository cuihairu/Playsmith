package main

import (
	"github.com/hyperledger/fabric-chaincode-go/pkg/cid"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
)

// testCtx implements contractapi.TransactionContextInterface for unit tests.
type testCtx struct {
	stub  *shimtest.MockStub
	cid   cid.ClientIdentity
}

func (t *testCtx) GetStub() shim.ChaincodeStubInterface { return t.stub }
func (t *testCtx) GetClientIdentity() cid.ClientIdentity { return t.cid }

// setCtx populates the client identity from the stub creator.
func (t *testCtx) setCtx() error {
	ci, err := cid.New(t.stub)
	if err != nil {
		return err
	}
	t.cid = ci
	return nil
}
