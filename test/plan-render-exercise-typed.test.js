/**
 * Exercise kinds: unscramble, translate, dictation (INIT-012/SPEC-005).
 */
'use strict';

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const compare = require('../shared/ui/blocks/compare.js');
const exercise = require('../shared/ui/blocks/exercise.js');
const { createElement, makeDocument, walk, matches } = require('./helpers/fake-dom');

const ROOT = path.resolve(__dirname, '..');
const XSS = '<img src=x onerror=alert(1)>';

function patchClosest(node) {
    if (!node || node.closest) return;
    node.closest = function (selector) {
        for (let cur = this; cur; cur = cur.parentNode) {
            if (cur.tagName && matches(cur, selector)) return cur;
        }
        return null;
    };
}

function wireTree(node) {
    patchClosest(node);
    walk(node, (child) => patchClosest(child));
}

function makePlanDoc() {
    const doc = makeDocument();
    const root = createElement('div');
    doc._mount = root;
    doc.appendChild = function (child) {
        child.parentNode = root;
        root.children.push(child);
        wireTree(child);
        return child;
    };
    doc.querySelectorAll = function (sel) {
        const out = [];
        walk(root, (child) => { if (matches(child, sel)) out.push(child); });
        return out;
    };
    doc.querySelector = function (sel) { return doc.querySelectorAll(sel)[0] || null; };
    return doc;
}

function dispatchKeydown(node, key) {
    const ev = { type: 'keydown', target: node, key, preventDefault() {} };
    for (let cur = node; cur; cur = cur.parentNode) {
        (cur.listeners.keydown || []).slice().forEach((fn) => {
            ev.currentTarget = cur;
            fn(ev);
        });
    }
}

function mountSection(doc, block, options) {
    const section = createElement('section');
    section.setAttribute('data-block-type', 'exercise');
    section.setAttribute('data-exercise-kind', block.kind);
    if (options && options.lesson && options.lesson.id) {
        section.setAttribute('data-exercise-lesson', options.lesson.id);
    }
    wireTree(section);

    const live = createElement('p');
    live.setAttribute('data-exercise-live', '');
    section.appendChild(live);

    const summary = createElement('p');
    summary.setAttribute('data-exercise-summary', '');
    summary.hidden = true;
    section.appendChild(summary);

    (block.items || []).forEach((item, idx) => {
        const li = createElement('li');
        li.setAttribute('data-exercise-item', '');
        li.setAttribute('data-item-id', item.id);
        li.setAttribute('data-exercise-kind', block.kind);
        wireTree(li);

        const feedback = createElement('p');
        feedback.setAttribute('data-exercise-feedback', '');
        feedback.hidden = true;
        li.appendChild(feedback);

        if (block.kind === 'unscramble') {
            const words = item.words || [];
            const alts = item.alt || [];
            li.setAttribute('data-unscramble-words', JSON.stringify(words));
            li.setAttribute('data-unscramble-alt', JSON.stringify(alts));
            const shuffled = compare.shuffleWords(words, item.id, alts);
            const bank = createElement('div');
            bank.setAttribute('data-unscramble-bank', '');
            shuffled.forEach((word, i) => {
                const btn = createElement('button');
                btn.setAttribute('type', 'button');
                btn.setAttribute('class', 'plan-exercise-choice');
                btn.setAttribute('data-unscramble-bank-idx', String(i));
                btn.setAttribute('data-word', word);
                btn.textContent = word;
                bank.appendChild(btn);
            });
            li.appendChild(bank);
            const picked = createElement('div');
            picked.setAttribute('data-unscramble-picked', '');
            li.appendChild(picked);
            const check = createElement('button');
            check.setAttribute('type', 'button');
            check.setAttribute('class', 'plan-exercise-choice');
            check.setAttribute('data-unscramble-check', '');
            check.textContent = 'Check';
            li.appendChild(check);
            const reveal = createElement('p');
            reveal.setAttribute('data-unscramble-reveal', '');
            reveal.hidden = true;
            li.appendChild(reveal);
        }

        if (block.kind === 'translate' || block.kind === 'dictation') {
            li.setAttribute('data-answer', item.answer);
            li.setAttribute('data-accept', JSON.stringify(item.accept || []));
            if (block.kind === 'dictation') {
                const play = createElement('button');
                play.setAttribute('type', 'button');
                play.setAttribute('class', 'plan-exercise-choice');
                play.setAttribute('data-dictation-play', '');
                if (item.audioPath) play.setAttribute('data-audio-path', item.audioPath);
                li.appendChild(play);
                const skip = createElement('button');
                skip.setAttribute('type', 'button');
                skip.setAttribute('class', 'plan-exercise-choice');
                skip.setAttribute('data-dictation-skip', '');
                skip.hidden = true;
                li.appendChild(skip);
            }
            const input = createElement('input');
            input.setAttribute('type', 'text');
            input.setAttribute('data-exercise-input', '');
            input.setAttribute('lang', 'he');
            input.setAttribute('dir', 'rtl');
            li.appendChild(input);
            const submit = createElement('button');
            submit.setAttribute('type', 'button');
            submit.setAttribute('class', 'plan-exercise-choice');
            submit.setAttribute('data-exercise-submit', '');
            li.appendChild(submit);
            const reveal = createElement('p');
            reveal.setAttribute('data-exercise-answer-reveal', '');
            reveal.hidden = true;
            li.appendChild(reveal);
        }

        section.appendChild(li);
    });

    doc.appendChild(section);
    return { section, live, summary, items: section.querySelectorAll('[data-exercise-item]') };
}

