package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strings"

    "playsmith/internal/sdk"
)

type Server struct {
	Fabric sdk.Client
}

type MintReq struct {
	TokenID string `json:"tokenId"`
	To      string `json:"to"`
	URI     string `json:"uri"`
	Amount  uint64 `json:"amount"`
}

type TransferReq struct {
	TokenID string `json:"tokenId"`
	From    string `json:"from"`
	To      string `json:"to"`
	Amount  uint64 `json:"amount"`
}

type ListReq struct {
	TokenID string `json:"tokenId"`
	Seller  string `json:"seller"`
	Amount  uint64 `json:"amount"`
	Price   uint64 `json:"price"`
}

type BuyReq struct {
	TokenID   string `json:"tokenId"`
	Seller    string `json:"seller"`
	ListingID string `json:"listingId"`
	Buyer     string `json:"buyer"`
	Amount    uint64 `json:"amount"`
}

func (s *Server) mint(w http.ResponseWriter, r *http.Request) {
	var req MintReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
	res, err := s.Fabric.Submit("playsmith-assets", "Mint", req.TokenID, req.To, req.URI, itoa(req.Amount))
	writeJSON(w, res, err)
}

func (s *Server) transfer(w http.ResponseWriter, r *http.Request) {
	var req TransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
	res, err := s.Fabric.Submit("playsmith-assets", "Transfer", req.TokenID, req.From, req.To, itoa(req.Amount))
	writeJSON(w, res, err)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	var req ListReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
	res, err := s.Fabric.Submit("playsmith-assets", "ListForSale", req.TokenID, req.Seller, itoa(req.Amount), itoa(req.Price))
	writeJSON(w, res, err)
}

func (s *Server) buy(w http.ResponseWriter, r *http.Request) {
	var req BuyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
	res, err := s.Fabric.Submit("playsmith-assets", "Buy", req.TokenID, req.Seller, req.ListingID, req.Buyer, itoa(req.Amount))
	writeJSON(w, res, err)
}

func writeJSON(w http.ResponseWriter, res string, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil { w.WriteHeader(500); _ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()}); return }
	_ = json.NewEncoder(w).Encode(map[string]any{"result": res})
}

func itoa(v uint64) string { return fmt.Sprintf("%d", v) }

func main() {
	// minimal net/http with patterns (Go 1.22+ PathValue)
	s := &Server{Fabric: &sdk.NoopClient{}}
	http.HandleFunc("/api/v1/assets/mint", s.mint)
	http.HandleFunc("/api/v1/assets/transfer", s.transfer)
	http.HandleFunc("/api/v1/market/list", s.list)
	http.HandleFunc("/api/v1/market/buy", s.buy)
	http.HandleFunc("/api/v1/assets/", func(w http.ResponseWriter, r *http.Request) {
		// expect /api/v1/assets/{tokenId}/balance/{owner}
		if r.Method != http.MethodGet { http.NotFound(w, r); return }
		// crude parse
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/"), "/")
		if len(parts) == 4 && parts[1] == "balance" {
			// /{tokenId}/balance/{owner}
			r = r.WithContext(r.Context())
			// create a fake PathValue handler by reusing write directly
			tokenID := parts[0]
			owner := parts[2]
			res, err := s.Fabric.Evaluate("playsmith-assets", "BalanceOf", tokenID, owner)
			writeJSON(w, res, err)
			return
		}
		http.NotFound(w, r)
	})

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" { addr = ":" + v }
	log.Printf("API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
