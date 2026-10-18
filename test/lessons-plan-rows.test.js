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

    it('keeps one card when a shelf lesson and a plan lesson are the same worksheet', () => {
        const page = load([], []);
        const status = {
            kind: 'new', rank: 1, tone: 'fresh', chipHe: 'חדש', chipEn: 'new', he: '', en: ''
        };
        function shelf(slug, title, index) {
            return {
                index: index,
                lesson: { slug: slug, title: title, level: 'basic', subtitle: 'basic', blurb: '' },
                status: status
            };
        }
        const rows = [
            shelf('my-morning', 'שגרה 1 — הבוקר שלי', 0),
            shelf('my-morning-one-verb-two-verbs', 'שגרה 1 — הבוקר שלי', 1),
            shelf('plane-small-talk-yesh-li', 'שיחה 1 — במטוס', 2),
            shelf('exact-match-lesson', 'Exact id', 3),
            shelf('anal-sex-vocab', 'Private shelf only', 4)
        ];
        const planRows = [
            {
                planId: 'learning-path',
                lessonId: 'ws-my-morning-one-verb-two-verbs',
                title: 'שגרה 1 — הבוקר שלי',
                topic: 'Routine',
                recipe: 'topic-conversation'
            },
            {
                planId: 'learning-path',
                lessonId: 'plane-stand-in',
                sourceWorksheet: 'worksheets/plane-small-talk-yesh-li/worksheet.js',
                title: 'שיחה 1 — במטוס',
                recipe: 'topic-conversation'
            },
            {
                planId: 'learning-path',
                lessonId: 'exact-match-lesson',
                title: 'Exact id',
                recipe: 'topic-conversation'
            },
            {
                planId: 'learning-path',
                lessonId: 'br-01-yesh-li-raayon',
                title: 'גשר — יש לי רעיון',
                recipe: 'topic-conversation'
            }
        ];
        const html = page.render({ state: 'ready', filter: 'all', rows: rows, planRows: planRows });

        assert.doesNotMatch(html, /data-lesson-slug="my-morning-one-verb-two-verbs"/);
        assert.match(html, /data-lesson-id="ws-my-morning-one-verb-two-verbs"/);
        assert.match(html, /href="\/shared\/plan\/\?id=learning-path&amp;lesson=ws-my-morning-one-verb-two-verbs"/);

        assert.doesNotMatch(html, /data-lesson-slug="plane-small-talk-yesh-li"/);
        assert.match(html, /data-lesson-id="plane-stand-in"/);
        assert.doesNotMatch(html, /data-lesson-slug="exact-match-lesson"/);
        assert.match(html, /data-lesson-id="exact-match-lesson"/);

        assert.match(html, /data-lesson-slug="my-morning"/);
        assert.match(html, /data-lesson-slug="anal-sex-vocab"/);
        assert.match(html, /data-lesson-id="br-01-yesh-li-raayon"/);
        assert.ok(
            html.indexOf('data-lesson-slug="anal-sex-vocab"') < html.indexOf('data-plan-id="learning-path"'),
            'shelf cards that remain still come before plan cards'
        );

        const basic = page.render({ state: 'ready', filter: 'basic', rows: rows, planRows: planRows });
        assert.match(basic, /data-lesson-slug="my-morning-one-verb-two-verbs"/);
        assert.doesNotMatch(basic, /data-plan-id/);
    });

    it('escapes a title', async () => {
        const html = await mounted(load([], [Object.assign({}, WORKSHOP, { title: '<img src=x onerror=1>' })]));
        assert.doesNotMatch(html, /<img/);
        assert.match(html, /&lt;img/);
    });
});
