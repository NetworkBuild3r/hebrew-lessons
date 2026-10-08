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
        const html = fs.readFileSync(path.join(root, 'review', 'index.html'), 'utf8');
        assert.match(html, /worksheets\/_review\/vocab-index\.js/);
        assert.match(html, /shared\/review-page\.js/);
        assert.match(html, /shared\/player\/review\.js/);
        assert.doesNotMatch(html, /\bfetch\s*\(/);
        const js = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.doesNotMatch(js, /\bfetch\s*\(/);
    });

    it('escapes index text (XSS fixture renders as text, not markup)', () => {
        const dirty = '<img onerror="alert(1)">evil';
        const out = ReviewPage.escapeIndexText(dirty);
        assert.match(out, /&lt;img/);
        assert.doesNotMatch(out, /<img/);
        assert.match(out, /onerror/);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /esc\(meaning\)/);
        assert.match(pageJs, /esc\(word\)|esc\(displayHebrew/);
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
        assert.match(pageJs, /data-signal="miss"/);
        assert.match(pageJs, /if \(!state\.revealed/);
        assert.match(pageJs, /review-answer hidden/);
    });

    it('typed en→he answers match without nikud', () => {
        assert.equal(ReviewPage.typedMatches('בקר', 'בֹּקֶר'), true);
        assert.equal(ReviewPage.typedMatches('בֹּקֶר', 'בֹּקֶר'), true);
        assert.equal(ReviewPage.typedMatches('קם', 'בֹּקֶר'), false);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /normalizeHebrew/);
        assert.match(pageJs, /review-typed/);
    });

    it('home card due count matches filterQueue and hides when zero', () => {
        // INIT-002/SPEC-014 moved the home due card into Today. The count still
        // comes from HebrewReview.dueList; the page links to /review/.
        const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
        const todayJs = fs.readFileSync(path.join(root, 'shared', 'today-page.js'), 'utf8');
        assert.match(html, /shared\/player\/review\.js/);
        assert.match(html, /shared\/today-page\.js/);
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
        assert.match(pageJs, /ידעתי/);
        assert.match(pageJs, /Knew it/);
        assert.match(pageJs, /פספסתי/);
        assert.match(pageJs, /Missed it/);
        assert.match(pageJs, /הסתר ניקוד/);
        assert.match(pageJs, /Hide vowels/);
        assert.match(pageJs, /hebrew-review-prefs:v1/);
        const html = fs.readFileSync(path.join(root, 'review', 'index.html'), 'utf8');
        assert.match(html, /Review today/);
        assert.match(html, /חזרה היום/);
    });

    it('missing vocab-index shows bilingual no-review-data message instead of failing', () => {
        assert.equal(ReviewPage.vocabListPresent(undefined), false);
        assert.equal(ReviewPage.vocabListPresent(null), false);
        assert.equal(ReviewPage.vocabListPresent([]), true);
        assert.equal(ReviewPage.vocabListPresent({}), true);
        const pageJs = fs.readFileSync(path.join(root, 'shared', 'review-page.js'), 'utf8');
        assert.match(pageJs, /review-no-index/);
        assert.match(pageJs, /No review data yet/);
        assert.match(pageJs, /אין עדיין נתוני חזרה/);
        assert.match(pageJs, /renderMissing/);
    });

    it('worksheet.css marks SPEC-004 review-page section for SPEC-007', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        assert.match(css, /INIT-001\/SPEC-004 review-page/);
        assert.match(css, /SPEC-007 appends after this block/);
        assert.match(css, /end INIT-001\/SPEC-004 review-page/);
    });

    it('web server allowlists /review/ for the Review today page', () => {
        const { publicFile } = require('../server/web.js');
        const file = publicFile('/review/index.html');
        assert.ok(file);
        assert.ok(fs.existsSync(file));
        assert.equal(path.basename(path.dirname(file)), 'review');
        assert.equal(publicFile('/review/../shared/worksheet.css'), null);
    });
});
