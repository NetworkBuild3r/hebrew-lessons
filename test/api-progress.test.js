/**
 * Learner progress API routes (SPEC-009 ac-1 … ac-6).
 * Provenance: INIT-002/SPEC-009
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');

const harness = require('./helpers/signed-in-server');

describe('api progress (SPEC-009)', () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(u: string) => string} */
    let cookieFor;
    /** @type {Record<string, { id: number, username: string, role: string, token: string }>} */
    let users;
    /** @type {typeof import('../server/progress')} */
    let progress;
    /** @type {typeof import('../server/visibility')} */
    let visibility;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        users = started.users;
        progress = require('../server/progress');
        visibility = require('../server/visibility');
        // Restricted lesson: hidden from non-admins (fixture already hidden; re-assert via API).
        visibility.setVisibility('mcp', 'hidden-lesson', 'hidden');
    });

    after(async () => {
        await harness.stop();
    });

    function jsonHeaders(username, extra) {
        return Object.assign({
            'Content-Type': 'application/json',
            Origin: baseUrl,
            Cookie: cookieFor(username)
        }, extra || {});
    }

    async function api(method, path, username, body) {
        const opts = {
            method,
            headers: jsonHeaders(username)
        };
        if (body !== undefined) {
            opts.body = typeof body === 'string' ? body : JSON.stringify(body);
        }
        return fetch(baseUrl + path, opts);
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

    it('ac-1: session user A cannot read or change user B progress via routes or userId injection', async () => {
        const a = users.learner;
        const b = users.admin;

        // Seed B's own data via B's session.
        const seedSignals = [
            signal({ id: 'b-seed-1', word: 'שָׁלוֹם', result: 'pass', slug: 'public-lesson' })
        ];
        const seedRes = await api('POST', '/api/review/signals', 'admin', { signals: seedSignals });
        assert.equal(seedRes.status, 200);
        const seedJson = await seedRes.json();
        assert.ok(seedJson.applied >= 1);

        const putB = await api('PUT', '/api/lessons/public-lesson/progress', 'admin', {
            state: { lesson: 'public-lesson', round: 3 },
            rev: 0
        });
        assert.equal(putB.status, 200);
        const putBJson = await putB.json();
        assert.equal(putBJson.rev, 1);

        const bReviewBefore = progress.reviewState(b.id);
        const bLessonBefore = progress.getLessonProgress(b.id, 'public-lesson');
        const bMetricsBefore = progress.metrics(b.id);

        // A tries to read B via query userId — must still get A's empty/own data.
        const getReview = await api(
            'GET',
            '/api/review/state?userId=' + encodeURIComponent(String(b.id)),
            'learner'
        );
        assert.equal(getReview.status, 200);
        assert.equal(getReview.headers.get('cache-control'), 'no-store');
        const reviewA = await getReview.json();
        assert.equal(reviewA.v, 1);
        assert.ok(reviewA.words);
        // A's review must not include B's seeded word state as if it were A's.
        // (A has no signals yet — word absent or not matching B.)
        const aWord = reviewA.words['שָׁלוֹם'];
        if (aWord && aWord.he_en && aWord.he_en.lastResult) {
            assert.notEqual(
                JSON.stringify(aWord.he_en),
                JSON.stringify(bReviewBefore.words['שָׁלוֹם'].he_en)
            );
        }

        const getMetrics = await api(
            'GET',
            '/api/me/metrics?userId=' + encodeURIComponent(String(b.id)),
            'learner'
        );
        assert.equal(getMetrics.status, 200);
        const metricsA = await getMetrics.json();
        assert.ok(metricsA.stages);
        assert.ok(metricsA.accuracy);
        // B had a pass; A has none — stages differ or A's new counts stay low.
        assert.notDeepEqual(metricsA, bMetricsBefore);

        const getLesson = await api(
            'GET',
            '/api/lessons/public-lesson/progress?userId=' + encodeURIComponent(String(b.id)),
            'learner'
        );
        assert.equal(getLesson.status, 200);
        const lessonA = await getLesson.json();
        assert.equal(lessonA.rev, 0);
        assert.deepEqual(lessonA.state, {});

        // A posts signals / puts progress with B's userId in body — only A changes.
        const injectSignals = [
            signal({ id: 'a-inject-1', word: 'עוֹלָם', result: 'pass', slug: 'public-lesson' })
        ];
        const postInject = await api('POST', '/api/review/signals', 'learner', {
            userId: b.id,
            signals: injectSignals
        });
        assert.equal(postInject.status, 200);
        const postInjectJson = await postInject.json();
        assert.equal(postInjectJson.applied, 1);

        const putInject = await api('PUT', '/api/lessons/public-lesson/progress', 'learner', {
            userId: b.id,
            state: { lesson: 'public-lesson', round: 9 },
            rev: 0
        });
        assert.equal(putInject.status, 200);
        const putInjectJson = await putInject.json();
        assert.equal(putInjectJson.rev, 1);
        assert.equal(putInjectJson.state.round, 9);

        // B unchanged.
        assert.deepEqual(progress.reviewState(b.id).words['שָׁלוֹם'], bReviewBefore.words['שָׁלוֹם']);
        assert.deepEqual(progress.getLessonProgress(b.id, 'public-lesson'), bLessonBefore);
        assert.deepEqual(progress.metrics(b.id), bMetricsBefore);

        // A owns the injected writes.
        const aState = progress.reviewState(a.id);
        assert.ok(aState.words['עוֹלָם']);
        const aLesson = progress.getLessonProgress(a.id, 'public-lesson');
        assert.equal(aLesson.rev, 1);
        assert.equal(aLesson.state.round, 9);

        // Unauthenticated → 401 (requireSession present).
        const bare = await fetch(baseUrl + '/api/review/state');
        assert.equal(bare.status, 401);
    });

    it('ac-2: same signal batch twice → second applied is 0 and state matches first', async () => {
        const batch = [
            signal({ id: 'idem-batch-1', word: 'שָׁלוֹם', result: 'pass' }),
            signal({ id: 'idem-batch-2', word: 'שָׁלוֹם', direction: 'en_he', result: 'miss' })
        ];
        const first = await api('POST', '/api/review/signals', 'learner', { signals: batch });
        assert.equal(first.status, 200);
        const firstJson = await first.json();
        assert.equal(firstJson.applied, 2);
        assert.equal(firstJson.rejected, 0);
        assert.ok(firstJson.state && firstJson.state.words);

        const second = await api('POST', '/api/review/signals', 'learner', { signals: batch });
        assert.equal(second.status, 200);
        const secondJson = await second.json();
        assert.equal(secondJson.applied, 0);
        assert.deepEqual(secondJson.state.words['שָׁלוֹם'], firstJson.state.words['שָׁלוֹם']);
    });

    it('ac-3: invisible slug signals rejected; GET/PUT progress on that slug return 404', async () => {
        const before = progress.reviewState(users.learner.id);

        const bad = await api('POST', '/api/review/signals', 'learner', {
            signals: [
                signal({
                    id: 'hidden-sig-1',
                    word: 'סוֹד',
                    slug: 'hidden-lesson',
                    result: 'pass'
                })
            ]
        });
        assert.equal(bad.status, 200);
        const badJson = await bad.json();
        assert.equal(badJson.applied, 0);
        assert.equal(badJson.rejected, 1);
        assert.deepEqual(
            progress.reviewState(users.learner.id).words['סוֹד'],
            before.words['סוֹד']
        );

        const getHidden = await api('GET', '/api/lessons/hidden-lesson/progress', 'learner');
        assert.equal(getHidden.status, 404);
        const getBody = await getHidden.json();
        assert.equal(getBody.error, 'Not found');
        assert.ok(!JSON.stringify(getBody).includes('stack'));

        const putHidden = await api('PUT', '/api/lessons/hidden-lesson/progress', 'learner', {
            state: { lesson: 'hidden-lesson', round: 1 },
            rev: 0
        });
        assert.equal(putHidden.status, 404);
        assert.equal((await putHidden.json()).error, 'Not found');
        assert.equal(progress.getLessonProgress(users.learner.id, 'hidden-lesson'), null);
    });

    it('ac-4: 201 signals and body over 64 KB return 413 and change nothing', async () => {
        const beforeState = progress.reviewState(users.learner.id);
        const beforeEvents = require('../server/db').all(
            'SELECT COUNT(*) AS n FROM practice_events WHERE user_id = ?',
            users.learner.id
        )[0].n;

        const tooMany = [];
        for (let i = 0; i < 201; i++) {
            tooMany.push(signal({ id: 'overflow-' + i, word: 'שָׁלוֹם', result: 'pass' }));
        }
        const res201 = await api('POST', '/api/review/signals', 'learner', { signals: tooMany });
        assert.equal(res201.status, 413);
        assert.equal(res201.headers.get('cache-control'), 'no-store');

        const after201 = require('../server/db').all(
            'SELECT COUNT(*) AS n FROM practice_events WHERE user_id = ?',
            users.learner.id
        )[0].n;
        assert.equal(after201, beforeEvents);
        assert.deepEqual(progress.reviewState(users.learner.id), beforeState);

        // Body over 64 KB → 413 from readJson (req.destroy may abort the socket first).
        const pad = 'x'.repeat(70 * 1024);
        const hugeBody = JSON.stringify({
            signals: [signal({ id: 'huge-1', word: 'שָׁלוֹם', pad })]
        });
        assert.ok(Buffer.byteLength(hugeBody, 'utf8') > 64 * 1024);
        let hugeStatus = null;
        try {
            const resHuge = await api('POST', '/api/review/signals', 'learner', hugeBody);
            hugeStatus = resHuge.status;
        } catch (err) {
            // body.js destroys the request when over limit; undici may see UND_ERR_SOCKET.
            assert.ok(err && (err.cause || err.message), 'expected refusal of oversize body');
            hugeStatus = 'aborted';
        }
        assert.ok(
            hugeStatus === 413 || hugeStatus === 'aborted',
            'expected 413 or connection abort, got ' + hugeStatus
        );

        const afterHuge = require('../server/db').all(
            'SELECT COUNT(*) AS n FROM practice_events WHERE user_id = ?',
            users.learner.id
        )[0].n;
        assert.equal(afterHuge, beforeEvents);
        assert.deepEqual(progress.reviewState(users.learner.id), beforeState);
    });

    it('ac-5: stale rev returns 409 with stored state; fresh rev succeeds and increments', async () => {
        const slug = 'public-lesson';
        // Ensure a known starting rev for this user/slug.
        const existing = progress.getLessonProgress(users.learner.id, slug);
        let rev = existing ? existing.rev : 0;
        if (!existing) {
            const init = await api('PUT', '/api/lessons/' + slug + '/progress', 'learner', {
                state: { lesson: slug, round: 1 },
                rev: 0
            });
            assert.equal(init.status, 200);
            rev = (await init.json()).rev;
        }

        const stale = await api('PUT', '/api/lessons/' + slug + '/progress', 'learner', {
            state: { lesson: slug, round: 99 },
            rev: 0
        });
        assert.equal(stale.status, 409);
        assert.equal(stale.headers.get('cache-control'), 'no-store');
        const staleJson = await stale.json();
        assert.equal(staleJson.rev, rev);
        assert.ok(staleJson.state);
        assert.notEqual(staleJson.state.round, 99);

        const fresh = await api('PUT', '/api/lessons/' + slug + '/progress', 'learner', {
            state: { lesson: slug, round: 7, vowelsOn: true },
            rev
        });
        assert.equal(fresh.status, 200);
        const freshJson = await fresh.json();
        assert.equal(freshJson.rev, rev + 1);
        assert.equal(freshJson.state.round, 7);
        assert.equal(freshJson.state.vowelsOn, true);

        const stored = progress.getLessonProgress(users.learner.id, slug);
        assert.equal(stored.rev, rev + 1);
        assert.equal(stored.state.round, 7);
    });

    it('ac-6: GET /api/me/metrics returns stages + accuracy with pct null under 10 attempts', async () => {
        const res = await api('GET', '/api/me/metrics', 'learner');
        assert.equal(res.status, 200);
        assert.equal(res.headers.get('cache-control'), 'no-store');
        const json = await res.json();
        assert.ok(json.stages);
        assert.ok(json.stages.he_en);
        assert.ok(json.stages.en_he);
        for (const dir of ['he_en', 'en_he']) {
            assert.equal(typeof json.stages[dir].new, 'number');
            assert.equal(typeof json.stages[dir].learning, 'number');
            assert.equal(typeof json.stages[dir].longTerm, 'number');
        }
        assert.ok(json.accuracy);
        for (const dir of ['he_en', 'en_he']) {
            assert.equal(typeof json.accuracy[dir].pass, 'number');
            assert.equal(typeof json.accuracy[dir].miss, 'number');
            const attempts = json.accuracy[dir].pass + json.accuracy[dir].miss;
            if (attempts < 10) {
                assert.equal(json.accuracy[dir].pct, null);
            }
        }
        // Learner has only a few signals in this suite → pct stays null.
        assert.equal(json.accuracy.he_en.pct, null);
        assert.equal(json.accuracy.en_he.pct, null);
    });
});
