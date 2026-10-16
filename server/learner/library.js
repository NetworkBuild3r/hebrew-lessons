/**
 * Personal library: the learner's review words union their user_words rows.
 * A learner never writes a lexicon row. An unknown word keeps its meaning
 * on that learner's user_words.gloss_en only.
 * Provenance: INIT-006/SPEC-009
 */
'use strict';

const db = require('../db');
const reviewRepo = require('../db/repos/review');
const userWords = require('../db/repos/user-words');
const lexiconRepo = require('../db/repos/lexicon');
const { exactKey, buildVocabIndex } = require('../vocab-index');
const { POS, BINYAN, TENSE } = require('../lexicon/validate');
const model = require('./model');

const DIRECTIONS = ['he_en', 'en_he'];
const BATCH_MAX = 50;
const LIST_DEFAULT = 50;
const LIST_MAX = 200;
const KEY_MAX = 60;
const GLOSS_MAX = 100;
const QUERY_MAX = 80;
const RAW_MAX = 500;
const SOURCES = new Set(['self', 'ai']);
const PERSONAL_SOURCES = new Set(['self', 'ai', 'lesson']);
const STATUSES = new Set(['new', 'learning', 'longTerm']);
const HEBREW_LETTER = /[\u05D0-\u05EA]/;
const MARKUP = /[<>]/;
const CONTROL = /[\u0000-\u001F\u007F]/;
const BUCKET_RANK = { fragile: 0, shaky: 1, steady: 2, solid: 3 };

/**
 * @param {unknown} userId
 * @returns {number|null}
 */
function requireUser(userId) {
    const n = Number(userId);
    if (!Number.isInteger(n) || n <= 0) return null;
    return n;
}

/**
 * @param {number|Date|string|null|undefined} now
 * @returns {string}
 */
function toIso(now) {
    if (now instanceof Date) return now.toISOString();
    if (typeof now === 'number' && Number.isFinite(now)) return new Date(now).toISOString();
    if (typeof now === 'string' && now && Number.isFinite(Date.parse(now))) {
        return new Date(now).toISOString();
    }
    return new Date().toISOString();
}

/**
 * @param {string} code
 * @param {number} [status]
 * @returns {{ ok: false, error: string, status: number }}
 */
function fail(code, status) {
    return { ok: false, error: code, status: status == null ? 400 : status };
}

/**
 * @param {unknown} value
 * @returns {boolean}
 */
function oversized(value) {
    return typeof value === 'string' && value.length > RAW_MAX;
}

/**
 * @param {string} text
 * @returns {number}
 */
function codePoints(text) {
    return [...text].length;
}

/**
 * Lexicon-key rules: exactKey, 1 to 60 characters, one Hebrew letter, no markup.
 * @param {unknown} raw
 * @returns {{ ok: true, key: string }|{ ok: false, error: string, status: number }}
 */
function checkKey(raw) {
    if (typeof raw !== 'string' || oversized(raw)) return fail('invalid_key');
    const key = exactKey(raw);
    if (!key || codePoints(key) > KEY_MAX) return fail('invalid_key');
    if (MARKUP.test(key) || CONTROL.test(key)) return fail('invalid_key');
    if (!HEBREW_LETTER.test(key)) return fail('invalid_key');
    if (key !== exactKey(key)) return fail('invalid_key');
    return { ok: true, key };
}

/**
 * Learner note: 1 to 100 characters, no markup or controls. Not a lexicon write.
 * @param {unknown} raw
 * @returns {{ ok: true, gloss: string }|{ ok: false, error: string, status: number }}
 */
function checkGloss(raw) {
    if (typeof raw !== 'string' || oversized(raw)) return fail('invalid_gloss');
    const gloss = exactKey(raw);
    if (!gloss || codePoints(gloss) > GLOSS_MAX) return fail('invalid_gloss');
    if (MARKUP.test(gloss) || CONTROL.test(gloss)) return fail('invalid_gloss');
    return { ok: true, gloss };
}

/**
 * @returns {Map<string, { meaning: string, sound: string, audio: string }>}
 */
function vocabByKey() {
    const built = buildVocabIndex();
    const map = new Map();
    for (const entry of built.entries) {
        map.set(entry.key, {
            meaning: entry.meaning || '',
            sound: entry.sound || '',
            audio: entry.audio || ''
        });
    }
    return map;
}

