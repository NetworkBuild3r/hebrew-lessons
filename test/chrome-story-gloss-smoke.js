#!/usr/bin/env node
/**
 * Headless Chrome smoke: tap story word → gloss popover + not_yet in store.
 * Authored-by-spec: INIT-001/SPEC-007
 * Sign-in first (INIT-002/SPEC-012). Ephemeral port via signed-in-server.
 *
 *   node test/chrome-story-gloss-smoke.js
 */
const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const harness = require('./helpers/signed-in-server');
const CHROME = process.env.CHROME_PATH
    || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

function get(url) {
    return new Promise((resolve, reject) => {
        http.get(url, res => {
            let data = '';
            res.on('data', c => { data += c; });
            res.on('end', () => resolve({ status: res.statusCode, body: data }));
        }).on('error', reject);
    });
}

function cookieParts(cookieHeader) {
    const eq = cookieHeader.indexOf('=');
    return {
        name: cookieHeader.slice(0, eq),
        value: decodeURIComponent(cookieHeader.slice(eq + 1))
    };
}

function cdpSession(wsUrl) {
    let nextId = 1;
    let socket = null;
    const pending = new Map();
    const open = new Promise((resolve, reject) => {
        socket = new WebSocket(wsUrl);
        const timer = setTimeout(() => reject(new Error('CDP open timeout')), 15000);
        socket.addEventListener('open', () => {
            clearTimeout(timer);
            resolve();
        });
        socket.addEventListener('error', () => {
            clearTimeout(timer);
            reject(new Error('WebSocket error talking to Chrome'));
        });
        socket.addEventListener('message', event => {
            const msg = JSON.parse(event.data);
            if (msg.id == null) return;
            const entry = pending.get(msg.id);
            if (!entry) return;
            pending.delete(msg.id);
            if (msg.error) entry.reject(new Error(JSON.stringify(msg.error)));
            else entry.resolve(msg.result);
        });
    });
    return {
        async call(method, params) {
            await open;
            const id = nextId++;
            return new Promise((resolve, reject) => {
                const timer = setTimeout(() => {
                    pending.delete(id);
                    reject(new Error('CDP timeout for ' + method));
                }, 25000);
                pending.set(id, {
                    resolve: (v) => { clearTimeout(timer); resolve(v); },
                    reject: (e) => { clearTimeout(timer); reject(e); }
                });
                socket.send(JSON.stringify({ id, method, params: params || {} }));
            });
        },
        close() {
            try { socket && socket.close(); } catch (err) { /* ignore */ }
        }
    };
}

