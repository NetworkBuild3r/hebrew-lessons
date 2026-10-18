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

    it('keeps real-bug and environment entries only, each with an owner and an issue or expiry', () => {
        assert.equal(entries.length, 6);
        const keys = new Set();
        for (const entry of entries) {
            for (const field of q.REQUIRED) assert.equal(typeof entry[field], 'string');
            assert.ok(entry.category === 'real-bug' || entry.category === 'environment');
            assert.ok(entry.issue || entry.expires);
            keys.add(q.entryKey(entry.file, entry.name));
        }
        assert.equal(keys.size, entries.length);
        assert.equal(entries.filter((entry) => entry.category === 'stale').length, 0);
        assert.equal(entries.filter((entry) => entry.category === 'real-bug').length, 6);
        assert.equal(q.expiredEntries(entries, '2026-10-18').length, 0);
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

    it('unmatchedSkips reports a test that was not skipped with its reason', () => {
        const seen = entries.map((entry) => ({
            file: path.join(q.ROOT, entry.file),
            name: entry.name,
            skip: entry.reason
        }));
        assert.deepEqual(q.unmatchedSkips(entries, seen), []);
        const dropped = seen[0];
        const missing = q.unmatchedSkips(entries, seen.slice(1));
        assert.equal(missing.length, 1);
        assert.equal(missing[0].name, dropped.name);
        const wrong = seen.slice();
        wrong[0] = Object.assign({}, wrong[0], { skip: 'other' });
        assert.equal(q.unmatchedSkips(entries, wrong).length, 1);
    });

    it('optionsFor skips a listed test and throws once expires is past', () => {
        const entry = {
            file: 'test/quarantine.test.js',
            name: 'sentinel-quarantine-name',
            category: 'real-bug',
            reason: 'because',
            owner: 'Hebrew QA',
            expires: '2000-01-01'
        };
        assert.equal(q.expiredEntries([entry], '2000-01-01').length, 0);
        assert.equal(q.expiredEntries([entry], '2000-01-02').length, 1);
        const live = q.optionsFor([entry], entry.file, entry.name, { timeout: 5 }, '2000-01-01');
        assert.equal(live.skip, 'because');
        assert.equal(live.timeout, 5);
        assert.throws(
            () => q.optionsFor([entry], entry.file, entry.name, { timeout: 5 }, '2000-01-02'),
            /quarantine expired/
        );
        assert.deepEqual(q.optionsFor([entry], entry.file, 'plain', { timeout: 5 }, '2099-01-01'), { timeout: 5 });
    });

    it('loadList rejects a stale category and an entry with no issue or expiry', () => {
        const file = path.join(os.tmpdir(), 'heb-quarantine-bad.json');
        const base = {
            file: 'test/quarantine.test.js',
            name: 'n',
            category: 'stale',
            reason: 'r',
            owner: 'o'
        };
        fs.writeFileSync(file, JSON.stringify([base]));
        assert.throws(() => q.loadList(file), /category/);
        base.category = 'real-bug';
        fs.writeFileSync(file, JSON.stringify([base]));
        assert.throws(() => q.loadList(file), /issue or expires/);
        base.expires = '2026-02-31';
        fs.writeFileSync(file, JSON.stringify([base]));
        assert.throws(() => q.loadList(file), /expires/);
    });

    it('literal test titles are unique, so a skip or a failure names one test', () => {
        const re = /\bit\(\s*(['"`])((?:\\.|(?!\1).)*?)\1/g;
        const names = new Map();
        const dir = path.join(q.ROOT, 'test');
        for (const file of fs.readdirSync(dir).filter((name) => name.endsWith('.test.js'))) {
            const text = fs.readFileSync(path.join(dir, file), 'utf8');
            re.lastIndex = 0;
            let match;
            while ((match = re.exec(text))) {
                const after = text.slice(match.index + match[0].length);
                if (/^\s*\+/.test(after)) continue;
                const title = match[2];
                if (!names.has(title)) names.set(title, []);
                names.get(title).push('test/' + file);
            }
        }
        const dupes = [];
        for (const [title, files] of names) {
            if (files.length > 1) dupes.push(title + ' @ ' + files.join(', '));
        }
        assert.deepEqual(dupes, []);
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
