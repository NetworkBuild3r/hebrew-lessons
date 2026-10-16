/**
 * Per-learner concept mastery — prepared statements only. No file IO.
 * Stores attempts, correct, and the recent score. Not per-event history.
 * Provenance: INIT-006/SPEC-003
 */
'use strict';

const db = require('../index');

const COLUMNS = 'user_id, concept_key, attempts, correct, ewma, updated_at';

/**
 * @param {string|null|undefined} text
 * @returns {string}
 */
function exactKey(text) {
    return String(text == null ? '' : text).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/**
 * @param {{
 *   user_id: number,
 *   concept_key: string,
 *   attempts?: number,
 *   correct?: number,
 *   ewma?: number|null,
 *   updated_at?: string
 * }} row
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function upsert(row) {
    return db.run(
        `INSERT INTO concept_mastery (
            user_id, concept_key, attempts, correct, ewma, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(user_id, concept_key) DO UPDATE SET
            attempts = excluded.attempts,
            correct = excluded.correct,
            ewma = excluded.ewma,
            updated_at = excluded.updated_at`,
        row.user_id,
        exactKey(row.concept_key),
        row.attempts == null ? 0 : row.attempts,
        row.correct == null ? 0 : row.correct,
        row.ewma == null ? null : row.ewma,
        row.updated_at || new Date().toISOString()
    );
}

/**
 * @param {number} userId
 * @param {string} conceptKey
 * @returns {object|undefined}
 */
function find(userId, conceptKey) {
    return db.get(
        'SELECT ' + COLUMNS + ' FROM concept_mastery WHERE user_id = ? AND concept_key = ?',
        userId,
        exactKey(conceptKey)
    );
}

/**
 * @param {number} userId
 * @returns {object[]}
 */
function listByUser(userId) {
    return db.all(
        'SELECT ' + COLUMNS + ' FROM concept_mastery WHERE user_id = ? ORDER BY concept_key',
        userId
    );
}

/** Stay under SQLite's bound-variable limit. One slot is the user id. */
const IN_CHUNK = 200;

/**
 * Mastery rows for this user and these concept keys. One query per chunk.
 * Provenance: INIT-006/SPEC-006
 * @param {number} userId
 * @param {string[]} keys
 * @returns {object[]}
 */
function listByKeys(userId, keys) {
    const unique = [];
    const seen = new Set();
    const list = Array.isArray(keys) ? keys : [];
    for (const raw of list) {
        const key = exactKey(raw);
        if (!key || seen.has(key)) continue;
        seen.add(key);
        unique.push(key);
    }
    if (!unique.length) return [];
    const out = [];
    for (let i = 0; i < unique.length; i += IN_CHUNK) {
        const slice = unique.slice(i, i + IN_CHUNK);
        const marks = slice.map(() => '?').join(', ');
        out.push(...db.all(
            'SELECT ' + COLUMNS + ' FROM concept_mastery WHERE user_id = ? AND concept_key IN ('
                + marks + ')',
            userId,
            ...slice
        ));
    }
    return out;
}

/**
 * Remove this learner's mastery rows. Used when rebuilding from events.
 * Provenance: INIT-006/SPEC-006
 * @param {number} userId
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function deleteByUser(userId) {
    return db.run('DELETE FROM concept_mastery WHERE user_id = ?', userId);
}

module.exports = {
    upsert,
    find,
    listByUser,
    listByKeys,
    deleteByUser
};
