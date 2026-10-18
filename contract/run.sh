#!/usr/bin/env bash
# Boot the server under test against a fresh DATA_DIR, run the contract suite, tear down.
# TZ is pinned so later date-sensitive cases (the library daily cap) do not drift.
#
# CONTRACT_SERVER_CMD is a shell command (default: node server/start.js), run as
# `bash -c` from the repo root. It inherits the env contract in contract/README.md.
# The command must stay in the foreground. Teardown signals its process group.
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

# bash 3.2 (macOS /bin/bash) errors on "${arr[@]}" when the array is empty and
# set -u is on. Expand only when the caller passed extra go test args.
go_test() {
  if [[ ${#args[@]} -gt 0 ]]; then
    go test ./... -count=1 -v "${args[@]}"
  else
    go test ./... -count=1 -v
  fi
}

if [[ -n "${CONTRACT_MCP_URL:-}" && -n "${CONTRACT_WEB_URL:-}" && -n "${CONTRACT_MCP_TOKEN:-}" && -n "${CONTRACT_MCP_PUBLIC_URL:-}" ]]; then
  echo "contract: using existing server $CONTRACT_WEB_URL $CONTRACT_MCP_URL"
  cd "$CONTRACT_DIR"
  go_test
  exit $?
fi

DATA_DIR="$(mktemp -d "${TMPDIR:-/tmp}/hebrew-contract.XXXXXX")"
PORT="${PORT:-18080}"
MCP_PORT="${MCP_PORT:-18787}"
TOKEN="${CONTRACT_MCP_TOKEN:-contract-test-token}"
MCP_PUBLIC="${MCP_PUBLIC_URL:-http://127.0.0.1:${MCP_PORT}}"
ADMIN_USER="${CONTRACT_ADMIN_USER:-contract-admin}"
ADMIN_PASS="${CONTRACT_ADMIN_PASS:-contract-admin-pass}"

# Descendants of pid, one per line, parent before children. pgrep is on macOS
# and Linux; without it, process-group signaling still covers children that
# stayed in the group.
descendant_pids() {
  local pid="$1"
  local child
  echo "$pid"
  command -v pgrep >/dev/null 2>&1 || return 0
  for child in $(pgrep -P "$pid" 2>/dev/null || true); do
    descendant_pids "$child"
  done
}

runner_pgid() {
  ps -o pgid= -p "$$" 2>/dev/null | tr -d '[:space:]' || true
}

server_pgid() {
  local pid="$1"
  ps -o pgid= -p "$pid" 2>/dev/null | tr -d '[:space:]' || true
}

# pids in pgid, one per line. ps -ax is procps and BSD.
group_pids() {
  local pgid="$1"
  [[ "$pgid" =~ ^[0-9]+$ ]] || return 0
  ps -ax -o pid=,pgid= 2>/dev/null | awk -v g="$pgid" '$2 == g { print $1 }'
}

signal_pid() {
  local sig="$1"
  local pid="$2"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 0
  [[ "$pid" -gt 1 ]] || return 0
  kill -"$sig" "$pid" 2>/dev/null || true
}

# Signal the server process group, then every descendant captured before the
# parent can exit and reparent them. Never signal the runner's own group.
stop_server() {
  local pid="${1:-}"
  [[ -n "$pid" ]] || return 0
  local pgid self member
  pgid="$(server_pgid "$pid")"
  self="$(runner_pgid)"
  # The launch paths make SERVER_PID the process-group leader. If it has
  # already exited, ps has no pgid; the group id was that pid.
  if [[ -z "$pgid" ]]; then
    pgid="$pid"
  fi
  if [[ -n "$self" && "$pgid" == "$self" ]]; then
    pgid=""
  fi
  local tree
  tree="$(descendant_pids "$pid" || true)"
  if [[ "$pgid" =~ ^[0-9]+$ ]] && [[ "$pgid" -gt 1 ]]; then
    kill -TERM -"$pgid" 2>/dev/null || true
  else
    pgid=""
  fi
  while IFS= read -r member; do
    signal_pid TERM "$member"
  done <<<"$tree"
  if [[ -n "$pgid" ]]; then
    while IFS= read -r member; do
      signal_pid TERM "$member"
    done < <(group_pids "$pgid" || true)
  fi
  local _
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    sleep 0.1
  done
  if [[ "$pgid" =~ ^[0-9]+$ ]] && [[ "$pgid" -gt 1 ]]; then
    kill -KILL -"$pgid" 2>/dev/null || true
    while IFS= read -r member; do
      signal_pid KILL "$member"
    done < <(group_pids "$pgid" || true)
  fi
  while IFS= read -r member; do
    signal_pid KILL "$member"
  done <<<"$tree"
  wait "$pid" 2>/dev/null || true
}

cleanup() {
  status=$?
  if [[ -n "${SERVER_PID:-}" ]]; then
    stop_server "$SERVER_PID"
  fi
  if [[ "${KEEP_DATA:-}" == "1" ]]; then
    echo "contract: kept DATA_DIR=$DATA_DIR"
    echo "contract: kept HEBREW_ROOT=${HEBREW_ROOT:-}"
  else
    rm -rf "$DATA_DIR"
    if [[ -n "${LESSONS_ROOT:-}" ]]; then
      rm -rf "$LESSONS_ROOT"
    fi
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

# The fixture is two lessons plus _template and catalog.js. server/start.js
# copies any missing seed from HEBREW_ROOT/worksheets into LESSONS_DIR, which
# would pull the rest of the repo worksheets (including lessons the fixture
# omits) if HEBREW_ROOT stayed the repo. Point both at the same copy so that
# copy is a no-op. shared/ is linked so the web app still resolves its files.
LESSONS_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/hebrew-contract-lessons.XXXXXX")"
mkdir -p "$LESSONS_ROOT"
cp -a "$CONTRACT_DIR/fixtures/lessons" "$LESSONS_ROOT/worksheets"
ln -s "$ROOT/shared" "$LESSONS_ROOT/shared"
export HEBREW_ROOT="$LESSONS_ROOT"
export LESSONS_DIR="$LESSONS_ROOT/worksheets"
export CONTRACT_LESSONS_DIR="$LESSONS_DIR"

# A normal run boots both server legs from the committed database. -update
# leaves DATA_DIR empty so the server migrates, then the suite seeds through
# HTTP and MCP and re-snapshots the file. Sidecars are not part of the fixture.
if [[ "${UPDATE_GOLDEN:-}" == "1" || "${UPDATE_GOLDEN:-}" == "true" ]]; then
  echo "contract: update mode seeds an empty DATA_DIR through HTTP and MCP"
else
  SEED_DB="${CONTRACT_SEED_DB:-$CONTRACT_DIR/fixtures/db/seeded.sqlite}"
  if [[ ! -f "$SEED_DB" ]]; then
    echo "contract: seeded fixture not found: $SEED_DB" >&2
    exit 1
  fi
  SEED_ABS="$(cd "$(dirname "$SEED_DB")" && pwd)/$(basename "$SEED_DB")"
  cp -- "$SEED_ABS" "$DATA_DIR/hebrew.sqlite"
  rm -f "$DATA_DIR/hebrew.sqlite-wal" "$DATA_DIR/hebrew.sqlite-shm"
  echo "contract: using seeded fixture $SEED_ABS"
fi

if [[ -n "${CONTRACT_SERVER_CMD:-}" ]]; then
  CMD="$CONTRACT_SERVER_CMD"
else
  CMD="node $ROOT/server/start.js"
fi
LOG="$DATA_DIR/server.log"
cd "$ROOT"

# setsid (util-linux) makes the server a session and process-group leader.
# macOS has no setsid. Monitor mode gives the background command its own
# process group there; it is turned off again so later job notices stay quiet.
# The group id does not change when monitor mode is turned off.
if command -v setsid >/dev/null 2>&1; then
  LAUNCHER=setsid
  setsid bash -c "$CMD" >"$LOG" 2>&1 &
  SERVER_PID=$!
else
  LAUNCHER=process-group
  set -m
  bash -c "$CMD" >"$LOG" 2>&1 &
  SERVER_PID=$!
  set +m
fi
echo "contract: booting pid=$SERVER_PID launcher=$LAUNCHER data=$DATA_DIR web=$PORT mcp=$MCP_PORT cmd=$CMD"

dump_log() {
  echo "contract: server log:"
  if [[ -s "$LOG" ]]; then
    cat "$LOG"
  else
    echo "(no output)"
  fi
}

ready=0
web_code="000"
mcp_code="000"
for _ in $(seq 1 120); do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "contract: server exited before it was ready."
    echo "contract: CONTRACT_SERVER_CMD=$CMD"
    dump_log
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
  echo "contract: server was not ready (web=$web_code mcp=$mcp_code)."
  echo "contract: CONTRACT_SERVER_CMD=$CMD"
  dump_log
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
go_test
