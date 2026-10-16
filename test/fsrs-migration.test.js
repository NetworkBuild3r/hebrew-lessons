/**
 * FSRS columns on review_items and practice_events.
 * Temporary database only. Does not open data/hebrew.sqlite.
 * Provenance: INIT-006/SPEC-002
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const LIVE_DB = path.join(root, 'data', 'hebrew.sqlite');
const NOW = '2026-01-01T00:00:00.000Z';
const AT = '2026-03-01T12:00:00.000Z';

const LADDER_FIELDS = [
    'id', 'user_id', 'word', 'direction', 'due', 'step', 'streak', 'lapses',
    'last_result', 'last_at', 'sources'
];
const EVENT_FIELDS = [
    'id', 'user_id', 'word', 'direction', 'result', 'at', 'slug', 'activity'
];
const REVIEW_ORIGINAL = [
    'id', 'user_id', 'word', 'direction', 'due', 'step', 'streak', 'lapses',
    'last_result', 'last_at', 'sources'
];
const EVENT_ORIGINAL = [
    'id', 'user_id', 'word', 'direction', 'result', 'at', 'slug', 'activity'
];

const MODULES = [
    '../server/paths',
    '../server/db',
    '../server/db/repos/review',
    '../server/db/repos/events',
    '../server/progress',
    '../server/db/migrations/001-initial',
    '../server/db/migrations/002-plan-store',
    '../server/db/migrations/003-media-and-ai-planning',
    '../server/db/migrations/004-fsrs-review-state'
];

function stamp(file) {
    const names = [file, file + '-wal', file + '-shm'];
    const out = {};
    for (const name of names) {
        if (!fs.existsSync(name)) {
            out[name] = null;
            continue;
        }
        const stat = fs.statSync(name);
        out[name] = { size: stat.size, mtimeMs: stat.mtimeMs };
    }
    return out;
}

function unload() {
    for (const name of MODULES) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

function plain(rows, fields) {
    return rows.map((row) => {
        const out = {};
        for (const field of fields) out[field] = row[field];
        return out;
    });
}

function columnNames(database, table) {
    if (table !== 'review_items' && table !== 'practice_events') {
        throw new Error('Unexpected table');
    }
    const sql = table === 'review_items'
        ? 'PRAGMA table_info(review_items)'
        : 'PRAGMA table_info(practice_events)';
    return database.prepare(sql).all().map((row) => row.name);
}

/**
 * Same ever-passed test as server/progress.js metrics(), lines 264–268.
 * @param {object} row
 * @param {Set<string>} passKeys word + NUL + direction for this user
 */
function everPassed(row, passKeys) {
    return Boolean(
        passKeys.has(row.word + '\0' + row.direction)
        || row.last_result === 'pass'
        || (Number(row.step) >= 1)
    );
}

/**
 * Ladder stage, same rules as stageFor in server/progress.js.
 * @param {object} row
 * @param {boolean} ever
 */
function ladderStage(row, ever) {
    const step = Number.isInteger(row.step) ? row.step : 0;
    if (!ever) return 'new';
    if (step >= 3) return 'longTerm';
    return 'learning';
}

/**
 * ADR-0009 §2 stage after FSRS stability is stored.
 * @param {boolean} ever
 * @param {number|null} stability
 */
function adrStage(ever, stability) {
    if (!ever) return 'new';
    if (Number(stability) >= 14) return 'longTerm';
    return 'learning';
}

function emptyStages() {
    return {
        he_en: { new: 0, learning: 0, longTerm: 0 },
        en_he: { new: 0, learning: 0, longTerm: 0 }
    };
}

function countStages(rows, passKeys, stageOf) {
    const stages = emptyStages();
    for (const row of rows) {
        if (!stages[row.direction]) continue;
        const ever = everPassed(row, passKeys);
        stages[row.direction][stageOf(row, ever)] += 1;
    }
    return stages;
}

