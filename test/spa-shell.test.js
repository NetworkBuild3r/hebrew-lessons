/**
 * App shell, route table, router, and static caching (INIT-005/SPEC-002..004, ADR-0007).
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const spa = require('../server/spa');
const ROUTES = require('../shared/app-routes.js');

describe('server shell (server/spa.js)', () => {
    it('every signed-in screen is an app path; lessons and login are not', () => {
        for (const p of ['/', '/index.html', '/lessons', '/lessons/', '/lessons/index.html',
            '/review/', '/guides/', '/guides', '/guides/index.html', '/words/', '/words', '/words/index.html', '/progress/', '/admin/', '/admin/index.html']) {
            assert.equal(spa.isAppPath(p), true, p);
        }
        for (const p of ['/login/', '/worksheets/my-morning/', '/shared/worksheet.css', '/nope/']) {
            assert.equal(spa.isAppPath(p), false, p);
        }
    });

    it('APP_PATHS and the client route table list the same screens', () => {
        assert.deepEqual([...spa.APP_PATHS].sort(), ROUTES.map((r) => r.path).sort());
    });

    it('no per-screen HTML files remain', () => {
        for (const rel of ['index.html', 'lessons/index.html', 'review/index.html',
            'progress/index.html', 'admin/index.html']) {
            assert.equal(fs.existsSync(path.join(root, rel)), false, rel + ' should not exist');
        }
    });

    it('renderShell embeds an /api/me-shaped user and escapes < in JSON', () => {
        const html = spa.renderShell({
            id: 'user-1',
            username: 'learner',
            display_name: 'A <b>name</b>',
            role: 'user',
            must_change_password: false
        });
        assert.match(html, /window\.__HEBREW_ME__ = \{"id":"user-1"/);
        assert.doesNotMatch(html, /%%HEBREW_BOOT_JSON%%/);
        assert.match(html, /\\u003cb>/);
        assert.doesNotMatch(html, /<b>name<\/b>/);
        assert.match(html, /id="app-main"/);
        assert.match(html, /shared\/router\.js/);
        assert.match(html, /shared\/app-routes\.js/);
        assert.match(spa.renderShell({ id: 'u2', username: 'u', display_name: "a$&b$'c", role: 'user' }),
            /"display_name":"a\$&b\$'c"/);
        assert.equal(spa.mePayload({
            id: 'x', username: 'u', display_name: 'D', role: 'admin',
            must_change_password: 1, password_hash: 'secret'
        }).password_hash, undefined);
    });

    it('template has no remote stylesheet', () => {
        const raw = fs.readFileSync(spa.TEMPLATE_PATH, 'utf8');
        assert.doesNotMatch(raw, /href="https?:/);
        assert.match(raw, /href="\/shared\/worksheet\.css"/);
    });
});

describe('route table (shared/app-routes.js)', () => {
    it('each route names a page module that exists and exposes mount/unmount', () => {
        const ids = new Set();
        for (const route of ROUTES) {
            assert.ok(!ids.has(route.id), 'duplicate id ' + route.id);
            ids.add(route.id);
            assert.ok(route.path === '/' || route.path.endsWith('/'), route.path);
            assert.ok(route.rootId && route.view && route.title, route.id);
            for (const src of route.scripts) {
                assert.ok(src.startsWith('/'), 'absolute script url: ' + src);
                assert.ok(fs.existsSync(path.join(root, src)), 'missing ' + src);
            }
            const own = route.scripts[route.scripts.length - 1];
            const code = fs.readFileSync(path.join(root, own), 'utf8');
            assert.match(code, new RegExp('root\\.' + route.view + '\\s*='), route.view + ' in ' + own);
            assert.match(code, /\bmount\s*:/, 'mount exported by ' + own);
            assert.match(code, /\bunmount\s*:/, 'unmount exported by ' + own);
        }
    });

    it('page modules do not auto-run on load', () => {
        for (const route of ROUTES) {
            const own = route.scripts[route.scripts.length - 1];
            const code = fs.readFileSync(path.join(root, own), 'utf8');
            assert.doesNotMatch(code, /DOMContentLoaded/, own);
            assert.doesNotMatch(code, /HebrewSync\.init\(|Sync\.init\(/, own + ' must await ctx.ready instead');
        }
    });
});

/** Minimal browser stand-in: enough DOM for router.js + layout.js. */
function makeBrowser(user) {
    const listeners = { doc: {}, win: {} };
    const roots = {};
    const bodyClasses = new Set();
    const pushes = [];
    const actions = { innerHTML: '' };
    const main = {
        _html: '',
        set innerHTML(v) { this._html = String(v); for (const k of Object.keys(roots)) delete roots[k]; },
        get innerHTML() { return this._html; },
        classList: { add() {}, remove() {} },
        offsetWidth: 0,
        attrs: {},
        setAttribute(k, v) { this.attrs[k] = String(v); },
        removeAttribute(k) { delete this.attrs[k]; },
        focus() {},
        querySelector(sel) {
            if (sel === 'h1') return { focus() { main.focused = 'h1'; } };
            if (sel === '[data-page-actions]') return actions;
            return null;
        }
    };
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        Promise,
        AbortController,
        URL,
        scrollY: 0,
        matchMedia: () => ({ matches: true }),
        requestIdleCallback() {},
        scrollTo() {},
        location: {
            pathname: '/', hash: '', search: '', origin: 'http://app.test',
            get href() { return 'http://app.test' + this.pathname + (this.search || '') + this.hash; },
            reload() { sandbox.reloaded = true; }
        },
        history: {
            state: null,
            pushState(state, _t, url) { pushes.push(url); setUrl(url); this.state = state; },
            replaceState(state, _t, url) { setUrl(url); this.state = state; }
        },
        addEventListener(type, fn) { listeners.win[type] = fn; },
        document: {
            title: '',
            body: { classList: { add: (c) => bodyClasses.add(c), remove: (c) => bodyClasses.delete(c) } },
            addEventListener(type, fn) { listeners.doc[type] = fn; },
            querySelectorAll() { return []; },
            getElementById(id) {
                if (id === 'app-route-announce') return { textContent: '' };
                if (!main.innerHTML.includes('id="' + id + '"')) return null;
                roots[id] = roots[id] || { id, innerHTML: '' };
                return roots[id];
            }
        }
    };
    function setUrl(url) {
        const u = new URL(url, 'http://app.test');
        sandbox.location.pathname = u.pathname;
        sandbox.location.hash = u.hash;
        sandbox.location.search = u.search;
    }
    sandbox.window = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'layout.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'router.js'), 'utf8'), sandbox);
    return { win: sandbox, main, listeners, bodyClasses, pushes, actions, roots, user };
}

