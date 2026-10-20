-- Learner list reads one user's attempts newest first:
--   ORDER BY lexeme_id DESC, skill DESC, created_at DESC, id DESC
-- and it also reads correct. attempts_user_lexeme_skill ends at created_at, so
-- that ORDER BY made SQLite sort every row and then look up correct in the
-- table. This index holds the sort key and correct. The scan names it with
-- INDEXED BY and walks it backwards. Nothing else is changed. A database at
-- 011 keeps every row. The runner applies this file in one transaction.
-- There is no down migration. To roll back, restore the pre-migration file,
-- or: DROP INDEX attempts_user_lexeme_skill_id;

CREATE INDEX attempts_user_lexeme_skill_id ON attempts (user_id, lexeme_id, skill, created_at, id, correct);
