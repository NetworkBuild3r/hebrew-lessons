/**
 * Builder media library, live preview, and validation panel.
 * Fake fetch only. No OpenAI call.
 * Provenance: INIT-004/SPEC-016
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const SCHEMA_TEXT = fs.readFileSync(path.join(root, 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json'), 'utf8');
const LESSON_PATH = 'units/unit-1/lessons/cafe-order';
const WORD_PATH = LESSON_PATH + '/blocks/introduction/words/1';
const ESTIMATE = 7.25;

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
        checked: false,
        files: null,
        rows: 0,
        type: '',
        alt: '',
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
        click() { fire(this, 'click'); }
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

function fire(node, type) {
    const ev = { target: node, type, preventDefault() {}, defaultPrevented: false };
    let current = node;
    while (current) {
        const list = current.listeners && current.listeners[type] ? current.listeners[type].slice() : [];
        list.forEach((fn) => {
            ev.currentTarget = current;
            fn(ev);
        });
        current = current.parentNode;
    }
}

function FormData() {
    this._parts = [];
}
FormData.prototype.append = function append(name, value) {
    this._parts.push({ name: name, value: value });
};

function lessonFixture() {
    return {
        id: 'cafe-order',
        title: 'Order',
        blocks: [
            {
                id: 'introduction',
                type: 'introduction',
                words: [
                    {
                        id: '1',
                        word: 'קפה',
                        meaning: 'coffee',
                        image: { mediaId: 'coffee-cup', alt: 'a cup' }
                    },
                    {
                        id: '2',
                        word: 'מים',
                        meaning: 'water'
                    }
                ]
            },
            {
                id: 'story',
                type: 'graded-story',
                paragraphs: [{ id: '1', text: 'קפה' }]
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

function validationBody() {
    return {
        error: 'validation',
        revision: 3,
        blocks: [{
            rule: 'schema',
            severity: 'block',
            path: LESSON_PATH,
            message: 'schema failed'
        }],
        warnings: [{
            rule: 'N1',
            severity: 'warn',
            path: LESSON_PATH,
            message: 'server said nine'
        }],
        informs: [{
            rule: 'recall-day',
            severity: 'inform',
            path: '',
            message: '<img src=x onerror=alert(1)>'
        }]
    };
}

/**
 * @param {{ publishValidation?: boolean }} [opts]
 */
function loadBuilder(opts) {
    const options = opts || {};
    const calls = [];
    let revision = 3;
    const lesson = lessonFixture();
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        Promise,
        URL,
        FormData,
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
            const raw = init && init.body;
            let body = null;
            if (typeof raw === 'string') body = JSON.parse(raw);
            else if (raw) body = raw;
            calls.push({
                method,
                url: String(url),
                body,
                credentials: init && init.credentials,
                contentType: init && init.headers && (init.headers['Content-Type'] || init.headers['content-type'])
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
            const target = String(url);
            if (method === 'GET' && target.indexOf('/api/admin/images/image-plan') !== -1) {
                return json(200, { count: 1, estimatedDollars: ESTIMATE });
            }
            if (method === 'GET' && String(url) === '/api/plan-schema') return json(200, JSON.parse(SCHEMA_TEXT));
            if (method === 'GET') {
                const data = target.indexOf('path=') === -1
                    ? { part: planFixture(), revision }
                    : { part: lesson, revision };
                return json(200, data);
            }
            if (method === 'POST' && target.indexOf('/api/admin/images/image-plan') !== -1) {
                return json(500, { error: 'dry-run is not a POST' });
            }
            if (method === 'POST' && target === '/api/admin/images') {
                return json(201, {
                    id: body && body.id,
                    status: 'pending',
                    altText: body && body.altText ? body.altText : ''
                });
            }
            if (method === 'POST' && target === '/api/media') {
                const idPart = body && body._parts && body._parts.find((part) => part.name === 'id');
                return json(201, {
                    id: idPart ? idPart.value : '',
                    status: 'pending',
                    altText: ''
                });
            }
            if (method === 'POST' && /\/api\/admin\/media\/[^/]+\/approval$/.test(target)) {
                return json(200, {
                    id: decodeURIComponent(target.split('/')[4]),
                    status: 'approved',
                    altText: body && body.altText
                });
            }
            if (method === 'POST' && target.endsWith('/publish') && options.publishValidation) {
                return json(400, validationBody());
            }
            if (method === 'PATCH' && body && body.field === 'meaning') {
                lesson.blocks[0].words[0].meaning = body.value;
            }
            if (method === 'PATCH' && body && body.field === 'image' && body.value) {
                const word = lesson.blocks[0].words.find((item) => body.path.endsWith('/' + item.id));
                if (word) word.image = body.value;
            }
            if (method === 'PATCH' && body && (body.field === 'mediaId' || body.field === 'alt')) {
                const word = lesson.blocks[0].words.find((item) => body.path.endsWith('/' + item.id));
                if (word && word.image) word.image[body.field] = body.value;
            }
            revision += 1;
            return json(200, {
                plan: planFixture(),
                revision,
                status: 'draft',
                validation: { ok: true, blocks: [], warnings: [], informs: [] }
            });
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'api.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'ui', 'plan-render.js'), 'utf8'), sandbox);
    for (const name of ['plan-hints.js', 'plan-widgets.js', 'plan-form.js', 'plan-builder.js']) {
        vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'admin', name), 'utf8'), sandbox);
    }
    return { sandbox, calls, panel: sandbox.document.createElement('div'), lesson };
}

