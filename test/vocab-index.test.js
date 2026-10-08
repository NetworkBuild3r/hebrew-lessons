/**
 * Vocab index generator tests — INIT-001/SPEC-002
 */
const { describe, it, before, after, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function writeWorksheet(dir, worksheet) {
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(
        path.join(dir, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );
}

function writeRawWorksheet(dir, code) {
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, 'worksheet.js'), code, 'utf8');
}

function loadIndex(file) {
    const sandbox = { window: {} };
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(file, 'utf8'), sandbox);
    return sandbox.window.HEBREW_VOCAB_INDEX;
}

describe('vocab-index generator', () => {
    let tmp;
    let lessons;
    let prevLessonsDir;
    let prevHebrewRoot;

    before(() => {
        prevLessonsDir = process.env.LESSONS_DIR;
        prevHebrewRoot = process.env.HEBREW_ROOT;
    });

    beforeEach(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-vocab-idx-'));
        lessons = path.join(tmp, 'worksheets');
        fs.mkdirSync(lessons, { recursive: true });
        process.env.LESSONS_DIR = lessons;
        process.env.HEBREW_ROOT = root;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/vocab-index')];
    });

    afterEach(() => {
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/vocab-index')];
    });

    after(() => {
        if (prevLessonsDir === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = prevLessonsDir;
        if (prevHebrewRoot === undefined) delete process.env.HEBREW_ROOT;
        else process.env.HEBREW_ROOT = prevHebrewRoot;
    });

    it('merges a shared word across two lessons and skips malformed / _-prefixed / require-touching', () => {
        writeWorksheet(path.join(lessons, 'alpha'), {
            id: 'alpha',
            title: 'Alpha',
            vocab: {
                items: [
                    { id: '1', word: 'קָמָה', meaning: 'gets up (f)', sound: 'ka-MA', sentence: 'הִיא קָמָה.' },
                    { id: '2', word: 'קָם', meaning: 'gets up (m)', sound: 'kam' }
                ]
            }
        });
        writeWorksheet(path.join(lessons, 'beta'), {
            id: 'beta',
            title: 'Beta',
            vocab: {
                items: [
                    { id: '1', word: 'קָמָה', meaning: 'gets up (f)', sound: 'ka-MA', sentence: 'הִיא קָמָה שׁוּב.' },
                    { id: '2', word: 'בֹּקֶר', meaning: 'morning', sound: 'BO-ker' }
                ]
            }
        });
        // Malformed: throws at eval time
        writeRawWorksheet(path.join(lessons, 'broken'), 'throw new Error("boom");\n');
        // _-prefixed must be ignored entirely
        writeWorksheet(path.join(lessons, '_hidden'), {
            id: '_hidden',
            vocab: { items: [{ id: '1', word: 'סוֹד', meaning: 'secret' }] }
        });
        // Hostile: touches require — sandbox has no require → skip
        writeRawWorksheet(
            path.join(lessons, 'hostile'),
            'require("fs");\nwindow.WORKSHEET = { vocab: { items: [{ word: "רַע", meaning: "bad" }] } };\n'
        );
        // Missing vocab section
        writeWorksheet(path.join(lessons, 'novocab'), {
            id: 'novocab',
            title: 'No vocab'
        });

        const { rebuildVocabIndex } = require('../server/vocab-index');
        const result = rebuildVocabIndex();
        assert.ok(fs.existsSync(result.path));
        assert.match(result.path, /_review[\\/]vocab-index\.js$/);

        const index = loadIndex(result.path);
        assert.ok(Array.isArray(index));

        const keys = index.map(e => e.key);
        assert.ok(keys.includes('קָמָה'));
        assert.ok(keys.includes('קָם'));
        assert.ok(keys.includes('בֹּקֶר'));
        assert.ok(!keys.includes('סוֹד'), '_-prefixed lesson must not contribute');
        assert.ok(!keys.includes('רַע'), 'require-touching lesson must not contribute');

        const kama = index.find(e => e.key === 'קָמָה');
        assert.equal(kama.lessons.length, 2);
        assert.ok(kama.lessons.includes('alpha'));
        assert.ok(kama.lessons.includes('beta'));
        assert.equal(kama.word, 'קָמָה');
        assert.equal(kama.meaning, 'gets up (f)');

        // Sorted by key (deterministic)
        const sorted = keys.slice().sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
        assert.deepEqual(keys, sorted);

        assert.ok(result.warnings.length >= 2, 'expected warnings for broken/hostile/novocab');
        const warningText = result.warnings.join('\n');
        assert.match(warningText, /broken/);
        assert.match(warningText, /hostile|novocab/);

        // Only the index file under _review — no other sibling writes outside the tree root
        const reviewDir = path.join(lessons, '_review');
        const reviewFiles = fs.readdirSync(reviewDir);
        assert.deepEqual(reviewFiles, ['vocab-index.js']);

        // File body uses JSON.stringify shape
        const body = fs.readFileSync(result.path, 'utf8');
        assert.match(body, /^window\.HEBREW_VOCAB_INDEX = /);
        assert.ok(!body.includes('ELEVENLABS'));
        assert.ok(!body.includes('process.env'));
    });

    it('attaches shared audio URL from the word-audio catalog when present', () => {
        const cache = path.join(lessons, '_audio', 'words');
        fs.mkdirSync(cache, { recursive: true });
        const clip = 'aabbccdd1122334455667788.mp3';
        fs.writeFileSync(path.join(cache, clip), 'fake');
        fs.writeFileSync(
            path.join(cache, 'catalog.js'),
            'window.HEBREW_WORD_AUDIO = Object.assign({}, window.HEBREW_WORD_AUDIO || {}, '
                + JSON.stringify({ קָמָה: '../_audio/words/' + clip }, null, 4) + ');\n',
            'utf8'
        );

        writeWorksheet(path.join(lessons, 'one'), {
            id: 'one',
            vocab: {
                items: [
                    { id: '1', word: 'קָמָה', meaning: 'gets up', sound: 'ka-MA' }
                ]
            }
        });

        const { rebuildVocabIndex } = require('../server/vocab-index');
        const result = rebuildVocabIndex();
        const index = loadIndex(result.path);
        assert.equal(index.length, 1);
        assert.equal(index[0].audio, '../_audio/words/' + clip);
    });

    it('rebuildVocabIndexSafe swallows builder failures', () => {
        // Point LESSONS_DIR at a file so readdirSync throws inside the builder.
        const bogey = path.join(tmp, 'not-a-dir');
        fs.writeFileSync(bogey, 'x');
        process.env.LESSONS_DIR = bogey;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/vocab-index')];
        const { rebuildVocabIndexSafe } = require('../server/vocab-index');
        const safe = rebuildVocabIndexSafe();
        assert.equal(safe.ok, false);
        assert.ok(safe.error);
    });

    it('upsert_lesson wiring calls rebuildVocabIndexSafe (source contract)', () => {
        const toolsSrc = fs.readFileSync(path.join(root, 'mcp', 'tools.js'), 'utf8');
        assert.match(toolsSrc, /rebuildVocabIndexSafe/);
        assert.match(toolsSrc, /require\('\.\.\/server\/vocab-index'\)/);
    });

    it('exactKey matches ADR-0001 NFC + whitespace collapse', () => {
        const { exactKey } = require('../server/vocab-index');
        assert.equal(exactKey('  קָמָה  '), 'קָמָה');
        assert.equal(exactKey('א  ב'), 'א ב');
        assert.equal(exactKey('קָמָה'.normalize('NFD')).normalize('NFC'), exactKey('קָמָה'));
    });
});

