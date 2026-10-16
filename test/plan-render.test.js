/**
 * INIT-004/SPEC-014 — Today, plan overview, and the six block renderers.
 * Does not import spikes/skeleton. The café plan is the ADR-0003 sample.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const PLAN_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json');
const CSS_PATH = path.join(ROOT, 'shared', 'worksheet.css');
const TOKENS_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0006-assets', 'tokens.json');
const SRC_PATH = path.join(ROOT, 'shared', 'ui', 'plan-render.js');
const CSS_START = '/* === INIT-004/SPEC-014 plan screens === */';
const CSS_END = '/* === end INIT-004/SPEC-014 plan screens === */';
const PRIVACY = 'מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה. (Admins can see all your activity, including every answer.)';
const SCRIPT_PAYLOAD = '<script>alert(1)</script>';
const HANDLER_PAYLOAD = '<img src=x onerror=alert(1)><div onclick=alert(1)>';

const ui = require('../shared/ui/plan-render');

function loadPlan() {
    return JSON.parse(fs.readFileSync(PLAN_PATH, 'utf8'));
}

function coffee(plan) {
    return plan.units[0].lessons[0].blocks[0].words[1];
}

function checkpointLesson(plan) {
    return plan.units[0].lessons[2];
}

function section(html, name, until) {
    const parts = html.split('data-screen="' + name + '"');
    assert.equal(parts.length >= 2, true, name);
    const rest = parts[1];
    if (!until) return rest;
    return rest.split('data-screen="' + until + '"')[0];
}

