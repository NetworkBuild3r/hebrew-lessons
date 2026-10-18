/**
 * Plan exercise block (choose, true-false). Authored-by-spec: INIT-012/SPEC-004
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const compare = require('../shared/ui/blocks/compare.js');
const exercise = require('../shared/ui/blocks/exercise.js');
const ui = require('../shared/ui/plan-render.js');
const dom = require('./helpers/fake-dom');
const { createElement, makeDocument, walk, matches } = require('./helpers/fake-dom');
const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const { sleep, waitFor } = chromeCdp;

const ROOT = path.resolve(__dirname, '..');
const CSS_PATH = path.join(ROOT, 'shared', 'worksheet.css');
const CSS_START = '/* === INIT-004/SPEC-014 plan screens === */';
const CSS_END = '/* === end INIT-004/SPEC-014 plan screens === */';
const XSS = '<img src=x onerror=alert(1)>';

const CHOOSE_FIXTURE = {
    id: 'ex-choose',
    type: 'exercise',
    kind: 'choose',
    heading: 'Pick one',
    items: [{
        id: 'ch-1',
        prompt: 'Which means water?',
        choices: ['מַיִם', 'לַיְלָה', 'שׁוֹר'],
        answer: 'מַיִם',
        why: 'Mayim is water.'
    }]
};

const TF_FIXTURE = {
    id: 'ex-tf',
    type: 'exercise',
    kind: 'true-false',
    items: [{
        id: 'tf-1',
        statement: 'הַיָּם גָּדוֹל.',
        answer: true,
        why: 'Yam is sea.'
    }]
};

function cssPlanBlock() {
    const css = fs.readFileSync(CSS_PATH, 'utf8');
    const start = css.indexOf(CSS_START);
    const end = css.indexOf(CSS_END);
    return css.slice(start, end + CSS_END.length);
}

