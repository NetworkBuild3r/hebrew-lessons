-- Drill answers, cleared confusion pairs, and implied review_log rows.
-- Additive. A database at 020 keeps every row.
-- There is no down migration. To roll back, restore the pre-migration file.
-- The new tables and column can stay; nothing reads them until this revision of the app.
-- INIT-027/SPEC-002

-- One graded, timed answer to a letter-pair or agreement drill.
-- Deleting the learner deletes these rows.
CREATE TABLE drill_answers (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    kind TEXT NOT NULL CHECK (kind IN ('letter_pair', 'agreement')), -- letter_pair or agreement.
    key TEXT NOT NULL CHECK (length(key) BETWEEN 1 AND 64), -- The pair, for example ב/כ.
    correct INTEGER NOT NULL CHECK (correct IN (0, 1)), -- 1 when the pick was right.
    latency_ms INTEGER NOT NULL DEFAULT 0 CHECK (latency_ms >= 0), -- Server-measured pick latency in milliseconds.
    answered_at TEXT NOT NULL -- UTC RFC3339 time of the answer.
);

CREATE INDEX drill_answers_learner_kind_key_at ON drill_answers (learner_id, kind, key, answered_at);

-- One clear of a confusion pair. Many rows per pair are allowed; the latest counts.
-- Deleting the learner deletes these rows.
CREATE TABLE confusion_clears (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    kind TEXT NOT NULL CHECK (kind IN ('letter_pair', 'agreement')), -- letter_pair or agreement.
    key TEXT NOT NULL CHECK (length(key) BETWEEN 1 AND 64), -- The pair, for example ב/כ.
    cleared_at TEXT NOT NULL -- UTC RFC3339 time the pair was cleared.
);

CREATE INDEX confusion_clears_learner_kind_key_at ON confusion_clears (learner_id, kind, key, cleared_at);

-- 1 when this review_log row was written as implied credit from another skill's answer.
-- Existing rows stay 0. Accuracy counts skip implied rows.
ALTER TABLE review_log ADD COLUMN implied INTEGER NOT NULL DEFAULT 0 CHECK (implied IN (0, 1));
