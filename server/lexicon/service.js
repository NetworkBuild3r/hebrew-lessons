/**
 * Lexicon read/write service.
 * All lexicon writes go through upsert. setVerified is the only other
 * mutation, and it changes the verified flag alone. Adapters do not write SQL.
 * Seed refreshes source='seed' rows through applySeed and never overwrites a
 * row whose source is not seed.
 * Provenance: INIT-006/SPEC-005, INIT-012/SPEC-003
 */
'use strict';

const db = require('../db');
const lexicon = require('../db/repos/lexicon');
const concepts = require('../db/repos/concepts');
const audit = require('../db/repos/audit');
const { validateEntries, POS, BINYAN, TENSE, ROOT_CLASS } = require('./validate');

const LIST_DEFAULT = 50;
const LIST_MAX = 200;
const QUERY_MAX = 80;
const MARKUP = /[<>]/;
const CONTROL = /[\u0000-\u001F\u007F]/;

const TENSE_ORDER = ['infinitive', 'present', 'past', 'future', 'imperative'];
const CELL_ORDER = {
    infinitive: ['inf'],
    present: ['ms', 'fs', 'mp', 'fp'],
    past: ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'],
    future: ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p']
};

/** Future person cells share past pronoun labels (ADR-0014 D5). INIT-012/SPEC-003 */
const FUTURE_CELL_CONCEPTS = [
    { key: 'cell:future.1s', labelEn: 'first person singular', labelHe: 'אני' },
    { key: 'cell:future.2ms', labelEn: 'second person masculine singular', labelHe: 'אתה' },
    { key: 'cell:future.2fs', labelEn: 'second person feminine singular', labelHe: 'את' },
    { key: 'cell:future.3ms', labelEn: 'third person masculine singular', labelHe: 'הוא' },
    { key: 'cell:future.3fs', labelEn: 'third person feminine singular', labelHe: 'היא' },
    { key: 'cell:future.1p', labelEn: 'first person plural', labelHe: 'אנחנו' },
    { key: 'cell:future.2mp', labelEn: 'second person masculine plural', labelHe: 'אתם' },
    { key: 'cell:future.2fp', labelEn: 'second person feminine plural', labelHe: 'אתן' },
    { key: 'cell:future.3p', labelEn: 'third person plural', labelHe: 'הם, הן' }
];

let futureCellConceptsEnsured = false;

/**
 * Idempotent: migration 005 seeds past cells only; existing DBs get future cells on first list.
 */
function ensureFutureCellConcepts() {
    if (futureCellConceptsEnsured) return;
    const row = db.get('SELECT COALESCE(MAX(sort), 0) AS n FROM grammar_concepts');
    const baseSort = row && Number.isInteger(row.n) ? row.n : 0;
    const insert = db.prepare(
        'INSERT OR IGNORE INTO grammar_concepts (key, family, label_en, label_he, colour, sort) VALUES (?, ?, ?, ?, ?, ?)'
    );
    FUTURE_CELL_CONCEPTS.forEach((concept, index) => {
        insert.run(
            concept.key,
            'cell',
            concept.labelEn,
            concept.labelHe,
            null,
            baseSort + index + 1
        );
    });
    futureCellConceptsEnsured = true;
}

/**
 * Grammar concept registry for MCP and API. Ensures future cell keys exist.
 * @returns {object[]}
 */
function listGrammarConcepts() {
    ensureFutureCellConcepts();
    return concepts.list().map((row) => ({
        key: row.key,
        family: row.family,
        label_en: row.label_en,
        label_he: row.label_he,
        colour: row.colour,
        sort: row.sort
    }));
}

/**
 * @param {string} message
 * @param {object} result
 * @returns {Error}
 */
function rejected(message, result) {
    const err = new Error(message);
    err.lexiconResult = result;
    return err;
}

/**
 * @param {unknown} actor
 * @returns {'admin'|'mcp'|null}
 */
function writerKind(actor) {
    if (actor === 'mcp') return 'mcp';
    if (typeof actor !== 'string' || !actor.startsWith('admin:')) return null;
    const name = actor.slice('admin:'.length);
    if (name.length < 1 || [...name].length > 64) return null;
    if (MARKUP.test(name) || CONTROL.test(name) || /\s/.test(name)) return null;
    return 'admin';
}

/**
 * @param {object} row
 * @returns {object}
 */
