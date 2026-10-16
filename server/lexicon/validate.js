/**
 * Lexicon entry checks from ADR-0009 §6.
 * exactKey comes from server/vocab-index.js. Point detection uses the
 * ADR-0001 range U+0591–U+05C7. Nothing here touches the database.
 * Provenance: INIT-006/SPEC-005
 */
'use strict';

const { exactKey } = require('../vocab-index');

const BATCH_LIMIT = 200;
const KEY_MAX = 60;
const GLOSS_MAX = 120;
const TRANSLIT_MAX = 80;
const ACCEPT_MAX_ITEMS = 8;
const ACCEPT_MAX_CHARS = 60;

const HEBREW_LETTER = /[\u05D0-\u05EA]/;
const MARKUP = /[<>]/;
const CONTROL = /[\u0000-\u001F\u007F]/;
const ROOT = /^[\u05D0-\u05EA](?:\.[\u05D0-\u05EA]){1,3}$/;

const KINDS = new Set(['lemma', 'form', 'phrase']);
const POS = new Set([
    'noun', 'verb', 'adjective', 'adverb', 'preposition',
    'pronoun', 'conjunction', 'particle', 'numeral', 'other'
]);
const BINYAN = new Set(['paal', 'nifal', 'piel', 'pual', 'hifil', 'hufal', 'hitpael']);
const ROOT_CLASS = new Set(['regular', 'lamed-he', 'peh-yod', 'peh-nun', 'ayin-vav', 'irregular']);
const TENSE = new Set(['infinitive', 'present', 'past', 'future', 'imperative']);
const CELLS = {
    present: new Set(['ms', 'fs', 'mp', 'fp']),
    past: new Set(['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p']),
    future: new Set(['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p']),
    infinitive: new Set(['inf'])
};

/**
 * Remove Hebrew points and cantillation (U+0591–U+05C7).
 * @param {string} text
 * @returns {string}
 */
function stripPoints(text) {
    return String(text).replace(/[\u0591-\u05C7]/g, '');
}

/**
 * @param {string} text
 * @returns {number}
 */
function codePoints(text) {
    return [...text].length;
}

/**
 * @param {unknown} value
 * @returns {string}
 */
function asText(value) {
    return String(value).normalize('NFC');
}

/**
 * @param {{ key: string, field: string, code: string }[]} errors
 * @param {string} key
 * @param {string} field
 * @param {string} code
 */
function push(errors, key, field, code) {
    const shown = [...String(key || '')].slice(0, KEY_MAX).join('');
    errors.push({ key: shown, field, code });
}

/**
 * Cheap bound before NFC or code-point walks. Well above every field limit.
 * @param {unknown} value
 * @returns {boolean}
 */
function oversized(value) {
    return typeof value === 'string' && value.length > 500;
}

/**
 * @param {unknown} value
 * @returns {boolean}
 */
function isBlank(value) {
    return value == null || value === '';
}

/**
 * NFC text, or null when the field is absent. Non-strings are a type error.
 * @param {unknown} value
 * @param {string} key
 * @param {string} field
 * @param {{ key: string, field: string, code: string }[]} errors
 * @returns {string|null}
 */
function optionalText(value, key, field, errors) {
    if (isBlank(value)) return null;
    if (typeof value !== 'string') {
        push(errors, key, field, 'type');
        return null;
    }
    if (oversized(value)) {
        push(errors, key, field, 'length');
        return null;
    }
    return asText(value);
}

/**
 * Length, markup, and control-character checks for one text field.
 * @param {string|null} text
 * @param {string} key
 * @param {string} field
 * @param {number} maxLen
 * @param {number} minLen
 * @param {{ key: string, field: string, code: string }[]} errors
 */
function checkPlain(text, key, field, maxLen, minLen, errors) {
    if (text == null) return;
    if (MARKUP.test(text)) push(errors, key, field, 'markup');
    if (CONTROL.test(text)) push(errors, key, field, 'control');
    const length = codePoints(text);
    if (length < minLen || length > maxLen) push(errors, key, field, 'length');
}

/**
 * @param {object} raw
 * @param {{ key: string, field: string, code: string }[]} errors
 * @returns {object|null}
 */
