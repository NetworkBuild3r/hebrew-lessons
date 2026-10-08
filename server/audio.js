/**
 * ElevenLabs lesson audio generation.
 * Words are permanent under LESSONS_DIR/_audio/words/ (never overwritten).
 * Lesson clips live under <lesson>/audio/ and regenerate when text changes.
 */
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const https = require('https');
const { rootDir, lessonsDir, lessonDir, wordAudioDir, catalogFile } = require('./paths');

const DEFAULT_VOICE = '6WRlPmU1fqPwYbSxeP16';
const DEFAULT_MODEL = 'eleven_v3';
const SITE_WORD_PREFIX = '../_audio/words/';
const LEGACY_SHARED_PREFIX = '/shared/audio-cache/words/';
const CONCURRENCY = 3;

function loadEnvFile() {
    const file = path.join(rootDir(), '.env');
    if (!fs.existsSync(file)) return;
    fs.readFileSync(file, 'utf8').split(/\r?\n/).forEach(line => {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) return;
        const eq = trimmed.indexOf('=');
        if (eq < 1) return;
        const key = trimmed.slice(0, eq).trim();
        let value = trimmed.slice(eq + 1).trim();
        if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
            value = value.slice(1, -1);
        }
        if (process.env[key] === undefined) process.env[key] = value;
    });
}

function apiKeyFromEnv() {
    return process.env.ELEVENLABS_API_KEY
        || process.env['11_labs_api_key']
        || process.env.ELEVEN_LABS_API_KEY
        || '';
}

function loadWindow(file) {
    const code = fs.readFileSync(file, 'utf8');
    const sandbox = { window: {}, console, globalThis: {} };
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(code, sandbox, { filename: file });
    return sandbox.window || sandbox.globalThis;
}

function loadUtils() {
    const file = path.join(rootDir(), 'shared', 'player', 'utils.js');
    return loadWindow(file).HebrewUtils;
}

function writeWorksheet(file, worksheet) {
    fs.writeFileSync(file, 'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n', 'utf8');
}

function cacheKey(voiceId, model, text) {
    return crypto.createHash('sha256')
        .update([voiceId, model, text].join('\0'), 'utf8')
        .digest('hex')
        .slice(0, 24);
}

function textHash(voiceId, model, text) {
    return crypto.createHash('sha256')
        .update([voiceId, model, String(text || '').trim()].join('\0'), 'utf8')
        .digest('hex');
}

function readJson(file, fallback) {
    if (!fs.existsSync(file)) return fallback;
    try {
        return JSON.parse(fs.readFileSync(file, 'utf8'));
    } catch (err) {
        throw new Error('Corrupt JSON at ' + file + ': ' + (err.message || err));
    }
}

function writeJson(file, value) {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, JSON.stringify(value, null, 2) + '\n', 'utf8');
}

function postTts(apiKey, voiceId, model, text) {
    const body = JSON.stringify({
        text,
        model_id: model,
        language_code: 'he'
    });
    const url = new URL('https://api.elevenlabs.io/v1/text-to-speech/' + encodeURIComponent(voiceId));
    url.searchParams.set('output_format', 'mp3_44100_128');
    return new Promise((resolve, reject) => {
        const req = https.request(url, {
            method: 'POST',
            headers: {
                'xi-api-key': apiKey,
                'Content-Type': 'application/json',
                Accept: 'audio/mpeg',
                'Content-Length': Buffer.byteLength(body)
            }
        }, res => {
            const chunks = [];
            res.on('data', c => chunks.push(c));
            res.on('end', () => {
                const buf = Buffer.concat(chunks);
                if (res.statusCode < 200 || res.statusCode >= 300) {
                    let detail = buf.toString('utf8').slice(0, 500);
                    try {
                        const parsed = JSON.parse(buf.toString('utf8'));
                        const d = parsed.detail || parsed;
                        if (d && d.code === 'paid_plan_required') {
                            detail = 'This voice needs a paid ElevenLabs plan for API use (library voices are blocked on free accounts). Upgrade, or pick a voice your plan can use and set ELEVENLABS_VOICE_ID.';
                        } else if (d && d.message) {
                            detail = d.message;
                        }
                    } catch (err) { /* keep raw */ }
                    reject(new Error('ElevenLabs HTTP ' + res.statusCode + ': ' + detail));
                    return;
                }
                resolve(buf);
            });
        });
        req.on('error', reject);
        req.write(body);
        req.end();
    });
}

