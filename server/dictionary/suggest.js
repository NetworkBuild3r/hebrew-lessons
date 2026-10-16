/**
 * Word suggestions: Wiktionary glosses, with lexicon rows first so a stored
 * verb is still offered beside the dictionary.
 */
'use strict';

const db = require('../db');
const dictionary = require('../db/repos/dictionary');
const { exactKey } = require('../vocab-index');
const { foldEnglish, foldHebrew } = require('./parse-dsl');
const { shortGloss } = require('./lookup');

const DEFAULT_LIMIT = 8;
const MAX_LIMIT = 20;
const QUERY_MAX = 80;

/**
 * @param {string} needle
 * @param {string} text
 * @returns {number}
 */
function rank(needle, text) {
    if (!text) return 2;
    if (text === needle) return 0;
    if (text.startsWith(needle)) return 1;
    return 2;
}

/**
 * 0 exact, 1 a short or hyphenated extension, -1 ignore.
 * Mid-word hits such as grandmother for mother, or a city name that
 * merely contains אבא, stay out of the list.
 * @param {string} needle
 * @param {string} enFold
 * @param {string} heFold
 * @param {boolean} hebrewQuery
 * @returns {number}
 */
function glossClass(needle, enFold, heFold, hebrewQuery) {
    const fold = hebrewQuery ? heFold : enFold;
    if (fold === needle) return 0;
    if (hebrewQuery) return heFold.startsWith(needle) ? 1 : -1;
    if (enFold.startsWith(needle + ' ') || enFold.startsWith(needle + '-')) return 1;
    if (enFold.startsWith(needle) && enFold.length - needle.length <= 3) return 1;
    return -1;
}

/**
 * @param {unknown} q
 * @param {unknown} limitRaw
 * @returns {{ ok: true, entries: object[] }|{ ok: false, error: string }}
 */
function suggest(q, limitRaw) {
    if (q != null && typeof q !== 'string') return { ok: false, error: 'invalid_filter' };
    const term = exactKey(q || '');
    const clipped = [...term].slice(0, QUERY_MAX).join('');
    if (!clipped) return { ok: true, entries: [] };
    let limit = DEFAULT_LIMIT;
    if (limitRaw != null && limitRaw !== '') {
        const parsed = Number(limitRaw);
        if (!Number.isInteger(parsed) || parsed < 1) return { ok: false, error: 'invalid_filter' };
        limit = Math.min(parsed, MAX_LIMIT);
    }
    const hebrew = /[\u05D0-\u05EA]/.test(clipped);
    const needle = hebrew ? foldHebrew(clipped) : foldEnglish(clipped);
    if (!needle) return { ok: true, entries: [] };

    const lexicon = db.all(
        'SELECT key, gloss_en, kind, pos, lemma_key FROM lexicon_entries'
    ).filter((row) => {
        const keyFold = foldHebrew(row.key || '');
        const glossFold = foldEnglish(row.gloss_en || '');
        return keyFold.includes(needle) || glossFold.includes(needle);
    }).sort((a, b) => {
        const ar = Math.min(rank(needle, foldHebrew(a.key || '')), rank(needle, foldEnglish(a.gloss_en || '')));
        const br = Math.min(rank(needle, foldHebrew(b.key || '')), rank(needle, foldEnglish(b.gloss_en || '')));
        return ar - br;
    }).slice(0, limit);

    const glosses = dictionary.searchGlosses(needle, limit * 8).filter((row) => {
        return glossClass(needle, foldEnglish(row.en), foldHebrew(row.he), hebrew) >= 0;
    });
    const book = dictionary.searchEntries(needle, hebrew, limit);
    const entries = [];
    const seen = new Set();
    lexicon.forEach((row) => {
        const gloss = typeof row.gloss_en === 'string' ? row.gloss_en : '';
        const id = row.key + '\n' + foldEnglish(gloss);
        if (seen.has(id)) return;
        seen.add(id);
        entries.push({
            key: row.key,
            gloss_en: gloss,
            kind: row.kind,
            pos: row.pos,
            lemma_key: row.lemma_key,
            source: 'lexicon'
        });
    });
    book.forEach((row) => {
        if (entries.length >= limit) return;
        const gloss = shortGloss(row.en);
        const id = row.he + '\n' + foldEnglish(gloss);
        if (!gloss || seen.has(id)) return;
        seen.add(id);
        entries.push({
            key: row.he,
            gloss_en: gloss,
            kind: null,
            pos: row.pos || null,
            lemma_key: null,
            source: 'dictionary'
        });
    });
    glosses.forEach((row) => {
        if (entries.length >= limit) return;
        const id = row.he + '\n' + foldEnglish(row.en);
        if (seen.has(id)) return;
        seen.add(id);
        entries.push({
            key: row.he,
            gloss_en: row.en,
            kind: null,
            pos: null,
            lemma_key: null,
            source: 'wiktionary'
        });
    });
    return { ok: true, entries: entries };
}

module.exports = { suggest };
