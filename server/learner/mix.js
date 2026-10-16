/**
 * Daily mix. Computed on request and never stored (ADR-0009 §8).
 *
 * Sections, in order: due review, weak words, one concept drill, one
 * reading. A personal queue may insert due reviews and skip known words.
 * It does not pull new cards from a lesson the learner has not opened,
 * and it does not reorder unseen lessons.
 *
 * Review caps come from shared/player/review.js DEFAULT_CAPS.
 * Shuffle uses a seed of the user id and the local date.
 *
 * Reading coverage calls INIT-006/SPEC-007 checkCoverage with audience 'self' when
 * server/learner/coverage.js is present. Until that module exists, the
 * same ADR-0009 §1 rule is applied here so the section can still be built.
 *
 * Every query is bound to the requesting learner.
 * Provenance: INIT-006/SPEC-008
 */
'use strict';

const reviewRepo = require('../db/repos/review');
const userWordsRepo = require('../db/repos/user-words');
const lexiconRepo = require('../db/repos/lexicon');
const conceptsRepo = require('../db/repos/concepts');
const lessonProgressRepo = require('../db/repos/lesson-progress');
const model = require('./model');
const thresholds = require('./thresholds');
const { buildVocabIndex } = require('../vocab-index');
const Review = require('../../shared/player/review.js');

const DIRECTIONS = ['he_en', 'en_he'];

/** ADR-0009 §8. Not the review caps. */
const WEAK_LIMIT = 8;
const DRILL_LIMIT = 6;
const READING_LIMIT = 3;
const MIN_COVERAGE = 0.95;

const PAGE = 200;
const STRENGTH_RANK = { fragile: 0, shaky: 1, steady: 2, solid: 3 };

/**
 * @param {string|null|undefined} text
 * @returns {string}
 */
