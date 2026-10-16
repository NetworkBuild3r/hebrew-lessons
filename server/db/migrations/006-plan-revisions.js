/**
 * Plan revision log and field provenance (ADR-0011 §2, §4, §9).
 *
 * plan_revisions   one row per successful plan write: who, which operation, a
 *                  value-free summary, and the full document gzipped.
 * plan_field_marks the last writer of each id-path field, and the last time a
 *                  person wrote it, so read_part can show an advisory edit note.
 *
 * Additive only. `plans`, `audit_log`, and every other table are not altered,
 * and no existing row changes. The plan document and its schema are not
 * touched: provenance lives beside the plan, not in it. Plans that exist today
 * have no log rows until their first write after this migration.
 * Neither table has a column that refuses a write. Nothing here pins or
 * locks a field (ADR-0011 §4).
 *
 * Foreign keys follow 005: REFERENCES ... ON DELETE CASCADE. Migration 002
 * declares none, and nothing deletes a plan today, so the cascade only matters
 * to tests and to a future delete.
 * The runner (server/db/index.js migrate) wraps up() in one transaction.
 * Provenance: INIT-008/SPEC-003; lesson ops in op CHECK: INIT-010/SPEC-002
 */
'use strict';

/**
 * Version recorded in schema_migrations. Must match this file's numeric prefix.
 * migrate() derives the same number from the filename.
 */
const VERSION = 6;

/** Value-free summary cap, in characters. */
const SUMMARY_MAX = 500;

/** Cap on the stored previous value of a field, in bytes (ADR-0011 §4). */
const PREVIOUS_VALUE_MAX_BYTES = 32768;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE plan_revisions (
            plan_id TEXT NOT NULL
                REFERENCES plans(id) ON DELETE CASCADE,
            revision INTEGER NOT NULL
                CHECK (revision >= 1),
            actor_kind TEXT NOT NULL
                CHECK (actor_kind IN ('ai', 'human', 'system')),
            actor_name TEXT NOT NULL
                CHECK (length(actor_name) BETWEEN 1 AND 128),
            tool TEXT,
            client TEXT,
            request_id TEXT,
            op TEXT NOT NULL
                CHECK (op IN (
                    'create', 'baseline', 'set_field', 'replace_part', 'insert_block',
                    'move_block', 'remove_block', 'insert_lesson', 'move_lesson', 'remove_lesson',
                    'assign', 'publish', 'restore'
                )),
            path TEXT,
            field TEXT,
            summary TEXT NOT NULL
                CHECK (length(summary) <= ${SUMMARY_MAX}),
            document_gz BLOB NOT NULL,
            document_sha256 TEXT NOT NULL
                CHECK (length(document_sha256) = 64),
            created_at TEXT NOT NULL
                CHECK (length(created_at) BETWEEN 1 AND 40),
            PRIMARY KEY (plan_id, revision)
        );

        CREATE INDEX idx_plan_revisions_plan_created
            ON plan_revisions (plan_id, created_at);

        CREATE TABLE plan_field_marks (
            plan_id TEXT NOT NULL
                REFERENCES plans(id) ON DELETE CASCADE,
            path TEXT NOT NULL,
            field TEXT NOT NULL,
            last_actor_kind TEXT NOT NULL
                CHECK (last_actor_kind IN ('ai', 'human', 'system')),
            last_actor_name TEXT NOT NULL
                CHECK (length(last_actor_name) BETWEEN 1 AND 128),
            last_revision INTEGER NOT NULL
                CHECK (last_revision >= 1),
            last_at TEXT NOT NULL
                CHECK (length(last_at) BETWEEN 1 AND 40),
            value_hash TEXT NOT NULL,
            human_revision INTEGER
                CHECK (human_revision IS NULL OR human_revision >= 1),
            human_at TEXT,
            human_actor_name TEXT,
            previous_value_json TEXT
                CHECK (
                    previous_value_json IS NULL
                    OR length(CAST(previous_value_json AS BLOB)) <= ${PREVIOUS_VALUE_MAX_BYTES}
                ),
            previous_writer_kind TEXT
                CHECK (
                    previous_writer_kind IS NULL
                    OR previous_writer_kind IN ('ai', 'human', 'system', 'unknown')
                ),
            PRIMARY KEY (plan_id, path, field)
        );

        CREATE INDEX idx_plan_field_marks_last_actor
            ON plan_field_marks (plan_id, last_actor_kind);
    `);
}

/**
 * Drop only the two tables and two indexes this migration created, and forget
 * version 6 so migrate() can apply it again. `plans` and `audit_log` are not named.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec(`
        DROP INDEX IF EXISTS idx_plan_field_marks_last_actor;
        DROP TABLE IF EXISTS plan_field_marks;
        DROP INDEX IF EXISTS idx_plan_revisions_plan_created;
        DROP TABLE IF EXISTS plan_revisions;
    `);
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { up, down, VERSION, SUMMARY_MAX, PREVIOUS_VALUE_MAX_BYTES };
