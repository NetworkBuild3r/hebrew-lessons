-- History rules stamp and daily known-any totals (INIT-033/SPEC-002).
-- Additive. A database at 021 keeps every row.
-- There is no down migration. To roll back, restore the pre-migration file.
-- The new column and table can stay; nothing reads them until this revision of the app.
-- Snapshots are a derived cache. rule_version says which learner-model rules produced the counts (D3).

-- Which edition of the learner-model rules wrote this snapshot row. Existing rows are rule 1.
ALTER TABLE daily_snapshots ADD COLUMN rule_version INTEGER NOT NULL DEFAULT 1;

-- One row per learner per day: how many words were known or solid in any skill that day.
-- Written with the day's snapshots. Progress and rebuilds read it for the headline series.
CREATE TABLE daily_totals (
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their totals.
    day TEXT NOT NULL CHECK (day GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]' AND date(day) IS NOT NULL AND date(day) = day), -- Calendar day, YYYY-MM-DD, a real date.
    known_any_skill INTEGER NOT NULL CHECK (known_any_skill >= 0), -- Words known or solid in at least one skill, counted once.
    rule_version INTEGER NOT NULL CHECK (rule_version >= 1), -- Which edition of the learner-model rules wrote this total.
    PRIMARY KEY (user_id, day)
);

-- When the card row was created (INIT-033/SPEC-003 as-of replay), in the
-- db.FormatTime layout. New rows are stamped by the code that inserts them.
ALTER TABLE cards ADD COLUMN created_at TEXT;

-- Backfill: a reviewed card existed by its first review, so it takes that moment.
-- Exposures and attempts are not used: either can exist before the card does, and
-- backdating a card would show it as learning on days before it was made. A card with
-- no review has no recoverable creation moment, so it takes the time this migration
-- runs: it is left out of earlier days rather than invented into them, and kept in
-- later ones (a NULL would leave it out of every as-of read).
UPDATE cards
SET created_at = COALESCE(
    (SELECT MIN(rl.reviewed_at) FROM review_log AS rl WHERE rl.card_id = cards.id),
    strftime('%Y-%m-%dT%H:%M:%S', 'now') || '.000000000Z'
);
