#!/usr/bin/env node
/**
 * Chrome smoke: "What your AI sees" and the admin AI-planning switch.
 * Provenance: INIT-006/SPEC-017
 *
 * Run: node test/chrome-ai-profile-smoke.js
 * Needs Chrome at the default Windows path (or CHROME_PATH).
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');
const assert = require('node:assert/strict');
const harness = require('./helpers/signed-in-server');

const { CHROME } = require('./helpers/chrome-cdp');
const DISCLOSURE = 'If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator\'s AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.';
const ADMIN_SENTENCE = 'Admins can see all your activity, including every answer';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function api(base, method, apiPath, username, body) {
    const headers = { Origin: base };
    if (username) headers.Cookie = harness.cookieFor(username);
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
            throw new Error((details.text || 'evaluate failed') + ' in: ' + expression.slice(0, 180));
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

async function setCookie(cdp, base, username) {
    const name = require('../server/http/cookies').cookieName();
    await cdp.call('Network.clearBrowserCookies');
    try {
        await cdp.call('Network.deleteCookies', { name, url: base });
    } catch (err) { /* already gone */ }
    if (!username) {
        await cdp.call('Network.setExtraHTTPHeaders', { headers: {} });
        return;
    }
    const header = harness.cookieFor(username);
    const eq = header.indexOf('=');
    await cdp.call('Network.setCookie', {
        name: header.slice(0, eq),
        value: decodeURIComponent(header.slice(eq + 1)),
        url: base,
        path: '/'
    });
    await cdp.call('Network.setExtraHTTPHeaders', { headers: { Cookie: header } });
}

