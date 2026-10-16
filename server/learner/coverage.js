/**
 * ADR-0009 §1 known-word coverage of a Hebrew text for one learner.
 *
 * The text is NFC-normalized and split on whitespace and punctuation. A token
 * with a Hebrew letter is a running word. Any other run of letters or digits is
 * non-Hebrew and is not counted. A running word is known only when
 *   1. its exactKey, or its unique vowel-stripped form (ADR-0001 A1), names a
 *      word in the learner's library, or
 *   2. that key is a lexicon `form` whose `lemma_key` names such a word.
 * The library is every review-row word plus every user_words word (ADR-0009
 * §6); a passed word has a review row, so passed words are included. There is
 * no prefix stripping: a token with a leading ו stays unknown until a `form`
 * entry records it. A stripped form must be unique across the lexicon and the
 * learner's words together, so an unpointed token that could be a word the
 * learner has not met is unknown. Coverage under-reports rather than guesses.
 * Provenance: INIT-006/SPEC-007
 */
'use strict';

require('../../shared/player/utils.js');
const HebrewUtils = globalThis.HebrewUtils;
const coverageRepo = require('../db/repos/coverage');
const lexiconRepo = require('../db/repos/lexicon');
const contract = require('./profile-contract');
const gate = require('./gate');

/** Bidi marks and joiners are invisible and are dropped before splitting. */
const FORMAT_CHARS = /\p{Cf}/gu;

/**
 * Group 1 is a Hebrew run: letters, points and cantillation, maqaf, geresh,
 * and gershayim, the characters HebrewUtils.tokenizeHebrew keeps together,
 * except paseq (U+05C0), sof pasuq (U+05C3), and nun hafukha (U+05C6), which
 * are punctuation. Group 2 is a run of letters, marks, or digits outside the
 * Hebrew block.
 */
const TOKEN = /([\u05D0-\u05EA\u05EF-\u05F2\u0591-\u05BF\u05C1\u05C2\u05C4\u05C5\u05C7\u05F3\u05F4]+)|((?:(?![\u0590-\u05FF])[\p{L}\p{M}\p{N}])+)/gu;
const HEBREW_LETTER = /[\u05D0-\u05EA]/;
/** Marks, maqaf, geresh, or gershayim before the first letter. */
const LEADING = /^[^\u05D0-\u05EA\u05EF-\u05F2]+/;
/** Maqaf or gershayim after the last letter. A final geresh is part of the spelling. */
const TRAILING = /[\u05BE\u05F4]+$/;

/**
 * Split text into tokens. `key` is the token's exactKey. `hebrew` is false for
 * a run with no Hebrew letter.
 * @param {unknown} text
 * @returns {{ key: string, hebrew: boolean }[]}
 */
function tokenize(text) {
    const source = String(text == null ? '' : text).normalize('NFC').replace(FORMAT_CHARS, '');
    const tokens = [];
    for (const match of source.matchAll(TOKEN)) {
        if (match[2] !== undefined) {
            tokens.push({ key: match[2], hebrew: false });
            continue;
        }
        const word = match[1].replace(LEADING, '').replace(TRAILING, '');
        if (!word) continue;
        if (HEBREW_LETTER.test(word)) tokens.push({ key: lexiconRepo.exactKey(word), hebrew: true });
        else tokens.push({ key: word, hebrew: false });
    }
    return tokens;
}

/**
 * ADR-0001 A1 stripped key: exactKey with U+0591 to U+05C7 removed.
 * @param {string} text
 * @returns {string}
 */
function strippedKey(text) {
    return lexiconRepo.exactKey(HebrewUtils.stripVowels(text));
}

/**
 * The learner's library words, the lexicon lemma and form keys, and one
 * stripped-form index over both.
 * @param {number} userId
 * @returns {{ known: Set<string>, lexicon: Map<string, { kind: string, lemmaKey: string|null }>,
 *   byStripped: Map<string, Set<string>> }}
 */
function matchIndex(userId) {
    const known = new Set();
    for (const word of coverageRepo.listLibraryWords(userId)) {
        const key = lexiconRepo.exactKey(word);
        if (key) known.add(key);
    }
    const lexicon = new Map();
    for (const row of coverageRepo.listMatchKeys()) {
        lexicon.set(String(row.key), {
            kind: String(row.kind),
            lemmaKey: row.lemma_key == null ? null : String(row.lemma_key)
        });
    }
    const byStripped = new Map();
    function add(key) {
        const stripped = strippedKey(key);
        if (!stripped) return;
        if (!byStripped.has(stripped)) byStripped.set(stripped, new Set());
        byStripped.get(stripped).add(key);
    }
    for (const key of known) add(key);
    for (const key of lexicon.keys()) add(key);
    return { known, lexicon, byStripped };
}

