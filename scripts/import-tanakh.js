/**
 * Load the Tanakh corpus into the app database from data/tanakh.
 * Fetch the files first: node scripts/fetch-tanakh.js
 * The server also does this on start when the table is empty.
 *
 *   node scripts/import-tanakh.js
 */
'use strict';

const db = require('../server/db');
const load = require('../server/tanakh/load');
const tanakh = require('../server/db/repos/tanakh');

if (!load.filesPresent()) {
    process.stderr.write('No Tanakh files in ' + load.folder() + ' — run: node scripts/fetch-tanakh.js\n');
    process.exit(1);
}
db.migrate();
const { loaded } = load.load();
process.stdout.write('Loaded ' + loaded + ' verses across ' + tanakh.books().length + ' books\n');

const meta = tanakh.allMeta();
process.stdout.write('Versions: ' + meta.version_he + ' (he), ' + meta.version_en + ' (en)\n');
process.stdout.write('Licenses: ' + meta.license_he + ' (he), ' + meta.license_en + ' (en)\n');

[
    ['genesis', 1, 1],
    ['deuteronomy', 6, 4],
    ['psalms', 23, 1]
].forEach(([book, chapter, verse]) => {
    const row = tanakh.verse(book, chapter, verse);
    if (!row) return;
    process.stdout.write(book + ' ' + chapter + ':' + verse + ' — ' + row.he + '\n  ' + row.en + '\n');
});
db.close();
