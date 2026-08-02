package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

// Server holds the API handlers and shared state.
type Server struct {
	rpc         *RPCClient
	logger      *slog.Logger
	authEnabled bool
}

func NewServer(rpc *RPCClient, logger *slog.Logger, authEnabled bool) *Server {
	return &Server{rpc: rpc, logger: logger, authEnabled: authEnabled}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// writeRPCErr converts a daemon RPC error into an HTTP response, exposing the
// daemon's own error message to the UI.
func writeRPCErr(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if strings.Contains(err.Error(), "could not reach") {
		status = http.StatusServiceUnavailable
	}
	writeErr(w, status, err.Error())
}

// handleInfo exposes webapp metadata to the frontend (version, auth state, current wallet, dev address).
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    version,
		"auth":       s.authEnabled,
		"wallet":     s.rpc.Wallet(),
		"devAddress": devOXCAddress,
		"support":    supportAddresses,
	})
}

// handleStatus aggregates node-level status RPCs.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	type named struct {
		name string
		raw  json.RawMessage
	}
	results := make(chan named, 3)
	errs := make(chan error, 3)

	fetch := func(name, method string, params ...any) {
		raw, err := s.rpc.CallNode(ctx, method, params...)
		if err != nil {
			errs <- fmt.Errorf("%s: %w", name, err)
			return
		}
		results <- named{name, raw}
	}

	go fetch("chain", "getblockchaininfo")
	go fetch("network", "getnetworkinfo")
	go fetch("mempool", "getmempoolinfo")

	out := map[string]json.RawMessage{}
	for i := 0; i < 3; i++ {
		select {
		case n := <-results:
			out[n.name] = n.raw
		case err := <-errs:
			writeRPCErr(w, err)
			return
		case <-ctx.Done():
			writeErr(w, http.StatusGatewayTimeout, "timed out querying daemon")
			return
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWallet returns wallet summary RPCs for the selected wallet.
func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	get := func(name, method string, params ...any) (json.RawMessage, error) {
		return s.rpc.Call(ctx, method, params...)
	}

	info, err := get("walletinfo", "getwalletinfo")
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	balances, err := get("balances", "getbalances")
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	total, err := get("total", "getbalance")
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"wallet":   s.rpc.Wallet(),
		"info":     json.RawMessage(info),
		"balances": json.RawMessage(balances),
		"total":    json.RawMessage(total),
	})
}

// handleBalances computes spendable totals from listunspent.
func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	raw, err := s.rpc.Call(ctx, "listunspent", 0, 9999999, []any{}, true)
	if err != nil {
		writeRPCErr(w, err)
		return
	}

	var utxos []struct {
		Amount        float64 `json:"amount"`
		Confirmations int     `json:"confirmations"`
		Spendable     bool    `json:"spendable"`
		Safe          bool    `json:"safe"`
		Coinbase      bool    `json:"coinbase"`
		Address       string  `json:"address"`
		Label         string  `json:"label"`
		TxID          string  `json:"txid"`
		Vout          int     `json:"vout"`
	}
	if err := json.Unmarshal(raw, &utxos); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not parse listunspent")
		return
	}

	var available, confirmed, unconfirmed, immature float64
	for _, u := range utxos {
		if !u.Spendable || !u.Safe {
			continue
		}
		available += u.Amount
		if u.Coinbase && u.Confirmations < 100 {
			immature += u.Amount
			continue
		}
		if u.Confirmations >= 1 {
			confirmed += u.Amount
		} else {
			unconfirmed += u.Amount
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"available":   available,
		"confirmed":   confirmed,
		"unconfirmed": unconfirmed,
		"immature":    immature,
		"utxoCount":   len(utxos),
	})
}

// handleTransactions returns the latest wallet transactions.
func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	raw, err := s.rpc.Call(ctx, "listtransactions", "*", limit, 0, true)
	if err != nil {
		writeRPCErr(w, err)
		return
	}

	// Return the raw array; the frontend renders it.
	writeJSON(w, http.StatusOK, json.RawMessage(raw))
}

