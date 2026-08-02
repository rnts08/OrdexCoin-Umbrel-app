# OrdexCoin container image.
#
# Build context: repository ROOT (this Dockerfile lives at webapp/Dockerfile).
#   docker build -f webapp/Dockerfile -t ordexcoin:local .
#   ./build-docker-web.sh   (convenience wrapper)
#
# The container runs BOTH the daemon (ordexcoind) and the web UI
# (ordexcoin-web) via webapp/entrypoint.sh. This is the shape Umbrel expects:
# one app container that exposes a web UI and persists data under /data.
#
# Target platform: umbrelOS / Linux on x86_64 (amd64). The daemon binaries in
# linux-static/bin are amd64. ARM64 (Raspberry Pi) support would require an
# aarch64 static build plus a multi-arch manifest — a separate future task, see
# docs/UMBREL.md. The web UI binary is built with CGO_ENABLED=0 (pure Go) so it
# is fully static and architecture-independent; only the daemon binaries
# constrain the image to amd64.

# --- stage 1: build the Go web UI (fully static, no CGO) --------------------
FROM golang:1.22-alpine AS webapp-build
WORKDIR /src
COPY webapp/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ordexcoin-web .

# --- stage 2: runtime --------------------------------------------------------
# glibc-based slim base is required: the "static" daemon binaries still link
# libc / libstdc++ / libgcc_s (they are only static for the bundled deps).
FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=webapp-build /out/ordexcoin-web /usr/local/bin/ordexcoin-web
COPY linux-static/bin/ordexcoind /usr/local/bin/ordexcoind
COPY linux-static/bin/ordexcoin-cli /usr/local/bin/ordexcoin-cli
COPY webapp/entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/ordexcoind /usr/local/bin/ordexcoin-cli /usr/local/bin/ordexcoin-web /usr/local/bin/entrypoint.sh

ENV ORDEXCOIN_DATA=/data
VOLUME ["/data"]

EXPOSE 3000 25173

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
