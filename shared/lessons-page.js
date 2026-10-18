/**
 * Lessons library — filtered catalog cards with status chips and level filter.
 * Provenance: INIT-002/SPEC-014
 * Published plan lessons (verb workshop, conversation, verse ...) are listed after the worksheet
 * lessons; guides stay on the Guides page. INIT-013 follow-up: a published plan lesson had no way
 * in from the app, because only the Guides page read /api/plans/catalog and it keeps recipe "guide".
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
            : ('<span class="ui-pair"><span class="ui-he" lang="he">' + esc(he) + '</span>'
                + '<span class="ui-en" lang="en" dir="ltr">' + esc(en) + '</span></span>');
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

    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    // Same labels as the plan player's recipe chip (shared/ui/plan-render.js).
    var PLAN_CHIP = {
        'topic-conversation': ['שיחה', 'Conversation'],
        'review-worksheet': ['חזרה', 'Review'],
        'verse-reading': ['פסוק', 'Verse'],
        'verb-workshop': ['פעלים', 'Verbs'],
        'letters-and-vowels': ['אותיות', 'Letters']
    };

    function planHref(planId, lessonId) {
        return '/shared/plan/?id=' + encodeURIComponent(planId) + '&lesson=' + encodeURIComponent(lessonId);
    }

    function pathHref(planId) {
        return '/shared/plan/?id=' + encodeURIComponent(planId);
    }

    function isHebrewText(value) {
        return /[\u0590-\u05FF]/.test(String(value || ''));
    }

    /**
     * Published plan lessons the catalog returned, minus guides. A plan id or lesson id that is not
     * a slug is dropped so it cannot become a URL.
     * @param {unknown} lessons
     * @returns {object[]}
     */
    function planRowsOnly(lessons) {
        if (!Array.isArray(lessons)) return [];
        var out = [];
        for (var i = 0; i < lessons.length && out.length < 200; i += 1) {
            var row = lessons[i];
            if (!row || typeof row !== 'object' || row.recipe === 'guide') continue;
            if (typeof row.planId !== 'string' || !SLUG.test(row.planId)) continue;
            if (typeof row.lessonId !== 'string' || !SLUG.test(row.lessonId)) continue;
            out.push(row);
        }
        return out;
    }

    /**
     * One entry per plan, in catalog order. A non-slug id is skipped so it cannot become a URL.
     * Counts come from the rows already loaded. The title is the plan title when the catalog sent one.
     */
    function pathSummaries(lessons) {
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

    function pathMeta(entry) {
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

    function renderPaths(planIds) {
        if (!planIds || !planIds.length) return '';
        var seen = [];
        var cards = '';
        for (var i = 0; i < planIds.length; i += 1) {
            var item = planIds[i];
            var id = typeof item === 'string' ? item : (item && item.id);
            if (typeof id !== 'string' || !SLUG.test(id) || seen.indexOf(id) >= 0) continue;
            seen.push(id);
            var title = item && typeof item.title === 'string' && item.title ? item.title : 'מסלול לימוד';
            cards += '<article class="lesson-card" data-learning-path="' + esc(id) + '">'
                + '<a class="lesson-link" href="' + esc(pathHref(id)) + '">'
                + '<span class="path-copy">'
                + '<h2 class="path-title"><bdi lang="he">' + esc(title) + '</bdi></h2>'
                + pathMeta(typeof item === 'string' ? null : item)
                + '</span>'
                + '<span class="path-chevron" aria-hidden="true"></span>'
                + '</a></article>';
        }
        if (!cards) return '';
        return '<div class="home-list lessons-paths">' + cards + '</div>';
    }

    function renderPlanCard(row) {
        var title = String(row.title || row.lessonId);
        var dir = isHebrewText(title) ? 'rtl' : 'ltr';
        var lang = dir === 'rtl' ? 'he' : 'en';
        var chip = Object.prototype.hasOwnProperty.call(PLAN_CHIP, row.recipe) ? PLAN_CHIP[row.recipe] : null;
        var chipHtml = chip
            ? '<span class="recipe-chip" data-recipe="' + esc(row.recipe) + '">' + bi(chip[0], chip[1]) + '</span>'
            : '';
        var topic = row.topic ? '<p class="fine">' + esc(row.topic) + '</p>' : '';
        return '<article class="lesson-card" data-plan-id="' + esc(row.planId)
            + '" data-lesson-id="' + esc(row.lessonId) + '"'
            + (chip ? ' data-recipe="' + esc(row.recipe) + '"' : '') + '>'
            + '<a class="lesson-link" href="' + esc(planHref(row.planId, row.lessonId)) + '">'
            + chipHtml
            + topic
            + '<h2 lang="' + lang + '" dir="' + dir + '">' + esc(title) + '</h2>'
            + '</a></article>';
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
        // Hebrew only: one language per label (REQ-016). The visible buttons carry the glosses.
        return '<div class="lessons-filter" role="group" aria-label="'
            + esc('סינון לפי רמה') + '">' + buttons + '</div>';
    }

    var LEVEL_LABELS = {
        basic: { he: 'בסיסי', en: 'Basic' },
        intermediate: { he: 'בינוני', en: 'Intermediate' },
        advanced: { he: 'מתקדם', en: 'Advanced' }
    };

    /**
     * Subtitle line under the status chip: the catalog subtitle, then the level as a Hebrew word
     * with the English gloss (REQ-016). The catalog sometimes already ends the subtitle with the
     * raw English level word; that word is dropped so the level shows once.
     */
    function metaLine(lesson) {
        var level = normalizeLevel(lesson.level);
        var subtitle = String(lesson.subtitle || '');
        if (level) {
            subtitle = subtitle
                .replace(/[\s\u00b7\u2022|\u2013\u2014-]*\b(basic|intermediate|advanced)\s*$/i, '')
                .replace(/[\s\u00b7\u2022|\u2013\u2014-]+$/, '');
        }
        var label = LEVEL_LABELS[level];
        var levelHtml = label
            ? bi(label.he, label.en)
            : (lesson.level ? esc(lesson.level) : '');
        var parts = [];
        if (subtitle.trim()) parts.push(esc(subtitle.trim()));
        if (levelHtml) parts.push('<span class="lesson-level">' + levelHtml + '</span>');
        return parts.join(' · ');
    }

    /** The catalog blurb is English text: mark it so punctuation and direction are right. */
    function blurbHtml(lesson) {
        return lesson.blurb
            ? '<p class="lesson-blurb" lang="en" dir="ltr">' + esc(lesson.blurb) + '</p>'
            : '';
    }

    function renderCard(lesson, status) {
        return ''
            + '<article class="lesson-card" data-lesson-slug="' + esc(lesson.slug)
            + '" data-lesson-level="' + esc(normalizeLevel(lesson.level))
            + '" data-lesson-status="' + esc(status.kind) + '">'
            + '<a class="lesson-link" href="' + esc(lessonHref(lesson.slug)) + '">'
            + '<span class="lesson-status ' + esc(status.tone) + '">'
            + bi(status.chipHe, status.chipEn)
            + '</span>'
            + '<p class="fine">' + metaLine(lesson) + '</p>'
            + '<h2>' + esc(lesson.title) + '</h2>'
            + blurbHtml(lesson)
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
        var planRows = model.planRows || [];
        var html = renderPaths(model.planIds) + renderFilter(filter);

        if (!rows.length && !planRows.length && filter === 'all') {
            return html + renderNoLessons();
        }

        var visible = rows;
        if (filter !== 'all') {
            visible = rows.filter(function (row) {
                return normalizeLevel(row.lesson.level) === filter;
            });
        }

        // Plan lessons carry no level, so they appear under "All" only.
        var plans = filter === 'all' ? planRows : [];
        if (!visible.length && !plans.length) {
            return html + renderEmptyFilter();
        }

        var sorted = sortRows(visible);
        html += '<div class="home-list lessons-list" data-lessons-state="ready">';
        html += sorted.map(function (row) {
            return renderCard(row.lesson, row.status);
        }).join('');
        html += plans.map(renderPlanCard).join('');
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

    function emptyCatalog() {
        return { rows: [], planIds: [] };
    }

    function logCatalog(message, detail) {
        if (root.console && typeof root.console.error === 'function') {
            root.console.error('[lessons] ' + message, detail);
        }
    }

    /**
     * Published plan lessons this session may read, plus one id per plan for the path link.
     * An error here never hides the worksheet lessons.
     */
    function loadPlanRows() {
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') return Promise.resolve(emptyCatalog());
        return Api.get('/api/plans/catalog', { redirectOn401: false }).then(function (res) {
            var lessons = res && res.ok && res.data && Array.isArray(res.data.lessons)
                ? res.data.lessons
                : null;
            if (!lessons) {
                logCatalog('plan catalog unusable', {
                    status: res && res.status,
                    error: res && res.error
                });
                return emptyCatalog();
            }
            return { rows: planRowsOnly(lessons), planIds: pathSummaries(lessons) };
        }).catch(function (err) {
            logCatalog('plan catalog failed', err);
            return emptyCatalog();
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

    // ---- view lifecycle (contract: shared/router.js) ----
    // `mounted` is the element of the open view, or null once the route closes.
    var mounted = null;

    function mount(el, ctx) {
        mounted = el;
        var ready = (ctx && ctx.ready) || Promise.resolve(null);
        return ready.then(function () {
            return Promise.all([loadProgressMap(catalog()), loadPlanRows()]);
        }).then(function (loaded) {
            if (mounted !== el) return;
            var progressBySlug = loaded[0];
            var catalogLoad = loaded[1] || emptyCatalog();
            var model = {
                state: 'ready',
                filter: 'all',
                rows: buildRows(progressBySlug || Object.create(null), Date.now()),
                planRows: catalogLoad.rows || [],
                planIds: catalogLoad.planIds || []
            };
            paint(el, model);
            bindFilter(el, model);
        }).catch(function (err) {
            if (mounted !== el) return;
            paint(el, { state: 'error', message: (err && err.message) || 'Network error' });
        });
    }

    function unmount() {
        mounted = null;
    }

    root.HebrewLessonsPage = {
        render: renderList,
        buildRows: buildRows,
        planRowsOnly: planRowsOnly,
        mount: mount,
        unmount: unmount
    };
})(typeof window !== 'undefined' ? window : globalThis);
