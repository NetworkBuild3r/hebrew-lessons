/**
 * Plan operations hardening (INIT-008/SPEC-002): the id-mismatch defect, the
 * audience and assigneeId bypass, and the write context. Temporary database
 * only. No test opens data/hebrew.sqlite.
 * Provenance: INIT-008/SPEC-002
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const pretty = fs.readFileSync(path.join(
    root, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json'
), 'utf8');
const PLAN_ID = 'beginner-unit-1';
const LESSON = 'units/unit-1/lessons/cafe-order';
const LISTEN = LESSON + '/blocks/listen';
const WORD = LESSON + '/blocks/introduction/words/2';

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const modules = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/audit',
        '../server/db/repos/users',
        '../server/plans/store',
        '../server/plans/validate',
        '../server/plans/ops',
        '../server/plans/publish',
        '../server/plans/personal'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    return {
        db,
        store: require('../server/plans/store'),
        ops: require('../server/plans/ops'),
        publish: require('../server/plans/publish'),
        personal: require('../server/plans/personal'),
        users: require('../server/db/repos/users'),
        context: require('../server/plans/context')
    };
}

describe('plan ops hardening (INIT-008/SPEC-002)', { concurrency: false }, () => {
    let tmp;
    let db;
    let store;
    let ops;
    let publish;
    let personal;
    let users;
    let context;
    let ctx;
    const savedLessons = process.env.LESSONS_DIR;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-hardening-'));
        process.env.LESSONS_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-hardening-lessons-'));
        ({ db, store, ops, publish, personal, users, context } = freshModules(tmp));
        db.migrate();
        ctx = context.makeContext({ channel: 'admin', username: 'example-admin', tool: 'test' });
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
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM users');
    });

    function seed(edit) {
        const plan = JSON.parse(pretty);
        if (edit) edit(plan);
        const text = JSON.stringify(plan);
        store.saveWorkingCopy({
            id: plan.id,
            document: text,
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return store.loadWorkingCopy(plan.id);
    }

    function auditCount() {
        return db.get('SELECT COUNT(*) AS n FROM audit_log').n;
    }

    function assertUntouched(before) {
        const row = store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, before.document);
        assert.equal(row.revision, before.revision);
        assert.equal(row.updatedAt, before.updatedAt);
        assert.equal(auditCount(), 0);
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

    it('ac-1: replace_part with an id that differs from its path is id_mismatch and writes nothing', () => {
        const before = seed();
        const block = { id: 'other-id', type: 'listening', passage: 'תּוֹדָה.' };
        const refused = ops.replacePart(store, PLAN_ID, { path: LISTEN, value: block, expectedRevision: 1 }, ctx);
        assert.deepEqual(refused, { error: 'id_mismatch', revision: 1, path: LISTEN });
        assertUntouched(before);

        const word = { id: '9', word: 'קָפֶה', meaning: 'coffee', sound: 'ka-FE' };
        const wordRefused = ops.replacePart(store, PLAN_ID, { path: WORD, value: word, expectedRevision: 1 }, ctx);
        assert.equal(wordRefused.error, 'id_mismatch');
        assert.equal(wordRefused.path, WORD);
        assertUntouched(before);

        const ok = ops.replacePart(store, PLAN_ID, {
            path: LISTEN,
            value: { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' },
            expectedRevision: 1
        }, ctx);
        assert.equal(ok.error, undefined);
        assert.equal(ok.revision, 2);
        const okWord = ops.replacePart(store, PLAN_ID, {
            path: WORD,
            value: { id: '2', word: 'קָפֶה', meaning: 'coffee', sound: 'ka-FE' },
            expectedRevision: 2
        }, ctx);
        assert.equal(okWord.error, undefined);
        assert.equal(auditCount(), 2);
    });

    it('ac-1: set_field cannot change the id of a part either (same defect, other door)', () => {
        const before = seed();
        const refused = ops.setField(store, PLAN_ID, {
            path: LISTEN, field: 'id', value: 'renamed', expectedRevision: 1
        }, ctx);
        assert.equal(refused.error, 'id_mismatch');
        assertUntouched(before);
        const same = ops.setField(store, PLAN_ID, {
            path: LISTEN, field: 'id', value: 'listen', expectedRevision: 1
        }, ctx);
        assert.equal(same.error, undefined);
    });

    it('ac-2: set_field of audience or assigneeId on the plan is assign_required', () => {
        const before = seed();
        for (const input of [
            { field: 'audience', value: 'personal' },
            { field: 'assigneeId', value: 'learner01' },
            { field: 'audience', value: 'shared' }
        ]) {
            const refused = ops.setField(store, PLAN_ID, Object.assign({ path: '', expectedRevision: 1 }, input), ctx);
            assert.deepEqual(refused, { error: 'assign_required', revision: 1 }, input.field);
            assertUntouched(before);
        }
    });

    it('ac-2: replace_part of the plan with a changed audience or assigneeId is assign_required; unchanged is accepted', () => {
        const before = seed();
        const whole = JSON.parse(before.document);

        const audience = Object.assign({}, whole, { audience: 'personal', assigneeId: 'learner01' });
        assert.deepEqual(
            ops.replacePart(store, PLAN_ID, { path: '', value: audience, expectedRevision: 1 }, ctx),
            { error: 'assign_required', revision: 1 }
        );
        const assignee = Object.assign({}, whole, { assigneeId: 'learner01' });
        assert.equal(
            ops.replacePart(store, PLAN_ID, { path: '', value: assignee, expectedRevision: 1 }, ctx).error,
            'assign_required'
        );
        assertUntouched(before);

        const titled = Object.assign({}, whole, { title: 'יחידה חדשה' });
        const ok = ops.replacePart(store, PLAN_ID, { path: '', value: titled, expectedRevision: 1 }, ctx);
        assert.equal(ok.error, undefined);
        assert.equal(ok.plan.title, 'יחידה חדשה');
        assert.equal(ok.plan.audience, 'shared');
        assert.equal(ok.plan.assigneeId, null);
        assert.equal(auditCount(), 1);
    });

    it('ac-2: a personal plan keeps its assignee through an unchanged whole-plan replace', () => {
        createAccount('learner01');
        seed();
        const assigned = personal.assign(store, PLAN_ID, {
            audience: 'personal', assigneeId: 'learner01', expectedRevision: 1
        }, ctx);
        assert.equal(assigned.error, undefined);
        const value = JSON.parse(JSON.stringify(assigned.plan));
        value.title = 'כותרת';
        const ok = ops.replacePart(store, PLAN_ID, { path: '', value, expectedRevision: 2 }, ctx);
        assert.equal(ok.error, undefined, JSON.stringify(ok));
        assert.equal(store.loadWorkingCopy(PLAN_ID).assigneeId, 'learner01');
    });

    it('ac-3: assignPlan is the only writer of audience and assigneeId and checks only shape', () => {
        seed();
        const bad = [
            { audience: 'everyone', assigneeId: null },
            { audience: 'personal', assigneeId: null },
            { audience: 'personal', assigneeId: '' },
            { audience: 'shared', assigneeId: 'learner01' }
        ];
        for (const input of bad) {
            const result = ops.assignPlan(store, PLAN_ID, Object.assign({ expectedRevision: 1 }, input), ctx);
            assert.equal(result.error, 'invalid_op', JSON.stringify(input));
        }
        assert.equal(store.loadWorkingCopy(PLAN_ID).revision, 1);
        assert.equal(auditCount(), 0);

        const stale = ops.assignPlan(store, PLAN_ID, { audience: 'shared', assigneeId: null, expectedRevision: 7 }, ctx);
        assert.deepEqual(stale, { error: 'revision_conflict', revision: 1 });

        const done = ops.assignPlan(store, PLAN_ID, {
            audience: 'personal', assigneeId: 'learner01', expectedRevision: 1
        }, ctx);
        assert.equal(done.error, undefined);
        assert.equal(done.plan.audience, 'personal');
        assert.equal(done.plan.assigneeId, 'learner01');
        assert.equal(done.status, 'draft');
        const row = db.get('SELECT * FROM audit_log');
        assert.equal(row.action, 'plan.assign');
        assert.equal(row.actor, 'admin:example-admin');
    });

    it('ac-3: personal.assign refuses a disabled or unknown account, then assigns an active one through assignPlan', () => {
        createAccount('disabled1', 'disabled');
        createAccount('learner01');
        const before = seed();
        assert.deepEqual(
            personal.assign(store, PLAN_ID, { audience: 'personal', assigneeId: 'disabled1', expectedRevision: 1 }, ctx),
            { error: 'assignee_refused', revision: 1 }
        );
        assert.deepEqual(
            personal.assign(store, PLAN_ID, { audience: 'personal', assigneeId: 'nobody-here', expectedRevision: 1 }, ctx),
            { error: 'assignee_refused', revision: 1 }
        );
        assertUntouched(before);

        const calls = [];
        const original = ops.assignPlan;
        ops.assignPlan = function () {
            calls.push('assignPlan');
            return original.apply(ops, arguments);
        };
        let ok;
        try {
            ok = personal.assign(store, PLAN_ID, { audience: 'personal', assigneeId: 'learner01', expectedRevision: 1 }, ctx);
        } finally {
            ops.assignPlan = original;
        }
        assert.deepEqual(calls, ['assignPlan']);
        assert.equal(ok.plan.audience, 'personal');
        assert.equal(ok.plan.assigneeId, 'learner01');
        const shared = personal.assign(store, PLAN_ID, { audience: 'shared', assigneeId: 'learner01', expectedRevision: 2 }, ctx);
        assert.equal(shared.plan.assigneeId, null);
    });

    it('ac-4: every mutating function throws TypeError without a valid context and writes nothing', () => {
        const before = seed();
        const forged = { actor: { kind: 'human', name: 'example-admin' } };
        const parsed = JSON.parse(JSON.stringify(ctx));
        const input = (extra) => Object.assign({ path: '', expectedRevision: 1 }, extra);
        const calls = {
            replacePart: (c) => ops.replacePart(store, PLAN_ID, input({ value: JSON.parse(before.document) }), c),
            insertBlock: (c) => ops.insertBlock(store, PLAN_ID, input({
                path: LESSON, block: { id: 'x', type: 'listening', passage: 'תּוֹדָה.' }
            }), c),
            moveBlock: (c) => ops.moveBlock(store, PLAN_ID, input({ path: LISTEN, index: 0 }), c),
            removeBlock: (c) => ops.removeBlock(store, PLAN_ID, input({ path: LISTEN }), c),
            setField: (c) => ops.setField(store, PLAN_ID, input({ field: 'title', value: 'x' }), c),
            assignPlan: (c) => ops.assignPlan(store, PLAN_ID, input({ audience: 'shared', assigneeId: null }), c),
            publishPlan: (c) => publish.publishPlan(store, PLAN_ID, { expectedRevision: 1 }, c),
            personalAssign: (c) => personal.assign(store, PLAN_ID, { audience: 'shared', expectedRevision: 1 }, c)
        };
        for (const name of Object.keys(calls)) {
            for (const bad of [undefined, null, {}, forged, parsed, 'admin']) {
                assert.throws(() => calls[name](bad), TypeError, name);
            }
        }
        assertUntouched(before);
        assert.throws(() => ops.readPart(store, PLAN_ID, { path: '' }, forged), TypeError);
        assert.equal(ops.readPart(store, PLAN_ID, { path: '' }).revision, 1);
    });

    it('context: the actor is derived from the channel and nothing else', () => {
        const mcp = context.makeContext({ channel: 'mcp', tool: 'set_field', name: 'human', username: 'example-admin' });
        assert.deepEqual(mcp.actor, { kind: 'ai', name: 'mcp', tool: 'set_field' });
        assert.equal(context.actorString(mcp), 'mcp');

        const admin = context.makeContext({ channel: 'admin', username: 'example-admin', name: 'ignored' });
        assert.deepEqual(admin.actor, { kind: 'human', name: 'example-admin' });
        assert.equal(context.actorString(admin), 'admin:example-admin');

        const system = context.makeContext({ channel: 'system', name: 'seed-starter-content', requestId: 'r-1' });
        assert.deepEqual(system.actor, { kind: 'system', name: 'seed-starter-content' });
        assert.equal(system.requestId, 'r-1');
        assert.equal(context.actorString(system), 'system:seed-starter-content');

        assert.throws(() => context.makeContext({ channel: 'ai' }), TypeError);
        assert.throws(() => context.makeContext({ channel: 'human', username: 'x' }), TypeError);
        assert.throws(() => context.makeContext({ channel: 'admin' }), TypeError);
        assert.throws(() => context.makeContext({ channel: 'system' }), TypeError);
        assert.throws(() => context.makeContext({ channel: '__proto__' }), TypeError);
        assert.throws(() => context.makeContext(), TypeError);
        assert.equal(Object.isFrozen(mcp), true);
        assert.equal(Object.isFrozen(mcp.actor), true);
        assert.equal(context.isContext(mcp), true);
        assert.equal(context.isContext(JSON.parse(JSON.stringify(mcp))), false);
    });

    it('security: assignPlan is reached only through personal.assign', () => {
        const hits = [];
        for (const dir of ['server', 'mcp']) {
            const stack = [path.join(root, dir)];
            while (stack.length) {
                const current = stack.pop();
                for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
                    const full = path.join(current, entry.name);
                    if (entry.isDirectory()) stack.push(full);
                    else if (entry.name.endsWith('.js') && fs.readFileSync(full, 'utf8').includes('assignPlan')) {
                        hits.push(path.relative(root, full).split(path.sep).join('/'));
                    }
                }
            }
        }
        assert.deepEqual(hits.sort(), ['server/plans/ops.js', 'server/plans/personal.js']);
    });
});
