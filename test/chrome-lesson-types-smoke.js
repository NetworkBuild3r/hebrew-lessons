#!/usr/bin/env node
/**
 * Chrome smoke: five lesson types built, published, and played (INIT-012/SPEC-009).
 *   node test/chrome-lesson-types-smoke.js
 */
'use strict';

const fs = require('fs');
const path = require('path');

const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const fx = require('./fixtures/lesson-types/fixture-lib');

const { sleep, waitFor, signInAs, setViewport } = chromeCdp;

const LEARNER_PLAN = 'five-lesson-types-chrome';
const HAND_B = 'hand-type-b-review';
const XSS = fx.XSS;

const failures = [];
function fail(where, msg) { failures.push(where + ': ' + msg); }

async function mouseClick(cdp, x, y) {
    await cdp.call('Input.dispatchMouseEvent', { type: 'mousePressed', x, y, button: 'left', clickCount: 1 });
    await cdp.call('Input.dispatchMouseEvent', { type: 'mouseReleased', x, y, button: 'left', clickCount: 1 });
}

function q(sel) { return 'document.querySelector(' + JSON.stringify(sel) + ')'; }

async function click(cdp, sel) {
    await cdp.evaluate('(() => { const el = ' + q(sel) + '; if (!el) throw new Error("missing " + ' + JSON.stringify(sel) + '); el.click(); return true; })()');
}

async function typeInto(cdp, sel, text) {
    await cdp.evaluate(`(() => {
        const el = ${q(sel)};
        if (!el) throw new Error('missing field');
        el.focus();
        el.value = ${JSON.stringify(text)};
        el.dispatchEvent(new Event('input', { bubbles: true }));
        return true;
    })()`);
}

async function exerciseFeedback(cdp, kind) {
    return cdp.evaluate(`(() => {
        const section = document.querySelector('section[data-exercise-kind="${kind}"]');
        if (!section) return '';
        const item = section.querySelector('[data-exercise-item]');
        const fb = item && item.querySelector('[data-exercise-feedback]');
        const live = section.querySelector('[data-exercise-live]');
        return [fb && !fb.hidden ? fb.textContent : '', live ? live.textContent : ''].join('|');
    })()`);
}

async function exerciseCorrect(cdp, kind) {
    const sel = 'section[data-exercise-kind="' + kind + '"] [data-exercise-item]';
    await waitFor(cdp, '!!' + q(sel), kind + ' item', 15000);
    if (kind === 'choose') {
        await cdp.evaluate(`(() => {
            const item = document.querySelector('section[data-exercise-kind="choose"] [data-exercise-item]');
            const answer = item.getAttribute('data-answer');
            const btn = [...item.querySelectorAll('[data-choice-value]')].find((b) => b.getAttribute('data-choice-value') === answer);
            if (btn) btn.click();
            return answer;
        })()`);
    } else if (kind === 'true-false') {
        await click(cdp, 'section[data-exercise-kind="true-false"] [data-tf-value="true"]');
    } else if (kind === 'unscramble') {
        await cdp.evaluate(`(() => {
            const item = document.querySelector('section[data-exercise-kind="unscramble"] [data-exercise-item]');
            const words = JSON.parse(item.getAttribute('data-unscramble-words'));
            words.forEach((w) => {
                const btn = [...item.querySelectorAll('[data-unscramble-bank-idx]')].find((b) => !b.hidden && b.getAttribute('data-word') === w);
                if (btn) btn.click();
            });
            item.querySelector('[data-unscramble-check]').click();
            return true;
        })()`);
    } else if (kind === 'translate') {
        await typeInto(cdp, sel + ' [data-exercise-input]', 'שלוש');
        await click(cdp, sel + ' [data-exercise-submit]');
    } else if (kind === 'dictation') {
        return;
    }
    await sleep(150);
    const fb = await exerciseFeedback(cdp, kind);
    if (kind !== 'dictation' && !/Correct|נכון/i.test(fb)) fail(kind, 'correct path: ' + fb);
}

