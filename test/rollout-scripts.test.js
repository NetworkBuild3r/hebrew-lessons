/**
 * Language-program rollout scripts (backup-db, stage-counts) on temporary
 * fixtures only. Never opens or mutates the live learner database for
 * destructive ops. Ambient suite stays on ladder; fsrs checks set and restore
 * SRS_SCHEDULER inside this file.
 *
 * Provenance: INIT-006/SPEC-020
 */
'use strict';

const { describe, it, before, after, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const LIVE_DB = path.join(root, 'data', 'hebrew.sqlite');
const BACKUP_SCRIPT = path.join(root, 'scripts', 'backup-db.js');
const STAGE_SCRIPT = path.join(root, 'scripts', 'stage-counts.js');
const NOW = '2026-01-01T00:00:00.000Z';
const AT = '2026-03-01T12:00:00.000Z';

const MODULES = [
    '../server/paths',
    '../server/db',
    '../server/db/repos/users',
    '../server/db/repos/review',
    '../server/db/repos/events',
    '../server/progress',
    '../server/db/migrations/001-initial',
    '../server/db/migrations/002-plan-store',
    '../server/db/migrations/003-media-and-ai-planning',
    '../server/db/migrations/004-fsrs-review-state'
];

function unload() {
    for (const name of MODULES) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

function assertNotLive(dataDir) {
    const resolved = path.resolve(dataDir);
    assert.ok(resolved.startsWith(path.resolve(os.tmpdir()))
        || resolved.includes(path.sep + 'heb-rollout-')
        || resolved.includes(path.sep + 'hebrew-rollout-'),
    'fixture DATA_DIR must be temporary: ' + resolved);
    assert.notEqual(path.resolve(path.join(dataDir, 'hebrew.sqlite')), path.resolve(LIVE_DB));
}

/**
 * @param {string} script
 * @param {string} dataDir
 * @param {NodeJS.ProcessEnv} [extraEnv]
 */
function runScript(script, dataDir, extraEnv) {
    assertNotLive(dataDir);
    const env = Object.assign({}, process.env, extraEnv || {}, {
        HEBREW_ROOT: root,
        DATA_DIR: dataDir
    });
    // Pin ladder unless the caller sets SRS_SCHEDULER for an isolated check.
    if (!Object.prototype.hasOwnProperty.call(extraEnv || {}, 'SRS_SCHEDULER')
        && env.SRS_SCHEDULER === undefined) {
        env.SRS_SCHEDULER = 'ladder';
    }
    const result = spawnSync(process.execPath, [script], {
        cwd: root,
        env,
        encoding: 'utf8',
        windowsHide: true
    });
    return result;
}

function seedPreFsrs(database) {
    database.exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            applied_at TEXT NOT NULL
        )
    `);
    require('../server/db/migrations/001-initial').up(database);
    require('../server/db/migrations/002-plan-store').up(database);
    require('../server/db/migrations/003-media-and-ai-planning').up(database);
    const record = database.prepare(
        'INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)'
    );
    record.run(1, '001-initial.js', NOW);
    record.run(2, '002-plan-store.js', NOW);
    record.run(3, '003-media-and-ai-planning.js', NOW);

    const insertUser = database.prepare(
        `INSERT INTO users (username, password_hash, created_at, updated_at)
         VALUES (?, ?, ?, ?)`
    );
    const user1 = Number(insertUser.run('learner-a', 'hash-not-printed', NOW, NOW).lastInsertRowid);
    const user2 = Number(insertUser.run('learner-b', 'hash-not-printed', NOW, NOW).lastInsertRowid);

    const insertReview = database.prepare(
        `INSERT INTO review_items (
            user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
    );
    const rows = [
        [user1, 'חדש', 'he_en', NOW, 0, 0, 0, null, null],
        [user1, 'למד', 'he_en', NOW, 1, 1, 0, 'pass', AT],
        [user1, 'יודע', 'he_en', NOW, 3, 1, 0, 'pass', AT],
        [user1, 'write', 'en_he', NOW, 2, 1, 0, 'pass', AT],
        [user2, 'שלום', 'he_en', NOW, 0, 0, 0, 'miss', AT],
        [user2, 'hello', 'en_he', NOW, 4, 2, 0, 'pass', AT]
    ];
    for (const row of rows) {
        insertReview.run(row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8], '[]');
    }
    database.prepare(
        `INSERT INTO practice_events (
            id, user_id, word, direction, result, at, slug, activity
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
    ).run('evt-1', user1, 'למד', 'he_en', 'pass', AT, null, null);

    return { user1, user2 };
}

describe('rollout scripts (INIT-006/SPEC-020)', () => {
    let savedScheduler;
    let tmp;
    let db;

    before(() => {
        savedScheduler = process.env.SRS_SCHEDULER;
        process.env.SRS_SCHEDULER = 'ladder';
    });

    after(() => {
        if (savedScheduler === undefined) delete process.env.SRS_SCHEDULER;
        else process.env.SRS_SCHEDULER = savedScheduler;
    });

    beforeEach(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-rollout-'));
        assertNotLive(tmp);
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        process.env.SRS_SCHEDULER = 'ladder';
        unload();
        db = require('../server/db');
        db.close();
        const database = db.open({ force: true });
        assert.notEqual(path.resolve(db.databasePath()), path.resolve(LIVE_DB));
        seedPreFsrs(database);
    });

    afterEach(() => {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
            db = null;
        }
        unload();
        if (tmp) {
            try {
                fs.rmSync(tmp, { recursive: true, force: true });
            } catch (err) {
                // Windows can briefly hold the SQLite -wal/-shm after close.
                if (err && err.code !== 'EPERM' && err.code !== 'EBUSY') throw err;
            }
            tmp = null;
        }
        delete process.env.DATA_DIR;
        process.env.SRS_SCHEDULER = 'ladder';
    });

    it('ac-4: backup-db writes a dated flat file and prints only path and size', () => {
        db.close();
        const result = runScript(BACKUP_SCRIPT, tmp, { SRS_SCHEDULER: 'ladder' });
        assert.equal(result.status, 0, result.stderr || result.stdout);
        const lines = String(result.stdout).trim().split(/\r?\n/);
        assert.equal(lines.length, 2, 'expected path line and size line: ' + result.stdout);
        const dest = lines[0].trim();
        const sizeLine = lines[1].trim();
        assert.match(path.basename(dest), /^hebrew-\d{4}-\d{2}-\d{2}T.+\.sqlite$/);
        assert.ok(dest.includes(path.join('backups', path.basename(dest)))
            || dest.replace(/\//g, path.sep).includes(path.join('backups', path.basename(dest))));
        assert.ok(fs.existsSync(dest));
        assert.match(sizeLine, /^\d+ bytes$/);
        const size = Number(sizeLine.split(' ')[0]);
        assert.equal(size, fs.statSync(dest).size);
        assert.ok(size > 0);

        const joined = result.stdout + '\n' + result.stderr;
        assert.doesNotMatch(joined, /hash-not-printed/);
        assert.doesNotMatch(joined, /learner-a/);
        assert.doesNotMatch(joined, /INSERT|SELECT|password/i);
        assert.doesNotMatch(joined, /חדש|למד|יודע/);
    });

    function applyFsrsMigration() {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        unload();
        process.env.DATA_DIR = tmp;
        process.env.SRS_SCHEDULER = 'ladder';
        db = require('../server/db');
        db.open({ force: true });
        require('../server/db/migrations/004-fsrs-review-state').up(db.open());
        db.run(
            'INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)',
            4,
            '004-fsrs-review-state.js',
            NOW
        );
    }

    it('ac-1: stage counts under ladder match before and after FSRS migration', () => {
        db.close();
        const before = runScript(STAGE_SCRIPT, tmp, { SRS_SCHEDULER: 'ladder' });
        assert.equal(before.status, 0, before.stderr || before.stdout);
        assert.match(before.stdout, /scheduler=ladder/);
        assert.match(before.stdout, /learner-a/);
        assert.match(before.stdout, /learner-b/);
        assert.doesNotMatch(before.stdout, /hash-not-printed/);
        assert.doesNotMatch(before.stdout, /password/i);

        applyFsrsMigration();
        db.close();

        const after = runScript(STAGE_SCRIPT, tmp, { SRS_SCHEDULER: 'ladder' });
        assert.equal(after.status, 0, after.stderr || after.stdout);

        function userLines(text) {
            return text.trim().split(/\r?\n/)
                .filter((line) => line.startsWith('learner-'))
                .sort();
        }
        assert.deepEqual(userLines(after.stdout), userLines(before.stdout));
    });

    it('stage-counts under fsrs still matches ladder totals after backfill', () => {
        applyFsrsMigration();
        db.close();

        const ladderOut = runScript(STAGE_SCRIPT, tmp, { SRS_SCHEDULER: 'ladder' });
        const fsrsOut = runScript(STAGE_SCRIPT, tmp, { SRS_SCHEDULER: 'fsrs' });
        assert.equal(ladderOut.status, 0, ladderOut.stderr);
        assert.equal(fsrsOut.status, 0, fsrsOut.stderr);
        assert.match(fsrsOut.stdout, /scheduler=fsrs/);

        function stripScheduler(text) {
            return text.trim().split(/\r?\n/)
                .filter((line) => line.startsWith('learner-'))
                .sort()
                .join('\n');
        }
        assert.equal(stripScheduler(fsrsOut.stdout), stripScheduler(ladderOut.stdout));
    });

    it('ac-3: SRS_SCHEDULER=ladder grades advance step (rollback path)', () => {
        applyFsrsMigration();

        delete require.cache[require.resolve('../server/progress')];
        const progress = require('../server/progress');
        assert.equal(progress.SCHEDULER, 'ladder');

        const review = require('../server/db/repos/review');
        const userId = db.get('SELECT id FROM users WHERE username = ?', 'learner-a').id;
        const word = 'גדר';
        assert.ok(!review.find(userId, word, 'he_en'));

        progress.applySignals(userId, [{
            id: 'rollout-grade-1',
            word,
            direction: 'he_en',
            result: 'pass',
            slug: 'fixture',
            activity: 'words'
        }], AT);

        const afterPass = review.find(userId, word, 'he_en');
        assert.ok(afterPass);
        assert.equal(afterPass.step, 1);

        progress.applySignals(userId, [{
            id: 'rollout-grade-2',
            word,
            direction: 'he_en',
            result: 'pass',
            slug: 'fixture',
            activity: 'words'
        }], '2026-03-03T12:00:00.000Z');

        const afterSecond = review.find(userId, word, 'he_en');
        assert.equal(afterSecond.step, 2);
        assert.notEqual(afterSecond.step, afterPass.step);
    });

    it('ac-4: .env.example documents SRS_SCHEDULER with no secret values', () => {
        const text = fs.readFileSync(path.join(root, '.env.example'), 'utf8');
        assert.match(text, /SRS_SCHEDULER=/);
        assert.match(text, /Not a secret|not a secret|ladder|fsrs/i);
        // Secret keys may appear as empty assignments; none may carry a value.
        const secretLines = text.split(/\r?\n/).filter((line) =>
            /^(11_labs_api_key|ELEVENLABS_API_KEY|OPENAI_API_KEY|ADMIN_PASSWORD|MCP_AUTH_TOKEN)\s*=/.test(line)
        );
        for (const line of secretLines) {
            assert.match(line, /=\s*$/, 'secret must have empty value: ' + line);
        }
        assert.doesNotMatch(text, /SRS_SCHEDULER=\s*fsrs/);
        // The example may set SRS_SCHEDULER= empty only.
        assert.match(text, /^SRS_SCHEDULER=\s*$/m);
    });

    it('never targets the live learner database path from these scripts under the fixture', () => {
        db.close();
        const result = runScript(BACKUP_SCRIPT, tmp, { SRS_SCHEDULER: 'ladder' });
        assert.equal(result.status, 0, result.stderr);
        const dest = String(result.stdout).trim().split(/\r?\n/)[0];
        assert.ok(path.resolve(dest).startsWith(path.resolve(tmp)));
        assert.ok(!path.resolve(dest).startsWith(path.resolve(path.dirname(LIVE_DB)))
            || path.resolve(dest).startsWith(path.resolve(tmp)));
        if (fs.existsSync(LIVE_DB)) {
            assert.notEqual(path.resolve(dest), path.resolve(LIVE_DB));
        }
    });
});
