'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const SECRET = 'test-google-secret';
const GOOGLE_PORT = 38790;

function call(base, method, pathAndQuery, { headers = {}, body } = {}) {
    const url = new URL(pathAndQuery, base);
    const payload = body == null ? null : Buffer.from(body);
    return new Promise((resolve, reject) => {
        const req = http.request({
            host: '127.0.0.1',
            port: url.port,
            path: url.pathname + url.search,
            method,
            headers: Object.assign({}, headers, payload ? { 'Content-Length': payload.length } : {})
        }, (res) => {
            let data = '';
            res.on('data', (c) => { data += c; });
            res.on('end', () => resolve({
                status: res.statusCode,
                headers: res.headers,
                data
            }));
        });
        req.on('error', reject);
        req.end(payload);
    });
}

describe('Google app sign-in', () => {
    let tmpData;
    let tmpLessons;
    let server;
    let google;
    let base;
    let accounts;
    let db;
    let lastRedirect = '';

    before(async () => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-google-signin-'));
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-google-signin-lessons-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmpData;
        process.env.LESSONS_DIR = tmpLessons;
        process.env.HOST = '127.0.0.1';
        process.env.PORT = '0';
        process.env.MCP_AUTH_TOKEN = 'google-signin-test-token-' + 'q'.repeat(24);
        process.env.GOOGLE_OAUTH_CLIENT_ID = 'test-google-client';
        process.env.GOOGLE_OAUTH_CLIENT_SECRET = SECRET;
        process.env.GOOGLE_OAUTH_AUTH_URL = 'http://127.0.0.1:' + GOOGLE_PORT + '/auth';
        process.env.GOOGLE_OAUTH_TOKEN_URL = 'http://127.0.0.1:' + GOOGLE_PORT + '/token';
        process.env.GOOGLE_OAUTH_USERINFO_URL = 'http://127.0.0.1:' + GOOGLE_PORT + '/userinfo';
        delete process.env.COOKIE_SECURE;
        delete process.env.PUBLIC_ORIGIN;

        google = http.createServer((req, res) => {
            const chunks = [];
            req.on('data', (c) => chunks.push(c));
            req.on('end', () => {
                const raw = Buffer.concat(chunks).toString('utf8');
                if (req.url.startsWith('/token')) {
                    const params = new URLSearchParams(raw);
                    lastRedirect = params.get('redirect_uri') || '';
                    if (params.get('client_secret') !== SECRET || !params.get('code_verifier')) {
                        res.writeHead(400, { 'Content-Type': 'application/json' });
                        res.end('{}');
                        return;
                    }
                    const code = params.get('code');
                    if (code === 'boom') {
                        res.writeHead(400, { 'Content-Type': 'application/json' });
                        res.end('{"error":"invalid_grant"}');
                        return;
                    }
                    res.writeHead(200, { 'Content-Type': 'application/json' });
                    res.end(JSON.stringify({ access_token: 'gat-' + code }));
                    return;
                }
                if (req.url.startsWith('/userinfo')) {
                    const token = String(req.headers.authorization || '').replace(/^Bearer /, '');
                    const map = {
                        'gat-good': { email: 'Owner@Example.com', email_verified: true, sub: 'sub-owner', name: 'Owner Person', given_name: 'Owner' },
                        'gat-stranger': { email: 'stranger@example.com', email_verified: true, sub: 'sub-stranger', name: 'Sam Stranger' },
                        'gat-alice': { email: 'alice@example.com', email_verified: true, sub: 'sub-alice-google', name: 'Alice Google' },
                        'gat-unverified': { email: 'owner@example.com', email_verified: false, sub: 'sub-owner' }
                    };
                    if (!map[token]) {
                        res.writeHead(401, { 'Content-Type': 'application/json' });
                        res.end('{}');
                        return;
                    }
                    res.writeHead(200, { 'Content-Type': 'application/json' });
                    res.end(JSON.stringify(map[token]));
                    return;
                }
                res.writeHead(404);
                res.end();
            });
        });
        await new Promise((resolve) => google.listen(GOOGLE_PORT, '127.0.0.1', resolve));

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        accounts = require('../server/accounts');
        accounts.createUser({
            username: 'alice',
            password: 'a-strong-pass1',
            role: 'user',
            displayName: 'Alice',
            actor: 'test'
        });
        const web = require('../server/web');
        server = web.listen();
        await new Promise((resolve) => server.once('listening', resolve));
        base = 'http://127.0.0.1:' + server.address().port;
        process.env.PUBLIC_ORIGIN = base;
    });

    after(async () => {
        if (server) {
            if (typeof server.closeAllConnections === 'function') server.closeAllConnections();
            await new Promise((r) => server.close(() => r()));
        }
        if (google) {
            if (typeof google.closeAllConnections === 'function') google.closeAllConnections();
            await new Promise((r) => google.close(() => r()));
        }
        if (db) try { db.close(); } catch (_) { /* ignore */ }
        try { fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 }); } catch (_) { /* ignore */ }
        try { fs.rmSync(tmpLessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 }); } catch (_) { /* ignore */ }
        delete process.env.PUBLIC_ORIGIN;
        delete process.env.GOOGLE_OAUTH_CLIENT_ID;
        delete process.env.GOOGLE_OAUTH_CLIENT_SECRET;
        delete process.env.GOOGLE_OAUTH_AUTH_URL;
        delete process.env.GOOGLE_OAUTH_TOKEN_URL;
        delete process.env.GOOGLE_OAUTH_USERINFO_URL;
    });

    async function begin(next) {
        const start = await call(base, 'GET', '/api/auth/google/start?next=' + encodeURIComponent(next));
        assert.equal(start.status, 302);
        assert.doesNotMatch(start.headers.location, /client_secret|test-google-secret/);
        const loc = new URL(start.headers.location);
        assert.equal(loc.origin + loc.pathname, 'http://127.0.0.1:' + GOOGLE_PORT + '/auth');
        assert.equal(loc.searchParams.get('redirect_uri'), base + '/api/auth/google/callback');
        assert.equal(loc.searchParams.get('client_id'), 'test-google-client');
        return loc.searchParams.get('state');
    }

    it('signs in a linked Google account and refuses everyone else', async () => {
        const status = await call(base, 'GET', '/api/auth/google');
        assert.equal(status.status, 200);
        assert.deepEqual(JSON.parse(status.data), { enabled: true });

        const createdState = await begin('/today');
        const created = await call(base, 'GET', '/api/auth/google/callback?code=good&state=' + encodeURIComponent(createdState));
        assert.equal(created.status, 302, created.data);
        assert.equal(created.headers.location, '/today');
        const createdCookie = [].concat(created.headers['set-cookie'] || []).join('\n');
        assert.match(createdCookie, /hebrew_session=/);
        const createdMe = await call(base, 'GET', '/api/me', { headers: { Cookie: createdCookie.split(';')[0] } });
        assert.equal(createdMe.status, 200);
        const createdBody = JSON.parse(createdMe.data);
        assert.equal(createdBody.username, 'owner');
        assert.equal(createdBody.display_name, 'Owner Person');
        assert.equal(createdBody.role, 'user');
        assert.equal(createdBody.google_email, undefined);
        assert.equal(accounts.listUsers().length, 2);
        const owner = accounts.listUsers().find((u) => u.username === 'owner');
        assert.equal(owner.google_email, 'owner@example.com');
        assert.equal(owner.auth_provider, 'google');
        assert.equal(owner.role, 'user');

        const againState = await begin('/today');
        const again = await call(base, 'GET', '/api/auth/google/callback?code=good&state=' + encodeURIComponent(againState));
        assert.equal(again.headers.location, '/today');
        assert.equal(accounts.listUsers().length, 2);

        const state = await begin('https://evil.example/phish');
        const ok = await call(base, 'GET', '/api/auth/google/callback?code=good&state=' + encodeURIComponent(state));
        assert.equal(ok.status, 302);
        assert.equal(ok.headers.location, '/');
        const setCookie = [].concat(ok.headers['set-cookie'] || []).join('\n');
        assert.match(setCookie, /hebrew_session=/);
        assert.equal(lastRedirect, base + '/api/auth/google/callback');
        const cookie = setCookie.split(';')[0];
        const me = await call(base, 'GET', '/api/me', { headers: { Cookie: cookie } });
        assert.equal(me.status, 200);
        const body = JSON.parse(me.data);
        assert.equal(body.username, 'owner');
        assert.equal(body.google_email, undefined);

        const strangerState = await begin('/today');
        const stranger = await call(base, 'GET', '/api/auth/google/callback?code=stranger&state=' + encodeURIComponent(strangerState));
        assert.equal(stranger.headers.location, '/today');
        assert.doesNotMatch(stranger.data, /stranger@example.com/);
        const strangerMe = await call(base, 'GET', '/api/me', {
            headers: { Cookie: [].concat(stranger.headers['set-cookie']).join('\n').split(';')[0] }
        });
        assert.equal(JSON.parse(strangerMe.data).username, 'stranger');
        assert.equal(JSON.parse(strangerMe.data).display_name, 'Sam Stranger');

        const beforeAlice = accounts.listUsers().length;
        const aliceState = await begin('/today');
        const aliceGoogle = await call(base, 'GET', '/api/auth/google/callback?code=alice&state=' + encodeURIComponent(aliceState));
        assert.equal(aliceGoogle.headers.location, '/today');
        const aliceMe = await call(base, 'GET', '/api/me', {
            headers: { Cookie: [].concat(aliceGoogle.headers['set-cookie']).join('\n').split(';')[0] }
        });
        assert.equal(JSON.parse(aliceMe.data).username, 'alice2');
        assert.equal(JSON.parse(aliceMe.data).display_name, 'Alice Google');
        assert.equal(accounts.listUsers().length, beforeAlice + 1);

        const unverifiedState = await begin('/today');
        const unverified = await call(base, 'GET', '/api/auth/google/callback?code=unverified&state=' + encodeURIComponent(unverifiedState));
        assert.match(unverified.headers.location, /error=google_unverified/);

        const brokenState = await begin('/today');
        const broken = await call(base, 'GET', '/api/auth/google/callback?code=boom&state=' + encodeURIComponent(brokenState));
        assert.match(broken.headers.location, /error=google_failed/);
        assert.doesNotMatch(broken.data, /invalid_grant|test-google-secret/);

        const countBeforeDisable = accounts.listUsers().length;
        accounts.setStatus(owner.id, 'disabled', 'test');
        const disabledState = await begin('/today');
        const disabled = await call(base, 'GET', '/api/auth/google/callback?code=good&state=' + encodeURIComponent(disabledState));
        assert.match(disabled.headers.location, /error=google_disabled/);
        assert.equal(accounts.listUsers().length, countBeforeDisable);
        const alice = accounts.listUsers().find((u) => u.username === 'alice');
        accounts.setStatus(alice.id, 'disabled', 'test');

        const password = await call(base, 'POST', '/api/session', {
            headers: { 'Content-Type': 'application/json', Origin: base },
            body: JSON.stringify({ username: 'alice', password: 'a-strong-pass1' })
        });
        assert.equal(password.status, 401);
        accounts.createUser({
            username: 'bob',
            password: 'a-strong-pass1',
            role: 'user',
            actor: 'test'
        });
        const bob = await call(base, 'POST', '/api/session', {
            headers: { 'Content-Type': 'application/json', Origin: base },
            body: JSON.stringify({ username: 'bob', password: 'a-strong-pass1', next: '/today' })
        });
        assert.equal(bob.status, 200);
        assert.equal(JSON.parse(bob.data).username, 'bob');
        assert.equal(JSON.parse(bob.data).redirect, '/today');
    });
});
