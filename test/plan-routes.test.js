/**
 * Plan HTTP routes: admin writes, revision conflicts, learner snapshot reads.
 * Temporary database, no ElevenLabs or OpenAI calls.
 * Provenance: INIT-004/SPEC-004
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const harness = require('./helpers/signed-in-server');

const SAMPLE = path.join(
    harness.REPO_ROOT,
    'docs',
    'decisions',
    'ADR-0003-assets',
    'samples',
    'beginner-unit-1.plan.json'
);
const PRETTY = fs.readFileSync(SAMPLE, 'utf8');
const LESSON = 'units/unit-1/lessons/cafe-order';
const LISTEN = LESSON + '/blocks/listen';
const WORD = LESSON + '/blocks/introduction/words/2';
const EXTRA = {
    id: 'extra',
    type: 'listening',
    passage: 'אֲנִי רוֹצֶה קָפֶה.'
};

const WRITE_BODIES = {
    replacePart: () => ({
        path: WORD,
        value: { id: '2', word: 'קָפֶה', meaning: 'coffee', sound: 'ka-FE' },
        expectedRevision: 1
    }),
    insertBlock: () => ({
        path: LESSON,
        index: 1,
        block: EXTRA,
        expectedRevision: 1
    }),
    moveBlock: () => ({
        path: LISTEN,
        index: 0,
        expectedRevision: 1
    }),
    removeBlock: () => ({
        path: LISTEN,
        expectedRevision: 1
    }),
    setField: () => ({
        path: WORD,
        field: 'meaning',
        value: 'espresso',
        expectedRevision: 1
    }),
    publishPlan: () => ({ expectedRevision: 1 })
};

function samplePlan(id, edit) {
    const plan = JSON.parse(PRETTY);
    plan.id = id;
    if (edit) edit(plan);
    return plan;
}

function eachLesson(plan, visit) {
    for (const unit of plan.units) {
        for (const lesson of unit.lessons) visit(lesson);
    }
}

describe('plan HTTP routes (INIT-004/SPEC-004)', { concurrency: false }, () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(username: string) => string} */
    let cookieFor;
    /** @type {Record<string, { id: number, username: string, role: string, token: string }>} */
    let users;
    /** @type {typeof import('../server/plans/store')} */
    let store;
    /** @type {typeof import('../server/plans/ops')} */
    let ops;
    /** @type {typeof import('../server/plans/publish')} */
    let publish;
    /** @type {typeof import('../server/db/repos/visibility')} */
    let visibilityRepo;
    /** @type {string[]} */
    let calls;
    /** @type {() => void} */
    let restoreSpies;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        users = started.users;

        const accounts = require('../server/accounts');
        const cookies = require('../server/http/cookies');
        const assignee = accounts.createUser({
            username: 'assignee1',
            password: harness.GOOD_PASSWORD,
            role: 'user',
            displayName: 'The Assignee',
            actor: 'test'
        });
        const stranger = accounts.createUser({
            username: 'stranger1',
            password: harness.GOOD_PASSWORD,
            role: 'user',
            displayName: 'assignee1',
            actor: 'test'
        });
        const assigneeSession = accounts.signIn('assignee1', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        const strangerSession = accounts.signIn('stranger1', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        users.assignee1 = {
            id: assignee.id,
            username: assignee.username,
            role: assignee.role,
            token: assigneeSession.token
        };
        users.stranger1 = {
            id: stranger.id,
            username: stranger.username,
            role: stranger.role,
            token: strangerSession.token
        };
        const cookieName = cookies.cookieName();
        const prior = cookieFor;
        cookieFor = (username) => {
            const entry = users[username];
            if (entry && entry.token && username !== 'admin' && username !== 'learner') {
                return cookieName + '=' + encodeURIComponent(entry.token);
            }
            return prior(username);
        };

        store = require('../server/plans/store');
        ops = require('../server/plans/ops');
        publish = require('../server/plans/publish');
        visibilityRepo = require('../server/db/repos/visibility');

        calls = [];
        const originals = {
            readPart: ops.readPart,
            replacePart: ops.replacePart,
            insertBlock: ops.insertBlock,
            moveBlock: ops.moveBlock,
            removeBlock: ops.removeBlock,
            setField: ops.setField,
            publishPlan: publish.publishPlan
        };
        function wrap(name, fn) {
            return function wrapped(...args) {
                calls.push(name);
                return fn.apply(this, args);
            };
        }
        ops.readPart = wrap('readPart', originals.readPart);
        ops.replacePart = wrap('replacePart', originals.replacePart);
        ops.insertBlock = wrap('insertBlock', originals.insertBlock);
        ops.moveBlock = wrap('moveBlock', originals.moveBlock);
        ops.removeBlock = wrap('removeBlock', originals.removeBlock);
        ops.setField = wrap('setField', originals.setField);
        publish.publishPlan = wrap('publishPlan', originals.publishPlan);
        restoreSpies = () => {
            ops.readPart = originals.readPart;
            ops.replacePart = originals.replacePart;
            ops.insertBlock = originals.insertBlock;
            ops.moveBlock = originals.moveBlock;
            ops.removeBlock = originals.removeBlock;
            ops.setField = originals.setField;
            publish.publishPlan = originals.publishPlan;
        };
    });

    after(async () => {
        if (restoreSpies) restoreSpies();
        await harness.stop();
    });

    beforeEach(() => {
        calls.length = 0;
        const db = require('../server/db');
        db.run('DELETE FROM plans');
    });

    function seed(plan) {
        const document = JSON.stringify(plan);
        store.saveWorkingCopy({
            id: plan.id,
            document: document,
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return document;
    }

    /**
     * @param {string} method
     * @param {string} urlPath
     * @param {{ cookie?: string, body?: object, origin?: boolean, contentType?: boolean }} [opts]
     */
    async function api(method, urlPath, opts) {
        const options = opts || {};
        /** @type {Record<string, string>} */
        const headers = {};
        if (options.cookie) headers.Cookie = options.cookie;
        const sendJson = options.body !== undefined;
        if (sendJson && options.contentType !== false) {
            headers['Content-Type'] = 'application/json';
        }
        if (options.origin !== false && method !== 'GET') headers.Origin = baseUrl;
        const res = await fetch(baseUrl + urlPath, {
            method,
            headers,
            body: sendJson ? JSON.stringify(options.body) : undefined
        });
        const text = await res.text();
        let json = null;
        if (text) {
            try {
                json = JSON.parse(text);
            } catch (_) {
                json = null;
            }
        }
        return { status: res.status, text, json };
    }

    it('ac-1: each write route calls only its function', async () => {
        const planRoutes = require('../server/api/plan-routes');
        assert.equal(planRoutes.WRITE_ROUTES.length, 6);
        for (const route of planRoutes.WRITE_ROUTES) {
            calls.length = 0;
            const plan = samplePlan('beginner-unit-1');
            const before = seed(plan);
            const urlPath = route.path.replace(':id', plan.id);
            const res = await api(route.method, urlPath, {
                cookie: cookieFor('admin'),
                body: WRITE_BODIES[route.fn]()
            });
            assert.equal(res.status, 200, route.fn + ' ' + res.text);
            assert.deepEqual(calls, [route.fn]);
            const row = store.loadWorkingCopy(plan.id);
            assert.notEqual(row.document, before);
            assert.equal(row.revision, 2);
        }
    });

    it('ac-1: GET parts calls readPart and does not write', async () => {
        const plan = samplePlan('beginner-unit-1');
        const before = seed(plan);
        const res = await api(
            'GET',
            '/api/plans/beginner-unit-1/parts?path=' + encodeURIComponent(WORD),
            { cookie: cookieFor('admin') }
        );
        assert.equal(res.status, 200);
        assert.equal(res.json.part.meaning, 'coffee');
        assert.equal(res.json.revision, 1);
        assert.deepEqual(calls, ['readPart']);
        assert.equal(store.loadWorkingCopy(plan.id).document, before);
        assert.equal(store.loadSnapshot(plan.id), null);
    });

    it('ac-2: a stale revision is 409 revision_conflict and the document is unchanged', async () => {
        const plan = samplePlan('beginner-unit-1');
        const before = seed(plan);
        const rowBefore = store.loadWorkingCopy(plan.id);
        const res = await api('PATCH', '/api/plans/beginner-unit-1/fields', {
            cookie: cookieFor('admin'),
            body: {
                path: WORD,
                field: 'meaning',
                value: 'espresso',
                expectedRevision: 99
            }
        });
        assert.equal(res.status, 409);
        assert.deepEqual(res.json, { error: 'revision_conflict', revision: 1 });
        const row = store.loadWorkingCopy(plan.id);
        assert.equal(row.document, before);
        assert.equal(row.revision, 1);
        assert.equal(row.updatedAt, rowBefore.updatedAt);
        assert.equal(store.loadSnapshot(plan.id), null);
        assert.deepEqual(calls, ['setField']);
    });

    it('ac-3: personal snapshot, selected grant, and signed-out 401', async () => {
        const personal = samplePlan('personal-cafe', (plan) => {
            plan.audience = 'personal';
            plan.assigneeId = 'assignee1';
        });
        seed(personal);
        const publishedPersonal = publish.publishPlan(store, personal.id, { expectedRevision: 1 });
        assert.equal(publishedPersonal.error, undefined, JSON.stringify(publishedPersonal.blocks));
        calls.length = 0;

        const stranger = await api('GET', '/api/plans/personal-cafe/published', {
            cookie: cookieFor('stranger1')
        });
        assert.equal(stranger.status, 404);
        assert.equal(stranger.text.includes('newWords'), false);
        assert.equal(stranger.text.includes('assigneeId'), false);
        assert.equal(stranger.text.includes('רוֹצֶה'), false);

        const adminDenied = await api('GET', '/api/plans/personal-cafe/published', {
            cookie: cookieFor('admin')
        });
        assert.equal(adminDenied.status, 404);
        assert.equal(adminDenied.text.includes('assigneeId'), false);

        const assignee = await api('GET', '/api/plans/personal-cafe/published', {
            cookie: cookieFor('assignee1')
        });
        assert.equal(assignee.status, 200);
        assert.equal(assignee.json.audience, 'personal');
        assert.equal(assignee.json.assigneeId, 'assignee1');
        assert.equal(assignee.json.status, 'published');
        assert.equal(assignee.json.units[0].lessons[0].blocks[0].words[1].meaning, 'coffee');

        const selected = samplePlan('selected-cafe', (plan) => {
            eachLesson(plan, (lesson) => {
                lesson.visibility = 'selected';
            });
        });
        seed(selected);
        const publishedSelected = publish.publishPlan(store, selected.id, { expectedRevision: 1 });
        assert.equal(publishedSelected.error, undefined, JSON.stringify(publishedSelected.blocks));

        const ungranted = await api('GET', '/api/plans/selected-cafe/published', {
            cookie: cookieFor('stranger1')
        });
        assert.equal(ungranted.status, 404);
        assert.equal(ungranted.text.includes('newWords'), false);
        assert.equal(ungranted.text.includes('assigneeId'), false);

        const lessonIds = [];
        eachLesson(selected, (lesson) => lessonIds.push(lesson.id));
        assert.ok(lessonIds.length > 1);
        visibilityRepo.grant(lessonIds[0], users.assignee1.id);
        const partial = await api('GET', '/api/plans/selected-cafe/published', {
            cookie: cookieFor('assignee1')
        });
        assert.equal(partial.status, 404);
        assert.equal(partial.text.includes('newWords'), false);

        for (const id of lessonIds) visibilityRepo.grant(id, users.assignee1.id);
        const granted = await api('GET', '/api/plans/selected-cafe/published', {
            cookie: cookieFor('assignee1')
        });
        assert.equal(granted.status, 200);
        assert.equal(granted.json.id, 'selected-cafe');
        assert.equal(granted.json.status, 'published');
        assert.ok(Array.isArray(granted.json.units[0].lessons[0].newWords));

        const signedOut = await api('GET', '/api/plans/personal-cafe/published', {});
        assert.equal(signedOut.status, 401);
        assert.equal(signedOut.json.code, 'no_session');
        assert.equal(signedOut.text.includes('newWords'), false);
        assert.equal(signedOut.text.includes('assigneeId'), false);
        assert.deepEqual(calls.filter((name) => name !== 'publishPlan'), []);
    });

    it('ac-3: draft, hidden, and a later edit stay off the learner read', async () => {
        const draft = samplePlan('draft-cafe', (plan) => {
            eachLesson(plan, (lesson) => {
                lesson.visibility = 'everyone';
            });
        });
        seed(draft);
        const draftRead = await api('GET', '/api/plans/draft-cafe/published', {
            cookie: cookieFor('learner')
        });
        assert.equal(draftRead.status, 404);
        assert.equal(draftRead.text.includes('newWords'), false);

        const hidden = samplePlan('hidden-cafe');
        seed(hidden);
        const publishedHidden = publish.publishPlan(store, hidden.id, { expectedRevision: 1 });
        assert.equal(publishedHidden.error, undefined);
        const hiddenRead = await api('GET', '/api/plans/hidden-cafe/published', {
            cookie: cookieFor('learner')
        });
        assert.equal(hiddenRead.status, 404);
        assert.equal(hiddenRead.text.includes('newWords'), false);
        assert.equal(hiddenRead.text.includes('רוֹצֶה'), false);

        const shared = samplePlan('shared-cafe', (plan) => {
            eachLesson(plan, (lesson) => {
                lesson.visibility = 'everyone';
            });
        });
        seed(shared);
        const publishedShared = publish.publishPlan(store, shared.id, { expectedRevision: 1 });
        assert.equal(publishedShared.error, undefined);
        const seen = await api('GET', '/api/plans/shared-cafe/published', {
            cookie: cookieFor('learner')
        });
        assert.equal(seen.status, 200);
        assert.equal(seen.json.units[0].lessons[0].blocks[0].words[1].meaning, 'coffee');

        calls.length = 0;
        const edited = await api('PATCH', '/api/plans/shared-cafe/fields', {
            cookie: cookieFor('admin'),
            body: {
                path: WORD,
                field: 'meaning',
                value: 'espresso',
                expectedRevision: publishedShared.revision
            }
        });
        assert.equal(edited.status, 200);
        assert.equal(edited.json.plan.status, 'draft');
        assert.equal(edited.json.plan.units[0].lessons[0].blocks[0].words[1].meaning, 'espresso');
        const still = await api('GET', '/api/plans/shared-cafe/published', {
            cookie: cookieFor('learner')
        });
        assert.equal(still.status, 200);
        assert.equal(still.json.status, 'published');
        assert.equal(still.json.units[0].lessons[0].blocks[0].words[1].meaning, 'coffee');
        assert.equal(still.text.includes('espresso'), false);
        const missing = await api('GET', '/api/plans/does-not-exist/published', {
            cookie: cookieFor('learner')
        });
        assert.equal(missing.status, 404);
        assert.equal(missing.text, hiddenRead.text);
    });

    it('ac-4: a non-admin session is 403 on every write route and worksheet admin routes still gate', async () => {
        const planRoutes = require('../server/api/plan-routes');
        seed(samplePlan('beginner-unit-1'));
        for (const route of planRoutes.WRITE_ROUTES) {
            calls.length = 0;
            const res = await api(route.method, route.path.replace(':id', 'beginner-unit-1'), {
                cookie: cookieFor('learner'),
                body: WRITE_BODIES[route.fn]()
            });
            assert.equal(res.status, 403, route.fn);
            assert.equal(res.json.code, 'forbidden');
            assert.deepEqual(calls, []);
        }
        const parts = await api('GET', '/api/plans/beginner-unit-1/parts', {
            cookie: cookieFor('learner')
        });
        assert.equal(parts.status, 403);
        assert.equal(parts.json.code, 'forbidden');
        assert.deepEqual(calls, []);

        const learnerAdmin = await api('GET', '/api/admin/users', { cookie: cookieFor('learner') });
        assert.equal(learnerAdmin.status, 403);
        const adminUsers = await api('GET', '/api/admin/users', { cookie: cookieFor('admin') });
        assert.equal(adminUsers.status, 200);
        assert.ok(Array.isArray(adminUsers.json.users));
    });

    it('security: unknown JSON fields are rejected and the document is unchanged', async () => {
        const before = seed(samplePlan('beginner-unit-1'));
        const res = await api('PATCH', '/api/plans/beginner-unit-1/fields', {
            cookie: cookieFor('admin'),
            body: {
                path: WORD,
                field: 'meaning',
                value: 'espresso',
                expectedRevision: 1,
                status: 'published'
            }
        });
        assert.equal(res.status, 400);
        assert.equal(res.json.code, 'bad_request');
        assert.deepEqual(calls, []);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, before);
    });

    it('security: a write without Origin is the existing CSRF 403 and does not call ops', async () => {
        const before = seed(samplePlan('beginner-unit-1'));
        const res = await api('POST', '/api/plans/beginner-unit-1/publish', {
            cookie: cookieFor('admin'),
            body: { expectedRevision: 1 },
            origin: false
        });
        assert.equal(res.status, 403);
        assert.equal(res.json.code, 'csrf');
        assert.deepEqual(calls, []);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, before);
        assert.equal(store.loadSnapshot('beginner-unit-1'), null);
    });

    it('security: a signed-out write is 401 when the CSRF headers are present', async () => {
        seed(samplePlan('beginner-unit-1'));
        const res = await api('PATCH', '/api/plans/beginner-unit-1/fields', {
            body: {
                path: WORD,
                field: 'meaning',
                value: 'espresso',
                expectedRevision: 1
            }
        });
        assert.equal(res.status, 401);
        assert.equal(res.json.code, 'no_session');
        assert.deepEqual(calls, []);
    });
});
