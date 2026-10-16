/**
 * Explainer sections: headings, paragraphs, examples, a plain table, and
 * traffic-light callouts. A callout's tone is a word as well as a colour.
 * No pictures. Plan text is escaped.
 * Provenance: INIT-006/SPEC-014
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewBlockExplainer = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var AUDIO_FILE = /^(?:[A-Za-z0-9_-]+\/)*[A-Za-z0-9_.-]+\.mp3$/;
    var ROOT = /^[\u05D0-\u05EA](?:\.[\u05D0-\u05EA]){1,3}$/;
    var TONE_WORD = {
        green: { en: 'Go', he: 'קדימה' },
        yellow: { en: 'Check', he: 'בדקו' },
        red: { en: 'Stop', he: 'עצרו' },
        info: { en: 'Note', he: 'שימו לב' }
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

    function isPoint(ch) {
        var code = ch.charCodeAt(0);
        return code >= 0x0591 && code <= 0x05C7;
    }

    function isBase(ch) {
        var code = ch.charCodeAt(0);
        return code >= 0x05D0 && code <= 0x05EA;
    }

    /**
     * Wrap root letters, in order, ignoring points. Every character is escaped.
     * A root that is not the lexicon pattern is not applied.
     * @param {*} he
     * @param {*} root
     * @returns {string}
     */
    function highlightRoot(he, root) {
        var raw = he == null ? '' : String(he);
        if (raw.length > TEXT_LIMIT) raw = raw.slice(0, TEXT_LIMIT);
        var pattern = typeof root === 'string' && ROOT.test(root) ? root.split('.') : [];
        var want = 0;
        var chars = Array.from(raw);
        var out = '';
        for (var i = 0; i < chars.length; i += 1) {
            var ch = chars[i];
            if (want < pattern.length && isBase(ch) && ch === pattern[want]) {
                var chunk = ch;
                while (i + 1 < chars.length && isPoint(chars[i + 1])) {
                    i += 1;
                    chunk += chars[i];
                }
                out += '<span class="root-letter">' + escapeHtml(chunk) + '</span>';
                want += 1;
            } else {
                out += escapeHtml(ch);
            }
        }
        return out;
    }

    function list(value, max) {
        if (!Array.isArray(value)) return [];
        return value.slice(0, max);
    }

    function toneOf(value) {
        return Object.prototype.hasOwnProperty.call(TONE_WORD, value) ? value : '';
    }

    function clipButton(audio, key) {
        var path = audio && key != null ? audio[key] : '';
        if (typeof path !== 'string' || !AUDIO_FILE.test(path)) return '';
        return '<button type="button" class="audio" data-plan-audio="ready">'
            + 'השמעה <span lang="en" dir="ltr">Play</span></button>';
    }

    function renderExample(example, audio) {
        example = example || {};
        var he = example.he == null ? '' : String(example.he);
        var root = typeof example.root === 'string' ? example.root : '';
        var translit = example.translit
            ? '<span class="meaning" lang="en" dir="ltr">' + text(example.translit) + '</span>'
            : '';
        var rootText = root
            ? '<span class="meaning root-label" lang="he">' + text(root) + '</span>'
            : '';
        return '<li data-example-id="' + text(example.id) + '">'
            + '<p class="example" lang="he">' + highlightRoot(he, root) + '</p>'
            + '<p class="gloss"><span class="meaning" lang="en" dir="ltr">' + text(example.en) + '</span>'
            + translit + rootText + '</p>'
            + clipButton(audio, he)
            + '</li>';
    }

    function renderTable(table) {
        if (!table || typeof table !== 'object') return '';
        var columns = list(table.columns, 8);
        if (!columns.length) return '';
        var heads = columns.map(function (column) {
            return '<th scope="col">' + text(column) + '</th>';
        }).join('');
        var rows = list(table.rows, 12).map(function (row) {
            var cells = Array.isArray(row) ? row : [];
            var body = [];
            for (var i = 0; i < columns.length; i += 1) {
                body.push('<td>' + text(cells[i] == null ? '' : cells[i]) + '</td>');
            }
            return '<tr>' + body.join('') + '</tr>';
        }).join('');
        return '<table class="explainer-table"><thead><tr>' + heads + '</tr></thead><tbody>'
            + rows + '</tbody></table>';
    }

    function renderCallout(callout) {
        callout = callout || {};
        var tone = toneOf(callout.tone);
        var word = TONE_WORD[tone] || TONE_WORD.info;
        var toneAttr = tone || 'info';
        return '<aside class="callout" data-tone="' + toneAttr + '" data-callout-id="' + text(callout.id) + '">'
            + '<p class="kicker"><span class="callout-word" lang="en" dir="ltr">' + word.en + '</span> '
            + '<span lang="he">' + word.he + '</span></p>'
            + '<h3>' + text(callout.title) + '</h3>'
            + '<p>' + text(callout.text) + '</p>'
            + '</aside>';
    }

    /**
     * @param {object} block
     * @param {{ audio?: object }} [options]
     * @returns {string}
     */
    function render(block, options) {
        if (!block || block.type !== 'explainer') return '';
        var audio = options && options.audio;
        var sections = list(block.sections, 12).map(function (section) {
            section = section || {};
            var paragraphs = list(section.paragraphs, 8).map(function (paragraph) {
                return '<p>' + text(paragraph) + '</p>';
            }).join('');
            var examples = list(section.examples, 8).map(function (example) {
                return renderExample(example, audio);
            }).join('');
            var exampleList = examples ? '<ul class="explainer-examples">' + examples + '</ul>' : '';
            var callouts = list(section.callouts, 6).map(renderCallout).join('');
            return '<section data-section-id="' + text(section.id) + '">'
                + '<h3>' + text(section.heading) + '</h3>'
                + paragraphs
                + exampleList
                + renderTable(section.table)
                + callouts
                + '</section>';
        }).join('');
        return '<section data-block-type="explainer" data-block-id="' + text(block.id) + '">'
            + '<h2>' + text(block.title) + '</h2>'
            + sections
            + '</section>';
    }

    return {
        AUDIO_FILE: AUDIO_FILE,
        TONE_WORD: TONE_WORD,
        escapeHtml: escapeHtml,
        highlightRoot: highlightRoot,
        render: render,
        text: text
    };
});
