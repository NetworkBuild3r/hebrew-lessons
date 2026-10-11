/**
 * Shared top bar (nav + account menu + forced password change).
 * Provenance: INIT-002/SPEC-012
 */
(function (root) {
    'use strict';

    var PRIVACY_NOTE =
        'מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה. (Admins can see all your activity, including every answer.)';
    /* Hebrew line is a draft until the operator approves it (ADR-0009). Provenance: INIT-006/SPEC-017 */
    var AI_DISCLOSURE_EN = 'If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator\'s AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.';
    var AI_DISCLOSURE_HE = 'אם מנהל מפעיל תכנון בינה מלאכותית לחשבון שלך, סיכום של ההתקדמות ושם המשתמש נשלחים לספק הבינה המלאכותית של המפעיל כדי לנסח את השיעור הבא. הסיכום כולל את המילים שתורגלו ומידת הידיעה שלהן, את הדקדוק שתורגל ומידת הדיוק, את השיעורים שהושלמו, ובאיזו תדירות הקריאה היא בלי ניקוד. הסיכום אינו כולל תשובות בודדות, טקסט שהוקלד, או מועדי תרגול.';
    var AI_DISCLOSURE = AI_DISCLOSURE_HE + ' (' + AI_DISCLOSURE_EN + ')';

    var NAV = [
        { id: 'today', href: '/', he: 'היום', en: 'Today', icon: 'today' },
        { id: 'lessons', href: '/lessons/', he: 'שיעורים', en: 'Lessons', icon: 'lessons' },
        { id: 'guides', href: '/guides/', he: 'מדריכים', en: 'Guides' },
        { id: 'review', href: '/review/', he: 'חזרה', en: 'Review', icon: 'review' },
        { id: 'words', href: '/words/', he: 'מילים', en: 'Words' },
        { id: 'progress', href: '/progress/', he: 'התקדמות', en: 'Progress', icon: 'progress' },
        { id: 'admin', href: '/admin/', he: 'ניהול', en: 'Admin', adminOnly: true, icon: 'admin' }
    ];

    var ICON_PATHS = {
        today: '<circle cx="12" cy="14" r="4"/><path d="M12 6v2M5.5 8.5l1.4 1.4M18.5 8.5 17.1 9.9M4 14h2M18 14h2M6 20h12"/>',
        lessons: '<path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v15H6.5A2.5 2.5 0 0 0 4 20.5Z"/><path d="M4 20.5A2.5 2.5 0 0 1 6.5 18H20"/>',
        review: '<path d="M4 12a8 8 0 0 1 14-5l2 2M20 12a8 8 0 0 1-14 5l-2-2"/><path d="M20 4v5h-5M4 20v-5h5"/>',
        progress: '<path d="M4 20V10M10 20V4M16 20v-8M21 20H3"/>',
        admin: '<path d="M12 3 5 6v5c0 5 3 8 7 10 4-2 7-5 7-10V6Z"/><path d="m9.5 12 2 2 3.5-4"/>'
    };

    function navIcon(name) {
        // Prefer the shared set when loaded; fall back to the built-in paths so
        // tests that load only this file still render icons.
        // A screen with no icon (Words, until one exists) is a text label only.
        if (!name) return '';
        var U = root.HebrewIcons;
        var known = (U && U.PATHS && U.PATHS[name]) || ICON_PATHS[name];
        if (!known) return '';
        if (U && typeof U.icon === 'function') {
            try { return U.icon(name, 18); } catch (err) { /* fall through */ }
        }
        var inner = ICON_PATHS[name];
        return '<svg class="hebrew-icon hebrew-icon-' + name + '" width="18" height="18"'
            + ' viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"'
            + ' stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">'
            + inner + '</svg>';
    }

    function brandMark() {
        var U = root.HebrewIcons;
        if (U && typeof U.logoMark === 'function') {
            try { return U.logoMark(30); } catch (err) { /* fall through */ }
        }
        return '<svg class="hebrew-logo-mark" width="30" height="30" viewBox="0 0 512 512"'
            + ' aria-hidden="true" focusable="false">'
            + '<rect width="512" height="512" rx="112" fill="#4338ca"/>'
            + '<text x="256" y="348" text-anchor="middle" font-family="Rubik, Arial, sans-serif"'
            + ' font-weight="700" font-size="280" fill="#ffffff">ע</text>'
            + '<circle cx="392" cy="128" r="36" fill="#fbbf24"/></svg>';
    }

    function escapeHtml(str) {
        var U = root.HebrewUtils;
        if (U && typeof U.escapeHtml === 'function') return U.escapeHtml(str);
        return String(str == null ? '' : str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function safeNextPath(value) {
        var Api = root.HebrewApi;
        if (Api && typeof Api.safeNextPath === 'function') return Api.safeNextPath(value);
        if (typeof value !== 'string' || !value) return '/';
        var next = value.trim();
        if (!next || !next.startsWith('/') || next.startsWith('//') || next.indexOf('\\') !== -1) {
            return '/';
        }
        return next;
    }

    /**
     * Which nav id matches the current path.
     * @param {string} pathname
     * @returns {string|null}
     */
    function currentNavId(pathname) {
        var path = String(pathname || '/');
        if (path === '/lessons' || path.indexOf('/lessons/') === 0) return 'lessons';
        if (path === '/guides' || path.indexOf('/guides/') === 0) return 'guides';
        if (path === '/review' || path.indexOf('/review/') === 0) return 'review';
        if (path === '/words' || path.indexOf('/words/') === 0) return 'words';
        if (path === '/progress' || path.indexOf('/progress/') === 0) return 'progress';
        if (path === '/admin' || path.indexOf('/admin/') === 0) return 'admin';
        if (path.indexOf('/worksheets/') === 0) return 'lessons';
        if (path === '/' || path === '/index.html' || path.indexOf('/index.html') === path.length - 11) {
            return 'today';
        }
        return 'today';
    }

    /**
     * Build nav + chrome HTML for a /api/me-shaped user.
     * @param {object|null} me
     * @param {string} pathname
     * @returns {string}
     */
    function renderHtml(me, pathname) {
        var role = me && me.role;
        var current = currentNavId(pathname);
        var links = NAV.filter(function (item) {
            return !item.adminOnly || role === 'admin';
        }).map(function (item) {
            var cur = item.id === current ? ' aria-current="page"' : '';
            var label = navIcon(item.icon) + '<span class="hebrew-shell-label">'
                + escapeHtml(item.he) + ' <span class="en" dir="ltr">(' + escapeHtml(item.en) + ')</span></span>';
            return '<a class="hebrew-shell-link" href="' + escapeHtml(item.href) + '"' + cur
                + ' data-nav="' + escapeHtml(item.id) + '">' + label + '</a>';
        }).join('');

        var display = escapeHtml((me && (me.display_name || me.username)) || '');
        return ''
            + '<div class="hebrew-shell" dir="rtl" lang="he">'
            + '<div class="hebrew-shell-inner">'
            + '<a class="hebrew-brand" href="/" aria-label="דרך עברית — Today">'
            + brandMark()
            + '<span class="hebrew-brand-text"><span class="hebrew-brand-he">דרך עברית</span>'
            + '<span class="hebrew-brand-en" dir="ltr">Hebrew Way</span></span></a>'
            + '<nav class="hebrew-shell-nav" aria-label="Primary">' + links + '</nav>'
            + '<div class="hebrew-shell-sync" data-shell-sync aria-live="polite"></div>'
            + '<div class="hebrew-shell-account">'
            + '<button type="button" class="hebrew-shell-account-btn" id="hebrew-shell-account-btn"'
            + ' aria-expanded="false" aria-haspopup="true" aria-controls="hebrew-shell-account-menu">'
            + '<span class="hebrew-shell-account-name">' + display + '</span>'
            + '<span class="hebrew-shell-account-caret" aria-hidden="true">▾</span>'
            + '</button>'
            + '<div class="hebrew-shell-account-menu hidden" id="hebrew-shell-account-menu" role="menu" hidden>'
            + '<p class="hebrew-shell-menu-name" role="presentation">' + display + '</p>'
            + '<button type="button" class="hebrew-shell-menu-item" role="menuitem" data-shell-action="change-password">'
            + 'החלפת סיסמה <span class="en" dir="ltr">(Change password)</span></button>'
            + '<button type="button" class="hebrew-shell-menu-item" role="menuitem" data-shell-action="sign-out">'
            + 'יציאה <span class="en" dir="ltr">(Sign out)</span></button>'
            + '<p class="hebrew-shell-privacy" role="note">' + escapeHtml(PRIVACY_NOTE) + '</p>'
            + '<p class="hebrew-shell-privacy" data-ai-disclosure="1" role="note">' + escapeHtml(AI_DISCLOSURE) + '</p>'
            + '</div></div></div></div>'
            + '<div class="hebrew-shell-pw-dialog hidden" id="hebrew-shell-pw-dialog" role="dialog"'
            + ' aria-modal="true" aria-labelledby="hebrew-shell-pw-title" hidden>'
            + '<div class="hebrew-shell-pw-panel">'
            + '<h2 id="hebrew-shell-pw-title">יש להחליף סיסמה <span class="en" dir="ltr">(Change your password)</span></h2>'
            + '<p class="hebrew-shell-pw-lead">המנהל איפס את הסיסמה. בחרו סיסמה חדשה לפני שממשיכים.'
            + ' <span class="en" dir="ltr">(An admin reset your password. Choose a new one before continuing.)</span></p>'
            + '<form id="hebrew-shell-pw-form" class="hebrew-shell-pw-form" autocomplete="on">'
            // Hidden username so password managers file the new password under the right account.
            + '<input type="text" name="username" autocomplete="username" hidden readonly tabindex="-1"'
            + ' aria-hidden="true" value="' + escapeHtml((me && me.username) || '') + '">'
            + '<label class="hebrew-shell-field">סיסמה נוכחית <span class="en" dir="ltr">(Current password)</span>'
            + '<input type="password" name="current_password" id="hebrew-shell-pw-current"'
            + ' autocomplete="current-password" required minlength="1"></label>'
            + '<label class="hebrew-shell-field">סיסמה חדשה <span class="en" dir="ltr">(New password)</span>'
            + '<input type="password" name="new_password" id="hebrew-shell-pw-new"'
            + ' autocomplete="new-password" required minlength="10"></label>'
            + '<p class="hebrew-shell-pw-error" id="hebrew-shell-pw-error" hidden></p>'
            + '<button type="submit" class="btn btn-primary" id="hebrew-shell-pw-submit">'
            + 'שמירה <span class="en" dir="ltr">(Save)</span></button>'
            + '</form></div></div>';
    }

    function setMenuOpen(rootEl, open) {
        var btn = rootEl.querySelector('#hebrew-shell-account-btn');
        var menu = rootEl.querySelector('#hebrew-shell-account-menu');
        if (!btn || !menu) return;
        btn.setAttribute('aria-expanded', open ? 'true' : 'false');
        if (open) {
            menu.classList.remove('hidden');
            menu.hidden = false;
        } else {
            menu.classList.add('hidden');
            menu.hidden = true;
        }
    }

    function setPasswordDialog(rootEl, open, forced) {
        var dlg = rootEl.querySelector('#hebrew-shell-pw-dialog');
        if (!dlg) return;
        dlg.dataset.forced = forced ? '1' : '0';
        if (open) {
            dlg.classList.remove('hidden');
            dlg.hidden = false;
            var first = dlg.querySelector('input');
            if (first && typeof first.focus === 'function') {
                try { first.focus(); } catch (err) { /* ignore */ }
            }
        } else {
            dlg.classList.add('hidden');
            dlg.hidden = true;
        }
        if (forced && open) {
            rootEl.classList.add('hebrew-shell-nav-locked');
        } else if (!forced) {
            rootEl.classList.remove('hebrew-shell-nav-locked');
        }
    }

    function bindShell(rootEl, state) {
        var nav = rootEl.querySelector('.hebrew-shell-nav');
        if (nav) {
            nav.addEventListener('click', function (event) {
                if (!state.mustChange) return;
                var link = event.target.closest && event.target.closest('a.hebrew-shell-link');
                if (link) {
                    event.preventDefault();
                    setPasswordDialog(rootEl, true, true);
                }
            });
        }

        var accountBtn = rootEl.querySelector('#hebrew-shell-account-btn');
        var menu = rootEl.querySelector('#hebrew-shell-account-menu');
        if (accountBtn && menu) {
            accountBtn.addEventListener('click', function () {
                var open = accountBtn.getAttribute('aria-expanded') !== 'true';
                setMenuOpen(rootEl, open);
            });
            accountBtn.addEventListener('keydown', function (event) {
                if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    var open = accountBtn.getAttribute('aria-expanded') !== 'true';
                    setMenuOpen(rootEl, open);
                    if (open) {
                        var firstItem = menu.querySelector('[role="menuitem"]');
                        if (firstItem) firstItem.focus();
                    }
                } else if (event.key === 'Escape') {
                    setMenuOpen(rootEl, false);
                    accountBtn.focus();
                } else if (event.key === 'ArrowDown') {
                    event.preventDefault();
                    setMenuOpen(rootEl, true);
                    var item = menu.querySelector('[role="menuitem"]');
                    if (item) item.focus();
                }
            });
            menu.addEventListener('keydown', function (event) {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    setMenuOpen(rootEl, false);
                    accountBtn.focus();
                }
            });
            document.addEventListener('click', function (event) {
                if (!rootEl.contains(event.target)) setMenuOpen(rootEl, false);
            });
        }

        rootEl.addEventListener('click', function (event) {
            var actionBtn = event.target.closest && event.target.closest('[data-shell-action]');
            if (!actionBtn) return;
            var action = actionBtn.getAttribute('data-shell-action');
            setMenuOpen(rootEl, false);
            if (action === 'sign-out') {
                var Api = root.HebrewApi;
                if (!Api) return;
                Api.signOut().then(function () {
                    root.location.href = '/login/';
                }).catch(function () {
                    root.location.href = '/login/';
                });
                return;
            }
            if (action === 'change-password') {
                setPasswordDialog(rootEl, true, !!state.mustChange);
            }
        });

        var form = rootEl.querySelector('#hebrew-shell-pw-form');
        if (form) {
            form.addEventListener('submit', function (event) {
                event.preventDefault();
                var errEl = rootEl.querySelector('#hebrew-shell-pw-error');
                var current = rootEl.querySelector('#hebrew-shell-pw-current');
                var next = rootEl.querySelector('#hebrew-shell-pw-new');
                var submit = rootEl.querySelector('#hebrew-shell-pw-submit');
                if (errEl) {
                    errEl.hidden = true;
                    errEl.textContent = '';
                }
                var Api = root.HebrewApi;
                if (!Api || typeof Api.changePassword !== 'function') return;
                if (submit) submit.disabled = true;
                Api.changePassword(
                    current ? current.value : '',
                    next ? next.value : ''
                ).then(function (res) {
                    if (submit) submit.disabled = false;
                    if (res.ok) {
                        state.mustChange = false;
                        state.me = res.data || state.me;
                        if (state.me) state.me.must_change_password = false;
                        setPasswordDialog(rootEl, false, false);
                        rootEl.classList.remove('hebrew-shell-nav-locked');
                        if (form) form.reset();
                        return;
                    }
                    if (errEl) {
                        errEl.hidden = false;
                        errEl.textContent = res.error
                            || 'לא ניתן לשמור את הסיסמה. (Could not save the password.)';
                    }
                }).catch(function () {
                    if (submit) submit.disabled = false;
                    if (errEl) {
                        errEl.hidden = false;
                        errEl.textContent = 'שגיאת רשת. נסו שוב. (Network error. Try again.)';
                    }
                });
            });
        }

        // Forced dialog: block Escape / outside dismiss
        var dlg = rootEl.querySelector('#hebrew-shell-pw-dialog');
        if (dlg) {
            dlg.addEventListener('keydown', function (event) {
                if (event.key === 'Escape' && dlg.dataset.forced === '1') {
                    event.preventDefault();
                    event.stopPropagation();
                }
            });
        }
    }

    /**
     * @param {HTMLElement} target
     * @param {{ me?: object|null, pathname?: string, skipFetch?: boolean }} [opts]
     * @returns {Promise<object|null>}
     */
    /**
     * Update aria-current on nav links without re-rendering the shell (ADR-0007).
     * @param {string} pathname
     */
    function setActiveNav(pathname) {
        var current = currentNavId(pathname);
        var rootEl = root.document && root.document.querySelector('.hebrew-shell');
        if (!rootEl) return;
        var links = rootEl.querySelectorAll('[data-nav]');
        for (var i = 0; i < links.length; i += 1) {
            var link = links[i];
            var id = link.getAttribute('data-nav');
            if (id === current) link.setAttribute('aria-current', 'page');
            else link.removeAttribute('aria-current');
        }
    }

    function mount(target, opts) {
        if (!target) return Promise.resolve(null);
        var options = opts || {};
        var embedded = options.me != null ? options.me : root.__HEBREW_ME__;
        var state = { me: embedded || null, mustChange: false };

        function paint(me) {
            state.me = me;
            state.mustChange = !!(me && me.must_change_password);
            // Read the URL at paint time: in the app shell it changes without a reload.
            var pathname = options.pathname != null
                ? options.pathname
                : ((root.location && root.location.pathname) || '/');
            target.innerHTML = renderHtml(me, pathname);
            bindShell(target, state);
            if (state.mustChange) {
                setPasswordDialog(target, true, true);
            }
            try {
                document.documentElement.classList.add('has-hebrew-shell');
                document.body.classList.add('has-hebrew-shell');
            } catch (err) { /* ignore */ }
            return me;
        }

        if (options.skipFetch) {
            paint(embedded || null);
            return Promise.resolve(embedded || null);
        }

        // ADR-0007: the server embedded the user in this same response, so paint once and
        // do not re-fetch /api/me. The shell is painted once per document; route changes
        // only call setActiveNav(). Never re-render the shell on navigation.
        if (embedded) {
            paint(embedded);
            return Promise.resolve(embedded);
        }

        var Api = root.HebrewApi;
        if (!Api || typeof Api.me !== 'function') {
            paint(null);
            return Promise.resolve(null);
        }

        target.innerHTML = '<div class="hebrew-shell hebrew-shell-loading" dir="rtl" lang="he">'
            + '<div class="hebrew-shell-inner"><span class="hebrew-shell-loading-label">…</span></div></div>';

        return Api.me().then(function (res) {
            if (!res.ok) {
                paint(null);
                return null;
            }
            return paint(res.data);
        }).catch(function () {
            paint(null);
            return null;
        });
    }

    /**
     * Ensure api.js + this file are available from the lesson player.
     * @param {function(): void} done
     */
    function ensureLoaded(done) {
        if (root.HebrewApi && root.HebrewShell) {
            done();
            return;
        }
        var base = '../../shared/';
        try {
            var scripts = document.getElementsByTagName('script');
            for (var i = 0; i < scripts.length; i += 1) {
                var src = scripts[i].src || '';
                if (/\/player\/core\.js(\?|$)/.test(src)) {
                    base = src.replace(/player\/core\.js(\?.*)?$/, '');
                    break;
                }
                if (/\/app-shell\.js(\?|$)/.test(src)) {
                    base = src.replace(/app-shell\.js(\?.*)?$/, '');
                    break;
                }
            }
        } catch (err) { /* use default */ }

        function load(src, next) {
            var el = document.createElement('script');
            el.src = src;
            el.onload = next;
            el.onerror = next;
            (document.head || document.documentElement).appendChild(el);
        }

        var chain = function () { done(); };
        if (!root.HebrewShell) {
            var prev = chain;
            chain = function () { load(base + 'app-shell.js', prev); };
        }
        if (!root.HebrewApi) {
            var prev2 = chain;
            chain = function () { load(base + 'api.js', prev2); };
        }
        chain();
    }

    root.HebrewShell = {
        PRIVACY_NOTE: PRIVACY_NOTE,
        AI_DISCLOSURE: AI_DISCLOSURE,
        NAV: NAV,
        escapeHtml: escapeHtml,
        safeNextPath: safeNextPath,
        currentNavId: currentNavId,
        renderHtml: renderHtml,
        setActiveNav: setActiveNav,
        mount: mount,
        ensureLoaded: ensureLoaded
    };
})(typeof window !== 'undefined' ? window : globalThis);
