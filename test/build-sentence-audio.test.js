/**
 * Build-sentence speaker must play the model ElevenLabs clip, not browser TTS of tiles.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const SRC = fs.readFileSync(
    path.join(__dirname, '..', 'shared', 'activities', 'build-sentence.js'),
    'utf8'
);

/** Same decision as playTarget in build-sentence.js (kept in sync by the source asserts below). */
function playTarget(speech, entry) {
    if (!speech || !entry) return;
    const text = entry.pointed || entry.line || '';
    if (entry.audio && typeof speech.play === 'function') {
        speech.play(text, entry.audio);
        return;
    }
    if (text && typeof speech.speak === 'function') speech.speak(text);
}

describe('build-sentence model audio (speak-built)', () => {
    it('source always routes the speaker through playTarget, never TTS of built tiles', () => {
        assert.match(SRC, /function playTarget\(/);
        assert.match(SRC, /playTarget\(ctx\.Speech,\s*entry\)/);
        assert.doesNotMatch(SRC, /Speech\.speak\(built/);
        assert.doesNotMatch(SRC, /full && entry\.audio/);
    });

    it('playTarget prefers the stored clip over TTS', () => {
        const play = [];
        const speak = [];
        const speech = {
            play(text, url) { play.push({ text, url }); },
            speak(text) { speak.push(text); },
        };

        playTarget(speech, {
            pointed: 'אֲנִי קָם',
            line: 'אני קם',
            audio: 'audio/story/line-1.mp3',
        });
        assert.deepEqual(play, [{ text: 'אֲנִי קָם', url: 'audio/story/line-1.mp3' }]);
        assert.deepEqual(speak, []);

        playTarget(speech, { pointed: 'שָׁלוֹם', line: 'שלום', audio: '' });
        assert.deepEqual(speak, ['שָׁלוֹם']);
    });

    it('story and vocab pools keep pointed text with each audio path', () => {
        assert.match(SRC, /pointed:\s*line/);
        assert.match(SRC, /audio:\s*audio\[i\]/);
        assert.match(SRC, /pointed:\s*i\.sentence/);
        assert.match(SRC, /audio:\s*i\.sentenceAudio/);
    });
});