function pretty(text) {
    return JSON.stringify(JSON.parse(text), null, 2);
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'fixture' });
    const base = started.baseUrl;
    const learnerId = String(started.users.learner.id);

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-ai-profile-'));
    const port = 9400 + Math.floor(Math.random() * 400);
    const chrome = spawn(CHROME, [
        '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
        '--user-data-dir=' + userData, '--remote-debugging-port=' + port,
        '--remote-allow-origins=*', '--window-size=1280,900', 'about:blank'
    ], { stdio: 'ignore' });

    const report = { ok: false };
    try {
        const profile = await api(base, 'GET', '/api/me/profile', 'learner');
        if (profile.status !== 200) throw new Error('profile GET failed: ' + profile.status + ' ' + profile.text);
        const denied = await api(base, 'PATCH', '/api/admin/users/' + learnerId + '/ai-planning', 'learner', {
            enabled: true
        });
        if (denied.status !== 403) throw new Error('learner PATCH should be 403, got ' + denied.status);

        const cdp = await connect(port);
        await cdp.call('Network.enable');
        await cdp.call('Network.setCacheDisabled', { cacheDisabled: true });
        await cdp.call('Runtime.enable');
        await cdp.call('Page.enable');
        await setCookie(cdp, base, 'learner');
        await cdp.call('Page.navigate', { url: base + '/progress/' });

        await waitFor(cdp, `(() => {
            var ready = document.getElementById('app-main')
                && document.getElementById('app-main').getAttribute('data-route-ready') === 'progress';
            var wire = document.querySelector('[data-ai-profile-wire]');
            return { ok: !!(ready && wire && wire.textContent), ready: !!ready };
        })()`, 'progress profile');

        const raw = await cdp.evaluate(`(() => {
            var wire = document.querySelector('[data-ai-profile-wire]');
            var shown = document.querySelector('[data-ai-profile-shown]');
            var note = document.querySelector('[data-ai-profile] [data-ai-disclosure]');
            var flag = document.querySelector('[data-ai-planning]');
            var toggle = document.querySelector('[data-ai-planning-toggle]');
            return {
                wire: wire ? wire.textContent : '',
                shown: shown ? shown.textContent : '',
                hidden: shown ? shown.hasAttribute('hidden') : null,
                disclosure: note ? note.textContent : '',
                planning: flag ? flag.getAttribute('data-ai-planning') : '',
                toggle: !!toggle
            };
        })()`);
        assert.equal(raw.wire, profile.text);
        assert.equal(raw.shown, pretty(profile.text));
        assert.equal(JSON.stringify(JSON.parse(raw.shown)), JSON.stringify(JSON.parse(profile.text)));
        assert.equal(raw.hidden, true);
        assert.equal(raw.toggle, false);
        assert.equal(raw.planning, profile.json.ai_planning ? 'on' : 'off');
        assert.ok(raw.disclosure.includes(DISCLOSURE));

        const opened = await cdp.evaluate(`(() => {
            var btn = document.querySelector('[data-ai-profile-raw-toggle]');
            btn.click();
            var shown = document.querySelector('[data-ai-profile-shown]');
            return {
                ok: shown && !shown.hasAttribute('hidden') && btn.getAttribute('aria-expanded') === 'true',
                text: shown ? shown.textContent : ''
            };
        })()`);
        assert.equal(opened.text, pretty(profile.text));

        await cdp.evaluate(`(() => {
            document.getElementById('hebrew-shell-account-btn').click();
            return true;
        })()`);
        const menu = await waitFor(cdp, `(() => {
            var notes = document.querySelectorAll('[role="note"]');
            var texts = [];
            for (var i = 0; i < notes.length; i += 1) texts.push(notes[i].textContent || '');
            var joined = texts.join('\\n');
            return {
                ok: joined.indexOf(${JSON.stringify(DISCLOSURE)}) >= 0
                    && joined.indexOf(${JSON.stringify(ADMIN_SENTENCE)}) >= 0,
                joined: joined
            };
        })()`, 'account menu notes');
        report.ac2Menu = menu.ok;

        const nullWord = '<img src=x onerror=alert(1)>';
        const fixture = {
            username: nullWord,
            profile_version: 1,
            ai_planning: false,
            journey: {
                stage: 'script',
                counts: {
                    he_en: { new: 1, learning: 0, longTerm: 0 },
                    en_he: { new: 0, learning: 0, longTerm: 0 }
                },
                lessons_completed: [],
                recipes_completed: {},
                tenses_practiced: [],
                binyanim_practiced: [],
                vowels_off_share: null
            },
            words_total: 1,
            words: [{
                word_id: nullWord,
                he_en: { stage: 'learning', strength: null },
                lapses: 0
            }],
            weak_words: [nullWord],
            very_weak_words: [],
            grammar: [
                { concept: 'binyan:paal', status: 'unseen', attempts: 0 },
                { concept: 'root-class:regular', status: 'ok', attempts: 5, accuracy: 0.8 },
                { concept: 'root-class:lamed-he', status: 'weak', attempts: 5, accuracy: 0.6 },
                { concept: 'root-class:peh-yod', status: 'very_weak', attempts: 5, accuracy: 0.2 }
            ]
        };
        const fixtureRaw = JSON.stringify(fixture);
        const nullCheck = await cdp.evaluate(`(() => {
            var host = document.createElement('div');
            var raw = ${JSON.stringify(fixtureRaw)};
            var profile = JSON.parse(raw);
            HebrewProgressView.renderAiProfile(host, profile, raw, '');
            var strength = host.querySelector('[data-strength="null"]');
            var link = host.querySelector('[data-ai-word]');
            var green = host.querySelector('[data-colour="green"]');
            var yellow = host.querySelector('[data-colour="yellow"]');
            var red = host.querySelector('[data-colour="red"]');
            var families = ['binyan', 'tense', 'cell', 'root-class'].every(function (name) {
                return !!host.querySelector('[data-grammar-family="' + name + '"]');
            });
            return {
                ok: !!(strength && link && green && yellow && red && families && !host.querySelector('img')),
                strength: strength ? strength.getAttribute('data-strength') : '',
                strengthText: strength ? strength.textContent : '',
                href: link ? link.getAttribute('href') : '',
                img: !!host.querySelector('img'),
                wire: (host.querySelector('[data-ai-profile-wire]') || {}).textContent || ''
            };
        })()`);
        assert.equal(nullCheck.strength, 'null');
        assert.equal(nullCheck.strengthText.includes('null'), false);
        assert.equal(nullCheck.img, false);
        assert.equal(nullCheck.wire, fixtureRaw);
        assert.ok(nullCheck.href.indexOf('/words/?weak=weak&q=') === 0);
        assert.equal(nullCheck.ok, true);
        report.nullStrength = true;

        await cdp.call('Page.navigate', { url: base + '/admin/' });
        const blocked = await waitFor(cdp, `(() => {
            var text = document.body ? document.body.textContent : '';
            var toggle = document.querySelector('[data-ai-planning-toggle]');
            var refused = text.indexOf('Forbidden') >= 0 || text.indexOf('This page is for admins only') >= 0;
            return {
                ok: !toggle && refused,
                toggle: !!toggle,
                href: String(location.href || '')
            };
        })()`, 'non-admin admin page');
        report.ac3 = blocked;

        await setCookie(cdp, base, 'admin');
        await cdp.call('Page.navigate', { url: base + '/admin/?n=1#users' });
        const row = await waitFor(cdp, `(() => {
            var btn = document.querySelector('[data-ai-planning-toggle][data-user-id="${learnerId}"]');
            var text = document.body ? document.body.textContent : '';
            return {
                ok: !!(btn && btn.getAttribute('aria-pressed') === 'false'),
                pressed: btn ? btn.getAttribute('aria-pressed') : null,
                href: String(location.href || ''),
                ready: document.getElementById('app-main')
                    ? document.getElementById('app-main').getAttribute('data-route-ready')
                    : '',
                text: text.slice(0, 240)
            };
        })()`, 'admin AI planning toggle', 25000);
        report.toggleOff = row.pressed;

        await cdp.evaluate(`(() => {
            document.querySelector('[data-ai-planning-toggle][data-user-id="${learnerId}"]').click();
            return true;
        })()`);
        const dialog = await waitFor(cdp, `(() => {
            var dlg = document.getElementById('admin-ai-planning-dialog');
            var open = dlg && !dlg.hasAttribute('hidden');
            var text = dlg ? dlg.textContent : '';
            var pressed = document.querySelector('[data-ai-planning-toggle][data-user-id="${learnerId}"]');
            return {
                ok: !!(open && text.indexOf(${JSON.stringify(DISCLOSURE)}) >= 0
                    && pressed && pressed.getAttribute('aria-pressed') === 'false'),
                pressed: pressed ? pressed.getAttribute('aria-pressed') : null
            };
        })()`, 'confirm dialog');
        report.confirmOpen = dialog.pressed;

        await cdp.evaluate(`(() => {
            var dlg = document.getElementById('admin-ai-planning-dialog');
            dlg.querySelector('[data-admin-dialog-close]').click();
            return true;
        })()`);
        const cancelled = await waitFor(cdp, `(() => {
            var dlg = document.getElementById('admin-ai-planning-dialog');
            var pressed = document.querySelector('[data-ai-planning-toggle][data-user-id="${learnerId}"]');
            return {
                ok: !!(dlg && dlg.hasAttribute('hidden') && pressed && pressed.getAttribute('aria-pressed') === 'false')
            };
        })()`, 'cancel leaves the switch off');

        const stillOff = await api(base, 'GET', '/api/admin/users', 'admin');
        const beforeRow = (stillOff.json.users || []).find((user) => String(user.id) === learnerId);
        if (!beforeRow || beforeRow.ai_planning !== false) {
            throw new Error('cancel changed AI planning: ' + JSON.stringify(beforeRow));
        }

        await cdp.evaluate(`(() => {
            document.querySelector('[data-ai-planning-toggle][data-user-id="${learnerId}"]').click();
            return true;
        })()`);
        await waitFor(cdp, `(() => {
            var dlg = document.getElementById('admin-ai-planning-dialog');
            return { ok: !!(dlg && !dlg.hasAttribute('hidden')) };
        })()`, 'confirm dialog again');
        await cdp.evaluate(`(() => {
            document.querySelector('#admin-ai-planning-form button[type="submit"]').click();
            return true;
        })()`);
        const turnedOn = await waitFor(cdp, `(() => {
            var pressed = document.querySelector('[data-ai-planning-toggle][data-user-id="${learnerId}"]');
            var dlg = document.getElementById('admin-ai-planning-dialog');
            return {
                ok: !!(pressed && pressed.getAttribute('aria-pressed') === 'true'
                    && dlg && dlg.hasAttribute('hidden')),
                pressed: pressed ? pressed.getAttribute('aria-pressed') : null
            };
        })()`, 'switch on after 200');

        const after = await api(base, 'GET', '/api/admin/users', 'admin');
        const afterRow = (after.json.users || []).find((user) => String(user.id) === learnerId);
        if (!afterRow || afterRow.ai_planning !== true) {
            throw new Error('toggle did not persist: ' + JSON.stringify(afterRow));
        }
        report.ac4 = { pressed: turnedOn.pressed, cancelled: cancelled.ok };

        await setCookie(cdp, base, null);
        await cdp.call('Page.navigate', { url: base + '/login/' });
        const login = await waitFor(cdp, `(() => {
            var text = document.body ? document.body.textContent : '';
            return {
                ok: text.indexOf(${JSON.stringify(DISCLOSURE)}) >= 0
                    && text.indexOf(${JSON.stringify(ADMIN_SENTENCE)}) >= 0
            };
        })()`, 'sign-in disclosure');
        report.login = login.ok;

        if (cdp.errors.length) throw new Error('page errors: ' + cdp.errors.join(' | '));
        report.ok = true;
        report.ac1 = true;
        console.log(JSON.stringify(report, null, 2));
        console.log('chrome-ai-profile-smoke: PASS');
        cdp.ws.close();
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-ai-profile-smoke") || main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
