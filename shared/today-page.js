/**
 * Today page — next-step cards from server state + filtered catalog,
 * plus the daily mix from GET /api/me/mix.
 * The mix only arranges the sections the server returned. It does not invent words.
 * Drill checks go through HebrewBlockConjugation when that module is on the page.
 * A typed drill answer is not sent. Done marks are session memory, not a new table.
 * Provenance: INIT-002/SPEC-014, INIT-006/SPEC-015
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
            : ('<span class="ui-pair"><span class="ui-he" lang="he">' + esc(he) + '</span>'
                + '<span class="ui-en" lang="en" dir="ltr">' + esc(en) + '</span></span>');
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

    // INIT-005/SPEC-011 (REQ-016): the level is a Hebrew word with an English gloss, and the
    // English catalog blurb is marked as English so its punctuation sits on the right side.
    var LEVEL_LABELS = {
        basic: { he: 'בסיסי', en: 'Basic' },
        intermediate: { he: 'בינוני', en: 'Intermediate' },
        advanced: { he: 'מתקדם', en: 'Advanced' }
    };

    function levelKey(raw) {
        var level = String(raw || '').toLowerCase();
        if (level === 'basic' || level === 'a1' || level === 'beginner') return 'basic';
        if (level === 'advanced' || level === 'b1' || level === 'b2') return 'advanced';
        if (level === 'intermediate' || level === 'a2') return 'intermediate';
        return level;
    }

    function metaLine(lesson) {
        var level = levelKey(lesson.level);
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

    function blurbHtml(lesson) {
        return lesson.blurb
            ? '<p class="lesson-blurb" lang="en" dir="ltr">' + esc(lesson.blurb) + '</p>'
            : '';
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
        return bi(he, en);
    }

    function lessonHref(slug) {
        return '/worksheets/' + encodeURIComponent(String(slug || '')) + '/';
    }

    var Paths = root.HebrewLearningPaths
        || (typeof require === 'function' ? require('./learning-paths.js') : null);

    function renderPathCards(planIds) {
        if (!Paths) return '';
        return Paths.render(planIds, { firstOnly: true, allPaths: true, today: true });
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

    function ctaIcon(kind) {
        // Brand sprite (INIT-005/SPEC-010). The word beside the icon is the control's name.
        var name = (kind === 'play' || kind === 'next' || kind === 'review') ? kind : 'next';
        // "next" points right; it must point the other way in this right-to-left page.
        var cls = 'icon hebrew-icon' + (name === 'next' ? ' icon-directional' : '');
        return '<svg class="' + cls + '" width="20" height="20" aria-hidden="true" focusable="false">'
            + '<use href="/shared/brand/sprite.svg#icon-' + name + '"></use></svg> ';
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
                + bi('מילים לחזרה', 'due')
                + ' · '
                + '<span id="review-new-count">' + esc(String(summary.newCount)) + '</span> '
                + bi('חדשות', 'new')
                + '</p>'
                + '<p class="today-actions">'
                + '<a class="btn btn-primary" id="today-review-start" href="/review/">'
                + ctaIcon('review') + bi('התחילו', 'Start') + '</a>'
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
            + bi(status.he, status.en) + '</span>'
            + '<p class="fine">' + metaLine(lesson) + '</p>'
            + '<h2>' + esc(lesson.title) + '</h2>'
            + blurbHtml(lesson)
            + '<p class="today-actions">'
            + '<a class="btn btn-primary" id="today-continue-link" href="' + esc(lessonHref(lesson.slug)) + '">'
            + ctaIcon('play') + bi('המשיכו', 'Continue') + '</a>'
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
            + '<p class="fine">' + metaLine(lesson) + '</p>'
            + '<h2>' + esc(lesson.title) + '</h2>'
            + blurbHtml(lesson)
            + '<p class="today-actions">'
            + '<a class="btn btn-primary" id="today-next-link" href="' + esc(lessonHref(lesson.slug)) + '">'
            + action + '</a>'
            + '</p>'
            + '</div></article>';
    }

    function renderVerbsCard(verbs) {
        if (!verbs || !verbs.lessons) return '';
        var line = verbs.due > 0
            ? '<span id="today-verbs-due">' + esc(String(verbs.due)) + '</span> '
                + bi('צורות לחזרה', 'forms due')
            : bi('סבב של 12 כרטיסים, כשלוש דקות.', 'A round of 12 cards, about three minutes.');
        var known = verbs.known > 0
            ? '<p class="fine">' + esc(String(verbs.known)) + ' ' + bi('צורות ידועות', 'forms known') + '</p>'
            : '';
        return ''
            + '<article class="lesson-card today-card today-verbs" id="today-verbs-card" data-today-card="verbs">'
            + '<div class="lesson-link today-card-body">'
            + '<span class="lesson-status' + (verbs.due > 0 ? ' ready' : ' fresh') + '">'
            + (verbs.due > 0 ? bi('מוכן לתרגול', 'Ready to practise') : bi('פעלים', 'Verbs')) + '</span>'
            + '<h2>' + bi('תרגול פעלים', 'Verb practice') + '</h2>'
            + '<p>' + line + '</p>'
            + known
            + '<p class="today-actions">'
            + '<a class="btn btn-primary" id="today-verbs-start" href="/verbs/">'
            + ctaIcon('play') + bi('לתרגול', 'Practise') + '</a>'
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

    var SECTION_IDS = ['review', 'weak', 'drill', 'reading'];
    var SECTION_CAP = 40;
    var DONE_KEY = 'hebrew-today-mix-done:v1';
    var TEXT_CAP = 500;

    var mixById = Object.create(null);
    var glossMap = Object.create(null);
    var doneMemory = Object.create(null);
    var mixSession = null;
    var clipNode = null;
    var viewSignal = null;
    var lastMix = null;

    function shortText(value, max) {
        var raw = value == null ? '' : String(value);
        if (raw.length > max) raw = raw.slice(0, max);
        return raw;
    }

    function drillReady() {
        var drill = root.HebrewBlockConjugation;
        return !!(drill
            && typeof drill.decideAttempt === 'function'
            && typeof drill.formSignal === 'function');
    }

    function itemsOf(section) {
        var raw = section && Array.isArray(section.items) ? section.items : [];
        var out = [];
        for (var i = 0; i < raw.length && out.length < SECTION_CAP; i += 1) {
            if (raw[i] && typeof raw[i] === 'object') out.push(raw[i]);
        }
        return out;
    }

    /**
     * Sections the card may show, in practice order.
     * A drill section is omitted until the conjugation drill module is loaded.
     * A section the server did not send is not invented.
     * @param {{ sections?: object[] }|null|undefined} mix
     * @returns {object[]}
     */
    function visibleSections(mix) {
        var list = mix && Array.isArray(mix.sections) ? mix.sections : [];
        var byId = Object.create(null);
        list.forEach(function (section) {
            if (!section || SECTION_IDS.indexOf(section.id) === -1) return;
            if (!byId[section.id]) byId[section.id] = section;
        });
        var out = [];
        SECTION_IDS.forEach(function (id) {
            var section = byId[id];
            if (!section) return;
            if (id === 'drill' && !drillReady()) return;
            if (!itemsOf(section).length) return;
            out.push(section);
        });
        return out;
    }

    function todayKey(ms) {
        var d = new Date(ms || Date.now());
        function pad(n) { return (n < 10 ? '0' : '') + n; }
        return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate());
    }

    function readDone() {
        var today = todayKey();
        var saved = Object.create(null);
        try {
            var store = root.sessionStorage;
            var raw = store && store.getItem(DONE_KEY);
            if (raw) {
                var parsed = JSON.parse(raw);
                var sections = parsed && parsed.date === today ? parsed.sections : null;
                if (sections && typeof sections === 'object') {
                    SECTION_IDS.forEach(function (id) {
                        if (sections[id] === true) saved[id] = true;
                    });
                }
            }
        } catch (err) { /* keep the in-memory marks */ }
        SECTION_IDS.forEach(function (id) {
            if (doneMemory[id] === today) saved[id] = true;
        });
        return saved;
    }

    function markDone(id) {
        if (SECTION_IDS.indexOf(id) === -1) return;
        doneMemory[id] = todayKey();
        var sections = readDone();
        try {
            if (root.sessionStorage) {
                root.sessionStorage.setItem(DONE_KEY, JSON.stringify({
                    date: todayKey(),
                    sections: sections
                }));
            }
        } catch (err) { /* the in-memory mark still holds for this view */ }
    }

    function stripVowels(value) {
        var raw = String(value == null ? '' : value);
        if (U && typeof U.stripVowels === 'function') return U.stripVowels(raw);
        return raw.replace(/[\u0591-\u05C7]/g, '').normalize('NFC').replace(/\s+/g, ' ').trim();
    }

    function rememberGloss(key, gloss) {
        var word = shortText(key, 80).trim();
        var meaning = shortText(gloss, TEXT_CAP).trim();
        if (!word || !meaning) return;
        glossMap[word] = meaning;
    }

    function absorbGlosses(mix) {
        var list = mix && Array.isArray(mix.sections) ? mix.sections : [];
        list.forEach(function (section) {
            itemsOf(section).forEach(function (item) {
                if (!item.key) return;
                var english = item.direction === 'he_en' ? item.meaning : item.prompt;
                rememberGloss(item.key, english);
            });
        });
    }

    function glossFor(token) {
        var raw = shortText(token, 80).trim();
        if (!raw) return '';
        if (glossMap[raw]) return glossMap[raw];
        var stripped = stripVowels(raw);
        if (stripped && glossMap[stripped]) return glossMap[stripped];
        var keys = Object.keys(glossMap);
        for (var i = 0; i < keys.length; i += 1) {
            if (stripVowels(keys[i]) === stripped) return glossMap[keys[i]];
        }
        return '';
    }

    /**
     * Same-origin worksheet clip only. Anything else is not a playable URL.
     * @param {unknown} audio
     * @returns {string}
     */
    function clipHref(audio) {
        if (typeof audio !== 'string' || !audio) return '';
        if (/[\u0000-\u001F\u007F]/.test(audio) || audio.indexOf('..') !== -1 || audio.indexOf('\\') !== -1) {
            return '';
        }
        var loc = root.location;
        if (!loc || !loc.href) return '';
        try {
            var resolved = new URL(audio, loc.href);
            if (resolved.protocol !== 'http:' && resolved.protocol !== 'https:') return '';
            if (loc.origin && resolved.origin !== loc.origin) return '';
            if (!/^\/worksheets\/[a-z0-9]+(?:-[a-z0-9]+)*\/.+\.mp3$/i.test(resolved.pathname)) return '';
            return resolved.pathname;
        } catch (err) {
            return '';
        }
    }

    function stopClip() {
        if (!clipNode) return;
        try { clipNode.pause(); } catch (err) { /* already stopped */ }
        clipNode = null;
    }

    function indexMix(mix) {
        mixById = Object.create(null);
        glossMap = Object.create(null);
        visibleSections(mix).forEach(function (section) {
            mixById[section.id] = {
                id: section.id,
                label: shortText(section.label || section.concept || '', 200),
                concept: shortText(section.concept || '', 80),
                items: itemsOf(section)
            };
        });
        absorbGlosses(mix);
    }

    function sectionHeading(section) {
        if (section.id === 'weak') return bi('מילים חלשות', 'Weak words');
        if (section.id === 'drill') {
            var label = shortText(section.label || section.concept || '', 200);
            return bi('תרגול', 'Drill')
                + (label
                    ? ' · <span class="today-mix-concept" dir="auto">' + esc(label) + '</span>'
                    : '');
        }
        if (section.id === 'reading') return bi('קריאה', 'Reading');
        return bi('חזרה', 'Review');
    }

    function renderMixRow(section) {
        var id = section.id;
        var count = itemsOf(section).length;
        var done = readDone()[id] === true;
        var action = done
            ? '<p class="today-mix-done" data-mix-done="1">' + bi('בוצע להיום', 'Done for today') + '</p>'
            : '<button type="button" class="btn btn-primary" data-mix-action="start" data-mix-id="'
                + esc(id) + '">' + bi('התחילו', 'Start') + '</button>';
        return '<li class="today-mix-row" id="today-mix-' + esc(id) + '" data-mix-section="' + esc(id) + '">'
            + '<div class="today-mix-copy"><p class="today-mix-title">' + sectionHeading(section) + '</p>'
            + '<p class="fine"><span class="today-mix-count" data-mix-count="' + esc(id) + '">'
            + esc(String(count)) + '</span> ' + bi('פריטים', 'items') + '</p></div>'
            + action + '</li>';
    }

    function mixBodyHtml(mix) {
        var sections = visibleSections(mix);
        if (!sections.length) return '';
        return '<h2>' + bi('התרגול של היום', "Today's practice") + '</h2>'
            + '<ul class="today-mix-list" id="today-mix-list">'
            + sections.map(renderMixRow).join('')
            + '</ul>';
    }

    function renderMixCard(mix) {
        var body = mixBodyHtml(mix);
        if (!body) return '';
        return '<article class="lesson-card today-card today-mix" id="today-mix" data-today-card="mix">'
            + '<div class="lesson-link today-card-body" id="today-mix-body">' + body + '</div></article>';
    }

    function node(tag, className) {
        var created = root.document.createElement(tag);
        if (className) created.className = className;
        return created;
    }

    function appendBi(parent, he, en) {
        parent.appendChild(root.document.createTextNode(he + ' '));
        var span = node('span', 'en');
        span.dir = 'ltr';
        span.lang = 'en';
        span.textContent = '(' + en + ')';
        parent.appendChild(span);
    }

    function actionButton(action, id, he, en, primary) {
        var button = node('button', primary ? 'btn btn-primary' : 'btn today-mix-back');
        button.type = 'button';
        button.setAttribute('data-mix-action', action);
        if (id) button.setAttribute('data-mix-id', id);
        appendBi(button, he, en);
        return button;
    }

    function showMixList() {
        stopClip();
        mixSession = null;
        var body = root.document && root.document.getElementById('today-mix-body');
        if (!body || !mounted) return;
        body.innerHTML = mixBodyHtml(lastMix);
    }

    function hebrewPrompt(item) {
        return !item || item.direction === 'en_he' ? false : true;
    }

    function renderFlash(body, section, index) {
        var items = section.items;
        var item = items[index];
        if (!item) return;
        mixSession = { id: section.id, index: index, revealed: false };
        body.textContent = '';
        var session = node('div', 'today-mix-session');
        session.id = 'today-mix-session';
        session.setAttribute('data-mix-phase', 'card');
        var kicker = node('p', 'fine');
        kicker.appendChild(root.document.createTextNode(
            section.id === 'weak' ? 'מילים חלשות ' : 'חזרה '
        ));
        var en = node('span', 'en');
        en.dir = 'ltr';
        en.lang = 'en';
        en.textContent = section.id === 'weak' ? '(Weak words)' : '(Review)';
        kicker.appendChild(en);
        var prompt = node('p', 'today-mix-prompt');
        prompt.id = 'today-mix-prompt';
        var hePrompt = hebrewPrompt(item);
        prompt.lang = hePrompt ? 'he' : 'en';
        prompt.dir = hePrompt ? 'rtl' : 'ltr';
        prompt.textContent = shortText(item.prompt, TEXT_CAP);
        var actions = node('p', 'today-actions');
        actions.appendChild(actionButton('reveal', section.id, 'הצגת התשובה', 'Show answer', true));
        actions.appendChild(actionButton('back', section.id, 'חזרה להיום', 'Back to Today', false));
        session.appendChild(kicker);
        session.appendChild(prompt);
        session.appendChild(actions);
        body.appendChild(session);
    }

    function revealFlash() {
        if (!mixSession || mixSession.revealed) return;
        var section = mixById[mixSession.id];
        var item = section && section.items[mixSession.index];
        var session = root.document && root.document.getElementById('today-mix-session');
        if (!item || !session) return;
        mixSession.revealed = true;
        var answer = node('p', 'today-mix-answer');
        answer.id = 'today-mix-answer';
        var heAnswer = !hebrewPrompt(item);
        answer.lang = heAnswer ? 'he' : 'en';
        answer.dir = heAnswer ? 'rtl' : 'ltr';
        answer.textContent = shortText(item.meaning, TEXT_CAP);
        var actions = session.querySelector('.today-actions');
        var reveal = session.querySelector('[data-mix-action="reveal"]');
        if (reveal && reveal.parentNode) reveal.parentNode.removeChild(reveal);
        session.insertBefore(answer, actions);
        if (!actions) return;
        var last = mixSession.index >= section.items.length - 1;
        actions.insertBefore(
            last
                ? actionButton('finish', section.id, 'סיום', 'Done', true)
                : actionButton('next', section.id, 'הבא', 'Next', true),
            actions.firstChild
        );
    }

    function renderReading(body, section) {
        mixSession = { id: 'reading', index: 0, revealed: true };
        body.textContent = '';
        var session = node('div', 'today-mix-session');
        session.id = 'today-mix-session';
        session.setAttribute('data-mix-phase', 'reading');
        var kicker = node('p', 'fine');
        appendBi(kicker, 'קריאה', 'Reading');
        session.appendChild(kicker);
        section.items.forEach(function (item) {
            var line = node('p', 'today-mix-sentence');
            line.lang = 'he';
            line.dir = 'rtl';
            var sentence = shortText(item.sentence, TEXT_CAP);
            sentence.split(/(\s+)/).forEach(function (part) {
                if (!part) return;
                if (/[\u0590-\u05FF]/.test(part)) {
                    var word = node('button', 'today-mix-word');
                    word.type = 'button';
                    word.lang = 'he';
                    word.dir = 'rtl';
                    word.setAttribute('data-mix-action', 'gloss');
                    word.textContent = part;
                    line.appendChild(word);
                } else {
                    line.appendChild(root.document.createTextNode(part));
                }
            });
            var clip = clipHref(item.audio);
            if (clip) {
                var play = actionButton('play', 'reading', 'השמעה', 'Play', false);
                play.className = 'btn today-mix-play';
                play.setAttribute('data-mix-src', clip);
                line.appendChild(root.document.createTextNode(' '));
                line.appendChild(play);
            }
            session.appendChild(line);
        });
        var gloss = node('p', 'today-mix-gloss');
        gloss.id = 'today-mix-gloss';
        gloss.lang = 'en';
        gloss.dir = 'ltr';
        gloss.setAttribute('aria-live', 'polite');
        session.appendChild(gloss);
        var actions = node('p', 'today-actions');
        actions.appendChild(actionButton('finish', 'reading', 'סיום', 'Done', true));
        actions.appendChild(actionButton('back', 'reading', 'חזרה להיום', 'Back to Today', false));
        session.appendChild(actions);
        body.appendChild(session);
        loadLibraryGlosses();
    }

    function loadLibraryGlosses() {
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') return;
        var signal = viewSignal;
        Api.get('/api/me/library?limit=200', { redirectOn401: false, signal: signal })
            .then(function (res) {
                if (!mounted || (signal && signal.aborted)) return;
                if (!res || !res.ok || !res.data || !Array.isArray(res.data.words)) return;
                res.data.words.forEach(function (row) {
                    if (row && row.word) rememberGloss(row.word, row.gloss_en);
                });
            })
            .catch(function () { /* taps still use glosses already on the mix */ });
    }

    function showGloss(button) {
        var gloss = root.document && root.document.getElementById('today-mix-gloss');
        if (!gloss || !button) return;
        var meaning = glossFor(button.textContent);
        gloss.textContent = meaning || 'אין תרגום שמור. (No saved gloss.)';
    }

    function playClip(button) {
        var src = clipHref(button && button.getAttribute('data-mix-src'));
        if (!src) return;
        stopClip();
        try {
            clipNode = new root.Audio(src);
            var played = clipNode.play();
            if (played && typeof played.catch === 'function') played.catch(function () { /* no clip output */ });
        } catch (err) {
            clipNode = null;
        }
    }

    function renderDrill(body, section) {
        mixSession = { id: 'drill', index: 0, revealed: false, tries: 0, settled: false };
        body.textContent = '';
        var session = node('div', 'today-mix-session');
        session.id = 'today-mix-session';
        session.setAttribute('data-mix-phase', 'drill');
        var kicker = node('p', 'fine');
        appendBi(kicker, 'תרגול', 'Drill');
        var concept = node('p', 'today-mix-concept');
        concept.dir = 'auto';
        concept.textContent = section.label || section.concept || '';
        var prompt = node('p', 'today-mix-prompt');
        prompt.id = 'today-mix-prompt';
        prompt.lang = 'en';
        prompt.dir = 'ltr';
        var feedback = node('p', 'today-mix-feedback');
        feedback.id = 'today-mix-feedback';
        var reveal = node('p', 'today-mix-answer');
        reveal.id = 'today-mix-reveal';
        reveal.lang = 'he';
        reveal.dir = 'rtl';
        reveal.hidden = true;
        var label = node('label', 'today-mix-field');
        appendBi(label, 'התשובה שלכם', 'Your answer');
        var input = node('input', 'today-mix-input');
        input.id = 'today-mix-drill-input';
        input.type = 'text';
        input.lang = 'he';
        input.dir = 'rtl';
        input.setAttribute('autocomplete', 'off');
        input.setAttribute('autocorrect', 'off');
        input.spellcheck = false;
        label.appendChild(input);
        var actions = node('p', 'today-actions');
        actions.appendChild(actionButton('check', 'drill', 'בדיקה', 'Check', true));
        actions.appendChild(actionButton('override', 'drill', 'צדקתי', 'I was right', false));
        var next = actionButton('next', 'drill', 'הבא', 'Next', true);
        next.hidden = true;
        next.id = 'today-mix-drill-next';
        actions.appendChild(next);
        actions.appendChild(actionButton('back', 'drill', 'חזרה להיום', 'Back to Today', false));
        session.appendChild(kicker);
        session.appendChild(concept);
        session.appendChild(prompt);
        session.appendChild(feedback);
        session.appendChild(reveal);
        session.appendChild(label);
        session.appendChild(actions);
        body.appendChild(session);
        showDrillItem();
    }

    function showDrillItem() {
        var section = mixById.drill;
        var item = section && mixSession && section.items[mixSession.index];
        var prompt = root.document && root.document.getElementById('today-mix-prompt');
        var feedback = root.document && root.document.getElementById('today-mix-feedback');
        var reveal = root.document && root.document.getElementById('today-mix-reveal');
        var input = root.document && root.document.getElementById('today-mix-drill-input');
        var next = root.document && root.document.getElementById('today-mix-drill-next');
        if (!item || !prompt) return;
        mixSession.tries = 0;
        mixSession.settled = false;
        mixSession.revealed = false;
        prompt.textContent = shortText(item.prompt, TEXT_CAP);
        if (feedback) feedback.textContent = '';
        if (reveal) {
            reveal.textContent = '';
            reveal.hidden = true;
        }
        if (input) {
            input.value = '';
            input.disabled = false;
        }
        if (next) next.hidden = true;
    }

    function drillForm(item) {
        return {
            key: item.key,
            formKey: item.key,
            lemma: item.lemma || '',
            accept: Array.isArray(item.accept) ? item.accept : []
        };
    }

    function postForm(item, result, activity) {
        var Drill = root.HebrewBlockConjugation;
        if (!Drill || typeof Drill.formSignal !== 'function') return;
        var signal = Drill.formSignal(drillForm(item), result, activity, '');
        var Sync = root.HebrewSync;
        if (!signal || !Sync || typeof Sync.queueSignal !== 'function') return;
        if (typeof Sync.isActive === 'function' && !Sync.isActive()) return;
        Sync.queueSignal(signal);
    }

    function settleDrill(item, message, revealText) {
        if (!mixSession) return;
        mixSession.settled = true;
        mixSession.revealed = true;
        var feedback = root.document.getElementById('today-mix-feedback');
        var reveal = root.document.getElementById('today-mix-reveal');
        var input = root.document.getElementById('today-mix-drill-input');
        var next = root.document.getElementById('today-mix-drill-next');
        if (feedback) feedback.textContent = message;
        if (revealText && reveal) {
            reveal.hidden = false;
            reveal.textContent = shortText(revealText, TEXT_CAP);
        }
        if (input) input.disabled = true;
        if (!next) return;
        var section = mixById.drill;
        var last = !section || mixSession.index >= section.items.length - 1;
        next.hidden = false;
        next.textContent = '';
        appendBi(next, last ? 'סיום' : 'הבא', last ? 'Done' : 'Next');
        next.setAttribute('data-mix-action', last ? 'finish' : 'next');
    }

    function checkDrill() {
        var Drill = root.HebrewBlockConjugation;
        var section = mixById.drill;
        var item = section && mixSession && section.items[mixSession.index];
        var input = root.document && root.document.getElementById('today-mix-drill-input');
        if (!Drill || !item || !input || !mixSession || mixSession.settled) return;
        var decision = Drill.decideAttempt(input.value, drillForm(item), mixSession.tries);
        if (!decision.attempted) return;
        var kept = input.value;
        postForm(item, decision.result, decision.activity);
        input.value = kept;
        if (decision.correct) {
            settleDrill(item, 'נכון. (Correct.)', '');
            return;
        }
        if (decision.reveal) {
            settleDrill(item, 'הצורה. (The form.)', String(item.key || '').replace(/-[0-9a-z]+$/i, ''));
            return;
        }
        mixSession.tries += 1;
        var feedback = root.document.getElementById('today-mix-feedback');
        if (feedback) feedback.textContent = 'נסו שוב. (Try again.)';
    }

    function overrideDrill() {
        var section = mixById.drill;
        var item = section && mixSession && section.items[mixSession.index];
        if (!item || !mixSession || mixSession.settled) return;
        postForm(item, 'pass', 'conjugation-override');
        settleDrill(item, 'נכון. (I was right.)', '');
    }

    function openSection(id) {
        if (!mounted || !mixById[id]) return;
        var body = root.document.getElementById('today-mix-body');
        if (!body) return;
        stopClip();
        if (id === 'reading') renderReading(body, mixById[id]);
        else if (id === 'drill') renderDrill(body, mixById[id]);
        else renderFlash(body, mixById[id], 0);
    }

    function advanceFlash() {
        if (!mixSession || !mixSession.revealed) return;
        var section = mixById[mixSession.id];
        var body = root.document.getElementById('today-mix-body');
        if (!section || !body) return;
        if (mixSession.index >= section.items.length - 1) {
            markDone(section.id);
            showMixList();
            return;
        }
        renderFlash(body, section, mixSession.index + 1);
    }

    function onMixClick(event) {
        if (!mounted) return;
        var target = event.target;
        if (target && target.nodeType === 3) target = target.parentElement;
        if (!target || typeof target.closest !== 'function') return;
        var button = target.closest('[data-mix-action]');
        if (!button || !mounted.contains(button)) return;
        var action = button.getAttribute('data-mix-action');
        var id = button.getAttribute('data-mix-id') || (mixSession && mixSession.id);
        if (action === 'start') openSection(id);
        else if (action === 'back') showMixList();
        else if (action === 'reveal') revealFlash();
        else if (action === 'next') {
            if (mixSession && mixSession.id === 'drill') {
                var drill = mixById.drill;
                if (drill && mixSession.index < drill.items.length - 1) {
                    mixSession.index += 1;
                    showDrillItem();
                }
            } else advanceFlash();
        } else if (action === 'finish') {
            markDone(id);
            showMixList();
        } else if (action === 'gloss') showGloss(button);
        else if (action === 'play') playClip(button);
        else if (action === 'check') checkDrill();
        else if (action === 'override') overrideDrill();
    }

    function loadMix(signal) {
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') return Promise.resolve(null);
        return Api.get('/api/me/mix', { redirectOn401: false, signal: signal })
            .then(function (res) {
                if (signal && signal.aborted) return null;
                if (!res || !res.ok || !res.data || !Array.isArray(res.data.sections)) return null;
                return res.data;
            })
            .catch(function () { return null; });
    }

    /**
     * Published verb workshops plus this device's verb ladder. Null hides the card.
     */
    function loadVerbs(signal, user) {
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') return Promise.resolve({ verbs: null, planIds: [] });
        return Api.get('/api/plans/catalog', { redirectOn401: false, signal: signal })
            .then(function (res) {
                var rows = res && res.ok && res.data && Array.isArray(res.data.lessons) ? res.data.lessons : null;
                if (!rows) {
                    if (root.console && typeof root.console.error === 'function') {
                        root.console.error('[today] plan catalog unusable', {
                            status: res && res.status,
                            error: res && res.error
                        });
                    }
                    return { verbs: null, planIds: [] };
                }
                var planIds = Paths ? Paths.summaries(rows) : [];
                var lessons = rows.filter(function (row) { return row && row.recipe === 'verb-workshop'; }).length;
                if (!lessons) return { verbs: null, planIds: planIds };
                var VP = root.HebrewVerbPractice;
                var summary = { due: 0, known: 0 };
                if (VP && typeof VP.loadLadder === 'function') {
                    var storage = null;
                    try { storage = root.localStorage; } catch (err) { storage = null; }
                    summary = VP.dueSummary(VP.loadLadder(storage, user), Date.now());
                }
                return {
                    verbs: { lessons: lessons, due: summary.due, known: summary.known },
                    planIds: planIds
                };
            })
            .catch(function (err) {
                if (root.console && typeof root.console.error === 'function') {
                    root.console.error('[today] plan catalog failed', err);
                }
                return { verbs: null, planIds: [] };
            });
    }

    function renderToday(model) {
        if (!model || model.state === 'loading') return renderLoading();
        if (model.state === 'error') return renderError(model.message);
        var list = model.catalog || [];
        var planIds = model.planIds || [];
        if (!list.length && !planIds.length) return renderEmptyCatalog();
        var parts = [];
        parts.push(renderPathCards(planIds));
        parts.push(renderMixCard(model.mix));
        parts.push(renderReviewCard(model.review || {}, model.brandNew));
        parts.push(renderVerbsCard(model.verbs));
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
        lastMix = model && model.mix;
        indexMix(lastMix);
        el.innerHTML = renderToday(model);
    }

    // ---- view lifecycle (contract: shared/router.js) ----
    // `mounted` is the element of the open view, or null once the route closes.
    // Every async callback checks it so a late response never paints a closed view.
    var mounted = null;

    function mount(el, ctx) {
        mounted = el;
        viewSignal = (ctx && ctx.signal) || null;
        mixSession = null;
        stopClip();
        if (el && typeof el.addEventListener === 'function') {
            el.addEventListener('click', onMixClick);
        }
        var ready = (ctx && ctx.ready) || Promise.resolve(null);
        var signal = viewSignal;
        return ready.then(function () {
            if (mounted !== el) return null;
            return Promise.all([
                loadProgressMap(catalog()),
                loadMix(signal),
                loadVerbs(signal, ctx && ctx.user)
            ]);
        }).then(function (pair) {
            if (!pair || mounted !== el) return;
            if (signal && signal.aborted) return;
            var model = buildModel(pair[0] || Object.create(null));
            var catalogBits = pair[2] || { verbs: null, planIds: [] };
            model.mix = pair[1];
            model.verbs = catalogBits.verbs;
            model.planIds = catalogBits.planIds || [];
            paint(el, model);
        }).catch(function (err) {
            if (mounted !== el) return;
            if (signal && signal.aborted) return;
            paint(el, { state: 'error', message: (err && err.message) || 'Network error' });
        });
    }

    function unmount() {
        stopClip();
        if (mounted && typeof mounted.removeEventListener === 'function') {
            mounted.removeEventListener('click', onMixClick);
        }
        mounted = null;
        viewSignal = null;
        mixSession = null;
    }

    var pageApi = {
        render: renderToday,
        buildModel: buildModel,
        mount: mount,
        unmount: unmount,
        visibleSections: visibleSections,
        clipHref: clipHref
    };

    root.HebrewTodayPage = pageApi;
    if (typeof module !== 'undefined' && module.exports) module.exports = pageApi;
})(typeof window !== 'undefined' ? window : globalThis);
