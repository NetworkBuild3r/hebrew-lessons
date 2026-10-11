/**
 * Product thresholds for the learner model.
 * ADR-0009 §2 ladder strength approximation, §3 weak flags and strength
 * buckets, §4 concept mastery, §5 journey stages, and §1 share rounding.
 * server/learner/model.js reads these constants and does not copy them.
 * Provenance: INIT-006/SPEC-006
 */
'use strict';

/** Milliseconds in one civil day. Used only to number local calendar days. */
const MS_PER_DAY = 24 * 60 * 60 * 1000;

/**
 * Pass is 1 and miss is 0, the scores the mastery formula consumes.
 * minPassGrade is the lowest flashcard grade that counts as a pass (ADR-0009 §2).
 */
const SCORE = Object.freeze({
    pass: 1,
    miss: 0,
    minPassGrade: 2
});

/**
 * Retrievability bucket edges. A value below `fragileBelow` is fragile,
 * below `shakyBelow` is shaky, below `steadyBelow` is steady, and the rest
 * is solid (ADR-0009 §3).
 */
const STRENGTH = Object.freeze({
    fragileBelow: 0.5,
    shakyBelow: 0.8,
    steadyBelow: 0.95
});

/**
 * Weak-flag thresholds, first matching row wins (ADR-0009 §3 W1–W5).
 * againGrade is FSRS Again.
 */
const WEAK = Object.freeze({
    longTermLapses: 4,
    veryWeakLapses: 3,
    weakLapses: 2,
    againWithLapses: 1,
    againGrade: 1,
    fsrsDifficulty: 8,
    fsrsReps: 3
});

/** Concept mastery (ADR-0009 §4). Judged statuses start at judgedMinAttempts. */
const MASTERY = Object.freeze({
    ewmaAlpha: 0.2,
    emergingMinAttempts: 1,
    emergingMaxAttempts: 4,
    judgedMinAttempts: 5,
    veryWeakBelow: 0.5,
    weakBelow: 0.7,
    okBelow: 0.9
});

/**
 * Journey stage from the count of he_en long-term words (ADR-0009 §5).
 * A count below foundationsAt is script. independentAt and above is independent.
 */
const JOURNEY = Object.freeze({
    foundationsAt: 50,
    buildingAt: 150,
    expandingAt: 500,
    independentAt: 1500
});

/** Ladder step that first counts as long-term (ADR-0002 §10). */
const LADDER_LONG_TERM_STEP = 3;

/** FSRS stability, in days, that first counts as long-term (ADR-0009 §2). */
const LONG_TERM_STABILITY_DAYS = 14;

/**
 * Stability in days for ladder steps 0 through 4.
 * Under SRS_SCHEDULER=ladder, strength uses these values, not a stored stability.
 */
const LADDER_STABILITY_DAYS = Object.freeze([1, 3, 7, 14, 30]);

/**
 * Ladder retrievability is base^(t/S), so it is `base` on the due day
 * (ADR-0009 §2). FSRS mode uses shared/player/fsrs.js instead.
 */
const LADDER_RETRIEVABILITY_BASE = 0.9;

/** Accuracy and vowels-off share round to this step (ADR-0009 §1). */
const ROUND_STEP = 0.05;

module.exports = {
    MS_PER_DAY,
    SCORE,
    STRENGTH,
    WEAK,
    MASTERY,
    JOURNEY,
    LADDER_LONG_TERM_STEP,
    LONG_TERM_STABILITY_DAYS,
    LADDER_STABILITY_DAYS,
    LADDER_RETRIEVABILITY_BASE,
    ROUND_STEP
};
