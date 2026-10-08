const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

describe('shared word audio catalog', () => {
    it('maps קָם to the permanent shared clip', () => {
        const { rebuildWordCatalog } = require('../server/audio');
        rebuildWordCatalog();
        const file = path.join(root, 'worksheets', '_audio', 'words', 'catalog.js');
        assert.ok(fs.existsSync(file));
        const code = fs.readFileSync(file, 'utf8');
        assert.match(code, /HEBREW_WORD_AUDIO/);
        assert.match(code, /קָם/);
        assert.match(code, /\.\.\/_audio\/words\/806bb3fc99c984531106aeb5\.mp3/);
        assert.ok(fs.existsSync(path.join(root, 'shared', 'word-audio-catalog.js')));
    });

    it('link-only attaches an existing shared word into a new lesson without API calls', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-link-word-'));
        const lessons = path.join(tmp, 'worksheets');
        const cache = path.join(lessons, '_audio', 'words');
        fs.mkdirSync(cache, { recursive: true });

        // Seed the shared cache with קָם from the real project file.
        const real = path.join(root, 'worksheets', '_audio', 'words', '806bb3fc99c984531106aeb5.mp3');
        fs.copyFileSync(real, path.join(cache, '806bb3fc99c984531106aeb5.mp3'));
        fs.writeFileSync(path.join(cache, 'manifest.json'), JSON.stringify({
            version: 1,
            voiceId: 'voice',
            model: 'eleven_v3',
            entries: {
                'voice|eleven_v3|קָם': {
                    text: 'קָם',
                    file: '806bb3fc99c984531106aeb5.mp3',
                    voiceId: 'voice',
                    model: 'eleven_v3'
                }
            }
        }, null, 2));

        const lessonDir = path.join(lessons, 'new-lesson');
        fs.mkdirSync(lessonDir, { recursive: true });
        fs.writeFileSync(path.join(lessonDir, 'worksheet.js'), 'window.WORKSHEET = '
            + JSON.stringify({
                id: 'new-lesson',
                title: 'New',
                level: 'basic',
                vocab: { items: [{ id: 1, word: 'קָם', meaning: 'gets up' }] },
                reading: { paragraphs: ['קָם.'] },
                grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
                askFriend: { questions: [] }
            }, null, 4) + ';\n');

        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/audio')];
        process.env.LESSONS_DIR = lessons;
        process.env.HEBREW_ROOT = root;
        delete process.env.ELEVENLABS_API_KEY;
        delete process.env['11_labs_api_key'];

        try {
            const audio = require('../server/audio');
            const result = await audio.generateLessonAudio('new-lesson', { wordsOnly: true, linkOnly: true });
            assert.equal(result.wordsCreated, 0);
            assert.equal(result.wordsReused, 1);
            const sandbox = { window: {} };
            vm.createContext(sandbox);
            vm.runInContext(fs.readFileSync(path.join(lessonDir, 'worksheet.js'), 'utf8'), sandbox);
            assert.equal(
                sandbox.window.WORKSHEET.vocab.items[0].audio,
                '../_audio/words/806bb3fc99c984531106aeb5.mp3'
            );
        } finally {
            delete process.env.LESSONS_DIR;
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });
});
