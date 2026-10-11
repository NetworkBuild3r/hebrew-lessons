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
const planStore = require('../server/plans/store');
const planOps = require('../server/plans/ops');
const planPublish = require('../server/plans/publish');
const ingest = require('../server/media/ingest');
const images = require('../server/media/openai-images');
const audit = require('../server/db/repos/audit');
const profile = require('../server/learner/profile');
const coverage = require('../server/learner/coverage');
const library = require('../server/learner/library');
const lexicon = require('../server/lexicon/service');
const conceptsRepo = require('../server/db/repos/concepts');
const userWordsRepo = require('../server/db/repos/user-words');

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

/**
 * Map a plan-op result to an MCP tool reply.
 * Errors name `error` (revision_conflict includes the stored revision) and
 * omit the working copy. Success is the function's own return value.
 * Provenance: INIT-004/SPEC-005
 * @param {object} result
 * @returns {{ content: object[], isError?: boolean }}
 */
function planToolResult(result) {
    if (!result || typeof result !== 'object' || result.error) {
        const body = { error: result && result.error ? result.error : 'invalid_op' };
        if (result && result.revision != null) body.revision = result.revision;
        if (result && typeof result.path === 'string') body.path = result.path;
        if (result && Array.isArray(result.blocks)) body.blocks = result.blocks;
        if (result && Array.isArray(result.warnings)) body.warnings = result.warnings;
        if (result && Array.isArray(result.informs)) body.informs = result.informs;
        return { isError: true, content: [{ type: 'text', text: JSON.stringify(body) }] };
    }
    return text(result);
}

/**
 * Authenticate, then call one plan function. The token and the plan body are not logged.
 * @param {() => object} fn
 * @returns {{ content: object[], isError?: boolean }}
 */
function callPlanTool(fn) {
    try {
        requireMcpAuthToken();
        return planToolResult(fn());
    } catch (err) {
        return failure(err);
    }
}

/**
 * Arguments shared with the HTTP plan body: path, expectedRevision, and the
 * operation fields. Unknown arguments are not forwarded.
 * @param {object} args
 * @param {string[]} keys
 * @returns {object}
 */
function planInput(args, keys) {
    const input = {};
    const src = args || {};
    if (src.path === '' || src.path === '/' || typeof src.path === 'string') input.path = src.path;
    if (Object.prototype.hasOwnProperty.call(src, 'expectedRevision')) {
        input.expectedRevision = src.expectedRevision;
    }
    for (const key of keys) {
        if (Object.prototype.hasOwnProperty.call(src, key)) input[key] = src[key];
    }
    return input;
}

/** Actor stored on every MCP upload or generation. Callers cannot choose another. */
const MCP_MEDIA_ACTOR = 'mcp';

/** Base64 character cap. Decoded bytes still have to fit ingest's 5242880 limit. */
const MAX_MEDIA_BASE64_CHARS = 7000000;

/**
 * Options the tests inject so planImage and generateImage use a fake transport.
 * Production leaves this null. The services then use their own transport and ingest.
 * Provenance: INIT-004/SPEC-012
 * @type {{ transport?: Function, ingest?: Function, now?: Date|number|string }|null}
 */
let injectedMediaOptions = null;

/**
 * Install or clear the fake transport (and optional ingest or clock) for media tools.
 * @param {{ transport?: Function, ingest?: Function, now?: Date|number|string }|null} options
 */
function _setMediaCallOptions(options) {
    if (!options || typeof options !== 'object') {
        injectedMediaOptions = null;
        return;
    }
    /** @type {{ transport?: Function, ingest?: Function, now?: Date|number|string }} */
    const next = {};
    if (typeof options.transport === 'function') next.transport = options.transport;
    if (typeof options.ingest === 'function') next.ingest = options.ingest;
    if (options.now != null) next.now = options.now;
    injectedMediaOptions = next;
}

/**
 * @returns {string}
 */
function configuredApiKey() {
    const value = process.env.OPENAI_API_KEY;
    if (typeof value !== 'string') return '';
    return value.trim();
}

