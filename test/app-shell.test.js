/**
 * DOM-level tests for HebrewApi + HebrewShell (SPEC-012).
 * Provenance: INIT-002/SPEC-012
 * Theme engine: INIT-013/SPEC-002
 * Adaptive shell rail/dock: INIT-013/SPEC-003
 */
'use strict';

const { describe, it, before, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadBrowserScripts() {
    const store = Object.create(null);
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        fetch: () => Promise.reject(new Error('fetch should be stubbed')),
        location: { pathname: '/', search: '', href: 'http://127.0.0.1/' },
        localStorage: {
            getItem(key) { return Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null; },
            setItem(key, value) { store[key] = String(value); },
            removeItem(key) { delete store[key]; },
            clear() { for (const k of Object.keys(store)) delete store[k]; },
            _store: store
        },
        document: {
            readyState: 'complete',
            documentElement: {
                attrs: Object.create(null),
                classList: { add() {}, remove() {} },
                setAttribute(name, value) { this.attrs[name] = String(value); },
                getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
                removeAttribute(name) { delete this.attrs[name]; }
            },
            body: { classList: { add() {}, remove() {} } },
            addEventListener() {},
            querySelector() { return null; }
        }
    };
    const scheme = {
        matches: false,
        listeners: [],
        addEventListener(type, fn) { if (type === 'change') this.listeners.push(fn); },
        addListener(fn) { this.listeners.push(fn); },
        dispatch() {
            const ev = { matches: this.matches, media: '(prefers-color-scheme: dark)' };
            this.listeners.forEach((fn) => fn(ev));
        }
    };
    sandbox.matchMedia = () => scheme;
    sandbox._colorScheme = scheme;
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'api.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'app-shell.js'), 'utf8'),
        sandbox
    );
    return sandbox;
}

