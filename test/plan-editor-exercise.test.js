/**
 * Exercise editor widgets and round-trip (INIT-012/SPEC-007).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const Form = require('../shared/admin/plan-form.js');
const Preview = require('../shared/admin/plan-preview.js');
const Render = require('../shared/ui/plan-render.js');
const { validatePlan } = require('../server/plans/validate');
const dom = require('./helpers/fake-dom');
const { LESSON, SCHEMA, HEBREW, makeEditor, ownFields } = require('./helpers/plan-editor-fixtures');

const XSS = '<img src=x onerror=alert(1)>';
const HE = 'בַּיִת';
const AUDIO = '../_audio/words/1111111111111111111111aa.mp3';
const LATIN = /[A-Za-z]/;

function tick() {
    return new Promise((resolve) => setImmediate(resolve));
}

function planShell(blocks) {
    return {
        schemaVersion: '1.2.0',
        id: 'ex-edit',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Exercise edit',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'u1', title: 'U', theme: 't',
            lessons: [{
                id: 'l1', title: 'L', level: 'basic', visibility: 'hidden', recipe: 'review-worksheet',
                buildsOn: [], newWords: [], recycles: [], blocks
            }]
        }]
    };
}

function applyCall(plan, call) {
    const node = Form.nodeAt(plan, call.path);
    assert.ok(node, call.path);
    if (call.op === 'set') {
        node[call.field] = call.value;
    } else if (call.op === 'put') {
        Object.keys(node).forEach((k) => delete node[k]);
        Object.assign(node, call.value);
    }
}

function makePlanEditor(plan, extra) {
    const calls = [];
    const transport = {
        setField(p, f, v, hooks) {
            calls.push({ op: 'set', path: p, field: f, value: v });
            applyCall(plan, calls[calls.length - 1]);
            if (hooks && hooks.onSaved) hooks.onSaved();
            return Promise.resolve({ ok: true, status: 200, data: {} });
        },
        putPart(p, v, hooks) {
            calls.push({ op: 'put', path: p, value: v });
            applyCall(plan, calls[calls.length - 1]);
            if (hooks && hooks.onSaved) hooks.onSaved();
            return Promise.resolve({ ok: true, status: 200, data: {} });
        }
    };
    const env = makeEditor(Object.assign({ getPlan: () => plan, transport }, extra || {}));
    return Object.assign({}, env, { calls, plan });
}

function renderExercise(plan, block, extra) {
    const env = makePlanEditor(plan, extra);
    const form = env.editor.renderNode({
        path: LESSON + '/blocks/' + block.id,
        node: block,
        kind: 'block',
        plan
    });
    env.host.appendChild(form.el);
    return Object.assign({ form }, env);
}

function saveBtn(form, prop) {
    return dom.collect(form.fields[prop].row, (n) => n.hasAttribute && n.hasAttribute('data-plan-save'))[0];
}

async function saveField(form, env, prop) {
    const before = env.calls.length;
    saveBtn(form, prop).click();
    await tick();
    assert.ok(env.calls.length > before, prop + ' save');
    return env.calls[env.calls.length - 1];
}

function chromeText(formEl) {
    const bits = [];
    const walk = (n) => {
        if (!n || !n.tagName) return;
        const cls = String(n.className || '');
        const tag = n.tagName;
        if (tag === 'LABEL' || tag === 'BUTTON' || cls.includes('plan-label') || cls.includes('plan-mini-label')) {
            bits.push(String(n.textContent || ''));
        }
        (n.children || []).forEach(walk);
    };
    walk(formEl);
    return bits.join('\n');
}

function choiceRows(formEl) {
    return dom.collect(formEl, (n) => n.className && String(n.className).includes('plan-exercise-choice-row'));
}

function itemPromptAreas(itemsEl) {
    return dom.collect(itemsEl, (n) => n.className && String(n.className).includes('plan-exercise-editor-row'))
        .map((row) => dom.collect(row, (n) => n.tagName === 'TEXTAREA')[0]);
}

function itemsHandle(form) {
    return form.fields.items;
}

function exerciseRowBody(itemsEl) {
    const row = dom.collect(itemsEl, (n) => n.className && String(n.className).includes('plan-exercise-editor-row'))[0];
    return row
        ? dom.collect(row, (n) => n.className && String(n.className).includes('plan-exercise-row-body'))[0]
        : null;
}

function stringSublistInputs(itemsEl, index) {
    const sublists = dom.collect(itemsEl, (n) => n.className && String(n.className).includes('plan-string-sublist'));
    const sub = sublists[index];
    if (!sub) return [];
    return dom.collect(sub, (n) => n.tagName === 'INPUT' && n.type === 'text');
}

function acceptInputs(itemsEl) {
    const sublists = dom.collect(itemsEl, (n) => n.className && String(n.className).includes('plan-string-sublist'));
    if (!sublists.length) return [];
    return dom.collect(sublists[sublists.length - 1], (n) => n.tagName === 'INPUT' && n.type === 'text');
}

function assertNoTeacherLeaks(text) {
    assert.doesNotMatch(text, /\bId\b/);
    assert.doesNotMatch(text, /מזהה/);
    assert.doesNotMatch(text, /\bex-1\b/);
    assert.doesNotMatch(text, /\bas-1\b/);
    assert.doesNotMatch(text, /\bitem-1\b/);
    assert.doesNotMatch(text, /\bpath\b/i);
    assert.doesNotMatch(text, /\bindex\b/i);
    assert.doesNotMatch(text, /\bJSON\b/);
    assert.doesNotMatch(text, /\{/);
    assert.doesNotMatch(text, /\}/);
}

describe('INIT-012/SPEC-007 exercise editor', () => {
    it('ac-1: new exercise defaults to choose with item-1', () => {
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        assert.equal(block.kind, 'choose');
        assert.equal(block.items.length, 1);
        assert.equal(block.items[0].id, 'item-1');
    });

    it('ac-1: choose with three choices marks one right in saved value', async () => {
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        const plan = planShell([block]);
        const env = renderExercise(plan, block);
        const prompt = dom.collect(env.form.el, (n) => n.tagName === 'TEXTAREA')[0];
        assert.ok(prompt);
        prompt.value = 'Which?';
        dom.collect(env.form.el, (n) => n.tagName === 'BUTTON' && /Add choice|הוספת אפשרות/.test(n.textContent))[0].click();
        const rows = choiceRows(env.form.el);
        ['one', 'two', 'three'].forEach((text, i) => { rows[i].children[1].value = text; });
        rows[1].children[0].click();
        await saveField(env.form, env, 'items');
        const saved = Form.nodeAt(plan, LESSON + '/blocks/ex-1').items[0];
        assert.equal(saved.answer, 'two');
        assert.deepEqual(saved.choices, ['one', 'two', 'three']);
    });

    describe('ac-2: each kind filled, saved, and reopened', () => {
        async function roundTripKind(spec) {
            const block = Object.assign(Form.newBlock('exercise', 'ex-' + spec.kind, SCHEMA), {
                kind: spec.kind,
                items: spec.preItems || [Form.freshExerciseItem(spec.kind, Form.createTools(SCHEMA), 'item-1')]
            });
            const plan = planShell([block]);
            const path = LESSON + '/blocks/' + block.id;
            let env = renderExercise(plan, block);
            await spec.fill(env.form);
            if (spec.saveKind) {
                await saveField(env.form, env, 'kind');
            }
            await saveField(env.form, env, 'items');
            const savedBlock = Form.nodeAt(plan, path);
            assert.equal(savedBlock.kind, spec.kind);
            spec.assertSaved(savedBlock);
            assert.equal(validatePlan(plan).ok, true, spec.kind + ' validate');
            env = renderExercise(plan, savedBlock);
            spec.assertReopened(env.form);
            const lesson = plan.units[0].lessons[0];
            assert.match(String(Render.renderBlock(savedBlock, { lesson, plan })), /plan-exercise/);
            const prev = Preview.compute({
                render: Render,
                lesson,
                lessonPath: LESSON,
                block: savedBlock,
                blockPath: path,
                handles: []
            });
            assert.ok(prev.html, spec.kind + ' preview');
        }

        it('choose', async () => roundTripKind({
            kind: 'choose',
            fill: async (form) => {
                dom.collect(form.el, (n) => n.tagName === 'TEXTAREA')[0].value = 'Which one?';
                const rows = choiceRows(form.el);
                rows[0].children[1].value = 'a';
                rows[1].children[1].value = 'b';
                rows[0].children[0].click();
            },
            assertSaved: (block) => {
                assert.equal(block.items[0].prompt, 'Which one?');
                assert.equal(block.items[0].answer, 'a');
            },
            assertReopened: (form) => {
                assert.equal(dom.collect(form.el, (n) => n.tagName === 'TEXTAREA')[0].value, 'Which one?');
            }
        }));

        it('true-false', async () => roundTripKind({
            kind: 'true-false',
            fill: async (form) => {
                dom.collect(form.el, (n) => n.tagName === 'TEXTAREA')[0].value = 'Sky is blue.';
                dom.collect(form.el, (n) => n.tagName === 'BUTTON' && /True|נכון/.test(n.textContent))[0].click();
            },
            assertSaved: (block) => {
                assert.equal(block.items[0].statement, 'Sky is blue.');
                assert.equal(block.items[0].answer, true);
            },
            assertReopened: (form) => {
                assert.equal(dom.collect(form.el, (n) => n.tagName === 'TEXTAREA')[0].value, 'Sky is blue.');
            }
        }));

        it('unscramble', async () => roundTripKind({
            kind: 'unscramble',
            fill: async (form) => {
                const words = dom.collect(form.el, (n) => n.tagName === 'INPUT' && n.type === 'text' && n.getAttribute('dir') === 'rtl');
                words[0].value = HE;
                words[1].value = HE;
                words[2].value = HE;
            },
            assertSaved: (block) => {
                assert.deepEqual(block.items[0].words, [HE, HE, HE]);
            },
            assertReopened: (form) => {
                const handle = itemsHandle(form);
                const item = handle.getValue()[0];
                assert.deepEqual(item.words, [HE, HE, HE]);
                assert.equal(Object.prototype.hasOwnProperty.call(item, 'english'), false);
                const body = exerciseRowBody(handle.el);
                assert.deepEqual(stringSublistInputs(handle.el, 0).map((n) => n.value), [HE, HE, HE]);
                const english = dom.collect(body, (n) => n.tagName === 'INPUT' && n.type === 'text' && n.getAttribute('lang') === 'en')[0];
                assert.equal(english.value, '');
            }
        }));

        it('translate', async () => roundTripKind({
            kind: 'translate',
            fill: async (form) => {
                dom.collect(form.el, (n) => n.tagName === 'INPUT' && n.type === 'text' && n.getAttribute('lang') === 'en')[0].value = 'house';
                dom.collect(form.el, (n) => n.tagName === 'INPUT' && n.type === 'text' && n.getAttribute('dir') === 'rtl')[0].value = HE;
            },
            assertSaved: (block) => {
                assert.equal(block.items[0].english, 'house');
                assert.equal(block.items[0].answer, HE);
            },
            assertReopened: (form) => {
                const handle = itemsHandle(form);
                const item = handle.getValue()[0];
                assert.equal(item.english, 'house');
                assert.equal(item.answer, HE);
                assert.equal(Object.prototype.hasOwnProperty.call(item, 'accept'), false);
                const body = exerciseRowBody(handle.el);
                assert.equal(dom.collect(body, (n) => n.tagName === 'INPUT' && n.getAttribute('lang') === 'en')[0].value, 'house');
                assert.equal(dom.collect(body, (n) => n.tagName === 'INPUT' && n.getAttribute('dir') === 'rtl')[0].value, HE);
                assert.deepEqual(acceptInputs(handle.el).map((n) => n.value), []);
            }
        }));

        it('dictation', async () => roundTripKind({
            kind: 'dictation',
            preItems: [{ id: 'item-1', audioPath: AUDIO, answer: '' }],
            fill: async (form) => {
                dom.collect(form.el, (n) => n.tagName === 'INPUT' && n.type === 'text' && n.getAttribute('dir') === 'rtl')[0].value = HE;
            },
            assertSaved: (block) => {
                assert.equal(block.items[0].audioPath, AUDIO);
                assert.equal(block.items[0].answer, HE);
            },
            assertReopened: (form) => {
                const handle = itemsHandle(form);
                const item = handle.getValue()[0];
                assert.equal(item.audioPath, AUDIO);
                assert.equal(item.answer, HE);
                assert.equal(Object.prototype.hasOwnProperty.call(item, 'accept'), false);
                const body = exerciseRowBody(handle.el);
                const select = dom.collect(body, (n) => n.getAttribute && n.getAttribute('data-plan-audio-select') === '')[0];
                assert.equal(select.value, AUDIO);
                assert.equal(dom.collect(body, (n) => n.tagName === 'INPUT' && n.getAttribute('dir') === 'rtl')[0].value, HE);
                assert.deepEqual(acceptInputs(handle.el).map((n) => n.value), []);
            }
        }));

        it('cleared optional heading uses putPart and drops the field', async () => {
            const block = Object.assign(Form.newBlock('exercise', 'ex-h', SCHEMA), { heading: 'Warm-up' });
            const plan = planShell([block]);
            const env = renderExercise(plan, block, {
                confirm: () => true
            });
            const clear = dom.collect(env.form.fields.heading.row, (n) => n.hasAttribute && n.hasAttribute('data-plan-clear'))[0];
            assert.ok(clear);
            clear.click();
            await tick();
            const last = env.calls[env.calls.length - 1];
            assert.equal(last.op, 'put');
            assert.equal(Object.prototype.hasOwnProperty.call(last.value, 'heading'), false);
            assert.equal(Object.prototype.hasOwnProperty.call(Form.nodeAt(plan, LESSON + '/blocks/ex-h'), 'heading'), false);
        });
    });

    it('ac-3: refuses to remove the choice marked right', () => {
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        block.items[0].choices = ['a', 'b', 'c'];
        block.items[0].answer = 'b';
        const plan = planShell([block]);
        const { form } = renderExercise(plan, block);
        const note = dom.collect(form.el, (n) => n.getAttribute && n.getAttribute('data-plan-exercise-note') === '')[0];
        const removeMarked = dom.collect(form.el, (n) => n.getAttribute && n.getAttribute('data-plan-choice-remove') === '')[1];
        removeMarked.click();
        assert.match(note.textContent, /Clear the right mark first, or mark another choice\./);
        assert.equal(form.fields.items.getValue()[0].choices.length, 3);
    });

    it('refuses a sixth choice and removing the only item', () => {
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        const plan = planShell([block]);
        const { form } = renderExercise(plan, block);
        const note = dom.collect(form.el, (n) => n.getAttribute && n.getAttribute('data-plan-exercise-note') === '')[0];
        const addChoice = dom.collect(form.el, (n) => n.tagName === 'BUTTON' && /Add choice|הוספת אפשרות/.test(n.textContent))[0];
        assert.ok(addChoice);
        for (let i = 0; i < 4; i++) addChoice.click();
        addChoice.click();
        assert.match(note.textContent, /five|חמש/);
        const removeItem = dom.collect(form.el, (n) => n.getAttribute && n.getAttribute('data-plan-item-op') === 'remove')[0];
        removeItem.click();
        assert.match(note.textContent, /only item|פריט היחיד/);
    });

    it('ac-10: XSS in prompt is text in form and preview', () => {
        const block = Form.newBlock('exercise', 'ex-xss', SCHEMA);
        block.items[0].prompt = XSS;
        const plan = planShell([block]);
        const { form } = renderExercise(plan, block);
        const promptInput = dom.collect(form.el, (n) => n.tagName === 'TEXTAREA' && n.value === XSS)[0];
        assert.ok(promptInput, 'prompt shows authored XSS as value');
        assert.equal(String(form.el.innerHTML || '').includes('onerror='), false);
        const lesson = plan.units[0].lessons[0];
        const prev = Preview.compute({
            render: Render,
            lesson,
            lessonPath: LESSON,
            block,
            blockPath: LESSON + '/blocks/' + block.id,
            handles: []
        });
        assert.ok(prev.html && (prev.html.includes('&lt;img') || !prev.html.includes('onerror=')));
    });

    it('ac-6: no id, path, index, or JSON in exercise or assignment forms', () => {
        for (const type of ['exercise', 'assignment']) {
            const block = Form.newBlock(type, type === 'exercise' ? 'ex-1' : 'as-1', SCHEMA);
            const plan = planShell([block]);
            const { form } = renderExercise(plan, block);
            assertNoTeacherLeaks(form.el.textContent);
            ownFields(form.el).forEach((n) => assert.notEqual(n.getAttribute('data-plan-widget'), 'json'));
        }
    });

    it('ac-8: exercise form chrome is one language (English UI)', () => {
        globalThis.HebrewUtils = { readUiLang: () => 'en' };
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        const { form } = renderExercise(planShell([block]), block);
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, HEBREW);
        assert.match(chrome, /What kind\?|Items|Save field/);
        delete globalThis.HebrewUtils;
    });

    it('ac-8: exercise form chrome is one language (Hebrew UI)', () => {
        globalThis.HebrewUtils = { readUiLang: () => 'he' };
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        const { form } = renderExercise(planShell([block]), block);
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, LATIN);
        assert.match(chrome, /סוג התרגיל|פריטים|שמירת שדה/);
        delete globalThis.HebrewUtils;
    });

    it('ac-5: future tense table cells widget shows nine rows and lexicon notes', async () => {
        const gets = [];
        globalThis.HebrewApi = {
            get(url) {
                gets.push(url);
                if (String(url).includes('missing')) return Promise.resolve({ ok: false, status: 404 });
                return Promise.resolve({ ok: true, data: { entry: { verified: 0 } } });
            }
        };
        const block = {
            id: 'pat-1', type: 'conjugation-table', lemmaKey: HE, tense: 'future', mode: 'study',
            cells: [{ cell: '1s', formKey: 'missing' }, { cell: '2ms', formKey: HE }]
        };
        const plan = planShell([block]);
        const env = makeEditor({ getPlan: () => plan });
        const form = env.editor.renderNode({
            path: LESSON + '/blocks/pat-1',
            node: block,
            kind: 'block',
            plan
        });
        env.host.appendChild(form.el);
        assert.equal(form.fields.cells.widgetName, 'table-cells');
        const rows = dom.collect(form.el, (n) => n.getAttribute && n.getAttribute('data-plan-cell-code'));
        assert.equal(rows.length, 9);
        await new Promise((r) => setTimeout(r, 15));
        const notes = dom.collect(form.el, (n) => n.tagName === 'P' && !n.hidden && /dictionary|verified|מילון|אומתה/.test(n.textContent));
        assert.ok(notes.length >= 1);
        delete globalThis.HebrewApi;
    });

    it('aud-1: kind change warns once, then clears items on confirm', async () => {
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        block.items[0].prompt = 'Filled in';
        const plan = planShell([block]);
        const confirms = [];
        let refused = false;
        const env = renderExercise(plan, block, {
            confirm(msg) {
                confirms.push(String(msg));
                if (!refused) {
                    refused = true;
                    return false;
                }
                return true;
            }
        });
        env.form.fields.kind.el.value = 'true-false';
        env.form.fields.kind.el.dispatch('change');
        const before = env.calls.length;
        saveBtn(env.form, 'kind').click();
        await tick();
        assert.equal(env.calls.length, before);
        assert.equal(plan.units[0].lessons[0].blocks[0].kind, 'choose');
        assert.equal(plan.units[0].lessons[0].blocks[0].items[0].prompt, 'Filled in');
        assert.ok(confirms.some((m) => /clear the items you already filled in/i.test(m)));
        await saveField(env.form, env, 'kind');
        const saved = plan.units[0].lessons[0].blocks[0];
        assert.equal(saved.kind, 'true-false');
        assert.equal(saved.items.length, 1);
        assert.equal(saved.items[0].id, 'item-1');
        assert.equal(Object.prototype.hasOwnProperty.call(saved.items[0], 'statement'), true);
        assert.equal(Object.prototype.hasOwnProperty.call(saved.items[0], 'prompt'), false);
    });

    it('aud-1: duplicate item text saves and round-trips', async () => {
        const block = Object.assign(Form.newBlock('exercise', 'ex-dup', SCHEMA), {
            kind: 'true-false',
            items: [Form.freshExerciseItem('true-false', Form.createTools(SCHEMA), 'item-1')]
        });
        const plan = planShell([block]);
        const env = renderExercise(plan, block);
        itemPromptAreas(env.form.fields.items.el)[0].value = 'Same text';
        dom.collect(env.form.fields.items.el, (n) => n.getAttribute && n.getAttribute('data-plan-add') === '')[0].click();
        itemPromptAreas(env.form.fields.items.el)[1].value = 'Same text';
        await saveField(env.form, env, 'items');
        const items = plan.units[0].lessons[0].blocks[0].items;
        assert.equal(items.length, 2);
        assert.equal(items[0].statement, 'Same text');
        assert.equal(items[1].statement, 'Same text');
        assert.equal(validatePlan(plan).ok, true);
        const reopened = renderExercise(plan, plan.units[0].lessons[0].blocks[0]).form;
        const reopenedPrompts = itemPromptAreas(reopened.fields.items.el);
        assert.equal(reopenedPrompts[0].value, 'Same text');
        assert.equal(reopenedPrompts[1].value, 'Same text');
    });

    it('aud-1: touched items are detected before a kind change', () => {
        const block = Form.newBlock('exercise', 'ex-1', SCHEMA);
        assert.equal(Form.exerciseItemsTouched(block.items, block.kind), false);
        block.items[0].prompt = 'Filled';
        assert.equal(Form.exerciseItemsTouched(block.items, block.kind), true);
    });
});
