/**
 * The Plans tab of the admin (INIT-008/SPEC-013): a table of every plan, a "New plan" form, and
 * the assign control that the plan view shows. It is a client of the plan routes and holds no
 * plan logic:
 *   list     GET  /api/plans                        -> { plans: [...], truncated }
 *   create   POST /api/plans   { id, title, recipe? } -> { plan, revision, status, validation }
 *   assign   POST /api/plans/:id/assign { audience, assigneeId?, expectedRevision }
 *   accounts GET  /api/admin/users                  (the same read the Users tab uses)
 *   schema   GET  /api/plan-schema                  (the id pattern and the recipe list come from it)
 * The id pattern and the recipe options are read from the schema, never copied here; the server
 * checks every value again. A failed call shows the server's code in plain words and keeps what
 * the person typed. Server text goes in through textContent only.
 *
 *   var stop = HebrewPlanList.mount(panel, { api, setHash, signal })   // returns a cleanup function
 *   var assign = HebrewPlanList.createAssign({ doc, api, planId, signal, getRevision, onAssigned, onConflict })
 *
 * Plain browser script, no build step. Node tests load it with require().
 * Provenance: INIT-008/SPEC-013
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
     * What the New plan form checks before it asks the server. The pattern and the title limit
     * come from the schema when it loaded; the server decides in any case.
     * @param {{ id?: string, title?: string }} input
     * @param {{ slug?: RegExp, titleMax?: number }} [limits]
     * @returns {{ id?: string, title?: string }} a code per bad field; empty when both are fine
     */
    function checkNew(input, limits) {
        var slug = (limits && limits.slug) || SLUG_FALLBACK;
        var max = (limits && limits.titleMax) || TITLE_FALLBACK;
        var id = String(input && input.id != null ? input.id : '').trim();
        var title = String(input && input.title != null ? input.title : '').trim();
        var out = {};
        if (!slug.test(id) || id.length > 64) out.id = 'invalid_id';
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
        out.recipes = Form.enumOf('lesson', 'recipe', schema);
        return out;
    }

    /** Hebrew over English labels for a recipe, from the editor's hints. */
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
        var typed = { id: '', title: '', recipe: '' };

        function go(planId) {
            if (typeof opts.setHash === 'function') opts.setHash('plan/' + planId);
            else if (root.HebrewAdminShell && typeof root.HebrewAdminShell.setHash === 'function') {
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

        var heading = make('h2', '', 'תוכניות (Plans)');
        box.appendChild(heading);
        var intro = make('p', 'plan-help');
        var introHe = make('span', 'plan-help-he', 'כל תוכניות השיעור, בין שה-AI בנה אותן ובין שאדם. פתחו תוכנית כדי לערוך אותה.');
        introHe.setAttribute('lang', 'he');
        var introEn = make('span', 'plan-help-en', 'Every lesson plan, whether the AI or a person made it. Open one to edit it.');
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
                    td.setAttribute('data-label', Ui.label(COLUMNS[index][1], COLUMNS[index][2]));
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
            errorText.textContent = 'לא ניתן לטעון את התוכניות — ' + Ui.failureText(res) + ' (Could not load the plans.)';
        }

        function load() {
            status.textContent = 'טוען תוכניות… (Loading the plans…)';
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
                var rows = rowsOf(res.data);
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
                status.textContent = rows.length + ' תוכניות (' + rows.length + ' plans)';
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
                var p = make('p', 'plan-help');
                var hHe = make('span', 'plan-help-he', help[0]);
                hHe.setAttribute('lang', 'he');
                var hEn = make('span', 'plan-help-en', help[1]);
                hEn.setAttribute('lang', 'en');
                hEn.setAttribute('dir', 'ltr');
                p.appendChild(hHe);
                p.appendChild(hEn);
                group.appendChild(p);
            }
            return group;
        }

        function buildForm() {
            Ui.clear(formHost);
            var card = make('form', 'plan-new');
            card.setAttribute('data-plan-new', '');
            card.setAttribute('novalidate', '');
            card.setAttribute('aria-label', 'תוכנית חדשה (New plan)');
            var h = make('h3', '', 'תוכנית חדשה (New plan)');
            card.appendChild(h);

            var idInput = make('input');
            idInput.type = 'text';
            idInput.setAttribute('data-plan-new-id', '');
            idInput.setAttribute('autocomplete', 'off');
            idInput.setAttribute('autocapitalize', 'none');
            idInput.setAttribute('spellcheck', 'false');
            idInput.setAttribute('dir', 'ltr');
            idInput.setAttribute('lang', 'en');
            idInput.value = typed.id;
            card.appendChild(field('plan-new-id', 'מזהה', 'Id', idInput,
                ['אותיות לועזיות קטנות, ספרות ומקפים. אי אפשר לשנות אחר כך.',
                    'Lowercase letters, digits and hyphens. It cannot be changed later.']));
            var idMsg = make('p', 'plan-issue plan-issue-block');
            idMsg.setAttribute('data-plan-new-id-message', '');
            idMsg.setAttribute('role', 'alert');
            idMsg.hidden = true;
            card.appendChild(idMsg);

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

            var recipeSelect = null;
            if (limits.recipes.length) {
                recipeSelect = make('select');
                recipeSelect.setAttribute('data-plan-new-recipe', '');
                var none = make('option', '', 'ללא מתכון (No recipe)');
                none.value = '';
                recipeSelect.appendChild(none);
                limits.recipes.forEach(function (recipe) {
                    var o = make('option', '', recipeLabel(recipe));
                    o.value = recipe;
                    recipeSelect.appendChild(o);
                });
                recipeSelect.value = typed.recipe;
                card.appendChild(field('plan-new-recipe', 'מתכון', 'Recipe', recipeSelect,
                    ['מתכון קובע צורת פתיחה בלבד ואינו כותב עברית.',
                        'A recipe sets a starting shape and writes no Hebrew.']));
            }

            var message = make('p', 'admin-inline-error');
            message.setAttribute('data-plan-new-message', '');
            message.setAttribute('role', 'alert');
            message.hidden = true;
            card.appendChild(message);
            var create = Ui.button(doc, 'יצירה (Create)', 'btn btn-primary', { 'data-plan-create': '' });
            card.appendChild(create);
            formHost.appendChild(card);

            function remember() {
                typed.id = String(idInput.value);
                typed.title = String(titleInput.value);
                typed.recipe = recipeSelect ? String(recipeSelect.value) : '';
            }
            function show(node, text) {
                node.hidden = !text;
                node.textContent = text || '';
            }
            function submit() {
                if (creating) return Promise.resolve();
                remember();
                show(message, '');
                var problems = checkNew(typed, limits);
                show(idMsg, problems.id ? problems.id + ' — ' + Ui.label(
                    'המזהה: אותיות לועזיות קטנות, ספרות ומקפים בלבד.',
                    'The id may hold lowercase letters, digits and hyphens only.') : '');
                show(titleMsg, problems.title ? problems.title + ' — ' + Ui.label(
                    problems.title === 'empty_title' ? 'צריך כותרת.' : 'הכותרת ארוכה מדי.',
                    problems.title === 'empty_title' ? 'A title is needed.' : 'The title is too long.') : '');
                if (problems.id || problems.title) return Promise.resolve();
                var body = { id: typed.id.trim(), title: typed.title.trim() };
                if (typed.recipe) body.recipe = typed.recipe;
                creating = true;
                create.disabled = true;
                return Promise.resolve(Api.post('/api/plans', body, signal ? { signal: signal } : undefined)).then(function (res) {
                    if (!alive) return;
                    creating = false;
                    create.disabled = false;
                    if (res && res.ok && res.data && res.data.plan) {
                        typed = { id: '', title: '', recipe: '' };
                        go(body.id);
                        return;
                    }
                    var f = Ui.failure(res);
                    if (f.code === 'invalid_id') show(idMsg, Ui.failureText(res));
                    else show(message, Ui.failureText(res));
                }).catch(function (err) {
                    if (!alive) return;
                    creating = false;
                    create.disabled = false;
                    show(message, Ui.failureText({ status: 0, data: null, error: err && err.message }));
                });
            }
            state.submit = submit;
            state.remember = remember;
            return { card: card, idInput: idInput };
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
        removers.push(Ui.listen(panel, 'click', onClick, signal));
        removers.push(Ui.listen(panel, 'submit', onSubmit, signal));

        // the schema first, so the form has its id pattern and recipe list when it opens
        var schemaDone = Promise.resolve(Api.get('/api/plan-schema', signal ? { signal: signal } : undefined)).then(function (res) {
            if (!alive || !res || !res.ok || !isObject(res.data) || !isObject(res.data.definitions)) return;
            limits = limitsFromSchema(res.data);
            if (newOpen) {
                // rebuild with the recipe list, keeping what was typed
                if (state.remember) state.remember();
                Ui.clear(formHost);
                buildForm();
            }
        }).catch(function () { /* the fallbacks stay */ });
        schemaDone.then(function () { if (alive) return load(); return undefined; });

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
        var summary = make('p', 'plan-note');
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
            summary.textContent = current.audience === 'personal'
                ? 'אישית עבור ' + (current.assigneeId || '—') + ' (Personal, for ' + (current.assigneeId || '—') + ')'
                : 'משותפת לכל הלומדים (Shared with every learner)';
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
                    showMessage('לא ניתן לטעון את החשבונות — ' + Ui.failureText(res) + ' (Could not load the accounts.)');
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
                showMessage('invalid_assignee — בחרו חשבון מהרשימה (Choose an account from the list.)');
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
