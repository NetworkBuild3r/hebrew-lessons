/**
 * Lessons library — filtered catalog cards with status chips and level filter.
 * Provenance: INIT-002/SPEC-014
 */
(function (root) {
    'use strict';

    var U = root.HebrewUtils;
    var LEVELS = ['all', 'basic', 'intermediate', 'advanced'];

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

    function lessonStatus(state, now, activityCount) {
        if (U && typeof U.lessonStatus === 'function') {
            return U.lessonStatus(state, now, activityCount);
        }
        return {
            kind: 'new',
            rank: 1,
            tone: 'fresh',
            chipHe: 'חדש',
            chipEn: 'new',
            he: '',
            en: ''
        };
    }

    function catalog() {
        return Array.isArray(root.HEBREW_CATALOG) ? root.HEBREW_CATALOG : [];
    }

    function activityCountFor(lesson) {
        if (!lesson || typeof lesson !== 'object') return null;
        if (Number.isInteger(lesson.activityCount)) return lesson.activityCount;
        if (Array.isArray(lesson.activities)) return lesson.activities.length;
        return null;
    }

    function normalizeLevel(raw) {
        var level = String(raw || '').toLowerCase();
        if (level === 'basic' || level === 'a1' || level === 'beginner') return 'basic';
        if (level === 'advanced' || level === 'b1' || level === 'b2') return 'advanced';
        if (level === 'intermediate' || level === 'a2') return 'intermediate';
        return level || '';
    }

    function lessonHref(slug) {
        return '/worksheets/' + encodeURIComponent(String(slug || '')) + '/';
    }

    function renderLoading() {
        return '<div class="card lessons-state" data-lessons-state="loading">'
            + '<p>' + bi('טוען…', 'Loading…') + '</p>'
            + '</div>';
    }

    function renderError(message) {
        return '<div class="card lessons-state" data-lessons-state="error" role="alert">'
            + '<p>' + bi('שגיאה בטעינת השיעורים.', 'Could not load lessons.') + '</p>'
            + '<p class="lessons-error-detail en" dir="ltr">' + esc(message || 'Something went wrong.') + '</p>'
            + '</div>';
    }

    function renderFilter(active) {
        var current = LEVELS.indexOf(active) >= 0 ? active : 'all';
        var labels = {
            all: { he: 'הכול', en: 'All' },
            basic: { he: 'בסיסי', en: 'Basic' },
            intermediate: { he: 'בינוני', en: 'Intermediate' },
            advanced: { he: 'מתקדם', en: 'Advanced' }
        };
        var buttons = LEVELS.map(function (level) {
            var meta = labels[level];
            var pressed = level === current ? 'true' : 'false';
            var cls = 'lessons-filter-btn' + (level === current ? ' is-active' : '');
            return '<button type="button" class="' + cls + '" data-level-filter="' + esc(level)
                + '" aria-pressed="' + pressed + '">'
                + bi(meta.he, meta.en)
                + '</button>';
        }).join('');
        return '<div class="lessons-filter" role="group" aria-label="'
            + esc('Level filter') + '">' + buttons + '</div>';
    }

    function renderCard(lesson, status) {
        return ''
            + '<article class="lesson-card" data-lesson-slug="' + esc(lesson.slug)
            + '" data-lesson-level="' + esc(normalizeLevel(lesson.level))
            + '" data-lesson-status="' + esc(status.kind) + '">'
            + '<a class="lesson-link" href="' + esc(lessonHref(lesson.slug)) + '">'
            + '<span class="lesson-status ' + esc(status.tone) + '">'
            + esc(status.chipHe) + ' <span class="en" dir="ltr">(' + esc(status.chipEn) + ')</span>'
            + '</span>'
            + '<p class="fine">' + esc(lesson.subtitle || '')
            + (lesson.level ? ' · ' + esc(lesson.level) : '') + '</p>'
            + '<h2>' + esc(lesson.title) + '</h2>'
            + (lesson.blurb ? '<p>' + esc(lesson.blurb) + '</p>' : '')
            + '</a></article>';
    }

    function renderEmptyFilter() {
        return '<div class="card lessons-state" data-lessons-state="filtered-empty">'
            + '<p>' + bi('אין שיעורים ברמה הזו.', 'No lessons at this level.') + '</p>'
            + '</div>';
    }

    function renderNoLessons() {
        return '<div class="card lessons-state" data-lessons-state="no-lessons">'
            + '<p>' + bi('אין שיעורים זמינים.', 'No lessons are visible.') + '</p>'
            + '</div>';
    }

    function sortRows(rows) {
        return rows.slice().sort(function (a, b) {
            if (a.status.rank !== b.status.rank) return a.status.rank - b.status.rank;
            return a.index - b.index;
        });
    }

    function renderList(model) {
        if (!model || model.state === 'loading') return renderLoading();
        if (model.state === 'error') return renderError(model.message);

        var filter = model.filter || 'all';
        var rows = model.rows || [];
        var html = renderFilter(filter);

        if (!rows.length && filter === 'all') {
            return html + renderNoLessons();
        }

        var visible = rows;
        if (filter !== 'all') {
            visible = rows.filter(function (row) {
                return normalizeLevel(row.lesson.level) === filter;
            });
        }

        if (!visible.length) {
            return html + renderEmptyFilter();
        }

        var sorted = sortRows(visible);
        html += '<div class="home-list lessons-list" data-lessons-state="ready">';
        html += sorted.map(function (row) {
            return renderCard(row.lesson, row.status);
        }).join('');
        html += '</div>';
        return html;
    }

    function loadProgressMap(list) {
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') {
            return Promise.reject(new Error('API unavailable'));
        }
        return Promise.all(list.map(function (lesson) {
            var slug = lesson && lesson.slug;
            if (!slug) return Promise.resolve({ slug: '', state: {} });
            return Api.get('/api/lessons/' + encodeURIComponent(slug) + '/progress', { redirectOn401: false })
                .then(function (res) {
                    if (res && res.ok && res.data && res.data.state && typeof res.data.state === 'object') {
                        return { slug: slug, state: res.data.state };
                    }
                    return { slug: slug, state: {} };
                })
                .catch(function () {
                    return { slug: slug, state: {} };
                });
        })).then(function (rows) {
            var map = Object.create(null);
            rows.forEach(function (row) {
                if (row && row.slug) map[row.slug] = row.state || {};
            });
            return map;
        });
    }

    function buildRows(progressBySlug, now) {
        var list = catalog();
        var t = now != null ? now : Date.now();
        return list.map(function (lesson, index) {
            return {
                lesson: lesson,
                index: index,
                status: lessonStatus(progressBySlug[lesson.slug], t, activityCountFor(lesson))
            };
        });
    }

    function paint(el, model) {
        if (!el) return;
        el.innerHTML = renderList(model);
    }

    function bindFilter(el, model) {
        if (!el) return;
        el.onclick = function (ev) {
            var target = ev.target;
            while (target && target !== el && !(target.getAttribute && target.getAttribute('data-level-filter'))) {
                target = target.parentNode;
            }
            if (!target || target === el) return;
            var level = target.getAttribute('data-level-filter');
            if (!level || LEVELS.indexOf(level) < 0) return;
            model.filter = level;
            paint(el, model);
            bindFilter(el, model);
        };
    }

    function boot() {
        var el = root.document && root.document.getElementById('lessons-app');
        if (!el) return;
        paint(el, { state: 'loading' });

        var Sync = root.HebrewSync;
        var start = Sync && typeof Sync.init === 'function'
            ? Promise.resolve(Sync.init({}))
            : Promise.resolve(null);

        start.then(function () {
            if (Sync && typeof Sync.paintStatus === 'function') Sync.paintStatus();
            return loadProgressMap(catalog());
        }).then(function (progressBySlug) {
            var model = {
                state: 'ready',
                filter: 'all',
                rows: buildRows(progressBySlug || Object.create(null), Date.now())
            };
            paint(el, model);
            bindFilter(el, model);
        }).catch(function (err) {
            paint(el, {
                state: 'error',
                message: (err && err.message) || 'Network error'
            });
        });
    }

    root.HebrewLessonsPage = {
        render: renderList,
        buildRows: buildRows,
        boot: boot
    };

    if (root.document) {
        if (root.document.readyState === 'loading') {
            root.document.addEventListener('DOMContentLoaded', boot);
        } else if (root.document.getElementById('lessons-app')) {
            boot();
        }
    }
})(typeof window !== 'undefined' ? window : globalThis);
