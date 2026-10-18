/**
 * `npm test`. Runs the node:test files with the quarantine preload, then
 * fails if any quarantine.json entry was never registered.
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

const seenDir = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-quarantine-'));
const child = spawn(process.execPath, [
    '--require', path.join(root, 'test', 'helpers', 'quarantine-register.js'),
    '--test',
    '--test-reporter', 'spec',
    ...files
], {
    cwd: root,
    env: Object.assign({}, process.env, { HEBREW_QUARANTINE_SEEN: seenDir }),
    stdio: 'inherit'
});

child.on('exit', (code, signal) => {
    let entries;
    try {
        entries = quarantine.loadList();
    } catch (err) {
        console.error(err && err.message ? err.message : err);
        process.exit(1);
    }
    const seen = new Set();
    if (fs.existsSync(seenDir)) {
        for (const name of fs.readdirSync(seenDir)) {
            const text = fs.readFileSync(path.join(seenDir, name), 'utf8');
            for (const line of text.split('\n')) {
                if (!line) continue;
                const row = JSON.parse(line);
                seen.add(quarantine.entryKey(row.file, row.name));
            }
        }
    }
    const missing = quarantine.missingEntries(entries, seen);
    const matched = entries.length - missing.length;
    console.log('# quarantined ' + matched);
    try { fs.rmSync(seenDir, { recursive: true, force: true }); } catch (err) { /* ignore */ }
    if (missing.length) {
        console.error('quarantine list is stale; these tests were not registered:');
        for (const entry of missing) {
            console.error('  ' + entry.file + ' :: ' + entry.name);
        }
        process.exit(1);
    }
    if (signal) {
        process.exit(1);
    }
    process.exit(code == null ? 1 : code);
});
