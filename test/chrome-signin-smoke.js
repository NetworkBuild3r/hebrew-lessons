#!/usr/bin/env node
/**
 * Headless Chrome smoke: sign-in, hostile next, nav shell, forced password.
 * Provenance: INIT-002/SPEC-012
 *
 * Boots an ephemeral server (never 8080) with a temp DATA_DIR.
 *
 *   node test/chrome-signin-smoke.js
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

function get(url, headers) {
    return new Promise((resolve, reject) => {
        const req = http.get(url, { headers: headers || {} }, (res) => {
            let data = '';
            res.on('data', (c) => { data += c; });
            res.on('end', () => resolve({
                status: res.statusCode,
                body: data,
                headers: res.headers,
                url: res.url
            }));
        });
        req.on('error', reject);
    });
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
                }, 30000);
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

async function currentHref(session) {
    const result = await session.call('Runtime.evaluate', {
        returnByValue: true,
        expression: 'String(location.href || "")'
    });
    return (result.result && result.result.value) || '';
}

async function waitUrl(session, pred, ms) {
    const deadline = Date.now() + (ms || 15000);
    let last = '';
    while (Date.now() < deadline) {
        last = await currentHref(session);
        if (pred(last)) return last;
        await new Promise((r) => setTimeout(r, 200));
    }
    throw new Error('URL wait timed out; last=' + last);
}

/** After a full page load into the app shell, wait until the first screen has mounted. */
async function waitAppReady(session, ms) {
    const deadline = Date.now() + (ms || 15000);
    while (Date.now() < deadline) {
        const ready = await evalJson(session,
            `(() => { const m = document.getElementById('app-main'); return !!(m && m.getAttribute('data-route-ready')); })()`)
            .catch(() => false);
        if (ready) return;
        await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error('app shell did not become ready');
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

async function runOnce() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);

    const started = await harness.start({ lessons: 'real' });
    const baseUrl = started.baseUrl;
    const learnerPass = harness.users.learner.password;
    const accounts = require('../server/accounts');

    // Extra user for forced password flow
    const forced = accounts.createUser({
        username: 'mustchange',
        password: 'temp-password1',
        role: 'user',
        displayName: 'Must Change',
        actor: 'test'
    });
    accounts.resetPassword(forced.id, 'reset-password1', 'admin');

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-signin-'));
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

    const report = {
        ok: false,
        signedOutRedirect: null,
        afterSignIn: null,
        hostileNext: null,
        nav: {},
        adminLinkForUser: null,
        forcedDialog: null
    };

    try {
        const page = await findPage(debugPort);
        const session = cdpSession(page.webSocketDebuggerUrl);
        await session.call('Page.enable', {});
        await session.call('Runtime.enable', {});
        await session.call('Network.enable', {});

        // ac-1: signed-out / → /login/?next=/
        await session.call('Page.navigate', { url: baseUrl + '/' });
        const loginUrl = await waitUrl(
            session,
            (u) => u.includes('/login/') && u.includes('next='),
            12000
        );
        report.signedOutRedirect = loginUrl;
        if (!/next=%2F(?:$|&)|next=\/(?:$|&)/.test(loginUrl) && !loginUrl.includes('next=/')) {
            // Accept encoded or raw next=/
            if (!decodeURIComponent(loginUrl).includes('next=/')) {
                throw new Error('expected next=/ in redirect, got ' + loginUrl);
            }
        }

        // Sign in as learner (login page is self-contained; /shared/* gated when signed out)
        await evalJson(session, `(() => new Promise((resolve, reject) => {
            const deadline = Date.now() + 10000;
            const tick = () => {
                if (!window.HebrewLoginPage || !document.getElementById('login-form')) {
                    if (Date.now() > deadline) return reject(new Error('login page missing'));
                    return setTimeout(tick, 100);
                }
                document.getElementById('login-username').value = 'learner';
                document.getElementById('login-password').value = ${JSON.stringify(learnerPass)};
                document.getElementById('login-submit').click();
                resolve(true);
            };
            tick();
        }))()`);

        const homeUrl = await waitUrl(session, (u) => {
            try {
                const parsed = new URL(u);
                return parsed.pathname === '/' || parsed.pathname === '/index.html';
            } catch (_) {
                return false;
            }
        }, 15000);
        report.afterSignIn = homeUrl;
        await waitAppReady(session);

        // Shell on Today
        const todayShell = await evalJson(session, `(() => {
            const shell = document.querySelector('.hebrew-shell');
            const admin = document.querySelector('[data-nav="admin"]');
            const current = document.querySelector('.hebrew-shell-link[aria-current="page"]');
            const privacy = document.querySelector('.hebrew-shell-privacy');
            return {
                hasShell: !!shell,
                hasAdmin: !!admin,
                currentNav: current && current.getAttribute('data-nav'),
                privacy: privacy && privacy.textContent || ''
            };
        })()`);
        report.nav.today = todayShell;
        if (!todayShell.hasShell) throw new Error('shell missing on Today');
        if (todayShell.hasAdmin) throw new Error('user role must not see Admin link');
        report.adminLinkForUser = false;
        if (!/מנהלים יכולים לראות את כל הפעילות שלך/.test(todayShell.privacy)) {
            throw new Error('privacy note missing in account menu');
        }

        // Review page
        await session.call('Page.navigate', { url: baseUrl + '/review/' });
        await waitUrl(session, (u) => u.includes('/review'), 12000);
        await waitAppReady(session);
        const reviewShell = await evalJson(session, `(() => {
            const shell = document.querySelector('.hebrew-shell');
            const current = document.querySelector('.hebrew-shell-link[aria-current="page"]');
            return {
                hasShell: !!shell,
                currentNav: current && current.getAttribute('data-nav')
            };
        })()`);
        report.nav.review = reviewShell;
        if (!reviewShell.hasShell) throw new Error('shell missing on Review');

        // Progress page (SPEC-015): new account → empty state
        await session.call('Page.navigate', { url: baseUrl + '/progress/' });
        await waitUrl(session, (u) => u.includes('/progress'), 12000);
        await new Promise((r) => setTimeout(r, 1000));
        const progressEmpty = await evalJson(session, `(() => new Promise((resolve) => {
            const deadline = Date.now() + 8000;
            const tick = () => {
                const root = document.getElementById('progress-app');
                const empty = root && root.querySelector('[data-progress-state="empty"]');
                const shell = document.querySelector('.hebrew-shell');
                const current = document.querySelector('.hebrew-shell-link[aria-current="page"]');
                const text = (root && root.innerText) || '';
                if (empty || Date.now() > deadline) {
                    resolve({
                        hasShell: !!shell,
                        currentNav: current && current.getAttribute('data-nav'),
                        empty: !!empty,
                        hasTodayLink: !!(root && root.querySelector('a[href="/"]')),
                        hasLessonsLink: !!(root && root.querySelector('a[href="/lessons/"]')),
                        text: text.slice(0, 400)
                    });
                    return;
                }
                setTimeout(tick, 150);
            };
            tick();
        }))()`);
        report.nav.progress = progressEmpty;
        if (!progressEmpty.hasShell) throw new Error('shell missing on Progress');
        if (progressEmpty.currentNav !== 'progress') {
            throw new Error('expected progress nav current, got ' + progressEmpty.currentNav);
        }
        if (!progressEmpty.empty) {
            throw new Error('expected empty progress state for new account; got ' + progressEmpty.text);
        }
        if (!progressEmpty.hasTodayLink || !progressEmpty.hasLessonsLink) {
            throw new Error('empty state must link to Today and Lessons');
        }

        // Seed metrics in temp DB, then expect a non-empty progress view
        const reviewRepo = require('../server/db/repos/review');
        const eventsRepo = require('../server/db/repos/events');
        const learnerId = started.users.learner.id;
        const nowIso = new Date().toISOString();
        reviewRepo.upsert({
            user_id: learnerId,
            word: 'בֹּקֶר',
            direction: 'he_en',
            due: nowIso,
            step: 1,
            streak: 1,
            lapses: 0,
            last_result: 'pass',
            last_at: nowIso,
            sources: JSON.stringify(['my-morning'])
        });
        reviewRepo.upsert({
            user_id: learnerId,
            word: 'קָם',
            direction: 'he_en',
            due: nowIso,
            step: 0,
            streak: 0,
            lapses: 0,
            last_result: null,
            last_at: null,
            sources: JSON.stringify(['my-morning'])
        });
        for (let i = 0; i < 10; i += 1) {
            eventsRepo.insert({
                id: 'signin-smoke-pass-' + i,
                user_id: learnerId,
                word: 'בֹּקֶר',
                direction: 'he_en',
                result: i < 8 ? 'pass' : 'miss',
                at: nowIso,
                slug: 'my-morning',
                activity: 'words'
            });
        }

        await session.call('Page.navigate', { url: baseUrl + '/progress/' });
        await waitUrl(session, (u) => u.includes('/progress'), 12000);
        await new Promise((r) => setTimeout(r, 1000));
        const progressReady = await evalJson(session, `(() => new Promise((resolve) => {
            const deadline = Date.now() + 8000;
            const tick = () => {
                const root = document.getElementById('progress-app');
                const ready = root && (
                    root.querySelector('[data-progress-state="ready"]')
                    || root.querySelector('[data-progress-state="low-data"]')
                );
                const empty = root && root.querySelector('[data-progress-state="empty"]');
                const text = (root && root.innerText) || '';
                if (ready || empty || Date.now() > deadline) {
                    resolve({
                        ready: !!ready,
                        empty: !!empty,
                        hasStageCounts: !!(root && root.querySelector('.progress-stage-counts')),
                        text: text.slice(0, 400)
                    });
                    return;
                }
                setTimeout(tick, 150);
            };
            tick();
        }))()`);
        report.nav.progressReady = progressReady;
        if (!progressReady.ready || progressReady.empty) {
            throw new Error('expected ready/low-data progress after seed; got ' + progressReady.text);
        }
        if (!progressReady.hasStageCounts) {
            throw new Error('expected stage count text after seed');
        }

        // Real lesson page
        await session.call('Page.navigate', {
            url: baseUrl + '/worksheets/my-morning/index.html'
        });
        await waitUrl(session, (u) => u.includes('/worksheets/my-morning'), 12000);
        await new Promise((r) => setTimeout(r, 2000));
        const lessonShell = await evalJson(session, `(() => new Promise((resolve) => {
            const deadline = Date.now() + 8000;
            const tick = () => {
                const shell = document.querySelector('.hebrew-shell');
                const homeBtn = document.querySelector('.player-top .back-link');
                const current = document.querySelector('.hebrew-shell-link[aria-current="page"]');
                if (shell || Date.now() > deadline) {
                    resolve({
                        hasShell: !!shell,
                        hasPlayerHome: !!homeBtn,
                        currentNav: current && current.getAttribute('data-nav'),
                        worksheetScript: !!document.querySelector('script[src*="worksheet.js"]')
                            || /worksheet\\.js/.test(document.documentElement.innerHTML)
                    });
                    return;
                }
                setTimeout(tick, 150);
            };
            tick();
        }))()`);
        report.nav.lesson = lessonShell;
        if (!lessonShell.hasShell) throw new Error('shell missing on lesson page');
        if (lessonShell.hasPlayerHome) throw new Error('player Home button should be removed');

        // Hostile next
        await session.call('Page.navigate', { url: baseUrl + '/login/?next=' + encodeURIComponent('//evil.example') });
        await waitUrl(session, (u) => u.includes('/login/'), 12000);
        await evalJson(session, `(() => new Promise((resolve, reject) => {
            const deadline = Date.now() + 8000;
            const tick = () => {
                if (!window.HebrewLoginPage || !document.getElementById('login-form')) {
                    if (Date.now() > deadline) return reject(new Error('login page missing'));
                    return setTimeout(tick, 100);
                }
                document.getElementById('login-username').value = 'learner';
                document.getElementById('login-password').value = ${JSON.stringify(learnerPass)};
                document.getElementById('login-submit').click();
                resolve(true);
            };
            tick();
        }))()`);
        const afterHostile = await waitUrl(session, (u) => {
            try {
                const parsed = new URL(u);
                return parsed.pathname === '/' || parsed.pathname === '/index.html';
            } catch (_) {
                return false;
            }
        }, 15000);
        if (/evil\.example/i.test(afterHostile)) {
            throw new Error('hostile next leaked: ' + afterHostile);
        }
        report.hostileNext = afterHostile;

        // Forced password change
        await session.call('Network.clearBrowserCookies', {});
        await session.call('Page.navigate', { url: baseUrl + '/login/?next=/' });
        await waitUrl(session, (u) => u.includes('/login/'), 12000);
        await evalJson(session, `(() => new Promise((resolve, reject) => {
            const deadline = Date.now() + 8000;
            const tick = () => {
                if (!window.HebrewLoginPage || !document.getElementById('login-form')) {
                    if (Date.now() > deadline) return reject(new Error('login page missing'));
                    return setTimeout(tick, 100);
                }
                document.getElementById('login-username').value = 'mustchange';
                document.getElementById('login-password').value = 'reset-password1';
                document.getElementById('login-submit').click();
                resolve(true);
            };
            tick();
        }))()`);
        await waitUrl(session, (u) => {
            try {
                const parsed = new URL(u);
                return parsed.pathname === '/' || parsed.pathname === '/index.html';
            } catch (_) {
                return false;
            }
        }, 15000);
        await new Promise((r) => setTimeout(r, 1000));

        const forcedBefore = await evalJson(session, `(() => {
            const dlg = document.getElementById('hebrew-shell-pw-dialog');
            const locked = document.querySelector('.hebrew-shell-nav-locked');
            const review = document.querySelector('[data-nav="review"]');
            let navigated = false;
            if (review) {
                const href = review.getAttribute('href');
                review.click();
                navigated = location.pathname.indexOf('/review') === 0;
                // restore if somehow navigated
                if (navigated) history.pushState({}, '', '/');
            }
            return {
                dialogOpen: !!(dlg && !dlg.hidden && !dlg.classList.contains('hidden')),
                locked: !!locked,
                navigatedOnClick: navigated
            };
        })()`);
        if (!forcedBefore.dialogOpen) throw new Error('forced password dialog not open');
        if (forcedBefore.navigatedOnClick) throw new Error('nav should be blocked while must_change_password');

        await evalJson(session, `(() => {
            const cur = document.getElementById('hebrew-shell-pw-current');
            const neu = document.getElementById('hebrew-shell-pw-new');
            const form = document.getElementById('hebrew-shell-pw-form');
            cur.value = 'reset-password1';
            neu.value = 'fresh-password9';
            form.requestSubmit ? form.requestSubmit() : form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
            return true;
        })()`);
        await new Promise((r) => setTimeout(r, 1500));

        const forcedAfter = await evalJson(session, `(() => {
            const dlg = document.getElementById('hebrew-shell-pw-dialog');
            const locked = document.querySelector('.hebrew-shell-nav-locked');
            return {
                dialogOpen: !!(dlg && !dlg.hidden && !dlg.classList.contains('hidden')),
                locked: !!locked
            };
        })()`);
        if (forcedAfter.dialogOpen) throw new Error('password dialog still open after save');
        if (forcedAfter.locked) throw new Error('nav still locked after password change');
        report.forcedDialog = { before: forcedBefore, after: forcedAfter };

        report.ok = true;
        console.log(JSON.stringify(report, null, 2));
        session.close();
    } finally {
        try { chrome.kill(); } catch (_) { /* ignore */ }
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (_) { /* ignore */ }
        await harness.stop();
    }
}

async function main() {
    try {
        await runOnce();
    } catch (err) {
        const msg = String(err && err.message || err);
        if (/EBUSY|EPERM/i.test(msg)) {
            console.error('retrying once after', msg);
            await new Promise((r) => setTimeout(r, 500));
            await runOnce();
            return;
        }
        throw err;
    }
}

main().catch((err) => {
    console.error(err.message || err);
    process.exit(1);
});