async function exerciseRetryRevealCorrect(cdp, kind) {
    const sel = 'section[data-exercise-kind="' + kind + '"] [data-exercise-item]';
    await waitFor(cdp, '!!' + q(sel), kind + ' item', 15000);
    if (kind === 'choose') {
        await cdp.evaluate(`(() => {
            const item = ${q(sel)};
            const wrong = item.querySelector('[data-choice-value]:not([data-choice-value=""])');
            const buttons = [...item.querySelectorAll('[data-choice-value]')];
            const right = buttons.find((b) => b.getAttribute('data-exercise-state') === 'correct')
                || buttons[buttons.length - 1];
            wrong.click();
            return item.getAttribute('data-exercise-attempts');
        })()`);
        await sleep(150);
        await cdp.evaluate(`(() => {
            const item = ${q(sel)};
            const buttons = [...item.querySelectorAll('[data-choice-value]')];
            const wrong = buttons.find((b) => b.getAttribute('data-exercise-state') !== 'correct') || buttons[0];
            wrong.click();
            return item.getAttribute('data-exercise-resolved');
        })()`);
        await sleep(150);
    } else if (kind === 'true-false') {
        await cdp.evaluate(`(() => {
            const item = ${q(sel)};
            item.querySelector('[data-tf-value="false"]').click();
            return true;
        })()`);
        await sleep(100);
        await cdp.evaluate(`(() => {
            const item = ${q(sel)};
            item.querySelector('[data-tf-value="false"]').click();
            return item.getAttribute('data-exercise-resolved');
        })()`);
    } else if (kind === 'unscramble') {
        await cdp.evaluate(`(() => {
            const item = document.querySelector('section[data-exercise-kind="unscramble"] [data-exercise-item]');
            const words = JSON.parse(item.getAttribute('data-unscramble-words'));
            const wrong = words.slice().reverse();
            wrong.forEach((w) => {
                const btn = [...item.querySelectorAll('[data-unscramble-bank-idx]')].find((b) => !b.hidden && b.getAttribute('data-word') === w);
                if (btn) btn.click();
            });
            item.querySelector('[data-unscramble-check]').click();
            return true;
        })()`);
        await sleep(200);
        await cdp.evaluate(`(() => {
            const item = document.querySelector('section[data-exercise-kind="unscramble"] [data-exercise-item]');
            item.querySelector('[data-unscramble-check]').click();
            return item.getAttribute('data-exercise-resolved');
        })()`);
        await sleep(200);
    } else if (kind === 'translate') {
        await typeInto(cdp, sel + ' [data-exercise-input]', 'wrong');
        await click(cdp, sel + ' [data-exercise-submit]');
        await sleep(100);
        await typeInto(cdp, sel + ' [data-exercise-input]', 'wrong2');
        await click(cdp, sel + ' [data-exercise-submit]');
    } else if (kind === 'dictation') {
        await typeInto(cdp, sel + ' [data-exercise-input]', 'wrong');
        await click(cdp, sel + ' [data-exercise-submit]');
        await sleep(100);
        await typeInto(cdp, sel + ' [data-exercise-input]', 'wrong2');
        await click(cdp, sel + ' [data-exercise-submit]');
    }
    let fb = await exerciseFeedback(cdp, kind);
    if (!/Try again|Answer shown|Correct|נסו|התשובה|Audio could not load|Skipped/i.test(fb)) {
        fail(kind, 'feedback after misses: ' + fb);
    }
}

async function seedPublishedPlan(store, publish, ctx) {
    const plan = fx.combinedFiveLessonPlan();
    plan.id = LEARNER_PLAN;
    plan.revision = 1;
    for (const lesson of plan.units[0].lessons) lesson.visibility = 'everyone';
    const lex = require('../server/lexicon/service');
    const rows = fx.readJson('lexicon-ledaber.json').entries;
    assertLexicon(lex.upsert(rows, { actor: 'mcp', expectedRev: 0 }));
    store.saveWorkingCopy({
        id: plan.id,
        document: JSON.stringify(plan),
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null
    });
    const pub = publish.publishPlan(store, plan.id, { expectedRevision: 1 }, ctx);
    if (!pub.plan) throw new Error('publish failed: ' + JSON.stringify(pub.blocks || pub.error));
}

function assertLexicon(result) {
    if (!result.ok) throw new Error('lexicon upsert: ' + JSON.stringify(result));
}

