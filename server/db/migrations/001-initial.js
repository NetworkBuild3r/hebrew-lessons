/**
 * Initial schema — ADR-0002 §2 tables and constraints.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    db.exec(`
        CREATE TABLE users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT NOT NULL COLLATE NOCASE,
            password_hash TEXT NOT NULL,
            display_name TEXT,
            role TEXT NOT NULL DEFAULT 'user'
                CHECK (role IN ('user', 'admin')),
            status TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'disabled')),
            auth_provider TEXT NOT NULL DEFAULT 'local',
            provider_subject TEXT,
            must_change_password INTEGER NOT NULL DEFAULT 0
                CHECK (must_change_password IN (0, 1)),
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            CHECK (username = lower(username)),
            UNIQUE (username),
            UNIQUE (auth_provider, provider_subject)
        );

        CREATE TABLE sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            token_hash TEXT NOT NULL UNIQUE,
            created_at TEXT NOT NULL,
            last_seen_at TEXT NOT NULL,
            expires_at TEXT NOT NULL
        );

        CREATE TABLE review_items (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            direction TEXT NOT NULL
                CHECK (direction IN ('he_en', 'en_he')),
            due TEXT NOT NULL,
            step INTEGER NOT NULL DEFAULT 0,
            streak INTEGER NOT NULL DEFAULT 0,
            lapses INTEGER NOT NULL DEFAULT 0,
            last_result TEXT,
            last_at TEXT,
            sources TEXT NOT NULL DEFAULT '[]',
            UNIQUE (user_id, word, direction)
        );
        CREATE INDEX idx_review_items_user_due ON review_items (user_id, due);

        CREATE TABLE practice_events (
            id TEXT NOT NULL,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            word TEXT NOT NULL,
            direction TEXT NOT NULL
                CHECK (direction IN ('he_en', 'en_he')),
            result TEXT NOT NULL,
            at TEXT NOT NULL,
            slug TEXT,
            activity TEXT,
            PRIMARY KEY (user_id, id)
        );
        CREATE INDEX idx_practice_events_user_at ON practice_events (user_id, at);

        CREATE TABLE lesson_progress (
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            slug TEXT NOT NULL,
            state_json TEXT NOT NULL,
            rev INTEGER NOT NULL DEFAULT 0,
            updated_at TEXT NOT NULL,
            PRIMARY KEY (user_id, slug),
            CHECK (length(state_json) <= 65536)
        );

        CREATE TABLE lesson_visibility (
            slug TEXT PRIMARY KEY,
            mode TEXT NOT NULL
                CHECK (mode IN ('hidden', 'everyone', 'selected')),
            updated_at TEXT NOT NULL
        );

        CREATE TABLE lesson_grants (
            slug TEXT NOT NULL,
            user_id INTEGER NOT NULL
                REFERENCES users(id) ON DELETE CASCADE,
            PRIMARY KEY (slug, user_id)
        );

        CREATE TABLE audit_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            actor TEXT NOT NULL,
            action TEXT NOT NULL,
            target TEXT,
            at TEXT NOT NULL,
            detail TEXT
        );

        CREATE TABLE meta (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL
        );
    `);
}

module.exports = { up };
