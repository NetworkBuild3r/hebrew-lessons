-- Learner-model tables (INIT-015/SPEC-004). Applied after 006_evidence.sql.
-- These seven tables hold what the learner model keeps beyond attempts, exposures, and cards:
-- daily history, state-change events, lesson briefs, coach notes, the publish audit trail,
-- learner reports about a unit, and weekly reports.
-- Every table is new. No existing table is touched, so nothing here can change an existing row.
-- Nothing reads or writes these tables yet. SPEC-013 and later specs do that.
-- The runner applies this file in one transaction. If any statement fails, the database stays at 006.
-- There is no down migration. To roll back, restore the pre-migration SQLite file, or drop the seven tables.
-- The state vocabulary is the learner states of docs/north-star.md:
-- unseen, seen, learning, struggling, known, solid, regressed.
-- Text written by a learner or a client is plain text with a length cap. Escape it on output.
-- The typeof checks keep a blob from slipping past a length check, which counts bytes for a blob.

-- One row per learner, per day, per skill, per state: how many words were in that state that day.
-- Written once a day. Progress reads it to draw history after the state rules change.
CREATE TABLE daily_snapshots (
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their snapshots.
    day TEXT NOT NULL CHECK (day GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]' AND date(day) IS NOT NULL AND date(day) = day), -- Calendar day, YYYY-MM-DD, a real date.
    skill TEXT NOT NULL CHECK (skill IN ('listen', 'read', 'recall', 'spell')), -- Skill the count is for.
    state TEXT NOT NULL CHECK (state IN ('unseen', 'seen', 'learning', 'struggling', 'known', 'solid', 'regressed')), -- Learner state of the words counted.
    count INTEGER NOT NULL CHECK (count >= 0), -- How many words were in this state for this skill on this day.
    est_known REAL NOT NULL CHECK (est_known >= 0), -- Estimated words known in this skill on this day. A model estimate, so it can be fractional.
    retention REAL CHECK (retention IS NULL OR (retention >= 0 AND retention <= 1)), -- Estimated share of known words still recalled, 0 to 1. Null when there is not enough data.
    PRIMARY KEY (user_id, day, skill, state)
);

-- History of one learner for one skill, by day.
CREATE INDEX daily_snapshots_user_skill_day ON daily_snapshots (user_id, skill, day);

-- One time a word changed state, kept so a regression can be shown and explained.
CREATE TABLE regression_events (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their events.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word that changed state. A lexeme with events cannot be deleted, as with attempts.
    skill TEXT NOT NULL CHECK (skill IN ('listen', 'read', 'recall', 'spell')), -- Skill the change is for.
    from_state TEXT NOT NULL CHECK (from_state IN ('unseen', 'seen', 'learning', 'struggling', 'known', 'solid', 'regressed')), -- State before the change.
    to_state TEXT NOT NULL CHECK (to_state IN ('unseen', 'seen', 'learning', 'struggling', 'known', 'solid', 'regressed') AND to_state <> from_state), -- State after the change. It differs from the state before.
    day TEXT NOT NULL CHECK (day GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]' AND date(day) IS NOT NULL AND date(day) = day), -- Calendar day of the change, YYYY-MM-DD, a real date.
    cause TEXT NOT NULL CHECK (typeof(cause) = 'text' AND length(cause) >= 1) -- Short reason code or text that explains the change. Not empty.
);

-- Events of one learner, by day.
CREATE INDEX regression_events_user_day ON regression_events (user_id, day);

-- History of one learner on one word and skill.
CREATE INDEX regression_events_user_lexeme_skill ON regression_events (user_id, lexeme_id, skill);

-- A brief for the next lesson, built from the learner model and handed to the lesson author.
-- The id is random so a brief id cannot be guessed or counted over MCP.
CREATE TABLE lesson_briefs (
    id TEXT PRIMARY KEY NOT NULL CHECK (typeof(id) = 'text' AND length(id) BETWEEN 22 AND 64), -- Random 128-bit id as text. 22 characters in base64url, 32 in hex. Never a counter.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner the brief is for. Deleting the user deletes their briefs.
    theme TEXT NOT NULL CHECK (typeof(theme) = 'text' AND length(theme) BETWEEN 1 AND 200), -- Theme chosen for the lesson. Plain text, 200 characters at most.
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)), -- The brief itself as JSON: words to teach, words to review, and limits.
    created_at TEXT NOT NULL, -- UTC RFC3339 time the brief was made.
    used_by_unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL -- Unit built from this brief. Null until a unit uses it. Deleting the unit clears this.
);

