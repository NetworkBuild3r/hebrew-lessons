/**
 * Story glosses, coverage estimate, XSS escape, not_yet once/day.
 * Authored-by-spec: INIT-001/SPEC-007
 */
const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function makeDom() {
    const listeners = { keydown: [], click: [] };
    const document = {
        createElement(tag) {
            const el = {
                tagName: String(tag).toUpperCase(),
                className: '',
                style: {},
                children: [],
                attributes: Object.create(null),
                dataset: Object.create(null),
                textContent: '',
                innerHTML: '',
                parentNode: null,
                childNodes: [],
                setAttribute(k, v) { this.attributes[k] = String(v); },
                getAttribute(k) { return this.attributes[k] || null; },
                appendChild(child) {
                    child.parentNode = this;
                    this.children.push(child);
                    this.childNodes.push(child);
                    return child;
                },
                remove() {
                    if (!this.parentNode) return;
                    const p = this.parentNode;
                    if (Array.isArray(p.children)) p.children = p.children.filter(c => c !== this);
                    if (Array.isArray(p.childNodes)) p.childNodes = p.childNodes.filter(c => c !== this);
                    if (p._pop === this) p._pop = null;
                    this.parentNode = null;
                },
                querySelector() { return null; },
                querySelectorAll() { return []; },
                closest() { return null; },
                focus() {},
                getBoundingClientRect() {
                    return { top: 0, left: 0, bottom: 20, right: 40, width: 40, height: 20 };
                },
                scrollTop: 0,
                scrollLeft: 0,
                clientWidth: 400,
                offsetWidth: 160
            };
            return el;
        },
        createDocumentFragment() {
            return {
                childNodes: [],
                appendChild(c) { this.childNodes.push(c); return c; }
            };
        },
        createTreeWalker() {
            return { nextNode() { return null; } };
        },
        createTextNode(text) {
            return { nodeType: 3, nodeValue: String(text), textContent: String(text), parentNode: null };
        },
        addEventListener(type, fn) {
            if (listeners[type]) listeners[type].push(fn);
        },
        removeEventListener(type, fn) {
            if (listeners[type]) listeners[type] = listeners[type].filter(f => f !== fn);
        }
    };
    return { document, listeners };
}

function loadStory() {
    const { document, listeners } = makeDom();
    const storage = {
        _data: Object.create(null),
        getItem(k) { return Object.prototype.hasOwnProperty.call(this._data, k) ? this._data[k] : null; },
        setItem(k, v) { this._data[k] = String(v); },
        removeItem(k) { delete this._data[k]; }
    };
    let registered = null;
    const sandbox = {
        console,
        localStorage: storage,
        document,
        NodeFilter: { SHOW_TEXT: 4 },
        window: {},
        globalThis: {},
        module: { exports: {} },
        exports: {}
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    sandbox.HebrewPlayer = {
        register(def) { registered = def; }
    };
    sandbox.window.HebrewPlayer = sandbox.HebrewPlayer;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'review.js'), 'utf8'), sandbox);
    sandbox.HebrewSpeech = {
        story: { status: 'idle', index: 0 },
        onStoryPaint: null,
        startStory() {},
        pauseStory() {},
        stopStory() {},
        speak() {}
    };
    sandbox.window.HebrewSpeech = sandbox.HebrewSpeech;
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'activities', 'story.js'), 'utf8'), sandbox);
    return {
        def: registered,
        U: sandbox.HebrewUtils,
        Review: sandbox.HebrewReview,
        storage,
        sandbox,
        listeners
    };
}

function baseWs(extra) {
    return Object.assign({
        id: 'my-morning',
        vocab: {
            items: [
                { id: '1', word: 'קָם', meaning: 'gets up (m. sg.)', audio: 'a.mp3' },
                { id: '2', word: 'קָפֶה', meaning: 'coffee', audio: 'b.mp3' }
            ]
        },
        reading: {
            paragraphs: [
                ['אֲנִי ', { word: 'קָם' }, '.']
            ],
            questions: [],
            audio: []
        }
    }, extra || {});
}

