const test = require('node:test');
const assert = require('node:assert');
const http = require('http');

const TOKEN = 'test-token-' + 'x'.repeat(40);

function request(port, headers) {
    return new Promise((resolve, reject) => {
        const body = JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list' });
        const req = http.request({
            host: '127.0.0.1', port, path: '/mcp', method: 'POST',
            headers: Object.assign({
                'Content-Type': 'application/json',
                Accept: 'application/json, text/event-stream',
                'Content-Length': Buffer.byteLength(body)
            }, headers)
        }, res => {
            let data = '';
            res.on('data', c => { data += c; });
            res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, data }));
        });
        req.on('error', reject);
        req.end(body);
    });
}

test('MCP requires the bearer token', async () => {
    process.env.MCP_AUTH_TOKEN = TOKEN;
    process.env.MCP_PORT = '0';
    process.env.HOST = '127.0.0.1';
    const server = require('../mcp/http').listen();
    await new Promise(r => server.once('listening', r));
    const port = server.address().port;
    try {
        const none = await request(port, {});
        assert.strictEqual(none.status, 401);
        assert.match(none.headers['www-authenticate'], /^Bearer/);
        assert.strictEqual((await request(port, { Authorization: 'Bearer wrong' })).status, 401);
        assert.strictEqual((await request(port, { Authorization: 'Basic ' + TOKEN })).status, 401);
        assert.strictEqual((await request(port, { Authorization: 'Bearer ' + TOKEN, Host: 'evil.example' })).status, 403);
        const ok = await request(port, { Authorization: 'Bearer ' + TOKEN });
        assert.strictEqual(ok.status, 200);
        assert.match(ok.data, /list_lessons/);
    } finally {
        server.close();
    }
});

test('refuses to listen beyond loopback without a token', () => {
    delete process.env.MCP_AUTH_TOKEN;
    delete process.env.MCP_ALLOW_NO_AUTH;
    process.env.HOST = '0.0.0.0';
    assert.throws(() => require('../mcp/http').listen(), /MCP_AUTH_TOKEN is required/);
});
