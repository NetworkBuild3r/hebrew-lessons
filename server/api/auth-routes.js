/**
 * Sign-in / sign-out / me / password routes.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const accounts = require('../accounts');
const sessions = require('../auth/sessions');
const cookies = require('../http/cookies');
const body = require('../http/body');
const security = require('../http/security');

/**
 * Shape returned by GET /api/me and POST /api/session.
 * @param {object} user publicUser-shaped row
 * @returns {object}
 */
function mePayload(user) {
    return {
        id: user.id,
        username: user.username,
        display_name: user.display_name,
        role: user.role,
        must_change_password: Boolean(user.must_change_password)
    };
}

function clientIp(req) {
    const xf = req.headers['x-forwarded-for'];
    if (typeof xf === 'string' && xf) return xf.split(',')[0].trim();
    return (req.socket && req.socket.remoteAddress) || '';
}

/**
 * Map AccountsError to an HTTP status without weakening generic auth text.
 * @param {import('../accounts').AccountsError|Error} err
 * @returns {{ status: number, message: string, code?: string }}
 */
function mapAccountsError(err) {
    if (err && err.name === 'AccountsError') {
        if (err.code === 'throttled') {
            return { status: 429, message: err.message, code: 'throttled' };
        }
        if (err.code === 'auth_failed') {
            return { status: 401, message: err.message, code: 'auth_failed' };
        }
        if (err.code === 'password_policy') {
            return { status: 400, message: err.message, code: 'password_policy' };
        }
        return { status: 400, message: err.message, code: err.code };
    }
    return { status: 500, message: 'Internal error', code: 'internal' };
}

/**
 * Register auth routes on a router from createRouter().
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    router.add('POST', '/api/session', async (req, res) => {
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (err && err.name === 'BodyError') {
                security.sendJsonError(res, err.status, err.message, err.code);
                return;
            }
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const username = payload && payload.username;
        const password = payload && payload.password;
        if (typeof username !== 'string' || typeof password !== 'string') {
            security.sendJsonError(res, 400, 'username and password required', 'bad_request');
            return;
        }
        let result;
        try {
            result = accounts.signIn(username, password, { ip: clientIp(req) });
        } catch (err) {
            const mapped = mapAccountsError(err);
            security.sendJsonError(res, mapped.status, mapped.message, mapped.code);
            return;
        }
        // Rotate: signIn already created a new session; set the new cookie (do not reuse).
        const setCookie = cookies.serializeSession(result.token);
        const me = mePayload(result.user);
        const redirect = security.safeNextPath(payload && payload.next);
        security.sendJson(res, 200, Object.assign({}, me, { redirect }), { setCookie });
    });

    router.add('DELETE', '/api/session', async (req, res) => {
        // Body optional; allow empty. Still subject to CSRF checks upstream.
        try {
            await body.readJson(req);
        } catch (err) {
            if (err && err.name === 'BodyError' && err.code === 'body_too_large') {
                security.sendJsonError(res, err.status, err.message, err.code);
                return;
            }
            // invalid JSON on logout → still clear if we can; prefer strict:
            if (err && err.name === 'BodyError') {
                security.sendJsonError(res, err.status, err.message, err.code);
                return;
            }
        }
        const token = cookies.readSessionToken(req.headers.cookie);
        if (token) {
            sessions.revoke(token);
        }
        security.sendJson(res, 200, { ok: true }, { setCookie: cookies.clearSession() });
    });

    router.add('GET', '/api/me', router.requireSession(async (req, res, ctx) => {
        security.sendJson(res, 200, mePayload(ctx.user));
    }));

    router.add('POST', '/api/me/password', router.requireSession(async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (err && err.name === 'BodyError') {
                security.sendJsonError(res, err.status, err.message, err.code);
                return;
            }
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const currentPassword = payload
            && (payload.current_password != null ? payload.current_password : payload.currentPassword);
        const newPassword = payload
            && (payload.new_password != null ? payload.new_password : payload.newPassword);
        if (typeof currentPassword !== 'string' || typeof newPassword !== 'string') {
            security.sendJsonError(
                res,
                400,
                'current_password and new_password required',
                'bad_request'
            );
            return;
        }
        let fresh;
        try {
            fresh = accounts.changePassword(ctx.user.id, currentPassword, newPassword, {
                keepToken: ctx.token
            });
        } catch (err) {
            const mapped = mapAccountsError(err);
            security.sendJsonError(res, mapped.status, mapped.message, mapped.code);
            return;
        }
        security.sendJson(res, 200, mePayload(fresh || ctx.user));
    }));
}

module.exports = {
    register,
    mePayload,
    mapAccountsError
};
