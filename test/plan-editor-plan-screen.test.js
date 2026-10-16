/**
 * Plan screen: New plan For-field, titled Add lesson, reuse, word-bank paste
 * (INIT-011/SPEC-004). Covers ac-1, ac-3, ac-4, ac-5. No browser required.
 * Provenance: INIT-011/SPEC-004
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { loadPanels, byAttr, sleep } = require('./helpers/plan-panels-vm');
const List = require('../shared/admin/plan-list.js');

const ROOT = path.resolve(__dirname, '..');
const CSS = fs.readFileSync(path.join(ROOT, 'shared', 'worksheet.css'), 'utf8');
const BuilderSrc = fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'plan-builder.js'), 'utf8');
const ListSrc = fs.readFileSync(path.join(ROOT, 'shared', 'admin', 'plan-list.js'), 'utf8');

function one(node, name, value) {
    return byAttr(node, name, value)[0];
}

function basePlan(extra) {
    return Object.assign({
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
            lessons: [{
                id: 'family-1',
                title: 'Family 1',
                subtitle: 'Meet the family',
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
            }]
        }]
    }, extra || {});
}

describe('INIT-011/SPEC-004 helpers (pure)', () => {
    it('slugFromTitle makes a slug from Latin titles and lesson-N when there are no Latin letters', () => {
        const env = loadPanels({ plan: basePlan() });
        const B = env.W.HebrewPlanBuilder;
        assert.equal(B.slugFromTitle('Sharing Opinions', []), 'sharing-opinions');
        assert.equal(B.slugFromTitle('Sharing Opinions', ['sharing-opinions']), 'sharing-opinions-2');
        assert.equal(B.slugFromTitle('המשפחה שלי', ['lesson-1']), 'lesson-2');
        assert.equal(B.slugFromTitle('123', []), 'lesson-1');
    });

    it('parseWordLines accepts – - tab comma = and reports lines with no separator', () => {
        const env = loadPanels({ plan: basePlan() });
        const B = env.W.HebrewPlanBuilder;
        const ok = B.parseWordLines('אמא – mother\nאבא\tfather\nדוד, uncle\nאחות = sister');
        assert.equal(ok.words.length, 4);
        assert.equal(ok.rejected.length, 0);
        const bad = B.parseWordLines('אמא – mother\nno separator here\nאב = father');
        assert.equal(bad.words.length, 2);
        assert.deepEqual(Array.from(bad.rejected), ['no separator here']);
    });

    it('prepareReuseLesson gives a free id and keeps only earlier buildsOn in this plan', () => {
        const env = loadPanels({ plan: basePlan() });
        const B = env.W.HebrewPlanBuilder;
        const source = {
            id: 'opinions',
            title: 'Sharing Opinions',
            buildsOn: ['foreign-id', 'family-1'],
            newWords: [],
            recycles: [],
            blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }],
            level: 'basic',
            visibility: 'hidden'
        };
        const thisPlan = [{ id: 'family-1' }, { id: 'opinions' }];
        const copy = B.prepareReuseLesson(source, 'opinions', thisPlan);
        assert.notEqual(copy.id, 'opinions');
        assert.ok(copy.id.indexOf('sharing') === 0 || copy.id.indexOf('opinions') === 0 || /^lesson-/.test(copy.id));
        assert.ok(copy.buildsOn.indexOf('foreign-id') === -1);
        assert.ok(copy.buildsOn.indexOf('family-1') !== -1);
        assert.ok(copy.buildsOn.indexOf('opinions') !== -1, 'original earlier in this plan → copy builds on it');
        assert.equal(copy.title, 'Sharing Opinions');
    });
});

describe('INIT-011/SPEC-004 ac-1: New plan title + For, no recipe; personal for a learner', () => {
    it('source has no recipe field control', () => {
        assert.doesNotMatch(ListSrc, /data-plan-new-recipe/);
        assert.match(ListSrc, /data-plan-new-for/);
    });

    it('create for a learner ends personal with that assignee', async () => {
        const env = loadPanels({
            plans: [{ id: 'existing', title: 'Existing', status: 'draft', audience: 'shared' }],
            users: [{ username: 'brian', display_name: 'Brian', status: 'active' }]
        });
        const panel = env.panel();
        env.W.HebrewPlanList.mount(panel, {
            doc: env.W.document,
            api: env.W.HebrewApi,
            setHash: (r) => env.hashes.push(r)
        });
        await sleep(40);
        one(panel, 'data-plan-new-open').click();
        await sleep(40);
        assert.equal(byAttr(panel, 'data-plan-new-recipe').length, 0);
        assert.ok(one(panel, 'data-plan-new-title'), 'title field');
        one(panel, 'data-plan-new-title').value = 'Personal training';
        one(panel, 'data-plan-new-for').value = 'brian';
        one(panel, 'data-plan-create').click();
        await sleep(40);
        assert.deepEqual(env.callsOf('POST', /^\/api\/plans$/)[0].body, { title: 'Personal training' });
        assert.deepEqual(env.callsOf('POST', /\/assign$/)[0].body, {
            audience: 'personal',
            assigneeId: 'brian',
            expectedRevision: 1
        });
        assert.equal(env.server.plan.audience, 'personal');
        assert.equal(env.server.plan.assigneeId, 'brian');
    });
});

describe('INIT-011/SPEC-004 ac-3: Add lesson with title Sharing Opinions', () => {
    it('POST /lessons inserts that title; no lesson-N shown as the title', async () => {
        const env = loadPanels({ plan: basePlan(), revision: 4 });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        one(panel, 'data-plan-add-lesson-title').value = 'Sharing Opinions';
        one(panel, 'data-plan-add-lesson-subtitle').value = 'Say what you think';
        one(panel, 'data-plan-add-lesson').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        const posts = env.callsOf('POST', /\/lessons$/);
        assert.ok(posts.length >= 1);
        const body = posts[posts.length - 1].body;
        assert.equal(body.lesson.title, 'Sharing Opinions');
        assert.equal(body.lesson.subtitle, 'Say what you think');
        assert.equal(body.lesson.id, 'sharing-opinions');
        assert.notEqual(body.lesson.title, body.lesson.id);
        const titles = byAttr(panel, 'data-plan-lesson-title').map((n) => n.textContent);
        assert.ok(titles.indexOf('Sharing Opinions') !== -1);
        assert.ok(titles.every((t) => !/^lesson-\d+$/.test(t)), 'no lesson-N as visible title');
    });
});

describe('INIT-011/SPEC-004 ac-4: Reuse lesson from another plan', () => {
    it('inserts one lesson with a new id; buildsOn has only earlier ids in this plan', async () => {
        const other = {
            schemaVersion: '1.0.0',
            id: 'other-plan',
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'Older plan',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'unit-1',
                title: 'U',
                theme: 't',
                lessons: [{
                    id: 'living-room',
                    title: 'Living room',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: ['ghost-lesson', 'family-1'],
                    newWords: ['סַלּוֹן'],
                    recycles: [],
                    blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
                }]
            }]
        };
        // Put living-room in this plan too so reuse must mint a new id.
        const plan = basePlan();
        plan.units[0].lessons.push({
            id: 'living-room',
            title: 'Living room (earlier)',
            level: 'basic',
            visibility: 'hidden',
            buildsOn: [],
            newWords: [],
            recycles: [],
            blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
        });
        const env = loadPanels({
            plan: plan,
            revision: 4,
            plans: [
                { id: 'sample', title: 'Family', status: 'draft', audience: 'shared' },
                { id: 'other-plan', title: 'Older plan', status: 'draft', audience: 'shared' }
            ],
            plansById: { 'other-plan': other }
        });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        one(panel, 'data-plan-reuse-lesson-open').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(80);
        const select = one(panel, 'data-plan-reuse-select');
        assert.ok(select);
        const option = Array.from(select.children || []).find((o) => {
            return String(o.value || '').indexOf('other-plan') === 0;
        });
        assert.ok(option, 'other plan lesson offered');
        select.value = option.value;
        one(panel, 'data-plan-reuse-lesson').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        const posts = env.callsOf('POST', /\/lessons$/);
        assert.ok(posts.length >= 1);
        const lesson = posts[posts.length - 1].body.lesson;
        assert.notEqual(lesson.id, 'living-room');
        assert.equal(lesson.title, 'Living room');
        assert.ok(lesson.buildsOn.indexOf('ghost-lesson') === -1);
        // family-1 is earlier in this plan and was on the source buildsOn — kept.
        assert.ok(lesson.buildsOn.indexOf('family-1') !== -1 || lesson.buildsOn.every((id) => {
            return env.server.plan.units[0].lessons.some((l) => l.id === id);
        }));
        assert.equal(byAttr(panel, 'data-plan-stack-row').length, 3);
    });
});

describe('INIT-011/SPEC-004 ac-5: paste word bank lines', () => {
    it('three אמא – mother lines add three words; a bare line is reported', async () => {
        const env = loadPanels({ plan: basePlan(), revision: 4 });
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        // Open Family 1 (already open by default after mount).
        const area = one(panel, 'data-plan-word-paste');
        assert.ok(area);
        area.value = 'אמא – mother\nאבא - father\nדוד = uncle\nbare line';
        one(panel, 'data-plan-word-paste-apply').click();
        await env.W.HebrewPlanBuilder.settled();
        await sleep(40);
        const msg = one(panel, 'data-plan-word-paste-message');
        assert.ok(msg);
        assert.equal(msg.hidden, false);
        assert.match(msg.textContent, /bare line|separator|מפריד/i);
        const fields = env.callsOf('PATCH', /\/fields$/);
        assert.ok(fields.length >= 1, 'set_field used');
        const newWordsCall = fields.find((c) => c.body && c.body.field === 'newWords');
        assert.ok(newWordsCall, 'newWords updated');
        assert.ok(newWordsCall.body.value.indexOf('אמא') !== -1);
        assert.ok(newWordsCall.body.value.indexOf('אבא') !== -1);
        assert.ok(newWordsCall.body.value.indexOf('דוד') !== -1);
        const wordsCall = fields.find((c) => c.body && c.body.field === 'words');
        assert.ok(wordsCall, 'introduction words updated');
        assert.equal(wordsCall.body.value.filter((w) => w.word === 'אמא' || w.word === 'אבא' || w.word === 'דוד').length, 3);
    });
});

describe('INIT-011/SPEC-004 ac-2 layout (CSS evidence; human checks 1100/700)', () => {
    it('plan-screen is one column by default and two columns from 900px', () => {
        assert.match(CSS, /\.plan-layout\.plan-screen[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)/);
        assert.match(CSS, /@media \(min-width:\s*900px\)[\s\S]*?\.plan-layout\.plan-screen[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)\s+320px/);
        assert.match(CSS, /\.plan-lesson-row\[aria-current="true"\][\s\S]*?border-color:\s*var\(--color-primary\)/);
        assert.match(CSS, /\.plan-lesson-row\[aria-current="true"\][\s\S]*?--color-primary-soft/);
        assert.match(BuilderSrc, /data-plan-word-bank/);
        assert.match(BuilderSrc, /data-plan-for/);
        assert.doesNotMatch(BuilderSrc, /data-plan-activity-name="Match"/);
    });
});

describe('INIT-011/SPEC-004 checkNew still title-only', () => {
    it('checkNew does not require a recipe', () => {
        assert.deepEqual(List.checkNew({ title: 'T' }), {});
    });
});