function exactKey(text) {
    return String(text == null ? '' : text).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/**
 * @param {number|Date|string|null|undefined} now
 * @returns {number}
 */
function toTime(now) {
    if (now instanceof Date) return now.getTime();
    if (typeof now === 'number' && Number.isFinite(now)) return now;
    if (typeof now === 'string' && now) {
        const parsed = Date.parse(now);
        if (Number.isFinite(parsed)) return parsed;
    }
    return NaN;
}

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
 * @param {number} nowMs
 * @returns {string}
 */
function localDateKey(nowMs) {
    const d = new Date(nowMs);
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return d.getFullYear() + '-' + month + '-' + day;
}

/**
 * @param {string|null|undefined} raw
 * @returns {string[]}
 */
function parseSources(raw) {
    if (raw == null || raw === '') return [];
    if (Array.isArray(raw)) return raw.map(String);
    try {
        const parsed = JSON.parse(String(raw));
        return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch (_) {
        return [];
    }
}

/**
 * @param {object} row
 * @returns {boolean}
 */
function everPassed(row) {
    if (!row) return false;
    if (row.last_result === 'pass') return true;
    if (Number(row.step) >= thresholds.SCORE.pass) return true;
    if (row.last_grade != null && row.last_grade !== ''
        && Number(row.last_grade) >= thresholds.SCORE.minPassGrade) {
        return true;
    }
    return Number(row.passed_event) === thresholds.SCORE.pass;
}

/**
 * FNV-1a, then mulberry32. The seed is a string.
 * @param {string} seed
 * @returns {() => number}
 */
function rngFor(seed) {
    let h = 2166136261;
    const text = String(seed);
    for (let i = 0; i < text.length; i += 1) {
        h ^= text.charCodeAt(i);
        h = Math.imul(h, 16777619);
    }
    let a = h >>> 0;
    return function next() {
        a |= 0;
        a = a + 0x6D2B79F5 | 0;
        let t = Math.imul(a ^ a >>> 15, 1 | a);
        t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
        return ((t ^ t >>> 14) >>> 0) / 4294967296;
    };
}

/**
 * @param {object[]} items
 * @param {() => number} rng
 * @returns {object[]}
 */
function seededShuffle(items, rng) {
    const arr = items.slice();
    for (let i = arr.length - 1; i > 0; i -= 1) {
        const j = Math.floor(rng() * (i + 1));
        const swap = arr[i];
        arr[i] = arr[j];
        arr[j] = swap;
    }
    return arr;
}

/**
 * @param {string} text
 * @returns {string}
 */
function stripPoints(text) {
    return String(text).replace(/[\u0591-\u05C7]/g, '');
}

/**
 * @param {string} token
 * @returns {boolean}
 */
function isHebrewToken(token) {
    return /[\u05D0-\u05EA]/.test(token);
}

/**
 * Whitespace and punctuation. Combining marks stay on the letter.
 * @param {string} text
 * @returns {string[]}
 */
function tokenize(text) {
    const nfc = String(text || '').normalize('NFC');
    return nfc.split(/[^\p{L}\p{M}\p{N}]+/u).filter(Boolean);
}

/** @type {Function|null|undefined} */
let loadedCoverage;

/**
 * INIT-006/SPEC-007 checkCoverage, or null when that module is not on disk yet.
 * A missing dependency inside that module is not treated as absence.
 * @returns {Function|null}
 */
function loadCheckCoverage() {
    if (loadedCoverage !== undefined) return loadedCoverage;
    try {
        const mod = require('./coverage');
        if (!mod || typeof mod.checkCoverage !== 'function') {
            throw new TypeError('server/learner/coverage.js must export checkCoverage');
        }
        loadedCoverage = mod.checkCoverage;
    } catch (err) {
        const message = String(err && err.message || '');
        const missingSelf = err
            && err.code === 'MODULE_NOT_FOUND'
            && /Cannot find module ['"](?:\.\/coverage|[^'"]*[/\\]learner[/\\]coverage)['"]/.test(message);
        if (!missingSelf) throw err;
        loadedCoverage = null;
    }
    return loadedCoverage;
}

/**
 * @param {number} coverage
 * @returns {boolean}
 */
function meetsCoverage(coverage) {
    return Number.isFinite(coverage) && coverage + 1e-9 >= MIN_COVERAGE;
}

/**
 * @returns {{ id: string, items: object[] }[]}
 */
function emptySections() {
    return [
        { id: 'review', items: [] },
        { id: 'weak', items: [] }
    ];
}

/**
 * @param {number|null} verified 1, 0, or null for every row
 * @returns {object[]}
 */
function listLexicon(verified) {
    const out = [];
    let cursor = null;
    for (let page = 0; page < 50; page += 1) {
        const rows = lexiconRepo.search({
            q: null,
            pos: null,
            binyan: null,
            tense: null,
            rootClass: null,
            verified: verified == null ? null : verified,
            cursor,
            limit: PAGE
        });
        if (!rows.length) break;
        out.push(...rows);
        if (rows.length < PAGE) break;
        cursor = rows[rows.length - 1].key;
    }
    return out;
}

/**
 * Review rows plus user_words. Both are the learner's library (ADR-0009 §6).
 * @param {number} userId
 * @returns {Set<string>}
 */
function libraryKeys(userId) {
    const keys = new Set();
    for (const row of reviewRepo.listForLearner(userId)) {
        const key = exactKey(row.word);
        if (key) keys.add(key);
    }
    for (const row of userWordsRepo.listByUser(userId)) {
        const key = exactKey(row.word);
        if (key) keys.add(key);
    }
    return keys;
}

/**
 * @param {number} userId
 * @returns {Set<string>}
 */
function seenLessons(userId) {
    const slugs = new Set();
    for (const row of lessonProgressRepo.listByUser(userId)) {
        if (row.slug) slugs.add(String(row.slug));
    }
    return slugs;
}

/**
 * @param {Set<string>} library
 * @returns {{ words: Set<string>, uniqueStrips: Set<string>, forms: Set<string> }}
 */
function knownIndex(library) {
    const counts = new Map();
    for (const word of library) {
        const bare = stripPoints(word);
        if (!bare) continue;
        counts.set(bare, (counts.get(bare) || 0) + 1);
    }
    const uniqueStrips = new Set();
    for (const [bare, count] of counts) {
        if (count === 1) uniqueStrips.add(bare);
    }
    const forms = new Set();
    for (const row of listLexicon(null)) {
        if (row.kind !== 'form' || !row.lemma_key) continue;
        if (!library.has(exactKey(row.lemma_key))) continue;
        const key = exactKey(row.key);
        if (key) forms.add(key);
    }
    return { words: library, uniqueStrips, forms };
}

/**
 * ADR-0009 §1 coverage for one text. Non-Hebrew tokens are not running words.
 * @param {string} text
 * @param {{ words: Set<string>, uniqueStrips: Set<string>, forms: Set<string> }} known
 * @returns {number}
 */
function localCoverage(text, known) {
    let running = 0;
    let hits = 0;
    for (const token of tokenize(text)) {
        if (!isHebrewToken(token)) continue;
        running += 1;
        const key = exactKey(token);
        if (known.words.has(key) || known.forms.has(key)) {
            hits += 1;
            continue;
        }
        const bare = stripPoints(key);
        if (bare && known.uniqueStrips.has(bare)) hits += 1;
    }
    if (running === 0) return 0;
    return hits / running;
}

/**
 * @param {number} userId
 * @param {string} text
 * @param {Function|null} checkCoverage
 * @param {{ words: Set<string>, uniqueStrips: Set<string>, forms: Set<string> }|null} known
 * @returns {number|null}
 */
function coverageFor(userId, text, checkCoverage, known) {
    if (checkCoverage) {
        const result = checkCoverage(userId, text, { audience: 'self' });
        if (!result || result.error) return null;
        const coverage = Number(result.coverage);
        return Number.isFinite(coverage) ? coverage : null;
    }
    if (!known) return null;
    return localCoverage(text, known);
}

/**
 * @param {number} userId
 * @returns {Map<string, string>}
 */
function glossesFor(userId) {
    const gloss = new Map();
    for (const row of userWordsRepo.listByUser(userId)) {
        const key = exactKey(row.word);
        if (key && row.gloss_en) gloss.set(key, String(row.gloss_en));
    }
    for (const row of listLexicon(null)) {
        const key = exactKey(row.key);
        if (key && row.gloss_en) gloss.set(key, String(row.gloss_en));
    }
    return gloss;
}

/**
 * @param {string} lessonsDir
 * @returns {Map<string, { meaning: string, word: string }>}
 */
function vocabGloss(entries) {
    const gloss = new Map();
    for (const entry of entries) {
        const key = exactKey(entry.key || entry.word);
        if (!key || gloss.has(key)) continue;
        gloss.set(key, {
            meaning: entry.meaning != null ? String(entry.meaning) : '',
            word: entry.word != null ? String(entry.word) : key
        });
    }
    return gloss;
}

/**
 * @param {string} key
 * @param {Map<string, string>} gloss
 * @param {Map<string, { meaning: string, word: string }>} vocab
 * @returns {{ display: string, meaning: string }}
 */
function sides(key, gloss, vocab) {
    const fromVocab = vocab.get(key);
    const meaning = gloss.get(key) || (fromVocab && fromVocab.meaning) || '';
    const display = (fromVocab && fromVocab.word) || key;
    return { display, meaning };
}

/**
 * @param {object} picked
 * @param {{ display: string, meaning: string }} face
 * @returns {object}
 */
function reviewItem(picked, face) {
    const hebrewPrompt = picked.direction === 'he_en';
    return {
        key: picked.key,
        direction: picked.direction,
        prompt: hebrewPrompt ? face.display : face.meaning,
        meaning: hebrewPrompt ? face.meaning : face.display,
        lesson: picked.lesson || '',
        due: picked.due,
        isNew: picked.isNew === true
    };
}

/**
 * Due cards, not suspended. New cards (never passed) must already be in
 * the library and must not come only from a lesson with no progress row.
 * Caps and lesson interleaving are HebrewReview.dueList.
 * @param {number} userId
 * @param {number} nowMs
 * @param {Set<string>} library
 * @param {Set<string>} seen
 * @param {Map<string, string>} gloss
 * @param {Map<string, { meaning: string, word: string }>} vocab
 * @returns {object[]}
 */
function reviewItems(userId, nowMs, library, seen, gloss, vocab) {
    const words = {};
    for (const row of reviewRepo.listForLearner(userId)) {
        if (Number(row.suspended) === 1) continue;
        if (DIRECTIONS.indexOf(row.direction) === -1) continue;
        const key = exactKey(row.word);
        if (!key) continue;
        if (!words[key]) words[key] = { sources: [], passed: false };
        const entry = words[key];
        entry[row.direction] = {
            due: row.due,
            step: Number.isInteger(Number(row.step)) ? Number(row.step) : 0,
            streak: Number(row.streak) || 0,
            lapses: Number(row.lapses) || 0,
            lastResult: row.last_result == null ? null : String(row.last_result),
            lastAt: row.last_at == null ? null : String(row.last_at)
        };
        if (everPassed(row)) entry.passed = true;
        for (const source of parseSources(row.sources)) {
            if (entry.sources.indexOf(source) === -1) entry.sources.push(source);
        }
    }

    for (const key of Object.keys(words)) {
        const entry = words[key];
        if (entry.passed) continue;
        const inLibrary = library.has(key);
        const sourced = entry.sources.length > 0;
        const fromSeen = entry.sources.some((slug) => seen.has(slug));
        if (!inLibrary || (sourced && !fromSeen)) delete words[key];
    }

    const picked = Review.dueList({ words }, nowMs, Review.DEFAULT_CAPS);
    return picked.map((item) => reviewItem(item, sides(item.key, gloss, vocab)));
}

/**
 * @param {object} state
 * @returns {number}
 */
function worstStrength(state) {
    let rank = 4;
    for (const direction of DIRECTIONS) {
        const bucket = state[direction] && state[direction].strength_bucket;
        const value = Object.prototype.hasOwnProperty.call(STRENGTH_RANK, bucket)
            ? STRENGTH_RANK[bucket]
            : 4;
        if (value < rank) rank = value;
    }
    return rank;
}

/**
 * @param {object} state
 * @returns {number}
 */
function maxLapses(state) {
    let n = 0;
    for (const direction of DIRECTIONS) {
        const lapses = state[direction] ? Number(state[direction].lapses) || 0 : 0;
        if (lapses > n) n = lapses;
    }
    return n;
}

/**
 * @param {object} state
 * @param {{ display: string, meaning: string }} face
 * @returns {object[]}
 */
function bothDirections(state, face) {
    return DIRECTIONS.map((direction) => ({
        key: exactKey(state.word),
        direction,
        prompt: direction === 'he_en' ? face.display : face.meaning,
        meaning: direction === 'he_en' ? face.meaning : face.display,
        weak: state.weak
    }));
}

/**
 * Up to 8 words, very_weak then weak, not already chosen for review.
 * Each word is returned in both directions.
 * @param {object[]} states
 * @param {Set<string>} inReview
 * @param {Map<string, string>} gloss
 * @param {Map<string, { meaning: string, word: string }>} vocab
 * @returns {object[]}
 */
function weakItems(states, inReview, gloss, vocab) {
    const ranked = states.filter((state) => {
        if (state.suspended) return false;
        if (state.weak !== 'very_weak' && state.weak !== 'weak') return false;
        return !inReview.has(exactKey(state.word));
    });
    ranked.sort((a, b) => {
        if (a.weak !== b.weak) return a.weak === 'very_weak' ? -1 : 1;
        const strength = worstStrength(a) - worstStrength(b);
        if (strength !== 0) return strength;
        const lapses = maxLapses(b) - maxLapses(a);
        if (lapses !== 0) return lapses;
        return a.word < b.word ? -1 : a.word > b.word ? 1 : 0;
    });
    const items = [];
    for (const state of ranked.slice(0, WEAK_LIMIT)) {
        items.push(...bothDirections(state, sides(state.word, gloss, vocab)));
    }
    return items;
}

/**
 * Same tags as server/learner/model.js conceptKeys.
 * @param {object} form
 * @param {object|undefined} lemma
 * @returns {string[]}
 */
function formTags(form, lemma) {
    if (!form || form.kind !== 'form') return [];
    const tags = [];
    if (form.tense) tags.push('tense:' + form.tense);
    if (form.tense && form.cell) tags.push('cell:' + form.tense + '.' + form.cell);
    const binyan = form.binyan || (lemma && lemma.binyan) || '';
    const rootClass = form.root_class || (lemma && lemma.root_class) || '';
    if (binyan) tags.push('binyan:' + binyan);
    if (rootClass) tags.push('root-class:' + rootClass);
    return tags;
}

/**
 * @param {object} form
 * @returns {string[]}
 */
function acceptedSpellings(form) {
    const out = [];
    const seen = new Set();
    function add(value) {
        const text = exactKey(value);
        if (!text || seen.has(text)) return;
        seen.add(text);
        out.push(text);
    }
    add(form.key);
    let list = [];
    try {
        const parsed = JSON.parse(form.accept_json || '[]');
        if (Array.isArray(parsed)) list = parsed;
    } catch (_) {
        list = [];
    }
    for (const item of list) add(item);
    return out;
}

/**
 * Verified forms whose lemma is in the library, grouped by concept tag.
 * @param {Set<string>} library
 * @returns {Map<string, object[]>}
 */
function formsByConcept(library) {
    const forms = listLexicon(1).filter((row) => {
        return row.kind === 'form'
            && Number(row.verified) === 1
            && row.lemma_key
            && library.has(exactKey(row.lemma_key));
    });
    const lemmas = new Map();
    for (const row of lexiconRepo.listByKeys(forms.map((row) => row.lemma_key))) {
        lemmas.set(row.key, row);
    }
    const byConcept = new Map();
    for (const form of forms) {
        const lemma = lemmas.get(exactKey(form.lemma_key));
        for (const tag of formTags(form, lemma)) {
            if (!byConcept.has(tag)) byConcept.set(tag, []);
            byConcept.get(tag).push(form);
        }
    }
    return byConcept;
}

/**
 * @param {object[]} forms
 * @param {() => number} rng
 * @returns {object[]}
 */
function interleaveLemmas(forms, rng) {
    const groups = new Map();
    for (const form of forms) {
        const lemma = exactKey(form.lemma_key);
        if (!groups.has(lemma)) groups.set(lemma, []);
        groups.get(lemma).push(form);
    }
    const lemmas = seededShuffle(
        [...groups.keys()].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0)),
        rng
    );
    const queues = lemmas.map((lemma) => {
        const sorted = groups.get(lemma).slice().sort((a, b) => {
            return a.key < b.key ? -1 : a.key > b.key ? 1 : 0;
        });
        return seededShuffle(sorted, rng);
    });
    const out = [];
    let guard = 0;
    while (out.length < DRILL_LIMIT && queues.some((q) => q.length) && guard < 1000) {
        guard += 1;
        let progressed = false;
        for (const queue of queues) {
            if (!queue.length || out.length >= DRILL_LIMIT) continue;
            const lemma = exactKey(queue[0].lemma_key);
            const last = out.length ? exactKey(out[out.length - 1].lemma_key) : '';
            const another = queues.some((other) => {
                return other.length > 0 && exactKey(other[0].lemma_key) !== lemma;
            });
            if (last && last === lemma && another) continue;
            out.push(queue.shift());
            progressed = true;
        }
        if (!progressed) break;
    }
    return out;
}

