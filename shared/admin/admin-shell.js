/**
 * Admin landing: tab strip + hash routing.
 * Provenance: INIT-002/SPEC-016
 * Lessons editor is SPEC-017 — placeholder only here.
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

    function bi(he, en) {
        return U && typeof U.bi === 'function'
            ? U.bi(he, en)
            : (esc(he) + ' <span class="en" dir="ltr">(' + esc(en) + ')</span>');
    }

    var PLAN_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

    /**
     * @param {string} hash
     * @returns {{ view: 'users'|'user'|'lessons'|'lesson'|'plans'|'plan', userId: string|null, slug: string|null, planId: string|null }}
     */
    function parseHash(hash) {
        var raw = String(hash || '').replace(/^#/, '').trim();
        if (!raw || raw === 'users') {
            return { view: 'users', userId: null, slug: null, planId: null };
        }
        if (raw === 'lessons') {
            return { view: 'lessons', userId: null, slug: null, planId: null };
        }
        if (raw === 'plans') {
            return { view: 'plans', userId: null, slug: null, planId: null };
        }
        var userMatch = /^user\/([^/]+)$/.exec(raw);
        if (userMatch) {
            return { view: 'user', userId: decodeURIComponent(userMatch[1]), slug: null, planId: null };
        }
        var lessonMatch = /^lesson\/([^/]+)$/.exec(raw);
        if (lessonMatch) {
            return { view: 'lesson', userId: null, slug: decodeURIComponent(lessonMatch[1]), planId: null };
        }
        var planMatch = /^plan\/([^/]+)$/.exec(raw);
        if (planMatch) {
            var planId = '';
            try {
                planId = decodeURIComponent(planMatch[1]);
            } catch (err) {
                planId = '';
            }
            if (PLAN_SLUG.test(planId)) {
                return { view: 'plan', userId: null, slug: null, planId: planId };
            }
        }
        return { view: 'users', userId: null, slug: null, planId: null };
    }

    function setHash(route) {
        try {
            if (root.location) {
                root.location.hash = route;
            }
        } catch (err) { /* ignore */ }
    }

    function renderTabs(active) {
        var usersActive = active === 'users' || active === 'user';
        var lessonsActive = active === 'lessons' || active === 'lesson';
        var plansActive = active === 'plans' || active === 'plan';
        return ''
            + '<nav class="admin-tabs" aria-label="' + esc('מדורי ניהול') + '" dir="rtl" lang="he">'
            + '<a class="admin-tab' + (usersActive ? ' is-active' : '') + '" href="#users"'
            + ' data-admin-tab="users"'
            + (usersActive ? ' aria-current="page"' : '') + '>'
            + bi('משתמשים', 'Users')
            + '</a>'
            + '<a class="admin-tab' + (lessonsActive ? ' is-active' : '') + '" href="#lessons"'
            + ' data-admin-tab="lessons"'
            + (lessonsActive ? ' aria-current="page"' : '') + '>'
            + bi('שיעורים', 'Lessons')
            + '</a>'
            + '<a class="admin-tab' + (plansActive ? ' is-active' : '') + '" href="#plans"'
            + ' data-admin-tab="plans"'
            + (plansActive ? ' aria-current="page"' : '') + '>'
            + bi('תוכניות', 'Plans')
            + '</a>'
            + '</nav>';
    }

    function renderLessonsPlaceholder() {
        return ''
            + '<div class="card admin-lessons-placeholder" data-admin-view="lessons">'
            + '<h2>' + bi('שיעורים', 'Lessons') + '</h2>'
            + '<p>'
            + bi(
                'עורך השיעורים עדיין לא בעמוד הזה.',
                'The lesson editor is not on this page yet.'
            )
            + '</p>'
            + '</div>';
    }

    /**
     * Router view (contract: shared/router.js). Admin sub-pages stay on the URL hash
     * (#users, #lessons, #user/<id>, #lesson/<slug>); the app router ignores hash-only changes.
     * @param {HTMLElement} target
     * @param {{ Users?: object, Lessons?: object, LessonEditor?: object, PlanBuilder?: object, signal?: AbortSignal }} [opts]
     *        Router ctx when opened from the app; tests pass module stubs.
     */
    function mount(target, opts) {
        if (!target) return;
        var options = opts || {};
        var Users = options.Users || root.HebrewAdminUsers;
        var Lessons = options.Lessons || root.HebrewAdminLessons;
        var LessonEditor = options.LessonEditor || root.HebrewAdminLessonEditor;
        var PlanBuilder = options.PlanBuilder || root.HebrewPlanBuilder;
        activeEditor = LessonEditor;
        var state = { route: parseHash(root.location && root.location.hash) };

        function paint() {
            var route = state.route;
            var html = '<div class="admin-shell" dir="rtl" lang="he">';
            html += renderTabs(route.view);
            html += '<div class="admin-panel" id="admin-panel" data-admin-panel></div>';
            html += '</div>';
            target.innerHTML = html;

            var panel = target.querySelector('[data-admin-panel]');
            if (!panel) return;

            var showingPlan = route.view === 'plan' || route.view === 'plans';
            if (!showingPlan && PlanBuilder && typeof PlanBuilder.unmount === 'function') {
                PlanBuilder.unmount();
            }
            activeBuilder = showingPlan ? PlanBuilder : null;

            if (route.view === 'plan' && route.planId && PlanBuilder && typeof PlanBuilder.mount === 'function') {
                PlanBuilder.mount(panel, route.planId);
                return;
            }
            if (route.view === 'plans') {
                if (PlanBuilder && typeof PlanBuilder.mountPicker === 'function') {
                    PlanBuilder.mountPicker(panel);
                } else {
                    panel.innerHTML = '<div class="card" data-admin-view="plans"><p>'
                        + bi('בוחר התוכנית לא נטען.', 'The plan picker is not loaded.')
                        + '</p></div>';
                }
                return;
            }
            if (route.view === 'lesson' && route.slug) {
                if (LessonEditor && typeof LessonEditor.mount === 'function') {
                    LessonEditor.mount(panel, route.slug);
                } else if (Lessons && typeof Lessons.mountEditor === 'function') {
                    Lessons.mountEditor(panel, route.slug);
                } else {
                    panel.innerHTML = renderLessonsPlaceholder();
                }
                return;
            }
            if (route.view === 'lessons') {
                if (Lessons && typeof Lessons.mountList === 'function') {
                    Lessons.mountList(panel);
                } else if (Lessons && typeof Lessons.mount === 'function') {
                    Lessons.mount(panel);
                } else {
                    panel.innerHTML = renderLessonsPlaceholder();
                }
                return;
            }
            if (route.view === 'user' && route.userId && Users && typeof Users.mountUserDetail === 'function') {
                Users.mountUserDetail(panel, route.userId);
                return;
            }
            if (Users && typeof Users.mountUsersList === 'function') {
                Users.mountUsersList(panel);
            } else {
                panel.innerHTML = '<div class="card"><p>' + bi('טוען…', 'Loading…') + '</p></div>';
            }
        }

        function onHash() {
            state.route = parseHash(root.location && root.location.hash);
            paint();
        }

        if (root.addEventListener) {
            // Removed automatically when the route closes (options.signal is the router's AbortSignal).
            root.addEventListener('hashchange', onHash, options.signal ? { signal: options.signal } : false);
        }

        if (!root.location || !root.location.hash) {
            // replaceState, not location.hash: a hash assignment would add a history entry.
            try {
                root.history.replaceState(root.history.state, '', '#users');
            } catch (err) {
                setHash('users');
            }
        }
        state.route = parseHash(root.location && root.location.hash);
        paint();
    }

    var activeEditor = null;
    var activeBuilder = null;

    /** Router asks before leaving /admin/: blocks if an editor has unsaved field text. */
    function canLeave() {
        if (activeBuilder && typeof activeBuilder.canLeave === 'function' && !activeBuilder.canLeave()) {
            return false;
        }
        if (activeEditor && typeof activeEditor.canLeave === 'function') return activeEditor.canLeave();
        return true;
    }

    function unmount() {
        if (activeBuilder && typeof activeBuilder.unmount === 'function') activeBuilder.unmount();
        activeBuilder = null;
        if (activeEditor && typeof activeEditor.unmount === 'function') activeEditor.unmount();
        activeEditor = null;
    }

    root.HebrewAdminShell = {
        parseHash: parseHash,
        renderTabs: renderTabs,
        renderLessonsPlaceholder: renderLessonsPlaceholder,
        mount: mount,
        unmount: unmount,
        canLeave: canLeave,
        setHash: setHash
    };
})(typeof window !== 'undefined' ? window : globalThis);
