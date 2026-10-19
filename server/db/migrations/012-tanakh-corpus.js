/**
 * Tanakh corpus: one row per verse, Hebrew text paired with public-domain
 * English (JPS 1917). Rows come from data/tanakh, fetched from the Sefaria
 * API with versions pinned to Public Domain editions (scripts/fetch-tanakh.js).
 * tanakh_meta records provenance (source, version titles, licenses, fetch
 * date) so the app can always say where the text came from.
 */
'use strict';

const VERSION = 12;

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE tanakh_verses (
            id INTEGER PRIMARY KEY,
            book TEXT NOT NULL,
            chapter INTEGER NOT NULL,
            verse INTEGER NOT NULL,
            he TEXT NOT NULL DEFAULT '',
            en TEXT NOT NULL DEFAULT '',
            CHECK (length(book) BETWEEN 1 AND 40),
            CHECK (chapter >= 1),
            CHECK (verse >= 1),
            CHECK (length(he) <= 2000),
            CHECK (length(en) <= 2000),
            UNIQUE (book, chapter, verse)
        );

        CREATE INDEX idx_tanakh_verses_book_chapter ON tanakh_verses (book, chapter);

        CREATE TABLE tanakh_meta (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL,
            CHECK (length(key) BETWEEN 1 AND 80),
            CHECK (length(value) <= 4000)
        );
    `);
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec('DROP INDEX IF EXISTS idx_tanakh_verses_book_chapter');
    db.exec('DROP TABLE IF EXISTS tanakh_verses');
    db.exec('DROP TABLE IF EXISTS tanakh_meta');
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { VERSION, up, down };