/**
 * @param {string} word
 * @returns {string}
 */
function encodeCursor(word) {
    return Buffer.from(word, 'utf8').toString('base64url');
}

/**
 * @param {unknown} cursor
 * @returns {{ ok: true, word: string|null }|{ ok: false, error: string, status: number }}
 */
function decodeCursor(cursor) {
    if (cursor == null || cursor === '') return { ok: true, word: null };
    if (typeof cursor !== 'string' || !/^[A-Za-z0-9_-]{1,120}$/.test(cursor)) {
        return fail('invalid_cursor');
    }
    const word = Buffer.from(cursor, 'base64url').toString('utf8');
    if (!word || codePoints(word) > KEY_MAX) return fail('invalid_cursor');
    if (MARKUP.test(word) || CONTROL.test(word)) return fail('invalid_cursor');
    if (encodeCursor(word) !== cursor) return fail('invalid_cursor');
    return { ok: true, word };
}

/**
 * @param {string|null|undefined} a
 * @param {string|null|undefined} b
 * @returns {string|null}
 */
function worseBucket(a, b) {
    if (a == null) return b == null ? null : b;
    if (b == null) return a;
    return BUCKET_RANK[a] <= BUCKET_RANK[b] ? a : b;
}

/**
 * @param {object|undefined} dir
 * @returns {object|null}
 */
function directionView(dir) {
    if (!dir) return null;
    return {
        stage: dir.stage,
        strength_bucket: dir.strength_bucket,
        retrievability: dir.retrievability,
        lapses: dir.lapses,
        due_in_days: dir.due_in_days
    };
}

/**
 * @param {object|undefined} entry
 * @param {Map<string, object>} lexByKey
 * @returns {object|null}
 */
function lemmaOf(entry, lexByKey) {
    if (!entry) return null;
    if (entry.kind === 'lemma') return entry;
    if (entry.kind === 'form' && entry.lemma_key) {
        return lexByKey.get(entry.lemma_key) || lexiconRepo.find(entry.lemma_key) || null;
    }
    return null;
}

/**
 * @param {object|null} entry
 * @param {object|null} lemma
 * @returns {{ pos: string|null, root: string|null, binyan: string|null, root_class: string|null, tense: string|null }}
 */
function tagsOf(entry, lemma) {
    return {
        pos: (entry && entry.pos) || (lemma && lemma.pos) || null,
        root: (entry && entry.root) || (lemma && lemma.root) || null,
        binyan: (entry && entry.binyan) || (lemma && lemma.binyan) || null,
        root_class: (entry && entry.root_class) || (lemma && lemma.root_class) || null,
        tense: (entry && entry.tense) || null
    };
}

/**
 * @param {string} lemmaKey
 * @param {Map<string, object[]>} cache
 * @returns {object[]}
 */
function formsOf(lemmaKey, cache) {
    if (cache.has(lemmaKey)) return cache.get(lemmaKey);
    const forms = lexiconRepo.listByLemma(lemmaKey).filter((row) => row.kind === 'form');
    cache.set(lemmaKey, forms);
    return forms;
}

/**
 * @param {object} query
 * @returns {{ ok: true, filters: object }|{ ok: false, error: string, status: number }}
 */
