package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// GameContract implements a simple 1155-like asset and marketplace
// Minimal, production should add authorization, operator approvals, and better validation.
type GameContract struct {
	contractapi.Contract
}

type Meta struct {
	TokenID   string `json:"tokenId"`
	URI       string `json:"uri"`
	Creator   string `json:"creator"`
	SupplyCap uint64 `json:"supplyCap,omitempty"`
}

// Mint mints amount of tokenID to 'to' and writes meta if first time.
func (c *GameContract) Mint(ctx contractapi.TransactionContextInterface, tokenID, to, uri string, amount uint64) error {
	if tokenID == "" || to == "" || amount == 0 {
		return errors.New("invalid args")
	}
	// TODO: check client role attribute == MINTER
	if err := putAddBalance(ctx, tokenID, to, amount); err != nil {
		return err
	}
	metaKey, _ := ctx.GetStub().CreateCompositeKey("meta", []string{tokenID})
	exists, err := ctx.GetStub().GetState(metaKey)
	if err != nil {
		return err
	}
	if len(exists) == 0 {
		b, _ := json.Marshal(Meta{TokenID: tokenID, URI: uri, Creator: to})
		if err := ctx.GetStub().PutState(metaKey, b); err != nil {
			return err
		}
	}
	return ctx.GetStub().SetEvent("Mint", []byte(tokenID))
}

// Transfer moves amount of tokenID from -> to
func (c *GameContract) Transfer(ctx contractapi.TransactionContextInterface, tokenID, from, to string, amount uint64) error {
	if tokenID == "" || from == "" || to == "" || amount == 0 {
		return errors.New("invalid args")
	}
	// TODO: check ownership / operator approvals
	if err := putSubBalance(ctx, tokenID, from, amount); err != nil {
		return err
	}
	if err := putAddBalance(ctx, tokenID, to, amount); err != nil {
		return err
	}
	evt := fmt.Sprintf("%s:%s->%s:%d", tokenID, from, to, amount)
	return ctx.GetStub().SetEvent("Transfer", []byte(evt))
}

// ListForSale creates a listing entry. Price model simplified.
func (c *GameContract) ListForSale(ctx contractapi.TransactionContextInterface, tokenID, seller string, amount uint64, price uint64) (string, error) {
	if tokenID == "" || seller == "" || amount == 0 {
		return "", errors.New("invalid args")
	}
	// Note: in production use private data collection for sensitive price
	listingID := ctx.GetStub().GetTxID()
	key, _ := ctx.GetStub().CreateCompositeKey("ask", []string{tokenID, seller, listingID})
	val := []byte(strconv.FormatUint(price, 10) + ":" + strconv.FormatUint(amount, 10))
	if err := ctx.GetStub().PutState(key, val); err != nil {
		return "", err
	}
	if err := ctx.GetStub().SetEvent("List", []byte(key)); err != nil {
		return "", err
	}
	return listingID, nil
}

// Buy consumes a listing and transfers amount from seller to buyer
func (c *GameContract) Buy(ctx contractapi.TransactionContextInterface, tokenID, seller, listingID, buyer string, amount uint64) error {
	if tokenID == "" || seller == "" || buyer == "" || listingID == "" || amount == 0 {
		return errors.New("invalid args")
	}
	askKey, _ := ctx.GetStub().CreateCompositeKey("ask", []string{tokenID, seller, listingID})
	v, err := ctx.GetStub().GetState(askKey)
	if err != nil {
		return err
	}
	if len(v) == 0 {
		return errors.New("listing not found")
	}
	if err := putSubBalance(ctx, tokenID, seller, amount); err != nil {
		return err
	}
	if err := putAddBalance(ctx, tokenID, buyer, amount); err != nil {
		return err
	}
	if err := ctx.GetStub().DelState(askKey); err != nil {
		return err
	}
	return ctx.GetStub().SetEvent("Trade", []byte(listingID))
}

// BalanceOf returns current balance for owner of tokenID
func (c *GameContract) BalanceOf(ctx contractapi.TransactionContextInterface, tokenID, owner string) (uint64, error) {
	k, _ := balKey(ctx, tokenID, owner)
	b, err := ctx.GetStub().GetState(k)
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	v, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// helpers
func balKey(ctx contractapi.TransactionContextInterface, tokenID, owner string) (string, error) {
	return ctx.GetStub().CreateCompositeKey("bal", []string{tokenID, owner})
}

func putAddBalance(ctx contractapi.TransactionContextInterface, tokenID, owner string, amt uint64) error {
	k, _ := balKey(ctx, tokenID, owner)
	cur, err := ctx.GetStub().GetState(k)
	if err != nil {
		return err
	}
	var n uint64
	if len(cur) > 0 {
		v, err := strconv.ParseUint(string(cur), 10, 64)
		if err != nil {
			return err
		}
		n = v
	}
	return ctx.GetStub().PutState(k, []byte(strconv.FormatUint(n+amt, 10)))
}

func putSubBalance(ctx contractapi.TransactionContextInterface, tokenID, owner string, amt uint64) error {
	k, _ := balKey(ctx, tokenID, owner)
	cur, err := ctx.GetStub().GetState(k)
	if err != nil {
		return err
	}
	v, err := strconv.ParseUint(string(cur), 10, 64)
	if err != nil {
		return err
	}
	if v < amt {
		return errors.New("insufficient balance")
	}
	return ctx.GetStub().PutState(k, []byte(strconv.FormatUint(v-amt, 10)))
}

func main() {
	chaincode, err := contractapi.NewChaincode(new(GameContract))
	if err != nil {
		panic(err)
	}
	if err := chaincode.Start(); err != nil {
		panic(err)
	}
}

