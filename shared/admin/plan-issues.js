/**
 * The issues panel of the plan view (INIT-008/SPEC-015). It shows what GET /api/plans/:id/issues
 * reports, grouped hard errors first (marked "blocks publish"), then warnings ("does not block
 * publish"), and notes when asked for. Each item links to the field in the editor and, where the
 * server gave a fix, offers it in one click. The panel recomputes no rule and no cap: it only
 * orders and shows what the server sent.
 *
 *   var panel = HebrewPlanIssues.create({ doc, api, planId, signal, getPlan, onJump, onFix, onLoaded });
 *   panel.el        the section to put in the page
 *   panel.load()    fetch and show the issues (Promise)
 *   panel.last()    the last answer { revision, status, counts, issues, truncated } or null
 *
 * `onJump(issue)` opens and focuses the field. `onFix(request, issue)` sends one set_field
 * through the editor's save path and resolves true (saved), 'refreshed' (saved and the host already
 * reloaded the issues) or false.
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-015
 */
(function (root, factory) {
    'use strict';

    if (typeof module !== 'undefined' && module.exports && typeof require === 'function') {
        require('./plan-ui.js');
        require('./plan-hints.js');
        require('./plan-widgets.js');
        require('./plan-form.js');
    }
    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanIssues = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var Ui = root.HebrewPlanUi;

    var GROUPS = [
        { severity: 'block', he: 'חוסמות פרסום', en: 'Blocks publish' },
        { severity: 'warn', he: 'אזהרות — אינן חוסמות פרסום', en: 'Warnings — does not block publish' },
        { severity: 'inform', he: 'הערות', en: 'Notes' }
    ];

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    function clone(value) {
        return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
    }

    // ---- pure helpers ---------------------------------------------------------------------------

    /**
     * The issues as groups by severity (hard errors first), each holding its codes in the order they
     * first appear. A severity with no issue has no group. The server already sorted the issues;
     * this keeps that order.
     * @param {object[]} issues
     * @returns {{ severity: string, hard: boolean, he: string, en: string,
     *   codes: { code: string, issues: object[] }[] }[]}
     */
    function groupIssues(issues) {
        var list = Array.isArray(issues) ? issues : [];
        return GROUPS.map(function (group) {
            var codes = [];
            var byCode = {};
            list.forEach(function (issue) {
                if (!isObject(issue) || issue.severity !== group.severity) return;
                var code = typeof issue.code === 'string' ? issue.code : '';
                if (!Object.prototype.hasOwnProperty.call(byCode, code)) {
                    byCode[code] = { code: code, issues: [] };
                    codes.push(byCode[code]);
                }
                byCode[code].issues.push(issue);
            });
            return { severity: group.severity, hard: group.severity === 'block', he: group.he, en: group.en, codes: codes };
        }).filter(function (group) { return group.codes.length > 0; });
    }

    function count(counts, key) {
        return counts && Number.isInteger(counts[key]) ? counts[key] : 0;
    }

    /**
     * The counts as one bilingual line.
     * @param {{ block?: number, warn?: number, inform?: number }|null} counts
     * @returns {{ he: string, en: string, text: string }}
     */
    function countsText(counts) {
        var b = count(counts, 'block');
        var w = count(counts, 'warn');
        var i = count(counts, 'inform');
        var he = b + ' שגיאות חוסמות · ' + w + ' אזהרות';
        var en = b + ' blocking · ' + w + ' warnings';
        if (i > 0) {
            he += ' · ' + i + ' הערות';
            en += ' · ' + i + ' notes';
        }
        return { he: he, en: en, text: he + ' (' + en + ')' };
    }

    /**
     * The line above the Publish button. It says warnings do not stop publishing, asks for no approval.
     * @param {{ block?: number, warn?: number }|null} counts
     * @returns {string}
     */
    function publishNote(counts) {
        var b = count(counts, 'block');
        var w = count(counts, 'warn');
        var he = 'לפני פרסום: ' + b + ' שגיאות חוסמות, ' + w + ' אזהרות. אזהרות אינן עוצרות פרסום.';
        var en = 'Before publishing: ' + b + ' blocking errors, ' + w + ' warnings. Warnings do not stop publishing.';
        if (b > 0) {
            he += ' שגיאה חוסמת מונעת פרסום.';
            en += ' A blocking error stops publishing.';
        }
        return he + ' (' + en + ')';
    }

    function unescapeSegment(segment) {
        return String(segment).replace(/~1/g, '/').replace(/~0/g, '~');
    }

    function fieldSegments(field) {
        return String(field == null ? '' : field).split('/').filter(function (s) { return s !== ''; }).map(unescapeSegment);
    }

    /** The node an issue is about, or null. */
    function nodeOf(plan, issue) {
        var Form = root.HebrewPlanForm;
        if (!Form || !isObject(plan) || !isObject(issue)) return null;
        return Form.nodeAt(plan, issue.path || '');
    }

    /**
     * The current value of the field an issue names, for the "current / suggested" view. A nested
     * field (`cloze/items/0/answer`) is followed down; a missing one is undefined.
     */
    function currentValue(plan, issue) {
        var node = nodeOf(plan, issue);
        var segs = fieldSegments(issue && issue.field);
        if (!node || !segs.length) return undefined;
        var value = node;
        var i;
        for (i = 0; i < segs.length; i++) {
            if (value === null || typeof value !== 'object' || !Object.prototype.hasOwnProperty.call(value, segs[i])) return undefined;
            value = value[segs[i]];
        }
        return value;
    }

    /**
     * The one set_field that applies an issue's fix. A fix on a top-level field is that field with the
     * suggested value. A fix inside a field (`newWords/1`, `cloze/items/0/answer`) is that one field,
     * whole, with the suggested value put at the place the finding names: set_field takes a field of a
     * node, so the whole field goes. Nothing else of the plan is touched or sent.
     * @param {object} issue an issue with `fix`
     * @param {object} plan the working copy the issue was reported against
     * @param {*} [choice] one of `fix.candidates`, for a pointing-consistency fix
     * @returns {{ path: string, field: string, value: * }|null} null when the fix cannot be built
     */
    function fixRequest(issue, plan, choice) {
        if (!isObject(issue) || !isObject(issue.fix)) return null;
        var fix = issue.fix;
        var value = choice !== undefined ? choice : fix.suggested;
        if (value === undefined) return null;
        var path = typeof fix.path === 'string' ? fix.path : issue.path;
        var segs = fieldSegments(typeof fix.field === 'string' ? fix.field : issue.field);
        if (!segs.length || typeof path !== 'string') return null;
        if (segs.length === 1) return { path: path, field: segs[0], value: value };
        var Form = root.HebrewPlanForm;
        var node = Form && isObject(plan) ? Form.nodeAt(plan, path) : null;
        if (!node || !Object.prototype.hasOwnProperty.call(node, segs[0])) return null;
        var whole = clone(node[segs[0]]);
        var cursor = whole;
        var i;
        for (i = 1; i < segs.length - 1; i++) {
            if (cursor === null || typeof cursor !== 'object' || !Object.prototype.hasOwnProperty.call(cursor, segs[i])) return null;
            cursor = cursor[segs[i]];
        }
        var last = segs[segs.length - 1];
        if (cursor === null || typeof cursor !== 'object' || !Object.prototype.hasOwnProperty.call(cursor, last)) return null;
        cursor[last] = value;
        return { path: path, field: segs[0], value: whole };
    }

    /**
     * The issues in the shape the editor's setIssues takes (`rule` for the code, and the id path
     * with its field already split, marked by `pointer`).
     * @param {object[]} issues
     * @returns {object[]}
     */
    function forEditor(issues) {
        return (Array.isArray(issues) ? issues : []).filter(isObject).map(function (issue) {
            return {
                rule: issue.code,
                severity: issue.severity,
                message: issue.message,
                path: issue.path || '',
                field: issue.field || '',
                pointer: typeof issue.pointer === 'string' ? issue.pointer : ''
            };
        });
    }

    // ---- the panel ------------------------------------------------------------------------------

    /**
     * @param {object} options see the file header
     */
    function create(options) {
        var opts = options || {};
        var doc = opts.doc || root.document;
        var Api = opts.api || root.HebrewApi;
        var signal = opts.signal || null;
        var make = function (tag, cls, text) { return Ui.make(doc, tag, cls, text); };
        var removers = [];
        var alive = true;
        var seq = 0;
        var result = null;
        var showInforms = false;
        var busy = false;

        var el = make('section', 'card plan-issues-panel');
        el.setAttribute('data-plan-issues', 'panel');
        el.setAttribute('aria-label', 'בעיות בתוכנית (Plan issues)');
        var head = make('div', 'plan-panel-head');
        var heading = make('h3', '', 'בעיות (Issues)');
        head.appendChild(heading);
        var counter = make('p', 'plan-issues-count');
        counter.setAttribute('data-plan-issues-count', '');
        counter.setAttribute('role', 'status');
        head.appendChild(counter);
        el.appendChild(head);

        var tools = make('div', 'plan-issues-tools');
        var informLabel = make('label', 'plan-check');
        var informBox = make('input');
        informBox.type = 'checkbox';
        informBox.setAttribute('data-plan-issues-informs', '');
        informLabel.appendChild(informBox);
        informLabel.appendChild(make('span', '', 'הצגת הערות (Show notes)'));
        tools.appendChild(informLabel);
        tools.appendChild(Ui.button(doc, 'רענון (Refresh)', 'btn btn-ghost admin-row-btn', { 'data-plan-issues-refresh': '' }));
        el.appendChild(tools);

        var stateLine = make('p', 'plan-note');
        stateLine.setAttribute('data-plan-issues-state', '');
        stateLine.setAttribute('role', 'status');
        el.appendChild(stateLine);
        var fixMessage = make('p', 'admin-inline-error');
        fixMessage.setAttribute('data-plan-fix-message', '');
        fixMessage.setAttribute('role', 'alert');
        fixMessage.hidden = true;
        el.appendChild(fixMessage);
        var body = make('div', 'plan-issues-body');
        body.setAttribute('data-plan-issues-body', '');
        el.appendChild(body);

        function setFixMessage(text) {
            fixMessage.hidden = !text;
            fixMessage.textContent = text || '';
        }

        function valueBox(caption, value, attr) {
            var box = make('div', 'plan-fix-value');
            var cap = make('span', 'plan-fix-caption', caption);
            var text = Ui.valueText(value);
            var val = make('bdi', 'plan-fix-text', text === '' ? '—' : text);
            Ui.applyTextAttrs(val, text);
            val.setAttribute(attr, '');
            box.appendChild(cap);
            box.appendChild(val);
            return box;
        }

        function renderIssue(issue, index) {
            var li = make('li', 'plan-issue plan-issue-' + issue.severity);
            li.setAttribute('data-plan-issue', issue.code);
            li.setAttribute('data-plan-issue-severity', issue.severity);
            li.setAttribute('data-plan-issue-index', String(index));
            var message = make('p', 'plan-issue-message', typeof issue.message === 'string' ? issue.message : '');
            message.setAttribute('dir', 'auto');
            li.appendChild(message);

            var where = (issue.path || '') + (issue.field ? '#' + issue.field : '');
            var link = make('button', 'btn btn-ghost admin-row-btn plan-issue-link');
            link.type = 'button';
            link.setAttribute('data-plan-issue-link', issue.path || '');
            link.setAttribute('data-plan-issue-field', issue.field || '');
            var goLabel = make('span', 'plan-issue-go', 'מעבר לשדה (Go to the field) ');
            link.appendChild(goLabel);
            var place = make('bdi', 'plan-issue-place', where || 'plan');
            place.setAttribute('dir', 'ltr');
            place.setAttribute('lang', 'en');
            link.appendChild(place);
            li.appendChild(link);

            var fix = isObject(issue.fix) ? issue.fix : null;
            if (fix) {
                var box = make('div', 'plan-fix');
                box.setAttribute('data-plan-fix-box', '');
                if (fix.suggested !== undefined) {
                    var pair = make('div', 'plan-fix-pair');
                    pair.appendChild(valueBox('הערך הנוכחי (Current)', currentValue(opts.getPlan ? opts.getPlan() : null, issue), 'data-plan-fix-current'));
                    pair.appendChild(valueBox('הערך המוצע (Suggested)', fix.suggested, 'data-plan-fix-suggested'));
                    box.appendChild(pair);
                    box.appendChild(Ui.button(doc, 'שימוש בערך הזה (Use this value)', 'btn', { 'data-plan-fix': '' }));
                } else if (Array.isArray(fix.candidates) && fix.candidates.length) {
                    box.appendChild(make('p', 'plan-fix-caption', 'בחרו את הצורה הנכונה (Choose the right form)'));
                    var choices = make('div', 'plan-fix-choices');
                    fix.candidates.forEach(function (candidate, n) {
                        var c = Ui.button(doc, typeof candidate === 'string' ? candidate : Ui.valueText(candidate), 'btn',
                            { 'data-plan-fix-choice': String(n) });
                        Ui.applyTextAttrs(c, typeof candidate === 'string' ? candidate : '');
                        choices.appendChild(c);
                    });
                    box.appendChild(choices);
                }
                if (typeof fix.note === 'string' && fix.note) {
                    var note = make('p', 'plan-note plan-fix-note', fix.note);
                    note.setAttribute('data-plan-fix-note', '');
                    note.setAttribute('dir', 'ltr');
                    note.setAttribute('lang', 'en');
                    box.appendChild(note);
                }
                if (box.firstChild) li.appendChild(box);
            }
            return li;
        }

        function render() {
            Ui.clear(body);
            var data = result;
            if (!data) {
                counter.textContent = '';
                return;
            }
            counter.textContent = countsText(data.counts).text;
            var issues = Array.isArray(data.issues) ? data.issues : [];
            if (!issues.length) {
                var none = make('p', 'plan-note', 'אין בעיות. (No issues.)');
                none.setAttribute('data-plan-issues-empty', '');
                body.appendChild(none);
            }
            groupIssues(issues).forEach(function (group) {
                var section = make('div', 'plan-issue-group');
                section.setAttribute('data-plan-issue-group', group.severity);
                var title = make('h4', 'plan-issue-group-title', Ui.label(group.he, group.en));
                section.appendChild(title);
                group.codes.forEach(function (entry) {
                    var codeBox = make('div', 'plan-issue-code-group');
                    var code = make('p', 'plan-issue-code');
                    var name = make('bdi', '', entry.code);
                    name.setAttribute('dir', 'ltr');
                    name.setAttribute('lang', 'en');
                    code.appendChild(name);
                    code.appendChild(make('span', 'plan-issue-code-count', ' · ' + entry.issues.length));
                    codeBox.appendChild(code);
                    var ul = make('ul', 'plan-issue-list');
                    entry.issues.forEach(function (issue) {
                        ul.appendChild(renderIssue(issue, issues.indexOf(issue)));
                    });
                    codeBox.appendChild(ul);
                    section.appendChild(codeBox);
                });
                body.appendChild(section);
            });
            if (data.truncated === true) {
                var cut = make('p', 'plan-note', 'יש עוד בעיות ממה שמוצג. (There are more issues than are shown.)');
                cut.setAttribute('data-plan-issues-truncated', '');
                body.appendChild(cut);
            }
        }

        function url() {
            var base = '/api/plans/' + encodeURIComponent(opts.planId) + '/issues';
            return showInforms ? base + '?severity=all' : base;
        }

        function load() {
            seq += 1;
            var mine = seq;
            if (!result) stateLine.textContent = 'טוען בעיות… (Loading the issues…)';
            return Promise.resolve(Api.get(url(), signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive || mine !== seq) return null;
                if (!res || !res.ok || !isObject(res.data) || !Array.isArray(res.data.issues)) {
                    stateLine.textContent = 'לא ניתן לטעון את הבעיות — ' + Ui.failureText(res) + ' (Could not load the issues.)';
                    return null;
                }
                stateLine.textContent = '';
                result = res.data;
                render();
                if (typeof opts.onLoaded === 'function') opts.onLoaded(result);
                return result;
            }).catch(function (err) {
                if (!alive || mine !== seq) return null;
                stateLine.textContent = 'לא ניתן לטעון את הבעיות — ' + Ui.failureText({ status: 0, data: null, error: err && err.message })
                    + ' (Could not load the issues.)';
                return null;
            });
        }

        function apply(issue, choice) {
            if (busy || typeof opts.onFix !== 'function') return Promise.resolve(false);
            var request = fixRequest(issue, opts.getPlan ? opts.getPlan() : null, choice);
            setFixMessage('');
            if (!request) {
                setFixMessage('אי אפשר להחיל את התיקון הזה כאן. (This fix cannot be applied here.)');
                return Promise.resolve(false);
            }
            busy = true;
            return Promise.resolve(opts.onFix(request, issue)).then(function (outcome) {
                busy = false;
                if (!alive) return false;
                if (outcome === 'refreshed') return true;
                if (outcome && typeof outcome === 'object' && typeof outcome.error === 'string') {
                    setFixMessage('התיקון לא נשמר — ' + outcome.error + ' (The fix was not saved.)');
                    return false;
                }
                return outcome ? load().then(function () { return true; }) : false;
            }).catch(function () {
                busy = false;
                return false;
            });
        }

        function issueAt(node) {
            var li = Ui.closestAttr(node, 'data-plan-issue-index', el);
            if (!li || !result || !Array.isArray(result.issues)) return null;
            var index = Number(li.getAttribute('data-plan-issue-index'));
            return Number.isInteger(index) ? result.issues[index] || null : null;
        }

        function onClick(ev) {
            var node = ev && ev.target;
            if (node && node.nodeType === 3) node = node.parentNode;
            if (Ui.closestAttr(node, 'data-plan-issues-refresh', el)) {
                load();
                return;
            }
            var jump = Ui.closestAttr(node, 'data-plan-issue-link', el);
            if (jump) {
                var target = issueAt(jump);
                if (target && typeof opts.onJump === 'function') opts.onJump(target);
                return;
            }
            var choice = Ui.closestAttr(node, 'data-plan-fix-choice', el);
            if (choice) {
                var issue = issueAt(choice);
                var n = Number(choice.getAttribute('data-plan-fix-choice'));
                if (issue && isObject(issue.fix) && Array.isArray(issue.fix.candidates)) apply(issue, issue.fix.candidates[n]);
                return;
            }
            var fixButton = Ui.closestAttr(node, 'data-plan-fix', el);
            if (fixButton) {
                var which = issueAt(fixButton);
                if (which) apply(which);
            }
        }
        function onChange(ev) {
            var node = ev && ev.target;
            if (node && Ui.closestAttr(node, 'data-plan-issues-informs', el)) {
                showInforms = informBox.checked === true;
                load();
            }
        }
        removers.push(Ui.listen(el, 'click', onClick, signal));
        removers.push(Ui.listen(el, 'change', onChange, signal));

        return {
            el: el,
            load: load,
            last: function () { return result; },
            apply: apply,
            destroy: function () {
                alive = false;
                removers.forEach(function (remove) { remove(); });
                removers = [];
            }
        };
    }

    return {
        GROUPS: GROUPS,
        countsText: countsText,
        create: create,
        currentValue: currentValue,
        fixRequest: fixRequest,
        forEditor: forEditor,
        groupIssues: groupIssues,
        publishNote: publishNote
    };
});
