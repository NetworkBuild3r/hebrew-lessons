/**
 * Users repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function normalizeUsername(username) {
    return String(username).toLowerCase();
}

function create(row) {
    const now = row.created_at || new Date().toISOString();
    return db.run(
        `INSERT INTO users (
            username, password_hash, display_name, role, status,
            auth_provider, provider_subject, must_change_password,
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        normalizeUsername(row.username),
        row.password_hash,
        row.display_name == null ? null : row.display_name,
        row.role || 'user',
        row.status || 'active',
        row.auth_provider || 'local',
        row.provider_subject == null ? null : row.provider_subject,
        row.must_change_password ? 1 : 0,
        now,
        row.updated_at || now
    );
}

function findById(id) {
    return db.get('SELECT * FROM users WHERE id = ?', id);
}

function findByUsername(username) {
    return db.get(
        'SELECT * FROM users WHERE username = ?',
        normalizeUsername(username)
    );
}

function findByProvider(authProvider, providerSubject) {
    return db.get(
        'SELECT * FROM users WHERE auth_provider = ? AND provider_subject = ?',
        authProvider,
        providerSubject
    );
}

function updateStatus(id, status) {
    return db.run(
        'UPDATE users SET status = ?, updated_at = ? WHERE id = ?',
        status,
        new Date().toISOString(),
        id
    );
}

function deleteById(id) {
    return db.run('DELETE FROM users WHERE id = ?', id);
}

function listAll() {
    return db.all('SELECT * FROM users ORDER BY username ASC');
}

module.exports = {
    create,
    findById,
    findByUsername,
    findByProvider,
    updateStatus,
    deleteById,
    listAll,
    normalizeUsername
};