function openFixture() {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-fsrs-'));
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = tmp;
    unload();
    const db = require('../server/db');
    db.close();
    const database = db.open({ force: true });
    const live = path.resolve(LIVE_DB);
    assert.notEqual(path.resolve(db.databasePath()), live);
    assert.ok(path.resolve(db.databasePath()).startsWith(path.resolve(tmp)));

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
        `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`
    );
    record.run(1, '001-initial.js', NOW);
    record.run(2, '002-plan-store.js', NOW);
    record.run(3, '003-media-and-ai-planning.js', NOW);

    const insertUser = database.prepare(
        `INSERT INTO users (username, password_hash, created_at, updated_at)
         VALUES (?, ?, ?, ?)`
    );
    const user1 = Number(insertUser.run('learner', 'hash', NOW, NOW).lastInsertRowid);
    const user2 = Number(insertUser.run('other', 'hash', NOW, NOW).lastInsertRowid);

    const reviewRows = [
        row(user1, 'חדש', 'he_en', '2026-01-01T00:00:00.000Z', 0, 0, 0, null, null, '[]'),
        row(user1, 'שלב1', 'he_en', '2026-01-02T00:00:00.000Z', 1, 1, 0, 'pass', AT, '["lesson-a"]'),
        row(user1, 'שלב2', 'he_en', '2026-01-03T00:00:00.000Z', 2, 2, 1, 'pass', AT, '["lesson-b"]'),
        row(user1, 'שלב3', 'he_en', '2026-01-04T00:00:00.000Z', 3, 1, 0, 'pass', AT, '["שיעור א"]'),
        row(user1, 'שלב4', 'he_en', '2026-01-05T00:00:00.000Z', 4, 2, 6, 'pass', AT, '["x","y"]'),
        row(user1, 'כשל', 'he_en', '2026-01-06T00:00:00.000Z', 0, 0, 2, 'miss', AT, '[]'),
        row(user1, 'פספוס', 'he_en', '2026-01-07T00:00:00.000Z', 0, 0, 0, 'miss', AT, '[]'),
        row(user1, 'זר', 'he_en', '2026-01-08T00:00:00.000Z', 0, 0, 0, null, null, '[]'),
        row(user1, 'גבוה', 'he_en', '2026-01-09T00:00:00.000Z', 5, 1, 0, 'pass', AT, '[]'),
        row(user1, 'כיוון', 'he_en', '2026-01-10T00:00:00.000Z', 0, 0, 0, null, null, '[]'),
        row(user1, 'אפס', 'en_he', '2026-01-11T00:00:00.000Z', 0, 1, 0, 'pass', AT, '[]'),
        row(user1, 'אירוע', 'en_he', '2026-01-12T00:00:00.000Z', 0, 0, 0, null, null, '["note"]'),
        row(user1, 'מדרגה', 'en_he', '2026-01-13T00:00:00.000Z', 3, 0, 1, 'miss', AT, '[]'),
        row(user1, 'עדיין', 'en_he', '2026-01-14T00:00:00.000Z', 2, 0, 0, 'not_yet', AT, '[]'),
        row(user2, 'אירוע', 'en_he', '2026-02-01T00:00:00.000Z', 0, 0, 0, null, null, '[]'),
        row(user2, 'שלב1', 'en_he', '2026-02-02T00:00:00.000Z', 1, 4, 2, 'pass', AT, '[]')
    ];
    const insertReview = database.prepare(
        `INSERT INTO review_items (
            user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
    );
    for (const item of reviewRows) {
        insertReview.run(
            item.user_id, item.word, item.direction, item.due, item.step,
            item.streak, item.lapses, item.last_result, item.last_at, item.sources
        );
    }

    const insertEvent = database.prepare(
        `INSERT INTO practice_events (
            id, user_id, word, direction, result, at, slug, activity
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
    );
    insertEvent.run('evt-own', user1, 'אירוע', 'en_he', 'pass', AT, 'cafe', 'vocab');
    insertEvent.run('evt-other-user', user2, 'זר', 'he_en', 'pass', AT, null, null);
    insertEvent.run('evt-other-dir', user1, 'כיוון', 'en_he', 'pass', AT, null, null);
    insertEvent.run('evt-miss', user1, 'כשל', 'he_en', 'miss', AT, null, null);

    const passKeys = new Map();
    function keysFor(userId) {
        if (!passKeys.has(userId)) {
            const found = database.prepare(
                `SELECT word, direction FROM practice_events
                 WHERE user_id = ? AND result = 'pass'`
            ).all(userId);
            const set = new Set();
            for (const foundRow of found) set.add(foundRow.word + '\0' + foundRow.direction);
            passKeys.set(userId, set);
        }
        return passKeys.get(userId);
    }

    function rowsFor(userId) {
        return database.prepare(
            `SELECT user_id, word, direction, due, step, streak, lapses,
                    last_result, last_at, sources
             FROM review_items WHERE user_id = ?`
        ).all(userId);
    }

    const before = {
        user1: countStages(rowsFor(user1), keysFor(user1), ladderStage),
        user2: countStages(rowsFor(user2), keysFor(user2), ladderStage)
    };
    const ladderBefore = plain(
        database.prepare(
            `SELECT id, user_id, word, direction, due, step, streak, lapses,
                    last_result, last_at, sources
             FROM review_items ORDER BY id`
        ).all(),
        LADDER_FIELDS
    );
    const eventsBefore = plain(
        database.prepare(
            `SELECT id, user_id, word, direction, result, at, slug, activity
             FROM practice_events ORDER BY user_id, id`
        ).all(),
        EVENT_FIELDS
    );

    return {
        tmp,
        db,
        database,
        user1,
        user2,
        before,
        ladderBefore,
        eventsBefore,
        rowsFor,
        keysFor,
        fsrs: require('../server/db/migrations/004-fsrs-review-state'),
        review: require('../server/db/repos/review'),
        events: require('../server/db/repos/events'),
        progress: require('../server/progress')
    };
}

