/**
 * FSRS-6 scheduler (shared/player/fsrs.js): golden values from an independent
 * reference implementation, the ADR-0009 §2 interval and same-day rules, and purity.
 * Authored-by-spec: INIT-006/SPEC-004
 */
'use strict';

// The fixture's times are UTC and the scheduler counts local calendar days, so this
// file pins the local zone to UTC. One test below switches zones on purpose.
process.env.TZ = 'UTC';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const Fsrs = require('../shared/player/fsrs.js');
const golden = require('./fixtures/fsrs/fsrs6-golden.json');

const DAY_MS = 24 * 60 * 60 * 1000;
const HOUR_MS = 60 * 60 * 1000;
const RELATIVE_TOLERANCE = 1e-9;
const W = Fsrs.PARAMS.w;

function assertClose(actual, expected, label) {
    assert.equal(typeof actual, 'number', label + ': not a number');
    const limit = RELATIVE_TOLERANCE * Math.max(1, Math.abs(expected));
    assert.ok(
        Math.abs(actual - expected) <= limit,
        label + ': expected ' + expected + ', got ' + actual
    );
}

function intervalDays(card, now) {
    return (Date.parse(card.due) - now) / DAY_MS;
}

function reviewCard(fields) {
    return Object.assign({
        stability: 10,
        difficulty: 5,
        reps: 3,
        state: 'review',
        due: '2026-02-10T12:00:00.000Z',
        lastGrade: 3,
        lapses: 0,
        lastAt: '2026-01-31T12:00:00.000Z'
    }, fields);
}

describe('golden fixture provenance', () => {
    it('ac-1: records the formula source and reference implementation, and matches the pinned parameters', () => {
        assert.equal(golden.formulaSource.url,
            'https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm');
        assert.equal(golden.formulaSource.retrieved, '2026-10-09');
        assert.equal(golden.implementation.name, 'py-fsrs (PyPI package fsrs)');
        assert.equal(golden.implementation.version, '6.3.2');
        assert.equal(golden.implementation.fsrsVersion, 'FSRS-6');
        assert.equal(golden.implementation.ranOutsideRepository, true);
        assert.equal(golden.crossCheck.name, 'ts-fsrs');
        for (const key of Object.keys(golden.crossCheck.maxRelativeDiff)) {
            assert.ok(golden.crossCheck.maxRelativeDiff[key] <= golden.crossCheck.tolerance, key);
        }
        assert.deepEqual(golden.parameters, Array.from(Fsrs.PARAMS.w));
        assert.equal(golden.desiredRetention, Fsrs.PARAMS.desiredRetention);
        assert.deepEqual(golden.intervalDays, {
            min: Fsrs.PARAMS.minIntervalDays,
            max: Fsrs.PARAMS.maxIntervalDays
        });
    });
});

describe('nextCard golden values (FSRS-6, py-fsrs 6.3.2)', () => {
    for (const testCase of golden.cases) {
        it('ac-1: ' + testCase.name + ' — ' + testCase.about, () => {
            let card = Fsrs.initCard(testCase.start);
            testCase.steps.forEach((step, index) => {
                const label = testCase.name + ' step ' + (index + 1);
                const at = Date.parse(step.at);
                const r = Fsrs.retrievability(card, at);
                if (step.retrievability == null) assert.equal(r, null, label + ' retrievability');
                else assertClose(r, step.retrievability, label + ' retrievability');
                assert.deepEqual(Fsrs.previewIntervals(card, at), step.previewIntervals,
                    label + ' preview intervals');

                card = Fsrs.nextCard(card, step.grade, at);

                assertClose(card.stability, step.stability, label + ' stability');
                assertClose(card.difficulty, step.difficulty, label + ' difficulty');
                assert.equal(intervalDays(card, at), step.interval, label + ' interval');
                assert.equal(card.lastGrade, step.grade, label + ' lastGrade');
                assert.equal(card.lastAt, new Date(at).toISOString(), label + ' lastAt');
            });
        });
    }

    it('ac-1: the fixture marks every step where the ADR interval differs from the reference', () => {
        let differing = 0;
        for (const testCase of golden.cases) {
            for (const step of testCase.steps) {
                if (step.interval === step.referenceInterval) continue;
                differing += 1;
                assert.match(step.note, /ADR-0009 §2/);
                if (step.grade === 1) assert.equal(step.interval, 1);
                else assert.equal(step.interval, step.previewIntervals[String(step.grade)]);
            }
        }
        assert.ok(differing >= 2, 'expected the Again and ordering rules to be exercised');
    });
});

