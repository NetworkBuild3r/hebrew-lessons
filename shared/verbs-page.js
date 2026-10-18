/**
 * Verbs: daily verb rounds across every published verb workshop the learner can open.
 * The card engine, ladder and Look tables come from shared/verb-practice.js.
 * Provenance: INIT-013 follow-up (verb practice redesign).
 */
(function (root) {
    'use strict';

    var SLUG = /^[a-z0-9][a-z0-9-]*$/;
    var PLAN_LIMIT = 12;

    function esc(value) {
        var U = root.HebrewUtils;
        if (U && typeof U.escapeHtml === 'function') return U.escapeHtml(value);
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }

    function bi(he, en) {
        return '<span lang="he">' + esc(he) + '</span> <span class="en" lang="en" dir="ltr">' + esc(en) + '</span>';
    }

    /**
     * @param {Array} rows catalog lessons
     * @returns {Array<{planId:string, lessonId:string, title:string}>}
     */
    function verbLessons(rows) {
        return (Array.isArray(rows) ? rows : []).filter(function (row) {
            return row && row.recipe === 'verb-workshop'
                && SLUG.test(String(row.planId || '')) && SLUG.test(String(row.lessonId || ''));
        }).map(function (row) {
            return { planId: row.planId, lessonId: row.lessonId, title: String(row.title || row.lessonId) };
        });
    }

    function lessonHref(row) {
        return '/shared/plan/?id=' + encodeURIComponent(row.planId) + '&lesson=' + encodeURIComponent(row.lessonId);
    }

    function renderState(kind, he, en, extra) {
        return '<div class="card verbs-state" data-verbs-state="' + esc(kind) + '">'
            + '<p>' + bi(he, en) + '</p>' + (extra || '') + '</div>';
    }

    function renderEmpty() {
        return renderState('empty', 'עוד אין שיעור פעלים פתוח.', 'No verb lesson is open yet.',
            '<p class="fine">' + bi('כשיתפרסם שיעור פעלים, התרגול יופיע כאן.',
                'When a verb lesson is published, its practice shows up here.') + '</p>'
            + '<p><a class="btn" href="/lessons/">' + bi('לשיעורים', 'Lessons') + '</a></p>');
    }

    function renderSources(rows) {
        if (!rows.length) return '';
        return '<p class="verbs-sources fine">' + bi('מתוך', 'From') + ': '
            + rows.map(function (row) {
                return '<a href="' + esc(lessonHref(row)) + '">' + esc(row.title) + '</a>';
            }).join(' · ') + '</p>';
    }

    // ---- view lifecycle (contract: shared/router.js) ----
    var mounted = null;
    var controller = null;

    function getJson(url, signal) {
        var Api = root.HebrewApi;
        return Api.get(url, { redirectOn401: false, signal: signal })
            .then(function (res) { return res && res.ok ? res.data : null; })
            .catch(function () { return null; });
    }

    function mount(el, ctx) {
        mounted = el;
        var VP = root.HebrewVerbPractice;
        var Api = root.HebrewApi;
        var signal = (ctx && ctx.signal) || null;
        if (!el) return Promise.resolve();
        if (!VP || !Api || typeof Api.get !== 'function') {
            el.innerHTML = renderState('error', 'התרגול לא נטען.', 'Practice did not load.');
            return Promise.resolve();
        }
        el.innerHTML = renderState('loading', 'טוען…', 'Loading…');
        var ready = (ctx && ctx.ready) || Promise.resolve(null);
        var rows = [];
        return ready.then(function () {
            if (mounted !== el) return null;
            return getJson('/api/plans/catalog', signal);
        }).then(function (catalog) {
            if (mounted !== el) return null;
            rows = verbLessons(catalog && catalog.lessons);
            var planIds = [];
            rows.forEach(function (row) {
                if (planIds.indexOf(row.planId) < 0 && planIds.length < PLAN_LIMIT) planIds.push(row.planId);
            });
            return Promise.all(planIds.map(function (id) {
                return getJson('/api/plans/' + encodeURIComponent(id) + '/published', signal)
                    .then(function (plan) { return { id: id, plan: plan }; });
            }));
        }).then(function (plans) {
            if (!plans || mounted !== el) return null;
            var byId = Object.create(null);
            plans.forEach(function (p) { if (p.plan && typeof p.plan === 'object') byId[p.id] = p.plan; });
            rows = rows.filter(function (row) { return byId[row.planId]; });
            var blocks = [];
            rows.forEach(function (row) {
                blocks = blocks.concat(VP.conjugationBlocks(byId[row.planId], row.lessonId));
            });
            if (!blocks.length) return { blocks: blocks, data: null };
            return VP.loadVerbData(blocks, function (url) { return getJson(url, signal); })
                .then(function (data) { return { blocks: blocks, data: data }; });
        }).then(function (loaded) {
            if (!loaded || mounted !== el) return;
            var verbs = loaded.data
                ? VP.collectVerbs(loaded.blocks, loaded.data.forms, loaded.data.lemmas, loaded.data.paradigms)
                : [];
            if (!verbs.length) {
                el.innerHTML = renderEmpty();
                return;
            }
            el.innerHTML = VP.renderStudio({
                verbs: verbs,
                tabs: ['practice', 'look'],
                tab: 'practice',
                lookHtml: VP.renderLook(verbs, verbs[0].lemma, '')
            }) + renderSources(rows);
            controller = VP.mount(el, {
                verbs: verbs,
                user: ctx && ctx.user,
                slug: rows.length === 1 ? rows[0].lessonId : ''
            });
        }).catch(function () {
            if (mounted !== el) return;
            el.innerHTML = renderState('error', 'התרגול לא נטען.', 'Practice did not load.');
        });
    }

    function unmount() {
        if (controller && typeof controller.destroy === 'function') controller.destroy();
        controller = null;
        mounted = null;
    }

    var api = { mount: mount, unmount: unmount, verbLessons: verbLessons, lessonHref: lessonHref };
    root.HebrewVerbsPage = api;
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
