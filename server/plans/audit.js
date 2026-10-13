/**
 * One audit_log row per plan write (ADR-0011 §1, ADR-0004 §10).
 * The row carries the actor string, the action, the plan id as target, and a
 * compact detail: revision, path, field name, tool, request id. It never
 * carries a field value or any document text. Callers write it inside the same
 * db.transaction as the plan write, so no mutation exists without its row.
 * audit.append is the only audit writer.
 * Provenance: INIT-008/SPEC-002
 */
'use strict';

const { actorString, requireContext } = require('./context');

/** The nine actions ADR-0011 §1 names. Only SPEC-002 writes seven of them. */
const ACTIONS = Object.freeze([
    'plan.create',
    'plan.set_field',
    'plan.replace_part',
    'plan.insert_block',
    'plan.move_block',
    'plan.remove_block',
    'plan.publish',
    'plan.restore',
    'plan.assign'
]);

const DETAIL_TEXT_MAX = 256;

/**
 * @param {unknown} value
 * @returns {string|undefined}
 */
function clip(value) {
    if (typeof value !== 'string') return undefined;
    return value.length > DETAIL_TEXT_MAX ? value.slice(0, DETAIL_TEXT_MAX) : value;
}

/**
 * Append one row. Throws when the insert fails so the caller's transaction
 * rolls the plan write back.
 * @param {object} ctx Context from makeContext.
 * @param {string} action One of ACTIONS.
 * @param {string} planId
 * @param {{ revision?: number, path?: string, field?: string }} detail
 */
function record(ctx, action, planId, detail) {
    requireContext(ctx);
    if (ACTIONS.indexOf(action) === -1) throw new TypeError('unknown plan audit action');
    const src = detail || {};
    const out = {};
    if (Number.isInteger(src.revision)) out.revision = src.revision;
    const path = clip(src.path);
    if (path !== undefined) out.path = path;
    const field = clip(src.field);
    if (field !== undefined) out.field = field;
    if (ctx.actor.tool) out.tool = ctx.actor.tool;
    if (ctx.requestId) out.requestId = ctx.requestId;
    // Resolved at call time so a reloaded database module and its repo stay paired.
    require('../db/repos/audit').append({
        actor: actorString(ctx),
        action: action,
        target: planId,
        detail: JSON.stringify(out)
    });
}

module.exports = { record, ACTIONS };
