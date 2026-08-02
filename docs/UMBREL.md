# Packaging OrdexCoin Web for umbrelOS

This document captures everything needed to finish the Umbrel integration. It is
written so the work can be resumed in a fresh session without prior context.

## Status

The **webapp** and its **container image** are done and verified:

- `webapp/` — Go binary + embedded UI (single static binary, no external deps)
- `webapp/Dockerfile` — combined image running `ordexcoind` + `ordexcoin-web` in
  one container (exactly the shape Umbrel apps expect)
- `webapp/entrypoint.sh` — starts the daemon, waits for RPC, execs the web UI
- Verified: `docker build -f webapp/Dockerfile .` and the container serves the
  UI on port 3000 with the daemon's RPC reachable on 25173.

What remains is the **Umbrel app package** (manifest + compose + store
submission). The app package lives in `ordexcoin/`.

## Umbrel app model (reference)

An Umbrel app is a directory containing:

- `umbrel-app.yml` — manifest (id, name, version, port, category, description, …)
- `docker-compose.yml` — services; images are pinned **by digest** (multi-arch manifest)
- optional `exports.sh`, `hooks/`, `torrc.template`

The reference is the official Bitcoin Node app (`getumbrel/umbrel-apps/bitcoin`):
`bitcoind` runs as user `1000:1000`, data is persisted at `${APP_DATA_DIR}/data:/data`,
P2P/RPC ports are exported, and a small sidecar web UI is reached through
`app_proxy` (`APP_HOST: <app>_<svc>_1`, `APP_PORT: 3000`).

Store distribution: PR to `getumbrel/umbrel-apps` (heavy review). Quick testing:
a **community app store** (any git repo with `umbrel-app-store.yml`).

## Container model chosen

One image, one container, two processes:

```
ordexcoin:latest
 ├── /usr/local/bin/ordexcoind     (x86_64 static build, datadir /data)
 ├── /usr/local/bin/ordexcoin-cli
 ├── /usr/local/bin/ordexcoin-web  (Go web UI, listens :3000)
 └── /usr/local/bin/entrypoint.sh  (PID 1; starts daemon, then web UI)
```

Web UI → RPC on `127.0.0.1:25173` inside the container, cookie auth (daemon
writes `/data/.cookie`, webapp reads it automatically). No rpcuser/rpcpassword
required.

### Env vars the image understands

| Variable | Default | Purpose |
| --- | --- | --- |
| `ORDEXCOIN_DATA` | `/data` | daemon data directory (persist this) |
| `ORDEXCOIN_RPC_URL` | `http://127.0.0.1:25173` | RPC endpoint for the web UI |
| `ORDEXCOIN_WEB_ADDR` | `0.0.0.0:3000` | web UI listen address |
| `ORDEXCOIN_WEB_USER` | _(none)_ | optional web UI basic auth user |
| `ORDEXCOIN_WEB_PASS` | _(none)_ | optional web UI basic auth pass |

## Architecture target

**Target: umbrelOS on x86_64 (amd64).** umbrelOS is available for standard x86
systems and this is what we are building for. The `linux-static/bin` daemon
binaries are amd64 and the container image is therefore amd64 — a perfectly good
starting point for x86 umbrelOS devices.

**ARM64 (Raspberry Pi) — optional, later.** Umbrel also supports Raspberry Pi
(ARM64), but supporting it requires an aarch64 static build of the daemon.
That is a separate future task; it does not block the x86 target. When it is
done, publish a multi-arch manifest (`docker buildx --platform linux/amd64,linux/arm64`)
and update the pinned digest. The Go web UI is already architecture-independent
(`CGO_ENABLED=0`), so only the daemon binaries need the ARM64 pass.

### Networking for "becoming part of the network"

The mainnet P2P port is **25174** (RPC is 25173, bech32 HRP `oxc`). The daemon
ships with DNS seeds (`node3.walletbuilders.com`, `ordexcoinddns.xyz`), so
outbound peer discovery works with no config. For the node to also *accept*
inbound peers, the container must **publish port 25174** — this is what makes
an umbrelOS install a real part of the network. The entrypoint writes
`port=<ORDEXCOIN_P2P_PORT>` (default 25174) into the generated `ordexcoin.conf`.

## Finish steps (in order)

### 1. Publish the image (registry + digest)

```sh
docker buildx build --platform linux/amd64 -t <org>/ordexcoin:<ver> -f webapp/Dockerfile .
docker buildx build --platform linux/amd64,linux/arm64 -t <org>/ordexcoin:<ver> --push .   # once ARM64 exists
docker images --digests <org>/ordexcoin   # note the sha256:<digest> and paste into compose
```

### 2. Finalize `ordexcoin/umbrel-app.yml`

See `ordexcoin/umbrel-app.yml`. Fill in: `id` (suggest `ordexcoin`),
`category` (`bitcoin`), `name` ("OrdexCoin Node"), description, developer,
`port` (the app_proxy-exposed web port, e.g. `3000`), `releaseNotes`, gallery
(3–5 × 1440×900 screenshots), and a square `icon.svg`.

### 3. Finalize `ordexcoin/docker-compose.yml`

See `ordexcoin/docker-compose.yml`. Key points:
- `app_proxy` with `APP_HOST: ordexcoin_app_1`, `APP_PORT: 3000`
- main service pinned `image: <org>/ordexcoin:<ver>@sha256:<digest>`
- `restart: on-failure`, `stop_grace_period: 15m30s`. The container starts as
  root and `entrypoint.sh` repairs `${APP_DATA_DIR}/data` ownership then drops
  to `1000:1000` (the umbrelOS app uid/gid), so root-created/data mounts are
  writable and `ordexcoind` still runs as a non-root user.
- `volumes: ${APP_DATA_DIR}/data:/data`
- publish P2P port (e.g. `${APP_ORDEXCOIN_P2P_PORT}`) if the node should be reachable
- pass through web UI auth from `${APP_PASSWORD}`/`${APP_SEED}` if desired

### 4. Test on a device

Fastest: **community app store** (no review). Create a throwaway public repo:

```
umbrel-app-store.yml      # id: ordexcoin-test, name: OrdexCoin Test
ordexcoin/umbrel-app.yml
ordexcoin/docker-compose.yml
```

Then in the umbrelOS App Store → "Community App Store" → paste the repo URL.
Alternatively rsync `ordexcoin/` into the on-device app-store dir and install via
`umbreld client apps.install.mutate --appId ordexcoin`.

### 5. Submit to the official store

Fork `getumbrel/umbrel-apps`, add the package, open a PR with the review
checklist (screenshots, manifest fields, digest pinning, tested-on-device).

## Useful references

- App framework docs: `https://github.com/getumbrel/umbrel-apps#readme`
- Bitcoin app package (closest reference): `getumbrel/umbrel-apps/bitcoin`
- Community app store template: `getumbrel/umbrel-community-app-store`
- umbrelOS on x86: `https://github.com/getumbrel/umbrel/wiki/Install-umbrelOS-on-x86-Systems`
