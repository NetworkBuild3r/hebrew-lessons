/**
 * SQLite schema, migrations, DATA_DIR guard, backup.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const { describe, it, before, after, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

function freshDbModule(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    delete require.cache[require.resolve('../server/paths')];
    delete require.cache[require.resolve('../server/db')];
    delete require.cache[require.resolve('../server/db/migrations/001-initial')];
    try {
        delete require.cache[require.resolve('../server/db/migrations/002-plan-store')];
    } catch (_) { /* not loaded yet */ }
    try {
        delete require.cache[require.resolve('../server/db/migrations/003-media-and-ai-planning')];
    } catch (_) { /* not loaded yet */ }
    try {
        delete require.cache[require.resolve('../server/db/migrations/004-fsrs-review-state')];
    } catch (_) { /* not loaded yet */ }
    const db = require('../server/db');
    db.close();
    return db;
}

describe('DATA_DIR served-root guard (ac-1)', () => {
    const saved = {
        DATA_DIR: process.env.DATA_DIR,
        HEBREW_ROOT: process.env.HEBREW_ROOT,
        LESSONS_DIR: process.env.LESSONS_DIR
    };

    afterEach(() => {
        for (const [key, value] of Object.entries(saved)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        delete require.cache[require.resolve('../server/paths')];
    });

    it('allows default <root>/data', () => {
        delete process.env.DATA_DIR;
        process.env.HEBREW_ROOT = root;
        const paths = require('../server/paths');
        assert.equal(paths.dataDir(), path.join(root, 'data'));
    });

    it('throws when DATA_DIR is inside shared/', () => {
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = path.join(root, 'shared', 'db-data');
        const paths = require('../server/paths');
        assert.throws(() => paths.dataDir(), /served root/i);
    });

    it('throws when DATA_DIR equals shared/', () => {
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = path.join(root, 'shared');
        const paths = require('../server/paths');
        assert.throws(() => paths.dataDir(), /served root/i);
    });

    it('throws when DATA_DIR is inside review/', () => {
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = path.join(root, 'review', 'nested');
        const paths = require('../server/paths');
        assert.throws(() => paths.dataDir(), /served root/i);
    });

    it('throws when DATA_DIR is inside LESSONS_DIR', () => {
        const tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lessons-'));
        process.env.HEBREW_ROOT = root;
        process.env.LESSONS_DIR = tmpLessons;
        process.env.DATA_DIR = path.join(tmpLessons, 'data');
        const paths = require('../server/paths');
        assert.throws(() => paths.dataDir(), /served root/i);
        fs.rmSync(tmpLessons, { recursive: true, force: true });
    });
});

describe('migrate + schema (ac-2, ac-3, ac-6)', () => {
    let tmp;
    let db;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-db-'));
        db = freshDbModule(tmp);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    beforeEach(() => {
        db.close();
        db.open({ force: true });
    });

    it('migrate creates ADR-0002 tables and records version 1; second run is no-op', () => {
        db.migrate();
        const tables = db.all(
            `SELECT name FROM sqlite_master
             WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
             ORDER BY name`
        ).map((row) => row.name);

        const expected = [
            'audit_log',
            'concept_mastery',
            'grammar_concepts',
            'lesson_grants',
            'lesson_progress',
            'lesson_visibility',
            'lexicon_entries',
            'media',
            'meta',
            'plans',
            'practice_events',
            'review_items',
            'schema_migrations',
            'sessions',
            'tanakh_meta',
            'tanakh_verses',
            'user_words',
            'users'
        ];
        for (const name of expected) {
            assert.ok(tables.includes(name), 'missing table ' + name);
        }

        const versions = db.all(
            'SELECT version, name FROM schema_migrations ORDER BY version'
        );
        const byVersion = new Map(versions.map((row) => [row.version, row.name]));
        assert.match(byVersion.get(1), /^001-initial\.js$/);
        assert.match(byVersion.get(2), /^002-plan-store\.js$/);
        assert.match(byVersion.get(3), /^003-media-and-ai-planning\.js$/);
        assert.match(byVersion.get(4), /^004-fsrs-review-state\.js$/);
        assert.match(byVersion.get(5), /^005-lexicon-library-mastery\.js$/);
        assert.match(byVersion.get(6), /^006-plan-revisions\.js$/);
        assert.match(byVersion.get(7), /^007-plan-revision-lesson-ops\.js$/);
        assert.match(byVersion.get(8), /^008-dictionary-glosses\.js$/);
        assert.match(byVersion.get(9), /^009-dictionary-entries\.js$/);
        assert.match(byVersion.get(10), /^010-lexicon-future-cells\.js$/);
        assert.match(byVersion.get(11), /^011-google-email\.js$/);
        assert.match(byVersion.get(12), /^012-tanakh-corpus\.js$/);
        for (const version of byVersion.keys()) {
            assert.ok(version >= 1 && version <= 12, 'unexpected migration ' + version);
        }

        db.migrate();
        const again = db.all('SELECT version FROM schema_migrations');
        assert.equal(again.length, versions.length);
    });

    it('CHECK rejects role=owner', () => {
        db.migrate();
        assert.throws(() => {
            db.run(
                `INSERT INTO users (
                    username, password_hash, role, status,
                    created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?)`,
                'alice',
                'hash',
                'owner',
                'active',
                '2026-01-01T00:00:00.000Z',
                '2026-01-01T00:00:00.000Z'
            );
        }, /CHECK|constraint/i);
    });

    it('deleting a user cascades to per-user rows', () => {
        db.migrate();
        const now = '2026-01-01T00:00:00.000Z';
        const user = db.run(
            `INSERT INTO users (
                username, password_hash, role, status, created_at, updated_at
            ) VALUES (?, ?, 'user', 'active', ?, ?)`,
            'bob',
            'hash',
            now,
            now
        );
        const userId = Number(user.lastInsertRowid);
        db.run(
            `INSERT INTO sessions (
                user_id, token_hash, created_at, last_seen_at, expires_at
            ) VALUES (?, ?, ?, ?, ?)`,
            userId, 'tok', now, now, now
        );
        db.run(
            `INSERT INTO review_items (
                user_id, word, direction, due, step, streak, lapses, sources
            ) VALUES (?, ?, 'he_en', ?, 0, 0, 0, '[]')`,
            userId, 'שלום', now
        );
        db.run(
            `INSERT INTO practice_events (
                id, user_id, word, direction, result, at
            ) VALUES (?, ?, ?, 'he_en', 'pass', ?)`,
            'evt-1', userId, 'שלום', now
        );
        db.run(
            `INSERT INTO lesson_progress (
                user_id, slug, state_json, rev, updated_at
            ) VALUES (?, ?, '{}', 0, ?)`,
            userId, 'my-morning', now
        );
        db.run(
            `INSERT INTO lesson_visibility (slug, mode, updated_at)
             VALUES ('my-morning', 'selected', ?)`,
            now
        );
        db.run(
            'INSERT INTO lesson_grants (slug, user_id) VALUES (?, ?)',
            'my-morning',
            userId
        );

        db.run('DELETE FROM users WHERE id = ?', userId);

        assert.equal(db.get('SELECT COUNT(*) AS n FROM sessions WHERE user_id = ?', userId).n, 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM review_items WHERE user_id = ?', userId).n, 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM practice_events WHERE user_id = ?', userId).n, 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_progress WHERE user_id = ?', userId).n, 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_grants WHERE user_id = ?', userId).n, 0);
    });

    it('auth_provider defaults to local; provider_subject unique per provider', () => {
        db.migrate();
        const now = '2026-01-01T00:00:00.000Z';
        db.run(
            `INSERT INTO users (
                username, password_hash, created_at, updated_at
            ) VALUES (?, ?, ?, ?)`,
            'local-user',
            'hash',
            now,
            now
        );
        const row = db.get('SELECT auth_provider, provider_subject FROM users WHERE username = ?', 'local-user');
        assert.equal(row.auth_provider, 'local');
        assert.equal(row.provider_subject, null);

        db.run(
            `INSERT INTO users (
                username, password_hash, auth_provider, provider_subject,
                created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?)`,
            'oidc-a',
            'hash',
            'oidc',
            'sub-1',
            now,
            now
        );
        assert.throws(() => {
            db.run(
                `INSERT INTO users (
                    username, password_hash, auth_provider, provider_subject,
                    created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?)`,
                'oidc-b',
                'hash',
                'oidc',
                'sub-1',
                now,
                now
            );
        }, /UNIQUE|constraint/i);

        // Multiple local users with null provider_subject are allowed.
        db.run(
            `INSERT INTO users (
                username, password_hash, created_at, updated_at
            ) VALUES (?, ?, ?, ?)`,
            'local-user-2',
            'hash',
            now,
            now
        );
        assert.equal(
            db.get(
                `SELECT COUNT(*) AS n FROM users
                 WHERE auth_provider = 'local' AND provider_subject IS NULL`
            ).n,
            2
        );
    });
});

describe('db:backup (ac-5)', () => {
    let tmp;
    let db;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-backup-'));
        db = freshDbModule(tmp);
        db.migrate();
        const now = '2026-01-01T00:00:00.000Z';
        db.run(
            `INSERT INTO users (
                username, password_hash, created_at, updated_at
            ) VALUES (?, ?, ?, ?)`,
            'backup-user',
            'hash',
            now,
            now
        );
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    it('writes a backup that opens and passes integrity_check', () => {
        const dest = db.backup();
        assert.ok(fs.existsSync(dest));
        assert.ok(dest.includes(path.join('backups', 'hebrew-')));
        assert.match(path.basename(dest), /^hebrew\.sqlite$/);
        assert.match(path.basename(path.dirname(dest)), /^hebrew-.+$/);

        db.close();
        const { DatabaseSync } = require('node:sqlite');
        const copy = new DatabaseSync(dest, { readOnly: true });
        const check = copy.prepare('PRAGMA integrity_check').get();
        assert.equal(check.integrity_check, 'ok');
        const user = copy.prepare(
            'SELECT username FROM users WHERE username = ?'
        ).get('backup-user');
        assert.equal(user.username, 'backup-user');
        copy.close();
    });
});
