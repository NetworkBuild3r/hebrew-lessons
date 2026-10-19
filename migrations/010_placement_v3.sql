-- Placement engine v3 (INIT-015/SPEC-007). Applied after 009_daily_job.sql.
-- Two new tables, and nothing else. No existing table, column, index, or row is changed, so a database
-- at 009 keeps every v2 placement run and answer exactly as it was. A run is a v3 run when it has a
-- row in placement_v3_runs. A v2 run has none and is still read through placement_runs and placement_answers.
-- The runner applies this file in one transaction. If any statement fails, the database stays at 009.
-- There is no down migration. To roll back, restore the pre-migration SQLite file, or drop both tables:
-- nothing else refers to them.

-- The v3 side of a placement run. One row per v3 run.
CREATE TABLE placement_v3_runs (
    run_id INTEGER PRIMARY KEY REFERENCES placement_runs (id) ON DELETE CASCADE, -- The run this row describes. Deleting the run deletes this row.
    kind TEXT NOT NULL CHECK (kind IN ('initial', 'recheck')), -- initial is the full test. recheck is a short sitting of one band.
    recheck_skill TEXT CHECK (recheck_skill IS NULL OR recheck_skill IN ('listen', 'read')), -- Skill a recheck tests. Null for an initial run.
    recheck_band INTEGER CHECK (recheck_band IS NULL OR recheck_band BETWEEN 1 AND 5), -- Band a recheck tests, 1 to 5 (the deck's bands 0 to 4, plus one). Null for an initial run.
    phase TEXT NOT NULL DEFAULT 'parts' CHECK (phase IN ('parts', 'typing_offer', 'typing')), -- parts is letters, listening, and reading. typing_offer waits for the learner to take or skip the typing part. typing is the typing part.
    CHECK ((kind = 'recheck') = (recheck_skill IS NOT NULL AND recheck_band IS NOT NULL)) -- A recheck names its skill and band. An initial run names neither.
);

-- One item dealt in a v3 run, from the first to the last. The server deals the item and keeps the right option.
CREATE TABLE placement_items (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    run_id INTEGER NOT NULL REFERENCES placement_runs (id) ON DELETE CASCADE, -- Run. Deleting the run deletes its items.
    seq INTEGER NOT NULL CHECK (seq >= 1), -- Order the item was dealt in the run, from 1. The client sees it as the item id.
    part TEXT NOT NULL CHECK (part IN ('letters', 'listening', 'reading', 'typing')), -- Part of the test the item belongs to.
    skill TEXT CHECK (skill IS NULL OR skill IN ('listen', 'read', 'recall')), -- Skill the item exercised. Null for a letters item, which names no word.
    band INTEGER CHECK (band IS NULL OR band BETWEEN 1 AND 5), -- Band of the word, 1 to 5. Null for a letters item.
    item_id TEXT NOT NULL, -- Fixture id of the word, or of the letter. Not a lexeme id.
    options TEXT NOT NULL CHECK (json_valid(options)), -- JSON object {audio, options}. audio is the clip id, when the item plays one. options lists the choices in the order shown, each {id, text, item}, and is empty for a typing item.
    correct_idx INTEGER CHECK (correct_idx IS NULL OR correct_idx >= 0), -- Index in options of the right one. Never sent to the client. Null for a typing item.
    outcome TEXT CHECK (outcome IS NULL OR outcome IN ('right', 'wrong', 'dont_know')), -- Result of the answer. Null while the item is open. A claimed answer is never changed.
    picked TEXT, -- What the learner picked or typed. Learner input: escape it on output and show it only to that learner. Null while the item is open.
    lexeme_id INTEGER REFERENCES lexemes (id), -- Lexicon row of the word. Null for a letters item, and while the item is open.
    shown_at TEXT NOT NULL, -- UTC time the item was dealt, from the process clock.
    answered_at TEXT, -- UTC time the answer was stored. Null while the item is open.
    latency_ms INTEGER CHECK (latency_ms IS NULL OR latency_ms >= 0), -- Milliseconds from dealing to the answer. Null while the item is open.
    UNIQUE (run_id, seq),
    CHECK ((outcome IS NULL) = (answered_at IS NULL)) -- An item has an outcome exactly when it has an answer time.
);

-- No word is asked twice in listening and reading in one run. The typing part asks words again, on purpose, so it is left out.
CREATE UNIQUE INDEX placement_items_no_repeat ON placement_items (run_id, item_id) WHERE part IN ('letters', 'listening', 'reading');
