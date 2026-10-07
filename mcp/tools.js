const { z } = require('zod');
const store = require('./store');

function text(value) {
    return { content: [{ type: 'text', text: typeof value === 'string' ? value : JSON.stringify(value, null, 2) }] };
}

function failure(err) {
    return { isError: true, content: [{ type: 'text', text: err && err.message ? err.message : String(err) }] };
}

function registerLessonTools(server) {
    server.registerTool('list_lessons', {
        title: 'List Hebrew lessons',
        description: 'List published lessons on the shelf: slug, title, subtitle, and blurb.',
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

    server.registerTool('upsert_lesson', {
        title: 'Add or replace a Hebrew lesson',
        description: [
            'Write a published lesson the shared player can open.',
            'Pass slug plus worksheet. worksheet is the object assigned to window.WORKSHEET: title, subtitle, and any of vocab, fill, grammar, reading, askFriend.',
            'Lesson rules: the story is most of the lesson, written with vowel points, new words wrapped in { word }, and reading.retell cues for round 2. grammar.rules holds exactly one pattern; extra points go in grammar.reminders as one-liners. Bring back 3 to 5 earlier words with from (see list_words). Every speaking question has pointed Hebrew.',
            'The tool sets id to the slug, copies worksheets/_template/index.html when the folder is new, updates worksheets/catalog.js, and runs the project validator.',
            'A failed check restores the previous files. Warnings do not block the write; fix them. Do not send a learner name, email, or class transcript. Do not include a paid voice or accounts.'
        ].join(' '),
        inputSchema: {
            slug: z.string().describe('New folder name: lowercase words separated by hyphens.'),
            title: z.string().optional().describe('Shelf title. Defaults to worksheet.title.'),
            subtitle: z.string().optional().describe('Shelf subtitle. Defaults to worksheet.subtitle.'),
            blurb: z.string().optional().describe('One line under the card on the home page.'),
            worksheet: z.any().describe('Lesson object: title plus vocab, fill, grammar, reading, and askFriend sections to include.')
        },
        annotations: { idempotentHint: true }
    }, async (args) => {
        try {
            const saved = store.upsertLesson(args);
            const head = (saved.created ? 'Created ' : 'Updated ') + saved.path + '. Validation passed.';
            return text(saved.warnings.length
                ? head + '\nFix these warnings and call upsert_lesson again:\n- ' + saved.warnings.join('\n- ')
                : head);
        } catch (err) {
            return failure(err);
        }
    });
}

module.exports = { registerLessonTools };
