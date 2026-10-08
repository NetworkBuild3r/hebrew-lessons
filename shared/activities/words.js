(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    P.register({
        type: 'words',
        labelHe: 'מילים',
        labelEn: 'Words',
        icon: 'fa-book-bookmark',
        tone: 'tone-blue',
        available(ws) { return (ws.vocab?.items || []).length > 0; },
        render(ctx) {
            const items = ordered(ctx);
            const note = ctx.ws.vocab?.note || 'קראו את המילה בעברית קודם. פתחו את ההגייה רק אם נתקעתם.';
            return `
                <div class="stack">
                    <div class="banner banner-blue">
                        <h3>${ctx.esc(ctx.ws.vocab?.heading || 'אוצר מילים')}</h3>
                        <p>${ctx.esc(note)}</p>
                        <div class="pill">${items.length} ${ctx.bi('מילות מפתח', 'key words')}</div>
                    </div>
                    <div class="card-grid" id="vocab-cards">${items.map(item => card(ctx, item)).join('')}</div>
                </div>`;
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'toggle-reveal') {
                const id = action.dataset.id;
                const box = root.querySelector('[data-reveal="' + id + '"]');
                box?.classList.toggle('hidden');
                if (ctx.recallMode() && box && !box.classList.contains('hidden')) {
                    const marks = root.querySelector('[data-marks="' + id + '"]');
                    marks?.classList.remove('hidden');
                }
            }
            if (action.dataset.action === 'mark-word') {
                const id = action.dataset.id;
                const value = action.dataset.mark;
                ctx.wordMarks = Object.assign({}, ctx.wordMarks, { [id]: value });
                root.querySelectorAll('[data-marks="' + id + '"] .btn-mark').forEach(btn => {
                    btn.classList.toggle('active', btn.dataset.mark === value);
                });
                ctx.save();
            }
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function ordered(ctx) {
        const items = ctx.ws.vocab.items.slice();
        if (!ctx.recallMode() || !ctx.missedLast.length) return items;
        const missed = new Set(ctx.missedLast.map(String));
        return items.filter(i => missed.has(String(i.id))).concat(items.filter(i => !missed.has(String(i.id))));
    }

    function card(ctx, item) {
        const id = ctx.sid(item.id);
        const recall = ctx.recallMode();
        const mark = ctx.wordMarks[id];
        const review = ctx.missedLast.map(String).includes(id);
        return `
            <article class="vocab-card card${review ? ' review-first' : ''}">
                ${review ? `<span class="badge">${ctx.bi('לחזור קודם', 'Review first')}</span>` : ''}
                <div class="vocab-head">
                    <div>
                        <div class="he-word" data-vowels="${ctx.esc(item.word)}">${ctx.esc(ctx.vowelsOn ? item.word : U.stripVowels(item.word))}</div>
                        <div class="pos">${ctx.esc(item.pos || '')}</div>
                    </div>
                    ${ctx.speakButton(item.word, item.audio, 'Word')}
                </div>
                ${recall ? `
                    <button type="button" class="linkish" data-action="toggle-reveal" data-id="${id}">${ctx.bi('איך אומרים את זה?', 'Show pronunciation')}</button>
                    <div class="reveal hidden" data-reveal="${id}">
                        <p class="meaning">${ctx.esc(item.meaning || '')}</p>
                        ${item.trap ? `<p class="trap">${ctx.esc(item.trap)}</p>` : ''}
                        <p class="sound hidden-until" dir="ltr">${ctx.esc(item.sound || '')}${item.translit ? ' · ' + ctx.esc(item.translit) : ''}</p>
                    </div>
                    <div class="mark-row hidden" data-marks="${id}">
                        <button type="button" class="btn btn-mark${mark === 'knew' ? ' active' : ''}" data-action="mark-word" data-id="${id}" data-mark="knew">${ctx.bi('ידעתי', 'Knew it')}</button>
                        <button type="button" class="btn btn-mark missed${mark === 'missed' ? ' active' : ''}" data-action="mark-word" data-id="${id}" data-mark="missed">${ctx.bi('פספסתי', 'Missed it')}</button>
                    </div>
                ` : `
                    <p class="meaning">${ctx.esc(item.meaning || '')}</p>
                    ${item.trap ? `<p class="trap">${ctx.esc(item.trap)}</p>` : ''}
                    <button type="button" class="linkish" data-action="toggle-reveal" data-id="${id}">${ctx.bi('איך אומרים את זה?', 'Show pronunciation')}</button>
                    <div class="reveal hidden" data-reveal="${id}">
                        <p class="sound" dir="ltr">${ctx.esc(item.sound || '')}${item.translit ? ' · ' + ctx.esc(item.translit) : ''}</p>
                        ${item.infinitive ? `<p class="fine">${ctx.esc(item.infinitive)}</p>` : ''}
                    </div>
                `}
                ${item.sentence ? `
                    <div class="example">
                        <p data-vowels="${ctx.esc(item.sentence)}">${ctx.esc(ctx.vowelsOn ? item.sentence : U.stripVowels(item.sentence))}</p>
                        <div class="row-tools">
                            ${ctx.speakButton(item.sentence, item.sentenceAudio, 'Sentence')}
                            ${ctx.recorderHtml('vocab-' + id)}
                        </div>
                    </div>
                ` : ''}
            </article>`;
    }
})();
