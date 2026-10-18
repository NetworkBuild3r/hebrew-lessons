#!/usr/bin/env node
'use strict';

// Regenerates the committed due-count and exactKey goldens from the Node
// server at this commit. Go tests read the files and do not run Node.
//
//   node go/tools/golden/path.js
//
// Node 22.14.0, process.versions.unicode 16.0. CI runs this and fails on
// git diff.

const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

if (process.versions.unicode !== '16.0') {
    console.error(
        'path goldens require process.versions.unicode 16.0, got '
        + process.versions.unicode + ' (' + process.version + ')'
    );
    process.exit(1);
}

const root = path.resolve(__dirname, '../../..');
const dueCasesPath = path.join(root, 'go/internal/learner/path/testdata/due-cases.jsonl');
const dueGoldenDir = path.join(root, 'go/internal/learner/path/testdata/golden');
const keyGoldenDir = path.join(root, 'go/internal/textkey/testdata/golden');
const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-path-golden-'));

function run(script, args, options) {
    const result = spawnSync(process.execPath, [script].concat(args), Object.assign({
        cwd: root,
        env: Object.assign({}, process.env, { TZ: 'UTC' })
    }, options));
    if (result.status !== 0) {
        const err = result.stderr ? result.stderr.toString() : '';
        console.error(err);
        process.exit(result.status == null ? 1 : result.status);
    }
    return result.stdout ? result.stdout.toString() : '';
}

const cases = fs.readFileSync(dueCasesPath, 'utf8')
    .split('\n')
    .filter((line) => line.trim() !== '')
    .map((line) => JSON.parse(line));
const seedBody = {
    users: cases.map((item) => item.username),
    reviews: []
};
for (const item of cases) {
    for (const row of item.rows) {
        seedBody.reviews.push(Object.assign({ username: item.username }, row));
    }
}

run(
    path.join(root, 'go/internal/learner/path/testdata/seed.js'),
    [dataDir],
    { input: JSON.stringify(seedBody) }
);

const dueOut = run(
    path.join(root, 'go/internal/learner/path/testdata/due-oracle.js'),
    [dataDir, dueCasesPath]
);

const { DatabaseSync } = require('node:sqlite');
const database = new DatabaseSync(path.join(dataDir, 'hebrew.sqlite'));
database.exec("UPDATE schema_migrations SET applied_at = '2026-01-01T00:00:00.000Z'");
database.exec('PRAGMA wal_checkpoint(TRUNCATE)');
database.close();
for (const suffix of ['-wal', '-shm']) {
    const side = path.join(dataDir, 'hebrew.sqlite' + suffix);
    if (fs.existsSync(side)) fs.rmSync(side);
}

fs.mkdirSync(dueGoldenDir, { recursive: true });
fs.copyFileSync(path.join(dataDir, 'hebrew.sqlite'), path.join(dueGoldenDir, 'due.sqlite'));
fs.writeFileSync(path.join(dueGoldenDir, 'due.jsonl'), dueOut.endsWith('\n') ? dueOut : dueOut + '\n');

const keyDir = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-exact-key-golden-'));
const keyOut = run(
    path.join(root, 'go/internal/textkey/testdata/exact-key-oracle.js'),
    [],
    { env: Object.assign({}, process.env, { TZ: 'UTC', DATA_DIR: keyDir }) }
);
fs.mkdirSync(keyGoldenDir, { recursive: true });
fs.writeFileSync(path.join(keyGoldenDir, 'exact-key.jsonl'), keyOut.endsWith('\n') ? keyOut : keyOut + '\n');

fs.rmSync(dataDir, { recursive: true, force: true });
fs.rmSync(keyDir, { recursive: true, force: true });
