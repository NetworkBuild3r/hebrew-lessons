#!/usr/bin/env node
/**
 * Headless Chrome: Guides shelf opens a published guide's explainer sections.
 * Authored-by-spec: INIT-006/SPEC-016
 *
 *   node test/chrome-guides-smoke.js
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const harness = require('./helpers/signed-in-server');

const CHROME = process.env.CHROME_PATH || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const PLAN_ID = 'guides-shelf';
const HIDDEN_ID = 'secret-guides';

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function guidePlan(id, lesson) {
    return {
        schemaVersion: '1.1.0',
        id: id,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Guides',
        language: { target: 'he', gloss: 'en' },
        units: [
            {
                id: 'verbs',
                title: 'Verbs',
                theme: 'how verbs work',
                lessons: [lesson]
            },
            {
                id: 'reading',
                title: 'Reading',
                theme: 'how to read',
                lessons: [{
                    id: 'how-to-read',
                    title: 'How to read',
                    level: 'basic',
                    visibility: lesson.visibility,
                    recipe: 'guide',
                    buildsOn: [],
                    newWords: [],
                    recycles: [],
                    blocks: [{
                        id: 'read',
                        type: 'explainer',
                        title: 'Reading',
                        sections: [{ id: 'letters', heading: 'Read the letters', paragraphs: ['Start with the letters.'] }]
                    }]
                }]
            }
        ]
    };
}

function explainerLesson(id, title, heading, visibility) {
    return {
        id: id,
        title: title,
        level: 'basic',
        visibility: visibility,
        recipe: 'guide',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{
            id: 'rule',
            type: 'explainer',
            title: 'The root',
            sections: [{
                id: 'roots',
                heading: heading,
                paragraphs: ['A Hebrew verb has a root.']
            }]
        }]
    };
}

async function connect(port) {
    let page = null;
    for (let i = 0; i < 60 && !page; i += 1) {
        try {
            const list = await (await fetch('http://127.0.0.1:' + port + '/json/list')).json();
            page = list.find((item) => item.type === 'page' && item.webSocketDebuggerUrl);
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
        if (msg.method === 'Runtime.exceptionThrown') {
            const details = msg.params && msg.params.exceptionDetails;
            errors.push((details && (details.text || details.exception && details.exception.description)) || 'exception');
        }
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
    return { ws, call, evaluate, errors };
}

async function waitFor(cdp, expression, label) {
    const deadline = Date.now() + 15000;
    let last;
    while (Date.now() < deadline) {
        last = await cdp.evaluate(expression);
        if (last && last.ok) return last;
        await sleep(100);
    }
    throw new Error('timed out waiting for ' + label + ': ' + JSON.stringify(last));
}

function publish(store, publishPlan, id, lesson) {
    const document = guidePlan(id, lesson);
    store.saveWorkingCopy({
        id: id,
        document: JSON.stringify(document),
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null
    });
    const published = publishPlan(store, id, { expectedRevision: 1 }, require('../server/plans/context').makeContext({ channel: 'system', name: 'guides-smoke', tool: 'smoke' }));
    if (!published.plan) {
        throw new Error('publish failed for ' + id + ': ' + JSON.stringify({
            error: published.error,
            blocks: published.blocks
        }));
    }
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'fixture' });
    const base = started.baseUrl;
    const store = require('../server/plans/store');
    const publishPlan = require('../server/plans/publish').publishPlan;
    publish(store, publishPlan, PLAN_ID, explainerLesson('decoding', 'Decoding Hebrew Verbs', 'Three letters', 'everyone'));
    publish(store, publishPlan, HIDDEN_ID, explainerLesson('secret', 'ZZ-HIDDEN-GUIDE', 'Do not show', 'hidden'));

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-guides-'));
    const port = 9800 + Math.floor(Math.random() * 400);
    const chrome = spawn(CHROME, [
        '--headless=new',
        '--disable-gpu',
        '--no-first-run',
        '--no-default-browser-check',
        '--user-data-dir=' + userData,
        '--remote-debugging-port=' + port,
        '--remote-allow-origins=*',
        'about:blank'
    ], { stdio: 'ignore' });

    try {
        const cdp = await connect(port);
        await cdp.call('Network.enable');
        await cdp.call('Runtime.enable');
        await cdp.call('Page.enable');
        const cookie = harness.cookieFor('learner');
        const eq = cookie.indexOf('=');
        await cdp.call('Network.setCookie', {
            name: cookie.slice(0, eq),
            value: decodeURIComponent(cookie.slice(eq + 1)),
            url: base,
            path: '/'
        });
        await cdp.call('Page.navigate', { url: base + '/guides/' });
        const shelf = await waitFor(cdp, `(() => {
            var main = document.getElementById('app-main');
            var ready = main && main.getAttribute('data-route-ready');
            var link = document.querySelector('a[href*="id=guides-shelf"][href*="lesson=decoding"]');
            var text = document.body ? document.body.textContent : '';
            return {
                ok: ready === 'guides' && !!link && text.indexOf('Verbs') >= 0 && text.indexOf('Reading') >= 0
                    && text.indexOf('ZZ-HIDDEN-GUIDE') < 0,
                ready: ready,
                href: link ? link.getAttribute('href') : '',
                hidden: text.indexOf('ZZ-HIDDEN-GUIDE') >= 0
            };
        })()`, 'guides shelf');
        if (shelf.hidden) throw new Error('hidden guide was listed');
        const opened = await cdp.evaluate(`(() => {
            var link = document.querySelector('a[href*="lesson=decoding"]');
            if (!link) return false;
            link.click();
            return true;
        })()`);
        if (!opened) throw new Error('guide link was not clicked');
        const lesson = await waitFor(cdp, `(() => {
            var root = document.getElementById('plan-root');
            var section = document.querySelector('[data-section-id="roots"]');
            var text = section ? section.textContent : '';
            return {
                ok: !!(root && root.getAttribute('data-plan-ready') === '1' && section
                    && text.indexOf('Three letters') >= 0),
                ready: root && root.getAttribute('data-plan-ready'),
                heading: text,
                images: document.querySelectorAll('#plan-root img').length
            };
        })()`, 'guide explainer');
        if (lesson.images !== 0) throw new Error('guide rendered a picture');
        if (cdp.errors.length) throw new Error('page errors: ' + cdp.errors.join(' | '));
        console.log('chrome-guides-smoke: PASS');
        cdp.ws.close();
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
