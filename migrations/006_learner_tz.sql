-- Learner time zone for the study day.
-- Applied after 005_lesson_progress.sql. Version 7 may already be recorded;
-- this file does not depend on it.
--
-- The study day rolls over at 04:00 in this IANA zone.
-- Existing rows and new inserts use America/Denver until a setter changes the column.

ALTER TABLE users ADD COLUMN tz TEXT NOT NULL DEFAULT 'America/Denver';
