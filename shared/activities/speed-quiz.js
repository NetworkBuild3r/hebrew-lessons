(function () {
    const P = window.HebrewPlayer;
    const U = window.HebrewUtils;

    P.register({
        type: 'speed-quiz',
        labelHe: 'חידון מהיר',
        labelEn: 'Speed quiz',
        icon: 'fa-bolt',
        tone: 'tone-green',
        available(ws) { return (ws.vocab?.items || []).filter(i => i.sentence).length > 0; },
        defaults(level, levelDefaults) {
            return { seconds: levelDefaults.quizSeconds || 60, choices: levelDefaults.choices || 3 };
        },
        render(ctx, config, state) {
            return `<div class="card stack" id="speed-quiz-root">
                <div class="quiz-top">
                    <span class="pill" id="quiz-score">0</span>
                    <span class="quiz-clock-wrap">
                        <span class="retell-clock quiz-clock" id="quiz-clock">${config.seconds}s</span>
                    </span>
                </div>
                <div class="quiz-timer" aria-hidden="true">
                    <div class="quiz-timer-fill" id="quiz-timer-fill" style="width:100%"></div>
                </div>
                <div id="quiz-body"></div>
                <div id="quiz-done" class="hidden">
                    <h3>${ctx.bi('סיום', 'Done')}</h3>
                    <p id="quiz-final"></p>
                </div>
            </div>`;
        },
        mount(ctx, root, config, state) {
            const items = (ctx.ws.vocab.items || []).filter(i => i.sentence);
            const queue = U.shuffle(items.map(i => ctx.sid(i.id)));
            const missed = [];
            root._quiz = {
                queue,
                missed,
                index: 0,
                score: state.score || 0,
                left: config.seconds || 60,
                total: config.seconds || 60,
                choices: config.choices || 3,
                timer: null,
                phase: 'main',
                answered: false
            };
            startClock(ctx, root);
            showItem(ctx, root);
        },
        unmount(ctx, root) {
            if (root._quiz && root._quiz.timer) clearInterval(root._quiz.timer);
        },
        onClick(ctx, root, event, action) {
            if (!action || action.dataset.action !== 'quiz-pick') return;
            if (root._quiz.answered) return;
            root._quiz.answered = true;
            const id = action.dataset.id;
            const pick = action.dataset.word;
            const item = ctx.ws.vocab.items.find(i => ctx.sid(i.id) === id);
            const good = U.normalizeHebrew(pick) === U.normalizeHebrew(U.stripVowels(item.word));
            action.classList.add(good ? 'correct' : 'wrong');
            action.classList.add('selected');
            root.querySelectorAll('.choice-card').forEach(btn => {
                if (btn !== action) btn.classList.add('dimmed');
                if (!good && U.normalizeHebrew(btn.dataset.word) === U.normalizeHebrew(U.stripVowels(item.word))) {
                    btn.classList.add('correct');
                }
            });
            const Feedback = window.HebrewFeedback;
            if (Feedback && typeof Feedback.chime === 'function') {
                Feedback.chime(good ? 'correct' : 'miss');
            }
            if (good) root._quiz.score += 1;
            else root._quiz.missed.push(id);
            root.querySelector('#quiz-score').textContent = String(root._quiz.score);
            ctx.save();
            setTimeout(() => {
                root._quiz.index += 1;
                root._quiz.answered = false;
                showItem(ctx, root);
            }, 450);
        },
        collect(ctx, root) {
            return { score: root._quiz ? root._quiz.score : 0 };
        },
        actionBar(ctx) {
            return `<button type="button" class="btn btn-primary btn-wide" data-action="continue">${ctx.bi('המשך', 'Continue')}</button>`;
        }
    });

    function paintTimer(root) {
        const clock = root.querySelector('#quiz-clock');
        const fill = root.querySelector('#quiz-timer-fill');
        const left = Math.max(0, root._quiz.left);
        const total = Math.max(1, root._quiz.total || 1);
        if (clock) clock.textContent = left + 's';
        if (fill) fill.style.width = Math.max(0, Math.min(100, (left / total) * 100)) + '%';
        if (clock) {
            clock.classList.toggle('quiz-clock-low', left <= 10 && left > 0);
        }
    }

    function startClock(ctx, root) {
        paintTimer(root);
        root._quiz.timer = setInterval(() => {
            root._quiz.left -= 1;
            paintTimer(root);
            if (root._quiz.left <= 0) {
                clearInterval(root._quiz.timer);
                finish(ctx, root);
            }
        }, 1000);
    }

    function currentList(root) {
        if (root._quiz.phase === 'main') return root._quiz.queue;
        return root._quiz.missed;
    }

    function showItem(ctx, root) {
        const list = currentList(root);
        if (root._quiz.index >= list.length) {
            if (root._quiz.phase === 'main' && root._quiz.missed.length) {
                root._quiz.phase = 'missed';
                root._quiz.index = 0;
                showItem(ctx, root);
                return;
            }
            finish(ctx, root);
            return;
        }
        const id = list[root._quiz.index];
        const item = ctx.ws.vocab.items.find(i => ctx.sid(i.id) === id);
        if (!item) {
            root._quiz.index += 1;
            showItem(ctx, root);
            return;
        }
        const blank = U.stripVowels(item.word);
        const sentence = U.stripVowels(item.sentence).replace(blank, '____');
        const options = U.shuffle(
            [item.word].concat(
                U.shuffle(ctx.ws.vocab.items.filter(i => ctx.sid(i.id) !== id))
                    .slice(0, Math.max(0, (root._quiz.choices || 3) - 1))
                    .map(i => i.word)
            )
        );
        const body = root.querySelector('#quiz-body');
        body.innerHTML = `
            <p class="quiz-prompt" dir="rtl">${ctx.esc(sentence)}</p>
            <p class="english" dir="ltr">${ctx.esc(item.meaning || '')}</p>
            <div class="choices">
                ${options.map(word => `<button type="button" class="choice-card" data-action="quiz-pick" data-id="${id}" data-word="${ctx.esc(U.stripVowels(word))}">${ctx.esc(word)}</button>`).join('')}
            </div>
            ${root._quiz.phase === 'missed' ? `<p class="fine">${ctx.bi('חזרה על מה שפספסתם', 'Retry missed items')}</p>` : ''}`;
        root.querySelector('#quiz-done')?.classList.add('hidden');
        body.classList.remove('hidden');
    }

    function finish(ctx, root) {
        if (root._quiz.timer) clearInterval(root._quiz.timer);
        root.querySelector('#quiz-body')?.classList.add('hidden');
        const done = root.querySelector('#quiz-done');
        const final = root.querySelector('#quiz-final');
        if (done) done.classList.remove('hidden');
        if (final) final.textContent = 'ניקוד: ' + root._quiz.score + ' (Score: ' + root._quiz.score + ')';
        ctx.markComplete(ctx.stepIndex);
        ctx.save();
    }
})();
