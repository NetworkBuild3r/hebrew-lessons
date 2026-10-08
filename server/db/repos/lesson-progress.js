/**
 * Lesson progress repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function upsert(row) {
    return db.run(
        `INSERT INTO lesson_progress (user_id, slug, state_json, rev, updated_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(user_id, slug) DO UPDATE SET
            state_json = excluded.state_json,
            rev = excluded.rev,
            updated_at = excluded.updated_at`,
        row.user_id,
        row.slug,
        row.state_json,
        row.rev == null ? 0 : row.rev,
        row.updated_at || new Date().toISOString()
    );
}

function find(userId, slug) {
    return db.get(
        'SELECT * FROM lesson_progress WHERE user_id = ? AND slug = ?',
        userId,
        slug
    );
}

function listByUser(userId) {
    return db.all(
        'SELECT * FROM lesson_progress WHERE user_id = ? ORDER BY slug',
        userId
    );
}

function deleteByUser(userId) {
    return db.run('DELETE FROM lesson_progress WHERE user_id = ?', userId);
}

module.exports = {
    upsert,
    find,
    listByUser,
    deleteByUser
};