/**
 * @param {object[]} states
 * @param {Map<string, object[]>} byConcept
 * @param {Map<string, object>} concepts
 * @param {Map<string, string>} gloss
 * @param {() => number} rng
 * @returns {{ id: string, concept: string, items: object[] }|null}
 */
function drillSection(states, byConcept, concepts, gloss, rng) {
    const ranked = states.filter((state) => {
        if (state.status !== 'weak' && state.status !== 'very_weak') return false;
        if (Number(state.attempts) < thresholds.MASTERY.judgedMinAttempts) return false;
        return Number.isFinite(state.ewma);
    });
    ranked.sort((a, b) => {
        if (a.ewma !== b.ewma) return a.ewma - b.ewma;
        if (a.status !== b.status) return a.status === 'very_weak' ? -1 : 1;
        return a.key < b.key ? -1 : a.key > b.key ? 1 : 0;
    });
    for (const state of ranked) {
        const forms = byConcept.get(state.key);
        if (!forms || !forms.length) continue;
        const chosen = interleaveLemmas(forms, rng);
        if (!chosen.length) continue;
        const concept = concepts.get(state.key);
        return {
            id: 'drill',
            concept: state.key,
            label: concept && (concept.label_he || concept.label_en) || state.key,
            items: chosen.map((form) => drillItem(form, state.key, concepts, gloss))
        };
    }
    return null;
}

