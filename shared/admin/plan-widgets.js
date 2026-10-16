/**
 * Widget registry for the schema-driven plan editor. A widget is
 *   { name, isDefault, test(info, ctx) -> priority, render(info, ctx) -> handle }
 * The highest priority wins (the pattern JSON Forms documents, written by hand here).
 * A hint can name a widget; otherwise the default mapping picks by schema shape:
 *   string -> text input, or a textarea above 200 characters
 *   enum -> select          boolean -> checkbox        integer -> number input
 *   array of strings -> string list
 *   array of objects -> item list (object list)
 *   object with properties -> nested object
 *   a child list of a block (words, paragraphs, lines, questions) -> bucket list
 *   anything else -> the generic JSON widget (priority 1, always matches)
 * so no node of the schema is ever uneditable.
 *
 * `info` describes one property: { def, prop, key, schema (resolved), refDef, required,
 * value, hint, readOnly, level, path, nodeKind }. `ctx` is built by plan-form.js and
 * supplies the document, the hints, and the action registry. A handle is
 *   { info, el, control, after, getValue(), check(), isDirty(), markSaved(), focus() }
 * `getValue()` is the whole new value of the property; `undefined` means absent.
 * Nothing here talks to the server: saving is plan-form.js's job.
 *
 * Every string from the server is set with textContent or .value, never as HTML.
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-011, INIT-008/SPEC-012, INIT-009/SPEC-003
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanWidgets = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var registry = [];
    var LONG_TEXT = 200;

    function readUiLang() {
        try {
            if (root.HebrewUtils && typeof root.HebrewUtils.readUiLang === 'function') {
                return root.HebrewUtils.readUiLang() === 'he' ? 'he' : 'en';
            }
        } catch (err) { /* ignore */ }
        return 'en';
    }

    /** One language on the option; applyUiLang swaps via data-ui-text-*. */
    function setOptionUiText(option, he, en) {
        option.setAttribute('data-ui-text-he', he);
        option.setAttribute('data-ui-text-en', en);
        option.textContent = readUiLang() === 'he' ? he : en;
    }

    // ---- small helpers -------------------------------------------------------------------

    function clone(value) {
        return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
    }

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    function deepEqual(a, b) {
        if (a === b) return true;
        if (a === null || b === null || typeof a !== 'object' || typeof b !== 'object') return false;
        if (Array.isArray(a) !== Array.isArray(b)) return false;
        var i;
        if (Array.isArray(a)) {
            if (a.length !== b.length) return false;
            for (i = 0; i < a.length; i++) {
                if (!deepEqual(a[i], b[i])) return false;
            }
            return true;
        }
        var ka = Object.keys(a).filter(function (k) { return a[k] !== undefined; });
        var kb = Object.keys(b).filter(function (k) { return b[k] !== undefined; });
        if (ka.length !== kb.length) return false;
        for (i = 0; i < ka.length; i++) {
            if (!Object.prototype.hasOwnProperty.call(b, ka[i])) return false;
            if (!deepEqual(a[ka[i]], b[ka[i]])) return false;
        }
        return true;
    }

    /** An empty list and an absent optional list are the same stored value. */
    function norm(value) {
        return Array.isArray(value) && value.length === 0 ? undefined : value;
    }

    function types(schema) {
        if (!schema) return [];
        return [].concat(schema.type || []);
    }

    function has(schema, type) {
        return types(schema).indexOf(type) !== -1;
    }

    function setText(node, text) {
        node.textContent = text;
        return node;
    }

    function make(ctx, tag, className, text) {
        var node = ctx.doc.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) {
            var Ui = root.HebrewPlanUi;
            if (Ui && typeof Ui.fill === 'function') Ui.fill(node, text);
            else node.textContent = text;
        }
        return node;
    }

    function button(ctx, label, className, attrs) {
        var node = make(ctx, 'button', className || 'btn btn-ghost admin-row-btn', label);
        node.type = 'button';
        var name;
        for (name in attrs || {}) {
            if (Object.prototype.hasOwnProperty.call(attrs, name)) node.setAttribute(name, attrs[name]);
        }
        return node;
    }

    /** "Hebrew (English)" for a hint label, or the property name when there is none. */
    function labelText(info) {
        var hint = info.hint || {};
        if (hint.label) return hint.label[0] + ' (' + hint.label[1] + ')';
        return info.prop;
    }

    function applyLang(ctx, node, lang) {
        var attrs = ctx.langAttrs(lang);
        if (attrs.dir) node.setAttribute('dir', attrs.dir);
        if (attrs.lang) node.setAttribute('lang', attrs.lang);
    }

    /**
     * The handle every widget returns. Dirty is "the value now differs from the value the
     * widget was built from" (the saved value, even when it was drawn from an unsaved draft).
     */
    function makeHandle(info, parts) {
        var loaded = info.hasLoaded ? clone(info.loaded) : clone(info.value);
        var handle = {
            info: info,
            el: parts.el,
            control: parts.control || parts.el,
            after: parts.after || [],
            group: !!parts.group,
            getValue: parts.read,
            check: parts.check || function () { return null; },
            isDirty: function () {
                return !deepEqual(norm(handle.getValue()), norm(loaded));
            },
            markSaved: function () {
                loaded = clone(handle.getValue());
            },
            focus: function () {
                if (handle.control && typeof handle.control.focus === 'function') handle.control.focus();
            }
        };
        return handle;
    }

    // ---- registry ------------------------------------------------------------------------

    /**
     * @param {{ name: string, isDefault?: boolean, test: Function, render: Function }} widget
     */
    function register(widget) {
        if (!widget || typeof widget.name !== 'string' || typeof widget.test !== 'function'
            || typeof widget.render !== 'function') {
            throw new TypeError('a widget needs a name, test, and render');
        }
        registry = registry.filter(function (w) { return w.name !== widget.name; });
        registry.push({
            name: widget.name,
            isDefault: widget.isDefault !== false,
            test: widget.test,
            render: widget.render
        });
    }

    function unregister(name) {
        registry = registry.filter(function (w) { return w.name !== name; });
    }

    function byName(name) {
        var i;
        for (i = 0; i < registry.length; i++) {
            if (registry[i].name === name) return registry[i];
        }
        return null;
    }

    /** @returns {{ name: string, isDefault: boolean }[]} */
    function list() {
        return registry.map(function (w) { return { name: w.name, isDefault: w.isDefault }; });
    }

    /**
     * The widget for one property. A hint's named widget wins; otherwise the highest
     * test priority does. The JSON widget matches everything, so this never returns null.
     */
    function pick(info, ctx) {
        var hint = info.hint || {};
        if (hint.widget) {
            var named = byName(hint.widget);
            if (named) return named;
        }
        var best = null;
        var bestScore = 0;
        var i;
        for (i = 0; i < registry.length; i++) {
            var score = registry[i].test(info, ctx) || 0;
            if (score > bestScore) {
                best = registry[i];
                bestScore = score;
            }
        }
        return best || byName('json');
    }

    // ---- text ----------------------------------------------------------------------------

    function wantsTextarea(info) {
        var hint = info.hint || {};
        if (hint.single) return false;
        if (hint.multiline || hint.rows) return true;
        return !!(info.schema && info.schema.maxLength > LONG_TEXT);
    }

    /** fromLessonId: pick an earlier lesson in the plan stack when one exists. */
    function renderFromLessonPicker(info, ctx) {
        var earlier = typeof ctx.earlierLessons === 'function' ? ctx.earlierLessons(info.path) : [];
        var node = make(ctx, 'select');
        node.setAttribute('data-plan-from-lesson', '');
        var none = make(ctx, 'option', '', '—');
        none.value = '';
        node.appendChild(none);
        var seen = {};
        earlier.forEach(function (lesson) {
            if (!lesson || typeof lesson.id !== 'string' || seen[lesson.id]) return;
            seen[lesson.id] = true;
            var option = make(ctx, 'option');
            option.value = lesson.id;
            option.textContent = lesson.title || lesson.id;
            node.appendChild(option);
        });
        var current = typeof info.value === 'string' ? info.value : '';
        if (current && !seen[current]) {
            var keep = make(ctx, 'option');
            keep.value = current;
            keep.textContent = current;
            node.appendChild(keep);
        }
        node.value = current;
        return makeHandle(info, {
            el: node,
            read: function () {
                var text = String(node.value);
                if (text === '' && !info.required) return undefined;
                return text;
            }
        });
    }

    function textWidget(multi) {
        return {
            name: multi ? 'textarea' : 'text',
            test: function (info) {
                if (!has(info.schema, 'string') || info.schema.enum || info.schema.const !== undefined) return 0;
                return wantsTextarea(info) === multi ? 40 : 0;
            },
            render: function (info, ctx) {
                if (!multi && info.key === 'recycle.fromLessonId') {
                    return renderFromLessonPicker(info, ctx);
                }
                var hint = info.hint || {};
                var node = make(ctx, multi ? 'textarea' : 'input');
                if (multi) node.rows = hint.rows || 3;
                else node.type = 'text';
                applyLang(ctx, node, hint.lang);
                node.setAttribute('autocomplete', 'off');
                node.setAttribute('spellcheck', 'false');
                node.value = typeof info.value === 'string' ? info.value : '';
                var after = [];
                var max = info.schema && info.schema.maxLength;
                if ((hint.lang === 'he' && max) || hint.pointed) {
                    var note = make(ctx, 'p', 'plan-note');
                    note.setAttribute('data-plan-note', '');
                    var count = null;
                    if (hint.lang === 'he' && max) {
                        count = make(ctx, 'span', 'plan-count');
                        count.setAttribute('data-plan-count', '');
                        count.textContent = node.value.length + ' / ' + max;
                        note.appendChild(count);
                        ctx.onInput(node, function () {
                            count.textContent = String(node.value).length + ' / ' + max;
                        });
                    }
                    if (hint.pointed) {
                        var pointed = make(ctx, 'span', 'plan-pointed', (count ? ' · ' : '') + 'ניקוד מלא מותר ');
                        var pointedEn = make(ctx, 'span', 'plan-pointed-en', '(Full pointing is fine)');
                        pointedEn.setAttribute('dir', 'ltr');
                        pointedEn.setAttribute('lang', 'en');
                        note.appendChild(pointed);
                        note.appendChild(pointedEn);
                    }
                    after.push(note);
                }
                return makeHandle(info, {
                    el: node,
                    after: after,
                    read: function () {
                        var text = String(node.value);
                        if (text === '' && !info.required) return undefined;
                        return text;
                    }
                });
            }
        };
    }

    // ---- number, select, checkbox, readonly ------------------------------------------------

    var numberWidget = {
        name: 'number',
        test: function (info) {
            return has(info.schema, 'integer') || has(info.schema, 'number') ? 50 : 0;
        },
        render: function (info, ctx) {
            var node = make(ctx, 'input');
            node.type = 'number';
            node.setAttribute('inputmode', 'numeric');
            node.setAttribute('step', '1');
            node.setAttribute('dir', 'ltr');
            if (info.schema.minimum !== undefined) node.setAttribute('min', String(info.schema.minimum));
            if (info.schema.maximum !== undefined) node.setAttribute('max', String(info.schema.maximum));
            node.value = typeof info.value === 'number' ? String(info.value) : '';
            return makeHandle(info, {
                el: node,
                read: function () {
                    var text = String(node.value).trim();
                    if (text === '') return info.required ? null : undefined;
                    return /^-?\d+$/.test(text) ? Number(text) : text;
                }
            });
        }
    };

    function optionLabel(hint, value) {
        var options = hint && hint.options;
        if (options && Object.prototype.hasOwnProperty.call(options, value)) {
            return options[value][0] + ' (' + options[value][1] + ')';
        }
        return String(value);
    }

    function fillSelectOption(option, hint, value) {
        var options = hint && hint.options;
        if (options && Object.prototype.hasOwnProperty.call(options, value)
            && Array.isArray(options[value]) && options[value].length >= 2) {
            setOptionUiText(option, String(options[value][0]), String(options[value][1]));
            return;
        }
        option.textContent = String(value);
    }

    var selectWidget = {
        name: 'select',
        test: function (info) {
            return Array.isArray(info.schema && info.schema.enum) ? 50 : 0;
        },
        render: function (info, ctx) {
            var node = make(ctx, 'select');
            var values = info.schema.enum.slice();
            if (typeof info.value === 'string' && values.indexOf(info.value) === -1) values.unshift(info.value);
            if (!info.required) {
                var none = make(ctx, 'option', '', '—');
                none.value = '';
                node.appendChild(none);
            }
            values.forEach(function (value) {
                var option = make(ctx, 'option');
                option.value = String(value);
                fillSelectOption(option, info.hint, value);
                node.appendChild(option);
            });
            node.value = info.value === undefined ? '' : String(info.value);
            if (info.required && info.value === undefined) node.value = String(values[0]);
            return makeHandle(info, {
                el: node,
                read: function () {
                    var text = String(node.value);
                    if (text === '' && !info.required) return undefined;
                    return text;
                }
            });
        }
    };

    var checkboxWidget = {
        name: 'checkbox',
        test: function (info) {
            return has(info.schema, 'boolean') ? 50 : 0;
        },
        render: function (info, ctx) {
            var node = make(ctx, 'input');
            node.type = 'checkbox';
            node.checked = info.value === true;
            return makeHandle(info, {
                el: node,
                read: function () { return node.checked === true; }
            });
        }
    };

    function display(value) {
        if (value === undefined) return '—';
        if (value === null) return '—';
        if (typeof value === 'object') return JSON.stringify(value);
        return String(value);
    }

    var readonlyWidget = {
        name: 'readonly',
        test: function (info) {
            if (info.readOnly) return 100;
            return info.schema && info.schema.const !== undefined ? 60 : 0;
        },
        render: function (info, ctx) {
            var node = make(ctx, 'output', 'plan-readonly', display(info.value));
            node.setAttribute('dir', 'auto');
            return makeHandle(info, {
                el: node,
                read: function () { return clone(info.value); }
            });
        }
    };

    // ---- generic JSON ----------------------------------------------------------------------

    function jsonRead(node, info) {
        var text = String(node.value);
        if (text.trim() === '') return info.required ? null : undefined;
        try {
            return JSON.parse(text);
        } catch (err) {
            return { $invalidJson: text, message: err && err.message ? String(err.message) : 'invalid' };
        }
    }

    var jsonWidget = {
        name: 'json',
        test: function () { return 1; },
        render: function (info, ctx) {
            var node = make(ctx, 'textarea', 'plan-json');
            node.rows = 6;
            node.setAttribute('dir', 'ltr');
            node.setAttribute('lang', 'en');
            node.setAttribute('spellcheck', 'false');
            node.setAttribute('autocomplete', 'off');
            if (isObject(info.value) && typeof info.value.$invalidJson === 'string') node.value = info.value.$invalidJson;
            else node.value = info.value === undefined ? '' : JSON.stringify(info.value, null, 2);
            return makeHandle(info, {
                el: node,
                read: function () { return jsonRead(node, info); },
                check: function () {
                    var value = jsonRead(node, info);
                    if (isObject(value) && typeof value.$invalidJson === 'string') {
                        return 'JSON לא תקין (Invalid JSON): ' + value.message;
                    }
                    return null;
                }
            });
        }
    };

    // ---- lists -----------------------------------------------------------------------------

    /** Move one entry of a list of rows and re-append the row elements in the new order. */
    function reorder(rows, from, to) {
        if (to < 0 || to >= rows.length) return false;
        var row = rows.splice(from, 1)[0];
        rows.splice(to, 0, row);
        return true;
    }

    function listChrome(ctx, info, kindOf) {
        var box = make(ctx, 'div', 'plan-list');
        box.setAttribute('role', 'group');
        var items = make(ctx, 'ol', 'plan-list-items');
        box.appendChild(items);
        var add = button(ctx, uiText('הוספה', 'Add'), 'btn btn-ghost admin-row-btn', { 'data-plan-add': '' });
        var bar = make(ctx, 'div', 'plan-list-bar');
        bar.appendChild(add);
        box.appendChild(bar);
        return { box: box, items: items, bar: bar, add: add };
    }

    function rowButtons(ctx, row) {
        var wrap = make(ctx, 'span', 'plan-item-ops');
        var up = button(ctx, '↑', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'up' });
        up.setAttribute('aria-label', readUiLang() === 'he' ? 'למעלה' : 'Up');
        var down = button(ctx, '↓', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'down' });
        down.setAttribute('aria-label', readUiLang() === 'he' ? 'למטה' : 'Down');
        var remove = button(ctx, '×', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'remove' });
        remove.setAttribute('aria-label', readUiLang() === 'he' ? 'הסרה' : 'Remove');
        wrap.appendChild(up);
        wrap.appendChild(down);
        wrap.appendChild(remove);
        return { el: wrap, up: up, down: down, remove: remove };
    }

    /** buildsOn: checkboxes for earlier lessons in the stack; keeps ids from outside the plan. */
    function renderBuildsOnPicker(info, ctx) {
        var earlier = typeof ctx.earlierLessons === 'function' ? ctx.earlierLessons(info.path) : [];
        var selected = {};
        (Array.isArray(info.value) ? info.value : []).forEach(function (id) {
            if (typeof id === 'string' && id) selected[id] = true;
        });
        var box = make(ctx, 'div', 'plan-builds-on');
        box.setAttribute('data-plan-builds-on', '');
        box.setAttribute('role', 'group');
        var checks = [];
        var known = {};
        earlier.forEach(function (lesson) {
            if (!lesson || typeof lesson.id !== 'string' || known[lesson.id]) return;
            known[lesson.id] = true;
            var label = make(ctx, 'label', 'plan-builds-on-row');
            var input = make(ctx, 'input');
            input.type = 'checkbox';
            input.value = lesson.id;
            input.checked = !!selected[lesson.id];
            input.setAttribute('data-plan-builds-on-id', lesson.id);
            label.appendChild(input);
            var name = make(ctx, 'span', '', lesson.title || lesson.id);
            label.appendChild(name);
            box.appendChild(label);
            checks.push(input);
            ctx.onInput(input, function () { ctx.touch(box); });
        });
        var extras = [];
        Object.keys(selected).forEach(function (id) {
            if (known[id]) return;
            extras.push(id);
            var keep = make(ctx, 'p', 'plan-note');
            keep.setAttribute('data-plan-builds-on-extra', id);
            keep.textContent = id;
            box.appendChild(keep);
        });
        if (!earlier.length && !extras.length) {
            var empty = make(ctx, 'p', 'plan-note');
            empty.setAttribute('data-plan-builds-on-empty', '');
            empty.setAttribute('data-ui-text-he', 'אין שיעור קודם בתוכנית.');
            empty.setAttribute('data-ui-text-en', 'No earlier lesson in this plan.');
            empty.textContent = readUiLang() === 'he' ? 'אין שיעור קודם בתוכנית.' : 'No earlier lesson in this plan.';
            box.appendChild(empty);
        }
        return makeHandle(info, {
            el: box,
            control: checks[0] || box,
            group: true,
            read: function () {
                var out = [];
                checks.forEach(function (input) {
                    if (input.checked) out.push(String(input.value));
                });
                extras.forEach(function (id) { out.push(id); });
                return !out.length && !info.required && !Array.isArray(info.value) ? undefined : out;
            }
        });
    }

    var stringListWidget = {
        name: 'string-list',
        test: function (info) {
            if (!has(info.schema, 'array') || !info.itemSchema) return 0;
            return has(info.itemSchema, 'string') && !info.itemSchema.enum ? 40 : 0;
        },
        render: function (info, ctx) {
            if (info.key === 'lesson.buildsOn') {
                return renderBuildsOnPicker(info, ctx);
            }
            var hint = info.hint || {};
            var chrome = listChrome(ctx, info);
            var rows = [];
            var multi = !hint.single && (hint.multiline || hint.rows || (info.itemSchema && info.itemSchema.maxLength > LONG_TEXT));

            function refreshLabels() {
                rows.forEach(function (row, index) {
                    row.input.setAttribute('aria-label', labelText(info) + ' ' + (index + 1));
                });
            }

            function place() {
                rows.forEach(function (row) { chrome.items.appendChild(row.li); });
                refreshLabels();
            }

            function addRow(text) {
                var li = make(ctx, 'li', 'plan-list-item');
                var input = make(ctx, multi ? 'textarea' : 'input');
                if (multi) input.rows = hint.rows || 3;
                else input.type = 'text';
                applyLang(ctx, input, hint.lang);
                input.setAttribute('autocomplete', 'off');
                input.setAttribute('spellcheck', 'false');
                input.value = typeof text === 'string' ? text : '';
                var ops = rowButtons(ctx, li);
                li.appendChild(input);
                if (hint.lang === 'he') {
                    var look = button(ctx, 'פירוש (Meaning)', 'btn btn-ghost admin-row-btn', { 'data-lookup-from': '' });
                    li.appendChild(look);
                }
                li.appendChild(ops.el);
                var row = { li: li, input: input };
                rows.push(row);
                ctx.act(ops.up, function () {
                    if (reorder(rows, rows.indexOf(row), rows.indexOf(row) - 1)) place();
                }, ctx.focusKey(info, 'up'));
                ctx.act(ops.down, function () {
                    if (reorder(rows, rows.indexOf(row), rows.indexOf(row) + 1)) place();
                }, ctx.focusKey(info, 'down'));
                ctx.act(ops.remove, function () {
                    rows.splice(rows.indexOf(row), 1);
                    chrome.items.removeChild(li);
                    refreshLabels();
                    ctx.touch(chrome.box);
                }, ctx.focusKey(info, 'remove'));
                return row;
            }

            (Array.isArray(info.value) ? info.value : []).forEach(function (text) { addRow(text); });
            place();
            ctx.act(chrome.add, function () {
                var row = addRow('');
                place();
                row.input.focus();
                ctx.touch(chrome.box);
            }, ctx.focusKey(info, 'add'));
            return makeHandle(info, {
                el: chrome.box,
                control: rows.length ? rows[0].input : chrome.add,
                read: function () {
                    var out = [];
                    rows.forEach(function (row) {
                        if (String(row.input.value).trim() !== '') out.push(String(row.input.value));
                    });
                    return !out.length && !info.required && !Array.isArray(info.value) ? undefined : out;
                }
            });
        }
    };

    /**
     * A draft object: the property list of one object definition as nested rows. The
     * value starts from the original object, so a property the editor does not know is
     * kept, and only the edited properties change.
     */
    var objectWidget = {
        name: 'object',
        test: function (info) {
            return has(info.schema, 'object') && info.schema.properties ? 40 : 0;
        },
        render: function (info, ctx) {
            var original = isObject(info.value) ? info.value : null;
            var wrap = make(ctx, 'div', 'plan-object');
            wrap.setAttribute('role', 'group');
            var body = make(ctx, 'div', 'plan-object-body');
            var fields = [];
            var present = original !== null || info.required;
            var addBtn = null;
            var removeBtn = null;

            function buildBody() {
                fields = [];
                var source = original || ctx.skeleton(info.schema, info.refDef) || {};
                ctx.propsOf(info.schema, info.refDef, info.only).forEach(function (entry) {
                    var child = ctx.childInfo(info, entry, source[entry.prop]);
                    var handle = ctx.field(child);
                    fields.push({ entry: entry, handle: handle });
                    body.appendChild(handle.row);
                });
            }

            function showBody() {
                body.hidden = false;
                if (!fields.length) buildBody();
            }

            if (present) showBody();
            else body.hidden = true;

            if (!info.required) {
                addBtn = button(ctx, uiText('הוספה', 'Add'), 'btn btn-ghost admin-row-btn', { 'data-plan-add': '' });
                removeBtn = button(ctx, 'הסרה (Remove)', 'btn btn-ghost admin-row-btn', { 'data-plan-remove-object': '' });
                var bar = make(ctx, 'div', 'plan-list-bar');
                bar.appendChild(addBtn);
                bar.appendChild(removeBtn);
                wrap.appendChild(bar);
                addBtn.hidden = present;
                removeBtn.hidden = !present;
                ctx.act(addBtn, function () {
                    present = true;
                    showBody();
                    addBtn.hidden = true;
                    removeBtn.hidden = false;
                    ctx.touch(wrap);
                }, ctx.focusKey(info, 'add'));
                ctx.act(removeBtn, function () {
                    present = false;
                    body.hidden = true;
                    addBtn.hidden = false;
                    removeBtn.hidden = true;
                    ctx.touch(wrap);
                }, ctx.focusKey(info, 'remove'));
            }
            wrap.appendChild(body);

            return makeHandle(info, {
                el: wrap,
                control: fields.length ? fields[0].handle.control : (addBtn || wrap),
                read: function () {
                    if (!present) return undefined;
                    var out = original ? clone(original) : {};
                    var seed = original || ctx.skeleton(info.schema, info.refDef) || {};
                    Object.keys(seed).forEach(function (key) {
                        if (out[key] === undefined && seed[key] !== undefined) out[key] = clone(seed[key]);
                    });
                    fields.forEach(function (item) {
                        if (item.handle.info.readOnly) return;
                        var value = item.handle.getValue();
                        if (value === undefined) delete out[item.entry.prop];
                        else out[item.entry.prop] = value;
                    });
                    return out;
                },
                check: function () {
                    var i;
                    for (i = 0; i < fields.length; i++) {
                        var problem = fields[i].handle.check();
                        if (problem) return problem;
                    }
                    return null;
                }
            });
        }
    };

    /** The ids already used in a list of objects, for a new item's id. */
    function nextId(items) {
        var used = {};
        items.forEach(function (item) {
            if (isObject(item) && typeof item.id === 'string') used[item.id] = true;
        });
        var n = 1;
        while (used[String(n)]) n += 1;
        return String(n);
    }

    var objectListWidget = {
        name: 'object-list',
        test: function (info) {
            if (!has(info.schema, 'array') || !info.itemSchema) return 0;
            return has(info.itemSchema, 'object') && info.itemSchema.properties ? 40 : 0;
        },
        render: function (info, ctx) {
            var chrome = listChrome(ctx, info);
            var rows = [];
            var itemTitle = ctx.kindLabel(info.itemDef);

            function refreshTitles() {
                rows.forEach(function (row, index) {
                    row.title.textContent = itemTitle + ' ' + (index + 1);
                });
            }

            function place() {
                rows.forEach(function (row) { chrome.items.appendChild(row.li); });
                refreshTitles();
            }

            function addRow(value) {
                var li = make(ctx, 'li', 'plan-list-item plan-item');
                var head = make(ctx, 'div', 'plan-item-head');
                var title = make(ctx, 'strong', 'plan-item-title');
                var ops = rowButtons(ctx, li);
                head.appendChild(title);
                head.appendChild(ops.el);
                li.appendChild(head);
                var childInfo = ctx.itemInfo(info, value);
                var handle = ctx.field(childInfo, { bare: true });
                li.appendChild(handle.el);
                var row = { li: li, title: title, handle: handle };
                rows.push(row);
                ctx.act(ops.up, function () {
                    if (reorder(rows, rows.indexOf(row), rows.indexOf(row) - 1)) place();
                }, ctx.focusKey(info, 'up'));
                ctx.act(ops.down, function () {
                    if (reorder(rows, rows.indexOf(row), rows.indexOf(row) + 1)) place();
                }, ctx.focusKey(info, 'down'));
                ctx.act(ops.remove, function () {
                    rows.splice(rows.indexOf(row), 1);
                    chrome.items.removeChild(li);
                    refreshTitles();
                    ctx.touch(chrome.box);
                }, ctx.focusKey(info, 'remove'));
                return row;
            }

            (Array.isArray(info.value) ? info.value : []).forEach(function (value) { addRow(value); });
            place();
            ctx.act(chrome.add, function () {
                var fresh = ctx.skeleton(info.itemSchema, info.itemDef) || {};
                if (info.itemSchema.properties && info.itemSchema.properties.id) {
                    fresh.id = nextId(rows.map(function (row) { return row.handle.getValue(); }));
                }
                addRow(fresh);
                place();
                ctx.touch(chrome.box);
            }, ctx.focusKey(info, 'add'));
            return makeHandle(info, {
                el: chrome.box,
                control: chrome.add,
                read: function () {
                    var out = rows.map(function (row) { return row.handle.getValue(); });
                    return !out.length && !info.required && !Array.isArray(info.value) ? undefined : out;
                },
                check: function () {
                    var i;
                    for (i = 0; i < rows.length; i++) {
                        var problem = rows[i].handle.check();
                        if (problem) return problem;
                    }
                    return null;
                }
            });
        }
    };

    /**
     * The child lists of a block that have id paths (words, paragraphs, lines, questions):
     * each item is its own form at its own id path, so a save is one field of one item and
     * the history names the item. Add, remove, and move send the whole list of the block.
     */
    var bucketListWidget = {
        name: 'bucket-list',
        test: function (info) {
            if (info.nodeKind !== 'block' || info.level !== 0 || !info.bucket) return 0;
            return 60;
        },
        render: function (info, ctx) {
            var items = Array.isArray(info.value) ? info.value : [];
            var chrome = listChrome(ctx, info);
            var itemTitle = ctx.kindLabel(info.itemDef);
            var drafting = null;

            items.forEach(function (item, index) {
                var li = make(ctx, 'li', 'plan-list-item plan-item');
                var head = make(ctx, 'div', 'plan-item-head');
                var id = isObject(item) && typeof item.id === 'string' ? item.id : String(index + 1);
                var path = info.path + '/' + info.prop + '/' + id;
                var open = ctx.isOpen(path, items.length <= 3);
                var toggle = button(ctx, (open ? '▾ ' : '▸ ') + itemTitle + ' ' + (index + 1) + ' · ',
                    'btn btn-ghost admin-row-btn plan-item-toggle', { 'data-plan-toggle': path });
                // The summary is its own isolate, so Hebrew or a "?" in it cannot reorder the label.
                toggle.appendChild(make(ctx, 'bdi', '', ctx.summary(item)));
                toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
                ctx.act(toggle, function () { ctx.toggleOpen(path, items.length <= 3); }, 'toggle:' + path);
                head.appendChild(toggle);
                var ops = rowButtons(ctx, li);
                head.appendChild(ops.el);
                li.appendChild(head);
                if (open) {
                    li.appendChild(ctx.renderNode({
                        path: path, node: item, def: info.itemDef, kind: 'item', owner: info
                    }).el);
                }
                chrome.items.appendChild(li);
                ops.up.disabled = index === 0;
                ops.down.disabled = index === items.length - 1;
                ctx.act(ops.up, function () {
                    var next = items.slice();
                    next.splice(index - 1, 0, next.splice(index, 1)[0]);
                    return ctx.writeList(info, next);
                }, ctx.focusKey(info, 'up' + index));
                ctx.act(ops.down, function () {
                    var next = items.slice();
                    next.splice(index + 1, 0, next.splice(index, 1)[0]);
                    return ctx.writeList(info, next);
                }, ctx.focusKey(info, 'down' + index));
                ctx.act(ops.remove, function () {
                    if (!ctx.confirm(root.HebrewPlanUi && root.HebrewPlanUi.line
                        ? root.HebrewPlanUi.line('להסיר את הפריט?', 'Remove this item?')
                        : 'Remove this item?')) return undefined;
                    var next = items.slice();
                    next.splice(index, 1);
                    return ctx.writeList(info, next);
                }, ctx.focusKey(info, 'remove' + index));
            });

            var draftBox = make(ctx, 'div', 'plan-draft');
            draftBox.hidden = true;
            chrome.box.appendChild(draftBox);
            ctx.act(chrome.add, function () {
                if (drafting) return;
                var fresh = ctx.skeleton(info.itemSchema, info.itemDef) || {};
                fresh.id = nextId(items);
                var draftInfo = ctx.itemInfo(info, fresh, { only: ctx.requiredOf(info.itemSchema) });
                var handle = ctx.field(draftInfo, { bare: true });
                var confirmBtn = button(ctx, 'הוספת הפריט (Add the item)', 'btn btn-primary', { 'data-plan-add-confirm': '' });
                var cancelBtn = button(ctx, 'ביטול (Cancel)', 'btn btn-ghost admin-row-btn', { 'data-plan-add-cancel': '' });
                var message = make(ctx, 'p', 'admin-inline-error');
                message.setAttribute('role', 'alert');
                message.setAttribute('data-plan-add-error', '');
                message.hidden = true;
                drafting = { handle: handle };
                draftBox.hidden = false;
                draftBox.appendChild(handle.el);
                draftBox.appendChild(message);
                draftBox.appendChild(confirmBtn);
                draftBox.appendChild(cancelBtn);
                ctx.act(cancelBtn, function () {
                    drafting = null;
                    while (draftBox.firstChild) draftBox.removeChild(draftBox.firstChild);
                    draftBox.hidden = true;
                    chrome.add.focus();
                }, ctx.focusKey(info, 'add-cancel'));
                ctx.act(confirmBtn, function () {
                    var problem = handle.check();
                    if (problem) {
                        message.hidden = false;
                        message.textContent = problem;
                        return undefined;
                    }
                    return ctx.writeList(info, items.concat([handle.getValue()]), function (out) {
                        message.hidden = false;
                        message.textContent = ctx.describeFailure(out);
                    });
                }, ctx.focusKey(info, 'add-confirm'));
                handle.focus();
            }, ctx.focusKey(info, 'add'));

            return makeHandle(info, {
                el: chrome.box,
                control: chrome.add,
                read: function () { return clone(items); }
            });
        }
    };

    // ---- word meanings of a story ------------------------------------------------------------

    var GLOSS_MAX = 80;

    function uiText(he, en) {
        return readUiLang() === 'he' ? he : en;
    }

    /** The Hebrew words of the story block this field belongs to, for a draft. */
    function storyText(info, ctx) {
        var Form = root.HebrewPlanForm;
        var plan = typeof ctx.getPlan === 'function' ? ctx.getPlan() : null;
        if (!Form || typeof Form.nodeAt !== 'function' || !plan) return { text: '', support: [] };
        var block = Form.nodeAt(plan, info.path);
        if (!isObject(block)) return { text: '', support: [] };
        var paragraphs = Array.isArray(block.paragraphs) ? block.paragraphs : [];
        return {
            text: paragraphs.map(function (p) { return isObject(p) && typeof p.text === 'string' ? p.text : ''; }).join(' '),
            support: (Array.isArray(block.support) ? block.support : [])
                .map(function (pair) { return isObject(pair) && typeof pair.he === 'string' ? pair.he : ''; })
                .filter(Boolean)
        };
    }

    /**
     * A word and its meaning per row, in place of raw JSON. "Suggest meanings" asks the
     * app dictionary for the story's unglossed words and adds them as draft rows; a person
     * reads each draft against the story before saving.
     */
    var glossMapWidget = {
        name: 'gloss-map',
        test: function () { return 0; },
        render: function (info, ctx) {
            var original = isObject(info.value) ? info.value : null;
            var box = make(ctx, 'div', 'plan-gloss-map');
            box.setAttribute('role', 'group');
            box.setAttribute('data-plan-gloss-map', '');
            var items = make(ctx, 'ol', 'plan-list-items');
            box.appendChild(items);
            var note = make(ctx, 'p', 'plan-note');
            note.hidden = true;
            note.setAttribute('data-plan-gloss-note', '');
            var rows = [];

            function place() {
                rows.forEach(function (row) { items.appendChild(row.li); });
            }

            function addRow(word, meaning, draft, form, senses) {
                var li = make(ctx, 'li', 'plan-list-item plan-gloss-row' + (draft ? ' plan-gloss-draft' : ''));
                if (draft) li.setAttribute('data-plan-gloss-draft', '');
                var he = make(ctx, 'input');
                he.type = 'text';
                he.setAttribute('dir', 'rtl');
                he.setAttribute('lang', 'he');
                he.setAttribute('autocomplete', 'off');
                he.setAttribute('spellcheck', 'false');
                he.setAttribute('aria-label', uiText('מילה', 'Word'));
                he.value = word || '';
                var en = make(ctx, 'input');
                en.type = 'text';
                en.setAttribute('dir', 'ltr');
                en.setAttribute('lang', 'en');
                en.setAttribute('autocomplete', 'off');
                en.setAttribute('aria-label', uiText('פירוש', 'Meaning'));
                en.placeholder = uiText('פירוש באנגלית', 'English meaning');
                en.value = meaning || '';
                // A draft shows what else the dictionary lists, so a wrong sense is quick to swap.
                var hints = [];
                if (form) hints.push(uiText('צורה של ', 'Form of ') + form);
                (Array.isArray(senses) ? senses : []).forEach(function (sense) {
                    if (sense && typeof sense.en === 'string') hints.push((sense.pos ? sense.pos + ': ' : '') + sense.en);
                });
                if (hints.length) en.title = hints.join('\n');
                var alts = null;
                var others = (Array.isArray(senses) ? senses : []).slice(1).filter(function (sense) {
                    return sense && typeof sense.en === 'string';
                });
                if (draft && others.length) {
                    alts = make(ctx, 'span', 'plan-gloss-alts');
                    alts.textContent = uiText('או: ', 'Or: ') + others.map(function (sense) {
                        return (sense.pos ? sense.pos + ': ' : '') + sense.en;
                    }).join(' | ');
                    alts.setAttribute('lang', 'en');
                    alts.setAttribute('dir', 'ltr');
                    alts.setAttribute('data-plan-gloss-alts', '');
                }
                var remove = button(ctx, '×', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'remove' });
                remove.setAttribute('aria-label', uiText('הסרה', 'Remove'));
                li.appendChild(he);
                li.appendChild(en);
                li.appendChild(remove);
                if (alts) li.appendChild(alts);
                var row = { li: li, he: he, en: en };
                rows.push(row);
                ctx.act(remove, function () {
                    rows.splice(rows.indexOf(row), 1);
                    items.removeChild(li);
                    ctx.touch(box);
                }, ctx.focusKey(info, 'remove'));
                return row;
            }

            Object.keys(original || {}).forEach(function (word) {
                addRow(word, typeof original[word] === 'string' ? original[word] : '', false, '');
            });
            place();

            var bar = make(ctx, 'div', 'plan-list-bar');
            var add = button(ctx, uiText('הוספה', 'Add'), 'btn btn-ghost admin-row-btn', { 'data-plan-add': '' });
            var suggest = button(ctx, 'הצעת פירושים (Suggest meanings)', 'btn btn-ghost admin-row-btn', {
                'data-plan-gloss-suggest': ''
            });
            bar.appendChild(add);
            bar.appendChild(suggest);
            box.appendChild(bar);
            box.appendChild(note);

            function say(he, en) {
                note.hidden = false;
                note.textContent = uiText(he, en);
            }

            ctx.act(add, function () {
                var row = addRow('', '', false, '');
                place();
                row.he.focus();
                ctx.touch(box);
            }, ctx.focusKey(info, 'add'));

            ctx.act(suggest, function () {
                var api = root.HebrewApi;
                var story = storyText(info, ctx);
                if (!api || typeof api.post !== 'function') return undefined;
                if (!story.text.trim()) {
                    say('אין עדיין טקסט בפסקאות.', 'The paragraphs have no text yet.');
                    return undefined;
                }
                if (rows.length >= GLOSS_MAX) {
                    say('כבר יש ' + GLOSS_MAX + ' פירושים.', 'Already at ' + GLOSS_MAX + ' meanings.');
                    return undefined;
                }
                say('מחפש פירושים…', 'Looking up meanings…');
                var skip = rows.map(function (row) { return String(row.he.value).trim(); })
                    .filter(Boolean).concat(story.support);
                return api.post('/api/dictionary/suggest-glosses', { text: story.text, skip: skip }).then(function (res) {
                    if (!res || !res.ok || !res.data || !Array.isArray(res.data.suggestions)) {
                        say('לא ניתן היה להציע פירושים.', 'Meanings could not be suggested.');
                        return;
                    }
                    var added = 0;
                    res.data.suggestions.forEach(function (s) {
                        if (rows.length >= GLOSS_MAX || !s || typeof s.word !== 'string' || !s.meaning) return;
                        addRow(s.word, s.meaning, true, typeof s.form === 'string' ? s.form : '', s.senses);
                        added += 1;
                    });
                    place();
                    var missing = Array.isArray(res.data.unmatched) ? res.data.unmatched.length : 0;
                    if (!added && !missing) {
                        say('לכל המילים כבר יש פירוש.', 'Every word already has a meaning.');
                        return;
                    }
                    ctx.touch(box);
                    say(
                        'נוספו ' + added + ' פירושים. קראו כל אחד מול הסיפור לפני השמירה.'
                            + (missing ? ' ל-' + missing + ' מילים אין ערך.' : ''),
                        'Added ' + added + ' draft meanings. Check each one against the story before you save.'
                            + (missing ? ' ' + missing + ' words had no entry.' : '')
                    );
                });
            }, ctx.focusKey(info, 'suggest'));

            return makeHandle(info, {
                el: box,
                control: rows.length ? rows[0].he : add,
                read: function () {
                    var out = {};
                    var count = 0;
                    rows.forEach(function (row) {
                        var word = String(row.he.value).trim();
                        var meaning = String(row.en.value).trim();
                        if (!word || !meaning) return;
                        if (!Object.prototype.hasOwnProperty.call(out, word)) count += 1;
                        out[word] = meaning;
                    });
                    return !count && !info.required && !original ? undefined : out;
                },
                check: function () {
                    var words = {};
                    rows.forEach(function (row) {
                        var word = String(row.he.value).trim();
                        if (word && String(row.en.value).trim()) words[word] = true;
                    });
                    return Object.keys(words).length > GLOSS_MAX
                        ? 'יותר מדי פירושים (Too many meanings): at most ' + GLOSS_MAX
                        : null;
                }
            });
        }
    };

    // ---- sound for a word --------------------------------------------------------------------

    var CLIP_PATH = /^(?:\.\.\/)?_audio\/words\/([A-Za-z0-9_.-]+\.mp3)$/;
    var clipsLoad = null;
    var playing = null;

    /** The page URL of a shared word clip, or '' for any other path. */
    function clipUrl(value) {
        var match = CLIP_PATH.exec(String(value || ''));
        return match ? '/worksheets/_audio/words/' + match[1] : '';
    }

    /** Every word that already has a clip. Asked once; a failure is asked again next time. */
    function loadClips() {
        var api = root.HebrewApi;
        if (!api || typeof api.get !== 'function') return Promise.resolve([]);
        if (!clipsLoad) {
            clipsLoad = api.get('/api/admin/word-audio').then(function (res) {
                return res && res.ok && res.data && Array.isArray(res.data.clips) ? res.data.clips : [];
            }).catch(function () {
                clipsLoad = null;
                return [];
            });
        }
        return clipsLoad;
    }

    function playClip(url) {
        var Player = root.Audio;
        if (typeof Player !== 'function') return Promise.resolve(false);
        try {
            if (playing && typeof playing.pause === 'function') playing.pause();
            playing = new Player(url);
            var started = playing.play();
            return Promise.resolve(started).then(function () { return true; }, function () { return false; });
        } catch (err) {
            return Promise.resolve(false);
        }
    }

    var AUDIO_ERRORS = {
        bad_word: ['המילה צריכה להיות בעברית.', 'The word needs to be Hebrew.'],
        audio_not_configured: ['השרת לא מוגדר ליצירת צליל.', 'The server is not set up to make sound.'],
        audio_failed: ['ElevenLabs לא החזיר צליל. נסו שוב.', 'ElevenLabs did not return sound. Try again.'],
        audio_throttle: ['יותר מדי צלילים חדשים בשעה האחרונה.', 'Too many new sounds this hour. Try again later.']
    };

    /**
     * The audio file of a word: a list of sounds that already exist, in place of a typed
     * path. Hear plays the chosen one. Make sound asks for this word's own clip (the one
     * that exists, or a new one made once). Nothing is saved until the word is saved.
     */
    var audioClipWidget = {
        name: 'audio-clip',
        test: function () { return 0; },
        render: function (info, ctx) {
            var original = typeof info.value === 'string' ? info.value : '';
            var box = make(ctx, 'div', 'plan-audio-clip');
            box.setAttribute('role', 'group');
            box.setAttribute('data-plan-audio-clip', '');
            var select = make(ctx, 'select');
            select.setAttribute('aria-label', uiText('קובץ שמע', 'Audio file'));
            select.setAttribute('data-plan-audio-select', '');
            // The list, in order. A word's own clip is kept first, then the rest.
            var entries = [];
            var bar = make(ctx, 'div', 'plan-list-bar');
            var hear = button(ctx, '▶ שמיעה (Hear)', 'btn btn-ghost admin-row-btn', { 'data-plan-audio-hear': '' });
            var made = button(ctx, 'יצירת צליל למילה (Make sound)', 'btn btn-ghost admin-row-btn', {
                'data-plan-audio-make': ''
            });
            var note = make(ctx, 'p', 'plan-note');
            note.hidden = true;
            note.setAttribute('data-plan-audio-note', '');

            function say(he, en) {
                note.hidden = false;
                note.textContent = uiText(he, en);
            }

            function wordHere() {
                var Form = root.HebrewPlanForm;
                var plan = typeof ctx.getPlan === 'function' ? ctx.getPlan() : null;
                if (!Form || typeof Form.nodeAt !== 'function' || !plan) return '';
                var part = Form.nodeAt(plan, info.path);
                return isObject(part) && typeof part.word === 'string' ? part.word.trim() : '';
            }

            function paintOptions(chosen) {
                while (select.firstChild) select.removeChild(select.firstChild);
                var none = make(ctx, 'option', '', uiText('ללא צליל', 'No sound'));
                none.value = '';
                select.appendChild(none);
                entries.forEach(function (entry) {
                    var option = make(ctx, 'option');
                    option.value = entry.path;
                    option.textContent = entry.label;
                    option.setAttribute('lang', 'he');
                    select.appendChild(option);
                });
                select.value = chosen;
            }

            /** Add a clip once; a clip for this word goes first. A known path gains its word. */
            function addOption(path, label, first) {
                if (!path) return;
                var at = -1;
                entries.forEach(function (entry, i) { if (entry.path === path) at = i; });
                if (at !== -1) {
                    if (label && entries[at].label === entries[at].path) entries[at].label = label;
                    return;
                }
                var entry = { path: path, label: label || path };
                if (first) entries.unshift(entry);
                else entries.push(entry);
            }

            addOption(original, original, false);
            paintOptions(original);

            loadClips().then(function (clips) {
                var mine = wordHere();
                clips.forEach(function (clip) {
                    if (clip && typeof clip.word === 'string' && typeof clip.audioPath === 'string') {
                        addOption(clip.audioPath, clip.word, !!mine && clip.word === mine && clip.audioPath !== original);
                    }
                });
                paintOptions(String(select.value || original));
                var hit = mine && clips.some(function (c) { return c && c.word === mine; });
                if (!original && hit) {
                    say('למילה הזו כבר יש צליל. בחרו אותו ברשימה.',
                        'This word already has a sound. Choose it in the list.');
                }
            });

            ctx.act(hear, function () {
                if (!select.value) {
                    say('בחרו צליל, או צרו אחד למילה.', 'Choose a sound, or make one for this word.');
                    return undefined;
                }
                var url = clipUrl(select.value);
                if (!url) {
                    say('אי אפשר להשמיע את הקובץ הזה מכאן.', 'This file cannot be played from here.');
                    return undefined;
                }
                return playClip(url).then(function (ok) {
                    if (!ok) say('הצליל לא הושמע.', 'The sound could not be played.');
                });
            }, ctx.focusKey(info, 'hear'));

            ctx.act(made, function () {
                var api = root.HebrewApi;
                var word = wordHere();
                if (!api || typeof api.post !== 'function') return undefined;
                if (!word) {
                    say('למילה עדיין אין טקסט.', 'The word has no text yet.');
                    return undefined;
                }
                say('מכין צליל…', 'Making the sound…');
                return api.post('/api/admin/word-audio', { text: word }).then(function (res) {
                    if (!res || !res.ok || !res.data || typeof res.data.audioPath !== 'string') {
                        var code = res && res.data && (res.data.code || res.data.error);
                        var line = AUDIO_ERRORS[code] || ['הצליל לא נוצר.', 'The sound could not be made.'];
                        say(line[0], line[1]);
                        return undefined;
                    }
                    addOption(res.data.audioPath, word, true);
                    paintOptions(res.data.audioPath);
                    clipsLoad = null;
                    ctx.touch(box);
                    say(
                        res.data.created
                            ? 'הצליל נוצר. שמרו את המילה כדי לשמור אותו.'
                            : 'כבר היה צליל למילה הזו. הוא נבחר. שמרו את המילה.',
                        res.data.created
                            ? 'Sound made. Save the word to keep it.'
                            : 'This word already had a sound, so it is chosen. Save the word to keep it.'
                    );
                    var url = clipUrl(res.data.audioPath);
                    return url ? playClip(url) : undefined;
                });
            }, ctx.focusKey(info, 'make'));

            bar.appendChild(hear);
            bar.appendChild(made);
            box.appendChild(select);
            box.appendChild(bar);
            box.appendChild(note);

            return makeHandle(info, {
                el: box,
                control: select,
                read: function () {
                    var value = String(select.value || '');
                    return value || undefined;
                }
            });
        }
    };

    // ---- exercise, assignment, conjugation cells (INIT-012/SPEC-007) -----------------------

    var PAST_FUTURE_CELLS = ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'];
    var PRESENT_CELLS = ['ms', 'fs', 'mp', 'fp'];
    var CELL_LABELS = {
        ms: ['זכר יחיד', 'Masculine singular'], fs: ['נקבה יחידה', 'Feminine singular'],
        mp: ['זכרים רבים', 'Masculine plural'], fp: ['נקבות רבות', 'Feminine plural'],
        '1s': ['אני', 'I'], '2ms': ['אתה', 'You, masculine'], '2fs': ['את', 'You, feminine'],
        '3ms': ['הוא', 'He'], '3fs': ['היא', 'She'], '1p': ['אנחנו', 'We'],
        '2mp': ['אתם', 'You, masculine plural'], '2fp': ['אתן', 'You, feminine plural'],
        '3p': ['הם והן', 'They']
    };

    function planWordsFromCtx(info, ctx) {
        var plan = typeof ctx.getPlan === 'function' ? ctx.getPlan() : null;
        var Builder = root.HebrewPlanBuilder;
        if (Builder && typeof Builder.planWordBank === 'function' && plan) return Builder.planWordBank(plan);
        return [];
    }

    function blockNodeFromCtx(info, ctx) {
        var Form = root.HebrewPlanForm;
        var plan = typeof ctx.getPlan === 'function' ? ctx.getPlan() : null;
        if (!Form || typeof Form.nodeAt !== 'function' || !plan) return null;
        return Form.nodeAt(plan, info.path);
    }

    function inputField(ctx, hint, value, multi) {
        var node = make(ctx, multi ? 'textarea' : 'input');
        if (multi) node.rows = hint.rows || 2;
        else node.type = 'text';
        applyLang(ctx, node, hint.lang);
        node.setAttribute('autocomplete', 'off');
        node.setAttribute('spellcheck', 'false');
        node.value = typeof value === 'string' ? value : '';
        return node;
    }

    function renderDictationAudio(ctx, info, item, box) {
        var sub = {
            def: info.def, prop: 'audioPath', key: info.key + '.audioPath', schema: { type: 'string' },
            required: true, value: item.audioPath || '', hint: { lang: 'latin' }, readOnly: false,
            level: info.level + 1, path: info.path, nodeKind: info.nodeKind
        };
        var clip = audioClipWidget.render(sub, Object.assign({}, ctx, {
            getPlan: function () {
                var plan = typeof ctx.getPlan === 'function' ? ctx.getPlan() : null;
                if (!plan) return plan;
                var copy = clone(plan);
                var Form = root.HebrewPlanForm;
                if (!Form || typeof Form.nodeAt !== 'function') return copy;
                var part = Form.nodeAt(copy, info.path);
                if (isObject(part) && typeof item.answer === 'string') part.word = item.answer;
                return copy;
            }
        }));
        box.appendChild(clip.el);
        ctx.onInput(clip.el, function () {
            item.audioPath = clip.getValue();
        });
        return clip;
    }

    var exerciseItemsWidget = {
        name: 'exercise-items',
        test: function (info) { return info.key === 'exerciseBlock.items' ? 70 : 0; },
        render: function (info, ctx) {
            var block = blockNodeFromCtx(info, ctx) || {};
            var kind = typeof block.kind === 'string' ? block.kind : 'choose';
            var chrome = listChrome(ctx, info);
            chrome.box.className += ' plan-exercise-editor';
            chrome.box.setAttribute('data-plan-exercise-items', '');
            var note = make(ctx, 'p', 'plan-note');
            note.hidden = true;
            note.setAttribute('data-plan-exercise-note', '');
            chrome.box.appendChild(note);
            var rows = [];

            function say(he, en) { note.hidden = false; note.textContent = uiText(he, en); }

            function readItems() {
                return rows.map(function (row) { return row.read(); });
            }

            function place() {
                rows.forEach(function (row, index) {
                    row.ops.up.disabled = index === 0;
                    row.ops.down.disabled = index === rows.length - 1;
                    chrome.items.appendChild(row.li);
                });
            }

            function renderChoose(item, li) {
                var body = make(ctx, 'div', 'plan-exercise-row-body');
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('שאלה', 'Prompt')));
                var prompt = inputField(ctx, { lang: 'mixed', rows: 2 }, item.prompt, true);
                body.appendChild(prompt);
                var choicesBox = make(ctx, 'div', 'plan-exercise-choices-edit');
                var choiceRows = [];
                function readChoices() {
                    return choiceRows.map(function (r) { return String(r.input.value); });
                }
                function paintChoices() {
                    choiceRows.forEach(function (r) { choicesBox.appendChild(r.li); });
                }
                function addChoice(text, mark) {
                    var cli = make(ctx, 'li', 'plan-exercise-choice-row');
                    var radio = make(ctx, 'input');
                    radio.type = 'radio';
                    radio.name = 'right-' + item.id;
                    radio.checked = !!mark;
                    var inp = inputField(ctx, { lang: 'mixed' }, text, false);
                    var rm = button(ctx, '×', 'btn btn-ghost admin-row-btn', { 'data-plan-choice-remove': '' });
                    cli.appendChild(radio);
                    cli.appendChild(inp);
                    cli.appendChild(rm);
                    var row = { li: cli, input: inp, radio: radio };
                    choiceRows.push(row);
                    ctx.act(rm, function () {
                        if (choiceRows.length <= 2) {
                            say('צריך לפחות שתי אפשרויות.', 'Keep at least two choices.');
                            return;
                        }
                        if (radio.checked) {
                            say('הסירו את הסימון לפני שמסירים את האפשרות, או סמנו אפשרות אחרת.', 'Clear the right mark first, or mark another choice.');
                            return;
                        }
                        choiceRows.splice(choiceRows.indexOf(row), 1);
                        choicesBox.removeChild(cli);
                        ctx.touch(chrome.box);
                    });
                    ctx.act(radio, function () {
                        choiceRows.forEach(function (row) { row.radio.checked = false; });
                        radio.checked = true;
                        ctx.touch(chrome.box);
                    });
                    return row;
                }
                (Array.isArray(item.choices) ? item.choices : ['', '']).forEach(function (c) {
                    addChoice(c, item.answer && c === item.answer);
                });
                paintChoices();
                var addChoiceBtn = button(ctx, uiText('הוספת אפשרות', 'Add choice'), 'btn btn-ghost admin-row-btn', {});
                ctx.act(addChoiceBtn, function () {
                    if (choiceRows.length >= 5) {
                        say('אפשר לכל היותר חמש אפשרויות.', 'At most five choices.');
                        return;
                    }
                    addChoice('', false);
                    paintChoices();
                    ctx.touch(chrome.box);
                });
                body.appendChild(choicesBox);
                body.appendChild(addChoiceBtn);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('למה (אופציונלי)', 'Why (optional)')));
                var why = inputField(ctx, { lang: 'en', rows: 2 }, item.why, true);
                body.appendChild(why);
                li.appendChild(body);
                return function () {
                    var choices = readChoices();
                    var answer = '';
                    choiceRows.forEach(function (r) {
                        if (r.radio.checked) answer = String(r.input.value).trim();
                    });
                    var out = { id: item.id, prompt: String(prompt.value).trim(), choices: choices, answer: answer };
                    var whyText = String(why.value).trim();
                    if (whyText) out.why = whyText;
                    return out;
                };
            }

            function renderTrueFalse(item, li) {
                var body = make(ctx, 'div', 'plan-exercise-row-body');
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('משפט', 'Statement')));
                var statement = inputField(ctx, { lang: 'mixed', rows: 2 }, item.statement, true);
                body.appendChild(statement);
                var bar = make(ctx, 'div', 'plan-exercise-tf');
                var trueBtn = button(ctx, uiText('נכון', 'True'), 'btn btn-ghost admin-row-btn', {});
                var falseBtn = button(ctx, uiText('לא נכון', 'False'), 'btn btn-ghost admin-row-btn', {});
                var val = item.answer === false ? false : true;
                function paint() {
                    trueBtn.className = 'btn btn-ghost admin-row-btn' + (val === true ? ' is-active' : '');
                    falseBtn.className = 'btn btn-ghost admin-row-btn' + (val === false ? ' is-active' : '');
                }
                paint();
                ctx.act(trueBtn, function () { val = true; paint(); ctx.touch(chrome.box); });
                ctx.act(falseBtn, function () { val = false; paint(); ctx.touch(chrome.box); });
                bar.appendChild(trueBtn);
                bar.appendChild(falseBtn);
                body.appendChild(bar);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('למה (אופציונלי)', 'Why (optional)')));
                var why = inputField(ctx, { lang: 'en', rows: 2 }, item.why, true);
                body.appendChild(why);
                li.appendChild(body);
                return function () {
                    var out = { id: item.id, statement: String(statement.value).trim(), answer: val === true };
                    var whyText = String(why.value).trim();
                    if (whyText) out.why = whyText;
                    return out;
                };
            }

            function stringListEditor(values, min, max, hint, shuffleNote) {
                var wrap = make(ctx, 'div', 'plan-string-sublist');
                var list = make(ctx, 'ol', 'plan-list-items');
                wrap.appendChild(list);
                if (shuffleNote) {
                    var sn = make(ctx, 'p', 'plan-note');
                    sn.textContent = uiText('הלומד רואה את המילים מעורבבות.', 'The learner sees the words shuffled.');
                    wrap.appendChild(sn);
                }
                var subs = [];
                function placeSub() { subs.forEach(function (s) { list.appendChild(s.li); }); }
                function add(text) {
                    var li = make(ctx, 'li', 'plan-list-item');
                    var inp = inputField(ctx, hint, text, false);
                    var rm = button(ctx, '×', 'btn btn-ghost admin-row-btn', {});
                    li.appendChild(inp);
                    li.appendChild(rm);
                    var row = { li: li, input: inp };
                    subs.push(row);
                    ctx.act(rm, function () {
                        if (subs.length <= min) {
                            say('מספר פריטים נמוך מדי.', 'Too few entries.');
                            return;
                        }
                        subs.splice(subs.indexOf(row), 1);
                        list.removeChild(li);
                        ctx.touch(chrome.box);
                    });
                    return row;
                }
                (Array.isArray(values) ? values : []).forEach(add);
                if (!subs.length) {
                    var i;
                    for (i = 0; i < min; i++) add('');
                }
                placeSub();
                var addBtn = button(ctx, uiText('הוספה', 'Add'), 'btn btn-ghost admin-row-btn', {});
                ctx.act(addBtn, function () {
                    if (subs.length >= max) {
                        say('יותר מדי פריטים.', 'Too many entries.');
                        return;
                    }
                    add('');
                    placeSub();
                    ctx.touch(chrome.box);
                });
                wrap.appendChild(addBtn);
                wrap.read = function () {
                    return subs.map(function (s) { return String(s.input.value).trim(); }).filter(Boolean);
                };
                return wrap;
            }

            function renderUnscramble(item, li) {
                var body = make(ctx, 'div', 'plan-exercise-row-body');
                var words = stringListEditor(item.words, 3, 10, { lang: 'he', pointed: true }, true);
                body.appendChild(words);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('אנגלית (אופציונלי)', 'English (optional)')));
                var english = inputField(ctx, { lang: 'en' }, item.english, false);
                body.appendChild(english);
                li.appendChild(body);
                return function () {
                    var out = { id: item.id, words: words.read() };
                    var en = String(english.value).trim();
                    if (en) out.english = en;
                    return out;
                };
            }

            function renderTranslate(item, li) {
                var body = make(ctx, 'div', 'plan-exercise-row-body');
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('רמז באנגלית', 'English cue')));
                var english = inputField(ctx, { lang: 'en' }, item.english, false);
                body.appendChild(english);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('תשובה בעברית', 'Hebrew answer')));
                var answer = inputField(ctx, { lang: 'he', pointed: true }, item.answer, false);
                body.appendChild(answer);
                var accept = stringListEditor(item.accept, 0, 8, { lang: 'he' }, false);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('כתיב חלופי', 'Alternate spellings')));
                body.appendChild(accept);
                li.appendChild(body);
                return function () {
                    var out = { id: item.id, english: String(english.value).trim(), answer: String(answer.value).trim() };
                    var acc = accept.read();
                    if (acc.length) out.accept = acc;
                    return out;
                };
            }

            function renderDictation(item, li) {
                var body = make(ctx, 'div', 'plan-exercise-row-body');
                var live = clone(item);
                renderDictationAudio(ctx, info, live, body);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('תשובה בעברית', 'Hebrew answer')));
                var answer = inputField(ctx, { lang: 'he', pointed: true }, item.answer, false);
                ctx.onInput(answer, function () { live.answer = String(answer.value); });
                body.appendChild(answer);
                var accept = stringListEditor(item.accept, 0, 8, { lang: 'he' }, false);
                body.appendChild(make(ctx, 'label', 'plan-mini-label', uiText('כתיב חלופי', 'Alternate spellings')));
                body.appendChild(accept);
                li.appendChild(body);
                return function () {
                    var out = { id: item.id, audioPath: live.audioPath || '', answer: String(answer.value).trim() };
                    var acc = accept.read();
                    if (acc.length) out.accept = acc;
                    return out;
                };
            }

            function addRow(item) {
                var li = make(ctx, 'li', 'plan-list-item plan-exercise-editor-row');
                var head = make(ctx, 'div', 'plan-item-head');
                var title = make(ctx, 'strong', 'plan-item-title', uiText('פריט', 'Item') + ' ' + (rows.length + 1));
                var ops = rowButtons(ctx, li);
                head.appendChild(title);
                head.appendChild(ops.el);
                li.appendChild(head);
                var readFn;
                if (kind === 'choose') readFn = renderChoose(item, li);
                else if (kind === 'true-false') readFn = renderTrueFalse(item, li);
                else if (kind === 'unscramble') readFn = renderUnscramble(item, li);
                else if (kind === 'translate') readFn = renderTranslate(item, li);
                else readFn = renderDictation(item, li);
                var row = { li: li, ops: ops, read: readFn, title: title };
                rows.push(row);
                ctx.act(ops.up, function () {
                    if (reorder(rows, rows.indexOf(row), rows.indexOf(row) - 1)) place();
                }, ctx.focusKey(info, 'up'));
                ctx.act(ops.down, function () {
                    if (reorder(rows, rows.indexOf(row), rows.indexOf(row) + 1)) place();
                }, ctx.focusKey(info, 'down'));
                ctx.act(ops.remove, function () {
                    if (rows.length <= 1) {
                        say('לא ניתן להסיר את הפריט היחיד.', 'You cannot remove the only item.');
                        return;
                    }
                    rows.splice(rows.indexOf(row), 1);
                    chrome.items.removeChild(li);
                    rows.forEach(function (r, idx) {
                        r.title.textContent = uiText('פריט', 'Item') + ' ' + (idx + 1);
                    });
                    ctx.touch(chrome.box);
                }, ctx.focusKey(info, 'remove'));
                return row;
            }

            (Array.isArray(info.value) ? info.value : []).forEach(function (item) {
                if (isObject(item)) addRow(clone(item));
            });
            function skeletonItem(id) {
                var Form = root.HebrewPlanForm;
                var t = typeof ctx.tools === 'function' ? ctx.tools() : null;
                if (Form && typeof Form.freshExerciseItem === 'function' && t) {
                    return Form.freshExerciseItem(kind, t, id);
                }
                var base = ctx.skeleton(info.itemSchema, info.itemDef) || {};
                base.id = id;
                if (kind === 'choose') {
                    base.choices = ['', ''];
                    base.answer = '';
                } else if (kind === 'unscramble') base.words = ['', '', ''];
                return base;
            }

            if (!rows.length) addRow(skeletonItem('item-1'));
            place();
            ctx.act(chrome.add, function () {
                if (rows.length >= 12) {
                    say('אפשר לכל היותר שנים עשר פריטים.', 'At most twelve items.');
                    return;
                }
                var id = 'item-' + nextId(readItems());
                addRow(skeletonItem(id));
                place();
                ctx.touch(chrome.box);
            }, ctx.focusKey(info, 'add'));
            return makeHandle(info, {
                el: chrome.box,
                control: chrome.add,
                group: true,
                read: function () { return readItems(); }
            });
        }
    };

    var assignmentUseWordsWidget = {
        name: 'assignment-use-words',
        test: function (info) { return info.key === 'assignmentBlock.useWords' ? 70 : 0; },
        render: function (info, ctx) {
            var box = make(ctx, 'div', 'plan-assignment-words-edit');
            box.setAttribute('data-plan-assignment-use-words', '');
            box.setAttribute('lang', 'he');
            box.setAttribute('dir', 'rtl');
            var picked = make(ctx, 'ul', 'plan-list-items plan-assignment-picked');
            box.appendChild(picked);
            var bank = make(ctx, 'div', 'plan-assignment-bank');
            var note = make(ctx, 'p', 'plan-note');
            note.hidden = true;
            box.appendChild(bank);
            box.appendChild(note);
            var rows = [];
            var selected = {};
            (Array.isArray(info.value) ? info.value : []).forEach(function (w) {
                if (typeof w === 'string' && w) selected[w] = true;
            });

            function say(he, en) { note.hidden = false; note.textContent = uiText(he, en); }

            function paintPicked() {
                while (picked.firstChild) picked.removeChild(picked.firstChild);
                rows.forEach(function (row) { picked.appendChild(row.li); });
            }

            function addPicked(word) {
                if (rows.some(function (r) { return r.word === word; })) return;
                selected[word] = true;
                var li = make(ctx, 'li', 'plan-list-item');
                var span = make(ctx, 'span', 'plan-assignment-picked-word', word);
                span.setAttribute('lang', 'he');
                span.setAttribute('dir', 'rtl');
                var rm = button(ctx, '×', 'btn btn-ghost admin-row-btn', {});
                li.appendChild(span);
                li.appendChild(rm);
                var row = { li: li, word: word, span: span };
                rows.push(row);
                ctx.act(rm, function () {
                    delete selected[word];
                    rows.splice(rows.indexOf(row), 1);
                    paintPicked();
                    ctx.touch(box);
                });
                paintPicked();
            }

            Object.keys(selected).forEach(addPicked);

            function paintBank() {
                while (bank.firstChild) bank.removeChild(bank.firstChild);
                var words = planWordsFromCtx(info, ctx);
                if (!words.length) {
                    var empty = make(ctx, 'p', 'plan-note');
                    empty.textContent = uiText('אין עדיין מילים במאגר התוכנית.', 'No words in the plan word bank yet.');
                    bank.appendChild(empty);
                    return;
                }
                words.forEach(function (entry) {
                    if (!entry || typeof entry.word !== 'string') return;
                    var btn = button(ctx, entry.word, 'btn btn-ghost admin-row-btn plan-assignment-bank-word', {});
                    btn.setAttribute('lang', 'he');
                    btn.setAttribute('dir', 'rtl');
                    if (selected[entry.word]) btn.disabled = true;
                    ctx.act(btn, function () {
                        if (rows.length >= 12) {
                            say('אפשר לבחור לכל היותר שנים עשר מילים.', 'At most twelve words.');
                            return;
                        }
                        addPicked(entry.word);
                        btn.disabled = true;
                        ctx.touch(box);
                    });
                    bank.appendChild(btn);
                });
            }
            paintBank();

            var focusHe = rows.length ? rows[0].span : make(ctx, 'span', '', uiText('מילים', 'Words'));
            if (!rows.length) {
                focusHe.setAttribute('lang', 'he');
                focusHe.setAttribute('dir', 'rtl');
                focusHe.hidden = true;
                box.appendChild(focusHe);
            }
            return makeHandle(info, {
                el: box,
                control: focusHe,
                group: true,
                read: function () {
                    var out = rows.map(function (r) { return r.word; });
                    return !out.length && !info.required && !Array.isArray(info.value) ? undefined : out;
                }
            });
        }
    };

    var assignmentHomeworkWidget = {
        name: 'assignment-homework',
        test: function (info) { return info.key === 'assignmentBlock.homework' ? 70 : 0; },
        render: function (info, ctx) {
            var chrome = listChrome(ctx, info);
            chrome.box.className += ' plan-assignment-homework-edit';
            chrome.box.setAttribute('data-plan-assignment-homework', '');
            var note = make(ctx, 'p', 'plan-note');
            note.hidden = true;
            chrome.box.appendChild(note);
            var rows = [];
            function say(he, en) { note.hidden = false; note.textContent = uiText(he, en); }
            function addRow(text) {
                var li = make(ctx, 'li', 'plan-list-item');
                var inp = inputField(ctx, { lang: 'mixed', rows: 2 }, text, true);
                var rm = button(ctx, '×', 'btn btn-ghost admin-row-btn', {});
                li.appendChild(inp);
                li.appendChild(rm);
                var row = { li: li, input: inp };
                rows.push(row);
                ctx.act(rm, function () {
                    rows.splice(rows.indexOf(row), 1);
                    chrome.items.removeChild(li);
                    ctx.touch(chrome.box);
                });
                return row;
            }
            (Array.isArray(info.value) ? info.value : []).forEach(addRow);
            rows.forEach(function (r) { chrome.items.appendChild(r.li); });
            ctx.act(chrome.add, function () {
                if (rows.length >= 6) {
                    say('אפשר לכל היותר שש שורות שיעורי בית.', 'At most six homework lines.');
                    return;
                }
                var row = addRow('');
                chrome.items.appendChild(row.li);
                row.input.focus();
                ctx.touch(chrome.box);
            }, ctx.focusKey(info, 'add'));
            return makeHandle(info, {
                el: chrome.box,
                control: chrome.add,
                group: true,
                read: function () {
                    var out = rows.map(function (r) { return String(r.input.value).trim(); }).filter(Boolean);
                    return !out.length && !info.required && !Array.isArray(info.value) ? undefined : out;
                }
            });
        }
    };

    var tableCellsWidget = {
        name: 'table-cells',
        test: function (info) { return info.key === 'conjugationTableBlock.cells' ? 70 : 0; },
        render: function (info, ctx) {
            var block = blockNodeFromCtx(info, ctx) || {};
            var tense = block.tense === 'past' || block.tense === 'future' ? block.tense : 'present';
            var order = tense === 'present' ? PRESENT_CELLS : PAST_FUTURE_CELLS;
            var box = make(ctx, 'div', 'plan-table-cells');
            box.setAttribute('data-plan-table-cells', '');
            var list = make(ctx, 'ol', 'plan-list-items');
            box.appendChild(list);
            var byCell = {};
            (Array.isArray(info.value) ? info.value : []).forEach(function (row) {
                if (isObject(row) && typeof row.cell === 'string') byCell[row.cell] = row;
            });
            var rows = [];
            order.forEach(function (code) {
                var li = make(ctx, 'li', 'plan-list-item plan-table-cell-row');
                var lab = CELL_LABELS[code] || [code, code];
                var label = make(ctx, 'span', 'plan-table-cell-label', uiText(lab[0], lab[1]));
                var inp = inputField(ctx, { lang: 'he', pointed: true }, byCell[code] && byCell[code].formKey, false);
                inp.setAttribute('data-plan-cell-code', code);
                var msg = make(ctx, 'p', 'plan-note');
                msg.hidden = true;
                li.appendChild(label);
                li.appendChild(inp);
                li.appendChild(msg);
                list.appendChild(li);
                rows.push({ code: code, input: inp, msg: msg });
            });

            function checkForm(row) {
                var key = String(row.input.value).trim();
                row.msg.hidden = true;
                if (!key) return;
                var api = root.HebrewApi;
                if (!api || typeof api.get !== 'function') return;
                api.get('/api/lexicon/' + encodeURIComponent(key)).then(function (res) {
                    if (!res || !res.ok || !res.data || !res.data.entry) {
                        row.msg.hidden = false;
                        row.msg.textContent = uiText('הצורה לא במילון.', 'This form is not in the dictionary.');
                        return;
                    }
                    var entry = res.data.entry;
                    if (entry.verified !== 1 && entry.verified !== true) {
                        row.msg.hidden = false;
                        row.msg.textContent = uiText('הצורה לא אומתה עדיין.', 'This form is not verified yet.');
                    }
                }).catch(function () { /* ignore */ });
            }
            rows.forEach(function (row) {
                ctx.onInput(row.input, function () { checkForm(row); ctx.touch(box); });
                checkForm(row);
            });
            return makeHandle(info, {
                el: box,
                control: rows[0] ? rows[0].input : box,
                group: true,
                read: function () {
                    return rows.map(function (row) {
                        return { cell: row.code, formKey: String(row.input.value).trim() };
                    }).filter(function (row) { return row.formKey !== ''; });
                }
            });
        }
    };

    register(Object.assign({ isDefault: false }, exerciseItemsWidget));
    register(Object.assign({ isDefault: false }, assignmentUseWordsWidget));
    register(Object.assign({ isDefault: false }, assignmentHomeworkWidget));
    register(Object.assign({ isDefault: false }, tableCellsWidget));

    // ---- default registrations -------------------------------------------------------------

    [
        readonlyWidget,
        selectWidget,
        checkboxWidget,
        numberWidget,
        textWidget(false),
        textWidget(true),
        stringListWidget,
        objectWidget,
        objectListWidget,
        bucketListWidget,
        jsonWidget
    ].forEach(register);
    register(Object.assign({ isDefault: false }, glossMapWidget));
    register(Object.assign({ isDefault: false }, audioClipWidget));

    return {
        register: register,
        unregister: unregister,
        byName: byName,
        pick: pick,
        list: list,
        makeHandle: makeHandle,
        clone: clone,
        deepEqual: deepEqual,
        isObject: isObject,
        nextId: nextId,
        optionLabel: optionLabel,
        labelText: labelText,
        DEFAULT_NAMES: [
            'exercise-items', 'assignment-use-words', 'assignment-homework', 'table-cells',
            'readonly', 'select', 'checkbox', 'number', 'text', 'textarea',
            'string-list', 'object', 'object-list', 'bucket-list', 'json', 'gloss-map', 'audio-clip'
        ]
    };
});
