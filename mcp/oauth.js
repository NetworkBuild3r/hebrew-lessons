// MCP authorization (OAuth 2.1 + PKCE + dynamic client registration), single owner, no database.
//
// When GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET are set, /authorize sends the
// owner to Google and accepts only the emails in MCP_OAUTH_ALLOWED_EMAILS. Those are the
// same client credentials oauth2-proxy uses for Tautulli. The Google client must list
// {MCP_PUBLIC_URL}/oauth/google/callback as a redirect URI.
//
// Without those two Google variables, /authorize still asks for MCP_AUTH_TOKEN. Everything
// issued (client ids, auth codes, access and refresh tokens) is an HMAC-signed blob keyed
// from MCP_AUTH_TOKEN, so rotating that token revokes every client, grant, and token at once.
//
// Enabled only when MCP_PUBLIC_URL (the https origin clients use) and MCP_AUTH_TOKEN are both set.
const crypto = require('crypto');
const google = require('../server/auth/google');

const ACCESS_TTL = 3600;
const REFRESH_TTL = 30 * 24 * 3600;
const CODE_TTL = 60;
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
    'http://localhost:8787/callback'
];

const usedCodes = new Map();
let failures = [];

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

function now() {
    return Math.floor(Date.now() / 1000);
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

function sameSecret(a, b) {
    const x = crypto.createHash('sha256').update(String(a)).digest();
    const y = crypto.createHash('sha256').update(String(b)).digest();
    return crypto.timingSafeEqual(x, y);
}

function verifyAccessToken(token) {
    if (!enabled()) return false;
    return Boolean(verify(token, 'access'));
}

function resourceMetadataUrl() {
    return baseUrl() + '/.well-known/oauth-protected-resource';
}

function json(res, status, body, extra) {
    res.writeHead(status, Object.assign({
        'Content-Type': 'application/json; charset=utf-8',
        'Cache-Control': 'no-store'
    }, extra || {}));
    res.end(JSON.stringify(body));
}

function oauthError(res, status, error, description) {
    json(res, status, { error, error_description: description });
}

function esc(value) {
    return String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function page(res, status, title, bodyHtml) {
    res.writeHead(status, {
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'no-store',
        'X-Frame-Options': 'DENY',
        'Referrer-Policy': 'no-referrer',
        'X-Content-Type-Options': 'nosniff',
        'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'"
    });
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
        token_endpoint_auth_methods_supported: ['none'],
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
        return oauthError(res, 400, 'invalid_redirect_uri', 'redirect_uri is not allowed by this server: ' + bad.slice(0, 120));
    }
    const name = String(body.client_name || 'MCP client').slice(0, 80);
    const clientId = sign('client', CLIENT_TTL, { ru: uris, nm: name });
    json(res, 201, {
        client_id: clientId,
        client_id_issued_at: now(),
        client_name: name,
        redirect_uris: uris,
        grant_types: ['authorization_code', 'refresh_token'],
        response_types: ['code'],
        token_endpoint_auth_method: 'none'
    });
}

function resolveAuthorizeRequest(q) {
    const client = verify(q.client_id, 'client');
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
        res.writeHead(302, { Location: target, 'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer' });
        res.end();
        return;
    }
    // Custom-scheme callbacks (cursor://): browsers handle a page better than a 302 after a POST.
    res.writeHead(200, {
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'no-store',
        'Referrer-Policy': 'no-referrer',
        'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'"
    });
    res.end('<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=' + esc(target) +
        '"><title>' + esc(plainTitle) + '</title></head><body style="font:16px system-ui,sans-serif;max-width:30rem;margin:12vh auto;padding:0 1rem">' +
        '<p>' + esc(plainTitle) + '</p><p><a href="' + esc(target) + '">Return to the app</a></p></body></html>');
}

