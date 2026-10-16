/**
 * ADR-0009 §1 learner profile: the aggregate object the MCP may read for a
 * learner whose AI-planning switch is on, and that the learner can always read.
 *
 * Every field is copied by name from the INIT-006/SPEC-006 learner model
 * (server/learner/model.js). No model object is spread into the result.
 * Order, caps, and rounding follow ADR-0009 §1 and §3. Access, the hourly MCP
 * count, and the audit row are in server/learner/gate.js.
 * Provenance: INIT-006/SPEC-007
 */
'use strict';

const model = require('./model');
const thresholds = require('./thresholds');
const contract = require('./profile-contract');
const gate = require('./gate');

const DIRECTIONS = ['he_en', 'en_he'];
/** ADR-0009 §3 order: very_weak, then weak, then no flag. */
const FLAG_ORDER = ['very_weak', 'weak'];
/** ADR-0009 §3 order inside a flag group. A direction with no bucket sorts after solid. */
const STRENGTH_ORDER = ['fragile', 'shaky', 'steady', 'solid'];
const RECIPE_ID = /^[a-z0-9-]{1,64}$/;

/**
 * @param {number|Date|string|null|undefined} now
 * @returns {number}
 */
function toMs(now) {
    let t;
    if (now == null) t = Date.now();
    else if (now instanceof Date) t = now.getTime();
    else if (typeof now === 'number') t = now;
    else t = Date.parse(String(now));
    return Number.isFinite(t) ? t : Date.now();
}

/**
 * Nearest ROUND_STEP, clamped to 0..1. Same rounding as the model's vowels-off share.
 * @param {unknown} value
 * @returns {number}
 */
function roundShare(value) {
    const n = Number(value);
    const share = Number.isFinite(n) ? Math.min(1, Math.max(0, n)) : 0;
    const step = thresholds.ROUND_STEP;
    const text = String(step);
    const dot = text.indexOf('.');
    const places = dot < 0 ? 0 : text.length - dot - 1;
    return Number((Math.round(share / step) * step).toFixed(places));
}

/**
 * @param {unknown} value
 * @returns {number} a non-negative integer
 */
function count(value) {
    const n = Number(value);
    return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0;
}

/**
 * @param {unknown} list
 * @returns {string[]}
 */
function stringList(list) {
    return Array.isArray(list) ? list.map(String) : [];
}

/**
 * A direction has a graded review once it has passed or has a lapse. A grade
 * of 2 or more is a pass, and every grade of 1 adds a lapse, so a direction
 * that only has rows from a library add or a `not_yet` fails both tests.
 * @param {{ stage: string, lapses: number }} dir
 * @returns {boolean}
 */
function wasGraded(dir) {
    return dir.stage !== 'new' || count(dir.lapses) > 0;
}

/**
 * One `words[]` entry from one wordStates entry, or null when neither
 * direction has a graded review.
 * @param {object} state
 * @returns {object|null}
 */
function projectWord(state) {
    const entry = { word_id: String(state.word) };
    let graded = false;
    let lapses = 0;
    for (const direction of DIRECTIONS) {
        const dir = state[direction];
        if (!dir || !wasGraded(dir)) continue;
        graded = true;
        entry[direction] = {
            stage: String(dir.stage),
            strength: dir.strength_bucket == null ? null : String(dir.strength_bucket)
        };
        lapses = Math.max(lapses, count(dir.lapses));
    }
    if (!graded) return null;
    if (state.weak === 'weak' || state.weak === 'very_weak') entry.weak = state.weak;
    entry.lapses = Math.min(lapses, contract.LIMITS.lapses);
    return entry;
}

/**
 * @param {object} entry
 * @returns {number}
 */
function flagRank(entry) {
    const index = FLAG_ORDER.indexOf(entry.weak);
    return index === -1 ? FLAG_ORDER.length : index;
}

/**
 * The weaker of the entry's graded directions.
 * @param {object} entry
 * @returns {number}
 */
function strengthRank(entry) {
    let rank = STRENGTH_ORDER.length;
    for (const direction of DIRECTIONS) {
        if (!entry[direction]) continue;
        const index = STRENGTH_ORDER.indexOf(entry[direction].strength);
        if (index !== -1 && index < rank) rank = index;
    }
    return rank;
}

/**
 * @param {string} a
 * @param {string} b
 * @returns {number}
 */
function compareCodePoints(a, b) {
    const x = Array.from(a);
    const y = Array.from(b);
    const n = Math.min(x.length, y.length);
    for (let i = 0; i < n; i += 1) {
        const diff = x[i].codePointAt(0) - y[i].codePointAt(0);
        if (diff !== 0) return diff;
    }
    return x.length - y.length;
}

/**
 * ADR-0009 §3 profile order, read from returned fields only, so the order
 * says nothing the fields do not (lapses above 9 do not reorder words).
 * @param {object} a
 * @param {object} b
 * @returns {number}
 */
function compareWords(a, b) {
    return (flagRank(a) - flagRank(b))
        || (strengthRank(a) - strengthRank(b))
        || (b.lapses - a.lapses)
        || compareCodePoints(a.word_id, b.word_id);
}

/**
 * @param {object[]} ordered
 * @param {'weak'|'very_weak'} flag
 * @returns {string[]}
 */