function assertCleanTags(html) {
    const tags = html.match(/<\/?[a-zA-Z][^>]*>/g) || [];
    tags.forEach((tag) => {
        const body = tag.replace(/^<\/?[a-zA-Z][\w:-]*/, '').replace(/>$/, '');
        const re = /([:@\w-]+)(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?/g;
        let match;
        while ((match = re.exec(body))) {
            assert.equal(/^on/.test(match[1].toLowerCase()), false, tag);
        }
    });
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

/** Bubble keydown like fake-dom dispatch, with key set (fake-dom has no keydown helper). */
function dispatchKeydown(node, key) {
    const ev = { type: 'keydown', target: node, key, preventDefault() {} };
    for (let cur = node; cur; cur = cur.parentNode) {
        (cur.listeners.keydown || []).slice().forEach((fn) => {
            ev.currentTarget = cur;
            fn(ev);
        });
    }
}

function mountExerciseBlock(doc, block) {
    const kind = block.kind || 'choose';
    const items = block.items || [];
    const section = createElement('section');
    section.setAttribute('data-block-type', 'exercise');
    section.setAttribute('data-exercise-kind', kind);
    wireTree(section);

    const live = createElement('p');
    live.setAttribute('data-exercise-live', '');
    section.appendChild(live);

    const progress = createElement('p');
    progress.setAttribute('data-exercise-progress', '');
    section.appendChild(progress);

    const summary = createElement('p');
    summary.setAttribute('data-exercise-summary', '');
    summary.hidden = true;
    section.appendChild(summary);

    items.forEach((item) => {
        const li = createElement('li');
        li.setAttribute('data-exercise-item', '');
        li.setAttribute('data-item-id', item.id);
        li.setAttribute('data-exercise-kind', kind);
        li.setAttribute('data-answer', kind === 'true-false' ? (item.answer ? 'true' : 'false') : item.answer);
        wireTree(li);

        const feedback = createElement('p');
        feedback.setAttribute('data-exercise-feedback', '');
        feedback.hidden = true;
        li.appendChild(feedback);

        if (item.why) {
            const why = createElement('p');
            why.setAttribute('data-exercise-why', '');
            why.hidden = true;
            why.textContent = item.why;
            li.appendChild(why);
        }

        const choicesWrap = createElement('div');
        choicesWrap.setAttribute('class', 'plan-exercise-choices');
        if (kind === 'true-false') {
            ['true', 'false'].forEach((val) => {
                const btn = createElement('button');
                btn.setAttribute('type', 'button');
                btn.setAttribute('class', 'plan-exercise-choice');
                btn.setAttribute('data-exercise-choice', '');
                btn.setAttribute('data-tf-value', val);
                btn.textContent = val;
                choicesWrap.appendChild(btn);
            });
        } else {
            (item.choices || []).forEach((choice) => {
                const btn = createElement('button');
                btn.setAttribute('type', 'button');
                btn.setAttribute('class', 'plan-exercise-choice');
                btn.setAttribute('data-exercise-choice', '');
                btn.setAttribute('data-choice-value', choice);
                btn.textContent = choice;
                choicesWrap.appendChild(btn);
            });
        }
        li.appendChild(choicesWrap);
        section.appendChild(li);
    });

    doc.appendChild(section);
    return { section, live, summary, items: section.querySelectorAll('[data-exercise-item]') };
}

function mountChooseItem(doc, item, kind) {
    const section = createElement('section');
    section.setAttribute('data-block-type', 'exercise');
    section.setAttribute('data-exercise-kind', kind || 'choose');
    wireTree(section);

    const live = createElement('p');
    live.setAttribute('data-exercise-live', '');
    section.appendChild(live);

    const progress = createElement('p');
    progress.setAttribute('data-exercise-progress', '');
    progress.textContent = '1 of 1';
    section.appendChild(progress);

    const summary = createElement('p');
    summary.setAttribute('data-exercise-summary', '');
    summary.hidden = true;
    section.appendChild(summary);

    const li = createElement('li');
    li.setAttribute('data-exercise-item', '');
    li.setAttribute('data-item-id', item.id);
    li.setAttribute('data-exercise-kind', kind || 'choose');
    li.setAttribute('data-answer', kind === 'true-false' ? (item.answer ? 'true' : 'false') : item.answer);
    wireTree(li);

    const feedback = createElement('p');
    feedback.setAttribute('data-exercise-feedback', '');
    feedback.hidden = true;
    li.appendChild(feedback);

    if (item.why) {
        const why = createElement('p');
        why.setAttribute('data-exercise-why', '');
        why.hidden = true;
        why.textContent = item.why;
        li.appendChild(why);
    }

    if (kind === 'true-false') {
        ['true', 'false'].forEach((val) => {
            const btn = createElement('button');
            btn.setAttribute('data-exercise-choice', '');
            btn.setAttribute('data-tf-value', val);
            btn.textContent = val;
            li.appendChild(btn);
        });
    } else {
        item.choices.forEach((choice) => {
            const btn = createElement('button');
            btn.setAttribute('data-exercise-choice', '');
            btn.setAttribute('data-choice-value', choice);
            btn.textContent = choice;
            li.appendChild(btn);
        });
    }

    section.appendChild(li);
    doc.appendChild(section);
    return { section, li, live };
}

describe('compare helper (INIT-012/SPEC-004)', () => {
    it('strips points, normalizes finals, and steps retry before reveal', () => {
        assert.equal(compare.stripPoints('ב\u05B9\u05BC\u05E7\u05B6\u05E8'), 'בקר');
        assert.equal(compare.normalizeHebrew('שָׁלוֹם'), compare.normalizeHebrew('שלום'));
        assert.equal(compare.normalizeHebrew('כֶּסֶף'), compare.normalizeHebrew('כסף'));
        assert.equal(compare.sameHebrew('מַיִם', 'מים'), true);
        assert.equal(compare.stepAttempt(1, false, 2), 'retry');
        assert.equal(compare.stepAttempt(2, false, 2), 'reveal');
        assert.equal(compare.stepAttempt(1, true, 2), 'correct');
    });
});

describe('ac-1 choose renders buttons only', () => {
    it('renders each choice as a button in authored order', () => {
        const block = {
            id: 'c5',
            type: 'exercise',
            kind: 'choose',
            items: [{
                id: 'i1',
                prompt: 'Same as choice',
                choices: ['alpha', 'beta', 'gamma', 'delta', 'epsilon'],
                answer: 'gamma'
            }]
        };
        const html = exercise.render(block);
        assert.match(html, /data-block-type="exercise"/);
        assert.match(html, /data-exercise-kind="choose"/);
        const buttons = html.match(/<button type="button" class="plan-exercise-choice"/g) || [];
        assert.equal(buttons.length, 5);
        const order = ['alpha', 'beta', 'gamma', 'delta', 'epsilon'];
        let from = 0;
        order.forEach((label) => {
            const at = html.indexOf('>' + label + '<', from);
            assert.ok(at >= from, label);
            from = at + 1;
        });
        assert.equal(html.includes('<input'), false);
        assert.equal(html.includes('<a '), false);
    });
});

describe('ac-2 try again then reveal (choose)', () => {
    it('shows try again on first miss and answer on second', () => {
        const doc = makePlanDoc();
        const item = CHOOSE_FIXTURE.items[0];
        const { section, li, live } = mountChooseItem(doc, item);
        exercise.bind(doc);

        const wrong = [...li.children].find((n) => n.getAttribute('data-choice-value') === 'לַיְלָה');
        const right = [...li.children].find((n) => n.getAttribute('data-choice-value') === 'מַיִם');
        wrong.click();
        assert.match(live.textContent, /Try again/);
        assert.equal(li.querySelector('[data-exercise-why]').hidden, true);
        assert.equal(li.getAttribute('data-exercise-resolved'), null);

        const wrong2 = [...li.children].find((n) => n.getAttribute('data-choice-value') === 'שׁוֹר');
        wrong2.click();
        assert.equal(li.getAttribute('data-exercise-resolved'), 'reveal');
        assert.equal(li.querySelector('[data-exercise-why]').hidden, false);
        assert.match(live.textContent, /Answer shown/);
        assert.equal(right.getAttribute('data-exercise-state'), 'answer');

        right.click();
        assert.equal(li.getAttribute('data-exercise-resolved'), 'reveal');
    });
});

describe('ac-3 true-false', () => {
    it('uses the same retry rule', () => {
        const doc = makePlanDoc();
        const { li, live } = mountChooseItem(doc, TF_FIXTURE.items[0], 'true-false');
        exercise.bind(doc);
        const falseBtn = [...li.children].find((n) => n.getAttribute('data-tf-value') === 'false');
        const trueBtn = [...li.children].find((n) => n.getAttribute('data-tf-value') === 'true');
        falseBtn.click();
        assert.match(live.textContent, /Try again/);
        falseBtn.click();
        assert.equal(li.getAttribute('data-exercise-resolved'), 'reveal');
        assert.equal(trueBtn.getAttribute('data-exercise-state'), 'answer');
    });
});

describe('ac-4 XSS in every authored field', () => {
    it('escapes markup and creates no img element', () => {
        const block = {
            id: XSS,
            type: 'exercise',
            kind: 'choose',
            heading: XSS,
            items: [{
                id: XSS,
                prompt: XSS,
                choices: [XSS, 'ok'],
                answer: XSS,
                why: XSS
            }]
        };
        const html = exercise.render(block);
        assert.equal(html.includes(XSS), false);
        assert.ok(html.includes(exercise.escapeHtml(XSS)));
        assert.ok(html.includes('data-answer="' + exercise.escapeHtml(XSS) + '"'));
        assert.ok(html.includes('data-choice-value="' + exercise.escapeHtml(XSS) + '"'));
        assert.equal(/<img\b/i.test(html), false);
        assertCleanTags(html);
    });

    it('escapes true-false authored fields including statement', () => {
        const block = {
            id: XSS,
            type: 'exercise',
            kind: 'true-false',
            heading: XSS,
            items: [{
                id: XSS,
                statement: XSS,
                answer: true,
                why: XSS
            }]
        };
        const html = exercise.render(block);
        assert.equal(html.includes(XSS), false);
        assert.ok(html.includes(exercise.escapeHtml(XSS)));
        assert.equal(/<img\b/i.test(html), false);
        assertCleanTags(html);
    });
});

describe('ac-5 buttons and focus styles', () => {
    it('renders type="button" choices with plan-exercise-choice class', () => {
        const html = exercise.render(CHOOSE_FIXTURE);
        assert.match(html, /<button type="button" class="plan-exercise-choice"/);
        const css = cssPlanBlock();
        assert.match(css, /\.plan-exercise-choice:focus-visible/);
    });

    it('completes multi-item choose and true-false using keyboard only', () => {
        const chooseBlock = {
            id: 'kb-choose',
            type: 'exercise',
            kind: 'choose',
            items: [
                { id: 'c1', prompt: 'One', choices: ['yes', 'no'], answer: 'yes' },
                { id: 'c2', prompt: 'Two', choices: ['a', 'b'], answer: 'b' }
            ]
        };
        const tfBlock = {
            id: 'kb-tf',
            type: 'exercise',
            kind: 'true-false',
            items: [
                { id: 't1', statement: 'S1', answer: true },
                { id: 't2', statement: 'S2', answer: false }
            ]
        };

        const docChoose = makePlanDoc();
        const chooseMount = mountExerciseBlock(docChoose, chooseBlock);
        exercise.bind(docChoose);
        const chooseRows = [...chooseMount.items];
        const btn1 = chooseRows[0].querySelector('[data-choice-value="yes"]');
        dispatchKeydown(btn1, 'Enter');
        const btn2 = chooseRows[1].querySelector('[data-choice-value="b"]');
        dispatchKeydown(btn2, ' ');
        assert.equal(chooseMount.summary.hidden, false);
        chooseRows.forEach((row) => {
            assert.ok(row.getAttribute('data-exercise-resolved'));
        });

        const docTf = makePlanDoc();
        const tfMount = mountExerciseBlock(docTf, tfBlock);
        exercise.bind(docTf);
        const tfRows = [...tfMount.items];
        dispatchKeydown(tfRows[0].querySelector('[data-tf-value="true"]'), 'Enter');
        dispatchKeydown(tfRows[1].querySelector('[data-tf-value="false"]'), ' ');
        assert.equal(tfMount.summary.hidden, false);
        tfRows.forEach((row) => {
            assert.ok(row.getAttribute('data-exercise-resolved'));
        });
    });
});

describe('ac-6 live region and icon words', () => {
    it('announces results with words in the live region', () => {
        const doc = makePlanDoc();
        const { li, live } = mountChooseItem(doc, CHOOSE_FIXTURE.items[0]);
        exercise.bind(doc);
        const right = [...li.children].find((n) => n.getAttribute('data-choice-value') === 'מַיִם');
        right.click();
        assert.match(live.textContent, /Correct/);
        assert.match(live.textContent, /\u2713/);
    });
});

describe('ac-7 no network calls', () => {
    it('exercise source never calls fetch, beacon, or XHR', () => {
        const src = fs.readFileSync(path.join(ROOT, 'shared', 'ui', 'blocks', 'exercise.js'), 'utf8');
        assert.equal(/\bfetch\s*\(/.test(src), false);
        assert.equal(/sendBeacon/.test(src), false);
        assert.equal(/XMLHttpRequest/.test(src), false);
    });

    it('bind does not invoke fetch when answering', () => {
        let fetchCalls = 0;
        const prior = global.fetch;
        global.fetch = () => { fetchCalls += 1; return Promise.reject(new Error('no')); };
        try {
            const doc = makePlanDoc();
            const { li } = mountChooseItem(doc, CHOOSE_FIXTURE.items[0]);
            exercise.bind(doc);
            const right = [...li.children].find((n) => n.getAttribute('data-choice-value') === 'מַיִם');
            right.click();
            assert.equal(fetchCalls, 0);
        } finally {
            if (prior) global.fetch = prior;
            else delete global.fetch;
        }
    });
});

/** Runs in Chrome: outline on the focused exercise choice after keyboard Tab. */
function chromeFocusProbe() {
    const el = document.activeElement;
    if (!el || !el.classList || !el.classList.contains('plan-exercise-choice')) {
        return { ok: false, reason: 'activeElement is not .plan-exercise-choice', tag: el && el.tagName };
    }
    const cs = getComputedStyle(el);
    const w = parseFloat(cs.outlineWidth) || 0;
    const outlineOk = cs.outlineStyle !== 'none' && w >= 1;
    return {
        ok: outlineOk,
        focusVisible: el.matches(':focus-visible'),
        outlineWidth: cs.outlineWidth,
        outlineStyle: cs.outlineStyle,
        outlineColor: cs.outlineColor
    };
}

/** Runs in Chrome: document and exercise section horizontal overflow. */
function chromeLayoutProbe() {
    const de = document.documentElement;
    const sections = Array.from(document.querySelectorAll('section.plan-exercise'));
    return {
        docScrollWidth: de.scrollWidth,
        docClientWidth: de.clientWidth,
        sections: sections.map((s) => ({
            kind: s.getAttribute('data-exercise-kind'),
            scrollWidth: s.scrollWidth,
            clientWidth: s.clientWidth
        }))
    };
}

async function pressKeyOnFocused(cdp, key) {
    await cdp.evaluate(`(() => {
        const el = document.activeElement;
        if (!el) throw new Error('no activeElement for key ' + ${JSON.stringify(key)});
        el.dispatchEvent(new KeyboardEvent('keydown', {
            key: ${JSON.stringify(key)},
            code: ${key === ' ' ? "'Space'" : key === 'Enter' ? "'Enter'" : JSON.stringify(key)},
            bubbles: true,
            cancelable: true
        }));
        return true;
    })()`);
}

describe('ac-8 layout at 360px', () => {
    it('plan-exercise rules avoid horizontal overflow and raw hex', () => {
        const css = cssPlanBlock();
        const slice = css.slice(css.indexOf('/* INIT-012/SPEC-004'));
        assert.match(slice, /\.plan-exercise[\s\S]*overflow-x:\s*hidden/);
        assert.match(slice, /max-width:\s*100%/);
        assert.equal(/#[0-9a-fA-F]{3,8}/.test(slice.replace(/\/\*[\s\S]*?\*\//g, '')), false);
    });

    it('headless Chrome at 360×640: no overflow, focus ring, keyboard, live region', { timeout: 90000, skip: chromeCdp.chromeSkip() }, async () => {
        const CHROME_CHOOSE = {
            id: 'chrome-choose',
            type: 'exercise',
            kind: 'choose',
            items: [{ id: 'c1', prompt: 'Pick', choices: ['yes', 'no'], answer: 'yes', why: 'Yes.' }]
        };
        const CHROME_TF = {
            id: 'chrome-tf',
            type: 'exercise',
            kind: 'true-false',
            items: [{ id: 't1', statement: 'True?', answer: true, why: 'It is.' }]
        };

        let browser;
        let layoutBefore;
        let layoutAfter;
        let focusProbe;
        let liveAfterChoose;
        let liveAfterTf;

        try {
            await harness.start({ lessons: 'real' });
            const BASE = harness.baseUrl;
            browser = await chromeCdp.launch({ tag: 'exercise-360', portBase: 9850, width: 360, height: 640 });
            const cdp = browser.cdp;
            await chromeCdp.signInAs(cdp, harness, BASE, 'learner');
            await cdp.call('Page.navigate', { url: BASE + '/shared/plan/index.html' });
            await waitFor(
                cdp,
                'typeof window.HebrewPlanRender !== "undefined" && typeof window.HebrewBlockExercise !== "undefined"',
                'plan render scripts',
                15000
            );
            await waitFor(cdp, 'document.readyState === "complete"', 'document complete', 15000);
            await chromeCdp.setViewport(cdp, 360, 640);
            await sleep(200);

            const blocksJson = JSON.stringify([CHROME_CHOOSE, CHROME_TF]);
            const mounted = await cdp.evaluate(`(() => {
                const blocks = ${blocksJson};
                const R = window.HebrewPlanRender;
                document.getElementById('plan-root').innerHTML = blocks.map((b) => R.renderBlock(b)).join('');
                R.bind(document);
                return document.querySelectorAll('section.plan-exercise').length;
            })()`);
            assert.equal(mounted, 2);

            layoutBefore = await cdp.evaluate('(' + chromeLayoutProbe + ')()');
            assert.ok(layoutBefore.docScrollWidth <= layoutBefore.docClientWidth,
                'document overflow: scrollWidth ' + layoutBefore.docScrollWidth + ' > clientWidth ' + layoutBefore.docClientWidth);
            layoutBefore.sections.forEach((s) => {
                assert.ok(s.scrollWidth <= s.clientWidth,
                    s.kind + ' section overflow: ' + s.scrollWidth + ' > ' + s.clientWidth);
            });
            assert.ok(layoutBefore.docClientWidth <= 360, 'expected viewport clientWidth <= 360, got ' + layoutBefore.docClientWidth);

            await cdp.evaluate(`(() => {
                document.body.setAttribute('tabindex', '-1');
                document.body.focus();
                return true;
            })()`);
            let tabStops = 0;
            for (let i = 0; i < 20; i += 1) {
                await chromeCdp.pressTab(cdp, false);
                await sleep(50);
                tabStops += 1;
                const onChoice = await cdp.evaluate(`document.activeElement && document.activeElement.classList.contains('plan-exercise-choice')`);
                if (onChoice) break;
            }
            focusProbe = await cdp.evaluate('(' + chromeFocusProbe + ')()');
            assert.ok(focusProbe.ok, 'focus ring on .plan-exercise-choice: ' + JSON.stringify(focusProbe));

            const chooseYes = await cdp.evaluate(`(() => {
                const btn = document.querySelector('section[data-exercise-kind="choose"] [data-choice-value="yes"]');
                btn.focus();
                return btn === document.activeElement;
            })()`);
            assert.equal(chooseYes, true);
            await pressKeyOnFocused(cdp, 'Enter');
            await sleep(150);
            liveAfterChoose = await cdp.evaluate(`document.querySelector('section[data-exercise-kind="choose"] [data-exercise-live]').textContent`);
            assert.match(liveAfterChoose, /Correct/);

            const tfTrue = await cdp.evaluate(`(() => {
                const btn = document.querySelector('section[data-exercise-kind="true-false"] [data-tf-value="true"]');
                btn.focus();
                return btn === document.activeElement;
            })()`);
            assert.equal(tfTrue, true);
            await pressKeyOnFocused(cdp, ' ');
            await sleep(150);
            liveAfterTf = await cdp.evaluate(`document.querySelector('section[data-exercise-kind="true-false"] [data-exercise-live]').textContent`);
            assert.match(liveAfterTf, /Correct/);

            layoutAfter = await cdp.evaluate('(' + chromeLayoutProbe + ')()');
            assert.ok(layoutAfter.docScrollWidth <= 360,
                'after keyboard: document scrollWidth ' + layoutAfter.docScrollWidth + ' > 360');
            layoutAfter.sections.forEach((s) => {
                assert.ok(s.scrollWidth <= 360, s.kind + ' scrollWidth ' + s.scrollWidth + ' > 360');
            });
        } finally {
            if (browser) await browser.close();
            await harness.stop();
        }
    });
});

describe('verification edge cases', () => {
    it('item with no why keeps why hidden after correct', () => {
        const block = {
            id: 'no-why',
            type: 'exercise',
            kind: 'choose',
            items: [{ id: 'a', prompt: 'Q', choices: ['x', 'y'], answer: 'x' }]
        };
        const html = exercise.render(block);
        assert.equal(html.includes('data-exercise-why'), false);
        const doc = makePlanDoc();
        const { li } = mountChooseItem(doc, block.items[0]);
        exercise.bind(doc);
        const x = [...li.children].find((n) => n.getAttribute('data-choice-value') === 'x');
        x.click();
        assert.equal(li.querySelector('[data-exercise-why]'), null);
    });

    it('choice text may match prompt text', () => {
        const block = {
            id: 'dup',
            type: 'exercise',
            kind: 'choose',
            items: [{ id: 'a', prompt: 'repeat', choices: ['repeat', 'other'], answer: 'repeat' }]
        };
        const html = exercise.render(block);
        assert.equal((html.match(/repeat/g) || []).length >= 2, true);
    });

    it('lesson with only exercise blocks renders through plan-render', () => {
        const plan = {
            title: 'Only exercises',
            units: [{
                id: 'u1',
                lessons: [{
                    id: 'l1',
                    title: 'Lesson',
                    blocks: [CHOOSE_FIXTURE, TF_FIXTURE]
                }]
            }]
        };
        const html = ui.renderBlock(CHOOSE_FIXTURE) + ui.renderBlock(TF_FIXTURE);
        assert.match(html, /data-exercise-kind="choose"/);
        assert.match(html, /data-exercise-kind="true-false"/);
    });

    it('very long English why is capped and escaped', () => {
        const long = 'W'.repeat(5000);
        const block = {
            id: 'long',
            type: 'exercise',
            kind: 'choose',
            items: [{ id: 'a', prompt: 'Q', choices: ['a', 'b'], answer: 'a', why: long }]
        };
        const html = exercise.render(block);
        assert.ok(html.includes('W'.repeat(100)));
        assert.equal(html.includes(long), false);
        assert.ok(html.length < long.length + 500);
    });

    it('registry wires exercise render and bind', () => {
        assert.ok(ui.blocks.exercise);
        assert.equal(typeof ui.blocks.exercise.render, 'function');
        assert.equal(typeof ui.blocks.exercise.bind, 'function');
        const html = ui.renderBlock(CHOOSE_FIXTURE);
        assert.match(html, /plan-exercise/);
    });
});