function pickAllUnscramble(item) {
    const bank = [...item.querySelectorAll('[data-unscramble-bank-idx]')];
    bank.forEach((btn) => btn.click());
}

describe('ac-1 ac-2 unscramble render and shuffle', () => {
    it('accepts main and alt orders; shuffle never equals words', () => {
        const block = {
            id: 'ex-us',
            type: 'exercise',
            kind: 'unscramble',
            items: [{
                id: 'us-1',
                english: 'Hello',
                words: ['א', 'ב', 'ג'],
                alt: [['ג', 'ב', 'א']]
            }]
        };
        const html = exercise.render(block);
        assert.match(html, /data-unscramble-bank-idx/);
        assert.match(html, /data-unscramble-check/);
        const words = block.items[0].words;
        const shuffled = compare.shuffleWords(words, 'us-1', block.items[0].alt);
        assert.notDeepEqual(shuffled, words);
    });

    it('ac-2: double render is deterministic for unscramble', () => {
        const block = {
            id: 'ex-us-det',
            type: 'exercise',
            kind: 'unscramble',
            items: [{
                id: 'us-det',
                english: 'Hello',
                words: ['א', 'ב', 'ג'],
                alt: [['ג', 'ב', 'א']]
            }]
        };
        assert.equal(exercise.render(block), exercise.render(block));
    });
});

describe('ac-1 unscramble alt scoring', () => {
    function pickOrder(item, order) {
        order.forEach((w) => {
            const btn = [...item.querySelectorAll('[data-unscramble-bank-idx]')]
                .find((b) => !b.hidden && b.getAttribute('data-word') === w);
            assert.ok(btn, w);
            btn.click();
        });
    }

    function clearPicked(item) {
        [...item.querySelectorAll('[data-unscramble-picked-idx]')].forEach((b) => b.click());
    }

    it('ac-1: Unscramble accepts main order and every alt, and rejects any other order', () => {
        const words = ['א', 'ב', 'ג'];
        const alt0 = ['ג', 'ב', 'א'];
        const alt1 = ['ב', 'ג', 'א'];
        const wrong = ['א', 'ג', 'ב'];
        const block = {
            kind: 'unscramble',
            items: [
                { id: 'u-main', words, alt: [alt0, alt1] },
                { id: 'u-alt0', words: words.slice(), alt: [alt0, alt1] },
                { id: 'u-alt1', words: words.slice(), alt: [alt0, alt1] }
            ]
        };
        const doc = makePlanDoc();
        const { live, items } = mountSection(doc, block);
        exercise.bind(doc);

        pickOrder(items[0], wrong);
        items[0].querySelector('[data-unscramble-check]').click();
        assert.match(live.textContent, /Try again/);
        assert.equal(items[0].getAttribute('data-exercise-resolved'), null);

        clearPicked(items[0]);
        pickOrder(items[0], words);
        items[0].querySelector('[data-unscramble-check]').click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'correct');

        pickOrder(items[1], alt0);
        items[1].querySelector('[data-unscramble-check]').click();
        assert.equal(items[1].getAttribute('data-exercise-resolved'), 'correct');

        pickOrder(items[2], alt1);
        items[2].querySelector('[data-unscramble-check]').click();
        assert.equal(items[2].getAttribute('data-exercise-resolved'), 'correct');
    });
});

