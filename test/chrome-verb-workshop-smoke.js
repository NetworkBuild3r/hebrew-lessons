#!/usr/bin/env node
/**
 * Headless Chrome: verb workshop drill signals and a personal-question done mark.
 * Authored-by-spec: INIT-006/SPEC-014
 *
 *   node test/chrome-verb-workshop-smoke.js
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const harness = require('./helpers/signed-in-server');

const CHROME = process.env.CHROME_PATH || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const PLAN_ID = 'verb-workshop';
const LESSON = 'public-lesson';
const LEMMA = 'לַחְשֹׁב';
const ROOT = 'ח.ש.ב';
const SECRET = 'pq-secret-typed';
const WRONG = 'WRONG-XYZ';
const UNPOINTED = 'חשבתי';
const FORM_1S = 'חָשַׁבְתִּי';
const FORM_2MS = 'חָשַׁבְתָּ';
const FORM_UNVERIFIED = 'חָשַׁב';

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function entry(row) {
    const form = row.cell != null;
    return {
        key: row.key,
        kind: form ? 'form' : 'lemma',
        lemma_key: form ? LEMMA : undefined,
        gloss_en: row.gloss || 'to think',
        pos: 'verb',
        root: ROOT,
        binyan: 'paal',
        tense: row.tense,
        cell: row.cell,
        accept: row.accept || []
    };
}

function planDocument() {
    return {
        schemaVersion: '1.1.0',
        id: PLAN_ID,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Verb workshop',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'verbs',
            title: 'Verbs',
            theme: 'thinking',
            lessons: [{
                id: LESSON,
                title: 'לחשוב',
                level: 'basic',
                visibility: 'everyone',
                recipe: 'verb-workshop',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks: [
                    {
                        id: 'rule',
                        type: 'explainer',
                        title: 'Present tense',
                        sections: [{
                            id: 'forms',
                            heading: 'Four forms',
                            paragraphs: ['The present tense has four forms.'],
                            callouts: [{
                                id: 'go',
                                tone: 'green',
                                title: 'Three root letters',
                                text: 'The root stays in place.'
                            }]
                        }]
                    },
                    {
                        id: 'present',
                        type: 'conjugation-table',
                        lemmaKey: LEMMA,
                        tense: 'present',
                        mode: 'study',
                        cells: [
                            { cell: 'ms', formKey: 'חוֹשֵׁב' },
                            { cell: 'fs', formKey: 'חוֹשֶׁבֶת' },
                            { cell: 'mp', formKey: 'חוֹשְׁבִים' },
                            { cell: 'fp', formKey: 'חוֹשְׁבוֹת' }
                        ],
                        sentences: [{ id: 's1', he: 'אֲנִי חוֹשֵׁב.', en: 'I think.' }],
                        questions: [{ id: 'q1', he: 'עַל מָה?', en: 'About what?' }]
                    },
                    {
                        id: 'past',
                        type: 'conjugation-table',
                        lemmaKey: LEMMA,
                        tense: 'past',
                        mode: 'drill',
                        cells: [
                            { cell: '1s', formKey: FORM_1S },
                            { cell: '2ms', formKey: FORM_2MS },
                            { cell: '3ms', formKey: FORM_UNVERIFIED }
                        ],
                        sentences: [{ id: 's1', he: 'אֲנִי קורא.', en: 'I am reading.' }],
                        questions: [{ id: 'q1', he: 'מָה עָשִׂיתָ?', en: 'What did you do?' }]
                    },
                    {
                        id: 'you',
                        type: 'personal-question',
                        prompts: [{ id: 'p1', he: 'עַל מָה אַתָּה חוֹשֵׁב?', en: 'What are you thinking about?' }],
                        answer: 'either'
                    }
                ]
            }]
        }]
    };
}

async function connect(port, requests) {
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
        if (msg.method === 'Network.requestWillBeSent' && msg.params && msg.params.request) {
            const req = msg.params.request;
            requests.push({
                url: req.url,
                method: req.method,
                postData: req.postData || ''
            });
        }
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
    const deadline = Date.now() + 8000;
    let last;
    while (Date.now() < deadline) {
        last = await cdp.evaluate(expression);
        if (last && last.ok) return last;
        await sleep(100);
    }
    throw new Error('timed out waiting for ' + label + ': ' + JSON.stringify(last));
}

function stationExpr() {
    return `(() => {
        var prompt = document.querySelector('[data-drill-prompt]');
        var feedback = document.querySelector('[data-drill-feedback]');
        var reveal = document.querySelector('[data-drill-reveal]');
        var next = document.querySelector('[data-drill-next]');
        var text = prompt ? prompt.textContent : '';
        return {
            cell: prompt && prompt.getAttribute('data-drill-cell'),
            feedback: feedback && !feedback.hidden ? feedback.textContent : '',
            reveal: reveal && !reveal.hidden ? reveal.textContent : '',
            next: !!(next && !next.hidden),
            done: text.indexOf('Done') >= 0
        };
    })()`;
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'fixture' });
    const base = started.baseUrl;
    const userId = started.users.learner.id;
    const lexicon = require('../server/lexicon/service');
    const store = require('../server/plans/store');
    const publish = require('../server/plans/publish');
    const db = require('../server/db');

    const rows = [
        { key: LEMMA, gloss: 'to think' },
        { key: 'חוֹשֵׁב', tense: 'present', cell: 'ms', gloss: 'thinks (m.)' },
        { key: 'חוֹשֶׁבֶת', tense: 'present', cell: 'fs', gloss: 'thinks (f.)' },
        { key: 'חוֹשְׁבִים', tense: 'present', cell: 'mp', gloss: 'think (m. pl.)' },
        { key: 'חוֹשְׁבוֹת', tense: 'present', cell: 'fp', gloss: 'think (f. pl.)' },
        { key: FORM_1S, tense: 'past', cell: '1s', gloss: 'I thought', accept: [UNPOINTED] },
        { key: FORM_2MS, tense: 'past', cell: '2ms', gloss: 'you thought', accept: ['חשבת'] },
        { key: FORM_UNVERIFIED, tense: 'past', cell: '3ms', gloss: 'he thought' }
    ];
    const wrote = lexicon.upsert(rows.map(entry), { actor: 'admin:test', expectedRev: 0 });
    if (!wrote.ok) throw new Error('lexicon seed failed: ' + JSON.stringify(wrote.errors || wrote));
    const verified = lexicon.setVerified(
        ['חוֹשֵׁב', 'חוֹשֶׁבֶת', 'חוֹשְׁבִים', FORM_1S, FORM_2MS],
        true,
        { actor: 'admin:test' }
    );
    if (!verified.ok) throw new Error('verify failed: ' + JSON.stringify(verified));

    const document = planDocument();
    store.saveWorkingCopy({
        id: PLAN_ID,
        document: JSON.stringify(document),
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null
    });
    const published = publish.publishPlan(store, PLAN_ID, { expectedRevision: 1 });
    if (!published.plan) {
        throw new Error('publish failed: ' + JSON.stringify({
            error: published.error,
            blocks: published.blocks
        }));
    }

    const reviewsBefore = db.all('SELECT word FROM review_items WHERE user_id = ?', userId);
    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-verb-'));
    const port = 9700 + Math.floor(Math.random() * 400);
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
    const requests = [];

    try {
        const cdp = await connect(port, requests);
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
        await cdp.call('Page.navigate', { url: base + '/shared/plan/?id=' + PLAN_ID });

        const ready = await waitFor(cdp, `(() => {
            var root = document.getElementById('plan-root');
            var study = document.querySelector('[data-mode="study"] table');
            var drill = document.querySelector('[data-drill-owner="1"]');
            var personal = document.querySelector('[data-block-type="personal-question"]');
            var heads = study ? study.querySelectorAll('th') : [];
            var labels = Array.prototype.map.call(heads, function (th) { return th.textContent; });
            var order = ['SM', 'SF', 'PM', 'PF'].every(function (label, index) {
                return labels[index] && labels[index].indexOf(label) >= 0;
            });
            var drillText = drill ? drill.textContent : '';
            return {
                ok: !!(root && root.getAttribute('data-plan-ready') === '1' && study && drill && personal && order),
                ready: root && root.getAttribute('data-plan-ready'),
                order: order,
                labels: labels,
                unverifiedMarked: study ? study.textContent.indexOf('not yet checked') >= 0 : false,
                drilledUnverified: drillText.indexOf(${JSON.stringify(FORM_UNVERIFIED)}) >= 0,
                images: document.querySelectorAll('#plan-root img').length,
                pad: drill ? drill.querySelectorAll('[data-letter]').length : 0
            };
        })()`, 'workshop ready');
        if (!ready.unverifiedMarked) throw new Error('study table did not mark an unverified form');
        if (ready.drilledUnverified) throw new Error('drill rendered an unverified form');
        if (ready.images !== 0) throw new Error('plan root has a picture');
        if (ready.pad < 22) throw new Error('letter pad is missing: ' + ready.pad);

        let sawPass = false;
        let sawRetry = false;
        let sawReveal = false;
        for (let step = 0; step < 6; step += 1) {
            const now = await cdp.evaluate(stationExpr());
            if (now.done) break;
            if (now.cell === '3ms') throw new Error('unverified cell was drilled');
            if (now.cell === '1s') {
                await cdp.evaluate(`(() => {
                    document.querySelector('[data-drill-input]').value = ${JSON.stringify(UNPOINTED)};
                    document.querySelector('[data-drill-check]').click();
                    return true;
                })()`);
                const passed = await waitFor(cdp, `(() => {
                    var state = ${stationExpr()};
                    state.ok = state.reveal === ${JSON.stringify(FORM_1S)} && state.feedback.indexOf('Correct') >= 0;
                    return state;
                })()`, 'unpointed pass');
                if (!passed.ok) throw new Error('unpointed answer did not pass: ' + JSON.stringify(passed));
                sawPass = true;
            } else if (now.cell === '2ms') {
                await cdp.evaluate(`(() => {
                    document.querySelector('[data-drill-input]').value = ${JSON.stringify(WRONG)};
                    document.querySelector('[data-drill-check]').click();
                    return true;
                })()`);
                const retry = await waitFor(cdp, `(() => {
                    var state = ${stationExpr()};
                    state.ok = state.feedback.indexOf('try again') >= 0 && state.reveal === '';
                    return state;
                })()`, 'try again');
                sawRetry = true;
                await cdp.evaluate(`(() => {
                    document.querySelector('[data-drill-input]').value = ${JSON.stringify(WRONG)};
                    document.querySelector('[data-drill-check]').click();
                    return true;
                })()`);
                const revealed = await waitFor(cdp, `(() => {
                    var state = ${stationExpr()};
                    state.ok = state.reveal === ${JSON.stringify(FORM_2MS)};
                    return state;
                })()`, 'reveal');
                if (!revealed.ok) throw new Error('second miss did not reveal: ' + JSON.stringify(revealed));
                if (!retry.ok) throw new Error('first miss did not ask to try again');
                sawReveal = true;
            } else {
                throw new Error('unexpected drill cell: ' + JSON.stringify(now));
            }
            await cdp.evaluate(`(() => { document.querySelector('[data-drill-next]').click(); return true; })()`);
            await sleep(50);
        }
        if (!sawPass || !sawRetry || !sawReveal) {
            throw new Error('drill path incomplete: ' + JSON.stringify({ sawPass, sawRetry, sawReveal }));
        }

        await sleep(800);
        const events = db.all(
            `SELECT kind, word, lemma, result, activity FROM practice_events
             WHERE user_id = ? ORDER BY at, id`,
            userId
        );
        const reviewsAfter = db.all('SELECT word FROM review_items WHERE user_id = ?', userId);
        if (events.length !== 3) throw new Error('expected 3 form events, got ' + JSON.stringify(events));
        events.forEach((row) => {
            if (row.kind !== 'form') throw new Error('event is not a form: ' + JSON.stringify(row));
            if (row.lemma !== LEMMA) throw new Error('lemma mismatch: ' + JSON.stringify(row));
            if (row.activity !== 'conjugation-drill') throw new Error('activity mismatch: ' + JSON.stringify(row));
            if (row.word !== FORM_1S && row.word !== FORM_2MS) {
                throw new Error('drilled an unexpected form: ' + JSON.stringify(row));
            }
        });
        const pass = events.filter((row) => row.word === FORM_1S && row.result === 'pass');
        const misses = events.filter((row) => row.word === FORM_2MS && row.result === 'miss');
        if (pass.length !== 1 || misses.length !== 2) {
            throw new Error('signal results: ' + JSON.stringify(events));
        }
        if (reviewsAfter.length !== reviewsBefore.length) {
            throw new Error('form signals created review rows: ' + JSON.stringify(reviewsAfter));
        }
        const signalPosts = requests.filter((req) => req.url.indexOf('/api/review/signals') >= 0);
        signalPosts.forEach((req) => {
            if (req.postData.indexOf(WRONG) >= 0 || req.postData.indexOf(UNPOINTED) >= 0) {
                throw new Error('typed text was posted: ' + req.postData);
            }
        });

        const mark = requests.length;
        await cdp.evaluate(`(() => {
            var input = document.querySelector('[data-personal-input]');
            input.value = ${JSON.stringify(SECRET)};
            document.querySelector('[data-say-aloud]').click();
            document.querySelector('[data-english-toggle]').click();
            document.querySelector('[data-personal-done]').click();
            return input.value;
        })()`);
        await sleep(1200);
        const kept = await cdp.evaluate(`document.querySelector('[data-personal-input]').value`);
        if (kept !== SECRET) throw new Error('typed answer did not stay in the page');
        const added = requests.slice(mark).filter((req) => req.url.indexOf('/api/') >= 0);
        added.forEach((req) => {
            if (req.url.indexOf(SECRET) >= 0 || req.postData.indexOf(SECRET) >= 0) {
                throw new Error('personal answer was sent: ' + req.method + ' ' + req.url);
            }
        });
        const puts = added.filter((req) => req.method === 'PUT' && req.url.indexOf('/progress') >= 0);
        if (puts.length !== 1) throw new Error('expected one lesson-blob PUT, got ' + JSON.stringify(added));
        const body = JSON.parse(puts[0].postData);
        const flag = body && body.state && body.state.activityState
            && body.state.activityState['personal-question']
            && body.state.activityState['personal-question'].you;
        if (flag !== true) throw new Error('done mark is not a boolean: ' + puts[0].postData);
        const others = added.filter((req) => puts.indexOf(req) < 0);
        if (others.length) throw new Error('personal question called more than the blob: ' + JSON.stringify(others));

        const saved = db.get(
            'SELECT state_json FROM lesson_progress WHERE user_id = ? AND slug = ?',
            userId,
            LESSON
        );
        const stored = JSON.parse(saved.state_json);
        if (stored.activityState['personal-question'].you !== true) {
            throw new Error('stored blob is not the boolean: ' + saved.state_json);
        }
        if (saved.state_json.indexOf(SECRET) >= 0) throw new Error('blob stored the typed answer');

        if (cdp.errors.length) throw new Error('page errors: ' + cdp.errors.join(' | '));
        console.log('chrome-verb-workshop-smoke: PASS');
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
