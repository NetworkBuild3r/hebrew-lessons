/**
 * The six ADR-0003 part operations. One apply() edits the document.
 * HTTP and MCP adapters call the six exports; they do not edit JSON.
 * Every mutation loads through the store passed in, checks expectedRevision
 * against the stored working-copy revision, validates, and saves compact JSON.
 * A stale revision returns { error: 'revision_conflict', revision } and does
 * not write. The plan object passed to apply() is left unchanged on conflict.
 * Provenance: INIT-004/SPEC-003; schemaVersion raise: INIT-006/SPEC-011
 */
'use strict';

const { validatePlan, usesV110, V100, V110 } = require('./validate');
const lexicon = require('../db/repos/lexicon');

const FIELD_NAME = /^[A-Za-z][A-Za-z0-9]{0,40}$/;
const BUCKETS = ['words', 'paragraphs', 'lines', 'questions'];

/**
 * Structural and security blocks. A mutation refuses the write when any of
 * these is a block. Pedagogical blocks (N1, N5, N12, meaning, romanized-answer)
 * stay on the draft so an author can save, and publishPlan refuses those.
 * SPEC-002: do not refuse only `schema`.
 */
const WRITE_BLOCK_RULES = new Set([
    'schema',
    'html',
    'audio-path',
    'image-placement',
    'N8',
    'N9',
    'N10',
    'N11'
]);

/**
 * @param {unknown} value
 * @returns {unknown}
 */
function cloneJson(value) {
    const text = JSON.stringify(value);
    if (typeof text !== 'string') {
        throw new TypeError('value is not JSON');
    }
    return JSON.parse(text);
}

/**
 * @param {string} error
 * @param {object} [extra]
 * @returns {{ conflict: { error: string } }}
 */
function conflict(error, extra) {
    return { conflict: Object.assign({ error: error }, extra || {}) };
}

/**
 * Address a part by id path (ADR-0003 §4). An empty path is the plan document,
 * where audience, assigneeId, and title live. Array indexes are not paths.
 * @param {object} plan
 * @param {unknown} pathStr
 * @returns {object|null}
 */
