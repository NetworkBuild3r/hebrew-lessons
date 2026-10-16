/**
 * Learner profile: the ADR-0009 §1 contract, the AI-planning switch, the
 * shared hourly MCP limit, and the audit row.
 * Temporary database. Does not open data/hebrew.sqlite. No network.
 * Authored-by-spec: INIT-006/SPEC-007
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const DAY_MS = 24 * 60 * 60 * 1000;
const HOUR_MS = 60 * 60 * 1000;
const ID_A = 7919;
const ID_B = 7920;
const ID_C = 7921;
const SECRET_NAME = 'Zelda Quartermaine';
const SECRET_HASH = 'scrypt$secret-hash-value';
const SECRET_TOKEN_HASH = 'session-token-hash-value';
const SECRET_NOTE = 'private note text';
const SECRET_TYPED = 'typed answer text';
const SECRET_GLOSS = 'he thought';
const OTHER_SLUG = 'b-only-lesson';
const ISO_DATE = /\d{4}-\d{2}-\d{2}/;
const HEBREW_LETTER = /[\u05D0-\u05EA]/;

function nfc(text) {
    return text.normalize('NFC');
}

const W = {
    cat: nfc('חָתוּל'),
    house: nfc('בַּיִת'),
    water: nfc('מַיִם'),
    peace: nfc('שָׁלוֹם'),
    city: nfc('עִיר'),
    father: nfc('אָב'),
    book: nfc('סֵפֶר'),
    dog: nfc('כֶּלֶב'),
    fish: nfc('דָּג'),
    library: nfc('מִלָּה'),
    other: nfc('אוֹרֶז')
};
const LEMMA = nfc('חָשַׁב');
const FORM = nfc('חָשַׁבְתָּ');
const FOUR = ['binyan:paal', 'cell:past.2ms', 'root-class:regular', 'tense:past'];

function atLocal(y, m, d, h = 12) {
    return new Date(y, m - 1, d, h, 0, 0, 0).getTime();
}

const NOW = atLocal(2026, 10, 10, 12);
const NOW_ISO = new Date(NOW).toISOString();

function daysAgo(n) {
    return new Date(NOW - n * DAY_MS).toISOString();
}

/** Every key and leaf value, with its path. Independent of profile-contract.fieldPaths. */
function walk(value, visit, at) {
    const here = at || '$';
    if (Array.isArray(value)) {
        value.forEach((item, i) => walk(item, visit, here + '[' + i + ']'));
        return;
    }
    if (value !== null && typeof value === 'object') {
        for (const key of Object.keys(value)) {
            visit('key', key, here);
            walk(value[key], visit, here + '.' + key);
        }
        return;
    }
    visit('value', value, here);
}

/** Field paths in RETURNED_FIELDS notation, written separately from the module's helper. */
function pathsOf(value, maps) {
    const out = new Set();
    function step(node, prefix) {
        if (Array.isArray(node)) {
            for (const item of node) {
                if (item !== null && typeof item === 'object') step(item, prefix + '[]');
            }
            return;
        }
        if (node === null || typeof node !== 'object') return;
        for (const key of Object.keys(node)) {
            if (maps.includes(prefix)) {
                out.add(prefix + '.*');
                continue;
            }
            const next = prefix ? prefix + '.' + key : key;
            out.add(next);
            step(node[key], next);
        }
    }
    step(value, '');
    return [...out].sort();
}

function isStep(value) {
    return typeof value === 'number' && value >= 0 && value <= 1
        && Math.abs(value * 20 - Math.round(value * 20)) < 1e-9;
}

