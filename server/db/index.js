/**
 * SQLite wrapper — sole module that requires node:sqlite (ADR-0002 §1).
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { dataDir } = require('../paths');

/**
 * Load node:sqlite. Node 22 emits ExperimentalWarning on require; newer Node
 * may not. Track on globalThis so a process emits the warning once even if
 * this module is re-required (tests clear require.cache).
 */
const SQLITE_WARN_KEY = Symbol.for('hebrew.nodeSqlite.warned');
function onSqliteWarning(warning) {
    if (warning && warning.name === 'ExperimentalWarning'
        && /SQLite/i.test(String(warning.message || ''))) {
        globalThis[SQLITE_WARN_KEY] = true;
    }
}
process.on('warning', onSqliteWarning);
const { DatabaseSync } = require('node:sqlite');
process.off('warning', onSqliteWarning);
if (!globalThis[SQLITE_WARN_KEY]) {
    process.emitWarning(
        'SQLite is an experimental feature and might change at any time',
        'ExperimentalWarning'
    );
    globalThis[SQLITE_WARN_KEY] = true;
}

const BUSY_TIMEOUT_MS = 5000;
const DB_FILENAME = 'hebrew.sqlite';

let db = null;

function databasePath() {
    return path.join(dataDir(), DB_FILENAME);
}

function open(options) {
    const opts = options || {};
    if (db && !opts.force) return db;
    if (db) {
        try { db.close(); } catch (_) { /* already closed */ }
        db = null;
    }
    const dir = dataDir();
    fs.mkdirSync(dir, { recursive: true });
    const file = opts.path || databasePath();
    if (file !== ':memory:') {
        fs.mkdirSync(path.dirname(file), { recursive: true });
    }
    db = new DatabaseSync(file, { timeout: BUSY_TIMEOUT_MS });
    db.exec('PRAGMA journal_mode = WAL');
    db.exec('PRAGMA foreign_keys = ON');
    db.exec('PRAGMA busy_timeout = ' + BUSY_TIMEOUT_MS);
    return db;
}

function getDb() {
    if (!db) open();
    return db;
}

function prepare(sql) {
    return getDb().prepare(sql);
}

function get(sql, ...params) {
    return prepare(sql).get(...params);
}

function all(sql, ...params) {
    return prepare(sql).all(...params);
}

function run(sql, ...params) {
    return prepare(sql).run(...params);
}

function transaction(fn) {
    const database = getDb();
    database.exec('BEGIN IMMEDIATE');
    try {
        const result = fn();
        database.exec('COMMIT');
        return result;
    } catch (err) {
        try { database.exec('ROLLBACK'); } catch (_) { /* ignore */ }
        throw err;
    }
}

function close() {
    if (!db) return;
    try { db.close(); } catch (_) { /* ignore */ }
    db = null;
}

function ensureSchemaMigrations(database) {
    database.exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            applied_at TEXT NOT NULL
        )
    `);
}

function migrate() {
    const database = open();
    ensureSchemaMigrations(database);
    const migrationsDir = path.join(__dirname, 'migrations');
    if (!fs.existsSync(migrationsDir)) return;
    const files = fs.readdirSync(migrationsDir)
        .filter((name) => /^\d{3,}-.+\.js$/.test(name))
        .sort();
    const applied = database.prepare(
        'SELECT 1 AS ok FROM schema_migrations WHERE version = ?'
    );
    const record = database.prepare(
        'INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)'
    );
    for (const file of files) {
        const version = Number(file.split('-')[0]);
        if (!Number.isInteger(version)) {
            throw new Error('Invalid migration filename: ' + file);
        }
        if (applied.get(version)) continue;
        const migration = require(path.join(migrationsDir, file));
        if (typeof migration.up !== 'function') {
            throw new Error('Migration missing up(): ' + file);
        }
        database.exec('BEGIN IMMEDIATE');
        try {
            migration.up(database);
            record.run(version, file, new Date().toISOString());
            database.exec('COMMIT');
        } catch (err) {
            try { database.exec('ROLLBACK'); } catch (_) { /* ignore */ }
            throw err;
        }
    }
}

/**
 * Consistent file backup via VACUUM INTO under DATA_DIR/backups/.
 * @returns {string} absolute path of the backup file
 */
function backup() {
    open();
    const backupsDir = path.join(dataDir(), 'backups');
    fs.mkdirSync(backupsDir, { recursive: true });
    const stamp = new Date().toISOString().replace(/[:.]/g, '-');
    const dest = path.resolve(backupsDir, 'hebrew-' + stamp + '.sqlite');
    if (dest.includes("'")) {
        throw new Error('Backup path must not contain a single quote');
    }
    const sqlPath = dest.replace(/\\/g, '/');
    getDb().exec("VACUUM INTO '" + sqlPath + "'");
    return dest;
}

function integrityCheck() {
    open();
    const row = prepare('PRAGMA integrity_check').get();
    return row ? row.integrity_check : null;
}

module.exports = {
    open,
    prepare,
    get,
    all,
    run,
    transaction,
    close,
    migrate,
    backup,
    integrityCheck,
    databasePath,
    DB_FILENAME
};
