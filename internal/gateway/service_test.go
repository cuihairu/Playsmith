package gateway

import (
	"context"
	"testing"

	pb "playsmith/internal/gatewaypb"
)

type fake struct {
	last string
	args []string
	txid string
}

func (f *fake) Submit(contract, txn string, args ...string) (string, string, error) {
	f.last = txn
	f.args = args
	return "TX_OK", f.txid, nil
}
func (f *fake) SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, string, error) {
	f.last = txn
	f.args = args
	return "TX_OK", f.txid, nil
}
func (f *fake) Evaluate(contract, txn string, args ...string) (string, error) {
	f.last = txn
	f.args = args
	if txn == "Exists" {
		return "true", nil
	}
	return "7", nil
}

func TestMint(t *testing.T) {
	f := &fake{txid: "TXID-1"}
	s := &Service{Fabric: f}
	res, err := s.Mint(context.Background(), &pb.MintRequest{TokenId: "t1", To: "o1", Uri: "u", Amount: 5})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != "TX_OK" || f.last != "Mint" {
		t.Fatalf("res=%v last=%s", res, f.last)
	}
	if res.TxId != "TXID-1" {
		t.Fatalf("tx_id = %q, want TXID-1", res.TxId)
	}
	if len(f.args) != 4 || f.args[0] != "t1" || f.args[1] != "o1" || f.args[2] != "u" || f.args[3] != "5" {
		t.Fatalf("args=%v", f.args)
	}
}

func TestBuy(t *testing.T) {
	f := &fake{}
	s := &Service{Fabric: f}
	_, err := s.Buy(context.Background(), &pb.BuyRequest{TokenId: "t", Seller: "s", ListingId: "l", Buyer: "b", Amount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if f.last != "Buy" {
		t.Fatalf("last=%s", f.last)
	}
}

func TestExistsTrue(t *testing.T) {
	f := &fake{}
	s := &Service{Fabric: f}
	res, err := s.Exists(context.Background(), &pb.ExistsRequest{TokenId: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Value {
		t.Fatal("want true")
	}
}

func TestBatchMintJSON(t *testing.T) {
	f := &fake{}
	s := &Service{Fabric: f}
	_, err := s.BatchMint(context.Background(), &pb.BatchMintRequest{Ids: []string{"a", "b"}, Tos: []string{"x"}, Amounts: []uint64{1, 2}, Uris: []string{"u1", "u2"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.args[0] != `["a","b"]` || f.args[1] != `["x"]` || f.args[2] != `[1,2]` {
		t.Fatalf("args=%v", f.args)
	}
}

func TestCancelListingCarriesTxID(t *testing.T) {
	f := &fake{txid: "TXID-CANCEL"}
	s := &Service{Fabric: f}
	res, err := s.CancelListing(context.Background(), &pb.CancelListingRequest{TokenId: "t", Seller: "s", ListingId: "l"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TxId != "TXID-CANCEL" {
		t.Fatalf("tx_id = %q, want TXID-CANCEL", res.TxId)
	}
}
