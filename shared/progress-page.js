/**
 * Progress page — memory stages and direction accuracy.
 * Provenance: INIT-002/SPEC-015
 * Renders GET /api/me/metrics; does not recompute metrics.
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

    var DIR_LABELS = {
        he_en: { he: 'עברית → אנגלית', en: 'Hebrew → English' },
        en_he: { he: 'אנגלית → עברית', en: 'English → Hebrew' }
    };

    var STAGE_KEYS = [
        { key: 'new', he: 'חדש', en: 'New', cls: 'progress-seg-new' },
        { key: 'learning', he: 'בלמידה', en: 'Learning', cls: 'progress-seg-learning' },
        { key: 'longTerm', he: 'לטווח ארוך', en: 'Long-term', cls: 'progress-seg-long' }
    ];

    function stageTotal(stages) {
        if (!stages) return 0;
        var dirs = ['he_en', 'en_he'];
        var total = 0;
        for (var i = 0; i < dirs.length; i += 1) {
            var s = stages[dirs[i]] || {};
            total += Number(s.new) || 0;
            total += Number(s.learning) || 0;
            total += Number(s.longTerm) || 0;
        }
        return total;
    }

    function hasAnyPct(accuracy) {
        if (!accuracy) return false;
        var dirs = ['he_en', 'en_he'];
        for (var i = 0; i < dirs.length; i += 1) {
            var a = accuracy[dirs[i]];
            if (a && a.pct != null && typeof a.pct === 'number' && !isNaN(a.pct)) {
                return true;
            }
        }
        return false;
    }

    function resolveState(metrics) {
        if (!metrics || metrics.state === 'loading') return 'loading';
        if (metrics.state === 'error') return 'error';
        if (!metrics.stages || stageTotal(metrics.stages) === 0) return 'empty';
        if (!hasAnyPct(metrics.accuracy)) return 'low-data';
        return 'ready';
    }

    function renderLoading() {
        return '<div class="card progress-card" data-progress-state="loading">'
            + '<p>' + bi('טוען…', 'Loading…') + '</p>'
            + '</div>';
    }

    function renderError(message) {
        var msg = message || 'Something went wrong.';
        return '<div class="card progress-card" data-progress-state="error" role="alert">'
            + '<p>' + bi('שגיאה בטעינת ההתקדמות.', 'Could not load progress.') + '</p>'
            + '<p class="progress-error-detail en" dir="ltr">' + esc(msg) + '</p>'
            + '</div>';
    }

    function renderEmpty() {
        return '<div class="card progress-card progress-empty" data-progress-state="empty">'
            + '<h2>' + bi('עדיין אין חזרות.', 'No reviews yet.') + '</h2>'
            + '<p>'
            + esc('התחילו ב') + ' <span class="en" dir="ltr">(Start with)</span> '
            + '<a href="/">' + bi('היום', 'Today') + '</a> '
            + esc('או') + ' <span class="en" dir="ltr">(or)</span> '
            + '<a href="/lessons/">' + bi('שיעורים', 'Lessons') + '</a>.'
            + '</p>'
            + '</div>';
    }

    function stageCountsText(stages) {
        var parts = [];
        for (var i = 0; i < STAGE_KEYS.length; i += 1) {
            var meta = STAGE_KEYS[i];
            var n = Number(stages[meta.key]) || 0;
            parts.push(
                '<span class="progress-count" data-stage="' + esc(meta.key) + '">'
                + '<span class="progress-count-n">' + esc(String(n)) + '</span> '
                + bi(meta.he, meta.en)
                + '</span>'
            );
        }
        return parts.join('<span class="progress-count-sep" aria-hidden="true"> · </span>');
    }

    function stageBar(stages) {
        var total = (Number(stages.new) || 0)
            + (Number(stages.learning) || 0)
            + (Number(stages.longTerm) || 0);
        var segs = [];
        for (var i = 0; i < STAGE_KEYS.length; i += 1) {
            var meta = STAGE_KEYS[i];
            var n = Number(stages[meta.key]) || 0;
            var pct = total > 0 ? (n / total) * 100 : 0;
            segs.push(
                '<span class="progress-seg ' + meta.cls + '" style="width:' + esc(String(pct)) + '%;" '
                + 'title="' + esc(meta.he + ' (' + meta.en + '): ' + n) + '" '
                + 'data-stage="' + esc(meta.key) + '" data-count="' + esc(String(n)) + '"></span>'
            );
        }
        return '<div class="progress-stage-bar" role="img" aria-label="'
            + esc('Memory stages') + '">'
            + segs.join('')
            + '</div>';
    }

    function renderDirectionStages(dir, stages) {
        var label = DIR_LABELS[dir];
        var s = stages || { new: 0, learning: 0, longTerm: 0 };
        return '<section class="progress-direction" data-direction="' + esc(dir) + '">'
            + '<h3 class="progress-dir-label">' + bi(label.he, label.en) + '</h3>'
            + stageBar(s)
            + '<p class="progress-stage-counts">' + stageCountsText(s) + '</p>'
            + '</section>';
    }

    function formatPct(pct) {
        return String(Math.round(Number(pct) * 100)) + '%';
    }

    function renderAccuracy(dir, acc) {
        var label = DIR_LABELS[dir];
        var a = acc || { pass: 0, miss: 0, pct: null };
        var attempts = (Number(a.pass) || 0) + (Number(a.miss) || 0);
        var body;
        var stateAttr;
        if (a.pct == null) {
            stateAttr = 'low-data';
            body = '<p class="progress-low-data" data-accuracy-state="low-data">'
                + bi('עדיין אין מספיק תשובות.', 'Not enough answers yet.')
                + '</p>'
                + '<p class="progress-attempts">'
                + bi('ניסיונות', 'Attempts') + ': '
                + '<span class="progress-attempt-n" data-attempts="' + esc(String(attempts)) + '">'
                + esc(String(attempts)) + '</span></p>';
        } else {
            stateAttr = 'ready';
            body = '<p class="progress-accuracy-pct" data-accuracy-state="ready">'
                + '<span class="progress-pct" data-pct="' + esc(String(a.pct)) + '">'
                + esc(formatPct(a.pct)) + '</span>'
                + ' <span class="progress-attempts">('
                + bi('ניסיונות', 'Attempts') + ': '
                + '<span class="progress-attempt-n" data-attempts="' + esc(String(attempts)) + '">'
                + esc(String(attempts)) + '</span>)</span></p>';
        }
        return '<section class="progress-accuracy-dir" data-direction="' + esc(dir)
            + '" data-accuracy-state="' + stateAttr + '">'
            + '<h3 class="progress-dir-label">' + bi(label.he, label.en) + '</h3>'
            + body
            + '</section>';
    }

    function renderMetrics(metrics) {
        var state = resolveState(metrics);
        if (state === 'empty') return renderEmpty();

        var stages = metrics.stages || {};
        var accuracy = metrics.accuracy || {};
        var html = '<div class="progress-view" data-progress-state="' + esc(state) + '">';

        html += '<section class="card progress-card progress-stages">';
        html += '<h2>' + bi('מילים לפי שלב זיכרון', 'Words by memory stage') + '</h2>';
        html += '<p class="progress-long-term-note">'
            + bi(
                'לטווח ארוך = החזרה הבאה בעוד שבועיים או יותר.',
                'Long-term = next review 2+ weeks away.'
            )
            + '</p>';
        html += renderDirectionStages('he_en', stages.he_en);
        html += renderDirectionStages('en_he', stages.en_he);
        html += '<p class="progress-why">'
            + bi(
                'מילים עוברות לטווח ארוך אחרי שנזכרים בהן בימים נפרדים.',
                'Words move to long-term after you recall them on separate days.'
            )
            + '</p>';
        html += '</section>';

        html += '<section class="card progress-card progress-accuracy">';
        html += '<h2>' + bi('דיוק בשלושים הימים האחרונים', 'Recall accuracy, last 30 days') + '</h2>';
        html += renderAccuracy('he_en', accuracy.he_en);
        html += renderAccuracy('en_he', accuracy.en_he);
        html += '<p class="progress-why">'
            + bi(
                'עברית לאנגלית ואנגלית לעברית בונות ידע שונה.',
                'Hebrew to English and English to Hebrew build different knowledge.'
            )
            + '</p>';
        html += '</section>';

        html += '</div>';
        return html;
    }

    /**
     * @param {HTMLElement|{ innerHTML: string }} el
     * @param {object|null|undefined} metrics
     *   API shape { stages, accuracy }, or { state: 'loading'|'error', message?: string }
     */
    function render(el, metrics) {
        if (!el) return;
        var state = resolveState(metrics);
        var html;
        if (state === 'loading') {
            html = renderLoading();
        } else if (state === 'error') {
            html = renderError(metrics && metrics.message);
        } else {
            html = renderMetrics(metrics);
        }
        el.innerHTML = html;
    }

    function boot() {
        var el = root.document && root.document.getElementById('progress-app');
        if (!el) return;
        render(el, { state: 'loading' });
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') {
            render(el, { state: 'error', message: 'API unavailable' });
            return;
        }
        Api.get('/api/me/metrics').then(function (res) {
            if (!res || !res.ok || !res.data) {
                render(el, {
                    state: 'error',
                    message: (res && res.error) || ('HTTP ' + ((res && res.status) || '?'))
                });
                return;
            }
            render(el, res.data);
        }).catch(function (err) {
            render(el, {
                state: 'error',
                message: (err && err.message) || 'Network error'
            });
        });
    }

    root.HebrewProgressView = {
        render: render,
        resolveState: resolveState
    };

    if (root.document) {
        if (root.document.readyState === 'loading') {
            root.document.addEventListener('DOMContentLoaded', boot);
        } else if (root.document.getElementById('progress-app')) {
            boot();
        }
    }
})(typeof window !== 'undefined' ? window : globalThis);
