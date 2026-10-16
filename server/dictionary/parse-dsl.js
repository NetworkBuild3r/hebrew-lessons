/**
 * Pull Hebrew headwords and English glosses out of a Wiktionary DSL article.
 * The file is the bidirectional English–Hebrew dictionary from
 * open-dsl-dict/wiktionary-dict (CC BY-SA 3.0 / GFDL).
 * A gloss is the English headword, not the long sense note.
 */
'use strict';

const { exactKey } = require('../vocab-index');

const HEBREW = /[\u05D0-\u05EA]/;
const POINTS = /[\u0591-\u05C7]/g;
const TAG = /\[[^\]\n]*\]/g;
const HE_MAX = 40;
const EN_MAX = 60;

/**
 * @param {string} text
 * @returns {string}
 */
function foldHebrew(text) {
    return exactKey(text).replace(POINTS, '');
}

/**
 * @param {string} text
 * @returns {string}
 */
function foldEnglish(text) {
    return exactKey(text).toLowerCase();
}

/**
 * @param {string} text
 * @returns {number}
 */
function codePoints(text) {
    return [...text].length;
}

/**
 * Hebrew phrases inside one [m1]…[/m] block.
 * @param {string} body
 * @returns {string[]}
 */
function hebrewPhrases(body) {
        const plain = String(body)
            .replace(/\[c gray\][\s\S]*?\[\/c\]/g, ' ')
            .replace(TAG, ' ')
            .replace(/<[^>\n]*>/g, ' ')
            .replace(/\/[^/\n]+\//g, ' ');
    const out = [];
    plain.split(/[,;]/).forEach((piece) => {
        const he = exactKey(piece).replace(/^head=/, '');
        if (!HEBREW.test(he)) return;
        if (codePoints(he) < 1 || codePoints(he) > HE_MAX) return;
        if (/[<>]/.test(he)) return;
        out.push(he);
    });
    return out;
}

/**
 * @param {string} text UTF-16 text of a DSL dictionary, BOM optional.
 * @returns {{ he: string, en: string }[]}
 */
function parseDsl(text) {
    const lines = String(text || '').replace(/^\uFEFF/, '').split(/\r?\n/);
    const rows = [];
    const seen = new Set();
    let i = 0;
    while (i < lines.length) {
        const line = lines[i];
        if (!line.startsWith('\t') || line.indexOf('[b]') === -1) {
            i += 1;
            continue;
        }
        let block = line;
        i += 1;
        while (i < lines.length && lines[i].startsWith('\t')) {
            block += '\n' + lines[i];
            i += 1;
        }
        const enMatch = block.match(/\[c darkblue\]([^[]+)\[\/c\]/);
        const en = enMatch ? exactKey(enMatch[1]) : '';
        if (!en || HEBREW.test(en) || codePoints(en) > EN_MAX || /[<>]/.test(en)) continue;
        const senses = block.match(/\[m\d+\][\s\S]*?\[\/m\]/g) || [];
        senses.forEach((sense) => {
            hebrewPhrases(sense).forEach((he) => {
                const id = he + '\n' + foldEnglish(en);
                if (seen.has(id)) return;
                seen.add(id);
                rows.push({ he: he, en: en });
            });
        });
    }
    return rows;
}

module.exports = {
    foldEnglish,
    foldHebrew,
    parseDsl
};
