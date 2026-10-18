journal_mode: wal
user_version: 0

# schema_migrations
1 001-initial.js <timestamp>
2 002-plan-store.js <timestamp>
3 003-media-and-ai-planning.js <timestamp>
4 004-fsrs-review-state.js <timestamp>
5 005-lexicon-library-mastery.js <timestamp>
6 006-plan-revisions.js <timestamp>
7 007-plan-revision-lesson-ops.js <timestamp>
8 008-dictionary-glosses.js <timestamp>
9 009-dictionary-entries.js <timestamp>
10 010-lexicon-future-cells.js <timestamp>
11 011-google-email.js <timestamp>

# sqlite_master
--- table audit_log
CREATE TABLE audit_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            actor TEXT NOT NULL,
            action TEXT NOT NULL,
            target TEXT,
            at TEXT NOT NULL,
            detail TEXT
        )
--- table concept_mastery
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
            CHECK (
                length(updated_at) BETWEEN 1 AND 40
                AND updated_at NOT LIKE '%<%'
                AND updated_at NOT LIKE '%>%'
                AND instr(updated_at, char(0)) = 0
                AND instr(updated_at, char(1)) = 0
                AND instr(updated_at, char(2)) = 0
                AND instr(updated_at, char(3)) = 0
                AND instr(updated_at, char(4)) = 0
                AND instr(updated_at, char(5)) = 0
                AND instr(updated_at, char(6)) = 0
                AND instr(updated_at, char(7)) = 0
                AND instr(updated_at, char(8)) = 0
                AND instr(updated_at, char(9)) = 0
                AND instr(updated_at, char(10)) = 0
                AND instr(updated_at, char(11)) = 0
                AND instr(updated_at, char(12)) = 0
                AND instr(updated_at, char(13)) = 0
                AND instr(updated_at, char(14)) = 0
                AND instr(updated_at, char(15)) = 0
                AND instr(updated_at, char(16)) = 0
                AND instr(updated_at, char(17)) = 0
                AND instr(updated_at, char(18)) = 0
                AND instr(updated_at, char(19)) = 0
                AND instr(updated_at, char(20)) = 0
                AND instr(updated_at, char(21)) = 0
                AND instr(updated_at, char(22)) = 0
                AND instr(updated_at, char(23)) = 0
                AND instr(updated_at, char(24)) = 0
                AND instr(updated_at, char(25)) = 0
                AND instr(updated_at, char(26)) = 0
                AND instr(updated_at, char(27)) = 0
                AND instr(updated_at, char(28)) = 0
                AND instr(updated_at, char(29)) = 0
                AND instr(updated_at, char(30)) = 0
                AND instr(updated_at, char(31)) = 0
                AND instr(updated_at, char(127)) = 0
            )
        )
--- table dictionary_entries
CREATE TABLE dictionary_entries (
            id INTEGER PRIMARY KEY,
            he TEXT NOT NULL,
            he_fold TEXT NOT NULL,
            pos TEXT NOT NULL DEFAULT '',
            en TEXT NOT NULL,
            CHECK (length(he) BETWEEN 1 AND 240),
            CHECK (length(en) BETWEEN 1 AND 800),
            UNIQUE (he, pos, en)
        )
--- table dictionary_glosses
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
        )
