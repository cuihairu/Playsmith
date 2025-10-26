package main

import (
    "crypto/sha256"
    "encoding/hex"
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
        // emit URI event on first set
        _ = emitEvent(ctx, "URI", map[string]any{"id": tokenID, "uri": uri})
    }
    // increase total supply
    if err := addSupply(ctx, tokenID, amount); err != nil { return err }
    // emit TransferSingle from zero address
    _ = emitEvent(ctx, "TransferSingle", map[string]any{"operator": invokerID(ctx), "from": "0x0", "to": to, "id": tokenID, "value": amount})
    return nil
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
    _ = emitEvent(ctx, "TransferSingle", map[string]any{"operator": invokerID(ctx), "from": from, "to": to, "id": tokenID, "value": amount})
    return nil
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
    _ = emitEvent(ctx, "AskCreated", map[string]any{"id": tokenID, "seller": seller, "listingId": listingID, "amount": amount, "price": price})
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
    _ = emitEvent(ctx, "Trade", map[string]any{"id": tokenID, "seller": seller, "buyer": buyer, "listingId": listingID, "amount": amount})
    return nil
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

// BatchMint accepts JSON arrays for ids, tos, amounts, uris
func (c *GameContract) BatchMint(ctx contractapi.TransactionContextInterface, idsJSON, tosJSON, amountsJSON, urisJSON string) error {
    ids, err := parseStrArray(idsJSON); if err != nil { return err }
    tos, err := parseStrArray(tosJSON); if err != nil { return err }
    amts, err := parseUintArray(amountsJSON); if err != nil { return err }
    uris, err := parseStrArray(urisJSON); if err != nil { return err }
    if len(ids) != len(tos) || len(ids) != len(amts) || len(ids) != len(uris) { return errors.New("length mismatch") }
    for i := range ids {
        if err := c.Mint(ctx, ids[i], tos[i], uris[i], amts[i]); err != nil { return err }
    }
    _ = emitEvent(ctx, "TransferBatch", map[string]any{"operator": invokerID(ctx), "from": "0x0", "to": "batch", "ids": ids, "values": amts})
    return nil
}

// BatchTransfer accepts JSON arrays for ids and amounts
func (c *GameContract) BatchTransfer(ctx contractapi.TransactionContextInterface, idsJSON, from, to, amountsJSON string) error {
    ids, err := parseStrArray(idsJSON); if err != nil { return err }
    amts, err := parseUintArray(amountsJSON); if err != nil { return err }
    if len(ids) != len(amts) { return errors.New("length mismatch") }
    for i := range ids {
        if err := c.Transfer(ctx, ids[i], from, to, amts[i]); err != nil { return err }
    }
    _ = emitEvent(ctx, "TransferBatch", map[string]any{"operator": invokerID(ctx), "from": from, "to": to, "ids": ids, "values": amts})
    return nil
}

// SetApprovalForAll sets operator approval for owner
func (c *GameContract) SetApprovalForAll(ctx contractapi.TransactionContextInterface, owner, operator, approved string) error {
    if owner == "" || operator == "" { return errors.New("invalid args") }
    key, _ := ctx.GetStub().CreateCompositeKey("op", []string{owner, operator})
    v := "0"
    if approved == "true" || approved == "1" { v = "1" }
    if err := ctx.GetStub().PutState(key, []byte(v)); err != nil { return err }
    _ = emitEvent(ctx, "ApprovalForAll", map[string]any{"owner": owner, "operator": operator, "approved": (v == "1")})
    return nil
}

// IsApprovedForAll returns operator approval
func (c *GameContract) IsApprovedForAll(ctx contractapi.TransactionContextInterface, owner, operator string) (bool, error) {
    key, _ := ctx.GetStub().CreateCompositeKey("op", []string{owner, operator})
    b, err := ctx.GetStub().GetState(key)
    if err != nil { return false, err }
    return string(b) == "1", nil
}

// URI returns token metadata URI
func (c *GameContract) URI(ctx contractapi.TransactionContextInterface, tokenID string) (string, error) {
    metaKey, _ := ctx.GetStub().CreateCompositeKey("meta", []string{tokenID})
    b, err := ctx.GetStub().GetState(metaKey)
    if err != nil { return "", err }
    if len(b) == 0 { return "", nil }
    var m Meta
    if err := json.Unmarshal(b, &m); err != nil { return "", err }
    return m.URI, nil
}

