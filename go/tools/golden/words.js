#!/usr/bin/env node
'use strict';

// Regenerates the committed words-backend goldens from the Node server at
// this commit. Go tests read the files and do not run Node.
//
//   node go/tools/golden/words.js
//
// Node 22.14.0. CI runs this and fails on git diff.

const { spawnSync } = require('child_process');
const path = require('path');

const root = path.resolve(__dirname, '../../..');
const scripts = [
    'go/internal/auth/testdata/sessions.js',
    'go/internal/learner/library/testdata/golden.js',
    'go/internal/httpx/testdata/origin.js'
];

for (const rel of scripts) {
    const result = spawnSync(process.execPath, [path.join(root, rel)], {
        cwd: root,
        stdio: 'inherit',
        env: Object.assign({}, process.env, { TZ: 'UTC' })
    });
    if (result.status !== 0) {
        process.exit(result.status == null ? 1 : result.status);
    }
}
