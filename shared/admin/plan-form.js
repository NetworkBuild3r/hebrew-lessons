/**
 * Schema-driven editor core for lesson plans. Given the plan JSON Schema (the text of
 * GET /api/plan-schema), it renders an edit form for any part of a plan: the plan root,
 * a unit, a lesson, a block, or a bucket item (a word, paragraph, line, or question).
 * Each property gets a widget from plan-widgets.js, a bilingual label and a language
 * (dir and lang) from plan-hints.js, and inline issues at the field path. A node the
 * generator does not understand gets a raw-JSON editor, and any property with no
 * purpose-built widget gets the JSON widget, so nothing in the schema is uneditable.
 *
 * The editor builds no plan document and no patch of one. A save is one operation
 * through the transport the host supplies:
 *   a field          transport.setField(path, field, value, hooks)   PATCH .../fields
 *   an absent field  transport.putPart(path, node, hooks)        PUT .../parts
 *                    (the node as loaded, without that one property)
 *   raw node JSON    transport.putPart(path, parsed JSON, hooks)
 * The host adds `expectedRevision` as the server last returned it. The server
 * validates; the editor shows what it says.
 *
 *   var editor = HebrewPlanForm.createEditor({ doc, host, signal, schema, transport, track,
 *                                              onToggle, confirm });
 *   editor.loadSchema(HebrewApi)                 -> Promise  (once per mount)
 *   editor.renderNode({ path, node, def?, kind?, plan? })   -> form { el, path, kind, def, fields }
 *   editor.captureDrafts() / editor.reset()      keep unsaved text across a repaint
 *   editor.setIssues(findings, plan)             inline issues at field paths
 *   editor.hasUnsaved()                          for canLeave
 *   editor.focusKey() / editor.restoreFocus(key)
 *   editor.destroy()                             removes the delegated listeners
 *
 * Pure helpers: newBlock, findUncoveredSchemaNodes, splitPointer, createTools, langAttrs.
 * Plain browser script, no build step; every string from the server is set with
 * textContent or .value. Node tests load it with require().
 * Provenance: INIT-008/SPEC-011, INIT-008/SPEC-012
 */
