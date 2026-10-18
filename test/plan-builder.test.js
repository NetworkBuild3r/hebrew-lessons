/**
 * Admin plan builder — outline actions are the SPEC-004 routes.
 * Provenance: INIT-004/SPEC-015
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const SCHEMA_TEXT = fs.readFileSync(path.join(root, 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json'), 'utf8');
const LESSON_PATH = 'units/unit-1/lessons/cafe-order';

function matchSel(node, sel) {
    const found = /^([a-zA-Z]+)?(?:\[([^\]]+)\])?$/.exec(sel);
    if (!found || (!found[1] && !found[2])) return false;
    if (found[1] && node.tagName !== found[1].toUpperCase()) return false;
    if (!found[2]) return true;
    const eq = found[2].indexOf('=');
    if (eq === -1) return node.hasAttribute(found[2]);
    let value = found[2].slice(eq + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
        value = value.slice(1, -1);
    }
    return node.getAttribute(found[2].slice(0, eq)) === value;
}

function walk(node, fn) {
    (node.children || []).forEach((child) => {
        fn(child);
        walk(child, fn);
    });
}

function createElement(tag) {
    const node = {
        tagName: String(tag).toUpperCase(),
        nodeType: 1,
        children: [],
        attrs: {},
        listeners: {},
        parentNode: null,
        className: '',
        value: '',
        textContent: '',
        _html: '',
        _hidden: false,
        disabled: false,
        rows: 0,
        type: '',
        setAttribute(name, value) { this.attrs[name] = String(value); },
        getAttribute(name) {
            return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null;
        },
        hasAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name); },
        removeAttribute(name) { delete this.attrs[name]; },
        appendChild(child) {
            if (child.parentNode && child.parentNode.removeChild) child.parentNode.removeChild(child);
            child.parentNode = this;
            this.children.push(child);
            return child;
        },
        removeChild(child) {
            const index = this.children.indexOf(child);
            if (index >= 0) this.children.splice(index, 1);
            child.parentNode = null;
            return child;
        },
        addEventListener(type, fn) {
            if (!this.listeners[type]) this.listeners[type] = [];
            this.listeners[type].push(fn);
        },
        removeEventListener(type, fn) {
            const list = this.listeners[type] || [];
            this.listeners[type] = list.filter((item) => item !== fn);
        },
        querySelector(sel) {
            const all = this.querySelectorAll(sel);
            return all[0] || null;
        },
        querySelectorAll(sel) {
            const out = [];
            walk(this, (child) => {
                if (matchSel(child, sel)) out.push(child);
            });
            return out;
        },
        focus() {},
        click() {
            const ev = { target: this, preventDefault() {}, defaultPrevented: false };
            const chain = [];
            let current = this;
            while (current) {
                chain.push(current);
                current = current.parentNode;
            }
            chain.forEach((host) => {
                const list = host.listeners && host.listeners.click ? host.listeners.click.slice() : [];
                list.forEach((fn) => {
                    ev.currentTarget = host;
                    fn(ev);
                });
            });
        }
    };
    Object.defineProperty(node, 'firstChild', {
        get() { return node.children[0] || null; }
    });
    Object.defineProperty(node, 'hidden', {
        get() { return node._hidden; },
        set(value) {
            node._hidden = !!value;
            if (value) node.attrs.hidden = '';
            else delete node.attrs.hidden;
        }
    });
    Object.defineProperty(node, 'innerHTML', {
        get() { return node._html; },
        set(value) { node._html = String(value); }
    });
    return node;
}

function lessonFixture() {
    return {
        id: 'cafe-order',
        title: 'Order',
        blocks: [
            {
                id: 'introduction',
                type: 'introduction',
                words: [{
                    id: '1',
                    word: 'קפה',
                    meaning: '<script>alert(1)</script>'
                }]
            },
            {
                id: 'listen',
                type: 'listening',
                passage: 'שלום'
            }
        ]
    };
}

function planFixture() {
    return {
        id: 'beginner-unit-1',
        revision: 3,
        status: 'draft',
        title: 'Cafe',
        units: [{
            id: 'unit-1',
            title: 'Unit',
            lessons: [lessonFixture()]
        }]
    };
}

/**
 * @param {{ conflict?: boolean }} [opts]
 */
