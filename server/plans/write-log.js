/**
 * Everything a successful plan write records besides the working copy: the
 * revision row (with its lazy baseline), the provenance marks, and the plan.*
 * audit row. One function, so every writer (the six operations, publish,
 * create, restore) logs the same way and none can skip a part.
 *
 * Call it inside the same db.transaction as the working-copy write. Any part
 * that throws rolls the whole write back: no mutation without its row, and no
 * row without its mutation (ADR-0011 §1, §2, §4). It returns the advisory
 * `overwrote` list for the caller to put on the write result.
 * Provenance: INIT-008/SPEC-004, SPEC-006
 */
'use strict';

const revisions = require('./revisions');
const provenance = require('./provenance');
const planAudit = require('./audit');
const { requireContext } = require('./context');

/**
 * @param {{
 *   planId: string,
 *   before: object|null,
 *   after: object,
 *   afterText: string,
 *   op: string,
 *   path?: string,
 *   field?: string,
 *   summary?: string
 * }} write `before` is the plan before the write (null for create), `after` the
 *   plan as stored, `afterText` the stored JSON text.
 * @param {object} ctx Context from makeContext.
 * @returns {{ overwrote: object[] }}
 */
function logWrite(write, ctx) {
    requireContext(ctx);
    revisions.record(write, ctx);

    let overwrote = [];
    if (write.before) {
        const changes = provenance.changesOf(write.before, write.after);
        overwrote = provenance.overwrittenBy(write.planId, changes, ctx);
        provenance.recordWrite(write.planId, write.before, write.after, ctx, write.after.revision, changes);
    }

    planAudit.record(ctx, 'plan.' + write.op, write.planId, {
        revision: write.after.revision,
        path: write.path || '',
        field: write.field
    });
    return { overwrote: overwrote };
}

module.exports = { logWrite };
