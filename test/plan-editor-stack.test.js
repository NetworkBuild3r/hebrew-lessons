/**
 * Ordered lesson stack and empty new-lesson form (INIT-009/SPEC-003).
 * Fails if a new lesson form includes the six section markers before blocks are added,
 * and asserts lesson reorder round-trips through set_field on the unit.
 * Provenance: INIT-009/SPEC-003
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('node:vm');

const Form = require('../shared/admin/plan-form.js');
const { loadPanels, byAttr, sleep } = require('./helpers/plan-panels-vm');
const { SCHEMA, makeEditor } = require('./helpers/plan-editor-fixtures');

const SECTION_MARKERS = [
    'data-section="vocab"',
    'data-section="activities"',
    'data-section="fill"',
    'data-section="grammar"',
    'data-section="reading"',
    'data-section="askFriend"',
    'lesson-section-vocab',
    'lesson-section-activities',
    'lesson-section-fill',
    'lesson-section-grammar',
    'lesson-section-reading',
    'lesson-section-askFriend'
];

function twoLessonPlan() {
    return {
        schemaVersion: '1.0.0',
        id: 'sample',
        revision: 4,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Stack sample',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-1',
            title: 'יחידה',
            theme: 'home',
            lessons: [
                {
                    id: 'lesson-1',
                    title: 'ראשון',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: [],
                    newWords: [],
                    recycles: [],
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
                },
                {
                    id: 'lesson-2',
                    title: 'שני',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: [],
                    newWords: [],
                    recycles: [],
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
                }
            ]
        }]
    };
}

function assertNoSixSections(htmlOrNode) {
    if (htmlOrNode && typeof htmlOrNode === 'object' && htmlOrNode.querySelectorAll) {
        const bad = ['vocab', 'activities', 'fill', 'grammar', 'reading', 'askFriend'];
        for (const key of bad) {
            assert.equal(
                htmlOrNode.querySelectorAll('[data-section="' + key + '"]').length,
                0,
                'must not include data-section=' + key
            );
            assert.equal(
                htmlOrNode.querySelectorAll('[id="lesson-section-' + key + '"]').length,
                0,
                'must not include id lesson-section-' + key
            );
        }
        return;
    }
    const text = String(htmlOrNode || '');
    for (const marker of SECTION_MARKERS) {
        assert.equal(text.includes(marker), false, 'must not include ' + marker);
    }
}

function loadLessonsModule() {
    const repo = path.resolve(__dirname, '..');
    const sandbox = { console, document: { readyState: 'complete' }, window: {}, globalThis: {} };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(repo, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(repo, 'shared', 'admin', 'lessons.js'), 'utf8'), sandbox);
    return sandbox.HebrewAdminLessons;
}

describe('plan editor stack (INIT-009/SPEC-003)', () => {
    it('ac-1: a new lesson form has none of the six section markers', () => {
        const plan = twoLessonPlan();
        const lesson = plan.units[0].lessons[0];
        const { editor } = makeEditor({ getPlan: () => plan });
        const form = editor.renderNode({
            path: 'units/unit-1/lessons/lesson-1',
            node: lesson,
            kind: 'lesson',
            plan: plan
        });
        assertNoSixSections(form.el);
        const types = Form.blockTypeList(SCHEMA).map((t) => t.type);
        assert.ok(types.indexOf('introduction') !== -1);
        assert.ok(types.indexOf('graded-story') !== -1);
        assert.ok(types.indexOf('fluency-pass') !== -1);
        assert.equal(lesson.blocks.length, 1);
        assert.equal(lesson.blocks[0].type, 'fluency-pass');
        assert.equal(Object.prototype.hasOwnProperty.call(lesson, 'vocab'), false);
        assert.equal(Object.prototype.hasOwnProperty.call(lesson, 'fill'), false);
        assert.equal(Object.prototype.hasOwnProperty.call(lesson, 'grammar'), false);
        assert.equal(Object.prototype.hasOwnProperty.call(lesson, 'reading'), false);
        assert.equal(Object.prototype.hasOwnProperty.call(lesson, 'askFriend'), false);
    });

    it('ac-1: blankWorksheet skips the six-part ladder; no-plan gate is gone', () => {
        const Lessons = loadLessonsModule();
        const blank = Lessons.blankWorksheet('x', 'כותרת');
        assert.ok(Array.isArray(blank.activities));
        assert.equal(blank.activities.length, 0);
        assert.ok(blank.vocab && Array.isArray(blank.vocab.items));
        assert.equal(blank.vocab.items.length, 0);
        assert.equal(blank.fill, undefined);
        assert.equal(blank.grammar, undefined);
        assert.equal(blank.reading, undefined);
        assert.equal(blank.askFriend, undefined);
        assert.equal(typeof Lessons.noPlanHtml, 'undefined');
        assert.equal(typeof Lessons.mountLessonGate, 'undefined');
    });

    it('ac-2: reordering two lessons changes their order in the saved plan', async () => {
        const plan = twoLessonPlan();
        const env = loadPanels({ plan: plan, revision: 4 });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.deepEqual(
            env.server.plan.units[0].lessons.map((l) => l.id),
            ['lesson-1', 'lesson-2']
        );
        const rows = byAttr(panel, 'data-plan-stack-row');
        assert.equal(rows.length, 2, 'stack shows both lessons');
        const up = byAttr(rows[1], 'data-plan-lesson-move', 'up')[0];
        assert.ok(up, 'up control on second lesson');
        up.click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.deepEqual(
            env.server.plan.units[0].lessons.map((l) => l.id),
            ['lesson-2', 'lesson-1'],
            'saved plan lesson order'
        );
        const moveCalls = env.callsOf('POST', /\/lessons\/move$/);
        assert.ok(moveCalls.length >= 1);
        const last = moveCalls[moveCalls.length - 1];
        assert.equal(last.body.path, 'units/unit-1/lessons/lesson-2');
        assert.equal(last.body.index, 0);
        const after = byAttr(panel, 'data-plan-stack-row');
        assert.deepEqual(after.map((r) => r.getAttribute('data-plan-stack-row')), ['lesson-2', 'lesson-1']);
    });

    it('ac-3: buildsOn lists earlier lessons and reads the chosen lesson id', () => {
        const plan = twoLessonPlan();
        const later = plan.units[0].lessons[1];
        const { editor } = makeEditor({ getPlan: () => plan });
        const form = editor.renderNode({
            path: 'units/unit-1/lessons/lesson-2',
            node: later,
            kind: 'lesson',
            plan: plan
        });
        assert.equal(form.fields.buildsOn.widgetName, 'string-list');
        const box = form.fields.buildsOn.el;
        assert.equal(box.getAttribute('data-plan-builds-on'), '');
        const check = box.querySelector('[data-plan-builds-on-id="lesson-1"]');
        assert.ok(check, 'earlier lesson-1 is offered');
        assert.equal(box.querySelector('[data-plan-builds-on-id="lesson-2"]'), null);
        check.checked = true;
        assert.deepEqual(form.fields.buildsOn.getValue(), ['lesson-1']);
        const earlier = Form.earlierLessonsInPlan(plan, 'units/unit-1/lessons/lesson-2');
        assert.deepEqual(earlier.map((e) => e.id), ['lesson-1']);
    });
});
