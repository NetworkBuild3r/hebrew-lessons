/**
 * Sound for one Hebrew word, on request.
 *
 * Word clips live in one shared folder (`_audio/words`) and are keyed by the exact
 * pointed text, so a word made once is reused by every lesson and never overwritten.
 * `ensureWordClip` reuses a clip that already exists and only asks ElevenLabs when there
 * is none. The clip's `audioPath` is the same `../_audio/words/<name>.mp3` shape the
 * lesson-plan schema accepts, so it can be saved on a word with a normal field edit.
 *
 * This module never reads or returns the API key.
 */
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const audio = require('./audio');
const { wordAudioDir } = require('./paths');

const MAX_CHARS = 40;
const MAX_LIST = 5000;
const URL_PREFIX = '/worksheets/_audio/words/';
// Hebrew letters, points, cantillation, maqaf, geresh/gershayim, and spaces. Nothing else.
const ALLOWED = /^[\u0590-\u05FF \u05BE'"\u05F3\u05F4-]+$/;
const LETTER = /[\u05D0-\u05EA]/;

class WordAudioError extends Error {
    constructor(code, message) {
        super(message || code);
        this.name = 'WordAudioError';
        this.code = code;
    }
}

/** @type {Map<string, Promise<object>>} one request per word at a time. */
const inflight = new Map();

/**
 * The word as stored: one space between parts, NFC. Anything that is not a short
 * Hebrew word or phrase is refused before it can reach a paid service.
 * @param {*} text
 * @returns {string}
 */
function cleanWord(text) {
    const word = typeof text === 'string' ? text.replace(/\s+/g, ' ').trim().normalize('NFC') : '';
    if (!word || word.length > MAX_CHARS || !LETTER.test(word) || !ALLOWED.test(word)) {
        throw new WordAudioError('bad_word', 'A word is Hebrew letters, up to ' + MAX_CHARS + ' characters.');
    }
    return word;
}

function clipOf(word, fileName, created) {
    return {
        word: word,
        audioPath: audio.SITE_WORD_PREFIX + fileName,
        url: URL_PREFIX + fileName,
        created: created === true
    };
}

/**
 * The clip already made for this word, or null. Never calls a service.
 * @param {string} text
 * @param {{ cacheDir?: string }} [options]
 * @returns {{ word: string, audioPath: string, url: string, created: boolean }|null}
 */
function findWordClip(text, options) {
    const word = cleanWord(text);
    const dir = (options && options.cacheDir) || wordAudioDir();
    const found = audio.findWordFileByText(dir, audio.readWordManifest(dir), word);
    return found ? clipOf(word, found.fileName, false) : null;
}

/**
 * The clip for this word: reuse it, or make it once.
 * @param {string} text
 * @param {{ cacheDir?: string, tts?: Function, apiKey?: string }} [options] tests inject `tts`.
 * @returns {Promise<{ word: string, audioPath: string, url: string, created: boolean }>}
 */
function ensureWordClip(text, options) {
    const opts = options || {};
    const word = cleanWord(text);
    const dir = opts.cacheDir || wordAudioDir();
    const existing = audio.findWordFileByText(dir, audio.readWordManifest(dir), word);
    if (existing) return Promise.resolve(clipOf(word, existing.fileName, false));

    const flight = dir + '\0' + word;
    if (inflight.has(flight)) return inflight.get(flight);

    const pending = (async () => {
        const tts = opts.tts || audio.postTts;
        let apiKey = opts.apiKey;
        if (apiKey === undefined) {
            audio.loadEnvFile();
            apiKey = audio.apiKeyFromEnv();
        }
        if (!apiKey && !opts.tts) {
            throw new WordAudioError('no_key', 'Sound is not set up: the server has no ElevenLabs key.');
        }
        const voiceId = process.env.ELEVENLABS_VOICE_ID || audio.DEFAULT_VOICE;
        const model = process.env.ELEVENLABS_MODEL || audio.DEFAULT_MODEL;

        let bytes;
        try {
            bytes = await tts(apiKey, voiceId, model, word);
        } catch (err) {
            throw new WordAudioError('tts_failed', err && err.message ? err.message : 'ElevenLabs failed');
        }
        if (!bytes || !bytes.length) throw new WordAudioError('tts_failed', 'ElevenLabs sent no sound.');

        // Another request may have made it while this one waited: keep the first.
        const manifest = audio.readWordManifest(dir);
        const raced = audio.findWordFileByText(dir, manifest, word);
        if (raced) return clipOf(word, raced.fileName, false);

        const fileName = audio.cacheKey(voiceId, model, word) + '.mp3';
        fs.mkdirSync(dir, { recursive: true });
        fs.writeFileSync(path.join(dir, fileName), bytes);
        manifest.voiceId = voiceId;
        manifest.model = model;
        manifest.entries = manifest.entries || {};
        manifest.entries[voiceId + '|' + model + '|' + word] = {
            text: word,
            file: fileName,
            voiceId: voiceId,
            model: model,
            createdAt: new Date().toISOString(),
            bytes: bytes.length
        };
        audio.writeWordManifest(dir, manifest);
        return clipOf(word, fileName, true);
    })();

    inflight.set(flight, pending);
    const done = () => inflight.delete(flight);
    pending.then(done, done);
    return pending;
}

/**
 * Every word that has a clip, for a picker. Sorted by the Hebrew letters.
 * @param {{ cacheDir?: string }} [options]
 * @returns {{ word: string, audioPath: string, url: string }[]}
 */
function listWordClips(options) {
    const dir = (options && options.cacheDir) || wordAudioDir();
    const entries = (audio.readWordManifest(dir).entries) || {};
    const seen = new Set();
    const out = [];
    Object.keys(entries).forEach((key) => {
        const entry = entries[key];
        if (!entry || typeof entry.text !== 'string' || typeof entry.file !== 'string') return;
        if (!/^[A-Za-z0-9_.-]+\.mp3$/.test(entry.file)) return;
        if (!fs.existsSync(path.join(dir, entry.file))) return;
        const word = entry.text.trim().normalize('NFC');
        if (!word || seen.has(word)) return;
        seen.add(word);
        out.push({ word: word, audioPath: audio.SITE_WORD_PREFIX + entry.file, url: URL_PREFIX + entry.file });
    });
    const strip = (s) => s.replace(/[\u0591-\u05C7]/g, '');
    out.sort((a, b) => strip(a.word).localeCompare(strip(b.word), 'he') || a.word.localeCompare(b.word, 'he'));
    return out.slice(0, MAX_LIST);
}

module.exports = { ensureWordClip, findWordClip, listWordClips, cleanWord, WordAudioError, MAX_CHARS };
