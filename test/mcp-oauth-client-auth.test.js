const test = require('node:test');
const assert = require('node:assert');
const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');

const TOKEN = 'oauth-client-auth-' + 'x'.repeat(40);
const PORT = 38790;
const BASE = 'http://127.0.0.1:' + PORT;
const CB = 'cursor://anysphere.cursor-mcp/oauth/callback';
const LEGACY_CLIENT = '11111111-1111-4111-8111-111111111111';
const LEGACY_CONF = '22222222-2222-4222-8222-222222222222';
const LEGACY_SECRET = 'c'.repeat(48);

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

function hmacKey() {
    return crypto.createHmac('sha256', TOKEN).update('hebrew-mcp-oauth-v1').digest();
}

function tokenHash(raw) {
    return crypto.createHmac('sha256', hmacKey()).update(String(raw)).digest('hex');
}

function sign(payload) {
    const body = Buffer.from(JSON.stringify(payload)).toString('base64url');
    const sig = crypto.createHmac('sha256', hmacKey()).update(body).digest('base64url');
    return body + '.' + sig;
}

function basic(id, secret) {
    return 'Basic ' + Buffer.from(id + ':' + secret, 'utf8').toString('base64');
}

function pkce() {
    const verifier = crypto.randomBytes(48).toString('base64url');
    return { verifier, challenge: crypto.createHash('sha256').update(verifier).digest('base64url') };
}

function codeFrom(res) {
    const target = /url=([^"]+)"/.exec(res.data)[1].replace(/&amp;/g, '&');
    return new URL(target).searchParams.get('code');
}

async function register(method, name) {
    const body = { client_name: name || 'Grok Bot', redirect_uris: [CB] };
    if (method) body.token_endpoint_auth_method = method;
    const res = await call('POST', '/register', {
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    });
    assert.strictEqual(res.status, 201);
    return JSON.parse(res.data);
}

async function authorize(clientId, challenge) {
    const q = form({
        response_type: 'code', client_id: clientId, redirect_uri: CB,
        code_challenge: challenge, code_challenge_method: 'S256', state: 'st8'
    });
    const page = await call('GET', '/authorize?' + q);
    assert.strictEqual(page.status, 200, page.data.slice(0, 200));
    const req = /name="req" value="([^"]+)"/.exec(page.data)[1];
    const approved = await call('POST', '/authorize', {
        headers: FORM,
        body: form({ req, password: TOKEN, action: 'approve' })
    });
    return codeFrom(approved);
}

function tokenRequest(body, headers) {
    return call('POST', '/token', { headers: Object.assign({}, FORM, headers || {}), body: form(body) });
}

let server;
let dataDir;
let legacyRefresh;
let unboundRefresh;
let confRefresh;
let hmacClient;
let hmacRefresh;
let legacySecretHash;

test.before(async () => {
    process.env.MCP_AUTH_TOKEN = TOKEN;
    process.env.MCP_PUBLIC_URL = BASE;
    process.env.MCP_PORT = String(PORT);
    process.env.HOST = '127.0.0.1';
    delete process.env.GOOGLE_OAUTH_CLIENT_ID;
    delete process.env.GOOGLE_OAUTH_CLIENT_SECRET;
    delete process.env.web_client_id;
    delete process.env.web_client_secret;
    delete process.env.client_id;
    delete process.env.client_secret;
    dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-mcp-client-auth-'));
    process.env.DATA_DIR = dataDir;

    legacyRefresh = crypto.randomBytes(32).toString('base64url');
    unboundRefresh = crypto.randomBytes(32).toString('base64url');
    confRefresh = crypto.randomBytes(32).toString('base64url');
    legacySecretHash = tokenHash(LEGACY_SECRET);
    hmacClient = sign({ ru: [CB], nm: 'HMAC client', t: 'client', exp: 4102444800 });
    hmacRefresh = sign({ cid: hmacClient, scope: 'mcp', t: 'refresh', exp: 2000000000 });

    const state = {
        clients: {
            [LEGACY_CLIENT]: { ru: [CB], nm: 'Legacy MCP client', am: 'none', exp: 2000000000 },
            [LEGACY_CONF]: {
                ru: [CB], nm: 'Legacy confidential', am: 'client_secret_post',
                sh: legacySecretHash, exp: 2000000000
            }
        },
        access: {},
        refresh: {
            [tokenHash(legacyRefresh)]: { cid: LEGACY_CLIENT, scope: 'mcp', exp: 2000000000 },
            [tokenHash(unboundRefresh)]: { scope: 'mcp', exp: 2000000000 },
            [tokenHash(confRefresh)]: { cid: LEGACY_CONF, scope: 'mcp', exp: 2000000000 }
        }
    };
    fs.writeFileSync(path.join(dataDir, 'mcp-oauth-state.json'), JSON.stringify(state), { mode: 0o600 });
    server = require('../mcp/http').listen();
    await new Promise(r => server.once('listening', r));
});

