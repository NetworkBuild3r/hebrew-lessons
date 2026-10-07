const fs = require('fs');
const http = require('http');
const path = require('path');

const TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.js': 'text/javascript; charset=utf-8',
    '.json': 'application/json; charset=utf-8',
    '.svg': 'image/svg+xml',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.webp': 'image/webp',
    '.mp3': 'audio/mpeg',
    '.m4a': 'audio/mp4',
    '.ogg': 'audio/ogg',
    '.wav': 'audio/wav',
    '.webm': 'audio/webm',
    '.woff': 'font/woff',
    '.woff2': 'font/woff2'
};

function rootDir() {
    return process.env.HEBREW_ROOT
        ? path.resolve(process.env.HEBREW_ROOT)
        : path.resolve(__dirname, '..');
}

function publicFile(urlPath) {
    let rel;
    try {
        rel = decodeURIComponent(urlPath.split('?')[0]);
    } catch (err) {
        return null;
    }
    if (rel.includes('\0')) return null;
    if (rel === '/' || rel === '') rel = '/index.html';
    const ok = rel === '/index.html' || rel.startsWith('/shared/') || rel.startsWith('/worksheets/');
    if (!ok) return null;
    const parts = rel.split('/').slice(1);
    if (parts.some(part => part === '' || part === '.' || part === '..' || part.startsWith('.'))) return null;

    const root = rootDir();
    const full = path.resolve(root, '.' + rel);
    const index = path.resolve(root, 'index.html');
    const shared = path.resolve(root, 'shared') + path.sep;
    const worksheets = path.resolve(root, 'worksheets') + path.sep;
    if (full !== index && !full.startsWith(shared) && !full.startsWith(worksheets)) return null;
    return full;
}

function listen() {
    const port = Number(process.env.PORT || 8080);
    const host = process.env.HOST || '127.0.0.1';
    const httpServer = http.createServer((req, res) => {
        if (req.method !== 'GET' && req.method !== 'HEAD') {
            res.writeHead(405, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end('Method not allowed');
            return;
        }
        const file = publicFile(req.url || '/');
        if (!file || !fs.existsSync(file) || !fs.statSync(file).isFile()) {
            res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end('Not found');
            return;
        }
        const type = TYPES[path.extname(file).toLowerCase()] || 'application/octet-stream';
        res.writeHead(200, {
            'Content-Type': type,
            'Cache-Control': 'no-cache'
        });
        if (req.method === 'HEAD') {
            res.end();
            return;
        }
        fs.createReadStream(file).pipe(res);
    });
    httpServer.listen(port, host);
    return httpServer;
}

if (require.main === module) listen();

module.exports = { listen };
