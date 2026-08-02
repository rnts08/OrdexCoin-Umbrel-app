# OrdexCoin — Umbrel App

Single-container package to run an **OrdexCoin node + web UI** on umbrelOS
(x86_64). This README is about running OrdexCoin as an Umbrel application and
what is still required to bring it to umbrelOS. For running the web UI on its
own (dev, standalone against an existing daemon), see
**[DEVELOPMENT.md](./DEVELOPMENT.md)**.

## What this app is

One container runs both the OrdexCoin daemon and the web UI:

```
ordexcoin:latest
 ├── /usr/local/bin/ordexcoind     (x86_64, datadir /data)
 ├── /usr/local/bin/ordexcoin-cli
 ├── /usr/local/bin/ordexcoin-web  (Go web UI, listens :3000)
 └── /usr/local/bin/entrypoint.sh  (PID 1; starts daemon, then web UI)
```

The web UI talks to the daemon over `127.0.0.1:25173` inside the container using
cookie auth (the daemon writes `/data/.cookie`, the webapp reads it
automatically). P2P port **25174** is published so an installed node can accept
inbound peers and join the network (outbound discovery uses the built-in DNS
seeds `node3.walletbuilders.com`, `ordexcoinddns.xyz`).

Web UI features: status, balances, transactions, addresses, send (with
subtract-fee-from-amount), an RPC console, and an About page with one-click
developer tips. Dark/gold theme from the OrdexCoin icon.

## Status

| Piece | State |
| --- | --- |
| Web app (`webapp/`) | Done, verified (23/23 E2E checks) |
| Container image (`webapp/Dockerfile`) | Done, verified locally |
| Umbrel manifest (`umbrel/umbrel-app.yml`) | **Stub** — TODO fields |
| Umbrel compose (`umbrel/docker-compose.yml`) | **Stub** — needs digest pin |
| Image published to a registry | **Not done** |
| Tested on umbrelOS device | **Not done** |

## Run it today (without umbrelOS)

You can run the exact same container locally:

```sh
./build-docker-web.sh
docker run -p 3000:3000 -p 25174:25174 -v ~/.ordexcoin:/data ordexcoin-web:local
```

Open `http://localhost:3000`. The container writes `/data/ordexcoin.conf` and
`/data/.cookie` on first run, starts the daemon, waits for RPC, then serves the UI.

## Step-by-step requirements to bring this to umbrelOS

Detailed, resumable guidance lives in **[docs/UMBREL.md](../docs/UMBREL.md)**;
`umbrel/` contains the package stubs. The steps, in order:

1. **Publish the image.** Build with `docker buildx --platform linux/amd64 -t
   <org>/ordexcoin:<ver> -f webapp/Dockerfile .`, push to a registry, and pin the
   resulting `sha256:<digest>` in `umbrel/docker-compose.yml`.
2. **Finalize `umbrel/umbrel-app.yml`.** Fill in `id`, `category`, `name`,
   description, developer, the app_proxy web port, release notes, gallery
   screenshots and a square `icon.svg`.
3. **Finalize `umbrel/docker-compose.yml`.** `app_proxy` → `APP_HOST:
   ordexcoin_app_1`, `APP_PORT: 3000`; image pinned by digest; `user: "1000:1000"`;
   `volumes: ${APP_DATA_DIR}/data:/data`; publish the P2P port
   (`${APP_ORDEXCOIN_P2P_PORT}`); optionally pass web UI auth from
   `${APP_PASSWORD}`/`${APP_SEED}`.
4. **Test on a device.** Fastest path is a community app store repo (any git repo
   with `umbrel-app-store.yml`), or rsync `umbrel/` onto the device and install
   via `umbreld client apps.install.mutate --appId ordexcoin`.
5. **Submit to the official store.** Fork `getumbrel/umbrel-apps`, add the
   package, open a PR with the review checklist.

### Technical TODO

- [ ] Publish amd64 image to a registry and paste the digest into
      `umbrel/docker-compose.yml`
- [ ] Fill every `TODO` in `umbrel/umbrel-app.yml` (id, category, name,
      description, developer, port, releaseNotes, gallery, icon.svg)
- [ ] Add a square `icon.svg` (reuse `webapp/web/logo.png` as a starting point)
- [ ] Add 3–5 gallery screenshots (1440×900)
- [ ] Finalize compose: app_proxy block, digest pin, `user: "1000:1000"`,
      `stop_grace_period: 15m30s`, data volume, P2P port publish, auth pass-through
- [ ] Test via community app store on a real umbrelOS x86 device
- [ ] Re-run `./webapp/test-docker-web.sh` before any release
- [ ] (later, optional) ARM64 support: aarch64 static daemon build + multi-arch
      manifest; the Go web UI is already architecture-independent

## Package layout

```
webapp/         # Go web UI + Dockerfile + entrypoint + E2E tests
umbrel/         # Umbrel app package stubs (umbrel-app.yml, docker-compose.yml)
docs/UMBREL.md  # resumable umbrelOS integration guide
linux-static/   # static amd64 daemon binaries used by the image
```

## License

MIT. Based on Bitcoin Core v25.0 (The Bitcoin Core developers). Modifications
and branding by OrdexNetwork developers. See the About page in-app.
