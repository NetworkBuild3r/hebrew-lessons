/**
 * Verb practice: one card at a time, verbs and tenses mixed, about three minutes a round.
 * Each form climbs a ladder: pick it, then type it, then known and spaced across days.
 * A first miss gets a pattern hint and one more try; a second miss shows the form and
 * brings it back later in the round. The verb's table stays hidden until the learner answers.
 * A draft (unverified) form may be practised but never sends a form signal.
 * Provenance: INIT-013 follow-up (verb practice redesign); research rules R1-R4 in INIT-013 §1b.
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewVerbPractice = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var ROUND = 12;
    var NEW_PER_ROUND = 3;
    var DAY = 86400000;
    // Days until a form is due again, by ladder level. 0 and 1 are "still learning": due at once.
    var INTERVAL_DAYS = [0, 0, 1, 3, 7, 16];
    var MAX_LEVEL = INTERVAL_DAYS.length - 1;
    var STORE_PREFIX = 'hebrew-verb-ladder';
    var STORE_LIMIT = 3000;
    var TEXT_LIMIT = 400;
    var MARKS = /[\u0591-\u05C7]/g;
    var IS_MARK = /[\u0591-\u05C7]/;
    var LETTERS = 'א ב ג ד ה ו ז ח ט י כ ך ל מ ם נ ן ס ע פ ף צ ץ ק ר ש ת'.split(' ');
    var FINAL = { 'ך': 'כ', 'ם': 'מ', 'ן': 'נ', 'ף': 'פ', 'ץ': 'צ' };

    var TENSES = ['past', 'present', 'future'];
    var PERSON_CELLS = ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'];
    var CELLS = { past: PERSON_CELLS, present: ['ms', 'fs', 'mp', 'fp'], future: PERSON_CELLS };

    var TENSE = {
        past: { he: 'עָבָר', en: 'Past', anchor: 'אֶתְמוֹל', anchorEn: 'yesterday',
            note: 'Endings mark the person: ־תִי I, ־נוּ we, ־וּ they.' },
        present: { he: 'הוֹוֶה', en: 'Present', anchor: 'עַכְשָׁיו', anchorEn: 'now',
            note: 'Four forms: masculine and feminine, singular and plural (־ִים, ־וֹת).' },
        future: { he: 'עָתִיד', en: 'Future', anchor: 'מָחָר', anchorEn: 'tomorrow',
            note: 'Prefixes mark the person: א I, ת you and she, י he and they, נ we.' }
    };

    var PERSON = {
        ms: ['הוּא', 'm. sg.'], fs: ['הִיא', 'f. sg.'], mp: ['הֵם', 'm. pl.'], fp: ['הֵן', 'f. pl.'],
        '1s': ['אֲנִי', 'I'], '2ms': ['אַתָּה', 'you (m.)'], '2fs': ['אַתְּ', 'you (f.)'],
        '3ms': ['הוּא', 'he'], '3fs': ['הִיא', 'she'], '1p': ['אֲנַחְנוּ', 'we'],
        '2mp': ['אַתֶּם', 'you (m. pl.)'], '2fp': ['אַתֶּן', 'you (f. pl.)'], '3p': ['הֵם', 'they']
    };

    var HINT = {
        past: {
            '1s': 'I: the past ending is ־תִי.', '2ms': 'you (m.): the ending is ־תָ.',
            '2fs': 'you (f.): the ending is ־תְ.', '3ms': 'he: the bare past stem, no ending.',
            '3fs': 'she: the ending is ־ָה.', '1p': 'we: the ending is ־נוּ.',
            '2mp': 'you (m. pl.): the ending is ־תֶם.', '2fp': 'you (f. pl.): the ending is ־תֶן.',
            '3p': 'they: the ending is ־וּ.'
        },
        present: {
            ms: 'The base present form.', fs: 'Feminine: ends in ־ָה or ־ֶת.',
            mp: 'Masculine plural: ends in ־ִים.', fp: 'Feminine plural: ends in ־וֹת.'
        },
        future: {
            '1s': 'I: starts with א.', '2ms': 'you (m.): starts with ת.',
            '2fs': 'you (f.): starts with ת and ends in ־ִי.', '3ms': 'he: starts with י.',
            '3fs': 'she: starts with ת, the same as you (m.).', '1p': 'we: starts with נ.',
            '2mp': 'you (m. pl.): ת at the start, ־וּ at the end.',
            '2fp': 'you (f. pl.): ת at the start, ־נָה at the end.',
            '3p': 'they: י at the start, ־וּ at the end.'
        }
    };

    // Base letters of the person ending. Skipped when marking root letters, so a final ן in ־תֶן
    // is not taken for a root nun.
    var SUFFIX = {
        past: { '1s': 2, '2ms': 1, '2fs': 1, '3ms': 0, '3fs': 1, '1p': 2, '2mp': 2, '2fp': 2, '3p': 1 },
        present: { ms: 0, fs: 1, mp: 2, fp: 2 },
        future: { '1s': 0, '2ms': 0, '2fs': 1, '3ms': 0, '3fs': 0, '1p': 0, '2mp': 1, '2fp': 2, '3p': 1 }
    };

    var BINYAN = {
        paal: ["pa'al", 'פָּעַל', "Pa'al present puts וֹ after the first root letter."],
        piel: ["pi'el", 'פִּיעֵל', "Pi'el present starts with מְ."],
        hifil: ["hif'il", 'הִפְעִיל', "Hif'il present starts with מַ."],
        nifal: ["nif'al", 'נִפְעַל', "Nif'al present starts with נִ."],
        hitpael: ["hitpa'el", 'הִתְפַּעֵל', "Hitpa'el present starts with מִתְ."],
        pual: ["pu'al", 'פֻּעַל', "Pu'al present starts with מְ."],
        hufal: ["huf'al", 'הֻפְעַל', "Huf'al present starts with מָ."]
    };

    // ---- small helpers ----

    function str(value) {
        var raw = value == null ? '' : String(value);
        return raw.length > TEXT_LIMIT ? raw.slice(0, TEXT_LIMIT) : raw;
    }

    function esc(value) {
        var U = root.HebrewUtils;
        if (U && typeof U.escapeHtml === 'function') return U.escapeHtml(str(value));
        return str(value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function bi(he, en) {
        var U = root.HebrewUtils;
        if (U && typeof U.bi === 'function') return U.bi(he, en);
        return '<span class="ui-pair"><span class="ui-he" lang="he">' + esc(he) + '</span>'
            + '<span class="ui-en" lang="en" dir="ltr">' + esc(en) + '</span></span>';
    }

    function he(text, cls) {
        return '<span' + (cls ? ' class="' + cls + '"' : '') + ' lang="he" dir="rtl">' + esc(text) + '</span>';
    }

    function en(text, cls) {
        return '<span' + (cls ? ' class="' + cls + '"' : '') + ' lang="en" dir="ltr">' + esc(text) + '</span>';
    }

    /** Letters only: no vowel points, no spaces. */
    function plain(value) {
        return String(value == null ? '' : value).replace(MARKS, '').normalize('NFC').replace(/\s+/g, '');
    }

    /**
     * The form as a learner sees it. A lexicon key may carry a cell tag to stay unique
     * (future she has the same spelling as future you m.), e.g. "תַּחְלִיט-3fs".
     */
    function displayKey(key) {
        return String(key == null ? '' : key).replace(/-[0-9a-z]+$/i, '');
    }

    function isVerified(form) {
        return !!(form && (form.verified === true || form.verified === 1));
    }

    function binyanLabel(binyan) {
        var row = BINYAN[binyan];
        return row ? row[0] : str(binyan);
    }

    function shuffle(list, rng) {
        var random = typeof rng === 'function' ? rng : Math.random;
        var next = list.slice();
        for (var i = next.length - 1; i > 0; i -= 1) {
            var j = Math.floor(random() * (i + 1));
            if (j < 0 || j > i) j = 0;
            var tmp = next[i];
            next[i] = next[j];
            next[j] = tmp;
        }
        return next;
    }

    // ---- verb model ----

    function formEntry(verb, tense, cell, key, row) {
        return {
            id: key,
            key: key,
            he: displayKey(key),
            lemma: verb.lemma,
            tense: tense,
            cell: cell,
            gloss: str(row.gloss_en),
            accept: Array.isArray(row.accept) ? row.accept.slice(0, 8).map(str) : [],
            verified: isVerified(row),
            root: str(row.root) || verb.root
        };
    }

    /**
     * Verbs named by a lesson's conjugation tables (study or drill), in the order they first appear.
     * Forms come from the lexicon paradigm of each verb, so every tense is practised even when the
     * lesson shows one table per verb; a table cell fills a gap the paradigm leaves.
     * Only forms that are in the lexicon are kept: a form the app cannot check is never asked.
     * @param {object[]} blocks
     * @param {object} forms formKey -> lexicon form row
     * @param {object} [lemmas] lemmaKey -> lexicon lemma row
     * @param {object} [paradigms] lemmaKey -> { tense: { cell: form row } }
     * @returns {object[]}
     */
    function collectVerbs(blocks, forms, lemmas, paradigms) {
        var order = [];
        var byLemma = Object.create(null);
        (Array.isArray(blocks) ? blocks : []).forEach(function (block) {
            if (!block || block.type !== 'conjugation-table') return;
            var tense = block.tense;
            if (TENSES.indexOf(tense) < 0) return;
            var lemma = typeof block.lemmaKey === 'string' ? block.lemmaKey : '';
            if (!lemma) return;
            var verb = byLemma[lemma];
            if (!verb) {
                var entry = (lemmas && lemmas[lemma]) || {};
                verb = {
                    lemma: lemma,
                    gloss: str(entry.gloss_en),
                    binyan: str(entry.binyan),
                    root: str(entry.root),
                    tenses: { past: Object.create(null), present: Object.create(null), future: Object.create(null) },
                    lines: [],
                    cells: []
                };
                byLemma[lemma] = verb;
                order.push(verb);
            }
            var cells = Array.isArray(block.cells) ? block.cells.slice(0, 9) : [];
            cells.forEach(function (entryCell) {
                if (!entryCell || typeof entryCell.formKey !== 'string') return;
                if (CELLS[tense].indexOf(entryCell.cell) < 0) return;
                var row = forms && forms[entryCell.formKey];
                if (!row || typeof row !== 'object') return;
                verb.cells.push({ tense: tense, cell: entryCell.cell, key: entryCell.formKey, row: row });
            });
            ['sentences', 'questions'].forEach(function (field) {
                var list = Array.isArray(block[field]) ? block[field] : [];
                list.slice(0, 4).forEach(function (line) {
                    if (!line || typeof line.he !== 'string' || verb.lines.length >= 6) return;
                    verb.lines.push({ tense: tense, he: str(line.he), en: str(line.en) });
                });
            });
        });
        order.forEach(function (verb) {
            var paradigm = paradigms && paradigms[verb.lemma];
            TENSES.forEach(function (tense) {
                var cells = paradigm && paradigm[tense] && typeof paradigm[tense] === 'object' ? paradigm[tense] : {};
                CELLS[tense].forEach(function (cell) {
                    var row = cells[cell];
                    if (!row || typeof row !== 'object' || typeof row.key !== 'string') return;
                    if (!verb.root && typeof row.root === 'string') verb.root = row.root;
                    if (!verb.binyan && typeof row.binyan === 'string') verb.binyan = row.binyan;
                    verb.tenses[tense][cell] = formEntry(verb, tense, cell, row.key, row);
                });
            });
            verb.cells.forEach(function (c) {
                if (verb.tenses[c.tense][c.cell]) return;
                if (!verb.root && typeof c.row.root === 'string') verb.root = c.row.root;
                if (!verb.binyan && typeof c.row.binyan === 'string') verb.binyan = c.row.binyan;
                verb.tenses[c.tense][c.cell] = formEntry(verb, c.tense, c.cell, c.key, c.row);
            });
            delete verb.cells;
        });
        return order.filter(function (verb) {
            return allForms([verb]).length > 0;
        });
    }

    /** Every form of the given verbs, tense by tense in table order. */
    function allForms(verbs) {
        var out = [];
        (verbs || []).forEach(function (verb) {
            TENSES.forEach(function (tense) {
                CELLS[tense].forEach(function (cell) {
                    var form = verb.tenses[tense][cell];
                    if (form) out.push(form);
                });
            });
        });
        return out;
    }

    /**
     * @param {object[]} verbs
     * @param {{ verbs?: string[], tenses?: string[], allowDraft?: boolean }} [settings]
     * @returns {object[]}
     */
    function poolOf(verbs, settings) {
        var s = settings || {};
        return allForms(verbs).filter(function (form) {
            if (s.verbs && s.verbs.indexOf(form.lemma) < 0) return false;
            if (s.tenses && s.tenses.indexOf(form.tense) < 0) return false;
            if (!form.verified && s.allowDraft === false) return false;
            return true;
        });
    }

    function verbOf(verbs, lemma) {
        for (var i = 0; i < (verbs || []).length; i += 1) {
            if (verbs[i].lemma === lemma) return verbs[i];
        }
        return null;
    }

    function hasDraft(verbs) {
        return allForms(verbs).some(function (form) { return !form.verified; });
    }

    // ---- ladder (per form, saved on this device) ----

    function storageKey(user) {
        var name = user && (user.username || user.id);
        return STORE_PREFIX + (name != null && name !== '' ? ':' + String(name).slice(0, 64) : '');
    }

    function cleanEntry(raw) {
        if (!raw || typeof raw !== 'object') return null;
        var l = Math.floor(Number(raw.l));
        if (!(l >= 0)) l = 0;
        if (l > MAX_LEVEL) l = MAX_LEVEL;
        return {
            l: l,
            d: Number(raw.d) || 0,
            r: Math.max(0, Math.floor(Number(raw.r) || 0)),
            w: Math.max(0, Math.floor(Number(raw.w) || 0)),
            s: Number(raw.s) || 0
        };
    }

    function loadLadder(storage, user) {
        var out = Object.create(null);
        try {
            var raw = storage && typeof storage.getItem === 'function' ? storage.getItem(storageKey(user)) : null;
            var parsed = raw ? JSON.parse(raw) : null;
            if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return out;
            Object.keys(parsed).slice(0, STORE_LIMIT).forEach(function (id) {
                var entry = cleanEntry(parsed[id]);
                if (entry && id.length <= 80) out[id] = entry;
            });
        } catch (err) { /* private mode or bad JSON: start fresh */ }
        return out;
    }

    function saveLadder(storage, user, ladder) {
        try {
            if (!storage || typeof storage.setItem !== 'function') return;
            var ids = Object.keys(ladder || {});
            if (ids.length > STORE_LIMIT) {
                ids.sort(function (a, b) { return (ladder[b].s || 0) - (ladder[a].s || 0); });
                ids = ids.slice(0, STORE_LIMIT);
            }
            var out = {};
            ids.forEach(function (id) { out[id] = ladder[id]; });
            storage.setItem(storageKey(user), JSON.stringify(out));
        } catch (err) { /* quota: practice still works for this visit */ }
    }

    function entryOf(ladder, id) {
        return ladder && Object.prototype.hasOwnProperty.call(ladder, id) ? ladder[id] : null;
    }

    /** Never answered right: pick it from four. Answered right once: type it. */
    function modeFor(entry, again) {
        return again || (entry && entry.l >= 1) ? 'type' : 'recognize';
    }

    /**
     * @param {object} ladder
     * @param {string} id
     * @param {'recognize'|'type'} mode
     * @param {'first'|'retry'|'miss'} outcome
     * @param {number} now
     * @returns {object} a new ladder
     */
    function applyResult(ladder, id, mode, outcome, now) {
        var prev = cleanEntry(entryOf(ladder, id)) || { l: 0, d: 0, r: 0, w: 0, s: 0 };
        var level = prev.l;
        if (outcome === 'miss') level = level >= 3 ? 1 : 0;
        else if (mode === 'recognize') level = Math.max(level, 1);
        else if (outcome === 'first') level = Math.min(MAX_LEVEL, Math.max(level + 1, 2));
        else level = Math.max(level, 1);
        var next = Object.create(null);
        Object.keys(ladder || {}).forEach(function (key) { next[key] = ladder[key]; });
        next[id] = {
            l: level,
            d: now + INTERVAL_DAYS[level] * DAY,
            r: prev.r + (outcome === 'miss' ? 0 : 1),
            w: prev.w + (outcome === 'miss' ? 1 : 0),
            s: now
        };
        return next;
    }

    /** Counts for Today: due now, known (typed from memory), and seen at all. */
    function dueSummary(ladder, now, ids) {
        var due = 0;
        var known = 0;
        var seen = 0;
        var keys = Array.isArray(ids) ? ids : Object.keys(ladder || {});
        keys.forEach(function (id) {
            var entry = entryOf(ladder, id);
            if (!entry) return;
            seen += 1;
            if (entry.l >= 2) known += 1;
            if (entry.d <= now) due += 1;
        });
        return { due: due, known: known, seen: seen };
    }

    // ---- rounds ----

    /** No verb twice in a row; change tense too when another card allows it. */
    function interleave(list) {
        var rest = list.slice();
        var out = [];
        function find(pred) {
            for (var i = 0; i < rest.length; i += 1) if (pred(rest[i])) return i;
            return -1;
        }
        while (rest.length) {
            var last = out[out.length - 1];
            var i = find(function (f) { return !last || (f.lemma !== last.lemma && f.tense !== last.tense); });
            if (i < 0) i = find(function (f) { return !last || f.lemma !== last.lemma; });
            if (i < 0) i = 0;
            out.push(rest.splice(i, 1)[0]);
        }
        return out;
    }

    /**
     * Due forms first (weakest and most overdue first), a few new ones, then the rest.
     * @param {object[]} pool
     * @param {object} ladder
     * @param {number} now
     * @param {function(): number} [rng]
     * @param {number} [size]
     * @returns {object[]}
     */
    function buildRound(pool, ladder, now, rng, size) {
        var n = size || ROUND;
        var random = typeof rng === 'function' ? rng : Math.random;
        var due = [];
        var fresh = [];
        var later = [];
        (pool || []).forEach(function (form) {
            var entry = entryOf(ladder, form.id);
            if (!entry) fresh.push(form);
            else if (entry.d <= now) due.push({ form: form, k: entry.l * 10 - Math.min(9, (now - entry.d) / DAY) + random() });
            else later.push({ form: form, k: entry.d + random() * DAY });
        });
        due.sort(function (a, b) { return a.k - b.k; });
        later.sort(function (a, b) { return a.k - b.k; });
        fresh = shuffle(fresh, random);
        var takeNew = Math.min(fresh.length, due.length >= n ? NEW_PER_ROUND : n - due.length);
        var picked = due.slice(0, n - takeNew).map(function (x) { return x.form; })
            .concat(fresh.slice(0, takeNew));
        if (picked.length < n) {
            picked = picked.concat(fresh.slice(takeNew, takeNew + n - picked.length));
        }
        if (picked.length < n) {
            picked = picked.concat(later.slice(0, n - picked.length).map(function (x) { return x.form; }));
        }
        return interleave(shuffle(picked, random));
    }

    /** The answer plus three forms of the same verb: two from the same tense, one from another. */
    function optionsFor(form, verb, rng) {
        var seen = Object.create(null);
        seen[plain(form.he)] = true;
        var forms = allForms(verb ? [verb] : []);
        function pick(list, count) {
            var out = [];
            shuffle(list, rng).forEach(function (other) {
                if (out.length >= count) return;
                var p = plain(other.he);
                if (seen[p]) return;
                seen[p] = true;
                out.push(other.he);
            });
            return out;
        }
        var sameTense = forms.filter(function (f) { return f.tense === form.tense; });
        var same = pick(sameTense, 2);
        var other = pick(forms.filter(function (f) { return f.tense !== form.tense; }), 3 - same.length);
        var more = pick(sameTense, 3 - same.length - other.length);
        return shuffle([form.he].concat(same, other, more), rng);
    }

    /**
     * Letters only: vowel points are optional and spelling variants in `accept` count.
     * @returns {boolean|null} null when nothing was typed
     */
    function isMatch(typed, form) {
        var got = plain(typed);
        if (!got) return null;
        if (got === plain(form.he)) return true;
        var accept = Array.isArray(form.accept) ? form.accept : [];
        for (var i = 0; i < accept.length; i += 1) {
            if (got === plain(accept[i])) return true;
        }
        return false;
    }

    function makeItem(form, verbs, ladder, rng, again) {
        var mode = modeFor(entryOf(ladder, form.id), again);
        return {
            form: form,
            mode: mode,
            again: !!again,
            options: mode === 'recognize' ? optionsFor(form, verbOf(verbs, form.lemma), rng) : []
        };
    }

    function newSession(verbs, settings, ladder, now, rng, onlyIds) {
        var list;
        if (onlyIds && onlyIds.length) {
            list = interleave(allForms(verbs).filter(function (form) { return onlyIds.indexOf(form.id) >= 0; }));
        } else {
            list = buildRound(poolOf(verbs, settings), ladder, now, rng);
        }
        return {
            items: list.map(function (form) { return makeItem(form, verbs, ladder, rng, false); }),
            idx: 0,
            tries: 0,
            phase: 'ask',
            input: '',
            wrong: [],
            results: []
        };
    }

    // ---- rendering ----

    /** Root letters wrapped in .root-letter; the person ending is skipped first. */
    function markRoot(form, rootLetters) {
        var text = form ? String(form.he || '') : '';
        var clusters = [];
        Array.from(text).forEach(function (ch) {
            if (IS_MARK.test(ch) && clusters.length) clusters[clusters.length - 1] += ch;
            else clusters.push(ch);
        });
        var letters = typeof rootLetters === 'string' && /^[\u05D0-\u05EA](\.[\u05D0-\u05EA]){1,3}$/.test(rootLetters)
            ? rootLetters.split('.')
            : [];
        function mark(skip) {
            var hits = Object.create(null);
            var r = letters.length - 1;
            var count = 0;
            for (var k = clusters.length - 1 - skip; k >= 0 && r >= 0; k -= 1) {
                var base = FINAL[clusters[k][0]] || clusters[k][0];
                if (base === letters[r]) {
                    hits[k] = true;
                    r -= 1;
                    count += 1;
                }
            }
            return { hits: hits, count: count };
        }
        var skip = form && SUFFIX[form.tense] ? (SUFFIX[form.tense][form.cell] || 0) : 0;
        var found = mark(skip);
        if (found.count < letters.length) found = mark(0);
        return clusters.map(function (c, k) {
            return found.hits[k] ? '<span class="root-letter">' + esc(c) + '</span>' : esc(c);
        }).join('');
    }

    function chip(attr, value, pressed, inner) {
        return '<button type="button" class="vp-chip" ' + attr + '="' + esc(value) + '" aria-pressed="'
            + (pressed ? 'true' : 'false') + '">' + inner + '</button>';
    }

    function renderSetup(verbs, settings) {
        var verbChips = verbs.map(function (verb) {
            return chip('data-vp-verb', verb.lemma, settings.verbs.indexOf(verb.lemma) >= 0,
                he(verb.lemma) + (verb.gloss ? ' ' + en(verb.gloss, 'vp-chip-gloss') : ''));
        }).join('');
        var tenseChips = TENSES.map(function (tense) {
            return chip('data-vp-tense', tense, settings.tenses.indexOf(tense) >= 0, bi(TENSE[tense].he, TENSE[tense].en));
        }).join('');
        return '<div class="vp-setup">'
            + '<div class="vp-chips" role="group" aria-label="פעלים">' + verbChips + '</div>'
            + '<div class="vp-chips" role="group" aria-label="זמנים">' + tenseChips
            + '<button type="button" class="vp-new-round" data-vp-action="new-round">'
            + bi('סבב חדש', 'New round') + '</button></div>'
            + '</div>';
    }

    function renderProgress(session) {
        var bits = session.items.map(function (item, i) {
            var r = session.results[i];
            var state = r || (i === session.idx ? 'current' : 'todo');
            return '<span class="vp-dot" data-state="' + state + '"></span>';
        }).join('');
        return '<div class="vp-progress" aria-hidden="true">' + bits + '</div>';
    }

    function stepLabel(item) {
        if (item.again) return bi('שוב: להקליד', 'Back again: type it');
        if (item.mode === 'recognize') return bi('שלב 1 מתוך 3: לבחור', 'Step 1 of 3: pick it');
        return bi('שלב 2 מתוך 3: להקליד', 'Step 2 of 3: type it');
    }

    function letterPad() {
        return '<div class="vp-pad letter-pad" data-letter-pad>'
            + LETTERS.map(function (letter) {
                return '<button type="button" class="letter-key" data-vp-letter="' + letter + '">' + letter + '</button>';
            }).join('')
            + '<button type="button" class="letter-key vp-pad-back" data-vp-letter="" data-vp-backspace="1">'
            + bi('מחק', 'Delete') + '</button></div>';
    }

    function renderParadigm(verb, form, reveal) {
        if (!reveal) {
            return '<div class="vp-table-hidden" data-vp-table="hidden">'
                + '<p class="vp-side-title">' + bi('הטבלה מוסתרת', 'Table hidden while you recall') + '</p>'
                + '<p class="vp-side-note" lang="en" dir="ltr">Seeing the other forms while you answer weakens memory '
                + 'for the form. The whole row opens here as soon as you answer.</p></div>';
        }
        var rows = CELLS[form.tense].map(function (cell) {
            var other = verb && verb.tenses[form.tense][cell];
            if (!other) return '';
            var current = other.id === form.id;
            return '<tr' + (current ? ' class="is-current" aria-current="true"' : '') + '>'
                + '<th scope="row" lang="he">' + esc(PERSON[cell][0]) + '</th>'
                + '<td class="vp-table-form" lang="he">' + markRoot(other, verb.root) + '</td>'
                + '<td class="vp-table-gloss" lang="en" dir="ltr">' + esc(other.gloss) + '</td></tr>';
        }).join('');
        return '<div class="vp-table" data-vp-table="open">'
            + '<p class="vp-side-title">' + he(verb ? verb.lemma : '') + ' · ' + bi(TENSE[form.tense].he, TENSE[form.tense].en) + '</p>'
            + '<table><tbody>' + rows + '</tbody></table></div>';
    }

    function renderCard(view) {
        var s = view.session;
        var item = s.items[s.idx];
        var form = item.form;
        var verb = verbOf(view.verbs, form.lemma);
        var answered = s.phase !== 'ask';
        var t = TENSE[form.tense];
        var meta = [verb && verb.gloss, verb && binyanLabel(verb.binyan), verb && verb.root ? 'root ' + verb.root : '']
            .filter(Boolean).join(' · ');
        var blank = answered
            ? '<span class="vp-answer">' + markRoot(form, verb && verb.root) + '</span>'
            : '<span class="vp-blank" aria-label="____">&nbsp;</span>';
        var body = '';
        if (!answered && item.mode === 'recognize') {
            body = '<div class="vp-options" role="group" aria-label="בחרו צורה">'
                + item.options.map(function (option, i) {
                    var wrong = s.wrong.indexOf(option) >= 0;
                    return '<button type="button" class="vp-option" data-vp-option="' + esc(option) + '"'
                        + (wrong ? ' disabled data-wrong="1"' : '') + '>'
                        + '<kbd class="vp-key" aria-hidden="true">' + (i + 1) + '</kbd>' + he(option) + '</button>';
                }).join('')
                + '</div>';
        } else if (!answered) {
            body = '<div class="vp-type">'
                + '<label class="vp-field"><span class="vp-field-label">' + bi('התשובה שלכם', 'Your answer') + '</span>'
                + '<input data-vp-input type="text" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false"'
                + ' lang="he" dir="rtl" value="' + esc(s.input) + '"></label>'
                + '<button type="button" class="primary vp-check" data-vp-action="check">' + bi('בדיקה', 'Check') + '</button>'
                + letterPad()
                + '<p class="vp-type-note" lang="en" dir="ltr">Vowel points are optional.</p>'
                + '</div>';
        }
        var feedback = '';
        if (!answered && s.tries === 1) {
            var hint = HINT[form.tense][form.cell] || '';
            if (form.tense === 'present' && verb && BINYAN[verb.binyan]) hint += ' ' + BINYAN[verb.binyan][2];
            feedback = '<div class="vp-feedback" data-result="retry">'
                + '<p class="vp-feedback-title">' + bi('כמעט. נסו שוב.', 'Not quite. Try once more.') + '</p>'
                + '<p lang="en" dir="ltr">' + esc(hint) + '</p></div>';
        } else if (s.phase === 'right') {
            feedback = '<div class="vp-feedback" data-result="pass">'
                + '<p class="vp-feedback-title">' + (s.tries === 0 ? bi('נכון!', 'Right first time') : bi('נכון.', 'Right after a retry')) + '</p>'
                + '<p lang="en" dir="ltr">The root letters are marked.'
                + (item.mode === 'recognize' ? ' Next time this form comes up you type it.' : '') + '</p></div>';
        } else if (s.phase === 'reveal') {
            feedback = '<div class="vp-feedback" data-result="miss">'
                + '<p class="vp-feedback-title">' + bi('זאת הצורה', 'Here is the form') + '</p>'
                + '<p lang="en" dir="ltr">' + esc(HINT[form.tense][form.cell] || '')
                + ' It comes back later in this round so you can recall it yourself.</p></div>';
        }
        var last = s.idx + 1 >= s.items.length;
        var next = answered
            ? '<p class="vp-next-row"><button type="button" class="primary vp-next" data-vp-action="next">'
                + (last ? bi('לסיכום', 'See the round') : bi('הבא', 'Next')) + '</button></p>'
            : '';
        return '<article class="vp-card" data-vp-card data-tense="' + form.tense + '" data-cell="' + esc(form.cell)
            + '" data-mode="' + item.mode + '" data-phase="' + s.phase + '">'
            + '<header class="vp-card-head">'
            + '<span class="vp-tense" data-tense="' + form.tense + '">' + bi(t.he, t.en) + '</span>'
            + '<span class="vp-meta">' + he(form.lemma) + (meta ? ' ' + en(meta) : '') + '</span>'
            + '<span class="vp-step">' + stepLabel(item) + '</span>'
            + '</header>'
            + '<p class="vp-prompt" lang="he" dir="rtl">'
            + '<span class="vp-anchor">' + esc(t.anchor) + '</span> '
            + '<span class="vp-person">' + esc(PERSON[form.cell][0]) + '</span> '
            + blank
            + ' <span class="vp-inf">(' + esc(form.lemma) + ')</span></p>'
            + '<p class="vp-cue" lang="en" dir="ltr">' + esc(t.anchorEn) + ': ' + esc(form.gloss) + '</p>'
            + (form.verified ? '' : '<p class="vp-draft">' + bi('טיוטה', 'Draft form: not checked by a person yet') + '</p>')
            + body
            + '<div class="vp-live" role="status" aria-live="polite">' + feedback + '</div>'
            + next
            + '</article>';
    }

    function renderMasteryMap(verbs, ladder) {
        var rows = verbs.map(function (verb) {
            var groups = TENSES.map(function (tense) {
                var squares = CELLS[tense].map(function (cell) {
                    var form = verb.tenses[tense][cell];
                    if (!form) return '';
                    var entry = entryOf(ladder, form.id);
                    var state = !entry ? 'new' : entry.l >= 2 ? 'known' : entry.l === 1 ? 'picked' : 'missed';
                    return '<span class="vp-sq" data-state="' + state + '" title="' + esc(form.gloss) + '"></span>';
                }).join('');
                return '<span class="vp-map-group" data-tense="' + tense + '">' + squares + '</span>';
            }).join('');
            return '<div class="vp-map-row"><span class="vp-map-verb">' + he(verb.lemma)
                + (verb.gloss ? ' ' + en(verb.gloss) : '') + '</span>' + groups + '</div>';
        }).join('');
        var head = '<div class="vp-map-row vp-map-head"><span class="vp-map-verb"></span>'
            + TENSES.map(function (tense) {
                return '<span class="vp-map-group">' + bi(TENSE[tense].he, TENSE[tense].en) + '</span>';
            }).join('') + '</div>';
        var legend = [
            ['new', 'לא תורגל', 'not practised'],
            ['missed', 'עוד לא', 'missed, not yet recalled'],
            ['picked', 'זוהה', 'recognised: next time you type it'],
            ['known', 'יודעים', 'typed from memory']
        ].map(function (row) {
            return '<span class="vp-legend-item"><span class="vp-sq" data-state="' + row[0] + '"></span>' + bi(row[1], row[2]) + '</span>';
        }).join('');
        return '<div class="vp-map" data-vp-map>'
            + '<p class="vp-side-title">' + bi('מפת הצורות', 'Form map: one square per person, verb, and tense') + '</p>'
            + head + rows
            + '<p class="vp-legend">' + legend + '</p></div>';
    }

    function renderSummary(view) {
        var s = view.session;
        var count = function (kind) { return s.results.filter(function (r) { return r === kind; }).length; };
        var missed = missedIds(s);
        return '<div class="vp-summary" data-vp-summary>'
            + '<div class="vp-summary-main">'
            + '<h2>' + bi('הסבב הסתיים', 'Round finished') + '</h2>'
            + '<div class="vp-stats">'
            + '<p class="vp-stat" data-kind="first"><strong>' + count('first') + '</strong>' + bi('בפעם הראשונה', 'first try') + '</p>'
            + '<p class="vp-stat" data-kind="retry"><strong>' + count('retry') + '</strong>' + bi('אחרי ניסיון נוסף', 'after one retry') + '</p>'
            + '<p class="vp-stat" data-kind="miss"><strong>' + count('miss') + '</strong>' + bi('ראיתם את התשובה', 'shown the answer') + '</p>'
            + '</div>'
            + '<p class="vp-summary-note" lang="en" dir="ltr">Missed forms already came back once in this round. '
            + 'Forms you got right move up a step: pick, then type, then known. Known forms come back in a day, '
            + 'then three, then a week, so the next round starts with whatever is due.</p>'
            + '<p class="vp-summary-actions">'
            + '<button type="button" class="primary" data-vp-action="new-round">' + bi('עוד סבב', 'Another round') + '</button>'
            + (missed.length
                ? ' <button type="button" data-vp-action="misses">' + bi('רק מה שפספסתי', 'Only the ' + missed.length + ' I missed') + '</button>'
                : '')
            + '</p></div>'
            + renderMasteryMap(view.verbs, view.ladder)
            + '</div>';
    }

    function missedIds(session) {
        var out = [];
        session.items.forEach(function (item, i) {
            if (session.results[i] === 'miss' && out.indexOf(item.form.id) < 0) out.push(item.form.id);
        });
        return out;
    }

    /**
     * @param {{ verbs: object[], settings: object, session: object, ladder: object }} view
     * @returns {string}
     */
    function renderPractice(view) {
        var s = view.session;
        var setup = renderSetup(view.verbs, view.settings);
        if (!s.items.length) {
            return '<div class="vp" data-vp data-no-lookup data-vp-state="empty">' + setup
                + '<p class="vp-empty">' + bi('אין צורות לתרגול בבחירה הזו.', 'No forms to practise with this choice.') + '</p></div>';
        }
        if (s.idx >= s.items.length) {
            return '<div class="vp" data-vp data-no-lookup data-vp-state="summary">' + setup + renderSummary(view) + '</div>';
        }
        var item = s.items[s.idx];
        var verb = verbOf(view.verbs, item.form.lemma);
        // No word lookup on the card: root-letter spans split a form, and the whole form is the answer.
        return '<div class="vp" data-vp data-no-lookup data-vp-state="card">' + setup
            + '<div class="vp-stage">'
            + renderCard(view)
            + '<aside class="vp-side">'
            + renderParadigm(verb, item.form, s.phase !== 'ask')
            + renderProgress(s)
            + '<p class="vp-count" lang="en" dir="ltr">' + Math.min(s.idx + 1, s.items.length) + ' / ' + s.items.length + '</p>'
            + '</aside></div></div>';
    }

    function renderLookTable(verb, tense) {
        var t = TENSE[tense];
        var rows = CELLS[tense].map(function (cell) {
            var form = verb.tenses[tense][cell];
            if (!form) return '';
            return '<tr' + (form.verified ? '' : ' data-draft="1"') + '>'
                + '<th scope="row" lang="he">' + esc(PERSON[cell][0]) + '</th>'
                + '<td class="vl-form" lang="he">' + markRoot(form, verb.root) + '</td>'
                + '<td class="vl-gloss" lang="en" dir="ltr">' + esc(form.gloss) + '</td></tr>';
        }).join('');
        if (!rows) return '';
        var lines = verb.lines.filter(function (line) { return line.tense === tense; }).map(function (line) {
            return '<li><span class="pointed-line" lang="he">' + esc(line.he) + '</span>'
                + (line.en ? '<span class="vl-line-en" lang="en" dir="ltr">' + esc(line.en) + '</span>' : '') + '</li>';
        }).join('');
        var note = tense === 'present' && BINYAN[verb.binyan] ? t.note + ' ' + BINYAN[verb.binyan][2] : t.note;
        return '<section class="vl-tense" data-tense="' + tense + '">'
            + '<h3>' + bi(t.he, t.en) + ' <span class="vl-anchor">' + he(t.anchor) + ' ' + en(t.anchorEn) + '</span></h3>'
            + '<p class="vl-note" lang="en" dir="ltr" data-no-lookup>' + esc(note) + '</p>'
            + '<table class="vl-table" data-no-lookup><tbody>' + rows + '</tbody></table>'
            + (lines ? '<ul class="vl-lines">' + lines + '</ul>' : '')
            + '</section>';
    }

    /**
     * One verb per screen: past, present, and future side by side, root letters marked.
     * @param {object[]} verbs
     * @param {string} lemma selected verb
     * @param {string} [extraHtml] e.g. the lesson's explainer, already rendered
     * @returns {string}
     */
    function renderLook(verbs, lemma, extraHtml) {
        var verb = verbOf(verbs, lemma) || verbs[0];
        if (!verb) return '';
        var tabs = verbs.map(function (v) {
            var selected = v.lemma === verb.lemma;
            return '<button type="button" class="vl-verb-tab" data-vl-verb="' + esc(v.lemma) + '" aria-pressed="'
                + (selected ? 'true' : 'false') + '">' + he(v.lemma) + (v.gloss ? ' ' + en(v.gloss) : '') + '</button>';
        }).join('');
        var meta = [verb.gloss, verb.binyan ? binyanLabel(verb.binyan) : ''].filter(Boolean).join(' · ');
        var binyanHe = BINYAN[verb.binyan] ? BINYAN[verb.binyan][1] : '';
        return '<div class="vl" data-vl>'
            + '<div class="vl-verbs" role="group" aria-label="פעלים">' + tabs + '</div>'
            + '<section class="vl-verb" data-vl-lemma="' + esc(verb.lemma) + '">'
            + '<header class="vl-head" data-no-lookup><h2 lang="he">' + esc(verb.lemma) + '</h2>'
            + '<p class="vl-meta">' + (meta ? en(meta) : '')
            + (binyanHe ? ' · ' + he(binyanHe) : '')
            + (verb.root ? ' · ' + en('root') + ' <span class="vl-root" lang="he">' + esc(verb.root) + '</span>' : '')
            + '</p></header>'
            + '<div class="vl-grid">' + TENSES.map(function (tense) { return renderLookTable(verb, tense); }).join('') + '</div>'
            + '</section>'
            + (hasDraft([verb])
                ? '<p class="vl-draft">' + bi('טיוטה: הניקוד עוד לא נבדק.', 'Draft: the pointing has not been checked by a person yet.') + '</p>'
                : '')
            + (extraHtml || '')
            + '<p class="vl-go"><button type="button" class="primary" data-vs-tab="practice">'
            + bi('לתרגל את הפעלים', 'Practise these verbs') + '</button></p>'
            + '</div>';
    }

    var TAB_LABEL = {
        look: ['לִרְאוֹת', 'Look'],
        practice: ['לְתַרְגֵּל', 'Practise'],
        use: ['לְהִשְׁתַּמֵּשׁ', 'Use']
    };

    /**
     * The studio around the loop. Tabs are Look / Practise / Use (Use only when there is sentence work).
     * @param {{ title?: string, kicker?: string, backHref?: string, backHe?: string, backEn?: string,
     *   verbs: object[], tabs?: string[], tab?: string, lookHtml?: string, useHtml?: string, subtitle?: string }} opts
     * @returns {string}
     */
    function renderStudio(opts) {
        var o = opts || {};
        var tabs = (o.tabs || ['look', 'practice', 'use']).filter(function (id) {
            return TAB_LABEL[id] && (id !== 'use' || o.useHtml);
        });
        var current = tabs.indexOf(o.tab) >= 0 ? o.tab : tabs[0];
        var count = allForms(o.verbs).length;
        var verbsText = o.verbs.length + (o.verbs.length === 1 ? ' verb' : ' verbs') + ' · ' + count + ' forms';
        var nav = tabs.map(function (id, i) {
            var selected = id === current;
            return '<button type="button" role="tab" id="vs-tab-' + id + '" aria-controls="vs-panel-' + id + '"'
                + ' aria-selected="' + (selected ? 'true' : 'false') + '" tabindex="' + (selected ? '0' : '-1') + '"'
                + ' data-vs-tab="' + id + '"><span class="vs-tab-num" aria-hidden="true">' + (i + 1) + '</span>'
                + bi(TAB_LABEL[id][0], TAB_LABEL[id][1]) + '</button>';
        }).join('');
        var panels = tabs.map(function (id) {
            var inner = id === 'look' ? (o.lookHtml || '')
                : id === 'use' ? '<p class="vs-use-intro" lang="en" dir="ltr">Sentences once the forms come easily: '
                    + 'pick the sentence that fits, translate, then write your own.</p>' + o.useHtml
                : '<div data-vp-host></div>';
            return '<section class="vs-panel" role="tabpanel" id="vs-panel-' + id + '" aria-labelledby="vs-tab-' + id + '"'
                + ' data-vs-panel="' + id + '"' + (id === current ? '' : ' hidden') + '>' + inner + '</section>';
        }).join('');
        return '<div class="verb-studio" data-verb-studio data-vs-tab-current="' + current + '">'
            + '<header class="vs-head">'
            + (o.backHref ? '<a class="vs-back" href="' + esc(o.backHref) + '">' + bi(o.backHe || 'שיעורים', o.backEn || 'Lessons') + '</a>' : '')
            + (o.kicker ? '<p class="kicker">' + esc(o.kicker) + '</p>' : '')
            + (o.title ? '<h1>' + esc(o.title) + '</h1>' : '')
            + '<p class="vs-sub" lang="en" dir="ltr">' + esc(o.subtitle || verbsText) + '</p>'
            + '</header>'
            + '<div class="vs-tabs" role="tablist" aria-label="מצב">' + nav + '</div>'
            + panels
            + '</div>';
    }

    // ---- behaviour ----

    function signalFor(form, result, slug) {
        var Conj = root.HebrewBlockConjugation;
        var item = { formKey: form.key, lemma: form.lemma };
        if (Conj && typeof Conj.formSignal === 'function') return Conj.formSignal(item, result, 'conjugation-drill', slug);
        var signal = { word: form.key, lemma: form.lemma, kind: 'form', result: result, direction: 'en_he', activity: 'conjugation-drill' };
        if (slug) signal.slug = slug;
        return signal;
    }

    function defaultPost(signal) {
        var Sync = root.HebrewSync;
        if (signal && Sync && typeof Sync.queueSignal === 'function' && typeof Sync.isActive === 'function' && Sync.isActive()) {
            Sync.queueSignal(signal);
        }
    }

    /**
     * Wire a rendered studio. Returns { destroy }.
     * @param {Element} el the element holding renderStudio output
     * @param {{ verbs: object[], user?: object, storage?: Storage, now?: function(): number,
     *   rng?: function(): number, slug?: string, post?: function(object): void,
     *   explainerHtml?: string, onTab?: function(string): void }} cfg
     */
    function mount(el, cfg) {
        var c = cfg || {};
        var verbs = c.verbs || [];
        var now = typeof c.now === 'function' ? c.now : function () { return Date.now(); };
        var storage = c.storage !== undefined ? c.storage : (function () {
            try { return root.localStorage || null; } catch (err) { return null; }
        })();
        var post = typeof c.post === 'function' ? c.post : defaultPost;
        var state = {
            ladder: loadLadder(storage, c.user),
            settings: { verbs: verbs.map(function (v) { return v.lemma; }), tenses: TENSES.slice(), allowDraft: true },
            look: verbs[0] ? verbs[0].lemma : '',
            session: null
        };
        state.session = newSession(verbs, state.settings, state.ladder, now(), c.rng);

        function host() { return el.querySelector('[data-vp-host]'); }

        function visible(node) {
            var panel = node && typeof node.closest === 'function' ? node.closest('[data-vs-panel]') : null;
            return !panel || !panel.hidden;
        }

        function paint(focus) {
            var h = host();
            if (!h) return;
            h.innerHTML = renderPractice({ verbs: verbs, settings: state.settings, session: state.session, ladder: state.ladder });
            if (!focus || !visible(h)) return;
            var target = h.querySelector('[data-vp-input]') || h.querySelector('[data-vp-action="next"]')
                || h.querySelector('.vp-option:not([disabled])') || h.querySelector('[data-vp-action="new-round"]');
            if (target && typeof target.focus === 'function') target.focus();
        }

        function paintLook() {
            var panel = el.querySelector('[data-vs-panel="look"]');
            if (!panel) return;
            panel.innerHTML = renderLook(verbs, state.look, c.explainerHtml || '');
        }

        function record(form, mode, outcome) {
            state.ladder = applyResult(state.ladder, form.id, mode, outcome, now());
            saveLadder(storage, c.user, state.ladder);
        }

        function submit(answer) {
            var s = state.session;
            var item = s.items[s.idx];
            if (!item || s.phase !== 'ask') return;
            var ok = isMatch(answer, item.form);
            if (ok === null) return;
            if (item.mode === 'type' && item.form.verified) post(signalFor(item.form, ok ? 'pass' : 'miss', c.slug || ''));
            if (ok) {
                record(item.form, item.mode, s.tries === 0 ? 'first' : 'retry');
                s.phase = 'right';
                s.results.push(s.tries === 0 ? 'first' : 'retry');
            } else if (s.tries === 0) {
                s.tries = 1;
                s.input = '';
                if (item.mode === 'recognize') s.wrong.push(answer);
            } else {
                record(item.form, item.mode, 'miss');
                s.phase = 'reveal';
                s.results.push('miss');
                if (!item.again) s.items.push(makeItem(item.form, verbs, state.ladder, c.rng, true));
            }
            paint(true);
        }

        function next() {
            var s = state.session;
            if (s.phase === 'ask') return;
            s.idx += 1;
            s.tries = 0;
            s.phase = 'ask';
            s.input = '';
            s.wrong = [];
            paint(true);
        }

        function restart(onlyIds) {
            state.session = newSession(verbs, state.settings, state.ladder, now(), c.rng, onlyIds);
            paint(true);
        }

        function toggle(list, value) {
            var at = list.indexOf(value);
            if (at >= 0 && list.length > 1) list.splice(at, 1);
            else if (at < 0) list.push(value);
        }

        function showTab(id, focusTab) {
            var tabs = el.querySelectorAll('[data-vs-tab][role="tab"]');
            var found = false;
            for (var i = 0; i < tabs.length; i += 1) {
                var on = tabs[i].getAttribute('data-vs-tab') === id;
                if (on) found = true;
                tabs[i].setAttribute('aria-selected', on ? 'true' : 'false');
                tabs[i].setAttribute('tabindex', on ? '0' : '-1');
                if (on && focusTab && typeof tabs[i].focus === 'function') tabs[i].focus();
            }
            if (!found) return;
            var panels = el.querySelectorAll('[data-vs-panel]');
            for (var j = 0; j < panels.length; j += 1) panels[j].hidden = panels[j].getAttribute('data-vs-panel') !== id;
            var studio = el.querySelector('[data-verb-studio]');
            if (studio) studio.setAttribute('data-vs-tab-current', id);
            if (id === 'practice') paint(!focusTab);
            if (typeof c.onTab === 'function') c.onTab(id);
        }

        function onClick(event) {
            var target = event.target;
            if (!target || typeof target.closest !== 'function') return;
            var tab = target.closest('[data-vs-tab]');
            if (tab) {
                showTab(tab.getAttribute('data-vs-tab'), tab.getAttribute('role') === 'tab');
                return;
            }
            var lookVerb = target.closest('[data-vl-verb]');
            if (lookVerb) {
                state.look = lookVerb.getAttribute('data-vl-verb');
                paintLook();
                return;
            }
            var verbChip = target.closest('[data-vp-verb]');
            if (verbChip) {
                toggle(state.settings.verbs, verbChip.getAttribute('data-vp-verb'));
                restart();
                return;
            }
            var tenseChip = target.closest('[data-vp-tense]');
            if (tenseChip) {
                toggle(state.settings.tenses, tenseChip.getAttribute('data-vp-tense'));
                restart();
                return;
            }
            var option = target.closest('[data-vp-option]');
            if (option && !option.disabled) {
                submit(option.getAttribute('data-vp-option'));
                return;
            }
            var letter = target.closest('[data-vp-letter]');
            if (letter) {
                var input = el.querySelector('[data-vp-input]');
                if (!input) return;
                if (letter.getAttribute('data-vp-backspace') === '1') input.value = Array.from(input.value).slice(0, -1).join('');
                else input.value += letter.getAttribute('data-vp-letter') || '';
                state.session.input = input.value;
                if (typeof input.focus === 'function') input.focus();
                return;
            }
            var action = target.closest('[data-vp-action]');
            if (!action) return;
            var name = action.getAttribute('data-vp-action');
            if (name === 'check') {
                var field = el.querySelector('[data-vp-input]');
                submit(field ? field.value : state.session.input);
            } else if (name === 'next') next();
            else if (name === 'new-round') restart();
            else if (name === 'misses') restart(missedIds(state.session));
        }

        function onInput(event) {
            var target = event.target;
            if (target && target.hasAttribute && target.hasAttribute('data-vp-input')) state.session.input = target.value;
        }

        function onKey(event) {
            var target = event.target;
            var s = state.session;
            var inField = target && target.hasAttribute && target.hasAttribute('data-vp-input');
            if (target && target.getAttribute && target.getAttribute('role') === 'tab'
                && (event.key === 'ArrowLeft' || event.key === 'ArrowRight')) {
                var tabs = Array.prototype.slice.call(el.querySelectorAll('[data-vs-tab][role="tab"]'));
                var at = tabs.indexOf(target);
                // RTL: ArrowLeft moves to the next tab.
                var step = event.key === 'ArrowLeft' ? 1 : -1;
                var nextTab = tabs[(at + step + tabs.length) % tabs.length];
                if (nextTab) showTab(nextTab.getAttribute('data-vs-tab'), true);
                event.preventDefault();
                return;
            }
            if (!host() || !visible(host()) || !s || !s.items[s.idx]) return;
            if (event.key === 'Enter') {
                if (inField && s.phase === 'ask') {
                    submit(target.value);
                    event.preventDefault();
                } else if (s.phase !== 'ask' && (!target.closest || !target.closest('button'))) {
                    next();
                    event.preventDefault();
                }
                return;
            }
            if (!inField && s.phase === 'ask' && s.items[s.idx].mode === 'recognize' && /^[1-4]$/.test(event.key || '')) {
                var opts = el.querySelectorAll('.vp-option');
                var pick = opts[Number(event.key) - 1];
                if (pick && !pick.disabled) {
                    submit(pick.getAttribute('data-vp-option'));
                    event.preventDefault();
                }
            }
        }

        el.addEventListener('click', onClick);
        el.addEventListener('input', onInput);
        el.addEventListener('keydown', onKey);
        paint(false);

        return {
            state: state,
            showTab: showTab,
            destroy: function () {
                el.removeEventListener('click', onClick);
                el.removeEventListener('input', onInput);
                el.removeEventListener('keydown', onKey);
            }
        };
    }

    // ---- data loading ----

    function safeKey(value) {
        var key = String(value == null ? '' : value).normalize('NFC').trim();
        if (!key || key.length > 60 || !/[\u05D0-\u05EA]/.test(key) || /[\u0000-\u001F<>]/.test(key)) return '';
        return key;
    }

    /** Conjugation-table blocks of one lesson (or of every lesson when lessonId is empty). */
    function conjugationBlocks(plan, lessonId) {
        var out = [];
        var units = plan && Array.isArray(plan.units) ? plan.units : [];
        units.forEach(function (unit) {
            (unit && Array.isArray(unit.lessons) ? unit.lessons : []).forEach(function (lesson) {
                if (!lesson || (lessonId && lesson.id !== lessonId)) return;
                (Array.isArray(lesson.blocks) ? lesson.blocks : []).forEach(function (block) {
                    if (block && block.type === 'conjugation-table') out.push(block);
                });
            });
        });
        return out;
    }

    /**
     * Forms and lemma rows for the verbs in these blocks, from GET /api/lexicon/:lemma.
     * @param {object[]} blocks
     * @param {function(string): Promise<object|null>} getJson resolves the parsed body or null
     * @returns {Promise<{ forms: object, lemmas: object, paradigms: object }>}
     */
    function loadVerbData(blocks, getJson) {
        var keys = [];
        (blocks || []).forEach(function (block) {
            var key = safeKey(block && block.lemmaKey);
            if (key && keys.indexOf(key) < 0) keys.push(key);
        });
        return Promise.all(keys.slice(0, 24).map(function (key) {
            return Promise.resolve(getJson('/api/lexicon/' + encodeURIComponent(key))).then(function (body) {
                return body ? { key: key, body: body } : null;
            }, function () { return null; });
        })).then(function (rows) {
            var forms = Object.create(null);
            var lemmas = Object.create(null);
            var paradigms = Object.create(null);
            rows.forEach(function (row) {
                if (!row || !row.body || typeof row.body !== 'object') return;
                var entry = row.body.entry || row.body.lemma;
                if (entry && typeof entry === 'object') lemmas[row.key] = entry;
                var paradigm = row.body.paradigm && typeof row.body.paradigm === 'object' ? row.body.paradigm : {};
                paradigms[row.key] = paradigm;
                Object.keys(paradigm).forEach(function (tense) {
                    var cells = paradigm[tense] || {};
                    Object.keys(cells).forEach(function (cell) {
                        var form = cells[cell];
                        if (!form || typeof form !== 'object' || typeof form.key !== 'string') return;
                        if (!form.root && entry && typeof entry.root === 'string') form.root = entry.root;
                        forms[form.key] = form;
                    });
                });
            });
            return { forms: forms, lemmas: lemmas, paradigms: paradigms };
        });
    }

    return {
        CELLS: CELLS,
        DAY: DAY,
        INTERVAL_DAYS: INTERVAL_DAYS,
        ROUND: ROUND,
        TENSES: TENSES,
        allForms: allForms,
        applyResult: applyResult,
        buildRound: buildRound,
        collectVerbs: collectVerbs,
        conjugationBlocks: conjugationBlocks,
        displayKey: displayKey,
        dueSummary: dueSummary,
        hasDraft: hasDraft,
        interleave: interleave,
        isMatch: isMatch,
        loadLadder: loadLadder,
        loadVerbData: loadVerbData,
        markRoot: markRoot,
        modeFor: modeFor,
        mount: mount,
        newSession: newSession,
        optionsFor: optionsFor,
        poolOf: poolOf,
        renderLook: renderLook,
        renderPractice: renderPractice,
        renderStudio: renderStudio,
        saveLadder: saveLadder,
        storageKey: storageKey
    };
});
