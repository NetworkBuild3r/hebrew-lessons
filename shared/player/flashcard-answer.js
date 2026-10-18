/**
 * Local flashcard answer check. Compares a typed string to a key and accept[]
 * after stripping Hebrew vowels. Suggests a grade; it does not schedule, and
 * it does not put the typed text on the signal the server receives.
 *
 * Authored-by-spec: INIT-006/SPEC-012
 */
(function (root) {
    'use strict';

    // Maqaf (U+05BE) stays. An ASCII hyphen is the same joiner, so it becomes a maqaf.
    // This matches HebrewUtils.stripVowels, including when that file is not loaded.
    var VOWELS = /[\u0591-\u05BD\u05BF-\u05C7]/g;

    function stripVowels(value) {
        var utils = root.HebrewUtils;
        var raw = String(value == null ? '' : value);
        var stripped = utils && typeof utils.stripVowels === 'function'
            ? utils.stripVowels(raw)
            : raw.replace(VOWELS, '').replace(/\u002D/g, '\u05BE');
        return stripped.normalize('NFC').replace(/\s+/g, ' ').trim();
    }

    /**
     * True when the vowel-stripped typing equals the stripped key or one accept[] entry.
     * @param {string} typed
     * @param {string} key
     * @param {string[]} [accept]
     * @returns {boolean}
     */
    function matches(typed, key, accept) {
        var got = stripVowels(typed);
        if (!got) return false;
        if (got === stripVowels(key)) return true;
        var list = Array.isArray(accept) ? accept : [];
        for (var i = 0; i < list.length; i += 1) {
            if (typeof list[i] === 'string' && got === stripVowels(list[i])) return true;
        }
        return false;
    }

    /**
     * A grade to highlight. Empty typing suggests nothing. A match suggests Good (3).
     * Any other typing suggests Again (1). The learner still chooses.
     * @param {string} typed
     * @param {string} key
     * @param {string[]} [accept]
     * @returns {1|3|null}
     */
    function suggestGrade(typed, key, accept) {
        if (!stripVowels(typed)) return null;
        return matches(typed, key, accept) ? 3 : 1;
    }

    /**
     * @param {string} key
     * @returns {1|2|3|4|null}
     */
    function keyToGrade(key) {
        if (key === '1' || key === '2' || key === '3' || key === '4') return Number(key);
        return null;
    }

    /**
     * Signal fields the server accepts. Typed text, the prompt, and the answer
     * are never copied onto it.
     * @param {{ word?: string, direction?: string }} card
     * @param {number} grade
     * @param {string} [slug]
     * @returns {{ word: string, direction: string, result: string, grade: number, activity: string, slug?: string }|null}
     */
    function gradeSignal(card, grade, slug) {
        var g = Number(grade);
        if (g !== 1 && g !== 2 && g !== 3 && g !== 4) return null;
        var signal = {
            word: card && card.word ? String(card.word) : '',
            direction: card && card.direction === 'en_he' ? 'en_he' : 'he_en',
            result: g === 1 ? 'miss' : 'pass',
            grade: g,
            activity: 'flashcard'
        };
        if (slug) signal.slug = String(slug);
        return signal;
    }

    var api = {
        stripVowels: stripVowels,
        matches: matches,
        suggestGrade: suggestGrade,
        keyToGrade: keyToGrade,
        gradeSignal: gradeSignal
    };

    root.HebrewFlashcardAnswer = api;
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    }
})(typeof window !== 'undefined' ? window : globalThis);