--- table grammar_concepts
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
            CHECK (
                length(key) BETWEEN 3 AND 64
                AND key NOT LIKE '%<%'
                AND key NOT LIKE '%>%'
                AND instr(key, char(0)) = 0
                AND instr(key, char(1)) = 0
                AND instr(key, char(2)) = 0
                AND instr(key, char(3)) = 0
                AND instr(key, char(4)) = 0
                AND instr(key, char(5)) = 0
                AND instr(key, char(6)) = 0
                AND instr(key, char(7)) = 0
                AND instr(key, char(8)) = 0
                AND instr(key, char(9)) = 0
                AND instr(key, char(10)) = 0
                AND instr(key, char(11)) = 0
                AND instr(key, char(12)) = 0
                AND instr(key, char(13)) = 0
                AND instr(key, char(14)) = 0
                AND instr(key, char(15)) = 0
                AND instr(key, char(16)) = 0
                AND instr(key, char(17)) = 0
                AND instr(key, char(18)) = 0
                AND instr(key, char(19)) = 0
                AND instr(key, char(20)) = 0
                AND instr(key, char(21)) = 0
                AND instr(key, char(22)) = 0
                AND instr(key, char(23)) = 0
                AND instr(key, char(24)) = 0
                AND instr(key, char(25)) = 0
                AND instr(key, char(26)) = 0
                AND instr(key, char(27)) = 0
                AND instr(key, char(28)) = 0
                AND instr(key, char(29)) = 0
                AND instr(key, char(30)) = 0
                AND instr(key, char(31)) = 0
                AND instr(key, char(127)) = 0
            ),
            CHECK (
                length(label_en) BETWEEN 1 AND 80
                AND label_en NOT LIKE '%<%'
                AND label_en NOT LIKE '%>%'
                AND instr(label_en, char(0)) = 0
                AND instr(label_en, char(1)) = 0
                AND instr(label_en, char(2)) = 0
                AND instr(label_en, char(3)) = 0
                AND instr(label_en, char(4)) = 0
                AND instr(label_en, char(5)) = 0
                AND instr(label_en, char(6)) = 0
                AND instr(label_en, char(7)) = 0
                AND instr(label_en, char(8)) = 0
                AND instr(label_en, char(9)) = 0
                AND instr(label_en, char(10)) = 0
                AND instr(label_en, char(11)) = 0
                AND instr(label_en, char(12)) = 0
                AND instr(label_en, char(13)) = 0
                AND instr(label_en, char(14)) = 0
                AND instr(label_en, char(15)) = 0
                AND instr(label_en, char(16)) = 0
                AND instr(label_en, char(17)) = 0
                AND instr(label_en, char(18)) = 0
                AND instr(label_en, char(19)) = 0
                AND instr(label_en, char(20)) = 0
                AND instr(label_en, char(21)) = 0
                AND instr(label_en, char(22)) = 0
                AND instr(label_en, char(23)) = 0
                AND instr(label_en, char(24)) = 0
                AND instr(label_en, char(25)) = 0
                AND instr(label_en, char(26)) = 0
                AND instr(label_en, char(27)) = 0
                AND instr(label_en, char(28)) = 0
                AND instr(label_en, char(29)) = 0
                AND instr(label_en, char(30)) = 0
                AND instr(label_en, char(31)) = 0
                AND instr(label_en, char(127)) = 0
            ),
            CHECK (label_he IS NULL OR (
                length(label_he) BETWEEN 1 AND 80
                AND label_he NOT LIKE '%<%'
                AND label_he NOT LIKE '%>%'
                AND instr(label_he, char(0)) = 0
                AND instr(label_he, char(1)) = 0
                AND instr(label_he, char(2)) = 0
                AND instr(label_he, char(3)) = 0
                AND instr(label_he, char(4)) = 0
                AND instr(label_he, char(5)) = 0
                AND instr(label_he, char(6)) = 0
                AND instr(label_he, char(7)) = 0
                AND instr(label_he, char(8)) = 0
                AND instr(label_he, char(9)) = 0
                AND instr(label_he, char(10)) = 0
                AND instr(label_he, char(11)) = 0
                AND instr(label_he, char(12)) = 0
                AND instr(label_he, char(13)) = 0
                AND instr(label_he, char(14)) = 0
                AND instr(label_he, char(15)) = 0
                AND instr(label_he, char(16)) = 0
                AND instr(label_he, char(17)) = 0
                AND instr(label_he, char(18)) = 0
                AND instr(label_he, char(19)) = 0
                AND instr(label_he, char(20)) = 0
                AND instr(label_he, char(21)) = 0
                AND instr(label_he, char(22)) = 0
                AND instr(label_he, char(23)) = 0
                AND instr(label_he, char(24)) = 0
                AND instr(label_he, char(25)) = 0
                AND instr(label_he, char(26)) = 0
                AND instr(label_he, char(27)) = 0
                AND instr(label_he, char(28)) = 0
                AND instr(label_he, char(29)) = 0
                AND instr(label_he, char(30)) = 0
                AND instr(label_he, char(31)) = 0
                AND instr(label_he, char(127)) = 0
            )),
            CHECK (sort >= 0)
        )
--- table lesson_grants
CREATE TABLE lesson_grants (
            slug TEXT NOT NULL,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            PRIMARY KEY (slug, user_id)
        )
--- table lesson_progress
CREATE TABLE lesson_progress (
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            slug TEXT NOT NULL,
            state_json TEXT NOT NULL,
            rev INTEGER NOT NULL DEFAULT 0,
            updated_at TEXT NOT NULL,
            PRIMARY KEY (user_id, slug),
            CHECK (length(state_json) <= 65536)
        )
--- table lesson_visibility
CREATE TABLE lesson_visibility (
            slug TEXT PRIMARY KEY,
            mode TEXT NOT NULL
                CHECK (mode IN ('hidden', 'everyone', 'selected')),
            updated_at TEXT NOT NULL
        )
