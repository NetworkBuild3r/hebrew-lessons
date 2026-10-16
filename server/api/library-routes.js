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
const dictionary = require('../dictionary/suggest');
const wordLookup = require('../dictionary/lookup');
const glossDraft = require('../dictionary/suggest-glosses');
const wordAudio = require('../word-audio');
const conceptsRepo = require('../db/repos/concepts');

/** Fifty short notes fit; a larger body is refused before it is parsed. */
const BODY_LIMIT = 16 * 1024;

/** New word clips an admin may make per hour. A reused clip is free and not counted. */
const WORD_CLIP_MAX = 60;
const WORD_CLIP_WINDOW_MS = 60 * 60 * 1000;
const wordClipRuns = new Map();

/**
 * @param {string|number} adminId
 * @returns {boolean} true when this admin may make one more new clip (and counts it).
 */
function allowNewWordClip(adminId) {
    const now = Date.now();
    const runs = (wordClipRuns.get(adminId) || []).filter((t) => now - t < WORD_CLIP_WINDOW_MS);
    if (runs.length >= WORD_CLIP_MAX) {
        wordClipRuns.set(adminId, runs);
        return false;
    }
    runs.push(now);
    wordClipRuns.set(adminId, runs);
    return true;
}

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

    // Tap a Hebrew word: what it means, from checked forms and the dictionary.
    router.add('GET', '/api/dictionary/lookup', router.requireSession(async (req, res) => {
        const params = queryOf(req);
        const word = param(params, 'word');
        if (typeof word !== 'string' || !word.trim()) {
            sendError(res, 400, 'invalid_filter');
            return;
        }
        security.sendJson(res, 200, wordLookup.lookup(word));
    }));

    // Draft meanings for the words of a story. Reads the dictionary; writes nothing.
    router.add('POST', '/api/dictionary/suggest-glosses', router.requireSession(router.requireAdmin(async (req, res) => {
        let payload;
        try {
            payload = await body.readJson(req, { limit: BODY_LIMIT });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            sendError(res, 400, 'bad_request');
            return;
        }
        const valid = payload && typeof payload === 'object' && !Array.isArray(payload)
            && typeof payload.text === 'string' && payload.text.length <= 8000
            && Object.keys(payload).every((key) => key === 'text' || key === 'skip')
            && (payload.skip === undefined
                || (Array.isArray(payload.skip) && payload.skip.length <= 200
                    && payload.skip.every((word) => typeof word === 'string' && word.length <= 80)));
        if (!valid) {
            sendError(res, 400, 'bad_request');
            return;
        }
        security.sendJson(res, 200, glossDraft.suggestGlosses(payload.text, { skip: payload.skip }));
    })));

    // Words that already have sound, for the picker beside a word's audio file.
    router.add('GET', '/api/admin/word-audio', router.requireSession(router.requireAdmin(async (req, res) => {
        security.sendJson(res, 200, { clips: wordAudio.listWordClips() });
    })));

    // Sound for one word: reuse the clip if there is one, otherwise make it once.
    router.add('POST', '/api/admin/word-audio', router.requireSession(router.requireAdmin(async (req, res, ctx) => {
        let payload;
        try {
            payload = await body.readJson(req, { limit: 1024 });
        } catch (err) {
            if (sendBodyError(err, res)) return;
            sendError(res, 400, 'bad_request');
            return;
        }
        const valid = payload && typeof payload === 'object' && !Array.isArray(payload)
            && typeof payload.text === 'string' && Object.keys(payload).every((key) => key === 'text');
        if (!valid) {
            sendError(res, 400, 'bad_request');
            return;
        }
        try {
            let clip = wordAudio.findWordClip(payload.text);
            if (!clip) {
                // Only a new clip costs anything, so only a new clip counts toward the cap.
                if (!allowNewWordClip(ctx.user.id)) {
                    sendError(res, 429, 'audio_throttle');
                    return;
                }
                clip = await wordAudio.ensureWordClip(payload.text);
            }
            security.sendJson(res, 200, clip);
        } catch (err) {
            const code = err && err.code;
            if (code === 'bad_word') return sendError(res, 400, 'bad_word');
            if (code === 'no_key') return sendError(res, 503, 'audio_not_configured');
            if (code === 'tts_failed') return sendError(res, 502, 'audio_failed');
            sendError(res, 500, 'server_error');
        }
    })));

    router.add('GET', '/api/dictionary', router.requireSession(async (req, res) => {
        const params = queryOf(req);
        const result = dictionary.suggest(param(params, 'q'), param(params, 'limit'));
        if (!result.ok) {
            sendError(res, 400, result.error || 'invalid_filter');
            return;
        }
        security.sendJson(res, 200, {
            entries: result.entries,
            attribution: 'Wiktionary English–Hebrew, CC BY-SA 3.0'
        });
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
