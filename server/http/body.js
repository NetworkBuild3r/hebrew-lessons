/**
 * JSON-only request body reader with a size limit.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

class BodyError extends Error {
    /**
     * @param {string} message
     * @param {number} status
     * @param {string} code
     */
    constructor(message, status, code) {
        super(message);
        this.name = 'BodyError';
        this.status = status;
        this.code = code;
    }
}

const DEFAULT_LIMIT = 64 * 1024;

/**
 * Read and parse a JSON body. Empty body → null.
 * @param {import('http').IncomingMessage} req
 * @param {{ limit?: number }} [opts]
 * @returns {Promise<object|array|null>}
 */
function readJson(req, opts) {
    const limit = opts && opts.limit != null ? Number(opts.limit) : DEFAULT_LIMIT;
    return new Promise((resolve, reject) => {
        const chunks = [];
        let size = 0;
        let settled = false;

        function fail(err) {
            if (settled) return;
            settled = true;
            reject(err);
        }

        function ok(value) {
            if (settled) return;
            settled = true;
            resolve(value);
        }

        req.on('data', (chunk) => {
            size += chunk.length;
            if (size > limit) {
                req.destroy();
                fail(new BodyError('Request body too large', 413, 'body_too_large'));
                return;
            }
            chunks.push(chunk);
        });
        req.on('end', () => {
            if (settled) return;
            const raw = Buffer.concat(chunks).toString('utf8').trim();
            if (!raw) {
                ok(null);
                return;
            }
            try {
                ok(JSON.parse(raw));
            } catch (_) {
                fail(new BodyError('Request body must be JSON', 400, 'invalid_json'));
            }
        });
        req.on('error', (err) => {
            fail(err);
        });
    });
}

module.exports = {
    readJson,
    BodyError,
    DEFAULT_LIMIT
};
