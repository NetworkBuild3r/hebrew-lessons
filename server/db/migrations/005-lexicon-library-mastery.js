/**
 * Lexicon, grammar-concept registry, personal library, and concept mastery.
 * Content (lexicon_entries, grammar_concepts) is separate from learner rows
 * (user_words, concept_mastery) so a profile read cannot join them by accident.
 * lemma_key is not a foreign key: a batch may insert a lemma and its forms
 * in one transaction in either order (SPEC-005). No file IO.
 * Provenance: INIT-006/SPEC-003
 */
'use strict';

/**
 * Version recorded in schema_migrations. Must match this file's numeric prefix.
 * migrate() derives the same number from the filename.
 */
const VERSION = 5;

/** U+05D0 through U+05EA, the Hebrew letters ADR-0009 §6 requires in a key. */
const HEBREW_FIRST = 0x05D0;
const HEBREW_LAST = 0x05EA;

/**
 * Compact JSON ceiling for accept_json: 8 strings of 60 characters.
 * [" + 60 + "," repeated, + "]. 2 brackets + 16 quotes + 7 commas + 480.
 */
const ACCEPT_JSON_MAX = 505;

/**
 * @param {string} column
 * @returns {string}
 */
function hebrewLetterCheck(column) {
    const parts = [];
    for (let cp = HEBREW_FIRST; cp <= HEBREW_LAST; cp++) {
        parts.push('instr(' + column + ', char(' + cp + ')) > 0');
    }
    return '(' + parts.join(' OR ') + ')';
}

/**
 * Length, no angle brackets, no ASCII controls. ADR-0009 §6 text rules.
 * @param {string} column
 * @param {number} maxLen
 * @param {number|null} minLen
 * @returns {string}
 */
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

/**
 * @param {string} column
 * @param {number} maxLen
 * @param {number|null} [minLen]
 * @returns {string}
 */
function requiredPlain(column, maxLen, minLen) {
    return 'CHECK (\n                ' + plainTextCheck(column, maxLen, minLen == null ? null : minLen) + '\n            )';
}

/**
 * @param {string} column
 * @param {number} maxLen
 * @param {number|null} [minLen]
 * @returns {string}
 */
function nullablePlain(column, maxLen, minLen) {
    return 'CHECK (' + column + ' IS NULL OR (\n                '
        + plainTextCheck(column, maxLen, minLen == null ? null : minLen)
        + '\n            ))';
}

/**
 * Reject an accept_json value that is not a JSON array of at most 8
 * text spellings, each 1 to 60 characters, with no Hebrew points
 * (U+0591 through U+05C7). json_each is not allowed inside a CHECK,
 * and a '$[n]' path inside a CHECK is parsed as a parameter, so the
 * per-element rule lives in these triggers.
 * @param {'INSERT'|'UPDATE'} event
 * @returns {string}
 */
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

/**
 * exactKey shape: 1 to 60 characters and at least one Hebrew letter.
 * @param {string} column
 * @param {boolean} nullable
 * @returns {string}
 */
function keyCheck(column, nullable) {
    const body = plainTextCheck(column, 60, 1)
        + '\n                AND ' + hebrewLetterCheck(column);
    if (nullable) {
        return 'CHECK (' + column + ' IS NULL OR (\n                ' + body + '\n            ))';
    }
    return 'CHECK (\n                ' + body + '\n            )';
}

/**
 * ADR-0009 §4, in table order. Keys are family:name. 32 rows.
 * label_he is the pointed name, the Hebrew gloss, or the pronoun.
 * Present-person and infinitive cells have no pronoun, so label_he is null.
 * Colour is only the root-class traffic light.
 */
