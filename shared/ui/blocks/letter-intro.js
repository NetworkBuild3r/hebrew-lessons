/**
 * One letter: the letter large, its name, the sound with each vowel,
 * and which words contain the letter. Writing the letter is a paper prompt.
 * There is no picture field (ADR-0009 §7, Pictures). Nothing here is scored.
 * Plan text is escaped.
 * Provenance: INIT-006/SPEC-016
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockLetter = api;
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

    function list(value, max) {
        if (!Array.isArray(value)) return [];
        return value.slice(0, max);
    }

    function isHebrew(value) {
        return /[\u0590-\u05FF]/.test(String(value || ''));
    }

    function clipButton(audio, key) {
        var path = audio && key != null ? audio[key] : '';
        if (typeof path !== 'string' || !AUDIO_FILE.test(path)) return '';
        return '<button type="button" class="audio" data-plan-audio="ready">'
            + 'השמעה <span lang="en" dir="ltr">Play</span></button>';
    }

    function renderSounds(sounds, audio) {
        var items = list(sounds, 16).map(function (sound) {
            if (!sound || typeof sound !== 'object') return '';
            var pointed = cap(sound.pointed);
            var soundText = sound.sound == null ? '' : String(sound.sound);
            var soundDir = isHebrew(soundText) ? 'rtl' : 'ltr';
            var soundLang = soundDir === 'rtl' ? 'he' : 'en';
            return '<li data-sound-id="' + text(sound.id) + '">'
                + '<span lang="he" dir="rtl">' + text(sound.vowel) + '</span> '
                + '<span class="pointed-line" lang="he" dir="rtl" data-vowels="' + escapeHtml(pointed) + '">'
                + text(pointed) + '</span> '
                + '<span lang="' + soundLang + '" dir="' + soundDir + '">' + text(soundText) + '</span> '
                + clipButton(audio, pointed)
                + '</li>';
        }).join('');
        return '<ul class="letter-sounds" data-letter-part="sounds">' + items + '</ul>';
    }

    function renderFind(words) {
        var items = list(words, 12).map(function (item) {
            if (!item || typeof item !== 'object') return '';
            var yes = item.contains === true ? 'yes' : 'no';
            return '<li><button type="button" data-find-choice data-contains="' + yes + '"'
                + ' lang="he" dir="rtl">' + text(item.word) + '</button></li>';
        }).join('');
        return '<fieldset data-letter-part="find">'
            + '<legend>איזו מילה מכילה את האות? '
            + '<span lang="en" dir="ltr">Which word contains the letter?</span></legend>'
            + '<ul class="recycle">' + items + '</ul>'
            + '<p class="gloss" data-find-feedback hidden></p>'
            + '</fieldset>';
    }

    /**
     * @param {object} block
     * @param {{ audio?: object }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'letter-intro') return '';
        var audio = options && options.audio;
        return '<section data-block-type="letter-intro" data-block-id="' + text(block.id) + '" dir="rtl">'
            + '<p class="letter-hero" lang="he" dir="rtl" data-vowels="' + escapeHtml(cap(block.letter)) + '">'
            + text(block.letter) + '</p>'
            + '<p class="letter-name" lang="he" dir="rtl">' + text(block.name) + '</p>'
            + renderSounds(block.sounds, audio)
            + renderFind(block.findIn)
            + '<p data-paper-practice>כתבו את האות על הנייר. '
            + '<span lang="en" dir="ltr">Write the letter on paper. This is not scored.</span></p>'
            + '</section>';
    }

    /**
     * Tap a word to hear whether it contains the letter. Nothing is posted.
     * @param {Document} doc
     * @returns {void}
     */
    function bind(doc) {
        if (!doc || typeof doc.querySelectorAll !== 'function') return;
        var sections = doc.querySelectorAll('[data-block-type="letter-intro"]');
        for (var i = 0; i < sections.length; i += 1) {
            var section = sections[i];
            if (!section || section.getAttribute('data-plan-bound') === '1') continue;
            if (typeof section.addEventListener !== 'function') continue;
            section.setAttribute('data-plan-bound', '1');
            section.addEventListener('click', function (event) {
                var target = event.target;
                if (!target || typeof target.closest !== 'function') return;
                var choice = target.closest('[data-find-choice]');
                if (!choice) return;
                var host = choice.closest('[data-block-type="letter-intro"]');
                if (!host || typeof host.querySelector !== 'function') return;
                var note = host.querySelector('[data-find-feedback]');
                if (!note) return;
                var yes = choice.getAttribute('data-contains') === 'yes';
                note.hidden = false;
                note.textContent = yes
                    ? 'כן. המילה מכילה את האות. / Yes. This word contains the letter.'
                    : 'לא. המילה לא מכילה את האות. / No. This word does not contain the letter.';
            });
        }
    }

    return {
        bind: bind,
        escapeHtml: escapeHtml,
        render: render
    };
});
