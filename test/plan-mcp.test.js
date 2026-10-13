/**
 * Plan MCP tools (INIT-004/SPEC-005).
 * Temporary database. Handlers are called directly, same pattern as
 * test/mcp-assignment.test.js. No network. No audio providers.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const http = require('http');
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
const WORD = 'units/unit-1/lessons/cafe-order/blocks/introduction/words/2';
const LESSON = 'units/unit-1/lessons/cafe-order';
const LISTEN = LESSON + '/blocks/listen';
const DIALOGUE = LESSON + '/blocks/dialogue';
const EXTRA = {
    id: 'extra',
    type: 'listening',
    passage: 'אֲנִי רוֹצֶה קָפֶה.'
};
const LISTEN_BLOCK = { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' };
const LEGACY_TOOLS = [
    'list_lessons',
    'get_lesson',
    'list_words',
    'list_activity_types',
    'list_users',
    'upsert_lesson',
    'set_lesson_visibility',
    'generate_lesson_audio'
];
const PLAN_TOOLS = [
    'read_part',
    'replace_part',
    'insert_block',
    'move_block',
    'remove_block',
    'set_field',
    'publish_plan'
];
const FN_BY_TOOL = {
    read_part: 'readPart',
    replace_part: 'replacePart',
    insert_block: 'insertBlock',
    move_block: 'moveBlock',
    remove_block: 'removeBlock',
    set_field: 'setField',
    publish_plan: 'publishPlan'
};

function replyText(result) {
    assert.ok(result && result.content && result.content[0]);
    return result.content[0].text;
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/plans/store',
        '../server/plans/validate',
        '../server/plans/ops',
        '../server/plans/publish',
        '../server/accounts',
        '../mcp/tools',
        '../mcp/http'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

describe('plan MCP tools (INIT-004/SPEC-005)', { concurrency: false }, () => {
    let tmp;
    let lessons;
    let db;
    let store;
    let ops;
    let publishMod;
    let tools;
    let handlers;
    let metas;
    let originals;
    let spyLog;
    let prevEnv;
    const token = 'test-plan-mcp-token';

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

    function resetPlan() {
        db.run('DELETE FROM plans');
        return seed();
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-plan-mcp-'));
        lessons = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-plan-mcp-lessons-'));
        fs.writeFileSync(path.join(lessons, 'catalog.js'), 'window.HEBREW_CATALOG = [];\n', 'utf8');
        prevEnv = {
            HEBREW_ROOT: process.env.HEBREW_ROOT,
            DATA_DIR: process.env.DATA_DIR,
            LESSONS_DIR: process.env.LESSONS_DIR,
            MCP_AUTH_TOKEN: process.env.MCP_AUTH_TOKEN,
            MCP_PORT: process.env.MCP_PORT,
            HOST: process.env.HOST
        };
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        process.env.LESSONS_DIR = lessons;
        process.env.MCP_AUTH_TOKEN = token;
        process.env.MCP_PORT = '0';
        process.env.HOST = '127.0.0.1';
        clearModules();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        store = require('../server/plans/store');
        ops = require('../server/plans/ops');
        publishMod = require('../server/plans/publish');
        tools = require('../mcp/tools');

        originals = {
            readPart: ops.readPart,
            replacePart: ops.replacePart,
            insertBlock: ops.insertBlock,
            moveBlock: ops.moveBlock,
            removeBlock: ops.removeBlock,
            setField: ops.setField,
            publishPlan: publishMod.publishPlan
        };
        spyLog = [];
        function wrap(target, name) {
            target[name] = function () {
                spyLog.push(name);
                return originals[name].apply(this, arguments);
            };
        }
        wrap(ops, 'readPart');
        wrap(ops, 'replacePart');
        wrap(ops, 'insertBlock');
        wrap(ops, 'moveBlock');
        wrap(ops, 'removeBlock');
        wrap(ops, 'setField');
        wrap(publishMod, 'publishPlan');

        metas = Object.create(null);
        tools.registerLessonTools({
            registerTool(name, meta) {
                metas[name] = meta;
            }
        });
        handlers = tools.toolHandlers;
    });

    after(() => {
        for (const name of Object.keys(originals)) {
            const target = name === 'publishPlan' ? publishMod : ops;
            target[name] = originals[name];
        }
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        try {
            fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(lessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        for (const [key, value] of Object.entries(prevEnv)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        clearModules();
    });

    it('ac-4: legacy tools and the seven plan tools are registered', () => {
        const names = Object.keys(handlers);
        for (const name of LEGACY_TOOLS) {
            assert.equal(names.includes(name), true, name);
        }
        for (const name of PLAN_TOOLS) {
            assert.equal(names.includes(name), true, name);
        }
        assert.equal(typeof handlers.list_lessons, 'function');
        assert.equal(typeof handlers.get_lesson, 'function');
        assert.equal(typeof handlers.upsert_lesson, 'function');
        assert.equal(typeof handlers.set_lesson_visibility, 'function');
        assert.equal(typeof handlers.generate_lesson_audio, 'function');
        assert.equal(metas.upsert_lesson.inputSchema.worksheet !== undefined, true);
        assert.equal(metas.upsert_lesson.inputSchema.plan, undefined);
        assert.equal(metas.upsert_lesson.inputSchema.document, undefined);
        assert.equal(metas.set_field.inputSchema.path !== undefined, true);
        assert.equal(metas.set_field.inputSchema.expectedRevision !== undefined, true);
        assert.equal(metas.set_field.inputSchema.field !== undefined, true);
        assert.equal(metas.set_field.inputSchema.value !== undefined, true);
        assert.equal(metas.insert_block.inputSchema.index !== undefined, true);
        assert.equal(metas.insert_block.inputSchema.block !== undefined, true);
    });

    it('ac-1: each plan tool calls its function once and stores the same bytes', async () => {
        const cases = [
            {
                tool: 'read_part',
                args: { id: PLAN_ID, path: WORD },
                input: { path: WORD }
            },
            {
                tool: 'replace_part',
                args: {
                    id: PLAN_ID,
                    path: LISTEN,
                    value: LISTEN_BLOCK,
                    expectedRevision: 1
                },
                input: {
                    path: LISTEN,
                    value: LISTEN_BLOCK,
                    expectedRevision: 1
                }
            },
            {
                tool: 'insert_block',
                args: {
                    id: PLAN_ID,
                    path: LESSON,
                    index: 1,
                    block: EXTRA,
                    expectedRevision: 1
                },
                input: {
                    path: LESSON,
                    index: 1,
                    block: EXTRA,
                    expectedRevision: 1
                }
            },
            {
                tool: 'move_block',
                args: { id: PLAN_ID, path: DIALOGUE, index: 0, expectedRevision: 1 },
                input: { path: DIALOGUE, index: 0, expectedRevision: 1 }
            },
            {
                tool: 'remove_block',
                args: { id: PLAN_ID, path: DIALOGUE, expectedRevision: 1 },
                input: { path: DIALOGUE, expectedRevision: 1 }
            },
            {
                tool: 'set_field',
                args: {
                    id: PLAN_ID,
                    path: WORD,
                    field: 'meaning',
                    value: 'espresso',
                    expectedRevision: 1
                },
                input: {
                    path: WORD,
                    field: 'meaning',
                    value: 'espresso',
                    expectedRevision: 1
                }
            },
            {
                tool: 'publish_plan',
                args: { id: PLAN_ID, expectedRevision: 1 },
                input: { expectedRevision: 1 }
            }
        ];

        for (const spec of cases) {
            const fn = FN_BY_TOOL[spec.tool];
            resetPlan();
            spyLog.length = 0;
            const result = await handlers[spec.tool](spec.args);
            assert.equal(result.isError, undefined, spec.tool + ' ' + replyText(result));
            const viaTool = store.loadWorkingCopy(PLAN_ID);
            const toolSpy = spyLog.slice();
            let partBytes = null;
            if (spec.tool === 'read_part') {
                partBytes = JSON.stringify(JSON.parse(replyText(result)).part);
            }
            resetPlan();
            const direct = originals[fn](store, PLAN_ID, spec.input, testContext());
            const viaFn = store.loadWorkingCopy(PLAN_ID);
            assert.equal(viaTool.document, viaFn.document, spec.tool);
            assert.equal(viaTool.revision, viaFn.revision, spec.tool);
            assert.equal(viaTool.status, viaFn.status, spec.tool);
            if (spec.tool === 'publish_plan') {
                assert.equal(store.loadSnapshot(PLAN_ID), viaFn.document);
                assert.equal(viaTool.document, viaFn.document);
            }
            if (spec.tool === 'read_part') {
                assert.equal(partBytes, JSON.stringify(direct.part));
                assert.equal(viaTool.document, pretty);
            }
            assert.deepEqual(toolSpy, [fn], spec.tool);
        }
    });

    it('ac-2: a stale expectedRevision returns revision_conflict and does not write', async () => {
        const seeded = resetPlan();
        const staleValue = 'MUST-NOT-LAND-revision-conflict';
        const result = await handlers.set_field({
            id: PLAN_ID,
            path: WORD,
            field: 'meaning',
            value: staleValue,
            expectedRevision: 0
        });
        assert.equal(result.isError, true);
        const body = JSON.parse(replyText(result));
        assert.equal(body.error, 'revision_conflict');
        assert.equal(body.revision, seeded.revision);
        assert.equal(body.plan, undefined);
        assert.equal(body.document, undefined);
        assert.equal(replyText(result).includes(staleValue), false);
        assert.equal(replyText(result).includes('schemaVersion'), false);
        const row = store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, seeded.document);
        assert.equal(row.revision, seeded.revision);
        assert.equal(row.updatedAt, seeded.updatedAt);

        const published = originals.publishPlan(store, PLAN_ID, { expectedRevision: 1 }, testContext());
        assert.equal(published.error, undefined);
        const frozen = store.loadWorkingCopy(PLAN_ID);
        const again = await handlers.publish_plan({
            id: PLAN_ID,
            expectedRevision: 1
        });
        assert.equal(again.isError, true);
        const conflict = JSON.parse(replyText(again));
        assert.equal(conflict.error, 'revision_conflict');
        assert.equal(conflict.revision, frozen.revision);
        const after = store.loadWorkingCopy(PLAN_ID);
        assert.equal(after.document, frozen.document);
        assert.equal(after.revision, frozen.revision);
        assert.equal(store.loadSnapshot(PLAN_ID), frozen.document);
    });

    it('ac-3: with the token unset, read_part returns no plan JSON', async () => {
        resetPlan();
        const previous = process.env.MCP_AUTH_TOKEN;
        delete process.env.MCP_AUTH_TOKEN;
        try {
            for (const name of PLAN_TOOLS) {
                const result = await handlers[name]({
                    id: PLAN_ID,
                    path: '',
                    expectedRevision: 1,
                    field: 'title',
                    value: 'leaked',
                    block: EXTRA,
                    index: 0
                });
                assert.equal(result.isError, true, name);
                const text = replyText(result);
                assert.match(text, /MCP_AUTH_TOKEN/);
                assert.equal(text.includes('schemaVersion'), false, name);
                assert.equal(text.includes('יחידה 1'), false, name);
                assert.equal(text.includes('leaked'), false, name);
            }
            const row = store.loadWorkingCopy(PLAN_ID);
            assert.equal(row.document, pretty);
            assert.equal(row.revision, 1);
        } finally {
            process.env.MCP_AUTH_TOKEN = previous;
        }
    });

    it('a wrong bearer token is refused and the body has no plan JSON', async () => {
        resetPlan();
        const server = require('../mcp/http').listen();
        await new Promise((resolve) => server.once('listening', resolve));
        const port = server.address().port;
        const payload = JSON.stringify({
            jsonrpc: '2.0',
            id: 1,
            method: 'tools/call',
            params: { name: 'read_part', arguments: { id: PLAN_ID, path: '' } }
        });
        function post(headers) {
            return new Promise((resolve, reject) => {
                const req = http.request({
                    host: '127.0.0.1',
                    port,
                    path: '/mcp',
                    method: 'POST',
                    headers: Object.assign({
                        'Content-Type': 'application/json',
                        Accept: 'application/json, text/event-stream',
                        'Content-Length': Buffer.byteLength(payload)
                    }, headers)
                }, (res) => {
                    let data = '';
                    res.on('data', (chunk) => { data += chunk; });
                    res.on('end', () => resolve({ status: res.statusCode, data }));
                });
                req.on('error', reject);
                req.end(payload);
            });
        }
        try {
            const wrong = await post({ Authorization: 'Bearer wrong-token' });
            assert.equal(wrong.status, 401);
            assert.equal(wrong.data.includes('schemaVersion'), false);
            assert.equal(wrong.data.includes('יחידה 1'), false);
            const missing = await post({});
            assert.equal(missing.status, 401);
            assert.equal(missing.data.includes('schemaVersion'), false);
        } finally {
            server.close();
        }
    });

    it('a plan tool reply does not include learner progress fields', async () => {
        resetPlan();
        const result = await handlers.read_part({ id: PLAN_ID, path: '' });
        assert.equal(result.isError, undefined);
        const text = replyText(result);
        assert.equal(text.includes('review_items'), false);
        assert.equal(text.includes('practice_events'), false);
        assert.equal(text.includes('lesson_progress'), false);
        const row = store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, pretty);
    });
});
