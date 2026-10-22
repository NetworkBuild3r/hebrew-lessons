-- Adds lesson_listen to exposures.context. Applied after 012_attempt_list_index.sql.
-- A lesson can show a word with audio and no question. That showing is an exposure, and the
-- context is lesson_listen. Versions 013 and 014 are not in this tree.
-- SQLite cannot change a CHECK in place, so the table is rebuilt. Every row and id is copied unchanged.
-- The columns, types, defaults, foreign keys, and the exposures_user_lexeme index match migration 006.
-- Nothing references exposures: no inbound foreign key, view, or trigger. Confirmed against migrations 001-012.
-- The runner (internal/db/migrate.go apply) runs this file in one transaction and then records version 15.
-- Open turns foreign_keys on in the DSN (_foreign_keys=on). PRAGMA foreign_keys cannot change inside
-- a transaction, so this file leaves it on and leaves legacy_alter_table at its default of 0.
-- DROP TABLE of a parent runs an implicit DELETE, which would cascade or restrict if another table
-- pointed at exposures. None does, so the drop does not need legacy_alter_table and does not touch
-- users, lexemes, or units. The foreign keys from exposures to those tables are declared again below.
--
-- Old binary: a binary built before this migration does not break when it reads a lesson_listen row.
-- The only production read is LearnerEvidence, which selects lexeme_id, COUNT(*), and MAX(created_at)
-- and never the context value, so the row is counted like any other exposure. SQLite checks CHECK
-- on INSERT and UPDATE, not on SELECT, and this code never updates an exposures row. The five contexts
-- the old Expose switch writes are still in the CHECK, so those inserts still succeed. The old switch
-- rejects lesson_listen before it reaches SQL, so the old binary will not write the new value.
--
-- Rollback does not delete rows. lesson_intro is already in the pre-015 CHECK (migrations/006_evidence.sql).
-- Remap lesson_listen onto lesson_intro, rebuild the table with the old CHECK, then forget version 15.
-- Alternatively, restore the pre-migration backup. Do this with no writer open.
-- ROLLBACK RECIPE BEGIN
-- UPDATE exposures SET context = 'lesson_intro' WHERE context = 'lesson_listen';
-- CREATE TABLE exposures_old (
--     id INTEGER PRIMARY KEY,
--     user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
--     lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT,
--     context TEXT NOT NULL CHECK (context IN ('story', 'dialogue', 'gloss_tap', 'placement_review', 'lesson_intro')),
--     unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL,
--     created_at TEXT NOT NULL
-- );
-- INSERT INTO exposures_old (id, user_id, lexeme_id, context, unit_id, created_at)
-- SELECT id, user_id, lexeme_id, context, unit_id, created_at
-- FROM exposures;
-- DROP TABLE exposures;
-- ALTER TABLE exposures_old RENAME TO exposures;
-- CREATE INDEX exposures_user_lexeme ON exposures (user_id, lexeme_id);
-- DELETE FROM schema_migrations WHERE version = 15;
-- ROLLBACK RECIPE END

CREATE TABLE exposures_new (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their exposures.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word that was shown. A lexeme with exposures cannot be deleted.
    context TEXT NOT NULL CHECK (context IN ('story', 'dialogue', 'gloss_tap', 'placement_review', 'lesson_intro', 'lesson_listen')), -- Where the word was shown.
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL, -- Unit the exposure happened in. Null outside a unit. Deleting the unit keeps the exposure.
    created_at TEXT NOT NULL -- UTC RFC3339 time the word was shown.
);

INSERT INTO exposures_new (id, user_id, lexeme_id, context, unit_id, created_at)
SELECT id, user_id, lexeme_id, context, unit_id, created_at
FROM exposures;

-- The index is dropped with the old table.
DROP TABLE exposures;

ALTER TABLE exposures_new RENAME TO exposures;

-- Exposure history of one learner on one word.
CREATE INDEX exposures_user_lexeme ON exposures (user_id, lexeme_id);

-- Reports any row whose parent is missing. The runner discards the rows, so the copy above is the enforcing check.
PRAGMA foreign_key_check;
