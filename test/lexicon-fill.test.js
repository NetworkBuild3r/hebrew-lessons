/**
 * Dictionary fill for the teacher builder: a stored paradigm becomes table cells,
 * and verb search collapses forms onto one lemma.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

require('../shared/admin/plan-builder.js');

const Builder = globalThis.HebrewPlanBuilder;

describe('lexicon fill', () => {
    it('builds present, past, and future cells from stored forms only', () => {
        const paradigm = {
            present: {
                ms: { key: 'חוֹשֵׁב' },
                fs: { key: 'חוֹשֶׁבֶת' },
                mp: { key: 'חוֹשְׁבִים' },
                fp: { key: '' }
            },
            past: {
                '1s': { key: 'חָשַׁבְתִּי' },
                '3ms': { key: 'חָשַׁב' }
            },
            future: {
                '1s': { key: 'אֶחְשֹׁב' }
            }
        };
        assert.deepEqual(Builder.cellsFromParadigm(paradigm, 'present'), [
            { cell: 'ms', formKey: 'חוֹשֵׁב' },
            { cell: 'fs', formKey: 'חוֹשֶׁבֶת' },
            { cell: 'mp', formKey: 'חוֹשְׁבִים' }
        ]);
        assert.deepEqual(Builder.cellsFromParadigm(paradigm, 'past'), [
            { cell: '1s', formKey: 'חָשַׁבְתִּי' },
            { cell: '3ms', formKey: 'חָשַׁב' }
        ]);
        assert.deepEqual(Builder.cellsFromParadigm(paradigm, 'future'), [
            { cell: '1s', formKey: 'אֶחְשֹׁב' }
        ]);
        assert.deepEqual(Builder.cellsFromParadigm(null, 'present'), []);
    });

    it('offers one row per verb, preferring the lemma gloss', () => {
        const rows = Builder.verbChoices([
            { key: 'חוֹשֵׁב', kind: 'form', lemma_key: 'לַחְשֹׁב', gloss_en: 'he thinks' },
            { key: 'לַחְשֹׁב', kind: 'lemma', gloss_en: 'to think' },
            { key: 'חוֹשֶׁבֶת', kind: 'form', lemma_key: 'לַחְשֹׁב', gloss_en: 'she thinks' },
            { key: 'לְהַרְגִּישׁ', kind: 'lemma', gloss_en: 'to feel' }
        ]);
        assert.deepEqual(rows, [
            { key: 'לַחְשֹׁב', gloss: 'to think' },
            { key: 'לְהַרְגִּישׁ', gloss: 'to feel' }
        ]);
    });
});
