-- Placement runs, the ratings, and the columns the struggle and scope reads need.
-- Applied after 002_learning.sql. There is no learner_lexemes table:
-- known and learning are read from cards.

-- One placement run. A run can take several sittings.
CREATE TABLE placement_runs (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes the run.
    started_at TEXT NOT NULL, -- UTC time the run started, from the process clock.
    finished_at TEXT, -- UTC time the run was scored. Null while it is in progress.
    level INTEGER CHECK (level IS NULL OR level BETWEEN -1 AND 4), -- Highest band completed without tripping the struggle rule. -1 when a finished run completed none. Null until the run is finished.
    stop_reason TEXT CHECK (stop_reason IS NULL OR stop_reason IN ('band_stop', 'cap_cards', 'exhausted')), -- Why the deck stopped. Null when the learner finished before a stop, and while the run is in progress.
    result TEXT CHECK (result IS NULL OR json_valid(result)), -- JSON object with per-band counts and the known-word estimate. Null until the run is finished.
    seed INTEGER NOT NULL, -- Shuffle seed. The same seed always deals the same order inside each band.
    sitting_no INTEGER NOT NULL DEFAULT 1 CHECK (sitting_no >= 1) -- Current sitting. Start and ContinueSitting advance it when the previous sitting is full.
);

-- Latest run for a learner: one user, newest finish first.
CREATE INDEX placement_runs_user_finished ON placement_runs (user_id, finished_at);

-- One card in the deck. item_id is the fixture id.
-- rating is null while the card is face up and unanswered.
CREATE TABLE placement_answers (
    run_id INTEGER NOT NULL REFERENCES placement_runs (id) ON DELETE CASCADE, -- Run. Deleting the run deletes its answers.
    item_id TEXT NOT NULL, -- Fixture id. Not a lexeme id.
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 0 AND 4), -- 0 I don't know, 1 Again, 2 Hard, 3 Good, 4 Easy. Null until the learner rates the card. 0 is graded as Again.
    shown_at TEXT NOT NULL, -- UTC time the card was dealt, from the process clock.
    answered_at TEXT, -- UTC time the rating was stored. Null while the card is unanswered.
    lexeme_id INTEGER REFERENCES lexemes (id), -- Word this rating graded. Null until the card is rated.
    sitting_no INTEGER NOT NULL DEFAULT 1 CHECK (sitting_no >= 1), -- Sitting that dealt this card.
    PRIMARY KEY (run_id, item_id)
);

-- The wrong option the learner picked, as pointed NFC. Null when they did not pick one.
ALTER TABLE review_log ADD COLUMN chosen TEXT;

-- The word a confusion was about. Null when the miss is not tied to a lexeme. 0 is stored as NULL.
ALTER TABLE confusion_misses ADD COLUMN lexeme_id INTEGER REFERENCES lexemes (id);

-- shared is the course. learner:<id> is that learner's own unit.
ALTER TABLE units ADD COLUMN scope TEXT NOT NULL DEFAULT 'shared' CHECK (scope = 'shared' OR scope GLOB 'learner:[0-9]*');

CREATE INDEX units_scope_position ON units (scope, position);
