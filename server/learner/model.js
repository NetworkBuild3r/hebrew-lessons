/**
 * Learner model: word strength, weak flags, concept mastery, and journey.
 * Thresholds come from server/learner/thresholds.js only; do not inline numbers.
 *
 * Word strength uses shared/player/fsrs.js retrievability when SRS_SCHEDULER
 * is fsrs. Under ladder (the default until INIT-006/SPEC-020), stability is
 * the ladder day for the step and retrievability is base^(t/S), which is
 * `base` on the due day (ADR-0009 §2).
 *
 * concept_mastery is the only stored derivation. Everything else is computed
 * at read time. rebuildConceptMastery replays form pass/miss events when
 * lexicon tags change. It does not run on every lexicon write.
 *
 * Every learner query binds that learner's user id.
 * Provenance: INIT-006/SPEC-006
 */
'use strict';

const db = require('../db');
const reviewRepo = require('../db/repos/review');
const eventsRepo = require('../db/repos/events');
const lessonProgressRepo = require('../db/repos/lesson-progress');
const lexiconRepo = require('../db/repos/lexicon');
const conceptsRepo = require('../db/repos/concepts');
const masteryRepo = require('../db/repos/concept-mastery');
const planStore = require('../plans/store');
const HebrewFsrs = require('../../shared/player/fsrs.js');
const thresholds = require('./thresholds');

const DIRECTIONS = ['he_en', 'en_he'];

/**
 * 'ladder' or 'fsrs' from SRS_SCHEDULER, read on each call so a test can
 * cover both approximations. An empty or unknown value is ladder, matching
 * server/progress.js. The warning lives there, at process start.
 * @returns {'ladder'|'fsrs'}
 */
function schedulerMode() {
    const raw = process.env.SRS_SCHEDULER;
    if (raw == null || String(raw).trim() === '') return 'ladder';
    const value = String(raw).trim().toLowerCase();
    if (value === 'fsrs' || value === 'ladder') return value;
    return 'ladder';
}

/**
 * @param {unknown} userId
 * @returns {number|null}
 */
function requireUser(userId) {
    const n = Number(userId);
    if (!Number.isInteger(n) || n <= 0) return null;
    return n;
}

/**
 * @param {number|Date|string|null|undefined} now
 * @returns {number}
 */
function toTime(now) {
    if (now == null) return Date.now();
    if (now instanceof Date) return now.getTime();
    if (typeof now === 'number' && Number.isFinite(now)) return now;
    if (typeof now === 'string' && now) {
        const parsed = Date.parse(now);
        if (Number.isFinite(parsed)) return parsed;
    }
    return NaN;
}

/**
 * @param {number|Date|string|null|undefined} now
 * @returns {string}
 */
function toIso(now) {
    const t = toTime(now);
    if (!Number.isFinite(t)) return new Date().toISOString();
    return new Date(t).toISOString();
}

/**
 * Days since the epoch for the local calendar date of t.
 * Same numbering as shared/player/fsrs.js.
 * @param {number} t
 * @returns {number}
 */
function localDayNumber(t) {
    const d = new Date(t);
    return Math.round(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / thresholds.MS_PER_DAY);
}

/**
 * @param {string|null|undefined} raw
 * @returns {string[]}
 */
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

/**
 * @param {number} step
 * @returns {number}
 */
function ladderStability(step) {
    const ladder = thresholds.LADDER_STABILITY_DAYS;
    const index = Number.isInteger(step) && step >= 0 ? step : 0;
    const capped = index >= ladder.length ? ladder.length - 1 : index;
    return ladder[capped];
}

/**
 * @param {object} row
 * @returns {boolean}
 */
function everPassed(row) {
    if (row.last_result === 'pass') return true;
    if (Number(row.step) >= thresholds.SCORE.pass) return true;
    if (Number(row.last_grade) >= thresholds.SCORE.minPassGrade) return true;
    return Number(row.passed_event) === thresholds.SCORE.pass;
}

/**
 * A direction counts toward the weak flag only after a real grade.
 * not_yet leaves last_grade empty and last_result as not_yet.
 * @param {object} row
 * @returns {boolean}
 */
function graded(row) {
    if (row.last_grade != null && row.last_grade !== '') return true;
    return row.last_result === 'pass' || row.last_result === 'miss';
}