describe('same-day rule (ADR-0009 §2)', () => {
    const morning = Date.parse('2026-02-10T08:00:00.000Z');
    const evening = Date.parse('2026-02-10T20:00:00.000Z');

    it('ac-3: a lesson Good on a card already graded today is recorded and leaves the schedule alone', () => {
        const first = Fsrs.nextCard(Fsrs.initCard(morning), 3, morning, { fromLesson: true });
        const second = Fsrs.nextCard(first, 3, evening, { fromLesson: true });
        assert.equal(second.stability, first.stability);
        assert.equal(second.difficulty, first.difficulty);
        assert.equal(second.due, first.due);
        assert.equal(second.reps, first.reps);
        assert.equal(second.state, first.state);
        assert.equal(second.lapses, first.lapses);
        assert.equal(second.lastGrade, 3);
        assert.equal(second.lastAt, new Date(evening).toISOString());
    });

    it('ac-3: the next calendar day advances, even when less than 24 hours have passed', () => {
        const lateNight = Date.parse('2026-02-10T23:30:00.000Z');
        const earlyNext = Date.parse('2026-02-11T00:30:00.000Z');
        const first = Fsrs.nextCard(reviewCard({ lastAt: '2026-02-01T12:00:00.000Z' }), 3, lateNight,
            { fromLesson: true });
        const next = Fsrs.nextCard(first, 3, earlyNext, { fromLesson: true });
        assert.notEqual(next.due, first.due);
        assert.equal(next.reps, first.reps + 1);
        assert.ok(next.stability >= first.stability);
    });

    it('ac-3: the calendar day is the local day of the caller', () => {
        // 14:30 and 15:30 UTC fall on 23:30 and 00:30 in Tokyo (UTC+9).
        const before = Date.parse('2026-02-10T14:30:00.000Z');
        const after = Date.parse('2026-02-10T15:30:00.000Z');
        const graded = Fsrs.nextCard(reviewCard({ lastAt: '2026-02-01T12:00:00.000Z' }), 3, before);
        try {
            process.env.TZ = 'Asia/Tokyo';
            const lesson = Fsrs.nextCard(graded, 3, after, { fromLesson: true });
            assert.equal(lesson.reps, graded.reps + 1, 'a new Tokyo day advances');
        } finally {
            process.env.TZ = 'UTC';
        }
        const sameUtcDay = Fsrs.nextCard(graded, 3, after, { fromLesson: true });
        assert.equal(sameUtcDay.reps, graded.reps, 'the same UTC day does not');
    });

    it('a flashcard grade later the same day uses the FSRS same-day formula', () => {
        const first = Fsrs.nextCard(Fsrs.initCard(morning), 3, morning);
        const s = first.stability;
        const expected = {
            1: s * Math.exp(W[17] * (1 - 3 + W[18])) * Math.pow(s, -W[19]),
            2: s * Math.max(Math.exp(W[17] * (2 - 3 + W[18])) * Math.pow(s, -W[19]), 1),
            3: s * Math.max(Math.exp(W[17] * (3 - 3 + W[18])) * Math.pow(s, -W[19]), 1),
            4: s * Math.max(Math.exp(W[17] * (4 - 3 + W[18])) * Math.pow(s, -W[19]), 1)
        };
        for (const grade of [1, 2, 3, 4]) {
            const next = Fsrs.nextCard(first, grade, evening);
            assertClose(next.stability, expected[grade], 'grade ' + grade);
            assert.equal(next.reps, first.reps + 1);
        }
        assert.ok(Fsrs.nextCard(first, 3, evening).stability >= s, 'Good never lowers stability');
    });

    it('a lesson miss later the same day still counts: Again, one lapse, due the next day', () => {
        const first = Fsrs.nextCard(Fsrs.initCard(morning), 3, morning, { fromLesson: true });
        const miss = Fsrs.nextCard(first, 1, evening, { fromLesson: true });
        assert.equal(miss.lapses, first.lapses + 1);
        assert.equal(miss.lastGrade, 1);
        assert.ok(miss.stability < first.stability);
        assert.equal(intervalDays(miss, evening), 1);
        const again = Fsrs.nextCard(miss, 1, evening + HOUR_MS, { fromLesson: true });
        assert.equal(again.lapses, first.lapses + 2);
    });
});

