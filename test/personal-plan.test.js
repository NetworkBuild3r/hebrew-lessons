/**
 * Personal-plan assignee checks, on a temporary database.
 * Provenance: INIT-004/SPEC-010
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { testContext } = require('./helpers/plan-ctx');

const root = path.resolve(__dirname, '..');
const SAMPLE = path.join(
    root,
    'docs',
    'decisions',
    'ADR-0003-assets',
    'samples',
    'beginner-unit-1.plan.json'
);
const pretty = fs.readFileSync(SAMPLE, 'utf8');
const PLAN_ID = 'beginner-unit-1';

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const modules = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/users',
        '../server/plans/store',
        '../server/plans/validate',
        '../server/plans/ops',
        '../server/plans/personal'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    const store = require('../server/plans/store');
    const ops = require('../server/plans/ops');
    const personal = require('../server/plans/personal');
    const users = require('../server/db/repos/users');
    const { validatePlan } = require('../server/plans/validate');
    return { db, store, ops, personal, users, validatePlan };
}

describe('personal plan assignee checks', { concurrency: false }, () => {
    let tmp;
    let db;
    let store;
    let ops;
    let personal;
    let users;
    let validatePlan;
    const savedLessons = process.env.LESSONS_DIR;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-personal-plan-'));
        process.env.LESSONS_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-personal-plan-lessons-'));
        ({ db, store, ops, personal, users, validatePlan } = freshModules(tmp));
        // INIT-008/SPEC-002: assign takes a write context; these calls add one and nothing else.
        const rawPersonal = personal;
        const writeCtx = testContext('personal-plan-test');
        personal = { assign: (planStore, id, input) => rawPersonal.assign(planStore, id, input, writeCtx) };
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        if (process.env.LESSONS_DIR) fs.rmSync(process.env.LESSONS_DIR, { recursive: true, force: true });
        if (savedLessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = savedLessons;
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    beforeEach(() => {
        db.run('DELETE FROM plans');
        db.run('DELETE FROM users');
    });

    function seed() {
        const plan = JSON.parse(pretty);
        store.saveWorkingCopy({
            id: plan.id,
            document: pretty,
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return store.loadWorkingCopy(plan.id);
    }

    function createAccount(username, status) {
        users.create({
            username: username,
            password_hash: 'not-used',
            display_name: 'Hidden Name',
            role: 'user',
            status: status || 'active'
        });
    }

    function withSpies(fn) {
        const calls = { assignPlan: 0, replacePart: 0, setField: 0, runs: [] };
        const originalAssign = ops.assignPlan;
        const originalReplace = ops.replacePart;
        const originalSet = ops.setField;
        const originalRun = db.run.bind(db);
        ops.assignPlan = function (planStore, id, input, ctx) {
            calls.assignPlan += 1;
            calls.lastAssign = { id: id, input: input };
            return originalAssign(planStore, id, input, ctx);
        };
        ops.replacePart = function () {
            calls.replacePart += 1;
            return originalReplace.apply(ops, arguments);
        };
        ops.setField = function () {
            calls.setField += 1;
            return originalSet.apply(ops, arguments);
        };
        db.run = function (sql) {
            calls.runs.push(String(sql));
            return originalRun.apply(db, arguments);
        };
        try {
            return fn(calls);
        } finally {
            ops.assignPlan = originalAssign;
            ops.replacePart = originalReplace;
            ops.setField = originalSet;
            db.run = originalRun;
        }
    }

    it('ac-1: audience and assigneeId go through assignPlan, and personal.js writes no plan SQL', () => {
        seed();
        createAccount('learner01');
        const src = fs.readFileSync(path.join(root, 'server/plans/personal.js'), 'utf8');
        for (const token of ['UPDATE plans', 'INSERT INTO plans', 'FROM plans', 'password_hash', 'display_name']) {
            assert.equal(src.includes(token), false, token);
        }

        withSpies((calls) => {
            const originalAssign = ops.assignPlan;
            ops.assignPlan = function () {
                calls.assignPlan += 1;
                return { error: 'spy' };
            };
            try {
                const before = store.loadWorkingCopy(PLAN_ID);
                const spied = personal.assign(store, PLAN_ID, {
                    audience: 'personal',
                    assigneeId: 'learner01',
                    expectedRevision: 1
                });
                assert.equal(spied.error, 'spy');
                assert.equal(calls.assignPlan, 1);
                assert.equal(calls.runs.length, 0);
                const afterSpy = store.loadWorkingCopy(PLAN_ID);
                assert.equal(afterSpy.document, before.document);
                assert.equal(afterSpy.revision, before.revision);
            } finally {
                ops.assignPlan = originalAssign;
            }
        });

        const result = withSpies((calls) => {
            const assigned = personal.assign(store, PLAN_ID, {
                audience: 'personal',
                assigneeId: 'learner01',
                expectedRevision: 1
            });
            assert.equal(assigned.error, undefined);
            assert.equal(calls.assignPlan, 1);
            assert.equal(calls.replacePart, 0);
            assert.equal(calls.setField, 0);
            assert.equal(calls.lastAssign.input.audience, 'personal');
            assert.equal(calls.lastAssign.input.assigneeId, 'learner01');
            const planWrites = calls.runs.filter((sql) => /plans/i.test(sql));
            assert.equal(planWrites.length, 1);
            assert.equal(planWrites[0].includes('password_hash'), false);
            return assigned;
        });
        assert.equal(result.plan.audience, 'personal');
        assert.equal(result.plan.assigneeId, 'learner01');
        assert.equal(result.revision, 2);
        assert.equal(result.plan.revision, 2);
    });

    it('ac-2: a disabled or unknown assignee is refused and the plan is unchanged', () => {
        const seeded = seed();
        createAccount('disabled1', 'disabled');

        const unknown = personal.assign(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'missing01',
            expectedRevision: 1
        });
        assert.deepEqual(unknown, { error: 'assignee_refused', revision: 1 });

        const disabled = personal.assign(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'disabled1',
            expectedRevision: 1
        });
        assert.deepEqual(disabled, { error: 'assignee_refused', revision: 1 });

        const stored = store.loadWorkingCopy(PLAN_ID);
        assert.equal(stored.revision, seeded.revision);
        assert.equal(stored.document, seeded.document);
        assert.equal(stored.updatedAt, seeded.updatedAt);
        assert.equal(stored.audience, 'shared');
        assert.equal(stored.assigneeId, null);
        assert.equal(stored.document.includes('disabled1'), false);
        assert.equal(stored.document.includes('missing01'), false);
    });

    it('ac-3: shared sets assigneeId null in one operation and validates', () => {
        seed();
        createAccount('learner01');
        const personalPlan = personal.assign(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'Learner01',
            expectedRevision: 1
        });
        assert.equal(personalPlan.error, undefined);
        assert.equal(personalPlan.plan.assigneeId, 'learner01');
        assert.equal(personalPlan.revision, 2);

        const calls = { assignPlan: 0 };
        const original = ops.assignPlan;
        ops.assignPlan = function (planStore, id, input, ctx) {
            calls.assignPlan += 1;
            calls.input = input;
            return original(planStore, id, input, ctx);
        };
        let shared;
        try {
            shared = personal.assign(store, PLAN_ID, {
                audience: 'shared',
                assigneeId: 'learner01',
                expectedRevision: 2
            });
        } finally {
            ops.assignPlan = original;
        }
        assert.equal(calls.assignPlan, 1);
        assert.equal(calls.input.audience, 'shared');
        assert.equal(calls.input.assigneeId, null);
        assert.equal(shared.error, undefined);
        assert.equal(shared.plan.audience, 'shared');
        assert.equal(shared.plan.assigneeId, null);
        assert.equal(shared.revision, 3);
        const validation = validatePlan(shared.plan);
        assert.equal(validation.ok, true);
        assert.equal(validation.blocks.some((item) => item.rule === 'schema'), false);
        const stored = store.loadWorkingCopy(PLAN_ID);
        assert.equal(stored.assigneeId, null);
        assert.equal(stored.audience, 'shared');
    });

    it('refuses a 7-character username in plain words and writes nothing', () => {
        seed();
        createAccount('bnelson');
        const before = store.loadWorkingCopy(PLAN_ID);
        const assigned = personal.assign(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'bnelson',
            expectedRevision: 1
        });
        assert.equal(assigned.error, 'validation');
        assert.equal(assigned.revision, before.revision);
        const hit = assigned.blocks.find((item) => item.path === '/assigneeId');
        assert.equal(hit.rule, 'schema');
        assert.equal(hit.message, "This username format can't be assigned to a personal plan yet.");
        assert.ok(Array.isArray(assigned.warnings));
        assert.ok(Array.isArray(assigned.informs));
        const stored = store.loadWorkingCopy(PLAN_ID);
        assert.equal(stored.document, before.document);
        assert.equal(stored.assigneeId, null);
        assert.equal(stored.audience, 'shared');

        const unknown = personal.assign(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'not-real',
            expectedRevision: before.revision
        });
        assert.deepEqual(unknown, { error: 'assignee_refused', revision: before.revision });
        assert.equal(store.loadWorkingCopy(PLAN_ID).document, before.document);

        const bob = ops.assignPlan(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'Bob',
            expectedRevision: before.revision
        }, testContext('personal-plan-test'));
        assert.equal(bob.error, 'validation');
        const bobHit = bob.blocks.find((item) => item.path === '/assigneeId');
        assert.equal(bobHit.rule, 'schema');
        assert.equal(bobHit.message, "This username format can't be assigned to a personal plan yet.");
        assert.equal(store.loadWorkingCopy(PLAN_ID).document, before.document);
    });

    it('a stale revision does not assign, and progress is not read', () => {
        seed();
        createAccount('learner01');
        const before = store.loadWorkingCopy(PLAN_ID);
        const stale = personal.assign(store, PLAN_ID, {
            audience: 'personal',
            assigneeId: 'learner01',
            expectedRevision: 0
        });
        assert.deepEqual(stale, { error: 'revision_conflict', revision: 1 });
        const after = store.loadWorkingCopy(PLAN_ID);
        assert.equal(after.document, before.document);
        assert.equal(after.revision, before.revision);

        const src = fs.readFileSync(path.join(root, 'server/plans/personal.js'), 'utf8');
        for (const token of ['review_items', 'practice_events', 'lesson_progress', 'openai', 'fetch(']) {
            assert.equal(src.includes(token), false, token);
        }
        const accountSrc = fs.readFileSync(path.join(root, 'server/db/repos/users.js'), 'utf8');
        const finderMatch = accountSrc.match(/function findAccount\([\s\S]*?\n\}\n/);
        assert.ok(finderMatch, 'findAccount function body');
        const finderBody = finderMatch[0];
        assert.equal(finderBody.includes('password_hash'), false);
        assert.equal(finderBody.includes('display_name'), false);
        assert.match(finderBody, /SELECT id, username, status FROM users/);
    });
});