// handleAddresses lists all wallet addresses grouped by label, with per-address balances.
func (s *Server) handleAddresses(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	labelsRaw, err := s.rpc.Call(ctx, "listlabels")
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	var labels []string
	if err := json.Unmarshal(labelsRaw, &labels); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not parse listlabels")
		return
	}

	type addrEntry struct {
		Label   string  `json:"label"`
		Address string  `json:"address"`
		Balance float64 `json:"balance"`
		Total   float64 `json:"total"`
		TxCount int     `json:"txCount"`
	}

	// Per-address received totals (include empty addresses so all are shown).
	receivedRaw, err := s.rpc.Call(ctx, "listreceivedbyaddress", 0, true, true)
	var received map[string]struct {
		Amount float64 `json:"amount"`
		TxIDs  []string `json:"txids"`
	}
	if err != nil {
		received = map[string]struct {
			Amount float64 `json:"amount"`
			TxIDs  []string `json:"txids"`
		}{}
	} else {
		var recvList []struct {
			Address string   `json:"address"`
			Amount  float64  `json:"amount"`
			TxIDs   []string `json:"txids"`
		}
		if err := json.Unmarshal(receivedRaw, &recvList); err == nil {
			received = make(map[string]struct {
				Amount float64 `json:"amount"`
				TxIDs  []string `json:"txids"`
			}, len(recvList))
			for _, e := range recvList {
				received[e.Address] = struct {
					Amount float64 `json:"amount"`
					TxIDs  []string `json:"txids"`
				}{e.Amount, e.TxIDs}
			}
		}
	}

	entries := []addrEntry{}
	for _, label := range labels {
		addrsRaw, err := s.rpc.Call(ctx, "getaddressesbylabel", label)
		if err != nil {
			continue
		}
		var addrs map[string]any
		if err := json.Unmarshal(addrsRaw, &addrs); err != nil {
			continue
		}
		for addr := range addrs {
			rec := received[addr]
			entries = append(entries, addrEntry{
				Label:   label,
				Address: addr,
				Balance: rec.Amount,
				Total:   rec.Amount,
				TxCount: len(rec.TxIDs),
			})
		}
	}

	writeJSON(w, http.StatusOK, entries)
}

type newAddressRequest struct {
	Label string `json:"label"`
}

// handleNewAddress creates a fresh receiving address via getnewaddress.
func (s *Server) handleNewAddress(w http.ResponseWriter, r *http.Request) {
	var req newAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Label = strings.TrimSpace(req.Label)

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	raw, err := s.rpc.Call(ctx, "getnewaddress", req.Label)
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	var address string
	if err := json.Unmarshal(raw, &address); err != nil {
		writeErr(w, http.StatusInternalServerError, "unexpected response from daemon")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"address": address,
		"label":   req.Label,
	})
}

type sendRequest struct {
	Address     string  `json:"address"`
	Amount      float64 `json:"amount"`
	Comment     string  `json:"comment"`
	CommentTo   string  `json:"commentTo"`
	SubtractFee bool    `json:"subtractFee"`
}

// handleSend sends OXC to an arbitrary address using sendtoaddress.
// The SubtractFee checkbox maps to subtractfeefromamount.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Address = strings.TrimSpace(req.Address)
	if req.Address == "" {
		writeErr(w, http.StatusBadRequest, "destination address is required")
		return
	}
	if req.Amount <= 0 {
		writeErr(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	raw, err := s.rpc.Call(ctx, "sendtoaddress",
		req.Address, req.Amount, req.Comment, req.CommentTo, req.SubtractFee)
	if err != nil {
		writeRPCErr(w, err)
		return
	}

	var txid string
	if err := json.Unmarshal(raw, &txid); err != nil {
		writeErr(w, http.StatusInternalServerError, "unexpected response from daemon")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"txid":        txid,
		"address":     req.Address,
		"amount":      req.Amount,
		"subtractFee": req.SubtractFee,
	})
}

