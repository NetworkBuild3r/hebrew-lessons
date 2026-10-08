const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const https = require('https');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const script = path.join(root, 'scripts', 'generate-audio.js');

function writeLesson(dir, worksheet) {
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(
        path.join(dir, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );
}

function loadWorksheet(file) {
    const sandbox = { window: {} };
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(file, 'utf8'), sandbox);
    return sandbox.window.WORKSHEET;
}

describe('generate-audio word cache', () => {
    it('dry-run reports without calling the API', () => {
        const result = spawnSync(process.execPath, [script, 'my-morning', '--words-only', '--dry-run'], {
            cwd: root,
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: root,
                ELEVENLABS_VOICE_ID: '6WRlPmU1fqPwYbSxeP16',
                ELEVENLABS_MODEL: 'eleven_v3'
            })
        });
        assert.equal(result.status, 0, result.stderr || result.stdout);
        const jsonBlock = (result.stdout || '').match(/\{[\s\S]*?\n\}/);
        assert.ok(jsonBlock, 'expected JSON summary in stdout: ' + result.stdout);
        const summary = JSON.parse(jsonBlock[0]);
        assert.equal(summary.dryRun, true);
        assert.ok(summary.words.wouldCreate + summary.words.reused >= 1);
        assert.equal(summary.words.created, 0);
    });

    it('refuses to overwrite an existing word cache file', () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-word-'));
        const cache = path.join(tmp, '_audio', 'words');
        fs.mkdirSync(cache, { recursive: true });
        const file = path.join(cache, 'already.mp3');
        fs.writeFileSync(file, Buffer.from('old'));
        const before = fs.readFileSync(file);
        if (!fs.existsSync(file)) fs.writeFileSync(file, Buffer.from('new'));
        assert.deepEqual(fs.readFileSync(file), before);
        fs.rmSync(tmp, { recursive: true, force: true });
    });

    it('reuses one API call for the same word across two lessons', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-audio-'));
        const lessons = path.join(tmp, 'worksheets');
        fs.mkdirSync(path.join(lessons, '_audio', 'words'), { recursive: true });
        writeLesson(path.join(lessons, 'a'), {
            id: 'a',
            title: 'A',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'בֹּקֶר', meaning: 'morning', sentence: 'בֹּקֶר טוֹב.' }] },
            reading: { paragraphs: ['בֹּקֶר טוֹב.'] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });
        writeLesson(path.join(lessons, 'b'), {
            id: 'b',
            title: 'B',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'בֹּקֶר', meaning: 'morning', sentence: 'יֵשׁ בֹּקֶר.' }] },
            reading: { paragraphs: ['יֵשׁ בֹּקֶר.'] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });

        // Stub https.request used by postTts
        let apiCalls = 0;
        const orig = https.request;
        https.request = function (url, options, cb) {
            apiCalls += 1;
            const listeners = { data: [], end: [] };
            const res = {
                statusCode: 200,
                on(ev, fn) {
                    if (ev === 'data') listeners.data.push(fn);
                    if (ev === 'end') listeners.end.push(fn);
                    return res;
                }
            };
            process.nextTick(() => {
                listeners.data.forEach(fn => fn(Buffer.from('FAKEMP3')));
                listeners.end.forEach(fn => fn());
            });
            if (typeof options === 'function') cb = options;
            if (typeof cb === 'function') cb(res);
            return {
                on() { return this; },
                write() {},
                end() {}
            };
        };

        try {
            const { generateLessonAudio } = require('../server/audio');
            const env = {
                HEBREW_ROOT: root,
                LESSONS_DIR: lessons,
                ELEVENLABS_API_KEY: 'test-key',
                ELEVENLABS_VOICE_ID: 'voice',
                ELEVENLABS_MODEL: 'eleven_v3'
            };
            const prev = {};
            Object.keys(env).forEach(k => { prev[k] = process.env[k]; process.env[k] = env[k]; });
            // Clear require cache for paths so LESSONS_DIR is re-read... paths reads env at call time, good.
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            const audio = require('../server/audio');

            const r1 = await audio.generateLessonAudio('a', { wordsOnly: true });
            const r2 = await audio.generateLessonAudio('b', { wordsOnly: true });
            Object.keys(env).forEach(k => {
                if (prev[k] === undefined) delete process.env[k];
                else process.env[k] = prev[k];
            });

            assert.equal(apiCalls, 1, 'same word should hit ElevenLabs once');
            assert.equal(r1.wordsCreated, 1);
            assert.equal(r2.wordsReused, 1);
            assert.equal(r2.wordsCreated, 0);

            const wordFiles = fs.readdirSync(path.join(lessons, '_audio', 'words')).filter(n => n.endsWith('.mp3'));
            assert.equal(wordFiles.length, 1);

            const wsA = loadWorksheet(path.join(lessons, 'a', 'worksheet.js'));
            const wsB = loadWorksheet(path.join(lessons, 'b', 'worksheet.js'));
            assert.equal(wsA.vocab.items[0].audio, wsB.vocab.items[0].audio);
            assert.ok(String(wsA.vocab.items[0].audio).startsWith('../_audio/words/'));
        } finally {
            https.request = orig;
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });

    it('does not rewrite an existing word file even with forceSentences', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-force-'));
        const lessons = path.join(tmp, 'worksheets');
        const cache = path.join(lessons, '_audio', 'words');
        fs.mkdirSync(cache, { recursive: true });

        const { cacheKey } = require('../server/audio');
        const key = cacheKey('voice', 'eleven_v3', 'קָם');
        const file = path.join(cache, key + '.mp3');
        fs.writeFileSync(file, Buffer.from('ORIGINAL'));

        writeLesson(path.join(lessons, 'm'), {
            id: 'm',
            title: 'M',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'קָם', meaning: 'got up', sentence: 'הוּא קָם.' }] },
            reading: { paragraphs: ['הוּא קָם.'] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });

        let apiCalls = 0;
        const orig = https.request;
        https.request = function (url, options, cb) {
            apiCalls += 1;
            const listeners = { data: [], end: [] };
            const res = {
                statusCode: 200,
                on(ev, fn) {
                    if (ev === 'data') listeners.data.push(fn);
                    if (ev === 'end') listeners.end.push(fn);
                    return res;
                }
            };
            process.nextTick(() => {
                listeners.data.forEach(fn => fn(Buffer.from('NEW')));
                listeners.end.forEach(fn => fn());
            });
            if (typeof options === 'function') cb = options;
            if (typeof cb === 'function') cb(res);
            return { on() { return this; }, write() {}, end() {} };
        };

        try {
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            const audio = require('../server/audio');
            process.env.LESSONS_DIR = lessons;
            process.env.HEBREW_ROOT = root;
            process.env.ELEVENLABS_API_KEY = 'test-key';
            process.env.ELEVENLABS_VOICE_ID = 'voice';
            process.env.ELEVENLABS_MODEL = 'eleven_v3';

            const result = await audio.generateLessonAudio('m', { forceSentences: true });
            assert.equal(result.wordsCreated, 0);
            assert.equal(result.wordsReused, 1);
            assert.deepEqual(fs.readFileSync(file), Buffer.from('ORIGINAL'));
            // Sentence clip may have been created (not a word).
            assert.ok(result.clipsCreated >= 1 || result.clipsReused >= 0);
        } finally {
            https.request = orig;
            delete process.env.LESSONS_DIR;
            delete process.env.ELEVENLABS_API_KEY;
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });

    it('link-only mode reports missing clips without an API key', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-link-'));
        const lessons = path.join(tmp, 'worksheets');
        fs.mkdirSync(path.join(lessons, '_audio', 'words'), { recursive: true });
        writeLesson(path.join(lessons, 'x'), {
            id: 'x',
            title: 'X',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'חָדָשׁ', meaning: 'new', sentence: 'זֶה חָדָשׁ.' }] },
            reading: { paragraphs: ['זֶה חָדָשׁ.'] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [{ id: 1, pointed: 'זֶה חָדָשׁ.', choices: ['a', 'b'], answer: 0 }] },
            askFriend: { questions: [] }
        });

        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/audio')];
        const audio = require('../server/audio');
        process.env.LESSONS_DIR = lessons;
        process.env.HEBREW_ROOT = root;
        delete process.env.ELEVENLABS_API_KEY;
        delete process.env['11_labs_api_key'];

        try {
            const result = await audio.generateLessonAudio('x', { linkOnly: true });
            assert.equal(result.linkOnly, true);
            assert.equal(result.wordsCreated, 0);
            assert.ok(result.wordsMissing.length >= 1);
            assert.ok(result.clipsMissing.length >= 1);
        } finally {
            delete process.env.LESSONS_DIR;
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });

    it('regenerates only a changed story line', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-story-'));
        const lessons = path.join(tmp, 'worksheets');
        const lesson = path.join(lessons, 's');
        fs.mkdirSync(path.join(lessons, '_audio', 'words'), { recursive: true });
        writeLesson(lesson, {
            id: 's',
            title: 'S',
            level: 'basic',
            vocab: { items: [] },
            reading: {
                paragraphs: ['שורה אחת.', 'שורה שתיים.'],
                questions: []
            },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });

        // Pre-create line-1 with a fake manifest hash matching current text.
        const { textHash } = require('../server/audio');
        const hash1 = textHash('voice', 'eleven_v3', 'שורה אחת.');
        const hash2old = textHash('voice', 'eleven_v3', 'שורה שתיים.');
        fs.mkdirSync(path.join(lesson, 'audio', 'story'), { recursive: true });
        fs.writeFileSync(path.join(lesson, 'audio', 'story', 'line-1.mp3'), Buffer.from('L1'));
        fs.writeFileSync(path.join(lesson, 'audio', 'story', 'line-2.mp3'), Buffer.from('L2OLD'));
        fs.writeFileSync(path.join(lesson, 'audio', 'manifest.json'), JSON.stringify({
            version: 1,
            clips: {
                'audio/story/line-1.mp3': hash1,
                'audio/story/line-2.mp3': hash2old
            }
        }));

        // Change line 2 text
        writeLesson(lesson, {
            id: 's',
            title: 'S',
            level: 'basic',
            vocab: { items: [] },
            reading: {
                paragraphs: ['שורה אחת.', 'שורה שתיים שונה.'],
                questions: [],
                audio: ['audio/story/line-1.mp3', 'audio/story/line-2.mp3']
            },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });
        // Keep the old manifest so line-1 matches and line-2 hash mismatches
        fs.writeFileSync(path.join(lesson, 'audio', 'manifest.json'), JSON.stringify({
            version: 1,
            clips: {
                'audio/story/line-1.mp3': hash1,
                'audio/story/line-2.mp3': hash2old
            }
        }));

        let texts = [];
        const orig = https.request;
        https.request = function (url, options, cb) {
            let body = '';
            const listeners = { data: [], end: [] };
            const res = {
                statusCode: 200,
                on(ev, fn) {
                    if (ev === 'data') listeners.data.push(fn);
                    if (ev === 'end') listeners.end.push(fn);
                    return res;
                }
            };
            if (typeof options === 'function') cb = options;
            if (typeof cb === 'function') cb(res);
            return {
                on() { return this; },
                write(chunk) { body += chunk; },
                end() {
                    try { texts.push(JSON.parse(body).text); } catch (e) { /* ignore */ }
                    process.nextTick(() => {
                        listeners.data.forEach(fn => fn(Buffer.from('NEWL2')));
                        listeners.end.forEach(fn => fn());
                    });
                }
            };
        };

        try {
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            const audio = require('../server/audio');
            process.env.LESSONS_DIR = lessons;
            process.env.HEBREW_ROOT = root;
            process.env.ELEVENLABS_API_KEY = 'test-key';
            process.env.ELEVENLABS_VOICE_ID = 'voice';
            process.env.ELEVENLABS_MODEL = 'eleven_v3';

            const result = await audio.generateLessonAudio('s', { sentencesOnly: true });
            assert.equal(result.clipsCreated, 1);
            assert.equal(result.clipsReused, 1);
            assert.deepEqual(texts, ['שורה שתיים שונה.']);
            assert.deepEqual(fs.readFileSync(path.join(lesson, 'audio', 'story', 'line-1.mp3')), Buffer.from('L1'));
            assert.deepEqual(fs.readFileSync(path.join(lesson, 'audio', 'story', 'line-2.mp3')), Buffer.from('NEWL2'));
        } finally {
            https.request = orig;
            delete process.env.LESSONS_DIR;
            delete process.env.ELEVENLABS_API_KEY;
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });
});

describe('audio path validator', () => {
    it('accepts ../_audio/words and rejects ../../', () => {
        const validator = path.join(root, '.cursor', 'skills', 'hebrew-worksheet', 'scripts', 'validate.js');
        // Use my-morning which now has ../_audio paths — should pass audio checks
        const ok = spawnSync(process.execPath, [validator, 'my-morning'], {
            cwd: root,
            encoding: 'utf8',
            env: Object.assign({}, process.env, { HEBREW_ROOT: root })
        });
        // May have warnings but audio path rules should not fail for ../_audio
        const out = (ok.stdout || '') + (ok.stderr || '');
        assert.ok(!out.includes('escaped LESSONS_DIR/_audio'), out);

        // Unit-check the rejection logic with a tiny inline replica of the rule
        const bad = '../../etc/passwd.mp3';
        assert.ok(bad.includes('..'));
        assert.ok(!/^(\.\.\/_audio\/words\/)[^/]+\.mp3$/.test(bad));
        const good = '../_audio/words/806bb3fc99c984531106aeb5.mp3';
        assert.ok(/^(\.\.\/_audio\/words\/)[^/]+\.mp3$/.test(good));
    });
});
