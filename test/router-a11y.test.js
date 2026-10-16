/**
 * Route accessibility, history and not-found behaviour (INIT-005/SPEC-002..004, ADR-0007).
 *
 * Covers REQ-004 (title, focus, announce, scroll), REQ-005 (reduced motion), REQ-006
 * (unknown route inside the shell), REQ-019 (direct URL, back/forward, plain links).
 * The router runs in a vm with a small DOM stand-in. No network except the local
 * test server in the last block.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const ROUTES = require('../shared/app-routes.js');
const spa = require('../server/spa');

const tick = (ms) => new Promise((r) => setTimeout(r, ms || 0));

/**
 * @param {{ reduced?: boolean, path?: string }} [opts]
 */
function makeBrowser(opts) {
    const o = opts || {};
    const listeners = { doc: {}, win: {} };
    const roots = {};
    const bodyClasses = new Set();
    const pushes = [];
    const replaces = [];
    const scrolls = [];
    const classOps = [];
    const announce = { textContent: '' };
    const actions = { innerHTML: '' };
    const navLink = (id, current) => ({
        id,
        current,
        removeAttribute(name) { if (name === 'aria-current') this.current = false; }
    });
    const navLinks = [navLink('today', true), navLink('lessons', false)];
    const main = {
        _html: '',
        set innerHTML(v) { this._html = String(v); for (const k of Object.keys(roots)) delete roots[k]; },
        get innerHTML() { return this._html; },
        classList: {
            add(c) { classOps.push('add:' + c); },
            remove(c) { classOps.push('remove:' + c); }
        },
        offsetWidth: 0,
        attrs: {},
        focused: null,
        setAttribute(k, v) { this.attrs[k] = String(v); },
        removeAttribute(k) { delete this.attrs[k]; },
        focus() { main.focused = 'main'; },
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
        matchMedia: (q) => ({ matches: Boolean(o.reduced) && /prefers-reduced-motion/.test(q) }),
        requestIdleCallback() {},
        scrollTo(x, y) { scrolls.push([x, y]); },
        location: {
            pathname: o.path || '/', hash: '', search: '', origin: 'http://app.test',
            get href() { return 'http://app.test' + this.pathname + (this.search || '') + this.hash; },
            reload() { sandbox.reloaded = true; }
        },
        history: {
            state: null,
            pushState(state, _t, url) { pushes.push(url); setUrl(url); this.state = state; },
            replaceState(state, _t, url) { replaces.push(state); setUrl(url); this.state = state; }
        },
        addEventListener(type, fn) { listeners.win[type] = fn; },
        document: {
            title: '',
            body: { classList: { add: (c) => bodyClasses.add(c), remove: (c) => bodyClasses.delete(c) } },
            addEventListener(type, fn) { listeners.doc[type] = fn; },
            querySelectorAll(sel) {
                return /data-nav/.test(sel) ? navLinks.filter((l) => l.current) : [];
            },
            getElementById(id) {
                if (id === 'app-route-announce') return announce;
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
    return { win: sandbox, main, listeners, bodyClasses, pushes, replaces, scrolls, classOps, announce, actions, navLinks };
}

const TEST_ROUTES = [
    { id: 'today', path: '/', title: 'Today title', announce: 'היום', bodyClass: 'today-page',
        rootId: 'today-app', heading: { he: 'היום', en: 'Today' }, scripts: [], view: 'TodayView' },
    { id: 'lessons', path: '/lessons/', title: 'Lessons title', announce: 'שיעורים', bodyClass: 'lessons-page',
        rootId: 'lessons-app', heading: { he: 'שיעורים', en: 'Lessons' }, scripts: [], view: 'LessonsView' }
];

function view(log, name) {
    return {
        mount() { log.push(name + ':mount'); },
        unmount() { log.push(name + ':unmount'); }
    };
}

function started(opts) {
    const b = makeBrowser(opts);
    const log = [];
    b.log = log;
    b.win.TodayView = view(log, 'today');
    b.win.LessonsView = view(log, 'lessons');
    b.win.HebrewRouter.start({ routes: TEST_ROUTES, main: b.main, user: { role: 'user' } });
    return b;
}

function click(b, href, extra) {
    const a = {
        href: new URL(href, 'http://app.test').href,
        target: '',
        getAttribute: (n) => (n === 'href' ? href : null),
        hasAttribute: () => false
    };
    Object.assign(a, (extra && extra.anchor) || {});
    const ev = Object.assign({
        button: 0,
        defaultPrevented: false,
        target: { closest: () => a },
        preventDefault() { this.defaultPrevented = true; }
    }, (extra && extra.event) || {});
    b.listeners.doc.click(ev);
    return ev.defaultPrevented;
}

describe('route change accessibility (REQ-004)', () => {
    it('title updates on every in-app navigation', async () => {
        const b = started();
        assert.equal(b.win.document.title, 'Today title');
        b.win.HebrewRouter.navigate('/lessons/');
        assert.equal(b.win.document.title, 'Lessons title');
        await tick();
        b.win.HebrewRouter.navigate('/');
        assert.equal(b.win.document.title, 'Today title');
    });

    it('focus moves to the main heading before data loads', () => {
        const b = started();
        assert.equal(b.main.focused, 'h1', 'first paint');
        b.main.focused = null;
        b.win.HebrewRouter.navigate('/lessons/');
        assert.equal(b.main.focused, 'h1', 'after navigation');
    });

    it('a polite live region announces the new screen (Hebrew name)', async () => {
        const html = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8');
        assert.match(html, /id="app-route-announce"[^>]*aria-live="polite"/);

        const b = started();
        await tick(60);
        assert.equal(b.announce.textContent, 'Today');
        b.win.HebrewRouter.navigate('/lessons/');
        assert.equal(b.announce.textContent, '', 'cleared first so the same text re-announces');
        await tick(60);
        assert.equal(b.announce.textContent, 'Lessons');
    });

    it('scroll resets to the top on a new route', () => {
        const b = started();
        b.win.scrollY = 500;
        b.scrolls.length = 0;
        b.win.HebrewRouter.navigate('/lessons/');
        assert.deepEqual(b.scrolls[0], [0, 0]);
        assert.equal(b.replaces.some((s) => s && s.scrollY === 500), true, 'leaving saves the old position in history.state');
    });

    it('Back restores the saved scroll position and remounts the old view', async () => {
        const b = started();
        await tick();
        b.win.scrollY = 420;
        b.win.HebrewRouter.navigate('/lessons/');
        await tick();
        // Browser goes Back: URL already changed, state carries what we saved.
        b.win.location.pathname = '/';
        b.scrolls.length = 0;
        b.listeners.win.popstate({ state: { scrollY: 420 } });
        assert.deepEqual(b.scrolls[0], [0, 420], 'restored at once');
        await tick();
        assert.deepEqual(b.scrolls[b.scrolls.length - 1], [0, 420], 'restored again after the view mounts');
        assert.equal(b.win.document.title, 'Today title');
        assert.deepEqual([...b.log], ['today:mount', 'today:unmount', 'lessons:mount', 'lessons:unmount', 'today:mount']);
    });

    it('Forward works the same way and never reloads the document', async () => {
        const b = started();
        await tick();
        b.win.HebrewRouter.navigate('/lessons/');
        await tick();
        b.win.location.pathname = '/';
        b.listeners.win.popstate({ state: { scrollY: 0 } });
        await tick();
        b.win.location.pathname = '/lessons/';
        b.listeners.win.popstate({ state: { scrollY: 90 } });
        await tick();
        assert.equal(b.win.document.title, 'Lessons title');
        assert.equal(b.main.attrs['data-route-ready'], 'lessons');
        assert.equal(b.win.reloaded, undefined);
    });

    it('Back to a path that is not an app screen asks the browser for a normal load', () => {
        const b = started();
        b.win.location.pathname = '/worksheets/my-morning/';
        b.listeners.win.popstate({ state: null });
        assert.equal(b.win.reloaded, true);
    });
});

describe('direct URL and unknown route (REQ-002, REQ-006)', () => {
    it('a direct hit on /lessons/ opens Lessons, not Today', async () => {
        const b = started({ path: '/lessons/' });
        await tick();
        assert.equal(b.win.document.title, 'Lessons title');
        assert.deepEqual([...b.log], ['lessons:mount']);
        assert.equal(b.win.location.pathname, '/lessons/');
        assert.equal(b.pushes.length, 0, 'start replaces the URL; it does not add a history entry');
    });

    it('/index.html and a missing trailing slash resolve to the canonical screen', async () => {
        assert.equal(started({ path: '/lessons' }).win.location.pathname, '/lessons/');
        assert.equal(started({ path: '/index.html' }).win.location.pathname, '/');
    });

    it('an unknown path shows "not found" inside the shell and mounts no view', async () => {
        const b = started({ path: '/nope/' });
        await tick(60);
        assert.deepEqual([...b.log], []);
        assert.match(b.main.innerHTML, /<h1[^>]*>[\s\S]*העמוד לא נמצא/);
        assert.match(b.main.innerHTML, /Page not found/);
        assert.match(b.main.innerHTML, /href="\/"/, 'a way back to Today');
        assert.equal(b.main.focused, 'h1');
        assert.match(b.win.document.title, /Page not found/);
        assert.equal(b.announce.textContent, 'Page not found');
        assert.equal(b.main.attrs['data-route-ready'], 'not-found');
        assert.equal(b.win.location.pathname, '/nope/', 'URL is left as typed');
        assert.equal(b.win.reloaded, undefined);
        assert.equal(b.navLinks.some((l) => l.current), false, 'no nav link is marked current');
    });

    it('the not-found screen leaves cleanly: its Today link opens Today in place', async () => {
        const b = started({ path: '/nope/' });
        assert.equal(click(b, '/'), true, 'Back to Today is an in-app link');
        await tick();
        assert.deepEqual([...b.log], ['today:mount']);
        assert.equal(b.main.attrs['data-route-ready'], 'today');
        assert.equal(b.win.location.pathname, '/');
    });

    it('navigate() still returns false for unknown paths (links to files load normally)', () => {
        const b = started();
        assert.equal(b.win.HebrewRouter.navigate('/nope/'), false);
        assert.equal(b.win.document.title, 'Today title', 'unchanged');
    });
});

describe('links the router must not intercept (REQ-019)', () => {
    it('plain left-click on an app link is handled in place', async () => {
        const b = started();
        assert.equal(click(b, '/lessons/'), true);
        await tick();
        assert.ok(b.log.includes('lessons:mount'));
    });

    it('middle-click, modifier-click, target=_blank, download and right-click are left to the browser', () => {
        const b = started();
        assert.equal(click(b, '/lessons/', { event: { button: 1 } }), false, 'middle click → new tab');
        assert.equal(click(b, '/lessons/', { event: { button: 2 } }), false, 'right click');
        for (const key of ['ctrlKey', 'metaKey', 'shiftKey', 'altKey']) {
            assert.equal(click(b, '/lessons/', { event: { [key]: true } }), false, key);
        }
        assert.equal(click(b, '/lessons/', { anchor: { target: '_blank' } }), false, 'target=_blank');
        assert.equal(click(b, '/lessons/', { anchor: { hasAttribute: (n) => n === 'download' } }), false, 'download');
        click(b, '/lessons/', { event: { defaultPrevented: true } });   // already handled elsewhere
        assert.equal(b.pushes.length, 0, 'none of these touched history');
        assert.equal(b.win.location.pathname, '/');
    });

    it('lesson pages, other origins, in-page anchors and hash-only changes load normally', () => {
        const b = started();
        assert.equal(click(b, '/worksheets/my-morning/'), false);
        assert.equal(click(b, 'https://example.com/lessons/'), false);
        assert.equal(click(b, '#top'), false);
        assert.equal(click(b, '/#tab'), false, 'same route + hash lets hashchange fire');
    });
});

describe('route transition honours prefers-reduced-motion (REQ-005)', () => {
    it('the router plays the transition class for normal motion', () => {
        const b = started({ reduced: false });
        b.classOps.length = 0;
        b.win.HebrewRouter.navigate('/lessons/');
        assert.deepEqual(b.classOps, ['remove:route-enter', 'add:route-enter']);
    });

    it('the router never adds the transition class under reduce', () => {
        const b = started({ reduced: true });
        b.classOps.length = 0;
        b.win.HebrewRouter.navigate('/lessons/');
        assert.deepEqual(b.classOps, ['remove:route-enter'], 'a leftover class from before is cleared, nothing is added');
    });

    it('CSS: short fade/slide, <= 200 ms, switched off under reduce, nothing infinite', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        const marker = '/* === INIT-005/SPEC-009 route transition + sound toggle === */';
        const at = css.indexOf(marker);
        assert.ok(at > 0, 'delimited block exists');
        assert.ok(css.indexOf('/* === end INIT-005/SPEC-009 route transition + sound toggle === */') > at);
        const block = css.slice(at);

        const rule = block.match(/#app-main\.route-enter\s*\{[^}]*animation:\s*hebrew-route-enter\s+(\d+)ms/);
        assert.ok(rule, 'transition rule');
        assert.ok(Number(rule[1]) <= 200, 'duration ' + rule[1] + 'ms must be <= 200');
        assert.match(block, /@keyframes hebrew-route-enter\s*\{[^}]*opacity[^}]*\}/);
        assert.doesNotMatch(block.slice(0, block.indexOf('@media (prefers-reduced-motion')), /infinite/);

        const reduce = block.match(/@media \(prefers-reduced-motion: reduce\)\s*\{\s*#app-main\.route-enter\s*\{([^}]*)\}/);
        assert.ok(reduce, 'reduced-motion override');
        assert.match(reduce[1], /animation:\s*none/);
        assert.match(reduce[1], /transform:\s*none/);
    });

    it('the other agent blocks stay in place before ours (light by default is not altered)', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        assert.ok(css.indexOf('Light by default') > 0);
        assert.ok(css.indexOf('INIT-005/SPEC-009 route transition') > css.indexOf('Light by default'));
    });
});

describe('unknown app URL on the server (REQ-006)', () => {
    const harness = require('./helpers/signed-in-server');

    before(async () => { await harness.start(); });
    after(async () => { await harness.stop(); });

    const get = (p, headers) => fetch(harness.baseUrl + p, { redirect: 'manual', headers: headers || {} });

    it('isUnknownAppPath: screens, files and trees are not "unknown"', () => {
        for (const p of ['/nope/', '/nope', '/review/typo/', '/admin/nope/']) {
            assert.equal(spa.isUnknownAppPath(p), true, p);
        }
        for (const p of ['/', '/lessons/', '/shared/missing.js', '/shared/nope/', '/worksheets/x/', '/worksheets/x/a.mp3',
            '/login/', '/login', '/api/me', '/manifest.webmanifest', '/icon.svg', '/favicon.ico', '/x.html']) {
            assert.equal(spa.isUnknownAppPath(p), false, p);
        }
    });

    it('signed in: /nope/ is the shell with HTTP 404 and never cached', async () => {
        const res = await get('/nope/', { Cookie: harness.cookieFor('learner'), Accept: 'text/html' });
        assert.equal(res.status, 404);
        assert.match(res.headers.get('content-type') || '', /text\/html/);
        assert.match(res.headers.get('cache-control') || '', /no-store/);
        const html = await res.text();
        assert.match(html, /id="app-main"/);
        assert.match(html, /shared\/router\.js/);
        assert.match(html, /"role":"user"/);
    });

    it('signed in: HEAD and a typed URL without a trailing slash behave the same', async () => {
        const cookie = harness.cookieFor('learner');
        const head = await fetch(harness.baseUrl + '/nope/', { method: 'HEAD', headers: { Cookie: cookie, Accept: 'text/html' } });
        assert.equal(head.status, 404);
        const bare = await get('/nope', { Cookie: cookie, Accept: 'text/html' });
        assert.equal(bare.status, 404);
        assert.match(await bare.text(), /id="app-main"/);
    });

    it('signed out: an unknown app path still goes to sign-in with a safe next', async () => {
        const res = await get('/nope/', { Accept: 'text/html' });
        assert.equal(res.status, 302);
        assert.equal(res.headers.get('location'), '/login/?next=%2Fnope%2F');
        const real = await get('/review/', { Accept: 'text/html' });
        assert.equal(real.status, 302);
        assert.equal(real.headers.get('location'), '/login/?next=%2Freview%2F');
    });

    it('real file and API misses keep the plain 404 (no shell)', async () => {
        const cookie = harness.cookieFor('learner');
        for (const p of ['/shared/missing.js', '/shared/nope/', '/worksheets/not-a-lesson/', '/worksheets/public-lesson/missing.mp3',
            '/favicon.ico', '/nope.html']) {
            const res = await get(p, { Cookie: cookie, Accept: 'text/html' });
            assert.equal(res.status, 404, p);
            assert.match(res.headers.get('content-type') || '', /text\/plain/, p);
            assert.equal(await res.text(), 'Not found', p);
        }
    });

    it('/worksheets/ lesson pages and admin gating are unchanged', async () => {
        const lesson = await get('/worksheets/public-lesson/', { Cookie: harness.cookieFor('learner'), Accept: 'text/html' });
        assert.equal(lesson.status, 200);
        assert.doesNotMatch(await lesson.text(), /id="app-main"/);
        const admin = await get('/admin/', { Cookie: harness.cookieFor('learner'), Accept: 'text/html' });
        assert.equal(admin.status, 403);
        const adminOk = await get('/admin/', { Cookie: harness.cookieFor('admin'), Accept: 'text/html' });
        assert.equal(adminOk.status, 200);
    });

    it('every client route path is served as a 200 screen, not as not-found', async () => {
        const cookie = harness.cookieFor('admin');
        for (const r of ROUTES) {
            const res = await get(r.path, { Cookie: cookie, Accept: 'text/html' });
            assert.equal(res.status, 200, r.path);
            await res.arrayBuffer();
        }
    });
});
