/**
 * Auth API: session cookie, rotation, revoke, me, password, next (SPEC-008 ac-3, ac-5).
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

describe('api auth (ac-3, ac-5)', () => {
    let tmpData;
    let tmpLessons;
    let server;
    let baseUrl;
    let accounts;
    let cookies;
    let sessions;
    let db;

    const PASS = 'a-strong-pass1';

    function clearMods() {
        const names = [
            '../server/paths',
            '../server/db',
            '../server/accounts',
            '../server/auth/sessions',
            '../server/auth/throttle',
            '../server/http/cookies',
            '../server/http/security',
            '../server/api/router',
            '../server/api/auth-routes',
            '../server/visibility',
            '../server/content-filter',
            '../server/web'
        ];
        for (const name of names) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* */ }
        }
    }

    before(async () => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-api-auth-'));
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-api-auth-lessons-'));
        fs.mkdirSync(path.join(tmpLessons, 'public-lesson'), { recursive: true });
        fs.writeFileSync(
            path.join(tmpLessons, 'public-lesson', 'worksheet.js'),
            'window.WORKSHEET = { id: "public-lesson", title: "P", vocab: { items: [] }, activities: [] };\n'
        );
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmpData;
        process.env.LESSONS_DIR = tmpLessons;
        process.env.HOST = '127.0.0.1';
        process.env.PORT = '0';
        delete process.env.COOKIE_SECURE;
        delete process.env.PUBLIC_ORIGIN;
        clearMods();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        require('../server/db/repos/visibility').setVisibility('public-lesson', 'everyone');

        accounts = require('../server/accounts');
        cookies = require('../server/http/cookies');
        sessions = require('../server/auth/sessions');

        accounts.createUser({
            username: 'alice',
            password: PASS,
            role: 'user',
            displayName: 'Alice',
            actor: 'test'
        });

        const web = require('../server/web');
        server = web.listen();
        await new Promise((resolve) => server.once('listening', resolve));
        baseUrl = 'http://127.0.0.1:' + server.address().port;
    });

    after(async () => {
        if (server) await new Promise((r) => server.close(() => r()));
        if (db) try { db.close(); } catch (_) { /* */ }
        try { fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 }); } catch (_) { /* */ }
        try { fs.rmSync(tmpLessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 }); } catch (_) { /* */ }
        delete process.env.DATA_DIR;
        delete process.env.LESSONS_DIR;
        delete process.env.COOKIE_SECURE;
        clearMods();
    });

    beforeEach(() => {
        require('../server/auth/throttle').reset();
    });

    async function postSession(body, headerExtra) {
        return fetch(baseUrl + '/api/session', {
            method: 'POST',
            headers: Object.assign({
                'Content-Type': 'application/json',
                Origin: baseUrl
            }, headerExtra || {}),
            body: JSON.stringify(body)
        });
    }

    it('ac-3: sign-in sets HttpOnly SameSite=Lax cookie without Secure by default', async () => {
        const res = await postSession({ username: 'alice', password: PASS });
        assert.equal(res.status, 200);
        const setCookie = res.headers.getSetCookie ? res.headers.getSetCookie() : [res.headers.get('set-cookie')];
        const line = setCookie.find((c) => c && c.includes('hebrew_session='));
        assert.ok(line, 'Set-Cookie present');
        assert.match(line, /HttpOnly/i);
        assert.match(line, /SameSite=Lax/i);
        assert.match(line, /Path=\//i);
        assert.doesNotMatch(line, /;\s*Secure/i);
        assert.doesNotMatch(line, /__Host-hebrew_session/);
        const json = await res.json();
        assert.equal(json.username, 'alice');
        assert.equal(json.must_change_password, false);
        assert.equal(json.redirect, '/');
        assert.equal(res.headers.get('cache-control'), 'no-store');
    });

    it('ac-3: COOKIE_SECURE=1 uses __Host- prefix and Secure', async () => {
        process.env.COOKIE_SECURE = '1';
        delete require.cache[require.resolve('../server/http/cookies')];
        cookies = require('../server/http/cookies');
        assert.equal(cookies.cookieName(), '__Host-hebrew_session');
        const line = cookies.serializeSession('tok');
        assert.match(line, /^__Host-hebrew_session=/);
        assert.match(line, /Secure/);
        assert.match(line, /HttpOnly/);
        assert.match(line, /SameSite=Lax/);
        delete process.env.COOKIE_SECURE;
        delete require.cache[require.resolve('../server/http/cookies')];
        cookies = require('../server/http/cookies');
    });

    it('ac-3: pre-existing session cookie is replaced, not reused', async () => {
        const first = await postSession({ username: 'alice', password: PASS });
        const firstCookies = first.headers.getSetCookie
            ? first.headers.getSetCookie()
            : [first.headers.get('set-cookie')];
        const firstLine = firstCookies.find((c) => c.includes('hebrew_session='));
        const firstToken = decodeURIComponent(firstLine.split(';')[0].split('=')[1]);

        const second = await postSession(
            { username: 'alice', password: PASS },
            { Cookie: 'hebrew_session=' + encodeURIComponent(firstToken) }
        );
        assert.equal(second.status, 200);
        const secondCookies = second.headers.getSetCookie
            ? second.headers.getSetCookie()
            : [second.headers.get('set-cookie')];
        const secondLine = secondCookies.find((c) => c.includes('hebrew_session='));
        const secondToken = decodeURIComponent(secondLine.split(';')[0].split('=')[1]);
        assert.notEqual(secondToken, firstToken);
        // Old token may still resolve until revoked on password change; rotation means new cookie.
        assert.ok(sessions.resolve(secondToken));
    });

    it('ac-3: sign-out revokes session; old cookie then gets 401', async () => {
        const signIn = await postSession({ username: 'alice', password: PASS });
        const setCookies = signIn.headers.getSetCookie
            ? signIn.headers.getSetCookie()
            : [signIn.headers.get('set-cookie')];
        const line = setCookies.find((c) => c.includes('hebrew_session='));
        const token = decodeURIComponent(line.split(';')[0].split('=')[1]);
        const cookieHeader = 'hebrew_session=' + encodeURIComponent(token);

        const del = await fetch(baseUrl + '/api/session', {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
                Origin: baseUrl,
                Cookie: cookieHeader
            },
            body: '{}'
        });
        assert.equal(del.status, 200);
        assert.equal(sessions.resolve(token), null);

        const me = await fetch(baseUrl + '/api/me', {
            headers: { Cookie: cookieHeader }
        });
        assert.equal(me.status, 401);
    });

    it('ac-3: GET /api/me returns id, username, display_name, role, must_change_password only', async () => {
        const signIn = await postSession({ username: 'alice', password: PASS });
        const setCookies = signIn.headers.getSetCookie
            ? signIn.headers.getSetCookie()
            : [signIn.headers.get('set-cookie')];
        const line = setCookies.find((c) => c.includes('hebrew_session='));
        const token = decodeURIComponent(line.split(';')[0].split('=')[1]);

        const me = await fetch(baseUrl + '/api/me', {
            headers: { Cookie: 'hebrew_session=' + encodeURIComponent(token) }
        });
        assert.equal(me.status, 200);
        const json = await me.json();
        assert.deepEqual(Object.keys(json).sort(), [
            'display_name',
            'id',
            'must_change_password',
            'role',
            'username'
        ]);
        assert.equal(json.username, 'alice');
        assert.equal(json.display_name, 'Alice');
        assert.equal(json.role, 'user');
    });

    it('ac-3: POST /api/me/password changes password and revokes other sessions', async () => {
        const a = await postSession({ username: 'alice', password: PASS });
        const b = await postSession({ username: 'alice', password: PASS });
        const cookieA = (() => {
            const lines = a.headers.getSetCookie ? a.headers.getSetCookie() : [a.headers.get('set-cookie')];
            const line = lines.find((c) => c.includes('hebrew_session='));
            return 'hebrew_session=' + line.split(';')[0].split('=').slice(1).join('=');
        })();
        const tokenB = (() => {
            const lines = b.headers.getSetCookie ? b.headers.getSetCookie() : [b.headers.get('set-cookie')];
            const line = lines.find((c) => c.includes('hebrew_session='));
            return decodeURIComponent(line.split(';')[0].split('=')[1]);
        })();

        const NEW = 'another-strong-9';
        const change = await fetch(baseUrl + '/api/me/password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Origin: baseUrl,
                Cookie: cookieA
            },
            body: JSON.stringify({
                current_password: PASS,
                new_password: NEW
            })
        });
        assert.equal(change.status, 200);
        assert.equal(sessions.resolve(tokenB), null);

        // Restore password for later tests.
        const again = await postSession({ username: 'alice', password: NEW });
        assert.equal(again.status, 200);
        const lines = again.headers.getSetCookie ? again.headers.getSetCookie() : [again.headers.get('set-cookie')];
        const line = lines.find((c) => c.includes('hebrew_session='));
        const cookie = 'hebrew_session=' + line.split(';')[0].split('=').slice(1).join('=');
        const restore = await fetch(baseUrl + '/api/me/password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Origin: baseUrl,
                Cookie: cookie
            },
            body: JSON.stringify({
                current_password: NEW,
                new_password: PASS
            })
        });
        assert.equal(restore.status, 200);
    });

    it('ac-5: hostile next values redirect to / after sign-in', async () => {
        const hostile = [
            'https://evil.example',
            '//evil.example',
            '/\\evil.example',
            'javascript:alert(1)'
        ];
        for (const next of hostile) {
            const res = await postSession({ username: 'alice', password: PASS, next });
            assert.equal(res.status, 200, next);
            const json = await res.json();
            assert.equal(json.redirect, '/', next);
        }
        // Direct sanitizer coverage (server must not emit hostile next).
        const { safeNextPath } = require('../server/http/security');
        for (const next of hostile) {
            assert.equal(safeNextPath(next), '/', next);
        }
    });

    it('failed sign-in stays generic (no user enumeration)', async () => {
        const res = await postSession({ username: 'nobody', password: PASS });
        assert.equal(res.status, 401);
        const json = await res.json();
        assert.equal(json.error, accounts.GENERIC_AUTH_ERROR);
        assert.ok(!JSON.stringify(json).toLowerCase().includes('nobody'));
        assert.ok(!JSON.stringify(json).includes('stack'));
    });
});