function redirectWith(res, redirectUri, params) {
    const url = new URL(redirectUri);
    Object.keys(params).forEach(k => { if (params[k] !== undefined && params[k] !== '') url.searchParams.set(k, params[k]); });
    sendRedirect(res, url.toString(), params.error ? 'Access was not granted.' : 'Signed in. Returning to the app.');
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

function issueAuthCode(q) {
    return sign('code', CODE_TTL, {
        jti: crypto.randomBytes(12).toString('base64url'), cid: q.client_id, ru: q.redirect_uri, cc: q.code_challenge
    });
}

function startGoogle(res, q) {
    if (allowedEmails().length === 0) {
        return page(res, 503, 'Sign-in is not configured',
            '<h1>Sign-in is not configured</h1><p class="err">No allowed email is set.</p>');
    }
    const verifier = crypto.randomBytes(48).toString('base64url');
    const challenge = crypto.createHash('sha256').update(verifier).digest('base64url');
    const state = sign('req', REQUEST_TTL, {
        cid: q.client_id, ru: q.redirect_uri, cc: q.code_challenge, st: q.state || '', gv: verifier
    });
    let location;
    try {
        location = google.authorizationUrl({
            redirectUri: googleRedirectUri(),
            state: state,
            codeChallenge: challenge
        });
    } catch (err) {
        return page(res, 500, 'Sign-in is not configured', '<h1>Sign-in is not configured</h1><p class="err">The Google sign-in address is invalid.</p>');
    }
    res.writeHead(302, { Location: location, 'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer' });
    res.end();
}

async function googleEmail(code, verifier) {
    const identity = await google.identityFromCode({
        code: code,
        verifier: verifier,
        redirectUri: googleRedirectUri()
    });
    return identity && identity.email;
}

async function googleCallback(res, url) {
    if (!googleConfigured()) return page(res, 404, 'Not found', '<p>Not found.</p>');
    const pending = verify(url.searchParams.get('state'), 'req');
    if (!pending || !pending.gv) {
        return page(res, 400, 'Expired', '<h1>Sign-in expired</h1><p>Start again from the app.</p>');
    }
    const q = {
        client_id: pending.cid, redirect_uri: pending.ru, code_challenge: pending.cc, state: pending.st,
        response_type: 'code', code_challenge_method: 'S256'
    };
    const resolved = resolveAuthorizeRequest(q);
    if (resolved.fatal || resolved.redirectError) {
        return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">Invalid request.</p>');
    }
    const err = url.searchParams.get('error');
    if (err) {
        return redirectWith(res, q.redirect_uri, {
            error: err === 'access_denied' ? 'access_denied' : 'server_error',
            state: q.state
        });
    }
    const code = url.searchParams.get('code');
    if (!code) return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">Google did not return a code.</p>');
    let email;
    try {
        email = await googleEmail(code, pending.gv);
    } catch (e) {
        return page(res, 502, 'Google sign-in failed', '<h1>Google sign-in failed</h1><p class="err">Try again from the app.</p>');
    }
    if (!email || allowedEmails().indexOf(email) === -1) {
        return redirectWith(res, q.redirect_uri, { error: 'access_denied', state: q.state });
    }
    redirectWith(res, q.redirect_uri, { code: issueAuthCode(q), state: q.state });
}

function authorizeGet(req, res, url) {
    const q = {};
    url.searchParams.forEach((value, name) => { if (!(name in q)) q[name] = value; });
    const resolved = resolveAuthorizeRequest(q);
    if (resolved.fatal) return page(res, 400, 'Cannot sign in', '<h1>Cannot sign in</h1><p class="err">' + esc(resolved.fatal) + '</p>');
    if (resolved.redirectError) {
        return redirectWith(res, q.redirect_uri, { error: resolved.redirectError, error_description: resolved.description, state: q.state });
    }
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
    const pending = verify(form.req, 'req');
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

function issueTokens(res, clientId) {
    json(res, 200, {
        access_token: sign('access', ACCESS_TTL, { cid: clientId, scope: SCOPE }),
        token_type: 'Bearer',
        expires_in: ACCESS_TTL,
        refresh_token: sign('refresh', REFRESH_TTL, { cid: clientId, scope: SCOPE }),
        scope: SCOPE
    });
}

function sweepUsedCodes() {
    const t = now();
    usedCodes.forEach((exp, jti) => { if (exp <= t) usedCodes.delete(jti); });
}

async function token(req, res) {
    let form;
    try { form = await readForm(req); } catch (err) {
        return oauthError(res, err.status || 400, 'invalid_request', 'Bad body.');
    }
    if (form.grant_type === 'authorization_code') {
        const grant = verify(form.code, 'code');
        if (!grant) return oauthError(res, 400, 'invalid_grant', 'Code is invalid or expired.');
        sweepUsedCodes();
        if (usedCodes.has(grant.jti)) return oauthError(res, 400, 'invalid_grant', 'Code was already used.');
        usedCodes.set(grant.jti, grant.exp);
        if (form.client_id !== grant.cid || form.redirect_uri !== grant.ru) {
            return oauthError(res, 400, 'invalid_grant', 'client_id or redirect_uri does not match the code.');
        }
        if (!pkceMatches(form.code_verifier, grant.cc)) {
            return oauthError(res, 400, 'invalid_grant', 'PKCE verification failed.');
        }
        return issueTokens(res, grant.cid);
    }
    if (form.grant_type === 'refresh_token') {
        const grant = verify(form.refresh_token, 'refresh');
        if (!grant) return oauthError(res, 400, 'invalid_grant', 'Refresh token is invalid or expired.');
        if (form.client_id && form.client_id !== grant.cid) {
            return oauthError(res, 400, 'invalid_grant', 'client_id does not match the refresh token.');
        }
        if (!verify(grant.cid, 'client')) return oauthError(res, 400, 'invalid_grant', 'Client is no longer registered.');
        return issueTokens(res, grant.cid);
    }
    return oauthError(res, 400, 'unsupported_grant_type', 'Use authorization_code or refresh_token.');
}

// Returns true when it answered the request.
async function handle(req, res) {
    if (!enabled()) return false;
    const url = new URL(req.url || '/', 'http://127.0.0.1');
    const p = url.pathname;
    const get = req.method === 'GET' || req.method === 'HEAD';
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

module.exports = { handle, enabled, verifyAccessToken, resourceMetadataUrl };
