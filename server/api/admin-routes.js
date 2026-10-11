/**
 * Admin API routes — users, progress, visibility, lessons, audio.
 * Provenance: INIT-002/SPEC-010
 */
'use strict';

const body = require('../http/body');
const security = require('../http/security');
const accounts = require('../accounts');
const progress = require('../progress');
const visibility = require('../visibility');
const visibilityRepo = require('../db/repos/visibility');
const authoring = require('../authoring');
const audit = require('../db/repos/audit');
const sessionsRepo = require('../db/repos/sessions');
const usersRepo = require('../db/repos/users');
const db = require('../db');
const lexicon = require('../lexicon/service');

/** Lexicon batches are at most 200 short entries. */
const LEXICON_BODY_LIMIT = 256 * 1024;
const SWITCH_BODY_LIMIT = 1024;

const LESSON_BODY_LIMIT = 1024 * 1024;
const AUDIO_THROTTLE_MAX = 10;
const AUDIO_THROTTLE_WINDOW_MS = 60 * 60 * 1000;

/** @type {Map<number, number[]>} adminId → generation start timestamps */
const audioRunsByAdmin = new Map();

/**
 * Route table as data so tests can iterate every admin path (ac-1).
 * @type {ReadonlyArray<{ method: string, path: string }>}
 */
const ROUTES = Object.freeze([
    { method: 'GET', path: '/api/admin/users' },
    { method: 'POST', path: '/api/admin/users' },
    { method: 'PATCH', path: '/api/admin/users/:id' },
    { method: 'POST', path: '/api/admin/users/:id/password' },
    { method: 'GET', path: '/api/admin/users/:id/progress' },
    { method: 'GET', path: '/api/admin/users/:id/events' },
    { method: 'GET', path: '/api/admin/lessons' },
    { method: 'PUT', path: '/api/admin/lessons/:slug/visibility' },
    { method: 'GET', path: '/api/admin/lessons/:slug' },
    { method: 'POST', path: '/api/admin/lessons/:slug/validate' },
    { method: 'PUT', path: '/api/admin/lessons/:slug' },
    { method: 'POST', path: '/api/admin/lessons' },
    { method: 'GET', path: '/api/admin/lessons/:slug/audio-plan' },
    { method: 'POST', path: '/api/admin/lessons/:slug/audio' },
    { method: 'PUT', path: '/api/admin/lexicon' },
    { method: 'PATCH', path: '/api/admin/lexicon/:key/verified' },
    { method: 'PATCH', path: '/api/admin/users/:id/ai-planning' }
]);

/**
 * @param {unknown} err
 * @param {import('http').ServerResponse} res
 * @returns {boolean}
 */
function sendBodyError(err, res) {
    if (err && err.name === 'BodyError') {
        security.sendJsonError(res, err.status, err.message, err.code);
        return true;
    }
    return false;
}

/**
 * Map AccountsError (and known service refusals) to HTTP status.
 * @param {unknown} err
 * @returns {{ status: number, message: string, code?: string }}
 */
function mapAccountsError(err) {
    if (err && err.name === 'AccountsError') {
        if (err.code === 'last_admin' || err.code === 'self_disable' || err.code === 'username_taken') {
            return { status: 409, message: err.message, code: err.code };
        }
        if (err.code === 'not_found') {
            return { status: 404, message: err.message, code: err.code };
        }
        if (err.code === 'password_policy') {
            return { status: 400, message: err.message, code: err.code };
        }
        return { status: 400, message: err.message, code: err.code };
    }
    return { status: 500, message: 'Internal error', code: 'internal' };
}

/**
 * @param {number} userId
 * @returns {string|null}
 */
function lastSignInFor(userId) {
    const rows = sessionsRepo.listByUser(userId);
    if (!rows || !rows.length) return null;
    return rows[0].created_at || null;
}

/**
 * Public list shape — never hashes or tokens.
 * @param {object} row accounts.publicUser-shaped
 * @returns {object}
 */
