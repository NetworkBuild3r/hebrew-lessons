/**
 * Lexicon repository — prepared statements only. No file IO.
 * Keys are stored as exactKey (NFC, collapsed whitespace, trimmed).
 * Provenance: INIT-006/SPEC-003
 */
'use strict';

const db = require('../index');

const COLUMNS = [
    'key', 'kind', 'lemma_key', 'gloss_en', 'pos', 'root', 'binyan', 'root_class',
    'tense', 'cell', 'translit', 'accept_json', 'verified', 'verified_by',
    'source', 'rev', 'updated_at'
];

/**
 * ADR-0001 exactKey: NFC, collapse whitespace, trim.
 * @param {string|null|undefined} text
 * @returns {string}
 */
function exactKey(text) {
    return String(text == null ? '' : text).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/**
 * @param {string|null|undefined} text
 * @returns {string|null}
 */
function exactKeyOrNull(text) {
    if (text == null) return null;
    const key = exactKey(text);
    return key.length === 0 ? null : key;
}

/**
 * @param {boolean|number|null|undefined} value
 * @returns {number}
 */
function verifiedFlag(value) {
    if (value === true || value === 1) return 1;
    if (value == null || value === false || value === 0) return 0;
    return value;
}

/**
 * Insert or replace one lexicon row. Does not check that lemma_key names a lemma.
 * @param {object} row
 * @returns {import('node:sqlite').StatementResultingChanges}
 */
function upsert(row) {
    const key = exactKey(row.key);
    const lemmaKey = exactKeyOrNull(row.lemma_key);
    return db.run(
        `INSERT INTO lexicon_entries (
            key, kind, lemma_key, gloss_en, pos, root, binyan, root_class,
            tense, cell, translit, accept_json, verified, verified_by,
            source, rev, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(key) DO UPDATE SET
            kind = excluded.kind,
            lemma_key = excluded.lemma_key,
            gloss_en = excluded.gloss_en,
            pos = excluded.pos,
            root = excluded.root,
            binyan = excluded.binyan,
            root_class = excluded.root_class,
            tense = excluded.tense,
            cell = excluded.cell,
            translit = excluded.translit,
            accept_json = excluded.accept_json,
            verified = excluded.verified,
            verified_by = excluded.verified_by,
            source = excluded.source,
            rev = excluded.rev,
            updated_at = excluded.updated_at`,
        key,
        row.kind,
        lemmaKey,
        row.gloss_en == null ? null : row.gloss_en,
        row.pos == null ? null : row.pos,
        row.root == null ? null : row.root,
        row.binyan == null ? null : row.binyan,
        row.root_class == null ? null : row.root_class,
        row.tense == null ? null : row.tense,
        row.cell == null ? null : row.cell,
        row.translit == null ? null : row.translit,
        row.accept_json == null ? '[]' : row.accept_json,
        verifiedFlag(row.verified),
        row.verified_by == null ? null : row.verified_by,
        row.source,
        row.rev == null ? 0 : row.rev,
        row.updated_at || new Date().toISOString()
    );
}

/**
 * @param {string} key
 * @returns {object|undefined}
 */
function find(key) {
    return db.get(
        'SELECT ' + COLUMNS.join(', ') + ' FROM lexicon_entries WHERE key = ?',
        exactKey(key)
    );
}

/**
 * @param {string} lemmaKey
 * @returns {object[]}
 */
function listByLemma(lemmaKey) {
    return db.all(
        'SELECT ' + COLUMNS.join(', ')
            + ' FROM lexicon_entries WHERE lemma_key = ? ORDER BY key',
        exactKey(lemmaKey)
    );
}

/**
 * @param {string} binyan
 * @returns {object[]}
 */
function listByBinyan(binyan) {
    return db.all(
        'SELECT ' + COLUMNS.join(', ')
            + ' FROM lexicon_entries WHERE binyan = ? ORDER BY key',
        binyan
    );
}

/**
 * Keys of rows whose kind is lemma. Used to resolve a form's lemma_key.
 * Provenance: INIT-006/SPEC-005
 * @returns {string[]}
 */
function listLemmaKeys() {
    return db.all(
        "SELECT key FROM lexicon_entries WHERE kind = 'lemma' ORDER BY key"
    ).map((row) => row.key);
}

/** Stay under SQLite's bound-variable limit when a rebuild names many keys. */
const IN_CHUNK = 200;

/**
 * Rows whose key is in `keys`, in one query per chunk. Empty input is no query.
 * Provenance: INIT-006/SPEC-006
 * @param {string[]} keys
 * @returns {object[]}
 */
function listByKeys(keys) {
    const unique = [];
    const seen = new Set();
    const list = Array.isArray(keys) ? keys : [];
    for (const raw of list) {
        const key = exactKey(raw);
        if (!key || seen.has(key)) continue;
        seen.add(key);
        unique.push(key);
    }
    if (!unique.length) return [];
    const out = [];
    for (let i = 0; i < unique.length; i += IN_CHUNK) {
        const slice = unique.slice(i, i + IN_CHUNK);
        const marks = slice.map(() => '?').join(', ');
        out.push(...db.all(
            'SELECT ' + COLUMNS.join(', ') + ' FROM lexicon_entries WHERE key IN (' + marks + ')',
            ...slice
        ));
    }
    return out;
}

/**
 * ADR-0001 strippedKey: exactKey without vowel points and cantillation (U+0591 to U+05C7).
 * @param {string|null|undefined} text
 * @returns {string}
 */
function strippedKey(text) {
    return exactKey(exactKey(text).replace(/[\u0591-\u05C7]/g, ''));
}

/**
 * Rows whose stripped key is in `keys`. The table stores no stripped column, so this
 * reads the key column once, matches in memory, and fetches the matching rows: one scan
 * per call, no schema change. Read only. Used by the plan `lexicon-form` check.
 * Provenance: INIT-008/SPEC-008
 * @param {string[]} keys Stripped keys (they are stripped again, so pointed input also works).
 * @returns {object[]}
 */
function listByStrippedKeys(keys) {
    const wanted = new Set();
    for (const raw of Array.isArray(keys) ? keys : []) {
        const stripped = strippedKey(raw);
        if (stripped) wanted.add(stripped);
    }
    if (!wanted.size) return [];
    const matched = db.all('SELECT key FROM lexicon_entries')
        .map((row) => row.key)
        .filter((key) => wanted.has(strippedKey(key)));
    return listByKeys(matched);
}

/**
 * Keyset page. Every column name is fixed in this function; values are bound.
 * `q` is already escaped for LIKE. `limit` includes the extra row the
 * service uses to decide whether another page exists.
 * Provenance: INIT-006/SPEC-005
 * @param {{ q: string|null, pos: string|null, binyan: string|null, tense: string|null, rootClass: string|null, verified: number|null, cursor: string|null, limit: number }} filters
 * @returns {object[]}
 */
function search(filters) {
    const where = [];
    const params = [];
    if (filters.q) {
        where.push(
            "(key LIKE ? ESCAPE '\\' OR gloss_en LIKE ? ESCAPE '\\' OR translit LIKE ? ESCAPE '\\')"
        );
        params.push(filters.q, filters.q, filters.q);
    }
    if (filters.pos) {
        where.push('pos = ?');
        params.push(filters.pos);
    }
    if (filters.binyan) {
        where.push('binyan = ?');
        params.push(filters.binyan);
    }
    if (filters.tense) {
        where.push('tense = ?');
        params.push(filters.tense);
    }
    if (filters.rootClass) {
        where.push('root_class = ?');
        params.push(filters.rootClass);
    }
    if (filters.verified === 0 || filters.verified === 1) {
        where.push('verified = ?');
        params.push(filters.verified);
    }
    if (filters.cursor) {
        where.push('key > ?');
        params.push(filters.cursor);
    }
    const sql = 'SELECT ' + COLUMNS.join(', ') + ' FROM lexicon_entries'
        + (where.length ? ' WHERE ' + where.join(' AND ') : '')
        + ' ORDER BY key LIMIT ?';
    params.push(filters.limit);
    return db.all(sql, ...params);
}

module.exports = {
    upsert,
    find,
    listByLemma,
    listByBinyan,
    listLemmaKeys,
    listByKeys,
    listByStrippedKeys,
    search,
    exactKey,
    strippedKey
};
