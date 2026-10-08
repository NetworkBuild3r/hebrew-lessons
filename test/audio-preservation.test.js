/**
 * Audio preservation guard + checksum inventory (INIT-002/SPEC-002).
 * Temp LESSONS_DIR, fake TTS, no network.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const https = require('https');
const os = require('os');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const inventoryScript = path.join(root, 'scripts', 'audio-inventory.js');

function writeLesson(dir, worksheet) {
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(
        path.join(dir, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );
}

function sha256Sync(bufOrPath) {
    if (Buffer.isBuffer(bufOrPath)) {
        return crypto.createHash('sha256').update(bufOrPath).digest('hex');
    }
    return crypto.createHash('sha256').update(fs.readFileSync(bufOrPath)).digest('hex');
}

function stubHttps(payloadFactory) {
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
                let text = '';
                try { text = JSON.parse(body).text; } catch (e) { /* ignore */ }
                const buf = Buffer.from(
                    typeof payloadFactory === 'function' ? payloadFactory(text) : payloadFactory
                );
                process.nextTick(() => {
                    listeners.data.forEach(fn => fn(buf));
                    listeners.end.forEach(fn => fn());
                });
            }
        };
    };
    return () => { https.request = orig; };
}

function withAudioModule(env, fn) {
    const prev = {};
    Object.keys(env).forEach(k => {
        prev[k] = process.env[k];
        if (env[k] === undefined) delete process.env[k];
        else process.env[k] = env[k];
    });
    delete require.cache[require.resolve('../server/paths')];
    delete require.cache[require.resolve('../server/audio')];
    const audio = require('../server/audio');
    return Promise.resolve()
        .then(() => fn(audio))
        .finally(() => {
            Object.keys(env).forEach(k => {
                if (prev[k] === undefined) delete process.env[k];
                else process.env[k] = prev[k];
            });
            delete require.cache[require.resolve('../server/paths')];
            delete require.cache[require.resolve('../server/audio')];
        });
}

