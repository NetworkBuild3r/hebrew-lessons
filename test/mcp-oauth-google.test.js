const test = require('node:test');
const assert = require('node:assert');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');

const TOKEN = 'oauth-google-test-token-' + 'z'.repeat(32);
const SECRET = 'test-google-secret';
const PORT = 38788;
const GOOGLE_PORT = 38789;
const BASE = 'http://127.0.0.1:' + PORT;
const GOOGLE = 'http://127.0.0.1:' + GOOGLE_PORT;
const CB = 'cursor://anysphere.cursor-mcp/oauth/callback';

function call(method, path, { headers = {}, body } = {}) {
    return new Promise((resolve, reject) => {
        const req = http.request({
            host: '127.0.0.1', port: PORT, path, method,
            headers: Object.assign({}, headers, body ? { 'Content-Length': Buffer.byteLength(body) } : {})
        }, res => {
            let data = '';
            res.on('data', c => { data += c; });
            res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, data }));
        });
        req.on('error', reject);
        req.end(body);
    });
}

const form = obj => new URLSearchParams(obj).toString();
const FORM = { 'Content-Type': 'application/x-www-form-urlencoded' };

function pkce() {
    const verifier = crypto.randomBytes(48).toString('base64url');
    return { verifier, challenge: crypto.createHash('sha256').update(verifier).digest('base64url') };
}

function pageTarget(res) {
    const match = /url=([^"]+)"/.exec(res.data);
    assert.ok(match, 'expected a return link');
    return new URL(match[1].replace(/&amp;/g, '&'));
}

let server;
let google;
let lastToken = '';

test.before(async () => {
    process.env.MCP_AUTH_TOKEN = TOKEN;
    process.env.MCP_PUBLIC_URL = BASE;
    process.env.MCP_PORT = String(PORT);
    process.env.HOST = '127.0.0.1';
    process.env.GOOGLE_OAUTH_CLIENT_ID = 'test-google-client';
    process.env.GOOGLE_OAUTH_CLIENT_SECRET = SECRET;
    process.env.MCP_OAUTH_ALLOWED_EMAILS = 'owner@example.com';
    process.env.GOOGLE_OAUTH_AUTH_URL = GOOGLE + '/auth';
    process.env.GOOGLE_OAUTH_TOKEN_URL = GOOGLE + '/token';
    process.env.GOOGLE_OAUTH_USERINFO_URL = GOOGLE + '/userinfo';
    process.env.DATA_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-mcp-oauth-'));
    process.env.LESSONS_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-mcp-oauth-lessons-'));
    const db = require('../server/db');
    db.close();
    db.open({ force: true });
    db.migrate();
    require('../server/accounts').createUser({
        username: 'owner',
        password: 'a-strong-pass1',
        role: 'admin',
        googleEmail: 'owner@example.com',
        actor: 'test'
    });

    google = http.createServer((req, res) => {
        const chunks = [];
        req.on('data', c => chunks.push(c));
        req.on('end', () => {
            const raw = Buffer.concat(chunks).toString('utf8');
            if (req.url.startsWith('/token')) {
                lastToken = raw;
                const params = new URLSearchParams(raw);
                if (params.get('client_secret') !== SECRET || params.get('code_verifier')) {
                    res.writeHead(400, { 'Content-Type': 'application/json' });
                    res.end('{}');
                    return;
                }
                if (params.get('redirect_uri') !== BASE + '/oauth/google/callback') {
                    res.writeHead(400, { 'Content-Type': 'application/json' });
                    res.end('{}');
                    return;
                }
                const code = params.get('code');
                if (code === 'boom') {
                    res.writeHead(400, { 'Content-Type': 'application/json' });
                    res.end('{"error":"invalid_grant"}');
                    return;
                }
                res.writeHead(200, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ access_token: 'gat-' + code }));
                return;
            }
            if (req.url.startsWith('/userinfo')) {
                const token = String(req.headers.authorization || '').replace(/^Bearer /, '');
                const map = {
                    'gat-good': { email: 'Owner@Example.com', verified_email: true, id: 'sub-owner' },
                    'gat-stranger': { email: 'stranger@example.com', email_verified: true },
                    'gat-unverified': { email: 'owner@example.com', email_verified: false }
                };
                if (!map[token]) {
                    res.writeHead(401, { 'Content-Type': 'application/json' });
                    res.end('{}');
                    return;
                }
                res.writeHead(200, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify(map[token]));
                return;
            }
            res.writeHead(404);
            res.end();
        });
    });
    await new Promise(resolve => google.listen(GOOGLE_PORT, '127.0.0.1', resolve));
    server = require('../mcp/http').listen();
    await new Promise(resolve => server.once('listening', resolve));
});

test.after(() => {
    server.close();
    google.close();
    delete process.env.GOOGLE_OAUTH_CLIENT_ID;
    delete process.env.GOOGLE_OAUTH_CLIENT_SECRET;
    delete process.env.MCP_OAUTH_ALLOWED_EMAILS;
    delete process.env.GOOGLE_OAUTH_AUTH_URL;
    delete process.env.GOOGLE_OAUTH_TOKEN_URL;
    delete process.env.GOOGLE_OAUTH_USERINFO_URL;
});

