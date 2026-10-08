const { z } = require('zod');
const store = require('./store');
const { CATALOG, LEVEL_DEFAULTS } = require('../shared/activities/catalog');
const { generateLessonAudio, formatAudioSummary } = require('../server/audio');
const { rebuildVocabIndexSafe } = require('../server/vocab-index');
const accounts = require('../server/accounts');
const visibility = require('../server/visibility');
const visibilityRepo = require('../server/db/repos/visibility');
const usersRepo = require('../server/db/repos/users');
const fs = require('fs');
const path = require('path');
const { lessonDir } = require('../server/paths');

/** Test seam: registered tool async handlers by name. INIT-002/SPEC-007 */
const toolHandlers = Object.create(null);

function text(value) {
    return { content: [{ type: 'text', text: typeof value === 'string' ? value : JSON.stringify(value, null, 2) }] };
}

function failure(err) {
    return { isError: true, content: [{ type: 'text', text: err && err.message ? err.message : String(err) }] };
}

/**
 * Build the upsert_lesson reply head (path + validation summary + warnings).
 * Warnings come from validate.js WARN lines via store.upsertLesson — keep this
 * ahead of rebuildVocabIndexSafe so index rebuild never hides them (INIT-001/SPEC-008).
 */
function formatUpsertHead(saved) {
    let head = (saved.created ? 'Created ' : 'Updated ') + saved.path + '. Validation passed.';
    if (saved.warnings && saved.warnings.length) {
        head += '\nFix these warnings and call upsert_lesson again:\n- ' + saved.warnings.join('\n- ');
    }
    return head;
}

/** Fail closed when MCP_AUTH_TOKEN is missing or empty. INIT-002/SPEC-007 */
function requireMcpAuthToken() {
    const token = process.env.MCP_AUTH_TOKEN;
    if (typeof token !== 'string' || !token) {
        throw new Error(
            'MCP_AUTH_TOKEN must be configured (non-empty) to use this tool or visibility argument.'
        );
    }
}

/**
 * Resolve usernames to active user ids before any write.
 * @param {string[]} usernames
 * @returns {number[]}
 */
function resolveActiveUserIds(usernames) {
    if (!Array.isArray(usernames)) {
        throw new Error('visibility.users must be an array of usernames.');
    }
    const ids = [];
    const seen = new Set();
    for (const raw of usernames) {
        const name = String(raw || '').trim();
        if (!name) throw new Error('visibility.users contains an empty username.');
        if (seen.has(name)) continue;
        seen.add(name);
        const row = usersRepo.findByUsername(name);
        if (!row || row.status !== 'active') {
            throw new Error('Unknown or inactive username: "' + name + '".');
        }
        ids.push(Number(row.id));
    }
    return ids;
}

/**
 * Normalize a visibility argument.
 * @param {unknown} value
 * @returns {{ kind: 'everyone'|'hidden'|'selected', userIds?: number[], usernames?: string[] }}
 */
function parseVisibilityArg(value) {
    if (value === 'everyone') return { kind: 'everyone' };
    if (value === 'hidden') return { kind: 'hidden' };
    if (value && typeof value === 'object' && !Array.isArray(value) && Array.isArray(value.users)) {
        const usernames = value.users.map((u) => String(u));
        const userIds = resolveActiveUserIds(usernames);
        return { kind: 'selected', userIds, usernames };
    }
    throw new Error(
        'visibility must be "everyone", "hidden", or { users: ["username", ...] }.'
    );
}

/**
 * Plain-words description of current visibility for a slug.
 * @param {string} slug
 */
function describeVisibility(slug) {
    const row = visibilityRepo.getVisibility(slug);
    if (!row || row.mode === 'hidden') {
        return 'hidden (not visible to learners)';
    }
    if (row.mode === 'everyone') {
        return 'everyone (visible to all learners)';
    }
    if (row.mode === 'selected') {
        const grants = visibilityRepo.listGrants(slug);
        const names = [];
        for (const g of grants) {
            const u = usersRepo.findById(g.user_id);
            if (u) names.push(u.username);
        }
        if (names.length) {
            return 'selected (visible to: ' + names.join(', ') + ')';
        }
        return 'selected (no users granted yet)';
    }
    return String(row.mode);
}

