const { z } = require('zod');
const oauth = require('./oauth');
const store = require('./store');
const { CATALOG, LEVEL_DEFAULTS } = require('../shared/activities/catalog');
const { generateLessonAudio, formatAudioSummary } = require('../server/audio');
const { rebuildVocabIndexSafe } = require('../server/vocab-index');
const accounts = require('../server/accounts');
const visibility = require('../server/visibility');
const visibilityRepo = require('../server/db/repos/visibility');
const usersRepo = require('../server/db/repos/users');
const { resolveActiveUserIds } = require('../server/username-list');
const fs = require('fs');
const path = require('path');
const { lessonDir } = require('../server/paths');
const planStore = require('../server/plans/store');
const planOps = require('../server/plans/ops');
const planPublish = require('../server/plans/publish');
const planCreate = require('../server/plans/create');
const planRevisions = require('../server/plans/revisions');
const planIssues = require('../server/plans/issues');
const planPersonal = require('../server/plans/personal');
const planDelete = require('../server/plans/delete');
const authoring = require('../server/authoring');
const { makeContext } = require('../server/plans/context');
const ingest = require('../server/media/ingest');
const images = require('../server/media/openai-images');
const audit = require('../server/db/repos/audit');
const profile = require('../server/learner/profile');
const coverage = require('../server/learner/coverage');
const glossDraft = require('../server/dictionary/suggest-glosses');
const wordAudio = require('../server/word-audio');
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

