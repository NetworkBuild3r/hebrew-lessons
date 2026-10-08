/**
 * Per-user filtering of shared catalog / vocab / word-audio scripts.
 * Literal-only parse via lesson-data.js; fail closed; cache by mtime + visible set.
 * Provenance: INIT-002/SPEC-006
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { parseDataWindow } = require('./lesson-data');
const { lessonsDir, catalogFile, rootDir, lessonDir } = require('./paths');
const { visibleSlugs } = require('./visibility');

// Trusted repo IIFE — exposes HebrewUtils on globalThis (never for lesson data).
require('../shared/player/utils.js');
const HebrewUtils = globalThis.HebrewUtils;

/** @type {Map<string, { mtimeMs: number, body: string }>} */
const scriptCache = new Map();

function exactKey(text) {
    return String(text || '').normalize('NFC').replace(/\s+/g, ' ').trim();
}

function strippedKey(text) {
    const strip = HebrewUtils && typeof HebrewUtils.stripVowels === 'function'
        ? HebrewUtils.stripVowels
        : (s) => String(s || '').replace(/[\u0591-\u05C7]/g, '');
    return exactKey(strip(text));
}

function tokenizeHebrew(text) {
    if (HebrewUtils && typeof HebrewUtils.tokenizeHebrew === 'function') {
        return HebrewUtils.tokenizeHebrew(text);
    }
    return [];
}

function storyPlain(paragraphs) {
    if (HebrewUtils && typeof HebrewUtils.storyPlain === 'function') {
        return HebrewUtils.storyPlain(paragraphs);
    }
    return [];
}

function isAdminUser(user) {
    return Boolean(user && user.role === 'admin');
}

/**
 * ADR-0001 identity resolve: exact hit, else unique stripped hit in catalog keys.
 * @param {string} text
 * @param {Set<string>} catalogExactKeys
 * @returns {string|null}
 */
function resolveInExactSet(text, catalogExactKeys) {
    const e = exactKey(text);
    if (!e) return null;
    if (catalogExactKeys.has(e)) return e;
    const s = strippedKey(e);
    if (!s) return null;
    let match = null;
    for (const key of catalogExactKeys) {
        if (strippedKey(key) === s) {
            if (match && match !== key) return null;
            match = key;
        }
    }
    return match;
}

function emitScript(name, value) {
    // JSON.stringify handles quotes; \u003c keeps </script> from breaking an HTML
    // <script> wrapper while remaining valid for the literal-only parser.
    const json = JSON.stringify(value).replace(/</g, '\\u003c');
    return 'window.' + name + ' = ' + json + ';\n';
}

function emptyFor(name) {
    if (name === 'HEBREW_WORD_AUDIO') return emitScript(name, {});
    return emitScript(name, []);
}

function fileMtimeMs(file) {
    try {
        return fs.statSync(file).mtimeMs;
    } catch (_) {
        return -1;
    }
}

function visibleKey(user) {
    if (isAdminUser(user)) return 'admin';
    const id = user && user.id != null ? String(user.id) : 'anon';
    return id + ':' + visibleSlugs(user).slice().sort().join(',');
}

function cachedOrBuild(cacheId, file, mtimeMs, build) {
    const hit = scriptCache.get(cacheId);
    if (hit && hit.mtimeMs === mtimeMs) return hit.body;
    const body = build();
    scriptCache.set(cacheId, { mtimeMs, body });
    return body;
}

function readAndParse(file, globalName) {
    const src = fs.readFileSync(file, 'utf8');
    const globals = parseDataWindow(src, file);
    const value = globals[globalName];
    return value;
}

/**
 * Collect exactKeys from vocab words + tokenized lesson text for visible slugs.
 * @param {string[]} slugs
 * @returns {Set<string>}
 */
function visibleWordExactKeys(slugs) {
    const keys = new Set();
    for (const slug of slugs) {
        let worksheet;
        try {
            const file = path.join(lessonDir(slug), 'worksheet.js');
            if (!fs.existsSync(file)) continue;
            worksheet = readAndParse(file, 'WORKSHEET');
        } catch (err) {
            console.warn(
                '[content-filter] lesson worksheet unreadable for ' + slug + ':',
                err && err.message ? err.message : err
            );
            continue;
        }
        if (!worksheet || typeof worksheet !== 'object') continue;

        const items = worksheet.vocab && Array.isArray(worksheet.vocab.items)
            ? worksheet.vocab.items
            : [];
        for (const item of items) {
            if (!item || item.word == null) continue;
            const key = exactKey(item.word);
            if (key) keys.add(key);
        }

        const lines = storyPlain(
            worksheet.reading && worksheet.reading.paragraphs
                ? worksheet.reading.paragraphs
                : []
        );
        for (const line of lines) {
            for (const token of tokenizeHebrew(line)) {
                const key = exactKey(token);
                if (key) keys.add(key);
            }
        }
    }
    return keys;
}

/**
 * @param {unknown} user
 * @returns {string}
 */
