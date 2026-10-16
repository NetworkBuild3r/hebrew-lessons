/**
 * schemaVersion 1.2.0 guards and ops auto-raise (INIT-012/SPEC-002).
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { withCtx, testContext } = require('./helpers/plan-ctx');
const { validatePlan, SCHEMA_PATH } = require('../server/plans/validate');
const { SAMPLE_NAMES, VARIANTS, sample } = require('./fixtures/plans-1.1/variants-1.0.0');

const ROOT = path.resolve(__dirname, '..');
const EXAMPLES = JSON.parse(fs.readFileSync(
    path.join(ROOT, 'docs', 'decisions', 'ADR-0014-assets', 'exercise-examples.json'), 'utf8'
));
const EXERCISE = EXAMPLES.blocks.find((block) => block.type === 'exercise' && block.kind === 'choose');
const BASELINE = JSON.parse(fs.readFileSync(
    path.join(__dirname, 'fixtures', 'plans-1.1', 'samples-1.0.0-results.json'), 'utf8'
));
const SCHEMA = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));

const INIT_008_RULES = ['pointing-form', 'field-language', 'lexicon-form', 'pointing-consistency', 'audio-stale'];

function withoutInit008(result) {
    return Object.assign({}, result, {
        warnings: result.warnings.filter((item) => !INIT_008_RULES.includes(item.rule))
    });
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function freshModules(dataDir) {
    process.env.DATA_DIR = dataDir;
    delete require.cache[require.resolve('../server/db')];
    delete require.cache[require.resolve('../server/plans/store')];
    delete require.cache[require.resolve('../server/plans/ops')];
    delete require.cache[require.resolve('../server/plans/validate')];
    delete require.cache[require.resolve('../server/plans/revisions')];
    const db = require('../server/db');
    return {
        db,
        store: require('../server/plans/store'),
        ops: require('../server/plans/ops'),
        revisions: require('../server/plans/revisions')
    };
}

describe('INIT-012/SPEC-002: schemaVersion 1.2.0', () => {
    it('ac-10: ADR-0003 sample plans stay 1.0.0 with the same validator outcome', () => {
        assert.deepEqual(Object.keys(BASELINE.samples).sort(), SAMPLE_NAMES.slice().sort());
        SAMPLE_NAMES.forEach((name) => {
            const plan = sample(name);
            assert.equal(plan.schemaVersion, '1.0.0', name);
            assert.deepEqual(withoutInit008(validatePlan(plan)), BASELINE.samples[name], name);
        });
    });

    it('schema lists 1.2.0 and 1.2.0 documents may use 1.1.0 blocks', () => {
        assert.deepEqual(SCHEMA.properties.schemaVersion.enum, ['1.0.0', '1.1.0', '1.2.0']);
        const plan = JSON.parse(fs.readFileSync(
            path.join(__dirname, 'fixtures', 'plans-1.1', 'personal-question.plan.json'), 'utf8'
        ));
        plan.schemaVersion = '1.2.0';
        plan.units[0].lessons[0].blocks.push(clone(EXERCISE));
        const result = validatePlan(plan);
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
    });
});

describe('ac-3: ops.js raises to 1.2.0 and never lowers', { concurrency: false }, () => {
    const saved = {};
    let tmp;
    let db;
    let store;
    let ops;
    let revisions;

    before(() => {
        ['DATA_DIR', 'LESSONS_DIR', 'HEBREW_ROOT'].forEach((name) => { saved[name] = process.env[name]; });
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-1-2-'));
        process.env.LESSONS_DIR = path.join(tmp, 'lessons');
        fs.mkdirSync(process.env.LESSONS_DIR);
        const modules = freshModules(path.join(tmp, 'data'));
        ({ db, store, ops, revisions } = modules);
        ops = withCtx(ops);
        db.open();
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        Object.keys(saved).forEach((name) => {
            if (saved[name] === undefined) delete process.env[name];
            else process.env[name] = saved[name];
        });
    });

    beforeEach(() => {
        db.run('DELETE FROM plans');
    });

    function seed(plan) {
        store.saveWorkingCopy({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return plan;
    }

    it('insertBlock of exercise into a 1.1.0 plan sets schemaVersion 1.2.0 in the same revision', () => {
        const before = seed(JSON.parse(fs.readFileSync(
            path.join(__dirname, 'fixtures', 'plans-1.1', 'personal-question.plan.json'), 'utf8'
        )));
        assert.equal(before.schemaVersion, '1.1.0');
        const block = clone(EXERCISE);
        const result = ops.insertBlock(store, before.id, {
            path: 'units/family/lessons/my-family',
            block,
            expectedRevision: 1
        });
        assert.equal(result.error, undefined, JSON.stringify(result));
        assert.equal(result.plan.schemaVersion, '1.2.0');
        assert.equal(result.revision, 2);
        const expected = clone(before);
        expected.units[0].lessons[0].blocks.push(block);
        expected.revision = 2;
        expected.status = 'draft';
        expected.schemaVersion = '1.2.0';
        assert.deepEqual(result.plan, expected);
        assert.deepEqual(result.validation.blocks, []);
    });

    it('restoring a pre-1.2.0 revision onto a 1.2.0 plan keeps schemaVersion 1.2.0', () => {
        const ctx = testContext('plan-1-2-restore');
        const before = seed(JSON.parse(fs.readFileSync(
            path.join(__dirname, 'fixtures', 'plans-1.1', 'personal-question.plan.json'), 'utf8'
        )));
        const block = clone(EXERCISE);
        const raised = ops.insertBlock(store, before.id, {
            path: 'units/family/lessons/my-family',
            block,
            expectedRevision: 1
        });
        assert.equal(raised.error, undefined, JSON.stringify(raised));
        assert.equal(raised.plan.schemaVersion, '1.2.0');
        assert.equal(revisions.get(before.id, 1, ctx).document.schemaVersion, '1.1.0');
        const restored = revisions.restore(store, before.id, {
            fromRevision: 1,
            expectedRevision: raised.revision
        }, ctx);
        assert.equal(restored.error, undefined, JSON.stringify(restored));
        const now = JSON.parse(store.loadWorkingCopy(before.id).document);
        assert.equal(now.schemaVersion, '1.2.0');
        assert.equal(now.units[0].lessons[0].blocks.some((b) => b.type === 'exercise'), false);
    });

    it('setField cannot lower schemaVersion from 1.2.0 to 1.1.0', () => {
        const plan = seed(JSON.parse(fs.readFileSync(
            path.join(__dirname, 'fixtures', 'plans-1.1', 'personal-question.plan.json'), 'utf8'
        )));
        plan.schemaVersion = '1.2.0';
        store.saveWorkingCopy({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: 2,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        const result = ops.setField(store, plan.id, {
            path: '',
            field: 'schemaVersion',
            value: '1.1.0',
            expectedRevision: 2
        });
        assert.equal(result.error, 'invalid_op');
    });
});
