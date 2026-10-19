-- Applied after 005_lesson_progress.sql.
-- 006 adds users.tz and does not have to be applied first.
-- Remembers that a dynamic client was issued an authorization code or a token.
-- Sweep deletes a registration that is still 0 about a day after created_at.
-- A client that has been issued anything keeps the 400-day lifetime.
-- Existing rows are set to 1 so they keep that 400-day lifetime. Registrations after this migration start at 0.
ALTER TABLE oauth_clients ADD COLUMN issued INTEGER NOT NULL DEFAULT 0 CHECK (issued IN (0, 1));
UPDATE oauth_clients SET issued = 1;
