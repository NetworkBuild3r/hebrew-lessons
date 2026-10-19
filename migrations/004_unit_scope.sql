-- Scope is metadata. It is not a field of the unit JSON document.
-- shared is a unit every learner receives. learner:<id> is one learner's unit.
-- Existing rows are shared. put_unit writes the column on the unit and on the revision.

ALTER TABLE units ADD COLUMN scope TEXT NOT NULL DEFAULT 'shared';

ALTER TABLE unit_revisions ADD COLUMN scope TEXT NOT NULL DEFAULT 'shared';

CREATE INDEX units_scope_position ON units (scope, position, id);
