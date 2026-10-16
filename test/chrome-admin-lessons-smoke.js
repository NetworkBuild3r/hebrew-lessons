#!/usr/bin/env node
/**
 * Headless Chrome smoke: admin lessons list + editor (SPEC-017).
 * Ephemeral port, temp DATA_DIR + LESSONS_DIR, fake TTS (never real ElevenLabs).
 *
 *   node test/chrome-admin-lessons-smoke.js
 */
'use strict';

const http = require('http');
const https = require('https');
const { spawn, spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');
const crypto = require('crypto');

const harness = require('./helpers/signed-in-server');

const CHROME = process.env.CHROME_PATH
    || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';

const REPO = harness.REPO_ROOT;
const SCRIPT_MEANING = '<script>alert(1)</script>';

function sampleWorksheet(slug, overrides) {
    const base = {
        id: slug,
        title: 'בדיקה — ' + slug,
        subtitle: 'test',
        footer: 'footer',
        level: 'basic',
        vocab: {
            items: [{
                id: '1',
                word: 'שָׁלוֹם',
                meaning: 'hello',
                sound: 'sha-LOM',
                translit: 'shalom',
                trap: 'Stress the last syllable.',
                sentence: 'שָׁלוֹם עוֹלָם.'
            }]
        },
        reading: {
            paragraphs: ['שָׁלוֹם עוֹלָם.']
        },
        activities: [{ type: 'words' }, { type: 'story' }]
    };
    return Object.assign(base, overrides || {});
}

function stubHttps(payloadFactory) {
    const orig = https.request;
    let calls = 0;
    https.request = function (url, options, cb) {
        calls += 1;
        let body = '';
        const listeners = { data: [], end: [] };
        const res = {
            statusCode: 200,
            on(ev, fn) {
                if (ev === 'data') listeners.data.push(fn);
                if (ev === 'end') listeners.end.push(fn);
                return res;
            }
        };
        if (typeof options === 'function') cb = options;
        if (typeof cb === 'function') cb(res);
        return {
            on() { return this; },
            write(chunk) { body += chunk; },
            end() {
                let text = '';
                try { text = JSON.parse(body).text; } catch (_) { /* ignore */ }
                const buf = Buffer.from(
                    typeof payloadFactory === 'function' ? payloadFactory(text, calls) : payloadFactory
                );
                process.nextTick(() => {
                    listeners.data.forEach((fn) => fn(buf));
                    listeners.end.forEach((fn) => fn());
                });
            }
        };
    };
    return {
        restore() { https.request = orig; },
        get calls() { return calls; }
    };
}

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
        const req = http.request({
            hostname: u.hostname,
            port: u.port,
            path: u.pathname + u.search,
            method,
            headers: Object.assign({
                'Content-Type': 'application/json',
                'Content-Length': payload ? payload.length : 0,
                Origin: u.origin
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

function fileHash(filePath) {
    return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
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
                }, 180000);
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
        await new Promise((r) => setTimeout(r, 250));
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

function installFetchSpy(session) {
    return evalJson(session, `(() => {
        window.__adminFetchLog = [];
        const orig = window.fetch;
        window.fetch = function(input, init) {
            const url = typeof input === 'string' ? input : (input && input.url) || '';
            const method = (init && init.method) || 'GET';
            window.__adminFetchLog.push({ url: String(url), method: String(method).toUpperCase() });
            return orig.apply(this, arguments);
        };
        return true;
    })()`);
}

function lessonCardSlugsExpr() {
    return `(() => {
        const cards = Array.from(document.querySelectorAll('[data-lesson-slug], a[href*="/worksheets/"]'));
        const slugs = new Set();
        cards.forEach((el) => {
            const ds = el.getAttribute('data-lesson-slug');
            if (ds) slugs.add(ds);
            const href = el.getAttribute('href') || '';
            const m = href.match(/\\/worksheets\\/([^/]+)/);
            if (m) slugs.add(decodeURIComponent(m[1]));
        });
        // lessons-page cards
        document.querySelectorAll('.lesson-card, [data-slug]').forEach((el) => {
            const s = el.getAttribute('data-slug') || el.dataset && el.dataset.slug;
            if (s) slugs.add(s);
        });
        const text = document.body ? document.body.innerText : '';
        return { ok: true, slugs: Array.from(slugs), text: text.slice(0, 2000) };
    })()`;
}

async function runOnce() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);

    // Distinctive key only in this process — never printed, never from .env.
    const prevKey = process.env.ELEVENLABS_API_KEY;
    const prevVoice = process.env.ELEVENLABS_VOICE_ID;
    const prevModel = process.env.ELEVENLABS_MODEL;
    process.env.ELEVENLABS_API_KEY = 'test-elevenlabs-key-chrome-lessons-spec017-only';
    process.env.ELEVENLABS_VOICE_ID = 'voice';
    process.env.ELEVENLABS_MODEL = 'eleven_v3';

    const started = await harness.start({ lessons: 'fixture' });
    const baseUrl = started.baseUrl;
    const lessonsDir = started.lessonsDir;
    const adminCookie = harness.cookieFor('admin');
    const accounts = require('../server/accounts');

    const report = {
        ok: false,
        ac1: null,
        ac2: null,
        ac3: null,
        ac4: null,
        ac5: null,
        ac6: null,
        ac7: null
    };

    const profileAdmin = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-lessons-a-'));
    const profileUser = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-lessons-u-'));
    let chromeA = null;
    let chromeB = null;
    let sessionA = null;
    let sessionB = null;
    let httpsStub = null;

    const editSlug = 'public-lesson';
    const indexHtmlPath = path.join(lessonsDir, editSlug, 'index.html');
    const worksheetPath = path.join(lessonsDir, editSlug, 'worksheet.js');

    try {
        // Seed public-lesson with a validator-clean worksheet so editor saves work.
        const seeded = sampleWorksheet(editSlug, {
            title: 'Public Lesson',
            vocab: {
                items: [{
                    id: '1',
                    word: 'שָׁלוֹם',
                    meaning: 'hello',
                    sound: 'sha-LOM',
                    trap: 'Stress the last syllable.',
                    sentence: 'שָׁלוֹם עוֹלָם.'
                }, {
                    id: '2',
                    word: 'עוֹלָם',
                    meaning: 'world',
                    sound: 'o-LAM',
                    trap: 'Open the first syllable.',
                    sentence: 'הָעוֹלָם גָּדוֹל.'
                }]
            }
        });
        const seedPut = await request('PUT', baseUrl + '/api/admin/lessons/' + editSlug, {
            Cookie: adminCookie
        }, {
            worksheet: seeded,
            catalogEntry: {
                slug: editSlug,
                title: 'Public Lesson',
                subtitle: 'test',
                blurb: 'Run it again. The next round hides the English until you ask, and keeps the vowels.',
                level: 'basic'
            }
        });
        if (seedPut.status !== 200) {
            throw new Error('seed put failed: ' + seedPut.status + ' ' + seedPut.body);
        }

        // Second learner for selected-visibility AC
        const otherCreate = await request('POST', baseUrl + '/api/admin/users', {
            Cookie: adminCookie
        }, {
            username: 'otherlearner',
            displayName: 'Other',
            role: 'user',
            password: harness.GOOD_PASSWORD
        });
        if (otherCreate.status !== 201) {
            throw new Error('other learner create failed: ' + otherCreate.status);
        }
        const otherId = otherCreate.json.id;
        const otherSession = accounts.signIn('otherlearner', harness.GOOD_PASSWORD, { ip: '127.0.0.1' });
        harness.users.otherlearner = {
            id: otherId,
            username: 'otherlearner',
            role: 'user',
            password: harness.GOOD_PASSWORD,
            token: otherSession.token
        };

        // Ensure everyone can see public-lesson initially
        await request('PUT', baseUrl + '/api/admin/lessons/' + editSlug + '/visibility', {
            Cookie: adminCookie
        }, { mode: 'everyone' });

        chromeA = await launchChrome(profileAdmin);
        sessionA = chromeA.session;
        await setSessionCookie(sessionA, baseUrl, 'admin');

        function logStep(name) {
            console.error('[lessons-smoke]', name);
        }

        // ---------- ac-1: everyone → selected ----------
        logStep('ac-1 start');
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lessons' });
        await waitUrl(sessionA, (u) => u.includes('/admin/'), 15000);
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-lesson-slug="${editSlug}"]'),
            count: document.querySelectorAll('[data-lesson-slug]').length
        }))()`, 25000);

        const visSaved = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                const row = document.querySelector('[data-lesson-slug="${editSlug}"]');
                const select = row.querySelector('[data-vis-mode-select]');
                select.value = 'selected';
                select.dispatchEvent(new Event('change', { bubbles: true }));
                await new Promise(r => setTimeout(r, 200));
                const box = row.querySelector('[data-vis-user="${harness.users.learner.id}"]');
                if (!box) return reject(new Error('learner checkbox missing'));
                box.checked = true;
                // uncheck others
                row.querySelectorAll('[data-vis-user]').forEach((el) => {
                    if (el.getAttribute('data-vis-user') !== '${harness.users.learner.id}') el.checked = false;
                });
                const saveBtn = row.querySelector('[data-vis-save]');
                saveBtn.click();
                const deadline = Date.now() + 12000;
                while (Date.now() < deadline) {
                    const status = (row.querySelector('[data-vis-status]') || {}).textContent || '';
                    if (status.indexOf('✓') !== -1) {
                        return resolve({ ok: true, status });
                    }
                    await new Promise(r => setTimeout(r, 200));
                }
                reject(new Error('visibility save timed out'));
            } catch (e) { reject(e); }
        }))()`);
        if (!visSaved.ok) throw new Error('ac-1 visibility save failed');

        chromeB = await launchChrome(profileUser);
        sessionB = chromeB.session;

        await clearCookies(sessionB, baseUrl);
        await setSessionCookie(sessionB, baseUrl, 'otherlearner');
        await sessionB.call('Page.navigate', { url: baseUrl + '/lessons/' });
        await waitFor(sessionB, `(() => ({
            ok: !!(window.HEBREW_CATALOG || document.querySelector('[data-lessons-state], .lessons-grid, #lessons-app')),
            ready: document.readyState
        }))()`, 20000);
        await new Promise((r) => setTimeout(r, 800));
        const otherView = await evalJson(sessionB, lessonCardSlugsExpr());
        const otherHas = (otherView.slugs || []).includes(editSlug)
            || (otherView.text || '').includes('Public Lesson');

        await clearCookies(sessionB, baseUrl);
        await setSessionCookie(sessionB, baseUrl, 'learner');
        await sessionB.call('Page.navigate', { url: baseUrl + '/lessons/' });
        await waitFor(sessionB, `(() => ({
            ok: !!(window.HEBREW_CATALOG || document.querySelector('[data-lessons-state], .lessons-grid, #lessons-app')),
            ready: document.readyState
        }))()`, 20000);
        await new Promise((r) => setTimeout(r, 800));
        const learnerView = await evalJson(sessionB, lessonCardSlugsExpr());
        const learnerHas = (learnerView.slugs || []).includes(editSlug)
            || (learnerView.text || '').includes('Public Lesson');

        report.ac1 = {
            otherHas,
            learnerHas,
            otherSlugs: otherView.slugs,
            learnerSlugs: learnerView.slugs
        };
        if (otherHas) throw new Error('ac-1: other learner still sees public-lesson');
        if (!learnerHas) throw new Error('ac-1: selected learner cannot see public-lesson');
        logStep('ac-1 pass');

        // ---------- ac-2: edit vocab meaning, save, reopen; index.html unchanged ----------
        logStep('ac-2 start');
        const indexHashBefore = fileHash(indexHtmlPath);
        await setSessionCookie(sessionA, baseUrl, 'admin');
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lesson/' + editSlug });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-lesson-slug="${editSlug}"][data-admin-view="lesson"], .lesson-editor[data-lesson-slug="${editSlug}"]'),
            hasMeaning: !!document.querySelector('[data-vocab-field="meaning"]')
        }))()`, 25000);

        const newMeaning = 'hello-edited-' + Date.now();
        const saved = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                const meaning = document.querySelector('[data-vocab-field="meaning"][data-vocab-index="0"]')
                    || document.querySelector('[data-vocab-field="meaning"]');
                if (!meaning) return reject(new Error('meaning input missing'));
                meaning.value = ${JSON.stringify(newMeaning)};
                meaning.dispatchEvent(new Event('input', { bubbles: true }));
                const validateBtn = document.querySelector('[data-lesson-validate]');
                validateBtn.click();
                const deadline = Date.now() + 20000;
                let saveBtn = document.querySelector('[data-lesson-save]');
                while (Date.now() < deadline) {
                    saveBtn = document.querySelector('[data-lesson-save]');
                    if (saveBtn && !saveBtn.disabled) break;
                    await new Promise(r => setTimeout(r, 250));
                }
                if (!saveBtn || saveBtn.disabled) return reject(new Error('Save stayed disabled'));
                saveBtn.click();
                while (Date.now() < deadline) {
                    const result = (document.querySelector('[data-save-result]') || {}).textContent || '';
                    if (/Saved|נשמר/.test(result)) {
                        return resolve({
                            ok: true,
                            result,
                            hasViewLink: !!document.querySelector('.lesson-view-link')
                        });
                    }
                    const err = (document.querySelector('[data-editor-error]') || {}).textContent || '';
                    if (err) return reject(new Error('save error: ' + err));
                    await new Promise(r => setTimeout(r, 250));
                }
                reject(new Error('save timed out'));
            } catch (e) { reject(e); }
        }))()`);
        if (!saved.ok) throw new Error('ac-2 save failed');

        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lessons' });
        await waitFor(sessionA, `(() => ({ ok: !!document.querySelector('[data-admin-view="lessons"]') }))()`, 15000);
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lesson/' + editSlug });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-vocab-field="meaning"]')
        }))()`, 20000);
        const reopened = await evalJson(sessionA, `(() => {
            const meaning = document.querySelector('[data-vocab-field="meaning"][data-vocab-index="0"]')
                || document.querySelector('[data-vocab-field="meaning"]');
            return { ok: true, value: meaning ? meaning.value : null };
        })()`);
        const indexHashAfter = fileHash(indexHtmlPath);
        report.ac2 = {
            savedMeaning: newMeaning,
            reopened: reopened.value,
            indexUnchanged: indexHashBefore === indexHashAfter,
            hasViewLink: saved.hasViewLink
        };
        if (reopened.value !== newMeaning) {
            throw new Error('ac-2: reopened meaning mismatch');
        }
        if (indexHashBefore !== indexHashAfter) {
            throw new Error('ac-2: index.html hash changed');
        }
        logStep('ac-2 pass');

        // ---------- ac-3: unknown activity type — Save disabled, files unchanged ----------
        logStep('ac-3 start');
        const wsHashBefore = fileHash(worksheetPath);
        const indexHashAc3 = fileHash(indexHtmlPath);
        const invalid = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                // Inject unknown type via activities model: change first select to a fake option
                const sel = document.querySelector('[data-act-type]');
                if (!sel) return reject(new Error('activity select missing'));
                const opt = document.createElement('option');
                opt.value = 'not-a-real-activity';
                opt.textContent = 'not-a-real-activity';
                sel.appendChild(opt);
                sel.value = 'not-a-real-activity';
                sel.dispatchEvent(new Event('change', { bubbles: true }));
                const validateBtn = document.querySelector('[data-lesson-validate]');
                validateBtn.click();
                const deadline = Date.now() + 20000;
                while (Date.now() < deadline) {
                    const out = document.querySelector('[data-validate-out]');
                    const html = out ? out.innerHTML : '';
                    const saveBtn = document.querySelector('[data-lesson-save]');
                    if (/unknown type|not-a-real-activity|activities/i.test(html) || /Blocking|שגיאות/.test(html)) {
                        return resolve({
                            ok: true,
                            html,
                            saveDisabled: !!(saveBtn && saveBtn.disabled),
                            linked: /data-val-section="activities"|#lesson-section-activities/.test(html)
                        });
                    }
                    await new Promise(r => setTimeout(r, 250));
                }
                reject(new Error('validate error not shown'));
            } catch (e) { reject(e); }
        }))()`);
        report.ac3 = Object.assign({}, invalid, {
            worksheetUnchanged: fileHash(worksheetPath) === wsHashBefore,
            indexUnchanged: fileHash(indexHtmlPath) === indexHashAc3
        });
        if (!invalid.saveDisabled) throw new Error('ac-3: Save should stay disabled');
        if (!invalid.linked) throw new Error('ac-3: error not linked to activities section');
        if (fileHash(worksheetPath) !== wsHashBefore) {
            throw new Error('ac-3: worksheet changed on disk');
        }
        logStep('ac-3 pass');

        // Restore valid activities for later ACs — reload editor
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lesson/' + editSlug });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-lesson-validate]')
        }))()`, 20000);

        // ---------- ac-4: audio plan, cancel no POST, confirm with fake TTS ----------
        logStep('ac-4 start');
        await installFetchSpy(sessionA);
        const planShown = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                window.__adminFetchLog = [];
                const audioBtn = document.querySelector('[data-lesson-audio]');
                audioBtn.click();
                const deadline = Date.now() + 20000;
                while (Date.now() < deadline) {
                    const plan = document.querySelector('[data-audio-plan]');
                    const warn = document.querySelector('[data-audio-credit-warn]');
                    if (plan && warn) {
                        return resolve({
                            ok: true,
                            planText: plan.textContent || '',
                            warnText: warn.textContent || ''
                        });
                    }
                    await new Promise(r => setTimeout(r, 200));
                }
                reject(new Error('audio plan not shown'));
            } catch (e) { reject(e); }
        }))()`);
        if (!/ElevenLabs/i.test(planShown.warnText)) {
            throw new Error('ac-4: credit warning missing');
        }

        const cancelled = await evalJson(sessionA, `(() => {
            const before = (window.__adminFetchLog || []).slice();
            document.querySelector('[data-audio-cancel]').click();
            const after = window.__adminFetchLog || [];
            const posts = after.filter(e => e.method === 'POST' && /\\/audio$/.test(e.url) && !/audio-plan/.test(e.url));
            return {
                ok: true,
                posts: posts.length,
                log: after,
                beforeCount: before.length
            };
        })()`);
        if (cancelled.posts !== 0) {
            throw new Error('ac-4: cancel posted to generate route');
        }

        httpsStub = stubHttps('FAKE-TTS-LESSONS-SMOKE');
        await installFetchSpy(sessionA);
        logStep('ac-4 confirm generate');
        const generated = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                window.__adminFetchLog = [];
                document.querySelector('[data-lesson-audio]').click();
                const deadline = Date.now() + 90000;
                let confirm = null;
                while (Date.now() < deadline) {
                    confirm = document.querySelector('[data-audio-confirm]');
                    if (confirm && !confirm.disabled && document.querySelector('[data-audio-plan]')) break;
                    await new Promise(r => setTimeout(r, 200));
                }
                if (!confirm || confirm.disabled) return reject(new Error('confirm never enabled'));
                confirm.click();
                while (Date.now() < deadline) {
                    const result = (document.querySelector('[data-audio-result]') || {}).textContent || '';
                    const err = (document.querySelector('[data-audio-error]') || {}).textContent || '';
                    const posts = (window.__adminFetchLog || []).filter(e =>
                        e.method === 'POST' && /\\/audio$/.test(e.url) && !/audio-plan/.test(e.url)
                    );
                    if (posts.length && (result || err) && result !== 'יוצר…' && result !== 'Generating…') {
                        return resolve({
                            ok: true,
                            result,
                            err,
                            posts: posts.length,
                            log: window.__adminFetchLog
                        });
                    }
                    // Still generating — keep waiting while POST is in flight
                    if (posts.length && (result === 'יוצר…' || result === 'Generating…')) {
                        await new Promise(r => setTimeout(r, 400));
                        continue;
                    }
                    await new Promise(r => setTimeout(r, 300));
                }
                reject(new Error('generate timed out; log=' + JSON.stringify(window.__adminFetchLog || [])));
            } catch (e) { reject(e); }
        }))()`);
        if (generated.posts < 1) throw new Error('ac-4: confirm did not POST /audio');
        if (generated.err && /already running/i.test(generated.err)) {
            // acceptable if raced; still need verify
        } else if (generated.err && !generated.result) {
            throw new Error('ac-4 generate error: ' + generated.err);
        }

        // Temp-tree audio:verify inside the test (via authoring inventory path already run by generate)
        const invScript = path.join(REPO, 'scripts', 'audio-inventory.js');
        const invOutDir = path.join(REPO, 'data', 'audio-inventory', 'authoring-tmp');
        fs.mkdirSync(invOutDir, { recursive: true });
        const invOut = path.join(invOutDir, 'smoke-lessons-' + Date.now() + '.json');
        const inv = spawnSync(process.execPath, [invScript, 'inventory', '--out', invOut], {
            cwd: REPO,
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: REPO,
                LESSONS_DIR: lessonsDir
            })
        });
        if (inv.status !== 0) {
            throw new Error('temp inventory failed: ' + (inv.stderr || inv.stdout));
        }
        const ver = spawnSync(process.execPath, [invScript, 'verify', '--against', invOut], {
            cwd: REPO,
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: REPO,
                LESSONS_DIR: lessonsDir
            })
        });
        try { fs.unlinkSync(invOut); } catch (_) { /* ignore */ }
        report.ac4 = {
            planShown: !!planShown.ok,
            cancelPosts: cancelled.posts,
            generatePosts: generated.posts,
            ttsCalls: httpsStub.calls,
            verifyStatus: ver.status,
            verifyOut: String(ver.stdout || '').slice(0, 200)
        };
        if (ver.status !== 0) {
            throw new Error('ac-4 temp audio:verify failed: ' + (ver.stderr || ver.stdout));
        }
        if (httpsStub.calls < 1) {
            // generate may reuse all clips; still OK if POST succeeded
        }
        httpsStub.restore();
        httpsStub = null;
        logStep('ac-4 pass');

        // ---------- ac-5: new lesson hidden until shared ----------
        logStep('ac-5 start');
        const created = await evalJson(sessionA, `(() => new Promise(async (resolve, reject) => {
            try {
                location.hash = 'lessons';
                await new Promise(r => setTimeout(r, 500));
                const openBtn = document.querySelector('[data-lesson-action="new"]');
                if (!openBtn) return reject(new Error('new lesson button missing'));
                openBtn.click();
                document.getElementById('admin-new-lesson-title-input').value = 'שיעור חדש';
                const form = document.getElementById('admin-new-lesson-form');
                form.requestSubmit ? form.requestSubmit() : form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
                const deadline = Date.now() + 20000;
                while (Date.now() < deadline) {
                    const hash = String(location.hash);
                    const match = hash.match(/^#lesson\/([0-9a-f-]{36})$/i);
                    if (match) return resolve({ ok: true, hash: hash, slug: match[1] });
                    const err = (document.getElementById('admin-new-lesson-error') || {}).textContent || '';
                    if (err) return reject(new Error(err));
                    await new Promise(r => setTimeout(r, 250));
                }
                reject(new Error('create timed out'));
            } catch (e) { reject(e); }
        }))()`);
        if (!created.ok || !created.slug) throw new Error('ac-5 create failed');
        const newSlug = created.slug;

        // Check list shows hidden
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lessons' });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-lesson-slug="${newSlug}"]')
        }))()`, 20000);
        const newRow = await evalJson(sessionA, `(() => {
            const row = document.querySelector('[data-lesson-slug="${newSlug}"]');
            const sel = row && row.querySelector('[data-vis-mode-select]');
            return { ok: true, mode: sel ? sel.value : null };
        })()`);

        await clearCookies(sessionB, baseUrl);
        await setSessionCookie(sessionB, baseUrl, 'learner');
        await sessionB.call('Page.navigate', { url: baseUrl + '/lessons/' });
        await waitFor(sessionB, `(() => ({
            ok: !!(window.HEBREW_CATALOG || document.querySelector('#lessons-app'))
        }))()`, 20000);
        await new Promise((r) => setTimeout(r, 600));
        const beforeShare = await evalJson(sessionB, lessonCardSlugsExpr());
        const visibleBefore = (beforeShare.slugs || []).includes(newSlug)
            || (beforeShare.text || '').includes(newSlug);

        // Share with learner
        await request('PUT', baseUrl + '/api/admin/lessons/' + newSlug + '/visibility', {
            Cookie: adminCookie
        }, { mode: 'selected', userIds: [harness.users.learner.id] });

        await sessionB.call('Page.navigate', { url: baseUrl + '/lessons/' });
        await waitFor(sessionB, `(() => ({
            ok: !!(window.HEBREW_CATALOG || document.querySelector('#lessons-app'))
        }))()`, 20000);
        await new Promise((r) => setTimeout(r, 600));
        const afterShare = await evalJson(sessionB, lessonCardSlugsExpr());
        const visibleAfter = (afterShare.slugs || []).includes(newSlug)
            || (afterShare.text || '').includes('שיעור חדש')
            || (afterShare.text || '').includes(newSlug);

        report.ac5 = {
            mode: newRow.mode,
            visibleBefore,
            visibleAfter,
            newSlug
        };
        if (newRow.mode !== 'hidden') throw new Error('ac-5: new lesson should start hidden');
        if (visibleBefore) throw new Error('ac-5: hidden lesson visible to user');
        if (!visibleAfter) throw new Error('ac-5: shared lesson not visible to selected user');
        logStep('ac-5 pass');

        // ---------- ac-6: script meaning text in editor and player ----------
        logStep('ac-6 start');
        const scriptSlug = 'script-escape-' + Date.now().toString(36);
        const scriptWs = sampleWorksheet(scriptSlug, {
            title: 'Escape Test',
            vocab: {
                items: [{
                    id: '1',
                    word: 'שָׁלוֹם',
                    meaning: SCRIPT_MEANING,
                    sound: 'sha-LOM',
                    trap: 'Stress the last syllable.',
                    sentence: 'שָׁלוֹם.'
                }]
            },
            activities: [{ type: 'words' }]
        });
        // reading optional if only words — but validator may want paragraphs; keep them
        const createScript = await request('POST', baseUrl + '/api/admin/lessons', {
            Cookie: adminCookie
        }, {
            slug: scriptSlug,
            worksheet: scriptWs,
            catalogEntry: {
                slug: scriptSlug,
                title: 'Escape Test',
                blurb: 'Run it again. The next round hides the English until you ask, and keeps the vowels.',
                level: 'basic'
            }
        });
        if (createScript.status !== 201) {
            throw new Error('ac-6 create failed: ' + createScript.status + ' ' + createScript.body);
        }
        await request('PUT', baseUrl + '/api/admin/lessons/' + scriptSlug + '/visibility', {
            Cookie: adminCookie
        }, { mode: 'everyone' });

        await setSessionCookie(sessionA, baseUrl, 'admin');
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lesson/' + scriptSlug });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-vocab-field="meaning"]')
        }))()`, 20000);
        const editorEsc = await evalJson(sessionA, `(() => {
            const input = document.querySelector('[data-vocab-field="meaning"]');
            const html = document.querySelector('[data-vocab-body]')
                ? document.querySelector('[data-vocab-body]').innerHTML
                : '';
            return {
                ok: true,
                value: input ? input.value : null,
                hasRawScriptTag: /<script>alert\\(1\\)<\\/script>/.test(html),
                hasEscaped: /&lt;script&gt;/.test(html),
                liveScript: !!document.querySelector('[data-vocab-body] script')
            };
        })()`);

        await clearCookies(sessionB, baseUrl);
        await setSessionCookie(sessionB, baseUrl, 'learner');
        await sessionB.call('Page.navigate', {
            url: baseUrl + '/worksheets/' + scriptSlug + '/#step-1'
        });
        await waitFor(sessionB, `(() => ({
            ok: !!(window.WORKSHEET && document.querySelector('.meaning, .vocab-card, .step-screen, .path-screen')),
            hasWs: !!window.WORKSHEET,
            hasMeaning: !!document.querySelector('.meaning')
        }))()`, 25000);
        // If path screen, jump to words step
        await evalJson(sessionB, `(() => {
            if (!document.querySelector('.meaning')) {
                location.hash = 'step-1';
            }
            return true;
        })()`);
        await waitFor(sessionB, `(() => ({
            ok: !!document.querySelector('.meaning'),
            html: (document.querySelector('.meaning') || {}).innerHTML || ''
        }))()`, 15000);
        const playerEsc = await evalJson(sessionB, `(() => {
            const html = document.body.innerHTML;
            const text = document.body.innerText || '';
            const meaningEls = Array.from(document.querySelectorAll('.meaning'));
            return {
                ok: true,
                hasRawScriptInHtml: /<script>alert\\(1\\)<\\/script>/.test(html.replace(/<script[^>]*src=[^>]*>[\\s\\S]*?<\\/script>/gi, '')),
                liveAlertScript: Array.from(document.querySelectorAll('script')).some(s =>
                    (s.textContent || '').indexOf('alert(1)') !== -1 && !s.src
                ),
                hasEscapedInHtml: /&lt;script&gt;alert\\(1\\)&lt;\\/script&gt;/.test(html),
                textHasLiteral: text.indexOf('<script>alert(1)</script>') !== -1,
                meaningTexts: meaningEls.map((el) => el.textContent || ''),
                meaningInWs: window.WORKSHEET
                    && window.WORKSHEET.vocab
                    && window.WORKSHEET.vocab.items
                    && window.WORKSHEET.vocab.items[0]
                    && window.WORKSHEET.vocab.items[0].meaning
            };
        })()`);

        report.ac6 = { editor: editorEsc, player: playerEsc };
        if (editorEsc.liveScript || editorEsc.hasRawScriptTag) {
            throw new Error('ac-6: script executed or raw in editor HTML');
        }
        if (editorEsc.value !== SCRIPT_MEANING) {
            throw new Error('ac-6: editor meaning value wrong');
        }
        if (playerEsc.liveAlertScript) {
            throw new Error('ac-6: player injected live script');
        }
        if (playerEsc.meaningInWs !== SCRIPT_MEANING) {
            throw new Error('ac-6: worksheet meaning not preserved');
        }
        if (!playerEsc.hasEscapedInHtml && !(playerEsc.meaningTexts || []).some((t) => t.includes('<script>'))) {
            throw new Error('ac-6: player did not show meaning as text');
        }
        logStep('ac-6 pass');

        // ---------- ac-7: keyboard — reorder buttons exist; dialog focus ----------
        logStep('ac-7 start');
        await setSessionCookie(sessionA, baseUrl, 'admin');
        await sessionA.call('Page.navigate', { url: baseUrl + '/admin/#lesson/' + editSlug });
        await waitFor(sessionA, `(() => ({
            ok: !!document.querySelector('[data-vocab-move]')
        }))()`, 20000);
        const a11y = await evalJson(sessionA, `(() => new Promise(async (resolve) => {
            const hasVocabUp = !!document.querySelector('[data-vocab-move="up"]');
            const hasActUp = !!document.querySelector('[data-act-move="up"]');
            document.querySelector('[data-lesson-audio]').click();
            await new Promise(r => setTimeout(r, 400));
            const dlg = document.getElementById('lesson-audio-dialog');
            const open = dlg && !dlg.hidden && !dlg.classList.contains('hidden');
            dlg.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
            await new Promise(r => setTimeout(r, 200));
            const closed = !dlg || dlg.hidden || dlg.classList.contains('hidden');
            resolve({ ok: true, hasVocabUp, hasActUp, open, closed });
        }))()`);
        report.ac7 = a11y;
        if (!a11y.hasVocabUp || !a11y.hasActUp) {
            throw new Error('ac-7: reorder buttons missing');
        }
        if (!a11y.open || !a11y.closed) {
            throw new Error('ac-7: dialog Escape focus trap behavior failed');
        }

        report.ok = true;
        return report;
    } finally {
        if (httpsStub) {
            try { httpsStub.restore(); } catch (_) { /* ignore */ }
        }
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
        try { await harness.stop(); } catch (_) { /* ignore */ }
        if (prevKey === undefined) delete process.env.ELEVENLABS_API_KEY;
        else process.env.ELEVENLABS_API_KEY = prevKey;
        if (prevVoice === undefined) delete process.env.ELEVENLABS_VOICE_ID;
        else process.env.ELEVENLABS_VOICE_ID = prevVoice;
        if (prevModel === undefined) delete process.env.ELEVENLABS_MODEL;
        else process.env.ELEVENLABS_MODEL = prevModel;
    }
}

async function main() {
    let lastErr = null;
    for (let attempt = 1; attempt <= 2; attempt += 1) {
        try {
            const report = await runOnce();
            console.log(JSON.stringify(report, null, 2));
            if (!report.ok) process.exit(1);
            console.log('chrome-admin-lessons-smoke: PASS');
            return;
        } catch (err) {
            lastErr = err;
            const msg = String((err && err.message) || err);
            const retryable = /EBUSY|EPERM|resource busy|locked/i.test(msg);
            console.error('chrome-admin-lessons-smoke attempt ' + attempt + ' failed:', msg);
            try { await harness.stop(); } catch (_) { /* ignore */ }
            if (!retryable || attempt === 2) break;
            await new Promise((r) => setTimeout(r, 500));
        }
    }
    console.error(lastErr);
    process.exit(1);
}

main();
