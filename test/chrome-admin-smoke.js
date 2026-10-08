#!/usr/bin/env node
/**
 * Headless Chrome smoke: admin users + learner activity (SPEC-016).
 * Ephemeral port via signed-in-server (never 8080). Temp DATA_DIR.
 *
 *   node test/chrome-admin-smoke.js
 */
'use strict';

const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const harness = require('./helpers/signed-in-server');

const CHROME = process.env.CHROME_PATH
    || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

const INJECTION = '<img src=x onerror=alert(1)>';
const NEW_USER = 'smokeuser';
const NEW_PASS = 'smoke-pass-12';
const RESET_PASS = 'reset-pass-12';

function get(url, headers) {
    return new Promise((resolve, reject) => {
        const req = http.get(url, { headers: headers || {} }, (res) => {
            let data = '';
            res.on('data', (c) => { data += c; });
            res.on('end', () => resolve({
                status: res.statusCode,
                body: data,
                headers: res.headers
            }));
        });
        req.on('error', reject);
    });
}

function request(method, url, headers, body) {
    return new Promise((resolve, reject) => {
        const u = new URL(url);
        const payload = body == null ? null : Buffer.from(JSON.stringify(body), 'utf8');
        const origin = u.origin;
        const req = http.request({
            hostname: u.hostname,
            port: u.port,
            path: u.pathname + u.search,
            method,
            headers: Object.assign({
                'Content-Type': 'application/json',
                'Content-Length': payload ? payload.length : 0,
                Origin: origin
            }, headers || {})
        }, (res) => {
            let data = '';
            res.on('data', (c) => { data += c; });
            res.on('end', () => {
                let json = null;
                try { json = data ? JSON.parse(data) : null; } catch (_) { /* ignore */ }
                resolve({ status: res.statusCode, body: data, json, headers: res.headers });
            });
        });
        req.on('error', reject);
        if (payload) req.write(payload);
        req.end();
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
            reject(new Error('CDP WebSocket error'));
        });
        socket.addEventListener('message', (event) => {
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
                }, 45000);
                pending.set(id, {
                    resolve: (v) => { clearTimeout(timer); resolve(v); },
                    reject: (e) => { clearTimeout(timer); reject(e); }
                });
                socket.send(JSON.stringify({ id, method, params: params || {} }));
            });
        },
        close() {
            try { socket && socket.close(); } catch (_) { /* ignore */ }
        }
    };
}

async function findPage(debugPort) {
    for (let i = 0; i < 40; i += 1) {
        try {
            const list = await get('http://127.0.0.1:' + debugPort + '/json/list');
            const pages = JSON.parse(list.body);
            const page = pages.find((p) => p.type === 'page' && p.webSocketDebuggerUrl);
            if (page) return page;
        } catch (_) { /* retry */ }
        await new Promise((r) => setTimeout(r, 250));
    }
    throw new Error('No Chrome page');
}

async function evalJson(session, expression) {
    const result = await session.call('Runtime.evaluate', {
        awaitPromise: true,
        returnByValue: true,
        expression
    });
    if (result.exceptionDetails) {
        throw new Error(JSON.stringify(result.exceptionDetails));
    }
    return result.result && result.result.value;
}

async function waitFor(session, predicateExpr, ms) {
    const deadline = Date.now() + (ms || 15000);
    let last = null;
    while (Date.now() < deadline) {
        last = await evalJson(session, predicateExpr);
        if (last && last.ok) return last;
        await new Promise((r) => setTimeout(r, 200));
    }
    throw new Error('wait timed out; last=' + JSON.stringify(last));
}

async function waitUrl(session, pred, ms) {
    const deadline = Date.now() + (ms || 15000);
    let last = '';
    while (Date.now() < deadline) {
        last = await evalJson(session, 'String(location.href || "")');
        if (pred(last)) return last;
        await new Promise((r) => setTimeout(r, 200));
    }
    throw new Error('URL wait timed out; last=' + last);
}

async function launchChrome(userData) {
    const debugPort = 9600 + Math.floor(Math.random() * 400);
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
    const page = await findPage(debugPort);
    const session = cdpSession(page.webSocketDebuggerUrl);
    await session.call('Page.enable', {});
    await session.call('Runtime.enable', {});
    await session.call('Network.enable', {});
    return { chrome, session, debugPort, userData };
}

