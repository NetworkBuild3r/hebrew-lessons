/**
 * FSRS-6 review scheduler (ADR-0009 §2). Loaded by the server with require() and
 * by the browser as a plain <script>.
 *
 * Before editing:
 * - Pure functions only. No Date.now(), no storage, no network, no DOM, and no
 *   other player module. Every function takes `now` from its caller.
 * - The server's `now` is authoritative, and the server is the only writer of a
 *   schedule. The browser may call previewIntervals() for button labels; it
 *   never stores what this file returns.
 * - PARAMS is pinned by ADR-0009 §2. Changing a number is an ADR amendment and a
 *   regenerated golden fixture (test/fixtures/fsrs/), not a local edit.
 * - A calendar day is a local day of the process that calls in, the same rule
 *   as review.js. On the server that is the server's time zone.
 *
 * Card shape, in and out:
 *   { stability, difficulty, reps, state, due, lastGrade, lapses, lastAt }
 * stability and difficulty are null until the first grade. state is 'new',
 * 'learning' (graded, no success yet), 'review', or 'relearning' (Again after a
 * success); it is informational and the math does not read it. lastAt is the
 * time of the last grade.
 *
 * Authored-by-spec: INIT-006/SPEC-004
 */
(function (root) {
    'use strict';

    var DAY_MS = 24 * 60 * 60 * 1000;

    /**
     * FSRS-6 default parameters w0..w20, desired retention, and interval bounds,
     * pinned by ADR-0009 §2. Version FSRS-6 as published in open-spaced-repetition,
     * "The Algorithm": https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm
     * (retrieved 2026-10-09). Never trained on a learner's history.
     */
    var PARAMS = Object.freeze({
        version: 'FSRS-6',
        source: 'https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm',
        w: Object.freeze([
            0.212, 1.2931, 2.3065, 8.2956, 6.4133, 0.8334, 3.0194, 0.001, 1.8722, 0.1666,
            0.796, 1.4835, 0.0614, 0.2629, 1.6483, 0.6014, 1.8729, 0.5425, 0.0912, 0.0658,
            0.1542
        ]),
        desiredRetention: 0.9,
        minIntervalDays: 1,
        maxIntervalDays: 365
    });

    /** Stability floor used by both reference implementations (py-fsrs, ts-fsrs). */
    var MIN_STABILITY = 0.001;
    var MIN_DIFFICULTY = 1;
    var MAX_DIFFICULTY = 10;

    var AGAIN = 1;
    var HARD = 2;
    var GOOD = 3;
    var EASY = 4;
    var GRADE_LIST = [AGAIN, HARD, GOOD, EASY];
    var STATES = ['new', 'learning', 'review', 'relearning'];

    var W = PARAMS.w;
    var DECAY = -W[20];
    var FACTOR = Math.pow(0.9, 1 / DECAY) - 1;

    function toTime(value) {
        if (value instanceof Date) return value.getTime();
        if (typeof value === 'number') return Number.isFinite(value) ? value : NaN;
        if (typeof value === 'string' && value) return Date.parse(value);
        return NaN;
    }

    function requireNow(now) {
        var t = toTime(now);
        if (!Number.isFinite(t)) {
            throw new TypeError('HebrewFsrs: now must be a Date, epoch milliseconds, or an ISO string');
        }
        return t;
    }

    function toIso(t) {
        return new Date(t).toISOString();
    }

    function isGrade(grade) {
        return grade === AGAIN || grade === HARD || grade === GOOD || grade === EASY;
    }

    function finiteOrNull(value) {
        if (value == null || value === '') return null;
        var n = Number(value);
        return Number.isFinite(n) ? n : null;
    }

    function countOf(value) {
        var n = Number(value);
        return Number.isInteger(n) && n > 0 ? n : 0;
    }

    /** Days since the epoch for the local calendar date of t. */
    function localDayNumber(t) {
        var d = new Date(t);
        return Math.round(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / DAY_MS);
    }

    /** Whole local calendar days from lastMs to nowMs; 0 on the same day, never negative. */
    function calendarDaysBetween(lastMs, nowMs) {
        return Math.max(0, localDayNumber(nowMs) - localDayNumber(lastMs));
    }

    function clampDifficulty(d) {
        return Math.min(Math.max(d, MIN_DIFFICULTY), MAX_DIFFICULTY);
    }

    function clampStability(s) {
        return Math.max(s, MIN_STABILITY);
    }

    function clampInterval(days) {
        return Math.min(Math.max(days, PARAMS.minIntervalDays), PARAMS.maxIntervalDays);
    }

    /** R(t, S) = (1 + FACTOR * t / S) ^ DECAY, so R(S, S) = 0.9. */
    function forgettingCurve(elapsedDays, stability) {
        return Math.pow(1 + FACTOR * elapsedDays / stability, DECAY);
    }

    function initialStability(grade) {
        return clampStability(W[grade - 1]);
    }

    function initialDifficulty(grade) {
        return W[4] - Math.exp(W[5] * (grade - 1)) + 1;
    }

    function nextDifficulty(difficulty, grade) {
        var delta = -W[6] * (grade - 3);
        var damped = difficulty + delta * (10 - difficulty) / 9;
        var reverted = W[7] * initialDifficulty(EASY) + (1 - W[7]) * damped;
        return clampDifficulty(reverted);
    }

    function recallStability(difficulty, stability, r, grade) {
        var hardPenalty = grade === HARD ? W[15] : 1;
        var easyBonus = grade === EASY ? W[16] : 1;
        return stability * (1
            + Math.exp(W[8])
            * (11 - difficulty)
            * Math.pow(stability, -W[9])
            * (Math.exp(W[10] * (1 - r)) - 1)
            * hardPenalty
            * easyBonus);
    }

    function forgetStability(difficulty, stability, r) {
        var longTerm = W[11]
            * Math.pow(difficulty, -W[12])
            * (Math.pow(stability + 1, W[13]) - 1)
            * Math.exp(W[14] * (1 - r));
        return Math.min(longTerm, stability / Math.exp(W[17] * W[18]));
    }

    /** Same-day review. FSRS-6 never lets a success lower stability here. */
    function shortTermStability(stability, grade) {
        var increase = Math.exp(W[17] * (grade - 3 + W[18])) * Math.pow(stability, -W[19]);
        if (grade >= HARD) increase = Math.max(increase, 1);
        return stability * increase;
    }

    /** I = (S / FACTOR) * (r ^ (1 / DECAY) - 1), rounded to whole days, 1 to 365. */
    function fsrsIntervalDays(stability) {
        return clampInterval(Math.round(
            (stability / FACTOR) * (Math.pow(PARAMS.desiredRetention, 1 / DECAY) - 1)
        ));
    }

    function readCard(card, t) {
        var c = card && typeof card === 'object' ? card : {};
        var stability = finiteOrNull(c.stability);
        var difficulty = finiteOrNull(c.difficulty);
        var hasMemory = stability != null && stability > 0 && difficulty != null;
        var lapses = countOf(c.lapses);
        var lastMs = toTime(c.lastAt);
        var dueValid = typeof c.due === 'string' && Number.isFinite(Date.parse(c.due));
        var state = STATES.indexOf(c.state) >= 0 ? c.state : null;
        if (!state) {
            if (hasMemory) state = 'review';
            else state = lapses > 0 ? 'learning' : 'new';
        }
        return {
            stability: hasMemory ? clampStability(stability) : null,
            difficulty: hasMemory ? clampDifficulty(difficulty) : null,
            reps: countOf(c.reps),
            state: state,
            due: dueValid ? c.due : toIso(t),
            lastGrade: isGrade(c.lastGrade) ? c.lastGrade : null,
            lapses: lapses,
            lastAt: Number.isFinite(lastMs) ? (typeof c.lastAt === 'string' ? c.lastAt : toIso(lastMs)) : null,
            lastMs: Number.isFinite(lastMs) ? lastMs : null
        };
    }

    /**
     * Stability, difficulty, and interval for each of the four grades.
     * Interval rules (ADR-0009 §2): Again is due the next day. Each other interval
     * is first bounded to 1–365 days; then Hard <= Good <= Easy with Good at least
     * Hard + 1 and Easy at least Good + 1; then the 365-day cap wins again.
     */
    function outcomes(c, t) {
        var out = {};
        var hasMemory = c.stability != null;
        var elapsed = c.lastMs == null ? null : calendarDaysBetween(c.lastMs, t);
        var sameDay = hasMemory && elapsed === 0;
        var r = hasMemory && elapsed != null ? forgettingCurve(elapsed, c.stability) : 1;
        GRADE_LIST.forEach(function (grade) {
            var stability;
            var difficulty;
            if (!hasMemory) {
                stability = initialStability(grade);
                difficulty = clampDifficulty(initialDifficulty(grade));
            } else if (sameDay) {
                stability = clampStability(shortTermStability(c.stability, grade));
                difficulty = nextDifficulty(c.difficulty, grade);
            } else {
                stability = clampStability(grade === AGAIN
                    ? forgetStability(c.difficulty, c.stability, r)
                    : recallStability(c.difficulty, c.stability, r, grade));
                difficulty = nextDifficulty(c.difficulty, grade);
            }
            out[grade] = { stability: stability, difficulty: difficulty };
        });
        var hard = Math.min(fsrsIntervalDays(out[HARD].stability), fsrsIntervalDays(out[GOOD].stability));
        var good = Math.max(fsrsIntervalDays(out[GOOD].stability), hard + 1);
        var easy = Math.max(fsrsIntervalDays(out[EASY].stability), good + 1);
        out[AGAIN].interval = PARAMS.minIntervalDays;
        out[HARD].interval = clampInterval(hard);
        out[GOOD].interval = clampInterval(good);
        out[EASY].interval = clampInterval(easy);
        return out;
    }

    function publicCard(c) {
        return {
            stability: c.stability,
            difficulty: c.difficulty,
            reps: c.reps,
            state: c.state,
            due: c.due,
            lastGrade: c.lastGrade,
            lapses: c.lapses,
            lastAt: c.lastAt
        };
    }

    /**
     * A card that has never been graded, due now.
     * @param {number|Date|string} now
     */
    function initCard(now) {
        var t = requireNow(now);
        return {
            stability: null,
            difficulty: null,
            reps: 0,
            state: 'new',
            due: toIso(t),
            lastGrade: null,
            lapses: 0,
            lastAt: null
        };
    }

    /**
     * Apply one grade. Same-day rule (ADR-0009 §2): a success sent by a lesson
     * (opts.fromLesson) on a card already graded today is recorded (lastGrade,
     * lastAt) and leaves the schedule alone. Any other grade on a card graded today
     * uses the FSRS same-day formula. Again always counts and adds a lapse.
     * @param {object|null} card
     * @param {1|2|3|4} grade
     * @param {number|Date|string} now
     * @param {{ fromLesson?: boolean }} [opts]
     * @returns {{stability:number|null,difficulty:number|null,reps:number,state:string,
     *   due:string,lastGrade:number|null,lapses:number,lastAt:string|null}}
     */
    function nextCard(card, grade, now, opts) {
        var t = requireNow(now);
        if (!isGrade(grade)) throw new RangeError('HebrewFsrs: grade must be the integer 1, 2, 3, or 4');
        var c = readCard(card, t);
        var gradedToday = c.lastMs != null && calendarDaysBetween(c.lastMs, t) === 0;
        if (opts && opts.fromLesson === true && grade !== AGAIN && gradedToday) {
            var recorded = publicCard(c);
            recorded.lastGrade = grade;
            recorded.lastAt = toIso(t);
            return recorded;
        }
        var chosen = outcomes(c, t)[grade];
        var hadSuccess = c.state === 'review' || c.state === 'relearning';
        var state;
        if (grade !== AGAIN) state = 'review';
        else state = hadSuccess ? 'relearning' : 'learning';
        return {
            stability: chosen.stability,
            difficulty: chosen.difficulty,
            reps: c.reps + 1,
            state: state,
            due: toIso(t + chosen.interval * DAY_MS),
            lastGrade: grade,
            lapses: c.lapses + (grade === AGAIN ? 1 : 0),
            lastAt: toIso(t)
        };
    }

    /**
     * Probability of recall now, R(t, S), with t in whole local calendar days since
     * the last grade. null when the card has no stability or was never graded.
     * @param {{stability?:number|null,lastAt?:string|null}} card
     * @param {number|Date|string} now
     * @returns {number|null}
     */
    function retrievability(card, now) {
        var t = requireNow(now);
        var stability = finiteOrNull(card && card.stability);
        var lastMs = toTime(card && card.lastAt);
        if (stability == null || stability <= 0 || !Number.isFinite(lastMs)) return null;
        return forgettingCurve(calendarDaysBetween(lastMs, t), clampStability(stability));
    }

    /**
     * Days until due for each grade, as flashcard button labels. The same numbers
     * nextCard() uses for an explicit grade.
     * @param {object|null} card
     * @param {number|Date|string} now
     * @returns {{1:number,2:number,3:number,4:number}}
     */
    function previewIntervals(card, now) {
        var t = requireNow(now);
        var out = outcomes(readCard(card, t), t);
        return { 1: out[AGAIN].interval, 2: out[HARD].interval, 3: out[GOOD].interval, 4: out[EASY].interval };
    }

    var Fsrs = {
        PARAMS: PARAMS,
        GRADES: Object.freeze({ AGAIN: AGAIN, HARD: HARD, GOOD: GOOD, EASY: EASY }),
        initCard: initCard,
        nextCard: nextCard,
        retrievability: retrievability,
        previewIntervals: previewIntervals
    };

    root.HebrewFsrs = Fsrs;
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = Fsrs;
    }
})(typeof window !== 'undefined' ? window : globalThis);
