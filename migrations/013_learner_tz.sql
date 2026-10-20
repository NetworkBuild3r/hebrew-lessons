-- Learner time zone for the study day.
-- Applied after 012_attempt_list_index.sql.
--
-- The study day rolls over at 04:00 in this IANA zone.
-- Existing rows and new inserts use America/Denver until a setter changes the column.

ALTER TABLE users ADD COLUMN tz TEXT NOT NULL DEFAULT 'America/Denver';
