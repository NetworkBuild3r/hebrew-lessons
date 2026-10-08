const { z } = require('zod');
const store = require('./store');
const { CATALOG, LEVEL_DEFAULTS } = require('../shared/activities/catalog');
const { generateLessonAudio, formatAudioSummary } = require('../server/audio');

function text(value) {
    return { content: [{ type: 'text', text: typeof value === 'string' ? value : JSON.stringify(value, null, 2) }] };
}

function failure(err) {
    return { isError: true, content: [{ type: 'text', text: err && err.message ? err.message : String(err) }] };
}

function registerLessonTools(server) {
    server.registerTool('list_lessons', {
        title: 'List Hebrew lessons',
        description: 'List published lessons on the shelf: slug, title, subtitle, level, and blurb.',
        annotations: { readOnlyHint: true }
    }, async () => {
        try {
            return text(store.listLessons());
        } catch (err) {
            return failure(err);
        }
    });

    server.registerTool('get_lesson', {
        title: 'Read a Hebrew lesson',
        description: 'Read one lesson worksheet and its catalog entry. Use this before changing a lesson that already exists.',
        inputSchema: {
            slug: z.string().describe('Folder name, such as shai-date-indirect-speech.')
        },
        annotations: { readOnlyHint: true }
    }, async ({ slug }) => {
        try {
            return text(store.getLesson(slug));
        } catch (err) {
            return failure(err);
        }
    });

    server.registerTool('list_words', {
        title: 'List vocabulary from published lessons',
        description: 'List every vocab item in the published lessons, or in one lesson. Use it to choose 3 to 5 earlier words to bring back in a new lesson: copy the item into the new lesson\'s vocab.items, give it a new id, and set from to the lesson slug it came from.',
        inputSchema: {
            slug: z.string().optional().describe('Only words from this lesson.')
        },
        annotations: { readOnlyHint: true }
    }, async ({ slug }) => {
        try {
            return text(store.listWords(slug));
        } catch (err) {
            return failure(err);
        }
    });

    server.registerTool('list_activity_types', {
        title: 'List lesson activity types',
        description: 'List every activity the player knows: type, when to use it, fields, level defaults, and whether it counts as meaning or drill. Call this before choosing the activities array for upsert_lesson.',
        annotations: { readOnlyHint: true }
    }, async () => {
        return text({
            levels: LEVEL_DEFAULTS,
            activities: CATALOG
        });
    });

    server.registerTool('upsert_lesson', {
        title: 'Add or replace a Hebrew lesson',
        description: [
            'Write a published lesson the shared player can open.',
            'Pass slug plus worksheet. worksheet is the object assigned to window.WORKSHEET: title, level (basic|intermediate|advanced), optional activities array, and any of vocab, fill, grammar, reading, askFriend.',
            'activities is an ordered path of steps, each { type, ...fields }. Call list_activity_types for the catalog. Omit activities and the player builds a default path from the sections that exist.',
            'Lesson rules: the story is most of the lesson, written with vowel points, new words wrapped in { word }, and reading.retell cues for round 2. grammar.rules holds exactly one pattern; extra points go in grammar.reminders as one-liners. Bring back 3 to 5 earlier words with from (see list_words). Every speaking question has pointed Hebrew.',
            'Level defaults: basic = 3 choices and open word bank; intermediate = 4 choices and closed bank; advanced = typing allowed and shorter timers. Vowels stay on with a manual toggle. Do not add root/binyan speed drills.',
            'Audio: after a successful write, the server generates ElevenLabs clips automatically (words, vocab sentences, story lines, grammar pointed lines, speaking and comprehension questions). Do not write audio, sentenceAudio, or reading.audio fields yourself — omit them. Words are cached under _audio/words and reused across lessons (never regenerated). Pass audio:false to skip generation.',
            'From a class call: pick the story, 8 to 12 words, one pattern, 3 to 5 recycled words, then an activity mix by level. Never put the transcript or the learner name in the lesson.',
            'The tool sets id to the slug, copies worksheets/_template/index.html when the folder is new, updates worksheets/catalog.js (or LESSONS_DIR), and runs the project validator.',
            'A failed check restores the previous files. Warnings do not block the write; fix them. Audio failures do not roll back the lesson.'
        ].join(' '),
        inputSchema: {
            slug: z.string().describe('New folder name: lowercase words separated by hyphens.'),
            title: z.string().optional().describe('Shelf title. Defaults to worksheet.title.'),
            subtitle: z.string().optional().describe('Shelf subtitle. Defaults to worksheet.subtitle.'),
            blurb: z.string().optional().describe('One line under the card on the home page.'),
            level: z.string().optional().describe('basic, intermediate, or advanced.'),
            worksheet: z.any().describe('Lesson object: title, level, activities, plus vocab, fill, grammar, reading, askFriend sections to include.'),
            audio: z.boolean().optional().describe('Generate ElevenLabs audio after a successful write. Defaults to true. Pass false to skip.')
        },
        annotations: { idempotentHint: true }
    }, async (args) => {
        try {
            const saved = store.upsertLesson(args);
            let head = (saved.created ? 'Created ' : 'Updated ') + saved.path + '. Validation passed.';
            if (saved.warnings.length) {
                head += '\nFix these warnings and call upsert_lesson again:\n- ' + saved.warnings.join('\n- ');
            }
            if (args.audio === false) {
                return text(head + '\nAudio skipped (audio:false).');
            }
            try {
                const audioResult = await generateLessonAudio(saved.slug, {});
                return text(head + '\n' + formatAudioSummary(audioResult));
            } catch (audioErr) {
                return text(head + '\nAudio failed (lesson kept): ' + (audioErr && audioErr.message ? audioErr.message : String(audioErr)));
            }
        } catch (err) {
            return failure(err);
        }
    });

    server.registerTool('generate_lesson_audio', {
        title: 'Generate ElevenLabs audio for a lesson',
        description: [
            'Generate or relink audio for an existing published lesson.',
            'Words are shared under _audio/words and never overwritten; existing word files are reused across lessons.',
            'Sentences, story lines, grammar pointed lines, and questions regenerate only when their text changes, or when forceSentences is true.',
            'Use after backfilling an older lesson, or when upsert_lesson was called with audio:false.'
        ].join(' '),
        inputSchema: {
            slug: z.string().describe('Lesson folder name.'),
            forceSentences: z.boolean().optional().describe('Regenerate sentence/story/grammar/question clips even when text is unchanged. Never regenerates words.')
        },
        annotations: { idempotentHint: true }
    }, async ({ slug, forceSentences }) => {
        try {
            const result = await generateLessonAudio(slug, { forceSentences: !!forceSentences });
            return text(formatAudioSummary(result) + '\n' + JSON.stringify({
                wordsReused: result.wordsReused,
                wordsCreated: result.wordsCreated,
                wordsMissing: result.wordsMissing,
                clipsReused: result.clipsReused,
                clipsCreated: result.clipsCreated,
                clipsMissing: result.clipsMissing,
                errors: result.errors
            }, null, 2));
        } catch (err) {
            return failure(err);
        }
    });
}

module.exports = { registerLessonTools };