function normalizeEntry(raw, errors) {
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
        push(errors, '', 'entry', 'type');
        return null;
    }
    if (typeof raw.key !== 'string') {
        push(errors, '', 'key', 'type');
        return null;
    }
    if (oversized(raw.key)) {
        push(errors, '', 'key', 'length');
        return null;
    }
    const key = exactKey(raw.key);
    const entry = {
        key,
        kind: null,
        lemma_key: null,
        gloss_en: optionalText(raw.gloss_en != null ? raw.gloss_en : raw.glossEn, key, 'gloss_en', errors),
        pos: null,
        root: null,
        binyan: null,
        root_class: null,
        tense: null,
        cell: null,
        translit: optionalText(raw.translit, key, 'translit', errors),
        accept: []
    };

    checkPlain(key, key, 'key', KEY_MAX, 1, errors);
    if (key && !HEBREW_LETTER.test(key)) push(errors, key, 'key', 'hebrew_required');
    if (key && key !== exactKey(key)) push(errors, key, 'key', 'exact_key');
    if (key && key !== key.normalize('NFC')) push(errors, key, 'key', 'nfc');

    const kindRaw = raw.kind == null ? (key.includes(' ') ? 'phrase' : 'lemma') : raw.kind;
    if (typeof kindRaw !== 'string' || !KINDS.has(kindRaw)) {
        push(errors, key, 'kind', 'enum');
    } else {
        entry.kind = kindRaw;
        const spaced = key.includes(' ');
        if (kindRaw === 'phrase' && !spaced) push(errors, key, 'kind', 'kind_space');
        if (kindRaw !== 'phrase' && spaced) push(errors, key, 'kind', 'kind_space');
    }

    const lemmaRaw = raw.lemma_key != null ? raw.lemma_key : raw.lemmaKey;
    if (entry.kind === 'form') {
        if (typeof lemmaRaw !== 'string' || (!oversized(lemmaRaw) && exactKey(lemmaRaw) === '')) {
            push(errors, key, 'lemma_key', 'lemma_required');
        } else if (oversized(lemmaRaw)) {
            push(errors, key, 'lemma_key', 'length');
        } else {
            entry.lemma_key = exactKey(lemmaRaw);
            checkPlain(entry.lemma_key, key, 'lemma_key', KEY_MAX, 1, errors);
            if (!HEBREW_LETTER.test(entry.lemma_key)) {
                push(errors, key, 'lemma_key', 'hebrew_required');
            }
        }
    } else if (entry.kind && !isBlank(lemmaRaw)) {
        push(errors, key, 'lemma_key', 'lemma_forbidden');
    }

    checkPlain(entry.gloss_en, key, 'gloss_en', GLOSS_MAX, 0, errors);
    checkPlain(entry.translit, key, 'translit', TRANSLIT_MAX, 0, errors);

    entry.pos = enumOrNull(raw.pos, key, 'pos', POS, errors);
    entry.binyan = enumOrNull(raw.binyan, key, 'binyan', BINYAN, errors);
    const rootClassRaw = raw.root_class != null ? raw.root_class : raw.rootClass;
    entry.root_class = enumOrNull(rootClassRaw, key, 'root_class', ROOT_CLASS, errors);
    entry.tense = enumOrNull(raw.tense, key, 'tense', TENSE, errors);

    if (!isBlank(raw.root)) {
        if (typeof raw.root !== 'string') {
            push(errors, key, 'root', 'type');
        } else if (oversized(raw.root)) {
            push(errors, key, 'root', 'length');
        } else {
            entry.root = asText(raw.root).trim();
            if (MARKUP.test(entry.root)) push(errors, key, 'root', 'markup');
            if (CONTROL.test(entry.root)) push(errors, key, 'root', 'control');
            if (!ROOT.test(entry.root)) push(errors, key, 'root', 'root_shape');
        }
    }

    if (!isBlank(raw.cell)) {
        if (typeof raw.cell !== 'string') {
            push(errors, key, 'cell', 'type');
        } else {
            entry.cell = asText(raw.cell).trim();
            const allowed = entry.tense ? CELLS[entry.tense] : null;
            if (!allowed || !allowed.has(entry.cell)) {
                push(errors, key, 'cell', 'cell_tense');
            }
        }
    }

    entry.accept = normalizeAccept(raw, key, errors);
    return entry;
}

