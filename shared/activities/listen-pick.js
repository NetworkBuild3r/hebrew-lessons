(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

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
                queue: U.shuffle((ctx.ws.vocab.items || []).map(i => ctx.sid(i.id))),
                index: state.index || 0,
                score: state.score || 0,
                mode: config.mode || 'meaning',
                choices: config.choices || 3,
                answered: false
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
            root._listen.answered = true;
            const item = current(ctx, root);
            const pick = action.dataset.value;
            const expected = root._listen.mode === 'form' ? item.word : item.meaning;
            const good = root._listen.mode === 'form'
                ? U.normalizeHebrew(pick) === U.normalizeHebrew(expected)
                : String(pick).toLowerCase() === String(expected).toLowerCase();
            action.classList.add(good ? 'correct' : 'wrong');
            if (good) root._listen.score += 1;
            ctx.save();
            setTimeout(() => {
                root._listen.index += 1;
                root._listen.answered = false;
                show(ctx, root);
            }, 450);
        },
        collect(ctx, root) {
            return root._listen ? { index: root._listen.index, score: root._listen.score } : {};
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function current(ctx, root) {
        const id = root._listen.queue[root._listen.index];
        return (ctx.ws.vocab.items || []).find(i => ctx.sid(i.id) === id);
    }

    function show(ctx, root) {
        const body = root.querySelector('#listen-body');
        if (root._listen.index >= root._listen.queue.length) {
            body.innerHTML = `<p>${ctx.bi('סיום', 'Done')}: ${root._listen.score} / ${root._listen.queue.length}</p>`;
            ctx.markComplete(ctx.stepIndex);
            return;
        }
        const item = current(ctx, root);
        if (!item) {
            root._listen.index += 1;
            show(ctx, root);
            return;
        }
        let options;
        if (root._listen.mode === 'form') {
            options = U.shuffle([item.word].concat(
                U.shuffle(ctx.ws.vocab.items.filter(i => ctx.sid(i.id) !== ctx.sid(item.id)))
                    .slice(0, root._listen.choices - 1)
                    .map(i => i.word)
            ));
        } else {
            options = U.shuffle([item.meaning].concat(
                U.shuffle(ctx.ws.vocab.items.filter(i => ctx.sid(i.id) !== ctx.sid(item.id)))
                    .slice(0, root._listen.choices - 1)
                    .map(i => i.meaning)
            ));
        }
        body.innerHTML = `
            <p class="pill">${root._listen.index + 1} / ${root._listen.queue.length}</p>
            <button type="button" class="btn btn-amber btn-wide" data-action="listen-again">${U.icon('volume-high')} ${ctx.bi('השמעה', 'Play')}</button>
            <div class="choices">
                ${options.map(opt => `<button type="button" class="choice-card" data-action="listen-pick" data-value="${ctx.esc(opt)}">${ctx.esc(opt)}</button>`).join('')}
            </div>`;
        ctx.Speech.play(item.word, item.audio);
    }
})();
