const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const bundle = require('./helpers/player-bundle');

function bundleSource() {
    let source = '';
    bundle.withBundleLock(() => {
        bundle.ensurePlayerBundleUnlocked();
        source = fs.readFileSync(bundle.outfile, 'utf8');
    });
    return source;
}

function loadSpeech() {
    const plays = [];
    const tts = [];
    const sandbox = {
        console,
        window: {},
        globalThis: {},
        setTimeout: fn => fn(),
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
            speak: utterance => { tts.push(utterance.text); },
            addEventListener: () => {}
        },
        URL,
        location: { href: 'http://127.0.0.1:8080/worksheets/my-morning/index.html' },
        addEventListener: () => {}
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    vm.runInContext(bundleSource(), sandbox, { filename: 'player.js' });
    return { Speech: sandbox.HebrewSpeech, plays, tts };
}

describe('HebrewSpeech clip registry', () => {
    it('plays clip first and falls back to TTS only when missing', () => {
        const { Speech, plays, tts } = loadSpeech();
        Speech.setClips([
            { text: 'קָם', url: '../_audio/words/a.mp3' },
            { text: 'שׁוֹתֶה', url: '../_audio/words/b.mp3' },
            { text: 'שׁוֹתָה', url: '../_audio/words/c.mp3' }
        ]);
        assert.equal(Speech.clipFor('קָם'), '../_audio/words/a.mp3');
        // Stripped collision: neither שותה form gets a stripped key
        assert.equal(Speech.clipFor('שותה'), '');
        assert.ok(Speech.clipFor('שׁוֹתֶה'));

        Speech.speak('קָם');
        assert.equal(plays.length, 1);
        assert.equal(plays[0], 'http://127.0.0.1:8080/worksheets/_audio/words/a.mp3');
        assert.equal(tts.length, 0);

        Speech.speak('מילה בלי קליפ');
        assert.equal(tts.length, 1);
    });

    it('startStory advances on clip onended', () => {
        const { Speech, plays } = loadSpeech();
        const lines = ['שורה א', 'שורה ב'];
        const clips = ['audio/story/line-1.mp3', 'audio/story/line-2.mp3'];
        let paints = 0;
        Speech.onStoryPaint = () => { paints += 1; };
        Speech.startStory(lines, 0, clips);
        assert.equal(Speech.story.status, 'playing');
        assert.equal(plays.length, 1);
        assert.equal(plays[0], 'http://127.0.0.1:8080/worksheets/my-morning/audio/story/line-1.mp3');
        // Simulate end of first clip
        const audio = Speech.audioEl;
        assert.ok(audio && typeof audio.onended === 'function');
        audio.onended();
        assert.equal(Speech.story.index, 1);
        assert.equal(plays.length, 2);
        audio.onended();
        assert.equal(Speech.story.status, 'idle');
        assert.ok(paints >= 2);
    });
});
