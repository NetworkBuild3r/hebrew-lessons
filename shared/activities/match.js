(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    P.register({
        type: 'match',
        labelHe: 'התאמה',
        labelEn: 'Match',
        icon: 'fa-table-cells-large',
        tone: 'tone-green',
        available(ws) { return (ws.vocab?.items || []).length > 0; },
        render(ctx, config, state) {
            const items = ctx.ws.vocab.items;
            const matched = new Set((state.matchedPairs || []).map(String));
            const en = U.shuffle(items.slice());
            const he = U.shuffle(items.slice());
            return `
                <div class="card stack">
                    <p class="fine">${ctx.bi('בחרו מילה בעברית ותרגום באנגלית', 'Tap a Hebrew word and its English meaning')}</p>
                    <div class="match-grid">
                        <div>
                            <h3 class="col-title">English</h3>
                            <div class="list" id="match-english-list">
                                ${en.map(item => btn(ctx, 'en', item, item.meaning, matched)).join('')}
                            </div>
                        </div>
                        <div class="he-list">
                            <h3 class="col-title">עברית</h3>
                            <div class="list" id="match-hebrew-list">
                                ${he.map(item => btn(ctx, 'he', item, item.word, matched)).join('')}
                            </div>
                        </div>
                    </div>
                    <div id="match-feedback" class="match-feedback${matched.size === items.length ? '' : ' hidden'}">${matched.size === items.length ? ctx.bi('כל הזוגות!', 'All matched!') : ''}</div>
                </div>`;
        },
        mount(ctx, root, config, state) {
            root._match = {
                selectedHe: null,
                selectedEn: null,
                matched: new Set((state.matchedPairs || []).map(String))
            };
        },
        onClick(ctx, root, event, action) {
            const card = event.target.closest('[data-match]');
            if (!card || !root._match) return;
            const side = card.dataset.match;
            const id = card.dataset.id;
            if (root._match.matched.has(id)) return;
            if (side === 'he') {
                root.querySelectorAll('[data-match="he"]').forEach(el => el.classList.remove('selected'));
                card.classList.add('selected');
                root._match.selectedHe = id;
            } else {
                root.querySelectorAll('[data-match="en"]').forEach(el => el.classList.remove('selected'));
                card.classList.add('selected');
                root._match.selectedEn = id;
            }
            if (root._match.selectedHe && root._match.selectedEn) {
                if (root._match.selectedHe === root._match.selectedEn) {
                    root._match.matched.add(root._match.selectedHe);
                    root.querySelectorAll('[data-id="' + root._match.selectedHe + '"]').forEach(el => {
                        el.classList.add('matched');
                        el.classList.remove('selected');
                    });
                    if (root._match.matched.size === ctx.ws.vocab.items.length) {
                        const fb = root.querySelector('#match-feedback');
                        if (fb) {
                            fb.classList.remove('hidden');
                            fb.textContent = 'כל הזוגות! (All matched!)';
                        }
                        ctx.markComplete(ctx.stepIndex);
                    }
                    ctx.save();
                } else {
                    root.querySelectorAll('.selected').forEach(el => el.classList.remove('selected'));
                }
                root._match.selectedHe = null;
                root._match.selectedEn = null;
            }
        },
        collect(ctx, root) {
            return { matchedPairs: root._match ? [...root._match.matched] : [] };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function btn(ctx, side, item, label, matched) {
        const id = ctx.sid(item.id);
        return `<button type="button" class="word-match-card${matched.has(id) ? ' matched' : ''}" data-match="${side}" data-id="${id}">${ctx.esc(label)}</button>`;
    }
})();