function createPool(limit) {
    let active = 0;
    const queue = [];
    const runNext = () => {
        if (active >= limit || !queue.length) return;
        active += 1;
        const { fn, resolve, reject } = queue.shift();
        Promise.resolve()
            .then(fn)
            .then(value => {
                active -= 1;
                resolve(value);
                runNext();
            })
            .catch(err => {
                active -= 1;
                reject(err);
                runNext();
            });
    };
    return function enqueue(fn) {
        return new Promise((resolve, reject) => {
            queue.push({ fn, resolve, reject });
            runNext();
        });
    };
}

/** One-time: move shared/audio-cache/words into LESSONS_DIR/_audio/words (no API calls). */
function migrateLegacyWordCache() {
    const legacyDir = path.join(rootDir(), 'shared', 'audio-cache', 'words');
    const destDir = wordAudioDir();
    if (!fs.existsSync(legacyDir)) return { moved: 0 };
    fs.mkdirSync(destDir, { recursive: true });
    let moved = 0;
    const legacyManifest = path.join(legacyDir, 'manifest.json');
    const destManifest = path.join(destDir, 'manifest.json');
    fs.readdirSync(legacyDir).forEach(name => {
        if (!name.endsWith('.mp3')) return;
        const src = path.join(legacyDir, name);
        const dest = path.join(destDir, name);
        if (!fs.existsSync(dest)) {
            fs.renameSync(src, dest);
            moved += 1;
        } else if (fs.existsSync(src)) {
            fs.unlinkSync(src);
        }
    });
    if (fs.existsSync(legacyManifest) && !fs.existsSync(destManifest)) {
        fs.renameSync(legacyManifest, destManifest);
    } else if (fs.existsSync(legacyManifest)) {
        fs.unlinkSync(legacyManifest);
    }
    // Leave .gitkeep / empty dir; rewrite lesson paths that still point at the legacy prefix.
    rewriteLegacyWordPathsInLessons();
    return { moved };
}

function rewriteLegacyWordPathsInLessons() {
    const base = lessonsDir();
    if (!fs.existsSync(base)) return;
    fs.readdirSync(base, { withFileTypes: true }).forEach(entry => {
        if (!entry.isDirectory() || entry.name.startsWith('_') || entry.name.startsWith('.')) return;
        const dataFile = path.join(base, entry.name, 'worksheet.js');
        if (!fs.existsSync(dataFile)) return;
        let code = fs.readFileSync(dataFile, 'utf8');
        if (!code.includes(LEGACY_SHARED_PREFIX)) return;
        code = code.split(LEGACY_SHARED_PREFIX).join(SITE_WORD_PREFIX);
        fs.writeFileSync(dataFile, code, 'utf8');
    });
}

function readWordManifest(cacheDir) {
    const file = path.join(cacheDir, 'manifest.json');
    return readJson(file, { version: 1, voiceId: '', model: '', entries: {} });
}

function writeWordManifest(cacheDir, manifest) {
    writeJson(path.join(cacheDir, 'manifest.json'), manifest);
}

function readLessonManifest(lessonDirPath) {
    return readJson(path.join(lessonDirPath, 'audio', 'manifest.json'), { version: 1, clips: {} });
}

function writeLessonManifest(lessonDirPath, manifest) {
    writeJson(path.join(lessonDirPath, 'audio', 'manifest.json'), manifest);
}

