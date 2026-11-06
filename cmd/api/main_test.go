package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"playsmith/internal/auth"
)

// fakeClient records calls.
type fakeClient struct {
	lastSubmit string
	lastArgs   []string
	lastEval   string
	evalArgs   []string
}

func (f *fakeClient) Submit(contract, txn string, args ...string) (string, string, error) {
	f.lastSubmit = txn
	f.lastArgs = args
	return "TX_OK", "TXID-1", nil
}
func (f *fakeClient) SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, string, error) {
	f.lastSubmit = txn
	f.lastArgs = args
	return "TX_OK", "TXID-1", nil
}
func (f *fakeClient) Evaluate(contract, txn string, args ...string) (string, error) {
	f.lastEval = txn
	f.evalArgs = args
	return "7", nil
}

func doRequest(s *Server, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	s.router().ServeHTTP(w, r)
	return w
}

func TestBalanceRouteParses(t *testing.T) {
	f := &fakeClient{}
	s := &Server{Fabric: f}
	w := doRequest(s, http.MethodGet, "/api/v1/assets/tok1/balance/owner1", "")
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if f.lastEval != "BalanceOf" {
		t.Fatalf("eval = %s, want BalanceOf", f.lastEval)
	}
	if len(f.evalArgs) != 2 || f.evalArgs[0] != "tok1" || f.evalArgs[1] != "owner1" {
		t.Fatalf("eval args = %v", f.evalArgs)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp["result"] != "7" {
		t.Fatalf("result = %v, want 7", resp["result"])
	}
}

func TestBalanceRouteWrongSegments404(t *testing.T) {
	f := &fakeClient{}
	s := &Server{Fabric: f}
	w := doRequest(s, http.MethodGet, "/api/v1/assets/tok1/balance", "")
	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestCancelUsesLowercaseJSON(t *testing.T) {
	f := &fakeClient{}
	s := &Server{Fabric: f}
	body := `{"tokenId":"t1","seller":"s1","listingId":"L1"}`
	w := doRequest(s, http.MethodPost, "/api/v1/market/cancel", body)
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if f.lastSubmit != "CancelListing" {
		t.Fatalf("submit = %s", f.lastSubmit)
	}
	if len(f.lastArgs) != 3 || f.lastArgs[0] != "t1" || f.lastArgs[1] != "s1" || f.lastArgs[2] != "L1" {
		t.Fatalf("args = %v", f.lastArgs)
	}
}

func TestIssueTokenEndpoint(t *testing.T) {
	manager, _ := auth.NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Hour)
	s := &Server{JWTManager: manager, issueSecret: []byte("0123456789abcdef0123456789abcdef")}
	body := `{"subject":"server-1","mspId":"Org1MSP","role":"MINTER"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", strings.NewReader(body))
	r.Header.Set("X-Issue-Secret", "0123456789abcdef0123456789abcdef")
	w := httptest.NewRecorder()
	s.router().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	token, _ := resp["result"].(string)
	if token == "" {
		t.Fatal("empty token")
	}
	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("validate issued token: %v", err)
	}
	if claims.Subject != "server-1" || claims.MSPID != "Org1MSP" || claims.Role != "MINTER" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestIssueTokenRequiresIssueSecret(t *testing.T) {
	manager, _ := auth.NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Hour)
	s := &Server{JWTManager: manager, issueSecret: []byte("0123456789abcdef0123456789abcdef")}
	w := doRequest(s, http.MethodPost, "/api/v1/auth/token", `{"subject":"s","mspId":"m","role":"r"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddlewareRequiresToken(t *testing.T) {
	manager, _ := auth.NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Hour)
	s := &Server{JWTManager: manager, Fabric: &fakeClient{}}
	w := doRequest(s, http.MethodGet, "/api/v1/assets/tok1/balance/owner1", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	manager, _ := auth.NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Hour)
	tok, _ := manager.Issue("server-1", "Org1MSP", "MINTER")
	s := &Server{JWTManager: manager, Fabric: &fakeClient{}}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/assets/tok1/balance/owner1", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	s.router().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestGRPCAuthInterceptor(t *testing.T) {
	manager, _ := auth.NewManager("0123456789abcdef0123456789abcdef", "playsmith", "game-server", time.Hour)
	tok, _ := manager.Issue("server-1", "Org1MSP", "MINTER")
	interceptor := grpcAuthUnaryInterceptor(manager)
	handler := func(ctx context.Context, req any) (any, error) { return "OK", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/playsmith.v1.Gateway/Mint"}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	res, err := interceptor(ctx, nil, info, handler)
	if err != nil || res != "OK" {
		t.Fatalf("valid token rejected: res=%v err=%v", res, err)
	}

	noAuth := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	if _, err := interceptor(noAuth, nil, info, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing auth: code=%v", status.Code(err))
	}

	bad := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad.token.here"))
	if _, err := interceptor(bad, nil, info, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("bad auth: code=%v", status.Code(err))
	}
}

func TestTLSPaths(t *testing.T) {
	t.Setenv("TLS_CERT_PATH", "/tmp/cert.pem")
	t.Setenv("TLS_KEY_PATH", "/tmp/key.pem")

	certFile, keyFile := tlsPaths()
	if certFile != "/tmp/cert.pem" || keyFile != "/tmp/key.pem" {
		t.Fatalf("cert=%q key=%q", certFile, keyFile)
	}
}
