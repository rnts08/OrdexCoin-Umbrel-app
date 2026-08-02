package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web
var webFS embed.FS

func main() {
	cfg := parseConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rpc := NewRPCClient(cfg.RPCURL, cfg.RPCUser, cfg.RPCPass, cfg.RPCTimeout, logger)
	if cfg.Wallet != "" {
		rpc.SetWallet(cfg.Wallet)
	} else {
		autoSelectWallet(rpc, logger)
	}

	authEnabled := cfg.UIUser != "" || cfg.UIPass != ""
	srv := NewServer(rpc, logger, authEnabled)

	mux := http.NewServeMux()
	registerRoutes(mux, srv)

	staticRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		logger.Error("failed to load embedded static assets", "error", err)
		os.Exit(1)
	}
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))

	handler := BasicAuth(mux, cfg.UIUser, cfg.UIPass)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("OrdexCoin Web UI starting",
		"version", version,
		"listen", cfg.ListenAddr,
		"rpc", cfg.RPCURL,
		"wallet", rpc.Wallet(),
		"auth", authEnabled,
	)

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("web server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func registerRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/wallet", s.handleWallet)
	mux.HandleFunc("GET /api/balances", s.handleBalances)
	mux.HandleFunc("GET /api/transactions", s.handleTransactions)
	mux.HandleFunc("GET /api/addresses", s.handleAddresses)
	mux.HandleFunc("POST /api/addresses/new", s.handleNewAddress)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("GET /api/fee-estimate", s.handleFeeEstimate)
	mux.HandleFunc("GET /api/pool", s.handlePool)
	mux.HandleFunc("POST /api/console", s.handleConsole)
	mux.HandleFunc("POST /api/tip", s.handleTip)
	mux.HandleFunc("GET /api/wallets", s.handleWallets)
	mux.HandleFunc("POST /api/wallet", s.handleSetWallet)
	mux.HandleFunc("POST /api/wallets/create", s.handleCreateWallet)
}

// autoSelectWallet picks a sensible default wallet when none was configured:
// the first loaded wallet, if any.
func autoSelectWallet(rpc *RPCClient, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, err := rpc.CallNode(ctx, "listwallets")
	if err != nil {
		logger.Warn("could not auto-select a wallet (daemon unreachable?)", "error", err)
		return
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		logger.Warn("unexpected listwallets response", "error", err)
		return
	}
	if len(names) == 0 {
		logger.Info("no wallets loaded; wallet features will be unavailable until one is loaded")
		return
	}
	rpc.SetWallet(names[0])
	logger.Info("auto-selected wallet", "wallet", names[0])
}
