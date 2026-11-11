-- One optional story per learner and lesson type (INIT-039/SPEC-010). Additive.
-- The story is not a graded key. A failing story stays stored so the next call can show every check.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.

CREATE TABLE story_slots (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes the story.
    lesson_type TEXT NOT NULL CHECK (lesson_type IN ('word', 'easy')), -- Which lesson the story belongs to.
    body TEXT NOT NULL CHECK (typeof(body) = 'text' AND length(body) BETWEEN 1 AND 2000), -- The story text. Private to this learner.
    status TEXT NOT NULL CHECK (status IN ('passed', 'failed')), -- passed may be shown. failed is dropped when the lesson starts.
    failures TEXT NOT NULL DEFAULT '' CHECK (length(failures) <= 2000), -- Check ids from the last call, separated by spaces. Empty when the story passed.
    created_at TEXT NOT NULL, -- UTC RFC3339 instant of the last put.
    UNIQUE (learner_id, lesson_type)
);
