/**
 * Compare helper table tests for typed Hebrew scoring and unscramble shuffle.
 * Authored-by-spec: INIT-012/SPEC-005
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const compare = require('../shared/ui/blocks/compare.js');

describe('stripPoints (INIT-012/SPEC-005)', () => {
    it('removes U+0591–U+05C7', () => {
        assert.equal(compare.stripPoints('ב\u05B9\u05BC\u05E7\u05B6\u05E8'), 'בקר');
        assert.equal(compare.stripPoints('מַיִם'), 'מים');
    });
});

describe('normalizeHebrew (INIT-012/SPEC-005)', () => {
    const cases = [
        ['pointed vs unpointed', 'שָׁלוֹם', 'שלום'],
        ['final kaf', 'ך', 'כ'],
        ['final mem', 'ם', 'מ'],
        ['final nun', 'ן', 'נ'],
        ['final pe', 'ף', 'פ'],
        ['final tsadi', 'ץ', 'צ'],
        ['whitespace collapse', '  שלום   עולם  ', 'שלום עולם'],
        ['trailing period', 'שלום.', 'שלום'],
        ['trailing question', 'מה?', 'מה'],
        ['geresh preserved', "צ'יפס", "צ'יפס"],
        ['gershayim normalized', 'צ\u05F4ח', 'צ"ח']
    ];
    cases.forEach(([name, a, b]) => {
        it(name, () => {
            assert.equal(compare.normalizeHebrew(a), compare.normalizeHebrew(b));
        });
    });
});

describe('sameHebrew (INIT-012/SPEC-005)', () => {
    it('matches pointed answer when typed unpointed', () => {
        assert.equal(compare.sameHebrew('מים', 'מַיִם'), true);
        assert.equal(compare.sameHebrew('מַיִם', 'מים'), true);
    });

    it('ac-4: plene wrong without accept entry', () => {
        assert.equal(compare.sameHebrew('שלש', 'שלש', ['שלש']), true);
        assert.equal(compare.sameHebrew('שלוש', 'שלש', ['שלש']), false);
        assert.equal(compare.sameHebrew('שלוש', 'שלש', []), false);
        assert.equal(compare.sameHebrew('שלוש', 'שלש'), false);
    });

    it('ac-4: plene in accept scores correct when typed', () => {
        assert.equal(compare.sameHebrew('שלוש', 'שלש', ['שלוש']), true);
        assert.equal(compare.sameHebrew('שלוש', 'שָׁלֹשׁ', ['שלוש']), true);
    });

    it('rejects empty typed input', () => {
        assert.equal(compare.sameHebrew('', 'שלום'), false);
        assert.equal(compare.sameHebrew('   ', 'שלום'), false);
    });

    it('single-letter answers', () => {
        assert.equal(compare.sameHebrew('א', 'א'), true);
        assert.equal(compare.sameHebrew('ב', 'א'), false);
    });

    it('accept list entries', () => {
        assert.equal(compare.sameHebrew('מיים', 'מַיִם', ['מיים']), true);
    });
});

describe('stepAttempt (INIT-012/SPEC-005)', () => {
    it('retry then reveal', () => {
        assert.equal(compare.stepAttempt(1, false, 2), 'retry');
        assert.equal(compare.stepAttempt(2, false, 2), 'reveal');
        assert.equal(compare.stepAttempt(1, true, 2), 'correct');
    });
});

describe('shuffleWords (INIT-012/SPEC-005)', () => {
    const words = ['א', 'ב', 'ג', 'ד'];

    it('is deterministic for the same seed', () => {
        const a = compare.shuffleWords(words, 'item-1', []);
        const b = compare.shuffleWords(words, 'item-1', []);
        assert.deepEqual(a, b);
    });

    it('never equals the correct order when 2+ distinct positions', () => {
        const shuffled = compare.shuffleWords(words, 'seed-x', []);
        assert.notDeepEqual(shuffled, words);
        assert.notEqual(shuffled.join(' '), words.join(' '));
    });

    it('avoids alt order when possible', () => {
        const alt = [['ג', 'ב', 'א', 'ד']];
        const shuffled = compare.shuffleWords(words, 'alt-seed', alt);
        assert.notDeepEqual(shuffled, words);
        assert.notDeepEqual(shuffled, alt[0]);
    });

    it('ac-10 edge: alt identical to words still shuffles away from main order', () => {
        const main = ['א', 'ב', 'ג'];
        const alt = [main.slice()];
        const shuffled = compare.shuffleWords(main, 'dup-alt-seed', alt);
        assert.notDeepEqual(shuffled, main);
    });

    it('two identical words in unscramble never starts in correct order', () => {
        const words = ['הַיּוֹם', 'טוֹב', 'הַיּוֹם'];
        const shuffled = compare.shuffleWords(words, 'dup-words-seed', []);
        assert.notDeepEqual(shuffled, words);
    });
});
