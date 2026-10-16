/**
 * The six ADR-0003 part operations. One apply() edits the document.
 * HTTP and MCP adapters call the six exports; they do not edit JSON.
 * Every mutation loads through the store passed in, checks expectedRevision
 * against the stored working-copy revision, validates, and saves compact JSON.
 * A stale revision returns { error: 'revision_conflict', revision } and does
 * not write. The plan object passed to apply() is left unchanged on conflict.
 *
 * Every mutating export takes a trailing write context (./context.js) built by
 * the adapter from its channel. Without one the call throws TypeError and
 * writes nothing. Each successful write also appends one plan.* audit_log row
 * inside the same transaction as the working-copy write (ADR-0011 §1).
 * `audience` and `assigneeId` change only through assignPlan, which
 * personal.assign calls after its account check (ADR-0011, INIT-008 D6).
 * The same transaction also writes the revision row and the provenance marks
 * (./write-log.js). A restore (revisions.restore) is one more operation here,
 * so it is revision-checked, validated, and logged like any write.
 * Provenance: INIT-004/SPEC-003; schemaVersion raise: INIT-006/SPEC-011;
 * schemaVersion 1.2.0 raise: INIT-012/SPEC-002;
 * id_mismatch, assign_required, context and audit: INIT-008/SPEC-002;
 * revision log: INIT-008/SPEC-004; restore: INIT-008/SPEC-005;
 * marks and editState: INIT-008/SPEC-006;
 * insert_lesson, move_lesson, remove_lesson: INIT-010/SPEC-002
 */
'use strict';

const { validatePlan, usesV110, usesV120, V100, V110, V120 } = require('./validate');
const lexicon = require('../db/repos/lexicon');
const db = require('../db');
const { requireContext } = require('./context');
const { logWrite } = require('./write-log');
const provenance = require('./provenance');

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

/** Plan-root fields only assignPlan may change (INIT-008 D6). */
const PLAN_ASSIGN_FIELDS = ['audience', 'assigneeId'];

/**
 * A missing assigneeId and null are the same stored value.
 * @param {unknown} value
 * @returns {unknown}
 */
function assignee(value) {
    return value == null ? null : value;
}

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
function schemaVersionRank(value) {
    if (value === V120) return 2;
    if (value === V110) return 1;
    return 0;
}