function toPublic(row) {
    let accept = [];
    try {
        const parsed = JSON.parse(row.accept_json || '[]');
        accept = Array.isArray(parsed) ? parsed : [];
    } catch (_) {
        accept = [];
    }
    return {
        key: row.key,
        kind: row.kind,
        lemma_key: row.lemma_key,
        gloss_en: row.gloss_en,
        pos: row.pos,
        root: row.root,
        binyan: row.binyan,
        root_class: row.root_class,
        tense: row.tense,
        cell: row.cell,
        translit: row.translit,
        accept,
        verified: row.verified,
        verified_by: row.verified_by,
        source: row.source,
        rev: row.rev,
        updated_at: row.updated_at
    };
}

/**
 * @param {object[]} forms
 * @returns {Record<string, Record<string, object>>}
 */
function groupParadigm(forms) {
    const buckets = new Map();
    forms.forEach((row) => {
        const tense = row.tense || '';
        const cell = row.cell || '';
        if (!buckets.has(tense)) buckets.set(tense, new Map());
        const cells = buckets.get(tense);
        if (!cells.has(cell)) cells.set(cell, toPublic(row));
    });
    const orderedTenses = [...buckets.keys()].sort((a, b) => {
        const ai = TENSE_ORDER.indexOf(a);
        const bi = TENSE_ORDER.indexOf(b);
        return (ai < 0 ? 99 : ai) - (bi < 0 ? 99 : bi);
    });
    const paradigm = {};
    orderedTenses.forEach((tense) => {
        const cells = buckets.get(tense);
        const order = CELL_ORDER[tense] || [];
        const names = [...cells.keys()].sort((a, b) => {
            const ai = order.indexOf(a);
            const bi = order.indexOf(b);
            return (ai < 0 ? 99 : ai) - (bi < 0 ? 99 : bi);
        });
        paradigm[tense] = {};
        names.forEach((cell) => {
            paradigm[tense][cell] = cells.get(cell);
        });
    });
    return paradigm;
}

/**
 * @param {string} lemmaKey
 * @returns {object[]}
 */
function formsOf(lemmaKey) {
    return lexicon.listByLemma(lemmaKey).filter((row) => row.kind === 'form');
}

/**
 * @param {object} entry
 * @param {object|null} existing
 * @param {string} source
 * @param {string} now
 * @returns {object}
 */
function rowForWrite(entry, existing, source, now) {
    return {
        key: entry.key,
        kind: entry.kind,
        lemma_key: entry.lemma_key,
        gloss_en: entry.gloss_en,
        pos: entry.pos,
        root: entry.root,
        binyan: entry.binyan,
        root_class: entry.root_class,
        tense: entry.tense,
        cell: entry.cell,
        translit: entry.translit,
        accept_json: JSON.stringify(entry.accept),
        verified: 0,
        verified_by: null,
        source,
        rev: existing ? existing.rev + 1 : 1,
        updated_at: now
    };
}

/**
 * Validate the batch, then write it in one transaction.
 * A stale expectedRev writes nothing. Payload `verified` and `source` are ignored.
 * An edit stores the new fields with verified cleared. A verified form
 * therefore loses the flag; the audit `cleared` count records that.
 * @param {unknown} entries
 * @param {{ actor?: unknown, expectedRev?: unknown }} [options]
 * @returns {{ ok: true, entries: object[] }|{ ok: false, errors?: { key: string, field: string, code: string }[], error?: string, conflicts?: { key: string, rev: number }[] }}
 */
function upsert(entries, options) {
    const opts = options || {};
    const kind = writerKind(opts.actor);
    if (!kind) return { ok: false, error: 'forbidden' };
    if (!Number.isInteger(opts.expectedRev) || opts.expectedRev < 0) {
        return { ok: false, error: 'expected_rev' };
    }
    const shape = validateEntries(entries, { tableLemmas: null });
    if (!shape.ok) return { ok: false, errors: shape.errors };

    const source = kind === 'mcp' ? 'mcp' : 'admin';
    const expectedRev = opts.expectedRev;
    try {
        return db.transaction(() => {
            const tableLemmas = new Set(lexicon.listLemmaKeys());
            const checked = validateEntries(shape.entries, { tableLemmas });
            if (!checked.ok) throw rejected('lexicon_validation', { ok: false, errors: checked.errors });

            const conflicts = [];
            const existingByKey = new Map();
            checked.entries.forEach((entry) => {
                const existing = lexicon.find(entry.key) || null;
                existingByKey.set(entry.key, existing);
                const current = existing ? existing.rev : 0;
                if (current !== expectedRev) conflicts.push({ key: entry.key, rev: current });
            });
            if (conflicts.length) {
                throw rejected('revision_conflict', {
                    ok: false,
                    error: 'revision_conflict',
                    conflicts
                });
            }

            const now = new Date().toISOString();
            let cleared = 0;
            const order = { lemma: 0, phrase: 1, form: 2 };
            const sorted = checked.entries.slice().sort((a, b) => order[a.kind] - order[b.kind]);
            const written = sorted.map((entry) => {
                const existing = existingByKey.get(entry.key);
                if (existing && existing.verified === 1) cleared += 1;
                lexicon.upsert(rowForWrite(entry, existing, source, now));
                return toPublic(lexicon.find(entry.key));
            });
            if (written.length) {
                audit.append({
                    actor: opts.actor,
                    action: 'lexicon_upsert',
                    target: null,
                    detail: JSON.stringify({ count: written.length, cleared })
                });
            }
            return { ok: true, entries: written };
        });
    } catch (err) {
        if (err && err.lexiconResult) return err.lexiconResult;
        throw err;
    }
}

