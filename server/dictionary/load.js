/**
 * Load the Hebrew–English dictionary files from data/dictionary into
 * dictionary_entries. The files stay under data/ (not served, not in git).
 */
'use strict';

const fs = require('fs');
const path = require('path');

const { dataDir } = require('../paths');
const dictionary = require('../db/repos/dictionary');
const { parseStarDictZipped } = require('./stardict');

const BASE = 'Hebrew-English';

/**
 * @returns {string}
 */
function folder() {
    return path.join(dataDir(), 'dictionary');
}

/**
 * @returns {boolean}
 */
function filesPresent() {
    const dir = folder();
    return ['.idx', '.dict.dz'].every((ext) => fs.existsSync(path.join(dir, BASE + ext)));
}

/**
 * Read the files and replace the table.
 * @returns {{ loaded: number }}
 */
function load() {
    const dir = folder();
    const synFile = path.join(dir, BASE + '.syn.dz');
    const rows = parseStarDictZipped({
        idx: fs.readFileSync(path.join(dir, BASE + '.idx')),
        dictDz: fs.readFileSync(path.join(dir, BASE + '.dict.dz')),
        synDz: fs.existsSync(synFile) ? fs.readFileSync(synFile) : null
    });
    return { loaded: dictionary.replaceEntries(rows) };
}

/**
 * Fill the table once, when it is empty and the files are there.
 * @returns {{ loaded: number, skipped: string }}
 */
function loadIfEmpty() {
    if (dictionary.countEntries() > 0) return { loaded: 0, skipped: 'already loaded' };
    if (!filesPresent()) return { loaded: 0, skipped: 'no files in ' + folder() };
    return { loaded: load().loaded, skipped: '' };
}

module.exports = { filesPresent, folder, load, loadIfEmpty };
