/**
 * The route table for the signed-in app (ADR-0007). This is the ONLY place a
 * screen is added, renamed, or removed.
 *
 * HOW TO ADD A SCREEN (read before editing):
 *   1. Write the page module in shared/ as an IIFE that sets one global, e.g.
 *      `root.HebrewThingPage = { mount: function (el, ctx) {...}, unmount: function () {...} }`.
 *      It must NOT auto-run on load. The router calls mount() when the route opens.
 *   2. Add an entry below. `scripts` lists what the screen needs beyond the core
 *      scripts already in shared/app.html, in load order. The router loads each
 *      URL once per visit and reuses it.
 *   3. Add the path to APP_PATHS in server/spa.js so a refresh or a typed URL
 *      gets the shell instead of a 404.
 *   4. If the screen needs a nav link, add it to NAV in shared/app-shell.js with
 *      the same `id`.
 *   Do not create a new top-level HTML file for a screen. That is what made
 *   every click reload the page.
 *
 * Fields:
 *   id         matches HebrewShell NAV id (drives aria-current)
 *   path       canonical URL, always with a trailing slash except "/"
 *   title      document.title
 *   announce   short name read by screen readers after navigation
 *   bodyClass  class put on <body> while this route is open (old CSS hooks)
 *   rootId     id of the view root; page modules and Chrome smokes look it up
 *   loadingHook data-* name for the loading card (e.g. "today-state")
 *   heading    { he, en, subHe, subEn, className, subtitleId }
 *   scripts    extra script URLs (absolute paths), in order
 *   view       global name of the page module, resolved after scripts load
 *   adminOnly  true → only admins can open it (the server also enforces this)
 *
 * Provenance: INIT-005/SPEC-002
 */