test.after(() => {
    if (server) server.close();
    delete process.env.DATA_DIR;
});

test('rejects a missing or wrong client secret', async () => {
    const post = await register('client_secret_post', 'Post client');
    const { verifier, challenge } = pkce();
    const code = await authorize(post.client_id, challenge);
    const base = {
        grant_type: 'authorization_code', code, client_id: post.client_id,
        redirect_uri: CB, code_verifier: verifier
    };

    const missing = await tokenRequest(base);
    assert.strictEqual(missing.status, 401);
    assert.strictEqual(JSON.parse(missing.data).error, 'invalid_client');
    assert.strictEqual(JSON.parse(missing.data).error_description, 'Client authentication failed.');
    assert.strictEqual(missing.headers['www-authenticate'], undefined);

    const empty = await tokenRequest(Object.assign({}, base, { client_secret: '' }));
    assert.strictEqual(empty.status, 401);
    assert.strictEqual(JSON.parse(empty.data).error, 'invalid_client');

    const wrong = await tokenRequest(Object.assign({}, base, { client_secret: 'nope' }));
    assert.strictEqual(wrong.status, 401);
    assert.strictEqual(JSON.parse(wrong.data).error, 'invalid_client');
    assert.strictEqual(wrong.headers['www-authenticate'], undefined);

    const wrongMethod = await tokenRequest(base, { Authorization: basic(post.client_id, post.client_secret) });
    assert.strictEqual(wrongMethod.status, 401);
    assert.strictEqual(JSON.parse(wrongMethod.data).error, 'invalid_client');
    assert.match(wrongMethod.headers['www-authenticate'], /^Basic realm="hebrew-lessons"/);

    const basicClient = await register('client_secret_basic', 'Basic client');
    const basicPkce = pkce();
    const basicCode = await authorize(basicClient.client_id, basicPkce.challenge);
    const bodyOnly = await tokenRequest({
        grant_type: 'authorization_code', code: basicCode, client_id: basicClient.client_id,
        client_secret: basicClient.client_secret, redirect_uri: CB, code_verifier: basicPkce.verifier
    });
    assert.strictEqual(bodyOnly.status, 401);
    assert.strictEqual(JSON.parse(bodyOnly.data).error, 'invalid_client');
    assert.match(bodyOnly.headers['www-authenticate'], /^Basic /);

    const badBasic = await tokenRequest({
        grant_type: 'authorization_code', code: basicCode, redirect_uri: CB, code_verifier: basicPkce.verifier
    }, { Authorization: basic(basicClient.client_id, 'not-the-secret') });
    assert.strictEqual(badBasic.status, 401);
    assert.strictEqual(JSON.parse(badBasic.data).error, 'invalid_client');
    assert.match(badBasic.headers['www-authenticate'], /^Basic realm="hebrew-lessons"/);

    const still = await tokenRequest(Object.assign({}, base, { client_secret: post.client_secret }));
    assert.strictEqual(still.status, 200, 'a rejected secret does not burn the code');
    const refreshed = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: JSON.parse(still.data).refresh_token, client_id: post.client_id
    });
    assert.strictEqual(refreshed.status, 401);
    assert.strictEqual(JSON.parse(refreshed.data).error, 'invalid_client');
    const refreshedOk = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: JSON.parse(still.data).refresh_token,
        client_id: post.client_id, client_secret: post.client_secret
    });
    assert.strictEqual(refreshedOk.status, 200, 'a rejected refresh does not rotate the token');
});

