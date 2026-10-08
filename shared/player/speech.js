(function (root) {
    const U = root.HebrewUtils;

    const speech = {
        hebrewVoice: null,
        story: { run: 0, status: 'idle', index: 0, utterance: null, clips: [] },
        clipsExact: Object.create(null),
        clipsStripped: Object.create(null),
        audioEl: null
    };

    function normalizeKey(text) {
        return String(text || '').replace(/\s+/g, ' ').trim();
    }

    function stripKey(text) {
        if (U && typeof U.stripVowels === 'function') return normalizeKey(U.stripVowels(text));
        return normalizeKey(String(text || '').replace(/[\u0591-\u05C7]/g, ''));
    }

    function resolveAudioUrl(audioPath) {
        if (!audioPath) return '';
        try {
            const url = new URL(audioPath, root.location ? root.location.href : 'http://localhost/');
            url.pathname = url.pathname
                .split('/')
                .map(part => {
                    if (!part) return part;
                    try { return encodeURIComponent(decodeURIComponent(part)); }
                    catch (err) { return encodeURIComponent(part); }
                })
                .join('/');
            return url.href;
        } catch (err) {
            return audioPath;
        }
    }

    function sharedAudio() {
        if (!speech.audioEl && typeof root.Audio === 'function') {
            speech.audioEl = new root.Audio();
            speech.audioEl.preload = 'auto';
        }
        return speech.audioEl;
    }

    function stopSharedAudio() {
        const audio = speech.audioEl;
        if (!audio) return;
        try {
            audio.onended = null;
            audio.onerror = null;
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
        } catch (err) { /* ignore */ }
    }

    function pickHebrewVoice() {
        if (!('speechSynthesis' in root)) return null;
        const voices = root.speechSynthesis.getVoices() || [];
        const prefer = voices.find(v => /carmit|yael|female/i.test(v.name) && /he/i.test(v.lang))
            || voices.find(v => /he/i.test(v.lang));
        speech.hebrewVoice = prefer || null;
        return speech.hebrewVoice;
    }

    function buildUtterance(text) {
        const utterance = new root.SpeechSynthesisUtterance(text);
        utterance.lang = 'he-IL';
        utterance.rate = 0.85;
        const voice = speech.hebrewVoice || pickHebrewVoice();
        if (voice) utterance.voice = voice;
        return utterance;
    }

    function whenVoicesReady(start) {
        if (!('speechSynthesis' in root)) return;
        const voices = root.speechSynthesis.getVoices();
        if (voices && voices.length) {
            pickHebrewVoice();
            start();
            return;
        }
        let done = false;
        const finish = () => {
            if (done) return;
            done = true;
            pickHebrewVoice();
            start();
        };
        root.speechSynthesis.addEventListener('voiceschanged', finish, { once: true });
        root.setTimeout(finish, 500);
    }

    function speakTts(text) {
        if (!('speechSynthesis' in root) || !text) return;
        whenVoicesReady(() => {
            const utterance = buildUtterance(text);
            root.speechSynthesis.cancel();
            root.setTimeout(() => root.speechSynthesis.speak(utterance), 60);
        });
    }

    speech.resolveAudioUrl = resolveAudioUrl;

    speech.setClips = function (entries) {
        speech.clipsExact = Object.create(null);
        speech.clipsStripped = Object.create(null);
        const strippedCounts = Object.create(null);
        const strippedFirst = Object.create(null);
        (entries || []).forEach(entry => {
            if (!entry || !entry.text || !entry.url) return;
            const exact = normalizeKey(entry.text);
            const url = String(entry.url);
            if (!exact) return;
            speech.clipsExact[exact] = url;
            const stripped = stripKey(exact);
            if (!stripped) return;
            strippedCounts[stripped] = (strippedCounts[stripped] || 0) + 1;
            if (!strippedFirst[stripped]) strippedFirst[stripped] = url;
        });
        Object.keys(strippedCounts).forEach(key => {
            if (strippedCounts[key] === 1) speech.clipsStripped[key] = strippedFirst[key];
        });
    };

    speech.clipFor = function (text) {
        const exact = normalizeKey(text);
        if (!exact) return '';
        if (speech.clipsExact[exact]) return speech.clipsExact[exact];
        const stripped = stripKey(exact);
        return speech.clipsStripped[stripped] || '';
    };

    speech.play = function (text, url) {
        const clip = url || speech.clipFor(text);
        speech.stopStory();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        if (!clip) {
            speakTts(text);
            return;
        }
        const audio = sharedAudio();
        if (!audio) {
            speakTts(text);
            return;
        }
        const resolved = resolveAudioUrl(clip);
        let settled = false;
        const fallback = () => {
            if (settled) return;
            settled = true;
            speakTts(text);
        };
        audio.onended = null;
        audio.onerror = () => fallback();
        try {
            audio.pause();
        } catch (err) { /* ignore */ }
        audio.src = resolved;
        const started = audio.play();
        if (started && typeof started.then === 'function') {
            started.then(() => { settled = true; }).catch(() => fallback());
        } else {
            settled = true;
        }
    };

    speech.speak = function (text) {
        if (!text) return;
        const clip = speech.clipFor(text);
        if (clip) {
            speech.play(text, clip);
            return;
        }
        speech.stopStory();
        speakTts(text);
    };

    speech.silence = function () {
        speech.stopStory();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
    };

    speech.stopStory = function () {
        speech.story.run += 1;
        speech.story.status = 'idle';
        speech.story.index = 0;
        speech.story.utterance = null;
        speech.story.clips = [];
        stopSharedAudio();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
    };

    speech.pauseStory = function () {
        speech.story.run += 1;
        speech.story.status = 'paused';
        speech.story.utterance = null;
        stopSharedAudio();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
    };

    speech.startStory = function (lines, from, clips) {
        const list = lines || [];
        if (!list.length) return;
        speech.story.run += 1;
        const run = speech.story.run;
        speech.story.status = 'playing';
        speech.story.index = Math.max(0, from || 0);
        speech.story.clips = Array.isArray(clips) ? clips.slice() : [];
        stopSharedAudio();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
        readLine(run, list);
    };

    function readLine(run, list) {
        if (run !== speech.story.run) return;
        if (speech.story.index >= list.length) {
            speech.story.status = 'idle';
            speech.story.index = 0;
            speech.story.utterance = null;
            stopSharedAudio();
            if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
            return;
        }
        const text = list[speech.story.index];
        const clip = speech.story.clips[speech.story.index];
        if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();

        if (clip) {
            const audio = sharedAudio();
            if (!audio) {
                readLineTts(run, list, text);
                return;
            }
            const resolved = resolveAudioUrl(clip);
            audio.onended = () => {
                if (run !== speech.story.run) return;
                speech.story.index += 1;
                if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
                readLine(run, list);
            };
            audio.onerror = () => {
                if (run !== speech.story.run) return;
                readLineTts(run, list, text);
            };
            try { audio.pause(); } catch (err) { /* ignore */ }
            audio.src = resolved;
            const started = audio.play();
            if (started && typeof started.then === 'function') {
                started.catch(() => {
                    if (run !== speech.story.run) return;
                    readLineTts(run, list, text);
                });
            }
            return;
        }
        readLineTts(run, list, text);
    }

    function readLineTts(run, list, text) {
        if (run !== speech.story.run) return;
        if (!('speechSynthesis' in root)) {
            speech.story.index += 1;
            readLine(run, list);
            return;
        }
        whenVoicesReady(() => {
            if (run !== speech.story.run) return;
            const utterance = buildUtterance(text);
            speech.story.utterance = utterance;
            utterance.onend = () => {
                if (run !== speech.story.run) return;
                speech.story.index += 1;
                if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
                readLine(run, list);
            };
            utterance.onerror = () => {
                if (run !== speech.story.run) return;
                speech.stopStory();
            };
            root.speechSynthesis.speak(utterance);
            if (typeof speech.onStoryPaint === 'function') speech.onStoryPaint();
        });
    }

    if (typeof root.addEventListener === 'function') {
        root.addEventListener('pagehide', () => {
            stopSharedAudio();
            if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        });
    }

    root.HebrewSpeech = speech;
})(typeof window !== 'undefined' ? window : globalThis);
