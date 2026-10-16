/**
 * Media, generation, and approval HTTP routes (ADR-0004 §4, §6, §8, §10).
 * Handlers call ingest and the image client. They do not re-encode and they
 * do not call OpenAI themselves. Request bodies never choose a codec, a clock,
 * a transport, or an actor kind. Approval and delete pass actorKind admin.
 * Audit rows name the media id and do not copy the generation prompt.
 * Provenance: INIT-004/SPEC-011
 */
'use strict';

const crypto = require('crypto');
const body = require('../http/body');
const security = require('../http/security');
const oauth = require('../../mcp/oauth');
const audit = require('../db/repos/audit');
const mediaRepo = require('../db/repos/media');
const visibilityRepo = require('../db/repos/visibility');
const store = require('../plans/store');
const ingest = require('../media/ingest');
const images = require('../media/openai-images');

const JSON_LIMIT = 64 * 1024;
const ACTOR_NAME = /^[a-z0-9_.-]{1,122}$/;
const MULTIPART_FIELDS = new Set(['id', 'altText', 'file']);

/**
 * Mutations. Tests walk this list for 401, 403, and CSRF.
 * Approval is not an MCP route.
 * @type {ReadonlyArray<{ method: string, path: string }>}
 */
const MUTATIONS = Object.freeze([
    { method: 'POST', path: '/api/media' },
    { method: 'POST', path: '/api/admin/media/:id/approval' },
    { method: 'POST', path: '/api/admin/media/:id/delete' },
    { method: 'POST', path: '/api/admin/images' }
]);

/**
 * @type {((outbound: object) => Promise<{ status: number, body: object|null }>)|null}
 */
let transportForTests = null;

class RouteInputError extends Error {
    /**
     * @param {number} status
     * @param {string} code
     * @param {string} message
     */
    constructor(status, code, message) {
        super(message);
        this.name = 'RouteInputError';
        this.status = status;
        this.code = code;
    }
}

const STATUS = Object.freeze({
    invalid_input: 400,
    invalid_actor: 400,
    invalid_id: 400,
    invalid_alt_text: 400,
    invalid_prompt: 400,
    invalid_provider: 400,
    invalid_model_id: 400,
    invalid_multipart: 400,
    alt_text_required: 400,
    unsupported_type: 400,
    malformed_image: 400,
    animated_refused: 400,
    dimensions_too_large: 400,
    reencode_failed: 400,
    confirm_required: 400,
    estimate_mismatch: 400,
    prompt_refused: 400,
    bad_request: 400,
    too_large: 413,
    body_too_large: 413,
    rate_limited: 429,
    hourly_cap: 429,
    daily_cap: 429,
    id_taken: 409,
    archive_conflict: 409,
    not_found: 404,
    forbidden_actor: 403,
    media_missing: 404,
    decoder_unavailable: 503,
    missing_key: 503,
    generation_disabled: 503,
    provider_error: 502,
    store_failed: 500,
    unsafe_path: 500,
    archive_failed: 500,
    archive_verify_failed: 500,
    delete_failed: 500
});

/**
 * Test seam. Production leaves this unset, so generate uses the HTTPS client.
 * The HTTP body cannot set it.
 * @param {Function|null} fn
 */
function _setTransportForTests(fn) {
    transportForTests = typeof fn === 'function' ? fn : null;
}

/**
 * @param {object|null|undefined} user
 * @returns {string|null}
 */
function adminActor(user) {
    const name = user && user.username;
    if (typeof name !== 'string' || !ACTOR_NAME.test(name)) return null;
    return 'admin:' + name;
}

function envKey() {
    const value = process.env.OPENAI_API_KEY;
    if (typeof value !== 'string') return '';
    return value.trim();
}

/**
 * @param {string} message
 * @returns {string}
 */
function scrub(message) {
    const key = envKey();
    let text = String(message || 'Error');
    if (key && text.includes(key)) text = 'Request failed';
    return text;
}

/**
 * @param {import('http').ServerResponse} res
 * @param {number} status
 * @param {string} message
 * @param {string} [code]
 */
function sendError(res, status, message, code) {
    security.sendJsonError(res, status, scrub(message), code);
}

/**
 * @param {import('http').ServerResponse} res
 * @param {number} status
 * @param {object} data
 */
