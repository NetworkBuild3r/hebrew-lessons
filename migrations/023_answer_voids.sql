-- Voids beside answers (INIT-038/SPEC-002). Additive. ADR-0033 D1.
-- A void is a flag. It never edits or deletes an attempts, review_log, or confusion_misses row.
-- undone_at null means the void is in force. A set undone_at counts the answer again.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.
-- The new table can stay; a database with no void rows reads as it did before this file.

CREATE TABLE answer_voids (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    attempt_id INTEGER NOT NULL REFERENCES attempts (id) ON DELETE CASCADE, -- The voided attempt. Deleting the attempt deletes this flag.
    review_log_id INTEGER REFERENCES review_log (id) ON DELETE CASCADE, -- The review_log row this void covers, when the answer wrote one. Null when it did not.
    confusion_id INTEGER REFERENCES confusion_misses (id) ON DELETE CASCADE, -- The confusion row this void covers, when there is one. Null when it did not.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner the answer belongs to. Deleting the user deletes their voids.
    reason TEXT NOT NULL CHECK (reason IN ('unit_12_34', 'no_cue_blank', 'also_right', 'report')), -- Why the answer was taken back.
    actor TEXT NOT NULL CHECK (typeof(actor) = 'text' AND length(actor) BETWEEN 1 AND 200), -- Who voided it: a person, or a command name. Not shown as learner input.
    batch_id TEXT NOT NULL CHECK (typeof(batch_id) = 'text' AND length(batch_id) BETWEEN 1 AND 200), -- Voids undone together. Opaque id.
    created_at TEXT NOT NULL CHECK (typeof(created_at) = 'text' AND length(created_at) >= 1), -- UTC time the void was written, FormatTime.
    undone_at TEXT CHECK (undone_at IS NULL OR (typeof(undone_at) = 'text' AND length(undone_at) >= 1)) -- UTC time the batch was undone. Null while the void is in force.
);

-- Voids of one learner, in the order they were written.
CREATE INDEX answer_voids_learner ON answer_voids (learner_id, id);

-- One batch, for undo and for listing.
CREATE INDEX answer_voids_batch ON answer_voids (batch_id, id);

-- Evidence reads look up an active void by the row it covers.
CREATE INDEX answer_voids_active_attempt ON answer_voids (attempt_id) WHERE undone_at IS NULL;

CREATE INDEX answer_voids_active_review ON answer_voids (review_log_id) WHERE undone_at IS NULL AND review_log_id IS NOT NULL;

CREATE INDEX answer_voids_active_confusion ON answer_voids (confusion_id) WHERE undone_at IS NULL AND confusion_id IS NOT NULL;
