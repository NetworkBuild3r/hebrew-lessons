/**
 * Plan operations and publish, on a temporary database.
 * Provenance: INIT-004/SPEC-003
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { withCtx, publishWithCtx } = require('./helpers/plan-ctx');

const root = path.resolve(__dirname, '..');
const SAMPLE = path.join(
    root,
    'docs',
    'decisions',
    'ADR-0003-assets',
    'samples',
    'beginner-unit-1.plan.json'
);
const pretty = fs.readFileSync(SAMPLE, 'utf8');

const WORD = 'units/unit-1/lessons/cafe-order/blocks/introduction/words/2';
const LESSON = 'units/unit-1/lessons/cafe-order';
const LISTEN = LESSON + '/blocks/listen';
const DIALOGUE = LESSON + '/blocks/dialogue';
const EXTRA = {
    id: 'extra',
    type: 'listening',
    passage: 'אֲנִי רוֹצֶה קָפֶה.'
};

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const modules = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/plans/store',
        '../server/plans/validate',
        '../server/plans/ops',
        '../server/plans/publish'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    const store = require('../server/plans/store');
    const ops = require('../server/plans/ops');
    const publish = require('../server/plans/publish');
    return { db, store, ops, publish };
}

function watch(store) {
    const calls = { saveWorkingCopy: 0, saveSnapshot: 0 };
    return {
        calls: calls,
        loadWorkingCopy: (id) => store.loadWorkingCopy(id),
        saveWorkingCopy: (input) => {
            calls.saveWorkingCopy += 1;
            return store.saveWorkingCopy(input);
        },
        loadSnapshot: (id) => store.loadSnapshot(id),
        saveSnapshot: (id, snapshot) => {
            calls.saveSnapshot += 1;
            return store.saveSnapshot(id, snapshot);
        }
    };
}

function assertOnly(result, beforeText, edit) {
    assert.equal(result.error, undefined);
    const expected = JSON.parse(beforeText);
    edit(expected);
    expected.revision = JSON.parse(beforeText).revision + 1;
    expected.status = 'draft';
    assert.deepEqual(result.plan, expected);
    assert.equal(result.document, JSON.stringify(result.plan));
    assert.equal(result.revision, expected.revision);
    assert.equal(result.status, 'draft');
}

describe('plan operations and publish', { concurrency: false }, () => {
    let tmp;
    let lessons;
    let db;
    let store;
    let ops;
    let publishPlan;
    let apply;
    const savedLessons = process.env.LESSONS_DIR;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-ops-'));
        lessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-ops-lessons-'));
        fs.writeFileSync(path.join(lessons, 'marker.txt'), 'untouched');
        process.env.LESSONS_DIR = lessons;
        ({ db, store, ops } = freshModules(tmp));
        // INIT-008/SPEC-002: every write takes a context; these calls add one and nothing else.
        ops = withCtx(ops);
        publishPlan = publishWithCtx(require('../server/plans/publish').publishPlan);
        apply = ops.apply;
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        fs.rmSync(lessons, { recursive: true, force: true });
        if (savedLessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = savedLessons;
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    beforeEach(() => {
        db.run('DELETE FROM plans');
    });

    function seed(document) {
        const text = document == null ? pretty : document;
        const plan = JSON.parse(text);
        store.saveWorkingCopy({
            id: plan.id,
            document: text,
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
        return store.loadWorkingCopy(plan.id);
    }

    function conflictLeaves(first, again, watched) {
        assert.deepEqual(again, { error: 'revision_conflict', revision: first.revision });
        const row = store.loadWorkingCopy('beginner-unit-1');
        assert.equal(row.document, first.document);
        assert.equal(row.revision, first.revision);
        assert.equal(row.updatedAt, first.updatedAt || row.updatedAt);
        assert.equal(watched.calls.saveWorkingCopy, 1);
        assert.equal(watched.calls.saveSnapshot, 0);
    }

    it('ac-1: setField updates only that field, then a stale revision does not write', () => {
        seed();
        const watched = watch(store);
        const first = ops.setField(watched, 'beginner-unit-1', {
            path: WORD,
            field: 'meaning',
            value: 'espresso',
            expectedRevision: 1
        });
        assertOnly(first, pretty, (plan) => {
            plan.units[0].lessons[0].blocks[0].words[1].meaning = 'espresso';
        });
        assert.equal(first.plan.units[0].lessons[0].blocks[0].words[0].meaning, 'wants (m. sg.)');
        const stored = store.loadWorkingCopy('beginner-unit-1');
        const again = ops.setField(watched, 'beginner-unit-1', {
            path: WORD,
            field: 'meaning',
            value: 'changed again',
            expectedRevision: 1
        });
        conflictLeaves(
            { document: stored.document, revision: stored.revision, updatedAt: stored.updatedAt },
            again,
            watched
        );
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document.includes('changed again'), false);
    });

    it('ac-1: replacePart swaps one block and a stale revision keeps those bytes', () => {
        seed();
        const watched = watch(store);
        const block = { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' };
        const first = ops.replacePart(watched, 'beginner-unit-1', {
            path: LISTEN,
            value: block,
            expectedRevision: 1
        });
        assertOnly(first, pretty, (plan) => {
            plan.units[0].lessons[0].blocks[2] = block;
        });
        const stored = store.loadWorkingCopy('beginner-unit-1');
        const again = ops.replacePart(watched, 'beginner-unit-1', {
            path: LISTEN,
            value: { id: 'listen', type: 'listening', passage: 'שׁוּב' },
            expectedRevision: 1
        });
        conflictLeaves(
            { document: stored.document, revision: stored.revision, updatedAt: stored.updatedAt },
            again,
            watched
        );
    });

    it('ac-1: insertBlock inserts at the destination index only', () => {
        seed();
        const watched = watch(store);
        const first = ops.insertBlock(watched, 'beginner-unit-1', {
            path: LESSON,
            index: 1,
            block: EXTRA,
            expectedRevision: 1
        });
        assertOnly(first, pretty, (plan) => {
            plan.units[0].lessons[0].blocks.splice(1, 0, EXTRA);
        });
        assert.equal(first.plan.units[0].lessons[0].blocks.length, 5);
        const stored = store.loadWorkingCopy('beginner-unit-1');
        const again = ops.insertBlock(watched, 'beginner-unit-1', {
            path: LESSON,
            index: 0,
            block: { id: 'other', type: 'listening', passage: 'שׁוּב' },
            expectedRevision: 1
        });
        conflictLeaves(
            { document: stored.document, revision: stored.revision, updatedAt: stored.updatedAt },
            again,
            watched
        );
    });

    it('ac-1: moveBlock moves one block and a stale revision keeps that order', () => {
        seed();
        const watched = watch(store);
        const first = ops.moveBlock(watched, 'beginner-unit-1', {
            path: DIALOGUE,
            index: 0,
            expectedRevision: 1
        });
        assertOnly(first, pretty, (plan) => {
            const blocks = plan.units[0].lessons[0].blocks;
            const item = blocks.splice(3, 1)[0];
            blocks.splice(0, 0, item);
        });
        assert.deepEqual(
            first.plan.units[0].lessons[0].blocks.map((block) => block.id),
            ['dialogue', 'introduction', 'story', 'listen']
        );
        const stored = store.loadWorkingCopy('beginner-unit-1');
        const again = ops.moveBlock(watched, 'beginner-unit-1', {
            path: DIALOGUE,
            index: 2,
            expectedRevision: 1
        });
        conflictLeaves(
            { document: stored.document, revision: stored.revision, updatedAt: stored.updatedAt },
            again,
            watched
        );
    });

    it('ac-1: removeBlock removes one block and a stale revision keeps the rest', () => {
        seed();
        const watched = watch(store);
        const first = ops.removeBlock(watched, 'beginner-unit-1', {
            path: DIALOGUE,
            expectedRevision: 1
        });
        assertOnly(first, pretty, (plan) => {
            plan.units[0].lessons[0].blocks.splice(3, 1);
        });
        assert.equal(first.plan.units[0].lessons[0].blocks.some((block) => block.id === 'dialogue'), false);
        const stored = store.loadWorkingCopy('beginner-unit-1');
        const again = ops.removeBlock(watched, 'beginner-unit-1', {
            path: LISTEN,
            expectedRevision: 1
        });
        conflictLeaves(
            { document: stored.document, revision: stored.revision, updatedAt: stored.updatedAt },
            again,
            watched
        );
    });

    it('ac-1: readPart returns the addressed part and does not write or bump the revision', () => {
        seed();
        const watched = watch(store);
        const first = ops.readPart(watched, 'beginner-unit-1', { path: WORD });
        assert.equal(first.part.id, '2');
        assert.equal(first.part.meaning, 'coffee');
        assert.equal(first.revision, 1);
        first.part.meaning = 'changed';
        const again = ops.readPart(watched, 'beginner-unit-1', { path: WORD });
        assert.equal(again.part.meaning, 'coffee');
        assert.equal(again.revision, 1);
        assert.equal(again.error, undefined);
        const row = store.loadWorkingCopy('beginner-unit-1');
        assert.equal(row.document, pretty);
        assert.equal(row.revision, 1);
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(watched.calls.saveSnapshot, 0);
    });

    it('ac-2: publish freezes the snapshot, and a later setField drafts only the working copy', () => {
        seed();
        const published = publishPlan(store, 'beginner-unit-1', { expectedRevision: 1 });
        assert.equal(published.error, undefined);
        assert.equal(published.revision, 2);
        assert.equal(published.status, 'published');
        assert.equal(published.validation.ok, true);
        assert.equal(published.snapshot, published.document);
        assert.equal(store.loadSnapshot('beginner-unit-1'), published.document);
        const frozen = JSON.parse(published.snapshot);
        assert.equal(frozen.status, 'published');
        assert.equal(frozen.revision, 2);
        assert.equal(frozen.units[0].lessons[0].blocks[0].words[1].meaning, 'coffee');

        const edited = ops.setField(store, 'beginner-unit-1', {
            path: WORD,
            field: 'meaning',
            value: 'espresso',
            expectedRevision: 2
        });
        assert.equal(edited.error, undefined);
        assert.equal(edited.revision, 3);
        assert.equal(edited.status, 'draft');
        assert.equal(edited.plan.units[0].lessons[0].blocks[0].words[1].meaning, 'espresso');
        assert.equal(store.loadSnapshot('beginner-unit-1'), published.snapshot);
        const still = JSON.parse(store.loadSnapshot('beginner-unit-1'));
        assert.equal(still.status, 'published');
        assert.equal(still.units[0].lessons[0].blocks[0].words[1].meaning, 'coffee');
        assert.notEqual(store.loadWorkingCopy('beginner-unit-1').document, published.snapshot);
    });

    it('ac-3: publish of a block-severity failure does not write a snapshot', () => {
        const row = seed();
        const words = JSON.parse(pretty).units[0].lessons[0].newWords.concat(['סֶפֶל']);
        const drafted = ops.setField(store, 'beginner-unit-1', {
            path: LESSON,
            field: 'newWords',
            value: words,
            expectedRevision: 1
        });
        assert.equal(drafted.error, undefined);
        assert.equal(drafted.validation.ok, false);
        assert.equal(drafted.validation.blocks.some((item) => item.rule === 'N1'), true);
        const stored = store.loadWorkingCopy('beginner-unit-1');
        assert.equal(stored.snapshot, null);
        assert.equal(stored.document.includes('סֶפֶל'), true);

        const watched = watch(store);
        const published = publishPlan(watched, 'beginner-unit-1', { expectedRevision: stored.revision });
        assert.equal(published.error, 'validation');
        assert.equal(published.blocks.some((item) => item.rule === 'N1'), true);
        const after = store.loadWorkingCopy('beginner-unit-1');
        assert.equal(after.snapshot, null);
        assert.equal(store.loadSnapshot('beginner-unit-1'), null);
        assert.equal(after.document, stored.document);
        assert.equal(after.revision, stored.revision);
        assert.equal(after.updatedAt, stored.updatedAt);
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(watched.calls.saveSnapshot, 0);
        assert.equal(row.id, 'beginner-unit-1');
    });

    it('ac-4: a stale revision leaves the caller plan unchanged and does not write', () => {
        const caller = JSON.parse(pretty);
        const before = JSON.stringify(caller);
        const result = apply(caller, {
            op: 'set_field',
            path: WORD,
            field: 'meaning',
            value: 'changed'
        }, 99);
        assert.deepEqual(result, { conflict: { error: 'revision_conflict', revision: 1 } });
        assert.equal(JSON.stringify(caller), before);

        seed();
        const watched = watch(store);
        const stored = ops.setField(watched, 'beginner-unit-1', {
            path: WORD,
            field: 'meaning',
            value: 'changed',
            expectedRevision: 99
        });
        assert.deepEqual(stored, { error: 'revision_conflict', revision: 1 });
        assert.equal(JSON.stringify(caller), before);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(watched.calls.saveSnapshot, 0);
    });

    it('setField writes alt onto the picture, and an unknown field is not stored', () => {
        seed();
        const alt = ops.setField(store, 'beginner-unit-1', {
            path: WORD,
            field: 'alt',
            value: 'a cup of coffee',
            expectedRevision: 1
        });
        const word = alt.plan.units[0].lessons[0].blocks[0].words[1];
        assert.equal(word.image.alt, 'a cup of coffee');
        assert.equal(word.alt, undefined);
        assert.equal(word.image.mediaId, 'coffee-cup');

        db.run('DELETE FROM plans');
        seed();
        const watched = watch(store);
        const unknown = ops.setField(watched, 'beginner-unit-1', {
            path: WORD,
            field: 'nope',
            value: 'x',
            expectedRevision: 1
        });
        assert.equal(unknown.error, 'validation');
        assert.equal(unknown.blocks.some((item) => item.rule === 'schema'), true);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);
        assert.equal(watched.calls.saveWorkingCopy, 0);
    });

    it('security: a conflict and a block-severity edit perform no UPDATE', () => {
        seed();
        const watched = watch(store);
        const html = ops.setField(watched, 'beginner-unit-1', {
            path: '',
            field: 'title',
            value: '<b>no</b>',
            expectedRevision: 1
        });
        assert.equal(html.error, 'validation');
        assert.equal(html.blocks.some((item) => item.rule === 'html'), true);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);
        assert.equal(watched.calls.saveWorkingCopy, 0);

        const audio = ops.setField(watched, 'beginner-unit-1', {
            path: LISTEN,
            field: 'audioPath',
            value: 'https://evil.example/a.mp3',
            expectedRevision: 1
        });
        assert.equal(audio.error, 'validation');
        assert.equal(audio.blocks.some((item) => item.rule === 'audio-path'), true);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);

        const polluted = ops.setField(watched, 'beginner-unit-1', {
            path: 'units/unit-1',
            field: '__proto__',
            value: { polluted: true },
            expectedRevision: 1
        });
        assert.equal(polluted.error, 'invalid_op');
        assert.equal(({}).polluted, undefined);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);

        const ctor = ops.setField(watched, 'beginner-unit-1', {
            path: WORD,
            field: 'constructor',
            value: { polluted: true },
            expectedRevision: 1
        });
        assert.equal(ctor.error, 'validation');
        assert.equal(({}).polluted, undefined);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);

        const status = ops.setField(watched, 'beginner-unit-1', {
            path: '',
            field: 'status',
            value: 'published',
            expectedRevision: 1
        });
        assert.equal(status.error, 'invalid_op');
        assert.equal(store.loadSnapshot('beginner-unit-1'), null);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').status, 'draft');
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(watched.calls.saveSnapshot, 0);
    });

    it('security: publish does not snapshot HTML that was stored past the editor', () => {
        const broken = JSON.parse(pretty);
        broken.title = '<b>no</b>';
        const text = JSON.stringify(broken);
        store.saveWorkingCopy({
            id: broken.id,
            document: text,
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        const before = store.loadWorkingCopy(broken.id);
        const watched = watch(store);
        const published = publishPlan(watched, broken.id, { expectedRevision: 1 });
        assert.equal(published.error, 'validation');
        assert.equal(published.blocks.some((item) => item.rule === 'html'), true);
        const after = store.loadWorkingCopy(broken.id);
        assert.equal(after.document, text);
        assert.equal(after.snapshot, null);
        assert.equal(after.revision, 1);
        assert.equal(after.updatedAt, before.updatedAt);
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(watched.calls.saveSnapshot, 0);
    });

    it('a stale publish does not replace the snapshot', () => {
        seed();
        const published = publishPlan(store, 'beginner-unit-1', { expectedRevision: 1 });
        const watched = watch(store);
        const again = publishPlan(watched, 'beginner-unit-1', { expectedRevision: 1 });
        assert.deepEqual(again, { error: 'revision_conflict', revision: 2 });
        assert.equal(store.loadSnapshot('beginner-unit-1'), published.snapshot);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, published.document);
        assert.equal(store.loadWorkingCopy('beginner-unit-1').revision, 2);
        assert.equal(watched.calls.saveWorkingCopy, 0);
        assert.equal(watched.calls.saveSnapshot, 0);
    });

    it('personal audience and assigneeId change only through assignPlan, and progress is not read', () => {
        const plan = JSON.parse(pretty);
        plan.audience = 'personal';
        plan.assigneeId = 'learner-01';
        const text = JSON.stringify(plan);
        store.saveWorkingCopy({
            id: plan.id,
            document: text,
            revision: 1,
            status: 'draft',
            audience: 'personal',
            assigneeId: 'learner-01'
        });
        const titled = ops.setField(store, plan.id, {
            path: '',
            field: 'title',
            value: 'יחידה חדשה',
            expectedRevision: 1
        });
        assert.equal(titled.error, undefined);
        assert.equal(titled.plan.title, 'יחידה חדשה');
        assert.equal(titled.plan.audience, 'personal');
        assert.equal(titled.plan.assigneeId, 'learner-01');
        assert.equal(store.loadWorkingCopy(plan.id).assigneeId, 'learner-01');

        const reassigned = ops.setField(store, plan.id, {
            path: '',
            field: 'assigneeId',
            value: 'learner-02',
            expectedRevision: 2
        });
        // INIT-008/SPEC-002: set_field can no longer reach assigneeId (was: it could).
        assert.equal(reassigned.error, 'assign_required');
        assert.equal(store.loadWorkingCopy(plan.id).assigneeId, 'learner-01');
        assert.equal(store.loadWorkingCopy(plan.id).audience, 'personal');
        assert.equal(store.loadWorkingCopy(plan.id).revision, 2);

        const opsSrc = fs.readFileSync(path.join(root, 'server/plans/ops.js'), 'utf8');
        const pubSrc = fs.readFileSync(path.join(root, 'server/plans/publish.js'), 'utf8');
        for (const name of ['review_items', 'practice_events', 'lesson_progress', 'spikes/', 'password_hash']) {
            assert.equal(opsSrc.includes(name), false, name);
            assert.equal(pubSrc.includes(name), false, name);
        }
        assert.equal(pubSrc.includes('.splice('), false);
        assert.equal(opsSrc.includes('function apply'), true);
    });

    it('a missing plan and an out-of-range insert do not write', () => {
        const missing = ops.setField(store, 'missing-plan', {
            path: '',
            field: 'title',
            value: 'x',
            expectedRevision: 1
        });
        assert.deepEqual(missing, { error: 'not_found' });
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans WHERE id = ?', 'missing-plan').n, 0);

        seed();
        const watched = watch(store);
        const range = ops.insertBlock(watched, 'beginner-unit-1', {
            path: LESSON,
            index: 99,
            block: EXTRA,
            expectedRevision: 1
        });
        assert.equal(range.error, 'invalid_op');
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);
        const word = ops.removeBlock(watched, 'beginner-unit-1', {
            path: WORD,
            expectedRevision: 1
        });
        assert.equal(word.error, 'not_found');
        assert.equal(store.loadWorkingCopy('beginner-unit-1').document, pretty);
        assert.equal(watched.calls.saveWorkingCopy, 0);

        store.saveWorkingCopy({
            id: 'not-json',
            document: '{',
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        const bad = ops.setField(store, 'not-json', {
            path: '',
            field: 'title',
            value: 'x',
            expectedRevision: 1
        });
        assert.equal(bad.error, 'invalid_plan');
        assert.equal(store.loadWorkingCopy('not-json').document, '{');
    });

    it('insert, move, and remove share one revision sequence', () => {
        seed();
        const inserted = ops.insertBlock(store, 'beginner-unit-1', {
            path: LESSON,
            index: 1,
            block: EXTRA,
            expectedRevision: 1
        });
        assert.equal(inserted.plan.units[0].lessons[0].blocks[1].id, 'extra');
        const moved = ops.moveBlock(store, 'beginner-unit-1', {
            path: LESSON + '/blocks/extra',
            index: 3,
            expectedRevision: 2
        });
        assert.deepEqual(
            moved.plan.units[0].lessons[0].blocks.map((block) => block.id),
            ['introduction', 'story', 'listen', 'extra', 'dialogue']
        );
        const removed = ops.removeBlock(store, 'beginner-unit-1', {
            path: LESSON + '/blocks/extra',
            expectedRevision: 3
        });
        assert.equal(removed.plan.units[0].lessons[0].blocks.some((block) => block.id === 'extra'), false);
        assert.equal(removed.revision, 4);
        assert.equal(store.loadSnapshot('beginner-unit-1'), null);
    });
});
