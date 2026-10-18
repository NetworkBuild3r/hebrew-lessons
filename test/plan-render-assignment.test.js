/**
 * Plan assignment block and future conjugation tables.
 * Authored-by-spec: INIT-012/SPEC-006
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const assignment = require('../shared/ui/blocks/assignment.js');
const table = require('../shared/ui/blocks/conjugation-table.js');
const ui = require('../shared/ui/plan-render.js');
const { createElement, makeDocument, walk, matches } = require('./helpers/fake-dom');
const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const { sleep, waitFor } = chromeCdp;

const ROOT = path.resolve(__dirname, '..');
const CSS_PATH = path.join(ROOT, 'shared', 'worksheet.css');
const CSS_START = '/* === INIT-004/SPEC-014 plan screens === */';
const CSS_END = '/* === end INIT-004/SPEC-014 plan screens === */';
const EXAMPLES = JSON.parse(fs.readFileSync(
    path.join(ROOT, 'docs', 'decisions', 'ADR-0014-assets', 'exercise-examples.json'), 'utf8'
));
const ASSIGNMENT = EXAMPLES.blocks.find((b) => b.type === 'assignment');
const FUTURE_TABLE = EXAMPLES.blocks.find((b) => b.type === 'conjugation-table' && b.tense === 'future');
const XSS = '<img src=x onerror=alert(1)>';
const CONJ_SRC = fs.readFileSync(path.join(ROOT, 'shared', 'ui', 'blocks', 'conjugation-table.js'), 'utf8');
const ASSIGN_SRC = fs.readFileSync(path.join(ROOT, 'shared', 'ui', 'blocks', 'assignment.js'), 'utf8');

const PRESENT_FIXTURE = {
    id: 'present-golden',
    type: 'conjugation-table',
    lemmaKey: 'לַחְשֹׁב',
    tense: 'present',
    mode: 'study',
    cells: [
        { cell: 'fp', formKey: 'חוֹשְׁבוֹת' },
        { cell: 'mp', formKey: 'חוֹשְׁבִים' },
        { cell: 'fs', formKey: 'חוֹשֶׁבֶת' },
        { cell: 'ms', formKey: 'חוֹשֵׁב' }
    ]
};

const PAST_FIXTURE = {
    id: 'past-golden',
    type: 'conjugation-table',
    lemmaKey: 'לַחְשֹׁב',
    tense: 'past',
    mode: 'study',
    cells: [
        { cell: '1s', formKey: 'חָשַׁבְתִּי' },
        { cell: '2ms', formKey: 'חָשַׁבְתָּ' },
        { cell: '2fs', formKey: 'חָשַׁבְתְּ' },
        { cell: '3ms', formKey: 'חָשַׁב' },
        { cell: '3fs', formKey: 'חָשְׁבָה' },
        { cell: '1p', formKey: 'חָשַׁבְנוּ' },
        { cell: '2mp', formKey: 'חֲשַׁבְתֶּם' },
        { cell: '2fp', formKey: 'חֲשַׁבְתֶּן' },
        { cell: '3p', formKey: 'חָשְׁבוּ' }
    ]
};

function cssPlanBlock() {
    const css = fs.readFileSync(CSS_PATH, 'utf8');
    const start = css.indexOf(CSS_START);
    const end = css.indexOf(CSS_END);
    return css.slice(start, end + CSS_END.length);
}

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