async function assertEditorAc3(cdp) {
    const leak = await cdp.evaluate(`(() => {
        const scope = document.querySelector('[data-plan-new-draft]');
        if (!scope) return { jsonVisible: false, bad: false, sample: '' };
        const jsonVisible = !!scope.querySelector('[data-plan-json]:not([hidden])');
        const text = scope.innerText || '';
        const bad = /\\bId\\b|מזהה|\\bpath\\b|\\bindex\\b/.test(text);
        return { jsonVisible, bad, sample: text.slice(0, 200) };
    })()`);
    if (leak.jsonVisible) fail('ac-3', 'raw JSON toggle/textarea visible on activity form');
    if (leak.bad) fail('ac-3', 'id/path/index label on activity form: ' + leak.sample);
    const overflow = await cdp.evaluate('document.documentElement.scrollWidth - document.documentElement.clientWidth');
    if (overflow > 2) fail('ac-9 editor', 'horizontal overflow at 390px: ' + overflow);
}

async function assertEditorXss(cdp) {
    const xss = await cdp.evaluate(`(() => {
        const imgs = document.querySelectorAll('img[onerror]').length;
        const live = document.body.innerHTML.includes('<img src=x');
        const shown = document.body.innerText.includes(${JSON.stringify(XSS)})
            || [...document.querySelectorAll('textarea, input')].some((el) => el.value === ${JSON.stringify(XSS)});
        return { imgs, live, shown };
    })()`);
    if (xss.imgs > 0 || xss.live) fail('ac-8', 'XSS became live HTML in editor/preview');
    if (!xss.shown) fail('ac-8', 'XSS string not shown as text in editor');
}

async function installLearnerNetworkGuard(cdp) {
    await cdp.evaluate(`(() => {
        if (window.__netGuardInstalled) return true;
        window.__netGuardInstalled = true;
        window.__netCount = 0;
        const bump = () => { window.__netCount += 1; };
        const origFetch = window.fetch;
        if (typeof origFetch === 'function') {
            window.fetch = function() { bump(); return origFetch.apply(this, arguments); };
        }
        const OrigXHR = window.XMLHttpRequest;
        window.XMLHttpRequest = function() {
            const xhr = new OrigXHR();
            const open = xhr.open;
            xhr.open = function() { bump(); return open.apply(xhr, arguments); };
            return xhr;
        };
        if (navigator.sendBeacon) {
            const origBeacon = navigator.sendBeacon.bind(navigator);
            navigator.sendBeacon = function() { bump(); return origBeacon.apply(navigator, arguments); };
        }
        return true;
    })()`);
}

async function resetLearnerNetworkGuard(cdp) {
    await cdp.evaluate('(() => { window.__netCount = 0; return true; })()');
}

async function assertLearnerNetworkQuiet(cdp, label) {
    const n = await cdp.evaluate('window.__netCount || 0');
    if (n > 0) fail('security', label + ' sent ' + n + ' network request(s)');
}

