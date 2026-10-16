// MCP authorization (OAuth 2.1 + PKCE + dynamic client registration).
//
// When GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET are set, /authorize sends the
// person to Google. The email must be in MCP_OAUTH_ALLOWED_EMAILS and already linked to an
// active app user (the same Google email as site sign-in). The issued token carries that
// user id. The Google client must list {MCP_PUBLIC_URL}/oauth/google/callback as a redirect
// URI. The Google leg is a confidential-client code exchange (client secret, scopes email
// and profile). Cursor cannot send the site cookie, so this browser step is the link.
//
// Without those two Google variables, /authorize still asks for MCP_AUTH_TOKEN.
// Registered clients and issued tokens are stored as short opaque IDs (UUID client_id,
// 43-char code/access/refresh) persisted under DATA_DIR so Cursor's OAuth broker never
// rejects oversized HMAC blobs, while HMAC tokens remain accepted for compatibility.
//
// Enabled only when MCP_PUBLIC_URL (the https origin clients use) and MCP_AUTH_TOKEN are both set.
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { AsyncLocalStorage } = require('async_hooks');
const google = require('../server/auth/google');
const accounts = require('../server/accounts');

const principals = new AsyncLocalStorage();

const ACCESS_TTL = 3600;
const REFRESH_TTL = 30 * 24 * 3600;
const CODE_TTL = 600;
const REQUEST_TTL = 600;
const CLIENT_TTL = 400 * 24 * 3600;
const MAX_FORM = 64 * 1024;
const MAX_FAILS = 10;
const FAIL_WINDOW_MS = 10 * 60 * 1000;
const SCOPE = 'mcp';

// Registered clients may only use these redirect URIs (plus http loopback on any port).
// Stops a dynamically registered client from pointing the code at an attacker's site.
const DEFAULT_REDIRECTS = [
    'cursor://anysphere.cursor-mcp/oauth/callback',
    'https://www.cursor.com/agents/mcp/oauth/callback',
    'https://cursor.com/agents/mcp/oauth/callback',
    'http://localhost:8787/callback'
];

const usedCodes = new Map();
const codes = new Map();
const pendingReqs = new Map();
let failures = [];
let cachedStore = null;
let cachedStoreKey = '';

function baseUrl() {
    return String(process.env.MCP_PUBLIC_URL || '').replace(/\/+$/, '');
}

function enabled() {
    return Boolean(baseUrl() && process.env.MCP_AUTH_TOKEN);
}

function googleConfigured() {
    return google.configured();
}

function allowedEmails() {
    return String(process.env.MCP_OAUTH_ALLOWED_EMAILS || '')
        .split(',').map(function (item) { return item.trim().toLowerCase(); }).filter(Boolean);
}

function googleRedirectUri() {
    return baseUrl() + '/oauth/google/callback';
}

function allowedRedirects() {
    const extra = String(process.env.MCP_OAUTH_REDIRECT_URIS || '')
        .split(',').map(s => s.trim()).filter(Boolean);
    return DEFAULT_REDIRECTS.concat(extra);
}

function redirectAllowed(uri) {
    let u;
    try { u = new URL(uri); } catch (err) { return false; }
    if (u.hash) return false;
    if (allowedRedirects().includes(uri)) return true;
    return u.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(u.hostname);
}

function key() {
    return crypto.createHmac('sha256', process.env.MCP_AUTH_TOKEN).update('hebrew-mcp-oauth-v1').digest();
}

function tokenHash(raw) {
    return crypto.createHmac('sha256', key()).update(String(raw)).digest('hex');
}

function now() {
    return Math.floor(Date.now() / 1000);
}

function storeFile() {
    const dir = process.env.DATA_DIR;
    if (!dir) return '';
    return path.join(dir, 'mcp-oauth-state.json');
}

