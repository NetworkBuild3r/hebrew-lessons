/**
 * The issue list and the five warnings through the real operations, on a
 * temporary database: issues.list, the write operations, and publishPlan.
 * No OpenAI, no ElevenLabs.
 * Provenance: INIT-008/SPEC-008
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { basePlan } = require('./helpers/plan-fixtures');

const root = path.resolve(__dirname, '..');
const CODES = ['pointing-form', 'field-language', 'lexicon-form', 'pointing-consistency', 'audio-stale'];
const LESSON = 'units/unit-1/lessons/lesson-1';

/** Exactly one hard error, three warnings of two codes, two informs, and nothing else. */
function issuePlan() {
    return {
        schemaVersion: '1.1.0',
        id: 'issue-plan',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Issue plan',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'u1',
            title: 'Unit',
            theme: 'home',
            lessons: [{
                id: 'l1',
                title: 'Lesson',
                level: 'basic',
                visibility: 'hidden',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks: [
                    {
                        id: 'talk',
                        type: 'short-dialogue',
                        source: 'authored',
                        lines: [
                            { id: 'a', pointed: 'ב\u05B7\u05B8ית', audioPath: 'bad path' },
                            { id: 'b', pointed: 'ג\u05B4\u05B7ן' },
                            { id: 'c', pointed: 'bayit' }
                        ]
                    },
                    { id: 'hear', type: 'listening', passage: 'לֹא כֵּן', questions: [{ id: 'q1', question: 'לֹא?' }] },
                    {
                        id: 'read',
                        type: 'graded-story',
                        paragraphs: [{ id: 'p1', text: 'לֹא כֵּן' }],
                        questions: [{ id: 'q2', question: 'כֵּן?' }]
                    }
                ]
            }]
        }]
    };
}

/** Hebrew-less plan of many lines, each with a doubled vowel, to pass the 200 cap. */
function manyIssuesPlan() {
    const plan = issuePlan();
    const lines = () => Array.from({ length: 12 }, (_, i) => ({ id: 'n' + i, pointed: 'ב\u05B7\u05B8ית' }));
    plan.id = 'many-plan';
    plan.units[0].lessons = Array.from({ length: 3 }, (_, l) => ({
        id: 'l' + l,
        title: 'Lesson',
        level: 'basic',
        visibility: 'hidden',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: Array.from({ length: 12 }, (__, b) => ({ id: 'b' + b, type: 'short-dialogue', source: 'authored', lines: lines() }))
    }));
    return plan;
}

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const marker = path.sep + 'server' + path.sep;
    Object.keys(require.cache).forEach((file) => {
        if (file.includes(marker)) delete require.cache[file];
    });
    const db = require('../server/db');
    db.close();
    return {
        db,
        store: require('../server/plans/store'),
        ops: require('../server/plans/ops'),
        publish: require('../server/plans/publish'),
        issues: require('../server/plans/issues'),
        lexicon: require('../server/db/repos/lexicon'),
        context: require('../server/plans/context')
    };
}