describe('ac-1 unscramble scoring', () => {
    it('retry then reveal correct sentence', () => {
        const doc = makePlanDoc();
        const words = ['א', 'ב', 'ג'];
        const block = {
            kind: 'unscramble',
            items: [{ id: 'u1', words, alt: [] }]
        };
        const { live, items } = mountSection(doc, block);
        exercise.bind(doc);
        const item = items[0];
        const wrongOrder = compare.shuffleWords(words, 'u1', []);

        function pickOrder(order) {
            order.forEach((w) => {
                const btn = [...item.querySelectorAll('[data-unscramble-bank-idx]')]
                    .find((b) => !b.hidden && b.getAttribute('data-word') === w);
                assert.ok(btn, w);
                btn.click();
            });
        }

        function clearPicked() {
            [...item.querySelectorAll('[data-unscramble-picked-idx]')].forEach((b) => b.click());
        }

        pickOrder(wrongOrder);
        item.querySelector('[data-unscramble-check]').click();
        assert.match(live.textContent, /Try again/);
        assert.equal(item.getAttribute('data-exercise-resolved'), null);

        clearPicked();
        pickOrder(wrongOrder);
        item.querySelector('[data-unscramble-check]').click();
        assert.equal(item.getAttribute('data-exercise-resolved'), 'reveal');
        assert.match(live.textContent, /Answer shown/);
    });
});

describe('ac-3 ac-4 ac-5 translate', () => {
    it('ac-4: plene in accept scores correct when typed', () => {
        const doc = makePlanDoc();
        const block = {
            kind: 'translate',
            items: [{ id: 't-plene', english: 'three', answer: 'שלש', accept: ['שלוש'] }]
        };
        const { items } = mountSection(doc, block);
        exercise.bind(doc);
        const input = items[0].querySelector('[data-exercise-input]');
        input.value = 'שלוש';
        items[0].querySelector('[data-exercise-submit]').click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'correct');
        assert.equal(compare.sameHebrew('שלוש', 'שלש', ['שלוש']), true);
    });

    it('points and accept scoring with retry reveal', () => {
        const doc = makePlanDoc();
        const block = {
            kind: 'translate',
            items: [{ id: 't1', english: 'three', answer: 'שלש', accept: ['שלש'] }]
        };
        const { live, items } = mountSection(doc, block);
        exercise.bind(doc);
        const input = items[0].querySelector('[data-exercise-input]');
        input.value = 'שלוש';
        items[0].querySelector('[data-exercise-submit]').click();
        assert.match(live.textContent, /Try again/);

        input.value = 'שלוש';
        items[0].querySelector('[data-exercise-submit]').click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'reveal');

        const doc2 = makePlanDoc();
        const m2 = mountSection(doc2, block);
        exercise.bind(doc2);
        const input2 = m2.items[0].querySelector('[data-exercise-input]');
        input2.value = 'שָׁלֹשׁ';
        m2.items[0].querySelector('[data-exercise-submit]').click();
        assert.equal(m2.items[0].getAttribute('data-exercise-resolved'), 'correct');
    });
});

describe('ac-6 ac-7 dictation audio', () => {
    let audioCalls;
    let priorAudio;

    beforeEach(() => {
        audioCalls = 0;
        priorAudio = global.Audio;
        global.Audio = class MockAudio {
            constructor(url) {
                this.url = url;
                this._listeners = {};
                audioCalls += 1;
            }
            addEventListener(type, fn) { this._listeners[type] = fn; }
            play() {
                if (String(this.url).includes('missing')) {
                    throw new Error('fail');
                }
                return Promise.resolve();
            }
        };
    });

    afterEach(() => {
        global.Audio = priorAudio;
    });

    it('does not play on bind; plays on tap; skip on load fail', () => {
        assert.equal(audioCalls, 0);
        const doc = makePlanDoc();
        const block = {
            kind: 'dictation',
            items: [{ id: 'd1', audioPath: 'audio/missing.mp3', answer: 'מים', accept: [] }]
        };
        const { live, items } = mountSection(doc, block, { lesson: { id: 'my-morning' } });
        exercise.bind(doc);
        assert.equal(audioCalls, 0);
        items[0].querySelector('[data-dictation-play]').click();
        assert.equal(audioCalls, 1);
        assert.match(live.textContent, /Audio could not load/);
        const skip = items[0].querySelector('[data-dictation-skip]');
        assert.equal(skip.hidden, false);
        skip.click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'skipped');
    });
});

