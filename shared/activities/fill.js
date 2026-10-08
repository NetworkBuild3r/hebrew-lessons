(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    P.register({
        type: 'fill',
        labelHe: 'השלמה',
        labelEn: 'Fill',
        icon: 'fa-pen-to-square',
        tone: 'tone-green',
        available(ws) { return (ws.fill?.sentences || []).length > 0; },
        defaults(level, levelDefaults) {
            return { bank: levelDefaults.bank || 'open' };
        },
        render(ctx, config, state) {
            const sentences = ctx.ws.fill.sentences;
            const bank = ctx.ws.fill.bank || [];
            const answers = Object.fromEntries((state.answers || []).map(a => [String(a.id), a.val]));
            const bankMode = config.bank || ctx.levelDefaults.bank;
            const bankOpen = bankMode === 'open' || (bankMode === 'closed' && state.bankOpen);
            return `
                <div class="card stack">
                    <p class="fine">${ctx.bi('מלאו את החסר', 'Fill in the blank')}</p>
                    <div class="fill-list" id="fill-list">
                        ${sentences.map(s => {
                            const id = ctx.sid(s.id);
                            const val = answers[id] || '';
                            return `<div class="fill-row">
                                <p class="english" dir="ltr">${ctx.esc(s.english || '')}</p>
                                <p class="fill-line">
                                    ${ctx.esc(s.textPre || '')}
                                    ${s.prefix ? `<span class="prefix">${ctx.esc(s.prefix)}</span>` : ''}
                                    <input class="fill-input" id="fill-input-${id}" data-fill="${id}" value="${ctx.esc(val)}" autocomplete="off" ${bankMode === 'none' ? '' : 'readonly'}>
                                    ${ctx.esc(s.textPost || '')}
                                </p>
                            </div>`;
                        }).join('')}
                    </div>
                    ${bankMode === 'none' ? '' : `
                        ${bankMode === 'closed' && !bankOpen ? `<button type="button" class="btn btn-ghost" data-action="open-bank">${ctx.bi('פתחו את בנק המילים', 'Open word bank')}</button>` : ''}
                        <div class="word-bank${bankOpen || bankMode === 'open' ? '' : ' hidden'}" id="word-bank">
                            ${bank.map(word => `<button type="button" class="bank-word" data-bank-word="${ctx.esc(word)}">${ctx.esc(word)}</button>`).join('')}
                        </div>
                    `}
                    <div id="fill-feedback" class="match-feedback hidden"></div>
                </div>`;
        },
        mount(ctx, root, config, state) {
            root._fill = { active: null, bankOpen: !!(state.bankOpen) || config.bank === 'open' };
            refreshBank(ctx, root);
        },
        onClick(ctx, root, event, action) {
            if (action && action.dataset.action === 'open-bank') {
                root._fill.bankOpen = true;
                root.querySelector('#word-bank')?.classList.remove('hidden');
                action.classList.add('hidden');
                ctx.save();
                return;
            }
            if (action && action.dataset.action === 'check') {
                check(ctx, root);
                return;
            }
            const input = event.target.closest('[data-fill]');
            if (input) {
                root._fill.active = input.dataset.fill;
                root.querySelectorAll('.fill-input').forEach(el => el.classList.toggle('active', el === input));
            }
            const word = event.target.closest('[data-bank-word]');
            if (word && root._fill.active) {
                const field = root.querySelector('#fill-input-' + root._fill.active);
                if (field) {
                    field.value = word.dataset.bankWord;
                    refreshBank(ctx, root);
                    ctx.save();
                }
            }
        },
        collect(ctx, root) {
            const answers = (ctx.ws.fill.sentences || []).map(s => ({
                id: s.id,
                val: root.querySelector('#fill-input-' + ctx.sid(s.id))?.value || ''
            }));
            return { answers, bankOpen: !!(root._fill && root._fill.bankOpen) };
        },
        actionBar(ctx) {
            return `
                <button type="button" class="btn btn-ghost" data-action="check">${ctx.bi('בדיקה', 'Check')}</button>
                <button type="button" class="btn btn-primary" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function refreshBank(ctx, root) {
        const used = new Set((ctx.ws.fill.sentences || []).map(s => {
            const val = root.querySelector('#fill-input-' + ctx.sid(s.id))?.value || '';
            return U.normalizeHebrew(val);
        }).filter(Boolean));
        root.querySelectorAll('[data-bank-word]').forEach(btn => {
            btn.classList.toggle('used', used.has(U.normalizeHebrew(btn.dataset.bankWord)));
        });
    }

    function check(ctx, root) {
        let ok = 0;
        (ctx.ws.fill.sentences || []).forEach(s => {
            const input = root.querySelector('#fill-input-' + ctx.sid(s.id));
            if (!input) return;
            const good = U.normalizeHebrew(input.value) === U.normalizeHebrew(s.answer);
            input.classList.toggle('correct', good);
            input.classList.toggle('wrong', !good && input.value);
            if (good) ok += 1;
        });
        const fb = root.querySelector('#fill-feedback');
        if (fb) {
            fb.classList.remove('hidden');
            fb.textContent = ok + ' / ' + ctx.ws.fill.sentences.length;
        }
        if (ok === ctx.ws.fill.sentences.length) ctx.markComplete(ctx.stepIndex);
        ctx.save();
    }
})();
