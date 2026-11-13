-- Words of the common-word course (INIT-039/SPEC-024). Additive. Shared content, not learner data.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.

CREATE TABLE course_words (
    lexeme_id INTEGER PRIMARY KEY REFERENCES lexemes (id) ON DELETE RESTRICT, -- The lexicon row this course word uses.
    course_order INTEGER NOT NULL CHECK (course_order >= 1), -- Position in internal/speech/course.tsv.
    list_rank INTEGER NOT NULL CHECK (list_rank >= 1), -- Rank of the source lemma in the frequency list.
    lemma TEXT NOT NULL, -- The source lemma, unpointed, as the list spells it.
    linked INTEGER NOT NULL DEFAULT 0 CHECK (linked IN (0, 1)), -- 1 when the course reused a lexeme row that already existed.
    added_at TEXT NOT NULL -- UTC RFC3339 instant the course first used this row.
);

CREATE INDEX course_words_order ON course_words (course_order);