describe('ac-8 XSS escaping', () => {
    it('escapes words, accept, and audioPath', () => {
        const block = {
            id: 'x',
            type: 'exercise',
            kind: 'translate',
            items: [{
                id: XSS,
                english: XSS,
                answer: XSS,
                accept: [XSS]
            }]
        };
        const html = exercise.render(block);
        assert.equal(html.includes(XSS), false);
        assert.equal(/<img\b/i.test(html), false);
        const dict = {
            id: 'd',
            type: 'exercise',
            kind: 'dictation',
            items: [{ id: 'd1', audioPath: XSS + '.mp3', answer: 'א', accept: [] }]
        };
        const htmlD = exercise.render(dict, { lesson: { id: 'lesson' } });
        assert.equal(htmlD.includes(XSS), false);
    });

    it('ac-8: unscramble word buttons escape XSS in words, alt, and english', () => {
        const block = {
            id: 'x-u',
            type: 'exercise',
            kind: 'unscramble',
            items: [{
                id: 'u-xss',
                english: XSS,
                words: [XSS, 'א'],
                alt: [[XSS, 'א']]
            }]
        };
        const html = exercise.render(block);
        assert.equal(html.includes(XSS), false);
        assert.equal(/<img\b/i.test(html), false);

        function assertButtonsHaveNoHandlerAttrs(fragment) {
            const buttons = fragment.match(/<button\b[^>]*>/gi) || [];
            buttons.forEach((tag) => {
                const withoutAttrValues = tag.replace(/="[^"]*"/g, '=""');
                assert.equal(/\son\w+\s*=/i.test(withoutAttrValues), false, tag);
            });
        }

        const itemMatch = html.match(/<li class="plan-exercise-item"[\s\S]*?<\/li>/);
        assert.ok(itemMatch, 'unscramble item container');
        assert.equal(/<img\b/i.test(itemMatch[0]), false);
        assertButtonsHaveNoHandlerAttrs(itemMatch[0]);

        const bankMatch = html.match(/data-unscramble-bank[^>]*>([\s\S]*?)<\/div>/);
        assert.ok(bankMatch, 'unscramble bank');
        assert.equal(/<img\b/i.test(bankMatch[1]), false);
        assertButtonsHaveNoHandlerAttrs(bankMatch[1]);
    });
});

describe('ac-9 keyboard unscramble', () => {
    it('completes with Enter and Space on word buttons', () => {
        const doc = makePlanDoc();
        const block = {
            kind: 'unscramble',
            items: [{ id: 'k1', words: ['yes', 'no'], alt: [] }]
        };
        const { items } = mountSection(doc, block);
        exercise.bind(doc);
        const item = items[0];
        const words = ['yes', 'no'];
        words.forEach((w) => {
            const btn = [...item.querySelectorAll('[data-unscramble-bank-idx]')]
                .find((b) => !b.hidden && b.getAttribute('data-word') === w);
            dispatchKeydown(btn, w === 'yes' ? 'Enter' : ' ');
        });
        dispatchKeydown(item.querySelector('[data-unscramble-check]'), 'Enter');
        assert.equal(item.getAttribute('data-exercise-resolved'), 'correct');
    });
});