function catalogScriptFor(user) {
    const file = catalogFile();
    const name = 'HEBREW_CATALOG';
    if (!fs.existsSync(file)) return emptyFor(name);

    const mtimeMs = fileMtimeMs(file);
    const vKey = visibleKey(user);
    const cacheId = 'catalog|' + vKey + '|' + file;

    return cachedOrBuild(cacheId, file, mtimeMs, () => {
        let catalog;
        try {
            catalog = readAndParse(file, name);
        } catch (err) {
            console.warn(
                '[content-filter] catalog parse failed — fail closed:',
                err && err.message ? err.message : err
            );
            return emptyFor(name);
        }
        if (!Array.isArray(catalog)) {
            console.warn('[content-filter] catalog is not an array — fail closed');
            return emptyFor(name);
        }
        if (isAdminUser(user)) {
            return emitScript(name, catalog);
        }
        const allowed = new Set(visibleSlugs(user));
        const filtered = catalog.filter(
            (entry) => entry && typeof entry === 'object' && allowed.has(entry.slug)
        );
        return emitScript(name, filtered);
    });
}

/**
 * @param {unknown} user
 * @returns {string}
 */
function vocabIndexScriptFor(user) {
    const file = path.join(lessonsDir(), '_review', 'vocab-index.js');
    const name = 'HEBREW_VOCAB_INDEX';
    if (!fs.existsSync(file)) return emptyFor(name);

    const mtimeMs = fileMtimeMs(file);
    const vKey = visibleKey(user);
    const cacheId = 'vocab|' + vKey + '|' + file;

    return cachedOrBuild(cacheId, file, mtimeMs, () => {
        let index;
        try {
            index = readAndParse(file, name);
        } catch (err) {
            console.warn(
                '[content-filter] vocab-index parse failed — fail closed:',
                err && err.message ? err.message : err
            );
            return emptyFor(name);
        }
        if (!Array.isArray(index)) {
            console.warn('[content-filter] vocab-index is not an array — fail closed');
            return emptyFor(name);
        }
        if (isAdminUser(user)) {
            return emitScript(name, index);
        }
        const allowed = new Set(visibleSlugs(user));
        const filtered = [];
        for (const entry of index) {
            if (!entry || typeof entry !== 'object') continue;
            const lessons = Array.isArray(entry.lessons) ? entry.lessons : [];
            const nextLessons = lessons.filter((slug) => allowed.has(slug));
            if (nextLessons.length === 0) continue;
            filtered.push(Object.assign({}, entry, { lessons: nextLessons }));
        }
        return emitScript(name, filtered);
    });
}

/**
 * Resolve which word-audio catalog file to filter.
 * @param {string} [which]
 * @returns {{ file: string, name: string }}
 */
function wordAudioSource(which) {
    const name = 'HEBREW_WORD_AUDIO';
    const key = String(which || 'words').toLowerCase();
    if (key === 'shared') {
        return { file: path.join(rootDir(), 'shared', 'word-audio-catalog.js'), name };
    }
    // 'words' | 'worksheets' | default
    return {
        file: path.join(lessonsDir(), '_audio', 'words', 'catalog.js'),
        name
    };
}

/**
 * Keep audio entries whose exact key (or unique stripped identity) appears in
 * visible-lesson vocab or tokenized lesson text.
 * @param {Record<string, string>} map
 * @param {Set<string>} visibleExact
 * @returns {Record<string, string>}
 */
function filterWordAudioMap(map, visibleExact) {
    const out = {};
    if (!map || typeof map !== 'object') return out;
    for (const key of Object.keys(map)) {
        if (resolveInExactSet(key, visibleExact)) {
            out[key] = map[key];
        }
    }
    return out;
}

/**
 * @param {unknown} user
 * @param {string} [which]
 * @returns {string}
 */
function wordAudioScriptFor(user, which) {
    const { file, name } = wordAudioSource(which);
    if (!fs.existsSync(file)) return emptyFor(name);

    const mtimeMs = fileMtimeMs(file);
    const vKey = visibleKey(user);
    const cacheId = 'word-audio|' + String(which || 'words') + '|' + vKey + '|' + file;

    return cachedOrBuild(cacheId, file, mtimeMs, () => {
        let map;
        try {
            map = readAndParse(file, name);
        } catch (err) {
            console.warn(
                '[content-filter] word-audio parse failed — fail closed:',
                err && err.message ? err.message : err
            );
            return emptyFor(name);
        }
        if (!map || typeof map !== 'object' || Array.isArray(map)) {
            console.warn('[content-filter] word-audio is not an object — fail closed');
            return emptyFor(name);
        }
        if (isAdminUser(user)) {
            return emitScript(name, map);
        }
        const slugs = visibleSlugs(user);
        const visibleExact = visibleWordExactKeys(slugs);
        return emitScript(name, filterWordAudioMap(map, visibleExact));
    });
}

/** Test helper — drop in-memory filtered-script cache. */
function clearScriptCache() {
    scriptCache.clear();
}

module.exports = {
    catalogScriptFor,
    vocabIndexScriptFor,
    wordAudioScriptFor,
    clearScriptCache,
    // exported for unit assertions
    exactKey,
    strippedKey,
    resolveInExactSet,
    emitScript
};
