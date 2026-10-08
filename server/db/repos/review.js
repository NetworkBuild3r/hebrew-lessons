/**
 * Review items repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function upsert(row) {
    return db.run(
        `INSERT INTO review_items (
            user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(user_id, word, direction) DO UPDATE SET
            due = excluded.due,
            step = excluded.step,
            streak = excluded.streak,
            lapses = excluded.lapses,
            last_result = excluded.last_result,
            last_at = excluded.last_at,
            sources = excluded.sources`,
        row.user_id,
        row.word,
        row.direction,
        row.due,
        row.step == null ? 0 : row.step,
        row.streak == null ? 0 : row.streak,
        row.lapses == null ? 0 : row.lapses,
        row.last_result == null ? null : row.last_result,
        row.last_at == null ? null : row.last_at,
        row.sources == null ? '[]' : row.sources
    );
}

function find(userId, word, direction) {
    return db.get(
        'SELECT * FROM review_items WHERE user_id = ? AND word = ? AND direction = ?',
        userId,
        word,
        direction
    );
}

function listDue(userId, beforeIso) {
    return db.all(
        'SELECT * FROM review_items WHERE user_id = ? AND due <= ? ORDER BY due ASC',
        userId,
        beforeIso
    );
}

function listByUser(userId) {
    return db.all(
        'SELECT * FROM review_items WHERE user_id = ? ORDER BY word, direction',
        userId
    );
}

function deleteByUser(userId) {
    return db.run('DELETE FROM review_items WHERE user_id = ?', userId);
}

module.exports = {
    upsert,
    find,
    listDue,
    listByUser,
    deleteByUser
};
