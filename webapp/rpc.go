package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// rpcError mirrors the JSON-RPC error object returned by the daemon.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// rpcResponse is the outer JSON-RPC response envelope.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	ID     json.RawMessage `json:"id"`
}

// RPCClient is a minimal JSON-RPC 1.0 client for OrdexCoin Core.
// It speaks to the node-level endpoint or, when a wallet is selected,
// to the wallet-scoped endpoint (/wallet/<name>).
type RPCClient struct {
	baseURL string
	user    string
	pass    string
	http    *http.Client
	logger  *slog.Logger

	mu     sync.RWMutex
	wallet string
}

func NewRPCClient(baseURL, user, pass string, timeout time.Duration, logger *slog.Logger) *RPCClient {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &RPCClient{
		baseURL: baseURL,
		user:    user,
		pass:    pass,
		http:    &http.Client{Timeout: timeout},
		logger:  logger,
	}
}

func (c *RPCClient) SetWallet(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wallet = name
}

func (c *RPCClient) Wallet() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.wallet
}

func (c *RPCClient) endpoint(wallet string) string {
	if wallet != "" {
		return c.baseURL + "wallet/" + url.PathEscape(wallet)
	}
	return c.baseURL
}

// Call invokes an RPC method on the currently selected wallet, or at node level
// when no wallet is selected. Params must be JSON-marshallable values.
func (c *RPCClient) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return c.call(ctx, c.Wallet(), method, params, false)
}

// CallNode invokes an RPC method at node level regardless of the selected wallet.
func (c *RPCClient) CallNode(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return c.call(ctx, "", method, params, false)
}

// ConsoleCall invokes an RPC method and returns the full response (result and/or error)
// so the console UI can render both cleanly. rawParams is passed through verbatim and
// may be a JSON array or object (named params).
func (c *RPCClient) ConsoleCall(ctx context.Context, wallet, method string, rawParams json.RawMessage) (json.RawMessage, *rpcError, error) {
	return c.callRaw(ctx, wallet, method, rawParams)
}

func (c *RPCClient) call(ctx context.Context, wallet, method string, params []any, _ bool) (json.RawMessage, error) {
	raw, rpcErr, err := c.callRaw(ctx, wallet, method, params)
	if err != nil {
		return nil, err
	}
	if rpcErr != nil {
		return nil, rpcErr
	}
	return raw, nil
}

func (c *RPCClient) callRaw(ctx context.Context, wallet, method string, params any) (json.RawMessage, *rpcError, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "1.0",
		"id":      "ordexcoin-web",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(wallet), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.user, c.pass)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("could not reach OrdexCoin RPC at %s: %w (is the daemon running?)", c.baseURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// The daemon reports RPC errors with HTTP 500 but still returns a JSON-RPC
		// envelope containing the error object; surface that cleanly.
		var rr rpcResponse
		if json.Unmarshal(data, &rr) == nil && rr.Error != nil {
			return nil, rr.Error, nil
		}
		return nil, nil, fmt.Errorf("RPC server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var rr rpcResponse
	if err := json.Unmarshal(data, &rr); err != nil {
		return nil, nil, fmt.Errorf("invalid RPC response: %w", err)
	}
	if rr.Error != nil {
		return nil, rr.Error, nil
	}
	return rr.Result, nil, nil
}
