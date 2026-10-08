(function () {
    const P = window.HebrewPlayer;

    P.register({
        type: 'ask-friend',
        labelHe: 'שאלו חבר',
        labelEn: 'Ask a Friend',
        icon: 'fa-comments',
        tone: 'tone-rose',
        available(ws) { return (ws.askFriend?.questions || []).length > 0; },
        render(ctx, config, state) {
            const questions = ctx.ws.askFriend.questions;
            const answers = Object.fromEntries((state.answers || []).map(a => [String(a.id), a.val]));
            return `
                <div class="stack">
                    <div class="banner banner-rose">
                        <h3>${ctx.bi('שאלו חבר', 'Ask a Friend')}</h3>
                        <p class="fine">${ctx.bi('שאלו בקול, כתבו את התשובה, והקליטו את עצמכם', 'Ask aloud, write the answer, and record yourself')}</p>
                    </div>
                    <div class="friend-grid">
                        ${questions.map(q => {
                            const id = ctx.sid(q.id);
                            return `<div class="card stack">
                                <p><span class="q-label">${id}</span> ${ctx.esc(q.question)}</p>
                                ${q.youFeminine ? `<p class="fine">${ctx.bi('לנקבה', 'to a woman')}: ${ctx.esc(q.youFeminine)}</p>` : ''}
                                <div class="row-tools">
                                    ${ctx.speakButton(q.pointed || q.question, q.audio)}
                                    ${ctx.recorderHtml('friend-' + id)}
                                </div>
                                <textarea id="friend-ans-${id}" rows="3" placeholder="תשובה...">${ctx.esc(answers[id] || '')}</textarea>
                            </div>`;
                        }).join('')}
                    </div>
                </div>`;
        },
        collect(ctx, root) {
            return {
                answers: (ctx.ws.askFriend.questions || []).map(q => ({
                    id: q.id,
                    val: root.querySelector('#friend-ans-' + ctx.sid(q.id))?.value || ''
                }))
            };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });
})();
