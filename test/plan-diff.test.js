/**
 * Field-level diff of plan documents and revisions (INIT-008/SPEC-005).
 * Temporary database only. No network, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-005
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { planEnv, samplePlan, PLAN_ID, LESSON, WORD } = require('./helpers/plan-db');
const { diffDocuments } = require('../server/plans/diff');
const { basePlan } = require('./helpers/plan-fixtures');

const ROOT = path.resolve(__dirname, '..');

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function deepFreeze(value) {
    if (value && typeof value === 'object') {
        Object.freeze(value);
        for (const child of Object.values(value)) deepFreeze(child);
    }
    return value;
}

/** The same document with every object's keys in reverse order. */
function reverseKeys(value) {
    if (Array.isArray(value)) return value.map(reverseKeys);
    if (value && typeof value === 'object') {
        const out = {};
        for (const key of Object.keys(value).reverse()) out[key] = reverseKeys(value[key]);
        return out;
    }
    return value;
}

function fixtures() {
    const list = [];
    const samples = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples');
    for (const name of fs.readdirSync(samples)) {
        if (name.endsWith('.plan.json')) list.push(JSON.parse(fs.readFileSync(path.join(samples, name), 'utf8')));
    }
    const v11 = path.join(ROOT, 'test', 'fixtures', 'plans-1.1');
    for (const name of fs.readdirSync(v11)) {
        if (name.endsWith('.json')) list.push(JSON.parse(fs.readFileSync(path.join(v11, name), 'utf8')));
    }
    list.push(basePlan());
    return list;
}

