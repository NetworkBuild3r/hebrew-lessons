#!/usr/bin/env node
/**
 * Chrome smoke: exercise and assignment on /editor/ (INIT-012/SPEC-007).
 * Run: node test/chrome-plan-editor-exercise.js
 */
'use strict';

const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const { samplePlan } = require('./helpers/plan-editor-fixtures');
const Form = require('../shared/admin/plan-form');
const { validatePlan } = require('../server/plans/validate');
const fs = require('fs');
const path = require('path');

const { sleep, waitFor } = chromeCdp;

const PLAN_ID = 'editor-exercise-smoke';
const SCHEMA = JSON.parse(fs.readFileSync(
    path.join(__dirname, '..', 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json'), 'utf8'
));
const failures = [];
function fail(where, msg) { failures.push(where + ': ' + msg); }

async function main() {
    const started = await harness.start({ lessons: 'fixture' });
    const BASE = started.baseUrl;
    const store = require('../server/plans/store');
    const plan = samplePlan();
    plan.id = PLAN_ID;
    plan.schemaVersion = '1.2.0';
    plan.title = 'Exercise editor smoke';
    plan.units[0].lessons[0].recipe = 'review-worksheet';
    plan.units[0].lessons[0].newWords = [];
    plan.units[0].lessons[0].blocks = [
        Object.assign(Form.newBlock('exercise', 'ex-choose', SCHEMA), {
            kind: 'choose',
            items: [{ id: 'i1', prompt: 'Pick', choices: ['א', 'ב', 'ג'], answer: 'א' }]
        }),
        Object.assign(Form.newBlock('exercise', 'ex-order', SCHEMA), {
            kind: 'unscramble',
            items: [{ id: 'i1', words: ['א', 'ב', 'ג'] }]
        }),
        Object.assign(Form.newBlock('exercise', 'ex-tr', SCHEMA), {
            kind: 'translate',
            items: [{ id: 'i1', english: 'house', answer: 'בַּיִת' }]
        }),
        Object.assign(Form.newBlock('assignment', 'as-1', SCHEMA), {
            prompt: 'Write three sentences about your week.',
            minSentences: 3,
            homework: ['Review vocabulary', 'Read aloud']
        })
    ];
    store.saveWorkingCopy({
        id: plan.id,
        document: JSON.stringify(plan),
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null
    });
    const seeded = validatePlan(plan);
    if (!seeded.ok) fail('seed', 'plan validation failed: ' + JSON.stringify(seeded.blocks && seeded.blocks.slice(0, 3)));

    const browser = await chromeCdp.launch({ tag: 'plan-editor-exercise', portBase: 9870, width: 1280, height: 900 });
    const cdp = browser.cdp;
    try {
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        await cdp.call('Page.navigate', { url: BASE + '/editor/#plan/' + PLAN_ID });
        await waitFor(cdp, '!!document.querySelector("[data-plan-builder]")', 'plan builder', 25000);
        await cdp.evaluate('(function(){ var b=document.querySelector("[data-plan-lesson]"); if(b) b.click(); })()');
        await waitFor(cdp, '!!document.querySelector("[data-plan-lesson-screen]")', 'lesson screen', 15000);
        await waitFor(cdp, '!!document.querySelector("[data-plan-preview]")', 'preview pane', 15000);
        const previewHasExercise = await cdp.evaluate(
            '(document.querySelector("[data-plan-preview]") || {}).textContent || ""'
        );
        if (!/Pick|בַּיִת|house/i.test(previewHasExercise)) {
            fail('preview', 'learner preview did not render exercise/assignment content');
        }

        for (const width of [390, 1280]) {
            await cdp.call('Emulation.setDeviceMetricsOverride', {
                width, height: width < 500 ? 700 : 900, deviceScaleFactor: 1, mobile: width < 500
            });
            await sleep(300);
            const overflow = await cdp.evaluate('Math.max(0, document.documentElement.scrollWidth - document.documentElement.clientWidth)');
            if (overflow > 2) fail('layout ' + width, 'horizontal overflow ' + overflow + 'px');
        }

    } finally {
        await browser.close();
        await harness.stop();
    }
    if (failures.length) {
        console.error('FAILURES:\n' + failures.map((f) => ' - ' + f).join('\n'));
        process.exit(1);
    }
    console.log('chrome-plan-editor-exercise: PASS');
}

main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