async function buildTypeBByHand(cdp, base, store) {
    await setViewport(cdp, 390, 800);
    await signInAs(cdp, harness, base, 'admin');
    await cdp.call('Page.navigate', { url: base + '/editor/#plans' });
    await waitFor(cdp, '!!' + q('[data-plan-new-open]'), 'plan list', 20000);
    await click(cdp, '[data-plan-new-open]');
    await waitFor(cdp, '!!' + q('[data-plan-new]'), 'new plan form', 15000);
    if (await cdp.evaluate(`(() => {
        const form = document.querySelector('[data-plan-new]');
        if (!form) return false;
        const text = form.innerText || '';
        return /\\bId\\b|מזהה/.test(text);
    })()`)) {
        fail('ac-3', 'new plan form still shows an id field');
    }
    await typeInto(cdp, '[data-plan-new-title]', 'Hand review worksheet');
    await click(cdp, '[data-plan-create]');
    await waitFor(cdp, '!!' + q('[data-plan-builder]'), 'plan builder', 25000);
    const planId = await cdp.evaluate(`location.hash.match(/plan\\/([^/?]+)/)?.[1] || ''`);
    if (!planId) fail('ac-3', 'could not read new plan id');

    await waitFor(cdp, '!!document.querySelector("[data-plan-lesson]")', 'lesson open control', 15000);
    await click(cdp, '[data-plan-lesson="0"]');
    await waitFor(cdp, '!!' + q('[data-plan-lesson-screen]'), 'lesson screen', 15000);

    await cdp.evaluate(`(() => {
        const el = document.querySelector('[data-plan-section="lesson"] [data-plan-field="recipe"]');
        if (!el) throw new Error('recipe field missing');
        el.value = 'review-worksheet';
        el.dispatchEvent(new Event('change', { bubbles: true }));
        return true;
    })()`);
    await click(cdp, '[data-plan-section="lesson"] [data-plan-row="recipe"] [data-plan-save]');
    await sleep(300);

    async function pickActivity(name) {
        await cdp.evaluate(`(() => {
            const sel = document.querySelector('[data-plan-new-activity]');
            if (!sel) throw new Error('activity select missing');
            const opt = [...sel.options].find((o) => o.getAttribute('data-plan-activity-name') === ${JSON.stringify(name)});
            if (!opt) throw new Error('activity ' + ${JSON.stringify(name)});
            sel.value = opt.value;
            sel.dispatchEvent(new Event('change', { bubbles: true }));
            return true;
        })()`);
        await sleep(200);
    }

    async function saveNewBlock() {
        await click(cdp, '[data-plan-insert]');
        await sleep(500);
    }

    await pickActivity('Assignment');
    await waitFor(cdp, '!!document.querySelector("[data-plan-new-draft] textarea")', 'assignment draft', 10000);
    await assertEditorAc3(cdp);
    await typeInto(cdp, '[data-plan-new-draft] textarea', XSS);
    await sleep(200);
    await assertEditorXss(cdp);
    await typeInto(cdp, '[data-plan-new-draft] textarea', 'Write two sentences about a table.');
    await saveNewBlock();

    await pickActivity('Words');
    await waitFor(cdp, '!!document.querySelector("[data-plan-new-draft] input")', 'words draft', 10000);
    await cdp.evaluate(`(() => {
        const inputs = [...document.querySelectorAll('[data-plan-new-draft] input[type="text"], [data-plan-new-draft] textarea')];
        if (inputs[0]) { inputs[0].value = 'שֻׁלְחָן'; inputs[0].dispatchEvent(new Event('input', { bubbles: true })); }
        if (inputs[1]) { inputs[1].value = 'table'; inputs[1].dispatchEvent(new Event('input', { bubbles: true })); }
        return inputs.length;
    })()`);
    await saveNewBlock();

    await pickActivity('Exercise');
    await waitFor(cdp, '!!document.querySelector("[data-plan-new-draft]")', 'exercise draft host', 10000);
    await assertEditorAc3(cdp);
    await cdp.evaluate(`(() => {
        const sel = document.querySelector('[data-plan-new-draft] [data-plan-field="kind"]')
            || document.querySelector('[data-plan-new-draft] select');
        if (sel) { sel.value = 'choose'; sel.dispatchEvent(new Event('change', { bubbles: true })); }
        return !!sel;
    })()`);
    await sleep(200);
    const promptSel = '[data-plan-new-draft] textarea, [data-plan-new-draft] input[type="text"]';
    if (await cdp.evaluate('!!' + q(promptSel))) {
        await typeInto(cdp, promptSel, XSS);
        await sleep(200);
        await assertEditorXss(cdp);
        await typeInto(cdp, promptSel, 'בחרו את המילה הנכונה.');
    }
    await saveNewBlock();

    await cdp.call('Page.navigate', { url: base + '/editor/#plan/' + planId });
    await waitFor(cdp, '!!' + q('[data-plan-publish]'), 'plan screen publish', 15000);
    const blockers = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-issue-severity="block"]')).length`);
    if (blockers > 0) fail('ac-3', 'hand plan has ' + blockers + ' blockers before publish');
    await click(cdp, '[data-plan-publish]');
    await waitFor(cdp, '/Published/i.test((' + q('[data-plan-notice]') + '||{}).textContent||"")', 'publish', 20000);
    const working = store.loadWorkingCopy(planId);
    if (working.status !== 'published') fail('ac-3', 'hand plan status ' + working.status);
}