/**
 * @param {object} row
 * @param {'ladder'|'fsrs'} scheduler
 * @returns {'new'|'learning'|'longTerm'}
 */
function stageOf(row, scheduler) {
    if (!everPassed(row)) return 'new';
    if (scheduler === 'fsrs') {
        let stability = row.stability == null || row.stability === ''
            ? null
            : Number(row.stability);
        if (!Number.isFinite(stability)) stability = ladderStability(Number(row.step));
        return stability >= thresholds.LONG_TERM_STABILITY_DAYS ? 'longTerm' : 'learning';
    }
    const step = Number.isInteger(Number(row.step)) ? Number(row.step) : 0;
    return step >= thresholds.LADDER_LONG_TERM_STEP ? 'longTerm' : 'learning';
}

/**
 * @param {number|null} retrievability
 * @returns {'fragile'|'shaky'|'steady'|'solid'|null}
 */
function strengthBucket(retrievability) {
    if (retrievability == null || !Number.isFinite(retrievability)) return null;
    if (retrievability < thresholds.STRENGTH.fragileBelow) return 'fragile';
    if (retrievability < thresholds.STRENGTH.shakyBelow) return 'shaky';
    if (retrievability < thresholds.STRENGTH.steadyBelow) return 'steady';
    return 'solid';
}

/**
 * Whole local calendar days from last review to now, never negative.
 * @param {string|null|undefined} lastAt
 * @param {number} nowMs
 * @returns {number|null}
 */
function elapsedDays(lastAt, nowMs) {
    if (lastAt == null || lastAt === '') return null;
    const lastMs = toTime(lastAt);
    if (!Number.isFinite(lastMs) || !Number.isFinite(nowMs)) return null;
    const days = localDayNumber(nowMs) - localDayNumber(lastMs);
    return days < 0 ? 0 : days;
}

/**
 * Ladder approximation (ADR-0009 §2): R = base^(t/S), S from the step.
 * @param {object} row
 * @param {number} nowMs
 * @returns {number|null}
 */
function ladderRetrievability(row, nowMs) {
    const stability = ladderStability(Number(row.step));
    const elapsed = elapsedDays(row.last_at, nowMs);
    if (elapsed == null || !(stability > 0)) return null;
    return Math.pow(thresholds.LADDER_RETRIEVABILITY_BASE, elapsed / stability);
}

/**
 * @param {object} row
 * @param {'ladder'|'fsrs'} scheduler
 * @param {number} nowMs
 * @returns {number|null}
 */
function retrievabilityOf(row, scheduler, nowMs) {
    if (scheduler === 'fsrs') {
        const stability = row.stability == null || row.stability === ''
            ? null
            : Number(row.stability);
        return HebrewFsrs.retrievability({
            stability: Number.isFinite(stability) ? stability : null,
            lastAt: row.last_at == null ? null : String(row.last_at)
        }, nowMs);
    }
    return ladderRetrievability(row, nowMs);
}

/**
 * @param {string|null|undefined} due
 * @param {number} nowMs
 * @returns {number|null}
 */
function dueInDays(due, nowMs) {
    if (due == null || due === '') return null;
    const dueMs = toTime(due);
    if (!Number.isFinite(dueMs) || !Number.isFinite(nowMs)) return null;
    return localDayNumber(dueMs) - localDayNumber(nowMs);
}

/**
 * ADR-0009 §3, first matching row. very_weak is worse than weak.
 * @param {object} row
 * @param {string} stage
 * @param {'ladder'|'fsrs'} scheduler
 * @returns {'weak'|'very_weak'|null}
 */
function flagForDirection(row, stage, scheduler) {
    if (!graded(row)) return null;
    const lapses = Number(row.lapses) || 0;
    const lastGrade = row.last_grade == null || row.last_grade === ''
        ? null
        : Number(row.last_grade);
    if (stage === 'longTerm' && lapses >= thresholds.WEAK.longTermLapses) return 'weak';
    if (stage === 'longTerm') return null;
    if (lapses >= thresholds.WEAK.veryWeakLapses) return 'very_weak';
    if (lapses === thresholds.WEAK.weakLapses
        || (lapses === thresholds.WEAK.againWithLapses && lastGrade === thresholds.WEAK.againGrade)) {
        return 'weak';
    }
    if (scheduler === 'fsrs') {
        const difficulty = row.difficulty == null || row.difficulty === ''
            ? null
            : Number(row.difficulty);
        const reps = Number(row.reps) || 0;
        if (difficulty != null
            && difficulty >= thresholds.WEAK.fsrsDifficulty
            && reps >= thresholds.WEAK.fsrsReps) {
            return 'weak';
        }
    }
    return null;
}

