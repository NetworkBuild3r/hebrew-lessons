/**
 * Parity between the MCP tools and the admin routes (INIT-008/SPEC-010).
 * One operations layer, two adapters: every row of the capability table
 * (test/helpers/plan-parity-table.js) calls the same function through its tool
 * and through its route, with equivalent arguments. The checker is also run on
 * mutated copies of the table and of the registries, so the test cannot pass
 * by being blind. Temporary database and in-process servers only. No network
 * beyond them, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-010
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const harness = require('./helpers/signed-in-server');
const { ROWS, HUMAN_ONLY, findParityProblems, routeKey } = require('./helpers/plan-parity-table');

const PRETTY = fs.readFileSync(path.join(
    harness.REPO_ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json'
), 'utf8');
const ID = 'beginner-unit-1';
const LESSON = 'units/unit-1/lessons/cafe-order';
const WORD = LESSON + '/blocks/introduction/words/2';
const DIALOGUE = LESSON + '/blocks/dialogue';
const LISTEN = LESSON + '/blocks/listen';
const EXTRA = { id: 'extra', type: 'listening', passage: 'אֲנִי רוֹצֶה קָפֶה.' };
const LISTEN_BLOCK = { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' };

/** For each tool: the MCP arguments and the HTTP request that mean the same thing. */
const CALLS = {
    read_part: (rev) => ({
        mcp: { id: ID, path: WORD },
        http: { method: 'GET', url: '/api/plans/' + ID + '/parts?path=' + encodeURIComponent(WORD) }
    }),
    replace_part: (rev) => ({
        mcp: { id: ID, path: LISTEN, value: LISTEN_BLOCK, expectedRevision: rev },
        http: { method: 'PUT', url: '/api/plans/' + ID + '/parts', body: { path: LISTEN, value: LISTEN_BLOCK, expectedRevision: rev } }
    }),
    insert_block: (rev) => ({
        mcp: { id: ID, path: LESSON, index: 1, block: EXTRA, expectedRevision: rev },
        http: { method: 'POST', url: '/api/plans/' + ID + '/blocks', body: { path: LESSON, index: 1, block: EXTRA, expectedRevision: rev } }
    }),
    move_block: (rev) => ({
        mcp: { id: ID, path: DIALOGUE, index: 0, expectedRevision: rev },
        http: { method: 'POST', url: '/api/plans/' + ID + '/blocks/move', body: { path: DIALOGUE, index: 0, expectedRevision: rev } }
    }),
    remove_block: (rev) => ({
        mcp: { id: ID, path: DIALOGUE, expectedRevision: rev },
        http: { method: 'DELETE', url: '/api/plans/' + ID + '/parts', body: { path: DIALOGUE, expectedRevision: rev } }
    }),
    set_field: (rev) => ({
        mcp: { id: ID, path: WORD, field: 'meaning', value: 'espresso', expectedRevision: rev },
        http: { method: 'PATCH', url: '/api/plans/' + ID + '/fields', body: { path: WORD, field: 'meaning', value: 'espresso', expectedRevision: rev } }
    }),
    publish_plan: (rev) => ({
        mcp: { id: ID, expectedRevision: rev },
        http: { method: 'POST', url: '/api/plans/' + ID + '/publish', body: { expectedRevision: rev } }
    }),
    list_plans: (rev) => ({
        mcp: { limit: 5 },
        http: { method: 'GET', url: '/api/plans?limit=5' }
    }),
    create_plan: (rev) => ({
        mcp: { id: 'parity-new', title: 'Parity new', recipe: 'guide', unitId: 'u-one', lessonId: 'l-one' },
        http: {
            method: 'POST',
            url: '/api/plans',
            body: { id: 'parity-new', title: 'Parity new', recipe: 'guide', unitId: 'u-one', lessonId: 'l-one' }
        }
    }),
    assign_plan: (rev) => ({
        mcp: { id: ID, audience: 'personal', assigneeId: 'learner', expectedRevision: rev },
        http: { method: 'POST', url: '/api/plans/' + ID + '/assign', body: { audience: 'personal', assigneeId: 'learner', expectedRevision: rev } }
    }),
    list_plan_revisions: (rev) => ({
        mcp: { id: ID, sinceRevision: 1, beforeRevision: 9, actorKind: 'system', pathPrefix: 'units', limit: 10 },
        http: {
            method: 'GET',
            url: '/api/plans/' + ID + '/revisions?sinceRevision=1&beforeRevision=9&actorKind=system&pathPrefix=units&limit=10'
        }
    }),
    get_plan_revision: (rev) => ({
        mcp: { id: ID, revision: 1 },
        http: { method: 'GET', url: '/api/plans/' + ID + '/revisions/1' }
    }),
    diff_plan: (rev) => ({
        mcp: { id: ID, from: 1, to: 'working', path: LESSON },
        http: { method: 'GET', url: '/api/plans/' + ID + '/diff?from=1&to=working&path=' + encodeURIComponent(LESSON) }
    }),
    restore_plan_revision: (rev) => ({
        mcp: { id: ID, fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: rev },
        http: {
            method: 'POST',
            url: '/api/plans/' + ID + '/restore',
            body: { fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: rev }
        }
    }),
    list_plan_issues: (rev) => ({
        mcp: { id: ID, severity: ['block', 'warn'], code: 'N1', pathPrefix: 'units', limit: 20 },
        http: { method: 'GET', url: '/api/plans/' + ID + '/issues?severity=block,warn&code=N1&pathPrefix=units&limit=20' }
    })
};

