/**
 * Media and image-generation MCP tools (INIT-004/SPEC-012).
 * Temporary database. Handlers are called directly. The image transport is
 * injected. No live OpenAI call. The API key is never written to a reply.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const http = require('http');
const https = require('https');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const PNG = fs.readFileSync(path.join(root, 'test', 'fixtures', 'media', 'tiny.png'));
const PNG_B64 = PNG.toString('base64');
const API_KEY = 'test-key-spec012';
const TOKEN = 'test-media-mcp-token';
const ESTIMATE = 0.011;
const PROMPT = 'a red apple on a wooden table';

const LEGACY_TOOLS = [
    'list_lessons',
    'get_lesson',
    'list_words',
    'list_activity_types',
    'list_users',
    'upsert_lesson',
    'set_lesson_visibility',
    'generate_lesson_audio'
];
const PLAN_TOOLS = [
    'read_part',
    'replace_part',
    'insert_block',
    'move_block',
    'remove_block',
    'set_field',
    'publish_plan'
];
const MEDIA_TOOLS = ['upload_media', 'plan_image', 'generate_image'];

function replyText(result) {
    assert.ok(result && result.content && result.content[0]);
    return result.content[0].text;
}

function assertNoSecret(text) {
    assert.equal(text.includes(API_KEY), false);
    assert.equal(text.includes(TOKEN), false);
    assert.equal(text.includes(PNG_B64), false);
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/db/repos/media',
        '../server/db/repos/audit',
        '../server/media/sniff',
        '../server/media/codec',
        '../server/media/ingest',
        '../server/media/openai-images',
        '../server/api/router',
        '../server/api/admin-routes',
        '../server/web',
        '../mcp/tools',
        '../mcp/http'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

describe('media generation MCP tools (INIT-004/SPEC-012)', { concurrency: false }, () => {
    let tmp;
    let lessons;
    let db;
    let mediaRepo;
    let images;
    let ingest;
    let tools;
    let handlers;
    let metas;
    let originals;
    let httpsCalls;
    let originalRequest;
    let transportCalls;
    let prevEnv;
    const seen = { ingest: 0, approve: 0, plan: 0, generate: 0 };

    function transport(outbound) {
        transportCalls.push(outbound);
        return Promise.resolve({
            status: 200,
            body: { data: [{ b64_json: PNG_B64 }] }
        });
    }

    function useTransport() {
        transportCalls = [];
        tools._setMediaCallOptions({ transport });
    }

    function resetCaps() {
        images._resetGenerationState();
        ingest._resetUploadCounts();
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-media-mcp-'));
        lessons = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-media-mcp-lessons-'));
        fs.writeFileSync(path.join(lessons, 'catalog.js'), 'window.HEBREW_CATALOG = [];\n', 'utf8');
        prevEnv = {
            HEBREW_ROOT: process.env.HEBREW_ROOT,
            DATA_DIR: process.env.DATA_DIR,
            LESSONS_DIR: process.env.LESSONS_DIR,
            MCP_AUTH_TOKEN: process.env.MCP_AUTH_TOKEN,
            OPENAI_API_KEY: process.env.OPENAI_API_KEY,
            MCP_PORT: process.env.MCP_PORT,
            PORT: process.env.PORT,
            HOST: process.env.HOST
        };
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        process.env.LESSONS_DIR = lessons;
        process.env.MCP_AUTH_TOKEN = TOKEN;
        process.env.OPENAI_API_KEY = API_KEY;
        process.env.MCP_PORT = '0';
        process.env.PORT = '0';
        process.env.HOST = '127.0.0.1';
        clearModules();

        httpsCalls = 0;
        originalRequest = https.request;
        https.request = function refusedRequest() {
            httpsCalls += 1;
            throw new Error('refused');
        };

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        mediaRepo = require('../server/db/repos/media');
        images = require('../server/media/openai-images');
        ingest = require('../server/media/ingest');
        tools = require('../mcp/tools');

        originals = {
            ingestImage: ingest.ingestImage,
            approveMedia: ingest.approveMedia,
            planImage: images.planImage,
            generateImage: images.generateImage
        };
        ingest.ingestImage = function () {
            seen.ingest += 1;
            return originals.ingestImage.apply(this, arguments);
        };
        ingest.approveMedia = function () {
            seen.approve += 1;
            return originals.approveMedia.apply(this, arguments);
        };
        images.planImage = function () {
            seen.plan += 1;
            return originals.planImage.apply(this, arguments);
        };
        images.generateImage = function () {
            seen.generate += 1;
            return originals.generateImage.apply(this, arguments);
        };

        metas = Object.create(null);
        tools.registerLessonTools({
            registerTool(name, meta) {
                metas[name] = meta;
            }
        });
        handlers = tools.toolHandlers;
        resetCaps();
        useTransport();
    });

    after(() => {
        if (ingest && originals) {
            ingest.ingestImage = originals.ingestImage;
            ingest.approveMedia = originals.approveMedia;
        }
        if (images && originals) {
            images.planImage = originals.planImage;
            images.generateImage = originals.generateImage;
            try { images._resetGenerationState(); } catch (_) { /* ignore */ }
        }
        if (tools) tools._setMediaCallOptions(null);
        https.request = originalRequest;
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        try {
            fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(lessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        for (const [key, value] of Object.entries(prevEnv)) {
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        clearModules();
    });

    it('ac-1: media tools and the SPEC-005 plan tools are registered', () => {
        const names = Object.keys(handlers);
        for (const name of LEGACY_TOOLS) assert.equal(names.includes(name), true, name);
        for (const name of PLAN_TOOLS) assert.equal(names.includes(name), true, name);
        for (const name of MEDIA_TOOLS) {
            assert.equal(typeof handlers[name], 'function', name);
            assert.equal(metas[name] != null, true, name);
        }
        assert.equal(names.includes('get_learner_snapshot'), false);
        // INIT-006/SPEC-010 registers the profile tool. The snapshot name stays absent.
        assert.equal(typeof handlers.get_learner_profile, 'function');
        for (const name of names) {
            assert.equal(/approv/i.test(name), false, name);
        }
    });

    it('ac-2: plan_image does not call the transport and returns the dry-run', async () => {
        resetCaps();
        useTransport();
        const before = httpsCalls;
        const result = await handlers.plan_image({ prompt: PROMPT, count: 2 });
        assert.equal(result.isError, undefined);
        const body = JSON.parse(replyText(result));
        assert.equal(body.count, 2);
        assert.equal(body.estimatedDollars, 0.022);
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, before);
        assert.equal(seen.plan >= 1, true);
        assertNoSecret(replyText(result));
        assert.equal(replyText(result).includes('b64'), false);
    });

    it('ac-2: generate_image without confirm does not call the transport', async () => {
        resetCaps();
        useTransport();
        const beforeHttps = httpsCalls;
        const beforeIngest = seen.ingest;
        const result = await handlers.generate_image({
            prompt: PROMPT,
            confirm: false,
            estimate: ESTIMATE,
            id: 'no-confirm',
            actor: 'admin:mallory',
            model: 'dall-e-3'
        });
        assert.equal(result.isError, true);
        const body = JSON.parse(replyText(result));
        assert.equal(body.error, 'confirm_required');
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, beforeHttps);
        assert.equal(seen.ingest, beforeIngest);
        assert.equal(mediaRepo.getById('no-confirm'), null);
        assertNoSecret(replyText(result));
    });

    it('a confirm string and a missing token do not return an image body', async () => {
        resetCaps();
        useTransport();
        const asString = await handlers.generate_image({
            prompt: PROMPT,
            confirm: 'true',
            estimate: ESTIMATE,
            id: 'string-confirm'
        });
        assert.equal(asString.isError, true);
        assert.equal(transportCalls.length, 0);
        assertNoSecret(replyText(asString));

        const previousToken = process.env.MCP_AUTH_TOKEN;
        const previousKey = process.env.OPENAI_API_KEY;
        delete process.env.MCP_AUTH_TOKEN;
        try {
            for (const name of MEDIA_TOOLS) {
                const result = await handlers[name]({
                    prompt: PROMPT,
                    confirm: true,
                    estimate: ESTIMATE,
                    id: 'missing-token',
                    bytes: PNG_B64,
                    altText: 'a red apple'
                });
                assert.equal(result.isError, true, name);
                const text = replyText(result);
                assert.match(text, /MCP_AUTH_TOKEN/);
                assertNoSecret(text);
                assert.equal(text.includes('pending'), false, name);
            }
            assert.equal(mediaRepo.getById('missing-token'), null);
        } finally {
            process.env.MCP_AUTH_TOKEN = previousToken;
            process.env.OPENAI_API_KEY = previousKey;
        }
    });

    it('upload_media calls ingest and ignores a caller-supplied actor', async () => {
        resetCaps();
        const before = seen.ingest;
        const result = await handlers.upload_media({
            id: 'uploaded-apple',
            bytes: PNG_B64,
            altText: 'a red apple',
            actor: 'admin:mallory',
            filename: 'secret.png',
            path: path.join(root, 'test', 'fixtures', 'media', 'tiny.png')
        });
        assert.equal(result.isError, undefined, replyText(result));
        assert.equal(seen.ingest, before + 1);
        const body = JSON.parse(replyText(result));
        assert.equal(body.id, 'uploaded-apple');
        assert.equal(body.status, 'pending');
        assert.match(body.storedName, /^[0-9a-f]{64}\.png$/);
        assert.equal(body.altText, 'a red apple');
        const row = mediaRepo.getById('uploaded-apple');
        assert.equal(row.actor, 'mcp');
        assert.equal(row.status, 'pending');
        assert.equal(row.promptText, null);
        const uploaded = db.all(
            'SELECT actor, action, target, detail FROM audit_log WHERE action = ? AND target = ?',
            'media_upload',
            'uploaded-apple'
        );
        assert.equal(uploaded.length, 1);
        assert.equal(uploaded[0].actor, 'mcp');
        assert.equal(uploaded[0].detail, null);
        assertNoSecret(replyText(result));
        assert.equal(seen.approve, 0);
    });

    it('a filesystem path is not read as an upload', async () => {
        resetCaps();
        const result = await handlers.upload_media({
            id: 'from-path',
            bytes: path.join(root, 'test', 'fixtures', 'media', 'tiny.png')
        });
        assert.equal(result.isError, true);
        assert.equal(mediaRepo.getById('from-path'), null);
        assertNoSecret(replyText(result));
    });

    it('generate_image with confirm calls the service once and stays pending', async () => {
        resetCaps();
        useTransport();
        const beforeHttps = httpsCalls;
        const result = await handlers.generate_image({
            prompt: PROMPT,
            confirm: true,
            estimate: ESTIMATE,
            id: 'generated-apple',
            altText: 'a red apple',
            actor: 'admin:mallory',
            model: 'dall-e-3'
        });
        assert.equal(result.isError, undefined, replyText(result));
        assert.equal(transportCalls.length, 1);
        assert.equal(transportCalls[0].body.model, 'gpt-image-1');
        assert.equal(JSON.stringify(transportCalls[0].body).includes('dall-e'), false);
        assert.equal(httpsCalls, beforeHttps);
        const body = JSON.parse(replyText(result));
        assert.equal(body.status, 'pending');
        assert.equal(body.id, 'generated-apple');
        const row = mediaRepo.getById('generated-apple');
        assert.equal(row.actor, 'mcp');
        assert.equal(row.status, 'pending');
        assert.equal(row.provider, 'openai');
        assert.equal(row.modelId, 'gpt-image-1');
        assert.equal(row.promptText, PROMPT);
        const generated = db.all(
            'SELECT actor, action, target, detail FROM audit_log WHERE action = ? AND target = ?',
            'media_generate',
            'generated-apple'
        );
        assert.equal(generated.length, 1);
        assert.equal(generated[0].actor, 'mcp');
        assert.equal(generated[0].detail, null);
        assert.equal(JSON.stringify(generated).includes(PROMPT), false);
        assertNoSecret(replyText(result));
        assert.equal(seen.approve, 0);
    });

    it('an ingest error that contains the API key is not returned', async () => {
        resetCaps();
        useTransport();
        tools._setMediaCallOptions({
            transport,
            ingest: async () => {
                throw new Error('store failed ' + API_KEY);
            }
        });
        const result = await handlers.generate_image({
            prompt: PROMPT,
            confirm: true,
            estimate: ESTIMATE,
            id: 'key-leak',
            altText: 'a red apple'
        });
        assert.equal(result.isError, true);
        assertNoSecret(replyText(result));
        assert.equal(mediaRepo.getById('key-leak'), null);
        useTransport();
    });

    it('with the key unset, plan and generate do not call the transport', async () => {
        resetCaps();
        useTransport();
        const previous = process.env.OPENAI_API_KEY;
        delete process.env.OPENAI_API_KEY;
        try {
            const plan = await handlers.plan_image({ prompt: PROMPT });
            const generated = await handlers.generate_image({
                prompt: PROMPT,
                confirm: true,
                estimate: ESTIMATE,
                id: 'no-key'
            });
            assert.equal(plan.isError, true);
            assert.equal(generated.isError, true);
            assert.equal(transportCalls.length, 0);
            assert.equal(JSON.parse(replyText(plan)).error, 'missing_key');
            assert.equal(JSON.parse(replyText(generated)).error, 'missing_key');
            assertNoSecret(replyText(plan));
            assertNoSecret(replyText(generated));
        } finally {
            process.env.OPENAI_API_KEY = previous;
        }
    });

    it('the hourly cap is the image service cap, refused before another transport call', async () => {
        resetCaps();
        const now = Date.UTC(2026, 0, 2, 3, 0, 0);
        transportCalls = [];
        tools._setMediaCallOptions({
            transport,
            now,
            ingest: async (input) => ({
                id: input.id,
                status: 'pending',
                storedName: 'ab'.repeat(32) + '.png',
                altText: '',
                width: 1,
                height: 1
            })
        });
        for (let i = 0; i < 15; i += 1) {
            const ok = await handlers.generate_image({
                prompt: PROMPT,
                confirm: true,
                estimate: ESTIMATE,
                id: 'cap-' + i
            });
            assert.equal(ok.isError, undefined, replyText(ok));
        }
        assert.equal(transportCalls.length, 15);
        const refused = await handlers.generate_image({
            prompt: PROMPT,
            confirm: true,
            estimate: ESTIMATE,
            id: 'cap-15'
        });
        assert.equal(refused.isError, true);
        assert.equal(JSON.parse(replyText(refused)).error, 'hourly_cap');
        assert.equal(transportCalls.length, 15);
        assertNoSecret(replyText(refused));
        resetCaps();
        useTransport();
    });

    it('ac-3: no tool approves, and HTTP approval with only the MCP token is 403', async () => {
        assert.equal(typeof handlers.approve_media, 'undefined');
        assert.equal(seen.approve, 0);
        const row = mediaRepo.getById('uploaded-apple');
        assert.equal(row && row.status, 'pending');

        const web = require('../server/web');
        const server = web.listen();
        await new Promise((resolve) => server.once('listening', resolve));
        const port = server.address().port;
        const payload = JSON.stringify({ altText: 'a red apple' });
        function post(headers) {
            return new Promise((resolve, reject) => {
                const req = http.request({
                    host: '127.0.0.1',
                    port,
                    path: '/api/admin/media/uploaded-apple/approval',
                    method: 'POST',
                    headers: Object.assign({
                        'Content-Type': 'application/json',
                        Origin: 'http://127.0.0.1:' + port,
                        'Content-Length': Buffer.byteLength(payload)
                    }, headers)
                }, (res) => {
                    let data = '';
                    res.on('data', (chunk) => { data += chunk; });
                    res.on('end', () => resolve({ status: res.statusCode, data }));
                });
                req.on('error', reject);
                req.end(payload);
            });
        }
        try {
            const onlyToken = await post({ Authorization: 'Bearer ' + TOKEN });
            assert.equal(onlyToken.status, 403);
            assert.equal(onlyToken.data.includes(API_KEY), false);
            assert.equal(onlyToken.data.includes(TOKEN), false);
            assert.equal(onlyToken.data.includes(PNG_B64), false);
            const after = mediaRepo.getById('uploaded-apple');
            assert.equal(after.status, 'pending');
            assert.equal(seen.approve, 0);
        } finally {
            await new Promise((resolve) => server.close(resolve));
        }
    });

    it('a wrong MCP bearer is refused and the body has no image', async () => {
        const server = require('../mcp/http').listen();
        await new Promise((resolve) => server.once('listening', resolve));
        const port = server.address().port;
        const payload = JSON.stringify({
            jsonrpc: '2.0',
            id: 1,
            method: 'tools/call',
            params: {
                name: 'generate_image',
                arguments: {
                    prompt: PROMPT,
                    confirm: true,
                    estimate: ESTIMATE,
                    id: 'via-http'
                }
            }
        });
        function post(headers) {
            return new Promise((resolve, reject) => {
                const req = http.request({
                    host: '127.0.0.1',
                    port,
                    path: '/mcp',
                    method: 'POST',
                    headers: Object.assign({
                        'Content-Type': 'application/json',
                        Accept: 'application/json, text/event-stream',
                        'Content-Length': Buffer.byteLength(payload)
                    }, headers)
                }, (res) => {
                    let data = '';
                    res.on('data', (chunk) => { data += chunk; });
                    res.on('end', () => resolve({ status: res.statusCode, data }));
                });
                req.on('error', reject);
                req.end(payload);
            });
        }
        try {
            const wrong = await post({ Authorization: 'Bearer wrong-token' });
            assert.equal(wrong.status, 401);
            assert.equal(wrong.data.includes(API_KEY), false);
            assert.equal(wrong.data.includes(PNG_B64), false);
            const missing = await post({});
            assert.equal(missing.status, 401);
            assert.equal(missing.data.includes(PNG_B64), false);
        } finally {
            await new Promise((resolve) => server.close(resolve));
        }
    });
});