/**
 * @param {Array<'weak'|'very_weak'|null>} flags
 * @returns {'weak'|'very_weak'|null}
 */
function worseFlag(flags) {
    let worst = null;
    for (const flag of flags) {
        if (flag === 'very_weak') return 'very_weak';
        if (flag === 'weak') worst = 'weak';
    }
    return worst;
}

/**
 * @param {object[]} rows
 * @param {'ladder'|'fsrs'} scheduler
 * @param {number} nowMs
 * @returns {object}
 */
function wordFromRows(rows, scheduler, nowMs) {
    const entry = {
        word: String(rows[0].word),
        weak: null,
        suspended: false,
        sources: []
    };
    const flags = [];
    const seenSources = new Set();
    let suspended = false;
    const ordered = rows.slice().sort((a, b) => {
        return DIRECTIONS.indexOf(a.direction) - DIRECTIONS.indexOf(b.direction);
    });
    for (const row of ordered) {
        if (DIRECTIONS.indexOf(row.direction) === -1) continue;
        const stage = stageOf(row, scheduler);
        const retrievability = retrievabilityOf(row, scheduler, nowMs);
        entry[row.direction] = {
            stage,
            retrievability,
            strength_bucket: strengthBucket(retrievability),
            lapses: Number(row.lapses) || 0,
            due_in_days: dueInDays(row.due, nowMs)
        };
        flags.push(flagForDirection(row, stage, scheduler));
        if (Number(row.suspended) === 1) suspended = true;
        for (const source of parseSources(row.sources)) {
            if (seenSources.has(source)) continue;
            seenSources.add(source);
            entry.sources.push(source);
        }
    }
    entry.weak = worseFlag(flags);
    entry.suspended = suspended;
    return entry;
}

/**
 * One entry per word that has a review row for this learner.
 * @param {number} userId
 * @param {number|Date|string} now
 * @returns {object[]}
 */
function wordStates(userId, now) {
    const uid = requireUser(userId);
    if (uid == null) return [];
    const nowMs = toTime(now);
    const scheduler = schedulerMode();
    const rows = reviewRepo.listForLearner(uid);
    const groups = new Map();
    for (const row of rows) {
        const word = String(row.word);
        if (!groups.has(word)) groups.set(word, []);
        groups.get(word).push(row);
    }
    const words = [...groups.keys()].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
    return words.map((word) => wordFromRows(groups.get(word), scheduler, nowMs));
}

/**
 * @returns {Set<string>}
 */
function registryKeys() {
    const keys = new Set();
    for (const row of conceptsRepo.list()) keys.add(String(row.key));
    return keys;
}

/**
 * The form's lemma row, or the signal's lemma when the form names none.
 * @param {object|undefined} form
 * @param {string} signalLemma
 * @param {Map<string, object>} byKey
 * @returns {object|undefined}
 */
function lemmaRow(form, signalLemma, byKey) {
    const fromForm = form && form.lemma_key ? String(form.lemma_key) : '';
    if (fromForm && byKey.has(fromForm)) return byKey.get(fromForm);
    if (signalLemma && byKey.has(signalLemma)) return byKey.get(signalLemma);
    return undefined;
}

/**
 * Concept keys for one form, each at most once.
 * tense and cell come from the form. binyan and root-class come from the
 * form, or from the lemma when the form has none. An untagged form yields
 * none. Keys absent from the registry are dropped so a bad tag cannot
 * fail the practice-event transaction.
 * @param {object|undefined} form
 * @param {object|undefined} lemma
 * @param {Set<string>} registry
 * @returns {string[]}
 */
