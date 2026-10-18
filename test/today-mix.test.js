/**
 * Today daily mix: section order, drill hidden until the drill module
 * is loaded, escaped labels, and ctx.signal on the mix fetch.
 * Authored-by-spec: INIT-006/SPEC-015
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadUtils() {
    const code = fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8');
    const sandbox = { window: {}, console };
    sandbox.globalThis = sandbox;
    vm.runInNewContext(code, sandbox);
    return sandbox.window.HebrewUtils;
}

let Today;

before(() => {
    global.HebrewUtils = loadUtils();
    Today = require('../shared/today-page.js');
});

function readyModel(mix) {
    return {
        state: 'ready',
        catalog: [{ slug: 'my-morning', title: 'בוקר', subtitle: '', level: 'basic' }],
        brandNew: false,
        review: { dueCount: 1, newCount: 0, nextDueMs: null },
        continueEntry: null,
        nextLesson: null,
        mix: mix
    };
}

describe('today mix (INIT-006/SPEC-015)', () => {
    it('omits drill without the drill module and omits a missing reading section', () => {
        delete global.HebrewBlockConjugation;
        const shown = Today.visibleSections({
            sections: [
                { id: 'reading', items: [{ sentence: 'שלום' }] },
                { id: 'drill', label: 'Past', items: [{ key: 'א', prompt: 'I' }] },
                { id: 'weak', items: [{ key: 'ב', prompt: 'b', meaning: 'b' }] },
                { id: 'review', items: [{ key: 'ג', prompt: 'c', meaning: 'c' }] },
                { id: 'extra', items: [{ key: 'ד' }] }
            ]
        }).map((section) => section.id);
        assert.deepEqual(shown, ['review', 'weak', 'reading']);
    });

    it('shows drill once the conjugation module is loaded, still in order', () => {
        global.HebrewBlockConjugation = {
            decideAttempt: function () { return { attempted: false }; },
            formSignal: function () { return null; }
        };
        const shown = Today.visibleSections({
            sections: [
                { id: 'drill', items: [{ key: 'א' }] },
                { id: 'review', items: [{ key: 'ב' }] }
            ]
        }).map((section) => section.id);
        assert.deepEqual(shown, ['review', 'drill']);
        delete global.HebrewBlockConjugation;
    });

    it('drops empty sections and does not invent a reading', () => {
        delete global.HebrewBlockConjugation;
        const shown = Today.visibleSections({
            sections: [
                { id: 'review', items: [] },
                { id: 'weak', items: [{ key: 'א' }] }
            ]
        }).map((section) => section.id);
        assert.deepEqual(shown, ['weak']);
    });

    it('escapes a drill label and does not put card text in the list', () => {
        global.HebrewBlockConjugation = {
            decideAttempt: function () { return { attempted: false }; },
            formSignal: function () { return null; }
        };
        const html = Today.render(readyModel({
            sections: [{
                id: 'drill',
                label: '<img src=x onerror=alert(1)>',
                items: [{ key: 'א', prompt: '<script>alert(1)</script>', accept: ['א'] }]
            }, {
                id: 'review',
                items: [{ key: 'ב', direction: 'he_en', prompt: '<script>alert(1)</script>', meaning: 'secret-gloss' }]
            }]
        }));
        assert.equal(html.includes('<img'), false);
        assert.equal(html.includes('<script'), false);
        assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.equal(html.includes('secret-gloss'), false);
        assert.equal(html.includes('&lt;script&gt;'), false);
        assert.match(html, /id="today-mix"/);
        assert.match(html, /href="\/review\/"/);
        delete global.HebrewBlockConjugation;
    });

    it('puts one Learning path link per plan id ahead of the other cards', () => {
        const model = readyModel(null);
        model.planIds = ['bdf0aee7-7c57-4982-a351-78063ff567e9', 'bdf0aee7-7c57-4982-a351-78063ff567e9'];
        const html = Today.render(model);
        assert.equal((html.match(/data-learning-path=/g) || []).length, 1);
        assert.match(html, /href="\/shared\/plan\/\?id=bdf0aee7-7c57-4982-a351-78063ff567e9"/);
        assert.match(html, /<bdi class="ui-he" lang="he">מסלול לימוד<\/bdi>/);
        assert.ok(html.indexOf('data-learning-path=') < html.indexOf('id="today-review-card"'));
    });

    it('keeps only a same-origin worksheet mp3 as a clip', () => {
        global.location = {
            href: 'http://127.0.0.1:9/',
            origin: 'http://127.0.0.1:9'
        };
        assert.equal(
            Today.clipHref('/worksheets/my-morning/audio/sentences/vocab-5.mp3'),
            '/worksheets/my-morning/audio/sentences/vocab-5.mp3'
        );
        assert.equal(Today.clipHref('javascript:alert(1)'), '');
        assert.equal(Today.clipHref('http://evil.example/worksheets/my-morning/a.mp3'), '');
        assert.equal(Today.clipHref('/worksheets/../secret.mp3'), '');
        assert.equal(Today.clipHref('audio/sentences/vocab-5.mp3'), '');
        assert.equal(Today.clipHref('/api/me/mix.mp3'), '');
    });

    it('passes ctx.signal to GET /api/me/mix and ignores the result after unmount', async () => {
        const calls = [];
        let release;
        const gate = new Promise((resolve) => { release = resolve; });
        global.HEBREW_CATALOG = [{ slug: 'my-morning' }];
        global.HebrewApi = {
            get(path, opts) {
                calls.push({ path: path, signal: opts && opts.signal });
                if (path === '/api/me/mix') return gate;
                return Promise.resolve({ ok: true, data: { state: {} } });
            }
        };
        const controller = new AbortController();
        const el = {
            innerHTML: '',
            listeners: 0,
            addEventListener() { this.listeners += 1; },
            removeEventListener() { this.listeners -= 1; }
        };
        const pending = Today.mount(el, { signal: controller.signal, ready: Promise.resolve() });
        await new Promise((resolve) => setImmediate(resolve));
        const mixCall = calls.find((call) => call.path === '/api/me/mix');
        assert.ok(mixCall, 'mix was fetched');
        assert.equal(mixCall.signal, controller.signal);
        controller.abort();
        assert.equal(controller.signal.aborted, true);
        release({
            ok: true,
            data: { sections: [{ id: 'review', items: [{ key: 'א', prompt: 'א', meaning: 'a' }] }] }
        });
        await pending;
        assert.equal(el.innerHTML, '');
        Today.unmount();
        assert.equal(el.listeners, 0);
    });
});
