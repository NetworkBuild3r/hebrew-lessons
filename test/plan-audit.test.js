/**
 * Plan audit rows and the channel-derived actor (INIT-008/SPEC-002).
 * One audit_log row per plan write, in the same transaction as the write.
 * Temporary database through the signed-in server harness. No network, no
 * ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-002
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const harness = require('./helpers/signed-in-server');
const { testContext } = require('./helpers/plan-ctx');

const PRETTY = fs.readFileSync(path.join(
    harness.REPO_ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json'
), 'utf8');
const PLAN_ID = 'beginner-unit-1';
const LESSON = 'units/unit-1/lessons/cafe-order';
const LISTEN = LESSON + '/blocks/listen';
const WORD = LESSON + '/blocks/introduction/words/2';
const NEW_VALUE = 'MUST-NOT-APPEAR-IN-AUDIT-espresso';
const EXTRA = { id: 'extra', type: 'listening', passage: 'אֲנִי רוֹצֶה קָפֶה.' };

describe('plan audit rows (INIT-008/SPEC-002)', { concurrency: false }, () => {
    let baseUrl;
    let adminCookie;
    let db;
    let store;
    let ops;
    let publish;
    let handlers;
    let metas;
    let prevToken;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        db = require('../server/db');

        const accounts = require('../server/accounts');
        const cookies = require('../server/http/cookies');
        accounts.createUser({
            username: 'example-admin',
            password: harness.GOOD_PASSWORD,
            role: 'admin',
            displayName: 'Example Admin',
            actor: 'test'
        });
        const session = accounts.signIn('example-admin', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        adminCookie = cookies.cookieName() + '=' + encodeURIComponent(session.token);

        store = require('../server/plans/store');
        ops = require('../server/plans/ops');
        publish = require('../server/plans/publish');

        prevToken = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'test-plan-audit-token';
        const tools = require('../mcp/tools');
        metas = Object.create(null);
        tools.registerLessonTools({ registerTool(name, meta) { metas[name] = meta; } });
        handlers = tools.toolHandlers;
    });

    after(async () => {
        if (prevToken === undefined) delete process.env.MCP_AUTH_TOKEN;
        else process.env.MCP_AUTH_TOKEN = prevToken;
        await harness.stop();
    });

    beforeEach(() => {
        db.run('DELETE FROM plans');
        db.run('DELETE FROM audit_log');
    });

    function seed() {
        const plan = JSON.parse(PRETTY);
        // A re-seed rewinds the plan to revision 1; its old log and marks would collide (SPEC-004).
        db.run('DELETE FROM plan_revisions WHERE plan_id = ?', plan.id);
        db.run('DELETE FROM plan_field_marks WHERE plan_id = ?', plan.id);
        store.saveWorkingCopy({
            id: plan.id,
            document: PRETTY,
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return store.loadWorkingCopy(plan.id);
    }

    async function api(method, urlPath, body, extraHeaders) {
        const headers = Object.assign({ Cookie: adminCookie }, extraHeaders || {});
        if (body !== undefined) headers['Content-Type'] = 'application/json';
        if (method !== 'GET') headers.Origin = baseUrl;
        const res = await fetch(baseUrl + urlPath, {
            method,
            headers,
            body: body === undefined ? undefined : JSON.stringify(body)
        });
        const text = await res.text();
        let json = null;
        try { json = JSON.parse(text); } catch (_) { /* not json */ }
        return { status: res.status, text, json };
    }

    function rows() {
        return db.all('SELECT * FROM audit_log ORDER BY id');
    }

    function assertNoValue(row) {
        for (const key of Object.keys(row)) {
            assert.equal(String(row[key]).includes(NEW_VALUE), false, key);
        }
    }

    it('ac-5: an admin set_field writes one row: admin:example-admin, plan.set_field, no value', async () => {
        seed();
        const res = await api('PATCH', '/api/plans/' + PLAN_ID + '/fields', {
            path: WORD, field: 'meaning', value: NEW_VALUE, expectedRevision: 1
        });
        assert.equal(res.status, 200, res.text);
        const all = rows();
        assert.equal(all.length, 1);
        const row = all[0];
        assert.equal(row.actor, 'admin:example-admin');
        assert.equal(row.action, 'plan.set_field');
        assert.equal(row.target, PLAN_ID);
        const detail = JSON.parse(row.detail);
        assert.deepEqual(detail, { revision: 2, path: WORD, field: 'meaning', tool: 'set_field' });
        assertNoValue(row);
        assert.equal(row.detail.includes('coffee'), false);
    });

    it('ac-5: the same call through the MCP adapter writes actor mcp', async () => {
        seed();
        const result = await handlers.set_field({
            id: PLAN_ID, path: WORD, field: 'meaning', value: NEW_VALUE, expectedRevision: 1
        }, { requestId: 17 });
        assert.equal(result.isError, undefined, JSON.stringify(result));
        const all = rows();
        assert.equal(all.length, 1);
        assert.equal(all[0].actor, 'mcp');
        assert.equal(all[0].action, 'plan.set_field');
        assert.deepEqual(JSON.parse(all[0].detail), {
            revision: 2, path: WORD, field: 'meaning', tool: 'set_field', requestId: '17'
        });
        assertNoValue(all[0]);
    });

    it('ac-5: every write route and tool names its action, and publish writes plan.publish', async () => {
        const routes = [
            ['PUT', '/parts', { path: LISTEN, value: { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' } }, 'plan.replace_part'],
            ['POST', '/blocks', { path: LESSON, index: 1, block: EXTRA }, 'plan.insert_block'],
            ['POST', '/blocks/move', { path: LISTEN, index: 0 }, 'plan.move_block'],
            ['DELETE', '/parts', { path: LISTEN }, 'plan.remove_block'],
            ['POST', '/publish', {}, 'plan.publish']
        ];
        for (const [method, suffix, body, action] of routes) {
            seed();
            db.run('DELETE FROM audit_log');
            const res = await api(method, '/api/plans/' + PLAN_ID + suffix, Object.assign({ expectedRevision: 1 }, body));
            assert.equal(res.status, 200, action + ' ' + res.text);
            const all = rows();
            assert.equal(all.length, 1, action);
            assert.equal(all[0].action, action);
            assert.equal(all[0].actor, 'admin:example-admin');
            assert.equal(JSON.parse(all[0].detail).revision, 2, action);
        }

        const tools = [
            ['replace_part', { path: LISTEN, value: { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' } }, 'plan.replace_part'],
            ['insert_block', { path: LESSON, index: 1, block: EXTRA }, 'plan.insert_block'],
            ['move_block', { path: LISTEN, index: 0 }, 'plan.move_block'],
            ['remove_block', { path: LISTEN }, 'plan.remove_block'],
            ['publish_plan', {}, 'plan.publish']
        ];
        for (const [tool, args, action] of tools) {
            seed();
            db.run('DELETE FROM audit_log');
            const result = await handlers[tool](Object.assign({ id: PLAN_ID, expectedRevision: 1 }, args));
            assert.equal(result.isError, undefined, tool + ' ' + JSON.stringify(result));
            const all = rows();
            assert.equal(all.length, 1, tool);
            assert.equal(all[0].action, action);
            assert.equal(all[0].actor, 'mcp');
            assert.equal(JSON.parse(all[0].detail).tool, tool);
        }
    });

    it('ac-6: a conflict, a validation refusal, id_mismatch and assign_required leave audit_log unchanged', async () => {
        const before = seed();
        const attempts = [
            [409, { path: WORD, field: 'meaning', value: 'x', expectedRevision: 99 }],
            [400, { path: '', field: 'title', value: '<b>no</b>', expectedRevision: 1 }],
            [400, { path: LISTEN, field: 'id', value: 'renamed', expectedRevision: 1 }],
            [400, { path: '', field: 'assigneeId', value: 'learner01', expectedRevision: 1 }]
        ];
        for (const [status, body] of attempts) {
            const res = await api('PATCH', '/api/plans/' + PLAN_ID + '/fields', body);
            assert.equal(res.status, status, res.text);
        }
        const replace = await api('PUT', '/api/plans/' + PLAN_ID + '/parts', {
            path: LISTEN, value: { id: 'other', type: 'listening', passage: 'תּוֹדָה.' }, expectedRevision: 1
        });
        assert.equal(replace.status, 400);
        assert.equal(replace.json.code, 'id_mismatch');
        const assign = await api('PUT', '/api/plans/' + PLAN_ID + '/parts', {
            path: '',
            value: Object.assign(JSON.parse(PRETTY), { audience: 'personal', assigneeId: 'learner01' }),
            expectedRevision: 1
        });
        assert.equal(assign.status, 400);
        assert.equal(assign.json.code, 'assign_required');
        const publishStale = await api('POST', '/api/plans/' + PLAN_ID + '/publish', { expectedRevision: 5 });
        assert.equal(publishStale.status, 409);

        assert.equal(rows().length, 0);
        const row = store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, before.document);
        assert.equal(row.revision, 1);
        assert.equal(store.loadSnapshot(PLAN_ID), null);
    });

    it('ac-6: a failing audit insert rolls the working-copy write back', () => {
        const before = seed();
        const repo = require('../server/db/repos/audit');
        const original = repo.append;
        repo.append = function () { throw new Error('audit insert failed'); };
        try {
            assert.throws(() => ops.setField(store, PLAN_ID, {
                path: WORD, field: 'meaning', value: 'x', expectedRevision: 1
            }, testContext()), /audit insert failed/);
            assert.throws(() => ops.moveBlock(store, PLAN_ID, {
                path: LISTEN, index: 0, expectedRevision: 1
            }, testContext()), /audit insert failed/);
            assert.throws(() => publish.publishPlan(store, PLAN_ID, { expectedRevision: 1 }, testContext()),
                /audit insert failed/);
        } finally {
            repo.append = original;
        }
        const row = store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, before.document);
        assert.equal(row.revision, 1);
        assert.equal(row.status, 'draft');
        assert.equal(row.snapshot, null);
        assert.equal(rows().length, 0);

        const ok = ops.setField(store, PLAN_ID, {
            path: WORD, field: 'meaning', value: 'x', expectedRevision: 1
        }, testContext());
        assert.equal(ok.error, undefined);
        assert.equal(rows().length, 1);
    });

    it('ac-7: a request body key actor, by, or kind is rejected and nothing is written', async () => {
        const before = seed();
        const base = { path: WORD, field: 'meaning', value: 'x', expectedRevision: 1 };
        for (const key of ['actor', 'by', 'kind']) {
            const body = Object.assign({}, base);
            body[key] = key === 'actor' ? { kind: 'ai', name: 'mcp' } : 'ai';
            const res = await api('PATCH', '/api/plans/' + PLAN_ID + '/fields', body);
            assert.equal(res.status, 400, key);
        }
        const publishRes = await api('POST', '/api/plans/' + PLAN_ID + '/publish', {
            expectedRevision: 1, actor: 'mcp'
        });
        assert.equal(publishRes.status, 400);
        assert.equal(rows().length, 0);
        assert.equal(store.loadWorkingCopy(PLAN_ID).document, before.document);
    });

    it('ac-7: no plan tool can set the actor, and an actor argument is ignored', async () => {
        for (const name of ['read_part', 'replace_part', 'insert_block', 'move_block', 'remove_block', 'set_field', 'publish_plan']) {
            const keys = Object.keys(metas[name].inputSchema);
            for (const forbidden of ['actor', 'by', 'kind', 'client', 'username', 'name']) {
                assert.equal(keys.includes(forbidden), false, name + ' ' + forbidden);
            }
        }
        seed();
        const result = await handlers.set_field({
            id: PLAN_ID, path: WORD, field: 'meaning', value: 'x', expectedRevision: 1,
            actor: { kind: 'human', name: 'example-admin' }, by: 'admin:example-admin', kind: 'human'
        });
        assert.equal(result.isError, undefined);
        assert.equal(rows()[0].actor, 'mcp');
    });

    it('no audit row holds document text: a whole-plan replace records the path only', async () => {
        seed();
        const value = JSON.parse(PRETTY);
        value.title = NEW_VALUE;
        const res = await api('PUT', '/api/plans/' + PLAN_ID + '/parts', { path: '', value, expectedRevision: 1 });
        assert.equal(res.status, 200, res.text);
        const all = rows();
        assert.equal(all.length, 1);
        assert.deepEqual(JSON.parse(all[0].detail), { revision: 2, path: '', tool: 'replace_part' });
        assertNoValue(all[0]);
    });
});
