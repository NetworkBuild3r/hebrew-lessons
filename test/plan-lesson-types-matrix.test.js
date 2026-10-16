/**
 * Lesson-type matrix: fixtures validate, render, and guard (INIT-012/SPEC-009).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

require('../shared/ui/blocks/explainer');
require('../shared/ui/blocks/exercise');
require('../shared/ui/blocks/assignment');
require('../shared/ui/blocks/conjugation-table');
require('../shared/ui/blocks/personal-question');
require('../shared/ui/blocks/verse-reading');
require('../shared/ui/blocks/letter-intro');

const Form = require('../shared/admin/plan-form.js');
const Preview = require('../shared/admin/plan-preview.js');
const { validatePlan } = require('../server/plans/validate');
const Render = require('../shared/ui/plan-render');
const dom = require('./helpers/fake-dom');
const { LESSON, SCHEMA, makeEditor } = require('./helpers/plan-editor-fixtures');
const fx = require('./fixtures/lesson-types/fixture-lib');

const TOOLS = Form.createTools(SCHEMA);

const INITIATIVE_TYPES = ['A', 'B', 'C', 'D', 'E', 'dictation'];
const MATCH_NOTE = 'match: dependent (INIT-010 SPEC-004)';

const REGISTRY_TYPES = ['explainer', 'conjugation-table', 'personal-question', 'verse-reading', 'letter-intro', 'exercise', 'assignment'];
const INLINE_RENDER_TYPES = ['introduction', 'graded-story', 'listening', 'short-dialogue', 'fluency-pass', 'spaced-review-checkpoint'];

const SPEC009_FORMS_MD = path.join(
    path.resolve(__dirname, '..'),
    'sdd/initiatives/INIT-012-lesson-type-coverage/specs/completion-summaries/SPEC-009-forms-to-check.md'
);
const SPEC009_NOT_HELD_MD = path.join(
    path.resolve(__dirname, '..'),
    'sdd/initiatives/INIT-012-lesson-type-coverage/specs/completion-summaries/SPEC-009-not-held.md'
);

const INIT012_TEST_FILES = new Set([
    'test/plan-exercise-schema.test.js',
    'test/plan-schema-version-1-2.test.js',
    'test/plan-assignment-schema.test.js',
    'test/plan-future-tense.test.js',
    'test/plan-render-compare.test.js',
    'test/plan-render-exercise.test.js',
    'test/plan-render-exercise-typed.test.js',
    'test/plan-render-assignment.test.js',
    'test/plan-editor-exercise.test.js',
    'test/plan-editor-assignment.test.js',
    'test/skill-recipes.test.js',
    'test/plan-lesson-types-matrix.test.js',
    'test/plan-lesson-types-mcp.test.js',
    'test/rollout-preflight.test.js',
    'test/restyle-gate.test.js',
    'test/plan-revisions-migration.test.js',
    'test/plan-editor-drift.test.js',
    'test/activity-menu-schema.test.js'
]);

const LESSON_BLOCK_SURFACES = [
    'introduction',
    'graded-story',
    'short-dialogue',
    'personal-question',
    'verse-reading',
    'explainer',
    'spaced-review-checkpoint'
];
const FUTURE_PERSON_CELLS = ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'];
const HEBREW_LETTER = /[\u0590-\u05FF]/;
const LATIN_IN_CELL = /[A-Za-z]/;

function parseSpec009FormsTable(markdown) {
    const rows = Object.create(null);
    for (const line of markdown.split(/\r?\n/)) {
        if (!line.startsWith('|') || line.includes('---') || line.includes('Cell |')) continue;
        const parts = line.split('|').map((s) => s.trim()).filter(Boolean);
        if (parts.length < 6) continue;
        const cell = parts[0];
        if (!FUTURE_PERSON_CELLS.includes(cell)) continue;
        rows[cell] = {
            futurePointed: parts[1],
            futureKey: parts[2],
            pastPointed: parts[3],
            pastKey: parts[4],
            verified: parts[5]
        };
    }
    return rows;
}

function assertRenders(block, lesson) {
    const html = Render.renderBlock(block, { lesson, plan: { language: { target: 'he', gloss: 'en' } } });
    assert.ok(typeof html === 'string' && html.trim().length > 0, block.type + '/' + block.id + ' rendered empty');
    if (block.type === 'exercise' || block.type === 'assignment') {
        assert.match(html, /plan-exercise|plan-assignment/);
    }
}

function validateLessonFile(name) {
    const lesson = fx.readJson(name);
    const plan = fx.planFromLessons('matrix-' + lesson.id, [lesson]);
    const result = validatePlan(plan, { lexicon: fx.lexiconRepo() });
    assert.equal(result.ok, true, name + ': ' + JSON.stringify(result.blocks));
    assert.equal(plan.schemaVersion, '1.2.0');
    for (const block of lesson.blocks) {
        assertRenders(block, lesson);
    }
    return lesson;
}

describe('INIT-012/SPEC-009: lesson-type matrix', () => {
    it('ac-1 ac-4: initiative §4 types map to fixtures; match is noted as dependent', () => {
        const map = fx.TYPE_FILES;
        for (const key of INITIATIVE_TYPES) {
            assert.ok(map[key], 'missing fixture map for ' + key);
            assert.ok(fs.existsSync(path.join(fx.DIR, map[key])), map[key]);
        }
        assert.equal(MATCH_NOTE.includes('INIT-010 SPEC-004'), true);
        for (const name of [
            'type-a-topic-conversation.json',
            'type-b-review-worksheet.json',
            'type-c-verse-reading.json',
            'type-d-verb-workshop.json',
            'type-e-short-story.json'
        ]) {
            validateLessonFile(name);
        }
    });

    it('ac-1: combined five-lesson 1.2.0 plan validates with zero blocking findings', () => {
        const plan = fx.combinedFiveLessonPlan();
        const result = validatePlan(plan, { lexicon: fx.lexiconRepo() });
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
        assert.equal(plan.schemaVersion, '1.2.0');
        assert.equal(plan.units[0].lessons.length, 5);
        for (const lesson of plan.units[0].lessons) {
            for (const block of lesson.blocks) {
                assertRenders(block, lesson);
            }
        }
    });

    it('ac-4: matrix guard fails when a lesson type has no fixture entry', () => {
        const broken = Object.assign({}, fx.TYPE_FILES);
        delete broken.E;
        assert.throws(() => {
            for (const key of INITIATIVE_TYPES) {
                if (!broken[key]) throw new Error('missing fixture for ' + key);
            }
        }, /missing fixture for E/);
    });

    it('ac-4: matrix guard fails when a registry block type has no renderer', () => {
        const fake = 'not-a-real-block-type';
        assert.equal(typeof Render.renderBlock({ id: 'x', type: fake }), 'string');
        assert.equal(Render.renderBlock({ id: 'x', type: fake }), '');
        for (const type of REGISTRY_TYPES) {
            assert.ok(Render.blocks[type] && typeof Render.blocks[type].render === 'function', type);
        }
        for (const type of INLINE_RENDER_TYPES) {
            const sample = {
                introduction: { id: 'i', type, words: [{ id: '1', word: 'שָׁלוֹם', meaning: 'peace' }] },
                'graded-story': { id: 'g', type, paragraphs: [{ id: 'p', text: 'שָׁלוֹם' }] },
                listening: { id: 'l', type, passage: 'שָׁלוֹם', questions: [] },
                'short-dialogue': { id: 'd', type, source: 'authored', lines: [{ id: '1', pointed: 'שָׁלוֹם' }] },
                'fluency-pass': { id: 'f', type, seconds: 10, source: 'vocab', cues: ['a'] },
                'spaced-review-checkpoint': { id: 'c', type, wordKeys: [], fromLessons: [] }
            }[type] || { id: 'z', type };
            assert.ok(Render.renderBlock(sample, {}).length > 0, type);
        }
    });

    it('verification edge: one-block lesson and twelve exercise items validate and render', () => {
        validateLessonFile('edge-one-block.json');
        validateLessonFile('edge-twelve-exercise.json');
    });

    function planShell(block) {
        return {
            schemaVersion: '1.2.0',
            id: 'xss-plan',
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'XSS',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'u1', title: 'U', theme: 't',
                lessons: [{
                    id: 'l1', title: 'L', level: 'basic', visibility: 'hidden', recipe: 'review-worksheet',
                    buildsOn: [], newWords: [], recycles: [], blocks: [block]
                }]
            }]
        };
    }

    function assertHtmlEscapesXss(html, label) {
        assert.equal(/<img\b/i.test(html), false, label + ' raw img');
        assert.ok(html.includes('&lt;img'), label + ' escaped lt;img');
        assert.equal(/<\w[^>]*\sonerror\s*=/i.test(html), false, label + ' live onerror attr');
    }

    function assertFormShowsXss(formEl, xss, label) {
        const fields = dom.collect(formEl, (n) => n.tagName === 'INPUT' || n.tagName === 'TEXTAREA');
        const valueHit = fields.some((n) => n.value === xss);
        const textHit = String(formEl.textContent || '').includes(xss);
        assert.ok(valueHit || textHit, (label || 'form') + ' shows XSS as text/value');
        assert.equal(/<img\b/i.test(formEl.innerHTML || ''), false, 'form innerHTML img tag');
        assert.equal(/<[^>]+\sonerror\s*=/i.test(formEl.innerHTML || ''), false, 'form live onerror attr');
    }

    function exerciseBlockWithXss(kind, xss) {
        const block = Form.newBlock('exercise', 'ex-' + kind, SCHEMA);
        block.kind = kind;
        const item = Form.freshExerciseItem(kind, TOOLS, 'item-x');
        if (kind === 'choose') {
            item.prompt = xss;
            item.choices = ['א', 'ב'];
            item.answer = 'א';
        } else if (kind === 'true-false') {
            item.statement = xss;
            item.answer = true;
        } else if (kind === 'unscramble') {
            item.english = xss;
            item.words = ['ש', 'ל', 'ו', 'ם'];
        } else if (kind === 'translate') {
            item.english = xss;
            item.answer = 'שלום';
            item.accept = ['שלום'];
        } else if (kind === 'dictation') {
            block.heading = xss;
            item.audioPath = '../_audio/words/1111111111111111111111aa.mp3';
            item.answer = 'א';
            item.accept = ['א'];
        }
        block.items = [item];
        return block;
    }

    function lessonBlockWithXss(type, xss) {
        const id = 'x-' + type.replace(/[^a-z0-9]+/g, '-');
        switch (type) {
            case 'introduction':
                return {
                    id,
                    type,
                    words: [{
                        id: 'w1',
                        word: 'שָׁלוֹם',
                        meaning: xss,
                        translit: 'shalom',
                        group: 1,
                        sentence: 'שָׁלוֹם.'
                    }],
                    pattern: {
                        heading: xss,
                        rules: [{ title: 't', english: xss, example: 'שָׁלוֹם' }]
                    }
                };
            case 'graded-story':
                return {
                    id,
                    type,
                    paragraphs: [{ id: 'p1', text: 'שָׁלוֹם' }],
                    support: [{ id: 's1', he: 'שָׁלוֹם', en: xss }],
                    glosses: {},
                    questions: [{ id: 'q1', question: xss }]
                };
            case 'short-dialogue':
                return {
                    id,
                    type,
                    source: 'authored',
                    lines: [{ id: 'l1', speaker: 'learner', pointed: xss }]
                };
            case 'personal-question':
                return {
                    id,
                    type,
                    prompts: [{ id: 'p1', he: 'שָׁלוֹם?', en: xss }],
                    answer: 'either'
                };
            case 'verse-reading':
                return {
                    id,
                    type,
                    ref: { book: 'Genesis', chapter: 1, from: 1, to: 1, source: xss },
                    newWords: ['בְּרֵאשִׁית'],
                    verse: 'בְּרֵאשִׁית',
                    questions: [{ id: 'q1', he: 'מָה?', en: xss }],
                    personalQuestion: { he: 'מָה?', en: xss }
                };
            case 'explainer':
                return {
                    id,
                    type,
                    title: xss,
                    sections: [{ id: 's1', heading: xss, paragraphs: [xss] }]
                };
            case 'spaced-review-checkpoint':
                return { id, type, wordKeys: ['שָׁלוֹם'], fromLessons: [xss] };
            default:
                throw new Error('unhandled lesson block surface: ' + type);
        }
    }

    function parseSpec009Ac11Baseline(markdown) {
        const heading = '## Full node suite baseline (ac-11)';
        const start = markdown.indexOf(heading);
        assert.notEqual(start, -1, 'missing ac-11 baseline section');
        const section = markdown.slice(start);
        assert.match(section, /\*\*51\*\*.*pre-SPEC-001|\*\*43\*\* fail/);
        assert.match(section, /\*\*43\*\* fail/);
        assert.match(section, /\*\*51\*\*/);
        assert.match(section, /0\*\* new failures attributable to `INIT-012\/SPEC-009`/);
        const named = [];
        const block = section.match(/Failing test names[\s\S]*/);
        assert.ok(block, 'missing failing test names list');
        for (const line of block[0].split(/\r?\n/)) {
            const m = line.match(/^\d+\.\s+`([^`]+)`\s+—\s+`(test\/[^`]+\.test\.js):\d+`/);
            if (m) named.push({ name: m[1], file: m[2] });
        }
        assert.equal(named.length, 43, 'expected 43 named pre-existing failures');
        return named;
    }

    function renderEditorSurfaces(block) {
        const plan = planShell(block);
        const lesson = plan.units[0].lessons[0];
        const blockPath = LESSON + '/blocks/' + block.id;
        const env = makeEditor({ getPlan: () => plan });
        const form = env.editor.renderNode({ path: blockPath, node: block, kind: 'block', plan });
        env.host.appendChild(form.el);
        const prev = Preview.compute({
            render: Render,
            lesson,
            lessonPath: LESSON,
            block,
            blockPath,
            handles: []
        });
        const playerHtml = Render.renderBlock(block, { lesson, plan });
        return { form, prev, playerHtml };
    }

    it('ac-12: SPEC-009 forms-to-check has nine future rows, verified 0, pure Hebrew', () => {
        const markdown = fs.readFileSync(SPEC009_FORMS_MD, 'utf8');
        const rows = parseSpec009FormsTable(markdown);
        assert.deepEqual(Object.keys(rows).sort(), FUTURE_PERSON_CELLS.slice().sort());
        for (const cell of FUTURE_PERSON_CELLS) {
            const row = rows[cell];
            assert.ok(row, cell);
            assert.equal(row.verified, '0', cell + ' verified');
            for (const col of ['futurePointed', 'pastPointed']) {
                assert.ok(HEBREW_LETTER.test(row[col]), cell + ' ' + col + ' missing Hebrew');
                assert.equal(LATIN_IN_CELL.test(row[col]), false, cell + ' ' + col + ' has Latin');
            }
        }
    });

    it('ac-8: XSS payload renders as text on exercise and assignment surfaces', () => {
        const xss = fx.XSS;
        const lesson = { id: 'xss-lesson', level: 'basic', visibility: 'everyone' };

        for (const kind of ['choose', 'true-false', 'unscramble', 'translate', 'dictation']) {
            const block = exerciseBlockWithXss(kind, xss);
            const { form, prev, playerHtml } = renderEditorSurfaces(block);
            assertFormShowsXss(form.el, xss, kind);
            assertHtmlEscapesXss(prev.html || '', 'preview ' + kind);
            assertHtmlEscapesXss(playerHtml, 'player ' + kind);
        }

        const assign = {
            id: 'x-as',
            type: 'assignment',
            prompt: xss,
            minSentences: 1,
            homework: [xss]
        };
        const assignSurfaces = renderEditorSurfaces(assign);
        assertFormShowsXss(assignSurfaces.form.el, xss);
        assertHtmlEscapesXss(assignSurfaces.prev.html || '', 'preview assignment');
        assertHtmlEscapesXss(assignSurfaces.playerHtml, 'player assignment');

        const conj = {
            id: 'x-ct',
            type: 'conjugation-table',
            lemmaKey: 'לדבר',
            tense: 'future',
            mode: 'study',
            cells: [{ cell: '1s', formKey: 'אדבר' }],
            sentences: [{ id: 's1', he: xss }]
        };
        const conjSurfaces = renderEditorSurfaces(conj);
        assertFormShowsXss(conjSurfaces.form.el, xss);
        assertHtmlEscapesXss(conjSurfaces.prev.html || '', 'preview conjugation-table');
        assertHtmlEscapesXss(Render.renderBlock(conj, { lesson }), 'player conjugation-table');

        for (const type of LESSON_BLOCK_SURFACES) {
            const block = lessonBlockWithXss(type, xss);
            const { form, prev, playerHtml } = renderEditorSurfaces(block);
            assertFormShowsXss(form.el, xss, type);
            assertHtmlEscapesXss(prev.html || '', 'preview ' + type);
            assertHtmlEscapesXss(playerHtml, 'player ' + type);
        }
    });

    it('ac-11: every INIT-012 and touched test file passes, and pre-existing failures are named in SPEC-009-not-held.md', () => {
        const markdown = fs.readFileSync(SPEC009_NOT_HELD_MD, 'utf8');
        const named = parseSpec009Ac11Baseline(markdown);
        for (const row of named) {
            assert.equal(
                INIT012_TEST_FILES.has(row.file),
                false,
                'pre-existing failure must not be an INIT-012 test file: ' + row.file + ' — ' + row.name
            );
        }
    });
});