async function playLearnerPlan(cdp, base, width, lang) {
    await setViewport(cdp, width, width < 500 ? 800 : 900);
    await cdp.evaluate(`(() => {
        document.documentElement.lang = ${JSON.stringify(lang)};
        document.documentElement.dir = ${lang === 'he' ? '"rtl"' : '"ltr"'};
        return true;
    })()`);
    await cdp.call('Page.navigate', { url: base + '/shared/plan/?id=' + LEARNER_PLAN });
    await waitFor(cdp, q('#plan-root') + ' && ' + q('#plan-root') + '.getAttribute("data-plan-ready") === "1"', 'plan ready', 25000);
    await installLearnerNetworkGuard(cdp);

    const overflow = await cdp.evaluate('document.documentElement.scrollWidth - document.documentElement.clientWidth');
    if (overflow > 2) fail('layout ' + width + '/' + lang, 'horizontal overflow ' + overflow);

    for (const kind of ['choose', 'true-false', 'unscramble', 'translate', 'dictation']) {
        await cdp.call('Page.navigate', { url: base + '/shared/plan/?id=' + LEARNER_PLAN });
        await waitFor(cdp, q('#plan-root') + ' && ' + q('#plan-root') + '.getAttribute("data-plan-ready") === "1"', 'plan ready', 25000);
        await installLearnerNetworkGuard(cdp);
        await waitFor(cdp, '!!document.querySelector(\'section[data-exercise-kind="' + kind + '"]\')', kind, 20000);
        try {
            await resetLearnerNetworkGuard(cdp);
            await exerciseCorrect(cdp, kind);
            await assertLearnerNetworkQuiet(cdp, 'exercise correct ' + kind);
            await cdp.call('Page.navigate', { url: base + '/shared/plan/?id=' + LEARNER_PLAN });
            await waitFor(cdp, q('#plan-root') + ' && ' + q('#plan-root') + '.getAttribute("data-plan-ready") === "1"', 'plan ready', 25000);
            await installLearnerNetworkGuard(cdp);
            await waitFor(cdp, '!!document.querySelector(\'section[data-exercise-kind="' + kind + '"]\')', kind, 20000);
            await resetLearnerNetworkGuard(cdp);
            await exerciseRetryRevealCorrect(cdp, kind);
            await assertLearnerNetworkQuiet(cdp, 'exercise retry ' + kind);
        } catch (err) {
            fail('ac-5 ' + kind, err.message);
        }
    }
    const playBox = await cdp.evaluate(`(() => {
        const btn = document.querySelector('section[data-exercise-kind="dictation"] [data-dictation-play]');
        if (!btn) return null;
        const r = btn.getBoundingClientRect();
        return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
    })()`);
    if (playBox) {
        await mouseClick(cdp, playBox.x, playBox.y);
        await sleep(300);
    }
    await resetLearnerNetworkGuard(cdp);
    await exerciseRetryRevealCorrect(cdp, 'dictation');
    await assertLearnerNetworkQuiet(cdp, 'dictation retry');

    const assignSel = '[data-assignment-textarea]';
    if (await cdp.evaluate('!!' + q(assignSel))) {
        await resetLearnerNetworkGuard(cdp);
        await typeInto(cdp, assignSel, 'משפט אחד. Second sentence here.');
        await assertLearnerNetworkQuiet(cdp, 'assignment answer');
        const count = await cdp.evaluate('(' + q('[data-assignment-count]') + '||{}).textContent');
        if (!count || count.indexOf('2') === -1) fail('ac-6', 'sentence count: ' + count);
        await resetLearnerNetworkGuard(cdp);
        await typeInto(cdp, assignSel, ' עוד משפט.');
        await sleep(200);
        await assertLearnerNetworkQuiet(cdp, 'assignment extra typing');
    }

    const futureRows = await cdp.evaluate(`(() => {
        const table = document.querySelector('[data-tense="future"][data-mode="study"] table')
            || document.querySelector('[data-tense="future"] table');
        return table ? table.querySelectorAll('tr').length : 0;
    })()`);
    if (futureRows !== 9) fail('ac-7', 'future study rows ' + futureRows);
    const unchecked = await cdp.evaluate('document.body.textContent.indexOf("not yet checked") >= 0');
    if (!unchecked) fail('ac-7', 'no not yet checked label');

    const futureDrill = await cdp.evaluate(`(() => {
        const section = document.querySelector(
            '[data-block-type="conjugation-table"][data-tense="future"][data-mode="drill"]'
        );
        if (!section) return { ok: false, reason: 'missing future drill section' };
        const enabledInputs = [...section.querySelectorAll('[data-drill-input]')].filter((el) => !el.disabled).length;
        const prompt = section.querySelector('[data-drill-prompt]');
        const activeCell = prompt && prompt.getAttribute('data-drill-cell');
        const empty = section.querySelector('[data-drill-empty]');
        const emptyShown = !!(empty && !empty.hidden);
        if (enabledInputs !== 0) return { ok: false, reason: 'enabled drill inputs ' + enabledInputs };
        if (activeCell) return { ok: false, reason: 'scored drill cell ' + activeCell };
        if (!emptyShown) return { ok: false, reason: 'No checked forms message hidden' };
        return { ok: true };
    })()`);
    if (!futureDrill.ok) fail('ac-7', futureDrill.reason);

    const xssScratch = await cdp.evaluate(`(() => {
        const payload = ${JSON.stringify(XSS)};
        const R = window.HebrewPlanRender;
        if (!R || typeof R.renderBlock !== 'function') {
            return { ok: false, reason: 'HebrewPlanRender.renderBlock missing' };
        }
        const root = document.querySelector('#plan-root');
        if (!root) return { ok: false, reason: '#plan-root missing' };
        const scratch = document.createElement('div');
        scratch.setAttribute('data-ac8-scratch', '1');
        const exercise = {
            id: 'x-exercise',
            type: 'exercise',
            kind: 'choose',
            items: [{ id: 'i1', prompt: payload, choices: ['א', 'ב'], answer: 'א' }]
        };
        const assignment = {
            id: 'x-assignment',
            type: 'assignment',
            prompt: payload,
            minSentences: 1,
            homework: [payload]
        };
        scratch.innerHTML = R.renderBlock(exercise, {}) + R.renderBlock(assignment, {});
        root.appendChild(scratch);
        const literal = scratch.textContent.includes(payload);
        const liveImgs = scratch.querySelectorAll('img[onerror]').length;
        scratch.remove();
        const rootImgs = root.querySelectorAll('img[onerror]').length;
        const rootRaw = root.innerHTML.includes('<img src=x');
        if (!literal) return { ok: false, reason: 'scratch textContent missing literal XSS string' };
        if (liveImgs !== 0) return { ok: false, reason: 'scratch mounted live img[onerror]' };
        if (rootImgs !== 0 || rootRaw) return { ok: false, reason: 'XSS became live HTML in player' };
        return { ok: true };
    })()`);
    if (!xssScratch.ok) fail('ac-8', xssScratch.reason);
}

