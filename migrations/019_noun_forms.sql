-- Noun and adjective forms: construct, possessive, and agreement cells.
-- Additive. A database at 018 keeps every row.
-- There is no down migration. To roll back, restore the pre-migration file.
-- The new table can stay; nothing reads it until this revision of the app.
-- INIT-023/SPEC-005

CREATE TABLE noun_forms (
    id INTEGER PRIMARY KEY,
    lemma_lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('singular', 'plural', 'definite', 'construct', 'possessive', 'agreement')),
    number TEXT NOT NULL DEFAULT '' CHECK (number IN ('', 'sg', 'pl')),
    person TEXT NOT NULL DEFAULT '' CHECK (person IN ('', '1', '2', '3')),
    gender TEXT NOT NULL DEFAULT '' CHECK (gender IN ('', 'm', 'f')),
    owner_number TEXT NOT NULL DEFAULT '' CHECK (owner_number IN ('', 'sg', 'pl')),
    form_he TEXT NOT NULL CHECK (length(form_he) BETWEEN 1 AND 200),
    form_translit TEXT NOT NULL DEFAULT '' CHECK (length(form_translit) <= 200),
    form_lexeme_id INTEGER REFERENCES lexemes (id) ON DELETE SET NULL,
    UNIQUE (lemma_lexeme_id, kind, number, person, gender, owner_number)
);
