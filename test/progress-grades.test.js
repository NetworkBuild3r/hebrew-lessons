/**
 * applySignals grades, form signals, and the SRS_SCHEDULER switch, on a temporary database.
 * Each scheduler is loaded explicitly, so this file passes under any ambient SRS_SCHEDULER.
 * Authored-by-spec: INIT-006/SPEC-004
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const DAY_MS = 24 * 60 * 60 * 1000;
const PROGRESS = '../server/progress';

function atLocal(y, m, d, h = 12) {
    return new Date(y, m - 1, d, h, 0, 0, 0).getTime();
}

function assertClose(actual, expected, label) {
    assert.equal(typeof actual, 'number', label + ': not a number');
    assert.ok(Math.abs(actual - expected) <= 1e-12 * Math.max(1, Math.abs(expected)),
        label + ': expected ' + expected + ', got ' + actual);
}

/** Load server/progress.js with SRS_SCHEDULER set to `mode` (undefined = unset). */
function loadProgress(mode) {
    const previous = process.env.SRS_SCHEDULER;
    if (mode === undefined) delete process.env.SRS_SCHEDULER;
    else process.env.SRS_SCHEDULER = mode;
    delete require.cache[require.resolve(PROGRESS)];
    try {
        return require(PROGRESS);
    } finally {
        if (previous === undefined) delete process.env.SRS_SCHEDULER;
        else process.env.SRS_SCHEDULER = previous;
        delete require.cache[require.resolve(PROGRESS)];
    }
}

