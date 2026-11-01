-- Learner civil-day zone, dated personal corrections, taught words outside the app,
-- and who wrote a coach note (admin or the learner).
-- Additive. A database at 019 keeps every row.
-- There is no down migration. To roll back, restore the pre-migration file.
-- The new columns and table can stay; nothing reads them until this revision of the app.
-- INIT-026/SPEC-002
-- time_zone holds only a name the app has already validated as IANA.
-- SQLite CHECK cannot call LoadLocation.

ALTER TABLE users ADD COLUMN time_zone TEXT CHECK (
    time_zone IS NULL OR (length(time_zone) BETWEEN 1 AND 64)
);

ALTER TABLE learner_mistakes ADD COLUMN first_on TEXT CHECK (
    first_on IS NULL OR (date(first_on) IS NOT NULL AND date(first_on) = first_on)
);
ALTER TABLE learner_mistakes ADD COLUMN last_on TEXT CHECK (
    last_on IS NULL OR (date(last_on) IS NOT NULL AND date(last_on) = last_on)
);

-- A word taught to a learner on a civil day, outside the in-app lesson path.
CREATE TABLE taught_words (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- The word. A lexeme with a taught row cannot be deleted.
    taught_on TEXT NOT NULL CHECK (
        date(taught_on) IS NOT NULL AND date(taught_on) = taught_on
    ), -- Learner's civil day the word was taught, YYYY-MM-DD.
    cards_created TEXT NOT NULL DEFAULT '', -- Comma-separated skills this row inserted: read, recall, listen.
    created_at TEXT NOT NULL, -- UTC RFC3339 instant the row was written.
    UNIQUE (learner_id, lexeme_id, taught_on)
);

CREATE INDEX taught_words_learner_day ON taught_words (learner_id, taught_on);

-- Existing notes were written by an admin client. DEFAULT fills them.
ALTER TABLE coach_notes ADD COLUMN author_role TEXT NOT NULL DEFAULT 'admin' CHECK (
    author_role IN ('admin', 'learner')
);
