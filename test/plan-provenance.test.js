/**
 * Provenance marks and the advisory editState (INIT-008/SPEC-006).
 * Temporary database only. No network beyond the in-process HTTP server, no
 * ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-006
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');

const { planEnv, samplePlan, PLAN_ID, LESSON, WORD, LISTEN } = require('./helpers/plan-db');
const harness = require('./helpers/signed-in-server');

describe('provenance marks and editState (INIT-008/SPEC-006)', { concurrency: false }, () => {
    let env;

    before(() => {
        env = planEnv('hebrew-plan-provenance-');
    });

    after(() => {
        env.close();
    });

    beforeEach(() => {
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM audit_log');
    });

    function setMeaning(id, value, revision, ctx, path) {
        return env.ops.setField(env.store, id, {
            path: path || WORD, field: 'meaning', value: value, expectedRevision: revision
        }, ctx);
    }

    /** The AI sets the meaning (rev 2); an admin changes it (rev 3). */
    function aiThenHuman(id) {
        const plan = id || PLAN_ID;
        env.seed(samplePlan((p) => { p.id = plan; }));
        const ai = setMeaning(plan, 'ai-value', 1, env.ai('set_field'));
        const human = setMeaning(plan, 'human-value', ai.revision, env.human('set_field'));
        assert.deepEqual([ai.revision, human.revision], [2, 3]);
        return plan;
    }

    function read(id, path, ctx) {
        const result = env.ops.readPart(env.store, id, { path: path }, ctx);
        assert.equal(result.error, undefined, JSON.stringify(result));
        return result;
    }

    it('ac-1: an AI reader sees the person\'s edit, the value it replaced, and no username', () => {
        const id = aiThenHuman();
        const result = read(id, WORD, env.ai('read_part'));
        assert.equal(result.revision, 3);
        assert.equal(result.part.meaning, 'human-value');
        const state = result.editState;
        assert.equal(state.advisory, true);
        assert.equal(state.more, 0);
        assert.equal(state.lastHumanEdit.revision, 3);
        assert.equal(typeof state.lastHumanEdit.at, 'string');
        assert.equal(state.humanEdited.length, 1);
        const entry = state.humanEdited[0];
        assert.equal(entry.path, WORD);
        assert.equal(entry.field, 'meaning');
        assert.equal(entry.revision, 3);
        assert.equal(entry.previousValue, 'ai-value');
        assert.equal(entry.previousWriter, 'ai');
        assert.equal(Object.prototype.hasOwnProperty.call(entry, 'by'), false);
        assert.equal(JSON.stringify(result).includes('example-admin'), false);

        const admin = read(id, WORD, env.human('read_part'));
        assert.equal(admin.editState.humanEdited[0].by, 'example-admin');
        assert.equal(admin.editState.humanEdited[0].previousValue, 'ai-value');

        const script = read(id, WORD, env.system('report'));
        assert.equal(script.editState.humanEdited[0].by, 'example-admin');

        const anonymous = env.ops.readPart(env.store, id, { path: WORD });
        assert.equal(JSON.stringify(anonymous).includes('example-admin'), false);
        assert.equal(anonymous.editState.humanEdited.length, 1);
    });

    it('ac-1: editState never changes part or revision, and a read writes nothing', () => {
        const id = aiThenHuman();
        const counts = ['plan_revisions', 'plan_field_marks', 'audit_log'].map((t) => env.count(t));
        const withState = read(id, WORD, env.ai());
        const { part, revision } = withState;
        assert.deepEqual(part, JSON.parse(env.store.loadWorkingCopy(id).document).units[0].lessons[0].blocks[0].words[1]);
        assert.equal(revision, env.store.loadWorkingCopy(id).revision);
        assert.deepEqual(['plan_revisions', 'plan_field_marks', 'audit_log'].map((t) => env.count(t)), counts);
    });

    it('ac-1: a read below or elsewhere sees only the marks at or below its path', () => {
        const id = aiThenHuman();
        assert.equal(read(id, LESSON, env.ai()).editState.humanEdited.length, 1);
        assert.equal(read(id, '', env.ai()).editState.humanEdited.length, 1);
        assert.equal(read(id, '/', env.ai()).editState.humanEdited.length, 1);
        assert.equal(read(id, LESSON + '/blocks/introduction', env.ai()).editState.humanEdited.length, 1);
        const elsewhere = read(id, LISTEN, env.ai()).editState;
        assert.deepEqual(elsewhere.humanEdited, []);
        assert.equal(elsewhere.lastHumanEdit.revision, 3, 'lastHumanEdit is the newest anywhere in the plan');
        // words/2 must not match words/20 or a sibling that shares the prefix.
        const sibling = read(id, LESSON + '/blocks/introduction/words/3', env.ai()).editState;
        assert.deepEqual(sibling.humanEdited, []);
    });

    it('ac-2: after the AI overwrites, the field is not human-edited; the write result says what it replaced', () => {
        const id = aiThenHuman();
        const over = setMeaning(id, 'ai-again', 3, env.ai('set_field'));
        assert.equal(over.error, undefined);
        assert.equal(over.revision, 4);
        assert.deepEqual(over.overwrote, [{
            path: WORD, field: 'meaning', humanRevision: 3, valueSetByPerson: 'human-value'
        }]);

        const state = read(id, WORD, env.ai()).editState;
        assert.deepEqual(state.humanEdited, []);
        assert.equal(state.lastHumanEdit.revision, 3);

        const history = env.revisions.list(id, {}, env.ai());
        assert.deepEqual(history.revisions.slice(0, 3).map((r) => [r.revision, r.actorKind, r.actorName]), [
            [4, 'ai', 'mcp'], [3, 'human', 'human'], [2, 'ai', 'mcp']
        ]);
    });

    it('ac-2: a person\'s own write, and a write that replaces nothing a person wrote, carry no overwrote', () => {
        const id = aiThenHuman();
        const human = setMeaning(id, 'human-two', 3, env.human());
        assert.equal(Object.prototype.hasOwnProperty.call(human, 'overwrote'), false);
        const other = env.ops.setField(env.store, id, {
            path: LISTEN, field: 'passage', value: 'תּוֹדָה.', expectedRevision: human.revision
        }, env.ai());
        assert.equal(Object.prototype.hasOwnProperty.call(other, 'overwrote'), false);
        const over = setMeaning(id, 'ai-final', other.revision, env.system('script'));
        assert.equal(over.overwrote.length, 1);
        assert.equal(over.overwrote[0].valueSetByPerson, 'human-two');
        assert.equal(over.overwrote[0].humanRevision, 3 + 1);
    });

    it('ac-2: a whole-part replace and a restore also report what they replaced', () => {
        const id = aiThenHuman();
        const word = read(id, WORD, env.ai()).part;
        word.meaning = 'replaced-by-ai';
        const replaced = env.ops.replacePart(env.store, id, { path: WORD, value: word, expectedRevision: 3 }, env.ai());
        assert.equal(replaced.overwrote[0].valueSetByPerson, 'human-value');

        const id2 = aiThenHuman('second-plan');
        const restored = env.revisions.restore(env.store, id2, {
            fromRevision: 2, path: WORD, field: 'meaning', expectedRevision: 3
        }, env.ai());
        assert.equal(restored.overwrote[0].valueSetByPerson, 'human-value');
        assert.equal(restored.restoredFrom, 2);
    });

    it('ac-3: content with no mark that a person edits gets previousWriter unknown; a clean plan has none', () => {
        env.seed();
        const clean = read(PLAN_ID, '', env.ai()).editState;
        assert.deepEqual(clean.humanEdited, []);
        assert.equal(clean.lastHumanEdit, null);
        assert.equal(clean.more, 0);
        assert.equal(clean.advisory, true);

        const edited = setMeaning(PLAN_ID, 'person-first', 1, env.human());
        const entry = read(PLAN_ID, WORD, env.ai()).editState.humanEdited[0];
        assert.equal(entry.previousWriter, 'unknown');
        assert.equal(entry.previousValue, 'coffee');
        assert.equal(entry.revision, edited.revision);
    });

    it('previousWriter names the earlier writer: a script, or a person before a person', () => {
        env.seed();
        const script = setMeaning(PLAN_ID, 'from-script', 1, env.system('seed-script'));
        const person = setMeaning(PLAN_ID, 'from-person-one', script.revision, env.human());
        assert.equal(read(PLAN_ID, WORD, env.ai()).editState.humanEdited[0].previousWriter, 'system');
        const again = setMeaning(PLAN_ID, 'from-person-two', person.revision, env.human('set_field', 'example-admin-two'));
        const entry = read(PLAN_ID, WORD, env.human('read_part')).editState.humanEdited[0];
        assert.equal(entry.previousWriter, 'human');
        assert.equal(entry.previousValue, 'from-person-one');
        assert.equal(entry.revision, again.revision);
        assert.equal(entry.by, 'example-admin-two');
    });

    it('ac-6: a removed block deletes the marks at and under it, and only those', () => {
        env.seed();
        const ctx = env.human();
        const a = env.ops.setField(env.store, PLAN_ID, { path: LISTEN, field: 'passage', value: 'תּוֹדָה.', expectedRevision: 1 }, ctx);
        const b = setMeaning(PLAN_ID, 'kept', a.revision, ctx);
        assert.equal(env.marksRepo.forPlan(PLAN_ID).length, 2);
        const removed = env.ops.removeBlock(env.store, PLAN_ID, { path: LISTEN, expectedRevision: b.revision }, ctx);
        assert.equal(removed.error, undefined);
        const left = env.marksRepo.forPlan(PLAN_ID);
        assert.deepEqual(left.map((m) => m.path + '#' + m.field), [WORD + '#meaning']);
    });

    it('ac-6: a removed word deletes its marks; a path that only shares a prefix is kept', () => {
        env.seed();
        const ctx = env.human();
        const a = setMeaning(PLAN_ID, 'one', 1, ctx);
        const b = setMeaning(PLAN_ID, 'three', a.revision, ctx, LESSON + '/blocks/introduction/words/3');
        const block = env.ops.readPart(env.store, PLAN_ID, { path: LESSON + '/blocks/introduction' }).part;
        block.words = block.words.filter((w) => w.id !== '2');
        const result = env.ops.replacePart(env.store, PLAN_ID, {
            path: LESSON + '/blocks/introduction', value: block, expectedRevision: b.revision
        }, ctx);
        assert.equal(result.error, undefined, JSON.stringify(result));
        assert.deepEqual(env.marksRepo.forPlan(PLAN_ID).map((m) => m.path), [LESSON + '/blocks/introduction/words/3']);
    });

    it('ac-6: an added block and a moved block write no marks; a restore by an admin writes human marks', () => {
        env.seed();
        const ai = env.ai();
        const added = env.ops.insertBlock(env.store, PLAN_ID, {
            path: LESSON, index: 1, block: { id: 'extra', type: 'listening', passage: 'תּוֹדָה.' }, expectedRevision: 1
        }, env.human());
        assert.equal(added.error, undefined);
        assert.equal(env.count('plan_field_marks'), 0);
        const moved = env.ops.moveBlock(env.store, PLAN_ID, { path: LESSON + '/blocks/extra', index: 0, expectedRevision: added.revision }, env.human());
        assert.equal(moved.error, undefined);
        assert.equal(env.count('plan_field_marks'), 0);

        const a = setMeaning(PLAN_ID, 'ai-says', moved.revision, ai);
        const b = setMeaning(PLAN_ID, 'then-person', a.revision, env.human());
        const c = setMeaning(PLAN_ID, 'ai-again', b.revision, ai);
        assert.equal(read(PLAN_ID, WORD, ai).editState.humanEdited.length, 0);
        const restored = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: b.revision, path: WORD, field: 'meaning', expectedRevision: c.revision
        }, env.human('restore_plan_revision'));
        assert.equal(restored.error, undefined);
        const mark = env.marksRepo.forPlan(PLAN_ID).find((m) => m.path === WORD);
        assert.equal(mark.lastActorKind, 'human');
        assert.equal(mark.humanRevision, restored.revision);
        assert.equal(mark.previousValueJson, JSON.stringify('ai-again'));
        assert.equal(mark.previousWriterKind, 'ai');
        assert.equal(read(PLAN_ID, WORD, ai).editState.humanEdited[0].previousValue, 'ai-again');
    });

    it('a publish is not a person edit of the plan status', () => {
        env.seed();
        const published = env.publish.publishPlan(env.store, PLAN_ID, { expectedRevision: 1 }, env.human('publish_plan'));
        assert.equal(published.error, undefined);
        const edit = env.ops.setField(env.store, PLAN_ID, { path: '', field: 'title', value: 'After publish', expectedRevision: published.revision }, env.human());
        assert.equal(edit.error, undefined);
        assert.deepEqual(env.marksRepo.forPlan(PLAN_ID).map((m) => m.path + '#' + m.field), ['#title']);
    });

    it('ac-7: a previous value over 32 KiB is not stored, and the note says so', () => {
        env.seed(samplePlan((p) => {
            p.units[0].lessons[0].newWords = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0') + 'a'.repeat(3000));
        }));
        const big = Buffer.byteLength(JSON.stringify(working().units[0].lessons[0].newWords), 'utf8');
        assert.equal(big > 32768, true);
        const result = env.ops.setField(env.store, PLAN_ID, { path: LESSON, field: 'newWords', value: ['small'], expectedRevision: 1 }, env.human());
        assert.equal(result.error, undefined, JSON.stringify(result.blocks));
        const mark = env.marksRepo.forPlan(PLAN_ID).find((m) => m.field === 'newWords');
        assert.equal(mark.previousValueJson, null);
        const entry = read(PLAN_ID, LESSON, env.ai()).editState.humanEdited[0];
        assert.equal(entry.previousValue, null);
        assert.equal(entry.previousValueOmitted, true);

        const small = setMeaning(PLAN_ID, 'x', result.revision, env.human());
        assert.equal(small.error, undefined);
        const normal = read(PLAN_ID, WORD, env.ai()).editState.humanEdited[0];
        assert.equal(Object.prototype.hasOwnProperty.call(normal, 'previousValueOmitted'), false);
    });

    function working() {
        return JSON.parse(env.store.loadWorkingCopy(PLAN_ID).document);
    }

    it('ac-7: 60 human-edited fields come back as 50 entries and more 10, newest first', () => {
        env.seed();
        let revision = 1;
        const fields = ['word', 'meaning', 'translit', 'sound', 'pos', 'trap', 'infinitive', 'sentence'];
        let count = 0;
        const order = [];
        outer: for (let word = 1; word <= 8; word += 1) {
            for (const field of fields) {
                const result = env.ops.setField(env.store, PLAN_ID, {
                    path: LESSON + '/blocks/introduction/words/' + word, field: field, value: 'edit ' + count, expectedRevision: revision
                }, env.human());
                assert.equal(result.error, undefined, JSON.stringify(result));
                revision = result.revision;
                order.push(revision);
                count += 1;
                if (count === 60) break outer;
            }
        }
        const state = read(PLAN_ID, '', env.ai()).editState;
        assert.equal(state.humanEdited.length, 50);
        assert.equal(state.more, 10);
        assert.deepEqual(state.humanEdited.map((e) => e.revision), order.slice().reverse().slice(0, 50));
        assert.equal(state.lastHumanEdit.revision, revision);
        const sorted = state.humanEdited.every((e, i, all) => i === 0 || all[i - 1].revision > e.revision);
        assert.equal(sorted, true);
    });

    it('ac-8: a failing marks upsert rolls back the working copy, the revision row, the audit row, and the marks', () => {
        env.seed();
        const before = env.store.loadWorkingCopy(PLAN_ID);
        const original = env.marksRepo.upsert;
        let calls = 0;
        env.marksRepo.upsert = function () { calls += 1; throw new Error('marks upsert failed'); };
        try {
            assert.throws(() => setMeaning(PLAN_ID, 'x', 1, env.human()), /marks upsert failed/);
        } finally {
            env.marksRepo.upsert = original;
        }
        assert.equal(calls, 1);
        const row = env.store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, before.document);
        assert.equal(row.revision, 1);
        assert.equal(env.count('plan_revisions'), 0, 'not even the baseline stays');
        assert.equal(env.count('audit_log'), 0);
        assert.equal(env.count('plan_field_marks'), 0);
        assert.equal(setMeaning(PLAN_ID, 'x', 1, env.human()).error, undefined);
        assert.equal(env.count('plan_field_marks'), 1);
    });

    it('ac-9: listPlans counts fields whose last writer is a person, from one grouped query', () => {
        env.seed(samplePlan((p) => { p.id = 'two-edited'; }));
        let r = setMeaning('two-edited', 'a', 1, env.human());
        r = env.ops.setField(env.store, 'two-edited', { path: '', field: 'title', value: 'B', expectedRevision: r.revision }, env.human());
        env.ops.setField(env.store, 'two-edited', { path: LISTEN, field: 'passage', value: 'תּוֹדָה.', expectedRevision: r.revision }, env.ai());

        env.seed(samplePlan((p) => { p.id = 'overwritten'; }));
        const a = setMeaning('overwritten', 'person', 1, env.human());
        setMeaning('overwritten', 'ai-after', a.revision, env.ai());

        env.seed(samplePlan((p) => { p.id = 'never-edited'; }));

        const seen = [];
        const original = env.db.all;
        env.db.all = function (sql, ...params) {
            seen.push(String(sql));
            return original.call(env.db, sql, ...params);
        };
        let result;
        try {
            result = env.create.listPlans(env.store, {}, env.ai('list_plans'));
        } finally {
            env.db.all = original;
        }
        const byId = Object.fromEntries(result.plans.map((p) => [p.id, p.humanEditedFields]));
        assert.deepEqual(byId, { 'two-edited': 2, overwritten: 0, 'never-edited': 0 });
        const markQueries = seen.filter((sql) => sql.includes('plan_field_marks'));
        assert.equal(markQueries.length, 1);
        assert.equal(/GROUP BY/i.test(markQueries[0]), true);
    });

    it('the marks repo compares paths with substr, never LIKE, and keeps a sibling that shares a prefix', () => {
        env.seed();
        const mark = (path) => ({
            planId: PLAN_ID, path: path, field: 'meaning', lastActorKind: 'ai', lastActorName: 'mcp',
            lastRevision: 1, lastAt: '2026-01-01T00:00:00.000Z', valueHash: 'h'
        });
        for (const path of [LISTEN, LISTEN + '/words/1', LISTEN + '-2', LISTEN + '_x', LISTEN + 'x', 'units/unit-1/lessons/cafe-orde%']) {
            env.marksRepo.upsert(mark(path));
        }
        assert.deepEqual(env.marksRepo.forPathPrefix(PLAN_ID, LISTEN).map((m) => m.path).sort(), [LISTEN, LISTEN + '/words/1'].sort());
        assert.equal(env.marksRepo.forPathPrefix(PLAN_ID, 'units/unit-1/lessons/cafe-orde%').length, 1);
        assert.equal(env.marksRepo.forPathPrefix(PLAN_ID, '%').length, 0);
        env.marksRepo.deleteUnder(PLAN_ID, LISTEN);
        assert.deepEqual(env.marksRepo.forPlan(PLAN_ID).map((m) => m.path).sort(),
            ['units/unit-1/lessons/cafe-orde%', LISTEN + '-2', LISTEN + '_x', LISTEN + 'x'].sort());
        env.marksRepo.deleteUnder(PLAN_ID, '');
        assert.equal(env.count('plan_field_marks'), 0);
    });
});