const CONCEPTS = [
    { key: 'binyan:paal', family: 'binyan', labelEn: 'paal', labelHe: 'פָּעַל', colour: null },
    { key: 'binyan:nifal', family: 'binyan', labelEn: 'nifal', labelHe: 'נִפְעַל', colour: null },
    { key: 'binyan:piel', family: 'binyan', labelEn: 'piel', labelHe: 'פִּעֵל', colour: null },
    { key: 'binyan:pual', family: 'binyan', labelEn: 'pual', labelHe: 'פֻּעַל', colour: null },
    { key: 'binyan:hifil', family: 'binyan', labelEn: 'hifil', labelHe: 'הִפְעִיל', colour: null },
    { key: 'binyan:hufal', family: 'binyan', labelEn: 'hufal', labelHe: 'הֻפְעַל', colour: null },
    { key: 'binyan:hitpael', family: 'binyan', labelEn: 'hitpael', labelHe: 'הִתְפַּעֵל', colour: null },
    { key: 'tense:infinitive', family: 'tense', labelEn: 'infinitive', labelHe: 'שם פועל', colour: null },
    { key: 'tense:present', family: 'tense', labelEn: 'present', labelHe: 'זמן הווה', colour: null },
    { key: 'tense:past', family: 'tense', labelEn: 'past', labelHe: 'זמן עבר', colour: null },
    { key: 'tense:future', family: 'tense', labelEn: 'future', labelHe: 'זמן עתיד', colour: null },
    { key: 'tense:imperative', family: 'tense', labelEn: 'imperative', labelHe: 'ציווי', colour: null },
    { key: 'root-class:regular', family: 'root-class', labelEn: 'regular', labelHe: 'שלם', colour: 'green' },
    { key: 'root-class:lamed-he', family: 'root-class', labelEn: 'lamed-he', labelHe: 'ל״ה', colour: 'yellow' },
    { key: 'root-class:peh-yod', family: 'root-class', labelEn: 'peh-yod', labelHe: 'פ״י', colour: 'red' },
    { key: 'root-class:peh-nun', family: 'root-class', labelEn: 'peh-nun', labelHe: 'פ״נ', colour: 'red' },
    { key: 'root-class:ayin-vav', family: 'root-class', labelEn: 'ayin-vav', labelHe: 'ע״ו', colour: 'red' },
    { key: 'root-class:irregular', family: 'root-class', labelEn: 'irregular', labelHe: 'חריג', colour: 'red' },
    { key: 'cell:present.ms', family: 'cell', labelEn: 'masculine singular', labelHe: null, colour: null },
    { key: 'cell:present.fs', family: 'cell', labelEn: 'feminine singular', labelHe: null, colour: null },
    { key: 'cell:present.mp', family: 'cell', labelEn: 'masculine plural', labelHe: null, colour: null },
    { key: 'cell:present.fp', family: 'cell', labelEn: 'feminine plural', labelHe: null, colour: null },
    { key: 'cell:past.1s', family: 'cell', labelEn: 'first person singular', labelHe: 'אני', colour: null },
    { key: 'cell:past.2ms', family: 'cell', labelEn: 'second person masculine singular', labelHe: 'אתה', colour: null },
    { key: 'cell:past.2fs', family: 'cell', labelEn: 'second person feminine singular', labelHe: 'את', colour: null },
    { key: 'cell:past.3ms', family: 'cell', labelEn: 'third person masculine singular', labelHe: 'הוא', colour: null },
    { key: 'cell:past.3fs', family: 'cell', labelEn: 'third person feminine singular', labelHe: 'היא', colour: null },
    { key: 'cell:past.1p', family: 'cell', labelEn: 'first person plural', labelHe: 'אנחנו', colour: null },
    { key: 'cell:past.2mp', family: 'cell', labelEn: 'second person masculine plural', labelHe: 'אתם', colour: null },
    { key: 'cell:past.2fp', family: 'cell', labelEn: 'second person feminine plural', labelHe: 'אתן', colour: null },
    { key: 'cell:past.3p', family: 'cell', labelEn: 'third person plural', labelHe: 'הם, הן', colour: null },
    { key: 'cell:infinitive.inf', family: 'cell', labelEn: 'infinitive', labelHe: null, colour: null }
];

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE lexicon_entries (
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
            CHECK (
                CASE
                    WHEN cell IS NULL THEN 1
                    WHEN tense = 'present' AND cell IN ('ms', 'fs', 'mp', 'fp') THEN 1
                    WHEN tense = 'past' AND cell IN (
                        '1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'
                    ) THEN 1
                    WHEN tense = 'infinitive' AND cell = 'inf' THEN 1
                    ELSE 0
                END
            ),
            ${nullablePlain('translit', 80)},
            ${requiredPlain('accept_json', ACCEPT_JSON_MAX, 2)},
            ${nullablePlain('verified_by', 128)},
            ${requiredPlain('updated_at', 40, 1)}
        );

        CREATE INDEX idx_lexicon_entries_lemma_key ON lexicon_entries (lemma_key);
        CREATE INDEX idx_lexicon_entries_binyan ON lexicon_entries (binyan);

        CREATE TABLE grammar_concepts (
            key TEXT PRIMARY KEY,
            family TEXT NOT NULL
                CHECK (family IN ('binyan', 'tense', 'cell', 'root-class')),
            label_en TEXT NOT NULL,
            label_he TEXT,
            colour TEXT,
            sort INTEGER NOT NULL,
            CHECK (substr(key, 1, length(family) + 1) = family || ':'),
            CHECK (
                (family = 'root-class' AND colour IN ('green', 'yellow', 'red'))
                OR (family <> 'root-class' AND colour IS NULL)
            ),
            ${requiredPlain('key', 64, 3)},
            ${requiredPlain('label_en', 80, 1)},
            ${nullablePlain('label_he', 80, 1)},
            CHECK (sort >= 0)
        );

        CREATE TABLE user_words (
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            source TEXT NOT NULL
                CHECK (source IN ('self', 'ai', 'lesson')),
            added_at TEXT NOT NULL,
            gloss_en TEXT,
            PRIMARY KEY (user_id, word),
            ${keyCheck('word', false)},
            ${requiredPlain('added_at', 40, 1)},
            ${nullablePlain('gloss_en', 100)}
        );

        CREATE TABLE concept_mastery (
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            concept_key TEXT NOT NULL
                REFERENCES grammar_concepts(key),
            attempts INTEGER NOT NULL DEFAULT 0,
            correct INTEGER NOT NULL DEFAULT 0,
            ewma REAL,
            updated_at TEXT NOT NULL,
            PRIMARY KEY (user_id, concept_key),
            CHECK (attempts >= 0),
            CHECK (correct >= 0 AND correct <= attempts),
            CHECK (ewma IS NULL OR (ewma >= 0 AND ewma <= 1)),
            ${requiredPlain('updated_at', 40, 1)}
        );

        CREATE INDEX idx_concept_mastery_user_id ON concept_mastery (user_id);
    `);

    db.exec(acceptJsonTrigger('INSERT') + acceptJsonTrigger('UPDATE'));

    const insertConcept = db.prepare(
        `INSERT INTO grammar_concepts (
            key, family, label_en, label_he, colour, sort
        ) VALUES (?, ?, ?, ?, ?, ?)`
    );
    CONCEPTS.forEach((concept, index) => {
        const labelHe = concept.labelHe == null ? null : concept.labelHe.normalize('NFC');
        insertConcept.run(
            concept.key,
            concept.family,
            concept.labelEn,
            labelHe,
            concept.colour,
            index + 1
        );
    });
}

/**
 * Drop only the four tables this migration created, children first,
 * and forget version 5 so migrate() can apply it again.
 * users, review_items, practice_events, and lesson_progress are not named.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    db.exec(`
        DROP TABLE IF EXISTS concept_mastery;
        DROP TABLE IF EXISTS user_words;
        DROP TABLE IF EXISTS lexicon_entries;
        DROP TABLE IF EXISTS grammar_concepts;
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
