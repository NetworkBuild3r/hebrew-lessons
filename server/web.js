/**
 * Static file server with session gating, filtered scripts, and /api/ mount.
 * Provenance: INIT-002/SPEC-008
 */
'use strict';

const fs = require('fs');
const http = require('http');
const path = require('path');
const { rootDir, lessonsDir } = require('./paths');
const sessions = require('./auth/sessions');
const cookies = require('./http/cookies');
const security = require('./http/security');
const { createRouter } = require('./api/router');
const authRoutes = require('./api/auth-routes');
const progressRoutes = require('./api/progress-routes');
const adminRoutes = require('./api/admin-routes');
const { canSeeLesson } = require('./visibility');
const contentFilter = require('./content-filter');

const TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.js': 'text/javascript; charset=utf-8',
    '.json': 'application/json; charset=utf-8',
    '.webmanifest': 'application/manifest+json; charset=utf-8',
    '.svg': 'image/svg+xml',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.webp': 'image/webp',
    '.ico': 'image/x-icon',
    '.mp3': 'audio/mpeg',
    '.m4a': 'audio/mp4',
    '.ogg': 'audio/ogg',
    '.wav': 'audio/wav',
    '.webm': 'audio/webm',
    '.woff': 'font/woff',
    '.woff2': 'font/woff2'
};

const MUTATING = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

const router = createRouter();
authRoutes.register(router);
progressRoutes.register(router);
adminRoutes.register(router);

/**
 * Decode and normalize the URL path. Returns null when the path is hostile.
 * Gating decisions use this value — never the raw URL string.
 * @param {string} urlPath
 * @returns {string|null}
 */
function decodePath(urlPath) {
    let rel;
    try {
        rel = decodeURIComponent(String(urlPath || '/').split('?')[0]);
    } catch (_) {
        return null;
    }
    if (rel.includes('\0')) return null;
    // Backslash is never a path separator we accept.
    if (rel.includes('\\')) return null;
    // Collapse repeated slashes.
    rel = rel.replace(/\/{2,}/g, '/');
    if (rel === '') rel = '/';
    // Reject empty / . / .. segments and Windows trailing-dot tricks.
    const parts = rel.split('/').slice(1);
    for (const part of parts) {
        if (part === '') continue; // trailing slash leaves an empty final part
        if (part === '.' || part === '..') return null;
        if (part.startsWith('.')) return null;
        if (part.endsWith('.')) return null;
    }
    return rel;
}

/**
 * Whether this relative path is under the public login tree.
 * @param {string} rel
 * @returns {boolean}
 */
function isPublicLoginPath(rel) {
    return rel === '/login' || rel === '/login/' || rel.startsWith('/login/');
}

/**
 * Map a decoded relative URL path to an absolute filesystem path, or null.
 * @param {string} rel
 * @returns {string|null}
 */
function resolveFile(rel) {
    let target = rel;
    if (target === '/' || target === '') target = '/index.html';
    // Directory index: /dir/ → /dir/index.html
    if (target.endsWith('/')) target = target + 'index.html';

    const ok = target === '/index.html'
        || target === '/manifest.webmanifest'
        || target === '/icon.svg'
        || target.startsWith('/shared/')
        || target.startsWith('/worksheets/')
        || target.startsWith('/review/')
        || target.startsWith('/lessons/')
        || target.startsWith('/progress/')
        || target.startsWith('/admin/')
        || target.startsWith('/login/');
    if (!ok) return null;

    const parts = target.split('/').slice(1);
    if (parts.some((part) => part === '' || part === '.' || part === '..' || part.startsWith('.'))) {
        return null;
    }
    // Trailing-dot segments (already rejected in decodePath; belt-and-suspenders).
    if (parts.some((part) => part.endsWith('.'))) return null;

    const root = rootDir();
    if (target.startsWith('/worksheets/')) {
        const under = target.slice('/worksheets/'.length);
        const full = path.resolve(lessonsDir(), under);
        const base = path.resolve(lessonsDir()) + path.sep;
        if (full !== path.resolve(lessonsDir()) && !full.startsWith(base)) return null;
        return full;
    }

    const full = path.resolve(root, '.' + target);
    const allowRoots = [
        path.resolve(root, 'index.html'),
        path.resolve(root, 'manifest.webmanifest'),
        path.resolve(root, 'icon.svg'),
        path.resolve(root, 'shared') + path.sep,
        path.resolve(root, 'review') + path.sep,
        path.resolve(root, 'lessons') + path.sep,
        path.resolve(root, 'progress') + path.sep,
        path.resolve(root, 'admin') + path.sep,
        path.resolve(root, 'login') + path.sep
    ];
    const allowed = allowRoots.some((entry) => {
        if (entry.endsWith(path.sep)) return full.startsWith(entry);
        return full === entry;
    });
    if (!allowed) return null;
    return full;
}

