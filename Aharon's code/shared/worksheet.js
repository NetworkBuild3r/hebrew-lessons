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
        { id: 'practice', label: 'Vocab Practice', icon: 'fa-layer-group', tone: 'tone-green', enabled: () => vocabItems().length || fillSentences().length },
        { id: 'grammar', label: ws.grammar?.tabLabel || 'Grammar', icon: ws.grammar?.icon || 'fa-puzzle-piece', tone: 'tone-purple', enabled: () => grammarItems().length },
        { id: 'reading', label: 'Reading', icon: 'fa-file-lines', tone: 'tone-amber', enabled: () => paragraphs().length },
        { id: 'ask-friend', label: 'Ask a Friend', icon: 'fa-comments', tone: 'tone-rose', enabled: () => friendQuestions().length }
    ].filter(section => section.enabled());

    let selectedHebrewId = null;
    let selectedEnglishId = null;
    let matchedPairs = [];
    let activeInputId = null;
    let suppressBankClick = false;
    const grammarState = {};

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
    validate();

    function vocabItems() { return ws.vocab?.items || []; }
    function fillSentences() { return ws.fill?.sentences || []; }
    function fillBank() { return ws.fill?.bank || []; }
    function grammarItems() { return ws.grammar?.items || []; }
    function paragraphs() { return ws.reading?.paragraphs || []; }
    function readingQuestions() { return ws.reading?.questions || []; }
    function friendQuestions() { return ws.askFriend?.questions || []; }

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

    function renderParts(parts) {
        const list = Array.isArray(parts) ? parts : [parts];
        return list.map(part => {
            if (typeof part === 'string') return escapeHtml(part);
            if (!part || typeof part !== 'object') return '';
            if (part.reported) return `<span class="reported">${escapeHtml(part.reported)}</span>`;
            if (part.direct) return `<span class="direct-quote">${escapeHtml(part.direct)}</span>`;
            if (part.strong) return `<strong>${escapeHtml(part.strong)}</strong>`;
            return '';
        }).join('');
    }

    function normalizeHebrew(str) {
        return String(str || '')
            .replace(/[\u0591-\u05C7]/g, '')
            .replace(/[.,!?;:"'״׳\-–—()]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
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
                        <a class="back-link" href="../../index.html">כל הדפי עבודה</a>
                        <div class="kicker">${escapeHtml(ws.series || 'דרך עברית • Light & Easy Hebrew')}</div>
                        <h1>${escapeHtml(ws.title || 'דף עבודה')}</h1>
                        ${ws.subtitle ? `<p class="subtitle">${escapeHtml(ws.subtitle)}</p>` : ''}
                    </div>
                    <div>
                        <div class="savebar">
                            <button type="button" class="btn btn-save" data-action="download">
                                <i class="fa-solid fa-download"></i>
                                <span>שמירת התקדמות (Save JSON)</span>
                            </button>
                            <label class="btn btn-load">
                                <i class="fa-solid fa-upload"></i>
                                <span>טעינת התקדמות (Upload JSON)</span>
                                <input type="file" id="upload-json-input" accept=".json,application/json" hidden>
                            </label>
                        </div>
                        <p class="save-note">התשובות נשמרות אוטומטית בדפדפן הזה</p>
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
        const note = ws.vocab?.note || 'לחצו על כפתור השמע כדי לשמוע את משפט הדוגמה לאט (85%). אם אין במחשב קול עברי, ההקראה תישמע לא נכון — צריך להתקין עברית (Hebrew) בהגדרות השפה.';
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
                        <p class="fine">Choose a Hebrew word and its English meaning. A correct pair turns green.</p>
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
                        <div class="bank-words" id="word-bank"></div>
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
        return 'There are ' + fillBank().length + ' words and ' + fillSentences().length + ' sentences. ' + extraText + ' Each word can be used only once. Click a word, or drag it into a blank. Click a used word again to remove it.';
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
                    ${renderRules(grammar.rules)}
                </div>
                <div class="stack" id="grammar-items"></div>
                <div class="actions">
                    <button type="button" class="btn btn-purple" data-action="check-grammar">בדיקת תשובות (Check)</button>
                    <button type="button" class="btn btn-ghost" data-action="reset-grammar">איפוס (Reset)</button>
                    <span id="grammar-summary"></span>
                </div>
            </div>
        `;
    }

    function renderRules(rules) {
        if (!rules?.length) return '';
        return `<ol class="rules">${rules.map(rule => `
            <li class="rule">
                <strong>${escapeHtml(rule.title || '')}</strong>
                ${rule.english ? `<span class="ltr">${escapeHtml(rule.english)}</span>` : ''}
                ${rule.example ? `<span class="example-line">${renderParts(rule.example)}</span>` : ''}
            </li>
        `).join('')}</ol>`;
    }

    function hasSpeechMarks() {
        return paragraphs().some(paragraph => {
            const parts = Array.isArray(paragraph) ? paragraph : [paragraph];
            return parts.some(part => part && typeof part === 'object' && (part.reported || part.direct));
        });
    }

    function renderReadingPanel() {
        const legend = hasSpeechMarks()
            ? `<p class="legend"><span class="reported">סגול = דיבור עקיף</span> · <span class="direct-quote">אדום במרכאות = דיבור ישיר</span></p>`
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
                <div class="story">
                    <div class="story-head">
                        <div>
                            <h2>${escapeHtml(ws.reading?.heading || 'קראו את הטקסט — Read the text')}</h2>
                            ${legend}
                        </div>
                        <button type="button" class="btn btn-amber" data-action="read-story">
                            <i class="fa-solid fa-volume-high"></i> הקראת טקסט (Read Aloud)
                        </button>
                    </div>
                    <div class="story-body" id="story-text">${paragraphs().map(paragraph => `<p>${renderParts(paragraph)}</p>`).join('')}</div>
                </div>
                ${questions}
            </div>
        `;
    }

    function renderFriendPanel() {
        const count = friendQuestions().length;
        const intro = ws.askFriend?.intro || 'שאלו חבר או חברה שדוברים עברית את השאלות, וכתבו את התשובה שלהם.';
        const introEn = ws.askFriend?.introEn || 'Ask one Hebrew-speaking friend these questions. Write their answer in Hebrew. Slash forms such as את/ה let you match the person you ask.';
        return `
            <div class="stack">
                <div class="banner banner-rose">
                    <h2>${escapeHtml(ws.askFriend?.heading || 'שאל חבר / חברה — Ask a Friend')}</h2>
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
        if (count) count.textContent = vocabItems().length + ' מילות מפתח';
        if (!container) return;
        container.innerHTML = vocabItems().map(item => `
            <article class="card vocab-card stack">
                <div class="banner-row">
                    <div>
                        <h3>${escapeHtml(item.word)}</h3>
                        <p class="meta">${escapeHtml(item.translit || '')}${item.pos ? ` • <span class="pos">${escapeHtml(item.pos)}</span>` : ''}</p>
                        ${item.infinitive ? `<p class="infinitive">שם הפועל: ${escapeHtml(item.infinitive)}</p>` : ''}
                    </div>
                    ${item.sentence ? `<button type="button" class="icon-btn" data-speak="${escapeHtml(item.sentence)}" title="השמעת המשפט"><i class="fa-solid fa-volume-high"></i></button>` : ''}
                </div>
                <div>
                    <p class="meaning-label">Meaning</p>
                    <p>${escapeHtml(item.meaning || '')}</p>
                </div>
                ${item.sentence ? `<div class="example"><p class="meaning-label">משפט דוגמה</p><p>${escapeHtml(item.sentence)}</p></div>` : ''}
            </article>
        `).join('');
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
            </div>
        `).join('');
        sentences.querySelectorAll('.fill-input').forEach(input => {
            input.addEventListener('focus', () => setActiveFill(input.id));
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
                    ${lead ? `<button type="button" class="speak" data-speak="${escapeHtml(lead)}"><i class="fa-solid fa-volume-high"></i> הקראה</button>` : ''}
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
                    <button type="button" class="speak" data-speak="${escapeHtml(question.speak || question.question)}"><i class="fa-solid fa-volume-high"></i> הקראה</button>
                </div>
                <h3>${escapeHtml(question.question)}</h3>
                <textarea id="friend-ans-${sid(question.id)}" rows="3" spellcheck="false" placeholder="כתבו כאן את התשובה של החבר/ה..."></textarea>
            </article>
        `).join('');
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
        document.getElementById('upload-json-input')?.addEventListener('change', uploadProgressJSON);
    }

    function onAppClick(event) {
        const speak = event.target.closest('[data-speak]');
        if (speak) speakHebrew(speak.getAttribute('data-speak') || '');

        const tab = event.target.closest('[data-tab]');
        if (tab) switchTab(tab.dataset.tab);

        const match = event.target.closest('[data-match-id]');
        if (match) handleMatchSelect(match.dataset.matchSide, match.dataset.matchId);

        const choice = event.target.closest('[data-choice-id]');
        if (choice) selectGrammar(choice.dataset.choiceId, Number(choice.dataset.choiceIndex));

        const sample = event.target.closest('[data-sample]');
        if (sample) toggleSample(sample.dataset.sample);

        const bank = event.target.closest('[data-bank-word]');
        if (bank && event.target.closest('#word-bank') && !suppressBankClick) onBankWordClick(bank.dataset.bankWord);

        const action = event.target.closest('[data-action]')?.dataset.action;
        if (action === 'download') downloadProgressJSON();
        if (action === 'reset-match') resetMatch();
        if (action === 'check-fill') checkFill();
        if (action === 'reset-fill') resetFill();
        if (action === 'check-grammar') checkGrammar();
        if (action === 'reset-grammar') resetGrammar();
        if (action === 'copy-notes') copyFriendNotes();
        if (action === 'read-story') speakHebrew(document.getElementById('story-text')?.innerText || '');
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

    function speakHebrew(text) {
        if (!('speechSynthesis' in window) || !text) return;
        const start = () => {
            const utterance = new SpeechSynthesisUtterance(text);
            utterance.lang = 'he-IL';
            utterance.rate = 0.85;
            pickHebrewVoice();
            if (hebrewVoice) utterance.voice = hebrewVoice;
            window.speechSynthesis.cancel();
            window.setTimeout(() => window.speechSynthesis.speak(utterance), 60);
        };
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
        feedback.textContent = 'כל הכבוד! התאמתם את כל המילים.';
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
            return `
                <button type="button" class="bank-word${isUsed ? ' used' : ''}" draggable="${isUsed ? 'false' : 'true'}" data-bank-word="${escapeHtml(word)}">
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
        input.value = word;
        input.classList.remove('good', 'bad');
        refreshWordBank();
        saveLocalProgress();
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
        if (clearWord(word)) return;
        if ((usedWordNorms()[normalizeHebrew(word)] || 0) > 0) return;
        if (activeInputId) {
            const active = document.getElementById(activeInputId);
            if (active && !normalizeHebrew(active.value)) {
                placeWord(active, word);
                return;
            }
        }
        for (const sentence of fillSentences()) {
            const input = document.getElementById('fill-input-' + sid(sentence.id));
            if (input && !normalizeHebrew(input.value)) {
                setActiveFill(input.id);
                placeWord(input, word);
                return;
            }
        }
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
            feedback.textContent = 'מצוין! כל המשפטים נכונים.';
        } else {
            feedback.className = 'feedback-bad';
            feedback.textContent = correctCount + ' מתוך ' + fillSentences().length + ' נכונים. נסו שוב.';
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
                feedback.textContent = 'עדיין לא בחרתם. ' + (item.why || '');
            } else if (isCorrect) {
                feedback.className = 'why feedback-ok';
                feedback.textContent = 'נכון. ' + (item.why || '');
            } else {
                feedback.className = 'why feedback-bad';
                feedback.textContent = 'לא מדויק. ' + (item.why || '');
            }
        });
        const summary = document.getElementById('grammar-summary');
        if (!summary) return;
        summary.className = correctCount === grammarItems().length ? 'feedback-ok' : 'feedback-bad';
        summary.textContent = correctCount === grammarItems().length
            ? 'מצוין! כל התרגילים נכונים.'
            : correctCount + ' מתוך ' + grammarItems().length + ' נכונים.';
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
        const done = () => alert('התשובות הועתקו.');
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
        else alert('לא הצלחנו להעתיק אוטומטית. סמנו את הסיכום והעתיקו אותו ידנית.');
    }

    function getProgressStateJSON() {
        const grammarAnswers = {};
        grammarItems().forEach(item => {
            if (grammarState[sid(item.id)] !== undefined) grammarAnswers[sid(item.id)] = grammarState[sid(item.id)];
        });
        return {
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
            }))
        };
    }

    function saveLocalProgress() {
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
    }

    function restoreSaved() {
        try {
            const saved = localStorage.getItem(storageKey) || (ws.legacyStorageKey ? localStorage.getItem(ws.legacyStorageKey) : '');
            if (saved) restoreProgressState(JSON.parse(saved));
        } catch (err) {
            console.error(err);
        }
    }

    function downloadProgressJSON() {
        const blob = new Blob([JSON.stringify(getProgressStateJSON(), null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = ws.id + '_progress.json';
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
    }

    function uploadProgressJSON(event) {
        const file = event.target.files && event.target.files[0];
        event.target.value = '';
        if (!file) return;
        const reader = new FileReader();
        reader.onload = function (e) {
            try {
                restoreProgressState(JSON.parse(e.target.result));
                saveLocalProgress();
                alert('ההתקדמות נטענה.');
            } catch (err) {
                alert('לא הצלחנו לקרוא את הקובץ. צריך קובץ JSON מהכפתור שמירה.');
            }
        };
        reader.readAsText(file);
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
