/**
 * Plan exercise block: choose, true-or-false, unscramble, translate, dictation.
 * Answers stay in the page; nothing is sent to the server.
 * Provenance: INIT-012/SPEC-004, INIT-012/SPEC-005
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockExercise = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var DICTATION_AUDIO = /^(?:\.\.\/)?(?:[A-Za-z0-9_-]+\/)*[A-Za-z0-9_.-]+\.mp3$/;
    var WORD_CLIP = /^(?:\.\.\/)?_audio\/words\/([A-Za-z0-9_-]{8,64})\.mp3$/;
    var LESSON_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

    var compare = (typeof require === 'function'
        ? (function () { try { return require('./compare.js'); } catch (e) { return null; } })()
        : null) || root.HebrewBlockCompare || {};
    var sameHebrew = typeof compare.sameHebrew === 'function' ? compare.sameHebrew : function (a, b) {
        return String(a || '') === String(b || '');
    };
    var stepAttempt = typeof compare.stepAttempt === 'function' ? compare.stepAttempt : function (n, ok) {
        if (ok) return 'correct';
        return (Number(n) || 0) >= 2 ? 'reveal' : 'retry';
    };
    var shuffleWords = typeof compare.shuffleWords === 'function' ? compare.shuffleWords : function (w) {
        return Array.isArray(w) ? w.slice() : [];
    };
    var stripPoints = typeof compare.stripPoints === 'function' ? compare.stripPoints : function (s) {
        return String(s == null ? '' : s);
    };

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

    function attr(value) {
        return text(value);
    }

    function lineLang(value) {
        return /[\u0590-\u05FF]/.test(String(value || '')) ? 'he' : 'en';
    }

    function dirFor(value) {
        return lineLang(value) === 'en' ? ' dir="ltr"' : ' dir="rtl"';
    }

    function lessonSlugFrom(options) {
        var lesson = options && options.lesson;
        var id = lesson && typeof lesson.id === 'string' ? lesson.id : '';
        return LESSON_SLUG.test(id) ? id : '';
    }

    function stubSection(block) {
        return '<section class="plan-exercise" data-block-type="exercise" data-block-id="' + text(block.id)
            + '" data-exercise-kind="' + text(block.kind) + '" data-exercise-stub="1">'
            + '<p class="gloss" lang="en" dir="ltr">This exercise type is not ready in the player yet.</p>'
            + '</section>';
    }

    function progressLine(index, total) {
        return 'שאלה ' + index + ' מתוך ' + total + ' (' + index + ' of ' + total + ')';
    }

    function summaryLine(firstTry, total) {
        return firstTry + ' מתוך ' + total + ' נכונים בניסיון הראשון ('
            + firstTry + ' of ' + total + ' right on the first try)';
    }

    function jsonAttr(value) {
        try {
            return attr(JSON.stringify(value));
        } catch (err) {
            return attr('[]');
        }
    }

    function renderChoose(block) {
        var items = Array.isArray(block.items) ? block.items.slice(0, 12) : [];
        var total = items.length;
        var rows = items.map(function (item, idx) {
            item = item || {};
            var prompt = item.prompt == null ? '' : String(item.prompt);
            var lang = lineLang(prompt);
            var choices = Array.isArray(item.choices) ? item.choices.slice(0, 5) : [];
            var buttons = choices.map(function (choice) {
                var label = choice == null ? '' : String(choice);
                return '<button type="button" class="plan-exercise-choice" data-exercise-choice'
                    + ' data-choice-value="' + attr(label) + '" lang="' + lineLang(label) + '"'
                    + dirFor(label) + '>' + text(label) + '</button>';
            }).join('');
            var why = item.why == null ? '' : String(item.why);
            var whyLang = lineLang(why);
            var whyBlock = why
                ? '<p class="plan-exercise-why" data-exercise-why hidden lang="' + whyLang + '"'
                    + (whyLang === 'en' ? ' dir="ltr"' : ' dir="rtl"') + '>' + text(why) + '</p>'
                : '';
            return '<li class="plan-exercise-item" data-exercise-item data-item-id="' + text(item.id) + '"'
                + ' data-item-index="' + String(idx + 1) + '" data-answer="' + attr(item.answer) + '"'
                + ' data-exercise-kind="choose">'
                + '<p class="plan-exercise-progress-item" aria-hidden="true">' + text(progressLine(idx + 1, total)) + '</p>'
                + '<p class="plan-exercise-prompt pointed-line" lang="' + lang + '"' + dirFor(prompt) + '>'
                + text(prompt) + '</p>'
                + '<div class="plan-exercise-choices">' + buttons + '</div>'
                + '<p class="plan-exercise-feedback" data-exercise-feedback hidden></p>'
                + whyBlock
                + '</li>';
        }).join('');
        var heading = block.heading
            ? '<h2 class="plan-exercise-heading">' + text(block.heading) + '</h2>'
            : '';
        return '<section class="plan-exercise" data-block-type="exercise" data-block-id="' + text(block.id) + '"'
            + ' data-exercise-kind="choose">'
            + heading
            + '<p class="plan-exercise-progress" data-exercise-progress>' + text(progressLine(1, total || 1)) + '</p>'
            + '<p class="plan-exercise-summary" data-exercise-summary hidden></p>'
            + '<p class="plan-exercise-live" data-exercise-live aria-live="polite" aria-atomic="true"></p>'
            + '<ol class="plan-exercise-items">' + rows + '</ol>'
            + '</section>';
    }

    function renderTrueFalse(block) {
        var items = Array.isArray(block.items) ? block.items.slice(0, 12) : [];
        var total = items.length;
        var rows = items.map(function (item, idx) {
            item = item || {};
            var statement = item.statement == null ? '' : String(item.statement);
            var lang = lineLang(statement);
            var answer = item.answer === true || item.answer === 'true';
            var why = item.why == null ? '' : String(item.why);
            var whyLang = lineLang(why);
            var whyBlock = why
                ? '<p class="plan-exercise-why" data-exercise-why hidden lang="' + whyLang + '"'
                    + (whyLang === 'en' ? ' dir="ltr"' : ' dir="rtl"') + '>' + text(why) + '</p>'
                : '';
            return '<li class="plan-exercise-item" data-exercise-item data-item-id="' + text(item.id) + '"'
                + ' data-item-index="' + String(idx + 1) + '" data-answer="' + (answer ? 'true' : 'false') + '"'
                + ' data-exercise-kind="true-false">'
                + '<p class="plan-exercise-progress-item" aria-hidden="true">' + text(progressLine(idx + 1, total)) + '</p>'
                + '<p class="plan-exercise-prompt pointed-line" lang="' + lang + '"' + dirFor(statement) + '>'
                + text(statement) + '</p>'
                + '<div class="plan-exercise-choices">'
                + '<button type="button" class="plan-exercise-choice" data-exercise-choice data-tf-value="true" lang="he" dir="rtl">'
                + 'נכון <span lang="en" dir="ltr">True</span></button>'
                + '<button type="button" class="plan-exercise-choice" data-exercise-choice data-tf-value="false" lang="he" dir="rtl">'
                + 'לא נכון <span lang="en" dir="ltr">False</span></button>'
                + '</div>'
                + '<p class="plan-exercise-feedback" data-exercise-feedback hidden></p>'
                + whyBlock
                + '</li>';
        }).join('');
        var heading = block.heading
            ? '<h2 class="plan-exercise-heading">' + text(block.heading) + '</h2>'
            : '';
        return '<section class="plan-exercise" data-block-type="exercise" data-block-id="' + text(block.id) + '"'
            + ' data-exercise-kind="true-false">'
            + heading
            + '<p class="plan-exercise-progress" data-exercise-progress>' + text(progressLine(1, total || 1)) + '</p>'
            + '<p class="plan-exercise-summary" data-exercise-summary hidden></p>'
            + '<p class="plan-exercise-live" data-exercise-live aria-live="polite" aria-atomic="true"></p>'
            + '<ol class="plan-exercise-items">' + rows + '</ol>'
            + '</section>';
    }

    function renderUnscramble(block) {
        var items = Array.isArray(block.items) ? block.items.slice(0, 12) : [];
        var total = items.length;
        var rows = items.map(function (item, idx) {
            item = item || {};
            var words = Array.isArray(item.words) ? item.words.map(function (w) { return String(w == null ? '' : w); }) : [];
            var alts = Array.isArray(item.alt) ? item.alt.slice(0, 3) : [];
            var shuffled = shuffleWords(words, String(item.id || idx), alts);
            var english = item.english == null ? '' : String(item.english);
            var englishBlock = english
                ? '<p class="plan-exercise-prompt gloss" lang="en" dir="ltr">' + text(english) + '</p>'
                : '';
            var bank = shuffled.map(function (word, i) {
                return '<button type="button" class="plan-exercise-choice" data-unscramble-bank-idx="' + String(i) + '"'
                    + ' data-word="' + attr(word) + '" lang="he" dir="rtl">' + text(word) + '</button>';
            }).join('');
            return '<li class="plan-exercise-item" data-exercise-item data-item-id="' + text(item.id) + '"'
                + ' data-exercise-kind="unscramble"'
                + ' data-unscramble-words="' + jsonAttr(words) + '"'
                + ' data-unscramble-alt="' + jsonAttr(alts) + '">'
                + '<p class="plan-exercise-progress-item" aria-hidden="true">' + text(progressLine(idx + 1, total)) + '</p>'
                + englishBlock
                + '<div class="plan-exercise-choices" data-unscramble-bank>' + bank + '</div>'
                + '<div class="plan-exercise-choices" data-unscramble-picked aria-label="Your sentence"></div>'
                + '<button type="button" class="plan-exercise-choice" data-unscramble-check lang="en" dir="ltr">'
                + 'Check</button>'
                + '<p class="plan-exercise-reveal pointed-line" data-unscramble-reveal hidden lang="he" dir="rtl"></p>'
                + '<p class="plan-exercise-feedback" data-exercise-feedback hidden></p>'
                + '</li>';
        }).join('');
        var heading = block.heading
            ? '<h2 class="plan-exercise-heading">' + text(block.heading) + '</h2>'
            : '';
        return '<section class="plan-exercise" data-block-type="exercise" data-block-id="' + text(block.id) + '"'
            + ' data-exercise-kind="unscramble">'
            + heading
            + '<p class="plan-exercise-progress" data-exercise-progress>' + text(progressLine(1, total || 1)) + '</p>'
            + '<p class="plan-exercise-summary" data-exercise-summary hidden></p>'
            + '<p class="plan-exercise-live" data-exercise-live aria-live="polite" aria-atomic="true"></p>'
            + '<ol class="plan-exercise-items">' + rows + '</ol>'
            + '</section>';
    }

    function fieldInputId(prefix, itemId, idx) {
        var raw = String(itemId == null ? idx : itemId).replace(/[^a-zA-Z0-9_-]/g, '-');
        return prefix + raw + '-' + String(idx);
    }

    function typedFieldRow(inputId, labelText) {
        return '<label class="plan-exercise-label" for="' + attr(inputId) + '">' + text(labelText) + '</label>'
            + '<input type="text" class="plan-exercise-input" id="' + attr(inputId) + '"'
            + ' lang="he" dir="rtl" autocomplete="off" spellcheck="false" data-exercise-input>';
    }

    function renderTranslate(block) {
        var items = Array.isArray(block.items) ? block.items.slice(0, 12) : [];
        var total = items.length;
        var rows = items.map(function (item, idx) {
            item = item || {};
            var english = item.english == null ? '' : String(item.english);
            var inputId = fieldInputId('ex-translate-', item.id, idx);
            var accept = Array.isArray(item.accept) ? item.accept.slice(0, 8) : [];
            return '<li class="plan-exercise-item" data-exercise-item data-item-id="' + text(item.id) + '"'
                + ' data-exercise-kind="translate"'
                + ' data-answer="' + attr(item.answer) + '"'
                + ' data-accept="' + jsonAttr(accept) + '">'
                + '<p class="plan-exercise-progress-item" aria-hidden="true">' + text(progressLine(idx + 1, total)) + '</p>'
                + '<p class="plan-exercise-prompt gloss" lang="en" dir="ltr">' + text(english) + '</p>'
                + typedFieldRow(inputId, 'Type your Hebrew answer / כתבו בעברית')
                + '<button type="button" class="plan-exercise-choice" data-exercise-submit lang="en" dir="ltr">Check</button>'
                + '<p class="plan-exercise-reveal pointed-line" data-exercise-answer-reveal hidden lang="he" dir="rtl"></p>'
                + '<p class="plan-exercise-feedback" data-exercise-feedback hidden></p>'
                + '</li>';
        }).join('');
        var heading = block.heading
            ? '<h2 class="plan-exercise-heading">' + text(block.heading) + '</h2>'
            : '';
        return '<section class="plan-exercise" data-block-type="exercise" data-block-id="' + text(block.id) + '"'
            + ' data-exercise-kind="translate">'
            + heading
            + '<p class="plan-exercise-progress" data-exercise-progress>' + text(progressLine(1, total || 1)) + '</p>'
            + '<p class="plan-exercise-summary" data-exercise-summary hidden></p>'
            + '<p class="plan-exercise-live" data-exercise-live aria-live="polite" aria-atomic="true"></p>'
            + '<ol class="plan-exercise-items">' + rows + '</ol>'
            + '</section>';
    }

    function safeDictationPath(raw) {
        if (typeof raw !== 'string' || !DICTATION_AUDIO.test(raw)) return '';
        return raw.replace(/^\.\.\//, '');
    }

    function renderDictation(block, options) {
        var slug = lessonSlugFrom(options);
        var slugAttr = slug ? ' data-exercise-lesson="' + text(slug) + '"' : '';
        var items = Array.isArray(block.items) ? block.items.slice(0, 12) : [];
        var total = items.length;
        var rows = items.map(function (item, idx) {
            item = item || {};
            var inputId = fieldInputId('ex-dictation-', item.id, idx);
            var accept = Array.isArray(item.accept) ? item.accept.slice(0, 8) : [];
            var safePath = safeDictationPath(item.audioPath);
            var playBtn = safePath
                ? '<button type="button" class="plan-exercise-choice" data-dictation-play'
                    + ' data-audio-path="' + attr(safePath) + '" lang="en" dir="ltr">Play / Replay</button>'
                : '<button type="button" class="plan-exercise-choice" data-dictation-play disabled'
                    + ' lang="en" dir="ltr">Play / Replay</button>';
            return '<li class="plan-exercise-item" data-exercise-item data-item-id="' + text(item.id) + '"'
                + ' data-exercise-kind="dictation"'
                + ' data-answer="' + attr(item.answer) + '"'
                + ' data-accept="' + jsonAttr(accept) + '">'
                + '<p class="plan-exercise-progress-item" aria-hidden="true">' + text(progressLine(idx + 1, total)) + '</p>'
                + playBtn
                + '<button type="button" class="plan-exercise-choice" data-dictation-skip hidden lang="en" dir="ltr">Skip</button>'
                + typedFieldRow(inputId, 'Type what you hear / כתבו מה ששמעתם')
                + '<button type="button" class="plan-exercise-choice" data-exercise-submit lang="en" dir="ltr">Check</button>'
                + '<p class="plan-exercise-reveal pointed-line" data-exercise-answer-reveal hidden lang="he" dir="rtl"></p>'
                + '<p class="plan-exercise-feedback" data-exercise-feedback hidden></p>'
                + '</li>';
        }).join('');
        var heading = block.heading
            ? '<h2 class="plan-exercise-heading">' + text(block.heading) + '</h2>'
            : '';
        return '<section class="plan-exercise" data-block-type="exercise" data-block-id="' + text(block.id) + '"'
            + ' data-exercise-kind="dictation"' + slugAttr + '>'
            + heading
            + '<p class="plan-exercise-progress" data-exercise-progress>' + text(progressLine(1, total || 1)) + '</p>'
            + '<p class="plan-exercise-summary" data-exercise-summary hidden></p>'
            + '<p class="plan-exercise-live" data-exercise-live aria-live="polite" aria-atomic="true"></p>'
            + '<ol class="plan-exercise-items">' + rows + '</ol>'
            + '</section>';
    }

    /**
     * @param {object} block
     * @param {{ lesson?: object }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'exercise') return '';
        var kind = block.kind == null ? '' : String(block.kind);
        if (kind === 'choose') return renderChoose(block);
        if (kind === 'true-false') return renderTrueFalse(block);
        if (kind === 'unscramble') return renderUnscramble(block);
        if (kind === 'translate') return renderTranslate(block);
        if (kind === 'dictation') return renderDictation(block, options);
        return stubSection(block);
    }

    function closest(node, selector) {
        if (!node) return null;
        if (typeof node.closest === 'function') return node.closest(selector);
        for (var cur = node; cur; cur = cur.parentNode) {
            if (!cur.getAttribute) continue;
            if (selector === '[data-exercise-choice]' && cur.getAttribute('data-exercise-choice') != null) return cur;
            if (selector === '[data-exercise-item]' && cur.getAttribute('data-exercise-item') != null) return cur;
            if (selector === '[data-block-type="exercise"]' && cur.getAttribute('data-block-type') === 'exercise') return cur;
            if (selector.charAt(0) === '[' && cur.hasAttribute && cur.hasAttribute(selector.slice(1, -1))) return cur;
        }
        return null;
    }

    function setClass(node, name, on) {
        if (!node) return;
        if (node.classList && typeof node.classList.toggle === 'function') {
            node.classList.toggle(name, !!on);
            return;
        }
        var raw = node.getAttribute ? (node.getAttribute('class') || '') : '';
        var parts = raw.split(/\s+/).filter(Boolean);
        var at = parts.indexOf(name);
        if (on && at < 0) parts.push(name);
        if (!on && at >= 0) parts.splice(at, 1);
        if (node.setAttribute) node.setAttribute('class', parts.join(' '));
    }

    function announce(section, doc, message) {
        var region = section.querySelector ? section.querySelector('[data-exercise-live]') : null;
        if (!region && doc && typeof doc.querySelector === 'function') {
            region = doc.querySelector('[data-exercise-live]');
        }
        if (region) region.textContent = message;
    }

    function feedbackText(step) {
        if (step === 'correct') {
            return '\u2713 \u05E0\u05DB\u05D5\u05DF \u2014 Correct';
        }
        if (step === 'retry') {
            return '\u21BB \u05E0\u05E1\u05D5 \u05E9\u05D5\u05D1 \u2014 Try again';
        }
        return '\u2717 \u05D4\u05EA\u05E9\u05D5\u05D1\u05D4 \u2014 Answer shown';
    }

    function parseJsonAttr(node, name, fallback) {
        if (!node || !node.getAttribute) return fallback;
        var raw = node.getAttribute(name);
        if (!raw) return fallback;
        try {
            return JSON.parse(raw);
        } catch (err) {
            return fallback;
        }
    }

    function isChooseCorrect(kind, picked, expected) {
        if (kind === 'true-false') {
            return String(picked) === String(expected);
        }
        return sameHebrew(picked, expected);
    }

    function markChoice(button, state) {
        if (!button) return;
        button.setAttribute('data-exercise-state', state);
        setClass(button, 'is-correct', state === 'correct' || state === 'answer');
        setClass(button, 'is-wrong', state === 'wrong');
        if (state === 'correct' || state === 'answer') button.disabled = true;
    }

    function revealAnswer(item) {
        var kind = item.getAttribute('data-exercise-kind') || 'choose';
        var expected = item.getAttribute('data-answer') || '';
        var buttons = item.querySelectorAll ? item.querySelectorAll('[data-exercise-choice]') : [];
        for (var i = 0; i < buttons.length; i += 1) {
            var btn = buttons[i];
            var val = kind === 'true-false'
                ? (btn.getAttribute('data-tf-value') || '')
                : (btn.getAttribute('data-choice-value') || '');
            if (isChooseCorrect(kind, val, expected)) markChoice(btn, 'answer');
            else if (btn.getAttribute('data-exercise-state') !== 'wrong') btn.disabled = true;
        }
    }

    function isResolvedState(st) {
        return st === 'correct' || st === 'reveal' || st === 'skipped';
    }

    function updateProgress(section, doc) {
        var items = section.querySelectorAll ? section.querySelectorAll('[data-exercise-item]') : [];
        var total = items.length;
        var resolved = 0;
        var firstTry = 0;
        var nextIndex = total;
        for (var i = 0; i < items.length; i += 1) {
            var st = items[i].getAttribute('data-exercise-resolved');
            if (isResolvedState(st)) {
                resolved += 1;
                if (st === 'correct') {
                    var attempts = Number(items[i].getAttribute('data-exercise-attempts') || '0');
                    if (attempts === 0) firstTry += 1;
                }
            } else if (nextIndex === total) {
                nextIndex = i;
            }
        }
        var progress = section.querySelector('[data-exercise-progress]');
        if (progress) {
            var show = resolved >= total ? total : nextIndex + 1;
            progress.textContent = progressLine(show, total);
        }
        var summary = section.querySelector('[data-exercise-summary]');
        if (summary && resolved >= total && total > 0) {
            var line = summaryLine(firstTry, total);
            var firstSummary = summary.hidden;
            summary.hidden = false;
            summary.textContent = line;
            if (firstSummary && total > 1) announce(section, doc, line);
        }
    }

    function applyStep(section, doc, item, step, feedback, revealNode, revealText) {
        if (feedback) {
            feedback.hidden = false;
            feedback.textContent = feedbackText(step);
        }
        if (step === 'retry') {
            if (revealNode) revealNode.hidden = true;
            announce(section, doc, feedbackText('retry'));
            return;
        }
        if (step === 'correct') {
            item.setAttribute('data-exercise-resolved', 'correct');
            if (revealNode && revealText != null) {
                revealNode.hidden = false;
                revealNode.textContent = revealText;
            }
            announce(section, doc, feedbackText('correct'));
            updateProgress(section, doc);
            return;
        }
        item.setAttribute('data-exercise-resolved', 'reveal');
        if (revealNode && revealText != null) {
            revealNode.hidden = false;
            revealNode.textContent = revealText;
        }
        announce(section, doc, feedbackText('reveal'));
        updateProgress(section, doc);
    }

    function onChoice(section, doc, item, button) {
        if (item.getAttribute('data-exercise-resolved')) return;
        var kind = item.getAttribute('data-exercise-kind') || 'choose';
        var expected = item.getAttribute('data-answer') || '';
        var picked = kind === 'true-false'
            ? (button.getAttribute('data-tf-value') || '')
            : (button.getAttribute('data-choice-value') || '');
        var correct = isChooseCorrect(kind, picked, expected);
        var attempts = Number(item.getAttribute('data-exercise-attempts') || '0');
        if (!correct) attempts += 1;
        item.setAttribute('data-exercise-attempts', String(attempts));
        var step = stepAttempt(attempts, correct, 2);
        var feedback = item.querySelector('[data-exercise-feedback]');
        var why = item.querySelector('[data-exercise-why]');

        if (step === 'retry') {
            markChoice(button, 'wrong');
            if (feedback) {
                feedback.hidden = false;
                feedback.textContent = feedbackText('retry');
            }
            if (why) why.hidden = true;
            announce(section, doc, feedbackText('retry'));
            return;
        }

        if (step === 'correct') {
            markChoice(button, 'correct');
            item.setAttribute('data-exercise-resolved', 'correct');
            revealAnswer(item);
            if (feedback) {
                feedback.hidden = false;
                feedback.textContent = feedbackText('correct');
            }
            if (why) why.hidden = false;
            announce(section, doc, feedbackText('correct'));
            updateProgress(section, doc);
            return;
        }

        markChoice(button, 'wrong');
        item.setAttribute('data-exercise-resolved', 'reveal');
        revealAnswer(item);
        if (feedback) {
            feedback.hidden = false;
            feedback.textContent = feedbackText('reveal');
        }
        if (why) why.hidden = false;
        announce(section, doc, feedbackText('reveal'));
        updateProgress(section, doc);
    }

    function pickedWords(item) {
        var picked = item.querySelector ? item.querySelector('[data-unscramble-picked]') : null;
        if (!picked || !picked.querySelectorAll) return [];
        var buttons = picked.querySelectorAll('[data-unscramble-picked-idx]');
        var out = [];
        for (var i = 0; i < buttons.length; i += 1) {
            out.push(buttons[i].getAttribute('data-word') || buttons[i].textContent || '');
        }
        return out;
    }

    function ordersMatch(a, b) {
        if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
        for (var i = 0; i < a.length; i += 1) {
            if (a[i] !== b[i]) return false;
        }
        return true;
    }

    function ordersMatchPoints(a, b) {
        if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
        for (var i = 0; i < a.length; i += 1) {
            if (stripPoints(a[i]) !== stripPoints(b[i])) return false;
        }
        return true;
    }

    function unscrambleCorrect(item, order) {
        var words = parseJsonAttr(item, 'data-unscramble-words', []);
        var alts = parseJsonAttr(item, 'data-unscramble-alt', []);
        if (ordersMatch(order, words) || ordersMatchPoints(order, words)) return true;
        for (var i = 0; i < alts.length; i += 1) {
            if (ordersMatch(order, alts[i]) || ordersMatchPoints(order, alts[i])) return true;
        }
        return false;
    }

    function refreshPickedIndices(pickedRow) {
        if (!pickedRow || !pickedRow.querySelectorAll) return;
        var buttons = pickedRow.querySelectorAll('[data-unscramble-picked-idx]');
        for (var i = 0; i < buttons.length; i += 1) {
            buttons[i].setAttribute('data-unscramble-picked-idx', String(i));
        }
    }

    function moveBankToPicked(doc, item, bankBtn) {
        if (item.getAttribute('data-exercise-resolved')) return;
        var pickedRow = item.querySelector('[data-unscramble-picked]');
        if (!pickedRow || bankBtn.hidden) return;
        var word = bankBtn.getAttribute('data-word') || '';
        var bankIdx = bankBtn.getAttribute('data-unscramble-bank-idx') || '';
        var owner = (bankBtn.ownerDocument || doc);
        var btn = owner && typeof owner.createElement === 'function'
            ? owner.createElement('button')
            : null;
        if (!btn) return;
        btn.setAttribute('type', 'button');
        btn.setAttribute('class', 'plan-exercise-choice');
        btn.setAttribute('data-unscramble-picked-idx', String(pickedRow.children.length));
        btn.setAttribute('data-word', word);
        btn.setAttribute('data-unscramble-from-bank', bankIdx);
        btn.setAttribute('lang', 'he');
        btn.setAttribute('dir', 'rtl');
        btn.textContent = word;
        pickedRow.appendChild(btn);
        bankBtn.hidden = true;
    }

    function movePickedToBank(item, pickedBtn) {
        if (item.getAttribute('data-exercise-resolved')) return;
        var bankIdx = pickedBtn.getAttribute('data-unscramble-from-bank');
        var bank = item.querySelector('[data-unscramble-bank]');
        if (!bank || bankIdx == null) return;
        var bankBtn = bank.querySelector('[data-unscramble-bank-idx="' + bankIdx + '"]');
        if (bankBtn) bankBtn.hidden = false;
        if (pickedBtn.parentNode) pickedBtn.parentNode.removeChild(pickedBtn);
        refreshPickedIndices(item.querySelector('[data-unscramble-picked]'));
    }

    function onUnscrambleCheck(section, doc, item) {
        if (item.getAttribute('data-exercise-resolved')) return;
        var order = pickedWords(item);
        var words = parseJsonAttr(item, 'data-unscramble-words', []);
        if (order.length !== words.length) return;
        var correct = unscrambleCorrect(item, order);
        var attempts = Number(item.getAttribute('data-exercise-attempts') || '0');
        if (!correct) attempts += 1;
        item.setAttribute('data-exercise-attempts', String(attempts));
        var step = stepAttempt(attempts, correct, 2);
        var feedback = item.querySelector('[data-exercise-feedback]');
        var reveal = item.querySelector('[data-unscramble-reveal]');
        var revealText = words.join(' ');
        if (step === 'retry') {
            if (feedback) {
                feedback.hidden = false;
                feedback.textContent = feedbackText('retry');
            }
            if (reveal) reveal.hidden = true;
            announce(section, doc, feedbackText('retry'));
            return;
        }
        if (step === 'correct') {
            applyStep(section, doc, item, 'correct', feedback, reveal, revealText);
            return;
        }
        applyStep(section, doc, item, 'reveal', feedback, reveal, revealText);
    }

    function onTypedSubmit(section, doc, item) {
        if (item.getAttribute('data-exercise-resolved')) return;
        var input = item.querySelector ? item.querySelector('[data-exercise-input]') : null;
        var val = input ? input.value : '';
        var answer = item.getAttribute('data-answer') || '';
        var accept = parseJsonAttr(item, 'data-accept', []);
        var correct = sameHebrew(val, answer, accept);
        var attempts = Number(item.getAttribute('data-exercise-attempts') || '0');
        if (!correct) attempts += 1;
        item.setAttribute('data-exercise-attempts', String(attempts));
        var step = stepAttempt(attempts, correct, 2);
        var feedback = item.querySelector('[data-exercise-feedback]');
        var reveal = item.querySelector('[data-exercise-answer-reveal]');
        if (step === 'correct') {
            applyStep(section, doc, item, 'correct', feedback, reveal, answer);
            if (input) input.disabled = true;
            return;
        }
        if (step === 'retry') {
            if (feedback) {
                feedback.hidden = false;
                feedback.textContent = feedbackText('retry');
            }
            if (reveal) reveal.hidden = true;
            announce(section, doc, feedbackText('retry'));
            return;
        }
        applyStep(section, doc, item, 'reveal', feedback, reveal, answer);
        if (input) input.disabled = true;
    }

    function resolveDictationUrl(audioPath, lessonSlug) {
        if (typeof audioPath !== 'string' || !DICTATION_AUDIO.test(audioPath)) return '';
        var wordMatch = WORD_CLIP.exec(audioPath);
        if (wordMatch) return '/worksheets/_audio/words/' + wordMatch[1] + '.mp3';
        var rel = audioPath.replace(/^\.\.\//, '');
        if (!LESSON_SLUG.test(lessonSlug)) return '';
        return '/worksheets/' + encodeURIComponent(lessonSlug) + '/'
            + rel.split('/').map(function (part) { return encodeURIComponent(part); }).join('/');
    }

    function dictationLoadError(section, doc, item) {
        var feedback = item.querySelector('[data-exercise-feedback]');
        var msg = 'השמע לא נטען — Audio could not load. You can skip this item.';
        if (feedback) {
            feedback.hidden = false;
            feedback.textContent = msg;
        }
        var skip = item.querySelector('[data-dictation-skip]');
        if (skip) skip.hidden = false;
        announce(section, doc, msg);
    }

    function onDictationPlay(section, doc, item, button) {
        var path = button.getAttribute('data-audio-path') || '';
        if (!path || !DICTATION_AUDIO.test(path)) {
            dictationLoadError(section, doc, item);
            return;
        }
        var slug = section.getAttribute('data-exercise-lesson') || '';
        var url = resolveDictationUrl(path, slug);
        if (!url) {
            dictationLoadError(section, doc, item);
            return;
        }
        var AudioCtor = (doc && doc.defaultView && doc.defaultView.Audio) || root.Audio;
        if (typeof AudioCtor !== 'function') {
            dictationLoadError(section, doc, item);
            return;
        }
        var player;
        try {
            player = new AudioCtor(url);
        } catch (err) {
            dictationLoadError(section, doc, item);
            return;
        }
        var failed = function () {
            dictationLoadError(section, doc, item);
        };
        if (typeof player.addEventListener === 'function') {
            player.addEventListener('error', failed);
        }
        var playResult;
        try {
            playResult = player.play();
        } catch (err2) {
            failed();
            return;
        }
        if (playResult && typeof playResult.then === 'function') {
            playResult.catch(failed);
        }
    }

    function onDictationSkip(section, doc, item) {
        if (item.getAttribute('data-exercise-resolved')) return;
        item.setAttribute('data-exercise-resolved', 'skipped');
        var input = item.querySelector('[data-exercise-input]');
        if (input) input.disabled = true;
        var feedback = item.querySelector('[data-exercise-feedback]');
        if (feedback) {
            feedback.hidden = false;
            feedback.textContent = 'Skipped — דילגתם';
        }
        announce(section, doc, 'Skipped');
        updateProgress(section, doc);
    }

    function activateButton(section, doc, button) {
        if (!button || button.disabled) return;
        if (button.getAttribute('data-exercise-choice') != null) {
            var itemChoice = closest(button, '[data-exercise-item]');
            if (itemChoice) onChoice(section, doc, itemChoice, button);
            return;
        }
        if (button.hasAttribute('data-unscramble-bank-idx')) {
            var itemBank = closest(button, '[data-exercise-item]');
            if (itemBank) moveBankToPicked(doc, itemBank, button);
            return;
        }
        if (button.hasAttribute('data-unscramble-picked-idx')) {
            var itemPick = closest(button, '[data-exercise-item]');
            if (itemPick) movePickedToBank(itemPick, button);
            return;
        }
        if (button.hasAttribute('data-unscramble-check')) {
            var itemCheck = closest(button, '[data-exercise-item]');
            if (itemCheck) onUnscrambleCheck(section, doc, itemCheck);
            return;
        }
        if (button.hasAttribute('data-exercise-submit')) {
            var itemSubmit = closest(button, '[data-exercise-item]');
            if (itemSubmit) onTypedSubmit(section, doc, itemSubmit);
            return;
        }
        if (button.hasAttribute('data-dictation-play')) {
            var itemPlay = closest(button, '[data-exercise-item]');
            if (itemPlay) onDictationPlay(section, doc, itemPlay, button);
            return;
        }
        if (button.hasAttribute('data-dictation-skip')) {
            var itemSkip = closest(button, '[data-exercise-item]');
            if (itemSkip) onDictationSkip(section, doc, itemSkip);
        }
    }

    /**
     * @param {Document} doc
     * @param {object} [_live]
     * @returns {void}
     */
    function bind(doc, _live) {
        if (!doc || typeof doc.querySelectorAll !== 'function') return;
        var sections = doc.querySelectorAll('[data-block-type="exercise"]');
        for (var s = 0; s < sections.length; s += 1) {
            var section = sections[s];
            if (!section || section.getAttribute('data-plan-bound') === '1') continue;
            if (section.getAttribute('data-exercise-stub') === '1') continue;
            if (typeof section.addEventListener !== 'function') continue;
            section.setAttribute('data-plan-bound', '1');
            (function (boundSection) {
                boundSection.addEventListener('click', function (event) {
                    var target = event.target;
                    if (!target) return;
                    var button = target;
                    if (typeof target.closest === 'function') {
                        button = target.closest('button') || target;
                    }
                    if (!button || !button.getAttribute) return;
                    activateButton(boundSection, doc, button);
                });
                boundSection.addEventListener('keydown', function (event) {
                    var key = event.key;
                    if (key !== 'Enter' && key !== ' ' && key !== 'Spacebar') return;
                    var target = event.target;
                    if (!target) return;
                    if (target.getAttribute && target.getAttribute('data-exercise-input') != null) {
                        if (key === 'Enter') {
                            if (event.preventDefault) event.preventDefault();
                            var submitBtn = closest(target, '[data-exercise-item]');
                            if (submitBtn) {
                                var btn = submitBtn.querySelector('[data-exercise-submit]');
                                if (btn) onTypedSubmit(boundSection, doc, submitBtn);
                            }
                        }
                        return;
                    }
                    var button = target;
                    if (typeof target.closest === 'function') button = target.closest('button') || target;
                    if (!button || button.disabled) return;
                    if (event.preventDefault) event.preventDefault();
                    activateButton(boundSection, doc, button);
                });
            })(section);
        }
    }

    return {
        bind: bind,
        escapeHtml: escapeHtml,
        render: render,
        resolveDictationUrl: resolveDictationUrl,
        unscrambleCorrect: unscrambleCorrect
    };
});
