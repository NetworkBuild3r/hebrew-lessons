(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;
    const TRY_AGAIN = 'נסו שוב (Try again)';

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
                            return `<div class="fill-row" data-fill-row="${id}">
                                <p class="english" dir="ltr">${ctx.esc(s.english || '')}</p>
                                <p class="fill-line">
                                    ${ctx.esc(s.textPre || '')}
                                    ${s.prefix ? `<span class="prefix">${ctx.esc(s.prefix)}</span>` : ''}
                                    <input class="fill-input" id="fill-input-${id}" data-fill="${id}" value="${ctx.esc(val)}" autocomplete="off" ${bankMode === 'none' ? '' : 'readonly'}>
                                    ${ctx.esc(s.textPost || '')}
                                </p>
                                <div class="fill-item-feedback fine hidden" data-fill-feedback="${id}"></div>
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
            root._fill = {
                active: null,
                bankOpen: !!(state.bankOpen) || config.bank === 'open',
                attempts: Object.assign({}, state.attempts || {}),
                recorded: Object.assign({}, state.recorded || {})
            };
            refreshBank(ctx, root);
            restoreFeedback(ctx, root);
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
                const id = input.dataset.fill;
                if (Number(root._fill.attempts[id] || 0) >= 2) return;
                root._fill.active = id;
                root.querySelectorAll('.fill-input').forEach(el => el.classList.toggle('active', el === input));
            }
            const word = event.target.closest('[data-bank-word]');
            if (word && root._fill.active) {
                const id = root._fill.active;
                if (Number(root._fill.attempts[id] || 0) >= 2) return;
                const field = root.querySelector('#fill-input-' + id);
                if (field) {
                    field.value = word.dataset.bankWord;
                    field.classList.remove('correct', 'wrong');
                    const itemFb = root.querySelector('[data-fill-feedback="' + id + '"]');
                    if (itemFb && Number(root._fill.attempts[id] || 0) < 2) {
                        itemFb.classList.add('hidden');
                        itemFb.innerHTML = '';
                    }
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
            return {
                answers,
                bankOpen: !!(root._fill && root._fill.bankOpen),
                attempts: root._fill ? root._fill.attempts : {},
                recorded: root._fill ? root._fill.recorded : {}
            };
        },
        actionBar(ctx) {
            return `
                <button type="button" class="btn btn-ghost" data-action="check">${ctx.bi('בדיקה', 'Check')}</button>
                <button type="button" class="btn btn-primary" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function vocabWordForSentence(ctx, sentence) {
        if (!sentence) return null;
        if (sentence.word) return sentence.word;
        const target = U.normalizeHebrew(sentence.answer || '');
        if (!target) return null;
        const match = (ctx.ws.vocab?.items || []).find(v => v && v.word && U.normalizeHebrew(v.word) === target);
        return match ? match.word : null;
    }

    function recordFirstAttempt(ctx, root, id, sentence, signal) {
        if (root._fill.recorded[id]) return;
        root._fill.recorded[id] = signal;
        const word = vocabWordForSentence(ctx, sentence);
        if (!word || !ctx.review || typeof ctx.review.record !== 'function') return;
        try {
            ctx.review.record(word, 'en_he', signal, { lesson: ctx.ws.id, activity: 'fill' });
        } catch (err) { /* fail-soft */ }
    }

    function revealHtml(ctx, sentence) {
        const pointed = sentence.pointed || sentence.answer || '';
        const audio = sentence.audio || null;
        return ctx.esc(pointed) + (pointed ? ' ' + ctx.speakButton(pointed, audio) : '');
    }

    function paintSentence(ctx, root, sentence, mode) {
        const id = ctx.sid(sentence.id);
        const input = root.querySelector('#fill-input-' + id);
        const itemFb = root.querySelector('[data-fill-feedback="' + id + '"]');
        if (!input) return;
        const good = U.normalizeHebrew(input.value) === U.normalizeHebrew(sentence.answer);
        input.classList.remove('correct', 'wrong');
        if (mode === 'correct' || (mode === 'reveal' && good)) {
            input.classList.add('correct');
        } else if (mode === 'retry' || mode === 'reveal') {
            input.classList.add('wrong');
        }
        if (itemFb) {
            itemFb.classList.remove('hidden');
            if (mode === 'retry') {
                itemFb.textContent = TRY_AGAIN;
            } else if (mode === 'reveal') {
                itemFb.innerHTML = TRY_AGAIN + ' — ' + revealHtml(ctx, sentence);
            } else if (mode === 'correct') {
                itemFb.innerHTML = ctx.bi('נכון', 'Correct') + (sentence.pointed || sentence.answer
                    ? ' ' + ctx.speakButton(sentence.pointed || sentence.answer, sentence.audio)
                    : '');
            }
        }
        if (mode === 'reveal' || mode === 'correct') {
            input.readOnly = true;
        }
    }

    function restoreFeedback(ctx, root) {
        (ctx.ws.fill.sentences || []).forEach(s => {
            const id = ctx.sid(s.id);
            const input = root.querySelector('#fill-input-' + id);
            if (!input) return;
            const attempts = Number(root._fill.attempts[id] || 0);
            const val = input.value || '';
            if (!val && !attempts) return;
            const good = U.normalizeHebrew(val) === U.normalizeHebrew(s.answer);
            if (good) paintSentence(ctx, root, s, 'correct');
            else if (attempts >= 2) paintSentence(ctx, root, s, 'reveal');
            else if (attempts === 1) paintSentence(ctx, root, s, 'retry');
        });
    }

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
            const id = ctx.sid(s.id);
            const input = root.querySelector('#fill-input-' + id);
            if (!input) return;
            const val = input.value || '';
            if (!val) return;
            const good = U.normalizeHebrew(val) === U.normalizeHebrew(s.answer);
            const attempts = Number(root._fill.attempts[id] || 0);

            if (good) {
                if (attempts === 0 && !root._fill.recorded[id]) {
                    recordFirstAttempt(ctx, root, id, s, 'pass');
                }
                root._fill.attempts[id] = 0;
                paintSentence(ctx, root, s, 'correct');
                ok += 1;
                return;
            }

            const nextAttempts = attempts + 1;
            root._fill.attempts[id] = nextAttempts;
            if (nextAttempts === 1) {
                recordFirstAttempt(ctx, root, id, s, 'miss');
                paintSentence(ctx, root, s, 'retry');
            } else {
                paintSentence(ctx, root, s, 'reveal');
            }
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
