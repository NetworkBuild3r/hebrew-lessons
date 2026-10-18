-- Learning columns and the confusion log.
-- Applied after 001_init.sql. lexemes.root already exists there, so this file does not add it.
--
-- confusion_misses is one row per miss. It has no rolling counter.
-- The 7-day total is COUNT(*) at query time.

-- m masculine, f feminine, both either. Null until the lexicon records it.
ALTER TABLE lexemes ADD COLUMN gender TEXT CHECK (gender IN ('m', 'f', 'both'));

-- Plural spelling. Null until it is recorded.
ALTER TABLE lexemes ADD COLUMN plural TEXT;

-- 1 when the plural is not the regular form of text_nfc. Existing rows stay 0.
ALTER TABLE lexemes ADD COLUMN irregular_plural INTEGER NOT NULL DEFAULT 0 CHECK (irregular_plural IN (0, 1));

-- Latin transliteration. Null until it is recorded.
ALTER TABLE lexemes ADD COLUMN translit TEXT;

-- How to address this learner. m or f. Null until they choose. 'both' is not a form of address.
ALTER TABLE users ADD COLUMN address_gender TEXT CHECK (address_gender IN ('m', 'f'));

-- 1 when the learner revealed the answer before this grade. Existing rows stay 0.
ALTER TABLE review_log ADD COLUMN peeked INTEGER NOT NULL DEFAULT 0 CHECK (peeked IN (0, 1));

-- One confusion. letter_pair feeds a letter drill (for example ב/כ). agreement feeds a pattern drill.
-- Deleting the learner deletes the misses.
CREATE TABLE confusion_misses (
    id INTEGER PRIMARY KEY, -- Surrogate key. Each row is one miss, not a counter.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    kind TEXT NOT NULL CHECK (kind IN ('letter_pair', 'agreement')), -- letter_pair or agreement.
    key TEXT NOT NULL, -- The confused pair, for example ב/כ.
    at TEXT NOT NULL -- UTC time of the miss, from the process clock.
);

-- The 7-day count reads one learner and one key across time.
CREATE INDEX confusion_misses_learner_key_at ON confusion_misses (learner_id, key, at);
