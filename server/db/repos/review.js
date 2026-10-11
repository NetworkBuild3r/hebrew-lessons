/**
 * Review items repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 * FSRS columns and setSuspended: INIT-006/SPEC-002
 */
'use strict';

const db = require('../index');

const REVIEW_COLUMNS = `
    id, user_id, word, direction, due, step, streak, lapses,
    last_result, last_at, sources,
    stability, difficulty, reps, fsrs_state, last_grade, suspended
`;

function provided(row, key) {
    return Object.prototype.hasOwnProperty.call(row, key) ? 1 : 0;
}

/**
 * Insert or replace the ladder fields. FSRS fields are written only when the
 * caller sets them on the row. An omitted field keeps the stored value, so a
 * ladder update does not clear stability or suspension.
 * @param {object} row
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function upsert(row) {
    const stabilityProvided = provided(row, 'stability');
    const difficultyProvided = provided(row, 'difficulty');
    const repsProvided = provided(row, 'reps');
    const stateProvided = provided(row, 'fsrs_state');
    const gradeProvided = provided(row, 'last_grade');
    const suspendedProvided = provided(row, 'suspended');
    const suspendedFlag = (row.suspended === true || row.suspended === 1) ? 1 : 0;
    return db.run(
        `INSERT INTO review_items (
            user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources,
            stability, difficulty, reps, fsrs_state, last_grade, suspended
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(user_id, word, direction) DO UPDATE SET
            due = excluded.due,
            step = excluded.step,
            streak = excluded.streak,
            lapses = excluded.lapses,
            last_result = excluded.last_result,
            last_at = excluded.last_at,
            sources = excluded.sources,
            stability = CASE WHEN ? = 1 THEN excluded.stability ELSE review_items.stability END,
            difficulty = CASE WHEN ? = 1 THEN excluded.difficulty ELSE review_items.difficulty END,
            reps = CASE WHEN ? = 1 THEN excluded.reps ELSE review_items.reps END,
            fsrs_state = CASE WHEN ? = 1 THEN excluded.fsrs_state ELSE review_items.fsrs_state END,
            last_grade = CASE WHEN ? = 1 THEN excluded.last_grade ELSE review_items.last_grade END,
            suspended = CASE WHEN ? = 1 THEN excluded.suspended ELSE review_items.suspended END`,
        row.user_id,
        row.word,
        row.direction,
        row.due,
        row.step == null ? 0 : row.step,
        row.streak == null ? 0 : row.streak,
        row.lapses == null ? 0 : row.lapses,
        row.last_result == null ? null : row.last_result,
        row.last_at == null ? null : row.last_at,
        row.sources == null ? '[]' : row.sources,
        stabilityProvided ? (row.stability == null ? null : row.stability) : null,
        difficultyProvided ? (row.difficulty == null ? null : row.difficulty) : null,
        repsProvided && row.reps != null ? row.reps : 0,
        stateProvided ? (row.fsrs_state == null ? null : row.fsrs_state) : null,
        gradeProvided ? (row.last_grade == null ? null : row.last_grade) : null,
        suspendedProvided ? suspendedFlag : 0,
        stabilityProvided,
        difficultyProvided,
        repsProvided,
        stateProvided,
        gradeProvided,
        suspendedProvided
    );
}

/**
 * @param {number} userId
 * @param {string} word
 * @param {string} direction
 */
function find(userId, word, direction) {
    return db.get(
        'SELECT ' + REVIEW_COLUMNS + ' FROM review_items WHERE user_id = ? AND word = ? AND direction = ?',
        userId,
        word,
        direction
    );
}

/**
 * @param {number} userId
 * @param {string} beforeIso
 */
function listDue(userId, beforeIso) {
    return db.all(
        'SELECT ' + REVIEW_COLUMNS + ' FROM review_items WHERE user_id = ? AND due <= ? ORDER BY due ASC',
        userId,
        beforeIso
    );
}

/**
 * @param {number} userId
 */
function listByUser(userId) {
    return db.all(
        'SELECT ' + REVIEW_COLUMNS + ' FROM review_items WHERE user_id = ? ORDER BY word, direction',
        userId
    );
}

/**
 * This learner's review rows, plus whether a word-card pass event exists
 * for that word and direction. One query. The flag matches metrics()'s
 * "ever passed" event test.
 * Provenance: INIT-006/SPEC-006
 * @param {number} userId
 * @returns {object[]}
 */
function listForLearner(userId) {
    return db.all(
        'SELECT ' + REVIEW_COLUMNS + `,
            EXISTS (
                SELECT 1 FROM practice_events AS e
                WHERE e.user_id = review_items.user_id
                  AND e.word = review_items.word
                  AND e.direction = review_items.direction
                  AND e.result = 'pass'
                  AND e.kind = 'word'
            ) AS passed_event
         FROM review_items
         WHERE review_items.user_id = ?
         ORDER BY review_items.word, review_items.direction`,
        userId
    );
}

/**
 * Set suspended on every direction row for this word. Missing rows stay missing.
 * @param {number} userId
 * @param {string} word
 * @param {boolean} suspended
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function setSuspended(userId, word, suspended) {
    const flag = suspended === true || suspended === 1 ? 1 : 0;
    return db.run(
        'UPDATE review_items SET suspended = ? WHERE user_id = ? AND word = ?',
        flag,
        userId,
        word
    );
}

/**
 * @param {number} userId
 */
function deleteByUser(userId) {
    return db.run('DELETE FROM review_items WHERE user_id = ?', userId);
}

module.exports = {
    upsert,
    find,
    listDue,
    listByUser,
    listForLearner,
    setSuspended,
    deleteByUser
};