/**
 * @param {object} form
 * @param {string} conceptKey
 * @param {Map<string, object>} concepts
 * @param {Map<string, string>} gloss
 * @returns {object}
 */
function drillItem(form, conceptKey, concepts, gloss) {
    const cellKey = form.tense && form.cell ? 'cell:' + form.tense + '.' + form.cell : '';
    const cell = cellKey ? concepts.get(cellKey) : null;
    const who = cell && (cell.label_he || cell.label_en);
    const meaning = gloss.get(exactKey(form.lemma_key)) || gloss.get(exactKey(form.key)) || '';
    const parts = [];
    if (meaning) parts.push(meaning);
    if (who) parts.push(String(who));
    return {
        key: form.key,
        lemma: exactKey(form.lemma_key),
        direction: 'en_he',
        prompt: parts.length ? parts.join(' · ') : form.key,
        accept: acceptedSpellings(form),
        concept: conceptKey
    };
}

/**
 * @param {object[]} entries
 * @returns {{ sentence: string, audio: string }[]}
 */
function sentencesFrom(entries) {
    const bySentence = new Map();
    for (const entry of entries) {
        const sentence = entry.sentence ? exactKey(entry.sentence) : '';
        if (!sentence || bySentence.has(sentence)) continue;
        bySentence.set(sentence, {
            sentence,
            audio: entry.sentenceAudio != null ? String(entry.sentenceAudio) : ''
        });
    }
    return [...bySentence.values()].sort((a, b) => {
        return a.sentence < b.sentence ? -1 : a.sentence > b.sentence ? 1 : 0;
    });
}

