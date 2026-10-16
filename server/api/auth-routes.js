/**
 * Sign-in / sign-out / me / password routes.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const accounts = require('../accounts');
const sessions = require('../auth/sessions');
const google = require('../auth/google');
const googleLogin = require('../auth/google-login');
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
function redirect(res, location, setCookie) {
    security.applySecurityHeaders(res, { noStore: true });
    const headers = {
        Location: location,
        'Content-Type': 'text/plain; charset=utf-8',
        'Cache-Control': 'no-store',
        'Referrer-Policy': 'no-referrer',
        'Content-Length': 0
    };
    if (setCookie) headers['Set-Cookie'] = setCookie;
    res.writeHead(302, headers);
    res.end();
}

function loginError(code) {
    return '/login/?error=' + encodeURIComponent(code);
}

function register(router) {
    router.add('GET', '/api/auth/google', async (req, res) => {
        security.sendJson(res, 200, { enabled: googleLogin.enabled() });
    });

    router.add('GET', '/api/auth/google/start', async (req, res) => {
        if (!googleLogin.enabled()) {
            redirect(res, loginError('google_off'));
            return;
        }
        let next = '/';
        try {
            const url = new URL(req.url || '/', 'http://127.0.0.1');
            next = security.safeNextPath(url.searchParams.get('next'));
        } catch (err) {
            next = '/';
        }
        let location;
        try {
            location = googleLogin.startUrl(next);
        } catch (err) {
            redirect(res, loginError('google_off'));
            return;
        }
        redirect(res, location);
    });

    router.add('GET', '/api/auth/google/callback', async (req, res) => {
        if (!googleLogin.enabled()) {
            redirect(res, loginError('google_off'));
            return;
        }
        const url = new URL(req.url || '/', 'http://127.0.0.1');
        const pending = googleLogin.readState(url.searchParams.get('state'));
        if (!pending) {
            redirect(res, loginError('google_expired'));
            return;
        }
        const next = security.safeNextPath(pending.n);
        if (url.searchParams.get('error')) {
            redirect(res, loginError('google_denied'));
            return;
        }
        const code = url.searchParams.get('code');
        if (!code) {
            redirect(res, loginError('google_failed'));
            return;
        }
        let identity;
        try {
            identity = await google.identityFromCode({
                code: code,
                verifier: pending.v,
                redirectUri: googleLogin.redirectUri()
            });
        } catch (err) {
            redirect(res, loginError('google_failed'));
            return;
        }
        if (!identity) {
            redirect(res, loginError('google_unverified'));
            return;
        }
        let result;
        try {
            result = accounts.signInWithGoogle(identity, { ip: clientIp(req) });
        } catch (err) {
            const codeName = err && err.code === 'throttled'
                ? 'google_throttled'
                : err && err.code === 'disabled'
                    ? 'google_disabled'
                    : 'google_unlinked';
            redirect(res, loginError(codeName));
            return;
        }
        redirect(res, next, cookies.serializeSession(result.token));
    });

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