--- table lexicon_entries
CREATE TABLE "lexicon_entries" (
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
            CHECK (
                length(key) BETWEEN 1 AND 60
                AND key NOT LIKE '%<%'
                AND key NOT LIKE '%>%'
                AND instr(key, char(0)) = 0
                AND instr(key, char(1)) = 0
                AND instr(key, char(2)) = 0
                AND instr(key, char(3)) = 0
                AND instr(key, char(4)) = 0
                AND instr(key, char(5)) = 0
                AND instr(key, char(6)) = 0
                AND instr(key, char(7)) = 0
                AND instr(key, char(8)) = 0
                AND instr(key, char(9)) = 0
                AND instr(key, char(10)) = 0
                AND instr(key, char(11)) = 0
                AND instr(key, char(12)) = 0
                AND instr(key, char(13)) = 0
                AND instr(key, char(14)) = 0
                AND instr(key, char(15)) = 0
                AND instr(key, char(16)) = 0
                AND instr(key, char(17)) = 0
                AND instr(key, char(18)) = 0
                AND instr(key, char(19)) = 0
                AND instr(key, char(20)) = 0
                AND instr(key, char(21)) = 0
                AND instr(key, char(22)) = 0
                AND instr(key, char(23)) = 0
                AND instr(key, char(24)) = 0
                AND instr(key, char(25)) = 0
                AND instr(key, char(26)) = 0
                AND instr(key, char(27)) = 0
                AND instr(key, char(28)) = 0
                AND instr(key, char(29)) = 0
                AND instr(key, char(30)) = 0
                AND instr(key, char(31)) = 0
                AND instr(key, char(127)) = 0
                AND (instr(key, char(1488)) > 0 OR instr(key, char(1489)) > 0 OR instr(key, char(1490)) > 0 OR instr(key, char(1491)) > 0 OR instr(key, char(1492)) > 0 OR instr(key, char(1493)) > 0 OR instr(key, char(1494)) > 0 OR instr(key, char(1495)) > 0 OR instr(key, char(1496)) > 0 OR instr(key, char(1497)) > 0 OR instr(key, char(1498)) > 0 OR instr(key, char(1499)) > 0 OR instr(key, char(1500)) > 0 OR instr(key, char(1501)) > 0 OR instr(key, char(1502)) > 0 OR instr(key, char(1503)) > 0 OR instr(key, char(1504)) > 0 OR instr(key, char(1505)) > 0 OR instr(key, char(1506)) > 0 OR instr(key, char(1507)) > 0 OR instr(key, char(1508)) > 0 OR instr(key, char(1509)) > 0 OR instr(key, char(1510)) > 0 OR instr(key, char(1511)) > 0 OR instr(key, char(1512)) > 0 OR instr(key, char(1513)) > 0 OR instr(key, char(1514)) > 0)
            ),
            CHECK (lemma_key IS NULL OR (
                length(lemma_key) BETWEEN 1 AND 60
                AND lemma_key NOT LIKE '%<%'
                AND lemma_key NOT LIKE '%>%'
                AND instr(lemma_key, char(0)) = 0
                AND instr(lemma_key, char(1)) = 0
                AND instr(lemma_key, char(2)) = 0
                AND instr(lemma_key, char(3)) = 0
                AND instr(lemma_key, char(4)) = 0
                AND instr(lemma_key, char(5)) = 0
                AND instr(lemma_key, char(6)) = 0
                AND instr(lemma_key, char(7)) = 0
                AND instr(lemma_key, char(8)) = 0
                AND instr(lemma_key, char(9)) = 0
                AND instr(lemma_key, char(10)) = 0
                AND instr(lemma_key, char(11)) = 0
                AND instr(lemma_key, char(12)) = 0
                AND instr(lemma_key, char(13)) = 0
                AND instr(lemma_key, char(14)) = 0
                AND instr(lemma_key, char(15)) = 0
                AND instr(lemma_key, char(16)) = 0
                AND instr(lemma_key, char(17)) = 0
                AND instr(lemma_key, char(18)) = 0
                AND instr(lemma_key, char(19)) = 0
                AND instr(lemma_key, char(20)) = 0
                AND instr(lemma_key, char(21)) = 0
                AND instr(lemma_key, char(22)) = 0
                AND instr(lemma_key, char(23)) = 0
                AND instr(lemma_key, char(24)) = 0
                AND instr(lemma_key, char(25)) = 0
                AND instr(lemma_key, char(26)) = 0
                AND instr(lemma_key, char(27)) = 0
                AND instr(lemma_key, char(28)) = 0
                AND instr(lemma_key, char(29)) = 0
                AND instr(lemma_key, char(30)) = 0
                AND instr(lemma_key, char(31)) = 0
                AND instr(lemma_key, char(127)) = 0
                AND (instr(lemma_key, char(1488)) > 0 OR instr(lemma_key, char(1489)) > 0 OR instr(lemma_key, char(1490)) > 0 OR instr(lemma_key, char(1491)) > 0 OR instr(lemma_key, char(1492)) > 0 OR instr(lemma_key, char(1493)) > 0 OR instr(lemma_key, char(1494)) > 0 OR instr(lemma_key, char(1495)) > 0 OR instr(lemma_key, char(1496)) > 0 OR instr(lemma_key, char(1497)) > 0 OR instr(lemma_key, char(1498)) > 0 OR instr(lemma_key, char(1499)) > 0 OR instr(lemma_key, char(1500)) > 0 OR instr(lemma_key, char(1501)) > 0 OR instr(lemma_key, char(1502)) > 0 OR instr(lemma_key, char(1503)) > 0 OR instr(lemma_key, char(1504)) > 0 OR instr(lemma_key, char(1505)) > 0 OR instr(lemma_key, char(1506)) > 0 OR instr(lemma_key, char(1507)) > 0 OR instr(lemma_key, char(1508)) > 0 OR instr(lemma_key, char(1509)) > 0 OR instr(lemma_key, char(1510)) > 0 OR instr(lemma_key, char(1511)) > 0 OR instr(lemma_key, char(1512)) > 0 OR instr(lemma_key, char(1513)) > 0 OR instr(lemma_key, char(1514)) > 0)
            )),
            CHECK (
                (kind = 'form' AND lemma_key IS NOT NULL)
                OR (kind IN ('lemma', 'phrase') AND lemma_key IS NULL)
            ),
            CHECK (
                (kind = 'phrase' AND instr(key, ' ') > 0)
                OR (kind IN ('lemma', 'form') AND instr(key, ' ') = 0)
            ),
            CHECK (gloss_en IS NULL OR (
                length(gloss_en) <= 120
                AND gloss_en NOT LIKE '%<%'
                AND gloss_en NOT LIKE '%>%'
                AND instr(gloss_en, char(0)) = 0
                AND instr(gloss_en, char(1)) = 0
                AND instr(gloss_en, char(2)) = 0
                AND instr(gloss_en, char(3)) = 0
                AND instr(gloss_en, char(4)) = 0
                AND instr(gloss_en, char(5)) = 0
                AND instr(gloss_en, char(6)) = 0
                AND instr(gloss_en, char(7)) = 0
                AND instr(gloss_en, char(8)) = 0
                AND instr(gloss_en, char(9)) = 0
                AND instr(gloss_en, char(10)) = 0
                AND instr(gloss_en, char(11)) = 0
                AND instr(gloss_en, char(12)) = 0
                AND instr(gloss_en, char(13)) = 0
                AND instr(gloss_en, char(14)) = 0
                AND instr(gloss_en, char(15)) = 0
                AND instr(gloss_en, char(16)) = 0
                AND instr(gloss_en, char(17)) = 0
                AND instr(gloss_en, char(18)) = 0
                AND instr(gloss_en, char(19)) = 0
                AND instr(gloss_en, char(20)) = 0
                AND instr(gloss_en, char(21)) = 0
                AND instr(gloss_en, char(22)) = 0
                AND instr(gloss_en, char(23)) = 0
                AND instr(gloss_en, char(24)) = 0
                AND instr(gloss_en, char(25)) = 0
                AND instr(gloss_en, char(26)) = 0
                AND instr(gloss_en, char(27)) = 0
                AND instr(gloss_en, char(28)) = 0
                AND instr(gloss_en, char(29)) = 0
                AND instr(gloss_en, char(30)) = 0
                AND instr(gloss_en, char(31)) = 0
                AND instr(gloss_en, char(127)) = 0
            )),
            CHECK (root IS NULL OR (
                length(root) IN (3, 5, 7)
                AND unicode(substr(root, 1, 1)) BETWEEN 1488 AND 1514
                AND substr(root, 2, 1) = '.'
                AND unicode(substr(root, 3, 1)) BETWEEN 1488 AND 1514
                AND (length(root) < 5 OR (
                    substr(root, 4, 1) = '.'
                    AND unicode(substr(root, 5, 1)) BETWEEN 1488 AND 1514
                ))
                AND (length(root) < 7 OR (
                    substr(root, 6, 1) = '.'
                    AND unicode(substr(root, 7, 1)) BETWEEN 1488 AND 1514
                ))
            )),

            CHECK (
                CASE
                    WHEN cell IS NULL THEN 1
                    WHEN tense = 'present' AND cell IN ('ms', 'fs', 'mp', 'fp') THEN 1
                    WHEN tense = 'past' AND cell IN (
                        '1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'
                    ) THEN 1
                    WHEN tense = 'future' AND cell IN (
                        '1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'
                    ) THEN 1
                    WHEN tense = 'infinitive' AND cell = 'inf' THEN 1
                    ELSE 0
                END
            ),
            CHECK (translit IS NULL OR (
                length(translit) <= 80
                AND translit NOT LIKE '%<%'
                AND translit NOT LIKE '%>%'
                AND instr(translit, char(0)) = 0
                AND instr(translit, char(1)) = 0
                AND instr(translit, char(2)) = 0
                AND instr(translit, char(3)) = 0
                AND instr(translit, char(4)) = 0
                AND instr(translit, char(5)) = 0
                AND instr(translit, char(6)) = 0
                AND instr(translit, char(7)) = 0
                AND instr(translit, char(8)) = 0
                AND instr(translit, char(9)) = 0
                AND instr(translit, char(10)) = 0
                AND instr(translit, char(11)) = 0
                AND instr(translit, char(12)) = 0
                AND instr(translit, char(13)) = 0
                AND instr(translit, char(14)) = 0
                AND instr(translit, char(15)) = 0
                AND instr(translit, char(16)) = 0
                AND instr(translit, char(17)) = 0
                AND instr(translit, char(18)) = 0
                AND instr(translit, char(19)) = 0
                AND instr(translit, char(20)) = 0
                AND instr(translit, char(21)) = 0
                AND instr(translit, char(22)) = 0
                AND instr(translit, char(23)) = 0
                AND instr(translit, char(24)) = 0
                AND instr(translit, char(25)) = 0
                AND instr(translit, char(26)) = 0
                AND instr(translit, char(27)) = 0
                AND instr(translit, char(28)) = 0
                AND instr(translit, char(29)) = 0
                AND instr(translit, char(30)) = 0
                AND instr(translit, char(31)) = 0
                AND instr(translit, char(127)) = 0
            )),
            CHECK (
                length(accept_json) BETWEEN 2 AND 505
                AND accept_json NOT LIKE '%<%'
                AND accept_json NOT LIKE '%>%'
                AND instr(accept_json, char(0)) = 0
                AND instr(accept_json, char(1)) = 0
                AND instr(accept_json, char(2)) = 0
                AND instr(accept_json, char(3)) = 0
                AND instr(accept_json, char(4)) = 0
                AND instr(accept_json, char(5)) = 0
                AND instr(accept_json, char(6)) = 0
                AND instr(accept_json, char(7)) = 0
                AND instr(accept_json, char(8)) = 0
                AND instr(accept_json, char(9)) = 0
                AND instr(accept_json, char(10)) = 0
                AND instr(accept_json, char(11)) = 0
                AND instr(accept_json, char(12)) = 0
                AND instr(accept_json, char(13)) = 0
                AND instr(accept_json, char(14)) = 0
                AND instr(accept_json, char(15)) = 0
                AND instr(accept_json, char(16)) = 0
                AND instr(accept_json, char(17)) = 0
                AND instr(accept_json, char(18)) = 0
                AND instr(accept_json, char(19)) = 0
                AND instr(accept_json, char(20)) = 0
                AND instr(accept_json, char(21)) = 0
                AND instr(accept_json, char(22)) = 0
                AND instr(accept_json, char(23)) = 0
                AND instr(accept_json, char(24)) = 0
                AND instr(accept_json, char(25)) = 0
                AND instr(accept_json, char(26)) = 0
                AND instr(accept_json, char(27)) = 0
                AND instr(accept_json, char(28)) = 0
                AND instr(accept_json, char(29)) = 0
                AND instr(accept_json, char(30)) = 0
                AND instr(accept_json, char(31)) = 0
                AND instr(accept_json, char(127)) = 0
            ),
            CHECK (verified_by IS NULL OR (
                length(verified_by) <= 128
                AND verified_by NOT LIKE '%<%'
                AND verified_by NOT LIKE '%>%'
                AND instr(verified_by, char(0)) = 0
                AND instr(verified_by, char(1)) = 0
                AND instr(verified_by, char(2)) = 0
                AND instr(verified_by, char(3)) = 0
                AND instr(verified_by, char(4)) = 0
                AND instr(verified_by, char(5)) = 0
                AND instr(verified_by, char(6)) = 0
                AND instr(verified_by, char(7)) = 0
                AND instr(verified_by, char(8)) = 0
                AND instr(verified_by, char(9)) = 0
                AND instr(verified_by, char(10)) = 0
                AND instr(verified_by, char(11)) = 0
                AND instr(verified_by, char(12)) = 0
                AND instr(verified_by, char(13)) = 0
                AND instr(verified_by, char(14)) = 0
                AND instr(verified_by, char(15)) = 0
                AND instr(verified_by, char(16)) = 0
                AND instr(verified_by, char(17)) = 0
                AND instr(verified_by, char(18)) = 0
                AND instr(verified_by, char(19)) = 0
                AND instr(verified_by, char(20)) = 0
                AND instr(verified_by, char(21)) = 0
                AND instr(verified_by, char(22)) = 0
                AND instr(verified_by, char(23)) = 0
                AND instr(verified_by, char(24)) = 0
                AND instr(verified_by, char(25)) = 0
                AND instr(verified_by, char(26)) = 0
                AND instr(verified_by, char(27)) = 0
                AND instr(verified_by, char(28)) = 0
                AND instr(verified_by, char(29)) = 0
                AND instr(verified_by, char(30)) = 0
                AND instr(verified_by, char(31)) = 0
                AND instr(verified_by, char(127)) = 0
            )),
            CHECK (
                length(updated_at) BETWEEN 1 AND 40
                AND updated_at NOT LIKE '%<%'
                AND updated_at NOT LIKE '%>%'
                AND instr(updated_at, char(0)) = 0
                AND instr(updated_at, char(1)) = 0
                AND instr(updated_at, char(2)) = 0
                AND instr(updated_at, char(3)) = 0
                AND instr(updated_at, char(4)) = 0
                AND instr(updated_at, char(5)) = 0
                AND instr(updated_at, char(6)) = 0
                AND instr(updated_at, char(7)) = 0
                AND instr(updated_at, char(8)) = 0
                AND instr(updated_at, char(9)) = 0
                AND instr(updated_at, char(10)) = 0
                AND instr(updated_at, char(11)) = 0
                AND instr(updated_at, char(12)) = 0
                AND instr(updated_at, char(13)) = 0
                AND instr(updated_at, char(14)) = 0
                AND instr(updated_at, char(15)) = 0
                AND instr(updated_at, char(16)) = 0
                AND instr(updated_at, char(17)) = 0
                AND instr(updated_at, char(18)) = 0
                AND instr(updated_at, char(19)) = 0
                AND instr(updated_at, char(20)) = 0
                AND instr(updated_at, char(21)) = 0
                AND instr(updated_at, char(22)) = 0
                AND instr(updated_at, char(23)) = 0
                AND instr(updated_at, char(24)) = 0
                AND instr(updated_at, char(25)) = 0
                AND instr(updated_at, char(26)) = 0
                AND instr(updated_at, char(27)) = 0
                AND instr(updated_at, char(28)) = 0
                AND instr(updated_at, char(29)) = 0
                AND instr(updated_at, char(30)) = 0
                AND instr(updated_at, char(31)) = 0
                AND instr(updated_at, char(127)) = 0
            )
        )
