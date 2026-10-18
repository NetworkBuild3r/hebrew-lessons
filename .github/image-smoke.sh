#!/bin/sh
# Boot a built hebrew-lessons image and fail if the process cannot serve.
# Usage: .github/image-smoke.sh <image>
# The token and admin password exist only for this process. They are not secrets.
set -eu

IMAGE=${1:-}
if [ -z "$IMAGE" ]; then
    echo "usage: $0 <image>" >&2
    exit 2
fi

WEB_PORT=${WEB_PORT:-18080}
MCP_PORT=${MCP_PORT:-18787}
TOKEN=${SMOKE_MCP_TOKEN:-smoke-token}
USER_NAME=${SMOKE_ADMIN_USER:-smokeadmin}
PASSWORD=${SMOKE_ADMIN_PASSWORD:-smoke-not-common-99}
ORIGIN="http://127.0.0.1:${WEB_PORT}"
NAME="hebrew-smoke-$$"
COOKIE=$(mktemp)
BODY=$(mktemp)
trap 'docker rm -f "$NAME" >/dev/null 2>&1 || true; rm -f "$COOKIE" "$BODY"' EXIT

docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" \
    -p "127.0.0.1:${WEB_PORT}:8080" \
    -p "127.0.0.1:${MCP_PORT}:8787" \
    -e HOST=0.0.0.0 \
    -e HEBREW_ROOT=/app \
    -e DATA_DIR=/tmp/hebrew-data \
    -e COOKIE_SECURE=0 \
    -e "PUBLIC_ORIGIN=${ORIGIN}" \
    -e "MCP_AUTH_TOKEN=${TOKEN}" \
    -e "ADMIN_USERNAME=${USER_NAME}" \
    -e "ADMIN_PASSWORD=${PASSWORD}" \
    "$IMAGE"

ready=0
i=0
while [ "$i" -lt 40 ]; do
    if curl -fsS "${ORIGIN}/healthz" 2>/dev/null | grep -qx 'ok'; then
        ready=1
        break
    fi
    if ! docker inspect -f '{{.State.Running}}' "$NAME" | grep -qx true; then
        echo "container exited before /healthz" >&2
        docker logs "$NAME" >&2 || true
        exit 1
    fi
    i=$((i + 1))
    sleep 1
done
if [ "$ready" -ne 1 ]; then
    echo "/healthz did not return 200 ok" >&2
    docker logs "$NAME" >&2 || true
    exit 1
fi
echo "healthz ok"

code=$(curl -sS -o "$BODY" -w '%{http_code}' -c "$COOKIE" \
    -H 'Content-Type: application/json' \
    -H "Origin: ${ORIGIN}" \
    -d "{\"username\":\"${USER_NAME}\",\"password\":\"${PASSWORD}\"}" \
    "${ORIGIN}/api/session")
if [ "$code" != "200" ]; then
    echo "sign-in failed: HTTP ${code}" >&2
    cat "$BODY" >&2
    exit 1
fi
echo "sign-in ok"

code=$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE" \
    "${ORIGIN}/worksheets/my-morning/index.html")
if [ "$code" != "200" ]; then
    echo "worksheet page failed: HTTP ${code}" >&2
    exit 1
fi
if ! grep -q 'src="worksheet.js"' "$BODY"; then
    echo "worksheet page is not the lesson shell" >&2
    exit 1
fi
echo "worksheet page ok"
code=$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE" \
    "${ORIGIN}/worksheets/my-morning/worksheet.js")
if [ "$code" != "200" ]; then
    echo "worksheet data failed: HTTP ${code}" >&2
    exit 1
fi
if ! grep -q '"id": "my-morning"' "$BODY"; then
    echo "worksheet data is not my-morning" >&2
    exit 1
fi
echo "worksheet ok"

code=$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE" \
    -H 'Accept: audio/mpeg' \
    "${ORIGIN}/worksheets/my-morning/audio/story/line-1.mp3")
if [ "$code" != "200" ]; then
    echo "audio fetch failed: HTTP ${code}" >&2
    exit 1
fi
bytes=$(wc -c < "$BODY" | tr -d ' ')
if [ "$bytes" -lt 100 ]; then
    echo "audio file is too small (${bytes} bytes)" >&2
    exit 1
fi
echo "audio ok (${bytes} bytes)"

code=$(curl -sS -o "$BODY" -w '%{http_code}' \
    -H "Authorization: Bearer ${TOKEN}" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
    "http://127.0.0.1:${MCP_PORT}/mcp")
if [ "$code" != "200" ]; then
    echo "tools/list failed: HTTP ${code}" >&2
    cat "$BODY" >&2
    exit 1
fi
docker exec -i "$NAME" node -e '
let raw = "";
process.stdin.on("data", (chunk) => { raw += chunk; });
process.stdin.on("end", () => {
    const payloads = [];
    const tryParse = (text) => {
        try { payloads.push(JSON.parse(text)); } catch (err) { /* not json */ }
    };
    tryParse(raw);
    raw.split(/\r?\n/).forEach((line) => {
        const trimmed = line.trim();
        if (!trimmed) return;
        tryParse(trimmed.startsWith("data:") ? trimmed.slice(5).trim() : trimmed);
    });
    let tools = null;
    payloads.forEach((payload) => {
        const found = payload && payload.result && payload.result.tools;
        if (Array.isArray(found)) tools = found;
    });
    if (!tools) {
        console.error("tools/list response has no tools array");
        console.error(raw.slice(0, 2000));
        process.exit(1);
    }
    if (tools.length !== 42) {
        console.error("expected 42 tools, got " + tools.length);
        tools.forEach((tool) => console.error("  " + tool.name));
        process.exit(1);
    }
    console.log("tools/list ok (42)");
});
' < "$BODY"
