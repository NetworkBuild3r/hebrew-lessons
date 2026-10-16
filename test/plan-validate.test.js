/**
 * Plan validator: the ADR-0003 schema (draft-07) and the Decision 6 registry.
 * Fixtures are the four sample plans plus in-memory mutations.
 * Provenance: INIT-004/SPEC-002
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const {
    FUNCTION_WORDS,
    RULES,
    SCHEMA_PATH,
    tokenize,
    validate,
    validatePlan
} = require('../server/plans/validate');

const ROOT = path.resolve(__dirname, '..');
const SAMPLES = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples');
const ADR = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-course-and-lesson-plan-language.md');
const MODULE = path.join(ROOT, 'server', 'plans', 'validate.js');

const SHALOM = 'שָׁלוֹם';
const SEFER = 'סֵפֶר';
const L0 = '/units/0/lessons/0';
const L1 = '/units/0/lessons/1';
const L2 = '/units/0/lessons/2';
const COFFEE = L0 + '/blocks/0/words/1';
const UNCHECKED = Object.keys(RULES).filter((id) => RULES[id].checked === false);

function sample(name) {
    return JSON.parse(fs.readFileSync(path.join(SAMPLES, name + '.plan.json'), 'utf8'));
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function lesson(plan, index) {
    return plan.units[0].lessons[index];
}

function all(result) {
    return result.blocks.concat(result.warnings, result.informs);
}

function hits(findings, rule, at) {
    return findings.filter((item) => item.rule === rule && (at === undefined || item.path === at));
}

function addWord(target, word, meaning, sentence) {
    target.newWords.push(word);
    const words = target.blocks[0].words;
    words.push({ id: String(words.length + 1), word, meaning, sentence: sentence || word + '.' });
}

describe('ac-1: the samples validate, and N1 follows migration.legacy', () => {
    it('all four samples return no block', () => {
        const names = fs.readdirSync(SAMPLES).filter((name) => name.endsWith('.plan.json')).sort();
        assert.deepEqual(names, [
            'anal-sex-vocab.plan.json',
            'beginner-unit-1.plan.json',
            'my-morning.plan.json',
            'shai-date-indirect-speech.plan.json'
        ]);
        names.forEach((name) => {
            const result = validatePlan(JSON.parse(fs.readFileSync(path.join(SAMPLES, name), 'utf8')));
            assert.equal(result.ok, true, name + ' ' + JSON.stringify(result.blocks));
            assert.deepEqual(result.blocks, [], name);
            assert.deepEqual(hits(all(result), 'schema'), [], name);
        });
    });

    it('anal-sex-vocab and shai-date-indirect-speech return N1 as warn because migration.legacy is true', () => {
        ['anal-sex-vocab', 'shai-date-indirect-speech'].forEach((name) => {
            const plan = sample(name);
            assert.equal(plan.migration.legacy, true);
            assert.ok(lesson(plan, 0).newWords.length > RULES.N1.maxNewWords, name);
            const result = validatePlan(plan);
            const n1 = hits(all(result), 'N1');
            assert.equal(n1.length, 1, name);
            assert.equal(n1[0].severity, 'warn');
            assert.equal(n1[0].path, L0 + '/newWords');
            assert.deepEqual(hits(result.warnings, 'N1'), n1);
            assert.deepEqual(hits(result.blocks, 'N1'), []);
        });
    });

    it('my-morning and the café unit stay within N1', () => {
        ['my-morning', 'beginner-unit-1'].forEach((name) => {
            assert.deepEqual(hits(all(validatePlan(sample(name))), 'N1'), [], name);
        });
    });

    it('my-morning without migration.legacy and with a ninth new word returns N1 as block', () => {
        const base = sample('my-morning');
        assert.equal(lesson(base, 0).newWords.length, RULES.N1.maxNewWords);
        addWord(lesson(base, 0), 'עֲבוֹדָה', 'work', 'אֲנִי יוֹצֵא לָעֲבוֹדָה.');

        const legacy = validatePlan(base);
        assert.equal(legacy.ok, true);
        assert.equal(hits(legacy.warnings, 'N1', L0 + '/newWords').length, 1);

        const removedLegacy = clone(base);
        delete removedLegacy.migration.legacy;
        const result = validatePlan(removedLegacy);
        assert.equal(result.ok, false);
        const n1 = hits(result.blocks, 'N1', L0 + '/newWords');
        assert.equal(n1.length, 1);
        assert.equal(n1[0].severity, 'block');
        assert.deepEqual(hits(result.warnings, 'N1'), []);

        const noMigration = clone(base);
        delete noMigration.migration;
        assert.deepEqual(validatePlan(noMigration).blocks.map((item) => item.rule), ['N1']);

        const notLegacy = clone(base);
        notLegacy.migration.legacy = false;
        assert.deepEqual(validatePlan(notLegacy).blocks.map((item) => item.rule), ['N1']);
    });
});

describe('ac-2: the spike broken cases are rejected for that reason', () => {
    it('a tag-like string is an html block', () => {
        const plan = sample('beginner-unit-1');
        plan.title = '<script>alert(1)</script>';
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        assert.equal(hits(result.blocks, 'html', '/title').length, 1);
    });

    it('a missing alt is an N8 block', () => {
        const plan = sample('beginner-unit-1');
        delete lesson(plan, 0).blocks[0].words[1].image.alt;
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        assert.equal(hits(result.blocks, 'N8', COFFEE + '/image/alt').length, 1);
    });

    it('text over the N11 limit is an N11 block', () => {
        const plan = sample('beginner-unit-1');
        plan.subtitle = 'x'.repeat(4001);
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        assert.equal(hits(result.blocks, 'N11', '/subtitle').length, 1);
    });

    it('a media id that is a URL is a schema block on mediaId', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks[0].words[1].image.mediaId = 'http://evil.example/x.png';
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        const found = hits(result.blocks, 'schema', COFFEE + '/image/mediaId');
        assert.equal(found.length, 1);
        assert.match(found[0].message, /media id/);
    });

    it('a missing required field is a schema block naming the field', () => {
        const plan = sample('beginner-unit-1');
        delete plan.title;
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        const found = hits(result.blocks, 'schema', '/title');
        assert.equal(found.length, 1);
        assert.equal(found[0].message, 'missing required field: title');
    });

    it('a well-formed media id outside the four spike pictures is not rejected', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks[0].words[1].image.mediaId = 'not-a-real-picture';
        const result = validatePlan(plan);
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
        assert.deepEqual(all(result).filter((item) => item.path.startsWith(COFFEE + '/image')), []);
    });
});

describe('ac-3: N12 blocks a word list on fluency-pass and warns beside new words', () => {
    it('a word list on a fluency-pass is an N12 block', () => {
        ['words', 'newWords'].forEach((key) => {
            const plan = sample('beginner-unit-1');
            const fluency = lesson(plan, 2).blocks[0];
            assert.equal(fluency.type, 'fluency-pass');
            fluency[key] = key === 'words'
                ? [{ id: '1', word: lesson(plan, 0).newWords[1], meaning: 'coffee' }]
                : [lesson(plan, 0).newWords[1]];
            const result = validatePlan(plan);
            assert.equal(result.ok, false, key);
            assert.equal(hits(result.blocks, 'N12', L2 + '/blocks/0/' + key).length, 1, key);
        });
    });

    it('each legacy lesson trips the N12 warn on every fluency block', () => {
        const expected = {
            'my-morning': [L0 + '/blocks/2', L0 + '/blocks/3'],
            'anal-sex-vocab': [L0 + '/blocks/2', L0 + '/blocks/3'],
            'shai-date-indirect-speech': [L0 + '/blocks/2']
        };
        Object.keys(expected).forEach((name) => {
            const result = validatePlan(sample(name));
            assert.deepEqual(hits(result.warnings, 'N12').map((item) => item.path), expected[name], name);
            assert.deepEqual(hits(result.blocks, 'N12'), [], name);
        });
    });

    it('the café unit does not trip N12', () => {
        assert.deepEqual(hits(all(validatePlan(sample('beginner-unit-1'))), 'N12'), []);
    });

    it('a fluency-pass beside new words in a non-legacy plan warns and does not block', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks.push(clone(lesson(plan, 2).blocks[0]));
        const result = validatePlan(plan);
        assert.equal(hits(result.warnings, 'N12', L0 + '/blocks/4').length, 1);
        assert.deepEqual(hits(result.blocks, 'N12'), []);
    });
});

describe('ac-4: no picture allow-list', () => {
    it('the module source does not contain the spike picture ids', () => {
        const source = fs.readFileSync(MODULE, 'utf8');
        ['coffee-cup', 'water-glass', 'milk', 'sugar'].forEach((id) => {
            assert.equal(source.includes(id), false, id);
        });
        assert.equal(/spikes/.test(source), false);
    });

    it('every well-formed slug is accepted alike, spike ids included', () => {
        ['coffee-cup', 'tea-pot', 'a', '0-9', 'x'.repeat(64)].forEach((mediaId) => {
            const plan = sample('beginner-unit-1');
            lesson(plan, 0).blocks[0].words[1].image.mediaId = mediaId;
            assert.equal(validatePlan(plan).ok, true, mediaId);
        });
        ['Coffee_Cup', '/media/x', 'x.png', '-x', 'x'.repeat(65)].forEach((mediaId) => {
            const plan = sample('beginner-unit-1');
            lesson(plan, 0).blocks[0].words[1].image.mediaId = mediaId;
            assert.equal(hits(validatePlan(plan).blocks, 'schema', COFFEE + '/image/mediaId').length, 1, mediaId);
        });
    });
});

describe('security: markup and audio paths', () => {
    it('every string that starts an HTML tag is a block, wherever it sits', () => {
        ['</b>', '<!-- x -->', '< img src=x onerror=alert(1)>', 'a </ b', '</>', '<Svg/onload=alert(1)>'].forEach((text) => {
            const plan = sample('beginner-unit-1');
            plan.title = text;
            assert.equal(hits(validatePlan(plan).blocks, 'html', '/title').length, 1, text);
        });
        const alt = sample('beginner-unit-1');
        lesson(alt, 0).blocks[0].words[1].image.alt = '<img src=x>';
        assert.equal(hits(validatePlan(alt).blocks, 'html', COFFEE + '/image/alt').length, 1);

        const glossKey = sample('beginner-unit-1');
        lesson(glossKey, 0).blocks[1].glosses = { '</b>': 'bold' };
        assert.equal(hits(validatePlan(glossKey).blocks, 'html', L0 + '/blocks/1/glosses/<~1b>').length, 1);

        const unknown = sample('beginner-unit-1');
        unknown.extra = { note: '<b>x</b>' };
        const result = validatePlan(unknown);
        assert.equal(hits(result.blocks, 'html', '/extra/note').length, 1);
        assert.equal(hits(result.blocks, 'schema', '/extra').length, 1);
    });

    it('a comparison sign that starts no tag is allowed', () => {
        const plan = sample('beginner-unit-1');
        plan.subtitle = 'a < 3 and 5 > 4';
        assert.equal(validatePlan(plan).ok, true);
    });

    it('an absolute, scheme-bearing, or escaping audioPath is a block', () => {
        ['/abs/x.mp3', 'https://evil.example/x.mp3', '//evil.example/x.mp3', 'javascript:alert(1).mp3',
            'C:\\clips\\x.mp3', '../../x.mp3', 'audio/../../x.mp3', 'audio/x.wav'].forEach((audioPath) => {
            const plan = sample('beginner-unit-1');
            lesson(plan, 0).blocks[0].words[1].audioPath = audioPath;
            lesson(plan, 0).blocks[2].audioPath = audioPath;
            const result = validatePlan(plan);
            assert.equal(hits(result.blocks, 'audio-path', COFFEE + '/audioPath').length, 1, audioPath);
            assert.equal(hits(result.blocks, 'audio-path', L0 + '/blocks/2/audioPath').length, 1, audioPath);
        });
    });

    it('relative clip paths, as the migrated lessons carry them, pass', () => {
        ['audio/story/line-1.mp3', '../_audio/words/806bb3fc99c984531106aeb5.mp3', 'clip.mp3'].forEach((audioPath) => {
            const plan = sample('beginner-unit-1');
            lesson(plan, 0).blocks[0].words[1].audioPath = audioPath;
            assert.equal(validatePlan(plan).ok, true, audioPath);
        });
    });

    it('the schema markup pattern runs in linear time', () => {
        const schema = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
        const markup = new RegExp(schema.definitions.plainText.not.pattern, 'u');
        const hostile = '<' + ' '.repeat(200000) + '1';
        const started = process.hrtime.bigint();
        assert.equal(markup.test(hostile), false);
        assert.ok(Number(process.hrtime.bigint() - started) / 1e6 < 100);
    });

    it('many phrases that share a long run of one token do not stall the text estimates', () => {
        const plan = sample('beginner-unit-1');
        const template = lesson(plan, 0);
        const run = Array(60).fill('\u05D0').join(' ');
        plan.units[0].lessons = Array.from({ length: 6 }, (unused, l) => {
            const item = clone(template);
            item.id = 'lesson-' + l;
            item.newWords = Array.from({ length: 24 }, (unused2, i) => run + ' \u05D1' + (l * 24 + i));
            item.blocks = [item.blocks[0], {
                id: 'story',
                type: 'graded-story',
                paragraphs: Array.from({ length: 24 }, (unused2, i) => ({ id: 'p' + i, text: Array(400).fill('\u05D0').join(' ') }))
            }];
            return item;
        });
        const started = process.hrtime.bigint();
        const result = validatePlan(plan);
        assert.ok(Number(process.hrtime.bigint() - started) / 1e6 < 1000);
        assert.deepEqual(hits(result.blocks, 'N9'), []);
        assert.equal(hits(result.warnings, 'N6').length, 6 * 24);
    });

    it('a long run of punctuation inside one token does not stall the tokenizer', () => {
        const token = 'a' + '!'.repeat(200000) + 'b';
        const started = process.hrtime.bigint();
        assert.deepEqual(tokenize('"' + token + '"'), [token]);
        assert.ok(Number(process.hrtime.bigint() - started) / 1e6 < 100);

        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks[1].paragraphs[0].text = '\u05D0' + '!'.repeat(300000) + '\u05D1';
        const begun = process.hrtime.bigint();
        const result = validatePlan(plan);
        assert.ok(Number(process.hrtime.bigint() - begun) / 1e6 < 1000);
        assert.equal(hits(result.blocks, 'N11', L0 + '/blocks/1/paragraphs/0/text').length, 1);
        assert.equal(hits(result.warnings, 'N3', L0 + '/blocks/1').length, 1);
    });

    it('long whitespace after a < does not stall validation', () => {
        const plan = sample('beginner-unit-1');
        plan.subtitle = '<' + ' '.repeat(100000) + '1';
        [0, 1].forEach((index) => {
            lesson(plan, index).blocks[0].words.slice(0, 4).forEach((word) => {
                word.trap = '<' + ' '.repeat(3990) + '1';
            });
        });
        const started = process.hrtime.bigint();
        const result = validatePlan(plan);
        assert.ok(Number(process.hrtime.bigint() - started) / 1e6 < 1000);
        assert.deepEqual(result.blocks.map((item) => item.rule + ' ' + item.path), ['N11 /subtitle']);
    });
});

describe('the registry matches ADR-0003 Decision 6', () => {
    const adr = fs.readFileSync(ADR, 'utf8');
    const rows = adr.split(/\r?\n/)
        .filter((line) => /^\| N\d+ \|/.test(line))
        .map((line) => line.split('|').map((cell) => cell.trim()));
    const bold = (cell) => (cell.match(/\*\*([^*]+)\*\*/g) || []).map((text) => text.replace(/\*\*/g, ''));

    it('N1 to N12 are the numbered rules, in order', () => {
        const ids = rows.map((cells) => cells[1]);
        assert.deepEqual(ids, ['N1', 'N2', 'N3', 'N4', 'N5', 'N6', 'N7', 'N8', 'N9', 'N10', 'N11', 'N12']);
        assert.deepEqual(Object.keys(RULES).filter((id) => /^N\d+$/.test(id)).slice(0, ids.length), ids);
    });

    it('each numbered rule carries the ADR severity', () => {
        rows.forEach(([, id, , severity]) => {
            const named = bold(severity).filter((word) => /^(Block|Warn|Inform)$/.test(word)).map((word) => word.toLowerCase());
            const rule = RULES[id];
            if (!named.length) {
                assert.equal(rule.severity, null, id);
                assert.equal(rule.checked, false, id);
                return;
            }
            assert.equal(rule.severity, named[0], id);
            assert.equal(rule.legacySeverity || rule.sharedLessonSeverity, named[1], id);
        });
        assert.equal(RULES.N1.legacySeverity, 'warn');
        assert.equal(RULES.N12.sharedLessonSeverity, 'warn');
    });

    it('each figure equals the ADR figure, in the registry or the schema', () => {
        const figures = Object.fromEntries(rows.map(([, id, text]) => [id, bold(text)]));
        const schema = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
        assert.deepEqual(figures.N1, [String(RULES.N1.maxNewWords)]);
        assert.deepEqual(figures.N2, [String(RULES.N2.minRecycles)]);
        assert.deepEqual(figures.N3, [RULES.N3.minKnownPercent + '%']);
        assert.deepEqual(figures.N5, ['one']);
        assert.deepEqual(figures.N6, ['twice']);
        assert.equal(RULES.N6.minOccurrences, 2);
        assert.deepEqual(figures.N7, ['one quarter']);
        assert.equal(RULES.N7.blocksPerIntroduction, 4);
        assert.deepEqual(figures.N8, [schema.definitions.altText.minLength + ' to ' + schema.definitions.altText.maxLength]);
        assert.deepEqual(figures.N9, [RULES.N9.planBytes, RULES.N9.lessonBytes, RULES.N9.blockBytes]
            .map((bytes) => bytes / 1024 + ' KiB'));
        assert.deepEqual(figures.N10, [
            schema.properties.units.maxItems,
            schema.definitions.lesson.properties.blocks.maxItems,
            schema.definitions.introductionBlock.properties.words.maxItems
        ].map(String));
        assert.equal(schema.definitions.unit.properties.lessons.maxItems, undefined);
        assert.equal(schema.definitions.lesson.properties.buildsOn.maxItems, undefined);
        assert.deepEqual(figures.N11, [String(schema.definitions.plainText.maxLength)]);
        assert.deepEqual(Object.keys(RULES.N4).sort(), ['checked', 'severity', 'summary']);
    });

    it('the further rules follow the ADR table, in order, with its severities', () => {
        const start = adr.indexOf('Further rules, with no second copy of a number:');
        const end = adr.indexOf('**Coverage estimate (N3).**');
        assert.ok(start > 0 && end > start);
        const severities = adr.slice(start, end).split(/\r?\n/)
            .filter((line) => line.startsWith('| ') && !line.startsWith('| Rule |'))
            .map((line) => line.split('|')[2].trim().split(/[\s,.]/)[0].toLowerCase());
        const further = Object.keys(RULES).filter((id) => id !== 'schema' && !/^N\d+$/.test(id));
        assert.equal(further.length, 17);
        assert.deepEqual(further.map((id) => RULES[id].severity), severities);
    });

    it('FUNCTION_WORDS is the ADR closed function list', () => {
        const lines = adr.split(/\r?\n/);
        const after = lines.findIndex((line) => line.includes('this closed function list'));
        const listLine = lines.slice(after + 1).find((line) => line.startsWith('`'));
        const words = (listLine.match(/`([^`]+)`/g) || []).map((word) => word.slice(1, -1).normalize('NFC'));
        assert.equal(words.length, 9);
        assert.deepEqual(Array.from(FUNCTION_WORDS), words);
    });

    it('rules the ADR leaves to a reviewer or to no 1.0.0 block are never reported', () => {
        assert.deepEqual(UNCHECKED.sort(), ['N4', 'form-recall-image', 'semantic-set']);
        ['my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech', 'beginner-unit-1'].forEach((name) => {
            all(validatePlan(sample(name))).forEach((item) => {
                assert.equal(UNCHECKED.includes(item.rule), false, name + ' ' + item.rule);
            });
        });
    });
});

describe('result shape and purity', () => {
    it('separates blocks, warnings, and informs, each naming a rule id and a path', () => {
        ['my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech', 'beginner-unit-1'].forEach((name) => {
            const result = validatePlan(sample(name));
            [['blocks', 'block'], ['warnings', 'warn'], ['informs', 'inform']].forEach(([key, severity]) => {
                assert.ok(Array.isArray(result[key]), name + ' ' + key);
                result[key].forEach((item) => {
                    assert.equal(item.severity, severity);
                    assert.ok(Object.prototype.hasOwnProperty.call(RULES, item.rule), item.rule);
                    assert.equal(typeof item.path, 'string');
                    assert.ok(item.path === '' || item.path.startsWith('/'), item.path);
                    assert.ok(item.message.length > 0);
                });
            });
            assert.equal(result.ok, result.blocks.length === 0);
        });
    });

    it('does not mutate the document and accepts its JSON text', () => {
        const plan = sample('shai-date-indirect-speech');
        const before = JSON.stringify(plan);
        validatePlan(plan);
        assert.equal(JSON.stringify(plan), before);
        const text = fs.readFileSync(path.join(SAMPLES, 'shai-date-indirect-speech.plan.json'), 'utf8');
        assert.deepEqual(validatePlan(text), validatePlan(plan));
        assert.equal(validate, validatePlan);
    });

    it('requires only fs and path: no database or network', () => {
        const source = fs.readFileSync(MODULE, 'utf8');
        const required = (source.match(/require\(\s*'([^']+)'\s*\)/g) || []).map((call) => call.match(/'([^']+)'/)[1]);
        // INIT-008/SPEC-008: the five Hebrew checks live in hebrew-checks.js, which itself
        // requires only fs, path, and ./id-path (plan-hebrew-checks.test.js pins that).
        assert.deepEqual(required.sort(), ['./activity-map', './hebrew-checks', 'fs', 'path']);
    });

    it('a non-object, non-JSON, or unserializable document is a schema block', () => {
        const cyclic = {};
        cyclic.self = cyclic;
        [[], 'not json', undefined, cyclic, 42].forEach((document) => {
            const result = validatePlan(document);
            assert.equal(result.ok, false);
            assert.deepEqual(result.blocks.map((item) => item.rule + ' ' + item.path), ['schema ']);
        });
    });

    it('deep nesting under an unknown property is rejected without throwing', () => {
        const plan = sample('beginner-unit-1');
        let deep = [];
        for (let i = 0; i < 100000; i += 1) deep = [deep];
        plan.extra = deep;
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        assert.equal(hits(result.blocks, 'schema', '/extra').length, 1);
    });
});

describe('schema: draft-07, closed properties, known schemaVersions', () => {
    it('an unknown schemaVersion is reported alone and the rest is not parsed', () => {
        ['1.3.0', '2.0.0', 1, null].forEach((version) => {
            const plan = sample('beginner-unit-1');
            plan.schemaVersion = version;
            plan.title = '<b>not reached</b>';
            const result = validatePlan(plan);
            assert.equal(result.ok, false, String(version));
            assert.deepEqual(result.blocks.map((item) => item.rule + ' ' + item.path), ['schema /schemaVersion']);
            assert.deepEqual(result.warnings, []);
            assert.deepEqual(result.informs, []);
        });
        const missing = sample('beginner-unit-1');
        delete missing.schemaVersion;
        assert.deepEqual(validatePlan(missing).blocks.map((item) => item.rule + ' ' + item.path), ['schema /schemaVersion']);
    });

    it('unknown properties fail at every level', () => {
        const plan = sample('beginner-unit-1');
        plan.extra = 1;
        lesson(plan, 0).extra = 1;
        lesson(plan, 0).blocks[1].extra = 1;
        lesson(plan, 0).blocks[0].words[0].extra = 1;
        const paths = hits(validatePlan(plan).blocks, 'schema').map((item) => item.path);
        assert.deepEqual(paths.sort(), [
            '/extra',
            L0 + '/blocks/0/words/0/extra',
            L0 + '/blocks/1/extra',
            L0 + '/extra'
        ].sort());
    });

    it('an unknown block type explains itself without guessing a branch', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks[1].type = 'speed-quiz';
        const found = hits(validatePlan(plan).blocks, 'schema', L0 + '/blocks/1');
        assert.equal(found.length, 1);
        assert.equal(found[0].message, 'unknown block type');
    });

    it('a known block type reports its own missing field', () => {
        const plan = sample('beginner-unit-1');
        delete lesson(plan, 0).blocks[2].passage;
        assert.equal(hits(validatePlan(plan).blocks, 'schema', L0 + '/blocks/2/passage').length, 1);
    });

    it('a personal plan needs an opaque assignee id and a shared plan none', () => {
        const personal = sample('beginner-unit-1');
        personal.audience = 'personal';
        personal.assigneeId = 'learner-0001';
        assert.equal(validatePlan(personal).ok, true);
        personal.assigneeId = 'Bob';
        assert.equal(hits(validatePlan(personal).blocks, 'schema', '/assigneeId').length, 1);
        const shared = sample('beginner-unit-1');
        shared.assigneeId = 'learner-0001';
        assert.equal(hits(validatePlan(shared).blocks, 'schema', '/assigneeId').length, 1);
    });

    it('lengths count code points, as JSON Schema does', () => {
        const plan = sample('beginner-unit-1');
        plan.subtitle = '\u{1F600}'.repeat(4000);
        assert.equal(validatePlan(plan).ok, true);
        plan.subtitle = 'x'.repeat(4000);
        assert.equal(validatePlan(plan).ok, true);
    });
});

describe('N2, N5, N7, N8, N9, N10, N11 limits', () => {
    it('N2 warns when buildsOn is set and fewer than its count are recycled', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 1).recycles = lesson(plan, 1).recycles.slice(0, RULES.N2.minRecycles - 1);
        assert.equal(hits(validatePlan(plan).warnings, 'N2', L1 + '/recycles').length, 1);
        assert.deepEqual(hits(all(validatePlan(sample('beginner-unit-1'))), 'N2'), []);
    });

    it('N5 blocks a second introduction and its second pattern', () => {
        const plan = sample('beginner-unit-1');
        const second = clone(lesson(plan, 0).blocks[0]);
        second.id = 'introduction-2';
        lesson(plan, 0).blocks.splice(1, 0, second);
        const found = hits(validatePlan(plan).blocks, 'N5');
        assert.deepEqual(found.map((item) => item.path), [L0 + '/blocks/1']);
        assert.match(found[0].message, /second pattern/);
    });

    it('N7 warns when introductions exceed one quarter of a unit\'s blocks', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks = lesson(plan, 0).blocks.slice(0, 1);
        lesson(plan, 1).blocks = lesson(plan, 1).blocks.slice(0, 1);
        assert.equal(hits(validatePlan(plan).warnings, 'N7', '/units/0').length, 1);
        assert.deepEqual(hits(all(validatePlan(sample('beginner-unit-1'))), 'N7'), []);
    });

    it('N8 blocks an empty or over-long alt and accepts the limit', () => {
        const schema = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
        const limit = schema.definitions.altText.maxLength;
        [['', 1], ['x'.repeat(limit + 1), 1], ['x'.repeat(limit), 0]].forEach(([alt, count]) => {
            const plan = sample('beginner-unit-1');
            lesson(plan, 0).blocks[0].words[1].image.alt = alt;
            assert.equal(hits(validatePlan(plan).blocks, 'N8', COFFEE + '/image/alt').length, count, String(alt.length));
        });
    });

    it('N9 blocks an oversized block and lesson', () => {
        const block = sample('beginner-unit-1');
        const intro = lesson(block, 0).blocks[0];
        intro.note = 'x'.repeat(4000);
        intro.words.forEach((word) => {
            word.trap = 'x'.repeat(4000);
        });
        const blockResult = validatePlan(block);
        assert.equal(hits(blockResult.blocks, 'N9', L0 + '/blocks/0').length, 1);
        assert.deepEqual(hits(blockResult.blocks, 'N9', L0), []);

        const big = sample('beginner-unit-1');
        lesson(big, 0).blocks[1].paragraphs = Array.from({ length: 24 }, (unused, i) => ({ id: 'p' + i, text: 'א'.repeat(4000) }));
        const lessonResult = validatePlan(big);
        assert.equal(hits(lessonResult.blocks, 'N9', L0 + '/blocks/1').length, 1);
        assert.equal(hits(lessonResult.blocks, 'N9', L0).length, 1);
        assert.deepEqual(hits(lessonResult.blocks, 'N9', ''), []);
        assert.equal(hits(lessonResult.warnings, 'N3', L0 + '/blocks/1').length, 1);
    });

    it('a plan over N9 is reported by size alone', () => {
        const plan = sample('beginner-unit-1');
        plan.title = '<b>not reached</b>';
        plan.units[0].lessons.forEach((item) => {
            item.blocks.push({
                id: 'filler',
                type: 'graded-story',
                paragraphs: Array.from({ length: 24 }, (unused, i) => ({ id: 'p' + i, text: 'א'.repeat(4000) }))
            });
        });
        const result = validatePlan(plan);
        assert.equal(hits(result.blocks, 'N9', '').length, 1);
        assert.deepEqual(hits(result.blocks, 'N9').map((item) => item.path).sort(), [
            '',
            L0, L0 + '/blocks/4',
            L1, L1 + '/blocks/4',
            L2, L2 + '/blocks/3'
        ].sort());
        assert.deepEqual(all(result).filter((item) => item.rule !== 'N9'), []);
    });

    it('an input far over N9 returns one finding, not one per error', () => {
        const plan = sample('beginner-unit-1');
        plan.extra = {};
        for (let i = 0; i < 50000; i += 1) plan.extra['k' + i] = '<b>' + 'x'.repeat(24) + '</b>';
        const findings = all(validatePlan(plan));
        assert.equal(findings.length, 1, 'findings: ' + findings.length);
        assert.equal(findings[0].rule + ' ' + findings[0].path, 'N9 ');
    });

    it('N10 blocks one past each ceiling and accepts the ceiling', () => {
        const schema = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
        const cases = [
            ['/units', schema.properties.units.maxItems, (plan, n) => {
                plan.units = Array.from({ length: n }, () => clone(plan.units[0]));
            }],
            [L0 + '/blocks', schema.definitions.lesson.properties.blocks.maxItems, (plan, n) => {
                const dialogue = lesson(plan, 0).blocks[3];
                while (lesson(plan, 0).blocks.length < n) lesson(plan, 0).blocks.push(clone(dialogue));
            }],
            [L0 + '/blocks/0/words', schema.definitions.introductionBlock.properties.words.maxItems, (plan, n) => {
                const words = lesson(plan, 0).blocks[0].words;
                while (words.length < n) words.push(clone(words[0]));
            }]
        ];
        cases.forEach(([at, ceiling, grow]) => {
            const over = sample('beginner-unit-1');
            grow(over, ceiling + 1);
            assert.equal(hits(validatePlan(over).blocks, 'N10', at).length, 1, at);
            const atCeiling = sample('beginner-unit-1');
            grow(atCeiling, ceiling);
            assert.deepEqual(hits(validatePlan(atCeiling).blocks, 'N10'), [], at);
        });
    });

    it('N10 does not refuse a plan for lesson count (INIT-010/SPEC-002)', () => {
        const plan = sample('beginner-unit-1');
        const small = {
            id: 'tiny',
            title: 'Tiny',
            level: 'basic',
            visibility: 'hidden',
            buildsOn: [],
            newWords: [],
            recycles: [],
            blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
        };
        plan.units[0].lessons = Array.from({ length: 100 }, (_, i) => Object.assign(clone(small), { id: 'l-' + i }));
        const result = validatePlan(plan);
        assert.deepEqual(hits(result.blocks, 'N10'), []);
        assert.deepEqual(hits(result.blocks, 'schema'), []);
    });

    it('N11 also limits keys the schema leaves open', () => {
        const plan = sample('beginner-unit-1');
        const key = 'א'.repeat(4001);
        lesson(plan, 0).blocks[1].glosses = { [key]: 'long' };
        assert.equal(hits(validatePlan(plan).blocks, 'N11', L0 + '/blocks/1/glosses/' + key).length, 1);
    });
});

describe('N3, N4, and the tokenizer', () => {
    const KAFE = lesson(sample('beginner-unit-1'), 0).newWords[1].normalize('NFC');
    const ANI = FUNCTION_WORDS[0];

    function withPassage(text, edit) {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks[2].passage = text;
        if (edit) edit(plan);
        return validatePlan(plan);
    }

    function n3(result) {
        return hits(all(result), 'N3', L0 + '/blocks/2');
    }

    it('the tokenizer splits on whitespace, strips edge punctuation, and keeps prefixes', () => {
        assert.deepEqual(tokenize('"' + KAFE + '," \u2014 ' + ANI + '!'), [KAFE, ANI]);
        assert.deepEqual(tokenize('\u05D5\u05B0' + KAFE), ['\u05D5\u05B0' + KAFE]);
        assert.deepEqual(tokenize('\u05D1\u05D9\u05EA\u05BE\u05E1\u05E4\u05E8 (\u05D2\u05D3\u05D5\u05DC).'),
            ['\u05D1\u05D9\u05EA\u05BE\u05E1\u05E4\u05E8', '\u05D2\u05D3\u05D5\u05DC']);
        assert.deepEqual(tokenize(' \u200f' + KAFE + '\u200f '), [KAFE]);
        assert.deepEqual(tokenize(' \u2014 , '), []);
    });

    it('N3 warns below its share and not at it', () => {
        assert.equal(n3(withPassage((KAFE + ' ').repeat(18) + SHALOM)).length, 1);
        assert.equal(n3(withPassage((KAFE + ' ').repeat(19) + SHALOM)).length, 0);
    });

    it('N4 adds no second bar: a listening passage between the two is clean', () => {
        const result = withPassage((KAFE + ' ').repeat(24) + SHALOM);
        assert.deepEqual(n3(result), []);
        assert.deepEqual(hits(all(result), 'N4'), []);
    });

    it('a prefixed form is not known; an unpointed form is known only when unique', () => {
        assert.equal(n3(withPassage(('ו\u05B0' + KAFE + ' ').repeat(20))).length, 1);
        assert.equal(n3(withPassage('קפה '.repeat(20))).length, 0);
        const ambiguous = withPassage('קפה '.repeat(20), (plan) => {
            lesson(plan, 0).recycles.push({ word: 'ק\u05B7פ\u05BC\u05B6ה', fromLessonId: 'elsewhere' });
        });
        assert.equal(n3(ambiguous).length, 1);
    });

    it('the closed function list counts as known', () => {
        assert.equal(n3(withPassage((FUNCTION_WORDS.join(' ') + ' ').repeat(3))).length, 0);
    });

    it('a multi-word entry is known as a run, not word by word', () => {
        const later = sample('my-morning');
        const phrase = lesson(later, 0).newWords[6];
        assert.equal(tokenize(phrase).length, 2);
        const recycle = (plan) => lesson(plan, 0).recycles.push({ word: phrase, fromLessonId: 'my-morning' });
        assert.equal(n3(withPassage((phrase + ' ').repeat(10), recycle)).length, 0);
        assert.equal(n3(withPassage((tokenize(phrase)[1] + ' ').repeat(20), recycle)).length, 1);
    });

    it('glosses, support, and spans count as treated-as-known in a graded story', () => {
        const story = (edit) => {
            const plan = sample('beginner-unit-1');
            const block = lesson(plan, 0).blocks[1];
            block.paragraphs = [{ id: 'p1', text: (SHALOM + ' ').repeat(20) }];
            edit(block);
            return hits(all(validatePlan(plan)), 'N3', L0 + '/blocks/1');
        };
        assert.equal(story(() => {}).length, 1);
        assert.equal(story((block) => { block.glosses = { [SHALOM]: 'hello' }; }).length, 0);
        assert.equal(story((block) => { block.support = [{ id: 's1', he: 'שלום', en: 'hello' }]; }).length, 0);
        assert.equal(story((block) => {
            block.paragraphs[0].spans = [{ id: 'sp1', kind: 'direct', text: SHALOM }];
        }).length, 0);
    });
});

describe('N6 and the later-lesson rules', () => {
    function withShalom(edit) {
        const plan = sample('beginner-unit-1');
        addWord(lesson(plan, 1), SHALOM, 'hello');
        if (edit) edit(plan);
        return validatePlan(plan);
    }
    const AT = L1 + '/newWords/5';

    it('the café unit meets N6, later-context, and later-checkpoint', () => {
        const result = validatePlan(sample('beginner-unit-1'));
        ['N6', 'later-context', 'later-checkpoint'].forEach((rule) => {
            assert.deepEqual(hits(all(result), rule), [], rule);
        });
    });

    it('a new word used once warns on N6 and on both later-lesson rules', () => {
        const result = withShalom();
        ['N6', 'later-context', 'later-checkpoint'].forEach((rule) => {
            assert.equal(hits(result.warnings, rule, AT).length, 1, rule);
        });
    });

    it('a second occurrence clears N6; only a different later sentence clears later-context', () => {
        const sameSentence = withShalom((plan) => {
            lesson(plan, 1).blocks[1].paragraphs.push({ id: 'p7', text: SHALOM + '.' });
            lesson(plan, 2).blocks[1].lines.push({ id: 'l5', speaker: 'learner', pointed: SHALOM + '!' });
        });
        assert.deepEqual(hits(all(sameSentence), 'N6', AT), []);
        assert.equal(hits(sameSentence.warnings, 'later-context', AT).length, 1);

        const different = withShalom((plan) => {
            lesson(plan, 1).blocks[1].paragraphs.push({ id: 'p7', text: SHALOM + '.' });
            lesson(plan, 2).blocks[1].lines.push({ id: 'l5', speaker: 'learner', pointed: SHALOM + ', תּוֹדָה.' });
            lesson(plan, 2).blocks[2].wordKeys.push(SHALOM);
        });
        ['N6', 'later-context', 'later-checkpoint'].forEach((rule) => {
            assert.deepEqual(hits(all(different), rule, AT), [], rule);
        });
    });

    it('a one-lesson plan is exempt from the later-lesson rules', () => {
        ['my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech'].forEach((name) => {
            const result = validatePlan(sample(name));
            assert.deepEqual(hits(all(result), 'later-context'), [], name);
            assert.deepEqual(hits(all(result), 'later-checkpoint'), [], name);
        });
    });

    it('legacy prefixed forms do not count as occurrences', () => {
        const plan = sample('my-morning');
        const boker = lesson(plan, 0).newWords[4].normalize('NFC');
        const story = lesson(plan, 0).blocks[1].paragraphs.flatMap((paragraph) => tokenize(paragraph.text));
        assert.equal(story.includes(boker), false);
        assert.ok(story.some((token) => token !== boker && token.endsWith(boker)));
        assert.equal(hits(validatePlan(plan).warnings, 'N6', L0 + '/newWords/4').length, 1);
    });
});

describe('further rules', () => {
    it('recall-day and questions are informs on the café unit', () => {
        const result = validatePlan(sample('beginner-unit-1'));
        assert.deepEqual(result.informs.map((item) => item.rule + ' ' + item.path).sort(), [
            'questions ' + L0 + '/blocks/1/questions',
            'recall-day ' + L2 + '/blocks/2'
        ]);
    });

    it('frequency-band informs when a unit records one', () => {
        const plan = sample('beginner-unit-1');
        plan.units[0].frequencyBand = 'band-1';
        assert.equal(hits(validatePlan(plan).informs, 'frequency-band', '/units/0/frequencyBand').length, 1);
    });

    it('queue-order informs on a personal plan only', () => {
        const plan = sample('beginner-unit-1');
        plan.audience = 'personal';
        plan.assigneeId = 'learner-0001';
        assert.equal(hits(validatePlan(plan).informs, 'queue-order', '').length, 1);
        assert.deepEqual(hits(all(validatePlan(sample('beginner-unit-1'))), 'queue-order'), []);
    });

    it('pattern-focus warns when the focus is in no story or dialogue, here or later in the unit', () => {
        const missing = sample('beginner-unit-1');
        lesson(missing, 1).blocks[0].pattern.rules[0].focus = [SEFER];
        assert.equal(hits(validatePlan(missing).warnings, 'pattern-focus', L1 + '/blocks/0/pattern/rules/0/focus/0').length, 1);

        const later = sample('beginner-unit-1');
        lesson(later, 0).blocks[0].pattern.rules[0].focus = [lesson(later, 1).newWords[0]];
        assert.deepEqual(hits(all(validatePlan(later)), 'pattern-focus'), []);
    });

    it('pointed-word warns on a new word with no vowel point', () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).newWords[1] = 'קפה';
        lesson(plan, 0).blocks[0].words[1].word = 'קפה';
        assert.equal(hits(validatePlan(plan).warnings, 'pointed-word', L0 + '/newWords/1').length, 1);
    });

    it('romanized-answer blocks a Latin cloze answer, a Latin correct choice, and a missing choice', () => {
        const plan = sample('my-morning');
        const intro = lesson(plan, 0).blocks[0];
        intro.cloze.items[0].answer = 'kam';
        intro.pattern.items[0].choices[1] = 'Ani kam ve-shote kafe.';
        intro.pattern.items[1].answer = 5;
        const found = hits(validatePlan(plan).blocks, 'romanized-answer').map((item) => item.path);
        assert.deepEqual(found, [
            L0 + '/blocks/0/cloze/items/0/answer',
            L0 + '/blocks/0/pattern/items/0/choices/1',
            L0 + '/blocks/0/pattern/items/1/answer'
        ]);
        assert.deepEqual(hits(all(validatePlan(sample('my-morning'))), 'romanized-answer'), []);
    });

    it('meaning blocks a new word with no introduction meaning', () => {
        const unglossed = sample('beginner-unit-1');
        lesson(unglossed, 0).newWords[7] = SHALOM;
        assert.equal(hits(validatePlan(unglossed).blocks, 'meaning', L0 + '/newWords/7').length, 1);

        const missing = sample('beginner-unit-1');
        delete lesson(missing, 0).blocks[0].words[0].meaning;
        const result = validatePlan(missing);
        assert.equal(hits(result.blocks, 'meaning', L0 + '/blocks/0/words/0/meaning').length, 1);
        assert.equal(hits(result.blocks, 'meaning', L0 + '/newWords/0').length, 1);

        const blank = sample('beginner-unit-1');
        lesson(blank, 0).blocks[0].words[0].meaning = '   ';
        assert.equal(hits(validatePlan(blank).blocks, 'meaning', L0 + '/newWords/0').length, 1);
    });

    it('image-placement blocks an image anywhere but an introduction word', () => {
        const image = () => ({ mediaId: 'coffee-cup', alt: 'coffee' });
        const cases = [
            [(plan) => { lesson(plan, 0).blocks[1].paragraphs[0].image = image(); }, L0 + '/blocks/1/paragraphs/0/image'],
            [(plan) => { lesson(plan, 2).blocks[2].image = image(); }, L2 + '/blocks/2/image'],
            [(plan) => { lesson(plan, 1).recycles[0].image = image(); }, L1 + '/recycles/0/image'],
            [(plan) => { lesson(plan, 0).extra = { image: image() }; }, L0 + '/extra/image'],
            [(plan) => { lesson(plan, 0).blocks[3].lines[0].mediaId = 'coffee-cup'; }, L0 + '/blocks/3/lines/0/mediaId']
        ];
        cases.forEach(([edit, at]) => {
            const plan = sample('beginner-unit-1');
            edit(plan);
            assert.equal(hits(validatePlan(plan).blocks, 'image-placement', at).length, 1, at);
        });
        assert.deepEqual(hits(all(validatePlan(sample('beginner-unit-1'))), 'image-placement'), []);

        const glossed = sample('beginner-unit-1');
        lesson(glossed, 0).blocks[1].glosses = { image: 'picture', mediaId: 'id' };
        assert.equal(validatePlan(glossed).ok, true);
    });
});
