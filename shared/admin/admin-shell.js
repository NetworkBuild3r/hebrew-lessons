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

    /**
     * @param {string} hash
     * @returns {{ view: 'users'|'user'|'lessons'|'lesson', userId: string|null, slug: string|null }}
     */
    function parseHash(hash) {
        var raw = String(hash || '').replace(/^#/, '').trim();
        if (!raw || raw === 'users') {
            return { view: 'users', userId: null, slug: null };
        }
        if (raw === 'lessons') {
            return { view: 'lessons', userId: null, slug: null };
        }
        var userMatch = /^user\/([^/]+)$/.exec(raw);
        if (userMatch) {
            return { view: 'user', userId: decodeURIComponent(userMatch[1]), slug: null };
        }
        var lessonMatch = /^lesson\/([^/]+)$/.exec(raw);
        if (lessonMatch) {
            return { view: 'lesson', userId: null, slug: decodeURIComponent(lessonMatch[1]) };
        }
        return { view: 'users', userId: null, slug: null };
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
        return ''
            + '<nav class="admin-tabs" aria-label="' + esc('Admin sections') + '" dir="rtl" lang="he">'
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
     * @param {HTMLElement} target
     * @param {{ Users?: object, Lessons?: object, LessonEditor?: object }} [opts]
     */
    function mount(target, opts) {
        if (!target) return;
        var options = opts || {};
        var Users = options.Users || root.HebrewAdminUsers;
        var Lessons = options.Lessons || root.HebrewAdminLessons;
        var LessonEditor = options.LessonEditor || root.HebrewAdminLessonEditor;
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
            root.addEventListener('hashchange', onHash);
        }

        if (!root.location || !root.location.hash) {
            setHash('users');
        }
        state.route = parseHash(root.location && root.location.hash);
        paint();
    }

    root.HebrewAdminShell = {
        parseHash: parseHash,
        renderTabs: renderTabs,
        renderLessonsPlaceholder: renderLessonsPlaceholder,
        mount: mount,
        setHash: setHash
    };
})(typeof window !== 'undefined' ? window : globalThis);
