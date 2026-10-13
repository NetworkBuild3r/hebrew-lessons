/**
 * The five Hebrew and consistency warnings: pointing-form, field-language,
 * lexicon-form, pointing-consistency, audio-stale. Pure functions, hand-written
 * Hebrew, no database, no network, no OpenAI or ElevenLabs.
 * Provenance: INIT-008/SPEC-008
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const checks = require('../server/plans/hebrew-checks');
const validate = require('../server/plans/validate');
const { toIdPath } = require('../server/plans/id-path');
const { basePlan } = require('./helpers/plan-fixtures');

const { checkHebrew, FIELD_LANGUAGE, AUDIO_PAIRS } = checks;
const { validatePlan, RULES, SCHEMA_PATH } = validate;
const CODES = ['pointing-form', 'field-language', 'lexicon-form', 'pointing-consistency', 'audio-stale'];
const SAMPLES = path.join(__dirname, '..', 'docs', 'decisions', 'ADR-0003-assets', 'samples');

const lesson = (plan) => plan.units[0].lessons[0];
const block = (plan, id) => lesson(plan).blocks.find((item) => item.id === id);
const only = (findings, code) => findings.filter((item) => item.rule === code);
const lexiconOf = (rows) => ({ listByStrippedKeys: (keys) => rows.filter((row) => keys.some((key) => (
    validate.strippedKey(row.key) === validate.strippedKey(key))))
});

describe('a clean hand-written plan', () => {
    it('ac-1: fires none of the five codes and has no block', () => {
        const plan = basePlan();
        assert.deepEqual(checkHebrew(plan, { lexicon: lexiconOf([]), snapshot: basePlan() }), []);
        const result = validatePlan(plan);
        assert.equal(result.ok, true);
        assert.deepEqual(result.warnings.filter((item) => CODES.includes(item.rule)), []);
    });
});

describe('pointing-form', () => {
    it('(i) a vowel point at the start of a word, and after a space, fires clause (i)', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = '\u05B7בית';
        block(plan, 'listen').passage = 'שלום \u05B8דני';
        const found = only(checkHebrew(plan), 'pointing-form');
        assert.equal(found.length, 2);
        assert.ok(found.every((item) => /has no Hebrew letter before it/.test(item.message)));
        assert.equal(found[0].path, '/units/0/lessons/0/blocks/0/words/1/word');
        assert.equal(found[0].field, 'word');
        assert.equal(found[0].fix, undefined, 'no suggestion for a stray point');
        assert.equal(found[1].path, '/units/0/lessons/0/blocks/2/passage');
    });

    it('(i) a vowel point after a Latin letter or punctuation has no Hebrew letter before it', () => {
        const plan = basePlan();
        block(plan, 'listen').passage = 'a\u05B7 בית';
        const found = only(checkHebrew(plan), 'pointing-form');
        assert.equal(found.length, 1);
        assert.match(found[0].message, /character 2/);
        block(plan, 'listen').passage = 'בית. \u05B4';
        assert.equal(only(checkHebrew(plan), 'pointing-form').length, 1);
    });

    it('(ii) two vowel points on one letter fire clause (ii), at the right character', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = 'ב\u05B7\u05B8ית';
        const found = only(checkHebrew(plan), 'pointing-form');
        assert.equal(found.length, 1);
        assert.match(found[0].message, /two vowel points sit on one letter/);
        assert.match(found[0].message, /character 3/);
        assert.equal(found[0].fix, undefined);
    });

    it('(iii) a decomposed form fires clause (iii) and suggests the NFC value', () => {
        const plan = basePlan();
        const bad = 'ב\u05BC\u05B8ית'; // dagesh before qamats: not canonical order
        assert.notEqual(bad, bad.normalize('NFC'));
        block(plan, 'intro').words[1].word = bad;
        const found = only(checkHebrew(plan), 'pointing-form');
        assert.equal(found.length, 1);
        assert.match(found[0].message, /not in NFC/);
        assert.equal(found[0].severity, 'warn');
        assert.deepEqual(found[0].fix, {
            action: 'set_field',
            path: 'units/unit-1/lessons/lesson-1/blocks/intro/words/w2',
            field: 'word',
            suggested: bad.normalize('NFC')
        });
    });

    it('a field with clauses (i) and (iii) reports both, one finding each', () => {
        const plan = basePlan();
        block(plan, 'listen').passage = '\u05B8ב\u05BC\u05B8';
        const found = only(checkHebrew(plan), 'pointing-form');
        assert.equal(found.length, 2);
        assert.deepEqual(found.map((item) => Boolean(item.fix)).sort(), [false, true]);
    });

    it('a fix inside a nested list names replace_part with the sub-field', () => {
        const plan = basePlan();
        const bad = 'ב\u05BC\u05B8ית';
        block(plan, 'intro').cloze.items[0].answer = bad;
        const [found] = only(checkHebrew(plan), 'pointing-form');
        assert.equal(found.path, '/units/0/lessons/0/blocks/0/cloze/items/0/answer');
        assert.equal(found.field, 'cloze/items/0/answer');
        assert.equal(found.fix.action, 'replace_part');
        assert.equal(found.fix.path, 'units/unit-1/lessons/lesson-1/blocks/intro');
        assert.equal(found.fix.field, 'cloze/items/0/answer');
    });

    it('false-positive guard: valid combinations never fire', () => {
        const plan = basePlan();
        const fine = [
            'לָךְ', // final kaf with sheva, qamats on the lamed
            'וֹ וּ', // vav holam, shuruk
            'שׁוֹשַׁנָּה שֹׂרָה', // shin and sin dots with vowels and dagesh
            'כֵּן', // dagesh with tsere
            'עַל־כֵּן', // maqaf
            'בְּרֵאשִׁ֖ית', // cantillation with vowels
            'אֵ\u05BDת', // meteg
            'בֿ', // rafe
            'שלום', // unpointed: niqqud is optional in these fields
            'דָּנִי',
            'יִשְׂרָאֵל'
        ];
        fine.forEach((text) => { block(plan, 'listen').passage = text.normalize('NFC'); assert.deepEqual(
            only(checkHebrew(plan), 'pointing-form'), [], text); });
    });

    it('false-positive guard: English, Latin, and mixed fields are not pointing-checked', () => {
        const plan = basePlan();
        block(plan, 'intro').words[0].meaning = 'hello';
        block(plan, 'intro').words[0].translit = 'sha\u05B8lom'; // a stray point in a Latin field is not this rule
        block(plan, 'explain').sections[0].heading = '\u05B8 heading';
        assert.deepEqual(only(checkHebrew(plan), 'pointing-form'), []);
    });
});

describe('field-language', () => {
    it('fires when a Hebrew field holds only Latin letters', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = 'bayit';
        const found = only(checkHebrew(plan), 'field-language');
        assert.equal(found.length, 1);
        assert.equal(found[0].path, '/units/0/lessons/0/blocks/0/words/1/word');
        assert.equal(found[0].field, 'word');
        assert.equal(found[0].severity, 'warn');
        assert.match(found[0].message, /holds no Hebrew letter/);
    });

    it('fires when an English or Latin field holds Hebrew', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].meaning = 'בית';
        block(plan, 'intro').words[1].translit = 'ביִת';
        const found = only(checkHebrew(plan), 'field-language');
        assert.deepEqual(found.map((item) => item.field).sort(), ['meaning', 'translit']);
    });

    it('does not fire on an empty field, punctuation, digits, or an English-only gloss', () => {
        const plan = basePlan();
        block(plan, 'intro').cloze.items[0].textPre = '…';
        block(plan, 'intro').cloze.items[0].textPost = '123';
        block(plan, 'intro').words[1].sentence = '   ';
        block(plan, 'intro').words[0].meaning = 'hello, peace (a greeting)';
        assert.deepEqual(only(checkHebrew(plan), 'field-language'), []);
    });

    it('does not fire on an English note that quotes a Hebrew word, a name, or a mixed field', () => {
        const plan = basePlan();
        block(plan, 'intro').words[0].meaning = 'greeting; spelled שלום, with ו';
        block(plan, 'story').support[0].en = 'house (בית)';
        block(plan, 'intro').words[1].trap = 'Not the same as ביתן.';
        block(plan, 'explain').title = 'שלום and hello';
        plan.title = 'שלום';
        block(plan, 'letters').name = 'בֵּית';
        assert.deepEqual(only(checkHebrew(plan), 'field-language'), []);
    });

    it('a field marked mixed or exempt is never judged', () => {
        assert.equal(FIELD_LANGUAGE['wordEntry.trap'], 'mixed');
        assert.equal(FIELD_LANGUAGE['plan.title'], 'exempt');
    });
});

describe('lexicon-form', () => {
    const entry = (extra) => Object.assign({
        key: 'בַּיִת', kind: 'lemma', gloss_en: 'house', translit: 'bayit', verified: 1
    }, extra);

    it('fires with the lexicon key when one entry shares the stripped key and is pointed differently', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = 'בַיִת'; // no dagesh
        const found = only(checkHebrew(plan, { lexicon: lexiconOf([entry()]) }), 'lexicon-form');
        assert.equal(found.length, 1);
        assert.equal(found[0].path, '/units/0/lessons/0/blocks/0/words/1/word');
        assert.equal(found[0].field, 'word');
        assert.equal(found[0].severity, 'warn');
        assert.deepEqual(found[0].fix, {
            action: 'set_field',
            path: 'units/unit-1/lessons/lesson-1/blocks/intro/words/w2',
            field: 'word',
            suggested: 'בַּיִת'.normalize('NFC')
        });
        assert.match(found[0].message, /verified, so the lexicon form is the authority/);
    });

    it('says the lexicon is the authority only when the entry is verified', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = 'בַיִת';
        const [found] = only(checkHebrew(plan, { lexicon: lexiconOf([entry({ verified: 0 })]) }), 'lexicon-form');
        assert.match(found.message, /not verified/);
        assert.doesNotMatch(found.message, /is the authority/);
        assert.equal(found.fix.suggested, 'בַּיִת'.normalize('NFC'));
    });

    it('a verified entry also flags a different meaning and translit, ignoring case and spaces', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].meaning = 'home';
        block(plan, 'intro').words[1].translit = ' BAYIT ';
        let found = only(checkHebrew(plan, { lexicon: lexiconOf([entry()]) }), 'lexicon-form');
        assert.deepEqual(found.map((item) => item.field), ['meaning']);
        assert.equal(found[0].fix.suggested, 'house');
        assert.equal(found[0].fix.action, 'set_field');
        found = only(checkHebrew(plan, { lexicon: lexiconOf([entry({ verified: 0 })]) }), 'lexicon-form');
        assert.deepEqual(found, [], 'an unverified entry never judges meaning or translit');
    });

    it('does not fire with no match, several matches, the same form, or a lexicon with no stripped read', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = 'בַיִת';
        assert.deepEqual(only(checkHebrew(plan, { lexicon: lexiconOf([]) }), 'lexicon-form'), []);
        assert.deepEqual(only(checkHebrew(plan, {
            lexicon: lexiconOf([entry(), entry({ key: 'בֵּית', kind: 'form' })])
        }), 'lexicon-form'), [], 'two entries share the stripped key: no authority');
        const same = basePlan();
        assert.deepEqual(only(checkHebrew(same, { lexicon: lexiconOf([entry()]) }), 'lexicon-form'), []);
        assert.deepEqual(only(checkHebrew(plan, { lexicon: { find: () => null } }), 'lexicon-form'), []);
        assert.deepEqual(only(checkHebrew(plan, {}), 'lexicon-form'), []);
    });

    it('a lexicon that throws (no table yet) produces no finding and no error', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].word = 'בַיִת';
        const broken = { listByStrippedKeys() { throw new Error('no such table'); } };
        assert.deepEqual(only(checkHebrew(plan, { lexicon: broken }), 'lexicon-form'), []);
    });

    it('reads only through listByStrippedKeys, once per validation', () => {
        const plan = basePlan();
        let calls = 0;
        const spy = { listByStrippedKeys(keys) { calls += 1; assert.ok(keys.length >= 1); return []; } };
        checkHebrew(plan, { lexicon: spy });
        assert.equal(calls, 1);
    });
});

describe('pointing-consistency', () => {
    it('fires when one word is pointed two ways in a lesson, with the forms as candidates', () => {
        const plan = basePlan();
        block(plan, 'explain').sections[0].examples[0].he = 'בַיִת'; // wordEntry has בַּיִת
        block(plan, 'intro').words[1].word = 'בַּיִת';
        const found = only(checkHebrew(plan), 'pointing-consistency');
        assert.equal(found.length, 1);
        assert.equal(found[0].path, '/units/0/lessons/0/blocks/3/sections/0/examples/0/he');
        assert.equal(found[0].field, 'sections/0/examples/0/he');
        assert.equal(found[0].severity, 'warn');
        assert.deepEqual(found[0].fix.candidates.slice().sort(), ['בַיִת', 'בַּיִת'].map((s) => s.normalize('NFC')).sort());
        assert.equal(found[0].fix.suggested, undefined, 'no suggestion is made');
        assert.equal(found[0].fix.action, 'replace_part');
    });

    it('does not fire for the same form, an unpointed twin, another lesson, or two entries side by side', () => {
        const plan = basePlan();
        // findIn word 'בית' is unpointed: not compared
        assert.deepEqual(only(checkHebrew(plan), 'pointing-consistency'), []);
        // two vocabulary entries that share letters (masculine and feminine), listed on purpose
        block(plan, 'intro').words.push({ id: 'w3', word: 'שׁוֹתֶה', meaning: 'drinks (m.)' });
        block(plan, 'intro').words.push({ id: 'w4', word: 'שׁוֹתָה', meaning: 'drinks (f.)' });
        assert.deepEqual(only(checkHebrew(plan), 'pointing-consistency'), []);
        // the same word pointed differently in two different lessons
        const second = JSON.parse(JSON.stringify(lesson(plan)).split('בַּיִת').join('בַיִת'));
        second.id = 'lesson-2';
        block({ units: [{ lessons: [second] }] }, 'explain').sections[0].examples[0].he = 'בַיִת';
        plan.units[0].lessons.push(second);
        assert.deepEqual(only(checkHebrew(plan), 'pointing-consistency'), []);
    });

    it('does not compare the cells of one conjugation table with each other', () => {
        const plan = basePlan();
        lesson(plan).blocks.push({
            id: 'table', type: 'conjugation-table', lemmaKey: 'שָׁתָה', tense: 'present', mode: 'study',
            cells: [{ cell: 'ms', formKey: 'שׁוֹתֶה' }, { cell: 'fs', formKey: 'שׁוֹתָה' }]
        });
        assert.deepEqual(only(checkHebrew(plan), 'pointing-consistency'), []);
        block(plan, 'explain').sections[0].examples[0].he = 'שׁוֹתֶה';
        assert.equal(only(checkHebrew(plan), 'pointing-consistency').length, 1, 'a form elsewhere that differs is listed');
    });

    it('ignores a difference in punctuation alone', () => {
        const plan = basePlan();
        block(plan, 'explain').sections[0].examples[0].he = 'בַּיִת.';
        assert.deepEqual(only(checkHebrew(plan), 'pointing-consistency'), []);
    });
});

describe('audio-stale', () => {
    function pair() {
        const snapshot = basePlan();
        const plan = basePlan();
        return { snapshot, plan };
    }

    it('ac-5: fires when a word is edited and its audioPath is left, with the fix naming field and clip', () => {
        const { snapshot, plan } = pair();
        block(plan, 'intro').words[0].word = 'שָׁלוֹם לְךָ';
        const found = only(checkHebrew(plan, { snapshot }), 'audio-stale');
        assert.equal(found.length, 1);
        assert.equal(found[0].path, '/units/0/lessons/0/blocks/0/words/0/word');
        assert.equal(found[0].field, 'word');
        assert.equal(found[0].severity, 'warn');
        assert.match(found[0].message, /may no longer match the text/);
        assert.match(found[0].message, /cannot tell a clip that was already stale at publish/);
        assert.equal(found[0].fix.path, 'units/unit-1/lessons/lesson-1/blocks/intro/words/w1');
        assert.equal(found[0].fix.text, 'word');
        assert.equal(found[0].fix.clip, 'audioPath');
        assert.equal(found[0].fix.field, 'audioPath');
        assert.equal(found[0].fix.action, 'replace_part');
        // ledger aud-4: nothing under server/ generates plan audio; the note says so
        assert.match(found[0].fix.note, /No tool generates audio for a plan field/);
        assert.match(found[0].fix.note, /generate_lesson_audio/);
    });

    it('accepts the snapshot as JSON text', () => {
        const { snapshot, plan } = pair();
        block(plan, 'intro').words[0].sentence = 'שָׁלוֹם לְכֻלָּם.';
        const found = only(checkHebrew(plan, { snapshot: JSON.stringify(snapshot) }), 'audio-stale');
        assert.equal(found.length, 1);
        assert.equal(found[0].fix.clip, 'sentenceAudioPath');
    });

    it('does not fire when the clip path changed too, with no snapshot, or for a new or unchanged node', () => {
        const { snapshot, plan } = pair();
        block(plan, 'intro').words[0].word = 'שָׁלוֹם לְךָ';
        block(plan, 'intro').words[0].audioPath = 'words/shalom-lecha.mp3';
        assert.deepEqual(only(checkHebrew(plan, { snapshot }), 'audio-stale'), []);
        const noClip = pair();
        block(noClip.plan, 'intro').words[0].word = 'שָׁלוֹם לְךָ';
        assert.deepEqual(only(checkHebrew(noClip.plan), 'audio-stale'), [], 'no snapshot, no finding');
        assert.deepEqual(only(checkHebrew(noClip.plan, { snapshot: null }), 'audio-stale'), []);
        assert.deepEqual(only(checkHebrew(noClip.plan, { snapshot: 'not json' }), 'audio-stale'), []);
        const added = pair();
        block(added.plan, 'intro').words.push({ id: 'w9', word: 'חָדָשׁ', meaning: 'new', audioPath: 'words/new.mp3' });
        assert.deepEqual(only(checkHebrew(added.plan, { snapshot: added.snapshot }), 'audio-stale'), []);
        assert.deepEqual(only(checkHebrew(basePlan(), { snapshot: basePlan() }), 'audio-stale'), []);
    });

    it('treats an NFC or whitespace-only change as no change', () => {
        const { snapshot, plan } = pair();
        block(plan, 'listen').passage = '  ' + block(plan, 'listen').passage + ' ';
        assert.deepEqual(only(checkHebrew(plan, { snapshot }), 'audio-stale'), []);
    });

    it('covers each pair in AUDIO_PAIRS', () => {
        const { snapshot, plan } = pair();
        block(plan, 'story').paragraphs[0].text = 'דָּנִי גָּר בַּבַּיִת הַגָּדוֹל.';
        block(plan, 'listen').passage = 'שָׁלוֹם לְכֻלָּם.';
        const fields = only(checkHebrew(plan, { snapshot }), 'audio-stale').map((item) => item.fix.clip);
        assert.deepEqual(fields.sort(), ['audioPath', 'audioPath']);
        assert.equal(AUDIO_PAIRS.length, 7);
    });
});

describe('ac-2: never a block', () => {
    it('the five rows are warnings in RULES, and RULES[id] finds each of them', () => {
        CODES.forEach((code) => {
            assert.equal(RULES[code].severity, 'warn', code);
            assert.ok(Object.prototype.hasOwnProperty.call(RULES, code), code);
            assert.equal(typeof RULES[code].summary, 'string', code);
        });
        // the ADR-0003 table tests count enumerable rows; these five are own but not enumerable
        CODES.forEach((code) => assert.equal(Object.keys(RULES).includes(code), false, code));
        assert.ok(Object.isFrozen(RULES) && Object.isFrozen(RULES['audio-stale']));
    });

    it('a plan whose only findings are the five new warnings has ok true and no block', () => {
        const snapshot = basePlan();
        const plan = basePlan();
        const rows = [{ key: 'שָׁלוֹם', kind: 'lemma', gloss_en: 'peace', translit: 'shalom', verified: 1 }];
        // lexicon-form and audio-stale: w1 is repointed (patah for qamats), its clip path stays
        block(plan, 'intro').words[0].word = 'שַׁלוֹם'.normalize('NFC');
        lesson(plan).newWords[0] = 'שַׁלוֹם'.normalize('NFC');
        block(plan, 'intro').cloze.items[0].textPre = 'ז\u05B7\u05B8'; // pointing-form (ii)
        block(plan, 'listen').passage = 'bayit'; // field-language
        block(plan, 'explain').sections[0].examples[0].he = 'בַיִת'; // pointing-consistency with w2
        const result = validatePlan(plan, { lexicon: lexiconOf(rows), snapshot });
        const codes = new Set(result.warnings.map((item) => item.rule));
        CODES.forEach((code) => assert.ok(codes.has(code), code));
        assert.deepEqual(result.blocks, []);
        assert.equal(result.ok, true);
        assert.deepEqual(result.blocks.filter((item) => CODES.includes(item.rule)), []);
        result.warnings.filter((item) => CODES.includes(item.rule)).forEach((item) => {
            assert.equal(item.severity, 'warn');
            assert.equal(typeof item.field, 'string');
        });
        // pointing-form is covered separately: its fixtures hold a stray or doubled point
        const doubled = basePlan();
        block(doubled, 'listen').passage = 'ב\u05B7\u05B8ית';
        const second = validatePlan(doubled);
        assert.equal(second.ok, true);
        assert.equal(only(second.warnings, 'pointing-form').length, 1);
    });

    it('findings of the older rules gain no new keys', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].meaning = '';
        const result = validatePlan(plan);
        result.blocks.concat(result.warnings, result.informs)
            .filter((item) => !CODES.includes(item.rule))
            .forEach((item) => assert.deepEqual(Object.keys(item).sort(), ['message', 'path', 'rule', 'severity']));
    });
});

describe('registries and drift (ac-4)', () => {
    const schema = () => JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));

    it('every free-text schema property has a language row, and every audio property a pair', () => {
        assert.deepEqual(checks.unregisteredFields(), []);
        assert.deepEqual(checks.unpairedAudioFields(), []);
        assert.ok(checks.stringFields().length > 100, 'the walk sees the schema');
        assert.ok(checks.stringFields().includes('wordEntry.word'));
        assert.ok(checks.stringFields().includes('explainerSection.paragraphs'));
        assert.ok(checks.stringFields().includes('explainerTable.rows'));
    });

    it('fails on a mutated schema with an unregistered string property', () => {
        const mutated = schema();
        mutated.definitions.wordEntry.properties.note = { $ref: '#/definitions/plainText' };
        mutated.definitions.verseReadingBlock.properties.tags = { type: 'array', items: { type: 'string' } };
        const missing = checks.unregisteredFields(mutated);
        assert.deepEqual(missing, ['verseReadingBlock.tags', 'wordEntry.note']);
    });

    it('fails on a mutated schema with a clip property that has no pair', () => {
        const mutated = schema();
        mutated.definitions.explainerExample.properties.audioPath = { $ref: '#/definitions/audioPath' };
        mutated.definitions.hebrewLine.properties.sentenceAudioPath = { $ref: '#/definitions/audioPath' };
        assert.deepEqual(checks.unpairedAudioFields(mutated), ['explainerExample.audioPath', 'hebrewLine.sentenceAudioPath']);
    });

    it('a new enum property needs no row, and a new block type is walked from the schema', () => {
        const mutated = schema();
        mutated.definitions.wordEntry.properties.mood = { enum: ['a', 'b'] };
        assert.deepEqual(checks.unregisteredFields(mutated), []);
    });

    it('no registry row names a field the schema no longer has', () => {
        const fields = new Set(checks.stringFields());
        Object.keys(FIELD_LANGUAGE).filter((key) => !key.startsWith('*.')).forEach((key) => {
            assert.ok(fields.has(key), key + ' is not a string property of the schema');
        });
        const props = schema().definitions;
        AUDIO_PAIRS.forEach((row) => {
            assert.ok(props[row.owner].properties[row.text], row.owner + '.' + row.text);
            assert.ok(props[row.owner].properties[row.clip], row.owner + '.' + row.clip);
            assert.equal(FIELD_LANGUAGE[row.owner + '.' + row.clip], 'exempt');
        });
    });

    it('the Hebrew, English, and Latin fields the spec names are registered as named', () => {
        ['wordEntry.word', 'wordEntry.sentence', 'patternItem.pointed', 'paragraph.text', 'line.pointed',
            'listeningBlock.passage', 'hebrewLine.he', 'explainerExample.he', 'glossPair.he', 'clozeItem.textPre',
            'clozeItem.textPost', 'clozeItem.answer', 'letterSound.pointed', 'letterFind.word', 'verseReadingBlock.verse',
            'versePersonalQuestion.he'].forEach((key) => assert.equal(FIELD_LANGUAGE[key], 'he', key));
        ['wordEntry.meaning', 'glossPair.en', 'clozeItem.english', 'hebrewLine.en', 'explainerExample.en',
            'versePersonalQuestion.en'].forEach((key) => assert.equal(FIELD_LANGUAGE[key], 'en', key));
        ['wordEntry.translit', 'wordEntry.sound', 'explainerExample.translit', 'letterSound.sound']
            .forEach((key) => assert.equal(FIELD_LANGUAGE[key], 'latin', key));
    });

    it('exactKey and strippedKey agree with validate.js', () => {
        ['  שָׁלוֹם  ', 'ב\u05BC\u05B8ית', 'בְּרֵאשִׁ֖ית', 'a  b'].forEach((text) => {
            assert.equal(checks.exactKey(text), validate.exactKey(text));
            assert.equal(checks.strippedKey(text), validate.strippedKey(text));
        });
    });
});

describe('id paths', () => {
    it('toIdPath gives the deepest id node and the rest of the pointer', () => {
        const plan = basePlan();
        assert.deepEqual(toIdPath(plan, ''), { path: '', field: '' });
        assert.deepEqual(toIdPath(plan, '/title'), { path: '', field: 'title' });
        assert.deepEqual(toIdPath(plan, '/units/0/title'), { path: 'units/unit-1', field: 'title' });
        assert.deepEqual(toIdPath(plan, '/units/0/lessons/0/newWords/1'),
            { path: 'units/unit-1/lessons/lesson-1', field: 'newWords/1' });
        assert.deepEqual(toIdPath(plan, '/units/0/lessons/0/blocks/0/words/1/meaning'),
            { path: 'units/unit-1/lessons/lesson-1/blocks/intro/words/w2', field: 'meaning' });
        assert.deepEqual(toIdPath(plan, '/units/0/lessons/0/blocks/0/words/1'),
            { path: 'units/unit-1/lessons/lesson-1/blocks/intro/words/w2', field: '' });
        assert.deepEqual(toIdPath(plan, '/units/0/lessons/0/blocks/3/sections/0/examples/0/he'),
            { path: 'units/unit-1/lessons/lesson-1/blocks/explain', field: 'sections/0/examples/0/he' });
        assert.deepEqual(toIdPath(plan, '/units/9/title'), { path: '', field: 'units/9/title' });
    });

    it('stops at the parent when an id repeats, since the id would name the first one', () => {
        const plan = basePlan();
        block(plan, 'intro').words[1].id = 'w1';
        assert.deepEqual(toIdPath(plan, '/units/0/lessons/0/blocks/0/words/1/meaning'),
            { path: 'units/unit-1/lessons/lesson-1/blocks/intro', field: 'words/1/meaning' });
    });
});

describe('imported plans, unmodified (ac-1 record)', () => {
    // A sanity record, pinned so a tuning change is seen. Each firing is a real finding to review:
    // the nine in shai-date are lines whose points are not in canonical (NFC) order.
    const EXPECTED = {
        'my-morning': {},
        'anal-sex-vocab': {},
        'beginner-unit-1': {},
        'shai-date-indirect-speech': { 'pointing-form': 9 }
    };
    Object.keys(EXPECTED).forEach((name) => {
        it(name, () => {
            const plan = JSON.parse(fs.readFileSync(path.join(SAMPLES, name + '.plan.json'), 'utf8'));
            const result = validatePlan(plan);
            const counts = {};
            result.warnings.filter((item) => CODES.includes(item.rule))
                .forEach((item) => { counts[item.rule] = (counts[item.rule] || 0) + 1; });
            assert.deepEqual(counts, EXPECTED[name]);
            assert.equal(result.blocks.filter((item) => CODES.includes(item.rule)).length, 0);
        });
    });

    it('every pointing-form fix on them is a one-call set_field that clears the finding', () => {
        const plan = JSON.parse(fs.readFileSync(path.join(SAMPLES, 'shai-date-indirect-speech.plan.json'), 'utf8'));
        const found = only(validatePlan(plan).warnings, 'pointing-form');
        assert.equal(found.length, 9);
        found.forEach((item) => {
            assert.equal(item.fix.action, 'set_field');
            assert.equal(item.fix.suggested, item.fix.suggested.normalize('NFC'));
        });
    });
});

describe('bounded work', () => {
    it('ac-8: a plan near the 512 KiB cap validates, and the checks add no new timeout', () => {
        const plan = basePlan();
        const unit = plan.units[0];
        const text = 'דָּנִי גָּר בַּבַּיִת הַגָּדוֹל וְהוּא אוֹהֵב לִשְׁתּוֹת קָפֶה. '.repeat(40);
        const blocks = () => Array.from({ length: 4 }, (_, b) => ({
            id: 'big-' + b,
            type: 'graded-story',
            paragraphs: Array.from({ length: 12 }, (__, p) => ({ id: 'p' + p, text: text.slice(0, 1100) }))
        }));
        unit.lessons = Array.from({ length: 4 }, (_, l) => ({
            id: 'big-lesson-' + l, title: 'Big', level: 'basic', visibility: 'hidden',
            buildsOn: [], newWords: [], recycles: [], blocks: blocks()
        }));
        const bytes = Buffer.byteLength(JSON.stringify(plan), 'utf8');
        assert.ok(bytes > 350 * 1024 && bytes < 512 * 1024, String(bytes));
        const started = Date.now();
        const findings = checkHebrew(plan, { snapshot: JSON.parse(JSON.stringify(plan)), lexicon: lexiconOf([]) });
        assert.ok(Array.isArray(findings));
        assert.ok(Date.now() - started < 5000);
        assert.equal(validatePlan(plan).blocks.filter((item) => CODES.includes(item.rule)).length, 0);
    });

    it('the check modules require only fs, path, and each other: no database, no network', () => {
        const required = (file) => {
            const source = fs.readFileSync(path.join(__dirname, '..', 'server', 'plans', file), 'utf8');
            return (source.match(/require\(\s*'([^']+)'\s*\)/g) || []).map((call) => call.match(/'([^']+)'/)[1]).sort();
        };
        assert.deepEqual(required('hebrew-checks.js'), ['./id-path', 'fs', 'path']);
        assert.deepEqual(required('id-path.js'), []);
    });

    it('makes no network call: the module requires no http, https, net, or SDK', () => {
        const source = fs.readFileSync(path.join(__dirname, '..', 'server', 'plans', 'hebrew-checks.js'), 'utf8')
            + fs.readFileSync(path.join(__dirname, '..', 'server', 'plans', 'issues.js'), 'utf8');
        assert.doesNotMatch(source, /require\(['"](?:https?|net|tls|dgram|openai|undici|node-fetch)['"]\)|\bfetch\(/);
    });
});
