-- Onboarding and placement history. Applied after 012_attempt_list_index.sql.
-- Additive: nullable columns, one new index, and one UPDATE that fills
-- users.onboarded_at where it is still null and a qualifying time exists.
-- No table is rebuilt. A database at 012 or 013 keeps every user and every
-- placement run. start_unit and words_known stay NULL on rows that already
-- existed. A binary that does not know these columns still reads the old
-- column lists.
-- The runner applies this file in one transaction. There is no down migration.
-- To roll back, restore the pre-migration SQLite file, or:
--   DROP INDEX placement_runs_user_finished_desc;
--   ALTER TABLE placement_runs DROP COLUMN words_known;
--   ALTER TABLE placement_runs DROP COLUMN start_unit;
--   ALTER TABLE users DROP COLUMN onboarded_at;
--   DELETE FROM schema_migrations WHERE version = 14;

-- UTC RFC3339 time onboarding was recorded. Null until the backfill below, SetOnboarded, or a v3 finish.
ALTER TABLE users ADD COLUMN onboarded_at TEXT;

-- Course position stored when a v3 run finishes. Null on older rows and on runs that never finished.
ALTER TABLE placement_runs ADD COLUMN start_unit INTEGER;

-- Distinct lexemes rated Good or Easy in the run, stored when it finishes and not recomputed.
-- Null on older rows and on runs that never finished.
ALTER TABLE placement_runs ADD COLUMN words_known INTEGER;

-- Newest finished run for a learner. The older index is (user_id, finished_at) ascending.
CREATE INDEX placement_runs_user_finished_desc ON placement_runs (user_id, finished_at DESC);

-- One-time backfill. The earliest time wins.
-- A placement counts when finished_at is set, except the v2 run Engine.Start
-- closes in resumeOpen: level -1, stop_reason NULL, result
-- '{"bands":[],"estimated_known_words":0}', and no rating or answered item.
-- A lesson counts as lesson_runs.started_at or an attempts row whose source
-- is not placement. Rows that have neither stay NULL.
UPDATE users
SET onboarded_at = (
    SELECT MIN(t) FROM (
        SELECT MIN(r.finished_at) AS t
        FROM placement_runs AS r
        WHERE r.user_id = users.id
          AND r.finished_at IS NOT NULL
          AND NOT (
            r.level = -1
            AND r.stop_reason IS NULL
            AND r.result = '{"bands":[],"estimated_known_words":0}'
            AND NOT EXISTS (SELECT 1 FROM placement_answers AS a WHERE a.run_id = r.id AND a.rating IS NOT NULL)
            AND NOT EXISTS (SELECT 1 FROM placement_items AS i WHERE i.run_id = r.id AND i.outcome IS NOT NULL)
          )
        UNION ALL
        SELECT MIN(started_at) FROM lesson_runs WHERE user_id = users.id
        UNION ALL
        SELECT MIN(created_at) FROM attempts WHERE user_id = users.id AND source <> 'placement'
    )
)
WHERE onboarded_at IS NULL
  AND (
    EXISTS (
        SELECT 1 FROM placement_runs AS r
        WHERE r.user_id = users.id
          AND r.finished_at IS NOT NULL
          AND NOT (
            r.level = -1
            AND r.stop_reason IS NULL
            AND r.result = '{"bands":[],"estimated_known_words":0}'
            AND NOT EXISTS (SELECT 1 FROM placement_answers AS a WHERE a.run_id = r.id AND a.rating IS NOT NULL)
            AND NOT EXISTS (SELECT 1 FROM placement_items AS i WHERE i.run_id = r.id AND i.outcome IS NOT NULL)
          )
    )
    OR EXISTS (SELECT 1 FROM lesson_runs WHERE user_id = users.id)
    OR EXISTS (SELECT 1 FROM attempts WHERE user_id = users.id AND source <> 'placement')
  );
