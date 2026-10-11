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

/**
 * A separate read-only connection, outside the shared one. It cannot write,
 * migrate, or checkpoint, so the database file is left as it was. A missing
 * file throws instead of being created.
 * Provenance: INIT-004/SPEC-020
 * @param {string} [file] Defaults to DATA_DIR/hebrew.sqlite.
 * @returns {import('node:sqlite').DatabaseSync} Close it when done.
 */
function openReadOnly(file) {
    return new DatabaseSync(file || databasePath(), { readOnly: true, timeout: BUSY_TIMEOUT_MS });
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
 * True when candidate is parent or a path under parent.
 * @param {string} candidate
 * @param {string} parent
 * @returns {boolean}
 */
function isInsideOrEqual(candidate, parent) {
    const resolved = path.resolve(candidate);
    const base = path.resolve(parent);
    if (resolved === base) return true;
    const prefix = base.endsWith(path.sep) ? base : base + path.sep;
    return resolved.startsWith(prefix);
}

/**
 * Real path of candidate, which must stay inside parent. Missing paths return null.
 * A link that resolves outside parent is refused.
 * Provenance: INIT-004/SPEC-017
 * @param {string} candidate
 * @param {string} parentReal
 * @returns {string|null}
 */
function realInside(candidate, parentReal) {
    let real;
    try {
        real = fs.realpathSync.native(candidate);
    } catch (err) {
        if (err && err.code === 'ENOENT') return null;
        throw err;
    }
    if (!isInsideOrEqual(real, parentReal)) {
        throw new Error('Backup refused a media path outside DATA_DIR/media');
    }
    return real;
}

/**
 * Copy one media tree into the backup directory. Sources outside DATA_DIR/media
 * are refused. Missing source becomes an empty directory. Nothing is deleted.
 * Provenance: INIT-004/SPEC-017
 * @param {string} src
 * @param {string} dest
 * @param {string} mediaRootReal
 * @param {Set<string>} seen
 */
function copyMediaEntries(src, dest, mediaRootReal, seen) {
    if (seen.has(src)) return;
    seen.add(src);
    fs.mkdirSync(dest, { recursive: true });
    for (const name of fs.readdirSync(src)) {
        const from = path.join(src, name);
        const to = path.resolve(dest, name);
        if (!isInsideOrEqual(to, dest)) {
            throw new Error('Backup refused a media path outside the backup directory');
        }
        const listed = fs.lstatSync(from);
        if (listed.isSymbolicLink()) {
            const real = realInside(from, mediaRootReal);
            if (!real) continue;
            const st = fs.statSync(real);
            if (st.isDirectory()) {
                copyMediaEntries(real, to, mediaRootReal, seen);
            } else if (st.isFile()) {
                fs.copyFileSync(real, to);
            } else {
                throw new Error('Backup refused a non-file in media');
            }
            continue;
        }
        if (listed.isDirectory()) {
            const real = realInside(from, mediaRootReal);
            if (!real) continue;
            copyMediaEntries(real, to, mediaRootReal, seen);
            continue;
        }
        if (listed.isFile()) {
            fs.copyFileSync(from, to);
            continue;
        }
        throw new Error('Backup refused a non-file in media');
    }
}

/**
 * Copy DATA_DIR/media into backupMedia, including media/_archive.
 * Creates both directories when the live tree is missing. Does not read
 * LESSONS_DIR or any other path outside DATA_DIR/media.
 * Provenance: INIT-004/SPEC-017
 * @param {string} backupMedia absolute destination (…/media)
 * @param {string} dataRoot DATA_DIR
 */
function copyMediaTree(backupMedia, dataRoot) {
    const dataReal = fs.realpathSync.native(dataRoot);
    const parentReal = fs.realpathSync.native(path.dirname(backupMedia));
    const destReal = path.join(parentReal, path.basename(backupMedia));
    if (!isInsideOrEqual(destReal, dataReal) || path.resolve(destReal) === path.resolve(dataReal)) {
        throw new Error('Backup media path must stay under DATA_DIR');
    }
    const src = path.join(dataRoot, 'media');
    fs.mkdirSync(backupMedia, { recursive: true });
    fs.mkdirSync(path.join(backupMedia, '_archive'), { recursive: true });
    if (!fs.existsSync(src)) return;
    const mediaReal = realInside(src, dataReal);
    if (!mediaReal) return;
    if (path.resolve(mediaReal) === path.resolve(dataReal)) {
        throw new Error('Backup refused a media path outside DATA_DIR/media');
    }
    copyMediaEntries(mediaReal, backupMedia, mediaReal, new Set());
    fs.mkdirSync(path.join(backupMedia, '_archive'), { recursive: true });
}

/**
 * Consistent backup under DATA_DIR/backups/hebrew-<stamp>/.
 * The directory holds hebrew.sqlite (VACUUM INTO), media/, and media/_archive/.
 * Recordings outside DATA_DIR are not read and are not required.
 * Provenance: INIT-002/SPEC-003, media trees INIT-004/SPEC-017
 * @returns {string} absolute path of the backup's hebrew.sqlite
 */
function backup() {
    open();
    const root = dataDir();
    const backupsDir = path.join(root, 'backups');
    fs.mkdirSync(backupsDir, { recursive: true });
    const stamp = new Date().toISOString().replace(/[:.]/g, '-');
    const bundle = path.resolve(backupsDir, 'hebrew-' + stamp);
    const dest = path.resolve(bundle, 'hebrew.sqlite');
    if (bundle.includes("'") || dest.includes("'")) {
        throw new Error('Backup path must not contain a single quote');
    }
    fs.mkdirSync(bundle, { recursive: true });
    const sqlPath = dest.replace(/\\/g, '/');
    getDb().exec("VACUUM INTO '" + sqlPath + "'");
    copyMediaTree(path.join(bundle, 'media'), root);
    return dest;
}

function integrityCheck() {
    open();
    const row = prepare('PRAGMA integrity_check').get();
    return row ? row.integrity_check : null;
}

module.exports = {
    open,
    openReadOnly,
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
