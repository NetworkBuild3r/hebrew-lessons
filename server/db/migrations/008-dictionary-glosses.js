/**
 * Suggestions for the teacher word box. Rows are glosses from the
 * Wiktionary English–Hebrew dictionary (CC BY-SA 3.0), not checked
 * conjugations. lexicon_entries stays the source for verb tables.
 */
'use strict';

const VERSION = 8;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE dictionary_glosses (
            id INTEGER PRIMARY KEY,
            he TEXT NOT NULL,
            en TEXT NOT NULL,
            he_fold TEXT NOT NULL,
            en_fold TEXT NOT NULL,
            CHECK (length(he) BETWEEN 1 AND 240),
            CHECK (length(en) BETWEEN 1 AND 240),
            CHECK (instr(he, '<') = 0 AND instr(en, '<') = 0),
            CHECK (instr(he, '>') = 0 AND instr(en, '>') = 0),
            UNIQUE (he, en)
        );

        CREATE INDEX idx_dictionary_he_fold ON dictionary_glosses (he_fold);
        CREATE INDEX idx_dictionary_en_fold ON dictionary_glosses (en_fold);
    `);
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec('DROP TABLE IF EXISTS dictionary_glosses');
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { VERSION, up, down };
