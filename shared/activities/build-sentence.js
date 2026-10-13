(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    P.register({
        type: 'build-sentence',
        labelHe: 'בנו משפט',
        labelEn: 'Build a sentence',
        icon: 'fa-cubes',
        tone: 'tone-blue',
        available(ws) {
            return (ws.reading?.paragraphs || []).length > 0 || (ws.vocab?.items || []).some(i => i.sentence);
        },
        defaults() { return { from: 'story', count: 4 }; },
        render(ctx, config, state) {
            return `<div class="card stack" id="build-root">
                <p class="fine">${ctx.bi('סדרו את האריחים למשפט', 'Put the tiles in order to build the sentence')}</p>
                <div id="build-body"></div>
            </div>`;
        },
        mount(ctx, root, config, state) {
            const pool = sentences(ctx, config);
            root._build = {
                pool,
                index: state.index || 0,
                built: (state.built || []).slice(),
                done: state.done || 0
            };
            show(ctx, root);
        },
        onClick(ctx, root, event, action) {
            if (!root._build) return;
            if (action && action.dataset.action === 'build-tile') {
                const word = action.dataset.word;
                root._build.built.push(word);
                action.classList.add('used');
                action.disabled = true;
                paintBuilt(ctx, root);
                const Feedback = window.HebrewFeedback;
                if (Feedback && typeof Feedback.chime === 'function') Feedback.chime('tap');
                return;
            }
            if (action && action.dataset.action === 'build-remove') {
                const index = Number(action.dataset.index);
                if (!Number.isFinite(index) || index < 0 || index >= root._build.built.length) return;
                const word = root._build.built.splice(index, 1)[0];
                restoreTile(root, word);
                paintBuilt(ctx, root);
                return;
            }
            if (action && action.dataset.action === 'build-clear') {
                root._build.built = [];
                show(ctx, root);
                return;
            }
            if (action && action.dataset.action === 'build-check') {
                check(ctx, root);
            }
            if (action && action.dataset.action === 'build-next') {
                root._build.index += 1;
                root._build.built = [];
                show(ctx, root);
            }
        },
        collect(ctx, root) {
            return root._build ? {
                index: root._build.index,
                built: root._build.built,
                done: root._build.done
            } : {};
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function sentences(ctx, config) {
        const count = config.count || 4;
        if (config.from === 'vocab') {
            return U.shuffle((ctx.ws.vocab.items || []).filter(i => i.sentence).map(i => ({
                pointed: i.sentence || '',
                line: U.stripVowels(i.sentence),
                audio: i.sentenceAudio || ''
            }))).slice(0, count);
        }
        const plain = U.storyPlain(ctx.ws.reading?.paragraphs || []);
        const audio = ctx.ws.reading?.audio || [];
        return U.shuffle(plain.map((line, i) => ({
            pointed: line,
            line: U.stripVowels(line),
            audio: audio[i] || ''
        }))).slice(0, count);
    }

    /** Always play the model sentence clip (ElevenLabs), never TTS of tiles in progress. */
    function playTarget(speech, entry) {
        if (!speech || !entry) return;
        const text = entry.pointed || entry.line || '';
        if (entry.audio && typeof speech.play === 'function') {
            speech.play(text, entry.audio);
            return;
        }
        if (text && typeof speech.speak === 'function') speech.speak(text);
    }

    function tokens(line) {
        return U.stripVowels(line).split(/\s+/).filter(Boolean);
    }

    function restoreTile(root, word) {
        const tiles = root.querySelectorAll('#build-tiles .bank-word.used');
        for (let i = 0; i < tiles.length; i += 1) {
            if (tiles[i].dataset.word === word) {
                tiles[i].classList.remove('used');
                tiles[i].disabled = false;
                break;
            }
        }
    }

    function show(ctx, root) {
        const body = root.querySelector('#build-body');
        if (root._build.index >= root._build.pool.length) {
            body.innerHTML = `<p>${ctx.bi('כל המשפטים!', 'All sentences done!')}</p>`;
            ctx.markComplete(ctx.stepIndex);
            return;
        }
        const entry = root._build.pool[root._build.index];
        const line = entry.line;
        const words = U.shuffle(tokens(line));
        body.innerHTML = `
            <p class="pill">${root._build.index + 1} / ${root._build.pool.length}</p>
            <div class="build-slot" id="build-slot" aria-label="${ctx.esc('משפט')}"></div>
            <div class="build-tiles" id="build-tiles">
                ${words.map(w => `<button type="button" class="bank-word" data-action="build-tile" data-word="${ctx.esc(w)}">${ctx.esc(w)}</button>`).join('')}
            </div>
            <div class="actions">
                <button type="button" class="btn btn-ghost" data-action="build-clear">${ctx.bi('נקה', 'Clear')}</button>
                <button type="button" class="btn btn-ghost" data-action="build-check">${ctx.bi('בדיקה', 'Check')}</button>
                <button type="button" class="btn btn-amber" data-action="speak-built" aria-label="${ctx.esc('השמעת המשפט')}" title="${ctx.esc('השמעת המשפט (Hear the sentence)')}">${U.icon('volume-high')}</button>
            </div>
            <div id="build-feedback" class="match-feedback hidden"></div>`;
        paintBuilt(ctx, root);
        root.querySelector('[data-action="speak-built"]')?.addEventListener('click', () => {
            playTarget(ctx.Speech, entry);
        });
    }

    function paintBuilt(ctx, root) {
        const slot = root.querySelector('#build-slot');
        if (!slot) return;
        if (!root._build.built.length) {
            slot.innerHTML = `<span class="build-slot-placeholder fine">${ctx.bi('הקישו על מילה', 'Tap a word')}</span>`;
            return;
        }
        slot.innerHTML = root._build.built.map((w, i) =>
            `<button type="button" class="bank-word build-placed" data-action="build-remove" data-index="${i}" data-word="${ctx.esc(w)}">${ctx.esc(w)}</button>`
        ).join('');
    }

    function check(ctx, root) {
        const entry = root._build.pool[root._build.index];
        const line = entry.line;
        const expected = tokens(line).join(' ');
        const got = root._build.built.join(' ');
        const fb = root.querySelector('#build-feedback');
        const good = U.normalizeHebrew(got) === U.normalizeHebrew(expected);
        const Feedback = window.HebrewFeedback;
        if (fb) {
            if (good && Feedback && typeof Feedback.showCorrect === 'function') {
                Feedback.showCorrect(fb);
            } else if (!good && Feedback && typeof Feedback.showMiss === 'function') {
                Feedback.showMiss(fb, {
                    pointed: entry.pointed || expected,
                    showRetry: false
                });
            } else {
                fb.classList.remove('hidden');
                fb.textContent = good ? 'נכון! (Correct!)' : 'נסו שוב. (Try again.)';
            }
        }
        if (good) {
            root._build.done += 1;
            ctx.save();
            setTimeout(() => {
                root._build.index += 1;
                root._build.built = [];
                show(ctx, root);
            }, 500);
        }
    }
})();
