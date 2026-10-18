const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function classList(names) {
    const set = new Set(names || []);
    return {
        contains: (name) => set.has(name),
        add: (name) => set.add(name),
        remove: (name) => set.delete(name),
        toggle: (name, on) => {
            const next = on === undefined ? !set.has(name) : !!on;
            if (next) set.add(name);
            else set.delete(name);
            return next;
        }
    };
}

function loadSpeech(opts) {
    const options = opts || {};
    const plays = [];
    const tts = [];
    const listeners = [];
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
        location: { href: 'http://127.0.0.1:8080/worksheets/my-morning/index.html' },
        addEventListener: () => {}
    };
    if (options.document) sandbox.document = options.document(listeners);
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    if (options.utils !== false) {
        vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    }
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'speech.js'), 'utf8'), sandbox);
    return { Speech: sandbox.HebrewSpeech, plays, tts, listeners, sandbox };
}

function guideDocument(guidesPage, bodyReady) {
    return function (listeners) {
        const body = bodyReady === false ? null : { classList: classList(guidesPage ? ['guides-page'] : []) };
        const doc = {
            body,
            addEventListener(type, fn) { listeners.push({ type, fn }); }
        };
        doc.setBody = function (next) { doc.body = next; };
        return doc;
    };
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
        assert.equal(tts.length, 0);

        Speech.speak('מילה בלי קליפ');
        assert.equal(tts.length, 1);
    });

    it('maqaf and hyphen share a stripped clip key, with or without utils', () => {
        const maqaf = 'עַל\u05BEכֵּן';
        const hyphen = 'על-כן';
        const url = '../_audio/words/al-ken.mp3';
        const withUtils = loadSpeech();
        withUtils.Speech.setClips([{ text: maqaf, url }]);
        assert.equal(withUtils.Speech.clipFor(hyphen), url);
        assert.equal(withUtils.Speech.clipFor(maqaf), url);
        const reverse = loadSpeech();
        reverse.Speech.setClips([{ text: hyphen, url }]);
        assert.equal(reverse.Speech.clipFor(maqaf), url);
        assert.equal(reverse.Speech.clipFor(hyphen), url);

        const bare = loadSpeech({ utils: false });
        bare.Speech.setClips([{ text: maqaf, url }]);
        assert.equal(bare.Speech.clipFor(hyphen), url);
        assert.equal(bare.Speech.clipFor(maqaf), url);
        const bareReverse = loadSpeech({ utils: false });
        bareReverse.Speech.setClips([{ text: hyphen, url }]);
        assert.equal(bareReverse.Speech.clipFor(maqaf), url);
    });

    it('plays a guides-page audio button once, and ignores other pages', () => {
        const lesson = loadSpeech({ document: guideDocument(false) });
        assert.equal(lesson.listeners.filter((entry) => entry.type === 'click').length, 0);

        const guides = loadSpeech({ document: guideDocument(true) });
        const clicks = guides.listeners.filter((entry) => entry.type === 'click');
        assert.equal(clicks.length, 1);

        const button = {
            classList: classList(['audio-btn']),
            getAttribute(name) {
                if (name === 'data-speak') return 'עַל־כֵּן';
                if (name === 'data-audio') return '/shared/audio/al-ken.mp3';
                return null;
            },
            closest() { return this; }
        };
        let prevented = 0;
        clicks[0].fn({ target: button, preventDefault() { prevented += 1; } });
        clicks[0].fn({ target: button, preventDefault() { prevented += 1; } });
        assert.equal(prevented, 2);
        assert.equal(guides.plays.length, 2);
        assert.ok(guides.plays[0].endsWith('/shared/audio/al-ken.mp3'));

        const quiet = { closest() { return null; } };
        const before = guides.plays.length;
        clicks[0].fn({ target: quiet, preventDefault() { prevented += 1; } });
        assert.equal(guides.plays.length, before);
        assert.equal(prevented, 2);
    });

    it('binds the guides audio listener only once', () => {
        const guides = loadSpeech({
            document: guideDocument(true, false)
        });
        const ready = guides.listeners.filter((entry) => entry.type === 'DOMContentLoaded');
        assert.equal(ready.length, 1);
        guides.sandbox.document.setBody({ classList: classList(['guides-page']) });
        ready[0].fn();
        ready[0].fn();
        assert.equal(guides.listeners.filter((entry) => entry.type === 'click').length, 1);
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
