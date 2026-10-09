/**
 * HebrewSync unit tests: queue, retry, idempotency, per-user keys, 409, cap.
 * Provenance: INIT-002/SPEC-013
 * Time is injected; no real timers.
 */
'use strict';

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const path = require('path');
const fs = require('fs');
const vm = require('vm');

const PENDING_TEXT = 'עדיין לא נשמר — ננסה שוב. (Not saved yet — will retry.)';

function makeFakeStorage() {
    const data = Object.create(null);
    return {
        getItem(key) {
            return Object.prototype.hasOwnProperty.call(data, key) ? data[key] : null;
        },
        setItem(key, value) {
            data[key] = String(value);
        },
        removeItem(key) {
            delete data[key];
        },
        clear() {
            Object.keys(data).forEach((k) => delete data[k]);
        },
        _data: data
    };
}

function loadModules(localStorage, opts) {
    const options = opts || {};
    const scheduled = [];
    let clock = options.clock != null ? options.clock : 1_700_000_000_000;
    let uuidSeq = 0;

    const sandbox = {
        console,
        localStorage,
        crypto: {
            randomUUID() {
                uuidSeq += 1;
                return 'uuid-' + uuidSeq;
            }
        },
        setTimeout() { throw new Error('real setTimeout must not run in unit tests'); },
        clearTimeout() { /* ignore */ },
        Date,
        JSON,
        Math,
        Object,
        Array,
        String,
        Number,
        Boolean,
        Error,
        Promise,
        module: { exports: {} },
        exports: {},
        window: null,
        globalThis: null,
        document: {
            querySelector() { return options.syncEl || null; },
            addEventListener() { /* no-op */ },
            visibilityState: 'visible'
        },
        addEventListener() { /* no-op */ },
        fetch: options.fetch || null,
        HebrewApi: options.HebrewApi || null,
        WORKSHEET: options.WORKSHEET || null
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);

    const reviewSrc = fs.readFileSync(
        path.resolve(__dirname, '..', 'shared', 'player', 'review.js'),
        'utf8'
    );
    const syncSrc = fs.readFileSync(
        path.resolve(__dirname, '..', 'shared', 'player', 'sync.js'),
        'utf8'
    );
    vm.runInContext(reviewSrc, sandbox);
    vm.runInContext(syncSrc, sandbox);

    const Sync = sandbox.HebrewSync;
    Sync._test.setNow(() => clock);
    Sync._test.setSchedule((fn, ms) => {
        const id = scheduled.length + 1;
        scheduled.push({ id, fn, ms: Number(ms) || 0, due: clock + (Number(ms) || 0) });
        return id;
    }, (id) => {
        const idx = scheduled.findIndex((t) => t.id === id);
        if (idx >= 0) scheduled.splice(idx, 1);
    });
    Sync._test.setUuid(() => {
        uuidSeq += 1;
        return 'uuid-' + uuidSeq;
    });

    return {
        Sync,
        Review: sandbox.HebrewReview,
        sandbox,
        scheduled,
        advance(ms) {
            clock += ms;
            const due = scheduled.filter((t) => t.due <= clock).sort((a, b) => a.due - b.due);
            due.forEach((t) => {
                const idx = scheduled.indexOf(t);
                if (idx >= 0) scheduled.splice(idx, 1);
                t.fn();
            });
        },
        flushScheduled() {
            while (scheduled.length) {
                const next = scheduled.sort((a, b) => a.due - b.due)[0];
                clock = Math.max(clock, next.due);
                this.advance(0);
            }
        },
        get clock() { return clock; },
        set clock(v) { clock = v; }
    };
}

