-- Isaiah verse notes (INIT-021/SPEC-001).
-- 014 created verse_notes as a navigation placeholder: id, ref, status.
-- That shape cannot hold a verse, a layer, or a body. This rebuilds it.
-- Placeholder rows are not copied. They named no verse in tanakh_verses,
-- and a note for a verse that is not there is refused.
-- Additive beside every other table. There is no down migration.
-- To roll back, restore the pre-migration file.

DROP TABLE verse_notes;

CREATE TABLE verse_notes (
    book TEXT NOT NULL CHECK (
        length(book) BETWEEN 1 AND 40
        AND book NOT GLOB '*[^a-z0-9-]*'
    ),
    chapter INTEGER NOT NULL CHECK (chapter >= 1),
    verse INTEGER NOT NULL CHECK (verse >= 1),
    layer TEXT NOT NULL CHECK (layer IN ('gloss', 'roots', 'manner', 'bridge')),
    body TEXT NOT NULL CHECK (length(CAST(body AS BLOB)) BETWEEN 1 AND 8000),
    status TEXT NOT NULL CHECK (status IN ('draft', 'checked')),
    drafted_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
    checked_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
    updated_at TEXT NOT NULL CHECK (length(updated_at) BETWEEN 20 AND 40),
    PRIMARY KEY (book, chapter, verse, layer),
    FOREIGN KEY (book, chapter, verse) REFERENCES tanakh_verses (book, chapter, verse)
);

-- One gloss per word position. status is draft until a person checks it.
-- root and lexeme_id are optional. A missing verse is refused.
CREATE TABLE verse_word_glosses (
    book TEXT NOT NULL CHECK (
        length(book) BETWEEN 1 AND 40
        AND book NOT GLOB '*[^a-z0-9-]*'
    ),
    chapter INTEGER NOT NULL CHECK (chapter >= 1),
    verse INTEGER NOT NULL CHECK (verse >= 1),
    position INTEGER NOT NULL CHECK (position >= 0 AND position <= 80),
    word_he TEXT NOT NULL CHECK (length(word_he) BETWEEN 1 AND 80),
    gloss_en TEXT NOT NULL CHECK (length(gloss_en) BETWEEN 1 AND 200),
    root TEXT CHECK (root IS NULL OR length(root) BETWEEN 1 AND 40),
    lexeme_id INTEGER REFERENCES lexemes (id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'checked')),
    PRIMARY KEY (book, chapter, verse, position),
    FOREIGN KEY (book, chapter, verse) REFERENCES tanakh_verses (book, chapter, verse)
);
