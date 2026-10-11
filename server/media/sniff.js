/**
 * Byte checks for untrusted image uploads and for re-encoded output (ADR-0004 §5).
 * Reads signatures, header dimensions, and container structure. Does not decode pixels.
 * Every function returns a result object; refusals carry a stable `code`.
 * Provenance: INIT-004/SPEC-008
 */
'use strict';

const PNG_SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
const PNG_ANIMATION_CHUNKS = new Set(['acTL', 'fcTL', 'fdAT']);

/** Chunk types a stored PNG may carry. Text, EXIF, time, ICC, and animation chunks are refused. */
const STORED_PNG_CHUNKS = new Set([
    'IHDR', 'PLTE', 'IDAT', 'IEND', 'tRNS', 'gAMA', 'cHRM', 'sRGB', 'sBIT', 'bKGD', 'pHYs'
]);

const JPEG_SOF_MARKERS = new Set([
    0xc0, 0xc1, 0xc2, 0xc3, 0xc5, 0xc6, 0xc7, 0xc9, 0xca, 0xcb, 0xcd, 0xce, 0xcf
]);
const JPEG_DIFFERENTIAL_SOF_MARKERS = new Set([0xc5, 0xc6, 0xc7, 0xcd, 0xce, 0xcf]);
const JPEG_DHP = 0xde;
const JPEG_APP2 = 0xe2;
const JPEG_SOS = 0xda;
const JPEG_EOI = 0xd9;
const JPEG_SOI_PREFIX = Buffer.from([0xff, 0xd8, 0xff]);
const MPF_IDENTIFIER = Buffer.from('MPF\0', 'latin1');

const WEBP_ANIMATION_FLAG = 0x02;

const MALFORMED = Object.freeze({ ok: false, code: 'malformed_image' });
const ANIMATED = Object.freeze({ ok: false, code: 'animated_refused' });
const UNSUPPORTED = Object.freeze({ ok: false, code: 'unsupported_type' });

/**
 * Identify PNG, JPEG, or WebP by leading bytes only.
 * @param {Buffer} bytes
 * @returns {'png'|'jpeg'|'webp'|null}
 */
function sniffFormat(bytes) {
    if (!Buffer.isBuffer(bytes)) return null;
    if (bytes.length >= PNG_SIGNATURE.length
        && bytes.subarray(0, PNG_SIGNATURE.length).equals(PNG_SIGNATURE)) {
        return 'png';
    }
    if (bytes.length >= 3 && bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
        return 'jpeg';
    }
    if (bytes.length >= 12
        && bytes.toString('latin1', 0, 4) === 'RIFF'
        && bytes.toString('latin1', 8, 12) === 'WEBP') {
        return 'webp';
    }
    return null;
}

/**
 * Walk PNG chunks from the signature to IEND.
 * @param {Buffer} bytes
 * @returns {{ ok: true, types: string[], end: number } | { ok: false, code: string }}
 */
function walkPngChunks(bytes) {
    const types = [];
    let pos = PNG_SIGNATURE.length;
    for (;;) {
        if (pos + 12 > bytes.length) return MALFORMED;
        const length = bytes.readUInt32BE(pos);
        if (length > 0x7fffffff) return MALFORMED;
        const type = bytes.toString('latin1', pos + 4, pos + 8);
        if (!/^[A-Za-z]{4}$/.test(type)) return MALFORMED;
        const next = pos + 12 + length;
        if (next > bytes.length) return MALFORMED;
        types.push(type);
        pos = next;
        if (type === 'IEND') return { ok: true, types, end: pos };
    }
}

/**
 * @param {Buffer} bytes
 * @returns {{ ok: true, format: 'png', width: number, height: number, chunks: string[], end: number }
 *   | { ok: false, code: string }}
 */
function readPngHeader(bytes) {
    if (bytes.length < 33) return MALFORMED;
    if (bytes.readUInt32BE(8) !== 13 || bytes.toString('latin1', 12, 16) !== 'IHDR') {
        return MALFORMED;
    }
    const width = bytes.readUInt32BE(16);
    const height = bytes.readUInt32BE(20);
    const walk = walkPngChunks(bytes);
    if (!walk.ok) return walk;
    if (walk.types.some((type) => PNG_ANIMATION_CHUNKS.has(type))) return ANIMATED;
    if (!walk.types.includes('IDAT')) return MALFORMED;
    return { ok: true, format: 'png', width, height, chunks: walk.types, end: walk.end };
}

