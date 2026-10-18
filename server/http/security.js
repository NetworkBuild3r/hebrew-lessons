/**
 * CSRF Origin/Referer checks, security headers, safe next paths.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const { peerTrusted } = require('./trusted-proxies');

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
 * Scheme of the connection the process accepted. TLS on the socket is https.
 * @param {import('http').IncomingMessage|object|null|undefined} req
 * @returns {'http'|'https'}
 */
function socketScheme(req) {
    const socket = req && req.socket;
    return socket && socket.encrypted ? 'https' : 'http';
}

/**
 * Scheme written by the immediate proxy.
 *
 * Traefik's forwarder appends the scheme of the connection it accepted onto
 * X-Forwarded-Proto. It does not replace a value an earlier hop already
 * forwarded, so the leftmost field can still be client-controlled. The value
 * Traefik added is the rightmost field, or the only field when the header has
 * one. Anything other than http or https is ignored.
 * @param {import('http').IncomingMessage|object|null|undefined} req
 * @returns {'http'|'https'|''}
 */
function immediateForwardedProto(req) {
    const headers = req && req.headers;
    if (!headers) return '';
    const raw = headers['x-forwarded-proto'];
    if (typeof raw !== 'string') return '';
    const parts = raw.split(',');
    const last = parts[parts.length - 1].trim().toLowerCase();
    if (last === 'http' || last === 'https') return last;
    return '';
}

/**
 * Origin derived from the Host the browser actually used (no trailing slash).
 * X-Forwarded-Proto is read only from a TRUSTED_PROXIES peer. Otherwise the
 * scheme is the socket's TLS state.
 * @param {import('http').IncomingMessage|object|null|undefined} req
 * @returns {string}
 */
function hostOrigin(req) {
    const headers = req && req.headers;
    const host = headers && headers.host;
    if (typeof host !== 'string' || !host) return '';
    const remoteAddress = req.socket && req.socket.remoteAddress;
    let proto = socketScheme(req);
    if (peerTrusted(remoteAddress)) {
        const forwarded = immediateForwardedProto(req);
        if (forwarded) proto = forwarded;
    }
    return proto + '://' + host;
}

/**
 * Public origin from PUBLIC_ORIGIN, or the request host when that is unset.
 * @param {import('http').IncomingMessage} req
 * @returns {string}
 */
function expectedOrigin(req) {
    if (process.env.PUBLIC_ORIGIN) {
        return String(process.env.PUBLIC_ORIGIN).replace(/\/$/, '');
    }
    return hostOrigin(req);
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
    // PUBLIC_ORIGIN covers the public site name (k8s). The request host covers
    // the name the browser actually used, so localhost and 127.0.0.1 both work
    // when PUBLIC_ORIGIN names only one of them.
    const pub = process.env.PUBLIC_ORIGIN
        ? String(process.env.PUBLIC_ORIGIN).replace(/\/$/, '')
        : '';
    const host = hostOrigin(req);
    return got === pub || (!!host && got === host);
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
    hostOrigin,
    expectedOrigin,
    requestOrigin,
    originAllowed,
    safeNextPath,
    sendJsonError,
    sendJson
};
