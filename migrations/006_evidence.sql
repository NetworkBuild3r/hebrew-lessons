-- Evidence tables and card skills (INIT-015/SPEC-003). Applied after 005_lesson_progress.sql.
-- attempts and exposures record what the learner did and saw. cards trade their dir column
-- for a skill column, and every existing row and id is kept so review_log still points at the right card.
-- The runner applies this file in one transaction. If any statement fails, the database stays at 005.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.

-- One checked answer by one learner on one word. Append-only. Learner state is derived from these rows.
CREATE TABLE attempts (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their attempts.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word that was asked. A lexeme with attempts cannot be deleted.
    skill TEXT NOT NULL CHECK (skill IN ('listen', 'read', 'recall', 'spell')), -- Skill the question exercised.
    activity TEXT NOT NULL, -- Activity kind that asked it, free text, for example listen_pick or cloze.
    source TEXT NOT NULL CHECK (source IN ('placement', 'review', 'lesson', 'practice')), -- Where the question came from.
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL, -- Unit the lesson belongs to. Null outside a lesson. Deleting the unit keeps the attempt.
    prompt TEXT NOT NULL, -- What the learner was shown or asked, as text.
    response TEXT NOT NULL, -- What the learner picked or typed. Learner input: escape it on output and show it only to that learner.
    correct INTEGER NOT NULL CHECK (correct IN (0, 1)), -- 1 when the response was right.
    dont_know INTEGER NOT NULL DEFAULT 0 CHECK (dont_know IN (0, 1)), -- 1 when the learner chose I don't know instead of answering.
    hint_used INTEGER NOT NULL DEFAULT 0 CHECK (hint_used IN (0, 1)), -- 1 when the learner opened a hint before answering.
    latency_ms INTEGER NOT NULL CHECK (latency_ms >= 0), -- Milliseconds from showing the question to the answer.
    created_at TEXT NOT NULL -- UTC RFC3339 time the answer was checked.
);

-- History of one learner on one word, in one skill, oldest first.
CREATE INDEX attempts_user_lexeme_skill ON attempts (user_id, lexeme_id, skill, created_at);

-- Recent activity of one learner across all words.
CREATE INDEX attempts_user_created ON attempts (user_id, created_at);

-- One time a word was put in front of a learner without a checked answer.
CREATE TABLE exposures (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their exposures.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word that was shown. A lexeme with exposures cannot be deleted.
    context TEXT NOT NULL CHECK (context IN ('story', 'dialogue', 'gloss_tap', 'placement_review', 'lesson_intro')), -- Where the word was shown.
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL, -- Unit the exposure happened in. Null outside a unit. Deleting the unit keeps the exposure.
    created_at TEXT NOT NULL -- UTC RFC3339 time the word was shown.
);

-- Exposure history of one learner on one word.
CREATE INDEX exposures_user_lexeme ON exposures (user_id, lexeme_id);

-- Rebuild cards with skill in place of dir. he_en becomes read and en_he becomes recall.
-- The runner holds foreign_keys on, and the pragma cannot change inside a transaction.
-- Dropping cards would therefore cascade into review_log, so the log is copied aside first and put back after.
-- SELECT * keeps every review_log column, including those added by 002 and 003.
CREATE TEMP TABLE review_log_keep AS SELECT * FROM review_log;

CREATE TABLE cards_new (
    id INTEGER PRIMARY KEY, -- Surrogate key. Copied unchanged from the old table. review_log rows point here.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their cards.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word. Deleting a lexeme that still has cards is refused.
    skill TEXT NOT NULL CHECK (skill IN ('listen', 'read', 'recall', 'spell')), -- Skill the card trains. Old he_en cards are read and old en_he cards are recall.
    state TEXT NOT NULL CHECK (state IN ('new', 'learning', 'review', 'relearning')), -- FSRS state. Maps to go-fsrs New, Learning, Review, Relearning.
    due TEXT NOT NULL, -- UTC RFC3339 time the card is due.
    stability REAL NOT NULL CHECK (stability >= 0), -- FSRS stability, in days.
    difficulty REAL NOT NULL CHECK (difficulty >= 0), -- FSRS difficulty.
    reps INTEGER NOT NULL CHECK (reps >= 0), -- How many times the card has been reviewed.
    lapses INTEGER NOT NULL CHECK (lapses >= 0), -- How many times the card has been forgotten.
    last_review TEXT, -- UTC RFC3339 time of the previous review. Null until the first review.
    suspended INTEGER NOT NULL DEFAULT 0 CHECK (suspended IN (0, 1)), -- 1 keeps the card out of the queue without deleting its history.
    UNIQUE (user_id, lexeme_id, skill)
);

-- A dir outside he_en and en_he yields a null skill, the NOT NULL check fails, and the whole migration rolls back.
INSERT INTO cards_new (id, user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
SELECT id, user_id, lexeme_id,
    CASE dir WHEN 'he_en' THEN 'read' WHEN 'en_he' THEN 'recall' END,
    state, due, stability, difficulty, reps, lapses, last_review, suspended
FROM cards;

-- This implicit delete cascades into review_log. The rows are restored below.
DROP TABLE cards;

ALTER TABLE cards_new RENAME TO cards;

-- Due queue: one learner, soonest due first. Suspended cards stay in the index and are filtered by the query.
CREATE INDEX cards_user_due ON cards (user_id, due);

-- Clear anything the cascade left, then restore the log. The foreign key on card_id is checked row by row here.
DELETE FROM review_log;

INSERT INTO review_log SELECT * FROM review_log_keep;

DROP TABLE review_log_keep;

-- Reports any row whose parent is missing. The runner discards the rows, so the inserts above are the enforcing check.
PRAGMA foreign_key_check;
