/**
 * Allow future-tense cells on lexicon form rows (ADR-0014 D5).
 * Migration 005's CHECK accepts present, past, and infinitive cells only.
 * Rebuilds lexicon_entries when that branch is missing; copies every row.
 * Provenance: INIT-012/SPEC-003
 */
'use strict';

const VERSION = 10;

const HEBREW_FIRST = 0x05D0;
const HEBREW_LAST = 0x05EA;
const ACCEPT_JSON_MAX = 505;

function hebrewLetterCheck(column) {
    const parts = [];
    for (let cp = HEBREW_FIRST; cp <= HEBREW_LAST; cp++) {
        parts.push('instr(' + column + ', char(' + cp + ')) > 0');
    }
    return '(' + parts.join(' OR ') + ')';
}

function plainTextCheck(column, maxLen, minLen) {
    const lengthSql = minLen == null
        ? 'length(' + column + ') <= ' + maxLen
        : 'length(' + column + ') BETWEEN ' + minLen + ' AND ' + maxLen;
    const parts = [
        lengthSql,
        column + " NOT LIKE '%<%'",
        column + " NOT LIKE '%>%'"
    ];
    for (let cp = 0; cp < 32; cp++) {
        parts.push('instr(' + column + ', char(' + cp + ')) = 0');
    }
    parts.push('instr(' + column + ', char(127)) = 0');
    return parts.join('\n                AND ');
}

function requiredPlain(column, maxLen, minLen) {
    return 'CHECK (\n                ' + plainTextCheck(column, maxLen, minLen == null ? null : minLen) + '\n            )';
}

function nullablePlain(column, maxLen, minLen) {
    return 'CHECK (' + column + ' IS NULL OR (\n                '
        + plainTextCheck(column, maxLen, minLen == null ? null : minLen)
        + '\n            ))';
}

function keyCheck(column, nullable) {
    const body = plainTextCheck(column, 60, 1)
        + '\n                AND ' + hebrewLetterCheck(column);
    if (nullable) {
        return 'CHECK (' + column + ' IS NULL OR (\n                ' + body + '\n            ))';
    }
    return 'CHECK (\n                ' + body + '\n            )';
}

function acceptJsonTrigger(event) {
    const points = [];
    for (let cp = 0x0591; cp <= 0x05C7; cp++) {
        points.push('instr(json_each.value, char(' + cp + ')) > 0');
    }
    const when = event === 'UPDATE' ? 'UPDATE OF accept_json' : 'INSERT';
    return `
        CREATE TRIGGER lexicon_entries_accept_${event.toLowerCase()}
        BEFORE ${when} ON lexicon_entries
        FOR EACH ROW
        BEGIN
            SELECT RAISE(ABORT, 'CHECK constraint failed: accept_json')
            WHERE json_valid(NEW.accept_json) = 0
                OR json_type(NEW.accept_json) != 'array'
                OR json_array_length(NEW.accept_json) > 8
                OR EXISTS (
                    SELECT 1 FROM json_each(
                        CASE
                            WHEN json_valid(NEW.accept_json) THEN NEW.accept_json
                            ELSE '[]'
                        END
                    )
                    WHERE json_valid(NEW.accept_json)
                        AND (
                            json_each.type != 'text'
                            OR length(json_each.value) < 1
                            OR length(json_each.value) > 60
                            OR ${points.join('\n                            OR ')}
                        )
                );
        END;
    `;
}

function cellCheckSql(allowFutureCells) {
    const futureBranch = allowFutureCells
        ? `
                    WHEN tense = 'future' AND cell IN (
                        '1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'
                    ) THEN 1`
        : '';
    return `
            CHECK (
                CASE
                    WHEN cell IS NULL THEN 1
                    WHEN tense = 'present' AND cell IN ('ms', 'fs', 'mp', 'fp') THEN 1
                    WHEN tense = 'past' AND cell IN (
                        '1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'
                    ) THEN 1${futureBranch}
                    WHEN tense = 'infinitive' AND cell = 'inf' THEN 1
                    ELSE 0
                END
            ),`;
}