function loadBuilder(opts) {
    const options = opts || {};
    const calls = [];
    let revision = 3;
    let conflictArmed = !!options.conflict;
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        Promise,
        URL,
        confirm() { return true; },
        location: { href: 'http://127.0.0.1/admin/', hash: '#plans', pathname: '/admin/' },
        document: {
            readyState: 'complete',
            getElementById() { return null; },
            querySelectorAll() { return []; },
            addEventListener() {},
            createElement
        },
        fetch(url, init) {
            const method = String((init && init.method) || 'GET').toUpperCase();
            let body = null;
            if (init && init.body) body = JSON.parse(init.body);
            calls.push({
                method,
                url: String(url),
                body,
                credentials: init && init.credentials,
                contentType: init && init.headers && init.headers['Content-Type']
            });
            const json = (status, data) => Promise.resolve({
                ok: status >= 200 && status < 300,
                status,
                headers: {
                    get(name) {
                        return String(name).toLowerCase() === 'content-type' ? 'application/json' : null;
                    }
                },
                json() { return Promise.resolve(data); },
                text() { return Promise.resolve(JSON.stringify(data)); }
            });
            if (method === 'GET' && String(url) === '/api/plan-schema') return json(200, JSON.parse(SCHEMA_TEXT));
            if (method === 'GET') {
                const data = String(url).indexOf('path=') === -1
                    ? { part: planFixture(), revision }
                    : { part: lessonFixture(), revision };
                return json(200, data);
            }
            if (options.failBlocks && method === 'POST' && String(url).endsWith('/blocks')) {
                return json(400, {
                    error: 'validation',
                    revision,
                    blocks: [{ rule: 'schema', severity: 'block', path: '/units/0/lessons/0/blocks/2/title', message: 'too short' }],
                    warnings: [],
                    informs: []
                });
            }
            if (conflictArmed && method === 'PATCH') {
                conflictArmed = false;
                return json(409, { error: 'revision_conflict', revision: 9 });
            }
            revision += 1;
            return json(200, { plan: planFixture(), revision, status: 'draft' });
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'api.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'ui', 'plan-render.js'), 'utf8'), sandbox);
    for (const name of [
        'activity-map.js', 'plan-ui.js', 'plan-hints.js', 'plan-widgets.js', 'plan-form.js', 'plan-builder.js'
    ]) {
        vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'admin', name), 'utf8'), sandbox);
    }
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'admin', 'admin-shell.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'admin', 'editor-shell.js'), 'utf8'), sandbox);
    return { sandbox, calls, panel: sandbox.document.createElement('div') };
}

function childWith(node, attr, value) {
    let found = null;
    walk(node, (child) => {
        if (found) return;
        if (child.getAttribute && child.getAttribute(attr) === value) found = child;
    });
    return found;
}

/** Block rows, the activity menu, and the media card are on the lesson screen. Mount opens the plan screen. */
async function openLessonScreen(Builder, panel) {
    const tab = panel.querySelector('[data-plan-lesson-tab="0"]');
    if (!tab) throw new Error('lesson tab 0 is missing');
    tab.click();
    await Builder.settled();
}