function sendData(res, status, data) {
    const key = envKey();
    const payload = JSON.stringify(data);
    if (key && payload.includes(key)) {
        security.sendJsonError(res, 500, 'Internal error', 'internal');
        return;
    }
    security.sendJson(res, status, data);
}

/**
 * @param {import('http').ServerResponse} res
 * @param {unknown} err
 */
function sendMappedError(res, err) {
    if (res.headersSent || res.writableEnded) return;
    if (err && err.name === 'BodyError') {
        sendError(res, err.status, err.message, err.code);
        return;
    }
    if (err && err.name === 'RouteInputError') {
        sendError(res, err.status, err.message, err.code);
        return;
    }
    if (err && (err.name === 'MediaError' || err.name === 'ImageGenError')) {
        const status = STATUS[err.code] || 400;
        const message = status >= 500 ? 'Internal error' : err.message;
        sendError(res, status, message, err.code);
        return;
    }
    sendError(res, 500, 'Internal error', 'internal');
}

/**
 * @param {string} actor
 * @param {string} action
 * @param {string} mediaId
 */
function writeAudit(actor, action, mediaId) {
    audit.append({
        actor: actor,
        action: action,
        target: mediaId,
        detail: null
    });
}

/**
 * @param {object} row
 * @param {{ width?: number, height?: number }} [size]
 */
function publicMedia(row, size) {
    const out = {
        id: row.id,
        status: row.status,
        altText: row.altText
    };
    if (size && Number.isInteger(size.width) && Number.isInteger(size.height)) {
        out.width = size.width;
        out.height = size.height;
    }
    return out;
}

/**
 * @param {import('http').IncomingMessage} req
 * @param {number} limit
 * @returns {Promise<Buffer>}
 */
function discard(req) {
    req.on('error', () => {});
    if (typeof req.destroy === 'function') req.destroy();
}

function readLimited(req, limit) {
    const declared = req.headers['content-length'];
    if (declared != null) {
        const n = Number(declared);
        if (!Number.isFinite(n) || n < 0 || n > limit) {
            discard(req);
            return Promise.reject(new body.BodyError(
                'Request body too large',
                413,
                'body_too_large'
            ));
        }
    }
    return new Promise((resolve, reject) => {
        const chunks = [];
        let size = 0;
        let settled = false;

        function fail(err) {
            if (settled) return;
            settled = true;
            reject(err);
        }

        req.on('data', (chunk) => {
            const buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
            size += buf.length;
            if (size > limit) {
                chunks.length = 0;
                discard(req);
                fail(new body.BodyError('Request body too large', 413, 'body_too_large'));
                return;
            }
            chunks.push(buf);
        });
        req.on('end', () => {
            if (settled) return;
            settled = true;
            resolve(Buffer.concat(chunks));
        });
        req.on('error', (err) => {
            if (err && err.name === 'BodyError') fail(err);
            else fail(new body.BodyError('Request body too large', 413, 'body_too_large'));
        });
    });
}

/**
 * @param {string} header
 * @returns {string|null}
 */
function boundaryOf(header) {
    if (typeof header !== 'string') return null;
    const media = header.split(';')[0].trim().toLowerCase();
    if (media !== 'multipart/form-data') return null;
    const match = /;\s*boundary=(?:"([^"]+)"|([^;\s]+))/i.exec(header);
    if (!match) return null;
    const boundary = match[1] != null ? match[1] : match[2];
    if (!boundary || boundary.length > 70 || /[\r\n\0]/.test(boundary)) return null;
    return boundary;
}

/**
 * @param {string} headers
 * @returns {string|null}
 */
function partName(headers) {
    const lines = headers.split('\r\n');
    for (const line of lines) {
        if (!/^content-disposition:/i.test(line)) continue;
        const quoted = /(?:^|;)\s*name="([^"]*)"/i.exec(line);
        if (quoted) return quoted[1];
        const bare = /(?:^|;)\s*name=([^;\s]+)/i.exec(line);
        if (bare) return bare[1];
        return null;
    }
    return null;
}

/**
 * @param {Buffer} buf
 * @param {string} boundary
 * @returns {Array<{ name: string, body: Buffer }>}
 */