function conceptKeys(form, lemma, registry) {
    if (!form || form.kind !== 'form') return [];
    const keys = [];
    const seen = new Set();
    function add(key) {
        if (!key || seen.has(key) || !registry.has(key)) return;
        seen.add(key);
        keys.push(key);
    }
    if (form.tense) add('tense:' + form.tense);
    if (form.tense && form.cell) add('cell:' + form.tense + '.' + form.cell);
    const binyan = form.binyan || (lemma && lemma.binyan) || null;
    const rootClass = form.root_class || (lemma && lemma.root_class) || null;
    if (binyan) add('binyan:' + binyan);
    if (rootClass) add('root-class:' + rootClass);
    return keys;
}

/**
 * @param {number|null} previous null on the first attempt
 * @param {boolean} pass
 * @returns {number}
 */
function nextEwma(previous, pass) {
    const x = pass ? thresholds.SCORE.pass : thresholds.SCORE.miss;
    if (previous == null || !Number.isFinite(Number(previous))) return x;
    const alpha = thresholds.MASTERY.ewmaAlpha;
    return Number(previous) + alpha * (x - Number(previous));
}

/**
 * @param {string[]} lookupKeys
 * @returns {Map<string, object>}
 */
function lexiconByKey(lookupKeys) {
    const byKey = new Map();
    for (const row of lexiconRepo.listByKeys(lookupKeys)) byKey.set(row.key, row);
    const missing = [];
    for (const row of byKey.values()) {
        if (row.kind === 'form' && row.lemma_key && !byKey.has(row.lemma_key)) {
            missing.push(row.lemma_key);
        }
    }
    if (missing.length) {
        for (const row of lexiconRepo.listByKeys(missing)) byKey.set(row.key, row);
    }
    return byKey;
}

/**
 * Fold one form pass or miss into concept_mastery.
 * A word-card grade (kind absent or 'word') updates nothing. not_yet updates
 * nothing. Called inside applySignals' transaction for kind 'form' only.
 * @param {number} userId
 * @param {{ word?: string, kind?: string, lemma?: string|null, result?: string }} signal
 * @param {number|Date|string} now
 * @returns {{ updated: string[] }}
 */
function recordConceptEvent(userId, signal, now) {
    const uid = requireUser(userId);
    if (uid == null || !signal) return { updated: [] };
    if (signal.kind !== 'form') return { updated: [] };
    if (signal.result !== 'pass' && signal.result !== 'miss') return { updated: [] };
    const word = lexiconRepo.exactKey(signal.word);
    const lemmaKey = signal.lemma ? lexiconRepo.exactKey(signal.lemma) : '';
    if (!word) return { updated: [] };

    const byKey = lexiconByKey(lemmaKey ? [word, lemmaKey] : [word]);
    const form = byKey.get(word);
    const lemma = lemmaRow(form, lemmaKey, byKey);
    const keys = conceptKeys(form, lemma, registryKeys());
    if (!keys.length) return { updated: [] };

    const existing = new Map();
    for (const row of masteryRepo.listByKeys(uid, keys)) {
        existing.set(String(row.concept_key), row);
    }
    const pass = signal.result === 'pass';
    const updatedAt = toIso(now);
    for (const key of keys) {
        const row = existing.get(key);
        const attempts = (row ? Number(row.attempts) : 0) + thresholds.SCORE.pass;
        const correct = (row ? Number(row.correct) : 0)
            + (pass ? thresholds.SCORE.pass : thresholds.SCORE.miss);
        masteryRepo.upsert({
            user_id: uid,
            concept_key: key,
            attempts,
            correct,
            ewma: nextEwma(row ? row.ewma : null, pass),
            updated_at: updatedAt
        });
    }
    return { updated: keys };
}

/**
 * @param {number} attempts
 * @param {number|null} ewma
 * @returns {'unseen'|'emerging'|'very_weak'|'weak'|'ok'|'strong'}
 */
function conceptStatus(attempts, ewma) {
    if (attempts < thresholds.MASTERY.emergingMinAttempts) return 'unseen';
    if (attempts <= thresholds.MASTERY.emergingMaxAttempts) return 'emerging';
    const score = ewma == null || !Number.isFinite(ewma) ? thresholds.SCORE.miss : ewma;
    if (score < thresholds.MASTERY.veryWeakBelow) return 'very_weak';
    if (score < thresholds.MASTERY.weakBelow) return 'weak';
    if (score < thresholds.MASTERY.okBelow) return 'ok';
    return 'strong';
}

