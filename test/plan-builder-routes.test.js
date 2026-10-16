/**
 * Plan-builder admin routes (INIT-008/SPEC-010): list, create, assign,
 * revisions, diff, restore, issues, and the plan schema. Admin only; the actor
 * comes from the session; nothing is written on a refusal. Temporary database
 * through the signed-in server harness. No network beyond it, no ElevenLabs,
 * no OpenAI.
 * Provenance: INIT-008/SPEC-010
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const harness = require('./helpers/signed-in-server');

const PRETTY = fs.readFileSync(path.join(
    harness.REPO_ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json'
), 'utf8');
const ID = 'beginner-unit-1';
const LESSON = 'units/unit-1/lessons/cafe-order';
const WORD = LESSON + '/blocks/introduction/words/2';
const MARKER = 'MARKER-MUST-NOT-ECHO';

describe('plan builder admin routes (INIT-008/SPEC-010)', { concurrency: false }, () => {
    let baseUrl;
    let adminCookie;
    let db;
    let store;
    let ops;
    let context;

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
        ops = require('../server/plans/ops');
        context = require('../server/plans/context');
    });

    after(async () => {
        await harness.stop();
    });

    beforeEach(() => {
        db.run('DELETE FROM plan_revisions');
        db.run('DELETE FROM plan_field_marks');
        db.run('DELETE FROM plans');
        db.run('DELETE FROM audit_log');
    });

    function seed() {
        const plan = JSON.parse(PRETTY);
        store.saveWorkingCopy({
            id: plan.id, document: PRETTY, revision: plan.revision, status: plan.status,
            audience: plan.audience, assigneeId: plan.assigneeId
        });
        return store.loadWorkingCopy(plan.id);
    }

    async function api(method, urlPath, opts) {
        const options = opts || {};
        const headers = {};
        if (options.cookie !== null) headers.Cookie = options.cookie || adminCookie;
        if (options.body !== undefined && options.contentType !== false) headers['Content-Type'] = 'application/json';
        if (options.origin !== false && method !== 'GET') headers.Origin = options.origin || baseUrl;
        const res = await fetch(baseUrl + urlPath, {
            method,
            headers,
            body: options.body === undefined ? undefined : (typeof options.body === 'string' ? options.body : JSON.stringify(options.body))
        });
        const text = await res.text();
        let json = null;
        try { json = JSON.parse(text); } catch (_) { /* not json */ }
        return { status: res.status, text, json, headers: res.headers };
    }

    const learner = () => harness.cookieFor('learner');

    function tables() {
        return JSON.stringify({
            plans: db.all('SELECT id, revision, working_copy, snapshot FROM plans ORDER BY id'),
            revisions: db.all('SELECT plan_id, revision, op, actor_kind FROM plan_revisions ORDER BY plan_id, revision'),
            audit: db.all('SELECT actor, action, target FROM audit_log ORDER BY id'),
            marks: db.all('SELECT plan_id, path, field FROM plan_field_marks ORDER BY plan_id, path, field')
        });
    }

    /** One request per new route, built for a plan at the given revision. */
    function requests(rev) {
        return [
            { name: 'list plans', method: 'GET', url: '/api/plans' },
            { name: 'create plan', method: 'POST', url: '/api/plans', body: { id: 'matrix-new', title: 'Matrix new' }, write: true },
            { name: 'assign plan', method: 'POST', url: '/api/plans/' + ID + '/assign', body: { audience: 'shared', expectedRevision: rev }, write: true },
            { name: 'list revisions', method: 'GET', url: '/api/plans/' + ID + '/revisions' },
            { name: 'get revision', method: 'GET', url: '/api/plans/' + ID + '/revisions/1' },
            { name: 'diff', method: 'GET', url: '/api/plans/' + ID + '/diff?from=1&to=working' },
            { name: 'restore', method: 'POST', url: '/api/plans/' + ID + '/restore', body: { fromRevision: 1, expectedRevision: rev }, write: true },
            { name: 'list issues', method: 'GET', url: '/api/plans/' + ID + '/issues' },
            { name: 'plan schema', method: 'GET', url: '/api/plan-schema' }
        ];
    }

    /** A plan at revision 3 with two human edits of one field, so revisions 1 to 3 exist. */
    async function editedPlan() {
        seed();
        for (const [value, rev] of [['espresso', 1], ['ristretto', 2]]) {
            const res = await api('PATCH', '/api/plans/' + ID + '/fields', {
                body: { path: WORD, field: 'meaning', value, expectedRevision: rev }
            });
            assert.equal(res.status, 200, res.text);
        }
        return 3;
    }

    it('ac-1: every new route is 401 signed out, 403 for a learner session, and 200 for an admin', async () => {
        const rev = await editedPlan();
        for (const request of requests(rev)) {
            const before = tables();
            const out = await api(request.method, request.url, { cookie: null, body: request.body });
            assert.equal(out.status, 401, request.name);
            assert.equal(out.json.code, 'no_session', request.name);
            const denied = await api(request.method, request.url, { cookie: learner(), body: request.body });
            assert.equal(denied.status, 403, request.name);
            assert.equal(denied.json.code, 'forbidden', request.name);
            assert.equal(tables(), before, request.name + ' wrote while refused');
        }
        // A write moves the revision, so each request is built for the revision as it stands.
        let current = rev;
        for (let i = 0; i < requests(current).length; i += 1) {
            const request = requests(current)[i];
            const ok = await api(request.method, request.url, { body: request.body });
            assert.equal(ok.status, 200, request.name + ' ' + ok.text);
            if (ok.json && Number.isInteger(ok.json.revision) && request.write && request.name !== 'create plan') {
                current = ok.json.revision;
            }
        }
    });

    it('ac-1: the three new write routes are in WRITE_ROUTES, and each is refused without Origin or JSON', async () => {
        const planRoutes = require('../server/api/plan-routes');
        const keys = planRoutes.WRITE_ROUTES.map((r) => r.method + ' ' + r.path);
        const rev = await editedPlan();
        const writes = requests(rev).filter((r) => r.write);
        assert.equal(writes.length, 3);
        for (const request of writes) {
            const key = request.method + ' ' + request.url.replace(ID, ':id').replace(/\?.*$/, '');
            assert.equal(keys.includes(key), true, key);
            const before = tables();
            const noOrigin = await api(request.method, request.url, { body: request.body, origin: false });
            assert.equal(noOrigin.status, 403, request.name);
            assert.equal(noOrigin.json.code, 'csrf');
            const foreign = await api(request.method, request.url, { body: request.body, origin: 'http://evil.example' });
            assert.equal(foreign.status, 403, request.name);
            assert.equal(foreign.json.code, 'csrf');
            const notJson = await api(request.method, request.url, { body: JSON.stringify(request.body), contentType: false });
            assert.equal(notJson.status, 403, request.name);
            assert.equal(notJson.json.code, 'csrf');
            assert.equal(tables(), before, request.name + ' wrote without a CSRF pass');
        }
    });

    it('ac-3: a body key actor, by, or kind is 400 on every new write route, and a query key too', async () => {
        const rev = await editedPlan();
        for (const request of requests(rev).filter((r) => r.write)) {
            for (const key of ['actor', 'by', 'kind']) {
                const before = tables();
                const body = Object.assign({}, request.body);
                body[key] = key === 'actor' ? { kind: 'ai', name: 'mcp' } : 'ai';
                const res = await api(request.method, request.url, { body });
                assert.equal(res.status, 400, request.name + ' ' + key);
                assert.equal(res.json.code, 'bad_request');
                assert.equal(tables(), before);
            }
        }
        for (const request of requests(rev).filter((r) => !r.write && !r.url.endsWith('plan-schema'))) {
            const url = request.url + (request.url.includes('?') ? '&' : '?') + 'actor=ai';
            const res = await api('GET', url);
            assert.equal(res.status, 400, request.name);
            assert.equal(res.json.code, 'bad_request');
        }
    });

    it('ac-3: a route write is human, named for the session, with audit actor admin:example-admin', async () => {
        const created = await api('POST', '/api/plans', { body: { id: 'by-route', title: 'By route' } });
        assert.equal(created.status, 200, created.text);
        let rev = created.json.revision;
        const assigned = await api('POST', '/api/plans/by-route/assign', {
            body: { audience: 'personal', assigneeId: 'example-learner', expectedRevision: rev }
        });
        assert.equal(assigned.status, 200, assigned.text);
        rev = assigned.json.revision;
        const field = await api('PATCH', '/api/plans/by-route/fields', {
            body: { path: '', field: 'title', value: 'By route two', expectedRevision: rev }
        });
        assert.equal(field.status, 200, field.text);
        rev = field.json.revision;
        const restored = await api('POST', '/api/plans/by-route/restore', {
            body: { fromRevision: 1, path: '', field: 'title', expectedRevision: rev }
        });
        assert.equal(restored.status, 200, restored.text);

        const rows = db.all('SELECT revision, op, actor_kind, actor_name, tool FROM plan_revisions WHERE plan_id = ? ORDER BY revision', 'by-route');
        assert.deepEqual(rows.map((r) => [r.op, r.actor_kind, r.actor_name, r.tool]), [
            ['create', 'human', 'example-admin', 'create_plan'],
            ['assign', 'human', 'example-admin', 'assign_plan'],
            ['set_field', 'human', 'example-admin', 'set_field'],
            ['restore', 'human', 'example-admin', 'restore_plan_revision']
        ]);
        const audit = db.all('SELECT actor, action FROM audit_log ORDER BY id');
        assert.deepEqual(audit.map((r) => [r.actor, r.action]), [
            ['admin:example-admin', 'plan.create'],
            ['admin:example-admin', 'plan.assign'],
            ['admin:example-admin', 'plan.set_field'],
            ['admin:example-admin', 'plan.restore']
        ]);
    });

    it('create: a new plan, then already_exists changes nothing, and bad input is refused with no echo', async () => {
        const created = await api('POST', '/api/plans', { body: { id: 'cafe-new', title: 'Cafe new', recipe: 'guide', unitId: 'unit-a', lessonId: 'lesson-a' } });
        assert.equal(created.status, 200, created.text);
        assert.equal(created.json.revision, 1);
        assert.equal(created.json.status, 'draft');
        assert.equal(created.json.plan.units[0].id, 'unit-a');
        assert.equal(created.json.plan.units[0].lessons[0].id, 'lesson-a');
        assert.equal(created.json.plan.units[0].lessons[0].recipe, 'guide');
        assert.equal(created.json.plan.units[0].lessons[0].visibility, 'hidden');
        assert.equal(typeof created.json.validation, 'object');

        const assignedId = await api('POST', '/api/plans', { body: { title: 'Server picks the id' } });
        assert.equal(assignedId.status, 200, assignedId.text);
        assert.match(assignedId.json.plan.id, /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);

        const before = tables();
        const again = await api('POST', '/api/plans', { body: { id: 'cafe-new', title: 'Changed title' } });
        assert.equal(again.status, 400);
        assert.equal(again.json.code, 'already_exists');
        assert.equal(tables(), before);

        for (const [body, code] of [
            [{ id: MARKER, title: 'x' }, 'invalid_id'],
            [{ id: 'fine-id', title: 'x', recipe: MARKER }, 'invalid_recipe'],
            [{ id: 'fine-id', title: 'x', unitId: MARKER }, 'invalid_id']
        ]) {
            const res = await api('POST', '/api/plans', { body });
            assert.equal(res.status, 400);
            assert.equal(res.json.code, code);
            assert.equal(res.text.includes(MARKER), false, 'no echo of the request text');
        }
        for (const body of [{ id: 'a-b' }, { id: 5, title: 'x' }, { id: 'a-b', title: 'x', extra: 1 }, []]) {
            const res = await api('POST', '/api/plans', { body });
            assert.equal(res.status, 400);
            assert.equal(res.json.code, 'bad_request');
        }
        const markup = await api('POST', '/api/plans', { body: { id: 'markup-plan', title: '<b>' + MARKER + '</b>' } });
        assert.equal(markup.status, 400);
        assert.equal(markup.text.includes(MARKER), false);
        assert.equal(store.loadWorkingCopy('markup-plan'), null);
        assert.equal(tables(), before);
    });

    it('ac-8: an oversized body is refused by the body limit and nothing is written', async () => {
        const before = tables();
        // The body reader may destroy the socket before the 413 is read, as the other body-limit tests allow.
        let status;
        try {
            const huge = await api('POST', '/api/plans', { body: { id: 'big-plan', title: 'x'.repeat(1024 * 1024 + 10) } });
            status = huge.status;
            if (status === 413) assert.equal(huge.json.code, 'body_too_large');
        } catch (err) {
            status = 'aborted';
        }
        assert.equal(status === 413 || status === 'aborted', true, String(status));
        assert.equal(tables(), before);
    });

    it('list: admin rows carry assigneeId, humanEditedFields, and the published flags; limit and truncated work', async () => {
        const rev = await editedPlan();
        const assigned = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'personal', assigneeId: 'example-learner', expectedRevision: rev }
        });
        assert.equal(assigned.status, 200, assigned.text);
        await api('POST', '/api/plans', { body: { id: 'second-plan', title: 'Second' } });

        const list = await api('GET', '/api/plans');
        assert.equal(list.status, 200);
        assert.equal(list.json.truncated, false);
        assert.equal(list.json.plans.length, 2);
        const row = list.json.plans.find((p) => p.id === ID);
        assert.equal(row.audience, 'personal');
        assert.equal(row.assigned, true);
        assert.equal(row.assigneeId, 'example-learner');
        assert.equal(row.humanEditedFields >= 1, true);
        assert.equal(row.published, false);
        assert.equal(row.unpublishedChanges, false);
        assert.equal(typeof row.title, 'string');
        assert.equal(typeof row.updatedAt, 'string');
        assert.equal(row.revision, 4);

        const one = await api('GET', '/api/plans?limit=1');
        assert.equal(one.json.plans.length, 1);
        assert.equal(one.json.truncated, true);
        for (const bad of ['?limit=abc', '?limit=-1', '?limit=1.5', '?bogus=1', '?limit=1&limit=2']) {
            const res = await api('GET', '/api/plans' + bad);
            assert.equal(res.status, 400, bad);
        }
    });

    it('assign: personal needs an active account, shared clears it, and a refusal writes nothing', async () => {
        seed();
        let before = tables();
        const unknown = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'personal', assigneeId: MARKER, expectedRevision: 1 }
        });
        assert.equal(unknown.status, 400);
        assert.equal(unknown.json.code, 'assignee_refused');
        assert.equal(unknown.text.includes(MARKER), false);
        const disabled = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'personal', assigneeId: 'example-retired', expectedRevision: 1 }
        });
        assert.equal(disabled.status, 400);
        assert.equal(disabled.json.code, 'assignee_refused');
        const badAudience = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'everyone', expectedRevision: 1 }
        });
        assert.equal(badAudience.status, 400);
        assert.equal(badAudience.json.code, 'invalid_audience');
        const stale = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'shared', expectedRevision: 9 }
        });
        assert.equal(stale.status, 409);
        assert.deepEqual(stale.json, { error: 'revision_conflict', revision: 1 });
        const gone = await api('POST', '/api/plans/no-such-plan/assign', {
            body: { audience: 'shared', expectedRevision: 1 }
        });
        assert.equal(gone.status, 404);
        assert.equal(tables(), before);

        const personal = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'personal', assigneeId: 'example-learner', expectedRevision: 1 }
        });
        assert.equal(personal.status, 200, personal.text);
        assert.equal(personal.json.plan.audience, 'personal');
        assert.equal(personal.json.plan.assigneeId, 'example-learner');
        assert.equal(personal.json.revision, 2);
        before = tables();
        const shared = await api('POST', '/api/plans/' + ID + '/assign', {
            body: { audience: 'shared', expectedRevision: 2 }
        });
        assert.equal(shared.status, 200, shared.text);
        assert.equal(shared.json.plan.audience, 'shared');
        assert.equal(shared.json.plan.assigneeId, null);
        assert.notEqual(tables(), before);

        for (const body of [
            { audience: 'shared' },
            { expectedRevision: 3 },
            { audience: 5, expectedRevision: 3 },
            { audience: 'personal', assigneeId: 5, expectedRevision: 3 },
            { audience: 'shared', expectedRevision: '3' }
        ]) {
            const res = await api('POST', '/api/plans/' + ID + '/assign', { body });
            assert.equal(res.status, 400, JSON.stringify(body));
            assert.equal(res.json.code, 'bad_request');
        }
    });

    it('ac-4: revisions, one revision, and diff return the SPEC-004 and SPEC-005 shapes', async () => {
        await editedPlan();

        const list = await api('GET', '/api/plans/' + ID + '/revisions?sinceRevision=2');
        assert.equal(list.status, 200, list.text);
        assert.equal(list.json.truncated, false);
        assert.deepEqual(list.json.revisions.map((r) => r.revision), [3]);
        const entry = list.json.revisions[0];
        assert.equal(entry.op, 'set_field');
        assert.equal(entry.path, WORD);
        assert.equal(entry.field, 'meaning');
        assert.equal(entry.actorKind, 'human');
        assert.equal(entry.actorName, 'example-admin');
        assert.equal(entry.tool, 'set_field');
        assert.equal(typeof entry.at, 'string');
        assert.equal(JSON.stringify(list.json).includes('ristretto'), false, 'a summary holds no value');

        const all = await api('GET', '/api/plans/' + ID + '/revisions');
        assert.deepEqual(all.json.revisions.map((r) => r.revision), [3, 2, 1]);
        assert.equal(all.json.revisions[2].op, 'baseline');
        const humans = await api('GET', '/api/plans/' + ID + '/revisions?actorKind=human&limit=1&beforeRevision=3');
        assert.deepEqual(humans.json.revisions.map((r) => r.revision), [2]);
        const prefixed = await api('GET', '/api/plans/' + ID + '/revisions?pathPrefix=' + encodeURIComponent(WORD));
        assert.deepEqual(prefixed.json.revisions.map((r) => r.revision), [3, 2]);

        const one = await api('GET', '/api/plans/' + ID + '/revisions/2');
        assert.equal(one.status, 200);
        assert.equal(one.json.revision, 2);
        assert.equal(one.json.actorName, 'example-admin');
        assert.equal(one.json.document.units[0].lessons[0].blocks[0].words[1].meaning, 'espresso');

        const diff = await api('GET', '/api/plans/' + ID + '/diff?from=2&to=working');
        assert.equal(diff.status, 200, diff.text);
        assert.deepEqual(diff.json.from, { ref: 2, revision: 2 });
        assert.deepEqual(diff.json.to, { ref: 'working', revision: 3 });
        assert.equal(diff.json.truncated, false);
        assert.equal(diff.json.entries.length, 1);
        assert.equal(diff.json.entries[0].path, WORD);
        assert.equal(diff.json.entries[0].field, 'meaning');
        assert.equal(diff.json.entries[0].before, 'espresso');
        assert.equal(diff.json.entries[0].after, 'ristretto');
        const narrowed = await api('GET', '/api/plans/' + ID + '/diff?from=1&to=3&path=' + encodeURIComponent(LESSON + '/blocks/dialogue'));
        assert.equal(narrowed.json.entries.length, 0);

        const noSnapshot = await api('GET', '/api/plans/' + ID + '/diff?from=published&to=working');
        assert.equal(noSnapshot.status, 400);
        assert.equal(noSnapshot.json.code, 'no_snapshot');
    });

    it('ac-4: read refusals: unknown plan or revision is 404, a bad query is 400, and no text is echoed', async () => {
        await editedPlan();
        for (const url of [
            '/api/plans/no-such-plan/revisions',
            '/api/plans/no-such-plan/revisions/1',
            '/api/plans/' + ID + '/revisions/99',
            '/api/plans/' + ID + '/revisions/0',
            '/api/plans/' + ID + '/revisions/abc',
            '/api/plans/no-such-plan/diff?from=1&to=working',
            '/api/plans/' + ID + '/diff?from=1&to=99',
            '/api/plans/no-such-plan/issues'
        ]) {
            const res = await api('GET', url);
            assert.equal(res.status, 404, url);
            assert.equal(res.json.code, 'not_found');
        }
        for (const url of [
            '/api/plans/' + ID + '/revisions?bogus=1',
            '/api/plans/' + ID + '/revisions?limit=abc',
            '/api/plans/' + ID + '/revisions?sinceRevision=-1',
            '/api/plans/' + ID + '/revisions?limit=1&limit=2',
            '/api/plans/' + ID + '/revisions/1?x=1',
            '/api/plans/' + ID + '/diff',
            '/api/plans/' + ID + '/diff?from=1',
            '/api/plans/' + ID + '/diff?from=' + MARKER + '&to=working',
            '/api/plans/' + ID + '/diff?from=1&to=working&bogus=1',
            '/api/plans/' + ID + '/issues?bogus=1',
            '/api/plans/' + ID + '/issues?limit=abc'
        ]) {
            const res = await api('GET', url);
            assert.equal(res.status, 400, url);
            assert.equal(res.json.code, 'bad_request');
            assert.equal(res.text.includes(MARKER), false);
        }
        for (const [url, field] of [
            ['/api/plans/' + ID + '/revisions?actorKind=' + MARKER, null],
            ['/api/plans/' + ID + '/issues?severity=' + MARKER, 'severity'],
            ['/api/plans/' + ID + '/issues?limit=0', 'limit']
        ]) {
            const res = await api('GET', url);
            assert.equal(res.status, 400, url);
            assert.equal(res.json.code, 'invalid_op');
            if (field) assert.equal(res.json.field, field);
            assert.equal(res.text.includes(MARKER), false);
        }
    });

    it('ac-4: restore puts a value back as a new revision; stale, assign_required, and missing revisions write nothing', async () => {
        await editedPlan();
        let before = tables();

        const stale = await api('POST', '/api/plans/' + ID + '/restore', {
            body: { fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: 2 }
        });
        assert.equal(stale.status, 409);
        assert.deepEqual(stale.json, { error: 'revision_conflict', revision: 3 });
        const audience = await api('POST', '/api/plans/' + ID + '/restore', {
            body: { fromRevision: 1, path: '', field: 'audience', expectedRevision: 3 }
        });
        assert.equal(audience.status, 400);
        assert.equal(audience.json.code, 'assign_required');
        const assignee = await api('POST', '/api/plans/' + ID + '/restore', {
            body: { fromRevision: 1, path: '', field: 'assigneeId', expectedRevision: 3 }
        });
        assert.equal(assignee.json.code, 'assign_required');
        const missing = await api('POST', '/api/plans/' + ID + '/restore', {
            body: { fromRevision: 99, expectedRevision: 3 }
        });
        assert.equal(missing.status, 404);
        assert.equal(missing.json.code, 'not_found');
        const noPlan = await api('POST', '/api/plans/no-such-plan/restore', {
            body: { fromRevision: 1, expectedRevision: 1 }
        });
        assert.equal(noPlan.status, 404);
        const noExpected = await api('POST', '/api/plans/' + ID + '/restore', { body: { fromRevision: 1 } });
        assert.equal(noExpected.status, 400);
        assert.equal(noExpected.json.code, 'bad_request');
        for (const body of [
            { fromRevision: '1', expectedRevision: 3 },
            { fromRevision: 1, expectedRevision: 3, path: 5 },
            { fromRevision: 1, expectedRevision: 3, field: [] },
            { fromRevision: 1, expectedRevision: 3, extra: true }
        ]) {
            const res = await api('POST', '/api/plans/' + ID + '/restore', { body });
            assert.equal(res.status, 400, JSON.stringify(body));
            assert.equal(res.json.code, 'bad_request');
        }
        assert.equal(tables(), before);

        const field = await api('POST', '/api/plans/' + ID + '/restore', {
            body: { fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: 3 }
        });
        assert.equal(field.status, 200, field.text);
        assert.equal(field.json.revision, 4);
        assert.equal(field.json.restoredFrom, 1);
        assert.equal(field.json.status, 'draft');
        assert.equal(field.json.plan.units[0].lessons[0].blocks[0].words[1].meaning, 'coffee');
        assert.equal(field.json.overwrote, undefined, 'a person restoring over a person');
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?', ID).n, 4);

        before = tables();
        const whole = await api('POST', '/api/plans/' + ID + '/restore', {
            body: { fromRevision: 2, expectedRevision: 4 }
        });
        assert.equal(whole.status, 200, whole.text);
        assert.equal(whole.json.revision, 5);
        assert.equal(whole.json.plan.units[0].lessons[0].blocks[0].words[1].meaning, 'espresso');
        assert.notEqual(tables(), before);
        const history = await api('GET', '/api/plans/' + ID + '/revisions');
        assert.deepEqual(history.json.revisions.map((r) => r.op).slice(0, 2), ['restore', 'restore']);
    });

    it('ac-6: GET parts carries editState with by for an admin, and only an ai or script write carries overwrote', async () => {
        await editedPlan();
        const read = await api('GET', '/api/plans/' + ID + '/parts?path=' + encodeURIComponent(WORD));
        assert.equal(read.status, 200);
        assert.deepEqual(Object.keys(read.json).sort(), ['editState', 'part', 'revision']);
        const edited = read.json.editState.humanEdited.find((e) => e.field === 'meaning');
        assert.equal(edited.path, WORD);
        assert.equal(edited.by, 'example-admin');
        assert.equal(edited.previousValue, 'espresso');
        assert.equal(edited.previousWriter, 'human');
        assert.equal(read.json.editState.advisory, true);

        const again = await api('PATCH', '/api/plans/' + ID + '/fields', {
            body: { path: WORD, field: 'meaning', value: 'doppio', expectedRevision: 3 }
        });
        assert.equal(again.status, 200);
        assert.equal(again.json.overwrote, undefined);

        const aiCtx = context.makeContext({ channel: 'mcp', tool: 'set_field' });
        const aiWrite = ops.setField(store, ID, { path: WORD, field: 'meaning', value: 'lungo', expectedRevision: 4 }, aiCtx);
        assert.equal(aiWrite.error, undefined);
        assert.equal(Array.isArray(aiWrite.overwrote), true);
        assert.equal(aiWrite.overwrote[0].field, 'meaning');
        assert.equal(JSON.stringify(aiWrite.overwrote).includes('example-admin'), false);
        const sysCtx = context.makeContext({ channel: 'system', name: 'parity-script', tool: 'test' });
        await api('PATCH', '/api/plans/' + ID + '/fields', {
            body: { path: WORD, field: 'meaning', value: 'cortado', expectedRevision: 5 }
        });
        const sysWrite = ops.setField(store, ID, { path: WORD, field: 'meaning', value: 'macchiato', expectedRevision: 6 }, sysCtx);
        assert.equal(Array.isArray(sysWrite.overwrote), true);
    });

    it('issues: the shape, the filters, and an id path the part operations accept', async () => {
        seed();
        const words = JSON.parse(PRETTY).units[0].lessons[0].newWords.concat(['סֶפֶל']);
        const drafted = await api('PATCH', '/api/plans/' + ID + '/fields', {
            body: { path: LESSON, field: 'newWords', value: words, expectedRevision: 1 }
        });
        assert.equal(drafted.status, 200, drafted.text);

        const res = await api('GET', '/api/plans/' + ID + '/issues');
        assert.equal(res.status, 200, res.text);
        assert.equal(res.json.revision, 2);
        assert.equal(res.json.status, 'draft');
        assert.equal(res.json.truncated, false);
        assert.equal(res.json.counts.block >= 1, true);
        const block = res.json.issues.find((i) => i.code === 'N1');
        assert.equal(block.severity, 'block');
        assert.equal(block.hard, true);
        assert.equal(typeof block.path, 'string');
        assert.equal(typeof block.pointer, 'string');
        const part = await api('GET', '/api/plans/' + ID + '/parts?path=' + encodeURIComponent(block.path));
        assert.equal(part.status, 200, 'the issue path is one read_part accepts');

        const onlyCode = await api('GET', '/api/plans/' + ID + '/issues?code=N1&severity=all&limit=5');
        assert.equal(onlyCode.status, 200);
        assert.equal(onlyCode.json.issues.every((i) => i.code === 'N1'), true);
        const list = await api('GET', '/api/plans/' + ID + '/issues?severity=block,warn');
        assert.equal(list.status, 200);
        assert.equal(list.json.issues.every((i) => i.severity === 'block' || i.severity === 'warn'), true);
        const prefixed = await api('GET', '/api/plans/' + ID + '/issues?pathPrefix=' + encodeURIComponent('units/unit-1/lessons/nowhere'));
        assert.equal(prefixed.json.issues.length, 0);
    });

    it('ac-5: GET /api/plan-schema is the validator\'s own schema file, byte for byte, admin only, no cookie', async () => {
        const validate = require('../server/plans/validate');
        const onDisk = fs.readFileSync(validate.SCHEMA_PATH);
        const res = await fetch(baseUrl + '/api/plan-schema', { headers: { Cookie: adminCookie } });
        assert.equal(res.status, 200);
        const buf = Buffer.from(await res.arrayBuffer());
        assert.equal(buf.equals(onDisk), true);
        assert.equal(buf.toString('utf8'), validate.schemaText());
        assert.match(res.headers.get('content-type'), /^application\/json/);
        assert.equal(res.headers.get('set-cookie'), null);
        assert.equal(res.headers.get('x-content-type-options'), 'nosniff');
        assert.equal(JSON.parse(buf.toString('utf8')).$schema, 'http://json-schema.org/draft-07/schema#');
        const etag = res.headers.get('etag');
        assert.match(etag, /^"[0-9a-f]{32}"$/);

        const cached = await fetch(baseUrl + '/api/plan-schema', { headers: { Cookie: adminCookie, 'If-None-Match': etag } });
        assert.equal(cached.status, 304);
        assert.equal(await cached.text(), '');
        assert.equal(cached.headers.get('set-cookie'), null);

        const signedOut = await api('GET', '/api/plan-schema', { cookie: null });
        assert.equal(signedOut.status, 401);
        assert.equal(signedOut.text.includes('definitions'), false);
        const asLearner = await api('GET', '/api/plan-schema', { cookie: learner() });
        assert.equal(asLearner.status, 403);
        assert.equal(asLearner.text.includes('definitions'), false);
    });

    it('ac-8: a failed create or restore does not change the tables; an existing plan keeps its revision log', async () => {
        const rev = await editedPlan();
        const before = tables();
        const exists = await api('POST', '/api/plans', { body: { id: ID, title: 'Overwrite attempt' } });
        assert.equal(exists.status, 400);
        assert.equal(exists.json.code, 'already_exists');
        assert.equal(tables(), before);
        assert.equal(store.loadWorkingCopy(ID).revision, rev);
    });
});
