/**
 * Publish one plan: copy the working document onto the snapshot, set
 * status to published, and increment revision. Refuses when validation
 * reports any block, and then writes neither the snapshot nor the working copy.
 * Not a seventh edit operation. Adapters call this function; they do not
 * copy the snapshot themselves.
 * A lesson whose visibility is everyone or selected cannot be published
 * while it references a pending media row. A hidden lesson may.
 * Provenance: INIT-004/SPEC-003. Pending-media refusal: INIT-004/SPEC-011.
 * Lexicon passed to the validator for N13 and N14: INIT-006/SPEC-011
 */
'use strict';

const db = require('../db');
const lexicon = require('../db/repos/lexicon');
const mediaRepo = require('../db/repos/media');
const { validatePlan } = require('./validate');
const { openWorkingCopy } = require('./ops');

/**
 * Media ids on introduction words. Other block types have no image field.
 * @param {object} lesson
 * @returns {string[]}
 */
function mediaIdsOnLesson(lesson) {
    const ids = [];
    const blocks = lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
    for (const block of blocks) {
        const words = block && Array.isArray(block.words) ? block.words : [];
        for (const word of words) {
            const mediaId = word && word.image && word.image.mediaId;
            if (typeof mediaId === 'string' && mediaId) ids.push(mediaId);
        }
    }
    return ids;
}

/**
 * True when a lesson that learners can be granted (everyone or selected)
 * points at a media row that is still pending. Hidden lessons are skipped.
 * A missing row is not pending.
 * @param {object} plan
 * @returns {boolean}
 */
function referencesPendingMedia(plan) {
    const units = plan && Array.isArray(plan.units) ? plan.units : [];
    for (const unit of units) {
        const lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
        for (const lesson of lessons) {
            if (!lesson) continue;
            if (lesson.visibility !== 'everyone' && lesson.visibility !== 'selected') continue;
            for (const mediaId of mediaIdsOnLesson(lesson)) {
                const row = mediaRepo.getById(mediaId);
                if (row && row.status === 'pending') return true;
            }
        }
    }
    return false;
}

/**
 * @param {object} store Plan store from server/plans/store.js. saveSnapshot is required.
 * @param {string} id Plan id.
 * @param {{ expectedRevision: number }} input
 * @returns {object}
 */
function publishPlan(store, id, input) {
    if (!store || typeof store.saveSnapshot !== 'function') {
        throw new TypeError('store must provide saveSnapshot');
    }
    const opened = openWorkingCopy(store, id);
    if (opened.error) {
        const missing = { error: opened.error };
        if (opened.revision != null) missing.revision = opened.revision;
        return missing;
    }

    const expectedRevision = input && typeof input === 'object' ? input.expectedRevision : undefined;
    if (expectedRevision !== opened.row.revision) {
        return { error: 'revision_conflict', revision: opened.row.revision };
    }

    const next = JSON.parse(JSON.stringify(opened.plan));
    next.status = 'published';
    next.revision = opened.row.revision + 1;
    if (next.id !== opened.row.id) {
        return { error: 'invalid_op', revision: opened.row.revision };
    }

    const validation = validatePlan(next, { lexicon });
    if (!validation.ok) {
        return {
            error: 'validation',
            revision: opened.row.revision,
            blocks: validation.blocks,
            warnings: validation.warnings,
            informs: validation.informs
        };
    }
    if (referencesPendingMedia(next)) {
        return { error: 'pending_media', revision: opened.row.revision };
    }

    const document = JSON.stringify(next);
    db.transaction(() => {
        store.saveWorkingCopy({
            id: opened.row.id,
            document: document,
            revision: next.revision,
            status: next.status,
            audience: next.audience,
            assigneeId: next.assigneeId == null ? null : next.assigneeId
        });
        store.saveSnapshot(opened.row.id, document);
    });

    const saved = store.loadWorkingCopy(opened.row.id);
    return {
        plan: JSON.parse(saved.document),
        document: saved.document,
        snapshot: store.loadSnapshot(opened.row.id),
        revision: saved.revision,
        status: saved.status,
        validation: validation
    };
}

module.exports = { publishPlan };
