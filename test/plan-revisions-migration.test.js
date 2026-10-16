/**
 * Migration 006: plan_revisions and plan_field_marks (INIT-008/SPEC-003).
 * Temporary database only. Does not open data/hebrew.sqlite or any DATA_DIR
 * other than the one this test creates.
 * Provenance: INIT-008/SPEC-003
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const zlib = require('zlib');

const root = path.resolve(__dirname, '..');
const LIVE_DB = path.join(root, 'data', 'hebrew.sqlite');
const MIGRATIONS = path.join(root, 'server', 'db', 'migrations');
const NOW = '2026-10-12T12:00:00.000Z';

const REVISION_COLUMNS = [
    ['plan_id', 'TEXT', 1, 1], ['revision', 'INTEGER', 1, 2], ['actor_kind', 'TEXT', 1, 0],
    ['actor_name', 'TEXT', 1, 0], ['tool', 'TEXT', 0, 0], ['client', 'TEXT', 0, 0],
    ['request_id', 'TEXT', 0, 0], ['op', 'TEXT', 1, 0], ['path', 'TEXT', 0, 0],
    ['field', 'TEXT', 0, 0], ['summary', 'TEXT', 1, 0], ['document_gz', 'BLOB', 1, 0],
    ['document_sha256', 'TEXT', 1, 0], ['created_at', 'TEXT', 1, 0]
];
const MARK_COLUMNS = [
    ['plan_id', 'TEXT', 1, 1], ['path', 'TEXT', 1, 2], ['field', 'TEXT', 1, 3],
    ['last_actor_kind', 'TEXT', 1, 0], ['last_actor_name', 'TEXT', 1, 0],
    ['last_revision', 'INTEGER', 1, 0], ['last_at', 'TEXT', 1, 0], ['value_hash', 'TEXT', 1, 0],
    ['human_revision', 'INTEGER', 0, 0], ['human_at', 'TEXT', 0, 0],
    ['human_actor_name', 'TEXT', 0, 0], ['previous_value_json', 'TEXT', 0, 0],
    ['previous_writer_kind', 'TEXT', 0, 0]
];

function stamp(file) {
    const out = {};
    for (const name of [file, file + '-wal', file + '-shm']) {
        if (!fs.existsSync(name)) {
            out[name] = null;
            continue;
        }
        const stat = fs.statSync(name);
        out[name] = { size: stat.size, mtimeMs: stat.mtimeMs };
    }
    return out;
}

describe('migration 006 plan revisions (INIT-008/SPEC-003)', { concurrency: false }, () => {
    let tmp;
    let db;
    let database;
    let migration;
    let liveBefore;
    const saved = {};

    function tables() {
        return database.prepare(
            "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
        ).all().map((row) => row.name);
    }

    function columns(table) {
        return database.prepare('PRAGMA table_info(' + table + ')').all()
            .map((row) => [row.name, row.type, row.notnull, row.pk]);
    }

    /** Per-table schema (columns and the CREATE sql) and a hash of every row. */
    function snapshot(names) {
        const out = {};
        for (const name of names) {
            const sql = database.prepare("SELECT sql FROM sqlite_master WHERE name = ?").get(name).sql;
            const rows = database.prepare('SELECT * FROM ' + name + ' ORDER BY rowid').all();
            const hash = crypto.createHash('sha256')
                .update(JSON.stringify(rows, (k, v) => (typeof v === 'bigint' ? String(v) : v)))
                .digest('hex');
            out[name] = { columns: columns(name), sql: sql, count: rows.length, hash: hash };
        }
        return out;
    }

    function insertPlan(id) {
        database.prepare(
            `INSERT INTO plans (id, revision, status, audience, assignee_id, working_copy, snapshot, created_at, updated_at)
             VALUES (?, 1, 'draft', 'shared', NULL, '{"id":"x"}', NULL, ?, ?)`
        ).run(id, NOW, NOW);
    }

    function revisionRow(overrides) {
        const document = JSON.stringify({ id: 'plan-a' });
        const gz = zlib.gzipSync(Buffer.from(document, 'utf8'));
        return Object.assign({
            plan_id: 'plan-a',
            revision: 1,
            actor_kind: 'ai',
            actor_name: 'mcp',
            tool: 'set_field',
            client: null,
            request_id: null,
            op: 'set_field',
            path: 'units/u',
            field: 'title',
            summary: 'set title',
            document_gz: gz,
            document_sha256: crypto.createHash('sha256').update(document).digest('hex'),
            created_at: NOW
        }, overrides || {});
    }

    function insertRevision(row) {
        const keys = Object.keys(row);
        return database.prepare(
            'INSERT INTO plan_revisions (' + keys.join(', ') + ') VALUES (' + keys.map(() => '?').join(', ') + ')'
        ).run(...keys.map((key) => row[key]));
    }

    function markRow(overrides) {
        return Object.assign({
            plan_id: 'plan-a',
            path: 'units/u',
            field: 'title',
            last_actor_kind: 'human',
            last_actor_name: 'example-admin',
            last_revision: 2,
            last_at: NOW,
            value_hash: 'abc',
            human_revision: 2,
            human_at: NOW,
            human_actor_name: 'example-admin',
            previous_value_json: '"before"',
            previous_writer_kind: 'ai'
        }, overrides || {});
    }

    function insertMark(row) {
        const keys = Object.keys(row);
        return database.prepare(
            'INSERT INTO plan_field_marks (' + keys.join(', ') + ') VALUES (' + keys.map(() => '?').join(', ') + ')'
        ).run(...keys.map((key) => row[key]));
    }

    before(() => {
        liveBefore = stamp(LIVE_DB);
        ['DATA_DIR', 'HEBREW_ROOT'].forEach((name) => { saved[name] = process.env[name]; });
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-revisions-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        for (const name of ['../server/paths', '../server/db']) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* not loaded */ }
        }
        db = require('../server/db');
        db.close();
        database = db.open({ force: true });
        migration = require('../server/db/migrations/006-plan-revisions');

        // Bring a temporary database to version 5 by hand, then let the runner apply 006.
        database.exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
            version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`);
        const files = fs.readdirSync(MIGRATIONS)
            .filter((name) => /^\d{3,}-.+\.js$/.test(name) && Number(name.split('-')[0]) <= 5)
            .sort();
        assert.equal(files.length, 5);
        for (const file of files) {
            database.exec('BEGIN IMMEDIATE');
            require(path.join(MIGRATIONS, file)).up(database);
            database.prepare('INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)')
                .run(Number(file.split('-')[0]), file, NOW);
            database.exec('COMMIT');
        }
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        for (const [name, value] of Object.entries(saved)) {
            if (value === undefined) delete process.env[name];
            else process.env[name] = value;
        }
        for (const name of ['../server/paths', '../server/db']) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* ignore */ }
        }
    });

    it('is numbered 006 and sits after 005', () => {
        const files = fs.readdirSync(MIGRATIONS).filter((name) => /^\d{3,}-.+\.js$/.test(name)).sort();
        assert.deepEqual(files.map((name) => name.split('-')[0]), [
            '001', '002', '003', '004', '005', '006', '007', '008', '009', '010'
        ]);
        assert.equal(migration.VERSION, 6);
        assert.equal(files[5], '006-plan-revisions.js');
    });

    it('ac-1 and ac-2: up creates exactly the two tables and leaves every other table and row identical', () => {
        // Rows in the tables 006 must not touch, including plans and audit_log.
        insertPlan('plan-a');
        insertPlan('plan-b');
        database.prepare('INSERT INTO audit_log (actor, action, target, at, detail) VALUES (?, ?, ?, ?, ?)')
            .run('mcp', 'plan.set_field', 'plan-a', NOW, '{"revision":2}');
        // schema_migrations gains the version-6 row by design; the runner tests below cover it.
        const existing = tables().filter((name) => name !== 'schema_migrations');
        assert.equal(existing.includes('plan_revisions'), false);
        const before = snapshot(existing);
        assert.equal(before.plans.count, 2);
        assert.equal(before.audit_log.count, 1);

        database.exec('BEGIN IMMEDIATE');
        migration.up(database);
        database.prepare('INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)')
            .run(6, '006-plan-revisions.js', NOW);
        database.exec('COMMIT');

        const after = snapshot(existing);
        assert.deepEqual(after, before);
        const now = tables().filter((name) => name !== 'schema_migrations');
        assert.deepEqual(now.filter((name) => existing.indexOf(name) === -1).sort(),
            ['plan_field_marks', 'plan_revisions']);

        assert.deepEqual(columns('plan_revisions'), REVISION_COLUMNS);
        assert.deepEqual(columns('plan_field_marks'), MARK_COLUMNS);
        for (const table of ['plan_revisions', 'plan_field_marks']) {
            for (const [name] of columns(table)) {
                assert.equal(/pin|lock/i.test(name), false, table + '.' + name);
            }
            assert.equal(/\b(pinned|locked|pin|lock)\b/i.test(
                database.prepare('SELECT sql FROM sqlite_master WHERE name = ?').get(table).sql
            ), false, table);
        }
        const indexes = database.prepare(
            "SELECT name, tbl_name FROM sqlite_master WHERE type = 'index' AND name LIKE 'idx_plan_%' ORDER BY name"
        ).all();
        assert.deepEqual(indexes.map((row) => [row.name, row.tbl_name]), [
            ['idx_plan_field_marks_last_actor', 'plan_field_marks'],
            ['idx_plan_revisions_plan_created', 'plan_revisions']
        ]);
        const indexColumns = (name) => database.prepare('PRAGMA index_info(' + name + ')').all().map((row) => row.name);
        assert.deepEqual(indexColumns('idx_plan_revisions_plan_created'), ['plan_id', 'created_at']);
        assert.deepEqual(indexColumns('idx_plan_field_marks_last_actor'), ['plan_id', 'last_actor_kind']);
    });

    it('ac-3: the runner records one schema_migrations row, and a second migrate adds none', () => {
        db.migrate();
        db.migrate();
        const rowsFor6 = database.prepare('SELECT * FROM schema_migrations WHERE version = 6').all();
        assert.equal(rowsFor6.length, 1);
        assert.equal(rowsFor6[0].name, '006-plan-revisions.js');
        const maxMigration = Math.max(...fs.readdirSync(MIGRATIONS)
            .filter((name) => /^\d{3,}-.+\.js$/.test(name))
            .map((name) => Number(name.split('-')[0])));
        assert.equal(database.prepare('SELECT MAX(version) AS v FROM schema_migrations').get().v, maxMigration);
    });

    it('ac-4: CHECK constraints, the primary key, and the foreign keys reject bad rows', () => {
        assert.equal(insertRevision(revisionRow()).changes, 1);
        assert.throws(() => insertRevision(revisionRow({ actor_kind: 'robot', revision: 2 })), /CHECK/);
        assert.throws(() => insertRevision(revisionRow({ op: 'pin', revision: 2 })), /CHECK/);
        assert.throws(() => insertRevision(revisionRow({ revision: 0 })), /CHECK/);
        assert.throws(() => insertRevision(revisionRow({ summary: 'x'.repeat(501), revision: 2 })), /CHECK/);
        assert.throws(() => insertRevision(revisionRow({ document_sha256: 'short', revision: 2 })), /CHECK/);
        assert.throws(() => insertRevision(revisionRow({ document_gz: null, revision: 2 })), /NOT NULL/);
        assert.throws(() => insertRevision(revisionRow()), /UNIQUE|PRIMARY/);
        assert.throws(() => insertRevision(revisionRow({ plan_id: 'no-such-plan', revision: 1 })), /FOREIGN KEY/);
        const ops = ['create', 'baseline', 'set_field', 'replace_part', 'insert_block',
            'move_block', 'remove_block', 'insert_lesson', 'move_lesson', 'remove_lesson',
            'assign', 'publish', 'restore'];
        // INIT-010/SPEC-002: lesson ops are in the 006 CHECK (fresh migrate). No new migration.
        for (const op of ops) {
            assert.equal(insertRevision(revisionRow({ op: op, revision: 10 + ops.indexOf(op) })).changes, 1, op);
        }
        for (const kind of ['human', 'system']) {
            assert.equal(insertRevision(revisionRow({ actor_kind: kind, revision: 30 + (kind === 'human' ? 0 : 1) })).changes, 1);
        }

        assert.equal(insertMark(markRow()).changes, 1);
        assert.throws(() => insertMark(markRow({ last_actor_kind: 'robot', field: 'a' })), /CHECK/);
        assert.throws(() => insertMark(markRow({ previous_writer_kind: 'bot', field: 'b' })), /CHECK/);
        assert.throws(() => insertMark(markRow({ last_revision: 0, field: 'c' })), /CHECK/);
        assert.throws(() => insertMark(markRow({ human_revision: 0, field: 'd' })), /CHECK/);
        assert.throws(() => insertMark(markRow({ previous_value_json: 'x'.repeat(32769), field: 'e' })), /CHECK/);
        assert.throws(() => insertMark(markRow()), /UNIQUE|PRIMARY/);
        assert.throws(() => insertMark(markRow({ plan_id: 'no-such-plan', field: 'f' })), /FOREIGN KEY/);
        assert.equal(insertMark(markRow({ field: 'g', previous_value_json: 'x'.repeat(32768), previous_writer_kind: 'unknown' })).changes, 1);
        assert.equal(insertMark(markRow({
            field: 'h', human_revision: null, human_at: null, human_actor_name: null,
            previous_value_json: null, previous_writer_kind: null, last_actor_kind: 'ai'
        })).changes, 1);
    });

    it('ac-4: deleting a plan removes its history and marks (cascade, as 005 does for learner rows)', () => {
        insertPlan('plan-c');
        insertRevision(revisionRow({ plan_id: 'plan-c', revision: 1 }));
        insertMark(markRow({ plan_id: 'plan-c' }));
        database.prepare('DELETE FROM plans WHERE id = ?').run('plan-c');
        assert.equal(database.prepare('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?').get('plan-c').n, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS n FROM plan_field_marks WHERE plan_id = ?').get('plan-c').n, 0);
    });

    it('ac-3: down removes both tables and indexes and leaves the pre-006 schema and rows identical', () => {
        database.exec('DELETE FROM plan_revisions; DELETE FROM plan_field_marks;');
        const names = tables().filter((name) => ['plan_revisions', 'plan_field_marks', 'schema_migrations'].indexOf(name) === -1);
        const before = snapshot(names);

        migration.down(database);

        assert.deepEqual(tables().filter((name) => name !== 'schema_migrations'), names.slice().sort());
        assert.equal(database.prepare(
            "SELECT COUNT(*) AS n FROM sqlite_master WHERE name LIKE 'idx_plan_revisions%' OR name LIKE 'idx_plan_field_marks%'"
        ).get().n, 0);
        assert.deepEqual(snapshot(names), before);
        assert.equal(database.prepare('SELECT COUNT(*) AS n FROM schema_migrations WHERE version = 6').get().n, 0);
    });

    it('ac-3: up after down is clean, through the runner, and is recorded once', () => {
        db.migrate();
        assert.deepEqual(columns('plan_revisions'), REVISION_COLUMNS);
        assert.deepEqual(columns('plan_field_marks'), MARK_COLUMNS);
        assert.equal(database.prepare('SELECT COUNT(*) AS n FROM schema_migrations WHERE version = 6').get().n, 1);
        assert.equal(insertRevision(revisionRow({ revision: 99 })).changes, 1);
    });

    it('a failing up() rolls back and records nothing (runner transaction)', () => {
        migration.down(database);
        const fn = migration.up;
        migration.up = function (target) {
            fn(target);
            throw new Error('forced failure after create');
        };
        try {
            assert.throws(() => db.migrate(), /forced failure/);
        } finally {
            migration.up = fn;
        }
        assert.equal(tables().includes('plan_revisions'), false);
        assert.equal(database.prepare('SELECT COUNT(*) AS n FROM schema_migrations WHERE version = 6').get().n, 0);
        db.migrate();
        assert.equal(tables().includes('plan_revisions'), true);
    });

    it('the migration holds no pin or lock column and no SQL that touches plans or audit_log', () => {
        const src = fs.readFileSync(path.join(MIGRATIONS, '006-plan-revisions.js'), 'utf8');
        assert.equal(/ALTER TABLE|DROP TABLE IF EXISTS (plans|audit_log)|INSERT INTO|UPDATE |DELETE FROM plans/.test(src), false);
        assert.equal(/\bpinned\b|\blocked\b|_pin\b|_lock\b/i.test(src.replace(/^\s*\*.*$/gm, '')), false);
    });

    it('the live database file was not touched', () => {
        assert.deepEqual(stamp(LIVE_DB), liveBefore);
    });
});
