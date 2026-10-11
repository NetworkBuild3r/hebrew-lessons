#!/usr/bin/env node
/**
 * Headless Chrome smoke: Review — answer stays out of the DOM until reveal,
 * lexicon text is text (not markup), four grade buttons exist.
 * Authored-by-spec: INIT-006/SPEC-012
 *   node test/chrome-review-smoke.js
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const harness = require('./helpers/signed-in-server');
const CHROME = process.env.CHROME_PATH
    || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

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
        ws.addEventListener('message', (event) => {
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
    const baseUrl = started.baseUrl;
    const cookie = harness.cookieFor('learner');
    const seeded = await api(baseUrl, cookie, 'POST', '/api/me/library', {
        words: [{ key: 'בֹּקֶר' }]
    });
    if (seeded.status !== 200) throw new Error('seed failed ' + JSON.stringify(seeded.json));

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-review-'));
    const debugPort = 9300 + Math.floor(Math.random() * 400);
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
                const list = await (await fetch('http://127.0.0.1:' + debugPort + '/json/list')).json();
                page = list.find((p) => p.type === 'page' && p.webSocketDebuggerUrl);
                if (page) break;
            } catch (err) { /* retry */ }
            await sleep(250);
        }
        if (!page) throw new Error('No Chrome page (debug port ' + debugPort + ')');

        await cdp(page.webSocketDebuggerUrl, 'Network.enable', {});
        const ck = cookieParts(cookie);
        await cdp(page.webSocketDebuggerUrl, 'Network.setCookie', {
            name: ck.name,
            value: ck.value,
            url: baseUrl,
            path: '/'
        });
        await cdp(page.webSocketDebuggerUrl, 'Page.enable', {});
        await cdp(page.webSocketDebuggerUrl, 'Page.navigate', { url: baseUrl + '/review/' });
        await sleep(500);

        const value = await cdp(page.webSocketDebuggerUrl, 'Runtime.evaluate', {
            awaitPromise: true,
            returnByValue: true,
            expression: `(() => new Promise((resolve, reject) => {
                const deadline = Date.now() + 12000;
                const wait = () => {
                    const main = document.getElementById('app-main');
                    const card = document.getElementById('review-card');
                    if (!main || main.getAttribute('data-route-ready') !== 'review' || !card) {
                        if (Date.now() > deadline) {
                            return reject(new Error('review card missing: ' + ((document.body && document.body.innerText) || '').slice(0, 240)));
                        }
                        setTimeout(wait, 100);
                        return;
                    }
                    const show = document.getElementById('review-show-answer');
                    if (!show) return reject(new Error('show answer missing'));
                    if (document.getElementById('review-answer')) return reject(new Error('answer node before reveal'));
                    show.click();
                    const answer = document.getElementById('review-answer');
                    const meaning = document.getElementById('review-answer-meaning')
                        || document.getElementById('review-answer-he');
                    const grades = ['1', '2', '3', '4'].map((g) => document.getElementById('review-grade-' + g));
                    resolve({
                        answerVisible: !!(answer && answer.textContent),
                        meaningHtml: meaning ? meaning.innerHTML : '',
                        meaningText: meaning ? meaning.textContent : '',
                        meaningChildren: meaning ? meaning.children.length : -1,
                        xssFired: !!window.__xss,
                        gradeCount: grades.filter(Boolean).length,
                        intervals: grades.map((btn) => btn && btn.querySelector('[data-days]') ? btn.querySelector('[data-days]').getAttribute('data-days') : ''),
                        bilingualShow: /Show answer|הצג תשובה/.test(show.textContent || ''),
                        key: card.getAttribute('data-key'),
                        direction: card.getAttribute('data-direction')
                    });
                };
                wait();
            }))()`
        });
        if (value.exceptionDetails) {
            const ex = value.exceptionDetails.exception;
            throw new Error((ex && ex.description) || value.exceptionDetails.text);
        }
        const report = value.result && value.result.value;
        if (!report) throw new Error('empty evaluate');
        if (!report.answerVisible) throw new Error('answer should show after reveal');
        if (report.xssFired) throw new Error('XSS payload executed');
        if (report.meaningChildren !== 0) throw new Error('meaning inserted elements');
        if (/[<>]/.test(report.meaningHtml || '')) throw new Error('markup in meaning: ' + report.meaningHtml);
        if (report.meaningHtml !== report.meaningText) throw new Error('meaning html differs from text');
        if (report.gradeCount !== 4) throw new Error('expected four grade buttons');
        if (report.intervals.filter(Boolean).length !== 4) throw new Error('expected four interval labels');
        if (!report.bilingualShow) throw new Error('Show answer label not bilingual');
        console.log(JSON.stringify({ ok: true, ...report }, null, 2));
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

main().catch((err) => {
    console.error(err.message || err);
    process.exit(1);
});
