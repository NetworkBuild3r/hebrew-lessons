/**
 * Learner progress / review / metrics routes (session user only).
 * Provenance: INIT-002/SPEC-009
 */
'use strict';

const body = require('../http/body');
const security = require('../http/security');
const progress = require('../progress');
const { canSeeLesson } = require('../visibility');

const MAX_SIGNALS = 200;

/**
 * Keep words that have at least one source lesson the user can see.
 * Hidden-only words stay in the DB and reappear if shared again.
 * @param {object} user
 * @param {{ v: number, words: object, migratedLessons: object }} state
 * @returns {{ v: number, words: object, migratedLessons: object }}
 */
function filterReviewState(user, state) {
    const words = Object.create(null);
    const src = (state && state.words) || {};
    for (const key of Object.keys(src)) {
        const entry = src[key];
        const sources = entry && Array.isArray(entry.sources) ? entry.sources : [];
        const visible = sources.some((slug) => canSeeLesson(user, String(slug)));
        if (!visible) continue;
        words[key] = entry;
    }
    return {
        v: state && state.v != null ? state.v : 1,
        words,
        migratedLessons: (state && state.migratedLessons) || {}
    };
}

/**
 * @param {unknown} err
 * @param {import('http').ServerResponse} res
 * @returns {boolean} true if handled as BodyError
 */
function sendBodyError(err, res) {
    if (err && err.name === 'BodyError') {
        security.sendJsonError(res, err.status, err.message, err.code);
        return true;
    }
    return false;
}

/**
 * Register learner progress routes on a router from createRouter().
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    router.add('GET', '/api/review/state', router.requireSession(async (req, res, ctx) => {
        const raw = progress.reviewState(ctx.user.id);
        security.sendJson(res, 200, filterReviewState(ctx.user, raw));
    }));

    router.add('POST', '/api/review/signals', router.requireSession(async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req);
        } catch (err) {
            if (sendBodyError(err, res)) return;
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }

        const signals = payload && payload.signals;
        if (!Array.isArray(signals)) {
            security.sendJsonError(res, 400, 'signals array required', 'bad_request');
            return;
        }
        if (signals.length > MAX_SIGNALS) {
            security.sendJsonError(res, 413, 'Too many signals', 'signals_too_many');
            return;
        }

        const allowed = [];
        let rejectedVisible = 0;
        for (const signal of signals) {
            const slug = signal && signal.slug != null ? String(signal.slug) : '';
            if (!slug || !canSeeLesson(ctx.user, slug)) {
                rejectedVisible += 1;
                continue;
            }
            allowed.push(signal);
        }

        // Ignore any injected userId — session user only.
        const result = progress.applySignals(ctx.user.id, allowed);
        const state = filterReviewState(ctx.user, result.state);
        security.sendJson(res, 200, {
            applied: result.applied,
            rejected: result.rejected + rejectedVisible,
            state
        });
    }));

    router.add('GET', '/api/lessons/:slug/progress', router.requireSession(async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        if (!slug || !canSeeLesson(ctx.user, slug)) {
            security.sendJsonError(res, 404, 'Not found', 'not_found');
            return;
        }
        const row = progress.getLessonProgress(ctx.user.id, slug);
        if (!row) {
            security.sendJson(res, 200, { state: {}, rev: 0 });
            return;
        }
        security.sendJson(res, 200, { state: row.state, rev: row.rev });
    }));

    router.add('PUT', '/api/lessons/:slug/progress', router.requireSession(async (req, res, ctx) => {
        const slug = ctx.params && ctx.params.slug;
        if (!slug || !canSeeLesson(ctx.user, slug)) {
            security.sendJsonError(res, 404, 'Not found', 'not_found');
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

        const state = payload && payload.state;
        const rev = payload && payload.rev;
        // Ignore injected userId — session user only.
        const result = progress.putLessonProgress(ctx.user.id, slug, state, rev);
        if (result.ok) {
            security.sendJson(res, 200, { state: result.state, rev: result.rev });
            return;
        }
        if (result.conflict) {
            security.sendJson(res, 409, {
                state: result.state == null ? {} : result.state,
                rev: result.rev
            });
            return;
        }
        security.sendJsonError(res, 400, result.error || 'Invalid state', 'bad_request');
    }));

    router.add('GET', '/api/me/metrics', router.requireSession(async (req, res, ctx) => {
        security.sendJson(res, 200, progress.metrics(ctx.user.id));
    }));
}

module.exports = {
    register,
    filterReviewState,
    MAX_SIGNALS
};
