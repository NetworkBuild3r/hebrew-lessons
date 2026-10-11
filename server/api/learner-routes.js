/**
 * Session routes for the learner's own profile, coverage, and daily mix.
 * The signed-in user id is the only learner these handlers pass. Request
 * fields are not forwarded as `now` or as mix options.
 * Provenance: INIT-006/SPEC-010
 */
'use strict';

const body = require('../http/body');
const security = require('../http/security');
const profile = require('../learner/profile');
const coverage = require('../learner/coverage');
const mix = require('../learner/mix');

/** A coverage body is one text of at most 4000 characters, plus JSON. */
const COVERAGE_BODY_LIMIT = 8192;

/**
 * @param {import('http').ServerResponse} res
 * @param {number} status
 * @param {string} code
 */
function sendError(res, status, code) {
    security.sendJson(res, status, { error: code, code });
}

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
 * @param {string} code
 * @returns {number}
 */
function statusFor(code) {
    if (code === 'not_found') return 404;
    if (code === 'rate_limited') return 429;
    if (code === 'planning_off' || code === 'forbidden') return 403;
    return 400;
}

/**
 * @param {import('http').ServerResponse} res
 * @param {object} result
 */
function sendService(res, result) {
    if (result && typeof result.error === 'string') {
        sendError(res, statusFor(result.error), result.error);
        return;
    }
    security.sendJson(res, 200, result);
}

/**
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    router.add('GET', '/api/me/profile', router.requireSession(async (req, res, ctx) => {
        sendService(res, profile.buildProfile(ctx.user.id, { audience: 'self' }));
    }));

    router.add('POST', '/api/me/coverage', router.requireSession(async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req, { limit: COVERAGE_BODY_LIMIT });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            sendError(res, 400, 'bad_request');
            return;
        }
        if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
            sendError(res, 400, 'bad_request');
            return;
        }
        if (Object.keys(payload).some((key) => key !== 'text')) {
            sendError(res, 400, 'bad_request');
            return;
        }
        sendService(res, coverage.checkCoverage(ctx.user.id, payload.text, { audience: 'self' }));
    }));

    router.add('GET', '/api/me/mix', router.requireSession(async (req, res, ctx) => {
        security.sendJson(res, 200, mix.compose(ctx.user.id, Date.now()));
    }));
}

module.exports = {
    register,
    COVERAGE_BODY_LIMIT
};
