/**
 * Practice events repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 * grade, kind, and lemma: INIT-006/SPEC-002
 */
'use strict';

const db = require('../index');

const EVENT_COLUMNS = `
    id, user_id, word, direction, result, at, slug, activity,
    grade, kind, lemma
`;

/**
 * Insert one practice event. Omitted kind is word. Omitted grade and lemma
 * are null. An invalid grade or kind fails the table CHECK.
 * @param {object} row
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function insert(row) {
    const kind = row.kind == null ? 'word' : row.kind;
    return db.run(
        `INSERT INTO practice_events (
            id, user_id, word, direction, result, at, slug, activity,
            grade, kind, lemma
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        row.id,
        row.user_id,
        row.word,
        row.direction,
        row.result,
        row.at,
        row.slug == null ? null : row.slug,
        row.activity == null ? null : row.activity,
        row.grade == null ? null : row.grade,
        kind,
        row.lemma == null ? null : row.lemma
    );
}

/**
 * @param {number} userId
 * @param {string} id
 */
function find(userId, id) {
    return db.get(
        'SELECT ' + EVENT_COLUMNS + ' FROM practice_events WHERE user_id = ? AND id = ?',
        userId,
        id
    );
}

/**
 * @param {number} userId
 * @param {number} [limit]
 */
function listByUser(userId, limit) {
    const cap = limit == null ? 200 : limit;
    return db.all(
        'SELECT ' + EVENT_COLUMNS + ' FROM practice_events WHERE user_id = ? ORDER BY at DESC LIMIT ?',
        userId,
        cap
    );
}

/**
 * @param {number} userId
 */
function deleteByUser(userId) {
    return db.run('DELETE FROM practice_events WHERE user_id = ?', userId);
}

/**
 * Form pass and miss rows for one learner, oldest first.
 * Rebuild walks this list. Word-card rows and not_yet are excluded.
 * Provenance: INIT-006/SPEC-006
 * @param {number} userId
 * @returns {object[]}
 */
function listFormOutcomes(userId) {
    return db.all(
        `SELECT id, word, lemma, result, at
         FROM practice_events
         WHERE user_id = ? AND kind = 'form' AND result IN ('pass', 'miss')
         ORDER BY at ASC, id ASC`,
        userId
    );
}

module.exports = {
    insert,
    find,
    listByUser,
    listFormOutcomes,
    deleteByUser
};
