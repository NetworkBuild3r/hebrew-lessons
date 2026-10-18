#!/usr/bin/env bash
# Boot the server under test against a fresh DATA_DIR, run the contract suite, tear down.
# TZ is pinned so later date-sensitive cases (the library daily cap) do not drift.
set -euo pipefail

export TZ=UTC

CONTRACT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$CONTRACT_DIR/.." && pwd)"

args=()
for arg in "$@"; do
  case "$arg" in
    -update|--update)
      export UPDATE_GOLDEN=1
      ;;
    *)
      args+=("$arg")
      ;;
  esac
done

if [[ -n "${CONTRACT_MCP_URL:-}" && -n "${CONTRACT_WEB_URL:-}" && -n "${CONTRACT_MCP_TOKEN:-}" && -n "${CONTRACT_MCP_PUBLIC_URL:-}" ]]; then
  echo "contract: using existing server $CONTRACT_WEB_URL $CONTRACT_MCP_URL"
  cd "$CONTRACT_DIR"
  go test ./... -count=1 -v "${args[@]}"
  exit $?
fi

DATA_DIR="$(mktemp -d "${TMPDIR:-/tmp}/hebrew-contract.XXXXXX")"
PORT="${PORT:-18080}"
MCP_PORT="${MCP_PORT:-18787}"
TOKEN="${CONTRACT_MCP_TOKEN:-contract-test-token}"
MCP_PUBLIC="${MCP_PUBLIC_URL:-http://127.0.0.1:${MCP_PORT}}"
ADMIN_USER="${CONTRACT_ADMIN_USER:-contract-admin}"
ADMIN_PASS="${CONTRACT_ADMIN_PASS:-contract-admin-pass}"

cleanup() {
  status=$?
  if [[ -n "${SERVER_PID:-}" ]]; then
    kill -- "-$SERVER_PID" 2>/dev/null || kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [[ "${KEEP_DATA:-}" == "1" ]]; then
    echo "contract: kept DATA_DIR=$DATA_DIR"
  else
    rm -rf "$DATA_DIR"
  fi
  exit "$status"
}
trap cleanup EXIT

export DATA_DIR PORT MCP_PORT
export HOST=127.0.0.1
export MCP_AUTH_TOKEN="$TOKEN"
export MCP_PUBLIC_URL="$MCP_PUBLIC"
export MCP_ALLOWED_HOSTS="${MCP_ALLOWED_HOSTS:-127.0.0.1}"
export ADMIN_USERNAME="$ADMIN_USER"
export ADMIN_PASSWORD="$ADMIN_PASS"
export ADMIN_DISPLAY_NAME="${ADMIN_DISPLAY_NAME:-Contract Admin}"

# Paid providers stay unset. Audio and image tools must not call out.
unset ELEVENLABS_API_KEY || true
unset ELEVENLABS_KEY || true
unset OPENAI_API_KEY || true
unset GOOGLE_OAUTH_CLIENT_ID || true
unset GOOGLE_OAUTH_CLIENT_SECRET || true
unset GOOGLE_CLIENT_ID || true
unset GOOGLE_CLIENT_SECRET || true

# LESSONS_DIR is left unset on purpose. server/start.js copies any missing
# seed from worksheets/ into LESSONS_DIR, so a partial fixture would be filled
# back in (including lessons the fixture omits). The trimmed tree in
# fixtures/lessons is for a later seed phase.

CMD="${CONTRACT_SERVER_CMD:-node $ROOT/server/start.js}"
LOG="$DATA_DIR/server.log"
cd "$ROOT"
# setsid makes the server a process-group leader so teardown can signal Node,
# not only the shell that launched it.
setsid bash -c "$CMD" >"$LOG" 2>&1 &
SERVER_PID=$!
echo "contract: booting pid=$SERVER_PID data=$DATA_DIR web=$PORT mcp=$MCP_PORT"

ready=0
for _ in $(seq 1 120); do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "contract: server exited early"
    cat "$LOG"
    exit 1
  fi
  web_code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 1 --max-redirs 0 "http://127.0.0.1:${PORT}/" || true)"
  mcp_code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 1 -X POST "http://127.0.0.1:${MCP_PORT}/mcp" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    --data '{"jsonrpc":"2.0","id":1,"method":"ping"}' || true)"
  if [[ "$web_code" != "000" && "$mcp_code" == "401" ]]; then
    ready=1
    break
  fi
  sleep 0.5
done

if [[ "$ready" != "1" ]]; then
  echo "contract: server was not ready (web=$web_code mcp=$mcp_code)"
  cat "$LOG"
  exit 1
fi

export CONTRACT_WEB_URL="http://127.0.0.1:${PORT}"
export CONTRACT_MCP_URL="http://127.0.0.1:${MCP_PORT}/mcp"
export CONTRACT_MCP_TOKEN="$TOKEN"
export CONTRACT_MCP_PUBLIC_URL="$MCP_PUBLIC"
export CONTRACT_DATA_DIR="$DATA_DIR"
export CONTRACT_ADMIN_USER="$ADMIN_USER"
export CONTRACT_ADMIN_PASS="$ADMIN_PASS"

cd "$CONTRACT_DIR"
go test ./... -count=1 -v "${args[@]}"
