/**
 * Shared learning-path rows used by Lessons and Today.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const Paths = require('../shared/learning-paths.js');
const PLAN = 'bdf0aee7-7c57-4982-a351-78063ff567e9';

function row(planId, extra) {
    return Object.assign({
        planId: planId,
        planTitle: '',
        unitId: 'u1-routine',
        lessonId: 'br-01-yesh-li-raayon',
        title: 'יש לי רעיון',
        recipe: ''
    }, extra || {});
}

describe('learning path rows', () => {
    it('summarises one entry per plan, in catalog order, with lesson and unit counts', () => {
        const entries = Paths.summaries([
            row(PLAN, { planTitle: 'מסלול לימוד — Learning path' }),
            row(PLAN, { lessonId: 'br-02-lesson', unitId: 'u2-next' }),
            { planId: 'not a slug', lessonId: 'ok' },
            row('second-plan', { planTitle: 'שני', unitId: 'u1-routine', lessonId: 'other' }),
            { planId: 'guide-only', lessonId: 'g', recipe: 'guide', unitId: 'u-guide' }
        ]);
        assert.deepEqual(entries.map((entry) => entry.id), [PLAN, 'second-plan', 'guide-only']);
        assert.equal(entries[0].title, 'מסלול לימוד — Learning path');
        assert.equal(entries[0].lessons, 2);
        assert.equal(entries[0].units, 2);
        assert.equal(entries[1].lessons, 1);
        assert.equal(entries[1].units, 1);
    });

    it('meta names lessons, and units when a plan has more than one', () => {
        assert.match(Paths.meta({ lessons: 1, units: 1 }), /שיעור אחד/);
        assert.match(Paths.meta({ lessons: 1, units: 1 }), /1 lesson/);
        assert.match(Paths.meta({ lessons: 35, units: 1 }), /35 שיעורים/);
        assert.match(Paths.meta({ lessons: 35, units: 1 }), /35 lessons/);
        assert.match(Paths.meta({ lessons: 35, units: 3 }), /3 יחידות/);
        assert.match(Paths.meta({ lessons: 35, units: 3 }), /3 units/);
        assert.equal(Paths.meta(null), '');
    });

    it('renders every plan as a row with the sprite chevron', () => {
        const html = Paths.render([
            { id: PLAN, title: 'מסלול לימוד — Learning path', lessons: 35, units: 1 },
            { id: 'second-plan', title: 'שני', lessons: 2, units: 1 }
        ]);
        assert.equal((html.match(/data-learning-path=/g) || []).length, 2);
        assert.match(html, /<bdi lang="he">מסלול לימוד — Learning path<\/bdi>/);
        assert.match(html, /href="\/shared\/plan\/\?id=bdf0aee7-7c57-4982-a351-78063ff567e9"/);
        assert.match(html, /<svg class="icon path-chevron icon-directional" width="20" height="20"/);
        assert.match(html, /sprite\.svg#icon-next/);
        assert.doesNotMatch(html, /class="path-all"/);
        assert.doesNotMatch(html, /data-today-card/);
    });

    it('uses the bilingual pair when a plan has no title', () => {
        const html = Paths.render([PLAN]);
        assert.match(html, /class="ui-he" lang="he">מסלול לימוד</);
        assert.match(html, /class="ui-en" lang="en" dir="ltr">Learning path</);
        assert.doesNotMatch(html, /<bdi lang="he">מסלול לימוד<\/bdi>/);
    });

    it('escapes a plan title', () => {
        const html = Paths.render([{ id: 'ok-plan', title: '<img src=x onerror=1>', lessons: 1, units: 1 }]);
        assert.doesNotMatch(html, /<img/);
        assert.match(html, /&lt;img/);
    });

    it('on Today shows only the first plan and a link to Lessons', () => {
        const html = Paths.render([
            { id: PLAN, title: 'ראשון', lessons: 35, units: 1 },
            { id: 'second-plan', title: 'שני', lessons: 2, units: 1 }
        ], { firstOnly: true, allPaths: true, today: true });
        assert.equal((html.match(/data-learning-path=/g) || []).length, 1);
        assert.match(html, /data-learning-path="bdf0aee7-7c57-4982-a351-78063ff567e9"/);
        assert.match(html, /data-today-card="path"/);
        assert.match(html, /class="lesson-card today-card"/);
        assert.doesNotMatch(html, /second-plan/);
        assert.match(html, /<a class="path-all" href="\/lessons\/">/);
        assert.match(html, /כל המסלולים/);
        assert.match(html, /All paths/);
    });

    it('renders nothing when there are zero plans', () => {
        assert.equal(Paths.render([]), '');
        assert.equal(Paths.render(null), '');
        assert.equal(Paths.summaries(null).length, 0);
    });

    it('the row styles keep the accent edge, the active fill, and the hover nudge', () => {
        const css = fs.readFileSync(path.join(__dirname, '..', 'shared', 'worksheet.css'), 'utf8');
        assert.match(css, /\[data-learning-path\]\s*\{[^}]*border-inline-start:\s*4px solid var\(--color-primary\)/);
        assert.match(css, /\[data-learning-path\] > \.lesson-link:active\s*\{[^}]*background:\s*var\(--color-primary-soft\)/);
        assert.match(css, /@media \(prefers-reduced-motion:\s*no-preference\)\s*\{[^}]*translateX\(2px\)/);
        assert.match(css, /html\[data-ui-lang="he"\] \[data-learning-path\] \.path-chevron\s*\{[^}]*scaleX\(-1\)/);
        assert.match(css, /\.path-chevron\s*\{[^}]*width:\s*20px/);
        assert.match(css, /\.path-chevron\s*\{[^}]*color:\s*var\(--color-muted\)/);
    });
});
