'use strict';

// CI/test oracle for go/internal/paths. The server never runs this.
// It asks the real server/paths.js for each case. HEBREW_ROOT is set on
// every case: Node's unset fallback is path.resolve(__dirname, '..'), the
// parent of server/, and that tree is not in the Go image.

const fs = require('fs');
const path = require('path');

const repoRoot = path.resolve(__dirname, '../../../..');
const paths = require(path.join(repoRoot, 'server', 'paths'));

const raw = fs.readFileSync(0, 'utf8');
let cases;
try {
    cases = JSON.parse(raw);
} catch (err) {
    console.error('golden-paths: stdin is not JSON: ' + (err && err.message ? err.message : err));
    process.exit(2);
}
if (!Array.isArray(cases)) {
    console.error('golden-paths: stdin must be a JSON array');
    process.exit(2);
}

for (const item of cases) {
    process.chdir(item.cwd);
    const env = item.env || {};
    for (const key of ['HEBREW_ROOT', 'DATA_DIR', 'LESSONS_DIR']) {
        const value = env[key];
        if (value) process.env[key] = value;
        else delete process.env[key];
    }
    const out = { name: item.name };
    try {
        const root = paths.rootDir();
        const lessons = paths.lessonsDir();
        const data = paths.dataDir();
        out.ok = true;
        out.root = root;
        out.lessons = lessons;
        out.data = data;
        out.db = path.join(data, 'hebrew.sqlite');
        out.media = path.join(data, 'media');
    } catch (err) {
        out.ok = false;
        out.error = err && err.message ? err.message : String(err);
    }
    process.stdout.write(JSON.stringify(out) + '\n');
}
