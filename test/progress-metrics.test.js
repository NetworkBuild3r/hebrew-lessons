/**
 * Progress metrics, trouble words, answer history.
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

describe('progress metrics (SPEC-005)', () => {
    let tmp;
    let db;
    let users;
    let progress;
    let review;
    let events;
    let userA;
    let userB;
    const now = Date.parse('2026-10-08T12:00:00.000Z');

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-metrics-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/migrations/001-initial')];
        for (const name of ['users', 'review', 'events', 'lesson-progress']) {
            delete require.cache[require.resolve('../server/db/repos/' + name)];
        }
        delete require.cache[require.resolve('../server/progress')];

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        users = require('../server/db/repos/users');
        review = require('../server/db/repos/review');
        events = require('../server/db/repos/events');
        progress = require('../server/progress');
        progress._clearLookups();

        userA = Number(users.create({
            username: 'metric-a',
            password_hash: 'h',
            display_name: 'A'
        }).lastInsertRowid);
        userB = Number(users.create({
            username: 'metric-b',
            password_hash: 'h',
            display_name: 'B'
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
    });

    function upsertItem(userId, word, direction, fields) {
        review.upsert({
            user_id: userId,
            word,
            direction,
            due: fields.due || new Date(now).toISOString(),
            step: fields.step == null ? 0 : fields.step,
            streak: fields.streak == null ? 0 : fields.streak,
            lapses: fields.lapses == null ? 0 : fields.lapses,
            last_result: fields.last_result == null ? null : fields.last_result,
            last_at: fields.last_at == null ? null : fields.last_at,
            sources: JSON.stringify(fields.sources || [])
        });
    }

    function insertEvent(userId, partial) {
        events.insert({
            id: partial.id,
            user_id: userId,
            word: partial.word,
            direction: partial.direction || 'he_en',
            result: partial.result,
            at: partial.at || new Date(now).toISOString(),
            slug: partial.slug || 'my-morning',
            activity: partial.activity || 'words'
        });
    }

    it('ac-4: stages new / learning / long-term per direction from fixture', () => {
        // he_en: 2 new (never passed), 2 learning (step 0–2 after pass), 1 long-term
        upsertItem(userA, 'new-a', 'he_en', { step: 0, last_result: null });
        upsertItem(userA, 'new-b', 'he_en', { step: 0, last_result: 'miss', lapses: 1 });
        upsertItem(userA, 'learn-a', 'he_en', { step: 1, last_result: 'pass' });
        insertEvent(userA, { id: 'pass-learn-a', word: 'learn-a', result: 'pass' });
        upsertItem(userA, 'learn-b', 'he_en', { step: 0, last_result: 'miss', lapses: 1 });
        insertEvent(userA, { id: 'pass-learn-b', word: 'learn-b', result: 'pass',
            at: new Date(now - DAY_MS).toISOString() });
        upsertItem(userA, 'lt-a', 'he_en', { step: 3, last_result: 'pass' });
        insertEvent(userA, { id: 'pass-lt-a', word: 'lt-a', result: 'pass' });

        // en_he: 1 new, 1 learning, 1 long-term
        upsertItem(userA, 'en-new', 'en_he', { step: 0, last_result: 'not_yet' });
        upsertItem(userA, 'en-learn', 'en_he', { step: 2, last_result: 'pass' });
        insertEvent(userA, {
            id: 'pass-en-learn', word: 'en-learn', direction: 'en_he', result: 'pass'
        });
        upsertItem(userA, 'en-lt', 'en_he', { step: 4, last_result: 'pass' });
        insertEvent(userA, {
            id: 'pass-en-lt', word: 'en-lt', direction: 'en_he', result: 'pass'
        });

        const m = progress.metrics(userA, now);
        assert.deepEqual(m.stages.he_en, { new: 2, learning: 2, longTerm: 1 });
        assert.deepEqual(m.stages.en_he, { new: 1, learning: 1, longTerm: 1 });
    });

    it('ac-4: accuracy excludes not_yet; pct null under 10 attempts', () => {
        for (let i = 0; i < 4; i += 1) {
            insertEvent(userA, {
                id: 'p-' + i,
                word: 'w' + i,
                result: 'pass',
                at: new Date(now - i * 3600 * 1000).toISOString()
            });
        }
        for (let i = 0; i < 3; i += 1) {
            insertEvent(userA, {
                id: 'm-' + i,
                word: 'w' + i,
                result: 'miss',
                at: new Date(now - i * 3600 * 1000).toISOString()
            });
        }
        insertEvent(userA, {
            id: 'ny-1', word: 'w0', result: 'not_yet',
            at: new Date(now).toISOString()
        });
        // old event outside 30 days — ignored
        insertEvent(userA, {
            id: 'old', word: 'old', result: 'pass',
            at: new Date(now - 40 * DAY_MS).toISOString()
        });

        const m = progress.metrics(userA, now);
        assert.equal(m.accuracy.he_en.pass, 4);
        assert.equal(m.accuracy.he_en.miss, 3);
        assert.equal(m.accuracy.he_en.pct, null);

        for (let i = 0; i < 5; i += 1) {
            insertEvent(userA, {
                id: 'extra-p-' + i,
                word: 'x' + i,
                direction: 'en_he',
                result: 'pass',
                at: new Date(now - i * 1000).toISOString()
            });
        }
        for (let i = 0; i < 5; i += 1) {
            insertEvent(userA, {
                id: 'extra-m-' + i,
                word: 'x' + i,
                direction: 'en_he',
                result: 'miss',
                at: new Date(now - i * 1000).toISOString()
            });
        }
        const m2 = progress.metrics(userA, now);
        assert.equal(m2.accuracy.en_he.pass, 5);
        assert.equal(m2.accuracy.en_he.miss, 5);
        assert.equal(m2.accuracy.en_he.pct, 0.5);
        assert.equal(m2.accuracy.he_en.pct, null);
    });

    it('ac-5: troubleWords orders by misses then lapses, limit 10', () => {
        const words = [];
        for (let i = 0; i < 12; i += 1) {
            const word = 'trouble-' + i;
            words.push(word);
            upsertItem(userA, word, 'he_en', {
                step: 0,
                lapses: i === 5 ? 9 : i,
                last_result: 'miss'
            });
            const missCount = i === 5 ? 5 : (i === 4 ? 5 : Math.min(i + 1, 4));
            for (let m = 0; m < missCount; m += 1) {
                insertEvent(userA, {
                    id: 'tw-' + i + '-' + m,
                    word,
                    result: 'miss',
                    at: new Date(now - m * 1000).toISOString()
                });
            }
        }
        // outside window
        insertEvent(userA, {
            id: 'old-miss',
            word: 'trouble-0',
            result: 'miss',
            at: new Date(now - 40 * DAY_MS).toISOString()
        });

        const top = progress.troubleWords(userA, now, 10);
        assert.equal(top.length, 10);
        assert.equal(top[0].misses, 5);
        assert.equal(top[0].word, 'trouble-5');
        assert.equal(top[0].lapses, 9);
        assert.equal(top[1].misses, 5);
        assert.equal(top[1].word, 'trouble-4');
        for (let i = 1; i < top.length; i += 1) {
            const prev = top[i - 1];
            const cur = top[i];
            assert.ok(
                prev.misses > cur.misses
                || (prev.misses === cur.misses && prev.lapses >= cur.lapses)
            );
        }
    });

    it('ac-5b: answerHistory user-isolated, newest-first, cursor paging, limit clamp', () => {
        for (let i = 0; i < 5; i += 1) {
            insertEvent(userA, {
                id: 'hist-a-' + i,
                word: 'קָמָה',
                result: i % 2 === 0 ? 'pass' : 'miss',
                at: new Date(now - i * 60 * 1000).toISOString(),
                slug: 'my-morning'
            });
        }
        insertEvent(userB, {
            id: 'hist-b-0',
            word: 'קָמָה',
            result: 'pass',
            at: new Date(now).toISOString()
        });

        const page1 = progress.answerHistory(userA, { limit: 2 });
        assert.equal(page1.events.length, 2);
        assert.equal(page1.events[0].id, 'hist-a-0');
        assert.equal(page1.events[1].id, 'hist-a-1');
        assert.ok(page1.nextBefore);
        assert.ok(!page1.events.some((e) => e.id.startsWith('hist-b')));

        const page2 = progress.answerHistory(userA, {
            before: page1.nextBefore,
            limit: 2
        });
        assert.equal(page2.events.length, 2);
        assert.equal(page2.events[0].id, 'hist-a-2');
        assert.equal(page2.events[1].id, 'hist-a-3');

        const page3 = progress.answerHistory(userA, {
            before: page2.nextBefore,
            limit: 2
        });
        assert.equal(page3.events.length, 1);
        assert.equal(page3.events[0].id, 'hist-a-4');

        const allIds = []
            .concat(page1.events, page2.events, page3.events)
            .map((e) => e.id);
        assert.deepEqual(allIds, [
            'hist-a-0', 'hist-a-1', 'hist-a-2', 'hist-a-3', 'hist-a-4'
        ]);
        assert.equal(new Set(allIds).size, 5);

        const clamped = progress.answerHistory(userA, { limit: 500 });
        assert.equal(clamped.events.length, 5);

        assert.ok(typeof page1.events[0].meaning === 'string');
        assert.ok(typeof page1.events[0].lessonTitle === 'string');
        assert.ok(page1.events[0].lessonTitle.length > 0);
    });

    it('ac-3 metrics isolation: user B metrics ignore user A events', () => {
        insertEvent(userA, {
            id: 'iso-a', word: 'קָמָה', result: 'pass',
            at: new Date(now).toISOString()
        });
        upsertItem(userA, 'קָמָה', 'he_en', { step: 1, last_result: 'pass' });
        const mB = progress.metrics(userB, now);
        assert.deepEqual(mB.stages.he_en, { new: 0, learning: 0, longTerm: 0 });
        assert.equal(mB.accuracy.he_en.pass, 0);
        assert.equal(progress.troubleWords(userB, now).length, 0);
        assert.equal(progress.answerHistory(userB, { limit: 50 }).events.length, 0);
    });
});
