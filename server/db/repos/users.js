/**
 * Users repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 * AI-planning switch reads and writes: INIT-004/SPEC-006
 * Learner lookups for profile and coverage reads: INIT-006/SPEC-007
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

/**
 * Account named by a plan assigneeId. The id is the username.
 * Does not select the password hash or the display-name column.
 * Provenance: INIT-004/SPEC-010
 * @param {string} username
 * @returns {{ id: number, username: string, status: string }|undefined}
 */
function findAccount(username) {
    if (typeof username !== 'string' || username.length === 0 || username.length > 64) {
        return undefined;
    }
    return db.get(
        'SELECT id, username, status FROM users WHERE username = ?',
        normalizeUsername(username)
    );
}

/** The columns a learner profile or coverage read needs. No secret, name, or role. */
const LEARNER_COLUMNS = 'id, username, status, ai_planning';

/**
 * The account a profile or coverage read is about, by id.
 * Does not select the password hash or the display-name column.
 * Provenance: INIT-006/SPEC-007
 * @param {number} id
 * @returns {{ id: number, username: string, status: string, ai_planning: number }|undefined}
 */
function findLearnerById(id) {
    return db.get('SELECT ' + LEARNER_COLUMNS + ' FROM users WHERE id = ?', id);
}

/**
 * The account a profile or coverage read is about, by username, with the
 * same length bound as findAccount. Does not select the password hash or the
 * display-name column.
 * Provenance: INIT-006/SPEC-007
 * @param {string} username
 * @returns {{ id: number, username: string, status: string, ai_planning: number }|undefined}
 */
function findLearnerByUsername(username) {
    if (typeof username !== 'string' || username.length === 0 || username.length > 64) {
        return undefined;
    }
    return db.get(
        'SELECT ' + LEARNER_COLUMNS + ' FROM users WHERE username = ?',
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

/**
 * @param {boolean|number} value
 * @returns {0|1}
 */
function toPlanningFlag(value) {
    if (value === true || value === 1) return 1;
    if (value === false || value === 0) return 0;
    throw new TypeError('ai_planning must be 0 or 1');
}

/**
 * Read the AI-planning switch. 0 is off. Null when the user does not exist.
 * Does not write an audit row.
 * @param {number} userId
 * @returns {0|1|null}
 */
function getAiPlanning(userId) {
    const row = db.get(
        'SELECT ai_planning FROM users WHERE id = ?',
        userId
    );
    if (!row) return null;
    return toPlanningFlag(row.ai_planning);
}

/**
 * Set the AI-planning switch and return the previous and new values.
 * The audit row is written by the caller (INIT-006 SPEC-010), not here.
 * @param {number} userId
 * @param {boolean|number} enabled
 * @returns {{ previous: 0|1, next: 0|1 }|null}
 */
function setAiPlanning(userId, enabled) {
    const next = toPlanningFlag(enabled);
    const current = db.get(
        'SELECT ai_planning FROM users WHERE id = ?',
        userId
    );
    if (!current) return null;
    const previous = toPlanningFlag(current.ai_planning);
    const result = db.run(
        'UPDATE users SET ai_planning = ?, updated_at = ? WHERE id = ?',
        next,
        new Date().toISOString(),
        userId
    );
    if (!result || Number(result.changes) !== 1) return null;
    return { previous, next };
}

module.exports = {
    create,
    findById,
    findByUsername,
    findAccount,
    findLearnerById,
    findLearnerByUsername,
    findByProvider,
    updateStatus,
    deleteById,
    listAll,
    normalizeUsername,
    getAiPlanning,
    setAiPlanning
};