function row(userId, word, direction, due, step, streak, lapses, lastResult, lastAt, sources) {
    return {
        user_id: userId,
        word,
        direction,
        due,
        step,
        streak,
        lapses,
        last_result: lastResult,
        last_at: lastAt,
        sources
    };
}

function closeFixture(fx) {
    if (fx && fx.db) fx.db.close();
    if (fx && fx.tmp) fs.rmSync(fx.tmp, { recursive: true, force: true });
    delete process.env.DATA_DIR;
    unload();
}

function assertBackfill(fx) {
    const expected = {
        'חדש|he_en': { stability: null, difficulty: null, reps: 0, fsrs_state: 'new' },
        'שלב1|he_en': { stability: 3, difficulty: 5, reps: 1, fsrs_state: 'review' },
        'שלב2|he_en': { stability: 7, difficulty: 5.5, reps: 3, fsrs_state: 'review' },
        'שלב3|he_en': { stability: 14, difficulty: 5, reps: 1, fsrs_state: 'review' },
        'שלב4|he_en': { stability: 30, difficulty: 7.5, reps: 8, fsrs_state: 'review' },
        'כשל|he_en': { stability: null, difficulty: null, reps: 0, fsrs_state: 'learning' },
        'פספוס|he_en': { stability: null, difficulty: null, reps: 0, fsrs_state: 'new' },
        'זר|he_en': { stability: null, difficulty: null, reps: 0, fsrs_state: 'new' },
        'גבוה|he_en': { stability: 30, difficulty: 5, reps: 1, fsrs_state: 'review' },
        'כיוון|he_en': { stability: null, difficulty: null, reps: 0, fsrs_state: 'new' },
        'אפס|en_he': { stability: 1, difficulty: 5, reps: 1, fsrs_state: 'review' },
        'אירוע|en_he': { stability: 1, difficulty: 5, reps: 0, fsrs_state: 'review' },
        'מדרגה|en_he': { stability: 14, difficulty: 5.5, reps: 1, fsrs_state: 'review' },
        'עדיין|en_he': { stability: 7, difficulty: 5, reps: 0, fsrs_state: 'review' }
    };
    const stored = fx.database.prepare(
        `SELECT word, direction, stability, difficulty, reps, fsrs_state,
                last_grade, suspended
         FROM review_items WHERE user_id = ?`
    ).all(fx.user1);
    assert.equal(stored.length, Object.keys(expected).length);
    for (const item of stored) {
        const want = expected[item.word + '|' + item.direction];
        assert.ok(want, item.word);
        assert.equal(item.stability, want.stability, item.word);
        assert.equal(item.difficulty, want.difficulty, item.word);
        assert.equal(item.reps, want.reps, item.word);
        assert.equal(item.fsrs_state, want.fsrs_state, item.word);
        assert.equal(item.last_grade, null, item.word);
        assert.equal(item.suspended, 0, item.word);
    }

    const other = fx.database.prepare(
        `SELECT word, direction, stability, fsrs_state, reps, difficulty
         FROM review_items WHERE user_id = ? ORDER BY word`
    ).all(fx.user2);
    assert.equal(other.length, 2);
    const isolated = other.find((item) => item.word === 'אירוע');
    assert.equal(isolated.fsrs_state, 'new');
    assert.equal(isolated.stability, null);
    const passed = other.find((item) => item.word === 'שלב1');
    assert.equal(passed.fsrs_state, 'review');
    assert.equal(passed.stability, 3);
    assert.equal(passed.difficulty, 6);
    assert.equal(passed.reps, 6);

    const event = fx.database.prepare(
        `SELECT grade, kind, lemma FROM practice_events WHERE user_id = ? AND id = ?`
    ).get(fx.user1, 'evt-own');
    assert.equal(event.grade, null);
    assert.equal(event.kind, 'word');
    assert.equal(event.lemma, null);
}

