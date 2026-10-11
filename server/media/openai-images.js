/**
 * OpenAI image generation client (ADR-0004 §7).
 * planImage is a dry-run: image count times the pinned $0.011, and no network call.
 * generateImage calls the provider only when the key is set, the prompt is allowed,
 * confirm is true, and the estimate matches that one-image dry-run. The hour and day
 * caps are reserved before the request is sent, so a provider error still counts.
 * The only model id in the request body is gpt-image-1. A rejection of that id
 * disables later generation; this module never substitutes another model.
 * Image bytes are decoded from the response and passed to ingest. The key is read
 * from the environment at call time and is not written to a row, a log, or an error.
 * Provenance: INIT-004/SPEC-009
 */
'use strict';

const https = require('https');
const ingest = require('./ingest');

const PINNED_UNIT_MILLS = 11;
const DOLLAR_MILLS = 1000;
const HOURLY_CALL_CAP = 15;
const DAILY_CAP_MILLS = 1000;
const MAX_PLAN_COUNT = 1000;
const MAX_PROMPT_LENGTH = 4000;
const MAX_ALT_TEXT_LENGTH = 2000;
const MAX_IMAGE_BYTES = 5242880;
const MAX_B64_CHARS = 7000000;
const MAX_RESPONSE_BYTES = 8 * 1024 * 1024;
const REQUEST_TIMEOUT_MS = 60000;
const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;
const ENDPOINT_HOST = 'api.openai.com';
const ENDPOINT_PATH = '/v1/images/generations';

const ACTOR_PATTERN = /^(?:mcp|admin:[a-z0-9_.-]{1,122})$/;

const MESSAGES = Object.freeze({
    missing_key: 'Image generation is not configured.',
    prompt_refused: 'That prompt is not allowed.',
    confirm_required: 'Generation requires confirm from the dry-run.',
    estimate_mismatch: 'The estimate does not match the dry-run for one image.',
    hourly_cap: 'The image generation limit for this hour has been reached.',
    daily_cap: 'The image generation limit for this day has been reached.',
    generation_disabled: 'Image generation is disabled until the pinned model is restored.',
    provider_error: 'The image provider did not return an image.',
    invalid_input: 'The image request is not valid.',
    invalid_actor: 'The actor is not valid.',
    invalid_id: 'The media id is not valid.',
    store_failed: 'The image could not be stored.'
});

/**
 * Fixed prompt refusals for the three ADR-0004 §7 categories: a sexual act, a minor,
 * or a real person's likeness. Matching uses letter and number boundaries on the
 * normalized prompt. This is a list, not a model call, and a refusal is not sent.
 */
const PROMPT_TERMS = Object.freeze([
    phrase('sexual act', 'sexual act'),
    phrase('sexual act', 'sexual'),
    phrase('sexual act', 'sex'),
    phrase('sexual act', 'intercourse'),
    phrase('sexual act', 'pornography'),
    phrase('sexual act', 'porn'),
    phrase('sexual act', 'nude'),
    phrase('sexual act', 'naked'),
    phrase('sexual act', 'erotic'),
    phrase('sexual act', 'anal'),
    phrase('minor', 'minor'),
    phrase('minor', 'minors'),
    phrase('minor', 'child'),
    phrase('minor', 'children'),
    phrase('minor', 'kid'),
    phrase('minor', 'kids'),
    phrase('minor', 'toddler'),
    phrase('minor', 'infant'),
    phrase('minor', 'baby'),
    phrase('minor', 'underage'),
    phrase('minor', 'teenager'),
    phrase('minor', 'teen'),
    phrase('minor', 'teens'),
    pattern('minor', /(?:^|[^\p{L}\p{N}])under\s*(?:18|eighteen)(?:$|[^\p{L}\p{N}])/iu),
    pattern('minor', /(?:^|[^\p{L}\p{N}])(?:[0-9]|1[0-7])\s*(?:years?|yrs?)\s*old(?:$|[^\p{L}\p{N}])/iu),
    phrase('likeness', 'likeness'),
    phrase('likeness', 'celebrity'),
    phrase('likeness', 'celebrities'),
    phrase('likeness', 'real person'),
    phrase('likeness', "real person's"),
    phrase('likeness', "person's likeness")
]);

let hourBucket = null;
let dayBucket = null;
let modelRejected = false;

class ImageGenError extends Error {
    /**
     * @param {string} code
     * @param {string} message
     */
    constructor(code, message) {
        super(message);
        this.name = 'ImageGenError';
        this.code = code;
    }
}

