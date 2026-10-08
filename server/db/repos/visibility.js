/**
 * Lesson visibility + grants repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function setVisibility(slug, mode, updatedAt) {
    return db.run(
        `INSERT INTO lesson_visibility (slug, mode, updated_at)
         VALUES (?, ?, ?)
         ON CONFLICT(slug) DO UPDATE SET
            mode = excluded.mode,
            updated_at = excluded.updated_at`,
        slug,
        mode,
        updatedAt || new Date().toISOString()
    );
}

function getVisibility(slug) {
    return db.get('SELECT * FROM lesson_visibility WHERE slug = ?', slug);
}

function listVisibility() {
    return db.all('SELECT * FROM lesson_visibility ORDER BY slug');
}

function grant(slug, userId) {
    return db.run(
        'INSERT OR IGNORE INTO lesson_grants (slug, user_id) VALUES (?, ?)',
        slug,
        userId
    );
}

function revoke(slug, userId) {
    return db.run(
        'DELETE FROM lesson_grants WHERE slug = ? AND user_id = ?',
        slug,
        userId
    );
}

function listGrants(slug) {
    return db.all(
        'SELECT * FROM lesson_grants WHERE slug = ? ORDER BY user_id',
        slug
    );
}

function listGrantsForUser(userId) {
    return db.all(
        'SELECT * FROM lesson_grants WHERE user_id = ? ORDER BY slug',
        userId
    );
}

module.exports = {
    setVisibility,
    getVisibility,
    listVisibility,
    grant,
    revoke,
    listGrants,
    listGrantsForUser
};
