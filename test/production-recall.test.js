/**
 * SPEC-005: production recall direction helpers + type-it level gate + signals.
 * Authored-by-spec: INIT-001/SPEC-005
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const path = require('path');
const fs = require('fs');
const vm = require('vm');

const { directionForCard } = require('../shared/activities/words');

function loadSandbox(extraFiles) {
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        module: { exports: {} },
        exports: {}
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    const files = [
        path.join(__dirname, '../shared/player/utils.js'),
        ...(extraFiles || [])
    ];
    for (const file of files) {
        vm.runInNewContext(fs.readFileSync(file, 'utf8'), sandbox, { filename: file });
    }
    return sandbox;
}

describe('production recall direction', () => {
    it('alternates he_en / en_he per card index when auto', () => {
        assert.equal(directionForCard(0, 'auto'), 'he_en');
        assert.equal(directionForCard(1, 'auto'), 'en_he');
        assert.equal(directionForCard(2, 'auto'), 'he_en');
        assert.equal(directionForCard(3, 'auto'), 'en_he');
    });

    it('honors forced direction', () => {
        assert.equal(directionForCard(0, 'en_he'), 'en_he');
        assert.equal(directionForCard(1, 'he_en'), 'he_en');
        assert.equal(directionForCard(5, 'en_he'), 'en_he');
    });
});

describe('type-it level gate', () => {
    it('is available at intermediate and advanced, blocked at basic', () => {
        let registered = null;
        const sandbox = loadSandbox();
        sandbox.HebrewPlayer = {
            register(def) { registered = def; }
        };
        sandbox.window.HebrewPlayer = sandbox.HebrewPlayer;
        vm.runInNewContext(
            fs.readFileSync(path.join(__dirname, '../shared/activities/type-it.js'), 'utf8'),
            sandbox
        );
        assert.ok(registered && typeof registered.available === 'function');
        const available = registered.available;
        const vocab = { vocab: { items: [{ id: '1', word: 'קָם' }] } };
        assert.equal(available({ ...vocab, level: 'basic' }, {}), false);
        assert.equal(available({ ...vocab, level: 'intermediate' }, {}), true);
        assert.equal(available({ ...vocab, level: 'advanced' }, {}), true);
        assert.equal(available({ ...vocab, level: 'basic' }, { force: true }), true);
    });
});

describe('type-it matching and en_he recording', () => {
    it('accepts typed Hebrew without nikud via normalizeHebrew', () => {
        const sandbox = loadSandbox();
        const U = sandbox.HebrewUtils;
        const pointed = 'קָמָה';
        const bare = 'קמה';
        assert.equal(U.normalizeHebrew(bare), U.normalizeHebrew(pointed));
        assert.notEqual(bare, pointed);
    });

    it('records en_he pass/miss on check', () => {
        const sandbox = loadSandbox([
            path.join(__dirname, '../shared/player/review.js')
        ]);
        let def = null;
        const records = [];
        sandbox.HebrewPlayer = { register(d) { def = d; } };
        sandbox.window.HebrewPlayer = sandbox.HebrewPlayer;
        sandbox.localStorage = {
            _data: {},
            getItem(k) { return this._data[k] || null; },
            setItem(k, v) { this._data[k] = String(v); },
            removeItem(k) { delete this._data[k]; }
        };
        vm.runInNewContext(
            fs.readFileSync(path.join(__dirname, '../shared/activities/type-it.js'), 'utf8'),
            sandbox
        );
        assert.ok(def);

        const item = { id: '1', word: 'קָם', meaning: 'gets up' };
        const root = {
            _type: { queue: ['1'], index: 0, score: 0, hintOn: false },
            querySelector(sel) {
                if (sel === '#type-input') return { value: 'קם' };
                if (sel === '#type-feedback') return { classList: { remove() {} }, textContent: '' };
                if (sel === '#type-body') return { innerHTML: '' };
                return null;
            }
        };
        const ctx = {
            ws: { id: 'lesson-x', level: 'intermediate', vocab: { items: [item] } },
            sid: (id) => String(id),
            save() {},
            markComplete() {},
            review: {
                record(word, direction, signal, meta) {
                    records.push({ word, direction, signal, meta });
                    return sandbox.HebrewReview.record(word, direction, signal, meta);
                }
            }
        };

        // Freeze timers so show() after check does not advance the queue mid-assert
        const realSetTimeout = sandbox.setTimeout;
        sandbox.setTimeout = () => 0;
        def.onClick(ctx, root, {}, { dataset: { action: 'type-check' } });
        sandbox.setTimeout = realSetTimeout;

        assert.equal(records.length, 1);
        assert.equal(records[0].direction, 'en_he');
        assert.equal(records[0].signal, 'pass');
        assert.equal(records[0].word, 'קָם');

        records.length = 0;
        root.querySelector = (sel) => {
            if (sel === '#type-input') return { value: 'לא' };
            if (sel === '#type-feedback') return { classList: { remove() {} }, textContent: '' };
            if (sel === '#type-body') return { innerHTML: '' };
            return null;
        };
        sandbox.setTimeout = () => 0;
        def.onClick(ctx, root, {}, { dataset: { action: 'type-check' } });
        sandbox.setTimeout = realSetTimeout;

        assert.equal(records.length, 1);
        assert.equal(records[0].direction, 'en_he');
        assert.equal(records[0].signal, 'miss');
    });
});

describe('words round 1 recognition unchanged', () => {
    it('shows Hebrew and meaning together when not in recall mode', () => {
        let def = null;
        const sandbox = {
            console,
            HebrewPlayer: { register(d) { def = d; } },
            HebrewUtils: {
                stripVowels: (s) => String(s || '').replace(/[\u0591-\u05C7]/g, ''),
                escapeHtml: (s) => String(s == null ? '' : s),
                bi: (he, en) => he + ' (' + en + ')',
                sid: (id) => String(id)
            }
        };
        sandbox.window = sandbox;
        sandbox.globalThis = sandbox;
        vm.runInNewContext(
            fs.readFileSync(path.join(__dirname, '../shared/activities/words.js'), 'utf8'),
            sandbox
        );
        assert.ok(def);

        const item = {
            id: '1',
            word: 'קָם',
            meaning: 'gets up (m. sg.)',
            pos: 'verb',
            sound: 'kam'
        };
        const html = def.render({
            ws: { vocab: { items: [item], heading: 'מילים', note: 'note' } },
            recallMode: () => false,
            wordMarks: {},
            missedLast: [],
            vowelsOn: true,
            activityState: {},
            esc: (s) => String(s == null ? '' : s),
            bi: (he, en) => he + ' (' + en + ')',
            sid: (id) => String(id),
            speakButton: () => '<button data-speak></button>',
            recorderHtml: () => ''
        });

        assert.match(html, /class="he-word"/);
        assert.match(html, /gets up \(m\. sg\.\)/);
        assert.doesNotMatch(html, /data-direction="en_he"/);
        assert.doesNotMatch(html, /data-action="set-dir"/);
    });

    it('marks en_he when grading a production card', () => {
        let def = null;
        const sandbox = {
            console,
            HebrewPlayer: { register(d) { def = d; } },
            HebrewUtils: {
                stripVowels: (s) => String(s || '').replace(/[\u0591-\u05C7]/g, ''),
                escapeHtml: (s) => String(s == null ? '' : s),
                bi: (he, en) => he + ' (' + en + ')',
                sid: (id) => String(id)
            }
        };
        sandbox.window = sandbox;
        sandbox.globalThis = sandbox;
        vm.runInNewContext(
            fs.readFileSync(path.join(__dirname, '../shared/activities/words.js'), 'utf8'),
            sandbox
        );

        const records = [];
        const markBtns = [];
        const root = {
            querySelectorAll() {
                return markBtns;
            }
        };
        const ctx = {
            ws: { id: 'my-morning', vocab: { items: [{ id: '1', word: 'קָם', meaning: 'gets up' }] } },
            wordMarks: {},
            recallMode: () => true,
            sid: (id) => String(id),
            save() {},
            review: {
                record(word, direction, signal, meta) {
                    records.push({ word, direction, signal, meta });
                }
            }
        };
        def.onClick(ctx, root, {}, {
            dataset: { action: 'mark-word', id: '1', mark: 'knew', direction: 'en_he' }
        });
        assert.equal(records.length, 1);
        assert.equal(records[0].direction, 'en_he');
        assert.equal(records[0].signal, 'pass');
        assert.equal(ctx.wordMarks['1'], 'knew');
    });
});
