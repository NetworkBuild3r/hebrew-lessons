/**
 * MCP lesson assignment tools (INIT-002/SPEC-007).
 * Temp database + LESSONS_DIR; call tool handlers directly (no port 8787).
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

function sampleWorksheet(slug) {
    return {
        id: slug,
        title: 'MCP ' + slug,
        subtitle: 'test',
        level: 'basic',
        vocab: {
            items: [{
                id: '1',
                word: 'שָׁלוֹם',
                meaning: 'hello',
                sound: 'sha-LOM',
                trap: 'Stress.',
                sentence: 'שָׁלוֹם.'
            }]
        },
        reading: { paragraphs: ['שָׁלוֹם.'] },
        activities: [{ type: 'words' }, { type: 'story' }]
    };
}

function replyText(result) {
    assert.ok(result && result.content && result.content[0]);
    return result.content[0].text;
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/repos/visibility',
        '../server/db/repos/audit',
        '../server/db/repos/users',
        '../server/auth/policy',
        '../server/visibility',
        '../server/accounts',
        '../server/audio',
        '../server/vocab-index',
        '../mcp/store',
        '../mcp/tools'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

describe('MCP assignment tools (SPEC-007)', () => {
    let tmpData;
    let tmpLessons;
    let db;
    let tools;
    let handlers;
    let visibility;
    let accounts;
    let audit;
    let alice;
    let bob;
    let prevEnv;

    before(() => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-mcp-asgn-data-'));
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-mcp-asgn-lessons-'));
        fs.mkdirSync(path.join(tmpLessons, '_audio', 'words'), { recursive: true });
        fs.writeFileSync(path.join(tmpLessons, 'catalog.js'), 'window.HEBREW_CATALOG = [];\n', 'utf8');

        prevEnv = {
            HEBREW_ROOT: process.env.HEBREW_ROOT,
            DATA_DIR: process.env.DATA_DIR,
            LESSONS_DIR: process.env.LESSONS_DIR,
            MCP_AUTH_TOKEN: process.env.MCP_AUTH_TOKEN
        };
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmpData;
        process.env.LESSONS_DIR = tmpLessons;
        process.env.MCP_AUTH_TOKEN = 'test-mcp-assignment-token';
        clearModules();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        accounts = require('../server/accounts');
        visibility = require('../server/visibility');
        audit = require('../server/db/repos/audit');
        tools = require('../mcp/tools');

        tools.registerLessonTools({
            registerTool() { /* capture via toolHandlers */ }
        });
        handlers = tools.toolHandlers;

        alice = accounts.createUser({
            username: 'alice',
            password: 'a-strong-pass1',
            displayName: 'Alice Learner',
            role: 'user',
            actor: 'test'
        });
        bob = accounts.createUser({
            username: 'bob',
            password: 'a-strong-pass1',
            displayName: 'Bob Learner',
            role: 'user',
            actor: 'test'
        });
        accounts.createUser({
            username: 'inactive',
            password: 'a-strong-pass1',
            displayName: 'Inactive',
            role: 'user',
            actor: 'test'
        });
        const inactive = accounts.listUsers().find((u) => u.username === 'inactive');
        accounts.setStatus(inactive.id, 'disabled', 'test');
    });

    after(() => {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        try {
            fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(tmpLessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        for (const [k, v] of Object.entries(prevEnv)) {
            if (v === undefined) delete process.env[k];
            else process.env[k] = v;
        }
        clearModules();
    });

    beforeEach(() => {
        process.env.MCP_AUTH_TOKEN = 'test-mcp-assignment-token';
        for (const name of fs.readdirSync(tmpLessons)) {
            if (name === 'catalog.js' || name.startsWith('_')) continue;
            fs.rmSync(path.join(tmpLessons, name), { recursive: true, force: true });
        }
        fs.writeFileSync(path.join(tmpLessons, 'catalog.js'), 'window.HEBREW_CATALOG = [];\n', 'utf8');
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM lesson_grants');
        db.run('DELETE FROM lesson_visibility');
    });

    it('ac-8: upsert_lesson visibility selected / everyone / omitted', async () => {
        const slug = 'ac8-vis';
        const worksheet = sampleWorksheet(slug);

        const selected = await handlers.upsert_lesson({
            slug,
            worksheet,
            audio: false,
            visibility: { users: ['alice'] }
        });
        assert.equal(selected.isError, undefined);
        assert.equal(visibility.canSeeLesson(alice, slug), true);
        assert.equal(visibility.canSeeLesson(bob, slug), false);
        assert.match(replyText(selected), /alice|selected/i);

        const everyone = await handlers.upsert_lesson({
            slug,
            worksheet: sampleWorksheet(slug),
            audio: false,
            visibility: 'everyone'
        });
        assert.equal(everyone.isError, undefined);
        assert.equal(visibility.canSeeLesson(alice, slug), true);
        assert.equal(visibility.canSeeLesson(bob, slug), true);
        assert.match(replyText(everyone), /everyone/i);

        const slugNew = 'ac8-new-hidden';
        const omittedNew = await handlers.upsert_lesson({
            slug: slugNew,
            worksheet: sampleWorksheet(slugNew),
            audio: false
        });
        assert.equal(omittedNew.isError, undefined);
        assert.equal(visibility.canSeeLesson(alice, slugNew), false);
        assert.match(replyText(omittedNew), /hidden/i);

        visibility.setVisibility('mcp', slugNew, 'everyone');
        assert.equal(visibility.canSeeLesson(alice, slugNew), true);
        const omittedExisting = await handlers.upsert_lesson({
            slug: slugNew,
            worksheet: sampleWorksheet(slugNew),
            audio: false
        });
        assert.equal(omittedExisting.isError, undefined);
        assert.equal(visibility.canSeeLesson(alice, slugNew), true);
        assert.equal(visibility.canSeeLesson(bob, slugNew), true);
        assert.match(replyText(omittedExisting), /unchanged/i);
    });

    it('ac-9: set_lesson_visibility changes access; audit mcp; unknown user no-op', async () => {
        const slug = 'ac9-set';
        await handlers.upsert_lesson({
            slug,
            worksheet: sampleWorksheet(slug),
            audio: false
        });
        assert.equal(visibility.canSeeLesson(alice, slug), false);

        const ok = await handlers.set_lesson_visibility({
            slug,
            visibility: { users: ['alice'] }
        });
        assert.equal(ok.isError, undefined);
        assert.equal(visibility.canSeeLesson(alice, slug), true);
        assert.equal(visibility.canSeeLesson(bob, slug), false);
        assert.match(replyText(ok), /alice|selected/i);

        const rows = audit.listRecent(20);
        const visAudit = rows.filter((r) => r.actor === 'mcp' && (
            r.action === 'set_visibility' || r.action === 'set_grants'
        ));
        assert.ok(visAudit.length >= 1, 'expected mcp audit rows');
        assert.ok(visAudit.every((r) => r.actor === 'mcp'));

        const beforeMode = require('../server/db/repos/visibility').getVisibility(slug);
        const beforeGrants = require('../server/db/repos/visibility').listGrants(slug);
        const bad = await handlers.set_lesson_visibility({
            slug,
            visibility: { users: ['no-such-user'] }
        });
        assert.equal(bad.isError, true);
        assert.match(replyText(bad), /unknown|inactive/i);
        const afterMode = require('../server/db/repos/visibility').getVisibility(slug);
        const afterGrants = require('../server/db/repos/visibility').listGrants(slug);
        assert.equal(afterMode.mode, beforeMode.mode);
        assert.deepEqual(
            afterGrants.map((g) => g.user_id).sort(),
            beforeGrants.map((g) => g.user_id).sort()
        );
    });

    it('ac-10: list_users returns only username and displayName for active accounts', async () => {
        const result = await handlers.list_users({});
        assert.equal(result.isError, undefined);
        const users = JSON.parse(replyText(result));
        assert.ok(Array.isArray(users));
        assert.ok(users.some((u) => u.username === 'alice'));
        assert.ok(users.some((u) => u.username === 'bob'));
        assert.ok(!users.some((u) => u.username === 'inactive'));

        for (const u of users) {
            const keys = Object.keys(u).sort();
            assert.deepEqual(keys, ['displayName', 'username']);
            assert.ok(!('password' in u));
            assert.ok(!('password_hash' in u));
            assert.ok(!('hash' in u));
            assert.ok(!('token' in u));
            assert.ok(!('role' in u));
            assert.ok(!('status' in u));
            assert.ok(!('activity' in u));
        }
        const aliceRow = users.find((u) => u.username === 'alice');
        assert.equal(aliceRow.displayName, 'Alice Learner');
    });

    it('ac-11: without MCP_AUTH_TOKEN, visibility tools refuse; plain upsert still writes', async () => {
        delete process.env.MCP_AUTH_TOKEN;

        const list = await handlers.list_users({});
        assert.equal(list.isError, true);
        assert.match(replyText(list), /MCP_AUTH_TOKEN/);

        const setVis = await handlers.set_lesson_visibility({
            slug: 'ac11-x',
            visibility: 'everyone'
        });
        assert.equal(setVis.isError, true);
        assert.match(replyText(setVis), /MCP_AUTH_TOKEN/);

        const slugVis = 'ac11-vis-blocked';
        const withVis = await handlers.upsert_lesson({
            slug: slugVis,
            worksheet: sampleWorksheet(slugVis),
            audio: false,
            visibility: 'everyone'
        });
        assert.equal(withVis.isError, true);
        assert.match(replyText(withVis), /MCP_AUTH_TOKEN/);
        assert.equal(
            fs.existsSync(path.join(tmpLessons, slugVis, 'worksheet.js')),
            false,
            'visibility upsert must write nothing when token missing'
        );

        const slugPlain = 'ac11-plain';
        const plain = await handlers.upsert_lesson({
            slug: slugPlain,
            worksheet: sampleWorksheet(slugPlain),
            audio: false
        });
        assert.equal(plain.isError, undefined);
        assert.ok(fs.existsSync(path.join(tmpLessons, slugPlain, 'worksheet.js')));
        assert.match(replyText(plain), /hidden|Created/i);
    });

    it('unknown username on upsert with visibility writes nothing', async () => {
        process.env.MCP_AUTH_TOKEN = 'test-mcp-assignment-token';
        const slug = 'ac8-unknown';
        const result = await handlers.upsert_lesson({
            slug,
            worksheet: sampleWorksheet(slug),
            audio: false,
            visibility: { users: ['ghost'] }
        });
        assert.equal(result.isError, true);
        assert.match(replyText(result), /unknown|inactive/i);
        assert.equal(fs.existsSync(path.join(tmpLessons, slug, 'worksheet.js')), false);
    });
});
