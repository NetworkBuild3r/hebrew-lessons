#!/usr/bin/env node
/**
 * Headless Chrome: Review flashcards — reveal, four grades, server Good.
 * Authored-by-spec: INIT-006/SPEC-012
 *
 *   node test/chrome-flashcards-smoke.js
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const harness = require('./helpers/signed-in-server');

const CHROME = process.env.CHROME_PATH || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const WORDS = ['בֹּקֶר', 'קָם'];
const TYPED = 'לא-נכון';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function api(baseUrl, cookie, method, urlPath, body) {
    const res = await fetch(baseUrl + urlPath, {
        method,
        headers: {
            'Content-Type': 'application/json',
            Origin: baseUrl,
            Cookie: cookie
        },
        body: body == null ? undefined : JSON.stringify(body)
    });
    const text = await res.text();
    let json = null;
    if (text) {
        try { json = JSON.parse(text); } catch (err) { json = null; }
    }
    return { status: res.status, json };
}

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
    ws.addEventListener('message', (ev) => {
        const msg = JSON.parse(ev.data);
        if (msg.id && pending.has(msg.id)) {
            pending.get(msg.id)(msg);
            pending.delete(msg.id);
        }
    });
    const call = (method, params) => new Promise((resolve, reject) => {
        seq += 1;
        const id = seq;
        pending.set(id, (msg) => {
            if (msg.error) reject(new Error(JSON.stringify(msg.error)));
            else resolve(msg.result || {});
        });
        ws.send(JSON.stringify({ id, method, params: params || {} }));
    });
    const evaluate = async (expression) => {
        const res = await call('Runtime.evaluate', {
            expression,
            returnByValue: true,
            awaitPromise: true
        });
        if (res.exceptionDetails) {
            const ex = res.exceptionDetails.exception;
            throw new Error((ex && ex.description) || res.exceptionDetails.text || 'evaluate failed');
        }
        return res.result ? res.result.value : undefined;
    };
    return { ws, call, evaluate };
}

function cookieParts(cookieHeader) {
    const eq = cookieHeader.indexOf('=');
    return {
        name: cookieHeader.slice(0, eq),
        value: decodeURIComponent(cookieHeader.slice(eq + 1))
    };
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'real' });
    const baseUrl = started.baseUrl;
    const cookie = harness.cookieFor('learner');
    const userId = started.users.learner.id;
    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-flash-'));
    const debugPort = 9400 + Math.floor(Math.random() * 400);
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
        const added = await api(baseUrl, cookie, 'POST', '/api/me/library', {
            words: WORDS.map((key) => ({ key }))
        });
        if (added.status !== 200) throw new Error('seed failed ' + added.status + ' ' + JSON.stringify(added.json));
        const deck = await api(baseUrl, cookie, 'GET', '/api/me/cards?deck=due');
        if (deck.status !== 200 || !deck.json || !Array.isArray(deck.json.cards)) {
            throw new Error('due deck failed ' + JSON.stringify(deck));
        }
        const cards = deck.json.cards;
        if (cards.length < 2) throw new Error('expected at least two due cards, got ' + cards.length);
        const session = cards.slice(0, 4);
        const grades = [1, 2, 3, 4].slice(0, session.length);
        const goodIndex = grades.indexOf(3);

        const cdp = await connect(debugPort);
        await cdp.call('Network.enable');
        const ck = cookieParts(cookie);
        await cdp.call('Network.setCookie', { name: ck.name, value: ck.value, url: baseUrl, path: '/' });
        await cdp.call('Page.enable');
        await cdp.call('Page.navigate', { url: baseUrl + '/review/' });

        const report = await cdp.evaluate(`(() => new Promise((resolve, reject) => {
            const cards = ${JSON.stringify(session)};
            const grades = ${JSON.stringify(grades)};
            const typed = ${JSON.stringify(TYPED)};
            const deadline = Date.now() + 15000;
            const wait = () => {
                const main = document.getElementById('app-main');
                const card = document.getElementById('review-card');
                if (!main || main.getAttribute('data-route-ready') !== 'review' || !card
                    || !window.HebrewSync || typeof window.HebrewSync.queueSignal !== 'function') {
                    if (Date.now() > deadline) return reject(new Error('review card did not appear'));
                    setTimeout(wait, 100);
                    return;
                }
                const orig = window.HebrewSync.queueSignal.bind(window.HebrewSync);
                const seen = [];
                window.HebrewSync.queueSignal = function (sig) {
                    seen.push(JSON.parse(JSON.stringify(sig)));
                    return orig(sig);
                };
                const press = (key) => {
                    document.dispatchEvent(new KeyboardEvent('keydown', { key: key, code: key === ' ' ? 'Space' : ('Digit' + key), bubbles: true }));
                };
                const step = (i) => {
                    const live = document.getElementById('review-card');
                    if (!live) return reject(new Error('card missing at step ' + i));
                    const answer = cards[i].answer;
                    const stage = document.getElementById('review-stage');
                    const text = (stage && stage.textContent) || '';
                    let leaked = text.indexOf(answer) !== -1 ? 'text' : '';
                    if (!leaked && answer && stage) {
                        stage.querySelectorAll('*').forEach((node) => {
                            ['data-speak', 'data-vowels', 'data-audio', 'data-key'].forEach((attr) => {
                                const value = node.getAttribute(attr) || '';
                                if (value.indexOf(answer) !== -1) leaked = attr;
                            });
                        });
                    }
                    if (answer && leaked) {
                        return reject(new Error('answer present before reveal at step ' + i + ' via ' + leaked));
                    }
                    if (document.getElementById('review-answer')) {
                        return reject(new Error('answer node present before reveal'));
                    }
                    ['1', '2', '3', '4'].forEach(press);
                    if (document.getElementById('review-answer')) {
                        return reject(new Error('a grade key revealed the answer'));
                    }
                    live.focus();
                    const input = document.getElementById('review-typed');
                    if (input) {
                        input.value = typed;
                        input.blur();
                        live.focus();
                    }
                    press(' ');
                    const shown = document.getElementById('review-answer');
                    if (!shown) return reject(new Error('Space did not reveal'));
                    if (answer && (document.body.textContent || '').indexOf(answer) === -1) {
                        return reject(new Error('answer missing after Space'));
                    }
                    const days = ['1', '2', '3', '4'].map((g) => {
                        const btn = document.getElementById('review-grade-' + g);
                        const interval = btn && btn.querySelector('[data-days]');
                        return interval ? interval.getAttribute('data-days') : null;
                    });
                    const expected = ['1', '2', '3', '4'].map((g) => String(cards[i].intervals[g]));
                    press(String(grades[i]));
                    const next = () => {
                        if (i + 1 >= grades.length) {
                            return resolve({
                                days,
                                expected,
                                signals: seen,
                                showLabel: (document.getElementById('review-show-answer') || {}).textContent || ''
                            });
                        }
                        if (!document.getElementById('review-card')) {
                            return reject(new Error('next card missing after grade ' + grades[i]));
                        }
                        step(i + 1);
                    };
                    setTimeout(next, 50);
                };
                step(0);
            };
            wait();
        }))()`);

        if (!report || !Array.isArray(report.days)) throw new Error('empty report ' + JSON.stringify(report));
        if (report.days.join(',') !== report.expected.join(',')) {
            throw new Error('interval labels ' + report.days.join(',') + ' != ' + report.expected.join(','));
        }
        if (!report.signals || report.signals.length < grades.length) {
            throw new Error('signals ' + JSON.stringify(report.signals));
        }
        report.signals.slice(0, grades.length).forEach((sig, i) => {
            if (sig.grade !== grades[i]) throw new Error('signal grade ' + sig.grade + ' at ' + i);
            if (JSON.stringify(sig).includes(TYPED)) throw new Error('typed text was sent');
            if (Object.prototype.hasOwnProperty.call(sig, 'typed')) throw new Error('typed field was sent');
        });

        const reviewRepo = require('../server/db/repos/review');
        const good = session[goodIndex];
        let row = null;
        for (let i = 0; i < 40; i += 1) {
            row = reviewRepo.find(userId, good.word, good.direction);
            if (row && Number(row.last_grade) === 3) break;
            await sleep(250);
        }
        if (!row || Number(row.last_grade) !== 3) {
            throw new Error('server grade for ' + good.word + ' ' + good.direction + ' is ' + (row && row.last_grade));
        }
        const goods = session.filter((card) => {
            const stored = reviewRepo.find(userId, card.word, card.direction);
            return stored && Number(stored.last_grade) === 3;
        });
        if (goods.length !== 1) throw new Error('expected one Good grade, saw ' + goods.length);

        console.log(JSON.stringify({
            ok: true,
            cards: session.length,
            good: { word: good.word, direction: good.direction, last_grade: 3 },
            signals: report.signals.map((sig) => sig.grade)
        }, null, 2));
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