describe('vocab-index CLI against repo worksheets', () => {
    it('npm-run-equivalent build writes worksheets/_review/vocab-index.js with merged קָמָה', () => {
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/vocab-index')];
        delete process.env.LESSONS_DIR;

        const { rebuildVocabIndex } = require('../server/vocab-index');
        const result = rebuildVocabIndex({ lessonsDir: path.join(root, 'worksheets') });
        assert.ok(fs.existsSync(result.path));
        const index = loadIndex(result.path);
        assert.ok(Array.isArray(index));
        assert.ok(index.length > 0);

        const kama = index.find(e => e.key === 'קָמָה' || e.word === 'קָמָה');
        assert.ok(kama, 'expected קָמָה in repo index');
        assert.ok(Array.isArray(kama.lessons));
        // my-morning and anal-sex-vocab both teach קָמָה when present
        const hasMorning = kama.lessons.includes('my-morning');
        const hasAnal = kama.lessons.includes('anal-sex-vocab');
        assert.ok(hasMorning, 'my-morning should contribute קָמָה');
        if (fs.existsSync(path.join(root, 'worksheets', 'anal-sex-vocab', 'worksheet.js'))) {
            assert.ok(hasAnal, 'anal-sex-vocab should contribute קָמָה when published');
            assert.ok(kama.lessons.length >= 2);
        }
        if (kama.audio) {
            assert.match(kama.audio, /_audio\/words\//);
        }
    });
});