function assertCleanTags(html) {
    const tags = html.match(/<\/?[a-zA-Z][^>]*>/g) || [];
    tags.forEach((tag) => {
        const body = tag.replace(/^<\/?[a-zA-Z][\w:-]*/, '').replace(/>$/, '');
        const names = [];
        const re = /([:@\w-]+)(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?/g;
        let match;
        while ((match = re.exec(body))) names.push(match[1].toLowerCase());
        names.forEach((name) => {
            assert.equal(/^on/.test(name), false, tag);
        });
    });
}

function lin(channel) {
    const s = channel / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
    const h = hex.replace('#', '');
    const r = parseInt(h.slice(0, 2), 16);
    const g = parseInt(h.slice(2, 4), 16);
    const b = parseInt(h.slice(4, 6), 16);
    return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a, b) {
    const hi = Math.max(luminance(a), luminance(b));
    const lo = Math.min(luminance(a), luminance(b));
    return (hi + 0.05) / (lo + 0.05);
}

function cssBlock() {
    const css = fs.readFileSync(CSS_PATH, 'utf8');
    const start = css.indexOf(CSS_START);
    const end = css.indexOf(CSS_END);
    assert.ok(start > css.indexOf('INIT-004/SPEC-013'), 'plan rules follow the token block');
    assert.ok(end > start);
    return css.slice(start, end + CSS_END.length);
}

describe('beginner-unit-1 fixture', () => {
    const plan = loadPlan();
    const html = ui.renderPage(plan);

    it('renders Today, the overview, an introduction, recall, and the checkpoint', () => {
        assert.match(html, /<html lang="he" dir="rtl">/);
        ['today', 'overview', 'introduction', 'recall', 'checkpoint'].forEach((name) => {
            assert.ok(html.includes('data-screen="' + name + '"'), name);
        });
        const today = section(html, 'today', 'overview');
        assert.match(today, /השיעור הבא בתוכנית/);
        assert.match(today, /בבית הקפה 1 — מה רוצים/);
        assert.match(today, /13 מילים מהשיעורים הקודמים בנקודת הביקורת/);
        assert.match(today, /href="#introduction"/);
        assert.doesNotMatch(today, /streak|leaderboard|\bpoints\b|נקודות|רצף/);

        const overview = section(html, 'overview', 'introduction');
        plan.units[0].lessons.forEach((lesson) => {
            assert.ok(overview.includes(lesson.title), lesson.id);
        });

        const intro = section(html, 'introduction', 'recall');
        assert.match(intro, /קָפֶה/);
        assert.match(intro, /lang="en" dir="ltr"/);
        assert.match(intro, /alt="coffee"/);
        assert.match(intro, /src="\/api\/media\/coffee-cup"/);
        assert.match(intro, /class="audio primary"/);
        assert.match(intro, /data-plan-audio="none"/);
        assert.equal(/autoplay/i.test(intro), false);

        const recall = section(html, 'recall', 'graded-story');
        assert.equal(/<img\b/i.test(recall), false);
        assert.match(recall, /id="recall-correction"[^>]*hidden/);
        assert.match(recall, /id="recall-attempt"[^>]*hidden/);
        assert.match(recall, /קָפֶה/);

        const checkpoint = section(html, 'checkpoint', null);
        const lesson = checkpointLesson(plan);
        assert.equal((checkpoint.match(/data-from-lesson="/g) || []).length, lesson.recycles.length);
        lesson.recycles.forEach((item) => {
            assert.ok(checkpoint.includes(item.word), item.word);
            assert.ok(checkpoint.includes('data-from-lesson="' + item.fromLessonId + '"'), item.fromLessonId);
        });
        lesson.blocks.find((block) => block.type === 'spaced-review-checkpoint').fromLessons.forEach((id) => {
            assert.ok(checkpoint.includes(id), id);
        });
        assert.equal(checkpoint.includes('שָׁלוֹם'), false);
        assert.equal((html.match(/<script/gi) || []).length, 1);
        assert.match(html, /<script src="\/shared\/ui\/plan-render\.js"><\/script>/);
        assert.equal(/\son[a-z]+\s*=/i.test(html), false);
        assert.equal(html.includes('data-action="set-dir"'), false);
    });
});

describe('exercise block registry (INIT-012/SPEC-004)', () => {
    it('loads HebrewBlockExercise in the plan renderer registry', () => {
        const mod = ui.blocks.exercise;
        assert.ok(mod, 'exercise module');
        assert.equal(typeof mod.render, 'function');
        assert.equal(typeof mod.bind, 'function');
    });
});

describe('six block renderers', () => {
    const blocks = {
        introduction: {
            id: 'intro',
            type: 'introduction',
            words: [{
                id: '1',
                word: 'קָפֶה',
                meaning: 'coffee',
                sentence: 'אֲנִי רוֹצֶה קָפֶה.',
                image: { mediaId: 'brand-new-photo', alt: 'a cup' }
            }]
        },
        'graded-story': {
            id: 'story',
            type: 'graded-story',
            paragraphs: [{ id: 'p1', text: 'אֲנִי רוֹצֶה קָפֶה.' }],
            questions: [{ id: 'q1', question: 'What does he want?' }],
            image: { mediaId: 'brand-new-photo', alt: 'a cup' }
        },
        listening: {
            id: 'listen',
            type: 'listening',
            passage: 'אֲנִי רוֹצֶה קָפֶה.',
            image: { mediaId: 'brand-new-photo', alt: 'a cup' }
        },
        'short-dialogue': {
            id: 'talk',
            type: 'short-dialogue',
            lines: [{ id: 'l1', speaker: 'learner', pointed: 'תּוֹדָה.', gloss: 'thank you' }],
            image: { mediaId: 'brand-new-photo', alt: 'a cup' }
        },
        'fluency-pass': {
            id: 'fluency',
            type: 'fluency-pass',
            seconds: 60,
            source: 'story',
            cues: ['קָפֶה', 'תּוֹדָה'],
            image: { mediaId: 'brand-new-photo', alt: 'a cup' }
        },
        'spaced-review-checkpoint': {
            id: 'check',
            type: 'spaced-review-checkpoint',
            wordKeys: ['קָפֶה'],
            fromLessons: ['cafe-order'],
            image: { mediaId: 'brand-new-photo', alt: 'a cup' }
        }
    };

    it('renders each type, and only an introduction with alt may emit an img', () => {
        Object.keys(blocks).forEach((type) => {
            const html = ui.renderBlock(blocks[type], {
                lesson: {
                    recycles: [{ word: 'קָפֶה', fromLessonId: 'cafe-order' }]
                }
            });
            assert.match(html, new RegExp('data-block-type="' + type + '"'), type);
            if (type === 'introduction') {
                assert.match(html, /<img\b/);
                assert.match(html, /src="\/api\/media\/brand-new-photo"/);
                assert.match(html, /alt="a cup"/);
            } else {
                assert.equal(/<img\b/i.test(html), false, type);
            }
        });
    });

    it('lists checkpoint words from wordKeys and recycles, and does not invent one', () => {
        const html = ui.renderBlock(blocks['spaced-review-checkpoint'], {
            lesson: {
                recycles: [
                    { word: 'קָפֶה', fromLessonId: 'cafe-order' },
                    { word: 'מַיִם', fromLessonId: 'cafe-order' }
                ]
            }
        });
        assert.match(html, /data-from-lesson="cafe-order"/);
        assert.ok(html.includes('קָפֶה'));
        assert.ok(html.includes('מַיִם'));
        assert.equal(html.includes('שָׁלוֹם'), false);
    });
});

describe('hostile plan text', () => {
    it('escapes a script payload, refuses an empty alt, and keeps one script src', () => {
        const plan = loadPlan();
        plan.title = SCRIPT_PAYLOAD;
        plan.subtitle = SCRIPT_PAYLOAD;
        plan.units[0].title = SCRIPT_PAYLOAD;
        plan.units[0].theme = SCRIPT_PAYLOAD;
        plan.units[0].lessons[0].title = SCRIPT_PAYLOAD;
        plan.units[0].lessons[0].subtitle = SCRIPT_PAYLOAD;
        const word = coffee(plan);
        word.word = SCRIPT_PAYLOAD;
        word.meaning = SCRIPT_PAYLOAD;
        word.sentence = SCRIPT_PAYLOAD;
        word.image.alt = '';
        word.image.mediaId = 'http://evil.example/a.png';
        plan.units[0].lessons[2].recycles[0].word = SCRIPT_PAYLOAD;
        plan.units[0].lessons[2].recycles[0].fromLessonId = 'cafe-order';

        const html = ui.renderPage(plan);
        assert.equal(html.includes(SCRIPT_PAYLOAD), false);
        assert.ok(html.includes(ui.escapeHtml(SCRIPT_PAYLOAD)));
        assert.equal((html.match(/<script/gi) || []).length, 1);
        assert.match(html, /<script src="\/shared\/ui\/plan-render\.js"><\/script>/);
        assert.equal(html.includes('onerror='), false);
        assert.equal(html.includes('onclick='), false);
        assert.equal(/<img\b/i.test(html), false);
        assert.equal(html.includes('evil.example'), false);
        assert.equal(html.includes('http://'), false);
        assert.equal(/javascript:/i.test(html), false);
        assert.match(html, /data-image-refused="missing-alt"/);
        assertCleanTags(html);
        assert.equal(ui.escapeHtml, globalThis.HebrewUtils.escapeHtml);
    });

    it('does not turn a handler string into an attribute', () => {
        const plan = loadPlan();
        plan.title = HANDLER_PAYLOAD;
        coffee(plan).word = HANDLER_PAYLOAD;
        coffee(plan).meaning = HANDLER_PAYLOAD;
        const html = ui.renderPage(plan);
        assert.equal(html.includes(HANDLER_PAYLOAD), false);
        assert.ok(html.includes(ui.escapeHtml(HANDLER_PAYLOAD)));
        assert.equal((html.match(/<script/gi) || []).length, 1);
        assertCleanTags(html);
        const imgs = html.match(/<img\b[^>]*>/gi) || [];
        assert.equal(imgs.length, 1);
        assert.match(imgs[0], /src="\/api\/media\/coffee-cup"/);
    });

    it('truncates text past the plan limit', () => {
        const long = 'א'.repeat(ui.TEXT_LIMIT + 50);
        const html = ui.renderPage({ title: long, units: [] });
        assert.ok(html.includes('א'.repeat(ui.TEXT_LIMIT)));
        assert.equal(html.includes('א'.repeat(ui.TEXT_LIMIT + 1)), false);
    });

    it('refuses a picture whose media id is not a media id', () => {
        const html = ui.picture({ mediaId: 'javascript:alert(1)', alt: 'cup', src: 'http://evil.example/x.png' });
        assert.equal(/<img\b/i.test(html), false);
        assert.match(html, /data-image-refused="broken-media"/);
        assert.equal(html.includes('javascript:'), false);
        assert.equal(html.includes('evil.example'), false);
    });
});

describe('audio and recall', () => {
    it('does not request a clip when audioPath is missing, and does not copy a path into the page', () => {
        const missing = ui.renderBlock({ id: 'l', type: 'listening', passage: 'שלום' });
        assert.match(missing, /data-plan-audio="none"/);
        assert.equal(missing.includes('.mp3'), false);
        assert.equal(/<audio\b/i.test(missing), false);

        const present = ui.renderBlock({
            id: 'l',
            type: 'listening',
            passage: 'שלום',
            audioPath: 'audio/clip.mp3'
        });
        assert.match(present, /data-plan-audio="ready"/);
        assert.equal(present.includes('audio/clip.mp3'), false);
        assert.equal(present.includes('.mp3'), false);
        assert.equal(/<audio\b/i.test(present), false);
    });

    const CLIP_NAME = '806bb3fc99c984531106aeb5';
    const introWith = (audioPath) => ui.renderBlock({
        id: 'introduction',
        type: 'introduction',
        words: [{ id: '1', word: 'קָפֶה', meaning: 'coffee', audioPath }]
    });

    it('marks a shared word clip as ready by its file name only, and carries the word for the browser voice', () => {
        const html = introWith('../_audio/words/' + CLIP_NAME + '.mp3');
        assert.match(html, /data-plan-audio="ready"/);
        assert.match(html, new RegExp('data-plan-audio-clip="' + CLIP_NAME + '"'));
        assert.match(html, /data-plan-audio-word="קָפֶה"/);
        assert.equal(html.includes('.mp3'), false, 'no file address is put in the page');
        assert.equal(html.includes('_audio'), false);

        const none = introWith(undefined);
        assert.match(none, /data-plan-audio="none"/);
        assert.equal(none.includes('data-plan-audio-clip'), false);
        assert.match(none, /data-plan-audio-word="קָפֶה"/, 'a word with no clip can still be spoken');

        const elsewhere = introWith('../../secret/' + CLIP_NAME + '.mp3');
        assert.equal(elsewhere.includes('data-plan-audio-clip'), false, 'only the shared word folder is played');
    });

    describe('playing a word', () => {
        function fakeBrowser(options) {
            const opts = options || {};
            const played = [];
            const spoken = [];
            const prior = {
                Audio: globalThis.Audio,
                Utterance: globalThis.SpeechSynthesisUtterance,
                speech: globalThis.speechSynthesis
            };
            globalThis.Audio = function Audio(url) {
                played.push(url);
                this.pause = () => {};
                this.play = () => (opts.clipFails ? Promise.reject(new Error('blocked')) : Promise.resolve());
            };
            if (opts.voice !== false) {
                globalThis.SpeechSynthesisUtterance = function Utterance(text) { this.text = text; };
                globalThis.speechSynthesis = { cancel() {}, speak(u) { spoken.push({ text: u.text, lang: u.lang }); } };
            } else {
                delete globalThis.SpeechSynthesisUtterance;
                delete globalThis.speechSynthesis;
            }
            return {
                played,
                spoken,
                restore() {
                    ['Audio', 'SpeechSynthesisUtterance', 'speechSynthesis'].forEach((name) => delete globalThis[name]);
                    if (prior.Audio) globalThis.Audio = prior.Audio;
                    if (prior.Utterance) globalThis.SpeechSynthesisUtterance = prior.Utterance;
                    if (prior.speech) globalThis.speechSynthesis = prior.speech;
                }
            };
        }

        it('plays the saved clip from the shared word folder', async () => {
            const env = fakeBrowser();
            try {
                assert.equal(await ui.playWord(CLIP_NAME, 'קָפֶה'), true);
                assert.deepEqual(env.played, ['/worksheets/_audio/words/' + CLIP_NAME + '.mp3']);
                assert.deepEqual(env.spoken, []);
            } finally { env.restore(); }
        });

        it('speaks the word in Hebrew when there is no clip, or the clip will not play', async () => {
            const env = fakeBrowser({ clipFails: true });
            try {
                assert.equal(await ui.playWord(CLIP_NAME, 'קָפֶה'), true);
                assert.equal(await ui.playWord(null, 'קָפֶה'), true);
                assert.deepEqual(env.spoken, [{ text: 'קָפֶה', lang: 'he-IL' }, { text: 'קָפֶה', lang: 'he-IL' }]);
            } finally { env.restore(); }
        });

        it('says nothing played when there is no clip and no voice, and never builds an address from a bad name', async () => {
            const env = fakeBrowser({ voice: false });
            try {
                assert.equal(await ui.playWord(null, 'קָפֶה'), false);
                assert.equal(await ui.playWord('../../etc/passwd', null), false);
                assert.deepEqual(env.played, []);
            } finally { env.restore(); }
        });
    });

    it('keeps an empty attempt hidden and shows the correct form after a miss', () => {
        const empty = ui.decideRecall('   ', 'קָפֶה');
        assert.equal(empty.attempted, false);
        assert.equal(empty.reveal, false);

        const miss = ui.decideRecall('לילה', 'קָפֶה');
        assert.equal(miss.attempted, true);
        assert.equal(miss.reveal, true);
        assert.equal(miss.correct, false);
        assert.equal(miss.keep, 'לילה');

        const hit = ui.decideRecall('קפה', 'קָפֶה');
        assert.equal(hit.correct, true);
        assert.equal(hit.reveal, false);
        assert.equal(hit.keep, 'קפה');
    });
});

describe('published plan entry', () => {
    it('fetches only GET /api/plans/:id/published and escapes the snapshot', async () => {
        const calls = [];
        const mount = { textContent: '', innerHTML: '' };
        const doc = {
            location: { search: '?id=beginner-unit-1' },
            getElementById(id) {
                return id === 'plan-root' ? mount : null;
            },
            querySelectorAll() {
                return [];
            }
        };
        await ui.start(doc, (url, options) => {
            calls.push({ url: url, options: options });
            return Promise.resolve({
                ok: true,
                json() {
                    return Promise.resolve({ title: SCRIPT_PAYLOAD, units: [] });
                }
            });
        });
        assert.equal(calls.length, 1);
        assert.equal(calls[0].url, '/api/plans/beginner-unit-1/published');
        assert.equal(calls[0].options.credentials, 'same-origin');
        assert.equal(mount.innerHTML.includes(SCRIPT_PAYLOAD), false);
        assert.match(mount.innerHTML, /&lt;script&gt;/);
        assert.equal(mount.innerHTML.includes('<script'), false);
    });

    it('does not fetch when the plan id is not a slug', async () => {
        const mount = { textContent: '', innerHTML: '' };
        const doc = {
            location: { search: '?id=../worksheets/my-morning' },
            getElementById(id) {
                return id === 'plan-root' ? mount : null;
            },
            querySelectorAll() {
                return [];
            }
        };
        await ui.start(doc, () => {
            throw new Error('fetch should not run');
        });
        assert.equal(mount.textContent, 'חסר מזהה תוכנית.');
        await assert.rejects(() => ui.loadPublished('https://evil.example/plan'));
        assert.equal(ui.publishedPath('beginner-unit-1'), '/api/plans/beginner-unit-1/published');
    });

    it('the page script does not fetch a worksheet or write plan text except through renderScreens', () => {
        const src = fs.readFileSync(SRC_PATH, 'utf8');
        const withoutPaint = src.replace(
            'mount.innerHTML = renderScreens(plan, {',
            ''
        );
        assert.equal((src.match(/innerHTML/g) || []).length, 1);
        assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write|\beval\s*\(/.test(withoutPaint), false);
        // The one worksheet-folder address is the shared word clips (sound for a new word).
        const worksheetLines = src.split('\n')
            .filter((line) => /worksheets/.test(line) && !/^\s*(\/\/|\*|\/\*)/.test(line))
            .map((line) => line.trim());
        assert.deepEqual(worksheetLines, ["var WORD_CLIP_BASE = '/worksheets/_audio/words/';"]);
        assert.equal(/coffee-cup|water-glass|placeholder-coffee/.test(src), false);
        assert.equal(/\bfetch\s*\(\s*['"`]/.test(src), false);
        assert.equal(/new\s+Audio\s*\(/.test(src), false);
        assert.equal(/XMLHttpRequest|WebSocket/.test(src), false);
        const entry = fs.readFileSync(path.join(ROOT, 'shared', 'plan', 'index.html'), 'utf8');
        assert.match(entry, /<html lang="he" dir="rtl">/);
        // Plan page loads fixed same-origin scripts only (block helpers + plan-render). No inline script.
        assert.match(entry, /src="\/shared\/ui\/plan-render\.js"/);
        assert.equal(/worksheets/.test(entry), false);
        assert.equal(/\bon\w+\s*=/.test(entry), false);
        assert.equal(/<script(?![^>]*\bsrc=)/i.test(entry), false);
        const spa = fs.readFileSync(path.join(ROOT, 'server', 'spa.js'), 'utf8');
        assert.equal(spa.includes('/shared/plan'), false);
    });
});

describe('tokens, targets, and the admin-visibility sentence', () => {
    it('uses token variables, meets 4.5:1, and sizes the controls', () => {
        const block = cssBlock();
        const stripped = block.replace(/\/\*[\s\S]*?\*\//g, '');
        assert.equal(/#[0-9a-fA-F]{3,8}/.test(stripped), false);
        assert.equal(/(^|[^-\w])direction\s*:/.test(stripped), false);
        assert.equal(/text-align\s*:\s*(left|right)/.test(stripped), false);
        assert.equal(/\b(margin|padding|border)-(left|right)\s*:/.test(stripped), false);
        assert.equal(/box-shadow\s*:/.test(stripped), false);
        assert.match(stripped, /var\(--color-text\)/);
        assert.match(stripped, /var\(--color-bg\)/);
        assert.match(stripped, /var\(--color-muted\)/);
        assert.match(stripped, /var\(--color-surface\)/);
        assert.match(stripped, /var\(--color-control\)/);
        assert.match(stripped, /var\(--color-control-text\)/);
        assert.match(stripped, /min-height:\s*var\(--size-target-primary\)/);
        assert.match(stripped, /min-width:\s*var\(--size-target-primary\)/);
        assert.match(stripped, /min-height:\s*var\(--size-target\)/);
        assert.match(stripped, /min-width:\s*var\(--size-target\)/);
        assert.match(stripped, /@media \(max-width:\s*599px\)/);
        assert.match(stripped, /\.dock\s*\{[^}]*position:\s*sticky/);
        assert.match(stripped, /font-size:\s*var\(--size-word\)/);
        assert.match(stripped, /line-height:\s*var\(--line-height-word\)/);
        assert.match(stripped, /@media \(prefers-reduced-motion: no-preference\)/);

        const doc = JSON.parse(fs.readFileSync(TOKENS_PATH, 'utf8'));
        const byId = new Map(doc.tokens.map((token) => [token.id, token]));
        doc.contrast.filter((row) => row.kind === 'text').forEach((row) => {
            const left = byId.get(row.pair[0]);
            const right = byId.get(row.pair[1]);
            assert.ok(contrast(left.light, right.light) + 1e-9 >= 4.5, row.pair.join('/') + ' light');
            assert.ok(contrast(left.dark, right.dark) + 1e-9 >= 4.5, row.pair.join('/') + ' dark');
        });
        const target = byId.get('size.target');
        const primary = byId.get('size.targetPrimary');
        assert.equal(target.value, '24px');
        assert.equal(primary.value, '44px');
    });

    it('leaves the sign-in page and the account menu sentence in place', () => {
        const shell = fs.readFileSync(path.join(ROOT, 'shared', 'app-shell.js'), 'utf8');
        const loginJs = fs.readFileSync(path.join(ROOT, 'shared', 'login.js'), 'utf8');
        const loginHtml = fs.readFileSync(path.join(ROOT, 'login', 'index.html'), 'utf8');
        assert.ok(shell.includes(PRIVACY));
        assert.ok(loginJs.includes(PRIVACY));
        assert.ok(loginHtml.includes(PRIVACY));
        assert.match(shell, /data-nav/);
        const today = fs.readFileSync(path.join(ROOT, 'shared', 'today-page.js'), 'utf8');
        assert.match(today, /data-today-card/);
        const page = ui.renderPage(loadPlan());
        assert.equal(page.includes('data-action="set-dir"'), false);
        assert.equal(page.includes('data-today-card'), false);
        assert.equal(page.includes('data-nav='), false);
    });
});
