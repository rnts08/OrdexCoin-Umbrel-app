package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCookie(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cookie")
	if err := os.WriteFile(path, []byte("__cookie__:secretpass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u, p, ok := readCookie(path)
	if !ok || u != "__cookie__" || p != "secretpass" {
		t.Fatalf("got %q %q %v", u, p, ok)
	}
}

func TestReadCookieMissing(t *testing.T) {
	if _, _, ok := readCookie(filepath.Join(t.TempDir(), ".cookie")); ok {
		t.Fatal("expected missing cookie to be reported as absent")
	}
}

func TestReadConfFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordexcoin.conf")
	content := `
# comment
server=1
rpcuser = alice
rpcpassword="s3cr#t"
rpcport=25173
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	kv, ok := readConfFile(path)
	if !ok {
		t.Fatal("expected conf file to parse")
	}
	if kv["rpcuser"] != "alice" || kv["rpcpassword"] != "s3cr#t" || kv["rpcport"] != "25173" {
		t.Fatalf("unexpected kv: %#v", kv)
	}
}

func TestParseConfigDefaults(t *testing.T) {
	for _, k := range []string{
		"ORDEXCOIN_RPC_URL", "ORDEXCOIN_DATA", "ORDEXCOIN_RPC_USER",
		"ORDEXCOIN_RPC_PASS", "ORDEXCOIN_WEB_ADDR", "ORDEXCOIN_WEB_USER",
		"ORDEXCOIN_WEB_PASS", "ORDEXCOIN_WALLET",
	} {
		os.Unsetenv(k)
	}
	cfg := parseConfig()
	if cfg.RPCURL != "http://127.0.0.1:25173" {
		t.Errorf("RPCURL = %q, want default", cfg.RPCURL)
	}
	if cfg.ListenAddr != "0.0.0.0:3000" {
		t.Errorf("ListenAddr = %q, want default", cfg.ListenAddr)
	}
}
