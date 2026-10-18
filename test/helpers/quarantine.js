/**
 * Quarantine list shared by the test runner and the node:test register hook.
 * A quarantined test is skipped with its reason. A list entry whose test was
 * never registered fails the run, so renamed or deleted tests cannot linger.
 */
'use strict';

const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..', '..');
const LIST_PATH = path.join(ROOT, 'test', 'quarantine.json');
const REQUIRED = ['file', 'name', 'category', 'reason', 'owner'];

function entryKey(file, name) {
    return file + '\n' + name;
}

function loadList(file) {
    const listPath = file || LIST_PATH;
    const raw = JSON.parse(fs.readFileSync(listPath, 'utf8'));
    if (!Array.isArray(raw)) throw new Error('quarantine list must be an array: ' + listPath);
    raw.forEach((entry, index) => {
        if (!entry || typeof entry !== 'object') {
            throw new Error('quarantine entry ' + index + ' is not an object');
        }
        for (const key of REQUIRED) {
            if (typeof entry[key] !== 'string' || entry[key].length === 0) {
                throw new Error('quarantine entry ' + index + ' missing ' + key);
            }
        }
    });
    return raw;
}

function indexEntries(entries) {
    const map = new Map();
    for (const entry of entries) map.set(entryKey(entry.file, entry.name), entry);
    return map;
}

function missingEntries(entries, seenKeys) {
    return entries.filter((entry) => !seenKeys.has(entryKey(entry.file, entry.name)));
}

function relTestFile(filename) {
    if (!filename) return null;
    const abs = path.resolve(filename);
    const rel = path.relative(ROOT, abs);
    if (rel.startsWith('..') || path.isAbsolute(rel)) return null;
    return rel.split(path.sep).join('/');
}

function callerTestFrame() {
    const stack = new Error().stack.split('\n');
    for (const line of stack) {
        const match = line.match(/\(?((?:[A-Za-z]:)?[^():\n]+?\.test\.js):(\d+):(\d+)\)?/);
        if (!match) continue;
        const rel = relTestFile(match[1]);
        if (!rel) continue;
        return {
            file: rel,
            abs: match[1],
            line: Number(match[2]),
            column: Number(match[3])
        };
    }
    return null;
}

function callerTestFile() {
    const frame = callerTestFrame();
    return frame ? frame.file : null;
}

// node:test records the file and line of the function that calls it(). Calling
// orig() from this helper would blame quarantine.js, so replay the call from
// an eval tagged with the test file's path, padded to the caller's line.
const invokeAtCache = new Map();

function invokeAtCaller(orig, name, options, fn) {
    const frame = callerTestFrame();
    if (!frame || /[\r\n]/.test(frame.abs)) return orig(name, options, fn);
    const key = frame.abs + ':' + frame.line + ':' + frame.column;
    let invoke = invokeAtCache.get(key);
    if (!invoke) {
        const pad = '\n'.repeat(Math.max(0, frame.line - 2));
        const indent = ' '.repeat(Math.max(0, frame.column - 1));
        const src = '(function (orig, name, options, fn) {\n'
            + pad + indent + 'orig(name, options, fn);\n'
            + '})\n//# sourceURL=' + frame.abs;
        invoke = eval(src);
        invokeAtCache.set(key, invoke);
    }
    return invoke(orig, name, options, fn);
}

/**
 * Read the name node:test would use, without changing how an unquarantined
 * call is forwarded.
 */
function normalizeTestArgs(name, options, fn) {
    if (typeof name === 'function') {
        return { testName: name.name || '', opts: {}, cb: name };
    }
    if (name && typeof name === 'object') {
        const opts = Object.assign({}, name);
        const cb = typeof options === 'function' ? options : fn;
        const testName = typeof opts.name === 'string' ? opts.name : (cb && cb.name) || '';
        return { testName, opts, cb };
    }
    if (typeof options === 'function' || options === undefined) {
        return { testName: name, opts: {}, cb: options };
    }
    return { testName: name, opts: Object.assign({}, options || {}), cb: fn };
}

/**
 * Wrap the node:test export. Quarantined tests gain `{ skip: reason }`.
 * Every registered name in a *.test.js file is reported to `record`.
 * @param {Function} orig
 * @param {Array<object>} entries
 * @param {(file: string, name: string) => void} record
 */
function install(orig, entries, record) {
    const index = indexEntries(entries);
    function wrapped(name, options, fn) {
        const parsed = normalizeTestArgs(name, options, fn);
        const file = callerTestFile();
        if (file && typeof parsed.testName === 'string' && parsed.testName) {
            record(file, parsed.testName);
            const hit = index.get(entryKey(file, parsed.testName));
            if (hit) {
                parsed.opts.skip = hit.reason;
                return invokeAtCaller(orig, parsed.testName, parsed.opts, parsed.cb);
            }
        }
        return invokeAtCaller(orig, name, options, fn);
    }
    for (const key of Object.getOwnPropertyNames(orig)) {
        if (key === 'length' || key === 'name' || key === 'prototype') continue;
        wrapped[key] = orig[key];
    }
    wrapped.test = wrapped;
    wrapped.it = wrapped;
    return wrapped;
}

module.exports = {
    ROOT,
    LIST_PATH,
    REQUIRED,
    entryKey,
    loadList,
    indexEntries,
    missingEntries,
    relTestFile,
    callerTestFile,
    normalizeTestArgs,
    install
};
