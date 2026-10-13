/**
 * Field provenance and the advisory edit state (ADR-0011 §4).
 *
 * plan_field_marks records, per id path and field, who wrote it last and the
 * last time a person did, with the value the person replaced. read_part returns
 * that as `editState`, so an AI about to re-edit a field a person fixed can see
 * what the person changed. It is a note. No function here returns an error,
 * throws, or changes a write's outcome because of a mark: a write by any actor
 * to any field succeeds or fails on exactly the grounds it did before marks.
 *
 * Marks come from the same field-level diff that powers history (./diff.js), so
 * there is one differ. A missing mark means "unknown", never "the AI wrote it".
 *
 * Names: a person's account name goes to an admin or a script caller and never
 * to an `ai` caller, which sees `human` and nothing else (initiative §16 A2).
 * To change that, change `showNames`.
 *
 * Provenance: INIT-008/SPEC-006
 */
'use strict';

const crypto = require('node:crypto');
const marksRepo = require('../db/repos/plan-field-marks');
const { diffDocuments, stable } = require('./diff');
const { requireContext, isContext } = require('./context');

/** previous_value_json is bounded in bytes by the migration's CHECK. */
const PREVIOUS_MAX_BYTES = 32768;
const HUMAN_EDITED_MAX = 50;
const OVERWROTE_MAX = 20;
const VALUE_MAX_CHARS = 4000;
const CUT = '\u2026';

/**
 * @param {string} planId
 * @returns {Map<string, object>} Marks keyed by path and field.
 */
function marksByKey(planId) {
    const map = new Map();
    for (const mark of marksRepo.forPlan(planId)) {
        map.set(mark.path + '\u0000' + mark.field, mark);
    }
    return map;
}

/**
 * The fields a write changed and the nodes it removed, uncapped. The plan
 * `status` is bookkeeping (every edit sets it to draft), not an edit.
 * @param {object} before
 * @param {object} after
 * @returns {object[]}
 */
function changesOf(before, after) {
    return diffDocuments(before, after, { full: true }).entries.filter(
        (entry) => !(entry.path === '' && entry.field === 'status')
    );
}

/**
 * @param {unknown} value
 * @returns {string}
 */
function hashOf(value) {
    return crypto.createHash('sha256').update(stable(value), 'utf8').digest('hex');
}

/**
 * @param {unknown} ctx
 * @returns {boolean} True when the caller may be told a person's account name.
 */
function showNames(ctx) {
    return isContext(ctx) && ctx.actor.kind !== 'ai';
}

/**
 * Record the marks for one write. Runs inside the write's transaction. A failure
 * throws, so the working copy, revision row, and audit row roll back with it.
 * @param {string} planId
 * @param {object} before Plan document before the write.
 * @param {object} after Plan document after the write.
 * @param {object} ctx Write context.
 * @param {number} revision The plan's new revision.
 * @param {object[]} [entries] Precomputed `changesOf(before, after)`.
 */
function recordWrite(planId, before, after, ctx, revision, entries) {
    requireContext(ctx);
    const changes = entries || changesOf(before, after);
    if (!changes.length) return;
    const existing = marksByKey(planId);
    const now = new Date().toISOString();
    const actor = ctx.actor;

    for (const entry of changes) {
        if (entry.kind === 'removed') {
            marksRepo.deleteUnder(planId, entry.path);
            continue;
        }
        if (entry.kind !== 'changed') continue;

        const prior = existing.get(entry.path + '\u0000' + entry.field) || null;
        const mark = {
            planId: planId,
            path: entry.path,
            field: entry.field,
            lastActorKind: actor.kind,
            lastActorName: actor.name,
            lastRevision: revision,
            lastAt: now,
            valueHash: hashOf(Object.prototype.hasOwnProperty.call(entry, 'after') ? entry.after : null),
            humanRevision: prior ? prior.humanRevision : null,
            humanAt: prior ? prior.humanAt : null,
            humanActorName: prior ? prior.humanActorName : null,
            previousValueJson: prior ? prior.previousValueJson : null,
            previousWriterKind: prior ? prior.previousWriterKind : null
        };
        if (actor.kind === 'human') {
            const text = JSON.stringify(Object.prototype.hasOwnProperty.call(entry, 'before') ? entry.before : null);
            mark.humanRevision = revision;
            mark.humanAt = now;
            mark.humanActorName = actor.name;
            mark.previousValueJson = Buffer.byteLength(text, 'utf8') > PREVIOUS_MAX_BYTES ? null : text;
            mark.previousWriterKind = prior ? prior.lastActorKind : 'unknown';
        }
        marksRepo.upsert(mark);
    }
}

