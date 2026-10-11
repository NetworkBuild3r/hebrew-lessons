/**
 * CSRF / Content-Type / method / security-header checks (SPEC-008 ac-4).
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const harness = require('./helpers/signed-in-server');

describe('http security (ac-4)', () => {
    before(async () => {
        await harness.start();
    });

    after(async () => {
        await harness.stop();
    });

    async function api(method, path, opts) {
        const options = opts || {};
        const headers = Object.assign({}, options.headers || {});
        return fetch(harness.baseUrl + path, {
            method,
            headers,
            body: options.body,
            redirect: 'manual'
        });
    }

    it('ac-4: POST without application/json Content-Type gets 403 before handler', async () => {
        const res = await api('POST', '/api/session', {
            headers: {
                'Content-Type': 'text/plain',
                Origin: harness.baseUrl
            },
            body: '{"username":"admin","password":"x"}'
        });
        assert.equal(res.status, 403);
        const json = await res.json();
        assert.equal(json.error, 'Forbidden');
        assert.ok(!String(JSON.stringify(json)).includes('stack'));
    });

    it('ac-4: POST with foreign Origin gets 403 before handler', async () => {
        const res = await api('POST', '/api/session', {
            headers: {
                'Content-Type': 'application/json',
                Origin: 'https://evil.example'
            },
            body: JSON.stringify({
                username: 'admin',
                password: harness.users.admin.password
            })
        });
        assert.equal(res.status, 403);
    });

    it('ac-4: request host is allowed when PUBLIC_ORIGIN names a different host', async () => {
        process.env.PUBLIC_ORIGIN = 'http://localhost:8080';
        try {
            const res = await api('POST', '/api/session', {
                headers: {
                    'Content-Type': 'application/json',
                    Origin: harness.baseUrl
                },
                body: JSON.stringify({
                    username: 'admin',
                    password: harness.users.admin.password
                })
            });
            assert.equal(res.status, 200);
            const foreign = await api('POST', '/api/session', {
                headers: {
                    'Content-Type': 'application/json',
                    Origin: 'https://evil.example'
                },
                body: JSON.stringify({
                    username: 'admin',
                    password: harness.users.admin.password
                })
            });
            assert.equal(foreign.status, 403);
        } finally {
            delete process.env.PUBLIC_ORIGIN;
        }
    });

    it('ac-4: POST with matching Origin and application/json; charset=utf-8 succeeds', async () => {
        const res = await api('POST', '/api/session', {
            headers: {
                'Content-Type': 'application/json; charset=utf-8',
                Origin: harness.baseUrl
            },
            body: JSON.stringify({
                username: 'admin',
                password: harness.users.admin.password
            })
        });
        assert.equal(res.status, 200);
        assert.equal(res.headers.get('cache-control'), 'no-store');
        assert.equal(res.headers.get('x-content-type-options'), 'nosniff');
        assert.equal(res.headers.get('referrer-policy'), 'same-origin');
        assert.equal(res.headers.get('x-frame-options'), 'DENY');
    });

    it('ac-4: missing Origin and Referer gets 403', async () => {
        const res = await api('POST', '/api/session', {
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                username: 'admin',
                password: harness.users.admin.password
            })
        });
        assert.equal(res.status, 403);
    });

    it('ac-4: non-GET methods outside /api/ get 405', async () => {
        for (const method of ['POST', 'PUT', 'PATCH', 'DELETE']) {
            const res = await api(method, '/index.html', {
                headers: {
                    'Content-Type': 'application/json',
                    Origin: harness.baseUrl
                },
                body: '{}'
            });
            assert.equal(res.status, 405, method + ' /index.html');
        }
    });

    it('ac-4: Referer matching Host is accepted when Origin absent', async () => {
        const res = await api('POST', '/api/session', {
            headers: {
                'Content-Type': 'application/json',
                Referer: harness.baseUrl + '/login/'
            },
            body: JSON.stringify({
                username: 'learner',
                password: harness.users.learner.password
            })
        });
        assert.equal(res.status, 200);
    });
});