/** @deprecated use resolveFile(decodePath(...)); kept for callers/tests */
function publicFile(urlPath) {
    const rel = decodePath(urlPath);
    if (!rel) return null;
    return resolveFile(rel);
}

/**
 * Extract lesson slug from /worksheets/<slug>/… when slug is a real lesson folder.
 * @param {string} rel
 * @returns {string|null}
 */
function lessonSlugFromPath(rel) {
    const m = /^\/worksheets\/([^/]+)(?:\/|$)/.exec(rel);
    if (!m) return null;
    const slug = m[1];
    if (!slug || slug.startsWith('_')) return null;
    // Root worksheets files (catalog.js, etc.) are not lesson folders.
    if (slug.includes('.')) return null;
    return slug;
}

/**
 * Filtered per-user data scripts (Cache-Control: no-store).
 * @param {string} rel
 * @returns {null | ((user: object) => string)}
 */
function filteredScriptFor(rel) {
    if (rel === '/worksheets/catalog.js') {
        return (user) => contentFilter.catalogScriptFor(user);
    }
    if (rel === '/worksheets/_review/vocab-index.js') {
        return (user) => contentFilter.vocabIndexScriptFor(user);
    }
    if (rel === '/worksheets/_audio/words/catalog.js') {
        return (user) => contentFilter.wordAudioScriptFor(user, 'words');
    }
    if (rel === '/shared/word-audio-catalog.js') {
        return (user) => contentFilter.wordAudioScriptFor(user, 'shared');
    }
    return null;
}

/**
 * HTML navigation vs other assets.
 * @param {import('http').IncomingMessage} req
 * @param {string} rel
 * @returns {boolean}
 */
function wantsHtml(req, rel) {
    const accept = String(req.headers.accept || '');
    if (accept.includes('text/html')) return true;
    if (rel.endsWith('.html')) return true;
    if (rel.endsWith('/')) return true;
    if (rel === '/' || rel === '') return true;
    return false;
}

/**
 * @param {import('http').IncomingMessage} req
 * @returns {{ user: object, session: object, token: string } | null}
 */
function resolveRequestSession(req) {
    const token = cookies.readSessionToken(req.headers.cookie);
    if (!token) return null;
    const resolved = sessions.resolve(token);
    if (!resolved) return null;
    return { user: resolved.user, session: resolved.session, token };
}

function sendPlain(res, status, text, extraHeaders) {
    security.applySecurityHeaders(res);
    const body = String(text);
    const headers = Object.assign({
        'Content-Type': 'text/plain; charset=utf-8',
        'Content-Length': Buffer.byteLength(body)
    }, extraHeaders || {});
    res.writeHead(status, headers);
    res.end(body);
}

function sendRedirect(res, location) {
    security.applySecurityHeaders(res);
    res.writeHead(302, {
        Location: location,
        'Content-Type': 'text/plain; charset=utf-8',
        'Content-Length': 0
    });
    res.end();
}

function sendFilteredScript(res, method, body) {
    security.applySecurityHeaders(res, { noStore: true });
    const buf = Buffer.from(body, 'utf8');
    res.writeHead(200, {
        'Content-Type': TYPES['.js'],
        'Cache-Control': 'no-store',
        'Content-Length': buf.length
    });
    if (method === 'HEAD') {
        res.end();
        return;
    }
    res.end(buf);
}

function sendFile(res, method, file) {
    security.applySecurityHeaders(res);
    const type = TYPES[path.extname(file).toLowerCase()] || 'application/octet-stream';
    const stat = fs.statSync(file);
    res.writeHead(200, {
        'Content-Type': type,
        'Cache-Control': 'no-cache',
        'Content-Length': stat.size
    });
    if (method === 'HEAD') {
        res.end();
        return;
    }
    fs.createReadStream(file).pipe(res);
}

/**
 * @param {import('http').IncomingMessage} req
 * @param {import('http').ServerResponse} res
 * @param {string} pathname
 */
