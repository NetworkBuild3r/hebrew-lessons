/**
 * Multi-sentence writing tasks with optional homework lines.
 * Learner text stays in the page. Nothing is posted or stored.
 * Provenance: INIT-012/SPEC-006
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockAssignment = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var WORD_CLIP = /^(?:\.\.\/)?_audio\/words\/([A-Za-z0-9_-]{8,64})\.mp3$/;
    var seq = 0;

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

    function uid(prefix) {
        seq += 1;
        return prefix + '-' + seq;
    }

    /**
     * Count non-empty sentence segments. `.`, `?`, `!`, and line breaks end a sentence.
     * @param {string} str
     * @returns {number}
     */
    function countSentences(str) {
        var raw = String(str == null ? '' : str);
        if (!raw.trim()) return 0;
        var parts = raw.split(/[.?!\r\n]+/);
        var count = 0;
        for (var i = 0; i < parts.length; i += 1) {
            if (parts[i].trim()) count += 1;
        }
        return count;
    }

    function clampMin(value) {
        var n = Number(value);
        if (!Number.isFinite(n)) return 1;
        if (n < 1) return 1;
        if (n > 12) return 12;
        return Math.floor(n);
    }

    function clipForWord(word, audioMap) {
        if (!word || !audioMap || typeof audioMap !== 'object') return '';
        var raw = String(word).replace(/\s+/g, ' ').trim();
        var path = audioMap[raw] || audioMap[raw.normalize('NFC')] || '';
        if (typeof path !== 'string') return '';
        var match = WORD_CLIP.exec(path);
        return match ? match[1] : '';
    }

    function resolvePlayWord(options) {
        if (options && typeof options.playWord === 'function') return options.playWord;
        var render = root.HebrewPlanRender;
        if (render && typeof render.playWord === 'function') return render.playWord;
        return null;
    }

    /**
     * @param {object} block
     * @param {{ audio?: object, playWord?: function }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'assignment') return '';
        var minSentences = clampMin(block.minSentences);
        var fieldId = uid('assignment-text');
        var words = Array.isArray(block.useWords) ? block.useWords.slice(0, 12) : [];
        var homework = Array.isArray(block.homework) ? block.homework.slice(0, 6) : [];
        var audioMap = options && options.audio;
        if (!audioMap && root.HEBREW_WORD_AUDIO) audioMap = root.HEBREW_WORD_AUDIO;

        var chips = words.filter(function (word) {
            return typeof word === 'string' && word.trim();
        }).map(function (word) {
            var trimmed = word.trim();
            var clip = clipForWord(trimmed, audioMap);
            var clipAttr = clip ? ' data-assignment-clip="' + text(clip) + '"' : '';
            return '<button type="button" class="plan-assignment-word" data-assignment-word="'
                + text(trimmed) + '"' + clipAttr + ' lang="he" dir="rtl">' + text(trimmed) + '</button>';
        }).join('');

        var wordsBlock = chips
            ? '<div class="plan-assignment-words" data-assignment-words>' + chips + '</div>'
            : '';

        var hwItems = homework.filter(function (line) {
            return typeof line === 'string' && line.trim();
        }).map(function (line, index) {
            var hwId = uid('assignment-hw');
            return '<li><input type="checkbox" id="' + hwId + '" data-assignment-hw="' + index + '">'
                + '<label for="' + hwId + '">' + text(line.trim()) + '</label></li>';
        }).join('');

        var hwBlock = hwItems
            ? '<fieldset class="plan-assignment-homework"><legend lang="he">שיעורי בית '
                + '<span lang="en" dir="ltr">Homework</span></legend><ul>' + hwItems + '</ul></fieldset>'
            : '';

        return '<section class="plan-assignment" data-block-type="assignment" data-block-id="' + text(block.id) + '"'
            + ' data-assignment-min="' + minSentences + '">'
            + '<h2 lang="he">מטלת כתיבה <span lang="en" dir="ltr">Writing assignment</span></h2>'
            + '<p class="plan-assignment-prompt" lang="he" dir="rtl">' + text(block.prompt) + '</p>'
            + '<p class="plan-assignment-min" data-assignment-min-line lang="he">כתבו לפחות '
            + minSentences + ' משפטים · <span lang="en" dir="ltr">Write at least ' + minSentences
            + ' sentences</span></p>'
            + '<p class="plan-assignment-help gloss" lang="he">נקודה, סימן שאלה, סימן קריאה או שורה חדשה מסיימים משפט. '
            + '<span lang="en" dir="ltr">A period, question mark, exclamation mark, or line break ends a sentence. '
            + 'Empty segments are ignored.</span></p>'
            + wordsBlock
            + '<label class="field plan-assignment-field" for="' + fieldId + '">הכתיבה שלכם '
            + '<span lang="en" dir="ltr">Your writing</span></label>'
            + '<textarea id="' + fieldId + '" class="plan-assignment-textarea" data-assignment-textarea '
            + 'lang="he" dir="rtl" rows="6" autocomplete="off" autocorrect="off" spellcheck="false"></textarea>'
            + '<p class="plan-assignment-count" data-assignment-count aria-live="polite">0 / '
            + minSentences + '</p>'
            + '<p class="plan-assignment-done" data-assignment-done hidden lang="he">'
            + 'הגעתם למינימום — אפשר להמשיך. '
            + '<span lang="en" dir="ltr">You reached the minimum — you can move on.</span></p>'
            + hwBlock
            + '</section>';
    }

    function syncCount(section) {
        var min = clampMin(section.getAttribute('data-assignment-min'));
        var textarea = section.querySelector('[data-assignment-textarea]');
        var countNode = section.querySelector('[data-assignment-count]');
        var done = section.querySelector('[data-assignment-done]');
        var value = textarea ? textarea.value : '';
        var count = countSentences(value);
        if (countNode) countNode.textContent = count + ' / ' + min;
        if (done) done.hidden = count < min;
    }

    function playAssignmentWord(section, button, playWord, audioMap) {
        if (typeof playWord !== 'function') return;
        var word = button.getAttribute('data-assignment-word') || '';
        var clip = button.getAttribute('data-assignment-clip') || '';
        if (!clip) {
            if (!audioMap && root.HEBREW_WORD_AUDIO) audioMap = root.HEBREW_WORD_AUDIO;
            clip = clipForWord(word, audioMap);
        }
        if (!clip) return;
        try {
            var result = playWord(clip, word || null);
            if (result && typeof result.then === 'function') {
                result.then(function () { /* quiet when nothing plays */ }, function () { /* quiet */ });
            }
        } catch (err) { /* quiet */ }
    }

    /**
     * @param {Document} doc
     * @param {{ playWord?: function, audio?: object }} [live]
     * @returns {void}
     */
    function bind(doc, live) {
        if (!doc || typeof doc.querySelectorAll !== 'function') return;
        var playWord = resolvePlayWord(live);
        var audioMap = live && live.audio;
        if (!audioMap && root.HEBREW_WORD_AUDIO) audioMap = root.HEBREW_WORD_AUDIO;
        var sections = doc.querySelectorAll('[data-block-type="assignment"]');
        for (var i = 0; i < sections.length; i += 1) {
            var section = sections[i];
            if (!section || section.getAttribute('data-plan-bound') === '1') continue;
            if (typeof section.addEventListener !== 'function') continue;
            section.setAttribute('data-plan-bound', '1');
            syncCount(section);
            section.addEventListener('input', function (event) {
                var target = event.target;
                if (!target || typeof target.matches !== 'function' || !target.matches('[data-assignment-textarea]')) return;
                var host = target.closest('[data-block-type="assignment"]');
                if (host) syncCount(host);
            });
            section.addEventListener('click', function (event) {
                var target = event.target;
                if (!target || typeof target.closest !== 'function') return;
                var host = target.closest('[data-block-type="assignment"]');
                if (!host) return;
                var wordBtn = target.closest('[data-assignment-word]');
                if (wordBtn) playAssignmentWord(host, wordBtn, playWord, audioMap);
            });
        }
    }

    return {
        bind: bind,
        clipForWord: clipForWord,
        countSentences: countSentences,
        escapeHtml: escapeHtml,
        render: render
    };
});
