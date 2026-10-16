/**
 * Wiktionary gloss lookup for the teacher word box.
 * Reads and replaces dictionary_glosses. Does not write lexicon_entries.
 */
'use strict';

const db = require('../../db');

/**
 * @param {string} text
 * @returns {string}
 */
function escapeLike(text) {
    return String(text).replace(/[\\%_]/g, (ch) => '\\' + ch);
}

/**
 * Replace every gloss. One transaction.
 * @param {{ he: string, en: string, heFold: string, enFold: string }[]} rows
 * @returns {number}
 */
function replaceAll(rows) {
    return db.transaction(() => {
        db.run('DELETE FROM dictionary_glosses');
        const insert = db.prepare(
            'INSERT INTO dictionary_glosses (he, en, he_fold, en_fold) VALUES (?, ?, ?, ?)'
        );
        let n = 0;
        (Array.isArray(rows) ? rows : []).forEach((row) => {
            insert.run(row.he, row.en, row.heFold, row.enFold);
            n += 1;
        });
        return n;
    });
}

/**
 * @param {string} needle Folded query, already trimmed.
 * @param {number} limit
 * @returns {{ he: string, en: string }[]}
 */
function searchGlosses(needle, limit) {
    if (!needle) return [];
    const contains = '%' + escapeLike(needle) + '%';
    const prefix = escapeLike(needle) + '%';
    return db.all(
        `SELECT he, en FROM dictionary_glosses
         WHERE he_fold LIKE ? ESCAPE '\\' OR en_fold LIKE ? ESCAPE '\\'
         ORDER BY CASE
            WHEN he_fold = ? OR en_fold = ? THEN 0
            WHEN he_fold LIKE ? ESCAPE '\\' OR en_fold LIKE ? ESCAPE '\\' THEN 1
            ELSE 2
         END,
         length(en), length(he), he
         LIMIT ?`,
        contains,
        contains,
        needle,
        needle,
        prefix,
        prefix,
        limit
    );
}

/**
 * Replace every dictionary entry. One transaction.
 * @param {{ he: string, heFold: string, pos: string, en: string }[]} rows
 * @returns {number}
 */
function replaceEntries(rows) {
    return db.transaction(() => {
        db.run('DELETE FROM dictionary_entries');
        const insert = db.prepare(
            'INSERT OR IGNORE INTO dictionary_entries (he, he_fold, pos, en) VALUES (?, ?, ?, ?)'
        );
        let n = 0;
        (Array.isArray(rows) ? rows : []).forEach((row) => {
            n += insert.run(row.he, row.heFold, row.pos || '', row.en).changes;
        });
        return n;
    });
}

/**
 * @returns {number}
 */
function countEntries() {
    const row = db.get('SELECT COUNT(*) AS n FROM dictionary_entries');
    return row ? row.n : 0;
}

/**
 * Entries whose folded Hebrew is one of the given forms, in the order of `folds`.
 * @param {string[]} folds
 * @param {number} limit
 * @returns {{ he: string, he_fold: string, pos: string, en: string }[]}
 */
function entriesForFolds(folds, limit) {
    const list = (Array.isArray(folds) ? folds : []).filter(Boolean);
    if (!list.length) return [];
    const marks = list.map(() => '?').join(', ');
    const rows = db.all(
        'SELECT he, he_fold, pos, en FROM dictionary_entries WHERE he_fold IN (' + marks + ') ORDER BY id',
        ...list
    );
    rows.sort((a, b) => list.indexOf(a.he_fold) - list.indexOf(b.he_fold));
    return rows.slice(0, limit);
}

/**
 * Wiktionary English words for one folded Hebrew word.
 * @param {string} fold
 * @param {number} limit
 * @returns {{ he: string, en: string }[]}
 */
function glossesForFold(fold, limit) {
    if (!fold) return [];
    return db.all('SELECT he, en FROM dictionary_glosses WHERE he_fold = ? ORDER BY length(en) LIMIT ?', fold, limit);
}

/**
 * Dictionary entries for the word box. A Hebrew query matches the start of a
 * headword; an English query matches a whole sense (so "dad" does not match "dado").
 * @param {string} needle Folded query.
 * @param {boolean} hebrew
 * @param {number} limit
 * @returns {{ he: string, pos: string, en: string }[]}
 */
function searchEntries(needle, hebrew, limit) {
    if (!needle) return [];
    if (hebrew) {
        return db.all(
            `SELECT he, pos, en FROM dictionary_entries
             WHERE he_fold = ? OR he_fold LIKE ? ESCAPE '\\'
             ORDER BY (he_fold = ?) DESC, length(he_fold), id
             LIMIT ?`,
            needle,
            escapeLike(needle) + '%',
            needle,
            limit * 2
        );
    }
    const rows = db.all(
        `SELECT he, pos, en FROM dictionary_entries
         WHERE lower(en) LIKE ? ESCAPE '\\'
         ORDER BY length(he), id
         LIMIT 400`,
        '%' + escapeLike(needle) + '%'
    );
    const hit = rows.filter((row) => row.en.toLowerCase().split(/[;,]/).some((s) => s.trim() === needle));
    return hit.slice(0, limit * 2);
}

module.exports = {
    searchEntries,
    countEntries,
    entriesForFolds,
    glossesForFold,
    replaceAll,
    replaceEntries,
    searchGlosses
};
