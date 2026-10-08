#!/usr/bin/env node
/**
 * Pull a lesson from a running MCP server into this repo (or LESSONS_DIR).
 *
 * Usage:
 *   node scripts/rescue-lesson.js <slug> [mcpBaseUrl]
 *
 * Example (local container with MCP_ALLOW_NO_AUTH):
 *   node scripts/rescue-lesson.js my-morning http://127.0.0.1:8787
 *
 * With a bearer token:
 *   set MCP_AUTH_TOKEN=...
 *   node scripts/rescue-lesson.js my-morning https://hebrew.ibhacked.us
 *
 * Call this against the live MCP before the next image deploy if a lesson
 * exists only on the pod's writable volume.
 */
const fs = require('fs');
const path = require('path');
const http = require('http');
const https = require('https');

const slug = process.argv[2];
const base = (process.argv[3] || 'http://127.0.0.1:8787').replace(/\/$/, '');
if (!slug) {
    console.error('Usage: node scripts/rescue-lesson.js <slug> [mcpBaseUrl]');
    process.exit(1);
}

const root = process.env.HEBREW_ROOT
    ? path.resolve(process.env.HEBREW_ROOT)
    : path.resolve(__dirname, '..');
const lessonsRoot = process.env.LESSONS_DIR
    ? path.resolve(process.env.LESSONS_DIR)
    : path.join(root, 'worksheets');

function post(url, body, headers) {
    const lib = url.startsWith('https') ? https : http;
    return new Promise((resolve, reject) => {
        const req = lib.request(url, { method: 'POST', headers }, res => {
            const chunks = [];
            res.on('data', c => chunks.push(c));
            res.on('end', () => {
                const raw = Buffer.concat(chunks).toString('utf8');
                let data;
                try { data = JSON.parse(raw); } catch (err) {
                    reject(new Error('Non-JSON response (' + res.statusCode + '): ' + raw.slice(0, 200)));
                    return;
                }
                if (res.statusCode >= 400) reject(new Error('HTTP ' + res.statusCode + ': ' + raw.slice(0, 300)));
                else resolve(data);
            });
        });
        req.on('error', reject);
        req.write(JSON.stringify(body));
        req.end();
    });
}

async function mcpCall(name, args) {
    const headers = {
        'Content-Type': 'application/json',
        Accept: 'application/json, text/event-stream'
    };
    if (process.env.MCP_AUTH_TOKEN) headers.Authorization = 'Bearer ' + process.env.MCP_AUTH_TOKEN;
    const body = {
        jsonrpc: '2.0',
        id: 1,
        method: 'tools/call',
        params: { name, arguments: args }
    };
    const data = await post(base + '/mcp', body, headers);
    if (data.error) throw new Error(JSON.stringify(data.error));
    const text = (data.result && data.result.content || [])
        .filter(part => part.type === 'text')
        .map(part => part.text)
        .join('\n');
    if (data.result && data.result.isError) throw new Error(text || 'tool error');
    return JSON.parse(text);
}

(async () => {
    const result = await mcpCall('get_lesson', { slug });
    const worksheet = result.worksheet;
    const catalog = result.catalog;
    if (!worksheet) throw new Error('No worksheet in MCP response.');

    const dir = path.join(lessonsRoot, slug);
    const templateShell = path.join(root, 'worksheets', '_template', 'index.html');
    fs.mkdirSync(dir, { recursive: true });
    const shell = path.join(dir, 'index.html');
    if (!fs.existsSync(shell)) {
        if (!fs.existsSync(templateShell)) throw new Error('Missing worksheets/_template/index.html');
        fs.copyFileSync(templateShell, shell);
    }
    fs.writeFileSync(
        path.join(dir, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );

    const catalogFile = path.join(lessonsRoot, 'catalog.js');
    let list = [];
    if (fs.existsSync(catalogFile)) {
        // Catalog is data: parsed as a literal, never executed (see server/lesson-data.js).
        const { loadDataWindow } = require('../server/lesson-data');
        const data = loadDataWindow(catalogFile);
        list = Array.isArray(data.HEBREW_CATALOG) ? data.HEBREW_CATALOG : [];
    }
    const entry = catalog || {
        slug,
        title: worksheet.title || slug,
        subtitle: worksheet.subtitle || '',
        blurb: 'Run it again. The next round hides the English until you ask, and keeps the vowels.'
    };
    const next = list.filter(item => item && item.slug !== slug);
    next.push(entry);
    fs.writeFileSync(
        catalogFile,
        'window.HEBREW_CATALOG = ' + JSON.stringify(next, null, 4) + ';\n',
        'utf8'
    );
    console.log('Wrote ' + path.relative(root, dir).replace(/\\/g, '/') + ' and updated catalog.');
})().catch(err => {
    console.error(err.message || err);
    process.exit(1);
});