async function handleApi(req, res, pathname) {
    const method = String(req.method || 'GET').toUpperCase();
    if (MUTATING.has(method)) {
        if (!security.isJsonContentType(req.headers['content-type'])) {
            security.sendJsonError(res, 403, 'Forbidden', 'csrf');
            return;
        }
        if (!security.originAllowed(req)) {
            security.sendJsonError(res, 403, 'Forbidden', 'csrf');
            return;
        }
    }
    await router.handle(req, res, pathname);
}

/**
 * @param {import('http').IncomingMessage} req
 * @param {import('http').ServerResponse} res
 */
async function handleStatic(req, res) {
    const method = String(req.method || 'GET').toUpperCase();
    const rawUrl = req.url || '/';
    const rel = decodePath(rawUrl);

    // Hostile / undecodable paths: still gated when signed out.
    if (!rel) {
        const auth = resolveRequestSession(req);
        if (!auth) {
            // No usable path — only redirect when the client explicitly wants HTML.
            const accept = String(req.headers.accept || '');
            if (accept.includes('text/html')) {
                sendRedirect(res, '/login/?next=' + encodeURIComponent('/'));
                return;
            }
            sendPlain(res, 401, 'Unauthorized');
            return;
        }
        sendPlain(res, 404, 'Not found');
        return;
    }

    // Public login tree — no session required.
    if (isPublicLoginPath(rel)) {
        const loginRel = rel === '/login' ? '/login/' : rel;
        const filePath = resolveFile(loginRel);
        if (!filePath || !fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
            sendPlain(res, 404, 'Not found');
            return;
        }
        sendFile(res, method, filePath);
        return;
    }

    const auth = resolveRequestSession(req);

    if (!auth) {
        if (wantsHtml(req, rel)) {
            const next = security.safeNextPath(rel);
            sendRedirect(res, '/login/?next=' + encodeURIComponent(next));
            return;
        }
        sendPlain(res, 401, 'Unauthorized');
        return;
    }

    // Admin pages require admin role.
    const underAdmin = rel === '/admin' || rel.startsWith('/admin/');
    if (underAdmin && auth.user.role !== 'admin') {
        sendPlain(res, 403, 'Forbidden');
        return;
    }

    // Lesson visibility: invisible lesson → 404 (never 403).
    const slug = lessonSlugFromPath(rel);
    if (slug && !canSeeLesson(auth.user, slug)) {
        sendPlain(res, 404, 'Not found');
        return;
    }

    // Per-user filtered data scripts.
    const filterFn = filteredScriptFor(rel);
    if (filterFn) {
        sendFilteredScript(res, method, filterFn(auth.user));
        return;
    }

    const filePath = resolveFile(rel);
    if (!filePath || !fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
        sendPlain(res, 404, 'Not found');
        return;
    }
    sendFile(res, method, filePath);
}

function listen() {
    const port = Number(process.env.PORT || 8080);
    const host = process.env.HOST || '127.0.0.1';
    const httpServer = http.createServer((req, res) => {
        const method = String(req.method || 'GET').toUpperCase();
        let pathname = '/';
        try {
            pathname = new URL(req.url || '/', 'http://127.0.0.1').pathname;
        } catch (_) {
            pathname = '/';
        }

        const run = async () => {
            if (pathname === '/api' || pathname.startsWith('/api/')) {
                // Normalize /api → /api/ for routing table (auth paths are absolute).
                const apiPath = pathname === '/api' ? '/api/' : pathname;
                if (MUTATING.has(method) || method === 'GET' || method === 'HEAD') {
                    await handleApi(req, res, apiPath);
                    return;
                }
                security.sendJsonError(res, 405, 'Method not allowed', 'method_not_allowed');
                return;
            }

            if (MUTATING.has(method)) {
                security.applySecurityHeaders(res);
                sendPlain(res, 405, 'Method not allowed');
                return;
            }
            if (method !== 'GET' && method !== 'HEAD') {
                sendPlain(res, 405, 'Method not allowed');
                return;
            }
            await handleStatic(req, res);
        };

        run().catch((err) => {
            console.error('[web]', err && err.message ? err.message : err);
            if (!res.headersSent) {
                if (pathname.startsWith('/api')) {
                    security.sendJsonError(res, 500, 'Internal error', 'internal');
                } else {
                    sendPlain(res, 500, 'Internal error');
                }
            }
        });
    });
    httpServer.listen(port, host);
    return httpServer;
}

if (require.main === module) listen();

module.exports = {
    listen,
    publicFile,
    decodePath,
    resolveFile,
    safeNextPath: security.safeNextPath,
    router
};
