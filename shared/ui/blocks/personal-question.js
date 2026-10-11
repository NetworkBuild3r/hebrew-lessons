/**
 * Personal questions. The learner may say an answer or type one.
 * The typed text stays in the page. The lesson blob stores a done boolean only.
 * Nothing in the answer is posted.
 * Provenance: INIT-006/SPEC-014
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockPersonal = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var ANSWERS = { speak: true, type: true, either: true };
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
     * Copy of the lesson blob with this block marked done.
     * Only `true` marks are kept. Typed text is not copied.
     * @param {object|null|undefined} state
     * @param {string} blockId
     * @returns {object}
     */
    function donePatch(state, blockId) {
        var base = state && typeof state === 'object' && !Array.isArray(state) ? state : {};
        var next = {};
        Object.keys(base).forEach(function (key) {
            next[key] = base[key];
        });
        var activity = {};
        var prev = base.activityState;
        if (prev && typeof prev === 'object' && !Array.isArray(prev)) {
            Object.keys(prev).forEach(function (key) {
                activity[key] = prev[key];
            });
        }
        var marks = {};
        var prevMarks = activity['personal-question'];
        if (prevMarks && typeof prevMarks === 'object' && !Array.isArray(prevMarks)) {
            Object.keys(prevMarks).forEach(function (key) {
                if (prevMarks[key] === true) marks[key] = true;
            });
        }
        if (blockId) marks[String(blockId)] = true;
        activity['personal-question'] = marks;
        next.activityState = activity;
        return next;
    }

    /**
     * @param {object} block
     * @param {{ lesson?: object }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'personal-question') return '';
        var answer = ANSWERS[block.answer] ? block.answer : 'speak';
        var prompts = Array.isArray(block.prompts) ? block.prompts.slice(0, 6) : [];
        var items = prompts.map(function (prompt) {
            prompt = prompt || {};
            var sayId = uid('say');
            var english = prompt.en
                ? '<p class="gloss personal-en" lang="en" dir="ltr" hidden>' + text(prompt.en) + '</p>'
                : '';
            return '<li data-prompt-id="' + text(prompt.id) + '">'
                + '<p class="pointed-line" lang="he" id="' + sayId + '" data-say-text>' + text(prompt.he) + '</p>'
                + english
                + '<p class="dock"><button type="button" class="audio" data-say-aloud data-say-for="' + sayId + '">'
                + 'אמרו בקול <span lang="en" dir="ltr">Say it aloud</span></button></p>'
                + '</li>';
        }).join('');
        var box = '';
        if (answer === 'type' || answer === 'either') {
            var fieldId = uid('personal-answer');
            box = '<label class="field" for="' + fieldId + '">התשובה שלכם '
                + '<span lang="en" dir="ltr">Your answer</span>'
                + '<input id="' + fieldId + '" data-personal-input type="text" autocomplete="off" '
                + 'autocorrect="off" spellcheck="false" lang="he" dir="rtl">'
                + '</label>';
        }
        var lesson = options && options.lesson;
        var slug = lesson && typeof lesson.id === 'string' && /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(lesson.id)
            ? lesson.id
            : '';
        var slugAttr = slug ? ' data-lesson-slug="' + slug + '"' : '';
        return '<section data-block-type="personal-question" data-block-id="' + text(block.id) + '"'
            + ' data-answer="' + answer + '"' + slugAttr + '>'
            + '<h2>שאלה אישית <span lang="en" dir="ltr">Personal question</span></h2>'
            + '<ul class="recycle">' + items + '</ul>'
            + '<p class="dock"><button type="button" data-english-toggle>'
            + 'English <span lang="he">אנגלית</span></button></p>'
            + box
            + '<p class="dock"><button type="button" class="primary" data-personal-done>'
            + 'סיימתי <span lang="en" dir="ltr">Done</span></button></p>'
            + '<p class="gloss" data-personal-saved hidden></p>'
            + '</section>';
    }

    function sayAloud(section, button) {
        var id = button.getAttribute('data-say-for');
        var node = id && section.ownerDocument
            ? section.ownerDocument.getElementById(id)
            : null;
        var line = node ? node.textContent : '';
        var synth = root.speechSynthesis;
        if (!line || !synth || typeof root.SpeechSynthesisUtterance !== 'function') return;
        try {
            var utter = new root.SpeechSynthesisUtterance(line);
            utter.lang = 'he-IL';
            synth.cancel();
            synth.speak(utter);
        } catch (err) { /* the line stays on the page; nothing is sent */ }
    }

    /**
     * @param {Document} doc
     * @returns {void}
     */
    function bind(doc) {
        if (!doc || typeof doc.querySelectorAll !== 'function') return;
        var sections = doc.querySelectorAll('[data-block-type="personal-question"]');
        for (var i = 0; i < sections.length; i += 1) {
            var section = sections[i];
            if (!section || section.getAttribute('data-plan-bound') === '1') continue;
            if (typeof section.addEventListener !== 'function') continue;
            section.setAttribute('data-plan-bound', '1');
            section.addEventListener('click', function (event) {
                var target = event.target;
                if (!target || typeof target.closest !== 'function') return;
                var host = target.closest('[data-block-type="personal-question"]');
                if (!host) return;
                var say = target.closest('[data-say-aloud]');
                if (say) {
                    sayAloud(host, say);
                    return;
                }
                var toggle = target.closest('[data-english-toggle]');
                if (toggle) {
                    var glosses = host.querySelectorAll('.personal-en');
                    for (var g = 0; g < glosses.length; g += 1) glosses[g].hidden = !glosses[g].hidden;
                    return;
                }
                var done = target.closest('[data-personal-done]');
                if (!done) return;
                var Sync = root.HebrewSync;
                var current = Sync && typeof Sync.getLessonProgress === 'function'
                    ? Sync.getLessonProgress()
                    : { state: {} };
                var blockId = host.getAttribute('data-block-id') || '';
                var patch = donePatch(current && current.state, blockId);
                if (Sync && typeof Sync.saveLessonProgress === 'function') Sync.saveLessonProgress(patch);
                var note = host.querySelector('[data-personal-saved]');
                if (note) {
                    note.hidden = false;
                    note.textContent = 'נשמר. / Saved.';
                }
            });
        }
    }

    return {
        bind: bind,
        donePatch: donePatch,
        escapeHtml: escapeHtml,
        render: render
    };
});