describe('plan builder outline (INIT-004/SPEC-015)', () => {
    it('keeps the legacy save hook and does not post to upsert_lesson', () => {
        const editor = fs.readFileSync(path.join(root, 'shared', 'admin', 'lesson-editor.js'), 'utf8');
        const builder = fs.readFileSync(path.join(root, 'shared', 'admin', 'plan-builder.js'), 'utf8');
        const shell = fs.readFileSync(path.join(root, 'shared', 'admin', 'admin-shell.js'), 'utf8');
        assert.match(editor, /data-lesson-save/);
        assert.doesNotMatch(builder, /data-lesson-save/);
        assert.doesNotMatch(builder, /upsert_lesson/);
        assert.doesNotMatch(builder, /\/api\/admin\/lessons/);
        assert.doesNotMatch(builder, /replace_part|replacePart/);
        assert.doesNotMatch(builder, /\.splice\s*\(/);
        // A lesson is cloned with JSON.parse before it is sent. That is not the legacy save hook.
        assert.doesNotMatch(builder, /data-revision|dataset\.revision/);
        assert.match(shell, /data-lesson-save|LessonEditor/);
        assert.doesNotMatch(shell, /HebrewPlanBuilder|PlanBuilder/);
    });

    it('sends the move index the route expects and does not renumber block ids', async () => {
        // SPEC-011/012: the hand-written fieldsForBlock and newBlock lists are gone; a new block
        // comes from the schema's required list, and the fields come from the editor form.
        const { sandbox, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        assert.equal(Builder.moveDestination(0, 'down'), 1);
        assert.equal(Builder.moveDestination(2, 'up'), 1);
        assert.equal(Builder.moveDestination(0, 'up'), -1);
        await Builder.mount(panel, 'beginner-unit-1');
        const created = Builder.newBlock('listening', 'extra');
        assert.equal(created.id, 'extra');
        assert.equal(created.type, 'listening');
        assert.equal(Object.prototype.hasOwnProperty.call(created, 'blocks'), false);
        await openLessonScreen(Builder, panel);
        childWith(panel.querySelector('[data-plan-block="listen"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        const form = Builder.editor().forms()[LESSON_PATH + '/blocks/listen'];
        assert.equal(form.path, LESSON_PATH + '/blocks/listen');
        assert.equal(form.fields.passage.prop, 'passage');
        assert.equal(form.fields.passage.getValue(), lessonFixture().blocks[1].passage);
    });

    it('ac-1: insert, move, remove, field save, and publish call the plan routes', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        assert.equal(calls[0].method, 'GET');
        assert.equal(calls[0].url, '/api/plans/beginner-unit-1/parts');
        assert.equal(calls[1].method, 'GET');
        assert.equal(
            calls[1].url,
            '/api/plans/beginner-unit-1/parts?path=' + encodeURIComponent(LESSON_PATH)
        );
        await openLessonScreen(Builder, panel);

        const listen = panel.querySelector('[data-plan-block="listen"]');
        childWith(listen, 'data-plan-edit', '').click();
        await Builder.settled();
        const passage = panel.querySelector('textarea[data-plan-field="passage"]');
        passage.value = 'שלום!';
        childWith(panel, 'data-plan-save-field', '0').click();
        await Builder.settled();

        panel.querySelector('[data-plan-move="down"]').click();
        await Builder.settled();
        childWith(panel.querySelector('[data-plan-block="listen"]'), 'data-plan-remove', '').click();
        await Builder.settled();

        // The teacher no longer types an id or an index. The page appends and chooses the id.
        panel.querySelector('[data-plan-new-type]').value = 'listening';
        panel.querySelector('[data-plan-insert]').click();
        await Builder.settled();

        panel.querySelector('[data-plan-revision]').textContent = '999';
        panel.querySelector('[data-plan-publish]').click();
        await Builder.settled();

        const writes = calls.filter((call) => call.method !== 'GET');
        assert.deepEqual(writes.map((call) => call.method + ' ' + call.url), [
            'PATCH /api/plans/beginner-unit-1/fields',
            'POST /api/plans/beginner-unit-1/blocks/move',
            'DELETE /api/plans/beginner-unit-1/parts',
            'POST /api/plans/beginner-unit-1/blocks',
            'POST /api/plans/beginner-unit-1/publish'
        ]);

        const field = writes[0].body;
        assert.deepEqual(Object.keys(field).sort(), ['expectedRevision', 'field', 'path', 'value']);
        assert.equal(field.path, LESSON_PATH + '/blocks/listen');
        assert.equal(field.field, 'passage');
        assert.equal(field.value, 'שלום!');
        assert.equal(field.expectedRevision, 3);

        const move = writes[1].body;
        assert.deepEqual(Object.keys(move).sort(), ['expectedRevision', 'index', 'path']);
        assert.equal(move.path, LESSON_PATH + '/blocks/introduction');
        assert.equal(move.index, 1);
        assert.equal(move.expectedRevision, 4);
        assert.equal(move.block, undefined);

        const removed = writes[2].body;
        assert.deepEqual(Object.keys(removed).sort(), ['expectedRevision', 'path']);
        assert.equal(removed.path, LESSON_PATH + '/blocks/listen');
        assert.equal(removed.expectedRevision, 5);

        const inserted = writes[3].body;
        assert.deepEqual(Object.keys(inserted).sort(), ['block', 'expectedRevision', 'index', 'path']);
        assert.equal(inserted.path, LESSON_PATH);
        assert.equal(inserted.index, 2);
        assert.equal(inserted.block.id, 'listening');
        assert.equal(inserted.block.type, 'listening');
        assert.equal(JSON.stringify(inserted).includes('introduction'), false);
        assert.equal(inserted.expectedRevision, 6);

        const published = writes[4].body;
        assert.deepEqual(published, { expectedRevision: 7 });
        writes.forEach((call) => {
            assert.equal(call.credentials, 'same-origin');
            assert.equal(call.contentType, 'application/json');
            assert.equal(call.body.units, undefined);
            assert.equal(call.body.plan, undefined);
        });
    });

    it('ac-2: a 409 keeps the textarea and names revision_conflict plus the stored revision', async () => {
        const { sandbox, calls, panel } = loadBuilder({ conflict: true });
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        await openLessonScreen(Builder, panel);
        const beforeGets = calls.filter((call) => call.method === 'GET').length;
        childWith(panel.querySelector('[data-plan-block="listen"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        const passage = panel.querySelector('textarea[data-plan-field="passage"]');
        passage.value = 'ADMIN DRAFT';
        sandbox.confirm = function () { return false; };
        assert.equal(Builder.canLeave(), false);
        assert.equal(passage.value, 'ADMIN DRAFT');
        childWith(panel, 'data-plan-save-field', '0').click();
        await Builder.settled();

        const live = panel.querySelector('textarea[data-plan-field="passage"]');
        assert.equal(live.value, 'ADMIN DRAFT');
        const conflict = panel.querySelector('[data-plan-conflict]');
        assert.match(conflict.textContent, /revision_conflict/);
        assert.match(conflict.textContent, /\b9\b/);
        const patch = calls.filter((call) => call.method === 'PATCH');
        assert.equal(patch.length, 1);
        assert.equal(patch[0].body.expectedRevision, 3);
        assert.equal(patch[0].body.value, 'ADMIN DRAFT');
        assert.equal(calls.filter((call) => call.method === 'GET').length, beforeGets);

        panel.querySelector('[data-plan-revision]').textContent = '999';
        panel.querySelector('[data-plan-publish]').click();
        await Builder.settled();
        const publish = calls.filter((call) => call.url.endsWith('/publish'));
        assert.equal(publish.length, 1);
        assert.equal(publish[0].body.expectedRevision, 3);
    });

    it('escapes plan text in the preview and refuses a hostile plan id', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        const hostile = sandbox.document.createElement('div');
        await Builder.mount(hostile, '../not-a-plan');
        assert.equal(calls.length, 0);
        await Builder.mount(panel, 'beginner-unit-1');
        await openLessonScreen(Builder, panel);
        childWith(panel.querySelector('[data-plan-block="introduction"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        const preview = panel.querySelector('[data-plan-preview]');
        assert.match(preview.innerHTML, /&lt;script/);
        assert.doesNotMatch(preview.innerHTML, /<script/i);
        const meaning = panel.querySelector('textarea[data-plan-field="meaning"]');
        assert.equal(meaning.value, '<script>alert(1)</script>');
        walk(panel, (node) => {
            assert.notEqual(node.tagName, 'SCRIPT');
        });
    });

    it('ac-3: a non-admin session does not get the builder; editor and admin are admin-only', () => {
        const routes = require('../shared/app-routes.js');
        const admin = routes.find((route) => route.id === 'admin');
        const editor = routes.find((route) => route.id === 'editor');
        assert.equal(admin.adminOnly, true);
        assert.equal(editor.adminOnly, true);
        assert.equal(admin.scripts.includes('/shared/admin/plan-builder.js'), false);
        assert.ok(editor.scripts.includes('/shared/admin/plan-builder.js'));
        assert.ok(editor.scripts.includes('/shared/ui/plan-render.js'));
        routes.forEach((route) => {
            if (route.id === 'editor') return;
            assert.equal(route.scripts.includes('/shared/admin/plan-builder.js'), false);
            if (route.id === 'admin') return;
            assert.equal(route.adminOnly, undefined);
        });
        const shellHtml = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8');
        assert.doesNotMatch(shellHtml, /plan-builder|data-plan-outline|data-plan-publish/);
        const spa = require('../server/spa');
        const user = { id: 2, username: 'learner', display_name: 'Learner', role: 'user', must_change_password: false };
        const adminUser = { id: 1, username: 'admin', display_name: 'Admin', role: 'admin', must_change_password: false };
        assert.doesNotMatch(spa.renderShell(user), /plan-builder|data-plan-outline/);
        assert.doesNotMatch(spa.renderShell(adminUser), /data-plan-outline|data-plan-publish/);
        const web = fs.readFileSync(path.join(root, 'server', 'web.js'), 'utf8');
        assert.match(web, /underAdmin && auth\.user\.role !== 'admin'/);
        assert.match(web, /\/editor/);

        const mounted = [];
        const main = {
            innerHTML: '',
            focus() {},
            querySelector() { return null; }
        };
        const sandbox = {
            console,
            setTimeout,
            clearTimeout,
            Promise,
            URL,
            location: { pathname: '/admin/', hash: '', origin: 'http://app.test', href: 'http://app.test/admin/' },
            history: { state: null, pushState() {}, replaceState() {} },
            document: {
                title: '',
                addEventListener() {},
                getElementById() { return { textContent: '' }; },
                querySelectorAll() { return []; }
            },
            addEventListener() {},
            HebrewLayout: {
                message(he, en) { return he + ' ' + en; }
            },
            HebrewAdminShell: {
                mount() { mounted.push('shell'); }
            }
        };
        sandbox.window = sandbox;
        vm.createContext(sandbox);
        vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'router.js'), 'utf8'), sandbox);
        sandbox.HebrewRouter.start({
            routes: [{
                id: 'admin',
                path: '/admin/',
                title: 'Admin',
                announce: 'ניהול',
                bodyClass: 'admin-page',
                rootId: 'admin-app',
                adminOnly: true,
                scripts: ['/shared/admin/plan-builder.js'],
                view: 'HebrewAdminShell',
                heading: { he: 'ניהול', en: 'Admin' }
            }],
            main,
            user: { role: 'user' }
        });
        assert.match(main.innerHTML, /No access/);
        assert.deepEqual(mounted, []);
    });

    it('admin keeps #lesson/; plans open on the editor shell, not admin tabs', () => {
        const { sandbox } = loadBuilder();
        const Admin = sandbox.HebrewAdminShell;
        const lesson = Admin.parseHash('#lesson/my-morning');
        assert.equal(lesson.view, 'lesson');
        assert.equal(lesson.slug, 'my-morning');
        assert.equal(Admin.parseHash('#lessons').view, 'lessons');
        assert.equal(Admin.parseHash('#users').view, 'users');
        assert.equal(Admin.parseHash('#plans').view, 'users', 'plans hash is not an admin view');
        const tabs = Admin.renderTabs('lessons');
        assert.doesNotMatch(tabs, /data-admin-tab="plans"/);
        assert.match(tabs, /data-admin-tab="lessons"/);

        const Editor = sandbox.HebrewEditorShell;
        assert.ok(Editor);
        assert.equal(Editor.parseHash('#plans').view, 'plans');
        const plan = Editor.parseHash('#plan/beginner-unit-1');
        assert.equal(plan.view, 'plan');
        assert.equal(plan.planId, 'beginner-unit-1');
        assert.equal(Editor.parseHash('#plan/../x').view, 'plans');
    });
});

/** Descendants with an attribute (and value). The ad-hoc fake here has no descendant selectors. */
function all(node, attr, value) {
    const out = [];
    walk(node, (child) => {
        if (child.hasAttribute && child.hasAttribute(attr) && (value === undefined || child.getAttribute(attr) === value)) out.push(child);
    });
    return out;
}

/** Fire a bubbling event through the fake's listeners. */
function fireEvent(node, type) {
    const ev = { target: node, type, preventDefault() {} };
    for (let cur = node; cur; cur = cur.parentNode) {
        (cur.listeners[type] || []).slice().forEach((fn) => { ev.currentTarget = cur; fn(ev); });
    }
}

function inSection(panel, name, attr, value) {
    const section = all(panel, 'data-plan-section', name)[0];
    return section ? all(section, attr, value)[0] || null : null;
}
describe('plan builder with the schema-driven editor (INIT-008/SPEC-011, SPEC-012)', () => {
    it('fetches the schema once per mount, after the plan and the lesson', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        await sandbox.HebrewPlanBuilder.mount(panel, 'beginner-unit-1');
        assert.deepEqual(calls.map((c) => c.url), [
            '/api/plans/beginner-unit-1/parts',
            '/api/plans/beginner-unit-1/parts?path=' + encodeURIComponent(LESSON_PATH),
            '/api/plan-schema'
        ]);
        await openLessonScreen(sandbox.HebrewPlanBuilder, panel);
        childWith(panel.querySelector('[data-plan-block="listen"]'), 'data-plan-edit', '').click();
        await sandbox.HebrewPlanBuilder.settled();
        assert.equal(calls.filter((c) => c.url === '/api/plan-schema').length, 1);
    });

    it('a save sends the revision the server last returned, one PATCH, and keeps other unsaved text', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        await openLessonScreen(Builder, panel);
        childWith(panel.querySelector('[data-plan-block="introduction"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        const meaning = panel.querySelector('textarea[data-plan-field="meaning"]');
        meaning.value = 'edited meaning';
        // unsaved text in another field of the same page
        const title = inSection(panel, 'lesson', 'data-plan-field', 'title');
        title.value = 'Typed, not saved';
        childWith(panel, 'data-plan-save-field', meaning.getAttribute('data-plan-field-index')).click();
        await Builder.settled();
        const patches = calls.filter((c) => c.method === 'PATCH');
        assert.equal(patches.length, 1);
        assert.deepEqual(patches[0].body, {
            expectedRevision: 3,
            path: LESSON_PATH + '/blocks/introduction/words/1',
            field: 'meaning',
            value: 'edited meaning'
        });
        assert.equal(panel.querySelector('[data-plan-revision]').textContent, '4');
        const kept = inSection(panel, 'lesson', 'data-plan-field', 'title');
        assert.equal(kept.value, 'Typed, not saved', 'unsaved text in another field survives the repaint');
    });

    it('the activity menu lists research names and insert sends the mapped block type', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        await openLessonScreen(Builder, panel);
        const select = panel.querySelector('select[data-plan-new-type]');
        assert.ok(panel.querySelector('[data-plan-activity-menu]'));
        const names = Array.from(select.children).map((o) => o.getAttribute('data-plan-activity-name'));
        assert.ok(names.indexOf('Words') !== -1);
        assert.ok(names.indexOf('Story') !== -1);
        assert.equal(names.indexOf('Match'), -1, 'Match stays out of the menu until it has a block');
        assert.ok(names.indexOf('Type it') !== -1);
        assert.equal(select.children.length, Builder.activityNames().length);
        select.value = 'fluency-pass';
        fireEvent(select, 'change');
        panel.querySelector('[data-plan-insert]').click();
        await Builder.settled();
        const post = calls.filter((c) => c.method === 'POST' && c.url.endsWith('/blocks'))[0];
        assert.equal(post.body.block.type, 'fluency-pass');
        assert.equal(post.body.block.id, 'fluency');
        assert.equal(post.body.block.source, 'vocab');
        assert.ok(Number.isInteger(post.body.block.seconds));
    });

    it('a refused insert lists the server rule codes in the form and does not repaint it away', async () => {
        const { sandbox, panel } = loadBuilder({ failBlocks: true });
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        await openLessonScreen(Builder, panel);
        const select = panel.querySelector('select[data-plan-new-type]');
        select.value = 'fluency-pass';
        fireEvent(select, 'change');
        panel.querySelector('[data-plan-insert]').click();
        await Builder.settled();
        const issue = all(panel.querySelector('[data-plan-new-issues]'), 'data-plan-issue', 'schema')[0];
        assert.match(issue.textContent, /too short/);
        assert.ok(panel.querySelector('[data-plan-finding="schema"]'), 'the validation panel lists it too');
    });

    it('unmount removes the panel listeners and the editor listeners', async () => {
        const { sandbox, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        assert.ok(panel.listeners.click.length >= 2);
        Builder.unmount();
        assert.equal(panel.listeners.click.length, 0);
        assert.equal(panel.listeners.change.length, 0);
        assert.equal(panel.listeners.input.length, 0);
    });

    it('opens the plan, unit, and lesson forms on demand and shows recipe and visibility as selects', async () => {
        const { sandbox, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        assert.equal(inSection(panel, 'plan', 'data-plan-field'), null, 'closed until opened');
        inSection(panel, 'plan', 'data-plan-disclose').click();
        await Builder.settled();
        assert.ok(inSection(panel, 'plan', 'data-plan-field', 'title'));
        await openLessonScreen(Builder, panel);
        assert.equal(inSection(panel, 'lesson', 'data-plan-field', 'recipe').getAttribute('data-plan-widget'), 'select');
        assert.equal(inSection(panel, 'lesson', 'data-plan-field', 'visibility').getAttribute('data-plan-widget'), 'select');
    });
});