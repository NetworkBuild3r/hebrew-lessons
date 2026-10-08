/**
 * Review store tests: localStorage, quota fail-soft, migration.
 * Authored-by-spec: INIT-001/SPEC-003
 */
const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const path = require('path');
const fs = require('fs');
const vm = require('vm');

const DAY_MS = 24 * 60 * 60 * 1000;
const STORAGE_KEY = 'hebrew-review:v1';

function makeFakeStorage(opts) {
    const data = Object.create(null);
    const throwOnSet = opts && opts.throwOnSet;
    return {
        getItem(key) {
            return Object.prototype.hasOwnProperty.call(data, key) ? data[key] : null;
        },
        setItem(key, value) {
            if (throwOnSet) {
                const err = new Error('QuotaExceededError');
                err.name = 'QuotaExceededError';
                throw err;
            }
            data[key] = String(value);
        },
        removeItem(key) {
            delete data[key];
        },
        clear() {
            Object.keys(data).forEach(k => delete data[k]);
        },
        _data: data
    };
}

function loadReview(localStorage) {
    const sandbox = {
        console,
        localStorage,
        window: {},
        globalThis: {},
        module: { exports: {} },
        exports: {}
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    const src = fs.readFileSync(
        path.resolve(__dirname, '..', 'shared', 'player', 'review.js'),
        'utf8'
    );
    vm.runInContext(src, sandbox);
    return sandbox.HebrewReview || sandbox.module.exports;
}

describe('HebrewReview store', () => {
    let storage;
    let Review;

    beforeEach(() => {
        storage = makeFakeStorage();
        Review = loadReview(storage);
        Review._resetMemory(null);
    });

    afterEach(() => {
        if (Review) Review._resetMemory(null);
    });

    it('load returns empty store when missing', () => {
        const store = Review.load();
        assert.equal(store.v, 1);
        assert.equal(Object.keys(store.words).length, 0);
        assert.equal(Object.keys(store.migratedLessons).length, 0);
    });

    it('load fail-softs on parse errors', () => {
        storage.setItem(STORAGE_KEY, '{not-json');
        const store = Review.load();
        assert.equal(Object.keys(store.words).length, 0);
    });

    it('record creates he_en entry and persists', () => {
        const now = Date.UTC(2026, 9, 8, 12, 0, 0);
        Review.load();
        Review.record('קָמָה', 'he_en', 'pass', { lesson: 'my-morning', now });
        const got = Review.get('קָמָה');
        assert.ok(got);
        assert.equal(got.he_en.step, 1);
        assert.equal(got.he_en.lastResult, 'pass');
        assert.ok(got.sources.includes('my-morning'));
        const raw = JSON.parse(storage.getItem(STORAGE_KEY));
        assert.ok(raw.words[Review.keyFor('קָמָה')]);
    });

    it('setItem quota keeps in-memory state and does not throw', () => {
        const quotaStorage = makeFakeStorage({ throwOnSet: true });
        const QReview = loadReview(quotaStorage);
        QReview._resetMemory(null);
        QReview.load();
        assert.doesNotThrow(() => {
            QReview.record('מַיִם', 'he_en', 'miss', { lesson: 'test', now: Date.now() });
        });
        const got = QReview.get('מַיִם');
        assert.ok(got);
        assert.equal(got.he_en.lastResult, 'miss');
        assert.equal(quotaStorage.getItem(STORAGE_KEY), null);
    });

    it('quota on setItem: mark + migrate paths stay runnable with a clean console', () => {
        const quotaStorage = makeFakeStorage({ throwOnSet: true });
        const errors = [];
        const sandbox = {
            console: {
                log: console.log.bind(console),
                warn: console.warn.bind(console),
                error: function () { errors.push(Array.from(arguments)); }
            },
            localStorage: quotaStorage,
            window: {},
            globalThis: {},
            module: { exports: {} },
            exports: {}
        };
        sandbox.window = sandbox;
        sandbox.globalThis = sandbox;
        vm.createContext(sandbox);
        vm.runInContext(
            fs.readFileSync(path.resolve(__dirname, '..', 'shared', 'player', 'review.js'), 'utf8'),
            sandbox
        );
        const QReview = sandbox.HebrewReview;
        QReview._resetMemory(null);
        QReview.load();

        // Simulate words self-mark + core migration under a full quota (ac-5).
        assert.doesNotThrow(() => {
            QReview.record('קָם', 'he_en', 'pass', { lesson: 'my-morning', now: Date.now() });
            QReview.migrateLessonMarks(
                { wordMarks: { w1: 'missed' } },
                { id: 'my-morning', vocab: { items: [{ id: 'w1', word: 'קָם' }] } },
                Date.now()
            );
            QReview.record('קָם', 'he_en', 'miss', { lesson: 'my-morning', now: Date.now() });
        });

        assert.ok(QReview.get('קָם'));
        assert.equal(QReview.get('קָם').he_en.lastResult, 'miss');
        assert.equal(quotaStorage.getItem(STORAGE_KEY), null);
        assert.equal(errors.length, 0, 'review store must not console.error on quota');
    });

    it('migrateLessonMarks maps missed→due now and knew→step 0 +1 day', () => {
        const now = Date.UTC(2026, 9, 8, 15, 0, 0);
        const ws = {
            id: 'my-morning',
            vocab: {
                items: [
                    { id: 'w1', word: 'קָם', meaning: 'he gets up' },
                    { id: 'w2', word: 'שׁוֹתֶה', meaning: 'drinks (m)' }
                ]
            }
        };
        Review.load();
        Review.migrateLessonMarks({
            wordMarks: { w1: 'missed', w2: 'knew' }
        }, ws, now);

        const missed = Review.get('קָם');
        assert.equal(missed.he_en.step, 0);
        assert.equal(missed.he_en.lastResult, 'miss');
        assert.equal(Date.parse(missed.he_en.due), now);
        assert.equal(missed.he_en.lapses, 1);

        const knew = Review.get('שׁוֹתֶה');
        assert.equal(knew.he_en.step, 0);
        assert.equal(knew.he_en.lastResult, 'pass');
        assert.equal(Date.parse(knew.he_en.due), now + DAY_MS);

        const store = Review.all();
        assert.equal(store.migratedLessons['my-morning'], true);
    });

    it('re-running migration is a no-op', () => {
        const now = Date.UTC(2026, 9, 8, 15, 0, 0);
        const ws = {
            id: 'lesson-x',
            vocab: { items: [{ id: 'a', word: 'בֹּקֶר', meaning: 'morning' }] }
        };
        Review.load();
        Review.migrateLessonMarks({ wordMarks: { a: 'knew' } }, ws, now);
        const firstDue = Review.get('בֹּקֶר').he_en.due;

        Review.record('בֹּקֶר', 'he_en', 'miss', { lesson: 'lesson-x', now: now + DAY_MS });
        assert.equal(Review.get('בֹּקֶר').he_en.lastResult, 'miss');

        Review.migrateLessonMarks({ wordMarks: { a: 'knew' } }, ws, now + 2 * DAY_MS);
        assert.equal(Review.get('בֹּקֶר').he_en.lastResult, 'miss');
        assert.notEqual(Review.get('בֹּקֶר').he_en.due, firstDue);
    });
});

describe('HebrewReview module surface', () => {
    it('exports via require and window.HebrewReview', () => {
        const viaRequire = require(path.resolve(__dirname, '..', 'shared', 'player', 'review.js'));
        assert.equal(typeof viaRequire.nextState, 'function');
        assert.equal(typeof viaRequire.record, 'function');
        assert.equal(viaRequire.STORAGE_KEY, 'hebrew-review:v1');
    });
});