// TotalSupply returns total minted minus burned (burn not implemented yet)
func (c *GameContract) TotalSupply(ctx contractapi.TransactionContextInterface, tokenID string) (uint64, error) {
    k, _ := ctx.GetStub().CreateCompositeKey("supply", []string{tokenID})
    b, err := ctx.GetStub().GetState(k)
    if err != nil { return 0, err }
    if len(b) == 0 { return 0, nil }
    v, err := strconv.ParseUint(string(b), 10, 64); if err != nil { return 0, err }
    return v, nil
}

// Exists reports whether token meta exists
func (c *GameContract) Exists(ctx contractapi.TransactionContextInterface, tokenID string) (bool, error) {
    metaKey, _ := ctx.GetStub().CreateCompositeKey("meta", []string{tokenID})
    b, err := ctx.GetStub().GetState(metaKey)
    if err != nil { return false, err }
    return len(b) > 0, nil
}

// ListForSalePrivate stores price in private data, public state has amount and priceHash
func (c *GameContract) ListForSalePrivate(ctx contractapi.TransactionContextInterface, tokenID, seller string, amount uint64) (string, error) {
    if tokenID == "" || seller == "" || amount == 0 { return "", errors.New("invalid args") }
    // expect transient key "price" with bytes of price as decimal string
    tm, err := ctx.GetStub().GetTransient()
    if err != nil { return "", err }
    pb, ok := tm["price"]; if !ok || len(pb) == 0 { return "", errors.New("missing transient price") }
    // compute hash
    sum := sha256.Sum256(pb)
    priceHash := hex.EncodeToString(sum[:])
    listingID := ctx.GetStub().GetTxID()
    askKey, _ := ctx.GetStub().CreateCompositeKey("ask", []string{tokenID, seller, listingID})
    // store public with amount and priceHash
    pv := map[string]any{"amount": amount, "priceHash": priceHash}
    pvb, _ := json.Marshal(pv)
    if err := ctx.GetStub().PutState(askKey, pvb); err != nil { return "", err }
    // store private price
    pvtKey, _ := ctx.GetStub().CreateCompositeKey("askpvt", []string{tokenID, seller, listingID})
    pdoc := map[string]any{"price": string(pb)}
    pdocb, _ := json.Marshal(pdoc)
    if err := ctx.GetStub().PutPrivateData("playsmithPvt", pvtKey, pdocb); err != nil { return "", err }
    _ = emitEvent(ctx, "AskCreated", map[string]any{"id": tokenID, "seller": seller, "listingId": listingID, "amount": amount, "price": 0, "priceHash": priceHash})
    return listingID, nil
}

// CancelListing removes a listing
func (c *GameContract) CancelListing(ctx contractapi.TransactionContextInterface, tokenID, seller, listingID string) error {
    askKey, _ := ctx.GetStub().CreateCompositeKey("ask", []string{tokenID, seller, listingID})
    if err := ctx.GetStub().DelState(askKey); err != nil { return err }
    // best-effort delete private
    pvtKey, _ := ctx.GetStub().CreateCompositeKey("askpvt", []string{tokenID, seller, listingID})
    _ = ctx.GetStub().DelPrivateData("playsmithPvt", pvtKey)
    _ = emitEvent(ctx, "AskCanceled", map[string]any{"id": tokenID, "seller": seller, "listingId": listingID})
    return nil
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

func addSupply(ctx contractapi.TransactionContextInterface, tokenID string, amt uint64) error {
    k, _ := ctx.GetStub().CreateCompositeKey("supply", []string{tokenID})
    cur, err := ctx.GetStub().GetState(k)
    if err != nil { return err }
    var n uint64
    if len(cur) > 0 {
        v, err := strconv.ParseUint(string(cur), 10, 64)
        if err != nil { return err }
        n = v
    }
    return ctx.GetStub().PutState(k, []byte(strconv.FormatUint(n+amt, 10)))
}

func emitEvent(ctx contractapi.TransactionContextInterface, name string, payload any) error {
    b, _ := json.Marshal(payload)
    return ctx.GetStub().SetEvent(name, b)
}

func invokerID(ctx contractapi.TransactionContextInterface) string {
    // best-effort invoker id (MSPID not included)
    cid, _ := ctx.GetClientIdentity().GetID()
    return cid
}

func parseStrArray(s string) ([]string, error) {
    var a []string
    if err := json.Unmarshal([]byte(s), &a); err != nil { return nil, errors.New("invalid string array json") }
    return a, nil
}

func parseUintArray(s string) ([]uint64, error) {
    var a []uint64
    if err := json.Unmarshal([]byte(s), &a); err != nil { return nil, errors.New("invalid uint array json") }
    return a, nil
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
