/**
 * Plan document store. The only SQL for plan working copies and snapshots.
 * Stores the caller's JSON text unchanged. Revision and publish rules live
 * in the operations module, not here.
 * Provenance: INIT-004/SPEC-001
 */
'use strict';

const db = require('../db');

/**
 * @param {string} id
 * @returns {string}
 */
function requireId(id) {
    if (typeof id !== 'string' || id.length === 0) {
        throw new TypeError('id must be a non-empty string');
    }
    return id;
}

/**
 * @param {string} document
 * @returns {string}
 */
function requireDocument(document) {
    if (typeof document !== 'string' || document.length === 0) {
        throw new TypeError('document must be a non-empty JSON string');
    }
    return document;
}

/**
 * @param {object|undefined} row
 * @returns {object|null}
 */
function mapRow(row) {
    if (!row) return null;
    return {
        id: row.id,
        revision: row.revision,
        status: row.status,
        audience: row.audience,
        assigneeId: row.assignee_id,
        document: row.working_copy,
        snapshot: row.snapshot,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

/**
 * Load one plan's working copy and the columns stored beside it.
 * @param {string} id Plan slug.
 * @returns {{
 *   id: string,
 *   revision: number,
 *   status: string,
 *   audience: string,
 *   assigneeId: string|null,
 *   document: string,
 *   snapshot: string|null,
 *   createdAt: string,
 *   updatedAt: string
 * }|null}
 */
function loadWorkingCopy(id) {
    const planId = requireId(id);
    return mapRow(db.get(
        `SELECT id, revision, status, audience, assignee_id,
                working_copy, snapshot, created_at, updated_at
         FROM plans WHERE id = ?`,
        planId
    ));
}

/**
 * Insert or replace the working-copy text and its columns.
 * Leaves snapshot and created_at untouched on update.
 * @param {{
 *   id: string,
 *   document: string,
 *   revision?: number,
 *   status?: string,
 *   audience: string,
 *   assigneeId?: string|null
 * }} input
 * @returns {object} The stored row, same shape as loadWorkingCopy.
 */
function saveWorkingCopy(input) {
    const row = input || {};
    const id = requireId(row.id);
    const document = requireDocument(row.document);
    const revision = row.revision == null ? 1 : row.revision;
    const status = row.status == null ? 'draft' : row.status;
    const audience = row.audience;
    const assigneeId = row.assigneeId == null ? null : row.assigneeId;
    if (!Number.isInteger(revision)) {
        throw new TypeError('revision must be an integer');
    }
    if (typeof status !== 'string') {
        throw new TypeError('status must be a string');
    }
    if (typeof audience !== 'string') {
        throw new TypeError('audience must be a string');
    }
    if (assigneeId != null && typeof assigneeId !== 'string') {
        throw new TypeError('assigneeId must be a string or null');
    }
    const now = new Date().toISOString();
    const existing = db.get('SELECT id, created_at FROM plans WHERE id = ?', id);
    if (!existing) {
        db.run(
            `INSERT INTO plans (
                id, revision, status, audience, assignee_id,
                working_copy, snapshot, created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
            id,
            revision,
            status,
            audience,
            assigneeId,
            document,
            now,
            now
        );
    } else {
        db.run(
            `UPDATE plans
             SET revision = ?, status = ?, audience = ?, assignee_id = ?,
                 working_copy = ?, updated_at = ?
             WHERE id = ?`,
            revision,
            status,
            audience,
            assigneeId,
            document,
            now,
            id
        );
    }
    return loadWorkingCopy(id);
}

/**
 * Load the published snapshot text for a plan.
 * @param {string} id Plan slug.
 * @returns {string|null} Snapshot JSON text, or null when the plan or snapshot is absent.
 */
function loadSnapshot(id) {
    const planId = requireId(id);
    const row = db.get('SELECT snapshot FROM plans WHERE id = ?', planId);
    if (!row) return null;
    return row.snapshot;
}

/**
 * Store snapshot JSON text on an existing plan. Does not change the working copy,
 * revision, status, audience, or assignee.
 * @param {string} id Plan slug.
 * @param {string|null} snapshot JSON text, or null to clear it.
 * @returns {string|null} The stored snapshot text.
 */
function saveSnapshot(id, snapshot) {
    const planId = requireId(id);
    if (snapshot != null && typeof snapshot !== 'string') {
        throw new TypeError('snapshot must be a string or null');
    }
    const now = new Date().toISOString();
    const result = db.run(
        'UPDATE plans SET snapshot = ?, updated_at = ? WHERE id = ?',
        snapshot,
        now,
        planId
    );
    if (!result.changes) {
        throw new Error('plan not found');
    }
    return loadSnapshot(planId);
}

/**
 * Published snapshot texts. Working copies are not included.
 * Callers decide who may see each lesson inside a snapshot.
 * @returns {Array<{ id: string, snapshot: string }>}
 */
function listSnapshots() {
    return db.all(
        `SELECT id, snapshot FROM plans
         WHERE snapshot IS NOT NULL AND snapshot != ''`
    ).map((row) => ({ id: row.id, snapshot: row.snapshot }));
}

/** Most rows listPlans returns. */
const LIST_MAX = 200;

/**
 * One row per plan for the plan list, most recently updated first. The two
 * documents are not loaded: the title, and the revision the snapshot holds, are
 * read in SQL and are null when a stored document is not valid JSON.
 * Provenance: INIT-008/SPEC-007
 * @param {number} [limit] At most LIST_MAX (default). Ask for one more than
 *   you show to learn whether the list was cut.
 * @returns {Array<{
 *   id: string, revision: number, status: string, audience: string,
 *   assigneeId: string|null, updatedAt: string, title: string|null,
 *   hasSnapshot: boolean, snapshotRevision: number|null
 * }>}
 */
function listPlans(limit) {
    const cap = Number.isInteger(limit) && limit > 0 ? Math.min(limit, LIST_MAX + 1) : LIST_MAX;
    return db.all(
        `SELECT id, revision, status, audience, assignee_id, updated_at,
                CASE WHEN json_valid(working_copy)
                     THEN json_extract(working_copy, '$.title') END AS title,
                (snapshot IS NOT NULL AND snapshot != '') AS has_snapshot,
                CASE WHEN snapshot IS NOT NULL AND json_valid(snapshot)
                     THEN json_extract(snapshot, '$.revision') END AS snapshot_revision
         FROM plans
         ORDER BY updated_at DESC, id
         LIMIT ?`,
        cap
    ).map((row) => ({
        id: row.id,
        revision: row.revision,
        status: row.status,
        audience: row.audience,
        assigneeId: row.assignee_id == null ? null : row.assignee_id,
        updatedAt: row.updated_at,
        title: typeof row.title === 'string' ? row.title : null,
        hasSnapshot: row.has_snapshot === 1,
        snapshotRevision: Number.isInteger(row.snapshot_revision) ? row.snapshot_revision : null
    }));
}

module.exports = {
    LIST_MAX,
    loadWorkingCopy,
    saveWorkingCopy,
    loadSnapshot,
    saveSnapshot,
    listSnapshots,
    listPlans
};
