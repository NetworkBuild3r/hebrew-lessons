/**
 * Chrome smoke: top-nav clicks change screens without reloading the document
 * (INIT-005/SPEC-002..004, ADR-0007).
 * INIT-007/SPEC-007: keeps review/lessons routes reachable for player a11y smokes;
 * full lesson + review session coverage lives in chrome-learner-support-smoke
 * and chrome-review-smoke.
 *
 * Run: node test/chrome-navigation-smoke.js
 * Needs Chrome at the default Windows path (or CHROME_PATH).
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');
const harness = require('./helpers/signed-in-server');

const { CHROME } = require('./helpers/chrome-cdp');
const SCREENS = [
    { id: 'lessons', path: '/lessons/' },
    { id: 'review', path: '/review/' },
    { id: 'words', path: '/words/' },
    { id: 'guides', path: '/guides/' },
    { id: 'progress', path: '/progress/' },
    { id: 'admin', path: '/admin/' },
    { id: 'today', path: '/' }
];

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function connect(port) {
    let page = null;
    for (let i = 0; i < 60 && !page; i += 1) {
        try {
            const list = await (await fetch('http://127.0.0.1:' + port + '/json/list')).json();
            page = list.find((p) => p.type === 'page' && p.webSocketDebuggerUrl);
        } catch (err) { /* Chrome still starting */ }
        if (!page) await sleep(250);
    }
    if (!page) throw new Error('No Chrome page on port ' + port);
    const ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => {
        ws.addEventListener('open', resolve);
        ws.addEventListener('error', () => reject(new Error('CDP socket error')));
    });
    let seq = 0;
    const pending = new Map();
    const errors = [];
    ws.addEventListener('message', (ev) => {
        const msg = JSON.parse(ev.data);
        if (msg.id && pending.has(msg.id)) {
            pending.get(msg.id)(msg);
            pending.delete(msg.id);
        }
        if (msg.method === 'Runtime.exceptionThrown') {
            const d = msg.params.exceptionDetails;
            errors.push((d.exception && d.exception.description) || d.text);
        }
    });
    const call = (method, params) => new Promise((resolve) => {
        seq += 1;
        pending.set(seq, resolve);
        ws.send(JSON.stringify({ id: seq, method, params: params || {} }));
    });
    const evaluate = async (expression) => {
        const res = await call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
        if (res.result && res.result.exceptionDetails) {
            throw new Error(res.result.exceptionDetails.text + ' in: ' + expression.slice(0, 80));
        }
        return res.result && res.result.result ? res.result.result.value : undefined;
    };
    return { ws, call, evaluate, errors };
}

