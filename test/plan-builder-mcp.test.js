/**
 * Plan-builder MCP tools (INIT-008/SPEC-009): list_plans, create_plan,
 * list_plan_revisions, get_plan_revision, diff_plan, restore_plan_revision,
 * list_plan_issues, assign_plan. The tools are called through a real MCP server
 * and client joined in memory, so the SDK validates each argument as it does in
 * production. Temporary database through the signed-in server harness. No
 * network beyond the in-process servers, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-009
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const http = require('http');
const path = require('path');

const harness = require('./helpers/signed-in-server');

const PRETTY = fs.readFileSync(path.join(
    harness.REPO_ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json'
), 'utf8');
const SNAPSHOT = JSON.parse(fs.readFileSync(path.join(__dirname, 'fixtures', 'mcp-tools-before-spec-009.json'), 'utf8'));
const ID = 'beginner-unit-1';
const LESSON = 'units/unit-1/lessons/cafe-order';
const WORD = LESSON + '/blocks/introduction/words/2';
const TOKEN = 'test-plan-builder-mcp-token';
const NEW_TOOLS = [
    'list_plans', 'create_plan', 'list_plan_revisions', 'get_plan_revision',
    'diff_plan', 'restore_plan_revision', 'list_plan_issues', 'assign_plan'
];
/** Tools added after the eight above. They are checked in their own tests. */
const LATER_TOOLS = ['insert_lesson', 'move_lesson', 'remove_lesson', 'suggest_glosses', 'make_word_audio'];
const NOT_NFC = 'א\u05B7ת\u05BC\u05B8ה';