/**
 * @param {number} userId
 * @param {object[]} entries
 * @param {Function|null} checkCoverage
 * @param {{ words: Set<string>, uniqueStrips: Set<string>, forms: Set<string> }|null} known
 * @param {() => number} rng
 * @returns {{ id: string, items: object[] }|null}
 */
function readingSection(userId, entries, checkCoverage, known, rng) {
    const items = [];
    for (const candidate of seededShuffle(sentencesFrom(entries), rng)) {
        const coverage = coverageFor(userId, candidate.sentence, checkCoverage, known);
        if (!meetsCoverage(coverage)) continue;
        items.push({
            sentence: candidate.sentence,
            audio: candidate.audio,
            coverage
        });
        if (items.length >= READING_LIMIT) break;
    }
    if (!items.length) return null;
    return { id: 'reading', items };
}

/**
 * @param {{ lessonsDir?: string }|null|undefined} opts
 * @returns {object[]}
 */
function vocabEntries(opts) {
    const lessonsDir = opts && typeof opts.lessonsDir === 'string' ? opts.lessonsDir : '';
    const built = lessonsDir
        ? buildVocabIndex({ lessonsDir })
        : buildVocabIndex();
    return Array.isArray(built.entries) ? built.entries : [];
}

/**
 * Compose one learner's mix. Same user, same local date, and the same
 * stored rows return the same sections.
 * @param {number} userId
 * @param {number|Date|string} now
 * @param {{ lessonsDir?: string }} [opts] lessonsDir is a server option for tests.
 *   Callers must not pass request input as opts.
 * @returns {{ sections: object[] }}
 */
