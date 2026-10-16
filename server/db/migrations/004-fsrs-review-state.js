/**
 * Additive FSRS state on review_items and practice_events.
 * Ladder columns and due dates are not rewritten.
 * Backfill follows ADR-0009 §2. "Ever passed" is the same test as
 * metrics() in server/progress.js: a pass event, last_result pass, or step >= 1.
 * A never-passed row with lapses stays fsrs_state learning; otherwise new.
 * Provenance: INIT-006/SPEC-002
 */
'use strict';

/**
 * Version recorded in schema_migrations. Must match this file's numeric prefix.
 * migrate() derives the same number from the filename.
 */
const VERSION = 4;

const LADDER = [1, 3, 7, 14, 30];

/**
 * Run fn inside a transaction. If the caller already opened one (migrate()),
 * join it and let the caller commit or roll back.
 * @param {import('node:sqlite').DatabaseSync} db
 * @param {() => void} fn
 */
function withTransaction(db, fn) {
    let owned = false;
    try {
        db.exec('BEGIN IMMEDIATE');
        owned = true;
    } catch (err) {
        const message = String(err && err.message || err);
        if (!/transaction/i.test(message)) throw err;
    }
    try {
        fn();
        if (owned) db.exec('COMMIT');
    } catch (err) {
        if (owned) {
            try { db.exec('ROLLBACK'); } catch (_) { /* already closed */ }
        }
        throw err;
    }
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 * @param {string} sql literal PRAGMA, table name is a constant
 * @returns {string[]}
 */
function columnNames(db, sql) {
    return db.prepare(sql).all().map((row) => row.name);
}

/**
 * Copy review_items without the FSRS columns. Used when DROP COLUMN is unavailable.
 * Original row values, including ids, are kept.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function rebuildReviewItems(db) {
    db.exec(`
        CREATE TABLE review_items__fsrs_down (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            direction TEXT NOT NULL
                CHECK (direction IN ('he_en', 'en_he')),
            due TEXT NOT NULL,
            step INTEGER NOT NULL DEFAULT 0,
            streak INTEGER NOT NULL DEFAULT 0,
            lapses INTEGER NOT NULL DEFAULT 0,
            last_result TEXT,
            last_at TEXT,
            sources TEXT NOT NULL DEFAULT '[]',
            UNIQUE (user_id, word, direction)
        );
        INSERT INTO review_items__fsrs_down (
            id, user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources
        )
        SELECT
            id, user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources
        FROM review_items;
        DROP TABLE review_items;
        ALTER TABLE review_items__fsrs_down RENAME TO review_items;
        CREATE INDEX idx_review_items_user_due ON review_items (user_id, due);
    `);
    const sequence = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'sqlite_sequence'`
    ).get();
    if (!sequence) return;
    const max = db.prepare('SELECT MAX(id) AS n FROM review_items').get();
    const seq = max && max.n != null ? max.n : 0;
    const existing = db.prepare(
        `SELECT 1 AS ok FROM sqlite_sequence WHERE name = 'review_items'`
    ).get();
    if (existing) {
        db.prepare(
            `UPDATE sqlite_sequence SET seq = ? WHERE name = 'review_items'`
        ).run(seq);
    } else if (seq > 0) {
        db.prepare(
            `INSERT INTO sqlite_sequence (name, seq) VALUES ('review_items', ?)`
        ).run(seq);
    }
}

/**
 * Copy practice_events without grade, kind, and lemma.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function rebuildPracticeEvents(db) {
    db.exec(`
        CREATE TABLE practice_events__fsrs_down (
            id TEXT NOT NULL,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            direction TEXT NOT NULL
                CHECK (direction IN ('he_en', 'en_he')),
            result TEXT NOT NULL,
            at TEXT NOT NULL,
            slug TEXT,
            activity TEXT,
            PRIMARY KEY (user_id, id)
        );
        INSERT INTO practice_events__fsrs_down (
            id, user_id, word, direction, result, at, slug, activity
        )
        SELECT id, user_id, word, direction, result, at, slug, activity
        FROM practice_events;
        DROP TABLE practice_events;
        ALTER TABLE practice_events__fsrs_down RENAME TO practice_events;
        CREATE INDEX idx_practice_events_user_at ON practice_events (user_id, at);
    `);
}

/**
 * Drop known columns, or rebuild the table if DROP COLUMN fails.
 * Statements are literals. Names are not taken from outside this file.
 * @param {import('node:sqlite').DatabaseSync} db
 * @param {string} pragma literal PRAGMA table_info(...)
 * @param {Array<[string, string]>} drops [column, ALTER statement]
 * @param {string} savepoint
 * @param {(db: import('node:sqlite').DatabaseSync) => void} rebuild
 */
function dropOrRebuild(db, pragma, drops, savepoint, rebuild) {
    const present = new Set(columnNames(db, pragma));
    const todo = drops.filter(([name]) => present.has(name));
    if (todo.length === 0) return;
    db.exec('SAVEPOINT ' + savepoint);
    try {
        for (const [, sql] of todo) db.exec(sql);
        db.exec('RELEASE ' + savepoint);
    } catch (err) {
        db.exec('ROLLBACK TO ' + savepoint);
        db.exec('RELEASE ' + savepoint);
        rebuild(db);
    }
}

/**
 * Set FSRS columns from the ladder. Does not assign due, step, streak,
 * lapses, last_result, last_at, or sources.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function backfill(db) {
    const passed = db.prepare(`
        UPDATE review_items
        SET stability = CASE
                WHEN step = 0 THEN ?
                WHEN step = 1 THEN ?
                WHEN step = 2 THEN ?
                WHEN step = 3 THEN ?
                WHEN step = 4 THEN ?
                WHEN step > 4 THEN ?
                ELSE ?
            END,
            difficulty = ? + (MIN(lapses, ?) * ?),
            reps = streak + lapses,
            fsrs_state = ?
        WHERE last_result = 'pass'
           OR COALESCE(step, 0) >= 1
           OR EXISTS (
                SELECT 1 FROM practice_events AS e
                WHERE e.user_id = review_items.user_id
                  AND e.word = review_items.word
                  AND e.direction = review_items.direction
                  AND e.result = 'pass'
           )
    `);
    passed.run(
        LADDER[0], LADDER[1], LADDER[2], LADDER[3], LADDER[4],
        LADDER[4], LADDER[0],
        5, 5, 0.5,
        'review'
    );

    const never = db.prepare(`
        UPDATE review_items
        SET stability = NULL,
            difficulty = NULL,
            reps = 0,
            fsrs_state = CASE WHEN lapses > 0 THEN ? ELSE ? END
        WHERE COALESCE(last_result, '') != 'pass'
          AND COALESCE(step, 0) < 1
          AND NOT EXISTS (
                SELECT 1 FROM practice_events AS e
                WHERE e.user_id = review_items.user_id
                  AND e.word = review_items.word
                  AND e.direction = review_items.direction
                  AND e.result = 'pass'
          )
    `);
    never.run('learning', 'new');
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    withTransaction(db, () => {
        db.exec(`
            ALTER TABLE review_items ADD COLUMN stability REAL;
            ALTER TABLE review_items ADD COLUMN difficulty REAL;
            ALTER TABLE review_items ADD COLUMN reps INTEGER NOT NULL DEFAULT 0;
            ALTER TABLE review_items ADD COLUMN fsrs_state TEXT
                CHECK (fsrs_state IN ('new', 'learning', 'review', 'relearning'));
            ALTER TABLE review_items ADD COLUMN last_grade INTEGER
                CHECK (last_grade BETWEEN 1 AND 4);
            ALTER TABLE review_items ADD COLUMN suspended INTEGER NOT NULL DEFAULT 0
                CHECK (suspended IN (0, 1));

            ALTER TABLE practice_events ADD COLUMN grade INTEGER
                CHECK (grade BETWEEN 1 AND 4);
            ALTER TABLE practice_events ADD COLUMN kind TEXT NOT NULL DEFAULT 'word'
                CHECK (kind IN ('word', 'form'));
            ALTER TABLE practice_events ADD COLUMN lemma TEXT;
        `);
        backfill(db);
    });
}

/**
 * Remove the columns this migration added. Every original row and value stays.
 * Forgets schema version 4 so migrate() can apply up again.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    withTransaction(db, () => {
        dropOrRebuild(
            db,
            'PRAGMA table_info(review_items)',
            [
                ['stability', 'ALTER TABLE review_items DROP COLUMN stability'],
                ['difficulty', 'ALTER TABLE review_items DROP COLUMN difficulty'],
                ['reps', 'ALTER TABLE review_items DROP COLUMN reps'],
                ['fsrs_state', 'ALTER TABLE review_items DROP COLUMN fsrs_state'],
                ['last_grade', 'ALTER TABLE review_items DROP COLUMN last_grade'],
                ['suspended', 'ALTER TABLE review_items DROP COLUMN suspended']
            ],
            'fsrs_drop_review',
            rebuildReviewItems
        );
        dropOrRebuild(
            db,
            'PRAGMA table_info(practice_events)',
            [
                ['grade', 'ALTER TABLE practice_events DROP COLUMN grade'],
                ['kind', 'ALTER TABLE practice_events DROP COLUMN kind'],
                ['lemma', 'ALTER TABLE practice_events DROP COLUMN lemma']
            ],
            'fsrs_drop_events',
            rebuildPracticeEvents
        );
        const migrations = db.prepare(
            `SELECT 1 AS ok FROM sqlite_master
             WHERE type = 'table' AND name = 'schema_migrations'`
        ).get();
        if (migrations) {
            db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
        }
    });
}

module.exports = { up, down, VERSION };
