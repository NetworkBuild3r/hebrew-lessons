const http = require('http');
const { McpServer } = require('@modelcontextprotocol/sdk/server/mcp.js');
const { StreamableHTTPServerTransport } = require('@modelcontextprotocol/sdk/server/streamableHttp.js');
const { registerLessonTools } = require('./tools');

const MAX_BODY = 1024 * 1024;

function allowedHost(header) {
    const host = String(header || '').replace(/:\d+$/, '').replace(/^\[|\]$/g, '');
    const extra = String(process.env.MCP_ALLOWED_HOSTS || '')
        .split(',')
        .map(item => item.trim())
        .filter(Boolean);
    return host === 'localhost' || host === '127.0.0.1' || host === '::1' || extra.includes(host);
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
    const httpServer = http.createServer(async (req, res) => {
        const url = new URL(req.url || '/', 'http://127.0.0.1');
        if (url.pathname !== '/mcp') {
            res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end('Not found');
            return;
        }
        if (!allowedHost(req.headers.host)) {
            res.writeHead(403, { 'Content-Type': 'text/plain; charset=utf-8' });
            res.end('Forbidden host');
            return;
        }
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
            await transport.handleRequest(req, res, body);
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
