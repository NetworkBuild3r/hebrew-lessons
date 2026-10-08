(function () {
    const P = window.HebrewPlayer;

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
                        <div class="card">
                            <h4>${ctx.esc(rule.title || '')}</h4>
                            <p>${ctx.esc(rule.english || '')}</p>
                            ${rule.example ? `<p class="example">${window.HebrewUtils.renderParts(Array.isArray(rule.example) ? rule.example : [rule.example])}</p>` : ''}
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
            root._grammar = { answers: Object.assign({}, state.answers || {}) };
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'pick-grammar') {
                const id = action.dataset.id;
                const index = Number(action.dataset.index);
                root._grammar.answers[id] = index;
                const wrap = root.querySelector('[data-grammar="' + id + '"]');
                wrap?.querySelectorAll('.choice-card').forEach(btn => {
                    btn.classList.toggle('selected', Number(btn.dataset.index) === index);
                });
                ctx.save();
            }
            if (action.dataset.action === 'check') check(ctx, root);
        },
        collect(ctx, root) {
            return { answers: root._grammar ? root._grammar.answers : {} };
        },
        actionBar(ctx) {
            return `
                <button type="button" class="btn btn-ghost" data-action="check">${ctx.bi('בדיקה', 'Check')}</button>
                <button type="button" class="btn btn-primary" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function check(ctx, root) {
        let ok = 0;
        (ctx.ws.grammar.items || []).forEach(item => {
            const id = ctx.sid(item.id);
            const picked = root._grammar.answers[id];
            const result = root.querySelector('[data-result="' + id + '"]');
            const wrap = root.querySelector('[data-grammar="' + id + '"]');
            wrap?.querySelectorAll('.choice-card').forEach(btn => {
                const idx = Number(btn.dataset.index);
                btn.classList.toggle('correct', idx === item.answer);
                btn.classList.toggle('wrong', picked === idx && idx !== item.answer);
            });
            if (result) {
                result.classList.remove('hidden');
                const good = picked === item.answer;
                if (good) ok += 1;
                result.innerHTML = good
                    ? ctx.bi('נכון', 'Correct') + (item.pointed ? ' ' + ctx.speakButton(item.pointed, item.audio) : '')
                    : ctx.esc(item.why || 'לא מדויק.') + (item.pointed ? ' ' + ctx.speakButton(item.pointed, item.audio) : '');
            }
        });
        if (ok === (ctx.ws.grammar.items || []).length) ctx.markComplete(ctx.stepIndex);
        ctx.save();
    }
})();