// handleFeeEstimate returns a fee estimate (sat/vB) for the send form preview.
func (s *Server) handleFeeEstimate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	raw, err := s.rpc.CallNode(ctx, "estimatesmartfee", 6)
	if err != nil {
		// Fee estimation may be unavailable early on; that's not fatal.
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	var res struct {
		Feerate float64  `json:"feerate"`
		Blocks  int      `json:"blocks"`
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &res); err != nil || len(res.Errors) > 0 || res.Feerate <= 0 {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available":   true,
		"satPerVbyte": res.Feerate * 1e5,
		"blocks":      res.Blocks,
	})
}

type consoleRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// walletMethods are RPC methods that require wallet scope. Everything else is
// forwarded to the node-level endpoint. Add methods here as needed.
var walletMethods = map[string]bool{
	"getbalance": true, "getwalletinfo": true, "getnewaddress": true,
	"getaddressesbylabel": true, "getaddressinfo": true, "listunspent": true,
	"listtransactions": true, "sendtoaddress": true, "gettransaction": true,
	"listlabels": true, "getreceivedbyaddress": true, "getreceivedbylabel": true,
	"listreceivedbyaddress": true, "getunconfirmedbalance": true,
	"listaddressgroupings": true, "getbalances": true, "dumpwallet": true,
	"importprivkey": true, "importaddress": true, "importpubkey": true,
	"signmessage": true, "verifymessage": true, "abandontransaction": true,
	"bumpfee": true, 	"listlockunspent": true, "lockunspent": true,
	"walletcreatefundedpsbt": true, "send": true, "fundrawtransaction": true,
}

// handleConsole forwards arbitrary RPC calls to the daemon and returns both the
// result and error (if any) so the console view can display them as JSON.
func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	var req consoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Method = strings.TrimSpace(req.Method)
	if req.Method == "" {
		writeErr(w, http.StatusBadRequest, "method is required")
		return
	}
	if len(req.Params) == 0 || string(req.Params) == "null" {
		req.Params = json.RawMessage("[]")
	}

	wallet := ""
	if walletMethods[req.Method] {
		wallet = s.rpc.Wallet()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	result, rpcErr, err := s.rpc.ConsoleCall(ctx, wallet, req.Method, req.Params)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	out := map[string]any{"result": nil, "error": nil}
	if rpcErr != nil {
		out["error"] = map[string]any{"code": rpcErr.Code, "message": rpcErr.Message}
	} else {
		out["result"] = result
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWallets lists loaded wallets and the current selection.
func (s *Server) handleWallets(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	raw, err := s.rpc.CallNode(ctx, "listwallets")
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not parse listwallets")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"wallets": names,
		"current": s.rpc.Wallet(),
	})
}

type setWalletRequest struct {
	Name string `json:"name"`
}

// handleSetWallet switches the active wallet for wallet-scoped RPC calls.
func (s *Server) handleSetWallet(w http.ResponseWriter, r *http.Request) {
	var req setWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "wallet name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	raw, err := s.rpc.CallNode(ctx, "listwallets")
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not parse listwallets")
		return
	}
	found := false
	for _, n := range names {
		if n == req.Name {
			found = true
			break
		}
	}
	if !found {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("wallet %q is not loaded", req.Name))
		return
	}

	s.rpc.SetWallet(req.Name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "wallet": req.Name})
}

type createWalletRequest struct {
	Name string `json:"name"`
}

// handleCreateWallet creates a wallet via createwallet and selects it.
// With no name given, the default wallet ("") is created — the usual first-run
// action for a fresh node.
func (s *Server) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	var req createWalletRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	req.Name = strings.TrimSpace(req.Name)

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	raw, err := s.rpc.CallNode(ctx, "createwallet", req.Name)
	if err != nil {
		writeRPCErr(w, err)
		return
	}
	var res struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		writeErr(w, http.StatusInternalServerError, "unexpected response from daemon")
		return
	}

	s.rpc.SetWallet(res.Name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "wallet": res.Name})
}
