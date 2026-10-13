/**
 * The issues panel, the one-click fixes, the live preview, and the publish line
 * (INIT-008/SPEC-015), on a fake DOM with a stand-in for the plan routes.
 * A browser run is test/chrome-plan-editor-smoke.js.
 * Provenance: INIT-008/SPEC-015
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { loadPanels, all, byAttr } = require('./helpers/plan-panels-vm');
const { samplePlan, LESSON } = require('./helpers/plan-editor-fixtures');

const Issues = require('../shared/admin/plan-issues.js');
const Preview = require('../shared/admin/plan-preview.js');
const Form = require('../shared/admin/plan-form.js');
const Render = require('../shared/ui/plan-render.js');

const LISTEN = LESSON + '/blocks/listening';
const INTRO = LESSON + '/blocks/introduction';
const WORD = INTRO + '/words/1';
const NFC_BAD = 'ת\u05BC\u05B8';
const text = (node) => node.textContent;
const one = (node, name, value) => byAttr(node, name, value)[0];

const ISSUES = {
    revision: 4,
    status: 'draft',
    counts: { block: 1, warn: 5, inform: 1 },
    truncated: false,
    issues: [
        { code: 'meaning', severity: 'block', hard: true, path: LESSON, field: 'newWords/0', pointer: '/units/0/lessons/0/newWords/0', message: 'no introduction word gives this new word a first-language meaning' },
        { code: 'lexicon-form', severity: 'warn', hard: false, path: WORD, field: 'word', pointer: '/units/0/lessons/0/blocks/0/words/0/word', message: 'differs from the lexicon form',
            fix: { action: 'set_field', path: WORD, field: 'word', suggested: 'בַּיִת' } },
        { code: 'lexicon-form', severity: 'warn', hard: false, path: INTRO, field: 'cloze/items/0/answer', pointer: '/units/0/lessons/0/blocks/0/cloze/items/0/answer', message: 'differs from the lexicon form',
            fix: { action: 'replace_part', path: INTRO, field: 'cloze/items/0/answer', suggested: 'בַּיִת' } },
        { code: 'pointing-form', severity: 'warn', hard: false, path: LISTEN, field: 'passage', pointer: '/units/0/lessons/0/blocks/2/passage', message: 'the marks are not in canonical order',
            fix: { action: 'set_field', path: LISTEN, field: 'passage', suggested: 'תָּ דָּנִי.' } },
        { code: 'pointing-consistency', severity: 'warn', hard: false, path: WORD, field: 'word', pointer: '/units/0/lessons/0/blocks/0/words/0/word', message: 'the word is pointed two ways in this lesson',
            fix: { action: 'set_field', path: WORD, field: 'word', candidates: ['בַּיִת', 'בַיִת'] } },
        { code: 'audio-stale', severity: 'warn', hard: false, path: WORD, field: 'sentence', pointer: '/units/0/lessons/0/blocks/0/words/0/sentence', message: 'this text changed since the last publish and its clip did not',
            fix: { action: 'replace_part', path: WORD, field: 'sentenceAudioPath', note: 'No tool generates audio for a plan field.' } },
        { code: 'N6', severity: 'inform', hard: false, path: LESSON, field: 'newWords', pointer: '/units/0/lessons/0/newWords', message: 'appears once' }
    ]
};

describe('issues: pure helpers', () => {
    it('groupIssues puts hard errors first, then warnings, then notes, by code in first-seen order', () => {
        const groups = Issues.groupIssues(ISSUES.issues);
        assert.deepEqual(groups.map((g) => g.severity), ['block', 'warn', 'inform']);
        assert.equal(groups[0].hard, true);
        assert.equal(groups[1].hard, false);
        assert.deepEqual(groups[1].codes.map((c) => c.code), ['lexicon-form', 'pointing-form', 'pointing-consistency', 'audio-stale']);
        assert.equal(groups[1].codes[0].issues.length, 2);
        // a severity with no issue has no group
        assert.deepEqual(Issues.groupIssues(ISSUES.issues.filter((i) => i.severity === 'warn')).map((g) => g.severity), ['warn']);
        assert.deepEqual(Issues.groupIssues([]), []);
        assert.deepEqual(Issues.groupIssues(null), []);
    });

    it('countsText gives both languages, and notes only when there are some', () => {
        const withNotes = Issues.countsText({ block: 1, warn: 5, inform: 2 });
        assert.match(withNotes.he, /1 שגיאות חוסמות · 5 אזהרות · 2 הערות/);
        assert.match(withNotes.en, /1 blocking · 5 warnings · 2 notes/);
        const none = Issues.countsText({ block: 0, warn: 0, inform: 0 });
        assert.doesNotMatch(none.en, /notes/);
        assert.match(Issues.countsText(null).text, /0 blocking/);
    });

    it('publishNote says warnings do not stop publishing and asks for no approval', () => {
        const note = Issues.publishNote({ block: 0, warn: 3 });
        assert.match(note, /3 אזהרות/);
        assert.match(note, /Warnings do not stop publishing/);
        assert.doesNotMatch(note, /approv|sign.?off|confirm/i);
        assert.doesNotMatch(note, /A blocking error stops/, 'no hard error, no such line');
        assert.match(Issues.publishNote({ block: 2, warn: 0 }), /A blocking error stops publishing/);
    });

    it('currentValue follows a nested field; fixRequest builds one set_field', () => {
        const plan = samplePlan();
        assert.equal(Issues.currentValue(plan, ISSUES.issues[1]), 'בַּיִת');
        assert.equal(Issues.currentValue(plan, ISSUES.issues[2]), 'בַּיִת', 'cloze answer of the sample');
        assert.equal(Issues.currentValue(plan, { path: WORD, field: 'nope' }), undefined);
        assert.equal(Issues.currentValue(plan, { path: 'units/nope', field: 'x' }), undefined);

        assert.deepEqual(Issues.fixRequest(ISSUES.issues[1], plan), { path: WORD, field: 'word', value: 'בַּיִת' });
        assert.deepEqual(Issues.fixRequest(ISSUES.issues[3], plan), { path: LISTEN, field: 'passage', value: 'תָּ דָּנִי.' });
        // a candidate is the value
        assert.deepEqual(Issues.fixRequest(ISSUES.issues[4], plan, 'בַיִת'), { path: WORD, field: 'word', value: 'בַיִת' });
        // no suggestion, no candidate choice, no fix: nothing to send
        assert.equal(Issues.fixRequest(ISSUES.issues[4], plan), null);
        assert.equal(Issues.fixRequest(ISSUES.issues[5], plan), null);
        assert.equal(Issues.fixRequest(ISSUES.issues[0], plan), null);
        assert.equal(Issues.fixRequest(null, plan), null);
    });

    it('a fix inside a field sends that one field whole, with the sub-value replaced and nothing else touched', () => {
        const plan = samplePlan();
        const before = JSON.stringify(plan);
        const request = Issues.fixRequest({
            code: 'lexicon-form', path: INTRO, field: 'cloze/items/0/answer',
            fix: { action: 'replace_part', path: INTRO, field: 'cloze/items/0/answer', suggested: 'חדש' }
        }, plan);
        assert.equal(request.path, INTRO);
        assert.equal(request.field, 'cloze');
        assert.equal(request.value.items[0].answer, 'חדש');
        assert.equal(request.value.items[0].textPre, plan.units[0].lessons[0].blocks[0].cloze.items[0].textPre);
        assert.equal(JSON.stringify(plan), before, 'the working copy in the page is not changed');
        // a list element
        const list = Issues.fixRequest({ path: LESSON, field: 'newWords/0', fix: { path: LESSON, field: 'newWords/0', suggested: 'מילה' } }, plan);
        assert.deepEqual(list, { path: LESSON, field: 'newWords', value: ['מילה'] });
        // a place that is not there: nothing is sent
        assert.equal(Issues.fixRequest({ path: INTRO, field: 'cloze/items/9/answer', fix: { suggested: 'x' } }, plan), null);
        assert.equal(Issues.fixRequest({ path: 'units/nope', field: 'a/b', fix: { suggested: 'x' } }, plan), null);
    });

    it('forEditor gives the editor rule, severity, message, and the split id path', () => {
        const mapped = Issues.forEditor(ISSUES.issues);
        assert.equal(mapped.length, ISSUES.issues.length);
        assert.deepEqual(mapped[3], {
            rule: 'pointing-form', severity: 'warn', message: 'the marks are not in canonical order',
            path: LISTEN, field: 'passage', pointer: '/units/0/lessons/0/blocks/2/passage'
        });
        assert.deepEqual(Issues.forEditor(null), []);
    });
});

describe('ac-1: the panel shows every issue the server sent', () => {
    function setup(extra) {
        const env = loadPanels({ issues: ISSUES });
        const jumps = [];
        const fixes = [];
        const loaded = [];
        let outcome = 'refreshed';
        const panel = env.W.HebrewPlanIssues.create(Object.assign({
            doc: env.W.document, api: env.W.HebrewApi, planId: 'sample',
            getPlan: () => env.server.plan,
            onJump: (issue) => jumps.push(issue),
            onFix: (request, issue) => { fixes.push({ request, issue }); return Promise.resolve(outcome); },
            onLoaded: (result) => loaded.push(result)
        }, extra || {}));
        return { env, panel, jumps, fixes, loaded, setOutcome: (v) => { outcome = v; } };
    }

    it('renders code, message, group, and counts; hard errors say "blocks publish", warnings say they do not', async () => {
        const { env, panel, loaded } = setup();
        await panel.load();
        assert.equal(loaded.length, 1);
        const el = panel.el;
        assert.equal(el.getAttribute('data-plan-issues'), 'panel');
        assert.match(text(one(el, 'data-plan-issues-count')), /1 שגיאות חוסמות · 5 אזהרות · 1 הערות/);
        const items = byAttr(el, 'data-plan-issue');
        assert.equal(items.length, ISSUES.issues.length);
        assert.deepEqual(items.map((i) => i.getAttribute('data-plan-issue')),
            ['meaning', 'lexicon-form', 'lexicon-form', 'pointing-form', 'pointing-consistency', 'audio-stale', 'N6']);
        assert.deepEqual(items.map((i) => i.getAttribute('data-plan-issue-severity')),
            ['block', 'warn', 'warn', 'warn', 'warn', 'warn', 'inform']);
        items.forEach((item, n) => assert.ok(text(item).indexOf(ISSUES.issues[n].message) !== -1, 'message ' + n));
        const hard = one(el, 'data-plan-issue-group', 'block');
        const warn = one(el, 'data-plan-issue-group', 'warn');
        assert.match(text(hard), /Blocks publish/);
        assert.match(text(warn), /does not block publish/);
        assert.doesNotMatch(text(warn).replace(/does not block publish/g, ''), /blocks? publish/i, 'the words appear only on hard errors');
        assert.doesNotMatch(text(one(el, 'data-plan-issue-group', 'inform')), /blocks? publish/i);
        // the stable codes are shown
        assert.match(text(warn), /pointing-consistency/);
        assert.equal(env.callsOf('GET', /issues$/).length, 1, 'the default read has no severity filter');
    });

    it('a link on each item names the node and field, and opens it', async () => {
        const { panel, jumps } = setup();
        await panel.load();
        const links = byAttr(panel.el, 'data-plan-issue-link');
        assert.equal(links.length, ISSUES.issues.length);
        assert.equal(links[3].getAttribute('data-plan-issue-link'), LISTEN);
        assert.equal(links[3].getAttribute('data-plan-issue-field'), 'passage');
        assert.match(text(links[3]), new RegExp(LISTEN.replace(/\//g, '\\/') + '#passage'));
        links[3].click();
        assert.equal(jumps.length, 1);
        assert.equal(jumps[0].code, 'pointing-form');
        links[0].click();
        assert.equal(jumps[1].code, 'meaning');
    });

    it('an empty answer says "no issues"; a truncated one says so; a failed read shows the code', async () => {
        const empty = setup();
        empty.env.server.issues = { revision: 4, status: 'draft', counts: { block: 0, warn: 0, inform: 0 }, issues: [], truncated: false };
        await empty.panel.load();
        assert.ok(one(empty.panel.el, 'data-plan-issues-empty'));
        assert.match(text(one(empty.panel.el, 'data-plan-issues-empty')), /No issues/);
        const cut = setup();
        cut.env.server.issues = Object.assign({}, ISSUES, { truncated: true });
        await cut.panel.load();
        assert.ok(one(cut.panel.el, 'data-plan-issues-truncated'));
        const bad = setup();
        bad.env.on('GET', /issues/, () => bad.env.json(404, { error: 'Not found', code: 'not_found' }));
        await bad.panel.load();
        assert.match(text(one(bad.panel.el, 'data-plan-issues-state')), /not_found/);
    });

    it('the notes toggle reads again with severity=all', async () => {
        const { env, panel } = setup();
        await panel.load();
        env.calls.length = 0;
        const box = one(panel.el, 'data-plan-issues-informs');
        box.checked = true;
        box.dispatch('change');
        await env.sleep(20);
        const reads = env.callsOf('GET', /issues/);
        assert.equal(reads.length, 1);
        assert.match(reads[0].url, /severity=all$/);
    });

    it('a stale answer does not overwrite a newer one', async () => {
        const { env, panel } = setup();
        let release;
        let first = true;
        const old = { revision: 3, status: 'draft', counts: { block: 0, warn: 1, inform: 0 }, truncated: false, issues: [ISSUES.issues[3]] };
        env.on('GET', /issues/, () => {
            if (first) {
                first = false;
                return new Promise((resolve) => { release = () => resolve(env.json(200, old)); });
            }
            return env.json(200, ISSUES);
        });
        const slow = panel.load();   // held back
        await panel.load();          // answers at once with the newer list
        release();
        await slow;
        assert.equal(byAttr(panel.el, 'data-plan-issue').length, ISSUES.issues.length, 'the older answer was dropped');
        assert.match(text(one(panel.el, 'data-plan-issues-count')), /1 שגיאות חוסמות/);
    });
});

describe('ac-2: one-click fixes', () => {
    async function setup() {
        const env = loadPanels({ issues: ISSUES });
        const fixes = [];
        let outcome = 'refreshed';
        const panel = env.W.HebrewPlanIssues.create({
            doc: env.W.document, api: env.W.HebrewApi, planId: 'sample',
            getPlan: () => env.server.plan,
            onFix: (request) => { fixes.push(request); return Promise.resolve(outcome); }
        });
        await panel.load();
        env.calls.length = 0;
        return { env, panel, fixes, setOutcome: (v) => { outcome = v; } };
    }

    it('"Use this value" sends exactly one request with the issue path, field, and suggested value', async () => {
        const { panel, fixes } = await setup();
        const item = byAttr(panel.el, 'data-plan-issue')[1];
        assert.match(text(one(item, 'data-plan-fix-current')), /בַּיִת/);
        assert.match(text(one(item, 'data-plan-fix-suggested')), /בַּיִת/);
        assert.equal(one(item, 'data-plan-fix-suggested').getAttribute('dir'), 'rtl');
        assert.equal(one(item, 'data-plan-fix-suggested').getAttribute('lang'), 'he');
        assert.equal(one(item, 'data-plan-fix-current').getAttribute('dir'), 'rtl');
        one(item, 'data-plan-fix').click();
        await new Promise((r) => setTimeout(r, 10));
        assert.deepEqual(JSON.parse(JSON.stringify(fixes)), [{ path: WORD, field: 'word', value: 'בַּיִת' }]);
    });

    it('after a fix the issues are read again, unless the host already did', async () => {
        const { env, panel, setOutcome } = await setup();
        const item = byAttr(panel.el, 'data-plan-issue')[1];
        setOutcome(true);
        one(item, 'data-plan-fix').click();
        await env.sleep(20);
        assert.equal(env.callsOf('GET', /issues/).length, 1);
        env.calls.length = 0;
        setOutcome('refreshed');
        one(byAttr(panel.el, 'data-plan-issue')[1], 'data-plan-fix').click();
        await env.sleep(20);
        assert.equal(env.callsOf('GET', /issues/).length, 0);
    });

    it('a refused fix shows the host\'s reason and sends nothing more', async () => {
        const { env, panel, fixes, setOutcome } = await setup();
        setOutcome({ error: 'validation 4: schema — The plan did not pass validation.' });
        one(byAttr(panel.el, 'data-plan-issue')[1], 'data-plan-fix').click();
        await env.sleep(20);
        assert.equal(fixes.length, 1);
        const message = one(panel.el, 'data-plan-fix-message');
        assert.equal(message.hidden, false);
        assert.match(text(message), /schema/);
        assert.equal(env.callsOf('GET', /issues/).length, 0);
    });

    it('a pointing-consistency item offers the forms as choices and sends the chosen one', async () => {
        const { panel, fixes } = await setup();
        const item = byAttr(panel.el, 'data-plan-issue').find((i) => i.getAttribute('data-plan-issue') === 'pointing-consistency');
        assert.equal(byAttr(item, 'data-plan-fix').length, 0, 'no single "use this" button');
        const choices = byAttr(item, 'data-plan-fix-choice');
        assert.deepEqual(choices.map((c) => text(c)), ['בַּיִת', 'בַיִת']);
        assert.equal(choices[0].getAttribute('lang'), 'he');
        choices[1].click();
        await new Promise((r) => setTimeout(r, 10));
        assert.deepEqual(JSON.parse(JSON.stringify(fixes)), [{ path: WORD, field: 'word', value: 'בַיִת' }]);
    });

    it('an audio-stale item has no apply button and shows the instruction text', async () => {
        const { panel, fixes } = await setup();
        const item = byAttr(panel.el, 'data-plan-issue').find((i) => i.getAttribute('data-plan-issue') === 'audio-stale');
        assert.equal(byAttr(item, 'data-plan-fix').length, 0);
        assert.equal(byAttr(item, 'data-plan-fix-choice').length, 0);
        assert.match(text(one(item, 'data-plan-fix-note')), /No tool generates audio/);
        assert.equal(fixes.length, 0);
    });

    it('a hard error with no fix has no apply button either', async () => {
        const { panel } = await setup();
        const item = byAttr(panel.el, 'data-plan-issue')[0];
        assert.equal(byAttr(item, 'data-plan-fix-box').length, 0);
    });
});

describe('ac-3: the live preview', () => {
    const LISTEN_BLOCK = () => samplePlan().units[0].lessons[0].blocks.find((b) => b.id === 'listening');
    const handle = (path, prop, value, saved, extra) => Object.assign({
        path, prop,
        isDirty: () => JSON.stringify(value) !== JSON.stringify(saved),
        check: () => null,
        getValue: () => value
    }, extra || {});

    it('overlay lays an unsaved value over a copy and leaves the saved node alone', () => {
        const block = LISTEN_BLOCK();
        const before = JSON.stringify(block);
        const out = Preview.overlay(block, LISTEN, [handle(LISTEN, 'passage', 'טקסט חדש', block.passage)], true);
        assert.equal(out.node.passage, 'טקסט חדש');
        assert.equal(out.dirty, true);
        assert.equal(JSON.stringify(block), before);
        const clean = Preview.overlay(block, LISTEN, [handle(LISTEN, 'passage', block.passage, block.passage)], true);
        assert.equal(clean.dirty, false);
        assert.equal(clean.node.passage, block.passage);
    });

    it('overlay reaches an item of the block, skips invalid JSON but counts it unsaved, and an emptied optional field is removed', () => {
        const block = samplePlan().units[0].lessons[0].blocks.find((b) => b.id === 'introduction');
        const path = INTRO + '/words/1';
        const out = Preview.overlay(block, INTRO, [
            handle(path, 'meaning', 'a home', 'house'),
            handle(path, 'sentence', undefined, 'x'),
            handle(path, 'translit', 'ignored', 'bayit', { check: () => 'bad json' }),
            handle(LESSON, 'title', 'not for this block', 'Lesson'),
            handle(INTRO + '/words/nope', 'meaning', 'no such word', 'x')
        ], true);
        assert.equal(out.node.words[0].meaning, 'a home');
        assert.equal('sentence' in out.node.words[0], false);
        assert.equal(out.node.words[0].translit, 'bayit', 'an invalid value is left out of the picture');
        assert.equal(out.dirty, true);
        // a handle of another node never changes this one
        assert.equal(out.node.title, undefined);
    });

    it('overlay for a lesson takes its own form only, not its blocks', () => {
        const lesson = samplePlan().units[0].lessons[0];
        const out = Preview.overlay(lesson, LESSON, [
            handle(LESSON, 'title', 'New title', lesson.title),
            handle(LISTEN, 'passage', 'block text', 'x')
        ], false);
        assert.equal(out.node.title, 'New title');
        assert.equal(out.node.blocks.find((b) => b.id === 'listening').passage, lesson.blocks.find((b) => b.id === 'listening').passage);
        assert.equal(out.dirty, true);
    });

    it('compute draws the block with the learner renderer and escapes what was typed', () => {
        const block = LISTEN_BLOCK();
        const hostile = '<img src=x onerror=alert(1)>';
        const out = Preview.compute({
            render: Render, lesson: samplePlan().units[0].lessons[0], lessonPath: LESSON, block, blockPath: LISTEN,
            handles: [handle(LISTEN, 'passage', hostile, block.passage)]
        });
        assert.equal(out.state, 'unsaved');
        assert.equal(out.kind, 'block');
        assert.match(out.html, /data-block-type="listening"/);
        assert.match(out.html, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.doesNotMatch(out.html, /<img/);
        // the same markup as the learner's own renderer gives for the same block
        assert.equal(Preview.compute({ render: Render, lesson: null, block, blockPath: LISTEN, handles: [] }).html, Render.renderBlock(block, { lesson: undefined }));
        assert.equal(Preview.compute({ render: Render, block, blockPath: LISTEN, handles: [] }).state, 'saved');
    });

    it('compute draws all eleven block types', () => {
        const lesson = samplePlan().units[0].lessons[0];
        lesson.blocks.forEach((block) => {
            const out = Preview.compute({ render: Render, lesson, lessonPath: LESSON, block, blockPath: LESSON + '/blocks/' + block.id, handles: [] });
            assert.ok(out.html && out.html.length > 20, block.type + ' has a preview');
            assert.match(out.html, new RegExp('data-block-type="' + block.type + '"'), block.type);
        });
    });

    it('with no block selected the preview is the lesson card, with the unsaved title', () => {
        const lesson = samplePlan().units[0].lessons[0];
        const out = Preview.compute({
            render: Render, lesson, lessonPath: LESSON, block: null,
            handles: [handle(LESSON, 'title', 'A <b>new</b> title', lesson.title)]
        });
        assert.equal(out.kind, 'lesson');
        assert.equal(out.state, 'unsaved');
        assert.match(out.html, /A &lt;b&gt;new&lt;\/b&gt; title/);
        assert.doesNotMatch(out.html, /<b>/);
    });

    it('compute with nothing to draw, or a renderer that throws, never throws', () => {
        assert.deepEqual(Preview.compute({ render: null }), { html: '', state: 'saved', kind: 'none' });
        assert.deepEqual(Preview.compute({ render: Render }), { html: '', state: 'saved', kind: 'none' });
        const broken = { renderBlock() { throw new Error('half-typed'); } };
        const out = Preview.compute({ render: broken, block: LISTEN_BLOCK(), blockPath: LISTEN, handles: [] });
        assert.equal(out.html, null, 'the page keeps the last picture');
        assert.equal(out.state, 'unsaved');
    });

    it('guardClick stops a link or a button inside the preview from acting', () => {
        const stop = { nodeType: 1, parentNode: null, tagName: 'DIV' };
        const link = { nodeType: 1, tagName: 'A', parentNode: stop };
        const inner = { nodeType: 1, tagName: 'SPAN', parentNode: link };
        const plain = { nodeType: 1, tagName: 'P', parentNode: stop };
        let prevented = 0;
        const ev = (target) => ({ target, preventDefault() { prevented += 1; } });
        assert.equal(Preview.guardClick(ev(inner), stop), true);
        assert.equal(prevented, 1);
        assert.equal(Preview.guardClick(ev({ nodeType: 1, tagName: 'BUTTON', parentNode: stop }), stop), true);
        assert.equal(Preview.guardClick(ev(plain), stop), false);
        assert.equal(prevented, 2);
    });

    it('the preview module makes no request and the renderer has no network call for a block', () => {
        const source = fs.readFileSync(path.resolve(__dirname, '..', 'shared', 'admin', 'plan-preview.js'), 'utf8');
        assert.doesNotMatch(source, /fetch\(|XMLHttpRequest|HebrewApi|\.request\(|sendBeacon/);
        assert.doesNotMatch(source, /innerHTML|eval\(|document\.write/);
    });
});

describe('the builder: issues, jump, fix, preview, and publish through one editor', () => {
    async function mounted(extra) {
        const env = loadPanels(Object.assign({ issues: ISSUES }, extra || {}));
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        return { env, panel, Builder: env.W.HebrewPlanBuilder };
    }
    const select = async (ctx, blockId) => {
        one(one(ctx.panel, 'data-plan-block', blockId), 'data-plan-edit').click();
        await ctx.Builder.settled();
    };

    it('shows the panel after the plan loads, and the publish line with the counts', async () => {
        const { panel, env } = await mounted();
        assert.equal(byAttr(panel, 'data-plan-issues', 'panel').length, 1);
        assert.equal(byAttr(byAttr(panel, 'data-plan-issues', 'panel')[0], 'data-plan-issue').length, ISSUES.issues.length);
        const note = text(one(panel, 'data-plan-publish-note'));
        assert.match(note, /1 שגיאות חוסמות, 5 אזהרות/);
        assert.match(note, /Warnings do not stop publishing/);
        assert.equal(env.callsOf('GET', /issues$/).length, 1);
    });

    it('the issues also show inline at their fields', async () => {
        const ctx = await mounted();
        await select(ctx, 'listening');
        const row = one(ctx.panel, 'data-plan-field', 'passage').parentNode;
        const inline = all(row, (n) => n.getAttribute && n.getAttribute('data-plan-issue') === 'pointing-form' && !n.hasAttribute('data-plan-issue-index'));
        assert.equal(inline.length, 1);
    });

    it('a link opens the block form and focuses the field', async () => {
        const ctx = await mounted();
        const link = byAttr(ctx.panel, 'data-plan-issue-link').find((l) => l.getAttribute('data-plan-issue-link') === LISTEN);
        link.click();
        await ctx.Builder.settled();
        assert.ok(one(ctx.panel, 'data-plan-node', LISTEN), 'the form of that block is on the page');
        const jumped = one(ctx.panel, 'data-plan-jump', 'true');
        assert.ok(jumped, 'the field is flagged');
        assert.equal(one(jumped, 'data-plan-field').getAttribute('data-plan-field'), 'passage');
        assert.equal(ctx.env.W.document.activeElement, one(ctx.panel, 'data-plan-field', 'passage'));
    });

    it('a link to a word opens that word inside its block', async () => {
        const ctx = await mounted();
        const link = byAttr(ctx.panel, 'data-plan-issue-link').find((l) => l.getAttribute('data-plan-issue-link') === WORD && l.getAttribute('data-plan-issue-field') === 'word');
        link.click();
        await ctx.Builder.settled();
        const form = one(ctx.panel, 'data-plan-node', WORD);
        assert.ok(form, 'the word form is open');
        assert.equal(ctx.env.W.document.activeElement.getAttribute('data-plan-field') || ctx.env.W.document.activeElement.getAttribute('data-plan-sub'), 'word');
    });

    it('a link to the lesson and one to the plan open those sections', async () => {
        const ctx = await mounted();
        ctx.env.server.issues = Object.assign({}, ISSUES, { issues: [
            { code: 'N6', severity: 'warn', hard: false, path: '', field: 'title', pointer: '/title', message: 'plan level' },
            { code: 'N7', severity: 'warn', hard: false, path: LESSON, field: 'level', pointer: '/units/0/lessons/0/level', message: 'lesson level' },
            { code: 'N8', severity: 'warn', hard: false, path: 'units/u1', field: 'theme', pointer: '/units/0/theme', message: 'unit level' }
        ] });
        const panel = ctx.panel;
        one(panel, 'data-plan-issues-refresh').click();
        await ctx.env.sleep(30);
        const links = byAttr(panel, 'data-plan-issue-link');
        assert.equal(links.length, 3);
        links[0].click();
        await ctx.Builder.settled();
        assert.ok(one(panel, 'data-plan-node', ''), 'the plan section is open');
        links[2].click();
        await ctx.Builder.settled();
        assert.ok(one(panel, 'data-plan-node', 'units/u1'), 'the unit section is open');
        byAttr(panel, 'data-plan-issue-link')[1].click();
        await ctx.Builder.settled();
        assert.ok(one(panel, 'data-plan-node', LESSON));
        assert.equal(one(panel, 'data-plan-jump', 'true') && true, true);
    });

    it('"Use this value" sends one set_field through the editor save path, with the builder\'s revision, then reads the issues', async () => {
        const ctx = await mounted();
        ctx.env.calls.length = 0;
        const item = byAttr(ctx.panel, 'data-plan-issue').find((i) => i.getAttribute('data-plan-issue') === 'pointing-form' && one(i, 'data-plan-fix'));
        one(item, 'data-plan-fix').click();
        await ctx.env.sleep(30);
        await ctx.Builder.settled();
        const patches = ctx.env.callsOf('PATCH', /fields$/);
        assert.equal(patches.length, 1);
        assert.deepEqual(patches[0].body, { path: LISTEN, field: 'passage', value: 'תָּ דָּנִי.', expectedRevision: 4 });
        assert.equal(ctx.env.callsOf('GET', /issues$/).length, 1, 'the issues are read once after the save');
        assert.equal(ctx.env.server.plan.units[0].lessons[0].blocks.find((b) => b.id === 'listening').passage, 'תָּ דָּנִי.');
    });

    it('a fix inside a field sends the whole field once; a stale revision is a conflict message and nothing is stored', async () => {
        const ctx = await mounted();
        ctx.env.calls.length = 0;
        const nested = byAttr(ctx.panel, 'data-plan-issue').find((i) => i.getAttribute('data-plan-issue') === 'lexicon-form' && one(i, 'data-plan-fix')
            && one(i, 'data-plan-issue-link').getAttribute('data-plan-issue-field') === 'cloze/items/0/answer');
        ctx.env.server.revision = 9; // the AI wrote in between
        one(nested, 'data-plan-fix').click();
        await ctx.env.sleep(30);
        await ctx.Builder.settled();
        const patches = ctx.env.callsOf('PATCH', /fields$/);
        assert.equal(patches.length, 1);
        assert.equal(patches[0].body.field, 'cloze');
        assert.equal(patches[0].body.path, INTRO);
        assert.equal(patches[0].body.value.items[0].answer, 'בַּיִת');
        assert.equal(patches[0].body.expectedRevision, 4);
        const conflict = one(ctx.panel, 'data-plan-conflict');
        assert.equal(conflict.hidden, false);
        assert.match(text(conflict), /revision_conflict 9/);
    });

    it('typing changes the preview after the debounce, with no request, and the state is unsaved until the save', async () => {
        const ctx = await mounted();
        await select(ctx, 'listening');
        const preview = one(ctx.panel, 'data-plan-preview');
        assert.equal(preview.getAttribute('data-plan-preview-state'), 'saved');
        assert.match(preview.innerHTML, /data-block-type="listening"/);
        const saved = samplePlan().units[0].lessons[0].blocks.find((b) => b.id === 'listening').passage;
        assert.ok(preview.innerHTML.indexOf(saved) !== -1);

        const field = one(ctx.panel, 'data-plan-field', 'passage');
        ctx.env.calls.length = 0;
        field.value = 'שלום חדש';
        field.dispatch('input');
        assert.ok(preview.innerHTML.indexOf('שלום חדש') === -1, 'not before the debounce');
        await ctx.env.sleep(Preview.DEBOUNCE_MS + 80);
        assert.ok(preview.innerHTML.indexOf('שלום חדש') !== -1, 'within the debounce interval plus a little');
        assert.equal(preview.getAttribute('data-plan-preview-state'), 'unsaved');
        assert.match(text(one(ctx.panel, 'data-plan-preview-label')), /not saved/);
        assert.equal(ctx.env.calls.length, 0, 'typing makes no request');

        one(field.parentNode, 'data-plan-save').click();
        await ctx.env.sleep(40);
        await ctx.Builder.settled();
        const after = one(ctx.panel, 'data-plan-preview');
        assert.equal(after.getAttribute('data-plan-preview-state'), 'saved');
        assert.ok(after.innerHTML.indexOf('שלום חדש') !== -1, 'the saved text is what is drawn');
        assert.match(text(one(ctx.panel, 'data-plan-preview-label')), /as saved/);
    });

    it('a value with <img onerror=…> appears as text in the preview', async () => {
        const ctx = await mounted();
        await select(ctx, 'listening');
        const field = one(ctx.panel, 'data-plan-field', 'passage');
        field.value = '<img src=x onerror=alert(1)>';
        field.dispatch('input');
        await ctx.env.sleep(Preview.DEBOUNCE_MS + 80);
        const html = one(ctx.panel, 'data-plan-preview').innerHTML;
        assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.doesNotMatch(html, /<img/);
    });

    it('with no block selected the lesson title is previewed as the lesson card', async () => {
        const ctx = await mounted();
        const preview = one(ctx.panel, 'data-plan-preview');
        assert.match(preview.innerHTML, /lesson-list/);
        const title = one(one(ctx.panel, 'data-plan-node', LESSON), 'data-plan-field', 'title');
        title.value = 'Typed lesson title';
        title.dispatch('input');
        await ctx.env.sleep(Preview.DEBOUNCE_MS + 80);
        assert.match(one(ctx.panel, 'data-plan-preview').innerHTML, /Typed lesson title/);
        assert.equal(one(ctx.panel, 'data-plan-preview').getAttribute('data-plan-preview-state'), 'unsaved');
    });

    it('a click on a link inside the preview does not leave the page', async () => {
        const ctx = await mounted();
        await select(ctx, 'listening');
        const preview = one(ctx.panel, 'data-plan-preview');
        let prevented = false;
        const listeners = preview.listeners.click || [];
        assert.equal(listeners.length, 1);
        listeners[0]({ target: { nodeType: 1, tagName: 'A', parentNode: preview }, preventDefault() { prevented = true; } });
        assert.equal(prevented, true);
    });

    it('no request goes anywhere but the plan routes, the schema, and the account list; none to a preview route', async () => {
        const ctx = await mounted();
        await select(ctx, 'listening');
        const field = one(ctx.panel, 'data-plan-field', 'passage');
        field.value = 'x';
        field.dispatch('input');
        await ctx.env.sleep(Preview.DEBOUNCE_MS + 50);
        ctx.calls = ctx.env.calls;
        ctx.env.calls.forEach((c) => {
            assert.ok(/^\/api\/(plans\/sample\/|plan-schema$|admin\/users$)/.test(c.url), c.url);
            assert.doesNotMatch(c.url, /preview/i);
        });
    });

    it('Publish sends one POST with the revision and nothing else, asks for no approval, and shows the counts after', async () => {
        const ctx = await mounted();
        ctx.env.calls.length = 0;
        one(ctx.panel, 'data-plan-publish').click();
        await ctx.env.sleep(40);
        await ctx.Builder.settled();
        const posts = ctx.env.callsOf('POST', /publish$/);
        assert.equal(posts.length, 1);
        assert.deepEqual(posts[0].body, { expectedRevision: 4 });
        assert.match(text(one(ctx.panel, 'data-plan-notice')), /Published.*1 blocking · 5 warnings/);
        assert.equal(ctx.env.callsOf('GET', /issues$/).length >= 1, true, 'the issues are read again');
    });

    it('a refused publish shows the server\'s code and its findings', async () => {
        const ctx = await mounted();
        ctx.env.on('POST', /publish$/, () => ctx.env.json(400, {
            error: 'validation', revision: 4,
            blocks: [{ rule: 'meaning', severity: 'block', path: '/units/0/lessons/0/newWords/0', message: 'no meaning' }],
            warnings: [], informs: []
        }));
        one(ctx.panel, 'data-plan-publish').click();
        await ctx.env.sleep(40);
        await ctx.Builder.settled();
        assert.match(text(one(ctx.panel, 'data-plan-publish-refusal')), /validation/);
        assert.ok(one(ctx.panel, 'data-plan-finding', 'meaning'));
        assert.equal(text(one(ctx.panel, 'data-plan-revision')), '4', 'nothing was published');
    });

    it('unmount removes every listener, the preview timer, and the panels', async () => {
        const ctx = await mounted();
        await select(ctx, 'listening');
        const field = one(ctx.panel, 'data-plan-field', 'passage');
        field.value = 'pending';
        field.dispatch('input');
        const issuesEl = one(ctx.panel, 'data-plan-issues', 'panel');
        ctx.Builder.unmount();
        assert.equal(ctx.panel.listeners.click.length, 0);
        assert.equal(ctx.panel.listeners.input.length, 0);
        assert.equal(ctx.panel.listeners.change.length, 0);
        assert.equal(issuesEl.listeners.click.length, 0);
        const before = ctx.env.calls.length;
        await ctx.env.sleep(Preview.DEBOUNCE_MS + 50);
        assert.equal(ctx.env.calls.length, before, 'a late timer does nothing');
    });
});

describe('the builder without the optional panels is the editor alone', () => {
    it('plan-builder.js names the panels only through root globals, so a missing one is skipped', () => {
        const source = fs.readFileSync(path.resolve(__dirname, '..', 'shared', 'admin', 'plan-builder.js'), 'utf8');
        ['HebrewPlanIssues', 'HebrewPlanHistory', 'HebrewPlanPreview', 'HebrewPlanList'].forEach((name) => {
            assert.ok(new RegExp('root\\.' + name).test(source), name);
            assert.doesNotMatch(source, new RegExp('[^.\\w]' + name + '\\.'), name + ' is read from root only');
        });
    });
});

describe('Form.focusField and Form.open', () => {
    it('open marks an item\'s form open, and focusField is false when the form is not on the page', () => {
        const { makeEditor } = require('./helpers/plan-editor-fixtures');
        const { editor } = makeEditor();
        assert.equal(editor.focusField('units/none', 'x'), false);
        editor.open(WORD);
        assert.equal(editor.isOpen(WORD, false), true);
        assert.equal(typeof Form.createEditor, 'function');
    });
});

void NFC_BAD;