function listUserRow(row) {
    return {
        id: row.id,
        username: row.username,
        display_name: row.display_name,
        role: row.role,
        status: row.status,
        created_at: row.created_at,
        last_sign_in: lastSignInFor(row.id),
        // Admin users list only. The toggle reads this and then the PATCH response.
        // Provenance: INIT-006/SPEC-017
        ai_planning: usersRepo.getAiPlanning(row.id) === 1
    };
}

/**
 * @param {number} adminId
 * @returns {boolean} true if this run is allowed
 */
function allowAudioRun(adminId) {
    const now = Date.now();
    const cutoff = now - AUDIO_THROTTLE_WINDOW_MS;
    let runs = audioRunsByAdmin.get(adminId) || [];
    runs = runs.filter((t) => t > cutoff);
    if (runs.length >= AUDIO_THROTTLE_MAX) {
        audioRunsByAdmin.set(adminId, runs);
        return false;
    }
    runs.push(now);
    audioRunsByAdmin.set(adminId, runs);
    return true;
}

/**
 * @returns {Array<{ slug: string, mode: string, grantCount: number }>}
 */
function listLessonsWithVisibility() {
    return visibility.listRealLessonSlugs().map((slug) => {
        const row = visibilityRepo.getVisibility(slug);
        const mode = row && row.mode ? String(row.mode) : 'hidden';
        const grantCount = visibilityRepo.listGrants(slug).length;
        return { slug, mode, grantCount };
    });
}

/**
 * @param {import('http').IncomingMessage} req
 * @returns {{ before: string|null, limit: number|null }}
 */
function parseEventsQuery(req) {
    let before = null;
    let limit = null;
    try {
        const url = new URL(req.url || '/', 'http://127.0.0.1');
        if (url.searchParams.has('before')) {
            before = url.searchParams.get('before');
        }
        if (url.searchParams.has('limit')) {
            const raw = Number(url.searchParams.get('limit'));
            if (Number.isFinite(raw)) limit = raw;
        }
    } catch (_) { /* ignore */ }
    return { before, limit };
}

/**
 * Clamp limit for answerHistory (1–200, default 50 handled by progress).
 * @param {number|null} limit
 * @returns {number|undefined}
 */
function clampHistoryLimit(limit) {
    if (limit == null) return undefined;
    let n = Number(limit);
    if (!Number.isFinite(n)) return undefined;
    if (n < 1) n = 1;
    if (n > 200) n = 200;
    return n;
}

/**
 * @param {import('http').IncomingMessage} req
 * @param {import('http').ServerResponse} res
 * @param {number} limit
 * @returns {Promise<object|undefined>} undefined when a response was already sent
 */
async function readAdminJson(req, res, limit) {
    let payload;
    try {
        payload = await body.readJson(req, { limit });
    } catch (err) {
        if (sendBodyError(err, res)) return undefined;
        security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
        return undefined;
    }
    if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
        security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
        return undefined;
    }
    return payload;
}

/**
 * @param {object} payload
 * @param {string[]} keys
 * @returns {boolean}
 */
function onlyKeys(payload, keys) {
    const allowed = new Set(keys);
    return Object.keys(payload).every((key) => allowed.has(key));
}

/**
 * @param {unknown} value
 * @returns {boolean}
 */
function isFlag(value) {
    return value === true || value === false || value === 0 || value === 1;
}

/**
 * Drop caller-supplied verify and source fields. The service ignores them too.
 * @param {unknown} entries
 * @returns {unknown}
 */
function withoutVerify(entries) {
    if (!Array.isArray(entries)) return entries;
    return entries.map((entry) => {
        if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return entry;
        const copy = Object.assign({}, entry);
        delete copy.verified;
        delete copy.verified_by;
        delete copy.source;
        return copy;
    });
}

/**
 * @param {import('http').ServerResponse} res
 * @param {object} result
 */
function sendLexicon(res, result) {
    if (!result || result.ok !== true) {
        const code = result && result.error ? result.error : 'bad_request';
        let status = 400;
        if (code === 'forbidden') status = 403;
        else if (code === 'not_found') status = 404;
        else if (code === 'revision_conflict') status = 409;
        const body = { error: code, code };
        if (result && Array.isArray(result.errors)) body.errors = result.errors;
        if (result && Array.isArray(result.conflicts)) body.conflicts = result.conflicts;
        if (result && typeof result.key === 'string') body.key = result.key;
        security.sendJson(res, status, body);
        return;
    }
    security.sendJson(res, 200, { entries: result.entries });
}

