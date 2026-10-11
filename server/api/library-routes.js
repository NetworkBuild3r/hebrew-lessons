/**
 * Learner library, flashcard, lexicon-read, and grammar-concept routes.
 * Every route uses the session user. There is no userId parameter.
 * CSRF for writes is enforced in server/web.js before dispatch.
 * Provenance: INIT-006/SPEC-009
 */
'use strict';

const body = require('../http/body');
const security = require('../http/security');
const library = require('../learner/library');
const cards = require('../learner/cards');
const lexicon = require('../lexicon/service');
const conceptsRepo = require('../db/repos/concepts');

/** Fifty short notes fit; a larger body is refused before it is parsed. */
const BODY_LIMIT = 16 * 1024;

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
 * @param {import('http').IncomingMessage} req
 * @returns {URLSearchParams}
 */
function queryOf(req) {
    const raw = String(req.url || '');
    const mark = raw.indexOf('?');
    if (mark < 0) return new URLSearchParams();
    return new URLSearchParams(raw.slice(mark + 1));
}

/**
 * @param {URLSearchParams} params
 * @param {string} name
 * @returns {string|undefined}
 */
function param(params, name) {
    if (!params.has(name)) return undefined;
    return params.get(name);
}

/**
 * Learners see a boolean, including unverified forms (ADR-0009 aud-1).
 * @param {object|null|undefined} entry
 * @returns {object|null}
 */
function publicEntry(entry) {
    if (!entry || typeof entry !== 'object') return entry == null ? null : entry;
    const copy = Object.assign({}, entry);
    if (Object.prototype.hasOwnProperty.call(copy, 'verified')) {
        copy.verified = copy.verified === true || copy.verified === 1;
    }
    // Operator identity stays off the learner read.
    delete copy.verified_by;
    return copy;
}

/**
 * @param {object} paradigm
 * @returns {object}
 */
function publicParadigm(paradigm) {
    const out = {};
    const source = paradigm || {};
    for (const tense of Object.keys(source)) {
        out[tense] = {};
        const cells = source[tense] || {};
        for (const cell of Object.keys(cells)) {
            out[tense][cell] = publicEntry(cells[cell]);
        }
    }
    return out;
}

/**
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    router.add('GET', '/api/me/library', router.requireSession(async (req, res, ctx) => {
        const params = queryOf(req);
        const result = library.list(ctx.user.id, {
            q: param(params, 'q'),
            status: param(params, 'status'),
            weak: param(params, 'weak'),
            pos: param(params, 'pos'),
            binyan: param(params, 'binyan'),
            tense: param(params, 'tense'),
            source: param(params, 'source'),
            suspended: param(params, 'suspended'),
            limit: param(params, 'limit'),
            cursor: param(params, 'cursor')
        }, Date.now());
        if (!result.ok) {
            sendError(res, result.status, result.error);
            return;
        }
        security.sendJson(res, 200, { words: result.words, nextCursor: result.nextCursor });
    }));

    router.add('POST', '/api/me/library', router.requireSession(async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req, { limit: BODY_LIMIT });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            sendError(res, 400, 'bad_request');
            return;
        }
        if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
            sendError(res, 400, 'bad_request');
            return;
        }
        if (Object.keys(payload).some((key) => key !== 'words')) {
            sendError(res, 400, 'bad_request');
            return;
        }
        const result = library.add(ctx.user.id, payload.words, { source: 'self' });
        if (!result.ok) {
            sendError(res, result.status, result.error);
            return;
        }
        security.sendJson(res, 200, { words: result.words, created: result.created });
    }));

    router.add('PATCH', '/api/me/library/:key', router.requireSession(async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req, { limit: BODY_LIMIT });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            sendError(res, 400, 'bad_request');
            return;
        }
        if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
            sendError(res, 400, 'bad_request');
            return;
        }
        if (Object.keys(payload).some((key) => key !== 'suspended')) {
            sendError(res, 400, 'bad_request');
            return;
        }
        const result = library.setSuspended(ctx.user.id, ctx.params && ctx.params.key, payload.suspended);
        if (!result.ok) {
            sendError(res, result.status, result.error);
            return;
        }
        security.sendJson(res, 200, { word: result.word, suspended: result.suspended });
    }));

    router.add('GET', '/api/me/cards', router.requireSession(async (req, res, ctx) => {
        const params = queryOf(req);
        const result = cards.deck(ctx.user.id, param(params, 'deck'), Date.now());
        if (!result.ok) {
            sendError(res, result.status, result.error);
            return;
        }
        security.sendJson(res, 200, { deck: result.deck, cards: result.cards });
    }));

    router.add('GET', '/api/lexicon', router.requireSession(async (req, res) => {
        const params = queryOf(req);
        const result = lexicon.list({
            q: param(params, 'q'),
            pos: param(params, 'pos'),
            binyan: param(params, 'binyan'),
            tense: param(params, 'tense'),
            rootClass: param(params, 'root_class'),
            verified: param(params, 'verified'),
            limit: param(params, 'limit'),
            cursor: param(params, 'cursor')
        });
        if (!result.ok) {
            sendError(res, 400, result.error || 'invalid_filter');
            return;
        }
        security.sendJson(res, 200, {
            entries: result.entries.map(publicEntry),
            nextCursor: result.nextCursor
        });
    }));

    router.add('GET', '/api/lexicon/:key', router.requireSession(async (req, res, ctx) => {
        const found = lexicon.get(ctx.params && ctx.params.key);
        if (!found) {
            sendError(res, 404, 'not_found');
            return;
        }
        security.sendJson(res, 200, {
            entry: publicEntry(found.entry),
            lemma: publicEntry(found.lemma),
            paradigm: publicParadigm(found.paradigm)
        });
    }));

    router.add('GET', '/api/grammar/concepts', router.requireSession(async (req, res) => {
        const concepts = conceptsRepo.list().map((row) => ({
            key: row.key,
            family: row.family,
            label_en: row.label_en,
            label_he: row.label_he,
            colour: row.colour,
            sort: row.sort
        }));
        security.sendJson(res, 200, { concepts });
    }));
}

module.exports = {
    register,
    BODY_LIMIT
};
