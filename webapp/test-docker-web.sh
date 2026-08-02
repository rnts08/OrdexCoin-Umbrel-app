#!/usr/bin/env bash
# End-to-end verification of the packaged OrdexCoin container
# (ordexcoind + ordexcoin-web). Builds the image, runs a fresh container with a
# throwaway datadir, and exercises every webapp API endpoint + static assets.
#
# No real transactions are broadcast: send/tip are only tested for their
# validation/error paths.
#
# Usage:  ./webapp/test-docker-web.sh
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="ordexcoin-web:test"
CONTAINER="oxc-web-test"
PORT=3099
DATADIR="$(mktemp -d /tmp/oxc-web-test-XXXX)"
BASE="http://127.0.0.1:${PORT}"

PASS=0
FAIL=0

ok()   { echo "  PASS: $1"; PASS=$((PASS + 1)); }
bad()  { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

check() { # check <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (expected $2, got $3)"; fi
}

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1
  rm -rf "$DATADIR"
}
trap cleanup EXIT

echo "==> Building image ${IMAGE}..."
docker build -q -f "$ROOT_DIR/webapp/Dockerfile" -t "$IMAGE" "$ROOT_DIR" >/dev/null || { echo "image build failed"; exit 1; }

echo "==> Starting container (fresh datadir $DATADIR)..."
docker run -d --name "$CONTAINER" -p "$PORT:3000" -p 25999:25174 -v "$DATADIR:/data" \
  -e ORDEXCOIN_P2P_PORT=25174 "$IMAGE" >/dev/null

echo -n "==> Waiting for web UI"
for _ in $(seq 1 60); do
  if curl -sf -o /dev/null "$BASE/" 2>/dev/null; then echo ""; break; fi
  echo -n "."
  sleep 2
done
echo ""

echo "==> Static assets"
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/")
check "index.html" 200 "$code"
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/style.css")
check "style.css" 200 "$code"
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/app.js")
check "app.js" 200 "$code"
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/favicon.png")
check "favicon.png" 200 "$code"
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/logo.png")
check "logo.png" 200 "$code"

echo "==> API: info"
info=$(curl -s "$BASE/api/info")
echo "$info" | grep -q '"version":"0.1.0"' && ok "info has version" || bad "info version"
echo "$info" | grep -q '"devAddress":"oxc1qcjav0mpjjvufc2zwfddnmep0janwv0czwk657e"' && ok "info has dev address" || bad "info dev address"

echo "==> API: status"
status=$(curl -s "$BASE/api/status")
echo "$status" | grep -q '"chain"' && ok "status has chain" || bad "status chain"
chain=$(echo "$status" | python3 -c "import json,sys; print(json.load(sys.stdin)['chain']['chain'])")
check "status chain == main" main "$chain"

echo "==> API: wallets (fresh node, none yet)"
wallets=$(curl -s "$BASE/api/wallets")
check "wallets empty" '{"current":"","wallets":[]}' "$wallets"

echo "==> API: create default wallet"
created=$(curl -s -X POST "$BASE/api/wallets/create" -d '{"name":""}')
echo "$created" | grep -q '"ok":true' && ok "createwallet ok" || bad "createwallet: $created"
wallets=$(curl -s "$BASE/api/wallets")
echo "$wallets" | grep -q '""' && ok "default wallet now listed" || bad "wallets: $wallets"

echo "==> API: new address"
addr=$(curl -s -X POST "$BASE/api/addresses/new" -d '{"label":"verify"}')
echo "$addr" | grep -qE '"address":"oxc1[a-z0-9]{10,}"' && ok "generated oxc address" || bad "address: $addr"

echo "==> API: addresses / balances / transactions"
addrs=$(curl -s "$BASE/api/addresses")
echo "$addrs" | grep -q '"label":"verify"' && ok "address listed" || bad "addresses: $addrs"
bal=$(curl -s "$BASE/api/balances")
echo "$bal" | python3 -c "import json,sys; d=json.load(sys.stdin); assert set(d)=={'available','confirmed','unconfirmed','immature','utxoCount'}, d" && ok "balances shape" || bad "balances: $bal"
txs=$(curl -s "$BASE/api/transactions")
check "transactions is array" "[]" "$txs"

echo "==> API: fee estimate"
fee=$(curl -s "$BASE/api/fee-estimate")
echo "$fee" | grep -q '"available"' && ok "fee estimate endpoint" || bad "fee: $fee"

echo "==> API: console (node + wallet scope)"
bc=$(curl -s -X POST "$BASE/api/console" -d '{"method":"getblockcount","params":[]}')
echo "$bc" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['error'] is None and isinstance(d['result'], int)" && ok "console getblockcount" || bad "console: $bc"
wi=$(curl -s -X POST "$BASE/api/console" -d '{"method":"getwalletinfo","params":[]}')
echo "$wi" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['error'] is None and 'walletname' in d['result'], d" && ok "console getwalletinfo (wallet scoped)" || bad "console: $wi"
err=$(curl -s -X POST "$BASE/api/console" -d '{"method":"bogusmethod","params":[]}')
echo "$err" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['error'] is not None and d['result'] is None, d" && ok "console error passthrough" || bad "console error: $err"

echo "==> API: send validation (no broadcast)"
sv=$(curl -s -X POST "$BASE/api/send" -d '{"address":"not-an-address","amount":1,"subtractFee":false}')
echo "$sv" | grep -qE '"error".*Invalid' && ok "send rejects invalid address" || bad "send: $sv"
sz=$(curl -s -X POST "$BASE/api/send" -d '{"address":"oxc1qcjav0mpjjvufc2zwfddnmep0janwv0czwk657e","amount":0,"subtractFee":false}')
echo "$sz" | grep -q '"error"' && ok "send rejects zero amount" || bad "send zero: $sz"

echo "==> API: tip validation (no broadcast)"
tz=$(curl -s -X POST "$BASE/api/tip" -d '{"amount":0,"subtractFee":false}')
echo "$tz" | grep -q '"error"' && ok "tip rejects zero amount" || bad "tip: $tz"

echo ""
echo "==> Container logs (tail)"
docker logs "$CONTAINER" 2>&1 | tail -6

echo ""
echo "=================================================="
echo "RESULT: ${PASS} passed, ${FAIL} failed"
echo "=================================================="
[ "$FAIL" -eq 0 ]