/** `field` and `reason` on a refusal are fixed identifiers from the services, never caller text. */
function isErrorWord(value) {
    return typeof value === 'string' && /^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(value);
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
        // Which input was refused, and why a restore could not place a part (INIT-008/SPEC-009).
        if (result && isErrorWord(result.field)) body.field = result.field;
        if (result && isErrorWord(result.reason)) body.reason = result.reason;
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

/**
 * Forward only the named arguments, and only those that were given. Nothing
 * else a caller sends (an `actor`, a `kind`) reaches a service.
 * Provenance: INIT-008/SPEC-009
 * @param {object} args
 * @param {string[]} keys
 * @returns {object}
 */
function planArgs(args, keys) {
    const input = {};
    const src = args && typeof args === 'object' ? args : {};
    for (const key of keys) {
        if (Object.prototype.hasOwnProperty.call(src, key) && src[key] !== undefined) input[key] = src[key];
    }
    return input;
}

/**
 * Write context for a plan tool call (ADR-0011 §1). The MCP is always actor
 * `ai` named `mcp`: the actor comes from the channel and no tool argument can
 * set it. `tool` is the tool name. `requestId` is the JSON-RPC id the SDK passes
 * in the handler's second argument, a client-chosen label and not an identity.
 * The SDK does not give a handler the clientInfo. When the OAuth token was issued
 * to an app user, that username is the `client` label. The actor stays `mcp`.
 * Provenance: INIT-008/SPEC-002
 * @param {string} tool
 * @param {{ requestId?: string|number }} [extra]
 * @returns {object}
 */
function mcpContext(tool, extra) {
    const input = { channel: 'mcp', tool: tool };
    const user = oauth.currentUser();
    if (user && user.username) input.client = user.username;
    if (extra && (typeof extra.requestId === 'string' || typeof extra.requestId === 'number')) {
        input.requestId = extra.requestId;
    }
    return makeContext(input);
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
        return text({ concepts: lexicon.listGrammarConcepts() });
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

const PLAN_ID_SCHEMA = z.string().min(1).max(64);
const PLAN_PATH_SCHEMA = z.string().max(1024);
const PLAN_REVISION_SCHEMA = z.number().int().min(1).max(1000000000);
const PLAN_EXPECTED_SCHEMA = z.number().int().min(0).max(1000000000);
const PLAN_REF_SCHEMA = z.union([PLAN_REVISION_SCHEMA, z.enum(['working', 'published'])]);
const PLAN_LIMIT_SCHEMA = z.number().int().min(1).max(200);

/**
 * Plan-builder tools (INIT-008/SPEC-009). Each handler is an adapter: token
 * check, forward the named arguments, build the `ai` context, call one function
 * in server/plans/, shape the result. The services validate their own input, so
 * a handler never edits plan JSON and never reads an actor from an argument.
 * Each function is looked up on its module at call time, as the plan tools above.
 * Provenance: INIT-008/SPEC-009
 * @param {{ registerTool: Function }} server
 */
function registerPlanBuilderTools(server) {
    register(server, 'list_plans', {
        title: 'List lesson plans',
        description: [
            'List every lesson plan: id, title, audience, status, revision, updatedAt, published, unpublishedChanges, and humanEditedFields.',
            'Call this first to find a plan id. Newest update first, at most 200 (truncated says there are more).',
            'published means a snapshot exists and learners can read it; unpublishedChanges means the working copy is ahead of that snapshot.',
            'humanEditedFields counts the fields a person last changed. If it is above 0, read_part shows them in editState.',
            'No assignee name is returned, only assigned (true or false).',
            'Does not change anything. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            limit: PLAN_LIMIT_SCHEMA.optional().describe('At most this many plans. Default and maximum 200.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => planCreate.listPlans(
        planStore,
        planArgs(args, ['limit']),
        mcpContext('list_plans', extra)
    )));

    register(server, 'create_plan', {
        title: 'Create a lesson plan',
        description: [
            'Make a new draft plan at revision 1 from a title and an optional recipe. Omit id: the server assigns a UUID.',
            'An id you pass must already be a lowercase slug. An id that exists returns already_exists and nothing is changed or overwritten.',
            'recipe is one of topic-conversation, review-worksheet, verse-reading, verb-workshop, letters-and-vowels, guide. It sets the lesson recipe and nothing else.',
            'unitId and lessonId are optional slugs for the one unit and one lesson made for you (default unit-1 and lesson-1). A part id cannot be renamed later, so pick them now.',
            'The new plan is shared, its lesson is hidden, and it has one placeholder block (id placeholder, type fluency-pass). It holds no Hebrew.',
            'Next: read_part with an empty path to get the revision, insert your real blocks with insert_block, then remove the placeholder with remove_block (a lesson must keep at least one block).',
            'A draft is invisible to learners until publish_plan. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.optional().describe('Omit this. The server assigns a UUID. Pass one only to retry an id you already chose.'),
            title: z.string().min(1).max(4000).describe('Plan title as plain text, no markup.'),
            recipe: z.string().max(64).optional().describe('Optional lesson recipe name.'),
            unitId: PLAN_ID_SCHEMA.optional().describe('Optional id of the first unit. Default unit-1.'),
            lessonId: PLAN_ID_SCHEMA.optional().describe('Optional id of the first lesson. Default lesson-1.')
        }
    }, async (args, extra) => callPlanTool(() => planCreate.createPlan(
        planStore,
        planArgs(args, ['id', 'title', 'recipe', 'unitId', 'lessonId']),
        mcpContext('create_plan', extra)
    )));

    register(server, 'list_plan_revisions', {
        title: 'List the revision history of a plan',
        description: [
            'List the revisions of one plan, newest first: revision, op, path, field, summary, actorKind, actorName, tool, at.',
            'actorKind is ai, human, or system. A person\'s revisions show actorName human, never a name.',
            'Use sinceRevision with the revision you last read to see what was written after it, and actorKind human to see only what a person changed.',
            'pathPrefix keeps revisions whose path starts with that text. limit defaults to 50 and is at most 200; truncated says there are more, so pass beforeRevision with the oldest revision shown to page back.',
            'A summary never holds a field value. Use get_plan_revision for the whole plan at one revision, or diff_plan for what changed.',
            'Does not change anything. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            sinceRevision: z.number().int().min(0).max(1000000000).optional()
                .describe('Only revisions after this one.'),
            beforeRevision: z.number().int().min(0).max(1000000000).optional()
                .describe('Only revisions before this one. For paging.'),
            actorKind: z.enum(['ai', 'human', 'system']).optional().describe('Only revisions written by this kind of actor.'),
            pathPrefix: PLAN_PATH_SCHEMA.optional().describe('Only revisions whose path starts with this text.'),
            limit: PLAN_LIMIT_SCHEMA.optional().describe('Default 50, at most 200.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => planRevisions.list(
        args && args.id,
        planArgs(args, ['sinceRevision', 'beforeRevision', 'actorKind', 'pathPrefix', 'limit']),
        mcpContext('list_plan_revisions', extra)
    )));

    register(server, 'get_plan_revision', {
        title: 'Read a plan as it was at one revision',
        description: [
            'Read the whole plan document as it was at one revision, with that revision\'s metadata.',
            'Use it to see an old value before you restore it. It does not change the plan or the revision counter.',
            'not_found means the plan or that revision does not exist. A plan that has not been written since history began has no rows before its first edit.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            revision: PLAN_REVISION_SCHEMA.describe('Revision number, from list_plan_revisions.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => planRevisions.get(
        args && args.id,
        args && args.revision,
        mcpContext('get_plan_revision', extra)
    )));

    register(server, 'diff_plan', {
        title: 'Diff a plan between two revisions',
        description: [
            'List what changed between two points of one plan. from and to are each a revision number, working (the draft now), or published (the last snapshot learners read).',
            'Example: from published to working is everything not yet published. from 5 to working is everything since revision 5.',
            'Each entry has path, field, kind (changed, added, removed, or moved), before, after. path is an id path you can pass to read_part, set_field, or restore_plan_revision.',
            'Pass path to keep one part and what is under it. truncated true means the list was cut: narrow it with path.',
            'no_snapshot means the plan was never published. Does not change anything. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            from: PLAN_REF_SCHEMA.describe('Revision number, working, or published.'),
            to: PLAN_REF_SCHEMA.describe('Revision number, working, or published.'),
            path: PLAN_PATH_SCHEMA.optional().describe('Id path: keep only this part and what is under it.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => planRevisions.diff(
        args && args.id,
        args && args.from,
        args && args.to,
        planArgs(args, ['path']),
        mcpContext('diff_plan', extra)
    )));

    register(server, 'restore_plan_revision', {
        title: 'Restore a plan, one part, or one field from a revision',
        description: [
            'Put an older revision back as a NEW revision. History is kept and nothing is erased, so a restore can itself be restored.',
            'Pass fromRevision and expectedRevision (the revision you last read). With only those, the whole plan comes back. With path, one part comes back. With path and field, one field comes back (path is empty for a field of the plan itself).',
            'Workflow: list_plan_revisions or diff_plan to find the revision and the id path, get_plan_revision if you want to see the old value, then restore_plan_revision with the narrowest path and field that fits. Use it to undo your own mistake, or to put back a value a person had set (editState and overwrote name that revision).',
            'Cannot change who a personal plan is assigned to: audience and assigneeId stay as they are (a field restore of either returns assign_required; use assign_plan). The draft keeps its status draft; learners keep reading the last published snapshot until you publish_plan.',
            'A stale expectedRevision returns revision_conflict and the stored revision, and writes nothing. The restored plan is validated like any write; a refusal lists the reasons and writes nothing. not_found carries a reason when the part cannot be placed back (parent_missing or absent_in_revision).',
            'Your result includes overwrote when the restore replaced a field a person last wrote. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            fromRevision: PLAN_REVISION_SCHEMA.describe('The revision to restore from.'),
            path: PLAN_PATH_SCHEMA.optional().describe('Id path of one part. Omit to restore the whole plan.'),
            field: z.string().max(64).optional().describe('One field of that part. Omit to restore the whole part.'),
            expectedRevision: PLAN_EXPECTED_SCHEMA.describe('Revision from the last read. Must match the stored working copy.')
        },
        annotations: { destructiveHint: false }
    }, async (args, extra) => callPlanTool(() => planRevisions.restore(
        planStore,
        args && args.id,
        planInput(args, ['fromRevision', 'field']),
        mcpContext('restore_plan_revision', extra)
    )));

    register(server, 'list_plan_issues', {
        title: 'List the problems the checks find in a plan',
        description: [
            'List what the checks find in the draft of one plan, with a stable code, severity, path, field, message, and a fix where one is known.',
            'severity block is a hard error: it stops publish_plan. warn is advice and never stops a write or a publish. By default you get block and warn; pass inform or all to see more. counts always covers the whole plan.',
            'Codes include pointing-form, field-language, lexicon-form, pointing-consistency, and audio-stale, plus the older rules. A fix is { action, path, field, suggested } where action is set_field or replace_part: apply it with that tool, or choose your own repair.',
            'path is an id path for read_part and set_field. pointer is the raw JSON pointer.',
            'Call this after you build or change a plan and before publish_plan, then fix what you can. Does not change anything. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            severity: z.union([
                z.enum(['block', 'warn', 'inform', 'all']),
                z.array(z.enum(['block', 'warn', 'inform'])).min(1).max(3)
            ]).optional().describe('block, warn, inform, all, or a list of them. Default block and warn.'),
            code: z.string().max(64).optional().describe('Only this rule code.'),
            pathPrefix: PLAN_PATH_SCHEMA.optional().describe('Only issues at or under this id path.'),
            limit: PLAN_LIMIT_SCHEMA.optional().describe('Default 100, at most 200.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => planIssues.list(
        planStore,
        args && args.id,
        planArgs(args, ['severity', 'code', 'pathPrefix', 'limit']),
        mcpContext('list_plan_issues', extra)
    )));

    register(server, 'assign_plan', {
        title: 'Assign a plan to everyone or to one learner',
        description: [
            'Set who a plan is for. audience is shared (every learner who may see its lessons) or personal (one learner). For personal pass assigneeId, the username of an active account (see list_users). For shared leave assigneeId out.',
            'Pass expectedRevision, the revision you last read. This is the only way to change audience or assigneeId: set_field, replace_part, and restore_plan_revision cannot.',
            'assignee_refused means that account is unknown or disabled; nothing was written. A stale expectedRevision returns revision_conflict and the stored revision.',
            'Learners see a plan only after publish_plan. A personal plan is shown only to its assignee. Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            audience: z.enum(['shared', 'personal']).describe('shared or personal.'),
            assigneeId: z.string().min(1).max(64).optional().describe('Username of an active account. Required for personal.'),
            expectedRevision: PLAN_EXPECTED_SCHEMA.describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planPersonal.assign(
        planStore,
        args && args.id,
        planInput(args, ['audience', 'assigneeId']),
        mcpContext('assign_plan', extra)
    )));
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
    register(server, 'delete_lesson', {
        title: 'Delete a worksheet lesson',
        description: [
            'Remove a worksheet lesson from the shelf. Pass slug.',
            'The lesson folder, including its recordings, is moved to an archive and is not deleted.',
            'Visibility and grants for that slug are cleared.',
            'A plan with the same id is left in place. delete_plan removes a plan.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            slug: z.string().describe('Lesson folder name, such as shai-date-indirect-speech.')
        }
    }, async (args) => {
        try {
            requireMcpAuthToken();
            return text(authoring.deleteLesson('mcp', args && args.slug));
        } catch (err) {
            return failure(err);
        }
    });

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
        description: 'Read the plan part at an id path. An empty path is the plan document. Requires MCP_AUTH_TOKEN. Returns the part, the current revision, and editState. editState lists the fields at or below that path that a person last changed (path, field, revision, previousValue, previousWriter), and lastHumanEdit is the newest person edit anywhere in the plan. It is advice: you may edit those fields anyway, and the note never blocks a write. If you overwrite one, your write result lists it in overwrote, and restore_plan_revision puts the person\'s value back. Read a part before you edit it. Use list_plans to find a plan id and create_plan to start one. Does not change the plan.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path such as units/unit-1/lessons/cafe-order/blocks/introduction/words/2. Empty string is the whole plan.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => planOps.readPart(
        planStore,
        args && args.id,
        planInput(args, []),
        mcpContext('read_part', extra)
    )));

    register(server, 'suggest_glosses', {
        title: 'Draft word meanings for a graded story',
        description: 'Draft an English meaning for every word in a graded-story block that has no gloss yet. Pass the plan id and the id path of the block, for example units/unit-1/lessons/cafe-order/blocks/story. Requires MCP_AUTH_TOKEN. The meanings come from the app dictionary and are a first draft: some are the base word of a form (form says which), some are a different sense from the one the story uses, and unmatched lists words with no entry. Read each one against the story, fix it, then save the whole map with set_field on the block (field glosses, at most 80 words). A reader sees these meanings when they tap a word, so a wrong one teaches the wrong thing. Does not change the plan.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of a graded-story block.')
        },
        annotations: { readOnlyHint: true }
    }, async (args, extra) => callPlanTool(() => {
        const read = planOps.readPart(
            planStore,
            args && args.id,
            planInput(args, []),
            mcpContext('suggest_glosses', extra)
        );
        if (!read || read.error) return read;
        const block = read.part;
        if (!block || block.type !== 'graded-story') return { error: 'not_a_story', path: args.path };
        return Object.assign({ revision: read.revision }, glossDraft.suggestForBlock(block));
    }));

    register(server, 'make_word_audio', {
        title: 'Make sound for Hebrew words',
        description: 'Make (or reuse) the spoken clip for up to 20 Hebrew words and return each word\'s audioPath. Requires MCP_AUTH_TOKEN. Pass the words as written in the plan, with vowel points: the clip is for that exact spelling, and points are what make the voice say it right. A word that already has a clip is reused and costs nothing; a new one is made once with ElevenLabs and kept for every lesson, never overwritten. This does not change a plan: save each audioPath with set_field on the word entry (field audioPath). Words with no clip and an error (not_configured, bad_word, failed) are listed in the same order.',
        inputSchema: {
            words: z.array(z.string()).min(1).max(20).describe('Hebrew words, with vowel points, as in the plan.')
        },
        annotations: { idempotentHint: true }
    }, async (args) => {
        try {
            requireMcpAuthToken();
            const words = Array.isArray(args && args.words) ? args.words : [];
            const clips = [];
            for (const word of words) {
                try {
                    let clip = wordAudio.findWordClip(word);
                    if (!clip) clip = await wordAudio.ensureWordClip(word);
                    clips.push({ word: clip.word, audioPath: clip.audioPath, created: clip.created });
                } catch (err) {
                    const code = err && err.code;
                    clips.push({
                        word: typeof word === 'string' ? word : '',
                        error: code === 'bad_word' ? 'bad_word' : (code === 'no_key' ? 'not_configured' : 'failed')
                    });
                }
            }
            return planToolResult({ clips });
        } catch (err) {
            return failure(err);
        }
    });

    register(server, 'replace_part', {
        title: 'Replace one part of a lesson plan',
        description: 'Replace the part at an id path. Pass path, value, and expectedRevision. Requires MCP_AUTH_TOKEN. The value\'s id must equal the id in the path (id_mismatch otherwise), and audience and assigneeId cannot change here (assign_required; use assign_plan). A stale expectedRevision returns revision_conflict and the stored revision, and writes nothing. The result lists overwrote when you replaced a field a person last wrote; restore_plan_revision puts it back.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the part to replace. Empty string replaces the plan document.'),
            value: z.any().describe('Replacement object for that part.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.replacePart(
        planStore,
        args && args.id,
        planInput(args, ['value']),
        mcpContext('replace_part', extra)
    )));

    register(server, 'insert_block', {
        title: 'Insert a block into a lesson plan',
        description: 'Insert a block into a lesson at index. Omit index to append. Pass path (the lesson), block, and expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision, and writes nothing.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the lesson, such as units/unit-1/lessons/cafe-order.'),
            index: z.number().int().optional().describe('Destination index. Omit to append.'),
            block: z.any().describe('Block object to insert.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.insertBlock(
        planStore,
        args && args.id,
        planInput(args, ['index', 'block']),
        mcpContext('insert_block', extra)
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
    }, async (args, extra) => callPlanTool(() => planOps.moveBlock(
        planStore,
        args && args.id,
        planInput(args, ['index']),
        mcpContext('move_block', extra)
    )));

    register(server, 'remove_block', {
        title: 'Remove a block from a lesson plan',
        description: 'Remove the block at path. Pass expectedRevision. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the block to remove.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.removeBlock(
        planStore,
        args && args.id,
        planInput(args, []),
        mcpContext('remove_block', extra)
    )));

    // INIT-010/SPEC-002: lesson stack. A plan has no lesson maximum.
    register(server, 'insert_lesson', {
        title: 'Insert a lesson into a plan',
        description: 'Insert a lesson into a unit at index. Omit index to append. Pass path (the unit), lesson, and expectedRevision. A plan has no lesson maximum: one lesson or a hundred is allowed. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision, and writes nothing.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the unit, such as units/unit-1.'),
            index: z.number().int().optional().describe('Destination index. Omit to append.'),
            lesson: z.any().describe('Lesson object to insert.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.insertLesson(
        planStore,
        args && args.id,
        planInput(args, ['index', 'lesson']),
        mcpContext('insert_lesson', extra)
    )));

    register(server, 'move_lesson', {
        title: 'Move a lesson in a plan',
        description: 'Move the lesson at path to index within its unit. Pass expectedRevision. A plan has no lesson maximum. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the lesson.'),
            index: z.number().int().describe('Destination index within the unit.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.moveLesson(
        planStore,
        args && args.id,
        planInput(args, ['index']),
        mcpContext('move_lesson', extra)
    )));

    register(server, 'remove_lesson', {
        title: 'Remove a lesson from a plan',
        description: 'Remove the lesson at path. Pass expectedRevision. Refused when it is the last lesson of its unit: a plan keeps at least one lesson. A plan has no lesson maximum. Requires MCP_AUTH_TOKEN. A stale expectedRevision returns revision_conflict and the stored revision.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the lesson to remove.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.removeLesson(
        planStore,
        args && args.id,
        planInput(args, []),
        mcpContext('remove_lesson', extra)
    )));

    register(server, 'set_field', {
        title: 'Set one field on a lesson plan part',
        description: 'Set field to value on the part at path. Pass expectedRevision. Requires MCP_AUTH_TOKEN. This is the usual way to fix one issue from list_plan_issues. id cannot be changed, and audience and assigneeId cannot be set here (use assign_plan). A stale expectedRevision returns revision_conflict and the stored revision, and writes nothing. The result lists overwrote when you replaced a value a person last wrote; restore_plan_revision puts it back.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            path: z.string().describe('Id path of the part. Empty string is the plan document.'),
            field: z.string().describe('Field name on that part.'),
            value: z.any().describe('New value.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planOps.setField(
        planStore,
        args && args.id,
        planInput(args, ['field', 'value']),
        mcpContext('set_field', extra)
    )));

    register(server, 'publish_plan', {
        title: 'Publish a lesson plan',
        description: 'Publish the working copy so learners can read it. Pass expectedRevision. Requires MCP_AUTH_TOKEN. No person has to approve a publish, and warnings never stop it. It is refused when a check finds a block (list_plan_issues shows them, validation lists them) or when the plan uses a picture that is still pending approval in a lesson visible to learners (pending_media). A stale expectedRevision returns revision_conflict and the stored revision. Nothing is written when it is refused. A lesson is visible only if its visibility is everyone or selected; a new lesson is hidden.',
        inputSchema: {
            id: z.string().describe('Plan id.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planPublish.publishPlan(
        planStore,
        args && args.id,
        planInput(args, []),
        mcpContext('publish_plan', extra)
    )));

    register(server, 'delete_plan', {
        title: 'Delete a lesson plan',
        description: [
            'Delete the plan, its published snapshot, and its history. Pass id and expectedRevision from the last read.',
            'A stale expectedRevision returns revision_conflict and deletes nothing.',
            'This does not remove a worksheet lesson that uses the same id. delete_lesson does that.',
            'remove_lesson still refuses to remove the last lesson of a unit. Delete the plan instead.',
            'Requires MCP_AUTH_TOKEN.'
        ].join(' '),
        inputSchema: {
            id: PLAN_ID_SCHEMA.describe('Plan id.'),
            expectedRevision: z.number().int().describe('Revision from the last read. Must match the stored working copy.')
        }
    }, async (args, extra) => callPlanTool(() => planDelete.deletePlan(
        planStore,
        args && args.id,
        planInput(args, []),
        mcpContext('delete_plan', extra)
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

    registerPlanBuilderTools(server);
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