function splitMultipart(buf, boundary) {
    const delim = Buffer.from('--' + boundary);
    const parts = [];
    let cursor = buf.indexOf(delim);
    if (cursor < 0) {
        throw new RouteInputError(400, 'invalid_multipart', 'Upload request is not valid.');
    }
    cursor += delim.length;
    while (parts.length < 4) {
        if (buf.length >= cursor + 2 && buf[cursor] === 45 && buf[cursor + 1] === 45) break;
        if (buf.length >= cursor + 2 && buf[cursor] === 13 && buf[cursor + 1] === 10) cursor += 2;
        const next = buf.indexOf(delim, cursor);
        if (next < 0) {
            throw new RouteInputError(400, 'invalid_multipart', 'Upload request is not valid.');
        }
        let chunk = buf.subarray(cursor, next);
        if (chunk.length >= 2 && chunk[chunk.length - 2] === 13 && chunk[chunk.length - 1] === 10) {
            chunk = chunk.subarray(0, chunk.length - 2);
        }
        const sep = chunk.indexOf('\r\n\r\n');
        if (sep < 0) {
            throw new RouteInputError(400, 'invalid_multipart', 'Upload request is not valid.');
        }
        const name = partName(chunk.subarray(0, sep).toString('latin1'));
        if (!name || !MULTIPART_FIELDS.has(name)) {
            throw new RouteInputError(400, 'bad_request', 'Invalid request');
        }
        parts.push({ name: name, body: chunk.subarray(sep + 4) });
        cursor = next + delim.length;
    }
    if (buf.indexOf(delim, cursor) !== -1) {
        throw new RouteInputError(400, 'bad_request', 'Invalid request');
    }
    return parts;
}

/**
 * @param {Buffer} buf
 * @returns {string}
 */
function textPart(buf, max) {
    if (buf.length > max || buf.includes(0)) {
        throw new RouteInputError(400, 'bad_request', 'Invalid request');
    }
    return buf.toString('utf8');
}

/**
 * id, optional altText, and one file. The client filename is not read.
 * @param {Buffer} buf
 * @param {string} boundary
 * @returns {{ id: string, altText?: string, bytes: Buffer }}
 */
function readUploadParts(buf, boundary) {
    const parts = splitMultipart(buf, boundary);
    /** @type {Record<string, Buffer>} */
    const found = Object.create(null);
    for (const part of parts) {
        if (found[part.name]) {
            throw new RouteInputError(400, 'bad_request', 'Invalid request');
        }
        found[part.name] = part.body;
    }
    if (!found.id || !found.file) {
        throw new RouteInputError(400, 'bad_request', 'Invalid request');
    }
    const input = {
        id: textPart(found.id, 64),
        bytes: found.file
    };
    if (found.altText) input.altText = textPart(found.altText, 2000);
    return input;
}

/**
 * @param {import('http').IncomingMessage} req
 * @param {readonly string[]} required
 * @param {readonly string[]} [optional]
 * @returns {Promise<object>}
 */
async function readJsonObject(req, required, optional) {
    let payload;
    try {
        payload = await body.readJson(req, { limit: JSON_LIMIT });
    } catch (err) {
        if (err && err.name === 'BodyError') throw err;
        throw new RouteInputError(400, 'bad_request', 'Invalid request');
    }
    if (payload == null) payload = {};
    if (typeof payload !== 'object' || Array.isArray(payload)) {
        throw new RouteInputError(400, 'bad_request', 'Invalid request');
    }
    const extra = optional || [];
    const keys = Object.keys(payload);
    for (const key of keys) {
        if (required.indexOf(key) === -1 && extra.indexOf(key) === -1) {
            throw new RouteInputError(400, 'bad_request', 'Invalid request');
        }
    }
    for (const key of required) {
        if (!Object.prototype.hasOwnProperty.call(payload, key)) {
            throw new RouteInputError(400, 'bad_request', 'Invalid request');
        }
    }
    return payload;
}

/**
 * @param {object} lesson
 * @param {string} mediaId
 * @returns {boolean}
 */
function lessonReferences(lesson, mediaId) {
    const blocks = lesson && Array.isArray(lesson.blocks) ? lesson.blocks : [];
    for (const block of blocks) {
        const words = block && Array.isArray(block.words) ? block.words : [];
        for (const word of words) {
            if (word && word.image && word.image.mediaId === mediaId) return true;
        }
    }
    return false;
}

/**
 * @param {string} slug
 * @param {number} userId
 * @returns {boolean}
 */
