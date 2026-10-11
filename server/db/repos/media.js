/**
 * Media asset rows. Bytes stay in the file pipeline; this module writes columns only.
 * The client upload name is not accepted and is not a column.
 * Prompt text is stored here and is not copied into audit_log.
 * Provenance: INIT-004/SPEC-006
 */
'use strict';

const db = require('../index');

const CLIENT_FILENAME_KEYS = [
    'filename',
    'original_name',
    'originalName',
    'original_filename',
    'originalFilename',
    'client_filename',
    'clientFilename'
];

/**
 * @param {object|null|undefined} row
 */
function rejectClientFilename(row) {
    if (row == null || typeof row !== 'object') return;
    for (const key of CLIENT_FILENAME_KEYS) {
        if (Object.prototype.hasOwnProperty.call(row, key)) {
            throw new TypeError('client filename is not stored');
        }
    }
}

/**
 * @param {string|null|undefined} value
 * @returns {string|null}
 */
function emptyToNull(value) {
    if (value == null) return null;
    if (typeof value !== 'string') {
        throw new TypeError('text fields must be strings');
    }
    return value.length === 0 ? null : value;
}

/**
 * @param {object|undefined} row
 * @returns {object|null}
 */
function mapRow(row) {
    if (!row) return null;
    return {
        id: row.id,
        storedName: row.stored_name,
        status: row.status,
        altText: row.alt_text,
        promptText: row.prompt_text,
        provider: row.provider,
        modelId: row.model_id,
        actor: row.actor,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

/**
 * @param {string} id
 * @returns {{
 *   id: string,
 *   storedName: string,
 *   status: string,
 *   altText: string,
 *   promptText: string|null,
 *   provider: string|null,
 *   modelId: string|null,
 *   actor: string,
 *   createdAt: string,
 *   updatedAt: string
 * }|null}
 */
function getById(id) {
    return mapRow(db.get(
        `SELECT id, stored_name, status, alt_text, prompt_text, provider,
                model_id, actor, created_at, updated_at
         FROM media WHERE id = ?`,
        id
    ));
}

/**
 * Insert one media row. Status defaults to pending. Alt text defaults to empty.
 * Does not write audit_log and does not touch the filesystem.
 * @param {{
 *   id: string,
 *   storedName: string,
 *   status?: string,
 *   altText?: string,
 *   promptText?: string|null,
 *   provider?: string|null,
 *   modelId?: string|null,
 *   actor: string,
 *   createdAt?: string,
 *   updatedAt?: string
 * }} row
 * @returns {object}
 */
function insert(row) {
    rejectClientFilename(row);
    if (row == null || typeof row !== 'object') {
        throw new TypeError('media row required');
    }
    const now = row.createdAt || new Date().toISOString();
    db.run(
        `INSERT INTO media (
            id, stored_name, status, alt_text, prompt_text, provider, model_id,
            actor, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        row.id,
        row.storedName,
        row.status || 'pending',
        row.altText == null ? '' : row.altText,
        emptyToNull(row.promptText),
        emptyToNull(row.provider),
        emptyToNull(row.modelId),
        row.actor,
        now,
        row.updatedAt || now
    );
    return getById(row.id);
}

/**
 * Update mutable columns on one media row. The id and the creating actor stay.
 * Omitted fields keep their stored values. Does not write audit_log.
 * @param {string} id
 * @param {{
 *   storedName?: string,
 *   status?: string,
 *   altText?: string,
 *   promptText?: string|null,
 *   provider?: string|null,
 *   modelId?: string|null
 * }} fields
 * @returns {object|null}
 */
function update(id, fields) {
    rejectClientFilename(fields);
    const current = getById(id);
    if (!current) return null;
    const patch = fields || {};
    const now = new Date().toISOString();
    db.run(
        `UPDATE media
         SET stored_name = ?, status = ?, alt_text = ?, prompt_text = ?,
             provider = ?, model_id = ?, updated_at = ?
         WHERE id = ?`,
        patch.storedName !== undefined ? patch.storedName : current.storedName,
        patch.status !== undefined ? patch.status : current.status,
        patch.altText !== undefined ? patch.altText : current.altText,
        patch.promptText !== undefined ? emptyToNull(patch.promptText) : current.promptText,
        patch.provider !== undefined ? emptyToNull(patch.provider) : current.provider,
        patch.modelId !== undefined ? emptyToNull(patch.modelId) : current.modelId,
        now,
        id
    );
    return getById(id);
}

module.exports = {
    getById,
    insert,
    update
};
