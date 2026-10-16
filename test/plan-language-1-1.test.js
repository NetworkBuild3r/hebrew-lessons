/**
 * Plan language 1.1.0: the five ADR-0009 §7 blocks, the lesson recipe, the
 * introduction-word group, rules N13 to N19, the changes to N1, N2, N6, and
 * N7, and the schemaVersion raise in ops.js. Validator tests are pure and use
 * an in-memory lexicon; the operation tests run on a temporary database.
 * Provenance: INIT-006/SPEC-011
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { withCtx, publishWithCtx } = require('./helpers/plan-ctx');
const { RULES, SCHEMA_PATH, validatePlan } = require('../server/plans/validate');
const { SAMPLE_NAMES, VARIANTS, sample } = require('./fixtures/plans-1.1/variants-1.0.0');

const ROOT = path.resolve(__dirname, '..');
const FIXTURES = path.join(__dirname, 'fixtures', 'plans-1.1');
const ADR_0009 = path.join(ROOT, 'docs', 'decisions', 'ADR-0009-learner-model-and-language-program.md');
const BASELINE = JSON.parse(fs.readFileSync(path.join(FIXTURES, 'samples-1.0.0-results.json'), 'utf8'));
const LEXICON_ENTRIES = JSON.parse(fs.readFileSync(path.join(FIXTURES, 'lexicon.json'), 'utf8')).entries;
const SCHEMA = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));

const BLOCK_TYPES = ['explainer', 'conjugation-table', 'personal-question', 'verse-reading', 'letter-intro'];
const NEW_RULES = ['N13', 'N14', 'N15', 'N16', 'N17', 'N18', 'N19'];
const L0 = '/units/0/lessons/0';
const L2 = '/units/0/lessons/2';
const B = ['units', 0, 'lessons', 0, 'blocks'];

function exactKey(value) {
    return String(value).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/** An in-memory stand-in for server/db/repos/lexicon.js `find`, with a call log. */
function memoryLexicon(entries) {
    const rows = new Map(entries.map((entry) => [exactKey(entry.key), {
        key: exactKey(entry.key),
        kind: entry.kind,
        lemma_key: entry.lemma_key ? exactKey(entry.lemma_key) : null,
        tense: entry.tense || null,
        cell: entry.cell || null,
        verified: entry.verified ? 1 : 0
    }]));
    const calls = [];
    return {
        calls,
        find(key) {
            calls.push(key);
            return rows.get(exactKey(key)) || null;
        }
    };
}

const LEXICON = memoryLexicon(LEXICON_ENTRIES);

