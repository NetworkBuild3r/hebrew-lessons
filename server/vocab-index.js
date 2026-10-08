/**
 * Cross-lesson vocabulary index generator (ADR-0001 §7).
 * Walks LESSONS_DIR lesson folders, merges vocab by exactKey, writes
 * LESSONS_DIR/_review/vocab-index.js as window.HEBREW_VOCAB_INDEX.
 * Authored-by-spec: INIT-001/SPEC-002
 */
const fs = require('fs');
const path = require('path');
const { lessonsDir } = require('./paths');
const { loadDataWindow } = require('./lesson-data');

const INDEX_REL = path.join('_review', 'vocab-index.js');

/** NFC + whitespace collapse + trim — ADR-0001 exactKey. */
function exactKey(text) {
    return String(text || '').normalize('NFC').replace(/\s+/g, ' ').trim();
}

/** Data-only read of a lesson or catalog file. Never executes it (see server/lesson-data.js). */
function loadWindow(file) {
    return loadDataWindow(file);
}

function reviewIndexPath(base) {
    return path.join(base, INDEX_REL);
}

function resolveLessonDir(base, name) {
    const resolvedBase = path.resolve(base);
    const dir = path.resolve(resolvedBase, name);
    if (dir !== path.join(resolvedBase, name)) {
        throw new Error('Lesson path escaped the lessons folder: ' + name);
    }
    return dir;
}

function listLessonNames(base) {
    if (!fs.existsSync(base)) return [];
    return fs.readdirSync(base, { withFileTypes: true })
        .filter(entry => entry.isDirectory() && !entry.name.startsWith('_') && !entry.name.startsWith('.'))
        .map(entry => entry.name);
}

function loadWordAudioMap(base) {
    const catalogPath = path.join(base, '_audio', 'words', 'catalog.js');
    if (!fs.existsSync(catalogPath)) return {};
    try {
        const win = loadWindow(catalogPath);
        const map = win.HEBREW_WORD_AUDIO;
        return map && typeof map === 'object' ? map : {};
    } catch (err) {
        console.warn('[vocab-index] word-audio catalog unreadable:', err && err.message ? err.message : err);
        return {};
    }
}

function catalogAudioFor(wordAudioMap, key, word) {
    if (!wordAudioMap) return '';
    if (key && wordAudioMap[key]) return String(wordAudioMap[key]);
    const raw = String(word || '').trim();
    if (raw && wordAudioMap[raw]) return String(wordAudioMap[raw]);
    const nfc = raw.normalize('NFC');
    if (nfc && wordAudioMap[nfc]) return String(wordAudioMap[nfc]);
    return '';
}

function pickField(existing, incoming, name) {
    const next = incoming && incoming[name] != null ? String(incoming[name]) : '';
    if (!existing) return next;
    if (existing[name]) return existing[name];
    return next;
}

/**
 * Merge one lesson's vocab.items into the map keyed by exactKey.
 * @returns {{ added: number, skipped: boolean, warning: string|null }}
 */
function mergeLessonItems(byKey, slug, items, wordAudioMap) {
    if (!Array.isArray(items)) {
        return { added: 0, skipped: true, warning: 'lesson ' + slug + ': vocab.items is not an array' };
    }
    let added = 0;
    items.forEach(item => {
        if (!item || typeof item !== 'object') return;
        const word = item.word != null ? String(item.word) : '';
        const key = exactKey(word);
        if (!key) return;

        const sharedAudio = catalogAudioFor(wordAudioMap, key, word);
        const lessonAudio = item.audio != null ? String(item.audio) : '';
        const audio = sharedAudio || lessonAudio;

        const prev = byKey.get(key);
        if (!prev) {
            byKey.set(key, {
                key,
                word,
                meaning: item.meaning != null ? String(item.meaning) : '',
                sound: item.sound != null ? String(item.sound) : '',
                audio: audio || '',
                sentence: item.sentence != null ? String(item.sentence) : '',
                sentenceAudio: item.sentenceAudio != null ? String(item.sentenceAudio) : '',
                lessons: [slug]
            });
            added += 1;
            return;
        }

        prev.meaning = pickField(prev, item, 'meaning');
        prev.sound = pickField(prev, item, 'sound');
        prev.sentence = pickField(prev, item, 'sentence');
        prev.sentenceAudio = pickField(prev, item, 'sentenceAudio');
        if (sharedAudio) prev.audio = sharedAudio;
        else if (!prev.audio && lessonAudio) prev.audio = lessonAudio;
        if (!prev.lessons.includes(slug)) prev.lessons.push(slug);
    });
    return { added, skipped: false, warning: null };
}