/**
 * True when text contains the live API key, the MCP token, or a key-shaped token.
 * @param {string} message
 * @returns {boolean}
 */
function messageMentionsSecret(message) {
    const key = configuredApiKey();
    const token = process.env.MCP_AUTH_TOKEN;
    if (key && message.includes(key)) return true;
    if (typeof token === 'string' && token && message.includes(token)) return true;
    return /sk-[A-Za-z0-9_-]{8,}/.test(message);
}

/**
 * Error text safe to return to the MCP client. The API key is never included.
 * @param {unknown} err
 * @returns {string}
 */
function safeErrorMessage(err) {
    const message = err && err.message ? String(err.message) : 'The image request failed.';
    if (messageMentionsSecret(message)) return 'The image request failed.';
    return message;
}

/**
 * @param {unknown} err
 * @returns {{ content: object[], isError: boolean }}
 */
function mediaFailure(err) {
    const message = safeErrorMessage(err);
    const code = err && typeof err.code === 'string' ? err.code : '';
    if (!code || messageMentionsSecret(code)) {
        return { isError: true, content: [{ type: 'text', text: message }] };
    }
    return {
        isError: true,
        content: [{ type: 'text', text: JSON.stringify({ error: code, message }) }]
    };
}

/**
 * Drop anything the services may attach besides the fields the tool may show.
 * Prompt text, bytes, and provider secrets are not copied.
 * @param {object} value
 * @returns {object}
 */
function publicMediaResult(value) {
    if (!value || typeof value !== 'object') return value;
    if (Object.prototype.hasOwnProperty.call(value, 'estimatedDollars')) {
        return { count: value.count, estimatedDollars: value.estimatedDollars };
    }
    return {
        id: value.id,
        status: value.status,
        storedName: value.storedName,
        altText: value.altText,
        width: value.width,
        height: value.height
    };
}

/**
 * @param {object} value
 * @returns {object}
 */
function scrubMediaResult(value) {
    const pub = publicMediaResult(value);
    const encoded = JSON.stringify(pub);
    if (messageMentionsSecret(encoded)) {
        throw Object.assign(new Error('The image request failed.'), { code: 'request_failed' });
    }
    return pub;
}

/**
 * Decode inline image bytes. A filesystem path is not read.
 * @param {unknown} value
 * @returns {Buffer}
 */
function imageBytesFromArg(value) {
    if (Buffer.isBuffer(value)) {
        if (value.length === 0 || value.length > ingest.MAX_INPUT_BYTES) {
            throw new Error('Image bytes are required.');
        }
        return value;
    }
    if (value instanceof Uint8Array) {
        return imageBytesFromArg(Buffer.from(value));
    }
    if (typeof value !== 'string' || value.length === 0 || value.length > MAX_MEDIA_BASE64_CHARS) {
        throw new Error('Image bytes are required.');
    }
    const compact = value.replace(/\s+/g, '');
    if (!/^[A-Za-z0-9+/]+={0,2}$/.test(compact)) {
        throw new Error('Image bytes are required.');
    }
    const bytes = Buffer.from(compact, 'base64');
    if (bytes.length === 0 || bytes.length > ingest.MAX_INPUT_BYTES) {
        throw new Error('Image bytes are required.');
    }
    return bytes;
}

/**
 * Options forwarded into the image service. Empty in production.
 * @returns {{ transport?: Function, ingest?: Function, now?: Date|number|string }|undefined}
 */
function serviceCallOptions() {
    if (!injectedMediaOptions) return undefined;
    return injectedMediaOptions;
}

/**
 * Audit one successful media action. The prompt is not copied into the row.
 * @param {string} action
 * @param {string} mediaId
 */
function auditMedia(action, mediaId) {
    audit.append({
        actor: MCP_MEDIA_ACTOR,
        action: action,
        target: mediaId,
        detail: null
    });
}

/**
 * Authenticate, then call ingest or the image service. Replies never include the API key.
 * @param {() => Promise<object>} fn
 * @returns {Promise<{ content: object[], isError?: boolean }>}
 */
async function callMediaTool(fn) {
    try {
        requireMcpAuthToken();
        return text(scrubMediaResult(await fn()));
    } catch (err) {
        return mediaFailure(err);
    }
}

