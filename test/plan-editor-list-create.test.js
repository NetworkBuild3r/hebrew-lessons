/**
 * The Plans tab, the New plan form, and the assign control (INIT-008/SPEC-013), on a fake DOM
 * with a stand-in for the plan routes. A browser run is test/chrome-plan-editor-smoke.js.
 * Provenance: INIT-008/SPEC-013
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { loadPanels, all, byAttr, sleep } = require('./helpers/plan-panels-vm');
const { SCHEMA } = require('./helpers/plan-editor-fixtures');
const { makeEditor } = require('./helpers/plan-editor-fixtures');

const ADMIN_DIR = path.resolve(__dirname, '..', 'shared', 'admin');
const text = (node) => node.textContent;
const one = (node, name, value) => byAttr(node, name, value)[0];

const PLANS = [
    { id: 'alpha', title: 'Alpha plan', audience: 'shared', assigned: false, assigneeId: null, status: 'published', revision: 2, updatedAt: '2026-10-12T20:00:00.000Z', published: true, unpublishedChanges: false, humanEditedFields: 0 },
    { id: 'beta', title: 'Beta plan', audience: 'personal', assigned: true, assigneeId: 'example-learner', status: 'draft', revision: 7, updatedAt: '2026-10-12T21:30:00.000Z', published: true, unpublishedChanges: true, humanEditedFields: 3 },
    { id: 'gamma', title: null, audience: 'shared', assigned: false, assigneeId: null, status: 'draft', revision: 1, updatedAt: '2026-10-12T19:00:00.000Z', published: false, unpublishedChanges: false, humanEditedFields: 0 }
];

async function mountList(env, panel) {
    const stop = env.W.HebrewPlanList.mount(panel, { doc: env.W.document, api: env.W.HebrewApi, setHash: (r) => env.hashes.push(r) });
    await env.sleep(30);
    return stop;
}

describe('plan list: pure helpers', () => {
    const List = require('../shared/admin/plan-list.js');

    it('checkNew uses the schema pattern and says which field is wrong', () => {
        assert.deepEqual(List.checkNew({ id: 'good-id', title: 'T' }), {});
        assert.equal(List.checkNew({ id: 'Bad Id', title: 'T' }).id, 'invalid_id');
        assert.equal(List.checkNew({ id: 'a--b', title: 'T' }).id, 'invalid_id');
        assert.equal(List.checkNew({ id: 'x'.repeat(65), title: 'T' }).id, 'invalid_id');
        assert.equal(List.checkNew({ id: 'ok', title: '   ' }).title, 'empty_title');
        assert.equal(List.checkNew({ id: 'ok', title: 'x'.repeat(41) }, { titleMax: 40 }).title, 'title_too_long');
        // a pattern from the schema replaces the fallback
        assert.equal(List.checkNew({ id: 'abc', title: 'T' }, { slug: /^z+$/ }).id, 'invalid_id');
    });

    it('limitsFromSchema reads the id pattern, the title limit, and the recipes from the schema', () => {
        const limits = List.limitsFromSchema(SCHEMA);
        assert.equal(limits.slug.test('good-id'), true);
        assert.equal(limits.slug.test('Bad'), false);
        assert.deepEqual(limits.recipes, SCHEMA.definitions.recipe.enum);
        assert.equal(limits.recipes.length, 6);
        assert.equal(limits.titleMax, SCHEMA.definitions.plainText.maxLength);
        // no schema: the fallbacks, and no recipe list to copy
        const none = List.limitsFromSchema(null);
        assert.deepEqual(none.recipes, []);
        assert.equal(none.slug.test('good-id'), true);
    });

    it('rowsOf, statusOf, audienceOf describe a row, and a plan with no title lists by its id', () => {
        const rows = List.rowsOf({ plans: PLANS });
        assert.deepEqual(rows.map((r) => r.id), ['alpha', 'beta', 'gamma']);
        assert.equal(rows[0].status.key, 'published');
        assert.equal(rows[1].status.key, 'draft');
        assert.equal(rows[1].unpublishedChanges, true);
        assert.equal(rows[1].audience.who, 'example-learner');
        assert.equal(rows[1].humanEditedFields, 3);
        assert.equal(rows[2].title, 'gamma');
        assert.deepEqual(List.rowsOf(null), []);
        assert.deepEqual(List.rowsOf({ plans: [null, { id: 3 }, 'x'] }), []);
    });

    it('activeUsers keeps active accounts only, sorted by username', () => {
        const users = [
            { username: 'zed', status: 'active' }, { username: 'retired', status: 'disabled' },
            { username: 'amy', status: 'active' }, { username: '', status: 'active' }, null
        ];
        assert.deepEqual(List.activeUsers(users).map((u) => u.username), ['amy', 'zed']);
        assert.deepEqual(List.activeUsers(undefined), []);
    });
});

describe('ac-1: the list renders one row per plan', () => {
    it('shows every column, a link per plan, and a count', async () => {
        const env = loadPanels({ plans: PLANS });
        const panel = env.panel();
        await mountList(env, panel);
        const rows = byAttr(panel, 'data-plan-row');
        assert.deepEqual(rows.map((r) => r.getAttribute('data-plan-row')), ['alpha', 'beta', 'gamma']);
        assert.deepEqual(rows.map((r) => r.getAttribute('data-plan-row-status')), ['published', 'draft', 'draft']);
        const links = byAttr(panel, 'data-plan-open-link');
        assert.deepEqual(links.map((l) => l.getAttribute('href')), ['#plan/alpha', '#plan/beta', '#plan/gamma']);
        assert.equal(text(links[0]), 'Alpha plan');
        assert.equal(text(links[2]), 'gamma', 'a plan with no title shows its id');
        const beta = rows[1];
        assert.match(text(beta), /Personal/);
        assert.equal(text(one(beta, 'data-plan-assignee')), 'example-learner');
        assert.ok(one(beta, 'data-plan-unpublished'), 'beta has unpublished changes');
        assert.equal(byAttr(rows[0], 'data-plan-unpublished').length, 0);
        assert.equal(one(beta, 'data-plan-human-fields').getAttribute('data-plan-human-fields'), '3');
        assert.match(text(beta), /7/);
        assert.match(text(beta), /2026-10-12 21:30/);
        // a real table: header cells with scope, and a label on every cell (for the card layout)
        const heads = all(panel, (n) => n.tagName === 'TH');
        assert.equal(heads.length, 6);
        heads.forEach((h) => assert.equal(h.getAttribute('scope'), 'col'));
        all(beta, (n) => n.tagName === 'TD').forEach((td) => assert.ok(td.getAttribute('data-label')));
        assert.equal(env.callsOf('GET', /^\/api\/plans$/).length, 1);
    });

    it('an empty list says so and opens the New plan form', async () => {
        const env = loadPanels({ plans: [] });
        const panel = env.panel();
        await mountList(env, panel);
        assert.equal(one(panel, 'data-plan-list-empty').hidden, false);
        assert.equal(one(panel, 'data-plan-list').hidden, true);
        assert.ok(one(panel, 'data-plan-new'), 'the form is open when there is nothing to open');
        assert.match(text(one(panel, 'data-plan-list-empty')), /New plan/);
    });

    it('a truncated answer says so; a load error shows the server code and retries', async () => {
        const env = loadPanels({ plans: PLANS, plansTruncated: true });
        const panel = env.panel();
        await mountList(env, panel);
        assert.equal(one(panel, 'data-plan-list-truncated').hidden, false);

        const env2 = loadPanels({ plans: PLANS });
        env2.on('GET', /^\/api\/plans$/, () => env2.json(403, { error: 'Forbidden', code: 'forbidden' }));
        const panel2 = env2.panel();
        await mountList(env2, panel2);
        const err = one(panel2, 'data-plan-error');
        assert.equal(err.hidden, false);
        assert.match(text(err), /forbidden/);
        assert.equal(one(panel2, 'data-plan-list').hidden, true);
        // retry reads again
        env2.calls.length = 0;
        one(panel2, 'data-plan-list-retry').click();
        await env2.sleep(20);
        assert.equal(env2.callsOf('GET', /^\/api\/plans$/).length, 1);
    });

    it('clicking a row (not its link) opens the plan through the hash', async () => {
        const env = loadPanels({ plans: PLANS });
        const panel = env.panel();
        await mountList(env, panel);
        const cell = all(one(panel, 'data-plan-row', 'beta'), (n) => n.tagName === 'TD')[2];
        cell.click();
        assert.deepEqual(env.hashes, ['plan/beta']);
        env.hashes.length = 0;
        one(panel, 'data-plan-open-link', 'alpha').click();
        assert.deepEqual(env.hashes, [], 'the link changes the hash itself; the row handler stays out of its way');
    });
});

describe('ac-2: the New plan form', () => {
    async function openForm(env) {
        const panel = env.panel();
        const stop = await mountList(env, panel);
        one(panel, 'data-plan-new-open').click();
        return { panel, stop };
    }
    const fill = (panel, id, title, recipe) => {
        one(panel, 'data-plan-new-id').value = id;
        one(panel, 'data-plan-new-title').value = title;
        const select = one(panel, 'data-plan-new-recipe');
        if (select) select.value = recipe || '';
    };

    it('the recipe options equal the schema recipe enum, plus "no recipe"', async () => {
        const env = loadPanels({ plans: PLANS });
        const { panel } = await openForm(env);
        const options = all(one(panel, 'data-plan-new-recipe'), (n) => n.tagName === 'OPTION').map((o) => o.value);
        assert.deepEqual(options, [''].concat(SCHEMA.definitions.recipe.enum));
        // every label has Hebrew and an English gloss
        all(one(panel, 'data-plan-new-recipe'), (n) => n.tagName === 'OPTION' && n.value).forEach((o) => {
            assert.match(text(o), /[\u0590-\u05FF]/);
            assert.match(text(o), /\([A-Za-z ]+\)/);
        });
        assert.match(text(one(panel, 'data-plan-new')), /writes no Hebrew/);
        assert.equal(env.callsOf('GET', /plan-schema/).length, 1);
    });

    it('sends exactly one POST /api/plans with id, title and recipe, then opens the plan', async () => {
        const env = loadPanels({ plans: PLANS });
        const { panel } = await openForm(env);
        fill(panel, '  my-plan  ', '  My plan  ', 'guide');
        one(panel, 'data-plan-create').click();
        await env.sleep(20);
        const posts = env.callsOf('POST', /^\/api\/plans$/);
        assert.equal(posts.length, 1);
        assert.deepEqual(posts[0].body, { id: 'my-plan', title: 'My plan', recipe: 'guide' });
        assert.deepEqual(env.hashes, ['plan/my-plan']);
    });

    it('sends no recipe key when none is chosen', async () => {
        const env = loadPanels({ plans: PLANS });
        const { panel } = await openForm(env);
        fill(panel, 'plain', 'Plain', '');
        one(panel, 'data-plan-create').click();
        await env.sleep(20);
        assert.deepEqual(env.callsOf('POST', /^\/api\/plans$/)[0].body, { id: 'plain', title: 'Plain' });
    });

    it('a bad id or an empty title sends nothing and says why, keeping the typed values', async () => {
        const env = loadPanels({ plans: PLANS });
        const { panel } = await openForm(env);
        fill(panel, 'Not A Slug', '', 'guide');
        one(panel, 'data-plan-create').click();
        await env.sleep(10);
        assert.equal(env.callsOf('POST', /^\/api\/plans$/).length, 0);
        assert.match(text(one(panel, 'data-plan-new-id-message')), /invalid_id/);
        assert.match(text(one(panel, 'data-plan-new-title-message')), /empty_title/);
        assert.equal(one(panel, 'data-plan-new-id').value, 'Not A Slug');
        assert.equal(one(panel, 'data-plan-new-recipe').value, 'guide');
    });

    it('already_exists and invalid_id from the server are plain messages and lose nothing', async () => {
        const env = loadPanels({ plans: PLANS });
        env.on('POST', /^\/api\/plans$/, () => env.json(400, { error: 'Invalid request', code: 'already_exists' }));
        const { panel } = await openForm(env);
        fill(panel, 'alpha', 'Again', 'verb-workshop');
        one(panel, 'data-plan-create').click();
        await env.sleep(20);
        const message = one(panel, 'data-plan-new-message');
        assert.equal(message.hidden, false);
        assert.match(text(message), /already_exists/);
        assert.match(text(message), /already exists/);
        assert.equal(one(panel, 'data-plan-new-id').value, 'alpha');
        assert.equal(one(panel, 'data-plan-new-title').value, 'Again');
        assert.equal(one(panel, 'data-plan-new-recipe').value, 'verb-workshop');
        assert.deepEqual(env.hashes, [], 'nothing opened');
        assert.equal(one(panel, 'data-plan-create').disabled, false, 'the button is usable again');
    });

    it('unmount removes the listeners', async () => {
        const env = loadPanels({ plans: PLANS });
        const { panel, stop } = await openForm(env);
        assert.ok(panel.listeners.click.length >= 1);
        stop();
        assert.equal(panel.listeners.click.length, 0);
        assert.equal(panel.listeners.submit.length, 0);
    });
});

describe('ac-3: the assign control', () => {
    const USERS = [
        { id: 2, username: 'example-learner', display_name: 'Example Learner', role: 'user', status: 'active' },
        { id: 3, username: 'retired', display_name: 'Retired', role: 'user', status: 'disabled' },
        { id: 1, username: 'example-admin', display_name: 'Admin', role: 'admin', status: 'active' }
    ];
    function setup(plan) {
        const env = loadPanels({ users: USERS });
        const changes = { assigned: [], conflicts: [] };
        let revision = 4;
        const assign = env.W.HebrewPlanList.createAssign({
            doc: env.W.document, api: env.W.HebrewApi, planId: 'sample',
            getRevision: () => revision,
            onAssigned: (data) => { changes.assigned.push(data); revision = data.revision; },
            onConflict: (data) => changes.conflicts.push(data)
        });
        assign.update(plan || { audience: 'shared', assigneeId: null, revision: 4 });
        return { env, assign, changes, setRevision: (n) => { revision = n; } };
    }

    it('offers active accounts only, and has no free-text control', async () => {
        const { env, assign } = setup();
        await assign.load();
        const select = one(assign.el, 'data-plan-assign-user');
        assert.deepEqual(all(select, (n) => n.tagName === 'OPTION').map((o) => o.value), ['', 'example-admin', 'example-learner']);
        // only <select>s: nothing to type an account name into
        assert.deepEqual(all(assign.el, (n) => n.tagName === 'INPUT' || n.tagName === 'TEXTAREA'), []);
        assert.equal(env.callsOf('GET', /admin\/users/).length, 1);
        await assign.load();
        assert.equal(env.callsOf('GET', /admin\/users/).length, 1, 'accounts are read once');
    });

    it('shows the account list only for a personal plan', async () => {
        const { assign } = setup();
        await assign.load();
        const audience = one(assign.el, 'data-plan-assign-audience');
        assert.equal(one(assign.el, 'data-plan-assign-user').parentNode.hidden, true);
        audience.value = 'personal';
        audience.dispatch('change');
        assert.equal(one(assign.el, 'data-plan-assign-user').parentNode.hidden, false);
    });

    it('sends one POST .../assign with the audience, the account, and the revision', async () => {
        const { env, assign, changes } = setup();
        await assign.load();
        const audience = one(assign.el, 'data-plan-assign-audience');
        audience.value = 'personal';
        audience.dispatch('change');
        one(assign.el, 'data-plan-assign-user').value = 'example-learner';
        one(assign.el, 'data-plan-assign-save').click();
        await env.sleep(20);
        const posts = env.callsOf('POST', /\/assign$/);
        assert.equal(posts.length, 1);
        assert.deepEqual(posts[0].body, { audience: 'personal', assigneeId: 'example-learner', expectedRevision: 4 });
        assert.equal(changes.assigned.length, 1);
        assert.equal(env.server.plan.assigneeId, 'example-learner');
    });

    it('a shared plan sends no assignee; a personal plan with no account sends nothing', async () => {
        const { env, assign } = setup({ audience: 'personal', assigneeId: 'example-learner', revision: 4 });
        await assign.load();
        const audience = one(assign.el, 'data-plan-assign-audience');
        assert.equal(audience.value, 'personal');
        one(assign.el, 'data-plan-assign-user').value = '';
        one(assign.el, 'data-plan-assign-save').click();
        await env.sleep(10);
        assert.equal(env.callsOf('POST', /\/assign$/).length, 0);
        assert.match(text(one(assign.el, 'data-plan-assign-message')), /Choose an account/);
        audience.value = 'shared';
        audience.dispatch('change');
        one(assign.el, 'data-plan-assign-save').click();
        await env.sleep(20);
        assert.deepEqual(env.callsOf('POST', /\/assign$/)[0].body, { audience: 'shared', expectedRevision: 4 });
    });

    it('assignee_refused is shown plainly and nothing changes', async () => {
        const { env, assign, changes } = setup();
        env.on('POST', /\/assign$/, () => env.json(400, { error: 'Invalid request', code: 'assignee_refused' }));
        await assign.load();
        const audience = one(assign.el, 'data-plan-assign-audience');
        audience.value = 'personal';
        audience.dispatch('change');
        one(assign.el, 'data-plan-assign-user').value = 'example-learner';
        one(assign.el, 'data-plan-assign-save').click();
        await env.sleep(20);
        const message = one(assign.el, 'data-plan-assign-message');
        assert.equal(message.hidden, false);
        assert.match(text(message), /assignee_refused/);
        assert.match(text(message), /not active/);
        assert.equal(changes.assigned.length, 0);
        assert.equal(one(assign.el, 'data-plan-assign-save').disabled, false);
    });

    it('a stale revision is a conflict handed to the host', async () => {
        const { env, assign, changes, setRevision } = setup();
        await assign.load();
        setRevision(2);
        one(assign.el, 'data-plan-assign-save').click();
        await env.sleep(20);
        assert.equal(changes.conflicts.length, 1);
        assert.equal(changes.conflicts[0].error, 'revision_conflict');
        assert.match(text(one(assign.el, 'data-plan-assign-message')), /revision_conflict/);
    });

    it('a current assignee whose account is no longer active stays in the list, marked', async () => {
        const { assign } = setup({ audience: 'personal', assigneeId: 'retired', revision: 4 });
        await assign.load();
        const options = all(one(assign.el, 'data-plan-assign-user'), (n) => n.tagName === 'OPTION');
        const retired = options.find((o) => o.value === 'retired');
        assert.ok(retired);
        assert.match(text(retired), /not active/);
        assert.equal(one(assign.el, 'data-plan-assign-user').value, 'retired');
    });

    it('the plan form shows audience and assignee read-only, and no other control sends them', () => {
        const { editor, host } = makeEditor();
        const plan = { id: 'p', title: 'T', audience: 'personal', assigneeId: 'example-learner', status: 'draft', revision: 2, schemaVersion: '1.1.0', language: { target: 'he', gloss: 'en' }, units: [] };
        const form = editor.renderNode({ path: '', node: plan, kind: 'plan' });
        host.appendChild(form.el);
        ['audience', 'assigneeId'].forEach((prop) => {
            const handle = form.fields[prop];
            assert.ok(handle, prop + ' has a row');
            assert.equal(handle.widgetName, 'readonly');
            assert.equal(byAttr(handle.row, 'data-plan-save').length, 0, prop + ' has no save button');
        });
        // and no module but plan-list.js names the assignee in a request
        const names = fs.readdirSync(ADMIN_DIR).filter((n) => /^plan-.*\.js$/.test(n));
        const sending = names.filter((n) => /assigneeId\s*:/.test(fs.readFileSync(path.join(ADMIN_DIR, n), 'utf8')));
        assert.deepEqual(sending.sort(), ['plan-list.js']);
        const audienceSenders = names.filter((n) => /audience\s*:\s*audience/.test(fs.readFileSync(path.join(ADMIN_DIR, n), 'utf8')));
        assert.deepEqual(audienceSenders, ['plan-list.js']);
    });
});

describe('wiring: the admin route loads the new scripts in order, behind the admin guard', () => {
    const routes = require('../shared/app-routes.js');
    const admin = routes.find((route) => route.id === 'admin');

    it('is admin-only and loads plan-ui before the panels, and the block renderers before plan-render', () => {
        assert.equal(admin.adminOnly, true);
        const at = (file) => admin.scripts.indexOf(file);
        ['plan-ui', 'plan-hints', 'plan-widgets', 'plan-form', 'plan-list', 'plan-issues', 'plan-history', 'plan-preview', 'plan-builder']
            .forEach((name) => assert.ok(at('/shared/admin/' + name + '.js') > -1, name + ' is loaded'));
        assert.ok(at('/shared/admin/plan-ui.js') < at('/shared/admin/plan-list.js'));
        assert.ok(at('/shared/admin/plan-form.js') < at('/shared/admin/plan-list.js'));
        assert.ok(at('/shared/admin/plan-preview.js') < at('/shared/admin/plan-builder.js'));
        assert.ok(at('/shared/admin/plan-builder.js') < at('/shared/admin/admin-shell.js'));
        ['explainer', 'conjugation-table', 'personal-question', 'verse-reading', 'letter-intro'].forEach((name) => {
            const file = '/shared/ui/blocks/' + name + '.js';
            assert.ok(at(file) > -1 && at(file) < at('/shared/ui/plan-render.js'), file + ' loads before plan-render.js');
        });
        // every script it names exists
        admin.scripts.filter((s) => s.indexOf('/shared/') === 0).forEach((s) => {
            assert.ok(fs.existsSync(path.resolve(__dirname, '..', s.slice(1))), s + ' exists');
        });
    });

    it('no other route loads a plan screen', () => {
        routes.filter((route) => route.id !== 'admin').forEach((route) => {
            (route.scripts || []).forEach((s) => assert.doesNotMatch(s, /\/admin\/plan-/));
        });
    });
});

describe('the Plans tab through the builder seam', () => {
    it('mountPicker mounts the list, and unmount removes it again', async () => {
        const env = loadPanels({ plans: PLANS });
        const panel = env.panel();
        env.W.HebrewPlanBuilder.mountPicker(panel);
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-row').length, 3);
        assert.ok(panel.listeners.click.length >= 1);
        env.W.HebrewPlanBuilder.unmount();
        assert.equal(panel.listeners.click.length, 0);
    });

    it('mounting a plan after the list leaves no list listener behind', async () => {
        const env = loadPanels({ plans: PLANS });
        const panel = env.panel();
        env.W.HebrewPlanBuilder.mountPicker(panel);
        await sleep(40);
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        assert.equal(byAttr(panel, 'data-plan-list').length, 0, 'the list markup was replaced');
        env.W.HebrewPlanBuilder.unmount();
        assert.equal(panel.listeners.click.length, 0);
        assert.equal(panel.listeners.input.length, 0);
    });
});