describe('app-shell (SPEC-012)', () => {
    let win;
    let Api;
    let Shell;

    before(() => {
        win = loadBrowserScripts();
        Api = win.HebrewApi;
        Shell = win.HebrewShell;
    });

    beforeEach(() => {
        win.localStorage.clear();
        win.document.documentElement.removeAttribute('data-theme');
    });

    it('escapes hostile display names as text (ac-6)', () => {
        const dirty = '<img src=x onerror=alert(1)>';
        const html = Shell.renderHtml({
            id: 1,
            username: 'u',
            display_name: dirty,
            role: 'user',
            must_change_password: false
        }, '/');
        assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.doesNotMatch(html, /<img src=x/);
        assert.equal(Shell.escapeHtml(dirty), '&lt;img src=x onerror=alert(1)&gt;');
    });

    it('shows Admin link only for admin role (ac-2)', () => {
        const userHtml = Shell.renderHtml({
            id: 1,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            must_change_password: false
        }, '/');
        const adminHtml = Shell.renderHtml({
            id: 2,
            username: 'admin',
            display_name: 'Admin',
            role: 'admin',
            must_change_password: false
        }, '/');
        assert.doesNotMatch(userHtml, /data-nav="admin"/);
        assert.doesNotMatch(userHtml, /href="\/admin\/"/);
        assert.doesNotMatch(userHtml, /data-nav="editor"/);
        assert.doesNotMatch(userHtml, /href="\/editor\/"/);
        assert.match(adminHtml, /data-nav="admin"/);
        assert.match(adminHtml, /href="\/admin\/"/);
        assert.match(adminHtml, /ניהול/);
        assert.match(adminHtml, />Admin</);
        assert.match(adminHtml, /data-nav="editor"/);
        assert.match(adminHtml, /href="\/editor\/"/);
        assert.match(adminHtml, /עריכה/);
        assert.match(adminHtml, />Editor</);
    });

    it('marks the current page with aria-current="page" (ac-2)', () => {
        const today = Shell.renderHtml({
            id: 1, username: 'a', display_name: 'A', role: 'user', must_change_password: false
        }, '/');
        assert.match(today, /href="\/"[^>]*aria-current="page"|aria-current="page"[^>]*href="\/"/);

        const review = Shell.renderHtml({
            id: 1, username: 'a', display_name: 'A', role: 'user', must_change_password: false
        }, '/review/');
        assert.match(review, /data-nav="review"[^>]*aria-current="page"|aria-current="page"[^>]*data-nav="review"/);

        const lesson = Shell.renderHtml({
            id: 1, username: 'a', display_name: 'A', role: 'user', must_change_password: false
        }, '/worksheets/my-morning/index.html');
        assert.match(lesson, /data-nav="lessons"[^>]*aria-current="page"|aria-current="page"[^>]*data-nav="lessons"/);
    });

    it('renders Lessons, Progress, and Admin nav targets for shell module (ac-2)', () => {
        const html = Shell.renderHtml({
            id: 2,
            username: 'admin',
            display_name: 'Admin',
            role: 'admin',
            must_change_password: false
        }, '/');
        assert.match(html, /href="\/lessons\/"/);
        assert.match(html, /שיעורים/);
        assert.match(html, />Lessons</);
        assert.match(html, /href="\/progress\/"/);
        assert.match(html, /התקדמות/);
        assert.match(html, />Progress</);
        assert.match(html, /href="\/admin\/"/);
        assert.match(html, /href="\/review\/"/);
        assert.match(html, /חזרה/);
        assert.match(html, /היום/);
    });

    it('rejects hostile next paths (ac-1)', () => {
        assert.equal(Api.safeNextPath('//evil.example'), '/');
        assert.equal(Api.safeNextPath('https://evil.example/'), '/');
        assert.equal(Api.safeNextPath('\\evil'), '/');
        assert.equal(Api.safeNextPath('evil'), '/');
        assert.equal(Api.safeNextPath('/review/'), '/review/');
        assert.equal(Shell.safeNextPath('//evil.example'), '/');
    });

    it('shows the privacy note in account menu HTML (ac-4)', () => {
        const html = Shell.renderHtml({
            id: 1,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            must_change_password: false
        }, '/');
        assert.match(html, /מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה/);
        assert.match(html, /Admins can see all your activity, including every answer/);
        assert.equal(
            Shell.PRIVACY_NOTE,
            'מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה. (Admins can see all your activity, including every answer.)'
        );
    });

    it('login page HTML includes the privacy note (ac-4)', () => {
        const login = fs.readFileSync(path.join(root, 'login', 'index.html'), 'utf8');
        assert.match(login, /מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה/);
        assert.match(login, /Admins can see all your activity, including every answer/);
        // Signed-out /shared/* is gated (SPEC-008); login is self-contained.
        assert.match(login, /\/api\/session/);
        assert.match(login, /safeNextPath/);
        assert.ok(fs.existsSync(path.join(root, 'shared', 'login.js')));
        assert.ok(fs.existsSync(path.join(root, 'shared', 'api.js')));
    });

    it('mount paints escaped name into a host element', () => {
        const dirty = '<img src=x onerror=alert(1)>';
        const host = {
            innerHTML: '',
            classList: { add() {}, remove() {}, contains() { return false; } },
            querySelector() { return null; },
            querySelectorAll() { return []; },
            addEventListener() {},
            contains() { return false; },
            className: ''
        };
        // Minimal: renderHtml path already covered; mount with skipFetch
        return Shell.mount(host, {
            skipFetch: true,
            me: {
                id: 1,
                username: 'x',
                display_name: dirty,
                role: 'user',
                must_change_password: false
            },
            pathname: '/'
        }).then(() => {
            assert.match(host.innerHTML, /&lt;img src=x onerror=alert\(1\)&gt;/);
            assert.doesNotMatch(host.innerHTML, /<img src=x/);
            assert.match(host.innerHTML, /hebrew-shell/);
            assert.match(host.innerHTML, /data-shell-sync/);
        });
    });
});

