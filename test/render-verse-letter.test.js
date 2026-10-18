/**
 * Verse-reading and letter-intro renderers, recipe chips, and the Guides shelf.
 * Hostile plan text stays text. Hebrew is dir="rtl".
 * Authored-by-spec: INIT-006/SPEC-016
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const verse = require('../shared/ui/blocks/verse-reading');
const letter = require('../shared/ui/blocks/letter-intro');
const guides = require('../shared/guides-page');
const ui = require('../shared/ui/plan-render');
const planRoutes = require('../server/api/plan-routes');

const ROOT = path.resolve(__dirname, '..');
const PAYLOAD = '<script>alert(1)</script>';
const HANDLER = '<img src=x onerror=alert(1)>';
const VERSE_FIXTURE = path.join(ROOT, 'test', 'fixtures', 'plans-1.1', 'verse-reading.plan.json');

function count(html, needle) {
    let n = 0;
    let from = 0;
    while (from < html.length) {
        const at = html.indexOf(needle, from);
        if (at < 0) break;
        n += 1;
        from = at + needle.length;
    }
    return n;
}

function hostileVerse() {
    return {
        id: PAYLOAD,
        type: 'verse-reading',
        ref: {
            book: PAYLOAD,
            chapter: PAYLOAD,
            from: PAYLOAD,
            to: PAYLOAD,
            source: PAYLOAD
        },
        newWords: [PAYLOAD],
        verse: PAYLOAD,
        questions: [
            { id: PAYLOAD, he: PAYLOAD, en: PAYLOAD },
            { id: 'q2', he: PAYLOAD, en: PAYLOAD },
            { id: 'q3', he: PAYLOAD, en: PAYLOAD },
            { id: 'q4', he: PAYLOAD, en: PAYLOAD },
            { id: 'q5', he: PAYLOAD, en: 'fifth question must be dropped' }
        ],
        personalQuestion: { he: PAYLOAD, en: HANDLER },
        translation: 'UNIQUE_TRANSLATION_SHOULD_NOT_SHOW',
        image: { src: 'http://evil.example/verse.png', alt: PAYLOAD }
    };
}

function hostileLetter() {
    return {
        id: PAYLOAD,
        type: 'letter-intro',
        letter: PAYLOAD,
        name: PAYLOAD,
        sounds: [{
            id: PAYLOAD,
            vowel: PAYLOAD,
            pointed: PAYLOAD,
            sound: PAYLOAD
        }],
        findIn: [
            { id: PAYLOAD, word: PAYLOAD, contains: true },
            { id: 'f2', word: HANDLER, contains: false },
            { id: 'f3', word: 'כן', contains: 'true' }
        ],
        image: { mediaId: 'x', alt: PAYLOAD, src: 'http://evil.example/a.png' },
        picture: { src: 'http://evil.example/b.png', alt: HANDLER }
    };
}

function publishedLesson(overrides) {
    const lesson = Object.assign({
        id: 'decoding',
        title: 'Decoding Hebrew Verbs',
        level: 'basic',
        visibility: 'everyone',
        recipe: 'guide',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{ id: 'rule', type: 'explainer' }]
    }, overrides || {});
    return {
        schemaVersion: '1.1.0',
        id: 'guides-shelf',
        revision: 2,
        status: 'published',
        audience: 'shared',
        assigneeId: null,
        title: 'Guides',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'verbs',
            title: 'Verbs',
            theme: 'how verbs work',
            lessons: [lesson]
        }]
    };
}

describe('verse-reading renderer', () => {
    it('escapes hostile strings, keeps Hebrew rtl, and drops a stored translation', () => {
        const html = verse.render(hostileVerse(), {
            audio: { [PAYLOAD]: 'javascript:alert(1)' },
            lexicon: { [PAYLOAD]: { gloss_en: PAYLOAD } }
        });
        const escaped = verse.escapeHtml(PAYLOAD);
        assert.equal(html.includes(PAYLOAD), false);
        assert.equal(html.includes(HANDLER), false);
        assert.ok(count(html, escaped) >= 8, String(count(html, escaped)));
        assert.ok(html.includes(verse.escapeHtml(HANDLER)));
        assert.equal(/<img\b/i.test(html), false);
        assert.equal(/<script/i.test(html), false);
        assert.equal(html.includes('javascript:'), false);
        assert.equal(html.includes('evil.example'), false);
        assert.equal(html.includes('UNIQUE_TRANSLATION_SHOULD_NOT_SHOW'), false);
        assert.equal(html.includes('fifth question must be dropped'), false);
        assert.equal((html.match(/data-question-id/g) || []).length, 4);
        assert.match(html, /lang="he" dir="rtl"/);
        assert.match(html, /data-verse-part="new-words"/);
        assert.match(html, /data-verse-part="verse"/);
        assert.match(html, /data-block-type="personal-question"/);
        const wordsAt = html.indexOf('data-verse-part="new-words"');
        const verseAt = html.indexOf('data-verse-part="verse"');
        const askAt = html.indexOf('data-block-type="personal-question"');
        assert.ok(wordsAt >= 0 && wordsAt < verseAt && verseAt < askAt);
    });

    it('shows fixture new words before the verse and the personal question last', () => {
        const plan = JSON.parse(fs.readFileSync(VERSE_FIXTURE, 'utf8'));
        const block = plan.units[0].lessons[0].blocks[0];
        const html = verse.render(block, {
            lexicon: { 'בָּרָא': { gloss_en: 'created' } },
            audio: { 'בָּרָא': 'words/bara.mp3' }
        });
        const wordsAt = html.indexOf('data-verse-part="new-words"');
        const verseAt = html.indexOf('data-verse-part="verse"');
        const askAt = html.indexOf('data-block-type="personal-question"');
        assert.ok(wordsAt < verseAt && verseAt < askAt);
        assert.ok(html.slice(wordsAt, verseAt).includes('בָּרָא'));
        assert.ok(html.slice(verseAt, askAt).includes('בְּרֵאשִׁית'));
        assert.match(html.slice(askAt), /מָה אַתָּה רוֹאֶה/);
        assert.match(html, /data-verse-gloss/);
        assert.match(html, /created/);
        assert.match(html, /data-plan-audio="ready"/);
        assert.equal(html.includes('words/bara.mp3'), false);
        assert.match(html, /lang="he" dir="rtl"/);
        assert.match(html, /Genesis/);
        assert.match(html, /1:1/);
        const screens = ui.renderScreens(plan);
        const screenWords = screens.indexOf('data-verse-part="new-words"');
        const screenVerse = screens.indexOf('data-verse-part="verse"');
        const screenAsk = screens.indexOf('data-block-type="personal-question"');
        assert.ok(screenWords < screenVerse && screenVerse < screenAsk);
    });

    it('respects the vowels toggle by stripping points from the shown text', () => {
        const html = verse.render({
            id: 'v',
            type: 'verse-reading',
            ref: { book: 'Genesis', chapter: 1, from: 1, to: 1, source: 'WLC' },
            newWords: ['בָּרָא'],
            verse: 'בָּרָא'
        }, { vowelsOn: false });
        assert.match(html, /data-vowels-on="0"/);
        assert.match(html, /aria-pressed="false"/);
        assert.match(html, /data-new-word="בָּרָא"><p lang="he" dir="rtl" data-vowels="בָּרָא" class="pointed-line">ברא<\/p>/);
        assert.match(html, /data-verse-part="verse"[^>]*>[\s\S]*data-vowels="בָּרָא">ברא</);
    });
});

describe('letter-intro renderer', () => {
    it('escapes hostile strings, offers tap choices, and draws no picture', () => {
        const html = letter.render(hostileLetter(), {
            audio: {
                [PAYLOAD]: 'javascript:alert(1)',
                safe: 'clips/a.mp3'
            }
        });
        const escaped = letter.escapeHtml(PAYLOAD);
        assert.equal(html.includes(PAYLOAD), false);
        assert.equal(html.includes(HANDLER), false);
        assert.ok(count(html, escaped) >= 6, String(count(html, escaped)));
        assert.ok(html.includes(letter.escapeHtml(HANDLER)));
        assert.equal(/<img\b/i.test(html), false);
        assert.equal(/<script/i.test(html), false);
        assert.equal(html.includes('javascript:'), false);
        assert.equal(html.includes('evil.example'), false);
        assert.equal(html.includes('.mp3'), false);
        assert.match(html, /lang="he" dir="rtl"/);
        assert.match(html, /class="letter-hero"/);
        assert.match(html, /data-letter-part="sounds"/);
        assert.match(html, /Which word contains the letter/);
        assert.match(html, /Write the letter on paper/);
        assert.match(html, /not scored/);
        assert.equal((html.match(/data-contains="yes"/g) || []).length, 1);
        assert.equal((html.match(/data-contains="no"/g) || []).length, 2);
        assert.equal(/<form\b/i.test(html), false);
    });

    it('plays a safe clip without copying the path, and skips a missing picture field', () => {
        const html = letter.render({
            id: 'bet',
            type: 'letter-intro',
            letter: 'בּ',
            name: 'בֵּית',
            sounds: [{ id: 's1', vowel: 'קָמָץ', pointed: 'בָּ', sound: 'ba' }],
            findIn: [{ id: 'f1', word: 'בַּיִת', contains: true }]
        }, { audio: { 'בָּ': 'letters/ba.mp3' } });
        assert.match(html, /data-plan-audio="ready"/);
        assert.equal(html.includes('letters/ba.mp3'), false);
        assert.match(html, /dir="rtl"/);
        assert.equal(/<img\b/i.test(html), false);
    });
});

describe('recipe chips on the plan lesson shelf', () => {
    it('names Conversation, Review, Verse, Verbs, Letters, and Guide', () => {
        const recipes = [
            ['topic-conversation', 'Conversation'],
            ['review-worksheet', 'Review'],
            ['verse-reading', 'Verse'],
            ['verb-workshop', 'Verbs'],
            ['letters-and-vowels', 'Letters'],
            ['guide', 'Guide']
        ];
        const html = ui.renderScreens({
            title: 'Program',
            units: [{
                id: 'unit',
                title: 'Unit',
                lessons: recipes.map(function (pair, index) {
                    return { id: 'lesson-' + index, title: 'Lesson ' + index, recipe: pair[0] };
                }).concat([{ id: 'plain', title: 'No recipe', recipe: 'nope<script>' }])
            }]
        });
        recipes.forEach(function (pair) {
            assert.match(html, new RegExp('data-recipe="' + pair[0] + '"'));
            assert.match(html, new RegExp('>' + pair[1] + '<'));
        });
        assert.equal(html.includes('data-recipe="nope'), false);
        assert.equal(html.includes('<script>'), false);
    });
});

describe('guides shelf', () => {
    it('groups guide lessons by topic and escapes titles', () => {
        const html = guides.render([
            {
                planId: 'guides-shelf',
                lessonId: 'decoding',
                recipe: 'guide',
                topic: '<script>alert(1)</script>',
                title: HANDLER
            },
            {
                planId: 'guides-shelf',
                lessonId: 'reading',
                recipe: 'guide',
                topic: 'Reading',
                title: 'How to read'
            },
            {
                planId: 'javascript:alert(1)',
                lessonId: 'decoding',
                recipe: 'guide',
                topic: 'Bad',
                title: 'Dropped'
            },
            {
                planId: 'verbs',
                lessonId: 'think',
                recipe: 'verb-workshop',
                topic: 'Verbs',
                title: 'Think'
            }
        ]);
        assert.equal(html.includes(PAYLOAD), false);
        assert.equal(html.includes(HANDLER), false);
        assert.equal(/<script/i.test(html), false);
        assert.equal(/<img\b/i.test(html), false);
        assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
        assert.match(html, /href="\/shared\/plan\/\?id=guides-shelf&amp;lesson=decoding"/);
        assert.match(html, /data-recipe="guide"/);
        assert.match(html, />Guide</);
        assert.match(html, />Reading</);
        assert.equal(html.includes('javascript:'), false);
        assert.equal(html.includes('Dropped'), false);
        assert.equal(html.includes('Think'), false);
        assert.equal((html.match(/data-topic/g) || []).length, 2);
    });

    it('says when no guide is published', () => {
        const html = guides.render([]);
        assert.match(html, /data-guides-state="empty"/);
        assert.match(html, /No published guides/);
    });
});

describe('published catalog', () => {
    const user = { id: 1, username: 'learner', role: 'user' };

    it('lists a readable guide and omits a hidden lesson and another learner\'s plan', () => {
        const visible = publishedLesson();
        const hidden = publishedLesson({ id: 'secret', title: 'ZZ-HIDDEN-GUIDE', visibility: 'hidden' });
        hidden.id = 'secret-guides';
        const personal = publishedLesson();
        personal.id = 'personal-guides';
        personal.audience = 'personal';
        personal.assigneeId = 'someone-else';
        const out = planRoutes.catalogFromSnapshots(user, [
            { id: 'guides-shelf', snapshot: JSON.stringify(visible) },
            { id: 'secret-guides', snapshot: JSON.stringify(hidden) },
            { id: 'personal-guides', snapshot: JSON.stringify(personal) },
            { id: 'not a slug', snapshot: JSON.stringify(visible) }
        ]);
        assert.equal(out.lessons.length, 1);
        assert.deepEqual(out.lessons[0], {
            planId: 'guides-shelf',
            planTitle: 'Guides',
            unitId: 'verbs',
            topic: 'Verbs',
            lessonId: 'decoding',
            title: 'Decoding Hebrew Verbs',
            recipe: 'guide'
        });
        assert.equal(Object.prototype.hasOwnProperty.call(out.lessons[0], 'blocks'), false);
    });

    it('keeps a hostile title as data, not as a field that the shelf would trust raw', () => {
        const doc = publishedLesson({ title: PAYLOAD });
        const out = planRoutes.catalogFromSnapshots(user, [
            { id: 'guides-shelf', snapshot: JSON.stringify(doc) }
        ]);
        assert.equal(out.lessons[0].title, PAYLOAD);
        const html = guides.render(out.lessons);
        assert.equal(html.includes(PAYLOAD), false);
        assert.match(html, /&lt;script&gt;/);
    });
});
