/**
 * First admin, from the environment (local .env or a k8s Secret).
 *
 * Called once from server/start.js, after migrations.
 *   - If any admin row exists (active or disabled), do nothing. The env values
 *     are ignored, including on later restarts and after the Secret changes.
 *   - If none exists, create one from ADMIN_USERNAME / ADMIN_PASSWORD.
 *     ADMIN_DISPLAY_NAME is optional.
 *
 * Never log or return the password. A missing or rejected password stops
 * startup; it does not create a user.
 *
 * Provenance: operator request 2026-10-09 (vault-injected first admin).
 */
'use strict';

const db = require('./db');
const accounts = require('./accounts');

function anyAdmin() {
    const row = db.get("SELECT COUNT(*) AS n FROM users WHERE role = 'admin'");
    return !!(row && Number(row.n) > 0);
}

/**
 * @param {NodeJS.ProcessEnv} [env]
 * @returns {{ created: false } | { created: true, username: string }}
 */
function ensureBootstrapAdmin(env) {
    if (anyAdmin()) return { created: false };

    const source = env || process.env;
    const username = String(source.ADMIN_USERNAME || '').trim();
    const password = source.ADMIN_PASSWORD == null ? '' : String(source.ADMIN_PASSWORD);
    const displayName = String(source.ADMIN_DISPLAY_NAME || '').trim();

    if (!username || !password) {
        const err = new Error(
            'No admin user in the database. Set ADMIN_USERNAME and ADMIN_PASSWORD '
            + '(in .env locally, or in the k8s Secret) and restart.'
        );
        err.code = 'bootstrap_admin_missing';
        throw err;
    }

    const user = accounts.createUser({
        username,
        password,
        role: 'admin',
        displayName: displayName || null,
        actor: 'bootstrap'
    });
    return { created: true, username: user.username };
}

module.exports = {
    ensureBootstrapAdmin
};