/**
 * Skip entropy-coded scan data. Returns the offset of the next marker's 0xFF, or the buffer length.
 * @param {Buffer} bytes
 * @param {number} start
 * @returns {number}
 */
function skipJpegScan(bytes, start) {
    let pos = start;
    for (;;) {
        pos = bytes.indexOf(0xff, pos);
        if (pos === -1 || pos + 1 >= bytes.length) return bytes.length;
        const next = bytes[pos + 1];
        if (next === 0x00 || (next >= 0xd0 && next <= 0xd7)) {
            pos += 2;
            continue;
        }
        return pos;
    }
}

/**
 * Walk JPEG markers. Refuses a file with no EOI, and more than one frame: a second SOF, a
 * hierarchical or differential frame, an MPF (multi-picture) segment, or another SOI after the EOI.
 * @param {Buffer} bytes
 * @returns {{ ok: true, format: 'jpeg', width: number, height: number } | { ok: false, code: string }}
 */
function readJpegHeader(bytes) {
    let pos = 2;
    let frames = 0;
    let width = 0;
    let height = 0;
    let eoiEnd = -1;
    while (pos < bytes.length) {
        if (bytes[pos] !== 0xff) return MALFORMED;
        while (pos < bytes.length && bytes[pos] === 0xff) pos += 1;
        if (pos >= bytes.length) return MALFORMED;
        const marker = bytes[pos];
        pos += 1;
        if (marker === JPEG_EOI) {
            eoiEnd = pos;
            break;
        }
        if (marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) continue;
        if (marker === 0x00 || marker === 0xd8) return MALFORMED;
        if (pos + 2 > bytes.length) return MALFORMED;
        const length = bytes.readUInt16BE(pos);
        if (length < 2 || pos + length > bytes.length) return MALFORMED;
        const body = pos + 2;
        if (JPEG_SOF_MARKERS.has(marker)) {
            frames += 1;
            if (frames > 1 || JPEG_DIFFERENTIAL_SOF_MARKERS.has(marker)) return ANIMATED;
            if (length < 8) return MALFORMED;
            height = bytes.readUInt16BE(body + 1);
            width = bytes.readUInt16BE(body + 3);
        } else if (marker === JPEG_DHP) {
            return ANIMATED;
        } else if (marker === JPEG_APP2 && length >= 6
            && bytes.subarray(body, body + MPF_IDENTIFIER.length).equals(MPF_IDENTIFIER)) {
            return ANIMATED;
        }
        pos += length;
        if (marker === JPEG_SOS) pos = skipJpegScan(bytes, pos);
    }
    if (frames !== 1 || width === 0 || height === 0 || eoiEnd === -1) return MALFORMED;
    if (bytes.indexOf(JPEG_SOI_PREFIX, eoiEnd) !== -1) return ANIMATED;
    return { ok: true, format: 'jpeg', width, height, end: eoiEnd };
}

/**
 * @param {Buffer} bytes
 * @param {number} body
 * @param {number} size
 * @returns {{ width: number, height: number }|null}
 */
function readVp8Frame(bytes, body, size) {
    if (size < 10) return null;
    if ((bytes[body] & 0x01) !== 0) return null;
    if (bytes[body + 3] !== 0x9d || bytes[body + 4] !== 0x01 || bytes[body + 5] !== 0x2a) {
        return null;
    }
    const width = bytes.readUInt16LE(body + 6) & 0x3fff;
    const height = bytes.readUInt16LE(body + 8) & 0x3fff;
    if (width === 0 || height === 0) return null;
    return { width, height };
}

/**
 * @param {Buffer} bytes
 * @param {number} body
 * @param {number} size
 * @returns {{ width: number, height: number }|null}
 */
function readVp8lFrame(bytes, body, size) {
    if (size < 5 || bytes[body] !== 0x2f) return null;
    const bits = bytes.readUInt32LE(body + 1);
    if ((bits >>> 29) !== 0) return null;
    return { width: (bits & 0x3fff) + 1, height: ((bits >>> 14) & 0x3fff) + 1 };
}

