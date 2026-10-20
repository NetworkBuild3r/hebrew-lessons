-- Storage for the daily snapshot job (INIT-015/SPEC-014). Applied after 008_confusion_other.sql.
-- Two changes:
--   1. job_state: a small key-value table for the job. It holds the process lock, each learner's word states
--      as of their newest snapshot, and each learner's band shares by day. No such table existed.
--   2. regression_events is rebuilt so one row can also say "a band slipped". That row has no word, so lexeme_id,
--      skill, from_state, and to_state become nullable, and a kind column says which sort of row it is.
-- SQLite cannot relax a NOT NULL or a CHECK in place, so regression_events is rebuilt: every row and id is copied.
-- Nothing references regression_events (no foreign key points at it, and there is no view or trigger on it),
-- so dropping it cascades nowhere. The runner holds foreign_keys on inside its transaction and the pragma
-- cannot change there, which is why this was checked before choosing a plain drop. Unlike 006, no child rows
-- need to be copied aside. A test fails if a table ever references regression_events.
-- The runner applies this file in one transaction. If any statement fails, the database stays at 009.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.
-- This is a db_migration gate item. Do not run it on real data without the operator.

-- Key-value state of the scheduled jobs. Keys: daily.lock, daily.words.<user id>, daily.bands.<user id>.
CREATE TABLE job_state (
    key TEXT PRIMARY KEY NOT NULL CHECK (typeof(key) = 'text' AND length(key) BETWEEN 1 AND 200), -- Name of the state. Plain text, 200 characters at most.
    value TEXT NOT NULL CHECK (typeof(value) = 'text'), -- The state, usually JSON. The lock row holds the holder and the time it was taken.
    updated_at TEXT NOT NULL -- UTC RFC3339 time of the last write, from the process clock.
);

CREATE TABLE regression_events_new (
    id INTEGER PRIMARY KEY, -- Surrogate key. Copied unchanged from the old table.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their events.
    lexeme_id INTEGER REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word that changed state. Null for a band_slip. A lexeme with events cannot be deleted, as with attempts.
    skill TEXT CHECK (skill IS NULL OR skill IN ('listen', 'read', 'recall', 'spell')), -- Skill the change is for. Null for a band_slip.
    from_state TEXT CHECK (from_state IS NULL OR from_state IN ('unseen', 'seen', 'learning', 'struggling', 'known', 'solid', 'regressed')), -- State before the change. Null for a band_slip.
    to_state TEXT CHECK (to_state IS NULL OR to_state IN ('unseen', 'seen', 'learning', 'struggling', 'known', 'solid', 'regressed')), -- State after the change. Null for a band_slip. It differs from the state before when both are set.
    day TEXT NOT NULL CHECK (day GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]' AND date(day) IS NOT NULL AND date(day) = day), -- Calendar day of the change, YYYY-MM-DD, a real date.
    cause TEXT NOT NULL CHECK (typeof(cause) = 'text' AND length(cause) >= 1), -- Short reason code or text that explains the change. Not empty.
    kind TEXT NOT NULL DEFAULT 'word' CHECK (kind IN ('word', 'band_slip')), -- word is one word that changed state. band_slip is a whole frequency band losing known words.
    CHECK (from_state IS NULL OR to_state IS NULL OR to_state <> from_state), -- A change of state is a change.
    CHECK ( -- A word row names its word, skill, and both states. A band_slip row names none of them.
        (kind = 'word' AND lexeme_id IS NOT NULL AND skill IS NOT NULL AND from_state IS NOT NULL AND to_state IS NOT NULL)
        OR (kind = 'band_slip' AND lexeme_id IS NULL AND skill IS NULL AND from_state IS NULL AND to_state IS NULL)
    )
);

-- Every old row is a word row, so kind takes its default.
INSERT INTO regression_events_new (id, user_id, lexeme_id, skill, from_state, to_state, day, cause)
SELECT id, user_id, lexeme_id, skill, from_state, to_state, day, cause
FROM regression_events;

-- The indexes go with the old table.
DROP TABLE regression_events;

ALTER TABLE regression_events_new RENAME TO regression_events;

-- Events of one learner, by day.
CREATE INDEX regression_events_user_day ON regression_events (user_id, day);

-- History of one learner on one word and skill.
CREATE INDEX regression_events_user_lexeme_skill ON regression_events (user_id, lexeme_id, skill);

-- Reports any row whose parent is missing. The runner discards the rows, so the copy above is the enforcing check.
PRAGMA foreign_key_check;
