(function () {
    const P = window.HebrewPlayer;

    P.register({
        type: 'retell',
        labelHe: 'ספרו שוב',
        labelEn: 'Retell',
        icon: 'fa-clock',
        tone: 'tone-amber',
        available(ws) { return !!(ws.reading && ws.reading.retell); },
        render(ctx, config, state) {
            const retell = ctx.ws.reading.retell || {};
            const seconds = config.seconds || retell.seconds || 90;
            const cues = retell.cues || [];
            return `
                <div class="card stack">
                    <h3>${ctx.bi('ספרו את הסיפור', 'Retell the story')}</h3>
                    <p class="fine">${ctx.bi('הסתירו את הסיפור וספרו בקול או בכתב', 'Hide the story and retell it aloud or in writing')}</p>
                    <div class="cue-row">${cues.map(c => `<span class="pill">${ctx.esc(c)}</span>`).join('')}</div>
                    <p class="retell-clock" id="retell-clock">${seconds}s</p>
                    <div class="actions">
                        <button type="button" class="btn btn-ghost" data-action="start-clock" data-seconds="${seconds}">${ctx.bi('התחל שעון', 'Start clock')}</button>
                        <button type="button" class="btn btn-ghost" data-action="toggle-story-hide">${ctx.bi('הסתר / הצג סיפור', 'Hide / show story')}</button>
                    </div>
                    <div class="story-body card${state.hideStory ? ' hidden' : ''}" id="retell-story">
                        ${(ctx.ws.reading.paragraphs || []).map(p => `<p>${window.HebrewUtils.renderParts(Array.isArray(p) ? p : [p])}</p>`).join('')}
                    </div>
                    <textarea id="retell-answer" rows="5" placeholder="כתבו כאן...">${ctx.esc(state.answer || '')}</textarea>
                </div>`;
        },
        mount(ctx, root) {
            root._retell = { timer: null };
        },
        unmount(ctx, root) {
            if (root._retell && root._retell.timer) clearInterval(root._retell.timer);
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'start-clock') {
                let left = Number(action.dataset.seconds) || 90;
                const clock = root.querySelector('#retell-clock');
                if (root._retell.timer) clearInterval(root._retell.timer);
                clock.textContent = left + 's';
                root._retell.timer = setInterval(() => {
                    left -= 1;
                    clock.textContent = Math.max(0, left) + 's';
                    if (left <= 0) clearInterval(root._retell.timer);
                }, 1000);
            }
            if (action.dataset.action === 'toggle-story-hide') {
                root.querySelector('#retell-story')?.classList.toggle('hidden');
            }
        },
        collect(ctx, root) {
            return {
                answer: root.querySelector('#retell-answer')?.value || '',
                hideStory: root.querySelector('#retell-story')?.classList.contains('hidden')
            };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });
})();
