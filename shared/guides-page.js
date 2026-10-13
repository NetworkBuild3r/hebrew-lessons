/**
 * Guides shelf. Published lessons whose recipe is guide, grouped by topic.
 * A card opens that lesson in the plan view. Plan text is escaped.
 * Provenance: INIT-006/SPEC-016
 */
(function (root) {
    'use strict';

    var TEXT_LIMIT = 120;
    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var mounted = null;

    function escapeHtml(value) {
        var U = root.HebrewUtils;
        if (U && typeof U.escapeHtml === 'function') return U.escapeHtml(value);
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function clip(value) {
        var raw = value == null ? '' : String(value);
        if (raw.length > TEXT_LIMIT) raw = raw.slice(0, TEXT_LIMIT);
        return raw;
    }

    function isHebrew(value) {
        return /[\u0590-\u05FF]/.test(String(value || ''));
    }

    /**
     * Guide rows only. A plan id or lesson id that is not a slug is dropped
     * so it cannot become a URL.
     * @param {unknown} lessons
     * @returns {object[]}
     */
    function guidesOnly(lessons) {
        if (!Array.isArray(lessons)) return [];
        var out = [];
        for (var i = 0; i < lessons.length && out.length < 200; i += 1) {
            var row = lessons[i];
            if (!row || row.recipe !== 'guide') continue;
            if (typeof row.planId !== 'string' || !SLUG.test(row.planId)) continue;
            if (typeof row.lessonId !== 'string' || !SLUG.test(row.lessonId)) continue;
            out.push(row);
        }
        return out;
    }

    /**
     * @param {object[]} rows
     * @returns {Array<{ topic: string, lessons: object[] }>}
     */
    function groupByTopic(rows) {
        var groups = [];
        var index = Object.create(null);
        rows.forEach(function (row) {
            var topic = clip(row.topic);
            var key = topic || 'Guides';
            if (!Object.prototype.hasOwnProperty.call(index, key)) {
                index[key] = groups.length;
                groups.push({ topic: key, lessons: [] });
            }
            groups[index[key]].lessons.push(row);
        });
        return groups;
    }

    function planHref(planId, lessonId) {
        return '/shared/plan/?id=' + encodeURIComponent(planId) + '&lesson=' + encodeURIComponent(lessonId);
    }

    function renderCard(row) {
        var title = clip(row.title);
        var titleDir = isHebrew(title) ? 'rtl' : 'ltr';
        var titleLang = titleDir === 'rtl' ? 'he' : 'en';
        return '<article class="lesson-card" data-recipe="guide" data-plan-id="' + escapeHtml(row.planId) + '"'
            + ' data-lesson-id="' + escapeHtml(row.lessonId) + '">'
            + '<a class="lesson-link" href="' + escapeHtml(planHref(row.planId, row.lessonId)) + '">'
            + '<span class="recipe-chip" data-recipe="guide">מדריך '
            + '<span lang="en" dir="ltr">Guide</span></span>'
            + '<h3 lang="' + titleLang + '" dir="' + titleDir + '">' + escapeHtml(title) + '</h3>'
            + '</a></article>';
    }

    /**
     * @param {unknown} lessons
     * @returns {string}
     */
    function render(lessons) {
        var groups = groupByTopic(guidesOnly(lessons));
        if (!groups.length) {
            return '<div class="card guides-state" data-guides-state="empty">'
                + '<p>אין מדריכים שפורסמו. <span lang="en" dir="ltr">(No published guides.)</span></p>'
                + '</div>';
        }
        return groups.map(function (group) {
            var topicDir = isHebrew(group.topic) ? 'rtl' : 'ltr';
            var topicLang = topicDir === 'rtl' ? 'he' : 'en';
            var cards = group.lessons.map(renderCard).join('');
            return '<section class="guides-topic" data-topic>'
                + '<h2 lang="' + topicLang + '" dir="' + topicDir + '">' + escapeHtml(group.topic) + '</h2>'
                + '<div class="home-list guides-list" data-guides-state="ready">' + cards + '</div>'
                + '</section>';
        }).join('');
    }

    function paint(el, html) {
        if (!el) return;
        el.innerHTML = html;
    }

    function mount(el, ctx) {
        mounted = el;
        var ready = (ctx && ctx.ready) || Promise.resolve(null);
        return ready.then(function () {
            var Api = root.HebrewApi;
            if (!Api || typeof Api.get !== 'function') {
                return Promise.reject(new Error('API unavailable'));
            }
            return Api.get('/api/plans/catalog', { redirectOn401: false });
        }).then(function (res) {
            if (mounted !== el) return;
            var lessons = res && res.ok && res.data && Array.isArray(res.data.lessons)
                ? res.data.lessons
                : null;
            if (!lessons) {
                paint(el, '<div class="card guides-state" data-guides-state="error" role="alert">'
                    + '<p>שגיאה בטעינת המדריכים. <span lang="en" dir="ltr">(Could not load guides.)</span></p>'
                    + '</div>');
                return;
            }
            paint(el, render(lessons));
        }).catch(function () {
            if (mounted !== el) return;
            paint(el, '<div class="card guides-state" data-guides-state="error" role="alert">'
                + '<p>שגיאה בטעינת המדריכים. <span lang="en" dir="ltr">(Could not load guides.)</span></p>'
                + '</div>');
        });
    }

    function unmount() {
        mounted = null;
    }

    var api = {
        groupByTopic: groupByTopic,
        guidesOnly: guidesOnly,
        mount: mount,
        render: render,
        unmount: unmount
    };
    root.HebrewGuidesPage = api;
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