(function (root) {
    'use strict';

    var SITE = ' · דרך עברית';

    var SPEECH_AND_WORD_DATA = [
        '/shared/player/speech.js',
        '/shared/player/feedback.js',
        '/shared/word-audio-catalog.js',
        '/worksheets/_audio/words/catalog.js',
        '/worksheets/_review/vocab-index.js'
    ];

    root.HebrewAppRoutes = [
        {
            id: 'today',
            path: '/',
            title: 'היום — Today' + SITE,
            announce: 'היום',
            bodyClass: 'today-page',
            rootId: 'today-app',
            loadingHook: 'today-state',
            heading: { he: 'היום', en: 'Today', subHe: 'הצעד הבא שלכם — קצר וברור.', subEn: 'Your next step — short and clear.' },
            scripts: [
                '/shared/ui/blocks/conjugation-table.js',
                '/shared/verb-practice.js',
                '/shared/today-page.js'
            ],
            view: 'HebrewTodayPage'
        },
        {
            id: 'lessons',
            path: '/lessons/',
            title: 'שיעורים — Lessons' + SITE,
            announce: 'שיעורים',
            bodyClass: 'lessons-page',
            rootId: 'lessons-app',
            loadingHook: 'lessons-state',
            heading: {
                he: 'שיעורים',
                en: 'Lessons',
                subHe: 'עולמות לבחירה — בוקר, דייטים, חיים.',
                subEn: 'Pick your world — morning, dating, real life.'
            },
            scripts: ['/shared/lessons-page.js'],
            view: 'HebrewLessonsPage'
        },
        {
            id: 'guides',
            path: '/guides/',
            title: 'מדריכים — Guides' + SITE,
            announce: 'מדריכים',
            bodyClass: 'guides-page',
            rootId: 'guides-app',
            loadingHook: 'guides-state',
            heading: {
                he: 'מדריכים',
                en: 'Guides',
                subHe: 'שיעורי הסבר — קוראים ומקשיבים.',
                subEn: 'Explanation lessons — read and listen.'
            },
            scripts: ['/shared/guides-page.js'],
            view: 'HebrewGuidesPage'
        },
        {
            id: 'verbs',
            path: '/verbs/',
            title: 'פעלים — Verbs' + SITE,
            announce: 'פעלים',
            bodyClass: 'verbs-page',
            rootId: 'verbs-app',
            loadingHook: 'verbs-state',
            heading: {
                he: 'פעלים',
                en: 'Verbs',
                subHe: 'סבב קצר בכל יום — זוכרים, לא מעתיקים.',
                subEn: 'A short round each day: recall the form, then see the table.'
            },
            scripts: [
                '/shared/ui/blocks/conjugation-table.js',
                '/shared/verb-practice.js',
                '/shared/verbs-page.js'
            ],
            view: 'HebrewVerbsPage'
        },
        {
            id: 'review',
            path: '/review/',
            title: 'חזרה היום — Review today' + SITE,
            announce: 'חזרה היום',
            bodyClass: 'review-page',
            rootId: 'review-app',
            heading: {
                he: 'חזרה היום',
                en: 'Review today',
                subHe: 'מילים שמחכות לכם — כמה דקות וסיימתם.',
                subEn: 'Words waiting for you — a few minutes and done.',
                className: 'review-top',
                subtitleId: 'review-subtitle'
            },
            // Script tags only for vocab data (ADR-0001 constraint 2): never fetch() it.
            scripts: SPEECH_AND_WORD_DATA.concat([
                '/shared/player/flashcard-answer.js',
                '/shared/review-page.js'
            ]),
            view: 'HebrewReviewPage'
        },
        {
            id: 'words',
            path: '/words/',
            title: 'מילים — Words' + SITE,
            announce: 'מילים',
            bodyClass: 'words-page',
            rootId: 'words-app',
            loadingHook: 'words-state',
            heading: {
                he: 'מילים',
                en: 'Words',
                subHe: 'הספרייה שלכם — חיפוש, תגיות, והוספה.',
                subEn: 'Your library — search, tags, and add.'
            },
            scripts: ['/shared/player/speech.js', '/shared/player/feedback.js', '/shared/words-page.js'],
            view: 'HebrewWordsPage'
        },
        {
            id: 'progress',
            path: '/progress/',
            title: 'התקדמות — Progress' + SITE,
            announce: 'התקדמות',
            bodyClass: 'progress-page',
            rootId: 'progress-app',
            loadingHook: 'progress-state',
            heading: {
                he: 'התקדמות',
                en: 'Progress',
                subHe: 'מה כבר שלכם — מילים ושלבים.',
                subEn: 'What is already yours — words and stages.',
                className: 'progress-top'
            },
            scripts: ['/shared/progress-page.js'],
            view: 'HebrewProgressView'
        },
        {
            id: 'admin',
            path: '/admin/',
            title: 'ניהול — Admin' + SITE,
            announce: 'ניהול',
            bodyClass: 'admin-page',
            rootId: 'admin-app',
            loadingHook: 'admin-boot',
            adminOnly: true,
            heading: {
                he: 'ניהול',
                en: 'Admin',
                subHe: 'משתמשים, שיעורים והתקדמות.',
                subEn: 'Users, lessons, and progress.'
            },
            scripts: SPEECH_AND_WORD_DATA.concat([
                '/shared/progress-page.js',
                '/shared/activities/catalog.js',
                '/shared/admin/users.js',
                '/shared/admin/lessons.js',
                '/shared/admin/lesson-editor.js',
                '/shared/admin/admin-shell.js'
            ]),
            view: 'HebrewAdminShell'
        },
        {
            id: 'editor',
            path: '/editor/',
            title: 'עריכה — Editor' + SITE,
            announce: 'עריכה',
            bodyClass: 'editor-page',
            rootId: 'editor-app',
            loadingHook: 'editor-boot',
            adminOnly: true,
            heading: {
                he: 'עריכה',
                en: 'Editor',
                subHe: 'תוכניות שיעור — מחסנית ומבנים.',
                subEn: 'Drafts and published plans. Open one to edit the sequence, activities, and learner preview.'
            },
            // Plan builder + preview renderers. Provenance: INIT-010/SPEC-003
            scripts: SPEECH_AND_WORD_DATA.concat([
                '/shared/ui/blocks/explainer.js',
                '/shared/ui/blocks/conjugation-table.js',
                '/shared/ui/blocks/personal-question.js',
                '/shared/ui/blocks/verse-reading.js',
                '/shared/ui/blocks/letter-intro.js',
                '/shared/ui/blocks/compare.js',
                '/shared/ui/blocks/exercise.js',
                '/shared/ui/blocks/assignment.js',
                '/shared/ui/plan-render.js',
                '/shared/admin/activity-map.js',
                '/shared/admin/plan-ui.js',
                '/shared/admin/plan-hints.js',
                '/shared/admin/plan-widgets.js',
                '/shared/admin/plan-form.js',
                '/shared/admin/plan-list.js',
                '/shared/admin/plan-issues.js',
                '/shared/admin/plan-history.js',
                '/shared/admin/plan-preview.js',
                '/shared/admin/plan-builder.js',
                '/shared/admin/editor-shell.js'
            ]),
            view: 'HebrewEditorShell'
        }
    ];

    if (typeof module !== 'undefined' && module.exports) {
        module.exports = root.HebrewAppRoutes;
    }
})(typeof window !== 'undefined' ? window : globalThis);
