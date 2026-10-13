/**
 * Plans start in the MCP: create a draft plan, and list plans with their
 * publish state (ADR-0011 §7). Service layer; the adapters (MCP tools, admin
 * routes) call these two functions and edit no JSON.
 *
 * createPlan never overwrites. It makes the smallest document the schema
 * accepts as a draft, hidden from learners, through the same write log as every
 * other write (revision row, audit row, in one transaction).
 *
 * The scaffold is a shape and carries no generated Hebrew. The schema forces a
 * lesson to hold at least one block, and a block's text to be non-empty, so the
 * one block is a `fluency-pass` placeholder (`id: placeholder`) that holds no
 * text at all. The author inserts real blocks and then removes it. A `recipe`
 * only sets the lesson's recipe and moves the document to schemaVersion 1.1.0:
 * the recipe cards' blocks all need text or Hebrew the schema does not let an
 * empty shell hold.
 * Provenance: INIT-008/SPEC-007
 */
'use strict';

const fs = require('fs');
const db = require('../db');
const lexicon = require('../db/repos/lexicon');
const marksRepo = require('../db/repos/plan-field-marks');
const { validatePlan, SCHEMA_PATH, V100, V110 } = require('./validate');
const { refusesWrite, validationError } = require('./ops');
const { logWrite } = require('./write-log');
const { requireContext, isContext } = require('./context');

const DEFAULT_UNIT = 'unit-1';
const DEFAULT_LESSON = 'lesson-1';
const PLACEHOLDER_ID = 'placeholder';
const LIST_DEFAULT = 200;

let facts = null;

/**
 * The slug rule and the recipe list, read from the plan schema so there is no
 * second copy of either.
 * @returns {{ slug: RegExp, slugMax: number, recipes: string[] }}
 */
function schemaFacts() {
    if (facts) return facts;
    const defs = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8')).definitions;
    facts = {
        slug: new RegExp(defs.slug.pattern),
        slugMax: defs.slug.maxLength,
        recipes: defs.recipe.enum.slice()
    };
    return facts;
}

/**
 * @param {unknown} value
 * @returns {boolean}
 */
function isSlug(value) {
    const s = schemaFacts();
    return typeof value === 'string' && value.length >= 1 && value.length <= s.slugMax && s.slug.test(value);
}

/**
 * Create a draft plan at revision 1.
 * @param {object} store Plan store.
 * @param {{ id: string, title: string, recipe?: string, unitId?: string, lessonId?: string }} input
 * @param {object} ctx Write context from makeContext.
 * @returns {{ plan: object, document: string, revision: 1, status: 'draft', validation: object }
 *   |{ error: 'already_exists'|'invalid_id'|'invalid_recipe'|'validation' }}
 *   Nothing is written on an error.
 */
function createPlan(store, input, ctx) {
    requireContext(ctx);
    const body = input && typeof input === 'object' && !Array.isArray(input) ? input : {};
    const s = schemaFacts();

    if (!isSlug(body.id)) return { error: 'invalid_id' };
    const unitId = body.unitId == null ? DEFAULT_UNIT : body.unitId;
    const lessonId = body.lessonId == null ? DEFAULT_LESSON : body.lessonId;
    if (!isSlug(unitId)) return { error: 'invalid_id', field: 'unitId' };
    if (!isSlug(lessonId)) return { error: 'invalid_id', field: 'lessonId' };
    if (body.recipe != null && s.recipes.indexOf(body.recipe) === -1) return { error: 'invalid_recipe' };

    const lesson = {
        id: lessonId,
        title: body.title,
        level: 'basic',
        visibility: 'hidden',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{ id: PLACEHOLDER_ID, type: 'fluency-pass', seconds: 60, source: 'vocab' }]
    };
    if (body.recipe != null) lesson.recipe = body.recipe;
    const plan = {
        schemaVersion: body.recipe != null ? V110 : V100,
        id: body.id,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: body.title,
        language: { target: 'he', gloss: 'en' },
        units: [{ id: unitId, title: body.title, theme: body.title, lessons: [lesson] }]
    };

    const validation = validatePlan(plan, { lexicon });
    if (refusesWrite(validation)) return validationError(validation, undefined);

    const document = JSON.stringify(plan);
    let saved = null;
    db.transaction(() => {
        // Checked inside the transaction so two calls with one id cannot both write.
        if (store.loadWorkingCopy(plan.id)) return;
        saved = store.saveWorkingCopy({
            id: plan.id,
            document: document,
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        logWrite({
            planId: plan.id,
            before: null,
            after: plan,
            afterText: saved.document,
            op: 'create',
            path: ''
        }, ctx);
    });
    if (!saved) return { error: 'already_exists' };
    return {
        plan: JSON.parse(saved.document),
        document: saved.document,
        revision: 1,
        status: 'draft',
        validation: validation
    };
}

/**
 * List plans, most recently updated first.
 * `published` is whether a snapshot exists; `unpublishedChanges` is whether the
 * working copy is ahead of it (false for a plan that was never published).
 * An `ai` caller gets `assigned` (a boolean) and no `assigneeId`; an admin or a
 * script caller gets `assigneeId` too.
 * @param {object} store Plan store.
 * @param {{ limit?: number }} [opts] `limit` is at most 200.
 * @param {object} ctx
 * @returns {{ plans: object[], truncated: boolean }}
 */
function listPlans(store, opts, ctx) {
    requireContext(ctx);
    const o = opts && typeof opts === 'object' ? opts : {};
    const limit = Number.isInteger(o.limit) && o.limit >= 1 ? Math.min(o.limit, LIST_DEFAULT) : LIST_DEFAULT;
    const rows = store.listPlans(limit + 1);
    const counts = marksRepo.humanCounts();
    const names = isContext(ctx) && ctx.actor.kind !== 'ai';
    const plans = rows.slice(0, limit).map((row) => {
        const published = row.hasSnapshot;
        const item = {
            id: row.id,
            title: row.title,
            audience: row.audience,
            assigned: row.assigneeId != null,
            status: row.status,
            revision: row.revision,
            updatedAt: row.updatedAt,
            published: published,
            unpublishedChanges: published
                && (row.snapshotRevision == null || row.revision > row.snapshotRevision),
            humanEditedFields: counts.get(row.id) || 0
        };
        if (names) item.assigneeId = row.assigneeId;
        return item;
    });
    return { plans: plans, truncated: rows.length > limit };
}

module.exports = { createPlan, listPlans };