function hasGrant(slug, userId) {
    if (typeof slug !== 'string' || !slug) return false;
    const rows = visibilityRepo.listGrants(slug);
    const id = Number(userId);
    return rows.some((row) => Number(row.user_id) === id);
}

/**
 * A published lesson this user may see references the id.
 * Personal plans are visible only to assigneeId. Shared hidden lessons are not.
 * Working copies are not consulted.
 * @param {object} user
 * @param {string} mediaId
 * @returns {boolean}
 */
function userMayFetch(user, mediaId) {
    if (!user || typeof mediaId !== 'string') return false;
    const rows = store.listSnapshots();
    for (const row of rows) {
        let snapshot;
        try {
            snapshot = JSON.parse(row.snapshot);
        } catch (_) {
            continue;
        }
        if (!snapshot || snapshot.status !== 'published') continue;
        const units = Array.isArray(snapshot.units) ? snapshot.units : [];
        if (snapshot.audience === 'personal') {
            if (snapshot.assigneeId !== user.username) continue;
            for (const unit of units) {
                const lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
                for (const lesson of lessons) {
                    if (lessonReferences(lesson, mediaId)) return true;
                }
            }
            continue;
        }
        if (snapshot.audience !== 'shared') continue;
        for (const unit of units) {
            const lessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            for (const lesson of lessons) {
                if (!lessonReferences(lesson, mediaId)) continue;
                if (lesson.visibility === 'everyone') return true;
                if (lesson.visibility === 'selected' && hasGrant(lesson.id, user.id)) return true;
            }
        }
    }
    return false;
}

/**
 * @param {ReturnType<import('./router').createRouter>} router
 * @param {string} method
 * @param {string} path
 * @param {Function} handler
 */
function addAdmin(router, method, path, handler) {
    router.add(method, path, router.requireSession(router.requireAdmin(handler)));
}

/**
 * True when the request presents the configured MCP token or an access token
 * this server issued from it. A wrong or missing bearer is not this credential.
 * The token value is not logged.
 * Provenance: INIT-004/SPEC-012
 * @param {import('http').IncomingMessage} req
 * @returns {boolean}
 */
function presentsMcpCredential(req) {
    const expected = process.env.MCP_AUTH_TOKEN;
    if (typeof expected !== 'string' || !expected) return false;
    const match = /^Bearer (\S+)$/.exec(String(req.headers.authorization || ''));
    if (!match) return false;
    const given = crypto.createHash('sha256').update(match[1]).digest();
    const wanted = crypto.createHash('sha256').update(expected).digest();
    if (crypto.timingSafeEqual(given, wanted)) return true;
    return oauth.verifyAccessToken(match[1]);
}

/**
 * The MCP actor cannot approve. A matching credential is 403 before the session
 * guard, including when an admin cookie is also present. Any other caller falls
 * through to the session checks.
 * @param {import('http').IncomingMessage} req
 * @param {import('http').ServerResponse} res
 */
function refuseMcpApproval(req, res) {
    if (!presentsMcpCredential(req)) return;
    security.sendJsonError(res, 403, 'Forbidden', 'forbidden');
}

