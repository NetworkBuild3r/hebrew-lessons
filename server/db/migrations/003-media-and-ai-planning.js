/**
 * Media asset rows and the per-learner AI-planning switch.
 * The switch default lives on the column. Existing rows and later inserts
 * that omit the column are off. Prompt text stays on the media row.
 * No file IO. The original client filename is not a column.
 * Provenance: INIT-004/SPEC-006
 */
'use strict';

/**
 * Version recorded in schema_migrations. Must match this file's numeric prefix.
 * migrate() derives the same number from the filename.
 */
const VERSION = 3;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        ALTER TABLE users ADD COLUMN ai_planning INTEGER NOT NULL DEFAULT 0
            CHECK (ai_planning IN (0, 1));

        CREATE TABLE media (
            id TEXT PRIMARY KEY,
            stored_name TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'approved')),
            alt_text TEXT NOT NULL DEFAULT '',
            prompt_text TEXT,
            provider TEXT,
            model_id TEXT,
            actor TEXT NOT NULL,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            CHECK (length(id) BETWEEN 1 AND 64),
            CHECK (id NOT GLOB '*[^a-z0-9-]*'),
            CHECK (id NOT GLOB '-*'),
            CHECK (id NOT GLOB '*-'),
            CHECK (id NOT GLOB '*--*'),
            CHECK (length(stored_name) BETWEEN 5 AND 68),
            CHECK (substr(stored_name, -4) = '.png'),
            CHECK (substr(stored_name, 1, length(stored_name) - 4) GLOB '[0-9a-f]*'),
            CHECK (substr(stored_name, 1, length(stored_name) - 4) NOT GLOB '*[^0-9a-f]*'),
            CHECK (prompt_text IS NULL OR length(prompt_text) BETWEEN 1 AND 4000),
            CHECK (provider IS NULL OR (
                length(provider) BETWEEN 1 AND 64
                AND provider NOT GLOB '*[^a-z0-9_-]*'
            )),
            CHECK (model_id IS NULL OR (
                length(model_id) BETWEEN 1 AND 64
                AND model_id NOT GLOB '*[^a-z0-9._-]*'
            )),
            CHECK (length(actor) BETWEEN 1 AND 128),
            CHECK (actor NOT GLOB '*[^a-z0-9:_.-]*'),
            CHECK (length(alt_text) <= 2000)
        );
    `);
}

/**
 * Drop the media table and remove users.ai_planning.
 * Plan tables and INIT-002 tables are not named here.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec('DROP TABLE IF EXISTS media');
    const columns = db.prepare('PRAGMA table_info(users)').all();
    if (columns.some((column) => column.name === 'ai_planning')) {
        db.exec('ALTER TABLE users DROP COLUMN ai_planning');
    }
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { up, down, VERSION };