describe('progress grades and SRS_SCHEDULER (INIT-006/SPEC-004)', () => {
    let tmp;
    let db;
    let review;
    let events;
    let Fsrs;
    let HebrewReview;
    let fsrs;
    let ladder;
    let userA;
    let counter = 0;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-grades-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        for (const name of ['users', 'review', 'events', 'lesson-progress']) {
            delete require.cache[require.resolve('../server/db/repos/' + name)];
        }

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        const users = require('../server/db/repos/users');
        review = require('../server/db/repos/review');
        events = require('../server/db/repos/events');
        Fsrs = require('../shared/player/fsrs.js');
        HebrewReview = require('../shared/player/review.js');
        fsrs = loadProgress('fsrs');
        ladder = loadProgress('ladder');

        userA = Number(users.create({
            username: 'grades-a',
            password_hash: 'h',
            display_name: 'A'
        }).lastInsertRowid);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    beforeEach(() => {
        db.run('DELETE FROM practice_events');
        db.run('DELETE FROM review_items');
    });

    function signal(partial) {
        counter += 1;
        return Object.assign({
            id: 'grade-sig-' + counter,
            word: 'קָמָה',
            direction: 'he_en',
            result: 'pass',
            slug: 'my-morning',
            activity: 'words'
        }, partial);
    }

    function rowOf(word, direction) {
        return review.find(userA, word, direction || 'he_en');
    }

    function cardFromRow(row) {
        return {
            stability: row.stability,
            difficulty: row.difficulty,
            reps: row.reps,
            state: row.fsrs_state,
            due: row.due,
            lastGrade: row.last_grade,
            lapses: row.lapses,
            lastAt: row.last_at
        };
    }

    function reviewRowCount() {
        return db.get('SELECT COUNT(*) AS n FROM review_items WHERE user_id = ?', userA).n;
    }

    describe('SRS_SCHEDULER', () => {
        it('ac-5: unset is ladder; fsrs and ladder are accepted; anything else is ladder with a warning', () => {
            assert.equal(loadProgress(undefined).SCHEDULER, 'ladder');
            assert.equal(loadProgress('').SCHEDULER, 'ladder');
            assert.equal(loadProgress('ladder').SCHEDULER, 'ladder');
            assert.equal(loadProgress('fsrs').SCHEDULER, 'fsrs');
            assert.equal(loadProgress(' FSRS ').SCHEDULER, 'fsrs');
            const warnings = [];
            const realWarn = console.warn;
            console.warn = (message) => { warnings.push(String(message)); };
            try {
                assert.equal(loadProgress('sm2').SCHEDULER, 'ladder');
            } finally {
                console.warn = realWarn;
            }
            assert.equal(warnings.length, 1);
            assert.match(warnings[0], /SRS_SCHEDULER "sm2" is not fsrs or ladder; using ladder/);
        });

        it('is read once when the module loads; changing the variable later switches nothing', () => {
            const day0 = atLocal(2026, 10, 1, 10);
            const previous = process.env.SRS_SCHEDULER;
            process.env.SRS_SCHEDULER = 'ladder';
            try {
                fsrs.applySignals(userA, [signal({ word: 'א' })], day0);
            } finally {
                if (previous === undefined) delete process.env.SRS_SCHEDULER;
                else process.env.SRS_SCHEDULER = previous;
            }
            assert.equal(rowOf('א').stability, Fsrs.PARAMS.w[2]);
            assert.equal(rowOf('א').fsrs_state, 'review');
        });
    });

    describe('grade validation (both schedulers)', () => {
        for (const mode of ['fsrs', 'ladder']) {
            it('rejects a grade that is not an integer 1–4 or does not fit the result, and counts it — ' + mode, () => {
                const progress = mode === 'fsrs' ? fsrs : ladder;
                const bad = [
                    signal({ id: 'bad-0', grade: 0 }),
                    signal({ id: 'bad-5', grade: 5 }),
                    signal({ id: 'bad-frac', grade: 2.5 }),
                    signal({ id: 'bad-string', grade: '3' }),
                    signal({ id: 'bad-bool', grade: true }),
                    signal({ id: 'bad-pass-again', result: 'pass', grade: 1 }),
                    signal({ id: 'bad-miss-good', result: 'miss', grade: 3 }),
                    signal({ id: 'bad-notyet-hard', result: 'not_yet', grade: 2 }),
                    signal({ id: 'bad-kind', kind: 'sentence' }),
                    signal({ id: 'bad-kind-case', kind: 'Form', lemma: 'לִכְתֹּב' }),
                    signal({ id: 'bad-kind-array', kind: ['form'], lemma: 'לִכְתֹּב' }),
                    signal({ id: 'bad-form-no-lemma', kind: 'form', word: 'כָּתַבְתָּ' }),
                    signal({ id: 'bad-form-lemma-object', kind: 'form', word: 'כָּתַבְתָּ', lemma: {} }),
                    signal({ id: 'bad-word-lemma-number', lemma: 123 })
                ];
                const good = signal({ id: 'good-easy', word: 'ב', grade: 4 });
                const result = progress.applySignals(userA, bad.concat([good]), atLocal(2026, 10, 2, 10));
                assert.equal(result.rejected, bad.length);
                assert.equal(result.applied, 1);
                for (const s of bad) assert.equal(events.find(userA, s.id), undefined, s.id);
                assert.equal(events.find(userA, 'good-easy').grade, 4);
                assert.equal(rowOf('קָמָה'), undefined);
                assert.equal(reviewRowCount(), 1);
            });
        }
    });

    describe('fsrs (SRS_SCHEDULER=fsrs)', () => {
        it('ac-2: a pass with no grade schedules as Good', () => {
            const day0 = atLocal(2026, 10, 3, 10);
            fsrs.applySignals(userA, [signal({ id: 'p1' })], day0);
            const first = rowOf('קָמָה');
            const expected1 = Fsrs.nextCard(Fsrs.initCard(day0), 3, day0);
            assertClose(first.stability, expected1.stability, 'stability');
            assertClose(first.difficulty, expected1.difficulty, 'difficulty');
            assert.equal(first.due, expected1.due);
            assert.equal(first.last_grade, 3);
            assert.equal(first.reps, 1);
            assert.equal(first.fsrs_state, 'review');
            assert.equal(events.find(userA, 'p1').grade, 3);
            assert.equal(events.find(userA, 'p1').kind, 'word');

            const day2 = day0 + 2 * DAY_MS;
            fsrs.applySignals(userA, [signal({ id: 'p2' })], day2);
            const second = rowOf('קָמָה');
            const expected2 = Fsrs.nextCard(cardFromRow(first), 3, day2);
            assertClose(second.stability, expected2.stability, 'stability after the second Good');
            assertClose(second.difficulty, expected2.difficulty, 'difficulty after the second Good');
            assert.equal(second.due, expected2.due);
            assert.equal(second.reps, 2);
        });

        it('ac-2: a miss with no grade schedules as Again and adds one lapse', () => {
            const day0 = atLocal(2026, 10, 4, 10);
            fsrs.applySignals(userA, [signal({ id: 'm-seed' })], day0);
            const seeded = rowOf('קָמָה');
            assert.equal(seeded.lapses, 0);

            const day3 = day0 + 3 * DAY_MS;
            fsrs.applySignals(userA, [signal({ id: 'm1', result: 'miss' })], day3);
            const missed = rowOf('קָמָה');
            const expected = Fsrs.nextCard(cardFromRow(seeded), 1, day3);
            assertClose(missed.stability, expected.stability, 'stability');
            assertClose(missed.difficulty, expected.difficulty, 'difficulty');
            assert.equal(missed.lapses, 1);
            assert.equal(missed.last_grade, 1);
            assert.equal(missed.last_result, 'miss');
            assert.equal(missed.fsrs_state, 'relearning');
            assert.equal(Date.parse(missed.due), day3 + DAY_MS);
            assert.equal(events.find(userA, 'm1').grade, 1);
        });

        it('ac-2: not_yet leaves due, stability, and difficulty unchanged', () => {
            const day0 = atLocal(2026, 10, 5, 10);
            fsrs.applySignals(userA, [signal({ id: 'ny-seed' })], day0);
            const seeded = rowOf('קָמָה');
            fsrs.applySignals(userA, [signal({ id: 'ny1', result: 'not_yet' })], day0 + DAY_MS);
            const after = rowOf('קָמָה');
            assert.equal(after.due, seeded.due);
            assert.equal(after.stability, seeded.stability);
            assert.equal(after.difficulty, seeded.difficulty);
            assert.equal(after.reps, seeded.reps);
            assert.equal(after.lapses, seeded.lapses);
            assert.equal(after.last_grade, seeded.last_grade);
            assert.equal(after.last_at, seeded.last_at);
            assert.equal(after.last_result, 'not_yet');
            assert.equal(events.find(userA, 'ny1').grade, null);

            fsrs.applySignals(userA, [signal({ id: 'ny-fresh', word: 'ג', result: 'not_yet' })], day0);
            const fresh = rowOf('ג');
            assert.equal(Date.parse(fresh.due), day0);
            assert.equal(fresh.stability, null);
            assert.equal(fresh.difficulty, null);
            assert.equal(fresh.lapses, 0);
            assert.equal(fresh.fsrs_state, 'new');
            assert.equal(fresh.last_result, 'not_yet');
        });

        it('ac-3: two pass signals on the same local day advance once', () => {
            const morning = atLocal(2026, 10, 6, 9);
            const evening = atLocal(2026, 10, 6, 18);
            const result = fsrs.applySignals(userA, [signal({ id: 's1' })], morning);
            assert.equal(result.applied, 1);
            const once = rowOf('קָמָה');
            fsrs.applySignals(userA, [signal({ id: 's2' })], evening);
            const twice = rowOf('קָמָה');
            assert.equal(twice.due, once.due);
            assert.equal(twice.stability, once.stability);
            assert.equal(twice.difficulty, once.difficulty);
            assert.equal(twice.reps, 1);
            assert.equal(twice.last_result, 'pass');
            assert.equal(Date.parse(twice.last_at), evening);
            assert.ok(events.find(userA, 's1') && events.find(userA, 's2'));

            const nextDay = atLocal(2026, 10, 7, 9);
            fsrs.applySignals(userA, [signal({ id: 's3' })], nextDay);
            const third = rowOf('קָמָה');
            assert.equal(third.reps, 2);
            assert.notEqual(third.due, once.due);
        });

        it('an explicit grade schedules with that grade; a flashcard grade later the same day uses the same-day formula', () => {
            const morning = atLocal(2026, 10, 8, 9);
            const noon = atLocal(2026, 10, 8, 12);
            fsrs.applySignals(userA, [signal({ id: 'e1', grade: 4 })], morning);
            const easy = rowOf('קָמָה');
            assert.equal(easy.stability, Fsrs.PARAMS.w[3]);
            assert.equal(easy.last_grade, 4);
            fsrs.applySignals(userA, [signal({ id: 'e2', grade: 2 })], noon);
            const hard = rowOf('קָמָה');
            const expected = Fsrs.nextCard(cardFromRow(easy), 2, noon);
            assertClose(hard.stability, expected.stability, 'same-day Hard stability');
            assertClose(hard.difficulty, expected.difficulty, 'same-day Hard difficulty');
            assert.equal(hard.due, expected.due);
            assert.equal(hard.reps, 2);
            assert.equal(hard.last_grade, 2);
        });

        it('a suspended row is not scheduled; the event is still logged', () => {
            const day0 = atLocal(2026, 10, 9, 10);
            fsrs.applySignals(userA, [signal({ id: 'sus-seed' })], day0);
            review.setSuspended(userA, 'קָמָה', true);
            const before = rowOf('קָמָה');
            const result = fsrs.applySignals(userA, [signal({ id: 'sus-miss', result: 'miss' })], day0 + 2 * DAY_MS);
            assert.equal(result.applied, 1);
            assert.deepEqual(rowOf('קָמָה'), before);
            assert.equal(events.find(userA, 'sus-miss').result, 'miss');
            const ladderResult = ladder.applySignals(userA, [signal({ id: 'sus-ladder', result: 'miss' })], day0 + 3 * DAY_MS);
            assert.equal(ladderResult.applied, 1);
            assert.deepEqual(rowOf('קָמָה'), before);
        });

        it('step and streak keep following the ladder, so a later ladder fallback resumes in place', () => {
            const day0 = atLocal(2026, 10, 10, 10);
            fsrs.applySignals(userA, [signal({ id: 'sh1' })], day0);
            fsrs.applySignals(userA, [signal({ id: 'sh2' })], day0 + 2 * DAY_MS);
            const row = rowOf('קָמָה');
            const ladder1 = HebrewReview.nextState(null, 'pass', day0);
            const ladder2 = HebrewReview.nextState(ladder1, 'pass', day0 + 2 * DAY_MS);
            assert.equal(row.step, ladder2.step);
            assert.equal(row.streak, ladder2.streak);
        });

        it('a passed row the migration never saw starts from the ADR-0009 §2 migration values', () => {
            const day0 = atLocal(2026, 10, 11, 10);
            review.upsert({
                user_id: userA,
                word: 'ד',
                direction: 'he_en',
                due: new Date(day0).toISOString(),
                step: 3,
                streak: 3,
                lapses: 1,
                last_result: 'pass',
                last_at: new Date(day0 - 14 * DAY_MS).toISOString(),
                sources: '[]'
            });
            const raw = rowOf('ד');
            assert.equal(raw.stability, null);
            const card = fsrs.cardForRow(userA, raw, day0);
            assert.equal(card.stability, 14);
            assert.equal(card.difficulty, 5.5);
            assert.equal(card.reps, 4);
            assert.equal(card.state, 'review');
            assert.equal(fsrs.metrics(userA, day0).stages.he_en.longTerm, 1);

            fsrs.applySignals(userA, [signal({ id: 'bf1', word: 'ד' })], day0);
            const graded = rowOf('ד');
            const expected = Fsrs.nextCard(card, 3, day0);
            assertClose(graded.stability, expected.stability, 'stability');
            assertClose(graded.difficulty, expected.difficulty, 'difficulty');
            assert.equal(graded.reps, 5);
        });

        it('stage reads stability: learning below 14 days, long-term from 14; ladder still reads step', () => {
            const day0 = atLocal(2026, 10, 12, 10);
            const base = {
                user_id: userA,
                direction: 'he_en',
                due: new Date(day0).toISOString(),
                streak: 1,
                lapses: 0,
                last_at: new Date(day0).toISOString(),
                sources: '[]'
            };
            review.upsert(Object.assign({}, base, { word: 'new', step: 0, last_result: null }));
            review.upsert(Object.assign({}, base, {
                word: 'below', step: 4, last_result: 'pass',
                stability: 13.99, difficulty: 5, reps: 5, fsrs_state: 'review', last_grade: 3
            }));
            review.upsert(Object.assign({}, base, {
                word: 'at', step: 4, last_result: 'pass',
                stability: 14, difficulty: 5, reps: 5, fsrs_state: 'review', last_grade: 3
            }));
            assert.deepEqual(fsrs.metrics(userA, day0).stages.he_en, { new: 1, learning: 1, longTerm: 1 });
            assert.deepEqual(ladder.metrics(userA, day0).stages.he_en, { new: 1, learning: 0, longTerm: 2 });
        });
    });

    describe('form signals (both schedulers)', () => {
        for (const mode of ['fsrs', 'ladder']) {
            it('ac-4: a form signal inserts a practice_events row with kind form and lemma, and creates no review_items row — ' + mode, () => {
                const progress = mode === 'fsrs' ? fsrs : ladder;
                const lemmaNfd = 'לִכְתֹּב'.normalize('NFD');
                const result = progress.applySignals(userA, [
                    signal({ id: 'form-1-' + mode, kind: 'form', word: 'כָּתַבְתָּ', lemma: lemmaNfd, direction: undefined }),
                    signal({ id: 'form-2-' + mode, kind: 'form', word: 'כָּתַבְתְּ', lemma: 'לִכְתֹּב', result: 'miss' })
                ], atLocal(2026, 10, 13, 10));
                assert.equal(result.applied, 2);
                assert.equal(result.rejected, 0);
                const first = events.find(userA, 'form-1-' + mode);
                assert.equal(first.kind, 'form');
                assert.equal(first.lemma, 'לִכְתֹּב'.normalize('NFC'));
                assert.equal(first.word, 'כָּתַבְתָּ'.normalize('NFC'));
                assert.equal(first.direction, 'en_he');
                assert.equal(first.grade, 3);
                assert.equal(events.find(userA, 'form-2-' + mode).grade, 1);
                assert.equal(reviewRowCount(), 0);
                assert.equal(Object.keys(result.state.words).length, 0);
            });
        }

        it('a form signal leaves an existing review row for the same word alone', () => {
            const day0 = atLocal(2026, 10, 14, 10);
            fsrs.applySignals(userA, [signal({ id: 'fw-seed', word: 'לִכְתֹּב' })], day0);
            const before = rowOf('לִכְתֹּב');
            fsrs.applySignals(userA, [
                signal({ id: 'fw-form', kind: 'form', word: 'לִכְתֹּב', lemma: 'לִכְתֹּב', result: 'miss' })
            ], day0 + 2 * DAY_MS);
            assert.deepEqual(rowOf('לִכְתֹּב'), before);
        });

        it('form events stay out of word stages, accuracy, and trouble words', () => {
            const day0 = atLocal(2026, 10, 15, 10);
            const forms = [];
            for (let i = 0; i < 12; i += 1) {
                forms.push(signal({ id: 'fm-' + i, kind: 'form', word: 'קָמָה', lemma: 'לָקוּם',
                    result: i < 2 ? 'pass' : 'miss' }));
            }
            fsrs.applySignals(userA, forms, day0);
            const m = fsrs.metrics(userA, day0);
            assert.deepEqual(m.accuracy.en_he, { pass: 0, miss: 0, pct: null });
            assert.deepEqual(m.accuracy.he_en, { pass: 0, miss: 0, pct: null });
            assert.deepEqual(m.stages.he_en, { new: 0, learning: 0, longTerm: 0 });
            assert.deepEqual(fsrs.troubleWords(userA, day0), []);
        });
    });

    describe('ladder (SRS_SCHEDULER=ladder)', () => {
        it('ac-5: the schedule is HebrewReview.nextState and the grade is still written', () => {
            const day0 = atLocal(2026, 10, 16, 10);
            ladder.applySignals(userA, [signal({ id: 'l1' })], day0);
            const passed = rowOf('קָמָה');
            const direct = HebrewReview.nextState(null, 'pass', day0);
            assert.equal(passed.due, direct.due);
            assert.equal(passed.step, direct.step);
            assert.equal(passed.streak, direct.streak);
            assert.equal(passed.last_grade, 3);
            assert.equal(passed.stability, null);
            assert.equal(passed.fsrs_state, null);
            assert.equal(events.find(userA, 'l1').grade, 3);

            ladder.applySignals(userA, [signal({ id: 'l2', result: 'miss' })], day0 + 2 * DAY_MS);
            const missed = rowOf('קָמָה');
            assert.equal(missed.step, 0);
            assert.equal(missed.lapses, 1);
            assert.equal(missed.last_grade, 1);
            assert.equal(Date.parse(missed.due), day0 + 3 * DAY_MS);

            ladder.applySignals(userA, [signal({ id: 'l3', result: 'not_yet' })], day0 + 3 * DAY_MS);
            assert.equal(rowOf('קָמָה').last_grade, 1);
            assert.equal(events.find(userA, 'l3').grade, null);
        });

        it('leaves stored FSRS values alone and schedules explicit Hard or Easy as a ladder pass', () => {
            const day0 = atLocal(2026, 10, 17, 10);
            review.upsert({
                user_id: userA,
                word: 'ה',
                direction: 'he_en',
                due: new Date(day0).toISOString(),
                step: 1,
                streak: 1,
                lapses: 0,
                last_result: 'pass',
                last_at: new Date(day0 - 3 * DAY_MS).toISOString(),
                sources: '[]',
                stability: 3,
                difficulty: 5,
                reps: 1,
                fsrs_state: 'review',
                last_grade: 3
            });
            const before = rowOf('ה');
            ladder.applySignals(userA, [signal({ id: 'lh', word: 'ה', grade: 2 })], day0);
            const after = rowOf('ה');
            const direct = HebrewReview.nextState({
                due: before.due, step: 1, streak: 1, lapses: 0, lastResult: 'pass', lastAt: before.last_at
            }, 'pass', day0);
            assert.equal(after.step, direct.step);
            assert.equal(after.due, direct.due);
            assert.equal(after.last_grade, 2);
            assert.equal(after.stability, 3);
            assert.equal(after.difficulty, 5);
            assert.equal(after.reps, 1);
            assert.equal(after.fsrs_state, 'review');
        });

        it('ac-3: two pass signals on the same local day advance one step', () => {
            ladder.applySignals(userA, [signal({ id: 'ls1' })], atLocal(2026, 10, 18, 9));
            ladder.applySignals(userA, [signal({ id: 'ls2' })], atLocal(2026, 10, 18, 18));
            assert.equal(rowOf('קָמָה').step, 1);
        });
    });
});
