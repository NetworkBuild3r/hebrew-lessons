/**
 * PATCH /api/admin/users/:id/ai-planning.
 * Temporary database, loopback only. No network beyond 127.0.0.1.
 * Provenance: INIT-006/SPEC-010
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');

const harness = require('./helpers/signed-in-server');

describe('AI-planning switch route (INIT-006/SPEC-010)', { concurrency: false }, () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(u: string) => string} */
    let cookieFor;
    /** @type {Record<string, { id: number, username: string }>} */
    let users;
    /** @type {typeof import('../server/db')} */
    let db;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        users = started.users;
        db = require('../server/db');
    });

    after(async () => {
        await harness.stop();
    });

    async function api(method, path, username, body, opts) {
        const options = opts || {};
        const headers = {
            'Content-Type': 'application/json',
            Origin: baseUrl
        };
        if (username) headers.Cookie = cookieFor(username);
        if (options.origin === false) delete headers.Origin;
        const init = { method, headers };
        if (body !== undefined) init.body = JSON.stringify(body);
        const res = await fetch(baseUrl + path, init);
        const text = await res.text();
        let json = null;
        if (text) {
            try { json = JSON.parse(text); } catch (_) { json = null; }
        }
        return { status: res.status, json };
    }

    it('ac-3: 401 signed out, 403 for a user, 200 for an admin, and an audit row', async () => {
        const path = '/api/admin/users/' + users.learner.id + '/ai-planning';
        const signedOut = await api('PATCH', path, null, { enabled: true });
        assert.equal(signedOut.status, 401);
        assert.equal(signedOut.json.code, 'no_session');

        const asUser = await api('PATCH', path, 'learner', { enabled: true });
        assert.equal(asUser.status, 403);
        assert.equal(asUser.json.code, 'forbidden');

        const before = db.get(
            "SELECT COUNT(*) AS n FROM audit_log WHERE action = 'ai_planning_switch'"
        );
        const turnedOn = await api('PATCH', path, 'admin', { enabled: true });
        assert.equal(turnedOn.status, 200);
        assert.deepEqual(turnedOn.json, { ai_planning: true, previous: 0, next: 1 });

        const row = db.get(
            "SELECT actor, action, target, detail FROM audit_log WHERE action = 'ai_planning_switch' ORDER BY id DESC LIMIT 1"
        );
        assert.equal(row.actor, 'admin:admin');
        assert.equal(row.action, 'ai_planning_switch');
        assert.equal(row.target, 'learner');
        assert.deepEqual(JSON.parse(row.detail), { previous: 0, next: 1 });
        const after = db.get(
            "SELECT COUNT(*) AS n FROM audit_log WHERE action = 'ai_planning_switch'"
        );
        assert.equal(Number(after.n), Number(before.n) + 1);

        const flag = db.get('SELECT ai_planning FROM users WHERE id = ?', users.learner.id);
        assert.equal(Number(flag.ai_planning), 1);

        const turnedOff = await api('PATCH', path, 'admin', { enabled: false });
        assert.equal(turnedOff.status, 200);
        assert.deepEqual(turnedOff.json, { ai_planning: false, previous: 1, next: 0 });
    });

    it('a missing user is 404 and a non-boolean body is 400', async () => {
        const missing = await api('PATCH', '/api/admin/users/999999/ai-planning', 'admin', {
            enabled: true
        });
        assert.equal(missing.status, 404);
        assert.equal(missing.json.code, 'not_found');

        const bad = await api('PATCH', '/api/admin/users/' + users.learner.id + '/ai-planning', 'admin', {
            enabled: 'yes'
        });
        assert.equal(bad.status, 400);
        const flag = db.get('SELECT ai_planning FROM users WHERE id = ?', users.learner.id);
        assert.equal(Number(flag.ai_planning), 0);
    });
});