function parseListQuery(query) {
    const q = query || {};
    const filters = {
        q: null,
        status: null,
        weak: null,
        pos: null,
        binyan: null,
        tense: null,
        source: null,
        suspended: null,
        limit: LIST_DEFAULT,
        cursor: null
    };

    if (q.q != null && q.q !== '') {
        if (typeof q.q !== 'string' || oversized(q.q)) return fail('invalid_filter');
        const term = exactKey(q.q).toLowerCase();
        if (codePoints(term) > QUERY_MAX) return fail('invalid_filter');
        if (MARKUP.test(term) || CONTROL.test(term)) return fail('invalid_filter');
        filters.q = term || null;
    }

    if (q.status != null && q.status !== '') {
        if (typeof q.status !== 'string' || !STATUSES.has(q.status)) return fail('invalid_filter');
        filters.status = q.status;
    }

    if (q.weak != null && q.weak !== '') {
        const weak = String(q.weak);
        if (weak === '1' || weak === 'true') filters.weak = 'any';
        else if (weak === '0' || weak === 'false') filters.weak = 'none';
        else if (weak === 'weak' || weak === 'very_weak') filters.weak = weak;
        else return fail('invalid_filter');
    }

    if (q.pos != null && q.pos !== '') {
        if (typeof q.pos !== 'string' || !POS.has(q.pos)) return fail('invalid_filter');
        filters.pos = q.pos;
    }
    if (q.binyan != null && q.binyan !== '') {
        if (typeof q.binyan !== 'string' || !BINYAN.has(q.binyan)) return fail('invalid_filter');
        filters.binyan = q.binyan;
    }
    if (q.tense != null && q.tense !== '') {
        if (typeof q.tense !== 'string' || !TENSE.has(q.tense)) return fail('invalid_filter');
        filters.tense = q.tense;
    }

    if (q.source != null && q.source !== '') {
        if (typeof q.source !== 'string' || !PERSONAL_SOURCES.has(q.source)) return fail('invalid_filter');
        filters.source = q.source;
    }

    if (q.suspended != null && q.suspended !== '') {
        const flag = String(q.suspended);
        if (flag === '1' || flag === 'true') filters.suspended = true;
        else if (flag === '0' || flag === 'false') filters.suspended = false;
        else return fail('invalid_filter');
    }

    if (q.limit != null && q.limit !== '') {
        const parsed = Number(q.limit);
        if (!Number.isInteger(parsed) || parsed < 1) return fail('invalid_filter');
        filters.limit = Math.min(parsed, LIST_MAX);
    }

    const cursor = decodeCursor(q.cursor);
    if (!cursor.ok) return cursor;
    filters.cursor = cursor.word;
    return { ok: true, filters };
}

/**
 * @param {object} row
 * @param {object} filters
 * @param {Map<string, object[]>} formCache
 * @param {Map<string, object>} lexByKey
 * @returns {boolean}
 */
function matches(row, filters, formCache, lexByKey) {
    if (filters.cursor && row.word <= filters.cursor) return false;
    if (filters.q) {
        const hay = [row.word, row.gloss_en, row.note || '', row.translit]
            .join('\n')
            .toLowerCase();
        if (!hay.includes(filters.q)) return false;
    }
    if (filters.status) {
        const stages = [row.he_en, row.en_he].filter(Boolean).map((dir) => dir.stage);
        if (!stages.includes(filters.status)) return false;
    }
    if (filters.weak === 'any' && row.weak !== 'weak' && row.weak !== 'very_weak') return false;
    if (filters.weak === 'none' && row.weak != null) return false;
    if (filters.weak === 'weak' && row.weak !== 'weak') return false;
    if (filters.weak === 'very_weak' && row.weak !== 'very_weak') return false;
    if (filters.pos && row.pos !== filters.pos) return false;
    if (filters.binyan && row.binyan !== filters.binyan) return false;
    if (filters.tense && !rowHasTense(row, filters.tense, formCache, lexByKey)) return false;
    if (filters.source === 'lesson') {
        const lesson = row.sources.some((source) => !PERSONAL_SOURCES.has(source));
        if (!lesson) return false;
    } else if (filters.source && !row.sources.includes(filters.source)) {
        return false;
    }
    if (filters.suspended === true && row.suspended !== true) return false;
    if (filters.suspended === false && row.suspended === true) return false;
    return true;
}

/**
 * @param {object} row
 * @param {string} tense
 * @param {Map<string, object[]>} formCache
 * @param {Map<string, object>} lexByKey
 * @returns {boolean}
 */
function rowHasTense(row, tense, formCache, lexByKey) {
    const entry = lexByKey.get(row.word);
    if (entry && entry.tense === tense) return true;
    const lemma = lemmaOf(entry, lexByKey);
    const lemmaKey = lemma ? lemma.key : (entry && entry.kind === 'lemma' ? entry.key : null);
    if (!lemmaKey) return false;
    return formsOf(lemmaKey, formCache).some((form) => form.tense === tense);
}

/**
 * @param {string} word
 * @param {object|undefined} state
 * @param {object|undefined} personal
 * @param {object|undefined} entry
 * @param {Map<string, object>} lexByKey
 * @param {Map<string, { meaning: string, sound: string, audio: string }>} vocab
 * @returns {object}
 */
