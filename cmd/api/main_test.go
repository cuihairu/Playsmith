package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeClient records calls.
type fakeClient struct {
	lastSubmit string
	lastArgs   []string
	lastEval   string
	evalArgs   []string
}

func (f *fakeClient) Submit(contract, txn string, args ...string) (string, error) {
	f.lastSubmit = txn
	f.lastArgs = args
	return "TX_OK", nil
}
func (f *fakeClient) SubmitWithTransient(contract, txn string, transient map[string][]byte, args ...string) (string, error) {
	f.lastSubmit = txn
	f.lastArgs = args
	return "TX_OK", nil
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