describe('editState through the HTTP and MCP read (INIT-008/SPEC-006)', { concurrency: false }, () => {
    let baseUrl;
    let adminCookie;
    let handlers;
    let prevToken;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        const accounts = require('../server/accounts');
        const cookies = require('../server/http/cookies');
        accounts.createUser({
            username: 'example-admin', password: harness.GOOD_PASSWORD, role: 'admin',
            displayName: 'Example Admin', actor: 'test'
        });
        const session = accounts.signIn('example-admin', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        adminCookie = cookies.cookieName() + '=' + encodeURIComponent(session.token);
        prevToken = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'test-plan-provenance-token';
        const tools = require('../mcp/tools');
        tools.registerLessonTools({ registerTool() {} });
        handlers = tools.toolHandlers;
    });

    after(async () => {
        if (prevToken === undefined) delete process.env.MCP_AUTH_TOKEN;
        else process.env.MCP_AUTH_TOKEN = prevToken;
        await harness.stop();
    });

    it('the admin read route returns editState with the name; the MCP read_part returns it without one', async () => {
        const db = require('../server/db');
        const store = require('../server/plans/store');
        const plan = samplePlan();
        store.saveWorkingCopy({
            id: plan.id, document: JSON.stringify(plan), revision: 1, status: plan.status,
            audience: plan.audience, assigneeId: plan.assigneeId
        });
        const set = async (headers, body) => fetch(baseUrl + '/api/plans/' + PLAN_ID + '/fields', {
            method: 'PATCH',
            headers: Object.assign({ 'Content-Type': 'application/json', Origin: baseUrl }, headers),
            body: JSON.stringify(body)
        });
        const viaMcp = await handlers.set_field({
            id: PLAN_ID, path: WORD, field: 'meaning', value: 'ai-value', expectedRevision: 1
        });
        assert.equal(viaMcp.isError, undefined, JSON.stringify(viaMcp));
        const viaAdmin = await set({ Cookie: adminCookie }, {
            path: WORD, field: 'meaning', value: 'human-value', expectedRevision: 2
        });
        assert.equal(viaAdmin.status, 200);
        const overwrite = await handlers.set_field({
            id: PLAN_ID, path: WORD, field: 'meaning', value: 'ai-final', expectedRevision: 3
        });
        const overwriteBody = JSON.parse(overwrite.content[0].text);
        assert.equal(overwriteBody.overwrote[0].valueSetByPerson, 'human-value');
        assert.equal(overwrite.content[0].text.includes('example-admin'), false);
        assert.equal(await (await set({ Cookie: adminCookie }, {
            path: WORD, field: 'meaning', value: 'human-final', expectedRevision: 4
        })).status, 200);

        const http = await fetch(baseUrl + '/api/plans/' + PLAN_ID + '/parts?path=' + encodeURIComponent(WORD), {
            headers: { Cookie: adminCookie }
        });
        const httpBody = await http.json();
        assert.deepEqual(Object.keys(httpBody).sort(), ['editState', 'part', 'revision']);
        assert.equal(httpBody.editState.humanEdited[0].previousValue, 'ai-final');
        assert.equal(httpBody.editState.humanEdited[0].by, 'example-admin');

        const mcp = await handlers.read_part({ id: PLAN_ID, path: WORD });
        assert.equal(mcp.isError, undefined);
        const mcpBody = JSON.parse(mcp.content[0].text);
        assert.equal(mcpBody.editState.humanEdited[0].previousValue, 'ai-final');
        assert.equal(Object.prototype.hasOwnProperty.call(mcpBody.editState.humanEdited[0], 'by'), false);
        assert.equal(mcp.content[0].text.includes('example-admin'), false);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?', PLAN_ID).n, 5);
    });
});
