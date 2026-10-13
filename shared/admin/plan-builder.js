/**
 * Admin outline for one lesson plan: insert, move, remove, publish, and the forms
 * that edit any part of it. The forms are not written here: plan-form.js renders
 * an edit form for any plan node from the plan JSON Schema (GET /api/plan-schema),
 * with the widgets of plan-widgets.js and the hints of plan-hints.js, and sends
 * each save through the `transport` this file supplies. A new block is generated
 * from the schema's `required` list, so no block type is written out here.
 * Media upload, dry-run, confirmed generation, and approval are the SPEC-011
 * routes. Picture attach sends set_field. The validation panel prints the
 * save and publish findings. It does not recompute a cap.
 * Every change is an HTTP call. This file does not edit the plan JSON,
 * does not save a worksheet, and does not read an API key.
 * Listeners go on the panel and are removed on unmount (an AbortSignal where the
 * browser has one).
 *
 * The panels of INIT-008 are separate modules this file mounts and wires:
 *   plan-list.js     the Plans tab (mountPicker) and the assign control
 *   plan-history.js  revisions, compare, restore, and the "edited by a person" restore
 *   plan-issues.js   the issues panel, with a jump to the field and a one-click fix
 *   plan-preview.js  the learner's view of the unsaved text
 * Each is optional: without it the plan view is the editor alone.
 * Provenance: INIT-004/SPEC-015, INIT-004/SPEC-016, INIT-008/SPEC-011, INIT-008/SPEC-012,
 *             INIT-008/SPEC-013, INIT-008/SPEC-014, INIT-008/SPEC-015
 */
