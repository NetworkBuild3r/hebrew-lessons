// App sign-in through the same Google client as MCP consent.
// Redirect URI is {PUBLIC_ORIGIN}/api/auth/google/callback.
// A Google account signs in only when an admin has stored that email on a user.
'use strict';

const crypto = require('crypto');
const google = require('./google');

const STATE_TTL = 600;

function publicOrigin() {
    return String(process.env.PUBLIC_ORIGIN || '').replace(/\/+$/, '');
}

function enabled() {
    return Boolean(google.configured() && process.env.MCP_AUTH_TOKEN && publicOrigin());
}

function redirectUri() {
    return publicOrigin() + '/api/auth/google/callback';
}

function stateKey() {
    return crypto.createHmac('sha256', process.env.MCP_AUTH_TOKEN).update('hebrew-app-google-v1').digest();
}

function signState(next) {
    const payload = {
        t: 'login',
        n: next,
        exp: Math.floor(Date.now() / 1000) + STATE_TTL
    };
    const body = Buffer.from(JSON.stringify(payload)).toString('base64url');
    const sig = crypto.createHmac('sha256', stateKey()).update(body).digest('base64url');
    return body + '.' + sig;
}

function readState(token) {
    if (typeof token !== 'string') return null;
    const parts = token.split('.');
    if (parts.length !== 2) return null;
    const want = crypto.createHmac('sha256', stateKey()).update(parts[0]).digest();
    let got;
    try { got = Buffer.from(parts[1], 'base64url'); } catch (err) { return null; }
    if (got.length !== want.length || !crypto.timingSafeEqual(got, want)) return null;
    let payload;
    try { payload = JSON.parse(Buffer.from(parts[0], 'base64url').toString('utf8')); } catch (err) { return null; }
    if (!payload || payload.t !== 'login' || !(payload.exp > Math.floor(Date.now() / 1000))) return null;
    return payload;
}

/**
 * @param {string} next safe same-site path
 * @returns {string} Google authorization URL
 */
function startUrl(next) {
    const state = signState(next);
    return google.authorizationUrl({
        redirectUri: redirectUri(),
        state: state
    });
}

module.exports = {
    enabled,
    redirectUri,
    readState,
    startUrl
};
