-- Let a say step record a self-check (INIT-020/SPEC-002).
-- Additive in effect: every existing attempts row is copied, and source gains 'self'.
-- SQLite cannot change a CHECK, so the table is rebuilt. There is no down migration.
-- To roll back, restore the pre-migration SQLite file. The new source stays unused until a say step is graded.

CREATE TABLE attempts_new (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT,
    skill TEXT NOT NULL CHECK (skill IN ('listen', 'read', 'recall', 'spell')),
    activity TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('placement', 'review', 'lesson', 'practice', 'self')),
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    correct INTEGER NOT NULL CHECK (correct IN (0, 1)),
    dont_know INTEGER NOT NULL DEFAULT 0 CHECK (dont_know IN (0, 1)),
    hint_used INTEGER NOT NULL DEFAULT 0 CHECK (hint_used IN (0, 1)),
    latency_ms INTEGER NOT NULL CHECK (latency_ms >= 0),
    created_at TEXT NOT NULL
);

INSERT INTO attempts_new (
    id, user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
    correct, dont_know, hint_used, latency_ms, created_at
)
SELECT
    id, user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
    correct, dont_know, hint_used, latency_ms, created_at
FROM attempts;

DROP TABLE attempts;

ALTER TABLE attempts_new RENAME TO attempts;

CREATE INDEX attempts_user_lexeme_skill ON attempts (user_id, lexeme_id, skill, created_at);
CREATE INDEX attempts_user_created ON attempts (user_id, created_at);
CREATE INDEX attempts_user_lexeme_skill_id ON attempts (user_id, lexeme_id, skill, created_at, id, correct);