function compose(userId, now, opts) {
    const uid = requireUser(userId);
    const nowMs = toTime(now);
    if (uid == null || !Number.isFinite(nowMs)) {
        return { sections: emptySections() };
    }

    const library = libraryKeys(uid);
    const seen = seenLessons(uid);
    const entries = vocabEntries(opts);
    const gloss = glossesFor(uid);
    const vocab = vocabGloss(entries);
    const checkCoverage = loadCheckCoverage();
    const known = checkCoverage ? null : knownIndex(library);
    const seed = uid + ':' + localDateKey(nowMs);

    const review = reviewItems(uid, nowMs, library, seen, gloss, vocab);
    const inReview = new Set(review.map((item) => item.key));
    const weak = weakItems(model.wordStates(uid, nowMs), inReview, gloss, vocab);
    const concepts = new Map();
    for (const row of conceptsRepo.list()) concepts.set(String(row.key), row);
    const drill = drillSection(
        model.conceptStates(uid),
        formsByConcept(library),
        concepts,
        gloss,
        rngFor(seed + '\0drill')
    );
    const reading = readingSection(
        uid,
        entries,
        checkCoverage,
        known,
        rngFor(seed + '\0reading')
    );

    const sections = [
        { id: 'review', items: review },
        { id: 'weak', items: weak }
    ];
    if (drill) sections.push(drill);
    if (reading) sections.push(reading);
    return { sections };
}

module.exports = {
    compose
};
