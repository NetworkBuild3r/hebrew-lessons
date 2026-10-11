/**
 * Image ingest, archive-before-delete, approval, and media response headers (ADR-0004 §5, §8, §9).
 * Bytes are measured and sniffed, then decoded and re-encoded to one PNG before anything is stored.
 * Live files sit in DATA_DIR/media under a random hex name; the media row keeps the slug.
 * Upload counts are in memory, per actor, per UTC clock hour (one replica, as ADR-0002).
 * This module does not write audit rows and does not log; adapters audit the actions they call.
 * Provenance: INIT-004/SPEC-008
 */
'use strict';

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const paths = require('../paths');
const mediaRepo = require('../db/repos/media');
const { applySecurityHeaders } = require('../http/security');
const { sharpCodec } = require('./codec');
const sniff = require('./sniff');

const MAX_INPUT_BYTES = 5242880;
const MAX_INPUT_SIDE = 4096;
const MAX_STORED_SIDE = 2048;
const STORED_NAME_BYTES = 32;
const UPLOADS_PER_ACTOR_PER_HOUR = 40;
const MAX_ALT_TEXT_LENGTH = 2000;
const MAX_PROMPT_LENGTH = 4000;
const HOUR_MS = 60 * 60 * 1000;

const STORED_NAME_PATTERN = /^[0-9a-f]{64}\.png$/;
const ACTOR_PATTERN = /^(?:mcp|admin:[a-z0-9_.-]{1,122})$/;
const PROVIDER_PATTERN = /^[a-z0-9_-]{1,64}$/;
const MODEL_ID_PATTERN = /^[a-z0-9._-]{1,64}$/;

/** Codes a decoder may report; any other decoder failure is `reencode_failed`. */
const DECODER_REFUSAL_CODES = new Set([
    'malformed_image', 'animated_refused', 'dimensions_too_large', 'decoder_unavailable'
]);

const ONLY_IMAGES = 'Only PNG, JPEG, or WebP images are accepted.';
const NO_DECODER = 'Image decoding is not configured on this server.';

class MediaError extends Error {
    /**
     * @param {string} code
     * @param {string} message
     * @param {{ retryAfterMs?: number }} [extra]
     */
    constructor(code, message, extra) {
        super(message);
        this.name = 'MediaError';
        this.code = code;
        if (extra && extra.retryAfterMs != null) this.retryAfterMs = extra.retryAfterMs;
    }
}

/**
 * Decoder seam. The default is the sharp codec in ./codec.js. A codec that cannot decode at all
 * reports `decoder_unavailable`, and ingest stores nothing.
 * @typedef {object} ImageCodec
 * @property {(bytes: Buffer, limits: {
 *   format: 'png'|'jpeg'|'webp',
 *   maxInputSide: number,
 *   maxStoredSide: number
 * }) => Promise<{ width: number, height: number, frames: number, png: Buffer }>} decodeAndReencode
 *   Decodes the bytes, reports the decoded width, height, and frame count, and returns one PNG
 *   frame whose longest side is at most maxStoredSide, with no metadata copied. Throws an error
 *   whose `code` is `malformed_image` when the decoder rejects the bytes, or `decoder_unavailable`
 *   when the decoder cannot be loaded.
 */

/** @type {Map<string, { hour: number, count: number }>} */
const uploadsByActor = new Map();

function toMs(now) {
    let t;
    if (now == null) t = Date.now();
    else if (now instanceof Date) t = now.getTime();
    else if (typeof now === 'number') t = now;
    else t = new Date(now).getTime();
    return Number.isFinite(t) ? t : Date.now();
}

/**
 * Count one upload attempt for this actor in the current UTC clock hour.
 * Refused attempts after the slot is taken still count. Each actor's bucket only moves forward:
 * an earlier clock value counts against the latest hour seen, and one actor's call never resets
 * another actor's count. One entry per actor; actors are accounts plus `mcp`, so the map stays small.
 * @param {string} actor
 * @param {Date|number|string} [now]
 * @returns {{ ok: true } | { ok: false, retryAfterMs: number }}
 */
function takeUploadSlot(actor, now) {
    const t = toMs(now);
    const hour = Math.floor(t / HOUR_MS);
    const entry = uploadsByActor.get(actor);
    if (!entry || hour > entry.hour) {
        uploadsByActor.set(actor, { hour, count: 1 });
        return { ok: true };
    }
    if (entry.count >= UPLOADS_PER_ACTOR_PER_HOUR) {
        return { ok: false, retryAfterMs: Math.max(0, (entry.hour + 1) * HOUR_MS - t) };
    }
    entry.count += 1;
    return { ok: true };
}

