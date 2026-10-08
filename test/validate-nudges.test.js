/**
 * Validator nudge warnings — recycling, encounters, coverage, glosses.
 * Authored-by-spec: INIT-001/SPEC-008
 */
const { describe, it, before, after, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const root = path.resolve(__dirname, '..');
const validate = path.join(root, '.cursor', 'skills', 'hebrew-worksheet', 'scripts', 'validate.js');
const templateShell = path.join(root, 'worksheets', '_template', 'index.html');

function writeJs(file, globalName, value) {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, 'window.' + globalName + ' = ' + JSON.stringify(value, null, 4) + ';\n', 'utf8');
}

function writeLesson(lessons, slug, worksheet) {
    const dir = path.join(lessons, slug);
    fs.mkdirSync(dir, { recursive: true });
    fs.copyFileSync(templateShell, path.join(dir, 'index.html'));
    writeJs(path.join(dir, 'worksheet.js'), 'WORKSHEET', Object.assign({ id: slug }, worksheet));
}

function runValidate(lessons, slug) {
    return spawnSync(process.execPath, [validate, slug], {
        cwd: root,
        encoding: 'utf8',
        env: Object.assign({}, process.env, {
            HEBREW_ROOT: root,
            LESSONS_DIR: lessons
        })
    });
}

function warnLines(stdout) {
    return String(stdout || '')
        .split(/\r?\n/)
        .filter(line => line.startsWith('WARN '))
        .map(line => line.slice(5));
}

const earlierVocab = {
    title: 'Earlier',
    level: 'basic',
    vocab: {
        items: [
            { id: '1', word: 'קָם', meaning: 'gets up (m)', sound: 'kam', trap: 't', sentence: 'אֲנִי קָם.' },
            { id: '2', word: 'קָמָה', meaning: 'gets up (f)', sound: 'ka-MA', trap: 't', sentence: 'הִיא קָמָה.' },
            { id: '3', word: 'בֹּקֶר', meaning: 'morning', sound: 'BO-ker', trap: 't', sentence: 'בֹּקֶר טוֹב.' },
            { id: '4', word: 'קָפֶה', meaning: 'coffee', sound: 'ka-FE', trap: 't', sentence: 'קָפֶה חַם.' },
            { id: '5', word: 'שׁוֹתֶה', meaning: 'drinks (m)', sound: 'sho-TE', trap: 't', sentence: 'הוּא שׁוֹתֶה.' }
        ]
    },
    reading: {
        paragraphs: [['בֹּקֶר. ', { word: 'קָם' }, ' שׁוֹתֶה קָפֶה.']]
    }
};

