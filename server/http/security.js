/**
 * CSRF Origin/Referer checks, security headers, safe next paths.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const SECURITY_HEADERS = {
    'X-Content-Type-Options': 'nosniff',
    'Referrer-Policy': 'same-origin',
    'X-Frame-Options': 'DENY'
};

/**
 * Apply baseline security headers to a response.
 * @param {import('http').ServerResponse} res
 * @param {{ noStore?: boolean }} [opts]
 */
function applySecurityHeaders(res, opts) {
    for (const [key, value] of Object.entries(SECURITY_HEADERS)) {
        if (!res.getHeader(key)) res.setHeader(key, value);
    }
    if (opts && opts.noStore) {
        res.setHeader('Cache-Control', 'no-store');
    }
}

/**
 * Content-Type is application/json, optionally with a charset parameter.
 * @param {string|undefined|null} header
 * @returns {boolean}
 */
function isJsonContentType(header) {
    if (typeof header !== 'string' || !header) return false;
    const media = header.split(';')[0].trim().toLowerCase();
    return media === 'application/json';
}

/**
 * Expected origin string for this request (no trailing slash).
 * @param {import('http').IncomingMessage} req
 * @returns {string}
 */
function expectedOrigin(req) {
    if (process.env.PUBLIC_ORIGIN) {
        return String(process.env.PUBLIC_ORIGIN).replace(/\/$/, '');
    }
    const host = req.headers.host;
    if (!host) return '';
    const proto = req.headers['x-forwarded-proto']
        || (cookieLooksSecure() ? 'https' : 'http');
    return proto + '://' + host;
}

function cookieLooksSecure() {
    return process.env.COOKIE_SECURE === '1';
}

/**
 * Extract origin from Origin header or Referer URL.
 * @param {import('http').IncomingMessage} req
 * @returns {string|null}
 */
function requestOrigin(req) {
    const origin = req.headers.origin;
    if (typeof origin === 'string' && origin && origin !== 'null') {
        return origin.replace(/\/$/, '');
    }
    const referer = req.headers.referer || req.headers.referrer;
    if (typeof referer === 'string' && referer) {
        try {
            const url = new URL(referer);
            return url.origin;
        } catch (_) {
            return null;
        }
    }
    return null;
}

/**
 * Origin/Referer must match PUBLIC_ORIGIN or the request Host.
 * Missing or foreign → false.
 * @param {import('http').IncomingMessage} req
 * @returns {boolean}
 */
function originAllowed(req) {
    const got = requestOrigin(req);
    if (!got) return false;
    const want = expectedOrigin(req);
    if (!want) return false;
    return got === want;
}

/**
 * Same-site absolute path for post-sign-in redirects.
 * Hostile values fall back to `/`.
 * @param {unknown} value
 * @returns {string}
 */
function safeNextPath(value) {
    if (typeof value !== 'string' || !value) return '/';
    const next = value.trim();
    if (!next) return '/';
    // Must be a same-site absolute path: single leading /, no scheme, no //, no \
    if (!next.startsWith('/')) return '/';
    if (next.startsWith('//')) return '/';
    if (next.includes('\\')) return '/';
    if (next.includes('\0')) return '/';
    // Reject scheme-like prefixes that somehow survived (defense in depth)
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(next)) return '/';
    return next;
}

/**
 * Write a JSON API error (no stacks, hashes, or tokens).
 * @param {import('http').ServerResponse} res
 * @param {number} status
 * @param {string} message
 * @param {string} [code]
 */
function sendJsonError(res, status, message, code) {
    applySecurityHeaders(res, { noStore: true });
    const body = { error: String(message || 'Error') };
    if (code) body.code = code;
    const payload = JSON.stringify(body);
    res.writeHead(status, {
        'Content-Type': 'application/json; charset=utf-8',
        'Cache-Control': 'no-store',
        'Content-Length': Buffer.byteLength(payload)
    });
    res.end(payload);
}

/**
 * Write a JSON API success body.
 * @param {import('http').ServerResponse} res
 * @param {number} status
 * @param {object} data
 * @param {{ setCookie?: string }} [extra]
 */
function sendJson(res, status, data, extra) {
    applySecurityHeaders(res, { noStore: true });
    const payload = JSON.stringify(data);
    const headers = {
        'Content-Type': 'application/json; charset=utf-8',
        'Cache-Control': 'no-store',
        'Content-Length': Buffer.byteLength(payload)
    };
    if (extra && extra.setCookie) {
        headers['Set-Cookie'] = extra.setCookie;
    }
    res.writeHead(status, headers);
    res.end(payload);
}

module.exports = {
    SECURITY_HEADERS,
    applySecurityHeaders,
    isJsonContentType,
    expectedOrigin,
    requestOrigin,
    originAllowed,
    safeNextPath,
    sendJsonError,
    sendJson
};