function loadStore() {
    const file = storeFile();
    const cacheKey = file + '|' + String(process.env.MCP_AUTH_TOKEN || '');
    if (cachedStore && cachedStoreKey === cacheKey) return cachedStore;
    const empty = { clients: {}, access: {}, refresh: {} };
    if (file && fs.existsSync(file)) {
        try {
            const raw = JSON.parse(fs.readFileSync(file, 'utf8'));
            if (raw && typeof raw === 'object') {
                if (raw.clients && typeof raw.clients === 'object') empty.clients = raw.clients;
                if (raw.access && typeof raw.access === 'object') empty.access = raw.access;
                if (raw.refresh && typeof raw.refresh === 'object') empty.refresh = raw.refresh;
            }
        } catch (err) {
            console.error('mcp oauth state load failed');
        }
    }
    cachedStore = empty;
    cachedStoreKey = cacheKey;
    return empty;
}

function saveStore() {
    const file = storeFile();
    if (!file || !cachedStore) return;
    try {
        fs.mkdirSync(path.dirname(file), { recursive: true });
        const tmp = file + '.tmp-' + process.pid;
        fs.writeFileSync(tmp, JSON.stringify(cachedStore), { mode: 0o600 });
        fs.renameSync(tmp, file);
    } catch (err) {
        console.error('mcp oauth state save failed');
    }
}

function sweepStore() {
    const t = now();
    usedCodes.forEach((exp, jti) => { if (exp <= t) usedCodes.delete(jti); });
    codes.forEach((rec, code) => { if (rec.exp <= t) codes.delete(code); });
    pendingReqs.forEach((rec, id) => { if (rec.exp <= t) pendingReqs.delete(id); });
    const store = loadStore();
    let dirty = false;
    Object.keys(store.access).forEach(k => {
        if (store.access[k].exp <= t) { delete store.access[k]; dirty = true; }
    });
    Object.keys(store.refresh).forEach(k => {
        if (store.refresh[k].exp <= t) { delete store.refresh[k]; dirty = true; }
    });
    Object.keys(store.clients).forEach(k => {
        if (store.clients[k].exp && store.clients[k].exp <= t) { delete store.clients[k]; dirty = true; }
    });
    if (dirty) saveStore();
}

function sign(type, ttl, claims) {
    const payload = Object.assign({}, claims, { t: type, exp: now() + ttl });
    const body = Buffer.from(JSON.stringify(payload)).toString('base64url');
    const sig = crypto.createHmac('sha256', key()).update(body).digest('base64url');
    return body + '.' + sig;
}

function verify(token, type) {
    if (typeof token !== 'string') return null;
    const parts = token.split('.');
    if (parts.length !== 2) return null;
    const want = crypto.createHmac('sha256', key()).update(parts[0]).digest();
    let got;
    try { got = Buffer.from(parts[1], 'base64url'); } catch (err) { return null; }
    if (got.length !== want.length || !crypto.timingSafeEqual(got, want)) return null;
    let payload;
    try { payload = JSON.parse(Buffer.from(parts[0], 'base64url').toString('utf8')); } catch (err) { return null; }
    if (!payload || payload.t !== type || !(payload.exp > now())) return null;
    return payload;
}

function findClient(clientId) {
    if (typeof clientId !== 'string' || !clientId) return null;
    const store = loadStore();
    const stored = store.clients[clientId];
    if (stored && (!stored.exp || stored.exp > now())) return stored;
    return verify(clientId, 'client');
}

function basicAuth(req) {
    const m = /^Basic\s+(\S+)$/i.exec(String(req && req.headers && req.headers.authorization || ''));
    if (!m) return null;
    try {
        const raw = Buffer.from(m[1], 'base64').toString('utf8');
        const idx = raw.indexOf(':');
        if (idx < 0) return { clientId: decodeURIComponent(raw), clientSecret: '' };
        return {
            clientId: decodeURIComponent(raw.slice(0, idx)),
            clientSecret: decodeURIComponent(raw.slice(idx + 1))
        };
    } catch (err) {
        return null;
    }
}

