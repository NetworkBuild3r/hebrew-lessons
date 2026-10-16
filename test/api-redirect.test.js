/**
 * 401 handling and `next` safety for views (INIT-005/SPEC-004 ac-3, REQ-007).
 *
 * shared/api.js runs in a vm with a stub fetch and a stub location. No network.
 * The default for every call is: a 401 sends the learner to /login/?next=<safe path>.
 * A call may opt out with { redirectOn401: false } (sign-in, sign-out, and best-effort
 * enrichment such as the Today mix or per-lesson progress).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const serverSecurity = require('../server/http/security');

function makeApi(pathname, status) {
    const calls = [];
    const location = { pathname: pathname === undefined ? '/review/' : pathname, href: '' };
    const sandbox = {
        console,
        location,
        Promise,
        JSON,
        fetch(url, init) {
            calls.push({ url, init });
            const code = typeof status === 'function' ? status(url) : (status || 200);
            return Promise.resolve({
                status: code,
                ok: code >= 200 && code < 300,
                headers: { get: () => 'application/json' },
                json: () => Promise.resolve({ error: 'x' }),
                text: () => Promise.resolve('')
            });
        }
    };
    sandbox.window = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'api.js'), 'utf8'), sandbox);
    return { Api: sandbox.HebrewApi, location, calls };
}

describe('HebrewApi.safeNextPath (shared/api.js)', () => {
    const { Api } = makeApi();

    it('keeps same-site absolute paths, with query and hash', () => {
        for (const ok of ['/', '/review/', '/words/?binyan=hifil', '/admin/#lessons', '/worksheets/my-morning/index.html']) {
            assert.equal(Api.safeNextPath(ok), ok);
        }
        assert.equal(Api.safeNextPath('  /progress/  '), '/progress/', 'trimmed');
    });

    it('turns every hostile or malformed value into "/"', () => {
        const bad = [
            undefined, null, 42, {}, [], '', '   ',
            'review/', 'evil', '//evil.example', '//evil.example/x', '///evil',
            'https://evil.example/', 'http://evil.example', 'javascript:alert(1)', 'data:text/html,x',
            '/\\evil.example', '\\evil', '/ok\\../evil', '/a\0b', 'JAVASCRIPT:alert(1)', 'mailto:a@b.c'
        ];
        for (const value of bad) {
            assert.equal(Api.safeNextPath(value), '/', JSON.stringify(value));
        }
    });

    it('agrees with the server rule for the cases the server also checks', () => {
        for (const value of ['//evil.example', 'https://evil.example/', '\\evil', 'evil', '/review/', '/']) {
            assert.equal(Api.safeNextPath(value), serverSecurity.safeNextPath(value), JSON.stringify(value));
        }
    });

    it('currentPathForNext reads the live pathname and is safe even if it is hostile', () => {
        const live = makeApi('/progress/');
        assert.equal(live.Api.currentPathForNext(), '/progress/');
        const odd = makeApi('//evil.example/x');
        assert.equal(odd.Api.currentPathForNext(), '/');
        const none = makeApi('');
        assert.equal(none.Api.currentPathForNext(), '/');
    });
});

describe('401 during a view redirects to sign-in (REQ-007)', () => {
    it('default: a 401 sends the browser to /login/?next=<current screen>', async () => {
        const { Api, location } = makeApi('/review/', 401);
        const res = await Api.get('/api/review/state');
        assert.equal(res.status, 401);
        assert.equal(res.ok, false);
        assert.equal(location.href, '/login/?next=%2Freview%2F');
    });

    it('works for every HTTP verb a view uses', async () => {
        for (const run of [
            (Api) => Api.post('/api/review/signals', { a: 1 }),
            (Api) => Api.del('/api/me/library/1'),
            (Api) => Api.request('/api/me', { method: 'PUT', body: {} })
        ]) {
            const { Api, location } = makeApi('/words/', 401);
            await run(Api);
            assert.equal(location.href, '/login/?next=%2Fwords%2F');
        }
    });

    it('each screen path produces a next that the server accepts unchanged', async () => {
        const routes = require('../shared/app-routes.js');
        for (const r of routes) {
            const { Api, location } = makeApi(r.path, 401);
            await Api.me();
            const next = decodeURIComponent(location.href.split('?next=')[1]);
            assert.equal(next, r.path);
            assert.equal(serverSecurity.safeNextPath(next), r.path);
        }
    });

    it('a hostile current pathname never becomes the next target', async () => {
        const { Api, location } = makeApi('//evil.example/x', 401);
        await Api.get('/api/me');
        assert.equal(location.href, '/login/?next=%2F');
    });

    it('opt-out: redirectOn401:false leaves the page alone (enrichment calls, sign-in, sign-out)', async () => {
        const a = makeApi('/', 401);
        const res = await a.Api.get('/api/me/mix', { redirectOn401: false });
        assert.equal(res.status, 401);
        assert.equal(a.location.href, '');

        const b = makeApi('/login/', 401);
        await b.Api.signIn('u', 'wrong', '/review/');
        assert.equal(b.location.href, '', 'a wrong password must not loop back to /login/');
        const c = makeApi('/', 401);
        await c.Api.signOut();
        assert.equal(c.location.href, '');
    });

    it('other statuses do not redirect', async () => {
        for (const code of [200, 204, 400, 403, 404, 409, 500]) {
            const { Api, location } = makeApi('/review/', code);
            await Api.get('/api/me');
            assert.equal(location.href, '', String(code));
        }
    });

    it('still refuses to call anything outside /api/', async () => {
        const { Api, calls } = makeApi('/');
        await assert.rejects(() => Api.request('https://evil.example/api/'), /only calls \/api\//);
        await assert.rejects(() => Api.request('/worksheets/x'), /only calls \/api\//);
        assert.equal(calls.length, 0);
    });

    it('a view aborted by the router (signal) passes the signal through', async () => {
        const { Api, calls } = makeApi('/', 200);
        const controller = new AbortController();
        await Api.get('/api/me', { signal: controller.signal });
        assert.equal(calls[0].init.signal, controller.signal);
    });
});

describe('views use HebrewApi, not raw fetch (so 401 handling cannot be bypassed)', () => {
    const viewFiles = [
        'today-page.js', 'lessons-page.js', 'guides-page.js', 'review-page.js',
        'words-page.js', 'progress-page.js'
    ];
    const adminDir = path.join(root, 'shared', 'admin');
    const files = viewFiles.map((f) => path.join(root, 'shared', f))
        .concat(fs.readdirSync(adminDir).filter((f) => f.endsWith('.js')).map((f) => path.join(adminDir, f)));

    it('no screen module calls fetch() or XMLHttpRequest directly', () => {
        for (const file of files) {
            const code = fs.readFileSync(file, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
            assert.doesNotMatch(code, /(^|[^.\w])fetch\s*\(/, path.relative(root, file));
            assert.doesNotMatch(code, /XMLHttpRequest/, path.relative(root, file));
        }
    });

    it('the shell and sign-in page pass `next` through safeNextPath', () => {
        const shell = fs.readFileSync(path.join(root, 'shared', 'app-shell.js'), 'utf8');
        const login = fs.readFileSync(path.join(root, 'shared', 'login.js'), 'utf8');
        assert.match(shell, /safeNextPath/);
        assert.match(login, /safeNextPath/);
    });
});
