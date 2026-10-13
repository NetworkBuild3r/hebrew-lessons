/**
 * INIT-007/SPEC-002 — feedback kit, playback classes, SFX defaults.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadModules() {
    const plays = [];
    const storage = Object.create(null);
    const button = {
        nodeType: 1,
        classList: {
            _set: new Set(),
            add(...names) { names.forEach(n => this._set.add(n)); },
            remove(...names) { names.forEach(n => this._set.delete(n)); },
            toggle(name, on) {
                if (on) this._set.add(name);
                else this._set.delete(name);
            },
            contains(name) { return this._set.has(name); }
        },
        dataset: { speak: 'קָם', audio: '../_audio/words/a.mp3' }
    };

    const sandbox = {
        console,
        setTimeout: fn => fn(),
        localStorage: {
            getItem: key => (Object.prototype.hasOwnProperty.call(storage, key) ? storage[key] : null),
            setItem: (key, val) => { storage[key] = String(val); },
            removeItem: key => { delete storage[key]; }
        },
        Audio: function FakeAudio() {
            this.preload = '';
            this.src = '';
            this.onended = null;
            this.onerror = null;
            this.pause = () => {};
            this.load = () => {};
            this.removeAttribute = () => {};
            this.play = () => {
                plays.push(this.src);
                return Promise.resolve();
            };
        },
        AudioContext: function FakeCtx() {
            this.state = 'running';
            this.currentTime = 0;
            this.createOscillator = () => ({
                type: 'sine',
                frequency: { value: 0 },
                connect() {},
                start() {},
                stop() {}
            });
            this.createGain = () => ({
                gain: {
                    setValueAtTime() {},
                    exponentialRampToValueAtTime() {}
                },
                connect() {}
            });
            this.destination = {};
            this.resume = () => Promise.resolve();
        },
        SpeechSynthesisUtterance: function (text) {
            this.text = text;
            this.lang = '';
            this.rate = 1;
            this.voice = null;
            this.onend = null;
            this.onerror = null;
        },
        speechSynthesis: {
            getVoices: () => [{ name: 'Carmit', lang: 'he-IL' }],
            cancel: () => {},
            speak: () => {},
            addEventListener: () => {}
        },
        CustomEvent: function (type, init) {
            this.type = type;
            this.detail = init && init.detail;
        },
        dispatchEvent: () => true,
        location: { href: 'http://127.0.0.1:8080/worksheets/my-morning/index.html' },
        addEventListener: () => {},
        document: {
            readyState: 'complete',
            addEventListener: () => {},
            querySelectorAll: () => []
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'speech.js'), 'utf8'), sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'feedback.js'), 'utf8'), sandbox);
    return {
        Speech: sandbox.HebrewSpeech,
        Feedback: sandbox.HebrewFeedback,
        plays,
        button,
        storage
    };
}

describe('INIT-007/SPEC-002 feedback kit and audio states', () => {
    it('ac-1: playClip adds is-playing and clears on ended/error', async () => {
        const { Speech, button } = loadModules();
        Speech.setClips([{ text: 'קָם', url: '../_audio/words/a.mp3' }]);
        Speech.playClip('קָם', '../_audio/words/a.mp3', button);
        await Promise.resolve();
        assert.equal(button.classList.contains('is-playing'), true);
        assert.equal(Speech.playback.status, 'playing');

        Speech.audioEl.onended();
        assert.equal(button.classList.contains('is-playing'), false);
        assert.equal(Speech.playback.status, 'idle');

        Speech.playClip('קָם', '../_audio/words/a.mp3', button);
        await Promise.resolve();
        assert.equal(button.classList.contains('is-playing'), true);
        Speech.audioEl.onerror();
        assert.equal(button.classList.contains('is-playing'), false);
    });

    it('ac-2: showMiss renders .correction with pointed Hebrew and data-action retry', () => {
        const { Feedback } = loadModules();
        const container = {
            classList: {
                _set: new Set(['hidden']),
                add(name) { this._set.add(name); },
                remove(name) { this._set.delete(name); }
            },
            innerHTML: ''
        };
        Feedback.showMiss(container, {
            pointed: 'קָם',
            highlight: 'קָ',
            retryAction: 'retry'
        });
        assert.equal(container.classList._set.has('hidden'), false);
        assert.match(container.innerHTML, /class="correction"/);
        assert.match(container.innerHTML, /קָם|קָ/);
        assert.match(container.innerHTML, /data-action="retry"/);
        assert.match(container.innerHTML, /correction-highlight/);
    });

    it('ac-4: SFX off by default and suppressed while speech is active', () => {
        const { Feedback } = loadModules();
        assert.equal(Feedback.isSfxEnabled(), false);
        assert.equal(Feedback.chime('correct'), false);

        Feedback.setSfxEnabled(true);
        assert.equal(Feedback.isSfxEnabled(), true);
        Feedback.setSpeechActive(true);
        assert.equal(Feedback.chime('correct'), false);

        Feedback.setSpeechActive(false);
        assert.equal(Feedback.chime('correct'), true);
    });

    it('showCorrect paints emerald affirmation without gamification copy', () => {
        const { Feedback } = loadModules();
        const container = {
            classList: {
                _set: new Set(['hidden']),
                add(name) { this._set.add(name); },
                remove(name) { this._set.delete(name); }
            },
            innerHTML: ''
        };
        Feedback.showCorrect(container);
        assert.match(container.innerHTML, /feedback-mark/);
        assert.match(container.innerHTML, /Correct|נכון/);
        assert.doesNotMatch(container.innerHTML, /streak|points|badge|leaderboard/i);
    });

    it('worksheet.css includes tactile scale, audio pulse, correction, reduced-motion zeros', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        assert.match(css, /INIT-007\/SPEC-002/);
        assert.match(css, /\.btn:active[\s\S]*scale\(0\.98\)/);
        assert.match(css, /\.choice-card:active[\s\S]*scale\(0\.98\)/);
        assert.match(css, /\.word-match-card:active[\s\S]*scale\(0\.98\)/);
        assert.match(css, /\.bank-word:active[\s\S]*scale\(0\.98\)/);
        assert.match(css, /\.icon-btn\.is-playing/);
        assert.match(css, /\.correction\s*\{/);
        assert.match(css, /@media \(prefers-reduced-motion: reduce\)[\s\S]*--motion-fast:\s*0s none/);
        assert.match(css, /prefers-reduced-motion: reduce\)[\s\S]*animation:\s*none/);
        assert.match(css, /prefers-reduced-motion: reduce\)[\s\S]*transform:\s*none/);
    });

    it('player load path includes feedback.js after speech.js', () => {
        const worksheet = fs.readFileSync(path.join(root, 'shared', 'worksheet.js'), 'utf8');
        const routes = fs.readFileSync(path.join(root, 'shared', 'app-routes.js'), 'utf8');
        assert.match(worksheet, /player\/speech\.js[\s\S]*player\/feedback\.js[\s\S]*player\/core\.js/);
        assert.match(routes, /\/shared\/player\/speech\.js[\s\S]*\/shared\/player\/feedback\.js/);
    });
});
