/**
 * Practice events repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function insert(row) {
    return db.run(
        `INSERT INTO practice_events (
            id, user_id, word, direction, result, at, slug, activity
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        row.id,
        row.user_id,
        row.word,
        row.direction,
        row.result,
        row.at,
        row.slug == null ? null : row.slug,
        row.activity == null ? null : row.activity
    );
}

function find(userId, id) {
    return db.get(
        'SELECT * FROM practice_events WHERE user_id = ? AND id = ?',
        userId,
        id
    );
}

function listByUser(userId, limit) {
    const cap = limit == null ? 200 : limit;
    return db.all(
        'SELECT * FROM practice_events WHERE user_id = ? ORDER BY at DESC LIMIT ?',
        userId,
        cap
    );
}

function deleteByUser(userId) {
    return db.run('DELETE FROM practice_events WHERE user_id = ?', userId);
}

module.exports = {
    insert,
    find,
    listByUser,
    deleteByUser
};