/**
 * Walk the RIFF chunks of a WebP file. Refuses the animation flag, ANIM/ANMF chunks, a second
 * image bitstream, and a VP8X canvas that does not match the image.
 * @param {Buffer} bytes
 * @returns {{ ok: true, format: 'webp', width: number, height: number } | { ok: false, code: string }}
 */
function readWebpHeader(bytes) {
    if (bytes.length < 20) return MALFORMED;
    const riffEnd = 8 + bytes.readUInt32LE(4);
    if (riffEnd < 20 || riffEnd > bytes.length) return MALFORMED;
    let pos = 12;
    let canvas = null;
    let image = null;
    while (pos < riffEnd) {
        if (pos + 8 > riffEnd) return MALFORMED;
        const fourcc = bytes.toString('latin1', pos, pos + 4);
        const size = bytes.readUInt32LE(pos + 4);
        const body = pos + 8;
        if (body + size > riffEnd) return MALFORMED;
        if (pos === 12 && fourcc !== 'VP8 ' && fourcc !== 'VP8L' && fourcc !== 'VP8X') {
            return MALFORMED;
        }
        if (fourcc === 'ANIM' || fourcc === 'ANMF') return ANIMATED;
        if (fourcc === 'VP8X') {
            if (pos !== 12 || size < 10) return MALFORMED;
            if (bytes[body] & WEBP_ANIMATION_FLAG) return ANIMATED;
            canvas = {
                width: 1 + bytes.readUIntLE(body + 4, 3),
                height: 1 + bytes.readUIntLE(body + 7, 3)
            };
        } else if (fourcc === 'VP8 ' || fourcc === 'VP8L') {
            if (image) return ANIMATED;
            image = fourcc === 'VP8 '
                ? readVp8Frame(bytes, body, size)
                : readVp8lFrame(bytes, body, size);
            if (!image) return MALFORMED;
        }
        pos = body + size + (size & 1);
    }
    if (!image) return MALFORMED;
    if (canvas && (canvas.width !== image.width || canvas.height !== image.height)) {
        return MALFORMED;
    }
    return { ok: true, format: 'webp', width: image.width, height: image.height, end: riffEnd };
}

/**
 * Sniff the format, then read header dimensions and refuse multi-frame structure.
 * The client filename and declared type are not inputs. `length` is the validated span: bytes
 * after PNG IEND, after the RIFF size, or after the JPEG EOI are not part of it.
 * @param {Buffer} bytes
 * @returns {{ ok: true, format: 'png'|'jpeg'|'webp', width: number, height: number, length: number }
 *   | { ok: false, code: 'unsupported_type'|'malformed_image'|'animated_refused' }}
 */
function readImageHeader(bytes) {
    const format = sniffFormat(bytes);
    if (!format) return UNSUPPORTED;
    let header;
    if (format === 'png') header = readPngHeader(bytes);
    else if (format === 'jpeg') header = readJpegHeader(bytes);
    else header = readWebpHeader(bytes);
    if (!header.ok) return header;
    if (!(header.width >= 1) || !(header.height >= 1)) return MALFORMED;
    return { ok: true, format, width: header.width, height: header.height, length: header.end };
}

/**
 * Confirm re-encoded output is one static PNG with only allowed chunks and nothing after IEND.
 * @param {Buffer} bytes
 * @returns {{ ok: true, width: number, height: number }
 *   | { ok: false, code: 'reencode_failed', reason: string }}
 */
function checkStoredPng(bytes) {
    if (sniffFormat(bytes) !== 'png') {
        return { ok: false, code: 'reencode_failed', reason: 'not_png' };
    }
    const header = readPngHeader(bytes);
    if (!header.ok) {
        const reason = header.code === 'animated_refused' ? 'animated' : 'malformed';
        return { ok: false, code: 'reencode_failed', reason };
    }
    if (header.end !== bytes.length) {
        return { ok: false, code: 'reencode_failed', reason: 'trailing_data' };
    }
    if (header.chunks.some((type) => !STORED_PNG_CHUNKS.has(type))) {
        return { ok: false, code: 'reencode_failed', reason: 'chunk_not_allowed' };
    }
    if (!(header.width >= 1) || !(header.height >= 1)) {
        return { ok: false, code: 'reencode_failed', reason: 'malformed' };
    }
    return { ok: true, width: header.width, height: header.height };
}

module.exports = {
    sniffFormat,
    readImageHeader,
    checkStoredPng,
    STORED_PNG_CHUNKS
};