/** Test helper — clear upload counts. */
function _resetUploadCounts() {
    uploadsByActor.clear();
}

function comparable(p) {
    const resolved = path.resolve(p);
    return process.platform === 'win32' ? resolved.toLowerCase() : resolved;
}

function isInsideOrEqual(candidate, root) {
    const a = comparable(candidate);
    const b = comparable(root);
    if (a === b) return true;
    return a.startsWith(b.endsWith(path.sep) ? b : b + path.sep);
}

function overlaps(a, b) {
    return isInsideOrEqual(a, b) || isInsideOrEqual(b, a);
}

/** Folders the static server can answer by URL, plus the seed worksheets tree. */
function staticRoots() {
    const root = paths.rootDir();
    return paths.servedRoots().concat([
        paths.seedWorksheetsDir(),
        path.resolve(root, 'shared'),
        path.resolve(root, 'login')
    ]);
}

function assertNotServed(dir) {
    for (const root of staticRoots()) {
        if (overlaps(dir, root)) {
            throw new MediaError('unsafe_path', 'The media folder must not overlap a served folder.');
        }
    }
}

/**
 * The path with symlinks and junctions resolved. For a path that does not exist yet, the nearest
 * existing ancestor is resolved and the remaining names are appended, so a folder about to be
 * created is checked where it will land.
 * @param {string} p
 * @returns {string}
 */
function realOrResolved(p) {
    const resolved = path.resolve(p);
    const rest = [];
    let current = resolved;
    for (;;) {
        try {
            return path.join(fs.realpathSync.native(current), ...rest);
        } catch (err) {
            const parent = path.dirname(current);
            if (!err || err.code !== 'ENOENT' || parent === current) return resolved;
            rest.unshift(path.basename(current));
            current = parent;
        }
    }
}

/**
 * DATA_DIR/media. Throws when it overlaps a served folder.
 * @returns {string}
 */
function mediaDir() {
    const dir = path.join(paths.dataDir(), 'media');
    assertNotServed(dir);
    return dir;
}

/** @returns {string} DATA_DIR/media/_archive */
function archiveDir() {
    return path.join(mediaDir(), '_archive');
}

/**
 * After symlinks and junctions resolve, child must sit strictly inside parent.
 * @param {string} child
 * @param {string} parent
 */
function assertRealInside(child, parent) {
    const realChild = comparable(realOrResolved(child));
    const realParent = comparable(realOrResolved(parent));
    if (realChild === realParent || !isInsideOrEqual(realChild, realParent)) {
        throw new MediaError('unsafe_path', 'Media files must stay inside DATA_DIR/media.');
    }
}

/**
 * After links resolve, the media folder must sit inside DATA_DIR and must not overlap a served folder.
 * @param {string} dir
 */
function assertMediaDirReal(dir) {
    assertRealInside(dir, paths.dataDir());
    const real = realOrResolved(dir);
    for (const root of staticRoots()) {
        if (overlaps(real, realOrResolved(root))) {
            throw new MediaError('unsafe_path', 'The media folder must not overlap a served folder.');
        }
    }
}

/** The media folder for writes: checked where it will land, created, then checked again. */
function ensureMediaDir() {
    const dir = mediaDir();
    assertMediaDirReal(dir);
    try {
        fs.mkdirSync(dir, { recursive: true });
    } catch (_) {
        throw new MediaError('store_failed', 'The image could not be stored.');
    }
    assertMediaDirReal(dir);
    return dir;
}

/** The media folder for reads: checked after links resolve when it exists, never created. */
function existingMediaDir() {
    const dir = mediaDir();
    if (fs.existsSync(dir)) assertMediaDirReal(dir);
    return dir;
}

/**
 * Absolute path of a stored name inside one folder. Only `<64 hex>.png` is accepted.
 * @param {string} storedName
 * @param {string} dir
 * @returns {string}
 */
function pathInside(storedName, dir) {
    if (typeof storedName !== 'string' || !STORED_NAME_PATTERN.test(storedName)) {
        throw new MediaError('unsafe_path', 'Stored media name is not valid.');
    }
    const full = path.resolve(dir, storedName);
    if (path.dirname(full) !== path.resolve(dir)) {
        throw new MediaError('unsafe_path', 'Stored media name is not valid.');
    }
    return full;
}

/**
 * Absolute path of a live media file under DATA_DIR/media.
 * @param {string} storedName
 * @returns {string}
 */
function storedFilePath(storedName) {
    return pathInside(storedName, existingMediaDir());
}

/**
 * @param {string} p
 * @returns {'missing'|'file'|'other'}
 */