--- table media
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
        )
--- table meta
CREATE TABLE meta (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL
        )
--- table plan_field_marks
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
                    OR length(CAST(previous_value_json AS BLOB)) <= 32768
                ),
            previous_writer_kind TEXT
                CHECK (
                    previous_writer_kind IS NULL
                    OR previous_writer_kind IN ('ai', 'human', 'system', 'unknown')
                ),
            PRIMARY KEY (plan_id, path, field)
        )
--- table plan_revisions
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
                CHECK (length(summary) <= 500),
            document_gz BLOB NOT NULL,
            document_sha256 TEXT NOT NULL
                CHECK (length(document_sha256) = 64),
            created_at TEXT NOT NULL
                CHECK (length(created_at) BETWEEN 1 AND 40),
            PRIMARY KEY (plan_id, revision)
        )
--- table plans
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
        )
--- table practice_events
CREATE TABLE practice_events (
            id TEXT NOT NULL,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            direction TEXT NOT NULL
                CHECK (direction IN ('he_en', 'en_he')),
            result TEXT NOT NULL,
            at TEXT NOT NULL,
            slug TEXT,
            activity TEXT, grade INTEGER
                CHECK (grade BETWEEN 1 AND 4), kind TEXT NOT NULL DEFAULT 'word'
                CHECK (kind IN ('word', 'form')), lemma TEXT,
            PRIMARY KEY (user_id, id)
        )
