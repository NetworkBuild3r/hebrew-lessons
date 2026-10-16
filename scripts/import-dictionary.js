/**
 * Load the Hebrew–English dictionary into the app database.
 * Put Hebrew-English.idx, .dict.dz and .syn.dz in data/dictionary first.
 * The server also does this on start when the table is empty.
 *
 *   node scripts/import-dictionary.js
 */
'use strict';

const db = require('../server/db');
const load = require('../server/dictionary/load');
const { lookup } = require('../server/dictionary/lookup');

if (!load.filesPresent()) {
    process.stderr.write('No dictionary files in ' + load.folder() + '\n');
    process.exit(1);
}
db.migrate();
const { loaded } = load.load();
process.stdout.write('Loaded ' + loaded + ' entries\n');
['אבא', 'אוכל', 'לאכול', 'רוצה'].forEach((word) => {
    const found = lookup(word);
    const line = found.meanings.map((m) => (m.pos ? m.pos + ': ' : '') + m.en).join(' | ');
    process.stdout.write(word + (found.form ? ' (' + found.form + ')' : '') + ' → ' + line + '\n');
});
db.close();
