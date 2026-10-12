(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    /** Pair colours cycle after this many; the number badge keeps pairs distinct. */
    const PAIR_COLORS = 8;

    P.register({
        type: 'match',
        labelHe: 'התאמה',
        labelEn: 'Match',
        icon: 'fa-table-cells-large',
        tone: 'tone-green',
        available(ws) { return (ws.vocab?.items || []).length > 0; },
        render(ctx, config, state) {
            const items = ctx.ws.vocab.items;
            const known = new Set(items.map(item => ctx.sid(item.id)));
            // Insertion order is the order the learner matched them, and it is saved.
            const order = (state.matchedPairs || []).map(String).filter(id => known.has(id));
            const matched = new Set(order);
            const pending = items.filter(item => !matched.has(ctx.sid(item.id)));
            const en = U.shuffle(pending.slice());
            const he = U.shuffle(pending.slice());
            const allDone = pending.length === 0;
            return `
                <div class="card stack">
                    <p class="fine">${ctx.bi('בחרו מילה בעברית ותרגום באנגלית', 'Tap a Hebrew word and its English meaning')}</p>
                    <div class="match-grid${allDone ? ' hidden' : ''}" id="match-grid">
                        <div>
                            <h3 class="col-title">English</h3>
                            <div class="list" id="match-english-list">
                                ${en.map(item => btn(ctx, 'en', item, item.meaning)).join('')}
                            </div>
                        </div>
                        <div class="he-list">
                            <h3 class="col-title">עברית</h3>
                            <div class="list" id="match-hebrew-list">
                                ${he.map(item => btn(ctx, 'he', item, item.word)).join('')}
                            </div>
                        </div>
                    </div>
                    <div class="match-pairs${order.length ? '' : ' hidden'}" id="match-pairs">
                        <h3 class="col-title">${ctx.bi('התאמות', 'Matched')}</h3>
                        <ol class="match-pair-list" id="match-pair-list">
                            ${order.map((id, index) => {
                                const item = items.find(i => ctx.sid(i.id) === id);
                                return pairRow(ctx, item, index);
                            }).join('')}
                        </ol>
                    </div>
                    <div id="match-feedback" class="match-feedback${allDone ? '' : ' hidden'}">${allDone ? ctx.bi('כל הזוגות!', 'All matched!') : ''}</div>
                </div>`;
        },
        mount(ctx, root, config, state) {
            const known = new Set(ctx.ws.vocab.items.map(item => ctx.sid(item.id)));
            root._match = {
                selectedHe: null,
                selectedEn: null,
                matched: new Set((state.matchedPairs || []).map(String).filter(id => known.has(id)))
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
            if (!(root._match.selectedHe && root._match.selectedEn)) return;

            if (root._match.selectedHe === root._match.selectedEn) {
                acceptPair(ctx, root, root._match.selectedHe);
            } else {
                const wrong = root.querySelectorAll('.word-match-card.selected');
                wrong.forEach(el => {
                    el.classList.remove('selected');
                    el.classList.add('wrong');
                });
                const Feedback = typeof window !== 'undefined' ? window.HebrewFeedback : null;
                if (Feedback && typeof Feedback.chime === 'function') Feedback.chime('miss');
                // 120ms shake (CSS); clear after a brief flash so the next tap is never blocked.
                setTimeout(() => wrong.forEach(el => el.classList.remove('wrong')), 180);
            }
            root._match.selectedHe = null;
            root._match.selectedEn = null;
        },
        collect(ctx, root) {
            return { matchedPairs: root._match ? [...root._match.matched] : [] };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function acceptPair(ctx, root, id) {
        const items = ctx.ws.vocab.items;
        const item = items.find(i => ctx.sid(i.id) === id);
        root._match.matched.add(id);
        const index = root._match.matched.size - 1;

        root.querySelectorAll('.word-match-card[data-id="' + id + '"]').forEach(el => el.remove());

        const list = root.querySelector('#match-pair-list');
        const panel = root.querySelector('#match-pairs');
        if (list && item) {
            list.insertAdjacentHTML('beforeend', pairRow(ctx, item, index));
        }
        if (panel) panel.classList.remove('hidden');

        // Hear the word as the pair locks in. Falls back to the browser voice when no clip exists.
        if (item && ctx.Speech && typeof ctx.Speech.play === 'function') {
            ctx.Speech.play(item.word, item.audio || null);
        }

        const Feedback = typeof window !== 'undefined' ? window.HebrewFeedback : null;
        if (Feedback && typeof Feedback.chime === 'function') Feedback.chime('correct');

        if (root._match.matched.size === items.length) {
            const grid = root.querySelector('#match-grid');
            if (grid) grid.classList.add('hidden');
            const fb = root.querySelector('#match-feedback');
            if (fb) {
                if (Feedback && typeof Feedback.showCorrect === 'function') {
                    Feedback.showCorrect(fb, {
                        messageHe: 'כל הזוגות!',
                        messageEn: 'All matched!'
                    });
                } else {
                    fb.classList.remove('hidden');
                    fb.textContent = 'כל הזוגות! (All matched!)';
                }
            }
            ctx.markComplete(ctx.stepIndex);
        }
        ctx.save();
    }

    function btn(ctx, side, item, label) {
        const id = ctx.sid(item.id);
        return `<button type="button" class="word-match-card" data-match="${side}" data-id="${id}">${ctx.esc(label)}</button>`;
    }

    /** One finished pair: English and Hebrew side by side, in its own colour, with a number. */
    function pairRow(ctx, item, index) {
        if (!item) return '';
        const id = ctx.sid(item.id);
        const color = index % PAIR_COLORS;
        return `<li class="match-pair pair-c${color}" data-pair-id="${id}">
            <span class="pair-num" aria-hidden="true">${index + 1}</span>
            <span class="pair-en" dir="ltr">${ctx.esc(item.meaning)}</span>
            <span class="pair-he" dir="rtl" lang="he">${ctx.esc(item.word)}</span>
            <span class="pair-check" aria-hidden="true">✓</span>
            ${ctx.speakButton(item.word, item.audio, 'Word')}
        </li>`;
    }
})();
