-- Class sheets, flags, lesson type, and run ids (INIT-039/SPEC-002). Additive. ADR-0034.
-- A class sheet is one dated tutor class, stored as it was exported. Its lines keep the tutor's text and
-- the learner's text in separate columns and are never split on punctuation.
-- A flag takes one item out of lessons and review. It never edits or deletes an answer. The matching
-- void, when there is one, is a normal answer_voids row.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.
-- The new tables and columns can stay; a database with none of these rows reads as it did before this file.

-- One class sheet of one learner.
CREATE TABLE class_sheets (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their sheets.
    sheet_ref TEXT NOT NULL CHECK (typeof(sheet_ref) = 'text' AND length(sheet_ref) BETWEEN 1 AND 200), -- Opaque id of the source document, for example a Drive file id. A repeat import of the same ref changes nothing.
    class_on TEXT NOT NULL CHECK (date(class_on) IS NOT NULL AND date(class_on) = class_on), -- Civil day of the class, YYYY-MM-DD.
    title TEXT NOT NULL DEFAULT '' CHECK (length(title) <= 200), -- Title as exported. Shown as the lesson's source label.
    status TEXT NOT NULL DEFAULT 'held' CHECK (status IN ('held', 'passed', 'failed')), -- held has not been checked. Only a passed sheet feeds lessons.
    check_note TEXT NOT NULL DEFAULT '' CHECK (length(check_note) <= 1000), -- Why the sheet check passed or failed. Counts, never the tutor's text.
    created_at TEXT NOT NULL, -- UTC RFC3339 instant the sheet was imported.
    checked_at TEXT, -- UTC RFC3339 instant the sheet check last ran. Null until it has.
    UNIQUE (learner_id, sheet_ref)
);

-- One learner's sheets, newest class first.
CREATE INDEX class_sheets_learner_day ON class_sheets (learner_id, class_on DESC);

-- One line of a class sheet. Exactly one of tutor_text and learner_text is set.
CREATE TABLE class_lines (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    sheet_id INTEGER NOT NULL REFERENCES class_sheets (id) ON DELETE CASCADE, -- Sheet. Deleting the sheet deletes its lines.
    position INTEGER NOT NULL CHECK (position >= 0), -- Zero-based order in the sheet.
    tutor_text TEXT CHECK (tutor_text IS NULL OR length(tutor_text) BETWEEN 1 AND 2000), -- The tutor's line, verbatim. Null when the learner wrote the line.
    learner_text TEXT CHECK (learner_text IS NULL OR length(learner_text) BETWEEN 1 AND 2000), -- The learner's own typed line, verbatim. Null when the tutor wrote the line.
    gloss_en TEXT CHECK (gloss_en IS NULL OR length(gloss_en) BETWEEN 1 AND 500), -- English shown beside the line in the sheet. Null when the sheet shows none. A key needs this.
    has_points INTEGER NOT NULL DEFAULT 0 CHECK (has_points IN (0, 1)), -- 1 when the line carries vowel points.
    state TEXT NOT NULL DEFAULT 'ok' CHECK (state IN ('ok', 'held')), -- held keeps a line out of lessons until it is fixed.
    held_reason TEXT NOT NULL DEFAULT '' CHECK (length(held_reason) <= 500), -- Why the line is held. Empty when state is ok.
    CHECK ((tutor_text IS NOT NULL) <> (learner_text IS NOT NULL)),
    CHECK (state = 'ok' OR length(held_reason) >= 1),
    UNIQUE (sheet_id, position)
);

-- Lines of one sheet, in order.
CREATE INDEX class_lines_sheet ON class_lines (sheet_id, position);

-- A word taught outside the app now says which sheet it came from. Null for rows written before this file.
ALTER TABLE taught_words ADD COLUMN sheet_id INTEGER REFERENCES class_sheets (id) ON DELETE SET NULL;

CREATE INDEX taught_words_sheet ON taught_words (sheet_id) WHERE sheet_id IS NOT NULL;

-- A code-built lesson says which kind it is. Null for every hand-written unit.
ALTER TABLE units ADD COLUMN lesson_type TEXT CHECK (lesson_type IS NULL OR lesson_type IN ('word', 'easy'));

-- An answer says which lesson run it came from, so a resumed lesson still groups. Null for every earlier answer.
ALTER TABLE attempts ADD COLUMN run_id INTEGER REFERENCES lesson_runs (id) ON DELETE SET NULL;

CREATE INDEX attempts_run ON attempts (run_id) WHERE run_id IS NOT NULL;

-- One item taken out of lessons and review for one learner. undone_at null means it is out.
CREATE TABLE item_flags (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    learner_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their flags.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- The word the item asked about. A flagged word cannot be deleted.
    attempt_id INTEGER REFERENCES attempts (id) ON DELETE SET NULL, -- The answer that was flagged. Null when the flag was set outside an answer.
    void_batch_id TEXT CHECK (void_batch_id IS NULL OR length(void_batch_id) BETWEEN 1 AND 200), -- The answer_voids batch the same tap wrote. Undo reads it. Null when no grade was taken back.
    reason TEXT NOT NULL CHECK (reason IN ('also_right', 'wrong_question', 'bad_item')), -- also_right: a miss that was right. wrong_question: a right answer to a bad question. bad_item: anything else.
    note TEXT NOT NULL DEFAULT '' CHECK (length(note) <= 500), -- Optional words from the learner. Learner input: escape it on output.
    created_at TEXT NOT NULL, -- UTC RFC3339 instant of the tap.
    undone_at TEXT CHECK (undone_at IS NULL OR length(undone_at) >= 1) -- UTC RFC3339 instant of Undo. Null while the item is out.
);

-- Items a learner has out, for the queue filter.
CREATE INDEX item_flags_active ON item_flags (learner_id, lexeme_id) WHERE undone_at IS NULL;

-- One batch, for undo.
CREATE INDEX item_flags_batch ON item_flags (void_batch_id) WHERE void_batch_id IS NOT NULL;
