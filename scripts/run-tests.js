/**
 * `npm test`. Runs the node:test files with the spec reporter and a second
 * reporter that records skipped tests. Fails when a quarantine.json entry
 * was not skipped with its reason, or when an entry is past its expiry.
 * TZ is not set here; CI sets TZ=UTC.
 */
'use strict';

const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const quarantine = require('../test/helpers/quarantine');

const root = quarantine.ROOT;
const files = fs.readdirSync(path.join(root, 'test'))
    .filter((name) => name.endsWith('.test.js'))
    .sort()
    .map((name) => path.join('test', name));

let entries;
try {
    entries = quarantine.loadList();
} catch (err) {
    console.error(err && err.message ? err.message : err);
    process.exit(1);
}

const expired = quarantine.expiredEntries(entries);
if (expired.length) {
    console.error('quarantine expired:');
    for (const entry of expired) {
        console.error('  ' + entry.file + ' :: ' + entry.name + ' (expires ' + entry.expires + ')');
    }
    process.exit(1);
}

const seenDir = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-quarantine-'));
const seenFile = path.join(seenDir, 'skipped.txt');
const child = spawn(process.execPath, [
    '--test',
    '--test-reporter', 'spec',
    '--test-reporter-destination', 'stdout',
    '--test-reporter', path.join(root, 'test', 'helpers', 'quarantine-reporter.js'),
    '--test-reporter-destination', seenFile,
    ...files
], {
    cwd: root,
    stdio: 'inherit'
});

child.on('exit', (code, signal) => {
    let seen = [];
    if (fs.existsSync(seenFile)) {
        const text = fs.readFileSync(seenFile, 'utf8');
        seen = text.split('\n').filter(Boolean).map((line) => JSON.parse(line));
    }
    const missing = quarantine.unmatchedSkips(entries, seen);
    const matched = entries.length - missing.length;
    console.log('# quarantined ' + matched);
    try { fs.rmSync(seenDir, { recursive: true, force: true }); } catch (err) { /* ignore */ }
    if (missing.length) {
        console.error('quarantine list is stale; these tests were not skipped with their reason:');
        for (const entry of missing) {
            console.error('  ' + entry.file + ' :: ' + entry.name);
        }
        process.exit(1);
    }
    if (signal) process.exit(1);
    process.exit(code == null ? 1 : code);
});
