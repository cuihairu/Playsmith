package main

import (
    "context"
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

type ApproveReq struct {
    Owner    string `json:"owner"`
    Operator string `json:"operator"`
    Approved bool   `json:"approved"`
}

type BatchMintReq struct {
    IDs     []string `json:"ids"`
    Tos     []string `json:"tos"`
    Amounts []uint64 `json:"amounts"`
    URIs    []string `json:"uris"`
}

type BatchTransferReq struct {
    IDs     []string `json:"ids"`
    From    string   `json:"from"`
    To      string   `json:"to"`
    Amounts []uint64 `json:"amounts"`
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

func (s *Server) listPrivate(w http.ResponseWriter, r *http.Request) {
    var req ListReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
    transient := map[string][]byte{"price": []byte(itoa(req.Price))}
    res, err := s.Fabric.SubmitWithTransient("playsmith-assets", "ListForSalePrivate", transient, req.TokenID, req.Seller, itoa(req.Amount))
    writeJSON(w, res, err)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
    var body struct {
        TokenID   string `json:"tokenId"`
        Seller    string `json:"seller"`
        ListingID string `json:"listingId"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil { http.Error(w, err.Error(), 400); return }
    res, err := s.Fabric.Submit("playsmith-assets", "CancelListing", body.TokenID, body.Seller, body.ListingID)
    writeJSON(w, res, err)
}

func (s *Server) approveAll(w http.ResponseWriter, r *http.Request) {
    var req ApproveReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
    approved := "false"; if req.Approved { approved = "true" }
    res, err := s.Fabric.Submit("playsmith-assets", "SetApprovalForAll", req.Owner, req.Operator, approved)
    writeJSON(w, res, err)
}

func (s *Server) uri(w http.ResponseWriter, r *http.Request) {
    tokenID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
    tokenID = strings.TrimSuffix(tokenID, "/uri")
    res, err := s.Fabric.Evaluate("playsmith-assets", "URI", tokenID)
    writeJSON(w, res, err)
}

func (s *Server) exists(w http.ResponseWriter, r *http.Request) {
    tokenID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
    tokenID = strings.TrimSuffix(tokenID, "/exists")
    res, err := s.Fabric.Evaluate("playsmith-assets", "Exists", tokenID)
    writeJSON(w, res, err)
}

func (s *Server) totalSupply(w http.ResponseWriter, r *http.Request) {
    tokenID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
    tokenID = strings.TrimSuffix(tokenID, "/totalSupply")
    res, err := s.Fabric.Evaluate("playsmith-assets", "TotalSupply", tokenID)
    writeJSON(w, res, err)
}

func (s *Server) batchMint(w http.ResponseWriter, r *http.Request) {
    var req BatchMintReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
    ids, _ := json.Marshal(req.IDs)
    tos, _ := json.Marshal(req.Tos)
    amts, _ := json.Marshal(req.Amounts)
    uris, _ := json.Marshal(req.URIs)
    res, err := s.Fabric.Submit("playsmith-assets", "BatchMint", string(ids), string(tos), string(amts), string(uris))
    writeJSON(w, res, err)
}

func (s *Server) batchTransfer(w http.ResponseWriter, r *http.Request) {
    var req BatchTransferReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
    ids, _ := json.Marshal(req.IDs)
    amts, _ := json.Marshal(req.Amounts)
    res, err := s.Fabric.Submit("playsmith-assets", "BatchTransfer", string(ids), req.From, req.To, string(amts))
    writeJSON(w, res, err)
}

func writeJSON(w http.ResponseWriter, res string, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil { w.WriteHeader(500); _ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()}); return }
	_ = json.NewEncoder(w).Encode(map[string]any{"result": res})
}

func itoa(v uint64) string { return fmt.Sprintf("%d", v) }

func main() {
    s := &Server{}
    if v := os.Getenv("FABRIC_GATEWAY_ENDPOINT"); v != "" {
        gc, err := sdk.NewGatewayClient(context.Background(), sdk.Config{
            MSPID:          os.Getenv("FABRIC_MSP_ID"),
            CertPath:       os.Getenv("FABRIC_CERT_PATH"),
            KeyPath:        os.Getenv("FABRIC_KEY_PATH"),
            TLSCertPath:    os.Getenv("FABRIC_TLS_PATH"),
            GatewayAddress: v,
            ChannelName:    os.Getenv("FABRIC_CHANNEL_NAME"),
            ChaincodeName:  "playsmith-assets",
        })
        if err != nil {
            log.Fatalf("Fabric Gateway client: %v", err)
        }
        s.Fabric = gc
    } else {
        s.Fabric = &sdk.NoopClient{}
        log.Printf("FABRIC_GATEWAY_ENDPOINT unset; using NoopClient (dev mode)")
    }
    addr := ":8080"
    if v := os.Getenv("PORT"); v != "" { addr = ":" + v }
    log.Printf("API listening on %s", addr)
    http.ListenAndServe(addr, s.router())
}

func (s *Server) router() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/assets/mint", s.mint)
    mux.HandleFunc("/api/v1/assets/batchMint", s.batchMint)
    mux.HandleFunc("/api/v1/assets/transfer", s.transfer)
    mux.HandleFunc("/api/v1/assets/batchTransfer", s.batchTransfer)
    mux.HandleFunc("/api/v1/assets/approveAll", s.approveAll)
    mux.HandleFunc("/api/v1/market/list", s.list)
    mux.HandleFunc("/api/v1/market/list-private", s.listPrivate)
    mux.HandleFunc("/api/v1/market/cancel", s.cancel)
    mux.HandleFunc("/api/v1/market/buy", s.buy)
    mux.HandleFunc("/api/v1/assets/", s.assetRoute)
    return mux
}

func (s *Server) assetRoute(w http.ResponseWriter, r *http.Request) {
    // expect /api/v1/assets/{tokenId}/balance/{owner}
    if r.Method != http.MethodGet { http.NotFound(w, r); return }
    // crude parse
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/"), "/")
    if len(parts) == 2 && parts[1] == "uri" {
        res, err := s.Fabric.Evaluate("playsmith-assets", "URI", parts[0])
        writeJSON(w, res, err)
        return
    }
    if len(parts) == 2 && parts[1] == "exists" {
        res, err := s.Fabric.Evaluate("playsmith-assets", "Exists", parts[0])
        writeJSON(w, res, err)
        return
    }
    if len(parts) == 2 && parts[1] == "totalSupply" {
        res, err := s.Fabric.Evaluate("playsmith-assets", "TotalSupply", parts[0])
        writeJSON(w, res, err)
        return
    }
    if len(parts) == 3 && parts[1] == "balance" {
        // /{tokenId}/balance/{owner} splits into 3 segments
        tokenID := parts[0]
        owner := parts[2]
        res, err := s.Fabric.Evaluate("playsmith-assets", "BalanceOf", tokenID, owner)
        writeJSON(w, res, err)
        return
    }
    http.NotFound(w, r)
}
