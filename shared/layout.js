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

    // Route heading icons are drawn from the brand sprite (INIT-005/SPEC-010). Guides has no icon yet.
    var ROUTE_ICONS = { today: 1, lessons: 1, review: 1, words: 1, progress: 1, admin: 1 };

    function routeIcon(id) {
        if (!ROUTE_ICONS[id]) return '';
        return '<svg class="icon hebrew-icon page-heading-icon" width="26" height="26"'
            + ' aria-hidden="true" focusable="false"><use href="/shared/brand/sprite.svg#icon-' + id
            + '"></use></svg>';
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
            // INIT-005/SPEC-011: the old "דרך עברית • Hebrew Way" kicker is gone. It repeated the
            // brand in the header above and was a Hebrew + English label with no gloss structure.
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
