/**
 * Server-side progress: signal apply, review state, lesson blobs, metrics.
 * Provenance: INIT-002/SPEC-005
 * Grades, form signals, and the SRS_SCHEDULER switch: INIT-006/SPEC-004
 */
'use strict';

const path = require('path');
const db = require('./db');
const reviewRepo = require('./db/repos/review');
const eventsRepo = require('./db/repos/events');
const lessonProgressRepo = require('./db/repos/lesson-progress');
const { lessonsDir, catalogFile } = require('./paths');
const { loadDataWindow } = require('./lesson-data');
const HebrewReview = require('../shared/player/review.js');
const HebrewFsrs = require('../shared/player/fsrs.js');
const learnerModel = require('./learner/model');

const VALID_DIRECTIONS = new Set(['he_en', 'en_he']);
const VALID_RESULTS = new Set(['pass', 'miss', 'not_yet']);
const VALID_KINDS = new Set(['word', 'form']);
/** Grade for a signal that carries none (ADR-0009 §2). not_yet has no grade. */
const GRADE_FOR_RESULT = Object.freeze({ pass: 3, miss: 1 });
/** Stored for a form signal that names no direction: the learner produces the Hebrew form. */
const FORM_DEFAULT_DIRECTION = 'en_he';
/** Under fsrs, a passed direction is long-term from this stability in days (ADR-0009 §2). */
const LONG_TERM_STABILITY_DAYS = 14;
const MAX_STATE_BYTES = 65536;
const DAY_MS = 24 * 60 * 60 * 1000;
const DEFAULT_HISTORY_LIMIT = 50;
const MAX_HISTORY_LIMIT = 200;

/** Top-level keys allowed in hebrew-worksheet:<slug> blobs (shared/player/core.js). */
const LESSON_STATE_KEYS = new Set([
    'lesson',
    'round',
    'completedRounds',
    'roundStartedAt',
    'wordMarks',
    'missedLast',
    'vowelsOn',
    'stepIndex',
    'completedSteps',
    'activityState'
]);

/**
 * 'ladder' or 'fsrs' from SRS_SCHEDULER. An empty value is ladder, the default
 * until INIT-006 SPEC-020. Any other value is ladder too, with a warning.
 * @param {string|undefined} raw
 * @returns {'ladder'|'fsrs'}
 */
function resolveScheduler(raw) {
    if (raw == null || String(raw).trim() === '') return 'ladder';
    const value = String(raw).trim().toLowerCase();
    if (value === 'fsrs' || value === 'ladder') return value;
    console.warn('[progress] SRS_SCHEDULER ' + JSON.stringify(String(raw))
        + ' is not fsrs or ladder; using ladder');
    return 'ladder';
}

/** Read once, when the server process loads this module. */
const SCHEDULER = resolveScheduler(process.env.SRS_SCHEDULER);

function toIso(now) {
    if (now instanceof Date) return now.toISOString();
    if (typeof now === 'number' && Number.isFinite(now)) {
        return new Date(now).toISOString();
    }
    if (typeof now === 'string' && now) {
        const parsed = Date.parse(now);
        if (Number.isFinite(parsed)) return new Date(parsed).toISOString();
    }
    return new Date().toISOString();
}

function toTime(now) {
    if (now instanceof Date) return now.getTime();
    if (typeof now === 'number' && Number.isFinite(now)) return now;
    if (typeof now === 'string' && now) {
        const parsed = Date.parse(now);
        if (Number.isFinite(parsed)) return parsed;
    }
    return Date.now();
}

