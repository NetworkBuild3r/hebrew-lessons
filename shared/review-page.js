/**
 * "Review today" session UI — interleaved due recall across lessons.
 * Authored-by-spec: INIT-001/SPEC-004
 */
(function (root) {
    var U = root.HebrewUtils;
    var Review = root.HebrewReview;
    var Speech = root.HebrewSpeech;
    var PREFS_KEY = 'hebrew-review-prefs:v1';
    var VOWELS_DEFAULT = true;

    var state = {
        vowelsOn: VOWELS_DEFAULT,
        queue: [],
        index: 0,
        revealed: false,
        reviewed: 0,
        passed: 0,
        missed: 0,
        finished: false,
        indexMissing: false
    };

    function bi(he, en) {
        return U && typeof U.bi === 'function' ? U.bi(he, en) : (he + ' (' + en + ')');
    }

    function esc(value) {
        return U && typeof U.escapeHtml === 'function'
            ? U.escapeHtml(value)
            : String(value == null ? '' : value)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
    }

    function icon(name, className) {
        return U && typeof U.icon === 'function' ? U.icon(name, className) : '';
    }

    function loadPrefs() {
        try {
            var raw = localStorage.getItem(PREFS_KEY);
            if (!raw) return;
            var data = JSON.parse(raw);
            if (typeof data.vowelsOn === 'boolean') state.vowelsOn = data.vowelsOn;
        } catch (err) { /* fail-soft */ }
    }

    function savePrefs() {
        try {
            localStorage.setItem(PREFS_KEY, JSON.stringify({ vowelsOn: state.vowelsOn }));
        } catch (err) { /* fail-soft */ }
    }

    /**
     * Vocab-index and word-audio catalog URLs are authored relative to
     * worksheets/_review/ (or worksheets/<lesson>/). Resolve from that base
     * so review/index.html can play the same clips.
     */
    function audioHref(url) {
        if (!url) return '';
        if (/^(https?:|data:|blob:)/i.test(String(url))) return String(url);
        try {
            var base = new URL('../worksheets/_review/', root.location.href);
            return new URL(String(url), base).href;
        } catch (err) {
            return String(url);
        }
    }

    function indexByKey(list) {
        var map = Object.create(null);
        (list || []).forEach(function (entry) {
            if (!entry) return;
            var key = entry.key || (Review ? Review.keyFor(entry.word) : entry.word);
            if (key) map[key] = entry;
        });
        return map;
    }

    function vocabList() {
        var raw = root.HEBREW_VOCAB_INDEX;
        if (raw == null) return null;
        if (Array.isArray(raw)) return raw;
        if (typeof raw === 'object') {
            return Object.keys(raw).map(function (k) {
                var e = raw[k];
                if (!e || typeof e !== 'object') return null;
                return Object.assign({ key: e.key || k }, e);
            }).filter(Boolean);
        }
        return null;
    }

    /**
     * aud-1: en→he only after the first he→en pass.
     * @param {Array} items dueList output
     * @param {object} words store.words
     */
    function filterQueue(items, words) {
        return (items || []).filter(function (item) {
            if (!item || item.direction !== 'en_he') return true;
            var entry = words && words[item.key];
            var he = entry && entry.he_en;
            return !!(he && he.lastResult === 'pass');
        });
    }

    function buildClips(entries) {
        if (!Speech || typeof Speech.setClips !== 'function') return;
        var clipEntries = [];
        var map = root.HEBREW_WORD_AUDIO || {};
        Object.keys(map).forEach(function (text) {
            var url = audioHref(map[text]);
            if (text && url) clipEntries.push({ text: text, url: url });
        });
        (entries || []).forEach(function (entry) {
            if (!entry || !entry.word) return;
            var url = audioHref(entry.audio || map[entry.word] || map[String(entry.word).normalize('NFC')] || '');
            if (url) clipEntries.push({ text: entry.word, url: url });
        });
        Speech.setClips(clipEntries);
    }

    function displayHebrew(text) {
        if (!text) return '';
        if (state.vowelsOn) return text;
        return U && typeof U.stripVowels === 'function' ? U.stripVowels(text) : text;
    }

    function speakButton(text, audioUrl, title) {
        var clip = '';
        if (audioUrl) clip = audioHref(audioUrl);
        else if (Speech && typeof Speech.clipFor === 'function') clip = Speech.clipFor(text) || '';
        var label = title || 'Listen';
        return '<button type="button" class="icon-btn' + (clip ? ' human' : '') + '" data-action="speak" data-speak="'
            + esc(text || '') + '" data-audio="' + esc(clip) + '" title="' + esc(label) + '">'
            + icon('volume-high') + '</button>';
    }

    function lessonLabel(slug) {
        if (!slug) return '';
        return '<p class="fine review-lesson-source">' + bi('משיעור', 'From lesson') + ': <span dir="ltr">' + esc(slug) + '</span></p>';
    }

    function nextDueIso(store) {
        var words = store && store.words ? store.words : {};
        var soonest = null;
        Object.keys(words).forEach(function (key) {
            var entry = words[key];
            if (!entry) return;
            ['he_en', 'en_he'].forEach(function (dir) {
                var due = entry[dir] && entry[dir].due;
                if (!due) return;
                var ms = Date.parse(String(due));
                if (!Number.isFinite(ms)) return;
                if (soonest == null || ms < soonest) soonest = ms;
            });
        });
        return soonest == null ? null : new Date(soonest).toISOString();
    }

    function formatDue(iso) {
        if (!iso) return bi('אין תאריך עדיין', 'No next due yet');
        var d = new Date(iso);
        if (Number.isNaN(d.getTime())) return bi('אין תאריך עדיין', 'No next due yet');
        var he = d.toLocaleString('he-IL', { weekday: 'long', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
        var en = d.toLocaleString('en-US', { weekday: 'long', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
        return esc(he) + ' <span class="en" dir="ltr">(' + esc(en) + ')</span>';
    }

    function paintChrome() {
        var home = document.getElementById('review-home-link');
        if (home) {
            home.innerHTML = icon('house') + ' ' + bi('דף הבית', 'Home');
        }
        var sub = document.getElementById('review-subtitle');
        if (sub) {
            sub.innerHTML = bi(
                'מילים שמגיע הזמן לחזור עליהן מכל השיעורים.',
                'Words due for recall across all lessons.'
            );
        }
        paintVowelsBtn();
    }

    function paintVowelsBtn() {
        var btn = document.getElementById('review-vowels-btn');
        if (!btn) return;
        btn.setAttribute('aria-pressed', state.vowelsOn ? 'true' : 'false');
        btn.innerHTML = state.vowelsOn
            ? bi('הסתר ניקוד', 'Hide vowels')
            : bi('הצג ניקוד', 'Show vowels');
    }

    function currentItem() {
        return state.queue[state.index] || null;
    }

    function currentEntry(vocabMap) {
        var item = currentItem();
        if (!item) return null;
        return vocabMap[item.key] || null;
    }

    function renderMissing() {
        state.indexMissing = true;
        var app = document.getElementById('review-app');
        if (!app) return;
        app.innerHTML = '<div class="card review-empty" id="review-no-index">'
            + '<h2>' + bi('אין עדיין נתוני חזרה', 'No review data yet') + '</h2>'
            + '<p>' + bi(
                'אחרי שמפרסמים שיעורים ובונים את האינדקס, החזרה תופיע כאן.',
                'After lessons are published and the vocab index is built, review will show here.'
            ) + '</p>'
            + '<p><a class="btn btn-primary" href="../index.html">' + bi('חזרה לדף הבית', 'Back home') + '</a></p>'
            + '</div>';
    }

    function renderEmpty() {
        var app = document.getElementById('review-app');
        if (!app) return;
        var store = Review.all();
        var next = nextDueIso(store);
        app.innerHTML = '<div class="card review-empty" id="review-nothing-due">'
            + '<h2>' + bi('אין מה לחזור עליו היום', 'Nothing due today') + '</h2>'
            + '<p>' + bi('חזרו לשיעורים או בדקו שוב מאוחר יותר.', 'Go back to lessons or check again later.') + '</p>'
            + (next ? '<p class="fine">' + bi('הבא', 'Next due') + ': ' + formatDue(next) + '</p>' : '')
            + '<p><a class="btn btn-primary" href="../index.html">' + bi('דף הבית', 'Home') + '</a></p>'
            + '</div>';
    }

    function renderFinished() {
        state.finished = true;
        var app = document.getElementById('review-app');
        if (!app) return;
        var store = Review.all();
        var next = nextDueIso(store);
        app.innerHTML = '<div class="card review-summary" id="review-finished">'
            + '<h2>' + bi('סיימנו להיום', 'Done for now') + '</h2>'
            + '<ul class="review-stats">'
            + '<li>' + bi('נבדקו', 'Reviewed') + ': <strong id="review-count-reviewed">' + state.reviewed + '</strong></li>'
            + '<li>' + bi('ידעתי', 'Knew') + ': <strong id="review-count-knew">' + state.passed + '</strong></li>'
            + '<li>' + bi('פספסתי', 'Missed') + ': <strong id="review-count-missed">' + state.missed + '</strong></li>'
            + '</ul>'
            + '<p class="fine">' + bi('החזרה הבאה', 'Next due') + ': <span id="review-next-due">' + formatDue(next) + '</span></p>'
            + '<p><a class="btn btn-primary" href="../index.html">' + bi('דף הבית', 'Home') + '</a></p>'
            + '</div>';
    }

    function renderItem(vocabMap) {
        var item = currentItem();
        var entry = currentEntry(vocabMap);
        var app = document.getElementById('review-app');
        if (!app || !item) {
            renderFinished();
            return;
        }

        var word = entry && entry.word ? entry.word : item.key;
        var meaning = entry && entry.meaning ? entry.meaning : '';
        var audio = entry && entry.audio ? entry.audio : '';
        var lesson = item.lesson
            || (entry && Array.isArray(entry.lessons) && entry.lessons[0])
            || '';
        var progress = (state.index + 1) + ' / ' + state.queue.length;
        var isHeEn = item.direction !== 'en_he';

        var promptHtml;
        if (isHeEn) {
            promptHtml = '<div class="review-prompt" id="review-prompt">'
                + '<div class="he-word" id="review-prompt-he" data-vowels="' + esc(word) + '">'
                + esc(displayHebrew(word)) + '</div>'
                + speakButton(word, audio, 'Word')
                + '</div>';
        } else {
            promptHtml = '<div class="review-prompt" id="review-prompt">'
                + '<p class="meaning review-prompt-en" id="review-prompt-en" dir="ltr">' + esc(meaning) + '</p>'
                + '<label class="fine" for="review-typed">' + bi('כתבו בעברית (לא חובה)', 'Type in Hebrew (optional)') + '</label>'
                + '<input type="text" id="review-typed" class="review-typed" lang="he" dir="rtl" autocomplete="off" '
                + 'aria-label="' + esc('Type Hebrew answer') + '">'
                + '</div>';
        }

        var answerInner;
        if (isHeEn) {
            answerInner = '<p class="meaning" id="review-answer-meaning" dir="ltr">' + esc(meaning) + '</p>';
        } else {
            answerInner = '<div class="he-word" id="review-answer-he" data-vowels="' + esc(word) + '">'
                + esc(displayHebrew(word)) + '</div>'
                + speakButton(word, audio, 'Word');
        }

        app.innerHTML = '<div class="card review-card" id="review-card" data-direction="'
            + esc(item.direction) + '" data-key="' + esc(item.key) + '" data-lesson="' + esc(lesson) + '">'
            + '<div class="review-progress fine" id="review-progress">' + esc(progress) + '</div>'
            + '<p class="fine">' + bi(
                isHeEn ? 'עברית ← אנגלית' : 'אנגלית ← עברית',
                isHeEn ? 'Hebrew → English' : 'English → Hebrew'
            ) + '</p>'
            + promptHtml
            + lessonLabel(lesson)
            + '<div class="review-actions" id="review-reveal-row">'
            + '<button type="button" class="btn btn-primary" id="review-show-answer" data-action="reveal">'
            + bi('הצג תשובה', 'Show answer') + '</button>'
            + '</div>'
            + '<div class="review-answer hidden" id="review-answer" aria-hidden="true">'
            + answerInner
            + '<div class="mark-row" id="review-grade-row">'
            + '<button type="button" class="btn btn-mark" id="review-knew" data-action="grade" data-signal="pass">'
            + bi('ידעתי', 'Knew it') + '</button>'
            + '<button type="button" class="btn btn-mark missed" id="review-missed" data-action="grade" data-signal="miss">'
            + bi('פספסתי', 'Missed it') + '</button>'
            + '</div>'
            + '</div>'
            + '</div>';

        state.revealed = false;
    }

    function revealAnswer(vocabMap) {
        if (state.revealed || state.finished) return;
        var answer = document.getElementById('review-answer');
        var revealRow = document.getElementById('review-reveal-row');
        if (!answer) return;
        answer.classList.remove('hidden');
        answer.setAttribute('aria-hidden', 'false');
        if (revealRow) revealRow.classList.add('hidden');
        state.revealed = true;

        var item = currentItem();
        if (item && item.direction === 'en_he') {
            var typed = document.getElementById('review-typed');
            var entry = currentEntry(vocabMap);
            if (typed && entry && entry.word && U && typeof U.normalizeHebrew === 'function') {
                var got = U.normalizeHebrew(typed.value);
                var want = U.normalizeHebrew(entry.word);
                if (got && want && got === want) {
                    typed.classList.add('review-typed-ok');
                }
            }
        }
    }

    function grade(signal, vocabMap) {
        if (!state.revealed || state.finished) return;
        var item = currentItem();
        if (!item || !Review) return;
        var entry = currentEntry(vocabMap);
        var word = entry && entry.word ? entry.word : item.key;
        var lesson = item.lesson
            || (entry && Array.isArray(entry.lessons) && entry.lessons[0])
            || '';
        Review.record(word, item.direction, signal, { lesson: lesson });
        state.reviewed += 1;
        if (signal === 'pass') state.passed += 1;
        if (signal === 'miss') state.missed += 1;
        state.index += 1;
        if (state.index >= state.queue.length) renderFinished();
        else renderItem(vocabMap);
    }

    function onKey(event, vocabMap) {
        if (state.finished || state.indexMissing) return;
        var tag = (event.target && event.target.tagName) || '';
        if (tag === 'INPUT' || tag === 'TEXTAREA') {
            if (event.key === 'Enter' && !state.revealed) {
                event.preventDefault();
                revealAnswer(vocabMap);
            }
            return;
        }
        if (event.key === ' ' || event.code === 'Space') {
            event.preventDefault();
            if (!state.revealed) revealAnswer(vocabMap);
            return;
        }
        if (!state.revealed) return;
        if (event.key === '1') {
            event.preventDefault();
            grade('pass', vocabMap);
        } else if (event.key === '2') {
            event.preventDefault();
            grade('miss', vocabMap);
        }
    }

    function bind(vocabMap) {
        var app = document.getElementById('review-app');
        if (!app || app._reviewBound) return;
        app._reviewBound = true;
        app.addEventListener('click', function (event) {
            var btn = event.target.closest('[data-action]');
            if (!btn) return;
            var action = btn.getAttribute('data-action');
            if (action === 'speak') {
                if (Speech && typeof Speech.play === 'function') {
                    Speech.play(btn.getAttribute('data-speak') || '', btn.getAttribute('data-audio') || null);
                }
                return;
            }
            if (action === 'reveal') {
                revealAnswer(vocabMap);
                return;
            }
            if (action === 'grade') {
                grade(btn.getAttribute('data-signal') || 'pass', vocabMap);
            }
        });
        document.addEventListener('keydown', function (event) {
            onKey(event, vocabMap);
        });
        var vowelsBtn = document.getElementById('review-vowels-btn');
        if (vowelsBtn && !vowelsBtn._reviewBound) {
            vowelsBtn._reviewBound = true;
            vowelsBtn.addEventListener('click', function () {
                state.vowelsOn = !state.vowelsOn;
                savePrefs();
                paintVowelsBtn();
                document.querySelectorAll('[data-vowels]').forEach(function (el) {
                    var full = el.getAttribute('data-vowels') || '';
                    el.textContent = displayHebrew(full);
                });
            });
        }
    }

    function start() {
        paintChrome();
        loadPrefs();
        paintVowelsBtn();

        if (!U || !Review) {
            var app = document.getElementById('review-app');
            if (app) {
                app.innerHTML = '<div class="card"><p>'
                    + bi('שגיאה בטעינת מנוע החזרה.', 'Review engine failed to load.')
                    + '</p></div>';
            }
            return;
        }

        var list = vocabList();
        if (list == null) {
            renderMissing();
            return;
        }

        var vocabMap = indexByKey(list);
        buildClips(list);
        Review.load();
        var store = Review.all();
        var rawQueue = Review.dueList(store, Date.now());
        state.queue = filterQueue(rawQueue, store.words || {});
        state.index = 0;
        state.reviewed = 0;
        state.passed = 0;
        state.missed = 0;
        state.finished = false;

        bind(vocabMap);

        if (!state.queue.length) {
            renderEmpty();
            return;
        }
        renderItem(vocabMap);
    }

    function sharedBase() {
        try {
            var scripts = document.getElementsByTagName('script');
            for (var i = 0; i < scripts.length; i += 1) {
                var src = scripts[i].src || '';
                if (/\/review-page\.js(\?|$)/.test(src)) {
                    return src.replace(/review-page\.js(\?.*)?$/, '');
                }
            }
        } catch (err) { /* fall through */ }
        return '../shared/';
    }

    function loadScript(src) {
        return new Promise(function (resolve) {
            var el = document.createElement('script');
            el.src = src;
            el.onload = function () { resolve(true); };
            el.onerror = function () { resolve(false); };
            (document.head || document.documentElement).appendChild(el);
        });
    }

    function ensureSyncThenStart() {
        var base = sharedBase();
        var chain = Promise.resolve(true);
        if (!root.HebrewApi) {
            chain = chain.then(function () { return loadScript(base + 'api.js'); });
        }
        if (!root.HebrewSync) {
            chain = chain.then(function () { return loadScript(base + 'player/sync.js'); });
        }
        return chain.then(function () {
            var Sync = root.HebrewSync;
            if (Sync && typeof Sync.init === 'function') {
                return Sync.init({}).then(function () {
                    if (Sync.paintStatus) Sync.paintStatus();
                    start();
                }).catch(function () { start(); });
            }
            start();
        }).catch(function () { start(); });
    }

    /**
     * Typed en→he check (vowel-insensitive) — exported for tests.
     * @param {string} typed
     * @param {string} expectedWord
     */
    function typedMatches(typed, expectedWord) {
        if (!U || typeof U.normalizeHebrew !== 'function') return false;
        var got = U.normalizeHebrew(typed);
        var want = U.normalizeHebrew(expectedWord);
        return !!(got && want && got === want);
    }

    var api = {
        filterQueue: filterQueue,
        audioHref: audioHref,
        escapeIndexText: esc,
        typedMatches: typedMatches,
        vocabListPresent: function (raw) {
            var prev = root.HEBREW_VOCAB_INDEX;
            try {
                root.HEBREW_VOCAB_INDEX = raw;
                return vocabList() != null;
            } finally {
                root.HEBREW_VOCAB_INDEX = prev;
            }
        },
        PREFS_KEY: PREFS_KEY,
        VOWELS_DEFAULT: VOWELS_DEFAULT,
        start: start
    };

    root.HebrewReviewPage = api;
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    }

    if (typeof document !== 'undefined') {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', ensureSyncThenStart);
        } else {
            ensureSyncThenStart();
        }
    }
})(typeof window !== 'undefined' ? window : globalThis);
