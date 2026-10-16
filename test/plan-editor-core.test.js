/**
 * The schema-driven editor core (INIT-008/SPEC-011): saves are single operations through the
 * transport, the JSON fallback, conflicts and validation shown at the field, drafts kept across
 * a repaint, escaped rendering. Fake DOM and a recording transport; a real browser run is
 * test/chrome-plan-editor-smoke.js.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const Form = require('../shared/admin/plan-form.js');
const dom = require('./helpers/fake-dom');
const { LESSON, SCHEMA, samplePlan, makeEditor } = require('./helpers/plan-editor-fixtures');

const tick = () => new Promise((resolve) => setImmediate(resolve));

function find(root, attr, value) {
    return dom.collect(root, (n) => n.hasAttribute && n.hasAttribute(attr)
        && (value === undefined || n.getAttribute(attr) === value));
}

/** Render one block of the sample plan in a fresh editor, with an optional canned transport answer. */
function open(type, answer) {
    const plan = samplePlan();
    const block = plan.units[0].lessons[0].blocks.find((b) => b.type === type);
    const env = makeEditor();
    if (answer) {
        env.editor.ctx.writeList = env.editor.ctx.writeList; // keep the real one
    }
    const path = LESSON + '/blocks/' + block.id;
    const form = env.editor.renderNode({ path, node: block, kind: 'block' });
    env.host.appendChild(form.el);
    return Object.assign({ plan, block, form, path }, env);
}

function openWith(type, transport) {
    const plan = samplePlan();
    const block = plan.units[0].lessons[0].blocks.find((b) => b.type === type);
    const doc = dom.makeDocument();
    const host = doc.createElement('div');
    const editor = Form.createEditor({ doc, host, schema: SCHEMA, transport, getPlan: () => plan });
    const path = LESSON + '/blocks/' + block.id;
    const form = editor.renderNode({ path, node: block, kind: 'block' });
    host.appendChild(form.el);
    return { plan, block, form, path, editor, host, doc };
}

/** The Add button of a list widget: the one in its own bar, not one inside an item. */
function addButton(handle) {
    return handle.el.children[1].children[0];
}

function saveButtonFor(form, prop) {
    return find(form.fields[prop].row, 'data-plan-save')[0];
}

