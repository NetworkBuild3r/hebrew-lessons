/**
 * Words library — the learner's own words.
 * Data comes only from /api/me/library and /api/lexicon; do not read review
 * state from localStorage.
 * Provenance: INIT-006/SPEC-013
 */
(function (root) {
    'use strict';

    var FILTER_KEYS = ['q', 'status', 'weak', 'pos', 'binyan', 'tense', 'source', 'suspended'];
    var LESSON_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var PERSONAL = { self: true, ai: true, lesson: true };

    var POS = [
        ['noun', 'שם עצם', 'noun'],
        ['verb', 'פועל', 'verb'],
        ['adjective', 'שם תואר', 'adjective'],
        ['adverb', 'תואר הפועל', 'adverb'],
        ['preposition', 'מילת יחס', 'preposition'],
        ['pronoun', 'כינוי', 'pronoun'],
        ['conjunction', 'מילת חיבור', 'conjunction'],
        ['particle', 'מילית', 'particle'],
        ['numeral', 'מספר', 'numeral'],
        ['other', 'אחר', 'other']
    ];
    var BINYAN = [
        ['paal', 'פָּעַל', 'paal'],
        ['nifal', 'נִפְעַל', 'nifal'],
        ['piel', 'פִּעֵל', 'piel'],
        ['pual', 'פֻּעַל', 'pual'],
        ['hifil', 'הִפְעִיל', 'hifil'],
        ['hufal', 'הֻפְעַל', 'hufal'],
        ['hitpael', 'הִתְפַּעֵל', 'hitpael']
    ];
    var TENSE = [
        ['infinitive', 'שם פועל', 'infinitive'],
        ['present', 'זמן הווה', 'present'],
        ['past', 'זמן עבר', 'past'],
        ['future', 'זמן עתיד', 'future'],
        ['imperative', 'ציווי', 'imperative']
    ];
    var ROOT_CLASS = {
        regular: { he: 'שלם', en: 'regular', colour: 'green' },
        'lamed-he': { he: 'ל״ה', en: 'lamed-he', colour: 'yellow' },
        'peh-yod': { he: 'פ״י', en: 'peh-yod', colour: 'red' },
        'peh-nun': { he: 'פ״נ', en: 'peh-nun', colour: 'red' },
        'ayin-vav': { he: 'ע״ו', en: 'ayin-vav', colour: 'red' },
        irregular: { he: 'חריג', en: 'irregular', colour: 'red' }
    };
    var STAGES = {
        new: { he: 'חדש', en: 'new' },
        learning: { he: 'בלמידה', en: 'learning' },
        longTerm: { he: 'לטווח ארוך', en: 'long-term' }
    };
    var BUCKETS = {
        fragile: { pct: 25, he: 'שביר', en: 'fragile' },
        shaky: { pct: 50, he: 'רועד', en: 'shaky' },
        steady: { pct: 75, he: 'יציב', en: 'steady' },
        solid: { pct: 100, he: 'איתן', en: 'solid' }
    };
    var DIRS = {
        he_en: { he: 'עברית ← אנגלית', en: 'Hebrew to English' },
        en_he: { he: 'אנגלית ← עברית', en: 'English to Hebrew' }
    };
    var ERRORS = {
        gloss_required: ['צריך גם אנגלית.', 'Add an English gloss.'],
        invalid_key: ['המילה בעברית לא תקינה.', 'That Hebrew word is not valid.'],
        invalid_gloss: ['האנגלית לא תקינה.', 'That English gloss is not valid.'],
        invalid_filter: ['הסינון לא תקין.', 'That filter is not valid.'],
        not_found: ['המילה לא בספרייה.', 'That word is not in your library.'],
        bad_request: ['לא הצלחנו לשמור.', 'Could not save that.'],
        csrf: ['הבקשה נחסמה.', 'The request was blocked.']
    };

    var alive = false;
    var rootEl = null;
    var pageSignal = null;
    var listCtrl = null;
    var detailCtrl = null;
    var requestSeq = 0;
    var vowelsBtn = null;
    var state = emptyState();

    function emptyState() {
        return {
            filters: {},
            words: [],
            nextCursor: null,
            selected: null,
            detail: null,
            vowelsOn: true,
            adding: false
        };
    }

    function doc() {
        return root.document;
    }

    function el(tag, attrs) {
        var node = doc().createElement(tag);
        var a = attrs || {};
        if (a.className) node.className = a.className;
        Object.keys(a).forEach(function (key) {
            if (key === 'className' || key === 'text') return;
            if (a[key] == null || a[key] === false) return;
            node.setAttribute(key, String(a[key]));
        });
        if (a.text != null) node.textContent = String(a.text);
        for (var i = 2; i < arguments.length; i += 1) append(node, arguments[i]);
        return node;
    }

    function append(node, child) {
        if (child == null || child === false) return;
        if (Array.isArray(child)) {
            child.forEach(function (c) { append(node, c); });
            return;
        }
        if (typeof child === 'string') {
            node.appendChild(doc().createTextNode(child));
            return;
        }
        node.appendChild(child);
    }

    function clear(node) {
        if (!node) return;
        while (node.firstChild) node.removeChild(node.firstChild);
    }

    function bi(heText, enText) {
        if (!enText) return el('span', { lang: 'he', text: heText || '' });
        if (!heText) return el('span', { className: 'ui-en', lang: 'en', dir: 'ltr', text: enText });
        return el('span', { className: 'ui-pair' },
            el('span', { className: 'ui-he', lang: 'he', text: heText }),
            el('span', { className: 'ui-en', lang: 'en', dir: 'ltr', text: enText }));
    }

    function stripVowels(text) {
        var U = root.HebrewUtils;
        if (U && typeof U.stripVowels === 'function') return U.stripVowels(text);
        return String(text || '').replace(/[\u0591-\u05C7]/g, '');
    }

    function shownHebrew(text) {
        var raw = String(text || '');
        return state.vowelsOn ? raw : stripVowels(raw);
    }

    function heNode(text, className) {
        var node = el('span', {
            className: className || 'words-he',
            lang: 'he',
            dir: 'rtl'
        });
        node.setAttribute('data-vowels', String(text || ''));
        node.textContent = shownHebrew(text);
        return node;
    }

    function labelFor(pairs, value) {
        for (var i = 0; i < pairs.length; i += 1) {
            if (pairs[i][0] === value) return { he: pairs[i][1], en: pairs[i][2] };
        }
        return null;
    }

    function filtersFromUrl() {
        var params;
        try {
            params = new URLSearchParams((root.location && root.location.search) || '');
        } catch (err) {
            params = new URLSearchParams();
        }
        var filters = {};
        FILTER_KEYS.forEach(function (key) {
            var value = params.get(key);
            if (value) filters[key] = value;
        });
        return filters;
    }

    function urlFor(filters) {
        var params = new URLSearchParams();
        FILTER_KEYS.forEach(function (key) {
            if (filters[key]) params.set(key, filters[key]);
        });
        var q = params.toString();
        return '/words/' + (q ? '?' + q : '');
    }

    function currentUrl() {
        var loc = root.location || {};
        return String(loc.pathname || '/words/') + String(loc.search || '');
    }

    function libraryQuery(filters, cursor) {
        var params = new URLSearchParams();
        FILTER_KEYS.forEach(function (key) {
            if (filters[key]) params.set(key, filters[key]);
        });
        if (cursor) params.set('cursor', cursor);
        var q = params.toString();
        return '/api/me/library' + (q ? '?' + q : '');
    }

    function reviewHref(filters) {
        var deck = '';
        if (filters.binyan && labelFor(BINYAN, filters.binyan)) {
            deck = 'concept:binyan:' + filters.binyan;
        } else if (filters.tense && labelFor(TENSE, filters.tense)) {
            deck = 'concept:tense:' + filters.tense;
        } else if (filters.weak === '1' || filters.weak === 'weak' || filters.weak === 'very_weak') {
            deck = 'weak';
        }
        if (!deck) return '/review/';
        return '/review/?deck=' + encodeURIComponent(deck);
    }

    function audioHref(url) {
        if (!url) return '';
        var raw = String(url);
        if (/[\u0000-\u001F\u007F]/.test(raw)) return '';
        try {
            var base = new URL('/worksheets/_review/', root.location.href);
            var resolved = new URL(raw, base);
            if (resolved.origin !== base.origin) return '';
            if (resolved.protocol !== 'http:' && resolved.protocol !== 'https:') return '';
            return resolved.href;
        } catch (err) {
            return '';
        }
    }

    function linkedSignal(parent) {
        var ctrl = typeof root.AbortController === 'function' ? new root.AbortController() : null;
        if (!ctrl) return { ctrl: null, signal: undefined };
        if (parent) {
            if (parent.aborted) ctrl.abort();
            else parent.addEventListener('abort', function () { ctrl.abort(); }, { once: true });
        }
        return { ctrl: ctrl, signal: ctrl.signal };
    }

    function apiRequest(method, path, body, signal) {
        var Api = root.HebrewApi;
        if (!Api || typeof Api.request !== 'function') {
            return Promise.resolve({ ok: false, status: 0, error: 'API unavailable', code: null, data: null });
        }
        return Api.request(path, { method: method, body: body, signal: signal });
    }

    function errorText(res) {
        var code = (res && (res.code || res.error)) || '';
        var pair = ERRORS[code];
        if (pair) return pair;
        return ['לא הצלחנו לטעון.', 'Could not load that.'];
    }

    function field(name) {
        return rootEl && rootEl.querySelector('[data-filter="' + name + '"]');
    }

    function readForm() {
        var filters = {};
        FILTER_KEYS.forEach(function (key) {
            var node = field(key);
            var value = node ? String(node.value || '').trim() : '';
            if (value) filters[key] = value;
        });
        return filters;
    }

    function syncForm() {
        FILTER_KEYS.forEach(function (key) {
            var node = field(key);
            if (node) node.value = state.filters[key] || '';
        });
    }

    function updateReviewLink() {
        var link = rootEl && rootEl.querySelector('[data-words-review]');
        if (!link) return;
        var href = reviewHref(state.filters);
        link.setAttribute('href', href);
        var deck = href.indexOf('deck=') >= 0 ? decodeURIComponent(href.split('deck=')[1]) : 'due';
        link.setAttribute('data-review-deck', deck);
    }

    function setPageState(kind) {
        if (rootEl) rootEl.setAttribute('data-words-state', kind);
    }

    function fillStatus(kind, heText, enText) {
        var status = rootEl && rootEl.querySelector('[data-words-status]');
        if (!status) return;
        clear(status);
        status.setAttribute('data-words-status', kind);
        if (kind === 'error') status.setAttribute('role', 'alert');
        else status.removeAttribute('role');
        if (heText) status.appendChild(bi(heText, enText));
    }

    function option(value, heText, enText) {
        var node = el('option', { value: value });
        node.textContent = heText + (enText ? ' (' + enText + ')' : '');
        return node;
    }

    function selectFilter(name, heText, enText, choices) {
        var id = 'words-filter-' + name;
        var wrap = el('label', { className: 'words-field', for: id });
        wrap.appendChild(bi(heText, enText));
        var select = el('select', { id: id, className: 'words-control' });
        select.setAttribute('data-filter', name);
        select.appendChild(option('', 'הכל', 'Any'));
        choices.forEach(function (choice) {
            select.appendChild(option(choice[0], choice[1], choice[2]));
        });
        wrap.appendChild(select);
        return wrap;
    }

    function build() {
        clear(rootEl);
        rootEl.className = 'words-page';
        setPageState('loading');

        var filters = el('form', { className: 'words-filters', id: 'words-filters' });
        var search = el('label', { className: 'words-field words-field-search', for: 'words-filter-q' });
        search.appendChild(bi('חיפוש', 'Search'));
        var input = el('input', {
            id: 'words-filter-q',
            className: 'words-control',
            type: 'search',
            name: 'q'
        });
        input.setAttribute('data-filter', 'q');
        input.setAttribute('dir', 'auto');
        search.appendChild(input);
        filters.appendChild(search);
        filters.appendChild(selectFilter('status', 'שלב', 'Stage', [
            ['new', 'חדש', 'new'],
            ['learning', 'בלמידה', 'learning'],
            ['longTerm', 'לטווח ארוך', 'long-term']
        ]));
        filters.appendChild(selectFilter('weak', 'חלש', 'Weak', [
            ['1', 'חלש או חלש מאוד', 'weak or very weak'],
            ['weak', 'חלש', 'weak'],
            ['very_weak', 'חלש מאוד', 'very weak']
        ]));
        filters.appendChild(selectFilter('pos', 'חלק דיבר', 'Part of speech', POS));
        filters.appendChild(selectFilter('binyan', 'בניין', 'Binyan', BINYAN));
        filters.appendChild(selectFilter('tense', 'זמן', 'Tense', TENSE));
        filters.appendChild(selectFilter('source', 'מקור', 'Source', [
            ['lesson', 'שיעור', 'lesson'],
            ['self', 'הוספתי', 'added by me'],
            ['ai', 'בינה', 'AI']
        ]));
        filters.appendChild(selectFilter('suspended', 'מושהה', 'Suspended', [
            ['0', 'פעיל', 'not suspended'],
            ['1', 'מושהה', 'suspended']
        ]));
        var apply = el('button', { type: 'submit', className: 'btn btn-primary words-apply' });
        apply.appendChild(bi('סנן', 'Filter'));
        filters.appendChild(apply);
        filters.addEventListener('change', function (ev) {
            var target = ev.target;
            if (!target || !target.getAttribute || !target.getAttribute('data-filter')) return;
            applyFilters(readForm(), true);
        });
        filters.addEventListener('submit', function (ev) {
            ev.preventDefault();
            applyFilters(readForm(), true);
        });

        var toolbar = el('div', { className: 'words-toolbar' });
        var review = el('a', { className: 'btn btn-ghost', href: '/review/' });
        review.setAttribute('data-words-review', 'true');
        review.appendChild(bi('חזרו על אלה', 'Review these'));
        toolbar.appendChild(review);
        toolbar.appendChild(el('p', { className: 'words-status', 'data-words-status': 'loading' }));

        var layout = el('div', { className: 'words-layout' });
        layout.appendChild(el('div', { className: 'words-list', 'data-words-list': 'true' }));
        layout.appendChild(el('aside', { className: 'words-detail card', 'data-words-detail': 'true' }));

        var add = el('form', { className: 'card words-add', id: 'words-add' });
        add.setAttribute('data-words-add', 'true');
        var addTitle = el('h2');
        addTitle.appendChild(bi('הוסיפו מילה', 'Add a word'));
        add.appendChild(addTitle);
        add.appendChild(addField('key', 'עברית', 'Hebrew', 'text', true));
        add.appendChild(addField('gloss', 'אנגלית', 'English', 'text', true));
        add.appendChild(addField('translit', 'תעתיק', 'Transliteration', 'text', false));
        var addBtn = el('button', { type: 'submit', className: 'btn btn-primary' });
        addBtn.appendChild(bi('הוספה', 'Add'));
        add.appendChild(addBtn);
        add.appendChild(el('p', { className: 'words-add-error', 'data-add-error': 'true' }));
        add.addEventListener('submit', onAdd);

        rootEl.appendChild(filters);
        rootEl.appendChild(toolbar);
        rootEl.appendChild(layout);
        rootEl.appendChild(add);
        syncForm();
        updateReviewLink();
        fillStatus('loading', 'טוען…', 'Loading…');
    }

    function addField(name, heText, enText, type, required) {
        var id = 'words-add-' + name;
        var wrap = el('label', { className: 'words-field', for: id });
        wrap.appendChild(bi(heText, enText));
        var input = el('input', { id: id, className: 'words-control', type: type, name: name });
        input.setAttribute('data-add', name);
        if (name === 'key') {
            input.setAttribute('lang', 'he');
            input.setAttribute('dir', 'rtl');
            input.setAttribute('autocomplete', 'off');
        } else {
            input.setAttribute('lang', 'en');
            input.setAttribute('dir', 'ltr');
        }
        if (required) input.setAttribute('required', 'required');
        wrap.appendChild(input);
        return wrap;
    }

    function applyFilters(filters, push) {
        state.filters = filters;
        state.selected = null;
        state.detail = null;
        syncForm();
        updateReviewLink();
        if (push && root.history && typeof root.history.pushState === 'function') {
            var next = urlFor(filters);
            if (next !== currentUrl()) {
                try { root.history.pushState({ words: 1 }, '', next); } catch (err) { /* ignore */ }
            }
        }
        return load(false);
    }

    function onPop() {
        if (!alive) return;
        var path = (root.location && root.location.pathname) || '';
        if (path !== '/words' && path.indexOf('/words/') !== 0) return;
        applyFilters(filtersFromUrl(), false);
    }

    function load(append) {
        if (!alive) return Promise.resolve();
        var my = ++requestSeq;
        if (listCtrl) listCtrl.abort();
        var linked = linkedSignal(pageSignal);
        listCtrl = linked.ctrl;
        if (!append) {
            setPageState('loading');
            fillStatus('loading', 'טוען…', 'Loading…');
        }
        var cursor = append ? state.nextCursor : null;
        return apiRequest('GET', libraryQuery(state.filters, cursor), undefined, linked.signal).then(function (res) {
            if (!alive || my !== requestSeq) return;
            if (!res || !res.ok || !res.data) {
                setPageState('error');
                var pair = errorText(res);
                fillStatus('error', pair[0], pair[1]);
                return;
            }
            var incoming = Array.isArray(res.data.words) ? res.data.words : [];
            state.words = append ? state.words.concat(incoming) : incoming;
            state.nextCursor = res.data.nextCursor || null;
            renderList();
        }).catch(function (err) {
            if (!alive || my !== requestSeq) return;
            if (err && err.name === 'AbortError') return;
            setPageState('error');
            fillStatus('error', 'לא הצלחנו לטעון.', 'Could not load your words.');
        });
    }

    function renderList() {
        var list = rootEl && rootEl.querySelector('[data-words-list]');
        if (!list) return;
        clear(list);
        var filtered = FILTER_KEYS.some(function (key) { return !!state.filters[key]; });
        if (!state.words.length) {
            setPageState(filtered ? 'ready' : 'empty');
            var empty = el('div', { className: 'card words-empty' });
            empty.setAttribute('data-words-empty', filtered ? 'filter' : 'library');
            var title = el('h2');
            title.appendChild(filtered
                ? bi('אין מילים שמתאימות.', 'No words match.')
                : bi('עדיין אין מילים.', 'No words yet.'));
            empty.appendChild(title);
            var note = el('p');
            note.appendChild(filtered
                ? bi('נסו סינון אחר, או הוסיפו מילה.', 'Try another filter, or add a word.')
                : bi('הוסיפו מילה למטה, או למדו שיעור.', 'Add a word below, or study a lesson.'));
            empty.appendChild(note);
            list.appendChild(empty);
            fillStatus('ready', '0', 'words');
            renderDetail();
            return;
        }
        setPageState('ready');
        fillStatus('ready', String(state.words.length), 'shown');
        state.words.forEach(function (row) { list.appendChild(wordRow(row)); });
        if (state.nextCursor) {
            var more = el('button', { type: 'button', className: 'btn btn-ghost words-more' });
            more.setAttribute('data-words-more', 'true');
            more.appendChild(bi('עוד', 'Load more'));
            more.addEventListener('click', function () { load(true); });
            list.appendChild(more);
        }
        renderDetail();
    }

    function wordRow(row) {
        var word = String(row.word || '');
        var btn = el('button', { type: 'button', className: 'words-row' });
        btn.setAttribute('data-word', word);
        if (row.binyan) btn.setAttribute('data-binyan', String(row.binyan));
        if (row.he_en && row.he_en.stage) btn.setAttribute('data-stage', String(row.he_en.stage));
        if (row.suspended) btn.setAttribute('data-suspended', 'true');
        btn.setAttribute('aria-pressed', state.selected === word ? 'true' : 'false');
        var head = el('span', { className: 'words-row-head' });
        head.appendChild(heNode(word, 'words-he words-row-word'));
        var gloss = el('span', { className: 'words-gloss', lang: 'en', dir: 'ltr', text: row.gloss_en || '' });
        gloss.setAttribute('data-gloss', 'true');
        head.appendChild(gloss);
        btn.appendChild(head);
        var meta = el('span', { className: 'words-row-meta' });
        ['he_en', 'en_he'].forEach(function (dir) {
            var view = row[dir];
            if (!view || !view.stage) return;
            var stage = STAGES[view.stage] || { he: view.stage, en: view.stage };
            var chip = el('span', { className: 'words-chip' });
            chip.setAttribute('data-direction', dir);
            chip.setAttribute('data-stage', String(view.stage));
            chip.appendChild(bi(stage.he, stage.en));
            meta.appendChild(chip);
        });
        if (row.weak === 'weak' || row.weak === 'very_weak') {
            var weak = el('span', { className: 'words-chip words-chip-weak' });
            weak.setAttribute('data-weak', String(row.weak));
            weak.appendChild(row.weak === 'very_weak'
                ? bi('חלש מאוד', 'very weak')
                : bi('חלש', 'weak'));
            meta.appendChild(weak);
        }
        append(meta, strengthBar(row.strength_bucket));
        meta.appendChild(tagChips(row));
        if (row.suspended) {
            var held = el('span', { className: 'words-chip' });
            held.appendChild(bi('מושהה', 'suspended'));
            meta.appendChild(held);
        }
        btn.appendChild(meta);
        btn.addEventListener('click', function () { openWord(word); });
        return btn;
    }

    function strengthBar(bucket) {
        var meta = BUCKETS[bucket];
        if (!meta) return null;
        var bar = el('span', { className: 'words-strength', role: 'img' });
        bar.setAttribute('data-strength', String(bucket));
        bar.setAttribute('aria-label', meta.he); // Hebrew only: one language per label (REQ-016)
        var fill = el('span');
        fill.style.width = meta.pct + '%';
        bar.appendChild(fill);
        return bar;
    }

    function tagChips(row) {
        var wrap = el('span', { className: 'words-tags' });
        var pos = labelFor(POS, row.pos);
        if (pos) {
            var chip = el('span', { className: 'words-chip' });
            chip.setAttribute('data-pos', String(row.pos));
            chip.appendChild(bi(pos.he, pos.en));
            wrap.appendChild(chip);
        }
        if (row.root) {
            var rootChip = el('span', { className: 'words-chip' });
            rootChip.setAttribute('data-root', String(row.root));
            rootChip.appendChild(heNode(row.root, 'words-he'));
            wrap.appendChild(rootChip);
        }
        var binyan = labelFor(BINYAN, row.binyan);
        if (binyan) {
            var bChip = el('span', { className: 'words-chip' });
            bChip.setAttribute('data-binyan', String(row.binyan));
            bChip.appendChild(bi(binyan.he, binyan.en));
            wrap.appendChild(bChip);
        }
        if (row.root_class) wrap.appendChild(rootClassChip(row.root_class));
        return wrap;
    }

    function rootClassChip(value) {
        var meta = ROOT_CLASS[value];
        var chip = el('span', { className: 'words-chip words-root' });
        chip.setAttribute('data-root-class', String(value));
        if (meta && (meta.colour === 'green' || meta.colour === 'yellow' || meta.colour === 'red')) {
            chip.setAttribute('data-colour', meta.colour);
        }
        if (meta) chip.appendChild(bi(meta.he, meta.en));
        else chip.textContent = String(value);
        return chip;
    }

    function findRow(word) {
        for (var i = 0; i < state.words.length; i += 1) {
            if (state.words[i] && state.words[i].word === word) return state.words[i];
        }
        return null;
    }

    function openWord(word) {
        state.selected = word;
        state.detail = { status: 'loading' };
        renderList();
        if (detailCtrl) detailCtrl.abort();
        var linked = linkedSignal(pageSignal);
        detailCtrl = linked.ctrl;
        var my = word;
        apiRequest('GET', '/api/lexicon/' + encodeURIComponent(word), undefined, linked.signal).then(function (res) {
            if (!alive || state.selected !== my) return;
            if (res && res.ok && res.data) {
                state.detail = { status: 'ready', data: res.data };
            } else if (res && res.status === 404) {
                state.detail = { status: 'missing' };
            } else {
                state.detail = { status: 'error', res: res };
            }
            renderDetail();
        }).catch(function (err) {
            if (!alive || state.selected !== my) return;
            if (err && err.name === 'AbortError') return;
            state.detail = { status: 'error' };
            renderDetail();
        });
    }

    function renderDetail() {
        var panel = rootEl && rootEl.querySelector('[data-words-detail]');
        if (!panel) return;
        clear(panel);
        var row = state.selected ? findRow(state.selected) : null;
        if (!row) {
            var hint = el('p');
            hint.appendChild(bi('בחרו מילה כדי לראות תגיות.', 'Choose a word to see its tags.'));
            panel.appendChild(hint);
            return;
        }
        var title = el('h2');
        title.appendChild(heNode(row.word, 'words-he words-detail-word'));
        panel.appendChild(title);
        if (row.gloss_en) {
            panel.appendChild(el('p', { className: 'words-gloss', lang: 'en', dir: 'ltr', text: row.gloss_en }));
        }
        if (row.note) {
            var note = el('p', { className: 'words-note' });
            note.appendChild(bi('ההערה שלכם', 'Your note'));
            note.appendChild(doc().createTextNode(' '));
            note.appendChild(el('span', { lang: 'en', dir: 'ltr', text: row.note }));
            panel.appendChild(note);
        }
        if (row.translit) {
            panel.appendChild(el('p', { className: 'words-translit', lang: 'en', dir: 'ltr', text: row.translit }));
        }
        var tags = el('div', { className: 'words-tags' });
        tags.appendChild(tagChips(row));
        panel.appendChild(tags);
        panel.appendChild(sourcesBlock(row));
        var play = el('button', { type: 'button', className: 'btn btn-ghost' });
        play.setAttribute('data-words-audio', 'true');
        play.appendChild(bi('השמעה', 'Play'));
        play.addEventListener('click', function () {
            var Speech = root.HebrewSpeech;
            if (Speech && typeof Speech.play === 'function') {
                Speech.play(row.word, audioHref(row.audio));
            }
        });
        panel.appendChild(play);

        if (!state.detail || state.detail.status === 'loading') {
            var loading = el('p');
            loading.appendChild(bi('טוען פרטים…', 'Loading details…'));
            panel.appendChild(loading);
        } else if (state.detail.status === 'error') {
            var err = el('p', { role: 'alert' });
            var pair = errorText(state.detail.res);
            err.appendChild(bi(pair[0], pair[1]));
            panel.appendChild(err);
        } else if (isVerb(row, state.detail)) {
            panel.appendChild(paradigmBlock(state.detail.data));
        }

        var suspend = el('button', { type: 'button', className: 'btn btn-ghost' });
        suspend.setAttribute('data-words-suspend', 'true');
        suspend.appendChild(row.suspended ? bi('החזירו', 'Resume') : bi('השהו', 'Suspend'));
        suspend.addEventListener('click', function () { toggleSuspended(row); });
        panel.appendChild(suspend);
    }

    function isVerb(row, detail) {
        if (row && row.pos === 'verb') return true;
        var data = detail && detail.data;
        if (!data) return false;
        if (data.entry && data.entry.pos === 'verb') return true;
        if (data.lemma && data.lemma.pos === 'verb') return true;
        return !!(data.paradigm && Object.keys(data.paradigm).length);
    }

    function paradigmBlock(data) {
        var section = el('section', { className: 'words-paradigm' });
        var heading = el('h3');
        heading.appendChild(bi('נטיות', 'Paradigm'));
        section.appendChild(heading);
        var paradigm = (data && data.paradigm) || {};
        var tenses = Object.keys(paradigm);
        if (!tenses.length) {
            var none = el('p');
            none.appendChild(bi('עדיין אין נטיות.', 'No forms yet.'));
            section.appendChild(none);
            return section;
        }
        tenses.forEach(function (tense) {
            var block = el('div', { className: 'words-tense' });
            block.setAttribute('data-tense', tense);
            var label = labelFor(TENSE, tense);
            var h = el('h4');
            h.appendChild(label ? bi(label.he, label.en) : el('span', { text: tense }));
            block.appendChild(h);
            var cells = paradigm[tense] || {};
            Object.keys(cells).forEach(function (cell) {
                var form = cells[cell] || {};
                var line = el('p', { className: 'words-cell' });
                line.setAttribute('data-cell', cell);
                line.appendChild(el('span', { className: 'words-cell-name', lang: 'en', dir: 'ltr', text: cell }));
                line.appendChild(doc().createTextNode(' '));
                line.appendChild(heNode(form.key || '', 'words-he'));
                if (form.gloss_en) {
                    line.appendChild(doc().createTextNode(' '));
                    line.appendChild(el('span', { className: 'words-gloss', lang: 'en', dir: 'ltr', text: form.gloss_en }));
                }
                if (form.verified !== true) {
                    line.appendChild(doc().createTextNode(' '));
                    var mark = el('span', { className: 'words-unchecked' });
                    mark.appendChild(bi('עדיין לא נבדק', 'not yet checked'));
                    line.appendChild(mark);
                }
                block.appendChild(line);
            });
            section.appendChild(block);
        });
        return section;
    }

    function sourcesBlock(row) {
        var section = el('div', { className: 'words-sources' });
        var label = el('p');
        label.appendChild(bi('מקורות', 'Sources'));
        section.appendChild(label);
        var sources = Array.isArray(row.sources) ? row.sources : [];
        if (!sources.length) {
            var none = el('p');
            none.appendChild(bi('אין מקור שמור.', 'No source saved.'));
            section.appendChild(none);
            return section;
        }
        var list = el('ul');
        sources.forEach(function (source) {
            var item = el('li');
            var name = String(source || '');
            if (name === 'self') item.appendChild(bi('הוספתי', 'added by me'));
            else if (name === 'ai') item.appendChild(bi('בינה', 'AI'));
            else if (LESSON_SLUG.test(name) && !PERSONAL[name]) {
                var link = el('a', { href: '/review/?deck=' + encodeURIComponent('lesson:' + name) });
                link.setAttribute('data-lesson-source', name);
                link.textContent = name;
                item.appendChild(link);
            } else {
                item.textContent = name;
            }
            list.appendChild(item);
        });
        section.appendChild(list);
        return section;
    }

    function toggleSuspended(row) {
        var next = !row.suspended;
        var word = row.word;
        var linked = linkedSignal(pageSignal);
        apiRequest('PATCH', '/api/me/library/' + encodeURIComponent(word), { suspended: next }, linked.signal)
            .then(function (res) {
                if (!alive) return;
                if (!res || !res.ok) {
                    state.detail = { status: 'error', res: res };
                    renderDetail();
                    return;
                }
                row.suspended = next;
                renderList();
            }).catch(function (err) {
                if (!alive || (err && err.name === 'AbortError')) return;
                state.detail = { status: 'error' };
                renderDetail();
            });
    }

    function showAddError(heText, enText, kind) {
        var node = rootEl && rootEl.querySelector('[data-add-error]');
        if (!node) return;
        clear(node);
        if (!heText) {
            node.removeAttribute('role');
            return;
        }
        node.setAttribute('role', kind === 'ok' ? 'status' : 'alert');
        node.appendChild(bi(heText, enText));
    }

    function onAdd(ev) {
        ev.preventDefault();
        if (!rootEl || state.adding) return;
        var keyInput = rootEl.querySelector('[data-add="key"]');
        var glossInput = rootEl.querySelector('[data-add="gloss"]');
        var translitInput = rootEl.querySelector('[data-add="translit"]');
        var key = keyInput ? String(keyInput.value || '').trim() : '';
        var gloss = glossInput ? String(glossInput.value || '').trim() : '';
        var translit = translitInput ? String(translitInput.value || '').trim() : '';
        if (!key || !/[\u05D0-\u05EA]/.test(key) || /[<>]/.test(key)) {
            showAddError('המילה בעברית לא תקינה.', 'That Hebrew word is not valid.');
            return;
        }
        if (!gloss || /[<>]/.test(gloss)) {
            showAddError('צריך גם אנגלית.', 'Add an English gloss.');
            return;
        }
        if (translit && /[<>]/.test(translit)) {
            showAddError('התעתיק לא תקין.', 'That transliteration is not valid.');
            return;
        }
        var item = { key: key, gloss_en: gloss };
        if (translit) item.translit = translit;
        state.adding = true;
        showAddError('', '');
        var linked = linkedSignal(pageSignal);
        apiRequest('POST', '/api/me/library', { words: [item] }, linked.signal).then(function (res) {
            state.adding = false;
            if (!alive) return;
            if (!res || !res.ok) {
                var pair = errorText(res);
                showAddError(pair[0], pair[1]);
                return;
            }
            if (keyInput) keyInput.value = '';
            if (glossInput) glossInput.value = '';
            if (translitInput) translitInput.value = '';
            showAddError('נוספה.', 'Added.', 'ok');
            return load(false);
        }).catch(function (err) {
            state.adding = false;
            if (!alive || (err && err.name === 'AbortError')) return;
            showAddError('לא הצלחנו לשמור.', 'Could not save that.');
        });
    }

    function mountVowels(actions) {
        if (!actions) return;
        clear(actions);
        vowelsBtn = el('button', { type: 'button', className: 'btn btn-ghost', id: 'words-vowels-btn' });
        vowelsBtn.addEventListener('click', function () {
            state.vowelsOn = !state.vowelsOn;
            applyVowels();
        });
        actions.appendChild(vowelsBtn);
        applyVowels();
    }

    function applyVowels() {
        if (rootEl) {
            var nodes = rootEl.querySelectorAll('[data-vowels]');
            for (var i = 0; i < nodes.length; i += 1) {
                nodes[i].textContent = shownHebrew(nodes[i].getAttribute('data-vowels') || '');
            }
        }
        if (!vowelsBtn) return;
        vowelsBtn.setAttribute('aria-pressed', state.vowelsOn ? 'true' : 'false');
        clear(vowelsBtn);
        vowelsBtn.appendChild(state.vowelsOn
            ? bi('הסתר ניקוד', 'Hide vowels')
            : bi('הצג ניקוד', 'Show vowels'));
    }

    /**
     * Fill the words view and load the first page of the library.
     * @param {HTMLElement} el
     * @param {{ signal?: AbortSignal, actions?: HTMLElement }} ctx
     * @returns {Promise<void>}
     */
    function mount(el, ctx) {
        rootEl = el;
        alive = true;
        state = emptyState();
        state.filters = filtersFromUrl();
        pageSignal = ctx && ctx.signal;
        if (root.addEventListener) {
            if (pageSignal) root.addEventListener('popstate', onPop, { signal: pageSignal });
            else root.addEventListener('popstate', onPop);
        }
        build();
        mountVowels(ctx && ctx.actions);
        return load(false);
    }

    /** Stop in-flight library and lexicon fetches. Listeners on ctx.signal are already aborted. */
    function unmount() {
        alive = false;
        if (listCtrl) listCtrl.abort();
        if (detailCtrl) detailCtrl.abort();
        listCtrl = null;
        detailCtrl = null;
        vowelsBtn = null;
        if (root.removeEventListener) root.removeEventListener('popstate', onPop);
        rootEl = null;
        pageSignal = null;
        state = emptyState();
    }

    root.HebrewWordsPage = {
        mount: mount,
        unmount: unmount
    };
})(typeof window !== 'undefined' ? window : globalThis);
