/**
 * Assignment editor (INIT-012/SPEC-007).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const Form = require('../shared/admin/plan-form.js');
const Preview = require('../shared/admin/plan-preview.js');
const Render = require('../shared/ui/plan-render.js');
const { validatePlan } = require('../server/plans/validate');
const dom = require('./helpers/fake-dom');
const { LESSON, SCHEMA, HEBREW, makeEditor } = require('./helpers/plan-editor-fixtures');

const HE = 'בַּיִת';
const HE2 = 'סֵפֶר';
const HE3 = 'שֻׁלְחָן';
const LATIN = /[A-Za-z]/;

function tick() {
    return new Promise((resolve) => setImmediate(resolve));
}

function planWithWords(block) {
    return {
        schemaVersion: '1.2.0',
        id: 'as-edit',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Assignment',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'u1', title: 'U', theme: 't',
            lessons: [{
                id: 'l1', title: 'L', level: 'basic', visibility: 'hidden', recipe: 'topic-conversation',
                buildsOn: [], newWords: [], recycles: [],
                blocks: [{
                    id: 'intro', type: 'introduction',
                    words: [
                        { id: 'w1', word: HE, meaning: 'house', translit: 'bayit', group: 1 },
                        { id: 'w2', word: HE2, meaning: 'book', translit: 'sefer', group: 1 },
                        { id: 'w3', word: HE3, meaning: 'table', translit: 'shulchan', group: 1 }
                    ]
                }, block]
            }]
        }]
    };
}

function applyCall(plan, call) {
    const node = Form.nodeAt(plan, call.path);
    if (call.op === 'set') node[call.field] = call.value;
    else if (call.op === 'put') {
        Object.keys(node).forEach((k) => delete node[k]);
        Object.assign(node, call.value);
    }
}

function renderAssignment(plan, block, extra) {
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
    const form = env.editor.renderNode({
        path: LESSON + '/blocks/' + block.id,
        node: block,
        kind: 'block',
        plan
    });
    env.host.appendChild(form.el);
    return Object.assign({}, env, { form, calls, plan });
}

function saveBtn(form, prop) {
    return dom.collect(form.fields[prop].row, (n) => n.hasAttribute && n.hasAttribute('data-plan-save'))[0];
}

async function saveField(form, calls, prop) {
    const before = calls.length;
    saveBtn(form, prop).click();
    await tick();
    assert.ok(calls.length > before, prop);
}

function chromeText(formEl) {
    const bits = [];
    const walk = (n) => {
        if (!n || !n.tagName) return;
        const cls = String(n.className || '');
        if (n.tagName === 'LABEL' || n.tagName === 'BUTTON' || cls.includes('plan-label') || cls.includes('plan-mini-label')) {
            bits.push(String(n.textContent || ''));
        }
        (n.children || []).forEach(walk);
    };
    walk(formEl);
    return bits.join('\n');
}

describe('INIT-012/SPEC-007 assignment editor', () => {
    it('ac-4: assignment form saves prompt, minSentences, three words, two homework lines', async () => {
        globalThis.HebrewPlanBuilder = {
            planWordBank: () => [
                { word: HE, lessonId: 'l1', lessonTitle: 'L' },
                { word: HE2, lessonId: 'l1', lessonTitle: 'L' },
                { word: HE3, lessonId: 'l1', lessonTitle: 'L' }
            ]
        };
        const block = Form.newBlock('assignment', 'as-1', SCHEMA);
        const plan = planWithWords(block);
        const { form, calls } = renderAssignment(plan, block);
        form.fields.prompt.el.value = 'Write about your room.';
        await saveField(form, calls, 'prompt');
        form.fields.minSentences.el.value = '4';
        await saveField(form, calls, 'minSentences');
        const bankButtons = dom.collect(form.el, (n) => n.className && String(n.className).includes('plan-assignment-bank-word'));
        assert.equal(bankButtons.length, 3);
        bankButtons[0].click();
        bankButtons[1].click();
        bankButtons[2].click();
        await saveField(form, calls, 'useWords');
        const addHomework = dom.collect(form.fields.homework.el, (n) => n.getAttribute && n.getAttribute('data-plan-add') === '')[0];
        addHomework.click();
        const hwAreas = dom.collect(form.fields.homework.el, (n) => n.tagName === 'TEXTAREA');
        hwAreas[0].value = 'Line one';
        addHomework.click();
        dom.collect(form.fields.homework.el, (n) => n.tagName === 'TEXTAREA')[1].value = 'Line two';
        await saveField(form, calls, 'homework');
        const saved = Form.nodeAt(plan, LESSON + '/blocks/as-1');
        assert.equal(saved.prompt, 'Write about your room.');
        assert.equal(saved.minSentences, 4);
        assert.deepEqual(saved.useWords, [HE, HE2, HE3]);
        assert.deepEqual(saved.homework, ['Line one', 'Line two']);
        assert.equal(validatePlan(plan).ok, true);
        delete globalThis.HebrewPlanBuilder;
    });

    it('word bank pick and removed word from bank still shows saved word', () => {
        globalThis.HebrewPlanBuilder = {
            planWordBank: () => [{ word: HE2, lessonId: 'l1', lessonTitle: 'L' }]
        };
        const block = Form.newBlock('assignment', 'as-1', SCHEMA);
        block.useWords = [HE];
        const plan = planWithWords(block);
        const env = makeEditor({ getPlan: () => plan });
        const form = env.editor.renderNode({
            path: LESSON + '/blocks/as-1',
            node: block,
            kind: 'block',
            plan
        });
        env.host.appendChild(form.el);
        const words = form.fields.useWords.getValue();
        assert.deepEqual(words, [HE]);
        delete globalThis.HebrewPlanBuilder;
    });

    it('ac-7: preview renders assignment through plan render', () => {
        const block = Object.assign(Form.newBlock('assignment', 'as-1', SCHEMA), {
            prompt: 'Tell a story.',
            minSentences: 2,
            useWords: [HE],
            homework: ['Read again']
        });
        const plan = planWithWords(block);
        const lesson = plan.units[0].lessons[0];
        const prev = Preview.compute({
            render: Render,
            lesson,
            lessonPath: LESSON,
            block,
            blockPath: LESSON + '/blocks/as-1',
            handles: []
        });
        assert.match(prev.html, /plan-assignment/);
    });

    it('ac-8: assignment form chrome is one language (English UI)', () => {
        globalThis.HebrewUtils = { readUiLang: () => 'en' };
        const block = Form.newBlock('assignment', 'as-1', SCHEMA);
        const { form } = renderAssignment(planWithWords(block), block);
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, HEBREW);
        assert.match(chrome, /Prompt|Minimum sentences|Save field/);
        delete globalThis.HebrewUtils;
    });

    it('ac-8: assignment form chrome is one language (Hebrew UI)', () => {
        globalThis.HebrewUtils = { readUiLang: () => 'he' };
        const block = Form.newBlock('assignment', 'as-1', SCHEMA);
        const { form } = renderAssignment(planWithWords(block), block);
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, LATIN);
        assert.match(chrome, /הנחיה|מינימום משפטים|שמירת שדה/);
        delete globalThis.HebrewUtils;
    });

    it('ac-9 CSS stacks assignment and exercise editor below 900px', () => {
        const fs = require('fs');
        const path = require('path');
        const css = fs.readFileSync(path.join(__dirname, '..', 'shared', 'worksheet.css'), 'utf8');
        assert.match(css, /@media \(max-width: 900px\)[\s\S]*plan-exercise-editor/);
        assert.match(css, /var\(--/);
    });
});
