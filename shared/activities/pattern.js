(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;
    const TRY_AGAIN = 'נסו שוב (Try again)';

    P.register({
        type: 'pattern',
        labelHe: 'דפוס',
        labelEn: 'Pattern',
        icon: 'fa-puzzle-piece',
        tone: 'tone-purple',
        available(ws) { return (ws.grammar?.items || []).length > 0; },
        render(ctx, config, state) {
            const g = ctx.ws.grammar;
            const answers = state.answers || {};
            return `
                <div class="stack">
                    <div class="banner banner-purple">
                        <h3>${ctx.esc(g.heading || g.tabLabel || 'Pattern')}</h3>
                        <p>${ctx.esc(g.lede || '')}</p>
                    </div>
                    ${(g.rules || []).map(rule => `
                        <div class="card pattern-rule">
                            <h4>${ctx.esc(rule.title || '')}</h4>
                            <p>${ctx.esc(rule.english || '')}</p>
                            ${rule.example ? `<div class="pattern-box example">${window.HebrewUtils.renderParts(Array.isArray(rule.example) ? rule.example : [rule.example])}</div>` : ''}
                        </div>
                    `).join('')}
                    ${(g.reminders || []).length ? `<ul class="reminders">${g.reminders.map(r => `<li>${ctx.esc(r)}</li>`).join('')}</ul>` : ''}
                    <p class="fine">${ctx.esc(g.prompt || 'בחרו את המשפט הנכון.')}</p>
                    <div class="grammar-list">
                        ${(g.items || []).map(item => {
                            const id = ctx.sid(item.id);
                            const selected = answers[id];
                            return `<div class="card grammar-item" data-grammar="${id}">
                                <p class="english" dir="ltr">${ctx.esc(item.direct || '')}</p>
                                <div class="choices">
                                    ${(item.choices || []).map((choice, index) => `
                                        <button type="button" class="choice-card${selected === index ? ' selected' : ''}" data-action="pick-grammar" data-id="${id}" data-index="${index}">${ctx.esc(choice)}</button>
                                    `).join('')}
                                </div>
                                <div class="grammar-result hidden" data-result="${id}"></div>
                            </div>`;
                        }).join('')}
                    </div>
                </div>`;
        },
        mount(ctx, root, config, state) {
            root._grammar = {
                answers: Object.assign({}, state.answers || {}),
                attempts: Object.assign({}, state.attempts || {}),
                recorded: Object.assign({}, state.recorded || {})
            };
            restoreFeedback(ctx, root);
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'pick-grammar') {
                const id = action.dataset.id;
                const index = Number(action.dataset.index);
                const item = (ctx.ws.grammar.items || []).find(i => ctx.sid(i.id) === id);
                const attempts = Number(root._grammar.attempts[id] || 0);
                const solved = !!(item && root._grammar.answers[id] === item.answer
                    && root.querySelector('[data-result="' + id + '"]:not(.hidden)'));
                if (attempts >= 2 || solved) return;
                root._grammar.answers[id] = index;
                const wrap = root.querySelector('[data-grammar="' + id + '"]');
                wrap?.querySelectorAll('.choice-card').forEach(btn => {
                    btn.classList.toggle('selected', Number(btn.dataset.index) === index);
                    btn.classList.remove('correct', 'wrong');
                });
                ctx.save();
            }
            if (action.dataset.action === 'check') check(ctx, root);
        },
        collect(ctx, root) {
            if (!root._grammar) return { answers: {}, attempts: {}, recorded: {} };
            return {
                answers: root._grammar.answers,
                attempts: root._grammar.attempts,
                recorded: root._grammar.recorded
            };
        },
        actionBar(ctx) {
            return `
                <button type="button" class="btn btn-ghost" data-action="check">${ctx.bi('בדיקה', 'Check')}</button>
                <button type="button" class="btn btn-primary" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function vocabWordsForItem(ctx, item) {
        const vocab = ctx.ws.vocab?.items || [];
        if (item && item.word) {
            const direct = vocab.find(v => U.normalizeHebrew(v.word) === U.normalizeHebrew(item.word));
            return direct && direct.word ? [direct.word] : [item.word];
        }
        const haystack = U.normalizeHebrew(
            (item && item.pointed) || (item && item.choices && item.choices[item.answer]) || ''
        );
        if (!haystack) return [];
        return vocab
            .filter(v => v && v.word && haystack.indexOf(U.normalizeHebrew(v.word)) !== -1)
            .map(v => v.word);
    }

    function recordFirstAttempt(ctx, root, id, item, signal) {
        if (root._grammar.recorded[id]) return;
        root._grammar.recorded[id] = signal;
        const words = vocabWordsForItem(ctx, item);
        if (!words.length || !ctx.review || typeof ctx.review.record !== 'function') return;
        words.forEach(word => {
            try {
                ctx.review.record(word, 'en_he', signal, { lesson: ctx.ws.id, activity: 'pattern' });
            } catch (err) { /* fail-soft */ }
        });
    }

    function paintItem(ctx, root, item, mode) {
        const id = ctx.sid(item.id);
        const picked = root._grammar.answers[id];
        const result = root.querySelector('[data-result="' + id + '"]');
        const wrap = root.querySelector('[data-grammar="' + id + '"]');
        const good = picked === item.answer;

        wrap?.querySelectorAll('.choice-card').forEach(btn => {
            const idx = Number(btn.dataset.index);
            btn.classList.remove('correct', 'wrong');
            if (mode === 'reveal') {
                btn.classList.toggle('correct', idx === item.answer);
                btn.classList.toggle('wrong', picked === idx && idx !== item.answer);
            } else if (mode === 'retry' && picked === idx && !good) {
                btn.classList.add('wrong');
            } else if (mode === 'correct' && good) {
                btn.classList.toggle('correct', idx === item.answer);
            }
        });

        if (!result) return;
        result.classList.remove('hidden');
        if (mode === 'retry') {
            // First miss: nudge only — answer stays hidden until second attempt.
            result.textContent = TRY_AGAIN;
            return;
        }
        if (mode === 'correct' || mode === 'reveal') {
            const Feedback = window.HebrewFeedback;
            const pointed = item.pointed || '';
            if (good && Feedback && typeof Feedback.showCorrect === 'function') {
                Feedback.showCorrect(result);
                if (pointed) result.insertAdjacentHTML('beforeend', ' ' + ctx.speakButton(pointed, item.audio));
            } else if (!good && Feedback && typeof Feedback.showMiss === 'function') {
                Feedback.showMiss(result, {
                    pointed: pointed || (item.choices && item.choices[item.answer]) || '',
                    showRetry: false
                });
                const why = item.why ? '<p class="fine">' + ctx.esc(item.why) + '</p>' : '';
                const speak = pointed ? ' ' + ctx.speakButton(pointed, item.audio) : '';
                result.insertAdjacentHTML('beforeend', why + speak);
            } else {
                result.innerHTML = (good
                    ? ctx.bi('נכון', 'Correct')
                    : ctx.esc(item.why || 'לא מדויק.'))
                    + (pointed ? ' ' + ctx.speakButton(pointed, item.audio) : '');
            }
        }
    }

    function restoreFeedback(ctx, root) {
        (ctx.ws.grammar.items || []).forEach(item => {
            const id = ctx.sid(item.id);
            const attempts = Number(root._grammar.attempts[id] || 0);
            const picked = root._grammar.answers[id];
            if (picked == null && !attempts) return;
            const good = picked === item.answer;
            if (good) paintItem(ctx, root, item, 'correct');
            else if (attempts >= 2) paintItem(ctx, root, item, 'reveal');
            else if (attempts === 1) paintItem(ctx, root, item, 'retry');
        });
    }

    function check(ctx, root) {
        let ok = 0;
        (ctx.ws.grammar.items || []).forEach(item => {
            const id = ctx.sid(item.id);
            const picked = root._grammar.answers[id];
            if (picked == null) return;
            const good = picked === item.answer;
            const attempts = Number(root._grammar.attempts[id] || 0);

            if (good) {
                if (attempts === 0 && !root._grammar.recorded[id]) {
                    recordFirstAttempt(ctx, root, id, item, 'pass');
                }
                root._grammar.attempts[id] = 0;
                paintItem(ctx, root, item, 'correct');
                ok += 1;
                return;
            }

            const nextAttempts = attempts + 1;
            root._grammar.attempts[id] = nextAttempts;
            if (nextAttempts === 1) {
                recordFirstAttempt(ctx, root, id, item, 'miss');
                paintItem(ctx, root, item, 'retry');
            } else {
                paintItem(ctx, root, item, 'reveal');
            }
        });
        if (ok === (ctx.ws.grammar.items || []).length) ctx.markComplete(ctx.stepIndex);
        ctx.save();
    }
})();
