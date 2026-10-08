/**
 * Admin API routes (SPEC-010 ac-1 … ac-7).
 * Temp DATA_DIR + LESSONS_DIR, fake TTS, no real ElevenLabs calls.
 * Provenance: INIT-002/SPEC-010
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const https = require('https');
const path = require('path');

const harness = require('./helpers/signed-in-server');

/** Distinctive key set only in this test process — never from .env. */
const TEST_ELEVENLABS_KEY = 'test-elevenlabs-key-api-admin-spec010-only';

function sampleWorksheet(slug, overrides) {
    const base = {
        id: slug,
        title: 'בדיקה — ' + slug,
        subtitle: 'test',
        level: 'basic',
        vocab: {
            items: [{
                id: '1',
                word: 'שָׁלוֹם',
                meaning: 'hello',
                sound: 'sha-LOM',
                trap: 'Stress the last syllable.',
                sentence: 'שָׁלוֹם עוֹלָם.'
            }]
        },
        reading: {
            paragraphs: ['שָׁלוֹם עוֹלָם.']
        },
        activities: [{ type: 'words' }, { type: 'story' }]
    };
    return Object.assign(base, overrides || {});
}

function stubHttps(payloadFactory) {
    const orig = https.request;
    let calls = 0;
    let delayMs = 0;
    https.request = function (url, options, cb) {
        calls += 1;
        let body = '';
        const listeners = { data: [], end: [] };
        const res = {
            statusCode: 200,
            on(ev, fn) {
                if (ev === 'data') listeners.data.push(fn);
                if (ev === 'end') listeners.end.push(fn);
                return res;
            }
        };
        if (typeof options === 'function') cb = options;
        if (typeof cb === 'function') cb(res);
        return {
            on() { return this; },
            write(chunk) { body += chunk; },
            end() {
                let text = '';
                try { text = JSON.parse(body).text; } catch (_) { /* ignore */ }
                const buf = Buffer.from(
                    typeof payloadFactory === 'function' ? payloadFactory(text, calls) : payloadFactory
                );
                const emit = () => {
                    listeners.data.forEach((fn) => fn(buf));
                    listeners.end.forEach((fn) => fn());
                };
                if (delayMs > 0) setTimeout(emit, delayMs);
                else process.nextTick(emit);
            }
        };
    };
    return {
        restore() { https.request = orig; },
        get calls() { return calls; },
        setDelay(ms) { delayMs = ms; }
    };
}