(function (root, factory) {
    'use strict';

    if (typeof module !== 'undefined' && module.exports && typeof require === 'function') {
        require('./plan-hints.js');
        require('./plan-widgets.js');
    }
    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanForm = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var Hints = root.HebrewPlanHints;
    var Widgets = root.HebrewPlanWidgets;
    var BUCKETS = ['words', 'paragraphs', 'lines', 'questions'];
    var CHILD_PROPS = { plan: 'units', unit: 'lessons', lesson: 'blocks' };
    var PLAN_READ_ONLY = ['schemaVersion', 'id', 'revision', 'status', 'audience', 'assigneeId'];
    var SUMMARY_PROPS = ['word', 'text', 'pointed', 'he', 'question', 'heading', 'title', 'name'];
    var DEFINITIONS = '#/definitions/';

    var currentTools = null;

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    function clone(value) {
        return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
    }

    function has(schema, type) {
        return !!schema && [].concat(schema.type || []).indexOf(type) !== -1;
    }

    // ---- the schema ------------------------------------------------------------------------

    /**
     * A reader over one parsed plan JSON Schema. `$ref` resolves against `#/definitions/`
     * only; a remote reference resolves to nothing.
     * @param {object} schema
     */
    function createTools(schema) {
        var defs = isObject(schema.definitions) ? schema.definitions : {};

        function defSchema(name) {
            if (name === 'plan') return schema;
            return Object.prototype.hasOwnProperty.call(defs, name) ? defs[name] : null;
        }

        /** Follow a `$ref` chain. `def` is the name of the last definition reached. */
        function resolve(node) {
            var def = null;
            var current = node;
            var hops = 0;
            while (isObject(current) && typeof current.$ref === 'string' && hops < 16) {
                hops += 1;
                if (current.$ref.indexOf(DEFINITIONS) !== 0) return { schema: null, def: null };
                def = current.$ref.slice(DEFINITIONS.length);
                current = defSchema(def);
            }
            return { schema: isObject(current) ? current : null, def: def };
        }

        function requiredOf(node) {
            return node && Array.isArray(node.required) ? node.required.slice() : [];
        }

        /** The properties of an object definition, in schema order. */
        function propsOf(node, def, only) {
            if (!node || !isObject(node.properties)) return [];
            var required = requiredOf(node);
            var out = [];
            Object.keys(node.properties).forEach(function (prop) {
                if (only && only.indexOf(prop) === -1) return;
                out.push({ prop: prop, raw: node.properties[prop], required: required.indexOf(prop) !== -1, def: def });
            });
            return out;
        }

        /** The block types the schema's `block` oneOf holds, as `{ type, def }`. */
        function blockTypes() {
            var lesson = defSchema('lesson');
            var raw = lesson && lesson.properties && lesson.properties.blocks && lesson.properties.blocks.items;
            var resolved = raw ? resolve(raw) : resolve({ $ref: DEFINITIONS + 'block' });
            var branches = resolved.schema && Array.isArray(resolved.schema.oneOf) ? resolved.schema.oneOf : [];
            var out = [];
            branches.forEach(function (branch) {
                var r = resolve(branch);
                var type = r.schema && r.schema.properties && r.schema.properties.type
                    ? r.schema.properties.type.const : undefined;
                if (typeof type === 'string') out.push({ type: type, def: r.def });
            });
            return out;
        }

        function blockDef(type) {
            var list = blockTypes();
            var i;
            for (i = 0; i < list.length; i++) {
                if (list[i].type === type) return list[i].def;
            }
            return null;
        }

        /** The definition of the items of `bucket` in a block definition, or null. */
        function itemDef(blockDefName, bucket) {
            var node = defSchema(blockDefName);
            var raw = node && node.properties ? node.properties[bucket] : null;
            if (!raw) return null;
            var arr = resolve(raw).schema;
            return arr && arr.items ? resolve(arr.items).def : null;
        }

        return {
            schema: schema,
            defs: defs,
            defSchema: defSchema,
            resolve: resolve,
            propsOf: propsOf,
            requiredOf: requiredOf,
            blockTypes: blockTypes,
            blockDef: blockDef,
            itemDef: itemDef
        };
    }

    /**
     * The smallest value that has every required property: an empty string, an empty list,
     * the schema's minimum, the first enum value, the hint's default. It writes no Hebrew.
     */
    function makeSkeleton(tools) {
        function skeleton(node, def) {
            if (!node) return null;
            if (node.const !== undefined) return clone(node.const);
            if (Array.isArray(node.enum)) return node.enum[0];
            if (has(node, 'string')) return '';
            if (has(node, 'integer') || has(node, 'number')) return node.minimum !== undefined ? node.minimum : 0;
            if (has(node, 'boolean')) return false;
            if (has(node, 'array')) return [];
            if (isObject(node.properties)) {
                var out = {};
                tools.requiredOf(node).forEach(function (prop) {
                    var raw = node.properties[prop];
                    if (!raw) return;
                    var hint = def ? Hints.fieldHint(def + '.' + prop) : {};
                    if (hint.default !== undefined && prop !== 'id') {
                        out[prop] = clone(hint.default);
                        return;
                    }
                    var r = tools.resolve(raw);
                    out[prop] = skeleton(r.schema, r.def);
                });
                return out;
            }
            return null;
        }
        return skeleton;
    }

    function setSchema(schema) {
        var parsed = typeof schema === 'string' ? JSON.parse(schema) : schema;
        currentTools = parsed ? createTools(parsed) : null;
        return currentTools;
    }

    /**
     * A new block of one type, from the schema's `required` list. No Hebrew is generated;
     * a required string is empty, so the server will say which field to fill in.
     * @param {string} type
     * @param {string} id
     * @param {object} [schema] A parsed schema; the one last loaded when omitted.
     * @returns {object|null}
     */
    function newBlock(type, id, schema) {
        var tools = schema ? createTools(schema) : currentTools;
        if (!tools) return null;
        var def = tools.blockDef(type);
        if (!def) return null;
        var block = makeSkeleton(tools)(tools.defSchema(def), def);
        if (!isObject(block)) return null;
        block.id = id;
        return block;
    }

    /**
     * Block types the loaded schema offers, as `{ type, he, en }` for a select.
     * @param {object} [schema]
     */
    function blockTypeList(schema) {
        var tools = schema ? createTools(schema) : currentTools;
        if (!tools) return [];
        return tools.blockTypes().map(function (entry) {
            var hint = Hints.blocks[entry.type];
            return {
                type: entry.type,
                he: hint ? hint.label[0] : entry.type,
                en: hint ? hint.label[1] : entry.type
            };
        });
    }

    /** The values of an enum property of a definition, for example the lesson recipes. */
    function enumOf(def, prop, schema) {
        var tools = schema ? createTools(schema) : currentTools;
        var node = tools ? tools.defSchema(def) : null;
        var raw = node && node.properties ? node.properties[prop] : null;
        var resolved = raw ? tools.resolve(raw).schema : null;
        return resolved && Array.isArray(resolved.enum) ? resolved.enum.slice() : [];
    }

    // ---- the drift checker ------------------------------------------------------------------

    /**
     * What the editor does not cover. A pure function of a schema and the registries:
     * it reports a block type with no hint or exemption, a property with neither, a hint
     * that names a path the schema lacks, a widget no hint or default uses, a hint that
     * names an unknown widget, and an exemption with no written reason.
     *
     * A property whose hint or exemption forces the JSON widget (or marks it read only)
     * covers its whole subtree, so the walk does not descend into it.
     * @param {object} schema A parsed plan JSON Schema.
     * @param {{ hints: object, exemptions?: object, widgets?: { name: string, isDefault: boolean }[] }} registries
     * @returns {{ kind: string, key: string, message: string }[]}
     */
    function findUncoveredSchemaNodes(schema, registries) {
        var tools = createTools(schema);
        var hints = registries.hints || { fields: {}, blocks: {}, kinds: {} };
        var exemptions = registries.exemptions || {};
        var widgets = registries.widgets || [];
        var problems = [];
        var visited = {};
        var typesSeen = {};
        var propertyKeys = {};

        function report(kind, key, message) {
            problems.push({ kind: kind, key: key, message: message });
        }

        function visit(def, node) {
            if (visited[def]) return;
            visited[def] = true;
            Object.keys(node.properties || {}).forEach(function (prop) {
                var key = def + '.' + prop;
                var raw = node.properties[prop];
                var resolved = tools.resolve(raw);
                var hint = hints.fields[key];
                var exemption = exemptions[key];
                propertyKeys[key] = true;
                var structural = prop === 'id' || (prop === 'type' && resolved.schema && resolved.schema.const !== undefined);
                if (!hint && !exemption && !structural) {
                    report('property', key, 'property ' + key + ' has no hint and no exemption');
                }
                var entry = hint || exemption || {};
                if (entry.widget === 'json' || entry.readOnly) return;
                descend(raw);
            });
        }

        function descend(raw) {
            var resolved = tools.resolve(raw);
            var node = resolved.schema;
            if (!node) return;
            if (Array.isArray(node.oneOf)) {
                node.oneOf.forEach(function (branch) {
                    var r = tools.resolve(branch);
                    if (!r.schema || !r.def) return;
                    var type = r.schema.properties && r.schema.properties.type
                        ? r.schema.properties.type.const : undefined;
                    if (typeof type === 'string') typesSeen[type] = true;
                    visit(r.def, r.schema);
                });
                return;
            }
            if (isObject(node.properties) && resolved.def) {
                visit(resolved.def, node);
                return;
            }
            if (node.items) descend(node.items);
            if (isObject(node.additionalProperties)) descend(node.additionalProperties);
        }

        visit('plan', schema);

        Object.keys(typesSeen).forEach(function (type) {
            if (!hints.blocks[type] && !exemptions['block:' + type]) {
                report('block-type', type, 'block type ' + type + ' has no hint and no exemption');
            }
        });

        function pathExists(key) {
            var dot = key.indexOf('.');
            if (dot < 1) return false;
            var node = tools.defSchema(key.slice(0, dot));
            return !!node && isObject(node.properties)
                && Object.prototype.hasOwnProperty.call(node.properties, key.slice(dot + 1));
        }

        var allTypes = {};
        tools.blockTypes().forEach(function (entry) { allTypes[entry.type] = true; });
        var used = {};

        function checkEntry(key, entry) {
            if (entry && entry.widget) used[entry.widget] = true;
            if (entry && entry.widget && !widgets.some(function (w) { return w.name === entry.widget; })) {
                report('widget-name', key, 'hint ' + key + ' names the unknown widget ' + entry.widget);
            }
        }

        Object.keys(hints.fields).forEach(function (key) {
            if (!pathExists(key)) report('hint-path', key, 'hint ' + key + ' names a property the schema does not have');
            checkEntry(key, hints.fields[key]);
        });
        Object.keys(exemptions).forEach(function (key) {
            if (key.indexOf('block:') === 0) {
                if (!allTypes[key.slice(6)]) report('hint-path', key, 'exemption ' + key + ' names a block type the schema does not have');
            } else if (!pathExists(key)) {
                report('hint-path', key, 'exemption ' + key + ' names a property the schema does not have');
            }
            checkEntry(key, exemptions[key]);
            if (!exemptions[key] || typeof exemptions[key].reason !== 'string' || !exemptions[key].reason.trim()) {
                report('exemption-reason', key, 'exemption ' + key + ' has no written reason');
            }
        });
        Object.keys(hints.blocks).forEach(function (type) {
            if (!allTypes[type]) report('hint-path', 'block:' + type, 'hint for block type ' + type + ' names a type the schema does not have');
        });
        Object.keys(hints.kinds || {}).forEach(function (def) {
            if (!tools.defSchema(def)) report('hint-path', 'def:' + def, 'title for ' + def + ' names a definition the schema does not have');
        });
        widgets.forEach(function (w) {
            if (!w.isDefault && !used[w.name]) {
                report('widget', w.name, 'widget ' + w.name + ' is not a default and no hint names it');
            }
        });
        return problems;
    }

    // ---- pointers and paths -----------------------------------------------------------------

    function unescapeSegment(segment) {
        return String(segment).replace(/~1/g, '/').replace(/~0/g, '~');
    }

    function addressable(items, index) {
        var item = Array.isArray(items) ? items[index] : undefined;
        if (!isObject(item) || typeof item.id !== 'string' || item.id === '') return false;
        var first = -1;
        var i;
        for (i = 0; i < items.length; i++) {
            if (isObject(items[i]) && items[i].id === item.id) {
                first = i;
                break;
            }
        }
        return first === index;
    }

    /**
     * An RFC 6901 pointer into a plan (the validator's `path`, with array indexes) as the
     * deepest id path an operation accepts plus the rest of the pointer. The same rule as
     * the server's toIdPath, so an issue lands on the form of the node it is about.
     * @param {object} plan
     * @param {string} pointer
     * @returns {{ path: string, field: string }}
     */
    function splitPointer(plan, pointer) {
        var segs = typeof pointer === 'string' && pointer !== '' ? pointer.split('/').slice(1) : [];
        var parts = [];
        var used = 0;
        if (isObject(plan) && segs.length >= 2 && segs[0] === 'units') {
            var u = Number(unescapeSegment(segs[1]));
            if (Number.isInteger(u) && addressable(plan.units, u)) {
                var unit = plan.units[u];
                parts.push('units', unit.id);
                used = 2;
                var l = segs.length >= 4 && segs[2] === 'lessons' ? Number(unescapeSegment(segs[3])) : NaN;
                if (Number.isInteger(l) && addressable(unit.lessons, l)) {
                    var lesson = unit.lessons[l];
                    parts.push('lessons', lesson.id);
                    used = 4;
                    var b = segs.length >= 6 && segs[4] === 'blocks' ? Number(unescapeSegment(segs[5])) : NaN;
                    if (Number.isInteger(b) && addressable(lesson.blocks, b)) {
                        var blk = lesson.blocks[b];
                        parts.push('blocks', blk.id);
                        used = 6;
                        var bucket = segs.length >= 8 ? unescapeSegment(segs[6]) : '';
                        var m = bucket ? Number(unescapeSegment(segs[7])) : NaN;
                        if (BUCKETS.indexOf(bucket) !== -1 && Number.isInteger(m) && addressable(blk[bucket], m)) {
                            parts.push(bucket, blk[bucket][m].id);
                            used = 8;
                        }
                    }
                }
            }
        }
        return { path: parts.join('/'), field: segs.slice(used).join('/') };
    }

    function kindOfPath(path) {
        var n = String(path || '').split('/').filter(Boolean).length;
        if (n === 0) return 'plan';
        if (n === 2) return 'unit';
        if (n === 4) return 'lesson';
        if (n === 6) return 'block';
        if (n === 8) return 'item';
        return null;
    }

    /** The node at an id path in a plan document, or null. */
    function nodeAt(plan, path) {
        var segs = String(path || '').split('/').filter(Boolean);
        if (!isObject(plan)) return null;
        if (!segs.length) return plan;
        function byId(list, id) {
            var i;
            for (i = 0; Array.isArray(list) && i < list.length; i++) {
                if (isObject(list[i]) && list[i].id === id) return list[i];
            }
            return null;
        }
        var unit = segs.length >= 2 && segs[0] === 'units' ? byId(plan.units, segs[1]) : null;
        if (segs.length === 2) return unit;
        var lesson = unit && segs[2] === 'lessons' ? byId(unit.lessons, segs[3]) : null;
        if (segs.length === 4) return lesson;
        var blk = lesson && segs[4] === 'blocks' ? byId(lesson.blocks, segs[5]) : null;
        if (segs.length === 6) return blk;
        if (blk && segs.length === 8 && BUCKETS.indexOf(segs[6]) !== -1) return byId(blk[segs[6]], segs[7]);
        return null;
    }

    function without(node, prop) {
        var copy = clone(node);
        delete copy[prop];
        return copy;
    }

    function firstSegment(field) {
        var first = String(field || '').split('/')[0];
        return unescapeSegment(first);
    }

    // ---- the editor --------------------------------------------------------------------------

    /**
     * @param {object} opts see the file header
     */
    function createEditor(opts) {
        var options = opts || {};
        var doc = options.doc || root.document;
        var transport = options.transport || {};
        var tools = null;
        var skeleton = null;
        var acts = {};
        var inps = {};
        var seq = 0;
        var handles = [];
        var forms = {};
        var drafts = {};
        var openState = {};
        var attached = [];
        var unplaced = [];

        function setTools(schema) {
            tools = createTools(typeof schema === 'string' ? JSON.parse(schema) : schema);
            skeleton = makeSkeleton(tools);
            currentTools = tools;
        }
        if (options.schema) setTools(options.schema);

        function el(tag, className, text) {
            var node = doc.createElement(tag);
            if (className) node.className = className;
            if (text !== undefined) node.textContent = text;
            return node;
        }

        function btn(label, className, attrs) {
            var node = el('button', className || 'btn btn-ghost admin-row-btn', label);
            node.type = 'button';
            var name;
            for (name in attrs || {}) {
                if (Object.prototype.hasOwnProperty.call(attrs, name)) node.setAttribute(name, attrs[name]);
            }
            return node;
        }

        function track(promise) {
            if (typeof options.track === 'function') options.track(promise);
            return promise;
        }

        // ---- context for the widgets ----

        var ctx = {
            doc: doc,
            langAttrs: Hints.langAttrs,
            act: function (node, fn, key) {
                seq += 1;
                acts[String(seq)] = fn;
                node.setAttribute('data-plan-act', String(seq));
                if (key) node.setAttribute('data-plan-focus-key', key);
            },
            onInput: function (node, fn) {
                seq += 1;
                inps[String(seq)] = fn;
                node.setAttribute('data-plan-inp', String(seq));
            },
            touch: function (node) {
                var cursor = node;
                while (cursor) {
                    if (cursor.getAttribute && cursor.hasAttribute && cursor.hasAttribute('data-plan-inp')) {
                        var fn = inps[cursor.getAttribute('data-plan-inp')];
                        if (fn) fn();
                    }
                    cursor = cursor.parentNode;
                }
            },
            uid: function (prefix) {
                seq += 1;
                return (prefix || 'plan') + '-' + seq;
            },
            focusKey: function (info, suffix) {
                return 'f:' + (info.path || '') + '#' + info.prop + ':' + suffix;
            },
            propsOf: function (schemaNode, def, only) {
                return tools ? tools.propsOf(schemaNode, def, only) : [];
            },
            requiredOf: function (schemaNode) {
                return tools ? tools.requiredOf(schemaNode) : [];
            },
            skeleton: function (schemaNode, def) {
                return skeleton ? skeleton(schemaNode, def) : null;
            },
            kindLabel: function (def) {
                var hint = def && Hints.kinds[def];
                return hint ? hint.label[0] + ' (' + hint.label[1] + ')' : (def || '');
            },
            summary: function (item) {
                if (!isObject(item)) return '';
                var i;
                for (i = 0; i < SUMMARY_PROPS.length; i++) {
                    var value = item[SUMMARY_PROPS[i]];
                    if (typeof value === 'string' && value.trim()) {
                        return value.length > 40 ? value.slice(0, 40) + '…' : value;
                    }
                }
                return typeof item.id === 'string' ? item.id : '';
            },
            childInfo: function (parent, entry, value) {
                return makeInfo(parent.refDef, entry, value, {
                    level: parent.level + 1,
                    path: parent.path,
                    only: null
                });
            },
            itemInfo: function (parent, value, extra) {
                var info = {
                    def: null,
                    prop: parent.prop,
                    key: null,
                    raw: parent.schema && parent.schema.items,
                    schema: parent.itemSchema,
                    refDef: parent.itemDef,
                    required: true,
                    value: value,
                    hint: {},
                    readOnly: false,
                    level: parent.level + 1,
                    path: parent.path,
                    nodeKind: null,
                    only: extra && extra.only ? extra.only : null
                };
                return info;
            },
            field: field,
            renderNode: renderNode,
            isOpen: function (path, dflt) {
                return Object.prototype.hasOwnProperty.call(openState, path) ? openState[path] : !!dflt;
            },
            toggleOpen: function (path, dflt) {
                openState[path] = !ctx.isOpen(path, dflt);
                if (typeof options.onToggle === 'function') options.onToggle(path);
            },
            confirm: function (message) {
                try {
                    var fn = options.confirm || root.confirm;
                    return typeof fn === 'function' ? !!fn(message) : true;
                } catch (err) {
                    return false;
                }
            },
            describeFailure: describeFailure,
            writeList: function (info, array, onFail) {
                var hooks = {};
                var p = Promise.resolve(transport.setField(info.path, info.prop, array, hooks));
                return track(p.then(function (out) {
                    if (out && out.ok) return true;
                    if (onFail) onFail(out);
                    else showFailure(info.path, info.prop, out);
                    return false;
                }));
            }
        };

        function makeInfo(ownerDef, entry, value, extra) {
            var resolved = tools.resolve(entry.raw);
            var schemaNode = resolved.schema || {};
            var key = ownerDef ? ownerDef + '.' + entry.prop : null;
            var hint = key ? Hints.fieldHint(key) : {};
            var info = {
                def: ownerDef,
                prop: entry.prop,
                key: key,
                raw: entry.raw,
                schema: schemaNode,
                refDef: resolved.def,
                required: !!entry.required,
                value: value,
                hint: hint,
                readOnly: !!(hint.readOnly || schemaNode.const !== undefined || entry.prop === 'id'
                    || (extra && extra.readOnly)),
                level: extra && extra.level !== undefined ? extra.level : 0,
                path: extra && extra.path !== undefined ? extra.path : '',
                nodeKind: extra && extra.nodeKind ? extra.nodeKind : null,
                only: extra && extra.only ? extra.only : null,
                bucket: false
            };
            if (has(schemaNode, 'array') && schemaNode.items) {
                var item = tools.resolve(schemaNode.items);
                info.itemSchema = item.schema;
                info.itemDef = item.def;
            }
            return info;
        }

        // ---- one field with its row ----

        function labelParts(info) {
            var hint = info.hint || {};
            return hint.label ? hint.label : [info.prop, ''];
        }

        function field(info, fopts) {
            var widget = Widgets.pick(info, ctx);
            var handle = widget.render(info, ctx);
            handle.widgetName = widget.name;
            handle.info = info;
            handle.el.setAttribute(info.level === 0 ? 'data-plan-field' : 'data-plan-sub', info.prop);
            handle.el.setAttribute('data-plan-widget', widget.name);
            if (fopts && fopts.bare) return handle;

            var group = ['string-list', 'object', 'object-list', 'bucket-list'].indexOf(widget.name) !== -1;
            var id = ctx.uid('plan-f');
            var row = el('div', 'plan-field');
            row.setAttribute('data-plan-row', info.prop);
            var parts = labelParts(info);
            var label = group ? el('span', 'plan-label') : el('label', 'plan-label');
            var he = el('span', 'plan-label-he', parts[0]);
            he.setAttribute('lang', 'he');
            label.appendChild(he);
            if (parts[1]) {
                var en = el('span', 'plan-label-en', parts[1]);
                en.setAttribute('lang', 'en');
                en.setAttribute('dir', 'ltr');
                label.appendChild(en);
            }
            var describedBy = [];
            if (group) {
                label.setAttribute('id', id + '-l');
                handle.el.setAttribute('aria-labelledby', id + '-l');
            } else {
                label.setAttribute('for', id);
                handle.el.setAttribute('id', id);
            }
            row.appendChild(label);
            row.appendChild(handle.el);
            handle.after.forEach(function (node) { row.appendChild(node); });
            if (info.hint && info.hint.help) {
                var help = el('p', 'plan-help');
                help.setAttribute('id', id + '-h');
                var hHe = el('span', 'plan-help-he', info.hint.help[0]);
                hHe.setAttribute('lang', 'he');
                var hEn = el('span', 'plan-help-en', info.hint.help[1]);
                hEn.setAttribute('lang', 'en');
                hEn.setAttribute('dir', 'ltr');
                help.appendChild(hHe);
                help.appendChild(hEn);
                row.appendChild(help);
                describedBy.push(id + '-h');
            }
            var issues = el('ul', 'plan-issues');
            issues.setAttribute('data-plan-issues', '');
            issues.setAttribute('id', id + '-i');
            row.appendChild(issues);
            describedBy.push(id + '-i');
            if (!group) handle.el.setAttribute('aria-describedby', describedBy.join(' '));
            handle.row = row;
            handle.issues = issues;
            return handle;
        }

        // ---- a node form ----

        function addIssue(list, finding, remainder) {
            var li = el('li', 'plan-issue plan-issue-' + (finding.severity || 'block'));
            li.setAttribute('data-plan-issue', finding.rule || '');
            li.setAttribute('data-severity', finding.severity || 'block');
            var text = (finding.severity || 'block') + ' · ' + (finding.rule || '') + ' — ' + (finding.message || '');
            if (remainder) text += ' (' + remainder + ')';
            li.textContent = text;
            list.appendChild(li);
        }

        function clearList(list) {
            while (list && list.firstChild) list.removeChild(list.firstChild);
        }

        function describeFailure(out) {
            if (!out) return 'request failed';
            var data = out.data || {};
            if (out.status === 409 || data.error === 'revision_conflict') {
                return 'revision_conflict ' + (data.revision !== undefined ? data.revision : '');
            }
            if (data.error === 'validation') return 'validation';
            var text = data.code || data.error || out.error || 'request failed';
            if (data.field) text += ' ' + data.field;
            if (data.reason) text += ' ' + data.reason;
            return String(text);
        }

        /** A failure that is not a validation list, shown at the field it came from. */
        function showFailure(path, prop, out) {
            var form = forms[path];
            var handle = form && form.fields[prop];
            var list = handle ? handle.issues : (form ? form.issues : null);
            if (!list) return;
            clearList(list);
            addIssue(list, { severity: 'block', rule: out && out.status === 409 ? 'revision_conflict' : 'refused', message: describeFailure(out) });
        }

        function keyOf(path, prop) {
            return path + '#' + prop;
        }

        function refreshDirty(handle) {
            var row = handle.row;
            if (!row) return;
            if (handle.isDirty()) row.setAttribute('data-plan-dirty', 'true');
            else row.removeAttribute('data-plan-dirty');
        }

        function saveHandle(handle) {
            var form = handle.form;
            var problem = handle.check();
            if (problem) {
                clearList(handle.issues);
                addIssue(handle.issues, { severity: 'block', rule: 'json', message: problem });
                return Promise.resolve(false);
            }
            if (!handle.isDirty()) {
                clearList(handle.issues);
                addIssue(handle.issues, { severity: 'inform', rule: 'unchanged', message: 'אין שינויים לשמירה (Nothing to save)' });
                return Promise.resolve(true);
            }
            clearList(handle.issues);
            var value = handle.getValue();
            var hooks = {
                onSaved: function () {
                    handle.markSaved();
                    delete drafts[keyOf(form.path, handle.prop)];
                }
            };
            var pending = value === undefined
                ? transport.putPart(form.path, without(form.node, handle.prop), hooks)
                : transport.setField(form.path, handle.prop, value, hooks);
            return track(Promise.resolve(pending).then(function (out) {
                if (out && out.ok) return true;
                afterFailure(form.path, handle.prop, out);
                return false;
            }));
        }

        /** Every finding of a 400 validation body, each with its severity. */
        function flatten(data) {
            var out = [];
            [['block', data.blocks], ['warn', data.warnings], ['inform', data.informs]].forEach(function (group) {
                (Array.isArray(group[1]) ? group[1] : []).forEach(function (finding) {
                    if (!isObject(finding)) return;
                    var copy = {};
                    Object.keys(finding).forEach(function (key) { copy[key] = finding[key]; });
                    copy.severity = copy.severity || group[0];
                    out.push(copy);
                });
            });
            return out;
        }

        /**
         * A failed write. A validation list goes inline at each finding's field (the ones
         * with no form on screen are listed where the save was made); anything else is
         * one line at the field. The typed text is never touched.
         */
        function afterFailure(path, prop, out) {
            var data = out && out.data;
            if (data && data.error === 'validation') {
                var findings = flatten(data);
                var plan = typeof options.getPlan === 'function' ? options.getPlan() : null;
                setIssues(findings, plan);
                var form = forms[path];
                var handle = form && form.fields[prop];
                var list = handle ? handle.issues : (form ? form.issues : null);
                // A finding about this node that no field of it shows is listed where the save was
                // made; findings about other parts stay in the validation panel.
                if (list) {
                    unplaced.forEach(function (item) {
                        if (item.target.path === path) addIssue(list, item.finding, item.target.field || item.finding.path);
                    });
                }
                if (typeof options.onValidation === 'function') options.onValidation(data);
                return;
            }
            showFailure(path, prop, out);
        }

        function rawJsonBox(form) {
            var box = el('div', 'plan-json-box');
            box.setAttribute('data-plan-json-box', '');
            var area = el('textarea', 'plan-json');
            area.rows = 12;
            area.setAttribute('dir', 'ltr');
            area.setAttribute('lang', 'en');
            area.setAttribute('spellcheck', 'false');
            area.setAttribute('data-plan-json', 'node');
            area.setAttribute('aria-label', 'JSON גולמי של הצומת (Raw JSON of this node)');
            area.value = JSON.stringify(form.node, null, 2);
            var error = el('p', 'admin-inline-error');
            error.setAttribute('role', 'alert');
            error.setAttribute('data-plan-json-error', '');
            error.hidden = true;
            var save = btn('שמירת JSON (Save JSON)', 'btn btn-primary', { 'data-plan-save-json': '' });
            ctx.act(save, function () {
                var parsed;
                try {
                    parsed = JSON.parse(String(area.value));
                } catch (err) {
                    error.hidden = false;
                    error.textContent = 'JSON לא תקין (Invalid JSON): ' + (err && err.message ? err.message : '');
                    return Promise.resolve(false);
                }
                if (!isObject(parsed)) {
                    error.hidden = false;
                    error.textContent = 'JSON לא תקין (Invalid JSON): an object is needed';
                    return Promise.resolve(false);
                }
                error.hidden = true;
                error.textContent = '';
                return track(Promise.resolve(transport.putPart(form.path, parsed, {})).then(function (out) {
                    if (out && out.ok) return true;
                    error.hidden = false;
                    error.textContent = describeFailure(out);
                    afterFailure(form.path, '', out);
                    return false;
                }));
            }, 'json-save:' + form.path);
            box.appendChild(area);
            box.appendChild(error);
            box.appendChild(save);
            return box;
        }

        function isReadOnlyProp(kind, prop, schemaNode) {
            if (prop === 'id') return true;
            if (kind === 'block' && prop === 'type') return true;
            if (kind === 'plan' && PLAN_READ_ONLY.indexOf(prop) !== -1) return true;
            return !!(schemaNode && schemaNode.const !== undefined);
        }

        /**
         * @param {{ path: string, node: object, def?: string, kind?: string, plan?: object,
         *           owner?: object }} spec
         * @returns {object} the node form
         */
        function renderNode(spec) {
            var path = spec.path || '';
            var kind = spec.kind || kindOfPath(path) || 'block';
            var node = isObject(spec.node) ? spec.node : {};
            var def = spec.def || null;
            if (!def && tools) {
                if (kind === 'plan') def = 'plan';
                else if (kind === 'unit') def = 'unit';
                else if (kind === 'lesson') def = 'lesson';
                else if (kind === 'block') def = tools.blockDef(node.type);
                else if (spec.plan) {
                    var segs = path.split('/').filter(Boolean);
                    var parent = nodeAt(spec.plan, segs.slice(0, 6).join('/'));
                    var parentDef = parent ? tools.blockDef(parent.type) : null;
                    def = parentDef ? tools.itemDef(parentDef, segs[6]) : null;
                }
            }
            var form = {
                path: path,
                kind: kind,
                def: def,
                node: node,
                fields: {},
                handles: [],
                el: el('div', 'plan-form'),
                issues: el('ul', 'plan-issues')
            };
            form.el.setAttribute('data-plan-form', '');
            form.el.setAttribute('data-plan-node', path);
            form.el.setAttribute('data-plan-kind', kind);
            if (def) form.el.setAttribute('data-plan-def', def);
            form.issues.setAttribute('data-plan-issues', '');
            form.issues.setAttribute('data-plan-node-issues', '');
            form.el.appendChild(form.issues);
            forms[path] = form;

            var nodeSchema = tools && def ? tools.defSchema(def) : null;
            var known = !!nodeSchema && isObject(nodeSchema.properties);
            if (!known) {
                var note = el('p', 'plan-note', 'אין טופס מוכן לצומת הזה — עריכה כ-JSON. (No ready form for this node; edit it as JSON.)');
                note.setAttribute('data-plan-unknown', '');
                form.el.appendChild(note);
            } else {
                var entries = tools.propsOf(nodeSchema, def).filter(function (entry) {
                    return CHILD_PROPS[kind] !== entry.prop;
                });
                entries.forEach(function (entry, index) {
                    var hint = Hints.fieldHint(def + '.' + entry.prop);
                    entry.order = typeof hint.order === 'number' ? hint.order : index * 10;
                    entry.advanced = !!hint.advanced;
                });
                entries.sort(function (a, b) {
                    if (a.advanced !== b.advanced) return a.advanced ? 1 : -1;
                    return a.order - b.order;
                });
                var advancedAt = -1;
                entries.forEach(function (entry, index) {
                    if (entry.advanced && advancedAt === -1) advancedAt = index;
                });
                entries.forEach(function (entry, index) {
                    if (index === advancedAt) {
                        var heading = el('p', 'plan-advanced', 'מתקדם (Advanced)');
                        heading.setAttribute('data-plan-advanced', '');
                        form.el.appendChild(heading);
                    }
                    renderEntry(form, entry, nodeSchema);
                });
            }

            var toggle = btn('JSON גולמי (Raw JSON)', 'btn btn-ghost admin-row-btn', { 'data-plan-json-toggle': '' });
            toggle.setAttribute('aria-expanded', 'false');
            var jsonHost = el('div', 'plan-json-host');
            var opened = null;
            function openJson() {
                if (!opened) {
                    opened = rawJsonBox(form);
                    jsonHost.appendChild(opened);
                }
                opened.hidden = false;
                toggle.setAttribute('aria-expanded', 'true');
            }
            ctx.act(toggle, function () {
                if (opened && !opened.hidden) {
                    opened.hidden = true;
                    toggle.setAttribute('aria-expanded', 'false');
                } else {
                    openJson();
                }
            }, 'json-toggle:' + path);
            form.el.appendChild(toggle);
            form.el.appendChild(jsonHost);
            if (!known) openJson();
            return form;
        }

        function renderEntry(form, entry, nodeSchema) {
            var path = form.path;
            var kind = form.kind;
            var value = form.node[entry.prop];
            var resolved = tools.resolve(entry.raw);
            var info = makeInfo(form.def, entry, value, {
                level: 0,
                path: path,
                nodeKind: kind,
                readOnly: isReadOnlyProp(kind, entry.prop, resolved.schema)
            });
            if (kind === 'block' && BUCKETS.indexOf(entry.prop) !== -1 && info.itemSchema
                && isObject(info.itemSchema.properties) && info.itemSchema.properties.id) {
                info.bucket = true;
            }
            var key = keyOf(path, entry.prop);
            if (Object.prototype.hasOwnProperty.call(drafts, key) && !info.readOnly) {
                info.hasLoaded = true;
                info.loaded = clone(value);
                info.value = clone(drafts[key]);
            }
            var handle = field(info);
            handle.form = form;
            handle.path = path;
            handle.prop = entry.prop;
            form.fields[entry.prop] = handle;
            form.handles.push(handle);
            form.el.appendChild(handle.row);
            applyMark(handle, path, entry.prop);
            if (handle.widgetName === 'bucket-list' || handle.widgetName === 'readonly' || info.readOnly) return;

            handles.push(handle);
            var index = handles.length - 1;
            handle.control.setAttribute('data-plan-focus-key', 'field:' + key);
            // The number of this field's Save button (data-plan-save-field), kept for the hooks tests read.
            handle.el.setAttribute('data-plan-field-index', String(index));
            var save = btn('שמירת שדה (Save field)', 'btn btn-primary', {
                'data-plan-save': '',
                'data-plan-save-field': String(index)
            });
            ctx.act(save, function () { return saveHandle(handle); }, 'save:' + key);
            var bar = el('div', 'plan-field-actions');
            bar.appendChild(save);
            if (!entry.required && value !== undefined) {
                var clear = btn('הסרת שדה (Remove field)', 'btn btn-ghost admin-row-btn', { 'data-plan-clear': '' });
                ctx.act(clear, function () {
                    if (!ctx.confirm('להסיר את השדה? (Remove this field?)')) return undefined;
                    var hooks = {
                        onSaved: function () { delete drafts[key]; }
                    };
                    return track(Promise.resolve(transport.putPart(path, without(form.node, entry.prop), hooks))
                        .then(function (out) {
                            if (out && out.ok) return true;
                            afterFailure(path, entry.prop, out);
                            return false;
                        }));
                }, 'clear:' + key);
                bar.appendChild(clear);
            }
            handle.row.appendChild(bar);
            ctx.onInput(handle.row, function () { refreshDirty(handle); });
            refreshDirty(handle);
        }

        /**
         * A form for a value that is not in the plan yet (a new block's required fields).
         * It is a nested object widget over `only`; its value is read with getValue().
         * @param {string} def the definition, for example `listeningBlock`
         * @param {object} value the starting value (a skeleton)
         * @param {string[]} [only] the properties to show
         */
        function renderDraft(def, value, only) {
            var info = {
                def: null,
                prop: def,
                key: null,
                raw: null,
                schema: tools.defSchema(def),
                refDef: def,
                required: true,
                value: value,
                hint: {},
                readOnly: false,
                level: 1,
                path: '',
                nodeKind: null,
                only: only || null,
                bucket: false
            };
            return field(info, { bare: true });
        }

        // ---- "edited by a person": information at a field, never a lock ----

        var editEntries = {};
        var markOpen = {};
        var jumped = null;

        function removeMark(handle) {
            if (handle.markEl && handle.markEl.parentNode) handle.markEl.parentNode.removeChild(handle.markEl);
            handle.markEl = null;
        }

        /**
         * Put the "edited by a person" mark on a field whose last writer was a person. It opens to
         * the value that person replaced and a button that restores it (the host confirms and sends
         * the restore). The mark never disables, hides, or warns away an edit.
         */
        function applyMark(handle, path, prop) {
            removeMark(handle);
            var key = keyOf(path, prop);
            var entry = editEntries[key];
            if (!entry || !handle.row) return;
            var Ui = root.HebrewPlanUi;
            var box = el('div', 'plan-edited');
            box.setAttribute('data-plan-edited', '');
            var toggle = btn('', 'btn btn-ghost admin-row-btn plan-edited-toggle', { 'data-plan-edited-mark': '' });
            var icon = Ui && typeof Ui.icon === 'function' ? Ui.icon(doc, 'admin') : null;
            if (icon) toggle.appendChild(icon);
            toggle.appendChild(el('span', '', 'נערך על ידי אדם (Edited by a person)'));
            var detail = el('div', 'plan-edited-detail');
            detail.setAttribute('data-plan-edited-detail', '');
            detail.hidden = markOpen[key] !== true;
            toggle.setAttribute('aria-expanded', markOpen[key] === true ? 'true' : 'false');

            var when = typeof entry.at === 'string' ? (Ui ? Ui.formatTime(entry.at) : entry.at) : '';
            var who = typeof entry.by === 'string' && entry.by ? ' · ' + entry.by : '';
            var meta = el('p', 'plan-note');
            var metaHe = el('span', 'plan-help-he', 'נערך בגרסה ' + entry.revision);
            metaHe.setAttribute('lang', 'he');
            var metaWhen = el('bdi', '', (when ? ' · ' + when : '') + who);
            metaWhen.setAttribute('dir', 'ltr');
            metaHe.appendChild(metaWhen);
            var metaEn = el('span', 'plan-help-en', 'Changed in revision ' + entry.revision);
            metaEn.setAttribute('lang', 'en');
            metaEn.setAttribute('dir', 'ltr');
            meta.appendChild(metaHe);
            meta.appendChild(metaEn);
            detail.appendChild(meta);
            var caption = el('p', 'plan-label-he', 'הערך הקודם (The earlier value)');
            detail.appendChild(caption);
            var omitted = entry.previousValueOmitted === true;
            var prevText = omitted ? '' : (Ui ? Ui.valueText(entry.previousValue) : JSON.stringify(entry.previousValue));
            var prev = el('bdi', 'plan-prev-value', omitted
                ? 'הערך הקודם גדול ולא נשמר כאן. (The earlier value was too large to keep here.)'
                : (prevText === '' || entry.previousValue === null ? '—' : prevText));
            if (!omitted && prevText !== '') {
                var langAttrs = handle.info && handle.info.hint && handle.info.hint.lang
                    ? Hints.langAttrs(handle.info.hint.lang) : (Ui ? Ui.textAttrs(prevText) : {});
                if (langAttrs.dir) prev.setAttribute('dir', langAttrs.dir);
                if (langAttrs.lang) prev.setAttribute('lang', langAttrs.lang);
            }
            prev.setAttribute('data-plan-prev-value', '');
            detail.appendChild(prev);
            var restore = btn('שחזור הערך הקודם (Restore the earlier value)', 'btn btn-ghost admin-row-btn',
                { 'data-plan-restore': 'value' });
            ctx.act(restore, function () {
                if (typeof options.onRestoreValue === 'function') {
                    options.onRestoreValue({ path: path, field: prop, fromRevision: entry.revision - 1, anchor: detail });
                }
            }, 'mark-restore:' + key);
            detail.appendChild(restore);

            ctx.act(toggle, function () {
                markOpen[key] = markOpen[key] !== true;
                detail.hidden = markOpen[key] !== true;
                toggle.setAttribute('aria-expanded', markOpen[key] === true ? 'true' : 'false');
            }, 'mark:' + key);
            box.appendChild(toggle);
            box.appendChild(detail);
            handle.row.appendChild(box);
            handle.markEl = box;
        }

        /**
         * The fields a person last wrote, from `editState.humanEdited` of GET .../parts. A field on
         * screen gets its mark now; a form drawn later gets it when it is drawn.
         * @param {{ path: string, field: string, revision: number, at?: string, by?: string,
         *   previousValue?: *, previousValueOmitted?: boolean }[]} list
         */
        function setEditState(list) {
            editEntries = {};
            (Array.isArray(list) ? list : []).forEach(function (entry) {
                if (!isObject(entry) || typeof entry.path !== 'string' || typeof entry.field !== 'string' || !entry.field
                    || !Number.isInteger(entry.revision)) return;
                editEntries[keyOf(entry.path, firstSegment(entry.field))] = entry;
            });
            Object.keys(forms).forEach(function (path) {
                var form = forms[path];
                Object.keys(form.fields).forEach(function (prop) { applyMark(form.fields[prop], path, prop); });
            });
        }

        /** Open an item's form (a word, a line) so a field in it can be shown. */
        function open(path) {
            openState[path] = true;
        }

        /**
         * Scroll a field into view, focus it, and flag it for a moment. False when no form for
         * that node is on the page (the host opens the section first).
         */
        function focusField(path, field) {
            var form = forms[path];
            if (!form) return false;
            var prop = firstSegment(field);
            var handle = prop ? form.fields[prop] : null;
            var target = handle && handle.row ? handle.row : form.el;
            if (jumped && jumped.removeAttribute) jumped.removeAttribute('data-plan-jump');
            target.setAttribute('data-plan-jump', 'true');
            jumped = target;
            if (typeof target.scrollIntoView === 'function') target.scrollIntoView({ block: 'center' });
            var control = handle ? handle.control : null;
            if (control && typeof control.focus === 'function') control.focus();
            return true;
        }

        // ---- drafts, issues, focus ----

        function captureDrafts() {
            handles.forEach(function (handle) {
                var key = keyOf(handle.path, handle.prop);
                if (handle.isDirty()) drafts[key] = clone(handle.getValue());
                else delete drafts[key];
            });
            return drafts;
        }

        function reset() {
            acts = {};
            inps = {};
            handles = [];
            forms = {};
        }

        function hasUnsaved() {
            var live = handles.some(function (handle) { return handle.isDirty(); });
            return live || Object.keys(drafts).length > 0;
        }

        function discardDrafts() {
            drafts = {};
        }

        /**
         * Show findings inline at their field. A finding is `{ rule, severity, message, path }`
         * with an RFC 6901 `path`, or `{ ..., path, field, pointer }` already split (the issues
         * route). Returns how many found a form on screen.
         */
        function setIssues(findings, plan) {
            Object.keys(forms).forEach(function (path) {
                var form = forms[path];
                clearList(form.issues);
                Object.keys(form.fields).forEach(function (prop) {
                    var handle = form.fields[prop];
                    clearList(handle.issues);
                    if (handle.control && handle.control.removeAttribute) handle.control.removeAttribute('aria-invalid');
                });
            });
            var placed = 0;
            unplaced = [];
            (findings || []).forEach(function (finding) {
                if (!finding || typeof finding !== 'object') return;
                var target = finding.pointer !== undefined
                    ? { path: finding.path || '', field: finding.field || '' }
                    : splitPointer(plan, finding.path || '');
                var form = forms[target.path];
                if (!form) {
                    unplaced.push({ finding: finding, target: target });
                    return;
                }
                var prop = firstSegment(target.field);
                var handle = prop ? form.fields[prop] : null;
                var remainder = target.field.indexOf('/') === -1 ? '' : target.field;
                if (handle) {
                    addIssue(handle.issues, finding, remainder);
                    if (finding.severity === 'block' && handle.control && handle.control.setAttribute
                        && handle.group !== true) {
                        handle.control.setAttribute('aria-invalid', 'true');
                    }
                } else {
                    addIssue(form.issues, finding, target.field);
                }
                placed += 1;
            });
            return placed;
        }

        function focusKey() {
            var active = doc && doc.activeElement;
            while (active) {
                if (active.getAttribute && active.hasAttribute && active.hasAttribute('data-plan-focus-key')) {
                    return active.getAttribute('data-plan-focus-key');
                }
                active = active.parentNode;
            }
            return '';
        }

        function restoreFocus(key) {
            if (!key || !options.host) return false;
            var nodes = options.host.querySelectorAll('[data-plan-focus-key]');
            var i;
            for (i = 0; i < nodes.length; i++) {
                if (nodes[i].getAttribute('data-plan-focus-key') === key) {
                    if (typeof nodes[i].focus === 'function') nodes[i].focus();
                    return true;
                }
            }
            return false;
        }

        // ---- events ----

        function targetNode(ev) {
            var node = ev && ev.target;
            if (node && node.nodeType === 3) node = node.parentNode;
            return node;
        }

        function onClick(ev) {
            var cursor = targetNode(ev);
            while (cursor && cursor !== options.host) {
                if (cursor.getAttribute && cursor.hasAttribute && cursor.hasAttribute('data-plan-act')) {
                    var fn = acts[cursor.getAttribute('data-plan-act')];
                    if (fn) {
                        var result = fn(ev);
                        if (result && typeof result.then === 'function') track(result);
                    }
                    return;
                }
                cursor = cursor.parentNode;
            }
        }

        function onInputEvent(ev) {
            var cursor = targetNode(ev);
            while (cursor && cursor !== options.host) {
                if (cursor.getAttribute && cursor.hasAttribute && cursor.hasAttribute('data-plan-inp')) {
                    var fn = inps[cursor.getAttribute('data-plan-inp')];
                    if (fn) fn(ev);
                }
                cursor = cursor.parentNode;
            }
        }

        function listen(target, type, fn) {
            if (options.signal) target.addEventListener(type, fn, { signal: options.signal });
            else target.addEventListener(type, fn);
            attached.push({ target: target, type: type, fn: fn });
        }

        if (options.host) {
            listen(options.host, 'click', onClick);
            listen(options.host, 'input', onInputEvent);
            listen(options.host, 'change', onInputEvent);
        }

        function destroy() {
            attached.forEach(function (item) { item.target.removeEventListener(item.type, item.fn); });
            attached = [];
            reset();
            drafts = {};
        }

        function loadSchema(api) {
            if (tools) return Promise.resolve(true);
            if (!api || typeof api.get !== 'function') return Promise.resolve(false);
            return api.get('/api/plan-schema').then(function (res) {
                if (!res || !res.ok || !isObject(res.data) || !isObject(res.data.definitions)) return false;
                setTools(res.data);
                return true;
            }).catch(function () { return false; });
        }

        return {
            ctx: ctx,
            loadSchema: loadSchema,
            setSchema: setTools,
            hasSchema: function () { return !!tools; },
            tools: function () { return tools; },
            renderNode: renderNode,
            renderDraft: renderDraft,
            captureDrafts: captureDrafts,
            discardDrafts: discardDrafts,
            reset: reset,
            hasUnsaved: hasUnsaved,
            setIssues: setIssues,
            setEditState: setEditState,
            focusField: focusField,
            open: open,
            focusKey: focusKey,
            restoreFocus: restoreFocus,
            isOpen: ctx.isOpen,
            forms: function () { return forms; },
            handles: function () { return handles; },
            destroy: destroy
        };
    }

    return {
        createEditor: createEditor,
        createTools: createTools,
        setSchema: setSchema,
        newBlock: newBlock,
        blockTypeList: blockTypeList,
        enumOf: enumOf,
        findUncoveredSchemaNodes: findUncoveredSchemaNodes,
        splitPointer: splitPointer,
        kindOfPath: kindOfPath,
        nodeAt: nodeAt,
        langAttrs: Hints.langAttrs,
        BUCKETS: BUCKETS,
        CHILD_PROPS: CHILD_PROPS
    };
});