test('accepts client_secret_post and client_secret_basic', async () => {
    const post = await register('client_secret_post', 'Post ok');
    assert.strictEqual(post.token_endpoint_auth_method, 'client_secret_post');
    assert.match(post.client_secret, /^[0-9a-f]{48}$/);
    assert.strictEqual(post.client_secret_expires_at, 0);
    const postPkce = pkce();
    const postCode = await authorize(post.client_id, postPkce.challenge);
    const postTok = await tokenRequest({
        grant_type: 'authorization_code', code: postCode, client_id: post.client_id,
        client_secret: post.client_secret, redirect_uri: CB, code_verifier: postPkce.verifier
    });
    assert.strictEqual(postTok.status, 200);
    const postTokens = JSON.parse(postTok.data);
    assert.strictEqual(postTokens.token_type, 'Bearer');
    const postRefresh = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: postTokens.refresh_token,
        client_id: post.client_id, client_secret: post.client_secret
    });
    assert.strictEqual(postRefresh.status, 200);

    const basicClient = await register('client_secret_basic', 'Basic ok');
    assert.strictEqual(basicClient.token_endpoint_auth_method, 'client_secret_basic');
    const basicPkce = pkce();
    const basicCode = await authorize(basicClient.client_id, basicPkce.challenge);
    const basicTok = await tokenRequest({
        grant_type: 'authorization_code', code: basicCode, client_id: basicClient.client_id,
        redirect_uri: CB, code_verifier: basicPkce.verifier
    }, { Authorization: basic(basicClient.client_id, basicClient.client_secret) });
    assert.strictEqual(basicTok.status, 200);
    assert.strictEqual(JSON.parse(basicTok.data).token_type, 'Bearer');
    const basicRefresh = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: JSON.parse(basicTok.data).refresh_token,
        client_id: basicClient.client_id
    }, { Authorization: basic(basicClient.client_id, basicClient.client_secret) });
    assert.strictEqual(basicRefresh.status, 200);
});

test('accepts a public client with no secret', async () => {
    const as = JSON.parse((await call('GET', '/.well-known/oauth-authorization-server')).data);
    assert.deepStrictEqual(as.token_endpoint_auth_methods_supported, ['none', 'client_secret_post', 'client_secret_basic']);

    const pub = await register(null, 'Public client');
    assert.strictEqual(pub.token_endpoint_auth_method, 'none');
    assert.strictEqual(pub.client_secret, undefined);
    const { verifier, challenge } = pkce();
    const code = await authorize(pub.client_id, challenge);
    const tok = await tokenRequest({
        grant_type: 'authorization_code', code, client_id: pub.client_id,
        redirect_uri: CB, code_verifier: verifier
    });
    assert.strictEqual(tok.status, 200);
    const tokens = JSON.parse(tok.data);
    assert.strictEqual(tokens.token_type, 'Bearer');
    const oauth = require('../mcp/oauth');
    assert.strictEqual(oauth.verifyAccessToken(tokens.access_token).cid, pub.client_id);
    const refreshed = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: tokens.refresh_token, client_id: pub.client_id
    });
    assert.strictEqual(refreshed.status, 200);
    assert.strictEqual(oauth.verifyAccessToken(JSON.parse(refreshed.data).access_token).cid, pub.client_id);
});

test('rejects an authorization code from a different client', async () => {
    const owner = await register(null, 'Code owner');
    const other = await register(null, 'Code other');
    const { verifier, challenge } = pkce();
    const code = await authorize(owner.client_id, challenge);
    const stolen = await tokenRequest({
        grant_type: 'authorization_code', code, client_id: other.client_id,
        redirect_uri: CB, code_verifier: verifier
    });
    assert.strictEqual(stolen.status, 400);
    assert.strictEqual(JSON.parse(stolen.data).error, 'invalid_grant');
    assert.strictEqual(JSON.parse(stolen.data).error_description, 'client_id or redirect_uri does not match the code.');

    const kept = await tokenRequest({
        grant_type: 'authorization_code', code, client_id: owner.client_id,
        redirect_uri: CB, code_verifier: verifier
    });
    assert.strictEqual(kept.status, 200, 'the issuing client can still redeem the code');
    assert.strictEqual(require('../mcp/oauth').verifyAccessToken(JSON.parse(kept.data).access_token).cid, owner.client_id);
});