/**
 * @param {ReturnType<import('./router').createRouter>} router
 * @param {string} method
 * @param {string} path
 * @param {Function} handler
 */
function addAdmin(router, method, path, handler) {
    router.add(
        method,
        path,
        router.requireSession(router.requireAdmin(handler))
    );
}

/**
 * Register admin routes on a router from createRouter().
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    addAdmin(router, 'GET', '/api/admin/users', async (req, res) => {
        const users = accounts.listUsers().map(listUserRow);
        security.sendJson(res, 200, { users });
    });

    addAdmin(router, 'POST', '/api/admin/users', async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!payload || typeof payload.username !== 'string' || typeof payload.password !== 'string') {
            security.sendJsonError(res, 400, 'username and password required', 'bad_request');
            return;
        }
        try {
            const created = accounts.createUser({
                username: payload.username,
                password: payload.password,
                role: payload.role || 'user',
                displayName: payload.displayName,
                actor: ctx.user
            });
            security.sendJson(res, 201, listUserRow(created));
        } catch (err) {
            const mapped = mapAccountsError(err);
            security.sendJsonError(res, mapped.status, mapped.message, mapped.code);
        }
    });

    addAdmin(router, 'PATCH', '/api/admin/users/:id', async (req, res, ctx) => {
        const id = Number(ctx.params && ctx.params.id);
        if (!Number.isInteger(id) || id < 1) {
            security.sendJsonError(res, 404, 'User not found', 'not_found');
            return;
        }
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!payload || typeof payload !== 'object') {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }

        const hasRole = payload.role !== undefined;
        const hasStatus = payload.status !== undefined;
        const hasDisplay = payload.displayName !== undefined;
        if (!hasRole && !hasStatus && !hasDisplay) {
            security.sendJsonError(res, 400, 'No updates provided', 'bad_request');
            return;
        }

        try {
            let updated = null;
            if (hasStatus) {
                updated = accounts.setStatus(id, payload.status, ctx.user);
            }
            if (hasRole) {
                updated = accounts.setRole(id, payload.role, ctx.user);
            }
            if (hasDisplay) {
                const target = usersRepo.findById(id);
                if (!target) {
                    throw new accounts.AccountsError('User not found', 'not_found');
                }
                const now = new Date().toISOString();
                db.run(
                    'UPDATE users SET display_name = ?, updated_at = ? WHERE id = ?',
                    payload.displayName == null ? null : String(payload.displayName),
                    now,
                    id
                );
                // Accounts has no setDisplayName; one audit row for this field only.
                if (!hasRole && !hasStatus) {
                    audit.append({
                        actor: String(ctx.user.username),
                        action: 'user.setDisplayName',
                        target: target.username,
                        detail: JSON.stringify({ displayName: payload.displayName })
                    });
                }
                updated = accounts.publicUser(usersRepo.findById(id));
            }
            security.sendJson(res, 200, listUserRow(updated));
        } catch (err) {
            const mapped = mapAccountsError(err);
            security.sendJsonError(res, mapped.status, mapped.message, mapped.code);
        }
    });

    addAdmin(router, 'POST', '/api/admin/users/:id/password', async (req, res, ctx) => {
        const id = Number(ctx.params && ctx.params.id);
        if (!Number.isInteger(id) || id < 1) {
            security.sendJsonError(res, 404, 'User not found', 'not_found');
            return;
        }
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!payload || typeof payload.password !== 'string') {
            security.sendJsonError(res, 400, 'password required', 'bad_request');
            return;
        }
        try {
            const updated = accounts.resetPassword(id, payload.password, ctx.user);
            security.sendJson(res, 200, listUserRow(updated));
        } catch (err) {
            const mapped = mapAccountsError(err);
            security.sendJsonError(res, mapped.status, mapped.message, mapped.code);
        }
    });

    addAdmin(router, 'GET', '/api/admin/users/:id/progress', async (req, res, ctx) => {
        const id = Number(ctx.params && ctx.params.id);
        if (!Number.isInteger(id) || id < 1) {
            security.sendJsonError(res, 404, 'User not found', 'not_found');
            return;
        }
        if (!usersRepo.findById(id)) {
            security.sendJsonError(res, 404, 'User not found', 'not_found');
            return;
        }
        security.sendJson(res, 200, {
            metrics: progress.metrics(id),
            troubleWords: progress.troubleWords(id)
        });
    });

    addAdmin(router, 'GET', '/api/admin/users/:id/events', async (req, res, ctx) => {
        const id = Number(ctx.params && ctx.params.id);
        if (!Number.isInteger(id) || id < 1) {
            security.sendJsonError(res, 404, 'User not found', 'not_found');
            return;
        }
        const target = usersRepo.findById(id);
        if (!target) {
            security.sendJsonError(res, 404, 'User not found', 'not_found');
            return;
        }
        const q = parseEventsQuery(req);
        const history = progress.answerHistory(id, {
            before: q.before,
            limit: clampHistoryLimit(q.limit)
        });
        audit.append({
            actor: String(ctx.user.username),
            action: 'user.readEvents',
            target: target.username,
            detail: JSON.stringify({ userId: id, limit: clampHistoryLimit(q.limit) })
        });
        security.sendJson(res, 200, history);
    });

    addAdmin(router, 'GET', '/api/admin/lessons', async (req, res) => {
        security.sendJson(res, 200, { lessons: listLessonsWithVisibility() });
    });

    addAdmin(router, 'PUT', '/api/admin/lessons/:slug/visibility', async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!payload || typeof payload.mode !== 'string') {
            security.sendJsonError(res, 400, 'mode required', 'bad_request');
            return;
        }
        try {
            visibility.setVisibility(ctx.user, slug, payload.mode);
            if (payload.userIds !== undefined || payload.mode === 'selected') {
                visibility.setGrants(
                    ctx.user,
                    slug,
                    Array.isArray(payload.userIds) ? payload.userIds : []
                );
            }
            const row = visibilityRepo.getVisibility(slug);
            const grants = visibilityRepo.listGrants(slug);
            security.sendJson(res, 200, {
                slug,
                mode: row && row.mode ? row.mode : payload.mode,
                grantCount: grants.length,
                userIds: grants.map((g) => g.user_id)
            });
        } catch (err) {
            security.sendJsonError(res, 400, err.message || 'Invalid request', 'bad_request');
        }
    });

    addAdmin(router, 'GET', '/api/admin/lessons/:slug', async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        try {
            const data = authoring.loadForEdit(slug);
            security.sendJson(res, 200, data);
        } catch (err) {
            security.sendJsonError(res, 404, err.message || 'Not found', 'not_found');
        }
    });

    addAdmin(router, 'POST', '/api/admin/lessons/:slug/validate', async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const worksheet = payload && payload.worksheet;
        const catalogEntry = payload && payload.catalogEntry;
        try {
            const result = authoring.validateDraft(slug, worksheet, catalogEntry);
            security.sendJson(res, 200, result);
        } catch (err) {
            security.sendJsonError(res, 400, err.message || 'Invalid request', 'bad_request');
        }
    });

    addAdmin(router, 'PUT', '/api/admin/lessons/:slug', async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        let payload;
        try {
            payload = await body.readJson(req, { limit: LESSON_BODY_LIMIT });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const worksheet = payload && payload.worksheet;
        const catalogEntry = payload && payload.catalogEntry;
        try {
            const saved = authoring.save(ctx.user, slug, worksheet, catalogEntry, { create: false });
            security.sendJson(res, 200, saved);
        } catch (err) {
            if (err && Array.isArray(err.errors)) {
                security.sendJson(res, 422, {
                    error: 'Validation failed',
                    code: 'validation_failed',
                    errors: err.errors,
                    warnings: err.warnings || []
                });
                return;
            }
            if (err && /does not exist/i.test(String(err.message || ''))) {
                security.sendJsonError(res, 404, err.message, 'not_found');
                return;
            }
            security.sendJsonError(res, 400, err.message || 'Invalid request', 'bad_request');
        }
    });

    addAdmin(router, 'POST', '/api/admin/lessons', async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req, { limit: LESSON_BODY_LIMIT });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const slug = payload && payload.slug;
        const worksheet = payload && payload.worksheet;
        const catalogEntry = payload && payload.catalogEntry;
        if (typeof slug !== 'string' || !slug) {
            security.sendJsonError(res, 400, 'slug required', 'bad_request');
            return;
        }
        try {
            const saved = authoring.save(ctx.user, slug, worksheet, catalogEntry, { create: true });
            security.sendJson(res, 201, saved);
        } catch (err) {
            if (err && Array.isArray(err.errors)) {
                security.sendJson(res, 422, {
                    error: 'Validation failed',
                    code: 'validation_failed',
                    errors: err.errors,
                    warnings: err.warnings || []
                });
                return;
            }
            if (err && /already exists/i.test(String(err.message || ''))) {
                security.sendJsonError(res, 409, err.message, 'slug_exists');
                return;
            }
            security.sendJsonError(res, 400, err.message || 'Invalid request', 'bad_request');
        }
    });

    addAdmin(router, 'GET', '/api/admin/lessons/:slug/audio-plan', async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        try {
            const plan = await authoring.audioPlan(slug);
            security.sendJson(res, 200, plan);
        } catch (err) {
            security.sendJsonError(res, 404, err.message || 'Not found', 'not_found');
        }
    });

    addAdmin(router, 'POST', '/api/admin/lessons/:slug/audio', async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!allowAudioRun(ctx.user.id)) {
            security.sendJsonError(res, 429, 'Audio generation rate limit exceeded', 'audio_throttle');
            return;
        }
        try {
            const result = await authoring.generateAudio(ctx.user, slug, {
                forceSentences: !!(payload && payload.forceSentences)
            });
            security.sendJson(res, 200, result);
        } catch (err) {
            const msg = String(err && err.message ? err.message : err);
            if (/already running/i.test(msg)) {
                security.sendJsonError(res, 409, msg, 'audio_in_flight');
                return;
            }
            security.sendJsonError(res, 400, msg || 'Audio generation failed', 'audio_failed');
        }
    });

    addAdmin(router, 'PUT', '/api/admin/lexicon', async (req, res, ctx) => {
        const payload = await readAdminJson(req, res, LEXICON_BODY_LIMIT);
        if (payload === undefined) return;
        if (!onlyKeys(payload, ['entries', 'expectedRev'])) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        sendLexicon(res, lexicon.upsert(withoutVerify(payload.entries), {
            actor: 'admin:' + String(ctx.user.username),
            expectedRev: payload.expectedRev
        }));
    });

    addAdmin(router, 'PATCH', '/api/admin/lexicon/:key/verified', async (req, res, ctx) => {
        const payload = await readAdminJson(req, res, SWITCH_BODY_LIMIT);
        if (payload === undefined) return;
        if (!onlyKeys(payload, ['verified']) || !isFlag(payload.verified)) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const key = ctx.params && ctx.params.key;
        sendLexicon(res, lexicon.setVerified([key], payload.verified, {
            actor: 'admin:' + String(ctx.user.username)
        }));
    });

    addAdmin(router, 'PATCH', '/api/admin/users/:id/ai-planning', async (req, res, ctx) => {
        const payload = await readAdminJson(req, res, SWITCH_BODY_LIMIT);
        if (payload === undefined) return;
        if (!onlyKeys(payload, ['enabled']) || typeof payload.enabled !== 'boolean') {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const id = Number(ctx.params && ctx.params.id);
        if (!Number.isInteger(id) || id <= 0) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        try {
            const result = accounts.setAiPlanning(id, payload.enabled, ctx.user);
            security.sendJson(res, 200, {
                ai_planning: result.next === 1,
                previous: result.previous,
                next: result.next
            });
        } catch (err) {
            const mapped = mapAccountsError(err);
            security.sendJsonError(res, mapped.status, mapped.message, mapped.code);
        }
    });
}

module.exports = {
    register,
    ROUTES,
    LESSON_BODY_LIMIT,
    AUDIO_THROTTLE_MAX
};
