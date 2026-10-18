/**
 * Conjugation tables in study and drill mode.
 * Form signals never create cards. Never drill an unverified form.
 * There is no timer. A typed answer stays in the page and is not put on the signal.
 * Provenance: INIT-006/SPEC-014; future tense: INIT-012/SPEC-006
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockConjugation = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var AUDIO_FILE = /^(?:[A-Za-z0-9_-]+\/)*[A-Za-z0-9_.-]+\.mp3$/;
    var LETTERS = Array.from('אבגדהוזחטיכלמנסעפצקרשת');
    var PRESENT = [
        { cell: 'ms', en: 'SM', he: 'זכר יחיד' },
        { cell: 'fs', en: 'SF', he: 'נקבה יחידה' },
        { cell: 'mp', en: 'PM', he: 'זכר רבים' },
        { cell: 'fp', en: 'PF', he: 'נקבה רבות' }
    ];
    var PAST = [
        { cell: '1s', he: 'אני', en: 'I' },
        { cell: '2ms', he: 'אתה', en: 'you (m.)' },
        { cell: '2fs', he: 'את', en: 'you (f.)' },
        { cell: '3ms', he: 'הוא', en: 'he' },
        { cell: '3fs', he: 'היא', en: 'she' },
        { cell: '1p', he: 'אנחנו', en: 'we' },
        { cell: '2mp', he: 'אתם', en: 'you (m. pl.)' },
        { cell: '2fp', he: 'אתן', en: 'you (f. pl.)' },
        { cell: '3p', he: 'הם/הן', en: 'they' }
    ];
    var sessions = typeof WeakMap === 'function' ? new WeakMap() : null;

    function escapeHtml(value) {
        if (root.HebrewUtils && typeof root.HebrewUtils.escapeHtml === 'function') {
            return root.HebrewUtils.escapeHtml(value);
        }
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function text(value) {
        var raw = value == null ? '' : String(value);
        if (raw.length > TEXT_LIMIT) raw = raw.slice(0, TEXT_LIMIT);
        return escapeHtml(raw);
    }

    function highlight(he, pattern) {
        var explainer = root.HebrewBlockExplainer;
        if (explainer && typeof explainer.highlightRoot === 'function') {
            return explainer.highlightRoot(he, pattern);
        }
        return text(he);
    }

    function stripVowels(value) {
        var utils = root.HebrewUtils;
        var raw = String(value == null ? '' : value);
        var stripped = utils && typeof utils.stripVowels === 'function'
            ? utils.stripVowels(raw)
            : raw.replace(/[\u0591-\u05C7]/g, '');
        return stripped.normalize('NFC').replace(/\s+/g, ' ').trim();
    }

    /**
     * A lexicon key may carry a cell tag to stay unique, e.g. "תַּחְלִיט-3fs" (future she is spelled
     * like future you m.). The learner sees the form without the tag.
     */
    function displayKey(key) {
        return String(key == null ? '' : key).replace(/-[0-9a-z]+$/i, '');
    }

    function isVerified(form) {
        return !!(form && (form.verified === true || form.verified === 1));
    }

    function known(tense, code) {
        var list = tense === 'present' ? PRESENT : (tense === 'past' || tense === 'future') ? PAST : [];
        for (var i = 0; i < list.length; i += 1) {
            if (list[i].cell === code) return list[i];
        }
        return null;
    }

    function labelOf(row) {
        if (!row) return '';
        if (row.en && row.he && row.en.length <= 3) return row.en;
        return row.he || row.en || '';
    }

    /** Bilingual tense cue for interleaved multi-verb / multi-tense drills (INIT-013/SPEC-005). */
    function tenseHeaderLabel(tense) {
        if (tense === 'past') return 'עבר / past';
        if (tense === 'future') return 'עתיד / future';
        if (tense === 'present') return 'הווה / present';
        return tense ? String(tense) : '';
    }

    function audioButton(audio, key) {
        var path = audio && key != null ? audio[key] : '';
        if (typeof path !== 'string' || !AUDIO_FILE.test(path)) return '';
        return '<button type="button" class="audio" data-plan-audio="ready">'
            + 'השמעה <span lang="en" dir="ltr">Play</span></button>';
    }

    function lines(items, className) {
        var list = Array.isArray(items) ? items.slice(0, className === 'conj-question' ? 4 : 3) : [];
        return list.map(function (line) {
            line = line || {};
            var english = line.en
                ? '<p class="gloss"><span class="meaning" lang="en" dir="ltr">' + text(line.en) + '</span></p>'
                : '';
            return '<div class="' + className + '" data-line-id="' + text(line.id) + '">'
                + '<p class="pointed-line" lang="he">' + text(line.he) + '</p>'
                + english
                + '</div>';
        }).join('');
    }

    function cellMap(block) {
        var map = Object.create(null);
        var cells = Array.isArray(block && block.cells) ? block.cells : [];
        cells.slice(0, 9).forEach(function (entry) {
            if (!entry || typeof entry !== 'object') return;
            if (typeof entry.cell !== 'string' || map[entry.cell]) return;
            map[entry.cell] = entry;
        });
        return map;
    }

    function studyCell(entry, forms, audio) {
        if (!entry) return '<td></td>';
        var formKey = entry.formKey == null ? '' : String(entry.formKey);
        var form = forms && forms[formKey];
        var verified = isVerified(form);
        var root = form && typeof form.root === 'string' ? form.root : '';
        var mark = verified
            ? ''
            : '<span class="not-checked" lang="en" dir="ltr">not yet checked</span>';
        return '<td data-cell="' + text(entry.cell) + '" data-verified="' + (verified ? '1' : '0') + '">'
            + '<span class="pointed" lang="he">' + highlight(displayKey(formKey), root) + '</span>'
            + mark
            + audioButton(audio, formKey)
            + '</td>';
    }

    function header(row, present) {
        if (present) {
            return '<th scope="col"><span lang="en" dir="ltr">' + row.en + '</span> '
                + '<span lang="he">' + row.he + '</span></th>';
        }
        return '<th scope="row"><span lang="he">' + row.he + '</span> '
            + '<span class="meaning" lang="en" dir="ltr">' + text(row.en) + '</span></th>';
    }

    function studyTable(block, forms, audio) {
        var present = block.tense === 'present';
        var rows = present ? PRESENT : block.tense === 'past' ? PAST : block.tense === 'future' ? PAST : [];
        var byCell = cellMap(block);
        if (!rows.length) return '';
        if (present) {
            var heads = rows.map(function (row) { return header(row, true); }).join('');
            var body = rows.map(function (row) {
                return studyCell(byCell[row.cell], forms, audio);
            }).join('');
            return '<table class="conj-table" data-tense="present"><thead><tr>' + heads
                + '</tr></thead><tbody><tr>' + body + '</tr></tbody></table>';
        }
        var bodyRows = rows.map(function (row) {
            return '<tr>' + header(row, false) + studyCell(byCell[row.cell], forms, audio) + '</tr>';
        }).join('');
        var tableTense = block.tense === 'future' ? 'future' : 'past';
        return '<table class="conj-table" data-tense="' + tableTense + '"><tbody>' + bodyRows + '</tbody></table>';
    }

    function extraCells(block) {
        var cells = Array.isArray(block && block.cells) ? block.cells : [];
        return cells.filter(function (entry) {
            return entry && !known(block.tense, entry.cell);
        }).map(function (entry) {
            return '<li><span lang="he">' + text(entry.cell) + '</span> '
                + '<span class="pointed" lang="he">' + text(displayKey(entry.formKey)) + '</span> '
                + '<span class="not-checked" lang="en" dir="ltr">not yet checked</span></li>';
        }).join('');
    }

    function letterPad() {
        var keys = LETTERS.map(function (letter) {
            return '<button type="button" class="letter-key" data-letter="' + letter + '">' + letter + '</button>';
        }).join('');
        return '<div class="letter-pad" data-letter-pad>' + keys
            + '<button type="button" class="letter-key" data-letter="" data-backspace="1">'
            + 'מחק <span lang="en" dir="ltr">Delete</span></button></div>';
    }

    function drillStation() {
        return '<div data-drill-station>'
            + '<p class="kicker" data-drill-prompt lang="he"></p>'
            + '<p class="attempt" data-drill-feedback hidden></p>'
            + '<p class="pointed" data-drill-reveal lang="he" hidden></p>'
            + '<label class="field">התשובה שלכם <span lang="en" dir="ltr">Your answer</span>'
            + '<input data-drill-input type="text" autocomplete="off" autocorrect="off" spellcheck="false" lang="he" dir="rtl">'
            + '</label>'
            + letterPad()
            + '<p class="dock"><button type="button" class="primary" data-drill-check>'
            + 'בדיקה <span lang="en" dir="ltr">Check</span></button> '
            + '<button type="button" data-drill-override>צדקתי <span lang="en" dir="ltr">I was right</span></button> '
            + '<button type="button" data-drill-next hidden>הבא <span lang="en" dir="ltr">Next</span></button></p>'
            + '</div>';
    }

    function drillBlocks(block, options) {
        var lesson = options && options.lesson;
        var blocks = lesson && Array.isArray(lesson.blocks) ? lesson.blocks : null;
        if (!blocks) return block && block.mode === 'drill' ? [block] : [];
        return blocks.filter(function (item) {
            return item && item.type === 'conjugation-table' && item.mode === 'drill';
        });
    }

    function isOwner(block, options) {
        if (!block || block.mode !== 'drill') return false;
        var list = drillBlocks(block, options);
        if (!list.length) return true;
        var first = list[0];
        return first === block || (first && first.id != null && first.id === block.id);
    }

    function lessonSlug(options) {
        var lesson = options && options.lesson;
        var id = lesson && typeof lesson.id === 'string' ? lesson.id : '';
        return /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id) ? id : '';
    }

    /**
     * Verified cells only, shuffled inside each table, then interleaved so the
     * same lemma is not repeated while another table still has a cell.
     * @param {object[]} blocks
     * @param {object} forms
     * @param {function(): number} [rng]
     * @returns {object[]}
     */
    function drillQueue(blocks, forms, rng) {
        var groups = [];
        (Array.isArray(blocks) ? blocks : []).forEach(function (block) {
            if (!block || block.mode !== 'drill') return;
            var items = [];
            var cells = Array.isArray(block.cells) ? block.cells.slice(0, 9) : [];
            cells.forEach(function (entry) {
                if (!entry || typeof entry !== 'object') return;
                var code = typeof entry.cell === 'string' ? entry.cell : '';
                var row = known(block.tense, code);
                if (!row) return;
                var formKey = typeof entry.formKey === 'string' ? entry.formKey : '';
                var form = forms && forms[formKey];
                if (!isVerified(form)) return;
                if (form.key && form.key !== formKey) return;
                var lemmaKey = typeof block.lemmaKey === 'string' ? block.lemmaKey : '';
                var lemma = (entry && typeof entry.lemma === 'string' && entry.lemma)
                    ? entry.lemma
                    : lemmaKey;
                items.push({
                    blockId: block.id == null ? '' : String(block.id),
                    lemmaKey: lemmaKey,
                    lemma: lemma,
                    tense: block.tense,
                    cell: code,
                    formKey: formKey,
                    accept: Array.isArray(form.accept) ? form.accept.slice(0, 8) : [],
                    label: labelOf(row),
                    root: typeof form.root === 'string' ? form.root : ''
                });
            });
            if (items.length) groups.push(shuffle(items, rng));
        });
        return interleave(groups);
    }

    function shuffle(list, rng) {
        var next = list.slice();
        var random = typeof rng === 'function' ? rng : function () { return Math.random(); };
        for (var i = next.length - 1; i > 0; i -= 1) {
            var j = Math.floor(random() * (i + 1));
            if (j < 0 || j > i) j = 0;
            var tmp = next[i];
            next[i] = next[j];
            next[j] = tmp;
        }
        return next;
    }

    function interleave(groups) {
        var queues = groups.map(function (group) { return group.slice(); });
        var out = [];
        var last = null;
        var guard = 0;
        while (guard < 80) {
            guard += 1;
            var pending = false;
            var picked = -1;
            for (var i = 0; i < queues.length; i += 1) {
                if (!queues[i].length) continue;
                pending = true;
                if (queues[i][0].lemma !== last) {
                    picked = i;
                    break;
                }
            }
            if (!pending) break;
            if (picked < 0) {
                for (var j = 0; j < queues.length; j += 1) {
                    if (queues[j].length) {
                        picked = j;
                        break;
                    }
                }
            }
            var item = queues[picked].shift();
            out.push(item);
            last = item.lemma;
        }
        return out;
    }

    /**
     * Compare after stripping U+0591–U+05C7. The first miss asks for another try.
     * The second miss reveals. An empty field is not an attempt.
     * @param {*} typed
     * @param {{ key?: string, formKey?: string, accept?: string[] }} form
     * @param {number} priorMisses
     * @returns {{ attempted: boolean, correct: boolean, reveal: boolean, result: string|null, activity: string|null }}
     */
    function decideAttempt(typed, form, priorMisses) {
        var raw = String(typed == null ? '' : typed);
        if (!raw.trim()) {
            return { attempted: false, correct: false, reveal: false, result: null, activity: null };
        }
        var key = form && (form.formKey || form.key) ? String(form.formKey || form.key) : '';
        var accept = form && Array.isArray(form.accept) ? form.accept : [];
        var got = stripVowels(raw);
        var correct = false;
        if (got && got === stripVowels(key)) correct = true;
        if (!correct) {
            for (var i = 0; i < accept.length && i < 8; i += 1) {
                if (typeof accept[i] === 'string' && got === stripVowels(accept[i])) correct = true;
            }
        }
        if (correct) {
            return {
                attempted: true,
                correct: true,
                reveal: false,
                result: 'pass',
                activity: 'conjugation-drill'
            };
        }
        var reveal = (Number(priorMisses) || 0) >= 1;
        return {
            attempted: true,
            correct: false,
            reveal: reveal,
            result: 'miss',
            activity: 'conjugation-drill'
        };
    }

    /**
     * One form signal. The typed string is not a field.
     * @param {{ formKey?: string, lemma?: string }} item
     * @param {string} result
     * @param {string} activity
     * @param {string} [slug]
     * @returns {object|null}
     */
    function formSignal(item, result, activity, slug) {
        if (!item || (result !== 'pass' && result !== 'miss')) return null;
        var signal = {
            word: String(item.formKey || ''),
            lemma: String(item.lemma || ''),
            kind: 'form',
            result: result,
            direction: 'en_he',
            activity: activity === 'conjugation-override' ? 'conjugation-override' : 'conjugation-drill'
        };
        if (slug) signal.slug = String(slug);
        return signal;
    }

    /**
     * @param {object} block
     * @param {{ lesson?: object, forms?: object, audio?: object }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'conjugation-table') return '';
        var forms = options && options.forms;
        var audio = options && options.audio;
        var mode = block.mode === 'drill' ? 'drill' : block.mode === 'study' ? 'study' : '';
        var owner = mode === 'drill' && isOwner(block, options);
        var slug = lessonSlug(options);
        var slugAttr = slug ? ' data-lesson-slug="' + slug + '"' : '';
        var body = mode === 'study' ? studyTable(block, forms, audio) : '';
        if (owner) body += drillStation();
        var extras = extraCells(block);
        var extraList = extras ? '<ul class="recycle">' + extras + '</ul>' : '';
        var empty = owner ? '<p class="gloss" data-drill-empty hidden>'
            + 'אין צורות שנבדקו. <span lang="en" dir="ltr">No checked forms.</span></p>' : '';
        var sectionTense = block.tense === 'past' ? 'past' : block.tense === 'future' ? 'future' : 'present';
        var tenseKicker = block.tense === 'future'
            ? '<p class="kicker"><span lang="he">עתיד</span> <span lang="en" dir="ltr">future</span></p>'
            : '';
        return '<section class="plan-block-conjugation-table" data-block-type="conjugation-table"'
            + ' data-block-id="' + text(block.id) + '"'
            + ' data-mode="' + (mode || 'study') + '" data-tense="' + sectionTense + '"'
            + ' data-drill-owner="' + (owner ? '1' : '0') + '"' + slugAttr + '>'
            + '<div class="conj-header">'
            + '<h2 lang="he">' + text(block.lemmaKey) + '</h2>'
            + tenseKicker
            + '<p class="kicker"><span lang="en" dir="ltr">' + (mode || 'study') + '</span></p>'
            + '</div>'
            + body
            + empty
            + (extraList ? '<div class="conj-extra-cells">' + extraList + '</div>' : '')
            + (Array.isArray(block.sentences) && block.sentences.length
                ? '<div class="conj-sentences">' + lines(block.sentences, 'conj-sentence') + '</div>'
                : '')
            + (Array.isArray(block.questions) && block.questions.length
                ? '<div class="conj-questions">' + lines(block.questions, 'conj-question') + '</div>'
                : '')
            + '</section>';
    }

    function postSignal(signal) {
        if (!signal) return;
        var Sync = root.HebrewSync;
        if (Sync && typeof Sync.queueSignal === 'function' && typeof Sync.isActive === 'function' && Sync.isActive()) {
            Sync.queueSignal(signal);
        }
    }

    function readState(section) {
        if (!sessions) return null;
        return sessions.get(section) || null;
    }

    function showItem(section, state) {
        var prompt = section.querySelector('[data-drill-prompt]');
        var feedback = section.querySelector('[data-drill-feedback]');
        var reveal = section.querySelector('[data-drill-reveal]');
        var input = section.querySelector('[data-drill-input]');
        var next = section.querySelector('[data-drill-next]');
        var empty = section.querySelector('[data-drill-empty]');
        var item = state.queue[state.index];
        state.tries = 0;
        state.settled = false;
        if (feedback) {
            feedback.hidden = true;
            feedback.textContent = '';
        }
        if (reveal) {
            reveal.hidden = true;
            reveal.textContent = '';
        }
        if (next) next.hidden = true;
        if (!item) {
            if (prompt) {
                prompt.textContent = state.queue.length ? 'סיימתם. / Done.' : '';
                prompt.removeAttribute('data-drill-cell');
                prompt.removeAttribute('data-drill-lemma');
                prompt.removeAttribute('data-drill-tense');
            }
            if (input) input.disabled = true;
            if (empty && !state.queue.length) empty.hidden = false;
            return;
        }
        if (empty) empty.hidden = true;
        if (prompt) {
            var tenseCue = tenseHeaderLabel(item.tense);
            var lemmaCue = item.lemma || item.lemmaKey || '';
            prompt.setAttribute('data-drill-cell', item.cell);
            prompt.setAttribute('data-drill-lemma', lemmaCue);
            prompt.setAttribute('data-drill-tense', item.tense || '');
            prompt.textContent = (lemmaCue ? lemmaCue + ' · ' : '')
                + (tenseCue ? tenseCue + ' · ' : '')
                + item.label;
        }
        if (input) {
            input.disabled = false;
            input.value = '';
        }
    }

    function settle(section, state, message, showForm) {
        var item = state.queue[state.index];
        var feedback = section.querySelector('[data-drill-feedback]');
        var reveal = section.querySelector('[data-drill-reveal]');
        var input = section.querySelector('[data-drill-input]');
        var next = section.querySelector('[data-drill-next]');
        state.settled = true;
        if (feedback) {
            feedback.hidden = false;
            feedback.textContent = message;
            feedback.setAttribute('data-result', message.indexOf('Correct') >= 0 || message.indexOf('I was right') >= 0
                ? 'pass'
                : 'miss');
        }
        if (reveal && showForm && item) {
            reveal.hidden = false;
            reveal.textContent = displayKey(item.formKey);
        }
        if (input) input.disabled = true;
        if (next) next.hidden = false;
    }

    function onCheck(section, state) {
        var item = state.queue[state.index];
        if (!item || state.settled) return;
        var input = section.querySelector('[data-drill-input]');
        var decision = decideAttempt(input ? input.value : '', item, state.tries);
        if (!decision.attempted) return;
        var kept = input ? input.value : '';
        postSignal(formSignal(item, decision.result, decision.activity, section.getAttribute('data-lesson-slug') || ''));
        if (decision.correct) {
            if (input) input.value = kept;
            settle(section, state, 'נכון. / Correct.', true);
            return;
        }
        if (decision.reveal) {
            settle(section, state, 'הצורה. / The form.', true);
            return;
        }
        state.tries += 1;
        var feedback = section.querySelector('[data-drill-feedback]');
        if (feedback) {
            feedback.hidden = false;
            feedback.textContent = 'נסו שוב. / try again';
            feedback.setAttribute('data-result', 'miss');
        }
    }

    function onOverride(section, state) {
        var item = state.queue[state.index];
        if (!item || state.settled) return;
        postSignal(formSignal(
            item,
            'pass',
            'conjugation-override',
            section.getAttribute('data-lesson-slug') || ''
        ));
        settle(section, state, 'נכון. / I was right.', true);
    }

    function onPad(section, button) {
        var input = section.querySelector('[data-drill-input]');
        if (!input || input.disabled) return;
        if (button.getAttribute('data-backspace') === '1') {
            input.value = Array.from(input.value).slice(0, -1).join('');
            return;
        }
        var letter = button.getAttribute('data-letter') || '';
        if (/^[\u05D0-\u05EA]$/.test(letter)) input.value += letter;
    }

    /**
     * @param {Document} doc
     * @param {{ plan?: object, forms?: object, rng?: function(): number }} [live]
     * @returns {void}
     */
    function bind(doc, live) {
        if (!doc || typeof doc.querySelectorAll !== 'function' || !sessions) return;
        var owners = doc.querySelectorAll('[data-drill-owner="1"]');
        for (var i = 0; i < owners.length; i += 1) {
            var section = owners[i];
            if (!section || section.getAttribute('data-plan-bound') === '1') continue;
            if (typeof section.addEventListener !== 'function') continue;
            section.setAttribute('data-plan-bound', '1');
            var blockId = section.getAttribute('data-block-id') || '';
            var blocks = drillBlocksFromPlan(live && live.plan, blockId);
            var state = {
                queue: drillQueue(blocks, live && live.forms, live && live.rng),
                index: 0,
                tries: 0,
                settled: false
            };
            sessions.set(section, state);
            showItem(section, state);
            section.addEventListener('click', function (event) {
                var target = event.target;
                if (!target || typeof target.closest !== 'function') return;
                var host = target.closest('[data-drill-owner="1"]');
                if (!host) return;
                var current = readState(host);
                if (!current) return;
                if (target.closest('[data-drill-check]')) {
                    onCheck(host, current);
                    return;
                }
                if (target.closest('[data-drill-override]')) {
                    onOverride(host, current);
                    return;
                }
                if (target.closest('[data-drill-next]')) {
                    if (!current.settled) return;
                    current.index += 1;
                    showItem(host, current);
                    return;
                }
                var key = target.closest('[data-letter]');
                if (key) onPad(host, key);
            });
        }
    }

    function drillBlocksFromPlan(plan, blockId) {
        var units = plan && Array.isArray(plan.units) ? plan.units : [];
        for (var u = 0; u < units.length; u += 1) {
            var lessons = units[u] && Array.isArray(units[u].lessons) ? units[u].lessons : [];
            for (var i = 0; i < lessons.length; i += 1) {
                var blocks = lessons[i] && Array.isArray(lessons[i].blocks) ? lessons[i].blocks : [];
                for (var b = 0; b < blocks.length; b += 1) {
                    if (blocks[b] && String(blocks[b].id) === String(blockId)) {
                        return blocks.filter(function (item) {
                            return item && item.type === 'conjugation-table' && item.mode === 'drill';
                        });
                    }
                }
            }
        }
        return [];
    }

    return {
        PAST: PAST,
        PRESENT: PRESENT,
        bind: bind,
        decideAttempt: decideAttempt,
        drillQueue: drillQueue,
        escapeHtml: escapeHtml,
        formSignal: formSignal,
        render: render
    };
});
