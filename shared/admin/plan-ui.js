/**
 * Small helpers shared by the plan screens that SPEC-013 to SPEC-015 added (the plan list, the
 * assign control, the history panel, the issues panel): element builders, a Hebrew-over-English
 * label, an icon from the brand sprite, a plain message for a refused call, and a few value
 * formatters. Nothing here talks to the server and nothing builds HTML from server text: every
 * string from the server goes in through textContent.
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-013, INIT-008/SPEC-014, INIT-008/SPEC-015
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanUi = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
    'use strict';

    var SVG_NS = 'http://www.w3.org/2000/svg';
    var SPRITE = '/shared/brand/sprite.svg#icon-';
    var HEBREW = /[\u0590-\u05FF]/;

    /** Plain-words messages for the codes the plan routes return. Hebrew first, English second. */
    var MESSAGES = {
        revision_conflict: ['התוכנית השתנתה מאז שנטענה.', 'The plan changed after it was loaded.'],
        already_exists: ['כבר יש תוכנית עם המזהה הזה.', 'A plan with this id already exists.'],
        invalid_id: ['המזהה אינו תקין.', 'The id is not valid.'],
        invalid_recipe: ['המתכון אינו מוכר.', 'The recipe is not one of the known recipes.'],
        invalid_audience: ['הקהל חייב להיות משותף או אישי.', 'The audience must be shared or personal.'],
        assignee_refused: ['החשבון לא פעיל או לא קיים.', 'That account is not active, or does not exist.'],
        assign_required: [
            'קהל ומשויך משתנים רק בבקרת השיוך.',
            'Audience and assignee change only through the assignment control.'
        ],
        no_snapshot: ['עוד לא פורסמה גרסה.', 'Nothing has been published yet.'],
        not_found: ['לא נמצא.', 'Not found.'],
        absent_in_revision: ['החלק הזה לא היה קיים בגרסה ההיא.', 'That part did not exist in that revision.'],
        parent_missing: ['ההורה של החלק הזה כבר לא קיים.', 'The parent of that part no longer exists.'],
        revision_corrupt: ['הגרסה הזו פגומה ואי אפשר לקרוא אותה.', 'That revision is damaged and cannot be read.'],
        validation: ['התוכנית לא עברה בדיקה.', 'The plan did not pass validation.'],
        pending_media: ['יש תמונה שממתינה לאישור.', 'A picture is waiting for approval.'],
        id_mismatch: ['המזהה בערך לא תואם לנתיב.', 'The id in the value does not match the path.'],
        invalid_op: ['הבקשה אינה תקינה.', 'The request is not valid.'],
        bad_request: ['הבקשה אינה תקינה.', 'The request is not valid.']
    };

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    /**
     * @param {Document} doc
     * @param {string} tag
     * @param {string} [className]
     * @param {string} [text]
     * @returns {HTMLElement}
     */
    function make(doc, tag, className, text) {
        var node = doc.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
    }

    /** A button with a name and attributes. `type` is `button`, so it never submits a form. */
    function button(doc, label, className, attrs) {
        var node = make(doc, 'button', className || 'btn btn-ghost admin-row-btn', label);
        node.type = 'button';
        var name;
        for (name in attrs || {}) {
            if (Object.prototype.hasOwnProperty.call(attrs, name)) node.setAttribute(name, attrs[name]);
        }
        return node;
    }

    /** "Hebrew (English)" as one string, for a control's text. */
    function label(he, en) {
        return he + ' (' + en + ')';
    }

    /**
     * Hebrew over English as two blocks, the way the editor labels a field.
     * @returns {HTMLElement} the wrapper
     */
    function bilingual(doc, tag, className, he, en) {
        var wrap = make(doc, tag, className);
        var h = make(doc, 'span', 'plan-label-he', he);
        h.setAttribute('lang', 'he');
        var e = make(doc, 'span', 'plan-label-en', en);
        e.setAttribute('lang', 'en');
        e.setAttribute('dir', 'ltr');
        wrap.appendChild(h);
        wrap.appendChild(e);
        return wrap;
    }

    /**
     * An icon from the brand sprite, or null where the document cannot make SVG (a test DOM).
     * It is decorative: the control that holds it carries the name.
     * @param {Document} doc
     * @param {string} name for example `admin`
     * @returns {Element|null}
     */
    function icon(doc, name) {
        if (!doc || typeof doc.createElementNS !== 'function') return null;
        var svg = doc.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('class', 'icon');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('width', '16');
        svg.setAttribute('height', '16');
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('focusable', 'false');
        var use = doc.createElementNS(SVG_NS, 'use');
        use.setAttribute('href', SPRITE + name);
        svg.appendChild(use);
        return svg;
    }

    /** `dir` and `lang` for a value, by whether it holds a Hebrew letter. */
    function textAttrs(text) {
        return HEBREW.test(String(text == null ? '' : text))
            ? { dir: 'rtl', lang: 'he' }
            : { dir: 'ltr', lang: 'en' };
    }

    function applyTextAttrs(node, text) {
        var attrs = textAttrs(text);
        node.setAttribute('dir', attrs.dir);
        node.setAttribute('lang', attrs.lang);
        return node;
    }

    /** A value as text for a before or after box: a string as it is, anything else as JSON. */
    function valueText(value) {
        if (value === undefined) return '';
        if (typeof value === 'string') return value;
        try {
            return JSON.stringify(value, null, 2);
        } catch (err) {
            return String(value);
        }
    }

    /** `2026-10-12T21:30:05.123Z` as `2026-10-12 21:30`. A value that is not a date is returned as it is. */
    function formatTime(iso) {
        var text = String(iso == null ? '' : iso);
        var m = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})/.exec(text);
        return m ? m[1] + ' ' + m[2] : text;
    }

    /**
     * What a refused call came back with, from the answer of HebrewApi.request.
     * @param {object|null} res
     * @returns {{ code: string, status: number, revision: number|null, field: string, reason: string,
     *   rules: { rule: string, severity: string, message: string }[] }}
     */
    function failure(res) {
        var data = res && isObject(res.data) ? res.data : {};
        var code = '';
        if (typeof data.code === 'string' && data.code) code = data.code;
        else if (typeof data.error === 'string' && data.error && data.error !== 'Invalid request' && data.error !== 'Not found') code = data.error;
        else if (res && typeof res.code === 'string') code = res.code;
        var rules = [];
        ['blocks', 'warnings', 'informs'].forEach(function (key, index) {
            (Array.isArray(data[key]) ? data[key] : []).forEach(function (finding) {
                if (!isObject(finding) || typeof finding.rule !== 'string') return;
                rules.push({
                    rule: finding.rule,
                    severity: ['block', 'warn', 'inform'][index],
                    path: typeof finding.path === 'string' ? finding.path : '',
                    message: typeof finding.message === 'string' ? finding.message : ''
                });
            });
        });
        return {
            code: code,
            status: res && typeof res.status === 'number' ? res.status : 0,
            revision: Number.isInteger(data.revision) ? data.revision : null,
            field: typeof data.field === 'string' ? data.field : '',
            reason: typeof data.reason === 'string' ? data.reason : '',
            rules: rules
        };
    }

    /**
     * One plain line for a refused call: the code, then the words. A conflict names the stored
     * revision, a validation refusal lists the rule codes, and `reason` and `field` are kept.
     * With `detail`, a validation refusal also gives the first blocking findings' path and message,
     * for a place with no field to show them at (the assign control).
     * @param {object|null} res
     * @param {{ detail?: boolean }} [options]
     * @returns {string}
     */
    function failureText(res, options) {
        var f = failure(res);
        var code = f.code || (res && res.status === 0 ? 'network' : 'request_failed');
        var words = MESSAGES[code];
        var text = code;
        if (f.revision !== null && (code === 'revision_conflict' || code === 'validation')) text += ' ' + f.revision;
        if (f.field) text += ' ' + f.field;
        if (f.reason) text += ' ' + f.reason;
        if (code === 'validation' && f.rules.length) {
            text += ': ' + f.rules.filter(function (r) { return r.severity === 'block'; }).map(function (r) { return r.rule; }).join(', ');
        }
        if (words) text += ' — ' + label(words[0], words[1]);
        else if (code === 'network') text += ' — ' + label('אין חיבור לשרת.', 'The server could not be reached.');
        else if (code === 'request_failed') text += ' — ' + label('הבקשה נכשלה.', 'The request failed.');
        if (code === 'validation' && options && options.detail) {
            var detail = f.rules.filter(function (r) { return r.severity === 'block'; }).slice(0, 2).map(function (r) {
                return (r.path || 'plan') + ': ' + r.message;
            });
            if (detail.length) text += ' (' + detail.join('; ') + ')';
        }
        return text;
    }

    /** The message pair for a code, or null. */
    function messageFor(code) {
        return Object.prototype.hasOwnProperty.call(MESSAGES, code) ? MESSAGES[code] : null;
    }

    function clear(node) {
        while (node && node.firstChild) node.removeChild(node.firstChild);
    }

    /** The nearest ancestor (or the node itself) that has `attr`, stopping at `stop`. */
    function closestAttr(node, attr, stop) {
        var cursor = node && node.nodeType === 3 ? node.parentNode : node;
        while (cursor && cursor !== stop) {
            if (cursor.hasAttribute && cursor.hasAttribute(attr)) return cursor;
            cursor = cursor.parentNode;
        }
        return null;
    }

    /**
     * Add a listener that goes away with the signal, and report a remover for a host with no
     * AbortSignal (the older test DOMs).
     * @returns {function(): void}
     */
    function listen(target, type, fn, signal) {
        if (signal) target.addEventListener(type, fn, { signal: signal });
        else target.addEventListener(type, fn);
        return function () { target.removeEventListener(type, fn); };
    }

    return {
        MESSAGES: MESSAGES,
        applyTextAttrs: applyTextAttrs,
        bilingual: bilingual,
        button: button,
        clear: clear,
        closestAttr: closestAttr,
        failure: failure,
        failureText: failureText,
        formatTime: formatTime,
        icon: icon,
        isObject: isObject,
        label: label,
        listen: listen,
        make: make,
        messageFor: messageFor,
        textAttrs: textAttrs,
        valueText: valueText
    };
});
