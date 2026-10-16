/**
 * Idempotent lexicon seed from the published lessons' vocab index.
 * One lemma or phrase per key. Phrase when the key contains a space.
 * Gloss from meaning, transliteration from sound. source='seed', verified=0.
 * Does not guess part of speech, root, or binyan. Never overwrites a row
 * whose source is not seed.
 * Provenance: INIT-006/SPEC-005
 */
'use strict';

const { buildVocabIndex, exactKey } = require('../vocab-index');
const { validateEntries, stripPoints } = require('./validate');
const { applySeed } = require('./service');

const GLOSS_MAX = 120;
const TRANSLIT_MAX = 80;
const MARKUP = /[<>]/;
const CONTROL = /[\u0000-\u001F\u007F]/;

/**
 * @param {unknown} value
 * @param {number} max
 * @returns {{ text: string|null }|{ reject: true }}
 */
function cleanField(value, max) {
    if (value == null) return { text: null };
    const text = String(value).normalize('NFC').trim();
    if (!text) return { text: null };
    if (MARKUP.test(text) || CONTROL.test(text)) return { reject: true };
    const clipped = [...text].slice(0, max).join('');
    if (!clipped || MARKUP.test(clipped) || CONTROL.test(clipped)) return { reject: true };
    return { text: clipped };
}

/**
 * @param {object[]} vocabEntries
 * @returns {{ entries: object[], skipped: number }}
 */
function entriesFromVocab(vocabEntries) {
    const entries = [];
    let skipped = 0;
    (vocabEntries || []).forEach((item) => {
        const rawKey = item && (item.key != null ? item.key : item.word);
        const key = exactKey(rawKey);
        if (!key) {
            skipped += 1;
            return;
        }
        const gloss = cleanField(item.meaning, GLOSS_MAX);
        const translit = cleanField(item.sound, TRANSLIT_MAX);
        if (gloss.reject || translit.reject) {
            skipped += 1;
            return;
        }
        if (stripPoints(key) === key && !/[\u05D0-\u05EA]/.test(key)) {
            skipped += 1;
            return;
        }
        const kind = key.includes(' ') ? 'phrase' : 'lemma';
        const checked = validateEntries([{
            key,
            kind,
            gloss_en: gloss.text,
            translit: translit.text
        }], { tableLemmas: null });
        if (!checked.ok || !checked.entries || !checked.entries.length) {
            skipped += 1;
            return;
        }
        entries.push(checked.entries[0]);
    });
    return { entries, skipped };
}

/**
 * Seed from buildVocabIndex(). Safe to call on every startup.
 * @param {{ lessonsDir?: string }} [opts]
 * @returns {{ inserted: number, updated: number, kept: number, skipped: number, vocab: number, warnings: number }}
 */
function run(opts) {
    const built = buildVocabIndex(opts || {});
    const prepared = entriesFromVocab(built.entries);
    const applied = applySeed(prepared.entries);
    return {
        inserted: applied.inserted,
        updated: applied.updated,
        kept: applied.kept,
        skipped: prepared.skipped,
        vocab: built.entries.length,
        warnings: built.warnings.length
    };
}

module.exports = {
    run,
    entriesFromVocab
};