function view(log, name, extra) {
    return Object.assign({
        mount(el, ctx) { log.push(name + ':mount'); log.ctx = ctx; log.el = el; },
        unmount() { log.push(name + ':unmount'); }
    }, extra || {});
}

const TEST_ROUTES = [
    { id: 'today', path: '/', title: 'Today', announce: 'היום', bodyClass: 'today-page',
        rootId: 'today-app', heading: { he: 'היום', en: 'Today' }, scripts: [], view: 'TodayView' },
    { id: 'lessons', path: '/lessons/', title: 'Lessons', announce: 'שיעורים', bodyClass: 'lessons-page',
        rootId: 'lessons-app', heading: { he: 'שיעורים', en: 'Lessons' }, scripts: [], view: 'LessonsView' },
    { id: 'admin', path: '/admin/', title: 'Admin', announce: 'ניהול', bodyClass: 'admin-page',
        rootId: 'admin-app', heading: { he: 'ניהול', en: 'Admin' }, scripts: [], view: 'AdminView',
        adminOnly: true }
];

const tick = () => new Promise((r) => setTimeout(r, 0));

describe('HebrewRouter (shared/router.js)', () => {
    it('opens the first route, then swaps views without touching the shell', async () => {
        const b = makeBrowser();
        const log = [];
        b.win.TodayView = view(log, 'today');
        b.win.LessonsView = view(log, 'lessons');
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main, user: { role: 'user' } });

        assert.match(b.main.innerHTML, /data-page-heading/, 'heading paints before data');
        assert.equal(b.win.document.title, 'Today');
        assert.ok(b.bodyClasses.has('today-page'));
        await tick();
        assert.deepEqual([...log], ['today:mount']);
        assert.equal(b.main.attrs['data-route-ready'], 'today', 'ready signal after mount');
        assert.equal(log.el.id, 'today-app');
        assert.equal(log.ctx.actions, b.actions);
        const firstSignal = log.ctx.signal;
        assert.equal(firstSignal.aborted, false);

        assert.equal(b.win.HebrewRouter.navigate('/lessons'), true);
        assert.equal(firstSignal.aborted, true, 'listeners on ctx.signal are removed on leave');
        assert.equal(b.main.attrs['data-route-ready'], undefined, 'ready signal cleared while loading');
        assert.deepEqual(b.pushes, ['/lessons/']);
        assert.ok(b.bodyClasses.has('lessons-page'));
        assert.ok(!b.bodyClasses.has('today-page'));
        await tick();
        assert.deepEqual([...log], ['today:mount', 'today:unmount', 'lessons:mount']);
    });

    it('returns false for paths that are not app routes', () => {
        const b = makeBrowser();
        b.win.TodayView = view([], 'today');
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main });
        assert.equal(b.win.HebrewRouter.navigate('/worksheets/my-morning/'), false);
    });

    it('canLeave() === false blocks link navigation and Back', async () => {
        const b = makeBrowser();
        const log = [];
        let allow = false;
        b.win.TodayView = view(log, 'today', { canLeave: () => allow });
        b.win.LessonsView = view(log, 'lessons');
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main });
        await tick();

        b.win.HebrewRouter.navigate('/lessons/');
        assert.equal(b.win.location.pathname, '/');
        assert.deepEqual([...log], ['today:mount']);

        b.win.location.pathname = '/lessons/';      // browser already moved on Back/Forward
        b.listeners.win.popstate({ state: null });
        assert.equal(b.win.location.pathname, '/', 'URL is put back');
        assert.deepEqual([...log], ['today:mount']);

        allow = true;
        b.win.HebrewRouter.navigate('/lessons/');
        await tick();
        assert.deepEqual([...log], ['today:mount', 'today:unmount', 'lessons:mount']);
    });

    it('admin-only route shows "No access" to a learner and does not mount', async () => {
        const b = makeBrowser();
        const log = [];
        b.win.TodayView = view(log, 'today');
        b.win.AdminView = view(log, 'admin');
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main, user: { role: 'user' } });
        b.win.HebrewRouter.navigate('/admin/');
        await tick();
        assert.match(b.main.innerHTML, /No access/);
        assert.ok(!log.includes('admin:mount'));
    });

    it('hash-only popstate leaves the open view alone (admin tabs)', async () => {
        const b = makeBrowser();
        const log = [];
        b.win.TodayView = view(log, 'today');
        b.win.AdminView = view(log, 'admin');
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main, user: { role: 'admin' } });
        b.win.HebrewRouter.navigate('/admin/#users');
        await tick();
        b.win.location.hash = '#lessons';
        b.listeners.win.popstate({ state: null });
        await tick();
        assert.equal(log.filter((x) => x === 'admin:mount').length, 1);
    });

    it('intercepts plain left-clicks on app links only', async () => {
        const b = makeBrowser();
        const log = [];
        b.win.TodayView = view(log, 'today');
        b.win.LessonsView = view(log, 'lessons');
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main });
        await tick();

        function click(href, extra) {
            const a = {
                href: new URL(href, 'http://app.test').href,
                target: '',
                getAttribute: (n) => (n === 'href' ? href : null),
                hasAttribute: () => false
            };
            const ev = Object.assign({
                button: 0, defaultPrevented: false,
                target: { closest: () => a },
                preventDefault() { this.defaultPrevented = true; }
            }, extra || {});
            b.listeners.doc.click(ev);
            return ev.defaultPrevented;
        }

        assert.equal(click('/worksheets/my-morning/'), false, 'lesson pages load normally');
        assert.equal(click('/lessons/', { ctrlKey: true }), false, 'ctrl-click opens a tab');
        assert.equal(click('https://example.com/lessons/'), false, 'other origins untouched');
        assert.equal(click('#section'), false, 'in-page anchors untouched');
        assert.equal(click('/lessons/'), true);
        await tick();
        assert.ok(log.includes('lessons:mount'));
    });

    it('normalize maps index.html and missing slashes to canonical paths', () => {
        const b = makeBrowser();
        const n = b.win.HebrewRouter.normalize;
        assert.equal(n('/index.html'), '/');
        assert.equal(n('/review'), '/review/');
        assert.equal(n('/review/index.html'), '/review/');
        assert.equal(n('/admin/?x=1#users'), '/admin/');
    });

    it('keeps a query string on the history URL (words filters)', async () => {
        const b = makeBrowser();
        b.win.TodayView = view([], 'today');
        b.win.LessonsView = view([], 'lessons');
        b.win.location.pathname = '/lessons/';
        b.win.location.search = '?binyan=hifil';
        b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main });
        assert.equal(b.win.location.pathname + b.win.location.search, '/lessons/?binyan=hifil');
        assert.equal(b.win.HebrewRouter.navigate('/?kept=1'), true);
        assert.equal(b.pushes[b.pushes.length - 1], '/?kept=1');
    });
});