describe('audio preservation guard (INIT-002/SPEC-002)', () => {
    it('ac-1: regenerating a changed sentence archives prior bytes; second run skips duplicate archive', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-pres-'));
        const lessons = path.join(tmp, 'worksheets');
        const lesson = path.join(lessons, 'p');
        fs.mkdirSync(path.join(lessons, '_audio', 'words'), { recursive: true });

        writeLesson(lesson, {
            id: 'p',
            title: 'P',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'שָׁלוֹם', meaning: 'hello', sentence: 'שָׁלוֹם עוֹלָם.' }] },
            reading: { paragraphs: [], questions: [] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });

        const oldBytes = Buffer.from('OLDCLIP-BYTES');
        const clipRel = path.join('audio', 'sentences', 'vocab-1.mp3');
        const clipFull = path.join(lesson, clipRel);
        fs.mkdirSync(path.dirname(clipFull), { recursive: true });
        fs.writeFileSync(clipFull, oldBytes);
        const oldHash = sha256Sync(oldBytes);

        const { textHash } = require('../server/audio');
        const oldTextHash = textHash('voice', 'eleven_v3', 'שָׁלוֹם עוֹלָם.');
        fs.writeFileSync(path.join(lesson, 'audio', 'manifest.json'), JSON.stringify({
            version: 1,
            clips: { 'audio/sentences/vocab-1.mp3': oldTextHash }
        }));

        // Change sentence text so regeneration is required.
        writeLesson(lesson, {
            id: 'p',
            title: 'P',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'שָׁלוֹם', meaning: 'hello', sentence: 'שָׁלוֹם חָדָשׁ.' }] },
            reading: { paragraphs: [], questions: [] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });
        fs.writeFileSync(path.join(lesson, 'audio', 'manifest.json'), JSON.stringify({
            version: 1,
            clips: { 'audio/sentences/vocab-1.mp3': oldTextHash }
        }));

        const restoreHttps = stubHttps('NEWCLIP-BYTES');
        try {
            await withAudioModule({
                HEBREW_ROOT: root,
                LESSONS_DIR: lessons,
                ELEVENLABS_API_KEY: 'test-key',
                ELEVENLABS_VOICE_ID: 'voice',
                ELEVENLABS_MODEL: 'eleven_v3'
            }, async (audio) => {
                const r1 = await audio.generateLessonAudio('p', { sentencesOnly: true });
                assert.equal(r1.clipsCreated, 1);
                assert.deepEqual(fs.readFileSync(clipFull), Buffer.from('NEWCLIP-BYTES'));

                const archiveName = 'vocab-1.' + oldHash.slice(0, 12) + '.mp3';
                const archivePath = path.join(lesson, 'audio', '_archive', archiveName);
                assert.ok(fs.existsSync(archivePath), 'expected archive at ' + archivePath);
                assert.equal(sha256Sync(archivePath), oldHash);

                // Force overwrite twice: archive of identical bytes must not duplicate. INIT-002/SPEC-002
                fs.writeFileSync(path.join(lesson, 'audio', 'manifest.json'), JSON.stringify({
                    version: 1,
                    clips: { 'audio/sentences/vocab-1.mp3': 'deadbeef' }
                }));
                await audio.generateLessonAudio('p', { sentencesOnly: true, forceSentences: true });
                fs.writeFileSync(path.join(lesson, 'audio', 'manifest.json'), JSON.stringify({
                    version: 1,
                    clips: { 'audio/sentences/vocab-1.mp3': 'deadbeef' }
                }));
                await audio.generateLessonAudio('p', { sentencesOnly: true, forceSentences: true });
                const afterArchive = fs.readdirSync(path.join(lesson, 'audio', '_archive'));
                const oldCopies = afterArchive.filter(n => n.startsWith('vocab-1.' + oldHash.slice(0, 12)));
                assert.equal(oldCopies.length, 1, 'identical old bytes must not create a second archive');
                const newHash = sha256Sync(Buffer.from('NEWCLIP-BYTES'));
                const newCopies = afterArchive.filter(n => n.startsWith('vocab-1.' + newHash.slice(0, 12)));
                assert.equal(newCopies.length, 1, 'identical new bytes must not duplicate archive');
                void textHash;
            });
        } finally {
            restoreHttps();
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });

    it('ac-3: legacy shared/audio-cache migration only unlinks after verified matching dest', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-mig-'));
        const lessons = path.join(tmp, 'worksheets');
        const legacyWords = path.join(tmp, 'shared', 'audio-cache', 'words');
        fs.mkdirSync(path.join(lessons, '_audio', 'words'), { recursive: true });
        fs.mkdirSync(legacyWords, { recursive: true });

        const matching = Buffer.from('MATCHING-MP3');
        const conflict = Buffer.from('LEGACY-ONLY');
        const destConflict = Buffer.from('DEST-DIFFERENT');

        fs.writeFileSync(path.join(legacyWords, 'a.mp3'), matching);
        fs.writeFileSync(path.join(legacyWords, 'b.mp3'), conflict);
        fs.writeFileSync(path.join(lessons, '_audio', 'words', 'b.mp3'), destConflict);
        fs.writeFileSync(path.join(legacyWords, 'manifest.json'), JSON.stringify({ version: 1, entries: {} }));

        try {
            await withAudioModule({
                HEBREW_ROOT: tmp,
                LESSONS_DIR: lessons
            }, async (audio) => {
                const result = audio.migrateLegacyWordCache();
                assert.ok(result.moved >= 1);
                // a.mp3 moved (legacy gone, dest present, same bytes)
                assert.ok(!fs.existsSync(path.join(legacyWords, 'a.mp3')));
                assert.deepEqual(fs.readFileSync(path.join(lessons, '_audio', 'words', 'a.mp3')), matching);
                // b.mp3 conflict: dest unchanged, legacy retained (not deleted)
                assert.ok(fs.existsSync(path.join(legacyWords, 'b.mp3')), 'mismatched legacy must not be deleted');
                assert.deepEqual(fs.readFileSync(path.join(lessons, '_audio', 'words', 'b.mp3')), destConflict);
            });
        } finally {
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });

    it('ac-2/ac-4: inventory outside LESSONS_DIR; verify detects loss and accepts archive', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-inv-'));
        const lessons = path.join(tmp, 'worksheets');
        const lesson = path.join(lessons, 'z');
        const dataDir = path.join(tmp, 'data', 'audio-inventory');
        fs.mkdirSync(path.join(lesson, 'audio', 'sentences'), { recursive: true });
        fs.mkdirSync(path.join(lessons, '_audio', 'words'), { recursive: true });
        fs.mkdirSync(dataDir, { recursive: true });

        const clip = path.join(lesson, 'audio', 'sentences', 'friend-1.mp3');
        const word = path.join(lessons, '_audio', 'words', 'w.mp3');
        fs.writeFileSync(clip, Buffer.from('CLIP-A'));
        fs.writeFileSync(word, Buffer.from('WORD-A'));
        const clipHash = sha256Sync(clip);

        const outFile = path.join(dataDir, 'test-inv.json');
        const env = Object.assign({}, process.env, {
            HEBREW_ROOT: tmp,
            LESSONS_DIR: lessons
        });

        try {
            const inv = spawnSync(process.execPath, [inventoryScript, 'inventory', '--out', outFile], {
                cwd: root,
                encoding: 'utf8',
                env
            });
            assert.equal(inv.status, 0, inv.stderr || inv.stdout);
            assert.ok(fs.existsSync(outFile));
            const payload = JSON.parse(fs.readFileSync(outFile, 'utf8'));
            assert.equal(payload.count, 2);
            assert.ok(!('content' in (payload.files[0] || {})));
            assert.ok(!('data' in (payload.files[0] || {})));
            payload.files.forEach(f => {
                assert.ok(f.path && typeof f.bytes === 'number' && f.sha256);
                assert.equal(Object.keys(f).sort().join(','), 'bytes,path,sha256');
            });
            // Output is under data/, not under LESSONS_DIR
            assert.ok(outFile.replace(/\\/g, '/').includes('/data/'));
            assert.ok(!outFile.startsWith(lessons));

            const ok = spawnSync(process.execPath, [inventoryScript, 'verify', '--against', outFile], {
                cwd: root,
                encoding: 'utf8',
                env
            });
            assert.equal(ok.status, 0, ok.stderr || ok.stdout);
            assert.match(ok.stdout, /2 recordings/);

            // Delete clip with no archive → verify fails and names the file
            fs.unlinkSync(clip);
            const fail = spawnSync(process.execPath, [inventoryScript, 'verify', '--against', outFile], {
                cwd: root,
                encoding: 'utf8',
                env
            });
            assert.notEqual(fail.status, 0);
            const failOut = (fail.stderr || '') + (fail.stdout || '');
            assert.match(failOut, /friend-1\.mp3/);

            // Restore bytes only in _archive → verify passes
            const archiveDir = path.join(lesson, 'audio', '_archive');
            fs.mkdirSync(archiveDir, { recursive: true });
            fs.writeFileSync(
                path.join(archiveDir, 'friend-1.' + clipHash.slice(0, 12) + '.mp3'),
                Buffer.from('CLIP-A')
            );
            const passArch = spawnSync(process.execPath, [inventoryScript, 'verify', '--against', outFile], {
                cwd: root,
                encoding: 'utf8',
                env
            });
            assert.equal(passArch.status, 0, passArch.stderr || passArch.stdout);
        } finally {
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });

    it('word clips are never overwritten (existing behavior)', async () => {
        const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-word-'));
        const lessons = path.join(tmp, 'worksheets');
        const cache = path.join(lessons, '_audio', 'words');
        fs.mkdirSync(cache, { recursive: true });

        delete require.cache[require.resolve('../server/audio')];
        const { cacheKey } = require('../server/audio');
        const key = cacheKey('voice', 'eleven_v3', 'קָם');
        const file = path.join(cache, key + '.mp3');
        fs.writeFileSync(file, Buffer.from('ORIGINAL-WORD'));

        writeLesson(path.join(lessons, 'm'), {
            id: 'm',
            title: 'M',
            level: 'basic',
            vocab: { items: [{ id: 1, word: 'קָם', meaning: 'got up', sentence: 'הוּא קָם.' }] },
            reading: { paragraphs: [], questions: [] },
            grammar: { rules: [{ title: 't', english: 'e' }], items: [] },
            askFriend: { questions: [] }
        });

        const restoreHttps = stubHttps('SHOULD-NOT-WRITE');
        try {
            await withAudioModule({
                HEBREW_ROOT: root,
                LESSONS_DIR: lessons,
                ELEVENLABS_API_KEY: 'test-key',
                ELEVENLABS_VOICE_ID: 'voice',
                ELEVENLABS_MODEL: 'eleven_v3'
            }, async (audio) => {
                await audio.generateLessonAudio('m', { forceSentences: true });
                assert.deepEqual(fs.readFileSync(file), Buffer.from('ORIGINAL-WORD'));
                // Word cache itself has no _archive; sentence may be new (no prior to archive).
                assert.ok(!fs.existsSync(path.join(cache, '_archive')));
            });
        } finally {
            restoreHttps();
            fs.rmSync(tmp, { recursive: true, force: true });
        }
    });
});

void vm;