describe('diffDocuments (INIT-008/SPEC-005)', () => {
    it('ac-1: a changed meaning is one changed entry at the word id path', () => {
        const a = samplePlan();
        const b = clone(a);
        b.units[0].lessons[0].blocks[0].words[1].meaning = 'espresso';
        const { entries, truncated } = diffDocuments(a, b);
        assert.equal(truncated, false);
        assert.deepEqual(entries, [{
            path: WORD, field: 'meaning', kind: 'changed', before: 'coffee', after: 'espresso'
        }]);
    });

    it('ac-1: an added block is added, a removed word is removed', () => {
        const a = samplePlan();
        const b = clone(a);
        const extra = { id: 'extra', type: 'listening', passage: 'תּוֹדָה.' };
        b.units[0].lessons[0].blocks.splice(1, 0, extra);
        const added = diffDocuments(a, b).entries;
        assert.deepEqual(added.filter((e) => e.kind === 'added'), [
            { path: LESSON + '/blocks/extra', field: '', kind: 'added', after: extra }
        ]);
        // The new block shifts nothing else, so the common blocks keep their order.
        assert.equal(added.filter((e) => e.kind === 'moved').length, 0);
        assert.equal(added.length, 1);

        const c = clone(a);
        const gone = c.units[0].lessons[0].blocks[0].words.splice(1, 1)[0];
        const removed = diffDocuments(a, c).entries;
        assert.deepEqual(removed, [{ path: WORD, field: '', kind: 'removed', before: gone }]);
    });

    it('ac-1: a reordered block list is one moved entry', () => {
        const a = samplePlan();
        const b = clone(a);
        const lesson = b.units[0].lessons[0];
        lesson.blocks.push(lesson.blocks.shift());
        const { entries } = diffDocuments(a, b);
        assert.equal(entries.length, 1);
        assert.deepEqual(entries[0], {
            path: LESSON,
            field: 'blocks',
            kind: 'moved',
            before: ['introduction', 'story', 'listen', 'dialogue'],
            after: ['story', 'listen', 'dialogue', 'introduction']
        });
    });

    it('ac-1: an unchanged plan and a key-order difference are an empty list', () => {
        const a = samplePlan();
        assert.deepEqual(diffDocuments(a, clone(a)), { entries: [], truncated: false });
        assert.deepEqual(diffDocuments(a, reverseKeys(a)).entries, []);
    });

    it('ac-1: diffDocuments(a, a) is empty for every fixture (property)', () => {
        const all = fixtures();
        assert.equal(all.length >= 15, true);
        for (const doc of all) {
            assert.deepEqual(diffDocuments(doc, doc).entries, [], doc.id);
            assert.deepEqual(diffDocuments(doc, reverseKeys(doc)).entries, [], doc.id);
        }
    });

    it('ac-1: a change to any single leaf of any fixture is found at some entry (property)', () => {
        for (const doc of fixtures()) {
            const changed = clone(doc);
            changed.title = changed.title + ' changed';
            const { entries } = diffDocuments(doc, changed);
            assert.deepEqual(entries.map((e) => [e.path, e.field, e.kind]), [['', 'title', 'changed']], doc.id);
        }
    });

    it('arrays that are not id lists are one field; the plan revision is not reported; status is', () => {
        const a = samplePlan();
        const b = clone(a);
        b.revision = 9;
        b.status = 'published';
        b.units[0].lessons[0].newWords.push('חָדָשׁ');
        b.units[0].lessons[0].blocks[0].pattern.rules[0].title = 'changed rule';
        const { entries } = diffDocuments(a, b);
        const keys = entries.map((e) => e.path + '|' + e.field + '|' + e.kind).sort();
        assert.deepEqual(keys, [
            '|status|changed',
            LESSON + '/blocks/introduction|pattern|changed',
            LESSON + '|newWords|changed'
        ].sort());
        const words = entries.find((e) => e.field === 'newWords');
        assert.deepEqual(words.after.slice(0, -1), words.before);
    });

    it('a field that is new has no before, and a removed field has no after', () => {
        const a = samplePlan();
        const b = clone(a);
        delete b.subtitle;
        b.units[0].title = 'New unit title';
        b.units[0].frequencyBand = 'A1';
        const { entries } = diffDocuments(a, b);
        const subtitle = entries.find((e) => e.field === 'subtitle');
        assert.equal(Object.prototype.hasOwnProperty.call(subtitle, 'after'), false);
        assert.equal(subtitle.before, 'Three lessons. Each one uses the words from the one before.');
        const band = entries.find((e) => e.field === 'frequencyBand');
        assert.equal(Object.prototype.hasOwnProperty.call(band, 'before'), false);
        assert.equal(band.after, 'A1');
    });

    it('a list with duplicate or missing ids is one field, and does not throw', () => {
        const a = samplePlan();
        const b = clone(a);
        a.units[0].lessons[0].blocks[1].id = a.units[0].lessons[0].blocks[0].id;
        b.units[0].lessons[0].blocks[1].id = b.units[0].lessons[0].blocks[0].id;
        b.units[0].lessons[0].blocks[2].passage = 'changed';
        const { entries } = diffDocuments(a, b);
        assert.equal(entries.length, 1);
        assert.deepEqual([entries[0].path, entries[0].field, entries[0].kind], [LESSON, 'blocks', 'changed']);
    });

    it('is pure: frozen inputs are accepted and never changed', () => {
        const a = deepFreeze(samplePlan());
        const b = clone(a);
        b.units[0].lessons[0].blocks[0].words[1].meaning = 'x';
        deepFreeze(b);
        const before = JSON.stringify([a, b]);
        diffDocuments(a, b);
        diffDocuments(a, b, { full: true });
        assert.equal(JSON.stringify([a, b]), before);
        assert.throws(() => diffDocuments(null, b), TypeError);
        assert.throws(() => diffDocuments(a, [b]), TypeError);
    });

    it('path keeps that node and everything below it', () => {
        const a = samplePlan();
        const b = clone(a);
        b.title = 'Other title';
        b.units[0].lessons[0].blocks[0].words[1].meaning = 'x';
        b.units[0].lessons[1].title = 'Second lesson';
        assert.equal(diffDocuments(a, b).entries.length, 3);
        assert.equal(diffDocuments(a, b, { path: LESSON }).entries.length, 1);
        assert.equal(diffDocuments(a, b, { path: 'units/unit-1' }).entries.length, 2);
        assert.equal(diffDocuments(a, b, { path: '/units/unit-1/lessons/cafe-again/' }).entries.length, 1);
        // A path is a node and what is below it, never a string prefix.
        assert.equal(diffDocuments(a, b, { path: 'units/unit' }).entries.length, 0);
        assert.equal(diffDocuments(a, b, { path: LESSON + '/blocks/intro' }).entries.length, 0);
    });

    it('ac-3: the entry count and total size are capped and set truncated; full turns the caps off', () => {
        const mk = (suffix) => ({
            schemaVersion: '1.0.0', id: 'big', revision: 1, status: 'draft', audience: 'shared', assigneeId: null,
            title: 't', language: { target: 'he', gloss: 'en' },
            units: [{ id: 'u', title: 'u', theme: 'u', lessons: [{
                id: 'l', title: 'l', level: 'basic', visibility: 'hidden', buildsOn: [], newWords: [], recycles: [],
                blocks: [{
                    id: 'b', type: 'introduction',
                    words: Array.from({ length: 600 }, (_, i) => ({ id: 'w' + i, word: 'x', meaning: 'm' + suffix }))
                }]
            }] }]
        });
        const many = diffDocuments(mk('a'), mk('b'));
        assert.equal(many.entries.length, 500);
        assert.equal(many.truncated, true);
        assert.equal(diffDocuments(mk('a'), mk('b'), { full: true }).entries.length, 600);
        assert.equal(diffDocuments(mk('a'), mk('b'), { full: true }).truncated, false);

        const bulky = (ch) => {
            const doc = mk('a');
            doc.units[0].lessons[0].blocks[0].words.forEach((w, i) => { w.meaning = ch.repeat(3000) + i; });
            return doc;
        };
        const sized = diffDocuments(bulky('p'), bulky('q'));
        assert.equal(sized.truncated, true);
        assert.equal(sized.entries.length < 30, true, 'the 64 KiB cap stops the list early');
        assert.equal(JSON.stringify(sized.entries).length <= 64 * 1024 + 6200, true);
    });

    it('ac-3: a value over 4000 characters is cut with a marker and valueTruncated', () => {
        const a = samplePlan();
        const b = clone(a);
        a.title = 'x'.repeat(5000);
        b.title = 'y'.repeat(5000);
        const { entries } = diffDocuments(a, b);
        assert.equal(entries.length, 1);
        assert.equal(entries[0].valueTruncated, true);
        assert.equal(entries[0].before.length, 4001);
        assert.equal(entries[0].before.endsWith('\u2026'), true);
        assert.equal(entries[0].after.startsWith('yyyy'), true);

        const c = clone(a);
        c.units[0].lessons[0].newWords = Array.from({ length: 800 }, (_, i) => 'word-number-' + i);
        const object = diffDocuments(a, c).entries.find((e) => e.field === 'newWords');
        assert.equal(object.valueTruncated, true);
        assert.equal(typeof object.after, 'string');
        assert.equal(diffDocuments(a, c, { full: true }).entries.find((e) => e.field === 'newWords').after.length, 800);
    });
});

