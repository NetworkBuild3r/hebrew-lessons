-- When the current lesson question was shown, so a meaning answer can store how long it took.
-- Additive: existing rows leave it null, and an untimed answer is not treated as fast. There is no down migration.

ALTER TABLE lesson_runs ADD COLUMN asked_at TEXT;
