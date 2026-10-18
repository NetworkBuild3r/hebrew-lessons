/**
 * Plan store migration and repository, on a temporary database.
 * Provenance: INIT-004/SPEC-001
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const modules = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/plans/store'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    const store = require('../server/plans/store');
    const migration = require('../server/db/migrations/002-plan-store');
    return { db, store, migration };
}

function tableNames(db) {
    return db.all(
        `SELECT name FROM sqlite_master
         WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
         ORDER BY name`
    ).map((row) => row.name);
}

describe('plan store', () => {
    let tmp;
    let lessons;
    let db;
    let store;
    let migration;
    const savedLessons = process.env.LESSONS_DIR;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-store-'));
        lessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-lessons-'));
        fs.writeFileSync(path.join(lessons, 'marker.txt'), 'untouched');
        process.env.LESSONS_DIR = lessons;
        ({ db, store, migration } = freshModules(tmp));
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        fs.rmSync(lessons, { recursive: true, force: true });
        if (savedLessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = savedLessons;
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    it('ac-1: inserts a draft at revision 1 with a null snapshot and reads the same JSON bytes', () => {
        const document = '{"id":"cafe-order","title":"קפה","note":"  spaced  "}';
        const saved = store.saveWorkingCopy({
            id: 'cafe-order',
            document,
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });

        assert.equal(saved.revision, 1);
        assert.equal(saved.status, 'draft');
        assert.equal(saved.audience, 'shared');
        assert.equal(saved.assigneeId, null);
        assert.equal(saved.snapshot, null);
        assert.equal(saved.document, document);

        const loaded = store.loadWorkingCopy('cafe-order');
        assert.equal(loaded.document, document);
        assert.equal(store.loadSnapshot('cafe-order'), null);

        const raw = db.get(
            'SELECT working_copy, snapshot, revision, status FROM plans WHERE id = ?',
            'cafe-order'
        );
        assert.equal(raw.working_copy, document);
        assert.equal(raw.snapshot, null);
        assert.equal(raw.revision, 1);
        assert.equal(raw.status, 'draft');
    });

    it('stores a quoted document through bound parameters', () => {
        const document = '{"id":"quote-plan","title":"it\'s a \\"plan\\""}';
        store.saveWorkingCopy({
            id: 'quote-plan',
            document,
            audience: 'shared',
            assigneeId: null
        });
        const loaded = store.loadWorkingCopy('quote-plan');
        assert.equal(loaded.document, document);
        assert.equal(loaded.revision, 1);
        assert.equal(loaded.status, 'draft');
        assert.equal(
            db.get('SELECT COUNT(*) AS n FROM plans WHERE id = ?', 'quote-plan').n,
            1
        );
    });

    it('rejects an id that is not a slug, including a SQL fragment', () => {
        assert.throws(() => {
            store.saveWorkingCopy({
                id: "x' OR 1=1 --",
                document: '{"id":"nope"}',
                audience: 'shared',
                assigneeId: null
            });
        }, /CHECK|constraint/i);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans').n >= 1, true);
    });

    it('rejects a display name, stores a short lowercase username, and stores an opaque id', () => {
        assert.throws(() => {
            store.saveWorkingCopy({
                id: 'personal-name',
                document: '{"id":"personal-name"}',
                audience: 'personal',
                assigneeId: 'Ada Lovelace'
            });
        }, /CHECK|constraint/i);
        const short = store.saveWorkingCopy({
            id: 'personal-short',
            document: '{"id":"personal-short"}',
            audience: 'personal',
            assigneeId: 'abc'
        });
        assert.equal(short.assigneeId, 'abc');
        const saved = store.saveWorkingCopy({
            id: 'personal-ok',
            document: '{"id":"personal-ok"}',
            audience: 'personal',
            assigneeId: 'learner-01'
        });
        assert.equal(saved.assigneeId, 'learner-01');
        assert.equal(saved.audience, 'personal');
    });

    it('rejects a shared plan with an assignee and a personal plan without one', () => {
        assert.throws(() => {
            store.saveWorkingCopy({
                id: 'shared-bad',
                document: '{"id":"shared-bad"}',
                audience: 'shared',
                assigneeId: 'learner-01'
            });
        }, /CHECK|constraint/i);
        assert.throws(() => {
            store.saveWorkingCopy({
                id: 'personal-bad',
                document: '{"id":"personal-bad"}',
                audience: 'personal',
                assigneeId: null
            });
        }, /CHECK|constraint/i);
    });

    it('saveSnapshot stores bytes and leaves the working copy in place', () => {
        const document = '{"id":"snap-plan","title":"קפה"}';
        const snapshot = '{"id":"snap-plan","title":"קפה","published":true}';
        store.saveWorkingCopy({
            id: 'snap-plan',
            document,
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        assert.equal(store.saveSnapshot('snap-plan', snapshot), snapshot);
        assert.equal(store.loadSnapshot('snap-plan'), snapshot);
        const loaded = store.loadWorkingCopy('snap-plan');
        assert.equal(loaded.document, document);
        assert.equal(loaded.snapshot, snapshot);
        assert.equal(loaded.revision, 1);
        assert.equal(loaded.status, 'draft');
    });

    it('a later working-copy save keeps the snapshot bytes', () => {
        const next = '{"id":"snap-plan","title":"תה"}';
        const saved = store.saveWorkingCopy({
            id: 'snap-plan',
            document: next,
            revision: 2,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        assert.equal(saved.document, next);
        assert.equal(saved.revision, 2);
        assert.equal(saved.snapshot, '{"id":"snap-plan","title":"קפה","published":true}');
    });

    it('ac-2: down drops only plans and keeps users and lesson_progress row counts', () => {
        const now = '2026-01-01T00:00:00.000Z';
        db.run(
            `INSERT INTO users (
                username, password_hash, created_at, updated_at
            ) VALUES (?, ?, ?, ?)`,
            'plan-store-user',
            'hash',
            now,
            now
        );
        const userId = Number(db.get(
            'SELECT id FROM users WHERE username = ?',
            'plan-store-user'
        ).id);
        db.run(
            `INSERT INTO lesson_progress (
                user_id, slug, state_json, rev, updated_at
            ) VALUES (?, ?, ?, 0, ?)`,
            userId,
            'cafe-order',
            '{}',
            now
        );
        db.run(
            `INSERT INTO review_items (
                user_id, word, direction, due, step, streak, lapses, sources
            ) VALUES (?, ?, 'he_en', ?, 0, 0, 0, '[]')`,
            userId,
            'שלום',
            now
        );
        db.run(
            `INSERT INTO audit_log (actor, action, at) VALUES (?, ?, ?)`,
            'test',
            'plan-store',
            now
        );

        const usersBefore = db.get('SELECT COUNT(*) AS n FROM users').n;
        const progressBefore = db.get('SELECT COUNT(*) AS n FROM lesson_progress').n;
        const reviewBefore = db.get('SELECT COUNT(*) AS n FROM review_items').n;
        const auditBefore = db.get('SELECT COUNT(*) AS n FROM audit_log').n;
        const before = tableNames(db);
        assert.ok(before.includes('plans'));
        assert.ok(usersBefore >= 1);
        assert.ok(progressBefore >= 1);

        migration.down(db.open());

        const usersAfter = db.get('SELECT COUNT(*) AS n FROM users').n;
        const progressAfter = db.get('SELECT COUNT(*) AS n FROM lesson_progress').n;
        assert.equal(usersAfter, usersBefore);
        assert.equal(progressAfter, progressBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM review_items').n, reviewBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM audit_log').n, auditBefore);

        const after = tableNames(db);
        const removed = before.filter((name) => !after.includes(name));
        assert.deepEqual(removed, ['plans']);
        assert.ok(after.includes('users'));
        assert.ok(after.includes('lesson_progress'));
        assert.ok(after.includes('review_items'));
        assert.ok(after.includes('audit_log'));
        assert.equal(
            db.get('SELECT username FROM users WHERE id = ?', userId).username,
            'plan-store-user'
        );
        assert.equal(
            db.get(
                'SELECT state_json FROM lesson_progress WHERE user_id = ? AND slug = ?',
                userId,
                'cafe-order'
            ).state_json,
            '{}'
        );

        const versions = db.all(
            'SELECT version FROM schema_migrations ORDER BY version'
        ).map((row) => row.version);
        assert.deepEqual(versions.filter((version) => version <= 3), [1, 3]);
        assert.equal(versions.includes(2), false);

        db.migrate();
        assert.ok(tableNames(db).includes('plans'));
        assert.equal(db.get('SELECT COUNT(*) AS n FROM users').n, usersBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_progress').n, progressBefore);
        const restored = db.all(
            'SELECT version, name FROM schema_migrations ORDER BY version'
        );
        const restoredByVersion = new Map(restored.map((row) => [row.version, row.name]));
        assert.match(restoredByVersion.get(2), /^002-plan-store\.js$/);
        assert.match(restoredByVersion.get(3), /^003-media-and-ai-planning\.js$/);
        assert.equal(restoredByVersion.has(1), true);
    });

    it('ac-3: the migration writes no audio paths and does not open LESSONS_DIR', () => {
        const migrationPath = path.join(
            root,
            'server',
            'db',
            'migrations',
            '002-plan-store.js'
        );
        const source = fs.readFileSync(migrationPath, 'utf8');
        assert.equal(/LESSONS_DIR/.test(source), false);
        assert.equal(/\.mp3/.test(source), false);
        assert.equal(/worksheets/.test(source), false);
        assert.equal(/\baudio\b/i.test(source), false);
        assert.equal(/require\(/.test(source), false);

        const marker = path.join(lessons, 'marker.txt');
        const before = fs.readFileSync(marker);
        const listingBefore = fs.readdirSync(lessons).sort();
        db.migrate();
        assert.deepEqual(fs.readdirSync(lessons).sort(), listingBefore);
        assert.deepEqual(fs.readFileSync(marker), before);
        assert.equal(fs.existsSync(path.join(lessons, 'marker.txt')), true);

        const copies = db.all('SELECT working_copy, snapshot FROM plans');
        for (const row of copies) {
            assert.equal(String(row.working_copy).includes('.mp3'), false);
            if (row.snapshot != null) {
                assert.equal(String(row.snapshot).includes('.mp3'), false);
            }
        }
    });
});
