/**
 * Learner MCP tools: token, switch, daily cap, unknown keys, and the
 * eight legacy tools.
 * Temporary database. Handlers are called directly. No network.
 * Provenance: INIT-006/SPEC-010
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { CATALOG, LEVEL_DEFAULTS } = require('../shared/activities/catalog');

const NEW_TOOLS = [
    'get_learner_profile',
    'check_text_coverage',
    'list_grammar_concepts',
    'list_lexicon',
    'get_lexicon_entry',
    'upsert_lexicon_entries',
    'add_library_words',
    'set_lexicon_verified',
    'set_ai_planning'
];
const LEGACY_TOOLS = [
    'list_lessons',
    'get_lesson',
    'list_words',
    'list_activity_types',
    'list_users',
    'upsert_lesson',
    'set_lesson_visibility',
    'generate_lesson_audio'
];
const LETTERS = 'אבגדהוזחטיכלמנסעפצקרשת';

function lexKey(n) {
    return LETTERS[Math.floor(n / LETTERS.length)] + LETTERS[n % LETTERS.length];
}

function replyText(result) {
    assert.ok(result && result.content && result.content[0]);
    return result.content[0].text;
}

function replyJson(result) {
    return JSON.parse(replyText(result));
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/db/migrations/004-fsrs-review-state',
        '../server/db/migrations/005-lexicon-library-mastery',
        '../server/accounts',
        '../server/db/repos/users',
        '../server/db/repos/user-words',
        '../server/db/repos/audit',
        '../server/learner/gate',
        '../server/learner/profile',
        '../server/learner/coverage',
        '../server/learner/library',
        '../server/lexicon/service',
        '../mcp/tools'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

describe('learner MCP tools (INIT-006/SPEC-010)', { concurrency: false }, () => {
    let tmp;
    let lessons;
    let db;
    let accounts;
    let usersRepo;
    let userWords;
    let lexicon;
    let gate;
    let profile;
    let tools;
    let handlers;
    let metas;
    let prevEnv;
    let learnerId;
    const token = 'test-learner-mcp-token';

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-learner-mcp-'));
        lessons = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-learner-mcp-lessons-'));
        fs.writeFileSync(path.join(lessons, 'catalog.js'), 'window.HEBREW_CATALOG = [];\n', 'utf8');
        prevEnv = {
            HEBREW_ROOT: process.env.HEBREW_ROOT,
            DATA_DIR: process.env.DATA_DIR,
            LESSONS_DIR: process.env.LESSONS_DIR,
            MCP_AUTH_TOKEN: process.env.MCP_AUTH_TOKEN
        };
        process.env.HEBREW_ROOT = path.resolve(__dirname, '..');
        process.env.DATA_DIR = tmp;
        process.env.LESSONS_DIR = lessons;
        process.env.MCP_AUTH_TOKEN = token;
        clearModules();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        accounts = require('../server/accounts');
        usersRepo = require('../server/db/repos/users');
        userWords = require('../server/db/repos/user-words');
        lexicon = require('../server/lexicon/service');
        gate = require('../server/learner/gate');
        profile = require('../server/learner/profile');
        tools = require('../mcp/tools');
        metas = Object.create(null);
        tools.registerLessonTools({
            registerTool(name, meta) {
                metas[name] = meta;
            }
        });
        handlers = tools.toolHandlers;
        const created = accounts.createUser({
            username: 'learner',
            password: 'a-strong-pass1',
            role: 'user',
            actor: 'test'
        });
        learnerId = created.id;
        accounts.createUser({
            username: 'quiet',
            password: 'a-strong-pass1',
            role: 'user',
            actor: 'test'
        });
    });

    beforeEach(() => {
        gate._resetMcpReads();
        process.env.MCP_AUTH_TOKEN = token;
        if (learnerId) usersRepo.setAiPlanning(learnerId, 0);
    });

    after(() => {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        try {
            fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(lessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        for (const [key, value] of Object.entries(prevEnv)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        clearModules();
    });

    function wordCount(userId) {
        return userWords.listByUser(userId).length;
    }

    function reviewCount(userId, word) {
        const row = db.get(
            'SELECT COUNT(*) AS n FROM review_items WHERE user_id = ? AND word = ?',
            userId,
            word
        );
        return row ? Number(row.n) : 0;
    }

    it('ac-2: every new tool refuses without MCP_AUTH_TOKEN', async () => {
        delete process.env.MCP_AUTH_TOKEN;
        for (const name of NEW_TOOLS) {
            const result = await handlers[name]({});
            assert.equal(result.isError, true, name);
            assert.match(replyText(result), /MCP_AUTH_TOKEN/, name);
        }
    });

    it('ac-2: get_learner_profile with the switch off returns planning_off only', async () => {
        const logs = [];
        const orig = console.log;
        console.log = (...args) => {
            logs.push(args.map((item) => String(item)).join(' '));
        };
        let result;
        try {
            result = await handlers.get_learner_profile({ username: 'learner' });
        } finally {
            console.log = orig;
        }
        assert.equal(result.isError, true);
        assert.deepEqual(replyJson(result), { error: 'planning_off' });
        const row = db.get(
            "SELECT actor, action, target, detail FROM audit_log WHERE action = 'learner_profile_read' ORDER BY id DESC LIMIT 1"
        );
        assert.equal(row.actor, 'mcp');
        assert.equal(row.target, 'learner');
        assert.equal(JSON.parse(row.detail).fields, 'none');
        assert.equal(logs.some((line) => line.includes('learner')), false);
        assert.match(metas.get_learner_profile.description, /planning_off/);
        assert.match(metas.get_learner_profile.description, /lesson file/);
        assert.match(metas.check_text_coverage.description, /planning_off/);
        assert.match(metas.add_library_words.description, /planning_off/);
        assert.match(metas.add_library_words.description, /unknown_keys/);
    });

    it('a profile with null strength is returned unchanged and now is not forwarded', async () => {
        usersRepo.setAiPlanning(learnerId, 1);
        const original = profile.buildProfile;
        let seen = null;
        profile.buildProfile = (learner, opts) => {
            seen = { learner, opts };
            return {
                username: 'learner',
                profile_version: 1,
                words: [{ word_id: 'אב', he_en: { stage: 'learning', strength: null } }]
            };
        };
        try {
            const result = await handlers.get_learner_profile({
                username: 'learner',
                now: '1999-01-01T00:00:00.000Z'
            });
            const body = replyJson(result);
            assert.equal(body.words[0].he_en.strength, null);
            assert.equal(replyText(result).includes('"strength": null'), true);
            assert.equal(seen.learner, 'learner');
            assert.deepEqual(seen.opts, { audience: 'mcp' });
        } finally {
            profile.buildProfile = original;
            usersRepo.setAiPlanning(learnerId, 0);
        }
    });

    it('coverage text over 4000 characters is invalid_text and is not copied into the reply', async () => {
        usersRepo.setAiPlanning(learnerId, 1);
        try {
            const result = await handlers.check_text_coverage({
                username: 'learner',
                text: 'א'.repeat(4001)
            });
            assert.deepEqual(replyJson(result), { error: 'invalid_text' });
            assert.equal(replyText(result).includes('אאא'), false);
        } finally {
            usersRepo.setAiPlanning(learnerId, 0);
        }
    });

    it('ac-4: an unknown lexicon key writes nothing', async () => {
        usersRepo.setAiPlanning(learnerId, 1);
        const known = lexKey(0);
        const missing = 'זרזיר';
        const wrote = lexicon.upsert(
            [{ key: known, kind: 'lemma', gloss_en: 'known' }],
            { actor: 'mcp', expectedRev: 0 }
        );
        assert.equal(wrote.ok, true, JSON.stringify(wrote));
        const beforeWords = wordCount(learnerId);
        const beforeReviews = reviewCount(learnerId, known);
        const result = await handlers.add_library_words({
            username: 'learner',
            words: [known, missing]
        });
        const body = replyJson(result);
        assert.equal(body.error, 'unknown_keys');
        assert.deepEqual(body.unknown_keys, [missing]);
        assert.equal(wordCount(learnerId), beforeWords);
        assert.equal(reviewCount(learnerId, known), beforeReviews);
        assert.equal(reviewCount(learnerId, missing), 0);
        usersRepo.setAiPlanning(learnerId, 0);
    });

    it('ac-4: the 51st word in a local day is refused and nothing past the cap is written', async () => {
        db.run('DELETE FROM user_words WHERE user_id = ?', learnerId);
        db.run('DELETE FROM review_items WHERE user_id = ?', learnerId);
        usersRepo.setAiPlanning(learnerId, 1);
        const keys = [];
        for (let n = 1; n <= 51; n += 1) keys.push(lexKey(n));
        const wrote = lexicon.upsert(
            keys.map((key, i) => ({ key, kind: 'lemma', gloss_en: 'w' + i })),
            { actor: 'mcp', expectedRev: 0 }
        );
        assert.equal(wrote.ok, true, JSON.stringify(wrote.errors || wrote.error));

        const first = await handlers.add_library_words({
            username: 'learner',
            words: keys.slice(0, 49)
        });
        assert.equal(first.isError, undefined, replyText(first));
        assert.equal(wordCount(learnerId), 49);

        const crossing = await handlers.add_library_words({
            username: 'learner',
            words: keys.slice(49, 51)
        });
        assert.deepEqual(replyJson(crossing), { error: 'daily_cap' });
        assert.equal(wordCount(learnerId), 49);
        assert.equal(reviewCount(learnerId, keys[49]), 0);
        assert.equal(reviewCount(learnerId, keys[50]), 0);

        const fiftieth = await handlers.add_library_words({
            username: 'learner',
            words: [keys[49]]
        });
        assert.equal(fiftieth.isError, undefined, replyText(fiftieth));
        assert.equal(wordCount(learnerId), 50);
        assert.equal(reviewCount(learnerId, keys[49]), 2);

        const fiftyFirst = await handlers.add_library_words({
            username: 'learner',
            words: [keys[50]]
        });
        assert.deepEqual(replyJson(fiftyFirst), { error: 'daily_cap' });
        assert.equal(wordCount(learnerId), 50);
        assert.equal(reviewCount(learnerId, keys[50]), 0);

        const audited = db.get(
            "SELECT detail FROM audit_log WHERE action = 'library_add' AND target = 'learner' ORDER BY id DESC LIMIT 1"
        );
        const detail = JSON.parse(audited.detail);
        assert.equal(detail.outcome, 'daily_cap');
        assert.equal(detail.count, 0);
        assert.equal(JSON.stringify(detail).includes(keys[50]), false);
        usersRepo.setAiPlanning(learnerId, 0);
    });

    it('add_library_words with the switch off returns planning_off and writes nothing', async () => {
        const quiet = usersRepo.findLearnerByUsername('quiet');
        const key = lexKey(0);
        const result = await handlers.add_library_words({
            username: 'quiet',
            words: [key]
        });
        assert.deepEqual(replyJson(result), { error: 'planning_off' });
        assert.equal(wordCount(quiet.id), 0);
        const row = db.get(
            "SELECT detail FROM audit_log WHERE action = 'library_add' AND target = 'quiet' ORDER BY id DESC LIMIT 1"
        );
        const detail = JSON.parse(row.detail);
        assert.equal(detail.outcome, 'planning_off');
        assert.equal(detail.count, 0);
        assert.equal(Object.prototype.hasOwnProperty.call(detail, 'words'), false);
    });

    it('verified input is ignored and the verify and switch tools stay forbidden', async () => {
        const key = 'מילה';
        const saved = await handlers.upsert_lexicon_entries({
            entries: [{ key, kind: 'lemma', gloss_en: 'word', verified: true, source: 'admin' }],
            expectedRev: 0
        });
        assert.equal(saved.isError, undefined, replyText(saved));
        const stored = lexicon.get(key);
        assert.equal(stored.entry.verified, 0);
        assert.equal(stored.entry.source, 'mcp');
        assert.equal(stored.entry.verified_by, null);

        const verify = await handlers.set_lexicon_verified({ key, verified: true });
        assert.deepEqual(replyJson(verify), { error: 'forbidden' });
        assert.equal(lexicon.get(key).entry.verified, 0);

        const flip = await handlers.set_ai_planning({ username: 'learner', enabled: true });
        assert.deepEqual(replyJson(flip), { error: 'forbidden' });
        assert.equal(usersRepo.getAiPlanning(learnerId), 0);
        const switches = db.get(
            "SELECT COUNT(*) AS n FROM audit_log WHERE action = 'ai_planning_switch'"
        );
        assert.equal(Number(switches.n), 0);
    });

    it('a library add does not consume the shared profile rate', async () => {
        usersRepo.setAiPlanning(learnerId, 1);
        const key = lexKey(0);
        const added = await handlers.add_library_words({
            username: 'learner',
            words: [key]
        });
        assert.notEqual(replyJson(added).error, 'rate_limited');
        for (let i = 0; i < 60; i += 1) {
            const result = await handlers.get_learner_profile({ username: 'learner' });
            assert.equal(result.isError, undefined, 'profile ' + i + ' ' + replyText(result));
        }
        const blocked = await handlers.get_learner_profile({ username: 'learner' });
        assert.equal(replyJson(blocked).error, 'rate_limited');
        usersRepo.setAiPlanning(learnerId, 0);
    });

    it('ac-5: the eight legacy tools keep their names and list shapes', async () => {
        for (const name of LEGACY_TOOLS) {
            assert.equal(typeof handlers[name], 'function', name);
        }
        const activities = replyJson(await handlers.list_activity_types());
        assert.deepEqual(activities.activities, CATALOG);
        assert.deepEqual(activities.levels, LEVEL_DEFAULTS);
        const users = replyJson(await handlers.list_users());
        assert.deepEqual(users.map((row) => row.username).sort(), ['learner', 'quiet']);
        for (const row of users) {
            assert.deepEqual(Object.keys(row).sort(), ['displayName', 'username']);
        }
        const words = replyJson(await handlers.list_words({}));
        assert.ok(Array.isArray(words));
    });
});
