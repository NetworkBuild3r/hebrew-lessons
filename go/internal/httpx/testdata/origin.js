'use strict';

// Writes go/internal/httpx/testdata/golden/origin.jsonl from Node's
// originAllowed and requestOrigin. Cases have no X-Forwarded-Proto; Go's
// trusted-proxy scheme rule is covered by the Go table tests.
// Go tests read the jsonl and do not run Node. Regenerate (Node 22.14.0):
//
//   node go/internal/httpx/testdata/origin.js

const fs = require('fs');
const path = require('path');

const security = require(path.join(__dirname, '../../../../server/http/security'));

const cases = [
    { id: 'public', publicOrigin: 'https://hebrew.example/', cookieSecure: true, headers: { host: '127.0.0.1:8090', origin: 'https://hebrew.example' } },
    { id: 'public-slash', publicOrigin: 'https://hebrew.example', cookieSecure: true, headers: { host: '127.0.0.1', origin: 'https://hebrew.example/' } },
    { id: 'host-https', cookieSecure: true, headers: { host: 'hebrew.example', origin: 'https://hebrew.example' } },
    { id: 'host-http', cookieSecure: false, headers: { host: 'localhost:8090', origin: 'http://localhost:8090' } },
    { id: 'mismatch', publicOrigin: 'https://hebrew.example', cookieSecure: true, headers: { host: 'hebrew.example', origin: 'https://evil.example' } },
    { id: 'missing', cookieSecure: true, headers: { host: 'hebrew.example' } },
    { id: 'null-origin-referer', cookieSecure: true, headers: { host: 'hebrew.example', origin: 'null', referer: 'https://hebrew.example/words/?q=1' } },
    { id: 'referer-port', cookieSecure: true, headers: { host: 'hebrew.example', referer: 'https://hebrew.example:443/words/' } },
    { id: 'bad-referer', cookieSecure: false, headers: { host: 'hebrew.example', referer: 'not a url' } },
    { id: 'origin-beats-referer', publicOrigin: 'https://hebrew.example', cookieSecure: true, headers: { host: 'hebrew.example', origin: 'https://evil.example', referer: 'https://hebrew.example/' } }
];

const lines = [JSON.stringify({ kind: 'meta', node: process.version })];
for (const item of cases) {
    if (item.publicOrigin) process.env.PUBLIC_ORIGIN = item.publicOrigin;
    else delete process.env.PUBLIC_ORIGIN;
    process.env.COOKIE_SECURE = item.cookieSecure ? '1' : '';
    const req = { headers: item.headers || {} };
    const origin = security.requestOrigin(req);
    lines.push(JSON.stringify({
        kind: 'case',
        id: item.id,
        publicOrigin: item.publicOrigin || '',
        cookieSecure: Boolean(item.cookieSecure),
        headers: item.headers || {},
        allowed: security.originAllowed(req),
        origin: origin == null ? null : origin
    }));
}

const outDir = path.join(__dirname, 'golden');
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, 'origin.jsonl'), lines.join('\n') + '\n');