-- Briefs of one learner, newest last.
CREATE INDEX lesson_briefs_user_created ON lesson_briefs (user_id, created_at);

-- A note that a client, for example the weekly coach, leaves for one learner.
CREATE TABLE coach_notes (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner the note is about. Deleting the user deletes their notes.
    author TEXT NOT NULL CHECK (typeof(author) = 'text' AND length(author) BETWEEN 1 AND 100), -- Name of the client that wrote the note. Plain text, 100 characters at most.
    body TEXT NOT NULL CHECK (typeof(body) = 'text' AND length(body) BETWEEN 1 AND 500), -- Note text. Plain text, 1 to 500 characters. Escape it on output.
    created_at TEXT NOT NULL -- UTC RFC3339 time the note was written.
);

-- Notes of one learner, newest last.
CREATE INDEX coach_notes_user_created ON coach_notes (user_id, created_at);

-- The record of every automatic publish, unpublish, and learner-triggered hide. It replaces human approval.
-- The row stays when its unit is deleted, with unit_id and brief_id set to null.
CREATE TABLE publish_audit (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL, -- Unit the action was on. Deleting the unit keeps the audit row and clears this.
    revision INTEGER NOT NULL CHECK (revision >= 1), -- Unit revision the action applied to.
    scope TEXT NOT NULL CHECK (typeof(scope) = 'text' AND length(scope) >= 1), -- Scope the unit was in, for example learner:7 or global.
    actor TEXT NOT NULL CHECK (typeof(actor) = 'text' AND length(actor) >= 1), -- Who acted: a client name, or the learner for a report.
    action TEXT NOT NULL CHECK (action IN ('auto_publish', 'unpublish', 'report_hide')), -- What was done.
    brief_id TEXT REFERENCES lesson_briefs (id) ON DELETE SET NULL, -- Brief the unit was checked against. Null when there was none. Deleting the brief clears this.
    issues_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(issues_json)), -- Validator issues found at the time, as JSON. An empty list when clean.
    created_at TEXT NOT NULL -- UTC RFC3339 time of the action.
);

-- Trail of one unit, oldest first.
CREATE INDEX publish_audit_unit_created ON publish_audit (unit_id, created_at);

-- Recent actions across all units.
CREATE INDEX publish_audit_created ON publish_audit (created_at);

-- A learner's "Report a problem" on a unit.
CREATE TABLE unit_reports (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    unit_id INTEGER NOT NULL REFERENCES units (id) ON DELETE CASCADE, -- Unit that was reported. Deleting the unit deletes its reports.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner who reported. Deleting the user deletes their reports.
    reason TEXT NOT NULL CHECK (typeof(reason) = 'text' AND length(reason) BETWEEN 1 AND 100), -- Reason the learner gave, or a reason code. Plain text, 100 characters at most.
    note TEXT NOT NULL DEFAULT '' CHECK (typeof(note) = 'text' AND length(note) <= 500), -- Free text from the learner. Plain text, 500 characters at most, may be empty. Escape it on output.
    created_at TEXT NOT NULL -- UTC RFC3339 time of the report.
);

-- Reports on one unit.
CREATE INDEX unit_reports_unit ON unit_reports (unit_id, created_at);

-- Reports by one learner.
CREATE INDEX unit_reports_user ON unit_reports (user_id, created_at);

-- One report per learner per week: the data and a short summary.
CREATE TABLE weekly_reports (
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their reports.
    week_start TEXT NOT NULL CHECK (week_start GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]' AND date(week_start) IS NOT NULL AND date(week_start) = week_start), -- First day of the week, YYYY-MM-DD, a real date.
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)), -- The full report as JSON.
    summary TEXT NOT NULL CHECK (typeof(summary) = 'text' AND length(summary) <= 600), -- Short plain-text summary, 600 characters at most. Escape it on output.
    created_at TEXT NOT NULL, -- UTC RFC3339 time the report was made.
    PRIMARY KEY (user_id, week_start)
);

-- Reports on one learner are read by their primary key, user_id then week_start, so no extra index is needed.

-- Reports any row whose parent is missing. The runner discards the rows. These tables are new, so none can fail.
PRAGMA foreign_key_check;