function buildRow(word, state, personal, entry, lexByKey, vocab) {
    const vocabEntry = vocab.get(word);
    const lemma = lemmaOf(entry, lexByKey);
    const tags = tagsOf(entry || null, lemma);
    const note = personal && personal.gloss_en ? String(personal.gloss_en) : '';
    let gloss = '';
    let shownNote = null;
    if (entry) {
        gloss = entry.gloss_en == null ? '' : String(entry.gloss_en);
        shownNote = note || null;
    } else if (note) {
        gloss = note;
    } else if (vocabEntry && vocabEntry.meaning) {
        gloss = vocabEntry.meaning;
    }
    const translit = (entry && entry.translit)
        || (lemma && lemma.translit)
        || (vocabEntry && vocabEntry.sound)
        || '';
    const sources = [];
    const seen = new Set();
    const fromReview = state && Array.isArray(state.sources) ? state.sources : [];
    for (const source of fromReview) {
        if (!source || seen.has(source)) continue;
        seen.add(source);
        sources.push(String(source));
    }
    if (personal && personal.source && !seen.has(personal.source)) {
        sources.push(String(personal.source));
    }
    const he = directionView(state && state.he_en);
    const en = directionView(state && state.en_he);
    return {
        word,
        gloss_en: gloss,
        note: shownNote,
        translit,
        pos: tags.pos,
        root: tags.root,
        binyan: tags.binyan,
        root_class: tags.root_class,
        he_en: he,
        en_he: en,
        strength_bucket: worseBucket(
            he && he.strength_bucket,
            en && en.strength_bucket
        ),
        weak: state ? state.weak : null,
        suspended: Boolean(state && state.suspended),
        sources,
        audio: vocabEntry && vocabEntry.audio ? vocabEntry.audio : ''
    };
}

/**
 * Union of review words and user_words for this learner, with lexicon tags.
 * @param {number} userId
 * @param {object} [query]
 * @param {number|Date|string} [now]
 * @returns {{ ok: true, words: object[], nextCursor: string|null }|{ ok: false, error: string, status: number }}
 */
function list(userId, query, now) {
    const uid = requireUser(userId);
    if (uid == null) return fail('bad_request');
    const parsed = parseListQuery(query);
    if (!parsed.ok) return parsed;
    const filters = parsed.filters;
    const states = model.wordStates(uid, now);
    const byWord = new Map(states.map((entry) => [entry.word, entry]));
    const personalRows = userWords.listByUser(uid);
    const personalByWord = new Map(personalRows.map((row) => [row.word, row]));
    const words = [...new Set([...byWord.keys(), ...personalByWord.keys()])];
    words.sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));

    const lexRows = lexiconRepo.listByKeys(words);
    const lexByKey = new Map(lexRows.map((row) => [row.key, row]));
    const lemmaKeys = [];
    for (const row of lexRows) {
        if (row.lemma_key && !lexByKey.has(row.lemma_key)) lemmaKeys.push(row.lemma_key);
    }
    for (const row of lexiconRepo.listByKeys(lemmaKeys)) lexByKey.set(row.key, row);

    const vocab = vocabByKey();
    const formCache = new Map();
    const page = [];
    let overflow = false;
    for (const word of words) {
        const row = buildRow(
            word,
            byWord.get(word),
            personalByWord.get(word),
            lexByKey.get(word),
            lexByKey,
            vocab
        );
        if (!matches(row, filters, formCache, lexByKey)) continue;
        if (page.length === filters.limit) {
            overflow = true;
            break;
        }
        page.push(row);
    }
    const nextCursor = overflow ? encodeCursor(page[page.length - 1].word) : null;
    return { ok: true, words: page, nextCursor };
}

/**
 * Gloss shown on a card: lexicon gloss, else the learner's note, else vocab.
 * @param {number} userId
 * @param {string[]} words
 * @returns {Map<string, { gloss_en: string, translit: string, audio: string }>}
 */
