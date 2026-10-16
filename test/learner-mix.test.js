/**
 * Daily mix: due review, weak words, one drill, one reading.
 * Temporary database and a temporary lessons folder. Does not open data/hebrew.sqlite.
 * Authored-by-spec: INIT-006/SPEC-008
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const DAY_MS = 24 * 60 * 60 * 1000;
const ALEF = [...'אבגדהוזחטיכלמנסעפצקרשת'];
const LEMMA_A = 'חשב';
const LEMMA_B = 'כתב';
const LEMMA_OUT = 'הלך';

function atLocal(y, m, d, h = 12) {
    return new Date(y, m - 1, d, h, 0, 0, 0).getTime();
}

const NOW = atLocal(2026, 10, 10, 12);
const NOW_ISO = new Date(NOW).toISOString();

function heb(n) {
    return 'כ' + ALEF[Math.floor(n / ALEF.length)] + ALEF[n % ALEF.length];
}

function weakHeb(n) {
    return 'ש' + ALEF[Math.floor(n / ALEF.length)] + ALEF[n % ALEF.length];
}

describe('learner mix (INIT-006/SPEC-008)', () => {
    let tmp;
    let lessons;
    let savedScheduler;
    let db;
    let users;
    let review;
    let lexicon;
    let mastery;
    let progressLessons;
    let userWords;
    let mix;
    let Review;
    let userA;
    let userB;

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
        '../server/db/repos/user-words',
        '../server/plans/store',
        '../server/learner/thresholds',
        '../server/learner/model',
        '../server/learner/mix',
        '../server/vocab-index',
        '../server/progress'
    ];

    function dropCache() {
        for (const name of cacheNames) {
            try { delete require.cache[require.resolve(name)]; } catch (_) { /* not loaded */ }
        }
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-mix-'));
        lessons = path.join(tmp, 'lessons');
        fs.mkdirSync(lessons, { recursive: true });
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
        progressLessons = require('../server/db/repos/lesson-progress');
        userWords = require('../server/db/repos/user-words');
        Review = require('../shared/player/review.js');
        mix = require('../server/learner/mix');
        userA = Number(users.create({
            username: 'mix-a',
            password_hash: 'h',
            display_name: 'A'
        }).lastInsertRowid);
        userB = Number(users.create({
            username: 'mix-b',
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
        db.run('DELETE FROM user_words');
        db.run('DELETE FROM plans');
        fs.rmSync(lessons, { recursive: true, force: true });
        fs.mkdirSync(lessons, { recursive: true });
    });

    function opts() {
        return { lessonsDir: lessons };
    }

    function compose(userId, now) {
        return mix.compose(userId == null ? userA : userId, now == null ? NOW : now, opts());
    }

    function section(result, id) {
        return result.sections.find((entry) => entry.id === id);
    }

    function writeLesson(slug, items) {
        const dir = path.join(lessons, slug);
        fs.mkdirSync(dir, { recursive: true });
        const body = 'window.WORKSHEET = ' + JSON.stringify({ vocab: { items } }) + ';\n';
        fs.writeFileSync(path.join(dir, 'worksheet.js'), body, 'utf8');
    }

    function see(slug) {
        progressLessons.upsert({
            user_id: userA,
            slug,
            state_json: JSON.stringify({ completedRounds: 1 }),
            rev: 1,
            updated_at: NOW_ISO
        });
    }

    function card(word, index, overrides) {
        review.upsert(Object.assign({
            user_id: userA,
            word,
            direction: 'he_en',
            due: new Date(NOW - (100 - index) * DAY_MS).toISOString(),
            step: 2,
            streak: 1,
            lapses: 0,
            last_result: 'pass',
            last_at: new Date(NOW - 10 * DAY_MS).toISOString(),
            sources: JSON.stringify(['seen-lesson']),
            last_grade: 3,
            suspended: 0
        }, overrides));
    }

    function lex(overrides) {
        lexicon.upsert(Object.assign({
            key: LEMMA_A,
            kind: 'lemma',
            lemma_key: null,
            gloss_en: 'he thought',
            pos: 'verb',
            root: null,
            binyan: 'paal',
            root_class: 'regular',
            tense: null,
            cell: null,
            translit: null,
            accept_json: '[]',
            verified: 1,
            source: 'admin',
            rev: 1,
            updated_at: NOW_ISO
        }, overrides));
    }

    function form(key, lemma, extra) {
        lex(Object.assign({
            key,
            kind: 'form',
            lemma_key: lemma,
            gloss_en: null,
            tense: 'past',
            cell: '2ms',
            verified: 1,
            accept_json: '[]'
        }, extra || {}));
    }

    function libraryWord(word, gloss) {
        userWords.add({
            user_id: userA,
            word,
            source: 'self',
            added_at: NOW_ISO,
            gloss_en: gloss || null
        });
    }

    function seedDrill() {
        lex({ key: LEMMA_A, gloss_en: 'he thought' });
        lex({ key: LEMMA_B, gloss_en: 'he wrote', root_class: 'regular' });
        lex({ key: LEMMA_OUT, gloss_en: 'he walked' });
        form('חשבתי', LEMMA_A, { cell: '1s' });
        form('חשבת', LEMMA_A, { cell: '2ms', accept_json: '["חשבתא"]' });
        form('חשבה', LEMMA_A, { cell: '3fs' });
        form('חשבנו', LEMMA_A, { cell: '1p', verified: 0 });
        form('כתבתי', LEMMA_B, { cell: '1s' });
        form('כתבת', LEMMA_B, { cell: '2ms' });
        form('כתבה', LEMMA_B, { cell: '3fs' });
        form('הלכתי', LEMMA_OUT, { cell: '1s' });
        form('חושב', LEMMA_A, { tense: 'present', cell: 'ms' });
        libraryWord(LEMMA_A, 'he thought');
        libraryWord(LEMMA_B, 'he wrote');
        mastery.upsert({
            user_id: userA,
            concept_key: 'tense:future',
            attempts: 5,
            correct: 0,
            ewma: 0.05,
            updated_at: NOW_ISO
        });
        mastery.upsert({
            user_id: userA,
            concept_key: 'tense:present',
            attempts: 4,
            correct: 0,
            ewma: 0.1,
            updated_at: NOW_ISO
        });
        mastery.upsert({
            user_id: userA,
            concept_key: 'tense:past',
            attempts: 5,
            correct: 2,
            ewma: 0.4,
            updated_at: NOW_ISO
        });
    }

    function weakCard(word, flag) {
        const very = flag === 'very_weak';
        card(word, 0, {
            due: new Date(NOW + 5 * DAY_MS).toISOString(),
            step: very ? 0 : 1,
            streak: 0,
            lapses: very ? 3 : 2,
            last_result: very ? 'miss' : 'pass',
            last_at: new Date(NOW - 2 * DAY_MS).toISOString(),
            last_grade: very ? 1 : 3,
            suspended: 0
        });
    }

    function librarySet() {
        const keys = new Set();
        for (const row of review.listByUser(userA)) keys.add(row.word);
        for (const row of userWords.listByUser(userA)) keys.add(row.word);
        return keys;
    }

    function unknownShare(sentence, known) {
        const tokens = String(sentence).split(/\s+/).filter((token) => /[\u05D0-\u05EA]/.test(token));
        if (!tokens.length) return 0;
        let unknown = 0;
        for (const token of tokens) {
            if (!known.has(token)) unknown += 1;
        }
        return unknown / tokens.length;
    }

    it('imports caps and does not draw an unseeded number', () => {
        const src = fs.readFileSync(path.join(root, 'server', 'learner', 'mix.js'), 'utf8');
        assert.equal(src.includes('Math.random'), false);
        assert.match(src, /Review\.DEFAULT_CAPS/);
        assert.equal(/\bdue\s*:\s*30\b/.test(src), false);
        assert.equal(/\bnew\s*:\s*10\b/.test(src), false);
        assert.match(src, /checkCoverage/);
        assert.match(src, /audience:\s*'self'/);
        assert.equal(Review.DEFAULT_CAPS.due, 30);
        assert.equal(Review.DEFAULT_CAPS.new, 10);
    });

    it('ac-1: 40 due cards return the due cap, and every item stays in this library', () => {
        see('seen-lesson');
        for (let i = 0; i < 40; i += 1) {
            if (i === 0) {
                card(heb(0), 0, {
                    step: 0,
                    streak: 0,
                    lapses: 3,
                    last_result: 'miss',
                    last_grade: 1
                });
            } else {
                card(heb(i), i);
            }
        }
        card('סגור', 0, {
            due: new Date(NOW - 400 * DAY_MS).toISOString(),
            suspended: 1,
            step: 0,
            lapses: 3,
            last_result: 'miss',
            last_grade: 1
        });
        card('סוד', 0, {
            user_id: userB,
            due: new Date(NOW - 500 * DAY_MS).toISOString(),
            step: 2,
            last_result: 'pass',
            last_grade: 3
        });
        review.upsert({
            user_id: userB,
            word: 'זר',
            direction: 'he_en',
            due: new Date(NOW + 5 * DAY_MS).toISOString(),
            step: 0,
            streak: 0,
            lapses: 3,
            last_result: 'miss',
            last_at: new Date(NOW - 2 * DAY_MS).toISOString(),
            sources: '[]',
            last_grade: 1,
            suspended: 0
        });
        weakCard(weakHeb(0), 'very_weak');
        weakCard(weakHeb(1), 'very_weak');
        weakCard(weakHeb(2), 'very_weak');
        weakCard(weakHeb(3), 'weak');
        weakCard(weakHeb(4), 'weak');
        seedDrill();
        const sentence = [heb(1), heb(1), heb(1)].join(' ');
        writeLesson('seen-lesson', [{
            word: heb(1),
            meaning: 'one',
            sentence,
            sentenceAudio: 'clip.mp3'
        }]);

        const result = compose();
        const reviewSection = section(result, 'review');
        const weakSection = section(result, 'weak');
        const drill = section(result, 'drill');
        const reading = section(result, 'reading');
        const known = librarySet();

        assert.equal(reviewSection.items.length, Review.DEFAULT_CAPS.due);
        assert.equal(reviewSection.items.some((item) => item.key === heb(0)), true);
        assert.equal(reviewSection.items.some((item) => item.key === heb(29)), true);
        assert.equal(reviewSection.items.some((item) => item.key === heb(30)), false);
        assert.equal(reviewSection.items.some((item) => item.key === 'סגור'), false);
        assert.equal(reviewSection.items.some((item) => item.key === 'סוד'), false);

        const reviewKeys = new Set(reviewSection.items.map((item) => item.key));
        assert.equal(reviewKeys.has(heb(0)), true);
        for (const item of reviewSection.items) {
            assert.equal(known.has(item.key), true);
            assert.equal(item.user_id, undefined);
        }
        for (const item of weakSection.items) {
            assert.equal(known.has(item.key), true);
            assert.equal(reviewKeys.has(item.key), false);
            assert.equal(item.key === 'זר', false);
            assert.equal(item.key === 'סוד', false);
        }
        assert.equal(weakSection.items.some((item) => item.key === heb(0)), false);
        const weakKeys = [...new Set(weakSection.items.map((item) => item.key))];
        assert.deepEqual(weakKeys, [weakHeb(0), weakHeb(1), weakHeb(2), weakHeb(3), weakHeb(4)]);
        for (const key of weakKeys) {
            const dirs = weakSection.items.filter((item) => item.key === key).map((item) => item.direction);
            assert.deepEqual(dirs, ['he_en', 'en_he']);
        }
        assert.equal(weakSection.items[0].weak, 'very_weak');
        assert.equal(weakSection.items[weakSection.items.length - 1].weak, 'weak');

        assert.ok(drill);
        assert.equal(drill.concept, 'tense:past');
        for (const item of drill.items) {
            assert.equal(known.has(item.lemma), true);
            assert.equal(item.lemma === LEMMA_OUT, false);
        }
        assert.ok(reading);
        for (const item of reading.items) {
            assert.ok(unknownShare(item.sentence, known) <= 0.05);
            assert.equal(item.sentence.includes('סוד'), false);
        }
        assert.equal(reading.items[0].audio, 'clip.mp3');
        assert.deepEqual(result.sections.map((entry) => entry.id), ['review', 'weak', 'drill', 'reading']);
    });

    it('keeps new cards inside the new cap and out of an unseen lesson', () => {
        see('seen-lesson');
        card('מלה', 0, {
            due: new Date(NOW - 100 * DAY_MS).toISOString(),
            step: 0,
            streak: 0,
            lapses: 0,
            last_result: null,
            last_at: null,
            last_grade: null,
            sources: '[]'
        });
        for (let i = 0; i < 15; i += 1) {
            card(heb(i), i, {
                due: new Date(NOW - (90 - i) * DAY_MS).toISOString(),
                step: 0,
                streak: 0,
                lapses: 0,
                last_result: null,
                last_at: null,
                last_grade: null,
                sources: JSON.stringify(['seen-lesson'])
            });
        }
        for (let i = 0; i < 5; i += 1) {
            card('נ' + ALEF[i], i, {
                due: new Date(NOW - 80 * DAY_MS).toISOString(),
                step: 0,
                streak: 0,
                lapses: 0,
                last_result: null,
                last_at: null,
                last_grade: null,
                sources: JSON.stringify(['unseen-lesson'])
            });
        }
        card('עבר', 0, {
            due: new Date(NOW - DAY_MS).toISOString(),
            step: 2,
            last_result: 'pass',
            last_grade: 3,
            sources: JSON.stringify(['unseen-lesson'])
        });

        const items = section(compose(), 'review').items;
        const fresh = items.filter((item) => item.isNew);
        assert.equal(fresh.length, Review.DEFAULT_CAPS.new);
        assert.equal(items.some((item) => item.key === 'עבר'), true);
        assert.equal(items.some((item) => item.key === 'מלה'), true);
        for (let i = 0; i < 5; i += 1) {
            assert.equal(items.some((item) => item.key === 'נ' + ALEF[i]), false);
        }
        assert.equal(items.length, Review.DEFAULT_CAPS.new + 1);
    });

    it('stops weak words at eight', () => {
        see('seen-lesson');
        for (let i = 0; i < 10; i += 1) weakCard(weakHeb(i), 'very_weak');
        const items = section(compose(), 'weak').items;
        const keys = [...new Set(items.map((item) => item.key))];
        assert.equal(keys.length, 8);
        assert.deepEqual(keys, weakHebKeys(8));
        assert.equal(items.length, 16);
    });

    function weakHebKeys(n) {
        const keys = [];
        for (let i = 0; i < n; i += 1) keys.push(weakHeb(i));
        return keys;
    }

    it('ac-2: drill uses only verified forms and two lemmas when two qualify', () => {
        seedDrill();
        const drill = section(compose(), 'drill');
        assert.equal(drill.concept, 'tense:past');
        assert.equal(drill.items.length, 6);
        const lemmas = new Set(drill.items.map((item) => item.lemma));
        assert.deepEqual([...lemmas].sort(), [LEMMA_A, LEMMA_B].sort());
        for (let i = 1; i < drill.items.length; i += 1) {
            assert.notEqual(drill.items[i].lemma, drill.items[i - 1].lemma);
        }
        for (const item of drill.items) {
            const row = lexicon.find(item.key);
            assert.equal(row.kind, 'form');
            assert.equal(Number(row.verified), 1);
            assert.equal(item.lemma === LEMMA_OUT, false);
            assert.equal(item.key === 'חשבנו', false);
            assert.equal(item.key === 'הלכתי', false);
            assert.equal(item.key === 'חושב', false);
            assert.ok(item.accept.includes(item.key));
            assert.equal(item.direction, 'en_he');
        }
        const spelled = drill.items.find((item) => item.key === 'חשבת');
        assert.ok(spelled);
        assert.ok(spelled.accept.includes('חשבתא'));
        assert.match(spelled.prompt, /אתה/);

        db.run('DELETE FROM lexicon_entries');
        db.run('DELETE FROM user_words');
        const omitted = section(compose(), 'drill');
        assert.equal(omitted, undefined);
    });

    it('ac-3: reading requires coverage of 0.95 and is omitted when none qualify', () => {
        const known = 'שלום';
        const unknownA = 'תפוח';
        const unknownB = 'ספר';
        libraryWord(known, 'peace');
        const boundary = Array(19).fill(known).concat(unknownA).join(' ');
        const tooLow = Array(18).fill(known).concat([unknownA, unknownB]).join(' ');
        writeLesson('read', [
            { word: known, meaning: 'peace', sentence: boundary },
            { word: unknownA, meaning: 'apple', sentence: tooLow }
        ]);

        const reading = section(compose(), 'reading');
        assert.ok(reading);
        assert.equal(reading.items.length, 1);
        assert.equal(reading.items[0].sentence, boundary);
        assert.ok(reading.items[0].coverage + 1e-9 >= 0.95);
        assert.ok(unknownShare(reading.items[0].sentence, librarySet()) <= 0.05);
        assert.equal(reading.items.some((item) => item.sentence === tooLow), false);

        fs.rmSync(lessons, { recursive: true, force: true });
        fs.mkdirSync(lessons, { recursive: true });
        writeLesson('read', [
            { word: unknownA, meaning: 'apple', sentence: tooLow }
        ]);
        assert.equal(section(compose(), 'reading'), undefined);
        assert.deepEqual(compose().sections.map((entry) => entry.id), ['review', 'weak']);
    });

    it('ac-4: the same now, and the same local date, return the same mix', () => {
        see('seen-lesson');
        card(heb(0), 1);
        card(heb(1), 2);
        weakCard(weakHeb(0), 'very_weak');
        seedDrill();
        const words = ['אחת', 'שתים', 'שלוש', 'ארבע'];
        for (const word of words) libraryWord(word, word);
        writeLesson('read', words.map((word) => ({
            word,
            meaning: word,
            sentence: word + ' ' + word
        })));

        const morning = atLocal(2026, 10, 10, 8);
        const evening = atLocal(2026, 10, 10, 20);
        const first = compose(userA, evening);
        const second = compose(userA, evening);
        const early = compose(userA, morning);
        assert.deepEqual(second, first);
        assert.deepEqual(early, first);
        const reading = section(first, 'reading');
        assert.equal(reading.items.length, 3);
        assert.ok(section(first, 'drill'));
    });

    it('returns an empty mix for an id that is not a learner', () => {
        card(heb(0), 1);
        const result = mix.compose(0, NOW, opts());
        assert.deepEqual(result.sections.map((entry) => entry.id), ['review', 'weak']);
        assert.equal(result.sections[0].items.length, 0);
        assert.equal(mix.compose(userB, NOW, opts()).sections[0].items.length, 0);
    });
});
