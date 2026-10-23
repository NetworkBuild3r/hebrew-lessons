-- Personal mistakes, the shared "learners often say" list, verb forms,
-- and the note that opens Isaiah in the navigation.
-- Additive. A database at 013 keeps every row.
-- There is no down migration. To roll back, restore the pre-migration file.
-- The new tables can stay; nothing reads them until this revision of the app.
-- Nothing copies a row from learner_mistakes into common_mistakes.

CREATE TABLE learner_mistakes (
    id INTEGER PRIMARY KEY,
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    wrong_he TEXT NOT NULL CHECK (length(wrong_he) BETWEEN 1 AND 200), -- What was said, pointed.
    right_he TEXT NOT NULL CHECK (length(right_he) BETWEEN 1 AND 200), -- The right form, pointed.
    note_en TEXT NOT NULL DEFAULT '' CHECK (length(note_en) <= 300), -- Optional English note.
    pattern_slug TEXT REFERENCES grammar_patterns (slug) ON DELETE SET NULL, -- Pattern this belongs to, or null.
    lexeme_id INTEGER REFERENCES lexemes (id) ON DELETE SET NULL, -- The word, when one is known.
    times INTEGER NOT NULL CHECK (times >= 1), -- How often this pair has come back.
    first_at TEXT NOT NULL, -- UTC RFC3339 of the first time.
    last_at TEXT NOT NULL, -- UTC RFC3339 of the latest time.
    source TEXT NOT NULL CHECK (source IN ('tutor', 'app')), -- Who recorded it.
    UNIQUE (learner_id, wrong_he, right_he)
);

CREATE INDEX learner_mistakes_learner ON learner_mistakes (learner_id, times DESC, last_at DESC);

-- Shared. No learner column. A personal row is never written here.
CREATE TABLE common_mistakes (
    id INTEGER PRIMARY KEY,
    pattern_slug TEXT REFERENCES grammar_patterns (slug) ON DELETE SET NULL,
    wrong_he TEXT NOT NULL CHECK (length(wrong_he) BETWEEN 1 AND 200),
    right_he TEXT NOT NULL CHECK (length(right_he) BETWEEN 1 AND 200),
    explain_en TEXT NOT NULL DEFAULT '' CHECK (length(explain_en) <= 300),
    status TEXT NOT NULL CHECK (status IN ('draft', 'published'))
);

CREATE UNIQUE INDEX common_mistakes_key ON common_mistakes (ifnull(pattern_slug, ''), wrong_he, right_he);

CREATE TABLE verb_forms (
    id INTEGER PRIMARY KEY,
    lemma_lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE, -- The dictionary form.
    tense TEXT NOT NULL CHECK (tense IN ('present', 'past', 'future', 'imperative', 'infinitive')),
    person TEXT NOT NULL DEFAULT '' CHECK (person IN ('', '1', '2', '3')),
    number TEXT NOT NULL DEFAULT '' CHECK (number IN ('', 'sg', 'pl')),
    gender TEXT NOT NULL DEFAULT '' CHECK (gender IN ('', 'm', 'f')),
    form_he TEXT NOT NULL CHECK (length(form_he) BETWEEN 1 AND 200), -- The pointed form.
    form_translit TEXT NOT NULL DEFAULT '' CHECK (length(form_translit) <= 200),
    form_lexeme_id INTEGER REFERENCES lexemes (id) ON DELETE SET NULL, -- The lexicon entry of this form, when it has one.
    UNIQUE (lemma_lexeme_id, tense, person, number, gender)
);

-- A checked row is what shows Isaiah in the navigation. The reader itself is later.
CREATE TABLE verse_notes (
    id INTEGER PRIMARY KEY,
    ref TEXT NOT NULL CHECK (length(ref) BETWEEN 1 AND 40),
    status TEXT NOT NULL CHECK (status IN ('draft', 'checked'))
);
