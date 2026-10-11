/**
 * INIT-006 §12: each operation reaches one service from every adapter that exists.
 * Temporary database, loopback only.
 * Provenance: INIT-006/SPEC-010
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');

const harness = require('./helpers/signed-in-server');

const KEY = 'שלום';
const OTHER = 'ספר';

function replyJson(result) {
    assert.ok(result && result.content && result.content[0]);
    return JSON.parse(result.content[0].text);
}

describe('learner capability parity (INIT-006/SPEC-010)', { concurrency: false }, () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(u: string) => string} */
    let cookieFor;
    /** @type {Record<string, { id: number }>} */
    let users;
    /** @type {typeof import('../mcp/tools').toolHandlers} */
    let handlers;
    /** @type {Array<{ label: string, args: unknown[] }>} */
    let calls;
    /** @type {Array<{ obj: object, name: string, original: Function }>} */
    let wrapped;
    let prevToken;

    before(async () => {
        prevToken = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'parity-token';
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        users = started.users;

        const tools = require('../mcp/tools');
        tools.registerLessonTools({ registerTool() { /* handlers are enough */ } });
        handlers = tools.toolHandlers;

        calls = [];
        wrapped = [];
        watch('library.list', require('../server/learner/library'), 'list');
        watch('library.add', require('../server/learner/library'), 'add');
        watch('library.setSuspended', require('../server/learner/library'), 'setSuspended');
        watch('cards.deck', require('../server/learner/cards'), 'deck');
        watch('progress.applySignals', require('../server/progress'), 'applySignals');
        watch('profile.buildProfile', require('../server/learner/profile'), 'buildProfile');
        watch('coverage.checkCoverage', require('../server/learner/coverage'), 'checkCoverage');
        watch('mix.compose', require('../server/learner/mix'), 'compose');
        watch('concepts.list', require('../server/db/repos/concepts'), 'list');
        const lexicon = require('../server/lexicon/service');
        watch('lexicon.list', lexicon, 'list');
        watch('lexicon.get', lexicon, 'get');
        watch('lexicon.upsert', lexicon, 'upsert');
        watch('lexicon.setVerified', lexicon, 'setVerified');
        watch('accounts.setAiPlanning', require('../server/accounts'), 'setAiPlanning');
    });

    after(async () => {
        for (const item of wrapped) item.obj[item.name] = item.original;
        await harness.stop();
        if (prevToken === undefined) delete process.env.MCP_AUTH_TOKEN;
        else process.env.MCP_AUTH_TOKEN = prevToken;
    });

    function watch(label, obj, name) {
        const original = obj[name];
        wrapped.push({ obj, name, original });
        obj[name] = function () {
            calls.push({ label, args: Array.from(arguments) });
            return original.apply(this, arguments);
        };
    }

    function taken(label) {
        return calls.filter((call) => call.label === label);
    }

    async function api(method, path, username, body) {
        const headers = {
            'Content-Type': 'application/json',
            Origin: baseUrl
        };
        if (username) headers.Cookie = cookieFor(username);
        const init = { method, headers };
        if (body !== undefined) init.body = JSON.stringify(body);
        const res = await fetch(baseUrl + path, init);
        const text = await res.text();
        let json = null;
        if (text) {
            try { json = JSON.parse(text); } catch (_) { json = null; }
        }
        return { status: res.status, json, text };
    }

    it('ac-1: each §12 row reaches the same service from MCP and HTTP where both exist', async () => {
        const learnerId = users.learner.id;
        calls.length = 0;

        const libraryRead = await api('GET', '/api/me/library', 'learner');
        assert.equal(libraryRead.status, 200);
        const listed = taken('library.list');
        assert.equal(listed.length, 1);
        assert.equal(listed[0].args[0], learnerId);

        calls.length = 0;
        const suspended = await api('PATCH', '/api/me/library/' + encodeURIComponent(KEY), 'learner', {
            suspended: false
        });
        assert.equal(suspended.status, 404);
        assert.equal(taken('library.setSuspended').length, 1);
        assert.equal(taken('library.setSuspended')[0].args[0], learnerId);
        assert.equal(taken('library.setSuspended')[0].args[2], false);

        calls.length = 0;
        const cards = await api('GET', '/api/me/cards?deck=due', 'learner');
        assert.equal(cards.status, 200);
        assert.equal(taken('cards.deck').length, 1);
        assert.equal(taken('cards.deck')[0].args[0], learnerId);
        assert.equal(taken('cards.deck')[0].args[1], 'due');

        calls.length = 0;
        const graded = await api('POST', '/api/review/signals', 'learner', {
            userId: 999999,
            signals: [{
                id: 'parity-signal',
                word: 'שָׁלוֹם',
                direction: 'he_en',
                result: 'pass',
                at: new Date().toISOString(),
                slug: 'public-lesson',
                activity: 'words'
            }]
        });
        assert.equal(graded.status, 200);
        const signals = taken('progress.applySignals');
        assert.equal(signals.length, 1);
        assert.equal(signals[0].args[0], learnerId);
        assert.notEqual(signals[0].args[0], 999999);

        calls.length = 0;
        const switchedOff = await handlers.set_ai_planning({ username: 'learner', enabled: true });
        assert.deepEqual(replyJson(switchedOff), { error: 'forbidden' });
        assert.equal(taken('accounts.setAiPlanning').length, 0);
        const switchedOn = await api('PATCH', '/api/admin/users/' + learnerId + '/ai-planning', 'admin', {
            enabled: true
        });
        assert.equal(switchedOn.status, 200);
        const planning = taken('accounts.setAiPlanning');
        assert.equal(planning.length, 1);
        assert.equal(planning[0].args[0], learnerId);
        assert.equal(planning[0].args[1], true);

        calls.length = 0;
        const profileMcp = await handlers.get_learner_profile({
            username: 'learner',
            now: '1999-01-01T00:00:00.000Z'
        });
        assert.equal(profileMcp.isError, undefined, profileMcp.content[0].text);
        const profileHttp = await api('GET', '/api/me/profile?now=1', 'learner');
        assert.equal(profileHttp.status, 200);
        const profiles = taken('profile.buildProfile');
        assert.equal(profiles.length, 2);
        assert.equal(profiles[0].args[0], 'learner');
        assert.deepEqual(profiles[0].args[1], { audience: 'mcp' });
        assert.equal(profiles[1].args[0], learnerId);
        assert.deepEqual(profiles[1].args[1], { audience: 'self' });
        assert.equal(profileHttp.json.ai_planning, true);

        calls.length = 0;
        const text = 'שלום עולם';
        const coverageMcp = await handlers.check_text_coverage({ username: 'learner', text });
        assert.equal(coverageMcp.isError, undefined, coverageMcp.content[0].text);
        const coverageHttp = await api('POST', '/api/me/coverage', 'learner', { text });
        assert.equal(coverageHttp.status, 200);
        const coverages = taken('coverage.checkCoverage');
        assert.equal(coverages.length, 2);
        assert.equal(coverages[0].args[0], 'learner');
        assert.equal(coverages[0].args[1], text);
        assert.deepEqual(coverages[0].args[2], { audience: 'mcp' });
        assert.equal(coverages[1].args[0], learnerId);
        assert.equal(coverages[1].args[1], text);
        assert.deepEqual(coverages[1].args[2], { audience: 'self' });
        const forwarded = await api('POST', '/api/me/coverage', 'learner', { text, now: 1 });
        assert.equal(forwarded.status, 400);
        assert.equal(taken('coverage.checkCoverage').length, 2);

        calls.length = 0;
        const before = Date.now();
        const mix = await api('GET', '/api/me/mix?now=1', 'learner');
        const after = Date.now();
        assert.equal(mix.status, 200);
        assert.ok(Array.isArray(mix.json.sections));
        const mixes = taken('mix.compose');
        assert.equal(mixes.length, 1);
        assert.equal(mixes[0].args[0], learnerId);
        assert.equal(mixes[0].args.length, 2);
        assert.ok(mixes[0].args[1] >= before && mixes[0].args[1] <= after);
        assert.notEqual(mixes[0].args[1], 1);

        calls.length = 0;
        const conceptsMcp = await handlers.list_grammar_concepts();
        assert.equal(conceptsMcp.isError, undefined);
        const conceptsHttp = await api('GET', '/api/grammar/concepts', 'learner');
        assert.equal(conceptsHttp.status, 200);
        const concepts = taken('concepts.list');
        assert.equal(concepts.length, 2);
        assert.equal(concepts[0].args.length, 0);
        assert.equal(concepts[1].args.length, 0);
        assert.ok(conceptsHttp.json.concepts.length > 0);
        assert.equal(replyJson(conceptsMcp).concepts.length, conceptsHttp.json.concepts.length);

        calls.length = 0;
        const filters = { q: undefined, pos: 'noun', binyan: undefined, tense: undefined,
            rootClass: undefined, verified: undefined, limit: undefined, cursor: undefined };
        const listMcp = await handlers.list_lexicon({ pos: 'noun' });
        assert.equal(listMcp.isError, undefined, listMcp.content[0].text);
        const listHttp = await api('GET', '/api/lexicon?pos=noun', 'learner');
        assert.equal(listHttp.status, 200);
        const lists = taken('lexicon.list');
        assert.equal(lists.length, 2);
        assert.deepEqual(lists[0].args[0], filters);
        assert.deepEqual(lists[1].args[0], filters);

        const seeded = require('../server/lexicon/service').upsert(
            [{ key: KEY, kind: 'lemma', gloss_en: 'peace' }],
            { actor: 'mcp', expectedRev: 0 }
        );
        assert.equal(seeded.ok, true, JSON.stringify(seeded));

        calls.length = 0;
        const gotMcp = await handlers.get_lexicon_entry({ key: KEY });
        assert.equal(gotMcp.isError, undefined, gotMcp.content[0].text);
        const gotHttp = await api('GET', '/api/lexicon/' + encodeURIComponent(KEY), 'learner');
        assert.equal(gotHttp.status, 200);
        const reads = taken('lexicon.get');
        assert.equal(reads.length, 2);
        assert.equal(reads[0].args[0], KEY);
        assert.equal(reads[1].args[0], KEY);
        assert.equal(replyJson(gotMcp).entry.strength, undefined);

        calls.length = 0;
        const entry = { key: OTHER, kind: 'lemma', gloss_en: 'book', verified: true, source: 'seed' };
        const putMcp = await handlers.upsert_lexicon_entries({
            entries: [entry],
            expectedRev: 0
        });
        assert.equal(putMcp.isError, undefined, putMcp.content[0].text);
        const putHttp = await handlers.upsert_lexicon_entries({
            entries: [{ key: 'דג', kind: 'lemma', gloss_en: 'fish', verified: true }],
            expectedRev: 0
        });
        assert.equal(putHttp.isError, undefined, putHttp.content[0].text);
        const httpPut = await api('PUT', '/api/admin/lexicon', 'admin', {
            entries: [{ key: 'בית', kind: 'lemma', gloss_en: 'house', verified: true, source: 'mcp' }],
            expectedRev: 0
        });
        assert.equal(httpPut.status, 200, JSON.stringify(httpPut.json));
        const writes = taken('lexicon.upsert');
        assert.equal(writes.length, 3);
        assert.equal(writes[0].args[1].actor, 'mcp');
        assert.equal(writes[0].args[1].expectedRev, 0);
        assert.equal(writes[0].args[0][0].verified, undefined);
        assert.equal(writes[2].args[1].actor, 'admin:admin');
        assert.equal(writes[2].args[1].expectedRev, 0);
        assert.equal(writes[2].args[0][0].verified, undefined);
        assert.equal(writes[2].args[0][0].source, undefined);
        assert.equal(writes[2].args[0][0].gloss_en, 'house');
        const stored = require('../server/lexicon/service').get('בית');
        assert.equal(stored.entry.source, 'admin');
        assert.equal(stored.entry.verified, 0);

        calls.length = 0;
        const verifyMcp = await handlers.set_lexicon_verified({ key: 'בית', verified: true });
        assert.deepEqual(replyJson(verifyMcp), { error: 'forbidden' });
        assert.equal(taken('lexicon.setVerified').length, 0);
        const verifyHttp = await api(
            'PATCH',
            '/api/admin/lexicon/' + encodeURIComponent('בית') + '/verified',
            'admin',
            { verified: true }
        );
        assert.equal(verifyHttp.status, 200, JSON.stringify(verifyHttp.json));
        const verifies = taken('lexicon.setVerified');
        assert.equal(verifies.length, 1);
        assert.deepEqual(verifies[0].args[0], ['בית']);
        assert.equal(verifies[0].args[1], true);
        assert.equal(verifies[0].args[2].actor, 'admin:admin');
        assert.equal(require('../server/lexicon/service').get('בית').entry.verified, 1);

        calls.length = 0;
        const addMcp = await handlers.add_library_words({ username: 'learner', words: [KEY] });
        assert.equal(addMcp.isError, undefined, addMcp.content[0].text);
        const addHttp = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: KEY }]
        });
        assert.equal(addHttp.status, 200, JSON.stringify(addHttp.json));
        const adds = taken('library.add');
        assert.equal(adds.length, 2);
        assert.equal(adds[0].args[0], learnerId);
        assert.deepEqual(adds[0].args[1], [{ key: KEY }]);
        assert.deepEqual(adds[0].args[2], { source: 'ai' });
        assert.equal(adds[1].args[0], learnerId);
        assert.equal(adds[1].args[1][0].key, KEY);
        assert.equal(adds[1].args[2].source, 'self');
    });

    it('HTTP profile JSON keeps a null strength', async () => {
        const profile = require('../server/learner/profile');
        const original = profile.buildProfile;
        profile.buildProfile = () => ({
            username: 'learner',
            profile_version: 1,
            words: [{ word_id: 'אב', he_en: { stage: 'learning', strength: null } }]
        });
        try {
            const res = await api('GET', '/api/me/profile', 'learner');
            assert.equal(res.status, 200);
            assert.equal(res.json.words[0].he_en.strength, null);
            assert.equal(res.text.includes('"strength":null'), true);
        } finally {
            profile.buildProfile = original;
        }
    });
});