describe('api admin (SPEC-010)', () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(u: string) => string} */
    let cookieFor;
    /** @type {Record<string, { id: number, username: string, role: string, password: string, token: string }>} */
    let users;
    /** @type {string} */
    let lessonsDir;
    /** @type {typeof import('../server/api/admin-routes')} */
    let adminRoutes;
    /** @type {typeof import('../server/db')} */
    let db;
    /** @type {typeof import('../server/accounts')} */
    let accounts;
    /** @type {typeof import('../server/progress')} */
    let progress;
    /** @type {string|undefined} */
    let prevElevenKey;
    /** @type {string|undefined} */
    let prevVoice;
    /** @type {string|undefined} */
    let prevModel;

    before(async () => {
        prevElevenKey = process.env.ELEVENLABS_API_KEY;
        prevVoice = process.env.ELEVENLABS_VOICE_ID;
        prevModel = process.env.ELEVENLABS_MODEL;
        process.env.ELEVENLABS_API_KEY = TEST_ELEVENLABS_KEY;
        process.env.ELEVENLABS_VOICE_ID = 'voice';
        process.env.ELEVENLABS_MODEL = 'eleven_v3';

        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        users = started.users;
        lessonsDir = started.lessonsDir;
        adminRoutes = require('../server/api/admin-routes');
        db = require('../server/db');
        accounts = require('../server/accounts');
        progress = require('../server/progress');
    });

    after(async () => {
        await harness.stop();
        if (prevElevenKey === undefined) delete process.env.ELEVENLABS_API_KEY;
        else process.env.ELEVENLABS_API_KEY = prevElevenKey;
        if (prevVoice === undefined) delete process.env.ELEVENLABS_VOICE_ID;
        else process.env.ELEVENLABS_VOICE_ID = prevVoice;
        if (prevModel === undefined) delete process.env.ELEVENLABS_MODEL;
        else process.env.ELEVENLABS_MODEL = prevModel;
    });

    function jsonHeaders(username, extra) {
        const headers = Object.assign({
            'Content-Type': 'application/json',
            Origin: baseUrl
        }, extra || {});
        if (username) headers.Cookie = cookieFor(username);
        return headers;
    }

    async function api(method, pathName, username, body) {
        const opts = {
            method,
            headers: jsonHeaders(username)
        };
        if (body !== undefined) {
            opts.body = typeof body === 'string' ? body : JSON.stringify(body);
        } else if (method !== 'GET' && method !== 'HEAD') {
            opts.body = '{}';
        }
        return fetch(baseUrl + pathName, opts);
    }

    function pathFor(route) {
        return route.path
            .replace(':id', String(users.learner.id))
            .replace(':slug', 'public-lesson');
    }

    function auditCount() {
        const row = db.get('SELECT COUNT(*) AS n FROM audit_log');
        return row ? Number(row.n) : 0;
    }

    function signal(partial) {
        return Object.assign({
            id: 'sig-' + Math.random().toString(36).slice(2),
            word: 'שָׁלוֹם',
            direction: 'he_en',
            result: 'pass',
            at: new Date().toISOString(),
            slug: 'public-lesson',
            activity: 'words'
        }, partial);
    }

    it('ac-1: every admin route returns 401 signed out and 403 for a user', async () => {
        assert.ok(adminRoutes.ROUTES.length >= 14, 'route table must list every admin route');
        for (const route of adminRoutes.ROUTES) {
            const p = pathFor(route);
            const signedOut = await api(route.method, p, null, route.method === 'GET' ? undefined : {});
            assert.equal(
                signedOut.status,
                401,
                route.method + ' ' + route.path + ' signed out → 401, got ' + signedOut.status
            );

            const asUser = await api(route.method, p, 'learner', route.method === 'GET' ? undefined : {});
            assert.equal(
                asUser.status,
                403,
                route.method + ' ' + route.path + ' user → 403, got ' + asUser.status
            );
        }
    });

    it('ac-2: last admin and self-disable return 409; disabled session then 401', async () => {
        // Last active admin cannot demote or disable themselves via guards.
        const demoteLast = await api('PATCH', '/api/admin/users/' + users.admin.id, 'admin', {
            role: 'user'
        });
        assert.equal(demoteLast.status, 409);
        assert.equal((await demoteLast.json()).code, 'last_admin');

        const disableSelf = await api('PATCH', '/api/admin/users/' + users.admin.id, 'admin', {
            status: 'disabled'
        });
        assert.equal(disableSelf.status, 409);
        assert.equal((await disableSelf.json()).code, 'self_disable');

        // Second admin so we can disable a non-last target with an existing session.
        const other = accounts.createUser({
            username: 'admin2',
            password: harness.GOOD_PASSWORD,
            role: 'admin',
            displayName: 'Admin Two',
            actor: 'test'
        });
        const otherSession = accounts.signIn('admin2', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        users.admin2 = {
            id: other.id,
            username: 'admin2',
            role: 'admin',
            password: harness.GOOD_PASSWORD,
            token: otherSession.token
        };

        const cookies = require('../server/http/cookies');
        const admin2Cookie = cookies.cookieName() + '=' + encodeURIComponent(otherSession.token);

        const disableOther = await api('PATCH', '/api/admin/users/' + other.id, 'admin', {
            status: 'disabled'
        });
        assert.equal(disableOther.status, 200);

        const afterDisable = await fetch(baseUrl + '/api/admin/users', {
            method: 'GET',
            headers: {
                Origin: baseUrl,
                Cookie: admin2Cookie
            }
        });
        assert.equal(afterDisable.status, 401);
    });

    it('ac-3: each mutating route writes exactly one audit row', async () => {
        const mutating = [
            {
                name: 'POST users',
                run: () => api('POST', '/api/admin/users', 'admin', {
                    username: 'audit-user-' + Date.now(),
                    password: harness.GOOD_PASSWORD,
                    role: 'user',
                    displayName: 'Audit User'
                }),
                expectStatus: 201
            },
            {
                name: 'PATCH users status',
                setup: () => accounts.createUser({
                    username: 'patch-target-' + Date.now(),
                    password: harness.GOOD_PASSWORD,
                    role: 'user',
                    actor: 'test'
                }),
                run: (created) => api('PATCH', '/api/admin/users/' + created.id, 'admin', {
                    status: 'disabled'
                }),
                expectStatus: 200
            },
            {
                name: 'POST password',
                run: async () => {
                    const newPass = harness.GOOD_PASSWORD + 'X';
                    const res = await api(
                        'POST',
                        '/api/admin/users/' + users.learner.id + '/password',
                        'admin',
                        { password: newPass }
                    );
                    // resetPassword revokes sessions — refresh harness cookie for later ACs.
                    if (res.status === 200) {
                        const session = accounts.signIn('learner', newPass, { ip: '127.0.0.1' });
                        users.learner.token = session.token;
                        users.learner.password = newPass;
                    }
                    return res;
                },
                expectStatus: 200
            },
            {
                name: 'PUT visibility',
                run: () => api('PUT', '/api/admin/lessons/public-lesson/visibility', 'admin', {
                    mode: 'everyone'
                }),
                expectStatus: 200
            },
            {
                name: 'POST lessons',
                run: () => {
                    const slug = 'audit-lesson-' + Date.now();
                    const worksheet = sampleWorksheet(slug);
                    return api('POST', '/api/admin/lessons', 'admin', {
                        slug,
                        worksheet,
                        catalogEntry: { slug, title: worksheet.title, level: 'basic' }
                    });
                },
                expectStatus: 201
            }
        ];

        // PUT lesson after creating one to update.
        const putSlug = 'audit-put-' + Date.now();
        const putWs = sampleWorksheet(putSlug);
        const createPut = await api('POST', '/api/admin/lessons', 'admin', {
            slug: putSlug,
            worksheet: putWs,
            catalogEntry: { slug: putSlug, title: putWs.title, level: 'basic' }
        });
        assert.equal(createPut.status, 201);

        mutating.push({
            name: 'PUT lessons',
            run: () => {
                const next = sampleWorksheet(putSlug, {
                    vocab: {
                        items: [{
                            id: '1',
                            word: 'שָׁלוֹם',
                            meaning: 'hello',
                            sound: 'sha-LOM',
                            trap: 'Stress the last syllable.',
                            sentence: 'שָׁלוֹם חָדָשׁ.'
                        }]
                    }
                });
                return api('PUT', '/api/admin/lessons/' + putSlug, 'admin', {
                    worksheet: next,
                    catalogEntry: { slug: putSlug, title: next.title, level: 'basic' }
                });
            },
            expectStatus: 200
        });

        const httpsStub = stubHttps('AUDIT-TTS');
        try {
            mutating.push({
                name: 'POST audio',
                run: () => api('POST', '/api/admin/lessons/' + putSlug + '/audio', 'admin', {}),
                expectStatus: 200
            });

            for (const step of mutating) {
                const setupResult = step.setup ? await step.setup() : undefined;
                const before = auditCount();
                const res = await step.run(setupResult);
                assert.equal(res.status, step.expectStatus, step.name + ' status');
                const after = auditCount();
                assert.equal(
                    after - before,
                    1,
                    step.name + ' must write exactly one audit row (before=' + before + ' after=' + after + ')'
                );
                const latest = db.get('SELECT * FROM audit_log ORDER BY id DESC LIMIT 1');
                assert.ok(latest.actor, step.name + ' audit actor');
                assert.ok(latest.action, step.name + ' audit action');
                assert.ok(latest.target != null && latest.target !== '', step.name + ' audit target');
            }
        } finally {
            httpsStub.restore();
        }
    });

    it('ac-4: progress and events for target user; user role gets 403', async () => {
        // Seed learner events newest-first paging.
        const t0 = Date.parse('2024-01-01T00:00:00.000Z');
        for (let i = 0; i < 5; i++) {
            progress.applySignals(users.learner.id, [
                signal({
                    id: 'hist-' + i,
                    word: 'שָׁלוֹם',
                    result: i % 2 === 0 ? 'pass' : 'miss',
                    at: new Date(t0 + i * 1000).toISOString()
                })
            ]);
        }

        const asUserProg = await api(
            'GET',
            '/api/admin/users/' + users.learner.id + '/progress',
            'learner'
        );
        assert.equal(asUserProg.status, 403);

        const asUserEvents = await api(
            'GET',
            '/api/admin/users/' + users.learner.id + '/events',
            'learner'
        );
        assert.equal(asUserEvents.status, 403);

        const prog = await api(
            'GET',
            '/api/admin/users/' + users.learner.id + '/progress',
            'admin'
        );
        assert.equal(prog.status, 200);
        assert.equal(prog.headers.get('cache-control'), 'no-store');
        const progJson = await prog.json();
        assert.ok(progJson.metrics);
        assert.ok(progJson.metrics.stages);
        assert.ok(Array.isArray(progJson.troubleWords));

        const page1 = await api(
            'GET',
            '/api/admin/users/' + users.learner.id + '/events?limit=2',
            'admin'
        );
        assert.equal(page1.status, 200);
        const page1Json = await page1.json();
        assert.equal(page1Json.events.length, 2);
        assert.ok(page1Json.nextBefore);
        assert.ok(
            page1Json.events[0].at >= page1Json.events[1].at,
            'events newest first'
        );

        const page2 = await api(
            'GET',
            '/api/admin/users/' + users.learner.id
                + '/events?limit=2&before=' + encodeURIComponent(page1Json.nextBefore),
            'admin'
        );
        assert.equal(page2.status, 200);
        const page2Json = await page2.json();
        assert.ok(page2Json.events.length >= 1);
        const ids = new Set(page1Json.events.map((e) => e.id));
        for (const ev of page2Json.events) {
            assert.ok(!ids.has(ev.id), 'pages must not duplicate events');
        }

        const clamped = await api(
            'GET',
            '/api/admin/users/' + users.learner.id + '/events?limit=999',
            'admin'
        );
        assert.equal(clamped.status, 200);
        const clampedJson = await clamped.json();
        assert.ok(clampedJson.events.length <= 200);
    });

    it('ac-4b: each events read writes one audit row naming admin and target', async () => {
        const before = auditCount();
        const res = await api(
            'GET',
            '/api/admin/users/' + users.learner.id + '/events?limit=1',
            'admin'
        );
        assert.equal(res.status, 200);
        assert.equal(auditCount() - before, 1);
        const row = db.get('SELECT * FROM audit_log ORDER BY id DESC LIMIT 1');
        assert.equal(row.actor, 'admin');
        assert.equal(row.target, 'learner');
        assert.ok(String(row.action).toLowerCase().includes('event')
            || String(row.action).includes('readEvents')
            || String(row.action).includes('Events'));
    });

    it('ac-5: validator 422 leaves files unchanged; valid save is served', async () => {
        const slug = 'ac5-lesson';
        const good = sampleWorksheet(slug);
        const created = await api('POST', '/api/admin/lessons', 'admin', {
            slug,
            worksheet: good,
            catalogEntry: { slug, title: good.title, level: 'basic' }
        });
        assert.equal(created.status, 201);

        const wsPath = path.join(lessonsDir, slug, 'worksheet.js');
        const catPath = path.join(lessonsDir, 'catalog.js');
        const wsBefore = fs.readFileSync(wsPath);
        const catBefore = fs.readFileSync(catPath);

        const bad = sampleWorksheet(slug, {
            vocab: { items: [{ id: '1', word: 'שָׁלוֹם' }] }
        });
        const invalid = await api('PUT', '/api/admin/lessons/' + slug, 'admin', {
            worksheet: bad,
            catalogEntry: { slug, title: bad.title, level: 'basic' }
        });
        assert.equal(invalid.status, 422);
        const invalidJson = await invalid.json();
        assert.ok(Array.isArray(invalidJson.errors) && invalidJson.errors.length > 0);
        assert.equal(Buffer.compare(wsBefore, fs.readFileSync(wsPath)), 0);
        assert.equal(Buffer.compare(catBefore, fs.readFileSync(catPath)), 0);

        const updated = sampleWorksheet(slug, {
            vocab: {
                items: [{
                    id: '1',
                    word: 'שָׁלוֹם',
                    meaning: 'hello',
                    sound: 'sha-LOM',
                    trap: 'Stress the last syllable.',
                    sentence: 'שָׁלוֹם חָדָשׁ.'
                }]
            }
        });
        const valid = await api('PUT', '/api/admin/lessons/' + slug, 'admin', {
            worksheet: updated,
            catalogEntry: { slug, title: updated.title, level: 'basic' }
        });
        assert.equal(valid.status, 200);

        const served = await api('GET', '/worksheets/' + slug + '/worksheet.js', 'admin');
        assert.equal(served.status, 200);
        const servedText = await served.text();
        assert.ok(servedText.includes('שָׁלוֹם חָדָשׁ'), 'new sentence must be served');
    });

    it('ac-6: POST audio verifies in temp tree; second in-flight request is 409', async () => {
        const slug = 'ac6-audio';
        const worksheet = sampleWorksheet(slug);
        const created = await api('POST', '/api/admin/lessons', 'admin', {
            slug,
            worksheet,
            catalogEntry: { slug, title: worksheet.title, level: 'basic' }
        });
        assert.equal(created.status, 201);

        const httpsStub = stubHttps('AC6-TTS-BYTES');
        httpsStub.setDelay(800);
        try {
            const firstPromise = api('POST', '/api/admin/lessons/' + slug + '/audio', 'admin', {});
            await new Promise((r) => setTimeout(r, 150));
            const second = await api('POST', '/api/admin/lessons/' + slug + '/audio', 'admin', {});
            assert.equal(second.status, 409);
            const secondJson = await second.json();
            assert.equal(secondJson.code, 'audio_in_flight');

            const first = await firstPromise;
            assert.equal(first.status, 200);
            const firstJson = await first.json();
            assert.ok(firstJson.verifyCount != null || firstJson.summary != null);
        } finally {
            httpsStub.restore();
        }
    });

    it('ac-7: no admin response contains password_hash, session token, or test ElevenLabs key', async () => {
        const bodies = [];

        async function capture(method, pathName, body) {
            const res = await api(method, pathName, 'admin', body);
            const text = await res.text();
            bodies.push({ method, path: pathName, status: res.status, text });
            return { res, text };
        }

        await capture('GET', '/api/admin/users');
        await capture('POST', '/api/admin/users', {
            username: 'scan-user-' + Date.now(),
            password: harness.GOOD_PASSWORD,
            role: 'user'
        });
        await capture('PATCH', '/api/admin/users/' + users.learner.id, {
            displayName: 'Learner Renamed'
        });
        await capture('POST', '/api/admin/users/' + users.learner.id + '/password', {
            password: harness.GOOD_PASSWORD + 'Y'
        });
        {
            const session = accounts.signIn('learner', harness.GOOD_PASSWORD + 'Y', { ip: '127.0.0.1' });
            users.learner.token = session.token;
            users.learner.password = harness.GOOD_PASSWORD + 'Y';
        }
        await capture('GET', '/api/admin/users/' + users.learner.id + '/progress');
        await capture('GET', '/api/admin/users/' + users.learner.id + '/events?limit=5');
        await capture('GET', '/api/admin/lessons');
        await capture('PUT', '/api/admin/lessons/public-lesson/visibility', {
            mode: 'everyone'
        });
        await capture('GET', '/api/admin/lessons/public-lesson');

        const scanSlug = 'scan-lesson-' + Date.now();
        const scanWs = sampleWorksheet(scanSlug);
        await capture('POST', '/api/admin/lessons', {
            slug: scanSlug,
            worksheet: scanWs,
            catalogEntry: { slug: scanSlug, title: scanWs.title, level: 'basic' }
        });
        await capture('POST', '/api/admin/lessons/' + scanSlug + '/validate', {
            worksheet: scanWs,
            catalogEntry: { slug: scanSlug, title: scanWs.title, level: 'basic' }
        });
        await capture('PUT', '/api/admin/lessons/' + scanSlug, {
            worksheet: scanWs,
            catalogEntry: { slug: scanSlug, title: scanWs.title, level: 'basic' }
        });
        await capture('GET', '/api/admin/lessons/' + scanSlug + '/audio-plan');

        const httpsStub = stubHttps('SCAN-TTS');
        try {
            await capture('POST', '/api/admin/lessons/' + scanSlug + '/audio', {});
        } finally {
            httpsStub.restore();
        }

        const adminToken = users.admin.token;
        const learnerToken = users.learner.token;

        for (const entry of bodies) {
            assert.ok(
                !entry.text.includes('password_hash'),
                entry.method + ' ' + entry.path + ' must not contain password_hash'
            );
            assert.ok(
                !entry.text.includes(adminToken),
                entry.method + ' ' + entry.path + ' must not contain admin session token'
            );
            assert.ok(
                !entry.text.includes(learnerToken),
                entry.method + ' ' + entry.path + ' must not contain learner session token'
            );
            assert.ok(
                !entry.text.includes(TEST_ELEVENLABS_KEY),
                entry.method + ' ' + entry.path + ' must not contain test ElevenLabs key'
            );
        }
    });
});