describe('HebrewShell setActiveNav + embedded me', () => {
    function shellSandbox(extra) {
        const sandbox = Object.assign({
            console,
            location: { pathname: '/', search: '', href: 'http://127.0.0.1/' },
            document: {
                readyState: 'complete',
                documentElement: { classList: { add() {} } },
                body: { classList: { add() {} } },
                addEventListener() {},
                querySelector() { return null; }
            },
            __HEBREW_ME__: { id: '1', username: 'u', display_name: 'User', role: 'user', must_change_password: false }
        }, extra || {});
        sandbox.window = sandbox;
        vm.createContext(sandbox);
        vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
        vm.runInContext(
            'window.HebrewApi = { me: function () { return Promise.resolve({ ok: true, data: window.__HEBREW_ME__ }); },'
            + ' safeNextPath: function (v) { return v || "/"; } };',
            sandbox
        );
        vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'app-shell.js'), 'utf8'), sandbox);
        return sandbox;
    }

    it('setActiveNav moves aria-current without re-rendering the shell', () => {
        const link = (id, cur) => ({
            cur,
            getAttribute: () => id,
            setAttribute() { this.cur = 'page'; },
            removeAttribute() { this.cur = null; }
        });
        const links = { today: link('today', 'page'), lessons: link('lessons', null) };
        const sandbox = shellSandbox();
        sandbox.document.querySelector = (sel) => (sel === '.hebrew-shell'
            ? { querySelectorAll: () => [links.today, links.lessons] }
            : null);
        sandbox.HebrewShell.setActiveNav('/lessons/');
        assert.equal(links.today.cur, null);
        assert.equal(links.lessons.cur, 'page');
    });

    it('mount with the embedded user paints the nav at once (no loading placeholder)', async () => {
        const sandbox = shellSandbox();
        const writes = [];
        sandbox._el = {
            _html: '',
            set innerHTML(v) { writes.push(String(v)); this._html = v; },
            get innerHTML() { return this._html; },
            querySelector() { return null; },
            addEventListener() {}
        };
        await vm.runInContext('HebrewShell.mount(_el, { me: __HEBREW_ME__ })', sandbox);
        assert.ok(writes.length >= 1);
        assert.ok(writes.every((w) => !/hebrew-shell-loading/.test(w)));
        assert.match(writes[0], /hebrew-shell-nav/);
    });
});

