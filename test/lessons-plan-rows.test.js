/**
 * Lessons page lists published plan lessons (INIT-013 follow-up).
 * A published verb-workshop lesson had no entry point: the Guides page keeps recipe "guide" only,
 * and the Lessons page read the worksheet catalog only.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const SRC = fs.readFileSync(path.resolve(__dirname, '..', 'shared', 'lessons-page.js'), 'utf8');

function load(catalog, planLessons) {
    const sandbox = {
        HEBREW_CATALOG: catalog,
        HebrewApi: {
            get(url) {
                if (url === '/api/plans/catalog') {
                    return planLessons instanceof Error
                        ? Promise.reject(planLessons)
                        : Promise.resolve({ ok: true, data: { lessons: planLessons } });
                }
                return Promise.resolve({ ok: true, data: { state: {} } });
            }
        }
    };
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(SRC, sandbox);
    return sandbox.HebrewLessonsPage;
}

const WORKSHOP = {
    planId: 'verb-tense-workshop',
    lessonId: 'decide-choose-believe-win',
    title: 'Decide, choose, believe, win',
    topic: 'Verbs',
    recipe: 'verb-workshop'
};
const GUIDE = { planId: 'p', lessonId: 'g', title: 'A guide', topic: 'Guides', recipe: 'guide' };

function mounted(page) {
    const el = { innerHTML: '' };
    return page.mount(el, {}).then(() => el.innerHTML);
}

describe('Lessons page: published plan lessons', () => {
    it('lists a published verb workshop with a link to the plan player', async () => {
        const page = load([], [WORKSHOP]);
        const html = await mounted(page);
        assert.match(html, /data-plan-id="verb-tense-workshop"/);
        assert.match(html, /href="\/shared\/plan\/\?id=verb-tense-workshop&amp;lesson=decide-choose-believe-win"/);
        assert.match(html, /data-recipe="verb-workshop"/);
        assert.match(html, /Decide, choose, believe, win/);
        assert.doesNotMatch(html, /data-lessons-state="no-lessons"/);
    });

    it('keeps guides on the Guides page', async () => {
        const html = await mounted(load([], [GUIDE, WORKSHOP]));
        assert.doesNotMatch(html, /data-lesson-id="g"/);
        assert.match(html, /data-lesson-id="decide-choose-believe-win"/);
    });

    it('drops a row whose ids are not slugs, so it cannot become a URL', () => {
        const page = load([], []);
        const rows = page.planRowsOnly([
            { planId: 'a/../b', lessonId: 'ok', recipe: 'verb-workshop' },
            { planId: 'ok', lessonId: 'x"onclick', recipe: 'verb-workshop' },
            { planId: 'ok', lessonId: 'fine', recipe: 'verb-workshop' }
        ]);
        assert.equal(rows.length, 1);
        assert.equal(rows[0].lessonId, 'fine');
    });

    it('shows worksheet lessons when the plan catalog fails', async () => {
        const html = await mounted(load(
            [{ slug: 'my-morning', title: 'Morning', level: 'basic' }],
            new Error('offline')
        ));
        assert.match(html, /data-lesson-slug="my-morning"/);
        assert.doesNotMatch(html, /data-plan-id/);
    });

    it('lists worksheet lessons first, then plan lessons', async () => {
        const html = await mounted(load([{ slug: 'my-morning', title: 'Morning', level: 'basic' }], [WORKSHOP]));
        assert.ok(html.indexOf('data-lesson-slug="my-morning"') < html.indexOf('data-plan-id="verb-tense-workshop"'));
    });

    it('plan lessons carry no level, so a level filter hides them', () => {
        const page = load([], []);
        const html = page.render({
            state: 'ready',
            filter: 'basic',
            rows: [],
            planRows: [WORKSHOP]
        });
        assert.doesNotMatch(html, /data-plan-id/);
        assert.match(html, /data-lessons-state="filtered-empty"/);
    });

    it('marks an English plan topic as English and left to right', () => {
        const page = load([], []);
        const html = page.render({
            state: 'ready',
            filter: 'all',
            rows: [],
            planRows: [Object.assign({}, WORKSHOP, { topic: 'Routine' })]
        });
        assert.match(html, /<p class="fine"><span lang="en" dir="ltr">Routine<\/span><\/p>/);
        const hebrew = page.render({
            state: 'ready',
            filter: 'all',
            rows: [],
            planRows: [Object.assign({}, WORKSHOP, { topic: 'יחידה 1' })]
        });
        assert.match(hebrew, /<p class="fine">יחידה 1<\/p>/);
        assert.doesNotMatch(hebrew, /<p class="fine"><span lang="en"/);
    });

    it('escapes a title', async () => {
        const html = await mounted(load([], [Object.assign({}, WORKSHOP, { title: '<img src=x onerror=1>' })]));
        assert.doesNotMatch(html, /<img/);
        assert.match(html, /&lt;img/);
    });
});
