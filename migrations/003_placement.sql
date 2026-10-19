-- Placement runs, the ratings, and words a learner already has.
-- Applied after 002_learning.sql.
-- Invented words are not stored here. They stay in the placement fixture.

-- One sitting of the placement deck.
CREATE TABLE placement_runs (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes the run.
    started_at TEXT NOT NULL, -- UTC time the run started, from the process clock.
    finished_at TEXT, -- UTC time the run was scored. Null while it is in progress.
    false_alarm_rate REAL CHECK (
        false_alarm_rate IS NULL OR (false_alarm_rate >= 0 AND false_alarm_rate <= 1)
    ), -- Good-or-Easy rate on the invented words that were shown. Null until the run is finished.
    reliable INTEGER NOT NULL DEFAULT 0 CHECK (reliable IN (0, 1)), -- 1 when a finished run may seed words and cards. 0 while in progress, and 0 when the false-alarm rate is above 0.25.
    level INTEGER CHECK (level IS NULL OR level BETWEEN -1 AND 4), -- Highest band whose Good-or-better rate is at least 0.5. -1 when a finished run passed none. Null until the run is finished.
    cap TEXT CHECK (cap IS NULL OR cap IN ('cards', 'time', 'band')), -- Why the deck stopped: 100 cards, 5 minutes, or a band under 50%. Null when the learner finished without a cap.
    result TEXT CHECK (result IS NULL OR json_valid(result)), -- JSON object with per-band counts and the known-word estimate. Null until the run is finished.
    seed INTEGER NOT NULL -- Shuffle seed. The same seed always deals the same band blocks.
);

-- Latest finished run for a learner: one user, newest finish first.
CREATE INDEX placement_runs_user_finished ON placement_runs (user_id, finished_at);

-- One card in the deck. item_id is the fixture id, including invented words.
-- rating is null while the card is face up and unanswered.
CREATE TABLE placement_answers (
    run_id INTEGER NOT NULL REFERENCES placement_runs (id) ON DELETE CASCADE, -- Run. Deleting the run deletes its answers.
    item_id TEXT NOT NULL, -- Fixture id. Not a lexeme id.
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 0 AND 4), -- 0 is "I don't know". 1 Again, 2 Hard, 3 Good, 4 Easy. Null until the learner rates the card.
    shown_at TEXT NOT NULL, -- UTC time the card was dealt, from the process clock.
    answered_at TEXT, -- UTC time the rating was stored. Null while the card is unanswered.
    PRIMARY KEY (run_id, item_id)
);

-- A word this learner already has, before or aside from review cards.
CREATE TABLE learner_lexemes (
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes these rows.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word. A lexeme a learner still has cannot be deleted.
    status TEXT NOT NULL CHECK (status IN ('known', 'learning')), -- known is Good or Easy. learning is Hard, Again, or "I don't know".
    seeded INTEGER NOT NULL DEFAULT 0 CHECK (seeded IN (0, 1)), -- 1 when a placement run wrote this row.
    source TEXT NOT NULL, -- Where the row came from. Placement writes placement.
    created_at TEXT NOT NULL, -- UTC time the row was inserted, from the process clock.
    PRIMARY KEY (user_id, lexeme_id)
);