describe('interval rules (ADR-0009 §2)', () => {
    const now = Date.parse('2026-03-01T12:00:00.000Z');

    it('Again is due the next day even when FSRS stability after the lapse is several days', () => {
        const mature = reviewCard({ stability: 240, difficulty: 1, lastAt: new Date(now - 240 * DAY_MS).toISOString() });
        const lapsed = Fsrs.nextCard(mature, 1, now);
        assert.ok(lapsed.stability > 2, 'FSRS keeps several days of stability');
        assert.equal(intervalDays(lapsed, now), 1);
        assert.equal(Fsrs.previewIntervals(mature, now)[1], 1);
    });

    it('previews keep Hard ≤ Good ≤ Easy, one day apart below the cap, within 1–365 days', () => {
        let checked = 0;
        for (const stability of [0.05, 0.3, 0.9, 1.4, 2.6, 5, 9.5, 21, 60, 140, 300, 900]) {
            for (const difficulty of [1, 3.3, 5, 7.7, 10]) {
                for (const elapsed of [0, 1, 3, 10, 45, 200, 500]) {
                    const card = reviewCard({
                        stability,
                        difficulty,
                        lastAt: new Date(now - elapsed * DAY_MS - 2 * HOUR_MS).toISOString()
                    });
                    const p = Fsrs.previewIntervals(card, now);
                    assert.equal(p[1], 1);
                    for (const grade of [1, 2, 3, 4]) {
                        assert.ok(Number.isInteger(p[grade]) && p[grade] >= 1 && p[grade] <= 365);
                    }
                    assert.ok(p[2] <= p[3] && p[3] <= p[4], JSON.stringify(p));
                    if (p[3] < 365) assert.ok(p[3] >= p[2] + 1, JSON.stringify(p));
                    if (p[4] < 365) assert.ok(p[4] >= p[3] + 1, JSON.stringify(p));
                    checked += 1;
                }
            }
        }
        assert.equal(checked, 12 * 5 * 7);
    });

    it('previewIntervals equals the interval nextCard schedules for each explicit grade', () => {
        for (const stability of [0.2, 1.1, 4, 33, 400]) {
            for (const elapsed of [0, 2, 30]) {
                const card = reviewCard({ stability, lastAt: new Date(now - elapsed * DAY_MS).toISOString() });
                const p = Fsrs.previewIntervals(card, now);
                for (const grade of [1, 2, 3, 4]) {
                    assert.equal(intervalDays(Fsrs.nextCard(card, grade, now), now), p[grade]);
                }
            }
        }
        const fresh = Fsrs.initCard(now);
        assert.deepEqual(Fsrs.previewIntervals(fresh, now), { 1: 1, 2: 1, 3: 2, 4: 8 });
    });
});

describe('card bookkeeping', () => {
    const t0 = Date.parse('2026-04-01T12:00:00.000Z');

    it('initCard is new, due now, and has no memory state', () => {
        assert.deepEqual(Fsrs.initCard(t0), {
            stability: null,
            difficulty: null,
            reps: 0,
            state: 'new',
            due: new Date(t0).toISOString(),
            lastGrade: null,
            lapses: 0,
            lastAt: null
        });
    });

    it('state is learning until a success, review after one, relearning on Again after a success', () => {
        const states = [];
        let card = Fsrs.initCard(t0);
        const grades = [1, 1, 3, 1, 2, 4];
        grades.forEach((grade, i) => {
            card = Fsrs.nextCard(card, grade, t0 + (i + 1) * 2 * DAY_MS);
            states.push(card.state);
        });
        assert.deepEqual(states, ['learning', 'learning', 'review', 'relearning', 'review', 'review']);
        assert.equal(card.reps, grades.length);
        assert.equal(card.lapses, 3);
    });

    it('a recorded same-day lesson pass is not a rep and adds no lapse', () => {
        const graded = Fsrs.nextCard(Fsrs.initCard(t0), 3, t0, { fromLesson: true });
        const recorded = Fsrs.nextCard(graded, 3, t0 + HOUR_MS, { fromLesson: true });
        assert.equal(recorded.reps, 1);
        assert.equal(recorded.lapses, 0);
    });

    it('a never-graded card with an old lastAt starts from the initial values for its grade', () => {
        const card = { stability: null, difficulty: null, reps: 0, state: 'learning', lapses: 2,
            due: new Date(t0).toISOString(), lastGrade: null, lastAt: new Date(t0 - DAY_MS).toISOString() };
        const next = Fsrs.nextCard(card, 3, t0);
        assert.equal(next.stability, W[2]);
        assertClose(next.difficulty, W[4] - Math.exp(W[5] * 2) + 1, 'initial difficulty');
        assert.equal(next.lapses, 2);
        assert.equal(next.state, 'review');
    });
});

