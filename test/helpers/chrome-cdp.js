/**
 * Headless Chrome over CDP for the restyle gate (INIT-005/SPEC-013).
 *
 * The same launch pattern as the older chrome-*-smoke.js files (spawn Chrome with a
 * throwaway profile and --remote-debugging-port, talk to the page over a WebSocket), put in
 * one place for the two SPEC-013 scripts. Not a test file: the name has no .test.js suffix.
 * Needs Chrome at the default Windows path, or CHROME_PATH.
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const CHROME = process.env.CHROME_PATH || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
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
    const requests = [];
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
        if (msg.method === 'Network.requestWillBeSent') requests.push(msg.params.request.url);
    });
    const call = (method, params) => new Promise((resolve) => {
        seq += 1;
        pending.set(seq, resolve);
        ws.send(JSON.stringify({ id: seq, method, params: params || {} }));
    });
    const evaluate = async (expression) => {
        const res = await call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
        if (res.result && res.result.exceptionDetails) {
            const d = res.result.exceptionDetails;
            throw new Error(((d.exception && d.exception.description) || d.text) + ' in: ' + expression.slice(0, 80));
        }
        return res.result && res.result.result ? res.result.result.value : undefined;
    };
    return { ws, call, evaluate, errors, requests };
}

async function waitFor(cdp, expression, label, timeoutMs) {
    const deadline = Date.now() + (timeoutMs || 10000);
    while (Date.now() < deadline) {
        if (await cdp.evaluate(expression)) return true;
        await sleep(100);
    }
    throw new Error('timed out waiting for ' + label);
}

/**
 * Start Chrome and return { cdp, close }. `close()` kills Chrome and removes the profile.
 * @param {{ width?: number, height?: number, portBase?: number, tag?: string }} [opts]
 */
async function launch(opts) {
    const o = opts || {};
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-' + (o.tag || 'cdp') + '-'));
    const port = (o.portBase || 9600) + Math.floor(Math.random() * 300);
    const chrome = spawn(CHROME, [
        '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
        '--user-data-dir=' + userData, '--remote-debugging-port=' + port,
        '--remote-allow-origins=*', '--window-size=' + (o.width || 1280) + ',' + (o.height || 900), 'about:blank'
    ], { stdio: 'ignore' });
    const cdp = await connect(port);
    await cdp.call('Network.enable');
    await cdp.call('Runtime.enable');
    await cdp.call('Page.enable');
    return {
        cdp,
        async close() {
            try { cdp.ws.close(); } catch (err) { /* ignore */ }
            try { chrome.kill(); } catch (err) { /* ignore */ }
            await sleep(300);
            try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        }
    };
}

/** Sign the browser in as one of the harness users (or out with null). */
async function signInAs(cdp, harness, baseUrl, who) {
    await cdp.call('Network.clearBrowserCookies');
    if (!who) return;
    const cookie = harness.cookieFor(who);
    const eq = cookie.indexOf('=');
    await cdp.call('Network.setCookie', {
        name: cookie.slice(0, eq),
        value: decodeURIComponent(cookie.slice(eq + 1)),
        url: baseUrl,
        path: '/'
    });
}

async function setViewport(cdp, width, height) {
    await cdp.call('Emulation.setDeviceMetricsOverride', {
        width, height, deviceScaleFactor: 1, mobile: width <= 480
    });
}

/** One Tab key press (down + up) sent to the page. */
async function pressTab(cdp, shift) {
    const base = { key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9, modifiers: shift ? 8 : 0 };
    await cdp.call('Input.dispatchKeyEvent', Object.assign({ type: 'rawKeyDown' }, base));
    await cdp.call('Input.dispatchKeyEvent', Object.assign({ type: 'keyUp' }, base));
}

module.exports = { CHROME, sleep, connect, waitFor, launch, signInAs, setViewport, pressTab };
