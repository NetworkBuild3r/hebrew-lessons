/**
 * Tap a Hebrew word to see what it means in English: the word, how it sounds,
 * a short meaning, then every sense by part of speech. Inside the plan editor
 * the card also shows the meaning the lesson saved, so a teacher can check the
 * two agree. The dictionary lives in the app (GET /api/dictionary/lookup).
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewWordLookup = api;
    if (typeof document !== 'undefined') api.bind(document);
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var WORD = /[\u05D0-\u05EA\u0591-\u05C7\u05BE\u05F3\u05F4]+/g;
    var LETTER = /[\u05D0-\u05EA]/;
    var SKIP = 'button, a, input, textarea, select, script, style, .lookup-word, .word-lookup-pop, .story-token, #story-text';
    var TIMEOUT_MS = 6000;
    var WORDS = {
        en: {
            close: 'Close',
            loading: 'Looking up…',
            none: 'No meaning found for this word.',
            failed: 'The meaning could not be loaded.',
            retry: 'Try again',
            dictionary: 'Dictionary',
            more: 'More meanings',
            less: 'Fewer meanings',
            from: 'Form of',
            seen: function (n) { return n === 1 ? 'In 1 lesson' : 'In ' + n + ' lessons'; }
        },
        he: {
            close: 'סגור',
            loading: 'מחפש…',
            none: 'לא נמצא פירוש למילה הזאת.',
            failed: 'לא ניתן לטעון את הפירוש.',
            retry: 'נסו שוב',
            dictionary: 'מילון',
            more: 'עוד פירושים',
            less: 'פחות פירושים',
            from: 'צורה של',
            seen: function (n) { return n === 1 ? 'בשיעור אחד' : 'ב-' + n + ' שיעורים'; }
        }
    };

    function words(doc) {
        var lang = doc && doc.documentElement ? doc.documentElement.getAttribute('data-ui-lang') : '';
        return lang === 'he' ? WORDS.he : WORDS.en;
    }

    /**
     * @param {string} text
     * @returns {{ type: 'text'|'word', value: string }[]}
     */
    function splitLookup(text) {
        var raw = String(text || '');
        var parts = [];
        var last = 0;
        var match;
        WORD.lastIndex = 0;
        while ((match = WORD.exec(raw))) {
            if (!LETTER.test(match[0])) continue;
            if (match.index > last) parts.push({ type: 'text', value: raw.slice(last, match.index) });
            parts.push({ type: 'word', value: match[0] });
            last = match.index + match[0].length;
        }
        if (last < raw.length) parts.push({ type: 'text', value: raw.slice(last) });
        return parts;
    }

    /**
     * Ask the app for a word's meanings.
     * @param {string} word
     * @returns {Promise<{ ok: boolean, short: string, form: string|null, meanings: { pos: string, en: string }[] }>}
     */
    function fetchWord(word) {
        var api = root.HebrewApi;
        var empty = { ok: false, short: '', form: null, meanings: [] };
        var q = String(word || '').trim();
        if (!api || typeof api.get !== 'function' || !q) return Promise.resolve(empty);
        // Never wait forever: a slow or stuck answer is shown as "could not load".
        var timer;
        var limit = new Promise(function (resolve) {
            timer = setTimeout(function () { resolve(empty); }, TIMEOUT_MS);
        });
        var asked;
        try {
            asked = Promise.resolve(api.get('/api/dictionary/lookup?word=' + encodeURIComponent(q), { redirectOn401: false }))
                .then(function (res) {
                    if (!res || !res.ok || !res.data) return empty;
                    return {
                        ok: true,
                        short: typeof res.data.short === 'string' ? res.data.short : '',
                        form: typeof res.data.form === 'string' ? res.data.form : null,
                        meanings: Array.isArray(res.data.meanings) ? res.data.meanings : []
                    };
                })
                .catch(function () { return empty; });
        } catch (err) {
            asked = Promise.resolve(empty);
        }
        return Promise.race([asked, limit]).then(function (found) {
            clearTimeout(timer);
            return found;
        });
    }

    /**
     * One short English line for a word, or '' when none is known.
     * @param {string} word
     * @returns {Promise<string>}
     */
    function meanings(word) {
        return fetchWord(word).then(function (found) { return found.short || ''; });
    }

    /**
     * What the open lesson plan saved for this word, if the editor is mounted.
     * @param {string} word
     * @returns {{ meaning: string, translit: string }|null}
     */
    function savedFacts(word) {
        try {
            var builder = root.HebrewPlanBuilder;
            if (builder && typeof builder.wordFacts === 'function') return builder.wordFacts(word);
        } catch (err) { /* the card works without it */ }
        return null;
    }

    function dismiss(doc) {
        var pop = doc && doc.querySelector && doc.querySelector('.word-lookup-pop');
        if (pop && pop.parentNode) pop.parentNode.removeChild(pop);
        if (!doc || !doc.querySelectorAll) return;
        var open = doc.querySelectorAll('.lookup-word[aria-expanded="true"]');
        var i;
        for (i = 0; i < open.length; i++) open[i].setAttribute('aria-expanded', 'false');
    }

    function place(pop, anchor) {
        var rect = anchor.getBoundingClientRect();
        pop.style.top = (rect.bottom + 6) + 'px';
        pop.style.left = rect.left + 'px';
        var box = pop.getBoundingClientRect();
        if (box.right > window.innerWidth - 8) {
            pop.style.left = Math.max(8, window.innerWidth - box.width - 8) + 'px';
        }
        if (box.bottom > window.innerHeight - 8) {
            pop.style.top = Math.max(8, rect.top - box.height - 6) + 'px';
        }
    }

    function part(doc, tag, className, text, lang) {
        var node = doc.createElement(tag);
        if (className) node.className = className;
        if (lang) {
            node.lang = lang;
            node.setAttribute('dir', lang === 'he' ? 'rtl' : 'ltr');
        }
        if (text != null) node.textContent = text;
        return node;
    }

    /**
     * Fill the card body. The meaning this lesson saved comes first, then the
     * dictionary's short meaning, then every sense behind a "More meanings" step.
     * @param {Document} doc
     * @param {HTMLElement} body
     * @param {object} found
     * @param {object|null} saved
     * @param {{ retry: Function }} hooks
     */
    function paint(doc, body, found, saved, hooks) {
        var text = words(doc);
        body.textContent = '';
        var mine = saved && saved.meaning ? saved.meaning : '';
        var headline = mine || found.short || '';
        if (headline) body.appendChild(part(doc, 'p', 'word-lookup-short', headline, 'en'));
        if (mine && found.short && found.short.toLowerCase() !== mine.toLowerCase()) {
            var dict = part(doc, 'p', 'word-lookup-note', text.dictionary + ': ');
            dict.appendChild(part(doc, 'span', '', found.short, 'en'));
            body.appendChild(dict);
        }
        if (!mine && found.form) {
            var form = part(doc, 'p', 'word-lookup-form', text.from + ' ');
            form.appendChild(part(doc, 'span', '', found.form, 'he'));
            body.appendChild(form);
        }
        if (saved && saved.lessons > 0) {
            body.appendChild(part(doc, 'p', 'word-lookup-seen', text.seen(saved.lessons)));
        }
        if (found.meanings.length) {
            var list = part(doc, 'ul', 'word-lookup-senses');
            list.hidden = true;
            found.meanings.forEach(function (sense) {
                var item = part(doc, 'li', '');
                if (sense.pos) item.appendChild(part(doc, 'span', 'word-lookup-pos', sense.pos, 'en'));
                item.appendChild(part(doc, 'span', 'word-lookup-sense', sense.en, 'en'));
                list.appendChild(item);
            });
            var more = part(doc, 'button', 'btn btn-ghost btn-sm word-lookup-more', text.more);
            more.type = 'button';
            more.setAttribute('aria-expanded', 'false');
            more.addEventListener('click', function () {
                list.hidden = !list.hidden;
                more.setAttribute('aria-expanded', list.hidden ? 'false' : 'true');
                more.textContent = list.hidden ? text.more : text.less;
            });
            body.appendChild(more);
            body.appendChild(list);
        }
        if (!found.ok) {
            if (!headline) body.appendChild(part(doc, 'p', 'word-lookup-note', text.failed, 'en'));
            var again = part(doc, 'button', 'btn btn-ghost btn-sm', text.retry);
            again.type = 'button';
            again.addEventListener('click', hooks.retry);
            body.appendChild(again);
        } else if (!headline && !found.meanings.length) {
            body.appendChild(part(doc, 'p', 'word-lookup-note', text.none, 'en'));
        }
    }

    function open(doc, anchor, word) {
        dismiss(doc);
        anchor.setAttribute('aria-expanded', 'true');
        var text = words(doc);
        var saved = savedFacts(word);
        var pop = part(doc, 'div', 'word-lookup-pop');
        pop.setAttribute('role', 'dialog');
        pop.setAttribute('aria-label', word);
        // The card follows the interface language, not whatever direction the page behind it has.
        var hebrewUi = doc.documentElement && doc.documentElement.getAttribute('data-ui-lang') === 'he';
        pop.setAttribute('dir', hebrewUi ? 'rtl' : 'ltr');
        pop.setAttribute('lang', hebrewUi ? 'he' : 'en');
        // A new word is shown with its vowel points when the lesson has them.
        var shown = saved && saved.pointed ? saved.pointed : word;
        pop.appendChild(part(doc, 'p', 'story-gloss-he', shown, 'he'));
        if (saved && saved.translit) pop.appendChild(part(doc, 'p', 'word-lookup-sound', saved.translit, 'en'));
        var body = part(doc, 'div', 'word-lookup-body');
        pop.appendChild(body);
        var close = part(doc, 'button', 'btn btn-ghost btn-sm', text.close);
        close.type = 'button';
        close.setAttribute('data-lookup-close', '');
        pop.appendChild(close);
        doc.body.appendChild(pop);
        place(pop, anchor);

        function load() {
            body.textContent = '';
            if (saved && saved.meaning) {
                // The lesson's own meaning is already known: show it while the dictionary answers.
                paint(doc, body, { ok: true, short: '', form: null, meanings: [] }, saved, { retry: load });
            } else {
                body.appendChild(part(doc, 'p', 'word-lookup-note', text.loading, 'en'));
            }
            fetchWord(word).then(function (found) {
                if (!pop.parentNode) return;
                paint(doc, body, found, saved, { retry: load });
                place(pop, anchor);
            }).catch(function () {
                if (!pop.parentNode) return;
                body.textContent = '';
                body.appendChild(part(doc, 'p', 'word-lookup-note', text.failed, 'en'));
            });
        }
        load();
    }

    function onClick(ev) {
        var doc = ev.currentTarget && ev.currentTarget.nodeType === 9 ? ev.currentTarget : document;
        var target = ev.target;
        if (!target || !target.closest) return;
        if (target.closest('[data-lookup-close]')) {
            dismiss(doc);
            return;
        }
        if (target.closest('.word-lookup-pop')) return;
        var from = target.closest('[data-lookup-from]');
        if (from) {
            var row = from.closest('li') || from.parentNode;
            var field = row && row.querySelector ? row.querySelector('input, textarea') : null;
            var typed = field ? String(field.value || '').trim() : '';
            if (!typed) return;
            ev.preventDefault();
            open(doc, from, typed);
            return;
        }
        var btn = target.closest('[data-lookup-word]');
        if (!btn) {
            dismiss(doc);
            return;
        }
        var word = btn.getAttribute('data-lookup-word') || btn.textContent || '';
        if (!word) return;
        ev.preventDefault();
        open(doc, btn, word);
    }

    function wrapRoot(el) {
        if (!el || !el.ownerDocument || typeof el.ownerDocument.createTreeWalker !== 'function') return;
        var doc = el.ownerDocument;
        var walker = doc.createTreeWalker(el, 4, null);
        var nodes = [];
        var node = walker.nextNode();
        while (node) {
            if (node.nodeValue && LETTER.test(node.nodeValue)) {
                var parent = node.parentElement;
                if (parent && !parent.closest(SKIP)) nodes.push(node);
            }
            node = walker.nextNode();
        }
        nodes.forEach(function (textNode) {
            var parts = splitLookup(textNode.nodeValue);
            if (!parts.some(function (piece) { return piece.type === 'word'; })) return;
            var frag = doc.createDocumentFragment();
            parts.forEach(function (piece) {
                if (piece.type === 'text') {
                    frag.appendChild(doc.createTextNode(piece.value));
                    return;
                }
                var button = doc.createElement('button');
                button.type = 'button';
                button.className = 'lookup-word';
                button.setAttribute('data-lookup-word', piece.value);
                button.textContent = piece.value;
                frag.appendChild(button);
            });
            if (textNode.parentNode) textNode.parentNode.replaceChild(frag, textNode);
        });
    }

    function scan(doc) {
        if (!doc || !doc.querySelectorAll) return;
        var nodes = doc.querySelectorAll('#app, #plan-root, [data-plan-preview]');
        var i;
        for (i = 0; i < nodes.length; i++) wrapRoot(nodes[i]);
    }

    function bind(doc) {
        if (!doc || doc.__hebrewLookup || typeof doc.addEventListener !== 'function') return;
        doc.__hebrewLookup = true;
        doc.addEventListener('click', onClick);
        doc.addEventListener('keydown', function (ev) {
            if (ev.key === 'Escape') dismiss(doc);
        });
        function arm() {
            scan(doc);
            if (typeof MutationObserver !== 'function' || !doc.body) return;
            var obs = new MutationObserver(function () {
                obs.disconnect();
                scan(doc);
                if (doc.body) obs.observe(doc.body, { childList: true, subtree: true });
            });
            obs.observe(doc.body, { childList: true, subtree: true });
        }
        if (doc.readyState === 'loading') doc.addEventListener('DOMContentLoaded', arm);
        else arm();
    }

    return {
        bind: bind,
        fetchWord: fetchWord,
        meanings: meanings,
        splitLookup: splitLookup
    };
});