/**
 * Every registry concept, in registry order, including unseen ones.
 * accuracy is correct/attempts, or null when attempts is 0. Unrounded.
 * @param {number} userId
 * @returns {object[]}
 */
function conceptStates(userId) {
    const uid = requireUser(userId);
    if (uid == null) return [];
    const stored = new Map();
    for (const row of masteryRepo.listByUser(uid)) {
        stored.set(String(row.concept_key), row);
    }
    return conceptsRepo.list().map((concept) => {
        const row = stored.get(String(concept.key));
        const attempts = row ? Number(row.attempts) : 0;
        const correct = row ? Number(row.correct) : 0;
        const ewma = row && row.ewma != null && Number.isFinite(Number(row.ewma))
            ? Number(row.ewma)
            : null;
        return {
            key: String(concept.key),
            attempts,
            accuracy: attempts > 0 ? correct / attempts : null,
            ewma,
            status: conceptStatus(attempts, ewma)
        };
    });
}

/**
 * Replay this learner's form pass/miss events against the lexicon as it
 * is now, replacing concept_mastery. Same fold as recordConceptEvent.
 * @param {number} userId
 * @returns {object[]} conceptStates after the rebuild
 */
function rebuildConceptMastery(userId) {
    const uid = requireUser(userId);
    if (uid == null) return [];
    const events = eventsRepo.listFormOutcomes(uid);
    const lookup = [];
    for (const event of events) {
        if (event.word) lookup.push(event.word);
        if (event.lemma) lookup.push(event.lemma);
    }
    const byKey = lexiconByKey(lookup);
    const registry = registryKeys();
    const state = new Map();
    for (const event of events) {
        const word = lexiconRepo.exactKey(event.word);
        const lemmaKey = event.lemma ? lexiconRepo.exactKey(event.lemma) : '';
        const form = byKey.get(word);
        const lemma = lemmaRow(form, lemmaKey, byKey);
        const keys = conceptKeys(form, lemma, registry);
        const pass = event.result === 'pass';
        for (const key of keys) {
            const cur = state.get(key) || { attempts: 0, correct: 0, ewma: null };
            const previous = cur.attempts < thresholds.MASTERY.emergingMinAttempts
                ? null
                : cur.ewma;
            cur.ewma = nextEwma(previous, pass);
            cur.attempts += thresholds.SCORE.pass;
            cur.correct += pass ? thresholds.SCORE.pass : thresholds.SCORE.miss;
            state.set(key, cur);
        }
    }
    const updatedAt = new Date().toISOString();
    db.transaction(() => {
        masteryRepo.deleteByUser(uid);
        for (const [key, cur] of state) {
            masteryRepo.upsert({
                user_id: uid,
                concept_key: key,
                attempts: cur.attempts,
                correct: cur.correct,
                ewma: cur.ewma,
                updated_at: updatedAt
            });
        }
    });
    return conceptStates(uid);
}

/**
 * @returns {{ new: number, learning: number, longTerm: number }}
 */
function emptyStages() {
    return { new: 0, learning: 0, longTerm: 0 };
}

/**
 * @param {number} secure he_en long-term words
 * @returns {'script'|'foundations'|'building'|'expanding'|'independent'}
 */
function journeyStage(secure) {
    if (secure < thresholds.JOURNEY.foundationsAt) return 'script';
    if (secure < thresholds.JOURNEY.buildingAt) return 'foundations';
    if (secure < thresholds.JOURNEY.expandingAt) return 'building';
    if (secure < thresholds.JOURNEY.independentAt) return 'expanding';
    return 'independent';
}

/**
 * Nearest ROUND_STEP. Binary multiples of the step are snapped back onto it.
 * @param {number} value
 * @returns {number}
 */
function roundToStep(value) {
    const step = thresholds.ROUND_STEP;
    const steps = Math.round(value / step);
    const text = String(step);
    const dot = text.indexOf('.');
    const places = dot < 0 ? 0 : text.length - dot - 1;
    return Number((steps * step).toFixed(places));
}

/**
 * @param {string} raw
 * @returns {object|null}
 */
