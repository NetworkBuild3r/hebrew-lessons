/**
 * Learner model: weak flags, concept mastery, rebuild, and journey stage.
 * Temporary database. Does not open data/hebrew.sqlite.
 * Authored-by-spec: INIT-006/SPEC-006
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const DAY_MS = 24 * 60 * 60 * 1000;
const LEMMA = 'חָשַׁב';
const FORM = 'חָשַׁבְתָּ';
const UNTAGGED_LEMMA = 'הָלַךְ';
const UNTAGGED_FORM = 'הָלַכְתָּ';
const OWN_BINYAN_FORM = 'פִּעֵל';
const FOUR = ['binyan:paal', 'tense:past', 'cell:past.2ms', 'root-class:regular'];

function atLocal(y, m, d, h = 12) {
    return new Date(y, m - 1, d, h, 0, 0, 0).getTime();
}

const NOW = atLocal(2026, 10, 10, 12);
const NOW_ISO = new Date(NOW).toISOString();

function assertClose(actual, expected, label) {
    assert.equal(typeof actual, 'number', label);
    const scale = Math.max(1, Math.abs(expected));
    assert.ok(Math.abs(actual - expected) <= 1e-9 * scale, label + ' expected ' + expected + ' got ' + actual);
}

describe('learner model (INIT-006/SPEC-006)', () => {
    let tmp;
    let savedScheduler;
    let db;
    let users;
    let review;
    let lexicon;
    let mastery;
    let lessons;
    let progress;
    let model;
    let thresholds;
    let Fsrs;
    let userA;
    let userB;
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
        '../server/plans/store',
        '../server/learner/thresholds',
        '../server/learner/model',
        '../server/progress'
    ];

    function dropCache() {
        for (const name of cacheNames) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* not loaded */ }
        }
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-learner-'));
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
        lexicon = require('../server/db/repos/lexicon');
        mastery = require('../server/db/repos/concept-mastery');
        lessons = require('../server/db/repos/lesson-progress');
        thresholds = require('../server/learner/thresholds');
        Fsrs = require('../shared/player/fsrs.js');
        progress = require('../server/progress');
        model = require('../server/learner/model');
        userA = Number(users.create({
            username: 'learner-a',
            password_hash: 'h',
            display_name: 'A'
        }).lastInsertRowid);
        userB = Number(users.create({
            username: 'learner-b',
            password_hash: 'h',
            display_name: 'B'
        }).lastInsertRowid);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        if (savedScheduler === undefined) delete process.env.SRS_SCHEDULER;
        else process.env.SRS_SCHEDULER = savedScheduler;
        delete process.env.DATA_DIR;
        dropCache();
    });

    beforeEach(() => {
        process.env.SRS_SCHEDULER = 'ladder';
        db.run('DELETE FROM concept_mastery');
        db.run('DELETE FROM practice_events');
        db.run('DELETE FROM review_items');
        db.run('DELETE FROM lesson_progress');
        db.run('DELETE FROM lexicon_entries');
        db.run('DELETE FROM plans');
    });

    function lex(overrides) {
        lexicon.upsert(Object.assign({
            key: LEMMA,
            kind: 'lemma',
            lemma_key: null,
            gloss_en: 'he thought',
            pos: 'verb',
            root: 'ח.ש.ב',
            binyan: 'paal',
            root_class: 'regular',
            tense: null,
            cell: null,
            translit: null,
            accept_json: '[]',
            verified: 0,
            source: 'admin',
            rev: 1,
            updated_at: NOW_ISO
        }, overrides));
    }

    function seedTagged() {
        lex({});
        lex({
            key: FORM,
            kind: 'form',
            lemma_key: LEMMA,
            gloss_en: null,
            root: null,
            binyan: null,
            root_class: null,
            tense: 'past',
            cell: '2ms'
        });
    }

    function card(overrides) {
        review.upsert(Object.assign({
            user_id: userA,
            word: 'מִלָּה',
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

    function wordOf(states, word) {
        return states.find((entry) => entry.word === word);
    }

    function signal(partial) {
        seq += 1;
        return Object.assign({
            id: 'lm-' + seq,
            word: FORM,
            kind: 'form',
            lemma: LEMMA,
            result: 'miss',
            at: new Date(NOW + seq * 1000).toISOString()
        }, partial);
    }

    function apply(list, userId) {
        return progress.applySignals(userId == null ? userA : userId, list, NOW);
    }

    function touched(userId) {
        return model.conceptStates(userId == null ? userA : userId)
            .filter((row) => row.attempts > 0)
            .map((row) => row.key);
    }

    it('ac-1 W1: long-term with 4 lapses is weak', () => {
        card({ word: 'א', step: 3, lapses: 4, last_result: 'miss', last_grade: 1 });
        const entry = wordOf(model.wordStates(userA, NOW), 'א');
        assert.equal(entry.he_en.stage, 'longTerm');
        assert.equal(entry.weak, 'weak');
    });

    it('ac-1 W2: long-term with 3 lapses is not flagged', () => {
        card({ word: 'ב', step: 3, lapses: 3, last_result: 'pass', last_grade: 3 });
        const entry = wordOf(model.wordStates(userA, NOW), 'ב');
        assert.equal(entry.he_en.stage, 'longTerm');
        assert.equal(entry.weak, null);
    });

    it('ac-1 W3: 3 lapses before long-term is very_weak, and the worse direction wins', () => {
        card({ word: 'ג', step: 1, lapses: 3, last_result: 'miss', last_grade: 1 });
        card({
            word: 'ג',
            direction: 'en_he',
            step: 1,
            lapses: 2,
            last_result: 'pass',
            last_grade: 3
        });
        const entry = wordOf(model.wordStates(userA, NOW), 'ג');
        assert.equal(entry.he_en.stage, 'learning');
        assert.equal(entry.weak, 'very_weak');
    });

    it('ac-1 W4: 2 lapses, or 1 lapse whose last grade is Again, is weak', () => {
        card({ word: 'ד', step: 1, lapses: 2, last_result: 'pass', last_grade: 3 });
        card({ word: 'ה', step: 1, lapses: 1, last_result: 'miss', last_grade: 1 });
        const states = model.wordStates(userA, NOW);
        assert.equal(wordOf(states, 'ד').weak, 'weak');
        assert.equal(wordOf(states, 'ה').weak, 'weak');
    });

    it('ac-1 W5: fsrs difficulty 8 and 3 reps is weak; ladder ignores that row', () => {
        card({
            word: 'ו',
            step: 1,
            lapses: 0,
            last_grade: 3,
            last_result: 'pass',
            stability: 7,
            difficulty: 8,
            reps: 3,
            fsrs_state: 'review'
        });
        card({
            word: 'ז',
            step: 1,
            lapses: 0,
            last_grade: 3,
            last_result: 'pass',
            stability: 7,
            difficulty: 8,
            reps: 2,
            fsrs_state: 'review'
        });
        process.env.SRS_SCHEDULER = 'fsrs';
        try {
            const fsrsStates = model.wordStates(userA, NOW);
            assert.equal(wordOf(fsrsStates, 'ו').he_en.stage, 'learning');
            assert.equal(wordOf(fsrsStates, 'ו').weak, 'weak');
            assert.equal(wordOf(fsrsStates, 'ז').weak, null);
        } finally {
            process.env.SRS_SCHEDULER = 'ladder';
        }
        assert.equal(wordOf(model.wordStates(userA, NOW), 'ו').weak, null);
    });

    it('ac-1 W6: one lapse after Good, and a clean learning card, are not flagged', () => {
        card({ word: 'ח', step: 1, lapses: 1, last_result: 'pass', last_grade: 3 });
        card({ word: 'ט', step: 1, lapses: 0, last_result: 'pass', last_grade: 3 });
        const states = model.wordStates(userA, NOW);
        assert.equal(wordOf(states, 'ח').weak, null);
        assert.equal(wordOf(states, 'ט').weak, null);
    });

    it('strength buckets follow retrievability, including the ladder approximation', () => {
        const base = thresholds.LADDER_RETRIEVABILITY_BASE;
        const cases = [
            { word: 'י', days: 0, bucket: 'solid' },
            { word: 'כ', days: 1, bucket: 'steady' },
            { word: 'ל', days: 4, bucket: 'shaky' },
            { word: 'מ', days: 7, bucket: 'fragile' }
        ];
        for (const item of cases) {
            card({
                word: item.word,
                step: 0,
                streak: 0,
                lapses: 0,
                last_result: 'miss',
                last_grade: 1,
                last_at: new Date(NOW - item.days * DAY_MS).toISOString()
            });
        }
        const states = model.wordStates(userA, NOW);
        for (const item of cases) {
            const dir = wordOf(states, item.word).he_en;
            assertClose(dir.retrievability, Math.pow(base, item.days), item.word);
            assert.equal(dir.strength_bucket, item.bucket, item.word);
        }

        process.env.SRS_SCHEDULER = 'fsrs';
        try {
            card({
                word: 'נ',
                step: 1,
                stability: 10,
                difficulty: 5,
                reps: 2,
                last_at: new Date(NOW - 3 * DAY_MS).toISOString(),
                fsrs_state: 'review'
            });
            const expected = Fsrs.retrievability({
                stability: 10,
                lastAt: new Date(NOW - 3 * DAY_MS).toISOString()
            }, NOW);
            const dir = wordOf(model.wordStates(userA, NOW), 'נ').he_en;
            assertClose(dir.retrievability, expected, 'fsrs');
            let bucket = 'solid';
            if (expected < thresholds.STRENGTH.fragileBelow) bucket = 'fragile';
            else if (expected < thresholds.STRENGTH.shakyBelow) bucket = 'shaky';
            else if (expected < thresholds.STRENGTH.steadyBelow) bucket = 'steady';
            assert.equal(dir.strength_bucket, bucket);
        } finally {
            process.env.SRS_SCHEDULER = 'ladder';
        }
    });

    it('a review row carries sources, suspension, lapses, and due_in_days for that user only', () => {
        card({
            word: 'ס',
            lapses: 2,
            due: new Date(NOW + 2 * DAY_MS).toISOString(),
            sources: JSON.stringify(['cafe-order', 'letters']),
            suspended: 1
        });
        card({
            word: 'ס',
            direction: 'en_he',
            sources: JSON.stringify(['letters', 'extra']),
            suspended: 0
        });
        const entry = wordOf(model.wordStates(userA, NOW), 'ס');
        assert.equal(entry.suspended, true);
        assert.deepEqual(entry.sources, ['cafe-order', 'letters', 'extra']);
        assert.equal(entry.he_en.lapses, 2);
        assert.equal(entry.he_en.due_in_days, 2);
        assert.deepEqual(model.wordStates(userB, NOW), []);
    });

    it('ac-2: a tagged form miss updates four concepts; a word card, not_yet, and an untagged form update none', () => {
        seedTagged();
        lex({
            key: UNTAGGED_LEMMA,
            kind: 'lemma',
            binyan: null,
            root_class: null,
            root: null,
            gloss_en: 'he walked'
        });
        lex({
            key: UNTAGGED_FORM,
            kind: 'form',
            lemma_key: UNTAGGED_LEMMA,
            binyan: null,
            root_class: null,
            root: null,
            gloss_en: null,
            tense: null,
            cell: null
        });

        apply([
            signal({ id: 'word-pass', word: LEMMA, kind: 'word', lemma: null, result: 'pass', direction: 'he_en' }),
            signal({ id: 'word-miss', word: LEMMA, kind: 'word', lemma: null, result: 'miss', direction: 'he_en' }),
            signal({ id: 'word-omit', word: LEMMA, kind: undefined, lemma: null, result: 'pass', direction: 'en_he' }),
            signal({ id: 'not-yet', result: 'not_yet' }),
            signal({ id: 'untagged', word: UNTAGGED_FORM, lemma: UNTAGGED_LEMMA, result: 'miss' }),
            signal({ id: 'missing', word: 'כָּתַב', lemma: LEMMA, result: 'pass' })
        ]);
        assert.deepEqual(touched(), []);

        apply([signal({ id: 'form-miss', result: 'miss' })]);
        const rows = model.conceptStates(userA);
        assert.deepEqual(touched().sort(), FOUR.slice().sort());
        for (const key of FOUR) {
            const row = rows.find((item) => item.key === key);
            assert.equal(row.attempts, 1, key);
            assert.equal(row.ewma, 0, key);
            assert.equal(row.accuracy, 0, key);
            assert.equal(row.status, 'emerging', key);
        }
        assert.equal(rows.filter((row) => row.status === 'unseen').length, rows.length - 4);
        assert.deepEqual(touched(userB), []);
    });

    it('ac-2: a form binyan overrides the lemma, and 1 to 4 attempts stay emerging', () => {
        seedTagged();
        lex({
            key: OWN_BINYAN_FORM,
            kind: 'form',
            lemma_key: LEMMA,
            binyan: 'piel',
            root_class: null,
            root: null,
            gloss_en: null,
            tense: 'past',
            cell: '2ms'
        });
        apply([signal({
            id: 'piel',
            word: OWN_BINYAN_FORM,
            lemma: LEMMA,
            result: 'miss'
        })]);
        assert.deepEqual(touched().sort(), ['binyan:piel', 'tense:past', 'cell:past.2ms', 'root-class:regular'].sort());
        assert.ok(!touched().includes('binyan:paal'));

        apply([
            signal({ id: 'm2', word: FORM, result: 'miss' }),
            signal({ id: 'm3', word: FORM, result: 'miss' }),
            signal({ id: 'm4', word: FORM, result: 'pass' })
        ]);
        const past = model.conceptStates(userA).find((row) => row.key === 'tense:past');
        assert.equal(past.attempts, 4);
        assert.equal(past.status, 'emerging');
        assert.equal(past.accuracy, 0.25);
    });

    it('ac-2: five attempts leave emerging and take a judged status', () => {
        seedTagged();
        const batch = [];
        for (let i = 0; i < 5; i += 1) batch.push(signal({ id: 'five-' + i, result: 'pass' }));
        apply(batch);
        const past = model.conceptStates(userA).find((row) => row.key === 'tense:past');
        assert.equal(past.attempts, 5);
        assert.equal(past.ewma, 1);
        assert.equal(past.status, 'strong');

        const edges = [
            [0.49, 'very_weak'],
            [0.5, 'weak'],
            [0.69, 'weak'],
            [0.7, 'ok'],
            [0.89, 'ok'],
            [0.9, 'strong']
        ];
        for (const [ewma, status] of edges) {
            mastery.upsert({
                user_id: userA,
                concept_key: 'binyan:nifal',
                attempts: 5,
                correct: 3,
                ewma,
                updated_at: NOW_ISO
            });
            const row = model.conceptStates(userA).find((item) => item.key === 'binyan:nifal');
            assert.equal(row.status, status, String(ewma));
        }
        const unseen = model.conceptStates(userA).find((row) => row.key === 'tense:future');
        assert.equal(unseen.attempts, 0);
        assert.equal(unseen.accuracy, null);
        assert.equal(unseen.ewma, null);
        assert.equal(unseen.status, 'unseen');
    });

    it('a repeated signal id does not count twice', () => {
        seedTagged();
        const once = signal({ id: 'same', result: 'pass' });
        apply([once]);
        apply([Object.assign({}, once)]);
        const past = model.conceptStates(userA).find((row) => row.key === 'tense:past');
        assert.equal(past.attempts, 1);
    });

    it('ac-3: rebuildConceptMastery matches the incremental fold, and retags from the lexicon', () => {
        seedTagged();
        apply([
            signal({ id: 'r1', result: 'pass', at: '2026-10-10T00:00:00.000Z' }),
            signal({ id: 'r2', result: 'miss', at: '2026-10-10T00:01:00.000Z' }),
            signal({ id: 'r3', result: 'pass', at: '2026-10-10T00:02:00.000Z' })
        ]);
        const alpha = thresholds.MASTERY.ewmaAlpha;
        const afterTwo = 1 + alpha * (0 - 1);
        const afterThree = afterTwo + alpha * (1 - afterTwo);
        const stored = mastery.find(userA, 'tense:past');
        const past = model.conceptStates(userA).find((row) => row.key === 'tense:past');
        assert.equal(stored.attempts, 3);
        assert.equal(stored.correct, 2);
        assert.equal(stored.ewma, afterThree);
        assert.equal(past.attempts, 3);
        assert.equal(past.accuracy, 2 / 3);

        function snap(userId) {
            return mastery.listByUser(userId).map((row) => ({
                concept_key: row.concept_key,
                attempts: row.attempts,
                correct: row.correct,
                ewma: row.ewma
            }));
        }
        const before = snap(userA);
        model.rebuildConceptMastery(userA);
        assert.deepEqual(snap(userA), before);

        lex({ binyan: null, root_class: null });
        lex({
            key: FORM,
            kind: 'form',
            lemma_key: LEMMA,
            binyan: null,
            root_class: null,
            root: null,
            gloss_en: null,
            tense: null,
            cell: null
        });
        model.rebuildConceptMastery(userA);
        assert.deepEqual(snap(userA), []);

        seedTagged();
        model.rebuildConceptMastery(userA);
        assert.deepEqual(snap(userA), before);

        apply([signal({ id: 'other-user', result: 'miss' })], userB);
        model.rebuildConceptMastery(userA);
        assert.deepEqual(snap(userA), before);
        assert.equal(snap(userB).length, 4);
    });

    function seedLongTerm(n) {
        const insert = db.prepare(
            `INSERT INTO review_items (
                user_id, word, direction, due, step, streak, lapses,
                last_result, last_at, sources
            ) VALUES (?, ?, 'he_en', ?, 3, 1, 0, 'pass', ?, '[]')`
        );
        db.transaction(() => {
            for (let i = 0; i < n; i += 1) {
                insert.run(userA, 'lt-' + i, NOW_ISO, NOW_ISO);
            }
        });
    }

    const boundaries = [
        [49, 'script'],
        [50, 'foundations'],
        [149, 'foundations'],
        [150, 'building'],
        [499, 'building'],
        [500, 'expanding'],
        [1499, 'expanding'],
        [1500, 'independent']
    ];
    for (const [count, stage] of boundaries) {
        it('ac-4: ' + count + ' he_en long-term words is ' + stage, () => {
            seedLongTerm(count);
            if (count === 49) {
                card({ word: 'en-only', direction: 'en_he', step: 3, last_result: 'pass' });
            }
            const trip = model.journey(userA, NOW);
            assert.equal(trip.stage, stage);
            assert.equal(trip.counts.he_en.longTerm, count);
            assert.equal(trip.counts.he_en.new, 0);
            assert.equal(trip.counts.he_en.learning, 0);
            if (count === 49) assert.equal(trip.counts.en_he.longTerm, 1);
        });
    }

    it('journey lists completed lessons, practiced grammar, recipe counts, and the vowels-off share', () => {
        seedTagged();
        apply([signal({ id: 'journey-form', result: 'pass' })]);
        card({ word: 'new-word', step: 0, streak: 0, last_result: 'miss', last_grade: 1, lapses: 1 });
        card({ word: 'learning-word', step: 1, last_result: 'pass' });

        lessons.upsert({
            user_id: userA,
            slug: 'zeta',
            state_json: JSON.stringify({ completedRounds: 0, vowelsOn: true }),
            rev: 1,
            updated_at: NOW_ISO
        });
        lessons.upsert({
            user_id: userA,
            slug: 'cafe-order',
            state_json: JSON.stringify({ completedRounds: 2, vowelsOn: false }),
            rev: 1,
            updated_at: NOW_ISO
        });
        lessons.upsert({
            user_id: userA,
            slug: 'legacy_slug',
            state_json: JSON.stringify({ completedRounds: 1, vowelsOn: false }),
            rev: 1,
            updated_at: NOW_ISO
        });
        lessons.upsert({
            user_id: userB,
            slug: 'other',
            state_json: JSON.stringify({ completedRounds: 4, vowelsOn: false }),
            rev: 1,
            updated_at: NOW_ISO
        });

        const snapshot = JSON.stringify({
            units: [{
                lessons: [
                    { id: 'cafe-order', recipe: 'topic-conversation' },
                    { id: 'plan-lesson', legacyStorageKey: 'legacy_slug', recipe: 'letters-and-vowels' },
                    { id: 'zeta', recipe: 'guide' },
                    { id: 'absent', recipe: 'verb-workshop' }
                ]
            }]
        });
        db.run(
            `INSERT INTO plans (
                id, revision, status, audience, assignee_id, working_copy, snapshot, created_at, updated_at
            ) VALUES ('unit-1', 1, 'published', 'shared', NULL, ?, ?, ?, ?)`,
            '{"id":"unit-1"}',
            snapshot,
            NOW_ISO,
            NOW_ISO
        );
        db.run(
            `INSERT INTO plans (
                id, revision, status, audience, assignee_id, working_copy, snapshot, created_at, updated_at
            ) VALUES ('unit-2', 1, 'published', 'shared', NULL, ?, ?, ?, ?)`,
            '{"id":"unit-2"}',
            snapshot,
            NOW_ISO,
            NOW_ISO
        );
        db.run(
            `INSERT INTO plans (
                id, revision, status, audience, assignee_id, working_copy, snapshot, created_at, updated_at
            ) VALUES ('draft-1', 1, 'draft', 'shared', NULL, ?, NULL, ?, ?)`,
            JSON.stringify({
                units: [{ lessons: [{ id: 'cafe-order', recipe: 'guide' }] }]
            }),
            NOW_ISO,
            NOW_ISO
        );

        const trip = model.journey(userA, NOW);
        assert.deepEqual(trip.lessons_completed, ['cafe-order', 'legacy_slug']);
        assert.deepEqual(trip.tenses_practiced, ['past']);
        assert.deepEqual(trip.binyanim_practiced, ['paal']);
        assert.deepEqual(trip.recipes_completed, {
            'topic-conversation': 1,
            'letters-and-vowels': 1
        });
        assert.equal(trip.vowels_off_share, 0.65);
        assert.equal(trip.counts.he_en.new, 1);
        assert.equal(trip.counts.he_en.learning, 1);
        assert.deepEqual(model.journey(userB, NOW).lessons_completed, ['other']);
        assert.equal(model.journey(userB, NOW).vowels_off_share, 1);
        assert.deepEqual(model.journey(userB, NOW).recipes_completed, {});

        db.run('DELETE FROM lesson_progress');
        db.run('DELETE FROM plans');
        const bare = model.journey(userA, NOW);
        assert.equal(bare.vowels_off_share, null);
        assert.deepEqual(bare.lessons_completed, []);
        assert.deepEqual(bare.recipes_completed, {});
    });
});