describe('app-shell theme engine (INIT-013/SPEC-002)', () => {
    let win;
    let Shell;

    before(() => {
        win = loadBrowserScripts();
        Shell = win.HebrewShell;
    });

    beforeEach(() => {
        win.localStorage.clear();
        win.document.documentElement.removeAttribute('data-theme');
        win._colorScheme.matches = false;
    });

    it('defaults to auto and allowlists light, dark, and auto (ac-3, security)', () => {
        assert.equal(Shell.THEME_KEY, 'hebrew-theme');
        assert.equal(Shell.normalizeTheme('dark'), 'dark');
        assert.equal(Shell.normalizeTheme('light'), 'light');
        assert.equal(Shell.normalizeTheme('auto'), 'auto');
        assert.equal(Shell.normalizeTheme('<script>'), 'auto');
        assert.equal(Shell.normalizeTheme(null), 'auto');
        assert.equal(Shell.readTheme(), 'auto');
        win.localStorage.setItem('hebrew-theme', 'evil');
        assert.equal(Shell.readTheme(), 'auto');
        win.localStorage.setItem('hebrew-theme', 'dark');
        assert.equal(Shell.readTheme(), 'dark');
    });

    it('applyTheme persists the choice and writes a concrete data-theme', () => {
        assert.equal(Shell.applyTheme('dark'), 'dark');
        assert.equal(win.document.documentElement.getAttribute('data-theme'), 'dark');
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'dark');
        assert.equal(Shell.applyTheme('nope'), 'auto');
        assert.equal(win.document.documentElement.getAttribute('data-theme'), 'light');
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'auto');
        assert.notEqual(win.document.documentElement.getAttribute('data-theme'), 'auto');
    });

    it('re-resolves auto when the OS color scheme changes', () => {
        assert.equal(Shell.applyTheme('auto'), 'auto');
        assert.equal(win.document.documentElement.getAttribute('data-theme'), 'light');
        win._colorScheme.matches = true;
        win._colorScheme.dispatch();
        assert.equal(win.document.documentElement.getAttribute('data-theme'), 'dark');
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'auto');
        Shell.applyTheme('light');
        win._colorScheme.matches = false;
        win._colorScheme.dispatch();
        assert.equal(win.document.documentElement.getAttribute('data-theme'), 'light');
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'light');
    });

    it('toggleTheme flips light ↔ dark and persists (ac-3)', () => {
        assert.equal(Shell.toggleTheme(), 'dark');
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'dark');
        assert.equal(Shell.toggleTheme(), 'light');
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'light');
    });

    it('renderHtml includes a 3-way theme control and shows the resolved theme (ac-3, REQ-016)', () => {
        const html = Shell.renderHtml({
            id: 1,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            must_change_password: false
        }, '/');
        assert.match(html, /data-theme-toggle/);
        assert.match(html, /data-theme-switch/);
        assert.match(html, /role="radiogroup"/);
        assert.match(html, /class="theme-switch hebrew-shell-theme-toggle"/);
        assert.match(html, /בהיר/);
        assert.match(html, /כהה/);
        assert.match(html, /אוטומטי/);
        assert.match(html, /data-theme-choice="auto"[^>]*aria-checked="true"/);
        assert.match(html, /data-resolved="light"/);
        assert.match(html, /aria-label="ערכת נושא"/);
        assert.doesNotMatch(html, /data-theme-toggle[^>]*aria-label="[A-Za-z]/);
        assert.doesNotMatch(html, /hebrew-shell-theme-toggle[^>]*title=/);

        win._colorScheme.matches = true;
        const autoDark = Shell.renderHtml({
            id: 1, username: 'learner', display_name: 'Learner', role: 'user', must_change_password: false
        }, '/');
        assert.match(autoDark, /data-resolved="dark"/);
        assert.match(autoDark, /data-theme-choice="auto"[^>]*aria-checked="true"/);
        assert.match(autoDark, /data-theme-resolved>כהה</);
        win._colorScheme.matches = false;

        Shell.applyTheme('dark');
        const darkHtml = Shell.renderHtml({
            id: 1, username: 'learner', display_name: 'Learner', role: 'user', must_change_password: false
        }, '/');
        assert.match(darkHtml, /data-theme-choice="dark"[^>]*aria-checked="true"/);
        assert.match(darkHtml, /data-resolved="dark"/);
        Shell.applyTheme('auto');
        const back = Shell.renderHtml({
            id: 1, username: 'learner', display_name: 'Learner', role: 'user', must_change_password: false
        }, '/');
        assert.match(back, /data-theme-choice="auto"[^>]*aria-checked="true"/);
        assert.equal(win.localStorage.getItem('hebrew-theme'), 'auto');
    });
});