describe('saving is one operation (INIT-008/SPEC-011 ac-2)', () => {
    it('a string field sends exactly one set_field with the id path, field, and the whole value', async () => {
        const env = open('listening');
        const area = env.form.fields.passage.el;
        assert.equal(area.tagName, 'TEXTAREA');
        area.value = 'new text';
        saveButtonFor(env.form, 'passage').click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.deepEqual(env.calls[0], { op: 'set', path: env.path, field: 'passage', value: 'new text' });
    });

    it('an unchanged field sends nothing and says so', async () => {
        const env = open('listening');
        saveButtonFor(env.form, 'passage').click();
        await tick();
        assert.equal(env.calls.length, 0);
        assert.match(env.form.fields.passage.issues.textContent, /unchanged/);
    });

    it('a number field sends a number; a bad number is sent as text for the server to refuse', async () => {
        const env = open('fluency-pass');
        env.form.fields.seconds.el.value = '45';
        saveButtonFor(env.form, 'seconds').click();
        await tick();
        assert.equal(env.calls[0].value, 45);
        assert.equal(typeof env.calls[0].value, 'number');
        env.form.fields.seconds.el.value = '4x';
        saveButtonFor(env.form, 'seconds').click();
        await tick();
        assert.equal(env.calls[1].value, '4x');
    });

    it('a select sends its value', async () => {
        const env = open('conjugation-table');
        env.form.fields.tense.el.value = 'past';
        saveButtonFor(env.form, 'tense').click();
        await tick();
        assert.deepEqual(env.calls[0], { op: 'set', path: env.path, field: 'tense', value: 'past' });
    });

    it('a string list: add, remove, and move are local; Save sends one set_field with the whole array', async () => {
        const env = open('spaced-review-checkpoint');
        const handle = env.form.fields.fromLessons;
        const add = find(handle.el, 'data-plan-add')[0];
        add.click();
        const inputs = () => dom.collect(handle.el, (n) => n.tagName === 'INPUT' && n.getAttribute('type') !== 'x' && n.type === 'text');
        assert.equal(inputs().length, 2);
        inputs()[1].value = 'second';
        // move the new row up
        find(handle.el, 'data-plan-item-op', 'up')[1].click();
        assert.deepEqual(handle.getValue(), ['second', 'earlier']);
        // remove the first row
        find(handle.el, 'data-plan-item-op', 'remove')[0].click();
        assert.deepEqual(handle.getValue(), ['earlier']);
        find(handle.el, 'data-plan-add')[0].click();
        inputs()[1].value = 'third';
        assert.equal(env.calls.length, 0, 'nothing is sent until Save');
        saveButtonFor(env.form, 'fromLessons').click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.deepEqual(env.calls[0], { op: 'set', path: env.path, field: 'fromLessons', value: ['earlier', 'third'] });
    });

    it('a blank row in a list is dropped, not sent', async () => {
        const env = open('spaced-review-checkpoint');
        find(env.form.fields.fromLessons.el, 'data-plan-add')[0].click();
        saveButtonFor(env.form, 'fromLessons').click();
        await tick();
        assert.equal(env.calls.length, 0, 'a blank row leaves the list as it was');
    });

    it('an object list adds an item with a new unique id and saves the whole array', async () => {
        const env = open('explainer');
        const handle = env.form.fields.sections;
        addButton(handle).click();
        const value = handle.getValue();
        assert.equal(value.length, 2);
        assert.equal(value[0].id, '1');
        assert.equal(value[1].id, '2');
        assert.deepEqual(Object.keys(value[1]).sort(), ['heading', 'id']);
        value.forEach((item) => assert.equal(typeof item.heading, 'string'));
        // fill the new heading and save
        const headings = find(handle.el, 'data-plan-sub', 'heading');
        headings[1].value = 'Second';
        saveButtonFor(env.form, 'sections').click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.equal(env.calls[0].field, 'sections');
        assert.equal(env.calls[0].value[1].heading, 'Second');
        assert.equal(env.calls[0].value[0].examples[0].he, env.block.sections[0].examples[0].he, 'untouched data is kept');
    });

    it('a nested object (verse reference) saves the whole object and keeps what it did not edit', async () => {
        const env = open('verse-reading');
        const chapter = find(env.form.fields.ref.el, 'data-plan-sub', 'chapter')[0];
        chapter.value = '2';
        saveButtonFor(env.form, 'ref').click();
        await tick();
        assert.deepEqual(env.calls[0].value, Object.assign({}, env.block.ref, { chapter: 2 }));
    });

    it('a bucket item is a form at its own id path, and its save names that path', async () => {
        const env = open('introduction');
        const itemPath = env.path + '/words/1';
        const itemForm = env.editor.forms()[itemPath];
        itemForm.fields.meaning.el.value = 'home';
        saveButtonFor(itemForm, 'meaning').click();
        await tick();
        assert.deepEqual(env.calls[0], { op: 'set', path: itemPath, field: 'meaning', value: 'home' });
    });

    it('bucket add, move, and remove send the whole list of the block, with a new unique item id', async () => {
        const env = open('introduction');
        const handle = env.form.fields.words;
        addButton(handle).click();
        // the draft shows only the required fields of a word
        const draftFields = find(handle.el, 'data-plan-sub').map((n) => n.getAttribute('data-plan-sub'));
        assert.deepEqual(draftFields.filter((p) => ['id', 'word', 'meaning'].indexOf(p) !== -1).sort(), ['id', 'meaning', 'word']);
        assert.equal(draftFields.indexOf('sentence'), -1);
        find(handle.el, 'data-plan-sub', 'word')[0].value = 'w';
        find(handle.el, 'data-plan-sub', 'meaning')[0].value = 'm';
        find(handle.el, 'data-plan-add-confirm')[0].click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.equal(env.calls[0].path, env.path);
        assert.equal(env.calls[0].field, 'words');
        assert.deepEqual(env.calls[0].value.map((w) => w.id), ['1', '2']);
        assert.equal(env.calls[0].value[1].word, 'w');
        // remove asks first, and sends the list without the item
        find(handle.el, 'data-plan-item-op', 'remove')[0].click();
        await tick();
        assert.equal(env.calls[1].value.length, 0);
    });

    it('an emptied optional field is removed with the node minus that one property, not set to ""', async () => {
        const env = open('introduction');
        const itemPath = env.path + '/words/1';
        const itemForm = env.editor.forms()[itemPath];
        assert.equal(itemForm.fields.translit.el.value, 'bayit');
        itemForm.fields.translit.el.value = '';
        saveButtonFor(itemForm, 'translit').click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.equal(env.calls[0].op, 'put');
        assert.equal(env.calls[0].path, itemPath);
        assert.equal(Object.prototype.hasOwnProperty.call(env.calls[0].value, 'translit'), false);
        assert.equal(env.calls[0].value.word, env.block.words[0].word);
    });

    it('a required field emptied is still sent (the server says what is wrong)', async () => {
        const env = open('listening');
        env.form.fields.passage.el.value = '';
        saveButtonFor(env.form, 'passage').click();
        await tick();
        assert.deepEqual(env.calls[0], { op: 'set', path: env.path, field: 'passage', value: '' });
    });
});