describe('validator nudges (INIT-001/SPEC-008)', () => {
    let tmp;
    let lessons;

    beforeEach(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-nudges-'));
        lessons = path.join(tmp, 'worksheets');
        fs.mkdirSync(lessons, { recursive: true });
    });

    afterEach(() => {
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
    });

    it('ac-1: warns when recycled words < 3; silent when ≥ 3', () => {
        writeLesson(lessons, 'earlier', earlierVocab);
        writeJs(path.join(lessons, 'catalog.js'), 'HEBREW_CATALOG', [
            { slug: 'earlier', title: 'Earlier' },
            { slug: 'low-recycle', title: 'Low' },
            { slug: 'ok-recycle', title: 'Ok' }
        ]);

        writeLesson(lessons, 'low-recycle', {
            title: 'Low recycle',
            level: 'basic',
            vocab: {
                items: [
                    { id: '1', word: 'חָדָשׁ', meaning: 'new', sound: 'cha-DASH', trap: 't', sentence: 'זֶה חָדָשׁ. חָדָשׁ מְאֹד.' }
                ]
            },
            reading: {
                paragraphs: [['זֶה ', { word: 'חָדָשׁ' }, ' חָדָשׁ מְאֹד.']]
            }
        });

        writeLesson(lessons, 'ok-recycle', {
            title: 'Ok recycle',
            level: 'basic',
            vocab: {
                items: [
                    { id: '1', word: 'קָם', meaning: 'gets up (m)', sound: 'kam', trap: 't', from: 'earlier', sentence: 'אֲנִי קָם. קָם מֻקְדָּם.' },
                    { id: '2', word: 'קָמָה', meaning: 'gets up (f)', sound: 'ka-MA', trap: 't', from: 'earlier', sentence: 'הִיא קָמָה. קָמָה עַכְשָׁו.' },
                    { id: '3', word: 'בֹּקֶר', meaning: 'morning', sound: 'BO-ker', trap: 't', from: 'earlier', sentence: 'בֹּקֶר טוֹב. בֹּקֶר שֶׁלִּי.' },
                    { id: '4', word: 'חָדָשׁ', meaning: 'new', sound: 'cha-DASH', trap: 't', sentence: 'זֶה חָדָשׁ. חָדָשׁ מְאֹד.' }
                ]
            },
            reading: {
                paragraphs: [[
                    { word: 'קָם' }, ' בַּ', { word: 'בֹּקֶר' }, '. ',
                    { word: 'קָמָה' }, ' גַּם. זֶה ', { word: 'חָדָשׁ' }, ' חָדָשׁ.'
                ]]
            }
        });

        const low = runValidate(lessons, 'low-recycle');
        assert.equal(low.status, 0, low.stderr || low.stdout);
        assert.ok(
            warnLines(low.stdout).some(w => /recycles 0|at least 3 words from earlier/i.test(w)),
            'expected recycling warning, got: ' + warnLines(low.stdout).join(' | ')
        );

        const ok = runValidate(lessons, 'ok-recycle');
        assert.equal(ok.status, 0, ok.stderr || ok.stdout);
        assert.ok(
            !warnLines(ok.stdout).some(w => /recycles \d+|at least 3 words from earlier/i.test(w)),
            'did not expect recycling warning, got: ' + warnLines(ok.stdout).join(' | ')
        );
    });

    it('ac-2: warns when a new word appears fewer than 2 times', () => {
        writeLesson(lessons, 'earlier', earlierVocab);
        writeJs(path.join(lessons, 'catalog.js'), 'HEBREW_CATALOG', [
            { slug: 'earlier', title: 'Earlier' },
            { slug: 'once-only', title: 'Once' }
        ]);
        writeLesson(lessons, 'once-only', {
            title: 'Once only',
            level: 'basic',
            vocab: {
                items: [
                    { id: '1', word: 'קָם', meaning: 'gets up (m)', sound: 'kam', trap: 't', from: 'earlier', sentence: 'אֲנִי קָם. קָם מֻקְדָּם.' },
                    { id: '2', word: 'קָמָה', meaning: 'gets up (f)', sound: 'ka-MA', trap: 't', from: 'earlier', sentence: 'הִיא קָמָה. קָמָה עַכְשָׁו.' },
                    { id: '3', word: 'בֹּקֶר', meaning: 'morning', sound: 'BO-ker', trap: 't', from: 'earlier', sentence: 'בֹּקֶר טוֹב. בֹּקֶר שֶׁלִּי.' },
                    // Listed once in vocab; sentence omitted → 0 encounters in corpus
                    { id: '4', word: 'נָדִיר', meaning: 'rare', sound: 'na-DIR', trap: 't' }
                ]
            },
            reading: {
                paragraphs: [['קָם קָמָה בֹּקֶר.']]
            }
        });

        const result = runValidate(lessons, 'once-only');
        assert.equal(result.status, 0, result.stderr || result.stdout);
        assert.ok(
            warnLines(result.stdout).some(w => /נָדִיר/.test(w) && /appears only/i.test(w)),
            'expected encounters warning, got: ' + warnLines(result.stdout).join(' | ')
        );
    });

    it('ac-3: warns with estimated coverage % when story is mostly unknown', () => {
        writeLesson(lessons, 'earlier', earlierVocab);
        writeJs(path.join(lessons, 'catalog.js'), 'HEBREW_CATALOG', [
            { slug: 'earlier', title: 'Earlier' },
            { slug: 'low-cover', title: 'Cover' }
        ]);
        writeLesson(lessons, 'low-cover', {
            title: 'Low cover',
            level: 'basic',
            vocab: {
                items: [
                    { id: '1', word: 'קָם', meaning: 'gets up (m)', sound: 'kam', trap: 't', from: 'earlier', sentence: 'אֲנִי קָם. קָם.' },
                    { id: '2', word: 'קָמָה', meaning: 'gets up (f)', sound: 'ka-MA', trap: 't', from: 'earlier', sentence: 'הִיא קָמָה. קָמָה.' },
                    { id: '3', word: 'בֹּקֶר', meaning: 'morning', sound: 'BO-ker', trap: 't', from: 'earlier', sentence: 'בֹּקֶר. בֹּקֶר.' },
                    { id: '4', word: 'חָדָשׁ', meaning: 'new', sound: 'cha-DASH', trap: 't', sentence: 'חָדָשׁ. חָדָשׁ.' }
                ]
            },
            reading: {
                paragraphs: [[
                    'אַבַּטִיחַ תַּפּוּחַ בָּנָנָה עֵנָב אֲגָס תֻּת אֲבַטִּיחַ מְלָפְפוֹן עַגְבָנִיָּה ',
                    { word: 'חָדָשׁ' }, ' חָדָשׁ.'
                ]]
            }
        });

        const result = runValidate(lessons, 'low-cover');
        assert.equal(result.status, 0, result.stderr || result.stdout);
        const coverage = warnLines(result.stdout).find(w => /Estimated known-word coverage/i.test(w));
        assert.ok(coverage, 'expected coverage warning, got: ' + warnLines(result.stdout).join(' | '));
        assert.match(coverage, /\d+%/);
        assert.ok(/below 90%/i.test(coverage));
    });

    it('ac-4: malformed glosses are errors; keys missing from story are warnings', () => {
        writeLesson(lessons, 'solo', {
            title: 'Gloss solo',
            level: 'basic',
            vocab: {
                items: [
                    { id: '1', word: 'קָם', meaning: 'gets up', sound: 'kam', trap: 't', sentence: 'קָם קָם.' }
                ]
            },
            reading: {
                paragraphs: [['אֲנִי ', { word: 'קָם' }, ' קָם.']],
                glosses: ['not-an-object']
            }
        });
        writeJs(path.join(lessons, 'catalog.js'), 'HEBREW_CATALOG', [{ slug: 'solo', title: 'Solo' }]);

        const bad = runValidate(lessons, 'solo');
        assert.notEqual(bad.status, 0, 'malformed glosses must fail');
        assert.match(bad.stderr || bad.stdout, /reading\.glosses must be an object/);

        writeLesson(lessons, 'stray-key', {
            title: 'Stray gloss',
            level: 'basic',
            vocab: {
                items: [
                    { id: '1', word: 'קָם', meaning: 'gets up', sound: 'kam', trap: 't', sentence: 'קָם קָם.' }
                ]
            },
            reading: {
                paragraphs: [['אֲנִי ', { word: 'קָם' }, ' קָם.']],
                glosses: {
                    'קָם': 'gets up',
                    'בַּיִת': 'house'
                }
            }
        });
        writeJs(path.join(lessons, 'catalog.js'), 'HEBREW_CATALOG', [{ slug: 'stray-key', title: 'Stray' }]);

        const stray = runValidate(lessons, 'stray-key');
        assert.equal(stray.status, 0, stray.stderr || stray.stdout);
        assert.ok(
            warnLines(stray.stdout).some(w => /glosses key "בַּיִת"/i.test(w) || /glosses key "בית"/i.test(w) || /בַּיִת/.test(w)),
            'expected stray gloss key warning, got: ' + warnLines(stray.stdout).join(' | ')
        );
    });

    it('ac-6: upsert_lesson reply head includes validation warnings', () => {
        const { formatUpsertHead } = require('../mcp/tools');
        const text = formatUpsertHead({
            created: true,
            path: 'worksheets/demo',
            warnings: [
                'Bring back at least 3 words from earlier lessons (set from: "<slug>" or reuse catalog/index words). This lesson recycles 0. Use list_words.',
                'Estimated known-word coverage of the story is 40% (below 90%). Estimate only; prefixes are not stripped.'
            ]
        });
        assert.match(text, /Created worksheets\/demo\. Validation passed\./);
        assert.match(text, /Fix these warnings and call upsert_lesson again/);
        assert.match(text, /recycles 0/);
        assert.match(text, /Estimated known-word coverage/);
    });
});

describe('validator nudges — existing published lessons (ac-5)', () => {
    const published = ['my-morning', 'shai-date-indirect-speech', 'anal-sex-vocab'];

    published.forEach(slug => {
        it('passes with exit 0 and no schema/gloss errors: ' + slug, () => {
            const result = spawnSync(process.execPath, [validate, slug], {
                cwd: root,
                encoding: 'utf8',
                env: Object.assign({}, process.env, { HEBREW_ROOT: root })
            });
            assert.equal(result.status, 0, result.stderr || result.stdout);
            assert.match(result.stdout, new RegExp('OK ' + slug));
            const errText = String(result.stderr || '');
            assert.doesNotMatch(errText, /reading\.glosses/);
        });
    });
});
