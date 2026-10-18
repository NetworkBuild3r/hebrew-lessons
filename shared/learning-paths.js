/**
 * One learning-path row for Lessons and Today.
 * Summaries, the muted count, and the row markup live here so the two pages do not each keep a copy.
 */
(function (root) {
    'use strict';

    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

    function utils() {
        return root.HebrewUtils;
    }

    function esc(value) {
        var U = utils();
        return U && typeof U.escapeHtml === 'function'
            ? U.escapeHtml(value)
            : String(value == null ? '' : value)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
    }

    function bi(he, en) {
        var U = utils();
        if (U && typeof U.bi === 'function') return U.bi(he, en);
        if (!en) return esc(he);
        if (!he) return '<span class="ui-en" lang="en" dir="ltr">' + esc(en) + '</span>';
        return '<span class="ui-pair"><span class="ui-he" lang="he">' + esc(he) + '</span>'
            + '<span class="ui-en" lang="en" dir="ltr">' + esc(en) + '</span></span>';
    }

    function pathHref(planId) {
        return '/shared/plan/?id=' + encodeURIComponent(planId);
    }

    /**
     * One entry per plan, in catalog order. A non-slug id is skipped so it cannot become a URL.
     * Counts come from the rows already loaded. The title is the plan title when the catalog sent one.
     */
    function summaries(lessons) {
        if (!Array.isArray(lessons)) return [];
        var order = [];
        var byId = Object.create(null);
        for (var i = 0; i < lessons.length; i += 1) {
            var row = lessons[i];
            if (!row || typeof row.planId !== 'string' || !SLUG.test(row.planId)) continue;
            var entry = byId[row.planId];
            if (!entry) {
                entry = { id: row.planId, title: '', lessons: 0, units: 0 };
                byId[row.planId] = entry;
                order.push(entry);
                entry._units = [];
            }
            entry.lessons += 1;
            if (!entry.title && typeof row.planTitle === 'string' && row.planTitle) entry.title = row.planTitle;
            if (typeof row.unitId === 'string' && row.unitId && entry._units.indexOf(row.unitId) < 0) {
                entry._units.push(row.unitId);
                entry.units += 1;
            }
        }
        return order;
    }

    function meta(entry) {
        if (!entry) return '';
        var units = entry.units;
        var lessons = entry.lessons;
        if (typeof units === 'number' && units > 1) {
            return '<p class="path-meta">' + bi(units + ' יחידות', units + ' units') + '</p>';
        }
        if (typeof lessons === 'number' && lessons > 0) {
            var he = lessons === 1 ? 'שיעור אחד' : (lessons + ' שיעורים');
            var en = lessons + (lessons === 1 ? ' lesson' : ' lessons');
            return '<p class="path-meta">' + bi(he, en) + '</p>';
        }
        return '';
    }

    function titleHtml(title) {
        if (title) return '<h2 class="path-title"><bdi lang="he">' + esc(title) + '</bdi></h2>';
        return '<h2 class="path-title">' + bi('מסלול לימוד', 'Learning path') + '</h2>';
    }

    function chevron() {
        return '<svg class="icon path-chevron icon-directional" width="20" height="20" aria-hidden="true" focusable="false">'
            + '<use href="/shared/brand/sprite.svg#icon-next"></use></svg>';
    }

    function allPathsLink() {
        return '<a class="path-all" href="/lessons/">' + bi('כל המסלולים', 'All paths') + '</a>';
    }

    /**
     * @param {Array<string|{id:string,title?:string,lessons?:number,units?:number}>} entries
     * @param {{ firstOnly?: boolean, allPaths?: boolean, today?: boolean }} [options]
     * Today passes firstOnly and allPaths. Lessons lists every plan.
     */
    function render(entries, options) {
        var opts = options || {};
        if (!entries || !entries.length) return '';
        var seen = [];
        var html = '';
        var limit = opts.firstOnly ? 1 : entries.length;
        var shown = 0;
        for (var i = 0; i < entries.length && shown < limit; i += 1) {
            var item = entries[i];
            var id = typeof item === 'string' ? item : (item && item.id);
            if (typeof id !== 'string' || !SLUG.test(id) || seen.indexOf(id) >= 0) continue;
            seen.push(id);
            shown += 1;
            var title = item && typeof item.title === 'string' ? item.title : '';
            var cls = 'lesson-card' + (opts.today ? ' today-card' : '');
            var todayAttr = opts.today ? ' data-today-card="path"' : '';
            html += '<article class="' + cls + '" data-learning-path="' + esc(id) + '"' + todayAttr + '>'
                + '<a class="lesson-link" href="' + esc(pathHref(id)) + '">'
                + '<span class="path-copy">'
                + titleHtml(title)
                + meta(typeof item === 'string' ? null : item)
                + '</span>'
                + chevron()
                + '</a></article>';
        }
        if (!html) return '';
        if (opts.allPaths) html = html.replace(/<\/article>$/, allPathsLink() + '</article>');
        return html;
    }

    var api = {
        summaries: summaries,
        meta: meta,
        render: render,
        pathHref: pathHref
    };
    root.HebrewLearningPaths = api;
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