function parseSources(raw) {
    if (raw == null || raw === '') return [];
    if (Array.isArray(raw)) return raw.map(String);
    try {
        const parsed = JSON.parse(String(raw));
        return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch (_) {
        return [];
    }
}

function rowToDirection(row) {
    if (!row) return null;
    return {
        due: row.due,
        step: row.step == null ? 0 : Number(row.step),
        streak: row.streak == null ? 0 : Number(row.streak),
        lapses: row.lapses == null ? 0 : Number(row.lapses),
        lastResult: row.last_result == null ? null : String(row.last_result),
        lastAt: row.last_at == null ? null : String(row.last_at)
    };
}

function directionToRow(userId, word, direction, dir, sources) {
    return {
        user_id: userId,
        word,
        direction,
        due: dir.due,
        step: dir.step,
        streak: dir.streak,
        lapses: dir.lapses,
        last_result: dir.lastResult,
        last_at: dir.lastAt,
        sources: JSON.stringify(sources)
    };
}

/** A grade must agree with its result: 1 is a miss, 2 to 4 are a pass, not_yet has none. */
function gradeFits(grade, result) {
    if (result === 'miss') return grade === 1;
    if (result === 'pass') return grade >= 2 && grade <= 4;
    return false;
}

/**
 * Validate one signal and normalize its fields, or return null to reject it.
 * `grade` is optional; when present it must be an integer 1–4 that fits the result.
 * `kind` is 'word' (default) or 'form'. A form signal must name its lemma.
 * `kind` and `lemma`, when present, must be strings.
 * @param {object} signal
 * @returns {object|null}
 */
function parseSignal(signal) {
    if (!signal || typeof signal !== 'object') return null;
    if (signal.id == null || String(signal.id) === '') return null;
    const word = HebrewReview.keyFor(signal.word);
    if (!word) return null;
    const result = String(signal.result);
    if (!VALID_RESULTS.has(result)) return null;
    if (signal.kind != null && typeof signal.kind !== 'string') return null;
    const kind = signal.kind == null ? 'word' : signal.kind;
    if (!VALID_KINDS.has(kind)) return null;
    const direction = signal.direction == null && kind === 'form'
        ? FORM_DEFAULT_DIRECTION
        : String(signal.direction);
    if (!VALID_DIRECTIONS.has(direction)) return null;
    if (signal.lemma != null && typeof signal.lemma !== 'string') return null;
    const lemma = signal.lemma == null ? '' : HebrewReview.keyFor(signal.lemma);
    if (kind === 'form' && !lemma) return null;
    let grade = null;
    if (signal.grade != null) {
        if (!Number.isInteger(signal.grade) || !gradeFits(signal.grade, result)) return null;
        grade = signal.grade;
    }
    return {
        id: String(signal.id),
        word,
        direction,
        result,
        kind,
        lemma: lemma || null,
        grade,
        slug: signal.slug != null && String(signal.slug) !== '' ? String(signal.slug) : null,
        activity: signal.activity == null ? null : String(signal.activity),
        at: signal.at != null ? String(signal.at) : null
    };
}

/**
 * ADR-0009 §2 migration values for a passed row. Used for a row the backfill never
 * saw (the ladder wrote it after migration 004) when it has no stability yet.
 * @param {object} row review_items row
 */
function ladderBackfill(row) {
    const ladder = HebrewReview.LADDER_DAYS;
    const step = Number(row.step);
    let stability = ladder[0];
    if (Number.isInteger(step) && step >= 0) stability = ladder[Math.min(step, ladder.length - 1)];
    const lapses = Number(row.lapses) || 0;
    return {
        stability,
        difficulty: 5 + 0.5 * Math.min(lapses, 5),
        reps: (Number(row.streak) || 0) + lapses,
        state: 'review'
    };
}

/** Same "ever passed" test as metrics(), for one row. */
function rowEverPassed(userId, row) {
    if (row.last_result === 'pass' || Number(row.step) >= 1 || Number(row.last_grade) >= 2) {
        return true;
    }
    return Boolean(db.get(
        `SELECT 1 AS ok FROM practice_events
         WHERE user_id = ? AND word = ? AND direction = ? AND result = 'pass' AND kind = 'word'
         LIMIT 1`,
        Number(userId),
        row.word,
        row.direction
    ));
}

/**
 * FSRS card for a review row (shared/player/fsrs.js shape). No row is a new card.
 * @param {number} userId
 * @param {object|null|undefined} row review_items row
 * @param {number|Date|string} now
 */
function cardForRow(userId, row, now) {
    if (!row) return HebrewFsrs.initCard(now);
    const card = {
        stability: row.stability == null ? null : Number(row.stability),
        difficulty: row.difficulty == null ? null : Number(row.difficulty),
        reps: Number(row.reps) || 0,
        state: row.fsrs_state == null ? null : String(row.fsrs_state),
        due: String(row.due),
        lastGrade: row.last_grade == null ? null : Number(row.last_grade),
        lapses: Number(row.lapses) || 0,
        lastAt: row.last_at == null ? null : String(row.last_at)
    };
    if (card.stability == null && rowEverPassed(userId, row)) {
        Object.assign(card, ladderBackfill(row));
    }
    return card;
}

/**
 * Review row for a word signal under fsrs. step and streak still follow the
 * ladder, so SRS_SCHEDULER=ladder picks up where the ladder would be.
 * not_yet changes no schedule: due, last_at, and the FSRS fields stay.
 */
function fsrsRow(userId, signal, grade, existing, ladderNext, sources, serverNow) {
    const row = directionToRow(userId, signal.word, signal.direction, ladderNext, sources);
    const card = cardForRow(userId, existing, serverNow);
    if (grade == null) {
        row.due = card.due;
        row.lapses = card.lapses;
        row.last_at = card.lastAt;
        if (!existing) row.fsrs_state = card.state;
        return row;
    }
    const next = HebrewFsrs.nextCard(card, grade, serverNow, { fromLesson: signal.grade == null });
    row.due = next.due;
    row.lapses = next.lapses;
    row.last_at = next.lastAt;
    row.stability = next.stability;
    row.difficulty = next.difficulty;
    row.reps = next.reps;
    row.fsrs_state = next.state;
    row.last_grade = next.lastGrade;
    return row;
}

/**
 * Schedule one word signal. A suspended row is not scheduled. Under ladder the
 * schedule is HebrewReview.nextState and the grade is still written.
 */
function applyWordSignal(userId, signal, grade, serverNow) {
    const existing = reviewRepo.find(userId, signal.word, signal.direction);
    if (existing && Number(existing.suspended) === 1) return;
    const sources = parseSources(existing && existing.sources);
    if (signal.slug && sources.indexOf(signal.slug) === -1) sources.push(signal.slug);
    const ladderNext = HebrewReview.nextState(rowToDirection(existing), signal.result, serverNow);
    let row;
    if (SCHEDULER === 'fsrs') {
        row = fsrsRow(userId, signal, grade, existing, ladderNext, sources, serverNow);
    } else {
        row = directionToRow(userId, signal.word, signal.direction, ladderNext, sources);
        if (grade != null) row.last_grade = grade;
    }
    reviewRepo.upsert(row);
}

/**
 * Apply a batch of practice signals for one user (one transaction).
 * Idempotent on (user_id, id). Uses server `now` for scheduling; client `at` is logged only.
 * A signal without a grade maps pass→3, miss→1, not_yet→none (ADR-0009 §2).
 * A `kind: 'form'` signal is logged with its lemma and touches no review row.
 * A form pass or miss updates concept mastery inside this transaction
 * (INIT-006/SPEC-006). A word-card grade and not_yet do not.
 * @param {number} userId
 * @param {Array<object>} signals
 * @param {number|Date|string} [now]
 * @returns {{ applied: number, rejected: number, state: object }}
 */
function applySignals(userId, signals, now) {
    const uid = Number(userId);
    const batch = Array.isArray(signals) ? signals : [];
    const serverNow = now != null ? now : Date.now();
    let applied = 0;
    let rejected = 0;

    db.transaction(() => {
        for (const raw of batch) {
            const signal = parseSignal(raw);
            if (!signal) {
                rejected += 1;
                continue;
            }
            if (eventsRepo.find(uid, signal.id)) {
                continue;
            }
            const grade = signal.grade != null
                ? signal.grade
                : (GRADE_FOR_RESULT[signal.result] || null);

            if (signal.kind === 'word') {
                applyWordSignal(uid, signal, grade, serverNow);
            }
            eventsRepo.insert({
                id: signal.id,
                user_id: uid,
                word: signal.word,
                direction: signal.direction,
                result: signal.result,
                at: signal.at != null ? signal.at : toIso(serverNow),
                slug: signal.slug,
                activity: signal.activity,
                grade,
                kind: signal.kind,
                lemma: signal.lemma
            });
            if (signal.kind === 'form' && (signal.result === 'pass' || signal.result === 'miss')) {
                learnerModel.recordConceptEvent(uid, {
                    word: signal.word,
                    kind: signal.kind,
                    lemma: signal.lemma,
                    result: signal.result
                }, serverNow);
            }
            applied += 1;
        }
    });

    return {
        applied,
        rejected,
        state: reviewState(uid)
    };
}

/**
 * ADR-0001 store shape for the browser dueList / page code.
 * @param {number} userId
 * @returns {{ v: 1, words: object, migratedLessons: object }}
 */
function reviewState(userId) {
    const rows = reviewRepo.listByUser(Number(userId));
    const words = Object.create(null);
    for (const row of rows) {
        const key = String(row.word);
        if (!words[key]) {
            words[key] = {
                he_en: null,
                en_he: null,
                sources: []
            };
        }
        const entry = words[key];
        entry[row.direction] = rowToDirection(row);
        const src = parseSources(row.sources);
        for (const s of src) {
            if (entry.sources.indexOf(s) === -1) entry.sources.push(s);
        }
    }
    function freshDir(due) {
        return {
            due: due || '1970-01-01T00:00:00.000Z',
            step: 0,
            streak: 0,
            lapses: 0,
            lastResult: null,
            lastAt: null
        };
    }
    for (const key of Object.keys(words)) {
        const entry = words[key];
        const anchor = (entry.he_en && entry.he_en.due)
            || (entry.en_he && entry.en_he.due)
            || null;
        if (!entry.he_en) entry.he_en = freshDir(anchor);
        if (!entry.en_he) entry.en_he = freshDir(anchor);
    }
    return { v: 1, words, migratedLessons: {} };
}

function emptyStages() {
    return { new: 0, learning: 0, longTerm: 0 };
}

function emptyAccuracy() {
    return { pass: 0, miss: 0, pct: null };
}

/**
 * Stage of one direction record. new = never passed.
 * ladder (ADR-0002 §10 + aud-2): learning = step 0–2; long-term = step ≥ 3.
 * fsrs (ADR-0009 §2): learning = stability under 14 days; long-term = 14 or more.
 * @param {{ step?: number, stability?: number|null }} dir
 * @param {boolean} everPassed
 */
function stageFor(dir, everPassed) {
    if (!everPassed) return 'new';
    if (SCHEDULER === 'fsrs') {
        const stability = dir && Number.isFinite(dir.stability) ? dir.stability : 0;
        return stability >= LONG_TERM_STABILITY_DAYS ? 'longTerm' : 'learning';
    }
    const step = dir && Number.isInteger(dir.step) ? dir.step : 0;
    if (step >= 3) return 'longTerm';
    return 'learning';
}

/**
 * @param {number} userId
 * @param {number|Date|string} [now]
 * @returns {{ stages: object, accuracy: object }}
 */
function metrics(userId, now) {
    const uid = Number(userId);
    const t = toTime(now != null ? now : Date.now());
    const since = new Date(t - 30 * DAY_MS).toISOString();

    const passedRows = db.all(
        `SELECT DISTINCT word, direction FROM practice_events
         WHERE user_id = ? AND result = 'pass' AND kind = 'word'`,
        uid
    );
    const passed = Object.create(null);
    for (const row of passedRows) {
        passed[row.word + '\0' + row.direction] = true;
    }

    const stages = {
        he_en: emptyStages(),
        en_he: emptyStages()
    };
    const items = reviewRepo.listByUser(uid);
    for (const row of items) {
        const dir = row.direction;
        if (!stages[dir]) continue;
        const ever = Boolean(
            passed[row.word + '\0' + dir]
            || row.last_result === 'pass'
            || (Number(row.step) >= 1)
            || (Number(row.last_grade) >= 2)
        );
        const record = rowToDirection(row);
        if (row.stability != null) record.stability = Number(row.stability);
        else if (ever) record.stability = ladderBackfill(row).stability;
        const bucket = stageFor(record, ever);
        stages[dir][bucket] += 1;
    }

    const accuracy = {
        he_en: emptyAccuracy(),
        en_he: emptyAccuracy()
    };
    const counts = db.all(
        `SELECT direction, result, COUNT(*) AS n
         FROM practice_events
         WHERE user_id = ? AND at >= ? AND result IN ('pass', 'miss') AND kind = 'word'
         GROUP BY direction, result`,
        uid,
        since
    );
    for (const row of counts) {
        const acc = accuracy[row.direction];
        if (!acc) continue;
        const n = Number(row.n) || 0;
        if (row.result === 'pass') acc.pass = n;
        else if (row.result === 'miss') acc.miss = n;
    }
    for (const dir of ['he_en', 'en_he']) {
        const acc = accuracy[dir];
        const attempts = acc.pass + acc.miss;
        acc.pct = attempts < 10 ? null : acc.pass / attempts;
    }

    return { stages, accuracy };
}

/**
 * Top trouble words by misses in the last 30 days; tie-break by lapses.
 * @param {number} userId
 * @param {number|Date|string} [now]
 * @param {number} [limit]
 * @returns {Array<{word:string,direction:string,misses:number,lapses:number}>}
 */
function troubleWords(userId, now, limit) {
    const uid = Number(userId);
    const t = toTime(now != null ? now : Date.now());
    const since = new Date(t - 30 * DAY_MS).toISOString();
    const cap = limit == null ? 10 : Math.max(1, Math.min(Number(limit) || 10, 10));

    return db.all(
        `SELECT e.word AS word,
                e.direction AS direction,
                COUNT(*) AS misses,
                COALESCE(r.lapses, 0) AS lapses
         FROM practice_events e
         LEFT JOIN review_items r
           ON r.user_id = e.user_id
          AND r.word = e.word
          AND r.direction = e.direction
         WHERE e.user_id = ?
           AND e.result = 'miss'
           AND e.kind = 'word'
           AND e.at >= ?
         GROUP BY e.word, e.direction
         ORDER BY misses DESC, lapses DESC, e.word ASC, e.direction ASC
         LIMIT ?`,
        uid,
        since,
        cap
    ).map((row) => ({
        word: String(row.word),
        direction: String(row.direction),
        misses: Number(row.misses) || 0,
        lapses: Number(row.lapses) || 0
    }));
}

function encodeCursor(at, id) {
    return Buffer.from(JSON.stringify({ at: String(at), id: String(id) }), 'utf8')
        .toString('base64url');
}

function decodeCursor(before) {
    if (before == null || before === '') return null;
    try {
        const raw = Buffer.from(String(before), 'base64url').toString('utf8');
        const parsed = JSON.parse(raw);
        if (!parsed || parsed.at == null || parsed.id == null) return null;
        return { at: String(parsed.at), id: String(parsed.id) };
    } catch (_) {
        return null;
    }
}

let meaningCache = null;
let titleCache = null;

function loadMeaningMap() {
    if (meaningCache) return meaningCache;
    const map = Object.create(null);
    try {
        const indexPath = path.join(lessonsDir(), '_review', 'vocab-index.js');
        const win = loadDataWindow(indexPath);
        const list = win.HEBREW_VOCAB_INDEX;
        if (Array.isArray(list)) {
            for (const item of list) {
                if (!item || item.key == null) continue;
                map[String(item.key)] = item.meaning != null ? String(item.meaning) : '';
            }
        }
    } catch (_) {
        /* fail soft — empty meanings */
    }
    meaningCache = map;
    return map;
}

function loadTitleMap() {
    if (titleCache) return titleCache;
    const map = Object.create(null);
    try {
        const win = loadDataWindow(catalogFile());
        const list = win.HEBREW_CATALOG;
        if (Array.isArray(list)) {
            for (const item of list) {
                if (!item || item.slug == null) continue;
                map[String(item.slug)] = item.title != null ? String(item.title) : '';
            }
        }
    } catch (_) {
        /* fail soft */
    }
    titleCache = map;
    return map;
}

/** Test helper: clear lesson lookup caches. */
function _clearLookups() {
    meaningCache = null;
    titleCache = null;
}

/**
 * Paged practice events newest-first (opaque `before` cursor).
 * @param {number} userId
 * @param {{ before?: string, limit?: number }} [opts]
 * @returns {{ events: Array<object>, nextBefore: string|null }}
 */
function answerHistory(userId, opts) {
    const uid = Number(userId);
    const options = opts || {};
    let limit = options.limit == null ? DEFAULT_HISTORY_LIMIT : Number(options.limit);
    if (!Number.isFinite(limit) || limit < 1) limit = DEFAULT_HISTORY_LIMIT;
    if (limit > MAX_HISTORY_LIMIT) limit = MAX_HISTORY_LIMIT;

    const cursor = decodeCursor(options.before);
    let rows;
    if (cursor) {
        rows = db.all(
            `SELECT * FROM practice_events
             WHERE user_id = ?
               AND (at < ? OR (at = ? AND id < ?))
             ORDER BY at DESC, id DESC
             LIMIT ?`,
            uid,
            cursor.at,
            cursor.at,
            cursor.id,
            limit
        );
    } else {
        rows = db.all(
            `SELECT * FROM practice_events
             WHERE user_id = ?
             ORDER BY at DESC, id DESC
             LIMIT ?`,
            uid,
            limit
        );
    }

    const meanings = loadMeaningMap();
    const titles = loadTitleMap();
    const events = rows.map((row) => ({
        id: String(row.id),
        word: String(row.word),
        direction: String(row.direction),
        result: String(row.result),
        at: String(row.at),
        slug: row.slug == null ? null : String(row.slug),
        activity: row.activity == null ? null : String(row.activity),
        meaning: meanings[row.word] != null ? meanings[row.word] : '',
        lessonTitle: row.slug && titles[row.slug] != null ? titles[row.slug] : ''
    }));

    const nextBefore = events.length
        ? encodeCursor(events[events.length - 1].at, events[events.length - 1].id)
        : null;

    return { events, nextBefore };
}

/**
 * @param {number} userId
 * @param {string} slug
 * @returns {{ state: object, rev: number }|null}
 */
function getLessonProgress(userId, slug) {
    const row = lessonProgressRepo.find(Number(userId), String(slug));
    if (!row) return null;
    let state;
    try {
        state = JSON.parse(row.state_json);
    } catch (_) {
        state = {};
    }
    return { state, rev: Number(row.rev) || 0 };
}

function validateLessonState(state) {
    if (!state || typeof state !== 'object' || Array.isArray(state)) {
        return { ok: false, error: 'state must be an object' };
    }
    for (const key of Object.keys(state)) {
        if (!LESSON_STATE_KEYS.has(key)) {
            return { ok: false, error: 'unknown key: ' + key };
        }
    }
    const json = JSON.stringify(state);
    const bytes = Buffer.byteLength(json, 'utf8');
    if (bytes > MAX_STATE_BYTES) {
        return { ok: false, error: 'state exceeds 64 KB' };
    }
    return { ok: true, json, bytes };
}

/**
 * Optimistic-rev put of a lesson progress blob.
 * @param {number} userId
 * @param {string} slug
 * @param {object} state
 * @param {number} rev expected current revision (0 if none)
 * @param {number|Date|string} [now]
 * @returns {{ ok: true, state: object, rev: number }
 *   |{ ok: false, conflict: true, state: object, rev: number }
 *   |{ ok: false, error: string }}
 */
function putLessonProgress(userId, slug, state, rev, now) {
    const uid = Number(userId);
    const lessonSlug = String(slug);
    const expected = rev == null ? 0 : Number(rev);
    const check = validateLessonState(state);
    if (!check.ok) return { ok: false, error: check.error };

    const existing = lessonProgressRepo.find(uid, lessonSlug);
    if (existing) {
        const currentRev = Number(existing.rev) || 0;
        if (currentRev !== expected) {
            let stored;
            try {
                stored = JSON.parse(existing.state_json);
            } catch (_) {
                stored = {};
            }
            return { ok: false, conflict: true, state: stored, rev: currentRev };
        }
    } else if (expected !== 0) {
        return { ok: false, conflict: true, state: null, rev: 0 };
    }

    const nextRev = existing ? (Number(existing.rev) || 0) + 1 : 1;
    const updatedAt = toIso(now != null ? now : Date.now());
    lessonProgressRepo.upsert({
        user_id: uid,
        slug: lessonSlug,
        state_json: check.json,
        rev: nextRev,
        updated_at: updatedAt
    });
    return { ok: true, state, rev: nextRev };
}

module.exports = {
    SCHEDULER,
    applySignals,
    cardForRow,
    reviewState,
    getLessonProgress,
    putLessonProgress,
    metrics,
    troubleWords,
    answerHistory,
    LESSON_STATE_KEYS,
    MAX_STATE_BYTES,
    _clearLookups,
    encodeCursor,
    decodeCursor
};
