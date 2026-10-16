/**
 * Vocab cards: round 1 recognition; recall rounds alternate he→en / en→he.
 * Authored-by-spec: INIT-001/SPEC-005
 * Provenance: INIT-007/SPEC-004 — 48px pointed word cluster (ADR-0010).
 */
(function (root) {
    const P = root.HebrewPlayer;
    const U = root.HebrewUtils;
    const MEDIA_ID = /^[a-z0-9][a-z0-9-]{0,63}$/;

    /** Resolve recall direction for a card index. force: 'auto' | 'he_en' | 'en_he'. */
    function directionForCard(cardIndex, force) {
        if (force === 'he_en' || force === 'en_he') return force;
        return (Number(cardIndex) % 2 === 0) ? 'he_en' : 'en_he';
    }

    function forceDir(ctx) {
        const st = (ctx.activityState && ctx.activityState.words) || {};
        return st.forceDir === 'he_en' || st.forceDir === 'en_he' ? st.forceDir : 'auto';
    }

    if (!P || typeof P.register !== 'function') {
        root.HebrewWordsActivity = { directionForCard };
        if (typeof module !== 'undefined' && module.exports) {
            module.exports = { directionForCard };
        }
        return;
    }

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
            const recall = ctx.recallMode();
            const force = forceDir(ctx);
            const dirBar = recall ? directionBar(ctx, force) : '';
            return `
                <div class="stack">
                    <div class="banner banner-blue">
                        <h3>${ctx.esc(ctx.ws.vocab?.heading || 'אוצר מילים')}</h3>
                        <p>${ctx.esc(note)}</p>
                        <div class="pill">${items.length} ${ctx.bi('מילות מפתח', 'key words')}</div>
                    </div>
                    ${dirBar}
                    <div class="card-grid" id="vocab-cards">${items.map((item, index) => card(ctx, item, index, force)).join('')}</div>
                </div>`;
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'set-dir') {
                const next = action.dataset.dir === 'he_en' || action.dataset.dir === 'en_he'
                    ? action.dataset.dir
                    : 'auto';
                ctx.activityState.words = Object.assign({}, ctx.activityState.words || {}, { forceDir: next });
                ctx.save();
                ctx.requestRender();
                return;
            }
            if (action.dataset.action === 'toggle-reveal') {
                const id = action.dataset.id;
                const box = root.querySelector('[data-reveal="' + id + '"]');
                box?.classList.toggle('hidden');
                // The "say it first" slot only fills the card while the answer is hidden.
                root.querySelector('[data-slot="' + id + '"]')?.classList.toggle('hidden', !!box && !box.classList.contains('hidden'));
                if (ctx.recallMode() && box && !box.classList.contains('hidden')) {
                    const marks = root.querySelector('[data-marks="' + id + '"]');
                    marks?.classList.remove('hidden');
                    if (action.dataset.direction === 'en_he' && ctx.Speech && typeof ctx.Speech.play === 'function') {
                        try {
                            ctx.Speech.play(action.dataset.word || '', action.dataset.audio || null);
                        } catch (err) { /* fail-soft */ }
                    }
                }
            }
            if (action.dataset.action === 'mark-word') {
                const id = action.dataset.id;
                const value = action.dataset.mark;
                const direction = action.dataset.direction === 'en_he' ? 'en_he' : 'he_en';
                ctx.wordMarks = Object.assign({}, ctx.wordMarks, { [id]: value });
                root.querySelectorAll('[data-marks="' + id + '"] .btn-mark').forEach(btn => {
                    btn.classList.toggle('active', btn.dataset.mark === value);
                });
                if (ctx.review && typeof ctx.review.record === 'function') {
                    const item = (ctx.ws.vocab?.items || []).find(i => ctx.sid(i.id) === id);
                    const signal = value === 'knew' ? 'pass' : value === 'missed' ? 'miss' : null;
                    if (item && item.word && signal) {
                        try {
                            ctx.review.record(item.word, direction, signal, { lesson: ctx.ws.id });
                        } catch (err) { /* fail-soft */ }
                    }
                }
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

    function directionBar(ctx, force) {
        const btn = (dir, he, en) => {
            const active = force === dir ? ' active' : '';
            return `<button type="button" class="btn btn-ghost btn-sm${active}" data-action="set-dir" data-dir="${dir}">${ctx.bi(he, en)}</button>`;
        };
        return `
            <div class="row-tools dir-switch" role="group" aria-label="${ctx.esc('כיוון שליפה')}">
                ${btn('auto', 'לסירוגין', 'Alternate')}
                ${btn('he_en', 'אנגלית ← עברית', 'Hebrew → English')}
                ${btn('en_he', 'עברית ← אנגלית', 'English → Hebrew')}
            </div>`;
    }

    function markRow(ctx, id, mark, direction) {
        return `
            <div class="mark-row hidden" data-marks="${id}">
                <button type="button" class="btn btn-mark${mark === 'knew' ? ' active' : ''}" data-action="mark-word" data-id="${id}" data-mark="knew" data-direction="${direction}">${ctx.bi('ידעתי', 'Knew it')}</button>
                <button type="button" class="btn btn-mark missed${mark === 'missed' ? ' active' : ''}" data-action="mark-word" data-id="${id}" data-mark="missed" data-direction="${direction}">${ctx.bi('פספסתי', 'Missed it')}</button>
            </div>`;
    }

    /** Optional contextual picture — mediaId slug + non-empty alt only (ADR-0003). */
    function vocabImage(ctx, item) {
        const image = item && item.image;
        if (!image || typeof image !== 'object' || Array.isArray(image)) return '';
        const alt = typeof image.alt === 'string' ? image.alt.trim() : '';
        if (!alt) return '';
        const id = typeof image.mediaId === 'string' ? image.mediaId.trim() : '';
        if (!MEDIA_ID.test(id)) return '';
        return `<figure class="vocab-image"><img src="/api/media/${ctx.esc(id)}" alt="${ctx.esc(alt)}" loading="lazy"></figure>`;
    }

    function posPill(ctx, item) {
        if (!item.pos) return '';
        return `<span class="vocab-pill pos">${ctx.esc(item.pos)}</span>`;
    }

    function trapPill(ctx, item) {
        if (!item.trap) return '';
        return `<span class="vocab-pill trap">${ctx.esc(item.trap)}</span>`;
    }

    function pills(ctx, item) {
        const pos = posPill(ctx, item);
        const trap = trapPill(ctx, item);
        if (!pos && !trap) return '';
        return `<div class="vocab-pills">${pos}${trap}</div>`;
    }

    /** Live pointed word — clickable to play (ADR-0010 / aud-2). */
    function heWordBtn(ctx, item, displayWord) {
        const clip = item.audio || '';
        return `<button type="button" class="he-word" data-vowels="${ctx.esc(item.word)}" data-speak="${ctx.esc(item.word)}" data-audio="${ctx.esc(clip)}" title="${ctx.esc('השמעה (Play word)')}">${ctx.esc(displayWord)}</button>`;
    }

    function card(ctx, item, index, force) {
        const id = ctx.sid(item.id);
        const recall = ctx.recallMode();
        const mark = ctx.wordMarks[id];
        const review = ctx.missedLast.map(String).includes(id);
        const displayWord = ctx.vowelsOn ? item.word : U.stripVowels(item.word);
        const direction = recall ? directionForCard(index, force) : null;
        const img = vocabImage(ctx, item);

        if (!recall) {
            return `
            <article class="vocab-card card${review ? ' review-first' : ''}">
                ${review ? `<span class="badge">${ctx.bi('לחזור קודם', 'Review first')}</span>` : ''}
                <div class="vocab-cluster">
                    ${img}
                    <div class="vocab-head">
                        <div class="vocab-main">
                            ${heWordBtn(ctx, item, displayWord)}
                            <p class="meaning">${ctx.esc(item.meaning || '')}</p>
                            ${pills(ctx, item)}
                        </div>
                        ${ctx.speakButton(item.word, item.audio, 'Word')}
                    </div>
                </div>
                <button type="button" class="linkish" data-action="toggle-reveal" data-id="${id}">${ctx.bi('איך אומרים את זה?', 'Show pronunciation')}</button>
                <div class="reveal hidden" data-reveal="${id}">
                    <p class="sound" dir="ltr">${ctx.esc(item.sound || '')}${item.translit ? ' · ' + ctx.esc(item.translit) : ''}</p>
                    ${item.infinitive ? `<p class="fine">${ctx.esc(item.infinitive)}</p>` : ''}
                </div>
                ${exampleBlock(ctx, item, id)}
            </article>`;
        }

        if (direction === 'en_he') {
            return `
            <article class="vocab-card card${review ? ' review-first' : ''}" data-direction="en_he">
                ${review ? `<span class="badge">${ctx.bi('לחזור קודם', 'Review first')}</span>` : ''}
                <div class="vocab-cluster">
                    ${img}
                    <div class="vocab-head">
                        <div class="vocab-main">
                            <p class="meaning" dir="ltr">${ctx.esc(item.meaning || '')}</p>
                            ${pills(ctx, item)}
                        </div>
                    </div>
                </div>
                <div class="recall-slot" data-slot="${id}" aria-hidden="true">${ctx.bi('נסו להגיד אותה בעברית', 'Say it in Hebrew first')}</div>
                <button type="button" class="linkish recall-reveal" data-action="toggle-reveal" data-id="${id}" data-direction="en_he" data-word="${ctx.esc(item.word)}" data-audio="${ctx.esc(item.audio || '')}">${ctx.bi('איך כותבים בעברית?', 'Show Hebrew')}</button>
                <div class="reveal hidden" data-reveal="${id}">
                    <div class="vocab-head">
                        <div class="vocab-main">
                            ${heWordBtn(ctx, item, displayWord)}
                        </div>
                        ${ctx.speakButton(item.word, item.audio, 'Word')}
                    </div>
                    <p class="sound hidden-until" dir="ltr">${ctx.esc(item.sound || '')}${item.translit ? ' · ' + ctx.esc(item.translit) : ''}</p>
                    ${exampleBlock(ctx, item, id)}
                </div>
                ${markRow(ctx, id, mark, 'en_he')}
            </article>`;
        }

        return `
            <article class="vocab-card card${review ? ' review-first' : ''}" data-direction="he_en">
                ${review ? `<span class="badge">${ctx.bi('לחזור קודם', 'Review first')}</span>` : ''}
                <div class="vocab-cluster">
                    ${img}
                    <div class="vocab-head">
                        <div class="vocab-main">
                            ${heWordBtn(ctx, item, displayWord)}
                            ${pills(ctx, item)}
                        </div>
                        ${ctx.speakButton(item.word, item.audio, 'Word')}
                    </div>
                </div>
                <button type="button" class="linkish" data-action="toggle-reveal" data-id="${id}" data-direction="he_en">${ctx.bi('איך אומרים את זה?', 'Show pronunciation')}</button>
                <div class="reveal hidden" data-reveal="${id}">
                    <p class="meaning">${ctx.esc(item.meaning || '')}</p>
                    <p class="sound hidden-until" dir="ltr">${ctx.esc(item.sound || '')}${item.translit ? ' · ' + ctx.esc(item.translit) : ''}</p>
                </div>
                ${markRow(ctx, id, mark, 'he_en')}
                ${exampleBlock(ctx, item, id)}
            </article>`;
    }

    function exampleBlock(ctx, item, id) {
        if (!item.sentence) return '';
        return `
            <div class="example">
                <p data-vowels="${ctx.esc(item.sentence)}">${ctx.esc(ctx.vowelsOn ? item.sentence : U.stripVowels(item.sentence))}</p>
                <div class="row-tools">
                    ${ctx.speakButton(item.sentence, item.sentenceAudio, 'Sentence')}
                    ${ctx.recorderHtml('vocab-' + id)}
                </div>
            </div>`;
    }

    root.HebrewWordsActivity = { directionForCard };

    if (typeof module !== 'undefined' && module.exports) {
        module.exports = { directionForCard };
    }
})(typeof window !== 'undefined' ? window : globalThis);
