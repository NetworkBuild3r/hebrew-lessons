/**
 * Author-facing activity names to stored plan block types (ADR-0013 §7,
 * INIT-010 initiative §4). One module for the validator and the admin editor.
 * Loads in Node (require) and in the browser (HebrewActivityMap).
 * Provenance: INIT-010/SPEC-002, INIT-010/SPEC-003, INIT-011/SPEC-002, INIT-011/SPEC-005;
 * Exercise: INIT-012/SPEC-002; Assignment: INIT-012/SPEC-003
 */
(function (root) {
    'use strict';

    /** @type {Readonly<Record<string, string>>} */
    var ACTIVITY_TO_BLOCK = Object.freeze({
        Words: 'introduction',
        Story: 'graded-story',
        Listening: 'listening',
        Dialogue: 'short-dialogue',
        Writing: 'personal-question',
        Notes: 'explainer',
        Verse: 'verse-reading',
        Fluency: 'fluency-pass',
        Review: 'spaced-review-checkpoint',
        Pattern: 'conjugation-table',
        'Type it': 'type-it',
        Exercise: 'exercise',
        Assignment: 'assignment'
    });

    /** Stable menu order for the plan builder (author-facing names). Match omitted until INIT-010 SPEC-004. */
    var ACTIVITY_NAMES = Object.freeze([
        'Words',
        'Story',
        'Listening',
        'Dialogue',
        'Writing',
        'Verse',
        'Pattern',
        'Type it',
        'Exercise',
        'Assignment',
        'Fluency',
        'Review',
        'Notes'
    ]);

    /** One-line teacher hint per activity (Hebrew, English). */
    var ACTIVITY_HINTS = Object.freeze({
        Words: Object.freeze(['בנק מילים, ואז השלמת משפטים', 'Word bank, then fill in the blank']),
        Story: Object.freeze(['קריאה קצרה ושאלות', 'A short reading and questions']),
        Listening: Object.freeze(['האזנה ושאלה', 'Listen, then answer']),
        Dialogue: Object.freeze(['שיחה קצרה לשחק', 'A short dialogue to practice']),
        Writing: Object.freeze(['שאלה לכתיבה אישית', 'A personal writing prompt']),
        Verse: Object.freeze(['פסוק עם פירושים', 'A verse with glosses']),
        Pattern: Object.freeze(['שורות פעלים לחזרה', 'Verb rows to chant']),
        'Type it': Object.freeze(['רמז באנגלית, כתיבה בעברית', 'English cue, type the Hebrew']),
        Exercise: Object.freeze(['תרגיל בחירה, נכון/לא נכון, או כתיבה', 'Choose, true/false, unscramble, translate, or dictation']),
        Assignment: Object.freeze(['כתיבה של כמה משפטים, לפעמים עם שיעורי בית', 'Write several sentences, sometimes with homework lines']),
        Fluency: Object.freeze(['סבב מתוזמן של מילים', 'A timed round of words']),
        Review: Object.freeze(['נקודת חזרה מרווחת', 'A spaced-review checkpoint']),
        Notes: Object.freeze(['הערת מורה', 'A teacher note'])
    });

    /**
     * @param {string} name Author-facing activity name.
     * @returns {string|undefined} Stored block type, or undefined when unknown.
     */
    function blockTypeFor(name) {
        return ACTIVITY_TO_BLOCK[name];
    }

    /**
     * @param {string} name
     * @returns {{ he: string, en: string }|null}
     */
    function hintFor(name) {
        var pair = ACTIVITY_HINTS[name];
        return pair ? { he: pair[0], en: pair[1] } : null;
    }

    var api = {
        ACTIVITY_TO_BLOCK: ACTIVITY_TO_BLOCK,
        ACTIVITY_NAMES: ACTIVITY_NAMES,
        ACTIVITY_HINTS: ACTIVITY_HINTS,
        blockTypeFor: blockTypeFor,
        hintFor: hintFor
    };

    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    }
    root.HebrewActivityMap = api;
})(typeof window !== 'undefined' ? window : globalThis);