/**
 * A value for a note: cut when long.
 * @param {unknown} value
 * @returns {{ value: unknown, cut: boolean }}
 */
function clipValue(value) {
    if (value === undefined) return { value: null, cut: false };
    const text = typeof value === 'string' ? value : JSON.stringify(value);
    if (text.length > VALUE_MAX_CHARS) return { value: text.slice(0, VALUE_MAX_CHARS) + CUT, cut: true };
    return { value: value, cut: false };
}

/**
 * The one place an AI write learns it replaced a person's value. Advisory only:
 * the result goes on the write result and changes nothing else. Remove this
 * function and its call in write-log.js and no other behavior changes
 * (initiative §16 A3). A person's own write returns an empty list.
 * Runs before recordWrite, while the marks still describe the earlier state.
 * @param {string} planId
 * @param {object[]} changed Entries from changesOf().
 * @param {object} ctx Write context.
 * @returns {Array<{ path: string, field: string, humanRevision: number, valueSetByPerson: unknown, valueTruncated?: true }>}
 */
function overwrittenBy(planId, changed, ctx) {
    requireContext(ctx);
    if (ctx.actor.kind === 'human') return [];
    const candidates = changed.filter((entry) => entry.kind === 'changed');
    if (!candidates.length) return [];
    const marks = marksByKey(planId);
    const out = [];
    for (const entry of candidates) {
        if (out.length >= OVERWROTE_MAX) break;
        const mark = marks.get(entry.path + '\u0000' + entry.field);
        if (!mark || mark.lastActorKind !== 'human') continue;
        const clipped = clipValue(entry.before);
        const item = {
            path: entry.path,
            field: entry.field,
            humanRevision: mark.humanRevision == null ? mark.lastRevision : mark.humanRevision,
            valueSetByPerson: clipped.value
        };
        if (clipped.cut) item.valueTruncated = true;
        out.push(item);
    }
    return out;
}

/**
 * @param {string} path
 * @returns {string} The path without empty segments.
 */
function normalPath(path) {
    return typeof path === 'string' ? path.split('/').filter(Boolean).join('/') : '';
}

/**
 * The advisory edit state for a read at `path` (empty is the whole plan).
 * @param {string} planId
 * @param {string} path
 * @param {object} [ctx] When absent the caller is treated as an `ai` caller.
 * @returns {{
 *   advisory: true,
 *   lastHumanEdit: { revision: number, at: string }|null,
 *   humanEdited: object[],
 *   more: number
 * }}
 */
function editStateFor(planId, path, ctx) {
    const names = showNames(ctx);
    const human = marksRepo.forPathPrefix(planId, normalPath(path))
        .filter((mark) => mark.lastActorKind === 'human')
        .sort((x, y) => (y.humanRevision || 0) - (x.humanRevision || 0)
            || (x.path < y.path ? -1 : x.path > y.path ? 1 : 0)
            || (x.field < y.field ? -1 : x.field > y.field ? 1 : 0));

    const humanEdited = human.slice(0, HUMAN_EDITED_MAX).map((mark) => {
        const item = {
            path: mark.path,
            field: mark.field,
            revision: mark.humanRevision,
            at: mark.humanAt,
            previousValue: null,
            previousWriter: mark.previousWriterKind || 'unknown'
        };
        if (mark.previousValueJson == null) {
            item.previousValueOmitted = true;
        } else {
            try {
                item.previousValue = JSON.parse(mark.previousValueJson);
            } catch (err) {
                item.previousValueOmitted = true;
            }
        }
        if (names) item.by = mark.humanActorName;
        return item;
    });

    return {
        advisory: true,
        lastHumanEdit: marksRepo.lastHuman(planId),
        humanEdited: humanEdited,
        more: Math.max(0, human.length - HUMAN_EDITED_MAX)
    };
}

module.exports = {
    HUMAN_EDITED_MAX,
    OVERWROTE_MAX,
    changesOf,
    editStateFor,
    overwrittenBy,
    recordWrite
};
