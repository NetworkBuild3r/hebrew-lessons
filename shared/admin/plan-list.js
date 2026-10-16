/**
 * The Plans tab of the admin (INIT-008/SPEC-013): a table of every plan, a "New plan" form, and
 * the assign control that the plan view shows. It is a client of the plan routes and holds no
 * plan logic:
 *   list     GET  /api/plans                        -> { plans: [...], truncated }
 *   create   POST /api/plans   { title }            -> { plan, revision, status, validation }
 *   assign   POST /api/plans/:id/assign { audience, assigneeId?, expectedRevision }
 *   accounts GET  /api/admin/users                  (the same read the Users tab uses)
 *   schema   GET  /api/plan-schema                  (the title limit comes from it)
 * The server assigns a UUID as the plan id. New plan asks for a title and who it is for
 * (one learner, or everyone) — no recipe field. A learner pick calls create then assign so
 * the plan ends personal with that assignee. A failed call shows the server's code in
 * plain words and keeps what the person typed. Server text goes in through textContent only.
 *
 *   var stop = HebrewPlanList.mount(panel, { api, setHash, signal })   // returns a cleanup function
 *   var assign = HebrewPlanList.createAssign({ doc, api, planId, signal, getRevision, onAssigned, onConflict })
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-013, INIT-011/SPEC-003, INIT-011/SPEC-004
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
    root.HebrewPlanList = api;
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var Ui = root.HebrewPlanUi;
    var SLUG_FALLBACK = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var TITLE_FALLBACK = 4000;
    var ASSIGN_AUDIENCES = ['shared', 'personal'];

    function isObject(value) {
        return value !== null && typeof value === 'object' && !Array.isArray(value);
    }

    // ---- pure helpers ---------------------------------------------------------------------------

    /**
     * What the New plan form checks before it asks the server. The title limit comes from the
     * schema when it loaded; the server assigns the id and checks the title again.
     * @param {{ title?: string }} input
     * @param {{ titleMax?: number }} [limits]
     * @returns {{ title?: string }} a code when the title is bad; empty when it is fine
     */
    function checkNew(input, limits) {
        var max = (limits && limits.titleMax) || TITLE_FALLBACK;
        var title = String(input && input.title != null ? input.title : '').trim();
        var out = {};
        var titleCode = !title ? 'empty_title' : (title.length > max ? 'title_too_long' : '');
        if (titleCode) out.title = titleCode;
        return out;
    }

    /** The slug pattern and the title limit the schema states, or the fallbacks. */
    function limitsFromSchema(schema) {
        var out = { slug: SLUG_FALLBACK, titleMax: TITLE_FALLBACK, recipes: [] };
        var Form = root.HebrewPlanForm;
        if (!isObject(schema) || !Form) return out;
        var tools = Form.createTools(schema);
        var idNode = schema.properties && tools.resolve(schema.properties.id).schema;
        if (idNode && typeof idNode.pattern === 'string') {
            try { out.slug = new RegExp(idNode.pattern); } catch (err) { out.slug = SLUG_FALLBACK; }
        }
        var titleNode = schema.properties && tools.resolve(schema.properties.title).schema;
        if (titleNode && Number.isInteger(titleNode.maxLength)) out.titleMax = titleNode.maxLength;
        // recipes stay on the schema helper for older callers; New plan no longer offers them.
        out.recipes = Form.enumOf('lesson', 'recipe', schema);
        return out;
    }

    /** Hebrew over English labels for a recipe, from the editor's hints (kept for schema helpers). */
    function recipeLabel(recipe) {
        var hints = root.HebrewPlanHints;
        var hint = hints && hints.fieldHint ? hints.fieldHint('lesson.recipe') : null;
        var pair = hint && hint.options && hint.options[recipe];
        return pair ? pair[0] + ' (' + pair[1] + ')' : String(recipe);
    }

    /** `{ key, he, en }` for a plan row's status. */
    function statusOf(plan) {
        if (plan && plan.status === 'published') return { key: 'published', he: 'פורסמה', en: 'Published' };
        return { key: 'draft', he: 'טיוטה', en: 'Draft' };
    }

    /** `{ key, he, en, who }` for a plan row's audience; `who` is the assignee an admin sees. */
    function audienceOf(plan) {
        if (plan && plan.audience === 'personal') {
            return {
                key: 'personal',
                he: 'אישית',
                en: 'Personal',
                who: typeof plan.assigneeId === 'string' ? plan.assigneeId : ''
            };
        }
        return { key: 'shared', he: 'משותפת', en: 'Shared', who: '' };
    }

    /** The accounts an admin may assign a plan to: active ones, by username. */
    function activeUsers(users) {
        return (Array.isArray(users) ? users : []).filter(function (u) {
            return isObject(u) && typeof u.username === 'string' && u.username && u.status === 'active';
        }).sort(function (a, b) { return a.username < b.username ? -1 : (a.username > b.username ? 1 : 0); });
    }

    /** The rows of the list as plain data, for the table and for tests. */
    function rowsOf(data) {
        var plans = data && Array.isArray(data.plans) ? data.plans : [];
        return plans.filter(function (p) { return isObject(p) && typeof p.id === 'string'; }).map(function (p) {
            return {
                id: p.id,
                title: typeof p.title === 'string' && p.title ? p.title : p.id,
                status: statusOf(p),
                audience: audienceOf(p),
                revision: Number.isInteger(p.revision) ? p.revision : null,
                unpublishedChanges: p.unpublishedChanges === true,
                published: p.published === true,
                humanEditedFields: Number.isInteger(p.humanEditedFields) ? p.humanEditedFields : 0,
                updatedAt: typeof p.updatedAt === 'string' ? p.updatedAt : ''
            };
        });
    }

    // ---- the list view --------------------------------------------------------------------------

    var COLUMNS = [
        ['title', 'תוכנית', 'Plan'],
        ['status', 'מצב', 'Status'],
        ['audience', 'קהל', 'Audience'],
        ['revision', 'גרסה', 'Revision'],
        ['human', 'נערכו על ידי אדם', 'Edited by a person'],
        ['updated', 'עודכנה', 'Updated']
    ];

    /**
     * @param {HTMLElement} panel
     * @param {{ api?: object, doc?: Document, setHash?: function(string): void, signal?: AbortSignal|null }} [options]
     * @returns {function(): void} cleanup
     */
    function mount(panel, options) {
        var opts = options || {};
        var doc = opts.doc || root.document;
        var Api = opts.api || root.HebrewApi;
        var make = function (tag, cls, text) { return Ui.make(doc, tag, cls, text); };
        var controller = typeof root.AbortController === 'function' ? new root.AbortController() : null;
        var signal = controller ? controller.signal : (opts.signal || null);
        var removers = [];
        var alive = true;
        var limits = { slug: SLUG_FALLBACK, titleMax: TITLE_FALLBACK, recipes: [] };
        var newOpen = null;
        var creating = false;
        var typed = { title: '', forUser: '' };
        var accountUsers = null;
        var listed = [];
        var filterText = '';
        var filterStatus = 'all';
        var filterAudience = 'all';

        function go(planId) {
            if (typeof opts.setHash === 'function') opts.setHash('plan/' + planId);
            else if (root.HebrewEditorShell && typeof root.HebrewEditorShell.setHash === 'function') {
                root.HebrewEditorShell.setHash('plan/' + planId);
            } else if (root.HebrewAdminShell && typeof root.HebrewAdminShell.setHash === 'function') {
                root.HebrewAdminShell.setHash('plan/' + planId);
            }
        }

        Ui.clear(panel);
        var box = make('div', 'card plan-builder');
        box.setAttribute('data-plan-builder', 'picker');
        box.setAttribute('data-admin-view', 'plans');
        box.setAttribute('dir', 'rtl');
        box.setAttribute('lang', 'he');
        panel.appendChild(box);

        var heading = make('h2', '', 'ספריית תוכניות (Plan library)');
        box.appendChild(heading);
        var intro = make('p', 'plan-help ui-pair');
        var introHe = make('span', 'plan-help-he ui-he', 'כל תוכניות השיעור, בין שה-AI בנה אותן ובין שאדם. פתחו תוכנית כדי לערוך אותה.');
        introHe.setAttribute('lang', 'he');
        var introEn = make('span', 'plan-help-en ui-en', 'Open a plan to edit its lesson order, add activities, and prepare new vocabulary notes.');
        introEn.setAttribute('lang', 'en');
        introEn.setAttribute('dir', 'ltr');
        intro.appendChild(introHe);
        intro.appendChild(introEn);
        box.appendChild(intro);

        var actions = make('div', 'plan-list-actions');
        var newButton = Ui.button(doc, 'תוכנית חדשה (New plan)', 'btn btn-primary', { 'data-plan-new-open': '' });
        newButton.setAttribute('aria-expanded', 'false');
        var refresh = Ui.button(doc, 'רענון (Refresh)', 'btn btn-ghost', { 'data-plan-list-refresh': '' });
        actions.appendChild(newButton);
        actions.appendChild(refresh);
        box.appendChild(actions);

        var filters = make('div', 'plan-filters');
        var search = make('input', 'plan-filter-search');
        search.setAttribute('type', 'search');
        search.setAttribute('data-plan-filter-search', '');
        search.setAttribute('placeholder', 'Search title or id');
        search.setAttribute('aria-label', 'Search plans');
        var statusFilter = make('select', 'plan-filter-status');
        statusFilter.setAttribute('data-plan-filter-status', '');
        statusFilter.setAttribute('aria-label', 'Status');
        [['all', 'All statuses'], ['draft', 'Draft'], ['published', 'Published']].forEach(function (pair) {
            var option = make('option', '', pair[1]);
            option.setAttribute('value', pair[0]);
            statusFilter.appendChild(option);
        });
        var audienceFilter = make('select', 'plan-filter-audience');
        audienceFilter.setAttribute('data-plan-filter-audience', '');
        audienceFilter.setAttribute('aria-label', 'Audience');
        [['all', 'All audiences'], ['shared', 'Shared'], ['personal', 'Personal']].forEach(function (pair) {
            var option = make('option', '', pair[1]);
            option.setAttribute('value', pair[0]);
            audienceFilter.appendChild(option);
        });
        filters.appendChild(search);
        filters.appendChild(statusFilter);
        filters.appendChild(audienceFilter);
        box.appendChild(filters);

        var formHost = make('div', 'plan-new-host');
        box.appendChild(formHost);

        var status = make('p', 'plan-note');
        status.setAttribute('role', 'status');
        status.setAttribute('data-plan-list-status', '');
        box.appendChild(status);
        var errorBox = make('div', 'admin-inline-error');
        errorBox.setAttribute('role', 'alert');
        errorBox.setAttribute('data-plan-error', '');
        errorBox.hidden = true;
        var errorText = make('p', '');
        errorBox.appendChild(errorText);
        var retry = Ui.button(doc, 'ניסיון חוזר (Retry)', 'btn btn-ghost', { 'data-plan-list-retry': '' });
        errorBox.appendChild(retry);
        box.appendChild(errorBox);

        var wrap = make('div', 'plan-table-wrap');
        wrap.setAttribute('data-plan-list', '');
        wrap.hidden = true;
        box.appendChild(wrap);
        var emptyNote = make('p', 'plan-note', 'אין עדיין תוכניות. אפשר ליצור אחת בלחיצה על ״תוכנית חדשה״. '
            + '(No plans yet. Use “New plan” to make one.)');
        emptyNote.setAttribute('data-plan-list-empty', '');
        emptyNote.hidden = true;
        box.appendChild(emptyNote);
        var truncatedNote = make('p', 'plan-note', 'מוצגות התוכניות האחרונות בלבד. (Only the most recently changed plans are shown.)');
        truncatedNote.setAttribute('data-plan-list-truncated', '');
        truncatedNote.hidden = true;
        box.appendChild(truncatedNote);

        function visibleRows() {
            return listed.filter(function (row) {
                if (filterStatus !== 'all' && row.status.key !== filterStatus) return false;
                if (filterAudience !== 'all' && row.audience.key !== filterAudience) return false;
                if (!filterText) return true;
                var hay = (row.title + ' ' + row.id).toLowerCase();
                return hay.indexOf(filterText) !== -1;
            });
        }

        function renderTable(rows) {
            Ui.clear(wrap);
            var table = make('table', 'plan-table');
            table.setAttribute('role', 'table');
            var cap = make('caption', 'plan-table-caption', 'רשימת תוכניות (List of plans)');
            table.appendChild(cap);
            var thead = make('thead');
            var headRow = make('tr');
            headRow.setAttribute('role', 'row');
            COLUMNS.forEach(function (col) {
                var th = make('th');
                th.setAttribute('scope', 'col');
                th.setAttribute('role', 'columnheader');
                th.appendChild(Ui.bilingual(doc, 'span', '', col[1], col[2]));
                headRow.appendChild(th);
            });
            thead.appendChild(headRow);
            table.appendChild(thead);
            var tbody = make('tbody');
            rows.forEach(function (row) {
                var tr = make('tr');
                tr.setAttribute('role', 'row');
                tr.setAttribute('data-plan-row', row.id);
                tr.setAttribute('data-plan-row-status', row.status.key);
                function cell(index, build) {
                    var td = make('td');
                    td.setAttribute('role', 'cell');
                    td.setAttribute('data-label', COLUMNS[index][2]);
                    td.setAttribute('data-ui-label-he', COLUMNS[index][1]);
                    td.setAttribute('data-ui-label-en', COLUMNS[index][2]);
                    build(td);
                    tr.appendChild(td);
                }
                cell(0, function (td) {
                    var link = make('a', 'plan-row-link', row.title);
                    link.setAttribute('href', '#plan/' + encodeURIComponent(row.id));
                    link.setAttribute('data-plan-open-link', row.id);
                    td.appendChild(link);
                    var idLine = make('span', 'plan-row-id', row.id);
                    idLine.setAttribute('dir', 'ltr');
                    idLine.setAttribute('lang', 'en');
                    td.appendChild(idLine);
                });
                cell(1, function (td) {
                    td.appendChild(make('span', 'plan-chip plan-chip-' + row.status.key, Ui.label(row.status.he, row.status.en)));
                    if (row.unpublishedChanges) {
                        var chip = make('span', 'plan-chip plan-chip-changes', Ui.label('שינויים שלא פורסמו', 'Unpublished changes'));
                        chip.setAttribute('data-plan-unpublished', '');
                        td.appendChild(chip);
                    }
                });
                cell(2, function (td) {
                    td.appendChild(make('span', '', Ui.label(row.audience.he, row.audience.en)));
                    if (row.audience.who) {
                        var who = make('span', 'plan-row-id', row.audience.who);
                        who.setAttribute('dir', 'ltr');
                        who.setAttribute('lang', 'en');
                        who.setAttribute('data-plan-assignee', '');
                        td.appendChild(who);
                    }
                });
                cell(3, function (td) {
                    var n = make('span', 'plan-num', row.revision === null ? '' : String(row.revision));
                    n.setAttribute('dir', 'ltr');
                    td.appendChild(n);
                });
                cell(4, function (td) {
                    var n = make('span', 'plan-num', String(row.humanEditedFields));
                    n.setAttribute('dir', 'ltr');
                    n.setAttribute('data-plan-human-fields', String(row.humanEditedFields));
                    td.appendChild(n);
                });
                cell(5, function (td) {
                    var t = make('span', 'plan-num', Ui.formatTime(row.updatedAt));
                    t.setAttribute('dir', 'ltr');
                    td.appendChild(t);
                });
                tbody.appendChild(tr);
            });
            table.appendChild(tbody);
            wrap.appendChild(table);
        }

        function showError(res) {
            errorBox.hidden = false;
            errorText.textContent = Ui.line(
                'לא ניתן לטעון את התוכניות — ' + Ui.failureText(res),
                'Could not load the plans — ' + Ui.failureText(res)
            );
        }

        function load() {
            Ui.fill(status, 'טוען תוכניות… (Loading the plans…)');
            errorBox.hidden = true;
            return Promise.resolve(Api.get('/api/plans', signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive) return;
                status.textContent = '';
                if (!res || !res.ok || !res.data || !Array.isArray(res.data.plans)) {
                    wrap.hidden = true;
                    emptyNote.hidden = true;
                    showError(res);
                    return;
                }
                listed = rowsOf(res.data);
                var rows = visibleRows();
                truncatedNote.hidden = res.data.truncated !== true;
                if (!rows.length) {
                    wrap.hidden = true;
                    emptyNote.hidden = false;
                    if (newOpen === null) openForm(true);
                    return;
                }
                emptyNote.hidden = true;
                wrap.hidden = false;
                renderTable(rows);
                Ui.clear(status);
                if (status.className.indexOf('ui-pair') === -1) status.className += ' ui-pair';
                var countHe = make('span', 'ui-he', rows.length + ' תוכניות');
                countHe.setAttribute('lang', 'he');
                var countEn = make('span', 'ui-en', rows.length + ' plans');
                countEn.setAttribute('lang', 'en');
                countEn.setAttribute('dir', 'ltr');
                status.appendChild(countHe);
                status.appendChild(countEn);
            }).catch(function (err) {
                if (!alive) return;
                status.textContent = '';
                showError({ status: 0, data: null, error: err && err.message });
            });
        }

        // ---- the New plan form ----

        function field(id, he, en, control, help) {
            var group = make('div', 'plan-field');
            var lab = make('label', 'plan-label');
            lab.setAttribute('for', id);
            lab.appendChild(Ui.bilingual(doc, 'span', '', he, en));
            control.setAttribute('id', id);
            group.appendChild(lab);
            group.appendChild(control);
            if (help) {
                var p = make('p', 'plan-help ui-pair');
                var hHe = make('span', 'plan-help-he ui-he', help[0]);
                hHe.setAttribute('lang', 'he');
                var hEn = make('span', 'plan-help-en ui-en', help[1]);
                hEn.setAttribute('lang', 'en');
                hEn.setAttribute('dir', 'ltr');
                p.appendChild(hHe);
                p.appendChild(hEn);
                group.appendChild(p);
            }
            return group;
        }

        function fillForSelect(forSelect) {
            Ui.clear(forSelect);
            var everyone = make('option');
            everyone.value = '';
            everyone.setAttribute('data-ui-text-he', 'לכל הלומדים');
            everyone.setAttribute('data-ui-text-en', 'Everyone');
            everyone.textContent = Ui.line('לכל הלומדים', 'Everyone');
            forSelect.appendChild(everyone);
            activeUsers(accountUsers).forEach(function (u) {
                var o = make('option');
                o.value = u.username;
                var label = u.display_name ? u.username + ' — ' + u.display_name : u.username;
                o.textContent = label;
                forSelect.appendChild(o);
            });
            forSelect.value = typed.forUser || '';
        }

        function loadAccounts() {
            if (accountUsers !== null || !Api) return Promise.resolve();
            return Promise.resolve(Api.get('/api/admin/users', signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive) return;
                accountUsers = (res && res.ok && res.data && Array.isArray(res.data.users))
                    ? res.data.users
                    : [];
                if (newOpen && formHost.firstChild) {
                    var forSelect = formHost.querySelector('[data-plan-new-for]');
                    if (forSelect) fillForSelect(forSelect);
                }
            }).catch(function () {
                if (!alive) return;
                accountUsers = [];
            });
        }

        function buildForm() {
            Ui.clear(formHost);
            var card = make('form', 'plan-new');
            card.setAttribute('data-plan-new', '');
            card.setAttribute('novalidate', '');
            card.setAttribute('aria-label', Ui.line('תוכנית חדשה', 'New plan'));
            var h = make('h3');
            Ui.fill(h, 'תוכנית חדשה (New plan)');
            card.appendChild(h);

            var titleInput = make('input');
            titleInput.type = 'text';
            titleInput.setAttribute('data-plan-new-title', '');
            titleInput.setAttribute('autocomplete', 'off');
            titleInput.value = typed.title;
            card.appendChild(field('plan-new-title', 'כותרת', 'Title', titleInput, null));
            var titleMsg = make('p', 'plan-issue plan-issue-block');
            titleMsg.setAttribute('data-plan-new-title-message', '');
            titleMsg.setAttribute('role', 'alert');
            titleMsg.hidden = true;
            card.appendChild(titleMsg);

            var forSelect = make('select');
            forSelect.setAttribute('data-plan-new-for', '');
            fillForSelect(forSelect);
            card.appendChild(field('plan-new-for', 'עבור', 'For', forSelect,
                ['לומד אחד מהרשימה, או כולם.',
                    'One learner from the account list, or everyone.']));

            var message = make('p', 'admin-inline-error');
            message.setAttribute('data-plan-new-message', '');
            message.setAttribute('role', 'alert');
            message.hidden = true;
            card.appendChild(message);
            var create = Ui.button(doc, 'יצירה (Create)', 'btn btn-primary', { 'data-plan-create': '' });
            card.appendChild(create);
            formHost.appendChild(card);
            loadAccounts();

            function remember() {
                typed.title = String(titleInput.value);
                typed.forUser = String(forSelect.value || '');
            }
            function show(node, text) {
                node.hidden = !text;
                node.textContent = text || '';
            }
            function showPair(node, he, en) {
                node.hidden = !(he || en);
                if (!he && !en) {
                    node.textContent = '';
                    return;
                }
                Ui.fillPair(node, he, en);
            }
            function submit() {
                if (creating) return Promise.resolve();
                remember();
                show(message, '');
                var problems = checkNew(typed, limits);
                if (problems.title) {
                    var code = problems.title + ' — ';
                    showPair(
                        titleMsg,
                        code + (problems.title === 'empty_title' ? 'צריך כותרת.' : 'הכותרת ארוכה מדי.'),
                        code + (problems.title === 'empty_title' ? 'A title is needed.' : 'The title is too long.')
                    );
                    return Promise.resolve();
                }
                showPair(titleMsg, '', '');
                var body = { title: typed.title.trim() };
                var assignee = typed.forUser.trim();
                creating = true;
                create.disabled = true;
                var opts = signal ? { signal: signal } : undefined;
                return Promise.resolve(Api.post('/api/plans', body, opts)).then(function (res) {
                    if (!alive) return;
                    if (!(res && res.ok && res.data && res.data.plan && res.data.plan.id)) {
                        creating = false;
                        create.disabled = false;
                        show(message, Ui.failureText(res));
                        return;
                    }
                    var planId = res.data.plan.id;
                    var revision = Number.isInteger(res.data.revision) ? res.data.revision : 1;
                    if (!assignee) {
                        typed = { title: '', forUser: '' };
                        creating = false;
                        create.disabled = false;
                        go(planId);
                        return;
                    }
                    return Promise.resolve(Api.post(
                        '/api/plans/' + encodeURIComponent(planId) + '/assign',
                        { audience: 'personal', assigneeId: assignee, expectedRevision: revision },
                        opts
                    )).then(function (assignRes) {
                        if (!alive) return;
                        creating = false;
                        create.disabled = false;
                        if (assignRes && assignRes.ok) {
                            typed = { title: '', forUser: '' };
                            go(planId);
                            return;
                        }
                        show(message, Ui.failureText(assignRes, { detail: true }));
                        go(planId);
                    });
                }).catch(function (err) {
                    if (!alive) return;
                    creating = false;
                    create.disabled = false;
                    show(message, Ui.failureText({ status: 0, data: null, error: err && err.message }));
                });
            }
            state.submit = submit;
            state.remember = remember;
            return { card: card };
        }

        var state = { submit: null, remember: null };

        function openForm(open) {
            newOpen = !!open;
            newButton.setAttribute('aria-expanded', newOpen ? 'true' : 'false');
            if (newOpen) {
                if (!formHost.firstChild) buildForm();
            } else {
                if (state.remember) state.remember();
                Ui.clear(formHost);
            }
        }

        // ---- events ----

        function onClick(ev) {
            var node = ev && ev.target;
            if (node && node.nodeType === 3) node = node.parentNode;
            if (Ui.closestAttr(node, 'data-plan-create', panel)) {
                if (ev.preventDefault) ev.preventDefault();
                if (state.submit) state.submit();
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-new-open', panel)) {
                openForm(!newOpen);
                return;
            }
            if (Ui.closestAttr(node, 'data-plan-list-refresh', panel) || Ui.closestAttr(node, 'data-plan-list-retry', panel)) {
                load();
                return;
            }
            var link = Ui.closestAttr(node, 'data-plan-open-link', panel);
            if (link) return; // a real link: the browser changes the hash
            var tr = Ui.closestAttr(node, 'data-plan-row', panel);
            if (tr && tr.getAttribute('data-plan-row')) go(tr.getAttribute('data-plan-row'));
        }
        function onSubmit(ev) {
            if (ev && ev.preventDefault) ev.preventDefault();
            if (state.submit) state.submit();
        }
        function applyFilters() {
            filterText = String(search.value || '').trim().toLowerCase();
            filterStatus = String(statusFilter.value || 'all');
            filterAudience = String(audienceFilter.value || 'all');
            if (!listed.length) return;
            renderTable(visibleRows());
        }
        removers.push(Ui.listen(panel, 'click', onClick, signal));
        removers.push(Ui.listen(panel, 'submit', onSubmit, signal));
        removers.push(Ui.listen(search, 'input', applyFilters, signal));
        removers.push(Ui.listen(statusFilter, 'change', applyFilters, signal));
        removers.push(Ui.listen(audienceFilter, 'change', applyFilters, signal));

        // the schema first, so the form has its title limit when it opens
        var schemaDone = Promise.resolve(Api.get('/api/plan-schema', signal ? { signal: signal } : undefined)).then(function (res) {
            if (!alive || !res || !res.ok || !isObject(res.data) || !isObject(res.data.definitions)) return;
            limits = limitsFromSchema(res.data);
            if (newOpen) {
                if (state.remember) state.remember();
                Ui.clear(formHost);
                buildForm();
            }
        }).catch(function () { /* the fallbacks stay */ });
        schemaDone.then(function () { if (alive) return load(); return undefined; });
        loadAccounts();

        return function cleanup() {
            alive = false;
            if (controller) controller.abort();
            removers.forEach(function (remove) { remove(); });
            removers = [];
        };
    }

    // ---- the assign control ---------------------------------------------------------------------

    /**
     * The control that changes who a plan is for. It is the only place the UI sends `audience` or
     * `assigneeId`. Accounts come from GET /api/admin/users; free text is not accepted.
     * @param {{ doc?: Document, api?: object, planId: string, signal?: AbortSignal|null,
     *   getRevision: function(): number|null, onAssigned: function(object): void,
     *   onConflict: function(object): void }} options
     * @returns {{ el: HTMLElement, update: function(object): void, load: function(): Promise, destroy: function(): void }}
     */
    function createAssign(options) {
        var opts = options || {};
        var doc = opts.doc || root.document;
        var Api = opts.api || root.HebrewApi;
        var signal = opts.signal || null;
        var make = function (tag, cls, text) { return Ui.make(doc, tag, cls, text); };
        var removers = [];
        var alive = true;
        var users = null;
        var usersFailed = false;
        var current = { audience: 'shared', assigneeId: '' };
        var shownKey = null;
        var busy = false;

        var el = make('section', 'card plan-assign');
        el.setAttribute('data-plan-assign', '');
        var heading = make('h3', '', 'שיוך (Assignment)');
        el.appendChild(heading);
        var summary = make('p', 'plan-note ui-pair');
        summary.setAttribute('data-plan-assign-summary', '');
        el.appendChild(summary);

        var row = make('div', 'plan-assign-row');
        var audienceSelect = make('select');
        audienceSelect.setAttribute('data-plan-assign-audience', '');
        ASSIGN_AUDIENCES.forEach(function (value) {
            var o = make('option', '', value === 'shared'
                ? 'משותפת — לכל הלומדים (Shared — every learner)'
                : 'אישית — לחשבון אחד (Personal — one account)');
            o.value = value;
            audienceSelect.appendChild(o);
        });
        var audienceField = make('div', 'plan-field');
        var audienceLabel = make('label', 'plan-label');
        audienceLabel.setAttribute('for', 'plan-assign-audience');
        audienceLabel.appendChild(Ui.bilingual(doc, 'span', '', 'קהל', 'Audience'));
        audienceSelect.setAttribute('id', 'plan-assign-audience');
        audienceField.appendChild(audienceLabel);
        audienceField.appendChild(audienceSelect);
        row.appendChild(audienceField);

        var userSelect = make('select');
        userSelect.setAttribute('data-plan-assign-user', '');
        var userField = make('div', 'plan-field');
        var userLabel = make('label', 'plan-label');
        userLabel.setAttribute('for', 'plan-assign-user');
        userLabel.appendChild(Ui.bilingual(doc, 'span', '', 'חשבון', 'Account'));
        userSelect.setAttribute('id', 'plan-assign-user');
        userField.appendChild(userLabel);
        userField.appendChild(userSelect);
        userField.hidden = true;
        row.appendChild(userField);
        el.appendChild(row);

        var save = Ui.button(doc, 'שמירת שיוך (Save assignment)', 'btn', { 'data-plan-assign-save': '' });
        el.appendChild(save);
        var message = make('p', 'admin-inline-error');
        message.setAttribute('data-plan-assign-message', '');
        message.setAttribute('role', 'alert');
        message.hidden = true;
        el.appendChild(message);

        function showMessage(text) {
            message.hidden = !text;
            message.textContent = text || '';
        }

        function fillUsers() {
            Ui.clear(userSelect);
            var pick = make('option', '', 'בחירת חשבון (Choose an account)');
            pick.value = '';
            userSelect.appendChild(pick);
            var list = activeUsers(users);
            var seen = {};
            list.forEach(function (u) {
                seen[u.username] = true;
                var text = u.display_name ? u.username + ' — ' + u.display_name : u.username;
                var o = make('option', '', text);
                o.value = u.username;
                userSelect.appendChild(o);
            });
            // The current assignee stays in the list even if the account is no longer active, so the
            // select shows the truth; choosing it again is refused by the server in plain words.
            if (current.assigneeId && !seen[current.assigneeId]) {
                var o2 = make('option', '', current.assigneeId + ' — לא פעיל (not active)');
                o2.value = current.assigneeId;
                userSelect.appendChild(o2);
            }
            userSelect.value = current.assigneeId || '';
        }

        function syncVisibility() {
            userField.hidden = audienceSelect.value !== 'personal';
        }

        function paintSummary() {
            Ui.clear(summary);
            var he = make('span', 'ui-he');
            he.setAttribute('lang', 'he');
            var en = make('span', 'ui-en');
            en.setAttribute('lang', 'en');
            en.setAttribute('dir', 'ltr');
            if (current.audience === 'personal') {
                var who = current.assigneeId || '—';
                he.textContent = 'אישית עבור ' + who;
                en.textContent = 'Personal, for ' + who;
            } else {
                he.textContent = 'משותפת לכל הלומדים';
                en.textContent = 'Shared with every learner';
            }
            summary.appendChild(he);
            summary.appendChild(en);
        }

        /** Take the plan's current audience and assignee (after a load, a save, or a restore). */
        function update(plan) {
            var audience = plan && plan.audience === 'personal' ? 'personal' : 'shared';
            var assignee = plan && typeof plan.assigneeId === 'string' ? plan.assigneeId : '';
            var key = audience + '|' + assignee + '|' + (plan && plan.revision);
            current = { audience: audience, assigneeId: assignee };
            paintSummary();
            if (key === shownKey) return;
            shownKey = key;
            audienceSelect.value = audience;
            fillUsers();
            syncVisibility();
        }

        function load() {
            if (users !== null || !Api) return Promise.resolve();
            return Promise.resolve(Api.get('/api/admin/users', signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive) return;
                if (!res || !res.ok || !res.data || !Array.isArray(res.data.users)) {
                    usersFailed = true;
                    users = [];
                    showMessage(Ui.line(
                        'לא ניתן לטעון את החשבונות — ' + Ui.failureText(res),
                        'Could not load the accounts — ' + Ui.failureText(res)
                    ));
                } else {
                    users = res.data.users;
                }
                fillUsers();
            }).catch(function () {
                if (!alive) return;
                usersFailed = true;
                users = [];
                fillUsers();
            });
        }

        function submit() {
            if (busy) return Promise.resolve(false);
            var audience = String(audienceSelect.value);
            var assignee = String(userSelect.value || '');
            if (audience === 'personal' && !assignee) {
                showMessage(Ui.line('בחרו חשבון מהרשימה.', 'Choose an account from the list.'));
                return Promise.resolve(false);
            }
            var revision = opts.getRevision ? opts.getRevision() : null;
            if (!Number.isInteger(revision)) return Promise.resolve(false);
            var body = { audience: audience, expectedRevision: revision };
            if (audience === 'personal') body.assigneeId = assignee;
            busy = true;
            save.disabled = true;
            showMessage('');
            return Promise.resolve(Api.post('/api/plans/' + encodeURIComponent(opts.planId) + '/assign', body,
                signal ? { signal: signal } : undefined)).then(function (res) {
                if (!alive) return false;
                busy = false;
                save.disabled = false;
                if (res && res.status === 409) {
                    showMessage(Ui.failureText(res));
                    if (opts.onConflict) opts.onConflict(res.data || {});
                    return false;
                }
                if (res && res.ok && res.data && Number.isInteger(res.data.revision)) {
                    shownKey = null;
                    if (opts.onAssigned) opts.onAssigned(res.data);
                    return true;
                }
                // detail: a short username is refused by the schema's assignee pattern, and the control
                // is the only place that can say so.
                showMessage(Ui.failureText(res, { detail: true }));
                return false;
            }).catch(function (err) {
                if (!alive) return false;
                busy = false;
                save.disabled = false;
                showMessage(Ui.failureText({ status: 0, data: null, error: err && err.message }));
                return false;
            });
        }

        function onClick(ev) {
            var node = ev && ev.target;
            if (node && node.nodeType === 3) node = node.parentNode;
            if (Ui.closestAttr(node, 'data-plan-assign-save', el)) submit();
        }
        function onChange(ev) {
            var node = ev && ev.target;
            if (node && Ui.closestAttr(node, 'data-plan-assign-audience', el)) {
                syncVisibility();
                showMessage('');
            }
        }
        removers.push(Ui.listen(el, 'click', onClick, signal));
        removers.push(Ui.listen(el, 'change', onChange, signal));

        return {
            el: el,
            update: update,
            load: load,
            submit: submit,
            usersFailed: function () { return usersFailed; },
            destroy: function () {
                alive = false;
                removers.forEach(function (remove) { remove(); });
                removers = [];
            }
        };
    }

    return {
        SLUG_FALLBACK: SLUG_FALLBACK,
        activeUsers: activeUsers,
        audienceOf: audienceOf,
        checkNew: checkNew,
        createAssign: createAssign,
        limitsFromSchema: limitsFromSchema,
        mount: mount,
        recipeLabel: recipeLabel,
        rowsOf: rowsOf,
        statusOf: statusOf
    };
});
