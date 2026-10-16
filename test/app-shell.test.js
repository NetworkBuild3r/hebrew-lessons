/**
 * DOM-level tests for HebrewApi + HebrewShell (SPEC-012).
 * Provenance: INIT-002/SPEC-012
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadBrowserScripts() {
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        fetch: () => Promise.reject(new Error('fetch should be stubbed')),
        location: { pathname: '/', search: '', href: 'http://127.0.0.1/' },
        document: {
            readyState: 'complete',
            documentElement: { classList: { add() {}, remove() {} } },
            body: { classList: { add() {}, remove() {} } },
            addEventListener() {},
            querySelector() { return null; }
        }
    };
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
