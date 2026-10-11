#!/usr/bin/env node
/**
 * Database CLI: migrate, backup, check.
 * Provenance: INIT-002/SPEC-003, media trees INIT-004/SPEC-017
 */
'use strict';

const path = require('path');
const db = require('../server/db');

function usage() {
    console.error('Usage: node scripts/db.js <migrate|backup|check>');
    process.exit(1);
}

const cmd = process.argv[2];
if (!cmd) usage();

try {
    if (cmd === 'migrate') {
        db.open();
        db.migrate();
        console.log('Migrations applied. Database: ' + db.databasePath());
        db.close();
    } else if (cmd === 'backup') {
        db.open();
        db.migrate();
        const dest = db.backup();
        console.log('Backup written: ' + path.dirname(dest));
        console.log('  hebrew.sqlite');
        console.log('  media/');
        console.log('  media/_archive/');
        db.close();
    } else if (cmd === 'check') {
        db.open();
        const result = db.integrityCheck();
        console.log(result);
        db.close();
        if (result !== 'ok') process.exit(1);
    } else {
        usage();
    }
} catch (err) {
    console.error(err && err.message ? err.message : err);
    try { db.close(); } catch (_) { /* ignore */ }
    process.exit(1);
}
