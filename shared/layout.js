/**
 * Page layout templates for the signed-in app (ADR-0007).
 *
 * Every screen uses the same frame: a heading block, an optional actions slot,
 * and one view root that the screen's module fills. Do not hand-write a
 * `<header class="topbar page-heading">` inside a view; describe the heading in
 * shared/app-routes.js and let this module render it.
 *
 * Provenance: INIT-005/SPEC-002
 */
(function (root) {
    'use strict';

    var U = root.HebrewUtils;

    function esc(value) {
        return U && typeof U.escapeHtml === 'function'
            ? U.escapeHtml(value)
            : String(value == null ? '' : value)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
    }

    /** Hebrew text with the English gloss in a dir="ltr" span (RSCH-006 rule 15). */
    function bi(he, en) {
        if (!en) return esc(he);
        return esc(he) + ' <span class="en" dir="ltr">(' + esc(en) + ')</span>';
    }

    /**
     * Loading / error / empty card used while a view has no data yet.
     * `hook` becomes data-<hook>="<kind>" so tests and smokes keep their selectors.
     * @param {{ kind: string, he: string, en: string, hook?: string, className?: string }} opts
     */
    function stateCard(opts) {
        var o = opts || {};
        var hookAttr = o.hook ? ' data-' + esc(o.hook) + '="' + esc(o.kind || 'loading') + '"' : '';
        var role = o.kind === 'error' ? ' role="alert"' : '';
        return '<div class="card' + (o.className ? ' ' + esc(o.className) : '') + '"' + hookAttr + role + '>'
            + '<p>' + bi(o.he || 'טוען…', o.en || 'Loading…') + '</p>'
            + '</div>';
    }

    var ROUTE_ICONS = {
        today: '<circle cx="12" cy="14" r="4"/><path d="M12 6v2M5.5 8.5l1.4 1.4M18.5 8.5 17.1 9.9M4 14h2M18 14h2M6 20h12"/>',
        lessons: '<path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v15H6.5A2.5 2.5 0 0 0 4 20.5Z"/><path d="M4 20.5A2.5 2.5 0 0 1 6.5 18H20"/>',
        review: '<path d="M4 12a8 8 0 0 1 14-5l2 2M20 12a8 8 0 0 1-14 5l-2-2"/><path d="M20 4v5h-5M4 20v-5h5"/>',
        progress: '<path d="M4 20V10M10 20V4M16 20v-8M21 20H3"/>',
        admin: '<path d="M12 3 5 6v5c0 5 3 8 7 10 4-2 7-5 7-10V6Z"/><path d="m9.5 12 2 2 3.5-4"/>'
    };

    function routeIcon(id) {
        var inner = ROUTE_ICONS[id];
        if (!inner) return '';
        return '<svg class="hebrew-icon page-heading-icon" width="26" height="26"'
            + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"'
            + ' stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">'
            + inner + '</svg>';
    }

    /**
     * Heading + actions slot + view root for one route.
     * @param {object} route an entry from shared/app-routes.js
     * @returns {string}
     */
    function page(route) {
        var h = route.heading || {};
        var innerClass = 'topbar-inner' + (h.className ? ' ' + esc(h.className) : '');
        var subtitle = h.subHe
            ? '<p class="subtitle"' + (h.subtitleId ? ' id="' + esc(h.subtitleId) + '"' : '') + '>'
                + bi(h.subHe, h.subEn) + '</p>'
            : '';
        var icon = routeIcon(route.id);
        var iconHtml = icon
            ? '<span class="page-heading-icon-wrap" aria-hidden="true">' + icon + '</span> '
            : '';
        return ''
            + '<header class="topbar page-heading" data-page-heading>'
            + '<div class="' + innerClass + '">'
            + '<div class="page-heading-text">'
            + '<div class="kicker"><span class="kicker-mark" aria-hidden="true">✦</span> דרך עברית • Hebrew Way</div>'
            + '<h1 tabindex="-1">' + iconHtml
            + bi(h.he || '', h.en || '') + '</h1>'
            + subtitle
            + '</div>'
            + '<div class="page-actions" data-page-actions></div>'
            + '</div>'
            + '</header>'
            + '<div id="' + esc(route.rootId) + '" class="spa-view" aria-live="polite">'
            + stateCard({ kind: 'loading', he: 'טוען…', en: 'Loading…', hook: route.loadingHook })
            + '</div>';
    }

    /** Full-screen message inside the shell (not found, no access, failed to load). */
    function message(he, en, bodyHe, bodyEn) {
        return ''
            + '<header class="topbar page-heading" data-page-heading><div class="topbar-inner"><div>'
            + '<h1 tabindex="-1">' + bi(he, en) + '</h1>'
            + '</div></div></header>'
            + '<div class="card" role="alert"><p>' + bi(bodyHe, bodyEn) + '</p>'
            + '<p><a class="btn btn-primary" href="/">' + bi('להיום', 'Back to Today') + '</a></p></div>';
    }

    root.HebrewLayout = {
        esc: esc,
        bi: bi,
        stateCard: stateCard,
        page: page,
        message: message
    };
})(typeof window !== 'undefined' ? window : globalThis);