/**
 * @param {unknown} value
 * @param {string} key
 * @param {string} field
 * @param {Set<string>} allowed
 * @param {{ key: string, field: string, code: string }[]} errors
 * @returns {string|null}
 */
function enumOrNull(value, key, field, allowed, errors) {
    if (isBlank(value)) return null;
    if (typeof value !== 'string' || !allowed.has(value)) {
        push(errors, key, field, 'enum');
        return null;
    }
    return value;
}

/**
 * @param {object} raw
 * @param {string} key
 * @param {{ key: string, field: string, code: string }[]} errors
 * @returns {string[]}
 */
function normalizeAccept(raw, key, errors) {
    let list = raw.accept;
    if (list == null && raw.accept_json != null) list = raw.accept_json;
    if (list == null) return [];
    if (typeof list === 'string') {
        try {
            list = JSON.parse(list);
        } catch (_) {
            push(errors, key, 'accept', 'type');
            return [];
        }
    }
    if (!Array.isArray(list)) {
        push(errors, key, 'accept', 'type');
        return [];
    }
    if (list.length > ACCEPT_MAX_ITEMS) push(errors, key, 'accept', 'accept_count');
    const out = [];
    list.slice(0, ACCEPT_MAX_ITEMS).forEach((item) => {
        if (typeof item !== 'string') {
            push(errors, key, 'accept', 'type');
            return;
        }
        if (oversized(item)) {
            push(errors, key, 'accept', 'length');
            return;
        }
        const text = exactKey(item);
        if (MARKUP.test(text)) push(errors, key, 'accept', 'markup');
        if (CONTROL.test(text)) push(errors, key, 'accept', 'control');
        if (stripPoints(text) !== text) push(errors, key, 'accept', 'accept_pointed');
        const length = codePoints(text);
        if (length < 1 || length > ACCEPT_MAX_CHARS) push(errors, key, 'accept', 'length');
        out.push(text);
    });
    return out;
}

/**
 * Validate a whole batch. `tableLemmas` is the set of lemma keys already
 * stored. Pass null to check shape only and defer the table lookup.
 * A form whose lemma is in this batch counts as present.
 * @param {unknown} entries
 * @param {{ tableLemmas?: Set<string>|null }} [options]
 * @returns {{ ok: boolean, errors: { key: string, field: string, code: string }[], entries?: object[] }}
 */
function validateEntries(entries, options) {
    const errors = [];
    if (!Array.isArray(entries)) {
        return { ok: false, errors: [{ key: '', field: 'entries', code: 'type' }] };
    }
    if (entries.length > BATCH_LIMIT) {
        return { ok: false, errors: [{ key: '', field: 'entries', code: 'batch_limit' }] };
    }

    const normalized = [];
    entries.forEach((raw) => {
        const entry = normalizeEntry(raw, errors);
        if (entry) normalized.push(entry);
    });

    const seen = new Set();
    normalized.forEach((entry) => {
        if (!entry.key) return;
        if (seen.has(entry.key)) push(errors, entry.key, 'key', 'duplicate');
        seen.add(entry.key);
    });

    const tableLemmas = options && options.tableLemmas instanceof Set
        ? new Set(options.tableLemmas)
        : null;
    const batchLemmas = new Set(
        normalized.filter((entry) => entry.kind === 'lemma' && entry.key).map((entry) => entry.key)
    );
    if (tableLemmas) {
        normalized.forEach((entry) => {
            if (!entry.key) return;
            if (entry.kind === 'lemma') tableLemmas.add(entry.key);
            else tableLemmas.delete(entry.key);
        });
    }

    normalized.forEach((entry) => {
        if (entry.kind !== 'form' || !entry.lemma_key) return;
        if (batchLemmas.has(entry.lemma_key)) return;
        if (!tableLemmas) return;
        if (!tableLemmas.has(entry.lemma_key)) {
            push(errors, entry.key, 'lemma_key', 'lemma_missing');
        }
    });

    if (errors.length) return { ok: false, errors };
    return { ok: true, errors: [], entries: normalized };
}

module.exports = {
    validateEntries,
    stripPoints,
    BATCH_LIMIT,
    POS,
    BINYAN,
    ROOT_CLASS,
    TENSE,
    CELLS
};
