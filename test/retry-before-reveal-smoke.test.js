/**
 * Chrome smoke: retry-before-reveal for pattern / fill / listen-pick (INIT-001/SPEC-006).
 * Starts the local web server, drives headless Chrome, asserts wrong-once / wrong-twice /
 * first-attempt miss recording, and activityState attempt persistence.
 */
const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const harness = require('./helpers/signed-in-server');

const root = path.resolve(__dirname, '..');
const CHROME = process.env.CHROME_PATH
    || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
// Sign-in is required for every page (INIT-002), so this runs against the signed-in
// test server on an ephemeral port and sets the learner's session cookie in Chrome.
let BASE = '';

function cookieParts(cookieHeader) {
    const eq = cookieHeader.indexOf('=');
    return { name: cookieHeader.slice(0, eq), value: decodeURIComponent(cookieHeader.slice(eq + 1)) };
}

let chromeProc = null;
let userData = null;
let pageWs = null;

function get(url) {
    return new Promise((resolve, reject) => {
        http.get(url, res => {
            let data = '';
            res.on('data', c => { data += c; });
            res.on('end', () => resolve({ status: res.statusCode, body: data }));
        }).on('error', reject);
    });
}

function cdp(wsUrl, method, params) {
    return new Promise((resolve, reject) => {
        const ws = new WebSocket(wsUrl);
        const id = 1;
        const timer = setTimeout(() => {
            try { ws.close(); } catch (err) { /* ignore */ }
            reject(new Error('CDP timeout for ' + method));
        }, 25000);
        ws.addEventListener('open', () => {
            ws.send(JSON.stringify({ id, method, params: params || {} }));
        });
        ws.addEventListener('message', event => {
            const msg = JSON.parse(event.data);
            if (msg.id !== id) return;
            clearTimeout(timer);
            ws.close();
            if (msg.error) reject(new Error(JSON.stringify(msg.error)));
            else resolve(msg.result);
        });
        ws.addEventListener('error', () => {
            clearTimeout(timer);
            reject(new Error('WebSocket error talking to Chrome'));
        });
    });
}

async function evaluate(expression) {
    const result = await cdp(pageWs, 'Runtime.evaluate', {
        awaitPromise: true,
        returnByValue: true,
        expression
    });
    if (result.exceptionDetails) {
        const ex = result.exceptionDetails;
        const text = ex.text
            || (ex.exception && (ex.exception.description || ex.exception.value))
            || JSON.stringify(ex);
        throw new Error(String(text));
    }
    return result.result && result.result.value;
}

async function openLesson(slug, stepHash) {
    const url = BASE + '/worksheets/' + slug + '/index.html' + (stepHash || '');
    await cdp(pageWs, 'Page.navigate', { url });
    await new Promise(r => setTimeout(r, 1400));
    const ready = await evaluate(`(() => new Promise((resolve, reject) => {
        const deadline = Date.now() + 12000;
        const wait = () => {
            if (window.HebrewPlayer && window.WORKSHEET && document.getElementById('step-body')) {
                return resolve({
                    stepType: document.querySelector('.step-screen')?.dataset?.stepType || '',
                    hasReview: !!(window.HebrewReview && window.HebrewReview.record)
                });
            }
            if (Date.now() > deadline) return reject(new Error('player did not boot'));
            setTimeout(wait, 100);
        };
        wait();
    }))()`);
    assert.ok(ready && ready.stepType, 'player boot failed');
    return ready;
}

function sleep(ms) {
    return new Promise(r => setTimeout(r, ms));
}

