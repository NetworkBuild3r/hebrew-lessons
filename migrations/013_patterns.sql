-- Grammar patterns, the prerequisites between them, and one explainer per pattern.
-- A unit may name the pattern it teaches and the worksheet shape it follows.
-- Pattern and explainer writes are logged on the publish audit, so that
-- table's action check is widened. SQLite cannot change a check in place,
-- so the audit table is rebuilt and every existing row is copied.
-- Additive. A database at 012 keeps every row.
-- There is no down migration. To roll back, restore the pre-migration file.
-- The new tables can stay; nothing reads them until this revision of the app.

CREATE TABLE grammar_patterns (
    slug TEXT PRIMARY KEY CHECK (
        length(slug) BETWEEN 1 AND 60
        AND slug NOT GLOB '*[^a-z0-9-]*'
    ), -- Stable id, used in URLs and on units.
    title_en TEXT NOT NULL CHECK (length(title_en) BETWEEN 1 AND 200), -- English name.
    summary_en TEXT NOT NULL CHECK (length(summary_en) BETWEEN 1 AND 300), -- One sentence.
    example_he TEXT NOT NULL DEFAULT '' CHECK (length(example_he) <= 200), -- Optional pointed example.
    created_at TEXT NOT NULL, -- UTC RFC3339.
    updated_at TEXT NOT NULL -- UTC RFC3339.
);

-- An edge means pattern_slug cannot be taught before requires_slug.
-- A row that points at itself is refused. A cycle is refused by the store,
-- which rolls the write back. The check here only catches a self-edge.
CREATE TABLE pattern_prereqs (
    pattern_slug TEXT NOT NULL REFERENCES grammar_patterns (slug) ON DELETE CASCADE,
    requires_slug TEXT NOT NULL REFERENCES grammar_patterns (slug) ON DELETE CASCADE,
    PRIMARY KEY (pattern_slug, requires_slug),
    CHECK (pattern_slug <> requires_slug)
);

-- One explainer per pattern. body is restricted markdown, checked by the
-- writer before it is stored. status draft is not shown to a learner.
CREATE TABLE explainers (
    pattern_slug TEXT PRIMARY KEY REFERENCES grammar_patterns (slug) ON DELETE CASCADE,
    title_en TEXT NOT NULL CHECK (length(title_en) BETWEEN 1 AND 200),
    body TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 20000),
    status TEXT NOT NULL CHECK (status IN ('draft', 'published')),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    updated_at TEXT NOT NULL
);

-- Nullable. An unknown slug is refused by the foreign key.
ALTER TABLE units ADD COLUMN pattern_slug TEXT REFERENCES grammar_patterns (slug);

-- Empty means the unit has no worksheet shape. The five names are the
-- class worksheet types. Folded into this migration because 013 had not
-- landed when the worksheet column was specified.
ALTER TABLE units ADD COLUMN worksheet_type TEXT NOT NULL DEFAULT '' CHECK (
    worksheet_type IN (
        '',
        'topic-conversation',
        'review-sheet',
        'verb-workshop',
        'short-story',
        'foundations-reading'
    )
);

CREATE INDEX units_pattern ON units (pattern_slug);

CREATE TABLE publish_audit_new (
    id INTEGER PRIMARY KEY,
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    scope TEXT NOT NULL CHECK (typeof(scope) = 'text' AND length(scope) >= 1),
    actor TEXT NOT NULL CHECK (typeof(actor) = 'text' AND length(actor) >= 1),
    action TEXT NOT NULL CHECK (action IN ('auto_publish', 'unpublish', 'report_hide', 'upsert_patterns', 'upsert_explainer')),
    brief_id TEXT REFERENCES lesson_briefs (id) ON DELETE SET NULL,
    issues_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(issues_json)),
    created_at TEXT NOT NULL
);

INSERT INTO publish_audit_new (id, unit_id, revision, scope, actor, action, brief_id, issues_json, created_at)
SELECT id, unit_id, revision, scope, actor, action, brief_id, issues_json, created_at FROM publish_audit;

DROP TABLE publish_audit;

ALTER TABLE publish_audit_new RENAME TO publish_audit;

CREATE INDEX publish_audit_unit_created ON publish_audit (unit_id, created_at);

CREATE INDEX publish_audit_created ON publish_audit (created_at);