function fixture(name) {
    return JSON.parse(fs.readFileSync(path.join(FIXTURES, name + '.plan.json'), 'utf8'));
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function lesson(plan, index) {
    return plan.units[0].lessons[index || 0];
}

function all(result) {
    return result.blocks.concat(result.warnings, result.informs);
}

function hits(findings, rule, at) {
    return findings.filter((item) => item.rule === rule && (at === undefined || item.path === at));
}

/**
 * These tests pin the ADR-0003 and ADR-0009 rules. INIT-008/SPEC-008 added five warnings
 * (pointing-form and four more) that read the same documents; they are tested in
 * plan-hebrew-checks.test.js and left out of these comparisons. Nothing else changed.
 */
const INIT_008_RULES = ['pointing-form', 'field-language', 'lexicon-form', 'pointing-consistency', 'audio-stale'];

function withoutInit008(result) {
    return Object.assign({}, result, {
        warnings: result.warnings.filter((item) => !INIT_008_RULES.includes(item.rule))
    });
}

function check(plan) {
    return withoutInit008(validatePlan(plan, { lexicon: LEXICON }));
}

function shape(result) {
    return all(result).map((item) => item.severity + ' ' + item.rule + ' ' + item.path);
}

function pointer(segments) {
    return '/' + segments.join('/');
}

function at(root, segments) {
    return segments.reduce((node, key) => node[key], root);
}

function setAt(root, segments, value) {
    at(root, segments.slice(0, -1))[segments[segments.length - 1]] = value;
}

/** The fixture block of each new type, by index in its fixture lesson. */
const BLOCK_AT = {
    explainer: ['explainer', 0],
    'conjugation-table': ['conjugation-table', 1],
    'personal-question': ['personal-question', 3],
    'verse-reading': ['verse-reading', 0],
    'letter-intro': ['letter-intro', 0]
};

function fixtureBlock(type) {
    const [name, index] = BLOCK_AT[type];
    return clone(lesson(fixture(name)).blocks[index]);
}

function addWord(target, word, meaning, group) {
    target.newWords.push(word);
    const words = target.blocks[0].words;
    const entry = { id: String(words.length + 1), word, meaning, sentence: word + '.' };
    if (group !== undefined) entry.group = group;
    words.push(entry);
}

const MORE_WORDS = [
    ['סַבָּא', 'grandpa'], ['סָבְתָא', 'grandma'], ['דּוֹד', 'uncle'], ['דּוֹדָה', 'aunt'], ['בֵּן', 'son'],
    ['בַּת', 'daughter'], ['חָתוּל', 'cat'], ['חֶדֶר', 'room'], ['גַּן', 'garden'], ['מִטְבָּח', 'kitchen']
];

/** The topic-conversation fixture (six words) with `count` new words in total. */
function topicWithWords(count) {
    const plan = fixture('personal-question');
    MORE_WORDS.slice(0, count - 6).forEach(([word, meaning]) => addWord(lesson(plan), word, meaning, 3));
    assert.equal(lesson(plan).newWords.length, count);
    return plan;
}

function dialogue(id) {
    return { id, type: 'short-dialogue', lines: [{ id: 'l1', speaker: 'learner', pointed: 'שָׁלוֹם.' }] };
}

/** A 1.1.0 plan with one unit of one lesson holding `blocks`, with `recipe` when given. */
function oneLesson(blocks, recipe) {
    const plan = fixture('explainer');
    const item = lesson(plan);
    item.blocks = blocks;
    if (recipe) item.recipe = recipe;
    else delete item.recipe;
    return plan;
}

describe('ac-1 and und-1: 1.0.0 documents keep their results', () => {
    it('the four ADR-0003 samples stay 1.0.0 and return the results recorded before this change', () => {
        assert.deepEqual(Object.keys(BASELINE.samples).sort(), SAMPLE_NAMES.slice().sort());
        SAMPLE_NAMES.forEach((name) => {
            const plan = sample(name);
            assert.equal(plan.schemaVersion, '1.0.0', name);
            assert.deepEqual(withoutInit008(validatePlan(plan)), BASELINE.samples[name], name);
            assert.deepEqual(check(plan), BASELINE.samples[name], name + ' with a lexicon');
            assert.equal(BASELINE.samples[name].ok, true, name);
        });
    });

    it('1.0.0 variants that trip N1, N2, N6, N7, and the further rules return the recorded results', () => {
        assert.deepEqual(Object.keys(BASELINE.variants).sort(), Object.keys(VARIANTS).sort());
        const tripped = new Set();
        Object.keys(VARIANTS).forEach((name) => {
            const result = validatePlan(VARIANTS[name]());
            assert.deepEqual(result, BASELINE.variants[name], name);
            assert.deepEqual(check(VARIANTS[name]()), BASELINE.variants[name], name + ' with a lexicon');
            all(result).forEach((item) => tripped.add(item.rule));
        });
        ['N1', 'N2', 'N6', 'N7', 'pointed-word', 'later-context', 'later-checkpoint', 'schema'].forEach((rule) => {
            assert.ok(tripped.has(rule), rule + ' is exercised by a variant');
        });
    });

    it('no rule introduced by ADR-0009 reaches a 1.0.0 document, even one holding 1.1.0 constructs', () => {
        const documents = SAMPLE_NAMES.map(sample).concat(Object.values(VARIANTS).map((build) => build()));
        const tagged = sample('my-morning');
        lesson(tagged).recipe = 'topic-conversation';
        lesson(tagged).buildsOn = ['elsewhere'];
        documents.push(tagged);
        const defective = {
            'conjugation-table': (block) => {
                block.cells[0] = { cell: '1s', formKey: 'שׁוֹאֵל' };
                block.cells[1] = { cell: 'xx', formKey: 'חוֹשֶׁבֶת' };
                block.cells[2] = { cell: 'fs', formKey: 'חוֹשֶׁבֶת' };
            },
            'verse-reading': (block) => { delete block.ref.source; },
            'personal-question': (block) => { block.prompts = []; },
            explainer: (block) => { block.sections = []; },
            'letter-intro': (block) => { block.sounds = []; }
        };
        BLOCK_TYPES.forEach((type) => {
            [false, true].forEach((broken) => {
                const plan = sample('beginner-unit-1');
                const block = fixtureBlock(type);
                if (broken) defective[type](block);
                lesson(plan, 2).blocks.push(block);
                lesson(plan, 2).recipe = 'verb-workshop';
                documents.push(plan);
            });
        });
        const verse = sample('beginner-unit-1');
        lesson(verse, 2).blocks.push(Object.assign(fixtureBlock('verse-reading'), { ref: { book: 'Genesis' } }));
        documents.push(verse);
        documents.forEach((plan) => {
            [check(plan), validatePlan(plan)].forEach((result) => {
                all(result).forEach((item) => assert.equal(NEW_RULES.includes(item.rule), false, item.rule + ' ' + item.path));
            });
        });
    });

    it('the N1, N2, and N7 changes reach only a 1.1.0 document', () => {
        const both = (plan) => ['1.0.0', '1.1.0'].map((version) => check(Object.assign(clone(plan), { schemaVersion: version })));

        const verse = sample('beginner-unit-1');
        const block = fixtureBlock('verse-reading');
        const words = ['א', 'ב', 'ג', 'ד', 'ה', 'ו'].map((letter) => letter + 'ָ');
        block.newWords = block.newWords.concat(words);
        block.verse += ' ' + words.concat(words).join(' ');
        lesson(verse, 2).blocks.push(block);
        const [verseOld, verseNew] = both(verse);
        assert.deepEqual(hits(all(verseOld), 'N1'), []);
        assert.equal(hits(verseNew.blocks, 'N1', L2 + '/newWords').length, 1);

        const focused = sample('beginner-unit-1');
        lesson(focused, 2).blocks.push(fixtureBlock('explainer'), fixtureBlock('conjugation-table'), fixtureBlock('letter-intro'));
        const [focusedOld, focusedNew] = both(focused);
        assert.deepEqual(hits(all(focusedOld), 'N7'), []);
        assert.equal(hits(focusedNew.warnings, 'N7', '/units/0').length, 1);

        const exempt = VARIANTS['beginner-unit-1-introductions-only']();
        lesson(exempt, 1).recycles = lesson(exempt, 1).recycles.slice(0, 2);
        [0, 1].forEach((index) => { lesson(exempt, index).recipe = 'guide'; });
        const [exemptOld, exemptNew] = both(exempt);
        assert.equal(hits(exemptOld.warnings, 'N2').length, 1);
        assert.equal(hits(exemptOld.warnings, 'N7').length, 1);
        assert.deepEqual(hits(all(exemptNew), 'N2'), []);
        assert.deepEqual(hits(all(exemptNew), 'N7'), []);
    });

    it('a sample read as 1.1.0 returns the same findings, because 1.1.0 only adds', () => {
        SAMPLE_NAMES.forEach((name) => {
            const plan = sample(name);
            plan.schemaVersion = '1.1.0';
            assert.deepEqual(check(plan), BASELINE.samples[name], name);
        });
        Object.keys(VARIANTS).forEach((name) => {
            const plan = VARIANTS[name]();
            plan.schemaVersion = '1.1.0';
            assert.deepEqual(shape(check(plan)), shape(BASELINE.variants[name]), name);
        });
    });
});

describe('schema 1.1.0', () => {
    const defs = SCHEMA.definitions;

    it('takes schemaVersion 1.0.0 or 1.1.0 and adds the five blocks as oneOf branches', () => {
        assert.equal(SCHEMA.$schema, 'http://json-schema.org/draft-07/schema#');
        assert.deepEqual(SCHEMA.properties.schemaVersion, { enum: ['1.0.0', '1.1.0', '1.2.0'] });
        const branches = defs.block.oneOf.map((branch) => defs[branch.$ref.slice('#/definitions/'.length)].properties.type.const);
        assert.deepEqual(branches.slice(6), BLOCK_TYPES.concat(['type-it', 'exercise']));
        assert.deepEqual(defs.v110BlockType.not.enum, BLOCK_TYPES);
        assert.deepEqual(defs.lesson.properties.recipe, { $ref: '#/definitions/recipe' });
        assert.deepEqual(defs.wordEntry.properties.group, { type: 'integer', minimum: 1, maximum: 3 });
    });

    it('every new object is closed with additionalProperties false', () => {
        ['explainerBlock', 'explainerSection', 'explainerExample', 'explainerTable', 'explainerCallout',
            'conjugationTableBlock', 'conjugationCell', 'personalQuestionBlock', 'verseReadingBlock', 'verseRef',
            'versePersonalQuestion', 'letterIntroBlock', 'letterSound', 'letterFind', 'hebrewLine'].forEach((name) => {
            assert.equal(defs[name].type, 'object', name);
            assert.equal(defs[name].additionalProperties, false, name);
        });
    });

    it('every object list item has an id, and a conjugation cell is keyed by its cell code', () => {
        ['explainerSection', 'explainerExample', 'explainerCallout', 'hebrewLine', 'letterSound', 'letterFind'].forEach((name) => {
            assert.ok(defs[name].required.includes('id'), name);
        });
        assert.deepEqual(defs.conjugationCell.required, ['cell', 'formKey']);
        assert.equal(Object.prototype.hasOwnProperty.call(defs.letterIntroBlock.properties, 'image'), false);
        assert.equal(Object.prototype.hasOwnProperty.call(defs.letterIntroBlock.properties, 'picture'), false);
    });

    it('a second draft-07 walker, the INIT-003 spike, reads the same schema', () => {
        const spike = require('../spikes/skeleton/validate');
        SAMPLE_NAMES.forEach((name) => assert.equal(spike.validatePlan(sample(name)).ok, true, name));
        BLOCK_TYPES.forEach((type) => assert.equal(spike.validatePlan(fixture(type)).ok, true, type));
        const tagged = sample('beginner-unit-1');
        lesson(tagged).recipe = 'guide';
        assert.equal(spike.validatePlan(tagged).ok, false);
        tagged.schemaVersion = '1.1.0';
        assert.equal(spike.validatePlan(tagged).ok, true);
    });
});

describe('ac-2: one fixture per new block validates; one hostile fixture per block fails with the rule id', () => {
    BLOCK_TYPES.forEach((type) => {
        it(type + ': the fixture validates', () => {
            const plan = fixture(type);
            assert.equal(plan.schemaVersion, '1.1.0');
            assert.ok(lesson(plan).blocks.some((block) => block.type === type));
            const result = check(plan);
            assert.equal(result.ok, true, JSON.stringify(result.blocks));
            assert.deepEqual(result.blocks, []);
            const expected = type === 'conjugation-table' ? ['warn N7 /units/0'] : [];
            assert.deepEqual(shape(result), expected);
        });
    });

    const HOSTILE = {
        explainer: [['html', L0 + '/blocks/0/title'], ['schema', L0 + '/blocks/0/sections/0/paragraphs'],
            ['schema', L0 + '/blocks/0/style']],
        'conjugation-table': [['html', L0 + '/blocks/1/questions/0/en'], ['schema', L0 + '/blocks/1/sentences'],
            ['schema', L0 + '/blocks/2/timer']],
        'personal-question': [['html', L0 + '/blocks/3/prompts/0/en'], ['schema', L0 + '/blocks/3/prompts'],
            ['schema', L0 + '/blocks/3/prompts/0/sampleAnswer']],
        'verse-reading': [['html', L0 + '/blocks/0/personalQuestion/en'], ['schema', L0 + '/blocks/0/questions'],
            ['schema', L0 + '/blocks/0/translation']],
        'letter-intro': [['html', L0 + '/blocks/0/name'], ['schema', L0 + '/blocks/0/findIn'],
            ['schema', L0 + '/blocks/0/picture']]
    };
    const MESSAGES = ['markup is not allowed in plan text', /^must have at most \d+ items$/, 'unknown property'];

    BLOCK_TYPES.forEach((type) => {
        it(type + ': the hostile fixture fails on markup, oversize, and an unknown property', () => {
            const result = check(fixture(type + '.hostile'));
            assert.equal(result.ok, false);
            HOSTILE[type].forEach(([rule, where], i) => {
                const found = hits(result.blocks, rule, where);
                assert.equal(found.length, 1, rule + ' ' + where);
                if (typeof MESSAGES[i] === 'string') assert.equal(found[0].message, MESSAGES[i]);
                else assert.match(found[0].message, MESSAGES[i]);
            });
        });
    });

    /** Pointers to every string in each new-type block of the fixtures, and each lesson recipe. */
    function stringFields() {
        const fields = [];
        BLOCK_TYPES.forEach((name) => {
            const plan = fixture(name);
            fields.push([name, ['units', 0, 'lessons', 0, 'recipe']]);
            lesson(plan).blocks.forEach((block, b) => {
                if (!BLOCK_TYPES.includes(block.type)) return;
                const walk = (node, segments) => {
                    if (typeof node === 'string') {
                        if (segments[segments.length - 1] !== 'type' || segments.length !== B.length + 2) fields.push([name, segments]);
                    } else if (Array.isArray(node)) {
                        node.forEach((child, i) => walk(child, segments.concat(i)));
                    } else if (node && typeof node === 'object') {
                        Object.keys(node).forEach((key) => walk(node[key], segments.concat(key)));
                    }
                };
                walk(block, B.concat(b));
            });
        });
        return fields;
    }

    it('security: markup in any string of a new block is an html block at that field', () => {
        const fields = stringFields();
        assert.ok(fields.length > 60, String(fields.length));
        fields.forEach(([name, segments]) => {
            const plan = fixture(name);
            setAt(plan, segments, '<b>x</b>');
            assert.equal(hits(check(plan).blocks, 'html', pointer(segments)).length, 1, name + ' ' + pointer(segments));
        });
    });

    it('security: every string of a new block is length-limited, so 4001 characters block at that field', () => {
        stringFields().forEach(([name, segments]) => {
            const plan = fixture(name);
            setAt(plan, segments, 'א'.repeat(4001));
            const where = pointer(segments);
            assert.ok(check(plan).blocks.some((item) => item.path === where), name + ' ' + where);
        });
    });

    const E = B.concat(0);
    const TEXT_LIMITS = [
        ['explainer', E.concat('title'), 120],
        ['explainer', E.concat('sections', 0, 'heading'), 120],
        ['explainer', E.concat('sections', 0, 'paragraphs', 0), 1500],
        ['explainer', E.concat('sections', 0, 'examples', 0, 'he'), 200],
        ['explainer', E.concat('sections', 0, 'examples', 0, 'en'), 300],
        ['explainer', E.concat('sections', 0, 'examples', 0, 'translit'), 200],
        ['explainer', E.concat('sections', 1, 'table', 'columns', 0), 200],
        ['explainer', E.concat('sections', 1, 'table', 'rows', 0, 0), 200],
        ['explainer', E.concat('sections', 1, 'callouts', 0, 'title'), 80],
        ['explainer', E.concat('sections', 1, 'callouts', 0, 'text'), 600],
        ['conjugation-table', B.concat(1, 'lemmaKey'), 60, 'א'],
        ['conjugation-table', B.concat(1, 'cells', 0, 'formKey'), 60, 'א'],
        ['verse-reading', B.concat(0, 'newWords', 0), 60, 'א']
    ];

    it('the ADR-0009 §7 text limits: the limit passes and one more character is a schema block', () => {
        TEXT_LIMITS.forEach(([name, segments, limit, letter]) => {
            const where = pointer(segments);
            const fits = fixture(name);
            setAt(fits, segments, (letter || 'x').repeat(limit));
            assert.deepEqual(hits(all(check(fits)), 'schema', where), [], where);
            const over = fixture(name);
            setAt(over, segments, (letter || 'x').repeat(limit + 1));
            const found = hits(check(over).blocks, 'schema', where);
            assert.equal(found.length, 1, where);
            assert.equal(found[0].message, 'must be at most ' + limit + ' characters');
        });
    });

    const item = (prefix) => (i, items) => Object.assign(clone(items[0]), { id: prefix + i });
    const LIST_LIMITS = [
        ['explainer', E.concat('sections'), 12, item('s')],
        ['explainer', E.concat('sections', 0, 'paragraphs'), 8, (i) => 'Paragraph ' + i + '.'],
        ['explainer', E.concat('sections', 0, 'examples'), 8, item('e')],
        ['explainer', E.concat('sections', 1, 'table', 'columns'), 8, (i) => 'Column ' + i],
        ['explainer', E.concat('sections', 1, 'table', 'rows'), 12, (i, items) => clone(items[0])],
        ['explainer', E.concat('sections', 1, 'table', 'rows', 0), 8, (i) => 'cell ' + i],
        ['explainer', E.concat('sections', 1, 'callouts'), 6, item('c')],
        ['conjugation-table', B.concat(1, 'sentences'), 3, item('s')],
        ['conjugation-table', B.concat(1, 'questions'), 4, item('q')],
        ['personal-question', B.concat(3, 'prompts'), 6, item('p')],
        ['verse-reading', B.concat(0, 'questions'), 4, item('q')]
    ];

    it('the ADR-0009 §7 list limits: the limit passes and one more item is a schema block', () => {
        LIST_LIMITS.forEach(([name, segments, limit, make]) => {
            const where = pointer(segments);
            const grow = (n) => {
                const plan = fixture(name);
                const items = at(plan, segments);
                while (items.length < n) items.push(make(items.length + 1, items));
                return plan;
            };
            assert.deepEqual(hits(all(check(grow(limit))), 'schema', where), [], where);
            const found = hits(check(grow(limit + 1)).blocks, 'schema', where);
            assert.equal(found.length, 1, where);
            assert.equal(found[0].message, 'must have at most ' + limit + ' items');
        });
    });

    it('a root is two to four Hebrew letters joined by dots; numbers and groups keep their ranges', () => {
        const rootAt = E.concat('sections', 0, 'examples', 0, 'root');
        ['ח.ש.ב', 'ש.א.ל', 'ג.ל.ג.ל'].forEach((root) => {
            const plan = fixture('explainer');
            setAt(plan, rootAt, root);
            assert.equal(check(plan).ok, true, root);
        });
        ['ח', 'חשב', 'ח.ש.ב.ת.ל', 'a.b.c', 'ח.ש.'].forEach((root) => {
            const plan = fixture('explainer');
            setAt(plan, rootAt, root);
            assert.equal(hits(check(plan).blocks, 'schema', pointer(rootAt)).length, 1, root);
        });
        [['chapter', 0], ['chapter', 151], ['from', 0], ['to', 177], ['chapter', 1.5], ['from', '1']].forEach(([field, value]) => {
            const plan = fixture('verse-reading');
            lesson(plan).blocks[0].ref[field] = value;
            assert.equal(hits(check(plan).blocks, 'schema', L0 + '/blocks/0/ref/' + field).length, 1, field + ' ' + value);
        });
        [0, 4, 1.5, '1'].forEach((group) => {
            const plan = fixture('personal-question');
            lesson(plan).blocks[0].words[0].group = group;
            assert.equal(hits(check(plan).blocks, 'schema', L0 + '/blocks/0/words/0/group').length, 1, String(group));
        });
    });
});

describe('security: personal-question stores no answer, verse-reading stores no translation', () => {
    it('the block has no field that holds an answer', () => {
        const block = SCHEMA.definitions.personalQuestionBlock;
        assert.deepEqual(Object.keys(block.properties).sort(), ['answer', 'id', 'prompts', 'type']);
        assert.deepEqual(block.properties.answer, { enum: ['speak', 'type', 'either'] });
        assert.deepEqual(Object.keys(SCHEMA.definitions.hebrewLine.properties).sort(), ['en', 'he', 'id']);
    });

    it('an answer written anywhere in the block is refused', () => {
        const Q = L0 + '/blocks/3';
        [
            [(block) => { block.answer = 'כֵּן, יֵשׁ לִי כֶּלֶב.'; }, Q + '/answer'],
            [(block) => { block.answers = ['כֵּן']; }, Q + '/answers'],
            [(block) => { block.response = 'כֵּן'; }, Q + '/response'],
            [(block) => { block.prompts[0].answer = 'כֵּן'; }, Q + '/prompts/0/answer'],
            [(block) => { block.prompts[0].sampleAnswer = 'כֵּן'; }, Q + '/prompts/0/sampleAnswer']
        ].forEach(([edit, where]) => {
            const plan = fixture('personal-question');
            edit(lesson(plan).blocks[3]);
            const result = check(plan);
            assert.equal(result.ok, false, where);
            assert.equal(hits(result.blocks, 'schema', where).length, 1, where);
        });
    });

    it('a verse-reading has no field for a translation of the verse', () => {
        assert.deepEqual(Object.keys(SCHEMA.definitions.verseReadingBlock.properties).sort(),
            ['id', 'newWords', 'personalQuestion', 'questions', 'ref', 'type', 'verse']);
        ['translation', 'en', 'gloss'].forEach((field) => {
            const plan = fixture('verse-reading');
            lesson(plan).blocks[0][field] = 'In the beginning';
            assert.equal(hits(check(plan).blocks, 'schema', L0 + '/blocks/0/' + field).length, 1, field);
        });
    });
});

describe('ac-3: a conjugation table must agree with the lexicon', () => {
    const PRESENT = L0 + '/blocks/1/cells/';
    const PAST = L0 + '/blocks/2/cells/';

    function withCell(table, index, cell) {
        const plan = fixture('conjugation-table');
        lesson(plan).blocks[table].cells[index] = cell;
        return check(plan);
    }

    it('a form of another lemma blocks with N13', () => {
        const result = withCell(1, 0, { cell: 'ms', formKey: 'שׁוֹאֵל' });
        assert.equal(result.ok, false);
        const found = hits(result.blocks, 'N13', PRESENT + '0/formKey');
        assert.equal(found.length, 1);
        assert.equal(found[0].message, 'formKey names a form of another lemma');
        assert.equal(found[0].severity, 'block');
    });

    it('an unverified form warns with N14 and does not block; a verified form does not warn', () => {
        const result = withCell(2, 3, { cell: '3fs', formKey: 'חָשְׁבָה' });
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
        const found = hits(result.warnings, 'N14', PAST + '3/formKey');
        assert.equal(found.length, 1);
        assert.equal(found[0].message, 'this form is not verified');
        assert.deepEqual(hits(all(check(fixture('conjugation-table'))), 'N14'), []);
    });

    it('a key naming no stored form, a lemma, another tense, or another cell blocks with N13', () => {
        [
            [{ cell: 'ms', formKey: 'חוֹשֶׁבֶתֶת' }, 'formKey names no stored form'],
            [{ cell: 'ms', formKey: 'לַחְשֹׁב' }, 'formKey names no stored form'],
            [{ cell: 'ms', formKey: 'חָשַׁב' }, 'formKey names a form of another tense'],
            [{ cell: 'ms', formKey: 'חוֹשֶׁבֶת' }, 'formKey names a form of another cell']
        ].forEach(([cell, message]) => {
            const found = hits(withCell(1, 0, cell).blocks, 'N13', PRESENT + '0/formKey');
            assert.equal(found.length, 1, cell.formKey);
            assert.equal(found[0].message, message, cell.formKey);
        });
    });

    it('a cell appears at most once, and only in its own tense', () => {
        const twice = withCell(1, 1, { cell: 'ms', formKey: 'חוֹשֵׁב' });
        assert.equal(hits(twice.blocks, 'N13', PRESENT + '1/cell')[0].message, 'this cell appears more than once in the table');
        const outside = withCell(1, 0, { cell: '1s', formKey: 'חָשַׁבְתִּי' });
        assert.equal(hits(outside.blocks, 'N13', PRESENT + '0/cell')[0].message, 'this cell does not belong to the table tense');
        const pastOutside = withCell(2, 0, { cell: 'ms', formKey: 'חוֹשֵׁב' });
        assert.equal(hits(pastOutside.blocks, 'N13', PAST + '0/cell').length, 1);
        [{ cell: 'xx', formKey: 'חוֹשֵׁב' }, { cell: 3, formKey: 'חוֹשֵׁב' }].forEach((cell) => {
            const notACode = withCell(1, 0, cell);
            const found = hits(notACode.blocks, 'schema', PRESENT + '0/cell');
            assert.equal(found.length, 1, String(cell.cell));
            assert.equal(found[0].message, 'not a present or past cell code');
            assert.deepEqual(hits(notACode.blocks, 'N13', PRESENT + '0/cell'), [], String(cell.cell));
        });
    });

    it('without a lexicon every form is an N13 block: the validator fails closed', () => {
        const result = validatePlan(fixture('conjugation-table'));
        assert.equal(result.ok, false);
        const found = hits(result.blocks, 'N13');
        assert.deepEqual(found.map((item) => item.path), [0, 1, 2, 3].map((i) => PRESENT + i + '/formKey')
            .concat([0, 1, 2].map((i) => PAST + i + '/formKey')));
        found.forEach((item) => assert.equal(item.message, 'no lexicon was given, so this form cannot be checked'));
        assert.equal(validatePlan(fixture('conjugation-table'), { lexicon: {} }).ok, false);
    });

    it('keys match by ADR-0001 exactKey, so marks out of canonical order find the NFC row', () => {
        const unordered = (key) => key.normalize('NFC').replace(/\u05E9([\u05B0-\u05BB])\u05C1/, '\u05E9\u05C1$1');
        const form = unordered('חוֹשֵׁב');
        const lemma = unordered('לַחְשֹׁב');
        assert.notEqual(form, form.normalize('NFC'));
        assert.notEqual(lemma, lemma.normalize('NFC'));
        const plan = fixture('conjugation-table');
        lesson(plan).blocks[1].cells[0].formKey = ' ' + form + ' ';
        lesson(plan).blocks[1].lemmaKey = lemma;
        assert.deepEqual(shape(check(plan)), ['warn N7 /units/0']);
    });

    it('each key is looked up once per validation, and a key the schema refuses is not looked up', () => {
        const lexicon = memoryLexicon(LEXICON_ENTRIES);
        const plan = fixture('conjugation-table');
        lesson(plan).blocks.push(clone(lesson(plan).blocks[1]));
        lesson(plan).blocks[2].cells.push({ cell: '3p', formKey: 'א'.repeat(61) });
        validatePlan(plan, { lexicon });
        assert.equal(lexicon.calls.length, 7);
        assert.equal(new Set(lexicon.calls).size, 7);
    });
});

describe('ac-4: each warn and inform rule of ADR-0009 §7 has a positive and a negative fixture', () => {
    it('N14 warn: an unverified form warns; verified forms do not', () => {
        const plan = fixture('conjugation-table');
        lesson(plan).blocks[2].cells.push({ cell: '3fs', formKey: 'חָשְׁבָה' });
        assert.equal(hits(check(plan).warnings, 'N14').length, 1);
        assert.deepEqual(hits(all(check(fixture('conjugation-table'))), 'N14'), []);
    });

    it('N15 warn: a recipe lesson that builds on another starts with a spaced-review-checkpoint', () => {
        const positive = fixture('personal-question');
        lesson(positive).buildsOn = ['earlier-lesson'];
        const found = hits(check(positive).warnings, 'N15');
        assert.deepEqual(found.map((item) => item.path), [L0 + '/blocks/0']);

        const reviewFirst = clone(positive);
        lesson(reviewFirst).blocks.unshift({
            id: 'review', type: 'spaced-review-checkpoint', wordKeys: ['כֶּלֶב'], fromLessons: ['earlier-lesson']
        });
        const guide = clone(positive);
        lesson(guide).recipe = 'guide';
        const noRecipe = clone(positive);
        delete lesson(noRecipe).recipe;
        [reviewFirst, guide, noRecipe, fixture('personal-question')].forEach((plan, i) => {
            assert.deepEqual(hits(all(check(plan)), 'N15'), [], String(i));
        });
    });

    it('N16 inform: topic-conversation, verb-workshop, and verse-reading lessons ask a personal question', () => {
        const workshop = fixture('conjugation-table');
        lesson(workshop).blocks.pop();
        const topic = fixture('personal-question');
        lesson(topic).blocks.pop();
        const verse = fixture('verse-reading');
        delete lesson(verse).blocks[0].personalQuestion;
        [workshop, topic, verse].forEach((plan) => {
            const found = hits(check(plan).informs, 'N16');
            assert.deepEqual(found.map((item) => item.path), [L0 + '/blocks'], lesson(plan).recipe);
        });

        const verseWithBlock = clone(verse);
        lesson(verseWithBlock).blocks.push(fixtureBlock('personal-question'));
        const worksheet = clone(topic);
        lesson(worksheet).recipe = 'review-worksheet';
        const noRecipe = clone(workshop);
        delete lesson(noRecipe).recipe;
        [fixture('conjugation-table'), fixture('personal-question'), fixture('verse-reading'), verseWithBlock,
            worksheet, noRecipe].forEach((plan, i) => {
            assert.deepEqual(hits(all(check(plan)), 'N16'), [], String(i));
        });
    });

    function lessonsWith(recipesPerUnit) {
        const plan = fixture('explainer');
        const base = lesson(plan);
        const unitBase = plan.units[0];
        plan.units = recipesPerUnit.map((recipes, u) => {
            const unit = clone(unitBase);
            unit.id = 'unit-' + u;
            unit.lessons = recipes.map((recipe, l) => {
                const item = clone(base);
                item.id = 'lesson-' + u + '-' + l;
                if (recipe) item.recipe = recipe;
                else delete item.recipe;
                return item;
            });
            return unit;
        });
        return plan;
    }

    it('N17 inform: three lessons in a row in one unit share a recipe, reported once per run', () => {
        const three = hits(check(lessonsWith([['guide', 'guide', 'guide']])).informs, 'N17');
        assert.deepEqual(three.map((item) => item.path), ['/units/0/lessons/2/recipe']);
        const six = hits(check(lessonsWith([['guide', 'guide', 'guide', 'guide', 'guide', 'guide']])).informs, 'N17');
        assert.deepEqual(six.map((item) => item.path), ['/units/0/lessons/2/recipe']);
        [
            [['guide', 'guide']],
            [['guide', 'review-worksheet', 'guide']],
            [['guide', null, 'guide']],
            [['guide', 'guide'], ['guide']],
            [[null, null, null]]
        ].forEach((units, i) => {
            assert.deepEqual(hits(all(check(lessonsWith(units))), 'N17'), [], String(i));
        });
    });

    it('N18 warn: a topic-conversation lesson over 8 new words, or with an introduction group over 5', () => {
        const nine = hits(check(topicWithWords(9)).warnings, 'N18');
        assert.deepEqual(nine.map((item) => item.path), [L0 + '/newWords']);

        const group = fixture('personal-question');
        lesson(group).blocks[0].words.forEach((word) => { word.group = 1; });
        const grouped = hits(check(group).warnings, 'N18');
        assert.deepEqual(grouped.map((item) => item.path), [L0 + '/blocks/0/words/5']);

        const five = fixture('personal-question');
        lesson(five).blocks[0].words.forEach((word, i) => { word.group = i < 5 ? 1 : 2; });
        const worksheet = clone(group);
        lesson(worksheet).recipe = 'review-worksheet';
        [topicWithWords(8), five, worksheet, fixture('personal-question')].forEach((plan, i) => {
            assert.deepEqual(hits(all(check(plan)), 'N18'), [], String(i));
        });
    });

    it('N13 and N19 blocks have negatives too: the fixtures carry neither', () => {
        BLOCK_TYPES.forEach((type) => {
            assert.deepEqual(hits(all(check(fixture(type))), 'N13'), [], type);
            assert.deepEqual(hits(all(check(fixture(type))), 'N19'), [], type);
        });
    });
});

describe('N19: a verse-reading names its source edition', () => {
    const SOURCE = L0 + '/blocks/0/ref/source';

    it('a missing, empty, or blank source is an N19 block at ref.source', () => {
        [
            (ref) => { delete ref.source; },
            (ref) => { ref.source = ''; },
            (ref) => { ref.source = '   '; }
        ].forEach((edit, i) => {
            const plan = fixture('verse-reading');
            edit(lesson(plan).blocks[0].ref);
            const result = check(plan);
            assert.equal(result.ok, false, String(i));
            const found = hits(result.blocks, 'N19', SOURCE);
            assert.equal(found.length, 1, String(i));
            assert.equal(found[0].message, 'a verse-reading needs a non-empty ref.source naming its edition');
            assert.deepEqual(hits(result.blocks, 'schema', SOURCE), [], String(i));
        });
    });

    it('a source that is not text is a schema block as well as N19', () => {
        [42, { not: ['a', 'string'] }, ['Leningrad'], null].forEach((source) => {
            const plan = fixture('verse-reading');
            lesson(plan).blocks[0].ref.source = source;
            const result = check(plan);
            assert.equal(hits(result.blocks, 'schema', SOURCE).length, 1, JSON.stringify(source));
            assert.equal(hits(result.blocks, 'N19', SOURCE).length, 1, JSON.stringify(source));
        });
    });

    it('a verse-reading with no ref at all fails the schema and N19', () => {
        const plan = fixture('verse-reading');
        delete lesson(plan).blocks[0].ref;
        const result = check(plan);
        assert.equal(hits(result.blocks, 'schema', L0 + '/blocks/0/ref').length, 1);
        assert.equal(hits(result.blocks, 'N19', SOURCE).length, 1);
    });
});

describe('ADR-0009 §7 changes to N1, N2, N6, N7, and pointed-word', () => {
    it('N1 counts verse-reading newWords, and letter-intro findIn words are not new words', () => {
        const plan = fixture('verse-reading');
        const verse = lesson(plan).blocks[0];
        const words = ['א', 'ב', 'ג', 'ד', 'ה', 'ו'].map((letter) => letter + 'ָ');
        verse.newWords = verse.newWords.concat(words);
        verse.verse += ' ' + words.concat(words).join(' ');
        const result = check(plan);
        const found = hits(result.blocks, 'N1', L0 + '/newWords');
        assert.equal(found.length, 1);
        assert.equal(found[0].message, '9 new words; N1 allows 8');

        const letters = fixture('letter-intro');
        const block = lesson(letters).blocks[0];
        while (block.findIn.length < 12) block.findIn.push({ id: 'f' + (block.findIn.length + 1), word: 'בית', contains: true });
        assert.deepEqual(shape(check(letters)), []);
    });

    it('N6 applies to verse-reading newWords, and a verse and its questions count', () => {
        assert.deepEqual(hits(all(check(fixture('verse-reading'))), 'N6'), []);
        const plan = fixture('verse-reading');
        lesson(plan).blocks[0].questions.pop();
        const found = hits(check(plan).warnings, 'N6');
        assert.deepEqual(found.map((item) => item.path), [L0 + '/blocks/0/newWords/1']);
        assert.equal(found[0].message, 'appears 1 time(s) in this lesson; N6 asks for 2');
    });

    it('pointed-word applies to verse-reading newWords; meaning does not', () => {
        const plan = fixture('verse-reading');
        const verse = lesson(plan).blocks[0];
        verse.newWords[0] = 'ברא';
        verse.verse += ' ברא';
        const result = check(plan);
        assert.equal(hits(result.warnings, 'pointed-word', L0 + '/blocks/0/newWords/0').length, 1);
        assert.deepEqual(hits(all(result), 'meaning'), []);
    });

    it('N2 does not apply to a guide', () => {
        const guide = fixture('explainer');
        lesson(guide).buildsOn = ['earlier-lesson'];
        assert.deepEqual(hits(all(check(guide)), 'N2'), []);
        const worksheet = clone(guide);
        lesson(worksheet).recipe = 'review-worksheet';
        assert.equal(hits(check(worksheet).warnings, 'N2', L0 + '/recycles').length, 1);
    });

    it('N7 counts introduction, explainer, conjugation-table, and letter-intro as language-focused', () => {
        const block = (type) => (type === 'introduction'
            ? { id: 'introduction', type, words: [{ id: '1', word: 'שָׁלוֹם', meaning: 'hello' }] }
            : Object.assign(fixtureBlock(type), { id: type }));
        ['introduction', 'explainer', 'conjugation-table', 'letter-intro'].forEach((type) => {
            const found = hits(check(oneLesson([block(type), dialogue('d1')])).warnings, 'N7', '/units/0');
            assert.equal(found.length, 1, type);
            assert.equal(found[0].message, '1 of 2 blocks are language-focused; N7 allows one in 4');
            assert.deepEqual(hits(all(check(oneLesson([block(type), dialogue('d1'), dialogue('d2'), dialogue('d3')]))), 'N7'),
                [], type);
        });
        ['personal-question', 'verse-reading'].forEach((type) => {
            assert.deepEqual(hits(all(check(oneLesson([block(type), dialogue('d1')]))), 'N7'), [], type);
        });
        assert.equal(hits(check(oneLesson([block('explainer')], 'verb-workshop')).warnings, 'N7').length, 1);
    });
});

describe('ac-6: 1.0.0 refuses 1.1.0 constructs; letter-intro has no picture; guide and letters-and-vowels', () => {
    it('a 1.0.0 document that contains any of the five blocks is rejected, and the same block is fine in 1.1.0', () => {
        BLOCK_TYPES.forEach((type) => {
            const plan = sample('beginner-unit-1');
            lesson(plan, 2).blocks.push(fixtureBlock(type));
            const result = check(plan);
            assert.equal(result.ok, false, type);
            const found = hits(result.blocks, 'schema');
            assert.deepEqual(found.map((item) => item.path), [L2 + '/blocks/3/type'], type);
            assert.equal(found[0].message, 'this block type needs schemaVersion 1.1.0');
            plan.schemaVersion = '1.1.0';
            assert.deepEqual(hits(all(check(plan)), 'schema'), [], type);
        });
    });

    it('a 1.0.0 document with a recipe or a group is rejected, and not guessed at', () => {
        const recipe = sample('beginner-unit-1');
        lesson(recipe).recipe = 'topic-conversation';
        const recipeFound = hits(check(recipe).blocks, 'schema');
        assert.deepEqual(recipeFound.map((item) => item.path + ' ' + item.message),
            [L0 + '/recipe recipe needs schemaVersion 1.1.0']);

        const group = sample('beginner-unit-1');
        lesson(group).blocks[0].words[0].group = 1;
        const groupFound = hits(check(group).blocks, 'schema');
        assert.deepEqual(groupFound.map((item) => item.path + ' ' + item.message),
            [L0 + '/blocks/0/words/0/group group needs schemaVersion 1.1.0']);

        recipe.schemaVersion = '1.1.0';
        group.schemaVersion = '1.1.0';
        assert.deepEqual(hits(all(check(recipe)), 'schema'), []);
        assert.deepEqual(hits(all(check(group)), 'schema'), []);
    });

    it('a letter-intro with a picture field fails', () => {
        const picture = { mediaId: 'bet-house', alt: 'a house' };
        const LETTER = L0 + '/blocks/0';
        [
            [(block) => { block.image = picture; }, 'image-placement', LETTER + '/image'],
            [(block) => { block.picture = picture; }, 'schema', LETTER + '/picture'],
            [(block) => { block.mediaId = 'bet-house'; }, 'image-placement', LETTER + '/mediaId'],
            [(block) => { block.findIn[0].image = picture; }, 'image-placement', LETTER + '/findIn/0/image'],
            [(block) => { block.sounds[0].image = picture; }, 'image-placement', LETTER + '/sounds/0/image']
        ].forEach(([edit, rule, where]) => {
            const plan = fixture('letter-intro');
            edit(lesson(plan).blocks[0]);
            const result = check(plan);
            assert.equal(result.ok, false, where);
            assert.equal(hits(result.blocks, rule, where).length, 1, where);
        });
    });

    it('a guide lesson triggers neither N2 nor N7, and the same lesson under another recipe triggers both', () => {
        const guide = fixture('explainer');
        lesson(guide).buildsOn = ['earlier-lesson'];
        assert.deepEqual(lesson(guide).recycles, []);
        assert.deepEqual(hits(all(check(guide)), 'N2'), []);
        assert.deepEqual(hits(all(check(guide)), 'N7'), []);
        const control = clone(guide);
        lesson(control).recipe = 'review-worksheet';
        assert.equal(hits(check(control).warnings, 'N2').length, 1);
        assert.equal(hits(check(control).warnings, 'N7').length, 1);
    });

    it('a letters-and-vowels lesson triggers neither N2 nor N7; N2 reaches one that builds on another (ADR-0009 exempts only guide)', () => {
        const letters = fixture('letter-intro');
        lesson(letters).blocks.push(Object.assign(fixtureBlock('letter-intro'), { id: 'gimel' }));
        assert.deepEqual(hits(all(check(letters)), 'N2'), []);
        assert.deepEqual(hits(all(check(letters)), 'N7'), []);
        const later = clone(letters);
        lesson(later).buildsOn = ['earlier-letters'];
        assert.deepEqual(hits(all(check(later)), 'N7'), []);
        assert.equal(hits(check(later).warnings, 'N2', L0 + '/recycles').length, 1);
        const control = clone(letters);
        lesson(control).recipe = 'review-worksheet';
        assert.equal(hits(check(control).warnings, 'N7').length, 1);
    });
});

describe('ac-7: topic-conversation thresholds', () => {
    it('9 new words warn, 15 still warn, 16 block, all as N18 and never N1', () => {
        [[9, 'warn'], [15, 'warn'], [16, 'block']].forEach(([count, severity]) => {
            const result = check(topicWithWords(count));
            const found = hits(all(result), 'N18', L0 + '/newWords');
            assert.equal(found.length, 1, String(count));
            assert.equal(found[0].severity, severity, String(count));
            assert.deepEqual(hits(all(result), 'N1'), [], String(count));
            assert.equal(result.ok, severity !== 'block', String(count));
        });
        assert.deepEqual(hits(all(check(topicWithWords(8))), 'N18'), []);
    });

    it('an introduction group of 6 words warns', () => {
        const plan = fixture('personal-question');
        lesson(plan).blocks[0].words.forEach((word) => { word.group = 2; });
        const found = hits(check(plan).warnings, 'N18');
        assert.equal(found.length, 1);
        assert.equal(found[0].message, 'introduction group 2 has more than 5 words');
    });

    it('migration.legacy keeps N1\'s warn-only rule for this recipe', () => {
        const plan = topicWithWords(16);
        plan.migration = { legacy: true, sourceWorksheet: 'worksheets/example/worksheet.js', activityOrder: [] };
        const found = hits(all(check(plan)), 'N18', L0 + '/newWords');
        assert.equal(found.length, 1);
        assert.equal(found[0].severity, 'warn');
        assert.match(found[0].message, /migration\.legacy: warn only/);
    });

    it('a lesson of another recipe, or with none, keeps the ADR-0003 N1 thresholds', () => {
        ['review-worksheet', 'verb-workshop', null].forEach((recipe) => {
            const nine = topicWithWords(9);
            if (recipe) lesson(nine).recipe = recipe;
            else delete lesson(nine).recipe;
            const result = check(nine);
            const found = hits(result.blocks, 'N1', L0 + '/newWords');
            assert.equal(found.length, 1, String(recipe));
            assert.equal(found[0].message, '9 new words; N1 allows 8');
            assert.deepEqual(hits(all(result), 'N18'), [], String(recipe));

            const eight = topicWithWords(8);
            if (recipe) lesson(eight).recipe = recipe;
            else delete lesson(eight).recipe;
            assert.deepEqual(hits(all(check(eight)), 'N1'), [], String(recipe));
        });
    });
});

describe('the registry matches ADR-0009 §7 one for one', () => {
    const adr = fs.readFileSync(ADR_0009, 'utf8');
    const lines = adr.split(/\r?\n/);
    const rows = lines.filter((line) => /^\| N\d+ \|/.test(line)).map((line) => line.split('|').map((cell) => cell.trim()));
    const byId = Object.fromEntries(rows.map((cells) => [cells[1], cells]));
    const bold = (cell) => (cell.match(/\*\*([^*]+)\*\*/g) || []).map((text) => text.replace(/\*\*/g, ''));
    const code = (text) => (text.match(/`([^`]+)`/g) || []).map((value) => value.slice(1, -1));
    const numbers = (text) => (text.match(/\d+/g) || []).map(Number);
    const between = (start, end) => {
        const from = adr.indexOf(start);
        const to = adr.indexOf(end, from);
        assert.ok(from > 0 && to > from, start);
        return adr.slice(from, to).split(/\r?\n/);
    };
    const bullet = (name) => lines.find((line) => line.startsWith('- **' + name + '**'));

    it('N13 to N19 follow N1 to N12 in the registry, in the ADR order', () => {
        const ids = rows.map((cells) => cells[1]);
        assert.deepEqual(ids, NEW_RULES);
        assert.deepEqual(Object.keys(RULES).filter((id) => /^N\d+$/.test(id)).slice(12, 19), ids);
    });

    it('each rule carries the ADR severity', () => {
        rows.forEach(([, id, , severity]) => {
            const named = bold(severity).filter((word) => /^(Block|Warn|Inform)$/.test(word)).map((word) => word.toLowerCase());
            assert.equal(RULES[id].severity, named[0], id);
            if (named[1]) assert.equal(RULES[id].blockSeverity, named[1], id);
        });
        assert.equal(RULES.N18.blockSeverity, 'block');
    });

    it('each figure and name in a rule equals the registry row', () => {
        assert.match(byId.N17[2], /^Three lessons in a row/);
        assert.equal(RULES.N17.runLength, 3);
        assert.deepEqual(numbers(byId.N18[2]), [RULES.N18.warnAbove, RULES.N18.maxGroupWords]);
        assert.deepEqual(numbers(byId.N18[3]), [RULES.N18.warnAbove + 1, RULES.N18.blockAbove, RULES.N18.maxGroupWords,
            RULES.N18.blockAbove]);
        assert.deepEqual(code(byId.N18[2]), [RULES.N18.recipe]);
        assert.deepEqual(code(byId.N16[2].split('contains')[0]), RULES.N16.recipes);
        assert.deepEqual(byId.N15[2].match(/other than `([^`]+)`/).slice(1), RULES.N15.exemptRecipes);
        assert.equal(byId.N15[2].match(/starts with a `([^`]+)`/)[1], RULES.N15.firstBlock);
    });

    it('the changes to N1, N2, and N7 are the registry rows', () => {
        const n1 = bullet('N1');
        assert.deepEqual(code(n1), [RULES.N18.recipe, 'migration.legacy']);
        assert.deepEqual(numbers(n1.split('For that recipe')[1]).slice(0, 2), [RULES.N18.blockAbove, RULES.N18.warnAbove]);
        assert.deepEqual(code(bullet('N2')), RULES.N2.exemptRecipes);
        const [focused, rest] = bullet('N7').split('language-focused.');
        assert.deepEqual(code(focused), RULES.N7.languageFocused);
        assert.deepEqual(code(rest.split('left out')[0]), RULES.N7.excludedRecipes);
    });

    it('the schema recipes and block types are the ADR tables, in order', () => {
        const recipes = between('| Recipe | Recommended blocks |', 'A speaking task about a picture')
            .filter((line) => line.startsWith('| `')).map((line) => code(line.split('|')[1])[0]);
        assert.deepEqual(SCHEMA.definitions.recipe.enum, recipes);
        const blocks = between('| Block | Purpose |', '**Fields.**')
            .filter((line) => line.startsWith('| `')).map((line) => code(line.split('|')[1])[0]);
        assert.deepEqual(blocks, BLOCK_TYPES);
        assert.deepEqual(SCHEMA.definitions.v110BlockType.not.enum, blocks);
    });
});