describe('retrievability', () => {
    const t0 = Date.parse('2026-05-01T12:00:00.000Z');

    it('is 1 on the day of the grade and 0.9 after exactly S days', () => {
        const card = reviewCard({ stability: 10, lastAt: new Date(t0).toISOString() });
        assert.equal(Fsrs.retrievability(card, t0 + 5 * HOUR_MS), 1);
        assertClose(Fsrs.retrievability(card, t0 + 10 * DAY_MS), 0.9, 'R(S, S)');
        assert.ok(Fsrs.retrievability(card, t0 + 30 * DAY_MS) < 0.9);
    });

    it('is null for a card that was never graded', () => {
        assert.equal(Fsrs.retrievability(Fsrs.initCard(t0), t0), null);
        assert.equal(Fsrs.retrievability({ stability: 4, lastAt: null }, t0), null);
    });
});

describe('inputs and purity', () => {
    const t0 = Date.parse('2026-06-01T12:00:00.000Z');

    it('rejects a grade outside 1–4 and a missing or invalid now', () => {
        for (const grade of [0, 5, 2.5, '3', null, undefined, NaN]) {
            assert.throws(() => Fsrs.nextCard(null, grade, t0), RangeError, String(grade));
        }
        assert.throws(() => Fsrs.nextCard(null, 3), TypeError);
        assert.throws(() => Fsrs.initCard(), TypeError);
        assert.throws(() => Fsrs.previewIntervals(null, 'not a date'), TypeError);
        assert.throws(() => Fsrs.retrievability(reviewCard({}), NaN), TypeError);
    });

    it('never reads the clock', () => {
        const realNow = Date.now;
        Date.now = () => { throw new Error('Date.now called'); };
        try {
            const card = Fsrs.nextCard(Fsrs.initCard(t0), 3, new Date(t0));
            Fsrs.nextCard(card, 4, '2026-06-03T12:00:00.000Z');
            Fsrs.previewIntervals(card, t0 + DAY_MS);
            Fsrs.retrievability(card, t0 + DAY_MS);
        } finally {
            Date.now = realNow;
        }
        const code = fs.readFileSync(path.join(__dirname, '..', 'shared', 'player', 'fsrs.js'), 'utf8')
            .replace(/\/\*[\s\S]*?\*\//g, '')
            .replace(/^\s*\/\/.*$/gm, '');
        assert.doesNotMatch(code, /Date\.now|new Date\(\)/);
        assert.doesNotMatch(code, /\bdocument\b|localStorage|sessionStorage|fetch\(|XMLHttpRequest|require\(/);
    });

    it('does not change the card it is given', () => {
        const card = reviewCard({});
        const before = JSON.stringify(card);
        Fsrs.nextCard(card, 1, t0);
        Fsrs.nextCard(card, 3, t0, { fromLesson: true });
        Fsrs.previewIntervals(card, t0);
        Fsrs.retrievability(card, t0);
        assert.equal(JSON.stringify(card), before);
    });

    it('PARAMS is one frozen constant with the 21 FSRS-6 weights and 0.90 retention', () => {
        assert.ok(Object.isFrozen(Fsrs.PARAMS));
        assert.ok(Object.isFrozen(Fsrs.PARAMS.w));
        assert.equal(Fsrs.PARAMS.version, 'FSRS-6');
        assert.match(Fsrs.PARAMS.source, /open-spaced-repetition\/awesome-fsrs\/wiki\/The-Algorithm/);
        assert.equal(Fsrs.PARAMS.w.length, 21);
        assert.equal(Fsrs.PARAMS.desiredRetention, 0.9);
        assert.throws(() => { Fsrs.PARAMS.w[0] = 1; }, TypeError);
    });

    it('loads as a browser script with no module system and sets window.HebrewFsrs', () => {
        const source = fs.readFileSync(path.join(__dirname, '..', 'shared', 'player', 'fsrs.js'), 'utf8');
        const sandbox = { window: {} };
        vm.createContext(sandbox);
        vm.runInContext(source, sandbox);
        const browser = sandbox.window.HebrewFsrs;
        assert.ok(browser);
        assert.deepEqual(
            JSON.parse(JSON.stringify(browser.previewIntervals(null, t0))),
            { 1: 1, 2: 1, 3: 2, 4: 8 }
        );
    });
});
