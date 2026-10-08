#!/usr/bin/env node
/**
 * Headless Chrome smoke: Review today — reveal, grade, store update.
 * Authored-by-spec: INIT-001/SPEC-004
 * Sign-in first (INIT-002/SPEC-012). Ephemeral port via signed-in-server.
 *
 *   node test/chrome-review-smoke.js
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
        }, 25000);
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

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-review-'));
    const debugPort = 9300 + Math.floor(Math.random() * 400);
    const url = BASE + '/review/index.html';
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
                if (page) break;
            } catch (err) { /* retry */ }
            await new Promise(r => setTimeout(r, 250));
        }
        if (!page) throw new Error('No Chrome page (debug port ' + debugPort + ')');

        await cdp(page.webSocketDebuggerUrl, 'Network.enable', {});
        const ck = cookieParts(harness.cookieFor('learner'));
        await cdp(page.webSocketDebuggerUrl, 'Network.setCookie', {
            name: ck.name,
            value: ck.value,
            url: BASE,
            path: '/'
        });
        await cdp(page.webSocketDebuggerUrl, 'Page.enable', {});
        await cdp(page.webSocketDebuggerUrl, 'Page.navigate', { url: url });
        await new Promise(r => setTimeout(r, 800));

        // Refresh page handle after navigate
        for (let i = 0; i < 20; i += 1) {
            try {
                const list = await get('http://127.0.0.1:' + debugPort + '/json/list');
                const pages = JSON.parse(list.body);
                const hit = pages.find(p => p.type === 'page' && (p.url || '').includes('/review'));
                if (hit && hit.webSocketDebuggerUrl) {
                    page = hit;
                    break;
                }
            } catch (err) { /* retry */ }
            await new Promise(r => setTimeout(r, 200));
        }

        await cdp(page.webSocketDebuggerUrl, 'Page.enable', {});
        await cdp(page.webSocketDebuggerUrl, 'Runtime.enable', {});
        await new Promise(r => setTimeout(r, 1200));

        let value = null;
        let lastErr = null;
        for (let attempt = 0; attempt < 5; attempt += 1) {
            try {
                const result = await cdp(page.webSocketDebuggerUrl, 'Runtime.evaluate', {
                    awaitPromise: true,
                    returnByValue: true,
                    expression: `(() => new Promise((resolve, reject) => {
                        const deadline = Date.now() + 10000;
                        const wait = () => {
                            if (!window.HebrewReview || !window.HebrewUtils || !window.HebrewReviewPage) {
                                if (Date.now() > deadline) return reject(new Error('review page did not boot'));
                                setTimeout(wait, 100);
                                return;
                            }
                            if (typeof window.HEBREW_VOCAB_INDEX === 'undefined') {
                                if (Date.now() > deadline) return reject(new Error('vocab index missing'));
                                setTimeout(wait, 100);
                                return;
                            }

                            const past = new Date(Date.now() - 86400000).toISOString();
                            const dir = (extra) => Object.assign({
                                due: past,
                                step: 0,
                                streak: 0,
                                lapses: 0,
                                lastResult: null,
                                lastAt: null
                            }, extra || {});
                            const xssKey = 'בֹּקֶר';
                            const store = {
                                v: 1,
                                words: {
                                    [xssKey]: {
                                        he_en: dir({ lastResult: 'pass', lastAt: past, step: 0 }),
                                        en_he: dir(),
                                        sources: ['my-morning']
                                    },
                                    'אָנָלִי': {
                                        he_en: dir({ lastResult: 'pass', lastAt: past }),
                                        en_he: dir(),
                                        sources: ['anal-sex-vocab']
                                    },
                                    'קָם': {
                                        he_en: dir({ lastResult: 'pass', lastAt: past }),
                                        en_he: dir(),
                                        sources: ['my-morning']
                                    }
                                },
                                migratedLessons: {}
                            };

                            // Inject XSS meaning into in-memory index entry if present
                            const idx = window.HEBREW_VOCAB_INDEX;
                            const list = Array.isArray(idx) ? idx : [];
                            const morning = list.find(e => e && e.key === xssKey);
                            if (morning) {
                                morning.meaning = '<img onerror="window.__xss=1">evil';
                            }

                            localStorage.setItem('hebrew-review:v1', JSON.stringify(store));
                            window.HebrewReview._resetMemory(null);
                            window.HebrewReview.load();

                            // Reboot UI
                            window.HebrewReviewPage.start();

                            setTimeout(() => {
                                const answer = document.getElementById('review-answer');
                                const showBtn = document.getElementById('review-show-answer');
                                const card = document.getElementById('review-card');
                                const noIndex = document.getElementById('review-no-index');
                                if (noIndex) {
                                    return reject(new Error('unexpected missing-index state'));
                                }
                                if (!card || !showBtn) {
                                    return reject(new Error('review card missing; body=' + (document.body.innerText || '').slice(0, 200)));
                                }
                                const answerHiddenBefore = !answer || answer.classList.contains('hidden');
                                const lessonsSeen = [];
                                const firstLesson = card.getAttribute('data-lesson') || '';
                                lessonsSeen.push(firstLesson);

                                showBtn.click();
                                const answerEl = document.getElementById('review-answer');
                                const answerVisible = answerEl && !answerEl.classList.contains('hidden');
                                const meaningHtml = (document.getElementById('review-answer-meaning')
                                    || document.getElementById('review-prompt-en')
                                    || { innerHTML: '' }).innerHTML;
                                const xssFired = !!window.__xss;
                                const meaningText = (document.getElementById('review-answer-meaning')
                                    || { textContent: '' }).textContent || '';

                                const beforeRaw = localStorage.getItem('hebrew-review:v1');
                                const before = JSON.parse(beforeRaw);
                                const key = card.getAttribute('data-key');
                                const direction = card.getAttribute('data-direction');
                                const beforeDue = before.words[key][direction].due;

                                document.getElementById('review-knew').click();

                                setTimeout(() => {
                                    const after = JSON.parse(localStorage.getItem('hebrew-review:v1'));
                                    const afterRec = after.words[key][direction];
                                    const card2 = document.getElementById('review-card');
                                    if (card2) lessonsSeen.push(card2.getAttribute('data-lesson') || '');

                                    resolve({
                                        answerHiddenBefore,
                                        answerVisible,
                                        xssFired,
                                        meaningHtml,
                                        meaningText,
                                        key,
                                        direction,
                                        beforeDue,
                                        afterDue: afterRec.due,
                                        afterResult: afterRec.lastResult,
                                        afterStep: afterRec.step,
                                        lessonsSeen,
                                        bilingualShow: !!(showBtn && /Show answer|הצג תשובה/.test(showBtn.textContent || '')),
                                        hasVocabIndex: Array.isArray(window.HEBREW_VOCAB_INDEX)
                                    });
                                }, 200);
                            }, 300);
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
        if (!value.answerHiddenBefore) throw new Error('answer should be hidden before reveal');
        if (!value.answerVisible) throw new Error('answer should show after reveal');
        if (value.xssFired) throw new Error('XSS payload executed');
        if (/<img/i.test(value.meaningHtml || '')) throw new Error('unescaped img in meaning HTML: ' + value.meaningHtml);
        if (value.afterResult !== 'pass') throw new Error('expected pass after Knew it, got ' + value.afterResult);
        if (value.afterDue === value.beforeDue) throw new Error('due date did not advance');
        if (!value.hasVocabIndex) throw new Error('vocab index not loaded');
        if (!value.bilingualShow) throw new Error('Show answer label not bilingual');

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
