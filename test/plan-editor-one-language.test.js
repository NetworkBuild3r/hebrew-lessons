/**
 * One-language chrome and visible refusals on /editor/ (INIT-011/SPEC-003).
 * Covers ac-1 (count + issue jump are .ui-pair, not one bilingual string) and
 * ac-3 (refused Add lesson / schema refusal show a sentence; two adds → three rows).
 * Provenance: INIT-011/SPEC-003
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { loadPanels, byAttr, sleep } = require('./helpers/plan-panels-vm');
const Ui = require('../shared/admin/plan-ui.js');
const Form = require('../shared/admin/plan-form.js');
const { LESSON, SCHEMA, makeEditor } = require('./helpers/plan-editor-fixtures');

const ROOT = path.resolve(__dirname, '..');
const CSS = fs.readFileSync(path.join(ROOT, 'shared', 'worksheet.css'), 'utf8');
const HEBREW = /[\u0590-\u05FF]/;

function one(node, name, value) {
    return byAttr(node, name, value)[0];
}

function pairParts(node) {
    const kids = node && Array.isArray(node.children) ? node.children : [];
    const he = kids.find((n) => n.className && String(n.className).indexOf('ui-he') !== -1);
    const en = kids.find((n) => n.className && String(n.className).indexOf('ui-en') !== -1);
    return { he, en };
}

function byClass(root, className) {
    const out = [];
    const walk = (n) => {
        if (n && n.className && String(n.className).split(/\s+/).indexOf(className) !== -1) out.push(n);
        (n.children || []).forEach(walk);
    };
    walk(root);
    return out;
}

describe('INIT-011/SPEC-003: splitUiLabel and search field', () => {
    it('splitUiLabel tolerates a trailing space and English inner parentheses', () => {
        assert.deepEqual(Ui.splitUiLabel('ספירה (Count) '), { he: 'ספירה', en: 'Count', suffix: '' });
        assert.deepEqual(
            Ui.splitUiLabel('סינון (Filter by path (optional))'),
            { he: 'סינון', en: 'Filter by path (optional)', suffix: '' }
        );
        assert.deepEqual(
            Ui.splitUiLabel('נשמר (Saved) · 4'),
            { he: 'נשמר', en: 'Saved', suffix: ' · 4' }
        );
        assert.equal(Ui.splitUiLabel('4 תוכניות (4 plans)').he, '4 תוכניות');
        assert.equal(Ui.splitUiLabel('4 תוכניות (4 plans)').en, '4 plans');
    });

    it('ac-2: .plan-filter-search has no background image', () => {
        assert.match(CSS, /\[data-plan-builder\] \.plan-filter-search \{[\s\S]*?background-image:\s*none/);
        assert.match(CSS, /plan-filter-search::-webkit-search-decoration/);
    });
});

describe('INIT-011/SPEC-003 ac-1: one language for chrome', () => {
    it('plan library count is .ui-pair: English span has no Hebrew, Hebrew span has no "plans"', async () => {
        const env = loadPanels({
            plans: [
                { id: 'a', title: 'A', status: 'draft', audience: 'shared' },
                { id: 'b', title: 'B', status: 'draft', audience: 'shared' },
                { id: 'c', title: 'כותרת עברית', status: 'draft', audience: 'shared' },
                { id: 'd', title: 'D', status: 'draft', audience: 'shared' }
            ]
        });
        const panel = env.panel();
        env.W.HebrewPlanList.mount(panel, {
            doc: env.W.document,
            api: env.W.HebrewApi,
            setHash: (r) => env.hashes.push(r)
        });
        await sleep(40);
        const status = one(panel, 'data-plan-list-status');
        assert.ok(status, 'status line');
        assert.match(status.className, /ui-pair/);
        const parts = pairParts(status);
        assert.ok(parts.he && parts.en, 'count split into .ui-he and .ui-en');
        assert.match(parts.he.textContent, /תוכניות/);
        assert.doesNotMatch(parts.he.textContent, /\bplans\b/i);
        assert.match(parts.en.textContent, /plans/i);
        assert.doesNotMatch(parts.en.textContent, HEBREW);
        // A Hebrew lesson/plan title stays Hebrew in the row (content, not chrome).
        const titleCell = byAttr(panel, 'data-plan-open-link', 'c')[0];
        assert.ok(titleCell);
        assert.match(titleCell.textContent, /כותרת עברית/);
    });

    it('issue jump label is .ui-pair: English gloss has no Hebrew', async () => {
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
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
                }]
            }]
        };
        const env = loadPanels({
            plan: plan,
            revision: 4,
            issues: {
                revision: 4,
                status: 'draft',
                counts: { block: 1, warn: 0, inform: 0 },
                truncated: false,
                issues: [{
                    code: 'schema',
                    severity: 'block',
                    hard: true,
                    path: 'units/unit-1/lessons/lesson-1',
                    field: 'title',
                    pointer: '/units/0/lessons/0/title',
                    message: 'missing required field: title'
                }]
            }
        });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(60);
        const go = byClass(panel, 'plan-issue-go')[0];
        assert.ok(go, 'jump label');
        assert.match(go.className, /ui-pair/);
        const parts = pairParts(go);
        assert.ok(parts.he && parts.en);
        assert.match(parts.he.textContent, /מעבר/);
        assert.doesNotMatch(parts.he.textContent, /Go to the field/i);
        assert.doesNotMatch(parts.en.textContent, HEBREW);
        assert.match(parts.en.textContent, /Go to the field/i);
    });
});

describe('INIT-011/SPEC-003 ac-3: refused writes show a sentence', () => {
    it('a refused Add lesson (no plan) shows data-plan-error with a reason', async () => {
        const env = loadPanels({
            plan: {
                schemaVersion: '1.0.0',
                id: 'sample',
                revision: 4,
                status: 'draft',
                audience: 'shared',
                assigneeId: null,
                title: 'Empty units',
                language: { target: 'he', gloss: 'en' },
                units: []
            },
            revision: 4
        });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        const add = one(panel, 'data-plan-add-lesson');
        assert.ok(add);
        add.click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        const err = one(panel, 'data-plan-error');
        assert.ok(err);
        assert.equal(err.hidden, false);
        assert.match(err.textContent, /unit|plan|lesson/i);
        assert.doesNotMatch(err.textContent, /\/units\/|\$\.|instancePath/);
    });

    it('a schema refusal names the activity and the missing field, not a JSON path', async () => {
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
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
                }]
            }]
        };
        const env = loadPanels({ plan: plan, revision: 4 });
        env.on('POST', /\/blocks$/, () => env.json(400, {
            error: 'validation',
            revision: 4,
            blocks: [{
                rule: 'schema',
                severity: 'block',
                path: '/units/0/lessons/0/blocks/0/words',
                message: 'missing required field: words'
            }],
            warnings: [],
            informs: []
        }));
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        // Lesson screen holds the activity menu (INIT-011/SPEC-005); open the lesson first.
        one(panel, 'data-plan-lesson').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-new-id').length, 0, 'no Id field');
        const typeSelect = one(panel, 'data-plan-new-type');
        assert.ok(typeSelect);
        // Pick Words → introduction
        const wordOpt = Array.from(typeSelect.children || []).find((o) => {
            return o.getAttribute && o.getAttribute('data-plan-activity-name') === 'Words';
        });
        if (wordOpt) {
            typeSelect.value = wordOpt.value;
            typeSelect.dispatch('change');
        } else {
            typeSelect.value = 'introduction';
        }
        one(panel, 'data-plan-insert').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        const err = one(panel, 'data-plan-error');
        assert.ok(err);
        assert.equal(err.hidden, false);
        assert.match(err.textContent, /Words/i);
        assert.match(err.textContent, /words/i);
        assert.doesNotMatch(err.textContent, /\/units\/0\/lessons/);
        assert.doesNotMatch(err.textContent, /instancePath|\$\./);
    });

    it('adding two lessons still ends with three rows', async () => {
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
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
                }]
            }]
        };
        const env = loadPanels({ plan: plan, revision: 4 });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-stack-row').length, 1);
        byAttr(panel, 'data-plan-add-lesson-title')[0].value = 'Second';
        byAttr(panel, 'data-plan-add-lesson')[0].click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        byAttr(panel, 'data-plan-add-lesson-title')[0].value = 'Third';
        byAttr(panel, 'data-plan-add-lesson')[0].click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-stack-row').length, 3, 'three lesson rows after two adds');
    });
});

describe('INIT-011/SPEC-003 leftover chrome: cut, revision title, new-plan title error', () => {
    it('a truncated diff value shows cut / נחתך as .ui-pair, not one nested-paren string', async () => {
        // The old label " … (נחתך (cut))" does not split — English mode would show Hebrew.
        assert.equal(Ui.splitUiLabel(' … (נחתך (cut))'), null);
        const cutDiff = {
            from: { ref: 1, revision: 1 },
            to: { ref: 'working', revision: 4 },
            truncated: false,
            entries: [{
                path: 'units/u/lessons/l',
                field: 'title',
                kind: 'changed',
                before: 'x…',
                after: 'y…',
                valueTruncated: true
            }]
        };
        const env = loadPanels({
            revisions: [{ revision: 4, op: 'set_field', actorKind: 'human', actorName: 'a', at: '2026-10-12T21:00:00.000Z' }],
            diff: cutDiff
        });
        const history = env.W.HebrewPlanHistory.create({
            doc: env.W.document,
            api: env.W.HebrewApi,
            planId: 'sample',
            getRevision: () => 4,
            hasUnsaved: () => false,
            onRestored: () => Promise.resolve(null),
            onReload: () => {}
        });
        byAttr(history.el, 'data-plan-history-toggle')[0].click();
        await sleep(30);
        byAttr(history.el, 'data-plan-diff-run')[0].click();
        await sleep(40);
        const mark = byAttr(history.el, 'data-plan-diff-cut')[0];
        assert.ok(mark, 'truncated cut marker');
        assert.match(mark.className, /ui-pair/);
        const parts = pairParts(mark);
        assert.ok(parts.he && parts.en);
        assert.match(parts.he.textContent, /נחתך/);
        assert.doesNotMatch(parts.he.textContent, /\bcut\b/i);
        assert.match(parts.en.textContent, /\bcut\b/i);
        assert.doesNotMatch(parts.en.textContent, HEBREW);
    });

    it('revision detail title is .ui-pair with N in both languages', async () => {
        const env = loadPanels({
            revisions: [{
                revision: 7,
                op: 'set_field',
                actorKind: 'human',
                actorName: 'example-admin',
                at: '2026-10-12T21:00:00.000Z',
                path: '',
                field: '',
                summary: 'set_field',
                tool: 'set_field',
                client: null
            }]
        });
        const history = env.W.HebrewPlanHistory.create({
            doc: env.W.document,
            api: env.W.HebrewApi,
            planId: 'sample',
            getRevision: () => 7,
            hasUnsaved: () => false,
            onRestored: () => Promise.resolve(null),
            onReload: () => {}
        });
        byAttr(history.el, 'data-plan-history-toggle')[0].click();
        await sleep(30);
        await history.openRevision(7);
        await sleep(40);
        const title = byAttr(history.el, 'data-plan-rev-title')[0];
        assert.ok(title, 'revision title');
        assert.match(title.className, /ui-pair/);
        const parts = pairParts(title);
        assert.ok(parts.he && parts.en);
        assert.match(parts.he.textContent, /גרסה 7/);
        assert.doesNotMatch(parts.he.textContent, /Revision/i);
        assert.match(parts.en.textContent, /Revision 7/i);
        assert.doesNotMatch(parts.en.textContent, HEBREW);
    });

    it('new-plan empty title error is .ui-pair, not Ui.label flattened into textContent', async () => {
        const env = loadPanels({ plans: [{ id: 'a', title: 'A', status: 'draft', audience: 'shared' }] });
        const panel = env.panel();
        env.W.HebrewPlanList.mount(panel, {
            doc: env.W.document,
            api: env.W.HebrewApi,
            setHash: (r) => env.hashes.push(r)
        });
        await sleep(40);
        byAttr(panel, 'data-plan-new-open')[0].click();
        await sleep(20);
        const titleInput = byAttr(panel, 'data-plan-new-title')[0];
        titleInput.value = '   ';
        byAttr(panel, 'data-plan-create')[0].click();
        await sleep(20);
        const msg = byAttr(panel, 'data-plan-new-title-message')[0];
        assert.ok(msg);
        assert.equal(msg.hidden, false);
        assert.match(msg.className, /ui-pair/);
        const parts = pairParts(msg);
        assert.ok(parts.he && parts.en);
        assert.match(parts.he.textContent, /צריך כותרת/);
        assert.doesNotMatch(parts.he.textContent, /title is needed/i);
        assert.match(parts.en.textContent, /title is needed/i);
        assert.doesNotMatch(parts.en.textContent, HEBREW);
    });
});

describe('INIT-012/SPEC-007 ac-8: exercise and assignment one-language chrome', () => {
    const LATIN = /[A-Za-z]/;

    function chromeText(formEl) {
        const bits = [];
        const walk = (n) => {
            if (!n || !n.tagName) return;
            const cls = String(n.className || '');
            if (n.tagName === 'LABEL' || n.tagName === 'BUTTON' || cls.includes('plan-label')) {
                bits.push(String(n.textContent || ''));
            }
            (n.children || []).forEach(walk);
        };
        walk(formEl);
        return bits.join('\n');
    }

    function renderBlock(type, lang) {
        globalThis.HebrewUtils = { readUiLang: () => lang };
        const block = Form.newBlock(type, type === 'exercise' ? 'ex-1' : 'as-1', SCHEMA);
        const plan = {
            schemaVersion: '1.2.0', id: 'ol', revision: 1, status: 'draft', audience: 'shared', assigneeId: null,
            title: 'One language', language: { target: 'he', gloss: 'en' },
            units: [{ id: 'u1', title: 'U', theme: 't', lessons: [{
                id: 'l1', title: 'L', level: 'basic', visibility: 'hidden', recipe: 'review-worksheet',
                buildsOn: [], newWords: [], recycles: [], blocks: [block]
            }] }]
        };
        const env = makeEditor({ getPlan: () => plan });
        const form = env.editor.renderNode({
            path: LESSON + '/blocks/' + block.id,
            node: block,
            kind: 'block',
            plan
        });
        env.host.appendChild(form.el);
        delete globalThis.HebrewUtils;
        return form;
    }

    it('exercise labels are English-only in English UI', () => {
        const form = renderBlock('exercise', 'en');
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, HEBREW);
        assert.match(chrome, /What kind\?|Items/);
    });

    it('exercise labels are Hebrew-only in Hebrew UI', () => {
        const form = renderBlock('exercise', 'he');
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, LATIN);
        assert.match(chrome, /סוג התרגיל|פריטים/);
    });

    it('assignment labels are English-only in English UI', () => {
        const form = renderBlock('assignment', 'en');
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, HEBREW);
        assert.match(chrome, /Prompt|Minimum sentences/);
    });

    it('assignment labels are Hebrew-only in Hebrew UI', () => {
        const form = renderBlock('assignment', 'he');
        const chrome = chromeText(form.el);
        assert.doesNotMatch(chrome, LATIN);
        assert.match(chrome, /הנחיה|מינימום משפטים/);
    });
});
