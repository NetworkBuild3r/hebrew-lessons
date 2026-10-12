/**
 * Type Hebrew from English meaning. Intermediate + advanced. Records en_he.
 * Authored-by-spec: INIT-001/SPEC-005
 * Restyle: INIT-007/SPEC-005
 *
 * Learner support (2026-10-08):
 * - Hint 1 shows the first letter and a blank per remaining letter.
 * - Hint 2 shows the whole word. Typing it after that is copying, so it is recorded as a miss.
 * - A miss keeps the answer on screen (with audio) until the learner taps Next.
 */
(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    const LETTER = /[\u05D0-\u05EA]/;

    P.register({
        type: 'type-it',
        labelHe: 'הקלידו',
        labelEn: 'Type it',
        icon: 'fa-keyboard',
        tone: 'tone-purple',
        available(ws, config) {
            const level = U.levelOf(ws);
            if (level === 'basic' && !(config && config.force)) return false;
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
                score: state.score || 0,
                hintLevel: 0,
                awaitingNext: false
            };
            show(ctx, root);
        },
        onClick(ctx, root, event, action) {
            if (!action) return;
            if (action.dataset.action === 'type-hint') {
                hint(ctx, root);
                return;
            }
            if (action.dataset.action === 'type-next') {
                advance(ctx, root);
                return;
            }
            if (action.dataset.action !== 'type-check') return;
            check(ctx, root);
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

    /** First letter, then one blank per remaining letter. Spaces between words are kept. */
    function firstLetterHint(word) {
        const bare = U.stripVowels(word || '');
        let seenLetter = false;
        return bare.split(/\s+/).filter(Boolean).map(part => {
            return Array.from(part).map(ch => {
                if (!LETTER.test(ch)) return ch;
                if (!seenLetter) {
                    seenLetter = true;
                    return ch;
                }
                return '_';
            }).join(' ');
        }).join('   ');
    }

    function hintText(item, level) {
        if (level >= 2) return U.stripVowels(item.word);
        if (level === 1) return firstLetterHint(item.word);
        return '';
    }

    function hintButtonHtml(ctx, level) {
        if (level >= 2) return '';
        const label = level === 0
            ? ctx.bi('רמז: אות ראשונה', 'Hint: first letter')
            : ctx.bi('הראה את המילה', 'Show the word');
        return `<button type="button" class="btn btn-ghost" data-action="type-hint">${label}</button>`;
    }

    /** First contiguous pointed run that differs from the typed (vowel-stripped) attempt. */
    function mismatchHighlight(pointed, typed) {
        const bareTarget = U.stripVowels(pointed || '');
        const bareTyped = U.stripVowels(typed || '');
        let ti = 0;
        let start = -1;
        let end = -1;
        for (let i = 0; i < pointed.length; i += 1) {
            const ch = pointed[i];
            if (!LETTER.test(ch) && ch !== ' ') continue;
            const tch = bareTarget[ti];
            const uch = bareTyped[ti];
            if (tch !== uch && start < 0) start = i;
            if (tch !== uch) end = i + 1;
            if (tch === uch && start >= 0) break;
            ti += 1;
        }
        if (start < 0) return '';
        return pointed.slice(start, end > start ? end : start + 1);
    }

    function hint(ctx, root) {
        if (root._type.awaitingNext) return;
        const item = current(ctx, root);
        if (!item) return;
        const level = Math.min((root._type.hintLevel || 0) + 1, 2);
        root._type.hintLevel = level;
        const el = root.querySelector('#type-hint');
        if (el) {
            el.classList.remove('hidden');
            el.textContent = hintText(item, level);
        }
        const slot = root.querySelector('#type-hint-slot');
        if (slot) slot.innerHTML = hintButtonHtml(ctx, level);
    }

    function paintMiss(ctx, fb, item, typedRight, typedValue) {
        const word = U.escapeHtml(item.word);
        const speak = typeof ctx.speakButton === 'function' ? ' ' + ctx.speakButton(item.word, item.audio, 'Word') : '';
        const lead = typedRight
            ? 'נכון, אבל עם עזרה — המילה תחזור. (Right, but with help — this word will come back.)'
            : 'התשובה: (Answer:)';
        const Feedback = window.HebrewFeedback;
        if (Feedback && typeof Feedback.showMiss === 'function') {
            Feedback.showMiss(fb, {
                pointed: item.word,
                highlight: typedRight ? '' : mismatchHighlight(item.word, typedValue),
                retryAction: 'type-next',
                retryLabelHe: 'הבא',
                retryLabelEn: 'Next',
                showRetry: true
            });
            const wordEl = fb.querySelector('.correction-word');
            if (wordEl) wordEl.classList.add('type-answer');
            const label = fb.querySelector('.correction-label');
            if (label) label.textContent = typedRight
                ? 'נכון, אבל עם עזרה — המילה תחזור. (Right, but with help — this word will come back.)'
                : lead;
            if (speak) {
                const box = fb.querySelector('.correction');
                if (box) box.insertAdjacentHTML('beforeend', speak);
            }
            return;
        }
        fb.classList.remove('hidden');
        fb.innerHTML = `<span>${U.escapeHtml(lead)}</span> <span class="type-answer" dir="rtl" lang="he">${word}</span>${speak}
                <button type="button" class="btn btn-primary" data-action="type-next">${U.escapeHtml('הבא (Next)')}</button>`;
    }

    function check(ctx, root) {
        if (root._type.awaitingNext) return;
        const item = current(ctx, root);
        const input = root.querySelector('#type-input');
        const fb = root.querySelector('#type-feedback');
        if (!item || !input) return;

        const typedRight = U.normalizeHebrew(input.value) === U.normalizeHebrew(item.word);
        // The whole word was on screen, so typing it is copying, not recalling.
        const helped = (root._type.hintLevel || 0) >= 2;
        const good = typedRight && !helped;

        if (good) root._type.score += 1;
        if (ctx.review && typeof ctx.review.record === 'function' && item.word) {
            try {
                ctx.review.record(item.word, 'en_he', good ? 'pass' : 'miss', { lesson: ctx.ws.id });
            } catch (err) { /* fail-soft */ }
        }
        ctx.save();

        if (good) {
            const Feedback = window.HebrewFeedback;
            if (fb) {
                if (Feedback && typeof Feedback.showCorrect === 'function') {
                    Feedback.showCorrect(fb);
                } else {
                    fb.classList.remove('hidden');
                    fb.textContent = 'נכון! (Correct!)';
                }
            }
            setTimeout(() => advance(ctx, root), 600);
            return;
        }

        // Miss (or copied from the hint): keep the answer up until the learner is ready.
        root._type.awaitingNext = true;
        input.readOnly = true;
        if (fb) paintMiss(ctx, fb, item, typedRight, input.value);
        if (ctx.Speech && typeof ctx.Speech.play === 'function') {
            ctx.Speech.play(item.word, item.audio || null);
        }
    }

    function advance(ctx, root) {
        root._type.index += 1;
        root._type.hintLevel = 0;
        root._type.awaitingNext = false;
        show(ctx, root);
    }

    function show(ctx, root) {
        const body = root.querySelector('#type-body');
        if (root._type.index >= root._type.queue.length) {
            body.innerHTML = `<p>${ctx.bi('סיום', 'Done')}: ${root._type.score} / ${root._type.queue.length}</p>`;
            ctx.markComplete(ctx.stepIndex);
            return;
        }
        const item = current(ctx, root);
        const level = root._type.hintLevel || 0;
        const hintClass = level ? '' : ' hidden';
        body.innerHTML = `
            <p class="pill">${root._type.index + 1} / ${root._type.queue.length}</p>
            <p class="english" dir="ltr">${ctx.esc(item.meaning || '')}</p>
            <input id="type-input" class="fill-input type-input" dir="rtl" lang="he" autocomplete="off" placeholder="עברית..." inputmode="text">
            <p class="type-kb-hint fine">${ctx.bi('בטלפון: החזיקו את מקש הגלובוס לבחירת עברית', 'On phone: long-press the globe key for Hebrew')}</p>
            <div class="row-tools">
                <button type="button" class="btn btn-ghost" data-action="type-check">${ctx.bi('בדיקה', 'Check')}</button>
                <span id="type-hint-slot">${hintButtonHtml(ctx, level)}</span>
            </div>
            <p id="type-hint" class="fine type-hint${hintClass}" dir="rtl">${ctx.esc(hintText(item, level))}</p>
            <div id="type-feedback" class="match-feedback hidden"></div>`;
        const input = root.querySelector('#type-input');
        if (input) {
            input.addEventListener('keydown', (event) => {
                if (event.key !== 'Enter') return;
                event.preventDefault();
                if (root._type.awaitingNext) advance(ctx, root);
                else check(ctx, root);
            });
            input.focus();
        }
    }
})();