--- table review_items
CREATE TABLE review_items (
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
            sources TEXT NOT NULL DEFAULT '[]', stability REAL, difficulty REAL, reps INTEGER NOT NULL DEFAULT 0, fsrs_state TEXT
                CHECK (fsrs_state IN ('new', 'learning', 'review', 'relearning')), last_grade INTEGER
                CHECK (last_grade BETWEEN 1 AND 4), suspended INTEGER NOT NULL DEFAULT 0
                CHECK (suspended IN (0, 1)),
            UNIQUE (user_id, word, direction)
        )
--- table schema_migrations
CREATE TABLE schema_migrations (
            version INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            applied_at TEXT NOT NULL
        )
--- table sessions
CREATE TABLE sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            token_hash TEXT NOT NULL UNIQUE,
            created_at TEXT NOT NULL,
            last_seen_at TEXT NOT NULL,
            expires_at TEXT NOT NULL
        )
--- table sqlite_sequence
CREATE TABLE sqlite_sequence(name,seq)
--- table user_words
CREATE TABLE user_words (
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            source TEXT NOT NULL
                CHECK (source IN ('self', 'ai', 'lesson')),
            added_at TEXT NOT NULL,
            gloss_en TEXT,
            PRIMARY KEY (user_id, word),
            CHECK (
                length(word) BETWEEN 1 AND 60
                AND word NOT LIKE '%<%'
                AND word NOT LIKE '%>%'
                AND instr(word, char(0)) = 0
                AND instr(word, char(1)) = 0
                AND instr(word, char(2)) = 0
                AND instr(word, char(3)) = 0
                AND instr(word, char(4)) = 0
                AND instr(word, char(5)) = 0
                AND instr(word, char(6)) = 0
                AND instr(word, char(7)) = 0
                AND instr(word, char(8)) = 0
                AND instr(word, char(9)) = 0
                AND instr(word, char(10)) = 0
                AND instr(word, char(11)) = 0
                AND instr(word, char(12)) = 0
                AND instr(word, char(13)) = 0
                AND instr(word, char(14)) = 0
                AND instr(word, char(15)) = 0
                AND instr(word, char(16)) = 0
                AND instr(word, char(17)) = 0
                AND instr(word, char(18)) = 0
                AND instr(word, char(19)) = 0
                AND instr(word, char(20)) = 0
                AND instr(word, char(21)) = 0
                AND instr(word, char(22)) = 0
                AND instr(word, char(23)) = 0
                AND instr(word, char(24)) = 0
                AND instr(word, char(25)) = 0
                AND instr(word, char(26)) = 0
                AND instr(word, char(27)) = 0
                AND instr(word, char(28)) = 0
                AND instr(word, char(29)) = 0
                AND instr(word, char(30)) = 0
                AND instr(word, char(31)) = 0
                AND instr(word, char(127)) = 0
                AND (instr(word, char(1488)) > 0 OR instr(word, char(1489)) > 0 OR instr(word, char(1490)) > 0 OR instr(word, char(1491)) > 0 OR instr(word, char(1492)) > 0 OR instr(word, char(1493)) > 0 OR instr(word, char(1494)) > 0 OR instr(word, char(1495)) > 0 OR instr(word, char(1496)) > 0 OR instr(word, char(1497)) > 0 OR instr(word, char(1498)) > 0 OR instr(word, char(1499)) > 0 OR instr(word, char(1500)) > 0 OR instr(word, char(1501)) > 0 OR instr(word, char(1502)) > 0 OR instr(word, char(1503)) > 0 OR instr(word, char(1504)) > 0 OR instr(word, char(1505)) > 0 OR instr(word, char(1506)) > 0 OR instr(word, char(1507)) > 0 OR instr(word, char(1508)) > 0 OR instr(word, char(1509)) > 0 OR instr(word, char(1510)) > 0 OR instr(word, char(1511)) > 0 OR instr(word, char(1512)) > 0 OR instr(word, char(1513)) > 0 OR instr(word, char(1514)) > 0)
            ),
            CHECK (
                length(added_at) BETWEEN 1 AND 40
                AND added_at NOT LIKE '%<%'
                AND added_at NOT LIKE '%>%'
                AND instr(added_at, char(0)) = 0
                AND instr(added_at, char(1)) = 0
                AND instr(added_at, char(2)) = 0
                AND instr(added_at, char(3)) = 0
                AND instr(added_at, char(4)) = 0
                AND instr(added_at, char(5)) = 0
                AND instr(added_at, char(6)) = 0
                AND instr(added_at, char(7)) = 0
                AND instr(added_at, char(8)) = 0
                AND instr(added_at, char(9)) = 0
                AND instr(added_at, char(10)) = 0
                AND instr(added_at, char(11)) = 0
                AND instr(added_at, char(12)) = 0
                AND instr(added_at, char(13)) = 0
                AND instr(added_at, char(14)) = 0
                AND instr(added_at, char(15)) = 0
                AND instr(added_at, char(16)) = 0
                AND instr(added_at, char(17)) = 0
                AND instr(added_at, char(18)) = 0
                AND instr(added_at, char(19)) = 0
                AND instr(added_at, char(20)) = 0
                AND instr(added_at, char(21)) = 0
                AND instr(added_at, char(22)) = 0
                AND instr(added_at, char(23)) = 0
                AND instr(added_at, char(24)) = 0
                AND instr(added_at, char(25)) = 0
                AND instr(added_at, char(26)) = 0
                AND instr(added_at, char(27)) = 0
                AND instr(added_at, char(28)) = 0
                AND instr(added_at, char(29)) = 0
                AND instr(added_at, char(30)) = 0
                AND instr(added_at, char(31)) = 0
                AND instr(added_at, char(127)) = 0
            ),
            CHECK (gloss_en IS NULL OR (
                length(gloss_en) <= 100
                AND gloss_en NOT LIKE '%<%'
                AND gloss_en NOT LIKE '%>%'
                AND instr(gloss_en, char(0)) = 0
                AND instr(gloss_en, char(1)) = 0
                AND instr(gloss_en, char(2)) = 0
                AND instr(gloss_en, char(3)) = 0
                AND instr(gloss_en, char(4)) = 0
                AND instr(gloss_en, char(5)) = 0
                AND instr(gloss_en, char(6)) = 0
                AND instr(gloss_en, char(7)) = 0
                AND instr(gloss_en, char(8)) = 0
                AND instr(gloss_en, char(9)) = 0
                AND instr(gloss_en, char(10)) = 0
                AND instr(gloss_en, char(11)) = 0
                AND instr(gloss_en, char(12)) = 0
                AND instr(gloss_en, char(13)) = 0
                AND instr(gloss_en, char(14)) = 0
                AND instr(gloss_en, char(15)) = 0
                AND instr(gloss_en, char(16)) = 0
                AND instr(gloss_en, char(17)) = 0
                AND instr(gloss_en, char(18)) = 0
                AND instr(gloss_en, char(19)) = 0
                AND instr(gloss_en, char(20)) = 0
                AND instr(gloss_en, char(21)) = 0
                AND instr(gloss_en, char(22)) = 0
                AND instr(gloss_en, char(23)) = 0
                AND instr(gloss_en, char(24)) = 0
                AND instr(gloss_en, char(25)) = 0
                AND instr(gloss_en, char(26)) = 0
                AND instr(gloss_en, char(27)) = 0
                AND instr(gloss_en, char(28)) = 0
                AND instr(gloss_en, char(29)) = 0
                AND instr(gloss_en, char(30)) = 0
                AND instr(gloss_en, char(31)) = 0
                AND instr(gloss_en, char(127)) = 0
            ))
        )
