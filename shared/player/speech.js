(function (root) {
    const U = root.HebrewUtils;

    const speech = {
        hebrewVoice: null,
        story: { run: 0, status: 'idle', index: 0, utterance: null, clips: [] },
        clipsExact: Object.create(null),
        clipsStripped: Object.create(null),
        audioEl: null,
        /** @type {{ status: string, el: Element|null }} */
        playback: { status: 'idle', el: null },
        _playingEl: null,
        _lastClip: null
    };

    function resolvePlayEl(elOrOpts) {
        if (!elOrOpts) return null;
        if (elOrOpts.nodeType === 1) return elOrOpts;
        if (elOrOpts.el && elOrOpts.el.nodeType === 1) return elOrOpts.el;
        if (elOrOpts.button && elOrOpts.button.nodeType === 1) return elOrOpts.button;
        return null;
    }

    function notifyFeedback(active) {
        const fb = root.HebrewFeedback;
        if (fb && typeof fb.setSpeechActive === 'function') fb.setSpeechActive(!!active);
    }

    function dispatchPlayback(status, el) {
        speech.playback = { status: status || 'idle', el: el || null };
        notifyFeedback(status === 'loading' || status === 'playing');
        if (typeof root.CustomEvent === 'function' && typeof root.dispatchEvent === 'function') {
            try {
                root.dispatchEvent(new root.CustomEvent('hebrew-speech-playback', {
                    detail: { status: speech.playback.status, el: speech.playback.el }
                }));
            } catch (err) { /* ignore */ }
        }
    }

    function clearPlayingClasses(el) {
        const target = el || speech._playingEl;
        if (target && target.classList) {
            target.classList.remove('is-loading', 'is-playing');
        }
        if (!el || el === speech._playingEl) speech._playingEl = null;
    }

    function setPlaybackUi(status, el) {
        if (speech._playingEl && speech._playingEl !== el) clearPlayingClasses(speech._playingEl);
        if (el && el.classList) {
            speech._playingEl = el;
            el.classList.toggle('is-loading', status === 'loading');
            el.classList.toggle('is-playing', status === 'playing');
        } else if (status === 'idle' || status === 'error') {
            clearPlayingClasses();
        }
        dispatchPlayback(status, el || null);
    }

    function endPlayback(status) {
        const el = speech._playingEl;
        clearPlayingClasses(el);
        dispatchPlayback(status || 'idle', null);
    }

    function normalizeKey(text) {
        return String(text || '').normalize('NFC').replace(/\s+/g, ' ').trim();
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
        endPlayback('idle');
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

    function speakTts(text, el) {
        if (!('speechSynthesis' in root) || !text) {
            endPlayback('error');
            return;
        }
        whenVoicesReady(() => {
            const utterance = buildUtterance(text);
            utterance.onend = () => endPlayback('idle');
            utterance.onerror = () => endPlayback('error');
            setPlaybackUi('playing', el || speech._playingEl);
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

    /**
     * Play a clip (or TTS fallback). Optional third arg is the associated
     * button/container Element, or `{ el }` / `{ button }`, so UI can show
     * loading/playing classes.
     */
    speech.play = function (text, url, elOrOpts) {
        const el = resolvePlayEl(elOrOpts);
        const clip = url || speech.clipFor(text);
        speech.stopStory();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        speech._lastClip = { text: text || '', url: clip || '', el: el || null };
        if (!clip) {
            speakTts(text, el);
            return;
        }
        const audio = sharedAudio();
        if (!audio) {
            speakTts(text, el);
            return;
        }
        const resolved = resolveAudioUrl(clip);
        let settled = false;
        const fallback = () => {
            if (settled) return;
            settled = true;
            speakTts(text, el);
        };
        setPlaybackUi('loading', el);
        audio.onended = () => endPlayback('idle');
        audio.onerror = () => {
            endPlayback('error');
            fallback();
        };
        try {
            audio.pause();
        } catch (err) { /* ignore */ }
        audio.src = resolved;
        const started = audio.play();
        if (started && typeof started.then === 'function') {
            started.then(() => {
                settled = true;
                setPlaybackUi('playing', el);
            }).catch(() => {
                endPlayback('error');
                fallback();
            });
        } else {
            settled = true;
            setPlaybackUi('playing', el);
        }
    };

    /** Alias used by feedback kit / ADR-0010 callers; same as play with an element hook. */
    speech.playClip = function (text, url, el) {
        return speech.play(text, url, el);
    };

    speech.replayLast = function () {
        const last = speech._lastClip;
        if (!last || (!last.text && !last.url)) return false;
        speech.play(last.text, last.url || null, last.el || null);
        return true;
    };

    speech.speak = function (text, elOrOpts) {
        if (!text) return;
        const el = resolvePlayEl(elOrOpts);
        const clip = speech.clipFor(text);
        if (clip) {
            speech.play(text, clip, el);
            return;
        }
        speech.stopStory();
        speech._lastClip = { text: text || '', url: '', el: el || null };
        speakTts(text, el);
    };

    speech.silence = function () {
        speech.stopStory();
        if ('speechSynthesis' in root) root.speechSynthesis.cancel();
        endPlayback('idle');
    };

    /** Stop every sound: story, TTS, and the shared clip player. The router calls this on navigation. */
    speech.stopAll = function () {
        speech.silence();
        stopSharedAudio();
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
        notifyFeedback(true);
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
            notifyFeedback(false);
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
