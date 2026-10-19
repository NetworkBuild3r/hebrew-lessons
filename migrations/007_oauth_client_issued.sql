-- Version 007. 006 is reserved.
-- Remembers that a dynamic client was issued an authorization code or a token.
-- Sweep deletes a registration that is still 0 about a day after created_at.
-- A client that has been issued anything keeps the 400-day lifetime.
-- Existing rows stay 0. Codes and tokens still in the tables are marked on the next sweep.
ALTER TABLE oauth_clients ADD COLUMN issued INTEGER NOT NULL DEFAULT 0 CHECK (issued IN (0, 1));
