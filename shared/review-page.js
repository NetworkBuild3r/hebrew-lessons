/**
 * Review screen: four-grade flashcards with calm recall + mastery summary.
 * Grades go to the server; never schedule in the browser.
 * Button intervals are the numbers on each card from GET /api/me/cards.
 * Authored-by-spec: INIT-006/SPEC-012, INIT-007/SPEC-006
 */
(function (root) {
    'use strict';

    var U = root.HebrewUtils;
    var Answers = root.HebrewFlashcardAnswer;
    if (!Answers && typeof require === 'function') {
        try { Answers = require('./player/flashcard-answer.js'); } catch (err) { Answers = null; }
    }

    var PREFS_KEY = 'hebrew-review-prefs:v1';
    var VOWELS_DEFAULT = true;
    var DAY_MS = 24 * 60 * 60 * 1000;
    var PERSONAL = { self: true, ai: true, lesson: true };
    var NAMED_DECKS = [
        { id: 'due', he: 'להיום', en: 'Due' },
        { id: 'weak', he: 'חלשות', en: 'Weak' },
        { id: 'new', he: 'חדשות', en: 'New' },
        { id: 'all', he: 'הכל', en: 'All' }
    ];
    var GRADE_LABELS = [
        { grade: 1, he: 'שוב', en: 'Again', signal: 'miss', family: 'miss' },
        { grade: 2, he: 'קשה', en: 'Hard', signal: 'pass', family: 'miss' },
        { grade: 3, he: 'טוב', en: 'Good', signal: 'pass', family: 'knew' },
        { grade: 4, he: 'קל', en: 'Easy', signal: 'pass', family: 'knew' }
    ];

    var state = {
        vowelsOn: VOWELS_DEFAULT,
        translitOn: false,
        deck: 'due',
        counts: Object.create(null),
        concepts: [],
        sourcesByWord: Object.create(null),
        lexicon: Object.create(null),
        queue: [],
        index: 0,
        revealed: false,
        grading: false,
        reviewed: 0,
        gradeCounts: { 1: 0, 2: 0, 3: 0, 4: 0 },
        nextDueMs: null,
        sessionNextDueMs: null,
        phase: 'loading',
        gen: 0,
        mountId: 0
    };

    var viewSignal = null;
    var mountedEl = null;
    var keyHandler = null;

    function biText(node, he, en) {
        // The separating space sits outside the isolated English span so the gloss is not glued
        // to the Hebrew in a right-to-left line (INIT-005/SPEC-011). textContent is unchanged.
        node.appendChild(root.document.createTextNode(en ? he + ' ' : he));
        if (!en) return;
        var span = root.document.createElement('span');
        span.className = 'en';
        span.dir = 'ltr';
        span.lang = 'en';
        span.textContent = '(' + en + ')';
        node.appendChild(span);
    }

    function clear(node) {
        if (!node) return;
        while (node.firstChild) node.removeChild(node.firstChild);
    }

    function el(tag, className) {
        var node = root.document.createElement(tag);
        if (className) node.className = className;
        return node;
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

    function appEl() {
        return mountedEl || (root.document && root.document.getElementById('review-app'));
    }

    function api() {
        return root.HebrewApi && typeof root.HebrewApi.get === 'function' ? root.HebrewApi : null;
    }

    function loadPrefs() {
        try {
            var raw = root.localStorage.getItem(PREFS_KEY);
            if (!raw) return;
            var data = JSON.parse(raw);
            if (typeof data.vowelsOn === 'boolean') state.vowelsOn = data.vowelsOn;
            if (typeof data.translitOn === 'boolean') state.translitOn = data.translitOn;
        } catch (err) { /* display prefs only; a bad value stays at the default */ }
    }

    function savePrefs() {
        try {
            root.localStorage.setItem(PREFS_KEY, JSON.stringify({
                vowelsOn: state.vowelsOn,
                translitOn: state.translitOn
            }));
        } catch (err) { /* display prefs only */ }
    }

    /**
     * Resolve a clip to an http(s) URL under the review worksheets base.
     * @param {string} url
     * @returns {string}
     */
    function audioHref(url) {
        if (!url) return '';
        var raw = String(url);
        if (/[\u0000-\u001F\u007F]/.test(raw)) return '';
        try {
            var base = new URL('/worksheets/_review/', root.location.href);
            var resolved = new URL(raw, base);
            if (resolved.protocol !== 'http:' && resolved.protocol !== 'https:') return '';
            return resolved.href;
        } catch (err) {
            return '';
        }
    }

    function indexByKey(list) {
        var map = Object.create(null);
        (list || []).forEach(function (entry) {
            if (!entry) return;
            var key = entry.key || entry.word;
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
                var entry = raw[k];
                if (!entry || typeof entry !== 'object') return null;
                return Object.assign({ key: entry.key || k }, entry);
            }).filter(Boolean);
        }
        return null;
    }

    /**
     * aud-1: en→he only after the first he→en pass. Kept for the home due count.
     * @param {Array} items
     * @param {object} words
     */
    function filterQueue(items, words) {
        return (items || []).filter(function (item) {
            if (!item || item.direction !== 'en_he') return true;
            var entry = words && words[item.key];
            var he = entry && entry.he_en;
            return !!(he && he.lastResult === 'pass');
        });
    }

    function displayHebrew(text) {
        if (!text) return '';
        if (state.vowelsOn) return text;
        return U && typeof U.stripVowels === 'function' ? U.stripVowels(text) : text;
    }

    function intervalDays(card, grade) {
        if (!card || !card.intervals) return null;
        var n = Number(card.intervals[grade]);
        return Number.isFinite(n) ? n : null;
    }

    function intervalPhrase(days) {
        var n = Number(days);
        if (!Number.isFinite(n)) return null;
        return {
            he: n === 1 ? '1 יום' : (n + ' ימים'),
            en: n === 1 ? '1 day' : (n + ' days')
        };
    }

    function noteNextDueMs(ms) {
        var n = Number(ms);
        if (!Number.isFinite(n) || n <= Date.now()) return;
        if (state.nextDueMs == null || n < state.nextDueMs) state.nextDueMs = n;
    }

    function noteSessionNextDueMs(days) {
        var n = Number(days);
        if (!Number.isFinite(n) || n < 0) return;
        var ms = Date.now() + Math.max(n, 0) * DAY_MS;
        if (state.sessionNextDueMs == null || ms < state.sessionNextDueMs) {
            state.sessionNextDueMs = ms;
        }
    }

    function formatDueMs(ms) {
        if (ms == null || !Number.isFinite(ms)) {
            return { he: 'מחר בבוקר', en: 'tomorrow morning' };
        }
        var d = new Date(ms);
        return {
            he: d.toLocaleString('he-IL', {
                weekday: 'long', month: 'short', day: 'numeric',
                hour: '2-digit', minute: '2-digit'
            }),
            en: d.toLocaleString('en-US', {
                weekday: 'long', month: 'short', day: 'numeric',
                hour: 'numeric', minute: '2-digit'
            })
        };
    }

    function appendNextDueLine(parent, ms) {
        var line = el('p', 'fine review-next-due');
        line.id = 'review-next-due';
        var when = formatDueMs(ms);
        biText(line, 'החזרה הבאה: ' + when.he, when.en);
        parent.appendChild(line);
        return line;
    }

    function lessonSlug(word) {
        var sources = state.sourcesByWord[word] || [];
        var i;
        for (i = 0; i < sources.length; i += 1) {
            if (sources[i] && !PERSONAL[sources[i]]) return String(sources[i]);
        }
        var list = vocabList();
        var entry = list ? indexByKey(list)[word] : null;
        if (entry && Array.isArray(entry.lessons) && entry.lessons[0]) return String(entry.lessons[0]);
        var catalog = root.HEBREW_CATALOG || [];
        if (catalog[0] && catalog[0].slug) return String(catalog[0].slug);
        return '';
    }

    function normalizeCard(raw) {
        var intervals = raw && raw.intervals && typeof raw.intervals === 'object' ? raw.intervals : {};
        return {
            word: raw && raw.word != null ? String(raw.word) : '',
            direction: raw && raw.direction === 'en_he' ? 'en_he' : 'he_en',
            prompt: raw && raw.prompt != null ? String(raw.prompt) : '',
            answer: raw && raw.answer != null ? String(raw.answer) : '',
            translit: raw && raw.translit != null ? String(raw.translit) : '',
            audio: raw && raw.audio != null ? String(raw.audio) : '',
            intervals: {
                1: Number(intervals[1]),
                2: Number(intervals[2]),
                3: Number(intervals[3]),
                4: Number(intervals[4])
            },
            accept: [],
            root: '',
            binyan: '',
            replayed: false
        };
    }

    function current() {
        return state.queue[state.index] || null;
    }

    function clipFor(text, audio) {
        var href = audioHref(audio);
        if (href) return href;
        var Speech = root.HebrewSpeech;
        if (Speech && typeof Speech.clipFor === 'function') return Speech.clipFor(text) || '';
        return '';
    }

    function paintToggles() {
        var vowels = root.document.getElementById('review-vowels-btn');
        if (vowels) {
            vowels.setAttribute('aria-pressed', state.vowelsOn ? 'true' : 'false');
            clear(vowels);
            biText(vowels, state.vowelsOn ? 'הסתר ניקוד' : 'הצג ניקוד', state.vowelsOn ? 'Hide vowels' : 'Show vowels');
        }
        var translit = root.document.getElementById('review-translit-btn');
        if (translit) {
            translit.setAttribute('aria-pressed', state.translitOn ? 'true' : 'false');
            clear(translit);
            biText(translit, 'תעתיק', 'Transliteration');
        }
    }

    function repaintHebrew() {
        var nodes = root.document.querySelectorAll('[data-vowels]');
        Array.prototype.forEach.call(nodes, function (node) {
            node.textContent = displayHebrew(node.getAttribute('data-vowels') || '');
        });
    }

    function countNode(deck) {
        var nodes = root.document.querySelectorAll('[data-deck-count]');
        for (var i = 0; i < nodes.length; i += 1) {
            if (nodes[i].getAttribute('data-deck-count') === deck) return nodes[i];
        }
        return null;
    }

    function setCount(deck, n) {
        state.counts[deck] = n;
        var node = countNode(deck);
        if (!node) return;
        if (node.tagName === 'OPTION') {
            var base = node.getAttribute('data-label') || '';
            node.textContent = base + ' · ' + n;
            return;
        }
        node.textContent = String(n);
    }

    function updatePressed() {
        var buttons = root.document.querySelectorAll('[data-deck]');
        Array.prototype.forEach.call(buttons, function (btn) {
            var on = btn.getAttribute('data-deck') === state.deck;
            btn.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
        var lesson = root.document.getElementById('review-lesson-deck');
        if (lesson && lesson.value !== state.deck) {
            var lessonMatch = false;
            for (var i = 0; i < lesson.options.length; i += 1) {
                if (lesson.options[i].value === state.deck) lessonMatch = true;
            }
            if (!lessonMatch) lesson.value = '';
        }
        var concept = root.document.getElementById('review-concept-deck');
        if (concept && concept.value !== state.deck) {
            var conceptMatch = false;
            for (var j = 0; j < concept.options.length; j += 1) {
                if (concept.options[j].value === state.deck) conceptMatch = true;
            }
            if (!conceptMatch) concept.value = '';
        }
    }

    function deckButton(spec) {
        var btn = el('button', 'btn review-deck-btn');
        btn.type = 'button';
        btn.setAttribute('data-action', 'deck');
        btn.setAttribute('data-deck', spec.id);
        btn.setAttribute('aria-pressed', state.deck === spec.id ? 'true' : 'false');
        biText(btn, spec.he, spec.en);
        var count = el('span', 'review-deck-count');
        count.setAttribute('data-deck-count', spec.id);
        count.textContent = state.counts[spec.id] == null ? '…' : String(state.counts[spec.id]);
        btn.appendChild(root.document.createTextNode(' '));
        btn.appendChild(count);
        return btn;
    }

    function selectField(id, he, en) {
        var wrap = el('label', 'review-deck-field');
        var name = el('span', 'fine');
        biText(name, he, en);
        var select = el('select', 'review-deck-select');
        select.id = id;
        var blank = el('option');
        blank.value = '';
        blank.textContent = '—';
        select.appendChild(blank);
        wrap.appendChild(name);
        wrap.appendChild(select);
        return wrap;
    }

    function fillLessonOptions() {
        var select = root.document.getElementById('review-lesson-deck');
        if (!select) return;
        var catalog = Array.isArray(root.HEBREW_CATALOG) ? root.HEBREW_CATALOG : [];
        catalog.forEach(function (lesson) {
            if (!lesson || !lesson.slug) return;
            var deck = 'lesson:' + lesson.slug;
            var title = lesson.title ? String(lesson.title) : lesson.slug;
            var option = el('option');
            option.value = deck;
            option.setAttribute('data-deck-count', deck);
            option.setAttribute('data-label', title);
            option.textContent = state.counts[deck] == null ? title : (title + ' · ' + state.counts[deck]);
            select.appendChild(option);
        });
    }

    function fillConceptOptions() {
        var select = root.document.getElementById('review-concept-deck');
        if (!select || select.getAttribute('data-filled') === '1') return;
        if (!state.concepts.length) return;
        ['binyan', 'tense'].forEach(function (family) {
            var group = el('optgroup');
            group.label = family === 'binyan' ? 'בניין (binyan)' : 'זמן (tense)';
            state.concepts.forEach(function (concept) {
                if (!concept || concept.family !== family) return;
                var option = el('option');
                var deckName = 'concept:' + concept.key;
                var he = concept.labelHe || concept.key;
                var en = concept.labelEn || '';
                var title = en ? (he + ' (' + en + ')') : he;
                option.value = deckName;
                option.setAttribute('data-deck-count', deckName);
                option.setAttribute('data-label', title);
                option.textContent = state.counts[deckName] == null ? title : (title + ' · ' + state.counts[deckName]);
                group.appendChild(option);
            });
            if (group.childNodes.length) select.appendChild(group);
        });
        select.setAttribute('data-filled', '1');
    }

    function buildDecks() {
        var bar = el('div', 'review-decks');
        bar.id = 'review-decks';
        NAMED_DECKS.forEach(function (spec) {
            bar.appendChild(deckButton(spec));
        });
        var lesson = selectField('review-lesson-deck', 'לפי שיעור', 'By lesson');
        var concept = selectField('review-concept-deck', 'לפי נושא', 'By concept');
        bar.appendChild(lesson);
        bar.appendChild(concept);
        return bar;
    }

    function ensureFrame() {
        var app = appEl();
        if (!app) return null;
        if (!app._flashBound) {
            app._flashBound = true;
            app.addEventListener('click', onClick);
            app.addEventListener('change', onChange);
            app.addEventListener('input', onType);
        }
        var decks = root.document.getElementById('review-decks');
        if (!decks) {
            clear(app);
            app.appendChild(buildDecks());
            fillLessonOptions();
            fillConceptOptions();
            var stage = el('div', 'review-stage');
            stage.id = 'review-stage';
            app.appendChild(stage);
        }
        updatePressed();
        return root.document.getElementById('review-stage');
    }

    function audioButton(text, audio) {
        var href = clipFor(text, audio);
        if (!href) return null;
        var btn = el('button', 'btn review-audio');
        btn.type = 'button';
        btn.setAttribute('data-action', 'speak');
        btn.setAttribute('data-speak', text || '');
        btn.setAttribute('data-audio', href);
        biText(btn, 'השמעה', 'Listen');
        return btn;
    }

    function hebrewNode(text, id) {
        var node = el('div', 'he-word review-he');
        if (id) node.id = id;
        node.lang = 'he';
        node.dir = 'rtl';
        node.setAttribute('data-vowels', text || '');
        node.textContent = displayHebrew(text || '');
        return node;
    }

    function renderStage() {
        var stage = ensureFrame();
        if (!stage) return;
        state.grading = false;
        clear(stage);
        if (state.phase === 'loading') {
            var loading = el('div', 'card review-empty');
            loading.id = 'review-loading';
            var loadingText = el('p');
            biText(loadingText, 'טוען כרטיסים…', 'Loading cards…');
            loading.appendChild(loadingText);
            stage.appendChild(loading);
            return;
        }
        if (state.phase === 'error') {
            var error = el('div', 'card review-empty');
            error.id = 'review-load-error';
            error.setAttribute('role', 'alert');
            var errorTitle = el('h2');
            biText(errorTitle, 'לא הצלחנו לטעון כרטיסים', "Couldn't load cards");
            var errorBody = el('p');
            biText(errorBody, 'נסו שוב בעוד רגע.', 'Try again in a moment.');
            error.appendChild(errorTitle);
            error.appendChild(errorBody);
            stage.appendChild(error);
            return;
        }
        if (state.phase === 'empty') {
            stage.appendChild(buildEmpty());
            return;
        }
        if (state.phase === 'summary') {
            stage.appendChild(buildSummary());
            return;
        }
        stage.appendChild(buildCard(current()));
    }

    function buildEmpty() {
        var empty = el('div', 'card review-empty review-caught-up');
        empty.id = 'review-nothing-due';
        var emptyTitle = el('h2');
        if (state.deck === 'due') {
            biText(emptyTitle, 'הכול מעודכן!', 'All caught up!');
            empty.appendChild(emptyTitle);
            var emptyBody = el('p');
            biText(emptyBody, 'אין מילים לחזרה כרגע', 'No words to review right now');
            empty.appendChild(emptyBody);
            appendNextDueLine(empty, state.nextDueMs);
        } else {
            biText(emptyTitle, 'אין כרטיסים בחפיסה', 'Nothing in this deck');
            empty.appendChild(emptyTitle);
        }
        var home = el('a', 'btn btn-primary review-home');
        home.href = '/';
        home.id = 'review-back-today';
        biText(home, 'בחזרה להיום', 'Back to Today');
        empty.appendChild(home);
        return empty;
    }

    function buildCard(card) {
        var wrap = el('div', 'card review-card');
        wrap.id = 'review-card';
        wrap.tabIndex = -1;
        wrap.setAttribute('data-direction', card.direction);
        var slug = lessonSlug(card.word);
        if (slug) wrap.setAttribute('data-lesson', slug);
        var progress = el('div', 'fine review-progress');
        progress.id = 'review-progress';
        var n = state.index + 1;
        var total = state.queue.length;
        biText(progress, 'כרטיס ' + n + ' מתוך ' + total, 'Card ' + n + ' of ' + total);
        wrap.appendChild(progress);
        var dir = el('p', 'fine');
        var heEn = card.direction !== 'en_he';
        biText(dir, heEn ? 'עברית ← אנגלית' : 'אנגלית ← עברית', heEn ? 'Hebrew → English' : 'English → Hebrew');
        wrap.appendChild(dir);
        var prompt = el('div', 'review-prompt');
        prompt.id = 'review-prompt';
        if (heEn) {
            prompt.appendChild(hebrewNode(card.prompt, 'review-prompt-he'));
            var promptAudio = audioButton(card.prompt, card.audio);
            if (promptAudio) prompt.appendChild(promptAudio);
        } else {
            var meaning = el('p', 'meaning review-prompt-en');
            meaning.id = 'review-prompt-en';
            meaning.dir = 'ltr';
            meaning.lang = 'en';
            meaning.textContent = card.prompt;
            prompt.appendChild(meaning);
            var typedLabel = el('label', 'fine');
            typedLabel.htmlFor = 'review-typed';
            biText(typedLabel, 'כתבו בעברית (לא חובה)', 'Type in Hebrew (optional)');
            var input = el('input', 'review-typed');
            input.id = 'review-typed';
            input.type = 'text';
            input.lang = 'he';
            input.dir = 'rtl';
            input.autocomplete = 'off';
            input.setAttribute('aria-label', 'הקלידו את התשובה בעברית');
            prompt.appendChild(typedLabel);
            prompt.appendChild(input);
        }
        wrap.appendChild(prompt);
        var hint = el('p', 'fine');
        biText(hint, 'רווח מציג את התשובה. אחר כך 1–4.', 'Space shows the answer. Then 1–4.');
        wrap.appendChild(hint);
        var actions = el('div', 'review-actions');
        actions.id = 'review-reveal-row';
        var show = el('button', 'btn btn-primary');
        show.type = 'button';
        show.id = 'review-show-answer';
        show.setAttribute('data-action', 'reveal');
        biText(show, 'הצג תשובה', 'Show answer');
        actions.appendChild(show);
        wrap.appendChild(actions);
        return wrap;
    }

    function buildAnswer(card) {
        var box = el('div', 'review-answer review-answer-reveal');
        box.id = 'review-answer';
        var heEn = card.direction !== 'en_he';
        if (heEn) {
            var gloss = el('p', 'meaning');
            gloss.id = 'review-answer-meaning';
            gloss.dir = 'ltr';
            gloss.lang = 'en';
            gloss.textContent = card.answer;
            box.appendChild(gloss);
            // Hebrew was the prompt; keep listen available after reveal.
            var promptAudio = audioButton(card.prompt, card.audio);
            if (promptAudio) box.appendChild(promptAudio);
        } else {
            box.appendChild(hebrewNode(card.answer, 'review-answer-he'));
            var answerAudio = audioButton(card.answer, card.audio);
            if (answerAudio) box.appendChild(answerAudio);
        }
        var translitSlot = el('div');
        translitSlot.id = 'review-translit-slot';
        fillTranslit(translitSlot, card);
        box.appendChild(translitSlot);
        var chips = el('div', 'review-chips');
        chips.id = 'review-chips';
        fillChips(chips, card);
        box.appendChild(chips);
        var grades = el('div', 'review-grades');
        grades.id = 'review-grade-row';
        GRADE_LABELS.forEach(function (item) {
            grades.appendChild(gradeButton(card, item));
        });
        var suggestion = el('p', 'fine review-suggestion');
        suggestion.id = 'review-suggestion';
        box.appendChild(grades);
        box.appendChild(suggestion);
        return box;
    }

    function gradeButton(card, item) {
        var family = item.family === 'knew' ? 'knew' : 'miss';
        var btn = el('button', 'btn review-grade review-grade-' + item.grade + ' review-grade-' + family);
        btn.type = 'button';
        btn.id = 'review-grade-' + item.grade;
        btn.setAttribute('data-action', 'grade');
        btn.setAttribute('data-grade', String(item.grade));
        btn.setAttribute('data-signal', item.signal || (item.grade === 1 ? 'miss' : 'pass'));
        btn.setAttribute('aria-keyshortcuts', String(item.grade));
        var key = el('kbd', 'review-key');
        key.textContent = String(item.grade);
        btn.appendChild(key);
        var name = el('span', 'review-grade-name');
        biText(name, item.he, item.en);
        btn.appendChild(name);
        var days = intervalDays(card, item.grade);
        var phrase = intervalPhrase(days);
        if (phrase) {
            var interval = el('span', 'review-interval');
            interval.setAttribute('data-days', String(days));
            biText(interval, phrase.he, phrase.en);
            btn.appendChild(interval);
        }
        return btn;
    }

    function fillTranslit(slot, card) {
        clear(slot);
        if (!state.translitOn || !card || !card.translit) return;
        var line = el('p', 'review-translit');
        line.dir = 'ltr';
        line.lang = 'en';
        line.textContent = card.translit;
        slot.appendChild(line);
    }

    function paintTranslit(card) {
        var slot = root.document.getElementById('review-translit-slot');
        if (!slot) return;
        fillTranslit(slot, card);
    }

    function fillChips(slot, card) {
        clear(slot);
        if (!card) return;
        if (card.root) slot.appendChild(chip('שורש', 'Root', card.root, true));
        if (card.binyan) slot.appendChild(chip('בניין', 'Binyan', card.binyan, false));
    }

    function paintChips(card) {
        var slot = root.document.getElementById('review-chips');
        if (!slot) return;
        fillChips(slot, card);
    }

    function chip(he, en, value, hebrew) {
        var node = el('span', 'review-chip');
        var name = el('span', 'review-chip-label');
        biText(name, he, en);
        node.appendChild(name);
        var text = el('span', 'review-chip-value');
        if (hebrew) {
            text.lang = 'he';
            text.dir = 'rtl';
        } else {
            text.dir = 'ltr';
            text.lang = 'en';
        }
        text.textContent = value;
        node.appendChild(text);
        return node;
    }

    function typedValue() {
        var input = root.document.getElementById('review-typed');
        return input ? input.value : '';
    }

    function applySuggestion() {
        var card = current();
        var note = root.document.getElementById('review-suggestion');
        var buttons = root.document.querySelectorAll('.review-grade');
        Array.prototype.forEach.call(buttons, function (btn) {
            btn.classList.remove('review-suggested');
        });
        if (note) clear(note);
        if (!card || card.direction !== 'en_he' || !Answers) return;
        var suggested = Answers.suggestGrade(typedValue(), card.word, card.accept);
        if (suggested == null) return;
        var btn = root.document.getElementById('review-grade-' + suggested);
        if (btn) btn.classList.add('review-suggested');
        if (!note) return;
        var label = GRADE_LABELS[suggested - 1];
        biText(note, 'הצעה: ' + label.he, 'suggested: ' + label.en);
    }

    function reveal() {
        if (state.revealed || state.phase !== 'card') return;
        var card = current();
        var wrap = root.document.getElementById('review-card');
        if (!card || !wrap) return;
        state.revealed = true;
        var row = root.document.getElementById('review-reveal-row');
        if (row) row.remove();
        wrap.appendChild(buildAnswer(card));
        applySuggestion();
    }

    function commitGrade(grade) {
        if (!state.revealed || state.phase !== 'card' || state.grading) return;
        var card = current();
        if (!card || !Answers) return;
        var signal = Answers.gradeSignal(card, grade, lessonSlug(card.word));
        if (!signal) return;
        state.grading = true;
        var Sync = root.HebrewSync;
        if (Sync && typeof Sync.queueSignal === 'function') Sync.queueSignal(signal);
        state.reviewed += 1;
        state.gradeCounts[grade] += 1;
        if (grade > 1) noteSessionNextDueMs(intervalDays(card, grade));
        if (grade === 1 && !card.replayed) {
            state.queue.push({
                word: card.word,
                direction: card.direction,
                prompt: card.prompt,
                answer: card.answer,
                translit: card.translit,
                audio: card.audio,
                intervals: card.intervals,
                accept: card.accept.slice(),
                root: card.root,
                binyan: card.binyan,
                replayed: true
            });
        }
        state.index += 1;
        state.revealed = false;
        state.phase = state.index >= state.queue.length ? 'summary' : 'card';
        renderStage();
        if (state.phase === 'card') prefetchLexicon(current());
    }

    function buildSummary() {
        var box = el('div', 'card review-summary mastery-ceremony complete-ceremony');
        box.id = 'review-finished';
        var title = el('h2');
        title.id = 'review-summary-title';
        biText(title, 'סיבוב הושלם', 'Round Completed!');
        box.appendChild(title);
        var lead = el('p', 'review-summary-lead');
        biText(lead, 'מילים שתרגלתם עכשיו', 'Words you practiced now');
        box.appendChild(lead);
        var list = el('ul', 'review-stats');
        var reviewed = el('li');
        biText(reviewed, 'נבדקו', 'Reviewed');
        reviewed.appendChild(root.document.createTextNode(': '));
        var reviewedN = el('strong');
        reviewedN.id = 'review-count-reviewed';
        reviewedN.textContent = String(state.reviewed);
        reviewed.appendChild(reviewedN);
        list.appendChild(reviewed);
        var mastered = el('li');
        biText(mastered, 'ידעתי / טוב וקל', 'Knew · Good & Easy');
        mastered.appendChild(root.document.createTextNode(': '));
        var masteredN = el('strong');
        masteredN.id = 'review-count-knew';
        masteredN.textContent = String(
            (state.gradeCounts[3] || 0) + (state.gradeCounts[4] || 0)
        );
        mastered.appendChild(masteredN);
        list.appendChild(mastered);
        GRADE_LABELS.forEach(function (item) {
            var row = el('li');
            biText(row, item.he, item.en);
            row.appendChild(root.document.createTextNode(': '));
            var n = el('strong');
            n.id = 'review-count-' + item.grade;
            n.textContent = String(state.gradeCounts[item.grade] || 0);
            row.appendChild(n);
            list.appendChild(row);
        });
        box.appendChild(list);
        appendNextDueLine(box, state.sessionNextDueMs != null ? state.sessionNextDueMs : state.nextDueMs);
        var actions = el('div', 'review-summary-actions path-actions');
        var home = el('a', 'btn btn-primary review-home');
        home.href = '/';
        home.id = 'review-back-today';
        biText(home, 'בחזרה להיום', 'Back to Today');
        actions.appendChild(home);
        var again = el('button', 'btn btn-again');
        again.type = 'button';
        again.id = 'review-practice-again';
        again.setAttribute('data-action', 'again-deck');
        biText(again, 'עוד סיבוב', 'Practice again');
        actions.appendChild(again);
        box.appendChild(actions);
        return box;
    }

    function onClick(event) {
        var btn = event.target && event.target.closest ? event.target.closest('[data-action]') : null;
        if (!btn) return;
        var action = btn.getAttribute('data-action');
        if (action === 'deck') {
            openDeck(btn.getAttribute('data-deck') || 'due');
            return;
        }
        if (action === 'speak') {
            var Speech = root.HebrewSpeech;
            if (Speech && typeof Speech.play === 'function') {
                Speech.play(btn.getAttribute('data-speak') || '', btn.getAttribute('data-audio') || '');
            }
            return;
        }
        if (action === 'reveal') {
            reveal();
            return;
        }
        if (action === 'grade') {
            commitGrade(Number(btn.getAttribute('data-grade')));
            return;
        }
        if (action === 'again-deck') {
            openDeck(state.deck || 'due');
        }
    }

    function onChange(event) {
        var target = event.target;
        if (!target) return;
        if (target.id !== 'review-lesson-deck' && target.id !== 'review-concept-deck') return;
        if (!target.value) return;
        openDeck(target.value);
    }

    function onType(event) {
        if (!event.target || event.target.id !== 'review-typed') return;
        if (state.revealed) applySuggestion();
    }

    function onKey(event) {
        if (!mountedEl || state.phase !== 'card') return;
        var tag = (event.target && event.target.tagName) || '';
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
            if (event.key === 'Enter' && !state.revealed) {
                event.preventDefault();
                reveal();
            }
            return;
        }
        if (event.key === ' ' || event.code === 'Space') {
            event.preventDefault();
            if (!state.revealed) reveal();
            return;
        }
        if (!state.revealed || !Answers) return;
        var grade = Answers.keyToGrade(event.key);
        if (!grade) return;
        event.preventDefault();
        commitGrade(grade);
    }

    function bindKeys() {
        if (keyHandler) root.document.removeEventListener('keydown', keyHandler);
        keyHandler = function (event) { onKey(event); };
        root.document.addEventListener('keydown', keyHandler, viewSignal ? { signal: viewSignal } : false);
    }

    function applyLexicon(card, info) {
        card.accept = info.accept.slice();
        card.root = info.root;
        card.binyan = info.binyan;
        if (state.revealed && current() === card) {
            paintChips(card);
            applySuggestion();
        }
    }

    function prefetchLexicon(card) {
        if (!card || !card.word) return;
        var cached = state.lexicon[card.word];
        if (cached) {
            applyLexicon(card, cached);
            return;
        }
        var client = api();
        if (!client) return;
        var word = card.word;
        var mountId = state.mountId;
        client.get('/api/lexicon/' + encodeURIComponent(word), { redirectOn401: false }).then(function (res) {
            if (mountId !== state.mountId) return;
            var info = { accept: [], root: '', binyan: '' };
            if (res && res.ok && res.data && res.data.entry) {
                var entry = res.data.entry;
                var lemma = res.data.lemma || {};
                info.accept = Array.isArray(entry.accept)
                    ? entry.accept.filter(function (item) { return typeof item === 'string'; })
                    : [];
                info.root = entry.root || lemma.root || '';
                info.binyan = entry.binyan || lemma.binyan || '';
            }
            state.lexicon[word] = info;
            var live = current();
            if (live && live.word === word) applyLexicon(live, info);
        }).catch(function () { /* the stripped key still scores a typed answer */ });
    }

    function loadSideData(mountId) {
        var client = api();
        if (!client) return;
        client.get('/api/me/library?limit=200', { redirectOn401: false }).then(function (res) {
            if (mountId !== state.mountId) return;
            var words = res && res.ok && res.data && Array.isArray(res.data.words) ? res.data.words : [];
            words.forEach(function (row) {
                if (!row || row.word == null) return;
                state.sourcesByWord[String(row.word)] = Array.isArray(row.sources) ? row.sources.map(String) : [];
                ['he_en', 'en_he'].forEach(function (dir) {
                    var days = row[dir] && row[dir].due_in_days;
                    var n = Number(days);
                    if (!Number.isFinite(n) || n <= 0) return;
                    noteNextDueMs(Date.now() + n * DAY_MS);
                });
            });
            if (state.phase === 'empty') renderStage();
        }).catch(function () { /* vocab-index lessons remain the fallback slug */ });
        client.get('/api/grammar/concepts', { redirectOn401: false }).then(function (res) {
            if (mountId !== state.mountId) return;
            var rows = res && res.ok && res.data && Array.isArray(res.data.concepts) ? res.data.concepts : [];
            state.concepts = rows.filter(function (row) {
                return row && (row.family === 'binyan' || row.family === 'tense');
            }).map(function (row) {
                return {
                    key: String(row.key || ''),
                    family: row.family,
                    labelHe: row.label_he || '',
                    labelEn: row.label_en || ''
                };
            });
            fillConceptOptions();
            state.concepts.forEach(function (concept) {
                fetchCount('concept:' + concept.key, mountId);
            });
        }).catch(function () { /* named decks still work without concept chips */ });
        var catalog = Array.isArray(root.HEBREW_CATALOG) ? root.HEBREW_CATALOG : [];
        catalog.forEach(function (lesson) {
            if (lesson && lesson.slug) fetchCount('lesson:' + lesson.slug, mountId);
        });
        NAMED_DECKS.forEach(function (spec) {
            if (spec.id !== state.deck) fetchCount(spec.id, mountId);
        });
    }

    function fetchCount(deck, mountId) {
        var client = api();
        if (!client) return;
        client.get('/api/me/cards?deck=' + encodeURIComponent(deck), { redirectOn401: false }).then(function (res) {
            if (mountId !== state.mountId) return;
            if (!res || !res.ok || !res.data || !Array.isArray(res.data.cards)) return;
            setCount(deck, res.data.cards.length);
        }).catch(function () { /* the deck name stays; the count stays blank */ });
    }

    function resetSession() {
        state.queue = [];
        state.index = 0;
        state.revealed = false;
        state.grading = false;
        state.reviewed = 0;
        state.gradeCounts = { 1: 0, 2: 0, 3: 0, 4: 0 };
        state.sessionNextDueMs = null;
    }

    function openDeck(deck) {
        var name = deck || 'due';
        var gen = state.gen + 1;
        state.gen = gen;
        state.deck = name;
        resetSession();
        state.phase = 'loading';
        renderStage();
        var client = api();
        if (!client) {
            state.phase = 'error';
            renderStage();
            return Promise.resolve();
        }
        return client.get('/api/me/cards?deck=' + encodeURIComponent(name), { redirectOn401: false })
            .then(function (res) {
                if (gen !== state.gen || !mountedEl) return;
                if (!res || !res.ok || !res.data || !Array.isArray(res.data.cards)) {
                    state.phase = 'error';
                    renderStage();
                    return;
                }
                state.queue = res.data.cards.map(normalizeCard);
                setCount(name, state.queue.length);
                state.phase = state.queue.length ? 'card' : 'empty';
                renderStage();
                if (state.phase === 'card') prefetchLexicon(current());
            }).catch(function () {
                if (gen !== state.gen || !mountedEl) return;
                state.phase = 'error';
                renderStage();
            });
    }

    function toggleButton(id, onClickToggle) {
        var btn = el('button', 'btn btn-ghost');
        btn.type = 'button';
        btn.id = id;
        btn.setAttribute('aria-pressed', 'false');
        btn.addEventListener('click', onClickToggle);
        return btn;
    }

    function mount(elNode, ctx) {
        mountedEl = elNode;
        viewSignal = (ctx && ctx.signal) || null;
        state.mountId += 1;
        var mountId = state.mountId;
        loadPrefs();
        if (ctx && ctx.actions) {
            clear(ctx.actions);
            ctx.actions.appendChild(toggleButton('review-vowels-btn', function () {
                state.vowelsOn = !state.vowelsOn;
                savePrefs();
                paintToggles();
                repaintHebrew();
            }));
            ctx.actions.appendChild(toggleButton('review-translit-btn', function () {
                state.translitOn = !state.translitOn;
                savePrefs();
                paintToggles();
                paintTranslit(current());
            }));
        }
        paintToggles();
        bindKeys();
        var ready = (ctx && ctx.ready) || Promise.resolve(null);
        return ready.then(function () {
            if (mountedEl !== elNode) return null;
            loadSideData(mountId);
            return openDeck('due');
        }).catch(function () {
            if (mountedEl === elNode) {
                state.phase = 'error';
                renderStage();
            }
        });
    }

    function unmount() {
        if (keyHandler && root.document) {
            root.document.removeEventListener('keydown', keyHandler);
        }
        keyHandler = null;
        state.gen += 1;
        state.mountId += 1;
        mountedEl = null;
        viewSignal = null;
    }

    /**
     * @param {string} typed
     * @param {string} expectedWord
     * @param {string[]} [accept]
     */
    function typedMatches(typed, expectedWord, accept) {
        if (!Answers) return false;
        return Answers.matches(typed, expectedWord, accept);
    }

    var pageApi = {
        filterQueue: filterQueue,
        audioHref: audioHref,
        escapeIndexText: esc,
        typedMatches: typedMatches,
        keyToGrade: function (key) { return Answers ? Answers.keyToGrade(key) : null; },
        suggestGrade: function (typed, key, accept) {
            return Answers ? Answers.suggestGrade(typed, key, accept) : null;
        },
        gradeSignal: function (card, grade, slug) {
            return Answers ? Answers.gradeSignal(card, grade, slug) : null;
        },
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
        start: function () { return openDeck(state.deck || 'due'); },
        mount: mount,
        unmount: unmount
    };

    root.HebrewReviewPage = pageApi;
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = pageApi;
    }
})(typeof window !== 'undefined' ? window : globalThis);