/**
 * Apply visibility after a successful upsert. Actor is always mcp.
 * @param {string} slug
 * @param {{ kind: string, userIds?: number[] }} parsed
 */
function applyVisibility(slug, parsed) {
    if (parsed.kind === 'everyone') {
        visibility.setVisibility('mcp', slug, 'everyone');
        return;
    }
    if (parsed.kind === 'hidden') {
        visibility.setVisibility('mcp', slug, 'hidden');
        return;
    }
    if (parsed.kind === 'selected') {
        visibility.setVisibility('mcp', slug, 'selected');
        visibility.setGrants('mcp', slug, parsed.userIds || []);
    }
}

function lessonExists(slug) {
    return fs.existsSync(path.join(lessonDir(slug), 'worksheet.js'));
}

function register(server, name, meta, handler) {
    toolHandlers[name] = handler;
    server.registerTool(name, meta, handler);
}

function registerLessonTools(server) {
    register(server, 'list_lessons', {
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

    register(server, 'get_lesson', {
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

    register(server, 'list_words', {
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

    register(server, 'list_activity_types', {
        title: 'List lesson activity types',
        description: 'List every activity the player knows: type, when to use it, fields, level defaults, and whether it counts as meaning or drill. Call this before choosing the activities array for upsert_lesson.',
        annotations: { readOnlyHint: true }
    }, async () => {
        return text({
            levels: LEVEL_DEFAULTS,
            activities: CATALOG
        });
    });

    // INIT-002/SPEC-007: usernames + display names of active accounts only.
    register(server, 'list_users', {
        title: 'List active learner accounts',
        description: 'List usernames and display names of active accounts. Requires MCP_AUTH_TOKEN. Returns nothing else (no roles, hashes, tokens, or activity).',
        annotations: { readOnlyHint: true }
    }, async () => {
        try {
            requireMcpAuthToken();
            const users = accounts.listUsers()
                .filter((u) => u && u.status === 'active')
                .map((u) => ({
                    username: u.username,
                    displayName: u.display_name == null ? null : u.display_name
                }));
            return text(users);
        } catch (err) {
            return failure(err);
        }
    });

    register(server, 'upsert_lesson', {
        title: 'Add or replace a Hebrew lesson',
        description: [
            'Write a published lesson the shared player can open.',
            'Pass slug plus worksheet. worksheet is the object assigned to window.WORKSHEET: title, level (basic|intermediate|advanced), optional activities array, and any of vocab, fill, grammar, reading, askFriend.',
            'activities is an ordered path of steps, each { type, ...fields }. Call list_activity_types for the catalog. Omit activities and the player builds a default path from the sections that exist.',
            'Lesson rules: the story is most of the lesson, written with vowel points, new words wrapped in { word }, and reading.retell cues for round 2. grammar.rules holds exactly one pattern; extra points go in grammar.reminders as one-liners. Bring back 3 to 5 earlier words with from (see list_words). Every speaking question has pointed Hebrew.',
            'Level defaults: basic = 3 choices and open word bank; intermediate = 4 choices and closed bank; advanced = typing allowed and shorter timers. Vowels stay on with a manual toggle. Do not add root/binyan speed drills.',
            'Audio: after a successful write, the server generates ElevenLabs clips automatically (words, vocab sentences, story lines, grammar pointed lines, speaking and comprehension questions). Do not write audio, sentenceAudio, or reading.audio fields yourself — omit them. Words are cached under _audio/words and reused across lessons (never regenerated). Pass audio:false to skip generation.',
            'Visibility (optional): "everyone" or { users: ["username", ...] }. Requires MCP_AUTH_TOKEN. Unknown usernames fail before any write. Omit on a new lesson to keep it hidden; omit on an existing lesson to leave visibility unchanged.',
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
            audio: z.boolean().optional().describe('Generate ElevenLabs audio after a successful write. Defaults to true. Pass false to skip.'),
            visibility: z.any().optional().describe(
                'Optional. "everyone" or { users: ["username", ...] }. Requires MCP_AUTH_TOKEN. Omit to leave new lessons hidden and existing lessons unchanged.'
            )
        },
        annotations: { idempotentHint: true }
    }, async (args) => {
        try {
            const hasVisibility = args && Object.prototype.hasOwnProperty.call(args, 'visibility')
                && args.visibility !== undefined;
            let parsedVisibility = null;
            if (hasVisibility) {
                // Fail closed before any file write. INIT-002/SPEC-007
                requireMcpAuthToken();
                // upsert_lesson accepts "everyone" or { users }; "hidden" is set_lesson_visibility only.
                if (args.visibility === 'everyone'
                    || (args.visibility && typeof args.visibility === 'object'
                        && Array.isArray(args.visibility.users))) {
                    parsedVisibility = parseVisibilityArg(args.visibility);
                } else {
                    throw new Error(
                        'visibility must be "everyone" or { users: ["username", ...] }.'
                    );
                }
            }

            const existedBefore = lessonExists(args.slug);
            const saved = store.upsertLesson(args);
            const head = formatUpsertHead(saved);

            let visibilityNote = '';
            if (parsedVisibility) {
                applyVisibility(saved.slug, parsedVisibility);
                visibilityNote = '\nVisibility is now ' + describeVisibility(saved.slug) + '.';
            } else if (saved.created || !existedBefore) {
                // ADR-0002 default: new lesson stays hidden when visibility omitted.
                visibilityNote = '\nVisibility is hidden (default for new lessons; no visibility argument was given).';
            } else {
                visibilityNote = '\nVisibility was left unchanged (no visibility argument was given). '
                    + 'It is currently ' + describeVisibility(saved.slug) + '.';
            }

            let body;
            if (args.audio === false) {
                // Still relink any words that already exist in the shared cache (no API calls).
                try {
                    const linked = await generateLessonAudio(saved.slug, { wordsOnly: true, linkOnly: true });
                    body = head + visibilityNote + '\nAudio skipped (audio:false), but shared words were linked.\n' + formatAudioSummary(linked);
                } catch (linkErr) {
                    body = head + visibilityNote + '\nAudio skipped (audio:false).';
                }
            } else {
                try {
                    // Always reuse existing word clips across lessons; create only missing ones.
                    const audioResult = await generateLessonAudio(saved.slug, {});
                    body = head + visibilityNote + '\n' + formatAudioSummary(audioResult);
                } catch (audioErr) {
                    try {
                        const linked = await generateLessonAudio(saved.slug, { wordsOnly: true, linkOnly: true });
                        body = head + visibilityNote + '\nAudio failed (lesson kept); shared words linked where possible.\n'
                            + formatAudioSummary(linked)
                            + '\nAudio error: ' + (audioErr && audioErr.message ? audioErr.message : String(audioErr));
                    } catch (linkErr) {
                        body = head + visibilityNote + '\nAudio failed (lesson kept): ' + (audioErr && audioErr.message ? audioErr.message : String(audioErr));
                    }
                }
            }
            // Guarded: builder failure must never fail publish (INIT-001/SPEC-002).
            rebuildVocabIndexSafe();
            return text(body);
        } catch (err) {
            return failure(err);
        }
    });

    // INIT-002/SPEC-007: change lesson access immediately.
    register(server, 'set_lesson_visibility', {
        title: 'Set who can see a lesson',
        description: [
            'Change lesson visibility immediately.',
            'Pass slug and visibility: "everyone", "hidden", or { users: ["username", ...] }.',
            'Requires MCP_AUTH_TOKEN. Unknown usernames fail without changing anything.',
            'Audited as actor mcp.'
        ].join(' '),
        inputSchema: {
            slug: z.string().describe('Lesson folder name.'),
            visibility: z.any().describe('"everyone", "hidden", or { users: ["username", ...] }.')
        },
        annotations: { idempotentHint: true }
    }, async ({ slug, visibility: visibilityArg }) => {
        try {
            requireMcpAuthToken();
            const parsed = parseVisibilityArg(visibilityArg);
            applyVisibility(slug, parsed);
            return text('Visibility for "' + slug + '" is now ' + describeVisibility(slug) + '.');
        } catch (err) {
            return failure(err);
        }
    });

    register(server, 'generate_lesson_audio', {
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

module.exports = { registerLessonTools, formatUpsertHead, toolHandlers };