function flaggedIds(ordered, flag) {
    return ordered
        .filter((entry) => entry.weak === flag)
        .slice(0, contract.LIMITS.flaggedWords)
        .map((entry) => entry.word_id);
}

/**
 * @param {object[]} states model.wordStates output
 * @returns {{ total: number, list: object[], weak: string[], veryWeak: string[] }}
 */
function projectWords(states) {
    const ordered = [];
    for (const state of states) {
        const entry = projectWord(state);
        if (entry) ordered.push(entry);
    }
    ordered.sort(compareWords);
    return {
        total: ordered.length,
        list: ordered.slice(0, contract.LIMITS.words),
        weak: flaggedIds(ordered, 'weak'),
        veryWeak: flaggedIds(ordered, 'very_weak')
    };
}

/**
 * One `grammar[]` entry. accuracy and recent appear when attempts is above 0.
 * A stored row with no recent score counts as a miss, as the model's status does.
 * @param {object} concept model.conceptStates entry
 * @returns {object}
 */
function projectConcept(concept) {
    const attempts = count(concept.attempts);
    const entry = {
        concept: String(concept.key),
        status: String(concept.status),
        attempts
    };
    if (attempts > 0) {
        entry.accuracy = roundShare(concept.accuracy);
        entry.recent = roundShare(concept.ewma == null ? thresholds.SCORE.miss : concept.ewma);
    }
    return entry;
}

/**
 * @param {object|undefined} counts
 * @returns {{ new: number, learning: number, longTerm: number }}
 */
function stageCounts(counts) {
    const source = counts || {};
    return {
        new: count(source.new),
        learning: count(source.learning),
        longTerm: count(source.longTerm)
    };
}

/**
 * @param {unknown} map recipe id to completed-lesson count
 * @returns {Record<string, number>}
 */
function recipeCounts(map) {
    const out = {};
    if (!map || typeof map !== 'object') return out;
    for (const recipe of Object.keys(map).sort()) {
        if (RECIPE_ID.test(recipe)) out[recipe] = count(map[recipe]);
    }
    return out;
}

/**
 * @param {object} trip model.journey output
 * @returns {object}
 */
function projectJourney(trip) {
    const counts = trip.counts || {};
    return {
        stage: String(trip.stage),
        counts: {
            he_en: stageCounts(counts.he_en),
            en_he: stageCounts(counts.en_he)
        },
        lessons_completed: stringList(trip.lessons_completed),
        recipes_completed: recipeCounts(trip.recipes_completed),
        tenses_practiced: stringList(trip.tenses_practiced),
        binyanim_practiced: stringList(trip.binyanim_practiced),
        vowels_off_share: trip.vowels_off_share == null ? null : roundShare(trip.vowels_off_share)
    };
}

/**
 * @param {{ id: number, username: string }} learner
 * @param {number} nowMs
 * @returns {object}
 */
function project(learner, nowMs) {
    const words = projectWords(model.wordStates(learner.id, nowMs));
    return {
        username: learner.username,
        profile_version: contract.PROFILE_VERSION,
        journey: projectJourney(model.journey(learner.id, nowMs)),
        words_total: words.total,
        words: words.list,
        weak_words: words.weak,
        very_weak_words: words.veryWeak,
        grammar: model.conceptStates(learner.id).map(projectConcept)
    };
}

/**
 * Audit detail for a returned profile: field paths and counts, no word ids.
 * word_count is every distinct word id in the response.
 * @param {object} profile
 * @returns {object}
 */
function describeProfile(profile) {
    const ids = new Set(profile.weak_words.concat(profile.very_weak_words));
    for (const word of profile.words) ids.add(word.word_id);
    return {
        fields: contract.fieldPaths(profile),
        word_count: ids.size,
        concept_count: profile.grammar.length,
        lesson_count: profile.journey.lessons_completed.length
    };
}

/**
 * ADR-0009 §1 profile for one learner.
 *
 * `audience: 'mcp'` is the MCP adapter. `learner` is the username the tool was
 * given, or a user id. A refusal is `{ error }` with no other key:
 * `rate_limited`, `not_found` (no such account, or a disabled one), or
 * `planning_off`. Every call counts toward the hourly MCP limit it shares with
 * coverage and writes one `learner_profile_read` audit row.
 *
 * `audience: 'self'` is `GET /api/me/profile`. `learner` is the signed-in
 * user's id. The result is the same object plus `ai_planning`, whatever the
 * switch says. No audit row and no rate count.
 *
 * `now` (default: the current time) is the day strength is measured on. The
 * hourly count and the audit time use the server clock, whatever `now` says.
 *
 * @param {number|string} learner
 * @param {{ audience: 'mcp'|'self', now?: number|Date|string }} opts
 * @returns {object}
 */
function buildProfile(learner, opts) {
    const options = opts || {};
    const audience = gate.audienceOf(options.audience);
    const nowMs = toMs(options.now);
    const action = contract.AUDIT.profileRead;
    const access = gate.admit(learner, { audience, action, invalid: null });
    if (access.refusal) return access.refusal;
    const subject = access.learner;
    if (audience === 'self') {
        const own = project(subject, nowMs);
        own.ai_planning = subject.aiPlanning;
        return own;
    }
    return gate.auditedRead(action, subject, () => project(subject, nowMs), describeProfile);
}

module.exports = {
    buildProfile
};