test('rejects a refresh token from a different client', async () => {
    const owner = await register(null, 'Refresh owner');
    const other = await register(null, 'Refresh other');
    const { verifier, challenge } = pkce();
    const code = await authorize(owner.client_id, challenge);
    const tok = await tokenRequest({
        grant_type: 'authorization_code', code, client_id: owner.client_id,
        redirect_uri: CB, code_verifier: verifier
    });
    assert.strictEqual(tok.status, 200);
    const refresh = JSON.parse(tok.data).refresh_token;
    const stolen = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: refresh, client_id: other.client_id
    });
    assert.strictEqual(stolen.status, 400);
    assert.strictEqual(JSON.parse(stolen.data).error, 'invalid_grant');
    assert.strictEqual(JSON.parse(stolen.data).error_description, 'client_id does not match the refresh token.');

    const kept = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: refresh, client_id: owner.client_id
    });
    assert.strictEqual(kept.status, 200, 'the issuing client can still refresh');
    assert.strictEqual(require('../mcp/oauth').verifyAccessToken(JSON.parse(kept.data).access_token).cid, owner.client_id);
});

test('legacy stored client and token still work', async () => {
    const bound = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: legacyRefresh, client_id: LEGACY_CLIENT
    });
    assert.strictEqual(bound.status, 200, bound.data);
    const oauth = require('../mcp/oauth');
    assert.strictEqual(oauth.verifyAccessToken(JSON.parse(bound.data).access_token).cid, LEGACY_CLIENT);

    const page = await call('GET', '/authorize?' + form({
        response_type: 'code', client_id: LEGACY_CLIENT, redirect_uri: CB,
        code_challenge: pkce().challenge, code_challenge_method: 'S256'
    }));
    assert.strictEqual(page.status, 200);
    assert.match(page.data, /Legacy MCP client/);

    const { verifier, challenge } = pkce();
    const code = await authorize(LEGACY_CLIENT, challenge);
    const exchanged = await tokenRequest({
        grant_type: 'authorization_code', code, client_id: LEGACY_CLIENT,
        redirect_uri: CB, code_verifier: verifier
    });
    assert.strictEqual(exchanged.status, 200);

    const claimed = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: unboundRefresh, client_id: LEGACY_CLIENT
    });
    assert.strictEqual(claimed.status, 200);
    const claimedRefresh = JSON.parse(claimed.data).refresh_token;
    const store = JSON.parse(fs.readFileSync(path.join(dataDir, 'mcp-oauth-state.json'), 'utf8'));
    assert.strictEqual(store.refresh[tokenHash(unboundRefresh)], undefined, 'the unbound grant is spent');
    assert.strictEqual(store.refresh[tokenHash(claimedRefresh)].cid, LEGACY_CLIENT, 'rotation stores the binding');
    assert.strictEqual(store.clients[LEGACY_CLIENT].am, 'none');
    assert.strictEqual(store.clients[LEGACY_CLIENT].sh, undefined);
    assert.strictEqual(store.clients[LEGACY_CONF].am, 'client_secret_post');
    assert.strictEqual(store.clients[LEGACY_CONF].sh, legacySecretHash);

    const other = await register(null, 'Not the legacy client');
    const rebound = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: claimedRefresh, client_id: other.client_id
    });
    assert.strictEqual(rebound.status, 400);
    assert.strictEqual(JSON.parse(rebound.data).error, 'invalid_grant');
    const again = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: claimedRefresh, client_id: LEGACY_CLIENT
    });
    assert.strictEqual(again.status, 200);

    const missingSecret = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: confRefresh, client_id: LEGACY_CONF
    });
    assert.strictEqual(missingSecret.status, 401);
    assert.strictEqual(JSON.parse(missingSecret.data).error, 'invalid_client');
    const withSecret = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: confRefresh, client_id: LEGACY_CONF,
        client_secret: LEGACY_SECRET
    });
    assert.strictEqual(withSecret.status, 200, 'a stored confidential client keeps its secret');
    assert.strictEqual(oauth.verifyAccessToken(JSON.parse(withSecret.data).access_token).cid, LEGACY_CONF);

    const foreign = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: hmacRefresh, client_id: other.client_id
    });
    assert.strictEqual(foreign.status, 400);
    assert.strictEqual(JSON.parse(foreign.data).error, 'invalid_grant');
    const hmacOk = await tokenRequest({
        grant_type: 'refresh_token', refresh_token: hmacRefresh, client_id: hmacClient
    });
    assert.strictEqual(hmacOk.status, 200, 'a legacy HMAC refresh token still verifies');
    assert.strictEqual(oauth.verifyAccessToken(JSON.parse(hmacOk.data).access_token).cid, hmacClient);
});