async function findPage(debugPort) {
    for (let i = 0; i < 40; i += 1) {
        try {
            const list = await get('http://127.0.0.1:' + debugPort + '/json/list');
            const pages = JSON.parse(list.body);
            const page = pages.find(p => p.type === 'page' && p.webSocketDebuggerUrl);
            if (page) return page;
        } catch (err) { /* retry */ }
        await new Promise(r => setTimeout(r, 250));
    }
    throw new Error('No Chrome page');
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'real' });
    const BASE = started.baseUrl;

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-gloss-'));
    const debugPort = 9240 + Math.floor(Math.random() * 200);
    const chrome = spawn(CHROME, [
        '--headless=new',
        '--disable-gpu',
        '--no-first-run',
        '--no-default-browser-check',
        '--user-data-dir=' + userData,
        '--remote-debugging-port=' + debugPort,
        '--remote-allow-origins=*',
        'about:blank'
    ], { stdio: 'ignore' });

    try {
        const page = await findPage(debugPort);
        const session = cdpSession(page.webSocketDebuggerUrl);
        await session.call('Network.enable', {});
        const ck = cookieParts(harness.cookieFor('learner'));
        await session.call('Network.setCookie', {
            name: ck.name,
            value: ck.value,
            url: BASE,
            path: '/'
        });
        await session.call('Page.enable', {});
        await session.call('Runtime.enable', {});

        await session.call('Page.addScriptToEvaluateOnNewDocument', {
            source: `
                localStorage.removeItem('hebrew-review:v1');
                localStorage.removeItem('hebrew-worksheet:my-morning');
            `
        });

        // story is 4th activity (1-based step-4) in my-morning
        await session.call('Page.navigate', {
            url: BASE + '/worksheets/my-morning/index.html#step-4'
        });
        await new Promise(r => setTimeout(r, 2500));

        let value = null;
        let lastErr = null;
        for (let attempt = 0; attempt < 6; attempt += 1) {
            try {
                const result = await session.call('Runtime.evaluate', {
                    awaitPromise: true,
                    returnByValue: true,
                    expression: `(() => new Promise((resolve, reject) => {
                        const deadline = Date.now() + 12000;
                        const wait = () => {
                            if (!window.HebrewReview || !window.WORKSHEET || !window.HebrewUtils) {
                                if (Date.now() > deadline) return reject(new Error('player did not boot'));
                                setTimeout(wait, 100);
                                return;
                            }
                            const story = document.querySelector('[data-step-type="story"]');
                            if (!story) {
                                if (Date.now() > deadline) return reject(new Error('story step not ready'));
                                setTimeout(wait, 100);
                                return;
                            }
                            const coverage = document.getElementById('story-coverage');
                            const coverageUnknown = coverage && coverage.getAttribute('data-coverage') === 'unknown';

                            const token = Array.from(document.querySelectorAll('.story-token')).find(el => {
                                const t = (el.getAttribute('data-token') || el.textContent || '').normalize('NFC');
                                return t === 'קָם' || window.HebrewUtils.stripVowels(t) === 'קם';
                            });
                            if (!token) {
                                return reject(new Error('story token קָם missing; tokens=' +
                                    Array.from(document.querySelectorAll('.story-token')).slice(0, 8)
                                        .map(t => t.textContent).join('|')));
                            }

                            token.click();

                            setTimeout(() => {
                                const pop = document.querySelector('.story-gloss-popover');
                                if (!pop) return reject(new Error('popover missing after tap'));
                                const meaning = (pop.querySelector('.story-gloss-en') || {}).textContent || '';
                                const meaningHtml = (pop.querySelector('.story-gloss-en') || {}).innerHTML || '';
                                const hasAudioBtn = !!pop.querySelector('[data-action="gloss-audio"]');
                                const rec = window.HebrewReview.get('קָם');
                                const storeRaw = localStorage.getItem('hebrew-review:v1');
                                const store = storeRaw ? JSON.parse(storeRaw) : null;
                                const stored = store && store.words && (store.words['קָם'] || store.words[token.getAttribute('data-token')]);

                                // Tap elsewhere dismiss (re-open first if needed after checks)
                                token.click();
                                const pop2 = document.querySelector('.story-gloss-popover');
                                const banner = document.querySelector('.banner.banner-amber') || document.querySelector('h3');
                                if (banner) banner.click();
                                else document.body.click();
                                const popAfterElsewhere = document.querySelector('.story-gloss-popover');

                                // Escape dismiss (re-open)
                                token.click();
                                const pop3 = document.querySelector('.story-gloss-popover');
                                document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
                                const popAfterEsc = document.querySelector('.story-gloss-popover');

                                resolve({
                                    coverageUnknown,
                                    meaning: meaning.trim(),
                                    meaningHtml,
                                    lastResult: rec && rec.he_en && rec.he_en.lastResult,
                                    lapses: rec && rec.he_en && rec.he_en.lapses,
                                    storedResult: stored && stored.he_en && stored.he_en.lastResult,
                                    popoverClosedByElsewhere: !!(pop2 && !popAfterElsewhere),
                                    popoverClosedByEscape: !!(pop3 && !popAfterEsc),
                                    hasAudioBtn
                                });
                            }, 200);
                        };
                        wait();
                    }))()`
                });
                value = result.result && result.result.value;
                if (value) break;
                if (result.result && result.result.subtype === 'error') {
                    lastErr = new Error(result.result.description || 'evaluate error');
                } else {
                    lastErr = new Error('empty evaluate: ' + JSON.stringify(result));
                }
            } catch (err) {
                lastErr = err;
                await new Promise(r => setTimeout(r, 500));
            }
        }
        session.close();
        if (!value) throw lastErr || new Error('No evaluate result');
        if (!value.coverageUnknown) throw new Error('expected coverage unknown with empty store');
        if (!/gets up/i.test(value.meaning || '')) {
            throw new Error('expected vocab meaning in popover, got: ' + value.meaning);
        }
        if (/<img|<b>/i.test(value.meaningHtml || '')) {
            throw new Error('unexpected raw HTML in gloss: ' + value.meaningHtml);
        }
        if (value.lastResult !== 'not_yet' && value.storedResult !== 'not_yet') {
            throw new Error('expected not_yet in store, got ' + JSON.stringify(value));
        }
        if (value.lapses !== 0) throw new Error('not_yet should not bump lapses');
        if (!value.popoverClosedByElsewhere) throw new Error('tap elsewhere should dismiss popover');
        if (!value.popoverClosedByEscape) throw new Error('Escape should dismiss popover');
        if (!value.hasAudioBtn) throw new Error('gloss audio button missing');

        console.log(JSON.stringify({ ok: true, ...value }, null, 2));
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

main().catch(err => {
    console.error(err.message || err);
    process.exit(1);
});
