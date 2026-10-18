#!/usr/bin/env node
/**
 * Headless Chrome smoke: Today + Lessons pages (SPEC-014 ac-1…ac-5).
 * Ephemeral port via signed-in-server (never 8080). Temp DATA_DIR.
 *
 *   node test/chrome-today-lessons-smoke.js
 */
'use strict';

const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const harness = require('./helpers/signed-in-server');

const { CHROME } = require('./helpers/chrome-cdp');

const DAY_MS = 24 * 60 * 60 * 1000;
const RESTRICTED = 'anal-sex-vocab';
const CONTINUE_SLUG = 'my-morning';

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
    const deadline = Date.now() + (ms || 20000);
    let last = null;
    while (Date.now() < deadline) {
        last = await evalJson(session, predicateExpr);
        if (last && last.ok) return last;
        await new Promise((r) => setTimeout(r, 200));
    }
    throw new Error('wait timed out; last=' + JSON.stringify(last));
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

async function installFetchLog(session) {
    await session.call('Page.addScriptToEvaluateOnNewDocument', {
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
}

async function openToday(session, baseUrl) {
    await session.call('Page.navigate', { url: baseUrl + '/' });
    return waitFor(session, `(() => {
        const ready = document.querySelector('[data-today-state="ready"], [data-today-state="no-lessons"], [data-today-card]');
        const err = document.querySelector('[data-today-state="error"]');
        return {
            ok: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive()
                && (ready || err)),
            error: !!(err),
            catalog: (window.HEBREW_CATALOG || []).map(function (l) { return l.slug; })
        };
    })()`, 25000);
}

async function openLessons(session, baseUrl) {
    await session.call('Page.navigate', { url: baseUrl + '/lessons/' });
    return waitFor(session, `(() => {
        const ready = document.querySelector('[data-lessons-state="ready"], [data-lessons-state="no-lessons"], [data-lessons-state="filtered-empty"], .lessons-filter');
        const err = document.querySelector('[data-lessons-state="error"]');
        return {
            ok: !!(window.HebrewSync && window.HebrewSync.isActive && window.HebrewSync.isActive()
                && (ready || err)),
            error: !!(err),
            catalog: (window.HEBREW_CATALOG || []).map(function (l) { return l.slug; })
        };
    })()`, 25000);
}

function signal(partial) {
    return Object.assign({
        id: 'sig-' + Math.random().toString(36).slice(2),
        word: 'שָׁלוֹם',
        direction: 'he_en',
        result: 'not_yet',
        at: new Date().toISOString(),
        slug: CONTINUE_SLUG,
        activity: 'words'
    }, partial);
}

async function runOnce() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);

    const started = await harness.start({ lessons: 'real' });
    const baseUrl = started.baseUrl;
    const accounts = require('../server/accounts');
    const progress = require('../server/progress');
    const visibility = require('../server/visibility');

    const report = {
        ok: false,
        ac1: null,
        ac2: null,
        ac3: null,
        ac4: null,
        ac5: null,
        fetchUrls: []
    };

    // Restrict one lesson to admin-only so the learner (no grant) cannot see it.
    visibility.setVisibility('mcp', RESTRICTED, 'selected');
    visibility.setGrants('mcp', RESTRICTED, [started.users.admin.id]);

    const fresh = accounts.createUser({
        username: 'todayfresh',
        password: harness.GOOD_PASSWORD,
        role: 'user',
        displayName: 'Today Fresh',
        actor: 'test'
    });
    const freshSession = accounts.signIn('todayfresh', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
    started.users.todayfresh = {
        id: fresh.id,
        username: 'todayfresh',
        role: 'user',
        password: harness.GOOD_PASSWORD,
        token: freshSession.token
    };

    const learnerId = started.users.learner.id;

    // ac-1 seed: 3 due he_en words + one in-progress lesson
    progress.applySignals(learnerId, [
        signal({ id: 'today-due-1', word: 'בֹּקֶר' }),
        signal({ id: 'today-due-2', word: 'קָפֶה' }),
        signal({ id: 'today-due-3', word: 'מַיִם' })
    ]);
    progress.putLessonProgress(learnerId, CONTINUE_SLUG, {
        lesson: CONTINUE_SLUG,
        round: 1,
        roundStartedAt: Date.now() - 60 * 60 * 1000,
        stepIndex: 0,
        completedSteps: {},
        completedRounds: 0,
        vowelsOn: true
    }, 0);

    const profileA = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-today-a-'));
    const profileB = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-today-b-'));
    let chromeA = null;
    let chromeB = null;
    let sessionA = null;
    let sessionB = null;

    try {
        const launchedA = await launchChrome(profileA);
        chromeA = launchedA.chrome;
        sessionA = launchedA.session;
        await setSessionCookie(sessionA, baseUrl, 'learner');
        await installFetchLog(sessionA);

        // --- ac-1: Today with due review + continue ---
        await openToday(sessionA, baseUrl);
        const ac1 = await evalJson(sessionA, `(() => {
            const review = document.getElementById('today-review-card');
            const due = document.getElementById('review-due-count');
            const start = document.getElementById('today-review-start');
            const cont = document.getElementById('today-continue-card');
            const contLink = document.getElementById('today-continue-link');
            const welcome = document.getElementById('today-welcome-card');
            const lessonCards = document.querySelectorAll('[data-today-card]');
            return {
                dueText: due && due.textContent,
                startHref: start && start.getAttribute('href'),
                continueSlug: cont && cont.getAttribute('data-lesson-slug'),
                continueHref: contLink && contLink.getAttribute('href'),
                hasWelcome: !!welcome,
                cardCount: lessonCards.length,
                catalog: (window.HEBREW_CATALOG || []).map(function (l) { return l.slug; }),
                fetchLog: window.__fetchLog || []
            };
        })()`);
        report.fetchUrls = report.fetchUrls.concat(ac1.fetchLog || []);
        if (String(ac1.dueText) !== '3') {
            throw new Error('ac-1: expected 3 due, got ' + JSON.stringify(ac1));
        }
        if (ac1.startHref !== '/review/') {
            throw new Error('ac-1: Start href expected /review/, got ' + ac1.startHref);
        }
        if (ac1.continueSlug !== CONTINUE_SLUG) {
            throw new Error('ac-1: Continue slug expected ' + CONTINUE_SLUG + ', got ' + ac1.continueSlug);
        }
        if (!ac1.continueHref || ac1.continueHref.indexOf('/worksheets/' + CONTINUE_SLUG) !== 0) {
            throw new Error('ac-1: Continue href should open lesson, got ' + ac1.continueHref);
        }
        if (ac1.hasWelcome) throw new Error('ac-1: Welcome should not show for active account');
        if ((ac1.catalog || []).indexOf(RESTRICTED) !== -1) {
            throw new Error('ac-1: restricted lesson leaked into learner catalog');
        }
        report.ac1 = {
            ok: true,
            due: ac1.dueText,
            startHref: ac1.startHref,
            continueSlug: ac1.continueSlug,
            continueHref: ac1.continueHref
        };

        // --- ac-3: Lessons list, chips, filter, empty ---
        await openLessons(sessionA, baseUrl);
        const ac3 = await evalJson(sessionA, `(() => new Promise((resolve) => {
            const catalog = window.HEBREW_CATALOG || [];
            const cards = Array.from(document.querySelectorAll('[data-lesson-slug]'));
            const slugs = cards.map(function (c) { return c.getAttribute('data-lesson-slug'); });
            const statuses = {};
            cards.forEach(function (c) {
                statuses[c.getAttribute('data-lesson-slug')] = c.getAttribute('data-lesson-status');
            });
            const morning = document.querySelector('[data-lesson-slug="${CONTINUE_SLUG}"]');
            const chip = morning && morning.querySelector('.lesson-status');
            const advancedBtn = document.querySelector('[data-level-filter="advanced"]');
            if (advancedBtn) advancedBtn.click();
            setTimeout(function () {
                const empty = document.querySelector('[data-lessons-state="filtered-empty"]');
                const allBtn = document.querySelector('[data-level-filter="all"]');
                if (allBtn) allBtn.click();
                setTimeout(function () {
                    const basicBtn = document.querySelector('[data-level-filter="basic"]');
                    if (basicBtn) basicBtn.click();
                    setTimeout(function () {
                        const basicCards = Array.from(document.querySelectorAll('[data-lesson-slug]'));
                        resolve({
                            catalogSlugs: catalog.map(function (l) { return l.slug; }),
                            cardSlugs: slugs,
                            statuses: statuses,
                            morningChip: chip && chip.textContent,
                            advancedEmpty: !!empty,
                            basicSlugs: basicCards.map(function (c) { return c.getAttribute('data-lesson-slug'); }),
                            basicLevels: basicCards.map(function (c) { return c.getAttribute('data-lesson-level'); }),
                            fetchLog: window.__fetchLog || []
                        });
                    }, 50);
                }, 50);
            }, 50);
        }))()`);
        report.fetchUrls = report.fetchUrls.concat(ac3.fetchLog || []);
        const cat = ac3.catalogSlugs || [];
        const cards = ac3.cardSlugs || [];
        if (cat.length !== cards.length || cat.some((s, i) => cards.indexOf(s) < 0)) {
            // Cards are sorted (due first); compare as sets
            const catSet = cat.slice().sort().join(',');
            const cardSet = cards.slice().sort().join(',');
            if (catSet !== cardSet) {
                throw new Error('ac-3: lessons cards !== catalog; catalog=' + catSet + ' cards=' + cardSet);
            }
        }
        if (cat.indexOf(RESTRICTED) !== -1) {
            throw new Error('ac-3: restricted lesson in filtered catalog');
        }
        if (ac3.statuses[CONTINUE_SLUG] !== 'in_progress') {
            throw new Error('ac-3: my-morning chip expected in_progress, got ' + JSON.stringify(ac3.statuses));
        }
        if (!ac3.advancedEmpty) {
            throw new Error('ac-3: advanced filter should show empty state');
        }
        if (!(ac3.basicSlugs || []).length || (ac3.basicLevels || []).some((l) => l !== 'basic')) {
            throw new Error('ac-3: basic filter failed; ' + JSON.stringify(ac3.basicSlugs));
        }
        report.ac3 = {
            ok: true,
            catalogCount: cat.length,
            statuses: ac3.statuses,
            advancedEmpty: ac3.advancedEmpty,
            basicCount: (ac3.basicSlugs || []).length
        };

        // --- ac-4: restricted absent on both pages for no-grant user ---
        await openToday(sessionA, baseUrl);
        const todayCheck = await evalJson(sessionA, `(() => ({
            text: document.body.innerText || '',
            catalog: (window.HEBREW_CATALOG || []).map(function (l) { return l.slug; }),
            cards: Array.from(document.querySelectorAll('[data-lesson-slug], [data-today-card]')).map(function (el) {
                return el.getAttribute('data-lesson-slug') || el.getAttribute('data-today-card');
            })
        }))()`);
        await openLessons(sessionA, baseUrl);
        const lessonsCheck = await evalJson(sessionA, `(() => ({
            text: document.body.innerText || '',
            catalog: (window.HEBREW_CATALOG || []).map(function (l) { return l.slug; }),
            slugs: Array.from(document.querySelectorAll('[data-lesson-slug]')).map(function (el) {
                return el.getAttribute('data-lesson-slug');
            })
        }))()`);
        const restrictedTitle = 'הִיא בַּמַּצַּב לָאָנָלי';
        if ((todayCheck.catalog || []).indexOf(RESTRICTED) !== -1
            || (lessonsCheck.catalog || []).indexOf(RESTRICTED) !== -1
            || (lessonsCheck.slugs || []).indexOf(RESTRICTED) !== -1
            || (todayCheck.text || '').indexOf(restrictedTitle) !== -1
            || (lessonsCheck.text || '').indexOf(restrictedTitle) !== -1) {
            throw new Error('ac-4: restricted lesson visible to learner without grant');
        }
        report.ac4 = {
            ok: true,
            todayCatalog: todayCheck.catalog,
            lessonsCatalog: lessonsCheck.catalog
        };

        // --- ac-5: bilingual labels + nikud preserved + fetch only /api/ ---
        const ac5 = await evalJson(sessionA, `(() => {
            const body = document.body.innerText || '';
            const sample = 'הִיא בַּמַּצַּב';
            const escaped = window.HebrewUtils.escapeHtml(sample);
            const titles = (window.HEBREW_CATALOG || []).map(function (l) { return String(l.title || ''); });
            return {
                hasHebrewEnglish: body.indexOf('שיעורים') !== -1 && body.indexOf('Lessons') !== -1,
                nikudPreserved: escaped === sample && /[\\u0591-\\u05C7]/.test(escaped),
                titleHasHebrew: titles.some(function (t) { return /[\\u05D0-\\u05EA]/.test(t); }),
                fetchLog: window.__fetchLog || []
            };
        })()`);
        report.fetchUrls = report.fetchUrls.concat(ac5.fetchLog || []);
        const nonApi = (report.fetchUrls || []).filter(function (url) {
            return String(url || '').indexOf('/api/') === -1;
        });
        if (!ac5.hasHebrewEnglish || !ac5.titleHasHebrew) {
            throw new Error('ac-5: Hebrew with English labels missing; ' + JSON.stringify(ac5));
        }
        if (!ac5.nikudPreserved) {
            throw new Error('ac-5: nikud not preserved by escapeHtml');
        }
        if (nonApi.length) {
            throw new Error('ac-5: non-/api/ fetch URLs: ' + JSON.stringify(nonApi.slice(0, 10)));
        }
        report.ac5 = {
            ok: true,
            hasHebrewEnglish: ac5.hasHebrewEnglish,
            nikudPreserved: ac5.nikudPreserved,
            fetchCount: report.fetchUrls.length
        };

        // --- ac-2: brand-new account ---
        const launchedB = await launchChrome(profileB);
        chromeB = launchedB.chrome;
        sessionB = launchedB.session;
        await setSessionCookie(sessionB, baseUrl, 'todayfresh');
        await installFetchLog(sessionB);
        await openToday(sessionB, baseUrl);
        const ac2 = await evalJson(sessionB, `(() => {
            return {
                hasWelcome: !!document.getElementById('today-welcome-card'),
                hasNext: !!document.getElementById('today-next-card'),
                hasReview: !!document.getElementById('today-review-card'),
                hasContinue: !!document.getElementById('today-continue-card'),
                nextSlug: (document.getElementById('today-next-card') || {}).getAttribute
                    && document.getElementById('today-next-card').getAttribute('data-lesson-slug'),
                catalog: (window.HEBREW_CATALOG || []).map(function (l) { return l.slug; })
            };
        })()`);
        if (!ac2.hasWelcome || !ac2.hasNext || ac2.hasReview || ac2.hasContinue) {
            throw new Error('ac-2: expected Welcome+Next without Review/Continue; got ' + JSON.stringify(ac2));
        }
        if ((ac2.catalog || []).indexOf(RESTRICTED) !== -1) {
            throw new Error('ac-2: restricted lesson in fresh user catalog');
        }
        report.ac2 = {
            ok: true,
            nextSlug: ac2.nextSlug,
            catalogCount: (ac2.catalog || []).length
        };

        report.ok = !!(report.ac1 && report.ac1.ok
            && report.ac2 && report.ac2.ok
            && report.ac3 && report.ac3.ok
            && report.ac4 && report.ac4.ok
            && report.ac5 && report.ac5.ok);
        return report;
    } finally {
        try { sessionA && sessionA.close(); } catch (_) { /* ignore */ }
        try { sessionB && sessionB.close(); } catch (_) { /* ignore */ }
        try { chromeA && chromeA.kill(); } catch (_) { /* ignore */ }
        try { chromeB && chromeB.kill(); } catch (_) { /* ignore */ }
        try {
            fs.rmSync(profileA, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(profileB, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        await harness.stop();
    }
}

async function main() {
    let lastErr = null;
    for (let attempt = 0; attempt < 2; attempt += 1) {
        try {
            const report = await runOnce();
            console.log(JSON.stringify(report, null, 2));
            if (!report.ok) process.exit(1);
            return;
        } catch (err) {
            lastErr = err;
            const msg = String(err && err.message || err);
            if (/EBUSY|EPERM/i.test(msg) && attempt === 0) {
                console.error('retry after', msg);
                await new Promise((r) => setTimeout(r, 500));
                continue;
            }
            throw err;
        }
    }
    throw lastErr;
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-today-lessons-smoke") || main().catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
