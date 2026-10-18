/**
 * Quarantine list shape and the skip/rot helpers. The runner's full check
 * (every list entry was registered) happens in scripts/run-tests.js.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const q = require('./helpers/quarantine');
const chromeCdp = require('./helpers/chrome-cdp');

const ARCHIVE_NAMES = [
    'ac-2/ac-4: inventory outside LESSONS_DIR; verify detects loss and accepts archive',
    'ac-3: generateAudio archives changed sentence; verify passes; forced verify fails',
    'ac-4: audioPlan is dry (no network/files) and counts match generateAudio',
    'fails when a recording is altered, moved to _archive, or added without a new inventory'
];

describe('quarantine list', () => {
    const entries = q.loadList();

    it('has one entry per quarantined test with the required fields', () => {
        assert.equal(entries.length, 43);
        const keys = new Set();
        for (const entry of entries) {
            for (const field of q.REQUIRED) assert.equal(typeof entry[field], 'string');
            assert.ok(entry.file.startsWith('test/') && entry.file.endsWith('.test.js'));
            assert.ok(entry.category === 'stale' || entry.category === 'real-bug');
            keys.add(q.entryKey(entry.file, entry.name));
        }
        assert.equal(keys.size, entries.length);
        assert.equal(entries.filter((entry) => entry.category === 'stale').length, 37);
        assert.equal(entries.filter((entry) => entry.category === 'real-bug').length, 6);
    });

    it('does not quarantine the archive-path tests', () => {
        for (const name of ARCHIVE_NAMES) {
            assert.equal(entries.some((entry) => entry.name === name), false);
        }
    });

    it('uses the agreed reason and owner for each real bug', () => {
        const audio = entries.filter((entry) => entry.file === 'test/plan-hebrew-checks.test.js' && entry.category === 'real-bug');
        assert.equal(audio.length, 2);
        for (const entry of audio) {
            assert.equal(entry.reason, 'real bug: missing audio pair registry row');
            assert.equal(entry.owner, 'Hebrew Backend Dev, fix in Go port');
        }
        const jumps = entries.filter((entry) => entry.file === 'test/plan-editor-issues-preview.test.js' && entry.category === 'real-bug');
        assert.equal(jumps.length, 3);
        for (const entry of jumps) {
            assert.equal(entry.reason, "real bug: issue links don't open the target form; UI being replaced");
            assert.equal(entry.owner, 'Hebrew Frontend Dev');
        }
        const deep = entries.find((entry) => entry.file === 'test/plan-validate.test.js');
        assert.equal(deep.reason, 'real bug, expected-to-be-fixed: should report /extra or a clear depth-limit error');
        assert.equal(deep.owner, 'Hebrew Backend Dev, fix in Go port');
    });

    it('cites the intentional commit on stale tests', () => {
        for (const entry of entries.filter((item) => item.category === 'stale')) {
            assert.equal(entry.owner, 'Hebrew QA');
            const commit = entry.file === 'test/plan-builder-mcp.test.js' ? '59d3367' : 'e3fb466';
            assert.equal(entry.reason, 'stale: behavior changed intentionally in ' + commit);
        }
    });

    it('missingEntries reports a name that was never registered', () => {
        const seen = new Set(entries.map((entry) => q.entryKey(entry.file, entry.name)));
        assert.deepEqual(q.missingEntries(entries, seen), []);
        const dropped = entries[0];
        seen.delete(q.entryKey(dropped.file, dropped.name));
        const missing = q.missingEntries(entries, seen);
        assert.equal(missing.length, 1);
        assert.equal(missing[0].name, dropped.name);
    });

    it('install skips a quarantined name from this file and leaves others alone', () => {
        const seen = [];
        const calls = [];
        function orig(name, options, fn) {
            calls.push({ name, options, fn });
            return 'orig';
        }
        orig.skip = function skip() { return 'skip'; };
        const wrapped = q.install(orig, [{
            file: 'test/quarantine.test.js',
            name: 'sentinel-quarantine-name',
            category: 'stale',
            reason: 'because',
            owner: 'Hebrew QA'
        }], (file, name) => seen.push({ file, name }));

        const fn = () => {};
        wrapped('sentinel-quarantine-name', fn);
        assert.equal(calls[0].name, 'sentinel-quarantine-name');
        assert.equal(calls[0].options.skip, 'because');
        assert.equal(calls[0].fn, fn);
        assert.deepEqual(seen, [{ file: 'test/quarantine.test.js', name: 'sentinel-quarantine-name' }]);

        const opts = { timeout: 5 };
        wrapped('plain', opts, fn);
        assert.equal(calls[1].options, opts);
        assert.equal(calls[1].name, 'plain');
        assert.equal(wrapped.skip, orig.skip);
        assert.equal(wrapped.it, wrapped);
        assert.equal(wrapped.test, wrapped);
    });
});

describe('chrome discovery', () => {
    it('tries CHROME_PATH before the Windows, Linux, and macOS candidates', () => {
        const seen = [];
        const found = chromeCdp.findChrome({ CHROME_PATH: '/opt/custom/chrome' }, (candidate) => {
            seen.push(candidate);
            return candidate.endsWith('/chromium');
        });
        assert.equal(found, '/usr/bin/chromium');
        assert.deepEqual(seen, [
            '/opt/custom/chrome',
            'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
            '/usr/bin/google-chrome-stable',
            '/usr/bin/google-chrome',
            '/usr/bin/chromium'
        ]);
    });

    it('returns CHROME_PATH when that file exists', () => {
        const found = chromeCdp.findChrome({ CHROME_PATH: '/opt/custom/chrome' }, (candidate) => candidate === '/opt/custom/chrome');
        assert.equal(found, '/opt/custom/chrome');
    });

    it('returns null when nothing exists', () => {
        assert.equal(chromeCdp.findChrome({ CHROME_PATH: '/missing' }, () => false), null);
        assert.equal(chromeCdp.findChrome({}, () => false), null);
    });

    it('skipIfMissing registers a skipped describe and does not run the body', () => {
        const file = path.join(os.tmpdir(), 'heb-no-chrome-smoke.js');
        const helper = path.join(__dirname, 'helpers', 'chrome-cdp.js');
        fs.writeFileSync(file, `
            const cdp = require(${JSON.stringify(helper)});
            let started = false;
            if (!cdp.skipIfMissing('demo-smoke', null)) started = true;
            if (started) { console.error('STARTED'); process.exit(2); }
        `);
        const out = execFileSync(process.execPath, [file], { encoding: 'utf8' });
        assert.match(out, /Chrome not found \(set CHROME_PATH\)/);
        assert.equal(out.includes('STARTED'), false);
    });
});
