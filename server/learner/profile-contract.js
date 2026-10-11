/**
 * ADR-0009 §1 learner-profile contract.
 * RETURNED_FIELDS is the "Returned" table written as field paths.
 * NEVER_RETURNED is the "Never returned" table written as the field names this
 * codebase uses for those items, copied once, one group per row of that table.
 * server/learner/profile.js builds the profile field by field and does not read
 * these lists; the tests compare its output with them. A change to either list
 * is a change to ADR-0009 §1.
 * Provenance: INIT-006/SPEC-007
 */
'use strict';

/** ADR-0009 §1 `profile_version`. */
const PROFILE_VERSION = 1;

/**
 * Every path an `audience: 'mcp'` profile may hold. `[]` is an array element.
 * `*` is a key of a map whose keys are data (recipe ids). Some paths appear only
 * when they apply: `words[].he_en` and `words[].en_he` for a direction with a
 * graded review, `words[].weak` for a flagged word, `grammar[].accuracy` and
 * `grammar[].recent` when attempts is above 0, and `journey.recipes_completed.*`
 * once a plan lesson with a recipe is complete.
 */
const RETURNED_FIELDS = Object.freeze([
    'username',
    'profile_version',
    'journey',
    'journey.stage',
    'journey.counts',
    'journey.counts.he_en',
    'journey.counts.he_en.new',
    'journey.counts.he_en.learning',
    'journey.counts.he_en.longTerm',
    'journey.counts.en_he',
    'journey.counts.en_he.new',
    'journey.counts.en_he.learning',
    'journey.counts.en_he.longTerm',
    'journey.lessons_completed',
    'journey.recipes_completed',
    'journey.recipes_completed.*',
    'journey.tenses_practiced',
    'journey.binyanim_practiced',
    'journey.vowels_off_share',
    'words_total',
    'words',
    'words[].word_id',
    'words[].he_en',
    'words[].he_en.stage',
    'words[].he_en.strength',
    'words[].en_he',
    'words[].en_he.stage',
    'words[].en_he.strength',
    'words[].weak',
    'words[].lapses',
    'weak_words',
    'very_weak_words',
    'grammar',
    'grammar[].concept',
    'grammar[].status',
    'grammar[].attempts',
    'grammar[].accuracy',
    'grammar[].recent'
]);

/** Added for `audience: 'self'` only: the learner's own switch state. */
const SELF_FIELDS = Object.freeze(['ai_planning']);

/** Paths whose keys are data. fieldPaths reports their keys as `*`. */
const MAP_FIELDS = Object.freeze(['journey.recipes_completed']);

/** The coverage result (ADR-0009 §1, "Coverage tool"). */
const COVERAGE_FIELDS = Object.freeze(['coverage', 'known_count', 'running_words', 'unknown']);

/**
 * ADR-0009 §1 "Never returned". No name here is also a returned field name, so
 * a test can fail on any key or string value that equals one.
 */
const NEVER_RETURNED = Object.freeze([
    // Display name, numeric user id, role, password hash, session token,
    // auth provider fields, or any other account secret.
    'display_name', 'displayName', 'id', 'user_id', 'userId', 'role',
    'password', 'password_hash', 'passwordHash', 'token', 'token_hash',
    'session', 'sessions', 'auth_provider', 'provider_subject', 'must_change_password',
    // practice_events rows and every field of one: result, time, lesson slug,
    // activity, client event id, kind, lemma, the grade of one event.
    'practice_events', 'events', 'word', 'direction', 'result', 'at', 'time', 'slug',
    'activity', 'kind', 'lemma', 'grade',
    // Review-row schedule and memory fields.
    'due', 'step', 'streak', 'last_result', 'lastResult', 'last_at', 'lastAt',
    'last_grade', 'lastGrade', 'stability', 'difficulty', 'reps', 'fsrs_state', 'state',
    'sources', 'due_in_days',
    // Any timestamp or date, time-of-day, or session-length figure.
    'created_at', 'updated_at', 'added_at', 'last_seen_at', 'expires_at',
    // Typed text, free text, answers given (right or wrong), wrong choices.
    'text', 'typed', 'answer', 'answers', 'choice', 'choices',
    // English glosses, transliteration, or any other meaning text.
    'gloss', 'gloss_en', 'meaning', 'translit', 'sound', 'sentence', 'note',
    // Lesson-record fields other than the completed-slug list and the vowels-off share.
    'lesson', 'wordMarks', 'missedLast', 'activityState', 'completedSteps', 'stepIndex',
    'roundStartedAt', 'completedRounds', 'round', 'vowelsOn', 'state_json', 'rev',
    // Exact accuracy, exact stability, exact retrievability.
    'retrievability', 'ewma', 'correct',
    // Library fields: user_words source, added date, and the learner's own note;
    // the suspended flag on a review row.
    'user_words', 'source', 'suspended',
    // Any audit_log content.
    'audit_log', 'actor', 'action', 'target', 'detail'
]);

/** ADR-0009 §1 caps. */
const LIMITS = Object.freeze({
    /** Entries in `words[]`. */
    words: 500,
    /** Entries in `weak_words` and in `very_weak_words`, each. */
    flaggedWords: 50,
    /** `words[].lapses` ceiling; 9 means nine or more. */
    lapses: 9,
    /** Characters of coverage text. */
    coverageText: 4000,
    /** Distinct tokens in coverage `unknown`. */
    coverageUnknown: 100,
    /** MCP profile and coverage reads per clock hour, one count for both. */
    mcpReadsPerHour: 60
});

/** ADR-0009 §1 audit actor and actions. */
const AUDIT = Object.freeze({
    actor: 'mcp',
    profileRead: 'learner_profile_read',
    coverageRead: 'learner_coverage_read'
});

/**
 * Paths present in a value, in RETURNED_FIELDS notation, sorted, each once.
 * Keys under a MAP_FIELDS path are reported as `*`.
 * @param {unknown} value
 * @returns {string[]}
 */
function fieldPaths(value) {
    const found = new Set();
    const maps = new Set(MAP_FIELDS);
    function walk(node, prefix) {
        if (Array.isArray(node)) {
            for (const item of node) {
                if (item !== null && typeof item === 'object') walk(item, prefix + '[]');
            }
            return;
        }
        if (node === null || typeof node !== 'object') return;
        for (const key of Object.keys(node)) {
            if (maps.has(prefix)) {
                found.add(prefix + '.*');
                continue;
            }
            const path = prefix === '' ? key : prefix + '.' + key;
            found.add(path);
            walk(node[key], path);
        }
    }
    walk(value, '');
    return [...found].sort();
}

module.exports = {
    PROFILE_VERSION,
    RETURNED_FIELDS,
    SELF_FIELDS,
    MAP_FIELDS,
    COVERAGE_FIELDS,
    NEVER_RETURNED,
    LIMITS,
    AUDIT,
    fieldPaths
};
