/**
 * Admin users list + learner activity detail.
 * Provenance: INIT-002/SPEC-016
 * All network calls go through HebrewApi → /api/ only.
 */
(function (root) {
    'use strict';

    var U = root.HebrewUtils;
    var HISTORY_LIMIT = 50;
    var NOTICE_LEARNERS =
        'הלומדים יודעים שמנהלים רואים את כל הפעילות, כולל כל תשובה. (Learners are told that admins can see all activity, including every answer.)';
    /* Hebrew line is a draft until the operator approves it (ADR-0009). Provenance: INIT-006/SPEC-017 */
    var AI_DISCLOSURE_EN = 'If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator\'s AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.';
    var AI_DISCLOSURE_HE = 'אם מנהל מפעיל תכנון בינה מלאכותית לחשבון שלך, סיכום של ההתקדמות ושם המשתמש נשלחים לספק הבינה המלאכותית של המפעיל כדי לנסח את השיעור הבא. הסיכום כולל את המילים שתורגלו ומידת הידיעה שלהן, את הדקדוק שתורגל ומידת הדיוק, את השיעורים שהושלמו, ובאיזו תדירות הקריאה היא בלי ניקוד. הסיכום אינו כולל תשובות בודדות, טקסט שהוקלד, או מועדי תרגול.';
    var AI_DISCLOSURE = AI_DISCLOSURE_HE + ' (' + AI_DISCLOSURE_EN + ')';
    var PASSWORD_HINT = 'לפחות 10 תווים. (At least 10 characters.)';

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

    function formatWhen(iso) {
        if (!iso) return '—';
        try {
            var d = new Date(iso);
            if (isNaN(d.getTime())) return String(iso);
            return d.toLocaleString();
        } catch (err) {
            return String(iso);
        }
    }

    function roleLabel(role) {
        if (role === 'admin') return bi('מנהל', 'Admin');
        return bi('משתמש', 'User');
    }

    function statusLabel(status) {
        if (status === 'disabled') return bi('מושבת', 'Disabled');
        return bi('פעיל', 'Active');
    }

    function directionLabel(dir) {
        if (dir === 'en_he') return bi('אנגלית → עברית', 'English → Hebrew');
        return bi('עברית → אנגלית', 'Hebrew → English');
    }

    function resultLabel(result) {
        if (result === 'pass') return bi('נכון', 'Pass');
        if (result === 'miss') return bi('פספוס', 'Miss');
        return esc(result || '—');
    }

    function lookupMeaning(word) {
        var index = root.HEBREW_VOCAB_INDEX;
        if (!index) return '';
        var list = Array.isArray(index) ? index : null;
        if (!list && typeof index === 'object') {
            list = Object.keys(index).map(function (k) {
                var e = index[k];
                if (!e || typeof e !== 'object') return null;
                return Object.assign({ key: e.key || k }, e);
            }).filter(Boolean);
        }
        if (!list) return '';
        var needle = String(word || '');
        for (var i = 0; i < list.length; i += 1) {
            var entry = list[i];
            if (!entry) continue;
            var key = entry.key || entry.word;
            if (key === needle || entry.word === needle) {
                return entry.meaning != null ? String(entry.meaning) : '';
            }
        }
        return '';
    }

    function lookupAudio(word) {
        var map = root.HEBREW_WORD_AUDIO || {};
        var needle = String(word || '');
        if (map[needle]) return String(map[needle]);
        try {
            var nfc = needle.normalize('NFC');
            if (map[nfc]) return String(map[nfc]);
        } catch (err) { /* ignore */ }
        var index = root.HEBREW_VOCAB_INDEX;
        if (Array.isArray(index)) {
            for (var i = 0; i < index.length; i += 1) {
                var entry = index[i];
                if (!entry) continue;
                if ((entry.key || entry.word) === needle || entry.word === needle) {
                    if (entry.audio) return String(entry.audio);
                }
            }
        }
        return '';
    }

    function audioHref(url) {
        if (!url) return '';
        if (/^(https?:|data:|blob:)/i.test(String(url))) return String(url);
        try {
            var base = new URL('../worksheets/_review/', root.location.href);
            return new URL(String(url), base).href;
        } catch (err) {
            return String(url);
        }
    }

    function speakButton(word) {
        var clip = audioHref(lookupAudio(word));
        if (!clip) return '';
        var icon = U && typeof U.icon === 'function'
            ? U.icon('volume-high')
            : '🔊';
        return '<button type="button" class="icon-btn human admin-speak-btn" data-admin-speak="'
            + esc(word) + '" data-audio="' + esc(clip) + '" title="'
            + esc('Listen') + '">' + icon + '</button>';
    }

    function hasMoreHistory(data, limit) {
        if (!data || !data.nextBefore) return false;
        if (!data.events || !data.events.length) return false;
        if (data.events.length < (limit || HISTORY_LIMIT)) return false;
        return true;
    }

    /* ---------- focus-trap dialogs ---------- */

    function getFocusable(container) {
        if (!container || !container.querySelectorAll) return [];
        var nodes = container.querySelectorAll(
            'a[href], button:not([disabled]), textarea:not([disabled]),'
            + ' input:not([disabled]):not([type="hidden"]), select:not([disabled]),'
            + ' [tabindex]:not([tabindex="-1"])'
        );
        return Array.prototype.slice.call(nodes);
    }

    /**
     * @param {HTMLElement} dialog
     * @param {HTMLElement|null} trigger
     * @param {{ onClose?: function(): void }} [opts]
     * @returns {{ close: function(): void }}
     */
    function openDialog(dialog, trigger, opts) {
        var options = opts || {};
        if (!dialog) return { close: function () {} };
        dialog.classList.remove('hidden');
        dialog.hidden = false;
        dialog.setAttribute('aria-hidden', 'false');

        var previouslyFocused = trigger || null;
        try {
            if (root.document && root.document.activeElement) {
                if (!previouslyFocused) previouslyFocused = root.document.activeElement;
            }
        } catch (err) { /* ignore */ }

        var focusables = getFocusable(dialog);
        var first = focusables[0];
        if (first && typeof first.focus === 'function') {
            try { first.focus(); } catch (err2) { /* ignore */ }
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
            if (previouslyFocused && typeof previouslyFocused.focus === 'function') {
                try { previouslyFocused.focus(); } catch (err3) { /* ignore */ }
            }
            if (typeof options.onClose === 'function') options.onClose();
        }

        dialog.addEventListener('keydown', onKey);
        dialog.addEventListener('click', onClick);
        return { close: close };
    }

    /* ---------- users table ---------- */

    function usersSnapshot(users) {
        return (users || []).map(function (u) {
            return {
                id: u.id,
                username: u.username,
                display_name: u.display_name,
                role: u.role,
                status: u.status,
                created_at: u.created_at,
                last_sign_in: u.last_sign_in,
                ai_planning: u.ai_planning === true
            };
        });
    }

    function aiPlanningButton(user, id) {
        var known = user.ai_planning === true || user.ai_planning === false;
        if (!known) {
            return '<button type="button" class="btn admin-row-btn" data-ai-planning-toggle disabled'
                + ' aria-disabled="true">'
                + bi('תכנון בינה', 'AI planning') + ': ' + bi('לא ידוע', 'unknown')
                + '</button>';
        }
        var on = user.ai_planning === true;
        return '<button type="button" class="btn admin-row-btn" data-admin-action="ai-planning"'
            + ' data-ai-planning-toggle data-user-id="' + id + '"'
            + ' aria-pressed="' + (on ? 'true' : 'false') + '">'
            + bi('תכנון בינה', 'AI planning') + ': '
            + (on ? bi('פעיל', 'on') : bi('כבוי', 'off'))
            + '</button>';
    }

    function renderUserRow(user) {
        var id = esc(String(user.id));
        var isAdmin = user.role === 'admin';
        var isDisabled = user.status === 'disabled';
        var actions = ''
            + '<button type="button" class="btn admin-row-btn" data-admin-action="reset" data-user-id="'
            + id + '">' + bi('איפוס סיסמה', 'Reset password') + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-admin-action="'
            + (isAdmin ? 'make-user' : 'make-admin') + '" data-user-id="' + id + '">'
            + (isAdmin ? bi('הפוך למשתמש', 'Make user') : bi('הפוך למנהל', 'Make admin'))
            + '</button> '
            + '<button type="button" class="btn admin-row-btn" data-admin-action="'
            + (isDisabled ? 'enable' : 'disable') + '" data-user-id="' + id + '">'
            + (isDisabled ? bi('הפעלה', 'Enable') : bi('השבתה', 'Disable'))
            + '</button>';

        return '<tr data-user-id="' + id + '" data-role="' + esc(user.role || '')
            + '" data-status="' + esc(user.status || '') + '">'
            + '<td><a class="admin-user-link" href="#user/' + id + '">'
            + esc(user.username || '') + '</a></td>'
            + '<td>' + esc(user.display_name || '') + '</td>'
            + '<td data-col="role">' + roleLabel(user.role) + '</td>'
            + '<td data-col="status">' + statusLabel(user.status) + '</td>'
            + '<td dir="ltr">' + esc(formatWhen(user.last_sign_in)) + '</td>'
            + '<td data-col="ai-planning">' + aiPlanningButton(user, id) + '</td>'
            + '<td class="admin-row-actions">' + actions + '</td>'
            + '</tr>';
    }

    function renderUsersTable(users) {
        var rows = (users || []).map(renderUserRow).join('');
        if (!rows) {
            rows = '<tr data-admin-empty="1"><td colspan="7">'
                + bi('אין משתמשים.', 'No users.') + '</td></tr>';
        }
        return ''
            + '<div class="card admin-users-card" data-admin-view="users">'
            + '<div class="admin-users-head">'
            + '<h2>' + bi('משתמשים', 'Users') + '</h2>'
            + '<button type="button" class="btn btn-primary" data-admin-action="create">'
            + bi('יצירת משתמש', 'Create user') + '</button>'
            + '</div>'
            + '<p class="admin-inline-error" data-admin-guard-error hidden></p>'
            + '<div class="admin-table-wrap">'
            + '<table class="admin-users-table">'
            + '<thead><tr>'
            + '<th>' + bi('שם משתמש', 'Username') + '</th>'
            + '<th>' + bi('שם תצוגה', 'Display name') + '</th>'
            + '<th>' + bi('תפקיד', 'Role') + '</th>'
            + '<th>' + bi('סטטוס', 'Status') + '</th>'
            + '<th>' + bi('התחברות אחרונה', 'Last sign-in') + '</th>'
            + '<th>' + bi('תכנון בינה', 'AI planning') + '</th>'
            + '<th>' + bi('פעולות', 'Actions') + '</th>'
            + '</tr></thead>'
            + '<tbody data-admin-users-body>' + rows + '</tbody>'
            + '</table></div></div>';
    }

    function createDialogHtml() {
        return ''
            + '<div class="admin-dialog hidden" id="admin-create-dialog" role="dialog" aria-modal="true"'
            + ' aria-labelledby="admin-create-title" hidden aria-hidden="true">'
            + '<div class="admin-dialog-panel" role="document">'
            + '<h2 id="admin-create-title">' + bi('יצירת משתמש', 'Create user') + '</h2>'
            + '<form id="admin-create-form" autocomplete="on">'
            + '<label class="hebrew-shell-field">' + bi('שם משתמש', 'Username')
            + '<input type="text" name="username" id="admin-create-username" required'
            + ' autocomplete="username"></label>'
            + '<label class="hebrew-shell-field">' + bi('שם תצוגה', 'Display name')
            + '<input type="text" name="displayName" id="admin-create-display"'
            + ' autocomplete="nickname"></label>'
            + '<label class="hebrew-shell-field">' + bi('תפקיד', 'Role')
            + '<select name="role" id="admin-create-role">'
            + '<option value="user">' + esc('user') + '</option>'
            + '<option value="admin">' + esc('admin') + '</option>'
            + '</select></label>'
            + '<label class="hebrew-shell-field">' + bi('סיסמה ראשונית', 'Initial password')
            + '<input type="password" name="password" id="admin-create-password" required'
            + ' minlength="10" autocomplete="new-password"></label>'
            + '<p class="admin-password-hint">' + esc(PASSWORD_HINT) + '</p>'
            + '<p class="admin-inline-error" id="admin-create-error" hidden></p>'
            + '<div class="admin-dialog-actions">'
            + '<button type="submit" class="btn btn-primary">' + bi('יצירה', 'Create') + '</button>'
            + '<button type="button" class="btn" data-admin-dialog-close>' + bi('ביטול', 'Cancel') + '</button>'
            + '</div></form></div></div>';
    }

    function resetDialogHtml() {
        return ''
            + '<div class="admin-dialog hidden" id="admin-reset-dialog" role="dialog" aria-modal="true"'
            + ' aria-labelledby="admin-reset-title" hidden aria-hidden="true">'
            + '<div class="admin-dialog-panel" role="document">'
            + '<h2 id="admin-reset-title">' + bi('איפוס סיסמה', 'Reset password') + '</h2>'
            + '<form id="admin-reset-form" autocomplete="on">'
            + '<input type="hidden" name="userId" id="admin-reset-user-id" value="">'
            + '<label class="hebrew-shell-field">' + bi('סיסמה חדשה', 'New password')
            + '<input type="password" name="password" id="admin-reset-password" required'
            + ' minlength="10" autocomplete="new-password"></label>'
            + '<p class="admin-password-hint">' + esc(PASSWORD_HINT) + '</p>'
            + '<p class="admin-inline-error" id="admin-reset-error" hidden></p>'
            + '<div class="admin-dialog-actions">'
            + '<button type="submit" class="btn btn-primary">' + bi('איפוס', 'Reset') + '</button>'
            + '<button type="button" class="btn" data-admin-dialog-close>' + bi('ביטול', 'Cancel') + '</button>'
            + '</div></form></div></div>';
    }

    function aiPlanningDialogHtml() {
        return ''
            + '<div class="admin-dialog hidden" id="admin-ai-planning-dialog" role="dialog" aria-modal="true"'
            + ' aria-labelledby="admin-ai-planning-title" hidden aria-hidden="true">'
            + '<div class="admin-dialog-panel" role="document">'
            + '<h2 id="admin-ai-planning-title">' + bi('תכנון בינה מלאכותית', 'AI planning') + '</h2>'
            + '<p data-ai-planning-name></p>'
            + '<p data-ai-planning-lead></p>'
            + '<p class="admin-ai-disclosure" data-ai-disclosure="1">' + esc(AI_DISCLOSURE) + '</p>'
            + '<form id="admin-ai-planning-form">'
            + '<p class="admin-inline-error" id="admin-ai-planning-error" hidden></p>'
            + '<div class="admin-dialog-actions">'
            + '<button type="submit" class="btn btn-primary">' + bi('אישור', 'Confirm') + '</button>'
            + '<button type="button" class="btn" data-admin-dialog-close>' + bi('ביטול', 'Cancel') + '</button>'
            + '</div></form></div></div>';
    }

    function showGuardError(rootEl, message) {
        var el = rootEl.querySelector('[data-admin-guard-error]');
        if (!el) return;
        el.hidden = false;
        el.removeAttribute('hidden');
        el.textContent = message || '';
    }

    function clearGuardError(rootEl) {
        var el = rootEl.querySelector('[data-admin-guard-error]');
        if (!el) return;
        el.hidden = true;
        el.setAttribute('hidden', '');
        el.textContent = '';
    }

    /**
     * On 409/refusal: restore prior table markup and show the server message.
     * @returns {boolean} true when the table was updated from res.data
     */
    function applyPatchResult(panel, state, beforeUsers, beforeBodyHtml, res) {
        if (!res || !res.ok) {
            state.users = beforeUsers;
            var body = panel && panel.querySelector('[data-admin-users-body]');
            if (body) body.innerHTML = beforeBodyHtml;
            showGuardError(panel, (res && res.error) || 'Request refused');
            return false;
        }
        if (res.data) {
            var list = state.users || [];
            for (var i = 0; i < list.length; i += 1) {
                if (String(list[i].id) === String(res.data.id)) {
                    var next = usersSnapshot([res.data])[0];
                    if (typeof res.data.ai_planning !== 'boolean') {
                        next.ai_planning = list[i].ai_planning === true;
                    }
                    list[i] = next;
                    break;
                }
            }
            var tbody = panel && panel.querySelector('[data-admin-users-body]');
            if (tbody) {
                tbody.innerHTML = list.map(renderUserRow).join('');
            }
        }
        return true;
    }

    /**
     * Append history events without duplicates; update cursor + Load older visibility.
     * @returns {{ appended: number, more: boolean }}
     */
    function applyHistoryPage(state, rootEl, data, limit) {
        var body = rootEl && rootEl.querySelector('[data-admin-history-body]');
        var appended = 0;
        var fresh = [];
        (data && data.events || []).forEach(function (ev) {
            var eid = String(ev.id);
            if (state.seenIds[eid]) return;
            state.seenIds[eid] = true;
            fresh.push(ev);
            appended += 1;
        });
        if (body && fresh.length) {
            if (typeof body.insertAdjacentHTML === 'function') {
                body.insertAdjacentHTML('beforeend', renderHistoryRows(fresh));
            } else {
                body.innerHTML = (body.innerHTML || '') + renderHistoryRows(fresh);
            }
        }
        state.nextBefore = (data && data.nextBefore) || null;
        var more = hasMoreHistory(data, limit || HISTORY_LIMIT);
        var btn = rootEl && rootEl.querySelector('[data-admin-load-older]');
        if (btn) {
            if (more) {
                btn.hidden = false;
                btn.removeAttribute('hidden');
            } else {
                btn.hidden = true;
                btn.setAttribute('hidden', '');
            }
        }
        return { appended: appended, more: more };
    }

    function findUser(state, userId) {
        var list = (state && state.users) || [];
        for (var i = 0; i < list.length; i += 1) {
            if (String(list[i].id) === String(userId)) return list[i];
        }
        return null;
    }

    function paintUsersBody(panel, users) {
        var tbody = panel && panel.querySelector('[data-admin-users-body]');
        if (tbody) tbody.innerHTML = (users || []).map(renderUserRow).join('');
    }

    /**
     * The row changes only after a 200 whose body includes boolean ai_planning.
     * A failure leaves the previous flag and shows the error.
     * @returns {boolean}
     */
    function applyAiPlanningResult(panel, state, userId, res) {
        var user = findUser(state, userId);
        if (!user) {
            showGuardError(panel, 'User not found');
            return false;
        }
        var body = res && res.data;
        var next = body && typeof body.ai_planning === 'boolean' ? body.ai_planning : null;
        if (!res || !res.ok || next == null) {
            showGuardError(panel, (res && res.error) || 'Could not change AI planning');
            paintUsersBody(panel, state.users);
            return false;
        }
        user.ai_planning = next;
        clearGuardError(panel);
        paintUsersBody(panel, state.users);
        return true;
    }

    function patchUser(userId, body) {
        var Api = api();
        return Api.request('/api/admin/users/' + encodeURIComponent(String(userId)), {
            method: 'PATCH',
            body: body
        });
    }

    /**
     * @param {HTMLElement} panel
     */
    function mountUsersList(panel) {
        if (!panel) return;
        var state = { users: [], dialogCtl: null, bound: false };

        panel.innerHTML = '<div class="card" data-admin-state="loading"><p>'
            + bi('טוען…', 'Loading…') + '</p></div>';

        function paintTable() {
            panel.innerHTML = renderUsersTable(state.users) + createDialogHtml() + resetDialogHtml()
                + aiPlanningDialogHtml();
            wireForms(panel);
        }

        function wireForms(rootEl) {
            var createFormEl = rootEl.querySelector('#admin-create-form');
            if (createFormEl && !createFormEl.dataset.bound) {
                createFormEl.dataset.bound = '1';
                createFormEl.addEventListener('submit', function (event) {
                    event.preventDefault();
                    var errEl = rootEl.querySelector('#admin-create-error');
                    var usernameEl = rootEl.querySelector('#admin-create-username');
                    var displayEl = rootEl.querySelector('#admin-create-display');
                    var roleEl = rootEl.querySelector('#admin-create-role');
                    var passwordEl = rootEl.querySelector('#admin-create-password');
                    if (errEl) {
                        errEl.hidden = true;
                        errEl.textContent = '';
                    }
                    var Api = api();
                    if (!Api) return;
                    var passwordValue = passwordEl ? passwordEl.value : '';
                    Api.post('/api/admin/users', {
                        username: usernameEl ? usernameEl.value : '',
                        displayName: displayEl ? displayEl.value : '',
                        role: roleEl ? roleEl.value : 'user',
                        password: passwordValue
                    }).then(function (res) {
                        if (passwordEl) passwordEl.value = '';
                        passwordValue = '';
                        if (!res.ok) {
                            if (errEl) {
                                errEl.hidden = false;
                                errEl.textContent = res.error || 'Could not create user';
                            }
                            return;
                        }
                        if (state.dialogCtl) state.dialogCtl.close();
                        refresh();
                    }).catch(function () {
                        if (passwordEl) passwordEl.value = '';
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = 'Network error';
                        }
                    });
                });
            }

            var resetFormEl = rootEl.querySelector('#admin-reset-form');
            if (resetFormEl && !resetFormEl.dataset.bound) {
                resetFormEl.dataset.bound = '1';
                resetFormEl.addEventListener('submit', function (event) {
                    event.preventDefault();
                    var errEl = rootEl.querySelector('#admin-reset-error');
                    var idEl = rootEl.querySelector('#admin-reset-user-id');
                    var passwordEl = rootEl.querySelector('#admin-reset-password');
                    if (errEl) {
                        errEl.hidden = true;
                        errEl.textContent = '';
                    }
                    var Api = api();
                    if (!Api || !idEl) return;
                    var passwordValue = passwordEl ? passwordEl.value : '';
                    var targetId = idEl.value;
                    Api.post('/api/admin/users/' + encodeURIComponent(targetId) + '/password', {
                        password: passwordValue
                    }).then(function (res) {
                        if (passwordEl) passwordEl.value = '';
                        passwordValue = '';
                        if (!res.ok) {
                            if (errEl) {
                                errEl.hidden = false;
                                errEl.textContent = res.error || 'Could not reset password';
                            }
                            return;
                        }
                        if (state.dialogCtl) state.dialogCtl.close();
                    }).catch(function () {
                        if (passwordEl) passwordEl.value = '';
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = 'Network error';
                        }
                    });
                });
            }

            var planningForm = rootEl.querySelector('#admin-ai-planning-form');
            if (planningForm && !planningForm.dataset.bound) {
                planningForm.dataset.bound = '1';
                planningForm.addEventListener('submit', function (event) {
                    event.preventDefault();
                    var dlg = rootEl.querySelector('#admin-ai-planning-dialog');
                    var errEl = rootEl.querySelector('#admin-ai-planning-error');
                    if (errEl) {
                        errEl.hidden = true;
                        errEl.textContent = '';
                    }
                    if (!dlg) return;
                    var userId = dlg.getAttribute('data-user-id') || '';
                    var enabled = dlg.getAttribute('data-enabled') === '1';
                    var subject = findUser(state, userId);
                    if (!subject) return;
                    var submitBtn = planningForm.querySelector('button[type="submit"]');
                    if (submitBtn) submitBtn.disabled = true;
                    var Api = api();
                    if (!Api || typeof Api.request !== 'function') {
                        if (submitBtn) submitBtn.disabled = false;
                        applyAiPlanningResult(rootEl, state, userId, { ok: false, error: 'API unavailable' });
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = 'API unavailable';
                        }
                        return;
                    }
                    Api.request('/api/admin/users/' + encodeURIComponent(userId) + '/ai-planning', {
                        method: 'PATCH',
                        body: { enabled: enabled }
                    }).then(function (res) {
                        if (submitBtn) submitBtn.disabled = false;
                        var changed = applyAiPlanningResult(rootEl, state, userId, res);
                        if (!changed) {
                            if (errEl) {
                                errEl.hidden = false;
                                errEl.textContent = (res && res.error) || 'Could not change AI planning';
                            }
                            return;
                        }
                        if (state.dialogCtl) state.dialogCtl.close();
                    }).catch(function () {
                        if (submitBtn) submitBtn.disabled = false;
                        applyAiPlanningResult(rootEl, state, userId, { ok: false, error: 'Network error' });
                        if (errEl) {
                            errEl.hidden = false;
                            errEl.textContent = 'Network error';
                        }
                    });
                });
            }
        }

        if (!state.bound) {
            state.bound = true;
            panel.addEventListener('click', function (event) {
                var closeBtn = event.target.closest && event.target.closest('[data-admin-dialog-close]');
                if (closeBtn && state.dialogCtl) {
                    state.dialogCtl.close();
                    return;
                }

                var btn = event.target.closest && event.target.closest('[data-admin-action]');
                if (!btn || !panel.contains(btn)) return;
                var action = btn.getAttribute('data-admin-action');
                var userId = btn.getAttribute('data-user-id');

                if (action === 'create') {
                    var createDlg = panel.querySelector('#admin-create-dialog');
                    var createForm = panel.querySelector('#admin-create-form');
                    var createErr = panel.querySelector('#admin-create-error');
                    if (createForm) createForm.reset();
                    if (createErr) {
                        createErr.hidden = true;
                        createErr.textContent = '';
                    }
                    state.dialogCtl = openDialog(createDlg, btn);
                    return;
                }

                if (action === 'reset' && userId) {
                    var resetDlg = panel.querySelector('#admin-reset-dialog');
                    var resetForm = panel.querySelector('#admin-reset-form');
                    var resetErr = panel.querySelector('#admin-reset-error');
                    var resetId = panel.querySelector('#admin-reset-user-id');
                    var resetPw = panel.querySelector('#admin-reset-password');
                    if (resetForm) resetForm.reset();
                    if (resetId) resetId.value = String(userId);
                    if (resetPw) resetPw.value = '';
                    if (resetErr) {
                        resetErr.hidden = true;
                        resetErr.textContent = '';
                    }
                    state.dialogCtl = openDialog(resetDlg, btn);
                    return;
                }

                if ((action === 'make-admin' || action === 'make-user') && userId) {
                    clearGuardError(panel);
                    var before = usersSnapshot(state.users);
                    var nextRole = action === 'make-admin' ? 'admin' : 'user';
                    var beforeHtml = panel.querySelector('[data-admin-users-body]')
                        ? panel.querySelector('[data-admin-users-body]').innerHTML
                        : '';
                    patchUser(userId, { role: nextRole }).then(function (res) {
                        if (!applyPatchResult(panel, state, before, beforeHtml, res)) return;
                        refresh();
                    }).catch(function () {
                        applyPatchResult(panel, state, before, beforeHtml, {
                            ok: false,
                            error: 'Network error'
                        });
                    });
                    return;
                }

                if (action === 'ai-planning' && userId) {
                    var subject = findUser(state, userId);
                    if (!subject || (subject.ai_planning !== true && subject.ai_planning !== false)) return;
                    var dlg = panel.querySelector('#admin-ai-planning-dialog');
                    var nameEl = panel.querySelector('[data-ai-planning-name]');
                    var leadEl = panel.querySelector('[data-ai-planning-lead]');
                    var errEl = panel.querySelector('#admin-ai-planning-error');
                    var turningOn = subject.ai_planning !== true;
                    if (nameEl) nameEl.textContent = subject.username || '';
                    if (leadEl) {
                        leadEl.textContent = turningOn
                            ? 'להפעיל תכנון בינה מלאכותית לחשבון הזה? (Turn AI planning on for this account?)'
                            : 'לכבות תכנון בינה מלאכותית לחשבון הזה? (Turn AI planning off for this account?)';
                    }
                    if (errEl) {
                        errEl.hidden = true;
                        errEl.textContent = '';
                    }
                    if (dlg) {
                        dlg.setAttribute('data-user-id', String(userId));
                        dlg.setAttribute('data-enabled', turningOn ? '1' : '0');
                    }
                    clearGuardError(panel);
                    state.dialogCtl = openDialog(dlg, btn);
                    return;
                }

                if ((action === 'disable' || action === 'enable') && userId) {
                    clearGuardError(panel);
                    var beforeStatus = usersSnapshot(state.users);
                    var nextStatus = action === 'disable' ? 'disabled' : 'active';
                    var beforeStatusHtml = panel.querySelector('[data-admin-users-body]')
                        ? panel.querySelector('[data-admin-users-body]').innerHTML
                        : '';
                    patchUser(userId, { status: nextStatus }).then(function (res) {
                        if (!applyPatchResult(panel, state, beforeStatus, beforeStatusHtml, res)) return;
                        refresh();
                    }).catch(function () {
                        applyPatchResult(panel, state, beforeStatus, beforeStatusHtml, {
                            ok: false,
                            error: 'Network error'
                        });
                    });
                }
            });
        }

        function refresh() {
            var Api = api();
            if (!Api) {
                panel.innerHTML = '<div class="card" data-admin-state="error" role="alert"><p>'
                    + bi('שגיאה', 'Error') + '</p></div>';
                return;
            }
            Api.get('/api/admin/users').then(function (res) {
                if (!res.ok || !res.data) {
                    panel.innerHTML = '<div class="card" data-admin-state="error" role="alert"><p>'
                        + bi('לא ניתן לטעון משתמשים.', 'Could not load users.') + '</p>'
                        + '<p class="en" dir="ltr">' + esc(res.error || ('HTTP ' + res.status)) + '</p></div>';
                    return;
                }
                state.users = usersSnapshot(res.data.users || []);
                paintTable();
            }).catch(function (err) {
                panel.innerHTML = '<div class="card" data-admin-state="error" role="alert"><p>'
                    + bi('שגיאת רשת.', 'Network error.') + '</p>'
                    + '<p class="en" dir="ltr">' + esc((err && err.message) || '') + '</p></div>';
            });
        }

        refresh();
    }

    /* ---------- user detail ---------- */

    function renderTroubleWords(items) {
        var list = (items || []).slice(0, 10);
        if (!list.length) {
            return '<p class="admin-empty">' + bi('אין מילים בעייתיות.', 'No trouble words.') + '</p>';
        }
        return '<ul class="admin-trouble-list">' + list.map(function (item) {
            var word = item.word || '';
            var meaning = lookupMeaning(word);
            var meaningHtml = meaning
                ? ' <span class="admin-trouble-meaning en" dir="ltr">(' + esc(meaning) + ')</span>'
                : '';
            return '<li class="admin-trouble-item" data-word="' + esc(word) + '">'
                + '<span class="admin-trouble-word">' + esc(word) + '</span>'
                + meaningHtml
                + ' ' + speakButton(word)
                + ' <span class="admin-trouble-meta en" dir="ltr">'
                + esc(String(item.direction || '')) + ' · misses '
                + esc(String(item.misses || 0)) + ' · lapses '
                + esc(String(item.lapses || 0))
                + '</span></li>';
        }).join('') + '</ul>';
    }

    function renderHistoryRows(events) {
        return (events || []).map(function (ev) {
            var id = esc(String(ev.id));
            var lesson = ev.lessonTitle || ev.slug || '—';
            return '<tr data-event-id="' + id + '">'
                + '<td dir="ltr">' + esc(formatWhen(ev.at)) + '</td>'
                + '<td>' + esc(ev.word || '') + '</td>'
                + '<td>' + directionLabel(ev.direction) + '</td>'
                + '<td>' + resultLabel(ev.result) + '</td>'
                + '<td>' + esc(lesson) + '</td>'
                + '<td>' + esc(ev.activity || '—') + '</td>'
                + '</tr>';
        }).join('');
    }

    function renderUserDetailShell(user) {
        var name = user ? (user.display_name || user.username || '') : '';
        return ''
            + '<div class="admin-user-detail" data-admin-view="user" data-user-id="'
            + esc(user ? String(user.id) : '') + '">'
            + '<p class="admin-back"><a href="#users">' + bi('← חזרה למשתמשים', '← Back to users') + '</a></p>'
            + '<header class="admin-user-head card">'
            + '<h2>' + esc(name) + '</h2>'
            + '<dl class="admin-account-details">'
            + '<div><dt>' + bi('שם משתמש', 'Username') + '</dt><dd data-field="username">'
            + esc(user ? user.username : '') + '</dd></div>'
            + '<div><dt>' + bi('שם תצוגה', 'Display name') + '</dt><dd data-field="display_name">'
            + esc(user ? (user.display_name || '') : '') + '</dd></div>'
            + '<div><dt>' + bi('תפקיד', 'Role') + '</dt><dd data-field="role">'
            + roleLabel(user && user.role) + '</dd></div>'
            + '<div><dt>' + bi('סטטוס', 'Status') + '</dt><dd data-field="status">'
            + statusLabel(user && user.status) + '</dd></div>'
            + '<div><dt>' + bi('נוצר', 'Created') + '</dt><dd dir="ltr">'
            + esc(formatWhen(user && user.created_at)) + '</dd></div>'
            + '<div><dt>' + bi('התחברות אחרונה', 'Last sign-in') + '</dt><dd dir="ltr">'
            + esc(formatWhen(user && user.last_sign_in)) + '</dd></div>'
            + '</dl>'
            + '<p class="admin-privacy-note" role="note">' + esc(NOTICE_LEARNERS) + '</p>'
            + '</header>'
            + '<section class="admin-progress-section" aria-label="Progress">'
            + '<div id="admin-user-progress" data-admin-progress></div>'
            + '</section>'
            + '<section class="card admin-trouble-section">'
            + '<h3>' + bi('מילים בעייתיות', 'Trouble words') + '</h3>'
            + '<div data-admin-trouble></div>'
            + '</section>'
            + '<section class="card admin-history-section">'
            + '<h3>' + bi('היסטוריית תשובות', 'Answer history') + '</h3>'
            + '<div class="admin-table-wrap">'
            + '<table class="admin-history-table">'
            + '<thead><tr>'
            + '<th>' + bi('זמן', 'Time') + '</th>'
            + '<th>' + bi('מילה', 'Word') + '</th>'
            + '<th>' + bi('כיוון', 'Direction') + '</th>'
            + '<th>' + bi('תוצאה', 'Result') + '</th>'
            + '<th>' + bi('שיעור', 'Lesson') + '</th>'
            + '<th>' + bi('פעילות', 'Activity') + '</th>'
            + '</tr></thead>'
            + '<tbody data-admin-history-body></tbody>'
            + '</table></div>'
            + '<button type="button" class="btn admin-load-older" data-admin-load-older hidden>'
            + bi('טען ישנים יותר', 'Load older') + '</button>'
            + '</section></div>';
    }

    /**
     * @param {HTMLElement} panel
     * @param {string|number} userId
     */
    function mountUserDetail(panel, userId) {
        if (!panel) return;
        var id = String(userId);
        var state = {
            user: null,
            nextBefore: null,
            seenIds: Object.create(null),
            loadingMore: false
        };

        panel.innerHTML = '<div class="card" data-admin-state="loading"><p>'
            + bi('טוען…', 'Loading…') + '</p></div>';

        var Api = api();
        if (!Api) {
            panel.innerHTML = '<div class="card" data-admin-state="error"><p>'
                + bi('שגיאה', 'Error') + '</p></div>';
            return;
        }

        function bindDetail(rootEl) {
            rootEl.addEventListener('click', function (event) {
                var speak = event.target.closest && event.target.closest('[data-admin-speak]');
                if (speak) {
                    var text = speak.getAttribute('data-admin-speak') || '';
                    var audio = speak.getAttribute('data-audio') || '';
                    var Speech = root.HebrewSpeech;
                    if (Speech && typeof Speech.play === 'function') {
                        Speech.play(text, audio);
                    } else if (audio) {
                        try {
                            var a = new Audio(audio);
                            a.play();
                        } catch (err) { /* ignore */ }
                    }
                    return;
                }

                var older = event.target.closest && event.target.closest('[data-admin-load-older]');
                if (older) {
                    loadOlder(rootEl);
                }
            });
        }

        function setLoadOlderVisible(rootEl, visible) {
            var btn = rootEl.querySelector('[data-admin-load-older]');
            if (!btn) return;
            if (visible) {
                btn.hidden = false;
                btn.removeAttribute('hidden');
            } else {
                btn.hidden = true;
                btn.setAttribute('hidden', '');
            }
        }

        function loadOlder(rootEl) {
            if (state.loadingMore || !state.nextBefore) return;
            state.loadingMore = true;
            var btn = rootEl.querySelector('[data-admin-load-older]');
            if (btn) btn.disabled = true;
            var url = '/api/admin/users/' + encodeURIComponent(id)
                + '/events?limit=' + HISTORY_LIMIT
                + '&before=' + encodeURIComponent(state.nextBefore);
            Api.get(url).then(function (res) {
                state.loadingMore = false;
                if (btn) btn.disabled = false;
                if (!res.ok || !res.data) {
                    return;
                }
                applyHistoryPage(state, rootEl, res.data, HISTORY_LIMIT);
            }).catch(function () {
                state.loadingMore = false;
                if (btn) btn.disabled = false;
            });
        }

        Promise.all([
            Api.get('/api/admin/users'),
            Api.get('/api/admin/users/' + encodeURIComponent(id) + '/progress'),
            Api.get('/api/admin/users/' + encodeURIComponent(id) + '/events?limit=' + HISTORY_LIMIT)
        ]).then(function (results) {
            var usersRes = results[0];
            var progRes = results[1];
            var histRes = results[2];

            if (!usersRes.ok) {
                panel.innerHTML = '<div class="card" data-admin-state="error" role="alert"><p>'
                    + bi('לא נמצא.', 'Not found.') + '</p></div>';
                return;
            }
            var users = (usersRes.data && usersRes.data.users) || [];
            var user = null;
            for (var i = 0; i < users.length; i += 1) {
                if (String(users[i].id) === id) {
                    user = users[i];
                    break;
                }
            }
            if (!user) {
                panel.innerHTML = '<div class="card" data-admin-state="not-found" role="alert"><p>'
                    + bi('משתמש לא נמצא.', 'User not found.') + '</p>'
                    + '<p><a href="#users">' + bi('חזרה', 'Back') + '</a></p></div>';
                return;
            }
            state.user = user;
            panel.innerHTML = renderUserDetailShell(user);
            bindDetail(panel);

            var progressEl = panel.querySelector('[data-admin-progress]');
            var View = root.HebrewProgressView;
            if (progressEl && View && typeof View.render === 'function') {
                if (!progRes.ok || !progRes.data) {
                    View.render(progressEl, {
                        state: 'error',
                        message: progRes.error || ('HTTP ' + progRes.status)
                    });
                } else {
                    View.render(progressEl, progRes.data.metrics || {});
                    var troubleHost = panel.querySelector('[data-admin-trouble]');
                    if (troubleHost) {
                        troubleHost.innerHTML = renderTroubleWords(progRes.data.troubleWords || []);
                    }
                }
            }

            if (histRes.ok && histRes.data) {
                (histRes.data.events || []).forEach(function (ev) {
                    state.seenIds[String(ev.id)] = true;
                });
                var body = panel.querySelector('[data-admin-history-body]');
                if (body) {
                    body.innerHTML = renderHistoryRows(histRes.data.events || [])
                        || ('<tr><td colspan="6">' + bi('אין תשובות עדיין.', 'No answers yet.') + '</td></tr>');
                }
                state.nextBefore = histRes.data.nextBefore || null;
                setLoadOlderVisible(panel, hasMoreHistory(histRes.data, HISTORY_LIMIT));
            }
        }).catch(function (err) {
            panel.innerHTML = '<div class="card" data-admin-state="error" role="alert"><p>'
                + bi('שגיאת רשת.', 'Network error.') + '</p>'
                + '<p class="en" dir="ltr">' + esc((err && err.message) || '') + '</p></div>';
        });
    }

    root.HebrewAdminUsers = {
        HISTORY_LIMIT: HISTORY_LIMIT,
        NOTICE_LEARNERS: NOTICE_LEARNERS,
        AI_DISCLOSURE: AI_DISCLOSURE,
        PASSWORD_HINT: PASSWORD_HINT,
        escapeHtml: esc,
        bi: bi,
        lookupMeaning: lookupMeaning,
        lookupAudio: lookupAudio,
        hasMoreHistory: hasMoreHistory,
        openDialog: openDialog,
        applyPatchResult: applyPatchResult,
        applyAiPlanningResult: applyAiPlanningResult,
        applyHistoryPage: applyHistoryPage,
        showGuardError: showGuardError,
        renderUsersTable: renderUsersTable,
        renderUserRow: renderUserRow,
        renderTroubleWords: renderTroubleWords,
        renderHistoryRows: renderHistoryRows,
        createDialogHtml: createDialogHtml,
        resetDialogHtml: resetDialogHtml,
        aiPlanningDialogHtml: aiPlanningDialogHtml,
        mountUsersList: mountUsersList,
        mountUserDetail: mountUserDetail,
        usersSnapshot: usersSnapshot
    };
})(typeof window !== 'undefined' ? window : globalThis);