async function waitFor(cdp, expression, label, timeoutMs) {
    const deadline = Date.now() + (timeoutMs || 10000);
    let last;
    while (Date.now() < deadline) {
        last = await cdp.evaluate(expression);
        if (last) return last;
        await sleep(100);
    }
    throw new Error('timed out waiting for ' + label);
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'real' });
    const BASE = started.baseUrl;
    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-nav-'));
    const port = 9100 + Math.floor(Math.random() * 400);
    const chrome = spawn(CHROME, [
        '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
        '--user-data-dir=' + userData, '--remote-debugging-port=' + port,
        '--remote-allow-origins=*', '--window-size=1280,900', 'about:blank'
    ], { stdio: 'ignore' });

    const report = { ok: false, screens: [] };
    try {
        const cdp = await connect(port);
        await cdp.call('Network.enable');
        await cdp.call('Runtime.enable');
        await cdp.call('Page.enable');
        const cookie = harness.cookieFor('admin');
        const eq = cookie.indexOf('=');
        await cdp.call('Network.setCookie', {
            name: cookie.slice(0, eq),
            value: decodeURIComponent(cookie.slice(eq + 1)),
            url: BASE,
            path: '/'
        });
        await cdp.call('Page.navigate', { url: BASE + '/' });
        await waitFor(cdp, `document.getElementById('app-main')
            && document.getElementById('app-main').getAttribute('data-route-ready') === 'today'`, 'Today');

        // Survives only if the document is never reloaded.
        await cdp.evaluate('window.__navMarker = "same-document"; true');

        for (const screen of SCREENS) {
            const clicked = await cdp.evaluate(`(() => {
                const a = document.querySelector('.hebrew-shell [data-nav="${screen.id}"]');
                if (!a) return false;
                a.click();
                return true;
            })()`);
            if (!clicked) throw new Error('no nav link for ' + screen.id);
            await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === '${screen.id}'`,
                screen.id);
            const state = await cdp.evaluate(`({
                marker: window.__navMarker || null,
                path: location.pathname,
                current: (document.querySelector('.hebrew-shell [aria-current="page"]')
                    || { getAttribute: () => null }).getAttribute('data-nav'),
                focused: document.activeElement && document.activeElement.tagName,
                headings: document.querySelectorAll('#app-main [data-page-heading]').length,
                title: document.title
            })`);
            report.screens.push(Object.assign({ id: screen.id }, state));
            if (state.marker !== 'same-document') throw new Error(screen.id + ': document reloaded');
            if (state.path !== screen.path) throw new Error(screen.id + ': path ' + state.path);
            if (state.current !== screen.id) {
                throw new Error(screen.id + ': aria-current not moved');
            }
            if (state.headings !== 1) throw new Error(screen.id + ': expected one page heading');
            if (state.focused !== 'H1') throw new Error(screen.id + ': focus not on heading');
        }

        // Back from Today returns to Admin in the same document.
        await cdp.evaluate('history.back(); true');
        await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === 'admin'`, 'back');
        const back = await cdp.evaluate('({ marker: window.__navMarker || null, path: location.pathname })');
        if (back.marker !== 'same-document' || back.path !== '/admin/') {
            throw new Error('Back did not restore Admin in place: ' + JSON.stringify(back));
        }

        // Route transition: a short animation normally, none under prefers-reduced-motion (SPEC-003 ac-3).
        const animationOf = `(() => { const m = document.getElementById('app-main');
            return { name: getComputedStyle(m).animationName, cls: m.className }; })()`;
        const clickNav = async (id) => {
            await cdp.evaluate(`document.querySelector('.hebrew-shell [data-nav="${id}"]').click(); true`);
            await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === '${id}'`, id);
        };
        await cdp.evaluate(`document.querySelector('.hebrew-shell [data-nav="lessons"]').click(); true`);
        const normal = await cdp.evaluate(animationOf);
        if (normal.name !== 'hebrew-route-enter') throw new Error('no route transition: ' + JSON.stringify(normal));
        await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === 'lessons'`, 'lessons');
        await cdp.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
        await cdp.evaluate(`document.querySelector('.hebrew-shell [data-nav="today"]').click(); true`);
        const reduced = await cdp.evaluate(animationOf);
        if (reduced.name !== 'none' || /route-enter/.test(reduced.cls)) {
            throw new Error('route transition not disabled under reduced motion: ' + JSON.stringify(reduced));
        }
        await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === 'today'`, 'today');
        await cdp.call('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'no-preference' }] });
        report.transition = { normal: normal.name, reduced: reduced.name };

        // Unknown path: the shell with a "not found" view, not a blank page (SPEC-002 ac-2).
        await cdp.call('Page.navigate', { url: BASE + '/nope/' });
        await waitFor(cdp, `document.getElementById('app-main')
            && document.getElementById('app-main').getAttribute('data-route-ready') === 'not-found'`, 'not found');
        const lost = await cdp.evaluate(`({
            nav: !!document.querySelector('.hebrew-shell [data-nav="today"]'),
            current: document.querySelectorAll('.hebrew-shell [aria-current="page"]').length,
            headings: document.querySelectorAll('#app-main h1').length,
            focused: document.activeElement && document.activeElement.tagName,
            text: document.getElementById('app-main').textContent,
            path: location.pathname,
            title: document.title
        })`);
        report.notFound = lost;
        if (!lost.nav || lost.current !== 0 || lost.headings !== 1 || lost.focused !== 'H1'
            || !/Page not found/.test(lost.text) || lost.path !== '/nope/') {
            throw new Error('not-found view wrong: ' + JSON.stringify(lost));
        }
        await cdp.evaluate('window.__navMarker = "same-document"; true');
        await cdp.evaluate(`document.querySelector('#app-main a[href="/"]').click(); true`);
        await waitFor(cdp, `document.getElementById('app-main').getAttribute('data-route-ready') === 'today'`, 'today again');
        if (await cdp.evaluate('window.__navMarker') !== 'same-document') throw new Error('not-found link reloaded the page');

        // Lesson player: one visible sound control, off by default, toggles (SPEC-009 ac-2).
        await cdp.call('Page.navigate', { url: BASE + '/worksheets/my-morning/' });
        await waitFor(cdp, `document.getElementById('btn-sfx')`, 'sound control');
        await cdp.evaluate(`localStorage.removeItem('hebrew-sfx-enabled'); HebrewFeedback.paintToggles(); true`);
        const sfx = await cdp.evaluate(`(() => {
            const b = document.getElementById('btn-sfx');
            const r = b.getBoundingClientRect();
            const before = b.getAttribute('aria-pressed');
            b.click();
            const after = b.getAttribute('aria-pressed');
            const stored = localStorage.getItem('hebrew-sfx-enabled');
            b.click();
            return { before, after, stored, again: b.getAttribute('aria-pressed'),
                w: Math.round(r.width), h: Math.round(r.height), text: b.textContent.trim() };
        })()`);
        report.soundToggle = sfx;
        if (sfx.before !== 'false' || sfx.after !== 'true' || sfx.stored !== '1' || sfx.again !== 'false') {
            throw new Error('sound toggle wrong: ' + JSON.stringify(sfx));
        }
        if (sfx.h < 44 || sfx.w < 44) throw new Error('sound toggle under 44px: ' + JSON.stringify(sfx));

        if (cdp.errors.length) throw new Error('page errors: ' + cdp.errors.join(' | '));
        report.ok = true;
        report.back = back;
        console.log(JSON.stringify(report, null, 2));
        console.log('chrome-navigation-smoke: PASS');
        cdp.ws.close();
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-navigation-smoke") || main().catch((err) => {
    console.error(err.message || err);
    process.exit(1);
});