function makeCtx(bundle, ws, reviewOverride) {
    const Review = reviewOverride || bundle.Review;
    Review._resetMemory(null);
    Review.load();
    return {
        ws,
        vowelsOn: true,
        review: Review,
        bi: bundle.U.bi,
        esc: bundle.U.escapeHtml,
        sid: bundle.U.sid,
        speakButton: () => '',
        save() {}
    };
}

describe('story glosses + coverage', () => {
    let bundle;

    beforeEach(() => {
        bundle = loadStory();
        bundle.Review._resetMemory(null);
    });

    it('ac-4: glosses vocab words without reading.glosses', () => {
        const ctx = makeCtx(bundle, baseWs());
        const html = bundle.def.render(ctx, {}, {});
        assert.match(html, /story-coverage/);
        assert.match(html, /Coverage unknown|כיסוי לא ידוע/);
        // Token buttons are added at mount; render still shows vocab word text
        assert.match(html, /קָם/);
    });

    it('ac-3: empty store → unknown, seeded pass → percentage', () => {
        const ctxEmpty = makeCtx(bundle, baseWs());
        const htmlUnknown = bundle.def.render(ctxEmpty, {}, {});
        assert.match(htmlUnknown, /data-coverage="unknown"/);
        assert.doesNotMatch(htmlUnknown, /≈0%/);

        const ctx = makeCtx(bundle, baseWs());
        bundle.Review.record('קָם', 'he_en', 'pass', { lesson: 'my-morning' });
        const htmlPct = bundle.def.render(ctx, {}, {});
        assert.match(htmlPct, /data-coverage="\d+"/);
        assert.match(htmlPct, /מילים מוכרות \(known words, estimate\)/);
        assert.match(htmlPct, /Estimate only|אומדן בלבד/);
    });

    it('ac-1: tapping elsewhere closes the gloss popover', () => {
        const ws = baseWs();
        const ctx = makeCtx(bundle, ws);
        const root = {
            querySelector(sel) {
                if (sel === '.story-gloss-popover') return this._pop || null;
                if (sel === '#story-text') return this._body;
                return null;
            },
            querySelectorAll() { return []; },
            _body: {
                clientWidth: 400,
                scrollTop: 0,
                scrollLeft: 0,
                children: [],
                childNodes: [],
                appendChild(child) {
                    root._pop = child;
                    child.parentNode = root;
                    this.children.push(child);
                    this.childNodes.push(child);
                    child.querySelector = () => ({ focus() {} });
                    child.closest = (s) => (s === '.story-gloss-popover' ? child : null);
                    return child;
                },
                getBoundingClientRect() {
                    return { top: 0, left: 0, bottom: 100, right: 100, width: 100, height: 100 };
                }
            },
            _pop: null
        };
        const btn = {
            dataset: { action: 'story-token', token: 'קָם' },
            textContent: 'קָם',
            setAttribute() {},
            getBoundingClientRect() {
                return { top: 10, left: 10, bottom: 30, right: 50, width: 40, height: 20 };
            }
        };
        bundle.def.onClick(ctx, root, { stopPropagation() {} }, btn);
        assert.ok(root._pop, 'popover should open');

        // Click elsewhere (no data-action) — must dismiss
        bundle.def.onClick(ctx, root, {
            stopPropagation() {},
            target: { closest() { return null; } }
        }, null);
        assert.equal(root._pop, null, 'popover should close on tap elsewhere');
    });

    it('ac-5: gloss HTML is escaped via escapeHtml', () => {
        const evil = '<img onerror="window.__xss=1">evil';
        const escaped = bundle.U.escapeHtml(evil);
        assert.equal(escaped.includes('<img'), false);
        assert.match(escaped, /&lt;img/);
        assert.equal(escaped.includes('onerror='), true); // attribute text remains but escaped tags
        assert.ok(!/<img/i.test(escaped));
    });

    it('ac-1/ac-2: openGloss path records not_yet once per day', () => {
        const ws = baseWs({
            reading: {
                glosses: { 'קָם': '<b>gets up</b>' },
                paragraphs: [['קָם']],
                questions: [],
                audio: []
            }
        });
        const ctx = makeCtx(bundle, ws);
        const root = {
            querySelector(sel) {
                if (sel === '.story-gloss-popover') return this._pop || null;
                if (sel === '#story-text') return this._body;
                return null;
            },
            querySelectorAll() { return []; },
            _body: null,
            _pop: null
        };
        root._body = {
            clientWidth: 400,
            scrollTop: 0,
            scrollLeft: 0,
            children: [],
            childNodes: [],
            appendChild(child) {
                root._pop = child;
                child.parentNode = root;
                this.children.push(child);
                this.childNodes.push(child);
                child.querySelector = (s) => {
                    if (s === '[data-action="gloss-dismiss"]') return { focus() {} };
                    return null;
                };
                return child;
            },
            getBoundingClientRect() {
                return { top: 0, left: 0, bottom: 100, right: 100, width: 100, height: 100 };
            }
        };

        // Drive via onClick story-token
        const btn = {
            dataset: { action: 'story-token', token: 'קָם' },
            textContent: 'קָם',
            setAttribute() {},
            getBoundingClientRect() {
                return { top: 10, left: 10, bottom: 30, right: 50, width: 40, height: 20 };
            }
        };
        bundle.def.onClick(ctx, root, { stopPropagation() {} }, btn);
        const rec1 = bundle.Review.get('קָם');
        assert.ok(rec1);
        assert.equal(rec1.he_en.lastResult, 'not_yet');
        assert.equal(rec1.he_en.lapses, 0);
        const stepBefore = rec1.he_en.step;

        // Second tap same day — still not_yet, no double schedule churn
        bundle.def.onClick(ctx, root, { stopPropagation() {} }, btn);
        const rec2 = bundle.Review.get('קָם');
        assert.equal(rec2.he_en.lastResult, 'not_yet');
        assert.equal(rec2.he_en.step, stepBefore);

        // Popover meaning escaped
        assert.ok(root._pop);
        assert.match(root._pop.innerHTML, /&lt;b&gt;gets up&lt;\/b&gt;/);
        assert.doesNotMatch(root._pop.innerHTML, /<b>gets up<\/b>/);
    });

    it('not_yet does not reset a higher step', () => {
        const ctx = makeCtx(bundle, baseWs());
        bundle.Review._resetMemory({
            v: 1,
            words: {
                'קָם': {
                    he_en: {
                        due: new Date(Date.now() + 86400000 * 7).toISOString(),
                        step: 3,
                        streak: 3,
                        lapses: 0,
                        lastResult: 'pass',
                        lastAt: new Date(Date.now() - 86400000 * 2).toISOString()
                    },
                    en_he: {
                        due: new Date().toISOString(),
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
        const root = {
            querySelector(sel) {
                if (sel === '.story-gloss-popover') return this._pop || null;
                if (sel === '#story-text') return this._body;
                return null;
            },
            querySelectorAll() { return []; },
            _body: {
                clientWidth: 400,
                scrollTop: 0,
                scrollLeft: 0,
                children: [],
                childNodes: [],
                appendChild(child) {
                    root._pop = child;
                    child.parentNode = root;
                    this.children.push(child);
                    this.childNodes.push(child);
                    child.querySelector = () => ({ focus() {} });
                    return child;
                },
                getBoundingClientRect() {
                    return { top: 0, left: 0, bottom: 100, right: 100, width: 100, height: 100 };
                }
            },
            _pop: null
        };
        const btn = {
            dataset: { action: 'story-token', token: 'קָם' },
            textContent: 'קָם',
            setAttribute() {},
            getBoundingClientRect() {
                return { top: 10, left: 10, bottom: 30, right: 50, width: 40, height: 20 };
            }
        };
        bundle.def.onClick(ctx, root, { stopPropagation() {} }, btn);
        const rec = bundle.Review.get('קָם');
        assert.equal(rec.he_en.step, 3);
        assert.equal(rec.he_en.lapses, 0);
        assert.equal(rec.he_en.lastResult, 'not_yet');
    });

    it('CSS marker: SPEC-007 block appends after SPEC-004 end marker', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        const end004 = css.indexOf('/* === end INIT-001/SPEC-004 review-page === */');
        const start007 = css.indexOf('/* === INIT-001/SPEC-007 story glosses + coverage === */');
        const end007 = css.indexOf('/* === end INIT-001/SPEC-007 story glosses + coverage === */');
        assert.ok(end004 >= 0);
        assert.ok(start007 > end004);
        assert.ok(end007 > start007);
    });
});
