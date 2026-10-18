/**
 * Shared top bar (nav + account menu + forced password change).
 * Provenance: INIT-002/SPEC-012
 * Theme persistence: INIT-013/SPEC-002 · ADR-0015 (hebrew-theme localStorage).
 * Adaptive rail / bottom dock: INIT-013/SPEC-003 · ADR-0015 REQ-001/REQ-002.
 */
(function (root) {
    'use strict';

    var PRIVACY_HE = 'מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה.';
    var PRIVACY_EN = 'Admins can see all your activity, including every answer.';
    var PRIVACY_NOTE = 'מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה. (Admins can see all your activity, including every answer.)';
    /* Hebrew line is a draft until the operator approves it (ADR-0009). Provenance: INIT-006/SPEC-017 */
    var AI_DISCLOSURE_EN = 'If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator\'s AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.';
    var AI_DISCLOSURE_HE = 'אם מנהל מפעיל תכנון בינה מלאכותית לחשבון שלך, סיכום של ההתקדמות ושם המשתמש נשלחים לספק הבינה המלאכותית של המפעיל כדי לנסח את השיעור הבא. הסיכום כולל את המילים שתורגלו ומידת הידיעה שלהן, את הדקדוק שתורגל ומידת הדיוק, את השיעורים שהושלמו, ובאיזו תדירות הקריאה היא בלי ניקוד. הסיכום אינו כולל תשובות בודדות, טקסט שהוקלד, או מועדי תרגול.';
    var AI_DISCLOSURE = AI_DISCLOSURE_HE + ' (' + AI_DISCLOSURE_EN + ')';
    /** localStorage key for Light/Dark (INIT-013/SPEC-002). Allowlisted to light|dark only. */
    var THEME_KEY = 'hebrew-theme';

    var NAV = [
        { id: 'today', href: '/', he: 'היום', en: 'Today', icon: 'today' },
        { id: 'lessons', href: '/lessons/', he: 'שיעורים', en: 'Lessons', icon: 'lessons' },
        { id: 'guides', href: '/guides/', he: 'מדריכים', en: 'Guides', icon: 'guides' },
        { id: 'verbs', href: '/verbs/', he: 'פעלים', en: 'Verbs', icon: 'verbs' },
        { id: 'review', href: '/review/', he: 'חזרה', en: 'Review', icon: 'review' },
        { id: 'words', href: '/words/', he: 'מילים', en: 'Words', icon: 'words' },
        { id: 'progress', href: '/progress/', he: 'התקדמות', en: 'Progress', icon: 'progress' },
        { id: 'editor', href: '/editor/', he: 'עריכה', en: 'Editor', adminOnly: true },
        { id: 'admin', href: '/admin/', he: 'ניהול', en: 'Admin', adminOnly: true, icon: 'admin' }
    ];

    // INIT-005/SPEC-010: icons come from the brand sprite (shared/brand/sprite.svg), drawn by
    // name with <use>. The sprite is public (server/web.js) and same-origin, and no CSP is sent
    // on app responses. This file stays self-contained because the lesson player loads it alone.
    var BRAND = '/shared/brand/';
    var ICON_NAMES = ['today', 'lessons', 'review', 'progress', 'admin', 'words', 'guides', 'verbs'];

    /** Decorative icon beside a visible word. aria-hidden: the word is the name. */
    function navIcon(name) {
        if (!name || ICON_NAMES.indexOf(name) === -1) return '';
        return '<svg class="icon hebrew-icon hebrew-icon-' + name + '" width="20" height="20"'
            + ' aria-hidden="true" focusable="false"><use href="' + BRAND + 'sprite.svg#icon-' + name
            + '"></use></svg>';
    }

    /** The chosen mark (option A, Ayin tile). Decorative: the wordmark text beside it is the name. */
    function brandMark() {
        return '<img class="hebrew-logo-mark" src="' + BRAND + 'mark-a.svg" width="30" height="30"'
            + ' alt="" decoding="async">';
    }

    /**
     * Put the favicon, touch icon and manifest links in <head> when the document did not
     * ship them (lesson player pages are not edited by this spec). Idempotent.
     */
    function ensureBrandHead() {
        var doc = root.document;
        var head = doc && doc.head;
        if (!head || typeof head.appendChild !== 'function' || typeof doc.createElement !== 'function') return;
        [
            { rel: 'icon', type: 'image/svg+xml', href: BRAND + 'favicon.svg' },
            { rel: 'icon', type: 'image/png', sizes: '32x32', href: BRAND + 'favicon-32.png' },
            { rel: 'apple-touch-icon', href: BRAND + 'apple-touch-icon.png' },
            { rel: 'manifest', href: '/manifest.webmanifest' }
        ].forEach(function (spec) {
            var sel = 'link[rel="' + spec.rel + '"]' + (spec.type ? '[type="' + spec.type + '"]' : '');
            if (typeof head.querySelector === 'function' && head.querySelector(sel)) return;
            var link = doc.createElement('link');
            Object.keys(spec).forEach(function (k) { link.setAttribute(k, spec[k]); });
            head.appendChild(link);
        });
    }

    function bi(he, en) {
        var U = root.HebrewUtils;
        if (U && typeof U.bi === 'function') return U.bi(he, en);
        var h = escapeHtml(he || '');
        var e = escapeHtml(en || '');
        if (!e) return h;
        if (!h) return '<span class="ui-en" lang="en" dir="ltr">' + e + '</span>';
        return '<span class="ui-pair"><span class="ui-he" lang="he">' + h + '</span>'
            + '<span class="ui-en" lang="en" dir="ltr">' + e + '</span></span>';
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

    /**
     * Allowlist localStorage theme to 'light' | 'dark' (default light).
     * Provenance: INIT-013/SPEC-002 · ADR-0015 REQ-005.
     * @param {string|null|undefined} val
     * @returns {'light'|'dark'}
     */
    function normalizeTheme(val) {
        return val === 'dark' ? 'dark' : 'light';
    }

    /**
     * Read persisted theme. Missing/invalid → 'light'.
     * @returns {'light'|'dark'}
     */
    function readTheme() {
        try {
            var store = root.localStorage;
            if (!store || typeof store.getItem !== 'function') return 'light';
            return normalizeTheme(store.getItem(THEME_KEY));
        } catch (err) {
            return 'light';
        }
    }

    /**
     * Apply theme to <html data-theme> and persist when requested.
     * @param {'light'|'dark'|string} theme
     * @param {{ persist?: boolean }} [opts]
     * @returns {'light'|'dark'}
     */
    function applyTheme(theme, opts) {
        var next = normalizeTheme(theme);
        try {
            var doc = root.document;
            if (doc && doc.documentElement && typeof doc.documentElement.setAttribute === 'function') {
                doc.documentElement.setAttribute('data-theme', next);
            }
        } catch (err) { /* ignore */ }
        if (!opts || opts.persist !== false) {
            try {
                var store = root.localStorage;
                if (store && typeof store.setItem === 'function') store.setItem(THEME_KEY, next);
            } catch (err2) { /* ignore quota / private mode */ }
        }
        return next;
    }

    /**
     * Toggle Light ↔ Dark and persist.
     * @returns {'light'|'dark'}
     */
    function toggleTheme() {
        return applyTheme(readTheme() === 'dark' ? 'light' : 'dark');
    }

    /** Bilingual visible label for the theme toggle (no English-only aria-label — REQ-016). */
    function themeToggleLabel(theme) {
        if (theme === 'dark') {
            return bi('מצב בהיר', 'Light mode');
        }
        return bi('מצב כהה', 'Dark mode');
    }

    function renderThemeToggle(theme) {
        var current = normalizeTheme(theme);
        return '<button type="button" class="hebrew-shell-theme-toggle" data-theme-toggle'
            + ' aria-pressed="' + (current === 'dark' ? 'true' : 'false') + '">'
            + themeToggleLabel(current)
            + '</button>';
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
        if (path === '/verbs' || path.indexOf('/verbs/') === 0) return 'verbs';
        if (path === '/review' || path.indexOf('/review/') === 0) return 'review';
        if (path === '/words' || path.indexOf('/words/') === 0) return 'words';
        if (path === '/progress' || path.indexOf('/progress/') === 0) return 'progress';
        if (path === '/editor' || path.indexOf('/editor/') === 0) return 'editor';
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
            var label = navIcon(item.icon) + '<span class="hebrew-shell-label">' + bi(item.he, item.en) + '</span>';
            return '<a class="hebrew-shell-link" href="' + escapeHtml(item.href) + '"' + cur
                + ' data-nav="' + escapeHtml(item.id) + '">' + label + '</a>';
        }).join('');

        var display = escapeHtml((me && (me.display_name || me.username)) || '');
        var theme = readTheme();
        return ''
            + '<div class="hebrew-shell ui-chrome" dir="rtl" lang="he">'
            + '<div class="hebrew-shell-inner">'
            + '<a class="hebrew-brand" href="/" data-brand>'
            + brandMark()
            + '<span class="hebrew-brand-text ui-pair"><span class="hebrew-brand-he ui-he" lang="he">דרך עברית</span>'
            + '<span class="hebrew-brand-en ui-en" lang="en" dir="ltr">Hebrew Way</span></span></a>'
            + '<nav class="hebrew-shell-nav" data-ui-aria-he="ניווט ראשי" data-ui-aria-en="Main" aria-label="ניווט ראשי">' + links + '</nav>'
            + '<div class="hebrew-shell-sync" data-shell-sync aria-live="polite"></div>'
            + '<div class="hebrew-shell-account">'
            + '<div class="ui-lang" role="group" data-ui-aria-he="שפת הממשק" data-ui-aria-en="Interface language" aria-label="שפת הממשק">'
            + '<button type="button" class="ui-lang-btn" data-ui-lang-set="en" aria-pressed="false">EN</button>'
            + '<button type="button" class="ui-lang-btn" data-ui-lang-set="he" aria-pressed="false">עב</button>'
            + '</div>'
            + renderThemeToggle(theme)
            + '<button type="button" class="hebrew-shell-account-btn" id="hebrew-shell-account-btn"'
            + ' aria-expanded="false" aria-haspopup="true" aria-controls="hebrew-shell-account-menu">'
            + '<span class="hebrew-shell-account-name">' + display + '</span>'
            + '<span class="hebrew-shell-account-caret" aria-hidden="true">▾</span>'
            + '</button>'
            + '<div class="hebrew-shell-account-menu hidden" id="hebrew-shell-account-menu" role="menu" hidden>'
            + '<p class="hebrew-shell-menu-name" role="presentation">' + display + '</p>'
            + '<button type="button" class="hebrew-shell-menu-item" role="menuitem" data-shell-action="change-password">'
            + bi('החלפת סיסמה', 'Change password') + '</button>'
            + '<button type="button" class="hebrew-shell-menu-item" role="menuitem" data-shell-action="sign-out">'
            + bi('יציאה', 'Sign out') + '</button>'
            + '<p class="hebrew-shell-privacy" role="note">' + bi(PRIVACY_HE, PRIVACY_EN) + '</p>'
            + '<p class="hebrew-shell-privacy" data-ai-disclosure="1" role="note">' + bi(AI_DISCLOSURE_HE, AI_DISCLOSURE_EN) + '</p>'
            + '</div></div></div></div>'
            + '<div class="hebrew-shell-pw-dialog hidden" id="hebrew-shell-pw-dialog" role="dialog"'
            + ' aria-modal="true" aria-labelledby="hebrew-shell-pw-title" hidden>'
            + '<div class="hebrew-shell-pw-panel">'
            + '<h2 id="hebrew-shell-pw-title">' + bi('יש להחליף סיסמה', 'Change your password') + '</h2>'
            + '<p class="hebrew-shell-pw-lead">' + bi(
                'המנהל איפס את הסיסמה. בחרו סיסמה חדשה לפני שממשיכים.',
                'An admin reset your password. Choose a new one before continuing.'
            ) + '</p>'
            + '<form id="hebrew-shell-pw-form" class="hebrew-shell-pw-form" autocomplete="on">'
            // Hidden username so password managers file the new password under the right account.
            + '<input type="text" name="username" autocomplete="username" hidden readonly tabindex="-1"'
            + ' aria-hidden="true" value="' + escapeHtml((me && me.username) || '') + '">'
            + '<label class="hebrew-shell-field">' + bi('סיסמה נוכחית', 'Current password')
            + '<input type="password" name="current_password" id="hebrew-shell-pw-current"'
            + ' autocomplete="current-password" required minlength="1"></label>'
            + '<label class="hebrew-shell-field">' + bi('סיסמה חדשה', 'New password')
            + '<input type="password" name="new_password" id="hebrew-shell-pw-new"'
            + ' autocomplete="new-password" required minlength="10"></label>'
            + '<p class="hebrew-shell-pw-error" id="hebrew-shell-pw-error" hidden></p>'
            + '<button type="submit" class="btn btn-primary" id="hebrew-shell-pw-submit">'
            + bi('שמירה', 'Save') + '</button>'
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
            var themeBtn = event.target.closest && event.target.closest('[data-theme-toggle]');
            if (themeBtn && rootEl.contains(themeBtn)) {
                var nextTheme = toggleTheme();
                themeBtn.setAttribute('aria-pressed', nextTheme === 'dark' ? 'true' : 'false');
                themeBtn.innerHTML = themeToggleLabel(nextTheme);
                return;
            }
            var langBtn = event.target.closest && event.target.closest('[data-ui-lang-set]');
            if (langBtn && rootEl.contains(langBtn)) {
                var U = root.HebrewUtils;
                if (U && typeof U.setUiLang === 'function') U.setUiLang(langBtn.getAttribute('data-ui-lang-set'));
                return;
            }
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
        revealCurrentLink(rootEl);
    }

    /**
     * On a phone the nav is one row that scrolls sideways (INIT-005/SPEC-011). Scroll only the nav
     * (never the page) so the current link is in view. A no-op where nothing overflows.
     * @param {HTMLElement} rootEl
     */
    function revealCurrentLink(rootEl) {
        try {
            var nav = rootEl.querySelector('.hebrew-shell-nav');
            var cur = nav && nav.querySelector('[aria-current="page"]');
            if (!nav || !cur || typeof cur.getBoundingClientRect !== 'function') return;
            if (!(nav.scrollWidth > nav.clientWidth)) return;
            var navBox = nav.getBoundingClientRect();
            var curBox = cur.getBoundingClientRect();
            nav.scrollLeft += (curBox.left + curBox.width / 2) - (navBox.left + navBox.width / 2);
        } catch (err) { /* cosmetic only */ }
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
            applyTheme(readTheme(), { persist: false });
            target.innerHTML = renderHtml(me, pathname);
            bindShell(target, state);
            if (root.HebrewUtils && typeof root.HebrewUtils.applyUiLang === 'function') {
                root.HebrewUtils.applyUiLang(root.HebrewUtils.readUiLang());
            }
            try { ensureBrandHead(); } catch (err) { /* ignore */ }
            revealCurrentLink(target);
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
        THEME_KEY: THEME_KEY,
        escapeHtml: escapeHtml,
        safeNextPath: safeNextPath,
        currentNavId: currentNavId,
        normalizeTheme: normalizeTheme,
        readTheme: readTheme,
        applyTheme: applyTheme,
        toggleTheme: toggleTheme,
        renderHtml: renderHtml,
        setActiveNav: setActiveNav,
        mount: mount,
        ensureLoaded: ensureLoaded
    };
})(typeof window !== 'undefined' ? window : globalThis);
