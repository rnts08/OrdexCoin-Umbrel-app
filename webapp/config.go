package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultRPCURL  = "http://127.0.0.1:25173"
	defaultDatadir = "~/.ordexcoin"
	defaultListen  = "0.0.0.0:3000"
	defaultTimeout = 60 * time.Second
)

// Config holds all runtime settings. Precedence: flags > environment > defaults.
type Config struct {
	RPCURL     string
	Datadir    string
	RPCUser    string
	RPCPass    string
	ListenAddr string
	UIUser     string
	UIPass     string
	Wallet     string
	RPCTimeout time.Duration
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "."
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~") {
		return filepath.Join(homeDir(), strings.TrimPrefix(p, "~"))
	}
	return p
}

func env(key string) string { return os.Getenv(key) }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseConfig resolves configuration from flags, environment variables and defaults,
// then attempts to discover RPC credentials from the data directory.
func parseConfig() Config {
	var cfg Config

	flag.StringVar(&cfg.RPCURL, "rpc-url", "", "OrdexCoin JSON-RPC URL (default http://127.0.0.1:25173)")
	flag.StringVar(&cfg.Datadir, "datadir", "", "OrdexCoin data directory (default ~/.ordexcoin); used to auto-detect .cookie and ordexcoin.conf")
	flag.StringVar(&cfg.RPCUser, "rpcuser", "", "RPC username (overrides cookie-based auth)")
	flag.StringVar(&cfg.RPCPass, "rpcpass", "", "RPC password (overrides cookie-based auth)")
	flag.StringVar(&cfg.ListenAddr, "listen", "", "Web UI listen address (default 0.0.0.0:3000)")
	flag.StringVar(&cfg.UIUser, "ui-user", "", "Optional web UI basic-auth username")
	flag.StringVar(&cfg.UIPass, "ui-pass", "", "Optional web UI basic-auth password")
	flag.StringVar(&cfg.Wallet, "wallet", "", "Fixed wallet to use (default: auto-select the first loaded wallet)")
	flag.DurationVar(&cfg.RPCTimeout, "rpc-timeout", 0, "RPC request timeout (default 60s)")
	flag.Parse()

	cfg.RPCURL = firstNonEmpty(cfg.RPCURL, env("ORDEXCOIN_RPC_URL"), defaultRPCURL)
	cfg.Datadir = expandHome(firstNonEmpty(cfg.Datadir, env("ORDEXCOIN_DATA"), defaultDatadir))
	cfg.RPCUser = firstNonEmpty(cfg.RPCUser, env("ORDEXCOIN_RPC_USER"))
	cfg.RPCPass = firstNonEmpty(cfg.RPCPass, env("ORDEXCOIN_RPC_PASS"))
	cfg.ListenAddr = firstNonEmpty(cfg.ListenAddr, env("ORDEXCOIN_WEB_ADDR"), defaultListen)
	cfg.UIUser = firstNonEmpty(cfg.UIUser, env("ORDEXCOIN_WEB_USER"))
	cfg.UIPass = firstNonEmpty(cfg.UIPass, env("ORDEXCOIN_WEB_PASS"))
	cfg.Wallet = firstNonEmpty(cfg.Wallet, env("ORDEXCOIN_WALLET"))
	if cfg.RPCTimeout == 0 {
		cfg.RPCTimeout = defaultTimeout
	}

	// RPC auth fallback: explicit user/pass, then .cookie, then ordexcoin.conf.
	if cfg.RPCUser == "" && cfg.RPCPass == "" {
		if u, p, ok := readCookie(filepath.Join(cfg.Datadir, ".cookie")); ok {
			cfg.RPCUser, cfg.RPCPass = u, p
		} else if conf, ok := readConfFile(filepath.Join(cfg.Datadir, "ordexcoin.conf")); ok {
			cfg.RPCUser = conf["rpcuser"]
			cfg.RPCPass = conf["rpcpassword"]
			if cfg.RPCURL == defaultRPCURL && conf["rpcport"] != "" {
				cfg.RPCURL = "http://127.0.0.1:" + conf["rpcport"]
			}
		}
	}
	return cfg
}

// readCookie reads the RPC cookie file (datadir/.cookie), which contains "<user>:<pass>".
func readCookie(path string) (user, pass string, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// readConfFile minimally parses an ordexcoin.conf into key/value pairs (no includes, no quoting).
func readConfFile(path string) (map[string]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	kv := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(strings.Trim(val, `"'`))
		if key == "" {
			continue
		}
		kv[strings.ToLower(key)] = val
	}
	if len(kv) == 0 {
		return nil, false
	}
	return kv, true
}

func (c Config) String() string {
	return fmt.Sprintf("listen=%s rpc=%s wallet=%q auth=%v", c.ListenAddr, c.RPCURL, c.Wallet, c.UIUser != "" || c.UIPass != "")
}
