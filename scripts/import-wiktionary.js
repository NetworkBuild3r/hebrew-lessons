/**
 * Load the Wiktionary English–Hebrew dictionary into dictionary_glosses.
 * Source: https://github.com/open-dsl-dict/wiktionary-dict
 * License: CC BY-SA 3.0 and GFDL. The compressed file stays under data/ (gitignored).
 *
 *   node scripts/import-wiktionary.js
 */
'use strict';

const fs = require('fs');
const path = require('path');
const zlib = require('zlib');

const db = require('../server/db');
const { dataDir } = require('../server/paths');
const { parseDsl, foldEnglish, foldHebrew } = require('../server/dictionary/parse-dsl');
const dictionary = require('../server/db/repos/dictionary');

const URL = 'https://raw.githubusercontent.com/open-dsl-dict/wiktionary-dict/master/dsl/twoway/en-he-enwiktionary.dsl.dz';

async function download(url, dest) {
    const res = await fetch(url);
    if (!res.ok) throw new Error('Download failed: ' + res.status);
    const buf = Buffer.from(await res.arrayBuffer());
    fs.mkdirSync(path.dirname(dest), { recursive: true });
    fs.writeFileSync(dest, buf);
    return buf.length;
}

async function main() {
    const dest = path.join(dataDir(), 'cache', 'en-he-enwiktionary.dsl.dz');
    if (!fs.existsSync(dest)) {
        process.stdout.write('Downloading Wiktionary English–Hebrew…\n');
        const bytes = await download(URL, dest);
        process.stdout.write('Saved ' + bytes + ' bytes\n');
    }
    const text = zlib.gunzipSync(fs.readFileSync(dest)).toString('utf16le');
    const parsed = parseDsl(text);
    const rows = parsed.map((row) => ({
        he: row.he,
        en: row.en,
        heFold: foldHebrew(row.he),
        enFold: foldEnglish(row.en)
    }));
    db.migrate();
    const n = dictionary.replaceAll(rows);
    const sample = dictionary.searchGlosses(foldEnglish('mother'), 5);
    process.stdout.write('Imported ' + n + ' glosses\n');
    sample.forEach((row) => {
        process.stdout.write(row.he + ' — ' + row.en + '\n');
    });
    db.close();
}

main().catch((err) => {
    process.stderr.write(String(err && err.stack ? err.stack : err) + '\n');
    process.exit(1);
});
