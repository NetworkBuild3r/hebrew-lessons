const fs = require('fs');
const path = require('path');
const paths = require('./paths');

// Local runs keep secrets in .env. Do not override variables already set in the process.
const envFile = path.join(paths.rootDir(), '.env');
if (fs.existsSync(envFile)) {
    fs.readFileSync(envFile, 'utf8').split(/\r?\n/).forEach((line) => {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) return;
        const eq = trimmed.indexOf('=');
        if (eq < 1) return;
        const key = trimmed.slice(0, eq).trim();
        let value = trimmed.slice(eq + 1).trim();
        if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
            value = value.slice(1, -1);
        }
        if (process.env[key] === undefined) process.env[key] = value;
    });
}

// Fail fast if DATA_DIR sits under a served root (assertDataDirNotServed).
paths.dataDir();

const mcpToken = process.env.MCP_AUTH_TOKEN;
if (!mcpToken || !String(mcpToken).trim()) {
    console.error('MCP_AUTH_TOKEN is required but missing or empty. Set it in the environment (e.g. .env) and restart.');
    process.exit(1);
}

const seeded = paths.seedLessons();
if (seeded.seeded) {
    console.log('Seeded lessons into ' + seeded.dir);
} else {
    console.log('Lessons directory: ' + seeded.dir);
}

const db = require('./db');
db.open();
db.migrate();

const visibility = require('./visibility');
const seed = visibility.seedVisibility();
if (seed.seeded) {
    console.log('Seeded lesson visibility for ' + seed.inserted.length + ' lesson(s)');
}

const lexiconSeed = require('./lexicon/seed').run();
console.log(
    'Lexicon seed: ' + lexiconSeed.inserted + ' inserted, '
    + lexiconSeed.updated + ' updated, ' + lexiconSeed.kept + ' kept'
);
if (lexiconSeed.skipped) {
    console.warn('[lexicon-seed] skipped ' + lexiconSeed.skipped + ' vocab entry(ies)');
}

// Hebrew–English dictionary for tap-a-word. Loads once, when data/dictionary has the files.
try {
    const loaded = require('./dictionary/load').loadIfEmpty();
    if (loaded.loaded) console.log('Dictionary: ' + loaded.loaded + ' entries loaded');
    else console.log('Dictionary: ' + loaded.skipped);
} catch (err) {
    console.warn('[dictionary] not loaded: ' + (err && err.message ? err.message : err));
}

// First admin only. An existing admin (even a disabled one) wins over the env.
try {
    const boot = require('./bootstrap-admin').ensureBootstrapAdmin();
    if (boot.created) {
        console.log('Created admin user "' + boot.username + '" from the environment.');
    }
} catch (err) {
    console.error(err && err.message ? err.message : err);
    process.exit(1);
}

require('./web').listen();
require('../mcp/http').listen();
