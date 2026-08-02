#!/usr/bin/env bash
# Build the combined OrdexCoin container image (daemon + web UI).
# Build context is the repository root; the Dockerfile lives at webapp/Dockerfile.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE="${1:-ordexcoin-web:local}"
VERSION="$(cat "${ROOT_DIR}/VERSION")"

cd "${ROOT_DIR}"
docker build --build-arg VERSION="${VERSION}" -t "${IMAGE}" -f webapp/Dockerfile .
echo ""
echo "==> Built ${IMAGE}"
echo "    Run: docker compose -f webapp/docker-compose.yml up"
echo "    or : docker run -p 3000:3000 -v $HOME/.ordexcoin:/data ${IMAGE}"
