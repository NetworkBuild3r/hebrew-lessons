/**
 * What a tapped Hebrew word means. Checked verb forms first, then the
 * dictionary, then the short glosses. A word the dictionary does not list
 * is tried again without a common prefix, suffix, or inner vav/yod.
 */
'use strict';

const db = require('../db');
const dictionary = require('../db/repos/dictionary');
const { foldHebrew } = require('./parse-dsl');

const WORD_MAX = 60;
const MEANINGS_MAX = 8;
const FETCH_MAX = 24;
const PREFIXES = ['ה', 'ו', 'ב', 'כ', 'ל', 'מ', 'ש'];
const SUFFIXES = ['ים', 'ות', 'ה', 'ת'];
const FINAL = { צ: 'ץ', כ: 'ך', מ: 'ם', נ: 'ן', פ: 'ף' };

/**
 * Other spellings to try, nearest first.
 * @param {string} fold
 * @returns {string[]}
 */
function candidates(fold) {
    const out = [];
    const seen = new Set([fold]);
    const add = (form) => {
        if ([...form].length < 2) return false;
        // A last letter takes its final shape (רצ is stored as רץ).
        const last = form[form.length - 1];
        const shaped = FINAL[last] ? form.slice(0, -1) + FINAL[last] : form;
        let fresh = false;
        [form, shaped].forEach((variant) => {
            if (seen.has(variant)) return;
            seen.add(variant);
            out.push(variant);
            fresh = true;
        });
        return fresh;
    };
    const prefixes = (form) => {
        const forms = [];
        if (PREFIXES.includes(form[0]) && [...form].length >= 3) forms.push(form.slice(1));
        if (PREFIXES.includes(form[0]) && PREFIXES.includes(form[1]) && [...form].length >= 4) forms.push(form.slice(2));
        return forms;
    };
    const suffixes = (form) => SUFFIXES
        .filter((s) => form.endsWith(s) && [...form].length - [...s].length >= 2)
        .map((s) => form.slice(0, form.length - s.length));
    const inner = (form) => {
        const forms = [];
        for (let i = 1; i < form.length - 1; i += 1) {
            if (form[i] === 'ו' || form[i] === 'י') forms.push(form.slice(0, i) + form.slice(i + 1));
        }
        return forms;
    };
    const layer = (bases, step) => {
        const next = [];
        bases.forEach((base) => step(base).forEach((form) => { if (add(form)) next.push(form); }));
        return next;
    };
    const withPrefix = layer([fold], prefixes);
    const stems = [fold].concat(withPrefix);
    const withInner = layer(stems, inner);
    const withSuffix = layer(stems.concat(withInner), suffixes);
    layer(withSuffix, inner);
    layer(withSuffix, prefixes);
    return out.slice(0, 40);
}

/**
 * The first few senses, as one short line.
 * @param {string} text
 * @returns {string}
 */
function shortGloss(text) {
    return String(text).split(/[;,]/).slice(0, 3).map((s) => s.trim()).filter(Boolean).join(', ');
}

/**
 * Everyday senses first: nouns and others before verbs (unless the word is an
 * infinitive), and passives ("be eaten") last.
 * @param {{ pos: string, en: string }[]} rows
 * @param {boolean} infinitive
 * @returns {{ pos: string, en: string }[]}
 */
function everydayFirst(rows, infinitive) {
    const score = (row) => {
        let n = 0;
        if (/^be\s/i.test(row.en)) n += 2;
        if (/\(comput|, name\b|\bname \(/i.test(row.en)) n += 1;
        if (!infinitive && /verb/.test(row.pos)) n += 1;
        return n;
    };
    return rows
        .map((row, i) => ({ row, i }))
        .sort((a, b) => score(a.row) - score(b.row) || a.i - b.i)
        .map((entry) => entry.row);
}

/**
 * @param {unknown} word
 * @returns {{ word: string, short: string, form: string|null, meanings: { pos: string, en: string, source: string }[] }}
 */
function lookup(word) {
    const raw = typeof word === 'string' ? word.normalize('NFC').replace(/\s+/g, ' ').trim() : '';
    const clipped = [...raw].slice(0, WORD_MAX).join('');
    const fold = foldHebrew(clipped);
    const result = { word: clipped, short: '', form: null, meanings: [] };
    if (!fold || !/[\u05D0-\u05EA]/.test(fold)) return result;
    const infinitive = fold[0] === 'ל';

    const seen = new Set();
    const push = (pos, en, source) => {
        const key = (pos || '') + '\n' + en.toLowerCase();
        if (!en || seen.has(key) || result.meanings.length >= MEANINGS_MAX) return;
        seen.add(key);
        result.meanings.push({ pos: pos || '', en, source });
    };

    db.all('SELECT key, gloss_en, pos FROM lexicon_entries').forEach((row) => {
        if (foldHebrew(row.key || '') === fold && row.gloss_en) push(row.pos || '', row.gloss_en, 'lexicon');
    });
    const exact = dictionary.entriesForFolds([fold], FETCH_MAX);
    everydayFirst(exact, infinitive).forEach((row) => push(row.pos, row.en, 'dictionary'));
    const glosses = dictionary.glossesForFold(fold, 6).map((row) => row.en);
    if (glosses.length) push('', shortGloss(glosses.join(', ')), 'wiktionary');

    if (!exact.length) {
        const rows = dictionary.entriesForFolds(candidates(fold), FETCH_MAX);
        if (rows.length) {
            result.form = rows[0].he;
            const same = rows.filter((row) => row.he_fold === rows[0].he_fold);
            everydayFirst(same, infinitive).forEach((row) => push(row.pos, row.en, 'dictionary'));
        }
    }
    if (result.meanings.length) result.short = shortGloss(result.meanings[0].en);
    return result;
}

module.exports = { candidates, lookup, shortGloss };