/**
 * ADR-0001 A1: the exact key when the lexicon or the learner has it, else the
 * one key whose stripped form matches, else null.
 * @param {string} key
 * @param {ReturnType<typeof matchIndex>} index
 * @returns {string|null}
 */
function resolve(key, index) {
    if (index.known.has(key) || index.lexicon.has(key)) return key;
    const candidates = index.byStripped.get(strippedKey(key));
    if (!candidates || candidates.size !== 1) return null;
    return candidates.values().next().value;
}

/**
 * @param {string} key
 * @param {ReturnType<typeof matchIndex>} index
 * @returns {boolean}
 */
function isKnown(key, index) {
    const resolved = resolve(key, index);
    if (resolved == null) return false;
    if (index.known.has(resolved)) return true;
    const entry = index.lexicon.get(resolved);
    return Boolean(entry && entry.kind === 'form' && entry.lemmaKey != null
        && index.known.has(entry.lemmaKey));
}

/**
 * @param {number} userId
 * @param {string} text
 * @returns {{ coverage: number|null, known_count: number, running_words: number, unknown: string[] }}
 */
function measure(userId, text) {
    const words = tokenize(text).filter((token) => token.hebrew);
    if (!words.length) return { coverage: null, known_count: 0, running_words: 0, unknown: [] };
    const index = matchIndex(userId);
    const verdicts = new Map();
    const unknown = [];
    let known = 0;
    for (const { key } of words) {
        if (!verdicts.has(key)) {
            const verdict = isKnown(key, index);
            verdicts.set(key, verdict);
            if (!verdict && unknown.length < contract.LIMITS.coverageUnknown) unknown.push(key);
        }
        if (verdicts.get(key)) known += 1;
    }
    return {
        coverage: known / words.length,
        known_count: known,
        running_words: words.length,
        unknown
    };
}

/**
 * Audit detail for a returned result: field names and counts, no text and no
 * token. unknown_count is the number of tokens the result listed, which is
 * what left the server; it stops at the 100-token cap.
 * @param {{ running_words: number, unknown: string[] }} result
 * @returns {object}
 */
function describeCoverage(result) {
    return {
        fields: contract.fieldPaths(result),
        running_words: result.running_words,
        unknown_count: result.unknown.length
    };
}

/**
 * Known-word coverage of `text` for one learner.
 *
 * Returns `{ coverage, known_count, running_words, unknown }`. `coverage` is
 * known_count / running_words, or null when the text has no Hebrew word.
 * `unknown` lists distinct unknown tokens in order of first use, at most 100.
 * A text that is not a string of at most 4000 characters is `invalid_text`.
 *
 * `audience: 'mcp'` is the MCP adapter. `learner` is the username the tool was
 * given, or a user id. It is gated, counted, and audited like the profile
 * (`learner_coverage_read`), with the same refusals plus `invalid_text`.
 *
 * `audience: 'self'` is `POST /api/me/coverage` and server-side callers acting
 * for the signed-in learner. `learner` is that user's id. No switch, no rate
 * count, no audit row. Each call reads the learner's words and the lexicon keys
 * once.
 *
 * @param {number|string} learner
 * @param {unknown} text
 * @param {{ audience: 'mcp'|'self' }} opts
 * @returns {{ coverage: number|null, known_count: number, running_words: number, unknown: string[] }
 *   | { error: string }}
 */
function checkCoverage(learner, text, opts) {
    const options = opts || {};
    const audience = gate.audienceOf(options.audience);
    const action = contract.AUDIT.coverageRead;
    const valid = typeof text === 'string' && text.length <= contract.LIMITS.coverageText;
    const access = gate.admit(learner, {
        audience,
        action,
        invalid: valid ? null : 'invalid_text'
    });
    if (access.refusal) return access.refusal;
    const subject = access.learner;
    if (audience === 'self') return measure(subject.id, String(text));
    return gate.auditedRead(action, subject, () => measure(subject.id, String(text)),
        describeCoverage);
}

module.exports = {
    checkCoverage,
    tokenize
};
