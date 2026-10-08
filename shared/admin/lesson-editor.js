/**
 * Admin lesson editor — builds worksheet objects in memory, never JS source.
 * Provenance: INIT-002/SPEC-017
 */
(function (root) {
    'use strict';

    var U = root.HebrewUtils;
    var META_KEYS = {
        id: 1, level: 1, title: 1, subtitle: 1, footer: 1,
        activities: 1, vocab: 1
    };
    var KNOWN_JSON_SECTIONS = ['fill', 'grammar', 'reading', 'askFriend'];
    var CORE_VOCAB = ['word', 'sound', 'translit', 'meaning', 'pos'];
    var EXTRA_VOCAB = ['trap', 'sentence', 'infinitive', 'from', 'audio', 'sentenceAudio', 'id'];

    function esc(value) {
        return U && typeof U.escapeHtml === 'function'
            ? U.escapeHtml(value)
            : String(value == null ? '' : value)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
    }

    function bi(he, en) {
        return U && typeof U.bi === 'function'
            ? U.bi(he, en)
            : (esc(he) + ' <span class="en" dir="ltr">(' + esc(en) + ')</span>');
    }

    function api() {
        return root.HebrewApi;
    }

    function nfc(value) {
        var s = value == null ? '' : String(value);
        try { return s.normalize('NFC'); } catch (err) { return s; }
    }

    function deepClone(obj) {
        return JSON.parse(JSON.stringify(obj == null ? null : obj));
    }

    function stableStringify(obj) {
        return JSON.stringify(obj);
    }

    function activityCatalog() {
        if (root.HebrewActivities && typeof root.HebrewActivities.list === 'function') {
            return root.HebrewActivities.list() || [];
        }
        return root.HEBREW_ACTIVITY_CATALOG || [];
    }

    function activityMeta(type) {
        var list = activityCatalog();
        for (var i = 0; i < list.length; i += 1) {
            if (list[i] && list[i].type === type) return list[i];
        }
        return null;
    }

    function getFocusable(container) {
        if (!container || !container.querySelectorAll) return [];
        var nodes = container.querySelectorAll(
            'a[href], button:not([disabled]), textarea:not([disabled]),'
            + ' input:not([disabled]):not([type="hidden"]), select:not([disabled]),'
            + ' [tabindex]:not([tabindex="-1"])'
        );
        return Array.prototype.slice.call(nodes);
    }

    function openDialog(dialog, trigger, opts) {
        if (root.HebrewAdminUsers && typeof root.HebrewAdminUsers.openDialog === 'function') {
            return root.HebrewAdminUsers.openDialog(dialog, trigger, opts);
        }
        var options = opts || {};
        if (!dialog) return { close: function () {} };
        dialog.classList.remove('hidden');
        dialog.hidden = false;
        dialog.setAttribute('aria-hidden', 'false');
        var previouslyFocused = trigger || (root.document && root.document.activeElement) || null;
        var focusables = getFocusable(dialog);
        if (focusables[0] && focusables[0].focus) {
            try { focusables[0].focus(); } catch (err) { /* ignore */ }
        }
        function onKey(event) {
            if (event.key === 'Escape') {
                event.preventDefault();
                close();
                return;
            }
            if (event.key !== 'Tab') return;
            focusables = getFocusable(dialog);
            if (!focusables.length) return;
            var firstEl = focusables[0];
            var lastEl = focusables[focusables.length - 1];
            var active = root.document && root.document.activeElement;
            if (event.shiftKey) {
                if (active === firstEl || !dialog.contains(active)) {
                    event.preventDefault();
                    lastEl.focus();
                }
            } else if (active === lastEl) {
                event.preventDefault();
                firstEl.focus();
            }
        }
        function onClick(event) {
            if (event.target === dialog) close();
        }
        function close() {
            dialog.classList.add('hidden');
            dialog.hidden = true;
            dialog.setAttribute('aria-hidden', 'true');
            dialog.removeEventListener('keydown', onKey);
            dialog.removeEventListener('click', onClick);
            if (previouslyFocused && previouslyFocused.focus) {
                try { previouslyFocused.focus(); } catch (err2) { /* ignore */ }
            }
            if (typeof options.onClose === 'function') options.onClose();
        }
        dialog.addEventListener('keydown', onKey);
        dialog.addEventListener('click', onClick);
        return { close: close };
    }

    /* ---------- pure model helpers (unit-tested) ---------- */

    function blankVocabItem() {
        return {
            id: String(Date.now()),
            word: '',
            sound: '',
            translit: '',
            meaning: '',
            pos: '',
            trap: '',
            sentence: ''
        };
    }

    function addVocabItem(items, item) {
        var next = (items || []).slice();
        next.push(item || blankVocabItem());
        return next;
    }

    function removeVocabItem(items, index) {
        var next = (items || []).slice();
        if (index < 0 || index >= next.length) return next;
        next.splice(index, 1);
        return next;
    }

    function moveVocabItem(items, index, delta) {
        var next = (items || []).slice();
        var target = index + delta;
        if (index < 0 || index >= next.length || target < 0 || target >= next.length) {
            return next;
        }
        var tmp = next[index];
        next[index] = next[target];
        next[target] = tmp;
        return next;
    }

    function addActivity(activities, entry) {
        var next = (activities || []).slice();
        next.push(entry && entry.type ? deepClone(entry) : { type: 'words' });
        return next;
    }

    function removeActivity(activities, index) {
        var next = (activities || []).slice();
        if (index < 0 || index >= next.length) return next;
        next.splice(index, 1);
        return next;
    }

    function moveActivity(activities, index, delta) {
        var next = (activities || []).slice();
        var target = index + delta;
        if (index < 0 || index >= next.length || target < 0 || target >= next.length) {
            return next;
        }
        var tmp = next[index];
        next[index] = next[target];
        next[target] = tmp;
        return next;
    }

    function parseJsonSection(text) {
        var raw = String(text == null ? '' : text).trim();
        if (!raw) return { ok: true, value: null, error: null };
        try {
            return { ok: true, value: JSON.parse(raw), error: null };
        } catch (err) {
            return { ok: false, value: null, error: (err && err.message) || 'Invalid JSON' };
        }
    }

    function sectionKeyFromError(message) {
        var msg = String(message || '');
        if (/activities/i.test(msg)) return 'activities';
        if (/vocab/i.test(msg)) return 'vocab';
        if (/fill/i.test(msg)) return 'fill';
        if (/grammar/i.test(msg)) return 'grammar';
        if (/reading|story|retell|paragraph/i.test(msg)) return 'reading';
        if (/askFriend|ask-friend|speaking/i.test(msg)) return 'askFriend';
        if (/catalog|title|subtitle|blurb|level|footer/i.test(msg)) return 'metadata';
        return 'general';
    }

    /* ---------- render ---------- */

    function renderVocabRow(item, index) {
        var it = item || {};
        var heAttrs = ' dir="rtl" lang="he"';
        var core = ''
            + '<td><input type="text" data-vocab-field="word" data-vocab-index="' + index + '"'
            + heAttrs + ' value="' + esc(it.word || '') + '" aria-label="word"></td>'
            + '<td><input type="text" data-vocab-field="sound" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.sound || '') + '" aria-label="sound"></td>'
            + '<td><input type="text" data-vocab-field="translit" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.translit || '') + '" aria-label="translit"></td>'
            + '<td><input type="text" data-vocab-field="meaning" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.meaning || '') + '" aria-label="meaning"></td>'
            + '<td><input type="text" data-vocab-field="pos" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.pos || '') + '" aria-label="pos"></td>';
        var actions = ''
            + '<td class="lesson-editor-row-actions">'
            + '<button type="button" class="btn admin-row-btn" data-vocab-move="up" data-vocab-index="'
            + index + '">' + bi('למעלה', 'Up') + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-vocab-move="down" data-vocab-index="'
            + index + '">' + bi('למטה', 'Down') + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-vocab-remove="' + index + '">'
            + bi('הסרה', 'Remove') + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-vocab-expand="' + index + '"'
            + ' aria-expanded="false">' + bi('עוד', 'More') + '</button>'
            + '</td>';
        var extra = ''
            + '<tr class="lesson-vocab-extra hidden" data-vocab-extra="' + index + '" hidden>'
            + '<td colspan="6">'
            + '<div class="lesson-vocab-extra-grid">'
            + '<label>' + bi('מלכודת', 'Trap')
            + '<input type="text" data-vocab-field="trap" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.trap || '') + '"></label>'
            + '<label>' + bi('משפט', 'Sentence')
            + '<input type="text" data-vocab-field="sentence" data-vocab-index="' + index + '"'
            + heAttrs + ' value="' + esc(it.sentence || '') + '"></label>'
            + '<label>' + bi('צורת מקור', 'Infinitive')
            + '<input type="text" data-vocab-field="infinitive" data-vocab-index="' + index + '"'
            + heAttrs + ' value="' + esc(it.infinitive || '') + '"></label>'
            + '<label>from'
            + '<input type="text" data-vocab-field="from" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.from || '') + '"></label>'
            + '<label>audio'
            + '<input type="text" data-vocab-field="audio" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.audio || '') + '"></label>'
            + '<label>sentenceAudio'
            + '<input type="text" data-vocab-field="sentenceAudio" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.sentenceAudio || '') + '"></label>'
            + '<label>id'
            + '<input type="text" data-vocab-field="id" data-vocab-index="' + index + '"'
            + ' dir="ltr" value="' + esc(it.id || '') + '"></label>'
            + '</div></td></tr>';
        return '<tr data-vocab-row="' + index + '">' + core + actions + '</tr>' + extra;
    }

    function renderVocabTable(items) {
        var rows = (items || []).map(renderVocabRow).join('');
        return ''
            + '<section class="card lesson-editor-section" data-section="vocab" id="lesson-section-vocab">'
            + '<div class="admin-users-head">'
            + '<h3>' + bi('אוצר מילים', 'Vocabulary') + '</h3>'
            + '<button type="button" class="btn btn-primary" data-vocab-add>'
            + bi('הוספת מילה', 'Add word') + '</button>'
            + '</div>'
            + '<div class="admin-table-wrap">'
            + '<table class="lesson-vocab-table">'
            + '<thead><tr>'
            + '<th>word</th><th>sound</th><th>translit</th><th>meaning</th><th>pos</th><th></th>'
            + '</tr></thead>'
            + '<tbody data-vocab-body>' + rows + '</tbody>'
            + '</table></div></section>';
    }

    function activityTypeOptions(selected) {
        var list = activityCatalog();
        var known = false;
        var html = list.map(function (meta) {
            var t = meta.type;
            if (t === selected) known = true;
            return '<option value="' + esc(t) + '"' + (t === selected ? ' selected' : '') + '>'
                + esc((meta.labelHe || t) + ' / ' + (meta.labelEn || t))
                + '</option>';
        }).join('');
        // Keep an unknown type visible so Validate can report it (ac-3).
        if (selected && !known) {
            html = '<option value="' + esc(selected) + '" selected>' + esc(selected) + '</option>' + html;
        }
        return html;
    }

    function renderActivityOptions(entry, index) {
        var type = entry && entry.type;
        var meta = activityMeta(type);
        var fields = (meta && meta.fields) || {};
        var html = '';
        if (Object.prototype.hasOwnProperty.call(fields, 'seconds') || entry.seconds != null) {
            html += '<label class="lesson-act-opt">seconds '
                + '<input type="number" min="0" data-act-field="seconds" data-act-index="'
                + index + '" value="' + esc(entry.seconds != null ? entry.seconds : '') + '"></label>';
        }
        if (Object.prototype.hasOwnProperty.call(fields, 'count') || entry.count != null) {
            html += '<label class="lesson-act-opt">count '
                + '<input type="number" min="0" data-act-field="count" data-act-index="'
                + index + '" value="' + esc(entry.count != null ? entry.count : '') + '"></label>';
        }
        if (Object.prototype.hasOwnProperty.call(fields, 'from') || entry.from != null) {
            html += '<label class="lesson-act-opt">from '
                + '<select data-act-field="from" data-act-index="' + index + '">'
                + '<option value="story"' + (entry.from !== 'vocab' ? ' selected' : '') + '>story</option>'
                + '<option value="vocab"' + (entry.from === 'vocab' ? ' selected' : '') + '>vocab</option>'
                + '</select></label>';
        }
        if (Object.prototype.hasOwnProperty.call(fields, 'mode') || entry.mode != null) {
            html += '<label class="lesson-act-opt">mode '
                + '<select data-act-field="mode" data-act-index="' + index + '">'
                + '<option value="meaning"' + (entry.mode !== 'form' ? ' selected' : '') + '>meaning</option>'
                + '<option value="form"' + (entry.mode === 'form' ? ' selected' : '') + '>form</option>'
                + '</select></label>';
        }
        return html;
    }

    function renderActivityRow(entry, index) {
        var type = (entry && entry.type) || 'words';
        return '<li class="lesson-act-row" data-act-row="' + index + '">'
            + '<select data-act-type="' + index + '" aria-label="activity type">'
            + activityTypeOptions(type)
            + '</select>'
            + '<span class="lesson-act-opts">' + renderActivityOptions(entry || {}, index) + '</span>'
            + '<span class="lesson-editor-row-actions">'
            + '<button type="button" class="btn admin-row-btn" data-act-move="up" data-act-index="'
            + index + '">' + bi('למעלה', 'Up') + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-act-move="down" data-act-index="'
            + index + '">' + bi('למטה', 'Down') + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-act-remove="' + index + '">'
            + bi('הסרה', 'Remove') + '</button>'
            + '</span></li>';
    }

    function renderActivityPath(activities) {
        var rows = (activities || []).map(renderActivityRow).join('');
        return ''
            + '<section class="card lesson-editor-section" data-section="activities" id="lesson-section-activities">'
            + '<div class="admin-users-head">'
            + '<h3>' + bi('מסלול פעילויות', 'Activity path') + '</h3>'
            + '<button type="button" class="btn btn-primary" data-act-add>'
            + bi('הוספת פעילות', 'Add activity') + '</button>'
            + '</div>'
            + '<ol class="lesson-act-list" data-act-list>' + rows + '</ol>'
            + '</section>';
    }

    function renderJsonSection(key, value) {
        var text = value == null ? '' : JSON.stringify(value, null, 2);
        return ''
            + '<section class="card lesson-editor-section" data-section="' + esc(key)
            + '" id="lesson-section-' + esc(key) + '">'
            + '<h3>' + esc(key) + '</h3>'
            + '<textarea class="lesson-json-area" data-json-section="' + esc(key) + '"'
            + ' dir="ltr" spellcheck="false" rows="10">' + esc(text) + '</textarea>'
            + '<p class="lesson-json-feedback" data-json-feedback="' + esc(key) + '" aria-live="polite"></p>'
            + '</section>';
    }

    function collectJsonSectionKeys(worksheet) {
        var keys = [];
        var seen = {};
        KNOWN_JSON_SECTIONS.forEach(function (k) {
            keys.push(k);
            seen[k] = true;
        });
        Object.keys(worksheet || {}).forEach(function (k) {
            if (META_KEYS[k] || seen[k]) return;
            if (worksheet[k] && typeof worksheet[k] === 'object') {
                keys.push(k);
                seen[k] = true;
            }
        });
        return keys;
    }

    function audioDialogHtml() {
        return ''
            + '<div class="admin-dialog hidden" id="lesson-audio-dialog" role="dialog" aria-modal="true"'
            + ' aria-labelledby="lesson-audio-title" hidden aria-hidden="true">'
            + '<div class="admin-dialog-panel lesson-audio-panel" role="document">'
            + '<h2 id="lesson-audio-title">' + bi('יצירת שמע', 'Generate audio') + '</h2>'
            + '<div data-audio-plan-body><p>' + bi('טוען תוכנית…', 'Loading plan…') + '</p></div>'
            + '<p class="admin-privacy-note" data-audio-credit-warn>'
            + bi(
                'יצירת שמע צורכת קרדיט של ElevenLabs.',
                'Audio generation spends ElevenLabs credit.'
            )
            + '</p>'
            + '<p class="admin-inline-error" data-audio-error hidden></p>'
            + '<p data-audio-result aria-live="polite"></p>'
            + '<div class="admin-dialog-actions">'
            + '<button type="button" class="btn btn-primary" data-audio-confirm disabled>'
            + bi('אישור יצירה', 'Confirm generate') + '</button>'
            + '<button type="button" class="btn" data-audio-cancel>' + bi('ביטול', 'Cancel') + '</button>'
            + '</div></div></div>';
    }

    function renderEditorShell(state) {
        var ws = state.worksheet || {};
        var cat = state.catalogEntry || {};
        var openedLabel = state.openedAtLocal || '';
        var jsonKeys = collectJsonSectionKeys(ws);
        var jsonHtml = jsonKeys.map(function (key) {
            return renderJsonSection(key, ws[key]);
        }).join('');

        return ''
            + '<div class="lesson-editor" data-admin-view="lesson" data-lesson-slug="'
            + esc(state.slug) + '" dir="rtl" lang="he">'
            + '<p class="admin-back"><a href="#lessons">' + bi('← חזרה לשיעורים', '← Back to lessons')
            + '</a></p>'
            + '<div class="card lesson-editor-toolbar">'
            + '<h2>' + bi('עריכת שיעור', 'Edit lesson') + ' <span class="en" dir="ltr">('
            + esc(state.slug) + ')</span></h2>'
            + '<p class="lesson-opened-at" data-opened-at>'
            + bi('עותק זה נפתח ב־', 'When this copy was opened:') + ' '
            + '<time dir="ltr">' + esc(openedLabel) + '</time></p>'
            + '<div class="lesson-editor-actions">'
            + '<button type="button" class="btn" data-lesson-validate>'
            + bi('בדיקה', 'Validate') + '</button> '
            + '<button type="button" class="btn btn-primary" data-lesson-save disabled>'
            + bi('שמירה', 'Save') + '</button> '
            + '<button type="button" class="btn" data-lesson-audio>'
            + bi('שמע', 'Audio') + '</button>'
            + '</div>'
            + '<p class="admin-inline-error" data-editor-error hidden></p>'
            + '<div class="lesson-validate-out" data-validate-out aria-live="polite"></div>'
            + '<p class="lesson-save-result" data-save-result aria-live="polite"></p>'
            + '</div>'

            + '<section class="card lesson-editor-section" data-section="metadata" id="lesson-section-metadata">'
            + '<h3>' + bi('מטא־נתונים', 'Metadata') + '</h3>'
            + '<div class="lesson-meta-grid">'
            + '<label>' + bi('כותרת', 'Title')
            + '<input type="text" data-meta="title" dir="rtl" lang="he" value="'
            + esc(ws.title || cat.title || '') + '"></label>'
            + '<label>' + bi('כותרת משנה', 'Subtitle')
            + '<input type="text" data-meta="subtitle" dir="rtl" lang="he" value="'
            + esc(ws.subtitle || cat.subtitle || '') + '"></label>'
            + '<label>' + bi('רמה', 'Level')
            + '<select data-meta="level">'
            + '<option value="basic"' + ((ws.level || cat.level) === 'basic' ? ' selected' : '') + '>basic</option>'
            + '<option value="intermediate"' + ((ws.level || cat.level) === 'intermediate' ? ' selected' : '') + '>intermediate</option>'
            + '<option value="advanced"' + ((ws.level || cat.level) === 'advanced' ? ' selected' : '') + '>advanced</option>'
            + '</select></label>'
            + '<label>' + bi('כותרת תחתונה', 'Footer')
            + '<input type="text" data-meta="footer" dir="rtl" lang="he" value="'
            + esc(ws.footer || '') + '"></label>'
            + '<label class="lesson-meta-blurb">' + bi('תקציר בקטלוג', 'Catalog blurb')
            + '<textarea data-meta="blurb" dir="ltr" rows="3">'
            + esc(cat.blurb || '') + '</textarea></label>'
            + '</div></section>'

            + renderVocabTable((ws.vocab && ws.vocab.items) || [])
            + renderActivityPath(ws.activities || [])
            + jsonHtml
            + audioDialogHtml()
            + '</div>';
    }

    function setSaveEnabled(rootEl, enabled) {
        var btn = rootEl.querySelector('[data-lesson-save]');
        if (!btn) return;
        btn.disabled = !enabled;
        if (enabled) btn.removeAttribute('disabled');
        else btn.setAttribute('disabled', '');
    }

    function showEditorError(rootEl, message) {
        var el = rootEl.querySelector('[data-editor-error]');
        if (!el) return;
        el.hidden = false;
        el.removeAttribute('hidden');
        el.textContent = message || '';
    }

    function clearEditorError(rootEl) {
        var el = rootEl.querySelector('[data-editor-error]');
        if (!el) return;
        el.hidden = true;
        el.setAttribute('hidden', '');
        el.textContent = '';
    }

    function renderValidationOut(rootEl, result) {
        var host = rootEl.querySelector('[data-validate-out]');
        if (!host) return;
        var errors = (result && result.errors) || [];
        var warnings = (result && result.warnings) || [];
        var html = '';
        if (errors.length) {
            html += '<div class="lesson-validate-errors"><strong>'
                + bi('שגיאות חוסמות', 'Blocking errors') + '</strong><ul>';
            errors.forEach(function (msg) {
                var section = sectionKeyFromError(msg);
                html += '<li data-val-section="' + esc(section) + '">'
                    + '<a href="#lesson-section-' + esc(section) + '">' + esc(section) + '</a>: '
                    + esc(msg) + '</li>';
            });
            html += '</ul></div>';
        }
        if (warnings.length) {
            html += '<div class="lesson-validate-warnings"><strong>'
                + bi('אזהרות', 'Warnings') + '</strong><ul>';
            warnings.forEach(function (msg) {
                html += '<li>' + esc(msg) + '</li>';
            });
            html += '</ul></div>';
        }
        if (!errors.length && result && result.ok) {
            html += '<p class="lesson-validate-ok">' + bi('הבדיקה עברה.', 'Validation passed.') + '</p>';
        }
        host.innerHTML = html;
    }

    /**
     * Read form into worksheet + catalogEntry. Mutates state.worksheet.
     * @returns {{ ok: boolean, error?: string }}
     */
    function readFormIntoState(rootEl, state) {
        var ws = deepClone(state.worksheet) || {};
        var cat = deepClone(state.catalogEntry) || { slug: state.slug };

        var titleEl = rootEl.querySelector('[data-meta="title"]');
        var subtitleEl = rootEl.querySelector('[data-meta="subtitle"]');
        var levelEl = rootEl.querySelector('[data-meta="level"]');
        var footerEl = rootEl.querySelector('[data-meta="footer"]');
        var blurbEl = rootEl.querySelector('[data-meta="blurb"]');

        ws.title = nfc(titleEl && titleEl.value);
        ws.subtitle = nfc(subtitleEl && subtitleEl.value);
        ws.level = String(levelEl && levelEl.value || 'basic');
        ws.footer = nfc(footerEl && footerEl.value);
        ws.id = state.slug;

        cat.slug = state.slug;
        cat.title = ws.title;
        cat.subtitle = ws.subtitle;
        cat.level = ws.level;
        cat.blurb = String(blurbEl && blurbEl.value || '');

        var items = [];
        var rows = rootEl.querySelectorAll('[data-vocab-row]');
        Array.prototype.forEach.call(rows, function (row) {
            var index = Number(row.getAttribute('data-vocab-row'));
            var item = {};
            var fields = rootEl.querySelectorAll('[data-vocab-field][data-vocab-index="' + index + '"]');
            Array.prototype.forEach.call(fields, function (input) {
                var field = input.getAttribute('data-vocab-field');
                var val = input.value;
                if (field === 'word' || field === 'sentence' || field === 'infinitive') {
                    val = nfc(val);
                }
                item[field] = val;
            });
            items.push(item);
        });
        ws.vocab = ws.vocab && typeof ws.vocab === 'object' ? ws.vocab : {};
        ws.vocab.items = items;

        var activities = [];
        var actRows = rootEl.querySelectorAll('[data-act-row]');
        Array.prototype.forEach.call(actRows, function (row) {
            var index = Number(row.getAttribute('data-act-row'));
            var typeSel = row.querySelector('[data-act-type]');
            var entry = { type: typeSel ? typeSel.value : 'words' };
            var optInputs = row.querySelectorAll('[data-act-field]');
            Array.prototype.forEach.call(optInputs, function (input) {
                var field = input.getAttribute('data-act-field');
                var raw = input.value;
                if (field === 'seconds' || field === 'count') {
                    if (raw === '' || raw == null) return;
                    var n = Number(raw);
                    if (Number.isFinite(n)) entry[field] = n;
                } else if (raw) {
                    entry[field] = raw;
                }
            });
            activities.push(entry);
        });
        ws.activities = activities;

        var jsonAreas = rootEl.querySelectorAll('[data-json-section]');
        var jsonError = null;
        Array.prototype.forEach.call(jsonAreas, function (area) {
            var key = area.getAttribute('data-json-section');
            var feedback = rootEl.querySelector('[data-json-feedback="' + key + '"]');
            var parsed = parseJsonSection(area.value);
            if (!parsed.ok) {
                if (feedback) feedback.textContent = parsed.error;
                if (!jsonError) jsonError = key + ': ' + parsed.error;
                return;
            }
            if (feedback) feedback.textContent = parsed.value == null ? '' : 'JSON ✓';
            if (parsed.value == null) {
                delete ws[key];
            } else {
                ws[key] = parsed.value;
            }
        });

        if (jsonError) {
            return { ok: false, error: jsonError };
        }

        state.worksheet = ws;
        state.catalogEntry = cat;
        return { ok: true };
    }

    function markDirty(state) {
        state.dirty = true;
        state.validationClean = false;
    }

    function rebuildVocabDom(rootEl, state) {
        var body = rootEl.querySelector('[data-vocab-body]');
        if (!body) return;
        var items = (state.worksheet.vocab && state.worksheet.vocab.items) || [];
        body.innerHTML = items.map(renderVocabRow).join('');
    }

    function rebuildActDom(rootEl, state) {
        var list = rootEl.querySelector('[data-act-list]');
        if (!list) return;
        list.innerHTML = (state.worksheet.activities || []).map(renderActivityRow).join('');
    }

    function syncStateFromDom(rootEl, state) {
        readFormIntoState(rootEl, state);
    }

    /**
     * @param {HTMLElement} panel
     * @param {string} slug
     */
    function mount(panel, slug) {
        if (!panel || !slug) return;
        var state = {
            slug: slug,
            worksheet: null,
            catalogEntry: null,
            baselineJson: null,
            openedAtLocal: '',
            dirty: false,
            validationClean: false,
            dialogCtl: null,
            audioCalls: 0
        };

        panel.innerHTML = '<div class="card" data-admin-view="lesson"><p>'
            + bi('טוען…', 'Loading…') + '</p></div>';

        function paint() {
            panel.innerHTML = renderEditorShell(state);
            wire(panel);
            setSaveEnabled(panel, false);
        }

        function confirmLeave() {
            if (!state.dirty) return true;
            try {
                return root.confirm(
                    'יש שינויים שלא נשמרו. לצאת בכל זאת? (You have unsaved changes. Leave anyway?)'
                );
            } catch (err) {
                return true;
            }
        }

        function onBeforeUnload(event) {
            if (!state.dirty) return;
            event.preventDefault();
            event.returnValue = '';
        }

        function wire(rootEl) {
            if (root.addEventListener) {
                root.addEventListener('beforeunload', onBeforeUnload);
            }

            rootEl.addEventListener('input', function () {
                markDirty(state);
                setSaveEnabled(rootEl, false);
                var area = arguments[0].target;
                if (area && area.getAttribute && area.getAttribute('data-json-section')) {
                    var key = area.getAttribute('data-json-section');
                    var feedback = rootEl.querySelector('[data-json-feedback="' + key + '"]');
                    var parsed = parseJsonSection(area.value);
                    if (feedback) {
                        feedback.textContent = parsed.ok
                            ? (parsed.value == null ? '' : 'JSON ✓')
                            : parsed.error;
                    }
                }
            });
            rootEl.addEventListener('change', function (event) {
                markDirty(state);
                setSaveEnabled(rootEl, false);
                var target = event.target;
                if (target && target.getAttribute && target.getAttribute('data-act-type') != null) {
                    syncStateFromDom(rootEl, state);
                    var idx = Number(target.getAttribute('data-act-type'));
                    if (state.worksheet.activities && state.worksheet.activities[idx]) {
                        state.worksheet.activities[idx] = { type: target.value };
                    }
                    rebuildActDom(rootEl, state);
                }
            });

            rootEl.addEventListener('click', function (event) {
                var target = event.target;
                while (target && target !== rootEl && !(target.getAttribute && (
                    target.hasAttribute('data-vocab-add')
                    || target.hasAttribute('data-vocab-remove')
                    || target.hasAttribute('data-vocab-move')
                    || target.hasAttribute('data-vocab-expand')
                    || target.hasAttribute('data-act-add')
                    || target.hasAttribute('data-act-remove')
                    || target.hasAttribute('data-act-move')
                    || target.hasAttribute('data-lesson-validate')
                    || target.hasAttribute('data-lesson-save')
                    || target.hasAttribute('data-lesson-audio')
                    || target.hasAttribute('data-audio-cancel')
                    || target.hasAttribute('data-audio-confirm')
                ))) {
                    target = target.parentNode;
                }
                if (!target || target === rootEl) return;

                if (target.hasAttribute('data-vocab-add')) {
                    syncStateFromDom(rootEl, state);
                    state.worksheet.vocab = state.worksheet.vocab || {};
                    state.worksheet.vocab.items = addVocabItem(state.worksheet.vocab.items);
                    markDirty(state);
                    setSaveEnabled(rootEl, false);
                    rebuildVocabDom(rootEl, state);
                    return;
                }
                if (target.hasAttribute('data-vocab-remove')) {
                    syncStateFromDom(rootEl, state);
                    var ri = Number(target.getAttribute('data-vocab-remove'));
                    state.worksheet.vocab.items = removeVocabItem(state.worksheet.vocab.items, ri);
                    markDirty(state);
                    setSaveEnabled(rootEl, false);
                    rebuildVocabDom(rootEl, state);
                    return;
                }
                if (target.hasAttribute('data-vocab-move')) {
                    syncStateFromDom(rootEl, state);
                    var mi = Number(target.getAttribute('data-vocab-index'));
                    var dir = target.getAttribute('data-vocab-move') === 'up' ? -1 : 1;
                    state.worksheet.vocab.items = moveVocabItem(state.worksheet.vocab.items, mi, dir);
                    markDirty(state);
                    setSaveEnabled(rootEl, false);
                    rebuildVocabDom(rootEl, state);
                    return;
                }
                if (target.hasAttribute('data-vocab-expand')) {
                    var ei = target.getAttribute('data-vocab-expand');
                    var extra = rootEl.querySelector('[data-vocab-extra="' + ei + '"]');
                    if (!extra) return;
                    var open = extra.classList.contains('hidden') || extra.hidden;
                    if (open) {
                        extra.classList.remove('hidden');
                        extra.hidden = false;
                        target.setAttribute('aria-expanded', 'true');
                    } else {
                        extra.classList.add('hidden');
                        extra.hidden = true;
                        target.setAttribute('aria-expanded', 'false');
                    }
                    return;
                }

                if (target.hasAttribute('data-act-add')) {
                    syncStateFromDom(rootEl, state);
                    state.worksheet.activities = addActivity(state.worksheet.activities, { type: 'words' });
                    markDirty(state);
                    setSaveEnabled(rootEl, false);
                    rebuildActDom(rootEl, state);
                    return;
                }
                if (target.hasAttribute('data-act-remove')) {
                    syncStateFromDom(rootEl, state);
                    var ari = Number(target.getAttribute('data-act-remove'));
                    state.worksheet.activities = removeActivity(state.worksheet.activities, ari);
                    markDirty(state);
                    setSaveEnabled(rootEl, false);
                    rebuildActDom(rootEl, state);
                    return;
                }
                if (target.hasAttribute('data-act-move')) {
                    syncStateFromDom(rootEl, state);
                    var ami = Number(target.getAttribute('data-act-index'));
                    var adir = target.getAttribute('data-act-move') === 'up' ? -1 : 1;
                    state.worksheet.activities = moveActivity(state.worksheet.activities, ami, adir);
                    markDirty(state);
                    setSaveEnabled(rootEl, false);
                    rebuildActDom(rootEl, state);
                    return;
                }

                if (target.hasAttribute('data-lesson-validate')) {
                    clearEditorError(rootEl);
                    var readV = readFormIntoState(rootEl, state);
                    if (!readV.ok) {
                        showEditorError(rootEl, readV.error);
                        state.validationClean = false;
                        setSaveEnabled(rootEl, false);
                        return;
                    }
                    target.disabled = true;
                    api().request('/api/admin/lessons/' + encodeURIComponent(state.slug) + '/validate', {
                        method: 'POST',
                        body: {
                            worksheet: state.worksheet,
                            catalogEntry: state.catalogEntry
                        }
                    }).then(function (res) {
                        target.disabled = false;
                        if (!res.ok && !(res.data && Array.isArray(res.data.errors))) {
                            showEditorError(rootEl, res.error || ('HTTP ' + res.status));
                            state.validationClean = false;
                            setSaveEnabled(rootEl, false);
                            return;
                        }
                        var result = res.data || { ok: false, errors: [res.error || 'validate failed'], warnings: [] };
                        if (result.ok == null) {
                            result.ok = !(result.errors && result.errors.length);
                        }
                        renderValidationOut(rootEl, result);
                        state.validationClean = !!result.ok && !(result.errors && result.errors.length);
                        setSaveEnabled(rootEl, state.validationClean);
                    }).catch(function (err) {
                        target.disabled = false;
                        showEditorError(rootEl, (err && err.message) || 'Network error');
                        state.validationClean = false;
                        setSaveEnabled(rootEl, false);
                    });
                    return;
                }

                if (target.hasAttribute('data-lesson-save')) {
                    if (!state.validationClean) return;
                    clearEditorError(rootEl);
                    var readS = readFormIntoState(rootEl, state);
                    if (!readS.ok) {
                        showEditorError(rootEl, readS.error);
                        return;
                    }
                    target.disabled = true;
                    api().get('/api/admin/lessons/' + encodeURIComponent(state.slug)).then(function (fresh) {
                        if (fresh.ok && fresh.data && fresh.data.worksheet) {
                            var serverJson = stableStringify(fresh.data.worksheet);
                            if (state.baselineJson && serverJson !== state.baselineJson) {
                                var proceed = true;
                                try {
                                    proceed = root.confirm(
                                        'השיעור השתנה בשרת מאז שנפתח עותק זה. שמירה תדרוס. '
                                        + '(The lesson changed on the server since this copy was opened. Save will overwrite.)'
                                    );
                                } catch (errC) { /* ignore */ }
                                if (!proceed) {
                                    target.disabled = false;
                                    setSaveEnabled(rootEl, true);
                                    return Promise.reject({ cancelled: true });
                                }
                            }
                        }
                        return api().request('/api/admin/lessons/' + encodeURIComponent(state.slug), {
                            method: 'PUT',
                            body: {
                                worksheet: state.worksheet,
                                catalogEntry: state.catalogEntry
                            }
                        });
                    }).then(function (res) {
                        if (!res || res.cancelled) return;
                        target.disabled = false;
                        if (!res.ok) {
                            if (res.status === 422 && res.data) {
                                renderValidationOut(rootEl, {
                                    ok: false,
                                    errors: res.data.errors || [],
                                    warnings: res.data.warnings || []
                                });
                                state.validationClean = false;
                                setSaveEnabled(rootEl, false);
                            }
                            showEditorError(rootEl, res.error || ('HTTP ' + res.status));
                            return;
                        }
                        state.dirty = false;
                        state.baselineJson = stableStringify(state.worksheet);
                        state.validationClean = true;
                        setSaveEnabled(rootEl, true);
                        var out = rootEl.querySelector('[data-save-result]');
                        if (out) {
                            out.innerHTML = bi('נשמר.', 'Saved.')
                                + ' <a class="lesson-view-link" href="/worksheets/'
                                + encodeURIComponent(state.slug) + '/" target="_blank" rel="noopener">'
                                + bi('צפייה בשיעור', 'View lesson') + '</a>';
                        }
                    }).catch(function (err) {
                        target.disabled = false;
                        if (err && err.cancelled) return;
                        showEditorError(rootEl, (err && err.message) || 'Network error');
                    });
                    return;
                }

                if (target.hasAttribute('data-lesson-audio')) {
                    openAudioDialog(rootEl, state, target);
                    return;
                }
                if (target.hasAttribute('data-audio-cancel')) {
                    if (state.dialogCtl) state.dialogCtl.close();
                    return;
                }
                if (target.hasAttribute('data-audio-confirm')) {
                    runAudioGenerate(rootEl, state, target);
                }
            });

            // Intercept in-app navigation away from editor
            var back = rootEl.querySelector('.admin-back a');
            if (back) {
                back.addEventListener('click', function (event) {
                    if (!confirmLeave()) {
                        event.preventDefault();
                    } else if (root.removeEventListener) {
                        root.removeEventListener('beforeunload', onBeforeUnload);
                    }
                });
            }
        }

        function openAudioDialog(rootEl, st, trigger) {
            var dlg = rootEl.querySelector('#lesson-audio-dialog');
            var planBody = rootEl.querySelector('[data-audio-plan-body]');
            var errEl = rootEl.querySelector('[data-audio-error]');
            var resultEl = rootEl.querySelector('[data-audio-result]');
            var confirmBtn = rootEl.querySelector('[data-audio-confirm]');
            if (errEl) {
                errEl.hidden = true;
                errEl.textContent = '';
            }
            if (resultEl) resultEl.textContent = '';
            if (confirmBtn) confirmBtn.disabled = true;
            if (planBody) {
                planBody.innerHTML = '<p>' + bi('טוען תוכנית…', 'Loading plan…') + '</p>';
            }
            st.dialogCtl = openDialog(dlg, trigger);
            api().get('/api/admin/lessons/' + encodeURIComponent(st.slug) + '/audio-plan')
                .then(function (res) {
                    if (!res.ok) {
                        if (planBody) {
                            planBody.innerHTML = '<p class="admin-inline-error">'
                                + esc(res.error || 'Failed to load plan') + '</p>';
                        }
                        return;
                    }
                    var p = res.data || {};
                    if (planBody) {
                        planBody.innerHTML = ''
                            + '<ul class="lesson-audio-plan" data-audio-plan>'
                            + '<li data-plan="words">' + bi('מילים חדשות', 'New words') + ': '
                            + esc(String(p.wordsToGenerate || 0)) + '</li>'
                            + '<li data-plan="sentences">' + bi('משפטים', 'Sentences') + ': '
                            + esc(String(p.sentencesToGenerate || 0)) + '</li>'
                            + '<li data-plan="story">' + bi('סיפור', 'Story') + ': '
                            + esc(String(p.storyToGenerate || 0)) + '</li>'
                            + '<li data-plan="archive">' + bi('לארכיון', 'To archive') + ': '
                            + esc(String(p.clipsToArchive || 0)) + '</li>'
                            + '<li data-plan="reused-words">' + bi('מילים משומשות מחדש', 'Words reused') + ': '
                            + esc(String(p.wordsReused || 0)) + '</li>'
                            + '<li data-plan="reused-clips">' + bi('קליפים משומשים מחדש', 'Clips reused') + ': '
                            + esc(String(p.clipsReused || 0)) + '</li>'
                            + '</ul>';
                    }
                    if (confirmBtn) {
                        confirmBtn.disabled = false;
                        confirmBtn.removeAttribute('disabled');
                    }
                })
                .catch(function (err) {
                    if (planBody) {
                        planBody.innerHTML = '<p class="admin-inline-error">'
                            + esc((err && err.message) || 'Network error') + '</p>';
                    }
                });
        }

        function runAudioGenerate(rootEl, st, btn) {
            var errEl = rootEl.querySelector('[data-audio-error]');
            var resultEl = rootEl.querySelector('[data-audio-result]');
            if (errEl) {
                errEl.hidden = true;
                errEl.textContent = '';
            }
            if (resultEl) resultEl.textContent = bi('יוצר…', 'Generating…');
            if (btn) btn.disabled = true;
            st.audioCalls += 1;
            api().request('/api/admin/lessons/' + encodeURIComponent(st.slug) + '/audio', {
                method: 'POST',
                body: {}
            }).then(function (res) {
                if (btn) btn.disabled = false;
                if (res.status === 409) {
                    if (errEl) {
                        errEl.hidden = false;
                        errEl.removeAttribute('hidden');
                        errEl.textContent = bi(
                            'יצירת שמע כבר רצה לשיעור הזה.',
                            'Audio generation is already running for this lesson.'
                        );
                    }
                    if (resultEl) resultEl.textContent = '';
                    return;
                }
                if (!res.ok) {
                    if (errEl) {
                        errEl.hidden = false;
                        errEl.removeAttribute('hidden');
                        errEl.textContent = res.error || ('HTTP ' + res.status);
                    }
                    if (resultEl) resultEl.textContent = '';
                    return;
                }
                if (resultEl) {
                    var summary = (res.data && (res.data.summary || JSON.stringify(res.data))) || 'OK';
                    resultEl.textContent = String(summary);
                }
            }).catch(function (err) {
                if (btn) btn.disabled = false;
                if (errEl) {
                    errEl.hidden = false;
                    errEl.removeAttribute('hidden');
                    errEl.textContent = (err && err.message) || 'Network error';
                }
            });
        }

        api().get('/api/admin/lessons/' + encodeURIComponent(slug)).then(function (res) {
            if (!res.ok) {
                panel.innerHTML = '<div class="card" data-admin-view="lesson" role="alert"><p>'
                    + bi('לא נמצא שיעור.', 'Lesson not found.') + '</p>'
                    + '<p class="en" dir="ltr">' + esc(res.error || '') + '</p>'
                    + '<p><a href="#lessons">' + bi('חזרה', 'Back') + '</a></p></div>';
                return;
            }
            state.worksheet = deepClone(res.data.worksheet) || {};
            state.catalogEntry = deepClone(res.data.catalogEntry) || { slug: slug };
            state.baselineJson = stableStringify(state.worksheet);
            try {
                state.openedAtLocal = new Date().toLocaleString();
            } catch (err) {
                state.openedAtLocal = String(new Date());
            }
            state.dirty = false;
            state.validationClean = false;
            paint();
        }).catch(function (err) {
            panel.innerHTML = '<div class="card" data-admin-view="lesson" role="alert"><p>'
                + bi('שגיאת רשת.', 'Network error.') + '</p>'
                + '<p class="en" dir="ltr">' + esc((err && err.message) || '') + '</p></div>';
        });
    }

    // Alias used by activity catalog consumers / tests
    if (!root.HebrewActivities) {
        root.HebrewActivities = {
            list: function () { return root.HEBREW_ACTIVITY_CATALOG || []; }
        };
    }

    root.HebrewAdminLessonEditor = {
        escapeHtml: esc,
        bi: bi,
        nfc: nfc,
        blankVocabItem: blankVocabItem,
        addVocabItem: addVocabItem,
        removeVocabItem: removeVocabItem,
        moveVocabItem: moveVocabItem,
        addActivity: addActivity,
        removeActivity: removeActivity,
        moveActivity: moveActivity,
        parseJsonSection: parseJsonSection,
        sectionKeyFromError: sectionKeyFromError,
        renderVocabRow: renderVocabRow,
        renderVocabTable: renderVocabTable,
        renderActivityPath: renderActivityPath,
        renderEditorShell: renderEditorShell,
        renderValidationOut: renderValidationOut,
        setSaveEnabled: setSaveEnabled,
        openDialog: openDialog,
        CORE_VOCAB: CORE_VOCAB,
        EXTRA_VOCAB: EXTRA_VOCAB,
        mount: mount
    };
})(typeof window !== 'undefined' ? window : globalThis);