/** The preview row reads the plan root through read_part. */
const PREVIEW = () => ({
    mcp: { id: ID, path: '' },
    http: { method: 'GET', url: '/api/plans/' + ID + '/parts' }
});

/**
 * The part of a function's call that two adapters must agree on: every argument,
 * with the store and the context replaced by labels.
 */
function normalize(args, isContext) {
    return args.map((arg) => {
        if (arg && typeof arg === 'object' && typeof arg.loadWorkingCopy === 'function') return '<store>';
        if (isContext(arg)) return '<context>';
        return arg;
    });
}

/**
 * Plan-builder code a handler must not contain: it parses or edits plan JSON,
 * touches the database, or writes the store. The adapters only authenticate,
 * call one function, and map the result.
 * @param {string} source
 * @returns {string[]}
 */
function findEditingCalls(source) {
    const banned = [
        [/JSON\.(?:parse|stringify)\(/, 'JSON.parse or JSON.stringify'],
        [/\.(?:units|lessons|blocks|words|paragraphs|lines|questions)\b/, 'a plan collection'],
        [/\.(?:splice|push|unshift|shift|pop)\(/, 'an array edit'],
        [/\bsaveWorkingCopy\b|\bsaveSnapshot\b/, 'a store write'],
        [/\bdb\.(?:run|get|all|exec|transaction)\b/, 'a database call'],
        [/\bplan\.[A-Za-z_]+\s*=[^=]/, 'an assignment into a plan'],
        [/\bdocument\b\s*=[^=]/, 'an assignment to a document']
    ];
    return banned.filter(([re]) => re.test(source)).map(([, what]) => what);
}

/** The text between two markers, or '' (a missing marker fails the caller's assertion). */
function between(source, start, end) {
    const from = source.indexOf(start);
    if (from === -1) return '';
    const to = source.indexOf(end, from + start.length);
    return to === -1 ? '' : source.slice(from, to);
}

describe('plan builder parity (INIT-008/SPEC-010)', { concurrency: false }, () => {
    let baseUrl;
    let adminCookie;
    let db;
    let store;
    let ops;
    let context;
    let tools;
    let handlers;
    let metas;
    let router;
    let planRoutes;
    let prevToken;
    let restore = [];
    let armed = null;
    let log = [];

    function moduleOf(name) {
        return require('../server/plans/' + name);
    }

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        db = require('../server/db');
        const accounts = require('../server/accounts');
        const cookies = require('../server/http/cookies');
        accounts.createUser({
            username: 'example-admin', password: harness.GOOD_PASSWORD, role: 'admin', displayName: 'Example Admin', actor: 'test'
        });
        const session = accounts.signIn('example-admin', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        adminCookie = cookies.cookieName() + '=' + encodeURIComponent(session.token);

        store = moduleOf('store');
        ops = moduleOf('ops');
        context = moduleOf('context');
        planRoutes = require('../server/api/plan-routes');

        prevToken = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'test-plan-parity-token';
        tools = require('../mcp/tools');
        metas = Object.create(null);
        tools.registerLessonTools({ registerTool(name, meta) { metas[name] = meta; } });
        handlers = tools.toolHandlers;

        const { createRouter } = require('../server/api/router');
        router = createRouter();
        planRoutes.register(router);
        require('../server/api/admin-routes').register(router);
        require('../server/api/media-routes').register(router);

        // A spy on each function a row names. It records only while a row is armed.
        for (const row of ROWS) {
            const mod = moduleOf(row.module);
            const key = row.module + '.' + row.fn;
            if (mod[row.fn].__parity) continue;
            const original = mod[row.fn];
            const wrapped = function parityWrapped(...args) {
                if (armed === key) log.push({ key, args });
                return original.apply(this, args);
            };
            wrapped.__parity = true;
            mod[row.fn] = wrapped;
            restore.push(() => { mod[row.fn] = original; });
        }
    });

    after(async () => {
        for (const undo of restore) undo();
        restore = [];
        if (prevToken === undefined) delete process.env.MCP_AUTH_TOKEN;
        else process.env.MCP_AUTH_TOKEN = prevToken;
        await harness.stop();
    });

    /** A plan at revision 2 with a baseline row, so revisions 1 and 2 both exist. */
    function prepare() {
        armed = null;
        db.run('DELETE FROM plan_revisions');
        db.run('DELETE FROM plan_field_marks');
        db.run('DELETE FROM plans');
        const plan = JSON.parse(PRETTY);
        store.saveWorkingCopy({
            id: plan.id, document: PRETTY, revision: plan.revision, status: plan.status,
            audience: plan.audience, assigneeId: plan.assigneeId
        });
        const seeded = ops.setField(store, ID, {
            path: WORD, field: 'meaning', value: 'latte', expectedRevision: 1
        }, context.makeContext({ channel: 'system', name: 'parity-setup', tool: 'test' }));
        assert.equal(seeded.error, undefined, JSON.stringify(seeded));
        return seeded.revision;
    }

    async function http(request) {
        const headers = { Cookie: adminCookie };
        if (request.body !== undefined) headers['Content-Type'] = 'application/json';
        if (request.method !== 'GET') headers.Origin = baseUrl;
        const res = await fetch(baseUrl + request.url, {
            method: request.method,
            headers,
            body: request.body === undefined ? undefined : JSON.stringify(request.body)
        });
        await res.text();
        return res.status;
    }

    const registry = () => ({
        tools: Object.keys(metas),
        routes: router.routes.map((r) => r.method + ' ' + r.path),
        writeRoutes: planRoutes.WRITE_ROUTES.map((r) => r.method + ' ' + r.path),
        functions: (module, fn) => typeof moduleOf(module)[fn] === 'function'
    });

    it('ac-2: the real table has no problem against the real registries', () => {
        assert.deepEqual(findParityProblems(ROWS, registry()), []);
        assert.equal(ROWS.length, 17);
        assert.equal(ROWS.filter((row) => row.tool && row.route).length, 16);
    });

    it('ac-2: each row of both sides calls the same function through its tool and its route', async () => {
        const isContext = context.isContext;
        const rows = ROWS.filter((row) => row.tool && row.route);
        for (const row of rows) {
            const key = row.module + '.' + row.fn;
            const make = row.operation.startsWith('preview') ? PREVIEW : CALLS[row.tool];
            assert.equal(typeof make, 'function', 'a call example for ' + row.tool);

            let rev = prepare();
            let call = make(rev);
            log = [];
            armed = key;
            const result = await handlers[row.tool](call.mcp, { requestId: 41 });
            armed = null;
            assert.equal(typeof result.content[0].text, 'string');
            const viaTool = log.slice();

            rev = prepare();
            call = make(rev);
            log = [];
            armed = key;
            const status = await http(call.http);
            armed = null;
            assert.equal(status < 500, true, row.tool + ' status ' + status);
            const viaRoute = log.slice();

            assert.equal(viaTool.length, 1, row.tool + ' once through the tool');
            assert.equal(viaRoute.length, 1, row.tool + ' once through the route');
            assert.deepEqual(normalize(viaTool[0].args, isContext), normalize(viaRoute[0].args, isContext), row.tool);

            const toolCtx = viaTool[0].args.find(isContext);
            const routeCtx = viaRoute[0].args.find(isContext);
            if (row.module === 'validate') {
                assert.equal(toolCtx, undefined);
            } else if (row.fn === 'readPart') {
                // readPart takes a context last; both adapters pass one.
                assert.equal(Boolean(toolCtx && routeCtx), true, row.tool);
            }
            if (toolCtx) {
                assert.deepEqual({ kind: toolCtx.actor.kind, name: toolCtx.actor.name, tool: toolCtx.actor.tool },
                    { kind: 'ai', name: 'mcp', tool: row.tool });
                assert.deepEqual({ kind: routeCtx.actor.kind, name: routeCtx.actor.name, tool: routeCtx.actor.tool },
                    { kind: 'human', name: 'example-admin', tool: row.tool });
            }
        }
    });

    it('ac-2: the schema route calls schemaText once and has no tool', async () => {
        const row = ROWS.find((r) => r.fn === 'schemaText');
        assert.equal(row.tool, null);
        assert.equal(typeof row.exemption, 'string');
        assert.equal(metas.get_plan_schema, undefined);
        assert.equal(Object.keys(metas).some((name) => /schema/i.test(name)), false);
        log = [];
        armed = 'validate.schemaText';
        const status = await http({ method: 'GET', url: '/api/plan-schema' });
        armed = null;
        assert.equal(status, 200);
        assert.equal(log.length, 1);
    });

    it('ac-2: the checker fails when a row loses a side, gains an unregistered side, or loses its write entry', () => {
        const real = registry();
        const without = (list, drop) => list.filter((item) => item !== drop);

        let problems = findParityProblems(ROWS, Object.assign({}, real, { tools: without(real.tools, 'restore_plan_revision') }));
        assert.equal(problems.some((p) => p.includes('tool not registered: restore_plan_revision')), true, problems.join('|'));

        problems = findParityProblems(ROWS, Object.assign({}, real, { routes: without(real.routes, 'GET /api/plans/:id/diff') }));
        assert.equal(problems.some((p) => p.includes('route not registered: GET /api/plans/:id/diff')), true, problems.join('|'));

        const ghostTool = ROWS.concat([{ operation: 'ghost', module: 'ops', fn: 'readPart', tool: 'ghost_tool', route: null, write: false, exemption: null }]);
        problems = findParityProblems(ghostTool, real);
        assert.equal(problems.some((p) => p.includes('tool not registered: ghost_tool')), true);
        assert.equal(problems.some((p) => p.includes('no route and no exemption: ghost')), true);

        const ghostRoute = ROWS.concat([{ operation: 'ghost', module: 'ops', fn: 'readPart', tool: null, route: { method: 'GET', path: '/api/plans/:id/ghost' }, write: false, exemption: null }]);
        problems = findParityProblems(ghostRoute, real);
        assert.equal(problems.some((p) => p.includes('route not registered: GET /api/plans/:id/ghost')), true);
        assert.equal(problems.some((p) => p.includes('no MCP tool and no exemption: ghost')), true);

        const noWrite = findParityProblems(ROWS, Object.assign({}, real, { writeRoutes: without(real.writeRoutes, 'POST /api/plans/:id/restore') }));
        assert.equal(noWrite.some((p) => p.includes('write route missing from WRITE_ROUTES: POST /api/plans/:id/restore')), true);

        const noFn = findParityProblems(ROWS, Object.assign({}, real, { functions: (m, f) => !(m === 'revisions' && f === 'diff') }));
        assert.equal(noFn.some((p) => p.includes('function not found: revisions.diff')), true);

        const both = ROWS.concat([{ operation: 'empty', module: 'ops', fn: 'readPart', tool: null, route: null, write: false, exemption: 'none' }]);
        assert.equal(findParityProblems(both, real).some((p) => p.includes('neither side exists: empty')), true);
    });

    it('ac-2: every mutating plan route registered is in WRITE_ROUTES, and every write row is registered', () => {
        const writes = new Set(planRoutes.WRITE_ROUTES.map((r) => r.method + ' ' + r.path));
        const mutating = router.routes.filter((r) => r.path.startsWith('/api/plans') && r.method !== 'GET');
        assert.equal(mutating.length >= 9, true);
        for (const route of mutating) {
            assert.equal(writes.has(route.method + ' ' + route.path), true, route.method + ' ' + route.path);
        }
        for (const entry of writes) {
            assert.equal(mutating.some((r) => r.method + ' ' + r.path === entry), true, entry);
        }
        for (const row of ROWS.filter((r) => r.write)) {
            assert.equal(writes.has(routeKey(row.route)), true, row.operation);
        }
        // Every plan route is behind the session and admin guard, except the learner reads.
        const open = router.routes.filter((r) => r.path.startsWith('/api/plan'))
            .filter((r) => r.path === '/api/plans/catalog' || r.path === '/api/plans/:id/published');
        assert.equal(open.length, 2);
    });

    it('ac-2: no handler edits plan JSON, opens the database, or writes the store', () => {
        const tools = fs.readFileSync(path.join(harness.REPO_ROOT, 'mcp', 'tools.js'), 'utf8');
        const routes = fs.readFileSync(path.join(harness.REPO_ROOT, 'server', 'api', 'plan-routes.js'), 'utf8');

        const toolSegments = [
            between(tools, 'function registerPlanBuilderTools(server) {', 'function registerLessonTools(server) {'),
            between(tools, "// INIT-004/SPEC-005: same functions as the plan HTTP routes", "// INIT-004/SPEC-012")
        ];
        const routeSegments = [
            between(routes, 'function registerBuilderRoutes(router) {', ' * Register plan routes on a router'),
            between(routes, 'function register(router) {', 'registerBuilderRoutes(router);')
        ];
        for (const segment of toolSegments.concat(routeSegments)) {
            assert.equal(segment.length > 500, true, 'a handler segment was found');
            assert.deepEqual(findEditingCalls(segment), []);
        }
        // The assign check lives in personal.assign. No adapter reaches ops.assignPlan.
        assert.equal(/\bassignPlan\b/.test(toolSegments.join('') + routeSegments.join('')), false);
    });

    it('ac-2: the editing-call scan fails on a handler that edits plan JSON', () => {
        const seeded = [
            "const next = JSON.parse(JSON.stringify(plan));",
            "plan.units.push(block);",
            "store.saveWorkingCopy({ id });",
            "db.run('UPDATE plans SET x = 1');",
            "plan.title = 'x';"
        ].join('\n');
        const found = findEditingCalls(seeded);
        assert.equal(found.includes('JSON.parse or JSON.stringify'), true);
        assert.equal(found.includes('a plan collection'), true);
        assert.equal(found.includes('an array edit'), true);
        assert.equal(found.includes('a store write'), true);
        assert.equal(found.includes('a database call'), true);
        assert.equal(found.includes('an assignment into a plan'), true);
        assert.deepEqual(findEditingCalls('return revisions.list(id, opts, ctx);'), []);
    });

    it('the human-only acts stay human-only: the tool answers forbidden or does not exist', async () => {
        for (const entry of HUMAN_ONLY) {
            assert.equal(router.routes.some((r) => routeKey(r) === routeKey(entry.route)), true, entry.act);
            if (entry.tool) {
                const result = await handlers[entry.tool]({});
                assert.equal(result.isError, true, entry.tool);
                assert.deepEqual(JSON.parse(result.content[0].text), { error: 'forbidden' });
            } else {
                assert.equal(Object.keys(metas).some((name) => /approv/i.test(name)), false, entry.act);
            }
        }
        const named = Object.keys(metas).filter((name) => /verif|ai_planning|approv/i.test(name)).sort();
        assert.deepEqual(named, ['set_ai_planning', 'set_lexicon_verified']);
    });

    it('ac-7: the learner read of a draft, a hidden lesson, and an unassigned personal plan is 404', async () => {
        const learner = harness.cookieFor('learner');
        async function learnerRead(id) {
            const res = await fetch(baseUrl + '/api/plans/' + id + '/published', { headers: { Cookie: learner } });
            const text = await res.text();
            return { status: res.status, text };
        }
        const shared = (id, edit) => {
            const plan = JSON.parse(PRETTY);
            plan.id = id;
            if (edit) edit(plan);
            store.saveWorkingCopy({
                id, document: JSON.stringify(plan), revision: 1, status: plan.status,
                audience: plan.audience, assigneeId: plan.assigneeId
            });
            return plan;
        };
        const system = context.makeContext({ channel: 'system', name: 'parity-setup', tool: 'test' });

        shared('draft-parity', (plan) => plan.units.forEach((u) => u.lessons.forEach((l) => { l.visibility = 'everyone'; })));
        const draft = await learnerRead('draft-parity');
        assert.equal(draft.status, 404);
        assert.equal(draft.text.includes('newWords'), false);

        shared('hidden-parity');
        assert.equal(moduleOf('publish').publishPlan(store, 'hidden-parity', { expectedRevision: 1 }, system).error, undefined);
        const hidden = await learnerRead('hidden-parity');
        assert.equal(hidden.status, 404);
        assert.equal(hidden.text, draft.text);

        shared('personal-parity', (plan) => {
            plan.audience = 'personal';
            plan.assigneeId = 'someone-else';
            plan.units.forEach((u) => u.lessons.forEach((l) => { l.visibility = 'everyone'; }));
        });
        assert.equal(moduleOf('publish').publishPlan(store, 'personal-parity', { expectedRevision: 1 }, system).error, undefined);
        const personal = await learnerRead('personal-parity');
        assert.equal(personal.status, 404);
        assert.equal(personal.text.includes('assigneeId'), false);
    });
});