function childWith(node, attr, value) {
    let found = null;
    walk(node, (child) => {
        if (found) return;
        if (child.getAttribute && child.getAttribute(attr) === value) found = child;
    });
    return found;
}

function generatePosts(calls) {
    return calls.filter((call) => call.method === 'POST' && call.url === '/api/admin/images');
}

describe('plan builder media, preview, validation (INIT-004/SPEC-016)', () => {
    it('ac-1: a graded-story block offers no picture; an introduction word saves mediaId and alt', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');

        childWith(panel.querySelector('[data-plan-block="story"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        assert.equal(panel.querySelector('[data-plan-pictures]'), null);
        assert.equal(panel.querySelector('[data-plan-attach]'), null);
        assert.equal(panel.querySelector('[data-plan-attach-media]'), null);

        childWith(panel.querySelector('[data-plan-block="introduction"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        const row = childWith(panel, 'data-plan-word', '1');
        assert.ok(row);
        assert.ok(row.querySelector('[data-plan-attach]'));
        const before = calls.length;
        row.querySelector('[data-plan-attach-media]').value = 'coffee-cup';
        row.querySelector('[data-plan-attach-alt]').value = 'a cup of coffee';
        row.querySelector('[data-plan-attach]').click();
        await Builder.settled();

        const patches = calls.slice(before).filter((call) => call.method === 'PATCH');
        assert.deepEqual(patches.map((call) => call.body.field), ['mediaId', 'alt']);
        assert.equal(patches[0].url, '/api/plans/beginner-unit-1/fields');
        assert.equal(patches[0].body.path, WORD_PATH);
        assert.equal(patches[0].body.value, 'coffee-cup');
        assert.equal(patches[1].body.path, WORD_PATH);
        assert.equal(patches[1].body.value, 'a cup of coffee');
        assert.equal(patches[0].credentials, 'same-origin');
        patches.forEach((call) => {
            assert.equal(call.body.units, undefined);
            assert.equal(call.body.plan, undefined);
        });
    });

    it('a word without a picture is given an image object, then mediaId and alt', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        await sandbox.HebrewPlanBuilder.mount(panel, 'beginner-unit-1');
        childWith(panel.querySelector('[data-plan-block="introduction"]'), 'data-plan-edit', '').click();
        await sandbox.HebrewPlanBuilder.settled();
        const row = childWith(panel, 'data-plan-word', '2');
        const before = calls.length;
        row.querySelector('[data-plan-attach-media]').value = 'water-glass';
        row.querySelector('[data-plan-attach-alt]').value = 'a glass of water';
        row.querySelector('[data-plan-attach]').click();
        await sandbox.HebrewPlanBuilder.settled();
        const patches = calls.slice(before).filter((call) => call.method === 'PATCH');
        assert.deepEqual(patches.map((call) => call.body.field), ['image', 'mediaId', 'alt']);
        assert.deepEqual(patches[0].body.value, { mediaId: 'water-glass', alt: 'a glass of water' });
        assert.equal(patches[1].body.value, 'water-glass');
        assert.equal(patches[2].body.value, 'a glass of water');
    });

    it('ac-2: generate stays disabled until the dry-run figure is shown and confirmed', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        const source = fs.readFileSync(path.join(root, 'shared', 'admin', 'plan-builder.js'), 'utf8');
        assert.equal(source.includes(String(ESTIMATE)), false);

        const generate = panel.querySelector('[data-plan-generate]');
        const confirm = panel.querySelector('[data-plan-confirm]');
        assert.equal(generate.disabled, true);
        generate.click();
        await Builder.settled();
        assert.equal(generatePosts(calls).length, 0);

        confirm.checked = true;
        fire(confirm, 'change');
        assert.equal(generate.disabled, true);
        generate.click();
        await Builder.settled();
        assert.equal(generatePosts(calls).length, 0);

        const prompt = 'a red cup on a table';
        panel.querySelector('[data-plan-gen-prompt]').value = prompt;
        panel.querySelector('[data-plan-dry-run]').click();
        await Builder.settled();

        const dry = calls.filter((call) => call.url.indexOf('/api/admin/images/image-plan') !== -1);
        assert.equal(dry.length, 1);
        assert.equal(dry[0].method, 'GET');
        assert.equal(dry[0].credentials, 'same-origin');
        assert.equal(dry[0].url.includes('confirm'), false);
        assert.equal(dry[0].url.includes(encodeURIComponent(prompt)), true);
        const estimate = panel.querySelector('[data-plan-estimate]');
        assert.equal(estimate.hidden, false);
        assert.equal(estimate.textContent, String(ESTIMATE));
        assert.equal(panel.querySelector('[data-plan-generate]').disabled, true);
        panel.querySelector('[data-plan-generate]').click();
        await Builder.settled();
        assert.equal(generatePosts(calls).length, 0);

        const box = panel.querySelector('[data-plan-confirm]');
        box.checked = true;
        fire(box, 'change');
        const armed = panel.querySelector('[data-plan-generate]');
        assert.equal(armed.disabled, false);
        panel.querySelector('[data-plan-gen-id]').value = 'red-cup';
        armed.click();
        await Builder.settled();

        const posts = generatePosts(calls);
        assert.equal(posts.length, 1);
        assert.equal(posts[0].credentials, 'same-origin');
        assert.equal(posts[0].contentType, 'application/json');
        assert.equal(posts[0].body.confirm, true);
        assert.equal(posts[0].body.estimate, ESTIMATE);
        assert.equal(posts[0].body.prompt, prompt);
        assert.equal(posts[0].body.id, 'red-cup');
        assert.equal(posts[0].body.apiKey, undefined);
        assert.equal(JSON.stringify(posts[0].body).includes('OPENAI'), false);
    });

    it('ac-3: an N1 warning is shown, and a block failure says the snapshot was not published', async () => {
        const { sandbox, calls, panel } = loadBuilder({ publishValidation: true });
        await sandbox.HebrewPlanBuilder.mount(panel, 'beginner-unit-1');
        assert.equal(panel.querySelector('[data-plan-status]').textContent, 'draft');
        const before = calls.length;
        panel.querySelector('[data-plan-publish]').click();
        await sandbox.HebrewPlanBuilder.settled();

        const publish = calls.slice(before).filter((call) => call.url.endsWith('/publish'));
        assert.equal(publish.length, 1);
        const n1 = panel.querySelector('[data-plan-finding="N1"]');
        assert.match(n1.textContent, /\bN1\b/);
        assert.match(n1.textContent, /server said nine/);
        assert.match(panel.querySelector('[data-plan-finding="schema"]').textContent, /\bschema\b/);
        assert.match(panel.querySelector('[data-plan-finding="recall-day"]').textContent, /recall-day/);
        const refusal = panel.querySelector('[data-plan-publish-refusal]');
        assert.match(refusal.textContent, /snapshot was not published/);
        assert.match(refusal.textContent, /validation/);
        assert.equal(panel.querySelector('[data-plan-status]').textContent, 'draft');
        const inform = panel.querySelector('[data-plan-finding="recall-day"]');
        assert.equal(inform.children.length, 0);
        assert.match(inform.textContent, /<img src=x onerror=alert\(1\)>/);
        const source = fs.readFileSync(path.join(root, 'shared', 'admin', 'plan-builder.js'), 'utf8');
        assert.equal(source.includes('maxNewWords'), false);
        assert.equal(source.includes('server said nine'), false);
    });

    it('ac-4: the approval button is disabled while alt text is empty', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        const button = panel.querySelector('[data-plan-approve]');
        const alt = panel.querySelector('[data-plan-approve-alt]');
        assert.equal(button.disabled, true);
        alt.value = '   ';
        fire(alt, 'input');
        assert.equal(panel.querySelector('[data-plan-approve]').disabled, true);
        panel.querySelector('[data-plan-approve]').click();
        await Builder.settled();
        assert.equal(calls.some((call) => call.url.includes('/approval')), false);

        alt.value = 'a cup of coffee';
        fire(alt, 'input');
        const armed = panel.querySelector('[data-plan-approve]');
        assert.equal(armed.disabled, false);
        panel.querySelector('[data-plan-approve-id]').value = 'coffee-cup';
        armed.click();
        await Builder.settled();

        const approval = calls.filter((call) => call.url.includes('/approval'));
        assert.equal(approval.length, 1);
        assert.equal(approval[0].method, 'POST');
        assert.equal(approval[0].url, '/api/admin/media/coffee-cup/approval');
        assert.deepEqual(approval[0].body, { altText: 'a cup of coffee' });
        assert.equal(approval[0].credentials, 'same-origin');
        assert.equal(panel.querySelector('[data-plan-media-status="approved"]').textContent, 'approved');
    });

    it('lists pending and approved images and uploads through the admin fetch', async () => {
        const { sandbox, calls, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        const source = fs.readFileSync(path.join(root, 'shared', 'admin', 'plan-builder.js'), 'utf8');
        assert.doesNotMatch(source, /<form|action\s*=|OPENAI_API_KEY|sk-/);
        assert.match(source, /\/api\/media/);

        panel.querySelector('[data-plan-upload-id]').value = 'cup-pic';
        panel.querySelector('[data-plan-upload-file]').files = [{ name: 'ignored.png' }];
        panel.querySelector('[data-plan-upload]').click();
        await Builder.settled();

        const upload = calls.filter((call) => call.url === '/api/media');
        assert.equal(upload.length, 1);
        assert.equal(upload[0].method, 'POST');
        assert.equal(upload[0].credentials, 'same-origin');
        assert.equal(upload[0].contentType, undefined);
        assert.deepEqual(upload[0].body._parts.map((part) => part.name), ['id', 'file']);
        assert.equal(upload[0].body._parts[0].value, 'cup-pic');
        assert.equal(panel.querySelector('[data-plan-media-status="pending"]').textContent, 'pending');
        const item = panel.querySelector('[data-plan-media-item="cup-pic"]');
        assert.equal(item.querySelector('img').getAttribute('src'), '/api/media/cup-pic');

        panel.querySelector('[data-plan-approve-id]').value = 'cup-pic';
        const alt = panel.querySelector('[data-plan-approve-alt]');
        alt.value = 'a cup';
        fire(alt, 'input');
        panel.querySelector('[data-plan-approve]').click();
        await Builder.settled();
        assert.equal(panel.querySelector('[data-plan-media-status="approved"]').textContent, 'approved');
        assert.equal(panel.querySelector('[data-plan-media-item="coffee-cup"]') != null, true);
    });

    it('re-renders the selected block from the renderer after a successful save', async () => {
        const { sandbox, panel } = loadBuilder();
        const Builder = sandbox.HebrewPlanBuilder;
        await Builder.mount(panel, 'beginner-unit-1');
        childWith(panel.querySelector('[data-plan-block="introduction"]'), 'data-plan-edit', '').click();
        await Builder.settled();
        const before = panel.querySelector('[data-plan-preview]').innerHTML;
        assert.match(before, /data-block-type="introduction"/);
        assert.match(before, /coffee/);
        assert.match(before, /\/api\/media\/coffee-cup/);

        const meaning = panel.querySelector('textarea[data-plan-field="meaning"]');
        meaning.value = 'a <b> tag';
        childWith(panel, 'data-plan-save-field', meaning.getAttribute('data-plan-field-index')).click();
        await Builder.settled();

        const after = panel.querySelector('[data-plan-preview]').innerHTML;
        assert.match(after, /data-block-type="introduction"/);
        assert.match(after, /a &lt;b&gt; tag/);
        assert.doesNotMatch(after, /<b>/);
        assert.equal(after.includes('>coffee</span>'), false);
        const source = fs.readFileSync(path.join(root, 'shared', 'admin', 'plan-builder.js'), 'utf8');
        assert.match(source, /HebrewPlanRender\.renderBlock/);
        assert.doesNotMatch(source, /eval\s*\(/);
        assert.doesNotMatch(source, /new Function/);
    });
});