function fakeApi(handlers) {
    const log = [];
    const api = {
        log,
        me(opts) {
            log.push({ method: 'GET', path: '/api/me', opts });
            return Promise.resolve(handlers.me ? handlers.me() : {
                ok: true,
                status: 200,
                data: { id: 7, username: 'learner', role: 'user' }
            });
        },
        get(path, opts) {
            log.push({ method: 'GET', path, opts });
            if (handlers.get) return Promise.resolve(handlers.get(path, opts));
            if (path === '/api/review/state') {
                return Promise.resolve({
                    ok: true,
                    status: 200,
                    data: { v: 1, words: {}, migratedLessons: {} }
                });
            }
            if (/^\/api\/lessons\/[^/]+\/progress$/.test(path)) {
                return Promise.resolve({
                    ok: true,
                    status: 200,
                    data: { state: {}, rev: 0 }
                });
            }
            return Promise.resolve({ ok: false, status: 404, data: null });
        },
        post(path, body, opts) {
            log.push({ method: 'POST', path, body, opts });
            if (handlers.post) return Promise.resolve(handlers.post(path, body, opts));
            return Promise.resolve({
                ok: true,
                status: 200,
                data: { applied: (body.signals || []).length, rejected: 0, state: { v: 1, words: {}, migratedLessons: {} } }
            });
        },
        request(path, opts) {
            log.push({ method: (opts && opts.method) || 'GET', path, body: opts && opts.body, opts });
            if (handlers.request) return Promise.resolve(handlers.request(path, opts));
            return Promise.resolve({ ok: true, status: 200, data: { state: opts && opts.body && opts.body.state, rev: 1 } });
        }
    };
    return api;
}

