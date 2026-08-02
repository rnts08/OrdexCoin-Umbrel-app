#!/usr/bin/env bash
# Update Umbrel app package from templates.
# Usage: ./scripts/update-umbrel-package.sh <version> <digest>
set -euo pipefail

VERSION="${1:?version required}"
DIGEST="${2:?digest required}"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PKG_DIR="${ROOT_DIR}/ordexcoin"

# Render templates
sed -e "s/__VERSION__/${VERSION}/g" -e "s/__DIGEST__/${DIGEST}/g" \
  "${PKG_DIR}/docker-compose.yml.tmpl" > "${PKG_DIR}/docker-compose.yml"

sed -e "s/__VERSION__/${VERSION}/g" \
  "${PKG_DIR}/umbrel-app.yml.tmpl" > "${PKG_DIR}/umbrel-app.yml"

echo "Updated ordexcoin package to version ${VERSION} with digest ${DIGEST}"