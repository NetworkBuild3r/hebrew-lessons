/**
 * Wiktionary DSL parsing and suggestion order.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { parseDsl, foldEnglish, foldHebrew } = require('../server/dictionary/parse-dsl');

const SAMPLE = [
    'mother',
    'אֵם',
    'em',
    '\t[b][c darkblue]mother[/c][/b] [p]<n>[/p] [i](female parent)[/i]',
    '\t[m1]אֵם [p]<f>[/p] [c gray]/em/[/c][/m]',
    '',
    'mother',
    '\t[b][c darkblue]mother[/c][/b] [p]<n>[/p]',
    '\t[m1]אֵם /em/, אמא [p]<f>[/p] [c gray]/imma/[/c][/m]',
    '',
    'dad',
    '\t[b][c darkblue]dad[/c][/b] [p]<n>[/p]',
    '\t[m1]אַבָּא [p]<m>[/p] [c gray]/ába/[/c][/m]',
    '',
    'man',
    '\t[b][c darkblue]man[/c][/b] [p]<n>[/p]',
    '\t[m1]אדם [p]<m>[/p] [c gray]/adam/[/c], גבר /gever/, איש [p]<m>[/p] [c gray]/ish/[/c][/m]',
    '',
    'think',
    '\t[b][c darkblue]think[/c][/b] [p]<v>[/p]',
    '\t[m1]head=חָשַׁב /khasháv/[/m]',
    ''
].join('\n');

describe('wiktionary dsl', () => {
    it('keeps one Hebrew–English pair and drops the head= marker', () => {
        const rows = parseDsl(SAMPLE);
        assert.deepEqual(rows, [
            { he: 'אֵם', en: 'mother' },
            { he: 'אמא', en: 'mother' },
            { he: 'אַבָּא', en: 'dad' },
            { he: 'אדם', en: 'man' },
            { he: 'גבר', en: 'man' },
            { he: 'איש', en: 'man' },
            { he: 'חָשַׁב', en: 'think' }
        ]);
    });
});

describe('dictionary suggest', () => {
    let tmp;
    let suggest;
    let dictionary;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-dict-'));
        process.env.DATA_DIR = tmp;
        const db = require('../server/db');
        db.close();
        db.migrate();
        dictionary = require('../server/db/repos/dictionary');
        suggest = require('../server/dictionary/suggest').suggest;
        dictionary.replaceAll([
            { he: 'אֵם', en: 'mother', heFold: foldHebrew('אֵם'), enFold: foldEnglish('mother') },
            { he: 'אמא', en: 'mother', heFold: foldHebrew('אמא'), enFold: foldEnglish('mother') },
            { he: 'אַבָּא', en: 'dad', heFold: foldHebrew('אַבָּא'), enFold: foldEnglish('dad') },
            { he: 'אבא', en: 'daddy', heFold: foldHebrew('אבא'), enFold: foldEnglish('daddy') },
            { he: 'בן זונה', en: 'motherfucker', heFold: foldHebrew('בן זונה'), enFold: foldEnglish('motherfucker') },
            { he: 'שפת אם', en: 'mother tongue', heFold: foldHebrew('שפת אם'), enFold: foldEnglish('mother tongue') }
        ]);
    });

    after(() => {
        try {
            require('../server/db').close();
        } catch (_) { /* closed */ }
        fs.rmSync(tmp, { recursive: true, force: true });
    });

    it('finds mother from English and from unpointed Hebrew', () => {
        const byEnglish = suggest('mother', 8);
        assert.equal(byEnglish.ok, true);
        assert.deepEqual(byEnglish.entries.slice(0, 2).map((row) => row.key), ['אֵם', 'אמא']);
        const byHebrew = suggest('אמא', 8);
        assert.equal(byHebrew.entries[0].key, 'אמא');
        assert.equal(byHebrew.entries[0].gloss_en, 'mother');
        const dad = suggest('dad', 8);
        assert.equal(dad.entries[0].key, 'אַבָּא');
        assert.equal(dad.entries[0].gloss_en, 'dad');
        assert.ok(dad.entries.some((row) => row.gloss_en === 'daddy'));
        const mothers = byEnglish.entries.map((row) => row.en || row.gloss_en);
        assert.ok(mothers.includes('mother'));
        assert.ok(mothers.includes('mother tongue'));
        assert.equal(mothers.includes('motherfucker'), false);
    });
});