function phrase(category, value) {
    return Object.freeze({ category, kind: 'phrase', value });
}

function pattern(category, value) {
    return Object.freeze({ category, kind: 'pattern', value });
}

function refusal(code) {
    return new ImageGenError(code, MESSAGES[code] || MESSAGES.invalid_input);
}

function toMs(now) {
    let t;
    if (now == null) t = Date.now();
    else if (now instanceof Date) t = now.getTime();
    else if (typeof now === 'number') t = now;
    else t = new Date(now).getTime();
    return Number.isFinite(t) ? t : Date.now();
}

function estimatedDollars(count) {
    return (count * PINNED_UNIT_MILLS) / DOLLAR_MILLS;
}

function sameEstimate(value, count) {
    if (typeof value !== 'number' || !Number.isFinite(value)) return false;
    return Math.round(value * DOLLAR_MILLS) === count * PINNED_UNIT_MILLS;
}

/**
 * The generation request body. The only model id here is gpt-image-1.
 * Callers cannot choose a model, a size, or a quality.
 * @param {string} prompt
 * @returns {{ model: string, prompt: string, n: number, size: string, quality: string }}
 */
function buildGenerationBody(prompt) {
    return Object.freeze({
        model: 'gpt-image-1',
        prompt,
        n: 1,
        size: '1024x1024',
        quality: 'low'
    });
}

/**
 * Read OPENAI_API_KEY at call time. An empty value is treated as unset.
 * The value is returned to the caller and is not logged.
 * @returns {string|null}
 */
function readApiKey() {
    const value = process.env.OPENAI_API_KEY;
    if (typeof value !== 'string') return null;
    const key = value.trim();
    return key ? key : null;
}

function normalizePrompt(text) {
    return text
        .normalize('NFKC')
        .replace(/\p{Default_Ignorable_Code_Point}/gu, '')
        .replace(/\s+/g, ' ')
        .trim();
}

