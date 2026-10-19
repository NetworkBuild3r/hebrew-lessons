-- Where an open lesson run is, so the player can resume and score it.
-- Applied after 004_tanakh.sql. Existing rows start at the first activity.

-- Zero-based index into the lesson's activities array.
ALTER TABLE lesson_runs ADD COLUMN activity_ix INTEGER NOT NULL DEFAULT 0 CHECK (activity_ix >= 0);

-- How many activities were correct so far. The finished score is this divided by the activity count.
ALTER TABLE lesson_runs ADD COLUMN correct_n INTEGER NOT NULL DEFAULT 0 CHECK (correct_n >= 0);

-- ask is waiting for an answer. reveal is showing the word after an answer.
ALTER TABLE lesson_runs ADD COLUMN phase TEXT NOT NULL DEFAULT 'ask' CHECK (phase IN ('ask', 'reveal'));

-- 1 when the answer just graded was correct. Continue adds it to correct_n.
ALTER TABLE lesson_runs ADD COLUMN pending INTEGER NOT NULL DEFAULT 0 CHECK (pending IN (0, 1));
