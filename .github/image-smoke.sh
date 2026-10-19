#!/bin/sh
# Boot a built hebrew-lessons image and fail if the process cannot serve.
# Usage: .github/image-smoke.sh <image>
# The token and admin password exist only for this process. They are not secrets.
#
# k8-build's DOCKER_HOST is a shared remote daemon, not a local engine.
# Ports published with -p land on that host, not on this runner, and a fixed
# host port collides with another job. This script publishes nothing.
# A probe container joins the app container's network namespace
# (--network container:$NAME) and connects to the app's own loopback as
# "localhost" (IPv4). That name is the shared namespace, not this runner.
set -eu

IMAGE=${1:-}
if [ -z "$IMAGE" ]; then
    echo "usage: $0 <image>" >&2
    exit 2
fi

RUN_ID=${GITHUB_RUN_ID:-local}
ATTEMPT=${GITHUB_RUN_ATTEMPT:-0}
# Unique on the shared daemon across concurrent jobs and re-runs.
NAME="hebrew-smoke-${RUN_ID}-${ATTEMPT}-$$"
PROBE="hebrew-probe-${RUN_ID}-${ATTEMPT}-$$"
# The image under test is already loaded and contains node. A separate curl
# image would be another pull. Override only to point at a different client
# image that has `node` on PATH (tag or digest).
PROBE_IMAGE=${SMOKE_PROBE_IMAGE:-$IMAGE}
TOKEN=${SMOKE_MCP_TOKEN:-smoke-token}
USER_NAME=${SMOKE_ADMIN_USER:-smokeadmin}
PASSWORD=${SMOKE_ADMIN_PASSWORD:-smoke-not-common-99}
PUBLIC_ORIGIN=${SMOKE_PUBLIC_ORIGIN:-http://localhost:8080}
TIMEOUT_MS=${SMOKE_TIMEOUT_MS:-45000}
PROBE_JS=$(mktemp)

cleanup() {
    code=$?
    rm -f "$PROBE_JS"
    docker rm -f "$PROBE" >/dev/null 2>&1 || true
    docker rm -f "$NAME" >/dev/null 2>&1 || true
    exit "$code"
}
trap cleanup EXIT
trap 'exit 1' INT TERM

docker rm -f "$PROBE" "$NAME" >/dev/null 2>&1 || true

echo "smoke: app ${NAME}, probe ${PROBE}, network container:${NAME}, no published ports"

docker run -d --name "$NAME" \
    -e HOST=0.0.0.0 \
    -e HEBREW_ROOT=/app \
    -e DATA_DIR=/tmp/hebrew-data \
    -e COOKIE_SECURE=0 \
    -e "PUBLIC_ORIGIN=${PUBLIC_ORIGIN}" \
    -e "MCP_AUTH_TOKEN=${TOKEN}" \
    -e "ADMIN_USERNAME=${USER_NAME}" \
    -e "ADMIN_PASSWORD=${PASSWORD}" \
    "$IMAGE"

# The probe shares the app network namespace, so nothing is published with -p.
# A bind mount would name a path on the remote daemon's host, not this runner,
# so the script is copied over the Docker API. sleep keeps the probe up; node
# in that image is the HTTP client (no extra curl pull).
cat > "$PROBE_JS" <<'JS'
'use strict';

const http = require('http');

const origin = process.env.PUBLIC_ORIGIN || 'http://localhost:8080';
const token = process.env.SMOKE_MCP_TOKEN || '';
const user = process.env.SMOKE_ADMIN_USER || '';
const password = process.env.SMOKE_ADMIN_PASSWORD || '';
const timeoutMs = Number(process.env.SMOKE_TIMEOUT_MS || 45000);
const web = 'http://localhost:8080';
const mcp = 'http://localhost:8787';

function fail(message) {
    console.error(message);
    process.exit(1);
}

function request(method, url, headers, body) {
    return new Promise((resolve, reject) => {
        const u = new URL(url);
        const reqHeaders = Object.assign({}, headers || {});
        const payload = body == null ? null : Buffer.from(body);
        if (payload && !reqHeaders['Content-Length'] && !reqHeaders['content-length']) {
            reqHeaders['Content-Length'] = String(payload.length);
        }
        const req = http.request({
            protocol: u.protocol,
            hostname: u.hostname,
            port: u.port,
            path: u.pathname + u.search,
            method,
            headers: reqHeaders,
            family: 4
        }, (res) => {
            const chunks = [];
            res.on('data', (chunk) => chunks.push(chunk));
            res.on('end', () => {
                resolve({
                    status: res.statusCode,
                    headers: res.headers,
                    body: Buffer.concat(chunks)
                });
            });
        });
        req.on('error', reject);
        if (payload) req.write(payload);
        req.end();
    });
}

function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

function cookieHeader(setCookie) {
    const list = Array.isArray(setCookie) ? setCookie : (setCookie ? [setCookie] : []);
    if (!list.length) return '';
    return list.map((line) => String(line).split(';')[0]).join('; ');
}

function assertNoCookie(res, label) {
    if (res.headers['set-cookie']) {
        fail(label + ' set a cookie');
    }
}

function assertHealth(res, label) {
    if (res.status !== 200) fail(label + ' HTTP ' + res.status + ' body ' + JSON.stringify(res.body.toString('utf8').slice(0, 200)));
    assertNoCookie(res, label);
    const text = res.body.toString('utf8');
    const type = String(res.headers['content-type'] || '');
    if (!/^text\/plain/i.test(type)) fail(label + ' content-type ' + type);
    if (text !== 'ok') fail(label + ' body is not ok: ' + JSON.stringify(text.slice(0, 200)));
    if (/my-morning|worksheet|"id"|<html/i.test(text)) fail(label + ' returned lesson data');
}

async function waitHealth() {
    const deadline = Date.now() + timeoutMs;
    let last = 'no response';
    while (Date.now() < deadline) {
        try {
            const res = await request('GET', web + '/healthz');
            if (res.status === 200 && res.body.toString('utf8') === 'ok') {
                return { present: true, res: res };
            }
            // An image from before this route answers 404. Do not keep waiting.
            if (res.status === 404) return { present: false, res: res };
            // The process is accepting connections. Any other status will not become ok.
            fail('GET /healthz HTTP ' + res.status + ' body ' + JSON.stringify(res.body.toString('utf8').slice(0, 200)));
        } catch (err) {
            last = err && err.message ? err.message : String(err);
        }
        await sleep(1000);
    }
    fail('/healthz did not return 200 ok within ' + timeoutMs + 'ms (' + last + ')');
}

function toolCount(raw) {
    const payloads = [];
    const tryParse = (text) => {
        try { payloads.push(JSON.parse(text)); } catch (err) { /* not json */ }
    };
    tryParse(raw);
    raw.split(/\r?\n/).forEach((line) => {
        const trimmed = line.trim();
        if (!trimmed) return;
        tryParse(trimmed.startsWith('data:') ? trimmed.slice(5).trim() : trimmed);
    });
    let tools = null;
    payloads.forEach((payload) => {
        const found = payload && payload.result && payload.result.tools;
        if (Array.isArray(found)) tools = found;
    });
    if (!tools) {
        fail('tools/list response has no tools array\n' + raw.slice(0, 2000));
    }
    return tools;
}

async function main() {
    const ready = await waitHealth();
    if (!ready.present) {
        console.log('GET /healthz returned 404. This image has no /healthz route. Falling back to GET /login/.');
        const login = await request('GET', web + '/login/', { Accept: 'text/html' });
        if (login.status !== 200) {
            fail('fallback GET /login/ HTTP ' + login.status + ' body ' + JSON.stringify(login.body.toString('utf8').slice(0, 200)));
        }
        console.log('fallback: GET /login/ returned 200');
    } else {
        assertHealth(ready.res, 'GET /healthz (no cookie)');
        console.log('healthz ok');

        const head = await request('HEAD', web + '/healthz');
        if (head.status !== 200) fail('HEAD /healthz HTTP ' + head.status);
        assertNoCookie(head, 'HEAD /healthz');
        if (head.body.length !== 0) fail('HEAD /healthz body is not empty (' + head.body.length + ' bytes)');
        console.log('healthz HEAD ok');

        const post = await request('POST', web + '/healthz', {
            'Content-Type': 'application/json',
            Origin: origin
        }, '{}');
        if (post.status !== 405) fail('POST /healthz HTTP ' + post.status + ' (want 405)');
        console.log('healthz POST rejected (405)');
    }

    const signInBody = JSON.stringify({ username: user, password: password });
    const signed = await request('POST', web + '/api/session', {
        'Content-Type': 'application/json',
        Origin: origin
    }, signInBody);
    if (signed.status !== 200) {
        fail('sign-in failed: HTTP ' + signed.status + ' ' + signed.body.toString('utf8').slice(0, 500));
    }
    const cookie = cookieHeader(signed.headers['set-cookie']);
    if (!cookie) fail('sign-in set no cookie');
    console.log('sign-in ok');

    const page = await request('GET', web + '/worksheets/my-morning/index.html', { Cookie: cookie });
    if (page.status !== 200) fail('worksheet page failed: HTTP ' + page.status);
    if (!page.body.toString('utf8').includes('src="worksheet.js"')) {
        fail('worksheet page is not the lesson shell');
    }
    console.log('worksheet page ok');

    const data = await request('GET', web + '/worksheets/my-morning/worksheet.js', { Cookie: cookie });
    if (data.status !== 200) fail('worksheet data failed: HTTP ' + data.status);
    if (!data.body.toString('utf8').includes('"id": "my-morning"')) {
        fail('worksheet data is not my-morning');
    }
    console.log('worksheet ok');

    const audio = await request('GET', web + '/worksheets/my-morning/audio/story/line-1.mp3', {
        Cookie: cookie,
        Accept: 'audio/mpeg'
    });
    if (audio.status !== 200) fail('audio fetch failed: HTTP ' + audio.status);
    if (audio.body.length < 100) fail('audio file is too small (' + audio.body.length + ' bytes)');
    console.log('audio ok (' + audio.body.length + ' bytes)');

    const listed = await request('POST', mcp + '/mcp', {
        Authorization: 'Bearer ' + token,
        'Content-Type': 'application/json',
        Accept: 'application/json, text/event-stream'
    }, '{"jsonrpc":"2.0","id":1,"method":"tools/list"}');
    if (listed.status !== 200) {
        fail('tools/list failed: HTTP ' + listed.status + ' ' + listed.body.toString('utf8').slice(0, 500));
    }
    const tools = toolCount(listed.body.toString('utf8'));
    if (tools.length !== 42) {
        const names = tools.map((tool) => tool && tool.name).join('\n  ');
        fail('expected 42 tools, got ' + tools.length + '\n  ' + names);
    }
    console.log('tools/list ok (42)');
}

main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
JS
chmod a+r "$PROBE_JS"

if ! docker run -d --name "$PROBE" \
    --network "container:${NAME}" \
    --entrypoint sleep \
    "$PROBE_IMAGE" 300
then
    echo "smoke probe failed to start" >&2
    docker logs "$NAME" >&2 || true
    exit 1
fi

docker cp "$PROBE_JS" "$PROBE:/tmp/smoke-probe.js"

if ! docker exec \
    -e "PUBLIC_ORIGIN=${PUBLIC_ORIGIN}" \
    -e "SMOKE_MCP_TOKEN=${TOKEN}" \
    -e "SMOKE_ADMIN_USER=${USER_NAME}" \
    -e "SMOKE_ADMIN_PASSWORD=${PASSWORD}" \
    -e "SMOKE_TIMEOUT_MS=${TIMEOUT_MS}" \
    "$PROBE" \
    node /tmp/smoke-probe.js
then
    echo "smoke probe failed" >&2
    docker logs "$NAME" >&2 || true
    exit 1
fi
