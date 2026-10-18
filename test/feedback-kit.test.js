/**
 * Feedback kit acceptance (INIT-005/SPEC-009, REQ-005, REQ-013, REQ-014; ADR-0010).
 *
 * The kit lives in shared/player/feedback.js (SPEC-009 assumed shared/ui/feedback.js).
 * INIT-007/SPEC-002 tests (test/player-feedback.test.js) cover playback classes and the
 * basic chime defaults; this file pins the SPEC-009 criteria:
 *   ac-1  kit exists; reduced-motion path verified
 *   ac-2  sound off by default; a visible mute control exists; word audio is never ducked by UI sound
 *   ac-3  wrong-answer markup is escaped and includes the correct form
 */
'use strict';

const { describe, it } = require('node:test');
const bundle = require('./helpers/player-bundle');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const read = (...p) => fs.readFileSync(path.join(root, ...p), 'utf8');

function fakeEl(extraClasses) {
    const set = new Set(extraClasses || []);
    return {
        nodeType: 1,
        innerHTML: '',
        classList: {
            _set: set,
            add(...n) { n.forEach((x) => set.add(x)); },
            remove(...n) { n.forEach((x) => set.delete(x)); },
            toggle(n, on) { if (on) set.add(n); else set.delete(n); },
            contains: (n) => set.has(n)
        }
    };
}

function fakeToggle() {
    const attrs = {};
    const el = fakeEl();
    el.attrs = attrs;
    el.setAttribute = (k, v) => { attrs[k] = String(v); };
    el.getAttribute = (k) => (k in attrs ? attrs[k] : null);
    return el;
}

