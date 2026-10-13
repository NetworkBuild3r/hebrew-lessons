/**
 * Personal-plan assignee checks. Confirms the account, then calls ops.assignPlan.
 * It does not reimplement an operation and it writes no plan SQL.
 * assigneeId is the account username (ADR-0003's opaque id), not the
 * integer primary key and not the name shown to people.
 * Provenance: INIT-004/SPEC-010; assignPlan and ctx: INIT-008/SPEC-002
 */
'use strict';

const ops = require('./ops');
const { requireContext } = require('./context');
const users = require('../db/repos/users');

const ASSIGNEE_MAX = 64;

/**
 * @param {unknown} assigneeId
 * @returns {{ id: number, username: string, status: string }|null}
 */
function activeAccount(assigneeId) {
    if (typeof assigneeId !== 'string' || assigneeId.length === 0 || assigneeId.length > ASSIGNEE_MAX) {
        return null;
    }
    const account = users.findAccount(assigneeId);
    if (!account || account.status !== 'active' || typeof account.username !== 'string') {
        return null;
    }
    return account;
}

/**
 * @param {{ error: string, revision?: number }} opened
 * @returns {object}
 */
function refused(opened) {
    const out = { error: 'assignee_refused' };
    if (opened.revision != null) out.revision = opened.revision;
    return out;
}

/**
 * Set audience and assigneeId through ops.assignPlan, the only writer of those
 * two fields. `shared` stores assigneeId null. `personal` stores the active
 * account's username. A disabled or unknown account is `assignee_refused` and
 * is not written. `ctx` is the write context the adapter built from its channel.
 * @param {object} store Plan store passed through to ops.js.
 * @param {string} id Plan id.
 * @param {{ audience?: unknown, assigneeId?: unknown, expectedRevision?: number }} input
 * @param {object} ctx Write context from server/plans/context.js.
 * @returns {object}
 */
function assign(store, id, input, ctx) {
    requireContext(ctx);
    const body = input && typeof input === 'object' ? input : {};
    const audience = body.audience;
    if (audience !== 'personal' && audience !== 'shared') {
        return { error: 'invalid_audience' };
    }

    const opened = ops.readPart(store, id, { path: '' });
    if (opened.error) {
        const missing = { error: opened.error };
        if (opened.revision != null) missing.revision = opened.revision;
        return missing;
    }

    let assigneeId = null;
    if (audience === 'personal') {
        const account = activeAccount(body.assigneeId);
        if (!account) return refused(opened);
        assigneeId = account.username;
    }

    return ops.assignPlan(store, id, {
        audience: audience,
        assigneeId: assigneeId,
        expectedRevision: body.expectedRevision
    }, ctx);
}

module.exports = { assign };
