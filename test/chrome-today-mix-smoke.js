#!/usr/bin/env node
/**
 * Headless Chrome: Today daily mix — counts, in-place Review, return.
 * Authored-by-spec: INIT-006/SPEC-015
 *
 *   node test/chrome-today-mix-smoke.js
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const harness = require('./helpers/signed-in-server');

const { CHROME } = require('./helpers/chrome-cdp');
const DAY_MS = 24 * 60 * 60 * 1000;
const DUE_WORD = 'בֹּקֶר';
const WEAK_WORD = 'כזזק';

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

function section(mix, id) {
    const list = mix && Array.isArray(mix.sections) ? mix.sections : [];
    return list.find((entry) => entry && entry.id === id) || null;
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'real' });
    const baseUrl = started.baseUrl;
    const cookie = harness.cookieFor('learner');
    const userId = started.users.learner.id;
    const review = require('../server/db/repos/review');
    const now = Date.now();
    const past = new Date(now - DAY_MS).toISOString();
    const future = new Date(now + 5 * DAY_MS).toISOString();
    const ago = new Date(now - 2 * DAY_MS).toISOString();

    review.upsert({
        user_id: userId,
        word: DUE_WORD,
        direction: 'he_en',
        due: past,
        step: 2,
        streak: 1,
        lapses: 0,
        last_result: 'pass',
        last_at: ago,
        sources: JSON.stringify(['my-morning']),
        last_grade: 3,
        suspended: 0,
        stability: 2,
        difficulty: 5,
        reps: 2,
        fsrs_state: 'review'
    });
    review.upsert({
        user_id: userId,
        word: WEAK_WORD,
        direction: 'he_en',
        due: future,
        step: 1,
        streak: 0,
        lapses: 2,
        last_result: 'pass',
        last_at: ago,
        sources: JSON.stringify(['my-morning']),
        last_grade: 3,
        suspended: 0,
        stability: 1,
        difficulty: 5,
        reps: 2,
        fsrs_state: 'review'
    });

    const mixRes = await api(baseUrl, cookie, 'GET', '/api/me/mix');
    if (mixRes.status !== 200 || !mixRes.json) {
        throw new Error('mix failed ' + mixRes.status + ' ' + JSON.stringify(mixRes.json));
    }
    const reviewSection = section(mixRes.json, 'review');
    const weakSection = section(mixRes.json, 'weak');
    const drillSection = section(mixRes.json, 'drill');
    const readingSection = section(mixRes.json, 'reading');
    if (!reviewSection || !reviewSection.items.length) {
        throw new Error('expected review items, got ' + JSON.stringify(mixRes.json.sections));
    }
    if (!weakSection || !weakSection.items.length) {
        throw new Error('expected weak items, got ' + JSON.stringify(mixRes.json.sections));
    }
    if (drillSection) throw new Error('drill should be absent: ' + JSON.stringify(drillSection));
    if (readingSection) throw new Error('reading should be absent: ' + JSON.stringify(readingSection));

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-today-mix-'));
    const debugPort = 9700 + Math.floor(Math.random() * 200);
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
        const cdp = await connect(debugPort);
        await cdp.call('Network.enable');
        const ck = cookieParts(cookie);
        await cdp.call('Network.setCookie', { name: ck.name, value: ck.value, url: baseUrl, path: '/' });
        await cdp.call('Page.enable');
        await cdp.call('Page.navigate', { url: baseUrl + '/' });

        const meaning = JSON.stringify(reviewSection.items[0].meaning || '');
        const report = await cdp.evaluate(`(() => new Promise((resolve, reject) => {
            const meaning = ${meaning};
            const deadline = Date.now() + 20000;
            const wait = () => {
                const review = document.getElementById('today-mix-review');
                const weak = document.getElementById('today-mix-weak');
                const ready = document.querySelector('[data-today-state="ready"]');
                if (!review || !weak || !ready) {
                    if (Date.now() > deadline) {
                        return reject(new Error('mix sections missing: ' + (document.body && document.body.innerText || '').slice(0, 400)));
                    }
                    setTimeout(wait, 150);
                    return;
                }
                const reviewCount = review.querySelector('[data-mix-count="review"]');
                const weakCount = weak.querySelector('[data-mix-count="weak"]');
                const start = review.querySelector('[data-mix-action="start"]');
                if (!start) return reject(new Error('review start missing'));
                start.click();
                const session = document.getElementById('today-mix-session');
                const prompt = document.getElementById('today-mix-prompt');
                const answer = document.getElementById('today-mix-answer');
                const sessionText = (session && session.textContent) || '';
                const leaked = meaning && meaning !== (prompt && prompt.textContent) && sessionText.indexOf(meaning) !== -1;
                const back = document.getElementById('today-mix-session')
                    && document.querySelector('#today-mix-session [data-mix-action="back"]');
                if (back) back.click();
                const oldStart = document.getElementById('today-review-start');
                const mix = document.getElementById('today-mix');
                const below = document.getElementById('today-review-card')
                    || document.getElementById('today-continue-card')
                    || document.getElementById('today-next-card')
                    || document.getElementById('today-welcome-card');
                resolve({
                    path: location.pathname,
                    reviewCount: reviewCount && reviewCount.textContent,
                    weakCount: weakCount && weakCount.textContent,
                    drill: !!document.getElementById('today-mix-drill'),
                    reading: !!document.getElementById('today-mix-reading'),
                    sessionOpened: !!session,
                    answerBeforeReveal: !!answer,
                    leaked: !!leaked,
                    listBack: !!document.getElementById('today-mix-list'),
                    sessionGone: !document.getElementById('today-mix-session'),
                    oldHref: oldStart ? oldStart.getAttribute('href') : null,
                    mixFirst: !!(mix && below && (mix.compareDocumentPosition(below) & Node.DOCUMENT_POSITION_FOLLOWING)),
                    route: (document.getElementById('app-main') || {}).getAttribute
                        && document.getElementById('app-main').getAttribute('data-route-ready')
                });
            };
            wait();
        }))()`);

        if (report.path !== '/') throw new Error('left Today: ' + report.path);
        if (report.route !== 'today') throw new Error('route ready was ' + report.route);
        if (String(report.reviewCount) !== String(reviewSection.items.length)) {
            throw new Error('review count ' + report.reviewCount + ' != ' + reviewSection.items.length);
        }
        if (String(report.weakCount) !== String(weakSection.items.length)) {
            throw new Error('weak count ' + report.weakCount + ' != ' + weakSection.items.length);
        }
        if (report.drill || report.reading) throw new Error('drill or reading rendered ' + JSON.stringify(report));
        if (!report.sessionOpened) throw new Error('review did not open in place');
        if (report.answerBeforeReveal) throw new Error('answer node existed before reveal');
        if (report.leaked) throw new Error('meaning was in the session before reveal');
        if (!report.listBack || !report.sessionGone) throw new Error('did not return to the mix list ' + JSON.stringify(report));
        if (report.oldHref && report.oldHref !== '/review/') {
            throw new Error('existing review link changed: ' + report.oldHref);
        }
        if (!report.mixFirst) throw new Error('mix card is not above the existing Today cards');

        console.log(JSON.stringify({
            ok: true,
            review: report.reviewCount,
            weak: report.weakCount,
            path: report.path,
            oldHref: report.oldHref
        }));
    } finally {
        try { chrome.kill(); } catch (err) { /* ignore */ }
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-today-mix-smoke") || main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
