package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *RPCClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewRPCClient(srv.URL, "user", "pass", 5*time.Second, nil)
}

func TestRPCCallSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Basic dXNlcjpwYXNz"; got != want {
			t.Errorf("authorization header = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": 42, "error": null, "id": "x"}`))
	})

	raw, err := c.Call(context.Background(), "getblockcount")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil || n != 42 {
		t.Fatalf("result = %v (%s), want 42", n, err)
	}
}

func TestRPCCallWalletScope(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/wallet/mywallet") {
			t.Errorf("path = %q, want wallet-scoped path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": "ok", "error": null, "id": "x"}`))
	})
	c.SetWallet("mywallet")
	if _, err := c.Call(context.Background(), "getwalletinfo"); err != nil {
		t.Fatalf("Call: %v", err)
	}
}

func TestRPCCallErrorBodyOn500(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"result": null, "error": {"code": -5, "message": "Invalid address"}, "id": "x"}`))
	})

	_, err := c.Call(context.Background(), "sendtoaddress")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "-5") || !strings.Contains(err.Error(), "Invalid address") {
		t.Errorf("error = %q, want clean RPC error body", err.Error())
	}
}

func TestRPCCallConnectionRefused(t *testing.T) {
	c := NewRPCClient("http://127.0.0.1:1", "u", "p", time.Second, nil)
	if _, err := c.Call(context.Background(), "getblockcount"); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestConsoleCallRawError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": null, "error": {"code": -32601, "message": "Method not found"}, "id": "x"}`))
	})

	result, rpcErr, err := c.ConsoleCall(context.Background(), "", "nope", json.RawMessage("[]"))
	if err != nil {
		t.Fatalf("ConsoleCall err: %v", err)
	}
	if rpcErr == nil || rpcErr.Code != -32601 {
		t.Fatalf("rpcErr = %+v, want code -32601", rpcErr)
	}
	if result != nil {
		t.Fatalf("result = %s, want nil", result)
	}
}
