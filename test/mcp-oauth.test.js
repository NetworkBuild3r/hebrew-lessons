const test = require('node:test');
const assert = require('node:assert');
const crypto = require('crypto');
const http = require('http');

const TOKEN = 'oauth-test-token-' + 'y'.repeat(40);
const PORT = 38787;
const BASE = 'http://127.0.0.1:' + PORT;
const CB = 'cursor://anysphere.cursor-mcp/oauth/callback';

function call(method, path, { headers = {}, body } = {}) {
    return new Promise((resolve, reject) => {
        const req = http.request({ host: '127.0.0.1', port: PORT, path, method, headers: Object.assign({}, headers, body ? { 'Content-Length': Buffer.byteLength(body) } : {}) }, res => {
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
const MCP_HEADERS = { 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream' };
const LIST = JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list' });

let server;
test.before(async () => {
    process.env.MCP_AUTH_TOKEN = TOKEN;
    process.env.MCP_PUBLIC_URL = BASE;
    process.env.MCP_PORT = String(PORT);
    process.env.HOST = '127.0.0.1';
    server = require('../mcp/http').listen();
    await new Promise(r => server.once('listening', r));
});
test.after(() => server.close());

async function register(redirect) {
    const res = await call('POST', '/register', { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ client_name: 'Grok Bot', redirect_uris: [redirect] }) });
    return res;
}

function pkce() {
    const verifier = crypto.randomBytes(48).toString('base64url');
    return { verifier, challenge: crypto.createHash('sha256').update(verifier).digest('base64url') };
}

async function authorize(clientId, challenge, password, redirect = CB) {
    const q = form({ response_type: 'code', client_id: clientId, redirect_uri: redirect, code_challenge: challenge, code_challenge_method: 'S256', state: 'st8' });
    const page = await call('GET', '/authorize?' + q);
    assert.strictEqual(page.status, 200);
    assert.match(page.data, /Grok Bot/);
    const req = /name="req" value="([^"]+)"/.exec(page.data)[1];
    return call('POST', '/authorize', { headers: FORM, body: form({ req, password, action: 'approve' }) });
}

function codeFrom(res) {
    const target = /url=([^"]+)"/.exec(res.data)[1].replace(/&amp;/g, '&');
    const url = new URL(target);
    assert.strictEqual(url.searchParams.get('state'), 'st8');
    return url.searchParams.get('code');
}

test('discovery documents and 401 point clients at the sign-in flow', async () => {
    const none = await call('POST', '/mcp', { headers: MCP_HEADERS, body: LIST });
    assert.strictEqual(none.status, 401);
    assert.match(none.headers['www-authenticate'], /resource_metadata="http:\/\/127\.0\.0\.1:38787\/\.well-known\/oauth-protected-resource"/);
    const prm = JSON.parse((await call('GET', '/.well-known/oauth-protected-resource')).data);
    assert.strictEqual(prm.authorization_servers[0], BASE);
    const as = JSON.parse((await call('GET', '/.well-known/oauth-authorization-server')).data);
    assert.deepStrictEqual(as.code_challenge_methods_supported, ['S256']);
    assert.strictEqual(as.registration_endpoint, BASE + '/register');
});

test('full flow: register, sign in, token, call MCP, refresh', async () => {
    const reg = await register(CB);
    assert.strictEqual(reg.status, 201);
    const clientId = JSON.parse(reg.data).client_id;
    const { verifier, challenge } = pkce();

    const bad = await authorize(clientId, challenge, 'not-the-token');
    assert.strictEqual(bad.status, 401);
    assert.match(bad.data, /not correct/);

    const ok = await authorize(clientId, challenge, TOKEN);
    const code = codeFrom(ok);

    const tok = await call('POST', '/token', { headers: FORM, body: form({ grant_type: 'authorization_code', code, client_id: clientId, redirect_uri: CB, code_verifier: verifier }) });
    assert.strictEqual(tok.status, 200);
    const tokens = JSON.parse(tok.data);
    assert.strictEqual(tokens.token_type, 'Bearer');

    const list = await call('POST', '/mcp', { headers: Object.assign({ Authorization: 'Bearer ' + tokens.access_token }, MCP_HEADERS), body: LIST });
    assert.strictEqual(list.status, 200);
    assert.match(list.data, /upsert_lesson/);

    const replay = await call('POST', '/token', { headers: FORM, body: form({ grant_type: 'authorization_code', code, client_id: clientId, redirect_uri: CB, code_verifier: verifier }) });
    assert.strictEqual(replay.status, 400, 'a code is single use');

    const refreshed = await call('POST', '/token', { headers: FORM, body: form({ grant_type: 'refresh_token', refresh_token: tokens.refresh_token, client_id: clientId }) });
    assert.strictEqual(refreshed.status, 200);
    const again = await call('POST', '/mcp', { headers: Object.assign({ Authorization: 'Bearer ' + JSON.parse(refreshed.data).access_token }, MCP_HEADERS), body: LIST });
    assert.strictEqual(again.status, 200);

    const asRefresh = await call('POST', '/mcp', { headers: Object.assign({ Authorization: 'Bearer ' + tokens.refresh_token }, MCP_HEADERS), body: LIST });
    assert.strictEqual(asRefresh.status, 401, 'a refresh token is not an access token');
});

test('rejects wrong PKCE verifier, foreign redirect URIs, and forged tokens', async () => {
    const evil = await register('https://attacker.example/cb');
    assert.strictEqual(evil.status, 400, 'unlisted redirect URI cannot be registered');

    const reg = JSON.parse((await register(CB)).data);
    const { challenge } = pkce();
    const code = codeFrom(await authorize(reg.client_id, challenge, TOKEN));
    const wrong = await call('POST', '/token', { headers: FORM, body: form({ grant_type: 'authorization_code', code, client_id: reg.client_id, redirect_uri: CB, code_verifier: crypto.randomBytes(48).toString('base64url') }) });
    assert.strictEqual(wrong.status, 400);

    const other = await call('GET', '/authorize?' + form({ response_type: 'code', client_id: reg.client_id, redirect_uri: 'https://attacker.example/cb', code_challenge: challenge, code_challenge_method: 'S256' }));
    assert.strictEqual(other.status, 400, 'redirect_uri must match the registration; no redirect is issued');
    assert.ok(!other.headers.location);

    const noPkce = await call('GET', '/authorize?' + form({ response_type: 'code', client_id: reg.client_id, redirect_uri: CB }));
    assert.match(noPkce.data, /error=invalid_request/, 'PKCE is mandatory');
    assert.ok(!/code=/.test(noPkce.data));

    const forged = await call('POST', '/mcp', { headers: Object.assign({ Authorization: 'Bearer eyJ0IjoiYWNjZXNzIiwiZXhwIjo5OTk5OTk5OTk5fQ.AAAA' }, MCP_HEADERS), body: LIST });
    assert.strictEqual(forged.status, 401);
});

test('static bearer token still works; deny never issues a code', async () => {
    const stat = await call('POST', '/mcp', { headers: Object.assign({ Authorization: 'Bearer ' + TOKEN }, MCP_HEADERS), body: LIST });
    assert.strictEqual(stat.status, 200);

    const reg = JSON.parse((await register(CB)).data);
    const { challenge } = pkce();
    const q = form({ response_type: 'code', client_id: reg.client_id, redirect_uri: CB, code_challenge: challenge, code_challenge_method: 'S256', state: 'st8' });
    const page = await call('GET', '/authorize?' + q);
    const req = /name="req" value="([^"]+)"/.exec(page.data)[1];
    const denied = await call('POST', '/authorize', { headers: FORM, body: form({ req, action: 'deny' }) });
    assert.match(denied.data, /error=access_denied/);
    assert.ok(!/code=/.test(denied.data));
});
