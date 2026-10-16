/**
 * Sound for one word on request (server/word-audio.js). A fake voice stands in for
 * ElevenLabs: no network, no key. Every test works in its own temporary folder.
 */
'use strict';

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const wordAudio = require('../server/word-audio');
const audio = require('../server/audio');

const COFFEE = 'קָפֶה';

describe('word audio on request', () => {
    let dir;

    beforeEach(() => {
        dir = fs.mkdtempSync(path.join(os.tmpdir(), 'word-audio-'));
    });

    afterEach(() => {
        fs.rmSync(dir, { recursive: true, force: true });
    });

    function fakeVoice(counter) {
        return async (key, voiceId, model, word) => {
            counter.calls.push({ key, voiceId, model, word });
            return Buffer.from('mp3:' + word);
        };
    }

    it('refuses anything that is not a short Hebrew word before it reaches a paid service', () => {
        const bad = ['', '   ', 'coffee', 'קפה coffee', 'קפה<script>', 'א'.repeat(41), 5, null, '123'];
        bad.forEach((value) => {
            assert.throws(() => wordAudio.cleanWord(value), (err) => err.code === 'bad_word', String(value));
        });
        assert.equal(wordAudio.cleanWord('  אַחַר   כָּךְ '), 'אַחַר כָּךְ');
        assert.equal(wordAudio.cleanWord('עֵץ-חַיִּים'), 'עֵץ-חַיִּים');
    });

    it('makes a clip once, saves it where the lesson plan can point to it, and reuses it after', async () => {
        const counter = { calls: [] };
        const first = await wordAudio.ensureWordClip(COFFEE, { cacheDir: dir, tts: fakeVoice(counter), apiKey: 'k' });
        assert.equal(first.created, true);
        assert.match(first.audioPath, /^\.\.\/_audio\/words\/[a-f0-9]{24}\.mp3$/);
        assert.equal(first.url, '/worksheets/_audio/words/' + path.basename(first.audioPath));
        assert.equal(fs.readFileSync(path.join(dir, path.basename(first.audioPath)), 'utf8'), 'mp3:' + COFFEE);
        assert.equal(counter.calls.length, 1);
        assert.equal(counter.calls[0].word, COFFEE);

        const again = await wordAudio.ensureWordClip(COFFEE, { cacheDir: dir, tts: fakeVoice(counter), apiKey: 'k' });
        assert.equal(again.created, false);
        assert.equal(again.audioPath, first.audioPath);
        assert.equal(counter.calls.length, 1, 'a word that has a clip is never requested again');

        assert.equal(wordAudio.findWordClip(COFFEE, { cacheDir: dir }).audioPath, first.audioPath);
        assert.equal(wordAudio.findWordClip('חָלָב', { cacheDir: dir }), null);

        // The browser catalog learns the word too.
        const catalog = fs.readFileSync(path.join(dir, 'catalog.js'), 'utf8');
        assert.ok(catalog.includes(COFFEE));
        assert.ok(catalog.includes(path.basename(first.audioPath)));
    });

    it('answers two requests for the same new word with one call to the voice', async () => {
        const counter = { calls: [] };
        const opts = { cacheDir: dir, tts: fakeVoice(counter), apiKey: 'k' };
        const [a, b] = await Promise.all([
            wordAudio.ensureWordClip(COFFEE, opts),
            wordAudio.ensureWordClip(COFFEE, opts)
        ]);
        assert.equal(counter.calls.length, 1);
        assert.equal(a.audioPath, b.audioPath);
    });

    it('says what went wrong, and leaves nothing behind, when the voice fails or sends nothing', async () => {
        await assert.rejects(
            wordAudio.ensureWordClip(COFFEE, {
                cacheDir: dir, apiKey: 'k', tts: async () => { throw new Error('ElevenLabs HTTP 401: nope'); }
            }),
            (err) => err.code === 'tts_failed' && /401/.test(err.message)
        );
        await assert.rejects(
            wordAudio.ensureWordClip(COFFEE, { cacheDir: dir, apiKey: 'k', tts: async () => Buffer.alloc(0) }),
            (err) => err.code === 'tts_failed'
        );
        await assert.rejects(
            wordAudio.ensureWordClip(COFFEE, { cacheDir: dir, apiKey: '' }),
            (err) => err.code === 'no_key'
        );
        assert.deepEqual(fs.readdirSync(dir), []);
        assert.deepEqual(wordAudio.listWordClips({ cacheDir: dir }), []);
    });

    it('lists each word once, sorted by its letters, and skips a clip whose file is gone', async () => {
        const counter = { calls: [] };
        const opts = { cacheDir: dir, tts: fakeVoice(counter), apiKey: 'k' };
        const words = ['שָׁלוֹם', 'אָב', COFFEE, 'בַּיִת'];
        for (const word of words) await wordAudio.ensureWordClip(word, opts);
        const made = wordAudio.findWordClip('בַּיִת', { cacheDir: dir });
        fs.unlinkSync(path.join(dir, path.basename(made.audioPath)));

        const clips = wordAudio.listWordClips({ cacheDir: dir });
        assert.deepEqual(clips.map((c) => c.word), ['אָב', COFFEE, 'שָׁלוֹם']);
        clips.forEach((clip) => assert.match(clip.audioPath, /^\.\.\/_audio\/words\/[a-f0-9]{24}\.mp3$/));
    });

    it('reuses a clip that an older lesson made for the same word with any voice', async () => {
        const manifest = audio.readWordManifest(dir);
        fs.writeFileSync(path.join(dir, 'oldclipoldclipoldclip0000.mp3'), Buffer.from('old'));
        manifest.entries['voice|model|' + COFFEE] = { text: COFFEE, file: 'oldclipoldclipoldclip0000.mp3' };
        audio.writeWordManifest(dir, manifest);
        const counter = { calls: [] };
        const clip = await wordAudio.ensureWordClip(COFFEE.normalize('NFD'), {
            cacheDir: dir, tts: fakeVoice(counter), apiKey: 'k'
        });
        assert.equal(clip.created, false);
        assert.equal(clip.audioPath, '../_audio/words/oldclipoldclipoldclip0000.mp3');
        assert.equal(counter.calls.length, 0);
    });
});
