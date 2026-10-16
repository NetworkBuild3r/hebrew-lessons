/**
 * Optional Google email on a user, for app sign-in with the shared Google client.
 * Null means that account cannot sign in with Google. Password sign-in is unchanged.
 */
'use strict';

const VERSION = 11;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    const columns = db.prepare('PRAGMA table_info(users)').all();
    if (!columns.some((col) => col.name === 'google_email')) {
        db.exec('ALTER TABLE users ADD COLUMN google_email TEXT');
    }
    db.exec(`
        CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_email
        ON users (google_email) WHERE google_email IS NOT NULL
    `);
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec('DROP INDEX IF EXISTS idx_users_google_email');
    const columns = db.prepare('PRAGMA table_info(users)').all();
    if (columns.some((col) => col.name === 'google_email')) {
        db.exec('ALTER TABLE users DROP COLUMN google_email');
    }
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { VERSION, up, down };