function assertStages(fx) {
    const metrics1 = fx.progress.metrics(fx.user1);
    const metrics2 = fx.progress.metrics(fx.user2);
    assert.deepEqual(metrics1.stages, fx.before.user1);
    assert.deepEqual(metrics2.stages, fx.before.user2);
    assert.deepEqual(fx.before.user1, {
        he_en: { new: 5, learning: 2, longTerm: 3 },
        en_he: { new: 0, learning: 3, longTerm: 1 }
    });
    assert.deepEqual(fx.before.user2, {
        he_en: { new: 0, learning: 0, longTerm: 0 },
        en_he: { new: 1, learning: 1, longTerm: 0 }
    });

    for (const userId of [fx.user1, fx.user2]) {
        const rows = fx.rowsFor(userId);
        const keys = fx.keysFor(userId);
        const mapped = countStages(rows, keys, (item, ever) => {
            const stability = fx.database.prepare(
                `SELECT stability FROM review_items
                 WHERE user_id = ? AND word = ? AND direction = ?`
            ).get(userId, item.word, item.direction).stability;
            return adrStage(ever, stability);
        });
        const metrics = userId === fx.user1 ? metrics1 : metrics2;
        assert.deepEqual(mapped, metrics.stages);
    }
}

function assertLadderUntouched(fx) {
    const ladderAfter = plain(
        fx.database.prepare(
            `SELECT id, user_id, word, direction, due, step, streak, lapses,
                    last_result, last_at, sources
             FROM review_items ORDER BY id`
        ).all(),
        LADDER_FIELDS
    );
    assert.deepEqual(ladderAfter, fx.ladderBefore);
    const eventsAfter = plain(
        fx.database.prepare(
            `SELECT id, user_id, word, direction, result, at, slug, activity
             FROM practice_events ORDER BY user_id, id`
        ).all(),
        EVENT_FIELDS
    );
    assert.deepEqual(eventsAfter, fx.eventsBefore);
}