describe('retry-before-reveal smoke (SPEC-006)', () => {
    before(async () => {
        if (!fs.existsSync(CHROME)) {
            throw new Error('Chrome not found at ' + CHROME + ' (set CHROME_PATH)');
        }
        const started = await harness.start({ lessons: 'real' });
        BASE = started.baseUrl;

        userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-retry-'));
        const debugPort = 9300 + Math.floor(Math.random() * 200);
        chromeProc = spawn(CHROME, [
            '--headless=new',
            '--disable-gpu',
            '--no-first-run',
            '--no-default-browser-check',
            '--user-data-dir=' + userData,
            '--remote-debugging-port=' + debugPort,
            '--remote-allow-origins=*',
            'about:blank'
        ], { stdio: 'ignore' });

        let page = null;
        for (let i = 0; i < 40; i += 1) {
            try {
                const list = await get('http://127.0.0.1:' + debugPort + '/json/list');
                const pages = JSON.parse(list.body);
                page = pages.find(p => p.type === 'page');
                if (page && page.webSocketDebuggerUrl) break;
            } catch (err) { /* retry */ }
            await sleep(250);
        }
        if (!page) throw new Error('No Chrome page');
        pageWs = page.webSocketDebuggerUrl;
        const ck = cookieParts(harness.cookieFor('learner'));
        await cdp(pageWs, 'Network.setCookie', { name: ck.name, value: ck.value, url: BASE, path: '/' });
        await cdp(pageWs, 'Page.enable', {});
        await cdp(pageWs, 'Runtime.enable', {});
    });

    after(async () => {
        if (chromeProc) try { chromeProc.kill(); } catch (err) { /* ignore */ }
        try { await harness.stop(); } catch (err) { /* ignore */ }
        if (userData) {
            try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        }
    });

    it('listen-pick: wrong-once tries again; wrong-twice reveals; miss recorded', async () => {
        const boot = await openLesson('my-morning', '#step-3');
        assert.equal(boot.stepType, 'listen-pick');

        const once = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const listen = root && root._listen;
            if (!listen) throw new Error('listen state missing');
            const item = (window.WORKSHEET.vocab.items || []).find(i => String(i.id) === String(listen.queue[listen.index]));
            if (!item) throw new Error('no current item');
            const expected = listen.mode === 'form' ? item.word : item.meaning;
            const wrongBtn = [...root.querySelectorAll('[data-action="listen-pick"]')]
                .find(b => {
                    const v = b.dataset.value;
                    return listen.mode === 'form'
                        ? window.HebrewUtils.normalizeHebrew(v) !== window.HebrewUtils.normalizeHebrew(expected)
                        : String(v).toLowerCase() !== String(expected).toLowerCase();
                });
            if (!wrongBtn) throw new Error('no wrong choice');
            window.__retrySmoke = { itemId: String(item.id), word: item.word, expected: expected };
            wrongBtn.click();
            return {
                hint: (root.querySelector('[data-listen-hint]') || {}).textContent || '',
                correctMarked: !!root.querySelector('.choice-card.correct'),
                attempts: listen.attempts[String(item.id)] || 0
            };
        })()`);

        assert.match(once.hint, /Try again/);
        assert.equal(once.correctMarked, false);
        assert.equal(once.attempts, 1);

        await sleep(500);

        const twice = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const listen = root._listen;
            const meta = window.__retrySmoke;
            const expected = meta.expected;
            const wrong2 = [...root.querySelectorAll('[data-action="listen-pick"]')]
                .find(b => {
                    const v = b.dataset.value;
                    const isCorrect = listen.mode === 'form'
                        ? window.HebrewUtils.normalizeHebrew(v) === window.HebrewUtils.normalizeHebrew(expected)
                        : String(v).toLowerCase() === String(expected).toLowerCase();
                    return !isCorrect;
                });
            if (!wrong2) throw new Error('no second wrong choice');
            wrong2.click();
            return {
                correctMarked: !!root.querySelector('.choice-card.correct'),
                hint: (root.querySelector('[data-listen-hint]') || {}).textContent || '',
                attempts: listen.attempts[meta.itemId] || 0,
                hasSpeak: !!root.querySelector('[data-listen-hint] [data-speak]')
            };
        })()`);

        assert.equal(twice.correctMarked, true);
        assert.ok(twice.attempts >= 2);
        assert.match(twice.hint, /Try again/);

        await sleep(50);
        const miss = await evaluate(`(() => {
            const meta = window.__retrySmoke;
            const store = window.HebrewReview.load();
            const key = window.HebrewReview.keyFor(meta.word);
            const entry = store.words[key];
            return {
                key,
                lastResult: entry && entry.he_en && entry.he_en.lastResult
            };
        })()`);
        assert.equal(miss.lastResult, 'miss');
    });

    it('pattern: wrong-once hides answer; wrong-twice reveals; attempts in activityState', async () => {
        const boot = await openLesson('my-morning', '#step-7');
        assert.equal(boot.stepType, 'pattern');

        const result = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            if (!root || !root._grammar) throw new Error('pattern state missing');
            const item = (window.WORKSHEET.grammar.items || [])[0];
            if (!item) throw new Error('no grammar item');
            const id = String(item.id);
            const wrap = root.querySelector('[data-grammar="' + id + '"]');
            const wrongIdx = item.choices.findIndex((_, i) => i !== item.answer);
            const wrongBtn = wrap.querySelector('[data-index="' + wrongIdx + '"]');
            const checkBtn = document.querySelector('[data-action="check"]');
            if (!checkBtn) throw new Error('check button missing');
            wrongBtn.click();
            checkBtn.click();
            const once = {
                text: (root.querySelector('[data-result="' + id + '"]') || {}).textContent || '',
                correctMarked: !!wrap.querySelector('.choice-card.correct'),
                attempts: root._grammar.attempts[id] || 0
            };
            checkBtn.click();
            const twice = {
                html: (root.querySelector('[data-result="' + id + '"]') || {}).innerHTML || '',
                correctMarked: !!wrap.querySelector('.choice-card.correct'),
                attempts: root._grammar.attempts[id] || 0,
                hasSpeak: !!root.querySelector('[data-result="' + id + '"] [data-speak]')
            };
            const state = root._activity.collect({
                sid: (x) => String(x),
                ws: window.WORKSHEET
            }, root);
            const words = [];
            const pointed = window.HebrewUtils.normalizeHebrew(item.pointed || '');
            (window.WORKSHEET.vocab.items || []).forEach(v => {
                if (v.word && pointed.indexOf(window.HebrewUtils.normalizeHebrew(v.word)) !== -1) words.push(v.word);
            });
            const store = window.HebrewReview.load();
            const missAny = words.some(w => {
                const e = store.words[window.HebrewReview.keyFor(w)];
                return e && e.en_he && e.en_he.lastResult === 'miss';
            });
            return { once, twice, attemptsPersisted: state.attempts[id], missAny, words };
        })()`);

        assert.match(result.once.text, /Try again/);
        assert.equal(result.once.correctMarked, false);
        assert.equal(result.once.attempts, 1);
        assert.equal(result.twice.correctMarked, true);
        assert.ok(result.twice.attempts >= 2);
        assert.equal(result.twice.hasSpeak, true);
        assert.equal(result.attemptsPersisted, result.twice.attempts);
        assert.equal(result.missAny, true);

        // ac-4: reload must restore revealed pattern state from activityState
        await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const collected = root._activity.collect({
                sid: (x) => String(x),
                ws: window.WORKSHEET
            }, root);
            const key = 'hebrew-worksheet:' + window.WORKSHEET.id;
            const raw = JSON.parse(localStorage.getItem(key) || '{}');
            raw.activityState = raw.activityState || {};
            raw.activityState.pattern = collected;
            localStorage.setItem(key, JSON.stringify(raw));
            return collected.attempts['1'];
        })()`);
        await openLesson('my-morning', '#step-7');
        const restored = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const id = '1';
            const wrap = root.querySelector('[data-grammar="' + id + '"]');
            return {
                attempts: root._grammar && root._grammar.attempts[id],
                correctMarked: !!(wrap && wrap.querySelector('.choice-card.correct')),
                hasSpeak: !!root.querySelector('[data-result="' + id + '"] [data-speak]'),
                resultText: (root.querySelector('[data-result="' + id + '"]') || {}).textContent || ''
            };
        })()`);
        assert.ok(restored.attempts >= 2);
        assert.equal(restored.correctMarked, true);
        assert.equal(restored.hasSpeak, true);
    });

    it('listen-pick restores attempts after reload (ac-4)', async () => {
        await openLesson('my-morning', '#step-3');
        await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const listen = root._listen;
            const itemId = String(listen.queue[listen.index]);
            const item = (window.WORKSHEET.vocab.items || []).find(i => String(i.id) === itemId);
            const expected = listen.mode === 'form' ? item.word : item.meaning;
            listen.attempts[itemId] = 2;
            listen.optionsByItem[itemId] = listen.optionsByItem[itemId] || [expected, 'wrong-a', 'wrong-b'];
            const key = 'hebrew-worksheet:' + window.WORKSHEET.id;
            const raw = JSON.parse(localStorage.getItem(key) || '{}');
            raw.activityState = raw.activityState || {};
            raw.activityState['listen-pick'] = {
                index: listen.index,
                score: listen.score,
                queue: listen.queue,
                attempts: listen.attempts,
                recorded: listen.recorded,
                optionsByItem: listen.optionsByItem
            };
            localStorage.setItem(key, JSON.stringify(raw));
            window.__listenRestore = { itemId, expected };
            return true;
        })()`);
        await openLesson('my-morning', '#step-3');
        const restored = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const meta = window.__listenRestore || {};
            // __listenRestore is lost on reload — read from storage
            const key = 'hebrew-worksheet:' + window.WORKSHEET.id;
            const raw = JSON.parse(localStorage.getItem(key) || '{}');
            const saved = raw.activityState && raw.activityState['listen-pick'];
            const itemId = saved && saved.queue ? String(saved.queue[saved.index]) : '';
            const item = (window.WORKSHEET.vocab.items || []).find(i => String(i.id) === itemId);
            const expected = root._listen.mode === 'form' ? item.word : item.meaning;
            return {
                attempts: root._listen.attempts[itemId] || 0,
                correctMarked: !!root.querySelector('.choice-card.correct'),
                hint: (root.querySelector('[data-listen-hint]') || {}).textContent || '',
                hasSpeak: !!root.querySelector('[data-listen-hint] [data-speak]'),
                expected
            };
        })()`);
        assert.ok(restored.attempts >= 2);
        assert.equal(restored.correctMarked, true);
        assert.match(restored.hint, /Try again/);
        assert.equal(restored.hasSpeak, true);
    });

    it('fill: wrong-once try-again; wrong-twice reveals answer audio; miss for vocab word', async () => {
        // my-morning ships fill.sentences but omits fill from activities[]; mount the
        // registered fill activity against that data so the smoke stays on one lesson.
        const boot = await openLesson('my-morning', '#step-1');
        assert.ok(boot.stepType);

        const result = await evaluate(`(() => {
            const def = window.HebrewPlayer.registry.fill;
            if (!def) throw new Error('fill activity not registered');
            if (!(window.WORKSHEET.fill && window.WORKSHEET.fill.sentences || []).length) {
                throw new Error('my-morning missing fill.sentences');
            }
            const host = document.createElement('div');
            host.id = 'fill-smoke-host';
            document.body.appendChild(host);
            const ctx = {
                ws: window.WORKSHEET,
                esc: window.HebrewUtils.escapeHtml,
                bi: window.HebrewUtils.bi,
                sid: window.HebrewUtils.sid,
                speakButton: (text, audio) =>
                    '<button type="button" data-speak="' + window.HebrewUtils.escapeHtml(text || '')
                    + '" data-audio="' + window.HebrewUtils.escapeHtml(audio || '') + '"></button>',
                Speech: window.HebrewSpeech,
                review: window.HebrewReview,
                levelDefaults: (window.HEBREW_LEVEL_DEFAULTS && window.HEBREW_LEVEL_DEFAULTS.basic) || { bank: 'open' },
                stepIndex: 99,
                markComplete: () => {},
                save: () => {}
            };
            const config = Object.assign({}, def.defaults('basic', ctx.levelDefaults), { type: 'fill', bank: 'open' });
            const state = { answers: [], attempts: {}, recorded: {}, bankOpen: true };
            host.innerHTML = def.render(ctx, config, state);
            def.mount(ctx, host, config, state);
            host._activity = def;
            host._config = config;

            const s = window.WORKSHEET.fill.sentences[0];
            const id = String(s.id);
            const input = host.querySelector('#fill-input-' + id);
            input.removeAttribute('readonly');
            input.value = 'zzzz';
            host._fill.active = id;

            const checkAction = { dataset: { action: 'check' } };
            def.onClick(ctx, host, { target: host }, checkAction);
            const once = {
                text: (host.querySelector('[data-fill-feedback="' + id + '"]') || {}).textContent || '',
                attempts: host._fill.attempts[id] || 0,
                inputCorrect: input.classList.contains('correct')
            };
            def.onClick(ctx, host, { target: host }, checkAction);
            const fb = host.querySelector('[data-fill-feedback="' + id + '"]');
            const twice = {
                text: fb ? fb.textContent : '',
                attempts: host._fill.attempts[id] || 0,
                hasSpeak: !!(fb && fb.querySelector('[data-speak]'))
            };
            const word = (window.WORKSHEET.vocab.items || []).find(v =>
                window.HebrewUtils.normalizeHebrew(v.word) === window.HebrewUtils.normalizeHebrew(s.answer));
            const store = window.HebrewReview.load();
            const key = word ? window.HebrewReview.keyFor(word.word) : null;
            const entry = key && store.words[key];
            const dir = entry && entry.en_he;
            const collected = def.collect(ctx, host);
            // ac-4: remount from persisted activityState (simulates reload restore)
            host.innerHTML = def.render(ctx, config, collected);
            def.mount(ctx, host, config, collected);
            const restoredFb = host.querySelector('[data-fill-feedback="' + id + '"]');
            const restored = {
                attempts: host._fill.attempts[id] || 0,
                text: restoredFb ? restoredFb.textContent : '',
                hasSpeak: !!(restoredFb && restoredFb.querySelector('[data-speak]')),
                inputWrong: host.querySelector('#fill-input-' + id).classList.contains('wrong')
            };
            host.remove();
            return {
                once,
                twice,
                miss: !!(dir && dir.lastResult === 'miss'),
                word: word && word.word,
                attemptsPersisted: collected.attempts[id],
                restored
            };
        })()`);

        assert.match(result.once.text, /Try again/);
        assert.equal(result.once.inputCorrect, false);
        assert.equal(result.once.attempts, 1);
        assert.ok(result.twice.attempts >= 2);
        assert.match(result.twice.text, /Try again/);
        assert.equal(result.miss, true);
        assert.equal(result.attemptsPersisted, result.twice.attempts);
        assert.equal(result.twice.hasSpeak, true);
        assert.ok(result.restored.attempts >= 2);
        assert.match(result.restored.text, /Try again/);
        assert.equal(result.restored.hasSpeak, true);
        assert.equal(result.restored.inputWrong, true);
    });
});
