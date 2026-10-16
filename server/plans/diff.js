/**
 * Field-level diff of two plan documents, addressed by id path (ADR-0011 §3).
 * The plan, its units, lessons, and blocks, and the items in a block's
 * `words`, `paragraphs`, `lines`, and `questions` lists are nodes. A node is
 * addressed the way ops.js locate() addresses it (`units/u/lessons/l/blocks/b`),
 * so an AI can act on a diff entry with one read_part or set_field call.
 * Every other field is compared whole: an array that is not an id-addressable
 * list (`newWords`, `rules`, `cells`, `spans`, `highlights`) is one field.
 * Pure: no I/O, and neither input is changed.
 *
 * Entry kinds
 *   changed  one field of a node differs: { path, field, kind, before?, after? }.
 *            `before` is absent when the field is new, `after` when it was removed.
 *   added    a node only in `b`: { path, field: '', kind, after: node }.
 *   removed  a node only in `a`: { path, field: '', kind, before: node }.
 *   moved    the relative order of the nodes both documents hold in one list
 *            differs: { path: parent, field: listName, kind, before: ids, after: ids }.
 * The plan `revision` counter always differs between two revisions and is not
 * reported. Every other plan field, `status` included, is.
 * Provenance: INIT-008/SPEC-005
 */
'use strict';

/** The block lists whose items have ids (ops.js BUCKETS). */
const BUCKETS = Object.freeze(['words', 'paragraphs', 'lines', 'questions']);

const MAX_ENTRIES = 500;
const MAX_BYTES = 64 * 1024;
const MAX_VALUE_CHARS = 4000;
const CUT = '\u2026';

/** Child lists by node depth: plan, unit, lesson, block, bucket item. */
const CHILD_LISTS = Object.freeze([['units'], ['lessons'], ['blocks'], BUCKETS, []]);

/**
 * @param {unknown} value
 * @returns {boolean}
 */
function isObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

/**
 * JSON text with object keys in sorted order, so key order never counts.
 * @param {unknown} value
 * @returns {string}
 */
function stable(value) {
    if (value === undefined) return 'undefined';
    if (value === null || typeof value !== 'object') return JSON.stringify(value);
    if (Array.isArray(value)) return '[' + value.map(stable).join(',') + ']';
    const keys = Object.keys(value).sort();
    return '{' + keys.map((key) => JSON.stringify(key) + ':' + stable(value[key])).join(',') + '}';
}

/**
 * A list is addressable when every item is an object with a distinct, non-empty
 * id that holds no slash. Otherwise the list is one field of its parent.
 * @param {unknown} list
 * @returns {boolean}
 */
function addressable(list) {
    if (!Array.isArray(list)) return false;
    const seen = new Set();
    for (const item of list) {
        if (!isObject(item) || typeof item.id !== 'string' || item.id === '' || item.id.indexOf('/') !== -1) {
            return false;
        }
        if (seen.has(item.id)) return false;
        seen.add(item.id);
    }
    return true;
}

/**
 * @param {string} parent
 * @param {string} list
 * @param {string} id
 * @returns {string}
 */
function join(parent, list, id) {
    return parent ? parent + '/' + list + '/' + id : list + '/' + id;
}

/**
 * Flatten a document into nodes keyed by id path, parents before children.
 * @param {object} doc
 * @returns {Map<string, { parent: string|null, raw: object, fields: object, lists: object }>}
 */
function collect(doc) {
    const nodes = new Map();

    function visit(path, parent, node, depth) {
        const childLists = CHILD_LISTS[depth] || [];
        const fields = {};
        const lists = {};
        const children = [];
        for (const key of Object.keys(node)) {
            if (childLists.indexOf(key) !== -1 && addressable(node[key])) {
                lists[key] = node[key].map((item) => item.id);
                children.push({ key: key, items: node[key] });
            } else {
                fields[key] = node[key];
            }
        }
        if (depth === 0) delete fields.revision;
        nodes.set(path, { parent: parent, raw: node, fields: fields, lists: lists });
        for (const child of children) {
            for (const item of child.items) {
                visit(join(path, child.key, item.id), path, item, depth + 1);
            }
        }
    }

    visit('', null, doc, 0);
    return nodes;
}

/**
 * @param {string[]} first
 * @param {string[]} second
 * @returns {boolean}
 */
function sameOrder(first, second) {
    if (first.length !== second.length) return false;
    for (let i = 0; i < first.length; i += 1) {
        if (first[i] !== second[i]) return false;
    }
    return true;
}

/**
 * Every entry, uncapped.
 * @param {object} a
 * @param {object} b
 * @returns {object[]}
 */