function loadLessonVocab(base, slug, warnings) {
    let dir;
    try {
        dir = resolveLessonDir(base, slug);
    } catch (err) {
        warnings.push(err.message);
        return null;
    }
    const dataFile = path.join(dir, 'worksheet.js');
    if (!fs.existsSync(dataFile)) {
        warnings.push('lesson ' + slug + ': missing worksheet.js');
        return null;
    }
    let win;
    try {
        win = loadWindow(dataFile);
    } catch (err) {
        warnings.push('lesson ' + slug + ': worksheet.js failed — ' + (err && err.message ? err.message : String(err)));
        return null;
    }
    const worksheet = win.WORKSHEET;
    if (!worksheet || typeof worksheet !== 'object') {
        warnings.push('lesson ' + slug + ': no window.WORKSHEET');
        return null;
    }
    if (!worksheet.vocab || typeof worksheet.vocab !== 'object') {
        warnings.push('lesson ' + slug + ': missing vocab');
        return null;
    }
    return worksheet.vocab.items;
}

/**
 * Build the merged vocab index for the current LESSONS_DIR (or override).
 * Malformed lessons are skipped with warnings; the index is still written.
 *
 * @param {{ lessonsDir?: string }} [opts]
 * @returns {{ path: string, entries: object[], warnings: string[], lessonsScanned: number, lessonsUsed: number }}
 */
function buildVocabIndex(opts) {
    const base = path.resolve((opts && opts.lessonsDir) || lessonsDir());
    const warnings = [];
    const byKey = new Map();
    const wordAudioMap = loadWordAudioMap(base);
    const names = listLessonNames(base);
    let lessonsUsed = 0;

    names.forEach(slug => {
        const items = loadLessonVocab(base, slug, warnings);
        if (items == null) return;
        const result = mergeLessonItems(byKey, slug, items, wordAudioMap);
        if (result.skipped) {
            if (result.warning) warnings.push(result.warning);
            return;
        }
        lessonsUsed += 1;
    });

    const entries = Array.from(byKey.values()).map(entry => {
        entry.lessons = entry.lessons.slice().sort();
        return entry;
    });
    entries.sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));

    return {
        path: reviewIndexPath(base),
        entries,
        warnings,
        lessonsScanned: names.length,
        lessonsUsed
    };
}

/**
 * Write LESSONS_DIR/_review/vocab-index.js from the current lessons tree.
 * @param {{ lessonsDir?: string }} [opts]
 * @returns {{ path: string, entries: object[], warnings: string[], lessonsScanned: number, lessonsUsed: number }}
 */
function rebuildVocabIndex(opts) {
    const built = buildVocabIndex(opts);
    const outPath = built.path;
    const outDir = path.dirname(outPath);
    const base = path.resolve((opts && opts.lessonsDir) || lessonsDir());
    const expected = path.resolve(reviewIndexPath(base));
    if (path.resolve(outPath) !== expected) {
        throw new Error('Vocab index path escaped LESSONS_DIR/_review');
    }
    fs.mkdirSync(outDir, { recursive: true });
    const body = 'window.HEBREW_VOCAB_INDEX = ' + JSON.stringify(built.entries, null, 4) + ';\n';
    fs.writeFileSync(outPath, body, 'utf8');
    built.warnings.forEach(w => console.warn('[vocab-index]', w));
    return built;
}

/**
 * Rebuild for publish hooks — never throws (logs and continues).
 * @returns {{ ok: boolean, result?: object, error?: string }}
 */
function rebuildVocabIndexSafe(opts) {
    try {
        const result = rebuildVocabIndex(opts);
        return { ok: true, result };
    } catch (err) {
        const message = err && err.message ? err.message : String(err);
        console.error('[vocab-index] rebuild failed:', message);
        return { ok: false, error: message };
    }
}

module.exports = {
    exactKey,
    loadWindow,
    buildVocabIndex,
    rebuildVocabIndex,
    rebuildVocabIndexSafe,
    reviewIndexPath,
    INDEX_REL
};
