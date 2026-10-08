/**
 * Repository prepared-statement / injection-safety tests.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const HOSTILE = "'; DROP TABLE users; --";

describe('db repos prepared statements (ac-4)', () => {
    let tmp;
    let db;
    let users;
    let sessions;
    let review;
    let events;
    let lessonProgress;
    let visibility;
    let audit;
    let userId;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-repos-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/migrations/001-initial')];
        for (const name of [
            'users', 'sessions', 'review', 'events',
            'lesson-progress', 'visibility', 'audit'
        ]) {
            delete require.cache[require.resolve('../server/db/repos/' + name)];
        }

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        users = require('../server/db/repos/users');
        sessions = require('../server/db/repos/sessions');
        review = require('../server/db/repos/review');
        events = require('../server/db/repos/events');
        lessonProgress = require('../server/db/repos/lesson-progress');
        visibility = require('../server/db/repos/visibility');
        audit = require('../server/db/repos/audit');

        const created = users.create({
            username: 'SafeUser',
            password_hash: 'hash',
            display_name: 'Safe'
        });
        userId = Number(created.lastInsertRowid);
        const stored = users.findByUsername('SafeUser');
        assert.equal(stored.username, 'safeuser');
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    it('stores injection-style input literally across repos', () => {
        const now = '2026-01-02T00:00:00.000Z';

        const hostileUser = users.create({
            username: HOSTILE,
            password_hash: HOSTILE,
            display_name: HOSTILE
        });
        const hostileId = Number(hostileUser.lastInsertRowid);
        const loadedUser = users.findById(hostileId);
        assert.equal(loadedUser.username, HOSTILE.toLowerCase());
        assert.equal(loadedUser.password_hash, HOSTILE);
        assert.equal(loadedUser.display_name, HOSTILE);

        sessions.create({
            user_id: userId,
            token_hash: HOSTILE,
            created_at: now,
            last_seen_at: now,
            expires_at: now
        });
        assert.equal(sessions.findByTokenHash(HOSTILE).token_hash, HOSTILE);

        review.upsert({
            user_id: userId,
            word: HOSTILE,
            direction: 'he_en',
            due: now,
            step: 0,
            streak: 0,
            lapses: 0,
            last_result: HOSTILE,
            last_at: now,
            sources: JSON.stringify([HOSTILE])
        });
        const item = review.find(userId, HOSTILE, 'he_en');
        assert.equal(item.word, HOSTILE);
        assert.equal(item.last_result, HOSTILE);

        events.insert({
            id: HOSTILE,
            user_id: userId,
            word: HOSTILE,
            direction: 'en_he',
            result: HOSTILE,
            at: now,
            slug: HOSTILE,
            activity: HOSTILE
        });
        const evt = events.find(userId, HOSTILE);
        assert.equal(evt.id, HOSTILE);
        assert.equal(evt.word, HOSTILE);
        assert.equal(evt.slug, HOSTILE);

        lessonProgress.upsert({
            user_id: userId,
            slug: HOSTILE,
            state_json: JSON.stringify({ note: HOSTILE }),
            rev: 1,
            updated_at: now
        });
        const progress = lessonProgress.find(userId, HOSTILE);
        assert.equal(progress.slug, HOSTILE);
        assert.match(progress.state_json, /DROP TABLE/);

        visibility.setVisibility(HOSTILE, 'hidden', now);
        assert.equal(visibility.getVisibility(HOSTILE).slug, HOSTILE);
        visibility.grant(HOSTILE, userId);
        assert.equal(visibility.listGrants(HOSTILE)[0].slug, HOSTILE);

        audit.append({
            actor: HOSTILE,
            action: HOSTILE,
            target: HOSTILE,
            at: now,
            detail: HOSTILE
        });
        const logs = audit.listByActor(HOSTILE, 10);
        assert.equal(logs.length, 1);
        assert.equal(logs[0].action, HOSTILE);
        assert.equal(logs[0].detail, HOSTILE);

        // Tables still exist — injection did not execute.
        const tables = db.all(
            `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'users'`
        );
        assert.equal(tables.length, 1);
        assert.ok(users.findByUsername('SafeUser'));
    });
});