function allEntries(a, b) {
    const left = collect(a);
    const right = collect(b);
    const entries = [];

    for (const [path, nodeB] of right) {
        const nodeA = left.get(path);
        if (!nodeA) {
            if (nodeB.parent !== null && left.has(nodeB.parent)) {
                entries.push({ path: path, field: '', kind: 'added', after: nodeB.raw });
            }
            continue;
        }
        const names = Object.keys(nodeB.fields);
        for (const name of Object.keys(nodeA.fields)) {
            if (names.indexOf(name) === -1) names.push(name);
        }
        for (const name of names) {
            const inA = Object.prototype.hasOwnProperty.call(nodeA.fields, name);
            const inB = Object.prototype.hasOwnProperty.call(nodeB.fields, name);
            if (inA && inB && stable(nodeA.fields[name]) === stable(nodeB.fields[name])) continue;
            const entry = { path: path, field: name, kind: 'changed' };
            if (inA) entry.before = nodeA.fields[name];
            if (inB) entry.after = nodeB.fields[name];
            entries.push(entry);
        }
        const listNames = Object.keys(nodeB.lists);
        for (const name of Object.keys(nodeA.lists)) {
            if (listNames.indexOf(name) === -1) listNames.push(name);
        }
        for (const name of listNames) {
            const idsA = nodeA.lists[name] || [];
            const idsB = nodeB.lists[name] || [];
            const inB = new Set(idsB);
            const inA = new Set(idsA);
            const commonA = idsA.filter((id) => inB.has(id));
            const commonB = idsB.filter((id) => inA.has(id));
            if (!sameOrder(commonA, commonB)) {
                entries.push({ path: path, field: name, kind: 'moved', before: commonA, after: commonB });
            }
        }
    }

    for (const [path, nodeA] of left) {
        if (right.has(path)) continue;
        if (nodeA.parent !== null && right.has(nodeA.parent)) {
            entries.push({ path: path, field: '', kind: 'removed', before: nodeA.raw });
        }
    }
    return entries;
}

/**
 * @param {string} path
 * @param {string} scope
 * @returns {boolean}
 */
function inScope(path, scope) {
    return scope === '' || path === scope || path.indexOf(scope + '/') === 0;
}

/**
 * Cut a value to the display limit. Strings are cut by characters; any other
 * JSON value over the limit becomes its JSON text, cut.
 * @param {unknown} value
 * @returns {{ value: unknown, cut: boolean }}
 */
function shape(value) {
    if (typeof value === 'string') {
        return value.length > MAX_VALUE_CHARS
            ? { value: value.slice(0, MAX_VALUE_CHARS) + CUT, cut: true }
            : { value: value, cut: false };
    }
    const text = JSON.stringify(value);
    if (text !== undefined && text.length > MAX_VALUE_CHARS) {
        return { value: text.slice(0, MAX_VALUE_CHARS) + CUT, cut: true };
    }
    return { value: JSON.parse(text === undefined ? 'null' : text), cut: false };
}

/**
 * Diff two plan documents.
 * @param {object} a The older document.
 * @param {object} b The newer document.
 * @param {{ path?: string, full?: boolean }} [options] `path` keeps that node and
 *   everything below it. `full` turns off the entry, size, and value caps; the
 *   provenance code uses it, and the entries then reference the inputs.
 * @returns {{ entries: object[], truncated: boolean }}
 */
function diffDocuments(a, b, options) {
    if (!isObject(a) || !isObject(b)) throw new TypeError('diffDocuments needs two plan documents');
    const opts = options || {};
    const scope = typeof opts.path === 'string'
        ? opts.path.split('/').filter(Boolean).join('/')
        : '';
    const wanted = allEntries(a, b).filter((entry) => inScope(entry.path, scope));
    if (opts.full === true) return { entries: wanted, truncated: false };

    const entries = [];
    let bytes = 0;
    let truncated = false;
    for (const entry of wanted) {
        if (entries.length >= MAX_ENTRIES) {
            truncated = true;
            break;
        }
        const out = { path: entry.path, field: entry.field, kind: entry.kind };
        let cut = false;
        for (const side of ['before', 'after']) {
            if (!Object.prototype.hasOwnProperty.call(entry, side)) continue;
            const shaped = shape(entry[side]);
            out[side] = shaped.value;
            if (shaped.cut) cut = true;
        }
        if (cut) out.valueTruncated = true;
        const size = JSON.stringify(out).length;
        if (bytes + size > MAX_BYTES && entries.length > 0) {
            truncated = true;
            break;
        }
        bytes += size;
        entries.push(out);
    }
    return { entries: entries, truncated: truncated };
}

module.exports = {
    BUCKETS,
    MAX_ENTRIES,
    MAX_BYTES,
    MAX_VALUE_CHARS,
    diffDocuments,
    stable
};
