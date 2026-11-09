package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"playsmith/internal/auth"
	"playsmith/internal/gateway"
	pb "playsmith/internal/gatewaypb/gen"
	"playsmith/internal/sdk"
)

type Server struct {
	Fabric      sdk.Client
	JWTManager  *auth.Manager
	issueSecret []byte
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

type TokenReq struct {
	Subject string `json:"subject"`
	MSPID   string `json:"mspId"`
	Role    string `json:"role"`
}

func (s *Server) mint(w http.ResponseWriter, r *http.Request) {
	var req MintReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res, txID, err := s.Fabric.Submit("playsmith-assets", "Mint", req.TokenID, req.To, req.URI, itoa(req.Amount))
	writeJSON(w, res, txID, err)
}

func (s *Server) transfer(w http.ResponseWriter, r *http.Request) {
	var req TransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res, txID, err := s.Fabric.Submit("playsmith-assets", "Transfer", req.TokenID, req.From, req.To, itoa(req.Amount))
	writeJSON(w, res, txID, err)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	var req ListReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res, txID, err := s.Fabric.Submit("playsmith-assets", "ListForSale", req.TokenID, req.Seller, itoa(req.Amount), itoa(req.Price))
	writeJSON(w, res, txID, err)
}

func (s *Server) buy(w http.ResponseWriter, r *http.Request) {
	var req BuyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res, txID, err := s.Fabric.Submit("playsmith-assets", "Buy", req.TokenID, req.Seller, req.ListingID, req.Buyer, itoa(req.Amount))
	writeJSON(w, res, txID, err)
}

func (s *Server) listPrivate(w http.ResponseWriter, r *http.Request) {
	var req ListReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	transient := map[string][]byte{"price": []byte(itoa(req.Price))}
	res, txID, err := s.Fabric.SubmitWithTransient("playsmith-assets", "ListForSalePrivate", transient, req.TokenID, req.Seller, itoa(req.Amount))
	writeJSON(w, res, txID, err)
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TokenID   string `json:"tokenId"`
		Seller    string `json:"seller"`
		ListingID string `json:"listingId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res, txID, err := s.Fabric.Submit("playsmith-assets", "CancelListing", body.TokenID, body.Seller, body.ListingID)
	writeJSON(w, res, txID, err)
}

func (s *Server) approveAll(w http.ResponseWriter, r *http.Request) {
	var req ApproveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	approved := "false"
	if req.Approved {
		approved = "true"
	}
	res, txID, err := s.Fabric.Submit("playsmith-assets", "SetApprovalForAll", req.Owner, req.Operator, approved)
	writeJSON(w, res, txID, err)
}

func (s *Server) uri(w http.ResponseWriter, r *http.Request) {
	tokenID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	tokenID = strings.TrimSuffix(tokenID, "/uri")
	res, err := s.Fabric.Evaluate("playsmith-assets", "URI", tokenID)
	writeJSON(w, res, "", err)
}

func (s *Server) exists(w http.ResponseWriter, r *http.Request) {
	tokenID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	tokenID = strings.TrimSuffix(tokenID, "/exists")
	res, err := s.Fabric.Evaluate("playsmith-assets", "Exists", tokenID)
	writeJSON(w, res, "", err)
}

func (s *Server) totalSupply(w http.ResponseWriter, r *http.Request) {
	tokenID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	tokenID = strings.TrimSuffix(tokenID, "/totalSupply")
	res, err := s.Fabric.Evaluate("playsmith-assets", "TotalSupply", tokenID)
	writeJSON(w, res, "", err)
}

func (s *Server) batchMint(w http.ResponseWriter, r *http.Request) {
	var req BatchMintReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ids, _ := json.Marshal(req.IDs)
	tos, _ := json.Marshal(req.Tos)
	amts, _ := json.Marshal(req.Amounts)
	uris, _ := json.Marshal(req.URIs)
	res, txID, err := s.Fabric.Submit("playsmith-assets", "BatchMint", string(ids), string(tos), string(amts), string(uris))
	writeJSON(w, res, txID, err)
}

func (s *Server) batchTransfer(w http.ResponseWriter, r *http.Request) {
	var req BatchTransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ids, _ := json.Marshal(req.IDs)
	amts, _ := json.Marshal(req.Amounts)
	res, txID, err := s.Fabric.Submit("playsmith-assets", "BatchTransfer", string(ids), req.From, req.To, string(amts))
	writeJSON(w, res, txID, err)
}

func writeJSON(w http.ResponseWriter, res, txID string, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": res, "txId": txID})
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
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	if os.Getenv("JWT_SECRET") != "" {
		manager, err := auth.NewManager(os.Getenv("JWT_SECRET"), "playsmith", "game-server", time.Hour)
		if err != nil {
			log.Fatalf("JWT auth: %v", err)
		}
		s.JWTManager = manager
		s.issueSecret = []byte(os.Getenv("JWT_SECRET"))
	}
	if v := os.Getenv("GRPC_PORT"); v != "" {
		if s.JWTManager == nil {
			log.Fatal("GRPC_PORT requires JWT_SECRET")
		}
		go func() {
			if err := serveGRPC(":"+v, s.Fabric, s.JWTManager); err != nil {
				log.Fatalf("gRPC server: %v", err)
			}
		}()
	}
	log.Printf("API listening on %s", addr)
	if certFile, keyFile := tlsPaths(); certFile != "" && keyFile != "" {
		log.Printf("TLS enabled with cert=%s", certFile)
		if err := http.ListenAndServeTLS(addr, certFile, keyFile, s.router()); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("TLS not configured; serving plaintext (dev mode)")
		if err := http.ListenAndServe(addr, s.router()); err != nil {
			log.Fatal(err)
		}
	}
}

func (s *Server) router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/token", s.issueToken)
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
	return s.authMiddleware(mux)
}

func (s *Server) issueToken(w http.ResponseWriter, r *http.Request) {
	if s.JWTManager == nil {
		http.Error(w, "JWT auth not enabled", 500)
		return
	}
	if len(s.issueSecret) == 0 || subtle.ConstantTimeCompare(s.issueSecret, []byte(r.Header.Get("X-Issue-Secret"))) != 1 {
		http.Error(w, "invalid issue secret", http.StatusUnauthorized)
		return
	}
	var req TokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token, err := s.JWTManager.Issue(req.Subject, req.MSPID, req.Role)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, token, "", nil)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/token" || s.JWTManager == nil {
			next.ServeHTTP(w, r)
			return
		}
		authz := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(authz) <= len(prefix) || !strings.HasPrefix(authz, prefix) {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		if _, err := s.JWTManager.Validate(authz[len(prefix):]); err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveGRPC(addr string, fabric sdk.Client, manager *auth.Manager) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	opts := []grpc.ServerOption{grpc.ChainUnaryInterceptor(grpcAuthUnaryInterceptor(manager))}
	if certFile, keyFile := tlsPaths(); certFile != "" && keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			return err
		}
		opts = append(opts, grpc.Creds(creds))
	}
	server := grpc.NewServer(opts...)
	pb.RegisterGatewayServer(server, &gateway.Service{Fabric: fabric})
	log.Printf("gRPC listening on %s", addr)
	return server.Serve(listener)
}

func grpcAuthUnaryInterceptor(manager *auth.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get("authorization")
		const prefix = "Bearer "
		if len(vals) != 1 || len(vals[0]) <= len(prefix) || !strings.HasPrefix(vals[0], prefix) {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		claims, err := manager.Validate(strings.TrimPrefix(vals[0], prefix))
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		if claims.Role == "" || claims.MSPID == "" {
			return nil, status.Error(codes.PermissionDenied, "missing Fabric identity")
		}
		return handler(ctx, req)
	}
}

func (s *Server) assetRoute(w http.ResponseWriter, r *http.Request) {
	// expect /api/v1/assets/{tokenId}/balance/{owner}
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	// crude parse
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/"), "/")
	if len(parts) == 2 && parts[1] == "uri" {
		res, err := s.Fabric.Evaluate("playsmith-assets", "URI", parts[0])
		writeJSON(w, res, "", err)
		return
	}
	if len(parts) == 2 && parts[1] == "exists" {
		res, err := s.Fabric.Evaluate("playsmith-assets", "Exists", parts[0])
		writeJSON(w, res, "", err)
		return
	}
	if len(parts) == 2 && parts[1] == "totalSupply" {
		res, err := s.Fabric.Evaluate("playsmith-assets", "TotalSupply", parts[0])
		writeJSON(w, res, "", err)
		return
	}
	if len(parts) == 3 && parts[1] == "balance" {
		// /{tokenId}/balance/{owner} splits into 3 segments
		tokenID := parts[0]
		owner := parts[2]
		res, err := s.Fabric.Evaluate("playsmith-assets", "BalanceOf", tokenID, owner)
		writeJSON(w, res, "", err)
		return
	}
	http.NotFound(w, r)
}

// tlsPaths returns the TLS certificate and key paths from the environment.
// When either is empty, TLS is not configured and the server listens plaintext.
func tlsPaths() (string, string) {
	return os.Getenv("TLS_CERT_PATH"), os.Getenv("TLS_KEY_PATH")
}