function listLessonSlugs() {
    const file = catalogFile();
    if (!fs.existsSync(file)) {
        const base = lessonsDir();
        if (!fs.existsSync(base)) return [];
        return fs.readdirSync(base, { withFileTypes: true })
            .filter(e => e.isDirectory() && !e.name.startsWith('_') && !e.name.startsWith('.') && e.name !== '_template')
            .filter(e => fs.existsSync(path.join(base, e.name, 'worksheet.js')))
            .map(e => e.name);
    }
    try {
        const catalog = loadWindow(file).HEBREW_CATALOG;
        if (!Array.isArray(catalog)) return [];
        return catalog.filter(e => e && e.slug).map(e => e.slug);
    } catch (err) {
        return [];
    }
}

/**
 * @param {string} slug
 * @param {{ forceSentences?: boolean, dryRun?: boolean, linkOnly?: boolean, wordsOnly?: boolean, sentencesOnly?: boolean }} [opts]
 */
async function generateLessonAudio(slug, opts) {
    loadEnvFile();
    migrateLegacyWordCache();

    const options = opts || {};
    const dryRun = !!options.dryRun;
    const forceSentences = !!options.forceSentences;
    const wordsOnly = !!options.wordsOnly;
    const sentencesOnly = !!options.sentencesOnly;
    const doWords = !sentencesOnly;
    const doSentences = !wordsOnly;

    const voiceId = process.env.ELEVENLABS_VOICE_ID || DEFAULT_VOICE;
    const model = process.env.ELEVENLABS_MODEL || DEFAULT_MODEL;
    let apiKey = apiKeyFromEnv();
    let linkOnly = !!options.linkOnly || (!apiKey && !dryRun);
    if (!apiKey && !dryRun && !linkOnly) linkOnly = true;

    const dir = lessonDir(slug);
    const dataFile = path.join(dir, 'worksheet.js');
    if (!fs.existsSync(dataFile)) {
        throw new Error('No worksheet at ' + path.relative(rootDir(), dataFile));
    }
    const worksheet = loadWindow(dataFile).WORKSHEET;
    if (!worksheet || typeof worksheet !== 'object') {
        throw new Error('worksheet.js did not set window.WORKSHEET');
    }

    const utils = loadUtils();
    const cacheDir = wordAudioDir();
    fs.mkdirSync(cacheDir, { recursive: true });
    const wordManifest = readWordManifest(cacheDir);
    wordManifest.voiceId = voiceId;
    wordManifest.model = model;
    let wordManifestDirty = false;

    const lessonManifest = readLessonManifest(dir);
    if (!lessonManifest.clips || typeof lessonManifest.clips !== 'object') lessonManifest.clips = {};
    let lessonManifestDirty = false;

    const result = {
        slug,
        voiceId,
        model,
        dryRun,
        linkOnly,
        forceSentences,
        wordsReused: 0,
        wordsCreated: 0,
        wordsMissing: [],
        clipsReused: 0,
        clipsCreated: 0,
        clipsMissing: [],
        errors: []
    };

    const pool = createPool(CONCURRENCY);
    const inflightWords = new Map();

    async function tts(text) {
        if (dryRun || linkOnly) return null;
        return pool(() => postTts(apiKey, voiceId, model, text));
    }

    async function ensureWordAudio(text) {
        const trimmed = String(text || '').trim();
        if (!trimmed) return null;
        const key = cacheKey(voiceId, model, trimmed);
        const entryKey = voiceId + '|' + model + '|' + trimmed;
        const fileName = key + '.mp3';
        const full = path.join(cacheDir, fileName);
        const sitePath = SITE_WORD_PREFIX + fileName;

        if (!fs.existsSync(full)) {
            const fromManifest = wordManifest.entries[entryKey]?.file;
            if (fromManifest && fromManifest !== fileName) {
                const legacyFull = path.join(cacheDir, fromManifest);
                if (fs.existsSync(legacyFull)) fs.renameSync(legacyFull, full);
            }
            if (!fs.existsSync(full) && fs.existsSync(cacheDir)) {
                const legacy = fs.readdirSync(cacheDir).find(name => name.startsWith(key + '-') && name.endsWith('.mp3'));
                if (legacy) fs.renameSync(path.join(cacheDir, legacy), full);
            }
        }

        if (fs.existsSync(full)) {
            const prev = wordManifest.entries[entryKey];
            wordManifest.entries[entryKey] = {
                text: trimmed,
                file: fileName,
                voiceId,
                model,
                createdAt: (prev && prev.createdAt) || new Date().toISOString(),
                bytes: fs.statSync(full).size
            };
            wordManifestDirty = true;
            result.wordsReused += 1;
            return sitePath;
        }

        if (dryRun) {
            result.wordsMissing.push(trimmed);
            return sitePath;
        }

        if (linkOnly) {
            result.wordsMissing.push(trimmed);
            return null;
        }

        if (inflightWords.has(key)) return inflightWords.get(key);

        const pending = (async () => {
            const audio = await tts(trimmed);
            if (fs.existsSync(full)) {
                result.wordsReused += 1;
                return sitePath;
            }
            fs.mkdirSync(cacheDir, { recursive: true });
            fs.writeFileSync(full, audio);
            wordManifest.entries[entryKey] = {
                text: trimmed,
                file: fileName,
                voiceId,
                model,
                createdAt: new Date().toISOString(),
                bytes: audio.length
            };
            wordManifestDirty = true;
            result.wordsCreated += 1;
            return sitePath;
        })();

        inflightWords.set(key, pending);
        try {
            return await pending;
        } finally {
            inflightWords.delete(key);
        }
    }

    async function ensureLessonClip(relPath, text) {
        const trimmed = String(text || '').trim();
        if (!trimmed) return null;
        const rel = relPath.replace(/\\/g, '/');
        const full = path.join(dir, rel);
        const hash = textHash(voiceId, model, trimmed);
        const prevHash = lessonManifest.clips[rel];

        if (fs.existsSync(full) && !forceSentences && prevHash === hash) {
            result.clipsReused += 1;
            return rel;
        }
        if (fs.existsSync(full) && !forceSentences && !prevHash) {
            // Adopt legacy clip with no manifest entry — no API call.
            lessonManifest.clips[rel] = hash;
            lessonManifestDirty = true;
            result.clipsReused += 1;
            return rel;
        }
        if (fs.existsSync(full) && !forceSentences && prevHash && prevHash !== hash) {
            // Text changed — regenerate below.
        } else if (fs.existsSync(full) && !forceSentences) {
            result.clipsReused += 1;
            return rel;
        }

        if (dryRun) {
            result.clipsMissing.push(rel);
            return rel;
        }
        if (linkOnly) {
            if (fs.existsSync(full)) {
                lessonManifest.clips[rel] = hash;
                lessonManifestDirty = true;
                result.clipsReused += 1;
                return rel;
            }
            result.clipsMissing.push(rel + ': ' + trimmed.slice(0, 40));
            return null;
        }

        try {
            const audio = await tts(trimmed);
            fs.mkdirSync(path.dirname(full), { recursive: true });
            fs.writeFileSync(full, audio);
            lessonManifest.clips[rel] = hash;
            lessonManifestDirty = true;
            result.clipsCreated += 1;
            return rel;
        } catch (err) {
            result.errors.push(String(err.message || err));
            return fs.existsSync(full) ? rel : null;
        }
    }

    try {
        const vocab = worksheet.vocab?.items || [];
        for (const item of vocab) {
            if (doWords && item.word) {
                try {
                    const audio = await ensureWordAudio(item.word);
                    if (audio) item.audio = audio;
                    else if (item.audio && String(item.audio).startsWith(LEGACY_SHARED_PREFIX)) {
                        item.audio = SITE_WORD_PREFIX + path.basename(item.audio);
                    }
                } catch (err) {
                    result.errors.push('word ' + item.word + ': ' + (err.message || err));
                }
            } else if (item.audio && String(item.audio).startsWith(LEGACY_SHARED_PREFIX)) {
                item.audio = SITE_WORD_PREFIX + path.basename(item.audio);
            }
            if (doSentences && item.sentence) {
                const rel = path.join('audio', 'sentences', 'vocab-' + String(item.id) + '.mp3');
                const clip = await ensureLessonClip(rel, item.sentence);
                if (clip) item.sentenceAudio = clip;
            }
        }

        if (doSentences) {
            const friends = worksheet.askFriend?.questions || [];
            for (const q of friends) {
                const text = q.pointed || q.question;
                if (!text) continue;
                const rel = path.join('audio', 'sentences', 'friend-' + String(q.id) + '.mp3');
                const clip = await ensureLessonClip(rel, text);
                if (clip) q.audio = clip;
            }

            const grammarItems = worksheet.grammar?.items || [];
            for (const item of grammarItems) {
                if (!item.pointed) continue;
                const rel = path.join('audio', 'sentences', 'grammar-' + String(item.id) + '.mp3');
                const clip = await ensureLessonClip(rel, item.pointed);
                if (clip) item.audio = clip;
            }

            const questions = worksheet.reading?.questions || [];
            for (const q of questions) {
                if (!q.question) continue;
                const rel = path.join('audio', 'sentences', 'question-' + String(q.id) + '.mp3');
                const clip = await ensureLessonClip(rel, q.question);
                if (clip) q.audio = clip;
            }

            const storyLines = utils.storyPlain(worksheet.reading?.paragraphs || []);
            if (storyLines.length) {
                if (!worksheet.reading) worksheet.reading = {};
                const audioList = Array.isArray(worksheet.reading.audio) ? worksheet.reading.audio.slice() : [];
                for (let i = 0; i < storyLines.length; i += 1) {
                    const rel = path.join('audio', 'story', 'line-' + (i + 1) + '.mp3');
                    const clip = await ensureLessonClip(rel, storyLines[i]);
                    audioList[i] = clip || audioList[i] || '';
                }
                worksheet.reading.audio = audioList.slice(0, storyLines.length);
            }
        }

        if (wordManifestDirty && !dryRun) writeWordManifest(cacheDir, wordManifest);
        if (lessonManifestDirty && !dryRun) writeLessonManifest(dir, lessonManifest);
        if (!dryRun) writeWorksheet(dataFile, worksheet);
    } catch (err) {
        result.errors.push(String(err.message || err));
    }

    return result;
}