--- table users
CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT NOT NULL COLLATE NOCASE,
            password_hash TEXT NOT NULL,
            display_name TEXT,
            role TEXT NOT NULL DEFAULT 'user'
                CHECK (role IN ('user', 'admin')),
            status TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'disabled')),
            auth_provider TEXT NOT NULL DEFAULT 'local',
            provider_subject TEXT,
            must_change_password INTEGER NOT NULL DEFAULT 0
                CHECK (must_change_password IN (0, 1)),
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL, ai_planning INTEGER NOT NULL DEFAULT 0
            CHECK (ai_planning IN (0, 1)), google_email TEXT,
            CHECK (username = lower(username)),
            UNIQUE (username),
            UNIQUE (auth_provider, provider_subject)
        )
--- index idx_concept_mastery_user_id on concept_mastery
CREATE INDEX idx_concept_mastery_user_id ON concept_mastery (user_id)
--- index idx_dictionary_en_fold on dictionary_glosses
CREATE INDEX idx_dictionary_en_fold ON dictionary_glosses (en_fold)
--- index idx_dictionary_entries_fold on dictionary_entries
CREATE INDEX idx_dictionary_entries_fold ON dictionary_entries (he_fold)
--- index idx_dictionary_he_fold on dictionary_glosses
CREATE INDEX idx_dictionary_he_fold ON dictionary_glosses (he_fold)
--- index idx_lexicon_entries_binyan on lexicon_entries
CREATE INDEX idx_lexicon_entries_binyan ON lexicon_entries (binyan)
--- index idx_lexicon_entries_lemma_key on lexicon_entries
CREATE INDEX idx_lexicon_entries_lemma_key ON lexicon_entries (lemma_key)
--- index idx_plan_field_marks_last_actor on plan_field_marks
CREATE INDEX idx_plan_field_marks_last_actor
            ON plan_field_marks (plan_id, last_actor_kind)