describe('app-shell adaptive layout hooks (INIT-013/SPEC-003)', () => {
    let win;
    let Shell;

    before(() => {
        win = loadBrowserScripts();
        Shell = win.HebrewShell;
    });

    it('preserves all shell hooks and DOM order brand → nav → sync → account', () => {
        const html = Shell.renderHtml({
            id: 1,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            must_change_password: false
        }, '/');
        assert.match(html, /hebrew-brand[\s\S]*hebrew-shell-nav[\s\S]*data-shell-sync[\s\S]*hebrew-shell-account/);
        assert.match(html, /id="hebrew-shell-account-btn"/);
        assert.match(html, /id="hebrew-shell-account-menu"/);
        assert.match(html, /id="hebrew-shell-pw-dialog"/);
        assert.match(html, /class="hebrew-shell-nav"/);
        assert.match(html, /class="hebrew-brand"/);
        assert.match(html, /data-brand/);
        assert.match(html, /data-theme-toggle/);
        // Learner: 7 nav links, none hidden via markup.
        assert.equal((html.match(/data-nav="/g) || []).length, 7);
    });

    it('admin still sees Editor + Admin links (aud-1 / und-1)', () => {
        const html = Shell.renderHtml({
            id: 2,
            username: 'admin',
            display_name: 'Admin',
            role: 'admin',
            must_change_password: false
        }, '/');
        const navCount = (html.match(/data-nav="/g) || []).length;
        assert.equal(navCount, 9);
        assert.match(html, /data-nav="editor"/);
        assert.match(html, /data-nav="admin"/);
    });

    it('mount adds has-hebrew-shell on html and body for CSS Grid rail', () => {
        const added = [];
        win.document.documentElement.classList = {
            add(name) { added.push('html:' + name); },
            remove() {}
        };
        win.document.body.classList = {
            add(name) { added.push('body:' + name); },
            remove() {}
        };
        const host = {
            innerHTML: '',
            classList: { add() {}, remove() {}, contains() { return false; } },
            querySelector() { return null; },
            querySelectorAll() { return []; },
            addEventListener() {},
            contains() { return false; },
            className: ''
        };
        return Shell.mount(host, {
            skipFetch: true,
            me: {
                id: 1, username: 'x', display_name: 'X', role: 'user', must_change_password: false
            },
            pathname: '/'
        }).then(() => {
            assert.ok(added.includes('html:has-hebrew-shell'));
            assert.ok(added.includes('body:has-hebrew-shell'));
            assert.match(host.innerHTML, /data-shell-sync/);
            assert.match(host.innerHTML, /hebrew-shell-account/);
        });
    });

    it('applyUiLang switches data-ui-lang and .hebrew-shell.ui-chrome dir (ac-3)', () => {
        const U = win.HebrewUtils;
        assert.ok(U && typeof U.applyUiLang === 'function');

        const shell = {
            attrs: Object.create(null),
            className: 'hebrew-shell ui-chrome',
            setAttribute(name, value) { this.attrs[name] = String(value); },
            getAttribute(name) {
                return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null;
            }
        };
        const prevQuery = win.document.querySelectorAll;
        win.document.querySelectorAll = function (sel) {
            if (sel === '.ui-chrome') return [shell];
            if (typeof prevQuery === 'function') return prevQuery.call(this, sel);
            return [];
        };

        U.applyUiLang('en');
        assert.equal(win.document.documentElement.getAttribute('data-ui-lang'), 'en');
        assert.equal(shell.getAttribute('dir'), 'ltr');

        U.applyUiLang('he');
        assert.equal(win.document.documentElement.getAttribute('data-ui-lang'), 'he');
        assert.equal(shell.getAttribute('dir'), 'rtl');

        win.document.querySelectorAll = prevQuery;
    });
});
