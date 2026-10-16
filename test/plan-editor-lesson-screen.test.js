/**
 * Lesson screen: activities by name, no placeholder, Words cloze, one-language
 * hints, Story save without Id/Index (INIT-011/SPEC-005). Covers ac-2–ac-5.
 * Provenance: INIT-011/SPEC-005
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { loadPanels, byAttr, sleep } = require('./helpers/plan-panels-vm');

const ROOT = path.resolve(__dirname, '..');
const CSS = fs.readFileSync(path.join(ROOT, 'shared', 'worksheet.css'), 'utf8');
const BuilderSrc = fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'plan-builder.js'), 'utf8');
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

function freshLessonPlan(extra) {
    return Object.assign({
        schemaVersion: '1.0.0',
        id: 'sample',
        revision: 4,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Likes',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-1',
            title: 'Likes',
            theme: 'likes',
            lessons: [{
                id: 'likes-1',
                title: 'אוהב לא אוהב',
                subtitle: 'Worksheet 02 Ex. 1',
                level: 'basic',
                visibility: 'hidden',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
            }]
        }]
    }, extra || {});
}

async function openLesson(env, panel) {
    await env.W.HebrewPlanBuilder.mount(panel, 'sample');
    await env.W.HebrewPlanBuilder.settled();
    await sleep(40);
    const open = one(panel, 'data-plan-lesson');
    assert.ok(open, 'Open lesson');
    open.click();
    await env.W.HebrewPlanBuilder.settled();
    await sleep(40);
    assert.ok(one(panel, 'data-plan-lesson-screen'), 'lesson screen');
}

describe('INIT-011/SPEC-005 ac-1: lesson layout CSS', () => {
    it('at 1100px three columns; below 900px the lesson screen stacks', () => {
        assert.match(CSS, /plan-layout\.plan-lesson-screen/);
        assert.match(
            CSS,
            /@media \(min-width: 1100px\)[\s\S]*plan-lesson-screen[\s\S]*grid-template-columns:\s*minmax\(12rem,\s*18rem\)\s+minmax\(0,\s*1fr\)\s+minmax\(16rem,\s*24rem\)/
        );
        assert.match(
            CSS,
            /plan-layout\.plan-lesson-screen \{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)/
        );
        assert.doesNotMatch(BuilderSrc, /data-plan-new-id/);
        assert.doesNotMatch(BuilderSrc, /data-plan-new-index/);
    });
});

describe('INIT-011/SPEC-005 ac-2: Story saves without Id or Index', () => {
    it('adding Story with one paragraph and one question POSTs /blocks; no Id or Index label', async () => {
        const env = loadPanels({ plan: freshLessonPlan(), revision: 4 });
        const panel = env.panel();
        await openLesson(env, panel);

        assert.equal(byAttr(panel, 'data-plan-new-id').length, 0);
        assert.equal(byAttr(panel, 'data-plan-new-index').length, 0);
        const labels = [];
        const walk = (n) => {
            if (n && n.tagName === 'LABEL') labels.push(String(n.textContent || ''));
            (n.children || []).forEach(walk);
        };
        walk(panel);
        labels.forEach((text) => {
            assert.doesNotMatch(text, /^\s*Id\s*$/i);
            assert.doesNotMatch(text, /^\s*Index\s*$/i);
            assert.doesNotMatch(text, /מזהה \(Id\)/);
            assert.doesNotMatch(text, /מקום \(Index\)/);
        });

        const typeSelect = one(panel, 'data-plan-new-type');
        const storyOpt = Array.from(typeSelect.children || []).find((o) => {
            return o.getAttribute && o.getAttribute('data-plan-activity-name') === 'Story';
        });
        assert.ok(storyOpt, 'Story on the menu');
        typeSelect.value = storyOpt.value;
        typeSelect.dispatch('change');
        await sleep(20);

        assert.ok(one(panel, 'data-plan-new-draft'), 'Story form open');
        const draft = env.W.HebrewPlanBuilder.draft();
        assert.ok(draft && draft.handle, 'staged draft');
        draft.handle.check = () => null;
        draft.handle.getValue = () => ({
            paragraphs: [{ id: 'p1', text: 'אני אוהב קפה.' }],
            questions: [{ id: 'q1', question: 'מה אתה אוהב?' }]
        });

        one(panel, 'data-plan-insert').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(60);

        const posts = env.callsOf('POST', /\/blocks$/);
        assert.ok(posts.length >= 1, 'POST /api/plans/:id/blocks');
        const body = posts[0].body;
        assert.equal(body.path, 'units/unit-1/lessons/likes-1');
        assert.equal(body.block.type, 'graded-story');
        assert.equal(body.block.paragraphs[0].text, 'אני אוהב קפה.');
        assert.equal(body.block.questions[0].question, 'מה אתה אוהב?');
        assert.ok(typeof body.block.id === 'string' && body.block.id);
        assert.ok(Number.isInteger(body.index));
        assert.equal(Object.prototype.hasOwnProperty.call(body, 'expectedRevision'), true);
    });
});

describe('INIT-011/SPEC-005 ac-3: placeholder never listed; removed after first save', () => {
    it('a new lesson activity list is empty; after first activity the placeholder is gone', async () => {
        const env = loadPanels({ plan: freshLessonPlan(), revision: 4 });
        const panel = env.panel();
        await openLesson(env, panel);

        const listed = byAttr(panel, 'data-plan-block');
        assert.equal(listed.length, 0, 'placeholder is not listed');
        assert.equal(byAttr(panel, 'data-plan-block', 'placeholder').length, 0);

        // Pure helper mirrors the paint filter (fails if placeholder were listed).
        const B = env.W.HebrewPlanBuilder;
        const lesson = env.server.plan.units[0].lessons[0];
        assert.deepEqual(
            B.listedBlocks(lesson).map((b) => b.id),
            [],
            'listedBlocks skips placeholder'
        );

        const typeSelect = one(panel, 'data-plan-new-type');
        const storyOpt = Array.from(typeSelect.children || []).find((o) => {
            return o.getAttribute && o.getAttribute('data-plan-activity-name') === 'Story';
        });
        typeSelect.value = storyOpt.value;
        typeSelect.dispatch('change');
        await sleep(20);
        const draft = env.W.HebrewPlanBuilder.draft();
        draft.handle.check = () => null;
        draft.handle.getValue = () => ({
            paragraphs: [{ id: 'p1', text: 'שלום.' }],
            questions: [{ id: 'q1', question: 'מה?' }]
        });
        one(panel, 'data-plan-insert').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(60);

        const after = env.server.plan.units[0].lessons[0].blocks;
        assert.equal(after.some((b) => b && b.id === 'placeholder'), false, 'placeholder removed');
        assert.ok(after.some((b) => b && b.type === 'graded-story'), 'story kept');
        const removes = env.callsOf('DELETE', /\/parts$/);
        assert.ok(
            removes.some((c) => c.body && c.body.path && String(c.body.path).endsWith('/blocks/placeholder')),
            'remove_block on placeholder'
        );
    });

    it('removing the only real activity is not sent', async () => {
        const plan = freshLessonPlan();
        plan.units[0].lessons[0].blocks = [{
            id: 'story-1',
            type: 'graded-story',
            paragraphs: [{ id: 'p1', text: 'שלום.' }],
            questions: [{ id: 'q1', question: 'מה?' }]
        }];
        const env = loadPanels({ plan: plan, revision: 4 });
        const panel = env.panel();
        await openLesson(env, panel);
        assert.equal(byAttr(panel, 'data-plan-block').length, 1);
        one(panel, 'data-plan-remove').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        assert.equal(env.callsOf('DELETE', /\/parts$/).length, 0, 'remove not sent');
        const err = one(panel, 'data-plan-error');
        assert.ok(err && err.hidden === false);
        assert.match(err.textContent, /at least one activity/i);
    });
});

describe('INIT-011/SPEC-005 ac-4: Words with two cloze items from worksheet 02 Ex. 1', () => {
    it('saves Words with two fill-in-the-blank items and shows a preview', async () => {
        // Worksheet 02 אוהב לא אוהב Ex. 1: word bank then complete the sentences.
        const cloze = {
            bank: ['אוהב', 'אוהבת', 'לא אוהב', 'לא אוהבת'],
            items: [
                { id: 'c1', textPre: 'אני', textPost: 'קפה.', answer: 'אוהב' },
                { id: 'c2', textPre: 'היא', textPost: 'תה.', answer: 'אוהבת' }
            ]
        };
        const env = loadPanels({ plan: freshLessonPlan(), revision: 4 });
        const panel = env.panel();
        await openLesson(env, panel);

        const typeSelect = one(panel, 'data-plan-new-type');
        const wordsOpt = Array.from(typeSelect.children || []).find((o) => {
            return o.getAttribute && o.getAttribute('data-plan-activity-name') === 'Words';
        });
        assert.ok(wordsOpt);
        typeSelect.value = wordsOpt.value;
        typeSelect.dispatch('change');
        await sleep(20);
        assert.ok(one(panel, 'data-plan-new-draft'), 'Words form includes fields');
        assert.match(BuilderSrc, /fields\.push\('cloze'\)/);

        const draft = env.W.HebrewPlanBuilder.draft();
        draft.handle.check = () => null;
        draft.handle.getValue = () => ({
            words: [
                { id: 'w1', word: 'אוהב', meaning: 'like (m)', sentence: 'אני אוהב קפה.' },
                { id: 'w2', word: 'אוהבת', meaning: 'like (f)', sentence: 'היא אוהבת תה.' }
            ],
            cloze: cloze
        });

        one(panel, 'data-plan-insert').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(60);

        const posts = env.callsOf('POST', /\/blocks$/);
        assert.ok(posts.length >= 1);
        assert.equal(posts[0].body.block.type, 'introduction');
        assert.equal(posts[0].body.block.cloze.items.length, 2);
        const saved = env.server.plan.units[0].lessons[0].blocks.find((b) => b.type === 'introduction');
        assert.ok(saved);
        assert.ok(saved.cloze && saved.cloze.items && saved.cloze.items.length === 2);
        assert.equal(saved.cloze.items[0].answer, 'אוהב');
        assert.equal(saved.cloze.items[1].answer, 'אוהבת');

        const preview = one(panel, 'data-plan-preview');
        assert.ok(preview, 'preview panel');
        assert.ok(
            preview.innerHTML || (preview.textContent && preview.textContent.length > 0),
            'preview shows the Words activity'
        );
    });
});

describe('INIT-011/SPEC-005 ac-5: activity names and hints are one language', () => {
    it('menu options and hints are .ui-pair or one language, never Hebrew+English in one textContent', async () => {
        const env = loadPanels({ plan: freshLessonPlan(), revision: 4 });
        const panel = env.panel();
        await openLesson(env, panel);

        const menu = one(panel, 'data-plan-activity-menu');
        assert.ok(menu);
        const options = byAttr(menu, 'data-plan-activity-name');
        assert.ok(options.length >= 5);
        assert.equal(
            options.some((o) => o.getAttribute('data-plan-activity-name') === 'Match'),
            false,
            'Match omitted until INIT-010 SPEC-004'
        );
        options.forEach((o) => {
            const en = o.getAttribute('data-ui-text-en') || '';
            const he = o.getAttribute('data-ui-text-he') || '';
            const text = String(o.textContent || '');
            assert.ok(text === en || text === he, 'option shows one language');
            assert.equal(text.includes(en) && text.includes(he) && en !== he, false);
        });

        const hint = one(panel, 'data-plan-activity-hint');
        assert.ok(hint);
        assert.match(hint.className, /ui-pair/);
        const parts = pairParts(hint);
        assert.ok(parts.he && parts.en);
        assert.match(parts.he.textContent, HEBREW);
        assert.doesNotMatch(parts.en.textContent, HEBREW);
        assert.match(parts.en.textContent, /Word bank|fill in the blank/i);
        assert.doesNotMatch(parts.he.textContent, /Word bank/i);

        const B = env.W.HebrewPlanBuilder;
        const wordsHint = B.activityHint('Words');
        assert.equal(wordsHint.en, 'Word bank, then fill in the blank');
        assert.ok(HEBREW.test(wordsHint.he));
        assert.doesNotMatch(wordsHint.he, /Word bank/);
    });
});
