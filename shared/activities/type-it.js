(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    P.register({
        type: 'type-it',
        labelHe: 'הקלידו',
        labelEn: 'Type it',
        icon: 'fa-keyboard',
        tone: 'tone-purple',
        available(ws, config) {
            const level = U.levelOf(ws);
            if (level !== 'advanced' && !(config && config.force)) return false;
            return (ws.vocab?.items || []).length > 0;
        },
        render(ctx, config, state) {
            return `<div class="card stack" id="type-root">
                <p class="fine">${ctx.bi('ראו את האנגלית והקלידו עברית. הניקוד לא נבדק.', 'See the English and type the Hebrew. Vowels are ignored.')}</p>
                <div id="type-body"></div>
            </div>`;
        },
        mount(ctx, root, config, state) {
            root._type = {
                queue: U.shuffle((ctx.ws.vocab.items || []).map(i => ctx.sid(i.id))),
                index: state.index || 0,
                score: state.score || 0
            };
            show(ctx, root);
        },
        onClick(ctx, root, event, action) {
            if (!action || action.dataset.action !== 'type-check') return;
            const item = current(ctx, root);
            const input = root.querySelector('#type-input');
            const fb = root.querySelector('#type-feedback');
            if (!item || !input) return;
            const good = U.normalizeHebrew(input.value) === U.normalizeHebrew(item.word);
            if (fb) {
                fb.classList.remove('hidden');
                fb.textContent = good ? 'נכון! (Correct!)' : ('התשובה: ' + item.word + ' (Answer: ' + item.word + ')');
            }
            if (good) root._type.score += 1;
            ctx.save();
            setTimeout(() => {
                root._type.index += 1;
                show(ctx, root);
            }, 600);
        },
        collect(ctx, root) {
            return root._type ? { index: root._type.index, score: root._type.score } : {};
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function current(ctx, root) {
        const id = root._type.queue[root._type.index];
        return (ctx.ws.vocab.items || []).find(i => ctx.sid(i.id) === id);
    }

    function show(ctx, root) {
        const body = root.querySelector('#type-body');
        if (root._type.index >= root._type.queue.length) {
            body.innerHTML = `<p>${ctx.bi('סיום', 'Done')}: ${root._type.score} / ${root._type.queue.length}</p>`;
            ctx.markComplete(ctx.stepIndex);
            return;
        }
        const item = current(ctx, root);
        body.innerHTML = `
            <p class="pill">${root._type.index + 1} / ${root._type.queue.length}</p>
            <p class="english" dir="ltr">${ctx.esc(item.meaning || '')}</p>
            <input id="type-input" class="fill-input" dir="rtl" autocomplete="off" placeholder="עברית...">
            <button type="button" class="btn btn-ghost" data-action="type-check">${ctx.bi('בדיקה', 'Check')}</button>
            <div id="type-feedback" class="match-feedback hidden"></div>`;
        root.querySelector('#type-input')?.focus();
    }
})();
