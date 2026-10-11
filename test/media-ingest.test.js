/**
 * Media ingest, archive-before-delete, and approval on a temporary DATA_DIR. No network.
 * The default codec is sharp: the decoder tests run real decode and re-encode on the fixtures.
 * The pipeline-wiring tests inject a codec that returns a fixed PNG.
 * Provenance: INIT-004/SPEC-008
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { PassThrough } = require('stream');
const zlib = require('zlib');
const sharp = require('sharp');

const root = path.resolve(__dirname, '..');
const FIXTURES = path.join(__dirname, 'fixtures', 'media');
const STORED_NAME = /^[0-9a-f]{64}\.png$/;
const T0 = Date.UTC(2026, 9, 9, 10, 0, 0);
const MINUTE = 60 * 1000;

function fixture(name) {
    return fs.readFileSync(path.join(FIXTURES, name));
}

const CRC_TABLE = (() => {
    const table = new Uint32Array(256);
    for (let n = 0; n < 256; n += 1) {
        let c = n;
        for (let k = 0; k < 8; k += 1) c = (c & 1) ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1);
        table[n] = c >>> 0;
    }
    return table;
})();

function crc32(buf) {
    let c = 0xffffffff;
    for (const byte of buf) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
}

function pngChunk(type, data) {
    const head = Buffer.alloc(8);
    head.writeUInt32BE(data.length, 0);
    head.write(type, 4, 'latin1');
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(crc32(Buffer.concat([head.subarray(4), data])), 0);
    return Buffer.concat([head, data, crc]);
}

/** 8-bit grayscale PNG; extra chunks go between IHDR and IDAT. */
function makePng(width, height, extraChunks) {
    const ihdr = Buffer.alloc(13);
    ihdr.writeUInt32BE(width, 0);
    ihdr.writeUInt32BE(height, 4);
    ihdr[8] = 8;
    return Buffer.concat([
        Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
        pngChunk('IHDR', ihdr),
        ...(extraChunks || []).map(([type, data]) => pngChunk(type, data)),
        pngChunk('IDAT', zlib.deflateSync(Buffer.alloc((width + 1) * height))),
        pngChunk('IEND', Buffer.alloc(0))
    ]);
}

function sha256(buf) {
    return crypto.createHash('sha256').update(buf).digest('hex');
}

function pngChunkTypes(png) {
    const types = [];
    for (let pos = 8; pos + 8 <= png.length; pos += 12 + png.readUInt32BE(pos)) {
        types.push(png.toString('latin1', pos + 4, pos + 8));
    }
    return types;
}

/** The JPEG with an EXIF APP1 segment after SOI that holds only an Orientation tag. */
function withExifOrientation(jpeg, orientation) {
    const tiff = Buffer.alloc(26);
    tiff.write('II*\0', 0, 'latin1');
    tiff.writeUInt32LE(8, 4);
    tiff.writeUInt16LE(1, 8);
    tiff.writeUInt16LE(0x0112, 10);
    tiff.writeUInt16LE(3, 12);
    tiff.writeUInt32LE(1, 14);
    tiff.writeUInt16LE(orientation, 18);
    const body = Buffer.concat([Buffer.from('Exif\0\0', 'latin1'), tiff]);
    const head = Buffer.from([0xff, 0xe1, 0, 0]);
    head.writeUInt16BE(body.length + 2, 2);
    return Buffer.concat([jpeg.subarray(0, 2), head, body, jpeg.subarray(2)]);
}

/** The RATIONAL values in the GPS IFD of a big-endian EXIF block, as raw bytes. */
function exifGpsValues(jpeg) {
    const tiff = jpeg.indexOf('Exif\0\0', 0, 'latin1') + 6;
    assert.equal(jpeg.toString('latin1', tiff, tiff + 2), 'MM');
    const u16 = (at) => jpeg.readUInt16BE(tiff + at);
    const u32 = (at) => jpeg.readUInt32BE(tiff + at);
    const entries = (at) => Array.from({ length: u16(at) }, (_, i) => at + 2 + i * 12);
    const gpsPointer = entries(u32(4)).find((entry) => u16(entry) === 0x8825);
    return entries(u32(gpsPointer + 8))
        .filter((entry) => u16(entry + 2) === 5)
        .map((entry) => jpeg.subarray(tiff + u32(entry + 8), tiff + u32(entry + 8) + 8 * u32(entry + 4)));
}

/** Offset and total size (marker, length field, body) of the first SOF segment. */
function jpegSof(jpeg) {
    let pos = 2;
    while (pos + 4 <= jpeg.length && jpeg[pos] === 0xff) {
        const marker = jpeg[pos + 1];
        const size = 2 + jpeg.readUInt16BE(pos + 2);
        if (marker >= 0xc0 && marker <= 0xcf && ![0xc4, 0xc8, 0xcc].includes(marker)) {
            return { at: pos, size };
        }
        pos += size;
    }
    throw new Error('fixture has no SOF segment');
}

class FakeResponse extends PassThrough {
    constructor() {
        super();
        this.headers = {};
        this.statusCode = null;
        this.chunks = [];
        this.on('data', (chunk) => this.chunks.push(chunk));
    }

    setHeader(key, value) {
        this.headers[key.toLowerCase()] = value;
    }

    getHeader(key) {
        return this.headers[key.toLowerCase()];
    }

    writeHead(status) {
        this.statusCode = status;
    }

    body() {
        return new Promise((resolve, reject) => {
            this.on('end', () => resolve(Buffer.concat(this.chunks)));
            this.on('error', reject);
        });
    }
}

async function assertRefused(promise, code) {
    await assert.rejects(promise, (err) => {
        assert.equal(err.name, 'MediaError');
        assert.equal(err.code, code);
        return true;
    });
}

function assertThrowsCode(fn, code) {
    assert.throws(fn, (err) => {
        assert.equal(err.name, 'MediaError');
        assert.equal(err.code, code);
        return true;
    });
}

