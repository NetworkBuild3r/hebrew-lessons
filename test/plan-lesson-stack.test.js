/**
 * Lesson stack ops, activity map, and research warnings (INIT-010/SPEC-002).
 * Temporary database only. No ElevenLabs or OpenAI.
 * Provenance: INIT-010/SPEC-002
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { withCtx, publishWithCtx, testContext } = require('./helpers/plan-ctx');

const root = path.resolve(__dirname, '..');

function smallLesson(id, title) {
    return {
        id: id,
        title: title || id,
        level: 'basic',
        visibility: 'hidden',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }]
    };
}

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
        '../server/plans/create',
        '../server/plans/publish',
        '../server/plans/issues',
        '../server/plans/activity-map'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    return {
        db: db,
        store: require('../server/plans/store'),
        ops: require('../server/plans/ops'),
        create: require('../server/plans/create'),
        publish: require('../server/plans/publish'),
        issues: require('../server/plans/issues'),
        validate: require('../server/plans/validate'),
        activityMap: require('../server/plans/activity-map')
    };
}

describe('plan lesson stack and research warnings (INIT-010/SPEC-002)', { concurrency: false }, () => {
    let tmp;
    let lessonsDir;
    let db;
    let store;
    let ops;
    let create;
    let publishPlan;
    let issues;
    let validatePlan;
    let RULES;
    let activityMap;
    const savedLessons = process.env.LESSONS_DIR;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lesson-stack-'));
        lessonsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lesson-stack-lessons-'));
        fs.writeFileSync(path.join(lessonsDir, 'marker.txt'), 'untouched');
        process.env.LESSONS_DIR = lessonsDir;
        const m = freshModules(tmp);
        db = m.db;
        store = m.store;
        ops = withCtx(m.ops);
        create = m.create;
        publishPlan = publishWithCtx(m.publish.publishPlan);
        issues = m.issues;
        validatePlan = m.validate.validatePlan;
        RULES = m.validate.RULES;
        activityMap = m.activityMap;
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        fs.rmSync(lessonsDir, { recursive: true, force: true });
        if (savedLessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = savedLessons;
        delete process.env.DATA_DIR;
    });

    beforeEach(() => {
        db.run('DELETE FROM plans');
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM plan_revisions');
        try {
            db.run('DELETE FROM plan_field_marks');
        } catch (_) { /* optional table */ }
    });

    it('ac-1: two insert_lesson calls yield three lessons; move_lesson swaps the last two', () => {
        const made = create.createPlan(store, { id: 'stack-plan', title: 'Stack plan' }, testContext('create_plan'));
        assert.equal(made.error, undefined, JSON.stringify(made));
        const unit = 'units/unit-1';
        const first = ops.insertLesson(store, 'stack-plan', {
            path: unit,
            lesson: smallLesson('lesson-2', 'Second'),
            expectedRevision: 1
        });
        assert.equal(first.error, undefined, JSON.stringify(first));
        const second = ops.insertLesson(store, 'stack-plan', {
            path: unit,
            lesson: smallLesson('lesson-3', 'Third'),
            expectedRevision: first.revision
        });
        assert.equal(second.error, undefined, JSON.stringify(second));
        const ids = second.plan.units[0].lessons.map((lesson) => lesson.id);
        assert.deepEqual(ids, ['lesson-1', 'lesson-2', 'lesson-3']);

        const moved = ops.moveLesson(store, 'stack-plan', {
            path: unit + '/lessons/lesson-3',
            index: 1,
            expectedRevision: second.revision
        });
        assert.equal(moved.error, undefined, JSON.stringify(moved));
        assert.deepEqual(moved.plan.units[0].lessons.map((lesson) => lesson.id),
            ['lesson-1', 'lesson-3', 'lesson-2']);
    });

    it('ac-1: a plan of 100 small lessons validates; N10 does not refuse for lesson count', () => {
        const plan = {
            schemaVersion: '1.0.0',
            id: 'hundred',
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'Hundred',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'unit-1',
                title: 'Unit',
                theme: 'theme',
                lessons: Array.from({ length: 100 }, (_, i) => smallLesson('l-' + i, 'L ' + i))
            }]
        };
        const bytes = Buffer.byteLength(JSON.stringify(plan), 'utf8');
        const result = validatePlan(plan);
        assert.equal(result.blocks.some((item) => item.rule === 'N10'), false);
        assert.equal(result.blocks.some((item) => item.rule === 'schema'), false);
        if (result.blocks.some((item) => item.rule === 'N9')) {
            assert.fail('N9 refused the 100-lesson fixture at ' + bytes + ' bytes; raise planBytes only as far as needed');
        }
        assert.equal(result.ok || result.blocks.every((item) => item.rule !== 'N9' && item.rule !== 'N10'), true);
    });

    it('ac-2: activity map maps Words, Story, and Type it to initiative section 4 types', () => {
        assert.equal(activityMap.blockTypeFor('Words'), 'introduction');
        assert.equal(activityMap.blockTypeFor('Story'), 'graded-story');
        // INIT-011/SPEC-002: Match stays off the menu until INIT-010 SPEC-004 lands a match block.
        assert.equal(activityMap.blockTypeFor('Match'), undefined);
        assert.equal(activityMap.blockTypeFor('Type it'), 'type-it');
        assert.equal(activityMap.ACTIVITY_TO_BLOCK.Listening, 'listening');
        assert.equal(activityMap.ACTIVITY_TO_BLOCK.Dialogue, 'short-dialogue');
        assert.equal(activityMap.ACTIVITY_TO_BLOCK.Fluency, 'fluency-pass');
        assert.equal(activityMap.ACTIVITY_TO_BLOCK.Review, 'spaced-review-checkpoint');
        assert.equal(activityMap.ACTIVITY_TO_BLOCK.Pattern, 'conjugation-table');
    });

    it('ac-3: later-context still warns and publish succeeds', () => {
        const plan = {
            schemaVersion: '1.0.0',
            id: 'later-ctx',
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'Later context',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'unit-1',
                title: 'Unit',
                theme: 'home',
                lessons: [
                    {
                        id: 'family-1',
                        title: 'Family 1',
                        level: 'basic',
                        visibility: 'hidden',
                        buildsOn: [],
                        newWords: ['שָׁלוֹם'],
                        recycles: [],
                        blocks: [{
                            id: 'intro',
                            type: 'introduction',
                            words: [{
                                id: 'w1',
                                word: 'שָׁלוֹם',
                                meaning: 'hello',
                                sentence: 'שָׁלוֹם דָּנִי.'
                            }]
                        }]
                    },
                    {
                        id: 'family-2',
                        title: 'Family 2',
                        level: 'basic',
                        visibility: 'hidden',
                        buildsOn: ['family-1'],
                        newWords: [],
                        recycles: [],
                        blocks: [{
                            id: 'story',
                            type: 'graded-story',
                            paragraphs: [{ id: 'p1', text: 'דָּנִי בַּבַּיִת.' }]
                        }]
                    }
                ]
            }]
        };
        store.saveWorkingCopy({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        const listed = issues.list(store, 'later-ctx', { severity: 'all' });
        assert.equal(listed.error, undefined, JSON.stringify(listed));
        assert.equal(listed.issues.some((issue) => issue.code === 'later-context'), true);
        const published = publishPlan(store, 'later-ctx', { expectedRevision: 1 });
        assert.equal(published.error, undefined, JSON.stringify(published));
        assert.equal(published.status, 'published');
    });

    it('ac-4: picture-on-recall and closed-set are warn and not publish blocks', () => {
        assert.equal(RULES['picture-on-recall'].severity, 'warn');
        assert.equal(RULES['closed-set'].severity, 'warn');

        const plan = {
            schemaVersion: '1.0.0',
            id: 'warn-only',
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'Warn only',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'unit-1',
                title: 'Unit',
                theme: 'kin',
                lessons: [{
                    id: 'kin-lesson',
                    title: 'Kin',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: [],
                    newWords: ['אֵם', 'אָב', 'אָח', 'אָחוֹת', 'בֵּן', 'בַּת'],
                    recycles: [],
                    blocks: [
                        {
                            id: 'intro',
                            type: 'introduction',
                            words: [
                                { id: 'w1', word: 'אֵם', meaning: 'mother' },
                                { id: 'w2', word: 'אָב', meaning: 'father' },
                                { id: 'w3', word: 'אָח', meaning: 'brother' },
                                { id: 'w4', word: 'אָחוֹת', meaning: 'sister' },
                                { id: 'w5', word: 'בֵּן', meaning: 'son' },
                                { id: 'w6', word: 'בַּת', meaning: 'daughter' }
                            ]
                        },
                        {
                            id: 'type',
                            type: 'type-it',
                            picture: { mediaId: 'mom-pic', alt: 'a mother' },
                            answer: 'אֵם'
                        }
                    ]
                }]
            }]
        };
        const result = validatePlan(plan);
        const picture = result.warnings.filter((item) => item.rule === 'picture-on-recall');
        const closed = result.warnings.filter((item) => item.rule === 'closed-set');
        assert.equal(picture.length, 1);
        assert.equal(closed.length, 1);
        assert.equal(result.blocks.some((item) => item.rule === 'picture-on-recall'), false);
        assert.equal(result.blocks.some((item) => item.rule === 'closed-set'), false);
        assert.equal(result.ok, true);

        store.saveWorkingCopy({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null
        });
        const listed = issues.list(store, 'warn-only', { severity: ['warn'] });
        assert.equal(listed.issues.some((issue) => issue.code === 'picture-on-recall'), true);
        assert.equal(listed.issues.some((issue) => issue.code === 'closed-set'), true);
        const published = publishPlan(store, 'warn-only', { expectedRevision: 1 });
        assert.equal(published.error, undefined, JSON.stringify(published));

        const mixed = JSON.parse(JSON.stringify(plan));
        mixed.id = 'mixed-kin';
        mixed.units[0].lessons[0].newWords.push('בַּיִת');
        mixed.units[0].lessons[0].blocks[0].words.push({ id: 'w7', word: 'בַּיִת', meaning: 'house' });
        const mixedResult = validatePlan(mixed);
        assert.equal(mixedResult.warnings.some((item) => item.rule === 'closed-set'), false);
    });

    it('ac-1 aud-1: removing the last lesson is refused', () => {
        create.createPlan(store, { id: 'one-lesson', title: 'One' }, testContext('create_plan'));
        const refused = ops.removeLesson(store, 'one-lesson', {
            path: 'units/unit-1/lessons/lesson-1',
            expectedRevision: 1
        });
        assert.equal(refused.error, 'invalid_op');
        assert.equal(store.loadWorkingCopy('one-lesson').revision, 1);
    });

    it('ac-5: the new lesson routes are on WRITE_ROUTES (non-admin 403 covered by plan-routes)', () => {
        const planRoutes = require('../server/api/plan-routes');
        const lessonRoutes = planRoutes.WRITE_ROUTES.filter((route) => /\/lessons/.test(route.path));
        assert.deepEqual(lessonRoutes.map((route) => route.method + ' ' + route.path + ' ' + route.fn), [
            'POST /api/plans/:id/lessons insertLesson',
            'POST /api/plans/:id/lessons/move moveLesson',
            'DELETE /api/plans/:id/lessons removeLesson'
        ]);
    });

    it('ac-5: MCP lesson tools require MCP_AUTH_TOKEN and say a plan has no lesson maximum', async () => {
        const prev = process.env.MCP_AUTH_TOKEN;
        try {
            delete require.cache[require.resolve('../mcp/tools')];
            delete process.env.MCP_AUTH_TOKEN;
            const tools = require('../mcp/tools');
            const metas = Object.create(null);
            tools.registerLessonTools({ registerTool(name, meta) { metas[name] = meta; } });
            const handlers = tools.toolHandlers;
            for (const name of ['insert_lesson', 'move_lesson', 'remove_lesson']) {
                assert.equal(typeof handlers[name], 'function', name);
                assert.equal(metas[name].description.includes('no lesson maximum'), true, name);
                assert.equal(metas[name].description.includes('Requires MCP_AUTH_TOKEN'), true, name);
                const denied = await handlers[name]({
                    id: 'stack-plan',
                    path: 'units/unit-1',
                    lesson: smallLesson('x'),
                    index: 0,
                    expectedRevision: 1
                }, { requestId: 1 });
                assert.equal(denied.isError, true, name);
            }
        } finally {
            if (prev === undefined) delete process.env.MCP_AUTH_TOKEN;
            else process.env.MCP_AUTH_TOKEN = prev;
            delete require.cache[require.resolve('../mcp/tools')];
        }
    });
});
