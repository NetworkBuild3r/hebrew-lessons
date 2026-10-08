(function (root) {
    const U = root.HebrewUtils;
    const Speech = root.HebrewSpeech;
    const PREFS_KEY = 'hebrew-worksheet-prefs';
    const DAY_MS = 24 * 60 * 60 * 1000;

    const Player = {
        registry: {},
        register(def) {
            if (!def || !def.type) throw new Error('Activity needs a type.');
            Player.registry[def.type] = def;
        }
    };

    function pathHas(ws, dotted) {
        const parts = dotted.split('.');
        let cur = ws;
        for (let i = 0; i < parts.length; i += 1) {
            if (!cur || typeof cur !== 'object') return false;
            cur = cur[parts[i]];
        }
        if (Array.isArray(cur)) return cur.length > 0;
        return !!cur;
    }

    function fallbackActivities(ws) {
        const list = [];
        if (pathHas(ws, 'vocab.items')) list.push({ type: 'words' });
        if (pathHas(ws, 'reading.paragraphs')) list.push({ type: 'story' });
        if (pathHas(ws, 'vocab.items')) list.push({ type: 'match' });
        if (pathHas(ws, 'fill.sentences')) list.push({ type: 'fill' });
        if (pathHas(ws, 'grammar.items')) list.push({ type: 'pattern' });
        if (pathHas(ws, 'reading.retell')) list.push({ type: 'retell' });
        if (pathHas(ws, 'askFriend.questions')) list.push({ type: 'ask-friend' });
        return list;
    }

    function resolveSteps(ws) {
        const level = U.levelOf(ws);
        const levelDefaults = (root.HEBREW_LEVEL_DEFAULTS || {})[level] || root.HEBREW_LEVEL_DEFAULTS.intermediate;
        const raw = Array.isArray(ws.activities) && ws.activities.length
            ? ws.activities
            : fallbackActivities(ws);
        const steps = [];
        raw.forEach((entry, index) => {
            const type = typeof entry === 'string' ? entry : entry && entry.type;
            const def = Player.registry[type];
            if (!def) return;
            if (typeof def.available === 'function' && !def.available(ws, entry || {})) return;
            const config = Object.assign({}, typeof def.defaults === 'function' ? def.defaults(level, levelDefaults) : {}, typeof entry === 'object' ? entry : { type });
            config.type = type;
            steps.push({
                index,
                type,
                def,
                config,
                labelHe: def.labelHe || type,
                labelEn: def.labelEn || type,
                icon: def.icon || 'fa-circle',
                tone: def.tone || 'tone-blue'
            });
        });
        return { level, levelDefaults, steps };
    }

    Player.boot = function () {
        const ws = root.WORKSHEET;
        const app = document.getElementById('app');
        if (!app) return;
        if (!ws || !ws.id) {
            app.innerHTML = '<main class="wrap"><div class="card"><h2>Missing worksheet</h2><p>This folder needs a worksheet.js that sets window.WORKSHEET.</p></div></main>';
            return;
        }

        document.documentElement.setAttribute('translate', 'no');
        if (!document.querySelector('meta[name="google"]')) {
            const meta = document.createElement('meta');
            meta.name = 'google';
            meta.content = 'notranslate';
            document.head.appendChild(meta);
        }
        if (navigator.storage && navigator.storage.persist) {
            navigator.storage.persist().catch(() => {});
        }

        const storageKey = 'hebrew-worksheet:' + ws.id;
        const resolved = resolveSteps(ws);
        let round = 1;
        let completedRounds = 0;
        let roundStartedAt = 0;
        let wordMarks = {};
        let missedLast = [];
        let vowelsOn = true;
        let stepIndex = -1; // -1 = path screen
        let completedSteps = {};
        let activityState = {};
        let currentRoot = null;
        const recordings = {};
        let recorderState = null;

        const ctx = {
            ws,
            get round() { return round; },
            get completedRounds() { return completedRounds; },
            get roundStartedAt() { return roundStartedAt; },
            get wordMarks() { return wordMarks; },
            set wordMarks(v) { wordMarks = v; },
            get missedLast() { return missedLast; },
            get vowelsOn() { return vowelsOn; },
            set vowelsOn(v) { vowelsOn = v; },
            get level() { return resolved.level; },
            get levelDefaults() { return resolved.levelDefaults; },
            get steps() { return resolved.steps; },
            get stepIndex() { return stepIndex; },
            get completedSteps() { return completedSteps; },
            get activityState() { return activityState; },
            get recordings() { return recordings; },
            recallMode() { return round > 1; },
            U,
            Speech,
            bi: U.bi,
            esc: U.escapeHtml,
            sid: U.sid,
            save: saveLocalProgress,
            markComplete,
            setActionBar,
            speakButton,
            recorderHtml,
            paintRecorder,
            playModel,
            toggleRecording,
            playRecording,
            silence: () => Speech.silence(),
            requestRender: () => showStep(stepIndex, false)
        };

        loadSharedWordAudio(() => {
            applySharedWordAudio(ws);
            Speech.setClips(buildClipEntries(ws));
            document.title = String(ws.title || 'דף עבודה').replace(/<[^>]+>/g, '');
            app.innerHTML = shellHtml();
            bindShell();
            restoreSaved();
            routeFromHash(false);
            window.addEventListener('hashchange', () => routeFromHash(true));
        });

        function sharedWordUrl(text) {
            const map = root.HEBREW_WORD_AUDIO;
            if (!map || !text) return '';
            const key = String(text).normalize('NFC').replace(/\s+/g, ' ').trim();
            return map[key] || '';
        }

        function applySharedWordAudio(worksheet) {
            (worksheet.vocab?.items || []).forEach(item => {
                if (item && item.word && !item.audio) {
                    const url = sharedWordUrl(item.word);
                    if (url) item.audio = url;
                }
            });
        }

        function loadSharedWordAudio(done) {
            if (root.HEBREW_WORD_AUDIO) {
                done();
                return;
            }
            let finished = false;
            const finish = () => {
                if (finished) return;
                finished = true;
                done();
            };
            const script = document.createElement('script');
            try {
                script.src = new URL('../_audio/words/catalog.js', window.location.href).href;
            } catch (err) {
                script.src = '../_audio/words/catalog.js';
            }
            script.onload = finish;
            script.onerror = finish;
            document.head.appendChild(script);
            root.setTimeout(finish, 1200);
        }

        function buildClipEntries(worksheet) {
            const entries = [];
            const push = (text, url) => {
                if (text && url) entries.push({ text, url });
            };
            (worksheet.vocab?.items || []).forEach(item => {
                push(item.word, item.audio || sharedWordUrl(item.word));
                push(item.sentence, item.sentenceAudio);
            });
            (worksheet.askFriend?.questions || []).forEach(q => {
                push(q.pointed || q.question, q.audio);
            });
            (worksheet.grammar?.items || []).forEach(item => {
                push(item.pointed, item.audio);
            });
            (worksheet.reading?.questions || []).forEach(q => {
                push(q.question, q.audio);
            });
            const storyLines = U.storyPlain(worksheet.reading?.paragraphs || []);
            const storyAudio = worksheet.reading?.audio || [];
            storyLines.forEach((line, i) => push(line, storyAudio[i]));
            return entries;
        }

        function shellHtml() {
            return `
            <div class="player" id="player">
                <header class="player-top">
                    <div class="player-top-inner">
                        <a class="back-link" href="../../index.html">${U.icon('house')} ${U.bi('חזרה', 'Home')}</a>
                        <div class="player-title">
                            <h1>${U.escapeHtml(ws.title || '')}</h1>
                            ${ws.subtitle ? `<p class="subtitle">${U.escapeHtml(ws.subtitle)}</p>` : ''}
                        </div>
                        <button type="button" class="btn btn-again" data-action="again">
                            ${U.icon('rotate-right')}
                            <span>${U.bi('עוד סיבוב', 'Practice again')}</span>
                        </button>
                    </div>
                    <div class="progress-track" aria-hidden="true"><div class="progress-fill" id="progress-fill"></div></div>
                    <p class="round-bar"><strong id="round-count"></strong> <span id="round-note"></span></p>
                    <p class="round-bar round-due" id="round-due"></p>
                </header>
                <main class="player-main" id="player-main"></main>
                <div class="action-bar hidden" id="action-bar">
                    <div class="action-bar-inner" id="action-bar-inner"></div>
                </div>
            </div>`;
        }

        function bindShell() {
            app.addEventListener('click', onClick);
            app.addEventListener('input', onInput);
            app.addEventListener('change', onChange);
        }

        function onClick(event) {
            const speak = event.target.closest('[data-speak]');
            if (speak) {
                playModel(speak);
                return;
            }
            const recToggle = event.target.closest('[data-rec-toggle]');
            if (recToggle) {
                toggleRecording(recToggle.dataset.recToggle);
                return;
            }
            const recPlay = event.target.closest('[data-rec-play]');
            if (recPlay) {
                playRecording(recPlay.dataset.recPlay);
                return;
            }
            const action = event.target.closest('[data-action]');
            if (action) {
                const name = action.dataset.action;
                if (name === 'again') startNextRound();
                if (name === 'go-path') goPath();
                if (name === 'go-step') goStep(Number(action.dataset.step));
                if (name === 'continue') continueStep();
                if (name === 'check' && currentRoot && currentRoot._activity && currentRoot._activity.onCheck) {
                    currentRoot._activity.onCheck(ctx, currentRoot);
                }
                if (currentRoot && currentRoot._activity && currentRoot._activity.onClick) {
                    currentRoot._activity.onClick(ctx, currentRoot, event, action);
                }
                return;
            }
            if (currentRoot && currentRoot._activity && currentRoot._activity.onClick) {
                currentRoot._activity.onClick(ctx, currentRoot, event, null);
            }
        }

        function onInput(event) {
            if (currentRoot && currentRoot._activity && currentRoot._activity.onInput) {
                currentRoot._activity.onInput(ctx, currentRoot, event);
            }
            saveLocalProgress();
        }

        function onChange(event) {
            if (currentRoot && currentRoot._activity && currentRoot._activity.onChange) {
                currentRoot._activity.onChange(ctx, currentRoot, event);
            }
            saveLocalProgress();
        }

        function paintRound() {
            const count = document.getElementById('round-count');
            const note = document.getElementById('round-note');
            const due = document.getElementById('round-due');
            if (count) count.textContent = 'סיבוב ' + round + ' (Round ' + round + ')';
            if (note) {
                note.textContent = round === 1
                    ? 'לומדים. כרבע מהזמן על המילים ועל דפוס אחד, והשאר הסיפור והדיבור.'
                    : 'שולפים מהזיכרון. המשמעויות מוסתרות עד שתבקשו.';
            }
            if (due) {
                if (!roundStartedAt) {
                    due.textContent = '';
                } else {
                    const when = roundStartedAt + DAY_MS;
                    if (Date.now() >= when) {
                        due.innerHTML = U.bi('מוכן לסיבוב נוסף', 'Ready to practice again');
                    } else {
                        const he = new Date(when).toLocaleString('he-IL', { weekday: 'long', hour: '2-digit', minute: '2-digit' });
                        const en = new Date(when).toLocaleString('en-US', { weekday: 'long', hour: 'numeric', minute: '2-digit' });
                        due.innerHTML = U.bi('סיבוב נוסף מ' + he, 'Next round ' + en);
                    }
                }
            }
            const fill = document.getElementById('progress-fill');
            if (fill) {
                const total = Math.max(resolved.steps.length, 1);
                const done = stepIndex < 0 ? Object.keys(completedSteps).length : Math.max(stepIndex, Object.keys(completedSteps).length);
                fill.style.width = Math.min(100, Math.round((done / total) * 100)) + '%';
            }
        }

        function setActionBar(html) {
            const bar = document.getElementById('action-bar');
            const inner = document.getElementById('action-bar-inner');
            if (!bar || !inner) return;
            if (!html) {
                bar.classList.add('hidden');
                inner.innerHTML = '';
                document.getElementById('player')?.classList.remove('has-action');
                return;
            }
            inner.innerHTML = html;
            bar.classList.remove('hidden');
            document.getElementById('player')?.classList.add('has-action');
        }

        function markComplete(index) {
            completedSteps[String(index)] = true;
            saveLocalProgress();
            paintRound();
        }

        function goPath() {
            location.hash = '';
            showPath();
        }

        function goStep(index) {
            if (index < 0 || index >= resolved.steps.length) return;
            location.hash = 'step-' + (index + 1);
        }

        function continueStep() {
            if (stepIndex >= 0) markComplete(stepIndex);
            const next = stepIndex + 1;
            if (next >= resolved.steps.length) goPath();
            else goStep(next);
        }

        function routeFromHash(fromNav) {
            const match = /^#step-(\d+)$/.exec(location.hash || '');
            if (!match) {
                showPath();
                return;
            }
            const index = Number(match[1]) - 1;
            if (index < 0 || index >= resolved.steps.length) {
                showPath();
                return;
            }
            showStep(index, fromNav);
        }

        function showPath() {
            if (currentRoot && currentRoot._activity && currentRoot._activity.unmount) {
                currentRoot._activity.unmount(ctx, currentRoot);
            }
            currentRoot = null;
            stepIndex = -1;
            Speech.silence();
            setActionBar('');
            const main = document.getElementById('player-main');
            const resume = resolved.steps.findIndex((_, i) => !completedSteps[String(i)]);
            main.innerHTML = `
                <section class="path-screen card">
                    <h2>${U.bi('מסלול השיעור', 'Lesson path')}</h2>
                    <p class="fine">${U.bi('רמה: ' + resolved.level, 'Level: ' + resolved.level)}</p>
                    <ol class="path-list">
                        ${resolved.steps.map((step, i) => {
                            const done = !!completedSteps[String(i)];
                            return `<li class="path-item${done ? ' done' : ''}">
                                <button type="button" class="path-btn" data-action="go-step" data-step="${i}">
                                    <span class="path-num">${done ? U.icon('check') : (i + 1)}</span>
                                    <span class="path-label">${U.icon(step.icon, 'tone-icon ' + step.tone)} ${U.bi(step.labelHe, step.labelEn)}</span>
                                </button>
                            </li>`;
                        }).join('')}
                    </ol>
                    <div class="path-actions">
                        <button type="button" class="btn btn-primary" data-action="go-step" data-step="${resume < 0 ? 0 : resume}">
                            ${resume > 0 ? U.bi('המשיכו מהמקום שעצרתם', 'Continue where you left off') : U.bi('התחילו', 'Start')}
                        </button>
                    </div>
                    <p class="save-note">${U.bi('התשובות נשמרות אוטומטית בדפדפן הזה', 'Answers save automatically in this browser')}</p>
                </section>`;
            paintRound();
            saveLocalProgress();
        }

        function showStep(index, fromNav) {
            if (currentRoot && currentRoot._activity && currentRoot._activity.unmount) {
                currentRoot._activity.unmount(ctx, currentRoot);
            }
            Speech.silence();
            stepIndex = index;
            const step = resolved.steps[index];
            const main = document.getElementById('player-main');
            const state = activityState[step.type] || (activityState[step.type] = {});
            const html = step.def.render(ctx, step.config, state);
            main.innerHTML = `
                <section class="step-screen" data-step-type="${U.escapeHtml(step.type)}">
                    <div class="step-head">
                        <button type="button" class="btn btn-ghost btn-sm" data-action="go-path">${U.icon('list')} ${U.bi('מסלול', 'Path')}</button>
                        <h2>${U.icon(step.icon, 'tone-icon ' + step.tone)} ${U.bi(step.labelHe, step.labelEn)}</h2>
                        <span class="pill">${index + 1} / ${resolved.steps.length}</span>
                    </div>
                    <div class="step-body" id="step-body">${html}</div>
                </section>`;
            currentRoot = document.getElementById('step-body');
            currentRoot._activity = step.def;
            currentRoot._config = step.config;
            if (step.def.mount) step.def.mount(ctx, currentRoot, step.config, state);
            if (step.def.actionBar) setActionBar(step.def.actionBar(ctx, step.config, state));
            else setActionBar(`<button type="button" class="btn btn-primary btn-wide" data-action="continue">${U.bi('המשך', 'Continue')}</button>`);
            paintRound();
            saveLocalProgress();
            if (!fromNav) window.scrollTo(0, 0);
        }

        function startNextRound() {
            const early = roundStartedAt && Date.now() < roundStartedAt + DAY_MS;
            const ok = window.confirm(early
                ? 'עברה פחות מיממה מתחילת הסיבוב. פער של יום עוזר לזכור. עוד סיבוב מוחק את התשובות בדף הזה. להמשיך בכל זאת?\n(Less than a day has passed. A day between rounds helps you remember. Practicing again clears your answers on this page. Continue anyway?)'
                : 'עוד סיבוב מוחק את התשובות בדף הזה, כדי לשלוף אותן מהזיכרון. להמשיך?\n(Practicing again clears your answers on this page so you recall them from memory. Continue?)');
            if (!ok) return;
            Speech.silence();
            completedRounds += 1;
            round += 1;
            missedLast = Object.keys(wordMarks).filter(id => wordMarks[id] === 'missed');
            wordMarks = {};
            completedSteps = {};
            activityState = {};
            roundStartedAt = Date.now();
            saveLocalProgress();
            goPath();
        }

        function getProgressStateJSON() {
            if (currentRoot && currentRoot._activity && currentRoot._activity.collect) {
                activityState[currentRoot._activity.type] = Object.assign(
                    activityState[currentRoot._activity.type] || {},
                    currentRoot._activity.collect(ctx, currentRoot) || {}
                );
            }
            return {
                lesson: ws.id,
                round,
                completedRounds,
                roundStartedAt,
                wordMarks,
                missedLast,
                vowelsOn,
                stepIndex,
                completedSteps,
                activityState
            };
        }

        function saveLocalProgress() {
            if (!roundStartedAt) roundStartedAt = Date.now();
            try {
                localStorage.setItem(storageKey, JSON.stringify(getProgressStateJSON()));
            } catch (err) {
                console.error(err);
            }
        }

        function restoreSaved() {
            try {
                const saved = localStorage.getItem(storageKey)
                    || (ws.legacyStorageKey ? localStorage.getItem(ws.legacyStorageKey) : '');
                if (!saved) return;
                const state = JSON.parse(saved);
                if (Number.isInteger(state.round) && state.round > 0) round = state.round;
                if (Number.isInteger(state.completedRounds) && state.completedRounds >= 0) completedRounds = state.completedRounds;
                if (Number.isFinite(state.roundStartedAt) && state.roundStartedAt > 0) roundStartedAt = state.roundStartedAt;
                if (state.wordMarks && typeof state.wordMarks === 'object') wordMarks = Object.assign({}, state.wordMarks);
                if (Array.isArray(state.missedLast)) missedLast = state.missedLast.map(U.sid);
                if (typeof state.vowelsOn === 'boolean') vowelsOn = state.vowelsOn;
                if (state.completedSteps && typeof state.completedSteps === 'object') completedSteps = Object.assign({}, state.completedSteps);
                if (state.activityState && typeof state.activityState === 'object') activityState = Object.assign({}, state.activityState);
                // Migrate legacy tab progress into activityState when missing.
                if (!activityState.match && Array.isArray(state.matchedPairs)) {
                    activityState.match = { matchedPairs: state.matchedPairs };
                }
                if (!activityState.fill && Array.isArray(state.fillAnswers)) {
                    activityState.fill = { answers: state.fillAnswers };
                }
                if (!activityState.pattern && (state.grammarAnswers || state.indirectAnswers)) {
                    activityState.pattern = { answers: state.grammarAnswers || state.indirectAnswers };
                }
                if (!activityState.story && Array.isArray(state.readingAnswers)) {
                    activityState.story = { answers: state.readingAnswers };
                }
                if (!activityState['ask-friend'] && Array.isArray(state.friendAnswers)) {
                    activityState['ask-friend'] = { answers: state.friendAnswers };
                }
                if (!activityState.retell && typeof state.retellAnswer === 'string') {
                    activityState.retell = { answer: state.retellAnswer };
                }
                if (Number.isInteger(state.stepIndex)) stepIndex = state.stepIndex;
            } catch (err) {
                console.error(err);
            }
        }

        function speakButton(text, audio, title) {
            const label = title || 'Listen';
            const clip = audio || Speech.clipFor(text) || '';
            return `<button type="button" class="icon-btn${clip ? ' human' : ''}" data-speak="${U.escapeHtml(text || '')}" data-audio="${U.escapeHtml(clip)}" title="${U.escapeHtml(label)}">${U.icon('volume-high')}</button>`;
        }

        function recorderHtml(key) {
            return `<span class="recorder" data-rec="${U.escapeHtml(key)}">
                <button type="button" class="icon-btn rec-btn" data-rec-toggle="${U.escapeHtml(key)}" title="Record">${U.icon('microphone')}</button>
                <button type="button" class="icon-btn hidden" data-rec-play="${U.escapeHtml(key)}" title="Play recording">${U.icon('play')}</button>
            </span>`;
        }

        function paintRecorder(key) {
            const wrap = document.querySelector('[data-rec="' + key + '"]');
            if (!wrap) return;
            const toggle = wrap.querySelector('[data-rec-toggle]');
            const play = wrap.querySelector('[data-rec-play]');
            const active = recorderState && recorderState.key === key;
            toggle.classList.toggle('recording', !!active);
            if (play) play.classList.toggle('hidden', !recordings[key]);
        }

        function playModel(button) {
            Speech.play(button.dataset.speak, button.dataset.audio || null);
        }

        function toggleRecording(key) {
            Speech.silence();
            if (recorderState && recorderState.key === key) {
                recorderState.recorder.stop();
                return;
            }
            if (recorderState) {
                try { recorderState.recorder.stop(); } catch (err) { /* ignore */ }
            }
            if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                alert('הדפדפן לא תומך בהקלטה. (This browser cannot record audio.)');
                return;
            }
            navigator.mediaDevices.getUserMedia({ audio: true }).then(stream => {
                const chunks = [];
                const recorder = new MediaRecorder(stream);
                recorderState = { key, recorder, stream, started: Date.now() };
                recorder.ondataavailable = event => { if (event.data.size) chunks.push(event.data); };
                recorder.onstop = () => {
                    stream.getTracks().forEach(track => track.stop());
                    if (chunks.length) {
                        recordings[key] = new Blob(chunks, { type: chunks[0].type || 'audio/webm' });
                    }
                    recorderState = null;
                    paintRecorder(key);
                };
                recorder.start();
                paintRecorder(key);
                root.setTimeout(() => {
                    if (recorderState && recorderState.key === key && recorder.state === 'recording') recorder.stop();
                }, 30000);
            }).catch(() => alert('אין גישה למיקרופון. (Microphone access was denied.)'));
        }

        function playRecording(key) {
            Speech.silence();
            const blob = recordings[key];
            if (!blob) return;
            new Audio(URL.createObjectURL(blob)).play();
        }
    };

    root.HebrewPlayer = Player;
})(typeof window !== 'undefined' ? window : globalThis);