function createLexiconEntriesSql(allowFutureCells) {
    if (allowFutureCells === undefined) allowFutureCells = true;
    return `
        CREATE TABLE lexicon_entries_new (
            key TEXT PRIMARY KEY,
            kind TEXT NOT NULL
                CHECK (kind IN ('lemma', 'form', 'phrase')),
            lemma_key TEXT,
            gloss_en TEXT,
            pos TEXT
                CHECK (pos IS NULL OR pos IN (
                    'noun', 'verb', 'adjective', 'adverb', 'preposition',
                    'pronoun', 'conjunction', 'particle', 'numeral', 'other'
                )),
            root TEXT,
            binyan TEXT
                CHECK (binyan IS NULL OR binyan IN (
                    'paal', 'nifal', 'piel', 'pual', 'hifil', 'hufal', 'hitpael'
                )),
            root_class TEXT
                CHECK (root_class IS NULL OR root_class IN (
                    'regular', 'lamed-he', 'peh-yod', 'peh-nun', 'ayin-vav', 'irregular'
                )),
            tense TEXT
                CHECK (tense IS NULL OR tense IN (
                    'infinitive', 'present', 'past', 'future', 'imperative'
                )),
            cell TEXT,
            translit TEXT,
            accept_json TEXT NOT NULL DEFAULT '[]',
            verified INTEGER NOT NULL DEFAULT 0
                CHECK (verified IN (0, 1)),
            verified_by TEXT,
            source TEXT NOT NULL
                CHECK (source IN ('seed', 'mcp', 'admin')),
            rev INTEGER NOT NULL DEFAULT 0
                CHECK (rev >= 0),
            updated_at TEXT NOT NULL,
            ${keyCheck('key', false)},
            ${keyCheck('lemma_key', true)},
            CHECK (
                (kind = 'form' AND lemma_key IS NOT NULL)
                OR (kind IN ('lemma', 'phrase') AND lemma_key IS NULL)
            ),
            CHECK (
                (kind = 'phrase' AND instr(key, ' ') > 0)
                OR (kind IN ('lemma', 'form') AND instr(key, ' ') = 0)
            ),
            ${nullablePlain('gloss_en', 120)},
            CHECK (root IS NULL OR (
                length(root) IN (3, 5, 7)
                AND unicode(substr(root, 1, 1)) BETWEEN ${HEBREW_FIRST} AND ${HEBREW_LAST}
                AND substr(root, 2, 1) = '.'
                AND unicode(substr(root, 3, 1)) BETWEEN ${HEBREW_FIRST} AND ${HEBREW_LAST}
                AND (length(root) < 5 OR (
                    substr(root, 4, 1) = '.'
                    AND unicode(substr(root, 5, 1)) BETWEEN ${HEBREW_FIRST} AND ${HEBREW_LAST}
                ))
                AND (length(root) < 7 OR (
                    substr(root, 6, 1) = '.'
                    AND unicode(substr(root, 7, 1)) BETWEEN ${HEBREW_FIRST} AND ${HEBREW_LAST}
                ))
            )),
            ${cellCheckSql(allowFutureCells)}
            ${nullablePlain('translit', 80)},
            ${requiredPlain('accept_json', ACCEPT_JSON_MAX, 2)},
            ${nullablePlain('verified_by', 128)},
            ${requiredPlain('updated_at', 40, 1)}
        );
    `;
}

const LEXICON_COLUMNS = [
    'key', 'kind', 'lemma_key', 'gloss_en', 'pos', 'root', 'binyan', 'root_class',
    'tense', 'cell', 'translit', 'accept_json', 'verified', 'verified_by',
    'source', 'rev', 'updated_at'
];

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    const row = db.prepare(
        "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'lexicon_entries'"
    ).get();
    if (!row || !row.sql) return;
    if (String(row.sql).includes("tense = 'future' AND cell IN") && !String(row.sql).includes("'hufil'")) return;

    db.exec(`
        DROP TRIGGER IF EXISTS lexicon_entries_accept_insert;
        DROP TRIGGER IF EXISTS lexicon_entries_accept_update;
    `);
    db.exec(createLexiconEntriesSql());
    db.exec(
        'INSERT INTO lexicon_entries_new (' + LEXICON_COLUMNS.join(', ') + ') '
        + 'SELECT ' + LEXICON_COLUMNS.join(', ') + ' FROM lexicon_entries'
    );
    db.exec(`
        DROP TABLE lexicon_entries;
        ALTER TABLE lexicon_entries_new RENAME TO lexicon_entries;
        CREATE INDEX idx_lexicon_entries_lemma_key ON lexicon_entries (lemma_key);
        CREATE INDEX idx_lexicon_entries_binyan ON lexicon_entries (binyan);
    `);
    db.exec(acceptJsonTrigger('INSERT') + acceptJsonTrigger('UPDATE'));
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    const future = db.prepare(
        "SELECT 1 AS ok FROM lexicon_entries WHERE tense = 'future' LIMIT 1"
    ).get();
    if (future) {
        throw new Error('Cannot reverse migration 010 while future-tense lexicon rows exist.');
    }
    const row = db.prepare(
        "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'lexicon_entries'"
    ).get();
    if (!row || !String(row.sql).includes("tense = 'future' AND cell IN")) {
        const migrations = db.prepare(
            `SELECT 1 AS ok FROM sqlite_master
             WHERE type = 'table' AND name = 'schema_migrations'`
        ).get();
        if (migrations) {
            db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
        }
        return;
    }

    db.exec(`
        DROP TRIGGER IF EXISTS lexicon_entries_accept_insert;
        DROP TRIGGER IF EXISTS lexicon_entries_accept_update;
    `);
    db.exec(createLexiconEntriesSql(false));
    db.exec(
        'INSERT INTO lexicon_entries_new (' + LEXICON_COLUMNS.join(', ') + ') '
        + 'SELECT ' + LEXICON_COLUMNS.join(', ') + ' FROM lexicon_entries'
    );
    db.exec(`
        DROP TABLE lexicon_entries;
        ALTER TABLE lexicon_entries_new RENAME TO lexicon_entries;
        CREATE INDEX idx_lexicon_entries_lemma_key ON lexicon_entries (lemma_key);
        CREATE INDEX idx_lexicon_entries_binyan ON lexicon_entries (binyan);
    `);
    db.exec(acceptJsonTrigger('INSERT') + acceptJsonTrigger('UPDATE'));
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { up, down, VERSION };