function register(server, name, meta, handler) {
    toolHandlers[name] = handler;
    server.registerTool(name, meta, handler);
}

/** Words `add_library_words` may add for one learner on one local day. ADR-0009 §1. */
const LIBRARY_DAILY_CAP = 50;

/**
 * @param {number} ms
 * @returns {string}
 */
function localDayKey(ms) {
    const d = new Date(ms);
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return d.getFullYear() + '-' + month + '-' + day;
}

/**
 * @param {unknown} iso
 * @param {string} dayKey
 * @returns {boolean}
 */
function addedOnLocalDay(iso, dayKey) {
    const t = Date.parse(String(iso == null ? '' : iso));
    if (!Number.isFinite(t)) return false;
    return localDayKey(t) === dayKey;
}

/**
 * @param {Error|unknown} err
 * @returns {boolean}
 */
function isTokenError(err) {
    return Boolean(err && typeof err.message === 'string' && err.message.includes('MCP_AUTH_TOKEN'));
}

/**
 * Tool refusal. The text is JSON and nothing else, so a Zod message cannot
 * echo the caller's text or a profile.
 * @param {object} body
 * @returns {{ isError: true, content: object[] }}
 */
function refusal(body) {
    return { isError: true, content: [{ type: 'text', text: JSON.stringify(body) }] };
}

/**
 * Token first. A schema failure is `invalid_input` with no echo of the value.
 * @param {() => object} fn
 * @returns {{ content: object[], isError?: boolean }}
 */
function callLearner(fn) {
    try {
        requireMcpAuthToken();
        return fn();
    } catch (err) {
        if (isTokenError(err)) return failure(err);
        return refusal({ error: 'invalid_input' });
    }
}

/**
 * @param {object|null|undefined} entry
 * @returns {object|null|undefined}
 */
function publicLexiconEntry(entry) {
    if (!entry || typeof entry !== 'object') return entry == null ? null : entry;
    const copy = Object.assign({}, entry);
    if (Object.prototype.hasOwnProperty.call(copy, 'verified')) {
        copy.verified = copy.verified === true || copy.verified === 1;
    }
    delete copy.verified_by;
    return copy;
}

/**
 * @param {object} paradigm
 * @returns {object}
 */
function publicParadigm(paradigm) {
    const out = {};
    const source = paradigm || {};
    for (const tense of Object.keys(source)) {
        out[tense] = {};
        const cells = source[tense] || {};
        for (const cell of Object.keys(cells)) {
            out[tense][cell] = publicLexiconEntry(cells[cell]);
        }
    }
    return out;
}

/**
 * @param {string|null} username
 * @param {string} outcome
 * @param {number} count
 */
function auditLibraryAdd(username, outcome, count) {
    audit.append({
        actor: 'mcp',
        action: 'library_add',
        target: username == null ? null : String(username),
        detail: JSON.stringify({ outcome, count })
    });
}

const USERNAME = z.string().min(1).max(64);
const LEX_KEY = z.string().min(1).max(60);
const SHORT = z.string().max(32);
const lexiconEntrySchema = z.object({
    key: LEX_KEY,
    kind: z.string().max(16).optional(),
    lemma_key: LEX_KEY.optional(),
    lemmaKey: LEX_KEY.optional(),
    gloss_en: z.string().max(120).optional(),
    glossEn: z.string().max(120).optional(),
    pos: SHORT.optional(),
    root: SHORT.optional(),
    binyan: SHORT.optional(),
    root_class: SHORT.optional(),
    rootClass: SHORT.optional(),
    tense: SHORT.optional(),
    cell: z.string().max(16).optional(),
    translit: z.string().max(80).optional(),
    accept: z.array(z.string().max(60)).max(8).optional()
});

/**
 * Profile, coverage, lexicon, and library tools. Each one calls a service.
 * Verification and the AI-planning switch are registered only to refuse.
 * Provenance: INIT-006/SPEC-010
 * @param {{ registerTool: Function }} server
 */
