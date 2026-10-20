-- Adds the other kind to confusion_misses (INIT-015/SPEC-013). Applied after 007_learner_model.sql.
-- Until now a miss was a letter_pair or an agreement. A wrong pick of any other word is also a confusion,
-- and the learner model reads those rows, so kind may now be other.
-- SQLite cannot change a CHECK in place, so the table is rebuilt: every row and id is copied unchanged.
-- Nothing references confusion_misses (no foreign key points at it, and there is no view or trigger on it),
-- so dropping it cascades nowhere. The runner holds foreign_keys on inside its transaction and the pragma
-- cannot change there, which is why this was checked before choosing a plain drop.
-- The runner applies this file in one transaction. If any statement fails, the database stays at 008.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.
-- This is a db_migration gate item. Do not run it on real data without the operator.

CREATE TABLE confusion_misses_new (
    id INTEGER PRIMARY KEY, -- Surrogate key. Each row is one miss, not a counter. Copied unchanged from the old table.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    kind TEXT NOT NULL CHECK (kind IN ('letter_pair', 'agreement', 'other')), -- letter_pair, agreement, or other (a wrong pick of another word).
    key TEXT NOT NULL, -- The confused pair, for example ב/כ. For other, the two words in pointed NFC, in code point order.
    at TEXT NOT NULL, -- UTC time of the miss, from the process clock.
    lexeme_id INTEGER REFERENCES lexemes (id) -- The word that was asked. Null when the miss is not tied to a lexeme.
);

INSERT INTO confusion_misses_new (id, learner_id, kind, key, at, lexeme_id)
SELECT id, learner_id, kind, key, at, lexeme_id
FROM confusion_misses;

-- The index goes with the old table.
DROP TABLE confusion_misses;

ALTER TABLE confusion_misses_new RENAME TO confusion_misses;

-- The 7-day count reads one learner and one key across time.
CREATE INDEX confusion_misses_learner_key_at ON confusion_misses (learner_id, key, at);

-- Reports any row whose parent is missing. The runner discards the rows, so the copy above is the enforcing check.
PRAGMA foreign_key_check;
