const crypto = require('crypto');
const http = require('http');
const { McpServer } = require('@modelcontextprotocol/sdk/server/mcp.js');
const { StreamableHTTPServerTransport } = require('@modelcontextprotocol/sdk/server/streamableHttp.js');
const { registerLessonTools } = require('./tools');
const oauth = require('./oauth');

const MAX_BODY = 1024 * 1024;

function allowedHost(header) {
    const host = String(header || '').replace(/:\d+$/, '').replace(/^\[|\]$/g, '');
    const extra = String(process.env.MCP_ALLOWED_HOSTS || '')
        .split(',')
        .map(item => item.trim())
        .filter(Boolean);
    return host === 'localhost' || host === '127.0.0.1' || host === '::1' || extra.includes(host);
}

// Bearer token from MCP_AUTH_TOKEN. Compared as SHA-256 digests in constant time.
// Static MCP_AUTH_TOKEN, or an OAuth access token. OAuth tokens carry the app user
// the Google sign-in resolved; the static token does not.
function principal(req) {
    const expected = process.env.MCP_AUTH_TOKEN;
    if (!expected) return null;
    const match = /^Bearer\s+(\S+)$/i.exec(String(req.headers.authorization || ''));
    if (!match) return null;
    const given = crypto.createHash('sha256').update(match[1]).digest();
    const wanted = crypto.createHash('sha256').update(expected).digest();
    if (crypto.timingSafeEqual(given, wanted)) return { user: null };
    const access = oauth.verifyAccessToken(match[1]);
    if (!access) return null;
    if (access.uid == null) return { user: null };
    return { user: { id: access.uid, username: access.un } };
}

function isLoopback(host) {
    return host === '127.0.0.1' || host === 'localhost' || host === '::1';
}

function readBody(req) {
    return new Promise((resolve, reject) => {
        const chunks = [];
        let size = 0;
        req.on('data', chunk => {
            size += chunk.length;
            if (size > MAX_BODY) {
                reject(Object.assign(new Error('Request body is too large.'), { status: 413 }));
                req.destroy();
                return;
            }
            chunks.push(chunk);
        });
        req.on('end', () => {
            const raw = Buffer.concat(chunks).toString('utf8');
            if (!raw) {
                resolve(undefined);
                return;
            }
            try {
                resolve(JSON.parse(raw));
            } catch (err) {
                reject(Object.assign(new Error('Request body is not JSON.'), { status: 400 }));
            }
        });
        req.on('error', reject);
    });
}

function createServer() {
    const server = new McpServer({ name: 'hebrew-lessons', version: '1.0.0' });
    registerLessonTools(server);
    return server;
}

function listen() {
    const port = Number(process.env.MCP_PORT || 8787);
    const host = process.env.HOST || '127.0.0.1';
    // upsert_lesson writes to disk: never listen beyond loopback without a token.
    if (!process.env.MCP_AUTH_TOKEN && !isLoopback(host)) {
        throw new Error('MCP_AUTH_TOKEN is required when HOST is not loopback.');
    }
    const httpServer = http.createServer(async (req, res) => {
        const url = new URL(req.url || '/', 'http://127.0.0.1');
        if (!allowedHost(req.headers.host)) {
            res.writeHead(403, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end('Forbidden host');
            return;
        }
        try {
            if (await oauth.handle(req, res)) return;
        } catch (err) {
            console.error(err);
            if (!res.headersSent) {
                res.writeHead(500, { 'Content-Type': 'text/plain; charset=utf-8' });
                res.end('OAuth request failed');
            }
            return;
        }
        if (url.pathname !== '/mcp' && url.pathname !== '/mcp/') {
            res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end('Not found');
            return;
        }
        const who = principal(req);
        if (!who) {
            console.error('mcp http 401 ' + req.method);
            res.writeHead(401, {
                'Content-Type': 'text/plain; charset=utf-8',
                'WWW-Authenticate': oauth.enabled()
                    ? 'Bearer realm="hebrew-lessons", resource_metadata="' + oauth.resourceMetadataUrl() + '"'
                    : 'Bearer realm="hebrew-lessons"'
            });
            res.end('Unauthorized');
            return;
        }
        console.error('mcp http ' + req.method + (who.user ? ' uid=' + who.user.id : ' static'));
        let body;
        try {
            if (req.method === 'POST') body = await readBody(req);
        } catch (err) {
            res.writeHead(err.status || 400, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end(err.message || 'Bad request');
            return;
        }
        const mcp = createServer();
        const transport = new StreamableHTTPServerTransport({ sessionIdGenerator: undefined });
        res.on('close', () => {
            transport.close().catch(() => {});
            mcp.close().catch(() => {});
        });
        try {
            await mcp.connect(transport);
            await oauth.runAs(who.user, () => transport.handleRequest(req, res, body));
        } catch (err) {
            if (!res.headersSent) {
                res.writeHead(500, { 'Content-Type': 'text/plain; charset=utf-8' });
                res.end('MCP request failed');
            }
            console.error(err);
        }
    });
    httpServer.listen(port, host);
    return httpServer;
}

if (require.main === module) listen();

module.exports = { listen };