function sameSecret(a, b) {
    const x = crypto.createHash('sha256').update(String(a)).digest();
    const y = crypto.createHash('sha256').update(String(b)).digest();
    return crypto.timingSafeEqual(x, y);
}

function resolvePayloadUser(payload) {
    if (!payload) return null;
    if (payload.uid == null) {
        if (googleConfigured() && allowedEmails().length === 1) {
            const linked = accounts.findLinkedGoogleUser({ email: allowedEmails()[0] });
            if (linked) return Object.assign({}, payload, { uid: linked.id, un: linked.username });
        }
        return payload;
    }
    const user = accounts.findActiveUser(payload.uid);
    if (!user || (payload.un && user.username !== payload.un)) return null;
    return payload;
}

function verifyAccessToken(token) {
    if (!enabled() || typeof token !== 'string' || !token) return null;
    const store = loadStore();
    const h = tokenHash(token);
    const stored = store.access[h];
    if (stored && stored.exp > now()) {
        return resolvePayloadUser(stored);
    }
    const payload = verify(token, 'access');
    if (!payload) return null;
    return resolvePayloadUser(payload);
}

function runAs(principal, fn) {
    return principals.run(principal || null, fn);
}

function currentUser() {
    return principals.getStore() || null;
}

function resourceMetadataUrl() {
    return baseUrl() + '/.well-known/oauth-protected-resource';
}

// Browser clients (the Cursor / Grok callback page) exchange the code with fetch().
// Without these headers the browser hides a successful token response and shows server_error.
function withCors(headers) {
    return Object.assign({
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization, Accept, MCP-Protocol-Version',
        'Access-Control-Allow-Methods': 'GET, POST, OPTIONS'
    }, headers || {});
}

function json(res, status, body, extra) {
    res.writeHead(status, withCors(Object.assign({
        'Content-Type': 'application/json; charset=utf-8',
        'Cache-Control': 'no-store',
        'Pragma': 'no-cache'
    }, extra || {})));
    res.end(JSON.stringify(body));
}

function oauthError(res, status, error, description) {
    json(res, status, { error, error_description: description });
}

