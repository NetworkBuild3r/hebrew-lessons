#!/usr/bin/env node
/**
 * Checkpoint WAL and copy DATA_DIR/hebrew.sqlite to a dated flat file under
 * DATA_DIR/backups/. Prints the destination path and byte size only — never
 * row contents or dumps.
 *
 *   node scripts/backup-db.js
 *
 * Does not migrate, does not copy media/, and does not replace the live file.
 * For the full media bundle, use `npm run db:backup` (scripts/db.js), which
 * writes DATA_DIR/backups/hebrew-<stamp>/hebrew.sqlite plus media trees.
 *
 * Provenance: INIT-006/SPEC-020
 */
'use strict';

const fs = require('fs');
const path = require('path');
const db = require('../server/db');

function isoStamp(now) {
    return now.toISOString().replace(/[:.]/g, '-');
}

function main() {
    db.open();
    const src = db.databasePath();
    if (!fs.existsSync(src)) {
        throw new Error('Database file not found: ' + src);
    }
    // Fold WAL into the main file so the copy is consistent.
    db.get('PRAGMA wal_checkpoint(TRUNCATE)');
    db.close();

    const root = path.dirname(src);
    const backupsDir = path.join(root, 'backups');
    fs.mkdirSync(backupsDir, { recursive: true });
    const dest = path.join(backupsDir, 'hebrew-' + isoStamp(new Date()) + '.sqlite');
    fs.copyFileSync(src, dest);
    const size = fs.statSync(dest).size;
    console.log(dest);
    console.log(String(size) + ' bytes');
}

try {
    main();
} catch (err) {
    console.error(err && err.message ? err.message : err);
    try { db.close(); } catch (_) { /* ignore */ }
    process.exit(1);
}
