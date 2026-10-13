/**
 * End-to-end AI-first check (INIT-008/SPEC-016).
 *
 * The loop the operator asked for, across every seam, on a temporary database:
 *   - the AI's steps are MCP tool calls through a real McpServer and Client joined in memory
 *     (the SDK validates each argument as in production), so the adapters and the token gate run;
 *   - the person's steps are admin HTTP routes with a signed-in session, so the channel-derived
 *     actor runs;
 *   - a learner reads only the published snapshot.
 * It adds no feature. No network beyond the in-process servers, no ElevenLabs, no OpenAI: a guard
 * records every outbound http, https, or fetch call to a non-local host, and no provider key is set.
 * Hebrew in fixtures is hand-written. The only account names are example-admin and example-learner.
 * Provenance: INIT-008/SPEC-016
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const http = require('http');
const https = require('https');
const path = require('path');

const harness = require('./helpers/signed-in-server');
const { ROWS, HUMAN_ONLY, findParityProblems } = require('./helpers/plan-parity-table');

const TOKEN = 'test-ai-first-e2e-token';
const ID = 'example-plan';
const LESSON = 'units/unit-1/lessons/lesson-1';
const INTRO = LESSON + '/blocks/intro';
const W1 = INTRO + '/words/w1';
const W2 = INTRO + '/words/w2';
const NEW_TOOLS = [
    'list_plans', 'create_plan', 'list_plan_revisions', 'get_plan_revision',
    'diff_plan', 'restore_plan_revision', 'list_plan_issues', 'assign_plan'
];
const OPS = {
    create_plan: 'create', insert_block: 'insert_block', move_block: 'move_block', remove_block: 'remove_block',
    set_field: 'set_field', replace_part: 'replace_part', publish_plan: 'publish', assign_plan: 'assign',
    restore_plan_revision: 'restore'
};

// Hand-written Hebrew.
const NOT_NFC = 'א\u05B7ת\u05BC\u05B8ה'; // a dagesh before a qamats: not in canonical order
const HOUSE_LEXICON = 'בַּיִת'.normalize('NFC');
const HOUSE_DRIFTED = 'ב\u05B7י\u05B4ת'; // the same letters, pointed differently from the lexicon entry
const SENTENCE_AI = 'זֶה בַּיִת.'.normalize('NFC');
const SENTENCE_HUMAN = 'זֶה הַבַּיִת שֶׁלִּי.'.normalize('NFC');
const CLIP = 'sentences/example-house.mp3';
const LISTEN = 'זֶה בַּיִת.'.normalize('NFC');
const MEANING_AI = 'house';
const MEANING_HUMAN = 'a home';
const MEANING_AI_AGAIN = 'house, home';

// Names (never values) of the provider keys the server code reads.
const PROVIDER_KEYS = ['OPENAI_API_KEY', 'ELEVENLABS_API_KEY', '11_labs_api_key', 'ELEVEN_LABS_API_KEY', 'ELEVENLABS_KEY'];

const AUDIT_DETAIL_KEYS = ['revision', 'path', 'field', 'tool', 'requestId'];

describe('the AI-first loop, end to end (INIT-008/SPEC-016)', { concurrency: false }, () => {
    let baseUrl;
    let adminCookie;
    let db;
    let store;
    let client;
    let server;
    let listed;
    let prevEnv;
    let realFetch;
    const realHttpRequest = http.request;
    const realHttpsRequest = https.request;
    const realHttpsGet = https.get;
    const outbound = [];
    /** Every text an MCP call returned in the run, for the username-absence check. */
    const transcript = [];
    /** Every error code an MCP call or a route returned, for the no-refusal check. */
    const errorCodes = new Set();
    /** Tool names called through the client. */
    const toolsCalled = new Set();
    /** One entry per successful plan write: { op, kind }, oldest first. */
    const writes = [];
    const state = {};

    const isLocalHost = (host) => /^(?:127\.0\.0\.1|localhost|\[::1\]|::1)$/i.test(String(host || ''));

    function installOutboundGuard() {
        realFetch = globalThis.fetch;
        globalThis.fetch = function guardedFetch(input, init) {
            let url;
            try { url = new URL(typeof input === 'string' ? input : (input && input.url) || String(input)); } catch (_) { url = null; }
            if (!url || !isLocalHost(url.hostname)) {
                outbound.push('fetch ' + String(url ? url.href : input));
                throw new Error('outbound call blocked by the test');
            }
            return realFetch.call(this, input, init);
        };
        http.request = function guardedHttp(first, ...rest) {
            let host = null;
            if (typeof first === 'string') host = new URL(first).hostname;
            else if (first instanceof URL) host = first.hostname;
            else if (first && typeof first === 'object') host = first.hostname || first.host || 'localhost';
            if (!isLocalHost(String(host || '').replace(/:\d+$/, ''))) {
                outbound.push('http ' + host);
                throw new Error('outbound call blocked by the test');
            }
            return realHttpRequest.call(http, first, ...rest);
        };
        https.request = function guardedHttps(first) {
            outbound.push('https ' + (typeof first === 'string' ? first : (first && (first.hostname || first.host)) || ''));
            throw new Error('outbound call blocked by the test');
        };
        https.get = function guardedHttpsGet(first) {
            outbound.push('https.get ' + (typeof first === 'string' ? first : (first && (first.hostname || first.host)) || ''));
            throw new Error('outbound call blocked by the test');
        };
    }

    before(async () => {
        prevEnv = { MCP_AUTH_TOKEN: process.env.MCP_AUTH_TOKEN, MCP_PORT: process.env.MCP_PORT };
        // No provider key. They are set to the empty string, not deleted: server/audio.js reads a local .env
        // file into any variable that is undefined, and an empty value counts as unset everywhere.
        for (const name of PROVIDER_KEYS) {
            prevEnv[name] = process.env[name];
            process.env[name] = '';
        }
        installOutboundGuard();

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
        const session = accounts.signIn('example-admin', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        adminCookie = cookies.cookieName() + '=' + encodeURIComponent(session.token);
        store = require('../server/plans/store');

        // A verified lexicon entry the AI's second word will drift from.
        require('../server/db/repos/lexicon').upsert({
            key: HOUSE_LEXICON, kind: 'lemma', gloss_en: 'house', translit: 'bayit', verified: 1, source: 'seed'
        });

        process.env.MCP_AUTH_TOKEN = TOKEN;
        process.env.MCP_PORT = '0';
        const { McpServer } = require('@modelcontextprotocol/sdk/server/mcp.js');
        const { Client } = require('@modelcontextprotocol/sdk/client/index.js');
        const { InMemoryTransport } = require('@modelcontextprotocol/sdk/inMemory.js');
        const tools = require('../mcp/tools');
        server = new McpServer({ name: 'ai-first-e2e', version: '1' });
        tools.registerLessonTools(server);
        const [a, b] = InMemoryTransport.createLinkedPair();
        client = new Client({ name: 'ai-first-e2e-client', version: '1' });
        await Promise.all([server.connect(a), client.connect(b)]);
        listed = (await client.listTools()).tools;
    });

    after(async () => {
        try { await client.close(); } catch (_) { /* closed */ }
        try { await server.close(); } catch (_) { /* closed */ }
        http.request = realHttpRequest;
        https.request = realHttpsRequest;
        https.get = realHttpsGet;
        globalThis.fetch = realFetch;
        for (const [key, value] of Object.entries(prevEnv)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        await harness.stop();
    });

    // ------------------------------------------------------------------ helpers

    /** An AI step: one tool call through the real client. */
    async function mcp(name, args) {
        toolsCalled.add(name);
        const result = await client.callTool({ name, arguments: args || {} });
        const text = result.content && result.content[0] ? result.content[0].text : '';
        transcript.push(text);
        let body = null;
        try { body = JSON.parse(text); } catch (_) { /* a plain message */ }
        if (result.isError && body && typeof body.error === 'string') errorCodes.add(body.error);
        return { isError: result.isError === true, text, body };
    }

    /** An AI write that must succeed. Records it for the counts at the end. */
    async function aiWrite(name, args) {
        const res = await mcp(name, args);
        assert.equal(res.isError, false, name + ': ' + res.text.slice(0, 400));
        writes.push({ op: OPS[name], kind: 'ai' });
        return res.body;
    }

    async function api(method, urlPath, body, cookie) {
        const headers = { Cookie: cookie || adminCookie };
        if (body !== undefined) headers['Content-Type'] = 'application/json';
        if (method !== 'GET') headers.Origin = baseUrl;
        const res = await fetch(baseUrl + urlPath, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
        const text = await res.text();
        let json = null;
        try { json = JSON.parse(text); } catch (_) { /* not JSON */ }
        return { status: res.status, json, text };
    }

    /** A person's write through an admin route. Records it. */
    async function humanWrite(method, urlPath, body, op) {
        const res = await api(method, urlPath, body);
        assert.equal(res.status, 200, urlPath + ': ' + res.text.slice(0, 400));
        writes.push({ op, kind: 'human' });
        return res.json;
    }

    const learnerCookie = () => harness.cookieFor('learner');
    const working = () => JSON.parse(store.loadWorkingCopy(ID).document);
    const lessonOf = (doc) => doc.units[0].lessons[0];
    const wordOf = (doc, id) => lessonOf(doc).blocks.find((b) => b.id === 'intro').words.find((w) => w.id === id);
    const revision = () => store.loadWorkingCopy(ID).revision;

    // ------------------------------------------------------------------ steps 1 to 4: the AI alone

    it('step 1: create_plan makes a draft at revision 1; a learner cannot read it; list_plans shows it', async () => {
        const created = await aiWrite('create_plan', { id: ID, title: 'Example plan', recipe: 'topic-conversation' });
        assert.equal(created.revision, 1);
        assert.equal(created.status, 'draft');
        assert.equal(created.plan.units[0].lessons[0].recipe, 'topic-conversation');
        assert.equal(created.plan.units[0].lessons[0].visibility, 'hidden');

        assert.equal((await api('GET', '/api/plans/' + ID + '/published', undefined, learnerCookie())).status, 404);
        assert.equal((await api('GET', '/api/plans/' + ID + '/published')).status, 404, 'not even an admin reads a draft as a learner');

        const list = await mcp('list_plans', {});
        assert.equal(list.isError, false, list.text);
        const row = list.body.plans.find((p) => p.id === ID);
        assert.ok(row, 'the plan is listed');
        assert.deepEqual(
            { status: row.status, revision: row.revision, published: row.published, humanEditedFields: row.humanEditedFields },
            { status: 'draft', revision: 1, published: false, humanEditedFields: 0 }
        );
    });

    it('step 2: the AI builds the lesson with the six operations and set_field', async () => {
        const intro = {
            id: 'intro',
            type: 'introduction',
            words: [
                { id: 'w1', word: NOT_NFC, meaning: 'you', translit: 'ata' },
                { id: 'w2', word: HOUSE_DRIFTED, meaning: 'a house', translit: 'bayit' }
            ]
        };
        await aiWrite('insert_block', { id: ID, path: LESSON, index: 0, block: intro, expectedRevision: revision() });
        await aiWrite('insert_block', {
            id: ID, path: LESSON, index: 1, block: { id: 'listen', type: 'listening', passage: LISTEN }, expectedRevision: revision()
        });
        await aiWrite('remove_block', { id: ID, path: LESSON + '/blocks/placeholder', expectedRevision: revision() });
        await aiWrite('move_block', { id: ID, path: LESSON + '/blocks/listen', index: 1, expectedRevision: revision() });
        await aiWrite('replace_part', {
            id: ID, path: LESSON + '/blocks/listen', value: { id: 'listen', type: 'listening', passage: LISTEN }, expectedRevision: revision()
        });

        // set_field: the meaning (so the AI is the last writer of it), a sentence and its clip, and the lesson fields
        await aiWrite('set_field', { id: ID, path: W2, field: 'meaning', value: MEANING_AI, expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: W2, field: 'sentence', value: SENTENCE_AI, expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: W2, field: 'sentenceAudioPath', value: CLIP, expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: LESSON, field: 'newWords', value: [NOT_NFC.normalize('NFC'), HOUSE_LEXICON], expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: LESSON, field: 'buildsOn', value: [], expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: LESSON, field: 'recycles', value: [], expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: LESSON, field: 'visibility', value: 'everyone', expectedRevision: revision() });
        // one warning the AI leaves on purpose: Hebrew letters in a transliteration
        await aiWrite('set_field', { id: ID, path: W1, field: 'translit', value: 'אתה', expectedRevision: revision() });

        const doc = working();
        assert.equal(lessonOf(doc).blocks.map((b) => b.id).join(','), 'intro,listen');
        assert.equal(wordOf(doc, 'w2').sentenceAudioPath, CLIP);
        assert.equal(lessonOf(doc).visibility, 'everyone');
        state.afterBuild = revision();
    });

    it('step 3: list_plan_issues names pointing-form and lexicon-form with fixes; one set_field each clears them', async () => {
        const first = await mcp('list_plan_issues', { id: ID });
        assert.equal(first.isError, false, first.text);
        const byCode = (body, code) => body.issues.filter((i) => i.code === code);
        // The one hard error in the list is a consequence of the drift: the lesson's newWords holds the
        // lexicon's pointing and no introduction word has that exact form yet. It clears with the fix below.
        const hard = first.body.issues.filter((i) => i.hard);
        assert.deepEqual(hard.map((i) => [i.code, i.severity, i.field]), [['meaning', 'block', 'newWords/1']], first.text.slice(0, 800));

        const pointing = byCode(first.body, 'pointing-form');
        const lexicon = byCode(first.body, 'lexicon-form');
        assert.equal(pointing.length >= 1, true, first.text.slice(0, 800));
        assert.equal(lexicon.length >= 1, true, first.text.slice(0, 800));
        const pf = pointing.find((i) => i.path === W1 && i.field === 'word');
        const lf = lexicon.find((i) => i.path === W2 && i.field === 'word');
        assert.ok(pf, 'pointing-form on the first word: ' + JSON.stringify(pointing));
        assert.ok(lf, 'lexicon-form on the second word: ' + JSON.stringify(lexicon));
        for (const issue of [pf, lf]) {
            assert.equal(issue.severity, 'warn');
            assert.equal(issue.hard, false);
            assert.equal(issue.fix.action, 'set_field');
            assert.equal(typeof issue.fix.suggested, 'string');
        }
        assert.equal(pf.fix.suggested, NOT_NFC.normalize('NFC'));
        assert.equal(lf.fix.suggested, HOUSE_LEXICON);
        assert.equal(byCode(first.body, 'field-language').some((i) => i.path === W1 && i.field === 'translit'), true);

        // each fix is one set_field
        await aiWrite('set_field', { id: ID, path: pf.fix.path, field: pf.fix.field, value: pf.fix.suggested, expectedRevision: revision() });
        await aiWrite('set_field', { id: ID, path: lf.fix.path, field: lf.fix.field, value: lf.fix.suggested, expectedRevision: revision() });

        const after = await mcp('list_plan_issues', { id: ID });
        assert.equal(byCode(after.body, 'pointing-form').length, 0);
        assert.equal(byCode(after.body, 'lexicon-form').length, 0);
        assert.equal(after.body.counts.block, 0);
        const left = byCode(after.body, 'field-language');
        assert.equal(left.length >= 1, true, 'one warning is left unfixed on purpose');
        assert.equal(left.every((i) => i.severity === 'warn' && i.hard === false), true);
        state.warningsLeft = after.body.counts.warn;
    });

    it('step 4: publish_plan succeeds with a warning left and no approval step; the learner then reads the snapshot', async () => {
        const meta = listed.find((t) => t.name === 'publish_plan');
        assert.deepEqual(Object.keys(meta.inputSchema.properties).sort(), ['expectedRevision', 'id'], 'no approval argument exists');
        assert.equal(state.warningsLeft >= 1, true);

        const published = await aiWrite('publish_plan', { id: ID, expectedRevision: revision() });
        assert.equal(published.status, 'published');
        state.publishedRevision = revision();
        assert.equal(store.loadWorkingCopy(ID).status, 'published');

        // The accepted D1 consequence, asserted so it is visible: the AI built, published, and exposed a lesson alone.
        const read = await api('GET', '/api/plans/' + ID + '/published', undefined, learnerCookie());
        assert.equal(read.status, 200);
        assert.equal(read.json.status, 'published');
        assert.equal(read.json.units[0].lessons[0].visibility, 'everyone');
        assert.equal(read.json.revision, state.publishedRevision);
        const snapWord = read.json.units[0].lessons[0].blocks.find((b) => b.id === 'intro').words.find((w) => w.id === 'w2');
        assert.equal(snapWord.meaning, MEANING_AI);
        assert.equal(snapWord.sentence, SENTENCE_AI);
    });

    // ------------------------------------------------------------------ steps 5 to 8: a person fixes, the AI sees it

    it('step 5: a simulated admin fixes a word through the admin route; the plan is a draft at a higher revision', async () => {
        const meaning = await humanWrite('PATCH', '/api/plans/' + ID + '/fields', {
            path: W2, field: 'meaning', value: MEANING_HUMAN, expectedRevision: revision()
        }, 'set_field');
        state.humanMeaningRevision = meaning.revision;
        const sentence = await humanWrite('PATCH', '/api/plans/' + ID + '/fields', {
            path: W2, field: 'sentence', value: SENTENCE_HUMAN, expectedRevision: revision()
        }, 'set_field');
        state.humanSentenceRevision = sentence.revision;
        assert.equal(state.humanMeaningRevision > state.publishedRevision, true);
        assert.equal(state.humanSentenceRevision, state.humanMeaningRevision + 1);
        assert.equal(store.loadWorkingCopy(ID).status, 'draft');
        assert.equal(wordOf(working(), 'w2').meaning, MEANING_HUMAN);

        // The learner still reads the published snapshot, not the person's draft.
        const read = await api('GET', '/api/plans/' + ID + '/published', undefined, learnerCookie());
        assert.equal(read.status, 200);
        const snapWord = read.json.units[0].lessons[0].blocks.find((b) => b.id === 'intro').words.find((w) => w.id === 'w2');
        assert.equal(snapWord.meaning, MEANING_AI);
        assert.equal(read.json.revision, state.publishedRevision);

        const list = await mcp('list_plans', {});
        const row = list.body.plans.find((p) => p.id === ID);
        assert.equal(row.status, 'draft');
        assert.equal(row.unpublishedChanges, true);
        assert.equal(row.humanEditedFields, 2);
    });

    it('step 6: read_part shows editState with both fields, the AI values they replaced, and no account name', async () => {
        const read = await mcp('read_part', { id: ID, path: W2 });
        assert.equal(read.isError, false, read.text);
        assert.equal(read.body.revision, state.humanSentenceRevision);
        const es = read.body.editState;
        assert.equal(es.advisory, true);
        const fields = es.humanEdited.map((e) => e.field).sort();
        assert.deepEqual(fields, ['meaning', 'sentence']);
        const meaning = es.humanEdited.find((e) => e.field === 'meaning');
        const sentence = es.humanEdited.find((e) => e.field === 'sentence');
        assert.equal(meaning.previousValue, MEANING_AI);
        assert.equal(meaning.previousWriter, 'ai');
        assert.equal(meaning.revision, state.humanMeaningRevision);
        assert.equal(sentence.previousValue, SENTENCE_AI);
        assert.equal(sentence.previousWriter, 'ai');
        assert.equal(sentence.revision, state.humanSentenceRevision);
        assert.equal(es.lastHumanEdit.revision, state.humanSentenceRevision);
        for (const entry of es.humanEdited) assert.equal('by' in entry, false, 'the MCP is told "a person", never a name');
        assert.equal(read.text.includes('example-admin'), false);
    });

    it('step 7: list_plan_issues reports audio-stale for the edited sentence whose clip was left', async () => {
        const res = await mcp('list_plan_issues', { id: ID });
        assert.equal(res.isError, false, res.text);
        const stale = res.body.issues.filter((i) => i.code === 'audio-stale');
        assert.equal(stale.length, 1, res.text.slice(0, 800));
        assert.equal(stale[0].severity, 'warn');
        assert.equal(stale[0].hard, false);
        assert.equal(stale[0].path, W2);
        assert.equal(stale[0].field, 'sentence');
        assert.equal(stale[0].fix.clip, 'sentenceAudioPath');
        assert.equal(typeof stale[0].fix.note, 'string');
        assert.match(stale[0].fix.note, /clip/i);
        assert.match(stale[0].message, new RegExp('sentenceAudioPath'));
        assert.equal(res.body.counts.block, 0);
    });

    it('step 8: the AI re-edits the person\'s field; nothing refuses, and the result lists overwrote', async () => {
        const body = await aiWrite('set_field', { id: ID, path: W2, field: 'meaning', value: MEANING_AI_AGAIN, expectedRevision: revision() });
        assert.equal(Array.isArray(body.overwrote), true);
        assert.equal(body.overwrote.length, 1);
        assert.equal(body.overwrote[0].field, 'meaning');
        assert.equal(body.overwrote[0].path, W2);
        assert.equal(body.overwrote[0].valueSetByPerson, MEANING_HUMAN);
        assert.equal(body.overwrote[0].humanRevision, state.humanMeaningRevision);
        assert.equal(JSON.stringify(body).includes('example-admin'), false);
        state.aiAgainRevision = body.revision;
        assert.equal(wordOf(working(), 'w2').meaning, MEANING_AI_AGAIN);

        // The field no longer shows as person-written; the sentence still does.
        const read = await mcp('read_part', { id: ID, path: W2 });
        assert.deepEqual(read.body.editState.humanEdited.map((e) => e.field), ['sentence']);
        assert.equal(read.body.editState.lastHumanEdit.revision, state.humanSentenceRevision, 'the last person edit is still the person\'s');
    });

    // ------------------------------------------------------------------ steps 9 and 10: history, diff, restore

    it('step 9: history lists every write in order with the right actors; diff lists the changed fields with id paths', async () => {
        const res = await mcp('list_plan_revisions', { id: ID, limit: 200 });
        assert.equal(res.isError, false, res.text);
        assert.equal(res.body.truncated, false);
        const oldestFirst = res.body.revisions.slice().reverse();
        assert.deepEqual(
            oldestFirst.map((r) => [r.op, r.actorKind]),
            writes.map((w) => [w.op, w.kind]),
            'one row per write, in order, with the actor kind'
        );
        assert.deepEqual(oldestFirst.map((r) => r.revision), writes.map((_, i) => i + 1));
        assert.equal(oldestFirst[0].op, 'create');
        assert.equal(oldestFirst.some((r) => r.op === 'publish'), true);
        for (const row of oldestFirst) {
            assert.equal(row.actorName, row.actorKind === 'human' ? 'human' : 'mcp', 'the MCP sees "human", never a name');
        }
        const humanRows = oldestFirst.filter((r) => r.actorKind === 'human');
        assert.equal(humanRows.length, 2);
        assert.deepEqual(humanRows.map((r) => r.revision), [state.humanMeaningRevision, state.humanSentenceRevision]);

        // The admin sees the account name on the same rows.
        const asAdmin = await api('GET', '/api/plans/' + ID + '/revisions?limit=200');
        assert.equal(asAdmin.status, 200);
        const adminRows = asAdmin.json.revisions.slice().reverse();
        assert.deepEqual(adminRows.map((r) => r.revision), oldestFirst.map((r) => r.revision));
        for (const row of adminRows) {
            assert.equal(row.actorName, row.actorKind === 'human' ? 'example-admin' : 'mcp');
        }

        // get_plan_revision: the person's revision holds the person's value
        const one = await mcp('get_plan_revision', { id: ID, revision: state.humanMeaningRevision });
        assert.equal(one.isError, false, one.text);
        assert.equal(wordOf(one.body.document, 'w2').meaning, MEANING_HUMAN);
        assert.equal(one.body.actorKind, 'human');

        // diff: the working copy against the published one
        const diff = await mcp('diff_plan', { id: ID, from: 'published', to: 'working' });
        assert.equal(diff.isError, false, diff.text);
        const entry = (field) => diff.body.entries.find((e) => e.path === W2 && e.field === field);
        assert.equal(entry('meaning').before, MEANING_AI);
        assert.equal(entry('meaning').after, MEANING_AI_AGAIN);
        assert.equal(entry('sentence').before, SENTENCE_AI);
        assert.equal(entry('sentence').after, SENTENCE_HUMAN);
        const viaRoute = await api('GET', '/api/plans/' + ID + '/diff?from=published&to=working');
        assert.deepEqual(viaRoute.json.entries, diff.body.entries, 'the route and the tool agree');

        const between = await mcp('diff_plan', { id: ID, from: state.humanMeaningRevision - 1, to: state.humanMeaningRevision });
        // the person's edit moved the plan from published back to draft, and the meaning changed
        assert.deepEqual(between.body.entries.map((e) => e.field).sort(), ['meaning', 'status']);
    });

    it('step 10: restore by the MCP and by the admin route each make one new revision with the chosen value and the rest intact', async () => {
        // MCP: bring the person's meaning back, from the person's own revision
        const before = working();
        const beforeRevision = revision();
        const restored = await aiWrite('restore_plan_revision', {
            id: ID, fromRevision: state.humanMeaningRevision, path: W2, field: 'meaning', expectedRevision: beforeRevision
        });
        assert.equal(restored.revision, beforeRevision + 1);
        assert.equal(restored.restoredFrom, state.humanMeaningRevision);
        const after = working();
        assert.equal(wordOf(after, 'w2').meaning, MEANING_HUMAN, 'the person\'s value is back');
        const expected = JSON.parse(JSON.stringify(before));
        wordOf(expected, 'w2').meaning = MEANING_HUMAN;
        expected.revision = beforeRevision + 1;
        assert.deepEqual(after, expected, 'nothing but that one field changed');
        assert.equal(wordOf(after, 'w1').translit, 'אתה', 'the AI\'s other edits are intact');
        assert.equal(wordOf(after, 'w2').sentence, SENTENCE_HUMAN);

        // The admin: put the AI's earlier sentence back, in one step, from the revision before the person's edit
        const beforeAdmin = working();
        const adminRevision = revision();
        const viaRoute = await humanWrite('POST', '/api/plans/' + ID + '/restore', {
            fromRevision: state.publishedRevision, path: W2, field: 'sentence', expectedRevision: adminRevision
        }, 'restore');
        assert.equal(viaRoute.revision, adminRevision + 1);
        assert.equal(viaRoute.restoredFrom, state.publishedRevision);
        const afterAdmin = working();
        assert.equal(wordOf(afterAdmin, 'w2').sentence, SENTENCE_AI);
        const expectedAdmin = JSON.parse(JSON.stringify(beforeAdmin));
        wordOf(expectedAdmin, 'w2').sentence = SENTENCE_AI;
        expectedAdmin.revision = adminRevision + 1;
        assert.deepEqual(afterAdmin, expectedAdmin);

        // history shows both restore rows, by the right actors
        const log = await mcp('list_plan_revisions', { id: ID, limit: 2 });
        assert.deepEqual(log.body.revisions.map((r) => [r.op, r.actorKind]), [['restore', 'human'], ['restore', 'ai']]);
        assert.equal(log.body.revisions[0].revision, adminRevision + 1);
        assert.equal(log.body.revisions[1].revision, beforeRevision + 1);
    });

    // ------------------------------------------------------------------ steps 11 and 12

    it('step 11: assign_plan works and returns to shared; audience through set_field is assign_required; a mismatched id is id_mismatch', async () => {
        await aiWrite('assign_plan', { id: ID, audience: 'personal', assigneeId: 'example-learner', expectedRevision: revision() });
        assert.equal(store.loadWorkingCopy(ID).audience, 'personal');
        assert.equal(store.loadWorkingCopy(ID).assigneeId, 'example-learner');
        await aiWrite('assign_plan', { id: ID, audience: 'shared', expectedRevision: revision() });
        assert.equal(store.loadWorkingCopy(ID).audience, 'shared');
        assert.equal(store.loadWorkingCopy(ID).assigneeId, null);

        const rev = revision();
        const rowsBefore = db.get('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?', ID).n;
        const audience = await mcp('set_field', { id: ID, path: '', field: 'audience', value: 'personal', expectedRevision: rev });
        assert.equal(audience.isError, true);
        assert.equal(audience.body.error, 'assign_required');
        const assignee = await mcp('set_field', { id: ID, path: '', field: 'assigneeId', value: 'example-learner', expectedRevision: rev });
        assert.equal(assignee.body.error, 'assign_required');
        const mismatch = await mcp('replace_part', {
            id: ID, path: LESSON + '/blocks/listen', value: { id: 'other-id', type: 'listening', passage: LISTEN }, expectedRevision: rev
        });
        assert.equal(mismatch.isError, true);
        assert.equal(mismatch.body.error, 'id_mismatch');
        const stale = await mcp('set_field', { id: ID, path: LESSON, field: 'title', value: 'Late', expectedRevision: rev - 1 });
        assert.equal(stale.body.error, 'revision_conflict');
        assert.equal(stale.body.revision, rev);

        assert.equal(revision(), rev, 'a refused write changes nothing');
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?', ID).n, rowsBefore);
        assert.equal(store.loadWorkingCopy(ID).audience, 'shared');
    });

    it('step 12: revision rows equal writes; audit rows equal them and hold no value; the older tools and the human-only acts behave', async () => {
        // revision log and audit trail
        const revisionRows = db.all('SELECT revision, op, actor_kind, actor_name, summary FROM plan_revisions WHERE plan_id = ? ORDER BY revision', ID);
        assert.equal(revisionRows.length, writes.length);
        assert.equal(revisionRows.some((r) => r.actor_name === 'baseline'), false, 'a plan made by create_plan has no baseline row');
        const auditRows = db.all("SELECT actor, action, target, detail FROM audit_log WHERE action LIKE 'plan.%' ORDER BY id");
        assert.equal(auditRows.length, writes.length);
        assert.deepEqual(auditRows.map((r) => r.action), writes.map((w) => 'plan.' + w.op));
        assert.deepEqual(auditRows.map((r) => r.actor), writes.map((w) => (w.kind === 'ai' ? 'mcp' : 'admin:example-admin')));
        const values = [
            NOT_NFC, HOUSE_DRIFTED, HOUSE_LEXICON, SENTENCE_AI, SENTENCE_HUMAN, LISTEN, CLIP,
            MEANING_HUMAN, MEANING_AI_AGAIN, 'Example plan', 'אתה'
        ];
        for (const row of auditRows) {
            assert.equal(row.target, ID);
            const detail = JSON.parse(row.detail);
            for (const key of Object.keys(detail)) assert.equal(AUDIT_DETAIL_KEYS.includes(key), true, 'audit detail key ' + key);
            for (const v of values) assert.equal(row.detail.includes(v), false, 'a value in an audit row: ' + v);
        }
        for (const row of revisionRows) {
            assert.equal(row.summary.length <= 200, true);
            for (const v of values) assert.equal(row.summary.includes(v), false, 'a value in a revision summary: ' + v);
        }

        // the older tools still work, one call each (the worksheet path; audio:false so no provider is called)
        const lessons = await mcp('list_lessons', {});
        assert.equal(lessons.isError, false, lessons.text);
        const types = await mcp('list_activity_types', {});
        assert.equal(types.isError, false, types.text);
        const worksheet = {
            id: 'e2e-legacy',
            title: 'Legacy worksheet',
            subtitle: 'test',
            level: 'basic',
            vocab: { items: [{ id: '1', word: 'שָׁלוֹם', meaning: 'hello', sound: 'sha-LOM', trap: 'Stress.', sentence: 'שָׁלוֹם.' }] },
            reading: { paragraphs: ['שָׁלוֹם.'] },
            activities: [{ type: 'words' }, { type: 'story' }]
        };
        const upsert = await mcp('upsert_lesson', { slug: 'e2e-legacy', worksheet, audio: false });
        assert.equal(upsert.isError, false, upsert.text);
        const got = await mcp('get_lesson', { slug: 'e2e-legacy' });
        assert.equal(got.isError, false, got.text);
        assert.equal(fs.existsSync(path.join(harness.REPO_ROOT, 'worksheets', 'e2e-legacy')), false, 'the test wrote under the temporary lessons folder only');

        // the three human-only acts stay refused for the AI
        for (const [name, args] of [['set_lexicon_verified', { key: HOUSE_LEXICON, verified: true }], ['set_ai_planning', { username: 'example-learner', enabled: true }]]) {
            const res = await mcp(name, args);
            assert.equal(res.isError, true, name);
            assert.equal(res.body.error, 'forbidden', name);
        }
        const verified = db.get('SELECT verified, verified_by FROM lexicon_entries WHERE key = ?', HOUSE_LEXICON);
        assert.equal(verified.verified, 1, 'the seed row is untouched');
        assert.equal(verified.verified_by == null || verified.verified_by === undefined || typeof verified.verified_by === 'string', true);
        assert.equal(listed.map((t) => t.name).some((n) => /approve/i.test(n)), false, 'no approval tool exists');
        assert.equal(HUMAN_ONLY.filter((h) => h.tool === null).length, 1);
    });

    // ------------------------------------------------------------------ whole-run invariants

    it('the AI is never refused because of who wrote last: the run met only structural errors and the two human-only acts', () => {
        const names = Array.from(errorCodes).sort();
        // forbidden is the answer of set_lexicon_verified and set_ai_planning (step 12); the rest are the deliberate bad writes of step 11
        assert.deepEqual(names, ['assign_required', 'forbidden', 'id_mismatch', 'revision_conflict']);
        for (const code of errorCodes) {
            assert.equal(/mark|human|person|\bpin|lock|edited|overwr|writer/i.test(code), false, code);
        }
    });

    it('no pin, lock, or person-related refusal exists in the plan code, columns, or tool text', () => {
        const root = harness.REPO_ROOT;
        const files = [];
        for (const f of fs.readdirSync(path.join(root, 'server', 'plans'))) if (f.endsWith('.js')) files.push(path.join(root, 'server', 'plans', f));
        for (const f of fs.readdirSync(path.join(root, 'server', 'db', 'repos'))) if (/^plan-.*\.js$/.test(f)) files.push(path.join(root, 'server', 'db', 'repos', f));
        for (const f of fs.readdirSync(path.join(root, 'server', 'db', 'migrations'))) if (f.startsWith('006-')) files.push(path.join(root, 'server', 'db', 'migrations', f));
        files.push(path.join(root, 'server', 'api', 'plan-routes.js'));
        assert.equal(files.length >= 14, true);
        const banned = [
            /field_pinned|keepPinned|pinned_by|pinned_at/,
            /\b(?:pinned|locked)\s*:/,
            /['"`](?:pinned|locked)['"`]/,
            /\.(?:pinned|locked)\b/,
            /\b(?:pinned|locked)\s+(?:INTEGER|TEXT|BOOLEAN|BLOB|NUMERIC)\b/i,
            /\b(?:pinned|locked)_[a-z]+\b/
        ];
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            for (const re of banned) assert.equal(re.test(text), false, path.basename(file) + ' ' + re);
        }
        // every error code the plan modules and routes can return (the validator's rule table is a rule registry, not a refusal)
        const skip = new Set(['validate.js', 'hebrew-checks.js', 'issues.js']);
        const found = new Set();
        for (const file of files.filter((f) => !skip.has(path.basename(f)) && !f.includes(path.sep + 'db' + path.sep))) {
            const text = fs.readFileSync(file, 'utf8');
            for (const re of [/\berror:\s*'([A-Za-z_]+)'/g, /\bconflict\('([A-Za-z_]+)'/g, /\bfailure\('([A-Za-z_]+)'/g]) {
                for (const m of text.matchAll(re)) found.add(m[1]);
            }
        }
        assert.equal(found.size >= 12, true, 'the scan found the codes: ' + Array.from(found).join(','));
        for (const code of found) assert.equal(/mark|human|person|\bpin|lock|edited|overwr|writer/i.test(code), false, code);

        for (const table of ['plans', 'plan_revisions', 'plan_field_marks']) {
            for (const col of db.all('PRAGMA table_info(' + table + ')')) {
                assert.equal(/^(?:pinned|locked|pin|lock)/i.test(col.name), false, table + '.' + col.name);
            }
        }
        for (const tool of listed.filter((t) => NEW_TOOLS.includes(t.name) || /plan|part|block|field/.test(t.name))) {
            const text = JSON.stringify([tool.name, tool.title, tool.description, tool.inputSchema]);
            assert.equal(/field_pinned|keepPinned|pinned_by|pinned_at|\bpinned\b|\blocked\b|\bpin\b|\block\b/i.test(text), false, tool.name);
        }
    });

    it('the eight new tools were all called, and every tool and route in the capability table exists (parity)', () => {
        for (const name of NEW_TOOLS) assert.equal(toolsCalled.has(name), true, name + ' was called in the run');
        const names = new Set(listed.map((t) => t.name));
        for (const name of NEW_TOOLS) assert.equal(names.has(name), true, name);

        const planRoutes = require('../server/api/plan-routes');
        const { createRouter } = require('../server/api/router');
        const router = createRouter();
        planRoutes.register(router);
        require('../server/api/admin-routes').register(router);
        require('../server/api/media-routes').register(router);
        const registry = {
            tools: names,
            routes: router.routes.map((r) => r.method + ' ' + r.path),
            writeRoutes: planRoutes.WRITE_ROUTES.map((r) => r.method + ' ' + r.path),
            functions: (module, fn) => typeof require('../server/plans/' + module)[fn] === 'function'
        };
        assert.deepEqual(findParityProblems(ROWS, registry), []);
        assert.equal(ROWS.filter((r) => r.tool && r.route).length >= 16, true);
        // the human-only acts: each has its admin route, and no tool of that name succeeds
        for (const act of HUMAN_ONLY) {
            assert.equal(registry.routes.includes(act.route.method + ' ' + act.route.path), true, act.act);
            if (act.tool) assert.equal(names.has(act.tool), true, act.tool + ' is registered only to answer forbidden');
        }
    });

    it('no username reaches an MCP result, nothing left the machine, and no provider key was set', () => {
        assert.equal(transcript.length >= 40, true, 'the run made many MCP calls: ' + transcript.length);
        for (const text of transcript) {
            assert.equal(text.includes('example-admin'), false);
            assert.equal(text.includes(TOKEN), false);
        }
        assert.deepEqual(outbound, [], 'the outbound guard recorded: ' + outbound.join(', '));
        // The guard is not blind: a call to a remote host throws and is recorded (then forgotten).
        assert.throws(() => https.request('https://remote.example/'), /blocked/);
        assert.throws(() => http.request({ host: 'remote.example', path: '/' }), /blocked/);
        assert.equal(outbound.length, 2);
        outbound.length = 0;
        // The check never prints a value: it only asks whether each key is empty.
        for (const name of PROVIDER_KEYS) assert.equal(process.env[name] === '', true, name + ' stayed empty for the whole run');
    });

    it('a learner reads only the published snapshot, and a learner or admin session cookie is refused on the MCP port', async () => {
        const read = await api('GET', '/api/plans/' + ID + '/published', undefined, learnerCookie());
        assert.equal(read.status, 200);
        assert.equal(read.json.revision, state.publishedRevision, 'the snapshot, not the working copy');
        assert.equal(revision() > state.publishedRevision, true);
        for (const urlPath of ['/api/plans', '/api/plans/' + ID + '/parts?path=', '/api/plans/' + ID + '/revisions', '/api/plans/' + ID + '/diff?from=1&to=working', '/api/plans/' + ID + '/issues', '/api/plan-schema']) {
            assert.equal((await api('GET', urlPath, undefined, learnerCookie())).status, 403, urlPath);
        }
        const learnerWrite = await api('POST', '/api/plans/' + ID + '/restore', { fromRevision: 1, expectedRevision: revision() }, learnerCookie());
        assert.equal(learnerWrite.status, 403);
        const learnerPlan = await api('POST', '/api/plans', { id: 'learner-made', title: 'No' }, learnerCookie());
        assert.equal(learnerPlan.status, 403);
        assert.equal(store.loadWorkingCopy('learner-made'), null);

        const mcpServer = require('../mcp/http').listen();
        await new Promise((resolve) => mcpServer.once('listening', resolve));
        const port = mcpServer.address().port;
        const payload = JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { name: 'list_plans', arguments: {} } });
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
        const before = JSON.stringify(db.all('SELECT id, revision FROM plans ORDER BY id'));
        try {
            const learnerCookieOnly = await post({ Cookie: learnerCookie() });
            assert.equal(learnerCookieOnly.status, 401);
            assert.equal(learnerCookieOnly.data.includes(ID), false);
            const adminCookieOnly = await post({ Cookie: adminCookie });
            assert.equal(adminCookieOnly.status, 401, 'not even an admin session reaches the MCP');
            assert.equal(adminCookieOnly.data.includes(ID), false);
            assert.equal((await post({ Authorization: 'Bearer not-the-token' })).status, 401);
            assert.equal((await post({})).status, 401);
            // the bearer token is the one key: with it the same call works
            const ok = await post({ Authorization: 'Bearer ' + TOKEN });
            assert.equal(ok.status, 200, ok.data.slice(0, 300));
            assert.equal(ok.data.includes(ID), true);
            assert.equal(ok.data.includes('example-admin'), false);
        } finally {
            mcpServer.close();
        }
        assert.equal(JSON.stringify(db.all('SELECT id, revision FROM plans ORDER BY id')), before, 'the refused calls wrote nothing');
        // the web server has no /mcp
        const viaWeb = await fetch(baseUrl + '/mcp', {
            method: 'POST',
            headers: { Cookie: learnerCookie(), 'Content-Type': 'application/json', Origin: baseUrl },
            body: payload
        });
        assert.equal(viaWeb.status >= 400, true);
        assert.equal((await viaWeb.text()).includes(ID), false);
    });
});
