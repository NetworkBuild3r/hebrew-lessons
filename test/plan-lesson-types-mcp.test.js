/**
 * Build five lesson-type samples through MCP in a temp database (INIT-012/SPEC-009).
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');

const fs = require('fs');
const path = require('path');

const harness = require('./helpers/signed-in-server');
const fx = require('./fixtures/lesson-types/fixture-lib');

const ROOT_SQLITE = path.join(__dirname, '..', 'data', 'hebrew.sqlite');

const PLAN_ID = 'lesson-types-mcp';
const UNIT = 'units/unit-1';
const LESSON_FILES = [
    'type-a-topic-conversation.json',
    'type-b-review-worksheet.json',
    'type-c-verse-reading.json',
    'type-d-verb-workshop.json',
    'type-e-short-story.json'
];

describe('INIT-012/SPEC-009: MCP lesson-type build', { concurrency: false }, () => {
    /** @type {string[]} */
    let toolsCalled;
    let db;
    let store;
    let handlers;
    let lexicon;

    before(async () => {
        await harness.start({ lessons: 'fixture' });
        db = require('../server/db');
        lexicon = require('../server/lexicon/service');
        store = require('../server/plans/store');
        process.env.MCP_AUTH_TOKEN = 'lesson-types-mcp-token';
        const tools = require('../mcp/tools');
        tools.registerLessonTools({ registerTool() { /* handlers only */ } });
        handlers = tools.toolHandlers;
    });

    after(async () => {
        delete process.env.MCP_AUTH_TOKEN;
        await harness.stop();
    });

    beforeEach(() => {
        db.run('DELETE FROM plan_revisions');
        db.run('DELETE FROM plan_field_marks');
        db.run('DELETE FROM plans');
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM lexicon_entries');
        toolsCalled = [];
    });

    async function call(name, args) {
        toolsCalled.push(name);
        assert.ok(handlers[name], 'unknown tool ' + name);
        const result = await handlers[name](args);
        const text = result.content && result.content[0] ? result.content[0].text : '';
        let body = null;
        try { body = JSON.parse(text); } catch (_) { /* plain */ }
        if (result.isError) {
            throw new Error('MCP ' + name + ' failed: ' + text.slice(0, 500));
        }
        return body;
    }

    function rev() {
        return store.loadWorkingCopy(PLAN_ID).revision;
    }

    it('ac-1 ac-2 ac-12: five lessons publish at 1.2.0; only existing tools; future forms stay unverified', async (t) => {
        const sqliteBefore = fs.existsSync(ROOT_SQLITE) ? fs.statSync(ROOT_SQLITE).mtimeMs : null;
        const knownTools = new Set(Object.keys(handlers));
        const lexRows = fx.readJson('lexicon-ledaber.json').entries;
        await call('upsert_lexicon_entries', {
            entries: lexRows.map((row) => Object.assign({}, row, {
                verified: row.tense === 'future' ? true : row.verified
            })),
            expectedRev: 0
        });
        for (const row of lexRows.filter((r) => r.tense === 'future')) {
            const stored = db.get('SELECT verified FROM lexicon_entries WHERE key = ?', row.key);
            assert.equal(stored.verified, 0, row.key);
        }

        const lessons = LESSON_FILES.map((f) => fx.readJson(f));
        await call('create_plan', {
            id: PLAN_ID,
            title: 'Lesson types MCP',
            recipe: lessons[0].recipe,
            lessonId: lessons[0].id
        });
        const firstPath = UNIT + '/lessons/' + lessons[0].id;
        for (let bi = lessons[0].blocks.length - 1; bi >= 0; bi -= 1) {
            await call('insert_block', {
                id: PLAN_ID,
                path: firstPath,
                index: 0,
                block: lessons[0].blocks[bi],
                expectedRevision: rev()
            });
        }
        await call('remove_block', {
            id: PLAN_ID,
            path: firstPath + '/blocks/placeholder',
            expectedRevision: rev()
        });
        await call('set_field', {
            id: PLAN_ID,
            path: firstPath,
            field: 'visibility',
            value: 'everyone',
            expectedRevision: rev()
        });

        for (let li = 1; li < lessons.length; li += 1) {
            const lesson = lessons[li];
            await call('insert_lesson', {
                id: PLAN_ID,
                path: UNIT,
                lesson,
                expectedRevision: rev()
            });
            const lessonPath = UNIT + '/lessons/' + lesson.id;
            await call('set_field', {
                id: PLAN_ID,
                path: lessonPath,
                field: 'visibility',
                value: 'everyone',
                expectedRevision: rev()
            });
        }

        const issues = await call('list_plan_issues', { id: PLAN_ID });
        assert.equal(issues.counts.block, 0, JSON.stringify(issues));

        const published = await call('publish_plan', { id: PLAN_ID, expectedRevision: rev() });
        assert.equal(published.status, 'published');
        assert.equal(published.plan.schemaVersion, '1.2.0');
        assert.equal(published.plan.units[0].lessons.length, 5);

        for (const name of toolsCalled) {
            assert.ok(knownTools.has(name), 'new or unknown tool: ' + name);
        }
        assert.ok(toolsCalled.includes('upsert_lexicon_entries'));
        assert.ok(toolsCalled.includes('create_plan'));
        assert.ok(toolsCalled.includes('insert_block'));
        assert.ok(toolsCalled.includes('insert_lesson'));
        assert.ok(toolsCalled.includes('publish_plan'));

        const toolLog = toolsCalled.join(', ');
        t.diagnostic('MCP tool calls: ' + toolLog);
        console.log('MCP tool calls:', toolLog);

        if (sqliteBefore != null) {
            const sqliteAfter = fs.statSync(ROOT_SQLITE).mtimeMs;
            assert.equal(sqliteAfter, sqliteBefore, 'data/hebrew.sqlite must not change during MCP test');
        }
    });
});