describe('FSRS review columns (INIT-006/SPEC-002)', () => {
    let liveBefore;

    before(() => {
        liveBefore = stamp(LIVE_DB);
    });

    after(() => {
        delete process.env.DATA_DIR;
        unload();
        assert.deepEqual(stamp(LIVE_DB), liveBefore);
    });

    it('ac-1 and ac-2: stage counts match and ladder values stay byte-identical', () => {
        const fx = openFixture();
        try {
            const progressSrc = fs.readFileSync(path.join(root, 'server', 'progress.js'), 'utf8');
            assert.match(progressSrc, /row\.last_result === 'pass'/);
            assert.match(progressSrc, /Number\(row\.step\) >= 1/);
            assert.match(progressSrc, /result = 'pass'/);

            fx.fsrs.up(fx.database);
            assertStages(fx);
            assertLadderUntouched(fx);
            assertBackfill(fx);
            assert.deepEqual(columnNames(fx.database, 'review_items'), REVIEW_ORIGINAL.concat([
                'stability', 'difficulty', 'reps', 'fsrs_state', 'last_grade', 'suspended'
            ]));
            assert.deepEqual(columnNames(fx.database, 'practice_events'), EVENT_ORIGINAL.concat([
                'grade', 'kind', 'lemma'
            ]));
        } finally {
            closeFixture(fx);
        }
    });

    it('ac-3: down restores the original columns and up after down is clean', () => {
        const fx = openFixture();
        try {
            fx.fsrs.up(fx.database);
            assertBackfill(fx);
            fx.fsrs.down(fx.database);

            assert.deepEqual(columnNames(fx.database, 'review_items'), REVIEW_ORIGINAL);
            assert.deepEqual(columnNames(fx.database, 'practice_events'), EVENT_ORIGINAL);
            assertLadderUntouched(fx);
            assert.equal(
                fx.database.prepare(
                    `SELECT name FROM sqlite_master
                     WHERE name IN ('review_items__fsrs_down', 'practice_events__fsrs_down')`
                ).all().length,
                0
            );
            const versionAfterDown = fx.database.prepare(
                'SELECT version FROM schema_migrations ORDER BY version'
            ).all().map((item) => item.version);
            assert.deepEqual(versionAfterDown, [1, 2, 3]);

            fx.db.migrate();
            assert.deepEqual(columnNames(fx.database, 'review_items'), REVIEW_ORIGINAL.concat([
                'stability', 'difficulty', 'reps', 'fsrs_state', 'last_grade', 'suspended'
            ]));
            assertBackfill(fx);
            assertLadderUntouched(fx);
            assertStages(fx);
            const versions = fx.database.prepare(
                'SELECT version, name FROM schema_migrations ORDER BY version'
            ).all();
            const byVersion = new Map(versions.map((item) => [item.version, item.name]));
            assert.match(byVersion.get(4), /^004-fsrs-review-state\.js$/);

            fx.fsrs.down(fx.database);
            fx.db.migrate();
            assertBackfill(fx);
            assertLadderUntouched(fx);
            const indexes = fx.database.prepare(
                `SELECT name FROM sqlite_master WHERE type = 'index' AND name IN (
                    'idx_review_items_user_due', 'idx_practice_events_user_at'
                ) ORDER BY name`
            ).all().map((item) => item.name);
            assert.deepEqual(indexes, ['idx_practice_events_user_at', 'idx_review_items_user_due']);
            const again = fx.database.prepare(
                'SELECT COUNT(*) AS n FROM schema_migrations WHERE version = 4'
            ).get();
            assert.equal(again.n, 1);
        } finally {
            closeFixture(fx);
        }
    });

    it('ac-4: grade outside 1–4 and kind other than word or form fail CHECK', () => {
        const fx = openFixture();
        try {
            fx.fsrs.up(fx.database);
            const insert = fx.database.prepare(
                `INSERT INTO practice_events (
                    id, user_id, word, direction, result, at, grade, kind
                ) VALUES (?, ?, ?, 'he_en', 'pass', ?, ?, ?)`
            );
            assert.throws(
                () => insert.run('bad-0', fx.user1, 'מילה', NOW, 0, 'word'),
                /CHECK constraint failed/i
            );
            assert.throws(
                () => insert.run('bad-5', fx.user1, 'מילה', NOW, 5, 'word'),
                /CHECK constraint failed/i
            );
            assert.throws(
                () => insert.run('bad-kind', fx.user1, 'מילה', NOW, 3, 'sentence'),
                /CHECK constraint failed/i
            );
            assert.throws(
                () => insert.run('bad-empty', fx.user1, 'מילה', NOW, null, ''),
                /CHECK constraint failed/i
            );
            insert.run('ok-null', fx.user1, 'מילה', NOW, null, 'word');
            insert.run('ok-ends', fx.user1, 'מילה2', NOW, 1, 'form');
            insert.run('ok-easy', fx.user1, 'מילה3', NOW, 4, 'word');
            assert.equal(
                fx.database.prepare(
                    'SELECT COUNT(*) AS n FROM practice_events WHERE id IN (?, ?, ?)'
                ).get('ok-null', 'ok-ends', 'ok-easy').n,
                3
            );

            assert.throws(
                () => fx.database.prepare(
                    `UPDATE review_items SET last_grade = ? WHERE user_id = ? AND word = ?`
                ).run(0, fx.user1, 'חדש'),
                /CHECK constraint failed/i
            );
            assert.throws(
                () => fx.database.prepare(
                    `UPDATE review_items SET last_grade = ? WHERE user_id = ? AND word = ?`
                ).run(5, fx.user1, 'חדש'),
                /CHECK constraint failed/i
            );
            assert.throws(
                () => fx.database.prepare(
                    `UPDATE review_items SET fsrs_state = ? WHERE user_id = ? AND word = ?`
                ).run('done', fx.user1, 'חדש'),
                /CHECK constraint failed/i
            );
            assert.throws(
                () => fx.database.prepare(
                    `UPDATE review_items SET suspended = ? WHERE user_id = ? AND word = ?`
                ).run(2, fx.user1, 'חדש'),
                /CHECK constraint failed/i
            );
            const still = fx.database.prepare(
                `SELECT fsrs_state, last_grade, suspended FROM review_items
                 WHERE user_id = ? AND word = ? AND direction = ?`
            ).get(fx.user1, 'חדש', 'he_en');
            assert.equal(still.fsrs_state, 'new');
            assert.equal(still.last_grade, null);
            assert.equal(still.suspended, 0);
        } finally {
            closeFixture(fx);
        }
    });

    it('repository reads and writes the new columns, and setSuspended covers both directions', () => {
        const fx = openFixture();
        try {
            fx.fsrs.up(fx.database);
            const hostile = "'; DROP TABLE users; --";
            fx.review.upsert({
                user_id: fx.user1,
                word: 'שעה',
                direction: 'he_en',
                due: '2026-06-01T00:00:00.000Z',
                step: 1,
                streak: 1,
                lapses: 0,
                last_result: 'pass',
                last_at: AT,
                sources: '[]',
                stability: 9,
                difficulty: 5,
                reps: 1,
                fsrs_state: 'review',
                last_grade: 3,
                suspended: false
            });
            fx.review.upsert({
                user_id: fx.user1,
                word: 'שעה',
                direction: 'en_he',
                due: '2026-06-02T00:00:00.000Z',
                step: 0,
                streak: 0,
                lapses: 0,
                last_result: null,
                last_at: null,
                sources: '[]'
            });
            fx.review.upsert({
                user_id: fx.user1,
                word: 'אחר',
                direction: 'he_en',
                due: '2026-06-03T00:00:00.000Z',
                step: 0,
                sources: '[]'
            });

            fx.review.upsert({
                user_id: fx.user1,
                word: 'שעה',
                direction: 'he_en',
                due: '2026-07-01T00:00:00.000Z',
                step: 2,
                streak: 2,
                lapses: 0,
                last_result: 'pass',
                last_at: AT,
                sources: '["kept"]'
            });
            const kept = fx.review.find(fx.user1, 'שעה', 'he_en');
            assert.equal(kept.due, '2026-07-01T00:00:00.000Z');
            assert.equal(kept.step, 2);
            assert.equal(kept.stability, 9);
            assert.equal(kept.difficulty, 5);
            assert.equal(kept.reps, 1);
            assert.equal(kept.fsrs_state, 'review');
            assert.equal(kept.last_grade, 3);
            assert.equal(kept.suspended, 0);
            assert.equal(kept.sources, '["kept"]');

            fx.review.upsert({
                user_id: fx.user1,
                word: 'שעה',
                direction: 'he_en',
                due: kept.due,
                step: kept.step,
                streak: kept.streak,
                lapses: kept.lapses,
                last_result: kept.last_result,
                last_at: kept.last_at,
                sources: kept.sources,
                stability: 12.5,
                last_grade: 4
            });
            const graded = fx.review.find(fx.user1, 'שעה', 'he_en');
            assert.equal(graded.stability, 12.5);
            assert.equal(graded.last_grade, 4);
            assert.equal(graded.difficulty, 5);
            assert.equal(graded.suspended, 0);

            fx.review.setSuspended(fx.user1, 'שעה', true);
            assert.equal(fx.review.find(fx.user1, 'שעה', 'he_en').suspended, 1);
            assert.equal(fx.review.find(fx.user1, 'שעה', 'en_he').suspended, 1);
            assert.equal(fx.review.find(fx.user1, 'אחר', 'he_en').suspended, 0);
            fx.review.setSuspended(fx.user1, 'שעה', false);
            assert.equal(fx.review.find(fx.user1, 'שעה', 'he_en').suspended, 0);
            assert.equal(fx.review.find(fx.user1, 'שעה', 'en_he').suspended, 0);

            fx.review.setSuspended(fx.user1, hostile, true);
            assert.equal(
                fx.database.prepare(`SELECT COUNT(*) AS n FROM users`).get().n >= 2,
                true
            );
            assert.equal(fx.review.find(fx.user1, hostile, 'he_en'), undefined);

            const listed = fx.review.listByUser(fx.user1);
            const listedWord = listed.find((item) => item.word === 'שעה' && item.direction === 'he_en');
            assert.equal(listedWord.stability, 12.5);
            assert.ok(Object.prototype.hasOwnProperty.call(listedWord, 'suspended'));

            fx.events.insert({
                id: 'repo-grade',
                user_id: fx.user1,
                word: 'שעה',
                direction: 'he_en',
                result: 'pass',
                at: AT,
                grade: 2,
                kind: 'form',
                lemma: 'ש.ע.ה'
            });
            const saved = fx.events.find(fx.user1, 'repo-grade');
            assert.equal(saved.grade, 2);
            assert.equal(saved.kind, 'form');
            assert.equal(saved.lemma, 'ש.ע.ה');
            fx.events.insert({
                id: 'repo-default',
                user_id: fx.user1,
                word: 'שעה',
                direction: 'en_he',
                result: 'miss',
                at: AT
            });
            const omitted = fx.events.find(fx.user1, 'repo-default');
            assert.equal(omitted.grade, null);
            assert.equal(omitted.kind, 'word');
            assert.equal(omitted.lemma, null);
            assert.throws(
                () => fx.events.insert({
                    id: 'repo-bad',
                    user_id: fx.user1,
                    word: 'שעה',
                    direction: 'he_en',
                    result: 'pass',
                    at: AT,
                    grade: 0
                }),
                /CHECK constraint failed/i
            );

            const migrationSrc = fs.readFileSync(
                path.join(root, 'server', 'db', 'migrations', '004-fsrs-review-state.js'),
                'utf8'
            );
            assert.equal(migrationSrc.includes('${'), false);
            const reviewSrc = fs.readFileSync(
                path.join(root, 'server', 'db', 'repos', 'review.js'),
                'utf8'
            );
            assert.equal(/SELECT \*/.test(reviewSrc), false);
            const eventsSrc = fs.readFileSync(
                path.join(root, 'server', 'db', 'repos', 'events.js'),
                'utf8'
            );
            assert.equal(/SELECT \*/.test(eventsSrc), false);
        } finally {
            closeFixture(fx);
        }
    });
});
