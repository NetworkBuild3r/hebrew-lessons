/**
 * Tiny method + path router mounted at /api/.
 * Provenance: INIT-002/SPEC-008
 * Param segments: INIT-002/SPEC-009
 */
'use strict';

const sessions = require('../auth/sessions');
const cookies = require('../http/cookies');
const security = require('../http/security');

/**
 * @typedef {(req: import('http').IncomingMessage, res: import('http').ServerResponse, ctx: object) => any} Handler
 */

/**
 * Match an exact path, or a pattern with a single :param segment.
 * Exact equality wins when both could apply (caller tries exact first).
 * @param {string} pattern
 * @param {string} pathname
 * @returns {Record<string, string>|null} params object (possibly empty), or null
 */
function matchRoute(pattern, pathname) {
    if (pattern === pathname) return {};
    if (!pattern.includes(':')) return null;

    const patternParts = pattern.split('/');
    const pathParts = pathname.split('/');
    if (patternParts.length !== pathParts.length) return null;

    /** @type {Record<string, string>} */
    const params = {};
    for (let i = 0; i < patternParts.length; i++) {
        const pp = patternParts[i];
        const vp = pathParts[i];
        if (pp.startsWith(':') && pp.length > 1) {
            if (vp === undefined || vp === '') return null;
            let decoded;
            try {
                decoded = decodeURIComponent(vp);
            } catch (_) {
                return null;
            }
            params[pp.slice(1)] = decoded;
        } else if (pp !== vp) {
            return null;
        }
    }
    return params;
}

function createRouter() {
    /** @type {Array<{ method: string, path: string, handlers: Handler[] }>} */
    const routes = [];

    /**
     * @param {string} method
     * @param {string} path absolute API path e.g. /api/session or /api/lessons/:slug/progress
     * @param {...Handler} handlers
     */
    function add(method, path, ...handlers) {
        routes.push({
            method: String(method).toUpperCase(),
            path,
            handlers
        });
    }

    /**
     * Resolve session from cookie; 401 if missing.
     * @param {Handler} handler
     * @returns {Handler}
     */
    function requireSession(handler) {
        return async function sessionGuard(req, res, ctx) {
            const token = cookies.readSessionToken(req.headers.cookie);
            const resolved = token ? sessions.resolve(token) : null;
            if (!resolved) {
                security.sendJsonError(res, 401, 'Unauthorized', 'no_session');
                return;
            }
            ctx.user = resolved.user;
            ctx.session = resolved.session;
            ctx.token = token;
            return handler(req, res, ctx);
        };
    }

    /**
     * Require an admin role (after requireSession).
     * @param {Handler} handler
     * @returns {Handler}
     */
    function requireAdmin(handler) {
        return async function adminGuard(req, res, ctx) {
            if (!ctx.user || ctx.user.role !== 'admin') {
                security.sendJsonError(res, 403, 'Forbidden', 'forbidden');
                return;
            }
            return handler(req, res, ctx);
        };
    }

    /**
     * Dispatch an /api/ request. Caller already enforced CSRF for mutating methods.
     * @param {import('http').IncomingMessage} req
     * @param {import('http').ServerResponse} res
     * @param {string} pathname
     * @returns {Promise<boolean>} true if a route matched
     */
    async function handle(req, res, pathname) {
        const method = String(req.method || 'GET').toUpperCase();
        let matched = null;
        let params = {};

        // Prefer exact path matches so static routes stay authoritative.
        for (const route of routes) {
            if (route.method !== method) continue;
            if (route.path === pathname) {
                matched = route;
                params = {};
                break;
            }
        }
        if (!matched) {
            for (const route of routes) {
                if (route.method !== method) continue;
                if (!route.path.includes(':')) continue;
                const got = matchRoute(route.path, pathname);
                if (got) {
                    matched = route;
                    params = got;
                    break;
                }
            }
        }

        if (!matched) {
            security.sendJsonError(res, 404, 'Not found', 'not_found');
            return true;
        }
        const ctx = { pathname, params };
        for (const handler of matched.handlers) {
            // eslint-disable-next-line no-await-in-loop
            await handler(req, res, ctx);
            if (res.writableEnded || res.headersSent) return true;
        }
        return true;
    }

    return {
        add,
        requireSession,
        requireAdmin,
        handle,
        routes,
        matchRoute
    };
}

module.exports = { createRouter, matchRoute };
