'use strict';

/**
 * Loopback static server for the walking skeleton only.
 * Listens on 127.0.0.1:8099. Not used by the production server.
 * Provenance: INIT-003/SPEC-009
 */

const http = require('http');
const fs = require('fs');
const path = require('path');
const { applyBody, loadPlan } = require('./ops');
const { placeholderPng } = require('./placeholder-png');

const HOST = '127.0.0.1';
const PORT = 8099;
const PUBLIC_DIR = path.join(__dirname, 'public');
const FONTS_DIR = path.resolve(__dirname, '..', '..', 'shared', 'fonts');
const BODY_LIMIT = 256 * 1024;
const FONT_FILE = /^rubik-[a-z0-9-]+\.woff2$/;

const TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.js': 'text/javascript; charset=utf-8',
    '.png': 'image/png',
    '.woff2': 'font/woff2',
    '.txt': 'text/plain; charset=utf-8'
};

let working = null;

function applyPosted(body, plan) {
    return applyBody(plan || loadPlan(), body);
}

function ensureMedia() {
    const file = path.join(PUBLIC_DIR, 'media', 'placeholder-coffee.png');
    fs.mkdirSync(path.dirname(file), { recursive: true });
    if (!fs.existsSync(file)) fs.writeFileSync(file, placeholderPng());
}

function inside(root, candidate) {
    const rel = path.relative(root, candidate);
    return rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel));
}

function send(res, status, body, type) {
    const payload = Buffer.isBuffer(body) ? body : Buffer.from(body);
    res.writeHead(status, {
        'Content-Type': type || 'text/plain; charset=utf-8',
        'Content-Length': payload.length,
        'Cache-Control': 'no-store',
        'X-Content-Type-Options': 'nosniff'
    });
    res.end(payload);
}

function sendJson(res, status, value) {
    send(res, status, JSON.stringify(value), 'application/json; charset=utf-8');
}

function readBody(req) {
    return new Promise((resolve, reject) => {
        const chunks = [];
        let size = 0;
        req.on('data', (chunk) => {
            size += chunk.length;
            if (size > BODY_LIMIT) {
                reject(Object.assign(new Error('too large'), { status: 413 }));
                req.destroy();
                return;
            }
            chunks.push(chunk);
        });
        req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
        req.on('error', reject);
    });
}

function fileFor(urlPath) {
    let decoded;
    try {
        decoded = decodeURIComponent(urlPath);
    } catch (err) {
        return null;
    }
    if (decoded.indexOf('\0') !== -1) return null;

    if (decoded === '/fonts' || decoded.startsWith('/fonts/')) {
        const name = path.basename(decoded);
        if (!FONT_FILE.test(name)) return null;
        const full = path.join(FONTS_DIR, name);
        return inside(FONTS_DIR, full) ? full : null;
    }

    const rel = decoded === '/' ? '/index.html' : decoded;
    const full = path.normalize(path.join(PUBLIC_DIR, rel));
    if (!inside(PUBLIC_DIR, full)) return null;
    return full;
}

function createServer() {
    working = loadPlan();
    ensureMedia();
    return http.createServer((req, res) => {
        const host = req.headers.host || '';
        if (host && host.indexOf(HOST) !== 0 && host.indexOf('localhost') !== 0) {
            send(res, 403, 'forbidden');
            return;
        }

        let url;
        try {
            url = new URL(req.url, 'http://' + HOST + ':' + PORT);
        } catch (err) {
            send(res, 400, 'bad request');
            return;
        }

        if (url.pathname === '/apply') {
            if (req.method !== 'POST') {
                send(res, 405, 'method not allowed');
                return;
            }
            readBody(req).then((raw) => {
                let body;
                try {
                    body = JSON.parse(raw);
                } catch (err) {
                    sendJson(res, 400, { error: 'invalid_json' });
                    return;
                }
                if (!body || typeof body !== 'object' || Array.isArray(body)) {
                    sendJson(res, 400, { error: 'invalid_json' });
                    return;
                }
                const result = applyBody(working, body);
                if (result.conflict) {
                    sendJson(res, result.conflict.error === 'revision_conflict' ? 409 : 400, result);
                    return;
                }
                working = result.plan;
                sendJson(res, 200, result);
            }).catch((err) => {
                send(res, err.status || 400, 'bad request');
            });
            return;
        }

        if (req.method !== 'GET' && req.method !== 'HEAD') {
            send(res, 405, 'method not allowed');
            return;
        }

        const file = fileFor(url.pathname);
        if (!file) {
            send(res, 400, 'bad request');
            return;
        }
        fs.stat(file, (err, stat) => {
            if (err || !stat.isFile()) {
                send(res, 404, 'not found');
                return;
            }
            const type = TYPES[path.extname(file).toLowerCase()] || 'application/octet-stream';
            if (req.method === 'HEAD') {
                res.writeHead(200, {
                    'Content-Type': type,
                    'Content-Length': stat.size,
                    'Cache-Control': 'no-store',
                    'X-Content-Type-Options': 'nosniff'
                });
                res.end();
                return;
            }
            fs.readFile(file, (readErr, data) => {
                if (readErr) {
                    send(res, 404, 'not found');
                    return;
                }
                send(res, 200, data, type);
            });
        });
    });
}

function start(port) {
    const server = createServer();
    server.listen(port || PORT, HOST);
    return server;
}

if (require.main === module) {
    const server = start(PORT);
    server.on('listening', () => {
        process.stdout.write('skeleton http://' + HOST + ':' + PORT + '/\n');
        process.stdout.write('stop with Ctrl+C\n');
    });
}

module.exports = {
    HOST,
    PORT,
    applyPosted,
    createServer,
    start
};
