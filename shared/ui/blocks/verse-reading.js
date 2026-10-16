/**
 * Verse-by-verse reading. The reference line, then new words, then the
 * pointed verse, then simple questions, then one personal question.
 * The renderer does not fetch scripture. Plan text is escaped.
 * A stored translation of the verse is not shown.
 * Provenance: INIT-006/SPEC-016
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockVerse = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var AUDIO_FILE = /^(?:[A-Za-z0-9_-]+\/)*[A-Za-z0-9_.-]+\.mp3$/;

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

    function cap(value) {
        var raw = value == null ? '' : String(value);
        if (raw.length > TEXT_LIMIT) raw = raw.slice(0, TEXT_LIMIT);
        return raw;
    }

    function stripVowels(value) {
        if (root.HebrewUtils && typeof root.HebrewUtils.stripVowels === 'function') {
            return root.HebrewUtils.stripVowels(value);
        }
        return String(value || '').replace(/[\u0591-\u05C7]/g, '');
    }

    function isHebrew(value) {
        return /[\u0590-\u05FF]/.test(String(value || ''));
    }

    function dirAttr(value) {
        return isHebrew(value) ? 'rtl' : 'ltr';
    }

    function list(value, max) {
        if (!Array.isArray(value)) return [];
        return value.slice(0, max);
    }

    function verseNumber(value, max) {
        var n = typeof value === 'number' ? value : Number(value);
        if (!Number.isInteger(n) || n < 1 || n > max) return null;
        return n;
    }

    function vowelsOn(options) {
        return !(options && options.vowelsOn === false);
    }

    function shown(value, pointsOn) {
        var raw = cap(value);
        return escapeHtml(pointsOn ? raw : stripVowels(raw));
    }

    function pointedSpan(value, pointsOn, tag, extra) {
        var raw = cap(value);
        return '<' + tag + ' lang="he" dir="rtl" data-vowels="' + escapeHtml(raw) + '"'
            + (extra || '') + '>' + shown(raw, pointsOn) + '</' + tag + '>';
    }

    function clipButton(audio, key) {
        var path = audio && key != null ? audio[key] : '';
        if (typeof path !== 'string' || !AUDIO_FILE.test(path)) return '';
        return '<button type="button" class="audio" data-plan-audio="ready">'
            + 'השמעה <span lang="en" dir="ltr">Play</span></button>';
    }

    function glossOf(lexicon, token) {
        if (!lexicon || typeof lexicon !== 'object' || Array.isArray(lexicon)) return '';
        var key = String(token || '').replace(/^[\s.,!?;:׃]+|[\s.,!?;:׃]+$/g, '');
        var row = Object.prototype.hasOwnProperty.call(lexicon, key) ? lexicon[key] : null;
        if (!row && Object.prototype.hasOwnProperty.call(lexicon, token)) row = lexicon[token];
        if (!row || typeof row !== 'object') return '';
        var gloss = row.gloss_en != null ? row.gloss_en : row.glossEn;
        return typeof gloss === 'string' ? gloss : '';
    }

    function personalApi() {
        if (root.HebrewBlockPersonal && typeof root.HebrewBlockPersonal.render === 'function') {
            return root.HebrewBlockPersonal;
        }
        if (typeof require === 'function') {
            try {
                return require('./personal-question.js');
            } catch (err) {
                return null;
            }
        }
        return null;
    }

    function renderRef(ref) {
        ref = ref && typeof ref === 'object' && !Array.isArray(ref) ? ref : {};
        var chapter = verseNumber(ref.chapter, 150);
        var from = verseNumber(ref.from, 176);
        var to = verseNumber(ref.to, 176);
        var span = '';
        if (chapter != null && from != null) {
            span = chapter + ':' + from;
            if (to != null && to !== from) span += '–' + to;
        }
        var book = ref.book == null ? '' : String(ref.book);
        var bookDir = dirAttr(book);
        var label = text(book) + (span ? ' ' + span : '');
        var source = ref.source == null ? '' : String(ref.source);
        return '<p class="kicker" data-verse-part="ref">'
            + '<span lang="' + (bookDir === 'rtl' ? 'he' : 'en') + '" dir="' + bookDir + '">'
            + label + '</span></p>'
            + '<p class="gloss" data-verse-source lang="' + (isHebrew(source) ? 'he' : 'en')
            + '" dir="' + dirAttr(source) + '">' + text(source) + '</p>';
    }

    function renderNewWords(words, audio, pointsOn) {
        var items = list(words, 24).map(function (word) {
            var raw = cap(word);
            return '<li data-new-word="' + escapeHtml(raw) + '">'
                + pointedSpan(raw, pointsOn, 'p', ' class="pointed-line"')
                + clipButton(audio, raw)
                + '</li>';
        }).join('');
        return '<section data-verse-part="new-words"><h3>מילים חדשות '
            + '<span lang="en" dir="ltr">New words</span></h3>'
            + '<ul class="recycle">' + items + '</ul></section>';
    }

    function renderVerseLine(verse, lexicon, pointsOn) {
        var raw = cap(verse);
        var parts = raw.split(/(\s+)/);
        var html = parts.map(function (part) {
            if (!part) return '';
            if (/^\s+$/.test(part)) return escapeHtml(part);
            var gloss = glossOf(lexicon, part);
            var body = shown(part, pointsOn);
            var attr = ' data-vowels="' + escapeHtml(part) + '"';
            if (!gloss) {
                return '<span lang="he" dir="rtl"' + attr + '>' + body + '</span>';
            }
            return '<button type="button" class="verse-word" data-verse-gloss lang="he" dir="rtl"'
                + attr + '>' + body + '</button>'
                + '<span class="meaning verse-gloss" lang="en" dir="ltr" hidden>' + text(gloss) + '</span>';
        }).join('');
        return '<p class="pointed-line" data-verse-part="verse" lang="he" dir="rtl">' + html + '</p>';
    }

    function renderQuestions(questions, pointsOn) {
        var items = list(questions, 4).map(function (question) {
            question = question && typeof question === 'object' ? question : {};
            var english = question.en
                ? '<p class="gloss" lang="en" dir="ltr">' + text(question.en) + '</p>'
                : '';
            return '<li data-question-id="' + text(question.id) + '">'
                + pointedSpan(question.he, pointsOn, 'p', ' class="pointed-line"')
                + english
                + '</li>';
        }).join('');
        if (!items) return '';
        return '<ol class="recycle" data-verse-part="questions">' + items + '</ol>';
    }

    function renderPersonal(block, options) {
        var ask = block.personalQuestion;
        if (!ask || typeof ask !== 'object' || Array.isArray(ask)) return '';
        if (ask.he == null || ask.he === '') return '';
        var api = personalApi();
        if (!api) return '';
        var id = cap(block.id || 'verse') + '-ask';
        return api.render({
            id: id,
            type: 'personal-question',
            prompts: [{ id: 'ask', he: ask.he, en: ask.en }],
            answer: 'speak'
        }, { lesson: options && options.lesson });
    }

    /**
     * @param {object} block
     * @param {{ audio?: object, lexicon?: object, vowelsOn?: boolean, lesson?: object }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'verse-reading') return '';
        var pointsOn = vowelsOn(options);
        var audio = options && options.audio;
        var lexicon = options && options.lexicon;
        var pressed = pointsOn ? 'true' : 'false';
        var toggle = pointsOn
            ? 'הסתר ניקוד (Hide vowels)'
            : 'הצג ניקוד (Show vowels)';
        return '<section data-block-type="verse-reading" data-block-id="' + text(block.id) + '"'
            + ' data-vowels-on="' + (pointsOn ? '1' : '0') + '" dir="rtl">'
            + renderRef(block.ref)
            + '<p class="dock"><button type="button" data-vowels-toggle aria-pressed="' + pressed + '">'
            + text(toggle) + '</button></p>'
            + renderNewWords(block.newWords, audio, pointsOn)
            + renderVerseLine(block.verse, lexicon, pointsOn)
            + renderQuestions(block.questions, pointsOn)
            + renderPersonal(block, options)
            + '</section>';
    }

    function hasClass(node, name) {
        if (!node) return false;
        if (node.classList && typeof node.classList.contains === 'function') {
            return node.classList.contains(name);
        }
        var cls = node.getAttribute ? (node.getAttribute('class') || '') : '';
        return (' ' + cls + ' ').indexOf(' ' + name + ' ') >= 0;
    }

    function applyVowels(section, pointsOn) {
        var nodes = section.querySelectorAll ? section.querySelectorAll('[data-vowels]') : [];
        for (var i = 0; i < nodes.length; i += 1) {
            var pointed = nodes[i].getAttribute('data-vowels') || '';
            nodes[i].textContent = pointsOn ? pointed : stripVowels(pointed);
        }
        section.setAttribute('data-vowels-on', pointsOn ? '1' : '0');
        var button = section.querySelector ? section.querySelector('[data-vowels-toggle]') : null;
        if (button) {
            button.setAttribute('aria-pressed', pointsOn ? 'true' : 'false');
            button.textContent = pointsOn
                ? 'הסתר ניקוד (Hide vowels)'
                : 'הצג ניקוד (Show vowels)';
        }
    }

    /**
     * Tap a known word to show its gloss. The vowels control rewrites text only.
     * @param {Document} doc
     * @returns {void}
     */
    function bind(doc) {
        if (!doc || typeof doc.querySelectorAll !== 'function') return;
        var sections = doc.querySelectorAll('[data-block-type="verse-reading"]');
        for (var i = 0; i < sections.length; i += 1) {
            var section = sections[i];
            if (!section || section.getAttribute('data-plan-bound') === '1') continue;
            if (typeof section.addEventListener !== 'function') continue;
            section.setAttribute('data-plan-bound', '1');
            section.addEventListener('click', function (event) {
                var target = event.target;
                if (!target || typeof target.closest !== 'function') return;
                var host = target.closest('[data-block-type="verse-reading"]');
                if (!host) return;
                var toggle = target.closest('[data-vowels-toggle]');
                if (toggle) {
                    applyVowels(host, host.getAttribute('data-vowels-on') !== '1');
                    return;
                }
                var word = target.closest('[data-verse-gloss]');
                if (!word) return;
                var gloss = word.nextElementSibling;
                if (!hasClass(gloss, 'verse-gloss')) return;
                gloss.hidden = !gloss.hidden;
            });
        }
    }

    return {
        bind: bind,
        escapeHtml: escapeHtml,
        render: render,
        stripVowels: stripVowels
    };
});
