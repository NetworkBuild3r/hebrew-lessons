/**
 * The live preview of the plan view (INIT-008/SPEC-015): the selected block (or the lesson, when no
 * block is selected) drawn the way a learner sees it, with the text the admin has typed and not yet
 * saved. It is the learner's own renderer (shared/ui/plan-render.js) given a copy of the saved node
 * with the unsaved field values laid over it. No second renderer, no markup language of its own, no
 * request: every string reaches the page through the renderer's escape.
 *
 *   var out = HebrewPlanPreview.compute({ render, lesson, lessonPath, block, blockPath, handles });
 *   out.html    the learner markup (or null when the renderer could not draw it)
 *   out.state   'unsaved' while a field in scope differs from the stored value, else 'saved'
 *
 * `handles` are the editor's field handles (path, prop, isDirty, check, getValue). A field whose
 * value is invalid JSON is left out of the picture and still counts as unsaved.
 * Nothing in a preview plays audio, writes progress, or calls an image or audio service: the
 * learner's controls are drawn but never bound, and `guardClick` stops a link from leaving the page.
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-015
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanPreview = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
    'use strict';

    var DEBOUNCE_MS = 250;
    var BUCKETS = ['words', 'paragraphs', 'lines', 'questions'];

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    function clone(value) {
        return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
    }

    /** The node a handle's form is for: the root itself, or an item of one of its lists. */
    function locate(copy, nodePath, path) {
        if (path === nodePath) return copy;
        var rest = path.slice(nodePath.length + 1).split('/');
        if (rest.length !== 2 || BUCKETS.indexOf(rest[0]) === -1 || !Array.isArray(copy[rest[0]])) return null;
        var list = copy[rest[0]];
        var i;
        for (i = 0; i < list.length; i++) {
            if (isObject(list[i]) && list[i].id === rest[1]) return list[i];
        }
        return null;
    }

    /**
     * A copy of `node` with the unsaved field values laid over it.
     * @param {object} node the saved node
     * @param {string} nodePath its id path
     * @param {object[]} handles the editor's field handles
     * @param {boolean} deep also take the forms of its list items (a block), not only its own form
     * @returns {{ node: object, dirty: boolean }}
     */
    function overlay(node, nodePath, handles, deep) {
        var copy = clone(node);
        var dirty = false;
        (Array.isArray(handles) ? handles : []).forEach(function (handle) {
            if (!handle || typeof handle.isDirty !== 'function') return;
            var path = typeof handle.path === 'string' ? handle.path : '';
            var inScope = path === nodePath || (deep === true && path.indexOf(nodePath + '/') === 0);
            if (!inScope || !handle.isDirty()) return;
            dirty = true;
            if (typeof handle.check === 'function' && handle.check()) return;
            var target = locate(copy, nodePath, path);
            if (!target) return;
            var value = handle.getValue();
            if (value === undefined) delete target[handle.prop];
            else target[handle.prop] = clone(value);
        });
        return { node: copy, dirty: dirty };
    }

    /**
     * @param {{ render?: object, lesson?: object|null, lessonPath?: string, block?: object|null,
     *   blockPath?: string, handles?: object[] }} spec
     * @returns {{ html: string|null, state: 'saved'|'unsaved', kind: 'block'|'lesson'|'none' }}
     */
    function compute(spec) {
        var Render = spec && spec.render;
        var none = { html: '', state: 'saved', kind: 'none' };
        if (!Render) return none;
        try {
            if (isObject(spec.block) && typeof Render.renderBlock === 'function') {
                var b = overlay(spec.block, spec.blockPath || '', spec.handles, true);
                var html = Render.renderBlock(b.node, { lesson: spec.lesson || undefined });
                return { html: typeof html === 'string' ? html : '', state: b.dirty ? 'unsaved' : 'saved', kind: 'block' };
            }
            if (isObject(spec.lesson) && typeof Render.lessonCard === 'function') {
                var l = overlay(spec.lesson, spec.lessonPath || '', spec.handles, false);
                return {
                    html: '<ol class="lesson-list">' + Render.lessonCard(l.node, '#overview') + '</ol>',
                    state: l.dirty ? 'unsaved' : 'saved',
                    kind: 'lesson'
                };
            }
        } catch (err) {
            // A half-typed value the renderer cannot draw: the page keeps the last picture.
            return { html: null, state: 'unsaved', kind: 'none' };
        }
        return none;
    }

    /**
     * Stop a click on a link or a button inside the preview from doing anything. The learner's
     * controls are drawn but not wired; a link would otherwise change the admin page's hash.
     * @param {{ target?: Node, preventDefault?: function }} ev
     * @param {Node} stop the preview element
     * @returns {boolean} true when the click was on a link or button
     */
    function guardClick(ev, stop) {
        var node = ev && ev.target;
        if (node && node.nodeType === 3) node = node.parentNode;
        while (node && node !== stop) {
            var tag = String(node.tagName || '').toUpperCase();
            if (tag === 'A' || tag === 'BUTTON') {
                if (ev.preventDefault) ev.preventDefault();
                return true;
            }
            node = node.parentNode;
        }
        return false;
    }

    return {
        DEBOUNCE_MS: DEBOUNCE_MS,
        compute: compute,
        guardClick: guardClick,
        overlay: overlay
    };
});
