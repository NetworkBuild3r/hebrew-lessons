/**
 * The plan revision log: write one row per successful plan write, list and read
 * past revisions, diff any two, and restore one back as a new revision
 * (ADR-0011 §2, §3). Service layer; the SQL is in db/repos/plan-revisions.js.
 *
 * History is append-only. Nothing here updates or deletes a revision row. A
 * restore is itself a write: it goes through ops.mutate like any other, so it
 * is validated, revision-checked, audited, logged, and restorable in turn.
 *
 * Names: an `ai` caller sees a person's revisions as `human` with no account
 * name; an admin or a script caller sees the name (initiative §16 A2).
 * Provenance: INIT-008/SPEC-004, SPEC-005
 */
'use strict';

const repo = require('../db/repos/plan-revisions');
const defaultStore = require('./store');
const { requireContext } = require('./context');
const { diffDocuments } = require('./diff');

const LIST_DEFAULT = 50;
const LIST_MAX = 200;
const SUMMARY_MAX = 200;
const ACTOR_KINDS = ['ai', 'human', 'system'];

/**
 * A value-free label for the log: the operation, where, and which field. It
 * never holds a field value or plan text.
 * @param {string} op
 * @param {string} [path]
 * @param {string} [field]
 * @returns {string}
 */
function summarize(op, path, field) {
    let text = op;
    if (path) text += ' ' + path;
    else if (field) text += ' (plan)';
    if (field) text += '#' + field;
    return text.length > SUMMARY_MAX ? text.slice(0, SUMMARY_MAX) : text;
}

/**
 * Append the row for one write, and a baseline row first when the plan has no
 * row for its current revision (a plan that predates the log, or one written
 * by a script). The baseline holds the pre-write document, so a restore to
 * "before the first logged edit" works. Runs in the write's transaction.
 * @param {{
 *   planId: string, before: object|null, after: object, afterText: string,
 *   op: string, path?: string, field?: string, summary?: string
 * }} entry
 * @param {object} ctx
 */
function record(entry, ctx) {
    requireContext(ctx);
    const actor = ctx.actor;
    if (entry.before && !repo.has(entry.planId, entry.before.revision)) {
        repo.append({
            planId: entry.planId,
            revision: entry.before.revision,
            actorKind: 'system',
            actorName: 'baseline',
            op: 'baseline',
            path: '',
            field: '',
            summary: 'baseline',
            document: JSON.stringify(entry.before)
        });
    }
    const summary = typeof entry.summary === 'string' && entry.summary
        ? entry.summary.slice(0, SUMMARY_MAX)
        : summarize(entry.op, entry.path, entry.field);
    repo.append({
        planId: entry.planId,
        revision: entry.after.revision,
        actorKind: actor.kind,
        actorName: actor.name,
        tool: actor.tool || null,
        client: actor.client || null,
        requestId: ctx.requestId || null,
        op: entry.op,
        path: entry.path || '',
        field: entry.field || '',
        summary: summary,
        document: entry.afterText
    });
}

/**
 * @param {object} meta A metadata row from the repo.
 * @param {object} ctx
 * @returns {object}
 */
function shapeMeta(meta, ctx) {
    const hide = ctx.actor.kind === 'ai' && meta.actorKind === 'human';
    return {
        revision: meta.revision,
        op: meta.op,
        path: meta.path,
        field: meta.field,
        summary: meta.summary,
        actorKind: meta.actorKind,
        actorName: hide ? 'human' : meta.actorName,
        tool: meta.tool,
        client: meta.client,
        at: meta.at
    };
}

/**
 * @param {unknown} value
 * @returns {boolean}
 */
function optionalInt(value) {
    return value == null || (Number.isInteger(value) && value >= 0);
}

/**
 * Revisions of one plan, newest first.
 * @param {string} planId
 * @param {{
 *   sinceRevision?: number, beforeRevision?: number, actorKind?: string,
 *   pathPrefix?: string, limit?: number
 * }} [opts]
 * @param {object} ctx
 * @returns {{ revisions: object[], truncated: boolean }|{ error: string }}
 */
function list(planId, opts, ctx) {
    requireContext(ctx);
    const o = opts || {};
    if (!optionalInt(o.sinceRevision) || !optionalInt(o.beforeRevision)
        || (o.actorKind != null && ACTOR_KINDS.indexOf(o.actorKind) === -1)
        || (o.pathPrefix != null && typeof o.pathPrefix !== 'string')
        || (o.limit != null && (!Number.isInteger(o.limit) || o.limit < 1))) {
        return { error: 'invalid_op' };
    }
    if (typeof planId !== 'string' || planId === '') return { error: 'not_found' };
    const limit = Math.min(o.limit == null ? LIST_DEFAULT : o.limit, LIST_MAX);
    const rows = repo.list(planId, {
        sinceRevision: o.sinceRevision,
        beforeRevision: o.beforeRevision,
        actorKind: o.actorKind,
        pathPrefix: o.pathPrefix,
        limit: limit + 1
    });
    if (!rows.length && !defaultStore.loadWorkingCopy(planId)) return { error: 'not_found' };
    const truncated = rows.length > limit;
    return {
        revisions: rows.slice(0, limit).map((meta) => shapeMeta(meta, ctx)),
        truncated: truncated
    };
}