function parseJsonObject(raw) {
    try {
        const parsed = JSON.parse(String(raw));
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;
        return parsed;
    } catch (_) {
        return null;
    }
}

/**
 * Completed plan lessons grouped by recipe. {} when no published snapshot exists.
 * A lesson counts once per recipe, matched by lesson id or legacyStorageKey.
 * @param {Set<string>} completed
 * @returns {Record<string, number>}
 */
function recipeCounts(completed) {
    const snapshots = planStore.listSnapshots();
    if (!snapshots.length) return {};
    const counts = {};
    const seen = new Set();
    for (const row of snapshots) {
        const doc = parseJsonObject(row.snapshot);
        const units = doc && Array.isArray(doc.units) ? doc.units : [];
        for (const unit of units) {
            const lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            for (const lesson of lessons) {
                if (!lesson || typeof lesson !== 'object') continue;
                const recipe = typeof lesson.recipe === 'string' ? lesson.recipe : '';
                if (!/^[a-z0-9-]{1,64}$/.test(recipe)) continue;
                const id = typeof lesson.id === 'string' ? lesson.id : '';
                const legacy = typeof lesson.legacyStorageKey === 'string'
                    ? lesson.legacyStorageKey
                    : '';
                const matched = (id && completed.has(id)) || (legacy && completed.has(legacy));
                if (!matched) continue;
                const token = recipe + '\0' + (id || legacy);
                if (seen.has(token)) continue;
                seen.add(token);
                counts[recipe] = (counts[recipe] || 0) + 1;
            }
        }
    }
    return counts;
}

/**
 * Journey label, stage counts, practiced grammar, completed lessons, recipe
 * counts, and the vowels-off share. `now` is accepted so stage rules that
 * depend on the clock stay on the caller's clock. Today's stage rules use
 * step and stability, so `now` is accepted and not read.
 * @param {number} userId
 * @param {number|Date|string} [now]
 * @returns {object}
 */
function journey(userId, now) {
    const uid = requireUser(userId);
    // Stage uses step and stability. `now` stays in the signature for callers.
    const empty = {
        stage: journeyStage(0),
        counts: { he_en: emptyStages(), en_he: emptyStages() },
        lessons_completed: [],
        recipes_completed: {},
        tenses_practiced: [],
        binyanim_practiced: [],
        vowels_off_share: null
    };
    if (uid == null) return empty;
    const scheduler = schedulerMode();
    const counts = { he_en: emptyStages(), en_he: emptyStages() };
    let secure = 0;
    for (const row of reviewRepo.listForLearner(uid)) {
        if (!counts[row.direction]) continue;
        const stage = stageOf(row, scheduler);
        counts[row.direction][stage] += 1;
        if (row.direction === 'he_en' && stage === 'longTerm') secure += 1;
    }

    const lessons = lessonProgressRepo.listByUser(uid);
    const completed = [];
    let records = 0;
    let vowelsOff = 0;
    for (const row of lessons) {
        records += 1;
        const state = parseJsonObject(row.state_json) || {};
        if (Number(state.completedRounds) > 0) completed.push(String(row.slug));
        if (state.vowelsOn === false) vowelsOff += 1;
    }
    completed.sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));

    const tenses = [];
    const binyanim = [];
    const mastery = new Map();
    for (const row of masteryRepo.listByUser(uid)) {
        mastery.set(String(row.concept_key), row);
    }
    for (const concept of conceptsRepo.list()) {
        const row = mastery.get(String(concept.key));
        if (!row || Number(row.attempts) < thresholds.MASTERY.emergingMinAttempts) continue;
        const suffix = String(concept.key).slice(String(concept.family).length + 1);
        if (concept.family === 'tense') tenses.push(suffix);
        else if (concept.family === 'binyan') binyanim.push(suffix);
    }

    return {
        stage: journeyStage(secure),
        counts,
        lessons_completed: completed,
        recipes_completed: recipeCounts(new Set(completed)),
        tenses_practiced: tenses,
        binyanim_practiced: binyanim,
        vowels_off_share: records > 0 ? roundToStep(vowelsOff / records) : null
    };
}

module.exports = {
    wordStates,
    recordConceptEvent,
    conceptStates,
    journey,
    rebuildConceptMastery
};