describe('the generic JSON fallback (INIT-008/SPEC-011 ac-3)', () => {
    it('invalid JSON sends nothing and shows an inline error; valid JSON sends one operation', async () => {
        // Story glosses have their own list now, so name the JSON widget for this one field.
        const Hints = require('../shared/admin/plan-hints.js');
        const hint = Hints.fields['gradedStoryBlock.glosses'];
        const named = hint.widget;
        hint.widget = 'json';
        try {
            const env = open('graded-story');
            const handle = env.form.fields.glosses;
            assert.equal(handle.widgetName, 'json');
            handle.el.value = '{ not json';
            saveButtonFor(env.form, 'glosses').click();
            await tick();
            assert.equal(env.calls.length, 0);
            assert.match(handle.issues.textContent, /Invalid JSON/);
            handle.el.value = '{"a": "b"}';
            saveButtonFor(env.form, 'glosses').click();
            await tick();
            assert.deepEqual(env.calls, [{ op: 'set', path: env.path, field: 'glosses', value: { a: 'b' } }]);
        } finally {
            hint.widget = named;
        }
    });

    it('story meanings save as one set_field with a plain word-to-meaning map', async () => {
        const env = open('graded-story');
        const handle = env.form.fields.glosses;
        assert.equal(handle.widgetName, 'gloss-map');
        const inputs = dom.collect(handle.el, (n) => n.tagName === 'INPUT');
        assert.equal(inputs.length, 2, 'one saved meaning is one row');
        inputs[1].value = 'a house';
        find(handle.el, 'data-plan-add')[0].click();
        const rows = dom.collect(handle.el, (n) => n.tagName === 'INPUT');
        rows[2].value = 'גָּדוֹל';
        rows[3].value = 'big';
        saveButtonFor(env.form, 'glosses').click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.equal(env.calls[0].field, 'glosses');
        assert.deepEqual(env.calls[0].value, { 'בַּיִת': 'a house', 'גָּדוֹל': 'big' });
    });

    it('any node can be edited as raw JSON: invalid sends nothing, valid sends one put of the node', async () => {
        const env = open('listening');
        const toggle = find(env.form.el, 'data-plan-json-toggle').pop();
        toggle.click();
        const area = find(env.form.el, 'data-plan-json', 'node').pop();
        assert.equal(JSON.parse(area.value).passage, env.block.passage);
        area.value = '[1,';
        find(env.form.el, 'data-plan-save-json').pop().click();
        await tick();
        assert.equal(env.calls.length, 0);
        assert.equal(find(env.form.el, 'data-plan-json-error').pop().hidden, false);
        area.value = JSON.stringify(Object.assign({}, env.block, { passage: 'raw' }));
        find(env.form.el, 'data-plan-save-json').pop().click();
        await tick();
        assert.equal(env.calls.length, 1);
        assert.equal(env.calls[0].op, 'put');
        assert.equal(env.calls[0].path, env.path);
        assert.equal(env.calls[0].value.passage, 'raw');
    });

    it('a node the generator does not know (a block type the schema lacks) opens as raw JSON', () => {
        const env = makeEditor();
        const form = env.editor.renderNode({
            path: LESSON + '/blocks/odd', kind: 'block', node: { id: 'odd', type: 'from-the-future', note: 'x' }
        });
        assert.equal(find(form.el, 'data-plan-unknown').length, 1);
        const area = find(form.el, 'data-plan-json', 'node')[0];
        assert.ok(area);
        assert.equal(JSON.parse(area.value).type, 'from-the-future');
    });

    it('a property of a mutated schema that no hint covers still gets a widget (the JSON fallback)', () => {
        const mutated = JSON.parse(JSON.stringify(SCHEMA));
        mutated.definitions.listeningBlock.properties.brandNew = { anyOf: [{ type: 'string' }, { type: 'integer' }] };
        const doc = dom.makeDocument();
        const editor = Form.createEditor({ doc, schema: mutated, transport: {} });
        const form = editor.renderNode({
            path: LESSON + '/blocks/l', kind: 'block', node: { id: 'l', type: 'listening', passage: 'p', brandNew: 5 }
        });
        assert.equal(form.fields.brandNew.widgetName, 'json');
        assert.equal(form.fields.brandNew.el.value, '5');
    });
});

