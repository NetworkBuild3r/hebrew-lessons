/**
 * Load the Tanakh corpus from data/tanakh into tanakh_verses + tanakh_meta.
 * The files stay under data/ (not served, not in git); fetch them first with
 * scripts/fetch-tanakh.js. The server also does this on start when the table
 * is empty.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const { dataDir } = require('../paths');
const tanakh = require('../db/repos/tanakh');

/**
 * @returns {string}
 */
function folder() {
    return path.join(dataDir(), 'tanakh');
}

/**
 * @returns {string}
 */
function manifestFile() {
    return path.join(folder(), 'manifest.json');
}

/**
 * @returns {boolean}
 */
function filesPresent() {
    return fs.existsSync(manifestFile());
}

/**
 * @param {string} file
 * @returns {object}
 */
function readJson(file) {
    try {
        return JSON.parse(fs.readFileSync(file, 'utf8'));
    } catch (err) {
        throw new Error('Bad JSON in ' + file + ': ' + (err && err.message ? err.message : err));
    }
}

/**
 * Provenance fields copied into tanakh_meta.
 * @param {object} manifest
 * @returns {Record<string, string>}
 */
function metaFromManifest(manifest) {
    return {
        source: String(manifest.source || ''),
        version_en: String(manifest.versionEn || ''),
        version_he: String(manifest.versionHe || ''),
        license_en: String(manifest.licenseEn || ''),
        license_he: String(manifest.licenseHe || ''),
        fetched_at: String(manifest.fetchedAt || ''),
        book_count: String(Array.isArray(manifest.books) ? manifest.books.length : 0)
    };
}

/**
 * Read the manifest + book files and replace the tables.
 * @returns {{ loaded: number }}
 */
function load() {
    const dir = folder();
    const manifest = readJson(manifestFile());
    const listed = Array.isArray(manifest.books)
        ? manifest.books.map((entry) => entry && entry.file).filter(Boolean)
        : [];
    const files = listed.length
        ? listed
        : fs.readdirSync(dir).filter((name) => name.endsWith('.json') && name !== 'manifest.json').sort();

    const rows = [];
    files.forEach((name) => {
        if (path.basename(name) !== name) {
            throw new Error('Manifest book file must be a plain file name: ' + name);
        }
        const payload = readJson(path.join(dir, name));
        (Array.isArray(payload.rows) ? payload.rows : []).forEach((row) => rows.push(row));
    });
    return { loaded: tanakh.replaceAll(rows, metaFromManifest(manifest)) };
}

/**
 * Fill the table once, when it is empty and the files are there.
 * @returns {{ loaded: number, skipped: string }}
 */
function loadIfEmpty() {
    if (tanakh.countVerses() > 0) return { loaded: 0, skipped: 'already loaded' };
    if (!filesPresent()) return { loaded: 0, skipped: 'no files in ' + folder() };
    return { loaded: load().loaded, skipped: '' };
}

module.exports = { filesPresent, folder, load, loadIfEmpty, metaFromManifest };