function esc(value) {
    return String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function page(res, status, title, bodyHtml) {
    res.writeHead(status, withCors({
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'no-store',
        'X-Frame-Options': 'DENY',
        'Referrer-Policy': 'no-referrer',
        'X-Content-Type-Options': 'nosniff',
        'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'"
    }));
    res.end('<!doctype html><html lang="en"><head><meta charset="utf-8">' +
        '<meta name="viewport" content="width=device-width,initial-scale=1">' +
        '<title>' + esc(title) + '</title><style>' +
        'body{font:16px system-ui,sans-serif;max-width:30rem;margin:12vh auto;padding:0 1rem;color:#1b1b1b}' +
        'input[type=password]{width:100%;padding:.6rem;margin:.4rem 0 1rem;font-size:1rem;box-sizing:border-box}' +
        'button{padding:.6rem 1.2rem;font-size:1rem;margin-right:.5rem}.err{color:#b00020}.dim{color:#555;font-size:.9rem}' +
        'code{background:#eee;padding:0 .25rem;word-break:break-all}' +
        '</style></head><body>' + bodyHtml + '</body></html>');
}

function readRaw(req) {
    return new Promise((resolve, reject) => {
        const chunks = [];
        let size = 0;
        req.on('data', chunk => {
            size += chunk.length;
            if (size > MAX_FORM) {
                reject(Object.assign(new Error('Request body is too large.'), { status: 413 }));
                req.destroy();
                return;
            }
            chunks.push(chunk);
        });
        req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
        req.on('error', reject);
    });
}

async function readForm(req) {
    const params = new URLSearchParams(await readRaw(req));
    const out = {};
    params.forEach((value, name) => { if (!(name in out)) out[name] = value; });
    return out;
}

function pkceMatches(verifier, challenge) {
    if (typeof verifier !== 'string' || !/^[A-Za-z0-9._~-]{43,128}$/.test(verifier)) return false;
    const digest = crypto.createHash('sha256').update(verifier).digest('base64url');
    return sameSecret(digest, challenge);
}

function metadata(res) {
    const base = baseUrl();
    json(res, 200, {
        issuer: base,
        authorization_endpoint: base + '/authorize',
        token_endpoint: base + '/token',
        registration_endpoint: base + '/register',
        response_types_supported: ['code'],
        grant_types_supported: ['authorization_code', 'refresh_token'],
        code_challenge_methods_supported: ['S256'],
        token_endpoint_auth_methods_supported: ['none', 'client_secret_post', 'client_secret_basic'],
        scopes_supported: [SCOPE]
    });
}

function protectedResource(res) {
    json(res, 200, {
        resource: baseUrl() + '/mcp',
        authorization_servers: [baseUrl()],
        scopes_supported: [SCOPE],
        bearer_methods_supported: ['header']
    });
}

async function register(req, res) {
    let body;
    try {
        body = JSON.parse(await readRaw(req) || '{}');
    } catch (err) {
        return oauthError(res, err.status || 400, 'invalid_client_metadata', 'Body must be JSON.');
    }
    const uris = body && body.redirect_uris;
    if (!Array.isArray(uris) || uris.length < 1 || uris.length > 10 || !uris.every(u => typeof u === 'string')) {
        return oauthError(res, 400, 'invalid_redirect_uri', 'redirect_uris must be 1 to 10 strings.');
    }
    const bad = uris.find(u => !redirectAllowed(u));
    if (bad) {
        console.error('mcp oauth register rejected redirect_uri');
        return oauthError(res, 400, 'invalid_redirect_uri', 'redirect_uri is not allowed by this server: ' + bad.slice(0, 120));
    }
    sweepStore();
    const name = String(body.client_name || 'MCP client').slice(0, 80);
    const requestedMethod = body.token_endpoint_auth_method;
    const authMethod = (requestedMethod === 'client_secret_post' || requestedMethod === 'client_secret_basic' || requestedMethod === 'none')
        ? requestedMethod
        : 'none';
    const clientId = crypto.randomUUID();
    const issuedAt = now();
    const clientSecret = authMethod === 'none' ? undefined : crypto.randomBytes(24).toString('hex');
    const store = loadStore();
    store.clients[clientId] = {
        ru: uris,
        nm: name,
        am: authMethod,
        sh: clientSecret ? tokenHash(clientSecret) : undefined,
        exp: issuedAt + CLIENT_TTL
    };
    saveStore();
    console.error('mcp oauth register ok ' + name + ' id=' + clientId + ' method=' + authMethod);
    const reply = {
        client_id: clientId,
        client_id_issued_at: issuedAt,
        client_name: name,
        redirect_uris: uris,
        grant_types: ['authorization_code', 'refresh_token'],
        response_types: ['code'],
        token_endpoint_auth_method: authMethod
    };
    if (clientSecret) {
        reply.client_secret = clientSecret;
        reply.client_secret_expires_at = 0;
    }
    json(res, 201, reply);
}

function resolveAuthorizeRequest(q) {
    const client = findClient(q.client_id);
    if (!client) return { fatal: 'Unknown or expired client. Register again.' };
    if (!q.redirect_uri || !client.ru.includes(q.redirect_uri) || !redirectAllowed(q.redirect_uri)) {
        return { fatal: 'redirect_uri does not match this client.' };
    }
    if (q.response_type !== 'code') return { redirectError: 'unsupported_response_type', client };
    if (!q.code_challenge || q.code_challenge_method !== 'S256' || !/^[A-Za-z0-9_-]{43}$/.test(q.code_challenge)) {
        return { redirectError: 'invalid_request', description: 'PKCE with S256 is required.', client };
    }
    if (q.resource && q.resource.replace(/\/+$/, '') !== baseUrl() + '/mcp' && q.resource.replace(/\/+$/, '') !== baseUrl()) {
        return { redirectError: 'invalid_target', client };
    }
    return { client };
}

function sendRedirect(res, target, plainTitle) {
    const isWeb = /^https?:\/\//i.test(target);
    if (isWeb) {
        res.writeHead(303, withCors({ Location: target, 'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer' }));
        res.end();
        return;
    }
    // Custom-scheme callbacks (cursor://): browsers handle a page better than a 302 after a POST.
    res.writeHead(200, withCors({
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'no-store',
        'Referrer-Policy': 'no-referrer',
        'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'"
    }));
    res.end('<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=' + esc(target) +
        '"><title>' + esc(plainTitle) + '</title></head><body style="font:16px system-ui,sans-serif;max-width:30rem;margin:12vh auto;padding:0 1rem">' +
        '<p>' + esc(plainTitle) + '</p><p><a href="' + esc(target) + '">Return to the app</a></p></body></html>');
}

function redirectWith(res, redirectUri, params) {
    const url = new URL(redirectUri);
    Object.keys(params).forEach(k => { if (params[k] !== undefined && params[k] !== '') url.searchParams.set(k, params[k]); });
    sendRedirect(res, url.toString(), params.error ? 'Access was not granted.' : 'Signed in. Returning to the app.');
}

function savePendingRequest(q) {
    sweepStore();
    const id = crypto.randomBytes(24).toString('base64url');
    pendingReqs.set(id, {
        cid: q.client_id,
        ru: q.redirect_uri,
        cc: q.code_challenge,
        st: q.state || '',
        exp: now() + REQUEST_TTL
    });
    return id;
}

function loadPendingRequest(stateParam) {
    if (typeof stateParam !== 'string' || !stateParam) return null;
    const mem = pendingReqs.get(stateParam);
    if (mem) {
        pendingReqs.delete(stateParam);
        if (mem.exp > now()) return mem;
        return null;
    }
    return verify(stateParam, 'req');
}

function renderConsent(res, resolved, q, message, status) {
    const requestToken = sign('req', REQUEST_TTL, {
        cid: q.client_id, ru: q.redirect_uri, cc: q.code_challenge, st: q.state || ''
    });
    let host;
    try { host = new URL(q.redirect_uri).host || new URL(q.redirect_uri).protocol; } catch (err) { host = q.redirect_uri; }
    page(res, status || 200, 'Sign in to Hebrew lessons MCP',
        '<h1>Sign in</h1>' +
        '<p><strong>' + esc(resolved.client.nm) + '</strong> wants to read and write Hebrew lessons on this server.</p>' +
        '<p class="dim">It will be sent back to <code>' + esc(q.redirect_uri) + '</code> (' + esc(host) + ').' +
        ' Continue only if you started this from your own app.</p>' +
        (message ? '<p class="err">' + esc(message) + '</p>' : '') +
        '<form method="post" action="/authorize">' +
        '<input type="hidden" name="req" value="' + esc(requestToken) + '">' +
        '<label>MCP access token<input type="password" name="password" autocomplete="off" autofocus></label>' +
        '<button type="submit" name="action" value="approve">Approve</button>' +
        '<button type="submit" name="action" value="deny">Deny</button></form>');
}

function lockedOut() {
    const cutoff = Date.now() - FAIL_WINDOW_MS;
    failures = failures.filter(t => t > cutoff);
    return failures.length >= MAX_FAILS;
}

function issueAuthCode(q, user) {
    sweepStore();
    const code = crypto.randomBytes(32).toString('base64url');
    const record = {
        jti: code,
        cid: q.client_id,
        ru: q.redirect_uri,
        cc: q.code_challenge,
        exp: now() + CODE_TTL
    };
    if (user) {
        record.uid = user.id;
        record.un = user.username;
    }
    codes.set(code, record);
    return code;
}

function loadAuthCode(code) {
    if (typeof code !== 'string' || !code) return null;
    const mem = codes.get(code);
    if (mem) {
        if (mem.exp > now()) return mem;
        codes.delete(code);
        return null;
    }
    return verify(code, 'code');
}

function tokenUser(grant) {
    if (grant.uid == null) {
        if (googleConfigured() && allowedEmails().length === 1) {
            const linked = accounts.findLinkedGoogleUser({ email: allowedEmails()[0] });
            if (linked) return { uid: linked.id, un: linked.username };
        }
        return {};
    }
    const user = accounts.findActiveUser(grant.uid);
    if (!user || (grant.un && user.username !== grant.un)) return null;
    return { uid: user.id, un: user.username };
}

function startGoogle(res, q) {
    if (allowedEmails().length === 0) {
        return page(res, 503, 'Sign-in is not configured',
            '<h1>Sign-in is not configured</h1><p class="err">No allowed email is set.</p>');
    }
    const state = savePendingRequest(q);
    let location;
    try {
        location = google.authorizationUrl({
            redirectUri: googleRedirectUri(),
            state: state
        });
    } catch (err) {
        return page(res, 500, 'Sign-in is not configured', '<h1>Sign-in is not configured</h1><p class="err">The Google sign-in address is invalid.</p>');
    }
    res.writeHead(302, { Location: location, 'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer' });
    res.end();
}

async function googleIdentity(code) {
    return google.identityFromCode({
        code: code,
        redirectUri: googleRedirectUri()
    });
}

async function googleCallback(res, url) {
    if (!googleConfigured()) return page(res, 404, 'Not found', '<p>Not found.</p>');
    const pending = loadPendingRequest(url.searchParams.get('state'));
    if (!pending) {
        console.error('mcp google callback expired state');
        return page(res, 400, 'Expired', '<h1>Sign-in expired</h1><p>Start again from the app.</p>');
    }
    const q = {
        client_id: pending.cid, redirect_uri: pending.ru, code_challenge: pending.cc, state: pending.st,
        response_type: 'code', code_challenge_method: 'S256'
    };
    const resolved = resolveAuthorizeRequest(q);
    if (resolved.fatal || resolved.redirectError) {
        console.error('mcp google callback invalid request');
        return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">Invalid request.</p>');
    }
    const err = url.searchParams.get('error');
    if (err) {
        const allowed = ['invalid_request', 'unauthorized_client', 'access_denied', 'unsupported_response_type', 'invalid_scope', 'server_error', 'temporarily_unavailable'];
        const code = allowed.indexOf(err) === -1 ? 'server_error' : err;
        console.error('mcp google callback rejected ' + code);
        return redirectWith(res, q.redirect_uri, { error: code, state: q.state });
    }
    const code = url.searchParams.get('code');
    if (!code) return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">Google did not return a code.</p>');
    let identity;
    try {
        identity = await googleIdentity(code);
    } catch (e) {
        console.error('mcp google identity failed ' + (e && e.statusCode ? e.statusCode : ''));
        return page(res, 502, 'Google sign-in failed', '<h1>Google sign-in failed</h1><p class="err">Try again from the app.</p>');
    }
    const email = identity && identity.email;
    if (!email || allowedEmails().indexOf(email) === -1) {
        console.error('mcp google callback rejected access_denied ' + (email ? 'not_allowed' : 'unverified'));
        return redirectWith(res, q.redirect_uri, { error: 'access_denied', state: q.state });
    }
    const user = accounts.findLinkedGoogleUser({ email: email, sub: identity && identity.sub });
    if (!user) {
        console.error('mcp google callback rejected access_denied not_linked');
        return redirectWith(res, q.redirect_uri, { error: 'access_denied', state: q.state });
    }
    const issuedCode = issueAuthCode(q, user);
    console.error('mcp google callback ok uid=' + user.id + ' cid=' + q.client_id + ' ru=' + q.redirect_uri + ' code_len=' + issuedCode.length);
    redirectWith(res, q.redirect_uri, { code: issuedCode, state: q.state });
}

function authorizeGet(req, res, url) {
    const q = {};
    url.searchParams.forEach((value, name) => { if (!(name in q)) q[name] = value; });
    const resolved = resolveAuthorizeRequest(q);
    if (resolved.fatal) {
        console.error('mcp oauth authorize fatal: ' + resolved.fatal);
        return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">' + esc(resolved.fatal) + '</p>');
    }
    if (resolved.redirectError) {
        console.error('mcp oauth authorize ' + resolved.redirectError);
        return redirectWith(res, q.redirect_uri, { error: resolved.redirectError, error_description: resolved.description, state: q.state });
    }
    console.error('mcp oauth authorize ok cid=' + q.client_id + ' ru=' + q.redirect_uri + ' state_len=' + String(q.state || '').length);
    if (googleConfigured()) return startGoogle(res, q);
    renderConsent(res, resolved, q);
}

async function authorizePost(req, res) {
    if (googleConfigured()) {
        return page(res, 400, 'Use Google', '<h1>Use Google sign-in</h1><p>Start again from the app.</p>');
    }
    let form;
    try { form = await readForm(req); } catch (err) {
        return page(res, err.status || 400, 'Bad request', '<p class="err">Bad request.</p>');
    }
    const pending = loadPendingRequest(form.req);
    if (!pending) {
        return page(res, 400, 'Expired', '<h1>Sign-in expired</h1><p>Start again from the app.</p>');
    }
    const q = { client_id: pending.cid, redirect_uri: pending.ru, code_challenge: pending.cc, state: pending.st,
        response_type: 'code', code_challenge_method: 'S256' };
    const resolved = resolveAuthorizeRequest(q);
    if (resolved.fatal || resolved.redirectError) {
        return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">Invalid request.</p>');
    }
    if (form.action === 'deny') {
        return redirectWith(res, q.redirect_uri, { error: 'access_denied', state: q.state });
    }
    if (lockedOut()) {
        return renderConsent(res, resolved, q, 'Too many failed attempts. Wait ten minutes.', 429);
    }
    if (!form.password || !sameSecret(form.password, process.env.MCP_AUTH_TOKEN)) {
        failures.push(Date.now());
        return renderConsent(res, resolved, q, 'That token is not correct.', 401);
    }
    redirectWith(res, q.redirect_uri, { code: issueAuthCode(q), state: q.state });
}

function issueTokens(res, clientId, userClaims) {
    sweepStore();
    const claims = Object.assign({ cid: clientId, scope: SCOPE }, userClaims || {});
    const accessToken = crypto.randomBytes(32).toString('base64url');
    const refreshToken = crypto.randomBytes(32).toString('base64url');
    const t = now();
    const store = loadStore();
    store.access[tokenHash(accessToken)] = Object.assign({}, claims, { exp: t + ACCESS_TTL });
    store.refresh[tokenHash(refreshToken)] = Object.assign({}, claims, { exp: t + REFRESH_TTL });
    saveStore();
    json(res, 200, {
        access_token: accessToken,
        token_type: 'Bearer',
        expires_in: ACCESS_TTL,
        refresh_token: refreshToken,
        scope: SCOPE
    });
}

function loadRefreshGrant(token) {
    if (typeof token !== 'string' || !token) return null;
    const store = loadStore();
    const h = tokenHash(token);
    const stored = store.refresh[h];
    if (stored && stored.exp > now()) return { grant: stored, hash: h };
    const verified = verify(token, 'refresh');
    if (verified) return { grant: verified, hash: '' };
    return null;
}

async function token(req, res) {
    let form;
    try { form = await readForm(req); } catch (err) {
        console.error('mcp token invalid_request body');
        return oauthError(res, err.status || 400, 'invalid_request', 'Bad body.');
    }
    const basic = basicAuth(req);
    if (form.grant_type === 'authorization_code') {
        const grant = loadAuthCode(form.code);
        if (!grant) {
            console.error('mcp token authorization_code invalid_grant code');
            return oauthError(res, 400, 'invalid_grant', 'Code is invalid or expired.');
        }
        sweepStore();
        if (usedCodes.has(grant.jti)) {
            console.error('mcp token authorization_code invalid_grant used_code');
            return oauthError(res, 400, 'invalid_grant', 'Code was already used.');
        }
        const givenClientId = form.client_id || (basic && basic.clientId) || grant.cid;
        const client = findClient(givenClientId) || findClient(grant.cid);
        const redirectOk = !form.redirect_uri || form.redirect_uri === grant.ru;
        const clientOk = Boolean(client && Array.isArray(client.ru) && client.ru.includes(grant.ru));
        if (!clientOk || !redirectOk) {
            console.error('mcp token authorization_code invalid_grant client_or_redirect');
            return oauthError(res, 400, 'invalid_grant', 'client_id or redirect_uri does not match the code.');
        }
        if (!pkceMatches(form.code_verifier, grant.cc)) {
            console.error('mcp token authorization_code invalid_grant pkce');
            return oauthError(res, 400, 'invalid_grant', 'PKCE verification failed.');
        }
        usedCodes.set(grant.jti, grant.exp);
        codes.delete(form.code);
        const userClaims = tokenUser(grant);
        if (!userClaims) {
            console.error('mcp token authorization_code invalid_grant account');
            return oauthError(res, 400, 'invalid_grant', 'That account can no longer sign in.');
        }
        console.error('mcp token authorization_code ok' + (userClaims.uid != null ? ' uid=' + userClaims.uid : ''));
        return issueTokens(res, givenClientId, userClaims);
    }
    if (form.grant_type === 'refresh_token') {
        const loaded = loadRefreshGrant(form.refresh_token);
        if (!loaded) {
            console.error('mcp token refresh_token invalid_grant token');
            return oauthError(res, 400, 'invalid_grant', 'Refresh token is invalid or expired.');
        }
        const grant = loaded.grant;
        const givenClientId = form.client_id || (basic && basic.clientId) || grant.cid;
        const userClaims = tokenUser(grant);
        if (!userClaims) {
            console.error('mcp token refresh_token invalid_grant account');
            return oauthError(res, 400, 'invalid_grant', 'That account can no longer sign in.');
        }
        if (loaded.hash) {
            const store = loadStore();
            delete store.refresh[loaded.hash];
            saveStore();
        }
        console.error('mcp token refresh_token ok' + (userClaims.uid != null ? ' uid=' + userClaims.uid : ''));
        return issueTokens(res, givenClientId, userClaims);
    }
    console.error('mcp token unsupported_grant_type');
    return oauthError(res, 400, 'unsupported_grant_type', 'Use authorization_code or refresh_token.');
}

// Returns true when it answered the request.
async function handle(req, res) {
    if (!enabled()) return false;
    const url = new URL(req.url || '/', 'http://127.0.0.1');
    const p = url.pathname;
    const get = req.method === 'GET' || req.method === 'HEAD';
    const oauthPath = p === '/register' || p === '/authorize' || p === '/token' || p === '/oauth/google/callback'
        || p.startsWith('/.well-known/');
    if (req.method === 'OPTIONS' && oauthPath) {
        res.writeHead(204, withCors({ 'Access-Control-Max-Age': '600', 'Cache-Control': 'no-store' }));
        res.end();
        return true;
    }
    if (get && (p === '/.well-known/oauth-authorization-server' || p === '/.well-known/openid-configuration')) {
        metadata(res); return true;
    }
    if (get && (p === '/.well-known/oauth-protected-resource' || p === '/.well-known/oauth-protected-resource/mcp')) {
        protectedResource(res); return true;
    }
    if (p === '/register' && req.method === 'POST') { await register(req, res); return true; }
    if (p === '/oauth/google/callback' && req.method === 'GET') { await googleCallback(res, url); return true; }
    if (p === '/authorize' && get) { authorizeGet(req, res, url); return true; }
    if (p === '/authorize' && req.method === 'POST') { await authorizePost(req, res); return true; }
    if (p === '/token' && req.method === 'POST') { await token(req, res); return true; }
    return false;
}

module.exports = { handle, enabled, verifyAccessToken, resourceMetadataUrl, runAs, currentUser };
