/**
 * A small fake DOM for the plan editor tests (INIT-008/SPEC-011, SPEC-012). It has only what
 * the editor uses: createElement, attributes, children, textContent, events that bubble
 * through parentNode, and querySelector(All) for `tag`, `[attr]`, `[attr="v"]` and
 * descendant combinations of those. A browser run is test/chrome-plan-editor-smoke.js.
 */
'use strict';

function parseSimple(part) {
    const tag = /^[a-zA-Z][a-zA-Z0-9]*/.exec(part);
    const attrs = [];
    const re = /\[([a-zA-Z0-9_:-]+)(?:=(?:"([^"]*)"|'([^']*)'|([^\]]*)))?\]/g;
    let m;
    while ((m = re.exec(part))) {
        attrs.push({ name: m[1], value: m[2] !== undefined ? m[2] : (m[3] !== undefined ? m[3] : m[4]) });
    }
    return { tag: tag ? tag[0].toUpperCase() : null, attrs };
}

function matchSimple(node, simple) {
    if (simple.tag && node.tagName !== simple.tag) return false;
    return simple.attrs.every((a) => (a.value === undefined
        ? node.hasAttribute(a.name)
        : node.getAttribute(a.name) === a.value));
}

function matches(node, selector) {
    const parts = selector.trim().split(/\s+/).map(parseSimple);
    if (!matchSimple(node, parts[parts.length - 1])) return false;
    let at = parts.length - 2;
    let cursor = node.parentNode;
    while (at >= 0 && cursor) {
        if (cursor.tagName && matchSimple(cursor, parts[at])) at -= 1;
        cursor = cursor.parentNode;
    }
    return at < 0;
}

function walk(node, fn) {
    node.children.forEach((child) => {
        fn(child);
        walk(child, fn);
    });
}

function createElement(tag) {
    const node = {
        tagName: String(tag).toUpperCase(),
        nodeType: 1,
        children: [],
        attrs: {},
        listeners: {},
        parentNode: null,
        className: '',
        value: '',
        disabled: false,
        checked: false,
        rows: 0,
        type: '',
        _text: '',
        _hidden: false,
        setAttribute(name, value) { this.attrs[name] = String(value); },
        getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
        hasAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name); },
        removeAttribute(name) { delete this.attrs[name]; },
        appendChild(child) {
            if (child.parentNode) child.parentNode.removeChild(child);
            child.parentNode = this;
            this.children.push(child);
            return child;
        },
        removeChild(child) {
            const i = this.children.indexOf(child);
            if (i >= 0) this.children.splice(i, 1);
            child.parentNode = null;
            return child;
        },
        addEventListener(type, fn) {
            (this.listeners[type] = this.listeners[type] || []).push(fn);
        },
        removeEventListener(type, fn) {
            this.listeners[type] = (this.listeners[type] || []).filter((f) => f !== fn);
        },
        querySelectorAll(sel) {
            const out = [];
            walk(this, (child) => { if (matches(child, sel)) out.push(child); });
            return out;
        },
        querySelector(sel) { return this.querySelectorAll(sel)[0] || null; },
        focus() { createElement.active = this; },
        dispatch(type) {
            const ev = { type, target: this, preventDefault() {} };
            for (let cur = this; cur; cur = cur.parentNode) {
                (cur.listeners[type] || []).slice().forEach((fn) => { ev.currentTarget = cur; fn(ev); });
            }
        },
        click() { this.dispatch('click'); }
    };
    Object.defineProperty(node, 'firstChild', { get() { return node.children[0] || null; } });
    Object.defineProperty(node, 'hidden', {
        get() { return node._hidden; },
        set(v) { node._hidden = !!v; if (v) node.attrs.hidden = ''; else delete node.attrs.hidden; }
    });
    Object.defineProperty(node, 'id', {
        get() { return node.getAttribute('id') || ''; },
        set(v) { node.setAttribute('id', v); }
    });
    Object.defineProperty(node, 'textContent', {
        get() { return node._text + node.children.map((c) => c.textContent).join(''); },
        set(v) { node.children.forEach((c) => { c.parentNode = null; }); node.children = []; node._text = String(v); }
    });
    return node;
}

function makeDocument() {
    const doc = { createElement, readyState: 'complete' };
    Object.defineProperty(doc, 'activeElement', { get() { return createElement.active || null; } });
    return doc;
}

/** Every descendant (and the node itself) matching `fn`. */
function collect(node, fn) {
    const out = [];
    if (fn(node)) out.push(node);
    walk(node, (child) => { if (fn(child)) out.push(child); });
    return out;
}

module.exports = { createElement, makeDocument, collect, walk, matches };
