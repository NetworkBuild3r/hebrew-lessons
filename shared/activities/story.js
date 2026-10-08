(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;
    const Speech = window.HebrewSpeech;
    const PREFS_KEY = 'hebrew-worksheet-prefs';

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
            return `
                <div class="stack">
                    <div class="banner banner-amber">
                        <h3>${ctx.bi('קראו את הטקסט', 'Read the text')}</h3>
                        <p class="fine">${ctx.bi('סגול = דיבור עקיף, אדום = דיבור ישיר', 'Purple = reported speech, red = direct speech')}</p>
                    </div>
                    <div class="story-tools">
                        <button type="button" class="btn btn-ghost" data-action="toggle-vowels">${ctx.vowelsOn ? ctx.bi('הסתר ניקוד', 'Hide vowels') : ctx.bi('הצג ניקוד', 'Show vowels')}</button>
                        <label class="follow-along"><input type="checkbox" id="follow-along"${follow ? ' checked' : ''}> ${ctx.bi('הדגשת השורה', 'Follow along')}</label>
                        <button type="button" class="btn btn-amber" id="story-play" data-action="story-play"><i class="fa-solid fa-volume-high"></i> ${ctx.bi('הקראת טקסט', 'Read aloud')}</button>
                        <button type="button" class="btn btn-ghost hidden" id="story-pause" data-action="story-pause"><i class="fa-solid fa-pause"></i> ${ctx.bi('השהיה', 'Pause')}</button>
                        <button type="button" class="btn btn-stop hidden" id="story-stop" data-action="story-stop"><i class="fa-solid fa-stop"></i> ${ctx.bi('עצירה', 'Stop')}</button>
                    </div>
                    <div class="story-body card" id="story-text">
                        ${paragraphs.map(p => `<p data-vowels-html="1">${U.renderParts(Array.isArray(p) ? p : [p])}</p>`).join('')}
                    </div>
                    ${questions.length ? `
                        <div class="card stack">
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
        },
        unmount() {
            Speech.onStoryPaint = null;
            Speech.stopStory();
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'toggle-vowels') {
                ctx.vowelsOn = !ctx.vowelsOn;
                applyVowels(ctx, root);
                action.textContent = ctx.vowelsOn ? 'הסתר ניקוד (Hide vowels)' : 'הצג ניקוד (Show vowels)';
                ctx.save();
            }
            if (action.dataset.action === 'story-play') {
                const lines = storyLines(ctx);
                const from = Speech.story.status === 'paused' ? Speech.story.index : 0;
                Speech.startStory(lines, from, ctx.ws.reading.audio || []);
            }
            if (action.dataset.action === 'story-pause') Speech.pauseStory();
            if (action.dataset.action === 'story-stop') Speech.stopStory();
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

    function storyLines(ctx) {
        return U.storyPlain(ctx.ws.reading.paragraphs).map(line => ctx.vowelsOn ? line : U.stripVowels(line));
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

    function paintStory(ctx, root) {
        const play = root.querySelector('#story-play');
        const pause = root.querySelector('#story-pause');
        const stop = root.querySelector('#story-stop');
        const status = Speech.story.status;
        if (play) {
            play.classList.toggle('hidden', status === 'playing');
            play.innerHTML = status === 'paused'
                ? '<i class="fa-solid fa-play"></i> המשך (Resume)'
                : '<i class="fa-solid fa-volume-high"></i> הקראת טקסט (Read aloud)';
        }
        pause?.classList.toggle('hidden', status !== 'playing');
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