describe('conflicts and validation show at the field and keep the text (INIT-008/SPEC-011 ac-4)', () => {
    it('a 409 keeps the typed text and shows revision_conflict and the stored revision at the field', async () => {
        const env = openWith('listening', {
            setField: () => Promise.resolve({ ok: false, status: 409, data: { error: 'revision_conflict', revision: 9 } }),
            putPart: () => Promise.resolve({ ok: false, status: 409, data: { error: 'revision_conflict', revision: 9 } })
        });
        env.form.fields.passage.el.value = 'ADMIN DRAFT';
        saveButtonFor(env.form, 'passage').click();
        await tick();
        assert.equal(env.form.fields.passage.el.value, 'ADMIN DRAFT');
        const text = env.form.fields.passage.issues.textContent;
        assert.match(text, /revision_conflict/);
        assert.match(text, /\b9\b/);
        assert.equal(env.form.fields.passage.isDirty(), true, 'still unsaved');
    });

    it('a validation error shows each rule code at its field, and keeps the text', async () => {
        const body = {
            error: 'validation', revision: 4,
            blocks: [{ rule: 'schema', severity: 'block', path: '/units/0/lessons/0/blocks/2/passage', message: 'too short' }],
            warnings: [{ rule: 'pointing-form', severity: 'warn', path: '/units/0/lessons/0/blocks/2/passage', message: 'a stray point' }],
            informs: []
        };
        const env = openWith('listening', {
            setField: () => Promise.resolve({ ok: false, status: 400, data: body }),
            putPart: () => Promise.resolve({ ok: false, status: 400, data: body })
        });
        assert.equal(env.plan.units[0].lessons[0].blocks[2].type, 'listening');
        env.form.fields.passage.el.value = 'x';
        saveButtonFor(env.form, 'passage').click();
        await tick();
        const issues = find(env.form.fields.passage.issues, 'data-plan-issue');
        assert.deepEqual(issues.map((n) => n.getAttribute('data-plan-issue')), ['schema', 'pointing-form']);
        assert.match(issues[0].textContent, /too short/);
        assert.equal(env.form.fields.passage.el.getAttribute('aria-invalid'), 'true');
        assert.equal(env.form.fields.passage.el.value, 'x');
    });

    it('issues on a nested property land on its top-level field with the remainder of the path', () => {
        const env = open('explainer');
        const placed = env.editor.setIssues([{
            rule: 'schema', severity: 'block', message: 'bad', path: '/units/0/lessons/0/blocks/6/sections/0/heading'
        }], env.plan);
        assert.equal(placed, 1);
        assert.match(env.form.fields.sections.issues.textContent, /sections\/0\/heading/);
    });

    it('a failure that is not a validation list shows its code', async () => {
        const env = openWith('listening', {
            setField: () => Promise.resolve({ ok: false, status: 400, data: { error: 'Invalid request', code: 'id_mismatch' } }),
            putPart: () => Promise.resolve({ ok: false, status: 0, data: null })
        });
        env.form.fields.passage.el.value = 'x';
        saveButtonFor(env.form, 'passage').click();
        await tick();
        assert.match(env.form.fields.passage.issues.textContent, /id_mismatch/);
    });
});

