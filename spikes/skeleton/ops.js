'use strict';

/**
 * Part-level plan edits. One function for the command and the form.
 * Provenance: INIT-003/SPEC-009
 *
 * apply(plan, op, expectedRev) → { plan } or { conflict }
 * A revision mismatch returns { conflict: { error: 'revision_conflict', revision } }
 * and does not change the plan object the caller passed in.
 */

const fs = require('fs');
const path = require('path');

const PLAN_PATH = path.resolve(
    __dirname,
    '..',
    '..',
    'docs',
    'decisions',
    'ADR-0003-assets',
    'samples',
    'beginner-unit-1.plan.json'
);

const FIELD_NAME = /^[A-Za-z][A-Za-z0-9]{0,40}$/;
const BUCKETS = ['words', 'paragraphs', 'lines', 'questions'];

function loadPlan() {
    return JSON.parse(fs.readFileSync(PLAN_PATH, 'utf8'));
}

function cloneJson(value) {
    return JSON.parse(JSON.stringify(value));
}

function locate(plan, pathStr) {
    const segs = String(pathStr || '').split('/').filter(Boolean);
    if (segs.length < 2 || segs[0] !== 'units' || !Array.isArray(plan.units)) return null;

    const units = plan.units;
    const unitIndex = units.findIndex((unit) => unit && unit.id === segs[1]);
    if (unitIndex < 0) return null;
    if (segs.length === 2) {
        return { list: units, index: unitIndex, node: units[unitIndex], kind: 'unit' };
    }

    if (segs[2] !== 'lessons' || segs.length < 4) return null;
    const lessons = units[unitIndex].lessons;
    if (!Array.isArray(lessons)) return null;
    const lessonIndex = lessons.findIndex((lesson) => lesson && lesson.id === segs[3]);
    if (lessonIndex < 0) return null;
    if (segs.length === 4) {
        return { list: lessons, index: lessonIndex, node: lessons[lessonIndex], kind: 'lesson' };
    }

    if (segs[4] !== 'blocks' || segs.length < 6) return null;
    const blocks = lessons[lessonIndex].blocks;
    if (!Array.isArray(blocks)) return null;
    const blockIndex = blocks.findIndex((block) => block && block.id === segs[5]);
    if (blockIndex < 0) return null;
    if (segs.length === 6) {
        return {
            list: blocks,
            index: blockIndex,
            node: blocks[blockIndex],
            kind: 'block',
            lesson: lessons[lessonIndex]
        };
    }

    const bucket = segs[6];
    if (BUCKETS.indexOf(bucket) === -1 || segs.length !== 8) return null;
    const items = blocks[blockIndex][bucket];
    if (!Array.isArray(items)) return null;
    const itemIndex = items.findIndex((item) => item && item.id === segs[7]);
    if (itemIndex < 0) return null;
    return { list: items, index: itemIndex, node: items[itemIndex], kind: bucket };
}

function conflict(error, extra) {
    return { conflict: Object.assign({ error: error }, extra || {}) };
}

function finish(next) {
    next.revision = next.revision + 1;
    next.status = 'draft';
    return { plan: next };
}

function apply(plan, op, expectedRev) {
    if (!plan || typeof plan !== 'object' || Array.isArray(plan)) {
        return conflict('invalid_plan');
    }
    if (!op || typeof op !== 'object') return conflict('invalid_op');

    if (op.op === 'read_part') {
        const found = locate(plan, op.path);
        if (!found) return conflict('not_found', { path: op.path || '' });
        return { plan: plan, part: cloneJson(found.node) };
    }

    if (expectedRev !== plan.revision) {
        return conflict('revision_conflict', { revision: plan.revision });
    }

    const next = cloneJson(plan);
    const found = locate(next, op.path);

    if (op.op === 'set_field') {
        if (!found) return conflict('not_found', { path: op.path || '' });
        if (typeof op.field !== 'string' || !FIELD_NAME.test(op.field)) {
            return conflict('invalid_op');
        }
        if (op.value === undefined) return conflict('invalid_op');
        const value = cloneJson(op.value);
        if ((op.field === 'alt' || op.field === 'mediaId')
            && found.node.image
            && typeof found.node.image === 'object'
            && !Object.prototype.hasOwnProperty.call(found.node, op.field)) {
            found.node.image[op.field] = value;
        } else {
            found.node[op.field] = value;
        }
        return finish(next);
    }

    if (op.op === 'replace_part') {
        if (!found) return conflict('not_found', { path: op.path || '' });
        if (op.value === undefined || op.value === null || typeof op.value !== 'object') {
            return conflict('invalid_op');
        }
        found.list[found.index] = cloneJson(op.value);
        return finish(next);
    }

    if (op.op === 'insert_block') {
        if (!found || found.kind !== 'lesson' || !Array.isArray(found.node.blocks)) {
            return conflict('not_found', { path: op.path || '' });
        }
        if (!op.block || typeof op.block !== 'object' || Array.isArray(op.block)) {
            return conflict('invalid_op');
        }
        const at = op.index == null ? found.node.blocks.length : op.index;
        if (!Number.isInteger(at) || at < 0 || at > found.node.blocks.length) {
            return conflict('invalid_op');
        }
        found.node.blocks.splice(at, 0, cloneJson(op.block));
        return finish(next);
    }

    if (op.op === 'move_block') {
        if (!found || found.kind !== 'block') return conflict('not_found', { path: op.path || '' });
        if (!Number.isInteger(op.index)) return conflict('invalid_op');
        const item = found.list.splice(found.index, 1)[0];
        const at = Math.max(0, Math.min(op.index, found.list.length));
        found.list.splice(at, 0, item);
        return finish(next);
    }

    if (op.op === 'remove_block') {
        if (!found || found.kind !== 'block') return conflict('not_found', { path: op.path || '' });
        found.list.splice(found.index, 1);
        return finish(next);
    }

    return conflict('invalid_op');
}

/**
 * Shared entry for cli.js and the form POST.
 * `body` is the JSON object both callers send.
 */
function applyBody(plan, body) {
    const op = {
        op: body && body.op,
        path: body && body.path,
        field: body && body.field,
        value: body && body.value,
        index: body && body.index,
        block: body && body.block
    };
    return apply(plan, op, body && body.expectedRevision);
}

module.exports = {
    PLAN_PATH,
    apply,
    applyBody,
    loadPlan
};
