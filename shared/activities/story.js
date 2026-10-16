/**
 * Story reading: warm cream surface, follow-along, gloss popover.
 * Provenance: INIT-007/SPEC-004 · ADR-0010 (reuse SPEC-002 is-playing / playClip).
 */
(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;
    const Speech = window.HebrewSpeech;
    const PREFS_KEY = 'hebrew-worksheet-prefs';

    /** @type {{root:Element,onKey:Function,onDocClick:Function}|null} */
    let glossSession = null;

    P.register({
        type: 'story',
        labelHe: 'סיפור',
        labelEn: 'Story',
        icon: 'fa-file-lines',
        tone: 'tone-amber',
        available(ws) { return (ws.reading?.paragraphs || []).length > 0; },
        render(ctx, config, state) {
            const paragraphs = ctx.ws.reading.paragraphs;
            const questions = ctx.ws.reading.questions || [];
            const answers = Object.fromEntries((state.answers || []).map(a => [String(a.id), a.val]));
            const follow = followAlongPref();
            const questionsOpen = questionsOpenPref();
            const coverage = estimateCoverage(ctx);
            return `
                <div class="stack">
                    <div class="banner banner-amber">
                        <h3>${ctx.bi('קראו את הטקסט', 'Read the text')}</h3>
                        <p class="fine">${ctx.bi('סגול = דיבור עקיף, אדום = דיבור ישיר', 'Purple = reported speech, red = direct speech')}</p>
                        ${coverageBadgeHtml(ctx, coverage)}
                    </div>
                    <div class="story-tools" role="toolbar" aria-label="${ctx.esc('הקראה')}">
                        <button type="button" class="btn btn-ghost" data-action="toggle-vowels">${ctx.vowelsOn ? ctx.bi('הסתר ניקוד', 'Hide vowels') : ctx.bi('הצג ניקוד', 'Show vowels')}</button>
                        <label class="follow-along"><input type="checkbox" id="follow-along"${follow ? ' checked' : ''}> ${ctx.bi('הדגשת השורה', 'Follow along')}</label>
                        <button type="button" class="btn btn-amber story-ctrl" id="story-play" data-action="story-play">${U.icon('volume-high')} <span class="story-ctrl-label">${ctx.bi('הקראת טקסט', 'Read aloud')}</span></button>
                        <button type="button" class="btn btn-ghost story-ctrl hidden" id="story-pause" data-action="story-pause">${U.icon('pause')} <span class="story-ctrl-label">${ctx.bi('השהיה', 'Pause')}</span></button>
                        <button type="button" class="btn btn-amber story-ctrl hidden" id="story-resume" data-action="story-play">${U.icon('play')} <span class="story-ctrl-label">${ctx.bi('המשך', 'Resume')}</span></button>
                        <button type="button" class="btn btn-stop story-ctrl hidden" id="story-stop" data-action="story-stop">${U.icon('stop')} <span class="story-ctrl-label">${ctx.bi('עצירה', 'Stop')}</span></button>
                    </div>
                    <div class="story-body card" id="story-text">
                        ${paragraphs.map(p => `<p data-vowels-html="1">${U.renderParts(Array.isArray(p) ? p : [p])}</p>`).join('')}
                    </div>
                    ${questions.length ? `
                        <div class="story-questions-toggle">
                            <button type="button" class="btn btn-ghost" data-action="toggle-questions" id="story-questions-btn" aria-expanded="${questionsOpen ? 'true' : 'false'}">${questionsButtonLabel(ctx, questionsOpen)}</button>
                            <p class="fine${questionsOpen ? ' hidden' : ''}" id="story-questions-note">${ctx.bi('אפשר לחזור לשאלות כשתכירו יותר מילים.', 'Come back to these when you know more words.')}</p>
                        </div>
                        <div class="card stack${questionsOpen ? '' : ' hidden'}" id="story-questions-card">
                            <h3>${ctx.bi('שאלות הבנה', 'Comprehension Questions')}</h3>
                            ${questions.map(q => `
                                <div class="q-block">
                                    <p><span class="q-label">${ctx.sid(q.id)}</span> ${ctx.esc(q.question)} ${ctx.speakButton(q.question, q.audio)}</p>
                                    <textarea id="reading-ans-${ctx.sid(q.id)}" rows="2" placeholder="כתבו את התשובה בעברית...">${ctx.esc(answers[ctx.sid(q.id)] || '')}</textarea>
                                </div>
                            `).join('')}
                        </div>
                    ` : ''}
                </div>`;
        },
        mount(ctx, root) {
            applyVowels(ctx, root);
            enhanceStoryTokens(ctx, root);
            Speech.onStoryPaint = () => paintStory(ctx, root);
            paintStory(ctx, root);
            root.querySelector('#follow-along')?.addEventListener('change', () => {
                try {
                    const prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
                    prefs.followAlong = !!root.querySelector('#follow-along')?.checked;
                    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
                } catch (err) { /* ignore */ }
                paintStory(ctx, root);
            });

            const onKey = (event) => {
                if (event.key === 'Escape') dismissGloss(root);
            };
            const onDocClick = (event) => {
                if (!root.contains(event.target)) dismissGloss(root);
            };
            document.addEventListener('keydown', onKey);
            document.addEventListener('click', onDocClick, true);
            glossSession = { root, onKey, onDocClick };
        },
        unmount() {
            Speech.onStoryPaint = null;
            Speech.stopStory();
            if (glossSession) {
                document.removeEventListener('keydown', glossSession.onKey);
                document.removeEventListener('click', glossSession.onDocClick, true);
                glossSession = null;
            }
        },
        onClick(ctx, root, event, action) {
            if (!action) {
                if (event && event.target && event.target.closest && event.target.closest('.story-gloss-popover')) {
                    return;
                }
                dismissGloss(root);
                return;
            }
            if (action.dataset.action === 'toggle-vowels') {
                dismissGloss(root);
                ctx.vowelsOn = !ctx.vowelsOn;
                applyVowels(ctx, root);
                enhanceStoryTokens(ctx, root);
                action.textContent = ctx.vowelsOn ? 'הסתר ניקוד (Hide vowels)' : 'הצג ניקוד (Show vowels)';
                ctx.save();
            }
            if (action.dataset.action === 'toggle-questions') {
                dismissGloss(root);
                const card = root.querySelector('#story-questions-card');
                if (!card) return;
                // Hidden, not removed: typed answers stay in the page and in collect().
                const open = card.classList.contains('hidden');
                card.classList.toggle('hidden', !open);
                action.setAttribute('aria-expanded', open ? 'true' : 'false');
                action.innerHTML = questionsButtonLabel(ctx, open);
                const note = root.querySelector('#story-questions-note');
                if (note) note.classList.toggle('hidden', open);
                setQuestionsOpenPref(open);
                return;
            }
            if (action.dataset.action === 'story-play') {
                dismissGloss(root);
                const lines = storyLines(ctx);
                const from = Speech.story.status === 'paused' ? Speech.story.index : 0;
                Speech.startStory(lines, from, ctx.ws.reading.audio || []);
            }
            if (action.dataset.action === 'story-pause') Speech.pauseStory();
            if (action.dataset.action === 'story-stop') Speech.stopStory();
            if (action.dataset.action === 'story-token') {
                event && event.stopPropagation && event.stopPropagation();
                openGloss(ctx, root, action);
            }
            if (action.dataset.action === 'gloss-audio') {
                event && event.stopPropagation && event.stopPropagation();
                const token = action.dataset.token || '';
                if (!token || !Speech) return;
                if (typeof Speech.playClip === 'function') {
                    Speech.playClip(token, null, action);
                } else if (typeof Speech.speak === 'function') {
                    Speech.speak(token, action);
                }
            }
            if (action.dataset.action === 'gloss-dismiss') {
                dismissGloss(root);
            }
        },
        collect(ctx, root) {
            return {
                answers: (ctx.ws.reading.questions || []).map(q => ({
                    id: q.id,
                    val: root.querySelector('#reading-ans-' + ctx.sid(q.id))?.value || ''
                }))
            };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function followAlongPref() {
        try {
            const prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return prefs.followAlong !== false;
        } catch (err) {
            return true;
        }
    }

    /** Comprehension questions start collapsed. The learner's choice is kept in the prefs key. */
    function questionsOpenPref() {
        try {
            const prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return prefs.questionsOpen === true;
        } catch (err) {
            return false;
        }
    }

    function setQuestionsOpenPref(open) {
        try {
            const prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            prefs.questionsOpen = !!open;
            localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
        } catch (err) { /* ignore */ }
    }

    function questionsButtonLabel(ctx, open) {
        return open
            ? ctx.bi('הסתר שאלות הבנה', 'Hide comprehension questions')
            : ctx.bi('הצג שאלות הבנה', 'Show comprehension questions');
    }

    function storyLines(ctx) {
        return U.storyPlain(ctx.ws.reading.paragraphs).map(line => ctx.vowelsOn ? line : U.stripVowels(line));
    }

    function exactKey(text) {
        return String(text || '').normalize('NFC').replace(/\s+/g, ' ').trim();
    }

    function strippedKey(text) {
        return exactKey(U.stripVowels(text));
    }

    /**
     * Identity resolve against a map of exactKey → value (ADR-0001 §1).
     * @param {string} text
     * @param {Record<string, *>} catalog
     */
    function resolveInCatalog(text, catalog) {
        if (!catalog || typeof catalog !== 'object') return null;
        const e = exactKey(text);
        if (!e) return null;
        if (Object.prototype.hasOwnProperty.call(catalog, e)) return e;
        const s = strippedKey(e);
        if (!s) return null;
        let match = null;
        Object.keys(catalog).forEach(key => {
            if (strippedKey(key) === s) {
                if (match && match !== key) match = '__ambiguous__';
                else if (!match) match = key;
            }
        });
        return match && match !== '__ambiguous__' ? match : null;
    }

    function vocabCatalog(ctx) {
        const map = Object.create(null);
        (ctx.ws.vocab && ctx.ws.vocab.items || []).forEach(item => {
            if (!item || !item.word) return;
            map[exactKey(item.word)] = item;
        });
        return map;
    }

    function glossCatalog(ctx) {
        const glosses = ctx.ws.reading && ctx.ws.reading.glosses;
        if (!glosses || typeof glosses !== 'object' || Array.isArray(glosses)) return Object.create(null);
        const map = Object.create(null);
        Object.keys(glosses).forEach(key => {
            map[exactKey(key)] = String(glosses[key] == null ? '' : glosses[key]);
        });
        return map;
    }

    /**
     * Lookup order: reading.glosses → lesson vocab meaning → null (audio-only).
     * @returns {{token:string, meaning:string|null, audioText:string}}
     */
    function lookupGloss(ctx, rawToken) {
        const token = exactKey(rawToken);
        const glosses = glossCatalog(ctx);
        const glossKey = resolveInCatalog(token, glosses);
        if (glossKey != null && glosses[glossKey]) {
            return { token, meaning: glosses[glossKey], audioText: token };
        }
        const vocab = vocabCatalog(ctx);
        const vocabKey = resolveInCatalog(token, vocab);
        if (vocabKey != null && vocab[vocabKey] && vocab[vocabKey].meaning) {
            return { token: vocabKey, meaning: String(vocab[vocabKey].meaning), audioText: vocabKey };
        }
        return { token, meaning: null, audioText: token };
    }

    function sameLocalDay(a, b) {
        if (a == null || b == null) return false;
        const da = new Date(a);
        const db = new Date(b);
        if (Number.isNaN(da.getTime()) || Number.isNaN(db.getTime())) return false;
        return da.getFullYear() === db.getFullYear()
            && da.getMonth() === db.getMonth()
            && da.getDate() === db.getDate();
    }

    /** Record not_yet at most once per word per local day (ADR-0001 §5). */
    function recordNotYet(ctx, wordKey) {
        const Review = ctx.review;
        if (!Review || typeof Review.record !== 'function') return;
        const key = exactKey(wordKey);
        if (!key) return;
        const existing = typeof Review.get === 'function' ? Review.get(key) : null;
        const he = existing && existing.he_en;
        if (he && he.lastResult === 'not_yet' && sameLocalDay(he.lastAt, Date.now())) {
            return;
        }
        try {
            Review.record(key, 'he_en', 'not_yet', { lesson: ctx.ws && ctx.ws.id });
        } catch (err) { /* fail-soft */ }
    }

    function lessonCompleted(slug) {
        if (!slug) return false;
        try {
            if (typeof localStorage === 'undefined' || !localStorage) return false;
            const raw = localStorage.getItem('hebrew-worksheet:' + slug);
            if (!raw) return false;
            const st = JSON.parse(raw);
            if ((st.completedRounds || 0) > 0) return true;
            return Object.keys(st.completedSteps || {}).length > 0;
        } catch (err) {
            return false;
        }
    }

    function hasPassingRecord(entry) {
        if (!entry) return false;
        const he = entry.he_en || {};
        const en = entry.en_he || {};
        return he.lastResult === 'pass' || en.lastResult === 'pass';
    }

    function resolveStoreKey(token, store) {
        if (!store || !store.words) return exactKey(token);
        const words = store.words;
        const hit = resolveInCatalog(token, words);
        return hit || exactKey(token);
    }

    function knownFromEarlierLesson(tokenKey, ctx) {
        const index = typeof window !== 'undefined' ? window.HEBREW_VOCAB_INDEX : null;
        const list = Array.isArray(index) ? index : [];
        if (!list.length) return false;
        const current = ctx.ws && ctx.ws.id ? String(ctx.ws.id) : '';
        const e = exactKey(tokenKey);
        const s = strippedKey(e);
        let entry = list.find(item => item && exactKey(item.key || item.word) === e);
        if (!entry && s) {
            const matches = list.filter(item => item && strippedKey(item.key || item.word) === s);
            if (matches.length === 1) entry = matches[0];
        }
        if (!entry || !Array.isArray(entry.lessons)) return false;
        return entry.lessons.some(slug => String(slug) !== current && lessonCompleted(slug));
    }

    function isKnownToken(token, ctx, store) {
        const key = resolveStoreKey(token, store);
        const entry = store && store.words ? store.words[key] : null;
        if (hasPassingRecord(entry)) return true;
        if (knownFromEarlierLesson(key, ctx) || knownFromEarlierLesson(token, ctx)) return true;
        return false;
    }

    /**
     * @returns {{kind:'unknown'}|{kind:'estimate',pct:number,known:number,total:number}}
     */
    function estimateCoverage(ctx) {
        const plain = U.storyPlain(ctx.ws.reading && ctx.ws.reading.paragraphs || []).join(' ');
        const tokens = typeof U.tokenizeHebrew === 'function' ? U.tokenizeHebrew(plain) : [];
        if (!tokens.length) return { kind: 'unknown' };

        const Review = ctx.review;
        let store = null;
        try {
            store = Review && typeof Review.all === 'function' ? Review.all() : null;
        } catch (err) {
            store = null;
        }
        const wordCount = store && store.words ? Object.keys(store.words).length : 0;
        if (!wordCount) return { kind: 'unknown' };

        let known = 0;
        tokens.forEach(tok => {
            if (isKnownToken(tok, ctx, store)) known += 1;
        });
        const pct = Math.round((known / tokens.length) * 100);
        return { kind: 'estimate', pct: pct, known: known, total: tokens.length };
    }

    function coverageBadgeHtml(ctx, coverage) {
        if (!coverage || coverage.kind === 'unknown') {
            return `<div class="story-coverage" id="story-coverage" data-coverage="unknown">
                <p class="story-coverage-value">${ctx.bi('כיסוי לא ידוע', 'Coverage unknown')}</p>
                <p class="fine story-coverage-note">${ctx.bi(
                    'אומדן מילים מוכרות לפי מאגר החזרה — עדיין אין נתונים.',
                    'Estimated known words from the review store — no data yet.'
                )}</p>
            </div>`;
        }
        const label = '≈' + coverage.pct + '% מילים מוכרות (known words, estimate)';
        return `<div class="story-coverage" id="story-coverage" data-coverage="${coverage.pct}">
            <p class="story-coverage-value">${U.escapeHtml(label)}</p>
            <p class="fine story-coverage-note">${ctx.bi(
                'אומדן בלבד — לפי מילים שעברו בחזרה או הופיעו בשיעור קודם.',
                'Estimate only — from words passed in review or seen in an earlier lesson.'
            )}</p>
        </div>`;
    }

    function applyVowels(ctx, root) {
        root.querySelectorAll('[data-vowels-html]').forEach((el, i) => {
            const p = ctx.ws.reading.paragraphs[i];
            if (!p) return;
            const html = U.renderParts(Array.isArray(p) ? p : [p]);
            el.innerHTML = ctx.vowelsOn ? html : stripHtmlVowels(html);
        });
    }

    function stripHtmlVowels(html) {
        const tmp = document.createElement('div');
        tmp.innerHTML = html;
        const walk = node => {
            if (node.nodeType === 3) node.textContent = U.stripVowels(node.textContent);
            else node.childNodes.forEach(walk);
        };
        walk(tmp);
        return tmp.innerHTML;
    }

    function enhanceStoryTokens(ctx, root) {
        const body = root.querySelector('#story-text');
        if (!body || typeof U.tokenizeHebrew !== 'function') return;
        const paragraphs = body.querySelectorAll('p[data-vowels-html]');
        paragraphs.forEach(p => wrapTokensInElement(p));
    }

    function wrapTokensInElement(el) {
        const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, null);
        const textNodes = [];
        let node;
        while ((node = walker.nextNode())) {
            if (!node.nodeValue || !/[\u05D0-\u05EA]/.test(node.nodeValue)) continue;
            if (node.parentElement && node.parentElement.closest('.story-token')) continue;
            textNodes.push(node);
        }
        textNodes.forEach(textNode => {
            const text = textNode.nodeValue;
            const tokens = U.tokenizeHebrew(text);
            if (!tokens.length) return;
            const frag = document.createDocumentFragment();
            let remaining = text;
            tokens.forEach(token => {
                const idx = remaining.indexOf(token);
                if (idx < 0) return;
                if (idx > 0) frag.appendChild(document.createTextNode(remaining.slice(0, idx)));
                const btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'story-token';
                btn.dataset.action = 'story-token';
                btn.dataset.token = token;
                btn.setAttribute('aria-label', 'Gloss: ' + token);
                btn.textContent = token;
                frag.appendChild(btn);
                remaining = remaining.slice(idx + token.length);
            });
            if (remaining) frag.appendChild(document.createTextNode(remaining));
            textNode.parentNode.replaceChild(frag, textNode);
        });
    }

    function dismissGloss(root) {
        const pop = root && root.querySelector && root.querySelector('.story-gloss-popover');
        if (pop && typeof pop.remove === 'function') {
            try { pop.remove(); } catch (err) { /* ignore */ }
        } else if (pop && pop.parentNode && typeof pop.parentNode.removeChild === 'function') {
            try { pop.parentNode.removeChild(pop); } catch (err) { /* ignore */ }
        }
        if (root && root._pop) root._pop = null;
        root && root.querySelectorAll && root.querySelectorAll('.story-token[aria-expanded="true"]').forEach(el => {
            el.setAttribute('aria-expanded', 'false');
        });
    }

    function openGloss(ctx, root, tokenBtn) {
        dismissGloss(root);
        const raw = tokenBtn.dataset.token || tokenBtn.textContent || '';
        const found = lookupGloss(ctx, raw);
        recordNotYet(ctx, found.token);

        tokenBtn.setAttribute('aria-expanded', 'true');
        const pop = document.createElement('div');
        pop.className = 'story-gloss-popover';
        pop.setAttribute('role', 'dialog');
        pop.setAttribute('aria-label', ctx.bi('פירוש', 'Gloss'));
        const meaningHtml = found.meaning
            ? `<p class="story-gloss-en" dir="ltr">${U.escapeHtml(found.meaning)}</p>`
            : `<p class="story-gloss-en story-gloss-missing" dir="ltr">${ctx.bi('אין פירוש', 'No gloss')}</p>`;
        const audioText = U.escapeHtml(found.audioText);
        pop.innerHTML = `
            <p class="story-gloss-he" lang="he">${U.escapeHtml(found.token)}</p>
            ${meaningHtml}
            <div class="story-gloss-actions">
                <button type="button" class="icon-btn" data-action="gloss-audio" data-speak="${audioText}" data-token="${audioText}" title="${ctx.esc('השמעה (Play audio)')}" aria-label="${ctx.esc('השמעה')}">${U.icon('volume-high')}</button>
                <button type="button" class="btn btn-ghost btn-sm" data-action="gloss-dismiss">${ctx.bi('סגור', 'Close')}</button>
            </div>
        `;
        const body = root.querySelector('#story-text') || root;
        body.appendChild(pop);

        // Place below the token without covering story-tools audio controls.
        try {
            const tokenRect = tokenBtn.getBoundingClientRect();
            const bodyRect = body.getBoundingClientRect();
            const top = tokenRect.bottom - bodyRect.top + body.scrollTop + 6;
            let left = tokenRect.left - bodyRect.left + body.scrollLeft;
            const maxLeft = Math.max(0, body.clientWidth - pop.offsetWidth - 8);
            if (left > maxLeft) left = maxLeft;
            if (left < 0) left = 0;
            pop.style.top = top + 'px';
            pop.style.left = left + 'px';
        } catch (err) { /* ignore layout errors */ }

        const closeBtn = pop.querySelector('[data-action="gloss-dismiss"]');
        if (closeBtn && typeof closeBtn.focus === 'function') closeBtn.focus();
        if (!found.meaning && window.HebrewWordLookup && typeof window.HebrewWordLookup.meanings === 'function') {
            window.HebrewWordLookup.meanings(found.token).then(function (line) {
                if (!line || !pop.parentNode) return;
                const missing = pop.querySelector('.story-gloss-missing');
                if (!missing) return;
                missing.className = 'story-gloss-en';
                missing.textContent = line;
            });
        }
    }

    function paintStory(ctx, root) {
        const play = root.querySelector('#story-play');
        const pause = root.querySelector('#story-pause');
        const resume = root.querySelector('#story-resume');
        const stop = root.querySelector('#story-stop');
        const status = Speech.story.status;
        // Idle: Read aloud. Playing: Pause + Stop. Paused: Resume + Stop.
        play?.classList.toggle('hidden', status !== 'idle');
        pause?.classList.toggle('hidden', status !== 'playing');
        resume?.classList.toggle('hidden', status !== 'paused');
        stop?.classList.toggle('hidden', status === 'idle');
        const lines = root.querySelectorAll('#story-text p');
        lines.forEach(p => p.classList.remove('reading-now'));
        const follow = root.querySelector('#follow-along')?.checked;
        if (follow && status !== 'idle' && lines[Speech.story.index]) {
            lines[Speech.story.index].classList.add('reading-now');
            if (status === 'playing') lines[Speech.story.index].scrollIntoView({ block: 'center', behavior: 'smooth' });
        }
    }
})();