describe('revisions.diff (INIT-008/SPEC-005)', { concurrency: false }, () => {
    let env;

    before(() => {
        env = planEnv('hebrew-plan-diff-');
    });

    after(() => {
        env.close();
    });

    beforeEach(() => {
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM audit_log');
    });

    /** Three writes after a published revision 2: revisions 3, 4, 5. */
    function buildHistory() {
        const { ops, publish, store } = env;
        env.seed();
        const first = ops.setField(store, PLAN_ID, { path: '', field: 'title', value: 'Title one', expectedRevision: 1 }, env.human('set_field'));
        const pub = publish.publishPlan(store, PLAN_ID, { expectedRevision: first.revision }, env.ai('publish_plan'));
        assert.equal(pub.error, undefined, JSON.stringify(pub.blocks));
        const second = ops.setField(store, PLAN_ID, {
            path: WORD, field: 'meaning', value: 'espresso', expectedRevision: pub.revision
        }, env.ai('set_field'));
        const third = ops.insertBlock(store, PLAN_ID, {
            path: LESSON, index: 1, block: { id: 'extra', type: 'listening', passage: 'תּוֹדָה.' },
            expectedRevision: second.revision
        }, env.human('insert_block'));
        const fourth = ops.setField(store, PLAN_ID, {
            path: '', field: 'title', value: 'Title two', expectedRevision: third.revision
        }, env.ai('set_field'));
        assert.deepEqual([first.revision, pub.revision, second.revision, third.revision, fourth.revision], [2, 3, 4, 5, 6]);
    }

    function summary(entries) {
        return entries.map((e) => e.kind + ':' + e.path + '#' + e.field).sort();
    }

    it('ac-2: between two revisions, working against published, and an integer against working', () => {
        buildHistory();
        const ctx = env.human('diff_plan');

        const one = env.revisions.diff(PLAN_ID, 1, 2, {}, ctx);
        assert.equal(one.error, undefined);
        assert.deepEqual(one.from, { ref: 1, revision: 1 });
        assert.deepEqual(one.to, { ref: 2, revision: 2 });
        assert.deepEqual(summary(one.entries), ['changed:#title']);
        assert.equal(one.entries[0].before, 'יחידה 1 — בבית הקפה');
        assert.equal(one.entries[0].after, 'Title one');

        const span = env.revisions.diff(PLAN_ID, 2, 5, {}, ctx);
        assert.deepEqual(summary(span.entries), [
            'added:' + LESSON + '/blocks/extra#',
            'changed:' + WORD + '#meaning'
        ].sort());

        const viaPublished = env.revisions.diff(PLAN_ID, 'published', 'working', {}, ctx);
        assert.deepEqual(viaPublished.from, { ref: 'published', revision: 3 });
        assert.deepEqual(viaPublished.to, { ref: 'working', revision: 6 });
        assert.deepEqual(summary(viaPublished.entries), [
            'added:' + LESSON + '/blocks/extra#',
            'changed:#status',
            'changed:#title',
            'changed:' + WORD + '#meaning'
        ].sort());

        const against = env.revisions.diff(PLAN_ID, 2, 'working', {}, ctx);
        assert.deepEqual(summary(against.entries), [
            'added:' + LESSON + '/blocks/extra#',
            'changed:#title',
            'changed:' + WORD + '#meaning'
        ].sort());
        assert.equal(against.entries.find((e) => e.field === 'title').after, 'Title two');

        const same = env.revisions.diff(PLAN_ID, 'working', 6, {}, ctx);
        assert.deepEqual(same.entries, []);
    });

    it('ac-2: published with no snapshot is no_snapshot, an unknown revision is not_found, and path narrows', () => {
        env.seed();
        env.ops.setField(env.store, PLAN_ID, { path: '', field: 'title', value: 'x', expectedRevision: 1 }, env.ai('set_field'));
        const ctx = env.ai('diff_plan');
        assert.equal(env.revisions.diff(PLAN_ID, 'published', 'working', {}, ctx).error, 'no_snapshot');
        assert.equal(env.revisions.diff(PLAN_ID, 'working', 'published', {}, ctx).error, 'no_snapshot');
        assert.equal(env.revisions.diff(PLAN_ID, 1, 99, {}, ctx).error, 'not_found');
        assert.equal(env.revisions.diff(PLAN_ID, 0, 1, {}, ctx).error, 'invalid_op');
        assert.equal(env.revisions.diff(PLAN_ID, 'latest', 1, {}, ctx).error, 'invalid_op');
        assert.equal(env.revisions.diff('no-such-plan', 1, 2, {}, ctx).error, 'not_found');
        assert.equal(env.revisions.diff('no-such-plan', 'working', 'working', {}, ctx).error, 'not_found');
        assert.equal(env.revisions.diff(PLAN_ID, 1, 2, { path: 5 }, ctx).error, 'invalid_op');
        assert.throws(() => env.revisions.diff(PLAN_ID, 1, 2, {}, undefined), TypeError);

        buildHistoryOn('narrow');
        const all = env.revisions.diff('narrow', 1, 'working', {}, ctx);
        assert.equal(all.entries.length > 1, true);
        const narrow = env.revisions.diff('narrow', 1, 'working', { path: WORD }, ctx);
        assert.deepEqual(summary(narrow.entries), ['changed:' + WORD + '#meaning']);
        assert.deepEqual(env.revisions.diff('narrow', 1, 'working', { path: LESSON + '/blocks/nothing' }, ctx).entries, []);
    });

    function buildHistoryOn(id) {
        env.seed(samplePlan((p) => { p.id = id; }));
        const a = env.ops.setField(env.store, id, { path: '', field: 'title', value: 'T', expectedRevision: 1 }, env.ai('set_field'));
        env.ops.setField(env.store, id, { path: WORD, field: 'meaning', value: 'espresso', expectedRevision: a.revision }, env.ai('set_field'));
    }

    it('the diff holds plan text only and no account name', () => {
        buildHistory();
        const result = env.revisions.diff(PLAN_ID, 1, 'working', {}, env.ai('diff_plan'));
        assert.equal(JSON.stringify(result).includes('example-admin'), false);
    });
});
