/**
 * The history panel of the plan view (INIT-008/SPEC-014): who changed the plan and when, a
 * field-level comparison of any two states, and a one-step restore. It is a client of four routes
 * and holds no diff logic and no restore logic:
 *   GET  /api/plans/:id/revisions?actorKind=&beforeRevision=      the list, newest first
 *   GET  /api/plans/:id/revisions/:n                              one revision
 *   GET  /api/plans/:id/diff?from=&to=&path=                      from and to: a number, working, published
 *   POST /api/plans/:id/restore { fromRevision, path?, field?, expectedRevision }
 * Nothing here locks or blocks a field: a mark that says "a person changed this" is information,
 * and a restore is a new revision, so history only ever grows. The confirmation before a restore
 * lives here, in the UI; the service restores in one call.
 *
 *   var history = HebrewPlanHistory.create({ doc, api, planId, signal, getRevision, hasUnsaved,
 *                                            onRestored, onReload });
 *   history.el, history.load(), history.open(), history.confirmAt(container, spec)
 *
 * `onRestored(data)` gets the 200 body of a restore and returns a promise; it resolves 'refreshed'
 * when the host has already reloaded the plan and its panels. Server text goes in through textContent.
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-014, INIT-011/SPEC-003
 */
(function (root, factory) {
    'use strict';

    if (typeof module !== 'undefined' && module.exports && typeof require === 'function') {
        require('./plan-ui.js');
    }
    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanHistory = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var Ui = root.HebrewPlanUi;

    var ACTOR_KINDS = ['human', 'ai', 'system'];
    var KIND_LABELS = {
        changed: ['שונה', 'Changed'],
        added: ['נוסף', 'Added'],
        removed: ['הוסר', 'Removed'],
        moved: ['הוזז', 'Moved']
    };
    var FILTERS = [
        ['', 'כל הכותבים', 'Everyone'],
        ['human', 'אדם', 'A person'],
        ['ai', 'בינה מלאכותית', 'AI'],
        ['system', 'מערכת', 'System']
    ];

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    // ---- pure helpers ---------------------------------------------------------------------------

    /**
     * How a revision row names its writer: text plus an icon name, never colour alone.
     * @param {object} row a revisions row
     * @returns {{ kind: string, he: string, en: string, detail: string, icon: string }}
     */
    function actorMark(row) {
        var kind = row && ACTOR_KINDS.indexOf(row.actorKind) !== -1 ? row.actorKind : 'system';
        var name = row && typeof row.actorName === 'string' ? row.actorName : '';
        var tool = row && typeof row.tool === 'string' ? row.tool : '';
        if (kind === 'human') return { kind: kind, he: 'אדם', en: 'A person', detail: name, icon: 'admin' };
        if (kind === 'ai') return { kind: kind, he: 'בינה מלאכותית', en: 'AI', detail: tool, icon: 'settings' };
        return { kind: kind, he: 'מערכת', en: 'System', detail: name || tool, icon: 'review' };
    }

    /**
     * Diff entries grouped by node path, keeping the server's order.
     * @param {object[]} entries
     * @returns {{ path: string, entries: object[] }[]}
     */
    function groupEntries(entries) {
        var out = [];
        var seen = {};
        (Array.isArray(entries) ? entries : []).forEach(function (entry) {
            if (!isObject(entry)) return;
            var path = typeof entry.path === 'string' ? entry.path : '';
            if (!Object.prototype.hasOwnProperty.call(seen, path)) {
                seen[path] = { path: path, entries: [] };
                out.push(seen[path]);
            }
            seen[path].entries.push(entry);
        });
        return out;
    }

    /**
     * "Revision 7", "the working copy", or "the published copy", from the `from` or `to` of a diff.
     * @param {{ ref?: string|number, revision?: number }|null} side
     * @returns {{ he: string, en: string, suffix: string }}
     */
    function describeRefParts(side) {
        var ref = side ? side.ref : '';
        var rev = side && Number.isInteger(side.revision) ? side.revision : null;
        if (ref === 'working') {
            return { he: 'העבודה', en: 'working copy', suffix: rev !== null ? ' · ' + rev : '' };
        }
        if (ref === 'published') {
            return { he: 'המפורסמת', en: 'published copy', suffix: rev !== null ? ' · ' + rev : '' };
        }
        var n = rev !== null ? rev : ref;
        return { he: 'גרסה ' + n, en: 'revision ' + n, suffix: '' };
    }

    /** One bilingual string for tests and older call sites. Prefer describeRefParts + fillPair in the UI. */
    function describeRef(side) {
        var p = describeRefParts(side);
        return p.he + ' (' + p.en + ')' + p.suffix;
    }

    /**
     * Which restores a diff entry offers. A restore takes a value from the earlier side, so there is
     * none when the earlier side is the working copy; an added node has no earlier value; a moved list
     * is restored through its part.
     * @param {object} entry
     * @param {{ ref?: string|number, revision?: number }|null} from
     * @returns {{ field: boolean, part: boolean, fromRevision: number|null }}
     */
    function restoreOffers(entry, from) {
        var none = { field: false, part: false, fromRevision: null };
        if (!isObject(entry) || !from || from.ref === 'working' || !Number.isInteger(from.revision)) return none;
        var path = typeof entry.path === 'string' ? entry.path : '';
        var changed = entry.kind === 'changed';
        return {
            field: changed && typeof entry.field === 'string' && entry.field !== '',
            part: path !== '' && (changed || entry.kind === 'removed'),
            fromRevision: from.revision
        };
    }

    /**
     * The body of one restore call. A whole-plan restore has no path and no field.
     * @param {{ fromRevision: number, path?: string, field?: string }} spec
     * @param {number} expectedRevision
     */
    function restoreBody(spec, expectedRevision) {
        var body = { fromRevision: spec.fromRevision, expectedRevision: expectedRevision };
        if (typeof spec.path === 'string') body.path = spec.path;
        if (typeof spec.field === 'string' && spec.field !== '') body.field = spec.field;
        return body;
    }

    /** What a restore will put back, in words: [Hebrew, English]. */
    function restoreWhat(spec) {
        var where = (spec.path || 'plan') + (spec.field ? '#' + spec.field : '');
        if (typeof spec.path !== 'string') {
            return ['את כל התוכנית לגרסה ' + spec.fromRevision, 'the whole plan to revision ' + spec.fromRevision];
        }
        if (spec.field) {
            return ['את השדה ' + where + ' לערך שבגרסה ' + spec.fromRevision, 'the field ' + where + ' to its value in revision ' + spec.fromRevision];
        }
        return ['את החלק ' + where + ' למצבו בגרסה ' + spec.fromRevision, 'the part ' + where + ' to its state in revision ' + spec.fromRevision];
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
        var isOpen = false;
        var stale = true;
        var filter = '';
        var rows = [];
        var truncated = false;
        var seq = 0;
        var diffSeq = 0;
        var confirmBox = null;
        var restoring = false;
        var last = { diff: null };
        var pick = { from: 'published', to: 'working' };

        var el = make('section', 'card plan-history');
        el.setAttribute('data-plan-history', '');
        var heading = make('h3', 'plan-history-title');
        var toggle = Ui.button(doc, '', 'btn btn-ghost plan-disclose', { 'data-plan-history-toggle': '' });
        toggle.setAttribute('aria-expanded', 'false');
        toggle.textContent = '▸ ';
        toggle.className += ' ui-pair';
        var histHe = doc.createElement('span');
        histHe.className = 'ui-he';
        histHe.lang = 'he';
        histHe.textContent = 'היסטוריה';
        var histEn = doc.createElement('span');
        histEn.className = 'ui-en';
        histEn.lang = 'en';
        histEn.setAttribute('dir', 'ltr');
        histEn.textContent = 'History';
        toggle.appendChild(histHe);
        toggle.appendChild(histEn);
        heading.appendChild(toggle);
        el.appendChild(heading);

        var bodyEl = make('div', 'plan-history-body');
        bodyEl.hidden = true;
        el.appendChild(bodyEl);

        // tools: who, refresh
        var tools = make('div', 'plan-history-tools');
        var filterField = make('div', 'plan-field');
        var filterLabel = make('label', 'plan-label');
        filterLabel.setAttribute('for', 'plan-history-filter');
        filterLabel.appendChild(Ui.bilingual(doc, 'span', '', 'מי כתב', 'Written by'));
        var filterSelect = make('select');
        filterSelect.setAttribute('id', 'plan-history-filter');
        filterSelect.setAttribute('data-plan-history-filter', '');
        FILTERS.forEach(function (f) {
            var o = make('option', '', Ui.label(f[1], f[2]));
            o.value = f[0];
            filterSelect.appendChild(o);
        });
        filterField.appendChild(filterLabel);
        filterField.appendChild(filterSelect);
        tools.appendChild(filterField);
        tools.appendChild(Ui.button(doc, 'רענון (Refresh)', 'btn btn-ghost admin-row-btn', { 'data-plan-history-refresh': '' }));
        bodyEl.appendChild(tools);

        var stateLine = make('p', 'plan-note');
        stateLine.setAttribute('data-plan-history-state', '');
        stateLine.setAttribute('role', 'status');
        bodyEl.appendChild(stateLine);
        var errorLine = make('p', 'admin-inline-error');
        errorLine.setAttribute('data-plan-history-error', '');
        errorLine.setAttribute('role', 'alert');
        errorLine.hidden = true;
        bodyEl.appendChild(errorLine);

        var list = make('ol', 'plan-rev-list');
        list.setAttribute('data-plan-history-list', '');
        bodyEl.appendChild(list);
        var more = Ui.button(doc, 'טעינת גרסאות ישנות יותר (Load older)', 'btn btn-ghost', { 'data-plan-history-more': '' });
        more.hidden = true;
        bodyEl.appendChild(more);

        // compare
        var compare = make('div', 'plan-compare');
        compare.setAttribute('data-plan-compare', '');
        compare.appendChild(make('h4', 'plan-compare-title', 'השוואה (Compare)'));
        var compareRow = make('div', 'plan-compare-row');
        function stateSelect(id, attr, he, en) {
            var field = make('div', 'plan-field');
            var lab = make('label', 'plan-label');
            lab.setAttribute('for', id);
            lab.appendChild(Ui.bilingual(doc, 'span', '', he, en));
            var sel = make('select');
            sel.setAttribute('id', id);
            sel.setAttribute(attr, '');
            field.appendChild(lab);
            field.appendChild(sel);
            compareRow.appendChild(field);
            return sel;
        }
        var fromSelect = stateSelect('plan-diff-from', 'data-plan-diff-from', 'מצב מוקדם', 'From (earlier)');
        var toSelect = stateSelect('plan-diff-to', 'data-plan-diff-to', 'מצב מאוחר', 'To (later)');
        compare.appendChild(compareRow);
        var pathField = make('div', 'plan-field');
        var pathLabel = make('label', 'plan-label');
        pathLabel.setAttribute('for', 'plan-diff-path');
        pathLabel.appendChild(Ui.bilingual(doc, 'span', '', 'סינון לפי נתיב (לא חובה)', 'Filter by path (optional)'));
        var pathInput = make('input');
        pathInput.type = 'text';
        pathInput.setAttribute('id', 'plan-diff-path');
        pathInput.setAttribute('data-plan-diff-path', '');
        pathInput.setAttribute('autocomplete', 'off');
        pathInput.setAttribute('dir', 'ltr');
        pathInput.setAttribute('lang', 'en');
        pathField.appendChild(pathLabel);
        pathField.appendChild(pathInput);
        compare.appendChild(pathField);
        var compareActions = make('div', 'plan-compare-actions');
        compareActions.appendChild(Ui.button(doc, 'השוואה (Compare)', 'btn', { 'data-plan-diff-run': '' }));
        compareActions.appendChild(Ui.button(doc, 'העבודה מול המפורסמת (Working copy vs. published)', 'btn btn-ghost',
            { 'data-plan-diff-quick': 'working-published' }));
        compare.appendChild(compareActions);
        bodyEl.appendChild(compare);

        var detail = make('div', 'plan-rev-detail');
        detail.setAttribute('data-plan-revision-detail', '');
        detail.hidden = true;
        bodyEl.appendChild(detail);
        var diffBox = make('div', 'plan-diff');
        diffBox.setAttribute('data-plan-diff', '');
        diffBox.setAttribute('aria-live', 'polite');
        bodyEl.appendChild(diffBox);

        function url(suffix) {
            return '/api/plans/' + encodeURIComponent(opts.planId) + suffix;
        }

        function setError(text) {
            errorLine.hidden = !text;
            errorLine.textContent = text || '';
        }

        // ---- the confirmation, in the UI ----

        function closeConfirm() {
            if (confirmBox && confirmBox.parentNode) confirmBox.parentNode.removeChild(confirmBox);
            confirmBox = null;
        }

        /**
         * One confirmation, in plain words, inside `container`. It is the only step before a restore.
         * @param {HTMLElement} container
         * @param {{ fromRevision: number, path?: string, field?: string }} spec
         */
        function confirmAt(container, spec) {
            closeConfirm();
            var box = make('div', 'plan-confirm ui-pair');
            box.setAttribute('data-plan-restore-box', '');
            box.setAttribute('role', 'group');
            box.setAttribute('data-ui-aria-he', 'אישור שחזור');
            box.setAttribute('data-ui-aria-en', 'Confirm restore');
            box.setAttribute('aria-label', 'אישור שחזור');
            var what = restoreWhat(spec);
            var he = make('p', 'plan-confirm-he ui-he', 'לשחזר ' + what[0] + '? זו תהיה גרסה חדשה וההיסטוריה נשמרת.');
            he.setAttribute('lang', 'he');
            var en = make('p', 'plan-confirm-en ui-en', 'Restore ' + what[1] + '? This becomes a new revision; history is kept.');
            en.setAttribute('lang', 'en');
            en.setAttribute('dir', 'ltr');
            box.appendChild(he);
            box.appendChild(en);
            if (typeof opts.hasUnsaved === 'function' && opts.hasUnsaved()) {
                var warn = make('p', 'plan-confirm-en', 'טקסט שלא נשמר בטופס יימחק. (Unsaved text in the form will be dropped.)');
                warn.setAttribute('data-plan-restore-unsaved', '');
                box.appendChild(warn);
            }
            var err = make('p', 'admin-inline-error');
            err.setAttribute('data-plan-restore-error', '');
            err.setAttribute('role', 'alert');
            err.hidden = true;
            box.appendChild(err);
            var row = make('div', 'plan-confirm-actions');
            var yes = Ui.button(doc, 'שחזור (Restore)', 'btn btn-primary', { 'data-plan-restore-confirm': '' });
            var no = Ui.button(doc, 'ביטול (Cancel)', 'btn btn-ghost', { 'data-plan-restore-cancel': '' });
            row.appendChild(yes);
            row.appendChild(no);
            box.appendChild(row);
            box.actionsEl = row;
            box.spec = spec;
            // The box can sit outside the panel (under a field's mark), so it listens for its own clicks.
            removers.push(Ui.listen(box, 'click', onBoxClick, signal));
            container.appendChild(box);
            confirmBox = box;
            if (typeof yes.focus === 'function') yes.focus();
            return box;
        }

        function boxError(box, text, withReload) {
            var err = box.querySelector('[data-plan-restore-error]');
            if (err) {
                err.hidden = !text;
                err.textContent = text || '';
            }
            var existing = box.querySelector('[data-plan-history-reload]');
            if (withReload && !existing) {
                var reload = Ui.button(doc, 'טעינה מחדש (Reload)', 'btn btn-ghost', { 'data-plan-history-reload': '' });
                (box.actionsEl || box).appendChild(reload);
            }
        }

        /** The one POST. Resolves true when the plan was restored. */
        function restore(spec, box) {
            if (restoring) return Promise.resolve(false);
            var revision = typeof opts.getRevision === 'function' ? opts.getRevision() : null;
            if (!Number.isInteger(revision)) return Promise.resolve(false);
            restoring = true;
            var yes = box && box.querySelector('[data-plan-restore-confirm]');
            if (yes) yes.disabled = true;
            if (box) boxError(box, '', false);
            return Promise.resolve(Api.post(url('/restore'), restoreBody(spec, revision), signal ? { signal: signal } : undefined))
                .then(function (res) {
                    restoring = false;
                    if (!alive) return false;
                    if (yes) yes.disabled = false;
                    if (res && res.ok && res.data && Number.isInteger(res.data.revision)) {
                        closeConfirm();
                        stale = true;
                        var after = typeof opts.onRestored === 'function' ? opts.onRestored(res.data) : null;
                        return Promise.resolve(after).then(function (outcome) {
                            if (!alive) return true;
                            if (outcome !== 'refreshed') {
                                stale = true;
                                if (isOpen) return load().then(function () { return true; });
                            }
                            return true;
                        });
                    }
                    var conflict = !!(res && res.status === 409);
                    if (box) boxError(box, Ui.failureText(res), conflict);
                    else setError(Ui.failureText(res));
                    return false;
                }).catch(function (err) {
                    restoring = false;
                    if (yes) yes.disabled = false;
                    if (box) boxError(box, Ui.failureText({ status: 0, data: null, error: err && err.message }), false);
                    return false;
                });
        }

        // ---- the list ----

        function actorNode(row) {
            var mark = actorMark(row);
            var span = make('span', 'plan-actor plan-actor-' + mark.kind);
            span.setAttribute('data-plan-actor-mark', mark.kind);
            var icon = Ui.icon(doc, mark.icon);
            if (icon) span.appendChild(icon);
            span.appendChild(make('span', 'plan-actor-name', Ui.label(mark.he, mark.en)));
            if (mark.detail) {
                var d = make('bdi', 'plan-actor-detail', ' · ' + mark.detail);
                d.setAttribute('dir', 'ltr');
                d.setAttribute('lang', 'en');
                span.appendChild(d);
            }
            return span;
        }

        function renderRow(row) {
            var li = make('li', 'plan-rev');
            li.setAttribute('data-plan-revision', String(row.revision));
            li.setAttribute('data-plan-actor', actorMark(row).kind);
            var head = make('div', 'plan-rev-head');
            var num = make('strong', 'plan-rev-num', '#' + row.revision);
            num.setAttribute('dir', 'ltr');
            head.appendChild(num);
            head.appendChild(actorNode(row));
            var time = make('time', 'plan-rev-time', Ui.formatTime(row.at));
            if (typeof row.at === 'string') time.setAttribute('datetime', row.at);
            time.setAttribute('dir', 'ltr');
            head.appendChild(time);
            li.appendChild(head);
            var op = make('p', 'plan-rev-summary plan-rev-op', typeof row.op === 'string' ? row.op : '');
            op.setAttribute('data-plan-op', '');
            op.setAttribute('dir', 'ltr');
            li.appendChild(op);
            var where = (typeof row.path === 'string' ? row.path : '') + (row.field ? '#' + row.field : '');
            if (where) {
                var w = make('p', 'plan-rev-summary plan-rev-where', where);
                w.setAttribute('dir', 'ltr');
                w.setAttribute('lang', 'en');
                li.appendChild(w);
            }
            var actions = make('div', 'plan-rev-actions');
            actions.appendChild(Ui.button(doc, 'פתיחה (Open)', 'btn btn-ghost admin-row-btn',
                { 'data-plan-rev-open': String(row.revision) }));
            actions.appendChild(Ui.button(doc, 'מול העבודה (Against the working copy)', 'btn btn-ghost admin-row-btn',
                { 'data-plan-diff-quick': 'revision', 'data-plan-rev': String(row.revision) }));
            actions.appendChild(Ui.button(doc, 'שחזור כל התוכנית לגרסה הזו (Restore the whole plan to this revision)',
                'btn btn-ghost admin-row-btn', { 'data-plan-restore': 'whole', 'data-plan-rev': String(row.revision) }));
            li.appendChild(actions);
            return li;
        }

        function fillSelect(select, wanted) {
            Ui.clear(select);
            var options = [['published', 'המפורסמת (published copy)'], ['working', 'העבודה (working copy)']];
            rows.forEach(function (r) {
                options.push([String(r.revision), 'גרסה ' + r.revision + ' (revision ' + r.revision + ')']);
            });
            options.forEach(function (o) {
                var node = make('option', '', o[1]);
                node.value = o[0];
                select.appendChild(node);
            });
            select.value = wanted;
        }

        function renderList() {
            Ui.clear(list);
            rows.forEach(function (row) { list.appendChild(renderRow(row)); });
            more.hidden = !truncated;
            fillSelect(fromSelect, pick.from);
            fillSelect(toSelect, pick.to);
            if (!rows.length) {
                var none = make('li', 'plan-note', filter ? 'אין גרסאות של הכותב הזה. (No revisions by that writer.)' : 'אין עדיין היסטוריה. (No history yet.)');
                none.setAttribute('data-plan-history-empty', '');
                list.appendChild(none);
            }
        }

        function load(older) {
            seq += 1;
            var mine = seq;
            stale = false;
            var query = [];
            if (filter) query.push('actorKind=' + encodeURIComponent(filter));
            if (older && rows.length) query.push('beforeRevision=' + encodeURIComponent(String(rows[rows.length - 1].revision)));
            Ui.fill(stateLine, 'טוען היסטוריה… (Loading the history…)');
            setError('');
            return Promise.resolve(Api.get(url('/revisions' + (query.length ? '?' + query.join('&') : '')),
                signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive || mine !== seq) return false;
                stateLine.textContent = '';
                if (!res || !res.ok || !isObject(res.data) || !Array.isArray(res.data.revisions)) {
                    setError(Ui.line(
                        'לא ניתן לטעון את ההיסטוריה — ' + Ui.failureText(res),
                        'Could not load the history — ' + Ui.failureText(res)
                    ));
                    return false;
                }
                var incoming = res.data.revisions.filter(function (r) { return isObject(r) && Number.isInteger(r.revision); });
                rows = older ? rows.concat(incoming) : incoming;
                truncated = res.data.truncated === true;
                renderList();
                return true;
            }).catch(function (err) {
                if (!alive || mine !== seq) return false;
                stateLine.textContent = '';
                var fail = Ui.failureText({ status: 0, data: null, error: err && err.message });
                setError(Ui.line(
                    'לא ניתן לטעון את ההיסטוריה — ' + fail,
                    'Could not load the history — ' + fail
                ));
                return false;
            });
        }

        // ---- diff ----

        function sideBox(caption, attr, present, value, truncatedValue) {
            var box = make('div', 'plan-diff-side');
            box.setAttribute(attr, '');
            box.appendChild(make('span', 'plan-diff-caption', caption));
            var text = present ? Ui.valueText(value) : '';
            var val = make('bdi', 'plan-diff-value', present ? (text === '' ? '""' : text) : '—');
            if (present) Ui.applyTextAttrs(val, text);
            box.appendChild(val);
            if (present && truncatedValue) {
                var cut = make('span', 'plan-diff-cut');
                cut.setAttribute('data-plan-diff-cut', '');
                Ui.fillPair(cut, ' … נחתך', ' … cut');
                box.appendChild(cut);
            }
            return box;
        }

        function renderEntry(entry, from) {
            var li = make('li', 'plan-diff-entry');
            li.setAttribute('data-plan-diff-entry', typeof entry.field === 'string' && entry.field ? entry.field : entry.kind);
            li.setAttribute('data-plan-diff-kind', entry.kind);
            var kindPair = KIND_LABELS[entry.kind] || [entry.kind, entry.kind];
            var head = make('p', 'plan-diff-field');
            head.appendChild(make('span', 'plan-chip plan-chip-' + (entry.kind === 'changed' ? 'draft' : 'changes'), Ui.label(kindPair[0], kindPair[1])));
            if (entry.field) {
                var f = make('bdi', 'plan-diff-fieldname', ' ' + entry.field);
                f.setAttribute('dir', 'ltr');
                f.setAttribute('lang', 'en');
                head.appendChild(f);
            }
            li.appendChild(head);
            var sides = make('div', 'plan-diff-sides');
            var hasBefore = Object.prototype.hasOwnProperty.call(entry, 'before');
            var hasAfter = Object.prototype.hasOwnProperty.call(entry, 'after');
            sides.appendChild(sideBox('לפני (Before)', 'data-plan-diff-before', hasBefore, entry.before, entry.valueTruncated === true));
            sides.appendChild(sideBox('אחרי (After)', 'data-plan-diff-after', hasAfter, entry.after, entry.valueTruncated === true));
            li.appendChild(sides);
            var offers = restoreOffers(entry, from);
            if (offers.field || offers.part) {
                var actions = make('div', 'plan-diff-actions');
                if (offers.field) {
                    actions.appendChild(Ui.button(doc, 'שחזור הערך הזה (Restore this value)', 'btn btn-ghost admin-row-btn', {
                        'data-plan-restore': 'field',
                        'data-plan-restore-path': entry.path,
                        'data-plan-restore-field': entry.field,
                        'data-plan-rev': String(offers.fromRevision)
                    }));
                }
                if (offers.part) {
                    actions.appendChild(Ui.button(doc, 'שחזור החלק הזה (Restore this part)', 'btn btn-ghost admin-row-btn', {
                        'data-plan-restore': 'part',
                        'data-plan-restore-path': entry.path,
                        'data-plan-rev': String(offers.fromRevision)
                    }));
                }
                li.appendChild(actions);
            }
            return li;
        }

        function renderDiff(data) {
            Ui.clear(diffBox);
            last.diff = data;
            var head = make('p', 'plan-diff-head');
            head.setAttribute('data-plan-diff-head', '');
            var fromP = describeRefParts(data.from);
            var toP = describeRefParts(data.to);
            Ui.fillPair(
                head,
                fromP.he + fromP.suffix + ' → ' + toP.he + toP.suffix,
                fromP.en + fromP.suffix + ' → ' + toP.en + toP.suffix
            );
            diffBox.appendChild(head);
            var entries = Array.isArray(data.entries) ? data.entries : [];
            if (!entries.length) {
                var none = make('p', 'plan-note', 'אין הבדלים. (No differences.)');
                none.setAttribute('data-plan-diff-empty', '');
                diffBox.appendChild(none);
            }
            groupEntries(entries).forEach(function (group) {
                var box = make('div', 'plan-diff-group');
                var path = make('p', 'plan-diff-path-name');
                var name = make('bdi', '', group.path || 'plan');
                name.setAttribute('dir', 'ltr');
                name.setAttribute('lang', 'en');
                path.appendChild(name);
                box.appendChild(path);
                var ul = make('ul', 'plan-diff-list');
                group.entries.forEach(function (entry) { ul.appendChild(renderEntry(entry, data.from)); });
                box.appendChild(ul);
                diffBox.appendChild(box);
            });
            if (data.truncated === true) {
                var cut = make('p', 'plan-note', 'ההשוואה נחתכה. אפשר לצמצם אותה לפי נתיב. (The comparison was cut. Narrow it with a path.)');
                cut.setAttribute('data-plan-diff-truncated', '');
                diffBox.appendChild(cut);
            }
        }

        /** One GET .../diff. `from` and `to` are a revision number as text, `working`, or `published`. */
        function diff(from, to, pathPrefix) {
            diffSeq += 1;
            var mine = diffSeq;
            pick = { from: String(from), to: String(to) };
            fromSelect.value = pick.from;
            toSelect.value = pick.to;
            var query = 'from=' + encodeURIComponent(String(from)) + '&to=' + encodeURIComponent(String(to));
            if (pathPrefix) query += '&path=' + encodeURIComponent(pathPrefix);
            Ui.clear(diffBox);
            diffBox.appendChild(make('p', 'plan-note', 'טוען השוואה… (Loading the comparison…)'));
            return Promise.resolve(Api.get(url('/diff?' + query), signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive || mine !== diffSeq) return null;
                if (!res || !res.ok || !isObject(res.data) || !Array.isArray(res.data.entries)) {
                    Ui.clear(diffBox);
                    var bad = make('p', 'admin-inline-error');
                    bad.setAttribute('role', 'alert');
                    bad.setAttribute('data-plan-diff-error', '');
                    bad.textContent = Ui.line(
                        'לא ניתן להשוות — ' + Ui.failureText(res),
                        'Could not compare — ' + Ui.failureText(res)
                    );
                    diffBox.appendChild(bad);
                    return null;
                }
                renderDiff(res.data);
                return res.data;
            }).catch(function (err) {
                if (!alive || mine !== diffSeq) return null;
                Ui.clear(diffBox);
                diffBox.appendChild(make('p', 'admin-inline-error', Ui.failureText({ status: 0, data: null, error: err && err.message })));
                return null;
            });
        }

        /** Open a revision: read it, then show what it changed against the one before. */
        function openRevision(n) {
            detail.hidden = false;
            Ui.clear(detail);
            detail.appendChild(make('p', 'plan-note', 'טוען גרסה… (Loading the revision…)'));
            return Promise.resolve(Api.get(url('/revisions/' + encodeURIComponent(String(n))), signal ? { signal: signal } : undefined))
                .then(function (res) {
                    if (!alive) return null;
                    Ui.clear(detail);
                    if (!res || !res.ok || !isObject(res.data)) {
                        var bad = make('p', 'admin-inline-error');
                        bad.setAttribute('role', 'alert');
                        bad.textContent = Ui.line(
                            'לא ניתן לפתוח את הגרסה — ' + Ui.failureText(res),
                            'Could not open the revision — ' + Ui.failureText(res)
                        );
                        detail.appendChild(bad);
                        return null;
                    }
                    var data = res.data;
                    var head = make('p', 'plan-rev-detail-head');
                    var mark = actorNode(data);
                    var revTitle = make('strong', '');
                    revTitle.setAttribute('data-plan-rev-title', '');
                    Ui.fillPair(revTitle, 'גרסה ' + data.revision, 'Revision ' + data.revision, ' · ');
                    head.appendChild(revTitle);
                    head.appendChild(mark);
                    head.appendChild(make('span', 'plan-rev-time', ' · ' + Ui.formatTime(data.at)));
                    detail.appendChild(head);
                    var meta = make('p', 'plan-note');
                    meta.setAttribute('dir', 'ltr');
                    meta.setAttribute('lang', 'en');
                    meta.textContent = [data.op, data.tool, data.client].filter(function (x) { return typeof x === 'string' && x; }).join(' · ');
                    detail.appendChild(meta);
                    if (data.revision > 1) return diff(data.revision - 1, data.revision, '').then(function () { return data; });
                    Ui.clear(diffBox);
                    diffBox.appendChild(make('p', 'plan-note', 'זו הגרסה הראשונה. (This is the first revision.)'));
                    return data;
                }).catch(function () { return null; });
        }

        // ---- opening and events ----

        function setOpen(next) {
            isOpen = !!next;
            toggle.setAttribute('aria-expanded', isOpen ? 'true' : 'false');
            Ui.clear(toggle);
            var mark = doc.createElement('span');
            mark.setAttribute('aria-hidden', 'true');
            mark.textContent = isOpen ? '▾ ' : '▸ ';
            toggle.appendChild(mark);
            if ((toggle.className || '').indexOf('ui-pair') === -1) toggle.className += ' ui-pair';
            var he = doc.createElement('span');
            he.className = 'ui-he';
            he.lang = 'he';
            he.textContent = 'היסטוריה';
            var en = doc.createElement('span');
            en.className = 'ui-en';
            en.lang = 'en';
            en.setAttribute('dir', 'ltr');
            en.textContent = 'History';
            toggle.appendChild(he);
            toggle.appendChild(en);
            bodyEl.hidden = !isOpen;
            if (isOpen && stale) return load();
            return Promise.resolve();
        }

        /** Clicks inside the confirmation: cancel, confirm, and the reload that follows a conflict. */
        function onBoxClick(ev) {
            var node = ev && ev.target;
            if (node && node.nodeType === 3) node = node.parentNode;
            var box = confirmBox;
            if (!box) return;
            if (Ui.closestAttr(node, 'data-plan-restore-cancel', box.parentNode)) {
                closeConfirm();
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-restore-confirm', box.parentNode)) {
                if (box.spec) restore(box.spec, box);
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-history-reload', box.parentNode)) {
                if (typeof opts.onReload === 'function') opts.onReload();
            }
        }

        function specFrom(node) {
            var kind = node.getAttribute('data-plan-restore');
            var from = Number(node.getAttribute('data-plan-rev'));
            if (!Number.isInteger(from)) return null;
            if (kind === 'whole') return { fromRevision: from };
            var path = node.getAttribute('data-plan-restore-path');
            if (path === null) return null;
            var spec = { fromRevision: from, path: path };
            if (kind === 'field') spec.field = node.getAttribute('data-plan-restore-field') || '';
            return spec;
        }

        function onClick(ev) {
            var node = ev && ev.target;
            if (node && node.nodeType === 3) node = node.parentNode;
            if (Ui.closestAttr(node, 'data-plan-history-toggle', el)) {
                setOpen(!isOpen);
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-history-refresh', el)) {
                load();
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-history-more', el)) {
                load(true);
                return;
            }
            var restoreButton = Ui.closestAttr(node, 'data-plan-restore', el);
            if (restoreButton) {
                var spec = specFrom(restoreButton);
                var host = Ui.closestAttr(restoreButton, 'data-plan-revision', el)
                    || Ui.closestAttr(restoreButton, 'data-plan-diff-entry', el) || restoreButton.parentNode;
                if (spec) confirmAt(host, spec);
                return;
            }
            var open = Ui.closestAttr(node, 'data-plan-rev-open', el);
            if (open) {
                openRevision(Number(open.getAttribute('data-plan-rev-open')));
                return;
            }
            var quick = Ui.closestAttr(node, 'data-plan-diff-quick', el);
            if (quick) {
                var which = quick.getAttribute('data-plan-diff-quick');
                if (which === 'working-published') diff('published', 'working', pathInput.value.trim());
                else diff(quick.getAttribute('data-plan-rev'), 'working', pathInput.value.trim());
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-diff-run', el)) {
                diff(fromSelect.value || 'published', toSelect.value || 'working', pathInput.value.trim());
            }
        }

        function onChange(ev) {
            var node = ev && ev.target;
            if (node && Ui.closestAttr(node, 'data-plan-history-filter', el)) {
                filter = String(filterSelect.value || '');
                rows = [];
                load();
            }
        }
        removers.push(Ui.listen(el, 'click', onClick, signal));
        removers.push(Ui.listen(el, 'change', onChange, signal));

        return {
            el: el,
            load: function () {
                if (!isOpen) {
                    stale = true;
                    return Promise.resolve(false);
                }
                return load();
            },
            open: function () { return setOpen(true); },
            isOpen: function () { return isOpen; },
            confirmAt: confirmAt,
            diff: diff,
            restore: restore,
            openRevision: openRevision,
            destroy: function () {
                alive = false;
                removers.forEach(function (remove) { remove(); });
                removers = [];
            }
        };
    }

    return {
        ACTOR_KINDS: ACTOR_KINDS,
        actorMark: actorMark,
        create: create,
        describeRef: describeRef,
        groupEntries: groupEntries,
        restoreBody: restoreBody,
        restoreOffers: restoreOffers,
        restoreWhat: restoreWhat
    };
});
