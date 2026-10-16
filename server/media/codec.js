/**
 * Default image codec for ingest (ADR-0004 §5): sharp decodes PNG, JPEG, or WebP and writes one
 * PNG frame. Ingest calls it only after the byte length, the sniffed format, and the header
 * dimensions pass. Before any pixel is decoded, this module checks the decoder's own format, frame
 * count, and dimensions again, because sharp also decodes SVG, GIF, TIFF, and other formats.
 * EXIF orientation is applied, the image is resized into 8-bit raw pixels, and the PNG is encoded
 * from those pixels alone. Nothing from the input's metadata reaches it: not EXIF, GPS, XMP, ICC,
 * or text, and not the input's resolution, which sharp would otherwise carry into pHYs.
 * sharp is loaded on first use: a server whose image lacks the native binary still starts, and
 * uploads are refused with `decoder_unavailable`.
 * Provenance: INIT-004/SPEC-008
 */
'use strict';

const FORMATS = new Set(['png', 'jpeg', 'webp']);
const DECODE_TIMEOUT_SECONDS = 20;

function codecError(code, message) {
    const err = new Error(message);
    err.code = code;
    return err;
}

function isPositiveInteger(n) {
    return Number.isInteger(n) && n > 0;
}

/**
 * @param {() => Function} load returns the sharp module, or throws when it cannot be loaded
 * @returns {import('./ingest').ImageCodec}
 */
function createSharpCodec(load) {
    let sharp = null;

    function decoder() {
        if (!sharp) {
            try {
                sharp = load();
            } catch (_) {
                throw codecError('decoder_unavailable', 'The image decoder could not be loaded.');
            }
        }
        return sharp;
    }

    return {
        async decodeAndReencode(bytes, limits) {
            const { format, maxInputSide, maxStoredSide } = limits || {};
            if (!Buffer.isBuffer(bytes) || !FORMATS.has(format)
                || !isPositiveInteger(maxInputSide) || !isPositiveInteger(maxStoredSide)) {
                throw new Error('Codec input or limits are not valid.');
            }
            const lib = decoder();
            let image;
            let meta;
            try {
                image = lib(bytes, {
                    failOn: 'warning',
                    limitInputPixels: maxInputSide * maxInputSide
                });
                meta = await image.metadata();
            } catch (_) {
                throw codecError('malformed_image', 'The decoder could not read the image.');
            }
            if (meta.format !== format) {
                throw codecError('malformed_image', 'The decoder read a different format than the bytes declare.');
            }
            const frames = meta.pages === undefined ? 1 : meta.pages;
            if (frames !== 1) {
                throw codecError('animated_refused', 'The image has more than one frame.');
            }
            const { width, height } = meta;
            if (!isPositiveInteger(width) || !isPositiveInteger(height)) {
                throw codecError('malformed_image', 'The decoder could not read the image size.');
            }
            if (width > maxInputSide || height > maxInputSide) {
                throw codecError('dimensions_too_large', 'The decoded image is too wide or too tall.');
            }
            let pixels;
            try {
                pixels = await image
                    .autoOrient()
                    .resize({
                        width: maxStoredSide,
                        height: maxStoredSide,
                        fit: 'inside',
                        withoutEnlargement: true
                    })
                    .raw({ depth: 'uchar' })
                    .timeout({ seconds: DECODE_TIMEOUT_SECONDS })
                    .toBuffer({ resolveWithObject: true });
            } catch (_) {
                throw codecError('malformed_image', 'The decoder could not read the image.');
            }
            const { data, info } = pixels;
            const png = await lib(data, {
                raw: { width: info.width, height: info.height, channels: info.channels }
            }).png().toBuffer();
            return { width, height, frames, png };
        }
    };
}

const sharpCodec = createSharpCodec(() => require('sharp'));

module.exports = {
    createSharpCodec,
    sharpCodec
};
