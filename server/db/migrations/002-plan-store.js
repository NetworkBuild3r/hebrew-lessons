/**
 * Plan document store — one row per plan.
 * Working-copy JSON, revision, status, audience, assignee, and a nullable snapshot.
 * Does not alter INIT-002 tables and does not read the lessons tree.
 * Provenance: INIT-004/SPEC-001
 */
'use strict';

/**
 * Version recorded in schema_migrations. Must match this file's numeric prefix.
 * migrate() derives the same number from the filename.
 */
const VERSION = 2;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE plans (
            id TEXT PRIMARY KEY,
            revision INTEGER NOT NULL DEFAULT 1
                CHECK (revision >= 1 AND revision <= 1000000),
            status TEXT NOT NULL DEFAULT 'draft'
                CHECK (status IN ('draft', 'published')),
            audience TEXT NOT NULL
                CHECK (audience IN ('shared', 'personal')),
            assignee_id TEXT,
            working_copy TEXT NOT NULL,
            snapshot TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            CHECK (length(id) BETWEEN 1 AND 64),
            CHECK (id NOT GLOB '*[^a-z0-9-]*'),
            CHECK (id NOT GLOB '-*'),
            CHECK (id NOT GLOB '*-'),
            CHECK (id NOT GLOB '*--*'),
            CHECK (length(working_copy) > 0),
            CHECK (snapshot IS NULL OR length(snapshot) > 0),
            CHECK (
                assignee_id IS NULL
                OR (
                    length(assignee_id) BETWEEN 8 AND 64
                    AND assignee_id GLOB '[a-z0-9]*'
                    AND assignee_id NOT GLOB '*[^a-z0-9-]*'
                )
            ),
            CHECK (
                (audience = 'shared' AND assignee_id IS NULL)
                OR (audience = 'personal' AND assignee_id IS NOT NULL)
            )
        );
    `);
}

/**
 * Drop only the table this migration created, and forget its version row
 * so migrate() can apply it again. INIT-002 tables are not named here.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec('DROP TABLE IF EXISTS plans');
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { up, down, VERSION };
