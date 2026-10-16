/**
 * Admin lessons list + visibility controls.
 * Teaching content opens HebrewAdminLessonEditor (worksheet).
 * Plans are authored on /editor/ (INIT-010/SPEC-003).
 * Provenance: INIT-002/SPEC-017, INIT-010/SPEC-003
 */
(function (root) {
    'use strict';

    var U = root.HebrewUtils;
    var SLUG_RE = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

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
            : ('<span class="ui-pair"><span class="ui-he" lang="he">' + esc(he) + '</span>'
                + '<span class="ui-en" lang="en" dir="ltr">' + esc(en) + '</span></span>');
    }

    function api() {
        return root.HebrewApi;
    }

    function uiLang() {
        try {
            if (U && typeof U.readUiLang === 'function') return U.readUiLang();
        } catch (err) { /* ignore */ }
        return 'en';
    }

    /** A menu row shows one language. Child spans inside an option stay visible in both. */
    function optionLabel(he, en) {
        return ' data-ui-text-he="' + esc(he) + '" data-ui-text-en="' + esc(en) + '">'
            + esc(uiLang() === 'he' ? he : en);
    }

    function modeLabel(mode) {
        if (mode === 'everyone') return bi('לכולם', 'Everyone');
        if (mode === 'selected') return bi('נבחרים', 'Selected');
        return bi('מוסתר', 'Hidden');
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
        function close() {
            dialog.classList.add('hidden');
            dialog.hidden = true;
            dialog.setAttribute('aria-hidden', 'true');
            dialog.removeEventListener('keydown', onKey);
            if (previouslyFocused && previouslyFocused.focus) {
                try { previouslyFocused.focus(); } catch (err) { /* ignore */ }
            }
            if (typeof options.onClose === 'function') options.onClose();
        }
        function onKey(event) {
            if (event.key === 'Escape') {
                event.preventDefault();
                close();
            }
        }
        dialog.addEventListener('keydown', onKey);
        return { close: close };
    }

    function enrichLesson(row, detail) {
        var ws = (detail && detail.worksheet) || {};
        var cat = (detail && detail.catalogEntry) || {};
        var vis = (detail && detail.visibility) || {};
        return {
            slug: row.slug,
            mode: vis.mode || row.mode || 'hidden',
            grantCount: row.grantCount || 0,
            userIds: Array.isArray(vis.userIds) ? vis.userIds.slice() : [],
            title: cat.title || ws.title || row.slug,
            level: cat.level || ws.level || '',
            subtitle: cat.subtitle || ws.subtitle || ''
        };
    }

    function renderUserPicker(lesson, users) {
        if (lesson.mode !== 'selected') return '';
        var selected = {};
        (lesson.userIds || []).forEach(function (id) {
            selected[String(id)] = true;
        });
        var options = (users || []).filter(function (u) {
            return u && u.status !== 'disabled' && u.role !== 'admin';
        });
        if (!options.length) {
            options = (users || []).filter(function (u) {
                return u && u.status !== 'disabled';
            });
        }
        var checks = options.map(function (u) {
            var id = String(u.id);
            var checked = selected[id] ? ' checked' : '';
            return '<label class="lesson-vis-user">'
                + '<input type="checkbox" data-vis-user="' + esc(id) + '"' + checked + '> '
                + esc(u.username || '')
                + (u.display_name ? ' <span class="en" dir="ltr">(' + esc(u.display_name) + ')</span>' : '')
                + '</label>';
        }).join('');
        return '<div class="lesson-vis-users" data-vis-users="' + esc(lesson.slug) + '">'
            + (checks || ('<p class="admin-empty">' + bi('אין משתמשים פעילים.', 'No active users.') + '</p>'))
            + '</div>';
    }

    function renderLessonRow(lesson, users) {
        var slug = esc(lesson.slug);
        var mode = lesson.mode || 'hidden';
        return '<tr data-lesson-slug="' + slug + '" data-vis-mode="' + esc(mode) + '">'
            + '<td><a class="admin-user-link" href="#lesson/' + encodeURIComponent(lesson.slug) + '">'
            + esc(lesson.title || lesson.slug) + '</a></td>'
            + '<td>' + esc(lesson.level || '—') + '</td>'
            + '<td class="lesson-vis-cell">'
            + '<select class="lesson-vis-mode" data-vis-mode-select="' + slug + '" aria-label="'
            + esc('Visibility') + '">'
            + '<option value="hidden"' + (mode === 'hidden' ? ' selected' : '')
            + optionLabel('מוסתר', 'Hidden') + '</option>'
            + '<option value="everyone"' + (mode === 'everyone' ? ' selected' : '')
            + optionLabel('לכולם', 'Everyone') + '</option>'
            + '<option value="selected"' + (mode === 'selected' ? ' selected' : '')
            + optionLabel('נבחרים', 'Selected') + '</option>'
            + '</select>'
            + renderUserPicker(lesson, users)
            + '<button type="button" class="btn btn-primary admin-row-btn" data-vis-save="'
            + slug + '">' + bi('שמירת נראות', 'Save visibility') + '</button>'
            + '<a class="btn btn-ghost admin-row-btn" href="#lesson/' + encodeURIComponent(lesson.slug) + '">'
            + bi('עריכה', 'Edit') + '</a>'
            + '<button type="button" class="btn btn-ghost admin-row-btn" data-lesson-delete="'
            + slug + '">' + bi('מחיקה', 'Delete') + '</button>'
            + '<span class="lesson-vis-status" data-vis-status="' + slug + '" aria-live="polite"></span>'
            + '</td></tr>';
    }

    function renderLessonsTable(lessons, users) {
        var rows = (lessons || []).map(function (lesson) {
            return renderLessonRow(lesson, users);
        }).join('');
        if (!rows) {
            rows = '<tr data-admin-empty="1"><td colspan="3">'
                + bi('אין שיעורים.', 'No lessons.') + '</td></tr>';
        }
        return ''
            + '<div class="card admin-lessons-card" data-admin-view="lessons">'
            + '<div class="admin-users-head">'
            + '<h2>' + bi('שיעורים', 'Lessons') + '</h2>'
            + '<button type="button" class="btn btn-primary" data-lesson-action="new">'
            + bi('שיעור חדש', 'New lesson') + '</button>'
            + '</div>'
            + '<p class="admin-inline-error" data-lessons-error hidden></p>'
            + '<div class="admin-table-wrap">'
            + '<table class="admin-lessons-table">'
            + '<thead><tr>'
            + '<th>' + bi('כותרת', 'Title') + '</th>'
            + '<th>' + bi('רמה', 'Level') + '</th>'
            + '<th>' + bi('נראות', 'Visibility') + '</th>'
            + '</tr></thead>'
            + '<tbody data-admin-lessons-body>' + rows + '</tbody>'
            + '</table></div></div>';
    }

    function newLessonDialogHtml() {
        return ''
            + '<div class="admin-dialog hidden" id="admin-new-lesson-dialog" role="dialog" aria-modal="true"'
            + ' aria-labelledby="admin-new-lesson-title" hidden aria-hidden="true">'
            + '<div class="admin-dialog-panel" role="document">'
            + '<h2 id="admin-new-lesson-title">' + bi('שיעור חדש', 'New lesson') + '</h2>'
            + '<form id="admin-new-lesson-form" autocomplete="off">'
            + '<label class="hebrew-shell-field">' + bi('כותרת', 'Title')
            + '<input type="text" name="title" id="admin-new-lesson-title-input" required'
            + ' dir="rtl" lang="he"></label>'
            + '<p class="admin-inline-error" id="admin-new-lesson-error" hidden></p>'
            + '<div class="admin-dialog-actions">'
            + '<button type="submit" class="btn btn-primary">' + bi('יצירה', 'Create') + '</button>'
            + '<button type="button" class="btn" data-admin-dialog-close>' + bi('ביטול', 'Cancel') + '</button>'
            + '</div></form></div></div>';
    }

    function blankWorksheet(slug, title) {
        return {
            id: slug,
            title: title || slug,
            subtitle: '',
            footer: '',
            level: 'basic',
            activities: [],
            vocab: { items: [] }
        };
    }

    function newLessonSlug() {
        try {
            if (root.crypto && typeof root.crypto.randomUUID === 'function') {
                return root.crypto.randomUUID();
            }
        } catch (err) { /* ignore */ }
        return 'lesson-' + String(Date.now()) + '-' + String(Math.floor(Math.random() * 1e6));
    }

    function catalogEntryFor(slug, title, level) {
        return {
            slug: slug,
            title: title || slug,
            subtitle: '',
            blurb: 'Run it again. The next round hides the English until you ask, and keeps the vowels.',
            level: level || 'basic'
        };
    }

    function showError(rootEl, message) {
        var el = rootEl.querySelector('[data-lessons-error]');
        if (!el) return;
        el.hidden = false;
        el.removeAttribute('hidden');
        el.textContent = message || '';
    }

    function clearError(rootEl) {
        var el = rootEl.querySelector('[data-lessons-error]');
        if (!el) return;
        el.hidden = true;
        el.setAttribute('hidden', '');
        el.textContent = '';
    }

    function readVisibilityPayload(row) {
        var select = row.querySelector('[data-vis-mode-select]');
        var mode = select ? select.value : 'hidden';
        var userIds = [];
        if (mode === 'selected') {
            var boxes = row.querySelectorAll('[data-vis-user]:checked');
            Array.prototype.forEach.call(boxes, function (box) {
                var id = Number(box.getAttribute('data-vis-user'));
                if (Number.isInteger(id) && id > 0) userIds.push(id);
            });
        }
        return { mode: mode, userIds: userIds };
    }

    /**
     * @param {HTMLElement} panel
     */
    function mountList(panel) {
        if (!panel) return;
        var state = {
            lessons: [],
            users: [],
            dialogCtl: null
        };

        panel.innerHTML = '<div class="card" data-admin-state="loading" data-admin-view="lessons"><p>'
            + bi('טוען…', 'Loading…') + '</p></div>';

        function paint() {
            panel.innerHTML = renderLessonsTable(state.lessons, state.users) + newLessonDialogHtml();
            wire(panel);
        }

        function wire(rootEl) {
            rootEl.addEventListener('change', function (event) {
                var select = event.target && event.target.closest
                    ? event.target.closest('[data-vis-mode-select]')
                    : null;
                if (!select) return;
                var slug = select.getAttribute('data-vis-mode-select');
                var row = rootEl.querySelector('[data-lesson-slug="' + slug + '"]');
                if (!row) return;
                var lesson = null;
                for (var i = 0; i < state.lessons.length; i += 1) {
                    if (state.lessons[i].slug === slug) {
                        lesson = state.lessons[i];
                        break;
                    }
                }
                if (!lesson) return;
                lesson.mode = select.value;
                var cell = row.querySelector('.lesson-vis-cell');
                if (!cell) return;
                var picker = cell.querySelector('[data-vis-users]');
                if (picker) picker.remove();
                if (lesson.mode === 'selected') {
                    var saveBtn = cell.querySelector('[data-vis-save]');
                    var html = renderUserPicker(lesson, state.users);
                    if (saveBtn && saveBtn.insertAdjacentHTML) {
                        saveBtn.insertAdjacentHTML('beforebegin', html);
                    } else {
                        cell.insertAdjacentHTML('beforeend', html);
                    }
                }
            });

            rootEl.addEventListener('click', function (event) {
                var target = event.target;
                while (target && target !== rootEl && !(target.getAttribute && (
                    target.getAttribute('data-vis-save')
                    || target.getAttribute('data-lesson-action')
                    || target.getAttribute('data-lesson-delete')
                    || target.hasAttribute('data-admin-dialog-close')
                ))) {
                    target = target.parentNode;
                }
                if (!target || target === rootEl) return;

                if (target.hasAttribute('data-admin-dialog-close')) {
                    if (state.dialogCtl) state.dialogCtl.close();
                    return;
                }

                if (target.getAttribute('data-lesson-action') === 'new') {
                    var dlg = rootEl.querySelector('#admin-new-lesson-dialog');
                    state.dialogCtl = openDialog(dlg, target);
                    return;
                }

                var deleteSlug = target.getAttribute('data-lesson-delete');
                if (deleteSlug) {
                    var confirmed = true;
                    try {
                        if (typeof root.confirm === 'function') {
                            confirmed = root.confirm('למחוק את השיעור מהמדף? ההקלטות נשמרות בארכיון. (Delete this lesson from the shelf? Recordings are kept in the archive.)');
                        }
                    } catch (err) {
                        confirmed = false;
                    }
                    if (!confirmed) return;
                    clearError(rootEl);
                    target.disabled = true;
                    api().request('/api/admin/lessons/' + encodeURIComponent(deleteSlug), {
                        method: 'DELETE',
                        body: {}
                    }).then(function (res) {
                        target.disabled = false;
                        if (!res.ok) {
                            showError(rootEl, res.error || ('HTTP ' + res.status));
                            return;
                        }
                        state.lessons = state.lessons.filter(function (lesson) {
                            return lesson.slug !== deleteSlug;
                        });
                        paint();
                    }).catch(function (err) {
                        target.disabled = false;
                        showError(rootEl, (err && err.message) || 'Network error');
                    });
                    return;
                }

                var saveSlug = target.getAttribute('data-vis-save');
                if (saveSlug) {
                    var row = rootEl.querySelector('[data-lesson-slug="' + saveSlug + '"]');
                    if (!row) return;
                    clearError(rootEl);
                    var payload = readVisibilityPayload(row);
                    var status = rootEl.querySelector('[data-vis-status="' + saveSlug + '"]');
                    if (status) status.textContent = '…';
                    target.disabled = true;
                    api().request('/api/admin/lessons/' + encodeURIComponent(saveSlug) + '/visibility', {
                        method: 'PUT',
                        body: payload
                    }).then(function (res) {
                        target.disabled = false;
                        if (!res.ok) {
                            if (status) status.textContent = '';
                            showError(rootEl, res.error || ('HTTP ' + res.status));
                            return;
                        }
                        for (var i = 0; i < state.lessons.length; i += 1) {
                            if (state.lessons[i].slug === saveSlug) {
                                state.lessons[i].mode = res.data.mode;
                                state.lessons[i].userIds = res.data.userIds || [];
                                state.lessons[i].grantCount = res.data.grantCount || 0;
                                break;
                            }
                        }
                        if (status) {
                            status.textContent = '✓ ' + (payload.mode === 'selected'
                                ? modeLabel('selected')
                                : modeLabel(payload.mode));
                        }
                    }).catch(function (err) {
                        target.disabled = false;
                        showError(rootEl, (err && err.message) || 'Network error');
                    });
                }
            });

            var form = rootEl.querySelector('#admin-new-lesson-form');
            if (form && !form.dataset.bound) {
                form.dataset.bound = '1';
                form.addEventListener('submit', function (event) {
                    event.preventDefault();
                    var errEl = rootEl.querySelector('#admin-new-lesson-error');
                    if (errEl) {
                        errEl.hidden = true;
                        errEl.textContent = '';
                    }
                    var titleInput = rootEl.querySelector('#admin-new-lesson-title-input');
                    var title = String(titleInput && titleInput.value || '').trim();
                    if (!title) {
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = 'A title is needed.';
                        }
                        return;
                    }
                    var slug = newLessonSlug();
                    if (!SLUG_RE.test(slug)) {
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = 'Could not make a lesson id.';
                        }
                        return;
                    }
                    var worksheet = blankWorksheet(slug, title);
                    var catalogEntry = catalogEntryFor(slug, title, 'basic');
                    api().request('/api/admin/lessons', {
                        method: 'POST',
                        body: { slug: slug, worksheet: worksheet, catalogEntry: catalogEntry }
                    }).then(function (res) {
                        if (!res.ok) {
                            if (errEl) {
                                errEl.hidden = false;
                                errEl.textContent = res.error
                                    || (res.data && res.data.error)
                                    || ('HTTP ' + res.status);
                            }
                            return;
                        }
                        if (state.dialogCtl) state.dialogCtl.close();
                        try {
                            root.location.hash = 'lesson/' + encodeURIComponent(slug);
                        } catch (err2) { /* ignore */ }
                    }).catch(function (err) {
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = (err && err.message) || 'Network error';
                        }
                    });
                });
            }
        }

        var Api = api();
        Promise.all([
            Api.get('/api/admin/lessons'),
            Api.get('/api/admin/users')
        ]).then(function (results) {
            var lessonsRes = results[0];
            var usersRes = results[1];
            if (!lessonsRes.ok) {
                panel.innerHTML = '<div class="card" data-admin-view="lessons" role="alert"><p>'
                    + bi('שגיאה בטעינת שיעורים.', 'Could not load lessons.') + '</p>'
                    + '<p class="en" dir="ltr">' + esc(lessonsRes.error || '') + '</p></div>';
                return;
            }
            state.users = (usersRes.ok && usersRes.data && usersRes.data.users) || [];
            var rows = (lessonsRes.data && lessonsRes.data.lessons) || [];
            return Promise.all(rows.map(function (row) {
                return Api.get('/api/admin/lessons/' + encodeURIComponent(row.slug)).then(function (detailRes) {
                    if (detailRes.ok && detailRes.data) {
                        return enrichLesson(row, detailRes.data);
                    }
                    return enrichLesson(row, null);
                });
            })).then(function (enriched) {
                state.lessons = enriched;
                paint();
            });
        }).catch(function (err) {
            panel.innerHTML = '<div class="card" data-admin-view="lessons" role="alert"><p>'
                + bi('שגיאת רשת.', 'Network error.') + '</p>'
                + '<p class="en" dir="ltr">' + esc((err && err.message) || '') + '</p></div>';
        });
    }

    root.HebrewAdminLessons = {
        SLUG_RE: SLUG_RE,
        escapeHtml: esc,
        bi: bi,
        modeLabel: modeLabel,
        enrichLesson: enrichLesson,
        renderLessonsTable: renderLessonsTable,
        renderLessonRow: renderLessonRow,
        renderUserPicker: renderUserPicker,
        blankWorksheet: blankWorksheet,
        catalogEntryFor: catalogEntryFor,
        mountList: mountList,
        mount: mountList
    };
})(typeof window !== 'undefined' ? window : globalThis);