function locate(plan, pathStr) {
    if (pathStr == null || pathStr === '' || pathStr === '/') {
        return { node: plan, kind: 'plan' };
    }
    if (typeof pathStr !== 'string') return null;

    const segs = pathStr.split('/').filter(Boolean);
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

/**
 * A part operation that leaves a 1.0.0 plan holding a 1.1.0 construct raises
 * schemaVersion to 1.1.0 in the same revision (ADR-0009 §7). That is the only
 * field an operation changes that it was not given. An operation that is
 * given schemaVersion keeps that value, and a 1.1.0 plan never returns to 1.0.0.
 * @param {object} before The stored plan.
 * @param {object} next
 * @param {boolean} versionGiven True when the operation itself sets schemaVersion.
 * @returns {{ plan: object }|{ conflict: { error: string } }}
 */
function finish(before, next, versionGiven) {
    if (before.schemaVersion === V110 && next.schemaVersion === V100) return conflict('invalid_op');
    if (!versionGiven && next.schemaVersion === V100 && usesV110(next)) next.schemaVersion = V110;
    next.revision = next.revision + 1;
    next.status = 'draft';
    return { plan: next };
}

/**
 * @param {object} node
 * @param {string} field
 * @param {unknown} value
 * @returns {boolean}
 */
function assignField(node, field, value) {
    if (typeof field !== 'string' || !FIELD_NAME.test(field)) return false;
    Object.defineProperty(node, field, {
        value: value,
        writable: true,
        enumerable: true,
        configurable: true
    });
    return true;
}

/**
 * One implementation of the six operations. Does not touch the store.
 * On revision_conflict the plan argument is not mutated.
 * A read does not check the revision and does not increment it (ADR-0003 §4).
 * @param {object} plan
 * @param {object} op
 * @param {number} expectedRev
 * @returns {{ plan: object, part?: object }|{ conflict: { error: string, revision?: number, path?: string } }}
 */
function apply(plan, op, expectedRev) {
    if (!plan || typeof plan !== 'object' || Array.isArray(plan)) {
        return conflict('invalid_plan');
    }
    if (!op || typeof op !== 'object' || Array.isArray(op)) return conflict('invalid_op');

    if (op.op === 'read_part') {
        const found = locate(plan, op.path);
        if (!found) return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        return { plan: plan, part: cloneJson(found.node) };
    }

    if (expectedRev !== plan.revision) {
        return conflict('revision_conflict', { revision: plan.revision });
    }

    let next;
    try {
        next = cloneJson(plan);
    } catch (err) {
        return conflict('invalid_plan');
    }
    const found = locate(next, op.path);

    if (op.op === 'set_field') {
        if (!found || !found.node || typeof found.node !== 'object') {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        if (typeof op.field !== 'string' || !FIELD_NAME.test(op.field)) {
            return conflict('invalid_op');
        }
        if (found.kind === 'plan' && (op.field === 'id' || op.field === 'revision' || op.field === 'status')) {
            return conflict('invalid_op');
        }
        if (op.value === undefined) return conflict('invalid_op');
        let value;
        try {
            value = cloneJson(op.value);
        } catch (err) {
            return conflict('invalid_op');
        }
        if ((op.field === 'alt' || op.field === 'mediaId')
            && found.node.image
            && typeof found.node.image === 'object'
            && !Array.isArray(found.node.image)
            && !Object.hasOwn(found.node, op.field)) {
            found.node.image[op.field] = value;
        } else if (!assignField(found.node, op.field, value)) {
            return conflict('invalid_op');
        }
        return finish(plan, next, found.kind === 'plan' && op.field === 'schemaVersion');
    }

    if (op.op === 'replace_part') {
        if (!found) return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        if (op.value === undefined || op.value === null || typeof op.value !== 'object' || Array.isArray(op.value)) {
            return conflict('invalid_op');
        }
        let value;
        try {
            value = cloneJson(op.value);
        } catch (err) {
            return conflict('invalid_op');
        }
        if (found.kind === 'plan') {
            value.revision = next.revision;
            return finish(plan, value, true);
        }
        found.list[found.index] = value;
        return finish(plan, next, false);
    }

    if (op.op === 'insert_block') {
        if (!found || found.kind !== 'lesson' || !Array.isArray(found.node.blocks)) {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        if (!op.block || typeof op.block !== 'object' || Array.isArray(op.block)) {
            return conflict('invalid_op');
        }
        const at = op.index == null ? found.node.blocks.length : op.index;
        if (!Number.isInteger(at) || at < 0 || at > found.node.blocks.length) {
            return conflict('invalid_op');
        }
        let block;
        try {
            block = cloneJson(op.block);
        } catch (err) {
            return conflict('invalid_op');
        }
        found.node.blocks.splice(at, 0, block);
        return finish(plan, next, false);
    }

    if (op.op === 'move_block') {
        if (!found || found.kind !== 'block') {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        if (!Number.isInteger(op.index)) return conflict('invalid_op');
        const item = found.list.splice(found.index, 1)[0];
        const at = Math.max(0, Math.min(op.index, found.list.length));
        found.list.splice(at, 0, item);
        return finish(plan, next, false);
    }

    if (op.op === 'remove_block') {
        if (!found || found.kind !== 'block') {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        found.list.splice(found.index, 1);
        return finish(plan, next, false);
    }

    return conflict('invalid_op');
}

/**
 * @param {object} store
 */
function requireStore(store) {
    if (!store || typeof store.loadWorkingCopy !== 'function' || typeof store.saveWorkingCopy !== 'function') {
        throw new TypeError('store must provide loadWorkingCopy and saveWorkingCopy');
    }
}

/**
 * Load the working copy and align revision, status, audience, and assigneeId
 * to the stored columns. The returned plan is a new object.
 * @param {object} store
 * @param {string} id
 * @returns {{ row: object, plan: object }|{ error: string, revision?: number }}
 */
function openWorkingCopy(store, id) {
    requireStore(store);
    if (typeof id !== 'string' || id.length === 0) return { error: 'not_found' };
    const row = store.loadWorkingCopy(id);
    if (!row) return { error: 'not_found' };
    let parsed;
    try {
        parsed = JSON.parse(row.document);
    } catch (err) {
        return { error: 'invalid_plan', revision: row.revision };
    }
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { error: 'invalid_plan', revision: row.revision };
    }
    const plan = parsed;
    plan.revision = row.revision;
    plan.status = row.status;
    plan.audience = row.audience;
    plan.assigneeId = row.assigneeId;
    return { row: row, plan: plan };
}

/**
 * @param {{ error: string, revision?: number, path?: string }} conflictBody
 * @param {number} storedRevision
 * @returns {object}
 */
function fromConflict(conflictBody, storedRevision) {
    if (conflictBody.error === 'revision_conflict') {
        return { error: 'revision_conflict', revision: storedRevision };
    }
    const out = { error: conflictBody.error };
    if (storedRevision != null) out.revision = storedRevision;
    if (conflictBody.path != null) out.path = conflictBody.path;
    return out;
}

/**
 * @param {string} error
 * @param {number|undefined} revision
 * @returns {object}
 */
function failure(error, revision) {
    const out = { error: error };
    if (revision != null) out.revision = revision;
    return out;
}

/**
 * @param {object} validation
 * @returns {boolean}
 */
function refusesWrite(validation) {
    return validation.blocks.some((item) => WRITE_BLOCK_RULES.has(item.rule));
}

/**
 * @param {object} validation
 * @param {number} revision
 * @returns {object}
 */
function validationError(validation, revision) {
    return {
        error: 'validation',
        revision: revision,
        blocks: validation.blocks,
        warnings: validation.warnings,
        informs: validation.informs
    };
}

/**
 * @param {object} store
 * @param {object} row
 * @param {object} plan
 * @returns {object}
 */
function persist(store, row, plan) {
    const document = JSON.stringify(plan);
    const saved = store.saveWorkingCopy({
        id: row.id,
        document: document,
        revision: plan.revision,
        status: plan.status,
        audience: plan.audience,
        assigneeId: plan.assigneeId == null ? null : plan.assigneeId
    });
    return {
        plan: JSON.parse(saved.document),
        document: saved.document,
        revision: saved.revision,
        status: saved.status
    };
}

/**
 * @param {object} store
 * @param {string} id
 * @param {object} op
 * @param {number} expectedRevision
 * @returns {object}
 */
function mutate(store, id, op, expectedRevision) {
    const opened = openWorkingCopy(store, id);
    if (opened.error) return failure(opened.error, opened.revision);
    const result = apply(opened.plan, op, expectedRevision);
    if (result.conflict) return fromConflict(result.conflict, opened.row.revision);
    if (result.plan.id !== opened.row.id) {
        return { error: 'invalid_op', revision: opened.row.revision };
    }
    const validation = validatePlan(result.plan, { lexicon });
    if (refusesWrite(validation)) return validationError(validation, opened.row.revision);
    const saved = persist(store, opened.row, result.plan);
    saved.validation = validation;
    return saved;
}

/**
 * @param {object|string|undefined} input
 * @returns {string}
 */
function pathOf(input) {
    if (typeof input === 'string') return input;
    if (input && typeof input.path === 'string') return input.path;
    if (input && (input.path === '' || input.path === '/')) return input.path;
    return '';
}

/**
 * Read one part. Does not check or increment the revision.
 * @param {object} store Plan store from server/plans/store.js.
 * @param {string} id Plan id.
 * @param {string|{ path?: string }} input Id path, or a body with `path`. Empty path is the plan.
 * @returns {{ part: object, revision: number }|{ error: string, revision?: number, path?: string }}
 */
function readPart(store, id, input) {
    const opened = openWorkingCopy(store, id);
    if (opened.error) return failure(opened.error, opened.revision);
    const result = apply(opened.plan, { op: 'read_part', path: pathOf(input) }, opened.row.revision);
    if (result.conflict) return fromConflict(result.conflict, opened.row.revision);
    return { part: result.part, revision: opened.row.revision };
}

/**
 * @param {object|undefined} input
 * @returns {number|undefined}
 */
function expectedOf(input) {
    if (!input || typeof input !== 'object') return undefined;
    return input.expectedRevision;
}

/**
 * Replace the part at an id path.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, value: object, expectedRevision: number }} input
 * @returns {object}
 */
function replacePart(store, id, input) {
    const body = input || {};
    return mutate(store, id, {
        op: 'replace_part',
        path: pathOf(body),
        value: body.value
    }, expectedOf(body));
}

/**
 * Insert a block into a lesson at a destination index. A missing index appends.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, index?: number, block: object, expectedRevision: number }} input
 * @returns {object}
 */
function insertBlock(store, id, input) {
    const body = input || {};
    return mutate(store, id, {
        op: 'insert_block',
        path: pathOf(body),
        index: body.index,
        block: body.block
    }, expectedOf(body));
}

/**
 * Move a block to a destination index within its lesson.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, index: number, expectedRevision: number }} input
 * @returns {object}
 */
function moveBlock(store, id, input) {
    const body = input || {};
    return mutate(store, id, {
        op: 'move_block',
        path: pathOf(body),
        index: body.index
    }, expectedOf(body));
}

/**
 * Remove the block at an id path.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, expectedRevision: number }} input
 * @returns {object}
 */
function removeBlock(store, id, input) {
    const body = input || {};
    return mutate(store, id, {
        op: 'remove_block',
        path: pathOf(body)
    }, expectedOf(body));
}

/**
 * Set one field under an id path. Unknown names fail validation and are not stored.
 * `alt` and `mediaId` on a word that has an image are written onto that image.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, field: string, value: unknown, expectedRevision: number }} input
 * @returns {object}
 */
function setField(store, id, input) {
    const body = input || {};
    return mutate(store, id, {
        op: 'set_field',
        path: pathOf(body),
        field: body.field,
        value: body.value
    }, expectedOf(body));
}

module.exports = {
    apply,
    openWorkingCopy,
    readPart,
    replacePart,
    insertBlock,
    moveBlock,
    removeBlock,
    setField
};
