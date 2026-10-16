/**
 * Today, the plan overview, and renderers for the six lesson-plan block types.
 * Each component is `(props) => string`. Plan text goes through `HebrewUtils.escapeHtml`.
 * A picture `src` is only `GET /api/media/:id`. The plan does not supply the URL.
 * The learner fetch is `GET /api/plans/:id/published`.
 * Provenance: INIT-004/SPEC-014
 * Explainer, conjugation-table, and personal-question: INIT-006/SPEC-014
 * Verse-reading, letter-intro, and recipe chips: INIT-006/SPEC-016
 */
(function (root, factory) {
    'use strict';

    var api = factory(root);
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.HebrewPlanRender = api;
    if (typeof document !== 'undefined') api.boot(document);
})(typeof window !== 'undefined' ? window : globalThis, function (root) {
    'use strict';

    var TEXT_LIMIT = 4000;
    var MEDIA_ID = /^[a-z0-9][a-z0-9-]{0,63}$/;
    var SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
    var AUDIO_FILE = /^(?:[A-Za-z0-9_-]+\/)*[A-Za-z0-9_.-]+\.mp3$/;
    // A shared word clip: `../_audio/words/<name>.mp3`. Only the name is put on the page;
    // the address is built when the learner presses play.
    var WORD_CLIP = /^(?:\.\.\/)?_audio\/words\/([A-Za-z0-9_-]{8,64})\.mp3$/;
    var WORD_CLIP_BASE = '/worksheets/_audio/words/';
    var SCRIPT_SRC = '/shared/ui/plan-render.js';

    /**
     * The app's HTML escape helper. In the browser the same four replacements
     * are used when `HebrewUtils` is not on the page (this file is the one script).
     * @param {object} scope
     * @returns {function(string): string}
     */
    function resolveEscape(scope) {
        if (scope.HebrewUtils && typeof scope.HebrewUtils.escapeHtml === 'function') {
            return scope.HebrewUtils.escapeHtml;
        }
        if (typeof require === 'function') {
            try {
                require('../player/utils.js');
            } catch (err) {
                /* Browser pages load this file alone. Node tests load the helper. */
            }
            if (scope.HebrewUtils && typeof scope.HebrewUtils.escapeHtml === 'function') {
                return scope.HebrewUtils.escapeHtml;
            }
        }
        return function (str) {
            return String(str == null ? '' : str)
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;');
        };
    }

    var escapeHtml = resolveEscape(root);
    var live = { plan: null, forms: Object.create(null), lexicon: Object.create(null), audio: null, rng: null };

    var RECIPE_CHIP = {
        'topic-conversation': ['שיחה', 'Conversation'],
        'review-worksheet': ['חזרה', 'Review'],
        'verse-reading': ['פסוק', 'Verse'],
        'verb-workshop': ['פעלים', 'Verbs'],
        'letters-and-vowels': ['אותיות', 'Letters'],
        guide: ['מדריך', 'Guide']
    };

    /**
     * @param {string} globalName
     * @param {string} file
     * @returns {object|null}
     */
    function loadBlock(globalName, file) {
        var existing = root[globalName];
        if (existing && typeof existing.render === 'function') return existing;
        if (typeof require === 'function') {
            try {
                return require(file);
            } catch (err) {
                return null;
            }
        }
        return null;
    }

    var REGISTRY = {
        explainer: loadBlock('HebrewBlockExplainer', './blocks/explainer.js'),
        'conjugation-table': loadBlock('HebrewBlockConjugation', './blocks/conjugation-table.js'),
        'personal-question': loadBlock('HebrewBlockPersonal', './blocks/personal-question.js'),
        'verse-reading': loadBlock('HebrewBlockVerse', './blocks/verse-reading.js'),
        'letter-intro': loadBlock('HebrewBlockLetter', './blocks/letter-intro.js'),
        exercise: loadBlock('HebrewBlockExercise', './blocks/exercise.js'),
        assignment: loadBlock('HebrewBlockAssignment', './blocks/assignment.js')
    };

    /**
     * Escape plan text and cap it at the lesson-plan text limit.
     * @param {*} value
     * @returns {string}
     */
    function text(value) {
        var raw = value == null ? '' : String(value);
        if (raw.length > TEXT_LIMIT) raw = raw.slice(0, TEXT_LIMIT);
        return escapeHtml(raw);
    }

    /**
     * English gloss. Direction is an attribute.
     * @param {*} value
     * @returns {string}
     */
    function gloss(value) {
        return '<span class="meaning" lang="en" dir="ltr">' + text(value) + '</span>';
    }

    /**
     * @param {*} value
     * @returns {'he'|'en'}
     */
    function lineLang(value) {
        return /[\u0590-\u05FF]/.test(String(value || '')) ? 'he' : 'en';
    }

    /**
     * @param {*} value
     * @returns {string}
     */
    function subtitleLine(value) {
        var lang = lineLang(value);
        var dir = lang === 'en' ? ' dir="ltr"' : '';
        return '<p class="pointed-line" lang="' + lang + '"' + dir + '>' + text(value) + '</p>';
    }

    /**
     * @param {string} id
     * @param {string} inner
     * @returns {string}
     */
    function screen(id, inner) {
        return '<article class="screen" id="' + id + '" data-screen="' + id + '">\n' + inner + '\n</article>';
    }

    /**
     * @param {string} href One of the fixed screen hashes. Never plan text.
     * @param {string} label Static Hebrew label.
     * @returns {string}
     */
    function dock(href, label) {
        return '<p class="dock"><a class="primary" href="' + href + '">' + label + '</a></p>';
    }

    /**
     * @param {string} kickerHtml Static markup, not plan text.
     * @param {*} title
     * @returns {string}
     */
    function mast(kickerHtml, title) {
        return '<header class="mast"><p class="kicker">' + kickerHtml + '</p>'
            + '<h1 lang="he">' + text(title) + '</h1></header>';
    }

    /**
     * Play control. A missing or unsafe path is not turned into a request.
     * A real `audioPath` is not copied into a URL. Only a shared word clip
     * (`_audio/words/<name>.mp3`) is played, and its address is built on press;
     * any other lesson-relative path is not fetched from this page.
     * A shared word clip carries its file name (no folder, no extension) as
     * `data-plan-audio-clip`; the word itself rides along as `data-plan-audio-word` so a
     * word with no clip can still be spoken by the browser's Hebrew voice.
     * @param {*} audioPath
     * @param {*} [wordText]
     * @returns {string}
     */
    function audioControl(audioPath, wordText) {
        var ready = typeof audioPath === 'string' && (AUDIO_FILE.test(audioPath) || WORD_CLIP.test(audioPath));
        var state = ready ? 'ready' : 'none';
        var clip = typeof audioPath === 'string' ? WORD_CLIP.exec(audioPath) : null;
        var spoken = typeof wordText === 'string' ? wordText.trim() : '';
        var extra = (clip ? ' data-plan-audio-clip="' + text(clip[1]) + '"' : '')
            + (spoken && spoken.length <= 60 ? ' data-plan-audio-word="' + text(spoken) + '"' : '');
        return '<button type="button" class="audio primary" data-plan-audio="' + state + '"' + extra + '>'
            // Brand sprite icon (INIT-005/SPEC-010). The visible word is the control's name.
            + '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">'
            + '<use href="/shared/brand/sprite.svg#icon-sound"></use>'
            + '</svg>השמעה</button>'
            + '<p class="gloss" data-plan-audio-note></p>';
    }

    /**
     * Picture frame. Empty alt renders no `img`. `mediaId` is not a URL.
     * @param {*} image
     * @returns {string}
     */
    function picture(image) {
        if (!image || typeof image !== 'object') return '';
        var alt = typeof image.alt === 'string' ? image.alt.trim() : '';
        if (!alt) {
            return '<p class="meaning" data-image-refused="missing-alt">Picture refused: missing alt text.</p>';
        }
        var id = typeof image.mediaId === 'string' ? image.mediaId : '';
        if (!MEDIA_ID.test(id)) {
            return '<p class="meaning" data-image-refused="broken-media">Picture refused: broken media reference.</p>';
        }
        return '<figure class="placeholder"><img src="/api/media/' + text(id) + '" alt="' + text(alt) + '"></figure>';
    }

    /**
     * @param {object|null|undefined} plan
     * @returns {object[]}
     */
    function unitsOf(plan) {
        return plan && Array.isArray(plan.units) ? plan.units : [];
    }

    /**
     * @param {object|null|undefined} plan
     * @returns {object}
     */
    function firstUnit(plan) {
        var units = unitsOf(plan);
        return units[0] || { lessons: [], title: '', theme: '' };
    }

    /**
     * @param {object|null|undefined} plan
     * @param {function(object): boolean} pred
     * @returns {{ unit: object, lesson: object, block: object }|null}
     */
    function findBlock(plan, pred) {
        var units = unitsOf(plan);
        for (var u = 0; u < units.length; u++) {
            var lessons = units[u] && Array.isArray(units[u].lessons) ? units[u].lessons : [];
            for (var i = 0; i < lessons.length; i++) {
                var blocks = lessons[i] && Array.isArray(lessons[i].blocks) ? lessons[i].blocks : [];
                for (var b = 0; b < blocks.length; b++) {
                    if (blocks[b] && pred(blocks[b])) {
                        return { unit: units[u], lesson: lessons[i], block: blocks[b] };
                    }
                }
            }
        }
        return null;
    }

    /**
     * First introduction word that has an image, else the first word.
     * @param {object|null|undefined} plan
     * @returns {{ unit: object, lesson: object, block: object, word: object }|null}
     */
    function findMeet(plan) {
        var found = findBlock(plan, function (block) {
            return block.type === 'introduction' && Array.isArray(block.words) && block.words.length > 0;
        });
        if (!found) return null;
        var words = found.block.words;
        var word = null;
        for (var i = 0; i < words.length; i++) {
            if (words[i] && words[i].image) {
                word = words[i];
                break;
            }
        }
        if (!word) word = words[0];
        if (!word) return null;
        return { unit: found.unit, lesson: found.lesson, block: found.block, word: word };
    }

    /**
     * @param {object|null|undefined} plan
     * @returns {{ unit: object, lesson: object, block: object }|null}
     */
    function findCheckpoint(plan) {
        return findBlock(plan, function (block) {
            return block.type === 'spaced-review-checkpoint';
        });
    }

    /**
     * @param {object|null|undefined} block
     * @param {{ word?: object, wordIndex?: number }|undefined} options
     * @returns {object|null}
     */
    function pickWord(block, options) {
        if (options && options.word && typeof options.word === 'object') return options.word;
        var words = block && Array.isArray(block.words) ? block.words : [];
        if (options && Number.isInteger(options.wordIndex) && words[options.wordIndex]) {
            return words[options.wordIndex];
        }
        for (var i = 0; i < words.length; i++) {
            if (words[i] && words[i].image) return words[i];
        }
        return words[0] || null;
    }

    /**
     * @param {*} value
     * @returns {number|null}
     */
    function safeSeconds(value) {
        if (typeof value !== 'number' || !Number.isFinite(value)) return null;
        var n = Math.trunc(value);
        if (n < 1 || n > 600) return null;
        return n;
    }

    /**
     * Words to list, taken from the plan's `wordKeys` and `recycles` only.
     * @param {object|null|undefined} block
     * @param {object|null|undefined} lesson
     * @returns {{ word: string, fromLessonId: string }[]}
     */
    function checkpointRows(block, lesson) {
        var recycles = lesson && Array.isArray(lesson.recycles) ? lesson.recycles : [];
        var byWord = Object.create(null);
        recycles.forEach(function (item) {
            if (!item || typeof item !== 'object') return;
            var word = typeof item.word === 'string' ? item.word : '';
            if (!word || byWord[word]) return;
            byWord[word] = item;
        });
        var keys = block && Array.isArray(block.wordKeys) ? block.wordKeys : [];
        var rows = [];
        var seen = Object.create(null);
        keys.forEach(function (key) {
            if (typeof key !== 'string' || !key || seen[key]) return;
            seen[key] = true;
            var item = byWord[key];
            rows.push({
                word: key,
                fromLessonId: item && typeof item.fromLessonId === 'string' ? item.fromLessonId : ''
            });
        });
        recycles.forEach(function (item) {
            if (!item || typeof item !== 'object') return;
            var word = typeof item.word === 'string' ? item.word : '';
            if (!word || seen[word]) return;
            seen[word] = true;
            rows.push({
                word: word,
                fromLessonId: typeof item.fromLessonId === 'string' ? item.fromLessonId : ''
            });
        });
        return rows;
    }

    /**
     * Introduction cluster: one word, gloss, optional picture, audio, example.
     * @param {object} block
     * @param {object} [options]
     * @returns {string}
     */
    function renderIntroduction(block, options) {
        var word = pickWord(block, options) || {};
        var figure = word.image ? picture(word.image) : '';
        var example = word.sentence
            ? '<p class="example" lang="he">' + text(word.sentence) + '</p>'
            : '';
        return '<section data-block-type="introduction" data-block-id="' + text(block && block.id) + '">'
            + '<div class="cluster">'
            + figure
            + '<p class="word" lang="he">' + text(word.word) + '</p>'
            + '<p class="gloss">' + gloss(word.meaning) + '</p>'
            + audioControl(word.audioPath, word.word)
            + '</div>'
            + example
            + dock('#recall', 'אפשר להמשיך')
            + '</section>';
    }

    /**
     * Recall state. No picture. The correction holds the correct form and starts hidden.
     * @param {object} [word]
     * @returns {string}
     */
    function renderRecall(word) {
        word = word || {};
        return '<article class="screen" id="recall" data-screen="recall" data-plan-answer="' + text(word.word) + '">'
            + '<p class="kicker">אחרי ניסיון</p>'
            + '<h2>מה המילה ל־' + gloss(word.meaning) + '&#x200F;?</h2>'
            + '<label class="field" for="recall-input">התשובה שלך'
            + '<input id="recall-input" name="attempt" type="text" autocomplete="off" lang="he" dir="rtl">'
            + '</label>'
            + '<p class="dock"><button type="button" class="primary" id="recall-check">בדיקה</button></p>'
            + '<p id="recall-attempt" class="attempt" hidden>התשובה שלך נשארת על המסך: '
            + '<span id="recall-kept" class="pointed" lang="he"></span>.</p>'
            + '<p id="recall-ok" hidden>נכון. המילה נשארת על המסך.</p>'
            + '<div id="recall-correction" class="correction" hidden>'
            + '<p>זו לא הצורה הנכונה. אחרי ניסיון אחד מוצגת המילה, והיא נשארת כאן.</p>'
            + '<p class="word" lang="he">' + text(word.word) + '</p>'
            + '<p class="gloss">' + gloss(word.meaning) + '</p>'
            + '</div>'
            + dock('#checkpoint', 'אפשר להמשיך')
            + '</article>';
    }

    /**
     * @param {object} block
     * @returns {string}
     */
    function renderGradedStory(block) {
        var paragraphs = block && Array.isArray(block.paragraphs) ? block.paragraphs : [];
        var paragraph = paragraphs[0] || {};
        var questions = block && Array.isArray(block.questions) ? block.questions : [];
        var question = questions[0] && questions[0].question
            ? '<p class="gloss">' + gloss(questions[0].question) + '</p>'
            : '';
        return '<section data-block-type="graded-story" data-block-id="' + text(block && block.id) + '">'
            + '<p class="pointed-line" lang="he">' + text(paragraph.text) + '</p>'
            + question
            + audioControl(paragraph.audioPath)
            + dock('#checkpoint', 'אפשר להמשיך')
            + '</section>';
    }

    /**
     * @param {object} block
     * @returns {string}
     */
    function renderListening(block) {
        return '<section data-block-type="listening" data-block-id="' + text(block && block.id) + '">'
            + '<p class="pointed-line" lang="he">' + text(block && block.passage) + '</p>'
            + audioControl(block && block.audioPath)
            + dock('#checkpoint', 'אפשר להמשיך')
            + '</section>';
    }

    /**
     * @param {object} block
     * @returns {string}
     */
    function renderShortDialogue(block) {
        var lines = block && Array.isArray(block.lines) ? block.lines : [];
        var items = lines.map(function (line) {
            line = line || {};
            var speaker = line.speaker === 'learner' || line.speaker === 'prompt' ? line.speaker : '';
            var who = speaker
                ? '<p class="kicker"><span lang="en" dir="ltr">' + speaker + '</span></p>'
                : '';
            var lineGloss = line.gloss ? '<p class="gloss">' + gloss(line.gloss) + '</p>' : '';
            return '<li>' + who
                + '<p class="pointed" lang="he">' + text(line.pointed) + '</p>'
                + lineGloss
                + '</li>';
        }).join('\n');
        var intro = block && block.introEn ? '<p class="gloss">' + gloss(block.introEn) + '</p>' : '';
        var clip = lines[0] && lines[0].audioPath;
        return '<section data-block-type="short-dialogue" data-block-id="' + text(block && block.id) + '">'
            + intro
            + '<ul class="recycle">' + items + '</ul>'
            + audioControl(clip)
            + dock('#checkpoint', 'אפשר להמשיך')
            + '</section>';
    }

    /**
     * @param {object} block
     * @returns {string}
     */
    function renderFluencyPass(block) {
        var seconds = safeSeconds(block && block.seconds);
        var cues = block && Array.isArray(block.cues) ? block.cues : [];
        var items = cues.map(function (cue) {
            return '<li><span class="pointed" lang="he">' + text(cue) + '</span></li>';
        }).join('\n');
        var time = seconds == null ? '' : '<p class="due-line">' + String(seconds) + ' שניות.</p>';
        return '<section data-block-type="fluency-pass" data-block-id="' + text(block && block.id) + '">'
            + time
            + '<ul class="recycle">' + items + '</ul>'
            + dock('#checkpoint', 'אפשר להמשיך')
            + '</section>';
    }

    /**
     * @param {object} block
     * @param {{ lesson?: object }} [options]
     * @returns {string}
     */
    function renderCheckpoint(block, options) {
        var lesson = options && options.lesson;
        var rows = checkpointRows(block, lesson);
        var fromLessons = block && Array.isArray(block.fromLessons) ? block.fromLessons : [];
        var fromText = fromLessons.map(function (id) {
            return id == null ? '' : String(id);
        }).join(', ');
        var items = rows.map(function (row) {
            var fromAttr = row.fromLessonId
                ? ' data-from-lesson="' + text(row.fromLessonId) + '"'
                : '';
            var fromGloss = row.fromLessonId ? gloss(row.fromLessonId) : '';
            return '<li' + fromAttr + ' data-word-key="' + text(row.word) + '">'
                + '<span class="pointed" lang="he">' + text(row.word) + '</span>'
                + fromGloss
                + '</li>';
        }).join('\n');
        return '<section data-block-type="spaced-review-checkpoint" data-block-id="' + text(block && block.id) + '">'
            + '<p class="gloss">' + gloss(fromText) + '</p>'
            + '<ul class="recycle">' + items + '</ul>'
            + '</section>';
    }

    /**
     * @param {object} block
     * @param {object} [options]
     * @returns {string}
     */
    function renderBlock(block, options) {
        if (!block || typeof block !== 'object') return '';
        if (block.type === 'introduction') return renderIntroduction(block, options);
        if (block.type === 'graded-story') return renderGradedStory(block);
        if (block.type === 'listening') return renderListening(block);
        if (block.type === 'short-dialogue') return renderShortDialogue(block);
        if (block.type === 'fluency-pass') return renderFluencyPass(block);
        if (block.type === 'spaced-review-checkpoint') return renderCheckpoint(block, options);
        var mod = REGISTRY[block.type];
        if (mod && typeof mod.render === 'function') return mod.render(block, options || {});
        return '';
    }

    /**
     * @param {object} lesson
     * @param {string} href
     * @returns {string}
     */
    function lessonCard(lesson, href) {
        lesson = lesson || {};
        var fresh = Array.isArray(lesson.newWords)
            ? lesson.newWords.map(function (word) { return word == null ? '' : String(word); }).join(' · ')
            : '';
        var older = Array.isArray(lesson.recycles) ? lesson.recycles.length : 0;
        var builds = Array.isArray(lesson.buildsOn)
            ? lesson.buildsOn.map(function (id) { return id == null ? '' : String(id); }).join(', ')
            : '';
        var sub = lesson.subtitle ? subtitleLine(lesson.subtitle) : '';
        var recipe = lesson && typeof lesson.recipe === 'string' ? lesson.recipe : '';
        var chipPair = Object.prototype.hasOwnProperty.call(RECIPE_CHIP, recipe) ? RECIPE_CHIP[recipe] : null;
        var chip = chipPair
            ? '<p class="kicker"><span class="recipe-chip" data-recipe="' + recipe + '">'
                + chipPair[0] + ' <span lang="en" dir="ltr">' + chipPair[1] + '</span></span></p>'
            : '';
        return '<li>' + chip + '<p class="kicker"><a href="' + href + '">' + text(lesson.title) + '</a></p>'
            + sub
            + '<p class="gloss">' + gloss(fresh || 'no new words') + '</p>'
            + '<p class="gloss">' + gloss(older + ' recycled · builds on ' + (builds || 'nothing earlier')) + '</p>'
            + '</li>';
    }

    /**
     * @param {object} lesson
     * @param {object|null} meet
     * @param {object|null} checkpoint
     * @returns {string}
     */
    function lessonHref(lesson, meet, checkpoint) {
        var id = lesson && lesson.id;
        if (meet && meet.lesson && id === meet.lesson.id) return '#introduction';
        if (checkpoint && checkpoint.lesson && id === checkpoint.lesson.id) return '#checkpoint';
        return '#overview';
    }

    /**
     * @param {object|null|undefined} plan
     * @param {object|null} meet
     * @param {object|null} checkpoint
     * @returns {string}
     */
    function renderToday(plan, meet, checkpoint) {
        var unit = firstUnit(plan);
        var lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
        var next = lessons[0] || {};
        var due = checkpoint && checkpoint.lesson && Array.isArray(checkpoint.lesson.recycles)
            ? checkpoint.lesson.recycles.length
            : 0;
        if (!Number.isFinite(due) || due < 0) due = 0;
        var subtitle = next.subtitle ? subtitleLine(next.subtitle) : '';
        return screen('today',
            mast('היום', plan && plan.title)
            + '<section class="block" aria-labelledby="next-heading">'
            + '<h2 id="next-heading">השיעור הבא בתוכנית</h2>'
            + '<p class="lesson" lang="he">' + text(next.title) + '</p>'
            + subtitle
            + '<p class="gloss">' + gloss(unit && unit.theme) + '</p>'
            + '</section>'
            + '<section class="block" aria-labelledby="due-heading">'
            + '<h2 id="due-heading">לחזרה</h2>'
            + '<p class="due-line">' + String(due) + ' מילים מהשיעורים הקודמים בנקודת הביקורת.</p>'
            + '</section>'
            + dock('#introduction', 'להתחיל את השיעור'));
    }

    /**
     * @param {object|null|undefined} plan
     * @param {object|null} meet
     * @param {object|null} checkpoint
     * @returns {string}
     */
    function renderOverview(plan, meet, checkpoint) {
        var cards = [];
        unitsOf(plan).forEach(function (unit) {
            var lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            lessons.forEach(function (lesson) {
                cards.push(lessonCard(lesson, lessonHref(lesson, meet, checkpoint)));
            });
        });
        var unit = firstUnit(plan);
        return screen('overview',
            mast('<a href="#today">היום</a>', unit && unit.title)
            + '<p class="gloss">' + gloss(plan && plan.subtitle) + '</p>'
            + '<ol class="lesson-list">' + cards.join('\n') + '</ol>');
    }

    /**
     * @param {object|null} meet
     * @returns {string}
     */
    function renderIntroductionScreen(meet) {
        var lessonTitle = meet && meet.lesson ? meet.lesson.title : '';
        var inner = meet
            ? renderIntroduction(meet.block, { word: meet.word })
            : renderIntroduction({ type: 'introduction', id: '', words: [] });
        return screen('introduction',
            mast('<a href="#today">היום</a> · היכרות', lessonTitle)
            + '<p class="kicker">היכרות · <span lang="en" dir="ltr">introduction</span></p>'
            + '<h2>פוגשים מילה</h2>'
            + inner);
    }

    /**
     * @param {object|null|undefined} plan
     * @param {string} type
     * @param {string} kicker
     * @returns {string}
     */
    function renderTypedScreen(plan, type, kicker) {
        var found = findBlock(plan, function (block) { return block.type === type; });
        if (!found) return '';
        return screen(type,
            mast('<a href="#today">היום</a> · ' + kicker, found.lesson && found.lesson.title)
            + renderBlock(found.block, { lesson: found.lesson }));
    }

    /**
     * @param {object|null} checkpoint
     * @returns {string}
     */
    function renderCheckpointScreen(checkpoint) {
        var lesson = checkpoint && checkpoint.lesson ? checkpoint.lesson : {};
        var block = checkpoint && checkpoint.block
            ? checkpoint.block
            : { type: 'spaced-review-checkpoint', id: '', wordKeys: [], fromLessons: [] };
        return screen('checkpoint',
            mast('<a href="#today">היום</a> · נקודת ביקורת', lesson.title)
            + '<h2>מילים מהשיעורים הקודמים</h2>'
            + renderCheckpoint(block, { lesson: lesson }));
    }

    /**
     * Screens for a published snapshot. No script tag.
     * @param {object|null|undefined} plan
     * @returns {string}
     */
    /**
     * Screens for registered blocks, including explainer, conjugation,
     * personal-question, verse-reading, and letter-intro.
     * @param {object|null|undefined} plan
     * @param {{ forms?: object, audio?: object }} [options]
     * @returns {string}
     */
    function renderProgram(plan, options) {
        var parts = [];
        var n = 0;
        unitsOf(plan).forEach(function (unit) {
            var lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            lessons.forEach(function (lesson) {
                var blocks = lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
                blocks.forEach(function (block) {
                    if (!block || !REGISTRY[block.type] || typeof REGISTRY[block.type].render !== 'function') return;
                    n += 1;
                    parts.push(screen('program-' + n,
                        mast('<a href="#today">היום</a>', lesson && lesson.title)
                        + renderBlock(block, {
                            lesson: lesson,
                            forms: options && options.forms,
                            lexicon: options && options.lexicon,
                            audio: options && options.audio,
                            vowelsOn: !options || options.vowelsOn !== false
                        })));
                });
            });
        });
        return parts.join('\n');
    }

    /**
     * @param {object|null|undefined} plan
     * @param {{ forms?: object, audio?: object }} [options]
     * @returns {string}
     */
    function renderScreens(plan, options) {
        var meet = findMeet(plan);
        var checkpoint = findCheckpoint(plan);
        var word = meet ? meet.word : {};
        return [
            renderToday(plan, meet, checkpoint),
            renderOverview(plan, meet, checkpoint),
            renderIntroductionScreen(meet),
            renderRecall(word),
            renderTypedScreen(plan, 'graded-story', 'סיפור'),
            renderTypedScreen(plan, 'listening', 'האזנה'),
            renderTypedScreen(plan, 'short-dialogue', 'שיחה'),
            renderTypedScreen(plan, 'fluency-pass', 'שטף'),
            renderCheckpointScreen(checkpoint),
            renderProgram(plan, options)
        ].filter(Boolean).join('\n');
    }

    /**
     * Fixture document. One script element, whose `src` is not taken from the plan.
     * @param {object|null|undefined} plan
     * @returns {string}
     */
    function renderPage(plan) {
        return [
            '<!DOCTYPE html>',
            '<html lang="he" dir="rtl">',
            '<head>',
            '    <meta charset="utf-8">',
            '    <meta name="viewport" content="width=device-width, initial-scale=1">',
            '    <title>' + text(plan && plan.title) + '</title>',
            '    <link rel="stylesheet" href="/shared/worksheet.css">',
            '</head>',
            '<body class="plan-page">',
            '<div class="page">',
            renderScreens(plan),
            '</div>',
            '<script src="' + SCRIPT_SRC + '"></script>',
            '</body>',
            '</html>',
            ''
        ].join('\n');
    }

    /**
     * @param {string} str
     * @returns {string}
     */
    function normalizeAnswer(str) {
        return String(str || '')
            .replace(/[\u0591-\u05C7]/g, '')
            .replace(/[.,!?;:"'״׳\-–—()]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
    }

    /**
     * Empty attempt does not reveal the word. A miss keeps the attempt and reveals the form.
     * @param {*} attempt
     * @param {*} expected
     * @returns {{ attempted: boolean, reveal: boolean, correct: boolean, keep: string }}
     */
    function decideRecall(attempt, expected) {
        var keep = String(attempt == null ? '' : attempt);
        if (!keep.trim()) {
            return { attempted: false, reveal: false, correct: false, keep: keep };
        }
        var correct = normalizeAnswer(keep) === normalizeAnswer(expected);
        return { attempted: true, reveal: !correct, correct: correct, keep: keep };
    }

    /**
     * @param {*} value
     * @returns {string}
     */
    function planId(value) {
        var id = value == null ? '' : String(value);
        if (!id || id.length > 64 || !SLUG.test(id)) return '';
        return id;
    }

    /**
     * @param {string} id Already checked with `planId`.
     * @returns {string}
     */
    function publishedPath(id) {
        return '/api/plans/' + encodeURIComponent(id) + '/published';
    }

    /**
     * @param {Document} doc
     * @returns {string}
     */
    function planIdFromLocation(doc) {
        var search = doc && doc.location && typeof doc.location.search === 'string' ? doc.location.search : '';
        var params;
        try {
            params = new URLSearchParams(search);
        } catch (err) {
            return '';
        }
        return planId(params.get('id'));
    }

    /**
     * @param {*} id
     * @param {function(string, object): Promise<{ok: boolean, json: function(): Promise<object>}>} [fetchImpl]
     * @returns {Promise<object>}
     */
    function loadPublished(id, fetchImpl) {
        var safe = planId(id);
        if (!safe) return Promise.reject(new Error('bad id'));
        var go = typeof fetchImpl === 'function' ? fetchImpl : fetch;
        return Promise.resolve(go(publishedPath(safe), {
            credentials: 'same-origin',
            headers: { Accept: 'application/json' }
        })).then(function (res) {
            if (!res || res.ok !== true || typeof res.json !== 'function') {
                return Promise.reject(new Error('not found'));
            }
            return res.json();
        });
    }

    var player = null;

    /**
     * Play a word: its saved clip first, then the browser's Hebrew voice. Resolves true
     * when something started, false when nothing could play.
     * @param {string|null} clip File name of a shared word clip (no folder, no extension).
     * @param {string|null} word The word, for the browser voice.
     * @returns {Promise<boolean>}
     */
    function playWord(clip, word) {
        function speak() {
            var Utterance = root.SpeechSynthesisUtterance;
            var synth = root.speechSynthesis;
            if (!word || typeof Utterance !== 'function' || !synth || typeof synth.speak !== 'function') {
                return false;
            }
            try {
                var utterance = new Utterance(word);
                utterance.lang = 'he-IL';
                utterance.rate = 0.85;
                if (typeof synth.cancel === 'function') synth.cancel();
                synth.speak(utterance);
                return true;
            } catch (err) {
                return false;
            }
        }
        if (clip && /^[A-Za-z0-9_-]{8,64}$/.test(clip) && typeof root.Audio === 'function') {
            try {
                if (player && typeof player.pause === 'function') player.pause();
                player = new root.Audio(WORD_CLIP_BASE + clip + '.mp3');
                return Promise.resolve(player.play()).then(function () {
                    return true;
                }, function () {
                    return speak();
                });
            } catch (err) {
                return Promise.resolve(speak());
            }
        }
        return Promise.resolve(speak());
    }

    /**
     * @param {Document} doc
     * @returns {void}
     */
    function bind(doc) {
        if (!doc || typeof doc.getElementById !== 'function') return;
        var check = doc.getElementById('recall-check');
        if (check && check.getAttribute('data-plan-bound') !== '1' && typeof check.addEventListener === 'function') {
            check.setAttribute('data-plan-bound', '1');
            check.addEventListener('click', function () {
                var input = doc.getElementById('recall-input');
                var section = doc.getElementById('recall');
                var decision = decideRecall(
                    input ? input.value : '',
                    section ? (section.getAttribute('data-plan-answer') || '') : ''
                );
                if (!decision.attempted) return;
                var kept = doc.getElementById('recall-kept');
                if (kept) kept.textContent = decision.keep;
                var attempt = doc.getElementById('recall-attempt');
                if (attempt) attempt.hidden = false;
                var correction = doc.getElementById('recall-correction');
                if (correction) correction.hidden = !decision.reveal;
                var ok = doc.getElementById('recall-ok');
                if (ok) ok.hidden = !decision.correct;
            });
        }
        if (typeof doc.querySelectorAll !== 'function') return;
        var buttons = doc.querySelectorAll('[data-plan-audio]');
        for (var i = 0; i < buttons.length; i++) {
            var button = buttons[i];
            if (!button || button.getAttribute('data-plan-bound') === '1') continue;
            if (typeof button.addEventListener !== 'function') continue;
            button.setAttribute('data-plan-bound', '1');
            button.addEventListener('click', function (event) {
                var target = event.currentTarget;
                var note = target.parentNode && typeof target.parentNode.querySelector === 'function'
                    ? target.parentNode.querySelector('[data-plan-audio-note]')
                    : null;
                if (!note) return;
                var clip = target.getAttribute('data-plan-audio-clip');
                var spoken = target.getAttribute('data-plan-audio-word');
                note.textContent = '';
                playWord(clip, spoken).then(function (played) {
                    if (played) return;
                    note.textContent = target.getAttribute('data-plan-audio') !== 'ready'
                        ? 'אין הקלטה בתוכנית.'
                        : 'ההקלטה לא מתנגנת מהדף הזה.';
                });
            });
        }
        if (REGISTRY['conjugation-table'] && typeof REGISTRY['conjugation-table'].bind === 'function') {
            REGISTRY['conjugation-table'].bind(doc, live);
        }
        if (REGISTRY['personal-question'] && typeof REGISTRY['personal-question'].bind === 'function') {
            REGISTRY['personal-question'].bind(doc, live);
        }
        if (REGISTRY['verse-reading'] && typeof REGISTRY['verse-reading'].bind === 'function') {
            REGISTRY['verse-reading'].bind(doc);
        }
        if (REGISTRY['letter-intro'] && typeof REGISTRY['letter-intro'].bind === 'function') {
            REGISTRY['letter-intro'].bind(doc);
        }
        if (REGISTRY.exercise && typeof REGISTRY.exercise.bind === 'function') {
            REGISTRY.exercise.bind(doc, live);
        }
        if (REGISTRY.assignment && typeof REGISTRY.assignment.bind === 'function') {
            REGISTRY.assignment.bind(doc, live);
        }
    }

    /**
     * A lexicon key safe to put in a URL path. Not plan HTML.
     * @param {*} value
     * @returns {string}
     */
    function safeLexiconKey(value) {
        var key = String(value == null ? '' : value).normalize('NFC').trim();
        if (!key || key.length > 60) return '';
        if (!/[\u05D0-\u05EA]/.test(key)) return '';
        if (/[\u0000-\u001F<>]/.test(key)) return '';
        return key;
    }

    /**
     * @param {object|null|undefined} plan
     * @returns {string[]}
     */
    function conjugationLemmas(plan) {
        var keys = [];
        var seen = Object.create(null);
        unitsOf(plan).forEach(function (unit) {
            var lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            lessons.forEach(function (lesson) {
                var blocks = lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
                blocks.forEach(function (block) {
                    if (!block || block.type !== 'conjugation-table') return;
                    var key = safeLexiconKey(block.lemmaKey);
                    if (!key || seen[key]) return;
                    seen[key] = true;
                    keys.push(key);
                });
            });
        });
        return keys;
    }

    /**
     * @param {Array<{ key: string, body: object }>|null} rows
     * @returns {object}
     */
    function indexForms(rows) {
        var map = Object.create(null);
        (rows || []).forEach(function (row) {
            if (!row || !row.body || typeof row.body !== 'object') return;
            var entry = row.body.entry;
            var lemmaRoot = entry && typeof entry.root === 'string' ? entry.root : '';
            var paradigm = row.body.paradigm || {};
            Object.keys(paradigm).forEach(function (tense) {
                var cells = paradigm[tense] || {};
                Object.keys(cells).forEach(function (cell) {
                    var form = cells[cell];
                    if (!form || typeof form !== 'object' || typeof form.key !== 'string') return;
                    if (!form.root && lemmaRoot) form.root = lemmaRoot;
                    map[form.key] = form;
                });
            });
        });
        return map;
    }

    /**
     * @param {object} plan
     * @param {Function} [fetchImpl]
     * @returns {Promise<object>}
     */
    function loadForms(plan, fetchImpl) {
        var keys = conjugationLemmas(plan);
        if (!keys.length) return Promise.resolve(Object.create(null));
        var go = typeof fetchImpl === 'function' ? fetchImpl : fetch;
        return Promise.all(keys.map(function (key) {
            return Promise.resolve(go('/api/lexicon/' + encodeURIComponent(key), {
                credentials: 'same-origin',
                headers: { Accept: 'application/json' }
            })).then(function (res) {
                if (!res || res.ok !== true || typeof res.json !== 'function') return null;
                return res.json().then(function (body) {
                    return { key: key, body: body };
                });
            }).catch(function () {
                return null;
            });
        })).then(indexForms);
    }

    /**
     * @param {object} plan
     * @returns {string}
     */
    /**
     * Pointed keys on verse-reading blocks, capped, safe to put in a lexicon URL.
     * @param {object|null|undefined} plan
     * @returns {string[]}
     */
    function verseLexiconKeys(plan) {
        var keys = [];
        var seen = Object.create(null);
        function add(value) {
            var key = safeLexiconKey(value);
            if (!key || seen[key] || keys.length >= 24) return;
            seen[key] = true;
            keys.push(key);
        }
        unitsOf(plan).forEach(function (unit) {
            var lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            lessons.forEach(function (lesson) {
                var blocks = lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
                blocks.forEach(function (block) {
                    if (!block || block.type !== 'verse-reading') return;
                    var words = Array.isArray(block.newWords) ? block.newWords : [];
                    words.forEach(add);
                    String(block.verse || '').split(/\s+/).forEach(add);
                });
            });
        });
        return keys;
    }

    /**
     * Gloss map for verse tap-to-gloss. A missing row is omitted.
     * @param {object} plan
     * @param {Function} [fetchImpl]
     * @returns {Promise<object>}
     */
    function loadGlosses(plan, fetchImpl) {
        var keys = verseLexiconKeys(plan);
        if (!keys.length) return Promise.resolve(Object.create(null));
        var go = typeof fetchImpl === 'function' ? fetchImpl : fetch;
        return Promise.all(keys.map(function (key) {
            return Promise.resolve(go('/api/lexicon/' + encodeURIComponent(key), {
                credentials: 'same-origin',
                headers: { Accept: 'application/json' }
            })).then(function (res) {
                if (!res || res.ok !== true || typeof res.json !== 'function') return null;
                return res.json().then(function (body) {
                    var entry = body && body.entry;
                    var gloss = entry && typeof entry.gloss_en === 'string' ? entry.gloss_en : '';
                    if (!gloss) return null;
                    return { key: key, gloss_en: gloss };
                });
            }).catch(function () {
                return null;
            });
        })).then(function (rows) {
            var map = Object.create(null);
            (rows || []).forEach(function (row) {
                if (!row || !row.key) return;
                map[row.key] = { gloss_en: row.gloss_en };
            });
            return map;
        });
    }

    /**
     * @param {object} plan
     * @returns {string}
     */
    function firstLessonSlug(plan) {
        var found = '';
        unitsOf(plan).forEach(function (unit) {
            if (found) return;
            var lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            lessons.forEach(function (lesson) {
                if (found || !lesson || typeof lesson.id !== 'string') return;
                if (/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(lesson.id)) found = lesson.id;
            });
        });
        return found;
    }

    /**
     * @param {object} plan
     * @returns {Promise<void>}
     */
    function bootSync(doc, plan) {
        var href = doc && doc.location && typeof doc.location.href === 'string' ? doc.location.href : '';
        if (href.indexOf('http') !== 0) return Promise.resolve();
        var Sync = root.HebrewSync;
        if (!Sync || typeof Sync.init !== 'function') return Promise.resolve();
        if (typeof Sync.isActive === 'function' && Sync.isActive()) return Promise.resolve();
        var slug = firstLessonSlug(plan);
        return Promise.resolve(Sync.init({ lessonSlug: slug || null })).then(function () {
            return undefined;
        }).catch(function () {
            return undefined;
        });
    }

    /**
     * Signed-in entry. Paints only `renderScreens` output. Errors use textContent.
     * @param {Document} doc
     * @param {Function} [fetchImpl]
     * @returns {Promise<void>}
     */
    function start(doc, fetchImpl) {
        bind(doc);
        var mount = doc.getElementById('plan-root');
        if (!mount) return Promise.resolve();
        var id = planIdFromLocation(doc);
        if (!id) {
            mount.textContent = 'חסר מזהה תוכנית.';
            return Promise.resolve();
        }
        return loadPublished(id, fetchImpl).then(function (plan) {
            if (!plan || typeof plan !== 'object' || Array.isArray(plan)) {
                throw new Error('bad plan');
            }
            return Promise.all([
                loadForms(plan, fetchImpl),
                loadGlosses(plan, fetchImpl)
            ]).then(function (loaded) {
                live.plan = plan;
                live.forms = loaded[0] || Object.create(null);
                live.lexicon = loaded[1] || Object.create(null);
                live.audio = root.HEBREW_WORD_AUDIO || null;
                mount.innerHTML = renderScreens(plan, {
                    forms: live.forms,
                    lexicon: live.lexicon,
                    audio: live.audio,
                    vowelsOn: true
                });
                bind(doc);
                return bootSync(doc, plan);
            });
        }).then(function () {
            if (mount && typeof mount.setAttribute === 'function') {
                mount.setAttribute('data-plan-ready', '1');
            }
        }).catch(function () {
            mount.textContent = 'התוכנית לא נמצאה.';
        });
    }

    /**
     * @param {Document} doc
     * @returns {void}
     */
    function boot(doc) {
        if (!doc || typeof doc.getElementById !== 'function') return;
        if (doc.readyState === 'loading' && typeof doc.addEventListener === 'function') {
            doc.addEventListener('DOMContentLoaded', function () { start(doc); });
            return;
        }
        start(doc);
    }

    return {
        AUDIO_FILE: AUDIO_FILE,
        MEDIA_ID: MEDIA_ID,
        SCRIPT_SRC: SCRIPT_SRC,
        TEXT_LIMIT: TEXT_LIMIT,
        bind: bind,
        blocks: REGISTRY,
        boot: boot,
        decideRecall: decideRecall,
        escapeHtml: escapeHtml,
        playWord: playWord,
        findCheckpoint: findCheckpoint,
        findMeet: findMeet,
        lessonCard: lessonCard,
        loadPublished: loadPublished,
        normalizeAnswer: normalizeAnswer,
        picture: picture,
        planId: planId,
        publishedPath: publishedPath,
        renderBlock: renderBlock,
        renderPage: renderPage,
        renderScreens: renderScreens,
        start: start,
        text: text
    };
});
