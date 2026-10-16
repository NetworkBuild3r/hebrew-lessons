/**
 * Known-word coverage: tokenizing, the two "known" rules, no prefix stripping,
 * the unknown list, the AI-planning switch, and the audit row.
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
const ID_A = 7919;
const ID_B = 7920;
const HEBREW_LETTER = /[\u05D0-\u05EA]/;
const NOW = Date.UTC(2026, 9, 10, 18, 0, 0);
const NOW_ISO = new Date(NOW).toISOString();

function nfc(text) {
    return text.normalize('NFC');
}

/** Passed review rows for learner A. הִיא is passed in en_he only. */
const PASSED = [
    'קָמָה', 'שׁוֹתֶה', 'קָפֶה', 'בַּבֹּקֶר', 'הוּא', 'גַּם', 'אוֹכֵל', 'לֶחֶם',
    'עִם', 'חָלָב', 'טוֹב', 'יוֹם', 'אֲנִי', 'חָשַׁב', 'בַּיִת'
].map(nfc);
const EN_HE_ONLY = nfc('הִיא');
const MISSED_ONLY = nfc('מַיִם');
const LIBRARY_ONLY = nfc('סֵפֶר');
const LEMMA = nfc('חָשַׁב');
const FORM = nfc('חָשַׁבְתָּ');
const ROSE = nfc('קָמָה');
const PREFIXED = nfc('וְקָמָה');
const CAR = nfc('מְכוֹנִית');
const OTHER = nfc('אוֹרֶז');

/**
 * Twenty Hebrew running words, eighteen known: passed rows, an unpointed form
 * that is unique once stripped (קפה), a library-only word, a never-passed
 * library word, a lexicon form of a known lemma, an en_he-only pass, and a
 * repeat. מְכוֹנִית is unknown, and אוֹרֶז is learner B's word. "OK" and "3"
 * are non-Hebrew and do not count.
 */
const TWENTY = nfc('קָמָה, שׁוֹתֶה קפה בַּבֹּקֶר. הוּא גַּם אוֹכֵל לֶחֶם עִם חָלָב! '
    + 'סֵפֶר מַיִם חָשַׁבְתָּ "טוֹב" יוֹם הִיא אֲנִי קָמָה מְכוֹנִית אוֹרֶז OK 3');

