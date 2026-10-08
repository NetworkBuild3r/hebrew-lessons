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
                line: U.stripVowels(i.sentence),
                audio: i.sentenceAudio || ''
            }))).slice(0, count);
        }
        const plain = U.storyPlain(ctx.ws.reading?.paragraphs || []);
        const audio = ctx.ws.reading?.audio || [];
        return U.shuffle(plain.map((line, i) => ({
            line: U.stripVowels(line),
            audio: audio[i] || ''
        }))).slice(0, count);
    }

    function tokens(line) {
        return U.stripVowels(line).split(/\s+/).filter(Boolean);
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
            <div class="build-slot" id="build-slot"></div>
            <div class="build-tiles" id="build-tiles">
                ${words.map(w => `<button type="button" class="bank-word" data-action="build-tile" data-word="${ctx.esc(w)}">${ctx.esc(w)}</button>`).join('')}
            </div>
            <div class="actions">
                <button type="button" class="btn btn-ghost" data-action="build-clear">${ctx.bi('נקה', 'Clear')}</button>
                <button type="button" class="btn btn-ghost" data-action="build-check">${ctx.bi('בדיקה', 'Check')}</button>
                <button type="button" class="btn btn-amber" data-action="speak-built"><i class="fa-solid fa-volume-high"></i></button>
            </div>
            <div id="build-feedback" class="match-feedback hidden"></div>`;
        paintBuilt(ctx, root);
        root.querySelector('[data-action="speak-built"]')?.addEventListener('click', () => {
            const built = root._build.built.join(' ');
            const expected = tokens(line).join(' ');
            const full = !built || U.normalizeHebrew(built) === U.normalizeHebrew(expected);
            if (full && entry.audio) ctx.Speech.play(line, entry.audio);
            else ctx.Speech.speak(built || line);
        });
    }

    function paintBuilt(ctx, root) {
        const slot = root.querySelector('#build-slot');
        if (slot) slot.innerHTML = root._build.built.map(w => `<span class="pill">${ctx.esc(w)}</span>`).join(' ') || '<span class="fine">…</span>';
    }

    function check(ctx, root) {
        const line = root._build.pool[root._build.index].line;
        const expected = tokens(line).join(' ');
        const got = root._build.built.join(' ');
        const fb = root.querySelector('#build-feedback');
        const good = U.normalizeHebrew(got) === U.normalizeHebrew(expected);
        if (fb) {
            fb.classList.remove('hidden');
            fb.textContent = good ? 'נכון! (Correct!)' : 'נסו שוב. (Try again.)';
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
