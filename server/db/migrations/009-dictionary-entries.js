/**
 * Hebrew headwords with their English senses, for tap-a-word lookup.
 * Rows come from the Hebrew–English dictionary file kept under data/dictionary.
 * pos is the part of speech as the dictionary words it; en is the senses text.
 * lexicon_entries stays the source for checked verb tables.
 */
'use strict';

const VERSION = 9;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE dictionary_entries (
            id INTEGER PRIMARY KEY,
            he TEXT NOT NULL,
            he_fold TEXT NOT NULL,
            pos TEXT NOT NULL DEFAULT '',
            en TEXT NOT NULL,
            CHECK (length(he) BETWEEN 1 AND 240),
            CHECK (length(en) BETWEEN 1 AND 800),
            UNIQUE (he, pos, en)
        );

        CREATE INDEX idx_dictionary_entries_fold ON dictionary_entries (he_fold);
    `);
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec('DROP INDEX IF EXISTS idx_dictionary_entries_fold');
    db.exec('DROP TABLE IF EXISTS dictionary_entries');
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { VERSION, up, down };