describe('HebrewSync (SPEC-013)', () => {
    let storage;
    let env;

    beforeEach(() => {
        storage = makeFakeStorage();
    });

    afterEach(() => {
        if (env && env.Sync) env.Sync._resetForTests();
    });

    it('queues signals with uuid, flushes batches, and retries with backoff', async () => {
        let posts = 0;
        const api = fakeApi({
            post(path, body) {
                posts += 1;
                if (posts === 1) {
                    return { ok: false, status: 500, data: { error: 'fail' } };
                }
                return {
                    ok: true,
                    status: 200,
                    data: {
                        applied: body.signals.length,
                        rejected: 0,
                        state: {
                            v: 1,
                            words: {
                                'שָׁלוֹם': {
                                    he_en: { due: '2099-01-01T00:00:00.000Z', step: 1, streak: 1, lapses: 0, lastResult: 'pass', lastAt: '2020-01-01T00:00:00.000Z' },
                                    en_he: { due: '2099-01-01T00:00:00.000Z', step: 0, streak: 0, lapses: 0, lastResult: null, lastAt: null },
                                    sources: ['public-lesson']
                                }
                            },
                            migratedLessons: {}
                        }
                    }
                };
            }
        });
        const syncEl = { textContent: '', setAttribute() {} };
        env = loadModules(storage, { HebrewApi: api, syncEl });
        const { Sync } = env;

        await Sync.init({});
        assert.equal(Sync.isActive(), true);
        assert.equal(Sync.getStatus(), 'synced');

        Sync.queueSignal({
            word: 'שָׁלוֹם',
            direction: 'he_en',
            result: 'pass',
            slug: 'public-lesson'
        });
        assert.equal(Sync.getStatus(), 'pending');
        assert.equal(syncEl.textContent, PENDING_TEXT);
        assert.equal(Sync._test.readQueue(7).length, 1);
        assert.equal(Sync._test.readQueue(7)[0].id, 'uuid-1');

        // First scheduled flush fails → backoff
        env.advance(0);
        await Promise.resolve();
        await Promise.resolve();
        assert.equal(posts, 1);
        assert.equal(Sync.getStatus(), 'pending');
        assert.equal(Sync._test.readQueue(7).length, 1);

        // After backoff, second flush succeeds and adopts server state
        env.advance(1000);
        await Promise.resolve();
        await Promise.resolve();
        assert.equal(posts, 2);
        assert.equal(Sync.getStatus(), 'synced');
        assert.equal(Sync._test.readQueue(7).length, 0);
        assert.equal(Sync.getReviewState().words['שָׁלוֹם'].he_en.lastResult, 'pass');
        assert.equal(env.Review.load().words['שָׁלוֹם'].he_en.step, 1);
    });

    it('second flush of same ids is idempotent (queue cleared after first success)', async () => {
        const seen = [];
        const api = fakeApi({
            post(path, body) {
                seen.push(body.signals.map((s) => s.id));
                return {
                    ok: true,
                    status: 200,
                    data: { applied: body.signals.length, rejected: 0, state: { v: 1, words: {}, migratedLessons: {} } }
                };
            }
        });
        env = loadModules(storage, { HebrewApi: api });
        const { Sync } = env;
        await Sync.init({});
        Sync.queueSignal({ word: 'א', direction: 'he_en', result: 'pass', slug: 'public-lesson', id: 'fixed-1' });
        Sync.queueSignal({ word: 'ב', direction: 'he_en', result: 'miss', slug: 'public-lesson', id: 'fixed-2' });
        env.advance(0);
        await Promise.resolve();
        await Promise.resolve();
        assert.deepEqual(seen[0], ['fixed-1', 'fixed-2']);
        assert.equal(Sync._test.readQueue(7).length, 0);

        // Manual second flush with empty queue does nothing
        await Sync.flush();
        assert.equal(seen.length, 1);
        assert.equal(Sync.getStatus(), 'synced');
    });

    it('per-user queue keys: user A queue is not read when user B is signed in', async () => {
        const apiA = fakeApi({
            me() {
                return { ok: true, status: 200, data: { id: 1, username: 'a', role: 'user' } };
            }
        });
        env = loadModules(storage, { HebrewApi: apiA });
        await env.Sync.init({});
        env.Sync.queueSignal({ word: 'א', direction: 'he_en', result: 'pass', slug: 'x', id: 'a-1' });
        assert.ok(storage.getItem('hebrew-sync:v1:1'));
        assert.equal(JSON.parse(storage.getItem('hebrew-sync:v1:1')).length, 1);
        env.Sync._resetForTests();

        const apiB = fakeApi({
            me() {
                return { ok: true, status: 200, data: { id: 2, username: 'b', role: 'user' } };
            }
        });
        env = loadModules(storage, { HebrewApi: apiB });
        await env.Sync.init({});
        assert.equal(env.Sync._test.readQueue(2).length, 0);
        assert.equal(JSON.parse(storage.getItem('hebrew-sync:v1:1')).length, 1);
        assert.equal(storage.getItem('hebrew-sync:v1:2'), null);
        // B must not flush A's queue
        const posts = apiB.log.filter((e) => e.method === 'POST');
        env.advance(0);
        await Promise.resolve();
        assert.equal(posts.length, 0);
    });

    it('409 on lesson progress adopts server state without throwing', async () => {
        const serverState = { lesson: 'public-lesson', round: 3, completedRounds: 1 };
        let conflictCalls = 0;
        const api = fakeApi({
            request(path, opts) {
                if (opts && opts.method === 'PUT') {
                    return {
                        ok: false,
                        status: 409,
                        data: { state: serverState, rev: 5 }
                    };
                }
                return { ok: true, status: 200, data: {} };
            }
        });
        env = loadModules(storage, {
            HebrewApi: api,
            WORKSHEET: { id: 'public-lesson' }
        });
        const { Sync } = env;
        Sync.setOnLessonConflict(() => { conflictCalls += 1; });
        await Sync.init({ lessonSlug: 'public-lesson' });
        assert.equal(Sync.getLessonProgress().rev, 0);

        Sync.saveLessonProgress({ lesson: 'public-lesson', round: 1 });
        env.advance(env.Sync._test.LESSON_DEBOUNCE_MS);
        await Promise.resolve();
        await Promise.resolve();

        const prog = Sync.getLessonProgress();
        assert.equal(prog.rev, 5);
        assert.equal(prog.state.round, 3);
        assert.equal(conflictCalls, 1);
    });

    it('overlapping lesson saves go one at a time and reuse the returned revision (no false 409)', async () => {
        const puts = [];
        const releases = [];
        let conflictCalls = 0;
        const api = fakeApi({
            request(path, opts) {
                if (!opts || opts.method !== 'PUT') return { ok: true, status: 200, data: {} };
                const body = JSON.parse(JSON.stringify(opts.body));
                puts.push(body);
                // Same rule as the server: a stale revision is a conflict.
                const expected = puts.length - 1;
                return new Promise((resolve) => {
                    releases.push(() => resolve(body.rev === expected
                        ? { ok: true, status: 200, data: { state: body.state, rev: body.rev + 1 } }
                        : { ok: false, status: 409, data: { state: { round: 1 }, rev: expected } }));
                });
            }
        });
        env = loadModules(storage, { HebrewApi: api, WORKSHEET: { id: 'public-lesson' } });
        const { Sync } = env;
        Sync.setOnLessonConflict(() => { conflictCalls += 1; });
        await Sync.init({ lessonSlug: 'public-lesson' });

        // First save is sent and is still waiting for the server.
        Sync.saveLessonProgress({ lesson: 'public-lesson', round: 1, answers: ['a'] });
        env.advance(Sync._test.LESSON_DEBOUNCE_MS);
        assert.equal(puts.length, 1);

        // A second save fires while the first is in flight. It must wait, not go out with rev 0.
        Sync.saveLessonProgress({ lesson: 'public-lesson', round: 1, answers: ['a', 'b'] });
        env.advance(Sync._test.LESSON_DEBOUNCE_MS);
        await Promise.resolve();
        assert.equal(puts.length, 1, 'no second request while the first is in flight');

        releases[0]();
        await new Promise((resolve) => setImmediate(resolve));
        assert.equal(puts.length, 2, 'the newer answers are sent after the first returns');
        assert.equal(puts[1].rev, 1, 'follow-up carries the revision the server returned');
        assert.deepEqual(puts[1].state.answers, ['a', 'b']);

        releases[1]();
        await new Promise((resolve) => setImmediate(resolve));
        assert.equal(conflictCalls, 0, 'no conflict, so the page is not redrawn');
        assert.equal(Sync.getLessonProgress().rev, 2);
        assert.deepEqual(Sync.getLessonProgress().state.answers, ['a', 'b']);
    });

    it('queue cap 1000 drops oldest and console.warns once per drop', async () => {
        const warnings = [];
        const origWarn = console.warn;
        console.warn = (...args) => { warnings.push(args.join(' ')); };
        try {
            const api = fakeApi({
                post() {
                    // Never succeed so queue stays full
                    return { ok: false, status: 503, data: null };
                }
            });
            env = loadModules(storage, { HebrewApi: api });
            const { Sync } = env;
            // Skip automatic flush during enqueue by not advancing timers
            Sync._test.setUser({ id: 7, username: 'learner' });
            Sync._test.setStatus('synced');

            for (let i = 0; i < 1000; i += 1) {
                Sync.queueSignal({
                    word: 'w' + i,
                    direction: 'he_en',
                    result: 'pass',
                    slug: 'public-lesson',
                    id: 'id-' + i
                });
            }
            assert.equal(Sync._test.readQueue(7).length, 1000);
            assert.equal(warnings.length, 0);

            Sync.queueSignal({
                word: 'overflow',
                direction: 'he_en',
                result: 'pass',
                slug: 'public-lesson',
                id: 'id-overflow'
            });
            const q = Sync._test.readQueue(7);
            assert.equal(q.length, 1000);
            assert.equal(q[0].id, 'id-1');
            assert.equal(q[q.length - 1].id, 'id-overflow');
            assert.equal(warnings.length, 1);
            assert.match(warnings[0], /dropped oldest/i);
        } finally {
            console.warn = origWarn;
        }
    });

    it('legacy hebrew-review and hebrew-worksheet keys stay untouched while signed in', async () => {
        const legacyReview = JSON.stringify({ v: 1, words: { X: {} }, migratedLessons: {} });
        const legacyLesson = JSON.stringify({ lesson: 'public-lesson', round: 9 });
        storage.setItem('hebrew-review:v1', legacyReview);
        storage.setItem('hebrew-worksheet:public-lesson', legacyLesson);

        const api = fakeApi({});
        env = loadModules(storage, { HebrewApi: api, WORKSHEET: { id: 'public-lesson' } });
        const { Sync, Review } = env;
        await Sync.init({ lessonSlug: 'public-lesson' });

        Review.load();
        Review.record('שָׁלוֹם', 'he_en', 'pass', { lesson: 'public-lesson' });
        Review.migrateLessonMarks({ wordMarks: { '1': 'knew' } }, { id: 'public-lesson', vocab: { items: [] } });

        assert.equal(storage.getItem('hebrew-review:v1'), legacyReview);
        assert.equal(storage.getItem('hebrew-worksheet:public-lesson'), legacyLesson);
        assert.ok(storage.getItem('hebrew-sync:v1:7'));
    });

    it('Review.record while sync active queues and does not write hebrew-review:v1', async () => {
        storage.setItem('hebrew-review:v1', '{"v":1,"words":{},"migratedLessons":{}}');
        const before = storage.getItem('hebrew-review:v1');
        const api = fakeApi({});
        env = loadModules(storage, { HebrewApi: api });
        await env.Sync.init({});
        env.Review.load();
        env.Review.record('עוֹלָם', 'he_en', 'miss', { lesson: 'public-lesson', activity: 'words' });
        const q = env.Sync._test.readQueue(7);
        assert.equal(q.length, 1);
        assert.equal(q[0].word, 'עוֹלָם');
        assert.equal(q[0].result, 'miss');
        assert.equal(q[0].slug, 'public-lesson');
        assert.equal(storage.getItem('hebrew-review:v1'), before);
    });
});
