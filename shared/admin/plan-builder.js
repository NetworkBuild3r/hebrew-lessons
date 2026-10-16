/**
 * Admin outline for one lesson plan: insert, move, remove, publish, and the forms
 * that edit any part of it. The forms are not written here: plan-form.js renders
 * an edit form for any plan node from the plan JSON Schema (GET /api/plan-schema),
 * with the widgets of plan-widgets.js and the hints of plan-hints.js, and sends
 * each save through the `transport` this file supplies. A new block is generated
 * from the schema's `required` list, so no block type is written out here.
 * Media upload, dry-run, confirmed generation, and approval are the SPEC-011
 * routes. Picture attach sends set_field. The validation panel prints the
 * save and publish findings. It does not recompute a cap.
 * Every change is an HTTP call. This file does not edit the plan JSON,
 * does not save a worksheet, and does not read an API key.
 * Listeners go on the panel and are removed on unmount (an AbortSignal where the
 * browser has one).
 *
 * The panels of INIT-008 are separate modules this file mounts and wires:
 *   plan-list.js     the Plans tab (mountPicker) and the assign control
 *   plan-history.js  revisions, compare, restore, and the "edited by a person" restore
 *   plan-issues.js   the issues panel, with a jump to the field and a one-click fix
 *   plan-preview.js  the learner's view of the unsaved text
 * Each is optional: without it the plan view is the editor alone.
 * Provenance: INIT-004/SPEC-015, INIT-004/SPEC-016, INIT-008/SPEC-011, INIT-008/SPEC-012,
 *             INIT-008/SPEC-013, INIT-008/SPEC-014, INIT-008/SPEC-015, INIT-009/SPEC-003,
 *             INIT-010/SPEC-003, INIT-011/SPEC-003, INIT-011/SPEC-004, INIT-011/SPEC-005
 */