describe('read-only fields and escaped rendering (INIT-008/SPEC-011 ac-5)', () => {
    it('audience, assigneeId, id, revision, status, and schemaVersion render read-only with no Save', () => {
        const env = makeEditor();
        const plan = samplePlan();
        const form = env.editor.renderNode({ path: '', node: plan, kind: 'plan' });
        for (const prop of ['audience', 'assigneeId', 'id', 'revision', 'status', 'schemaVersion', 'language']) {
            const handle = form.fields[prop];
            assert.equal(handle.widgetName, 'readonly', prop);
            assert.equal(find(handle.row, 'data-plan-save').length, 0, prop);
        }
        for (const prop of ['title', 'subtitle', 'migration']) {
            assert.equal(find(form.fields[prop].row, 'data-plan-save').length, 1, prop);
        }
        assert.equal(form.fields.migration.widgetName, 'json');
    });

    it('server text is set as text: a value with an <img onerror> makes no element', () => {
        const env = makeEditor();
        const plan = samplePlan();
        const hostile = '<img src=x onerror=alert(1)>';
        plan.title = hostile;
        plan.units[0].lessons[0].title = hostile;
        const block = plan.units[0].lessons[0].blocks.find((b) => b.type === 'introduction');
        block.words[0].meaning = hostile;
        const forms = [
            env.editor.renderNode({ path: '', node: plan, kind: 'plan' }),
            env.editor.renderNode({ path: LESSON + '/blocks/introduction', node: block, kind: 'block' })
        ];
        forms.forEach((f) => env.host.appendChild(f.el));
        env.editor.setIssues([{ rule: 'r', severity: 'warn', message: hostile, path: '' }], plan);
        const tags = dom.collect(env.host, (n) => n.tagName === 'IMG' || n.tagName === 'SCRIPT');
        assert.equal(tags.length, 0);
        assert.equal(forms[0].fields.title.el.value, hostile);
        assert.ok(find(env.host, 'data-plan-issue').some((n) => n.textContent.indexOf(hostile) !== -1));
    });
});

