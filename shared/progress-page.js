/**
 * Progress page — memory stages and direction accuracy.
 * Provenance: INIT-002/SPEC-015
 * Renders GET /api/me/metrics; does not recompute metrics.
 * The "What your AI sees" section (INIT-006/SPEC-017) renders GET /api/me/profile.
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
            // One Hebrew sentence; only the two destinations carry an English gloss (REQ-016). The old
            // word-by-word glosses "(Start with)" and "(or)" are gone.
            + '<p>'
            + esc('התחילו ב') + '<a href="/">' + bi('היום', 'Today') + '</a> ' + esc('או ב')
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
            + esc('שלבי זיכרון') + '">'
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

    /*
     * What your AI sees.
     * This panel must render the API object, not a recomputed copy, so it never
     * drifts from what the AI gets.
     * Provenance: INIT-006/SPEC-017
     *
     * The disclosure sentence lives on HebrewShell so this file does not repeat it.
     * The Hebrew line there is a draft until the operator approves it (ADR-0009).
     */
    function aiDisclosure() {
        var Shell = root.HebrewShell;
        return Shell && typeof Shell.AI_DISCLOSURE === 'string' ? Shell.AI_DISCLOSURE : '';
    }

    var FAMILY_ORDER = ['binyan', 'tense', 'cell', 'root-class'];
    var FAMILY_LABELS = {
        binyan: ['בניינים', 'Binyanim'],
        tense: ['זמנים', 'Tenses'],
        cell: ['גופים', 'Person cells'],
        'root-class': ['גזרות', 'Root classes']
    };
    var STATUS_LABELS = {
        unseen: ['לא נראה', 'unseen'],
        emerging: ['מתחיל', 'emerging'],
        very_weak: ['חלש מאוד', 'very weak'],
        weak: ['חלש', 'weak'],
        ok: ['בסדר', 'ok'],
        strong: ['חזק', 'strong']
    };
    var JOURNEY_LABELS = {
        script: ['כתב', 'script'],
        foundations: ['יסודות', 'foundations'],
        building: ['בנייה', 'building'],
        expanding: ['הרחבה', 'expanding'],
        independent: ['עצמאי', 'independent']
    };
    var CONCEPT_LABELS = {
        'binyan:paal': ['פָּעַל', 'paal'],
        'binyan:nifal': ['נִפְעַל', 'nifal'],
        'binyan:piel': ['פִּעֵל', 'piel'],
        'binyan:pual': ['פֻּעַל', 'pual'],
        'binyan:hifil': ['הִפְעִיל', 'hifil'],
        'binyan:hufal': ['הֻפְעַל', 'hufal'],
        'binyan:hitpael': ['הִתְפַּעֵל', 'hitpael'],
        'tense:infinitive': ['שם פועל', 'infinitive'],
        'tense:present': ['זמן הווה', 'present'],
        'tense:past': ['זמן עבר', 'past'],
        'tense:future': ['זמן עתיד', 'future'],
        'tense:imperative': ['ציווי', 'imperative'],
        'root-class:regular': ['שלם', 'regular'],
        'root-class:lamed-he': ['ל״ה', 'lamed-he'],
        'root-class:peh-yod': ['פ״י', 'peh-yod'],
        'root-class:peh-nun': ['פ״נ', 'peh-nun'],
        'root-class:ayin-vav': ['ע״ו', 'ayin-vav'],
        'root-class:irregular': ['חריג', 'irregular'],
        'cell:present.ms': ['זכר יחיד', 'masculine singular'],
        'cell:present.fs': ['נקבה יחידה', 'feminine singular'],
        'cell:present.mp': ['זכר רבים', 'masculine plural'],
        'cell:present.fp': ['נקבה רבות', 'feminine plural'],
        'cell:past.1s': ['אני', 'first person singular'],
        'cell:past.2ms': ['אתה', 'second person masculine singular'],
        'cell:past.2fs': ['את', 'second person feminine singular'],
        'cell:past.3ms': ['הוא', 'third person masculine singular'],
        'cell:past.3fs': ['היא', 'third person feminine singular'],
        'cell:past.1p': ['אנחנו', 'first person plural'],
        'cell:past.2mp': ['אתם', 'second person masculine plural'],
        'cell:past.2fp': ['אתן', 'second person feminine plural'],
        'cell:past.3p': ['הם, הן', 'third person plural'],
        'cell:infinitive.inf': ['שם פועל', 'infinitive']
    };
    var ROOT_COLOUR = {
        'root-class:regular': 'green',
        'root-class:lamed-he': 'yellow',
        'root-class:peh-yod': 'red',
        'root-class:peh-nun': 'red',
        'root-class:ayin-vav': 'red',
        'root-class:irregular': 'red'
    };
    var COLOUR_LABELS = {
        green: ['ירוק', 'green'],
        yellow: ['צהוב', 'yellow'],
        red: ['אדום', 'red']
    };

    function pageDoc() {
        return root.document;
    }

    function makeEl(tag, attrs) {
        var node = pageDoc().createElement(tag);
        if (!attrs) return node;
        Object.keys(attrs).forEach(function (key) {
            if (key === 'className') node.className = attrs[key];
            else if (key === 'text') node.textContent = attrs[key] == null ? '' : String(attrs[key]);
            else node.setAttribute(key, attrs[key]);
        });
        return node;
    }

    function biNode(he, en) {
        var frag = pageDoc().createDocumentFragment();
        if (!en) {
            frag.appendChild(pageDoc().createTextNode(String(he || '')));
            return frag;
        }
        var wrap = makeEl('span', { className: 'ui-pair' });
        wrap.appendChild(makeEl('span', { className: 'ui-he', lang: 'he', text: String(he || '') }));
        wrap.appendChild(makeEl('span', { className: 'ui-en', lang: 'en', dir: 'ltr', text: String(en) }));
        frag.appendChild(wrap);
        return frag;
    }

    function appendBi(parent, he, en) {
        parent.appendChild(biNode(he, en));
    }

    function prettyProfileJson(rawText) {
        var text = rawText == null ? '' : String(rawText);
        try {
            return JSON.stringify(JSON.parse(text), null, 2);
        } catch (err) {
            return text;
        }
    }

    function profileFromRaw(rawText) {
        try {
            var parsed = JSON.parse(String(rawText || ''));
            if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return parsed;
        } catch (err) { /* not the profile object */ }
        return null;
    }

    function familyOf(concept) {
        var key = String(concept || '');
        var cut = key.indexOf(':');
        return cut < 0 ? 'other' : key.slice(0, cut);
    }

    function addLabeledValue(parent, he, en, value) {
        var p = makeEl('p', { className: 'ai-profile-line' });
        appendBi(p, he, en);
        p.appendChild(pageDoc().createTextNode(': '));
        var val = makeEl('span', { className: 'ai-profile-value', text: value == null || value === '' ? '—' : String(value) });
        p.appendChild(val);
        parent.appendChild(p);
    }

    function addStringList(parent, he, en, items) {
        var wrap = makeEl('div', { className: 'ai-profile-block' });
        var h = makeEl('h3');
        appendBi(h, he, en);
        wrap.appendChild(h);
        var list = Array.isArray(items) ? items : [];
        if (!list.length) {
            var empty = makeEl('p', { className: 'ai-profile-empty' });
            appendBi(empty, 'אין', 'none');
            wrap.appendChild(empty);
        } else {
            var ul = makeEl('ul', { className: 'ai-profile-list' });
            list.forEach(function (item) {
                ul.appendChild(makeEl('li', { text: item == null ? '' : String(item) }));
            });
            wrap.appendChild(ul);
        }
        parent.appendChild(wrap);
    }

    function wordLinks(parent, ids, flag) {
        var list = Array.isArray(ids) ? ids : [];
        if (!list.length) {
            var empty = makeEl('p', { className: 'ai-profile-empty' });
            appendBi(empty, 'אין', 'none');
            parent.appendChild(empty);
            return;
        }
        var row = makeEl('ul', { className: 'ai-profile-words' });
        list.forEach(function (id) {
            var li = makeEl('li');
            var href = '/words/?weak=' + encodeURIComponent(flag) + '&q=' + encodeURIComponent(String(id));
            var link = makeEl('a', { href: href, text: String(id) });
            link.setAttribute('data-ai-word', String(id));
            link.setAttribute('data-ai-weak', flag);
            li.appendChild(link);
            row.appendChild(li);
        });
        parent.appendChild(row);
    }

    function addStrength(parent, value) {
        var span = makeEl('span');
        if (value == null) {
            span.setAttribute('data-strength', 'null');
            appendBi(span, 'אין חוזק', 'no strength');
        } else {
            span.setAttribute('data-strength', String(value));
            span.textContent = String(value);
        }
        parent.appendChild(span);
    }

    function addDirection(parent, name, dir) {
        var p = makeEl('p', { className: 'ai-profile-direction' });
        p.setAttribute('data-direction', name);
        appendBi(p, name === 'he_en' ? 'עברית → אנגלית' : 'אנגלית → עברית', name === 'he_en' ? 'Hebrew → English' : 'English → Hebrew');
        p.appendChild(pageDoc().createTextNode(' '));
        if (!dir) {
            appendBi(p, 'אין', 'none');
        } else {
            var stage = makeEl('span', { text: dir.stage == null ? '—' : String(dir.stage) });
            stage.setAttribute('data-stage', dir.stage == null ? '' : String(dir.stage));
            p.appendChild(stage);
            p.appendChild(pageDoc().createTextNode(' · '));
            addStrength(p, dir.strength);
        }
        parent.appendChild(p);
    }

    function grammarGroups(grammar) {
        var groups = {};
        FAMILY_ORDER.forEach(function (family) { groups[family] = []; });
        (Array.isArray(grammar) ? grammar : []).forEach(function (row) {
            var family = familyOf(row && row.concept);
            if (!groups[family]) groups[family] = [];
            groups[family].push(row);
        });
        return groups;
    }

    function addGrammar(parent, grammar) {
        var groups = grammarGroups(grammar);
        var order = FAMILY_ORDER.slice();
        Object.keys(groups).forEach(function (family) {
            if (order.indexOf(family) === -1 && groups[family].length) order.push(family);
        });
        order.forEach(function (family) {
            var rows = groups[family] || [];
            var block = makeEl('section', { className: 'ai-profile-grammar-group' });
            block.setAttribute('data-grammar-family', family);
            var h = makeEl('h3');
            var labels = FAMILY_LABELS[family] || [family, family];
            appendBi(h, labels[0], labels[1]);
            block.appendChild(h);
            if (!rows.length) {
                var empty = makeEl('p', { className: 'ai-profile-empty' });
                appendBi(empty, 'אין', 'none');
                block.appendChild(empty);
                parent.appendChild(block);
                return;
            }
            var ul = makeEl('ul', { className: 'ai-profile-list' });
            rows.forEach(function (row) {
                var li = makeEl('li', { className: 'ai-profile-concept' });
                var key = row && row.concept != null ? String(row.concept) : '';
                li.setAttribute('data-concept', key);
                var known = CONCEPT_LABELS[key];
                var name = makeEl('span', { className: 'ai-profile-concept-name' });
                if (known) appendBi(name, known[0], known[1]);
                else name.textContent = key;
                li.appendChild(name);
                li.appendChild(pageDoc().createTextNode(' · '));
                var status = row && row.status != null ? String(row.status) : '';
                var statusEl = makeEl('span');
                statusEl.setAttribute('data-status', status);
                var statusLabel = STATUS_LABELS[status];
                if (statusLabel) appendBi(statusEl, statusLabel[0], statusLabel[1]);
                else statusEl.textContent = status || '—';
                li.appendChild(statusEl);
                li.appendChild(pageDoc().createTextNode(' · '));
                var acc = makeEl('span');
                if (row && typeof row.accuracy === 'number') {
                    acc.setAttribute('data-accuracy', String(row.accuracy));
                    acc.textContent = String(row.accuracy);
                } else {
                    acc.setAttribute('data-accuracy', '');
                    acc.textContent = '—';
                }
                li.appendChild(acc);
                var colour = ROOT_COLOUR[key];
                if (colour) {
                    li.appendChild(pageDoc().createTextNode(' '));
                    var swatch = makeEl('span', { className: 'ai-profile-swatch' });
                    swatch.setAttribute('data-colour', colour);
                    swatch.setAttribute('role', 'img');
                    var colourLabel = COLOUR_LABELS[colour] || [colour, colour];
                    swatch.setAttribute('aria-label', colourLabel[1]);
                    li.appendChild(swatch);
                    li.appendChild(pageDoc().createTextNode(' '));
                    var colourText = makeEl('span', { className: 'ai-profile-colour-name' });
                    appendBi(colourText, colourLabel[0], colourLabel[1]);
                    li.appendChild(colourText);
                }
                ul.appendChild(li);
            });
            block.appendChild(ul);
            parent.appendChild(block);
        });
    }

    function addJourney(parent, journey) {
        var trip = journey && typeof journey === 'object' ? journey : {};
        var stage = trip.stage == null ? '' : String(trip.stage);
        var stageP = makeEl('p', { className: 'ai-profile-line' });
        stageP.setAttribute('data-journey-stage', stage);
        appendBi(stageP, 'שלב', 'Stage');
        stageP.appendChild(pageDoc().createTextNode(': '));
        var stageLabel = JOURNEY_LABELS[stage];
        if (stageLabel) appendBi(stageP, stageLabel[0], stageLabel[1]);
        else stageP.appendChild(pageDoc().createTextNode(stage || '—'));
        parent.appendChild(stageP);

        var counts = trip.counts || {};
        ['he_en', 'en_he'].forEach(function (dir) {
            var bag = counts[dir] || {};
            var p = makeEl('p', { className: 'ai-profile-counts' });
            p.setAttribute('data-count-direction', dir);
            appendBi(p, dir === 'he_en' ? 'עברית → אנגלית' : 'אנגלית → עברית', dir === 'he_en' ? 'Hebrew → English' : 'English → Hebrew');
            p.appendChild(pageDoc().createTextNode(' '));
            [
                ['new', 'חדש', 'New'],
                ['learning', 'בלמידה', 'Learning'],
                ['longTerm', 'לטווח ארוך', 'Long-term']
            ].forEach(function (meta, index) {
                if (index) p.appendChild(pageDoc().createTextNode(' · '));
                var n = bag[meta[0]];
                var bit = makeEl('span');
                bit.setAttribute('data-count', meta[0]);
                bit.setAttribute('data-count-value', n == null ? '' : String(n));
                appendBi(bit, meta[1], meta[2]);
                bit.appendChild(pageDoc().createTextNode(' ' + (n == null ? '—' : String(n))));
                p.appendChild(bit);
            });
            parent.appendChild(p);
        });

        addStringList(parent, 'שיעורים שהושלמו', 'Lessons finished', trip.lessons_completed);
        addStringList(parent, 'זמנים שתורגלו', 'Tenses practiced', trip.tenses_practiced);
        addStringList(parent, 'בניינים שתורגלו', 'Binyanim practiced', trip.binyanim_practiced);
        var share = trip.vowels_off_share;
        addLabeledValue(parent, 'קריאה בלי ניקוד', 'Vowels off', share == null ? '—' : String(share));

        var recipes = trip.recipes_completed && typeof trip.recipes_completed === 'object'
            ? trip.recipes_completed
            : {};
        var recipeBlock = makeEl('div', { className: 'ai-profile-block' });
        var recipeH = makeEl('h3');
        appendBi(recipeH, 'מתכונים שהושלמו', 'Recipes finished');
        recipeBlock.appendChild(recipeH);
        var recipeKeys = Object.keys(recipes);
        if (!recipeKeys.length) {
            var none = makeEl('p', { className: 'ai-profile-empty' });
            appendBi(none, 'אין', 'none');
            recipeBlock.appendChild(none);
        } else {
            var recipeList = makeEl('ul', { className: 'ai-profile-list' });
            recipeKeys.forEach(function (key) {
                recipeList.appendChild(makeEl('li', { text: String(key) + ': ' + String(recipes[key]) }));
            });
            recipeBlock.appendChild(recipeList);
        }
        parent.appendChild(recipeBlock);
    }

    /**
     * Paint the profile section from the response body.
     * `rawText` is the HTTP body. The visible raw block is that JSON pretty-printed.
     * A hidden wire node keeps the body unchanged for the byte comparison.
     * @param {HTMLElement} host
     * @param {object|null} profile parsed response, or null on failure
     * @param {string} rawText exact response body
     * @param {string} [errorMessage]
     */
    function renderAiProfile(host, profile, rawText, errorMessage) {
        var documentRef = pageDoc();
        if (!host || typeof host.appendChild !== 'function' || !documentRef || typeof documentRef.createElement !== 'function') {
            return;
        }
        if (typeof host.querySelector === 'function') {
            var previous = host.querySelector('[data-ai-profile]');
            if (previous && typeof previous.remove === 'function') previous.remove();
        }
        var section = makeEl('section', { className: 'card progress-card ai-profile' });
        section.setAttribute('data-ai-profile', '1');
        var heading = makeEl('h2', { id: 'ai-profile-title' });
        appendBi(heading, 'מה הבינה המלאכותית רואה', 'What your AI sees');
        section.appendChild(heading);
        section.setAttribute('aria-labelledby', 'ai-profile-title');

        var disclosure = makeEl('p', { className: 'ai-profile-note', role: 'note', text: aiDisclosure() });
        disclosure.setAttribute('data-ai-disclosure', '1');
        section.appendChild(disclosure);

        if (!profile || errorMessage) {
            var err = makeEl('p', { className: 'ai-profile-error', role: 'alert' });
            appendBi(err, 'לא הצלחנו לטעון את הפרופיל.', 'Could not load the profile.');
            if (errorMessage) {
                err.appendChild(documentRef.createTextNode(' '));
                err.appendChild(makeEl('span', { className: 'en', dir: 'ltr', text: String(errorMessage) }));
            }
            section.appendChild(err);
            host.appendChild(section);
            return;
        }

        addLabeledValue(section, 'שם משתמש', 'Username', profile.username);
        addLabeledValue(section, 'גרסת פרופיל', 'Profile version', profile.profile_version);
        var planningOn = profile.ai_planning === true;
        var switchP = makeEl('p', { className: 'ai-profile-switch' });
        switchP.setAttribute('data-ai-planning', planningOn ? 'on' : 'off');
        appendBi(
            switchP,
            planningOn ? 'תכנון בינה מלאכותית: פעיל — מנהל משנה את זה' : 'תכנון בינה מלאכותית: כבוי — מנהל משנה את זה',
            planningOn ? 'AI planning: on — an admin changes this' : 'AI planning: off — an admin changes this'
        );
        section.appendChild(switchP);

        addJourney(section, profile.journey);
        addLabeledValue(section, 'סך מילים', 'Words total', profile.words_total);

        var weak = makeEl('section', { className: 'ai-profile-block' });
        var weakH = makeEl('h3');
        var weakLink = makeEl('a', { href: '/words/?weak=weak' });
        appendBi(weakLink, 'מילים חלשות', 'Weak words');
        weakH.appendChild(weakLink);
        weak.appendChild(weakH);
        wordLinks(weak, profile.weak_words, 'weak');
        section.appendChild(weak);

        var very = makeEl('section', { className: 'ai-profile-block' });
        var veryH = makeEl('h3');
        var veryLink = makeEl('a', { href: '/words/?weak=very_weak' });
        appendBi(veryLink, 'מילים חלשות מאוד', 'Very weak words');
        veryH.appendChild(veryLink);
        very.appendChild(veryH);
        wordLinks(very, profile.very_weak_words, 'very_weak');
        section.appendChild(very);

        var grammar = makeEl('section', { className: 'ai-profile-block' });
        var grammarH = makeEl('h3');
        appendBi(grammarH, 'דקדוק', 'Grammar');
        grammar.appendChild(grammarH);
        addGrammar(grammar, profile.grammar);
        section.appendChild(grammar);

        var words = Array.isArray(profile.words) ? profile.words : [];
        var wordsBlock = makeEl('section', { className: 'ai-profile-block' });
        var wordsH = makeEl('h3');
        appendBi(wordsH, 'מילים', 'Words');
        wordsBlock.appendChild(wordsH);
        if (!words.length) {
            var noWords = makeEl('p', { className: 'ai-profile-empty' });
            appendBi(noWords, 'אין', 'none');
            wordsBlock.appendChild(noWords);
        } else {
            var wordList = makeEl('ul', { className: 'ai-profile-list' });
            words.forEach(function (entry) {
                var li = makeEl('li');
                var id = entry && entry.word_id != null ? String(entry.word_id) : '';
                li.setAttribute('data-profile-word', id);
                li.appendChild(makeEl('span', { className: 'ai-profile-word-id', text: id }));
                if (entry && entry.weak) {
                    li.appendChild(documentRef.createTextNode(' · '));
                    li.appendChild(makeEl('span', { text: String(entry.weak) }));
                }
                if (entry && entry.lapses != null) {
                    li.appendChild(documentRef.createTextNode(' · '));
                    appendBi(li, 'נפילות', 'lapses');
                    li.appendChild(documentRef.createTextNode(' ' + String(entry.lapses)));
                }
                addDirection(li, 'he_en', entry && entry.he_en);
                addDirection(li, 'en_he', entry && entry.en_he);
                wordList.appendChild(li);
            });
            wordsBlock.appendChild(wordList);
        }
        section.appendChild(wordsBlock);

        var toggle = makeEl('button', { type: 'button', className: 'btn' });
        toggle.setAttribute('data-ai-profile-raw-toggle', '1');
        toggle.setAttribute('aria-expanded', 'false');
        toggle.setAttribute('aria-controls', 'ai-profile-raw');
        appendBi(toggle, 'הצגת המקור', 'Show raw');
        section.appendChild(toggle);

        var shown = makeEl('pre', { className: 'ai-profile-raw', id: 'ai-profile-raw' });
        shown.setAttribute('data-ai-profile-shown', '1');
        shown.hidden = true;
        shown.setAttribute('hidden', '');
        shown.textContent = prettyProfileJson(rawText);
        section.appendChild(shown);

        var wire = makeEl('pre', { className: 'ai-profile-wire' });
        wire.setAttribute('data-ai-profile-wire', '1');
        wire.hidden = true;
        wire.setAttribute('hidden', '');
        wire.textContent = rawText == null ? '' : String(rawText);
        section.appendChild(wire);

        toggle.addEventListener('click', function () {
            var open = shown.hasAttribute('hidden');
            if (open) {
                shown.hidden = false;
                shown.removeAttribute('hidden');
            } else {
                shown.hidden = true;
                shown.setAttribute('hidden', '');
            }
            toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
        });

        host.appendChild(section);
    }

    // ---- view lifecycle (contract: shared/router.js) ----
    // `render` is also used by the admin learner-detail panel, so keep it standalone.
    var mounted = null;
    var mountGen = 0;

    function mount(el) {
        var gen = ++mountGen;
        mounted = el;
        render(el, { state: 'loading' });
        var Api = root.HebrewApi;
        if (!Api || typeof Api.get !== 'function') {
            render(el, { state: 'error', message: 'API unavailable' });
            renderAiProfile(el, null, '', 'API unavailable');
            return Promise.resolve();
        }
        var metricsP = Api.get('/api/me/metrics').then(function (res) {
            if (!res || !res.ok || !res.data) {
                return {
                    state: 'error',
                    message: (res && res.error) || ('HTTP ' + ((res && res.status) || '?'))
                };
            }
            return res.data;
        }).catch(function (err) {
            return { state: 'error', message: (err && err.message) || 'Network error' };
        });
        var profileP = Api.get('/api/me/profile', { raw: true }).then(function (res) {
            var raw = res && typeof res.rawText === 'string' ? res.rawText : '';
            var parsed = profileFromRaw(raw);
            return {
                ok: !!(res && res.ok && parsed),
                profile: parsed,
                raw: raw,
                message: (res && res.error) || ('HTTP ' + ((res && res.status) || '?'))
            };
        }).catch(function (err) {
            return { ok: false, profile: null, raw: '', message: (err && err.message) || 'Network error' };
        });
        return Promise.all([metricsP, profileP]).then(function (pair) {
            if (gen !== mountGen || mounted !== el) return;
            render(el, pair[0]);
            var profile = pair[1];
            if (profile.ok) renderAiProfile(el, profile.profile, profile.raw, '');
            else renderAiProfile(el, null, '', profile.message);
        });
    }

    function unmount() {
        mountGen += 1;
        mounted = null;
    }

    root.HebrewProgressView = {
        render: render,
        resolveState: resolveState,
        renderAiProfile: renderAiProfile,
        prettyProfileJson: prettyProfileJson,
        aiDisclosure: aiDisclosure,
        mount: mount,
        unmount: unmount
    };
})(typeof window !== 'undefined' ? window : globalThis);
