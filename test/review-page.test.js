/**
 * Review today page — static + scheduler wiring (INIT-001/SPEC-004).
 */
const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.join(__dirname, '..');
const Review = require('../shared/player/review.js');

function loadUtils() {
    const code = fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8');
    const sandbox = { window: {}, module: { exports: {} }, console };
    sandbox.globalThis = sandbox;
    vm.runInNewContext(code, sandbox);
    return sandbox.window.HebrewUtils || sandbox.globalThis.HebrewUtils;
}

const U = loadUtils();
globalThis.HebrewUtils = U;
const ReviewPage = require('../shared/review-page.js');

function homeFilterQueue(rawQueue, words) {
    return rawQueue.filter(function (item) {
        if (!item || item.direction !== 'en_he') return true;
        const he = words[item.key] && words[item.key].he_en;
        return !!(he && he.lastResult === 'pass');
    });
}

describe('review page (INIT-001/SPEC-004)', () => {
    beforeEach(() => {
        Review._resetMemory(null);
    });

    it('loads vocab via script tags only — no fetch of lesson data', () => {
        const routes = require('../shared/app-routes.js');
        const review = routes.find((r) => r.id === 'review');
        assert.ok(review.scripts.includes('/worksheets/_review/vocab-index.js'));
        assert.ok(review.scripts.includes('/shared/review-page.js'));
        const shell = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8');
        assert.match(shell, /shared\/player\/review\.js/);
        const router = fs.readFileSync(path.join(root, 'shared', 'router.js'), 'utf8');
        assert.doesNotMatch(router, /\bfetch\s*\(/);
        const js = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.doesNotMatch(js, /\bfetch\s*\(/);
    });

    it('escapes index text and does not assign card text with innerHTML', () => {
        const dirty = '<img onerror="alert(1)">evil';
        const out = ReviewPage.escapeIndexText(dirty);
        assert.match(out, /&lt;img/);
        assert.doesNotMatch(out, /<img/);
        assert.match(out, /onerror/);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /textContent/);
        assert.doesNotMatch(pageJs, /\.innerHTML\s*=/);
        assert.doesNotMatch(pageJs, /insertAdjacentHTML|outerHTML|document\.write/);
    });

    it('interleaves due items from my-morning and anal-sex-vocab', () => {
        const past = new Date(Date.now() - 3600 * 1000).toISOString();
        const dir = () => ({
            due: past,
            step: 0,
            streak: 0,
            lapses: 0,
            lastResult: 'pass',
            lastAt: past
        });
        const store = {
            v: 1,
            words: {
                'בֹּקֶר': { he_en: dir(), en_he: dir(), sources: ['my-morning'] },
                'קָם': { he_en: dir(), en_he: dir(), sources: ['my-morning'] },
                'אָנָלִי': { he_en: dir(), en_he: dir(), sources: ['anal-sex-vocab'] },
                'סֶקְס': { he_en: dir(), en_he: dir(), sources: ['anal-sex-vocab'] }
            },
            migratedLessons: {}
        };
        const queue = ReviewPage.filterQueue(Review.dueList(store, Date.now()), store.words);
        const lessons = queue.map((i) => i.lesson);
        assert.ok(lessons.includes('my-morning'));
        assert.ok(lessons.includes('anal-sex-vocab'));
        let ok = false;
        for (let i = 0; i < queue.length - 1; i += 1) {
            if (queue[i].lesson !== queue[i + 1].lesson) {
                ok = true;
                break;
            }
        }
        assert.ok(ok, 'expected lesson sources to interleave');
    });

    it('filters en→he until after first he→en pass (aud-1)', () => {
        const past = new Date(Date.now() - 1000).toISOString();
        const dueDir = {
            due: past,
            step: 0,
            streak: 0,
            lapses: 0,
            lastResult: null,
            lastAt: null
        };
        const passedHe = {
            due: past,
            step: 1,
            streak: 1,
            lapses: 0,
            lastResult: 'pass',
            lastAt: past
        };
        const words = {
            'בֹּקֶר': {
                he_en: dueDir,
                en_he: dueDir,
                sources: ['my-morning']
            },
            'קָם': {
                he_en: passedHe,
                en_he: dueDir,
                sources: ['my-morning']
            }
        };
        const items = [
            { key: 'בֹּקֶר', direction: 'he_en', lesson: 'my-morning' },
            { key: 'בֹּקֶר', direction: 'en_he', lesson: 'my-morning' },
            { key: 'קָם', direction: 'he_en', lesson: 'my-morning' },
            { key: 'קָם', direction: 'en_he', lesson: 'my-morning' }
        ];
        const filtered = ReviewPage.filterQueue(items, words);
        assert.deepEqual(
            filtered.map((i) => i.key + ':' + i.direction),
            ['בֹּקֶר:he_en', 'קָם:he_en', 'קָם:en_he']
        );
    });

    it('Knew it advances due via HebrewReview.record', () => {
        const past = new Date(Date.now() - 86400000).toISOString();
        Review._resetMemory({
            v: 1,
            words: {
                'בֹּקֶר': {
                    he_en: {
                        due: past,
                        step: 0,
                        streak: 0,
                        lapses: 0,
                        lastResult: null,
                        lastAt: null
                    },
                    en_he: {
                        due: past,
                        step: 0,
                        streak: 0,
                        lapses: 0,
                        lastResult: null,
                        lastAt: null
                    },
                    sources: ['my-morning']
                }
            },
            migratedLessons: {}
        });
        const before = Review.get('בֹּקֶר').he_en.due;
        const after = Review.record('בֹּקֶר', 'he_en', 'pass', {
            lesson: 'my-morning',
            now: Date.now()
        });
        assert.equal(after.lastResult, 'pass');
        assert.equal(after.step, 1);
        assert.notEqual(after.due, before);
        assert.ok(Date.parse(after.due) > Date.now());
    });

    it('Missed it resets step to 0 and due +1 day via record', () => {
        const past = new Date(Date.now() - 86400000).toISOString();
        Review._resetMemory({
            v: 1,
            words: {
                'קָם': {
                    he_en: {
                        due: past,
                        step: 3,
                        streak: 3,
                        lapses: 0,
                        lastResult: 'pass',
                        lastAt: past
                    },
                    en_he: {
                        due: past,
                        step: 0,
                        streak: 0,
                        lapses: 0,
                        lastResult: null,
                        lastAt: null
                    },
                    sources: ['my-morning']
                }
            },
            migratedLessons: {}
        });
        const after = Review.record('קָם', 'he_en', 'miss', {
            lesson: 'my-morning',
            now: Date.now()
        });
        assert.equal(after.lastResult, 'miss');
        assert.equal(after.step, 0);
        assert.equal(after.streak, 0);
        assert.equal(after.lapses, 1);
        assert.ok(Date.parse(after.due) > Date.now());
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /data-grade/);
        assert.match(pageJs, /if \(!state\.revealed\)/);
        assert.match(pageJs, /grades go to the server/i);
        assert.doesNotMatch(pageJs, /HebrewReview\.record/);
    });

    it('typed en→he answers match without nikud', () => {
        assert.equal(ReviewPage.typedMatches('בקר', 'בֹּקֶר'), true);
        assert.equal(ReviewPage.typedMatches('בֹּקֶר', 'בֹּקֶר'), true);
        assert.equal(ReviewPage.typedMatches('קם', 'בֹּקֶר'), false);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        const helper = fs.readFileSync(path.join(root, 'shared', 'player', 'flashcard-answer.js'), 'utf8');
        assert.match(helper, /stripVowels/);
        assert.match(pageJs, /review-typed/);
        assert.equal(ReviewPage.typedMatches('בקר', 'בֹּקֶר', ['בוקר']), true);
    });

    it('home card due count matches filterQueue and hides when zero', () => {
        // INIT-002/SPEC-014 moved the home due card into Today. The count still
        // comes from HebrewReview.dueList; the page links to /review/.
        const html = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8');
        const todayJs = fs.readFileSync(path.join(root, 'shared', 'today-page.js'), 'utf8');
        assert.match(html, /shared\/player\/review\.js/);
        const today = require('../shared/app-routes.js').find((r) => r.id === 'today');
        assert.deepEqual(today.scripts, [
            '/shared/ui/blocks/conjugation-table.js',
            '/shared/verb-practice.js',
            '/shared/learning-paths.js',
            '/shared/today-page.js'
        ]);
        assert.match(todayJs, /href="\/review\/"/);
        assert.match(todayJs, /dueCount/);

        const past = new Date(Date.now() - 1000).toISOString();
        const due = {
            due: past,
            step: 0,
            streak: 0,
            lapses: 0,
            lastResult: null,
            lastAt: null
        };
        const passed = {
            due: past,
            step: 1,
            streak: 1,
            lapses: 0,
            lastResult: 'pass',
            lastAt: past
        };
        const words = {
            'בֹּקֶר': { he_en: due, en_he: due, sources: ['my-morning'] },
            'קָם': { he_en: passed, en_he: due, sources: ['my-morning'] }
        };
        const store = { v: 1, words, migratedLessons: {} };
        const raw = Review.dueList(store, Date.now());
        const filtered = homeFilterQueue(raw, words);
        assert.deepEqual(
            ReviewPage.filterQueue(raw, words).map((i) => i.key + ':' + i.direction),
            filtered.map((i) => i.key + ':' + i.direction)
        );
        assert.ok(filtered.length > 0);

        const emptyStore = { v: 1, words: {}, migratedLessons: {} };
        assert.equal(homeFilterQueue(Review.dueList(emptyStore, Date.now()), {}).length, 0);
    });

    it('bilingual labels and nikud on by default with toggle', () => {
        assert.equal(ReviewPage.VOWELS_DEFAULT, true);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /הצג תשובה/);
        assert.match(pageJs, /Show answer/);
        assert.match(pageJs, /שוב/);
        assert.match(pageJs, /Again/);
        assert.match(pageJs, /קשה/);
        assert.match(pageJs, /Hard/);
        assert.match(pageJs, /טוב/);
        assert.match(pageJs, /Good/);
        assert.match(pageJs, /קל/);
        assert.match(pageJs, /Easy/);
        assert.match(pageJs, /הסתר ניקוד/);
        assert.match(pageJs, /Hide vowels/);
        assert.match(pageJs, /hebrew-review-prefs:v1/);
        const review = require('../shared/app-routes.js').find((r) => r.id === 'review');
        assert.equal(review.heading.en, 'Review today');
        assert.equal(review.heading.he, 'חזרה היום');
    });

    it('missing vocab-index shows bilingual no-review-data message instead of failing', () => {
        assert.equal(ReviewPage.vocabListPresent(undefined), false);
        assert.equal(ReviewPage.vocabListPresent(null), false);
        assert.equal(ReviewPage.vocabListPresent([]), true);
        assert.equal(ReviewPage.vocabListPresent({}), true);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /review-load-error/);
        assert.match(pageJs, /Couldn't load cards/);
        assert.match(pageJs, /לא הצלחנו לטעון כרטיסים/);
    });

    it('worksheet.css marks SPEC-004 review-page section for SPEC-007', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        assert.match(css, /INIT-001\/SPEC-004 review-page/);
        assert.match(css, /SPEC-007 appends after this block/);
        assert.match(css, /end INIT-001\/SPEC-004 review-page/);
    });

    it('server serves the app shell for /review/ (no separate review HTML file)', () => {
        const spa = require('../server/spa.js');
        assert.equal(spa.isAppPath('/review/'), true);
        assert.equal(spa.isAppPath('/review/index.html'), true);
        assert.equal(fs.existsSync(path.join(root, 'review', 'index.html')), false);
        const { publicFile } = require('../server/web.js');
        assert.equal(publicFile('/review/../shared/worksheet.css'), null);
    });
});
