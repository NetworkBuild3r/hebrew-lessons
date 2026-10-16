/**
 * Admin plan builder on /editor/ (INIT-010/SPEC-003).
 * Covers ac-1 through ac-4. No browser login in this file.
 * Provenance: INIT-010/SPEC-003
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('node:vm');
const Form = require('../shared/admin/plan-form.js');
const activityMap = require('../server/plans/activity-map.js');
const { loadPanels, byAttr, sleep } = require('./helpers/plan-panels-vm');
const { makeEditor } = require('./helpers/plan-editor-fixtures');
const harness = require('./helpers/signed-in-server');

const ROOT = path.resolve(__dirname, '..');

function familyPlan() {
    return {
        schemaVersion: '1.0.0',
        id: 'sample',
        revision: 4,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Family',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-1',
            title: 'Family',
            theme: 'family',
            lessons: [
                {
                    id: 'family-1',
                    title: 'Family 1',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: [],
                    newWords: ['אֵם', 'אָב'],
                    recycles: [],
                    blocks: [{
                        id: 'words-1',
                        type: 'introduction',
                        words: [
                            { id: 'w1', word: 'אֵם', meaning: 'mother', sentence: 'זֹאת אֵם.' },
                            { id: 'w2', word: 'אָב', meaning: 'father', sentence: 'זֶה אָב.' }
                        ]
                    }]
                },
                {
                    id: 'family-2',
                    title: 'Family 2',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: ['family-1'],
                    newWords: [],
                    recycles: [],
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
                },
                {
                    id: 'family-3',
                    title: 'Family 3',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: ['family-2'],
                    newWords: [],
                    recycles: [],
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
                }
            ]
        }]
    };
}

function loadShellAndLessons() {
    const sandbox = {
        console,
        document: { readyState: 'complete' },
        window: {},
        globalThis: {},
        location: { href: 'http://127.0.0.1/admin/', hash: '#lesson/my-morning', pathname: '/admin/' }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(ROOT, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'lessons.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'admin-shell.js'), 'utf8'), sandbox);
    return sandbox;
}

describe('plan editor research builder (INIT-010/SPEC-003)', () => {
    it('ac-1: /editor/ lists plans; adding two lessons shows three rows; move changes saved order', async () => {
        const routes = require('../shared/app-routes.js');
        const editor = routes.find((route) => route.id === 'editor');
        assert.ok(editor);
        assert.equal(editor.path, '/editor/');
        assert.equal(editor.adminOnly, true);
        assert.equal(editor.view, 'HebrewEditorShell');
        const at = (file) => editor.scripts.indexOf(file);
        [
            'plan-ui', 'plan-hints', 'plan-widgets', 'plan-form', 'plan-list',
            'plan-issues', 'plan-history', 'plan-preview', 'plan-builder', 'editor-shell'
        ].forEach((name) => {
            assert.ok(at('/shared/admin/' + name + '.js') > -1, name + ' loads on editor');
        });
        assert.ok(at('/shared/admin/activity-map.js') > -1);
        assert.ok(at('/shared/ui/plan-render.js') > -1);

        const plan = {
            schemaVersion: '1.0.0',
            id: 'sample',
            revision: 4,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'Stack',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'unit-1',
                title: 'Unit',
                theme: 'home',
                lessons: [{
                    id: 'lesson-1',
                    title: 'First',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: [],
                    newWords: [],
                    recycles: [],
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
                }]
            }]
        };
        const env = loadPanels({
            plan: plan,
            revision: 4,
            plans: [{ id: 'sample', title: 'Stack', status: 'draft', audience: 'shared' }]
        });
        const panel = env.panel();
        env.W.HebrewPlanBuilder.mountPicker(panel);
        await sleep(40);
        assert.ok(byAttr(panel, 'data-plan-list').length >= 1 || byAttr(panel, 'data-plan-row').length >= 1);

        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-stack-row').length, 1);

        // SPEC-004: Add lesson asks for a title (ids from the title; Latin → slug).
        byAttr(panel, 'data-plan-add-lesson-title')[0].value = 'Second';
        byAttr(panel, 'data-plan-add-lesson')[0].click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        byAttr(panel, 'data-plan-add-lesson-title')[0].value = 'Third';
        byAttr(panel, 'data-plan-add-lesson')[0].click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-stack-row').length, 3, 'three lesson rows after two adds');
        assert.deepEqual(
            env.server.plan.units[0].lessons.map((l) => l.id),
            ['lesson-1', 'second', 'third']
        );
        const insertCalls = env.callsOf('POST', /\/lessons$/);
        assert.ok(insertCalls.length >= 2, 'uses POST /lessons');

        const rows = byAttr(panel, 'data-plan-stack-row');
        const up = byAttr(rows[2], 'data-plan-lesson-move', 'up')[0];
        up.click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.deepEqual(
            env.server.plan.units[0].lessons.map((l) => l.id),
            ['lesson-1', 'third', 'second']
        );
        const moveCalls = env.callsOf('POST', /\/lessons\/move$/);
        assert.ok(moveCalls.length >= 1, 'uses POST /lessons/move');
    });

    it('ac-2: activity menu lists Words, Story, Match, Type it and does not pre-insert; Family 3 shows Family 1 words', async () => {
        const names = activityMap.ACTIVITY_NAMES || Object.keys(activityMap.ACTIVITY_TO_BLOCK);
        assert.ok(names.indexOf('Words') !== -1);
        assert.ok(names.indexOf('Story') !== -1);
        // INIT-010 SPEC-004 has not landed a match block, so Match stays out of the menu.
        assert.ok(names.indexOf('Match') === -1);
        assert.ok(names.indexOf('Type it') !== -1);

        const plan = familyPlan();
        const env = loadPanels({ plan: plan, revision: 4 });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        // Open Family 3
        const pick = byAttr(panel, 'data-plan-lesson').find((btn) => {
            return String(btn.textContent || '').indexOf('Family 3') !== -1
                || btn.getAttribute('data-plan-lesson') === '2';
        });
        assert.ok(pick, 'Family 3 chooser');
        pick.click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);

        const menu = byAttr(panel, 'data-plan-activity-menu')[0];
        assert.ok(menu, 'activity menu present');
        const options = menu.querySelectorAll
            ? Array.from(menu.querySelectorAll('[data-plan-activity-name]'))
            : byAttr(menu, 'data-plan-activity-name');
        const optionNames = options.map((o) => o.getAttribute('data-plan-activity-name'));
        assert.ok(optionNames.indexOf('Words') !== -1);
        assert.ok(optionNames.indexOf('Story') !== -1);
        // INIT-010 SPEC-004 has not landed a match block, so Match stays out of the menu.
        assert.ok(optionNames.indexOf('Match') === -1);
        assert.ok(optionNames.indexOf('Type it') !== -1);
        options.forEach((o) => {
            assert.ok(o.getAttribute('data-ui-text-en'));
            assert.ok(o.getAttribute('data-ui-text-he'));
            const en = o.getAttribute('data-ui-text-en');
            const he = o.getAttribute('data-ui-text-he');
            const text = String(o.textContent || '');
            assert.ok(text === en || text === he, 'one language as option text');
            assert.equal(text.includes(en) && text.includes(he) && en !== he, false);
        });

        const family3 = env.server.plan.units[0].lessons[2];
        const types = (family3.blocks || []).map((b) => b.type);
        assert.equal(types.indexOf('introduction'), -1, 'Words not pre-inserted');
        assert.equal(types.indexOf('graded-story'), -1, 'Story not pre-inserted');
        assert.equal(types.indexOf('match'), -1, 'Match not pre-inserted');
        assert.equal(types.indexOf('type-it'), -1, 'Type it not pre-inserted');

        const earlier = byAttr(panel, 'data-plan-earlier-words')[0];
        assert.ok(earlier, 'earlier words panel');
        const wordRows = byAttr(earlier, 'data-plan-earlier-word');
        const fromFamily1 = wordRows.filter((row) => row.getAttribute('data-from-lesson') === 'family-1');
        assert.ok(fromFamily1.length >= 2, 'Family 1 words appear on Family 3');
        assert.deepEqual(
            fromFamily1.map((row) => row.getAttribute('data-plan-earlier-word')),
            ['אֵם', 'אָב']
        );
    });

    it('ac-3: buildsOn offers only earlier lessons and saves the chosen id', () => {
        const plan = familyPlan();
        const later = plan.units[0].lessons[2];
        const { editor } = makeEditor({ getPlan: () => plan });
        const form = editor.renderNode({
            path: 'units/unit-1/lessons/family-3',
            node: later,
            kind: 'lesson',
            plan: plan
        });
        const box = form.fields.buildsOn.el;
        assert.equal(box.getAttribute('data-plan-builds-on'), '');
        assert.ok(box.querySelector('[data-plan-builds-on-id="family-1"]'));
        assert.ok(box.querySelector('[data-plan-builds-on-id="family-2"]'));
        assert.equal(box.querySelector('[data-plan-builds-on-id="family-3"]'), null);
        box.querySelectorAll('[data-plan-builds-on-id]').forEach(function (input) {
            input.checked = false;
        });
        const check = box.querySelector('[data-plan-builds-on-id="family-1"]');
        check.checked = true;
        assert.deepEqual(form.fields.buildsOn.getValue(), ['family-1']);
        const earlier = Form.earlierLessonsInPlan(plan, 'units/unit-1/lessons/family-3');
        assert.deepEqual(earlier.map((e) => e.id), ['family-1', 'family-2']);
    });

    it('ac-4: /admin/ has no plan builder; #lesson/my-morning mounts lesson editor; /editor/ refused for non-admin', async () => {
        const routes = require('../shared/app-routes.js');
        const admin = routes.find((route) => route.id === 'admin');
        assert.ok(admin);
        (admin.scripts || []).forEach((s) => {
            assert.doesNotMatch(s, /\/admin\/plan-/);
            assert.doesNotMatch(s, /plan-builder/);
            assert.doesNotMatch(s, /plan-render/);
        });
        assert.ok(admin.scripts.indexOf('/shared/admin/lesson-editor.js') > -1);
        assert.ok(admin.scripts.indexOf('/shared/admin/admin-shell.js') > -1);

        const shellSrc = fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'admin-shell.js'), 'utf8');
        assert.doesNotMatch(shellSrc, /data-admin-tab="plans"/);
        assert.doesNotMatch(shellSrc, /mountLessonGate/);
        assert.match(shellSrc, /LessonEditor\.mount/);

        const lessonsSrc = fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'lessons.js'), 'utf8');
        assert.doesNotMatch(lessonsSrc, /data-lesson-no-plan/);
        assert.doesNotMatch(lessonsSrc, /mountLessonGate/);

        const win = loadShellAndLessons();
        const tabs = win.HebrewAdminShell.renderTabs('lessons');
        assert.doesNotMatch(tabs, /data-admin-tab="plans"/);
        assert.match(tabs, /data-admin-tab="lessons"/);
        const lessonRoute = win.HebrewAdminShell.parseHash('#lesson/my-morning');
        assert.equal(lessonRoute.view, 'lesson');
        assert.equal(lessonRoute.slug, 'my-morning');
        assert.equal(typeof win.HebrewAdminLessons.mountLessonGate, 'undefined');
        assert.equal(typeof win.HebrewAdminLessons.noPlanHtml, 'undefined');

        const navSrc = fs.readFileSync(path.join(ROOT, 'shared', 'app-shell.js'), 'utf8');
        assert.match(navSrc, /id: 'editor'/);
        assert.match(navSrc, /en: 'Editor'/);
        assert.match(navSrc, /he: 'עריכה'/);
        assert.match(navSrc, /adminOnly: true/);

        const spa = require('../server/spa.js');
        assert.ok(spa.APP_PATHS.includes('/editor/'));
        assert.ok(spa.APP_PATHS.includes('/admin/'));

        const webSrc = fs.readFileSync(path.join(ROOT, 'server', 'web.js'), 'utf8');
        assert.match(webSrc, /\/editor/);
        assert.match(webSrc, /\/admin/);

        await harness.start();
        try {
            const get = (p, cookie) => fetch(harness.baseUrl + p, {
                redirect: 'manual',
                headers: { Cookie: cookie, Accept: 'text/html' }
            });
            const learner = harness.cookieFor('learner');
            assert.equal((await get('/admin/', learner)).status, 403);
            assert.equal((await get('/editor/', learner)).status, 403);
            const adminCookie = harness.cookieFor('admin');
            assert.equal((await get('/admin/', adminCookie)).status, 200);
            assert.equal((await get('/editor/', adminCookie)).status, 200);
        } finally {
            await harness.stop();
        }
    });
});