function fileState(p) {
    try {
        return fs.lstatSync(p).isFile() ? 'file' : 'other';
    } catch (err) {
        if (err && err.code === 'ENOENT') return 'missing';
        throw err;
    }
}

function removeQuietly(p) {
    try {
        fs.unlinkSync(p);
    } catch (_) { /* already gone */ }
}

function sha256File(p) {
    return crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
}

/**
 * Same rule as the media.id CHECK: 1-64 of [a-z0-9-], no leading, trailing, or double hyphen.
 * @param {unknown} id
 * @returns {boolean}
 */
function isValidMediaId(id) {
    return typeof id === 'string'
        && id.length >= 1
        && id.length <= 64
        && /^[a-z0-9-]+$/.test(id)
        && !id.startsWith('-')
        && !id.endsWith('-')
        && !id.includes('--');
}

function toBuffer(bytes) {
    if (Buffer.isBuffer(bytes)) return bytes;
    if (bytes instanceof Uint8Array) {
        return Buffer.from(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    }
    return null;
}

function optionalText(value, pattern, maxLength, code, message) {
    if (value == null || value === '') return null;
    if (typeof value !== 'string'
        || (pattern && !pattern.test(value))
        || (maxLength && value.length > maxLength)) {
        throw new MediaError(code, message);
    }
    return value;
}

/**
 * Validate the fields ingest reads. Anything else on the input, including a client filename or a
 * declared content type, is not read.
 * @param {object} input
 */
function readIngestInput(input) {
    if (!input || typeof input !== 'object') {
        throw new MediaError('invalid_input', 'Upload request is not valid.');
    }
    const bytes = toBuffer(input.bytes);
    if (!bytes) throw new MediaError('invalid_input', 'Upload bytes are required.');
    if (typeof input.actor !== 'string' || !ACTOR_PATTERN.test(input.actor)) {
        throw new MediaError('invalid_actor', 'Uploader is not valid.');
    }
    if (!isValidMediaId(input.id)) {
        throw new MediaError(
            'invalid_id',
            'Media id must be 1-64 lowercase letters, digits, and single hyphens.'
        );
    }
    const altText = input.altText == null ? '' : input.altText;
    if (typeof altText !== 'string' || altText.length > MAX_ALT_TEXT_LENGTH) {
        throw new MediaError('invalid_alt_text', 'Alt text must be text of at most 2000 characters.');
    }
    return {
        bytes,
        actor: input.actor,
        id: input.id,
        altText: altText.trim(),
        promptText: optionalText(
            input.promptText, null, MAX_PROMPT_LENGTH,
            'invalid_prompt', 'Prompt must be text of at most 4000 characters.'
        ),
        provider: optionalText(
            input.provider, PROVIDER_PATTERN, 0, 'invalid_provider', 'Provider name is not valid.'
        ),
        modelId: optionalText(
            input.modelId, MODEL_ID_PATTERN, 0, 'invalid_model_id', 'Model id is not valid.'
        )
    };
}

function assertInputSides(width, height) {
    if (!Number.isInteger(width) || !Number.isInteger(height) || width < 1 || height < 1) {
        throw refusal('malformed_image');
    }
    if (width > MAX_INPUT_SIDE || height > MAX_INPUT_SIDE) {
        throw refusal('dimensions_too_large');
    }
}

function refusal(code) {
    if (code === 'unsupported_type') return new MediaError(code, ONLY_IMAGES);
    if (code === 'animated_refused') {
        return new MediaError(code, 'Animated or multi-frame images are not accepted.');
    }
    if (code === 'dimensions_too_large') {
        return new MediaError(code, 'Image is wider or taller than 4096 pixels.');
    }
    if (code === 'decoder_unavailable') return new MediaError(code, NO_DECODER);
    return new MediaError('malformed_image', 'The image file could not be read.');
}

async function reencode(codec, bytes, format) {
    let out;
    try {
        out = await codec.decodeAndReencode(bytes, {
            format,
            maxInputSide: MAX_INPUT_SIDE,
            maxStoredSide: MAX_STORED_SIDE
        });
    } catch (err) {
        if (err && DECODER_REFUSAL_CODES.has(err.code)) throw refusal(err.code);
        throw new MediaError('reencode_failed', 'The image could not be re-encoded.');
    }
    if (!out || typeof out !== 'object') {
        throw new MediaError('reencode_failed', 'The image could not be re-encoded.');
    }
    if (out.frames !== 1) throw refusal('animated_refused');
    assertInputSides(out.width, out.height);
    const png = toBuffer(out.png);
    const stored = sniff.checkStoredPng(png);
    if (!stored.ok || Math.max(stored.width, stored.height) > MAX_STORED_SIDE) {
        throw new MediaError('reencode_failed', 'The image could not be re-encoded.');
    }
    return { png, width: stored.width, height: stored.height };
}

/**
 * Sniff, measure, decode, and re-encode one image, then store it as a pending media row.
 * Order: input fields, unused id, upload slot, byte length, sniff and header dimensions, decode
 * and re-encode, output check, write, row. A refusal at any step stores no file and no row.
 * @param {{
 *   bytes: Buffer|Uint8Array,
 *   id: string,
 *   actor: string,
 *   altText?: string,
 *   promptText?: string|null,
 *   provider?: string|null,
 *   modelId?: string|null
 * }} input
 * @param {{ codec?: ImageCodec, now?: Date|number|string }} [options]
 * @returns {Promise<{
 *   id: string, storedName: string, status: string, altText: string, width: number, height: number
 * }>}
 */
async function ingestImage(input, options) {
    const opts = options || {};
    const fields = readIngestInput(input);
    if (mediaRepo.getById(fields.id)) {
        throw new MediaError('id_taken', 'That media id is already in use.');
    }
    const slot = takeUploadSlot(fields.actor, opts.now);
    if (!slot.ok) {
        throw new MediaError('rate_limited', 'Too many uploads this hour. Try again later.', {
            retryAfterMs: slot.retryAfterMs
        });
    }

    const bytes = fields.bytes;
    if (bytes.length === 0) throw new MediaError('unsupported_type', ONLY_IMAGES);
    if (bytes.length > MAX_INPUT_BYTES) {
        throw new MediaError('too_large', 'Image is larger than 5242880 bytes.');
    }
    const header = sniff.readImageHeader(bytes);
    if (!header.ok) throw refusal(header.code);
    assertInputSides(header.width, header.height);

    const codec = opts.codec == null ? sharpCodec : opts.codec;
    if (typeof codec.decodeAndReencode !== 'function') {
        throw new MediaError('decoder_unavailable', NO_DECODER);
    }
    const out = await reencode(codec, bytes.subarray(0, header.length), header.format);

    const dir = ensureMediaDir();
    const storedName = crypto.randomBytes(STORED_NAME_BYTES).toString('hex') + '.png';
    const full = pathInside(storedName, dir);
    try {
        fs.writeFileSync(full, out.png, { flag: 'wx' });
    } catch (err) {
        if (!err || err.code !== 'EEXIST') removeQuietly(full);
        throw new MediaError('store_failed', 'The image could not be stored.');
    }
    let row;
    try {
        row = mediaRepo.insert({
            id: fields.id,
            storedName,
            status: 'pending',
            altText: fields.altText,
            promptText: fields.promptText,
            provider: fields.provider,
            modelId: fields.modelId,
            actor: fields.actor
        });
    } catch (_) {
        removeQuietly(full);
        if (mediaRepo.getById(fields.id)) {
            throw new MediaError('id_taken', 'That media id is already in use.');
        }
        throw new MediaError('store_failed', 'The image could not be stored.');
    }
    return {
        id: row.id,
        storedName: row.storedName,
        status: row.status,
        altText: row.altText,
        width: out.width,
        height: out.height
    };
}

/**
 * At least one letter or digit must remain once default-ignorable code points are removed, so
 * text made only of spaces, fillers, joiners, marks, controls, or blank symbols does not count.
 * @param {string} text
 * @returns {boolean}
 */
function hasVisibleText(text) {
    return /[\p{L}\p{N}]/u.test(text.replace(/\p{Default_Ignorable_Code_Point}/gu, ''));
}

function requireRow(id) {
    const row = typeof id === 'string' ? mediaRepo.getById(id) : null;
    if (!row) throw new MediaError('not_found', 'Media not found.');
    return row;
}

/**
 * Admin delete. Copies the live file into DATA_DIR/media/_archive/, checks the copy's SHA-256
 * against the live file, then removes the live file. A mismatch keeps the live file. The row is
 * kept, so copying the archive file back restores the image under the same id.
 * @param {{ id: string, actorKind: string }} request
 * @param {{ copyFile?: (src: string, dest: string, mode: number) => void }} [deps]
 * @returns {{ id: string, storedName: string, sha256: string }}
 */
function deleteMedia(request, deps) {
    const req = request || {};
    if (req.actorKind !== 'admin') {
        throw new MediaError('forbidden_actor', 'Only an admin can delete media.');
    }
    const row = requireRow(req.id);
    const copyFile = (deps && deps.copyFile) || fs.copyFileSync;
    const dir = ensureMediaDir();
    const live = pathInside(row.storedName, dir);
    if (fileState(live) !== 'file') {
        throw new MediaError('media_missing', 'The live media file is not present.');
    }
    const archiveFolder = path.join(dir, '_archive');
    fs.mkdirSync(archiveFolder, { recursive: true });
    assertRealInside(archiveFolder, dir);
    const archived = pathInside(row.storedName, archiveFolder);
    const liveHash = sha256File(live);

    const existing = fileState(archived);
    if (existing === 'other') {
        throw new MediaError('archive_conflict', 'The archive path is not a regular file.');
    }
    if (existing === 'file') {
        if (sha256File(archived) !== liveHash) {
            throw new MediaError('archive_conflict', 'An archive copy with different bytes exists.');
        }
    } else {
        try {
            copyFile(live, archived, fs.constants.COPYFILE_EXCL);
        } catch (err) {
            if (!err || err.code !== 'EEXIST') removeQuietly(archived);
            throw new MediaError('archive_failed', 'The archive copy failed. The live file was kept.');
        }
        if (fileState(archived) !== 'file' || sha256File(archived) !== liveHash) {
            removeQuietly(archived);
            throw new MediaError(
                'archive_verify_failed',
                'The archive copy did not match. The live file was kept.'
            );
        }
    }
    try {
        fs.unlinkSync(live);
    } catch (_) {
        throw new MediaError(
            'delete_failed',
            'The live file could not be removed. The verified archive copy was kept.'
        );
    }
    return { id: row.id, storedName: row.storedName, sha256: liveHash };
}

/**
 * Approve the pixels and the alt text in one action. Refuses the `mcp` actor kind, any kind
 * other than `admin`, alt text with no letter or digit, and a row whose live file is gone.
 * @param {{ id: string, actorKind: string, altText?: string }} request
 * @returns {object} the updated media row
 */
function approveMedia(request) {
    const req = request || {};
    if (req.actorKind === 'mcp') {
        throw new MediaError('forbidden_actor', 'The MCP actor cannot approve media.');
    }
    if (req.actorKind !== 'admin') {
        throw new MediaError('forbidden_actor', 'Only an admin can approve media.');
    }
    const row = requireRow(req.id);
    const altText = req.altText === undefined ? row.altText : req.altText;
    if (typeof altText !== 'string' || altText.length > MAX_ALT_TEXT_LENGTH) {
        throw new MediaError('invalid_alt_text', 'Alt text must be text of at most 2000 characters.');
    }
    const trimmed = altText.trim();
    if (!hasVisibleText(trimmed)) {
        throw new MediaError('alt_text_required', 'Alt text is required before approval.');
    }
    if (fileState(storedFilePath(row.storedName)) !== 'file') {
        throw new MediaError('media_missing', 'The live media file is not present.');
    }
    return mediaRepo.update(row.id, { status: 'approved', altText: trimmed });
}

/**
 * Headers for every media response. The type is always PNG, never the upload's declared type.
 * @param {import('http').ServerResponse} res
 */
function setMediaResponseHeaders(res) {
    applySecurityHeaders(res);
    res.setHeader('Content-Type', 'image/png');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Cache-Control', 'private, no-store');
}

/**
 * Stream one live PNG with the media headers. Returns false, writing nothing, when the name is
 * not a stored name or the live file is missing, so the route can answer 404.
 * @param {import('http').ServerResponse} res
 * @param {string} storedName
 * @returns {boolean}
 */
function sendStoredImage(res, storedName) {
    if (typeof storedName !== 'string' || !STORED_NAME_PATTERN.test(storedName)) return false;
    const full = storedFilePath(storedName);
    let stat;
    try {
        stat = fs.lstatSync(full);
    } catch (_) {
        return false;
    }
    if (!stat.isFile()) return false;
    setMediaResponseHeaders(res);
    res.setHeader('Content-Length', String(stat.size));
    res.writeHead(200);
    const stream = fs.createReadStream(full);
    stream.on('error', () => res.destroy());
    stream.pipe(res);
    return true;
}

module.exports = {
    ingestImage,
    deleteMedia,
    approveMedia,
    setMediaResponseHeaders,
    sendStoredImage,
    storedFilePath,
    mediaDir,
    archiveDir,
    isValidMediaId,
    MediaError,
    MAX_INPUT_BYTES,
    MAX_INPUT_SIDE,
    MAX_STORED_SIDE,
    UPLOADS_PER_ACTOR_PER_HOUR,
    STORED_NAME_PATTERN,
    _resetUploadCounts
};