(function (root) {
    'use strict';

    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var ENTRY = /^[a-z0-9][a-z0-9-]*$/;
    var LESSON_PATH = /^units\/[a-z0-9]+(?:-[a-z0-9]+)*\/lessons\/[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var PART_PATH = /^units\/[a-z0-9]+(?:-[a-z0-9]+)*\/lessons\/[a-z0-9]+(?:-[a-z0-9]+)*\/blocks\/[a-z0-9][a-z0-9-]*(?:\/(?:words|paragraphs|lines|questions)\/[a-z0-9][a-z0-9-]*)?$/;
    var MEDIA_ID = /^[a-z0-9][a-z0-9-]{0,63}$/;

    var token = 0;
    var state = null;
    var inflight = Promise.resolve();
    var pickerCleanup = null;

    function api() {
        return root.HebrewApi;
    }

    function el(tag) {
        return root.document.createElement(tag);
    }

    function empty(node) {
        while (node.firstChild) node.removeChild(node.firstChild);
    }

    function alive(mine) {
        return state && state.token === mine;
    }

    function track(promise) {
        inflight = Promise.resolve(promise);
        return inflight;
    }

    /**
     * Index the move route stores after it has taken the block out of the list.
     * Moving down one slot is `fromIndex + 1` in that shorter list. Block ids stay put.
     * @param {number} fromIndex
     * @param {'up'|'down'} direction
     * @returns {number|null}
     */
    function moveDestination(fromIndex, direction) {
        if (!Number.isInteger(fromIndex) || fromIndex < 0) return null;
        if (direction === 'up') return fromIndex - 1;
        if (direction === 'down') return fromIndex + 1;
        return null;
    }

    function form() {
        return root.HebrewPlanForm;
    }

    /**
     * A new block, from the schema's `required` list (plan-form.js). Does not copy or
     * renumber ids already in the lesson, and writes no Hebrew.
     * @param {string} type
     * @param {string} id
     * @returns {object|null}
     */
    function newBlock(type, id) {
        if (!ENTRY.test(id) || !form()) return null;
        return form().newBlock(type, id);
    }

    function partsUrl(partPath) {
        var url = '/api/plans/' + encodeURIComponent(state.planId) + '/parts';
        if (partPath) url += '?path=' + encodeURIComponent(partPath);
        return url;
    }

    function listLessons(plan) {
        if (form() && typeof form().stackLessons === 'function') {
            return form().stackLessons(plan);
        }
        var out = [];
        var units = plan && Array.isArray(plan.units) ? plan.units : [];
        var u;
        var i;
        for (u = 0; u < units.length; u++) {
            var unit = units[u];
            if (!unit || typeof unit.id !== 'string' || !SLUG.test(unit.id)) continue;
            var lessons = Array.isArray(unit.lessons) ? unit.lessons : [];
            for (i = 0; i < lessons.length; i++) {
                var lesson = lessons[i];
                if (!lesson || typeof lesson.id !== 'string' || !SLUG.test(lesson.id)) continue;
                out.push({
                    path: 'units/' + unit.id + '/lessons/' + lesson.id,
                    title: typeof lesson.title === 'string' ? lesson.title : lesson.id,
                    id: lesson.id,
                    unitId: unit.id
                });
            }
        }
        return out;
    }

    function readUiLang() {
        try {
            if (root.HebrewUtils && typeof root.HebrewUtils.readUiLang === 'function') {
                return root.HebrewUtils.readUiLang() === 'he' ? 'he' : 'en';
            }
        } catch (err) { /* ignore */ }
        return 'en';
    }

    function fillOptionUi(option, he, en) {
        option.setAttribute('data-ui-text-he', he);
        option.setAttribute('data-ui-text-en', en);
        option.textContent = readUiLang() === 'he' ? he : en;
    }

    /** Author-facing activity names (shared/admin/activity-map.js). */
    function activityMap() {
        return root.HebrewActivityMap || null;
    }

    function activityNames() {
        var map = activityMap();
        if (map && Array.isArray(map.ACTIVITY_NAMES)) return map.ACTIVITY_NAMES.slice();
        // Match omitted until INIT-010 SPEC-004 lands a match block (INIT-011/SPEC-002).
        return ['Words', 'Story', 'Listening', 'Dialogue', 'Writing', 'Verse', 'Pattern', 'Type it', 'Fluency', 'Review', 'Notes'];
    }

    function blockTypeForActivity(name) {
        var map = activityMap();
        if (map && typeof map.blockTypeFor === 'function') return map.blockTypeFor(name);
        return undefined;
    }

    /** Hebrew labels for the activity menu (one language shown via data-ui-text-*). */
    var ACTIVITY_HE = {
        Words: 'מילים',
        Story: 'סיפור',
        Listening: 'האזנה',
        Dialogue: 'דיאלוג',
        Writing: 'כתיבה',
        Notes: 'הערות',
        Verse: 'פסוק',
        'Type it': 'הקלדה',
        Fluency: 'שוטפות',
        Review: 'חזרה',
        Pattern: 'תבנית'
    };

    /** Short id prefix per activity for auto-made block ids (no Id field on screen). */
    var ACTIVITY_ID_BASE = {
        Words: 'words',
        Story: 'story',
        Listening: 'listening',
        Dialogue: 'dialogue',
        Writing: 'writing',
        Notes: 'notes',
        Verse: 'verse',
        'Type it': 'type-it',
        Fluency: 'fluency',
        Review: 'review',
        Pattern: 'pattern'
    };

    /**
     * One-line hint for an activity name. Prefer the shared map; fall back locally.
     * @param {string} name
     * @returns {{ he: string, en: string }}
     */
    function activityHint(name) {
        var map = activityMap();
        if (map && typeof map.hintFor === 'function') {
            var fromMap = map.hintFor(name);
            if (fromMap) return fromMap;
        }
        return { he: '', en: '' };
    }

    /**
     * Blocks the teacher sees: every block except the empty fluency placeholder.
     * @param {object|null} lesson
     * @returns {object[]}
     */
    function listedBlocks(lesson) {
        return blocksOf(lesson).filter(function (block) {
            return block && typeof block.id === 'string' && block.id !== 'placeholder' && ENTRY.test(block.id);
        });
    }

    /**
     * Next free block id for an activity type (page-made; teacher never types it).
     * @param {string} activityName
     * @param {object[]} blocks
     * @returns {string}
     */
    function nextBlockId(activityName, blocks) {
        var base = ACTIVITY_ID_BASE[activityName] || String(activityName || 'block')
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '-')
            .replace(/^-+|-+$/g, '') || 'block';
        if (!ENTRY.test(base)) base = 'block';
        var used = {};
        (Array.isArray(blocks) ? blocks : []).forEach(function (block) {
            if (block && typeof block.id === 'string') used[block.id] = true;
        });
        if (!used[base]) return base;
        var n = 2;
        while (used[base + '-' + n]) n += 1;
        return base + '-' + n;
    }

    /** Plain-text fallback when a block type has no learner renderer. */
    function activityPreviewText(block) {
        if (!block || typeof block !== 'object') return '';
        var parts = [];
        function push(value) {
            if (typeof value === 'string' && value.trim()) parts.push(value.trim());
        }
        push(block.heading);
        push(block.notice);
        push(block.passage);
        push(block.verse);
        push(block.prompt);
        push(block.focus);
        if (Array.isArray(block.words)) {
            block.words.forEach(function (w) {
                if (w && typeof w.word === 'string') push(w.word);
            });
        }
        if (Array.isArray(block.paragraphs)) {
            block.paragraphs.forEach(function (p) {
                if (p && typeof p.text === 'string') push(p.text);
            });
        }
        if (Array.isArray(block.questions)) {
            block.questions.forEach(function (q) {
                if (q && typeof q.question === 'string') push(q.question);
            });
        }
        if (block.cloze && Array.isArray(block.cloze.items)) {
            block.cloze.items.forEach(function (item) {
                if (!item) return;
                push((item.textPre || '') + ' ___ ' + (item.textPost || ''));
                push(item.answer);
            });
        }
        if (Array.isArray(block.lines)) {
            block.lines.forEach(function (line) {
                if (line && typeof line.pointed === 'string') push(line.pointed);
            });
        }
        if (Array.isArray(block.examples)) {
            block.examples.forEach(function (ex) {
                if (ex && typeof ex.he === 'string') push(ex.he);
            });
        }
        return parts.join('\n');
    }

    /** Smallest fluency-pass shell a new lesson needs (schema minItems: 1). */
    function newLessonShell(id, title, subtitle) {
        var lesson = {
            id: id,
            title: title || id,
            level: 'basic',
            visibility: 'hidden',
            buildsOn: [],
            newWords: [],
            recycles: [],
            blocks: [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }]
        };
        if (typeof subtitle === 'string' && subtitle.trim()) lesson.subtitle = subtitle.trim();
        return lesson;
    }

    function nextLessonId(lessons) {
        var used = {};
        (Array.isArray(lessons) ? lessons : []).forEach(function (lesson) {
            if (lesson && typeof lesson.id === 'string') used[lesson.id] = true;
        });
        var n = 1;
        while (used['lesson-' + n]) n += 1;
        return 'lesson-' + n;
    }

    /**
     * Lesson id from a teacher title. Latin letters become a slug; titles with no
     * Latin letters (e.g. Hebrew-only) fall back to lesson-N. Always unique in `used`.
     * @param {string} title
     * @param {object|string[]|{id?: string}[]} used ids already taken, or lesson list
     * @returns {string}
     */
    function slugFromTitle(title, used) {
        var taken = {};
        if (Array.isArray(used)) {
            used.forEach(function (item) {
                if (typeof item === 'string') taken[item] = true;
                else if (item && typeof item.id === 'string') taken[item.id] = true;
            });
        } else if (used && typeof used === 'object') {
            Object.keys(used).forEach(function (k) { if (used[k]) taken[k] = true; });
        }
        var raw = String(title || '');
        var hasLatin = /[A-Za-z]/.test(raw);
        var base = '';
        if (hasLatin) {
            base = raw.toLowerCase()
                .replace(/[^a-z0-9]+/g, '-')
                .replace(/^-+|-+$/g, '')
                .replace(/-{2,}/g, '-');
            if (!base || !SLUG.test(base)) base = '';
        }
        if (!base) {
            var n = 1;
            while (taken['lesson-' + n]) n += 1;
            return 'lesson-' + n;
        }
        if (!taken[base]) return base;
        var suffix = 2;
        while (taken[base + '-' + suffix]) suffix += 1;
        return base + '-' + suffix;
    }

    /**
     * Parse pasted word-bank lines. Separators: en-dash, hyphen, tab, comma, or =.
     * A line with no separator is reported, not dropped.
     * @param {string} text
     * @returns {{ words: { word: string, meaning: string }[], rejected: string[] }}
     */
    function parseWordLines(text) {
        var words = [];
        var rejected = [];
        String(text || '').split(/\r?\n/).forEach(function (line) {
            var trimmed = String(line || '').trim();
            if (!trimmed) return;
            var sep = /\s*[–\t,=]\s*|\s+-\s+/;
            var match = sep.exec(trimmed);
            if (!match) {
                rejected.push(trimmed);
                return;
            }
            var left = trimmed.slice(0, match.index).trim();
            var right = trimmed.slice(match.index + match[0].length).trim();
            if (!left || !right) {
                rejected.push(trimmed);
                return;
            }
            words.push({ word: left, meaning: right });
        });
        return { words: words, rejected: rejected };
    }

    /**
     * Words a lesson introduces (newWords plus introduction block words), unique.
     * @param {object} lesson
     * @returns {string[]}
     */
    function wordsIntroducedByLesson(lesson) {
        var out = [];
        var seen = {};
        function add(w) {
            if (typeof w !== 'string' || !w || seen[w]) return;
            seen[w] = true;
            out.push(w);
        }
        if (lesson && Array.isArray(lesson.newWords)) lesson.newWords.forEach(add);
        if (lesson && Array.isArray(lesson.blocks)) {
            lesson.blocks.forEach(function (block) {
                if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) return;
                block.words.forEach(function (entry) {
                    add(entry && typeof entry.word === 'string' ? entry.word : '');
                });
            });
        }
        return out;
    }

    /**
     * What a plan says about one word: its meaning (the introduction's, else the story's
     * gloss), the pointed spelling and sound it saved, and how many lessons use it. Marks
     * and niqqud are ignored when matching.
     * @param {object} plan
     * @param {string} word
     * @returns {{ meaning: string, translit: string, pointed: string, lessons: number }|null}
     */
    function factsFromPlan(plan, word) {
        var bare = function (text) {
            return String(text || '').normalize('NFC').replace(/[\u0591-\u05C7]/g, '');
        };
        var want = bare(word);
        if (!want || !plan || !Array.isArray(plan.units)) return null;
        var meaning = '';
        var translit = '';
        var pointed = '';
        var glossMeaning = '';
        var lessons = 0;

        function note(text) {
            var out = {};
            (String(text || '').match(/[\u05D0-\u05EA\u0591-\u05C7]+/g) || []).forEach(function (token) {
                out[bare(token)] = token;
            });
            return out;
        }

        plan.units.forEach(function (unit) {
            (unit && Array.isArray(unit.lessons) ? unit.lessons : []).forEach(function (lesson) {
                if (!lesson) return;
                var seen = {};
                var join = function (map) {
                    Object.keys(map).forEach(function (key) {
                        if (!seen[key]) seen[key] = map[key];
                    });
                };
                (Array.isArray(lesson.newWords) ? lesson.newWords : []).forEach(function (w) { join(note(w)); });
                (Array.isArray(lesson.blocks) ? lesson.blocks : []).forEach(function (block) {
                    if (!block) return;
                    (Array.isArray(block.words) ? block.words : []).forEach(function (entry) {
                        if (!entry) return;
                        join(note(entry.word));
                        if (bare(entry.word) !== want) return;
                        if (!meaning && typeof entry.meaning === 'string') meaning = entry.meaning.trim();
                        if (!translit && typeof entry.translit === 'string') translit = entry.translit.trim();
                        if (!pointed && /[\u0591-\u05C7]/.test(entry.word)) pointed = entry.word;
                    });
                    (Array.isArray(block.paragraphs) ? block.paragraphs : []).forEach(function (p) {
                        join(note(p && p.text));
                    });
                    (Array.isArray(block.lines) ? block.lines : []).forEach(function (line) {
                        join(note(line && line.pointed));
                    });
                    join(note(block.passage));
                    if (block.glosses && typeof block.glosses === 'object' && !Array.isArray(block.glosses)) {
                        Object.keys(block.glosses).forEach(function (key) {
                            join(note(key));
                            if (bare(key) !== want) return;
                            if (!glossMeaning && typeof block.glosses[key] === 'string') glossMeaning = block.glosses[key].trim();
                            if (!pointed && /[\u0591-\u05C7]/.test(key)) pointed = key;
                        });
                    }
                });
                if (seen[want]) {
                    lessons += 1;
                    if (!pointed && /[\u0591-\u05C7]/.test(seen[want])) pointed = seen[want];
                }
            });
        });
        meaning = meaning || glossMeaning;
        if (!meaning && !translit && !lessons) return null;
        return { meaning: meaning, translit: translit, pointed: pointed, lessons: lessons };
    }

    /**
     * The facts the open plan saved for a word, for the tap card.
     * @param {string} word
     * @returns {{ meaning: string, translit: string, pointed: string, lessons: number }|null}
     */
    function wordFacts(word) {
        return state && state.plan ? factsFromPlan(state.plan, word) : null;
    }

    /**
     * Plan word bank: each introduced word and the lesson that added it (first wins).
     * @param {object} plan
     * @returns {{ word: string, lessonId: string, lessonTitle: string }[]}
     */
    function planWordBank(plan) {
        var rows = [];
        var seen = {};
        var choices = listLessons(plan);
        var i;
        for (i = 0; i < choices.length; i++) {
            var choice = choices[i];
            var lesson = form() && form().nodeAt
                ? form().nodeAt(plan, choice.path)
                : null;
            if (!lesson) {
                var units = plan && Array.isArray(plan.units) ? plan.units : [];
                var u;
                for (u = 0; u < units.length && !lesson; u++) {
                    var lessons = units[u] && Array.isArray(units[u].lessons) ? units[u].lessons : [];
                    var l;
                    for (l = 0; l < lessons.length; l++) {
                        if (lessons[l] && lessons[l].id === choice.id) {
                            lesson = lessons[l];
                            break;
                        }
                    }
                }
            }
            wordsIntroducedByLesson(lesson).forEach(function (word) {
                if (seen[word]) return;
                seen[word] = true;
                rows.push({
                    word: word,
                    lessonId: choice.id,
                    lessonTitle: choice.title || choice.id
                });
            });
        }
        return rows;
    }

    /**
     * Copy a lesson for reuse into this plan. New id when taken. buildsOn keeps only
     * ids earlier in this plan; if the original is earlier here, the copy builds on it.
     * @param {object} sourceLesson
     * @param {string} sourceLessonId
     * @param {{ id: string }[]} thisPlanLessons ordered lessons already in this plan
     * @returns {object}
     */
    function prepareReuseLesson(sourceLesson, sourceLessonId, thisPlanLessons) {
        var lessons = Array.isArray(thisPlanLessons) ? thisPlanLessons : [];
        var used = lessons.map(function (l) { return l && l.id; }).filter(Boolean);
        var earlier = {};
        var sourceEarlier = false;
        var i;
        for (i = 0; i < lessons.length; i++) {
            if (lessons[i] && typeof lessons[i].id === 'string') {
                earlier[lessons[i].id] = true;
                if (lessons[i].id === sourceLessonId) sourceEarlier = true;
            }
        }
        var copy;
        try {
            copy = JSON.parse(JSON.stringify(sourceLesson || {}));
        } catch (err) {
            copy = {};
        }
        var preferred = typeof sourceLessonId === 'string' && SLUG.test(sourceLessonId)
            ? sourceLessonId
            : '';
        var newId = preferred && used.indexOf(preferred) === -1
            ? preferred
            : slugFromTitle(
                (copy && typeof copy.title === 'string' && copy.title) || preferred || 'lesson',
                used
            );
        if (used.indexOf(newId) !== -1) newId = nextLessonId(lessons);
        copy.id = newId;
        if (typeof copy.title !== 'string' || !copy.title) {
            copy.title = (sourceLesson && sourceLesson.title) || newId;
        }
        var builds = Array.isArray(copy.buildsOn) ? copy.buildsOn : [];
        var kept = builds.filter(function (id) {
            return typeof id === 'string' && earlier[id];
        });
        if (sourceEarlier && kept.indexOf(sourceLessonId) === -1) kept.push(sourceLessonId);
        copy.buildsOn = kept;
        if (!Array.isArray(copy.newWords)) copy.newWords = [];
        if (!Array.isArray(copy.recycles)) copy.recycles = [];
        if (!Array.isArray(copy.blocks) || !copy.blocks.length) {
            copy.blocks = [{ id: 'placeholder', type: 'fluency-pass', seconds: 60, focus: 'vocab' }];
        }
        if (!copy.level) copy.level = 'basic';
        if (!copy.visibility) copy.visibility = 'hidden';
        return copy;
    }

    /** Header sentence: who the plan is for. */
    function forAudienceLine(plan) {
        if (plan && plan.audience === 'personal' && typeof plan.assigneeId === 'string' && plan.assigneeId) {
            return {
                he: 'עבור ' + plan.assigneeId,
                en: 'For ' + plan.assigneeId
            };
        }
        return { he: 'עבור כל לומד', en: 'For every learner' };
    }

    function unitById(unitId) {
        var units = state && state.plan && Array.isArray(state.plan.units) ? state.plan.units : [];
        var i;
        for (i = 0; i < units.length; i++) {
            if (units[i] && units[i].id === unitId) return units[i];
        }
        return null;
    }

    function blocksOf(lesson) {
        return lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
    }

    function blockById(id) {
        var blocks = blocksOf(state && state.lesson);
        var i;
        for (i = 0; i < blocks.length; i++) {
            if (blocks[i] && blocks[i].id === id) return { block: blocks[i], index: i };
        }
        return null;
    }

    function labeled(caption, control) {
        var label = el('label');
        var name = el('span');
        fillUi(name, caption);
        label.appendChild(name);
        label.appendChild(control);
        return label;
    }

    function splitUiLabel(text) {
        var Ui = root.HebrewPlanUi;
        if (Ui && typeof Ui.splitUiLabel === 'function') return Ui.splitUiLabel(text);
        var raw = String(text == null ? '' : text).replace(/\s+$/, '');
        var close = raw.lastIndexOf(')');
        if (close !== raw.length - 1 || close < 0) return null;
        var open = raw.lastIndexOf(' (');
        if (open < 0) return null;
        var he = raw.slice(0, open);
        var en = raw.slice(open + 2, close);
        if (!he || !en) return null;
        if (!/[\u0590-\u05FF]/.test(he) || !/[A-Za-z]/.test(en)) return null;
        return { he: he, en: en, suffix: '' };
    }

    function fillUi(node, text) {
        if (!node) return;
        var Ui = root.HebrewPlanUi;
        if (Ui && typeof Ui.fill === 'function') {
            Ui.fill(node, text);
            return;
        }
        var parts = splitUiLabel(text);
        while (node.firstChild) node.removeChild(node.firstChild);
        if (!parts) {
            node.textContent = text || '';
            return;
        }
        if (node.className.indexOf('ui-pair') === -1) node.className += ' ui-pair';
        var h = el('span');
        h.className = 'ui-he';
        h.lang = 'he';
        h.textContent = parts.he;
        var e = el('span');
        e.className = 'ui-en';
        e.lang = 'en';
        e.dir = 'ltr';
        e.textContent = parts.en;
        node.appendChild(h);
        node.appendChild(e);
        if (parts.suffix) node.appendChild(root.document.createTextNode(parts.suffix));
    }

    /** One chrome sentence in the teacher's language. Provenance: INIT-011/SPEC-003 */
    function uiLine(he, en) {
        var Ui = root.HebrewPlanUi;
        if (Ui && typeof Ui.line === 'function') return Ui.line(he, en);
        return en;
    }

    /**
     * A schema/validation refusal as a plain sentence: activity name and missing field,
     * never a JSON path. Provenance: INIT-011/SPEC-003
     * @param {object|null} data
     * @returns {string}
     */
    function refusalSentence(data) {
        var blocks = data && Array.isArray(data.blocks) ? data.blocks : [];
        var first = blocks[0];
        if (!first || typeof first !== 'object') {
            return uiLine('התוכנית דחתה את השינוי.', 'The plan refused this change.');
        }
        var message = typeof first.message === 'string' ? first.message : '';
        var path = typeof first.path === 'string' ? first.path : '';
        var field = '';
        var missing = /missing required field:\s*(\S+)/.exec(message);
        if (missing) field = missing[1];
        else if (typeof first.property === 'string' && first.property) field = first.property;
        else if (typeof first.field === 'string' && first.field && first.field.indexOf('/') === -1) {
            field = first.field;
        } else if (path) {
            var segs = path.split('/').filter(Boolean);
            var last = segs.length ? segs[segs.length - 1] : '';
            if (last && !/^\d+$/.test(last) && last.indexOf('/') === -1) field = last;
            else if (segs.length >= 2 && /^\d+$/.test(last)) field = segs[segs.length - 2];
        }
        // Prefer the activity being drafted (Add activity) over a path that may still
        // point at a neighbouring block index before the write lands.
        var activity = '';
        if (state && state.draft && state.draft.type) {
            activity = activityNameForType(state.draft.type);
        }
        if (!activity) activity = activityNameFromPath(path);
        if (activity && field && field.indexOf('/') === -1) {
            return uiLine(
                activity + ' חסרה את השדה ' + field + '.',
                activity + ' is missing the ' + field + ' field.'
            );
        }
        if (message && message.indexOf('/') === -1 && !/\$\.|instancePath|\/units\//.test(message)) {
            return message;
        }
        return uiLine('התוכנית דחתה את השינוי.', 'The plan refused this change.');
    }

    function activityNameForType(type) {
        var map = root.HebrewActivityMap && root.HebrewActivityMap.ACTIVITY_TO_BLOCK;
        if (!map || !type) return type || '';
        var name;
        for (name in map) {
            if (Object.prototype.hasOwnProperty.call(map, name) && map[name] === type) return name;
        }
        return type;
    }

    function activityNameFromPath(pointer) {
        if (!state || !state.plan || typeof pointer !== 'string' || !pointer) return '';
        var match = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)/.exec(pointer);
        if (!match) return '';
        var units = state.plan.units;
        var unit = Array.isArray(units) ? units[Number(match[1])] : null;
        var lesson = unit && Array.isArray(unit.lessons) ? unit.lessons[Number(match[2])] : null;
        var block = lesson && Array.isArray(lesson.blocks) ? lesson.blocks[Number(match[3])] : null;
        if (!block || typeof block.type !== 'string') return '';
        return activityNameForType(block.type);
    }

    function button(attr, value, text, locked) {
        var node = el('button');
        node.type = 'button';
        node.className = 'btn';
        node.setAttribute(attr, value);
        fillUi(node, text);
        if (locked) {
            node.disabled = true;
            node.setAttribute('data-plan-locked', '');
        }
        return node;
    }

    function messageNode(attr) {
        var node = el('p');
        node.className = 'admin-inline-error';
        node.setAttribute(attr, '');
        node.setAttribute('role', 'alert');
        node.hidden = true;
        return node;
    }

    function showBox(attr, text) {
        if (!state || !state.panel) return;
        var node = state.panel.querySelector('[' + attr + ']');
        if (!node) return;
        node.hidden = false;
        // Prefer .ui-pair when the string is bilingual chrome; otherwise one language via textContent.
        fillUi(node, text == null ? '' : String(text));
    }

    function hideBox(attr) {
        if (!state || !state.panel) return;
        var node = state.panel.querySelector('[' + attr + ']');
        if (!node) return;
        node.hidden = true;
        node.textContent = '';
    }

    function errorText(res) {
        if (res && typeof res.error === 'string' && res.error) {
            return typeof res.code === 'string' && res.code ? res.error + ' ' + res.code : res.error;
        }
        if (res && res.data && typeof res.data.error === 'string') return res.data.error;
        return 'request failed';
    }

    /**
     * Findings the save and publish routes already computed.
     * A 400 validation body carries the arrays at the top. A 200 carries them
     * on `validation`. Missing arrays are not a client-side cap check.
     * @param {object|null|undefined} data
     * @returns {{ blocks: object[], warnings: object[], informs: object[] }|null}
     */
    function findingsFrom(data) {
        if (!data || typeof data !== 'object') return null;
        var nested = data.validation;
        var source = nested && typeof nested === 'object' && !Array.isArray(nested) ? nested : data;
        var blocks = Array.isArray(source.blocks) ? source.blocks : null;
        var warnings = Array.isArray(source.warnings) ? source.warnings : null;
        var informs = Array.isArray(source.informs) ? source.informs : null;
        if (!blocks && !warnings && !informs) return null;
        return {
            blocks: blocks || [],
            warnings: warnings || [],
            informs: informs || []
        };
    }

    /**
     * @param {string} id
     * @param {string} status
     * @param {string} altText
     * @param {object[]} rows
     * @param {Record<string, boolean>} seen
     */
    function addLibraryRow(id, status, altText, rows, seen) {
        if (typeof id !== 'string' || !MEDIA_ID.test(id) || seen[id]) return;
        seen[id] = true;
        rows.push({
            id: id,
            status: status === 'pending' || status === 'approved' ? status : '',
            altText: typeof altText === 'string' ? altText : ''
        });
    }

    /**
     * Session uploads plus pictures already on introduction words.
     * There is no list route. Status is only what a media response returned.
     * @returns {{ id: string, status: string, altText: string }[]}
     */
    function libraryRows() {
        var rows = [];
        var seen = Object.create(null);
        var i;
        var stored = state && Array.isArray(state.media) ? state.media : [];
        for (i = 0; i < stored.length; i++) {
            var item = stored[i];
            if (!item) continue;
            addLibraryRow(item.id, item.status, item.altText, rows, seen);
        }
        var blocks = blocksOf(state && state.lesson);
        for (i = 0; i < blocks.length; i++) {
            var block = blocks[i];
            if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) continue;
            var w;
            for (w = 0; w < block.words.length; w++) {
                var image = block.words[w] && block.words[w].image;
                if (!image || typeof image !== 'object' || Array.isArray(image)) continue;
                addLibraryRow(image.mediaId, '', image.alt, rows, seen);
            }
        }
        return rows;
    }

    /**
     * @param {object|null|undefined} row
     */
    function rememberMedia(row) {
        if (!state || !row || typeof row.id !== 'string' || !MEDIA_ID.test(row.id)) return;
        var status = row.status === 'pending' || row.status === 'approved' ? row.status : '';
        var altText = typeof row.altText === 'string' ? row.altText : '';
        var i;
        for (i = 0; i < state.media.length; i++) {
            if (state.media[i] && state.media[i].id === row.id) {
                state.media[i] = {
                    id: row.id,
                    status: status || state.media[i].status,
                    altText: altText || state.media[i].altText
                };
                return;
            }
        }
        state.media.push({ id: row.id, status: status, altText: altText });
    }

    function syncGenerate() {
        if (!state || !state.panel) return;
        var btn = state.panel.querySelector('[data-plan-generate]');
        var box = state.panel.querySelector('[data-plan-confirm]');
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        if (!btn) return;
        var prompt = promptInput ? String(promptInput.value || '') : '';
        var ready = state.estimateShown === true
            && typeof state.estimateDollars === 'number'
            && Number.isFinite(state.estimateDollars)
            && !!box
            && box.checked === true
            && prompt === state.estimatePrompt;
        btn.disabled = !ready;
    }

    function syncApprove() {
        if (!state || !state.panel) return;
        var btn = state.panel.querySelector('[data-plan-approve]');
        var altInput = state.panel.querySelector('[data-plan-approve-alt]');
        if (!btn) return;
        var alt = altInput ? String(altInput.value || '').trim() : '';
        btn.disabled = alt.length === 0;
    }

    function showError(res) {
        showBox('data-plan-error', errorText(res));
    }

    /**
     * Name the conflict and the revision the server stored. Leave every textarea alone.
     * @param {object} data
     */
    function showConflict(data) {
        var name = data && typeof data.error === 'string' ? data.error : 'revision_conflict';
        var revision = data && Object.prototype.hasOwnProperty.call(data, 'revision') ? data.revision : '';
        hideBox('data-plan-error');
        showBox('data-plan-conflict', name + ' ' + String(revision));
    }

    function paintMessage(text) {
        empty(state.panel);
        var box = el('div');
        box.setAttribute('data-plan-builder', '');
        box.setAttribute('data-admin-view', 'plan');
        box.setAttribute('dir', 'rtl');
        box.setAttribute('lang', 'he');
        var p = messageNode('data-plan-error');
        p.hidden = false;
        p.textContent = text;
        box.appendChild(p);
        state.panel.appendChild(box);
    }

    /**
     * Picture choice exists only on an introduction word.
     * @param {HTMLElement} parent
     * @param {object} block
     */
    function paintPictures(parent, block) {
        if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) return;
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-pictures', '');
        var heading = el('h3');
        fillUi(heading, 'תמונה (Picture)');
        box.appendChild(heading);
        var rows = libraryRows();
        var w;
        for (w = 0; w < block.words.length; w++) {
            var word = block.words[w];
            if (!word || typeof word.id !== 'string' || !ENTRY.test(word.id)) continue;
            var row = el('div');
            row.setAttribute('data-plan-word', word.id);
            var name = el('span');
            name.textContent = typeof word.word === 'string' ? word.word : word.id;
            row.appendChild(name);
            var select = el('select');
            select.setAttribute('data-plan-attach-media', '');
            var current = word.image && typeof word.image.mediaId === 'string' ? word.image.mediaId : '';
            var choices = rows.slice();
            var have = false;
            var c;
            for (c = 0; c < choices.length; c++) {
                if (choices[c].id === current) have = true;
            }
            if (current && MEDIA_ID.test(current) && !have) {
                choices.unshift({ id: current, status: '', altText: '' });
            }
            var emptyOpt = el('option');
            emptyOpt.value = '';
            emptyOpt.textContent = '—';
            select.appendChild(emptyOpt);
            var o;
            for (o = 0; o < choices.length; o++) {
                var opt = el('option');
                opt.value = choices[o].id;
                opt.textContent = choices[o].id;
                select.appendChild(opt);
            }
            select.value = current && MEDIA_ID.test(current) ? current : '';
            var alt = el('input');
            alt.type = 'text';
            alt.setAttribute('data-plan-attach-alt', '');
            alt.value = word.image && typeof word.image.alt === 'string' ? word.image.alt : '';
            row.appendChild(labeled('מזהה תמונה (Picture)', select));
            row.appendChild(labeled('טקסט חלופי (Alt)', alt));
            row.appendChild(button('data-plan-attach', word.id, 'שמירת תמונה (Save picture)', false));
            box.appendChild(row);
        }
        parent.appendChild(box);
    }

    /**
     * Pending and approved pictures, upload, dry-run, and approval.
     * Generate stays disabled until the dry-run number is on screen and confirmed.
     * @param {HTMLElement} parent
     */
    function paintMedia(parent) {
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-media', '');
        var heading = el('h3');
        fillUi(heading, 'ספריית תמונות (Media)');
        box.appendChild(heading);

        var list = el('ul');
        list.setAttribute('data-plan-media-list', '');
        var rows = libraryRows();
        var i;
        for (i = 0; i < rows.length; i++) {
            var item = el('li');
            item.setAttribute('data-plan-media-item', rows[i].id);
            var label = el('span');
            label.textContent = rows[i].id;
            item.appendChild(label);
            if (rows[i].status) {
                var status = el('span');
                status.setAttribute('data-plan-media-status', rows[i].status);
                status.textContent = rows[i].status;
                item.appendChild(status);
            }
            var img = el('img');
            img.alt = rows[i].altText;
            img.setAttribute('src', '/api/media/' + rows[i].id);
            item.appendChild(img);
            list.appendChild(item);
        }
        box.appendChild(list);

        var uploadId = el('input');
        uploadId.type = 'text';
        uploadId.setAttribute('data-plan-upload-id', '');
        uploadId.setAttribute('autocomplete', 'off');
        var uploadAlt = el('input');
        uploadAlt.type = 'text';
        uploadAlt.setAttribute('data-plan-upload-alt', '');
        var file = el('input');
        file.type = 'file';
        file.setAttribute('data-plan-upload-file', '');
        file.setAttribute('accept', 'image/png,image/jpeg,image/webp');
        box.appendChild(labeled('מזהה העלאה (Upload id)', uploadId));
        box.appendChild(labeled('טקסט חלופי (Alt)', uploadAlt));
        box.appendChild(labeled('קובץ (File)', file));
        box.appendChild(button('data-plan-upload', '', 'העלאה (Upload)', false));

        var prompt = el('textarea');
        prompt.setAttribute('data-plan-gen-prompt', '');
        prompt.setAttribute('dir', 'ltr');
        prompt.setAttribute('lang', 'en');
        prompt.rows = 3;
        prompt.value = typeof state.estimatePrompt === 'string' ? state.estimatePrompt : '';
        var genId = el('input');
        genId.type = 'text';
        genId.setAttribute('data-plan-gen-id', '');
        genId.setAttribute('autocomplete', 'off');
        box.appendChild(labeled('בקשה (Prompt)', prompt));
        box.appendChild(labeled('מזהה יצירה (Generate id)', genId));
        box.appendChild(button('data-plan-dry-run', '', 'הערכה (Estimate)', false));

        var estimate = el('p');
        estimate.setAttribute('data-plan-estimate', '');
        if (state.estimateShown && typeof state.estimateDollars === 'number') {
            estimate.hidden = false;
            estimate.textContent = String(state.estimateDollars);
        } else {
            estimate.hidden = true;
            estimate.textContent = '';
        }
        box.appendChild(estimate);

        var confirm = el('input');
        confirm.type = 'checkbox';
        confirm.checked = false;
        confirm.setAttribute('data-plan-confirm', '');
        box.appendChild(labeled('אישור הסכום (Confirm estimate)', confirm));
        box.appendChild(button('data-plan-generate', '', 'יצירה (Generate)', true));

        var approveId = el('input');
        approveId.type = 'text';
        approveId.setAttribute('data-plan-approve-id', '');
        approveId.setAttribute('autocomplete', 'off');
        var approveAlt = el('input');
        approveAlt.type = 'text';
        approveAlt.setAttribute('data-plan-approve-alt', '');
        box.appendChild(labeled('מזהה לאישור (Approve id)', approveId));
        box.appendChild(labeled('טקסט חלופי לאישור (Approval alt)', approveAlt));
        box.appendChild(button('data-plan-approve', '', 'אישור (Approve)', true));

        parent.appendChild(box);
    }

    /**
     * The validation panel lives in a stable host so a failed save can refresh it
     * without repainting the page (a repaint would drop the issues shown at the fields).
     * @param {HTMLElement} parent
     */
    function paintValidation(parent) {
        var host = el('div');
        host.setAttribute('data-plan-validation-host', '');
        state.validationHost = host;
        fillValidation(host);
        parent.appendChild(host);
    }

    function refreshValidation() {
        if (state && state.validationHost) fillValidation(state.validationHost);
    }

    /**
     * @param {HTMLElement} host
     */
    function fillValidation(host) {
        empty(host);
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-validation', '');
        var heading = el('h3');
        fillUi(heading, 'בדיקה (Validation)');
        box.appendChild(heading);
        if (state.publishRefused) {
            var refused = el('p');
            refused.setAttribute('data-plan-publish-refusal', '');
            refused.setAttribute('role', 'status');
            var code = typeof state.publishRefusal === 'string' ? state.publishRefusal : '';
            refused.textContent = 'snapshot was not published: ' + code;
            box.appendChild(refused);
        }
        var groups = [
            ['block', state.validation && state.validation.blocks],
            ['warn', state.validation && state.validation.warnings],
            ['inform', state.validation && state.validation.informs]
        ];
        var g;
        for (g = 0; g < groups.length; g++) {
            var severity = groups[g][0];
            var items = groups[g][1] || [];
            var list = el('ul');
            list.setAttribute('data-plan-findings', severity);
            var n;
            for (n = 0; n < items.length; n++) {
                var finding = items[n];
                if (!finding || typeof finding !== 'object') continue;
                var rule = typeof finding.rule === 'string' ? finding.rule : '';
                if (!rule) continue;
                var li = el('li');
                li.setAttribute('data-plan-finding', rule);
                var message = typeof finding.message === 'string' ? finding.message : '';
                li.textContent = rule + ' ' + severity + ' ' + message;
                list.appendChild(li);
            }
            box.appendChild(list);
        }
        host.appendChild(box);
    }

    /**
     * Findings from the last save or publish, flattened with their severity, for the
     * inline issues at the fields.
     * @returns {object[]}
     */
    function validationList() {
        var v = state && state.validation;
        if (!v) return [];
        var out = [];
        [['block', v.blocks], ['warn', v.warnings], ['inform', v.informs]].forEach(function (group) {
            (group[1] || []).forEach(function (finding) {
                if (!finding || typeof finding !== 'object') return;
                var copy = {};
                Object.keys(finding).forEach(function (key) { copy[key] = finding[key]; });
                copy.severity = copy.severity || group[0];
                out.push(copy);
            });
        });
        return out;
    }

    function blockLabel(type) {
        var hints = root.HebrewPlanHints;
        var hint = hints && hints.blocks && hints.blocks[type];
        return hint ? hint.label[0] + ' (' + hint.label[1] + ')' : String(type || '');
    }

    /**
     * A section the admin opens and closes. The body is built only while it is open,
     * so a closed plan or unit form costs nothing; unsaved text in a closed form is
     * kept as a draft by the editor.
     */
    function disclosure(name, he, en, build) {
        var open = state.open[name] === true;
        var card = el('section');
        card.className = 'card';
        card.setAttribute('data-plan-section', name);
        var heading = el('h3');
        var toggle = button('data-plan-disclose', name, (open ? '▾ ' : '▸ ') + he + ' (' + en + ')', false);
        toggle.className += ' btn-ghost plan-disclose';
        toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
        toggle.setAttribute('data-plan-focus-key', 'disclose:' + name);
        heading.appendChild(toggle);
        card.appendChild(heading);
        if (open) {
            var body = build();
            if (body) card.appendChild(body);
        }
        return card;
    }

    function unitPathOf(lessonPath) {
        var segs = String(lessonPath || '').split('/');
        return segs.length >= 2 ? segs[0] + '/' + segs[1] : '';
    }

    function renderNewDraft(type) {
        var host = state.newDraftHost;
        if (!host) return;
        empty(host);
        state.draft = null;
        var tools = state.editor.tools();
        var def = tools ? tools.blockDef(type) : null;
        if (!def) return;
        var required = tools.requiredOf(tools.defSchema(def)).filter(function (prop) {
            return prop !== 'id' && prop !== 'type';
        });
        // Words also offers fill-in-the-blank (cloze) in the same form (INIT-011/SPEC-005).
        var fields = required.slice();
        if (type === 'introduction' && fields.indexOf('cloze') === -1) fields.push('cloze');
        if (!fields.length) return;
        var seed = form().newBlock(type, 'new');
        if (!seed) return;
        if (type === 'conjugation-table' && state.patternFill && state.patternFill.cells && state.patternFill.cells.length) {
            seed.lemmaKey = state.patternFill.lemmaKey;
            seed.tense = state.patternFill.tense;
            seed.mode = seed.mode || 'study';
            seed.cells = state.patternFill.cells.map(function (cell) {
                return { cell: cell.cell, formKey: cell.formKey };
            });
        }
        if (type === 'introduction' && !seed.cloze) {
            seed.cloze = {
                items: [{ id: 'c1', textPre: '', textPost: '', answer: '' }]
            };
        }
        if (type === 'exercise' && root.HebrewPlanForm && typeof root.HebrewPlanForm.freshExerciseItem === 'function') {
            var ek = seed.kind || 'choose';
            var t = form() && typeof form().tools === 'function' ? form().tools() : null;
            seed.items = [root.HebrewPlanForm.freshExerciseItem(ek, t, 'item-1')];
        }
        var handle = state.editor.renderDraft(def, seed, fields);
        host.appendChild(handle.el);
        if (type === 'conjugation-table' && state.patternFill && state.patternFill.cells && state.patternFill.cells.length) {
            var filled = el('p');
            filled.className = 'plan-note';
            filled.setAttribute('data-plan-verb-filled', '');
            var tenseEn = state.patternFill.tense === 'past' ? 'past' : 'present';
            var tenseHe = state.patternFill.tense === 'past' ? 'עבר' : 'הווה';
            var gloss = state.patternFill.gloss ? ' — ' + state.patternFill.gloss : '';
            filled.textContent = uiLine(
                'המילון מילא את ' + tenseHe + gloss,
                'Dictionary filled the ' + tenseEn + ' table' + gloss
            );
            host.appendChild(filled);
        }
        state.draft = { type: type, handle: handle };
    }

    function selectedActivityName(select) {
        if (!select) return '';
        var value = String(select.value || '');
        var kids = select.children || [];
        var i;
        for (i = 0; i < kids.length; i++) {
            var opt = kids[i];
            if (!opt || !opt.getAttribute) continue;
            if (value && String(opt.value || '') === value) {
                return opt.getAttribute('data-plan-activity-name') || '';
            }
        }
        if (kids[0] && kids[0].getAttribute) {
            return kids[0].getAttribute('data-plan-activity-name') || '';
        }
        return '';
    }

    function syncActivityHint(select, hintNode) {
        var hint = hintNode || (state && state.panel && state.panel.querySelector('[data-plan-activity-hint]'));
        if (!hint || !select) return;
        var name = selectedActivityName(select);
        var pair = activityHint(name);
        empty(hint);
        if (!pair.he && !pair.en) {
            hint.hidden = true;
            return;
        }
        hint.hidden = false;
        hint.className = 'plan-note plan-activity-hint ui-pair';
        hint.setAttribute('data-plan-activity-hint', name);
        var he = el('span');
        he.className = 'ui-he';
        he.lang = 'he';
        he.textContent = pair.he;
        var en = el('span');
        en.className = 'ui-en';
        en.lang = 'en';
        en.setAttribute('dir', 'ltr');
        en.textContent = pair.en;
        hint.appendChild(he);
        hint.appendChild(en);
    }

    function paintInsert(parent) {
        var insert = el('div');
        insert.className = 'card';
        insert.setAttribute('data-plan-newblock', '');
        insert.setAttribute('data-plan-activity-menu', '');
        var heading = el('h3');
        fillUi(heading, 'פעילות חדשה (Add activity)');
        insert.appendChild(heading);
        var typeSelect = el('select');
        typeSelect.setAttribute('data-plan-new-type', '');
        typeSelect.setAttribute('data-plan-new-activity', '');
        typeSelect.setAttribute('aria-label', readUiLang() === 'he' ? 'פעילות' : 'Activity');
        var names = activityNames();
        var t;
        for (t = 0; t < names.length; t++) {
            var name = names[t];
            var blockType = blockTypeForActivity(name);
            if (!blockType) continue;
            var opt = el('option');
            opt.value = blockType;
            opt.setAttribute('data-plan-activity-name', name);
            fillOptionUi(opt, ACTIVITY_HE[name] || name, name);
            typeSelect.appendChild(opt);
        }
        if (names.length) {
            typeSelect.value = blockTypeForActivity(names[0]) || '';
        }
        insert.appendChild(labeled('פעילות (Activity)', typeSelect));
        var hint = el('p');
        hint.className = 'plan-note plan-activity-hint ui-pair';
        hint.setAttribute('data-plan-activity-hint', '');
        insert.appendChild(hint);
        var note = el('p');
        note.className = 'plan-note ui-pair';
        var noteHe = el('span');
        noteHe.className = 'ui-he';
        noteHe.lang = 'he';
        noteHe.textContent = 'בוחרים פעילות וממלאים — כלום לא מוכנס מראש.';
        var noteEn = el('span');
        noteEn.className = 'ui-en';
        noteEn.lang = 'en';
        noteEn.setAttribute('dir', 'ltr');
        noteEn.textContent = 'Pick an activity and fill it in — nothing is pre-inserted.';
        note.appendChild(noteHe);
        note.appendChild(noteEn);
        insert.appendChild(note);
        var verbFind = el('div');
        verbFind.className = 'plan-verb-find';
        verbFind.setAttribute('data-plan-verb-find', '');
        verbFind.hidden = typeSelect.value !== 'conjugation-table';
        var verbLabel = el('p');
        verbLabel.className = 'plan-note ui-pair';
        var verbHe = el('span');
        verbHe.className = 'ui-he';
        verbHe.lang = 'he';
        verbHe.textContent = 'חפשו פועל שכבר במילון והטבלה מתמלאת. כרגע יש לחשוב, להרגיש, לשאול, לענות.';
        var verbEn = el('span');
        verbEn.className = 'ui-en';
        verbEn.lang = 'en';
        verbEn.setAttribute('dir', 'ltr');
        verbEn.textContent = 'Search a verb already in the dictionary and the table fills in. So far: to think, to feel, to ask, to answer.';
        verbLabel.appendChild(verbHe);
        verbLabel.appendChild(verbEn);
        verbFind.appendChild(verbLabel);
        var verbInput = el('input');
        verbInput.type = 'text';
        verbInput.setAttribute('data-plan-verb-query', '');
        verbInput.setAttribute('autocomplete', 'off');
        verbInput.setAttribute('aria-label', readUiLang() === 'he' ? 'חיפוש פועל' : 'Search for a verb');
        verbInput.setAttribute('placeholder', readUiLang() === 'he' ? 'לחשוב או think' : 'think or לחשוב');
        verbFind.appendChild(verbInput);
        var verbHits = el('ul');
        verbHits.className = 'plan-lexicon-hits';
        verbHits.setAttribute('data-plan-verb-hits', '');
        verbHits.hidden = true;
        verbFind.appendChild(verbHits);
        var tenseRow = el('div');
        tenseRow.className = 'plan-verb-tenses';
        tenseRow.setAttribute('data-plan-verb-tenses', '');
        tenseRow.hidden = true;
        tenseRow.appendChild(button('data-plan-verb-tense', 'present', 'הווה (Present)', false));
        tenseRow.appendChild(button('data-plan-verb-tense', 'past', 'עבר (Past)', false));
        verbFind.appendChild(tenseRow);
        insert.appendChild(verbFind);
        var draftHost = el('div');
        draftHost.setAttribute('data-plan-new-draft', '');
        state.newDraftHost = draftHost;
        insert.appendChild(draftHost);
        var issues = el('ul');
        issues.className = 'plan-issues';
        issues.setAttribute('data-plan-new-issues', '');
        insert.appendChild(issues);
        insert.appendChild(button('data-plan-insert', '', 'שמירה (Save)', !state.lessonPath));
        parent.appendChild(insert);
        // Pass the hint node: paintInsert may run before rootEl is on the panel.
        syncActivityHint(typeSelect, hint);
        syncVerbFinder(typeSelect, insert);
        // Draft fields for the selected type only; the lesson is unchanged until Save.
        if (typeSelect.value && state.editor.hasSchema()) {
            renderNewDraft(typeSelect.value);
        }
    }

    /**
     * Words earlier lessons added, in stack order, marked with the lesson that added each.
     * @param {HTMLElement} parent
     */
    function paintEarlierWords(parent) {
        if (!state.plan || !LESSON_PATH.test(state.lessonPath || '') || !form()) return;
        var earlier = form().earlierLessonsInPlan(state.plan, state.lessonPath);
        if (!earlier.length) return;
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-earlier-words', '');
        var heading = el('h3');
        fillUi(heading, 'מילים משיעורים קודמים (Words from earlier lessons)');
        box.appendChild(heading);
        var list = el('ol');
        list.setAttribute('data-plan-earlier-words-list', '');
        var e;
        for (e = 0; e < earlier.length; e++) {
            var choice = earlier[e];
            var lessonNode = form().nodeAt(state.plan, choice.path);
            var words = [];
            var seen = {};
            if (lessonNode && Array.isArray(lessonNode.newWords)) {
                lessonNode.newWords.forEach(function (w) {
                    if (typeof w === 'string' && w && !seen[w]) {
                        seen[w] = true;
                        words.push(w);
                    }
                });
            }
            if (lessonNode && Array.isArray(lessonNode.blocks)) {
                lessonNode.blocks.forEach(function (block) {
                    if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) return;
                    block.words.forEach(function (entry) {
                        var w = entry && typeof entry.word === 'string' ? entry.word : '';
                        if (w && !seen[w]) {
                            seen[w] = true;
                            words.push(w);
                        }
                    });
                });
            }
            var w;
            for (w = 0; w < words.length; w++) {
                var li = el('li');
                li.setAttribute('data-plan-earlier-word', words[w]);
                li.setAttribute('data-from-lesson', choice.id);
                var wordSpan = el('span');
                wordSpan.setAttribute('lang', 'he');
                wordSpan.setAttribute('dir', 'rtl');
                wordSpan.textContent = words[w];
                li.appendChild(wordSpan);
                var mark = el('span');
                mark.className = 'plan-block-id';
                mark.setAttribute('dir', 'ltr');
                mark.textContent = choice.title || choice.id;
                li.appendChild(mark);
                list.appendChild(li);
            }
        }
        box.appendChild(list);
        parent.appendChild(box);
    }

    function showNewIssues(data) {
        if (!state || !state.panel) return;
        var list = state.panel.querySelector('[data-plan-new-issues]');
        if (!list) return;
        empty(list);
        var all = [];
        [['block', data.blocks], ['warn', data.warnings], ['inform', data.informs]].forEach(function (group) {
            (Array.isArray(group[1]) ? group[1] : []).forEach(function (finding) {
                if (finding && typeof finding === 'object') all.push({ severity: group[0], finding: finding });
            });
        });
        all.forEach(function (item) {
            var li = el('li');
            li.className = 'plan-issue plan-issue-' + item.severity;
            li.setAttribute('data-plan-issue', typeof item.finding.rule === 'string' ? item.finding.rule : '');
            var where = typeof item.finding.path === 'string' ? item.finding.path.split('/').slice(-2).join('/') : '';
            li.textContent = item.severity + ' · ' + (item.finding.rule || '') + ' — '
                + (item.finding.message || '') + (where ? ' (' + where + ')' : '');
            list.appendChild(li);
        });
    }

    function paint() {
        var panel = state.panel;
        var editor = state.editor;
        var lesson = state.lesson;
        var blocks = blocksOf(lesson);
        var selected = state.selectedId ? blockById(state.selectedId) : null;
        if (state.selectedId && !selected) state.selectedId = null;
        // The empty fluency placeholder is never the teacher's selected activity.
        if (selected && selected.block && selected.block.id === 'placeholder') {
            state.selectedId = null;
            selected = null;
        }
        if (!selected && state.screen === 'lesson') {
            var firstListed = listedBlocks(lesson)[0];
            if (firstListed) {
                state.selectedId = firstListed.id;
                selected = blockById(firstListed.id);
            }
        }

        // Keep unsaved text, the open sections, focus, and scroll across the repaint.
        var focus = editor.focusKey();
        editor.captureDrafts();
        editor.reset();
        var scrollY = typeof root.scrollY === 'number' ? root.scrollY : null;

        // The block form is made first, so its field buttons are numbered first.
        var ready = state.schema === 'ready';
        var blockForm = null;
        if (selected && ready && LESSON_PATH.test(state.lessonPath)) {
            blockForm = editor.renderNode({
                path: state.lessonPath + '/blocks/' + selected.block.id,
                node: selected.block,
                kind: 'block'
            });
        }
        var lessonCard = null;
        var unitCard = null;
        var planCard = null;
        if (ready && state.plan) {
            if (lesson && LESSON_PATH.test(state.lessonPath)) {
                lessonCard = disclosure('lesson', 'פרטי השיעור', 'Lesson details', function () {
                    return editor.renderNode({ path: state.lessonPath, node: lesson, kind: 'lesson' }).el;
                });
                unitCard = disclosure('unit', 'פרטי היחידה', 'Unit details', function () {
                    var path = unitPathOf(state.lessonPath);
                    var node = form().nodeAt(state.plan, path);
                    return node ? editor.renderNode({ path: path, node: node, kind: 'unit' }).el : null;
                });
            }
            planCard = disclosure('plan', 'פרטי התוכנית', 'Plan details', function () {
                return editor.renderNode({ path: '', node: state.plan, kind: 'plan' }).el;
            });
        }

        empty(panel);
        var rootEl = el('div');
        rootEl.className = 'plan-builder';
        rootEl.setAttribute('data-plan-builder', '');
        rootEl.setAttribute('data-admin-view', 'plan');
        rootEl.setAttribute('dir', 'rtl');
        rootEl.setAttribute('lang', 'he');

        var back = el('p');
        back.className = 'admin-back plan-crumb';
        back.setAttribute('data-plan-crumb', '');
        var backLink = el('a');
        backLink.setAttribute('href', '#plans');
        var backArrow = el('span');
        backArrow.textContent = '← ';
        var backName = el('span');
        fillUi(backName, 'תוכניות (Plans)');
        backLink.appendChild(backArrow);
        backLink.appendChild(backName);
        back.appendChild(backLink);
        if (state.screen === 'lesson' && state.lessonPath) {
            var crumbSep = el('span');
            crumbSep.textContent = ' / ';
            back.appendChild(crumbSep);
            var crumbPlan = el('button');
            crumbPlan.type = 'button';
            crumbPlan.className = 'btn btn-ghost plan-crumb-plan';
            crumbPlan.setAttribute('data-plan-crumb-plan', '');
            fillUi(crumbPlan, 'תוכנית (Plan)');
            back.appendChild(crumbPlan);
            var crumbLesson = el('span');
            crumbLesson.className = 'plan-crumb-here';
            crumbLesson.setAttribute('data-plan-crumb-lesson', '');
            crumbLesson.textContent = ' / ' + (lesson && typeof lesson.title === 'string' ? lesson.title : '');
            back.appendChild(crumbLesson);
        } else {
            var crumbHere = el('span');
            crumbHere.className = 'plan-crumb-here';
            crumbHere.textContent = ' / Plan builder';
            back.appendChild(crumbHere);
        }
        rootEl.appendChild(back);

        var toolbar = el('div');
        toolbar.className = 'plan-head';
        var title = el('h2');
        title.textContent = state.plan && typeof state.plan.title === 'string' ? state.plan.title : state.planId;
        var rev = el('p');
        rev.setAttribute('data-plan-revision', '');
        rev.textContent = Number.isInteger(state.revision) ? String(state.revision) : '';
        var status = el('p');
        status.setAttribute('data-plan-status', '');
        status.textContent = state.plan && typeof state.plan.status === 'string' ? state.plan.status : '';
        var headRow = el('div');
        headRow.className = 'plan-head-row';
        var titleBlock = el('div');
        titleBlock.className = 'plan-head-copy';
        titleBlock.appendChild(title);
        var forLine = el('p');
        forLine.className = 'plan-for ui-pair';
        forLine.setAttribute('data-plan-for', '');
        var forText = forAudienceLine(state.plan);
        var forHe = el('span');
        forHe.className = 'ui-he';
        forHe.setAttribute('lang', 'he');
        forHe.textContent = forText.he;
        var forEn = el('span');
        forEn.className = 'ui-en';
        forEn.setAttribute('lang', 'en');
        forEn.setAttribute('dir', 'ltr');
        forEn.textContent = forText.en;
        forLine.appendChild(forHe);
        forLine.appendChild(forEn);
        titleBlock.appendChild(forLine);
        headRow.appendChild(titleBlock);
        var actions = el('div');
        actions.className = 'plan-head-actions';
        var draft = el('div');
        draft.className = 'plan-draft-state';
        draft.appendChild(status);
        draft.appendChild(rev);
        actions.appendChild(draft);
        var reloadBtn = button('data-plan-reload', '', 'טעינה מחדש (Reload)', false);
        reloadBtn.className += ' btn-ghost';
        var publishBtn = button('data-plan-publish', '', 'פרסום (Publish)', false);
        publishBtn.className += ' btn-primary plan-publish';
        var publishIcon = el('img');
        publishIcon.className = 'plan-publish-icon';
        publishIcon.setAttribute('src', '/shared/brand/icon-publish.svg');
        publishIcon.setAttribute('alt', '');
        publishIcon.setAttribute('width', '14');
        publishIcon.setAttribute('height', '14');
        publishBtn.appendChild(publishIcon);
        actions.appendChild(reloadBtn);
        actions.appendChild(publishBtn);
        headRow.appendChild(actions);
        toolbar.appendChild(headRow);
        if (state.panels.issues) {
            // What the publish will meet, and that warnings never stop it. No approval step.
            var publishNote = el('p');
            publishNote.className = 'plan-note';
            publishNote.setAttribute('data-plan-publish-note', '');
            fillUi(publishNote, state.publishNote || '');
            toolbar.appendChild(publishNote);
        }
        var notice = el('p');
        notice.setAttribute('data-plan-notice', '');
        notice.setAttribute('role', 'status');
        fillUi(notice, state.notice || '');
        toolbar.appendChild(notice);
        rootEl.appendChild(toolbar);

        rootEl.appendChild(messageNode('data-plan-conflict'));
        rootEl.appendChild(messageNode('data-plan-error'));

        if (state.panels.assign) {
            state.panels.assign.update(state.plan);
            rootEl.appendChild(state.panels.assign.el);
        }

        var chooser = el('div');
        chooser.className = 'card plan-sequence';
        chooser.setAttribute('data-plan-lessons', '');
        chooser.setAttribute('data-plan-stack', '');
        var stackHeading = el('h3');
        fillUi(stackHeading, 'שיעורים (Lessons)');
        chooser.appendChild(stackHeading);
        var stack = el('ol');
        stack.setAttribute('data-plan-stack-list', '');
        var c;
        for (c = 0; c < state.lessonChoices.length; c++) {
            var choice = state.lessonChoices[c];
            var lessonNode = form() && form().nodeAt ? form().nodeAt(state.plan, choice.path) : null;
            var row = el('li');
            row.className = 'plan-lesson-row';
            row.setAttribute('data-plan-stack-row', choice.id);
            if (choice.path === state.lessonPath) {
                row.setAttribute('aria-current', 'true');
                row.className += ' plan-lesson-row-open';
            }
            var copy = el('div');
            copy.className = 'plan-lesson-row-copy';
            var titleEl = el('strong');
            titleEl.className = 'plan-lesson-row-title';
            titleEl.setAttribute('data-plan-lesson-title', '');
            // Lesson title is content, not chrome — keep the stored language.
            titleEl.textContent = choice.title || choice.id;
            copy.appendChild(titleEl);
            var sub = el('p');
            sub.className = 'plan-lesson-row-subtitle';
            sub.setAttribute('data-plan-lesson-subtitle', '');
            var subText = lessonNode && typeof lessonNode.subtitle === 'string' ? lessonNode.subtitle : '';
            sub.textContent = subText;
            if (!subText) sub.hidden = true;
            copy.appendChild(sub);
            var wordsLine = el('p');
            wordsLine.className = 'plan-lesson-row-words';
            wordsLine.setAttribute('data-plan-lesson-words', '');
            var introWords = wordsIntroducedByLesson(lessonNode);
            wordsLine.textContent = introWords.join(' · ');
            if (!introWords.length) wordsLine.hidden = true;
            copy.appendChild(wordsLine);
            row.appendChild(copy);
            var ops = el('div');
            ops.className = 'plan-lesson-row-ops';
            ops.appendChild(button('data-plan-lesson-move', 'up', 'למעלה (Up)', c === 0));
            ops.appendChild(button(
                'data-plan-lesson-move',
                'down',
                'למטה (Down)',
                c >= state.lessonChoices.length - 1
            ));
            var openBtn = button('data-plan-lesson', String(c), 'פתיחת שיעור (Open lesson)', false);
            if (choice.path === state.lessonPath) openBtn.setAttribute('aria-current', 'true');
            ops.appendChild(openBtn);
            ops.appendChild(button(
                'data-plan-lesson-remove',
                '',
                'הסרה (Remove)',
                state.lessonChoices.length <= 1
            ));
            row.appendChild(ops);
            stack.appendChild(row);
        }
        chooser.appendChild(stack);

        var addBox = el('div');
        addBox.className = 'plan-add-lesson';
        addBox.setAttribute('data-plan-add-lesson-form', '');
        var addTitle = el('input');
        addTitle.type = 'text';
        addTitle.setAttribute('data-plan-add-lesson-title', '');
        addTitle.setAttribute('autocomplete', 'off');
        addTitle.setAttribute('placeholder', readUiLang() === 'he' ? 'כותרת השיעור' : 'Lesson title');
        addTitle.setAttribute('aria-label', readUiLang() === 'he' ? 'כותרת השיעור' : 'Lesson title');
        var addSub = el('input');
        addSub.type = 'text';
        addSub.setAttribute('data-plan-add-lesson-subtitle', '');
        addSub.setAttribute('autocomplete', 'off');
        addSub.setAttribute('placeholder', readUiLang() === 'he' ? 'משפט אחד (רשות)' : 'One sentence (optional)');
        addSub.setAttribute('aria-label', readUiLang() === 'he' ? 'משפט אחד' : 'One sentence');
        addBox.appendChild(addTitle);
        addBox.appendChild(addSub);
        addBox.appendChild(button('data-plan-add-lesson', '', 'שיעור חדש (Add lesson)', !state.plan));
        addBox.appendChild(button('data-plan-reuse-lesson-open', '', 'שימוש חוזר (Reuse lesson)', !state.plan));
        chooser.appendChild(addBox);

        var reuseBox = el('div');
        reuseBox.className = 'plan-reuse-lesson';
        reuseBox.setAttribute('data-plan-reuse-form', '');
        reuseBox.hidden = state.reuseOpen !== true;
        var reuseSelect = el('select');
        reuseSelect.setAttribute('data-plan-reuse-select', '');
        reuseSelect.setAttribute('aria-label', readUiLang() === 'he' ? 'שיעור לשימוש חוזר' : 'Lesson to reuse');
        var reusePlaceholder = el('option');
        reusePlaceholder.value = '';
        fillOptionUi(reusePlaceholder, 'בחירת שיעור…', 'Choose a lesson…');
        reuseSelect.appendChild(reusePlaceholder);
        (state.reuseOptions || []).forEach(function (opt) {
            var o = el('option');
            o.value = opt.key;
            o.textContent = opt.label;
            reuseSelect.appendChild(o);
        });
        reuseBox.appendChild(reuseSelect);
        reuseBox.appendChild(button('data-plan-reuse-lesson', '', 'הכנסה (Insert copy)', !state.plan));
        chooser.appendChild(reuseBox);

        var bank = el('div');
        bank.className = 'card plan-word-bank';
        bank.setAttribute('data-plan-word-bank', '');
        var bankHeading = el('h3');
        fillUi(bankHeading, 'בנק מילים (Word bank)');
        bank.appendChild(bankHeading);
        var bankList = el('ol');
        bankList.setAttribute('data-plan-word-bank-list', '');
        var bankRows = planWordBank(state.plan);
        var bi;
        for (bi = 0; bi < bankRows.length; bi++) {
            var bw = bankRows[bi];
            var bli = el('li');
            bli.setAttribute('data-plan-bank-word', bw.word);
            bli.setAttribute('data-from-lesson', bw.lessonId);
            var bwSpan = el('button');
            bwSpan.type = 'button';
            bwSpan.className = 'lookup-word';
            bwSpan.setAttribute('data-lookup-word', bw.word);
            bwSpan.setAttribute('lang', 'he');
            bwSpan.setAttribute('dir', 'rtl');
            bwSpan.textContent = bw.word;
            bli.appendChild(bwSpan);
            var from = el('span');
            from.className = 'plan-block-id';
            from.textContent = bw.lessonTitle;
            bli.appendChild(from);
            bankList.appendChild(bli);
        }
        bank.appendChild(bankList);
        var paste = el('textarea');
        paste.setAttribute('data-plan-word-paste', '');
        paste.setAttribute('rows', '4');
        paste.setAttribute('placeholder', readUiLang() === 'he'
            ? 'הדבקת שורות: עברית – English'
            : 'Paste lines: Hebrew – English');
        paste.setAttribute('aria-label', readUiLang() === 'he' ? 'הדבקת מילים' : 'Paste words');
        bank.appendChild(paste);
        bank.appendChild(button('data-plan-word-paste-apply', '', 'הוספת מילים (Add words)', !state.lessonPath));
        var pasteMsg = el('p');
        pasteMsg.className = 'plan-issue plan-issue-block';
        pasteMsg.setAttribute('data-plan-word-paste-message', '');
        pasteMsg.setAttribute('role', 'alert');
        if (state.wordPasteRejected && state.wordPasteRejected.length) {
            pasteMsg.hidden = false;
            pasteMsg.textContent = uiLine(
                'שורה בלי מפריד: ' + state.wordPasteRejected.join(' · '),
                'Line with no separator: ' + state.wordPasteRejected.join(' · ')
            );
        } else {
            pasteMsg.hidden = true;
        }
        bank.appendChild(pasteMsg);
        var addWordRow = el('div');
        addWordRow.className = 'plan-word-add';
        var addWordInput = el('input');
        addWordInput.type = 'text';
        addWordInput.setAttribute('data-plan-word-add', '');
        addWordInput.setAttribute('autocomplete', 'off');
        addWordInput.setAttribute('aria-label', readUiLang() === 'he' ? 'מילה חדשה' : 'New word');
        addWordRow.appendChild(addWordInput);
        addWordRow.appendChild(button('data-plan-word-add-apply', '', 'הוספה (Add)', !state.lessonPath));
        bank.appendChild(addWordRow);
        var wordHint = el('p');
        wordHint.className = 'plan-note ui-pair';
        var wordHintHe = el('span');
        wordHintHe.className = 'ui-he';
        wordHintHe.lang = 'he';
        wordHintHe.textContent = 'לחצו על מילה בבנק כדי לראות את התרגום במילון.';
        var wordHintEn = el('span');
        wordHintEn.className = 'ui-en';
        wordHintEn.lang = 'en';
        wordHintEn.setAttribute('dir', 'ltr');
        wordHintEn.textContent = 'Click a word in the bank to see the dictionary meaning.';
        wordHint.appendChild(wordHintHe);
        wordHint.appendChild(wordHintEn);
        bank.appendChild(wordHint);
        var wordHits = el('ul');
        wordHits.className = 'plan-lexicon-hits';
        wordHits.setAttribute('data-plan-word-hits', '');
        wordHits.hidden = true;
        bank.appendChild(wordHits);

        // Lesson tabs (plan + lesson screens). Open lesson switches to the lesson screen.
        var tabs = el('nav');
        tabs.className = 'plan-lesson-tabs';
        tabs.setAttribute('data-plan-lesson-tabs', '');
        tabs.setAttribute('aria-label', readUiLang() === 'he' ? 'שיעורים' : 'Lessons');
        var tabI;
        for (tabI = 0; tabI < state.lessonChoices.length; tabI++) {
            var tabChoice = state.lessonChoices[tabI];
            var tabBtn = button('data-plan-lesson-tab', String(tabI), tabChoice.title || tabChoice.id, false);
            tabBtn.className += ' plan-lesson-tab';
            if (tabChoice.path === state.lessonPath && state.screen === 'lesson') {
                tabBtn.setAttribute('aria-current', 'true');
            }
            tabs.appendChild(tabBtn);
        }
        rootEl.appendChild(tabs);

        var mainCol = el('div');
        mainCol.className = 'plan-main';
        var aside = el('div');
        aside.className = 'plan-aside';

        if (state.screen !== 'lesson') {
            // Plan screen: lesson list and word bank. Below 900px the bank sits under the list.
            var layout = el('div');
            layout.className = 'plan-layout plan-screen';
            layout.setAttribute('data-plan-screen', '');
            var rail = el('div');
            rail.className = 'plan-rail';
            rail.appendChild(chooser);
            var bankCol = el('div');
            bankCol.className = 'plan-bank';
            bankCol.appendChild(bank);
            layout.appendChild(rail);
            layout.appendChild(bankCol);
            rootEl.appendChild(layout);
            if (planCard) {
                var planWrap = el('div');
                planWrap.className = 'plan-main';
                planWrap.appendChild(planCard);
                rootEl.appendChild(planWrap);
            }
            state.previewEl = null;
            state.previewLabel = null;
        } else {
            // Lesson screen (Figma 19:2592): bank | activities | preview. Provenance: INIT-011/SPEC-005
            if (lessonCard) {
                lessonCard.setAttribute('data-plan-lesson-details', '');
                rootEl.appendChild(lessonCard);
            } else if (lesson && ready && LESSON_PATH.test(state.lessonPath)) {
                var details = el('section');
                details.className = 'card';
                details.setAttribute('data-plan-lesson-details', '');
                var detailsHeading = el('h3');
                fillUi(detailsHeading, 'פרטי השיעור (Lesson details)');
                details.appendChild(detailsHeading);
                rootEl.appendChild(details);
            }

            var work = el('div');
            work.className = 'plan-layout plan-lesson-screen';
            work.setAttribute('data-plan-lesson-screen', '');
            var lessonBank = el('div');
            lessonBank.className = 'plan-bank';
            lessonBank.appendChild(bank);
            work.appendChild(lessonBank);
            work.appendChild(mainCol);
            work.appendChild(aside);
            rootEl.appendChild(work);

            if (state.schema === 'loading') {
                var loading = el('p');
                loading.className = 'plan-note';
                loading.setAttribute('data-plan-schema-loading', '');
                loading.setAttribute('role', 'status');
                fillUi(loading, 'טוען את הסכמה… (Loading the schema…)');
                mainCol.appendChild(loading);
            } else if (state.schema === 'failed') {
                var failed = el('p');
                failed.className = 'admin-inline-error';
                failed.setAttribute('data-plan-schema-failed', '');
                failed.setAttribute('role', 'alert');
                fillUi(failed, 'הסכמה לא נטענה; אפשר לערוך רק כ-JSON. (The schema did not load; only raw JSON editing is available.)');
                mainCol.appendChild(failed);
            }

            paintEarlierWords(mainCol);

            var outlineHeading = el('h3');
            fillUi(outlineHeading, 'פעילויות (Activities)');
            mainCol.appendChild(outlineHeading);
            var outline = el('ol');
            outline.setAttribute('data-plan-outline', '');
            outline.setAttribute('data-plan-activities', '');
            var visible = listedBlocks(lesson);
            var b;
            for (b = 0; b < visible.length; b++) {
                var block = visible[b];
                var fullIndex = -1;
                var bi;
                for (bi = 0; bi < blocks.length; bi++) {
                    if (blocks[bi] && blocks[bi].id === block.id) {
                        fullIndex = bi;
                        break;
                    }
                }
                var row = el('li');
                row.setAttribute('data-plan-block', block.id);
                if (state.selectedId === block.id) row.setAttribute('aria-current', 'true');
                var label = el('span');
                label.setAttribute('data-plan-activity-label', '');
                var actName = activityNameForType(block.type);
                if (actName && ACTIVITY_HE[actName]) {
                    fillUi(label, ACTIVITY_HE[actName] + ' (' + actName + ')');
                } else {
                    fillUi(label, blockLabel(block.type));
                }
                row.appendChild(label);
                var upIndex = moveDestination(fullIndex, 'up');
                var downIndex = moveDestination(fullIndex, 'down');
                // Skip over the placeholder when deciding ends; move still uses real indices.
                row.appendChild(button('data-plan-move', 'up', 'למעלה (Up)', upIndex == null || upIndex < 0));
                row.appendChild(button(
                    'data-plan-move',
                    'down',
                    'למטה (Down)',
                    downIndex == null || downIndex > blocks.length - 1
                ));
                row.appendChild(button('data-plan-remove', '', 'הסרה (Remove)', false));
                row.appendChild(button('data-plan-edit', '', 'שדות (Fields)', false));
                outline.appendChild(row);
            }
            mainCol.appendChild(outline);

            paintInsert(mainCol);

            var fields = el('div');
            fields.className = 'card';
            fields.setAttribute('data-plan-fields', '');
            if (blockForm && selected && selected.block && selected.block.id !== 'placeholder') {
                var fieldsHeading = el('h3');
                var openName = activityNameForType(selected.block.type);
                if (openName && ACTIVITY_HE[openName]) {
                    fillUi(fieldsHeading, ACTIVITY_HE[openName] + ' (' + openName + ')');
                } else {
                    fillUi(fieldsHeading, blockLabel(selected.block.type));
                }
                fields.appendChild(fieldsHeading);
                fields.appendChild(blockForm.el);
            }
            mainCol.appendChild(fields);

            if (selected && selected.block && selected.block.type === 'introduction') {
                paintPictures(mainCol, selected.block);
            }

            paintMedia(mainCol);

            var previewCard = el('section');
            previewCard.className = 'card plan-preview-card';
            previewCard.setAttribute('data-plan-preview-card', '');
            var previewHeading = el('h3');
            fillUi(previewHeading, 'תצוגה מקדימה (Preview)');
            previewCard.appendChild(previewHeading);
            var previewLabel = el('p');
            previewLabel.className = 'plan-note';
            previewLabel.setAttribute('data-plan-preview-label', '');
            previewLabel.setAttribute('role', 'status');
            previewCard.appendChild(previewLabel);
            var preview = el('div');
            preview.className = 'plan-page';
            preview.setAttribute('data-plan-preview', '');
            preview.setAttribute('aria-label', 'תצוגה מקדימה');
            var guard = function (ev) {
                if (root.HebrewPlanPreview) root.HebrewPlanPreview.guardClick(ev, preview);
            };
            preview.addEventListener('click', guard, state.controller ? { signal: state.controller.signal } : undefined);
            previewCard.appendChild(preview);
            state.previewEl = preview;
            state.previewLabel = previewLabel;
            aside.appendChild(previewCard);
        }

        var foot = el('div');
        foot.className = 'plan-foot';
        if (state.panels.issues) foot.appendChild(state.panels.issues.el);
        paintValidation(foot);
        if (state.panels.history) foot.appendChild(state.panels.history.el);
        rootEl.appendChild(foot);

        panel.appendChild(rootEl);
        refreshPreview();

        // Findings the server returned for this document show at their fields: the issues list when
        // it has arrived, otherwise the findings of the last save or publish.
        var Issues = root.HebrewPlanIssues;
        var served = state.issues && Array.isArray(state.issues.issues) && Issues;
        editor.setIssues(served ? Issues.forEditor(state.issues.issues) : validationList(), state.plan);
        editor.setEditState(state.editState);
        if (focus) editor.restoreFocus(focus);
        if (scrollY !== null && typeof root.scrollTo === 'function') root.scrollTo(0, scrollY);
    }

    function onClick(ev) {
        var result = handleClick(ev);
        if (result && typeof result.then === 'function') track(result);
    }

    function targetNode(ev) {
        var node = ev && ev.target;
        if (node && node.nodeType === 3) node = node.parentNode;
        return node;
    }

    function attrNode(node, name) {
        var stop = state && state.panel;
        while (node && node !== stop) {
            if (node.getAttribute && node.hasAttribute && node.hasAttribute(name)) return node;
            node = node.parentNode;
        }
        return null;
    }

    function rowBlockId(node) {
        var row = attrNode(node, 'data-plan-block');
        if (!row) return '';
        var id = row.getAttribute('data-plan-block') || '';
        return ENTRY.test(id) ? id : '';
    }

    function handleClick(ev) {
        if (!state || state.busy) return Promise.resolve();
        var node = targetNode(ev);
        if (!node || !node.hasAttribute) return Promise.resolve();
        if (attrNode(node, 'data-plan-reload')) return reload();
        if (attrNode(node, 'data-plan-publish')) return publish();
        if (attrNode(node, 'data-plan-crumb-plan')) {
            state.screen = 'plan';
            paint();
            return Promise.resolve();
        }
        var lessonTab = attrNode(node, 'data-plan-lesson-tab');
        if (lessonTab) {
            var tabIndex = Number(lessonTab.getAttribute('data-plan-lesson-tab'));
            var tabChoice = state.lessonChoices[tabIndex];
            if (!tabChoice || !LESSON_PATH.test(tabChoice.path)) return Promise.resolve();
            state.screen = 'lesson';
            return loadLesson(tabChoice.path);
        }
        var lessonBtn = attrNode(node, 'data-plan-lesson');
        if (lessonBtn) {
            var index = Number(lessonBtn.getAttribute('data-plan-lesson'));
            var choice = state.lessonChoices[index];
            if (!choice || !LESSON_PATH.test(choice.path)) return Promise.resolve();
            state.screen = 'lesson';
            return loadLesson(choice.path);
        }
        var bankWord = attrNode(node, 'data-plan-bank-word');
        if (bankWord && state.screen === 'lesson' && !attrNode(node, 'data-lookup-word')) {
            var picked = bankWord.getAttribute('data-plan-bank-word') || '';
            if (picked) return addBankWordToOpenWords(picked);
        }
        var lessonMove = attrNode(node, 'data-plan-lesson-move');
        if (lessonMove) return moveLesson(lessonMove);
        if (attrNode(node, 'data-plan-lesson-remove')) return removeLesson(node);
        if (attrNode(node, 'data-plan-add-lesson')) return addLesson();
        if (attrNode(node, 'data-plan-reuse-lesson-open')) return openReusePicker();
        if (attrNode(node, 'data-plan-reuse-lesson')) return reuseLesson();
        if (attrNode(node, 'data-plan-word-paste-apply')) return pasteWordBank();
        if (attrNode(node, 'data-plan-word-add-apply')) return addBankWord();
        var lexiconPick = attrNode(node, 'data-plan-lexicon-pick');
        if (lexiconPick) return pickLexicon(lexiconPick);
        var verbTense = attrNode(node, 'data-plan-verb-tense');
        if (verbTense) return applyVerbTense(verbTense.getAttribute('data-plan-verb-tense') || '');
        var edit = attrNode(node, 'data-plan-edit');
        if (edit) {
            var editId = rowBlockId(edit);
            if (!editId || !blockById(editId)) return Promise.resolve();
            state.selectedId = editId;
            paint();
            return Promise.resolve();
        }
        var move = attrNode(node, 'data-plan-move');
        if (move) return moveBlock(move);
        if (attrNode(node, 'data-plan-remove')) return removeBlock(node);
        if (attrNode(node, 'data-plan-insert')) return insertBlock();
        var disclose = attrNode(node, 'data-plan-disclose');
        if (disclose) {
            var section = disclose.getAttribute('data-plan-disclose');
            state.open[section] = state.open[section] !== true;
            paint();
            return Promise.resolve();
        }
        if (attrNode(node, 'data-plan-upload')) return uploadFile();
        if (attrNode(node, 'data-plan-dry-run')) return dryRun();
        if (attrNode(node, 'data-plan-generate')) return generateImage();
        if (attrNode(node, 'data-plan-approve')) return approveMedia();
        var attach = attrNode(node, 'data-plan-attach');
        if (attach) return attachPicture(attach);
        return Promise.resolve();
    }

    function onChange(ev) {
        if (!state) return;
        schedulePreview();
        var node = targetNode(ev);
        if (node && attrNode(node, 'data-plan-confirm')) syncGenerate();
        var typeSelect = node ? attrNode(node, 'data-plan-new-type') : null;
        if (typeSelect) {
            if (String(typeSelect.value || '') !== 'conjugation-table') state.patternFill = null;
            syncActivityHint(typeSelect);
            syncVerbFinder(typeSelect);
            renderNewDraft(String(typeSelect.value || ''));
        }
    }

    function onInput(ev) {
        if (!state || !state.panel) return;
        schedulePreview();
        var node = targetNode(ev);
        if (!node) return;
        if (attrNode(node, 'data-plan-word-add')) {
            if (state.pickedWord && String(node.value || '').trim() !== state.pickedWord) {
                state.pickedWord = '';
                state.pickedMeaning = '';
            }
            scheduleLexiconSearch(node, 'word');
        }
        if (attrNode(node, 'data-plan-verb-query')) scheduleLexiconSearch(node, 'verb');
        if (attrNode(node, 'data-plan-approve-alt')) syncApprove();
        if (!attrNode(node, 'data-plan-gen-prompt')) return;
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        var prompt = promptInput ? String(promptInput.value || '') : '';
        if (state.estimateShown && prompt !== state.estimatePrompt) {
            state.estimateShown = false;
            state.estimateDollars = null;
            var estimate = state.panel.querySelector('[data-plan-estimate]');
            if (estimate) {
                estimate.hidden = true;
                estimate.textContent = '';
            }
        }
        syncGenerate();
    }

    /** Reload takes what the server has and drops unsaved text (the admin asked for it). */
    function reload() {
        hideBox('data-plan-conflict');
        // reset() first: the next paint would otherwise take the live text for a draft again.
        state.editor.reset();
        state.editor.discardDrafts();
        return loadPlan();
    }

    function publish() {
        if (!Number.isInteger(state.revision)) return Promise.resolve();
        var mine = state.token;
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/publish', {}).then(function (ok) {
            if (!ok || !alive(mine)) return ok;
            // After a publish: the issues the server found, shown next to the new revision.
            var issues = state.panels.issues;
            var last = issues ? issues.last() : null;
            if (last && root.HebrewPlanIssues) {
                var counts = root.HebrewPlanIssues.countsText(last.counts);
                var suffix = ' · ' + state.revision + ' · ';
                state.notice = 'פורסם (Published) · ' + state.revision;
                var node = state.panel && state.panel.querySelector('[data-plan-notice]');
                if (node && root.HebrewPlanUi && typeof root.HebrewPlanUi.fillPair === 'function') {
                    root.HebrewPlanUi.fillPair(
                        node,
                        'פורסם' + suffix + counts.he,
                        'Published' + suffix + counts.en
                    );
                } else if (node) {
                    fillUi(node, state.notice);
                }
            }
            return ok;
        });
    }

    function moveBlock(node) {
        if (node.hasAttribute('data-plan-locked') || node.disabled) return Promise.resolve();
        var id = rowBlockId(node);
        var found = id ? blockById(id) : null;
        if (!found || !LESSON_PATH.test(state.lessonPath)) return Promise.resolve();
        var direction = node.getAttribute('data-plan-move');
        var index = moveDestination(found.index, direction);
        var count = blocksOf(state.lesson).length;
        if (!Number.isInteger(index) || index < 0 || index > count - 1) return Promise.resolve();
        var path = state.lessonPath + '/blocks/' + id;
        if (!PART_PATH.test(path)) return Promise.resolve();
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/blocks/move', {
            path: path,
            index: index
        });
    }

    /**
     * Reorder a lesson in its unit via POST /lessons/move (INIT-010/SPEC-002).
     * @param {HTMLElement} node
     * @returns {Promise<boolean>}
     */
    function moveLesson(node) {
        if (!state || !state.plan || node.hasAttribute('data-plan-locked') || node.disabled) {
            return Promise.resolve(false);
        }
        var row = attrNode(node, 'data-plan-stack-row');
        if (!row) return Promise.resolve(false);
        var lessonId = row.getAttribute('data-plan-stack-row') || '';
        var from = -1;
        var i;
        for (i = 0; i < state.lessonChoices.length; i++) {
            if (state.lessonChoices[i].id === lessonId) {
                from = i;
                break;
            }
        }
        var direction = node.getAttribute('data-plan-lesson-move');
        var choice = state.lessonChoices[from];
        if (!choice || !LESSON_PATH.test(choice.path)) return Promise.resolve(false);
        // Within one unit only (SPEC-002 move_lesson). Index is the post-splice destination.
        var unitLessons = [];
        for (i = 0; i < state.lessonChoices.length; i++) {
            if (state.lessonChoices[i].unitId === choice.unitId) {
                unitLessons.push(state.lessonChoices[i]);
            }
        }
        var unitFrom = -1;
        for (i = 0; i < unitLessons.length; i++) {
            if (unitLessons[i].id === choice.id) {
                unitFrom = i;
                break;
            }
        }
        var unitTo = moveDestination(unitFrom, direction);
        if (!Number.isInteger(unitTo) || unitTo < 0 || unitTo >= unitLessons.length) {
            return Promise.resolve(false);
        }
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/lessons/move', {
            path: choice.path,
            index: unitTo
        });
    }

    function currentUnitId() {
        var unitId = '';
        if (state.lessonPath) {
            var segs = state.lessonPath.split('/');
            if (segs.length >= 2) unitId = segs[1];
        }
        if (!unitId && state.lessonChoices.length) unitId = state.lessonChoices[0].unitId;
        if (!unitId && state.plan && state.plan.units && state.plan.units[0]) {
            unitId = state.plan.units[0].id;
        }
        return unitId;
    }

    function addLesson() {
        if (!state || !state.plan) {
            showBox('data-plan-error', uiLine('אין תוכנית פתוחה.', 'No plan is open.'));
            return Promise.resolve(false);
        }
        var titleInput = state.panel && state.panel.querySelector('[data-plan-add-lesson-title]');
        var subInput = state.panel && state.panel.querySelector('[data-plan-add-lesson-subtitle]');
        var title = titleInput ? String(titleInput.value || '').trim() : '';
        var subtitle = subInput ? String(subInput.value || '').trim() : '';
        if (!title) {
            showBox('data-plan-error', uiLine('צריך כותרת לשיעור.', 'A lesson title is needed.'));
            return Promise.resolve(false);
        }
        var unitId = currentUnitId();
        var unit = unitById(unitId);
        if (!unit || !Array.isArray(unit.lessons)) {
            showBox('data-plan-error', uiLine('לא נמצאה יחידה לשיעור החדש.', 'Could not find a unit for the new lesson.'));
            return Promise.resolve(false);
        }
        var id = slugFromTitle(title, unit.lessons);
        if (!SLUG.test(id)) {
            showBox('data-plan-error', uiLine('לא ניתן לבחור מזהה לשיעור החדש.', 'Could not choose an id for the new lesson.'));
            return Promise.resolve(false);
        }
        var path = 'units/' + unitId + '/lessons/' + id;
        var lesson = newLessonShell(id, title, subtitle);
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/lessons', {
            path: 'units/' + unitId,
            lesson: lesson
        }, {
            onSaved: function () {
                state.lessonPath = path;
                if (titleInput) titleInput.value = '';
                if (subInput) subInput.value = '';
            }
        });
    }

    function openReusePicker() {
        if (!state || !state.plan || !api()) return Promise.resolve(false);
        state.reuseOpen = true;
        state.reuseOptions = [];
        state.reuseCache = {};
        var mine = state.token;
        // This plan's lessons first.
        state.lessonChoices.forEach(function (choice) {
            var lesson = form().nodeAt(state.plan, choice.path);
            if (!lesson) return;
            var key = state.planId + '|' + choice.path;
            state.reuseOptions.push({
                key: key,
                label: (state.plan.title || state.planId) + ' — ' + (choice.title || choice.id),
                planId: state.planId,
                path: choice.path,
                lessonId: choice.id
            });
            state.reuseCache[key] = lesson;
        });
        paint();
        return Promise.resolve(api().get('/api/plans', state.controller ? { signal: state.controller.signal } : undefined)).then(function (res) {
            if (!alive(mine) || !res || !res.ok || !res.data || !Array.isArray(res.data.plans)) {
                return false;
            }
            var plans = res.data.plans.filter(function (row) {
                return row && typeof row.id === 'string' && row.id !== state.planId;
            });
            var chain = Promise.resolve();
            plans.forEach(function (row) {
                chain = chain.then(function () {
                    if (!alive(mine)) return;
                    return Promise.resolve(api().get(
                        '/api/plans/' + encodeURIComponent(row.id) + '/parts',
                        state.controller ? { signal: state.controller.signal } : undefined
                    )).then(function (partRes) {
                        if (!alive(mine) || !partRes || !partRes.ok || !partRes.data || !partRes.data.part) return;
                        var other = partRes.data.part;
                        listLessons(other).forEach(function (choice) {
                            var lesson = form().nodeAt(other, choice.path);
                            if (!lesson) return;
                            var key = row.id + '|' + choice.path;
                            state.reuseOptions.push({
                                key: key,
                                label: (other.title || row.title || row.id) + ' — ' + (choice.title || choice.id),
                                planId: row.id,
                                path: choice.path,
                                lessonId: choice.id
                            });
                            state.reuseCache[key] = lesson;
                        });
                    });
                });
            });
            return chain.then(function () {
                if (!alive(mine)) return false;
                state.reuseOpen = true;
                paint();
                return true;
            });
        });
    }

    function reuseLesson() {
        if (!state || !state.plan) {
            showBox('data-plan-error', uiLine('אין תוכנית פתוחה.', 'No plan is open.'));
            return Promise.resolve(false);
        }
        var select = state.panel && state.panel.querySelector('[data-plan-reuse-select]');
        var key = select ? String(select.value || '') : '';
        var opt = (state.reuseOptions || []).find(function (o) { return o.key === key; });
        var source = key && state.reuseCache ? state.reuseCache[key] : null;
        if (!opt || !source) {
            showBox('data-plan-error', uiLine('בחרו שיעור לשימוש חוזר.', 'Choose a lesson to reuse.'));
            return Promise.resolve(false);
        }
        var unitId = currentUnitId();
        var unit = unitById(unitId);
        if (!unit || !Array.isArray(unit.lessons)) {
            showBox('data-plan-error', uiLine('לא נמצאה יחידה לשיעור החדש.', 'Could not find a unit for the new lesson.'));
            return Promise.resolve(false);
        }
        var copy = prepareReuseLesson(source, opt.lessonId, state.lessonChoices);
        if (!SLUG.test(copy.id)) {
            showBox('data-plan-error', uiLine('לא ניתן לבחור מזהה לשיעור החדש.', 'Could not choose an id for the new lesson.'));
            return Promise.resolve(false);
        }
        var path = 'units/' + unitId + '/lessons/' + copy.id;
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/lessons', {
            path: 'units/' + unitId,
            lesson: copy
        }, {
            onSaved: function () {
                state.lessonPath = path;
                state.reuseOpen = false;
            }
        });
    }

    function introductionBlock(lesson) {
        if (!lesson || !Array.isArray(lesson.blocks)) return null;
        var i;
        for (i = 0; i < lesson.blocks.length; i++) {
            if (lesson.blocks[i] && lesson.blocks[i].type === 'introduction') return lesson.blocks[i];
        }
        return null;
    }

    function wordEntryId(existing, index) {
        var used = {};
        (Array.isArray(existing) ? existing : []).forEach(function (w) {
            if (w && typeof w.id === 'string') used[w.id] = true;
        });
        var n = index + 1;
        while (used['w' + n]) n += 1;
        return 'w' + n;
    }

    var PRESENT_CELLS = ['ms', 'fs', 'mp', 'fp'];
    var PAST_CELLS = ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'];

    /**
     * Cells a conjugation table can store, taken from a lexicon paradigm.
     * Each formKey is the stored pointed form. A tense with no stored forms yields [].
     * @param {object} paradigm
     * @param {string} tense
     * @returns {{ cell: string, formKey: string }[]}
     */
    function cellsFromParadigm(paradigm, tense) {
        var order = tense === 'past' || tense === 'future' ? PAST_CELLS
            : (tense === 'present' ? PRESENT_CELLS : []);
        var bucket = paradigm && order.length ? paradigm[tense] : null;
        if (!bucket || typeof bucket !== 'object') return [];
        var cells = [];
        var i;
        for (i = 0; i < order.length; i++) {
            var code = order[i];
            var form = bucket[code];
            var key = form && typeof form.key === 'string' ? form.key : '';
            if (key) cells.push({ cell: code, formKey: key });
        }
        return cells;
    }

    function lexiconApi() {
        return root.HebrewApi && typeof root.HebrewApi.get === 'function' ? root.HebrewApi : null;
    }

    /**
     * One row per verb lemma. A matching form still points at its lemma.
     * @param {object[]} entries
     * @returns {{ key: string, gloss: string }[]}
     */
    function verbChoices(entries) {
        var byKey = {};
        var order = [];
        (Array.isArray(entries) ? entries : []).forEach(function (entry) {
            if (!entry || typeof entry.key !== 'string') return;
            var key = entry.kind === 'lemma' ? entry.key : (typeof entry.lemma_key === 'string' ? entry.lemma_key : '');
            if (!key) return;
            if (!byKey[key]) order.push(key);
            var gloss = typeof entry.gloss_en === 'string' ? entry.gloss_en : '';
            if (!byKey[key] || entry.kind === 'lemma') {
                byKey[key] = { key: key, gloss: entry.kind === 'lemma' ? gloss : (byKey[key] ? byKey[key].gloss : '') };
            }
        });
        return order.slice(0, 8).map(function (key) { return byKey[key]; });
    }

    function searchLexicon(q, pos) {
        var api = lexiconApi();
        if (!api) return Promise.resolve([]);
        var path = pos
            ? '/api/lexicon?q=' + encodeURIComponent(q) + '&limit=40&pos=' + encodeURIComponent(pos)
            : '/api/dictionary?q=' + encodeURIComponent(q) + '&limit=8';
        return api.get(path).then(function (res) {
            return res && res.ok && res.data && Array.isArray(res.data.entries) ? res.data.entries : [];
        }).catch(function () {
            return [];
        });
    }

    function paintLexiconHits(list, entries, role) {
        if (!list) return;
        empty(list);
        var rows = role === 'verb' ? verbChoices(entries) : (Array.isArray(entries) ? entries : []).slice(0, 8).map(function (entry) {
            return {
                key: entry && typeof entry.key === 'string' ? entry.key : '',
                gloss: entry && typeof entry.gloss_en === 'string' ? entry.gloss_en : ''
            };
        }).filter(function (row) { return row.key; });
        if (!rows.length) {
            list.hidden = true;
            return;
        }
        list.hidden = false;
        rows.forEach(function (row) {
            var li = el('li');
            var hit = el('button');
            hit.type = 'button';
            hit.className = 'plan-lexicon-hit';
            hit.setAttribute('data-plan-lexicon-pick', role);
            hit.setAttribute('data-lexicon-key', row.key);
            hit.setAttribute('data-lexicon-gloss', row.gloss || '');
            var he = el('span');
            he.lang = 'he';
            he.setAttribute('dir', 'rtl');
            he.textContent = row.key;
            var en = el('span');
            en.lang = 'en';
            en.setAttribute('dir', 'ltr');
            en.textContent = row.gloss || '';
            hit.appendChild(he);
            hit.appendChild(en);
            li.appendChild(hit);
            list.appendChild(li);
        });
    }

    function scheduleLexiconSearch(input, role) {
        if (!state) return;
        if (state.lexiconTimer) clearTimeout(state.lexiconTimer);
        var q = String(input && input.value || '').trim();
        var list = state.panel && state.panel.querySelector(role === 'verb' ? '[data-plan-verb-hits]' : '[data-plan-word-hits]');
        if (q.length < 1) {
            if (list) {
                empty(list);
                list.hidden = true;
            }
            return;
        }
        state.lexiconTimer = setTimeout(function () {
            var seq = (state.lexiconSeq || 0) + 1;
            state.lexiconSeq = seq;
            searchLexicon(q, role === 'verb' ? 'verb' : '').then(function (entries) {
                if (!state || state.lexiconSeq !== seq || !list || !list.parentNode) return;
                paintLexiconHits(list, entries, role);
            });
        }, 200);
    }

    function applyWordsToOpenLesson(pairs) {
        if (!state || !state.lesson || !LESSON_PATH.test(state.lessonPath) || !pairs.length) {
            return Promise.resolve(false);
        }
        var lesson = state.lesson;
        var newWords = Array.isArray(lesson.newWords) ? lesson.newWords.slice() : [];
        pairs.forEach(function (pair) {
            if (newWords.indexOf(pair.word) === -1) newWords.push(pair.word);
        });
        var intro = introductionBlock(lesson);
        var wordsPath = state.lessonPath + '/newWords';
        var chain = state.transport.setField(state.lessonPath, 'newWords', newWords, {});
        if (intro && typeof intro.id === 'string') {
            var words = Array.isArray(intro.words) ? intro.words.slice() : [];
            pairs.forEach(function (pair, i) {
                words.push({
                    id: wordEntryId(words, words.length + i),
                    word: pair.word,
                    meaning: pair.meaning,
                    sentence: pair.word
                });
            });
            chain = chain.then(function (out) {
                if (!out || !out.ok) return out;
                return state.transport.setField(
                    state.lessonPath + '/blocks/' + intro.id,
                    'words',
                    words,
                    {}
                );
            });
        }
        return chain.then(function (out) {
            return !!(out && out.ok);
        });
    }

    function pasteWordBank() {
        if (!state || !state.lessonPath) {
            showBox('data-plan-error', uiLine('פתחו שיעור קודם.', 'Open a lesson first.'));
            return Promise.resolve(false);
        }
        var area = state.panel && state.panel.querySelector('[data-plan-word-paste]');
        var parsed = parseWordLines(area ? area.value : '');
        state.wordPasteRejected = parsed.rejected.slice();
        if (parsed.rejected.length && !parsed.words.length) {
            paint();
            return Promise.resolve(false);
        }
        if (!parsed.words.length) {
            paint();
            return Promise.resolve(false);
        }
        return applyWordsToOpenLesson(parsed.words).then(function (ok) {
            if (ok && area) area.value = '';
            // paint() from write already ran; re-paint so rejected lines stay visible.
            if (state.wordPasteRejected && state.wordPasteRejected.length) paint();
            return ok;
        });
    }

    function addBankWord() {
        if (!state || !state.lessonPath) {
            showBox('data-plan-error', uiLine('פתחו שיעור קודם.', 'Open a lesson first.'));
            return Promise.resolve(false);
        }
        var input = state.panel && state.panel.querySelector('[data-plan-word-add]');
        var word = input ? String(input.value || '').trim() : '';
        if (!word) {
            showBox('data-plan-error', uiLine('צריך מילה.', 'A word is needed.'));
            return Promise.resolve(false);
        }
        var meaning = state.pickedWord === word && state.pickedMeaning ? state.pickedMeaning : word;
        return applyWordsToOpenLesson([{ word: word, meaning: meaning }]).then(function (ok) {
            if (ok && input) input.value = '';
            if (ok) {
                state.pickedWord = '';
                state.pickedMeaning = '';
                var hits = state.panel && state.panel.querySelector('[data-plan-word-hits]');
                if (hits) {
                    empty(hits);
                    hits.hidden = true;
                }
            }
            return ok;
        });
    }

    function syncVerbFinder(select, scope) {
        var rootNode = scope || (state && state.panel);
        var box = rootNode && rootNode.querySelector ? rootNode.querySelector('[data-plan-verb-find]') : null;
        if (!box) return;
        box.hidden = !select || String(select.value || '') !== 'conjugation-table';
        var tenses = rootNode.querySelector('[data-plan-verb-tenses]');
        if (!tenses) return;
        var fill = state.patternFill;
        var present = fill && cellsFromParadigm(fill.paradigm, 'present').length;
        var past = fill && cellsFromParadigm(fill.paradigm, 'past').length;
        tenses.hidden = !(present && past);
    }

    function pickLexicon(node) {
        var role = node.getAttribute('data-plan-lexicon-pick') || '';
        var key = node.getAttribute('data-lexicon-key') || '';
        var gloss = node.getAttribute('data-lexicon-gloss') || '';
        if (!key) return Promise.resolve(false);
        if (role === 'word') {
            var input = state.panel && state.panel.querySelector('[data-plan-word-add]');
            if (input) input.value = key;
            state.pickedWord = key;
            state.pickedMeaning = gloss || key;
            return addBankWord();
        }
        return fillPattern(key);
    }

    function applyVerbTense(tense) {
        if (!state || !state.patternFill || (tense !== 'present' && tense !== 'past' && tense !== 'future')) {
            return Promise.resolve(false);
        }
        var cells = cellsFromParadigm(state.patternFill.paradigm, tense);
        if (!cells.length) return Promise.resolve(false);
        state.patternFill.tense = tense;
        state.patternFill.cells = cells;
        renderNewDraft('conjugation-table');
        return Promise.resolve(true);
    }

    function fillPattern(lemmaKey) {
        var api = lexiconApi();
        if (!api || !lemmaKey) return Promise.resolve(false);
        return api.get('/api/lexicon/' + encodeURIComponent(lemmaKey)).then(function (res) {
            if (!state || !res || !res.ok || !res.data) {
                showBox('data-plan-error', uiLine('הפועל לא נמצא במילון.', 'That verb is not in the dictionary.'));
                return false;
            }
            var data = res.data;
            var paradigm = data.paradigm || {};
            var present = cellsFromParadigm(paradigm, 'present');
            var past = cellsFromParadigm(paradigm, 'past');
            var tense = present.length ? 'present' : (past.length ? 'past' : '');
            var cells = tense === 'past' ? past : present;
            if (!tense || !cells.length) {
                showBox('data-plan-error', uiLine('אין הטיות שמורות לפועל הזה.', 'No stored forms for that verb.'));
                return false;
            }
            var lemma = data.lemma && typeof data.lemma.key === 'string' ? data.lemma : (data.entry || {});
            state.patternFill = {
                lemmaKey: lemma.key || lemmaKey,
                gloss: typeof lemma.gloss_en === 'string' ? lemma.gloss_en : '',
                paradigm: paradigm,
                tense: tense,
                cells: cells
            };
            var typeSelect = state.panel && state.panel.querySelector('[data-plan-new-type]');
            if (typeSelect && String(typeSelect.value || '') !== 'conjugation-table') {
                typeSelect.value = 'conjugation-table';
                syncActivityHint(typeSelect);
            }
            syncVerbFinder(typeSelect);
            var hits = state.panel && state.panel.querySelector('[data-plan-verb-hits]');
            if (hits) {
                empty(hits);
                hits.hidden = true;
            }
            renderNewDraft('conjugation-table');
            return true;
        }).catch(function () {
            showBox('data-plan-error', uiLine('המילון לא נענה.', 'The dictionary did not answer.'));
            return false;
        });
    }

    /**
     * Remove a lesson via DELETE /lessons (refused when it is the last in its unit).
     * @param {HTMLElement} node
     * @returns {Promise<boolean>}
     */
    function removeLesson(node) {
        if (!state || !state.plan || node.hasAttribute('data-plan-locked') || node.disabled) {
            return Promise.resolve(false);
        }
        var row = attrNode(node, 'data-plan-stack-row');
        if (!row) return Promise.resolve(false);
        var lessonId = row.getAttribute('data-plan-stack-row') || '';
        var choice = null;
        var i;
        for (i = 0; i < state.lessonChoices.length; i++) {
            if (state.lessonChoices[i].id === lessonId) {
                choice = state.lessonChoices[i];
                break;
            }
        }
        if (!choice || !LESSON_PATH.test(choice.path)) return Promise.resolve(false);
        var ok = true;
        try {
            if (typeof root.confirm === 'function') ok = root.confirm('Remove this lesson?');
        } catch (err) {
            ok = false;
        }
        if (!ok) return Promise.resolve(false);
        var wasOpen = state.lessonPath === choice.path;
        return write('DELETE', '/api/plans/' + encodeURIComponent(state.planId) + '/lessons', {
            path: choice.path
        }, {
            onSaved: function () {
                if (!wasOpen) return;
                var next = listLessons(state.plan);
                state.lessonPath = next.length ? next[0].path : '';
                state.lesson = null;
            }
        });
    }

    function removeBlock(node) {
        var id = rowBlockId(node);
        if (!id || !LESSON_PATH.test(state.lessonPath)) return Promise.resolve();
        if (id === 'placeholder') return Promise.resolve();
        // Schema requires at least one block; the page refuses removing the only real activity.
        var real = listedBlocks(state.lesson);
        if (real.length <= 1) {
            showBox(
                'data-plan-error',
                uiLine('שיעור צריך לפחות פעילות אחת.', 'A lesson needs at least one activity.')
            );
            return Promise.resolve();
        }
        var ok = true;
        try {
            if (typeof root.confirm === 'function') ok = root.confirm('Remove this block?');
        } catch (err) {
            ok = false;
        }
        if (!ok) return Promise.resolve();
        var path = state.lessonPath + '/blocks/' + id;
        if (!PART_PATH.test(path)) return Promise.resolve();
        return write('DELETE', '/api/plans/' + encodeURIComponent(state.planId) + '/parts', { path: path });
    }

    /**
     * After the first real activity is saved, remove the empty fluency placeholder.
     * @returns {Promise<boolean>}
     */
    function removePlaceholderAfterFirst() {
        if (!state || !LESSON_PATH.test(state.lessonPath || '')) return Promise.resolve(true);
        var blocks = blocksOf(state.lesson);
        var hasPlaceholder = false;
        var realCount = 0;
        var i;
        for (i = 0; i < blocks.length; i++) {
            if (!blocks[i] || typeof blocks[i].id !== 'string') continue;
            if (blocks[i].id === 'placeholder') hasPlaceholder = true;
            else realCount += 1;
        }
        if (!hasPlaceholder || realCount < 1) return Promise.resolve(true);
        var path = state.lessonPath + '/blocks/placeholder';
        if (!PART_PATH.test(path)) return Promise.resolve(true);
        return write('DELETE', '/api/plans/' + encodeURIComponent(state.planId) + '/parts', { path: path });
    }

    function insertBlock() {
        if (!state.panel || !LESSON_PATH.test(state.lessonPath || '')) return Promise.resolve();
        var typeSelect = state.panel.querySelector('[data-plan-new-type]');
        var type = typeSelect ? String(typeSelect.value || '') : '';
        var activityName = selectedActivityName(typeSelect) || activityNameForType(type);
        var id = nextBlockId(activityName, blocksOf(state.lesson));
        var index = blocksOf(state.lesson).length;
        if (!ENTRY.test(id) || blockById(id)) {
            showBox('data-plan-error', uiLine('לא ניתן לבחור מזהה לפעילות.', 'Could not choose an id for the activity.'));
            return Promise.resolve();
        }
        var block = newBlock(type, id);
        if (!block) {
            showBox('data-plan-error', uiLine('סוג הפעילות אינו תקין.', 'That activity type is not valid.'));
            return Promise.resolve();
        }
        // The required fields the admin filled in, when the form on screen is for this type.
        if (state.draft && state.draft.type === type) {
            var problem = state.draft.handle.check();
            if (problem) {
                showBox('data-plan-error', problem);
                return Promise.resolve();
            }
            var drafted = state.draft.handle.getValue();
            if (drafted && typeof drafted === 'object') {
                Object.keys(drafted).forEach(function (key) {
                    if (key !== 'id' && key !== 'type') block[key] = drafted[key];
                });
            }
        }
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/blocks', {
            path: state.lessonPath,
            index: index,
            block: block
        }, { onValidation: showNewIssues }).then(function (ok) {
            if (!ok) return false;
            state.selectedId = id;
            return removePlaceholderAfterFirst();
        });
    }

    /**
     * Picking a word in the bank adds it to the open Words activity (or the Words draft).
     * @param {string} word
     * @returns {Promise<boolean>}
     */
    function addBankWordToOpenWords(word) {
        var text = String(word || '').trim();
        if (!text || !state || !LESSON_PATH.test(state.lessonPath || '')) return Promise.resolve(false);
        var intro = null;
        var selected = state.selectedId ? blockById(state.selectedId) : null;
        if (selected && selected.block && selected.block.type === 'introduction') {
            intro = selected.block;
        } else {
            intro = introductionBlock(state.lesson);
        }
        if (!intro || typeof intro.id !== 'string') {
            // No Words activity yet: leave a note; teacher adds Words first.
            showBox(
                'data-plan-error',
                uiLine('פתחו פעילות מילים קודם.', 'Open a Words activity first.')
            );
            return Promise.resolve(false);
        }
        var words = Array.isArray(intro.words) ? intro.words.slice() : [];
        words.push({
            id: wordEntryId(words, words.length),
            word: text,
            meaning: text,
            sentence: text
        });
        return state.transport.setField(
            state.lessonPath + '/blocks/' + intro.id,
            'words',
            words,
            {}
        ).then(function (out) {
            return !!(out && out.ok);
        });
    }

    /**
     * @param {HTMLInputElement|null} input
     * @returns {Blob|object|null}
     */
    function selectedFile(input) {
        if (!input || !input.files || !input.files.length) return null;
        return input.files[0];
    }

    /**
     * Authenticated admin call that is not a plan revision write.
     * @param {string} method
     * @param {string} path
     * @param {object|FormData} body
     * @returns {Promise<boolean>}
     */
    function mediaCall(method, path, body) {
        var mine = state.token;
        if (!api() || state.busy) return Promise.resolve(false);
        state.busy = true;
        return api().request(path, { method: method, body: body }).then(function (res) {
            if (!alive(mine)) return false;
            state.busy = false;
            if (!res || !res.ok || !res.data) {
                showError(res);
                return false;
            }
            rememberMedia(res.data);
            hideBox('data-plan-error');
            paint();
            return true;
        }).catch(function (err) {
            if (!alive(mine)) return false;
            state.busy = false;
            showBox('data-plan-error', (err && err.message) || 'request failed');
            return false;
        });
    }

    function uploadFile() {
        if (!state.panel) return Promise.resolve(false);
        var idInput = state.panel.querySelector('[data-plan-upload-id]');
        var fileInput = state.panel.querySelector('[data-plan-upload-file]');
        var altInput = state.panel.querySelector('[data-plan-upload-alt]');
        var id = idInput ? String(idInput.value || '').trim() : '';
        var file = selectedFile(fileInput);
        if (!MEDIA_ID.test(id) || !file) {
            showBox('data-plan-error', 'invalid upload');
            return Promise.resolve(false);
        }
        if (typeof root.FormData !== 'function') {
            showBox('data-plan-error', 'upload unavailable');
            return Promise.resolve(false);
        }
        var form = new root.FormData();
        form.append('id', id);
        var alt = altInput ? String(altInput.value || '') : '';
        if (alt) form.append('altText', alt);
        form.append('file', file);
        return mediaCall('POST', '/api/media', form);
    }

    function dryRun() {
        if (!state.panel) return Promise.resolve();
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        var prompt = promptInput ? String(promptInput.value || '') : '';
        if (!prompt.trim() || /[\u0000-\u001F]/.test(prompt)) {
            showBox('data-plan-error', 'invalid prompt');
            return Promise.resolve();
        }
        var mine = state.token;
        if (!api() || state.busy) return Promise.resolve();
        state.busy = true;
        var path = '/api/admin/images/image-plan?prompt=' + encodeURIComponent(prompt) + '&count=1';
        return api().get(path).then(function (res) {
            if (!alive(mine)) return;
            state.busy = false;
            var dollars = res && res.data ? res.data.estimatedDollars : undefined;
            if (!res || !res.ok || typeof dollars !== 'number' || !Number.isFinite(dollars)) {
                showError(res);
                return;
            }
            state.estimateShown = true;
            state.estimateDollars = dollars;
            state.estimatePrompt = prompt;
            var node = state.panel.querySelector('[data-plan-estimate]');
            if (node) {
                node.hidden = false;
                node.textContent = String(dollars);
            }
            var box = state.panel.querySelector('[data-plan-confirm]');
            if (box) box.checked = false;
            hideBox('data-plan-error');
            syncGenerate();
        }).catch(function (err) {
            if (!alive(mine)) return;
            state.busy = false;
            showBox('data-plan-error', (err && err.message) || 'request failed');
        });
    }

    function generateImage() {
        if (!state.panel) return Promise.resolve(false);
        var btn = state.panel.querySelector('[data-plan-generate]');
        var box = state.panel.querySelector('[data-plan-confirm]');
        var idInput = state.panel.querySelector('[data-plan-gen-id]');
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        if (!btn || btn.disabled) return Promise.resolve(false);
        var id = idInput ? String(idInput.value || '').trim() : '';
        var prompt = promptInput ? String(promptInput.value || '') : '';
        if (!state.estimateShown || typeof state.estimateDollars !== 'number' || !box || box.checked !== true) {
            return Promise.resolve(false);
        }
        if (!MEDIA_ID.test(id) || prompt !== state.estimatePrompt) return Promise.resolve(false);
        return mediaCall('POST', '/api/admin/images', {
            prompt: state.estimatePrompt,
            id: id,
            confirm: true,
            estimate: state.estimateDollars
        });
    }

    function approveMedia() {
        if (!state.panel) return Promise.resolve(false);
        var btn = state.panel.querySelector('[data-plan-approve]');
        var altInput = state.panel.querySelector('[data-plan-approve-alt]');
        var idInput = state.panel.querySelector('[data-plan-approve-id]');
        if (!btn || btn.disabled) return Promise.resolve(false);
        var alt = altInput ? String(altInput.value || '').trim() : '';
        var id = idInput ? String(idInput.value || '').trim() : '';
        if (!alt || !MEDIA_ID.test(id)) return Promise.resolve(false);
        return mediaCall('POST', '/api/admin/media/' + encodeURIComponent(id) + '/approval', {
            altText: alt
        });
    }

    /**
     * @param {object} block
     * @param {string} id
     * @returns {object|null}
     */
    function wordById(block, id) {
        var words = block && Array.isArray(block.words) ? block.words : [];
        var i;
        for (i = 0; i < words.length; i++) {
            if (words[i] && words[i].id === id) return words[i];
        }
        return null;
    }

    /**
     * set_field mediaId and alt. A word with no picture gets an image object
     * first, because those two names are written onto image only when it exists.
     * @param {HTMLElement} node
     * @returns {Promise<boolean>}
     */
    function attachPicture(node) {
        var wordId = node.getAttribute('data-plan-attach') || '';
        if (!ENTRY.test(wordId) || !state.panel) return Promise.resolve(false);
        var found = state.selectedId ? blockById(state.selectedId) : null;
        if (!found || found.block.type !== 'introduction') return Promise.resolve(false);
        var word = wordById(found.block, wordId);
        var row = attrNode(node, 'data-plan-word');
        if (!word || !row) return Promise.resolve(false);
        var mediaInput = row.querySelector('[data-plan-attach-media]');
        var altInput = row.querySelector('[data-plan-attach-alt]');
        var mediaId = mediaInput ? String(mediaInput.value || '').trim() : '';
        var alt = altInput ? String(altInput.value || '') : '';
        if (!MEDIA_ID.test(mediaId) || !String(alt).trim()) {
            showBox('data-plan-error', 'invalid picture');
            return Promise.resolve(false);
        }
        var path = state.lessonPath + '/blocks/' + found.block.id + '/words/' + wordId;
        if (!PART_PATH.test(path) || !LESSON_PATH.test(state.lessonPath || '')) return Promise.resolve(false);
        var steps = [];
        if (!word.image || typeof word.image !== 'object' || Array.isArray(word.image)) {
            steps.push({ field: 'image', value: { mediaId: mediaId, alt: alt } });
        }
        steps.push({ field: 'mediaId', value: mediaId });
        steps.push({ field: 'alt', value: alt });
        return sendAttach(path, steps, 0);
    }

    /**
     * @param {string} path
     * @param {{ field: string, value: unknown }[]} steps
     * @param {number} index
     * @returns {Promise<boolean>}
     */
    function sendAttach(path, steps, index) {
        if (!state || index >= steps.length) return Promise.resolve(true);
        var step = steps[index];
        return write('PATCH', '/api/plans/' + encodeURIComponent(state.planId) + '/fields', {
            path: path,
            field: step.field,
            value: step.value
        }).then(function (ok) {
            if (!ok) return false;
            return sendAttach(path, steps, index + 1);
        });
    }

    /**
     * One revision-checked write. `expectedRevision` is the value the server last
     * returned (state.revision), never a value read from the page.
     * `hooks.onSaved` runs on success before the page repaints; `hooks.onValidation`
     * gets a 400 validation body. The status and body of the last answer stay in
     * `state.last` for the editor's transport.
     * @param {string} method
     * @param {string} path
     * @param {object} body
     * @param {{ onSaved?: Function, onValidation?: Function }} [hooks]
     * @returns {Promise<boolean>}
     */
    function write(method, path, body, hooks) {
        var mine = state.token;
        if (!Number.isInteger(state.revision) || !api() || state.busy) return Promise.resolve(false);
        var payload = {
            expectedRevision: state.revision
        };
        var key;
        for (key in body) {
            if (Object.prototype.hasOwnProperty.call(body, key)) payload[key] = body[key];
        }
        state.busy = true;
        return api().request(path, { method: method, body: payload }).then(function (res) {
            if (!alive(mine)) return false;
            state.busy = false;
            state.last = { ok: !!(res && res.ok), status: res ? res.status : 0, data: res ? res.data : null };
            if (res && res.status === 409) {
                showConflict(res.data || {});
                return false;
            }
            if (res && res.status === 400 && res.data && res.data.error === 'validation') {
                // No repaint: the typed text and the issues at the fields stay where they are.
                state.validation = findingsFrom(res.data);
                state.publishRefused = path.indexOf('/publish') !== -1;
                state.publishRefusal = typeof res.data.error === 'string' ? res.data.error : '';
                hideBox('data-plan-conflict');
                refreshValidation();
                showBox('data-plan-error', refusalSentence(res.data));
                if (hooks && typeof hooks.onValidation === 'function') hooks.onValidation(res.data);
                return false;
            }
            if (!res || !res.ok || !res.data || !Number.isInteger(res.data.revision)) {
                showError(res);
                return false;
            }
            state.revision = res.data.revision;
            if (res.data.plan && typeof res.data.plan === 'object') {
                state.plan = res.data.plan;
                state.lessonChoices = listLessons(state.plan);
            }
            state.validation = findingsFrom(res.data);
            state.publishRefused = false;
            state.publishRefusal = '';
            state.notice = 'נשמר (Saved) · ' + res.data.revision;
            hideBox('data-plan-conflict');
            if (hooks && typeof hooks.onSaved === 'function') hooks.onSaved();
            if (state.lessonPath) {
                return loadLesson(state.lessonPath).then(function () { return refreshSide(); }).then(function () { return true; });
            }
            paint();
            return refreshSide().then(function () { return true; });
        }).catch(function (err) {
            if (!alive(mine)) return false;
            state.busy = false;
            showBox('data-plan-error', (err && err.message) || 'request failed');
            return false;
        });
    }

    function loadLesson(path) {
        var mine = state.token;
        if (!LESSON_PATH.test(path) || !api()) return Promise.resolve();
        return api().get(partsUrl(path)).then(function (res) {
            if (!alive(mine)) return;
            if (!res || !res.ok || !res.data || typeof res.data.part !== 'object' || Array.isArray(res.data.part)) {
                showError(res);
                return;
            }
            if (!Number.isInteger(res.data.revision)) {
                showError(res);
                return;
            }
            state.lesson = res.data.part;
            state.lessonPath = path;
            state.revision = res.data.revision;
            hideBox('data-plan-conflict');
            paint();
        }).catch(function (err) {
            if (!alive(mine)) return;
            showBox('data-plan-error', (err && err.message) || 'request failed');
        });
    }

    function loadPlan() {
        var mine = state.token;
        if (!api()) {
            paintMessage('HebrewApi missing');
            return Promise.resolve();
        }
        return api().get(partsUrl('')).then(function (res) {
            if (!alive(mine)) return;
            if (!res || !res.ok || !res.data || typeof res.data.part !== 'object' || Array.isArray(res.data.part)) {
                paintMessage(errorText(res));
                return;
            }
            if (!Number.isInteger(res.data.revision)) {
                paintMessage(errorText(res));
                return;
            }
            state.plan = res.data.part;
            state.revision = res.data.revision;
            state.editState = humanEditedOf(res.data.editState);
            state.lessonChoices = listLessons(state.plan);
            if (state.panels.assign) state.panels.assign.load();
            var shown;
            // A lesson that was open stays open after a reload or a restore, when it is still there.
            var keep = state.lessonChoices.filter(function (c) { return c.path === state.lessonPath; })[0];
            if (state.lessonChoices.length === 1 || keep) {
                shown = loadLesson((keep || state.lessonChoices[0]).path);
            } else {
                state.lesson = null;
                state.lessonPath = '';
                paint();
                shown = Promise.resolve();
            }
            return shown.then(function () {
                if (!alive(mine)) return undefined;
                return ensureSchema();
            }).then(function () {
                if (!alive(mine)) return undefined;
                // the read above already carried who last wrote each field
                return refreshSide({ skipEditState: true });
            });
        }).catch(function (err) {
            if (!alive(mine)) return;
            paintMessage((err && err.message) || 'request failed');
        });
    }

    /**
     * The plan JSON Schema is fetched once per mount, after the plan has painted.
     * Until it arrives the form area says so; if it does not arrive, every node can
     * still be edited as raw JSON.
     * @returns {Promise<void>}
     */
    function ensureSchema() {
        var mine = state.token;
        if (state.editor.hasSchema()) {
            state.schema = 'ready';
            paint();
            return Promise.resolve();
        }
        return state.editor.loadSchema(api()).then(function (ok) {
            if (!alive(mine)) return;
            state.schema = ok ? 'ready' : 'failed';
            paint();
        });
    }

    function isDirty() {
        return !!(state && state.editor && state.editor.hasUnsaved());
    }

    /**
     * What the editor sends through. Each call is one revision-checked write; the
     * answer carries the status and body so the editor can show a conflict or the
     * server's rule codes at the field.
     */
    function makeTransport(mine) {
        function call(method, suffix, body, hooks) {
            if (!state || state !== mine) return Promise.resolve({ ok: false, status: 0, data: null });
            if (state.busy) return Promise.resolve({ ok: false, status: 0, data: { code: 'busy' } });
            var url = '/api/plans/' + encodeURIComponent(mine.planId) + suffix;
            return write(method, url, body, hooks).then(function (ok) {
                var last = mine.last || {};
                return { ok: ok, status: last.status, data: last.data };
            });
        }
        return {
            setField: function (path, field, value, hooks) {
                return call('PATCH', '/fields', { path: path, field: field, value: value }, hooks);
            },
            putPart: function (path, value, hooks) {
                return call('PUT', '/parts', { path: path, value: value }, hooks);
            }
        };
    }

    /**
     * @param {HTMLElement} panel
     * @param {string} planId
     * @returns {Promise<void>}
     */
    function mount(panel, planId) {
        unmount();
        var mine = token;
        state = {
            token: mine,
            panel: panel,
            planId: planId,
            plan: null,
            lesson: null,
            lessonPath: '',
            lessonChoices: [],
            selectedId: null,
            screen: 'plan',
            reuseOpen: false,
            reuseOptions: [],
            reuseCache: {},
            wordPasteRejected: [],
            pickedWord: '',
            pickedMeaning: '',
            patternFill: null,
            lexiconTimer: null,
            lexiconSeq: 0,
            open: { lesson: true, unit: false, plan: false },
            schema: 'loading',
            draft: null,
            newDraftHost: null,
            validationHost: null,
            notice: '',
            last: null,
            controller: null,
            revision: null,
            busy: false,
            media: [],
            estimateShown: false,
            estimateDollars: null,
            estimatePrompt: '',
            validation: null,
            publishRefused: false,
            publishRefusal: '',
            issues: null,
            publishNote: '',
            editState: [],
            panels: {},
            transport: null,
            previewTimer: null,
            previewEl: null,
            previewLabel: null
        };
        if (!panel || !SLUG.test(planId)) {
            if (panel) paintMessage('invalid plan id');
            return Promise.resolve();
        }
        var controller = typeof root.AbortController === 'function' ? new root.AbortController() : null;
        var listen = controller ? { signal: controller.signal } : undefined;
        state.controller = controller;
        panel.addEventListener('click', onClick, listen);
        panel.addEventListener('change', onChange, listen);
        panel.addEventListener('input', onInput, listen);
        if (!form()) {
            paintMessage('HebrewPlanForm missing');
            return Promise.resolve();
        }
        state.transport = makeTransport(state);
        state.editor = form().createEditor({
            doc: root.document,
            host: panel,
            signal: controller ? controller.signal : null,
            transport: state.transport,
            track: track,
            getPlan: function () { return state && state.plan; },
            onToggle: function () { if (state) paint(); },
            onRestoreValue: onRestoreValue
        });
        makePanels(controller ? controller.signal : null);
        return track(loadPlan());
    }

    // ---- the INIT-008 panels -------------------------------------------------------------------

    function makePanels(signal) {
        var current = state;
        var common = { doc: root.document, api: api(), planId: current.planId, signal: signal };
        function extend(extra) {
            var out = {};
            Object.keys(common).forEach(function (key) { out[key] = common[key]; });
            Object.keys(extra).forEach(function (key) { out[key] = extra[key]; });
            return out;
        }
        if (root.HebrewPlanIssues && typeof root.HebrewPlanIssues.create === 'function') {
            current.panels.issues = root.HebrewPlanIssues.create(extend({
                getPlan: function () { return state && state.plan; },
                onJump: jumpTo,
                onFix: applyFix,
                onLoaded: onIssues
            }));
        }
        if (root.HebrewPlanHistory && typeof root.HebrewPlanHistory.create === 'function') {
            current.panels.history = root.HebrewPlanHistory.create(extend({
                getRevision: function () { return state ? state.revision : null; },
                hasUnsaved: isDirty,
                onRestored: afterRestore,
                onReload: function () { if (state) track(reload()); }
            }));
        }
        if (root.HebrewPlanList && typeof root.HebrewPlanList.createAssign === 'function') {
            current.panels.assign = root.HebrewPlanList.createAssign(extend({
                getRevision: function () { return state ? state.revision : null; },
                onAssigned: afterAssign,
                onConflict: function (data) { if (state) showConflict(data); }
            }));
        }
    }

    /** `editState.humanEdited` of a read, or an empty list. */
    function humanEditedOf(editState) {
        return editState && Array.isArray(editState.humanEdited) ? editState.humanEdited : [];
    }

    /** The server's issues arrived: the publish line, the inline issues at the fields. */
    function onIssues(result) {
        if (!state) return;
        state.issues = result;
        var Issues = root.HebrewPlanIssues;
        state.publishNote = Issues ? Issues.publishNote(result.counts) : '';
        var note = state.panel && state.panel.querySelector('[data-plan-publish-note]');
        if (note) fillUi(note, state.publishNote);
        if (state.editor && Issues) state.editor.setIssues(Issues.forEditor(result.issues), state.plan);
    }

    /**
     * What the side panels show, after the plan was read or written: the issues, the history when it
     * is open, and which fields a person last wrote. Resolves when they have all answered.
     * @param {{ skipEditState?: boolean }} [opts]
     * @returns {Promise<void>}
     */
    function refreshSide(opts) {
        if (!state) return Promise.resolve();
        var mine = state.token;
        var panels = state.panels;
        var jobs = [];
        if (panels.issues) jobs.push(panels.issues.load());
        if (panels.history) {
            jobs.push(panels.history.load());
            if (!(opts && opts.skipEditState)) jobs.push(loadEditState());
        }
        if (!jobs.length) return Promise.resolve();
        return Promise.all(jobs).then(function () {
            if (!alive(mine)) return;
            if (state.editor) state.editor.setEditState(state.editState);
        });
    }

    function loadEditState() {
        var mine = state.token;
        return api().get(partsUrl('')).then(function (res) {
            if (!alive(mine)) return;
            if (res && res.ok && res.data && res.data.editState) state.editState = humanEditedOf(res.data.editState);
        }).catch(function () { /* the marks stay as they were */ });
    }

    /** Open the node's form and focus the field an issue is about. */
    function jumpTo(issue) {
        if (!state || !issue || !form()) return Promise.resolve();
        var path = typeof issue.path === 'string' ? issue.path : '';
        var field = typeof issue.field === 'string' ? issue.field : '';
        var kind = form().kindOfPath(path);
        if (kind === null) return Promise.resolve();
        var segs = path.split('/').filter(Boolean);
        var lessonPath = segs.length >= 4 ? segs.slice(0, 4).join('/') : '';
        if (kind === 'unit' && unitPathOf(state.lessonPath) !== path) {
            var inUnit = state.lessonChoices.filter(function (c) { return c.path.indexOf(path + '/lessons/') === 0; })[0];
            if (inUnit) lessonPath = inUnit.path;
        }
        var step = Promise.resolve();
        if (lessonPath && lessonPath !== state.lessonPath && LESSON_PATH.test(lessonPath)) step = loadLesson(lessonPath);
        var mine = state.token;
        return track(step.then(function () {
            if (!alive(mine)) return;
            if (kind === 'plan') state.open.plan = true;
            else if (kind === 'unit') state.open.unit = true;
            else if (kind === 'lesson') state.open.lesson = true;
            else {
                var blockId = segs[5];
                if (!ENTRY.test(blockId) || !blockById(blockId)) {
                    showBox('data-plan-error', 'not_found — ' + path);
                    return;
                }
                state.selectedId = blockId;
                if (kind === 'item') state.editor.open(path);
            }
            paint();
            state.editor.focusField(path, field);
        }));
    }

    /** One set_field through the same save path as every edit; the answer tells the issues panel what happened. */
    function applyFix(request) {
        if (!state || !state.transport) return Promise.resolve(false);
        return state.transport.setField(request.path, request.field, request.value, {}).then(function (out) {
            if (out && out.ok) return 'refreshed';
            var Ui = root.HebrewPlanUi;
            return { error: Ui ? Ui.failureText({ status: out ? out.status : 0, data: out ? out.data : null }) : 'refused' };
        });
    }

    /** A restore came back 200: take the new revision and read the plan again. */
    function afterRestore(data) {
        if (!state) return Promise.resolve('refreshed');
        state.revision = data.revision;
        if (data.plan && typeof data.plan === 'object') state.plan = data.plan;
        state.validation = findingsFrom(data);
        state.publishRefused = false;
        state.publishRefusal = '';
        state.notice = 'שוחזר (Restored) · ' + data.revision;
        hideBox('data-plan-conflict');
        hideBox('data-plan-error');
        // reset() first: the next paint would otherwise take the live text for a draft again.
        state.editor.reset();
        state.editor.discardDrafts();
        return track(loadPlan().then(function () { return 'refreshed'; }));
    }

    /** An assignment came back 200. */
    function afterAssign(data) {
        if (!state) return;
        state.revision = data.revision;
        if (data.plan && typeof data.plan === 'object') state.plan = data.plan;
        state.validation = findingsFrom(data);
        state.notice = 'השיוך נשמר (Assignment saved) · ' + data.revision;
        hideBox('data-plan-conflict');
        var step = state.lessonPath ? loadLesson(state.lessonPath) : (paint(), Promise.resolve());
        track(step.then(function () { return refreshSide(); }));
    }

    /** "Restore the earlier value" under a field: the history panel asks once, then restores. */
    function onRestoreValue(spec) {
        var history = state && state.panels.history;
        if (!history || !spec || !spec.anchor) return;
        history.confirmAt(spec.anchor, { fromRevision: spec.fromRevision, path: spec.path, field: spec.field });
    }

    // ---- the live preview ----

    /** Draw the preview from the saved node with the unsaved field text laid over it. */
    function refreshPreview() {
        if (!state || !state.previewEl) return;
        var node = state.previewEl;
        var selected = state.selectedId ? blockById(state.selectedId) : null;
        if (selected && selected.block && selected.block.id === 'placeholder') selected = null;
        var Preview = root.HebrewPlanPreview;
        var out;
        if (Preview) {
            out = Preview.compute({
                render: root.HebrewPlanRender,
                lesson: state.lesson,
                lessonPath: state.lessonPath,
                block: selected ? selected.block : null,
                blockPath: selected ? state.lessonPath + '/blocks/' + selected.block.id : '',
                handles: state.editor ? state.editor.handles() : []
            });
        } else {
            out = { html: '', state: 'saved' };
            if (selected && root.HebrewPlanRender && typeof root.HebrewPlanRender.renderBlock === 'function') {
                out.html = root.HebrewPlanRender.renderBlock(selected.block, { lesson: state.lesson });
            }
        }
        // No inventing a player: when the type has no renderer, show the activity's own text.
        var html = typeof out.html === 'string' ? out.html : '';
        if (selected && selected.block && !html) {
            var fallback = activityPreviewText(selected.block);
            empty(node);
            var pre = el('pre');
            pre.className = 'plan-preview-fallback';
            pre.setAttribute('data-plan-preview-fallback', '');
            pre.setAttribute('dir', 'auto');
            pre.textContent = fallback || activityNameForType(selected.block.type) || selected.block.type || '';
            node.appendChild(pre);
        } else {
            // Renderer output only: plan text is escaped there. This does not eval it.
            node.innerHTML = html;
        }
        node.setAttribute('data-plan-preview-state', out.state);
        if (state.previewLabel) {
            fillUi(state.previewLabel, out.state === 'unsaved'
                ? 'תצוגה מקדימה — לא נשמרה (Preview — not saved)'
                : 'תצוגה מקדימה — כפי ששמור (Preview — as saved)');
        }
    }

    function schedulePreview() {
        if (!state || !state.previewEl || typeof root.setTimeout !== 'function') return;
        if (state.previewTimer && typeof root.clearTimeout === 'function') root.clearTimeout(state.previewTimer);
        var mine = state.token;
        var wait = root.HebrewPlanPreview ? root.HebrewPlanPreview.DEBOUNCE_MS : 250;
        state.previewTimer = root.setTimeout(function () {
            if (alive(mine)) refreshPreview();
        }, wait);
    }

    /**
     * The Plans tab: a table of every plan, a New plan form, and a way to open one (plan-list.js).
     * Does not call the lesson worksheet API.
     * @param {HTMLElement} panel
     * @returns {void}
     */
    /**
     * @param {HTMLElement} panel
     * @param {{ setHash?: function(string): void }} [opts]
     */
    function mountPicker(panel, opts) {
        unmount();
        if (!panel) return;
        var options = opts || {};
        if (!root.HebrewPlanList || typeof root.HebrewPlanList.mount !== 'function') {
            empty(panel);
            var missing = el('p');
            missing.className = 'admin-inline-error';
            missing.setAttribute('role', 'alert');
            missing.textContent = 'HebrewPlanList missing';
            panel.appendChild(missing);
            return;
        }
        pickerCleanup = root.HebrewPlanList.mount(panel, {
            doc: root.document,
            api: api(),
            setHash: function (route) {
                if (typeof options.setHash === 'function') {
                    options.setHash(route);
                    return;
                }
                if (root.HebrewEditorShell && typeof root.HebrewEditorShell.setHash === 'function') {
                    root.HebrewEditorShell.setHash(route);
                    return;
                }
                if (root.HebrewAdminShell && typeof root.HebrewAdminShell.setHash === 'function') {
                    root.HebrewAdminShell.setHash(route);
                }
            }
        });
    }

    function destroyPanels(current) {
        if (current.previewTimer && typeof root.clearTimeout === 'function') root.clearTimeout(current.previewTimer);
        current.previewTimer = null;
        var panels = current.panels || {};
        Object.keys(panels).forEach(function (name) {
            if (panels[name] && typeof panels[name].destroy === 'function') panels[name].destroy();
        });
        current.panels = {};
    }

    function unmount() {
        token += 1;
        if (state) {
            if (state.lexiconTimer) clearTimeout(state.lexiconTimer);
            if (state.controller) state.controller.abort();
            if (state.panel) {
                state.panel.removeEventListener('click', onClick);
                state.panel.removeEventListener('change', onChange);
                state.panel.removeEventListener('input', onInput);
            }
            destroyPanels(state);
            if (state.editor) state.editor.destroy();
        }
        if (pickerCleanup) pickerCleanup();
        pickerCleanup = null;
        state = null;
    }

    /**
     * @returns {boolean}
     */
    function canLeave() {
        if (!isDirty()) return true;
        try {
            if (typeof root.confirm === 'function') return !!root.confirm('Discard unsaved field text?');
        } catch (err) {
            return false;
        }
        return true;
    }

    /**
     * The promise for the last outline action. Tests wait on this.
     * @returns {Promise<void>}
     */
    function settled() {
        return inflight;
    }

    root.HebrewPlanBuilder = {
        SLUG: SLUG,
        activityHint: activityHint,
        activityNames: activityNames,
        activityPreviewText: activityPreviewText,
        blockTypeForActivity: blockTypeForActivity,
        blockTypes: function () { return form() ? form().blockTypeList() : []; },
        canLeave: canLeave,
        draft: function () { return state ? state.draft : null; },
        editor: function () { return state ? state.editor : null; },
        forAudienceLine: forAudienceLine,
        listedBlocks: listedBlocks,
        mount: mount,
        mountPicker: mountPicker,
        moveDestination: moveDestination,
        newBlock: newBlock,
        nextBlockId: nextBlockId,
        cellsFromParadigm: cellsFromParadigm,
        parseWordLines: parseWordLines,
        verbChoices: verbChoices,
        planWordBank: planWordBank,
        prepareReuseLesson: prepareReuseLesson,
        settled: settled,
        slugFromTitle: slugFromTitle,
        unmount: unmount,
        factsFromPlan: factsFromPlan,
        wordFacts: wordFacts,
        wordsIntroducedByLesson: wordsIntroducedByLesson
    };
})(typeof window !== 'undefined' ? window : globalThis);