function registerLearnerTools(server) {
    register(server, 'get_learner_profile', {
        title: 'Read a learner profile',
        description: [
            'Read one learner\'s progress summary while you draft their next lesson.',
            'Call it only for that learner. planning_off means an admin has not turned AI planning on: stop, do not retry, and do not invent a profile.',
            'The profile has no glosses. Use get_lexicon_entry for a word\'s meaning.',
            'Never paste the profile, or any part of it, into a lesson file, a worksheet, or a saved plan.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            username: USERNAME.describe('The learner\'s account username.')
        },
        annotations: { readOnlyHint: true }
    }, async (args) => callLearner(() => {
        const parsed = z.object({ username: USERNAME }).parse(args || {});
        const result = profile.buildProfile(parsed.username, { audience: 'mcp' });
        if (result && typeof result.error === 'string') return refusal({ error: result.error });
        return text(result);
    }));

    register(server, 'check_text_coverage', {
        title: 'Check known-word coverage',
        description: [
            'Score a Hebrew text against one learner\'s library before you use it as a reading.',
            'text is at most 4000 characters. The result names unknown tokens from that text.',
            'planning_off means AI planning is off for that learner: nothing was scored.',
            'Never paste the learner profile into a lesson file.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            username: USERNAME.describe('The learner\'s account username.'),
            text: z.string().max(4096).describe('Hebrew text to score. At most 4000 characters are accepted.')
        },
        annotations: { readOnlyHint: true }
    }, async (args) => callLearner(() => {
        const parsed = z.object({
            username: USERNAME,
            text: z.string().max(4096)
        }).parse(args || {});
        const result = coverage.checkCoverage(parsed.username, parsed.text, { audience: 'mcp' });
        if (result && typeof result.error === 'string') return refusal({ error: result.error });
        return text(result);
    }));

    register(server, 'list_grammar_concepts', {
        title: 'List grammar concepts',
        description: [
            'List the grammar concepts the player tracks: key, family, and labels.',
            'Call this when a lesson or a drill needs a concept id. This is not learner data.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        annotations: { readOnlyHint: true }
    }, async () => callLearner(() => {
        const concepts = conceptsRepo.list().map((row) => ({
            key: row.key,
            family: row.family,
            label_en: row.label_en,
            label_he: row.label_he,
            colour: row.colour,
            sort: row.sort
        }));
        return text({ concepts });
    }));

    register(server, 'list_lexicon', {
        title: 'Search the lexicon',
        description: [
            'Search lexicon entries by gloss, part of speech, binyan, tense, or root class.',
            'Call this before you cite a word or add it to a learner\'s library.',
            'Unverified forms are included. You cannot mark a form verified.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            q: z.string().max(80).optional().describe('Gloss or key fragment.'),
            pos: SHORT.optional().describe('Part of speech, such as noun or verb.'),
            binyan: SHORT.optional().describe('Binyan, such as paal.'),
            tense: SHORT.optional().describe('Tense, such as past.'),
            rootClass: SHORT.optional().describe('Root class, such as regular.'),
            verified: z.union([z.boolean(), z.literal(0), z.literal(1)]).optional()
                .describe('Filter by the person\'s verified flag. You cannot change it.'),
            limit: z.number().int().min(1).max(200).optional(),
            cursor: LEX_KEY.optional().describe('Key of the last entry on the previous page.')
        },
        annotations: { readOnlyHint: true }
    }, async (args) => callLearner(() => {
        const parsed = z.object({
            q: z.string().max(80).optional(),
            pos: SHORT.optional(),
            binyan: SHORT.optional(),
            tense: SHORT.optional(),
            rootClass: SHORT.optional(),
            verified: z.union([z.boolean(), z.literal(0), z.literal(1)]).optional(),
            limit: z.number().int().min(1).max(200).optional(),
            cursor: LEX_KEY.optional()
        }).parse(args || {});
        const result = lexicon.list({
            q: parsed.q,
            pos: parsed.pos,
            binyan: parsed.binyan,
            tense: parsed.tense,
            rootClass: parsed.rootClass,
            verified: parsed.verified,
            limit: parsed.limit,
            cursor: parsed.cursor
        });
        if (!result.ok) return refusal({ error: result.error || 'invalid_filter' });
        return text({
            entries: result.entries.map(publicLexiconEntry),
            nextCursor: result.nextCursor
        });
    }));

    register(server, 'get_lexicon_entry', {
        title: 'Read one lexicon entry',
        description: [
            'Read one lexicon entry by its exact key, with its lemma and paradigm.',
            'Call this to join a profile word_id to a gloss. The profile itself must not be pasted into a lesson file.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            key: LEX_KEY.describe('Lexicon key, the same string as a profile word_id.')
        },
        annotations: { readOnlyHint: true }
    }, async (args) => callLearner(() => {
        const parsed = z.object({ key: LEX_KEY }).parse(args || {});
        const found = lexicon.get(parsed.key);
        if (!found) return refusal({ error: 'not_found' });
        return text({
            entry: publicLexiconEntry(found.entry),
            lemma: publicLexiconEntry(found.lemma),
            paradigm: publicParadigm(found.paradigm)
        });
    }));

    register(server, 'upsert_lexicon_entries', {
        title: 'Write lexicon entries',
        description: [
            'Write lexicon entries. Call this before add_library_words when a key is not in the lexicon yet.',
            'Rows are stored with source mcp. A verified field on an entry is ignored: you cannot mark a form verified.',
            'Pass expectedRev 0 for a new key, or the current rev when you edit.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            entries: z.array(lexiconEntrySchema).min(1).max(200),
            expectedRev: z.number().int().min(0).max(1000000)
        }
    }, async (args) => callLearner(() => {
        const parsed = z.object({
            entries: z.array(lexiconEntrySchema).min(1).max(200),
            expectedRev: z.number().int().min(0).max(1000000)
        }).parse(args || {});
        const result = lexicon.upsert(parsed.entries, {
            actor: 'mcp',
            expectedRev: parsed.expectedRev
        });
        if (!result.ok) {
            const body = { error: result.error || 'invalid_entry' };
            if (Array.isArray(result.errors)) body.errors = result.errors;
            if (Array.isArray(result.conflicts)) body.conflicts = result.conflicts;
            return refusal(body);
        }
        return text({
            entries: result.entries.map(publicLexiconEntry)
        });
    }));

    register(server, 'add_library_words', {
        title: 'Add lexicon words to a learner library',
        description: [
            'Add lexicon keys to one learner\'s library so they can review them.',
            'planning_off means AI planning is off: nothing was written.',
            'Every key must already be a lexicon entry. unknown_keys means the whole call was refused;',
            'write those entries with upsert_lexicon_entries first, then retry.',
            'At most 50 new words per learner per local day.',
            'Never paste the learner profile into a lesson file.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            username: USERNAME.describe('The learner\'s account username.'),
            words: z.array(LEX_KEY).min(1).max(50).describe('Lexicon keys only. Not glosses.')
        }
    }, async (args) => callLearner(() => addLibraryWords(args)));

    register(server, 'set_lexicon_verified', {
        title: 'Mark a lexicon form verified',
        description: [
            'Do not call. Marking a form verified is a person\'s act, done by an admin.',
            'This tool always returns forbidden and writes nothing.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            key: LEX_KEY.optional(),
            verified: z.boolean().optional()
        }
    }, async () => callLearner(() => refusal({ error: 'forbidden' })));

    register(server, 'set_ai_planning', {
        title: 'Change the AI-planning switch',
        description: [
            'Do not call. Only an admin can change AI planning, from the admin page.',
            'This tool always returns forbidden and does not change the switch.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            username: USERNAME.optional(),
            enabled: z.boolean().optional()
        }
    }, async () => callLearner(() => refusal({ error: 'forbidden' })));
}

