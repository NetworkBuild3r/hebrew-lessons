/**
 * createPlan and listPlans (INIT-008/SPEC-007). Temporary database only.
 * No network beyond the in-process HTTP server, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-007
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { planEnv } = require('./helpers/plan-db');
const harness = require('./helpers/signed-in-server');

const RECIPES = [
    'topic-conversation',
    'review-worksheet',
    'verse-reading',
    'verb-workshop',
    'letters-and-vowels',
    'guide'
];

describe('createPlan and listPlans (INIT-008/SPEC-007)', { concurrency: false }, () => {
    let env;
    let validate;

    before(() => {
        env = planEnv('hebrew-plan-create-');
        validate = require('../server/plans/validate');
    });

    after(() => {
        env.close();
    });

    beforeEach(() => {
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM audit_log');
    });

    function snapshotTables() {
        return JSON.stringify({
            plans: env.db.all('SELECT id, revision, working_copy FROM plans ORDER BY id'),
            revisions: env.db.all('SELECT plan_id, revision, op FROM plan_revisions ORDER BY plan_id, revision'),
            audit: env.db.all('SELECT actor, action, target FROM audit_log ORDER BY id'),
            marks: env.db.all('SELECT plan_id, path, field FROM plan_field_marks ORDER BY plan_id, path, field')
        });
    }

    it('ac-1: a valid id and title store a hidden draft at revision 1, logged and audited', () => {
        const result = env.create.createPlan(env.store, { id: 'my-new-plan', title: 'My new plan' }, env.ai('create_plan'));
        assert.equal(result.error, undefined, JSON.stringify(result));
        assert.equal(result.revision, 1);
        assert.equal(result.status, 'draft');

        const row = env.store.loadWorkingCopy('my-new-plan');
        assert.equal(row.revision, 1);
        assert.equal(row.status, 'draft');
        assert.equal(row.audience, 'shared');
        assert.equal(row.assigneeId, null);
        assert.equal(row.snapshot, null);

        const plan = JSON.parse(row.document);
        assert.equal(plan.schemaVersion, '1.0.0');
        assert.equal(plan.units.length, 1);
        assert.equal(plan.units[0].lessons.length, 1);
        assert.equal(plan.units[0].lessons[0].visibility, 'hidden');
        assert.deepEqual(plan.units[0].lessons[0].buildsOn, []);
        assert.deepEqual(plan.units[0].lessons[0].newWords, []);
        assert.deepEqual(plan.units[0].lessons[0].recycles, []);
        assert.equal(validate.validatePlan(plan, {}).blocks.length, 0);
        assert.equal(result.validation.blocks.length, 0);
        assert.deepEqual(result.plan, plan);

        const revisions = env.db.all('SELECT * FROM plan_revisions WHERE plan_id = ?', 'my-new-plan');
        assert.equal(revisions.length, 1);
        assert.equal(revisions[0].op, 'create');
        assert.equal(revisions[0].revision, 1);
        assert.equal(revisions[0].actor_kind, 'ai');
        assert.equal(revisions[0].tool, 'create_plan');
        assert.equal(env.revisionsRepo.get('my-new-plan', 1).document, row.document);

        const audit = env.db.all('SELECT * FROM audit_log');
        assert.equal(audit.length, 1);
        assert.equal(audit[0].action, 'plan.create');
        assert.equal(audit[0].actor, 'mcp');
        assert.equal(audit[0].target, 'my-new-plan');
        assert.equal(env.count('plan_field_marks'), 0);

        const read = env.ops.readPart(env.store, 'my-new-plan', { path: '' }, env.ai('read_part'));
        assert.deepEqual(read.part, plan);
        assert.equal(read.revision, 1);
    });

    it('ac-1: an admin creates the plan under their own name', () => {
        env.create.createPlan(env.store, { id: 'by-admin', title: 'By admin' }, env.human('create_plan'));
        assert.equal(env.db.get('SELECT actor FROM audit_log').actor, 'admin:example-admin');
        assert.equal(env.db.get('SELECT actor_name FROM plan_revisions').actor_name, 'example-admin');
    });

    it('ac-1: the first edit of a created plan adds one row, not a second baseline', () => {
        const created = env.create.createPlan(env.store, { id: 'edited-once', title: 'Edited once' }, env.ai('create_plan'));
        const edit = env.ops.setField(env.store, 'edited-once', {
            path: '', field: 'title', value: 'Edited', expectedRevision: created.revision
        }, env.ai('set_field'));
        assert.equal(edit.error, undefined);
        assert.deepEqual(
            env.db.all('SELECT revision, op FROM plan_revisions ORDER BY revision').map((r) => r.op),
            ['create', 'set_field']
        );
    });

    it('ac-2: an existing id returns already_exists and changes nothing', () => {
        env.create.createPlan(env.store, { id: 'taken-plan', title: 'Taken' }, env.ai('create_plan'));
        env.ops.setField(env.store, 'taken-plan', {
            path: '', field: 'title', value: 'Edited', expectedRevision: 1
        }, env.ai('set_field'));
        const before = snapshotTables();
        const again = env.create.createPlan(env.store, { id: 'taken-plan', title: 'Overwrite' }, env.ai('create_plan'));
        assert.deepEqual(again, { error: 'already_exists' });
        assert.equal(snapshotTables(), before);

        const seeded = env.seed();
        const beforeSeed = snapshotTables();
        assert.equal(env.create.createPlan(env.store, { id: seeded.id, title: 'No' }, env.human()).error, 'already_exists');
        assert.equal(snapshotTables(), beforeSeed);
    });

    it('ac-2: a bad id, recipe, unit id, lesson id, or title writes nothing', () => {
        const before = snapshotTables();
        const ctx = env.ai('create_plan');
        const cases = [
            [{ id: 'Not_A_Slug', title: 'x' }, 'invalid_id'],
            [{ id: 'UPPER', title: 'x' }, 'invalid_id'],
            [{ id: 'double--hyphen', title: 'x' }, 'invalid_id'],
            [{ id: '', title: 'x' }, 'invalid_id'],
            [{ id: 'a'.repeat(65), title: 'x' }, 'invalid_id'],
            [{ title: 'x' }, 'invalid_id'],
            [{ id: 42, title: 'x' }, 'invalid_id'],
            [{ id: 'good-id', title: 'x', unitId: 'Bad Unit' }, 'invalid_id'],
            [{ id: 'good-id', title: 'x', lessonId: '../up' }, 'invalid_id'],
            [{ id: 'good-id', title: 'x', recipe: 'free-form' }, 'invalid_recipe'],
            [{ id: 'good-id', title: 'x'.repeat(4001) }, 'validation'],
            [{ id: 'good-id', title: '<b>bold</b>' }, 'validation'],
            [{ id: 'good-id', title: '' }, 'validation'],
            [{ id: 'good-id' }, 'validation'],
            [{ id: 'good-id', title: 7 }, 'validation'],
            [null, 'invalid_id']
        ];
        for (const [input, error] of cases) {
            const result = env.create.createPlan(env.store, input, ctx);
            assert.equal(result.error, error, JSON.stringify(input) + ' ' + JSON.stringify(result).slice(0, 200));
        }
        assert.equal(snapshotTables(), before);
        assert.throws(() => env.create.createPlan(env.store, { id: 'good-id', title: 'x' }, undefined), TypeError);
        assert.equal(snapshotTables(), before);
    });

    it('ac-3: each of the six recipes sets the recipe and schemaVersion 1.1.0, with no block finding', () => {
        for (const recipe of RECIPES) {
            const result = env.create.createPlan(env.store, { id: 'recipe-' + recipe, title: 'Recipe ' + recipe, recipe: recipe }, env.ai('create_plan'));
            assert.equal(result.error, undefined, recipe + ' ' + JSON.stringify(result).slice(0, 300));
            const plan = JSON.parse(env.store.loadWorkingCopy('recipe-' + recipe).document);
            assert.equal(plan.schemaVersion, '1.1.0', recipe);
            assert.equal(plan.units[0].lessons[0].recipe, recipe);
            assert.equal(validate.validatePlan(plan, {}).blocks.length, 0, recipe);
            assert.equal(result.validation.blocks.length, 0, recipe);
        }
        const plain = env.create.createPlan(env.store, { id: 'no-recipe', title: 'No recipe' }, env.ai('create_plan'));
        assert.equal(plain.plan.schemaVersion, '1.0.0');
        assert.equal(Object.prototype.hasOwnProperty.call(plain.plan.units[0].lessons[0], 'recipe'), false);
    });

    it('ac-3: custom unit and lesson ids are used; the recipe list is the schema list', () => {
        const result = env.create.createPlan(env.store, {
            id: 'custom-ids', title: 'Custom', unitId: 'greetings', lessonId: 'hello-1'
        }, env.ai('create_plan'));
        assert.equal(result.error, undefined);
        assert.equal(result.plan.units[0].id, 'greetings');
        assert.equal(result.plan.units[0].lessons[0].id, 'hello-1');
        assert.equal(
            env.ops.readPart(env.store, 'custom-ids', { path: 'units/greetings/lessons/hello-1' }).part.id,
            'hello-1'
        );
        const schema = JSON.parse(fs.readFileSync(validate.SCHEMA_PATH, 'utf8'));
        assert.deepEqual(schema.definitions.recipe.enum, RECIPES);
    });

    it('ac-3: the placeholder can be replaced: insert a real block, then remove the placeholder', () => {
        env.create.createPlan(env.store, { id: 'fill-it', title: 'Fill it' }, env.ai('create_plan'));
        const lesson = 'units/unit-1/lessons/lesson-1';
        const removeFirst = env.ops.removeBlock(env.store, 'fill-it', {
            path: lesson + '/blocks/placeholder', expectedRevision: 1
        }, env.ai('remove_block'));
        assert.equal(removeFirst.error, 'validation');
        const insert = env.ops.insertBlock(env.store, 'fill-it', {
            path: lesson, block: { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' }, expectedRevision: 1
        }, env.ai('insert_block'));
        assert.equal(insert.error, undefined);
        const remove = env.ops.removeBlock(env.store, 'fill-it', {
            path: lesson + '/blocks/placeholder', expectedRevision: insert.revision
        }, env.ai('remove_block'));
        assert.equal(remove.error, undefined, JSON.stringify(remove));
        assert.deepEqual(remove.plan.units[0].lessons[0].blocks.map((b) => b.id), ['listen']);
    });

    describe('listPlans', () => {
        function plainDoc(id, title, extra) {
            return Object.assign({
                schemaVersion: '1.0.0', id: id, revision: 1, status: 'draft', audience: 'shared', assigneeId: null,
                title: title, language: { target: 'he', gloss: 'en' }, units: []
            }, extra || {});
        }

        it('ac-4: one row each for a created, a published-then-edited, and a personal plan', () => {
            const ctx = env.human('seed');
            env.create.createPlan(env.store, { id: 'created-plan', title: 'Created plan' }, env.ai('create_plan'));

            env.seed(env.samplePlan((plan) => { plan.id = 'published-plan'; }));
            const published = env.publish.publishPlan(env.store, 'published-plan', { expectedRevision: 1 }, ctx);
            assert.equal(published.error, undefined, JSON.stringify(published.blocks));
            const edited = env.ops.setField(env.store, 'published-plan', {
                path: '', field: 'title', value: 'Edited after publish', expectedRevision: published.revision
            }, ctx);
            assert.equal(edited.error, undefined);

            env.seed(env.samplePlan((plan) => {
                plan.id = 'personal-plan';
                plan.audience = 'personal';
                plan.assigneeId = 'example-learner';
            }));

            const admin = env.create.listPlans(env.store, {}, env.human('list_plans'));
            assert.equal(admin.truncated, false);
            assert.equal(admin.plans.length, 3);
            const byId = Object.fromEntries(admin.plans.map((p) => [p.id, p]));

            assert.equal(byId['created-plan'].published, false);
            assert.equal(byId['created-plan'].unpublishedChanges, false);
            assert.equal(byId['created-plan'].revision, 1);
            assert.equal(byId['created-plan'].status, 'draft');
            assert.equal(byId['created-plan'].title, 'Created plan');
            assert.equal(byId['created-plan'].audience, 'shared');
            assert.equal(byId['created-plan'].assigned, false);

            assert.equal(byId['published-plan'].published, true);
            assert.equal(byId['published-plan'].unpublishedChanges, true);
            assert.equal(byId['published-plan'].revision, 3);
            assert.equal(byId['published-plan'].status, 'draft');
            assert.equal(byId['published-plan'].title, 'Edited after publish');

            assert.equal(byId['personal-plan'].audience, 'personal');
            assert.equal(byId['personal-plan'].assigned, true);
            assert.equal(byId['personal-plan'].published, false);

            const justPublished = env.publish.publishPlan(env.store, 'published-plan', { expectedRevision: 3 }, ctx);
            assert.equal(justPublished.error, undefined);
            const after = env.create.listPlans(env.store, {}, env.human()).plans.find((p) => p.id === 'published-plan');
            assert.equal(after.published, true);
            assert.equal(after.unpublishedChanges, false);
            assert.equal(after.status, 'published');
        });

        it('ac-4: an ai caller gets no assigneeId and a boolean; an admin gets assigneeId', () => {
            env.seed(env.samplePlan((plan) => {
                plan.id = 'personal-plan';
                plan.audience = 'personal';
                plan.assigneeId = 'example-learner';
            }));
            env.create.createPlan(env.store, { id: 'shared-plan', title: 'Shared' }, env.ai('create_plan'));

            const ai = env.create.listPlans(env.store, {}, env.ai('list_plans'));
            for (const row of ai.plans) {
                assert.equal(Object.prototype.hasOwnProperty.call(row, 'assigneeId'), false, row.id);
            }
            assert.equal(ai.plans.find((p) => p.id === 'personal-plan').assigned, true);
            assert.equal(ai.plans.find((p) => p.id === 'shared-plan').assigned, false);
            assert.equal(JSON.stringify(ai).includes('example-learner'), false);

            const admin = env.create.listPlans(env.store, {}, env.human('list_plans'));
            assert.equal(admin.plans.find((p) => p.id === 'personal-plan').assigneeId, 'example-learner');
            assert.equal(admin.plans.find((p) => p.id === 'shared-plan').assigneeId, null);
        });

        it('ac-4: the list is capped at 200 with truncated, newest first, and honors limit', () => {
            for (let i = 0; i < 201; i += 1) {
                const id = 'bulk-' + String(i).padStart(3, '0');
                env.store.saveWorkingCopy({
                    id: id, document: JSON.stringify(plainDoc(id, 'Bulk ' + i)), revision: 1, status: 'draft',
                    audience: 'shared', assigneeId: null
                });
            }
            env.db.run('UPDATE plans SET updated_at = ? WHERE id = ?', '2099-01-01T00:00:00.000Z', 'bulk-005');
            const result = env.create.listPlans(env.store, {}, env.human());
            assert.equal(result.plans.length, 200);
            assert.equal(result.truncated, true);
            assert.equal(result.plans[0].id, 'bulk-005');
            const few = env.create.listPlans(env.store, { limit: 3 }, env.human());
            assert.equal(few.plans.length, 3);
            assert.equal(few.truncated, true);
            const huge = env.create.listPlans(env.store, { limit: 9999 }, env.human());
            assert.equal(huge.plans.length, 200);
        });

        it('ac-4: a stored document that is not JSON still lists, with a null title', () => {
            env.store.saveWorkingCopy({
                id: 'broken-plan', document: 'not json at all', revision: 1, status: 'draft',
                audience: 'shared', assigneeId: null
            });
            env.create.createPlan(env.store, { id: 'fine-plan', title: 'Fine' }, env.ai());
            const result = env.create.listPlans(env.store, {}, env.human());
            assert.equal(result.plans.find((p) => p.id === 'broken-plan').title, null);
            assert.equal(result.plans.find((p) => p.id === 'fine-plan').title, 'Fine');
        });

        it('ac-5: listPlans runs no SQL outside store.js and reads no learner table', () => {
            const createSource = fs.readFileSync(path.join(env.root, 'server', 'plans', 'create.js'), 'utf8');
            assert.equal(/\b(SELECT|INSERT|UPDATE|DELETE)\s/.test(createSource), false);
            assert.equal(/\bdb\.(get|all|run|prepare)\(/.test(createSource), false);

            const storeSource = fs.readFileSync(path.join(env.root, 'server', 'plans', 'store.js'), 'utf8');
            const start = storeSource.indexOf('function listPlans(');
            const body = storeSource.slice(start, storeSource.indexOf('module.exports', start));
            const tables = Array.from(body.matchAll(/\bFROM\s+(\w+)/g)).map((m) => m[1]);
            assert.deepEqual(tables, ['plans']);

            // Every statement a listing runs reads plans and the marks table, nothing else.
            const seen = [];
            const original = env.db.all;
            env.db.all = function (sql, ...params) {
                seen.push(String(sql));
                return original.call(env.db, sql, ...params);
            };
            try {
                env.create.listPlans(env.store, {}, env.human());
            } finally {
                env.db.all = original;
            }
            const read = seen.join(' ').match(/\bFROM\s+(\w+)/g).map((s) => s.replace(/FROM\s+/, ''));
            assert.deepEqual(read.sort(), ['plan_field_marks', 'plans']);
        });
    });
});

describe('a created plan is not readable by a learner (INIT-008/SPEC-007 ac-6)', { concurrency: false }, () => {
    let baseUrl;
    let cookieFor;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
    });

    after(async () => {
        await harness.stop();
    });

    async function read(id, who) {
        const res = await fetch(baseUrl + '/api/plans/' + id + '/published', { headers: { Cookie: cookieFor(who) } });
        return { status: res.status, text: await res.text() };
    }

    it('answers 404 for a new draft, and for a published plan whose lesson is still hidden, until it is exposed', async () => {
        const store = require('../server/plans/store');
        const ops = require('../server/plans/ops');
        const publish = require('../server/plans/publish');
        const create = require('../server/plans/create');
        const { makeContext } = require('../server/plans/context');
        const ctx = makeContext({ channel: 'mcp', tool: 'test' });

        const created = create.createPlan(store, { id: 'fresh-plan', title: 'Fresh plan' }, ctx);
        assert.equal(created.error, undefined);
        for (const who of ['learner', 'admin']) {
            const draft = await read('fresh-plan', who);
            assert.equal(draft.status, 404, who);
            assert.equal(draft.text.includes('Fresh plan'), false);
        }

        const published = publish.publishPlan(store, 'fresh-plan', { expectedRevision: 1 }, ctx);
        assert.equal(published.error, undefined, JSON.stringify(published.blocks));
        assert.equal((await read('fresh-plan', 'learner')).status, 404);

        const exposed = ops.setField(store, 'fresh-plan', {
            path: 'units/unit-1/lessons/lesson-1', field: 'visibility', value: 'everyone',
            expectedRevision: published.revision
        }, ctx);
        assert.equal(exposed.error, undefined);
        assert.equal((await read('fresh-plan', 'learner')).status, 404);
        const again = publish.publishPlan(store, 'fresh-plan', { expectedRevision: exposed.revision }, ctx);
        assert.equal(again.error, undefined);
        const seen = await read('fresh-plan', 'learner');
        assert.equal(seen.status, 200);
        assert.equal(JSON.parse(seen.text).status, 'published');
    });
});
