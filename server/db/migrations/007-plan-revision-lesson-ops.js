/**
 * Allow lesson stack operations in the plan revision log.
 *
 * Migration 006 already ran on existing databases with a CHECK list that
 * stops before insert_lesson. Changing 006 does not change those databases.
 * This migration rebuilds plan_revisions when that CHECK is missing
 * insert_lesson, and copies every row. A database created from the current
 * 006 already has the new list, so this migration only records its version.
 *
 * Provenance: INIT-010/SPEC-002
 */
'use strict';

const VERSION = 7;

const CREATE_NEW = `
    CREATE TABLE plan_revisions_new (
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
            CHECK (length(summary) <= 500),
        document_gz BLOB NOT NULL,
        document_sha256 TEXT NOT NULL
            CHECK (length(document_sha256) = 64),
        created_at TEXT NOT NULL
            CHECK (length(created_at) BETWEEN 1 AND 40),
        PRIMARY KEY (plan_id, revision)
    );
`;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    const row = db.prepare(
        "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'plan_revisions'"
    ).get();
    if (!row || !row.sql) return;
    if (String(row.sql).includes("'insert_lesson'")) return;

    db.exec(CREATE_NEW);
    db.exec(`
        INSERT INTO plan_revisions_new (
            plan_id, revision, actor_kind, actor_name, tool, client, request_id,
            op, path, field, summary, document_gz, document_sha256, created_at
        )
        SELECT
            plan_id, revision, actor_kind, actor_name, tool, client, request_id,
            op, path, field, summary, document_gz, document_sha256, created_at
        FROM plan_revisions;
        DROP TABLE plan_revisions;
        ALTER TABLE plan_revisions_new RENAME TO plan_revisions;
        CREATE INDEX idx_plan_revisions_plan_created
            ON plan_revisions (plan_id, created_at);
    `);
}

/**
 * Restore the pre-007 CHECK when no row uses a lesson operation.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    const used = db.prepare(
        `SELECT 1 AS ok FROM plan_revisions
         WHERE op IN ('insert_lesson', 'move_lesson', 'remove_lesson') LIMIT 1`
    ).get();
    if (used) {
        throw new Error('Cannot reverse migration 007 while lesson operations are stored.');
    }
    const row = db.prepare(
        "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'plan_revisions'"
    ).get();
    if (!row || !String(row.sql).includes("'insert_lesson'")) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
        return;
    }
    db.exec(`
        CREATE TABLE plan_revisions_old (
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
                    'move_block', 'remove_block', 'assign', 'publish', 'restore'
                )),
            path TEXT,
            field TEXT,
            summary TEXT NOT NULL
                CHECK (length(summary) <= 500),
            document_gz BLOB NOT NULL,
            document_sha256 TEXT NOT NULL
                CHECK (length(document_sha256) = 64),
            created_at TEXT NOT NULL
                CHECK (length(created_at) BETWEEN 1 AND 40),
            PRIMARY KEY (plan_id, revision)
        );
        INSERT INTO plan_revisions_old (
            plan_id, revision, actor_kind, actor_name, tool, client, request_id,
            op, path, field, summary, document_gz, document_sha256, created_at
        )
        SELECT
            plan_id, revision, actor_kind, actor_name, tool, client, request_id,
            op, path, field, summary, document_gz, document_sha256, created_at
        FROM plan_revisions;
        DROP TABLE plan_revisions;
        ALTER TABLE plan_revisions_old RENAME TO plan_revisions;
        CREATE INDEX idx_plan_revisions_plan_created
            ON plan_revisions (plan_id, created_at);
    `);
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { up, down, VERSION };
