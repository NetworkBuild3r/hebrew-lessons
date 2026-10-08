/**
 * Session create / resolve / revoke / lifetime tests.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const crypto = require('crypto');

const root = path.resolve(__dirname, '..');

describe('auth/sessions (ac-2)', () => {
    let tmp;
    let db;
    let users;
    let sessions;
    let sessionsRepo;
    let userId;
    const t0 = new Date('2026-03-01T12:00:00.000Z');

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-sessions-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/repos/users')];
        delete require.cache[require.resolve('../server/db/repos/sessions')];
        delete require.cache[require.resolve('../server/auth/sessions')];

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        users = require('../server/db/repos/users');
        sessions = require('../server/auth/sessions');
        sessionsRepo = require('../server/db/repos/sessions');

        const created = users.create({
            username: 'sessuser',
            password_hash: 'x',
            role: 'user'
        });
        userId = Number(created.lastInsertRowid);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
    });

    beforeEach(() => {
        db.run('DELETE FROM sessions');
    });

    it('token is 256-bit random; only SHA-256 is stored', () => {
        const { token, tokenHash } = sessions.create(userId, { now: t0 });
        const raw = Buffer.from(token, 'base64url');
        assert.equal(raw.length, 32);
        assert.equal(tokenHash, sessions.hashToken(token));
        const row = sessionsRepo.findByTokenHash(tokenHash);
        assert.ok(row);
        assert.equal(row.token_hash, tokenHash);
        assert.equal(row.token_hash.includes(token), false);
        // Ensure DB row does not equal the raw token
        assert.notEqual(row.token_hash, token);
    });

    it('resolve returns user on day 29 with no intervening activity', () => {
        const { token } = sessions.create(userId, { now: t0 });
        const day29 = new Date(t0.getTime() + 29 * 24 * 60 * 60 * 1000);
        const resolved = sessions.resolve(token, day29);
        assert.ok(resolved);
        assert.equal(resolved.user.id, userId);
        assert.equal(resolved.user.username, 'sessuser');
    });

    it('resolve returns null once 30 days have passed since sign-in', () => {
        const { token } = sessions.create(userId, { now: t0 });
        const day30 = new Date(t0.getTime() + 30 * 24 * 60 * 60 * 1000 + 1);
        assert.equal(sessions.resolve(token, day30), null);
    });

    it('resolve returns null after revoke', () => {
        const { token } = sessions.create(userId, { now: t0 });
        assert.equal(sessions.revoke(token), true);
        assert.equal(sessions.resolve(token, t0), null);
    });

    it('revokeAllForUser clears sessions except optional hash', () => {
        const a = sessions.create(userId, { now: t0 });
        const b = sessions.create(userId, { now: t0 });
        sessions.revokeAllForUser(userId, a.tokenHash);
        assert.ok(sessions.resolve(a.token, t0));
        assert.equal(sessions.resolve(b.token, t0), null);
    });

    it('hash of token matches crypto SHA-256 hex', () => {
        const token = crypto.randomBytes(32).toString('base64url');
        const expected = crypto.createHash('sha256').update(token, 'utf8').digest('hex');
        assert.equal(sessions.hashToken(token), expected);
    });
});
