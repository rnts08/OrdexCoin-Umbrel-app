# OrdexCoin Web — Development

Development guide for the OrdexCoin web UI. This covers building and running the
webapp on its own — including standalone against an already-running daemon. For
the Umbrel packaging (how to run OrdexCoin as an umbrelOS app), see
**[README.md](./README.md)** and **`../docs/UMBREL.md`**.

## Overview

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

## Run (binary, local daemon)

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

## Run standalone against an already-running daemon

If you already have an `ordexcoind` (or `ordexcoin-qt`) running on your host,
you can run the web UI by itself, next to it. There are two ways to supply the
RPC credentials.

### Option A — datadir access (auto cookie)

The webapp reads the daemon's `.cookie` (or `ordexcoin.conf`) from the data
directory, so no passwords need to be typed. The daemon must accept connections
on `127.0.0.1:<rpcport>`.

```sh
# from the repository root
docker run --rm -d --name ordexcoin-web \
  --network host \
  -v "$HOME/.ordexcoin:/data:ro" \
  --entrypoint /usr/local/bin/ordexcoin-web \
  ordexcoin-web:local \
  -datadir /data -listen 0.0.0.0:3000
```

- `--network host` makes the container's `127.0.0.1` the host loopback, so the
  default RPC URL `http://127.0.0.1:25173` reaches your daemon.
- `-v "$HOME/.ordexcoin:/data:ro"` gives read-only access to the cookie and the
  wallet list; `-datadir /data` points the webapp at it.
- `--entrypoint /usr/local/bin/ordexcoin-web` skips the single-container
  entrypoint (which would start a second daemon). The webapp runs standalone.
- Open `http://localhost:3000`.

### Option B — RPC only (no datadir access)

The webapp only needs a reachable RPC endpoint and credentials. This works from
any container network (or a remote daemon) without mounting the data directory.
You need the daemon's RPC user/password — either a `rpcuser`/`rpcpassword` pair
you configured, or the current cookie value in `~/.ordexcoin/.cookie`
(`user:pass`, can be extracted with `$(cat ~/.ordexcoin/.cookie)`).

```sh
# host networking + explicit cookie credentials, no volume mount
docker run --rm -d --name ordexcoin-web \
  --network host \
  --entrypoint /usr/local/bin/ordexcoin-web \
  -e ORDEXCOIN_RPC_USER="$(cut -d: -f1 ~/.ordexcoin/.cookie)" \
  -e ORDEXCOIN_RPC_PASS="$(cut -d: -f2 ~/.ordexcoin/.cookie)" \
  ordexcoin-web:local \
  -listen 0.0.0.0:3000
```

Or point at a daemon reachable on the Docker bridge / a remote host:

```sh
docker run --rm -d --name ordexcoin-web \
  -p 3000:3000 \
  --entrypoint /usr/local/bin/ordexcoin-web \
  -e ORDEXCOIN_RPC_URL="http://<host-or-container-ip>:25173" \
  -e ORDEXCOIN_RPC_USER="<rpcuser>" \
  -e ORDEXCOIN_RPC_PASS="<rpcpass>" \
  ordexcoin-web:local
```

Notes for Option B:

- Without `--network host`, `127.0.0.1` inside the container is the container
  itself — use the host's IP / the daemon's IP instead, and make sure the daemon
  is bound so it accepts that connection (`rpcbind`/`rpcallowip`).
- RPC-only mode needs `getwalletinfo`/`listunspent` etc., so the daemon must
  have the wallet enabled (`-server=1` and a loaded wallet) for the wallet
  features to work; node-wide RPC (status, console) works regardless.
- The web UI itself is unauthenticated by default; add `-ui-user`/`-ui-pass`
  when exposing it beyond a trusted network.

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