describe('learner profile (INIT-006/SPEC-007)', () => {
    let tmp;
    let savedScheduler;
    let db;
    let users;
    let review;
    let events;
    let lexicon;
    let lessons;
    let userWords;
    let progress;
    let contract;
    let gate;
    let profile;
    let coverage;
    let seq = 0;

    const cacheNames = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/users',
        '../server/db/repos/review',
        '../server/db/repos/events',
        '../server/db/repos/lesson-progress',
        '../server/db/repos/lexicon',
        '../server/db/repos/concepts',
        '../server/db/repos/concept-mastery',
        '../server/db/repos/audit',
        '../server/db/repos/user-words',
        '../server/db/repos/coverage',
        '../server/plans/store',
        '../server/learner/thresholds',
        '../server/learner/model',
        '../server/learner/profile-contract',
        '../server/learner/gate',
        '../server/learner/profile',
        '../server/learner/coverage',
        '../server/progress'
    ];

    function dropCache() {
        for (const name of cacheNames) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* not loaded */ }
        }
    }

    function addUser(id, username, opts) {
        db.run(
            `INSERT INTO users (
                id, username, password_hash, display_name, role, status, auth_provider,
                provider_subject, must_change_password, created_at, updated_at, ai_planning
            ) VALUES (?, ?, ?, ?, 'user', ?, 'local', NULL, 0, ?, ?, ?)`,
            id,
            username,
            SECRET_HASH,
            SECRET_NAME,
            opts.status || 'active',
            NOW_ISO,
            NOW_ISO,
            opts.aiPlanning ? 1 : 0
        );
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-profile-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        savedScheduler = process.env.SRS_SCHEDULER;
        process.env.SRS_SCHEDULER = 'ladder';
        dropCache();
        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        users = require('../server/db/repos/users');
        review = require('../server/db/repos/review');
        events = require('../server/db/repos/events');
        lexicon = require('../server/db/repos/lexicon');
        lessons = require('../server/db/repos/lesson-progress');
        userWords = require('../server/db/repos/user-words');
        progress = require('../server/progress');
        contract = require('../server/learner/profile-contract');
        gate = require('../server/learner/gate');
        profile = require('../server/learner/profile');
        coverage = require('../server/learner/coverage');
        addUser(ID_A, 'learner-a', { aiPlanning: true });
        addUser(ID_B, 'learner-b', { aiPlanning: true });
        addUser(ID_C, 'learner-c', { aiPlanning: true, status: 'disabled' });
        db.run(
            `INSERT INTO sessions (user_id, token_hash, created_at, last_seen_at, expires_at)
             VALUES (?, ?, ?, ?, ?)`,
            ID_A,
            SECRET_TOKEN_HASH,
            NOW_ISO,
            NOW_ISO,
            new Date(NOW + DAY_MS).toISOString()
        );
    });

    after(() => {
        if (gate) gate._setClock(null);
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        if (savedScheduler === undefined) delete process.env.SRS_SCHEDULER;
        else process.env.SRS_SCHEDULER = savedScheduler;
        delete process.env.DATA_DIR;
        dropCache();
    });

    beforeEach(() => {
        process.env.SRS_SCHEDULER = 'ladder';
        for (const table of [
            'concept_mastery', 'practice_events', 'review_items', 'lesson_progress',
            'user_words', 'lexicon_entries', 'plans', 'audit_log'
        ]) {
            db.run('DELETE FROM ' + table);
        }
        users.setAiPlanning(ID_A, 1);
        users.setAiPlanning(ID_B, 1);
        gate._resetMcpReads();
        gate._setClock(() => NOW);
    });

    function card(overrides) {
        review.upsert(Object.assign({
            user_id: ID_A,
            word: W.book,
            direction: 'he_en',
            due: new Date(NOW + 3 * DAY_MS).toISOString(),
            step: 1,
            streak: 1,
            lapses: 0,
            last_result: 'pass',
            last_at: NOW_ISO,
            sources: JSON.stringify(['cafe-order']),
            last_grade: 3,
            suspended: 0
        }, overrides));
    }

    function signal(partial) {
        seq += 1;
        return Object.assign({
            id: 'profile-' + seq,
            word: FORM,
            kind: 'form',
            lemma: LEMMA,
            result: 'pass',
            at: new Date(NOW - DAY_MS + seq * 1000).toISOString(),
            slug: 'cafe-order',
            activity: 'conjugation-drill'
        }, partial);
    }

    function lex(row) {
        lexicon.upsert(Object.assign({
            kind: 'lemma',
            lemma_key: null,
            gloss_en: SECRET_GLOSS,
            pos: 'verb',
            root: null,
            binyan: null,
            root_class: null,
            tense: null,
            cell: null,
            translit: 'chashav',
            accept_json: '[]',
            verified: 1,
            source: 'admin',
            rev: 1,
            updated_at: NOW_ISO
        }, row));
    }

    /**
     * Expected words[] order (ADR-0009 §3): very_weak by strength, weak by
     * strength then lapses, then the rest by key.
     */
    function seedProfile() {
        card({ word: W.cat, direction: 'en_he', step: 0, streak: 0, lapses: 12,
            last_result: 'miss', last_grade: 1, last_at: daysAgo(10) });
        card({ word: W.house, step: 1, lapses: 3, last_result: 'miss', last_grade: 1,
            last_at: daysAgo(7) });
        card({ word: W.water, step: 0, streak: 0, lapses: 1, last_result: 'miss', last_grade: 1,
            last_at: daysAgo(3) });
        card({ word: W.peace, step: 3, streak: 0, lapses: 4, last_result: 'miss', last_grade: 1,
            last_at: daysAgo(1) });
        card({ word: W.peace, direction: 'en_he', step: 1 });
        card({ word: W.city, step: 0, lapses: 2 });
        card({ word: W.father, step: 1 });
        card({ word: W.book, step: 2, last_at: daysAgo(2), suspended: 1,
            sources: JSON.stringify(['cafe-order', 'letters']) });
        card({ word: W.dog, step: 0, streak: 0, last_result: null, last_at: null, last_grade: null });
        card({ word: W.fish, step: 0, streak: 0, last_result: 'not_yet', last_grade: null });

        events.insert({
            id: 'typed-1',
            user_id: ID_A,
            word: W.book,
            direction: 'he_en',
            result: 'pass',
            at: daysAgo(2),
            slug: 'cafe-order',
            activity: 'typed-recall',
            grade: 3,
            kind: 'word',
            lemma: null
        });
        userWords.add({ user_id: ID_A, word: W.library, source: 'self', added_at: daysAgo(4),
            gloss_en: SECRET_NOTE });

        lex({ key: LEMMA, binyan: 'paal', root_class: 'regular', root: 'ח.ש.ב' });
        lex({ key: FORM, kind: 'form', lemma_key: LEMMA, gloss_en: null, pos: 'verb',
            tense: 'past', cell: '2ms' });
        progress.applySignals(ID_A, [
            signal({ result: 'pass' }),
            signal({ result: 'miss' }),
            signal({ result: 'pass' })
        ], NOW);

        lessons.upsert({
            user_id: ID_A,
            slug: 'cafe-order',
            state_json: JSON.stringify({
                completedRounds: 2,
                vowelsOn: false,
                round: 3,
                roundStartedAt: NOW - HOUR_MS,
                stepIndex: 2,
                completedSteps: { 0: true, 1: true },
                wordMarks: { [W.book]: 'known' },
                missedLast: [W.house],
                activityState: { typed: SECRET_TYPED }
            }),
            rev: 4,
            updated_at: daysAgo(1)
        });
        lessons.upsert({
            user_id: ID_A,
            slug: 'letters',
            state_json: JSON.stringify({ completedRounds: 0, vowelsOn: true }),
            rev: 1,
            updated_at: daysAgo(1)
        });
        db.run(
            `INSERT INTO plans (
                id, revision, status, audience, assignee_id, working_copy, snapshot, created_at, updated_at
            ) VALUES ('unit-1', 1, 'published', 'shared', NULL, ?, ?, ?, ?)`,
            '{"id":"unit-1"}',
            JSON.stringify({ units: [{ lessons: [{ id: 'cafe-order', recipe: 'topic-conversation' }] }] }),
            NOW_ISO,
            NOW_ISO
        );

        review.upsert({
            user_id: ID_B,
            word: W.other,
            direction: 'he_en',
            due: NOW_ISO,
            step: 0,
            streak: 0,
            lapses: 5,
            last_result: 'miss',
            last_at: NOW_ISO,
            sources: JSON.stringify([OTHER_SLUG]),
            last_grade: 1
        });
        lessons.upsert({
            user_id: ID_B,
            slug: OTHER_SLUG,
            state_json: JSON.stringify({ completedRounds: 5, vowelsOn: false }),
            rev: 1,
            updated_at: NOW_ISO
        });
    }

    function auditRows(action) {
        return db.all(
            'SELECT actor, action, target, at, detail FROM audit_log WHERE action = ? ORDER BY id',
            action
        ).map((row) => Object.assign({}, row, { detail: JSON.parse(row.detail) }));
    }

    function mcpProfile(learner, now) {
        return profile.buildProfile(learner, { audience: 'mcp', now: now == null ? NOW : now });
    }

    it('ac-1: the profile holds every ADR-0009 §1 field, in the ADR order, with no other key', () => {
        seedProfile();
        const out = mcpProfile(ID_A);

        assert.deepEqual(pathsOf(out, contract.MAP_FIELDS), contract.RETURNED_FIELDS.slice().sort());
        assert.deepEqual(contract.fieldPaths(out), contract.RETURNED_FIELDS.slice().sort());

        assert.equal(out.username, 'learner-a');
        assert.equal(out.profile_version, 1);
        assert.equal(out.words_total, 7);
        assert.deepEqual(out.words, [
            { word_id: W.cat, en_he: { stage: 'new', strength: 'fragile' }, weak: 'very_weak', lapses: 9 },
            { word_id: W.house, he_en: { stage: 'learning', strength: 'shaky' }, weak: 'very_weak', lapses: 3 },
            { word_id: W.water, he_en: { stage: 'new', strength: 'shaky' }, weak: 'weak', lapses: 1 },
            {
                word_id: W.peace,
                he_en: { stage: 'longTerm', strength: 'solid' },
                en_he: { stage: 'learning', strength: 'solid' },
                weak: 'weak',
                lapses: 4
            },
            { word_id: W.city, he_en: { stage: 'learning', strength: 'solid' }, weak: 'weak', lapses: 2 },
            { word_id: W.father, he_en: { stage: 'learning', strength: 'solid' }, lapses: 0 },
            { word_id: W.book, he_en: { stage: 'learning', strength: 'solid' }, lapses: 0 }
        ]);
        assert.deepEqual(out.very_weak_words, [W.cat, W.house]);
        assert.deepEqual(out.weak_words, [W.water, W.peace, W.city]);

        assert.deepEqual(out.journey, {
            stage: 'script',
            counts: {
                he_en: { new: 3, learning: 4, longTerm: 1 },
                en_he: { new: 1, learning: 1, longTerm: 0 }
            },
            lessons_completed: ['cafe-order'],
            recipes_completed: { 'topic-conversation': 1 },
            tenses_practiced: ['past'],
            binyanim_practiced: ['paal'],
            vowels_off_share: 0.5
        });

        assert.equal(out.grammar.length, 32);
        const seen = out.grammar.filter((row) => row.attempts > 0);
        assert.deepEqual(seen.map((row) => row.concept).sort(), FOUR);
        for (const row of seen) {
            assert.deepEqual(row, {
                concept: row.concept,
                status: 'emerging',
                attempts: 3,
                accuracy: 0.65,
                recent: 0.85
            });
        }
        for (const row of out.grammar.filter((item) => item.attempts === 0)) {
            assert.deepEqual(Object.keys(row), ['concept', 'status', 'attempts']);
            assert.equal(row.status, 'unseen');
        }
    });

    it('ac-1: a key and value walk finds no never-returned name, timestamp, user id, or other learner', () => {
        seedProfile();
        const out = mcpProfile(ID_A);
        const never = new Set(contract.NEVER_RETURNED);
        const ids = [ID_A, ID_B, ID_C];
        const secrets = [
            SECRET_NAME, SECRET_HASH, SECRET_TOKEN_HASH, SECRET_NOTE, SECRET_TYPED, SECRET_GLOSS,
            'chashav', W.other, OTHER_SLUG, W.library, W.dog, W.fish, 'typed-recall', 'letters'
        ];
        let visited = 0;
        walk(out, (kind, item, at) => {
            visited += 1;
            const label = kind + ' at ' + at + ': ' + String(item);
            if (kind === 'key') {
                assert.ok(!never.has(item), 'never-returned ' + label);
                assert.doesNotMatch(item, ISO_DATE, label);
                assert.ok(!ids.some((id) => item.includes(String(id))), 'user id ' + label);
                return;
            }
            if (typeof item === 'number') {
                assert.ok(!ids.includes(item), 'user id ' + label);
                assert.ok(Number.isFinite(item) && Math.abs(item) < 1e6, 'timestamp-sized ' + label);
            }
            if (typeof item === 'string') {
                assert.ok(!never.has(item), 'never-returned ' + label);
                assert.doesNotMatch(item, ISO_DATE, 'timestamp ' + label);
                assert.doesNotMatch(item, /\d{1,2}:\d{2}/, 'time of day ' + label);
                assert.ok(!ids.some((id) => item.includes(String(id))), 'user id ' + label);
                for (const secret of secrets) {
                    assert.ok(!item.includes(secret), 'secret ' + JSON.stringify(secret) + ' ' + label);
                }
            }
        });
        assert.ok(visited > 100);
        const body = JSON.stringify(out);
        for (const secret of secrets) assert.ok(!body.includes(secret), secret);

        for (const name of contract.NEVER_RETURNED) {
            assert.ok(!contract.RETURNED_FIELDS.some((p) => p.split(/[.[\]]+/).includes(name)), name);
            assert.ok(!contract.SELF_FIELDS.includes(name), name);
        }
    });

    it('ac-1: words[] stops at 500 and each flag list at 50, with lapses capped at 9', () => {
        const letters = Array.from({ length: 22 }, (_, i) => String.fromCharCode(0x05D0 + i))
            .filter((ch) => !'ךםןףץ'.includes(ch));
        function wordFor(i) {
            const a = letters[i % letters.length];
            const b = letters[Math.floor(i / letters.length) % letters.length];
            const c = letters[Math.floor(i / (letters.length * letters.length)) % letters.length];
            return a + b + c + 'ה';
        }
        const insert = db.prepare(
            `INSERT INTO review_items (
                user_id, word, direction, due, step, streak, lapses, last_result, last_at, sources, last_grade
            ) VALUES (?, ?, 'he_en', ?, ?, 0, ?, ?, ?, '[]', ?)`
        );
        db.transaction(() => {
            for (let i = 0; i < 520; i += 1) {
                if (i < 60) insert.run(ID_A, wordFor(i), NOW_ISO, 0, i % 2 ? 15 : 3, 'miss', NOW_ISO, 1);
                else if (i < 120) insert.run(ID_A, wordFor(i), NOW_ISO, 0, 2, 'pass', NOW_ISO, 3);
                else insert.run(ID_A, wordFor(i), NOW_ISO, 1, 0, 'pass', NOW_ISO, 3);
            }
        });
        const out = mcpProfile(ID_A);
        assert.equal(out.words_total, 520);
        assert.equal(out.words.length, 500);
        assert.equal(out.very_weak_words.length, 50);
        assert.equal(out.weak_words.length, 50);
        assert.ok(out.words.slice(0, 60).every((w) => w.weak === 'very_weak'));
        assert.ok(out.words.slice(60, 120).every((w) => w.weak === 'weak'));
        assert.ok(out.words.slice(120).every((w) => w.weak === undefined));
        assert.ok(out.words.every((w) => Number.isInteger(w.lapses) && w.lapses >= 0 && w.lapses <= 9));
        assert.ok(out.words.slice(0, 30).every((w) => w.lapses === 9));
        assert.deepEqual(out.very_weak_words, out.words.slice(0, 50).map((w) => w.word_id));
        assert.deepEqual(out.weak_words, out.words.slice(60, 110).map((w) => w.word_id));
        for (const row of out.grammar) {
            if (row.attempts > 0) assert.ok(isStep(row.accuracy) && isStep(row.recent));
        }
    });

    it('a direction with no strength estimate reports null and sorts after solid', () => {
        process.env.SRS_SCHEDULER = 'fsrs';
        try {
            card({ word: W.book, step: 1, stability: 30, difficulty: 5, reps: 2, fsrs_state: 'review' });
            card({ word: W.father, step: 1 });
            const out = mcpProfile(ID_A);
            assert.ok(W.father < W.book);
            assert.deepEqual(out.words, [
                { word_id: W.book, he_en: { stage: 'longTerm', strength: 'solid' }, lapses: 0 },
                { word_id: W.father, he_en: { stage: 'learning', strength: null }, lapses: 0 }
            ]);
        } finally {
            process.env.SRS_SCHEDULER = 'ladder';
        }
    });

    it('ac-2: switch off returns planning_off and nothing else, and audits fields none', () => {
        seedProfile();
        users.setAiPlanning(ID_A, 0);
        assert.deepEqual(mcpProfile(ID_A), { error: 'planning_off' });
        assert.deepEqual(mcpProfile('Learner-A'), { error: 'planning_off' });
        const rows = auditRows('learner_profile_read');
        assert.equal(rows.length, 2);
        for (const row of rows) {
            assert.equal(row.actor, 'mcp');
            assert.equal(row.target, 'learner-a');
            assert.equal(row.at, NOW_ISO);
            assert.deepEqual(row.detail, { outcome: 'planning_off', fields: 'none' });
        }
    });

    it('ac-2: a missing or disabled account is not_found for the MCP, audited with fields none', () => {
        assert.deepEqual(mcpProfile('learner-c'), { error: 'not_found' });
        assert.deepEqual(mcpProfile(ID_C), { error: 'not_found' });
        assert.deepEqual(mcpProfile('nobody'), { error: 'not_found' });
        assert.deepEqual(mcpProfile('x'.repeat(65)), { error: 'not_found' });
        assert.deepEqual(mcpProfile({ username: 'learner-a' }), { error: 'not_found' });
        const rows = auditRows('learner_profile_read');
        assert.deepEqual(rows.map((row) => row.target), ['learner-c', 'learner-c', null, null, null]);
        for (const row of rows) assert.deepEqual(row.detail, { outcome: 'not_found', fields: 'none' });
        assert.deepEqual(profile.buildProfile(ID_C, { audience: 'self', now: NOW }), { error: 'not_found' });
        assert.equal(auditRows('learner_profile_read').length, 5);
    });

    it('ac-3: the 61st MCP read in a clock hour, profile and coverage combined, is rate_limited and audited', () => {
        seedProfile();
        users.setAiPlanning(ID_B, 0);
        const hour = Date.UTC(2026, 9, 10, 15, 0, 0);
        let clockMs = hour + 5 * 60 * 1000;
        let calls = 0;
        gate._setClock(() => clockMs);
        function tick() {
            calls += 1;
            clockMs += 1000;
        }
        for (let i = 0; i < 29; i += 1) {
            tick();
            assert.equal(mcpProfile(ID_A).username, 'learner-a');
            tick();
            assert.equal(typeof coverage.checkCoverage(ID_A, W.peace, { audience: 'mcp' }).coverage, 'number');
        }
        tick();
        assert.deepEqual(mcpProfile(ID_B), { error: 'planning_off' });
        tick();
        assert.deepEqual(coverage.checkCoverage('nobody', W.peace, { audience: 'mcp' }), { error: 'not_found' });
        assert.equal(calls, 60);

        assert.equal(profile.buildProfile(ID_A, { audience: 'self', now: NOW }).username, 'learner-a');

        tick();
        const limitedAt = new Date(clockMs).toISOString();
        assert.deepEqual(mcpProfile(ID_A), { error: 'rate_limited' });
        tick();
        assert.deepEqual(coverage.checkCoverage(ID_A, W.peace, { audience: 'mcp' }), { error: 'rate_limited' });

        const profileRows = auditRows('learner_profile_read');
        const coverageRows = auditRows('learner_coverage_read');
        assert.equal(profileRows.length + coverageRows.length, 62);
        const limited = profileRows[profileRows.length - 1];
        assert.equal(limited.actor, 'mcp');
        assert.equal(limited.target, 'learner-a');
        assert.equal(limited.at, limitedAt);
        assert.deepEqual(limited.detail, { outcome: 'rate_limited', fields: 'none' });
        assert.deepEqual(coverageRows[coverageRows.length - 1].detail,
            { outcome: 'rate_limited', fields: 'none' });

        clockMs = hour + HOUR_MS;
        assert.equal(mcpProfile(ID_A).username, 'learner-a');
        assert.equal(auditRows('learner_profile_read').pop().detail.outcome, 'ok');
    });

    it('a caller-supplied now does not reset the hourly count or set the audit time', () => {
        seedProfile();
        for (let i = 0; i < 60; i += 1) assert.equal(mcpProfile(ID_A).username, 'learner-a');
        const future = NOW + 30 * 24 * HOUR_MS;
        assert.deepEqual(mcpProfile(ID_A, future), { error: 'rate_limited' });
        assert.deepEqual(coverage.checkCoverage(ID_A, W.peace, { audience: 'mcp', now: future }),
            { error: 'rate_limited' });
        const rows = auditRows('learner_profile_read').concat(auditRows('learner_coverage_read'));
        assert.equal(rows.length, 62);
        assert.ok(rows.every((row) => row.at === NOW_ISO));
    });

    it('ac-4: self returns the MCP object plus ai_planning, whatever the switch, with no audit and no rate count', () => {
        seedProfile();
        const forMcp = mcpProfile(ID_A);
        const before = auditRows('learner_profile_read').length;
        const forSelf = profile.buildProfile(ID_A, { audience: 'self', now: NOW });
        assert.equal(forSelf.ai_planning, true);
        const { ai_planning: switchOn, ...withoutSwitch } = forSelf;
        assert.equal(switchOn, true);
        assert.deepEqual(withoutSwitch, forMcp);
        assert.deepEqual(contract.fieldPaths(forSelf),
            contract.RETURNED_FIELDS.concat(contract.SELF_FIELDS).sort());

        users.setAiPlanning(ID_A, 0);
        const offSelf = profile.buildProfile(ID_A, { audience: 'self', now: NOW });
        assert.equal(offSelf.ai_planning, false);
        const { ai_planning: switchOff, ...offRest } = offSelf;
        assert.equal(switchOff, false);
        assert.deepEqual(offRest, forMcp);
        assert.equal(auditRows('learner_profile_read').length, before);

        users.setAiPlanning(ID_A, 1);
        gate._resetMcpReads();
        for (let i = 0; i < 5; i += 1) profile.buildProfile(ID_A, { audience: 'self', now: NOW });
        for (let i = 0; i < 60; i += 1) assert.equal(mcpProfile(ID_A).username, 'learner-a');
        assert.deepEqual(mcpProfile(ID_A), { error: 'rate_limited' });
        assert.equal(profile.buildProfile(ID_A, { audience: 'self', now: NOW }).username, 'learner-a');
    });

    it('an ok audit row holds the outcome, field names, and counts only', () => {
        seedProfile();
        const out = mcpProfile('learner-a');
        const rows = auditRows('learner_profile_read');
        assert.equal(rows.length, 1);
        const row = rows[0];
        assert.equal(row.actor, 'mcp');
        assert.equal(row.target, 'learner-a');
        assert.equal(row.at, NOW_ISO);
        assert.deepEqual(Object.keys(row.detail).sort(),
            ['concept_count', 'fields', 'lesson_count', 'outcome', 'word_count']);
        assert.equal(row.detail.outcome, 'ok');
        assert.deepEqual(row.detail.fields, contract.fieldPaths(out));
        assert.ok(row.detail.fields.every((field) => contract.RETURNED_FIELDS.includes(field)));
        assert.equal(row.detail.word_count, 7);
        assert.equal(row.detail.concept_count, 32);
        assert.equal(row.detail.lesson_count, 1);
        const stored = db.get('SELECT detail FROM audit_log WHERE action = ?', 'learner_profile_read').detail;
        assert.doesNotMatch(stored, HEBREW_LETTER);
        assert.doesNotMatch(stored, ISO_DATE);
    });

    it('profile and coverage queries name their columns and never read account secrets', () => {
        seedProfile();
        const seen = [];
        const originals = { get: db.get, all: db.all, run: db.run, prepare: db.prepare };
        for (const name of Object.keys(originals)) {
            db[name] = function (sql, ...params) {
                seen.push(String(sql));
                return originals[name].call(this, sql, ...params);
            };
        }
        try {
            mcpProfile(ID_A);
            mcpProfile('learner-a');
            profile.buildProfile(ID_A, { audience: 'self', now: NOW });
            coverage.checkCoverage('learner-a', W.peace + ' ' + W.book, { audience: 'mcp', now: NOW });
            coverage.checkCoverage(ID_A, W.peace, { audience: 'self', now: NOW });
        } finally {
            Object.assign(db, originals);
        }
        assert.ok(seen.length > 5);
        for (const sql of seen) {
            assert.doesNotMatch(sql, /SELECT\s+\*/i, sql);
            assert.doesNotMatch(sql, /password_hash|display_name|token_hash|\bsessions\b/i, sql);
        }
    });

    it('a self read takes only the signed-in user id, and the audience is required', () => {
        assert.throws(() => profile.buildProfile('learner-a', { audience: 'self' }), TypeError);
        assert.throws(() => profile.buildProfile(ID_A, {}), TypeError);
        assert.throws(() => profile.buildProfile(ID_A, { audience: 'admin' }), TypeError);
        assert.throws(() => profile.buildProfile(ID_A), TypeError);
        assert.equal(auditRows('learner_profile_read').length, 0);
    });

    it('a now that cannot be read throws before the call is counted', () => {
        seedProfile();
        const broken = { toString() { throw new Error('bad clock'); } };
        assert.throws(() => mcpProfile(ID_A, broken), /bad clock/);
        assert.equal(auditRows('learner_profile_read').length, 0);
        for (let i = 0; i < 60; i += 1) assert.equal(mcpProfile(ID_A).username, 'learner-a');
        assert.deepEqual(mcpProfile(ID_A), { error: 'rate_limited' });
    });

    it('a read that fails is audited as error and returns nothing', () => {
        seedProfile();
        const model = require('../server/learner/model');
        const original = model.conceptStates;
        model.conceptStates = () => { throw new Error('boom'); };
        try {
            assert.throws(() => mcpProfile(ID_A), /boom/);
        } finally {
            model.conceptStates = original;
        }
        const rows = auditRows('learner_profile_read');
        assert.equal(rows.length, 1);
        assert.deepEqual(rows[0].detail, { outcome: 'error', fields: 'none' });
    });
});
