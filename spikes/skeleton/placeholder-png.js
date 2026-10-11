'use strict';

/**
 * Tiny RGBA PNG, drawn in process. No network, no image library.
 * Provenance: INIT-003/SPEC-009
 */

const zlib = require('zlib');

function crc32(buf) {
    let c = ~0;
    for (let i = 0; i < buf.length; i++) {
        c ^= buf[i];
        for (let k = 0; k < 8; k++) {
            c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
        }
    }
    return ~c >>> 0;
}

function chunk(type, data) {
    const length = Buffer.alloc(4);
    length.writeUInt32BE(data.length, 0);
    const body = Buffer.concat([Buffer.from(type), data]);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(crc32(body), 0);
    return Buffer.concat([length, body, crc]);
}

function pixel(x, y) {
    const bg = [0xf7, 0xf6, 0xf3, 255];
    const ink = [0x1c, 0x19, 0x17, 255];
    const on = (nx, ny, w, h) => x >= nx && x < nx + w && y >= ny && y < ny + h;
    // Cup body, handle, and a steam mark. 64×64.
    const cup = on(18, 28, 22, 16) || on(20, 24, 18, 6);
    const handle = on(40, 32, 8, 8) && !on(42, 34, 4, 4);
    const steam = on(26, 14, 3, 8) || on(32, 12, 3, 8);
    if (cup || handle || steam) return ink;
    return bg;
}

function placeholderPng() {
    const width = 64;
    const height = 64;
    const raw = Buffer.alloc((width * 4 + 1) * height);
    for (let y = 0; y < height; y++) {
        const row = y * (width * 4 + 1);
        raw[row] = 0;
        for (let x = 0; x < width; x++) {
            const px = pixel(x, y);
            const i = row + 1 + x * 4;
            raw[i] = px[0];
            raw[i + 1] = px[1];
            raw[i + 2] = px[2];
            raw[i + 3] = px[3];
        }
    }
    const ihdr = Buffer.alloc(13);
    ihdr.writeUInt32BE(width, 0);
    ihdr.writeUInt32BE(height, 4);
    ihdr[8] = 8;
    ihdr[9] = 6;
    const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
    return Buffer.concat([
        signature,
        chunk('IHDR', ihdr),
        chunk('IDAT', zlib.deflateSync(raw)),
        chunk('IEND', Buffer.alloc(0))
    ]);
}

module.exports = { placeholderPng };