function load(opts) {
    const o = opts || {};
    const storage = Object.create(null);
    const audioContexts = [];
    const oscillators = [];
    const plays = [];
    const docListeners = {};
    const toggles = o.toggles || [];
    const sandbox = {
        console,
        setTimeout: (fn) => fn(),
        localStorage: {
            getItem: (k) => (k in storage ? storage[k] : null),
            setItem: (k, v) => { storage[k] = String(v); }
        },
        Audio: function FakeAudio() {
            this.pause = () => {};
            this.load = () => {};
            this.removeAttribute = () => {};
            this.play = () => { plays.push(this.src); return Promise.resolve(); };
        },
        AudioContext: function FakeCtx() {
            audioContexts.push(this);
            this.state = 'running';
            this.currentTime = 0;
            this.destination = {};
            this.createOscillator = () => {
                const osc = { type: '', frequency: { value: 0 }, connect() {}, start() { oscillators.push(osc); }, stop() {} };
                return osc;
            };
            this.createGain = () => ({
                gain: { setValueAtTime() {}, exponentialRampToValueAtTime() {} },
                connect() {}
            });
        },
        SpeechSynthesisUtterance: function () {},
        speechSynthesis: { getVoices: () => [], cancel() {}, speak() {}, addEventListener() {} },
        CustomEvent: function (type, init) { this.type = type; this.detail = init && init.detail; },
        dispatchEvent: () => true,
        location: { href: 'http://127.0.0.1:8080/worksheets/my-morning/index.html' },
        addEventListener() {},
        document: {
            readyState: 'complete',
            addEventListener(type, fn) { docListeners[type] = fn; },
            querySelectorAll(sel) { return /data-sfx-toggle/.test(sel) ? toggles : []; }
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(read('shared', 'player', 'utils.js'), sandbox);
    let player = '';
    bundle.withBundleLock(() => {
        bundle.ensurePlayerBundleUnlocked();
        player = fs.readFileSync(bundle.outfile, 'utf8');
    });
    vm.runInContext(player, sandbox, { filename: 'player.js' });
    vm.runInContext(read('shared', 'player', 'feedback.js'), sandbox);
    return {
        Feedback: sandbox.HebrewFeedback,
        Speech: sandbox.HebrewSpeech,
        storage, audioContexts, oscillators, plays, docListeners, toggles, sandbox
    };
}

describe('ac-3: wrong-answer markup is escaped and includes the correct form', () => {
    it('shows the correct pointed form with its label, retry control, and no flash-only feedback', () => {
        const { Feedback } = load();
        const box = fakeEl(['hidden']);
        Feedback.showMiss(box, { pointed: 'שָׁלוֹם', highlight: 'שָׁ', retryAction: 'retry' });
        assert.equal(box.classList.contains('hidden'), false);
        assert.ok(box.classList.contains('feedback-miss'));
        assert.match(box.innerHTML, /class="correction-label"[^>]*>[\s\S]*הצורה הנכונה/);
        assert.match(box.innerHTML, /The correct form/);
        assert.match(box.innerHTML, /lang="he" dir="rtl"/);
        assert.ok(box.innerHTML.includes('שָׁלוֹם'.replace('שָׁ', '')), 'rest of the word is present');
        assert.match(box.innerHTML, /<mark class="correction-highlight">שָׁ<\/mark>/);
        assert.match(box.innerHTML, /data-action="retry"/);
    });

    it('falls back to `correct` when `pointed` is missing, so the form is never omitted', () => {
        const { Feedback } = load();
        const box = fakeEl();
        Feedback.showMiss(box, { correct: 'בַּיִת' });
        assert.ok(box.innerHTML.includes('בַּיִת'));
    });

    it('escapes every caller-supplied string (word, highlight, retry action, retry labels)', () => {
        const { Feedback } = load();
        const evil = '<img src=x onerror="alert(1)">';
        const box = fakeEl();
        Feedback.showMiss(box, {
            pointed: evil + 'א',
            highlight: evil,
            retryAction: '"><script>alert(2)</script>',
            retryLabelHe: '<b>נסו</b>',
            retryLabelEn: '<i>Try</i>'
        });
        assert.doesNotMatch(box.innerHTML, /<img/i);
        assert.doesNotMatch(box.innerHTML, /<script/i);
        assert.doesNotMatch(box.innerHTML, /<b>|<i>/);
        assert.doesNotMatch(box.innerHTML, /"><script/);
        assert.match(box.innerHTML, /&lt;img/);
        assert.match(box.innerHTML, /&lt;script&gt;/);

        const ok = fakeEl();
        Feedback.showCorrect(ok, { messageHe: '<u>נכון</u>', messageEn: '<s>Right</s>' });
        assert.doesNotMatch(ok.innerHTML, /<u>|<s>/);
    });

    it('correct-answer feedback has no points, streaks, or badges (ADR-0010)', () => {
        const { Feedback } = load();
        const box = fakeEl();
        Feedback.showCorrect(box);
        assert.doesNotMatch(box.innerHTML, /streak|points|badge|leaderboard|confetti/i);
    });

    it('clear() removes the panel and hides it', () => {
        const { Feedback } = load();
        const box = fakeEl();
        Feedback.showMiss(box, { pointed: 'א' });
        Feedback.clear(box);
        assert.equal(box.innerHTML, '');
        assert.ok(box.classList.contains('hidden'));
        assert.equal(box.classList.contains('feedback-panel'), false);
    });
});

describe('ac-2: sound is off by default; word audio is never ducked by UI sound', () => {
    it('default state is off and nothing sounds', () => {
        const { Feedback, audioContexts, oscillators } = load();
        assert.equal(Feedback.isSfxEnabled(), false);
        Feedback.showCorrect(fakeEl());
        Feedback.showMiss(fakeEl(), { pointed: 'א' });
        assert.equal(Feedback.chime('tap'), false);
        assert.equal(audioContexts.length, 0, 'no AudioContext is even created while sound is off');
        assert.equal(oscillators.length, 0);
    });

    it('showCorrect and showMiss are silent by default and chime only after the learner opts in', () => {
        const { Feedback, oscillators } = load();
        Feedback.setSfxEnabled(true);
        Feedback.showCorrect(fakeEl());
        Feedback.showMiss(fakeEl(), { pointed: 'א' });
        assert.equal(oscillators.length, 2);
    });

    it('UI sound never touches word audio: playing a clip is unaffected by chimes, in either mode', async () => {
        for (const sfx of [false, true]) {
            const { Feedback, Speech, plays } = load();
            Feedback.setSfxEnabled(sfx);
            Speech.setClips([{ text: 'קָם', url: '/worksheets/_audio/words/a.mp3' }]);
            const btn = fakeEl();
            Speech.playClip('קָם', '/worksheets/_audio/words/a.mp3', btn);
            await Promise.resolve();
            assert.equal(Speech.playback.status, 'playing', 'sfx=' + sfx);
            Feedback.showCorrect(fakeEl());
            Feedback.chime('tap');
            assert.equal(Speech.playback.status, 'playing', 'chime must not stop or replace the word');
            assert.equal(plays.length, 1, 'chimes use Web Audio, never the word <audio> element');
        }
    });

    it('RSCH-008 P2: the miss tone is neutral, not a low 220 Hz penalty tone', () => {
        const { Feedback, oscillators } = load();
        Feedback.setSfxEnabled(true);
        Feedback.chime('miss');
        Feedback.chime('correct');
        Feedback.chime('tap');
        assert.equal(oscillators.length, 3);
        const [miss, correct, tap] = oscillators.map((o) => o.frequency.value);
        assert.ok(miss >= 400, 'miss tone must not be a low-valence tone, got ' + miss + ' Hz');
        assert.ok(miss <= correct, 'miss tone must not sound brighter than the correct chime');
        assert.ok(miss <= tap + 100 && correct > 0, 'tones stay in one calm range');
        assert.notEqual(miss, 220);
    });

    it('when sound is on, chimes are suppressed (not ducked) while a word is loading or playing', async () => {
        const { Feedback, Speech, oscillators } = load();
        Feedback.setSfxEnabled(true);
        Speech.setClips([{ text: 'קָם', url: '/worksheets/_audio/words/a.mp3' }]);
        Speech.playClip('קָם', '/worksheets/_audio/words/a.mp3', fakeEl());
        assert.equal(Feedback.speechActive, true);
        assert.equal(Feedback.chime('correct'), false);
        assert.equal(oscillators.length, 0);
        Speech.audioEl.onended();
        assert.equal(Feedback.speechActive, false);
        assert.equal(Feedback.chime('correct'), true);
    });

    it('the choice persists across pages through one storage key, written as 0/1', () => {
        const a = load();
        a.Feedback.setSfxEnabled(true);
        assert.equal(a.storage['hebrew-sfx-enabled'], '1');
        a.Feedback.toggleSfx();
        assert.equal(a.storage['hebrew-sfx-enabled'], '0');
        assert.equal(a.Feedback.isSfxEnabled(), false);
    });

    it('a broken or missing localStorage means off, never an error', () => {
        const { Feedback, sandbox } = load();
        sandbox.localStorage = { getItem() { throw new Error('denied'); }, setItem() { throw new Error('denied'); } };
        assert.equal(Feedback.isSfxEnabled(), false);
        assert.doesNotThrow(() => Feedback.setSfxEnabled(true));
        sandbox.localStorage = undefined;
        assert.equal(Feedback.isSfxEnabled(), false);
    });
});

describe('ac-2: the visible mute control', () => {
    it('paintToggles reflects the stored state in aria-pressed and .is-on', () => {
        const t = fakeToggle();
        const { Feedback } = load({ toggles: [t] });
        Feedback.paintToggles();
        assert.equal(t.attrs['aria-pressed'], 'false', 'off by default');
        assert.equal(t.classList.contains('is-on'), false);
        Feedback.setSfxEnabled(true);
        assert.equal(t.attrs['aria-pressed'], 'true');
        assert.equal(t.classList.contains('is-on'), true);
    });

    it('a click on [data-sfx-toggle] flips the setting through the document listener', () => {
        const t = fakeToggle();
        const { Feedback, docListeners } = load({ toggles: [t] });
        assert.equal(typeof docListeners.click, 'function', 'one delegated listener is bound');
        let prevented = false;
        docListeners.click({
            target: { closest: (sel) => (/data-sfx-toggle/.test(sel) ? t : null) },
            preventDefault() { prevented = true; }
        });
        assert.equal(prevented, true);
        assert.equal(Feedback.isSfxEnabled(), true);
        assert.equal(t.attrs['aria-pressed'], 'true');
        docListeners.click({
            target: { closest: (sel) => (/data-sfx-toggle/.test(sel) ? t : null) },
            preventDefault() {}
        });
        assert.equal(Feedback.isSfxEnabled(), false);
        assert.equal(t.attrs['aria-pressed'], 'false');
    });

    it('a click anywhere else does nothing to the setting', () => {
        const { Feedback, docListeners } = load();
        docListeners.click({ target: { closest: () => null }, preventDefault() { throw new Error('no'); } });
        assert.equal(Feedback.isSfxEnabled(), false);
    });

    it('the lesson player header renders the control: button, aria-pressed, Hebrew + English, in the header', () => {
        const core = read('shared', 'player', 'core.js');
        const head = core.slice(core.indexOf('<header class="player-top">'), core.indexOf('</header>', core.indexOf('<header class="player-top">')));
        const btn = head.match(/<button[^>]*data-sfx-toggle[^>]*>[\s\S]*?<\/button>/);
        assert.ok(btn, 'a [data-sfx-toggle] button is in .player-top');
        assert.match(btn[0], /type="button"/);
        assert.match(btn[0], /aria-pressed="false"/, 'starts off');
        assert.match(btn[0], /U\.bi\('צלילים', 'Sounds'\)/, 'Hebrew label with English gloss');
        assert.match(btn[0], /title="[^"]*[\u0590-\u05FF][^"]*\(Sound effects\)"/, 'Hebrew title with gloss');
        assert.ok(head.indexOf('data-sfx-toggle') > head.indexOf('player-top-end'), 'sits with the other header controls');
        // The control is painted from stored state right after the markup is built.
        assert.match(core, /HebrewFeedback\.paintToggles\(app\)/);
    });

    it('the player loads the kit before core.js so the control works', () => {
        const worksheet = read('shared', 'worksheet.js');
        assert.match(worksheet, /player\/feedback\.js[\s\S]*player\/core\.js/);
    });

    it('CSS: 44 px target, visible state beyond colour, label gloss hidden only on narrow screens', () => {
        const css = read('shared', 'worksheet.css');
        const block = css.slice(css.indexOf('/* === INIT-005/SPEC-009 route transition + sound toggle === */'));
        const rule = block.match(/\.player-top \.btn-sfx\s*\{([^}]*)\}/);
        assert.ok(rule, '.btn-sfx rule');
        assert.match(rule[1], /min-height:\s*44px/);
        assert.match(rule[1], /min-width:\s*44px/);
        assert.match(block, /\.btn-sfx\[aria-pressed="true"\]::after\s*\{[^}]*content:/);
        assert.match(block, /\.btn-sfx\.is-on\s*\{/);
        assert.match(block, /@media \(max-width: 599px\)\s*\{\s*\.player-top \.btn-sfx \.en/);
    });
});

describe('ac-1: reduced-motion path for the kit', () => {
    const css = read('shared', 'worksheet.css');

    it('the kit surfaces and the route transition are all switched off under prefers-reduced-motion', () => {
        const reduceBlocks = [];
        const re = /@media \(prefers-reduced-motion: reduce\)\s*\{/g;
        let m;
        while ((m = re.exec(css))) {
            let depth = 0;
            let end = m.index + m[0].length - 1;
            for (let i = end; i < css.length; i += 1) {
                if (css[i] === '{') depth += 1;
                if (css[i] === '}') { depth -= 1; if (depth === 0) { end = i; break; } }
            }
            reduceBlocks.push(css.slice(m.index, end + 1));
        }
        const joined = reduceBlocks.join('\n');
        for (const sel of ['.feedback-panel', '.correction', '.icon-btn.is-playing', '[data-speak].is-playing', '#app-main.route-enter']) {
            assert.ok(joined.includes(sel), sel + ' is covered by a reduced-motion block');
        }
        assert.match(joined, /--motion-fast:\s*0s none/);
        assert.match(joined, /animation:\s*none/);
        assert.match(joined, /transform:\s*none/);
    });

    it('nothing in the kit loops forever outside a reduced-motion override', () => {
        const kitCss = css.slice(css.indexOf('/* === INIT-005/SPEC-009 route transition + sound toggle === */'));
        const outside = kitCss.replace(/@media \(prefers-reduced-motion: reduce\)\s*\{[\s\S]*?\}\s*\}/g, '');
        assert.doesNotMatch(outside, /infinite/);
    });

    it('the feedback module itself starts no timers or animations (all motion is CSS)', () => {
        const code = read('shared', 'player', 'feedback.js');
        assert.doesNotMatch(code, /setInterval|requestAnimationFrame|\.animate\(/);
    });

    it('the kit sits at its real path, and the spec path is recorded as a deviation', () => {
        assert.ok(fs.existsSync(path.join(root, 'shared', 'player', 'feedback.js')));
        assert.equal(fs.existsSync(path.join(root, 'shared', 'ui', 'feedback.js')), false);
    });
});