/**
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    addAdmin(router, 'POST', '/api/media', async (req, res, ctx) => {
        const actor = adminActor(ctx.user);
        if (!actor) {
            sendError(res, 400, 'Uploader is not valid.', 'invalid_actor');
            return;
        }
        try {
            const raw = await readLimited(req, ingest.MAX_INPUT_BYTES);
            const boundary = boundaryOf(req.headers['content-type']);
            if (!boundary) {
                throw new RouteInputError(400, 'invalid_multipart', 'Upload request is not valid.');
            }
            const part = readUploadParts(raw, boundary);
            const input = {
                bytes: part.bytes,
                id: part.id,
                actor: actor
            };
            if (part.altText !== undefined) input.altText = part.altText;
            const stored = await ingest.ingestImage(input);
            writeAudit(actor, 'media_upload', stored.id);
            sendData(res, 201, publicMedia(stored, stored));
        } catch (err) {
            sendMappedError(res, err);
        }
    });

    router.add('GET', '/api/media/:id', router.requireSession(async (req, res, ctx) => {
        const id = ctx.params && ctx.params.id;
        const admin = ctx.user && ctx.user.role === 'admin';
        if (!admin && !userMayFetch(ctx.user, id)) {
            sendError(res, 404, 'Not found', 'not_found');
            return;
        }
        const row = typeof id === 'string' ? mediaRepo.getById(id) : null;
        if (!row) {
            sendError(res, 404, 'Not found', 'not_found');
            return;
        }
        try {
            const sent = ingest.sendStoredImage(res, row.storedName);
            if (!sent && !res.headersSent) sendError(res, 404, 'Not found', 'not_found');
        } catch (err) {
            sendMappedError(res, err);
        }
    }));

    router.add(
        'POST',
        '/api/admin/media/:id/approval',
        refuseMcpApproval,
        router.requireSession(router.requireAdmin(async (req, res, ctx) => {
            const actor = adminActor(ctx.user);
            if (!actor) {
                sendError(res, 400, 'Uploader is not valid.', 'invalid_actor');
                return;
            }
            try {
                const payload = await readJsonObject(req, [], ['altText']);
                const request = {
                    id: ctx.params.id,
                    actorKind: 'admin'
                };
                if (Object.prototype.hasOwnProperty.call(payload, 'altText')) {
                    request.altText = payload.altText;
                }
                const row = ingest.approveMedia(request);
                writeAudit(actor, 'media_approve', row.id);
                sendData(res, 200, publicMedia(row));
            } catch (err) {
                sendMappedError(res, err);
            }
        }))
    );

    addAdmin(router, 'POST', '/api/admin/media/:id/delete', async (req, res, ctx) => {
        const actor = adminActor(ctx.user);
        if (!actor) {
            sendError(res, 400, 'Uploader is not valid.', 'invalid_actor');
            return;
        }
        try {
            await readJsonObject(req, [], []);
            const result = ingest.deleteMedia({
                id: ctx.params.id,
                actorKind: 'admin'
            });
            writeAudit(actor, 'media_delete', result.id);
            sendData(res, 200, { id: result.id });
        } catch (err) {
            sendMappedError(res, err);
        }
    });

    addAdmin(router, 'GET', '/api/admin/images/image-plan', async (req, res) => {
        let url;
        try {
            url = new URL(req.url || '/', 'http://127.0.0.1');
        } catch (_) {
            sendError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const input = {};
        let sawPrompt = false;
        let sawCount = false;
        for (const [key, value] of url.searchParams.entries()) {
            if (key === 'prompt') {
                if (sawPrompt) {
                    sendError(res, 400, 'Invalid request', 'bad_request');
                    return;
                }
                sawPrompt = true;
                input.prompt = value;
                continue;
            }
            if (key === 'count') {
                if (sawCount || !/^[0-9]+$/.test(value)) {
                    sendError(res, 400, 'Invalid request', 'bad_request');
                    return;
                }
                sawCount = true;
                input.count = Number(value);
                continue;
            }
            sendError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!sawPrompt) {
            sendError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        try {
            const plan = await images.planImage(input, transportForTests
                ? { transport: transportForTests }
                : undefined);
            sendData(res, 200, {
                count: plan.count,
                estimatedDollars: plan.estimatedDollars
            });
        } catch (err) {
            sendMappedError(res, err);
        }
    });

    addAdmin(router, 'POST', '/api/admin/images', async (req, res, ctx) => {
        const actor = adminActor(ctx.user);
        if (!actor) {
            sendError(res, 400, 'Uploader is not valid.', 'invalid_actor');
            return;
        }
        try {
            const payload = await readJsonObject(
                req,
                ['prompt', 'id'],
                ['confirm', 'estimate', 'altText']
            );
            const input = {
                prompt: payload.prompt,
                id: payload.id,
                actor: actor
            };
            if (Object.prototype.hasOwnProperty.call(payload, 'confirm')) input.confirm = payload.confirm;
            if (Object.prototype.hasOwnProperty.call(payload, 'estimate')) input.estimate = payload.estimate;
            if (Object.prototype.hasOwnProperty.call(payload, 'altText')) input.altText = payload.altText;
            const options = {};
            if (transportForTests) options.transport = transportForTests;
            const stored = await images.generateImage(input, options);
            writeAudit(actor, 'media_generate', stored.id);
            sendData(res, 201, publicMedia(stored, stored));
        } catch (err) {
            sendMappedError(res, err);
        }
    });
}

module.exports = {
    register,
    MUTATIONS,
    _setTransportForTests
};
