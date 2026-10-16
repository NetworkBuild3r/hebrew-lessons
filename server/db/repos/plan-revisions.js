/**
 * plan_revisions repository: SQL only, prepared statements only.
 * The log is append-only. This module has an insert and reads; nothing here
 * changes or removes a row (ADR-0011 §2).
 * A document is gzipped on write with `document_sha256` of its UTF-8 text, and
 * inflated and checked on read. A row whose hash no longer matches is reported
 * as `revision_corrupt`, never returned.
 * Provenance: INIT-008/SPEC-004
 */
'use strict';

const zlib = require('node:zlib');
const crypto = require('node:crypto');
const db = require('../index');

const META_COLUMNS = `plan_id, revision, actor_kind, actor_name, tool, client, request_id,
    op, path, field, summary, created_at`;

/**
 * @param {string} text
 * @returns {string} Lowercase hex SHA-256 of the UTF-8 bytes.
 */
function sha256(text) {
    return crypto.createHash('sha256').update(text, 'utf8').digest('hex');
}

/**
 * @param {object} row
 * @returns {object}
 */
function mapMeta(row) {
    return {
        planId: row.plan_id,
        revision: row.revision,
        op: row.op,
        path: row.path == null ? '' : row.path,
        field: row.field == null ? '' : row.field,
        summary: row.summary,
        actorKind: row.actor_kind,
        actorName: row.actor_name,
        tool: row.tool == null ? null : row.tool,
        client: row.client == null ? null : row.client,
        requestId: row.request_id == null ? null : row.request_id,
        at: row.created_at
    };
}

/**
 * Insert one revision row. Throws when the insert fails (a duplicate
 * `(plan_id, revision)`, a CHECK, or a missing plan), so the caller's
 * transaction rolls back.
 * @param {{
 *   planId: string, revision: number, actorKind: string, actorName: string,
 *   tool?: string|null, client?: string|null, requestId?: string|null,
 *   op: string, path?: string, field?: string, summary: string,
 *   document: string, createdAt?: string
 * }} row
 */
function append(row) {
    if (typeof row.document !== 'string' || row.document.length === 0) {
        throw new TypeError('document must be a non-empty JSON string');
    }
    const gz = zlib.gzipSync(Buffer.from(row.document, 'utf8'));
    db.run(
        `INSERT INTO plan_revisions (
            plan_id, revision, actor_kind, actor_name, tool, client, request_id,
            op, path, field, summary, document_gz, document_sha256, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        row.planId,
        row.revision,
        row.actorKind,
        row.actorName,
        row.tool == null ? null : row.tool,
        row.client == null ? null : row.client,
        row.requestId == null ? null : row.requestId,
        row.op,
        row.path == null ? '' : row.path,
        row.field == null ? '' : row.field,
        row.summary,
        gz,
        sha256(row.document),
        row.createdAt || new Date().toISOString()
    );
}

/**
 * @param {string} planId
 * @param {number} revision
 * @returns {boolean}
 */
function has(planId, revision) {
    return !!db.get(
        'SELECT 1 AS ok FROM plan_revisions WHERE plan_id = ? AND revision = ?',
        planId,
        revision
    );
}

/**
 * One revision with its document text.
 * @param {string} planId
 * @param {number} revision
 * @returns {object|null|{ error: 'revision_corrupt' }} Metadata plus `document`
 *   (JSON text); null when there is no such row.
 */
function get(planId, revision) {
    const row = db.get(
        `SELECT ${META_COLUMNS}, document_gz, document_sha256
         FROM plan_revisions WHERE plan_id = ? AND revision = ?`,
        planId,
        revision
    );
    if (!row) return null;
    let text;
    try {
        text = zlib.gunzipSync(row.document_gz).toString('utf8');
    } catch (err) {
        return { error: 'revision_corrupt' };
    }
    if (sha256(text) !== row.document_sha256) return { error: 'revision_corrupt' };
    const out = mapMeta(row);
    out.document = text;
    return out;
}

/**
 * Metadata rows, newest first. No document is read.
 * `pathPrefix` is compared with substr(), so a `_` or `%` in an id is never a
 * wildcard.
 * @param {string} planId
 * @param {{
 *   sinceRevision?: number|null, beforeRevision?: number|null,
 *   actorKind?: string|null, pathPrefix?: string|null, limit: number
 * }} opts
 * @returns {object[]}
 */
function list(planId, opts) {
    const since = opts.sinceRevision == null ? null : opts.sinceRevision;
    const before = opts.beforeRevision == null ? null : opts.beforeRevision;
    const kind = opts.actorKind == null ? null : opts.actorKind;
    const prefix = opts.pathPrefix == null ? null : opts.pathPrefix;
    return db.all(
        `SELECT ${META_COLUMNS}
         FROM plan_revisions
         WHERE plan_id = ?
           AND (? IS NULL OR revision > ?)
           AND (? IS NULL OR revision < ?)
           AND (? IS NULL OR actor_kind = ?)
           AND (? IS NULL OR substr(path, 1, length(?)) = ?)
         ORDER BY revision DESC
         LIMIT ?`,
        planId,
        since, since,
        before, before,
        kind, kind,
        prefix, prefix, prefix,
        opts.limit
    ).map(mapMeta);
}

/**
 * Metadata of the highest-numbered revision.
 * @param {string} planId
 * @returns {object|null}
 */
function latest(planId) {
    const row = db.get(
        `SELECT ${META_COLUMNS}
         FROM plan_revisions WHERE plan_id = ?
         ORDER BY revision DESC LIMIT 1`,
        planId
    );
    return row ? mapMeta(row) : null;
}

module.exports = { append, has, get, list, latest, sha256 };