function mountAssignment(doc, block, options) {
    const min = Math.max(1, Math.min(12, Number(block.minSentences) || 1));
    const section = createElement('section');
    section.setAttribute('data-block-type', 'assignment');
    section.setAttribute('data-block-id', block.id || '');
    section.setAttribute('data-assignment-min', String(min));
    section.setAttribute('class', 'plan-assignment');
    wireTree(section);

    const textarea = createElement('textarea');
    textarea.setAttribute('data-assignment-textarea', '');
    textarea.setAttribute('lang', 'he');
    textarea.setAttribute('dir', 'rtl');
    textarea.matches = function (sel) { return matches(this, sel); };
    section.appendChild(textarea);

    const count = createElement('p');
    count.setAttribute('data-assignment-count', '');
    count.textContent = '0 / ' + min;
    section.appendChild(count);

    const done = createElement('p');
    done.setAttribute('data-assignment-done', '');
    done.hidden = true;
    section.appendChild(done);

    (block.useWords || []).forEach((word) => {
        if (typeof word !== 'string' || !word.trim()) return;
        const btn = createElement('button');
        btn.setAttribute('type', 'button');
        btn.setAttribute('class', 'plan-assignment-word');
        btn.setAttribute('data-assignment-word', word.trim());
        const clip = assignment.clipForWord(word.trim(), (options && options.audio) || null);
        if (clip) btn.setAttribute('data-assignment-clip', clip);
        btn.textContent = word.trim();
        section.appendChild(btn);
    });

    (block.homework || []).forEach((line, index) => {
        const box = createElement('input');
        box.setAttribute('type', 'checkbox');
        box.setAttribute('data-assignment-hw', String(index));
        section.appendChild(box);
    });

    doc.appendChild(section);
    return section;
}

function dispatchKeydown(node, key) {
    const ev = { type: 'keydown', target: node, key, preventDefault() {} };
    for (let cur = node; cur; cur = cur.parentNode) {
        (cur.listeners.keydown || []).slice().forEach((fn) => {
            ev.currentTarget = cur;
            fn(ev);
        });
    }
    return ev;
}

function toggleCheckboxWithSpace(box) {
    box.focus();
    const before = box.checked;
    dispatchKeydown(box, ' ');
    if (box.getAttribute('type') === 'checkbox') box.checked = !before;
}

function dispatchInput(textarea, value) {
    textarea.value = value;
    const ev = { type: 'input', target: textarea, preventDefault() {} };
    for (let cur = textarea; cur; cur = cur.parentNode) {
        (cur.listeners.input || []).slice().forEach((fn) => {
            ev.currentTarget = cur;
            fn(ev);
        });
    }
}

function formsForFuture(cells, unverifiedKeys) {
    const forms = Object.create(null);
    cells.forEach((entry) => {
        forms[entry.formKey] = {
            key: entry.formKey,
            verified: !unverifiedKeys.has(entry.formKey),
            root: 'ד.ב.ר',
            accept: []
        };
    });
    return forms;
}

describe('countSentences (aud-1)', () => {
    it('counts segments and ignores empty punctuation runs', () => {
        assert.equal(assignment.countSentences(''), 0);
        assert.equal(assignment.countSentences('שלום עולם'), 1);
        assert.equal(assignment.countSentences('שלום. מה שלומך?'), 2);
        assert.equal(assignment.countSentences('שורה א\nשורה ב\n'), 2);
        assert.equal(assignment.countSentences('...'), 0);
    });
});

