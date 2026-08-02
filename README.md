# OrdexCoin Node — umbrelOS app

Run a full **OrdexCoin** node on your umbrelOS device with the bundled web UI.

This repository is an Umbrel **community app store**. It contains the OrdexCoin
app package (`ordexcoin/umbrel-app.yml` + `docker-compose.yml`). The container
image is published and public on GHCR — no authentication is needed to pull it.

## What this is

| | |
| --- | --- |
| App ID | `ordexcoin` |
| Image | `ghcr.io/rnts08/ordexcoin:0.1.0` |
| Image digest | `sha256:e37b9f1575433003be774cc45faf5fd914c24b345904a6884dc1505cb420a69d` |
| Architecture | `linux/amd64` (x86_64 umbrelOS) |
| Web UI port | `3000` (via `app_proxy`) |
| P2P port | `25174` (published for inbound peers) |
| Data directory | `${APP_DATA_DIR}/data` (persisted) |

The image runs `ordexcoind` and the web UI in a single container. The web UI
talks to the daemon over RPC on `127.0.0.1:25173` using cookie auth, so no
credentials are needed. Outbound peer discovery uses the built-in DNS seeds;
publishing port `25174` lets the node also accept inbound peers so it becomes a
real part of the network.

## Install on umbrelOS

There are two ways to install this package.

### Option A — community app store (recommended for testing)

1. On your umbrelOS device, open **App Store → Community App Store**.
2. Paste this repository URL: `https://github.com/rnts08/OrdexCoin-Umbrel-app`
   (the repo root contains `umbrel-app-store.yml`).
3. Click **Install** on the **OrdexCoin Node** app.
4. Open the app from your umbrelOS home screen — the web UI is served on
   port `3000` through umbrelOS's reverse proxy (`app_proxy`).

### Option B — manual install over SSH

On the umbrelOS device:

```sh
# 1. Place the package into the on-device app-store directory
rsync -r ./ordexcoin/ ~/umbrel/app-data/ordexcoin/  # or your app-store dir

# 2. Install via the umbreld client
umbreld client apps.install.mutate --appId ordexcoin

# 3. Wait for the app to start (the daemon does the initial chain sync)
# 4. Open the app from the umbrelOS home screen
```

## Verify it works

Once installed, check:

- **Web UI** — opens from the umbrelOS home screen (served through `app_proxy`
  on port 3000). You should see node status, balances, transactions, addresses,
  a send screen and an RPC console.
- **Chain sync** — in the web UI, the block height should start advancing. The
  daemon data directory is `${APP_DATA_DIR}/data`; it persists across app
  restarts.
- **P2P** — `docker port ordexcoin_app_1 25174` should show the published port.
  Inbound peers confirm the node is part of the network.
- **Persistence** — restart the app; the node resumes where it left off.

## Run the image without umbrelOS (for testing on any Docker host)

```sh
docker run -d --name ordexcoin \
  -p 3000:3000 \
  -p 25174:25174 \
  -v $HOME/.ordexcoin:/data \
  ghcr.io/rnts08/ordexcoin:0.1.0
```

Then open `http://<host>:3000`. The data directory `~/.ordexcoin` is created on
first run; the daemon generates `ordexcoin.conf` there automatically.

Optional environment variables (see `webapp/entrypoint.sh`):

| Variable | Default | Purpose |
| --- | --- | --- |
| `ORDEXCOIN_DATA` | `/data` | daemon data directory |
| `ORDEXCOIN_RPC_URL` | `http://127.0.0.1:25173` | RPC endpoint for the web UI |
| `ORDEXCOIN_P2P_PORT` | `25174` | P2P listen port |
| `ORDEXCOIN_WEB_ADDR` | `0.0.0.0:3000` | web UI listen address |
| `ORDEXCOIN_WEB_USER` | _(none)_ | optional web UI basic auth user |
| `ORDEXCOIN_WEB_PASS` | _(none)_ | optional web UI basic auth pass |

## Support continued development

Support on-going development and infrastructure costs.

- EVM: `0x6e8e3c2b31424266e7cff59e910df1587c317427`
- BTC: `bc1qzzvcguvqjc6qhwe2y5vy38w2zke7hksukjhm68`
- LTC: `MPfm5QLKH1r9XxgWmH75Gyps4LDfX5c53L`
- SOL: `GGEaCMpnyM8tB5BU4RMuLm6tgMr3q9FgMHodxDxxAGby`
- OXC: `oxc1qcjav0mpjjvufc2zwfddnmep0janwv0czwk657e`

All donated support goes directly back into development and infrastructure.

## Architecture notes

- **amd64 only for now.** umbrelOS on x86_64 is fully supported. ARM64
  (Raspberry Pi) needs an aarch64 daemon build plus a multi-arch image manifest;
  this is a separate future task and does not block the x86 release.

## Development

- `webapp/` — Go web UI (single static binary) + `Dockerfile` + `entrypoint.sh`
- `build-docker-web.sh` — build the image locally (`ordexcoin-web:local`)
- `docs/UMBREL.md` — full integration guide and future (official store PR) steps
