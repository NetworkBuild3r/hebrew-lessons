/**
 * Session cookie parse / serialize — ADR-0002 §4.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const BASE_NAME = 'hebrew_session';
const HOST_NAME = '__Host-hebrew_session';
const MAX_AGE_SEC = 30 * 24 * 60 * 60; // 30 days

function cookieSecure() {
    return process.env.COOKIE_SECURE === '1';
}

/** Cookie name for the current COOKIE_SECURE setting. */
function cookieName() {
    return cookieSecure() ? HOST_NAME : BASE_NAME;
}

/**
 * Parse a Cookie header into a name→value map (first wins).
 * @param {string|undefined|null} header
 * @returns {Record<string, string>}
 */
function parse(header) {
    const out = Object.create(null);
    if (typeof header !== 'string' || !header) return out;
    const parts = header.split(';');
    for (const part of parts) {
        const eq = part.indexOf('=');
        if (eq < 0) continue;
        const name = part.slice(0, eq).trim();
        if (!name || Object.prototype.hasOwnProperty.call(out, name)) continue;
        let value = part.slice(eq + 1).trim();
        if (value.startsWith('"') && value.endsWith('"') && value.length >= 2) {
            value = value.slice(1, -1);
        }
        try {
            out[name] = decodeURIComponent(value);
        } catch (_) {
            out[name] = value;
        }
    }
    return out;
}

/**
 * Read the raw session token from a Cookie header (either name).
 * @param {string|undefined|null} header
 * @returns {string|null}
 */
function readSessionToken(header) {
    const cookies = parse(header);
    if (cookies[HOST_NAME]) return cookies[HOST_NAME];
    if (cookies[BASE_NAME]) return cookies[BASE_NAME];
    return null;
}

/**
 * Serialize a Set-Cookie value for a session token.
 * @param {string} token
 * @param {{ maxAge?: number, clear?: boolean }} [opts]
 * @returns {string}
 */
function serializeSession(token, opts) {
    const options = opts || {};
    const name = cookieName();
    const clear = Boolean(options.clear);
    const maxAge = clear ? 0 : (options.maxAge != null ? options.maxAge : MAX_AGE_SEC);
    const value = clear ? '' : String(token);
    let line = name + '=' + encodeURIComponent(value)
        + '; Path=/'
        + '; HttpOnly'
        + '; SameSite=Lax'
        + '; Max-Age=' + maxAge;
    if (cookieSecure()) {
        line += '; Secure';
    }
    return line;
}

/**
 * Set-Cookie that clears the session cookie.
 * @returns {string}
 */
function clearSession() {
    return serializeSession('', { clear: true });
}

module.exports = {
    BASE_NAME,
    HOST_NAME,
    MAX_AGE_SEC,
    cookieSecure,
    cookieName,
    parse,
    readSessionToken,
    serializeSession,
    clearSession
};
