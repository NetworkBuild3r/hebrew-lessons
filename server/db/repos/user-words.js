/**
 * Personal library rows — prepared statements only. No file IO.
 * The word is stored as exactKey. This table does not hold schedule state.
 * Provenance: INIT-006/SPEC-003
 */
'use strict';

const db = require('../index');

const COLUMNS = 'user_id, word, source, added_at, gloss_en';

/**
 * ADR-0001 exactKey: NFC, collapse whitespace, trimmed.
 * @param {string|null|undefined} text
 * @returns {string}
 */
function exactKey(text) {
    return String(text == null ? '' : text).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/**
 * @param {{ user_id: number, word: string, source: string, added_at?: string, gloss_en?: string|null }} row
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function add(row) {
    return db.run(
        `INSERT INTO user_words (user_id, word, source, added_at, gloss_en)
         VALUES (?, ?, ?, ?, ?)`,
        row.user_id,
        exactKey(row.word),
        row.source,
        row.added_at || new Date().toISOString(),
        row.gloss_en == null ? null : row.gloss_en
    );
}

/**
 * @param {number} userId
 * @param {string} word
 * @returns {object|undefined}
 */
function find(userId, word) {
    return db.get(
        'SELECT ' + COLUMNS + ' FROM user_words WHERE user_id = ? AND word = ?',
        userId,
        exactKey(word)
    );
}

/**
 * @param {number} userId
 * @returns {object[]}
 */
function listByUser(userId) {
    return db.all(
        'SELECT ' + COLUMNS + ' FROM user_words WHERE user_id = ? ORDER BY word',
        userId
    );
}

module.exports = {
    add,
    find,
    listByUser,
    exactKey
};
