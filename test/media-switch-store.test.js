/**
 * Media rows and the AI-planning switch, on a temporary database.
 * Provenance: INIT-004/SPEC-006
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const NOW = '2026-01-01T00:00:00.000Z';

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const modules = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/db/repos/users',
        '../server/db/repos/media'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    const users = require('../server/db/repos/users');
    const media = require('../server/db/repos/media');
    const migration = require('../server/db/migrations/003-media-and-ai-planning');
    return { db, users, media, migration };
}

function columnNames(db, table) {
    return db.all('PRAGMA table_info(' + table + ')').map((column) => column.name);
}

describe('media store and ai-planning switch', () => {
    let tmp;
    let db;
    let users;
    let media;
    let migration;
    let aliceInsertedBeforeColumn;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-switch-'));
        ({ db, users, media, migration } = freshModules(tmp));
        const database = db.open();
        database.exec(`
            CREATE TABLE IF NOT EXISTS schema_migrations (
                version INTEGER PRIMARY KEY,
                name TEXT NOT NULL,
                applied_at TEXT NOT NULL
            )
        `);
        const initial = require('../server/db/migrations/001-initial');
        const plans = require('../server/db/migrations/002-plan-store');
        initial.up(database);
        plans.up(database);
        db.run(
            `INSERT INTO schema_migrations (version, name, applied_at)
             VALUES (?, ?, ?)`,
            1,
            '001-initial.js',
            NOW
        );
        db.run(
            `INSERT INTO schema_migrations (version, name, applied_at)
             VALUES (?, ?, ?)`,
            2,
            '002-plan-store.js',
            NOW
        );

        aliceInsertedBeforeColumn = !columnNames(db, 'users').includes('ai_planning');
        db.run(
            `INSERT INTO users (
                username, password_hash, created_at, updated_at
            ) VALUES (?, ?, ?, ?)`,
            'alice',
            'hash',
            NOW,
            NOW
        );
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/repos/users')];
        delete require.cache[require.resolve('../server/db/repos/media')];
    });

    it('ac-1: existing users and inserts that omit the column stay off', () => {
        assert.equal(aliceInsertedBeforeColumn, true);

        const alice = db.get(
            'SELECT ai_planning FROM users WHERE username = ?',
            'alice'
        );
        assert.equal(alice.ai_planning, 0);
        assert.equal(users.getAiPlanning(Number(db.get(
            'SELECT id FROM users WHERE username = ?',
            'alice'
        ).id)), 0);

        db.run(
            `INSERT INTO users (
                username, password_hash, created_at, updated_at
            ) VALUES (?, ?, ?, ?)`,
            'bob',
            'hash',
            NOW,
            NOW
        );
        assert.equal(db.get(
            'SELECT ai_planning FROM users WHERE username = ?',
            'bob'
        ).ai_planning, 0);

        users.create({
            username: 'carol',
            password_hash: 'hash'
        });
        assert.equal(db.get(
            'SELECT ai_planning FROM users WHERE username = ?',
            'carol'
        ).ai_planning, 0);

        const planning = db.all('PRAGMA table_info(users)')
            .find((column) => column.name === 'ai_planning');
        assert.equal(planning.notnull, 1);
        assert.equal(String(planning.dflt_value).replace(/'/g, ''), '0');

        const usersSource = fs.readFileSync(
            path.join(root, 'server', 'db', 'repos', 'users.js'),
            'utf8'
        );
        const insertSql = usersSource.slice(
            usersSource.indexOf('INSERT INTO users'),
            usersSource.indexOf('function findById')
        );
        assert.equal(insertSql.includes('ai_planning'), false);

        const migrationSource = fs.readFileSync(
            path.join(root, 'server', 'db', 'migrations', '003-media-and-ai-planning.js'),
            'utf8'
        );
        assert.match(
            migrationSource,
            /ADD COLUMN ai_planning INTEGER NOT NULL DEFAULT 0/
        );
        assert.equal(/UPDATE\s+users/i.test(migrationSource), false);

        const flags = db.all('SELECT ai_planning FROM users').map((row) => row.ai_planning);
        assert.deepEqual(flags, [0, 0, 0]);
    });

    it('ac-2: media can be stored and the column list has no client filename', () => {
        const names = columnNames(db, 'media');
        assert.equal(names.includes('filename'), false);
        assert.equal(names.includes('original_name'), false);
        assert.equal(names.includes('stored_name'), true);
        assert.throws(
            () => db.get('SELECT filename FROM media'),
            /no such column/i
        );
        assert.throws(
            () => db.get('SELECT original_name FROM media'),
            /no such column/i
        );

        const saved = media.insert({
            id: 'coffee-cup',
            storedName: 'ab12cd34.png',
            altText: 'coffee',
            promptText: 'a cup of coffee',
            provider: 'openai',
            modelId: 'gpt-image-1',
            actor: 'mcp'
        });
        assert.equal(saved.id, 'coffee-cup');
        assert.equal(saved.storedName, 'ab12cd34.png');
        assert.equal(saved.status, 'pending');
        assert.equal(saved.altText, 'coffee');
        assert.equal(saved.promptText, 'a cup of coffee');
        assert.equal(saved.actor, 'mcp');
        assert.equal(fs.existsSync(path.join(tmp, 'media')), false);

        const updated = media.update('coffee-cup', {
            status: 'approved',
            altText: 'a cup of coffee'
        });
        assert.equal(updated.status, 'approved');
        assert.equal(updated.altText, 'a cup of coffee');
        assert.equal(updated.promptText, 'a cup of coffee');
        assert.equal(updated.actor, 'mcp');
        assert.equal(media.update('missing-id', { status: 'approved' }), null);

        assert.throws(
            () => db.run(
                `INSERT INTO media (
                    id, filename, stored_name, actor, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?)`,
                'water-glass',
                'upload.png',
                'ab12cd35.png',
                'mcp',
                NOW,
                NOW
            ),
            /no column named filename/i
        );
        assert.equal(db.get(
            'SELECT COUNT(*) AS n FROM media WHERE id = ?',
            'water-glass'
        ).n, 0);

        assert.throws(() => {
            media.insert({
                id: 'milk',
                storedName: 'ab12cd36.png',
                actor: 'admin:alice',
                filename: 'milk-photo.png'
            });
        }, /client filename is not stored/);
        assert.equal(db.get(
            'SELECT COUNT(*) AS n FROM media WHERE id = ?',
            'milk'
        ).n, 0);
    });

    it('set returns previous and next and does not copy the prompt into audit_log', () => {
        const aliceId = Number(db.get(
            'SELECT id FROM users WHERE username = ?',
            'alice'
        ).id);
        const auditBefore = db.get('SELECT COUNT(*) AS n FROM audit_log').n;
        const prompt = 'a glass of water';
        media.insert({
            id: 'water-switch',
            storedName: 'ab12cd39.png',
            promptText: prompt,
            actor: 'admin:bob'
        });

        const turnedOn = users.setAiPlanning(aliceId, 1);
        assert.deepEqual(turnedOn, { previous: 0, next: 1 });
        const turnedOff = users.setAiPlanning(aliceId, false);
        assert.deepEqual(turnedOff, { previous: 1, next: 0 });
        assert.equal(users.getAiPlanning(aliceId), 0);
        assert.equal(users.setAiPlanning(999999, 0), null);
        assert.equal(users.getAiPlanning(999999), null);
        assert.throws(() => users.setAiPlanning(aliceId, 2), /0 or 1/);
        assert.equal(users.getAiPlanning(aliceId), 0);

        assert.equal(db.get('SELECT COUNT(*) AS n FROM audit_log').n, auditBefore);
        const auditText = db.all(
            'SELECT actor, action, target, detail FROM audit_log'
        ).map((row) => JSON.stringify(row)).join('\n');
        assert.equal(auditText.includes(prompt), false);

        const mediaSource = fs.readFileSync(
            path.join(root, 'server', 'db', 'repos', 'media.js'),
            'utf8'
        );
        assert.equal(/INSERT\s+INTO\s+audit_log/i.test(mediaSource), false);
        assert.equal(/require\(['"]fs['"]\)/.test(mediaSource), false);
        assert.equal(/require\(['"]node:fs['"]\)/.test(mediaSource), false);
    });

    it('stores prompt text through bound parameters', () => {
        const prompt = 'it\'s a "cup"; DROP TABLE users; --';
        const saved = media.insert({
            id: 'quote-cup',
            storedName: 'ab12cd37.png',
            promptText: prompt,
            actor: 'admin:alice'
        });
        assert.equal(saved.promptText, prompt);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM users').n >= 3, true);
        assert.throws(() => {
            media.insert({
                id: 'bad name',
                storedName: 'ab12cd38.png',
                actor: 'mcp'
            });
        }, /CHECK|constraint/i);
        assert.throws(() => {
            media.insert({
                id: 'photo-name',
                storedName: 'My Photo.png',
                actor: 'mcp'
            });
        }, /CHECK|constraint/i);
    });

    it('ac-3: down drops media and ai_planning and keeps plans and lesson progress', () => {
        const aliceId = Number(db.get(
            'SELECT id FROM users WHERE username = ?',
            'alice'
        ).id);
        users.setAiPlanning(aliceId, true);
        assert.equal(users.getAiPlanning(aliceId), 1);

        db.run(
            `INSERT INTO lesson_progress (
                user_id, slug, state_json, rev, updated_at
            ) VALUES (?, ?, ?, 0, ?)`,
            aliceId,
            'cafe-order',
            '{}',
            NOW
        );
        db.run(
            `INSERT INTO plans (
                id, audience, working_copy, created_at, updated_at
            ) VALUES (?, 'shared', ?, ?, ?)`,
            'cafe-order',
            '{}',
            NOW,
            NOW
        );

        const usersBefore = db.get('SELECT COUNT(*) AS n FROM users').n;
        const progressBefore = db.get('SELECT COUNT(*) AS n FROM lesson_progress').n;
        const plansBefore = db.get('SELECT COUNT(*) AS n FROM plans').n;
        assert.ok(progressBefore >= 1);
        assert.ok(plansBefore >= 1);
        assert.ok(columnNames(db, 'users').includes('ai_planning'));
        assert.ok(columnNames(db, 'media').includes('stored_name'));

        migration.down(db.open());

        assert.equal(db.get('SELECT COUNT(*) AS n FROM users').n, usersBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_progress').n, progressBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans').n, plansBefore);
        assert.equal(columnNames(db, 'users').includes('ai_planning'), false);
        assert.throws(
            () => db.get('SELECT ai_planning FROM users WHERE username = ?', 'alice'),
            /no such column/i
        );
        const tables = db.all(
            `SELECT name FROM sqlite_master
             WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
        ).map((row) => row.name);
        assert.equal(tables.includes('media'), false);
        assert.equal(tables.includes('plans'), true);
        assert.equal(tables.includes('lesson_progress'), true);
        assert.equal(
            db.get(
                'SELECT state_json FROM lesson_progress WHERE user_id = ? AND slug = ?',
                aliceId,
                'cafe-order'
            ).state_json,
            '{}'
        );
        assert.equal(
            db.get('SELECT id FROM plans WHERE id = ?', 'cafe-order').id,
            'cafe-order'
        );
        const versions = db.all(
            'SELECT version FROM schema_migrations ORDER BY version'
        ).map((row) => row.version);
        assert.deepEqual(versions.filter((version) => version <= 3), [1, 2]);
        assert.equal(versions.includes(3), false);

        db.migrate();
        assert.equal(users.getAiPlanning(aliceId), 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_progress').n, progressBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans').n, plansBefore);
        assert.equal(tables.includes('users'), true);
        assert.ok(columnNames(db, 'media').includes('id'));
        assert.equal(columnNames(db, 'media').includes('filename'), false);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM media').n, 0);
        const restored = db.all(
            'SELECT version FROM schema_migrations ORDER BY version'
        ).map((row) => row.version);
        assert.deepEqual(restored.filter((version) => version <= 3), [1, 2, 3]);
        assert.equal(fs.existsSync(path.join(tmp, 'media')), false);
    });
});