async function register() {
    const res = await call('POST', '/register', {
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ client_name: 'Grok Bot', redirect_uris: [CB] })
    });
    assert.strictEqual(res.status, 201);
    return JSON.parse(res.data).client_id;
}

function authorizeQuery(clientId, challenge) {
    return form({
        response_type: 'code', client_id: clientId, redirect_uri: CB,
        code_challenge: challenge, code_challenge_method: 'S256', state: 'st8'
    });
}

async function begin(clientId, challenge) {
    const page = await call('GET', '/authorize?' + authorizeQuery(clientId, challenge));
    assert.strictEqual(page.status, 302);
    assert.doesNotMatch(page.headers.location, /password|client_secret|test-google-secret/);
    const loc = new URL(page.headers.location);
    assert.strictEqual(loc.origin + loc.pathname, GOOGLE + '/auth');
    assert.strictEqual(loc.searchParams.get('client_id'), 'test-google-client');
    assert.strictEqual(loc.searchParams.get('redirect_uri'), BASE + '/oauth/google/callback');
    assert.strictEqual(loc.searchParams.get('code_challenge'), null);
    assert.strictEqual(loc.searchParams.get('scope'), 'email profile');
    return loc.searchParams.get('state');
}

test('Google sign-in replaces the token form and still issues an MCP token', async () => {
    const clientId = await register();
    const { verifier, challenge } = pkce();
    const state = await begin(clientId, challenge);

    const ok = await call('GET', '/oauth/google/callback?code=good&state=' + encodeURIComponent(state));
    const granted = pageTarget(ok);
    assert.strictEqual(granted.searchParams.get('state'), 'st8');
    const code = granted.searchParams.get('code');
    assert.ok(code);
    assert.doesNotMatch(ok.data, /Owner@Example.com/i);

    const token = await call('POST', '/token', {
        headers: FORM,
        body: form({ grant_type: 'authorization_code', code, client_id: clientId, redirect_uri: CB, code_verifier: verifier })
    });
    assert.strictEqual(token.status, 200);
    assert.strictEqual(token.headers['access-control-allow-origin'], '*');
    const issued = JSON.parse(token.data);
    const access = issued.access_token;
    const claims = JSON.parse(Buffer.from(access.split('.')[0], 'base64url').toString());
    assert.strictEqual(claims.un, 'owner');
    assert.ok(claims.uid);
    const refreshed = await call('POST', '/token', {
        headers: FORM,
        body: form({ grant_type: 'refresh_token', refresh_token: issued.refresh_token, client_id: clientId })
    });
    assert.strictEqual(refreshed.status, 200);
    const again = JSON.parse(Buffer.from(JSON.parse(refreshed.data).access_token.split('.')[0], 'base64url').toString());
    assert.strictEqual(again.uid, claims.uid);
    assert.strictEqual(again.un, 'owner');
    const mcp = await call('POST', '/mcp', {
        headers: {
            Authorization: 'Bearer ' + access,
            'Content-Type': 'application/json',
            Accept: 'application/json, text/event-stream'
        },
        body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list' })
    });
    assert.strictEqual(mcp.status, 200);
    assert.match(lastToken, /code=good/);

    const strangerState = await begin(clientId, pkce().challenge);
    const stranger = await call('GET', '/oauth/google/callback?code=stranger&state=' + encodeURIComponent(strangerState));
    const denied = pageTarget(stranger);
    assert.strictEqual(denied.searchParams.get('error'), 'access_denied');
    assert.strictEqual(denied.searchParams.get('code'), null);
    assert.doesNotMatch(stranger.data, /stranger@example.com/);

    const unverifiedState = await begin(clientId, pkce().challenge);
    const unverified = await call('GET', '/oauth/google/callback?code=unverified&state=' + encodeURIComponent(unverifiedState));
    assert.strictEqual(pageTarget(unverified).searchParams.get('error'), 'access_denied');

    const googleDenied = await begin(clientId, pkce().challenge);
    const cancelled = await call('GET', '/oauth/google/callback?error=access_denied&state=' + encodeURIComponent(googleDenied));
    assert.strictEqual(pageTarget(cancelled).searchParams.get('error'), 'access_denied');

    const broken = await begin(clientId, pkce().challenge);
    const failed = await call('GET', '/oauth/google/callback?code=boom&state=' + encodeURIComponent(broken));
    assert.strictEqual(failed.status, 502);
    assert.doesNotMatch(failed.data, /invalid_grant|test-google-secret/);

    const bypass = await call('POST', '/authorize', {
        headers: FORM,
        body: form({ req: state, password: TOKEN, action: 'approve' })
    });
    assert.strictEqual(bypass.status, 400);
    assert.doesNotMatch(bypass.data, /code=/);

    process.env.MCP_OAUTH_ALLOWED_EMAILS = '';
    try {
        const closed = await call('GET', '/authorize?' + authorizeQuery(clientId, pkce().challenge));
        assert.strictEqual(closed.status, 503);
        assert.match(closed.data, /No allowed email/);
        assert.strictEqual(closed.headers.location, undefined);
    } finally {
        process.env.MCP_OAUTH_ALLOWED_EMAILS = 'owner@example.com';
    }
});
