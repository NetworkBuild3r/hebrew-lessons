(function () {
    const ws = window.WORKSHEET;
    const app = document.getElementById('app');
    if (!app) return;
    if (!ws || !ws.id) {
        app.innerHTML = '<main class="wrap"><div class="card"><h2>Missing worksheet</h2><p>This folder needs a worksheet.js that sets window.WORKSHEET.</p></div></main>';
        return;
    }

    const storageKey = 'hebrew-worksheet:' + ws.id;
    const sections = [
        { id: 'vocab', label: 'Vocabulary', icon: 'fa-book-bookmark', tone: 'tone-blue', enabled: () => vocabItems().length },
        { id: 'reading', label: 'Story', icon: 'fa-file-lines', tone: 'tone-amber', enabled: () => paragraphs().length },
        { id: 'practice', label: 'Vocab Practice', icon: 'fa-layer-group', tone: 'tone-green', enabled: () => vocabItems().length || fillSentences().length },
        { id: 'grammar', label: ws.grammar?.tabLabel || 'Grammar', icon: ws.grammar?.icon || 'fa-puzzle-piece', tone: 'tone-purple', enabled: () => grammarItems().length },
        { id: 'ask-friend', label: 'Ask a Friend', icon: 'fa-comments', tone: 'tone-rose', enabled: () => friendQuestions().length }
    ].filter(section => section.enabled());

    let selectedHebrewId = null;
    let selectedEnglishId = null;
    let matchedPairs = [];
    let activeInputId = null;
    let selectedBankWord = null;
    let suppressBankClick = false;
    let vowelsOn = true;
    let round = 1;
    let completedRounds = 0;
    let roundStartedAt = 0;
    let wordMarks = {};
    let missedLast = [];
    let bankOpen = false;
    let retellTimerId = null;
    let recorderState = null;
    const story = { run: 0, status: 'idle', index: 0, utterance: null };
    const grammarState = {};
    const recordings = {};
    const DAY_MS = 24 * 60 * 60 * 1000;
    const PREFS_KEY = 'hebrew-worksheet-prefs';

    document.title = plainTitle();
    app.innerHTML = renderPage();
    renderVocab();
    renderMatch();
    renderFill();
    renderGrammar();
    renderReading();
    renderFriends();
    bindEvents();
    restoreSaved();
    paintRound();
    validate();

    function vocabItems() { return ws.vocab?.items || []; }
    function fillSentences() { return ws.fill?.sentences || []; }
    function fillBank() { return ws.fill?.bank || []; }
    function grammarItems() { return ws.grammar?.items || []; }
    function paragraphs() { return ws.reading?.paragraphs || []; }
    function readingQuestions() { return ws.reading?.questions || []; }
    function friendQuestions() { return ws.askFriend?.questions || []; }
    function recallMode() { return round > 1; }

    function paintRound() {
        const count = document.getElementById('round-count');
        const note = document.getElementById('round-note');
        if (count) {
            const done = completedRounds ? ' · סיימתם ' + completedRounds + ' (finished ' + completedRounds + ')' : '';
            count.textContent = 'סיבוב ' + round + ' (Round ' + round + ')' + done;
        }
        if (!note) return;
        note.textContent = recallMode()
            ? 'שליפה מהזיכרון. המשמעות מוסתרת עד שתבקשו אותה, ואחר כך מסמנים אם ידעתם. (Recall round: meanings stay hidden until you ask. Mark each word, and the ones you missed come first next round. Most of the round is the story and speaking.)'
            : 'לומדים. כרבע מהזמן על המילים ועל דפוס אחד, והשאר הסיפור והדיבור. (Learning round: about a quarter of the time on words and one pattern, the rest on the story and speaking.)';
        const due = document.getElementById('round-due');
        if (due) due.textContent = dueText();
        paintBank();
        document.getElementById('retell-card')?.classList.toggle('hidden', !recallMode());
    }

    function dueText() {
        if (!roundStartedAt) return 'פער של יום בין סיבובים עוזר לזכור. (A day between rounds helps you remember.)';
        const due = roundStartedAt + DAY_MS;
        if (Date.now() >= due) return 'עבר יום. אפשר לעשות עוד סיבוב עכשיו. (A day has passed. You can practice again now.)';
        const when = new Date(due).toLocaleString('he-IL', { weekday: 'long', hour: '2-digit', minute: '2-digit' });
        const whenEn = new Date(due).toLocaleString('en-US', { weekday: 'long', hour: 'numeric', minute: '2-digit' });
        return 'עוד סיבוב מומלץ מ' + when + '. (Next round suggested from ' + whenEn + '. A day between rounds helps you remember.)';
    }

    function paintBank() {
        const gate = document.getElementById('bank-gate');
        const body = document.getElementById('bank-body');
        if (!gate || !body) return;
        const closed = recallMode() && !bankOpen;
        gate.classList.toggle('hidden', !closed);
        body.classList.toggle('hidden', closed);
    }

    function vocabCountText() {
        let text = vocabItems().length + ' מילות מפתח (key words)';
        if (recallMode()) {
            const marks = Object.values(wordMarks);
            const knew = marks.filter(mark => mark === 'knew').length;
            const missed = marks.filter(mark => mark === 'missed').length;
            if (knew || missed) text += ' · ידעתי ' + knew + ' (knew) · לא ידעתי ' + missed + ' (missed)';
        }
        return text;
    }

    function orderedVocab() {
        const items = vocabItems();
        if (!missedLast.length) return items;
        return items.filter(item => missedLast.includes(sid(item.id)))
            .concat(items.filter(item => !missedLast.includes(sid(item.id))));
    }

    function markWord(button) {
        const id = button.dataset.markId;
        const value = button.dataset.mark;
        wordMarks[id] = wordMarks[id] === value ? '' : value;
        if (!wordMarks[id]) delete wordMarks[id];
        app.querySelectorAll('[data-mark-id="' + id + '"]').forEach(other => {
            other.classList.toggle('chosen', wordMarks[id] === other.dataset.mark);
        });
        const count = document.getElementById('vocab-count');
        if (count) count.textContent = vocabCountText();
        saveLocalProgress();
    }

    function toggleMeaning(button) {
        const box = document.getElementById(button.dataset.meaning);
        if (!box) return;
        const open = box.classList.contains('hidden');
        box.classList.toggle('hidden', !open);
        button.textContent = open ? 'הסתר משמעות (Hide meaning)' : 'מה המשמעות? (Show meaning)';
    }

    function clearWrittenAnswers() {
        readingQuestions().forEach(question => {
            const el = document.getElementById('reading-ans-' + sid(question.id));
            if (el) el.value = '';
        });
        friendQuestions().forEach(question => {
            const el = document.getElementById('friend-ans-' + sid(question.id));
            if (el) el.value = '';
        });
        const retell = document.getElementById('retell-answer');
        if (retell) retell.value = '';
        updateFriendSummary();
    }

    function startNextRound() {
        const early = roundStartedAt && Date.now() < roundStartedAt + DAY_MS;
        const ok = window.confirm(early
            ? 'עברה פחות מיממה מתחילת הסיבוב. פער של יום עוזר לזכור. עוד סיבוב מוחק את התשובות בדף הזה. להמשיך בכל זאת?\n(Less than a day has passed. A day between rounds helps you remember. Practicing again clears your answers on this page. Continue anyway?)'
            : 'עוד סיבוב מוחק את התשובות בדף הזה, כדי לשלוף אותן מהזיכרון. להמשיך?\n(Practicing again clears your answers on this page so you recall them from memory. Continue?)');
        if (!ok) return;
        silence();
        completedRounds += 1;
        round += 1;
        missedLast = Object.keys(wordMarks).filter(id => wordMarks[id] === 'missed');
        wordMarks = {};
        bankOpen = false;
        roundStartedAt = Date.now();
        window.clearInterval(retellTimerId);
        retellTimerId = null;
        const clock = document.getElementById('retell-clock');
        if (clock) clock.textContent = '';
        document.getElementById('story-text')?.classList.remove('hidden');
        resetMatch();
        resetFill();
        resetGrammar();
        clearWrittenAnswers();
        renderVocab();
        paintRound();
        const startAt = sections.some(section => section.id === 'reading') ? 'reading' : sections[0]?.id;
        if (startAt) switchTab(startAt);
        saveLocalProgress();
    }

    function plainTitle() {
        const title = ws.title || 'Hebrew worksheet';
        const subtitle = ws.subtitle ? ' — ' + ws.subtitle : '';
        return title + subtitle;
    }

    function escapeHtml(str) {
        return String(str ?? '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function renderGlossChips(list) {
        return `<div class="gloss-list">${(list || []).map(item => `
            <span class="gloss-chip"><strong>${escapeHtml(item.he)}</strong> ${escapeHtml(item.en)}</span>
        `).join('')}</div>`;
    }

    function renderParts(parts) {
        const list = Array.isArray(parts) ? parts : [parts];
        const show = text => escapeHtml(vowelsOn ? text : stripVowels(text));
        return list.map(part => {
            if (typeof part === 'string') return show(part);
            if (!part || typeof part !== 'object') return '';
            if (part.reported) return `<span class="reported">${Array.isArray(part.reported) ? renderParts(part.reported) : show(part.reported)}</span>`;
            if (part.direct) return `<span class="direct-quote">${Array.isArray(part.direct) ? renderParts(part.direct) : show(part.direct)}</span>`;
            if (part.strong) return `<strong>${show(part.strong)}</strong>`;
            if (part.word) return `<mark class="new-word">${show(part.word)}</mark>`;
            return '';
        }).join('');
    }

    function partText(part) {
        if (typeof part === 'string') return part;
        if (!part || typeof part !== 'object') return '';
        const value = part.reported || part.direct || part.strong || part.word || '';
        return Array.isArray(value) ? value.map(partText).join('') : value;
    }

    function storyText() {
        return paragraphs().map(paragraph => (Array.isArray(paragraph) ? paragraph : [paragraph]).map(partText).join('')).join(' ');
    }

    function storyHasVowels() {
        return /[\u0591-\u05C7]/.test(storyText());
    }

    function renderStoryBody() {
        const el = document.getElementById('story-text');
        if (el) el.innerHTML = paragraphs().map(paragraph => `<p>${renderParts(paragraph)}</p>`).join('');
        paintCurrentLine();
    }

    function normalizeHebrew(str) {
        return String(str || '')
            .replace(/[\u0591-\u05C7]/g, '')
            .replace(/[.,!?;:"'״׳\-–—()]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
    }

    function stripVowels(str) {
        return String(str || '').replace(/[\u0591-\u05C7]/g, '');
    }

    function toggleReveal(button) {
        const box = document.getElementById(button.dataset.reveal);
        if (!box) return;
        const open = box.classList.contains('hidden');
        box.classList.toggle('hidden', !open);
        button.textContent = open ? 'הסתר הגייה (Hide pronunciation)' : 'איך אומרים את זה? (Show pronunciation)';
    }

    function toggleBox(id) {
        document.getElementById(id)?.classList.toggle('hidden');
    }

    function toggleVowels() {
        vowelsOn = !vowelsOn;
        applyVowels();
        renderStoryBody();
        app.querySelectorAll('[data-action="toggle-vowels"]').forEach(button => {
            button.textContent = vowelsOn ? 'הסתר ניקוד (Hide vowels)' : 'הצג ניקוד (Show vowels)';
        });
    }

    function applyVowels() {
        app.querySelectorAll('[data-pointed]').forEach(el => {
            el.textContent = vowelsOn && el.dataset.pointed ? el.dataset.pointed : (el.dataset.plain || el.textContent);
        });
        app.querySelectorAll('.you-f').forEach(el => el.classList.toggle('hidden', !vowelsOn));
    }

    function shuffle(list) {
        const arr = [...list];
        for (let i = arr.length - 1; i > 0; i--) {
            const j = Math.floor(Math.random() * (i + 1));
            [arr[i], arr[j]] = [arr[j], arr[i]];
        }
        return arr;
    }

    function sid(id) { return String(id); }

    function renderPage() {
        const tabs = sections.map((section, index) => `
            <button type="button" class="tab${index === 0 ? ' active' : ''}" data-tab="${section.id}">
                <i class="fa-solid ${section.icon} ${section.tone}"></i>
                <span>${index + 1}. ${escapeHtml(section.label)}</span>
            </button>
        `).join('');

        const panels = sections.map((section, index) => `
            <section id="panel-${section.id}" class="panel${index === 0 ? ' active' : ''}">
                ${renderPanel(section.id)}
            </section>
        `).join('');

        return `
            <header class="topbar">
                <div class="topbar-inner">
                    <div>
                        <a class="back-link" href="../../index.html"><i class="fa-solid fa-house"></i> חזרה לכל השיעורים (Back to all lessons)</a>
                        <div class="kicker">${escapeHtml(ws.series || 'דרך עברית • Light & Easy Hebrew')}</div>
                        <h1>${escapeHtml(ws.title || 'דף עבודה')}</h1>
                        ${ws.subtitle ? `<p class="subtitle">${escapeHtml(ws.subtitle)}</p>` : ''}
                        <p class="round-bar"><strong id="round-count">סיבוב 1</strong> <span id="round-note"></span></p>
                        <p class="round-bar round-due" id="round-due"></p>
                    </div>
                    <div>
                        <div class="savebar">
                            <button type="button" class="btn btn-again" data-action="again">
                                <i class="fa-solid fa-rotate-right"></i>
                                <span>עוד סיבוב (Practice again)</span>
                            </button>
                        </div>
                        <p class="save-note">התשובות נשמרות אוטומטית בדפדפן הזה (Answers save automatically in this browser)</p>
                    </div>
                </div>
            </header>
            <nav class="tabs"><div class="tabs-inner">${tabs}</div></nav>
            <main class="wrap">${panels}</main>
            <footer class="footer">
                <p>${escapeHtml(ws.school || 'Derech School • דרך עברית — Light & Easy Hebrew')}</p>
                <p>${escapeHtml(ws.footer || ws.title || '')}</p>
            </footer>
        `;
    }

    function renderPanel(id) {
        if (id === 'vocab') return renderVocabPanel();
        if (id === 'practice') return renderPracticePanel();
        if (id === 'grammar') return renderGrammarPanel();
        if (id === 'reading') return renderReadingPanel();
        if (id === 'ask-friend') return renderFriendPanel();
        return '';
    }

    function renderVocabPanel() {
        const note = ws.vocab?.note || 'קראו את המילה בעברית קודם. פתחו את ההגייה רק אם נתקעתם. הקול של המחשב הוא כיוון גס, לא מורה. Read the Hebrew first. Open the pronunciation only when you are stuck. The computer voice is a rough guide, not a teacher.';
        return `
            <div class="stack">
                <div class="banner banner-blue banner-row">
                    <div>
                        <h2>${escapeHtml(ws.vocab?.heading || 'אוצר מילים — Key Vocabulary')}</h2>
                        <p>${escapeHtml(note)}</p>
                    </div>
                    <div class="pill" id="vocab-count"></div>
                </div>
                <div class="card-grid" id="vocab-cards"></div>
            </div>
        `;
    }

    function renderPracticePanel() {
        const blocks = [];
        if (vocabItems().length) {
            blocks.push(`
                <div class="card stack">
                    <div>
                        <span class="part-label">חלק א׳ — Part A</span>
                        <h2>התאימו מילה בעברית לתרגום באנגלית</h2>
                        <p class="fine">Read the Hebrew word, then find the meaning. If a past-tense “I” is hiding, look for תי at the end.</p>
                    </div>
                    <div class="match-grid">
                        <div>
                            <h3 class="col-title">English meaning</h3>
                            <div class="list" id="match-english-list"></div>
                        </div>
                        <div class="he-list">
                            <h3 class="col-title">מילה בעברית</h3>
                            <div class="list" id="match-hebrew-list"></div>
                        </div>
                    </div>
                    <div class="actions">
                        <button type="button" class="btn btn-ghost" data-action="reset-match">איפוס (Reset)</button>
                        <div id="match-feedback" class="match-feedback hidden"></div>
                    </div>
                </div>
            `);
        }
        if (fillSentences().length) {
            blocks.push(`
                <div class="card stack">
                    <div>
                        <span class="part-label">חלק ב׳ — Part B</span>
                        <h2>השלימו את המשפטים מבנק המילים</h2>
                        <p class="fine">${escapeHtml(fillNote())}</p>
                    </div>
                    <div class="bank">
                        <p class="meaning-label">בנק מילים — Word bank</p>
                        <div id="bank-gate" class="stack hidden">
                            <p class="fine">אמרו בקול את המילה החסרה, ונסו לכתוב אותה בלי הבנק. אחר כך פתחו אותו. Say the missing word out loud and try to type it first. Then open the bank.</p>
                            <div><button type="button" class="btn btn-ghost" data-action="open-bank">פתחו את הבנק (Open the word bank)</button></div>
                        </div>
                        <div id="bank-body"><div class="bank-words" id="word-bank"></div></div>
                    </div>
                    <div class="stack" id="fill-sentences"></div>
                    <div class="actions">
                        <button type="button" class="btn btn-green" data-action="check-fill">בדיקת תשובות (Check)</button>
                        <button type="button" class="btn btn-ghost" data-action="reset-fill">איפוס (Reset)</button>
                        <span id="fill-feedback"></span>
                    </div>
                </div>
            `);
        }
        return `<div class="stack-lg">${blocks.join('')}</div>`;
    }

    function fillNote() {
        if (ws.fill?.note) return ws.fill.note;
        const extra = fillBank().length - fillSentences().length;
        const extraText = extra === 1
            ? 'One word is extra.'
            : extra > 1
                ? extra + ' words are extra.'
                : 'Use each word once.';
        return 'There are ' + fillBank().length + ' words and ' + fillSentences().length + ' sentences. ' + extraText + ' Choose a word, then click the blank it belongs in. Or drag it there. A click does not fill a blank by itself. Open “what is this sentence about?” when the words around the blank are new.';
    }

    function renderGrammarPanel() {
        const grammar = ws.grammar || {};
        return `
            <div class="stack">
                <div class="banner banner-purple stack">
                    <div>
                        <h2>${escapeHtml(grammar.heading || grammar.tabLabel || 'Grammar')}</h2>
                        ${grammar.lede ? `<p>${escapeHtml(grammar.lede)}</p>` : ''}
                    </div>
                    ${renderRules(grammar.rules, grammar.reminders)}
                </div>
                <div class="stack" id="grammar-items"></div>
                ${grammarItems().length ? `
                <div class="actions">
                    <button type="button" class="btn btn-purple" data-action="check-grammar">בדיקת תשובות (Check)</button>
                    <button type="button" class="btn btn-ghost" data-action="reset-grammar">איפוס (Reset)</button>
                    <span id="grammar-summary"></span>
                </div>` : ''}
            </div>
        `;
    }

    function renderRules(rules, reminders) {
        const list = rules?.length ? `<ol class="rules">${rules.map(rule => `
            <li class="rule">
                <strong>${escapeHtml(rule.title || '')}</strong>
                ${rule.english ? `<span class="ltr">${escapeHtml(rule.english)}</span>` : ''}
                ${rule.example ? `<span class="example-line">${renderParts(rule.example)}</span>` : ''}
            </li>
        `).join('')}</ol>` : '';
        const notes = reminders?.length ? `
            <div class="reminders">
                <p class="meaning-label">תזכורות קצרות — Quick reminders, not new rules</p>
                <ul>${reminders.map(note => `<li>${renderParts(note)}</li>`).join('')}</ul>
            </div>` : '';
        return list + notes;
    }

    function hasSpeechMarks() {
        return paragraphs().some(paragraph => {
            const parts = Array.isArray(paragraph) ? paragraph : [paragraph];
            return parts.some(part => part && typeof part === 'object' && (part.reported || part.direct));
        });
    }

    function renderReadingPanel() {
        const legend = hasSpeechMarks()
            ? `<p class="legend"><span class="reported">סגול = דיבור עקיף</span> · <span class="direct-quote">אדום במרכאות = דיבור ישיר</span></p>
               <p class="fine">${escapeHtml(ws.reading?.notice || 'Read for the story first. The purple pieces are how Hebrew reports what someone said: ש־ sticks to the next word, and the tense stays. You do not need to produce these yet.')}</p>`
            : '';
        const support = (ws.reading?.support || []).length
            ? `<div class="bank"><p class="meaning-label">מילים מסביב — words around the new ones</p>${renderGlossChips(ws.reading.support)}</div>`
            : '';
        const questions = readingQuestions().length ? `
            <div class="card stack">
                <div>
                    <h2>שאלות הבנה — Comprehension Questions</h2>
                    <p class="fine">${escapeHtml(ws.reading?.questionNote || 'תשובת הדוגמה היא אפשרות אחת. אפשר לענות גם בניסוח אחר. The sample is one possible answer. Other wording can be right too.')}</p>
                </div>
                <div class="stack" id="reading-questions"></div>
            </div>
        ` : '';
        return `
            <div class="stack">
                ${support}
                <div class="story">
                    <div class="story-head">
                        <div>
                            <h2>${escapeHtml(ws.reading?.heading || 'קראו את הטקסט — Read the text')}</h2>
                            ${legend}
                        </div>
                        <div class="story-tools">
                            ${storyHasVowels() ? `<button type="button" class="btn btn-ghost" data-action="toggle-vowels">הסתר ניקוד (Hide vowels)</button>` : ''}
                            <label class="follow-along"><input type="checkbox" id="follow-along"${followAlongPref() ? ' checked' : ''}> הדגשת השורה (Follow along)</label>
                            <button type="button" class="btn btn-amber" id="story-play" data-action="story-play">
                                <i class="fa-solid fa-volume-high"></i> הקראת טקסט (Read aloud)
                            </button>
                            <button type="button" class="btn btn-ghost hidden" id="story-pause" data-action="story-pause">
                                <i class="fa-solid fa-pause"></i> השהיה (Pause)
                            </button>
                            <button type="button" class="btn btn-stop hidden" id="story-stop" data-action="story-stop">
                                <i class="fa-solid fa-stop"></i> עצירה (Stop)
                            </button>
                        </div>
                    </div>
                    <div class="story-body" id="story-text">${paragraphs().map(paragraph => `<p>${renderParts(paragraph)}</p>`).join('')}</div>
                </div>
                ${renderRetell()}
                ${questions}
            </div>
        `;
    }

    function renderRetell() {
        const cues = ws.reading?.retell?.cues || vocabItems().slice(0, 6).map(item => item.word);
        const seconds = Number(ws.reading?.retell?.seconds) || 90;
        const chips = cues.map(cue => `<span class="gloss-chip" data-pointed="${escapeHtml(cue)}" data-plain="${escapeHtml(stripVowels(cue))}">${escapeHtml(cue)}</span>`).join('');
        return `
            <div class="card stack hidden" id="retell-card" data-seconds="${seconds}">
                <div>
                    <span class="part-label">שליפה — Retell</span>
                    <h2>ספרו את הסיפור במילים שלכם</h2>
                    <p class="fine">Hide the story, look at the cue words, and tell it out loud before the clock runs out. Then write what you remember.</p>
                </div>
                <div class="gloss-list">${chips}</div>
                <div class="actions">
                    <button type="button" class="btn btn-ghost" data-action="hide-story">הסתירו את הסיפור (Hide the story)</button>
                    <button type="button" class="btn btn-green" data-action="retell-timer">התחילו שעון (Start the clock, ${seconds}s)</button>
                    <strong id="retell-clock" class="retell-clock"></strong>
                </div>
                <textarea id="retell-answer" rows="4" spellcheck="false" placeholder="אמרו את הסיפור בקול, ואז כתבו מה זכרתם..."></textarea>
            </div>
        `;
    }

    function toggleStoryVisible(button) {
        const body = document.getElementById('story-text');
        if (!body) return;
        const hide = !body.classList.contains('hidden');
        if (hide) stopStory();
        body.classList.toggle('hidden', hide);
        button.textContent = hide ? 'הציגו את הסיפור (Show the story)' : 'הסתירו את הסיפור (Hide the story)';
    }

    function startRetellClock() {
        const card = document.getElementById('retell-card');
        const clock = document.getElementById('retell-clock');
        if (!card || !clock) return;
        window.clearInterval(retellTimerId);
        let left = Number(card.dataset.seconds) || 90;
        const show = () => { clock.textContent = Math.floor(left / 60) + ':' + String(left % 60).padStart(2, '0'); };
        show();
        retellTimerId = window.setInterval(() => {
            left -= 1;
            if (left <= 0) {
                window.clearInterval(retellTimerId);
                retellTimerId = null;
                clock.textContent = 'הזמן נגמר. Time is up.';
                return;
            }
            show();
        }, 1000);
    }

    function renderFriendPanel() {
        const count = friendQuestions().length;
        const intro = ws.askFriend?.intro || 'שאלו חבר או חברה שדוברים עברית את השאלות, וכתבו את התשובה שלהם.';
        const introEn = ws.askFriend?.introEn || 'Say your answer out loud before you write it. A wrong guess is useful. Vowels are on so you can read the question. Hide them when you want the harder version.';
        const vowelToggle = friendQuestions().some(question => question.pointed)
            ? `<button type="button" class="btn btn-ghost" data-action="toggle-vowels">הסתר ניקוד (Hide vowels)</button>`
            : '';
        return `
            <div class="stack">
                <div class="banner banner-rose">
                    <div class="banner-row">
                        <h2>${escapeHtml(ws.askFriend?.heading || 'שאל חבר / חברה — Ask a Friend')}</h2>
                        ${vowelToggle}
                    </div>
                    <p>${escapeHtml(intro)}</p>
                    <p class="fine">${escapeHtml(introEn)}</p>
                </div>
                <div class="friend-grid" id="friend-questions" data-count="${count}"></div>
                <div class="summary">
                    <div class="summary-head">
                        <h3><i class="fa-solid fa-clipboard-list tone-rose"></i> סיכום הראיון שלי (My Interview Summary)</h3>
                        <button type="button" class="btn btn-tiny" data-action="copy-notes">
                            <i class="fa-regular fa-copy"></i> העתקת תשובות (Copy All)
                        </button>
                    </div>
                    <pre id="friend-summary"></pre>
                </div>
            </div>
        `;
    }

    function renderVocab() {
        const container = document.getElementById('vocab-cards');
        const count = document.getElementById('vocab-count');
        if (count) count.textContent = vocabCountText();
        if (!container) return;
        const items = recallMode() ? orderedVocab() : vocabItems();
        container.innerHTML = items.map(item => {
            const id = sid(item.id);
            const reviewFirst = recallMode() && missedLast.includes(id);
            const mark = wordMarks[id] || '';
            return `
            <article class="card vocab-card stack${reviewFirst ? ' review-first' : ''}">
                <div class="banner-row">
                    <div>
                        <h3>${escapeHtml(item.word)}</h3>
                        <p class="meta">${item.pos ? `<span class="pos">${escapeHtml(item.pos)}</span>` : ''}${item.from ? ` <span class="badge badge-old">מילה חוזרת משיעור קודם</span>` : ''}${reviewFirst ? ` <span class="badge badge-review">לחזור קודם</span>` : ''}</p>
                        ${item.infinitive ? `<p class="infinitive">שם הפועל: ${escapeHtml(item.infinitive)}</p>` : ''}
                    </div>
                    ${item.word ? speakButton(item.word, item.audio, 'הגיית המילה. קול המחשב הוא כיוון גס.') : ''}
                </div>
                <div>
                    <p class="meaning-label">Meaning</p>
                    ${recallMode()
                        ? `<button type="button" class="linkish" data-meaning="mean-${id}">מה המשמעות? (Show meaning)</button>
                           <div id="mean-${id}" class="hidden stack">
                               <p>${escapeHtml(item.meaning || '')}</p>
                               <div class="mark-row">
                                   <button type="button" class="btn btn-mark${mark === 'knew' ? ' chosen' : ''}" data-mark="knew" data-mark-id="${escapeHtml(id)}">ידעתי (Knew it)</button>
                                   <button type="button" class="btn btn-mark btn-mark-missed${mark === 'missed' ? ' chosen' : ''}" data-mark="missed" data-mark-id="${escapeHtml(id)}">לא ידעתי (Missed it)</button>
                               </div>
                           </div>`
                        : `<p>${escapeHtml(item.meaning || '')}</p>`}
                </div>
                ${item.trap ? `<p class="trap">${escapeHtml(item.trap)}</p>` : ''}
                ${item.sound || item.translit ? `
                    <div>
                        <button type="button" class="linkish" data-reveal="say-${sid(item.id)}">איך אומרים את זה? (Show pronunciation)</button>
                        <div id="say-${sid(item.id)}" class="example hidden">
                            <p class="sound">${escapeHtml(item.sound || item.translit || '')}</p>
                            ${item.translit && item.sound ? `<p class="fine">${escapeHtml(item.translit)}</p>` : ''}
                        </div>
                    </div>
                ` : ''}
                ${item.sentence ? `<div class="example">
                    <p class="meaning-label">משפט דוגמה</p>
                    <div class="sentence-row">
                        <p>${escapeHtml(item.sentence)}</p>
                        ${speakButton(item.sentence, item.sentenceAudio, 'הקראת המשפט. קול המחשב הוא כיוון גס.')}
                        ${recorderHtml('vocab-' + id)}
                    </div>
                </div>` : ''}
            </article>
        `;
        }).join('');
    }

    function speakButton(text, audio, title) {
        const label = audio ? 'הקלטה של אדם' : title;
        return `<button type="button" class="icon-btn${audio ? ' human' : ''}" data-speak="${escapeHtml(text)}"${audio ? ` data-audio="${escapeHtml(audio)}"` : ''} title="${escapeHtml(label)}"><i class="fa-solid fa-volume-high"></i></button>`;
    }

    function recorderHtml(key) {
        const safe = escapeHtml(String(key).replace(/[^\w-]/g, '_'));
        return `<span class="recorder">
            <button type="button" class="icon-btn rec-btn" data-rec-toggle="${safe}" title="הקליטו את עצמכם. Record yourself."><i class="fa-solid fa-microphone"></i></button>
            <button type="button" class="icon-btn rec-play hidden" data-rec-play="${safe}" title="השמעת ההקלטה שלכם. Hear your recording."><i class="fa-solid fa-play"></i></button>
        </span>`;
    }

    function paintRecorder(key, recording) {
        const button = app.querySelector('[data-rec-toggle="' + key + '"]');
        if (!button) return;
        button.classList.toggle('recording', recording);
        button.innerHTML = recording ? '<i class="fa-solid fa-stop"></i>' : '<i class="fa-solid fa-microphone"></i>';
    }

    async function toggleRecording(key) {
        if (recorderState && recorderState.key === key) {
            recorderState.media.stop();
            return;
        }
        if (recorderState) recorderState.media.stop();
        if (!navigator.mediaDevices || !window.MediaRecorder) {
            alert('הדפדפן הזה לא מקליט. This browser cannot record.');
            return;
        }
        silence();
        let stream;
        try {
            stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        } catch (err) {
            alert('אין גישה למיקרופון. אשרו את המיקרופון בדפדפן ונסו שוב. Allow the microphone and try again.');
            return;
        }
        const media = new MediaRecorder(stream);
        const chunks = [];
        media.addEventListener('dataavailable', event => {
            if (event.data && event.data.size) chunks.push(event.data);
        });
        media.addEventListener('stop', () => {
            stream.getTracks().forEach(track => track.stop());
            if (recorderState && recorderState.media === media) recorderState = null;
            if (recordings[key]) URL.revokeObjectURL(recordings[key]);
            recordings[key] = URL.createObjectURL(new Blob(chunks, { type: media.mimeType || 'audio/webm' }));
            paintRecorder(key, false);
            app.querySelector('[data-rec-play="' + key + '"]')?.classList.remove('hidden');
        });
        recorderState = { key, media };
        media.start();
        paintRecorder(key, true);
        window.setTimeout(() => {
            if (media.state === 'recording') media.stop();
        }, 30000);
    }

    function playRecording(key) {
        if (!recordings[key]) return;
        silence();
        new Audio(recordings[key]).play().catch(err => console.error(err));
    }

    function playModel(button) {
        const text = button.getAttribute('data-speak') || '';
        const audio = button.getAttribute('data-audio');
        if (!audio) {
            speakHebrew(text);
            return;
        }
        silence();
        let fell = false;
        const fallback = () => {
            if (fell) return;
            fell = true;
            speakHebrew(text);
        };
        const el = new Audio(audio);
        el.addEventListener('error', fallback);
        el.play().catch(fallback);
    }

    function renderMatch() {
        const englishList = document.getElementById('match-english-list');
        const hebrewList = document.getElementById('match-hebrew-list');
        if (!englishList || !hebrewList) return;
        selectedHebrewId = null;
        selectedEnglishId = null;
        englishList.innerHTML = shuffle(vocabItems()).map(item => matchButton('en', item, item.meaning)).join('');
        hebrewList.innerHTML = shuffle(vocabItems()).map(item => matchButton('he', item, item.word)).join('');
    }

    function matchButton(side, item, label) {
        return `
            <button type="button" id="match-${side}-${sid(item.id)}" class="word-match-card" data-match-side="${side}" data-match-id="${escapeHtml(sid(item.id))}">
                <span>${escapeHtml(label)}</span>
                <i class="fa-regular fa-circle"></i>
            </button>
        `;
    }

    function renderFill() {
        const sentences = document.getElementById('fill-sentences');
        if (!sentences) return;
        sentences.innerHTML = fillSentences().map((sentence, index) => `
            <div class="fill-line">
                <span class="fill-num">${index + 1}.</span>
                <span>${escapeHtml(sentence.textPre)} </span>
                ${sentence.prefix ? `<span class="fill-prefix">${escapeHtml(sentence.prefix)}</span>` : ''}
                <input id="fill-input-${sid(sentence.id)}" class="fill-input${sentence.prefix ? ' tight' : ''}" autocomplete="off" autocorrect="off" spellcheck="false" placeholder="___">
                <span>${escapeHtml(sentence.textPost || '')}</span>
                ${sentence.english || (sentence.gloss || []).length ? `
                    <div>
                        <button type="button" class="linkish" data-gloss="${sid(sentence.id)}">על מה המשפט? (What is this sentence about?)</button>
                        <div id="gloss-${sid(sentence.id)}" class="example hidden">
                            ${sentence.english ? `<p class="ltr">${escapeHtml(sentence.english)}</p>` : ''}
                            ${renderGlossChips(sentence.gloss)}
                        </div>
                    </div>
                ` : ''}
            </div>
        `).join('');
        sentences.querySelectorAll('.fill-input').forEach(input => {
            input.addEventListener('focus', () => setActiveFill(input.id));
            input.addEventListener('click', () => onBlankClick(input));
            input.addEventListener('input', () => onFillInput(input));
            input.addEventListener('dragover', event => event.preventDefault());
            input.addEventListener('drop', event => handleWordDrop(event, input.id));
        });
        refreshWordBank();
    }

    function renderGrammar() {
        const container = document.getElementById('grammar-items');
        if (!container) return;
        const prompt = ws.grammar?.prompt || 'בחרו את הניסוח הנכון.';
        container.innerHTML = grammarItems().map((item, index) => {
            const lead = item.direct || item.prompt || '';
            return `
            <article class="card stack" id="grammar-${sid(item.id)}">
                <div class="banner-row">
                    <div>
                        <span class="part-label">תרגיל ${index + 1}</span>
                        <p class="prompt">${escapeHtml(prompt)}</p>
                        <p class="direct-line">${escapeHtml(lead)}</p>
                    </div>
                </div>
                <div class="choices">
                    ${(item.choices || []).map((choice, choiceIndex) => `
                        <button type="button" id="choice-${sid(item.id)}-${choiceIndex}" class="choice-card" data-choice-id="${escapeHtml(sid(item.id))}" data-choice-index="${choiceIndex}">
                            ${escapeHtml(choice)}
                        </button>
                    `).join('')}
                </div>
                <div id="grammar-fb-${sid(item.id)}" class="why hidden"></div>
            </article>
        `;
        }).join('');
    }

    function renderReading() {
        const container = document.getElementById('reading-questions');
        if (!container) return;
        container.innerHTML = readingQuestions().map((question, index) => `
            <div class="question stack">
                <div class="q-row">
                    <label for="reading-ans-${sid(question.id)}">${index + 1}. ${escapeHtml(question.question)}</label>
                    <button type="button" class="speak" data-speak="${escapeHtml(question.question)}" title="הקראת השאלה"><i class="fa-solid fa-volume-high"></i></button>
                </div>
                <textarea id="reading-ans-${sid(question.id)}" class="answer" rows="2" spellcheck="false" placeholder="כתבו את התשובה בעברית..."></textarea>
                ${question.sampleAnswer ? `
                    <div>
                        <button type="button" class="linkish" data-sample="${sid(question.id)}">
                            <i class="fa-regular fa-lightbulb"></i>
                            <span id="sample-label-${sid(question.id)}">הצגת תשובת דוגמה (Show sample)</span>
                        </button>
                        <div id="sample-${sid(question.id)}" class="sample hidden"><strong>תשובת דוגמה:</strong> ${escapeHtml(question.sampleAnswer)}</div>
                    </div>
                ` : ''}
            </div>
        `).join('');
        container.querySelectorAll('textarea').forEach(area => area.addEventListener('input', saveLocalProgress));
    }

    function renderFriends() {
        const container = document.getElementById('friend-questions');
        if (!container) return;
        container.innerHTML = friendQuestions().map((question, index) => `
            <article class="card friend-card stack">
                <div class="banner-row">
                    <span class="q-label">שאלה ${index + 1}</span>
                    <div class="row-tools">
                        <button type="button" class="speak${question.audio ? ' human' : ''}" data-speak="${escapeHtml(question.pointed || question.speak || question.question)}"${question.audio ? ` data-audio="${escapeHtml(question.audio)}"` : ''} title="${question.audio ? 'הקלטה של אדם' : 'קול המחשב הוא כיוון גס'}"><i class="fa-solid fa-volume-high"></i> הקראה</button>
                        ${recorderHtml('friend-' + sid(question.id))}
                    </div>
                </div>
                <h3 data-pointed="${escapeHtml(question.pointed || question.question)}" data-plain="${escapeHtml(question.question)}">${escapeHtml(question.pointed || question.question)}</h3>
                ${question.youFeminine ? `<p class="fine you-f">לה: ${escapeHtml(question.youFeminine)}</p>` : ''}
                ${question.gloss ? `<p class="trap">${escapeHtml(question.gloss)}</p>` : ''}
                ${question.followUp ? `<p class="fine">המשך: ${escapeHtml(question.followUp)}</p>` : ''}
                <textarea id="friend-ans-${sid(question.id)}" rows="3" spellcheck="false" placeholder="אמרו את התשובה בקול, ואז כתבו אותה..."></textarea>
            </article>
        `).join('');
        applyVowels();
        container.querySelectorAll('textarea').forEach(area => {
            area.addEventListener('input', () => {
                updateFriendSummary();
                saveLocalProgress();
            });
        });
        updateFriendSummary();
    }

    function bindEvents() {
        app.addEventListener('click', onAppClick);
        document.getElementById('retell-answer')?.addEventListener('input', saveLocalProgress);
        document.getElementById('follow-along')?.addEventListener('change', onFollowAlongChange);
    }

    function onAppClick(event) {
        const speak = event.target.closest('[data-speak]');
        if (speak) playModel(speak);

        const recToggle = event.target.closest('[data-rec-toggle]');
        if (recToggle) toggleRecording(recToggle.dataset.recToggle);

        const recPlay = event.target.closest('[data-rec-play]');
        if (recPlay) playRecording(recPlay.dataset.recPlay);

        const mark = event.target.closest('[data-mark]');
        if (mark) markWord(mark);

        const tab = event.target.closest('[data-tab]');
        if (tab) switchTab(tab.dataset.tab);

        const match = event.target.closest('[data-match-id]');
        if (match) handleMatchSelect(match.dataset.matchSide, match.dataset.matchId);

        const choice = event.target.closest('[data-choice-id]');
        if (choice) selectGrammar(choice.dataset.choiceId, Number(choice.dataset.choiceIndex));

        const sample = event.target.closest('[data-sample]');
        if (sample) toggleSample(sample.dataset.sample);

        const reveal = event.target.closest('[data-reveal]');
        if (reveal) toggleReveal(reveal);

        const meaning = event.target.closest('[data-meaning]');
        if (meaning) toggleMeaning(meaning);

        const gloss = event.target.closest('[data-gloss]');
        if (gloss) toggleBox('gloss-' + gloss.dataset.gloss);

        const bank = event.target.closest('[data-bank-word]');
        if (bank && event.target.closest('#word-bank') && !suppressBankClick) onBankWordClick(bank.dataset.bankWord);

        const action = event.target.closest('[data-action]')?.dataset.action;
        if (action === 'again') startNextRound();
        if (action === 'reset-match') resetMatch();
        if (action === 'check-fill') checkFill();
        if (action === 'reset-fill') resetFill();
        if (action === 'check-grammar') checkGrammar();
        if (action === 'reset-grammar') resetGrammar();
        if (action === 'copy-notes') copyFriendNotes();
        if (action === 'story-play') startStory(story.status === 'paused' ? story.index : 0);
        if (action === 'story-pause') pauseStory();
        if (action === 'story-stop') stopStory();
        if (action === 'toggle-vowels') toggleVowels();
        if (action === 'open-bank') {
            bankOpen = true;
            paintBank();
        }
        if (action === 'hide-story') toggleStoryVisible(event.target.closest('[data-action]'));
        if (action === 'retell-timer') startRetellClock();
    }

    function switchTab(id) {
        app.querySelectorAll('.panel').forEach(panel => panel.classList.remove('active'));
        app.querySelectorAll('.tab').forEach(tab => tab.classList.remove('active'));
        document.getElementById('panel-' + id)?.classList.add('active');
        app.querySelector(`[data-tab="${id}"]`)?.classList.add('active');
    }

    let hebrewVoice = null;

    function pickHebrewVoice() {
        if (!('speechSynthesis' in window)) return;
        const voices = window.speechSynthesis.getVoices();
        const hebrew = voices.filter(voice => (voice.lang || '').toLowerCase().startsWith('he'));
        hebrewVoice = hebrew.find(voice => /carmit|female|yael/i.test(voice.name)) || hebrew[0] || null;
    }

    function buildUtterance(text) {
        const utterance = new SpeechSynthesisUtterance(text);
        utterance.lang = 'he-IL';
        utterance.rate = 0.85;
        pickHebrewVoice();
        if (hebrewVoice) utterance.voice = hebrewVoice;
        return utterance;
    }

    function whenVoicesReady(start) {
        if (window.speechSynthesis.getVoices().length) {
            start();
            return;
        }
        let started = false;
        const once = () => {
            if (started) return;
            started = true;
            window.speechSynthesis.removeEventListener('voiceschanged', once);
            start();
        };
        window.speechSynthesis.addEventListener('voiceschanged', once);
        window.speechSynthesis.getVoices();
        window.setTimeout(once, 500);
    }

    function speakHebrew(text) {
        if (!('speechSynthesis' in window) || !text) return;
        stopStory();
        whenVoicesReady(() => {
            const utterance = buildUtterance(text);
            window.speechSynthesis.cancel();
            window.setTimeout(() => window.speechSynthesis.speak(utterance), 60);
        });
    }

    // Story read-aloud. One utterance per paragraph, so Pause and Stop take effect at once
    // and a long story is not cut off. `story.run` changes on every start, pause and stop,
    // which makes the events of a cancelled utterance harmless.
    function storyLines() {
        return paragraphs().map(paragraph => (Array.isArray(paragraph) ? paragraph : [paragraph]).map(partText).join('').trim());
    }

    function followAlongPref() {
        try {
            const prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return prefs.followAlong !== false;
        } catch (err) {
            return true;
        }
    }

    function followAlongOn() {
        return !!document.getElementById('follow-along')?.checked;
    }

    function onFollowAlongChange() {
        try {
            const prefs = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            prefs.followAlong = followAlongOn();
            localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
        } catch (err) {
            console.error(err);
        }
        paintCurrentLine();
    }

    function paintStory() {
        const play = document.getElementById('story-play');
        const pause = document.getElementById('story-pause');
        const stop = document.getElementById('story-stop');
        if (!play || !pause || !stop) return;
        play.classList.toggle('hidden', story.status === 'playing');
        pause.classList.toggle('hidden', story.status !== 'playing');
        stop.classList.toggle('hidden', story.status === 'idle');
        play.innerHTML = story.status === 'paused'
            ? '<i class="fa-solid fa-play"></i> המשך (Resume)'
            : '<i class="fa-solid fa-volume-high"></i> הקראת טקסט (Read aloud)';
        paintCurrentLine();
    }

    function paintCurrentLine() {
        const lines = app.querySelectorAll('#story-text p');
        lines.forEach(line => line.classList.remove('reading-now'));
        if (story.status === 'idle' || !followAlongOn()) return;
        const line = lines[story.index];
        if (!line) return;
        line.classList.add('reading-now');
        if (story.status === 'playing' && line.scrollIntoView) line.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }

    function readStoryLine(run) {
        if (run !== story.run || story.status !== 'playing') return;
        const lines = storyLines();
        if (story.index >= lines.length) {
            stopStory();
            return;
        }
        paintCurrentLine();
        const utterance = buildUtterance(lines[story.index]);
        story.utterance = utterance;
        utterance.onend = () => {
            if (run !== story.run || story.status !== 'playing') return;
            story.index += 1;
            readStoryLine(run);
        };
        utterance.onerror = () => {
            if (run !== story.run) return;
            stopStory();
        };
        window.speechSynthesis.speak(utterance);
    }

    function startStory(from) {
        if (!('speechSynthesis' in window) || !storyLines().length) return;
        story.run += 1;
        const run = story.run;
        story.status = 'playing';
        story.index = from || 0;
        paintStory();
        whenVoicesReady(() => {
            if (run !== story.run) return;
            window.speechSynthesis.cancel();
            window.setTimeout(() => readStoryLine(run), 60);
        });
    }

    function pauseStory() {
        if (story.status !== 'playing') return;
        story.run += 1;
        story.status = 'paused';
        window.speechSynthesis.cancel();
        paintStory();
    }

    function stopStory() {
        const wasActive = story.status !== 'idle';
        story.run += 1;
        story.status = 'idle';
        story.index = 0;
        story.utterance = null;
        if (wasActive && 'speechSynthesis' in window) window.speechSynthesis.cancel();
        paintStory();
    }

    function silence() {
        stopStory();
        if ('speechSynthesis' in window) window.speechSynthesis.cancel();
    }

    window.addEventListener('pagehide', () => {
        if ('speechSynthesis' in window) window.speechSynthesis.cancel();
    });

    if ('speechSynthesis' in window) {
        pickHebrewVoice();
        window.speechSynthesis.addEventListener('voiceschanged', pickHebrewVoice);
    }

    function handleMatchSelect(side, id) {
        if (matchedPairs.includes(id)) return;
        if (side === 'en') {
            if (selectedEnglishId) document.getElementById('match-en-' + selectedEnglishId)?.classList.remove('selected');
            selectedEnglishId = id;
            document.getElementById('match-en-' + id)?.classList.add('selected');
        } else {
            if (selectedHebrewId) document.getElementById('match-he-' + selectedHebrewId)?.classList.remove('selected');
            selectedHebrewId = id;
            document.getElementById('match-he-' + id)?.classList.add('selected');
        }
        if (!selectedHebrewId || !selectedEnglishId) return;

        const enBtn = document.getElementById('match-en-' + selectedEnglishId);
        const heBtn = document.getElementById('match-he-' + selectedHebrewId);
        if (selectedHebrewId === selectedEnglishId) {
            matchedPairs.push(selectedHebrewId);
            enBtn?.classList.remove('selected');
            heBtn?.classList.remove('selected');
            enBtn?.classList.add('matched');
            heBtn?.classList.add('matched');
            selectedHebrewId = null;
            selectedEnglishId = null;
            if (matchedPairs.length === vocabItems().length) showMatchSuccess();
        } else {
            enBtn?.classList.add('flash');
            heBtn?.classList.add('flash');
            const wrongEn = selectedEnglishId;
            const wrongHe = selectedHebrewId;
            selectedHebrewId = null;
            selectedEnglishId = null;
            window.setTimeout(() => {
                const en = document.getElementById('match-en-' + wrongEn);
                const he = document.getElementById('match-he-' + wrongHe);
                en?.classList.remove('flash');
                he?.classList.remove('flash');
                if (selectedEnglishId !== wrongEn) en?.classList.remove('selected');
                if (selectedHebrewId !== wrongHe) he?.classList.remove('selected');
            }, 600);
        }
        saveLocalProgress();
    }

    function showMatchSuccess() {
        const feedback = document.getElementById('match-feedback');
        if (!feedback) return;
        feedback.classList.remove('hidden');
        feedback.className = 'match-feedback feedback-ok';
        feedback.textContent = 'כל הכבוד! התאמתם את כל המילים. (Well done! Every word is matched.)';
    }

    function resetMatch() {
        matchedPairs = [];
        const feedback = document.getElementById('match-feedback');
        if (feedback) {
            feedback.className = 'match-feedback hidden';
            feedback.textContent = '';
        }
        renderMatch();
        saveLocalProgress();
    }

    function setActiveFill(id) {
        activeInputId = id;
        app.querySelectorAll('.fill-input').forEach(input => input.classList.toggle('ready', input.id === id));
    }

    function onFillInput(input) {
        input.classList.remove('good', 'bad');
        refreshWordBank();
        saveLocalProgress();
    }

    function usedWordNorms() {
        const counts = {};
        fillSentences().forEach(sentence => {
            const value = normalizeHebrew(document.getElementById('fill-input-' + sid(sentence.id))?.value || '');
            if (!value) return;
            counts[value] = (counts[value] || 0) + 1;
        });
        return counts;
    }

    function refreshWordBank() {
        const bank = document.getElementById('word-bank');
        if (!bank) return;
        const used = usedWordNorms();
        bank.innerHTML = fillBank().map(word => {
            const isUsed = (used[normalizeHebrew(word)] || 0) > 0;
            const picked = !isUsed && selectedBankWord && normalizeHebrew(selectedBankWord) === normalizeHebrew(word);
            return `
                <button type="button" class="bank-word${isUsed ? ' used' : ''}${picked ? ' picked' : ''}" draggable="${isUsed ? 'false' : 'true'}" data-bank-word="${escapeHtml(word)}">
                    <i class="fa-solid fa-grip-vertical"></i>
                    <span>${escapeHtml(word)}</span>
                </button>
            `;
        }).join('');
        bank.querySelectorAll('[data-bank-word]').forEach(button => {
            button.addEventListener('dragstart', event => {
                suppressBankClick = true;
                event.dataTransfer.setData('text/plain', button.dataset.bankWord);
                event.dataTransfer.effectAllowed = 'copy';
            });
            button.addEventListener('dragend', () => {
                window.setTimeout(() => { suppressBankClick = false; }, 80);
            });
        });
    }

    function placeWord(input, word) {
        if (!input) return;
        if (selectedBankWord && normalizeHebrew(selectedBankWord) === normalizeHebrew(word)) selectedBankWord = null;
        input.value = word;
        input.classList.remove('good', 'bad');
        refreshWordBank();
        saveLocalProgress();
    }

    function onBlankClick(input) {
        if (!selectedBankWord) return;
        placeWord(input, selectedBankWord);
    }

    function clearWord(word) {
        const norm = normalizeHebrew(word);
        for (const sentence of fillSentences()) {
            const input = document.getElementById('fill-input-' + sid(sentence.id));
            if (input && normalizeHebrew(input.value) === norm) {
                input.value = '';
                input.classList.remove('good', 'bad');
                refreshWordBank();
                saveLocalProgress();
                return true;
            }
        }
        return false;
    }

    function onBankWordClick(word) {
        if (clearWord(word)) {
            if (selectedBankWord && normalizeHebrew(selectedBankWord) === normalizeHebrew(word)) selectedBankWord = null;
            return;
        }
        if ((usedWordNorms()[normalizeHebrew(word)] || 0) > 0) return;
        selectedBankWord = selectedBankWord && normalizeHebrew(selectedBankWord) === normalizeHebrew(word) ? null : word;
        refreshWordBank();
    }

    function handleWordDrop(event, inputId) {
        event.preventDefault();
        const word = event.dataTransfer.getData('text/plain');
        if (!word || (usedWordNorms()[normalizeHebrew(word)] || 0) > 0) return;
        setActiveFill(inputId);
        placeWord(document.getElementById(inputId), word);
    }

    function checkFill() {
        let correctCount = 0;
        fillSentences().forEach(sentence => {
            const input = document.getElementById('fill-input-' + sid(sentence.id));
            if (!input) return;
            const correct = normalizeHebrew(input.value) === normalizeHebrew(sentence.answer);
            input.classList.remove('good', 'bad');
            input.classList.add(correct ? 'good' : 'bad');
            if (correct) correctCount++;
        });
        const feedback = document.getElementById('fill-feedback');
        if (!feedback) return;
        if (correctCount === fillSentences().length) {
            feedback.className = 'feedback-ok';
            feedback.textContent = 'מצוין! כל המשפטים נכונים. (Excellent! Every sentence is right.)';
        } else {
            feedback.className = 'feedback-bad';
            feedback.textContent = correctCount + ' מתוך ' + fillSentences().length + ' נכונים. נסו שוב. (' + correctCount + ' of ' + fillSentences().length + ' correct. Try again.)';
        }
        saveLocalProgress();
    }

    function resetFill() {
        fillSentences().forEach(sentence => {
            const input = document.getElementById('fill-input-' + sid(sentence.id));
            if (!input) return;
            input.value = '';
            input.classList.remove('good', 'bad', 'ready');
        });
        activeInputId = null;
        selectedBankWord = null;
        const feedback = document.getElementById('fill-feedback');
        if (feedback) feedback.textContent = '';
        refreshWordBank();
        saveLocalProgress();
    }

    function paintGrammar(id, showResult) {
        const item = grammarItems().find(entry => sid(entry.id) === sid(id));
        if (!item) return;
        const selected = grammarState[sid(id)];
        (item.choices || []).forEach((_, index) => {
            const button = document.getElementById('choice-' + sid(id) + '-' + index);
            if (!button) return;
            button.classList.remove('selected', 'correct', 'incorrect');
            if (showResult && index === item.answer) button.classList.add('correct');
            else if (showResult && index === selected) button.classList.add('incorrect');
            else if (index === selected) button.classList.add('selected');
        });
    }

    function selectGrammar(id, index) {
        grammarState[sid(id)] = index;
        paintGrammar(id, false);
        const feedback = document.getElementById('grammar-fb-' + sid(id));
        if (feedback) {
            feedback.className = 'why hidden';
            feedback.textContent = '';
        }
        const summary = document.getElementById('grammar-summary');
        if (summary) summary.textContent = '';
        saveLocalProgress();
    }

    function checkGrammar() {
        let correctCount = 0;
        grammarItems().forEach(item => {
            const selected = grammarState[sid(item.id)];
            const feedback = document.getElementById('grammar-fb-' + sid(item.id));
            const isCorrect = selected === item.answer;
            if (isCorrect) correctCount++;
            paintGrammar(item.id, true);
            if (!feedback) return;
            feedback.classList.remove('hidden');
            if (selected === undefined) {
                feedback.className = 'why feedback-warn';
                feedback.textContent = 'עדיין לא בחרתם. (Not chosen yet.) ' + (item.why || '');
            } else if (isCorrect) {
                feedback.className = 'why feedback-ok';
                feedback.textContent = 'נכון. (Correct.) ' + (item.why || '');
            } else {
                feedback.className = 'why feedback-bad';
                feedback.textContent = 'לא מדויק. (Not quite.) ' + (item.why || '');
            }
            const model = item.pointed || (item.choices || [])[item.answer];
            if (model) {
                feedback.insertAdjacentHTML('beforeend', ` <button type="button" class="speak" data-speak="${escapeHtml(model)}"><i class="fa-solid fa-volume-high"></i> שמעו את המשפט הנכון (Hear the right sentence)</button>`);
            }
        });
        const summary = document.getElementById('grammar-summary');
        if (!summary) return;
        summary.className = correctCount === grammarItems().length ? 'feedback-ok' : 'feedback-bad';
        summary.textContent = correctCount === grammarItems().length
            ? 'מצוין! כל התרגילים נכונים. (Excellent! Every exercise is right.)'
            : correctCount + ' מתוך ' + grammarItems().length + ' נכונים. (' + correctCount + ' of ' + grammarItems().length + ' correct.)';
        saveLocalProgress();
    }

    function resetGrammar() {
        grammarItems().forEach(item => delete grammarState[sid(item.id)]);
        const summary = document.getElementById('grammar-summary');
        if (summary) summary.textContent = '';
        renderGrammar();
        saveLocalProgress();
    }

    function toggleSample(id) {
        const box = document.getElementById('sample-' + id);
        const label = document.getElementById('sample-label-' + id);
        if (!box || !label) return;
        const open = box.classList.contains('hidden');
        box.classList.toggle('hidden', !open);
        label.textContent = open
            ? 'הסתרת תשובת דוגמה (Hide sample)'
            : 'הצגת תשובת דוגמה (Show sample)';
    }

    function updateFriendSummary() {
        const summary = friendQuestions().map((question, index) => {
            const value = document.getElementById('friend-ans-' + sid(question.id))?.value.trim() || '';
            return (index + 1) + '. ' + question.question + '\nתשובה: ' + (value || '(עדיין אין תשובה)');
        }).join('\n\n');
        const el = document.getElementById('friend-summary');
        if (el) el.textContent = summary;
    }

    function copyFriendNotes() {
        const text = document.getElementById('friend-summary')?.textContent || '';
        const done = () => alert('התשובות הועתקו. (Answers copied.)');
        if (navigator.clipboard?.writeText && window.isSecureContext) {
            navigator.clipboard.writeText(text).then(done).catch(() => fallbackCopy(text, done));
            return;
        }
        fallbackCopy(text, done);
    }

    function fallbackCopy(text, done) {
        const area = document.createElement('textarea');
        area.value = text;
        area.setAttribute('readonly', '');
        area.style.position = 'fixed';
        area.style.opacity = '0';
        document.body.appendChild(area);
        area.focus();
        area.select();
        let ok = false;
        try { ok = document.execCommand('copy'); } catch (err) { ok = false; }
        area.remove();
        if (ok) done();
        else alert('לא הצלחנו להעתיק אוטומטית. סמנו את הסיכום והעתיקו אותו ידנית. (Automatic copy failed. Select the summary and copy it by hand.)');
    }

    function getProgressStateJSON() {
        const grammarAnswers = {};
        grammarItems().forEach(item => {
            if (grammarState[sid(item.id)] !== undefined) grammarAnswers[sid(item.id)] = grammarState[sid(item.id)];
        });
        return {
            lesson: ws.id,
            matchedPairs,
            fillAnswers: fillSentences().map(sentence => ({
                id: sentence.id,
                val: document.getElementById('fill-input-' + sid(sentence.id))?.value || ''
            })),
            grammarAnswers,
            indirectAnswers: grammarAnswers,
            readingAnswers: readingQuestions().map(question => ({
                id: question.id,
                val: document.getElementById('reading-ans-' + sid(question.id))?.value || ''
            })),
            friendAnswers: friendQuestions().map(question => ({
                id: question.id,
                val: document.getElementById('friend-ans-' + sid(question.id))?.value || ''
            })),
            round,
            completedRounds,
            roundStartedAt,
            wordMarks,
            missedLast,
            retellAnswer: document.getElementById('retell-answer')?.value || ''
        };
    }

    function saveLocalProgress() {
        if (!roundStartedAt) roundStartedAt = Date.now();
        try {
            localStorage.setItem(storageKey, JSON.stringify(getProgressStateJSON()));
        } catch (err) {
            console.error(err);
        }
    }

    function restoreProgressState(state) {
        if (!state) return;
        if (Array.isArray(state.matchedPairs)) {
            matchedPairs = state.matchedPairs.map(sid);
            matchedPairs.forEach(id => {
                document.getElementById('match-en-' + id)?.classList.add('matched');
                document.getElementById('match-he-' + id)?.classList.add('matched');
            });
            if (vocabItems().length && matchedPairs.length === vocabItems().length) showMatchSuccess();
        }
        if (Array.isArray(state.fillAnswers)) {
            state.fillAnswers.forEach(item => {
                const input = document.getElementById('fill-input-' + sid(item.id));
                if (input) input.value = item.val;
            });
            refreshWordBank();
        }
        const savedGrammar = state.grammarAnswers || state.indirectAnswers;
        if (savedGrammar && typeof savedGrammar === 'object') {
            Object.entries(savedGrammar).forEach(([id, index]) => {
                grammarState[sid(id)] = index;
            });
            grammarItems().forEach(item => paintGrammar(item.id, false));
        }
        if (Array.isArray(state.readingAnswers)) {
            state.readingAnswers.forEach(item => {
                const el = document.getElementById('reading-ans-' + sid(item.id));
                if (el) el.value = item.val;
            });
        }
        if (Array.isArray(state.friendAnswers)) {
            state.friendAnswers.forEach(item => {
                const el = document.getElementById('friend-ans-' + sid(item.id));
                if (el) el.value = item.val;
            });
            updateFriendSummary();
        }
        if (Number.isInteger(state.round) && state.round > 0) round = state.round;
        if (Number.isInteger(state.completedRounds) && state.completedRounds >= 0) completedRounds = state.completedRounds;
        if (Number.isFinite(state.roundStartedAt) && state.roundStartedAt > 0) roundStartedAt = state.roundStartedAt;
        if (state.wordMarks && typeof state.wordMarks === 'object') wordMarks = Object.assign({}, state.wordMarks);
        if (Array.isArray(state.missedLast)) missedLast = state.missedLast.map(sid);
        const retell = document.getElementById('retell-answer');
        if (retell && typeof state.retellAnswer === 'string') retell.value = state.retellAnswer;
        if (recallMode()) renderVocab();
        paintRound();
    }

    function restoreSaved() {
        try {
            const saved = localStorage.getItem(storageKey) || (ws.legacyStorageKey ? localStorage.getItem(ws.legacyStorageKey) : '');
            if (saved) restoreProgressState(JSON.parse(saved));
        } catch (err) {
            console.error(err);
        }
    }

    function validate() {
        grammarItems().forEach((item, index) => {
            const choices = item.choices || [];
            if (!Number.isInteger(item.answer) || item.answer < 0 || item.answer >= choices.length) {
                console.error('Grammar item ' + (item.id ?? index) + ' has an answer index outside its choices.');
            }
        });
        const answers = new Set(fillSentences().map(sentence => normalizeHebrew(sentence.answer)));
        fillBank().forEach(word => {
            if (!answers.has(normalizeHebrew(word)) && fillBank().filter(entry => normalizeHebrew(entry) === normalizeHebrew(word)).length > 1) {
                console.error('Word bank repeats ' + word + '.');
            }
        });
    }
})();