describe('learner coverage (INIT-006/SPEC-007)', () => {
    let tmp;
    let db;
    let users;
    let review;
    let lexicon;
    let userWords;
    let contract;
    let gate;
    let coverage;

    const cacheNames = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/users',
        '../server/db/repos/review',
        '../server/db/repos/lexicon',
        '../server/db/repos/audit',
        '../server/db/repos/user-words',
        '../server/db/repos/coverage',
        '../server/learner/profile-contract',
        '../server/learner/gate',
        '../server/learner/coverage'
    ];

    function dropCache() {
        for (const name of cacheNames) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* not loaded */ }
        }
    }

    function addUser(id, username) {
        db.run(
            `INSERT INTO users (
                id, username, password_hash, display_name, role, status, auth_provider,
                must_change_password, created_at, updated_at, ai_planning
            ) VALUES (?, ?, 'h', 'Name', 'user', 'active', 'local', 0, ?, ?, 1)`,
            id,
            username,
            NOW_ISO,
            NOW_ISO
        );
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-coverage-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        dropCache();
        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        users = require('../server/db/repos/users');
        review = require('../server/db/repos/review');
        lexicon = require('../server/db/repos/lexicon');
        userWords = require('../server/db/repos/user-words');
        contract = require('../server/learner/profile-contract');
        gate = require('../server/learner/gate');
        coverage = require('../server/learner/coverage');
        addUser(ID_A, 'learner-a');
        addUser(ID_B, 'learner-b');
    });

    after(() => {
        if (gate) gate._setClock(null);
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        dropCache();
    });

    beforeEach(() => {
        for (const table of ['review_items', 'user_words', 'lexicon_entries', 'audit_log']) {
            db.run('DELETE FROM ' + table);
        }
        users.setAiPlanning(ID_A, 1);
        users.setAiPlanning(ID_B, 1);
        gate._resetMcpReads();
        gate._setClock(() => NOW);
    });

    function card(userId, word, overrides) {
        review.upsert(Object.assign({
            user_id: userId,
            word,
            direction: 'he_en',
            due: NOW_ISO,
            step: 1,
            streak: 1,
            lapses: 0,
            last_result: 'pass',
            last_at: NOW_ISO,
            sources: '[]',
            last_grade: 3
        }, overrides));
    }

    function lex(row) {
        lexicon.upsert(Object.assign({
            kind: 'lemma',
            lemma_key: null,
            gloss_en: null,
            accept_json: '[]',
            verified: 0,
            source: 'admin',
            rev: 1,
            updated_at: NOW_ISO
        }, row));
    }

    function seed() {
        for (const word of PASSED) card(ID_A, word);
        card(ID_A, EN_HE_ONLY, { direction: 'en_he' });
        card(ID_A, MISSED_ONLY, { step: 0, streak: 0, lapses: 1, last_result: 'miss', last_grade: 1 });
        userWords.add({ user_id: ID_A, word: LIBRARY_ONLY, source: 'self', gloss_en: 'book' });
        lex({ key: LEMMA, pos: 'verb' });
        lex({ key: FORM, kind: 'form', lemma_key: LEMMA, tense: 'past', cell: '2ms' });
        card(ID_B, OTHER);
    }

    function self(text, learner) {
        return coverage.checkCoverage(learner == null ? ID_A : learner, text, { audience: 'self' });
    }

    function mcp(text, learner) {
        return coverage.checkCoverage(learner == null ? 'learner-a' : learner, text, { audience: 'mcp' });
    }

    function auditRows() {
        return db.all(
            `SELECT actor, action, target, at, detail FROM audit_log
             WHERE action = 'learner_coverage_read' ORDER BY id`
        ).map((row) => Object.assign({}, row, { detail: JSON.parse(row.detail) }));
    }

    it('ac-5: twenty Hebrew words with eighteen known give 0.9 and list the other two', () => {
        seed();
        const result = self(TWENTY);
        assert.deepEqual(result, {
            coverage: 0.9,
            known_count: 18,
            running_words: 20,
            unknown: [CAR, OTHER]
        });
        assert.deepEqual(Object.keys(result).sort(), contract.COVERAGE_FIELDS.slice().sort());
        assert.equal('matched_by' in result, false);
        assert.deepEqual(mcp(TWENTY), result);
    });

    it('ac-5: a known word with a leading ו is unknown until a form entry names the known word as its lemma', () => {
        seed();
        const text = TWENTY + ' ' + PREFIXED;
        const before = self(text);
        assert.equal(before.running_words, 21);
        assert.equal(before.known_count, 18);
        assert.equal(before.coverage, 18 / 21);
        assert.deepEqual(before.unknown, [CAR, OTHER, PREFIXED]);

        lex({ key: PREFIXED, kind: 'form', lemma_key: ROSE });
        const after = self(text);
        assert.equal(after.known_count, 19);
        assert.equal(after.coverage, 19 / 21);
        assert.deepEqual(after.unknown, [CAR, OTHER]);
        assert.equal('matched_by' in after, false);
    });

    it('a stripped form counts only when it is unique, and an exact key is never re-read as another word', () => {
        seed();
        assert.equal(self('קפה').known_count, 1);
        const other = nfc('קִפָּה');
        lex({ key: other });
        assert.deepEqual(self('קפה'), { coverage: 0, known_count: 0, running_words: 1, unknown: ['קפה'] });
        assert.deepEqual(self(other), { coverage: 0, known_count: 0, running_words: 1, unknown: [other] });
        assert.equal(self(nfc('קָפֶה')).known_count, 1);
    });

    it('another learner\'s words are not known, and the same word is known to its own learner', () => {
        seed();
        assert.deepEqual(self(OTHER).unknown, [OTHER]);
        assert.deepEqual(self(OTHER, ID_B), { coverage: 1, known_count: 1, running_words: 1, unknown: [] });
        assert.equal(self(ROSE, ID_B).known_count, 0);
    });

    it('tokens split on whitespace and punctuation and keep maqaf, geresh, and gershayim', () => {
        const keys = (text) => coverage.tokenize(text).map((t) => [t.key, t.hebrew]);
        assert.deepEqual(keys(nfc('בֵּית־סֵפֶר, ג׳ירפה; צה״ל ״שלום״')), [
            [nfc('בֵּית־סֵפֶר'), true],
            [nfc('ג׳ירפה'), true],
            [nfc('צה״ל'), true],
            ['שלום', true]
        ]);
        assert.deepEqual(keys(nfc('בָּרָא׃ אֱלֹהִים׀ אֵת')), [
            [nfc('בָּרָא'), true],
            [nfc('אֱלֹהִים'), true],
            [nfc('אֵת'), true]
        ]);
        assert.deepEqual(keys("ג'ירפה"), [['ג', true], ['ירפה', true]]);
        assert.deepEqual(keys('Hello 123 שלום\u200F.'), [['Hello', false], ['123', false], ['שלום', true]]);
        assert.deepEqual(keys('abcשלום'), [['abc', false], ['שלום', true]]);
        assert.deepEqual(keys(''), []);
        assert.deepEqual(keys(null), []);
    });

    it('input is NFC-normalized before matching, and non-Hebrew tokens are not running words', () => {
        seed();
        const house = nfc('בַּיִת');
        const reordered = '\u05D1\u05BC\u05B7\u05D9\u05B4\u05EA';
        assert.notEqual(reordered, house);
        assert.equal(reordered.normalize('NFC'), house);
        assert.deepEqual(self(reordered), { coverage: 1, known_count: 1, running_words: 1, unknown: [] });
        assert.deepEqual(self('Hello 123, world.'), {
            coverage: null,
            known_count: 0,
            running_words: 0,
            unknown: []
        });
        assert.deepEqual(self(''), { coverage: null, known_count: 0, running_words: 0, unknown: [] });
    });

    it('unknown is deduped in order of first use and stops at 100; repeats still count as running words', () => {
        const letters = 'אבגדהוזחטיכלמנסעפצקרשת';
        const words = [];
        for (const a of letters) {
            for (const b of letters) {
                if (words.length < 150) words.push(a + b + 'ץ');
            }
        }
        const result = self(words.join(' ') + ' ' + words[0] + ' ' + words[1]);
        assert.equal(result.running_words, 152);
        assert.equal(result.known_count, 0);
        assert.equal(result.coverage, 0);
        assert.equal(result.unknown.length, 100);
        assert.deepEqual(result.unknown, words.slice(0, 100));
    });

    it('text must be a string of at most 4000 characters', () => {
        seed();
        const edge = 'א '.repeat(2000);
        assert.equal(edge.length, 4000);
        assert.equal(self(edge).running_words, 2000);
        assert.deepEqual(self(edge + 'א'), { error: 'invalid_text' });
        assert.deepEqual(self(42), { error: 'invalid_text' });
        assert.deepEqual(self(null), { error: 'invalid_text' });
        assert.deepEqual(self(['קָמָה']), { error: 'invalid_text' });
        assert.equal(auditRows().length, 0);

        assert.equal(mcp(edge).running_words, 2000);
        assert.deepEqual(mcp(edge + 'א'), { error: 'invalid_text' });
        const rows = auditRows();
        assert.deepEqual(rows.map((row) => row.detail.outcome), ['ok', 'invalid_text']);
        assert.deepEqual(rows[1].detail, { outcome: 'invalid_text', fields: 'none' });
        assert.equal(rows[1].target, 'learner-a');
    });

    it('switch off: the MCP gets planning_off and nothing else, audited with fields none; self still works', () => {
        seed();
        users.setAiPlanning(ID_A, 0);
        assert.deepEqual(mcp(TWENTY), { error: 'planning_off' });
        assert.deepEqual(mcp(TWENTY, ID_A), { error: 'planning_off' });
        const rows = auditRows();
        assert.equal(rows.length, 2);
        for (const row of rows) {
            assert.equal(row.actor, 'mcp');
            assert.equal(row.target, 'learner-a');
            assert.deepEqual(row.detail, { outcome: 'planning_off', fields: 'none' });
        }
        assert.equal(self(TWENTY).coverage, 0.9);
        assert.equal(auditRows().length, 2);
    });

    it('an ok MCP read writes one audit row with counts and field names, and no text or token', () => {
        seed();
        mcp(TWENTY);
        const rows = auditRows();
        assert.equal(rows.length, 1);
        const row = rows[0];
        assert.equal(row.actor, 'mcp');
        assert.equal(row.target, 'learner-a');
        assert.equal(row.at, NOW_ISO);
        assert.deepEqual(row.detail, {
            outcome: 'ok',
            fields: contract.COVERAGE_FIELDS.slice().sort(),
            running_words: 20,
            unknown_count: 2
        });
        const stored = db.get(
            "SELECT target, detail FROM audit_log WHERE action = 'learner_coverage_read'"
        );
        assert.doesNotMatch(stored.detail + stored.target, HEBREW_LETTER);
        assert.ok(!stored.detail.includes('OK'));
    });

    it('self reads write no audit row and take no MCP read', () => {
        seed();
        for (let i = 0; i < 70; i += 1) self(ROSE);
        assert.equal(auditRows().length, 0);
        for (let i = 0; i < 60; i += 1) assert.equal(mcp(ROSE).known_count, 1);
        assert.deepEqual(mcp(ROSE), { error: 'rate_limited' });
        assert.equal(self(ROSE).known_count, 1);
        assert.throws(() => coverage.checkCoverage('learner-a', ROSE, { audience: 'self' }), TypeError);
        assert.throws(() => coverage.checkCoverage(ID_A, ROSE, {}), TypeError);
    });
});