/**
 * One revision with its parsed document.
 * @param {string} planId
 * @param {number} revision
 * @param {object} ctx
 * @returns {object} `{ ...metadata, document }`, or `{ error }`.
 */
function get(planId, revision, ctx) {
    requireContext(ctx);
    if (typeof planId !== 'string' || planId === '' || !Number.isInteger(revision) || revision < 1) {
        return { error: 'not_found' };
    }
    const row = repo.get(planId, revision);
    if (!row) return { error: 'not_found' };
    if (row.error) return { error: row.error };
    let document;
    try {
        document = JSON.parse(row.document);
    } catch (err) {
        return { error: 'revision_corrupt' };
    }
    const out = shapeMeta(row, ctx);
    out.document = document;
    return out;
}

/**
 * Resolve a diff side: an integer revision, 'working', or 'published'.
 * @param {string} planId
 * @param {unknown} ref
 * @returns {{ document: object, revision: number|null, ref: string|number }|{ error: string }}
 */
function resolveSide(planId, ref) {
    if (ref === 'working') {
        const row = defaultStore.loadWorkingCopy(planId);
        if (!row) return { error: 'not_found' };
        let document;
        try {
            document = JSON.parse(row.document);
        } catch (err) {
            return { error: 'invalid_plan' };
        }
        document.revision = row.revision;
        document.status = row.status;
        return { document: document, revision: row.revision, ref: 'working' };
    }
    if (ref === 'published') {
        if (!defaultStore.loadWorkingCopy(planId)) return { error: 'not_found' };
        const text = defaultStore.loadSnapshot(planId);
        if (!text) return { error: 'no_snapshot' };
        let document;
        try {
            document = JSON.parse(text);
        } catch (err) {
            return { error: 'invalid_plan' };
        }
        return { document: document, revision: document.revision, ref: 'published' };
    }
    if (Number.isInteger(ref) && ref >= 1) {
        const row = repo.get(planId, ref);
        if (!row) return { error: 'not_found' };
        if (row.error) return { error: row.error };
        try {
            return { document: JSON.parse(row.document), revision: ref, ref: ref };
        } catch (err) {
            return { error: 'revision_corrupt' };
        }
    }
    return { error: 'invalid_op' };
}

/**
 * Field-level diff between two revisions of a plan.
 * @param {string} planId
 * @param {number|'working'|'published'} from
 * @param {number|'working'|'published'} to
 * @param {{ path?: string }} [opts] `path` keeps that node and everything below.
 * @param {object} ctx
 * @returns {{ from: object, to: object, entries: object[], truncated: boolean }|{ error: string }}
 */
function diff(planId, from, to, opts, ctx) {
    requireContext(ctx);
    const o = opts || {};
    if (o.path != null && typeof o.path !== 'string') return { error: 'invalid_op' };
    if (typeof planId !== 'string' || planId === '') return { error: 'not_found' };
    const left = resolveSide(planId, from);
    if (left.error) return { error: left.error };
    const right = resolveSide(planId, to);
    if (right.error) return { error: right.error };
    const result = diffDocuments(left.document, right.document, { path: o.path });
    return {
        from: { ref: left.ref, revision: left.revision },
        to: { ref: right.ref, revision: right.revision },
        entries: result.entries,
        truncated: result.truncated
    };
}

/**
 * Put an old revision, one part of it, or one field of it back as a new
 * revision. Either actor may call it. Whole-document restore keeps the current
 * `audience` and `assigneeId` and never lowers `schemaVersion`; a field restore
 * of `audience` or `assigneeId` is `assign_required`. The result is validated
 * like any write and nothing is written when it fails.
 * @param {object} store Plan store.
 * @param {string} planId
 * @param {{ fromRevision: number, path?: string, field?: string, expectedRevision: number }} input
 * @param {object} ctx Write context.
 * @returns {object} The write result plus `restoredFrom`, or `{ error }`.
 */
function restore(store, planId, input, ctx) {
    requireContext(ctx);
    const body = input && typeof input === 'object' ? input : {};
    const from = body.fromRevision;
    if (!Number.isInteger(from) || from < 1) return { error: 'invalid_op' };
    if ((body.path != null && typeof body.path !== 'string')
        || (body.field != null && typeof body.field !== 'string')) {
        return { error: 'invalid_op' };
    }
    if (typeof planId !== 'string' || planId === '') return { error: 'not_found' };
    const path = body.path == null ? '' : body.path;
    const field = body.field == null ? '' : body.field;

    const row = repo.get(planId, from);
    if (!row) return { error: 'not_found' };
    if (row.error) return { error: row.error };
    let source;
    try {
        source = JSON.parse(row.document);
    } catch (err) {
        return { error: 'revision_corrupt' };
    }

    // Resolved here and not at the top: ops requires write-log, which requires this module.
    const ops = require('./ops');
    const label = path || field ? summarize('restore', path, field) : 'restore';
    const result = ops.mutate(store, planId, {
        op: 'restore',
        path: path,
        field: field,
        source: source,
        summary: label + ' from r' + from
    }, body.expectedRevision, ctx);
    if (!result.error) result.restoredFrom = from;
    return result;
}

module.exports = { LIST_DEFAULT, LIST_MAX, diff, get, list, record, restore, summarize };
