/**
 * Grammar-concept registry — prepared statements only. No file IO.
 * The migration seeds the rows. This module only reads them.
 * Provenance: INIT-006/SPEC-003
 */
'use strict';

const db = require('../index');

const COLUMNS = 'key, family, label_en, label_he, colour, sort';

/**
 * @param {string|null|undefined} text
 * @returns {string}
 */
function exactKey(text) {
    return String(text == null ? '' : text).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/**
 * Every concept, in registry order.
 * @returns {object[]}
 */
function list() {
    return db.all(
        'SELECT ' + COLUMNS + ' FROM grammar_concepts ORDER BY sort ASC, key ASC'
    );
}

/**
 * @param {string} key
 * @returns {object|undefined}
 */
function find(key) {
    return db.get(
        'SELECT ' + COLUMNS + ' FROM grammar_concepts WHERE key = ?',
        exactKey(key)
    );
}

module.exports = {
    list,
    find
};
