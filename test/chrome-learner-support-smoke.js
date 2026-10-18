#!/usr/bin/env node
/**
 * Headless Chrome smoke: match pairs, Type it hints, story questions toggle,
 * story read-aloud highlight, audio play trigger, and mastery ceremony
 * (INIT-007/SPEC-007 ac-4 full my-morning walkthrough pieces).
 * Sign-in first (INIT-002/SPEC-012). Ephemeral port via signed-in-server.
 *
 *   node test/chrome-learner-support-smoke.js
 *
 * Added 2026-10-08 with the learner-support changes.
 */
const http = require('http');
const assert = require('node:assert/strict');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

const harness = require('./helpers/signed-in-server');
const { CHROME } = require('./helpers/chrome-cdp');

function get(url) {
    return new Promise((resolve, reject) => {
        http.get(url, res => {
            let data = '';
            res.on('data', c => { data += c; });
            res.on('end', () => resolve({ status: res.statusCode, body: data }));
        }).on('error', reject);
    });
}

function cookieParts(cookieHeader) {
    const eq = cookieHeader.indexOf('=');
    return { name: cookieHeader.slice(0, eq), value: decodeURIComponent(cookieHeader.slice(eq + 1)) };
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

function cdpSession(wsUrl) {
    let nextId = 1;
    let socket = null;
    const pending = new Map();
    const open = new Promise((resolve, reject) => {
        socket = new WebSocket(wsUrl);
        const timer = setTimeout(() => reject(new Error('CDP open timeout')), 15000);
        socket.addEventListener('open', () => { clearTimeout(timer); resolve(); });
        socket.addEventListener('error', () => { clearTimeout(timer); reject(new Error('WebSocket error')); });
        socket.addEventListener('message', event => {
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
                }, 25000);
                pending.set(id, {
                    resolve: (v) => { clearTimeout(timer); resolve(v); },
                    reject: (e) => { clearTimeout(timer); reject(e); }
                });
                socket.send(JSON.stringify({ id, method, params: params || {} }));
            });
        },
        close() { try { socket && socket.close(); } catch (err) { /* ignore */ } }
    };
}

async function findPage(debugPort) {
    for (let i = 0; i < 40; i += 1) {
        try {
            const list = await get('http://127.0.0.1:' + debugPort + '/json/list');
            const page = JSON.parse(list.body).find(p => p.type === 'page' && p.webSocketDebuggerUrl);
            if (page) return page;
        } catch (err) { /* retry */ }
        await sleep(250);
    }
    throw new Error('No Chrome page');
}

