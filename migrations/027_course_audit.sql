-- Allow the course sync to be recorded (INIT-039/SPEC-024). Additive rebuild of publish_audit.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.

CREATE TABLE publish_audit_new (
    id INTEGER PRIMARY KEY,
    unit_id INTEGER REFERENCES units (id) ON DELETE SET NULL,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    scope TEXT NOT NULL CHECK (typeof(scope) = 'text' AND length(scope) >= 1),
    actor TEXT NOT NULL CHECK (typeof(actor) = 'text' AND length(actor) >= 1),
    action TEXT NOT NULL CHECK (action IN ('auto_publish', 'unpublish', 'report_hide', 'upsert_patterns', 'upsert_explainer', 'sync_course')),
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
