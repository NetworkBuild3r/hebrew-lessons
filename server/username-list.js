/**
 * Username list checks shared by worksheet visibility and plan-lesson users.
 * The messages are the set_lesson_visibility contract. Callers reuse this
 * module; they do not copy the checks.
 */
'use strict';

const usersRepo = require('./db/repos/users');

/**
 * @param {string} message
 * @returns {Error}
 */
function usernameListError(message) {
    const err = new Error(message);
    err.code = 'username_list';
    return err;
}

/**
 * Active accounts named by a username list, in input order.
 * Duplicate spellings of the same trimmed string are skipped. A second
 * spelling that trims to a different string is looked up again, matching
 * set_lesson_visibility.
 * @param {string[]} usernames
 * @returns {Array<{ id: number, username: string, status: string }>}
 */
function resolveActiveUsers(usernames) {
    if (!Array.isArray(usernames)) {
        throw usernameListError('visibility.users must be an array of usernames.');
    }
    const rows = [];
    const seen = new Set();
    for (const raw of usernames) {
        const name = String(raw || '').trim();
        if (!name) throw usernameListError('visibility.users contains an empty username.');
        if (seen.has(name)) continue;
        seen.add(name);
        const row = usersRepo.findByUsername(name);
        if (!row || row.status !== 'active') {
            throw usernameListError('Unknown or inactive username: "' + name + '".');
        }
        rows.push(row);
    }
    return rows;
}

/**
 * Resolve usernames to active user ids before any write.
 * @param {string[]} usernames
 * @returns {number[]}
 */
function resolveActiveUserIds(usernames) {
    return resolveActiveUsers(usernames).map((row) => Number(row.id));
}

/**
 * Stored usernames, one per account, in first-seen order.
 * The users table keeps username = lower(username); this is that stored form.
 * @param {Array<{ id: number, username: string }>} rows
 * @returns {string[]}
 */
function canonicalUsernames(rows) {
    const names = [];
    const seen = new Set();
    for (const row of rows) {
        if (seen.has(row.username)) continue;
        seen.add(row.username);
        names.push(row.username);
    }
    return names;
}

module.exports = {
    resolveActiveUsers,
    resolveActiveUserIds,
    canonicalUsernames
};