async function main() {
    const started = await harness.start({ lessons: 'fixture' });
    const base = started.baseUrl;
    const store = require('../server/plans/store');
    const publish = require('../server/plans/publish');
    const ctx = require('../server/plans/context').makeContext({
        channel: 'system', name: 'chrome-lesson-types-smoke', tool: 'smoke'
    });

    const lessonsDir = process.env.LESSONS_DIR;
    const storyDir = path.join(lessonsDir, 'type-e-story-tf', 'audio', 'sentences');
    fs.mkdirSync(storyDir, { recursive: true });
    fs.writeFileSync(path.join(storyDir, 'vocab-1.mp3'), fs.readFileSync(path.join(lessonsDir, 'public-lesson', 'clip.mp3')));
    fs.copyFileSync(
        path.join(lessonsDir, 'public-lesson', 'clip.mp3'),
        path.join(lessonsDir, 'type-e-story-tf', 'clip.mp3')
    );

    seedPublishedPlan(store, publish, ctx);

    const browser = await chromeCdp.launch({ tag: 'lesson-types-smoke', portBase: 9880, width: 1280, height: 900 });
    const cdp = browser.cdp;
    try {
        await buildTypeBByHand(cdp, base, store);
        for (const width of [390, 1280]) {
            for (const lang of ['en', 'he']) {
                await signInAs(cdp, harness, base, 'learner');
                await playLearnerPlan(cdp, base, width, lang);
            }
        }
    } finally {
        await browser.close();
        await harness.stop();
    }

    if (failures.length) {
        console.error('FAILURES:\n' + failures.map((f) => ' - ' + f).join('\n'));
        process.exit(1);
    }
    console.log('chrome-lesson-types-smoke: PASS');
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-lesson-types-smoke") || main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
