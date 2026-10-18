'use strict';

// Builds a temp hebrew.sqlite by running the real Node migrations.
// Go tests call this. It does not change server/.
//
// usage: node apply-migrations.js <apply|missing|extra|wrong-name|empty> <data-dir>

const fs = require('fs');
const path = require('path');

const mode = process.argv[2];
const dataDirArg = process.argv[3];
if (!mode || !dataDirArg) {
    console.error('usage: apply-migrations.js <apply|missing|extra|wrong-name|empty> <data-dir>');
    process.exit(2);
}

const repoRoot = path.resolve(__dirname, '../../../..');
const dataDir = path.resolve(dataDirArg);
const lessonsDir = path.join(repoRoot, 'worksheets');
if (dataDir === lessonsDir || dataDir.startsWith(lessonsDir + path.sep)) {
    console.error('DATA_DIR is inside LESSONS_DIR: ' + dataDir);
    process.exit(2);
}

process.env.HEBREW_ROOT = repoRoot;
process.env.DATA_DIR = dataDir;
process.env.LESSONS_DIR = lessonsDir;
fs.mkdirSync(dataDir, { recursive: true });

const file = path.join(dataDir, 'hebrew.sqlite');

if (mode === 'empty') {
    const { DatabaseSync } = require('node:sqlite');
    const database = new DatabaseSync(file);
    database.exec('PRAGMA journal_mode = WAL');
    database.close();
    process.stdout.write(file + '\n');
    process.exit(0);
}

const db = require(path.join(repoRoot, 'server', 'db'));
db.migrate();

if (mode === 'missing') {
    db.run('DELETE FROM schema_migrations WHERE version = ?', 4);
} else if (mode === 'extra') {
    db.run(
        'INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)',
        12,
        '012-extra.js',
        new Date().toISOString()
    );
} else if (mode === 'wrong-name') {
    db.run(
        'UPDATE schema_migrations SET name = ? WHERE version = ?',
        '003-not-the-real-name.js',
        3
    );
} else if (mode !== 'apply') {
    console.error('unknown mode: ' + mode);
    process.exit(2);
}

db.close();
process.stdout.write(file + '\n');
