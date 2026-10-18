'use strict';

// Seeds a migrated hebrew.sqlite for the Go learner tests.
// The Go service does not write. This script runs only from go test.
//
// usage: node seed.js <data-dir>
// stdin: JSON { users, reviews, plans, progress, grants }

const fs = require('fs');
const path = require('path');

const dataDirArg = process.argv[2];
if (!dataDirArg) {
    console.error('usage: seed.js <data-dir>');
    process.exit(2);
}

const repoRoot = path.resolve(__dirname, '../../../../..');
const dataDir = path.resolve(dataDirArg);
const lessonsDir = path.join(repoRoot, 'worksheets');
if (dataDir === lessonsDir || dataDir.startsWith(lessonsDir + path.sep)) {
    console.error('DATA_DIR is inside LESSONS_DIR: ' + dataDir);
    process.exit(2);
}

const raw = fs.readFileSync(0, 'utf8');
let body;
try {
    body = JSON.parse(raw);
} catch (err) {
    console.error('seed stdin: ' + err.message);
    process.exit(2);
}

process.env.HEBREW_ROOT = repoRoot;
process.env.DATA_DIR = dataDir;
process.env.LESSONS_DIR = lessonsDir;
fs.mkdirSync(dataDir, { recursive: true });

const db = require(path.join(repoRoot, 'server', 'db'));
db.migrate();

const now = '2026-10-18T00:00:00.000Z';
const users = Object.create(null);

for (const username of body.users || []) {
    db.run(
        `INSERT INTO users (
            username, password_hash, display_name, role, status,
            auth_provider, provider_subject, must_change_password,
            created_at, updated_at
        ) VALUES (?, 'x', NULL, 'user', 'active', 'local', NULL, 0, ?, ?)`,
        String(username).toLowerCase(),
        now,
        now
    );
    const row = db.get('SELECT id FROM users WHERE username = ?', String(username).toLowerCase());
    users[String(username).toLowerCase()] = row.id;
}

function userId(username) {
    const id = users[String(username).toLowerCase()];
    if (!id) {
        throw new Error('unknown user ' + username);
    }
    return id;
}

for (const row of body.reviews || []) {
    db.run(
        `INSERT INTO review_items (
            user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources, suspended
        ) VALUES (?, ?, ?, ?, 0, 0, 0, ?, ?, ?, ?)`,
        userId(row.username),
        String(row.word),
        String(row.direction),
        String(row.due),
        row.lastResult == null || row.lastResult === '' ? null : String(row.lastResult),
        row.lastAt == null || row.lastAt === '' ? null : String(row.lastAt),
        Array.isArray(row.sources) ? JSON.stringify(row.sources) : '[]',
        row.suspended ? 1 : 0
    );
}

for (const plan of body.plans || []) {
    const status = plan.status || 'published';
    const audience = plan.audience;
    const assignee = audience === 'personal' ? String(plan.username).toLowerCase() : null;
    let snapshot = null;
    if (typeof plan.snapshotText === 'string') {
        snapshot = plan.snapshotText;
    } else if (plan.snapshot) {
        const snap = plan.snapshot;
        snap.id = plan.id;
        snap.schemaVersion = snap.schemaVersion || '1.2.0';
        snap.revision = snap.revision || 1;
        snap.status = status;
        snap.audience = audience;
        snap.assigneeId = assignee;
        if (!snap.title) snap.title = plan.id;
        if (!snap.language) snap.language = { target: 'he', gloss: 'en' };
        snapshot = JSON.stringify(snap);
    }
    db.run(
        `INSERT INTO plans (
            id, revision, status, audience, assignee_id,
            working_copy, snapshot, created_at, updated_at
        ) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?)`,
        plan.id,
        status,
        audience,
        assignee,
        snapshot || '{}',
        snapshot,
        now,
        now
    );
}

for (const row of body.progress || []) {
    db.run(
        `INSERT INTO lesson_progress (user_id, slug, state_json, rev, updated_at)
         VALUES (?, ?, ?, 1, ?)`,
        userId(row.username),
        String(row.slug),
        JSON.stringify({ completedRounds: row.completedRounds }),
        now
    );
}

for (const row of body.grants || []) {
    db.run(
        'INSERT INTO lesson_grants (slug, user_id) VALUES (?, ?)',
        String(row.slug),
        userId(row.username)
    );
}

db.close();
process.stdout.write(JSON.stringify({ users: users }) + '\n');