function facesFor(userId, words) {
    const uid = requireUser(userId);
    const listWords = Array.isArray(words) ? words : [];
    const personalByWord = new Map();
    if (uid != null) {
        for (const row of userWords.listByUser(uid)) personalByWord.set(row.word, row);
    }
    const lexByKey = new Map(lexiconRepo.listByKeys(listWords).map((row) => [row.key, row]));
    const vocab = vocabByKey();
    const faces = new Map();
    for (const word of listWords) {
        const row = buildRow(
            word,
            undefined,
            personalByWord.get(word),
            lexByKey.get(word),
            lexByKey,
            vocab
        );
        faces.set(word, {
            gloss_en: row.gloss_en,
            translit: row.translit,
            audio: row.audio
        });
    }
    return faces;
}

/**
 * Add words to this learner's library. Idempotent. Does not write lexicon rows.
 * `source` is `self` from the learner route and `ai` from the MCP adapter.
 * @param {number} userId
 * @param {Array<{ key?: unknown, gloss_en?: unknown }>} words
 * @param {{ source?: string, now?: number|Date|string }} [options]
 * @returns {{ ok: true, words: string[], created: number }|{ ok: false, error: string, status: number }}
 */
function add(userId, words, options) {
    const uid = requireUser(userId);
    if (uid == null) return fail('bad_request');
    if (!Array.isArray(words)) return fail('bad_request');
    if (words.length === 0) return fail('bad_request');
    if (words.length > BATCH_MAX) return fail('words_too_many', 413);

    const opts = options || {};
    const source = opts.source == null ? 'self' : opts.source;
    if (!SOURCES.has(source)) return fail('bad_request');

    /** @type {Map<string, { key: string, gloss: string|null, known: boolean }>} */
    const accepted = new Map();
    const vocab = vocabByKey();
    for (const item of words) {
        if (!item || typeof item !== 'object' || Array.isArray(item)) return fail('bad_request');
        const checked = checkKey(item.key);
        if (!checked.ok) return checked;
        const lexRow = lexiconRepo.find(checked.key);
        const known = Boolean(lexRow) || vocab.has(checked.key);
        let gloss = null;
        if (!known) {
            if (item.gloss_en == null || item.gloss_en === '') return fail('gloss_required');
            const glossCheck = checkGloss(item.gloss_en);
            if (!glossCheck.ok) return glossCheck;
            gloss = glossCheck.gloss;
        }
        accepted.set(checked.key, { key: checked.key, gloss, known });
    }

    const nowIso = toIso(opts.now);
    const created = db.transaction(() => {
        let inserted = 0;
        for (const item of accepted.values()) {
            if (!userWords.find(uid, item.key)) {
                userWords.add({
                    user_id: uid,
                    word: item.key,
                    source,
                    added_at: nowIso,
                    gloss_en: item.known ? null : item.gloss
                });
                inserted += 1;
            }
            for (const direction of DIRECTIONS) {
                if (reviewRepo.find(uid, item.key, direction)) continue;
                reviewRepo.upsert({
                    user_id: uid,
                    word: item.key,
                    direction,
                    due: nowIso,
                    step: 0,
                    streak: 0,
                    lapses: 0,
                    last_result: null,
                    last_at: null,
                    sources: '[]',
                    fsrs_state: 'new'
                });
            }
        }
        return inserted;
    });

    return { ok: true, words: [...accepted.keys()], created };
}

/**
 * Suspend or resume every direction of one library word.
 * @param {number} userId
 * @param {unknown} word
 * @param {boolean} suspended
 * @returns {{ ok: true, word: string, suspended: boolean }|{ ok: false, error: string, status: number }}
 */
function setSuspended(userId, word, suspended) {
    const uid = requireUser(userId);
    if (uid == null) return fail('bad_request');
    if (typeof suspended !== 'boolean') return fail('bad_request');
    const checked = checkKey(word);
    if (!checked.ok) return checked;
    const inReview = DIRECTIONS.some((direction) => reviewRepo.find(uid, checked.key, direction));
    const inPersonal = Boolean(userWords.find(uid, checked.key));
    if (!inReview && !inPersonal) return fail('not_found', 404);
    if (!inReview) return fail('not_found', 404);
    reviewRepo.setSuspended(uid, checked.key, suspended);
    return { ok: true, word: checked.key, suspended };
}

module.exports = {
    list,
    add,
    setSuspended,
    facesFor,
    BATCH_MAX,
    LIST_MAX
};