function formatAudioSummary(result) {
    if (!result) return '';
    const missing = (result.wordsMissing?.length || 0) + (result.clipsMissing?.length || 0);
    const parts = [
        (result.wordsReused || 0) + ' words reused',
        (result.wordsCreated || 0) + ' new',
        (result.clipsReused || 0) + ' clips reused',
        (result.clipsCreated || 0) + ' new',
        missing + ' missing'
    ];
    let line = 'Audio: ' + parts[0] + ', ' + parts[1] + '; ' + parts[2] + ', ' + parts[3] + '; ' + parts[4] + '.';
    if (result.linkOnly) line += ' (link-only — no API key)';
    if (result.errors && result.errors.length) {
        line += '\nAudio errors:\n- ' + result.errors.join('\n- ');
    }
    if (result.wordsMissing && result.wordsMissing.length) {
        line += '\nMissing words:\n- ' + result.wordsMissing.join('\n- ');
    }
    if (result.clipsMissing && result.clipsMissing.length) {
        line += '\nMissing clips:\n- ' + result.clipsMissing.join('\n- ');
    }
    return line;
}

module.exports = {
    loadEnvFile,
    generateLessonAudio,
    migrateLegacyWordCache,
    formatAudioSummary,
    listLessonSlugs,
    cacheKey,
    textHash,
    DEFAULT_VOICE,
    DEFAULT_MODEL,
    SITE_WORD_PREFIX,
    LEGACY_SHARED_PREFIX
};
