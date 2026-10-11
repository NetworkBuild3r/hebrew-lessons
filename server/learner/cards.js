/**
 * Flashcard decks for one learner. Suspended words are in no deck.
 * The due deck uses shared/player/review.js DEFAULT_CAPS (30 due, 10 new).
 * Intervals come from fsrs.previewIntervals; this module does not store them.
 * Provenance: INIT-006/SPEC-009
 */
'use strict';

const reviewRepo = require('../db/repos/review');
const lexiconRepo = require('../db/repos/lexicon');
const HebrewReview = require('../../shared/player/review.js');
const HebrewFsrs = require('../../shared/player/fsrs.js');
const progress = require('../progress');
const model = require('./model');
const library = require('./library');

const DIRECTIONS = ['he_en', 'en_he'];
const NAMED = new Set(['due', 'weak', 'new', 'all']);
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const CONCEPT = /^(binyan|tense|root-class|cell):[a-z0-9][a-z0-9.-]{0,40}$/;
const PERSONAL_SOURCES = new Set(['self', 'ai', 'lesson']);

/**
 * @param {string} code
 * @param {number} [status]
 * @returns {{ ok: false, error: string, status: number }}
 */
function fail(code, status) {
    return { ok: false, error: code, status: status == null ? 400 : status };
}

/**
 * @param {number|Date|string|null|undefined} now
 * @returns {number}
 */
function toTime(now) {
    if (now == null) return Date.now();
    if (now instanceof Date) return now.getTime();
    if (typeof now === 'number' && Number.isFinite(now)) return now;
    if (typeof now === 'string' && now) {
        const parsed = Date.parse(now);
        if (Number.isFinite(parsed)) return parsed;
    }
    return NaN;
}

/**
 * @param {unknown} raw
 * @returns {{ ok: true, kind: string, value: string|null, name: string }|{ ok: false, error: string, status: number }}
 */
function parseDeck(raw) {
    const deck = raw == null || raw === '' ? 'due' : String(raw);
    if (deck.length > 80) return fail('invalid_deck');
    if (NAMED.has(deck)) return { ok: true, kind: deck, value: null, name: deck };
    if (deck.startsWith('lesson:')) {
        const slug = deck.slice('lesson:'.length);
        if (!SLUG.test(slug) || slug === 'template') return fail('invalid_deck');
        return { ok: true, kind: 'lesson', value: slug, name: deck };
    }
    if (deck.startsWith('concept:')) {
        const key = deck.slice('concept:'.length);
        if (!CONCEPT.test(key)) return fail('invalid_deck');
        return { ok: true, kind: 'concept', value: key, name: deck };
    }
    return fail('invalid_deck');
}

/**
 * @param {string|null|undefined} raw
 * @returns {string[]}
 */
