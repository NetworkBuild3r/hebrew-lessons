/**
 * Today page — next-step cards from server state + filtered catalog.
 * Provenance: INIT-002/SPEC-014
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

    function lessonStatus(state, now, activityCount) {
        if (U && typeof U.lessonStatus === 'function') {
            return U.lessonStatus(state, now, activityCount);
        }
        return { kind: 'new', rank: 1, tone: 'fresh', he: '', en: '', chipHe: 'חדש', chipEn: 'new' };
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

    function hasLessonProgress(state) {
        if (!state || typeof state !== 'object') return false;
        return lessonStatus(state).kind !== 'new';
    }

    function isBrandNew(progressBySlug, reviewStore) {
        var words = reviewStore && reviewStore.words;
        if (words && Object.keys(words).length > 0) return false;
        var list = catalog();
        for (var i = 0; i < list.length; i += 1) {
            if (hasLessonProgress(progressBySlug[list[i].slug])) return false;
        }
        return true;
    }

    function filterDueQueue(rawQueue, words) {
        return (rawQueue || []).filter(function (item) {
            if (!item || item.direction !== 'en_he') return true;
            var he = words && words[item.key] && words[item.key].he_en;
            return !!(he && he.lastResult === 'pass');
        });
    }

    function dueSummary(reviewStore, now) {
        var Review = root.HebrewReview;
        if (!Review || typeof Review.dueList !== 'function') {
            return { dueCount: 0, newCount: 0, nextDueMs: null };
        }
        var store = reviewStore || { v: 1, words: {} };
        var words = store.words || {};
        var raw = Review.dueList(store, now);
        var queue = filterDueQueue(raw, words);
        var newKeys = Object.create(null);
        queue.forEach(function (item) {
            if (item && item.isNew) newKeys[item.key] = true;
        });
        var nextDueMs = null;
        Object.keys(words).forEach(function (key) {
            var entry = words[key];
            if (!entry) return;
            ['he_en', 'en_he'].forEach(function (dir) {
                var due = entry[dir] && entry[dir].due;
                if (!due) return;
                var ms = Date.parse(String(due));
                if (!Number.isFinite(ms)) return;
                if (nextDueMs == null || ms < nextDueMs) nextDueMs = ms;
            });
        });
        return {
            dueCount: queue.length,
            newCount: Object.keys(newKeys).length,
            nextDueMs: nextDueMs
        };
    }

    function formatDueDate(ms) {
        if (ms == null || !Number.isFinite(ms)) {
            return bi('אין תאריך עדיין', 'No next due yet');
        }
        var d = new Date(ms);
        var he = d.toLocaleString('he-IL', {
            weekday: 'long', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
        });
        var en = d.toLocaleString('en-US', {
            weekday: 'long', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit'
        });
        return esc(he) + ' <span class="en" dir="ltr">(' + esc(en) + ')</span>';
    }

    function lessonHref(slug) {
        return '/worksheets/' + encodeURIComponent(String(slug || '')) + '/';
    }

    function pickContinue(list, progressBySlug, now) {
        var best = null;
        for (var i = 0; i < list.length; i += 1) {
            var lesson = list[i];
            var state = progressBySlug[lesson.slug];
            var status = lessonStatus(state, now, activityCountFor(lesson));
            if (status.kind !== 'in_progress' && status.kind !== 'due') continue;
            var opened = Number(state && state.roundStartedAt) || 0;
            if (!best || opened >= best.opened) {
                best = { lesson: lesson, status: status, opened: opened };
            }
        }
        return best;
    }

    function pickNextNew(list, progressBySlug, now) {
        for (var i = 0; i < list.length; i += 1) {
            var lesson = list[i];
            var status = lessonStatus(progressBySlug[lesson.slug], now, activityCountFor(lesson));
            if (status.kind === 'new') return lesson;
        }
        return null;
    }

    function renderLoading() {
        return '<div class="card today-state" data-today-state="loading">'
            + '<p>' + bi('טוען…', 'Loading…') + '</p>'
            + '</div>';
    }

    function renderError(message) {
        return '<div class="card today-state" data-today-state="error" role="alert">'
            + '<p>' + bi('שגיאה בטעינת היום.', 'Could not load Today.') + '</p>'
            + '<p class="today-error-detail en" dir="ltr">' + esc(message || 'Something went wrong.') + '</p>'
            + '</div>';
    }

    function renderReviewCard(summary, brandNew) {
        if (brandNew) return '';
        if (summary.dueCount > 0) {
            return ''
                + '<article class="lesson-card review-today-card today-card" id="today-review-card" data-today-card="review">'
                + '<div class="lesson-link today-card-body">'
                + '<span class="lesson-status ready">' + bi('מוכן לחזרה', 'Ready to review') + '</span>'
                + '<h2>' + bi('חזרה', 'Review') + '</h2>'
                + '<p>'
                + '<span id="review-due-count">' + esc(String(summary.dueCount)) + '</span> '
                + esc('מילים לחזרה') + ' <span class="en" dir="ltr">(' + esc('due') + ')</span>'
                + ' · '
                + '<span id="review-new-count">' + esc(String(summary.newCount)) + '</span> '
                + esc('חדשות') + ' <span class="en" dir="ltr">(' + esc('new') + ')</span>'
                + '</p>'
                + '<p class="today-actions">'
                + '<a class="btn btn-primary" id="today-review-start" href="/review/">'
                + bi('התחילו', 'Start') + '</a>'
                + '</p>'
                + '</div></article>';
        }
        return ''
            + '<article class="lesson-card today-card today-nothing-due" id="today-review-card" data-today-card="review" data-review-state="nothing-due">'
            + '<div class="lesson-link today-card-body">'
            + '<span class="lesson-status">' + bi('אין חזרה עכשיו', 'Nothing due') + '</span>'
            + '<h2>' + bi('חזרה', 'Review') + '</h2>'
            + '<p>' + bi('אין מילים לחזרה כרגע.', 'Nothing due right now.') + '</p>'
            + '<p class="fine">' + bi('החזרה הבאה', 'Next review') + ': '
            + formatDueDate(summary.nextDueMs) + '</p>'
            + '</div></article>';
    }

    function renderContinueCard(entry) {
        if (!entry) return '';
        var lesson = entry.lesson;
        var status = entry.status;
        return ''
            + '<article class="lesson-card today-card" id="today-continue-card" data-today-card="continue" data-lesson-slug="'
            + esc(lesson.slug) + '">'
            + '<div class="lesson-link today-card-body">'
            + '<span class="lesson-status ' + esc(status.tone) + '">'
            + esc(status.he) + ' <span class="en" dir="ltr">(' + esc(status.en) + ')</span></span>'
            + '<p class="fine">' + esc(lesson.subtitle || '')
            + (lesson.level ? ' · ' + esc(lesson.level) : '') + '</p>'
            + '<h2>' + esc(lesson.title) + '</h2>'
            + (lesson.blurb ? '<p>' + esc(lesson.blurb) + '</p>' : '')
            + '<p class="today-actions">'
            + '<a class="btn btn-primary" id="today-continue-link" href="' + esc(lessonHref(lesson.slug)) + '">'
            + bi('המשיכו', 'Continue') + '</a>'
            + '</p>'
            + '</div></article>';
    }

    function renderNextCard(lesson, brandNew) {
        if (!lesson) return '';
        var action = brandNew
            ? bi('התחילו בשיעור הראשון', 'Start your first lesson')
            : bi('שיעור חדש', 'Next new lesson');
        return ''
            + '<article class="lesson-card today-card" id="today-next-card" data-today-card="next" data-lesson-slug="'
            + esc(lesson.slug) + '">'
            + '<div class="lesson-link today-card-body">'
            + '<span class="lesson-status fresh">' + bi('חדש', 'New') + '</span>'
            + '<p class="fine">' + esc(lesson.subtitle || '')
            + (lesson.level ? ' · ' + esc(lesson.level) : '') + '</p>'
            + '<h2>' + esc(lesson.title) + '</h2>'
            + (lesson.blurb ? '<p>' + esc(lesson.blurb) + '</p>' : '')
            + '<p class="today-actions">'
            + '<a class="btn btn-primary" id="today-next-link" href="' + esc(lessonHref(lesson.slug)) + '">'
            + action + '</a>'
            + '</p>'
            + '</div></article>';
    }

    function renderWelcomeCard(brandNew) {
        if (!brandNew) return '';
        return ''
            + '<article class="lesson-card today-card today-welcome" id="today-welcome-card" data-today-card="welcome">'
            + '<div class="lesson-link today-card-body">'
            + '<h2>' + bi('ברוכים הבאים', 'Welcome') + '</h2>'
            + '<p>'
            + bi(
                'התחילו בשיעור הראשון. החזרה וההמשך יופיעו כאן אחרי שתלמדו קצת.',
                'Start with your first lesson. Review and Continue will show up here after you learn a bit.'
            )
            + '</p>'
            + '</div></article>';
    }

    function renderEmptyCatalog() {
        return '<div class="card today-state" data-today-state="no-lessons">'
            + '<p>' + bi('אין שיעורים זמינים עדיין.', 'No lessons are visible yet.') + '</p>'
            + '</div>';
    }

    function renderToday(model) {
        if (!model || model.state === 'loading') return renderLoading();
        if (model.state === 'error') return renderError(model.message);
        var list = model.catalog || [];
        if (!list.length) return renderEmptyCatalog();
        var parts = [];
        parts.push(renderReviewCard(model.review || {}, model.brandNew));
        parts.push(renderContinueCard(model.continueEntry));
        parts.push(renderNextCard(model.nextLesson, model.brandNew));
        parts.push(renderWelcomeCard(model.brandNew));
        var html = parts.join('');
        if (!html) {
            return '<div class="card today-state" data-today-state="idle">'
                + '<p>' + bi('אין מה לעשות עכשיו.', 'Nothing to do right now.') + '</p>'
                + '</div>';
        }
        return '<div class="today-list home-list" data-today-state="ready">' + html + '</div>';
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

    function buildModel(progressBySlug) {
        var now = Date.now();
        var list = catalog();
        var Review = root.HebrewReview;
        var reviewStore = { v: 1, words: {} };
        if (Review && typeof Review.load === 'function') {
            reviewStore = Review.load() || reviewStore;
        } else if (root.HebrewSync && typeof root.HebrewSync.getReviewState === 'function') {
            reviewStore = root.HebrewSync.getReviewState() || reviewStore;
        }
        var brandNew = isBrandNew(progressBySlug, reviewStore);
        return {
            state: 'ready',
            catalog: list,
            brandNew: brandNew,
            review: dueSummary(reviewStore, now),
            continueEntry: pickContinue(list, progressBySlug, now),
            nextLesson: pickNextNew(list, progressBySlug, now)
        };
    }

    function paint(el, model) {
        if (!el) return;
        el.innerHTML = renderToday(model);
    }

    function boot() {
        var el = root.document && root.document.getElementById('today-app');
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
            paint(el, buildModel(progressBySlug || Object.create(null)));
        }).catch(function (err) {
            paint(el, {
                state: 'error',
                message: (err && err.message) || 'Network error'
            });
        });
    }

    root.HebrewTodayPage = {
        render: renderToday,
        buildModel: buildModel,
        boot: boot
    };

    if (root.document) {
        if (root.document.readyState === 'loading') {
            root.document.addEventListener('DOMContentLoaded', boot);
        } else if (root.document.getElementById('today-app')) {
            boot();
        }
    }
})(typeof window !== 'undefined' ? window : globalThis);