function freshModules(dataDir) {
    process.env.HEBREW_ROOT = ROOT;
    process.env.DATA_DIR = dataDir;
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/audit',
        '../server/db/repos/lexicon',
        '../server/db/repos/media',
        '../server/vocab-index',
        '../server/lesson-data',
        '../server/lexicon/validate',
        '../server/lexicon/service',
        '../server/plans/store',
        '../server/plans/validate',
        '../server/plans/ops',
        '../server/plans/publish'
    ];
    for (const file of fs.readdirSync(path.join(ROOT, 'server', 'db', 'migrations'))) {
        if (/^\d{3,}-.+\.js$/.test(file)) names.push('../server/db/migrations/' + file);
    }
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    return {
        db,
        store: require('../server/plans/store'),
        ops: require('../server/plans/ops'),
        publish: require('../server/plans/publish'),
        lexicon: require('../server/lexicon/service')
    };
}

describe('ac-5: part operations in ops.js, and schemaVersion', { concurrency: false }, () => {
    const RECALL = 'units/unit-1/lessons/cafe-recall';
    const LISTEN = 'units/unit-1/lessons/cafe-order/blocks/listen';
    const saved = {};
    let tmp;
    let db;
    let store;
    let ops;
    let publishPlan;

    before(() => {
        ['DATA_DIR', 'LESSONS_DIR', 'HEBREW_ROOT'].forEach((name) => { saved[name] = process.env[name]; });
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-1-1-'));
        process.env.LESSONS_DIR = path.join(tmp, 'lessons');
        fs.mkdirSync(process.env.LESSONS_DIR);
        const modules = freshModules(path.join(tmp, 'data'));
        ({ db, store, ops } = modules);
        // INIT-008/SPEC-002: every write takes a context; these calls add one and nothing else.
        ops = withCtx(ops);
        publishPlan = publishWithCtx(modules.publish.publishPlan);
        db.open();
        db.migrate();
        const entries = LEXICON_ENTRIES.map((entry) => {
            const row = Object.assign({}, entry);
            delete row.verified;
            return row;
        });
        const written = modules.lexicon.upsert(entries, { actor: 'admin:example-admin', expectedRev: 0 });
        assert.equal(written.ok, true, JSON.stringify(written.errors));
        const verified = modules.lexicon.setVerified(
            LEXICON_ENTRIES.filter((entry) => entry.verified).map((entry) => entry.key), true,
            { actor: 'admin:example-admin' });
        assert.equal(verified.ok, true);
    });

    after(() => {
        if (db) db.close();
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        Object.keys(saved).forEach((name) => {
            if (saved[name] === undefined) delete process.env[name];
            else process.env[name] = saved[name];
        });
    });

    beforeEach(() => {
        db.run('DELETE FROM plans');
    });

    function seed(plan) {
        store.saveWorkingCopy({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return plan;
    }

    function watch() {
        const calls = { saveWorkingCopy: 0, saveSnapshot: 0 };
        return {
            calls,
            loadWorkingCopy: (id) => store.loadWorkingCopy(id),
            saveWorkingCopy: (input) => {
                calls.saveWorkingCopy += 1;
                return store.saveWorkingCopy(input);
            },
            loadSnapshot: (id) => store.loadSnapshot(id),
            saveSnapshot: (id, snapshot) => {
                calls.saveSnapshot += 1;
                return store.saveSnapshot(id, snapshot);
            }
        };
    }

    function stored(id) {
        const row = store.loadWorkingCopy(id);
        return { row, plan: JSON.parse(row.document) };
    }

    BLOCK_TYPES.forEach((type) => {
        it('insertBlock of ' + type + ' into a 1.0.0 plan succeeds and sets schemaVersion 1.1.0 in the same revision', () => {
            const before = seed(sample('beginner-unit-1'));
            const block = fixtureBlock(type);
            const result = ops.insertBlock(store, before.id, { path: RECALL, block, expectedRevision: 1 });
            assert.equal(result.error, undefined, JSON.stringify(result));
            assert.equal(result.revision, 2);
            const expected = clone(before);
            lesson(expected, 2).blocks.push(block);
            expected.revision = 2;
            expected.status = 'draft';
            expected.schemaVersion = '1.1.0';
            assert.deepEqual(result.plan, expected);
            assert.deepEqual(result.validation.blocks, []);
            const after = stored(before.id);
            assert.equal(after.row.revision, 2);
            assert.deepEqual(after.plan, expected);
        });

        it('insertBlock of ' + type + ' into a 1.1.0 plan bumps the revision and changes nothing else', () => {
            const plan = sample('beginner-unit-1');
            plan.schemaVersion = '1.1.0';
            const before = seed(plan);
            const block = fixtureBlock(type);
            const result = ops.insertBlock(store, before.id, { path: RECALL, index: 0, block, expectedRevision: 1 });
            assert.equal(result.error, undefined, JSON.stringify(result));
            const expected = clone(before);
            lesson(expected, 2).blocks.unshift(block);
            expected.revision = 2;
            expected.status = 'draft';
            assert.deepEqual(result.plan, expected);
            assert.equal(result.plan.schemaVersion, '1.1.0');
            assert.deepEqual(result.validation.blocks, []);
        });
    });

    it('replacePart with a new block, and setField of a recipe or a group, also raise schemaVersion', () => {
        const before = seed(sample('beginner-unit-1'));
        const block = fixtureBlock('personal-question');
        // INIT-008/SPEC-002: a replacement keeps the id of the part it replaces.
        block.id = 'listen';
        const replaced = ops.replacePart(store, before.id, { path: LISTEN, value: block, expectedRevision: 1 });
        assert.equal(replaced.error, undefined, JSON.stringify(replaced));
        const expected = clone(before);
        lesson(expected).blocks[2] = block;
        expected.revision = 2;
        expected.status = 'draft';
        expected.schemaVersion = '1.1.0';
        assert.deepEqual(replaced.plan, expected);

        [
            { path: RECALL, field: 'recipe', value: 'review-worksheet' },
            { path: 'units/unit-1/lessons/cafe-order/blocks/introduction/words/1', field: 'group', value: 1 }
        ].forEach((input) => {
            db.run('DELETE FROM plans');
            seed(sample('beginner-unit-1'));
            const result = ops.setField(store, 'beginner-unit-1', Object.assign({ expectedRevision: 1 }, input));
            assert.equal(result.error, undefined, JSON.stringify(result));
            assert.equal(result.revision, 2, input.field);
            assert.equal(result.plan.schemaVersion, '1.1.0', input.field);
        });
    });

    it('a plan never returns to 1.0.0', () => {
        const original = seed(sample('beginner-unit-1'));
        const inserted = ops.insertBlock(store, original.id, { path: RECALL, block: fixtureBlock('explainer'), expectedRevision: 1 });
        assert.equal(inserted.plan.schemaVersion, '1.1.0');
        const removed = ops.removeBlock(store, original.id, { path: RECALL + '/blocks/explainer', expectedRevision: 2 });
        assert.equal(removed.error, undefined, JSON.stringify(removed));
        assert.equal(removed.revision, 3);
        assert.equal(removed.plan.schemaVersion, '1.1.0');

        const watched = watch();
        const before = store.loadWorkingCopy(original.id);
        const set = ops.setField(watched, original.id, { path: '', field: 'schemaVersion', value: '1.0.0', expectedRevision: 3 });
        assert.deepEqual(set, { error: 'invalid_op', revision: 3 });
        const replaced = ops.replacePart(watched, original.id, { path: '', value: clone(original), expectedRevision: 3 });
        assert.deepEqual(replaced, { error: 'invalid_op', revision: 3 });
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(store.loadWorkingCopy(original.id).document, before.document);
    });

    it('a draft write refuses a value of the wrong shape but keeps a missing source or a cell in the wrong tense', () => {
        const verse = 'units/genesis-1/lessons/genesis-1-1/blocks/verse-1';
        const table = 'units/verbs/lessons/to-think/blocks/present';
        [
            [fixture('verse-reading'), verse, (block) => { block.ref.source = { not: ['a', 'string'] }; },
                'schema', L0 + '/blocks/0/ref/source'],
            [fixture('conjugation-table'), table, (block) => { block.cells[0].cell = 'xx'; },
                'schema', L0 + '/blocks/1/cells/0/cell']
        ].forEach(([plan, where, edit, rule, finding]) => {
            db.run('DELETE FROM plans');
            seed(plan);
            const watched = watch();
            const block = clone(ops.readPart(store, plan.id, { path: where }).part);
            edit(block);
            const refused = ops.replacePart(watched, plan.id, { path: where, value: block, expectedRevision: 1 });
            assert.equal(refused.error, 'validation', finding);
            assert.equal(hits(refused.blocks, rule, finding).length, 1, finding);
            assert.equal(watched.calls.saveWorkingCopy, 0, finding);
        });
        [
            [fixture('verse-reading'), verse, (block) => { block.ref.source = ''; }, 'N19', L0 + '/blocks/0/ref/source'],
            [fixture('conjugation-table'), table, (block) => { block.cells[0] = { cell: '1s', formKey: 'חָשַׁבְתִּי' }; },
                'N13', L0 + '/blocks/1/cells/0/cell']
        ].forEach(([plan, where, edit, rule, finding]) => {
            db.run('DELETE FROM plans');
            seed(plan);
            const block = clone(ops.readPart(store, plan.id, { path: where }).part);
            edit(block);
            const drafted = ops.replacePart(store, plan.id, { path: where, value: block, expectedRevision: 1 });
            assert.equal(drafted.error, undefined, JSON.stringify(drafted));
            assert.equal(drafted.revision, 2, finding);
            assert.equal(hits(drafted.validation.blocks, rule, finding).length, 1, finding);
            assert.deepEqual(hits(drafted.validation.blocks, 'schema'), [], finding);
            const refused = publishPlan(store, plan.id, { expectedRevision: 2 });
            assert.equal(refused.error, 'validation', finding);
        });
    });

    it('an operation given schemaVersion keeps it: a 1.0.0 document with a recipe is refused, not raised', () => {
        const original = seed(sample('beginner-unit-1'));
        const watched = watch();
        const value = clone(original);
        lesson(value).recipe = 'topic-conversation';
        const refused = ops.replacePart(watched, original.id, { path: '', value, expectedRevision: 1 });
        assert.equal(refused.error, 'validation');
        assert.deepEqual(refused.blocks.filter((item) => item.rule === 'schema').map((item) => item.path + ' ' + item.message),
            [L0 + '/recipe recipe needs schemaVersion 1.1.0']);
        assert.equal(watched.calls.saveWorkingCopy, 0);

        const raised = ops.setField(store, original.id, { path: '', field: 'schemaVersion', value: '1.1.0', expectedRevision: 1 });
        assert.equal(raised.error, undefined, JSON.stringify(raised));
        const expected = clone(original);
        expected.schemaVersion = '1.1.0';
        expected.revision = 2;
        assert.deepEqual(raised.plan, expected);
    });

    it('publish reads the stored lexicon: stored forms publish, an unverified form warns, a missing form blocks', () => {
        const workshop = seed(fixture('conjugation-table'));
        const published = publishPlan(store, workshop.id, { expectedRevision: 1 });
        assert.equal(published.error, undefined, JSON.stringify(published));
        assert.equal(published.status, 'published');
        assert.deepEqual(shape(published.validation), ['warn N7 /units/0']);

        db.run('DELETE FROM plans');
        const unverified = fixture('conjugation-table');
        lesson(unverified).blocks[2].cells.push({ cell: '3fs', formKey: 'חָשְׁבָה' });
        seed(unverified);
        const warned = publishPlan(store, unverified.id, { expectedRevision: 1 });
        assert.equal(warned.error, undefined, JSON.stringify(warned));
        assert.equal(hits(warned.validation.warnings, 'N14', L0 + '/blocks/2/cells/3/formKey').length, 1);

        db.run('DELETE FROM plans');
        seed(fixture('conjugation-table'));
        const watched = watch();
        const missing = { cell: '3p', formKey: 'חָשְׁבוּ' };
        const drafted = ops.replacePart(watched, unverified.id, {
            path: 'units/verbs/lessons/to-think/blocks/past',
            value: Object.assign(fixtureBlock('conjugation-table'), { id: 'past', tense: 'past', cells: [missing] }),
            expectedRevision: 1
        });
        assert.equal(drafted.error, undefined, JSON.stringify(drafted));
        assert.equal(drafted.validation.ok, false);
        assert.deepEqual(hits(drafted.validation.blocks, 'N13').map((item) => item.path + ' ' + item.message),
            [L0 + '/blocks/2/cells/0/formKey formKey names no stored form']);
        const refused = publishPlan(watched, unverified.id, { expectedRevision: 2 });
        assert.equal(refused.error, 'validation');
        assert.equal(hits(refused.blocks, 'N13').length, 1);
        assert.equal(store.loadSnapshot(unverified.id), null);
        assert.equal(watched.calls.saveSnapshot, 0);
    });
});
