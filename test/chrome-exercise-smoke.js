#!/usr/bin/env node
/**
 * Headless Chrome smoke: all five exercise kinds + dictation skip on audio fail.
 * Authored-by-spec: INIT-012/SPEC-005
 *
 *   node test/chrome-exercise-smoke.js
 */
'use strict';

const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const { sleep, waitFor } = chromeCdp;

const BLOCKS = [
    {
        id: 'sm-choose',
        type: 'exercise',
        kind: 'choose',
        items: [{ id: 'c1', prompt: 'Pick yes', choices: ['yes', 'no'], answer: 'yes' }]
    },
    {
        id: 'sm-tf',
        type: 'exercise',
        kind: 'true-false',
        items: [{ id: 't1', statement: 'True?', answer: true }]
    },
    {
        id: 'sm-us',
        type: 'exercise',
        kind: 'unscramble',
        items: [{ id: 'u1', english: 'AB', words: ['A', 'B'], alt: [] }]
    },
    {
        id: 'sm-tr',
        type: 'exercise',
        kind: 'translate',
        items: [{ id: 'r1', english: 'alef', answer: 'א', accept: [] }]
    },
    {
        id: 'sm-di',
        type: 'exercise',
        kind: 'dictation',
        items: [{
            id: 'd1',
            audioPath: 'clip.mp3',
            answer: 'ב',
            accept: []
        }]
    },
    {
        id: 'sm-di-fail',
        type: 'exercise',
        kind: 'dictation',
        items: [{
            id: 'd2',
            audioPath: 'audio/missing-file.mp3',
            answer: 'ג',
            accept: []
        }]
    }
];

async function mouseClick(cdp, x, y) {
    await cdp.call('Input.dispatchMouseEvent', {
        type: 'mousePressed', x, y, button: 'left', clickCount: 1
    });
    await cdp.call('Input.dispatchMouseEvent', {
        type: 'mouseReleased', x, y, button: 'left', clickCount: 1
    });
}

async function main() {
    await harness.start({ lessons: 'real' });
    const BASE = harness.baseUrl;
    const browser = await chromeCdp.launch({ tag: 'exercise-smoke', portBase: 9860, width: 800, height: 900 });
    const cdp = browser.cdp;

    try {
        await chromeCdp.signInAs(cdp, harness, BASE, 'learner');
        await cdp.call('Page.navigate', { url: BASE + '/shared/plan/index.html' });
        await waitFor(
            cdp,
            'typeof window.HebrewPlanRender !== "undefined" && typeof window.HebrewBlockExercise !== "undefined"',
            'plan scripts',
            15000
        );

        const lesson = { id: 'my-morning' };
        const blocksJson = JSON.stringify(BLOCKS);
        await cdp.evaluate(`(() => {
            const blocks = ${blocksJson};
            const lesson = ${JSON.stringify(lesson)};
            const R = window.HebrewPlanRender;
            document.getElementById('plan-root').innerHTML = blocks.map((b) => R.renderBlock(b, { lesson })).join('');
            R.bind(document);
            return document.querySelectorAll('section.plan-exercise').length;
        })()`);

        const count = await cdp.evaluate('document.querySelectorAll("section.plan-exercise").length');
        if (count !== BLOCKS.length) {
            throw new Error('expected ' + BLOCKS.length + ' exercise sections, got ' + count);
        }

        await cdp.evaluate(`document.querySelector('section[data-exercise-kind="choose"] [data-choice-value="yes"]').click()`);
        await sleep(100);

        await cdp.evaluate(`document.querySelector('section[data-exercise-kind="true-false"] [data-tf-value="true"]').click()`);
        await sleep(100);

        await cdp.evaluate(`(() => {
            const item = document.querySelector('section[data-exercise-kind="unscramble"] [data-exercise-item]');
            const words = JSON.parse(item.getAttribute('data-unscramble-words'));
            words.forEach((w) => {
                const btn = [...item.querySelectorAll('[data-unscramble-bank-idx]')].find((b) => !b.hidden && b.getAttribute('data-word') === w);
                if (btn) btn.click();
            });
            item.querySelector('[data-unscramble-check]').click();
            return item.getAttribute('data-exercise-resolved');
        })()`);

        await cdp.evaluate(`(() => {
            const item = document.querySelector('section[data-exercise-kind="translate"] [data-exercise-item]');
            const input = item.querySelector('[data-exercise-input]');
            input.value = 'א';
            item.querySelector('[data-exercise-submit]').click();
            return item.getAttribute('data-exercise-resolved');
        })()`);
        await sleep(100);

        const playBox = await cdp.evaluate(`(() => {
            const btn = document.querySelector('section[data-exercise-kind="dictation"] [data-dictation-play]');
            const r = btn.getBoundingClientRect();
            return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
        })()`);
        await mouseClick(cdp, playBox.x, playBox.y);
        await sleep(400);

        await cdp.evaluate(`(() => {
            const sections = [...document.querySelectorAll('section[data-exercise-kind="dictation"]')];
            const failSection = sections[sections.length - 1];
            failSection.querySelector('[data-dictation-play]').click();
            return true;
        })()`);
        await waitFor(
            cdp,
            `(() => {
                const sections = [...document.querySelectorAll('section[data-exercise-kind="dictation"]')];
                const failSection = sections[sections.length - 1];
                const skip = failSection.querySelector('[data-dictation-skip]');
                const fb = failSection.querySelector('[data-exercise-feedback]');
                return skip && !skip.hidden && fb && /Audio could not load/.test(fb.textContent);
            })()`,
            'dictation skip after audio fail',
            8000
        );
        await cdp.evaluate(`(() => {
            const sections = [...document.querySelectorAll('section[data-exercise-kind="dictation"]')];
            sections[sections.length - 1].querySelector('[data-dictation-skip]').click();
            return true;
        })()`);

        const liveOk = await cdp.evaluate(`(() => {
            const lives = [...document.querySelectorAll('[data-exercise-live]')];
            return lives.some((p) => /Correct|Try again|Answer shown|Audio could not load|Skipped|first try/i.test(p.textContent));
        })()`);
        if (!liveOk) throw new Error('live region never updated');

        console.log('chrome-exercise-smoke: ok');
    } finally {
        await browser.close();
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-exercise-smoke") || main().catch((err) => {
    console.error(err);
    process.exit(1);
});
