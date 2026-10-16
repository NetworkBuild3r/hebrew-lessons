/**
 * Reads for the known-word coverage check — prepared statements only.
 * One learner's library words, and the lexicon keys a text token can match.
 * Provenance: INIT-006/SPEC-007
 */
'use strict';

const db = require('../index');

/**
 * Every word in this learner's library: the words on their review rows plus
 * the words on their user_words rows (ADR-0009 §6). Both sides are bound to
 * the same user id. A passed word has a review row, so it is in this list.
 * @param {number} userId
 * @returns {string[]}
 */
function listLibraryWords(userId) {
    return db.all(
        `SELECT word FROM review_items WHERE user_id = ?
         UNION
         SELECT word FROM user_words WHERE user_id = ?`,
        userId,
        userId
    ).map((row) => String(row.word));
}

/**
 * Lemma and form keys with their kind and lemma_key. Phrase keys hold a
 * space and a token never does, so phrases are not listed.
 * @returns {{ key: string, kind: string, lemma_key: string|null }[]}
 */
function listMatchKeys() {
    return db.all(
        "SELECT key, kind, lemma_key FROM lexicon_entries WHERE kind IN ('lemma', 'form')"
    );
}

module.exports = {
    listLibraryWords,
    listMatchKeys
};