describe('drafts, focus, and listeners', () => {
    it('unsaved text survives a repaint and is dirty again; a saved field does not come back', async () => {
        const env = open('listening');
        env.form.fields.passage.el.value = 'unsaved';
        env.editor.captureDrafts();
        env.editor.reset();
        const again = env.editor.renderNode({ path: env.path, node: env.block, kind: 'block' });
        env.host.appendChild(again.el);
        assert.equal(again.fields.passage.el.value, 'unsaved');
        assert.equal(again.fields.passage.isDirty(), true);
        assert.equal(again.fields.passage.row.getAttribute('data-plan-dirty'), 'true');
        // saving clears the draft
        saveButtonFor(again, 'passage').click();
        await tick();
        assert.equal(again.fields.passage.isDirty(), false);
        env.editor.captureDrafts();
        env.editor.reset();
        const third = env.editor.renderNode({ path: env.path, node: env.block, kind: 'block' });
        assert.equal(third.fields.passage.el.value, env.block.passage);
        assert.equal(env.editor.hasUnsaved(), false);
        // discarding drops drafts
        third.fields.passage.el.value = 'again';
        env.editor.captureDrafts();
        assert.equal(env.editor.hasUnsaved(), true);
        env.editor.discardDrafts();
        env.editor.reset();
        assert.equal(env.editor.hasUnsaved(), false);
    });

    it('focus is restored to the control that had it', () => {
        const env = open('listening');
        env.form.fields.passage.el.focus();
        const key = env.editor.focusKey();
        assert.match(key, /^field:/);
        env.editor.reset();
        const doc = env.doc;
        const form2 = env.editor.renderNode({ path: env.path, node: env.block, kind: 'block' });
        env.host.children.forEach((c) => env.host.removeChild(c));
        env.host.appendChild(form2.el);
        assert.equal(env.editor.restoreFocus(key), true);
        assert.equal(doc.activeElement, form2.fields.passage.el);
    });

    it('destroy removes the delegated listeners from the host', () => {
        const env = open('listening');
        assert.ok(env.host.listeners.click.length >= 1);
        env.editor.destroy();
        assert.equal(env.host.listeners.click.length, 0);
        assert.equal(env.host.listeners.input.length, 0);
        assert.equal(env.host.listeners.change.length, 0);
    });

    it('listeners are registered with the AbortSignal when one is given', () => {
        const doc = dom.makeDocument();
        const host = doc.createElement('div');
        const seen = [];
        host.addEventListener = (type, fn, opts) => seen.push({ type, opts });
        const signal = { aborted: false };
        Form.createEditor({ doc, host, schema: SCHEMA, transport: {}, signal });
        assert.deepEqual(seen.map((s) => s.type).sort(), ['change', 'click', 'input']);
        seen.forEach((s) => assert.equal(s.opts.signal, signal));
    });
});

describe('pointers and paths', () => {
    it('splitPointer turns a validator pointer into the deepest id path and the rest', () => {
        const plan = samplePlan();
        assert.deepEqual(Form.splitPointer(plan, '/units/0/lessons/0/blocks/0/words/0/meaning'),
            { path: LESSON + '/blocks/introduction/words/1', field: 'meaning' });
        assert.deepEqual(Form.splitPointer(plan, '/units/0/lessons/0/blocks/6/sections/0/heading'),
            { path: LESSON + '/blocks/explainer', field: 'sections/0/heading' });
        assert.deepEqual(Form.splitPointer(plan, '/title'), { path: '', field: 'title' });
        assert.deepEqual(Form.splitPointer(plan, ''), { path: '', field: '' });
        assert.deepEqual(Form.splitPointer(plan, '/units/0/lessons/0/newWords/0'), { path: LESSON, field: 'newWords/0' });
    });

    it('kindOfPath and nodeAt follow id paths', () => {
        const plan = samplePlan();
        assert.equal(Form.kindOfPath(''), 'plan');
        assert.equal(Form.kindOfPath('units/u1'), 'unit');
        assert.equal(Form.kindOfPath(LESSON), 'lesson');
        assert.equal(Form.kindOfPath(LESSON + '/blocks/listening'), 'block');
        assert.equal(Form.kindOfPath(LESSON + '/blocks/introduction/words/1'), 'item');
        assert.equal(Form.nodeAt(plan, LESSON + '/blocks/introduction/words/1').id, '1');
        assert.equal(Form.nodeAt(plan, LESSON + '/blocks/none'), null);
    });
});
