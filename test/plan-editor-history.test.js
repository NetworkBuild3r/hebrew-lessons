/**
 * The history panel, the compare view, the restore confirmation, and the "edited by a person" marks
 * (INIT-008/SPEC-014), on a fake DOM with a stand-in for the plan routes.
 * A browser run is test/chrome-plan-editor-smoke.js.
 * Provenance: INIT-008/SPEC-014
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { loadPanels, all, byAttr } = require('./helpers/plan-panels-vm');
const { makeEditor, samplePlan, LESSON } = require('./helpers/plan-editor-fixtures');

const History = require('../shared/admin/plan-history.js');

const LISTEN = LESSON + '/blocks/listening';
const text = (node) => node.textContent;
const one = (node, name, value) => byAttr(node, name, value)[0];

const REVS = [
    { revision: 6, op: 'restore', path: LISTEN, field: 'passage', summary: 'restore', actorKind: 'human', actorName: 'example-admin', tool: 'restore_plan_revision', client: null, at: '2026-10-12T22:05:00.000Z' },
    { revision: 5, op: 'set_field', path: LISTEN, field: 'passage', summary: 'set_field', actorKind: 'ai', actorName: 'mcp', tool: 'set_field', client: null, at: '2026-10-12T22:00:00.000Z' },
    { revision: 4, op: 'set_field', path: LISTEN, field: 'passage', summary: 'set_field', actorKind: 'human', actorName: 'example-admin', tool: 'set_field', client: null, at: '2026-10-12T21:00:00.000Z' },
    { revision: 3, op: 'baseline', path: '', field: '', summary: 'baseline', actorKind: 'system', actorName: 'baseline', tool: 'baseline', client: null, at: '2026-10-12T20:00:00.000Z' }
];

const DIFF = {
    from: { ref: 5, revision: 5 },
    to: { ref: 4, revision: 4 },
    entries: [
        { path: LISTEN, field: 'passage', kind: 'changed', before: 'שָׁלוֹם דָּנִי', after: 'שָׁלוֹם רוּת' },
        { path: LISTEN, field: 'audioPath', kind: 'changed', before: 'a.mp3', after: 'b.mp3' },
        { path: LESSON + '/blocks/gone', field: '', kind: 'removed', before: { id: 'gone', type: 'listening', passage: 'x' } },
        { path: LESSON + '/blocks/new', field: '', kind: 'added', after: { id: 'new', type: 'listening' } },
        { path: LESSON, field: 'blocks', kind: 'moved', before: ['a', 'b'], after: ['b', 'a'] }
    ],
    truncated: false
};

function setup(extra) {
    const env = loadPanels(Object.assign({ revisions: REVS }, extra || {}));
    env.server.revision = 6;
    env.server.plan.revision = 6;
    const events = { restored: [], reloads: 0 };
    let revision = 6;
    let unsaved = false;
    const history = env.W.HebrewPlanHistory.create({
        doc: env.W.document, api: env.W.HebrewApi, planId: 'sample',
        getRevision: () => revision,
        hasUnsaved: () => unsaved,
        onRestored: (data) => { events.restored.push(data); revision = data.revision; return Promise.resolve(null); },
        onReload: () => { events.reloads += 1; }
    });
    return { env, history, events, setRevision: (n) => { revision = n; }, setUnsaved: (v) => { unsaved = v; } };
}

async function openHistory(ctx) {
    one(ctx.history.el, 'data-plan-history-toggle').click();
    await ctx.env.sleep(20);
}

describe('history: pure helpers', () => {
    it('actorMark names the writer in text and an icon', () => {
        assert.deepEqual(History.actorMark(REVS[0]), { kind: 'human', he: 'אדם', en: 'A person', detail: 'example-admin', icon: 'admin' });
        assert.equal(History.actorMark(REVS[1]).kind, 'ai');
        assert.equal(History.actorMark(REVS[1]).detail, 'set_field');
        assert.equal(History.actorMark(REVS[3]).kind, 'system');
        assert.equal(History.actorMark({ actorKind: 'surprise' }).kind, 'system');
        ['human', 'ai', 'system'].forEach((kind) => {
            const mark = History.actorMark({ actorKind: kind });
            assert.ok(mark.he && mark.en && mark.icon, kind + ' has text in both languages and an icon');
        });
    });

    it('groupEntries keeps the server order and groups by path', () => {
        const groups = History.groupEntries(DIFF.entries);
        assert.deepEqual(groups.map((g) => g.path), [LISTEN, LESSON + '/blocks/gone', LESSON + '/blocks/new', LESSON]);
        assert.equal(groups[0].entries.length, 2);
        assert.deepEqual(History.groupEntries(null), []);
    });

    it('describeRef names a revision, the working copy, and the published copy', () => {
        assert.match(History.describeRef({ ref: 7, revision: 7 }), /revision 7/);
        assert.match(History.describeRef({ ref: 'working', revision: 9 }), /working copy/);
        assert.match(History.describeRef({ ref: 'published', revision: 3 }), /published copy/);
    });

    it('restoreOffers: a value from the earlier side, never from the working copy, never for an added node', () => {
        const from = { ref: 5, revision: 5 };
        assert.deepEqual(History.restoreOffers(DIFF.entries[0], from), { field: true, part: true, fromRevision: 5 });
        assert.deepEqual(History.restoreOffers(DIFF.entries[2], from), { field: false, part: true, fromRevision: 5 }, 'a removed node can be put back');
        assert.deepEqual(History.restoreOffers(DIFF.entries[3], from), { field: false, part: false, fromRevision: 5 }, 'an added node has no earlier value');
        assert.equal(History.restoreOffers(DIFF.entries[4], from).part, false, 'a moved list');
        assert.deepEqual(History.restoreOffers(DIFF.entries[0], { ref: 'working', revision: 9 }), { field: false, part: false, fromRevision: null });
        assert.equal(History.restoreOffers({ path: '', field: 'title', kind: 'changed' }, from).part, false, 'the plan root has no part restore');
        assert.equal(History.restoreOffers({ path: '', field: 'title', kind: 'changed' }, from).field, true);
        // the published side restores from its own revision
        assert.equal(History.restoreOffers(DIFF.entries[0], { ref: 'published', revision: 3 }).fromRevision, 3);
    });

    it('restoreBody: a whole restore has no path or field; the last revision rides along', () => {
        assert.deepEqual(History.restoreBody({ fromRevision: 3 }, 9), { fromRevision: 3, expectedRevision: 9 });
        assert.deepEqual(History.restoreBody({ fromRevision: 3, path: LISTEN }, 9), { fromRevision: 3, expectedRevision: 9, path: LISTEN });
        assert.deepEqual(History.restoreBody({ fromRevision: 3, path: LISTEN, field: 'passage' }, 9),
            { fromRevision: 3, expectedRevision: 9, path: LISTEN, field: 'passage' });
        assert.deepEqual(History.restoreBody({ fromRevision: 3, path: '', field: 'title' }, 9),
            { fromRevision: 3, expectedRevision: 9, path: '', field: 'title' });
    });

    it('restoreWhat says what will come back, in both languages', () => {
        assert.match(History.restoreWhat({ fromRevision: 3 })[0], /כל התוכנית/);
        assert.match(History.restoreWhat({ fromRevision: 3 })[1], /whole plan to revision 3/);
        assert.match(History.restoreWhat({ fromRevision: 3, path: LISTEN, field: 'passage' })[1], /field .*#passage/);
        assert.match(History.restoreWhat({ fromRevision: 3, path: LISTEN })[1], /the part/);
    });
});

describe('ac-1: the history list', () => {
    it('loads nothing until it is opened, then reads the list once', async () => {
        const ctx = setup();
        assert.equal(ctx.env.callsOf('GET', /revisions/).length, 0);
        await ctx.history.load();
        assert.equal(ctx.env.callsOf('GET', /revisions/).length, 0, 'a closed panel does not read');
        await openHistory(ctx);
        assert.equal(ctx.env.callsOf('GET', /revisions/).length, 1);
        assert.equal(one(ctx.history.el, 'data-plan-history-toggle').getAttribute('aria-expanded'), 'true');
    });

    it('renders one row per revision, newest first, with the actor mark as text', async () => {
        const ctx = setup();
        await openHistory(ctx);
        const rows = byAttr(ctx.history.el, 'data-plan-revision');
        assert.deepEqual(rows.map((r) => r.getAttribute('data-plan-revision')), ['6', '5', '4', '3']);
        assert.deepEqual(rows.map((r) => r.getAttribute('data-plan-actor')), ['human', 'ai', 'human', 'system']);
        assert.match(text(one(rows[0], 'data-plan-actor-mark')), /אדם/);
        assert.match(text(one(rows[0], 'data-plan-actor-mark')), /A person/);
        assert.match(text(one(rows[0], 'data-plan-actor-mark')), /example-admin/, 'a person row shows the account name');
        assert.match(text(one(rows[1], 'data-plan-actor-mark')), /AI/);
        assert.match(text(one(rows[1], 'data-plan-actor-mark')), /set_field/, 'an AI row shows the tool');
        assert.match(text(rows[2]), /2026-10-12 21:00/);
        assert.equal(text(one(rows[0], 'data-plan-op')), 'restore');
        // every row can be opened, compared with the working copy, and restored whole
        rows.forEach((row) => {
            assert.ok(one(row, 'data-plan-rev-open'));
            assert.ok(one(row, 'data-plan-diff-quick', 'revision'));
            assert.ok(one(row, 'data-plan-restore', 'whole'));
        });
    });

    it('the actor filter narrows the rows with one request, and "Load older" asks for the next page', async () => {
        const ctx = setup({ revisionsTruncated: true });
        await openHistory(ctx);
        assert.equal(one(ctx.history.el, 'data-plan-history-more').hidden, false);
        ctx.env.calls.length = 0;
        one(ctx.history.el, 'data-plan-history-more').click();
        await ctx.env.sleep(20);
        const more = ctx.env.callsOf('GET', /revisions/);
        assert.equal(more.length, 1);
        assert.match(more[0].url, /beforeRevision=3/, 'the next page starts below the oldest row shown');
        ctx.env.calls.length = 0;
        const filter = one(ctx.history.el, 'data-plan-history-filter');
        filter.value = 'ai';
        filter.dispatch('change');
        await ctx.env.sleep(20);
        const filtered = ctx.env.callsOf('GET', /revisions/);
        assert.equal(filtered.length, 1);
        assert.match(filtered[0].url, /actorKind=ai/);
        assert.deepEqual(byAttr(ctx.history.el, 'data-plan-revision').map((r) => r.getAttribute('data-plan-revision')), ['5']);
    });

    it('an empty history says so; a failed read shows the code', async () => {
        const empty = setup({ revisions: [] });
        await openHistory(empty);
        assert.ok(one(empty.history.el, 'data-plan-history-empty'));
        const failing = setup();
        failing.env.on('GET', /\/revisions/, () => failing.env.json(400, { error: 'Invalid request', code: 'invalid_op', field: 'limit' }));
        await openHistory(failing);
        const error = one(failing.history.el, 'data-plan-history-error');
        assert.equal(error.hidden, false);
        assert.match(text(error), /invalid_op/);
    });

    it('opening a revision reads it, then what it changed against the one before', async () => {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        ctx.env.calls.length = 0;
        one(one(ctx.history.el, 'data-plan-revision', '5'), 'data-plan-rev-open').click();
        await ctx.env.sleep(30);
        assert.equal(ctx.env.callsOf('GET', /revisions\/5$/).length, 1);
        const diffs = ctx.env.callsOf('GET', /\/diff/);
        assert.equal(diffs.length, 1);
        assert.match(diffs[0].url, /from=4&to=5/);
        assert.equal(one(ctx.history.el, 'data-plan-revision-detail').hidden, false);
    });
});

describe('ac-2: compare', () => {
    it('two revisions send one GET .../diff with the right from and to, and render one entry per server entry', async () => {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        ctx.env.calls.length = 0;
        one(ctx.history.el, 'data-plan-diff-from').value = '5';
        one(ctx.history.el, 'data-plan-diff-to').value = '4';
        one(ctx.history.el, 'data-plan-diff-run').click();
        await ctx.env.sleep(20);
        const diffs = ctx.env.callsOf('GET', /\/diff/);
        assert.equal(diffs.length, 1);
        assert.match(diffs[0].url, /from=5&to=4$/);
        const entries = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry');
        assert.equal(entries.length, DIFF.entries.length);
        assert.deepEqual(entries.map((e) => e.getAttribute('data-plan-diff-kind')), ['changed', 'changed', 'removed', 'added', 'moved']);
        const first = entries[0];
        assert.equal(text(one(first, 'data-plan-diff-before')).indexOf('לפני'), 0);
        assert.match(text(one(first, 'data-plan-diff-before')), /Before/);
        assert.match(text(one(first, 'data-plan-diff-before')), /שָׁלוֹם דָּנִי/);
        assert.match(text(one(first, 'data-plan-diff-after')), /שָׁלוֹם רוּת/);
    });

    it('Hebrew values are rtl and he; English are ltr and en', async () => {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        one(ctx.history.el, 'data-plan-diff-run').click();
        await ctx.env.sleep(20);
        const entries = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry');
        const hebrew = all(entries[0], (n) => n.tagName === 'BDI' && /שָׁלוֹם/.test(text(n)));
        assert.equal(hebrew.length, 2);
        hebrew.forEach((n) => { assert.equal(n.getAttribute('dir'), 'rtl'); assert.equal(n.getAttribute('lang'), 'he'); });
        const english = all(entries[1], (n) => n.tagName === 'BDI' && /\.mp3/.test(text(n)));
        assert.equal(english.length, 2);
        english.forEach((n) => { assert.equal(n.getAttribute('dir'), 'ltr'); assert.equal(n.getAttribute('lang'), 'en'); });
    });

    it('a revision against the working copy, and the working copy against published, each send one diff', async () => {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        ctx.env.calls.length = 0;
        one(one(ctx.history.el, 'data-plan-revision', '4'), 'data-plan-diff-quick', 'revision').click();
        await ctx.env.sleep(20);
        one(ctx.history.el, 'data-plan-diff-quick', 'working-published').click();
        await ctx.env.sleep(20);
        const urls = ctx.env.callsOf('GET', /\/diff/).map((c) => c.url.split('?')[1]);
        assert.deepEqual(urls, ['from=4&to=working', 'from=published&to=working']);
    });

    it('a path filter rides along as path=', async () => {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        ctx.env.calls.length = 0;
        one(ctx.history.el, 'data-plan-diff-path').value = LISTEN;
        one(ctx.history.el, 'data-plan-diff-run').click();
        await ctx.env.sleep(20);
        assert.match(ctx.env.callsOf('GET', /\/diff/)[0].url, new RegExp('&path=' + encodeURIComponent(LISTEN).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
    });

    it('a truncated result says so; a cut value is marked; an empty diff says there is none', async () => {
        const cut = Object.assign({}, DIFF, {
            truncated: true,
            entries: [{ path: LISTEN, field: 'passage', kind: 'changed', before: 'x…', after: 'y…', valueTruncated: true }]
        });
        const ctx = setup({ diff: cut });
        await openHistory(ctx);
        one(ctx.history.el, 'data-plan-diff-run').click();
        await ctx.env.sleep(20);
        const box = one(ctx.history.el, 'data-plan-diff');
        assert.ok(one(box, 'data-plan-diff-truncated'));
        assert.match(text(one(box, 'data-plan-diff-before')), /cut/);
        const none = setup({ diff: { from: { ref: 'published', revision: 2 }, to: { ref: 'working', revision: 3 }, entries: [], truncated: false } });
        await openHistory(none);
        one(none.history.el, 'data-plan-diff-run').click();
        await none.env.sleep(20);
        assert.ok(one(one(none.history.el, 'data-plan-diff'), 'data-plan-diff-empty'));
    });

    it('no_snapshot is shown in plain words', async () => {
        const ctx = setup();
        ctx.env.on('GET', /\/diff/, () => ctx.env.json(400, { error: 'Invalid request', code: 'no_snapshot' }));
        await openHistory(ctx);
        one(ctx.history.el, 'data-plan-diff-quick', 'working-published').click();
        await ctx.env.sleep(20);
        const error = one(ctx.history.el, 'data-plan-diff-error');
        assert.match(text(error), /no_snapshot/);
        assert.match(text(error), /Nothing has been published/);
    });

    it('restore controls follow the entries: field and part on a change, part on a removal, none on an addition or a move', async () => {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        one(ctx.history.el, 'data-plan-diff-run').click();
        await ctx.env.sleep(20);
        const entries = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry');
        const kinds = (entry) => byAttr(entry, 'data-plan-restore').map((b) => b.getAttribute('data-plan-restore'));
        assert.deepEqual(kinds(entries[0]), ['field', 'part']);
        assert.deepEqual(kinds(entries[2]), ['part']);
        assert.deepEqual(kinds(entries[3]), []);
        assert.deepEqual(kinds(entries[4]), []);
        // comparing from the working copy offers none at all
        const working = setup({ diff: Object.assign({}, DIFF, { from: { ref: 'working', revision: 6 }, to: { ref: 4, revision: 4 } }) });
        await openHistory(working);
        one(working.history.el, 'data-plan-diff-run').click();
        await working.env.sleep(20);
        assert.equal(byAttr(one(working.history.el, 'data-plan-diff'), 'data-plan-restore').length, 0);
    });
});

describe('ac-3: restore asks once, then sends one POST', () => {
    async function withDiff() {
        const ctx = setup({ diff: DIFF });
        await openHistory(ctx);
        one(ctx.history.el, 'data-plan-diff-run').click();
        await ctx.env.sleep(20);
        ctx.env.calls.length = 0;
        return ctx;
    }

    it('a click shows the confirmation and sends nothing; cancel sends nothing', async () => {
        const ctx = await withDiff();
        const entry = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry')[0];
        one(entry, 'data-plan-restore', 'field').click();
        const box = one(entry, 'data-plan-restore-box');
        assert.ok(box, 'a confirmation is shown');
        assert.match(text(box), /Restore the field/);
        assert.match(text(box), /history is kept/);
        assert.match(text(box), /לשחזר/);
        assert.equal(ctx.env.callsOf('POST', /restore/).length, 0);
        one(box, 'data-plan-restore-cancel').click();
        assert.equal(byAttr(entry, 'data-plan-restore-box').length, 0);
        assert.equal(ctx.env.callsOf('POST', /restore/).length, 0);
    });

    it('field, part, and whole each send one POST with fromRevision, path, field, and the last expectedRevision', async () => {
        const ctx = await withDiff();
        const entry = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry')[0];

        one(entry, 'data-plan-restore', 'field').click();
        one(one(entry, 'data-plan-restore-box'), 'data-plan-restore-confirm').click();
        await ctx.env.sleep(20);
        let posts = ctx.env.callsOf('POST', /restore/);
        assert.equal(posts.length, 1);
        assert.deepEqual(posts[0].body, { fromRevision: 5, expectedRevision: 6, path: LISTEN, field: 'passage' });
        assert.equal(ctx.events.restored.length, 1);

        const entry2 = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry')[0];
        one(entry2, 'data-plan-restore', 'part').click();
        one(one(entry2, 'data-plan-restore-box'), 'data-plan-restore-confirm').click();
        await ctx.env.sleep(20);
        posts = ctx.env.callsOf('POST', /restore/);
        assert.equal(posts.length, 2);
        assert.deepEqual(posts[1].body, { fromRevision: 5, expectedRevision: 7, path: LISTEN }, 'the revision moved on after the first restore');

        const row = one(ctx.history.el, 'data-plan-revision', '4');
        one(row, 'data-plan-restore', 'whole').click();
        one(one(row, 'data-plan-restore-box'), 'data-plan-restore-confirm').click();
        await ctx.env.sleep(20);
        posts = ctx.env.callsOf('POST', /restore/);
        assert.equal(posts.length, 3);
        assert.deepEqual(posts[2].body, { fromRevision: 4, expectedRevision: 8 });
    });

    it('one confirmation at a time, and a second click on Restore does not send a second POST', async () => {
        const ctx = await withDiff();
        const entries = byAttr(one(ctx.history.el, 'data-plan-diff'), 'data-plan-diff-entry');
        one(entries[0], 'data-plan-restore', 'field').click();
        one(entries[1], 'data-plan-restore', 'field').click();
        assert.equal(byAttr(ctx.history.el, 'data-plan-restore-box').length, 1, 'the first confirmation closed');
        const box = one(entries[1], 'data-plan-restore-box');
        const yes = one(box, 'data-plan-restore-confirm');
        yes.click();
        yes.click();
        await ctx.env.sleep(20);
        assert.equal(ctx.env.callsOf('POST', /restore/).length, 1);
    });

    it('a 409 shows revision_conflict and the stored revision, with a Reload that asks the host', async () => {
        const ctx = await withDiff();
        ctx.env.server.revision = 9;
        const row = one(ctx.history.el, 'data-plan-revision', '5');
        one(row, 'data-plan-restore', 'whole').click();
        const box = one(row, 'data-plan-restore-box');
        one(box, 'data-plan-restore-confirm').click();
        await ctx.env.sleep(20);
        const error = one(box, 'data-plan-restore-error');
        assert.equal(error.hidden, false);
        assert.match(text(error), /revision_conflict 9/);
        assert.equal(ctx.events.restored.length, 0);
        one(box, 'data-plan-history-reload').click();
        assert.equal(ctx.events.reloads, 1);
    });

    it('a validation refusal lists the rule codes and the plan view is not told it changed', async () => {
        const ctx = await withDiff();
        ctx.env.on('POST', /restore/, () => ctx.env.json(400, {
            error: 'validation', revision: 6,
            blocks: [{ rule: 'schema', severity: 'block', path: '/title', message: 'bad' }, { rule: 'html', severity: 'block', path: '/title', message: 'bad' }],
            warnings: [], informs: []
        }));
        const row = one(ctx.history.el, 'data-plan-revision', '5');
        one(row, 'data-plan-restore', 'whole').click();
        const box = one(row, 'data-plan-restore-box');
        one(box, 'data-plan-restore-confirm').click();
        await ctx.env.sleep(20);
        assert.match(text(one(box, 'data-plan-restore-error')), /validation.*schema, html/);
        assert.equal(ctx.events.restored.length, 0);
    });

    it('assign_required is explained', async () => {
        const ctx = await withDiff();
        ctx.env.on('POST', /restore/, () => ctx.env.json(400, { error: 'Invalid request', code: 'assign_required' }));
        const row = one(ctx.history.el, 'data-plan-revision', '5');
        one(row, 'data-plan-restore', 'whole').click();
        const box = one(row, 'data-plan-restore-box');
        one(box, 'data-plan-restore-confirm').click();
        await ctx.env.sleep(20);
        assert.match(text(one(box, 'data-plan-restore-error')), /assign_required.*assignment control/);
    });

    it('the confirmation warns when the form holds unsaved text', async () => {
        const ctx = await withDiff();
        ctx.setUnsaved(true);
        const row = one(ctx.history.el, 'data-plan-revision', '5');
        one(row, 'data-plan-restore', 'whole').click();
        assert.ok(one(row, 'data-plan-restore-unsaved'));
    });

    it('after a success the list is read again when the host did not refresh it', async () => {
        const ctx = await withDiff();
        const row = one(ctx.history.el, 'data-plan-revision', '5');
        one(row, 'data-plan-restore', 'whole').click();
        one(one(row, 'data-plan-restore-box'), 'data-plan-restore-confirm').click();
        await ctx.env.sleep(30);
        assert.equal(ctx.env.callsOf('GET', /revisions$/).length, 1);
    });
});

describe('ac-4: the "edited by a person" mark is information, never a lock', () => {
    const ENTRY = {
        path: LISTEN, field: 'passage', revision: 4, at: '2026-10-12T21:00:00.000Z', by: 'example-admin',
        previousValue: 'שָׁלוֹם דָּנִי', previousWriter: 'ai'
    };

    function editorWithMarks(entries, extra) {
        const restores = [];
        const made = makeEditor(Object.assign({ onRestoreValue: (spec) => restores.push(spec) }, extra || {}));
        const plan = samplePlan();
        const block = plan.units[0].lessons[0].blocks.find((b) => b.id === 'listening');
        made.editor.setEditState(entries);
        const form = made.editor.renderNode({ path: LISTEN, node: block, kind: 'block' });
        made.host.appendChild(form.el);
        return { made, form, restores, block };
    }

    it('the field whose last writer is a person carries the mark; the others do not', () => {
        const { form } = editorWithMarks([ENTRY]);
        const row = form.fields.passage.row;
        const mark = one(row, 'data-plan-edited-mark');
        assert.ok(mark);
        assert.match(text(mark), /נערך על ידי אדם/);
        assert.match(text(mark), /Edited by a person/);
        assert.equal(mark.getAttribute('aria-expanded'), 'false');
        assert.equal(byAttr(form.fields.audioPath ? form.fields.audioPath.row : form.el, 'data-plan-edited-mark').length, form.fields.audioPath ? 0 : 1);
        assert.equal(byAttr(form.el, 'data-plan-edited-mark').length, 1);
    });

    it('activating the mark shows the earlier value and a restore button for revision - 1', () => {
        const { form, restores } = editorWithMarks([ENTRY]);
        const row = form.fields.passage.row;
        const detail = one(row, 'data-plan-edited-detail');
        assert.equal(detail.hidden, true);
        one(row, 'data-plan-edited-mark').click();
        assert.equal(detail.hidden, false);
        assert.equal(one(row, 'data-plan-edited-mark').getAttribute('aria-expanded'), 'true');
        const prev = one(row, 'data-plan-prev-value');
        assert.equal(text(prev), 'שָׁלוֹם דָּנִי');
        assert.equal(prev.getAttribute('dir'), 'rtl');
        assert.equal(prev.getAttribute('lang'), 'he');
        assert.match(text(detail), /example-admin/);
        one(row, 'data-plan-restore', 'value').click();
        assert.equal(restores.length, 1);
        assert.deepEqual({ path: restores[0].path, field: restores[0].field, fromRevision: restores[0].fromRevision },
            { path: LISTEN, field: 'passage', fromRevision: 3 });
        assert.equal(restores[0].anchor, detail, 'the confirmation goes under the mark');
    });

    it('a large earlier value is said to be too large, and the restore button stays', () => {
        const { form } = editorWithMarks([Object.assign({}, ENTRY, { previousValue: null, previousValueOmitted: true })]);
        const row = form.fields.passage.row;
        assert.match(text(one(row, 'data-plan-prev-value')), /too large/);
        assert.ok(one(row, 'data-plan-restore', 'value'));
    });

    it('setEditState marks fields already on screen, and clearing the state removes the mark', () => {
        const { made, form } = editorWithMarks([]);
        assert.equal(byAttr(form.el, 'data-plan-edited-mark').length, 0);
        made.editor.setEditState([ENTRY]);
        assert.equal(byAttr(form.el, 'data-plan-edited-mark').length, 1);
        made.editor.setEditState([]);
        assert.equal(byAttr(form.el, 'data-plan-edited-mark').length, 0);
    });

    it('a nested sub-field and an unknown path do not throw and mark the field they sit in', () => {
        const { form } = editorWithMarks([
            { path: LISTEN, field: 'questions/0/question', revision: 3 },
            { path: 'units/nope', field: 'title', revision: 3 },
            { path: LISTEN, field: '', revision: 3 },
            null
        ]);
        assert.equal(byAttr(form.el, 'data-plan-edited-mark').length, 1);
    });

    it('no control is disabled, read only, or hidden because of a mark', () => {
        const entries = [ENTRY, { path: LISTEN, field: 'audioPath', revision: 4, previousValue: 'a.mp3' }];
        const { form } = editorWithMarks(entries);
        const controls = all(form.el, (n) => ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'].indexOf(n.tagName) !== -1);
        assert.ok(controls.length > 10);
        const refusing = (f) => all(f.el, (n) => ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'].indexOf(n.tagName) !== -1)
            .filter((c) => c.disabled === true || c.hasAttribute('readonly') || c.hasAttribute('disabled'))
            .map((c) => c.tagName + ' ' + (c.getAttribute('data-plan-field') || c.getAttribute('data-plan-act') || ''));
        // The only disabled controls are the end-of-list move buttons; the same form with no marks has the same ones.
        const plain = editorWithMarks([]).form;
        assert.deepEqual(refusing(form).map((s) => s.replace(/\d+$/, '')), refusing(plain).map((s) => s.replace(/\d+$/, '')));
        refusing(form).forEach((s) => assert.match(s, /^BUTTON/));
        // the marked field has its save button and its editable control, like the unmarked ones
        const row = form.fields.passage.row;
        assert.equal(form.fields.passage.control.disabled, false);
        assert.ok(one(row, 'data-plan-save'));
        assert.equal(one(row, 'data-plan-save').disabled, false);
    });
});

describe('the builder wires the marks, the history panel, and the restore together', () => {
    const MARK = {
        path: LISTEN, field: 'passage', revision: 4, at: '2026-10-12T21:00:00.000Z', by: 'example-admin',
        previousValue: 'שָׁלוֹם דָּנִי', previousWriter: 'ai'
    };
    const editState = { advisory: true, lastHumanEdit: { revision: 4, at: MARK.at }, humanEdited: [MARK], more: 0 };

    async function mounted() {
        const env = loadPanels({ revisions: REVS, editState, diff: DIFF });
        env.server.revision = 6;
        env.server.plan.revision = 6;
        const panel = env.panel();
        await env.W.HebrewPlanBuilder.mount(panel, 'sample');
        await env.W.HebrewPlanBuilder.settled();
        one(one(panel, 'data-plan-block', 'listening'), 'data-plan-edit').click();
        await env.W.HebrewPlanBuilder.settled();
        return { env, panel };
    }

    it('a field a person last wrote shows the mark after the plan loads', async () => {
        const { panel } = await mounted();
        const marks = byAttr(panel, 'data-plan-edited-mark');
        assert.equal(marks.length, 1);
        assert.ok(one(panel, 'data-plan-history'), 'the history panel is in the plan view');
        assert.equal(byAttr(panel, 'data-plan-history-list').length, 1);
    });

    it('"Restore the earlier value" confirms once, posts once with the builder\'s revision, and reloads at the new one', async () => {
        const { env, panel } = await mounted();
        const mark = one(panel, 'data-plan-edited-mark');
        mark.click();
        one(panel, 'data-plan-restore', 'value').click();
        const box = one(panel, 'data-plan-restore-box');
        assert.ok(box);
        assert.equal(env.callsOf('POST', /restore/).length, 0, 'nothing is sent before the confirmation');
        env.calls.length = 0;
        one(box, 'data-plan-restore-confirm').click();
        await env.sleep(30);
        await env.W.HebrewPlanBuilder.settled();
        await env.sleep(30);
        const posts = env.callsOf('POST', /restore/);
        assert.equal(posts.length, 1);
        assert.deepEqual(posts[0].body, { fromRevision: 3, expectedRevision: 6, path: LISTEN, field: 'passage' });
        assert.equal(env.callsOf('GET', /\/parts$/).length >= 1, true, 'the plan is read again');
        assert.equal(text(one(panel, 'data-plan-revision')), '7', 'the view is at the new revision');
        assert.match(text(one(panel, 'data-plan-notice')), /Restored/);
    });

    it('a revision restored from the list reloads the plan, and the confirmation names a 409', async () => {
        const { env, panel } = await mounted();
        one(one(panel, 'data-plan-history-toggle').parentNode, 'data-plan-history-toggle').click();
        await env.sleep(30);
        const row = one(one(panel, 'data-plan-history'), 'data-plan-revision', '5');
        one(row, 'data-plan-restore', 'whole').click();
        env.server.revision = 8; // someone else wrote in between
        one(one(row, 'data-plan-restore-box'), 'data-plan-restore-confirm').click();
        await env.sleep(30);
        assert.match(text(one(row, 'data-plan-restore-error')), /revision_conflict 8/);
        one(row, 'data-plan-history-reload').click();
        await env.W.HebrewPlanBuilder.settled();
        await env.sleep(30);
        assert.equal(text(one(panel, 'data-plan-revision')), '8', 'Reload takes the server copy');
    });

    it('typing is never blocked by the mark: the marked field takes text and saves', async () => {
        const { env, panel } = await mounted();
        const field = one(panel, 'data-plan-field', 'passage');
        assert.equal(field.disabled, false);
        field.value = 'new text from the admin';
        field.dispatch('input');
        const save = one(field.parentNode, 'data-plan-save');
        save.click();
        await env.sleep(30);
        await env.W.HebrewPlanBuilder.settled();
        const patches = env.callsOf('PATCH', /fields$/);
        assert.equal(patches.length, 1);
        assert.deepEqual(patches[0].body, { path: LISTEN, field: 'passage', value: 'new text from the admin', expectedRevision: 6 });
    });
});

describe('words and the source', () => {
    const files = ['plan-history.js', 'plan-issues.js', 'plan-preview.js', 'plan-list.js', 'plan-ui.js']
        .map((name) => path.resolve(__dirname, '..', 'shared', 'admin', name));

    it('the words "pinned" and "locked" appear in no new string', () => {
        files.forEach((file) => assert.doesNotMatch(fs.readFileSync(file, 'utf8'), /\b(pinned|locked|pin|lock)\b/i, path.basename(file)));
        const form = fs.readFileSync(path.resolve(__dirname, '..', 'shared', 'admin', 'plan-form.js'), 'utf8');
        const mark = form.slice(form.indexOf('"edited by a person"'), form.indexOf('// ---- drafts, issues, focus'));
        assert.doesNotMatch(mark, /\b(pinned|locked)\b/i);
    });

    it('the history panel holds no diff or restore logic of its own', () => {
        const source = fs.readFileSync(path.resolve(__dirname, '..', 'shared', 'admin', 'plan-history.js'), 'utf8');
        assert.doesNotMatch(source, /deepEqual|diffDocuments|JSON\.parse/);
        assert.equal((source.match(/Api\.post\(/g) || []).length, 1, 'one restore call');
    });

    it('no new string has a point, streak, or joke', () => {
        files.concat([path.resolve(__dirname, '..', 'shared', 'admin', 'plan-form.js')]).forEach((file) => {
            assert.doesNotMatch(fs.readFileSync(file, 'utf8'), /\b(points?|streaks?|xp|level up|well done|great job)\b/i, path.basename(file));
        });
    });
});
