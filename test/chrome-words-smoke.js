/**
 * Chrome smoke: Words library route (INIT-006/SPEC-013).
 * Signs in, opens /words/, filters by binyan, and opens an unchecked form.
 * Add and suspend are skipped until PR 2 (sessions, CSRF, and the write API).
 *
 * Run: node test/chrome-words-smoke.js
 * Needs Chrome at the default Windows path (or CHROME_PATH).
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');
const harness = require('./helpers/signed-in-server');

const { CHROME } = require('./helpers/chrome-cdp');
const NOUN_RAW = 'שָׁלוֹם';
const VERB_RAW = 'הִסְבִּיר';
const FORM_RAW = 'הִסְבַּרְתִּי';
const NEW_RAW = 'כֶּלֶב';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function api(base, method, apiPath, username, body) {
    const headers = {
        Origin: base,
        Cookie: harness.cookieFor(username)
    };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    const res = await fetch(base + apiPath, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body)
    });
    const text = await res.text();
    let json = null;
    if (text) {
        try { json = JSON.parse(text); } catch (err) { json = null; }
    }
    return { status: res.status, json, text };
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
            const details = res.result.exceptionDetails;
            throw new Error((details.text || 'evaluate failed') + ' in: ' + expression.slice(0, 160));
        }
        return res.result && res.result.result ? res.result.result.value : undefined;
    };
    return { ws, call, evaluate, errors };
}

async function waitFor(cdp, expression, label, timeoutMs) {
    const deadline = Date.now() + (timeoutMs || 15000);
    let last;
    while (Date.now() < deadline) {
        last = await cdp.evaluate(expression);
        if (last && last.ok) return last;
        await sleep(100);
    }
    throw new Error('timed out waiting for ' + label + ': ' + JSON.stringify(last));
}

function rowExpr(word) {
    return `(function () {
        var want = ${JSON.stringify(word)};
        var nodes = document.querySelectorAll('[data-word]');
        for (var i = 0; i < nodes.length; i += 1) {
            if (nodes[i].getAttribute('data-word') === want) return nodes[i];
        }
        return null;
    })()`;
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'fixture' });
    const base = started.baseUrl;
    const exactKey = require('../server/vocab-index').exactKey;
    const lexicon = require('../server/lexicon/service');
    const noun = exactKey(NOUN_RAW);
    const verb = exactKey(VERB_RAW);
    const form = exactKey(FORM_RAW);
    const added = exactKey(NEW_RAW);

    const wrote = lexicon.upsert([
        { key: noun, kind: 'lemma', gloss_en: 'peace', pos: 'noun' },
        {
            key: verb,
            kind: 'lemma',
            gloss_en: 'explained',
            pos: 'verb',
            binyan: 'hifil',
            root: 'ס.ב.ר',
            root_class: 'regular'
        },
        {
            key: form,
            kind: 'form',
            lemma_key: verb,
            gloss_en: 'I explained',
            pos: 'verb',
            binyan: 'hifil',
            tense: 'past',
            cell: '1s'
        }
    ], { actor: 'admin:test', expectedRev: 0 });
    if (!wrote.ok) throw new Error('lexicon seed failed: ' + JSON.stringify(wrote));

    const seeded = await api(base, 'POST', '/api/me/library', 'learner', {
        words: [
            { key: noun, gloss_en: 'peace' },
            { key: verb, gloss_en: 'explained' }
        ]
    });
    if (seeded.status !== 200) throw new Error('library seed failed: ' + seeded.status + ' ' + seeded.text);

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-words-'));
    const port = 9300 + Math.floor(Math.random() * 400);
    const chrome = spawn(CHROME, [
        '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
        '--user-data-dir=' + userData, '--remote-debugging-port=' + port,
        '--remote-allow-origins=*', '--window-size=1280,900', 'about:blank'
    ], { stdio: 'ignore' });

    const report = { ok: false };
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
        await cdp.call('Page.navigate', { url: base + '/words/' });

        const seen = await waitFor(cdp, `(() => {
            var ready = document.getElementById('app-main')
                && document.getElementById('app-main').getAttribute('data-route-ready') === 'words';
            var row = ${rowExpr(noun)};
            var gloss = row && row.querySelector('[data-gloss]');
            var page = document.querySelector('[data-words-state]');
            var status = document.querySelector('[data-words-status]');
            return {
                ok: !!(ready && row && gloss && gloss.textContent === 'peace' && row.getAttribute('data-stage')),
                ready: ready,
                stage: row && row.getAttribute('data-stage'),
                gloss: gloss && gloss.textContent,
                pageState: page && page.getAttribute('data-words-state'),
                status: status && status.textContent,
                words: Array.prototype.map.call(document.querySelectorAll('[data-word]'), function (el) {
                    return el.getAttribute('data-word');
                })
            };
        })()`, 'seeded word');
        report.ac1 = seen;

        await cdp.evaluate(`(() => {
            var sel = document.querySelector('[data-filter="binyan"]');
            sel.value = 'hifil';
            sel.dispatchEvent(new Event('change', { bubbles: true }));
            return true;
        })()`);
        const filtered = await waitFor(cdp, `(() => {
            var ready = document.querySelector('[data-words-state="ready"]');
            var rows = Array.from(document.querySelectorAll('[data-word]'));
            var words = rows.map(function (row) { return row.getAttribute('data-word'); });
            var link = document.querySelector('[data-words-review]');
            return {
                ok: !!(ready
                    && location.pathname === '/words/'
                    && location.search.indexOf('binyan=hifil') >= 0
                    && words.indexOf(${JSON.stringify(verb)}) >= 0
                    && words.indexOf(${JSON.stringify(noun)}) < 0
                    && rows.every(function (row) { return row.getAttribute('data-binyan') === 'hifil'; })
                    && link && link.getAttribute('data-review-deck') === 'concept:binyan:hifil'),
                search: location.search,
                words: words,
                deck: link && link.getAttribute('data-review-deck')
            };
        })()`, 'hifil filter');
        report.ac2 = { filtered: filtered };

        await cdp.evaluate('history.back(); true');
        const restored = await waitFor(cdp, `(() => {
            var ready = document.querySelector('[data-words-state="ready"]');
            var row = ${rowExpr(noun)};
            return {
                ok: !!(ready && location.search.indexOf('binyan=') < 0 && row),
                search: location.search,
                noun: !!row
            };
        })()`, 'back restores the list');
        report.ac2.restored = restored;

        await cdp.evaluate(`(() => {
            var row = ${rowExpr(verb)};
            if (!row) return false;
            row.click();
            return true;
        })()`);
        const paradigm = await waitFor(cdp, `(() => {
            var text = document.body ? document.body.innerText : '';
            return { ok: text.indexOf('not yet checked') >= 0 };
        })()`, 'unverified form');
        report.paradigm = paradigm;

        // TODO(PR 2): restore add and suspend once sessions, CSRF, and the
        // learner/library write API exist. The Go page is read-only, so these
        // steps cannot run against it, and they are skipped here too so the
        // smoke stays one script. NEW_RAW stays for that follow-up.
        void added;
        report.ac3 = { skipped: 'PR 2: sessions, CSRF, and the write API' };

        if (cdp.errors.length) throw new Error('page errors: ' + cdp.errors.join(' | '));
        report.ok = true;
        console.log(JSON.stringify(report, null, 2));
        console.log('chrome-words-smoke: PASS');
        cdp.ws.close();
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-words-smoke") || main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
