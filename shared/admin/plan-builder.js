/**
 * Admin outline for one lesson plan: insert, move, remove, set a field, publish.
 * Media upload, dry-run, confirmed generation, and approval are the SPEC-011
 * routes. Picture attach sends set_field. The validation panel prints the
 * save and publish findings. It does not recompute a cap. Preview is the
 * SPEC-014 renderer string for the selected block.
 * Every change is an HTTP call. This file does not edit the plan JSON,
 * does not save a worksheet, and does not read an API key.
 * Provenance: INIT-004/SPEC-015, INIT-004/SPEC-016
 */
(function (root) {
    'use strict';

    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var ENTRY = /^[a-z0-9][a-z0-9-]*$/;
    var LESSON_PATH = /^units\/[a-z0-9]+(?:-[a-z0-9]+)*\/lessons\/[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var PART_PATH = /^units\/[a-z0-9]+(?:-[a-z0-9]+)*\/lessons\/[a-z0-9]+(?:-[a-z0-9]+)*\/blocks\/[a-z0-9][a-z0-9-]*(?:\/(?:words|paragraphs|lines|questions)\/[a-z0-9][a-z0-9-]*)?$/;
    var MEDIA_ID = /^[a-z0-9][a-z0-9-]{0,63}$/;

    var BLOCK_TYPES = Object.freeze([
        'introduction',
        'graded-story',
        'listening',
        'short-dialogue',
        'fluency-pass',
        'spaced-review-checkpoint'
    ]);

    var BLOCK_FIELDS = {
        introduction: ['heading', 'note'],
        'graded-story': ['notice'],
        listening: ['passage'],
        'short-dialogue': ['intro', 'introEn'],
        'fluency-pass': ['source'],
        'spaced-review-checkpoint': []
    };

    var ITEM_FIELDS = {
        words: ['word', 'meaning', 'sound', 'translit', 'pos', 'trap', 'infinitive', 'sentence'],
        paragraphs: ['text'],
        lines: ['pointed', 'question', 'gloss', 'speak', 'youFeminine', 'followUp'],
        questions: ['question', 'sampleAnswer']
    };

    var FIELD_OK = {
        heading: true,
        note: true,
        notice: true,
        passage: true,
        intro: true,
        introEn: true,
        source: true,
        seconds: true,
        word: true,
        meaning: true,
        sound: true,
        translit: true,
        pos: true,
        trap: true,
        infinitive: true,
        sentence: true,
        text: true,
        pointed: true,
        question: true,
        gloss: true,
        speak: true,
        youFeminine: true,
        followUp: true,
        sampleAnswer: true
    };

    var token = 0;
    var state = null;
    var inflight = Promise.resolve();

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

    /**
     * A new block. Does not copy or renumber ids already in the lesson.
     * @param {string} type
     * @param {string} id
     * @returns {object|null}
     */
    function newBlock(type, id) {
        if (!ENTRY.test(id) || BLOCK_TYPES.indexOf(type) === -1) return null;
        if (type === 'introduction') {
            return { id: id, type: type, words: [{ id: '1', word: 'מילה', meaning: 'word' }] };
        }
        if (type === 'graded-story') {
            return { id: id, type: type, paragraphs: [{ id: '1', text: '…' }] };
        }
        if (type === 'listening') {
            return { id: id, type: type, passage: '…' };
        }
        if (type === 'short-dialogue') {
            return { id: id, type: type, lines: [{ id: '1', pointed: '…' }] };
        }
        if (type === 'fluency-pass') {
            return { id: id, type: type, seconds: 30, source: 'vocab' };
        }
        if (type === 'spaced-review-checkpoint') {
            return { id: id, type: type, wordKeys: ['מילה'], fromLessons: ['prior'] };
        }
        return null;
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

    function pushField(list, path, field, value, caption) {
        if (!FIELD_OK[field] || !PART_PATH.test(path)) return;
        if (typeof value !== 'string') return;
        list.push({ path: path, field: field, loaded: value, caption: caption, numeric: false });
    }

    function fieldsForBlock(lessonPath, block) {
        var list = [];
        if (!block || typeof block.id !== 'string' || !ENTRY.test(block.id)) return list;
        if (!LESSON_PATH.test(lessonPath)) return list;
        var bPath = lessonPath + '/blocks/' + block.id;
        var names = BLOCK_FIELDS[block.type] || [];
        var n;
        for (n = 0; n < names.length; n++) {
            if (typeof block[names[n]] === 'string') {
                pushField(list, bPath, names[n], block[names[n]], block.id + ' / ' + names[n]);
            }
        }
        if (block.type === 'fluency-pass' && Number.isInteger(block.seconds) && PART_PATH.test(bPath)) {
            list.push({
                path: bPath,
                field: 'seconds',
                loaded: String(block.seconds),
                caption: block.id + ' / seconds',
                numeric: true
            });
        }
        var bucket;
        for (bucket in ITEM_FIELDS) {
            if (!Object.prototype.hasOwnProperty.call(ITEM_FIELDS, bucket)) continue;
            var items = block[bucket];
            if (!Array.isArray(items)) continue;
            var i;
            for (i = 0; i < items.length; i++) {
                var item = items[i];
                if (!item || typeof item.id !== 'string' || !ENTRY.test(item.id)) continue;
                var path = bPath + '/' + bucket + '/' + item.id;
                var fields = ITEM_FIELDS[bucket];
                var f;
                for (f = 0; f < fields.length; f++) {
                    if (typeof item[fields[f]] === 'string') {
                        pushField(
                            list,
                            path,
                            fields[f],
                            item[fields[f]],
                            block.id + ' / ' + bucket + ' / ' + item.id + ' / ' + fields[f]
                        );
                    }
                }
            }
        }
        return list;
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
        if (res && typeof res.error === 'string' && res.error) return res.error;
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
     * @param {HTMLElement} parent
     */
    function paintValidation(parent) {
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
        parent.appendChild(box);
    }

    function paint() {
        var panel = state.panel;
        var lesson = state.lesson;
        var blocks = blocksOf(lesson);
        var selected = state.selectedId ? blockById(state.selectedId) : null;
        if (state.selectedId && !selected) state.selectedId = null;
        state.fields = selected ? fieldsForBlock(state.lessonPath, selected.block) : [];

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
        toolbar.appendChild(button('data-plan-publish', '', 'פרסום (Publish)', false));
        toolbar.appendChild(button('data-plan-reload', '', 'טעינה מחדש (Reload)', false));
        rootEl.appendChild(toolbar);

        rootEl.appendChild(messageNode('data-plan-conflict'));
        rootEl.appendChild(messageNode('data-plan-error'));

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

        var outlineHeading = el('h3');
        outlineHeading.textContent = 'בלוקים (Blocks)';
        rootEl.appendChild(outlineHeading);
        var outline = el('ol');
        outline.setAttribute('data-plan-outline', '');
        var b;
        for (b = 0; b < blocks.length; b++) {
            var block = blocks[b];
            if (!block || typeof block.id !== 'string' || !ENTRY.test(block.id)) continue;
            var row = el('li');
            row.setAttribute('data-plan-block', block.id);
            var label = el('span');
            label.textContent = (typeof block.type === 'string' ? block.type : '') + ' ' + block.id;
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
        rootEl.appendChild(outline);

        var insert = el('div');
        insert.className = 'card';
        var idInput = el('input');
        idInput.type = 'text';
        idInput.setAttribute('data-plan-new-id', '');
        idInput.setAttribute('autocomplete', 'off');
        var typeSelect = el('select');
        typeSelect.setAttribute('data-plan-new-type', '');
        var t;
        for (t = 0; t < BLOCK_TYPES.length; t++) {
            var opt = el('option');
            opt.value = BLOCK_TYPES[t];
            opt.textContent = BLOCK_TYPES[t];
            typeSelect.appendChild(opt);
        }
        typeSelect.value = BLOCK_TYPES[0];
        var indexInput = el('input');
        indexInput.type = 'text';
        indexInput.setAttribute('data-plan-new-index', '');
        indexInput.setAttribute('inputmode', 'numeric');
        indexInput.value = String(blocks.length);
        insert.appendChild(labeled('מזהה (Id)', idInput));
        insert.appendChild(labeled('סוג (Type)', typeSelect));
        insert.appendChild(labeled('מקום (Index)', indexInput));
        insert.appendChild(button('data-plan-insert', '', 'הוספה (Insert)', !state.lessonPath));
        rootEl.appendChild(insert);

        var fields = el('div');
        fields.className = 'card';
        fields.setAttribute('data-plan-fields', '');
        var fi;
        for (fi = 0; fi < state.fields.length; fi++) {
            var entry = state.fields[fi];
            var labelEl = el('label');
            var caption = el('span');
            caption.textContent = entry.caption;
            var area = el('textarea');
            area.value = entry.loaded;
            area.setAttribute('data-plan-field', entry.field);
            area.setAttribute('data-plan-field-index', String(fi));
            area.rows = 3;
            var save = button('data-plan-save-field', String(fi), 'שמירת שדה (Save field)', false);
            save.className = 'btn btn-primary';
            labelEl.appendChild(caption);
            labelEl.appendChild(area);
            labelEl.appendChild(save);
            fields.appendChild(labelEl);
        }
        rootEl.appendChild(fields);

        if (selected && selected.block && selected.block.type === 'introduction') {
            paintPictures(rootEl, selected.block);
        }

        paintMedia(rootEl);
        paintValidation(rootEl);

        var preview = el('div');
        preview.className = 'plan-page';
        preview.setAttribute('data-plan-preview', '');
        preview.setAttribute('aria-label', 'תצוגה מקדימה');
        if (selected && root.HebrewPlanRender && typeof root.HebrewPlanRender.renderBlock === 'function') {
            var html = root.HebrewPlanRender.renderBlock(selected.block, { lesson: lesson });
            // Renderer output only. Plan text is escaped there. This does not eval it.
            if (typeof html === 'string') preview.innerHTML = html;
        }
        rootEl.appendChild(preview);

        panel.appendChild(rootEl);
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
        var save = attrNode(node, 'data-plan-save-field');
        if (save) return saveField(save);
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
        var node = targetNode(ev);
        if (node && attrNode(node, 'data-plan-confirm')) syncGenerate();
    }

    function onInput(ev) {
        if (!state || !state.panel) return;
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

    function reload() {
        hideBox('data-plan-conflict');
        return loadPlan();
    }

    function publish() {
        if (!Number.isInteger(state.revision)) return Promise.resolve();
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/publish', {});
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
        return write('POST', '/api/plans/' + encodeURIComponent(state.planId) + '/blocks', {
            path: state.lessonPath,
            index: index,
            block: block
        });
    }

    function saveField(node) {
        var index = Number(node.getAttribute('data-plan-save-field'));
        var entry = state.fields && state.fields[index];
        if (!entry || !FIELD_OK[entry.field] || !PART_PATH.test(entry.path)) return Promise.resolve();
        var areas = state.panel.querySelectorAll('textarea');
        var area = null;
        var i;
        for (i = 0; i < areas.length; i++) {
            if (!areas[i].hasAttribute('data-plan-field-index')) continue;
            if (Number(areas[i].getAttribute('data-plan-field-index')) === index) area = areas[i];
        }
        if (!area) return Promise.resolve();
        var value = area.value;
        if (entry.numeric) {
            if (!/^\d+$/.test(value)) {
                showBox('data-plan-error', 'invalid number');
                return Promise.resolve();
            }
            value = Number(value);
        }
        return write('PATCH', '/api/plans/' + encodeURIComponent(state.planId) + '/fields', {
            path: entry.path,
            field: entry.field,
            value: value
        });
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
     * @param {string} method
     * @param {string} path
     * @param {object} body
     * @returns {Promise<boolean>}
     */
    function write(method, path, body) {
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
            if (res && res.status === 409) {
                showConflict(res.data || {});
                return false;
            }
            if (res && res.status === 400 && res.data && res.data.error === 'validation') {
                state.validation = findingsFrom(res.data);
                state.publishRefused = path.indexOf('/publish') !== -1;
                state.publishRefusal = typeof res.data.error === 'string' ? res.data.error : '';
                hideBox('data-plan-conflict');
                paint();
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
            hideBox('data-plan-conflict');
            if (state.lessonPath) {
                return loadLesson(state.lessonPath).then(function () { return true; });
            }
            paint();
            return true;
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
            state.lessonChoices = listLessons(state.plan);
            if (state.lessonChoices.length === 1) return loadLesson(state.lessonChoices[0].path);
            state.lesson = null;
            state.lessonPath = '';
            paint();
        }).catch(function (err) {
            if (!alive(mine)) return;
            paintMessage((err && err.message) || 'request failed');
        });
    }

    function isDirty() {
        if (!state || !state.panel || !state.fields) return false;
        var areas = state.panel.querySelectorAll('textarea');
        var i;
        for (i = 0; i < areas.length; i++) {
            if (!areas[i].hasAttribute('data-plan-field-index')) continue;
            var index = Number(areas[i].getAttribute('data-plan-field-index'));
            var entry = state.fields[index];
            if (entry && areas[i].value !== entry.loaded) return true;
        }
        return false;
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
            fields: [],
            revision: null,
            busy: false,
            media: [],
            estimateShown: false,
            estimateDollars: null,
            estimatePrompt: '',
            validation: null,
            publishRefused: false,
            publishRefusal: ''
        };
        if (!panel || !SLUG.test(planId)) {
            if (panel) paintMessage('invalid plan id');
            return Promise.resolve();
        }
        panel.removeEventListener('click', onClick);
        panel.removeEventListener('change', onChange);
        panel.removeEventListener('input', onInput);
        panel.addEventListener('click', onClick);
        panel.addEventListener('change', onChange);
        panel.addEventListener('input', onInput);
        return track(loadPlan());
    }

    /**
     * Ask for a plan id. Does not call the lesson worksheet API.
     * @param {HTMLElement} panel
     * @returns {void}
     */
    function mountPicker(panel) {
        unmount();
        if (!panel) return;
        empty(panel);
        var box = el('div');
        box.className = 'card plan-builder';
        box.setAttribute('data-plan-builder', 'picker');
        box.setAttribute('data-admin-view', 'plans');
        box.setAttribute('dir', 'rtl');
        box.setAttribute('lang', 'he');
        var heading = el('h2');
        heading.textContent = 'תוכנית (Plan)';
        var input = el('input');
        input.type = 'text';
        input.setAttribute('data-plan-id', '');
        input.setAttribute('autocomplete', 'off');
        var open = button('data-plan-open', '', 'פתיחה (Open)', false);
        var err = messageNode('data-plan-error');
        box.appendChild(heading);
        box.appendChild(input);
        box.appendChild(open);
        box.appendChild(err);
        panel.appendChild(box);
        panel.addEventListener('click', function (ev) {
            var node = targetNode(ev);
            if (!node || !attrNode(node, 'data-plan-open')) return;
            var id = String(input.value || '').trim();
            if (!SLUG.test(id)) {
                err.hidden = false;
                err.textContent = 'invalid plan id';
                return;
            }
            if (root.HebrewAdminShell && typeof root.HebrewAdminShell.setHash === 'function') {
                root.HebrewAdminShell.setHash('plan/' + id);
            }
        });
    }

    function unmount() {
        token += 1;
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
        BLOCK_TYPES: BLOCK_TYPES,
        SLUG: SLUG,
        canLeave: canLeave,
        fieldsForBlock: fieldsForBlock,
        mount: mount,
        mountPicker: mountPicker,
        moveDestination: moveDestination,
        newBlock: newBlock,
        settled: settled,
        unmount: unmount
    };
})(typeof window !== 'undefined' ? window : globalThis);
