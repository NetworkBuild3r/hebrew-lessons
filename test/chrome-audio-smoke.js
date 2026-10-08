#!/usr/bin/env node
/**
 * Headless Chrome smoke: listen-pick / story use clips, not TTS.
 * Sign-in first (INIT-002/SPEC-012). Ephemeral port via signed-in-server.
 *
 *   node test/chrome-audio-smoke.js
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

function cdp(wsUrl, method, params) {
    return new Promise((resolve, reject) => {
        const ws = new WebSocket(wsUrl);
        const id = 1;
        const timer = setTimeout(() => {
            try { ws.close(); } catch (err) { /* ignore */ }
            reject(new Error('CDP timeout for ' + method));
        }, 20000);
        ws.addEventListener('open', () => {
            ws.send(JSON.stringify({ id, method, params: params || {} }));
        });
        ws.addEventListener('message', event => {
            const msg = JSON.parse(event.data);
            if (msg.id !== id) return;
            clearTimeout(timer);
            ws.close();
            if (msg.error) reject(new Error(JSON.stringify(msg.error)));
            else resolve(msg.result);
        });
        ws.addEventListener('error', () => {
            clearTimeout(timer);
            reject(new Error('WebSocket error talking to Chrome'));
        });
    });
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'real' });
    const BASE = started.baseUrl;

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-chrome-'));
    const debugPort = 9229 + Math.floor(Math.random() * 200);
    const url = BASE + '/worksheets/my-morning/index.html#step-3';
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
        let page = null;
        for (let i = 0; i < 40; i += 1) {
            try {
                const list = await get('http://127.0.0.1:' + debugPort + '/json/list');
                const pages = JSON.parse(list.body);
                page = pages.find(p => p.type === 'page' && p.webSocketDebuggerUrl);
                if (page && page.webSocketDebuggerUrl) break;
            } catch (err) { /* retry */ }
            await new Promise(r => setTimeout(r, 250));
        }
        if (!page) throw new Error('No Chrome page for my-morning');

        await cdp(page.webSocketDebuggerUrl, 'Network.enable', {});
        const ck = cookieParts(harness.cookieFor('learner'));
        await cdp(page.webSocketDebuggerUrl, 'Network.setCookie', {
            name: ck.name,
            value: ck.value,
            url: BASE,
            path: '/'
        });
        await cdp(page.webSocketDebuggerUrl, 'Page.enable', {});
        await cdp(page.webSocketDebuggerUrl, 'Runtime.enable', {});
        await cdp(page.webSocketDebuggerUrl, 'Page.navigate', { url: url });
        // Give the player scripts time to boot after first paint
        await new Promise(r => setTimeout(r, 1500));

        let value = null;
        let lastErr = null;
        for (let attempt = 0; attempt < 5; attempt += 1) {
            try {
                const result = await cdp(page.webSocketDebuggerUrl, 'Runtime.evaluate', {
                    awaitPromise: true,
                    returnByValue: true,
                    expression: `(() => new Promise((resolve, reject) => {
                        const deadline = Date.now() + 8000;
                        const wait = () => {
                            if (!window.HebrewSpeech || !window.WORKSHEET || !window.HebrewUtils) {
                                if (Date.now() > deadline) return reject(new Error('player did not boot'));
                                setTimeout(wait, 100);
                                return;
                            }
                            const plays = [];
                            const tts = [];
                            const proto = window.HTMLMediaElement && window.HTMLMediaElement.prototype;
                            if (proto) {
                                proto.play = function () {
                                    plays.push(this.src || this.currentSrc || '');
                                    return Promise.resolve();
                                };
                            }
                            if (window.speechSynthesis) {
                                window.speechSynthesis.speak = function (u) { tts.push(u && u.text || ''); };
                            }
                            const hasWordClip = !!(window.WORKSHEET.vocab.items[0].audio);
                            const hasStory = Array.isArray(window.WORKSHEET.reading.audio) && window.WORKSHEET.reading.audio.length > 0;
                            const word = window.WORKSHEET.vocab.items[0].word;
                            window.HebrewSpeech.play(word, window.WORKSHEET.vocab.items[0].audio);
                            const lines = window.HebrewUtils.storyPlain(window.WORKSHEET.reading.paragraphs);
                            window.HebrewSpeech.startStory(lines, 0, window.WORKSHEET.reading.audio || []);
                            setTimeout(() => {
                                resolve({
                                    hasWordClip,
                                    hasStory,
                                    plays: plays.slice(0, 6),
                                    ttsCount: tts.length,
                                    clipForWord: window.HebrewSpeech.clipFor(word) || '',
                                    storyClip: (window.WORKSHEET.reading.audio || [])[0] || ''
                                });
                            }, 400);
                        };
                        wait();
                    }))()`
                });
                value = result.result && result.result.value;
                if (value) break;
                lastErr = new Error('empty evaluate: ' + JSON.stringify(result));
            } catch (err) {
                lastErr = err;
                await new Promise(r => setTimeout(r, 500));
            }
        }
        if (!value) throw lastErr || new Error('No evaluate result');
        if (!value.hasWordClip) throw new Error('vocab missing audio');
        if (!value.hasStory) throw new Error('reading.audio missing');
        if (!value.clipForWord) throw new Error('clipFor did not find word');
        if (!value.plays.length) throw new Error('expected HTMLMediaElement.play calls, got none; tts=' + value.ttsCount);
        // Prefer clips over TTS for these actions
        if (value.ttsCount > 0 && value.plays.length === 0) {
            throw new Error('TTS used instead of clips');
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