function parseSources(raw) {
    if (raw == null || raw === '') return [];
    try {
        const parsed = JSON.parse(String(raw));
        return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch (_) {
        return [];
    }
}

/**
 * @param {object[]} rows
 * @returns {boolean}
 */
function wordIsNew(rows) {
    return rows.every((row) => row.last_at == null && row.last_result == null);
}

/**
 * @param {object} row
 * @param {number} nowMs
 * @returns {boolean}
 */
function isDue(row, nowMs) {
    const dueMs = Date.parse(String(row.due));
    return Number.isFinite(dueMs) && dueMs <= nowMs;
}

/**
 * @param {object} row
 * @returns {string}
 */
function lessonOf(row) {
    for (const source of parseSources(row.sources)) {
        if (source && !PERSONAL_SOURCES.has(source)) return source;
    }
    return '';
}

/**
 * Oldest due first, then the same new-word cap as HebrewReview.dueList.
 * A second direction of a word already chosen still fits under the due cap.
 * @param {object[]} items
 * @returns {object[]}
 */
function selectDue(items) {
    const caps = HebrewReview.DEFAULT_CAPS;
    const dueCap = caps.due;
    const newCap = caps.new;
    const pending = items.slice().sort((a, b) => {
        if (a.dueMs !== b.dueMs) return a.dueMs - b.dueMs;
        if (a.word < b.word) return -1;
        if (a.word > b.word) return 1;
        return a.direction < b.direction ? -1 : 1;
    });
    const selected = [];
    const newWords = new Set();
    while (pending.length && selected.length < dueCap) {
        const lastTwo = selected.length >= 2
            ? [selected[selected.length - 2].lesson, selected[selected.length - 1].lesson]
            : null;
        let otherLessonWaiting = false;
        if (lastTwo && lastTwo[0] && lastTwo[0] === lastTwo[1]) {
            otherLessonWaiting = pending.some((item) => item.lesson && item.lesson !== lastTwo[0]);
        }
        let pick = -1;
        for (let i = 0; i < pending.length; i += 1) {
            const candidate = pending[i];
            if (candidate.isNew && !newWords.has(candidate.word) && newWords.size >= newCap) {
                continue;
            }
            if (otherLessonWaiting && lastTwo && candidate.lesson === lastTwo[0]) continue;
            pick = i;
            break;
        }
        if (pick < 0) break;
        const chosen = pending.splice(pick, 1)[0];
        if (chosen.isNew) newWords.add(chosen.word);
        selected.push(chosen);
    }
    return selected;
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
 * @param {string} word
 * @param {string} conceptKey
 * @param {Map<string, object>} lexByKey
 * @param {Map<string, object[]>} formCache
 * @returns {boolean}
 */
function matchesConcept(word, conceptKey, lexByKey, formCache) {
    const splitAt = conceptKey.indexOf(':');
    if (splitAt < 0) return false;
    const family = conceptKey.slice(0, splitAt);
    const rest = conceptKey.slice(splitAt + 1);
    const entry = lexByKey.get(word);
    if (!entry) return false;
    const lemma = entry.kind === 'lemma'
        ? entry
        : (entry.lemma_key ? (lexByKey.get(entry.lemma_key) || lexiconRepo.find(entry.lemma_key)) : null);
    const lemmaKey = lemma ? lemma.key : null;
    const forms = lemmaKey ? formsOf(lemmaKey, formCache) : [];
    if (family === 'binyan') {
        return entry.binyan === rest
            || (lemma && lemma.binyan === rest)
            || forms.some((form) => form.binyan === rest);
    }
    if (family === 'root-class') {
        return entry.root_class === rest
            || (lemma && lemma.root_class === rest)
            || forms.some((form) => form.root_class === rest);
    }
    if (family === 'tense') {
        return entry.tense === rest || forms.some((form) => form.tense === rest);
    }
    if (family === 'cell') {
        const dot = rest.indexOf('.');
        if (dot < 0) return false;
        const tense = rest.slice(0, dot);
        const cell = rest.slice(dot + 1);
        if (entry.tense === tense && entry.cell === cell) return true;
        return forms.some((form) => form.tense === tense && form.cell === cell);
    }
    return false;
}

/**
 * @param {object} row
 * @param {{ gloss_en: string, translit: string, audio: string }} face
 * @param {number} userId
 * @param {number} nowMs
 * @returns {object}
 */
function toCard(row, face, userId, nowMs) {
    const gloss = face ? face.gloss_en : '';
    const hebrew = String(row.word);
    const prompt = row.direction === 'en_he' ? gloss : hebrew;
    const answer = row.direction === 'en_he' ? hebrew : gloss;
    const card = progress.cardForRow(userId, row, nowMs);
    return {
        word: hebrew,
        direction: row.direction,
        prompt,
        answer,
        translit: face ? face.translit : '',
        audio: face ? face.audio : '',
        intervals: HebrewFsrs.previewIntervals(card, nowMs)
    };
}

/**
 * Cards for one deck. `deck` defaults to due.
 * @param {number} userId
 * @param {unknown} [deck]
 * @param {number|Date|string} [now]
 * @returns {{ ok: true, deck: string, cards: object[] }|{ ok: false, error: string, status: number }}
 */
function deck(userId, deckName, now) {
    const uid = Number(userId);
    if (!Number.isInteger(uid) || uid <= 0) return fail('bad_request');
    const parsed = parseDeck(deckName);
    if (!parsed.ok) return parsed;
    const nowMs = toTime(now);
    if (!Number.isFinite(nowMs)) return fail('bad_request');

    const rows = reviewRepo.listByUser(uid).filter((row) => Number(row.suspended) !== 1);
    const byWord = new Map();
    for (const row of rows) {
        if (!byWord.has(row.word)) byWord.set(row.word, []);
        byWord.get(row.word).push(row);
    }

    let chosen = [];
    if (parsed.kind === 'due') {
        const items = [];
        for (const group of byWord.values()) {
            const fresh = wordIsNew(group);
            for (const row of group) {
                if (!DIRECTIONS.includes(row.direction) || !isDue(row, nowMs)) continue;
                items.push({
                    row,
                    word: String(row.word),
                    direction: row.direction,
                    dueMs: Date.parse(String(row.due)),
                    lesson: lessonOf(row),
                    isNew: fresh
                });
            }
        }
        chosen = selectDue(items).map((item) => item.row);
    } else if (parsed.kind === 'weak') {
        const flags = new Map(model.wordStates(uid, nowMs).map((entry) => [entry.word, entry.weak]));
        chosen = rows.filter((row) => {
            const flag = flags.get(String(row.word));
            return flag === 'weak' || flag === 'very_weak';
        });
    } else if (parsed.kind === 'new') {
        const stages = new Map();
        for (const entry of model.wordStates(uid, nowMs)) {
            stages.set(entry.word + '\nhe_en', entry.he_en && entry.he_en.stage);
            stages.set(entry.word + '\nen_he', entry.en_he && entry.en_he.stage);
        }
        chosen = rows.filter((row) => stages.get(String(row.word) + '\n' + row.direction) === 'new');
    } else if (parsed.kind === 'lesson') {
        chosen = rows.filter((row) => parseSources(row.sources).includes(parsed.value));
    } else if (parsed.kind === 'concept') {
        const words = [...byWord.keys()];
        const lexByKey = new Map(lexiconRepo.listByKeys(words).map((row) => [row.key, row]));
        const formCache = new Map();
        const matching = new Set(words.filter((word) => {
            return matchesConcept(word, parsed.value, lexByKey, formCache);
        }));
        chosen = rows.filter((row) => matching.has(String(row.word)));
    } else {
        chosen = rows.filter((row) => DIRECTIONS.includes(row.direction));
    }

    if (parsed.kind !== 'due') {
        chosen = chosen.slice().sort((a, b) => {
            if (a.word < b.word) return -1;
            if (a.word > b.word) return 1;
            return a.direction < b.direction ? -1 : 1;
        });
    }

    const faces = library.facesFor(uid, [...new Set(chosen.map((row) => String(row.word)))]);
    const cards = chosen.map((row) => toCard(row, faces.get(String(row.word)), uid, nowMs));
    return { ok: true, deck: parsed.name, cards };
}

module.exports = {
    deck
};
