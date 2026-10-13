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
 * Provenance: INIT-008/SPEC-011, INIT-008/SPEC-012
 */
(function (root, factory) {
    'use strict';

    var api = factory();
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanWidgets = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
    'use strict';

    var registry = [];
    var LONG_TEXT = 200;

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
        if (text !== undefined) node.textContent = text;
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

    function textWidget(multi) {
        return {
            name: multi ? 'textarea' : 'text',
            test: function (info) {
                if (!has(info.schema, 'string') || info.schema.enum || info.schema.const !== undefined) return 0;
                return wantsTextarea(info) === multi ? 40 : 0;
            },
            render: function (info, ctx) {
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
                var option = make(ctx, 'option', '', optionLabel(info.hint, value));
                option.value = String(value);
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
        var add = button(ctx, 'הוספה (Add)', 'btn btn-ghost admin-row-btn', { 'data-plan-add': '' });
        var bar = make(ctx, 'div', 'plan-list-bar');
        bar.appendChild(add);
        box.appendChild(bar);
        return { box: box, items: items, bar: bar, add: add };
    }

    function rowButtons(ctx, row) {
        var wrap = make(ctx, 'span', 'plan-item-ops');
        var up = button(ctx, '↑', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'up' });
        up.setAttribute('aria-label', 'למעלה (Up)');
        var down = button(ctx, '↓', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'down' });
        down.setAttribute('aria-label', 'למטה (Down)');
        var remove = button(ctx, '×', 'btn btn-ghost admin-row-btn', { 'data-plan-item-op': 'remove' });
        remove.setAttribute('aria-label', 'הסרה (Remove)');
        wrap.appendChild(up);
        wrap.appendChild(down);
        wrap.appendChild(remove);
        return { el: wrap, up: up, down: down, remove: remove };
    }

    var stringListWidget = {
        name: 'string-list',
        test: function (info) {
            if (!has(info.schema, 'array') || !info.itemSchema) return 0;
            return has(info.itemSchema, 'string') && !info.itemSchema.enum ? 40 : 0;
        },
        render: function (info, ctx) {
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
                addBtn = button(ctx, 'הוספה (Add)', 'btn btn-ghost admin-row-btn', { 'data-plan-add': '' });
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
                    if (!ctx.confirm('להסיר את הפריט? (Remove this item?)')) return undefined;
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
            'readonly', 'select', 'checkbox', 'number', 'text', 'textarea',
            'string-list', 'object', 'object-list', 'bucket-list', 'json'
        ]
    };
});