(function (root) {
    'use strict';

    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var ENTRY = /^[a-z0-9][a-z0-9-]*$/;
    var LESSON_PATH = /^units\/[a-z0-9]+(?:-[a-z0-9]+)*\/lessons\/[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var PART_PATH = /^units\/[a-z0-9]+(?:-[a-z0-9]+)*\/lessons\/[a-z0-9]+(?:-[a-z0-9]+)*\/blocks\/[a-z0-9][a-z0-9-]*(?:\/(?:words|paragraphs|lines|questions)\/[a-z0-9][a-z0-9-]*)?$/;
    var MEDIA_ID = /^[a-z0-9][a-z0-9-]{0,63}$/;

    var token = 0;
    var state = null;
    var inflight = Promise.resolve();
    var pickerCleanup = null;

    function api() {
        return root.HebrewApi;
    }

    function el(tag) {
        return root.document.createElement(tag);
    }

    function empty(node) {
        while (node.firstChild) node.removeChild(node.firstChild);
    }

    function alive(mine) {
        return state && state.token === mine;
    }

    function track(promise) {
        inflight = Promise.resolve(promise);
        return inflight;
    }

    /**
     * Index the move route stores after it has taken the block out of the list.
     * Moving down one slot is `fromIndex + 1` in that shorter list. Block ids stay put.
     * @param {number} fromIndex
     * @param {'up'|'down'} direction
     * @returns {number|null}
     */
    function moveDestination(fromIndex, direction) {
        if (!Number.isInteger(fromIndex) || fromIndex < 0) return null;
        if (direction === 'up') return fromIndex - 1;
        if (direction === 'down') return fromIndex + 1;
        return null;
    }

    function form() {
        return root.HebrewPlanForm;
    }

    /**
     * A new block, from the schema's `required` list (plan-form.js). Does not copy or
     * renumber ids already in the lesson, and writes no Hebrew.
     * @param {string} type
     * @param {string} id
     * @returns {object|null}
     */
    function newBlock(type, id) {
        if (!ENTRY.test(id) || !form()) return null;
        return form().newBlock(type, id);
    }

    function partsUrl(partPath) {
        var url = '/api/plans/' + encodeURIComponent(state.planId) + '/parts';
        if (partPath) url += '?path=' + encodeURIComponent(partPath);
        return url;
    }

    function listLessons(plan) {
        var out = [];
        var units = plan && Array.isArray(plan.units) ? plan.units : [];
        var u;
        var i;
        for (u = 0; u < units.length; u++) {
            var unit = units[u];
            if (!unit || typeof unit.id !== 'string' || !SLUG.test(unit.id)) continue;
            var lessons = Array.isArray(unit.lessons) ? unit.lessons : [];
            for (i = 0; i < lessons.length; i++) {
                var lesson = lessons[i];
                if (!lesson || typeof lesson.id !== 'string' || !SLUG.test(lesson.id)) continue;
                out.push({
                    path: 'units/' + unit.id + '/lessons/' + lesson.id,
                    title: typeof lesson.title === 'string' ? lesson.title : lesson.id,
                    id: lesson.id
                });
            }
        }
        return out;
    }

    function blocksOf(lesson) {
        return lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
    }

    function blockById(id) {
        var blocks = blocksOf(state && state.lesson);
        var i;
        for (i = 0; i < blocks.length; i++) {
            if (blocks[i] && blocks[i].id === id) return { block: blocks[i], index: i };
        }
        return null;
    }

    function labeled(caption, control) {
        var label = el('label');
        var name = el('span');
        name.textContent = caption;
        label.appendChild(name);
        label.appendChild(control);
        return label;
    }

    function button(attr, value, text, locked) {
        var node = el('button');
        node.type = 'button';
        node.className = 'btn';
        node.setAttribute(attr, value);
        node.textContent = text;
        if (locked) {
            node.disabled = true;
            node.setAttribute('data-plan-locked', '');
        }
        return node;
    }

    function messageNode(attr) {
        var node = el('p');
        node.className = 'admin-inline-error';
        node.setAttribute(attr, '');
        node.setAttribute('role', 'alert');
        node.hidden = true;
        return node;
    }

    function showBox(attr, text) {
        if (!state || !state.panel) return;
        var node = state.panel.querySelector('[' + attr + ']');
        if (!node) return;
        node.hidden = false;
        node.textContent = text;
    }

    function hideBox(attr) {
        if (!state || !state.panel) return;
        var node = state.panel.querySelector('[' + attr + ']');
        if (!node) return;
        node.hidden = true;
        node.textContent = '';
    }

    function errorText(res) {
        if (res && typeof res.error === 'string' && res.error) {
            return typeof res.code === 'string' && res.code ? res.error + ' ' + res.code : res.error;
        }
        if (res && res.data && typeof res.data.error === 'string') return res.data.error;
        return 'request failed';
    }

    /**
     * Findings the save and publish routes already computed.
     * A 400 validation body carries the arrays at the top. A 200 carries them
     * on `validation`. Missing arrays are not a client-side cap check.
     * @param {object|null|undefined} data
     * @returns {{ blocks: object[], warnings: object[], informs: object[] }|null}
     */
    function findingsFrom(data) {
        if (!data || typeof data !== 'object') return null;
        var nested = data.validation;
        var source = nested && typeof nested === 'object' && !Array.isArray(nested) ? nested : data;
        var blocks = Array.isArray(source.blocks) ? source.blocks : null;
        var warnings = Array.isArray(source.warnings) ? source.warnings : null;
        var informs = Array.isArray(source.informs) ? source.informs : null;
        if (!blocks && !warnings && !informs) return null;
        return {
            blocks: blocks || [],
            warnings: warnings || [],
            informs: informs || []
        };
    }

    /**
     * @param {string} id
     * @param {string} status
     * @param {string} altText
     * @param {object[]} rows
     * @param {Record<string, boolean>} seen
     */
    function addLibraryRow(id, status, altText, rows, seen) {
        if (typeof id !== 'string' || !MEDIA_ID.test(id) || seen[id]) return;
        seen[id] = true;
        rows.push({
            id: id,
            status: status === 'pending' || status === 'approved' ? status : '',
            altText: typeof altText === 'string' ? altText : ''
        });
    }

    /**
     * Session uploads plus pictures already on introduction words.
     * There is no list route. Status is only what a media response returned.
     * @returns {{ id: string, status: string, altText: string }[]}
     */
    function libraryRows() {
        var rows = [];
        var seen = Object.create(null);
        var i;
        var stored = state && Array.isArray(state.media) ? state.media : [];
        for (i = 0; i < stored.length; i++) {
            var item = stored[i];
            if (!item) continue;
            addLibraryRow(item.id, item.status, item.altText, rows, seen);
        }
        var blocks = blocksOf(state && state.lesson);
        for (i = 0; i < blocks.length; i++) {
            var block = blocks[i];
            if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) continue;
            var w;
            for (w = 0; w < block.words.length; w++) {
                var image = block.words[w] && block.words[w].image;
                if (!image || typeof image !== 'object' || Array.isArray(image)) continue;
                addLibraryRow(image.mediaId, '', image.alt, rows, seen);
            }
        }
        return rows;
    }

    /**
     * @param {object|null|undefined} row
     */
    function rememberMedia(row) {
        if (!state || !row || typeof row.id !== 'string' || !MEDIA_ID.test(row.id)) return;
        var status = row.status === 'pending' || row.status === 'approved' ? row.status : '';
        var altText = typeof row.altText === 'string' ? row.altText : '';
        var i;
        for (i = 0; i < state.media.length; i++) {
            if (state.media[i] && state.media[i].id === row.id) {
                state.media[i] = {
                    id: row.id,
                    status: status || state.media[i].status,
                    altText: altText || state.media[i].altText
                };
                return;
            }
        }
        state.media.push({ id: row.id, status: status, altText: altText });
    }

    function syncGenerate() {
        if (!state || !state.panel) return;
        var btn = state.panel.querySelector('[data-plan-generate]');
        var box = state.panel.querySelector('[data-plan-confirm]');
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        if (!btn) return;
        var prompt = promptInput ? String(promptInput.value || '') : '';
        var ready = state.estimateShown === true
            && typeof state.estimateDollars === 'number'
            && Number.isFinite(state.estimateDollars)
            && !!box
            && box.checked === true
            && prompt === state.estimatePrompt;
        btn.disabled = !ready;
    }

    function syncApprove() {
        if (!state || !state.panel) return;
        var btn = state.panel.querySelector('[data-plan-approve]');
        var altInput = state.panel.querySelector('[data-plan-approve-alt]');
        if (!btn) return;
        var alt = altInput ? String(altInput.value || '').trim() : '';
        btn.disabled = alt.length === 0;
    }

    function showError(res) {
        showBox('data-plan-error', errorText(res));
    }

    /**
     * Name the conflict and the revision the server stored. Leave every textarea alone.
     * @param {object} data
     */
    function showConflict(data) {
        var name = data && typeof data.error === 'string' ? data.error : 'revision_conflict';
        var revision = data && Object.prototype.hasOwnProperty.call(data, 'revision') ? data.revision : '';
        hideBox('data-plan-error');
        showBox('data-plan-conflict', name + ' ' + String(revision));
    }

    function paintMessage(text) {
        empty(state.panel);
        var box = el('div');
        box.setAttribute('data-plan-builder', '');
        box.setAttribute('data-admin-view', 'plan');
        box.setAttribute('dir', 'rtl');
        box.setAttribute('lang', 'he');
        var p = messageNode('data-plan-error');
        p.hidden = false;
        p.textContent = text;
        box.appendChild(p);
        state.panel.appendChild(box);
    }

    /**
     * Picture choice exists only on an introduction word.
     * @param {HTMLElement} parent
     * @param {object} block
     */
    function paintPictures(parent, block) {
        if (!block || block.type !== 'introduction' || !Array.isArray(block.words)) return;
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-pictures', '');
        var heading = el('h3');
        heading.textContent = 'תמונה (Picture)';
        box.appendChild(heading);
        var rows = libraryRows();
        var w;
        for (w = 0; w < block.words.length; w++) {
            var word = block.words[w];
            if (!word || typeof word.id !== 'string' || !ENTRY.test(word.id)) continue;
            var row = el('div');
            row.setAttribute('data-plan-word', word.id);
            var name = el('span');
            name.textContent = typeof word.word === 'string' ? word.word : word.id;
            row.appendChild(name);
            var select = el('select');
            select.setAttribute('data-plan-attach-media', '');
            var current = word.image && typeof word.image.mediaId === 'string' ? word.image.mediaId : '';
            var choices = rows.slice();
            var have = false;
            var c;
            for (c = 0; c < choices.length; c++) {
                if (choices[c].id === current) have = true;
            }
            if (current && MEDIA_ID.test(current) && !have) {
                choices.unshift({ id: current, status: '', altText: '' });
            }
            var emptyOpt = el('option');
            emptyOpt.value = '';
            emptyOpt.textContent = '—';
            select.appendChild(emptyOpt);
            var o;
            for (o = 0; o < choices.length; o++) {
                var opt = el('option');
                opt.value = choices[o].id;
                opt.textContent = choices[o].id;
                select.appendChild(opt);
            }
            select.value = current && MEDIA_ID.test(current) ? current : '';
            var alt = el('input');
            alt.type = 'text';
            alt.setAttribute('data-plan-attach-alt', '');
            alt.value = word.image && typeof word.image.alt === 'string' ? word.image.alt : '';
            row.appendChild(labeled('מזהה תמונה (Picture)', select));
            row.appendChild(labeled('טקסט חלופי (Alt)', alt));
            row.appendChild(button('data-plan-attach', word.id, 'שמירת תמונה (Save picture)', false));
            box.appendChild(row);
        }
        parent.appendChild(box);
    }

    /**
     * Pending and approved pictures, upload, dry-run, and approval.
     * Generate stays disabled until the dry-run number is on screen and confirmed.
     * @param {HTMLElement} parent
     */
    function paintMedia(parent) {
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-media', '');
        var heading = el('h3');
        heading.textContent = 'ספריית תמונות (Media)';
        box.appendChild(heading);

        var list = el('ul');
        list.setAttribute('data-plan-media-list', '');
        var rows = libraryRows();
        var i;
        for (i = 0; i < rows.length; i++) {
            var item = el('li');
            item.setAttribute('data-plan-media-item', rows[i].id);
            var label = el('span');
            label.textContent = rows[i].id;
            item.appendChild(label);
            if (rows[i].status) {
                var status = el('span');
                status.setAttribute('data-plan-media-status', rows[i].status);
                status.textContent = rows[i].status;
                item.appendChild(status);
            }
            var img = el('img');
            img.alt = rows[i].altText;
            img.setAttribute('src', '/api/media/' + rows[i].id);
            item.appendChild(img);
            list.appendChild(item);
        }
        box.appendChild(list);

        var uploadId = el('input');
        uploadId.type = 'text';
        uploadId.setAttribute('data-plan-upload-id', '');
        uploadId.setAttribute('autocomplete', 'off');
        var uploadAlt = el('input');
        uploadAlt.type = 'text';
        uploadAlt.setAttribute('data-plan-upload-alt', '');
        var file = el('input');
        file.type = 'file';
        file.setAttribute('data-plan-upload-file', '');
        file.setAttribute('accept', 'image/png,image/jpeg,image/webp');
        box.appendChild(labeled('מזהה העלאה (Upload id)', uploadId));
        box.appendChild(labeled('טקסט חלופי (Alt)', uploadAlt));
        box.appendChild(labeled('קובץ (File)', file));
        box.appendChild(button('data-plan-upload', '', 'העלאה (Upload)', false));

        var prompt = el('textarea');
        prompt.setAttribute('data-plan-gen-prompt', '');
        prompt.setAttribute('dir', 'ltr');
        prompt.setAttribute('lang', 'en');
        prompt.rows = 3;
        prompt.value = typeof state.estimatePrompt === 'string' ? state.estimatePrompt : '';
        var genId = el('input');
        genId.type = 'text';
        genId.setAttribute('data-plan-gen-id', '');
        genId.setAttribute('autocomplete', 'off');
        box.appendChild(labeled('בקשה (Prompt)', prompt));
        box.appendChild(labeled('מזהה יצירה (Generate id)', genId));
        box.appendChild(button('data-plan-dry-run', '', 'הערכה (Estimate)', false));

        var estimate = el('p');
        estimate.setAttribute('data-plan-estimate', '');
        if (state.estimateShown && typeof state.estimateDollars === 'number') {
            estimate.hidden = false;
            estimate.textContent = String(state.estimateDollars);
        } else {
            estimate.hidden = true;
            estimate.textContent = '';
        }
        box.appendChild(estimate);

        var confirm = el('input');
        confirm.type = 'checkbox';
        confirm.checked = false;
        confirm.setAttribute('data-plan-confirm', '');
        box.appendChild(labeled('אישור הסכום (Confirm estimate)', confirm));
        box.appendChild(button('data-plan-generate', '', 'יצירה (Generate)', true));

        var approveId = el('input');
        approveId.type = 'text';
        approveId.setAttribute('data-plan-approve-id', '');
        approveId.setAttribute('autocomplete', 'off');
        var approveAlt = el('input');
        approveAlt.type = 'text';
        approveAlt.setAttribute('data-plan-approve-alt', '');
        box.appendChild(labeled('מזהה לאישור (Approve id)', approveId));
        box.appendChild(labeled('טקסט חלופי לאישור (Approval alt)', approveAlt));
        box.appendChild(button('data-plan-approve', '', 'אישור (Approve)', true));

        parent.appendChild(box);
    }

    /**
     * The validation panel lives in a stable host so a failed save can refresh it
     * without repainting the page (a repaint would drop the issues shown at the fields).
     * @param {HTMLElement} parent
     */
    function paintValidation(parent) {
        var host = el('div');
        host.setAttribute('data-plan-validation-host', '');
        state.validationHost = host;
        fillValidation(host);
        parent.appendChild(host);
    }

    function refreshValidation() {
        if (state && state.validationHost) fillValidation(state.validationHost);
    }

    /**
     * @param {HTMLElement} host
     */
    function fillValidation(host) {
        empty(host);
        var box = el('div');
        box.className = 'card';
        box.setAttribute('data-plan-validation', '');
        var heading = el('h3');
        heading.textContent = 'בדיקה (Validation)';
        box.appendChild(heading);
        if (state.publishRefused) {
            var refused = el('p');
            refused.setAttribute('data-plan-publish-refusal', '');
            refused.setAttribute('role', 'status');
            var code = typeof state.publishRefusal === 'string' ? state.publishRefusal : '';
            refused.textContent = 'snapshot was not published: ' + code;
            box.appendChild(refused);
        }
        var groups = [
            ['block', state.validation && state.validation.blocks],
            ['warn', state.validation && state.validation.warnings],
            ['inform', state.validation && state.validation.informs]
        ];
        var g;
        for (g = 0; g < groups.length; g++) {
            var severity = groups[g][0];
            var items = groups[g][1] || [];
            var list = el('ul');
            list.setAttribute('data-plan-findings', severity);
            var n;
            for (n = 0; n < items.length; n++) {
                var finding = items[n];
                if (!finding || typeof finding !== 'object') continue;
                var rule = typeof finding.rule === 'string' ? finding.rule : '';
                if (!rule) continue;
                var li = el('li');
                li.setAttribute('data-plan-finding', rule);
                var message = typeof finding.message === 'string' ? finding.message : '';
                li.textContent = rule + ' ' + severity + ' ' + message;
                list.appendChild(li);
            }
            box.appendChild(list);
        }
        host.appendChild(box);
    }

    /**
     * Findings from the last save or publish, flattened with their severity, for the
     * inline issues at the fields.
     * @returns {object[]}
     */
    function validationList() {
        var v = state && state.validation;
        if (!v) return [];
        var out = [];
        [['block', v.blocks], ['warn', v.warnings], ['inform', v.informs]].forEach(function (group) {
            (group[1] || []).forEach(function (finding) {
                if (!finding || typeof finding !== 'object') return;
                var copy = {};
                Object.keys(finding).forEach(function (key) { copy[key] = finding[key]; });
                copy.severity = copy.severity || group[0];
                out.push(copy);
            });
        });
        return out;
    }

    function blockLabel(type) {
        var hints = root.HebrewPlanHints;
        var hint = hints && hints.blocks && hints.blocks[type];
        return hint ? hint.label[0] + ' (' + hint.label[1] + ')' : String(type || '');
    }

    /**
     * A section the admin opens and closes. The body is built only while it is open,
     * so a closed plan or unit form costs nothing; unsaved text in a closed form is
     * kept as a draft by the editor.
     */
    function disclosure(name, he, en, build) {
        var open = state.open[name] === true;
        var card = el('section');
        card.className = 'card';
        card.setAttribute('data-plan-section', name);
        var heading = el('h3');
        var toggle = button('data-plan-disclose', name, (open ? '▾ ' : '▸ ') + he + ' (' + en + ')', false);
        toggle.className = 'btn btn-ghost plan-disclose';
        toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
        toggle.setAttribute('data-plan-focus-key', 'disclose:' + name);
        heading.appendChild(toggle);
        card.appendChild(heading);
        if (open) {
            var body = build();
            if (body) card.appendChild(body);
        }
        return card;
    }

    function unitPathOf(lessonPath) {
        var segs = String(lessonPath || '').split('/');
        return segs.length >= 2 ? segs[0] + '/' + segs[1] : '';
    }

    function renderNewDraft(type) {
        var host = state.newDraftHost;
        if (!host) return;
        empty(host);
        state.draft = null;
        var tools = state.editor.tools();
        var def = tools ? tools.blockDef(type) : null;
        if (!def) return;
        var required = tools.requiredOf(tools.defSchema(def)).filter(function (prop) {
            return prop !== 'id' && prop !== 'type';
        });
        if (!required.length) return;
        var seed = form().newBlock(type, 'new');
        if (!seed) return;
        var handle = state.editor.renderDraft(def, seed, required);
        host.appendChild(handle.el);
        state.draft = { type: type, handle: handle };
    }

    function paintInsert(parent, blocks) {
        var insert = el('div');
        insert.className = 'card';
        insert.setAttribute('data-plan-newblock', '');
        var heading = el('h3');
        heading.textContent = 'בלוק חדש (New block)';
        insert.appendChild(heading);
        var idInput = el('input');
        idInput.type = 'text';
        idInput.setAttribute('data-plan-new-id', '');
        idInput.setAttribute('autocomplete', 'off');
        var typeSelect = el('select');
        typeSelect.setAttribute('data-plan-new-type', '');
        var types = form() ? form().blockTypeList() : [];
        var t;
        for (t = 0; t < types.length; t++) {
            var opt = el('option');
            opt.value = types[t].type;
            opt.textContent = types[t].he + ' (' + types[t].en + ')';
            typeSelect.appendChild(opt);
        }
        if (types.length) typeSelect.value = types[0].type;
        var indexInput = el('input');
        indexInput.type = 'text';
        indexInput.setAttribute('data-plan-new-index', '');
        indexInput.setAttribute('inputmode', 'numeric');
        indexInput.value = String(blocks.length);
        insert.appendChild(labeled('מזהה (Id)', idInput));
        insert.appendChild(labeled('סוג (Type)', typeSelect));
        insert.appendChild(labeled('מקום (Index)', indexInput));
        var note = el('p');
        note.className = 'plan-note';
        note.textContent = 'ממלאים את השדות הנדרשים; השרת אומר מה חסר. (Fill in the required fields; the server says what is missing.)';
        insert.appendChild(note);
        var draftHost = el('div');
        draftHost.setAttribute('data-plan-new-draft', '');
        state.newDraftHost = draftHost;
        insert.appendChild(draftHost);
        var issues = el('ul');
        issues.className = 'plan-issues';
        issues.setAttribute('data-plan-new-issues', '');
        insert.appendChild(issues);
        insert.appendChild(button('data-plan-insert', '', 'הוספה (Insert)', !state.lessonPath));
        parent.appendChild(insert);
        if (types.length && state.editor.hasSchema()) renderNewDraft(typeSelect.value);
    }

    function showNewIssues(data) {
        if (!state || !state.panel) return;
        var list = state.panel.querySelector('[data-plan-new-issues]');
        if (!list) return;
        empty(list);
        var all = [];
        [['block', data.blocks], ['warn', data.warnings], ['inform', data.informs]].forEach(function (group) {
            (Array.isArray(group[1]) ? group[1] : []).forEach(function (finding) {
                if (finding && typeof finding === 'object') all.push({ severity: group[0], finding: finding });
            });
        });
        all.forEach(function (item) {
            var li = el('li');
            li.className = 'plan-issue plan-issue-' + item.severity;
            li.setAttribute('data-plan-issue', typeof item.finding.rule === 'string' ? item.finding.rule : '');
            var where = typeof item.finding.path === 'string' ? item.finding.path.split('/').slice(-2).join('/') : '';
            li.textContent = item.severity + ' · ' + (item.finding.rule || '') + ' — '
                + (item.finding.message || '') + (where ? ' (' + where + ')' : '');
            list.appendChild(li);
        });
    }

    function paint() {
        var panel = state.panel;
        var editor = state.editor;
        var lesson = state.lesson;
        var blocks = blocksOf(lesson);
        var selected = state.selectedId ? blockById(state.selectedId) : null;
        if (state.selectedId && !selected) state.selectedId = null;

        // Keep unsaved text, the open sections, focus, and scroll across the repaint.
        var focus = editor.focusKey();
        editor.captureDrafts();
        editor.reset();
        var scrollY = typeof root.scrollY === 'number' ? root.scrollY : null;

        // The block form is made first, so its field buttons are numbered first.
        var ready = state.schema === 'ready';
        var blockForm = null;
        if (selected && ready && LESSON_PATH.test(state.lessonPath)) {
            blockForm = editor.renderNode({
                path: state.lessonPath + '/blocks/' + selected.block.id,
                node: selected.block,
                kind: 'block'
            });
        }
        var lessonCard = null;
        var unitCard = null;
        var planCard = null;
        if (ready && state.plan) {
            if (lesson && LESSON_PATH.test(state.lessonPath)) {
                lessonCard = disclosure('lesson', 'פרטי השיעור', 'Lesson details', function () {
                    return editor.renderNode({ path: state.lessonPath, node: lesson, kind: 'lesson' }).el;
                });
                unitCard = disclosure('unit', 'פרטי היחידה', 'Unit details', function () {
                    var path = unitPathOf(state.lessonPath);
                    var node = form().nodeAt(state.plan, path);
                    return node ? editor.renderNode({ path: path, node: node, kind: 'unit' }).el : null;
                });
            }
            planCard = disclosure('plan', 'פרטי התוכנית', 'Plan details', function () {
                return editor.renderNode({ path: '', node: state.plan, kind: 'plan' }).el;
            });
        }

        empty(panel);
        var rootEl = el('div');
        rootEl.className = 'plan-builder';
        rootEl.setAttribute('data-plan-builder', '');
        rootEl.setAttribute('data-admin-view', 'plan');
        rootEl.setAttribute('dir', 'rtl');
        rootEl.setAttribute('lang', 'he');

        var back = el('p');
        back.className = 'admin-back';
        var backLink = el('a');
        backLink.setAttribute('href', '#plans');
        backLink.textContent = '← תוכניות (Plans)';
        back.appendChild(backLink);
        rootEl.appendChild(back);

        var toolbar = el('div');
        toolbar.className = 'card';
        var title = el('h2');
        title.textContent = state.plan && typeof state.plan.title === 'string' ? state.plan.title : state.planId;
        toolbar.appendChild(title);
        var rev = el('p');
        rev.setAttribute('data-plan-revision', '');
        rev.textContent = Number.isInteger(state.revision) ? String(state.revision) : '';
        toolbar.appendChild(rev);
        var status = el('p');
        status.setAttribute('data-plan-status', '');
        status.textContent = state.plan && typeof state.plan.status === 'string' ? state.plan.status : '';
        toolbar.appendChild(status);
        if (state.panels.issues) {
            // What the publish will meet, and that warnings never stop it. No approval step.
            var publishNote = el('p');
            publishNote.className = 'plan-note';
            publishNote.setAttribute('data-plan-publish-note', '');
            publishNote.textContent = state.publishNote || '';
            toolbar.appendChild(publishNote);
        }
        toolbar.appendChild(button('data-plan-publish', '', 'פרסום (Publish)', false));
        toolbar.appendChild(button('data-plan-reload', '', 'טעינה מחדש (Reload)', false));
        var notice = el('p');
        notice.setAttribute('data-plan-notice', '');
        notice.setAttribute('role', 'status');
        notice.textContent = state.notice || '';
        toolbar.appendChild(notice);
        rootEl.appendChild(toolbar);

        rootEl.appendChild(messageNode('data-plan-conflict'));
        rootEl.appendChild(messageNode('data-plan-error'));

        if (state.panels.assign) {
            state.panels.assign.update(state.plan);
            rootEl.appendChild(state.panels.assign.el);
        }

        var chooser = el('div');
        chooser.className = 'card';
        chooser.setAttribute('data-plan-lessons', '');
        var c;
        for (c = 0; c < state.lessonChoices.length; c++) {
            var choice = state.lessonChoices[c];
            var pick = button('data-plan-lesson', String(c), choice.title + ' (' + choice.id + ')', false);
            if (choice.path === state.lessonPath) pick.setAttribute('aria-current', 'true');
            chooser.appendChild(pick);
        }
        rootEl.appendChild(chooser);

        // Two columns on a wide screen: the forms, and beside them the preview, issues, and history.
        var layout = el('div');
        layout.className = 'plan-layout';
        var mainCol = el('div');
        mainCol.className = 'plan-main';
        var aside = el('div');
        aside.className = 'plan-aside';
        layout.appendChild(mainCol);
        layout.appendChild(aside);
        rootEl.appendChild(layout);

        if (state.schema === 'loading') {
            var loading = el('p');
            loading.className = 'plan-note';
            loading.setAttribute('data-plan-schema-loading', '');
            loading.setAttribute('role', 'status');
            loading.textContent = 'טוען את הסכמה… (Loading the schema…)';
            mainCol.appendChild(loading);
        } else if (state.schema === 'failed') {
            var failed = el('p');
            failed.className = 'admin-inline-error';
            failed.setAttribute('data-plan-schema-failed', '');
            failed.setAttribute('role', 'alert');
            failed.textContent = 'הסכמה לא נטענה; אפשר לערוך רק כ-JSON. (The schema did not load; only raw JSON editing is available.)';
            mainCol.appendChild(failed);
        }
        if (lessonCard) mainCol.appendChild(lessonCard);
        if (unitCard) mainCol.appendChild(unitCard);
        if (planCard) mainCol.appendChild(planCard);

        var outlineHeading = el('h3');
        outlineHeading.textContent = 'בלוקים (Blocks)';
        mainCol.appendChild(outlineHeading);
        var outline = el('ol');
        outline.setAttribute('data-plan-outline', '');
        var b;
        for (b = 0; b < blocks.length; b++) {
            var block = blocks[b];
            if (!block || typeof block.id !== 'string' || !ENTRY.test(block.id)) continue;
            var row = el('li');
            row.setAttribute('data-plan-block', block.id);
            if (state.selectedId === block.id) row.setAttribute('aria-current', 'true');
            var label = el('span');
            label.textContent = blockLabel(block.type) + ' · ' + block.id;
            row.appendChild(label);
            var upIndex = moveDestination(b, 'up');
            var downIndex = moveDestination(b, 'down');
            row.appendChild(button('data-plan-move', 'up', 'למעלה (Up)', upIndex == null || upIndex < 0));
            row.appendChild(button(
                'data-plan-move',
                'down',
                'למטה (Down)',
                downIndex == null || downIndex > blocks.length - 1
            ));
            row.appendChild(button('data-plan-remove', '', 'הסרה (Remove)', false));
            row.appendChild(button('data-plan-edit', '', 'שדות (Fields)', false));
            outline.appendChild(row);
        }
        mainCol.appendChild(outline);

        paintInsert(mainCol, blocks);

        var fields = el('div');
        fields.className = 'card';
        fields.setAttribute('data-plan-fields', '');
        if (blockForm) {
            var fieldsHeading = el('h3');
            fieldsHeading.textContent = blockLabel(selected.block.type) + ' ';
            var headingId = el('span');
            headingId.className = 'plan-block-id';
            headingId.setAttribute('dir', 'ltr');
            headingId.textContent = selected.block.id;
            fieldsHeading.appendChild(headingId);
            fields.appendChild(fieldsHeading);
            fields.appendChild(blockForm.el);
        }
        mainCol.appendChild(fields);

        if (selected && selected.block && selected.block.type === 'introduction') {
            paintPictures(mainCol, selected.block);
        }

        paintMedia(mainCol);

        // The aside: the preview first, so it stays beside the form, then what the server found.
        var previewCard = el('section');
        previewCard.className = 'card plan-preview-card';
        previewCard.setAttribute('data-plan-preview-card', '');
        var previewHeading = el('h3');
        previewHeading.textContent = 'תצוגה מקדימה (Preview)';
        previewCard.appendChild(previewHeading);
        var previewLabel = el('p');
        previewLabel.className = 'plan-note';
        previewLabel.setAttribute('data-plan-preview-label', '');
        previewLabel.setAttribute('role', 'status');
        previewCard.appendChild(previewLabel);
        var preview = el('div');
        preview.className = 'plan-page';
        preview.setAttribute('data-plan-preview', '');
        preview.setAttribute('aria-label', 'תצוגה מקדימה');
        // The learner's controls are drawn and not wired: a click on one does nothing.
        var guard = function (ev) {
            if (root.HebrewPlanPreview) root.HebrewPlanPreview.guardClick(ev, preview);
        };
        preview.addEventListener('click', guard, state.controller ? { signal: state.controller.signal } : undefined);
        previewCard.appendChild(preview);
        state.previewEl = preview;
        state.previewLabel = previewLabel;
        aside.appendChild(previewCard);
        if (state.panels.issues) aside.appendChild(state.panels.issues.el);
        paintValidation(aside);
        if (state.panels.history) aside.appendChild(state.panels.history.el);

        panel.appendChild(rootEl);
        refreshPreview();

        // Findings the server returned for this document show at their fields: the issues list when
        // it has arrived, otherwise the findings of the last save or publish.
        var Issues = root.HebrewPlanIssues;
        var served = state.issues && Array.isArray(state.issues.issues) && Issues;
        editor.setIssues(served ? Issues.forEditor(state.issues.issues) : validationList(), state.plan);
        editor.setEditState(state.editState);
        if (focus) editor.restoreFocus(focus);
        if (scrollY !== null && typeof root.scrollTo === 'function') root.scrollTo(0, scrollY);
    }

    function onClick(ev) {
        var result = handleClick(ev);
        if (result && typeof result.then === 'function') track(result);
    }

    function targetNode(ev) {
        var node = ev && ev.target;
        if (node && node.nodeType === 3) node = node.parentNode;
        return node;
    }

    function attrNode(node, name) {
        var stop = state && state.panel;
        while (node && node !== stop) {
            if (node.getAttribute && node.hasAttribute && node.hasAttribute(name)) return node;
            node = node.parentNode;
        }
        return null;
    }

    function rowBlockId(node) {
        var row = attrNode(node, 'data-plan-block');
        if (!row) return '';
        var id = row.getAttribute('data-plan-block') || '';
        return ENTRY.test(id) ? id : '';
    }

    function handleClick(ev) {
        if (!state || state.busy) return Promise.resolve();
        var node = targetNode(ev);
        if (!node || !node.hasAttribute) return Promise.resolve();
        if (attrNode(node, 'data-plan-reload')) return reload();
        if (attrNode(node, 'data-plan-publish')) return publish();
        var lessonBtn = attrNode(node, 'data-plan-lesson');
        if (lessonBtn) {
            var index = Number(lessonBtn.getAttribute('data-plan-lesson'));
            var choice = state.lessonChoices[index];
            if (!choice || !LESSON_PATH.test(choice.path)) return Promise.resolve();
            return loadLesson(choice.path);
        }
        var edit = attrNode(node, 'data-plan-edit');
        if (edit) {
            var editId = rowBlockId(edit);
            if (!editId || !blockById(editId)) return Promise.resolve();
            state.selectedId = editId;
            paint();
            return Promise.resolve();
        }
        var move = attrNode(node, 'data-plan-move');
        if (move) return moveBlock(move);
        if (attrNode(node, 'data-plan-remove')) return removeBlock(node);
        if (attrNode(node, 'data-plan-insert')) return insertBlock();
        var disclose = attrNode(node, 'data-plan-disclose');
        if (disclose) {
            var section = disclose.getAttribute('data-plan-disclose');
            state.open[section] = state.open[section] !== true;
            paint();
            return Promise.resolve();
        }
        if (attrNode(node, 'data-plan-upload')) return uploadFile();
        if (attrNode(node, 'data-plan-dry-run')) return dryRun();
        if (attrNode(node, 'data-plan-generate')) return generateImage();
        if (attrNode(node, 'data-plan-approve')) return approveMedia();
        var attach = attrNode(node, 'data-plan-attach');
        if (attach) return attachPicture(attach);
        return Promise.resolve();
    }

    function onChange(ev) {
        if (!state) return;
        schedulePreview();
        var node = targetNode(ev);
        if (node && attrNode(node, 'data-plan-confirm')) syncGenerate();
        var typeSelect = node ? attrNode(node, 'data-plan-new-type') : null;
        if (typeSelect) renderNewDraft(String(typeSelect.value || ''));
    }

    function onInput(ev) {
        if (!state || !state.panel) return;
        schedulePreview();
        var node = targetNode(ev);
        if (!node) return;
        if (attrNode(node, 'data-plan-approve-alt')) syncApprove();
        if (!attrNode(node, 'data-plan-gen-prompt')) return;
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        var prompt = promptInput ? String(promptInput.value || '') : '';
        if (state.estimateShown && prompt !== state.estimatePrompt) {
            state.estimateShown = false;
            state.estimateDollars = null;
            var estimate = state.panel.querySelector('[data-plan-estimate]');
            if (estimate) {
                estimate.hidden = true;
                estimate.textContent = '';
            }
        }
        syncGenerate();
    }

    /** Reload takes what the server has and drops unsaved text (the admin asked for it). */
    function reload() {
        hideBox('data-plan-conflict');
        // reset() first: the next paint would otherwise take the live text for a draft again.
        state.editor.reset();
        state.editor.discardDrafts();
        return loadPlan();
    }

    function publish() {
        if (!Number.isInteger(state.revision)) return Promise.resolve();
        var mine = state.token;
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/publish', {}).then(function (ok) {
            if (!ok || !alive(mine)) return ok;
            // After a publish: the issues the server found, shown next to the new revision.
            var issues = state.panels.issues;
            var last = issues ? issues.last() : null;
            if (last && root.HebrewPlanIssues) {
                state.notice = 'פורסם (Published) · ' + state.revision + ' · ' + root.HebrewPlanIssues.countsText(last.counts).text;
                var node = state.panel && state.panel.querySelector('[data-plan-notice]');
                if (node) node.textContent = state.notice;
            }
            return ok;
        });
    }

    function moveBlock(node) {
        if (node.hasAttribute('data-plan-locked') || node.disabled) return Promise.resolve();
        var id = rowBlockId(node);
        var found = id ? blockById(id) : null;
        if (!found || !LESSON_PATH.test(state.lessonPath)) return Promise.resolve();
        var direction = node.getAttribute('data-plan-move');
        var index = moveDestination(found.index, direction);
        var count = blocksOf(state.lesson).length;
        if (!Number.isInteger(index) || index < 0 || index > count - 1) return Promise.resolve();
        var path = state.lessonPath + '/blocks/' + id;
        if (!PART_PATH.test(path)) return Promise.resolve();
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/blocks/move', {
            path: path,
            index: index
        });
    }

    function removeBlock(node) {
        var id = rowBlockId(node);
        if (!id || !LESSON_PATH.test(state.lessonPath)) return Promise.resolve();
        var ok = true;
        try {
            if (typeof root.confirm === 'function') ok = root.confirm('Remove this block?');
        } catch (err) {
            ok = false;
        }
        if (!ok) return Promise.resolve();
        var path = state.lessonPath + '/blocks/' + id;
        if (!PART_PATH.test(path)) return Promise.resolve();
        return write('DELETE', '/api/plans/' + encodeURIComponent(state.planId) + '/parts', { path: path });
    }

    function insertBlock() {
        if (!state.panel || !LESSON_PATH.test(state.lessonPath || '')) return Promise.resolve();
        var idInput = state.panel.querySelector('[data-plan-new-id]');
        var typeSelect = state.panel.querySelector('[data-plan-new-type]');
        var indexInput = state.panel.querySelector('[data-plan-new-index]');
        var id = idInput ? String(idInput.value || '').trim() : '';
        var type = typeSelect ? String(typeSelect.value || '') : '';
        var rawIndex = indexInput ? String(indexInput.value || '').trim() : '';
        var count = blocksOf(state.lesson).length;
        var index = rawIndex === '' ? count : Number(rawIndex);
        if (!ENTRY.test(id) || blockById(id)) {
            showBox('data-plan-error', 'invalid id');
            return Promise.resolve();
        }
        if (!Number.isInteger(index) || index < 0 || index > count) {
            showBox('data-plan-error', 'invalid index');
            return Promise.resolve();
        }
        var block = newBlock(type, id);
        if (!block) {
            showBox('data-plan-error', 'invalid type');
            return Promise.resolve();
        }
        // The required fields the admin filled in, when the form on screen is for this type.
        if (state.draft && state.draft.type === type) {
            var problem = state.draft.handle.check();
            if (problem) {
                showBox('data-plan-error', problem);
                return Promise.resolve();
            }
            var drafted = state.draft.handle.getValue();
            if (drafted && typeof drafted === 'object') {
                Object.keys(drafted).forEach(function (key) {
                    if (key !== 'id' && key !== 'type') block[key] = drafted[key];
                });
            }
        }
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/blocks', {
            path: state.lessonPath,
            index: index,
            block: block
        }, { onValidation: showNewIssues });
    }

    /**
     * @param {HTMLInputElement|null} input
     * @returns {Blob|object|null}
     */
    function selectedFile(input) {
        if (!input || !input.files || !input.files.length) return null;
        return input.files[0];
    }

    /**
     * Authenticated admin call that is not a plan revision write.
     * @param {string} method
     * @param {string} path
     * @param {object|FormData} body
     * @returns {Promise<boolean>}
     */
    function mediaCall(method, path, body) {
        var mine = state.token;
        if (!api() || state.busy) return Promise.resolve(false);
        state.busy = true;
        return api().request(path, { method: method, body: body }).then(function (res) {
            if (!alive(mine)) return false;
            state.busy = false;
            if (!res || !res.ok || !res.data) {
                showError(res);
                return false;
            }
            rememberMedia(res.data);
            hideBox('data-plan-error');
            paint();
            return true;
        }).catch(function (err) {
            if (!alive(mine)) return false;
            state.busy = false;
            showBox('data-plan-error', (err && err.message) || 'request failed');
            return false;
        });
    }

    function uploadFile() {
        if (!state.panel) return Promise.resolve(false);
        var idInput = state.panel.querySelector('[data-plan-upload-id]');
        var fileInput = state.panel.querySelector('[data-plan-upload-file]');
        var altInput = state.panel.querySelector('[data-plan-upload-alt]');
        var id = idInput ? String(idInput.value || '').trim() : '';
        var file = selectedFile(fileInput);
        if (!MEDIA_ID.test(id) || !file) {
            showBox('data-plan-error', 'invalid upload');
            return Promise.resolve(false);
        }
        if (typeof root.FormData !== 'function') {
            showBox('data-plan-error', 'upload unavailable');
            return Promise.resolve(false);
        }
        var form = new root.FormData();
        form.append('id', id);
        var alt = altInput ? String(altInput.value || '') : '';
        if (alt) form.append('altText', alt);
        form.append('file', file);
        return mediaCall('POST', '/api/media', form);
    }

    function dryRun() {
        if (!state.panel) return Promise.resolve();
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        var prompt = promptInput ? String(promptInput.value || '') : '';
        if (!prompt.trim() || /[\u0000-\u001F]/.test(prompt)) {
            showBox('data-plan-error', 'invalid prompt');
            return Promise.resolve();
        }
        var mine = state.token;
        if (!api() || state.busy) return Promise.resolve();
        state.busy = true;
        var path = '/api/admin/images/image-plan?prompt=' + encodeURIComponent(prompt) + '&count=1';
        return api().get(path).then(function (res) {
            if (!alive(mine)) return;
            state.busy = false;
            var dollars = res && res.data ? res.data.estimatedDollars : undefined;
            if (!res || !res.ok || typeof dollars !== 'number' || !Number.isFinite(dollars)) {
                showError(res);
                return;
            }
            state.estimateShown = true;
            state.estimateDollars = dollars;
            state.estimatePrompt = prompt;
            var node = state.panel.querySelector('[data-plan-estimate]');
            if (node) {
                node.hidden = false;
                node.textContent = String(dollars);
            }
            var box = state.panel.querySelector('[data-plan-confirm]');
            if (box) box.checked = false;
            hideBox('data-plan-error');
            syncGenerate();
        }).catch(function (err) {
            if (!alive(mine)) return;
            state.busy = false;
            showBox('data-plan-error', (err && err.message) || 'request failed');
        });
    }

    function generateImage() {
        if (!state.panel) return Promise.resolve(false);
        var btn = state.panel.querySelector('[data-plan-generate]');
        var box = state.panel.querySelector('[data-plan-confirm]');
        var idInput = state.panel.querySelector('[data-plan-gen-id]');
        var promptInput = state.panel.querySelector('[data-plan-gen-prompt]');
        if (!btn || btn.disabled) return Promise.resolve(false);
        var id = idInput ? String(idInput.value || '').trim() : '';
        var prompt = promptInput ? String(promptInput.value || '') : '';
        if (!state.estimateShown || typeof state.estimateDollars !== 'number' || !box || box.checked !== true) {
            return Promise.resolve(false);
        }
        if (!MEDIA_ID.test(id) || prompt !== state.estimatePrompt) return Promise.resolve(false);
        return mediaCall('POST', '/api/admin/images', {
            prompt: state.estimatePrompt,
            id: id,
            confirm: true,
            estimate: state.estimateDollars
        });
    }

    function approveMedia() {
        if (!state.panel) return Promise.resolve(false);
        var btn = state.panel.querySelector('[data-plan-approve]');
        var altInput = state.panel.querySelector('[data-plan-approve-alt]');
        var idInput = state.panel.querySelector('[data-plan-approve-id]');
        if (!btn || btn.disabled) return Promise.resolve(false);
        var alt = altInput ? String(altInput.value || '').trim() : '';
        var id = idInput ? String(idInput.value || '').trim() : '';
        if (!alt || !MEDIA_ID.test(id)) return Promise.resolve(false);
        return mediaCall('POST', '/api/admin/media/' + encodeURIComponent(id) + '/approval', {
            altText: alt
        });
    }

    /**
     * @param {object} block
     * @param {string} id
     * @returns {object|null}
     */
    function wordById(block, id) {
        var words = block && Array.isArray(block.words) ? block.words : [];
        var i;
        for (i = 0; i < words.length; i++) {
            if (words[i] && words[i].id === id) return words[i];
        }
        return null;
    }

    /**
     * set_field mediaId and alt. A word with no picture gets an image object
     * first, because those two names are written onto image only when it exists.
     * @param {HTMLElement} node
     * @returns {Promise<boolean>}
     */
    function attachPicture(node) {
        var wordId = node.getAttribute('data-plan-attach') || '';
        if (!ENTRY.test(wordId) || !state.panel) return Promise.resolve(false);
        var found = state.selectedId ? blockById(state.selectedId) : null;
        if (!found || found.block.type !== 'introduction') return Promise.resolve(false);
        var word = wordById(found.block, wordId);
        var row = attrNode(node, 'data-plan-word');
        if (!word || !row) return Promise.resolve(false);
        var mediaInput = row.querySelector('[data-plan-attach-media]');
        var altInput = row.querySelector('[data-plan-attach-alt]');
        var mediaId = mediaInput ? String(mediaInput.value || '').trim() : '';
        var alt = altInput ? String(altInput.value || '') : '';
        if (!MEDIA_ID.test(mediaId) || !String(alt).trim()) {
            showBox('data-plan-error', 'invalid picture');
            return Promise.resolve(false);
        }
        var path = state.lessonPath + '/blocks/' + found.block.id + '/words/' + wordId;
        if (!PART_PATH.test(path) || !LESSON_PATH.test(state.lessonPath || '')) return Promise.resolve(false);
        var steps = [];
        if (!word.image || typeof word.image !== 'object' || Array.isArray(word.image)) {
            steps.push({ field: 'image', value: { mediaId: mediaId, alt: alt } });
        }
        steps.push({ field: 'mediaId', value: mediaId });
        steps.push({ field: 'alt', value: alt });
        return sendAttach(path, steps, 0);
    }

    /**
     * @param {string} path
     * @param {{ field: string, value: unknown }[]} steps
     * @param {number} index
     * @returns {Promise<boolean>}
     */
    function sendAttach(path, steps, index) {
        if (!state || index >= steps.length) return Promise.resolve(true);
        var step = steps[index];
        return write('PATCH', '/api/plans/' + encodeURIComponent(state.planId) + '/fields', {
            path: path,
            field: step.field,
            value: step.value
        }).then(function (ok) {
            if (!ok) return false;
            return sendAttach(path, steps, index + 1);
        });
    }

    /**
     * One revision-checked write. `expectedRevision` is the value the server last
     * returned (state.revision), never a value read from the page.
     * `hooks.onSaved` runs on success before the page repaints; `hooks.onValidation`
     * gets a 400 validation body. The status and body of the last answer stay in
     * `state.last` for the editor's transport.
     * @param {string} method
     * @param {string} path
     * @param {object} body
     * @param {{ onSaved?: Function, onValidation?: Function }} [hooks]
     * @returns {Promise<boolean>}
     */
    function write(method, path, body, hooks) {
        var mine = state.token;
        if (!Number.isInteger(state.revision) || !api() || state.busy) return Promise.resolve(false);
        var payload = {
            expectedRevision: state.revision
        };
        var key;
        for (key in body) {
            if (Object.prototype.hasOwnProperty.call(body, key)) payload[key] = body[key];
        }
        state.busy = true;
        return api().request(path, { method: method, body: payload }).then(function (res) {
            if (!alive(mine)) return false;
            state.busy = false;
            state.last = { ok: !!(res && res.ok), status: res ? res.status : 0, data: res ? res.data : null };
            if (res && res.status === 409) {
                showConflict(res.data || {});
                return false;
            }
            if (res && res.status === 400 && res.data && res.data.error === 'validation') {
                // No repaint: the typed text and the issues at the fields stay where they are.
                state.validation = findingsFrom(res.data);
                state.publishRefused = path.indexOf('/publish') !== -1;
                state.publishRefusal = typeof res.data.error === 'string' ? res.data.error : '';
                hideBox('data-plan-conflict');
                refreshValidation();
                if (hooks && typeof hooks.onValidation === 'function') hooks.onValidation(res.data);
                return false;
            }
            if (!res || !res.ok || !res.data || !Number.isInteger(res.data.revision)) {
                showError(res);
                return false;
            }
            state.revision = res.data.revision;
            if (res.data.plan && typeof res.data.plan === 'object') state.plan = res.data.plan;
            state.validation = findingsFrom(res.data);
            state.publishRefused = false;
            state.publishRefusal = '';
            state.notice = 'נשמר (Saved) · ' + res.data.revision;
            hideBox('data-plan-conflict');
            if (hooks && typeof hooks.onSaved === 'function') hooks.onSaved();
            if (state.lessonPath) {
                return loadLesson(state.lessonPath).then(function () { return refreshSide(); }).then(function () { return true; });
            }
            paint();
            return refreshSide().then(function () { return true; });
        }).catch(function (err) {
            if (!alive(mine)) return false;
            state.busy = false;
            showBox('data-plan-error', (err && err.message) || 'request failed');
            return false;
        });
    }

    function loadLesson(path) {
        var mine = state.token;
        if (!LESSON_PATH.test(path) || !api()) return Promise.resolve();
        return api().get(partsUrl(path)).then(function (res) {
            if (!alive(mine)) return;
            if (!res || !res.ok || !res.data || typeof res.data.part !== 'object' || Array.isArray(res.data.part)) {
                showError(res);
                return;
            }
            if (!Number.isInteger(res.data.revision)) {
                showError(res);
                return;
            }
            state.lesson = res.data.part;
            state.lessonPath = path;
            state.revision = res.data.revision;
            hideBox('data-plan-conflict');
            paint();
        }).catch(function (err) {
            if (!alive(mine)) return;
            showBox('data-plan-error', (err && err.message) || 'request failed');
        });
    }

    function loadPlan() {
        var mine = state.token;
        if (!api()) {
            paintMessage('HebrewApi missing');
            return Promise.resolve();
        }
        return api().get(partsUrl('')).then(function (res) {
            if (!alive(mine)) return;
            if (!res || !res.ok || !res.data || typeof res.data.part !== 'object' || Array.isArray(res.data.part)) {
                paintMessage(errorText(res));
                return;
            }
            if (!Number.isInteger(res.data.revision)) {
                paintMessage(errorText(res));
                return;
            }
            state.plan = res.data.part;
            state.revision = res.data.revision;
            state.editState = humanEditedOf(res.data.editState);
            state.lessonChoices = listLessons(state.plan);
            if (state.panels.assign) state.panels.assign.load();
            var shown;
            // A lesson that was open stays open after a reload or a restore, when it is still there.
            var keep = state.lessonChoices.filter(function (c) { return c.path === state.lessonPath; })[0];
            if (state.lessonChoices.length === 1 || keep) {
                shown = loadLesson((keep || state.lessonChoices[0]).path);
            } else {
                state.lesson = null;
                state.lessonPath = '';
                paint();
                shown = Promise.resolve();
            }
            return shown.then(function () {
                if (!alive(mine)) return undefined;
                return ensureSchema();
            }).then(function () {
                if (!alive(mine)) return undefined;
                // the read above already carried who last wrote each field
                return refreshSide({ skipEditState: true });
            });
        }).catch(function (err) {
            if (!alive(mine)) return;
            paintMessage((err && err.message) || 'request failed');
        });
    }

    /**
     * The plan JSON Schema is fetched once per mount, after the plan has painted.
     * Until it arrives the form area says so; if it does not arrive, every node can
     * still be edited as raw JSON.
     * @returns {Promise<void>}
     */
    function ensureSchema() {
        var mine = state.token;
        if (state.editor.hasSchema()) {
            state.schema = 'ready';
            paint();
            return Promise.resolve();
        }
        return state.editor.loadSchema(api()).then(function (ok) {
            if (!alive(mine)) return;
            state.schema = ok ? 'ready' : 'failed';
            paint();
        });
    }

    function isDirty() {
        return !!(state && state.editor && state.editor.hasUnsaved());
    }

    /**
     * What the editor sends through. Each call is one revision-checked write; the
     * answer carries the status and body so the editor can show a conflict or the
     * server's rule codes at the field.
     */
    function makeTransport(mine) {
        function call(method, suffix, body, hooks) {
            if (!state || state !== mine) return Promise.resolve({ ok: false, status: 0, data: null });
            if (state.busy) return Promise.resolve({ ok: false, status: 0, data: { code: 'busy' } });
            var url = '/api/plans/' + encodeURIComponent(mine.planId) + suffix;
            return write(method, url, body, hooks).then(function (ok) {
                var last = mine.last || {};
                return { ok: ok, status: last.status, data: last.data };
            });
        }
        return {
            setField: function (path, field, value, hooks) {
                return call('PATCH', '/fields', { path: path, field: field, value: value }, hooks);
            },
            putPart: function (path, value, hooks) {
                return call('PUT', '/parts', { path: path, value: value }, hooks);
            }
        };
    }

    /**
     * @param {HTMLElement} panel
     * @param {string} planId
     * @returns {Promise<void>}
     */
    function mount(panel, planId) {
        unmount();
        var mine = token;
        state = {
            token: mine,
            panel: panel,
            planId: planId,
            plan: null,
            lesson: null,
            lessonPath: '',
            lessonChoices: [],
            selectedId: null,
            open: { lesson: true, unit: false, plan: false },
            schema: 'loading',
            draft: null,
            newDraftHost: null,
            validationHost: null,
            notice: '',
            last: null,
            controller: null,
            revision: null,
            busy: false,
            media: [],
            estimateShown: false,
            estimateDollars: null,
            estimatePrompt: '',
            validation: null,
            publishRefused: false,
            publishRefusal: '',
            issues: null,
            publishNote: '',
            editState: [],
            panels: {},
            transport: null,
            previewTimer: null,
            previewEl: null,
            previewLabel: null
        };
        if (!panel || !SLUG.test(planId)) {
            if (panel) paintMessage('invalid plan id');
            return Promise.resolve();
        }
        var controller = typeof root.AbortController === 'function' ? new root.AbortController() : null;
        var listen = controller ? { signal: controller.signal } : undefined;
        state.controller = controller;
        panel.addEventListener('click', onClick, listen);
        panel.addEventListener('change', onChange, listen);
        panel.addEventListener('input', onInput, listen);
        if (!form()) {
            paintMessage('HebrewPlanForm missing');
            return Promise.resolve();
        }
        state.transport = makeTransport(state);
        state.editor = form().createEditor({
            doc: root.document,
            host: panel,
            signal: controller ? controller.signal : null,
            transport: state.transport,
            track: track,
            getPlan: function () { return state && state.plan; },
            onToggle: function () { if (state) paint(); },
            onRestoreValue: onRestoreValue
        });
        makePanels(controller ? controller.signal : null);
        return track(loadPlan());
    }

    // ---- the INIT-008 panels -------------------------------------------------------------------

    function makePanels(signal) {
        var current = state;
        var common = { doc: root.document, api: api(), planId: current.planId, signal: signal };
        function extend(extra) {
            var out = {};
            Object.keys(common).forEach(function (key) { out[key] = common[key]; });
            Object.keys(extra).forEach(function (key) { out[key] = extra[key]; });
            return out;
        }
        if (root.HebrewPlanIssues && typeof root.HebrewPlanIssues.create === 'function') {
            current.panels.issues = root.HebrewPlanIssues.create(extend({
                getPlan: function () { return state && state.plan; },
                onJump: jumpTo,
                onFix: applyFix,
                onLoaded: onIssues
            }));
        }
        if (root.HebrewPlanHistory && typeof root.HebrewPlanHistory.create === 'function') {
            current.panels.history = root.HebrewPlanHistory.create(extend({
                getRevision: function () { return state ? state.revision : null; },
                hasUnsaved: isDirty,
                onRestored: afterRestore,
                onReload: function () { if (state) track(reload()); }
            }));
        }
        if (root.HebrewPlanList && typeof root.HebrewPlanList.createAssign === 'function') {
            current.panels.assign = root.HebrewPlanList.createAssign(extend({
                getRevision: function () { return state ? state.revision : null; },
                onAssigned: afterAssign,
                onConflict: function (data) { if (state) showConflict(data); }
            }));
        }
    }

    /** `editState.humanEdited` of a read, or an empty list. */
    function humanEditedOf(editState) {
        return editState && Array.isArray(editState.humanEdited) ? editState.humanEdited : [];
    }

    /** The server's issues arrived: the publish line, the inline issues at the fields. */
    function onIssues(result) {
        if (!state) return;
        state.issues = result;
        var Issues = root.HebrewPlanIssues;
        state.publishNote = Issues ? Issues.publishNote(result.counts) : '';
        var note = state.panel && state.panel.querySelector('[data-plan-publish-note]');
        if (note) note.textContent = state.publishNote;
        if (state.editor && Issues) state.editor.setIssues(Issues.forEditor(result.issues), state.plan);
    }

    /**
     * What the side panels show, after the plan was read or written: the issues, the history when it
     * is open, and which fields a person last wrote. Resolves when they have all answered.
     * @param {{ skipEditState?: boolean }} [opts]
     * @returns {Promise<void>}
     */
    function refreshSide(opts) {
        if (!state) return Promise.resolve();
        var mine = state.token;
        var panels = state.panels;
        var jobs = [];
        if (panels.issues) jobs.push(panels.issues.load());
        if (panels.history) {
            jobs.push(panels.history.load());
            if (!(opts && opts.skipEditState)) jobs.push(loadEditState());
        }
        if (!jobs.length) return Promise.resolve();
        return Promise.all(jobs).then(function () {
            if (!alive(mine)) return;
            if (state.editor) state.editor.setEditState(state.editState);
        });
    }

    function loadEditState() {
        var mine = state.token;
        return api().get(partsUrl('')).then(function (res) {
            if (!alive(mine)) return;
            if (res && res.ok && res.data && res.data.editState) state.editState = humanEditedOf(res.data.editState);
        }).catch(function () { /* the marks stay as they were */ });
    }

    /** Open the node's form and focus the field an issue is about. */
    function jumpTo(issue) {
        if (!state || !issue || !form()) return Promise.resolve();
        var path = typeof issue.path === 'string' ? issue.path : '';
        var field = typeof issue.field === 'string' ? issue.field : '';
        var kind = form().kindOfPath(path);
        if (kind === null) return Promise.resolve();
        var segs = path.split('/').filter(Boolean);
        var lessonPath = segs.length >= 4 ? segs.slice(0, 4).join('/') : '';
        if (kind === 'unit' && unitPathOf(state.lessonPath) !== path) {
            var inUnit = state.lessonChoices.filter(function (c) { return c.path.indexOf(path + '/lessons/') === 0; })[0];
            if (inUnit) lessonPath = inUnit.path;
        }
        var step = Promise.resolve();
        if (lessonPath && lessonPath !== state.lessonPath && LESSON_PATH.test(lessonPath)) step = loadLesson(lessonPath);
        var mine = state.token;
        return track(step.then(function () {
            if (!alive(mine)) return;
            if (kind === 'plan') state.open.plan = true;
            else if (kind === 'unit') state.open.unit = true;
            else if (kind === 'lesson') state.open.lesson = true;
            else {
                var blockId = segs[5];
                if (!ENTRY.test(blockId) || !blockById(blockId)) {
                    showBox('data-plan-error', 'not_found — ' + path);
                    return;
                }
                state.selectedId = blockId;
                if (kind === 'item') state.editor.open(path);
            }
            paint();
            state.editor.focusField(path, field);
        }));
    }

    /** One set_field through the same save path as every edit; the answer tells the issues panel what happened. */
    function applyFix(request) {
        if (!state || !state.transport) return Promise.resolve(false);
        return state.transport.setField(request.path, request.field, request.value, {}).then(function (out) {
            if (out && out.ok) return 'refreshed';
            var Ui = root.HebrewPlanUi;
            return { error: Ui ? Ui.failureText({ status: out ? out.status : 0, data: out ? out.data : null }) : 'refused' };
        });
    }

    /** A restore came back 200: take the new revision and read the plan again. */
    function afterRestore(data) {
        if (!state) return Promise.resolve('refreshed');
        state.revision = data.revision;
        if (data.plan && typeof data.plan === 'object') state.plan = data.plan;
        state.validation = findingsFrom(data);
        state.publishRefused = false;
        state.publishRefusal = '';
        state.notice = 'שוחזר (Restored) · ' + data.revision;
        hideBox('data-plan-conflict');
        hideBox('data-plan-error');
        // reset() first: the next paint would otherwise take the live text for a draft again.
        state.editor.reset();
        state.editor.discardDrafts();
        return track(loadPlan().then(function () { return 'refreshed'; }));
    }

    /** An assignment came back 200. */
    function afterAssign(data) {
        if (!state) return;
        state.revision = data.revision;
        if (data.plan && typeof data.plan === 'object') state.plan = data.plan;
        state.validation = findingsFrom(data);
        state.notice = 'השיוך נשמר (Assignment saved) · ' + data.revision;
        hideBox('data-plan-conflict');
        var step = state.lessonPath ? loadLesson(state.lessonPath) : (paint(), Promise.resolve());
        track(step.then(function () { return refreshSide(); }));
    }

    /** "Restore the earlier value" under a field: the history panel asks once, then restores. */
    function onRestoreValue(spec) {
        var history = state && state.panels.history;
        if (!history || !spec || !spec.anchor) return;
        history.confirmAt(spec.anchor, { fromRevision: spec.fromRevision, path: spec.path, field: spec.field });
    }

    // ---- the live preview ----

    /** Draw the preview from the saved node with the unsaved field text laid over it. */
    function refreshPreview() {
        if (!state || !state.previewEl) return;
        var node = state.previewEl;
        var selected = state.selectedId ? blockById(state.selectedId) : null;
        var Preview = root.HebrewPlanPreview;
        var out;
        if (Preview) {
            out = Preview.compute({
                render: root.HebrewPlanRender,
                lesson: state.lesson,
                lessonPath: state.lessonPath,
                block: selected ? selected.block : null,
                blockPath: selected ? state.lessonPath + '/blocks/' + selected.block.id : '',
                handles: state.editor ? state.editor.handles() : []
            });
        } else {
            out = { html: '', state: 'saved' };
            if (selected && root.HebrewPlanRender && typeof root.HebrewPlanRender.renderBlock === 'function') {
                out.html = root.HebrewPlanRender.renderBlock(selected.block, { lesson: state.lesson });
            }
        }
        // Renderer output only: plan text is escaped there. This does not eval it.
        if (typeof out.html === 'string') node.innerHTML = out.html;
        node.setAttribute('data-plan-preview-state', out.state);
        if (state.previewLabel) {
            state.previewLabel.textContent = out.state === 'unsaved'
                ? 'תצוגה מקדימה — לא נשמרה (Preview — not saved)'
                : 'תצוגה מקדימה — כפי ששמור (Preview — as saved)';
        }
    }

    function schedulePreview() {
        if (!state || !state.previewEl || typeof root.setTimeout !== 'function') return;
        if (state.previewTimer && typeof root.clearTimeout === 'function') root.clearTimeout(state.previewTimer);
        var mine = state.token;
        var wait = root.HebrewPlanPreview ? root.HebrewPlanPreview.DEBOUNCE_MS : 250;
        state.previewTimer = root.setTimeout(function () {
            if (alive(mine)) refreshPreview();
        }, wait);
    }

    /**
     * The Plans tab: a table of every plan, a New plan form, and a way to open one (plan-list.js).
     * Does not call the lesson worksheet API.
     * @param {HTMLElement} panel
     * @returns {void}
     */
    function mountPicker(panel) {
        unmount();
        if (!panel) return;
        if (!root.HebrewPlanList || typeof root.HebrewPlanList.mount !== 'function') {
            empty(panel);
            var missing = el('p');
            missing.className = 'admin-inline-error';
            missing.setAttribute('role', 'alert');
            missing.textContent = 'HebrewPlanList missing';
            panel.appendChild(missing);
            return;
        }
        pickerCleanup = root.HebrewPlanList.mount(panel, {
            doc: root.document,
            api: api(),
            setHash: function (route) {
                if (root.HebrewAdminShell && typeof root.HebrewAdminShell.setHash === 'function') {
                    root.HebrewAdminShell.setHash(route);
                }
            }
        });
    }

    function destroyPanels(current) {
        if (current.previewTimer && typeof root.clearTimeout === 'function') root.clearTimeout(current.previewTimer);
        current.previewTimer = null;
        var panels = current.panels || {};
        Object.keys(panels).forEach(function (name) {
            if (panels[name] && typeof panels[name].destroy === 'function') panels[name].destroy();
        });
        current.panels = {};
    }

    function unmount() {
        token += 1;
        if (state) {
            if (state.controller) state.controller.abort();
            if (state.panel) {
                state.panel.removeEventListener('click', onClick);
                state.panel.removeEventListener('change', onChange);
                state.panel.removeEventListener('input', onInput);
            }
            destroyPanels(state);
            if (state.editor) state.editor.destroy();
        }
        if (pickerCleanup) pickerCleanup();
        pickerCleanup = null;
        state = null;
    }

    /**
     * @returns {boolean}
     */
    function canLeave() {
        if (!isDirty()) return true;
        try {
            if (typeof root.confirm === 'function') return !!root.confirm('Discard unsaved field text?');
        } catch (err) {
            return false;
        }
        return true;
    }

    /**
     * The promise for the last outline action. Tests wait on this.
     * @returns {Promise<void>}
     */
    function settled() {
        return inflight;
    }

    root.HebrewPlanBuilder = {
        SLUG: SLUG,
        blockTypes: function () { return form() ? form().blockTypeList() : []; },
        canLeave: canLeave,
        editor: function () { return state ? state.editor : null; },
        mount: mount,
        mountPicker: mountPicker,
        moveDestination: moveDestination,
        newBlock: newBlock,
        settled: settled,
        unmount: unmount
    };
})(typeof window !== 'undefined' ? window : globalThis);
