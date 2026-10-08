/**
 * Accounts service — sign-in, guards, audit, policy.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

describe('accounts service (ac-3, ac-4, ac-5, ac-7)', () => {
    let tmp;
    let db;
    let accounts;
    let passwords;
    let sessions;
    let throttle;
    let policy;
    let audit;
    let sessionsRepo;
    const t0 = new Date('2026-05-01T08:00:00.000Z');
    const GOOD = 'a-strong-pass1';

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-accounts-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        for (const mod of [
            '../server/paths',
            '../server/db',
            '../server/db/repos/users',
            '../server/db/repos/sessions',
            '../server/db/repos/audit',
            '../server/auth/passwords',
            '../server/auth/sessions',
            '../server/auth/throttle',
            '../server/auth/policy',
            '../server/accounts'
        ]) {
            delete require.cache[require.resolve(mod)];
        }
        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        accounts = require('../server/accounts');
        passwords = require('../server/auth/passwords');
        sessions = require('../server/auth/sessions');
        throttle = require('../server/auth/throttle');
        policy = require('../server/auth/policy');
        audit = require('../server/db/repos/audit');
        sessionsRepo = require('../server/db/repos/sessions');
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
    });

    beforeEach(() => {
        throttle.reset();
        db.run('DELETE FROM sessions');
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM users');
    });

    it('signIn succeeds for active user and returns token without hash', () => {
        accounts.createUser({
            username: 'alice',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        const result = accounts.signIn('alice', GOOD, { ip: '127.0.0.1', now: t0 });
        assert.equal(result.user.username, 'alice');
        assert.equal(result.user.password_hash, undefined);
        assert.ok(result.token);
        assert.ok(sessions.resolve(result.token, t0));
    });

    it('unknown user uses dummy verify (ac-4)', () => {
        assert.throws(
            () => accounts.signIn('nobody', GOOD, { ip: '127.0.0.1', now: t0 }),
            (err) => err.code === 'auth_failed' && err.message === accounts.GENERIC_AUTH_ERROR
        );
        assert.equal(accounts.didLastSignInUseDummy(), true);
    });

    it('wrong password uses real verify path, not dummy', () => {
        accounts.createUser({
            username: 'bob',
            password: GOOD,
            role: 'user',
            actor: 'test'
        });
        assert.throws(
            () => accounts.signIn('bob', 'wrong-password', { ip: '127.0.0.1', now: t0 }),
            (err) => err.code === 'auth_failed'
        );
        assert.equal(accounts.didLastSignInUseDummy(), false);
    });

    it('disabled account fails with generic error', () => {
        const admin = accounts.createUser({
            username: 'admin1',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        accounts.createUser({
            username: 'disabled1',
            password: GOOD,
            role: 'user',
            actor: 'test'
        });
        const target = accounts.listUsers().find((u) => u.username === 'disabled1');
        accounts.setStatus(target.id, 'disabled', admin, t0);
        assert.throws(
            () => accounts.signIn('disabled1', GOOD, { ip: '127.0.0.1', now: t0 }),
            (err) => err.code === 'auth_failed'
        );
    });

    it('lockout refuses further attempts even with right password (ac-3)', () => {
        accounts.createUser({
            username: 'locked',
            password: GOOD,
            role: 'user',
            actor: 'test'
        });
        for (let i = 0; i < throttle.MAX_USERNAME_FAILURES; i++) {
            assert.throws(
                () => accounts.signIn('locked', 'bad-pass-xx', {
                    ip: '10.1.1.' + i,
                    now: new Date(t0.getTime() + i)
                }),
                (err) => err.code === 'auth_failed'
            );
        }
        assert.throws(
            () => accounts.signIn('locked', GOOD, {
                ip: '10.9.9.9',
                now: new Date(t0.getTime() + 100)
            }),
            (err) => err.code === 'throttled' && /try again later/i.test(err.message)
        );
    });

    it('demoting last active admin throws (ac-5)', () => {
        const only = accounts.createUser({
            username: 'soleadmin',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        assert.throws(
            () => accounts.setRole(only.id, 'user', 'cli', t0),
            (err) => err.code === 'last_admin'
        );
    });

    it('disabling last active admin throws (ac-5)', () => {
        const only = accounts.createUser({
            username: 'soleadmin2',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        assert.throws(
            () => accounts.setStatus(only.id, 'disabled', 'cli', t0),
            (err) => err.code === 'last_admin'
        );
    });

    it('admin cannot disable themselves (ac-5)', () => {
        const a = accounts.createUser({
            username: 'adminA',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        accounts.createUser({
            username: 'adminB',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        assert.throws(
            () => accounts.setStatus(a.id, 'disabled', a, t0),
            (err) => err.code === 'self_disable'
        );
    });

    it('setRole revokes sessions and writes audit (ac-5)', () => {
        const admin = accounts.createUser({
            username: 'adminR',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        const user = accounts.createUser({
            username: 'memberR',
            password: GOOD,
            role: 'user',
            actor: 'test'
        });
        const signed = accounts.signIn('memberR', GOOD, { ip: '127.0.0.1', now: t0 });
        assert.ok(sessions.resolve(signed.token, t0));
        accounts.setRole(user.id, 'admin', admin, t0);
        assert.equal(sessions.resolve(signed.token, t0), null);
        const logs = audit.listRecent(10);
        assert.ok(logs.some((row) => row.action === 'user.setRole' && row.target === 'memberr'));
    });

    it('setStatus disable revokes sessions and writes audit (ac-5)', () => {
        const admin = accounts.createUser({
            username: 'adminS',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        const user = accounts.createUser({
            username: 'memberS',
            password: GOOD,
            role: 'user',
            actor: 'test'
        });
        const signed = accounts.signIn('memberS', GOOD, { ip: '127.0.0.1', now: t0 });
        accounts.setStatus(user.id, 'disabled', admin, t0);
        assert.equal(sessions.resolve(signed.token, t0), null);
        const logs = audit.listRecent(10);
        assert.ok(logs.some((row) => row.action === 'user.setStatus' && row.target === 'members'));
    });

    it('resetPassword sets must_change_password, revokes sessions, audits (ac-5)', () => {
        const admin = accounts.createUser({
            username: 'adminP',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        const user = accounts.createUser({
            username: 'memberP',
            password: GOOD,
            role: 'user',
            actor: 'test'
        });
        const signed = accounts.signIn('memberP', GOOD, { ip: '127.0.0.1', now: t0 });
        const updated = accounts.resetPassword(user.id, 'brand-new-pass1', admin, t0);
        assert.equal(updated.must_change_password, true);
        assert.equal(sessions.resolve(signed.token, t0), null);
        const logs = audit.listRecent(10);
        assert.ok(logs.some((row) => row.action === 'user.resetPassword' && row.target === 'memberp'));
        // New password works
        const again = accounts.signIn('memberP', 'brand-new-pass1', { ip: '127.0.0.1', now: t0 });
        assert.equal(again.user.must_change_password, true);
    });

    it('changePassword revokes other sessions and clears must_change_password', () => {
        const user = accounts.createUser({
            username: 'changer',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        const s1 = accounts.signIn('changer', GOOD, { ip: '127.0.0.1', now: t0 });
        const s2 = accounts.signIn('changer', GOOD, { ip: '127.0.0.1', now: t0 });
        accounts.changePassword(user.id, GOOD, 'even-stronger1', {
            now: t0,
            keepToken: s1.token
        });
        assert.ok(sessions.resolve(s1.token, t0));
        assert.equal(sessions.resolve(s2.token, t0), null);
        const row = accounts.listUsers().find((u) => u.username === 'changer');
        assert.equal(row.must_change_password, false);
    });

    it('listUsers never returns password_hash', () => {
        accounts.createUser({
            username: 'listed',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        for (const u of accounts.listUsers()) {
            assert.equal(Object.prototype.hasOwnProperty.call(u, 'password_hash'), false);
        }
    });

    it('can() refuses every admin action for user-role (ac-7)', () => {
        const learner = { role: 'user', status: 'active' };
        for (const action of policy.ADMIN_ACTIONS) {
            assert.equal(
                policy.can(learner, action),
                false,
                'user must be refused ' + action
            );
        }
        const admin = { role: 'admin', status: 'active' };
        for (const action of policy.ADMIN_ACTIONS) {
            assert.equal(policy.can(admin, action), true, 'admin must be allowed ' + action);
        }
        assert.equal(policy.requireAdmin(learner), false);
        assert.equal(policy.requireAdmin(admin), true);
    });

    it('password policy rejects common passwords on create', () => {
        assert.throws(
            () => accounts.createUser({
                username: 'weak',
                password: 'password123',
                role: 'user',
                actor: 'test'
            }),
            (err) => err.code === 'password_policy'
        );
    });

    it('sessions repo never stores raw token from signIn', () => {
        accounts.createUser({
            username: 'tokencheck',
            password: GOOD,
            role: 'admin',
            actor: 'test'
        });
        const signed = accounts.signIn('tokencheck', GOOD, { ip: '127.0.0.1', now: t0 });
        const rows = sessionsRepo.listByUser(
            accounts.listUsers().find((u) => u.username === 'tokencheck').id
        );
        assert.equal(rows.length, 1);
        assert.notEqual(rows[0].token_hash, signed.token);
        assert.equal(rows[0].token_hash, sessions.hashToken(signed.token));
    });
});