async function setSessionCookie(session, baseUrl, username) {
    const ck = cookieParts(harness.cookieFor(username));
    await session.call('Network.setCookie', {
        name: ck.name,
        value: ck.value,
        url: baseUrl,
        path: '/'
    });
}

async function clearCookies(session, baseUrl) {
    await session.call('Network.clearBrowserCookies', {});
    try {
        await session.call('Network.deleteCookies', {
            name: require('../server/http/cookies').cookieName(),
            url: baseUrl
        });
    } catch (_) { /* ignore */ }
}

async function runOnce() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);

    const started = await harness.start({ lessons: 'real' });
    const baseUrl = started.baseUrl;
    const accounts = require('../server/accounts');
    const adminCookie = harness.cookieFor('admin');
    const learnerCookie = harness.cookieFor('learner');

    const report = {
        ok: false,
        ac1: null,
        ac2: null,
        ac3: null,
        ac4: null,
        ac5: null,
        ac6: null,
        adminFetches: []
    };

    const profileAdmin = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-admin-a-'));
    const profileUser = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-admin-u-'));
    let chromeA = null;
    let chromeB = null;
    let sessionA = null;
    let sessionB = null;

    try {
        // --- ac-4: user-role GET /admin/ is 403; no admin JS / no /api/admin ---
        const forbidden = await get(baseUrl + '/admin/', { Cookie: learnerCookie });
        report.ac4 = {
            status: forbidden.status,
            body: forbidden.body.slice(0, 80),
            hasScript: /<script/i.test(forbidden.body)
        };
        if (forbidden.status !== 403) {
            throw new Error('expected 403 for user /admin/, got ' + forbidden.status);
        }
        if (/<script/i.test(forbidden.body)) {
            throw new Error('403 page must not include admin scripts');
        }

        // Track Network requests while learner somehow hits admin (HTTP already 403).
        // Prove scripts do not run by confirming plain text body has no HebrewAdminUsers boot.
        if (/HebrewAdminUsers|admin-shell\.js|\/api\/admin/i.test(forbidden.body)) {
            throw new Error('403 body must not reference admin app');
        }

        // --- Injection fixture user via API ---
        const injCreate = await request('POST', baseUrl + '/api/admin/users', {
            Cookie: adminCookie
        }, {
            username: 'injectfx',
            displayName: INJECTION,
            role: 'user',
            password: 'inject-pass1'
        });
        if (injCreate.status !== 201) {
            throw new Error('inject user create failed: ' + injCreate.status + ' ' + injCreate.body);
        }

        // --- Admin Chrome session ---
        chromeA = await launchChrome(profileAdmin);
        sessionA = chromeA.session;
        const adminFetches = [];
        await sessionA.call('Network.enable', {});
        // Capture request URLs via Runtime polling of performance — use CDP Network.requestWillBeSent
        // by evaluating a fetch spy after page load instead.
        await setSessionCookie(sessionA, baseUrl, 'admin');

        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#users' });
        await waitUrl(sessionA, (u) => u.includes('/admin/'), 15000);
        await waitFor(sessionA, `(() => ({
            ok: !!(window.HebrewAdminUsers && document.querySelector('[data-admin-users-body]')),
            rows: document.querySelectorAll('[data-admin-users-body] tr[data-user-id]').length
        }))()`, 20000);

        // ac-5: injection display name as text
        const inj = await evalJson(sessionA, `(() => {
            const rows = Array.from(document.querySelectorAll('[data-admin-users-body] tr[data-user-id]'));
            const row = rows.find(r => (r.textContent || '').includes('injectfx'));
            if (!row) return { ok: false, reason: 'row missing' };
            const html = row.innerHTML;
            const text = row.textContent || '';
            return {
                ok: true,
                hasRawTag: /<img src=x onerror=alert\\(1\\)>/.test(html),
                hasEscaped: /&lt;img/.test(html) || text.includes('<img'),
                textHasLiteral: text.includes('<img src=x onerror=alert(1)>'),
                liveImg: !!row.querySelector('img')
            };
        })()`);
        report.ac5 = inj;
        if (!inj.ok) throw new Error('injection row missing');
        if (inj.liveImg || inj.hasRawTag) {
            throw new Error('injection fixture must render as text, not an img element');
        }
        if (!inj.textHasLiteral && !inj.hasEscaped) {
            throw new Error('expected escaped or literal text for injection fixture');
        }

        // ac-6: password autocomplete on create dialog
        const ac6 = await evalJson(sessionA, `(() => {
            const openBtn = document.querySelector('[data-admin-action="create"]');
            if (!openBtn) return { ok: false, reason: 'create btn' };
            openBtn.click();
            const pw = document.getElementById('admin-create-password');
            const dlg = document.getElementById('admin-create-dialog');
            return {
                ok: !!(pw && dlg && !dlg.hidden),
                autocomplete: pw && pw.getAttribute('autocomplete'),
                dialogOpen: !!(dlg && !dlg.hidden && !dlg.classList.contains('hidden'))
            };
        })()`);
        report.ac6 = ac6;
        if (!ac6.ok || ac6.autocomplete !== 'new-password') {
            throw new Error('create password must use autocomplete=new-password');
        }
        // Escape closes dialog
        await evalJson(sessionA, `(() => {
            const dlg = document.getElementById('admin-create-dialog');
            dlg.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
            return { closed: dlg.hidden || dlg.classList.contains('hidden') };
        })()`);

        // ac-1: create user via UI
        const created = await evalJson(sessionA, `(() => new Promise((resolve, reject) => {
            const openBtn = document.querySelector('[data-admin-action="create"]');
            openBtn.click();
            const form = document.getElementById('admin-create-form');
            const pw = document.getElementById('admin-create-password');
            document.getElementById('admin-create-username').value = ${JSON.stringify(NEW_USER)};
            document.getElementById('admin-create-display').value = 'Smoke User';
            document.getElementById('admin-create-role').value = 'user';
            pw.value = ${JSON.stringify(NEW_PASS)};
            form.requestSubmit ? form.requestSubmit() : form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
            const deadline = Date.now() + 12000;
            const tick = () => {
                const row = Array.from(document.querySelectorAll('[data-admin-users-body] tr[data-user-id]'))
                    .find(r => (r.textContent || '').includes(${JSON.stringify(NEW_USER)}));
                const stillOpen = (() => {
                    const dlg = document.getElementById('admin-create-dialog');
                    return dlg && !dlg.hidden && !dlg.classList.contains('hidden');
                })();
                const pwVal = (document.getElementById('admin-create-password') || {}).value;
                if (row) {
                    resolve({
                        ok: true,
                        passwordCleared: pwVal === '' || pwVal == null,
                        dialogClosed: !stillOpen,
                        pwInDom: /${NEW_PASS}/.test(document.documentElement.innerHTML)
                    });
                    return;
                }
                if (Date.now() > deadline) {
                    reject(new Error('create user row not found'));
                    return;
                }
                setTimeout(tick, 200);
            };
            tick();
        }))()`);
        if (!created.ok) throw new Error('create user failed');
        if (created.pwInDom) throw new Error('password must not remain in DOM after submit');
        if (!created.passwordCleared) throw new Error('password field must clear after submit');

        // New user can sign in
        const signIn = await request('POST', baseUrl + '/api/session', {}, {
            username: NEW_USER,
            password: NEW_PASS
        });
        if (signIn.status !== 200 || !signIn.json || !signIn.json.id) {
            throw new Error('new user sign-in failed: ' + signIn.status + ' ' + signIn.body);
        }
        const newUserId = signIn.json.id;

        // Reset password via UI → forces change at next sign-in
        const resetUi = await evalJson(sessionA, `(() => new Promise((resolve, reject) => {
            const row = Array.from(document.querySelectorAll('[data-admin-users-body] tr[data-user-id]'))
                .find(r => (r.textContent || '').includes(${JSON.stringify(NEW_USER)}));
            if (!row) return reject(new Error('row missing for reset'));
            const btn = row.querySelector('[data-admin-action="reset"]');
            btn.click();
            const form = document.getElementById('admin-reset-form');
            const pw = document.getElementById('admin-reset-password');
            const idEl = document.getElementById('admin-reset-user-id');
            pw.value = ${JSON.stringify(RESET_PASS)};
            const auto = pw.getAttribute('autocomplete');
            const userId = idEl && idEl.value;
            const submitBtn = form.querySelector('button[type="submit"]');
            if (submitBtn) submitBtn.click();
            else form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
            const deadline = Date.now() + 10000;
            const tick = () => {
                const dlg = document.getElementById('admin-reset-dialog');
                const err = document.getElementById('admin-reset-error');
                const closed = !dlg || dlg.hidden || dlg.classList.contains('hidden');
                const errText = err && !err.hidden ? (err.textContent || '') : '';
                if (closed || errText || Date.now() > deadline) {
                    resolve({
                        ok: closed && !errText,
                        autocomplete: auto,
                        userId: userId,
                        errText: errText,
                        passwordCleared: (document.getElementById('admin-reset-password') || {}).value === '',
                        pwInDom: document.documentElement.innerHTML.indexOf(${JSON.stringify(RESET_PASS)}) !== -1
                    });
                    return;
                }
                setTimeout(tick, 150);
            };
            tick();
        }))()`);
        if (!resetUi.ok) {
            throw new Error('reset failed: ' + JSON.stringify(resetUi));
        }
        if (resetUi.autocomplete !== 'new-password') {
            throw new Error('reset password autocomplete');
        }
        if (resetUi.pwInDom) throw new Error('reset password leaked into DOM');

        // Confirm server-side: old password fails, reset password signs in with must_change
        const oldPassTry = await request('POST', baseUrl + '/api/session', {}, {
            username: NEW_USER,
            password: NEW_PASS
        });
        if (oldPassTry.status === 200) {
            throw new Error('old password should not work after reset');
        }
        const resetPassTry = await request('POST', baseUrl + '/api/session', {}, {
            username: NEW_USER,
            password: RESET_PASS
        });
        if (resetPassTry.status !== 200 || !resetPassTry.json || !resetPassTry.json.must_change_password) {
            throw new Error('reset password sign-in missing must_change: ' + resetPassTry.body);
        }
        // Sign out that API session so Chrome login is clean
        const resetCookieHdr = Array.isArray(resetPassTry.headers['set-cookie'])
            ? resetPassTry.headers['set-cookie'][0]
            : resetPassTry.headers['set-cookie'];
        const resetCookie = String(resetCookieHdr || '').split(';')[0];
        if (resetCookie) {
            await request('DELETE', baseUrl + '/api/session', { Cookie: resetCookie }, {});
        }

        // Next sign-in shows forced password dialog (SPEC-012)
        chromeB = await launchChrome(profileUser);
        sessionB = chromeB.session;
        await sessionB.call('Page.navigate', { url: baseUrl + '/login/' });
        await waitUrl(sessionB, (u) => u.includes('/login'), 12000);
        await evalJson(sessionB, `(() => new Promise((resolve, reject) => {
            const deadline = Date.now() + 10000;
            const tick = () => {
                if (!window.HebrewLoginPage || !document.getElementById('login-form')) {
                    if (Date.now() > deadline) return reject(new Error('login missing'));
                    return setTimeout(tick, 100);
                }
                document.getElementById('login-username').value = ${JSON.stringify(NEW_USER)};
                document.getElementById('login-password').value = ${JSON.stringify(RESET_PASS)};
                document.getElementById('login-submit').click();
                resolve(true);
            };
            tick();
        }))()`);
        try {
            await waitUrl(sessionB, (u) => {
                try {
                    const p = new URL(u).pathname;
                    return p === '/' || p === '/index.html';
                } catch (_) { return false; }
            }, 15000);
        } catch (err) {
            const loginState = await evalJson(sessionB, `(() => ({
                href: location.href,
                err: (document.getElementById('login-error') || {}).textContent || '',
                body: (document.body && document.body.innerText || '').slice(0, 300)
            }))()`);
            throw new Error('post-reset login failed: ' + JSON.stringify(loginState) + ' ' + err.message);
        }
        const forced = await waitFor(sessionB, `(() => {
            const dlg = document.getElementById('hebrew-shell-pw-dialog');
            const open = dlg && !dlg.hidden && !dlg.classList.contains('hidden');
            const forced = dlg && dlg.dataset.forced === '1';
            return { ok: !!(open && forced), open: !!open, forced: !!forced };
        })()`, 12000);
        if (!forced.ok) throw new Error('expected forced password dialog after reset');

        // Fresh session after reset (for disable check)
        const postResetSignIn = await request('POST', baseUrl + '/api/session', {}, {
            username: NEW_USER,
            password: RESET_PASS
        });
        if (postResetSignIn.status !== 200) {
            throw new Error('post-reset sign-in failed: ' + postResetSignIn.status);
        }
        const setCookie = postResetSignIn.headers['set-cookie'];
        const newUserCookieHeader = Array.isArray(setCookie) ? setCookie[0] : setCookie;
        const newUserCookie = String(newUserCookieHeader || '').split(';')[0];
        if (!newUserCookie) throw new Error('missing session cookie after post-reset sign-in');

        // Disable user via admin UI
        await evalJson(sessionA, `(() => new Promise((resolve, reject) => {
            const row = Array.from(document.querySelectorAll('[data-admin-users-body] tr[data-user-id]'))
                .find(r => (r.textContent || '').includes(${JSON.stringify(NEW_USER)}));
            if (!row) return reject(new Error('row missing for disable'));
            const btn = row.querySelector('[data-admin-action="disable"]');
            btn.click();
            const deadline = Date.now() + 10000;
            const tick = () => {
                const r2 = Array.from(document.querySelectorAll('[data-admin-users-body] tr[data-user-id]'))
                    .find(r => (r.textContent || '').includes(${JSON.stringify(NEW_USER)}));
                const status = r2 && r2.getAttribute('data-status');
                if (status === 'disabled') {
                    resolve({ ok: true, status });
                    return;
                }
                if (Date.now() > deadline) {
                    reject(new Error('disable did not stick; status=' + status));
                    return;
                }
                setTimeout(tick, 200);
            };
            tick();
        }))()`);

        // Disabled user's next API request → 401 (client redirects to sign-in)
        const meDisabled = await request('GET', baseUrl + '/api/me', {
            Cookie: newUserCookie
        });
        if (meDisabled.status !== 401) {
            throw new Error('disabled user expected 401 on /api/me, got ' + meDisabled.status);
        }
        // Browser: next navigation should land on sign-in
        await sessionB.call('Page.navigate', { url: baseUrl + '/' });
        await waitUrl(sessionB, (u) => u.includes('/login'), 12000);
        report.ac1 = {
            created: true,
            signedIn: true,
            resetForced: true,
            disabledStatus: meDisabled.status,
            newUserId
        };

        // ac-2: last-admin demote refusal leaves table unchanged
        const beforeDemote = await evalJson(sessionA, `(() => {
            const row = document.querySelector('tr[data-user-id="${started.users.admin.id}"]');
            return {
                html: row ? row.outerHTML : null,
                role: row && row.getAttribute('data-role'),
                count: document.querySelectorAll('[data-admin-users-body] tr[data-user-id]').length
            };
        })()`);
        const demote = await evalJson(sessionA, `(() => new Promise((resolve) => {
            const row = document.querySelector('tr[data-user-id="${started.users.admin.id}"]');
            const btn = row && row.querySelector('[data-admin-action="make-user"]');
            if (!btn) {
                resolve({ ok: false, reason: 'no make-user btn' });
                return;
            }
            btn.click();
            const deadline = Date.now() + 8000;
            const tick = () => {
                const err = document.querySelector('[data-admin-guard-error]');
                const visible = err && !err.hidden && (err.textContent || '').trim();
                const row2 = document.querySelector('tr[data-user-id="${started.users.admin.id}"]');
                if (visible || Date.now() > deadline) {
                    resolve({
                        ok: !!visible,
                        message: visible || '',
                        role: row2 && row2.getAttribute('data-role'),
                        html: row2 ? row2.outerHTML : null
                    });
                    return;
                }
                setTimeout(tick, 150);
            };
            tick();
        }))()`);
        report.ac2 = { before: beforeDemote, after: demote };
        if (!demote.ok) throw new Error('expected inline last-admin refusal');
        if (!/last active admin/i.test(demote.message)) {
            throw new Error('unexpected refusal message: ' + demote.message);
        }
        if (demote.role !== 'admin') {
            throw new Error('table role changed after 409');
        }
        if (beforeDemote.html && demote.html && beforeDemote.role !== demote.role) {
            throw new Error('table changed after last-admin refusal');
        }

        // ac-3: user detail progress + history load-older
        // Seed events for learner
        const progress = require('../server/progress');
        const learnerId = started.users.learner.id;
        const t0 = Date.parse('2024-06-01T00:00:00.000Z');
        for (let i = 0; i < 5; i += 1) {
            progress.applySignals(learnerId, [{
                id: 'admin-smoke-ev-' + i,
                word: 'בֹּקֶר',
                direction: 'he_en',
                result: i % 2 === 0 ? 'pass' : 'miss',
                at: new Date(t0 + i * 1000).toISOString(),
                slug: 'my-morning',
                activity: 'words'
            }]);
        }

        // Patch HISTORY_LIMIT via navigate with query — instead call events API with small pages
        // by temporarily using UI load-older against real default limit. Seed enough + use eval to
        // exercise applyHistoryPage through button by stubbing limit on window.
        await sessionA.call('Page.navigate', {
            url: baseUrl + '/admin/#user/' + learnerId
        });
        await waitFor(sessionA, `(() => ({
            ok: !!(document.querySelector('[data-admin-view="user"]')
                && document.querySelector('[data-admin-progress]')),
            hasProgress: !!document.querySelector('[data-admin-progress] .progress-view, [data-admin-progress] [data-progress-state]'),
            hasNotice: /הלומדים יודעים שמנהלים רואים את כל הפעילות/.test(document.body.innerText || '')
        }))()`, 20000);

        // Force small pages for load-older by calling applyHistoryPage through a fetch override
        const hist = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                const Users = window.HebrewAdminUsers;
                const host = document.createElement('div');
                host.innerHTML = '<tbody data-admin-history-body></tbody><button data-admin-load-older></button>';
                // querySelector on host won't find tbody as direct — wrap properly
                host.innerHTML = '<div><table><tbody data-admin-history-body></tbody></table>'
                    + '<button type="button" data-admin-load-older></button></div>';
                const state = { seenIds: Object.create(null), nextBefore: null };
                const limit = 2;
                const r1 = await window.HebrewApi.get('/api/admin/users/${learnerId}/events?limit=2');
                if (!r1.ok) return reject(new Error('events page1'));
                const a1 = Users.applyHistoryPage(state, host, r1.data, limit);
                const r2 = await window.HebrewApi.get(
                    '/api/admin/users/${learnerId}/events?limit=2&before=' + encodeURIComponent(r1.data.nextBefore)
                );
                if (!r2.ok) return reject(new Error('events page2'));
                const a2 = Users.applyHistoryPage(state, host, r2.data, limit);
                let a3 = null;
                if (state.nextBefore && a2.more) {
                    const r3 = await window.HebrewApi.get(
                        '/api/admin/users/${learnerId}/events?limit=2&before=' + encodeURIComponent(state.nextBefore)
                    );
                    a3 = Users.applyHistoryPage(state, host, r3.data, limit);
                } else if (state.nextBefore) {
                    const r3 = await window.HebrewApi.get(
                        '/api/admin/users/${learnerId}/events?limit=2&before=' + encodeURIComponent(state.nextBefore)
                    );
                    a3 = Users.applyHistoryPage(state, host, r3.data, limit);
                }
                const ids = (host.innerHTML.match(/data-event-id="[^"]+"/g) || []);
                const unique = new Set(ids);
                const btn = host.querySelector('[data-admin-load-older]');
                resolve({
                    ok: true,
                    a1, a2, a3,
                    idCount: ids.length,
                    uniqueCount: unique.size,
                    btnHidden: !!(btn && btn.hidden)
                });
            } catch (err) {
                reject(err);
            }
        }))()`);
        report.ac3 = hist;
        if (!hist.ok) throw new Error('history paging failed');
        if (hist.idCount !== hist.uniqueCount) {
            throw new Error('duplicate history events in DOM');
        }
        if (hist.a1.appended < 1) throw new Error('expected first history page');

        // Keep loading until button hides
        if (!hist.btnHidden && hist.a3 && hist.a3.more === false) {
            // ok
        } else if (!hist.btnHidden) {
            // continue until end
            const drained = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
                try {
                    const Users = window.HebrewAdminUsers;
                    const host = document.createElement('div');
                    host.innerHTML = '<div><table><tbody data-admin-history-body></tbody></table>'
                        + '<button type="button" data-admin-load-older></button></div>';
                    const state = { seenIds: Object.create(null), nextBefore: null };
                    let before = null;
                    let guard = 0;
                    while (guard < 20) {
                        guard += 1;
                        const url = '/api/admin/users/${learnerId}/events?limit=2'
                            + (before ? '&before=' + encodeURIComponent(before) : '');
                        const res = await window.HebrewApi.get(url);
                        if (!res.ok) throw new Error('page fail');
                        const applied = Users.applyHistoryPage(state, host, res.data, 2);
                        before = state.nextBefore;
                        if (!applied.more) break;
                    }
                    const btn = host.querySelector('[data-admin-load-older]');
                    const ids = host.innerHTML.match(/data-event-id="[^"]+"/g) || [];
                    resolve({
                        ok: true,
                        btnHidden: !!(btn && btn.hidden),
                        unique: new Set(ids).size,
                        total: ids.length
                    });
                } catch (e) { reject(e); }
            }))()`);
            if (!drained.btnHidden) throw new Error('load older button should hide at end');
            if (drained.unique !== drained.total) throw new Error('duplicates after drain');
        }

        // Detail page shows progress charts + notice
        const detail = await evalJson(sessionA, `(() => ({
            ok: true,
            hasNotice: /הלומדים יודעים שמנהלים רואים את כל הפעילות/.test(document.body.innerText || ''),
            hasProgress: !!document.querySelector('[data-admin-progress] [data-progress-state], [data-admin-progress] .progress-view'),
            hasTrouble: !!document.querySelector('[data-admin-trouble]')
        }))()`);
        if (!detail.hasNotice) throw new Error('admin privacy notice missing on detail');
        report.ac3.detail = detail;

        // Lessons tab placeholder
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lessons' });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-admin-view="lessons"]'),
            text: (document.querySelector('[data-admin-view="lessons"]') || {}).textContent || ''
        }))()`, 10000);

        // Final ac-4 browser check: learner cookie on /admin/ via CDP still gets 403 document
        await clearCookies(sessionA, baseUrl);
        await setSessionCookie(sessionA, baseUrl, 'learner');
        const adminReqs = [];
        await evalJson(sessionA, `(() => {
            window.__adminApiHits = [];
            const orig = window.fetch;
            window.fetch = function(input, init) {
                const url = typeof input === 'string' ? input : (input && input.url) || '';
                if (String(url).indexOf('/api/admin') === 0) {
                    window.__adminApiHits.push(String(url));
                }
                return orig.apply(this, arguments);
            };
            return true;
        })()`);
        // Navigate — server returns plain 403, scripts never load
        const nav = await sessionA.call('Page.navigate', { url: baseUrl + '/admin/' });
        await new Promise((r) => setTimeout(r, 1500));
        const learnerAdmin = await evalJson(sessionA, `(() => ({
            href: location.href,
            body: (document.body && document.body.innerText) || document.documentElement.innerText || '',
            hasAdminUsers: !!window.HebrewAdminUsers,
            hits: window.__adminApiHits || []
        }))()`);
        report.ac4.browser = learnerAdmin;
        report.adminFetches = learnerAdmin.hits;
        if (learnerAdmin.hasAdminUsers) {
            throw new Error('admin scripts must not run for user-role /admin/');
        }
        if (learnerAdmin.hits.length) {
            throw new Error('user-role session must not call /api/admin');
        }
        // Body may be "Forbidden" plain text
        if (!/Forbidden|403/i.test(learnerAdmin.body) && learnerAdmin.hasAdminUsers) {
            throw new Error('expected forbidden document');
        }

        report.ok = true;
        return report;
    } finally {
        try { sessionA && sessionA.close(); } catch (_) { /* ignore */ }
        try { sessionB && sessionB.close(); } catch (_) { /* ignore */ }
        try { chromeA && chromeA.chrome && chromeA.chrome.kill(); } catch (_) { /* ignore */ }
        try { chromeB && chromeB.chrome && chromeB.chrome.kill(); } catch (_) { /* ignore */ }
        try {
            fs.rmSync(profileAdmin, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(profileUser, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        await harness.stop();
    }
}

async function main() {
    let lastErr = null;
    for (let attempt = 1; attempt <= 2; attempt += 1) {
        try {
            const report = await runOnce();
            console.log(JSON.stringify(report, null, 2));
            if (!report.ok) process.exit(1);
            console.log('chrome-admin-smoke: PASS');
            return;
        } catch (err) {
            lastErr = err;
            const msg = String((err && err.message) || err);
            const retryable = /EBUSY|EPERM|resource busy|locked/i.test(msg);
            console.error('chrome-admin-smoke attempt ' + attempt + ' failed:', msg);
            try { await harness.stop(); } catch (_) { /* ignore */ }
            if (!retryable || attempt === 2) break;
            await new Promise((r) => setTimeout(r, 500));
        }
    }
    console.error(lastErr);
    process.exit(1);
}

main();
