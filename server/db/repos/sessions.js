/**
 * Sessions repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function create(row) {
    return db.run(
        `INSERT INTO sessions (
            user_id, token_hash, created_at, last_seen_at, expires_at
        ) VALUES (?, ?, ?, ?, ?)`,
        row.user_id,
        row.token_hash,
        row.created_at,
        row.last_seen_at,
        row.expires_at
    );
}

function findByTokenHash(tokenHash) {
    return db.get('SELECT * FROM sessions WHERE token_hash = ?', tokenHash);
}

function listByUser(userId) {
    return db.all(
        'SELECT * FROM sessions WHERE user_id = ? ORDER BY created_at DESC',
        userId
    );
}

function touch(id, lastSeenAt) {
    return db.run(
        'UPDATE sessions SET last_seen_at = ? WHERE id = ?',
        lastSeenAt,
        id
    );
}

function deleteById(id) {
    return db.run('DELETE FROM sessions WHERE id = ?', id);
}

function deleteByUser(userId) {
    return db.run('DELETE FROM sessions WHERE user_id = ?', userId);
}

module.exports = {
    create,
    findByTokenHash,
    listByUser,
    touch,
    deleteById,
    deleteByUser
};
