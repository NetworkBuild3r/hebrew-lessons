/**
 * Pure scheduler tests for HebrewReview (ADR-0001 ladder / caps / interleave).
 * Authored-by-spec: INIT-001/SPEC-003
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const path = require('path');

const Review = require(path.resolve(__dirname, '..', 'shared', 'player', 'review.js'));

const DAY_MS = 24 * 60 * 60 * 1000;

function atLocal(y, m, d, h = 12) {
    return new Date(y, m - 1, d, h, 0, 0, 0).getTime();
}

describe('HebrewReview.keyFor', () => {
    it('NFC-normalizes and collapses whitespace', () => {
        assert.equal(Review.keyFor('  קָמָה  '), 'קָמָה');
        const decomposed = 'ק\u05B8מ\u05B8ה'.normalize('NFD');
        assert.equal(Review.keyFor(decomposed), decomposed.normalize('NFC'));
        assert.equal(Review.keyFor('א  ב'), 'א ב');
    });
});

describe('HebrewReview.nextState (ladder)', () => {
    const day0 = atLocal(2026, 10, 1, 10);

    it('pass advances one step and sets due from the new step interval', () => {
        const start = { due: new Date(day0).toISOString(), step: 0, streak: 0, lapses: 0, lastResult: null, lastAt: null };
        const next = Review.nextState(start, 'pass', day0);
        assert.equal(next.step, 1);
        assert.equal(next.streak, 1);
        assert.equal(next.lastResult, 'pass');
        assert.equal(Date.parse(next.due), day0 + 3 * DAY_MS);
    });

    it('pass at step 4 stays at step 4 with 30-day due', () => {
        const start = {
            due: new Date(day0).toISOString(),
            step: 4,
            streak: 5,
            lapses: 0,
            lastResult: 'pass',
            lastAt: new Date(day0 - 40 * DAY_MS).toISOString()
        };
        const next = Review.nextState(start, 'pass', day0);
        assert.equal(next.step, 4);
        assert.equal(Date.parse(next.due), day0 + 30 * DAY_MS);
        assert.equal(next.streak, 6);
    });

    it('same-day second pass does not advance step or due', () => {
        const first = Review.nextState(
            { due: new Date(day0).toISOString(), step: 0, streak: 0, lapses: 0, lastResult: null, lastAt: null },
            'pass',
            day0
        );
        const laterSameDay = atLocal(2026, 10, 1, 18);
        const second = Review.nextState(first, 'pass', laterSameDay);
        assert.equal(second.step, first.step);
        assert.equal(second.due, first.due);
        assert.equal(second.streak, first.streak);
        assert.equal(second.lastResult, 'pass');
        assert.equal(Date.parse(second.lastAt), laterSameDay);
    });

    it('miss resets to step 0 and due +1 day regardless of same-day', () => {
        const start = {
            due: new Date(day0).toISOString(),
            step: 3,
            streak: 4,
            lapses: 1,
            lastResult: 'pass',
            lastAt: new Date(day0 - 2 * 3600 * 1000).toISOString()
        };
        const next = Review.nextState(start, 'miss', day0);
        assert.equal(next.step, 0);
        assert.equal(next.streak, 0);
        assert.equal(next.lapses, 2);
        assert.equal(next.lastResult, 'miss');
        assert.equal(Date.parse(next.due), day0 + DAY_MS);
    });

    it('not_yet does not advance or lapse; sets due=now when never scheduled', () => {
        const next = Review.nextState(null, 'not_yet', day0);
        assert.equal(next.step, 0);
        assert.equal(next.lapses, 0);
        assert.equal(next.lastResult, 'not_yet');
        assert.equal(Date.parse(next.due), day0);
    });

    it('not_yet preserves a higher step', () => {
        const start = {
            due: new Date(day0 + 7 * DAY_MS).toISOString(),
            step: 2,
            streak: 2,
            lapses: 0,
            lastResult: 'pass',
            lastAt: new Date(day0 - DAY_MS).toISOString()
        };
        const next = Review.nextState(start, 'not_yet', day0);
        assert.equal(next.step, 2);
        assert.equal(next.due, start.due);
        assert.equal(next.lapses, 0);
        assert.equal(next.lastResult, 'not_yet');
    });
});

describe('HebrewReview.isDue', () => {
    it('returns true when due <= now', () => {
        const now = atLocal(2026, 10, 8);
        assert.equal(Review.isDue({ due: new Date(now - 1).toISOString() }, now), true);
        assert.equal(Review.isDue({ due: new Date(now).toISOString() }, now), true);
        assert.equal(Review.isDue({ due: new Date(now + 1).toISOString() }, now), false);
    });
});

describe('HebrewReview.dueList (caps + interleave)', () => {
    const now = atLocal(2026, 10, 8);

    function dir(dueOffsetDays, step, lastAt) {
        return {
            due: new Date(now + dueOffsetDays * DAY_MS).toISOString(),
            step: step == null ? 0 : step,
            streak: 0,
            lapses: 0,
            lastResult: lastAt == null ? null : 'pass',
            lastAt: lastAt == null ? null : new Date(now + lastAt * DAY_MS).toISOString()
        };
    }

    it('returns overdue items oldest-first', () => {
        const words = {
            a: { he_en: dir(-5, 1, -10), en_he: dir(1, 0, -1), sources: ['lesson-a'] },
            b: { he_en: dir(-1, 0, -2), en_he: dir(2, 0, -1), sources: ['lesson-b'] },
            c: { he_en: dir(-10, 2, -20), en_he: dir(3, 0, -1), sources: ['lesson-c'] }
        };
        const list = Review.dueList(words, now, { due: 30, new: 10 });
        assert.deepEqual(list.map(x => x.key), ['c', 'a', 'b']);
    });

    it('respects due and new caps', () => {
        const words = {};
        for (let i = 0; i < 40; i += 1) {
            words['old-' + i] = {
                he_en: dir(-(i + 1), 1, -50),
                en_he: dir(5, 0, -1),
                sources: ['old']
            };
        }
        for (let i = 0; i < 15; i += 1) {
            words['new-' + i] = {
                he_en: dir(0, 0, null),
                en_he: dir(0, 0, null),
                sources: ['new']
            };
        }
        const list = Review.dueList(words, now, { due: 30, new: 10 });
        assert.ok(list.length <= 30);
        const newKeys = new Set(list.filter(x => x.isNew).map(x => x.key));
        assert.ok(newKeys.size <= 10);
    });

    it('interleaves so at most 2 consecutive items from one lesson when others are due', () => {
        const words = {};
        for (let i = 0; i < 6; i += 1) {
            words['L1-' + i] = {
                he_en: dir(-(20 - i), 1, -30),
                en_he: dir(5, 0, -1),
                sources: ['lesson-1']
            };
        }
        for (let i = 0; i < 4; i += 1) {
            words['L2-' + i] = {
                he_en: dir(-(15 - i), 1, -30),
                en_he: dir(5, 0, -1),
                sources: ['lesson-2']
            };
        }
        const list = Review.dueList(words, now, { due: 30, new: 10 });
        for (let i = 2; i < list.length; i += 1) {
            const a = list[i - 2].lesson;
            const b = list[i - 1].lesson;
            const c = list[i].lesson;
            if (a && a === b && b === c) {
                assert.fail('three consecutive items from lesson ' + a + ' at index ' + i);
            }
        }
        assert.ok(list.some(x => x.lesson === 'lesson-1'));
        assert.ok(list.some(x => x.lesson === 'lesson-2'));
    });
});
