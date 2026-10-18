#!/usr/bin/env node
/**
 * Headless Chrome smoke: server-backed progress client (SPEC-013).
 * Two profiles, legacy-key isolation, offline queue, /api/ fetch log.
 * Ephemeral port via signed-in-server (never 8080). Temp DATA_DIR.
 *
 *   node test/chrome-progress-smoke.js
 */
'use strict';

const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const harness = require('./helpers/signed-in-server');

const { CHROME } = require('./helpers/chrome-cdp');

const PENDING_TEXT = 'עדיין לא נשמר — ננסה שוב. (Not saved yet — will retry.)';
const WORD = 'בֹּקֶר';
const SLUG = 'my-morning';

function get(url) {
    return new Promise((resolve, reject) => {
        const req = http.get(url, (res) => {
            let data = '';
            res.on('data', (c) => { data += c; });
            res.on('end', () => resolve({ status: res.statusCode, body: data }));
        });
        req.on('error', reject);
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

async function launchChrome(userData) {
    const debugPort = 9500 + Math.floor(Math.random() * 400);
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

async function openReview(session, baseUrl) {
    await session.call('Page.navigate', { url: baseUrl + '/review/' });
    await waitFor(session, `(() => ({
        ok: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive()
            && window.HebrewReview && document.querySelector('[data-shell-sync]')),
        status: window.HebrewSync && window.HebrewSync.getStatus && window.HebrewSync.getStatus(),
        active: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive())
    }))()`, 20000);
}

function eventCount(userId) {
    const eventsRepo = require('../server/db/repos/events');
    return eventsRepo.listByUser(userId, 5000).length;
}

async function runOnce() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);

    const started = await harness.start({ lessons: 'real' });
    const baseUrl = started.baseUrl;
    const learnerId = started.users.learner.id;
    const accounts = require('../server/accounts');
    const progress = require('../server/progress');

    const report = {
        ok: false,
        ac1: null,
        ac2: null,
        ac3: null,
        ac4: null,
        ac6: null,
        fetchUrls: []
    };

    const profileA = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-prog-a-'));
    const profileB = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-prog-b-'));
    let chromeA = null;
    let chromeB = null;
    let sessionA = null;
    let sessionB = null;

    try {
        // --- Profile A: mark a word, flush to server ---
        const launchedA = await launchChrome(profileA);
        chromeA = launchedA.chrome;
        sessionA = launchedA.session;
        await setSessionCookie(sessionA, baseUrl, 'learner');
        await sessionA.call('Page.navigate', { url: baseUrl + '/review/' });
        // Install fetch logger before sync init completes — inject on new document
        await sessionA.call('Page.addScriptToEvaluateOnNewDocument', {
            source: `(() => {
                window.__fetchLog = [];
                const orig = window.fetch.bind(window);
                window.fetch = function (input, init) {
                    const url = typeof input === 'string' ? input : (input && input.url) || String(input);
                    window.__fetchLog.push(url);
                    return orig(input, init);
                };
                window.__fetchLogInstalled = true;
            })();`
        });
        await sessionA.call('Page.navigate', { url: baseUrl + '/review/' });
        await waitFor(sessionA, `(() => ({
            ok: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive()),
            status: window.HebrewSync && window.HebrewSync.getStatus && window.HebrewSync.getStatus()
        }))()`, 20000);

        const markA = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                const Sync = window.HebrewSync;
                const Review = window.HebrewReview;
                Review.load();
                Review.record(${JSON.stringify(WORD)}, 'he_en', 'pass', {
                    lesson: ${JSON.stringify(SLUG)},
                    activity: 'words'
                });
                const pendingText = (document.querySelector('[data-shell-sync]') || {}).textContent || '';
                await Sync.flush();
                const deadline = Date.now() + 8000;
                const waitSynced = () => new Promise((res) => {
                    const tick = () => {
                        if (Sync.getStatus() === 'synced' || Date.now() > deadline) return res(Sync.getStatus());
                        setTimeout(tick, 100);
                    };
                    tick();
                });
                const status = await waitSynced();
                const state = Sync.getReviewState();
                const entry = state.words && state.words[${JSON.stringify(WORD)}];
                resolve({
                    status,
                    pendingText,
                    lastResult: entry && entry.he_en && entry.he_en.lastResult,
                    step: entry && entry.he_en && entry.he_en.step,
                    fetchLog: window.__fetchLog || []
                });
            } catch (err) {
                reject(err);
            }
        }))()`);

        if (markA.status !== 'synced') {
            throw new Error('profile A did not reach synced; status=' + markA.status);
        }
        if (markA.lastResult !== 'pass') {
            throw new Error('profile A missing server word state');
        }
        report.fetchUrls = report.fetchUrls.concat(markA.fetchLog || []);

        // --- Profile B: same user, fresh browser — Review shows updated schedule ---
        const launchedB = await launchChrome(profileB);
        chromeB = launchedB.chrome;
        sessionB = launchedB.session;
        await setSessionCookie(sessionB, baseUrl, 'learner');
        await sessionB.call('Page.addScriptToEvaluateOnNewDocument', {
            source: `(() => {
                window.__fetchLog = [];
                const orig = window.fetch.bind(window);
                window.fetch = function (input, init) {
                    const url = typeof input === 'string' ? input : (input && input.url) || String(input);
                    window.__fetchLog.push(url);
                    return orig(input, init);
                };
            })();`
        });
        await openReview(sessionB, baseUrl);
        const markB = await evalJson(sessionB, `(() => {
            const Sync = window.HebrewSync;
            const Review = window.HebrewReview;
            Review.load();
            const store = Review.all();
            const entry = store.words && store.words[${JSON.stringify(WORD)}];
            const due = Review.dueList(store, Date.now());
            return {
                lastResult: entry && entry.he_en && entry.he_en.lastResult,
                step: entry && entry.he_en && entry.he_en.step,
                due: entry && entry.he_en && entry.he_en.due,
                dueCount: due.length,
                fetchLog: window.__fetchLog || []
            };
        })()`);
        report.fetchUrls = report.fetchUrls.concat(markB.fetchLog || []);
        if (markB.lastResult !== 'pass') {
            throw new Error('ac-1: profile B did not see updated schedule; got ' + JSON.stringify(markB));
        }
        report.ac1 = { ok: true, step: markB.step, due: markB.due };

        // --- ac-2: fresh account on browser with legacy keys ---
        const fresh = accounts.createUser({
            username: 'freshuser',
            password: harness.GOOD_PASSWORD,
            role: 'user',
            displayName: 'Fresh',
            actor: 'test'
        });
        const freshSession = accounts.signIn('freshuser', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        // Reuse profile B but inject legacy keys before sync, then switch cookie
        const legacyReview = JSON.stringify({
            v: 1,
            words: {
                [WORD]: {
                    he_en: {
                        due: '2000-01-01T00:00:00.000Z',
                        step: 4,
                        streak: 9,
                        lapses: 0,
                        lastResult: 'pass',
                        lastAt: '2000-01-01T00:00:00.000Z'
                    },
                    en_he: {
                        due: '2000-01-01T00:00:00.000Z',
                        step: 0,
                        streak: 0,
                        lapses: 0,
                        lastResult: null,
                        lastAt: null
                    },
                    sources: [SLUG]
                }
            },
            migratedLessons: { [SLUG]: true }
        });
        const legacyLesson = JSON.stringify({
            lesson: SLUG,
            round: 7,
            completedRounds: 3,
            wordMarks: { '1': 'knew' }
        });

        await evalJson(sessionB, `(() => {
            localStorage.setItem('hebrew-review:v1', ${JSON.stringify(legacyReview)});
            localStorage.setItem('hebrew-worksheet:${SLUG}', ${JSON.stringify(legacyLesson)});
            return {
                review: localStorage.getItem('hebrew-review:v1'),
                lesson: localStorage.getItem('hebrew-worksheet:${SLUG}')
            };
        })()`);

        // Sign out learner cookie; set fresh user cookie
        const cookiesMod = require('../server/http/cookies');
        await sessionB.call('Network.deleteCookies', {
            name: cookiesMod.cookieName(),
            url: baseUrl
        });
        await sessionB.call('Network.setCookie', {
            name: cookiesMod.cookieName(),
            value: freshSession.token,
            url: baseUrl,
            path: '/'
        });
        // Patch harness cookieFor won't know freshuser — we set CDP cookie directly
        await sessionB.call('Page.navigate', { url: baseUrl + '/review/' });
        await waitFor(sessionB, `(() => ({
            ok: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive()
                && window.HebrewSync.getUser && window.HebrewSync.getUser()
                && window.HebrewSync.getUser().username === 'freshuser'),
            user: window.HebrewSync && window.HebrewSync.getUser && window.HebrewSync.getUser()
        }))()`, 20000);

        const ac2 = await evalJson(sessionB, `(() => {
            const Review = window.HebrewReview;
            Review.load();
            const store = Review.all();
            const words = store.words || {};
            return {
                wordCount: Object.keys(words).length,
                hasLegacyWord: !!words[${JSON.stringify(WORD)}],
                reviewKey: localStorage.getItem('hebrew-review:v1'),
                lessonKey: localStorage.getItem('hebrew-worksheet:${SLUG}')
            };
        })()`);
        if (ac2.wordCount !== 0 || ac2.hasLegacyWord) {
            throw new Error('ac-2: fresh account showed legacy progress');
        }
        if (ac2.reviewKey !== legacyReview) {
            throw new Error('ac-2: hebrew-review:v1 mutated');
        }
        if (ac2.lessonKey !== legacyLesson) {
            throw new Error('ac-2: hebrew-worksheet key mutated');
        }
        report.ac2 = { ok: true, freshId: fresh.id };

        // --- ac-4: user A's pending queue not used when B signed in (same browser) ---
        // Seed learner queue while signed out of sync memory, then stay as freshuser
        await evalJson(sessionB, `(() => {
            localStorage.setItem('hebrew-sync:v1:' + ${JSON.stringify(String(learnerId))}, JSON.stringify([{
                id: 'leak-1',
                word: 'לֹא',
                direction: 'he_en',
                result: 'pass',
                slug: ${JSON.stringify(SLUG)}
            }]));
            return true;
        })()`);
        const ac4 = await evalJson(sessionB, `(() => {
            const Sync = window.HebrewSync;
            const uid = Sync.getUser().id;
            const myKey = 'hebrew-sync:v1:' + uid;
            const otherKey = 'hebrew-sync:v1:' + ${JSON.stringify(String(learnerId))};
            const myQ = localStorage.getItem(myKey);
            const otherQ = localStorage.getItem(otherKey);
            // Trigger flush — must not clear or send other user's queue
            return Sync.flush().then(() => ({
                myUserId: uid,
                otherStillThere: !!localStorage.getItem(otherKey),
                otherRaw: localStorage.getItem(otherKey),
                myQueue: myQ,
                status: Sync.getStatus()
            }));
        })()`);
        if (!ac4.otherStillThere) {
            throw new Error('ac-4: other user queue was cleared');
        }
        if (!/leak-1/.test(ac4.otherRaw || '')) {
            throw new Error('ac-4: other user queue content lost');
        }
        report.ac4 = { ok: true, myUserId: ac4.myUserId };

        // --- ac-3: network blocked → queue + pending text → restore → synced, event count ---
        // Switch back to learner on profile A
        await setSessionCookie(sessionA, baseUrl, 'learner');
        await sessionA.call('Page.navigate', { url: baseUrl + '/review/' });
        await waitFor(sessionA, `(() => ({
            ok: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive()
                && window.HebrewSync.getUser && window.HebrewSync.getUser().username === 'learner'),
            user: window.HebrewSync && window.HebrewSync.getUser && window.HebrewSync.getUser()
        }))()`, 20000);

        const eventsBefore = eventCount(learnerId);
        const offlineAnswers = 3;

        const ac3 = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                const Sync = window.HebrewSync;
                const Review = window.HebrewReview;
                const pendingExact = ${JSON.stringify(PENDING_TEXT)};
                const origFetch = window.fetch.bind(window);
                window.fetch = function (input, init) {
                    const url = typeof input === 'string' ? input : (input && input.url) || String(input);
                    window.__fetchLog = window.__fetchLog || [];
                    window.__fetchLog.push(url);
                    if (String(url).indexOf('/api/review/signals') !== -1) {
                        return Promise.reject(new Error('blocked'));
                    }
                    return origFetch(input, init);
                };
                Review.load();
                for (let i = 0; i < ${offlineAnswers}; i += 1) {
                    Review.record('קָם', 'he_en', i % 2 === 0 ? 'pass' : 'miss', {
                        lesson: ${JSON.stringify(SLUG)},
                        activity: 'words'
                    });
                }
                // Give scheduled flush a chance to fail
                await new Promise((r) => setTimeout(r, 300));
                const slot = document.querySelector('[data-shell-sync]');
                const pendingText = slot ? slot.textContent : '';
                const queued = JSON.parse(localStorage.getItem('hebrew-sync:v1:' + Sync.getUser().id) || '[]');
                if (Sync.getStatus() !== 'pending') {
                    return reject(new Error('expected pending status, got ' + Sync.getStatus()));
                }
                if (pendingText !== pendingExact) {
                    return reject(new Error('pending text mismatch: ' + JSON.stringify(pendingText)));
                }
                if (queued.length < ${offlineAnswers}) {
                    return reject(new Error('expected >= ${offlineAnswers} queued, got ' + queued.length));
                }
                // Restore network
                window.fetch = function (input, init) {
                    const url = typeof input === 'string' ? input : (input && input.url) || String(input);
                    window.__fetchLog = window.__fetchLog || [];
                    window.__fetchLog.push(url);
                    return origFetch(input, init);
                };
                await Sync.flush();
                const deadline = Date.now() + 10000;
                while (Sync.getStatus() !== 'synced' && Date.now() < deadline) {
                    await new Promise((r) => setTimeout(r, 100));
                    if (Sync.getStatus() === 'pending') await Sync.flush();
                }
                const left = JSON.parse(localStorage.getItem('hebrew-sync:v1:' + Sync.getUser().id) || '[]');
                resolve({
                    status: Sync.getStatus(),
                    pendingText,
                    queuedBefore: queued.length,
                    left: left.length,
                    fetchLog: window.__fetchLog || []
                });
            } catch (err) {
                reject(err);
            }
        }))()`);

        report.fetchUrls = report.fetchUrls.concat(ac3.fetchLog || []);
        if (ac3.status !== 'synced') {
            throw new Error('ac-3: expected synced after restore, got ' + ac3.status);
        }
        if (ac3.left !== 0) {
            throw new Error('ac-3: queue not empty after flush');
        }
        const eventsAfter = eventCount(learnerId);
        const delta = eventsAfter - eventsBefore;
        // Idempotent: exactly the offline answers (plus any from earlier mark — already flushed)
        if (delta !== offlineAnswers) {
            throw new Error('ac-3: expected ' + offlineAnswers + ' new events, got ' + delta
                + ' (before=' + eventsBefore + ' after=' + eventsAfter + ')');
        }
        report.ac3 = {
            ok: true,
            pendingText: ac3.pendingText,
            eventsDelta: delta
        };

        // --- ac-6: every fetch URL starts with /api/ ---
        const bad = report.fetchUrls.filter((u) => {
            const s = String(u);
            // Relative /api/... or absolute same-origin /api/...
            if (s.startsWith('/api/')) return false;
            try {
                const parsed = new URL(s, baseUrl);
                return !parsed.pathname.startsWith('/api/');
            } catch (_) {
                return true;
            }
        });
        if (bad.length) {
            throw new Error('ac-6: non-/api/ fetch URLs: ' + JSON.stringify(bad.slice(0, 10)));
        }
        report.ac6 = { ok: true, count: report.fetchUrls.length };

        // Sanity: server progress for learner includes בֹּקֶר
        const serverState = progress.reviewState(learnerId);
        if (!serverState.words[WORD] || serverState.words[WORD].he_en.lastResult !== 'pass') {
            throw new Error('server state missing profile A mark');
        }

        report.ok = true;
        console.log(JSON.stringify(report, null, 2));
    } finally {
        if (sessionA) try { sessionA.close(); } catch (_) { /* ignore */ }
        if (sessionB) try { sessionB.close(); } catch (_) { /* ignore */ }
        if (chromeA) try { chromeA.kill(); } catch (_) { /* ignore */ }
        if (chromeB) try { chromeB.kill(); } catch (_) { /* ignore */ }
        try { fs.rmSync(profileA, { recursive: true, force: true }); } catch (_) { /* ignore */ }
        try { fs.rmSync(profileB, { recursive: true, force: true }); } catch (_) { /* ignore */ }
        await harness.stop();
    }
}

async function main() {
    try {
        await runOnce();
    } catch (err) {
        const msg = String(err && err.message || err);
        if (/EBUSY|EPERM/i.test(msg)) {
            console.error('Retrying once after', msg);
            await new Promise((r) => setTimeout(r, 500));
            await runOnce();
            return;
        }
        throw err;
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-progress-smoke") || main().catch((err) => {
    console.error(err.message || err);
    process.exit(1);
});