describe('plan builder MCP tools (INIT-008/SPEC-009)', { concurrency: false }, () => {
    let baseUrl;
    let adminCookie;
    let db;
    let store;
    let tools;
    let listed;
    let client;
    let server;
    let prevToken;
    let prevPort;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        db = require('../server/db');
        const accounts = require('../server/accounts');
        const cookies = require('../server/http/cookies');
        accounts.createUser({
            username: 'example-admin', password: harness.GOOD_PASSWORD, role: 'admin', displayName: 'Example Admin', actor: 'test'
        });
        accounts.createUser({
            username: 'example-learner', password: harness.GOOD_PASSWORD, role: 'user', displayName: 'Example Learner', actor: 'test'
        });
        const retired = accounts.createUser({
            username: 'example-retired', password: harness.GOOD_PASSWORD, role: 'user', displayName: 'Example Retired', actor: 'test'
        });
        accounts.setStatus(retired.id, 'disabled', 'test');
        const session = accounts.signIn('example-admin', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        adminCookie = cookies.cookieName() + '=' + encodeURIComponent(session.token);
        store = require('../server/plans/store');

        prevToken = process.env.MCP_AUTH_TOKEN;
        prevPort = process.env.MCP_PORT;
        process.env.MCP_AUTH_TOKEN = TOKEN;
        process.env.MCP_PORT = '0';

        const { McpServer } = require('@modelcontextprotocol/sdk/server/mcp.js');
        const { Client } = require('@modelcontextprotocol/sdk/client/index.js');
        const { InMemoryTransport } = require('@modelcontextprotocol/sdk/inMemory.js');
        tools = require('../mcp/tools');
        server = new McpServer({ name: 'plan-builder-test', version: '1' });
        tools.registerLessonTools(server);
        const [a, b] = InMemoryTransport.createLinkedPair();
        client = new Client({ name: 'plan-builder-test-client', version: '1' });
        await Promise.all([server.connect(a), client.connect(b)]);
        listed = (await client.listTools()).tools;
    });

    after(async () => {
        try { await client.close(); } catch (_) { /* closed */ }
        try { await server.close(); } catch (_) { /* closed */ }
        if (prevToken === undefined) delete process.env.MCP_AUTH_TOKEN;
        else process.env.MCP_AUTH_TOKEN = prevToken;
        if (prevPort === undefined) delete process.env.MCP_PORT;
        else process.env.MCP_PORT = prevPort;
        await harness.stop();
    });

    beforeEach(() => {
        db.run('DELETE FROM plan_revisions');
        db.run('DELETE FROM plan_field_marks');
        db.run('DELETE FROM plans');
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM media');
    });

    function seed() {
        const plan = JSON.parse(PRETTY);
        store.saveWorkingCopy({
            id: plan.id, document: PRETTY, revision: plan.revision, status: plan.status,
            audience: plan.audience, assigneeId: plan.assigneeId
        });
        return store.loadWorkingCopy(plan.id);
    }

    /** Call a tool through the real client. Returns the parsed JSON body when there is one. */
    async function call(name, args) {
        const result = await client.callTool({ name, arguments: args || {} });
        const text = result.content && result.content[0] ? result.content[0].text : '';
        let body = null;
        try { body = JSON.parse(text); } catch (_) { /* a plain message */ }
        return { isError: result.isError === true, text, body };
    }

    async function adminApi(method, urlPath, body) {
        const headers = { Cookie: adminCookie };
        if (body !== undefined) headers['Content-Type'] = 'application/json';
        if (method !== 'GET') headers.Origin = baseUrl;
        const res = await fetch(baseUrl + urlPath, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
        const text = await res.text();
        return { status: res.status, json: JSON.parse(text) };
    }

    function tables() {
        return JSON.stringify({
            plans: db.all('SELECT id, revision, working_copy, snapshot FROM plans ORDER BY id'),
            revisions: db.all('SELECT plan_id, revision, op FROM plan_revisions ORDER BY plan_id, revision'),
            audit: db.all('SELECT actor, action, target FROM audit_log ORDER BY id'),
            marks: db.all('SELECT plan_id, path, field FROM plan_field_marks ORDER BY plan_id, path, field')
        });
    }

    it('ac-1: the eight new tools are listed once each with the annotations; the existing tools are unchanged', () => {
        const names = listed.map((t) => t.name);
        for (const name of NEW_TOOLS) {
            assert.equal(names.filter((n) => n === name).length, 1, name);
        }
        const byName = Object.fromEntries(listed.map((t) => [t.name, t]));
        for (const name of ['list_plans', 'list_plan_revisions', 'get_plan_revision', 'diff_plan', 'list_plan_issues']) {
            assert.equal(byName[name].annotations.readOnlyHint, true, name);
        }
        for (const name of ['create_plan', 'assign_plan']) {
            assert.notEqual(byName[name].annotations && byName[name].annotations.readOnlyHint, true, name);
        }
        assert.equal(byName.restore_plan_revision.annotations.destructiveHint, false);
        assert.notEqual(byName.restore_plan_revision.annotations.readOnlyHint, true);
        for (const name of NEW_TOOLS) {
            assert.equal(typeof byName[name].description, 'string');
            assert.equal(byName[name].description.length > 100, true, name);
            assert.match(byName[name].description, /MCP_AUTH_TOKEN/);
        }

        // Snapshot taken before this change: every earlier tool keeps its name, input schema, and annotations.
        const before = Object.keys(SNAPSHOT);
        assert.equal(before.length, 27);
        for (const name of before) {
            assert.ok(byName[name], name + ' is still registered');
            assert.deepEqual(byName[name].inputSchema, SNAPSHOT[name].inputSchema, name + ' input schema');
            assert.deepEqual(byName[name].annotations || null, SNAPSHOT[name].annotations, name + ' annotations');
        }
        assert.deepEqual(names.filter((n) => !before.includes(n)).sort(), NEW_TOOLS.concat(LATER_TOOLS).sort());
    });

    it('the editor route drafts meanings for an admin and refuses everyone else', async () => {
        require('../server/db/repos/dictionary').replaceEntries([
            { he: 'כלב', heFold: 'כלב', pos: 'masculine noun', en: 'dog, mutt' }
        ]);
        const ok = await adminApi('POST', '/api/dictionary/suggest-glosses', { text: 'הכלב כלב', skip: ['כלב'] });
        assert.equal(ok.status, 200);
        assert.deepEqual(ok.json.suggestions.map((s) => [s.word, s.meaning, s.form]), [['הכלב', 'dog, mutt', 'כלב']]);
        assert.equal(ok.json.skipped, 1);

        const bad = await adminApi('POST', '/api/dictionary/suggest-glosses', { text: 5 });
        assert.equal(bad.status, 400);
        const extra = await adminApi('POST', '/api/dictionary/suggest-glosses', { text: 'כלב', write: true });
        assert.equal(extra.status, 400);

        const anon = await fetch(baseUrl + '/api/dictionary/suggest-glosses', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', Origin: baseUrl },
            body: JSON.stringify({ text: 'כלב' })
        });
        assert.equal(anon.status, 401);
    });

    it('suggest_glosses drafts meanings for a story, refuses without a token, and never writes', async () => {
        require('../server/db/repos/dictionary').replaceEntries([
            { he: 'בית', heFold: 'בית', pos: 'masculine noun', en: 'house, home, residence' }
        ]);
        seed();
        const inserted = await call('insert_block', {
            id: ID, path: LESSON, index: 1, expectedRevision: 1,
            block: {
                id: 'story',
                type: 'graded-story',
                paragraphs: [{ id: 'p1', text: 'הַבַּיִת בית צצצ' }],
                glosses: { שלום: 'hello' }
            }
        });
        assert.equal(inserted.isError, false, inserted.text);
        const before = tables();

        const draft = await call('suggest_glosses', { id: ID, path: LESSON + '/blocks/story' });
        assert.equal(draft.isError, false, draft.text);
        assert.deepEqual(draft.body.suggestions.map((s) => [s.word, s.meaning]), [
            ['הַבַּיִת', 'house, home, residence'],
            ['בית', 'house, home, residence']
        ]);
        assert.equal(draft.body.suggestions[0].form, 'בית', 'the definite form says which word it is a form of');
        assert.equal(draft.body.suggestions[1].form, null);
        assert.deepEqual(draft.body.unmatched, ['צצצ']);
        assert.equal(typeof draft.body.revision, 'number');
        assert.equal(tables(), before, 'a draft changes nothing');

        const notStory = await call('suggest_glosses', { id: ID, path: LESSON });
        assert.equal(notStory.isError, true);

        delete process.env.MCP_AUTH_TOKEN;
        try {
            const refused = await call('suggest_glosses', { id: ID, path: LESSON + '/blocks/story' });
            assert.equal(refused.isError, true);
            assert.match(refused.text, /MCP_AUTH_TOKEN/);
            assert.equal(refused.text.includes('house'), false);
        } finally {
            process.env.MCP_AUTH_TOKEN = TOKEN;
        }
    });

    /** A word clip already in the (temporary) shared word folder. No ElevenLabs call is made. */
    function seedWordClip(word, file) {
        const audio = require('../server/audio');
        const dir = require('../server/paths').wordAudioDir();
        fs.mkdirSync(dir, { recursive: true });
        fs.writeFileSync(path.join(dir, file), Buffer.from('fake-mp3'));
        const manifest = audio.readWordManifest(dir);
        manifest.entries['v|m|' + word] = { text: word, file, voiceId: 'v', model: 'm', bytes: 8 };
        audio.writeWordManifest(dir, manifest);
    }

    it('the editor routes list and reuse word sound for an admin, and refuse bad words and everyone else', async () => {
        seedWordClip('קָפֶה', 'aaaaaaaaaaaaaaaaaaaaaaaa.mp3');

        const listedClips = await adminApi('GET', '/api/admin/word-audio');
        assert.equal(listedClips.status, 200);
        assert.deepEqual(listedClips.json.clips, [{
            word: 'קָפֶה',
            audioPath: '../_audio/words/aaaaaaaaaaaaaaaaaaaaaaaa.mp3',
            url: '/worksheets/_audio/words/aaaaaaaaaaaaaaaaaaaaaaaa.mp3'
        }]);

        const reused = await adminApi('POST', '/api/admin/word-audio', { text: 'קָפֶה' });
        assert.equal(reused.status, 200);
        assert.equal(reused.json.audioPath, '../_audio/words/aaaaaaaaaaaaaaaaaaaaaaaa.mp3');
        assert.equal(reused.json.created, false);

        const english = await adminApi('POST', '/api/admin/word-audio', { text: 'coffee' });
        assert.equal(english.status, 400);
        assert.equal(english.json.code, 'bad_word');
        const extra = await adminApi('POST', '/api/admin/word-audio', { text: 'קפה', force: true });
        assert.equal(extra.status, 400);
        const notText = await adminApi('POST', '/api/admin/word-audio', { text: 5 });
        assert.equal(notText.status, 400);

        const anon = await fetch(baseUrl + '/api/admin/word-audio', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', Origin: baseUrl },
            body: JSON.stringify({ text: 'קפה' })
        });
        assert.equal(anon.status, 401);
        const anonList = await fetch(baseUrl + '/api/admin/word-audio');
        assert.equal(anonList.status, 401);
    });

    it('make_word_audio reuses a clip, names a bad word, refuses without a token, and never writes a plan', async () => {
        seed();
        seedWordClip('שָׁלוֹם', 'bbbbbbbbbbbbbbbbbbbbbbbb.mp3');
        const before = tables();

        const out = await call('make_word_audio', { words: ['שָׁלוֹם', 'dog'] });
        assert.equal(out.isError, false, out.text);
        assert.deepEqual(out.body.clips, [
            { word: 'שָׁלוֹם', audioPath: '../_audio/words/bbbbbbbbbbbbbbbbbbbbbbbb.mp3', created: false },
            { word: 'dog', error: 'bad_word' }
        ]);
        assert.equal(tables(), before);

        const tooMany = await call('make_word_audio', { words: new Array(21).fill('שלום') });
        assert.equal(tooMany.isError, true);

        delete process.env.MCP_AUTH_TOKEN;
        try {
            const refused = await call('make_word_audio', { words: ['שָׁלוֹם'] });
            assert.equal(refused.isError, true);
            assert.match(refused.text, /MCP_AUTH_TOKEN/);
            assert.equal(refused.text.includes('bbbbbbbb'), false);
        } finally {
            process.env.MCP_AUTH_TOKEN = TOKEN;
        }
    });

    it('ac-2: every new tool refuses without MCP_AUTH_TOKEN and returns no plan content', async () => {
        seed();
        const before = tables();
        const calls = {
            list_plans: {},
            create_plan: { id: 'no-token', title: 'No token' },
            list_plan_revisions: { id: ID },
            get_plan_revision: { id: ID, revision: 1 },
            diff_plan: { id: ID, from: 1, to: 'working' },
            restore_plan_revision: { id: ID, fromRevision: 1, expectedRevision: 1 },
            list_plan_issues: { id: ID },
            assign_plan: { id: ID, audience: 'shared', expectedRevision: 1 }
        };
        assert.deepEqual(Object.keys(calls).sort(), NEW_TOOLS.slice().sort());
        delete process.env.MCP_AUTH_TOKEN;
        try {
            for (const name of NEW_TOOLS) {
                const res = await call(name, calls[name]);
                assert.equal(res.isError, true, name);
                assert.match(res.text, /MCP_AUTH_TOKEN/, name);
                assert.equal(res.text.includes('schemaVersion'), false, name);
                assert.equal(res.text.includes('יחידה'), false, name);
                assert.equal(res.text.includes('beginner-unit-1'), false, name);
            }
        } finally {
            process.env.MCP_AUTH_TOKEN = TOKEN;
        }
        assert.equal(tables(), before);
    });

    it('ac-2: each tool calls its one function with an ai context named mcp and its own tool name', async () => {
        const modules = {
            list_plans: ['create', 'listPlans'],
            create_plan: ['create', 'createPlan'],
            list_plan_revisions: ['revisions', 'list'],
            get_plan_revision: ['revisions', 'get'],
            diff_plan: ['revisions', 'diff'],
            restore_plan_revision: ['revisions', 'restore'],
            list_plan_issues: ['issues', 'list'],
            assign_plan: ['personal', 'assign']
        };
        const { isContext } = require('../server/plans/context');
        const seen = [];
        const undo = [];
        for (const [tool, [moduleName, fn]] of Object.entries(modules)) {
            const mod = require('../server/plans/' + moduleName);
            const original = mod[fn];
            mod[fn] = function spy(...args) {
                seen.push({ tool, fn: moduleName + '.' + fn, ctx: args.find(isContext) });
                return original.apply(this, args);
            };
            undo.push(() => { mod[fn] = original; });
        }
        try {
            const rev = seed().revision;
            const calls = [
                ['list_plans', {}],
                ['create_plan', { id: 'spy-new', title: 'Spy new' }],
                ['list_plan_revisions', { id: ID }],
                ['get_plan_revision', { id: ID, revision: 1 }],
                ['diff_plan', { id: ID, from: 1, to: 'working' }],
                ['list_plan_issues', { id: ID }],
                ['assign_plan', { id: ID, audience: 'shared', expectedRevision: rev }],
                ['restore_plan_revision', { id: ID, fromRevision: 1, expectedRevision: rev + 1 }]
            ];
            for (const [tool, args] of calls) {
                seen.length = 0;
                const res = await call(tool, args);
                assert.notEqual(res.body && res.body.error, 'invalid_input', tool);
                assert.equal(seen.filter((s) => s.tool === tool).length, 1, tool + ' calls its function once');
                const entry = seen.find((s) => s.tool === tool);
                assert.equal(entry.fn, modules[tool][0] + '.' + modules[tool][1]);
                assert.deepEqual(
                    { kind: entry.ctx.actor.kind, name: entry.ctx.actor.name, tool: entry.ctx.actor.tool },
                    { kind: 'ai', name: 'mcp', tool }
                );
            }
        } finally {
            for (const fn of undo) fn();
        }
    });

    it('ac-3: no input schema can name an actor, and an actor argument is not forwarded', async () => {
        for (const tool of listed.filter((t) => NEW_TOOLS.includes(t.name) || ['set_field', 'replace_part', 'read_part'].includes(t.name))) {
            const keys = Object.keys(tool.inputSchema.properties || {});
            for (const forbidden of ['actor', 'by', 'kind', 'client', 'username', 'name', 'ctx', 'context']) {
                assert.equal(keys.includes(forbidden), false, tool.name + ' ' + forbidden);
            }
        }
        const rev = seed().revision;
        const forged = { actor: { kind: 'human', name: 'example-admin' }, by: 'admin:example-admin', kind: 'human', client: 'x' };
        const handlers = tools.toolHandlers;
        const setField = await handlers.set_field(Object.assign({
            id: ID, path: WORD, field: 'meaning', value: 'espresso', expectedRevision: rev
        }, forged));
        assert.equal(setField.isError, undefined);
        const restored = await handlers.restore_plan_revision(Object.assign({
            id: ID, fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: rev + 1
        }, forged));
        assert.equal(restored.isError, undefined, restored.content[0].text);
        const created = await handlers.create_plan(Object.assign({ id: 'forged-new', title: 'Forged new' }, forged));
        assert.equal(created.isError, undefined);
        const assigned = await handlers.assign_plan(Object.assign({
            id: ID, audience: 'shared', expectedRevision: rev + 2
        }, forged));
        assert.equal(assigned.isError, undefined, assigned.content[0].text);
        for (const row of db.all('SELECT actor_kind, actor_name FROM plan_revisions')) {
            if (row.actor_name === 'baseline') continue;
            assert.deepEqual([row.actor_kind, row.actor_name], ['ai', 'mcp']);
        }
        for (const row of db.all('SELECT actor FROM audit_log')) assert.equal(row.actor, 'mcp');
    });

    it('ac-4: create, read, list, assign, issues, revisions, diff, and restore all work through the MCP alone', async () => {
        const created = await call('create_plan', { id: 'cafe-mcp', title: 'Cafe by MCP', recipe: 'topic-conversation' });
        assert.equal(created.isError, false, created.text);
        assert.equal(created.body.revision, 1);
        assert.equal(created.body.status, 'draft');
        assert.equal(created.body.plan.units[0].lessons[0].recipe, 'topic-conversation');
        const again = await call('create_plan', { id: 'cafe-mcp', title: 'Other title' });
        assert.equal(again.isError, true);
        assert.equal(again.body.error, 'already_exists');
        assert.equal(JSON.parse(db.get('SELECT working_copy FROM plans WHERE id = ?', 'cafe-mcp').working_copy).title, 'Cafe by MCP');
        const badId = await call('create_plan', { id: 'Not A Slug', title: 'x' });
        assert.equal(badId.isError, true);
        assert.equal(badId.body.error, 'invalid_id');
        const badRecipe = await call('create_plan', { id: 'recipe-bad', title: 'x', recipe: 'nonsense' });
        assert.equal(badRecipe.body.error, 'invalid_recipe');
        const lessonIds = await call('create_plan', { id: 'ids-set', title: 'Ids', unitId: 'unit-a', lessonId: 'lesson-a' });
        assert.equal(lessonIds.body.plan.units[0].lessons[0].id, 'lesson-a');

        const read = await call('read_part', { id: 'cafe-mcp', path: '' });
        assert.equal(read.body.revision, 1);
        assert.equal(read.body.part.id, 'cafe-mcp');
        assert.equal(read.body.editState.humanEdited.length, 0);

        const list = await call('list_plans', {});
        assert.equal(list.isError, false);
        const row = list.body.plans.find((p) => p.id === 'cafe-mcp');
        assert.deepEqual(Object.keys(row).sort(), [
            'assigned', 'audience', 'humanEditedFields', 'id', 'published', 'revision', 'status', 'title', 'unpublishedChanges', 'updatedAt'
        ]);
        assert.equal(row.assigned, false);
        const limited = await call('list_plans', { limit: 1 });
        assert.equal(limited.body.plans.length, 1);
        assert.equal(limited.body.truncated, true);

        const assigned = await call('assign_plan', { id: 'cafe-mcp', audience: 'personal', assigneeId: 'example-learner', expectedRevision: 1 });
        assert.equal(assigned.isError, false, assigned.text);
        assert.equal(assigned.body.revision, 2);
        const listed2 = await call('list_plans', {});
        const row2 = listed2.body.plans.find((p) => p.id === 'cafe-mcp');
        assert.equal(row2.assigned, true);
        assert.equal(row2.audience, 'personal');
        assert.equal('assigneeId' in row2, false, 'no assignee name for an ai caller');
        assert.equal(listed2.text.includes('example-learner'), false);

        const revBefore = store.loadWorkingCopy('cafe-mcp');
        const tablesBefore = tables();
        const refused = await call('assign_plan', { id: 'cafe-mcp', audience: 'personal', assigneeId: 'example-retired', expectedRevision: 2 });
        assert.equal(refused.isError, true);
        assert.equal(refused.body.error, 'assignee_refused');
        assert.equal(refused.text.includes('example-retired'), false);
        const unknown = await call('assign_plan', { id: 'cafe-mcp', audience: 'personal', assigneeId: 'nobody-here', expectedRevision: 2 });
        assert.equal(unknown.body.error, 'assignee_refused');
        assert.equal(tables(), tablesBefore);
        assert.equal(store.loadWorkingCopy('cafe-mcp').revision, revBefore.revision);
        const wrongAudience = await call('assign_plan', { id: 'cafe-mcp', audience: 'everyone', expectedRevision: 2 });
        assert.equal(wrongAudience.isError, true, 'the schema refuses an unknown audience');
        assert.equal(tables(), tablesBefore);

        const issues = await call('list_plan_issues', { id: 'cafe-mcp' });
        assert.equal(issues.isError, false, issues.text);
        assert.equal(issues.body.revision, 2);
        assert.deepEqual(Object.keys(issues.body.counts).sort(), ['block', 'inform', 'warn']);
        assert.equal(Array.isArray(issues.body.issues), true);
        const all = await call('list_plan_issues', { id: 'cafe-mcp', severity: 'all', limit: 5 });
        assert.equal(all.isError, false);
        const some = await call('list_plan_issues', { id: 'cafe-mcp', severity: ['block', 'warn'], code: 'N1', pathPrefix: 'units' });
        assert.equal(some.isError, false);
        const gone = await call('list_plan_issues', { id: 'no-such-plan' });
        assert.equal(gone.body.error, 'not_found');
        const badSeverity = await call('list_plan_issues', { id: 'cafe-mcp', severity: 'bogus' });
        assert.equal(badSeverity.isError, true);

        const revs = await call('list_plan_revisions', { id: 'cafe-mcp' });
        assert.deepEqual(revs.body.revisions.map((r) => [r.revision, r.op, r.actorKind, r.actorName]), [
            [2, 'assign', 'ai', 'mcp'],
            [1, 'create', 'ai', 'mcp']
        ]);
        assert.equal(revs.body.revisions[0].tool, 'assign_plan');
        const since = await call('list_plan_revisions', { id: 'cafe-mcp', sinceRevision: 1, actorKind: 'ai', limit: 5 });
        assert.deepEqual(since.body.revisions.map((r) => r.revision), [2]);
        const none = await call('list_plan_revisions', { id: 'cafe-mcp', actorKind: 'human' });
        assert.deepEqual(none.body.revisions, []);
        const noPlan = await call('list_plan_revisions', { id: 'no-such-plan' });
        assert.equal(noPlan.body.error, 'not_found');
        const badKind = await call('list_plan_revisions', { id: 'cafe-mcp', actorKind: 'robot' });
        assert.equal(badKind.isError, true);

        const first = await call('get_plan_revision', { id: 'cafe-mcp', revision: 1 });
        assert.equal(first.isError, false);
        assert.equal(first.body.document.audience, 'shared');
        assert.equal(first.body.document.revision, 1);
        const missing = await call('get_plan_revision', { id: 'cafe-mcp', revision: 9 });
        assert.equal(missing.body.error, 'not_found');

        const diff = await call('diff_plan', { id: 'cafe-mcp', from: 1, to: 'working' });
        assert.equal(diff.isError, false, diff.text);
        assert.deepEqual(diff.body.from, { ref: 1, revision: 1 });
        assert.deepEqual(diff.body.to, { ref: 'working', revision: 2 });
        assert.equal(diff.body.entries.some((e) => e.path === '' && e.field === 'audience' && e.before === 'shared' && e.after === 'personal'), true);
        const noSnapshot = await call('diff_plan', { id: 'cafe-mcp', from: 'published', to: 'working' });
        assert.equal(noSnapshot.body.error, 'no_snapshot');
        const badRef = await call('diff_plan', { id: 'cafe-mcp', from: 'latest', to: 'working' });
        assert.equal(badRef.isError, true);

        const restored = await call('restore_plan_revision', { id: 'cafe-mcp', fromRevision: 1, expectedRevision: 2 });
        assert.equal(restored.isError, false, restored.text);
        assert.equal(restored.body.revision, 3);
        assert.equal(restored.body.restoredFrom, 1);
        assert.equal(restored.body.plan.audience, 'personal', 'a whole restore keeps the assignment');
        assert.equal(restored.body.plan.assigneeId, 'example-learner');
        const log = await call('list_plan_revisions', { id: 'cafe-mcp', limit: 1 });
        assert.equal(log.body.revisions[0].op, 'restore');
        assert.equal(log.body.revisions[0].actorKind, 'ai');
    });

    it('ac-5: after a person edits, read_part shows editState; an AI write is not refused and lists overwrote', async () => {
        const created = await call('create_plan', { id: 'edit-state', title: 'Edit state' });
        assert.equal(created.isError, false, created.text);
        const lesson = 'units/unit-1/lessons/lesson-1';
        const ai1 = await call('set_field', { id: 'edit-state', path: lesson, field: 'title', value: 'AI title', expectedRevision: 1 });
        assert.equal(ai1.isError, false, ai1.text);
        assert.equal(ai1.body.overwrote, undefined);
        const human = await adminApi('PATCH', '/api/plans/edit-state/fields', {
            path: lesson, field: 'title', value: 'Person title', expectedRevision: 2
        });
        assert.equal(human.status, 200);

        const read = await call('read_part', { id: 'edit-state', path: lesson });
        assert.equal(read.isError, false);
        assert.equal(read.body.editState.advisory, true);
        const edited = read.body.editState.humanEdited.find((e) => e.field === 'title');
        assert.equal(edited.previousValue, 'AI title');
        assert.equal(edited.previousWriter, 'ai');
        assert.equal(edited.by, undefined);
        assert.equal(read.text.includes('example-admin'), false, 'no account name reaches the MCP');
        assert.equal(read.body.editState.lastHumanEdit.revision, 3);

        const revs = await call('list_plan_revisions', { id: 'edit-state', actorKind: 'human' });
        assert.equal(revs.body.revisions[0].actorName, 'human');
        assert.equal(revs.text.includes('example-admin'), false);

        const overwrite = await call('set_field', { id: 'edit-state', path: lesson, field: 'title', value: 'AI title again', expectedRevision: 3 });
        assert.equal(overwrite.isError, false, 'an advisory note never refuses a write');
        assert.equal(overwrite.body.revision, 4);
        assert.equal(overwrite.body.overwrote.length, 1);
        assert.equal(overwrite.body.overwrote[0].field, 'title');
        assert.equal(overwrite.body.overwrote[0].valueSetByPerson, 'Person title');
        assert.equal(overwrite.text.includes('example-admin'), false);

        const restored = await call('restore_plan_revision', {
            id: 'edit-state', fromRevision: 3, path: lesson, field: 'title', expectedRevision: 4
        });
        assert.equal(restored.isError, false, restored.text);
        assert.equal(restored.body.plan.units[0].lessons[0].title, 'Person title');
    });

    it('ac-6: publish_plan publishes a plan whose only findings are warnings, with no approval argument', async () => {
        seed();
        const inserted = await call('insert_block', {
            id: ID, path: LESSON, index: 1, expectedRevision: 1,
            block: { id: 'extra', type: 'listening', passage: NOT_NFC }
        });
        assert.equal(inserted.isError, false, inserted.text);
        const issues = await call('list_plan_issues', { id: ID });
        assert.equal(issues.body.counts.block, 0);
        assert.equal(issues.body.counts.warn >= 1, true);
        assert.equal(issues.body.issues[0].code, 'pointing-form');
        assert.equal(issues.body.issues[0].fix.action, 'set_field');

        const meta = listed.find((t) => t.name === 'publish_plan');
        assert.deepEqual(Object.keys(meta.inputSchema.properties).sort(), ['expectedRevision', 'id']);
        assert.match(meta.description, /No person has to approve a publish/);
        const published = await call('publish_plan', { id: ID, expectedRevision: inserted.body.revision });
        assert.equal(published.isError, false, published.text);
        assert.equal(published.body.status, 'published');
        assert.equal(typeof store.loadSnapshot(ID), 'string');
    });

    it('ac-6: a block finding and a pending picture on an everyone lesson are still refused', async () => {
        seed();
        const words = JSON.parse(PRETTY).units[0].lessons[0].newWords.concat(['סֶפֶל']);
        const drafted = await call('set_field', { id: ID, path: LESSON, field: 'newWords', value: words, expectedRevision: 1 });
        assert.equal(drafted.isError, false, drafted.text);
        const blocked = await call('publish_plan', { id: ID, expectedRevision: 2 });
        assert.equal(blocked.isError, true);
        assert.equal(blocked.body.error, 'validation');
        assert.equal(blocked.body.blocks.some((b) => b.rule === 'N1'), true);
        assert.equal(store.loadSnapshot(ID), null);
        const issues = await call('list_plan_issues', { id: ID });
        assert.equal(issues.body.issues.some((i) => i.code === 'N1' && i.hard === true), true);

        db.run('DELETE FROM plans');
        seed();
        const mediaRepo = require('../server/db/repos/media');
        mediaRepo.insert({
            id: 'coffee-cup', storedName: 'ab'.repeat(32) + '.png', status: 'pending',
            altText: 'a cup of coffee', actor: 'admin:example-admin', promptText: 'p'
        });
        const shown = await call('set_field', { id: ID, path: LESSON, field: 'visibility', value: 'everyone', expectedRevision: 1 });
        assert.equal(shown.isError, false, shown.text);
        const pending = await call('publish_plan', { id: ID, expectedRevision: 2 });
        assert.equal(pending.isError, true);
        assert.equal(pending.body.error, 'pending_media');
        assert.equal(store.loadSnapshot(ID), null);
        mediaRepo.update('coffee-cup', { status: 'approved' });
        const approved = await call('publish_plan', { id: ID, expectedRevision: 2 });
        assert.equal(approved.isError, false, approved.text);
    });

    it('ac-7: restore with a stale expectedRevision is revision_conflict with the stored revision; audience is assign_required', async () => {
        seed();
        const edit = await call('set_field', { id: ID, path: WORD, field: 'meaning', value: 'espresso', expectedRevision: 1 });
        assert.equal(edit.isError, false, edit.text);
        const before = tables();
        const stale = await call('restore_plan_revision', { id: ID, fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: 1 });
        assert.equal(stale.isError, true);
        assert.equal(stale.body.error, 'revision_conflict');
        assert.equal(stale.body.revision, 2);
        assert.equal(stale.body.plan, undefined);
        const audience = await call('restore_plan_revision', { id: ID, fromRevision: 1, path: '', field: 'audience', expectedRevision: 2 });
        assert.equal(audience.isError, true);
        assert.equal(audience.body.error, 'assign_required');
        const assignee = await call('restore_plan_revision', { id: ID, fromRevision: 1, path: '', field: 'assigneeId', expectedRevision: 2 });
        assert.equal(assignee.body.error, 'assign_required');
        const absent = await call('restore_plan_revision', { id: ID, fromRevision: 99, expectedRevision: 2 });
        assert.equal(absent.body.error, 'not_found');
        const noPart = await call('restore_plan_revision', {
            id: ID, fromRevision: 1, path: LESSON + '/blocks/no-such-block', expectedRevision: 2
        });
        assert.equal(noPart.isError, true);
        assert.equal(noPart.body.error, 'not_found');
        assert.equal(tables(), before);
        const noExpected = await call('restore_plan_revision', { id: ID, fromRevision: 1 });
        assert.equal(noExpected.isError, true, 'expectedRevision is required');
        assert.equal(tables(), before);
    });

    it('ac-8: no tool text holds a key, a real username, or the words pinned and locked', () => {
        const plan = listed.filter((t) => NEW_TOOLS.includes(t.name) || /plan|part|block|field/.test(t.name));
        assert.equal(plan.length >= 15, true);
        for (const tool of plan) {
            const text = JSON.stringify([tool.name, tool.title, tool.description, tool.inputSchema]);
            assert.equal(/\b(?:pinned|locked|pin|lock)\b/i.test(text), false, tool.name);
            assert.equal(/field_pinned|keepPinned|pinned_by|pinned_at/.test(text), false, tool.name);
            assert.equal(text.includes(TOKEN), false, tool.name);
            assert.equal(/sk-[A-Za-z0-9_-]{8,}/.test(text), false, tool.name);
            assert.equal(/BrianNelson|brian|ELEVENLABS|OPENAI_API_KEY|ADMIN_PASSWORD/i.test(text), false, tool.name);
        }
        const names = listed.map((t) => t.name);
        for (const name of ['set_lexicon_verified', 'set_ai_planning']) assert.equal(names.includes(name), true);
        assert.equal(names.some((n) => /approve/i.test(n)), false);
    });

    it('the tool descriptions teach the AI the notes: editState, overwrote, restore, issues, assign', () => {
        const byName = Object.fromEntries(listed.map((t) => [t.name, t.description]));
        assert.match(byName.read_part, /editState/);
        assert.match(byName.read_part, /never blocks a write/);
        assert.match(byName.read_part, /overwrote/);
        assert.match(byName.read_part, /restore_plan_revision/);
        assert.match(byName.set_field, /overwrote/);
        assert.match(byName.replace_part, /overwrote/);
        assert.match(byName.restore_plan_revision, /new revision/i);
        assert.match(byName.restore_plan_revision, /expectedRevision/);
        assert.match(byName.restore_plan_revision, /assign_plan/);
        assert.match(byName.list_plan_issues, /never stops a write or a publish/);
        assert.match(byName.list_plan_issues, /set_field/);
        assert.match(byName.assign_plan, /only way to change audience or assigneeId/);
        assert.match(byName.assign_plan, /assignee_refused/);
        assert.match(byName.create_plan, /already_exists/);
        assert.match(byName.create_plan, /placeholder/);
        assert.match(byName.list_plans, /No assignee name/);
        assert.match(byName.diff_plan, /working/);
        assert.match(byName.diff_plan, /published/);
        assert.match(byName.publish_plan, /No person has to approve/);
        assert.match(byName.publish_plan, /pending_media/);
    });

    it('a learner cannot reach any of these: a session cookie is not a bearer token, and the web server has no /mcp', async () => {
        seed();
        const before = tables();
        const mcpServer = require('../mcp/http').listen();
        await new Promise((resolve) => mcpServer.once('listening', resolve));
        const port = mcpServer.address().port;
        const payload = JSON.stringify({
            jsonrpc: '2.0', id: 1, method: 'tools/call',
            params: { name: 'list_plans', arguments: {} }
        });
        function post(headers) {
            return new Promise((resolve, reject) => {
                const req = http.request({
                    host: '127.0.0.1', port, path: '/mcp', method: 'POST',
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
            const cookieOnly = await post({ Cookie: harness.cookieFor('learner') });
            assert.equal(cookieOnly.status, 401);
            assert.equal(cookieOnly.data.includes('beginner-unit-1'), false);
            const adminCookieOnly = await post({ Cookie: adminCookie });
            assert.equal(adminCookieOnly.status, 401, 'not even an admin session reaches the MCP');
            const wrong = await post({ Authorization: 'Bearer not-the-token' });
            assert.equal(wrong.status, 401);
            assert.equal(wrong.data.includes('beginner-unit-1'), false);
            const missing = await post({});
            assert.equal(missing.status, 401);
        } finally {
            mcpServer.close();
        }
        const viaWeb = await fetch(baseUrl + '/mcp', {
            method: 'POST',
            headers: { Cookie: harness.cookieFor('learner'), 'Content-Type': 'application/json', Origin: baseUrl },
            body: payload
        });
        assert.equal(viaWeb.status >= 400, true);
        assert.equal((await viaWeb.text()).includes('beginner-unit-1'), false);
        assert.equal(tables(), before);
    });
});
