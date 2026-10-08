(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;
    const TRY_AGAIN = 'נסו שוב (Try again)';

    P.register({
        type: 'listen-pick',
        labelHe: 'הקשיבו ובחרו',
        labelEn: 'Listen and pick',
        icon: 'fa-ear-listen',
        tone: 'tone-blue',
        available(ws) { return (ws.vocab?.items || []).length > 0; },
        defaults(level, levelDefaults) {
            return { mode: 'meaning', choices: levelDefaults.choices || 3 };
        },
        render(ctx, config, state) {
            return `<div class="card stack" id="listen-root">
                <p class="fine">${config.mode === 'form'
                    ? ctx.bi('הקשיבו ובחרו את הצורה הכתובה', 'Listen and pick the written form')
                    : ctx.bi('הקשיבו ובחרו את המשמעות', 'Listen and pick the meaning')}</p>
                <div id="listen-body"></div>
            </div>`;
        },
        mount(ctx, root, config, state) {
            root._listen = {
                queue: Array.isArray(state.queue) && state.queue.length
                    ? state.queue.slice()
                    : U.shuffle((ctx.ws.vocab.items || []).map(i => ctx.sid(i.id))),
                index: state.index || 0,
                score: state.score || 0,
                mode: config.mode || 'meaning',
                choices: config.choices || 3,
                answered: false,
                attempts: Object.assign({}, state.attempts || {}),
                recorded: Object.assign({}, state.recorded || {}),
                optionsByItem: Object.assign({}, state.optionsByItem || {})
            };
            show(ctx, root);
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'listen-again') {
                const item = current(ctx, root);
                if (item) ctx.Speech.play(item.word, item.audio);
                return;
            }
            if (action.dataset.action !== 'listen-pick') return;
            if (root._listen.answered) return;
            const item = current(ctx, root);
            if (!item) return;
            const id = ctx.sid(item.id);
            const attempts = Number(root._listen.attempts[id] || 0);
            if (attempts >= 2) return;

            const pick = action.dataset.value;
            const expected = root._listen.mode === 'form' ? item.word : item.meaning;
            const good = root._listen.mode === 'form'
                ? U.normalizeHebrew(pick) === U.normalizeHebrew(expected)
                : String(pick).toLowerCase() === String(expected).toLowerCase();

            if (good) {
                root._listen.answered = true;
                action.classList.add('correct');
                if (attempts === 0 && !root._listen.recorded[id]) {
                    recordFirstAttempt(ctx, root, item, 'pass');
                }
                root._listen.attempts[id] = 0;
                root._listen.score += 1;
                clearHint(root);
                ctx.save();
                setTimeout(() => advance(ctx, root), 450);
                return;
            }

            const nextAttempts = attempts + 1;
            root._listen.attempts[id] = nextAttempts;
            action.classList.add('wrong');

            if (nextAttempts === 1) {
                root._listen.answered = true;
                recordFirstAttempt(ctx, root, item, 'miss');
                showHint(root, TRY_AGAIN);
                ctx.save();
                // Allow one retry without revealing the correct option.
                setTimeout(() => {
                    root._listen.answered = false;
                }, 350);
                return;
            }

            root._listen.answered = true;
            revealCorrect(root, expected);
            const body = root.querySelector('#listen-body');
            const hint = body && body.querySelector('[data-listen-hint]');
            if (hint) {
                hint.innerHTML = TRY_AGAIN + ' — ' + ctx.esc(expected)
                    + ' ' + ctx.speakButton(item.word, item.audio);
            } else {
                showHint(root, TRY_AGAIN + ' — ' + expected);
                const again = root.querySelector('[data-listen-hint]');
                if (again && item.word) {
                    again.innerHTML = TRY_AGAIN + ' — ' + ctx.esc(expected)
                        + ' ' + ctx.speakButton(item.word, item.audio);
                }
            }
            ctx.Speech.play(item.word, item.audio);
            ctx.save();
            setTimeout(() => advance(ctx, root), 900);
        },
        collect(ctx, root) {
            if (!root._listen) return {};
            return {
                index: root._listen.index,
                score: root._listen.score,
                queue: root._listen.queue,
                attempts: root._listen.attempts,
                recorded: root._listen.recorded,
                optionsByItem: root._listen.optionsByItem
            };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function current(ctx, root) {
        const id = root._listen.queue[root._listen.index];
        return (ctx.ws.vocab.items || []).find(i => ctx.sid(i.id) === id);
    }

    function recordFirstAttempt(ctx, root, item, signal) {
        const id = ctx.sid(item.id);
        if (root._listen.recorded[id]) return;
        root._listen.recorded[id] = signal;
        if (!item.word || !ctx.review || typeof ctx.review.record !== 'function') return;
        try {
            ctx.review.record(item.word, 'he_en', signal, { lesson: ctx.ws.id, activity: 'listen-pick' });
        } catch (err) { /* fail-soft */ }
    }

    function showHint(root, text) {
        const body = root.querySelector('#listen-body');
        if (!body) return;
        let hint = body.querySelector('[data-listen-hint]');
        if (!hint) {
            hint = document.createElement('p');
            hint.className = 'match-feedback';
            hint.setAttribute('data-listen-hint', '1');
            body.appendChild(hint);
        }
        hint.textContent = text;
        hint.classList.remove('hidden');
    }

    function clearHint(root) {
        const hint = root.querySelector('[data-listen-hint]');
        if (hint) hint.remove();
    }

    function revealCorrect(root, expected) {
        root.querySelectorAll('[data-action="listen-pick"]').forEach(btn => {
            const val = btn.dataset.value;
            const match = root._listen.mode === 'form'
                ? U.normalizeHebrew(val) === U.normalizeHebrew(expected)
                : String(val).toLowerCase() === String(expected).toLowerCase();
            if (match) btn.classList.add('correct');
        });
    }

    function advance(ctx, root) {
        root._listen.index += 1;
        root._listen.answered = false;
        show(ctx, root);
    }

    function optionsFor(ctx, root, item) {
        const id = ctx.sid(item.id);
        if (Array.isArray(root._listen.optionsByItem[id]) && root._listen.optionsByItem[id].length) {
            return root._listen.optionsByItem[id];
        }
        let options;
        if (root._listen.mode === 'form') {
            options = U.shuffle([item.word].concat(
                U.shuffle(ctx.ws.vocab.items.filter(i => ctx.sid(i.id) !== id))
                    .slice(0, root._listen.choices - 1)
                    .map(i => i.word)
            ));
        } else {
            options = U.shuffle([item.meaning].concat(
                U.shuffle(ctx.ws.vocab.items.filter(i => ctx.sid(i.id) !== id))
                    .slice(0, root._listen.choices - 1)
                    .map(i => i.meaning)
            ));
        }
        root._listen.optionsByItem[id] = options;
        return options;
    }

    function show(ctx, root) {
        const body = root.querySelector('#listen-body');
        if (root._listen.index >= root._listen.queue.length) {
            body.innerHTML = `<p>${ctx.bi('סיום', 'Done')}: ${root._listen.score} / ${root._listen.queue.length}</p>`;
            ctx.markComplete(ctx.stepIndex);
            ctx.save();
            return;
        }
        const item = current(ctx, root);
        if (!item) {
            root._listen.index += 1;
            show(ctx, root);
            return;
        }
        const id = ctx.sid(item.id);
        const attempts = Number(root._listen.attempts[id] || 0);
        const options = optionsFor(ctx, root, item);
        const expected = root._listen.mode === 'form' ? item.word : item.meaning;
        body.innerHTML = `
            <p class="pill">${root._listen.index + 1} / ${root._listen.queue.length}</p>
            <button type="button" class="btn btn-amber btn-wide" data-action="listen-again">${U.icon('volume-high')} ${ctx.bi('השמעה', 'Play')}</button>
            <div class="choices">
                ${options.map(opt => `<button type="button" class="choice-card" data-action="listen-pick" data-value="${ctx.esc(opt)}">${ctx.esc(opt)}</button>`).join('')}
            </div>
            <p class="match-feedback${attempts ? '' : ' hidden'}" data-listen-hint="1">${
                attempts >= 2
                    ? TRY_AGAIN + ' — ' + ctx.esc(expected) + ' ' + ctx.speakButton(item.word, item.audio)
                    : attempts === 1 ? TRY_AGAIN : ''
            }</p>`;
        if (attempts >= 2) {
            revealCorrect(root, expected);
            root._listen.answered = true;
        }
        ctx.Speech.play(item.word, item.audio);
        ctx.save();
    }
})();