describe('ac-1 assignment render', () => {
    it('shows prompt, minimum line, word chips, and labeled textarea', () => {
        const html = assignment.render(ASSIGNMENT, {
            audio: { 'סוף': '../_audio/words/abcdef12345678.mp3' }
        });
        assert.match(html, /plan-assignment-prompt/);
        assert.match(html, /data-assignment-min="6"/);
        assert.match(html, /Write at least 6 sentences/);
        assert.match(html, /plan-assignment-word/);
        assert.match(html, /data-assignment-textarea/);
        assert.match(html, /for="assignment-text-/);
        assert.match(html, /lang="he" dir="rtl"/);
        assert.match(html, /data-assignment-clip="abcdef12345678"/);
    });

    it('minSentences 1 and omits words and homework when absent', () => {
        const minimal = {
            id: 'min',
            type: 'assignment',
            prompt: 'One sentence.',
            minSentences: 1
        };
        const html = assignment.render(minimal);
        assert.match(html, /data-assignment-min="1"/);
        assert.equal(html.includes('data-assignment-words'), false);
        assert.equal(html.includes('data-assignment-hw'), false);
    });
});

describe('ac-2 live count and Done nudge', () => {
    it('updates count and toggles Done when crossing the minimum', () => {
        const doc = makePlanDoc();
        mountAssignment(doc, { id: 'a', type: 'assignment', prompt: 'Go', minSentences: 2 });
        assignment.bind(doc);
        const section = doc.querySelector('[data-block-type="assignment"]');
        const textarea = doc.querySelector('[data-assignment-textarea]');
        const done = doc.querySelector('[data-assignment-done]');
        const count = doc.querySelector('[data-assignment-count]');
        assert.equal(done.hidden, true);
        dispatchInput(textarea, 'א. ב.');
        assert.equal(count.textContent, '2 / 2');
        assert.equal(done.hidden, false);
        dispatchInput(textarea, 'א.');
        assert.equal(done.hidden, true);
    });
});

describe('ac-3 word chip audio', () => {
    it('calls playWord with clip and word, or quietly when no playWord', () => {
        let calls = [];
        const doc = makePlanDoc();
        mountAssignment(doc, {
            id: 'w',
            type: 'assignment',
            prompt: 'Use words',
            minSentences: 1,
            useWords: ['מים', 'שלום']
        }, {
            audio: { 'מים': '../_audio/words/clip123456789012.mp3' }
        });
        const playWordSpy = (clip, word) => {
            calls.push({ clip, word });
            return Promise.resolve(true);
        };
        assignment.bind(doc, { playWord: playWordSpy });
        const chips = doc.querySelectorAll('[data-assignment-word]');
        chips[0].click();
        assert.deepEqual(calls[0], { clip: 'clip123456789012', word: 'מים' });
        const beforeQuiet = calls.length;
        chips[1].click();
        assert.equal(calls.length, beforeQuiet);

        calls = [];
        const doc2 = makePlanDoc();
        mountAssignment(doc2, {
            id: 'w2',
            type: 'assignment',
            prompt: 'P',
            minSentences: 1,
            useWords: ['שלום']
        });
        assignment.bind(doc2, {});
        assert.doesNotThrow(() => doc2.querySelector('[data-assignment-word]').click());
        assert.equal(calls.length, 0);
    });
});

describe('ac-4 homework and ac-5 XSS', () => {
    it('homework checkboxes toggle without storage or network', () => {
        const doc = makePlanDoc();
        mountAssignment(doc, ASSIGNMENT);
        assignment.bind(doc);
        const box = doc.querySelector('[data-assignment-hw="0"]');
        assert.equal(box.checked, false);
        box.checked = true;
        assert.equal(box.checked, true);
        toggleCheckboxWithSpace(box);
        assert.equal(box.checked, false);
        toggleCheckboxWithSpace(box);
        assert.equal(box.checked, true);
        assert.equal(/\blocalStorage\b/.test(ASSIGN_SRC), false);
        assert.equal(/\bsessionStorage\b/.test(ASSIGN_SRC), false);
        assert.equal(/\bfetch\s*\(/.test(ASSIGN_SRC), false);
    });

    it('escapes markup in prompt, useWords, and homework', () => {
        const block = {
            id: 'x',
            type: 'assignment',
            prompt: XSS,
            minSentences: 1,
            useWords: [XSS],
            homework: [XSS]
        };
        const html = assignment.render(block);
        assert.equal(html.includes(XSS), false);
        assert.ok(html.includes('&lt;img'));
        assert.equal(/<img\b/i.test(html), false);
        const tags = html.match(/<\/?[a-zA-Z][^>]*>/g) || [];
        tags.forEach((tag) => {
            const body = tag.replace(/^<\/?[a-zA-Z][\w:-]*/, '').replace(/>$/, '');
            const re = /([:@\w-]+)(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?/g;
            let match;
            while ((match = re.exec(body))) {
                assert.equal(/^on/.test(match[1].toLowerCase()), false, tag);
            }
        });
    });
});

describe('ac-5 learner text never becomes HTML', () => {
    it('bind reads textarea value as text only', () => {
        const doc = makePlanDoc();
        mountAssignment(doc, { id: 't', type: 'assignment', prompt: 'P', minSentences: 1 });
        assignment.bind(doc);
        const textarea = doc.querySelector('[data-assignment-textarea]');
        dispatchInput(textarea, XSS);
        const count = doc.querySelector('[data-assignment-count]');
        assert.equal(count.textContent.includes('<img'), false);
        assert.equal(textarea.value, XSS);
    });
});

describe('ac-6 ac-7 ac-8 future conjugation table', () => {
    const threeCells = FUTURE_TABLE.cells.slice(0, 3);

    it('renders nine past-order rows for future study tables', () => {
        const html = table.render(FUTURE_TABLE, { forms: formsForFuture(FUTURE_TABLE.cells, new Set()) });
        assert.match(html, /data-tense="future"/);
        assert.match(html, /עתיד/);
        assert.equal((html.match(/<tr>/g) || []).length, 9);
        ['אני', 'אתה', 'את', 'הוא', 'היא', 'אנחנו', 'אתם', 'אתן', 'הם/הן'].forEach((label) => {
            assert.ok(html.includes(label), label);
        });
        const partial = table.render(Object.assign({}, FUTURE_TABLE, { cells: threeCells }), {
            forms: formsForFuture(threeCells, new Set())
        });
        assert.equal((partial.match(/<tr>/g) || []).length, 9);
    });

    it('drill queues verified future forms only', () => {
        const drill = Object.assign({}, FUTURE_TABLE, { mode: 'drill', id: 'f-drill' });
        const cells = threeCells;
        const unverified = new Set([cells[2].formKey]);
        const forms = formsForFuture(cells, unverified);
        const queue = table.drillQueue([drill], forms, () => 0.5);
        assert.equal(queue.some((item) => item.formKey === cells[2].formKey), false);
        assert.ok(queue.length >= 2);
    });

    it('present and past study HTML unchanged from golden fixtures', () => {
        const forms = Object.create(null);
        PRESENT_FIXTURE.cells.forEach((c) => {
            forms[c.formKey] = { key: c.formKey, verified: true, root: 'ח.ש.ב', accept: [] };
        });
        PAST_FIXTURE.cells.forEach((c) => {
            forms[c.formKey] = { key: c.formKey, verified: true, root: 'ח.ש.ב', accept: [] };
        });
        const presentHtml = table.render(PRESENT_FIXTURE, { forms });
        const pastHtml = table.render(PAST_FIXTURE, { forms });
        assert.match(presentHtml, /data-tense="present"/);
        assert.match(pastHtml, /data-tense="past"/);
        assert.equal(presentHtml.includes('עתיד'), false);
        assert.equal(pastHtml.includes('עתיד'), false);
        assert.equal(/data-tense="future"/.test(presentHtml), false);
        assert.equal(/data-tense="future"/.test(pastHtml), false);
    });
});

describe('ac-9 registry and plan-render wiring', () => {
    it('registry exposes assignment render and bind', () => {
        assert.ok(ui.blocks.assignment);
        assert.equal(typeof ui.blocks.assignment.render, 'function');
        const html = ui.renderBlock(ASSIGNMENT);
        assert.match(html, /plan-assignment/);
    });
});

describe('assignment CSS', () => {
    it('uses design tokens only in the assignment slice', () => {
        const css = cssPlanBlock();
        const slice = css.slice(css.indexOf('/* INIT-012/SPEC-006'));
        assert.match(slice, /\.plan-assignment-textarea/);
        assert.equal(/#[0-9a-fA-F]{3,8}/.test(slice.replace(/\/\*[\s\S]*?\*\//g, '')), false);
    });
});

describe('network and storage guards', () => {
    it('bind never calls fetch or localStorage', () => {
        let fetchCalls = 0;
        let storageCalls = 0;
        const priorFetch = global.fetch;
        const priorSet = global.localStorage;
        global.fetch = () => { fetchCalls += 1; return Promise.reject(new Error('no')); };
        global.localStorage = { setItem() { storageCalls += 1; } };
        try {
            const doc = makePlanDoc();
            mountAssignment(doc, ASSIGNMENT);
            assignment.bind(doc);
            const textarea = doc.querySelector('[data-assignment-textarea]');
            dispatchInput(textarea, 'משפט.');
            doc.querySelector('[data-assignment-hw="0"]').click();
            assert.equal(fetchCalls, 0);
            assert.equal(storageCalls, 0);
        } finally {
            if (priorFetch) global.fetch = priorFetch;
            else delete global.fetch;
            if (priorSet) global.localStorage = priorSet;
            else delete global.localStorage;
        }
    });
});

describe('verification edge cases', () => {
    it('Hebrew sentence without punctuation counts as one', () => {
        assert.equal(assignment.countSentences('שלום עולם'), 1);
    });

    it('pasted block counts multiple lines', () => {
        const pasted = 'שורה א\nשורה ב\nשורה ג.';
        assert.equal(assignment.countSentences(pasted), 3);
    });
});

describe('ac-9 resize keeps textarea text in Chrome', () => {
    it('headless Chrome: text survives 360→1280 resize and clears on reload', { timeout: 120000, skip: chromeCdp.chromeSkip() }, async () => {
        const CHROME_BLOCK = {
            id: 'chrome-as',
            type: 'assignment',
            prompt: 'Write two sentences.',
            minSentences: 2,
            useWords: ['מים'],
            homework: ['Read aloud.']
        };

        let browser;
        try {
            await harness.start({ lessons: 'real' });
            const BASE = harness.baseUrl;
            browser = await chromeCdp.launch({ tag: 'assignment-resize', portBase: 9860, width: 360, height: 640 });
            const cdp = browser.cdp;
            await chromeCdp.signInAs(cdp, harness, BASE, 'learner');
            await cdp.call('Page.navigate', { url: BASE + '/shared/plan/index.html' });
            await waitFor(
                cdp,
                'typeof window.HebrewPlanRender !== "undefined" && typeof window.HebrewBlockAssignment !== "undefined"',
                'plan assignment scripts',
                15000
            );
            await waitFor(cdp, 'document.readyState === "complete"', 'document complete', 15000);

            const blockJson = JSON.stringify(CHROME_BLOCK);
            await cdp.evaluate(`(() => {
                const block = ${blockJson};
                const R = window.HebrewPlanRender;
                document.getElementById('plan-root').innerHTML = R.renderBlock(block);
                R.bind(document);
                return true;
            })()`);

            await cdp.evaluate(`(() => {
                const ta = document.querySelector('[data-assignment-textarea]');
                ta.value = 'א. ב.';
                ta.dispatchEvent(new Event('input', { bubbles: true }));
                const hw = document.querySelector('[data-assignment-hw="0"]');
                if (hw) hw.checked = true;
                return document.querySelector('[data-assignment-count]').textContent;
            })()`);
            const count360 = await cdp.evaluate(`document.querySelector('[data-assignment-count]').textContent`);
            assert.equal(count360, '2 / 2');

            await chromeCdp.setViewport(cdp, 1280, 800);
            await sleep(200);
            const textAfterResize = await cdp.evaluate(`document.querySelector('[data-assignment-textarea]').value`);
            assert.equal(textAfterResize, 'א. ב.');

            await cdp.call('Page.reload', { ignoreCache: true });
            await waitFor(cdp, 'document.readyState === "complete"', 'reload complete', 15000);
            const afterReload = await cdp.evaluate(`(() => {
                const ta = document.querySelector('[data-assignment-textarea]');
                const hw = document.querySelector('[data-assignment-hw="0"]');
                return {
                    text: ta ? ta.value : '',
                    hwChecked: hw ? hw.checked : false
                };
            })()`);
            assert.equal(afterReload.text, '');
            assert.equal(afterReload.hwChecked, false);
        } finally {
            if (browser) await browser.close();
            await harness.stop();
        }
    });
});

describe('conjugation source invariants', () => {
    it('still refuses unverified drill forms and documents future support', () => {
        assert.match(CONJ_SRC, /Never drill an unverified form/);
        assert.match(CONJ_SRC, /future/);
    });
});
