/**
 * Tanakh verse store. Bulk replace for the corpus loader; read lookups for
 * verse-of-the-day, chapter reading, and word study. Provenance strings live
 * in tanakh_meta.
 */
'use strict';

const db = require('../../db');

/**
 * Replace every verse and (when given) all provenance meta. One transaction.
 * @param {{ book: string, chapter: number, verse: number, he: string, en: string }[]} rows
 * @param {Record<string, string>} [meta]
 * @returns {number} verses written
 */
function replaceAll(rows, meta) {
    return db.transaction(() => {
        db.run('DELETE FROM tanakh_verses');
        const insert = db.prepare(
            'INSERT OR IGNORE INTO tanakh_verses (book, chapter, verse, he, en) VALUES (?, ?, ?, ?, ?)'
        );
        let n = 0;
        (Array.isArray(rows) ? rows : []).forEach((row) => {
            n += insert.run(row.book, row.chapter, row.verse, row.he || '', row.en || '').changes;
        });
        if (meta && typeof meta === 'object') {
            db.run('DELETE FROM tanakh_meta');
            const put = db.prepare('INSERT INTO tanakh_meta (key, value) VALUES (?, ?)');
            Object.keys(meta).forEach((key) => {
                put.run(key, String(meta[key]));
            });
        }
        return n;
    });
}

/**
 * @returns {number}
 */
function countVerses() {
    const row = db.get('SELECT COUNT(*) AS n FROM tanakh_verses');
    return row ? row.n : 0;
}

/**
 * One row per book, in corpus order.
 * @returns {{ book: string, verses: number, chapters: number }[]}
 */
function books() {
    return db.all(
        `SELECT book, COUNT(*) AS verses, MAX(chapter) AS chapters
         FROM tanakh_verses GROUP BY book ORDER BY MIN(id)`
    );
}

/**
 * Verses of one chapter in order.
 * @param {unknown} book
 * @param {unknown} chapterNum
 * @returns {{ verse: number, he: string, en: string }[]}
 */
function chapter(book, chapterNum) {
    if (typeof book !== 'string' || !Number.isInteger(chapterNum) || chapterNum < 1) return [];
    return db.all(
        'SELECT verse, he, en FROM tanakh_verses WHERE book = ? AND chapter = ? ORDER BY verse',
        book,
        chapterNum
    );
}

/**
 * One verse, or null.
 * @param {unknown} book
 * @param {unknown} chapterNum
 * @param {unknown} verseNum
 * @returns {{ book: string, chapter: number, verse: number, he: string, en: string }|null}
 */
function verse(book, chapterNum, verseNum) {
    if (typeof book !== 'string' || !Number.isInteger(chapterNum) || !Number.isInteger(verseNum)) {
        return null;
    }
    return db.get(
        'SELECT book, chapter, verse, he, en FROM tanakh_verses WHERE book = ? AND chapter = ? AND verse = ?',
        book,
        chapterNum,
        verseNum
    ) || null;
}

/**
 * Provenance meta as a plain object.
 * @returns {Record<string, string>}
 */
function allMeta() {
    const out = {};
    db.all('SELECT key, value FROM tanakh_meta ORDER BY key').forEach((row) => {
        out[row.key] = row.value;
    });
    return out;
}

module.exports = {
    replaceAll,
    countVerses,
    books,
    chapter,
    verse,
    allMeta
};
