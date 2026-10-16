/**
 * RFC 6901 pointer to id path. The validator reports a finding at a pointer
 * with array indexes (`/units/0/lessons/1/blocks/2/words/3/meaning`). The part
 * operations (`read_part`, `set_field`, `replace_part`) address a node by id
 * (`units/u1/lessons/l1/blocks/b1/words/w4`). This module turns the first form
 * into the second plus the rest of the pointer, so an AI or an admin can act
 * on a finding with one call. It mirrors the ids `locate()` in ops.js accepts:
 * unit, lesson, block, and the four id-addressable block lists. Pure; no I/O.
 * Provenance: INIT-008/SPEC-008
 */
'use strict';

/** The block lists whose items `read_part` can address by id (ops.js BUCKETS). */
const BUCKETS = Object.freeze(['words', 'paragraphs', 'lines', 'questions']);

function isObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

/** RFC 6901: `~1` is `/`, `~0` is `~`. */
function unescapeSegment(segment) {
    return segment.replace(/~1/g, '/').replace(/~0/g, '~');
}

function escapeSegment(segment) {
    return String(segment).replace(/~/g, '~0').replace(/\//g, '~1');
}

/** Split a pointer into raw (still escaped) segments. '' is the document. */
function segmentsOf(pointer) {
    if (typeof pointer !== 'string' || pointer === '') return [];
    return pointer.split('/').slice(1);
}

/**
 * True when `items[index]` is the first item in the list with its id, so the
 * id names exactly this item (`locate()` takes the first match).
 */
function addressable(items, index) {
    const item = Array.isArray(items) ? items[index] : undefined;
    if (!isObject(item) || typeof item.id !== 'string' || item.id === '') return false;
    return items.findIndex((other) => isObject(other) && other.id === item.id) === index;
}

/**
 * @param {unknown} plan The plan document the pointer was reported against.
 * @param {string} pointer RFC 6901 pointer into `plan`.
 * @returns {{ path: string, field: string }} `path` is the deepest node `read_part` accepts
 *   (no leading slash; '' is the plan); `field` is the rest of the pointer, still RFC 6901
 *   escaped and without a leading slash, '' when the pointer is the node itself.
 */
function toIdPath(plan, pointer) {
    const segs = segmentsOf(pointer);
    const parts = [];
    let used = 0;
    if (isObject(plan) && segs.length >= 2 && segs[0] === 'units') {
        const units = plan.units;
        const u = Number(unescapeSegment(segs[1]));
        if (Number.isInteger(u) && addressable(units, u)) {
            parts.push('units', units[u].id);
            used = 2;
            const unit = units[u];
            const l = segs.length >= 4 && segs[2] === 'lessons' ? Number(unescapeSegment(segs[3])) : NaN;
            if (Number.isInteger(l) && addressable(unit.lessons, l)) {
                parts.push('lessons', unit.lessons[l].id);
                used = 4;
                const lesson = unit.lessons[l];
                const b = segs.length >= 6 && segs[4] === 'blocks' ? Number(unescapeSegment(segs[5])) : NaN;
                if (Number.isInteger(b) && addressable(lesson.blocks, b)) {
                    parts.push('blocks', lesson.blocks[b].id);
                    used = 6;
                    const block = lesson.blocks[b];
                    const bucket = segs.length >= 8 ? unescapeSegment(segs[6]) : '';
                    const m = bucket ? Number(unescapeSegment(segs[7])) : NaN;
                    if (BUCKETS.includes(bucket) && Number.isInteger(m) && addressable(block[bucket], m)) {
                        parts.push(bucket, block[bucket][m].id);
                        used = 8;
                    }
                }
            }
        }
    }
    return { path: parts.join('/'), field: segs.slice(used).join('/') };
}

/**
 * The part operation that sets the field a finding names. `set_field` takes
 * one top-level property of a node; anything deeper (`cloze/items/0/answer`,
 * `newWords/2`) is a `replace_part` of the node with the sub-field changed.
 * @param {{ path: string, field: string }} target From toIdPath.
 * @returns {'set_field'|'replace_part'}
 */
function actionFor(target) {
    return target.field && !target.field.includes('/') ? 'set_field' : 'replace_part';
}

module.exports = { BUCKETS, actionFor, escapeSegment, toIdPath };
