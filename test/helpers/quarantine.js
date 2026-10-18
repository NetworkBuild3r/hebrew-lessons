/**
 * Quarantine list for tests that are skipped on purpose.
 *
 * A test opts in with `it(name, quarantine.for(file, name), fn)`. That returns
 * node:test's own `{ skip: reason }`, so the runner prints the reason and
 * counts a skip. Nothing patches `require` or rewrites stack locations.
 *
 * `npm test` reads the reporter log afterwards and fails if a listed test was
 * not skipped with its reason. An entry whose `expires` date is before today
 * (UTC) fails the run.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..', '..');
const LIST_PATH = path.join(ROOT, 'test', 'quarantine.json');
const REQUIRED = ['file', 'name', 'category', 'reason', 'owner'];
const CATEGORIES = ['real-bug', 'environment'];
const ISSUE_RE = /^https:\/\/github\.com\/NetworkBuild3r\/hebrew-lessons\/issues\/\d+$/;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

function entryKey(file, name) {
    return file + '\n' + name;
}

function isRealDate(value) {
    if (typeof value !== 'string' || !DATE_RE.test(value)) return false;
    const [year, month, day] = value.split('-').map(Number);
    const date = new Date(Date.UTC(year, month - 1, day));
    return date.getUTCFullYear() === year
        && date.getUTCMonth() === month - 1
        && date.getUTCDate() === day;
}

function utcToday(now) {
    const date = now instanceof Date ? now : new Date();
    return date.toISOString().slice(0, 10);
}

function loadList(file) {
    const listPath = file || LIST_PATH;
    const raw = JSON.parse(fs.readFileSync(listPath, 'utf8'));
    if (!Array.isArray(raw)) throw new Error('quarantine list must be an array: ' + listPath);
    const keys = new Set();
    raw.forEach((entry, index) => {
        const where = 'quarantine entry ' + index;
        if (!entry || typeof entry !== 'object') throw new Error(where + ' is not an object');
        for (const key of REQUIRED) {
            if (typeof entry[key] !== 'string' || entry[key].length === 0) {
                throw new Error(where + ' missing ' + key);
            }
        }
        if (!CATEGORIES.includes(entry.category)) {
            throw new Error(where + ' category must be real-bug or environment');
        }
        if (!entry.file.startsWith('test/') || !entry.file.endsWith('.test.js')) {
            throw new Error(where + ' file must be a test/*.test.js path');
        }
        const hasIssue = entry.issue != null && entry.issue !== '';
        const hasExpires = entry.expires != null && entry.expires !== '';
        if (hasIssue && (typeof entry.issue !== 'string' || !ISSUE_RE.test(entry.issue))) {
            throw new Error(where + ' issue must be a hebrew-lessons issue URL');
        }
        if (hasExpires && !isRealDate(entry.expires)) {
            throw new Error(where + ' expires must be YYYY-MM-DD');
        }
        if (!hasIssue && !hasExpires) {
            throw new Error(where + ' needs an issue or expires');
        }
        const key = entryKey(entry.file, entry.name);
        if (keys.has(key)) throw new Error(where + ' duplicates ' + entry.file + ' :: ' + entry.name);
        keys.add(key);
    });
    return raw;
}

function indexEntries(entries) {
    const map = new Map();
    for (const entry of entries) map.set(entryKey(entry.file, entry.name), entry);
    return map;
}

function isExpired(entry, today) {
    return typeof entry.expires === 'string' && entry.expires < today;
}

function expiredEntries(entries, today) {
    const day = today || utcToday();
    return entries.filter((entry) => isExpired(entry, day));
}

/**
 * node:test options for one test. A listed test is skipped with its reason.
 * A listed test past `expires` throws, so the file fails instead of skipping.
 * @param {Array<object>} entries
 * @param {string} file
 * @param {string} name
 * @param {object} [options]
 * @param {string} [today] UTC YYYY-MM-DD
 */
function optionsFor(entries, file, name, options, today) {
    const base = options && typeof options === 'object' ? Object.assign({}, options) : {};
    const entry = indexEntries(entries).get(entryKey(file, name));
    if (!entry) return base;
    const day = today || utcToday();
    if (isExpired(entry, day)) {
        throw new Error('quarantine expired: ' + file + ' :: ' + name + ' (expires ' + entry.expires + ')');
    }
    base.skip = entry.reason;
    return base;
}

function forTest(file, name, options) {
    return optionsFor(loadList(), file, name, options);
}

function relTestFile(filename) {
    if (!filename) return null;
    if (filename.startsWith('test/')) return filename;
    const abs = path.resolve(filename);
    const rel = path.relative(ROOT, abs);
    if (rel.startsWith('..') || path.isAbsolute(rel)) return null;
    return rel.split(path.sep).join('/');
}

/**
 * Entries that were not skipped with their own reason.
 * @param {Array<object>} entries
 * @param {Array<{file: string, name: string, skip: string}>} seen
 */
function unmatchedSkips(entries, seen) {
    const got = new Map();
    for (const row of seen) {
        const rel = relTestFile(row.file);
        if (!rel || typeof row.name !== 'string') continue;
        got.set(entryKey(rel, row.name), row.skip);
    }
    return entries.filter((entry) => got.get(entryKey(entry.file, entry.name)) !== entry.reason);
}

module.exports = {
    ROOT,
    LIST_PATH,
    REQUIRED,
    CATEGORIES,
    entryKey,
    loadList,
    indexEntries,
    utcToday,
    isExpired,
    expiredEntries,
    optionsFor,
    for: forTest,
    relTestFile,
    unmatchedSkips
};