describe('ac-10 label lang dir and summary', () => {
    it('translate field has visible label and first-try summary in live region', () => {
        const block = {
            id: 'tr',
            type: 'exercise',
            kind: 'translate',
            items: [
                { id: 'a', english: 'a', answer: 'א', accept: [] },
                { id: 'b', english: 'b', answer: 'ב', accept: [] }
            ]
        };
        const html = exercise.render(block);
        assert.match(html, /<label[^>]+for="/);
        assert.match(html, /lang="he"[^>]*data-exercise-input|data-exercise-input[^>]*lang="he"/);
        assert.match(html, /dir="rtl"/);

        const doc = makePlanDoc();
        const { live, summary, items } = mountSection(doc, block);
        exercise.bind(doc);
        items.forEach((item) => {
            const input = item.querySelector('[data-exercise-input]');
            input.value = item.getAttribute('data-answer');
            item.querySelector('[data-exercise-submit]').click();
        });
        assert.equal(summary.hidden, false);
        assert.match(summary.textContent, /right on the first try/);
        assert.match(live.textContent, /right on the first try/);
    });

    it('ac-10: dictation field has label, lang he, dir rtl, and one-word answer', () => {
        const block = {
            id: 'dict',
            type: 'exercise',
            kind: 'dictation',
            items: [{ id: 'd-one', audioPath: 'audio/ok.mp3', answer: 'מַיִם', accept: [] }]
        };
        const html = exercise.render(block, { lesson: { id: 'my-morning' } });
        assert.match(html, /<label[^>]+for="/);
        assert.match(html, /lang="he"[^>]*data-exercise-input|data-exercise-input[^>]*lang="he"/);
        assert.match(html, /dir="rtl"/);

        const doc = makePlanDoc();
        const { items } = mountSection(doc, block, { lesson: { id: 'my-morning' } });
        exercise.bind(doc);
        const input = items[0].querySelector('[data-exercise-input]');
        input.value = 'מַיִם';
        items[0].querySelector('[data-exercise-submit]').click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'correct');
    });
});

describe('verification edge cases', () => {
    it('two identical words in unscramble', () => {
        const words = ['הַיּוֹם', 'טוֹב', 'הַיּוֹם'];
        const shuffled = compare.shuffleWords(words, 'dup', []);
        assert.notDeepEqual(shuffled, words);

        const doc = makePlanDoc();
        const block = {
            kind: 'unscramble',
            items: [{ id: 'dup', words, alt: [] }]
        };
        const { items } = mountSection(doc, block);
        exercise.bind(doc);
        words.forEach((w) => {
            const btn = [...items[0].querySelectorAll('[data-unscramble-bank-idx]')]
                .find((b) => !b.hidden && b.getAttribute('data-word') === w);
            assert.ok(btn, w);
            btn.click();
        });
        items[0].querySelector('[data-unscramble-check]').click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'correct');
    });

    it('ac-10 edge: alt identical to words still shuffles and scores main order', () => {
        const words = ['א', 'ב', 'ג'];
        const alt = [words.slice()];
        const shuffled = compare.shuffleWords(words, 'edge-dup-alt', alt);
        assert.notDeepEqual(shuffled, words);

        const doc = makePlanDoc();
        const block = {
            kind: 'unscramble',
            items: [{ id: 'edge-dup-alt', words, alt }]
        };
        const { items } = mountSection(doc, block);
        exercise.bind(doc);
        words.forEach((w) => {
            const btn = [...items[0].querySelectorAll('[data-unscramble-bank-idx]')]
                .find((b) => !b.hidden && b.getAttribute('data-word') === w);
            assert.ok(btn);
            btn.click();
        });
        items[0].querySelector('[data-unscramble-check]').click();
        assert.equal(items[0].getAttribute('data-exercise-resolved'), 'correct');
        assert.equal(exercise.unscrambleCorrect(items[0], words), true);
    });

    it('resolveDictationUrl builds lesson and word clip paths', () => {
        assert.match(
            exercise.resolveDictationUrl('audio/sentences/vocab-1.mp3', 'my-morning'),
            /\/worksheets\/my-morning\/audio/
        );
        assert.match(
            exercise.resolveDictationUrl('../_audio/words/abcdefgh12345678.mp3', ''),
            /\/worksheets\/_audio\/words\//
        );
    });

    it('exercise source has no fetch or XHR', () => {
        const src = fs.readFileSync(path.join(ROOT, 'shared', 'ui', 'blocks', 'exercise.js'), 'utf8');
        assert.equal(/\bfetch\s*\(/.test(src), false);
        assert.equal(/XMLHttpRequest/.test(src), false);
    });
});