describe('media ingest pipeline', () => {
    const savedEnv = {
        HEBREW_ROOT: process.env.HEBREW_ROOT,
        DATA_DIR: process.env.DATA_DIR,
        LESSONS_DIR: process.env.LESSONS_DIR
    };
    let tmp;
    let db;
    let media;
    let sniff;
    let codecModule;
    let ingest;
    const CODEC_OUTPUT = makePng(4, 3);

    /** Every file under DATA_DIR except the database, as forward-slash relative paths. */
    function allFiles() {
        return fs.readdirSync(tmp, { recursive: true })
            .map((rel) => String(rel).split(path.sep).join('/'))
            .filter((rel) => !rel.startsWith('hebrew.sqlite'))
            .filter((rel) => fs.statSync(path.join(tmp, rel)).isFile())
            .sort();
    }

    function rowCount() {
        return db.get('SELECT COUNT(*) AS n FROM media').n;
    }

    function recordingCodec(output) {
        const calls = [];
        return {
            calls,
            async decodeAndReencode(bytes, limits) {
                calls.push({ length: bytes.length, limits });
                const header = sniff.readImageHeader(bytes);
                return {
                    width: header.width,
                    height: header.height,
                    frames: 1,
                    png: output || CODEC_OUTPUT
                };
            }
        };
    }

    function codecReturning(result) {
        return { async decodeAndReencode() { return result; } };
    }

    function upload(id, extra, options) {
        return ingest.ingestImage(
            Object.assign({ bytes: fixture('tiny.png'), id, actor: 'admin:alice' }, extra),
            Object.assign({ codec: recordingCodec(), now: T0 }, options)
        );
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-ingest-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete process.env.LESSONS_DIR;
        for (const mod of [
            '../server/paths',
            '../server/db',
            '../server/db/repos/media',
            '../server/media/sniff',
            '../server/media/codec',
            '../server/media/ingest'
        ]) {
            try {
                delete require.cache[require.resolve(mod)];
            } catch (_) { /* not loaded yet */ }
        }
        db = require('../server/db');
        db.close();
        db.migrate();
        media = require('../server/db/repos/media');
        sniff = require('../server/media/sniff');
        codecModule = require('../server/media/codec');
        ingest = require('../server/media/ingest');
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        for (const [key, value] of Object.entries(savedEnv)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
    });

    beforeEach(() => {
        ingest._resetUploadCounts();
    });

    it('sniffs PNG, JPEG, and WebP by bytes and reads header dimensions', () => {
        const small = [
            ['tiny.png', 'png'],
            ['tiny.jpg', 'jpeg'],
            ['tiny-progressive.jpg', 'jpeg'],
            ['exif-gps.jpg', 'jpeg'],
            ['tiny.webp', 'webp'],
            ['tiny-lossless.webp', 'webp'],
            ['tiny-alpha.webp', 'webp']
        ];
        for (const [name, format] of small) {
            assert.deepEqual(
                sniff.readImageHeader(fixture(name)),
                { ok: true, format, width: 8, height: 6, length: fixture(name).length },
                name
            );
        }
        const large = [
            ['edge-4096.png', 4096, 1],
            ['wide-4097.png', 4097, 1],
            ['tall-4097.jpg', 1, 4097],
            ['wide-4097.webp', 4097, 1],
            ['wide-4097-alpha.webp', 4097, 1]
        ];
        for (const [name, width, height] of large) {
            const header = sniff.readImageHeader(fixture(name));
            assert.equal(header.ok, true, name);
            assert.equal(header.width, width, name);
            assert.equal(header.height, height, name);
        }
        assert.equal(sniff.sniffFormat(fixture('svg-named.png')), null);

        const jpeg = fixture('tiny.jpg');
        assert.deepEqual(jpeg.subarray(-2), Buffer.from([0xff, 0xd9]));
        assert.deepEqual(
            sniff.readImageHeader(jpeg.subarray(0, -2)),
            { ok: false, code: 'malformed_image' }
        );
    });

    it('ac-1: a file that starts with <svg and is named .png is refused and leaves no file', async () => {
        const codec = recordingCodec();
        const filesBefore = allFiles();
        const rowsBefore = rowCount();
        const svg = fixture('svg-named.png');
        assert.equal(svg.toString('latin1', 0, 4), '<svg');
        await assertRefused(ingest.ingestImage({
            bytes: svg,
            id: 'svg-as-png',
            actor: 'admin:alice',
            filename: 'svg-named.png',
            contentType: 'image/png'
        }, { codec, now: T0 }), 'unsupported_type');

        const otherRefusals = [
            ['bom-svg', Buffer.from('\uFEFF<svg xmlns="http://www.w3.org/2000/svg"/>', 'utf8')],
            ['xml-svg', Buffer.from('<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>')],
            ['spaced-svg', Buffer.from('   <svg/>')],
            ['html-page', Buffer.from('<!DOCTYPE html><html><script>alert(1)</script></html>')],
            ['gif-image', Buffer.from('GIF89a\x01\x00\x01\x00\x00\x00\x00;', 'latin1')],
            ['bmp-image', Buffer.from('BM\x1e\x00\x00\x00\x00\x00\x00\x00', 'latin1')],
            ['random-bytes', Buffer.concat([Buffer.from([0]), crypto.randomBytes(63)])],
            ['empty-file', Buffer.alloc(0)]
        ];
        for (const [id, bytes] of otherRefusals) {
            await assertRefused(ingest.ingestImage(
                { bytes, id, actor: 'admin:alice', filename: id + '.png' },
                { codec, now: T0 }
            ), 'unsupported_type');
        }

        const signatureThenJunk = [
            ['png-junk', Buffer.concat([fixture('tiny.png').subarray(0, 8), Buffer.from('<svg/>')])],
            ['jpeg-junk', Buffer.concat([Buffer.from([0xff, 0xd8, 0xff]), Buffer.from('<svg/>')])],
            ['webp-junk', Buffer.concat([
                Buffer.from('RIFF\x10\x00\x00\x00WEBP', 'latin1'),
                Buffer.from('<svg/><svg/>')
            ])],
            ['png-truncated', fixture('tiny.png').subarray(0, 40)],
            ['jpeg-no-eoi', Buffer.concat([
                fixture('tiny.jpg').subarray(0, -2),
                Buffer.from('<svg onload="alert(1)"/><script>alert(2)</script>')
            ])]
        ];
        for (const [id, bytes] of signatureThenJunk) {
            await assertRefused(ingest.ingestImage(
                { bytes, id, actor: 'admin:alice' },
                { codec, now: T0 }
            ), 'malformed_image');
        }

        assert.equal(codec.calls.length, 0);
        assert.deepEqual(allFiles(), filesBefore);
        assert.equal(rowCount(), rowsBefore);
        for (const id of ['svg-as-png'].concat(otherRefusals.map(([name]) => name))) {
            assert.equal(media.getById(id), null, id);
        }
    });

    it('ac-1: with the default sharp codec, PNG, JPEG, and WebP each become one re-encoded PNG with a 64-hex name', async () => {
        const cases = [
            ['tiny.png', undefined],
            ['tiny.jpg', { now: T0 }],
            ['tiny.webp', { codec: null, now: T0 }],
            ['tiny-progressive.jpg', { now: T0 }],
            ['tiny-lossless.webp', { now: T0 }],
            ['tiny-alpha.webp', { now: T0 }]
        ];
        for (const [name, options] of cases) {
            const input = fixture(name);
            const id = 'real-' + name.replace(/\W+/g, '-');
            const before = allFiles();
            const result = await ingest.ingestImage(
                { bytes: input, id, actor: options ? 'admin:alice' : 'admin:ivy', altText: 'a pattern' },
                options
            );

            assert.deepEqual(allFiles().filter((file) => !before.includes(file)), ['media/' + result.storedName], name);
            assert.match(result.storedName, STORED_NAME);
            const stored = fs.readFileSync(ingest.storedFilePath(result.storedName));
            assert.deepEqual(sniff.checkStoredPng(stored), { ok: true, width: 8, height: 6 }, name);
            assert.notDeepEqual(stored, input, name);
            assert.deepEqual(
                await sharp(stored).raw().toBuffer(),
                await sharp(input).raw().toBuffer(),
                name + ': the stored PNG holds the decoded pixels'
            );
            assert.deepEqual(result, {
                id, storedName: result.storedName, status: 'pending', altText: 'a pattern', width: 8, height: 6
            });
            assert.equal(media.getById(id).storedName, result.storedName);
        }

        const edge = await ingest.ingestImage(
            { bytes: fixture('edge-4096.png'), id: 'real-edge-4096', actor: 'admin:alice' },
            { now: T0 }
        );
        const resized = fs.readFileSync(ingest.storedFilePath(edge.storedName));
        assert.deepEqual(sniff.checkStoredPng(resized), { ok: true, width: 2048, height: 1 });
        assert.equal(edge.width, 2048);
    });

    it('ac-1: exif-gps.jpg is stored with no eXIf chunk and no GPS or camera bytes, after EXIF orientation is applied', async () => {
        const gpsJpeg = fixture('exif-gps.jpg');
        const gpsValues = exifGpsValues(gpsJpeg);
        assert.equal(gpsValues.length, 2);
        const rotated = withExifOrientation(fixture('tiny.jpg'), 6);
        assert.equal((await sharp(rotated).metadata()).orientation, 6);
        const cases = [
            ['meta-exif-gps', gpsJpeg, 8, 6],
            ['meta-exif-png', fixture('exif-chunk.png'), 8, 6],
            ['meta-text-png', fixture('text-chunk.png'), 8, 6],
            ['meta-rotated', rotated, 6, 8]
        ];
        for (const [id, input, width, height] of cases) {
            const saved = await ingest.ingestImage({ bytes: input, id, actor: 'admin:alice' }, { now: T0 });
            const stored = fs.readFileSync(ingest.storedFilePath(saved.storedName));
            assert.deepEqual(sniff.checkStoredPng(stored), { ok: true, width, height }, id);
            assert.deepEqual([saved.width, saved.height], [width, height], id);
            const types = pngChunkTypes(stored);
            for (const type of ['eXIf', 'tEXt', 'iTXt', 'zTXt', 'iCCP', 'tIME', 'acTL']) {
                assert.equal(types.includes(type), false, id + ' has ' + type);
            }
            for (const marker of ['Exif', 'II*\0', 'MM\0*', 'TestMake', 'TestModel', 'fixture text chunk']) {
                assert.equal(stored.includes(marker, 0, 'latin1'), false, id + ' contains ' + JSON.stringify(marker));
            }
            for (const value of gpsValues) assert.equal(stored.includes(value), false, id + ' contains GPS bytes');
            const meta = await sharp(stored).metadata();
            for (const key of ['exif', 'orientation', 'icc', 'xmp', 'iptc', 'comments']) {
                assert.equal(meta[key], undefined, id + ' keeps ' + key);
            }
        }
    });

    it('ac-1: the input\'s resolution does not reach the stored PNG', async () => {
        const oddPhys = Buffer.from('deadbeef0badf00d01', 'hex');
        const inputs = [
            ['res-plain', fixture('tiny.png')],
            ['res-odd-phys', makePng(8, 6, [['pHYs', oddPhys]])],
            ['res-jfif-314', await sharp(fixture('tiny.jpg')).withMetadata({ density: 314 }).jpeg().toBuffer()],
            ['res-exif-gps', fixture('exif-gps.jpg')]
        ];
        assert.equal((await sharp(inputs[2][1]).metadata()).density, 314);
        const physValues = new Set();
        for (const [id, bytes] of inputs) {
            const saved = await ingest.ingestImage({ bytes, id, actor: 'admin:alice' }, { now: T0 });
            const stored = fs.readFileSync(ingest.storedFilePath(saved.storedName));
            assert.equal(stored.includes(oddPhys.subarray(0, 8)), false, id);
            const at = stored.indexOf('pHYs', 0, 'latin1');
            physValues.add(at === -1 ? 'none' : stored.subarray(at + 4, at + 13).toString('hex'));
        }
        assert.equal(physValues.size, 1, [...physValues].join(' '));
    });

    it('ac-1: a buffer the sniff accepts but the decoder rejects stores nothing', async () => {
        const flipped = Buffer.from(fixture('tiny.png'));
        flipped[flipped.length - 20] ^= 0xff;
        const ihdr = Buffer.alloc(13);
        ihdr.writeUInt32BE(8, 0);
        ihdr.writeUInt32BE(6, 4);
        ihdr[8] = 8;
        ihdr[9] = 2;
        const notZlib = Buffer.concat([
            fixture('tiny.png').subarray(0, 8),
            pngChunk('IHDR', ihdr),
            pngChunk('IDAT', Buffer.from('these bytes are not zlib data')),
            pngChunk('IEND', Buffer.alloc(0))
        ]);
        const before = allFiles();
        const rowsBefore = rowCount();
        for (const [id, bytes] of [['decode-flipped-bit', flipped], ['decode-not-zlib', notZlib]]) {
            assert.equal(sniff.readImageHeader(bytes).ok, true, id);
            await assertRefused(
                ingest.ingestImage({ bytes, id, actor: 'admin:alice' }, { now: T0 }),
                'malformed_image'
            );
            assert.equal(media.getById(id), null, id);
        }
        assert.deepEqual(allFiles(), before);
        assert.equal(rowCount(), rowsBefore);
    });

    it('the sharp codec checks the decoder\'s own format, frame count, and size before decoding pixels', async () => {
        const limits = { maxInputSide: 4096, maxStoredSide: 2048 };
        const refusals = [
            ['svg-named.png', 'png', 'malformed_image'],
            ['tiny.jpg', 'png', 'malformed_image'],
            ['tiny.png', 'webp', 'malformed_image'],
            ['animated.webp', 'webp', 'animated_refused'],
            ['wide-4097.png', 'png', 'dimensions_too_large'],
            ['tall-4097.jpg', 'jpeg', 'dimensions_too_large'],
            ['wide-4097-alpha.webp', 'webp', 'dimensions_too_large']
        ];
        for (const [name, format, code] of refusals) {
            await assert.rejects(
                codecModule.sharpCodec.decodeAndReencode(fixture(name), Object.assign({ format }, limits)),
                (err) => {
                    assert.equal(err.code, code, name);
                    return true;
                }
            );
        }
        await assert.rejects(
            codecModule.sharpCodec.decodeAndReencode(Buffer.from('not an image'), Object.assign({ format: 'png' }, limits)),
            { code: 'malformed_image' }
        );
        const badLimits = [
            { format: 'png', maxInputSide: 0, maxStoredSide: 2048 },
            { format: 'png', maxStoredSide: 2048 },
            { format: 'png', maxInputSide: 4096, maxStoredSide: 2048.5 },
            { format: 'svg', maxInputSide: 4096, maxStoredSide: 2048 },
            undefined
        ];
        for (const bad of badLimits) {
            await assert.rejects(codecModule.sharpCodec.decodeAndReencode(fixture('tiny.png'), bad), (err) => {
                assert.equal(err.code, undefined, JSON.stringify(bad));
                return true;
            });
        }

        const out = await codecModule.sharpCodec.decodeAndReencode(
            fixture('edge-4096.png'),
            Object.assign({ format: 'png' }, limits)
        );
        assert.deepEqual(
            { width: out.width, height: out.height, frames: out.frames },
            { width: 4096, height: 1, frames: 1 }
        );
        assert.deepEqual(sniff.checkStoredPng(out.png), { ok: true, width: 2048, height: 1 });
    });

    it('ac-1: when the decoder cannot load, or the codec has no decode function, nothing is stored', async () => {
        const filesBefore = allFiles();
        const rowsBefore = rowCount();
        const missing = codecModule.createSharpCodec(() => {
            throw new Error("Cannot find module 'sharp'");
        });
        for (const name of ['tiny.png', 'tiny.jpg', 'tiny.webp']) {
            const id = 'no-decoder-' + name.split('.')[1];
            await assertRefused(ingest.ingestImage(
                { bytes: fixture(name), id, actor: 'admin:alice' },
                { codec: missing, now: T0 }
            ), 'decoder_unavailable');
            assert.equal(media.getById(id), null, id);
        }
        for (const [id, notACodec] of [['codec-empty', {}], ['codec-not-fn', { decodeAndReencode: 'x' }], ['codec-false', false]]) {
            await assertRefused(ingest.ingestImage(
                { bytes: fixture('tiny.png'), id, actor: 'admin:alice' },
                { codec: notACodec, now: T0 }
            ), 'decoder_unavailable');
            assert.equal(media.getById(id), null, id);
        }
        assert.deepEqual(allFiles(), filesBefore);
        assert.equal(rowCount(), rowsBefore);
    });

    it('ac-1 wiring: PNG, JPEG, and WebP each become one PNG under DATA_DIR/media with a 64-hex name', async () => {
        const codec = recordingCodec();
        const names = [];
        for (const [file, format] of [['tiny.png', 'png'], ['tiny.jpg', 'jpeg'], ['tiny.webp', 'webp']]) {
            const input = fixture(file);
            const before = allFiles();
            const result = await ingest.ingestImage({
                bytes: input,
                id: 'wire-' + format,
                actor: 'admin:alice',
                altText: '  a cup  ',
                status: 'approved',
                filename: 'holiday-photo.' + format,
                contentType: 'image/svg+xml'
            }, { codec, now: T0 });

            const added = allFiles().filter((name) => !before.includes(name));
            assert.deepEqual(added, ['media/' + result.storedName], file);
            assert.match(result.storedName, STORED_NAME);
            names.push(result.storedName);

            const full = ingest.storedFilePath(result.storedName);
            assert.equal(path.dirname(full), path.resolve(tmp, 'media'));
            assert.equal(full.startsWith(path.join(root, 'worksheets')), false);
            assert.equal(full.startsWith(path.join(root, 'shared')), false);
            const stored = fs.readFileSync(full);
            assert.deepEqual(stored, CODEC_OUTPUT);
            assert.notDeepEqual(stored, input);

            assert.deepEqual(codec.calls.at(-1).limits, {
                format,
                maxInputSide: 4096,
                maxStoredSide: 2048
            });
            assert.deepEqual(result, {
                id: 'wire-' + format,
                storedName: result.storedName,
                status: 'pending',
                altText: 'a cup',
                width: 4,
                height: 3
            });
            const row = media.getById('wire-' + format);
            assert.equal(row.status, 'pending');
            assert.equal(row.storedName, result.storedName);
            assert.equal(row.altText, 'a cup');
            assert.equal(row.actor, 'admin:alice');
            assert.equal(row.promptText, null);
            assert.equal(JSON.stringify(row).includes('holiday-photo'), false);
        }
        assert.equal(new Set(names).size, 3);
        const everyName = fs.readdirSync(tmp, { recursive: true }).map(String);
        assert.equal(everyName.some((name) => name.includes('holiday-photo')), false);
        const columns = db.all('PRAGMA table_info(media)').map((column) => column.name);
        assert.equal(columns.some((name) => /file_?name|original/i.test(name)), false);
        const allText = db.all('SELECT * FROM media').map((row) => JSON.stringify(row)).join('\n');
        assert.equal(allText.includes('holiday-photo'), false);
    });

    it('hands the decoder only the validated span, not bytes appended after the image', async () => {
        const payload = Buffer.from('<svg onload="alert(1)"/><script>alert(2)</script>');
        const cases = [
            ['span-png', fixture('tiny.png')],
            ['span-webp', fixture('tiny.webp')],
            ['span-jpeg', fixture('tiny.jpg')]
        ];
        for (const [id, image] of cases) {
            const seen = [];
            const codec = {
                async decodeAndReencode(bytes) {
                    seen.push(Buffer.from(bytes));
                    return { width: 8, height: 6, frames: 1, png: CODEC_OUTPUT };
                }
            };
            await upload(id, { bytes: Buffer.concat([image, payload]) }, { codec });
            assert.equal(seen.length, 1, id);
            assert.deepEqual(seen[0], image, id);
        }
    });

    it('stores decoder output only when it is one static PNG with no metadata chunks', async () => {
        const before = allFiles();
        const refusedOutputs = [
            ['out-exif', fixture('exif-chunk.png')],
            ['out-text', fixture('text-chunk.png')],
            ['out-iccp', makePng(4, 3, [['iCCP', Buffer.from('profile\0\0x')]])],
            ['out-time', makePng(4, 3, [['tIME', Buffer.alloc(7)]])],
            ['out-apng', fixture('animated.png')],
            ['out-jpeg', fixture('tiny.jpg')],
            ['out-trailing', Buffer.concat([makePng(4, 3), Buffer.from('<svg/>')])],
            ['out-too-wide', makePng(2049, 1)]
        ];
        for (const [id, png] of refusedOutputs) {
            await assertRefused(upload(id, {}, { codec: recordingCodec(png) }), 'reencode_failed');
        }
        const twoFrames = codecReturning({ width: 8, height: 6, frames: 2, png: CODEC_OUTPUT });
        await assertRefused(upload('out-frames', {}, { codec: twoFrames }), 'animated_refused');
        const decodedWide = codecReturning({ width: 5000, height: 6, frames: 1, png: CODEC_OUTPUT });
        await assertRefused(
            upload('out-decoded-wide', {}, { codec: decodedWide }),
            'dimensions_too_large'
        );
        await assertRefused(upload('out-throws', {}, {
            codec: { async decodeAndReencode() { throw new Error('decoder exploded'); } }
        }), 'reencode_failed');
        await assertRefused(upload('out-rejects', {}, {
            codec: {
                async decodeAndReencode() {
                    throw Object.assign(new Error('bad'), { code: 'malformed_image' });
                }
            }
        }), 'malformed_image');
        assert.deepEqual(allFiles(), before);
        for (const [id] of refusedOutputs) assert.equal(media.getById(id), null, id);

        const edge = await upload('out-2048', {}, { codec: recordingCodec(makePng(2048, 1)) });
        assert.equal(edge.width, 2048);
        assert.equal(allFiles().length, before.length + 1);
    });

    it('a write that fails partway leaves no file, and a name collision neither overwrites nor removes the other file', async (t) => {
        fs.mkdirSync(path.join(tmp, 'media'), { recursive: true });
        const before = allFiles();
        const realWrite = fs.writeFileSync;
        const write = t.mock.method(fs, 'writeFileSync', (file, data, options) => {
            realWrite.call(fs, file, data.subarray(0, 16), options);
            throw Object.assign(new Error('no space left on device'), { code: 'ENOSPC' });
        });
        await assertRefused(upload('write-partial'), 'store_failed');
        assert.equal(write.mock.callCount(), 1);
        write.mock.restore();
        assert.deepEqual(allFiles(), before);
        assert.equal(media.getById('write-partial'), null);

        const takenName = 'ab'.repeat(32) + '.png';
        const taken = path.join(tmp, 'media', takenName);
        fs.writeFileSync(taken, 'bytes that belong to another row');
        t.mock.method(crypto, 'randomBytes', () => Buffer.alloc(32, 0xab));
        await assertRefused(upload('write-collision'), 'store_failed');
        assert.equal(fs.readFileSync(taken, 'utf8'), 'bytes that belong to another row');
        assert.equal(media.getById('write-collision'), null);
        fs.rmSync(taken);
        assert.deepEqual(allFiles(), before);

        const blocked = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-blocked-'));
        try {
            fs.writeFileSync(path.join(blocked, 'media'), 'a file where the folder should be');
            process.env.DATA_DIR = blocked;
            await assert.rejects(upload('write-blocked'), (err) => {
                assert.equal(err.code, 'store_failed');
                assert.equal(err.message.includes(blocked), false);
                return true;
            });
        } finally {
            process.env.DATA_DIR = tmp;
            fs.rmSync(blocked, { recursive: true, force: true });
        }
        assert.equal(media.getById('write-blocked'), null);
    });

    it('ac-3: an oversize byte length is refused before decode and before any write', async () => {
        const codec = recordingCodec();
        const before = allFiles();
        const oversize = Buffer.alloc(ingest.MAX_INPUT_BYTES + 1);
        fixture('tiny.png').copy(oversize);
        await assertRefused(
            ingest.ingestImage({ bytes: oversize, id: 'too-big', actor: 'admin:alice' }, { codec, now: T0 }),
            'too_large'
        );
        assert.equal(codec.calls.length, 0);
        assert.deepEqual(allFiles(), before);
        assert.equal(media.getById('too-big'), null);

        const base = makePng(1, 1).length;
        const exact = makePng(1, 1, [['prVt', Buffer.alloc(ingest.MAX_INPUT_BYTES - base - 12)]]);
        assert.equal(exact.length, 5242880);
        const saved = await ingest.ingestImage(
            { bytes: exact, id: 'exact-limit', actor: 'admin:alice' },
            { codec, now: T0 }
        );
        assert.equal(codec.calls.length, 1);
        assert.equal(codec.calls[0].length, 5242880);
        assert.match(saved.storedName, STORED_NAME);
    });

    it('ac-3: a width or height over 4096 is refused before decode and before any write', async () => {
        const codec = recordingCodec();
        const before = allFiles();
        for (const name of ['wide-4097.png', 'tall-4097.jpg', 'wide-4097.webp', 'wide-4097-alpha.webp']) {
            const id = 'dims-' + name.replace(/\W+/g, '-');
            await assertRefused(ingest.ingestImage(
                { bytes: fixture(name), id, actor: 'admin:alice' },
                { codec, now: T0 }
            ), 'dimensions_too_large');
            assert.equal(media.getById(id), null, id);
        }
        assert.equal(codec.calls.length, 0);
        assert.deepEqual(allFiles(), before);

        await ingest.ingestImage(
            { bytes: fixture('edge-4096.png'), id: 'dims-edge', actor: 'admin:alice' },
            { codec, now: T0 }
        );
        assert.equal(codec.calls.length, 1);
    });

    it('ac-3: the 41st upload for one actor in a clock hour is refused', async () => {
        const codec = recordingCodec();
        const now = T0 + 5 * MINUTE;
        for (let i = 1; i <= 40; i += 1) {
            await ingest.ingestImage(
                { bytes: fixture('tiny.png'), id: 'rate-' + i, actor: 'admin:bob' },
                { codec, now }
            );
        }
        const before = allFiles();
        await assert.rejects(ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'rate-41', actor: 'admin:bob' },
            { codec, now }
        ), (err) => {
            assert.equal(err.code, 'rate_limited');
            assert.equal(err.retryAfterMs, 55 * MINUTE);
            return true;
        });
        assert.equal(codec.calls.length, 40);
        assert.deepEqual(allFiles(), before);
        assert.equal(media.getById('rate-41'), null);

        await ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'rate-mcp', actor: 'mcp' },
            { codec, now }
        );
        await assertRefused(ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'rate-41', actor: 'admin:bob' },
            { codec, now: T0 + 59 * MINUTE + 59 * 1000 }
        ), 'rate_limited');
        const nextHour = await ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'rate-41', actor: 'admin:bob' },
            { codec, now: T0 + 60 * MINUTE }
        );
        assert.equal(nextHour.status, 'pending');
    });

    it('ac-3: refused attempts count toward the hourly limit', async () => {
        const codec = recordingCodec();
        for (let i = 1; i <= 40; i += 1) {
            await assertRefused(ingest.ingestImage(
                { bytes: fixture('svg-named.png'), id: 'svg-try-' + i, actor: 'admin:carol' },
                { codec, now: T0 }
            ), 'unsupported_type');
        }
        await assertRefused(ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'carol-real', actor: 'admin:carol' },
            { codec, now: T0 }
        ), 'rate_limited');
        assert.equal(codec.calls.length, 0);
    });

    it('ac-3: a clock value that is not a time falls back to the real clock and still counts', async (t) => {
        t.mock.timers.enable({ apis: ['Date'], now: T0 + 30 * MINUTE });
        const codec = recordingCodec();
        for (let i = 1; i <= 40; i += 1) {
            await assertRefused(ingest.ingestImage(
                { bytes: fixture('svg-named.png'), id: 'nan-try-' + i, actor: 'admin:dave' },
                { codec, now: Number.NaN }
            ), 'unsupported_type');
        }
        await assertRefused(ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'dave-real', actor: 'admin:dave' },
            { codec, now: 'not a date' }
        ), 'rate_limited');
        await assertRefused(ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'dave-real', actor: 'admin:dave' },
            { codec, now: T0 + 30 * MINUTE }
        ), 'rate_limited');
        assert.equal(codec.calls.length, 0);
    });

    it('ac-3: an earlier clock value does not reopen a full hour, and one actor cannot reset another', async () => {
        const codec = recordingCodec();
        const now = T0 + 10 * MINUTE;
        for (let i = 1; i <= 40; i += 1) {
            await assertRefused(ingest.ingestImage(
                { bytes: fixture('svg-named.png'), id: 'clock-try-' + i, actor: 'admin:erin' },
                { codec, now }
            ), 'unsupported_type');
        }
        await ingest.ingestImage(
            { bytes: fixture('tiny.png'), id: 'clock-frank', actor: 'admin:frank' },
            { codec, now: T0 + 180 * MINUTE }
        );
        for (const clock of [now, 0, T0 - 1440 * MINUTE, true, '0']) {
            await assertRefused(ingest.ingestImage(
                { bytes: fixture('tiny.png'), id: 'clock-erin', actor: 'admin:erin' },
                { codec, now: clock }
            ), 'rate_limited');
        }
        assert.equal(codec.calls.length, 1);
        assert.equal(media.getById('clock-erin'), null);
    });

    it('ac-3: a later clock value opens only that actor\'s next hour, and earlier values then count against it', async () => {
        const codec = recordingCodec();
        const svg = fixture('svg-named.png');
        const attempt = (id, now) => ingest.ingestImage({ bytes: svg, id, actor: 'admin:gina' }, { codec, now });
        for (let i = 1; i <= 40; i += 1) await assertRefused(attempt('later-try-' + i, T0), 'unsupported_type');
        await assertRefused(upload('later-full', { actor: 'admin:gina' }, { codec }), 'rate_limited');

        const next = await upload('later-next', { actor: 'admin:gina' }, { codec, now: T0 + 60 * MINUTE });
        assert.equal(next.status, 'pending');
        for (let i = 2; i <= 40; i += 1) {
            await assertRefused(attempt('later-back-' + i, T0 + 5 * MINUTE), 'unsupported_type');
        }
        await assert.rejects(attempt('later-back-41', T0 + 5 * MINUTE), (err) => {
            assert.equal(err.code, 'rate_limited');
            assert.equal(err.retryAfterMs, 115 * MINUTE);
            return true;
        });
        await assertRefused(
            upload('later-again', { actor: 'admin:gina' }, { codec, now: T0 + 119 * MINUTE }),
            'rate_limited'
        );
        const other = await upload('later-other', { actor: 'admin:hank' }, { codec });
        assert.equal(other.status, 'pending');
        assert.equal(codec.calls.length, 2);
    });

    it('refuses animated PNG, animated WebP, and multi-frame JPEG before decode', async () => {
        const codec = recordingCodec();
        const before = allFiles();
        for (const name of ['animated.png', 'animated.webp', 'multi-frame.jpg']) {
            await assertRefused(ingest.ingestImage(
                { bytes: fixture(name), id: 'anim-' + name.replace(/\W+/g, '-'), actor: 'mcp' },
                { codec, now: T0 }
            ), 'animated_refused');
        }

        const jpeg = fixture('tiny.jpg');
        const sof = jpegSof(jpeg);
        const sofSegment = jpeg.subarray(sof.at, sof.at + sof.size);
        const differential = Buffer.from(jpeg);
        differential[sof.at + 1] = 0xc5;
        const jpegCases = [
            ['anim-second-sof', Buffer.concat([
                jpeg.subarray(0, sof.at + sof.size), sofSegment, jpeg.subarray(sof.at + sof.size)
            ])],
            ['anim-hierarchical', Buffer.concat([
                jpeg.subarray(0, sof.at), Buffer.from([0xff, 0xde]), sofSegment.subarray(2), jpeg.subarray(sof.at)
            ])],
            ['anim-differential', differential],
            ['anim-second-jpeg', Buffer.concat([jpeg, jpeg])]
        ];
        for (const [id, bytes] of jpegCases) {
            await assertRefused(
                ingest.ingestImage({ bytes, id, actor: 'mcp' }, { codec, now: T0 }),
                'animated_refused'
            );
            assert.equal(media.getById(id), null, id);
        }
        assert.equal(codec.calls.length, 0);
        assert.deepEqual(allFiles(), before);
    });

    it('ac-2: delete copies to _archive, checks the hash, then removes the live file', async () => {
        const saved = await upload('del-ok');
        const live = ingest.storedFilePath(saved.storedName);
        const liveHash = sha256(fs.readFileSync(live));
        const archived = path.join(tmp, 'media', '_archive', saved.storedName);
        let liveDuringCopy = null;
        const result = ingest.deleteMedia({ id: 'del-ok', actorKind: 'admin' }, {
            copyFile(src, dest, mode) {
                liveDuringCopy = fs.existsSync(src);
                fs.copyFileSync(src, dest, mode);
            }
        });
        assert.equal(liveDuringCopy, true);
        assert.deepEqual(result, { id: 'del-ok', storedName: saved.storedName, sha256: liveHash });
        assert.equal(fs.existsSync(live), false);
        assert.equal(sha256(fs.readFileSync(archived)), liveHash);
        assert.equal(path.dirname(archived), ingest.archiveDir());
        assert.equal(media.getById('del-ok').status, 'pending');

        assertThrowsCode(() => ingest.deleteMedia({ id: 'del-ok', actorKind: 'admin' }), 'media_missing');
        assert.equal(sha256(fs.readFileSync(archived)), liveHash);
    });

    it('ac-2: if the archive copy hash differs, the live file remains', async () => {
        const saved = await upload('del-bad');
        const live = ingest.storedFilePath(saved.storedName);
        const original = fs.readFileSync(live);
        const archived = path.join(ingest.archiveDir(), saved.storedName);

        assertThrowsCode(() => ingest.deleteMedia({ id: 'del-bad', actorKind: 'admin' }, {
            copyFile(src, dest) {
                fs.writeFileSync(dest, Buffer.concat([fs.readFileSync(src), Buffer.from([0])]));
            }
        }), 'archive_verify_failed');
        assert.deepEqual(fs.readFileSync(live), original);
        assert.equal(fs.existsSync(archived), false);
        assert.ok(media.getById('del-bad'));

        ingest.deleteMedia({ id: 'del-bad', actorKind: 'admin' });
        assert.equal(fs.existsSync(live), false);
        assert.deepEqual(fs.readFileSync(archived), original);
    });

    it('ac-2: delete keeps the live file when a different archive copy exists or the copy fails', async () => {
        const conflict = await upload('del-conflict');
        const conflictLive = ingest.storedFilePath(conflict.storedName);
        const conflictArchive = path.join(ingest.archiveDir(), conflict.storedName);
        fs.mkdirSync(ingest.archiveDir(), { recursive: true });
        fs.writeFileSync(conflictArchive, 'earlier archive bytes');
        assertThrowsCode(
            () => ingest.deleteMedia({ id: 'del-conflict', actorKind: 'admin' }),
            'archive_conflict'
        );
        assert.equal(fs.existsSync(conflictLive), true);
        assert.equal(fs.readFileSync(conflictArchive, 'utf8'), 'earlier archive bytes');

        const failing = await upload('del-throw');
        const failingLive = ingest.storedFilePath(failing.storedName);
        assertThrowsCode(() => ingest.deleteMedia({ id: 'del-throw', actorKind: 'admin' }, {
            copyFile() {
                throw Object.assign(new Error('disk full'), { code: 'ENOSPC' });
            }
        }), 'archive_failed');
        assert.equal(fs.existsSync(failingLive), true);
        assert.equal(fs.existsSync(path.join(ingest.archiveDir(), failing.storedName)), false);
    });

    it('ac-2: an unlink failure after a verified archive copy is a coded error, and both files are kept', async (t) => {
        const saved = await upload('del-busy');
        const live = ingest.storedFilePath(saved.storedName);
        const archived = path.join(ingest.archiveDir(), saved.storedName);
        const original = fs.readFileSync(live);
        const unlink = t.mock.method(fs, 'unlinkSync', () => {
            throw Object.assign(new Error('resource busy or locked'), { code: 'EBUSY' });
        });
        assertThrowsCode(() => ingest.deleteMedia({ id: 'del-busy', actorKind: 'admin' }), 'delete_failed');
        unlink.mock.restore();
        assert.deepEqual(fs.readFileSync(live), original);
        assert.deepEqual(fs.readFileSync(archived), original);

        const retried = ingest.deleteMedia({ id: 'del-busy', actorKind: 'admin' });
        assert.equal(retried.sha256, sha256(original));
        assert.equal(fs.existsSync(live), false);
        assert.deepEqual(fs.readFileSync(archived), original);
    });

    it('ac-2: only the admin actor kind can delete', async () => {
        const saved = await upload('del-actor');
        const live = ingest.storedFilePath(saved.storedName);
        for (const actorKind of ['mcp', 'user', undefined, 'admin:alice']) {
            assertThrowsCode(
                () => ingest.deleteMedia({ id: 'del-actor', actorKind }),
                'forbidden_actor'
            );
        }
        assert.equal(fs.existsSync(live), true);
        assert.equal(fs.existsSync(path.join(ingest.archiveDir(), saved.storedName)), false);
        assertThrowsCode(
            () => ingest.deleteMedia({ id: 'no-such-media', actorKind: 'admin' }),
            'not_found'
        );
    });

    it('ac-4: mcp cannot move a row to approved, and empty alt text cannot be approved', async () => {
        await upload('appr-empty', { altText: '' });
        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-empty', actorKind: 'mcp', altText: 'a cup of coffee' }),
            'forbidden_actor'
        );
        assert.equal(media.getById('appr-empty').status, 'pending');
        assert.equal(media.getById('appr-empty').altText, '');

        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-empty', actorKind: 'admin' }),
            'alt_text_required'
        );
        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-empty', actorKind: 'admin', altText: '   ' }),
            'alt_text_required'
        );
        const invisible = [
            '\u200B\u2060', '\u200F \u200E', '\u00A0\u3000\uFEFF',
            '\u3164', '\u115F', '\uFFA0', '\u034F', '\uFE0F', '\u{E0100}', '\u17B4', '\u180B',
            '\u2800', '\u0000', '\u0007', '\u0301', '\u200B\u3164\uFE0F', '\u2014 \u2026'
        ];
        for (const altText of invisible) {
            assertThrowsCode(
                () => ingest.approveMedia({ id: 'appr-empty', actorKind: 'admin', altText }),
                'alt_text_required'
            );
        }
        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-empty', actorKind: 'user', altText: 'a cup' }),
            'forbidden_actor'
        );
        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-empty', actorKind: 'admin', altText: 'x'.repeat(2001) }),
            'invalid_alt_text'
        );
        assert.equal(media.getById('appr-empty').status, 'pending');

        const approved = ingest.approveMedia({
            id: 'appr-empty',
            actorKind: 'admin',
            altText: '  a cup of coffee '
        });
        assert.equal(approved.status, 'approved');
        assert.equal(approved.altText, 'a cup of coffee');
        assert.equal(media.getById('appr-empty').status, 'approved');

        await upload('appr-drafted', { altText: 'a glass of water', actor: 'mcp' });
        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-drafted', actorKind: 'mcp' }),
            'forbidden_actor'
        );
        assert.equal(media.getById('appr-drafted').status, 'pending');
        assert.equal(
            ingest.approveMedia({ id: 'appr-drafted', actorKind: 'admin' }).status,
            'approved'
        );

        const pointedHebrew = '\u200F\u05DB\u05BC\u05D5\u05B9\u05E1 \u05E7\u05B8\u05E4\u05B6\u05D4';
        await upload('appr-hebrew', { altText: '' });
        const hebrew = ingest.approveMedia({ id: 'appr-hebrew', actorKind: 'admin', altText: pointedHebrew });
        assert.equal(hebrew.status, 'approved');
        assert.equal(hebrew.altText, pointedHebrew);

        assertThrowsCode(
            () => ingest.approveMedia({ id: 'no-such-media', actorKind: 'admin', altText: 'x' }),
            'not_found'
        );
        await upload('appr-deleted', { altText: 'bread' });
        ingest.deleteMedia({ id: 'appr-deleted', actorKind: 'admin' });
        assertThrowsCode(
            () => ingest.approveMedia({ id: 'appr-deleted', actorKind: 'admin' }),
            'media_missing'
        );
        assert.equal(media.getById('appr-deleted').status, 'pending');
    });

    it('stored paths stay inside DATA_DIR/media, and a media folder that overlaps a served root is refused', async () => {
        const hex = 'a'.repeat(64);
        for (const bad of [
            '../hebrew.sqlite',
            '..\\' + hex + '.png',
            '../' + hex + '.png',
            'A'.repeat(64) + '.png',
            'a'.repeat(63) + '.png',
            hex + '.PNG',
            hex + '.png/../../hebrew.sqlite',
            hex + '.svg',
            ''
        ]) {
            assertThrowsCode(() => ingest.storedFilePath(bad), 'unsafe_path');
        }
        assert.equal(path.dirname(ingest.storedFilePath(hex + '.png')), path.resolve(tmp, 'media'));

        const res = new FakeResponse();
        assert.equal(ingest.sendStoredImage(res, '../hebrew.sqlite'), false);
        assert.equal(res.statusCode, null);
        assert.deepEqual(res.headers, {});

        const before = allFiles();
        const servedCases = [
            ['served-media', path.join(tmp, 'media')],
            ['served-archive', path.join(tmp, 'media', '_archive')]
        ];
        for (const [id, served] of servedCases) {
            process.env.LESSONS_DIR = served;
            try {
                assertThrowsCode(() => ingest.mediaDir(), 'unsafe_path');
                await assertRefused(upload(id), 'unsafe_path');
            } finally {
                delete process.env.LESSONS_DIR;
            }
            assert.equal(media.getById(id), null, id);
        }
        assert.deepEqual(allFiles(), before);
    });

    it('a DATA_DIR in login/ or shared/, directly or through a link, is refused before a folder is created there', async (t) => {
        const fakeRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-root-'));
        const aliasHome = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-alias-'));
        const served = ['shared', 'login', 'review', 'worksheets'];
        for (const name of served) fs.mkdirSync(path.join(fakeRoot, name, 'inner'), { recursive: true });
        const aliases = [];
        try {
            process.env.HEBREW_ROOT = fakeRoot;
            process.env.DATA_DIR = path.join(fakeRoot, 'login', 'inner');
            await assertRefused(upload('root-login'), 'unsafe_path');
            process.env.DATA_DIR = path.join(fakeRoot, 'shared', 'inner');
            await assert.rejects(upload('root-shared'), /DATA_DIR must not equal or lie inside a served root/);

            for (const name of ['login', 'shared']) {
                const alias = path.join(aliasHome, name);
                try {
                    fs.symlinkSync(path.join(fakeRoot, name, 'inner'), alias, 'junction');
                } catch (err) {
                    if (err.code === 'EPERM') return t.skip('symlinks are not permitted here');
                    throw err;
                }
                aliases.push(alias);
                process.env.DATA_DIR = alias;
                await assertRefused(upload('alias-' + name), 'unsafe_path');
            }
            for (const name of served) {
                assert.deepEqual(fs.readdirSync(path.join(fakeRoot, name, 'inner')), [], name);
            }
            for (const id of ['root-login', 'root-shared', 'alias-login', 'alias-shared']) {
                assert.equal(media.getById(id), null, id);
            }
        } finally {
            process.env.HEBREW_ROOT = root;
            process.env.DATA_DIR = tmp;
            for (const alias of aliases) fs.rmSync(alias, { force: true });
            fs.rmSync(aliasHome, { recursive: true, force: true });
            fs.rmSync(fakeRoot, { recursive: true, force: true });
        }
    });

    it('refuses to read, write, approve, or delete when media/ or media/_archive/ resolves outside its parent', async (t) => {
        const second = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-link-'));
        const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-media-outside-'));
        const mediaPath = path.join(second, 'media');
        const parkedMedia = path.join(second, 'media-parked');
        const archiveLink = path.join(mediaPath, '_archive');
        try {
            process.env.DATA_DIR = second;
            const saved = await upload('link-row', { altText: 'a cup' });
            const live = ingest.storedFilePath(saved.storedName);
            assert.equal(path.dirname(live), path.resolve(second, 'media'));

            fs.renameSync(mediaPath, parkedMedia);
            try {
                fs.symlinkSync(outside, mediaPath, 'junction');
            } catch (err) {
                fs.renameSync(parkedMedia, mediaPath);
                if (err.code === 'EPERM') return t.skip('symlinks are not permitted here');
                throw err;
            }
            fs.copyFileSync(path.join(parkedMedia, saved.storedName), path.join(outside, saved.storedName));

            await assertRefused(upload('link-media'), 'unsafe_path');
            assert.equal(media.getById('link-media'), null);
            assertThrowsCode(() => ingest.storedFilePath(saved.storedName), 'unsafe_path');
            const res = new FakeResponse();
            assertThrowsCode(() => ingest.sendStoredImage(res, saved.storedName), 'unsafe_path');
            assert.equal(res.statusCode, null);
            assertThrowsCode(
                () => ingest.approveMedia({ id: 'link-row', actorKind: 'admin' }),
                'unsafe_path'
            );
            assertThrowsCode(
                () => ingest.deleteMedia({ id: 'link-row', actorKind: 'admin' }),
                'unsafe_path'
            );
            assert.deepEqual(fs.readdirSync(outside), [saved.storedName]);
            assert.equal(media.getById('link-row').status, 'pending');

            fs.rmSync(mediaPath, { force: true });
            fs.rmSync(path.join(outside, saved.storedName));
            fs.renameSync(parkedMedia, mediaPath);

            fs.symlinkSync(outside, archiveLink, 'junction');
            assertThrowsCode(
                () => ingest.deleteMedia({ id: 'link-row', actorKind: 'admin' }),
                'unsafe_path'
            );
            assert.equal(fs.existsSync(live), true);
            assert.deepEqual(fs.readdirSync(outside), []);
        } finally {
            process.env.DATA_DIR = tmp;
            for (const link of [archiveLink, mediaPath]) {
                try {
                    if (fs.lstatSync(link).isSymbolicLink()) fs.rmSync(link, { force: true });
                } catch (_) { /* not created */ }
            }
            fs.rmSync(second, { recursive: true, force: true });
            fs.rmSync(outside, { recursive: true, force: true });
        }
    });

    it('response helpers send image/png with nosniff, never the declared type', async () => {
        const headersOnly = new FakeResponse();
        headersOnly.setHeader('Content-Type', 'image/svg+xml');
        headersOnly.setHeader('X-Content-Type-Options', 'off');
        ingest.setMediaResponseHeaders(headersOnly);
        assert.equal(headersOnly.getHeader('Content-Type'), 'image/png');
        assert.equal(headersOnly.getHeader('X-Content-Type-Options'), 'nosniff');

        const saved = await upload('send-one', { contentType: 'image/svg+xml' });
        const res = new FakeResponse();
        const body = res.body();
        assert.equal(ingest.sendStoredImage(res, saved.storedName), true);
        assert.deepEqual(await body, fs.readFileSync(ingest.storedFilePath(saved.storedName)));
        assert.equal(res.statusCode, 200);
        assert.equal(res.getHeader('Content-Type'), 'image/png');
        assert.equal(res.getHeader('X-Content-Type-Options'), 'nosniff');
        assert.equal(res.getHeader('Content-Length'), String(CODEC_OUTPUT.length));

        const missing = new FakeResponse();
        assert.equal(ingest.sendStoredImage(missing, 'f'.repeat(64) + '.png'), false);
        assert.equal(missing.statusCode, null);
        assert.equal(missing.getHeader('Content-Type'), undefined);
    });

    it('does not log, and refusals carry neither the prompt nor file bytes', async () => {
        const prompt = 'PROMPT-MARKER a red apple on a table';
        const logged = [];
        const methods = ['log', 'info', 'warn', 'error', 'debug'];
        const saved = methods.map((name) => console[name]);
        methods.forEach((name) => {
            console[name] = (...args) => logged.push(args);
        });
        try {
            await assert.rejects(upload('quiet-fail', { promptText: prompt }, {
                codec: { async decodeAndReencode() { throw new Error(prompt); } }
            }), (err) => {
                assert.equal(err.code, 'reencode_failed');
                const text = String(err.message) + String(err.stack) + JSON.stringify(err);
                assert.equal(text.includes('PROMPT-MARKER'), false);
                assert.equal(text.includes('IHDR'), false);
                return true;
            });
            const generated = await upload('quiet-ok', {
                promptText: prompt,
                provider: 'openai',
                modelId: 'gpt-image-1',
                actor: 'mcp'
            });
            const row = media.getById(generated.id);
            assert.equal(row.promptText, prompt);
            assert.equal(row.provider, 'openai');
            assert.equal(row.modelId, 'gpt-image-1');
            assert.equal(row.status, 'pending');

            const flipped = Buffer.from(fixture('tiny.png'));
            flipped[flipped.length - 20] ^= 0xff;
            await assert.rejects(ingest.ingestImage(
                { bytes: flipped, id: 'quiet-real-fail', actor: 'mcp', promptText: prompt },
                { now: T0 }
            ), (err) => {
                assert.equal(err.code, 'malformed_image');
                const text = String(err.message) + String(err.stack) + JSON.stringify(err);
                assert.equal(/PROMPT-MARKER|IHDR|vips|png read/i.test(text), false);
                return true;
            });
            const real = await ingest.ingestImage(
                { bytes: fixture('exif-gps.jpg'), id: 'quiet-real-ok', actor: 'mcp', promptText: prompt },
                { now: T0 }
            );
            assert.equal(media.getById(real.id).promptText, prompt);
        } finally {
            methods.forEach((name, i) => {
                console[name] = saved[i];
            });
        }
        assert.deepEqual(logged, []);
        for (const file of ['ingest.js', 'sniff.js', 'codec.js']) {
            const source = fs.readFileSync(path.join(root, 'server', 'media', file), 'utf8');
            assert.equal(/console\.|process\.std(out|err)/.test(source), false, file);
        }
    });
});