--- index idx_plan_revisions_plan_created on plan_revisions
CREATE INDEX idx_plan_revisions_plan_created
            ON plan_revisions (plan_id, created_at)
--- index idx_practice_events_user_at on practice_events
CREATE INDEX idx_practice_events_user_at ON practice_events (user_id, at)
--- index idx_review_items_user_due on review_items
CREATE INDEX idx_review_items_user_due ON review_items (user_id, due)
--- index idx_users_google_email on users
CREATE UNIQUE INDEX idx_users_google_email
        ON users (google_email) WHERE google_email IS NOT NULL
--- index sqlite_autoindex_concept_mastery_1 on concept_mastery
--- index sqlite_autoindex_dictionary_entries_1 on dictionary_entries
--- index sqlite_autoindex_dictionary_glosses_1 on dictionary_glosses
--- index sqlite_autoindex_grammar_concepts_1 on grammar_concepts
--- index sqlite_autoindex_lesson_grants_1 on lesson_grants
--- index sqlite_autoindex_lesson_progress_1 on lesson_progress
--- index sqlite_autoindex_lesson_visibility_1 on lesson_visibility
--- index sqlite_autoindex_lexicon_entries_1 on lexicon_entries
--- index sqlite_autoindex_media_1 on media
--- index sqlite_autoindex_meta_1 on meta
--- index sqlite_autoindex_plan_field_marks_1 on plan_field_marks
--- index sqlite_autoindex_plan_revisions_1 on plan_revisions
--- index sqlite_autoindex_plans_1 on plans
--- index sqlite_autoindex_practice_events_1 on practice_events
--- index sqlite_autoindex_review_items_1 on review_items
--- index sqlite_autoindex_sessions_1 on sessions
--- index sqlite_autoindex_user_words_1 on user_words
--- index sqlite_autoindex_users_1 on users
--- index sqlite_autoindex_users_2 on users
--- trigger lexicon_entries_accept_insert on lexicon_entries
CREATE TRIGGER lexicon_entries_accept_insert
        BEFORE INSERT ON lexicon_entries
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
                            OR instr(json_each.value, char(1425)) > 0
                            OR instr(json_each.value, char(1426)) > 0
                            OR instr(json_each.value, char(1427)) > 0
                            OR instr(json_each.value, char(1428)) > 0
                            OR instr(json_each.value, char(1429)) > 0
                            OR instr(json_each.value, char(1430)) > 0
                            OR instr(json_each.value, char(1431)) > 0
                            OR instr(json_each.value, char(1432)) > 0
                            OR instr(json_each.value, char(1433)) > 0
                            OR instr(json_each.value, char(1434)) > 0
                            OR instr(json_each.value, char(1435)) > 0
                            OR instr(json_each.value, char(1436)) > 0
                            OR instr(json_each.value, char(1437)) > 0
                            OR instr(json_each.value, char(1438)) > 0
                            OR instr(json_each.value, char(1439)) > 0
                            OR instr(json_each.value, char(1440)) > 0
                            OR instr(json_each.value, char(1441)) > 0
                            OR instr(json_each.value, char(1442)) > 0
                            OR instr(json_each.value, char(1443)) > 0
                            OR instr(json_each.value, char(1444)) > 0
                            OR instr(json_each.value, char(1445)) > 0
                            OR instr(json_each.value, char(1446)) > 0
                            OR instr(json_each.value, char(1447)) > 0
                            OR instr(json_each.value, char(1448)) > 0
                            OR instr(json_each.value, char(1449)) > 0
                            OR instr(json_each.value, char(1450)) > 0
                            OR instr(json_each.value, char(1451)) > 0
                            OR instr(json_each.value, char(1452)) > 0
                            OR instr(json_each.value, char(1453)) > 0
                            OR instr(json_each.value, char(1454)) > 0
                            OR instr(json_each.value, char(1455)) > 0
                            OR instr(json_each.value, char(1456)) > 0
                            OR instr(json_each.value, char(1457)) > 0
                            OR instr(json_each.value, char(1458)) > 0
                            OR instr(json_each.value, char(1459)) > 0
                            OR instr(json_each.value, char(1460)) > 0
                            OR instr(json_each.value, char(1461)) > 0
                            OR instr(json_each.value, char(1462)) > 0
                            OR instr(json_each.value, char(1463)) > 0
                            OR instr(json_each.value, char(1464)) > 0
                            OR instr(json_each.value, char(1465)) > 0
                            OR instr(json_each.value, char(1466)) > 0
                            OR instr(json_each.value, char(1467)) > 0
                            OR instr(json_each.value, char(1468)) > 0
                            OR instr(json_each.value, char(1469)) > 0
                            OR instr(json_each.value, char(1470)) > 0
                            OR instr(json_each.value, char(1471)) > 0
                            OR instr(json_each.value, char(1472)) > 0
                            OR instr(json_each.value, char(1473)) > 0
                            OR instr(json_each.value, char(1474)) > 0
                            OR instr(json_each.value, char(1475)) > 0
                            OR instr(json_each.value, char(1476)) > 0
                            OR instr(json_each.value, char(1477)) > 0
                            OR instr(json_each.value, char(1478)) > 0
                            OR instr(json_each.value, char(1479)) > 0
                        )
                );
        END