describe('static caching (server/web.js)', () => {
    const harness = require('./helpers/signed-in-server');

    before(async () => { await harness.start(); });
    after(async () => { await harness.stop(); });

    it('scripts get an ETag and answer 304 when unchanged', async () => {
        const cookie = harness.cookieFor('learner');
        const first = await fetch(harness.baseUrl + '/shared/router.js', { headers: { Cookie: cookie } });
        assert.equal(first.status, 200);
        const etag = first.headers.get('etag');
        assert.ok(etag, 'ETag present');
        await first.arrayBuffer();
        const again = await fetch(harness.baseUrl + '/shared/router.js', {
            headers: { Cookie: cookie, 'If-None-Match': etag }
        });
        assert.equal(again.status, 304);
    });

    it('every screen URL returns the same shell, never cached', async () => {
        const cookie = harness.cookieFor('admin');
        for (const p of ['/', '/lessons/', '/guides/', '/review/', '/words/', '/progress/', '/admin/']) {
            const res = await fetch(harness.baseUrl + p, { headers: { Cookie: cookie, Accept: 'text/html' } });
            assert.equal(res.status, 200, p);
            assert.match(res.headers.get('cache-control') || '', /no-store/, p);
            const html = await res.text();
            assert.match(html, /id="app-main"/, p);
            assert.match(html, /"role":"admin"/, p);
        }
    });
});