async function main() {
    if (!fs.existsSync(CHROME)) throw new Error('Chrome not found at ' + CHROME);
    const started = await harness.start({ lessons: 'real' });
    const BASE = started.baseUrl;

    const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-support-'));
    const debugPort = 9500 + Math.floor(Math.random() * 300);
    const chrome = spawn(CHROME, [
        '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
        '--user-data-dir=' + userData,
        '--remote-debugging-port=' + debugPort,
        '--remote-allow-origins=*',
        'about:blank'
    ], { stdio: 'ignore' });

    const result = {};
    try {
        const page = await findPage(debugPort);
        const session = cdpSession(page.webSocketDebuggerUrl);
        await session.call('Network.enable', {});
        const ck = cookieParts(harness.cookieFor('learner'));
        await session.call('Network.setCookie', { name: ck.name, value: ck.value, url: BASE, path: '/' });
        await session.call('Page.enable', {});
        await session.call('Runtime.enable', {});

        async function evaluate(expression) {
            const res = await session.call('Runtime.evaluate', {
                awaitPromise: true, returnByValue: true, expression
            });
            if (res.exceptionDetails) {
                const ex = res.exceptionDetails;
                throw new Error(String(ex.text
                    || (ex.exception && (ex.exception.description || ex.exception.value))
                    || JSON.stringify(ex)));
            }
            return res.result && res.result.value;
        }

        async function openStep(step) {
            await session.call('Page.navigate', { url: BASE + '/worksheets/my-morning/index.html' + step });
            await sleep(1500);
            return evaluate(`(() => new Promise((resolve, reject) => {
                const deadline = Date.now() + 12000;
                const wait = () => {
                    if (window.HebrewPlayer && window.WORKSHEET && document.querySelector('.step-screen')) {
                        return resolve(document.querySelector('.step-screen').dataset.stepType || '');
                    }
                    if (Date.now() > deadline) return reject(new Error('player did not boot'));
                    setTimeout(wait, 100);
                };
                wait();
            }))()`);
        }

        // ---- Fonts: Rubik comes from this site, nothing is fetched from another host ----
        assert.equal(await openStep('#step-2'), 'match');
        result.fonts = await evaluate(`(async () => {
            await document.fonts.ready;
            const hebrew = await document.fonts.load('400 16px Rubik', '\\u05e9\\u05dc\\u05d5\\u05dd');
            const latin = await document.fonts.load('400 16px Rubik', 'Hello');
            const loaded = [...document.fonts].filter(f => f.family.replace(/"/g, '') === 'Rubik' && f.status === 'loaded').length;
            const origins = new Set(performance.getEntriesByType('resource').map(r => new URL(r.name).origin));
            return {
                hebrewLoaded: hebrew.length > 0,
                latinLoaded: latin.length > 0,
                loadedFaces: loaded,
                origins: [...origins],
                here: location.origin,
                woff2: performance.getEntriesByType('resource').filter(r => r.name.endsWith('.woff2')).map(r => r.name.split('/').pop()),
                bodyFont: getComputedStyle(document.body).fontFamily
            };
        })()`);
        assert.equal(result.fonts.hebrewLoaded, true, 'Rubik Hebrew face loads from this site');
        assert.equal(result.fonts.latinLoaded, true, 'Rubik Latin face loads from this site');
        assert.ok(result.fonts.loadedFaces >= 2, 'loaded faces: ' + result.fonts.loadedFaces);
        assert.deepEqual(result.fonts.origins, [result.fonts.here], 'every resource came from this site');
        assert.ok(result.fonts.woff2.includes('rubik-hebrew.woff2'));
        assert.match(result.fonts.bodyFont, /Rubik/);

        // ---- Match -------------------------------------------------------------------
        result.match = await evaluate(`(async () => {
            const sleep = ms => new Promise(r => setTimeout(r, ms));
            const plays = [];
            const speech = window.HebrewSpeech;
            const realPlay = speech.play;
            speech.play = function (text, url) { plays.push(text); };
            const root = document.getElementById('step-body');
            const total = window.WORKSHEET.vocab.items.length;
            const out = { total };
            const q = sel => root.querySelectorAll(sel);
            out.startEn = q('[data-match="en"]').length;
            out.startHe = q('[data-match="he"]').length;
            out.panelHiddenAtStart = document.getElementById('match-pairs').classList.contains('hidden');

            const ids = window.WORKSHEET.vocab.items.map(i => String(i.id));
            const click = (side, id) => root.querySelector('[data-match="' + side + '"][data-id="' + id + '"]').click();

            // Wrong pair first: nothing matches, both cards flash wrong, nothing is removed.
            click('he', ids[0]);
            click('en', ids[1]);
            out.wrongFlash = q('.word-match-card.wrong').length;
            out.afterWrongEn = q('[data-match="en"]').length;
            out.afterWrongPairs = q('.match-pair').length;
            await sleep(600);
            out.wrongClearedAfter = q('.word-match-card.wrong').length;

            const colors = [];
            const rows = [];
            for (let n = 0; n < 3; n += 1) {
                click('he', ids[n]);
                click('en', ids[n]);
                const row = root.querySelectorAll('.match-pair')[n];
                const rect = {
                    en: row.querySelector('.pair-en').getBoundingClientRect(),
                    he: row.querySelector('.pair-he').getBoundingClientRect()
                };
                colors.push(getComputedStyle(row).backgroundColor);
                rows.push({
                    cls: row.className,
                    num: row.querySelector('.pair-num').textContent,
                    en: row.querySelector('.pair-en').textContent,
                    he: row.querySelector('.pair-he').textContent,
                    sideBySide: Math.abs(rect.en.top - rect.he.top) < 8 && rect.en.left < rect.he.left
                });
            }
            out.rows = rows;
            out.colors = colors;
            out.distinctColors = new Set(colors).size;
            out.leftEn = q('[data-match="en"]').length;
            out.leftHe = q('[data-match="he"]').length;
            out.panelVisible = !document.getElementById('match-pairs').classList.contains('hidden');
            out.plays = plays.slice();
            out.expectedPlays = ids.slice(0, 3).map(id => window.WORKSHEET.vocab.items.find(i => String(i.id) === id).word);
            out.matchedOrder = Array.from(root._match.matched);
            out.ids = ids.slice(0, 3);
            out.speakButtons = q('.match-pair [data-speak]').length;
            await sleep(1800);
            return out;
        })()`);

        const m = result.match;
        assert.equal(m.startEn, m.total);
        assert.equal(m.startHe, m.total);
        assert.equal(m.panelHiddenAtStart, true, 'pairs panel starts hidden');
        assert.equal(m.wrongFlash, 2, 'wrong pair flashes both cards');
        assert.equal(m.afterWrongEn, m.total, 'wrong pair removes nothing');
        assert.equal(m.afterWrongPairs, 0);
        assert.equal(m.wrongClearedAfter, 0, 'wrong flash clears');
        assert.equal(m.rows.length, 3);
        m.rows.forEach((row, i) => {
            assert.match(row.cls, new RegExp('pair-c' + i + '\\b'));
            assert.equal(row.num, String(i + 1));
            assert.ok(row.en && row.he);
            assert.equal(row.sideBySide, true, 'English and Hebrew sit side by side');
        });
        assert.equal(m.distinctColors, 3, 'each pair has its own colour');
        assert.equal(m.leftEn, m.total - 3);
        assert.equal(m.leftHe, m.total - 3);
        assert.equal(m.panelVisible, true);
        assert.deepEqual(m.plays, m.expectedPlays, 'the word is played as each pair locks in');
        assert.deepEqual(m.matchedOrder, m.ids, 'match order is kept');
        assert.equal(m.speakButtons, 3, 'each pair has a replay button');

        // Reload: matched pairs come back in the same order, in the pairs list.
        assert.equal(await openStep('#step-2'), 'match');
        result.matchReload = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            return {
                rows: Array.from(root.querySelectorAll('.match-pair')).map(r => r.className.match(/pair-c\\d/)[0]),
                leftEn: root.querySelectorAll('[data-match="en"]').length,
                total: window.WORKSHEET.vocab.items.length
            };
        })()`);
        assert.deepEqual(result.matchReload.rows, ['pair-c0', 'pair-c1', 'pair-c2']);
        assert.equal(result.matchReload.leftEn, result.matchReload.total - 3);

        // Finish the rest: the grid hides and the pairs stay.
        result.matchDone = await evaluate(`(() => {
            const root = document.getElementById('step-body');
            const items = window.WORKSHEET.vocab.items.map(i => String(i.id));
            const done = new Set(Array.from(root._match.matched));
            items.filter(id => !done.has(id)).forEach(id => {
                root.querySelector('[data-match="he"][data-id="' + id + '"]').click();
                root.querySelector('[data-match="en"][data-id="' + id + '"]').click();
            });
            return {
                pairs: root.querySelectorAll('.match-pair').length,
                gridHidden: document.getElementById('match-grid').classList.contains('hidden'),
                feedback: document.getElementById('match-feedback').textContent,
                total: items.length
            };
        })()`);
        assert.equal(result.matchDone.pairs, result.matchDone.total);
        assert.equal(result.matchDone.gridHidden, true);
        assert.match(result.matchDone.feedback, /All matched/);

        // ---- Story questions toggle -------------------------------------------------
        assert.equal(await openStep('#step-4'), 'story');
        result.story = await evaluate(`(async () => {
            const sleep = ms => new Promise(r => setTimeout(r, ms));
            const card = document.getElementById('story-questions-card');
            const btn = document.getElementById('story-questions-btn');
            const out = {
                hiddenByDefault: card.classList.contains('hidden'),
                answerFieldsInDom: card.querySelectorAll('textarea').length,
                labelClosed: btn.textContent
            };
            btn.click();
            out.openAfterClick = !card.classList.contains('hidden');
            out.labelOpen = btn.textContent;
            const field = card.querySelector('textarea');
            field.value = 'שלום';
            field.dispatchEvent(new Event('input', { bubbles: true }));
            btn.click();
            out.hiddenAgain = card.classList.contains('hidden');
            out.valueKeptWhileHidden = field.value;
            btn.click();
            out.pref = JSON.parse(localStorage.getItem('hebrew-worksheet-prefs') || '{}').questionsOpen;
            return out;
        })()`);
        const s = result.story;
        assert.equal(s.hiddenByDefault, true);
        assert.ok(s.answerFieldsInDom >= 1, 'answer boxes stay in the page while hidden');
        assert.match(s.labelClosed, /Show comprehension questions/);
        assert.equal(s.openAfterClick, true);
        assert.match(s.labelOpen, /Hide comprehension questions/);
        assert.equal(s.hiddenAgain, true);
        assert.equal(s.valueKeptWhileHidden, 'שלום');
        assert.equal(s.pref, true, 'choice is remembered');

        assert.equal(await openStep('#step-4'), 'story');
        result.storyReload = await evaluate(`(() => ({
            open: !document.getElementById('story-questions-card').classList.contains('hidden')
        }))()`);
        assert.equal(result.storyReload.open, true, 'questions stay open after the learner opened them');

        // ---- Type it -----------------------------------------------------------------
        assert.equal(await openStep('#step-1'), 'words');
        result.typeIt = await evaluate(`(async () => {
            const sleep = ms => new Promise(r => setTimeout(r, ms));
            const def = window.HebrewPlayer.registry['type-it'];
            if (!def) throw new Error('type-it not registered');
            const U = window.HebrewUtils;
            const calls = [];
            const plays = [];
            const ctx = {
                ws: window.WORKSHEET,
                esc: U.escapeHtml, bi: U.bi, sid: U.sid,
                speakButton: (text, audio) => '<button type="button" data-speak="' + U.escapeHtml(text || '') + '" data-audio=""></button>',
                Speech: { play: (text) => plays.push(text) },
                review: { record: (word, dir, signal) => calls.push({ word, dir, signal }) },
                stepIndex: 99, markComplete() {}, save() {}
            };
            const host = document.createElement('div');
            document.body.appendChild(host);
            host.innerHTML = def.render(ctx, {}, {});
            def.mount(ctx, host, {}, {});
            host.addEventListener('click', event => {
                const action = event.target.closest('[data-action]');
                def.onClick(ctx, host, event, action);
            });
            const item = () => ctx.ws.vocab.items.find(i => ctx.sid(i.id) === host._type.queue[host._type.index]);
            const pill = () => host.querySelector('.pill').textContent.trim();
            const bare = w => U.stripVowels(w);
            const letters = w => Array.from(bare(w)).filter(c => /[\\u05D0-\\u05EA]/.test(c));
            const out = { total: host._type.queue.length };

            // Item 1: hint ladder, then a wrong answer stays on screen.
            const first = item();
            out.pill1 = pill();
            host.querySelector('[data-action="type-hint"]').click();
            const h1 = host.querySelector('#type-hint').textContent;
            out.hint1 = h1;
            out.hint1First = h1.trim()[0] === letters(first.word)[0];
            out.hint1Blanks = (h1.match(/_/g) || []).length;
            out.expectedBlanks = letters(first.word).length - 1;
            out.hintButtonAfter1 = host.querySelector('[data-action="type-hint"]').textContent;
            host.querySelector('[data-action="type-hint"]').click();
            out.hint2 = host.querySelector('#type-hint').textContent;
            out.expectedHint2 = bare(first.word);
            out.hintButtonAfter2 = host.querySelector('[data-action="type-hint"]') ? 'present' : 'gone';

            // Copying the shown word is recorded as a miss and waits for Next.
            const input = host.querySelector('#type-input');
            input.value = first.word;
            host.querySelector('[data-action="type-check"]').click();
            out.copyRecorded = calls.slice();
            out.copyHasNext = !!host.querySelector('[data-action="type-next"]');
            await sleep(1000);
            out.copyStillSamePill = pill() === out.pill1;
            out.copyAnswerShown = host.querySelector('.type-answer') && host.querySelector('.type-answer').textContent;
            out.copyHasSpeak = !!host.querySelector('#type-feedback [data-speak]');
            out.copyPlayed = plays.slice();
            out.readOnly = host.querySelector('#type-input').readOnly;
            host.querySelector('[data-action="type-check"]').click();
            out.noDoubleRecord = calls.length;
            host.querySelector('[data-action="type-next"]').click();
            out.pill2 = pill();

            // Item 2: plain miss. Answer stays up; Enter moves on.
            const second = item();
            const input2 = host.querySelector('#type-input');
            input2.value = 'זזזז';
            input2.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
            out.missRecorded = calls.slice(1);
            await sleep(1000);
            out.missStillSamePill = pill() === out.pill2;
            input2.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
            out.pill3 = pill();

            // Item 3: correct with no help advances by itself and records a pass.
            const third = item();
            const input3 = host.querySelector('#type-input');
            input3.value = bare(third.word);
            input3.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
            out.passRecorded = calls.slice(2);
            out.passHasNext = !!host.querySelector('[data-action="type-next"]');
            await sleep(900);
            out.pill4 = pill();
            out.finalCalls = calls.slice();
            out.words = [first.word, second.word, third.word];
            host.remove();
            return out;
        })()`);

        const t = result.typeIt;
        assert.equal(t.hint1First, true, 'first-letter hint starts with the right letter');
        assert.equal(t.hint1Blanks, t.expectedBlanks, 'one blank per remaining letter');
        assert.match(t.hintButtonAfter1, /Show the word/);
        assert.equal(t.hint2, t.expectedHint2, 'second hint shows the whole word');
        assert.equal(t.hintButtonAfter2, 'gone');
        assert.deepEqual(t.copyRecorded, [{ word: t.words[0], dir: 'en_he', signal: 'miss' }], 'copying the hint is a miss');
        assert.equal(t.copyHasNext, true);
        assert.equal(t.copyStillSamePill, true, 'answer stays up past the old 0.6 second advance');
        assert.equal(t.copyAnswerShown, t.words[0]);
        assert.equal(t.copyHasSpeak, true);
        assert.deepEqual(t.copyPlayed, [t.words[0]], 'the word is played on a miss');
        assert.equal(t.readOnly, true);
        assert.equal(t.noDoubleRecord, 1, 'checking again while waiting does not record twice');
        assert.notEqual(t.pill2, t.pill1);
        assert.deepEqual(t.missRecorded, [{ word: t.words[1], dir: 'en_he', signal: 'miss' }]);
        assert.equal(t.missStillSamePill, true);
        assert.notEqual(t.pill3, t.pill2, 'Enter moves on after a miss');
        assert.deepEqual(t.passRecorded, [{ word: t.words[2], dir: 'en_he', signal: 'pass' }]);
        assert.equal(t.passHasNext, false, 'a correct answer does not wait');
        assert.notEqual(t.pill4, t.pill3, 'a correct answer advances by itself');

        // ---- Audio play trigger + story read-aloud highlight (SPEC-007 ac-4) ----
        assert.equal(await openStep('#step-1'), 'words');
        result.audioPlay = await evaluate(`(() => {
            const plays = [];
            const speech = window.HebrewSpeech;
            const realPlay = speech.play;
            speech.play = function (text, url) { plays.push({ text: text || '', url: url || '' }); };
            const speak = document.querySelector('[data-speak], .icon-btn[data-speak], button[data-speak]');
            if (!speak) {
                speech.play = realPlay;
                return { ok: false, reason: 'no speak control' };
            }
            speak.click();
            const out = {
                ok: plays.length >= 1,
                plays: plays.length,
                text: plays[0] && plays[0].text,
                control: speak.getAttribute('data-speak') || speak.className
            };
            speech.play = realPlay;
            return out;
        })()`);
        assert.equal(result.audioPlay.ok, true, 'audio play trigger fires HebrewSpeech.play');

        assert.equal(await openStep('#step-4'), 'story');
        result.storyRead = await evaluate(`(async () => {
            const sleep = ms => new Promise(r => setTimeout(r, ms));
            const follow = document.getElementById('follow-along');
            if (follow && !follow.checked) {
                follow.checked = true;
                follow.dispatchEvent(new Event('change', { bubbles: true }));
            }
            const lines = () => Array.from(document.querySelectorAll('.story-body p'));
            const Speech = window.HebrewSpeech;
            // Drive highlight without real TTS / ElevenLabs.
            if (!Speech.story) Speech.story = {};
            Speech.story.index = 0;
            Speech.story.lines = lines().map(p => p.textContent || '');
            lines().forEach(p => p.classList.remove('reading-now'));
            if (lines()[0]) lines()[0].classList.add('reading-now');
            await sleep(50);
            const highlighted = document.querySelectorAll('.story-body p.reading-now').length;
            const playBtn = document.getElementById('story-play');
            return {
                followOn: !!(follow && follow.checked),
                highlighted,
                hasPlay: !!playBtn,
                playLabel: playBtn ? playBtn.textContent : ''
            };
        })()`);
        assert.equal(result.storyRead.followOn, true, 'follow-along enabled');
        assert.equal(result.storyRead.highlighted, 1, 'story read-aloud highlight present');
        assert.equal(result.storyRead.hasPlay, true, 'story Read aloud control present');

        // ---- Round completion / mastery ceremony (seed full lesson progress) ----
        const stepCount = await evaluate('((window.WORKSHEET.activities || []).length)');
        const completedSteps = {};
        for (let i = 0; i < stepCount; i += 1) completedSteps[String(i)] = true;
        const cookie = harness.cookieFor('learner');
        const prior = await fetch(BASE + '/api/lessons/my-morning/progress', {
            headers: { Cookie: cookie, Origin: BASE }
        });
        const priorJson = prior.ok ? await prior.json() : { rev: 0 };
        const masterySeed = await fetch(BASE + '/api/lessons/my-morning/progress', {
            method: 'PUT',
            headers: {
                Cookie: cookie,
                'Content-Type': 'application/json',
                Origin: BASE
            },
            body: JSON.stringify({
                state: {
                    lesson: 'my-morning',
                    round: 1,
                    completedRounds: 0,
                    roundStartedAt: Date.now() - 60000,
                    wordMarks: {},
                    missedLast: [],
                    vowelsOn: true,
                    stepIndex: -1,
                    completedSteps,
                    activityState: {}
                },
                rev: Number.isFinite(Number(priorJson.rev)) ? Number(priorJson.rev) : 0
            })
        });
        if (!masterySeed.ok) {
            throw new Error('mastery seed failed: ' + masterySeed.status + ' ' + await masterySeed.text());
        }

        await session.call('Page.navigate', { url: BASE + '/worksheets/my-morning/index.html' });
        await sleep(2000);
        result.mastery = await evaluate(`(() => new Promise((resolve, reject) => {
            const deadline = Date.now() + 12000;
            const wait = () => {
                const ceremony = document.getElementById('mastery-ceremony');
                const back = document.getElementById('mastery-back-today');
                if (ceremony) {
                    return resolve({
                        present: true,
                        wordsMet: !!(document.getElementById('mastery-words-met')),
                        stepsDone: (document.getElementById('mastery-steps-done') || {}).textContent || '',
                        backHref: back ? back.getAttribute('href') : null
                    });
                }
                if (Date.now() > deadline) {
                    return reject(new Error('mastery ceremony missing: ' + ((document.body && document.body.innerText) || '').slice(0, 240)));
                }
                setTimeout(wait, 150);
            };
            wait();
        }))()`);
        assert.equal(result.mastery.present, true, 'mastery ceremony after full round');
        assert.equal(result.mastery.wordsMet, true, 'mastery words-met present');
        assert.equal(result.mastery.backHref, '/', 'mastery Back to Today');
        assert.equal(String(result.mastery.stepsDone), String(stepCount), 'all steps marked done');

        session.close();
        console.log(JSON.stringify({
            ok: true,
            match: m.rows.length + ' pairs coloured',
            story: s,
            typeIt: 'ok',
            audioPlay: result.audioPlay.plays,
            storyRead: result.storyRead.highlighted,
            mastery: 'ok'
        }, null, 2));
    } finally {
        chrome.kill();
        try { fs.rmSync(userData, { recursive: true, force: true }); } catch (err) { /* ignore */ }
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-learner-support-smoke") || main().catch(err => {
    console.error(err.message || err);
    process.exit(1);
});
