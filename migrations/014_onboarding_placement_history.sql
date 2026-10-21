-- Onboarding and placement history. Applied after 012_attempt_list_index.sql.
-- Additive only: nullable columns and one new index. No table is rebuilt, and no
-- existing column, index, or row is changed. A database at 012 or 013 keeps every
-- user and every placement run. Old rows get NULL in the new columns. A binary
-- that does not know these columns still reads the old column lists.
-- The runner applies this file in one transaction. There is no down migration.
-- To roll back, restore the pre-migration SQLite file, or:
--   DROP INDEX placement_runs_user_finished_desc;
--   ALTER TABLE placement_runs DROP COLUMN words_known;
--   ALTER TABLE placement_runs DROP COLUMN start_unit;
--   ALTER TABLE users DROP COLUMN onboarded_at;

-- UTC RFC3339 time the learner finished a placement for the first time. Null until then.
ALTER TABLE users ADD COLUMN onboarded_at TEXT;

-- Course position stored when a v3 run finishes. Null on older rows and on runs that never finished.
ALTER TABLE placement_runs ADD COLUMN start_unit INTEGER;

-- Distinct lexemes rated Good or Easy in the run, stored when it finishes and not recomputed.
-- Null on older rows and on runs that never finished.
ALTER TABLE placement_runs ADD COLUMN words_known INTEGER;

-- Newest finished run for a learner. The older index is (user_id, finished_at) ascending.
CREATE INDEX placement_runs_user_finished_desc ON placement_runs (user_id, finished_at DESC);
