#!/usr/bin/env node
/**
 * Headless Chrome smoke: round-2 English-first words card records en_he.
 * Sign-in first (INIT-002/SPEC-012). Ephemeral port via signed-in-server.
 *
 *   node test/chrome-production-recall-smoke.js
 *
 * Authored-by-spec: INIT-001/SPEC-005
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

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-prod-'));
    const debugPort = 9230 + Math.floor(Math.random() * 200);
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

        const seed = {
            lesson: 'my-morning',
            round: 2,
            completedRounds: 1,
            roundStartedAt: Date.now() - 86400000,
            wordMarks: {},
            missedLast: [],
            vowelsOn: true,
            stepIndex: 0,
            completedSteps: {},
            activityState: { words: { forceDir: 'en_he' } }
        };

        // Signed in, lesson progress comes from the server (localStorage is ignored), so seed it there.
        const put = await fetch(BASE + '/api/lessons/my-morning/progress', {
            method: 'PUT',
            headers: {
                Cookie: harness.cookieFor('learner'),
                'Content-Type': 'application/json',
                Origin: BASE
            },
            body: JSON.stringify({ state: seed, rev: 0 })
        });
        if (!put.ok) throw new Error('seed progress failed: ' + put.status + ' ' + await put.text());

        await session.call('Page.navigate', {
            url: BASE + '/worksheets/my-morning/index.html#step-1'
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
                        const deadline = Date.now() + 10000;
                        const wait = () => {
                            if (!window.HebrewReview || !window.WORKSHEET) {
                                if (Date.now() > deadline) return reject(new Error('player did not boot'));
                                setTimeout(wait, 100);
                                return;
                            }
                            const wordsRoot = document.querySelector('[data-step-type="words"]');
                            if (!wordsRoot) {
                                if (Date.now() > deadline) return reject(new Error('words step not ready'));
                                setTimeout(wait, 100);
                                return;
                            }
                            const card = document.querySelector('.vocab-card[data-direction="en_he"]');
                            if (!card) return reject(new Error('no en_he card'));
                            const heInHead = card.querySelector('.vocab-head .he-word');
                            const reveal = card.querySelector('.reveal');
                            const revealHidden = !!(reveal && reveal.classList.contains('hidden'));
                            const meaning = card.querySelector('.vocab-head .meaning');
                            if (heInHead) return reject(new Error('Hebrew was visible before reveal'));
                            if (!revealHidden) return reject(new Error('reveal not hidden'));
                            if (!meaning || !(meaning.textContent || '').trim()) {
                                return reject(new Error('English meaning missing'));
                            }
                            const toggle = card.querySelector('[data-action="toggle-reveal"]');
                            toggle.click();
                            const heAfter = card.querySelector('.reveal .he-word');
                            const marks = card.querySelector('.mark-row');
                            if (!heAfter) return reject(new Error('reveal did not show Hebrew'));
                            if (marks && marks.classList.contains('hidden')) {
                                return reject(new Error('marks still hidden after reveal'));
                            }
                            const knew = card.querySelector('[data-action="mark-word"][data-mark="knew"]');
                            knew.click();
                            const word = window.WORKSHEET.vocab.items[0].word;
                            const rec = window.HebrewReview.get(word);
                            resolve({
                                hebrewHiddenBeforeReveal: !heInHead && revealHidden,
                                enHeLastResult: rec && rec.en_he && rec.en_he.lastResult,
                                direction: 'en_he'
                            });
                        };
                        wait();
                    }))()`
                });
                if (result.exceptionDetails) {
                    const ex = result.exceptionDetails.exception;
                    throw new Error((ex && ex.description) || result.exceptionDetails.text);
                }
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
        if (!value.hebrewHiddenBeforeReveal) throw new Error('Hebrew not hidden before reveal');
        if (value.enHeLastResult !== 'pass') {
            throw new Error('expected en_he pass, got ' + JSON.stringify(value));
        }
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
