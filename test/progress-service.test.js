/**
 * Server progress service: signals, isolation, lesson blobs, equivalence.
 * Provenance: INIT-002/SPEC-005
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const DAY_MS = 24 * 60 * 60 * 1000;

function atLocal(y, m, d, h = 12) {
    return new Date(y, m - 1, d, h, 0, 0, 0).getTime();
}

describe('progress service (SPEC-005)', () => {
    let tmp;
    let db;
    let users;
    let progress;
    let HebrewReview;
    let events;
    let userA;
    let userB;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-progress-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/migrations/001-initial')];
        for (const name of ['users', 'review', 'events', 'lesson-progress']) {
            delete require.cache[require.resolve('../server/db/repos/' + name)];
        }
        delete require.cache[require.resolve('../server/progress')];
        delete require.cache[require.resolve('../shared/player/review.js')];

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        users = require('../server/db/repos/users');
        events = require('../server/db/repos/events');
        progress = require('../server/progress');
        HebrewReview = require('../shared/player/review.js');

        userA = Number(users.create({
            username: 'alice',
            password_hash: 'hash-a',
            display_name: 'Alice'
        }).lastInsertRowid);
        userB = Number(users.create({
            username: 'bob',
            password_hash: 'hash-b',
            display_name: 'Bob'
        }).lastInsertRowid);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/progress')];
    });

    beforeEach(() => {
        db.run('DELETE FROM practice_events');
        db.run('DELETE FROM review_items');
        db.run('DELETE FROM lesson_progress');
    });

    function signal(partial) {
        return Object.assign({
            id: 'sig-' + Math.random().toString(36).slice(2),
            word: 'קָמָה',
            direction: 'he_en',
            result: 'pass',
            at: new Date().toISOString(),
            slug: 'my-morning',
            activity: 'words'
        }, partial);
    }

    it('ac-1: pass then same-day pass matches HebrewReview.nextState (one step only)', () => {
        const day0 = atLocal(2026, 10, 1, 10);
        const later = atLocal(2026, 10, 1, 18);
        const word = 'קָמָה';

        const direct1 = HebrewReview.nextState(null, 'pass', day0);
        const direct2 = HebrewReview.nextState(direct1, 'pass', later);

        progress.applySignals(userA, [
            signal({ id: 'p1', word, result: 'pass', at: new Date(day0).toISOString() })
        ], day0);
        const mid = progress.reviewState(userA).words[word].he_en;
        assert.deepEqual(
            { due: mid.due, step: mid.step, streak: mid.streak, lapses: mid.lapses,
                lastResult: mid.lastResult, lastAt: mid.lastAt },
            direct1
        );

        progress.applySignals(userA, [
            signal({ id: 'p2', word, result: 'pass', at: new Date(later).toISOString() })
        ], later);
        const end = progress.reviewState(userA).words[word].he_en;
        assert.equal(end.step, 1);
        assert.equal(end.step, direct2.step);
        assert.equal(end.due, direct2.due);
        assert.equal(end.streak, direct2.streak);
    });

    it('ac-1: miss resets to step 0 due +1 day — equals nextState', () => {
        const day0 = atLocal(2026, 10, 2, 10);
        const word = 'הוֹלֶכֶת';
        progress.applySignals(userA, [
            signal({ id: 'm-setup', word, result: 'pass' })
        ], day0 - 4 * DAY_MS);

        const before = progress.reviewState(userA).words[word].he_en;
        const direct = HebrewReview.nextState(before, 'miss', day0);

        progress.applySignals(userA, [
            signal({ id: 'm1', word, result: 'miss', at: new Date(day0).toISOString() })
        ], day0);
        const after = progress.reviewState(userA).words[word].he_en;
        assert.equal(after.step, 0);
        assert.equal(after.due, direct.due);
        assert.equal(Date.parse(after.due), day0 + DAY_MS);
        assert.equal(after.lapses, direct.lapses);
        assert.equal(after.lastResult, 'miss');
    });

    it('ac-1: not_yet matches nextState (no step advance)', () => {
        const day0 = atLocal(2026, 10, 3, 10);
        const word = 'אוֹהֶבֶת';
        const direct = HebrewReview.nextState(null, 'not_yet', day0);
        progress.applySignals(userA, [
            signal({ id: 'ny1', word, result: 'not_yet' })
        ], day0);
        const after = progress.reviewState(userA).words[word].he_en;
        assert.equal(after.step, direct.step);
        assert.equal(after.lastResult, 'not_yet');
        assert.equal(after.streak, 0);
    });

    it('ac-2: same signal ids twice are idempotent', () => {
        const day0 = atLocal(2026, 10, 4, 10);
        const batch = [
            signal({ id: 'idem-1', word: 'בֹּקֶר', result: 'pass' }),
            signal({ id: 'idem-2', word: 'בֹּקֶר', direction: 'en_he', result: 'miss' })
        ];
        const first = progress.applySignals(userA, batch, day0);
        assert.equal(first.applied, 2);
        assert.equal(first.rejected, 0);
        const state1 = progress.reviewState(userA);
        const events1 = events.listByUser(userA, 100);

        const second = progress.applySignals(userA, batch, day0 + DAY_MS);
        assert.equal(second.applied, 0);
        assert.equal(second.rejected, 0);
        const state2 = progress.reviewState(userA);
        const events2 = events.listByUser(userA, 100);

        assert.equal(events2.length, events1.length);
        assert.deepEqual(state2.words['בֹּקֶר'].he_en, state1.words['בֹּקֶר'].he_en);
        assert.deepEqual(state2.words['בֹּקֶר'].en_he, state1.words['בֹּקֶר'].en_he);
    });

    it('ac-3: user A signals never change user B records/events/metrics', () => {
        const day0 = atLocal(2026, 10, 5, 10);
        progress.applySignals(userB, [
            signal({ id: 'b-seed', word: 'שָׁלוֹם', result: 'pass' })
        ], day0);
        const bBefore = {
            state: progress.reviewState(userB),
            events: events.listByUser(userB, 100),
            metrics: progress.metrics(userB, day0)
        };

        progress.applySignals(userA, [
            signal({ id: 'a-1', word: 'שָׁלוֹם', result: 'miss' }),
            signal({ id: 'a-2', word: 'קָמָה', result: 'pass' })
        ], day0);

        const bAfter = {
            state: progress.reviewState(userB),
            events: events.listByUser(userB, 100),
            metrics: progress.metrics(userB, day0)
        };
        assert.deepEqual(bAfter.state, bBefore.state);
        assert.deepEqual(bAfter.events, bBefore.events);
        assert.deepEqual(bAfter.metrics, bBefore.metrics);
        assert.equal(events.listByUser(userA, 100).length, 2);
    });

    it('ac-6: putLessonProgress rejects unknown keys and oversize bodies', () => {
        const bad = progress.putLessonProgress(userA, 'my-morning', { evil: true }, 0);
        assert.equal(bad.ok, false);
        assert.match(bad.error, /unknown key/i);

        const huge = { wordMarks: { x: 'y'.repeat(70000) } };
        const over = progress.putLessonProgress(userA, 'my-morning', huge, 0);
        assert.equal(over.ok, false);
        assert.match(over.error, /64 KB/i);
    });

    it('ac-6: stale rev returns conflict with stored state', () => {
        const state1 = {
            lesson: 'my-morning',
            round: 1,
            wordMarks: { '1': 'knew' }
        };
        const put1 = progress.putLessonProgress(userA, 'my-morning', state1, 0);
        assert.equal(put1.ok, true);
        assert.equal(put1.rev, 1);

        const state2 = { lesson: 'my-morning', round: 2 };
        const conflict = progress.putLessonProgress(userA, 'my-morning', state2, 0);
        assert.equal(conflict.ok, false);
        assert.equal(conflict.conflict, true);
        assert.equal(conflict.rev, 1);
        assert.equal(conflict.state.round, 1);

        const ok = progress.putLessonProgress(userA, 'my-morning', state2, 1);
        assert.equal(ok.ok, true);
        assert.equal(ok.rev, 2);
        assert.equal(progress.getLessonProgress(userA, 'my-morning').rev, 2);
    });

    it('ac-7: bad signals are skipped and counted; valid ones apply', () => {
        const day0 = atLocal(2026, 10, 6, 10);
        const result = progress.applySignals(userA, [
            signal({ id: 'bad-dir', direction: 'sideways', result: 'pass' }),
            signal({ id: 'bad-res', result: 'maybe' }),
            signal({ id: 'bad-word', word: '   ' }),
            signal({ id: 'good-1', word: 'קָמָה', result: 'pass' })
        ], day0);
        assert.equal(result.rejected, 3);
        assert.equal(result.applied, 1);
        assert.ok(result.state.words['קָמָה']);
        assert.equal(result.state.words['קָמָה'].he_en.lastResult, 'pass');
        assert.equal(events.listByUser(userA, 100).length, 1);
    });

    it('uses server now for scheduling, not client at', () => {
        const serverNow = atLocal(2026, 10, 7, 10);
        const clientAt = new Date(serverNow - 10 * DAY_MS).toISOString();
        const word = 'אַחַר כָּךְ';
        progress.applySignals(userA, [
            signal({ id: 'clock-1', word, result: 'pass', at: clientAt })
        ], serverNow);
        const rec = progress.reviewState(userA).words[word].he_en;
        const expected = HebrewReview.nextState(null, 'pass', serverNow);
        assert.equal(rec.due, expected.due);
        assert.equal(rec.lastAt, expected.lastAt);
        assert.equal(events.find(userA, 'clock-1').at, clientAt);
    });

    it('reviewState returns ADR-0001 store shape (v:1, words)', () => {
        const day0 = atLocal(2026, 10, 8, 10);
        progress.applySignals(userA, [
            signal({ id: 'shape-1', word: 'קָמָה', result: 'pass', slug: 'my-morning' })
        ], day0);
        const state = progress.reviewState(userA);
        assert.equal(state.v, 1);
        assert.ok(state.words);
        assert.ok(state.migratedLessons);
        const entry = state.words['קָמָה'];
        assert.ok(entry.he_en);
        assert.ok(entry.en_he);
        assert.ok(Array.isArray(entry.sources));
        assert.ok(entry.sources.includes('my-morning'));
        assert.equal(typeof entry.he_en.due, 'string');
        assert.equal(typeof entry.he_en.step, 'number');
        assert.equal(entry.he_en.lastResult, 'pass');
    });
});
