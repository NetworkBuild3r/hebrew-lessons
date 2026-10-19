-- Placement runs, the ratings, and words a learner already has.
-- Applied after 002_learning.sql.

-- One sitting of the placement deck.
CREATE TABLE placement_runs (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes the run.
    started_at TEXT NOT NULL, -- UTC time the run started, from the process clock.
    finished_at TEXT, -- UTC time the run was scored. Null while it is in progress.
    level INTEGER CHECK (level IS NULL OR level BETWEEN -1 AND 4), -- Highest band completed without tripping the struggle rule. -1 when a finished run completed none. Null until the run is finished.
    stop_reason TEXT CHECK (stop_reason IS NULL OR stop_reason IN ('band_stop', 'cap', 'exhausted')), -- Why the deck stopped. Null when the learner finished before a stop, and while the run is in progress.
    result TEXT CHECK (result IS NULL OR json_valid(result)), -- JSON object with per-band counts and the known-word estimate. Null until the run is finished.
    seed INTEGER NOT NULL -- Shuffle seed. The same seed always deals the same order inside each band.
);

-- Latest run for a learner: one user, newest finish first.
CREATE INDEX placement_runs_user_finished ON placement_runs (user_id, finished_at);

-- One card in the deck. item_id is the fixture id.
-- rating is null while the card is face up and unanswered.
CREATE TABLE placement_answers (
    run_id INTEGER NOT NULL REFERENCES placement_runs (id) ON DELETE CASCADE, -- Run. Deleting the run deletes its answers.
    item_id TEXT NOT NULL, -- Fixture id. Not a lexeme id.
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 4), -- 1 Again, 2 Hard, 3 Good, 4 Easy. Null until the learner rates the card.
    shown_at TEXT NOT NULL, -- UTC time the card was dealt, from the process clock.
    answered_at TEXT, -- UTC time the rating was stored. Null while the card is unanswered.
    PRIMARY KEY (run_id, item_id)
);

-- A word this learner already has, before or aside from review cards.
CREATE TABLE learner_lexemes (
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word. A lexeme a learner still has cannot be deleted.
    status TEXT NOT NULL CHECK (status IN ('known', 'learning')), -- known is Good or Easy. learning is Again or Hard.
    seeded INTEGER NOT NULL DEFAULT 0 CHECK (seeded IN (0, 1)), -- 1 when a placement rating wrote this row.
    source TEXT NOT NULL, -- Where the row came from. Placement writes placement.
    created_at TEXT NOT NULL, -- UTC time the row was inserted, from the process clock.
    PRIMARY KEY (user_id, lexeme_id)
);