/**
 * Set verified on existing keys. Admin actors only. `mcp` is refused and
 * writes neither rows nor an audit record.
 * @param {unknown} keys
 * @param {unknown} value
 * @param {{ actor?: unknown }} [options]
 * @returns {{ ok: true, entries: object[] }|{ ok: false, error: string, key?: string }}
 */
function setVerified(keys, value, options) {
    const opts = options || {};
    if (opts.actor === 'mcp' || writerKind(opts.actor) !== 'admin') {
        return { ok: false, error: 'forbidden' };
    }
    const flag = value === true || value === 1 ? 1 : (value === false || value === 0 ? 0 : null);
    if (flag == null) return { ok: false, error: 'verified' };
    if (!Array.isArray(keys) || keys.length === 0 || keys.length > LIST_MAX) {
        return { ok: false, error: 'keys' };
    }

    const normalized = [];
    const seen = new Set();
    for (const raw of keys) {
        if (typeof raw !== 'string') return { ok: false, error: 'keys' };
        const key = lexicon.exactKey(raw);
        if (!key || seen.has(key)) continue;
        seen.add(key);
        normalized.push(key);
    }
    if (!normalized.length) return { ok: false, error: 'keys' };

    try {
        return db.transaction(() => {
            const rows = [];
            for (const key of normalized) {
                const row = lexicon.find(key);
                if (!row) throw rejected('not_found', { ok: false, error: 'not_found', key });
                rows.push(row);
            }
            const now = new Date().toISOString();
            const written = rows.map((row) => {
                lexicon.upsert(Object.assign({}, row, {
                    verified: flag,
                    verified_by: flag === 1 ? opts.actor : null,
                    updated_at: now
                }));
                return toPublic(lexicon.find(row.key));
            });
            audit.append({
                actor: opts.actor,
                action: 'lexicon_verify',
                target: null,
                detail: JSON.stringify({ count: written.length, value: flag })
            });
            return { ok: true, entries: written };
        });
    } catch (err) {
        if (err && err.lexiconResult) return err.lexiconResult;
        throw err;
    }
}

/**
 * @param {unknown} value
 * @param {Set<string>} allowed
 * @returns {string|null|false} null means absent, false means invalid
 */
function filterEnum(value, allowed) {
    if (value == null || value === '') return null;
    if (typeof value !== 'string' || !allowed.has(value)) return false;
    return value;
}

/**
 * @param {string} text
 * @returns {string}
 */
function likePattern(text) {
    const clipped = [...text].slice(0, QUERY_MAX).join('');
    return '%' + clipped.replace(/[\\%_]/g, (ch) => '\\' + ch) + '%';
}

/**
 * @param {{ q?: unknown, pos?: unknown, binyan?: unknown, tense?: unknown, rootClass?: unknown, verified?: unknown, limit?: unknown, cursor?: unknown }} [query]
 * @returns {{ ok: true, entries: object[], nextCursor: string|null }|{ ok: false, error: string, field?: string }}
 */
