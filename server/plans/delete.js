/**
 * Delete a plan: the working copy, the published snapshot, and the history
 * rows that belong to it. A worksheet lesson with the same id is not touched.
 */
'use strict';

const db = require('../db');
const { openWorkingCopy } = require('./ops');
const { record } = require('./audit');
const { requireContext } = require('./context');

/**
 * @param {object} store Plan store.
 * @param {string} id
 * @param {{ expectedRevision: number }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {{ id: string, deleted: true }|{ error: string, revision?: number, field?: string }}
 */
function deletePlan(store, id, input, ctx) {
    requireContext(ctx);
    const body = input && typeof input === 'object' && !Array.isArray(input) ? input : {};
    if (!Number.isInteger(body.expectedRevision)) {
        return { error: 'invalid_op', field: 'expectedRevision' };
    }
    const opened = openWorkingCopy(store, id);
    if (opened.error) return opened;
    if (opened.row.revision !== body.expectedRevision) {
        return { error: 'revision_conflict', revision: opened.row.revision };
    }
    let deleted = false;
    db.transaction(() => {
        const current = store.loadWorkingCopy(id);
        if (!current || current.revision !== body.expectedRevision) return;
        const result = db.run('DELETE FROM plans WHERE id = ?', id);
        if (!result.changes) return;
        record(ctx, 'plan.delete', id, { revision: current.revision });
        deleted = true;
    });
    if (!deleted) {
        const again = store.loadWorkingCopy(id);
        if (!again) return { error: 'not_found' };
        return { error: 'revision_conflict', revision: again.revision };
    }
    return { id: id, deleted: true };
}

module.exports = { deletePlan };