function escapeRegExp(value) {
    return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function termMatches(text, term) {
    if (term.kind === 'pattern') return term.value.test(text);
    const body = escapeRegExp(term.value).replace(/\\\s+/g, '\\s+');
    const re = new RegExp('(?:^|[^\\p{L}\\p{N}])' + body + '(?:$|[^\\p{L}\\p{N}])', 'iu');
    return re.test(text);
}

/**
 * @param {string} prompt normalized prompt
 * @returns {boolean}
 */
function promptIsRefused(prompt) {
    for (const term of PROMPT_TERMS) {
        if (termMatches(prompt, term)) return true;
    }
    return false;
}

/**
 * @param {object} input
 * @returns {{ prompt: string }}
 */
function readPrompt(input, apiKey) {
    if (!input || typeof input !== 'object') throw refusal('invalid_input');
    const raw = input.prompt;
    if (typeof raw !== 'string' || raw.length === 0 || raw.length > MAX_PROMPT_LENGTH) {
        throw refusal('invalid_input');
    }
    if (/[\u0000-\u001F]/.test(raw)) throw refusal('invalid_input');
    const prompt = normalizePrompt(raw);
    if (!prompt || prompt.length > MAX_PROMPT_LENGTH) throw refusal('invalid_input');
    if (apiKey && (raw.includes(apiKey) || prompt.includes(apiKey))) {
        throw refusal('prompt_refused');
    }
    if (promptIsRefused(prompt)) throw refusal('prompt_refused');
    return { prompt };
}

function readPlanCount(input) {
    if (input.count == null) return 1;
    if (!Number.isInteger(input.count) || input.count < 1 || input.count > MAX_PLAN_COUNT) {
        throw refusal('invalid_input');
    }
    return input.count;
}

function hourState(t) {
    const hour = Math.floor(t / HOUR_MS);
    if (!hourBucket || hour > hourBucket.hour) hourBucket = { hour, count: 0 };
    return hourBucket;
}

function dayState(t) {
    const day = Math.floor(t / DAY_MS);
    if (!dayBucket || day > dayBucket.day) dayBucket = { day, mills: 0 };
    return dayBucket;
}

/**
 * Refuse before any provider contact when this call would pass a cap.
 * On success the slot is taken immediately, before the await, so a later
 * provider error still counts and a concurrent call cannot share the slot.
 * @param {number} t
 */
function reserveGeneration(t) {
    const hour = hourState(t);
    const day = dayState(t);
    if (hour.count >= HOURLY_CALL_CAP) throw refusal('hourly_cap');
    if (day.mills + PINNED_UNIT_MILLS > DAILY_CAP_MILLS) throw refusal('daily_cap');
    hour.count += 1;
    day.mills += PINNED_UNIT_MILLS;
}

function mentionsSecret(err, apiKey) {
    const blob = String(err && err.message);
    if (apiKey && blob.includes(apiKey)) return true;
    return /sk-[A-Za-z0-9_-]{8,}/.test(blob);
}

function rethrowClean(err, apiKey) {
    if (err instanceof ImageGenError) throw err;
    if (err instanceof ingest.MediaError && !mentionsSecret(err, apiKey)) throw err;
    throw refusal('store_failed');
}

/**
 * @param {number} status
 * @param {object|null} body
 * @returns {boolean}
 */
function isModelRejection(status, body) {
    if (status !== 400 && status !== 404 && status !== 422) return false;
    const error = body && typeof body === 'object' ? body.error : null;
    if (!error || typeof error !== 'object') return false;
    if (error.param === 'model') return true;
    return error.code === 'model_not_found' || error.code === 'invalid_model';
}

function decodeImage(b64) {
    if (typeof b64 !== 'string' || b64.length === 0 || b64.length > MAX_B64_CHARS) return null;
    const compact = b64.replace(/\s+/g, '');
    if (!/^[A-Za-z0-9+/]+={0,2}$/.test(compact)) return null;
    const bytes = Buffer.from(compact, 'base64');
    if (bytes.length === 0 || bytes.length > MAX_IMAGE_BYTES) return null;
    return bytes;
}

function imageBytesFromResponse(status, body) {
    if (status < 200 || status >= 300) return null;
    const data = body && typeof body === 'object' ? body.data : null;
    const first = Array.isArray(data) ? data[0] : null;
    if (!first || typeof first !== 'object') return null;
    return decodeImage(first.b64_json);
}

/**
 * POST the generation body to the pinned OpenAI host. The URL is not taken
 * from the caller. Redirects are not followed. Response text is parsed and
 * then dropped; it is never copied into an error.
 * @param {Function} requestFn
 * @param {{ headers: { Authorization: string }, body: object }} outbound
 * @returns {Promise<{ status: number, body: object|null }>}
 */
function postGeneration(requestFn, outbound) {
    const payload = Buffer.from(JSON.stringify(outbound.body));
    return new Promise((resolve, reject) => {
        let settled = false;
        const fail = () => {
            if (settled) return;
            settled = true;
            reject(new Error('provider_transport'));
        };
        const finish = (value) => {
            if (settled) return;
            settled = true;
            resolve(value);
        };
        let req;
        try {
            req = requestFn({
                protocol: 'https:',
                hostname: ENDPOINT_HOST,
                port: 443,
                path: ENDPOINT_PATH,
                method: 'POST',
                headers: {
                    Authorization: outbound.headers.Authorization,
                    'Content-Type': 'application/json',
                    Accept: 'application/json',
                    'Content-Length': String(payload.length)
                }
            }, (res) => {
                const chunks = [];
                let total = 0;
                res.on('data', (chunk) => {
                    const buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
                    total += buf.length;
                    if (total > MAX_RESPONSE_BYTES) {
                        res.destroy();
                        fail();
                        return;
                    }
                    chunks.push(buf);
                });
                res.on('error', fail);
                res.on('end', () => {
                    if (settled) return;
                    const status = res.statusCode || 0;
                    let parsed = null;
                    try {
                        parsed = JSON.parse(Buffer.concat(chunks).toString('utf8'));
                    } catch (_) {
                        parsed = null;
                    }
                    finish({ status, body: parsed && typeof parsed === 'object' ? parsed : null });
                });
            });
        } catch (_) {
            fail();
            return;
        }
        if (!req || typeof req.on !== 'function' || typeof req.end !== 'function') {
            fail();
            return;
        }
        req.on('error', fail);
        if (typeof req.setTimeout === 'function') {
            req.setTimeout(REQUEST_TIMEOUT_MS, () => {
                if (typeof req.destroy === 'function') req.destroy();
                fail();
            });
        }
        req.end(payload);
    });
}

/**
 * @param {Function} requestFn https.request, or a fake in tests
 * @returns {(outbound: object) => Promise<{ status: number, body: object|null }>}
 */
function createHttpsTransport(requestFn) {
    return function httpsTransport(outbound) {
        return postGeneration(requestFn, outbound);
    };
}

function outboundRequest(apiKey, prompt) {
    return Object.freeze({
        url: 'https://' + ENDPOINT_HOST + ENDPOINT_PATH,
        method: 'POST',
        headers: Object.freeze({
            Authorization: 'Bearer ' + apiKey,
            'Content-Type': 'application/json'
        }),
        body: buildGenerationBody(prompt)
    });
}

/**
 * Dry-run. Returns the count and the pinned estimate. Does not call the provider
 * and does not consume an hour or day slot.
 * @param {{ prompt: string, count?: number }} input
 * @param {{ transport?: Function }} [options]
 * @returns {Promise<{ count: number, estimatedDollars: number }>}
 */
async function planImage(input, _options) {
    void _options;
    const apiKey = readApiKey();
    if (!apiKey) throw refusal('missing_key');
    readPrompt(input, apiKey);
    const count = readPlanCount(input);
    return { count, estimatedDollars: estimatedDollars(count) };
}

function readGenerateInput(input, apiKey) {
    const { prompt } = readPrompt(input, apiKey);
    if (input.confirm !== true) throw refusal('confirm_required');
    if (!sameEstimate(input.estimate, 1)) throw refusal('estimate_mismatch');
    if (typeof input.actor !== 'string' || !ACTOR_PATTERN.test(input.actor)) {
        throw refusal('invalid_actor');
    }
    if (!ingest.isValidMediaId(input.id)) throw refusal('invalid_id');
    const altText = input.altText == null ? '' : input.altText;
    if (typeof altText !== 'string' || altText.length > MAX_ALT_TEXT_LENGTH) {
        throw refusal('invalid_input');
    }
    return { prompt, actor: input.actor, id: input.id, altText };
}

/**
 * Generate one image and store it through ingest. Refusals throw before the
 * transport runs. The transport is invoked at most once, with model gpt-image-1.
 * Learner-typed fields on the input are not read.
 * @param {{
 *   prompt: string,
 *   confirm: boolean,
 *   estimate: number,
 *   actor: string,
 *   id: string,
 *   altText?: string
 * }} input
 * @param {{
 *   transport?: (outbound: object) => Promise<{ status: number, body: object|null }>,
 *   ingest?: Function,
 *   now?: Date|number|string
 * }} [options]
 * @returns {Promise<object>}
 */
async function generateImage(input, options) {
    const opts = options || {};
    const apiKey = readApiKey();
    if (!apiKey) throw refusal('missing_key');
    const fields = readGenerateInput(input, apiKey);
    if (modelRejected) throw refusal('generation_disabled');
    const transport = opts.transport || createHttpsTransport(https.request);
    if (typeof transport !== 'function') throw refusal('invalid_input');
    const store = opts.ingest || ingest.ingestImage;
    if (typeof store !== 'function') throw refusal('invalid_input');

    const t = toMs(opts.now);
    reserveGeneration(t);
    const outbound = outboundRequest(apiKey, fields.prompt);
    let response;
    try {
        response = await transport(outbound);
    } catch (_) {
        throw refusal('provider_error');
    }
    const status = response && Number.isInteger(response.status) ? response.status : 0;
    const body = response && response.body && typeof response.body === 'object' ? response.body : null;
    if (isModelRejection(status, body)) {
        modelRejected = true;
        throw refusal('generation_disabled');
    }
    const bytes = imageBytesFromResponse(status, body);
    if (!bytes) throw refusal('provider_error');
    try {
        return await store({
            bytes,
            id: fields.id,
            actor: fields.actor,
            altText: fields.altText,
            promptText: fields.prompt,
            provider: 'openai',
            modelId: 'gpt-image-1'
        }, { now: opts.now });
    } catch (err) {
        rethrowClean(err, apiKey);
    }
}

/** Test helper. Clears the shared hour cap, day cap, and model-rejection latch. */
function _resetGenerationState() {
    hourBucket = null;
    dayBucket = null;
    modelRejected = false;
}

/**
 * Test helper. Counts only; the key is not included.
 * @returns {{ hourCount: number, dayMills: number, disabled: boolean }}
 */
function _generationCounters() {
    return {
        hourCount: hourBucket ? hourBucket.count : 0,
        dayMills: dayBucket ? dayBucket.mills : 0,
        disabled: modelRejected
    };
}

module.exports = {
    planImage,
    generateImage,
    buildGenerationBody,
    createHttpsTransport,
    ImageGenError,
    PINNED_UNIT_MILLS,
    HOURLY_CALL_CAP,
    DAILY_CAP_MILLS,
    _resetGenerationState,
    _generationCounters
};