function finish(before, next, versionGiven) {
    if (schemaVersionRank(next.schemaVersion) < schemaVersionRank(before.schemaVersion)) {
        return conflict('invalid_op');
    }
    if (!versionGiven) {
        if (usesV120(next)) next.schemaVersion = V120;
        else if (next.schemaVersion === V100 && usesV110(next)) next.schemaVersion = V110;
    }
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
 * Restore from an older document (revisions.restore builds the op). With no path
 * and no field the whole document comes back, except that `id`, `audience`, and
 * `assigneeId` stay as they are now and `schemaVersion` never goes down. With a
 * path the node at that path replaces the node now, or is re-inserted at its
 * former index when it is gone and its parent is not. With a path and a field
 * that one field is set to its old value, or removed if it did not exist then.
 * @param {object} plan The stored plan.
 * @param {object} next A clone of `plan` to edit.
 * @param {object|null} found locate(next, op.path).
 * @param {{ path?: string, field?: string, source: object }} op
 * @returns {{ plan: object }|{ conflict: object }}
 */
function applyRestore(plan, next, found, op) {
    const source = op.source;
    if (!source || typeof source !== 'object' || Array.isArray(source)) return conflict('invalid_op');
    const path = typeof op.path === 'string' ? op.path : '';
    const field = typeof op.field === 'string' ? op.field : '';
    const segs = path.split('/').filter(Boolean);

    if (!field && segs.length === 0) {
        let value;
        try {
            value = cloneJson(source);
        } catch (err) {
            return conflict('invalid_op');
        }
        value.id = next.id;
        value.audience = next.audience;
        value.assigneeId = assignee(next.assigneeId);
        value.revision = next.revision;
        if (schemaVersionRank(next.schemaVersion) > schemaVersionRank(value.schemaVersion)) {
            value.schemaVersion = next.schemaVersion;
        }
        return finish(plan, value, false);
    }

    const then = locate(source, path);
    if (field) {
        if (!FIELD_NAME.test(field)) return conflict('invalid_op');
        if (!found) return conflict('not_found', { path: path, reason: 'node_missing' });
        if (found.kind === 'plan') {
            if (PLAN_ASSIGN_FIELDS.indexOf(field) !== -1) return conflict('assign_required');
            if (field === 'id' || field === 'revision' || field === 'status') return conflict('invalid_op');
        }
        if (!then) return conflict('not_found', { path: path, reason: 'absent_in_revision' });
        if (Object.prototype.hasOwnProperty.call(then.node, field)) {
            let value;
            try {
                value = cloneJson(then.node[field]);
            } catch (err) {
                return conflict('invalid_op');
            }
            if (!assignField(found.node, field, value)) return conflict('invalid_op');
        } else {
            delete found.node[field];
        }
        return finish(plan, next, found.kind === 'plan' && field === 'schemaVersion');
    }

    if (!then || then.kind === 'plan') return conflict('not_found', { path: path, reason: 'absent_in_revision' });
    let node;
    try {
        node = cloneJson(then.node);
    } catch (err) {
        return conflict('invalid_op');
    }
    if (found) {
        found.list[found.index] = node;
        return finish(plan, next, false);
    }

    const listName = segs[segs.length - 2];
    const parentPath = segs.slice(0, -2).join('/');
    const parent = parentPath === '' ? { node: next } : locate(next, parentPath);
    if (!parent || !parent.node || typeof parent.node !== 'object') {
        return conflict('not_found', { path: path, reason: 'parent_missing' });
    }
    let list = parent.node[listName];
    if (!Array.isArray(list)) {
        if (BUCKETS.indexOf(listName) === -1) return conflict('not_found', { path: path, reason: 'parent_missing' });
        list = [];
        parent.node[listName] = list;
    }
    list.splice(Math.max(0, Math.min(then.index, list.length)), 0, node);
    return finish(plan, next, false);
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
        if (found.kind === 'plan' && PLAN_ASSIGN_FIELDS.indexOf(op.field) !== -1) {
            return conflict('assign_required');
        }
        if (op.value === undefined) return conflict('invalid_op');
        let value;
        try {
            value = cloneJson(op.value);
        } catch (err) {
            return conflict('invalid_op');
        }
        // Same defect as replace_part: a changed id breaks every id path that pointed here.
        if (op.field === 'id' && found.kind !== 'plan' && value !== found.node.id) {
            return conflict('id_mismatch', { path: typeof op.path === 'string' ? op.path : '' });
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
            if (value.audience !== next.audience || assignee(value.assigneeId) !== assignee(next.assigneeId)) {
                return conflict('assign_required');
            }
            value.revision = next.revision;
            return finish(plan, value, true);
        }
        if (value.id !== found.node.id) {
            return conflict('id_mismatch', { path: typeof op.path === 'string' ? op.path : '' });
        }
        found.list[found.index] = value;
        return finish(plan, next, false);
    }

    if (op.op === 'restore') return applyRestore(plan, next, found, op);

    if (op.op === 'assign') {
        if (!found || found.kind !== 'plan') return conflict('invalid_op');
        if (op.audience !== 'shared' && op.audience !== 'personal') return conflict('invalid_op');
        if (op.audience === 'shared' && op.assigneeId != null) return conflict('invalid_op');
        if (op.audience === 'personal' && (typeof op.assigneeId !== 'string' || op.assigneeId.length === 0)) {
            return conflict('invalid_op');
        }
        next.audience = op.audience;
        next.assigneeId = op.audience === 'shared' ? null : op.assigneeId;
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

    // INIT-010/SPEC-002: lesson stack ops. A unit keeps at least one lesson
    // (schema minItems); removing the last lesson of a unit is refused here.
    if (op.op === 'insert_lesson') {
        if (!found || found.kind !== 'unit' || !Array.isArray(found.node.lessons)) {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        if (!op.lesson || typeof op.lesson !== 'object' || Array.isArray(op.lesson)) {
            return conflict('invalid_op');
        }
        const at = op.index == null ? found.node.lessons.length : op.index;
        if (!Number.isInteger(at) || at < 0 || at > found.node.lessons.length) {
            return conflict('invalid_op');
        }
        let lesson;
        try {
            lesson = cloneJson(op.lesson);
        } catch (err) {
            return conflict('invalid_op');
        }
        found.node.lessons.splice(at, 0, lesson);
        return finish(plan, next, false);
    }

    if (op.op === 'move_lesson') {
        if (!found || found.kind !== 'lesson') {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        if (!Number.isInteger(op.index)) return conflict('invalid_op');
        const item = found.list.splice(found.index, 1)[0];
        const at = Math.max(0, Math.min(op.index, found.list.length));
        found.list.splice(at, 0, item);
        return finish(plan, next, false);
    }

    if (op.op === 'remove_lesson') {
        if (!found || found.kind !== 'lesson') {
            return conflict('not_found', { path: typeof op.path === 'string' ? op.path : '' });
        }
        if (found.list.length <= 1) return conflict('invalid_op');
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
    if (conflictBody.reason != null) out.reason = conflictBody.reason;
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
 * Save the working copy, the revision row (with a baseline when the plan has
 * none for its current revision), the provenance marks, and the plan.* audit
 * row in one transaction. If any of them throws, the whole write rolls back and
 * the error propagates, so no mutation exists without its rows and no row
 * without its mutation.
 * @param {object} store
 * @param {object} row
 * @param {object} plan The document to store.
 * @param {object} ctx Context from makeContext.
 * @param {{ op: string, path: string, field?: string, summary?: string, before: object }} meta
 *   `before` is the plan as opened, before the write.
 * @returns {object} `overwrote` is present only when an AI or script write
 *   replaced a field a person last wrote.
 */
function persist(store, row, plan, ctx, meta) {
    const document = JSON.stringify(plan);
    let saved;
    let logged;
    db.transaction(() => {
        saved = store.saveWorkingCopy({
            id: row.id,
            document: document,
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId == null ? null : plan.assigneeId
        });
        logged = logWrite({
            planId: row.id,
            before: meta.before,
            after: plan,
            afterText: saved.document,
            op: meta.op,
            path: meta.path,
            field: meta.field,
            summary: meta.summary
        }, ctx);
    });
    const out = {
        plan: JSON.parse(saved.document),
        document: saved.document,
        revision: saved.revision,
        status: saved.status
    };
    if (logged.overwrote.length) out.overwrote = logged.overwrote;
    return out;
}

/**
 * @param {object} store
 * @param {string} id
 * @param {object} op
 * @param {number} expectedRevision
 * @param {object} ctx Context from makeContext. Required.
 * @returns {object}
 */
function mutate(store, id, op, expectedRevision, ctx) {
    requireContext(ctx);
    const opened = openWorkingCopy(store, id);
    if (opened.error) return failure(opened.error, opened.revision);
    const result = apply(opened.plan, op, expectedRevision);
    if (result.conflict) return fromConflict(result.conflict, opened.row.revision);
    if (result.plan.id !== opened.row.id) {
        return { error: 'invalid_op', revision: opened.row.revision };
    }
    const validation = validatePlan(result.plan, { lexicon, snapshot: opened.row.snapshot });
    if (refusesWrite(validation)) return validationError(validation, opened.row.revision);
    let field;
    if (op.op === 'set_field') field = op.field;
    else if (op.op === 'assign') field = 'audience';
    else if (op.op === 'restore' && op.field) field = op.field;
    const saved = persist(store, opened.row, result.plan, ctx, {
        op: op.op,
        path: typeof op.path === 'string' ? op.path : '',
        field: field,
        summary: op.op === 'restore' ? op.summary : undefined,
        before: opened.plan
    });
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
 * @param {object} [ctx] Optional. When given it must come from makeContext. It
 *   decides whether `editState` may name the person who made an edit: only an
 *   admin or script context sees the name. Without one, no name is returned.
 * @returns {{ part: object, revision: number, editState: object }
 *   |{ error: string, revision?: number, path?: string }}
 *   `editState` is advisory (provenance.editStateFor). It never changes `part`
 *   or `revision`, and nothing refuses a write because of it.
 */
function readPart(store, id, input, ctx) {
    if (ctx !== undefined) requireContext(ctx);
    const opened = openWorkingCopy(store, id);
    if (opened.error) return failure(opened.error, opened.revision);
    const path = pathOf(input);
    const result = apply(opened.plan, { op: 'read_part', path: path }, opened.row.revision);
    if (result.conflict) return fromConflict(result.conflict, opened.row.revision);
    return {
        part: result.part,
        revision: opened.row.revision,
        editState: provenance.editStateFor(opened.row.id, path, ctx)
    };
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
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function replacePart(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'replace_part',
        path: pathOf(body),
        value: body.value
    }, expectedOf(body), ctx);
}

/**
 * Insert a block into a lesson at a destination index. A missing index appends.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, index?: number, block: object, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function insertBlock(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'insert_block',
        path: pathOf(body),
        index: body.index,
        block: body.block
    }, expectedOf(body), ctx);
}

/**
 * Move a block to a destination index within its lesson.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, index: number, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function moveBlock(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'move_block',
        path: pathOf(body),
        index: body.index
    }, expectedOf(body), ctx);
}

/**
 * Remove the block at an id path.
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function removeBlock(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'remove_block',
        path: pathOf(body)
    }, expectedOf(body), ctx);
}

/**
 * Insert a lesson into a unit at a destination index. A missing index appends.
 * Provenance: INIT-010/SPEC-002
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, index?: number, lesson: object, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function insertLesson(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'insert_lesson',
        path: pathOf(body),
        index: body.index,
        lesson: body.lesson
    }, expectedOf(body), ctx);
}

/**
 * Move a lesson to a destination index within its unit.
 * Provenance: INIT-010/SPEC-002
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, index: number, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function moveLesson(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'move_lesson',
        path: pathOf(body),
        index: body.index
    }, expectedOf(body), ctx);
}

/**
 * Remove the lesson at an id path. Refused when it is the last lesson of its unit.
 * Provenance: INIT-010/SPEC-002
 * @param {object} store
 * @param {string} id
 * @param {{ path?: string, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function removeLesson(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'remove_lesson',
        path: pathOf(body)
    }, expectedOf(body), ctx);
}

/**
 * Set one field under an id path. Unknown names fail validation and are not stored.
 * `alt` and `mediaId` on a word that has an image are written onto that image.
 * @param {object} store
 * @param {string} id
 * `audience` and `assigneeId` on the plan are refused with `assign_required`.
 * @param {{ path?: string, field: string, value: unknown, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function setField(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'set_field',
        path: pathOf(body),
        field: body.field,
        value: body.value
    }, expectedOf(body), ctx);
}

/**
 * The only writer of the plan's `audience` and `assigneeId`. It checks shape
 * only: `shared` stores assigneeId null, `personal` needs a username. The
 * active-account check is personal.assign's, and adapters call personal.assign,
 * never this function (ADR-0011 §1, INIT-008 D6).
 * @param {object} store
 * @param {string} id
 * @param {{ audience: 'shared'|'personal', assigneeId?: string|null, expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {object}
 */
function assignPlan(store, id, input, ctx) {
    const body = input || {};
    return mutate(store, id, {
        op: 'assign',
        path: '',
        audience: body.audience,
        assigneeId: body.assigneeId
    }, expectedOf(body), ctx);
}

module.exports = {
    apply,
    mutate,
    openWorkingCopy,
    refusesWrite,
    validationError,
    readPart,
    replacePart,
    insertBlock,
    moveBlock,
    removeBlock,
    insertLesson,
    moveLesson,
    removeLesson,
    setField,
    assignPlan
};
