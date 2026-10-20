-- Learner list reads one user's attempts newest first:
--   ORDER BY lexeme_id DESC, skill DESC, created_at DESC, id DESC
-- attempts_user_lexeme_skill ends at created_at, so that ORDER BY made SQLite
-- sort every row. This index is the same key plus id. The scan walks it
-- backwards and does not sort. Nothing else is changed. A database at 011
-- keeps every row. The runner applies this file in one transaction.
-- There is no down migration. To roll back, restore the pre-migration file,
-- or: DROP INDEX attempts_user_lexeme_skill_id;

CREATE INDEX attempts_user_lexeme_skill_id ON attempts (user_id, lexeme_id, skill, created_at, id);