/**
 * Switch-gated library add. Keys must already be lexicon entries.
 * The daily cap is applied before `library.add`. The server clock picks the day.
 * @param {object} args
 * @returns {{ content: object[], isError?: boolean }}
 */
function addLibraryWords(args) {
    const parsed = z.object({
        username: USERNAME,
        words: z.array(LEX_KEY).min(1).max(50)
    }).parse(args || {});
    const username = parsed.username;
    const row = usersRepo.findLearnerByUsername(username);
    if (!row || row.status !== 'active') {
        auditLibraryAdd(row ? row.username : null, 'not_found', 0);
        return refusal({ error: 'not_found' });
    }
    if (Number(row.ai_planning) !== 1) {
        auditLibraryAdd(row.username, 'planning_off', 0);
        return refusal({ error: 'planning_off' });
    }

    const accepted = [];
    const unknown = [];
    const seen = new Set();
    const seenUnknown = new Set();
    for (const raw of parsed.words) {
        const found = lexicon.get(raw);
        if (!found || !found.entry) {
            if (!seenUnknown.has(raw)) {
                seenUnknown.add(raw);
                unknown.push(raw);
            }
            continue;
        }
        const key = found.entry.key;
        if (seen.has(key)) continue;
        seen.add(key);
        accepted.push(key);
    }
    if (unknown.length) {
        auditLibraryAdd(row.username, 'unknown_keys', 0);
        return refusal({ error: 'unknown_keys', unknown_keys: unknown });
    }

    const ownedRows = userWordsRepo.listByUser(row.id);
    const owned = new Set(ownedRows.map((item) => item.word));
    const day = localDayKey(Date.now());
    let addedToday = 0;
    for (const item of ownedRows) {
        if (item.source === 'ai' && addedOnLocalDay(item.added_at, day)) addedToday += 1;
    }
    const fresh = accepted.filter((key) => !owned.has(key));
    if (addedToday + fresh.length > LIBRARY_DAILY_CAP) {
        auditLibraryAdd(row.username, 'daily_cap', 0);
        return refusal({ error: 'daily_cap' });
    }

    let result;
    try {
        result = library.add(row.id, accepted.map((key) => ({ key })), { source: 'ai' });
    } catch (_err) {
        auditLibraryAdd(row.username, 'error', 0);
        return refusal({ error: 'error' });
    }
    if (!result || !result.ok) {
        const code = result && result.error ? result.error : 'bad_request';
        auditLibraryAdd(row.username, code, 0);
        return refusal({ error: code });
    }
    auditLibraryAdd(row.username, 'ok', result.created);
    return text({ words: result.words, created: result.created });
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

    // INIT-004/SPEC-005: same functions as the plan HTTP routes. Adapters only.
    register(server, 'read_part', {
        title: 'Read one part of a lesson plan',
        description: 'Read the plan part at an id path. An empty path is the plan document. Requires MCP_AUTH_TOKEN. Returns the part and the current revision. Does not change the plan.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path such as units/unit-1/lessons/cafe-order/blocks/introduction/words/2. Empty string is the whole plan.')
        },
        annotations: { readOnlyHint: true }
    }, async (args) => callPlanTool(() => planOps.readPart(planStore, args && args.id, planInput(args, []))));

    register(server, 'replace_part', {
        title: 'Replace one part of a lesson plan',
        description: 'Replace the part at an id path. Pass path, value, and expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision. The plan is unchanged.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the part to replace. Empty string replaces the plan document.'),
            value: z.any().describe('Replacement object for that part.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args) => callPlanTool(() => planOps.replacePart(
        planStore,
        args && args.id,
        planInput(args, ['value'])
    )));

    register(server, 'insert_block', {
        title: 'Insert a block into a lesson plan',
        description: 'Insert a block into a lesson at index. Omit index to append. Pass path (the lesson), block, and expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the lesson, such as units/unit-1/lessons/cafe-order.'),
            index: z.number().int().optional().describe('Destination index. Omit to append.'),
            block: z.any().describe('Block object to insert.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args) => callPlanTool(() => planOps.insertBlock(
        planStore,
        args && args.id,
        planInput(args, ['index', 'block'])
    )));

    register(server, 'move_block', {
        title: 'Move a block in a lesson plan',
        description: 'Move the block at path to index within its lesson. Pass expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the block.'),
            index: z.number().int().describe('Destination index within the lesson.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args) => callPlanTool(() => planOps.moveBlock(
        planStore,
        args && args.id,
        planInput(args, ['index'])
    )));

    register(server, 'remove_block', {
        title: 'Remove a block from a lesson plan',
        description: 'Remove the block at path. Pass expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the block to remove.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args) => callPlanTool(() => planOps.removeBlock(
        planStore,
        args && args.id,
        planInput(args, [])
    )));

    register(server, 'set_field', {
        title: 'Set one field on a lesson plan part',
        description: 'Set field to value on the part at path. Pass expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision. The plan is unchanged.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the part. Empty string is the plan document.'),
            field: z.string().describe('Field name on that part.'),
            value: z.any().describe('New value.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args) => callPlanTool(() => planOps.setField(
        planStore,
        args && args.id,
        planInput(args, ['field', 'value'])
    )));

    register(server, 'publish_plan', {
        title: 'Publish a lesson plan',
        description: 'Publish the working copy. Pass expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision. The plan is unchanged.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args) => callPlanTool(() => planPublish.publishPlan(
        planStore,
        args && args.id,
        planInput(args, [])
    )));

    // INIT-004/SPEC-012: same ingest and image service as the HTTP routes. No second client.
    register(server, 'upload_media', {
        title: 'Upload an image into the media library',
        description: [
            'Store one PNG, JPEG, or WebP image as pending media.',
            'Pass id and base64 bytes. Filename and content type are ignored.',
            'Requires MCP_AUTH_TOKEN. This tool does not approve the image.'
        ].join(' '),
        inputSchema: {
            id: z.string().describe('Media id: 1-64 lowercase letters, digits, and single hyphens.'),
            bytes: z.string().max(MAX_MEDIA_BASE64_CHARS).describe('Base64 image bytes. A file path is not accepted.'),
            altText: z.string().optional().describe('English alt-text draft. Empty alt cannot be approved later.')
        }
    }, async (args) => callMediaTool(async () => {
        const stored = await ingest.ingestImage({
            bytes: imageBytesFromArg(args && args.bytes),
            id: args && args.id,
            actor: MCP_MEDIA_ACTOR,
            altText: args && args.altText
        });
        auditMedia('media_upload', stored.id);
        return stored;
    }));

    register(server, 'plan_image', {
        title: 'Dry-run an image generation cost',
        description: [
            'Return the image count and the estimated US dollars.',
            'Does not call the image provider and does not spend a generation cap.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            prompt: z.string().describe('One ordinary object or action.'),
            count: z.number().int().optional().describe('How many images the estimate covers. Omit for one.')
        },
        annotations: { readOnlyHint: true }
    }, async (args) => callMediaTool(() => {
        const input = { prompt: args && args.prompt };
        if (args && args.count != null) input.count = args.count;
        return images.planImage(input, serviceCallOptions());
    }));

    register(server, 'generate_image', {
        title: 'Generate one image',
        description: [
            'Generate one image only when confirm is true and estimate matches the dry-run.',
            'Uses the pinned model. Without confirm, the provider is not called.',
            'Requires MCP_AUTH_TOKEN. The image stays pending. This tool does not approve it.'
        ].join(' '),
        inputSchema: {
            prompt: z.string().describe('The same prompt that was dry-run.'),
            confirm: z.boolean().describe('Must be true. Any other value does not call the provider.'),
            estimate: z.number().describe('Dollar estimate from plan_image for one image.'),
            id: z.string().describe('Media id for the pending image.'),
            altText: z.string().optional().describe('English alt-text draft.')
        }
    }, async (args) => callMediaTool(async () => {
        const stored = await images.generateImage({
            prompt: args && args.prompt,
            confirm: args ? args.confirm === true : false,
            estimate: args && args.estimate,
            actor: MCP_MEDIA_ACTOR,
            id: args && args.id,
            altText: args && args.altText
        }, serviceCallOptions());
        auditMedia('media_generate', stored.id);
        return stored;
    }));

    registerLearnerTools(server);
}

module.exports = {
    registerLessonTools,
    registerLearnerTools,
    formatUpsertHead,
    toolHandlers,
    _setMediaCallOptions,
    LIBRARY_DAILY_CAP
};
