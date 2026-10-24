-- Remember which line of a your-turn dialogue the learner is on (INIT-020/SPEC-004).
-- Additive: existing rows start at line 0 with no side chosen. There is no down migration.

ALTER TABLE lesson_runs ADD COLUMN line_ix INTEGER NOT NULL DEFAULT 0 CHECK (line_ix >= 0);
ALTER TABLE lesson_runs ADD COLUMN side TEXT NOT NULL DEFAULT '' CHECK (side IN ('', 'a', 'b'));