--- trigger lexicon_entries_accept_update on lexicon_entries
CREATE TRIGGER lexicon_entries_accept_update
        BEFORE UPDATE OF accept_json ON lexicon_entries
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
                            OR instr(json_each.value, char(1425)) > 0
                            OR instr(json_each.value, char(1426)) > 0
                            OR instr(json_each.value, char(1427)) > 0
                            OR instr(json_each.value, char(1428)) > 0
                            OR instr(json_each.value, char(1429)) > 0
                            OR instr(json_each.value, char(1430)) > 0
                            OR instr(json_each.value, char(1431)) > 0
                            OR instr(json_each.value, char(1432)) > 0
                            OR instr(json_each.value, char(1433)) > 0
                            OR instr(json_each.value, char(1434)) > 0
                            OR instr(json_each.value, char(1435)) > 0
                            OR instr(json_each.value, char(1436)) > 0
                            OR instr(json_each.value, char(1437)) > 0
                            OR instr(json_each.value, char(1438)) > 0
                            OR instr(json_each.value, char(1439)) > 0
                            OR instr(json_each.value, char(1440)) > 0
                            OR instr(json_each.value, char(1441)) > 0
                            OR instr(json_each.value, char(1442)) > 0
                            OR instr(json_each.value, char(1443)) > 0
                            OR instr(json_each.value, char(1444)) > 0
                            OR instr(json_each.value, char(1445)) > 0
                            OR instr(json_each.value, char(1446)) > 0
                            OR instr(json_each.value, char(1447)) > 0
                            OR instr(json_each.value, char(1448)) > 0
                            OR instr(json_each.value, char(1449)) > 0
                            OR instr(json_each.value, char(1450)) > 0
                            OR instr(json_each.value, char(1451)) > 0
                            OR instr(json_each.value, char(1452)) > 0
                            OR instr(json_each.value, char(1453)) > 0
                            OR instr(json_each.value, char(1454)) > 0
                            OR instr(json_each.value, char(1455)) > 0
                            OR instr(json_each.value, char(1456)) > 0
                            OR instr(json_each.value, char(1457)) > 0
                            OR instr(json_each.value, char(1458)) > 0
                            OR instr(json_each.value, char(1459)) > 0
                            OR instr(json_each.value, char(1460)) > 0
                            OR instr(json_each.value, char(1461)) > 0
                            OR instr(json_each.value, char(1462)) > 0
                            OR instr(json_each.value, char(1463)) > 0
                            OR instr(json_each.value, char(1464)) > 0
                            OR instr(json_each.value, char(1465)) > 0
                            OR instr(json_each.value, char(1466)) > 0
                            OR instr(json_each.value, char(1467)) > 0
                            OR instr(json_each.value, char(1468)) > 0
                            OR instr(json_each.value, char(1469)) > 0
                            OR instr(json_each.value, char(1470)) > 0
                            OR instr(json_each.value, char(1471)) > 0
                            OR instr(json_each.value, char(1472)) > 0
                            OR instr(json_each.value, char(1473)) > 0
                            OR instr(json_each.value, char(1474)) > 0
                            OR instr(json_each.value, char(1475)) > 0
                            OR instr(json_each.value, char(1476)) > 0
                            OR instr(json_each.value, char(1477)) > 0
                            OR instr(json_each.value, char(1478)) > 0
                            OR instr(json_each.value, char(1479)) > 0
                        )
                );
        END