describe('issue list and the five warnings on a temporary database', { concurrency: false }, () => {
    let tmp;
    let m;
    let ctx;
    const savedLessons = process.env.LESSONS_DIR;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-issues-'));
        process.env.LESSONS_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-issues-lessons-'));
        m = freshModules(tmp);
        m.db.migrate();
        ctx = m.context.makeContext({ channel: 'system', name: 'issues-test', tool: 'test' });
    });

    after(() => {
        if (m && m.db) m.db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        fs.rmSync(process.env.LESSONS_DIR, { recursive: true, force: true });
        if (savedLessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = savedLessons;
        delete process.env.DATA_DIR;
        const marker = path.sep + 'server' + path.sep;
        Object.keys(require.cache).forEach((file) => {
            if (file.includes(marker)) delete require.cache[file];
        });
    });

    beforeEach(() => {
        m.db.run('DELETE FROM plans');
        m.db.run('DELETE FROM lexicon_entries');
    });

    function seed(plan, extra) {
        const row = Object.assign({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        }, extra || {});
        m.store.saveWorkingCopy(row);
        return m.store.loadWorkingCopy(plan.id);
    }

    function lexiconEntry(key, extra) {
        m.lexicon.upsert(Object.assign({
            key, kind: 'lemma', gloss_en: 'house', translit: 'bayit', verified: 1, source: 'seed'
        }, extra || {}));
    }

    const codesOf = (result) => result.issues.map((issue) => issue.code);
    const at = (pointer, plan) => pointer.split('/').slice(1).reduce((node, key) => node[key], plan);

    it('ac-6: one block, three warnings of two codes, two informs: counts, order, and the inform filter', () => {
        seed(issuePlan());
        const result = m.issues.list(m.store, 'issue-plan');
        assert.deepEqual(result.counts, { block: 1, warn: 3, inform: 2 });
        assert.equal(result.revision, 1);
        assert.equal(result.status, 'draft');
        assert.equal(result.truncated, false);
        assert.deepEqual(result.issues.map((issue) => issue.severity), ['block', 'warn', 'warn', 'warn']);
        assert.deepEqual(codesOf(result), ['audio-path', 'pointing-form', 'pointing-form', 'field-language']);
        const [hard, ...warns] = result.issues;
        assert.equal(hard.hard, true);
        assert.equal(warns.every((issue) => issue.hard === false), true);
        assert.deepEqual(warns.map((issue) => issue.path), [
            'units/u1/lessons/l1/blocks/talk/lines/a',
            'units/u1/lessons/l1/blocks/talk/lines/b',
            'units/u1/lessons/l1/blocks/talk/lines/c'
        ]);
        assert.deepEqual(Object.keys(hard).sort(), ['code', 'field', 'hard', 'message', 'path', 'pointer', 'severity']);

        const informs = m.issues.list(m.store, 'issue-plan', { severity: 'inform' });
        assert.deepEqual(informs.counts, { block: 1, warn: 3, inform: 2 }, 'counts cover every severity');
        assert.deepEqual(codesOf(informs), ['questions', 'questions']);
        const everything = m.issues.list(m.store, 'issue-plan', { severity: 'all' });
        assert.deepEqual(everything.issues.map((issue) => issue.severity),
            ['block', 'warn', 'warn', 'warn', 'inform', 'inform']);
        const two = m.issues.list(m.store, 'issue-plan', { severity: ['block', 'inform'] });
        assert.equal(two.issues.length, 3);
    });

    it('ac-6: each issue path is accepted by read_part and field is the rest of the pointer', () => {
        const plan = issuePlan();
        seed(plan);
        const result = m.issues.list(m.store, 'issue-plan', { severity: 'all' });
        assert.equal(result.issues.length, 6);
        result.issues.forEach((issue) => {
            const read = m.ops.readPart(m.store, 'issue-plan', { path: issue.path });
            assert.equal(read.error, undefined, issue.path + ' ' + JSON.stringify(read));
            assert.ok(read.part && typeof read.part === 'object');
            assert.equal(issue.pointer.endsWith(issue.field ? '/' + issue.field : ''), true);
            assert.notEqual(at(issue.pointer, plan), undefined, issue.pointer);
        });
        const doubled = result.issues.find((issue) => issue.path.endsWith('lines/a') && issue.code === 'pointing-form');
        assert.equal(doubled.field, 'pointed');
        assert.equal(doubled.pointer, '/units/0/lessons/0/blocks/0/lines/0/pointed');
        const asked = result.issues.find((issue) => issue.code === 'questions' && issue.path.endsWith('blocks/hear'));
        assert.equal(asked.field, 'questions');
    });

    it('ac-6: severity, code, and pathPrefix each narrow the result', () => {
        seed(issuePlan());
        assert.deepEqual(codesOf(m.issues.list(m.store, 'issue-plan', { severity: 'block' })), ['audio-path']);
        assert.deepEqual(codesOf(m.issues.list(m.store, 'issue-plan', { severity: 'warn' })),
            ['pointing-form', 'pointing-form', 'field-language']);
        assert.deepEqual(codesOf(m.issues.list(m.store, 'issue-plan', { code: 'field-language' })), ['field-language']);
        assert.equal(m.issues.list(m.store, 'issue-plan', { code: 'nothing' }).issues.length, 0);
        const talk = m.issues.list(m.store, 'issue-plan', { pathPrefix: 'units/u1/lessons/l1/blocks/talk', severity: 'all' });
        assert.equal(talk.issues.length, 4);
        const line = m.issues.list(m.store, 'issue-plan', { pathPrefix: '/units/u1/lessons/l1/blocks/talk/lines/b' });
        assert.deepEqual(codesOf(line), ['pointing-form']);
        const none = m.issues.list(m.store, 'issue-plan', { pathPrefix: 'units/u1/lessons/l1/blocks/tal' });
        assert.equal(none.issues.length, 0, 'a prefix matches whole segments');
        const both = m.issues.list(m.store, 'issue-plan', { code: 'pointing-form', pathPrefix: 'units/u1/lessons/l1/blocks/talk/lines/a' });
        assert.equal(both.issues.length, 1);
        assert.deepEqual(both.counts, { block: 1, warn: 3, inform: 2 }, 'counts ignore the filters');
    });

    it('ac-6: the limit and the 200 cap set truncated', () => {
        seed(issuePlan());
        const short = m.issues.list(m.store, 'issue-plan', { limit: 2 });
        assert.equal(short.issues.length, 2);
        assert.equal(short.truncated, true);
        assert.equal(short.counts.warn, 3);
        assert.equal(m.issues.list(m.store, 'issue-plan', { limit: 4 }).truncated, false);

        seed(manyIssuesPlan());
        const dflt = m.issues.list(m.store, 'many-plan');
        assert.equal(dflt.issues.length, m.issues.DEFAULT_LIMIT);
        assert.equal(dflt.truncated, true);
        assert.equal(dflt.counts.warn, 432);
        const capped = m.issues.list(m.store, 'many-plan', { limit: 5000 });
        assert.equal(capped.issues.length, 200);
        assert.equal(capped.truncated, true);
    });

    it('refuses a bad filter and a missing plan without throwing', () => {
        seed(issuePlan());
        assert.deepEqual(m.issues.list(m.store, 'issue-plan', { severity: 'fatal' }), { error: 'invalid_op', field: 'severity' });
        assert.deepEqual(m.issues.list(m.store, 'issue-plan', { limit: 0 }), { error: 'invalid_op', field: 'limit' });
        assert.deepEqual(m.issues.list(m.store, 'issue-plan', { limit: '5' }), { error: 'invalid_op', field: 'limit' });
        assert.deepEqual(m.issues.list(m.store, 'issue-plan', { code: 7 }), { error: 'invalid_op', field: 'code' });
        assert.deepEqual(m.issues.list(m.store, 'absent-plan'), { error: 'not_found' });
    });

    it('reads only: listing changes no stored row', () => {
        const before = seed(issuePlan());
        m.issues.list(m.store, 'issue-plan', { severity: 'all' });
        const after = m.store.loadWorkingCopy('issue-plan');
        assert.equal(after.document, before.document);
        assert.equal(after.revision, before.revision);
        assert.equal(after.updatedAt, before.updatedAt);
    });

    it('ac-7: a lexicon-form fix, applied with one set_field, removes that finding', () => {
        const plan = basePlan();
        plan.units[0].lessons[0].blocks[0].words[1].word = 'בַיִת'; // no dagesh
        plan.units[0].lessons[0].newWords[1] = 'בַיִת';
        plan.units[0].lessons[0].blocks[0].cloze.items[0].answer = 'בַיִת';
        seed(plan);
        lexiconEntry('בַּיִת'.normalize('NFC'));
        const before = m.issues.list(m.store, plan.id, { code: 'lexicon-form' });
        assert.equal(before.issues.length, 1);
        const issue = before.issues[0];
        assert.equal(issue.severity, 'warn');
        assert.equal(issue.path, LESSON + '/blocks/intro/words/w2');
        assert.equal(issue.field, 'word');
        assert.equal(issue.fix.action, 'set_field');
        assert.equal(issue.fix.suggested, 'בַּיִת'.normalize('NFC'));
        const applied = m.ops.setField(m.store, plan.id, {
            path: issue.fix.path, field: issue.fix.field, value: issue.fix.suggested, expectedRevision: before.revision
        }, ctx);
        assert.equal(applied.error, undefined, JSON.stringify(applied));
        const after = m.issues.list(m.store, plan.id, { code: 'lexicon-form' });
        assert.equal(after.issues.length, 0);
        assert.equal(after.revision, before.revision + 1);
    });

    it('ac-7: a pointing-form (iii) fix, applied with one set_field, removes that finding', () => {
        const plan = basePlan();
        const bad = 'ב\u05BC\u05B8ית'; // dagesh before the vowel: not canonical, so not NFC
        assert.notEqual(bad, bad.normalize('NFC'));
        plan.units[0].lessons[0].blocks[0].words[0].sentence = bad;
        seed(plan);
        const before = m.issues.list(m.store, plan.id, { code: 'pointing-form' });
        assert.equal(before.issues.length, 1);
        const { fix } = before.issues[0];
        assert.equal(fix.action, 'set_field');
        assert.equal(fix.field, 'sentence');
        const applied = m.ops.setField(m.store, plan.id, {
            path: fix.path, field: fix.field, value: fix.suggested, expectedRevision: before.revision
        }, ctx);
        assert.equal(applied.error, undefined, JSON.stringify(applied));
        assert.equal(m.issues.list(m.store, plan.id, { code: 'pointing-form' }).issues.length, 0);
    });

    it('a replace_part fix names a nested field of the part it applies to', () => {
        const plan = basePlan();
        plan.units[0].lessons[0].blocks[0].cloze.items[0].answer = 'ב\u05BC\u05B8ית';
        seed(plan);
        const [issue] = m.issues.list(m.store, plan.id, { code: 'pointing-form' }).issues;
        assert.equal(issue.fix.action, 'replace_part');
        assert.equal(issue.fix.field, 'cloze/items/0/answer');
        const read = m.ops.readPart(m.store, plan.id, { path: issue.fix.path });
        const part = read.part;
        part.cloze.items[0].answer = issue.fix.suggested;
        const applied = m.ops.replacePart(m.store, plan.id, {
            path: issue.fix.path, value: part, expectedRevision: read.revision
        }, ctx);
        assert.equal(applied.error, undefined, JSON.stringify(applied));
        assert.equal(m.issues.list(m.store, plan.id, { code: 'pointing-form' }).issues.length, 0);
    });

    it('ac-2: every write operation accepts a plan with the new warnings, and it publishes', () => {
        const plan = basePlan();
        seed(plan);
        lexiconEntry('בַּיִת'.normalize('NFC'));

        // publish once, so a snapshot exists for audio-stale
        const first = m.publish.publishPlan(m.store, plan.id, { expectedRevision: 1 }, ctx);
        assert.equal(first.error, undefined, JSON.stringify(first));
        let revision = first.revision;
        assert.equal(m.issues.list(m.store, plan.id).issues.filter((i) => CODES.includes(i.code)).length, 0);

        const intro = LESSON + '/blocks/intro/words/w1';
        const steps = [
            // audio-stale: the sentence changes, its clip path stays
            () => m.ops.setField(m.store, plan.id, { path: intro, field: 'sentence', value: 'שָׁלוֹם לְכֻלָּם.', expectedRevision: revision }, ctx),
            // lexicon-form on meaning: the verified entry says house
            () => m.ops.setField(m.store, plan.id, { path: LESSON + '/blocks/intro/words/w2', field: 'meaning', value: 'home', expectedRevision: revision }, ctx),
            // field-language and pointing-form
            () => m.ops.setField(m.store, plan.id, { path: LESSON + '/blocks/listen', field: 'passage', value: 'bayit', expectedRevision: revision }, ctx),
            () => m.ops.setField(m.store, plan.id, { path: LESSON + '/blocks/story/paragraphs/p1', field: 'text', value: 'ד\u05B7\u05B8ני גר', expectedRevision: revision }, ctx),
            // pointing-consistency: the explainer example points בית another way than w2
            () => {
                const read = m.ops.readPart(m.store, plan.id, { path: LESSON + '/blocks/explain' });
                read.part.sections[0].examples[0].he = 'בַיִת';
                return m.ops.replacePart(m.store, plan.id, { path: LESSON + '/blocks/explain', value: read.part, expectedRevision: revision }, ctx);
            },
            // insert, move, remove all accept a block that carries warnings
            () => m.ops.insertBlock(m.store, plan.id, {
                path: LESSON, index: 0,
                block: { id: 'extra', type: 'listening', passage: 'שלום \u05B8', audioPath: 'listen/extra.mp3' },
                expectedRevision: revision
            }, ctx),
            () => m.ops.moveBlock(m.store, plan.id, { path: LESSON + '/blocks/extra', index: 2, expectedRevision: revision }, ctx)
        ];
        steps.forEach((step, i) => {
            const result = step();
            assert.equal(result.error, undefined, 'step ' + i + ' ' + JSON.stringify(result));
            revision = result.revision;
            assert.deepEqual(result.validation.blocks.filter((item) => CODES.includes(item.rule)), []);
        });

        const listed = m.issues.list(m.store, plan.id, { severity: 'warn', limit: 200 });
        const seen = new Set(codesOf(listed));
        ['pointing-form', 'field-language', 'lexicon-form', 'pointing-consistency', 'audio-stale'].forEach((code) => {
            assert.ok(seen.has(code), 'missing ' + code + ' in ' + Array.from(seen).join(','));
        });
        assert.equal(listed.counts.block, 0);

        // removing the extra block is accepted; then the plan publishes with the warnings still on it
        const removed = m.ops.removeBlock(m.store, plan.id, { path: LESSON + '/blocks/extra', expectedRevision: revision }, ctx);
        assert.equal(removed.error, undefined, JSON.stringify(removed));
        const published = m.publish.publishPlan(m.store, plan.id, { expectedRevision: removed.revision }, ctx);
        assert.equal(published.error, undefined, JSON.stringify(published));
        assert.equal(published.status, 'published');
        assert.equal(published.validation.ok, true);
        assert.deepEqual(published.validation.blocks, []);

        // after publish the snapshot is the working copy, so audio-stale has nothing to compare
        const after = m.issues.list(m.store, plan.id, { severity: 'warn', limit: 200 });
        assert.equal(codesOf(after).includes('audio-stale'), false);
        assert.equal(codesOf(after).includes('pointing-form'), true, 'the other warnings stay listed');
    });

    it('lists no snapshot finding for a plan that was never published', () => {
        const plan = basePlan();
        plan.units[0].lessons[0].blocks[0].words[0].sentence = 'שָׁלוֹם לְכֻלָּם.';
        seed(plan);
        assert.equal(codesOf(m.issues.list(m.store, plan.id, { severity: 'all' })).includes('audio-stale'), false);
    });

    it('ac-8: listByStrippedKeys is read only, one scan, and finds pointed variants', () => {
        lexiconEntry('בַּיִת'.normalize('NFC'));
        lexiconEntry('שָׁלוֹם'.normalize('NFC'), { gloss_en: 'peace', translit: 'shalom' });
        const rows = m.lexicon.listByStrippedKeys(['בית', 'בַיִת']);
        assert.deepEqual(rows.map((row) => row.key), ['בַּיִת'.normalize('NFC')]);
        assert.deepEqual(m.lexicon.listByStrippedKeys([]), []);
        assert.deepEqual(m.lexicon.listByStrippedKeys(['zzz']), []);
        assert.equal(m.lexicon.strippedKey('בַּיִת'), 'בית');
    });
});