function list(query) {
    const q = query || {};
    const pos = filterEnum(q.pos, POS);
    const binyan = filterEnum(q.binyan, BINYAN);
    const tense = filterEnum(q.tense, TENSE);
    const rootClass = filterEnum(q.rootClass != null ? q.rootClass : q.root_class, ROOT_CLASS);
    if (pos === false) return { ok: false, error: 'invalid_filter', field: 'pos' };
    if (binyan === false) return { ok: false, error: 'invalid_filter', field: 'binyan' };
    if (tense === false) return { ok: false, error: 'invalid_filter', field: 'tense' };
    if (rootClass === false) return { ok: false, error: 'invalid_filter', field: 'rootClass' };

    let verified = null;
    if (q.verified != null && q.verified !== '') {
        if (q.verified === true || q.verified === 1 || q.verified === '1') verified = 1;
        else if (q.verified === false || q.verified === 0 || q.verified === '0') verified = 0;
        else return { ok: false, error: 'invalid_filter', field: 'verified' };
    }

    let limit = LIST_DEFAULT;
    if (q.limit != null && q.limit !== '') {
        const parsed = Number(q.limit);
        if (!Number.isInteger(parsed) || parsed < 1) {
            return { ok: false, error: 'invalid_filter', field: 'limit' };
        }
        limit = Math.min(parsed, LIST_MAX);
    }

    let cursor = null;
    if (q.cursor != null && q.cursor !== '') {
        if (typeof q.cursor !== 'string') return { ok: false, error: 'invalid_filter', field: 'cursor' };
        cursor = lexicon.exactKey(q.cursor);
    }

    let pattern = null;
    if (q.q != null && q.q !== '') {
        if (typeof q.q !== 'string') return { ok: false, error: 'invalid_filter', field: 'q' };
        const term = lexicon.exactKey(q.q);
        if (term) pattern = likePattern(term);
    }

    const rows = lexicon.search({
        q: pattern,
        pos,
        binyan,
        tense,
        rootClass,
        verified,
        cursor,
        limit: limit + 1
    });
    const page = rows.slice(0, limit).map(toPublic);
    const nextCursor = rows.length > limit ? page[page.length - 1].key : null;
    return { ok: true, entries: page, nextCursor };
}

/**
 * The requested row, its lemma, and that lemma's forms grouped by tense and cell.
 * @param {unknown} key
 * @returns {{ entry: object, lemma: object|null, paradigm: object }|null}
 */
function get(key) {
    if (typeof key !== 'string') return null;
    const row = lexicon.find(key);
    if (!row) return null;
    if (row.kind === 'phrase') {
        return { entry: toPublic(row), lemma: null, paradigm: {} };
    }
    const lemmaKey = row.kind === 'form' ? row.lemma_key : row.key;
    const lemma = lemmaKey ? lexicon.find(lemmaKey) : null;
    const forms = lemmaKey ? formsOf(lemmaKey) : [];
    return {
        entry: toPublic(row),
        lemma: lemma && lemma.kind === 'lemma' ? toPublic(lemma) : null,
        paradigm: groupParadigm(forms)
    };
}

/**
 * Forms of one lemma, grouped by tense and cell.
 * @param {unknown} lemmaKey
 * @returns {{ lemma: object, paradigm: object }|null}
 */
function paradigm(lemmaKey) {
    if (typeof lemmaKey !== 'string') return null;
    const lemma = lexicon.find(lemmaKey);
    if (!lemma || lemma.kind !== 'lemma') return null;
    return { lemma: toPublic(lemma), paradigm: groupParadigm(formsOf(lemma.key)) };
}

/**
 * Insert vocab rows, or refresh gloss and translit when the stored source is seed.
 * A row whose source is not seed is left unchanged, including its gloss.
 * @param {object[]} entries Normalized lemma or phrase rows.
 * @returns {{ inserted: number, updated: number, kept: number }}
 */
function applySeed(entries) {
    const now = new Date().toISOString();
    return db.transaction(() => {
        let inserted = 0;
        let updated = 0;
        let kept = 0;
        (entries || []).forEach((entry) => {
            const existing = lexicon.find(entry.key);
            if (existing && existing.source !== 'seed') {
                kept += 1;
                return;
            }
            if (!existing) {
                lexicon.upsert({
                    key: entry.key,
                    kind: entry.kind,
                    lemma_key: null,
                    gloss_en: entry.gloss_en,
                    pos: null,
                    root: null,
                    binyan: null,
                    root_class: null,
                    tense: null,
                    cell: null,
                    translit: entry.translit,
                    accept_json: '[]',
                    verified: 0,
                    verified_by: null,
                    source: 'seed',
                    rev: 1,
                    updated_at: now
                });
                inserted += 1;
                return;
            }
            const same = existing.kind === entry.kind
                && existing.gloss_en === entry.gloss_en
                && existing.translit === entry.translit;
            if (same) {
                kept += 1;
                return;
            }
            lexicon.upsert({
                key: existing.key,
                kind: entry.kind,
                lemma_key: null,
                gloss_en: entry.gloss_en,
                pos: existing.pos,
                root: existing.root,
                binyan: existing.binyan,
                root_class: existing.root_class,
                tense: existing.tense,
                cell: existing.cell,
                translit: entry.translit,
                accept_json: existing.accept_json,
                verified: 0,
                verified_by: null,
                source: 'seed',
                rev: existing.rev + 1,
                updated_at: now
            });
            updated += 1;
        });
        return { inserted, updated, kept };
    });
}

module.exports = {
    upsert,
    setVerified,
    list,
    get,
    paradigm,
    applySeed,
    listGrammarConcepts,
    toPublic
};
