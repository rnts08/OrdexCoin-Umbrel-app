# OrdexCoin Web

A lightweight, single-binary web interface for an OrdexCoin node — status,
balances, transactions, addresses, sending, an RPC console, and an About page
with one-click developer tips. Dark/gold theme from the OrdexCoin icon.

- **Go stdlib only** — no external dependencies, no database, no build system.
- Everything is embedded into one binary with `//go:embed`.
- Talks to `ordexcoind` over JSON-RPC, using the daemon's `.cookie` for auth.
- Optional HTTP basic auth for the web UI (`-ui-user` / `-ui-pass` or env).

## Features

- **Status** — chain, height, headers, difficulty, verification progress,
  mempool, connections, version, node warnings.
- **Balances** — available / confirmed / unconfirmed (pending) / immature
  (coinbase) totals derived from `listunspent`.
- **Transactions** — latest wallet transactions with type, date, address,
  amount, confirmations and TXID.
- **Addresses** — all wallet addresses grouped by label with per-address
  received totals, plus one-click generation of new receiving addresses.
- **Send** — send OXC to any address, with an optional comment and a
  **"Deduct network fee from the total amount"** checkbox
  (`subtractfeefromamount`), plus a network fee estimate.
- **Console** — run any OrdexCoin RPC command against the node or active wallet
  and see the JSON response (with history via up/down arrows).
- **About / support** — credits, licensing, the support addresses from the
  OrdexCoin README, and an **auto-tip** feature that sends OXC directly to the
  developer address from your wallet.

## Build

Requires Go 1.22+. No other dependencies.

```sh
cd webapp
go build -o ordexcoin-web .
```

Single static binary: `./ordexcoin-web`.

## Run

Point it at a running `ordexcoind`/`ordexcoin-qt` (RPC server on the default
port). Credentials are auto-detected from `~/.ordexcoin/.cookie`, or from
`ordexcoin.conf` (`rpcuser`/`rpcpassword`/`rpcport`), or supplied explicitly.

```sh
./ordexcoin-web                          # default: RPC 127.0.0.1:25173, UI 0.0.0.0:3000
./ordexcoin-web -listen 127.0.0.1:8080   # local-only
./ordexcoin-web -ui-user admin -ui-pass secret
./ordexcoin-web -rpc-url http://127.0.0.1:8332 -rpcuser u -rpcpass p
```

Open `http://localhost:3000`.

### Options (flag / env / default)

| Flag | Env | Default |
| --- | --- | --- |
| `-rpc-url` | `ORDEXCOIN_RPC_URL` | `http://127.0.0.1:25173` |
| `-datadir` | `ORDEXCOIN_DATA` | `~/.ordexcoin` (for `.cookie` / `ordexcoin.conf`) |
| `-rpcuser` / `-rpcpass` | `ORDEXCOIN_RPC_USER` / `ORDEXCOIN_RPC_PASS` | auto (cookie) |
| `-listen` | `ORDEXCOIN_WEB_ADDR` | `0.0.0.0:3000` |
| `-ui-user` / `-ui-pass` | `ORDEXCOIN_WEB_USER` / `ORDEXCOIN_WEB_PASS` | disabled |
| `-wallet` | `ORDEXCOIN_WALLET` | first loaded wallet |
| `-rpc-timeout` | — | `60s` |

## Docker (daemon + web in one container)

Build from the repository root (the image also bundles the static daemon binaries):

```sh
./build-docker-web.sh                 # -> ordexcoin-web:local
docker compose -f webapp/docker-compose.yml up --build
```

Or run directly:

```sh
docker run -p 3000:3000 -p 25174:25174 -v ~/.ordexcoin:/data ordexcoin-web:local
```

The container starts `ordexcoind` (writes `/data/ordexcoin.conf` and
`/data/.cookie` on first run), waits for RPC, then serves the web UI. It
publishes the P2P port **25174** so the node can accept inbound peers and
become part of the network (outbound peer discovery uses the built-in DNS
seeds). It is shaped for umbrelOS on x86_64 — see `../docs/UMBREL.md` for the
Umbrel packaging guide.

### Verify the packaged image end-to-end

```sh
./webapp/test-docker-web.sh
```

Builds the image, starts a fresh container, and runs 23 checks: static assets,
status, wallet creation, address generation, balances, transactions, console
(node + wallet scope + error passthrough), and send/tip validation (no real
transactions are broadcast). Prints a PASS/FAIL summary.

## Development

```sh
go vet ./...
go test ./...
```

The frontend is plain HTML/CSS/JS in `web/`, embedded at build time. Edit and
rebuild; no bundler.

## Security notes

- The web UI is unauthenticated by default (intended for a trusted LAN / the
  local container). Enable `-ui-user`/`-ui-pass` when exposing it beyond a
  trusted network.
- The RPC endpoint itself is never exposed by the webapp; it only ever talks to
  the daemon over localhost using the daemon's own credentials.
- Sending OXC and tipping broadcast **real transactions**; review the
  confirmation dialog carefully.

## License

MIT. Based on Bitcoin Core v25.0 (The Bitcoin Core developers). Modifications
and branding by OrdexNetwork developers. See the About page in-app.
