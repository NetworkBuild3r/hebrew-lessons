/**
 * OpenAI image client. The transport is injected. fetch and https.request are
 * tripwired and must stay at zero calls. The fixture key is the string test-key.
 * Provenance: INIT-004/SPEC-009
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const fs = require('fs');
const http = require('http');
const https = require('https');
const os = require('os');
const path = require('path');
const { PassThrough } = require('stream');

const root = path.resolve(__dirname, '..');
const HOUR = 60 * 60 * 1000;
const T0 = Date.UTC(2026, 9, 9, 10, 0, 0);
const FIXED = {
    missing_key: 'Image generation is not configured.',
    prompt_refused: 'That prompt is not allowed.',
    confirm_required: 'Generation requires confirm from the dry-run.',
    estimate_mismatch: 'The estimate does not match the dry-run for one image.',
    hourly_cap: 'The image generation limit for this hour has been reached.',
    daily_cap: 'The image generation limit for this day has been reached.',
    generation_disabled: 'Image generation is disabled until the pinned model is restored.',
    provider_error: 'The image provider did not return an image.',
    invalid_input: 'The image request is not valid.',
    invalid_actor: 'The actor is not valid.',
    invalid_id: 'The media id is not valid.',
    store_failed: 'The image could not be stored.'
};

let networkCalls = 0;
function tripwire() {
    networkCalls += 1;
    throw new Error('network attempted');
}
https.request = tripwire;
https.get = tripwire;
http.request = tripwire;
http.get = tripwire;
globalThis.fetch = tripwire;

const pngBytes = fs.readFileSync(path.join(__dirname, 'fixtures', 'media', 'tiny.png'));
const pngB64 = pngBytes.toString('base64');

function assertRefusal(err, code) {
    if (!err || err.name !== 'ImageGenError' || err.code !== code) {
        throw new Error('expected refusal ' + code);
    }
    const blob = JSON.stringify({ name: err.name, code: err.code, message: err.message });
    if (blob.includes('test-key') || blob.includes('Bearer ') || /sk-[A-Za-z0-9_-]{8,}/.test(blob)) {
        throw new Error('refusal exposed a key-shaped string');
    }
    if (err.message !== FIXED[code]) {
        throw new Error('refusal message was not the fixed string for ' + code);
    }
    return true;
}

async function rejects(promise, code) {
    await assert.rejects(promise, (err) => assertRefusal(err, code));
}

function recordingTransport(response) {
    const calls = [];
    async function transport(outbound) {
        calls.push(outbound);
        if (typeof response === 'function') return response(calls.length, outbound);
        return response;
    }
    return { transport, calls };
}

function okResponse() {
    return { status: 200, body: { data: [{ b64_json: pngB64 }] } };
}

function stubStore() {
    const calls = [];
    async function ingestImage(input) {
        calls.push(input);
        return {
            id: input.id,
            storedName: 'ab'.repeat(32) + '.png',
            status: 'pending',
            altText: input.altText || '',
            width: 8,
            height: 8
        };
    }
    return { ingestImage, calls };
}

describe('openai image client', () => {
    const savedEnv = {
        HEBREW_ROOT: process.env.HEBREW_ROOT,
        DATA_DIR: process.env.DATA_DIR,
        LESSONS_DIR: process.env.LESSONS_DIR,
        OPENAI_API_KEY: process.env.OPENAI_API_KEY
    };
    const hadKey = Object.prototype.hasOwnProperty.call(process.env, 'OPENAI_API_KEY');
    let tmp;
    let db;
    let images;
    let media;

    function useKey(value) {
        if (value === undefined) delete process.env.OPENAI_API_KEY;
        else process.env.OPENAI_API_KEY = value;
    }

    function generate(extra, options) {
        const input = Object.assign({
            prompt: 'a red apple',
            confirm: true,
            estimate: 0.011,
            actor: 'admin:alice',
            id: 'red-apple'
        }, extra);
        return images.generateImage(input, options);
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-openai-images-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete process.env.LESSONS_DIR;
        for (const mod of [
            '../server/paths',
            '../server/db',
            '../server/db/repos/media',
            '../server/media/ingest',
            '../server/media/openai-images'
        ]) {
            try {
                delete require.cache[require.resolve(mod)];
            } catch (_) { /* not loaded yet */ }
        }
        db = require('../server/db');
        db.close();
        db.migrate();
        media = require('../server/db/repos/media');
        images = require('../server/media/openai-images');
        useKey('test-key');
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        for (const [key, value] of Object.entries(savedEnv)) {
            if (key === 'OPENAI_API_KEY') continue;
            if (value === undefined) delete process.env[key];
            else process.env[key] = value;
        }
        if (!hadKey) delete process.env.OPENAI_API_KEY;
        else process.env.OPENAI_API_KEY = savedEnv.OPENAI_API_KEY;
        if (networkCalls !== 0) {
            throw new Error('a test called fetch or https.request');
        }
    });

    beforeEach(() => {
        images._resetGenerationState();
        useKey('test-key');
        const ingest = require('../server/media/ingest');
        ingest._resetUploadCounts();
    });

    it('ac-1: dry-run of one image returns 0.011 and the fake records zero calls', async () => {
        const fake = recordingTransport(okResponse());
        const plan = await images.planImage(
            { prompt: 'a red apple' },
            { transport: fake.transport }
        );
        assert.equal(plan.count, 1);
        assert.equal(plan.estimatedDollars, 0.011);
        assert.equal(fake.calls.length, 0);
        assert.deepEqual(images._generationCounters(), {
            hourCount: 0,
            dayMills: 0,
            disabled: false
        });
    });

    it('ac-1: a counted dry-run is count times 0.011 and still records zero calls', async () => {
        const fake = recordingTransport(okResponse());
        const plan = await images.planImage(
            { prompt: 'a red apple', count: 2 },
            { transport: fake.transport }
        );
        assert.equal(plan.count, 2);
        assert.equal(plan.estimatedDollars, 0.022);
        assert.equal(fake.calls.length, 0);
    });

    it('ac-1: generate without confirm records zero calls', async () => {
        const fake = recordingTransport(okResponse());
        const store = stubStore();
        for (const confirm of [false, undefined]) {
            const input = {
                prompt: 'a red apple',
                estimate: 0.011,
                actor: 'admin:alice',
                id: 'red-apple'
            };
            if (confirm !== undefined) input.confirm = confirm;
            await rejects(
                images.generateImage(input, { transport: fake.transport, ingest: store.ingestImage }),
                'confirm_required'
            );
        }
        await rejects(
            generate({ confirm: 'true' }, { transport: fake.transport, ingest: store.ingestImage }),
            'confirm_required'
        );
        assert.equal(fake.calls.length, 0);
        assert.equal(store.calls.length, 0);
        assert.equal(images._generationCounters().hourCount, 0);
    });

    it('ac-2: with the key unset, plan and generate refuse and the fake records zero calls', async () => {
        const fake = recordingTransport(okResponse());
        useKey(undefined);
        await rejects(
            images.planImage({ prompt: 'a red apple' }, { transport: fake.transport }),
            'missing_key'
        );
        await rejects(
            generate({}, { transport: fake.transport, ingest: stubStore().ingestImage }),
            'missing_key'
        );
        useKey('');
        await rejects(images.planImage({ prompt: 'a red apple' }), 'missing_key');
        useKey('   ');
        await rejects(images.planImage({ prompt: 'a red apple' }), 'missing_key');
        assert.equal(fake.calls.length, 0);
    });

    it('ac-2: a missing-key child process refuses without calling out', () => {
        const env = Object.assign({}, process.env);
        delete env.OPENAI_API_KEY;
        const child = spawnSync(process.execPath, ['-e', `
            const m = require(${JSON.stringify(path.join(root, 'server/media/openai-images.js'))});
            let calls = 0;
            const transport = async () => { calls += 1; return { status: 200, body: {} }; };
            (async () => {
                let planErr = null;
                let genErr = null;
                try { await m.planImage({ prompt: 'a red apple' }, { transport }); }
                catch (err) { planErr = err; }
                try {
                    await m.generateImage({
                        prompt: 'a red apple', confirm: true, estimate: 0.011,
                        actor: 'mcp', id: 'one'
                    }, { transport });
                } catch (err) { genErr = err; }
                process.stdout.write(JSON.stringify({
                    calls,
                    plan: planErr && { name: planErr.name, code: planErr.code, message: planErr.message },
                    gen: genErr && { name: genErr.name, code: genErr.code, message: genErr.message }
                }));
            })();
        `], { env, cwd: root, encoding: 'utf8' });
        if (child.status !== 0) {
            throw new Error('missing-key subprocess failed with status ' + child.status);
        }
        let parsed;
        try {
            parsed = JSON.parse(child.stdout);
        } catch (_) {
            throw new Error('missing-key subprocess did not return a refusal record');
        }
        if (parsed.calls !== 0) throw new Error('missing-key subprocess called the transport');
        assertRefusal(parsed.plan, 'missing_key');
        assertRefusal(parsed.gen, 'missing_key');
    });

    it('ac-3: the 16th generate in an hour is refused with zero further provider calls', async () => {
        let calls = 0;
        const transport = async () => {
            calls += 1;
            throw new Error('provider down Bearer test-key');
        };
        const opts = { transport, ingest: stubStore().ingestImage, now: T0 };
        for (let i = 0; i < 15; i += 1) {
            await rejects(
                generate({ id: 'hour-' + i, actor: i % 2 === 0 ? 'admin:alice' : 'mcp' }, opts),
                'provider_error'
            );
        }
        assert.equal(calls, 15);
        assert.equal(images._generationCounters().hourCount, 15);
        await rejects(generate({ id: 'hour-16', actor: 'admin:bob' }, opts), 'hourly_cap');
        assert.equal(calls, 15);
        const plan = await images.planImage({ prompt: 'a red apple' }, { transport });
        assert.equal(plan.estimatedDollars, 0.011);
        assert.equal(calls, 15);
    });

    it('ac-3: a provider error counts, and the next hour is a new cap', async () => {
        let calls = 0;
        const transport = async () => {
            calls += 1;
            return { status: 500, body: { error: { message: 'sk-live-should-not-leak test-key' } } };
        };
        const store = stubStore();
        await rejects(
            generate({}, { transport, ingest: store.ingestImage, now: T0 }),
            'provider_error'
        );
        assert.equal(images._generationCounters().hourCount, 1);
        assert.equal(images._generationCounters().dayMills, 11);
        assert.equal(store.calls.length, 0);
        for (let i = 0; i < 14; i += 1) {
            await rejects(
                generate({ id: 'err-' + i }, { transport, ingest: store.ingestImage, now: T0 }),
                'provider_error'
            );
        }
        await rejects(
            generate({ id: 'err-15' }, { transport, ingest: store.ingestImage, now: T0 }),
            'hourly_cap'
        );
        assert.equal(calls, 15);
        await rejects(
            generate({ id: 'rewind' }, { transport, ingest: store.ingestImage, now: T0 - HOUR }),
            'hourly_cap'
        );
        assert.equal(calls, 15);
        const next = recordingTransport(okResponse());
        const stored = stubStore();
        const row = await generate(
            { id: 'next-hour' },
            { transport: next.transport, ingest: stored.ingestImage, now: T0 + HOUR }
        );
        assert.equal(row.status, 'pending');
        assert.equal(next.calls.length, 1);
        assert.equal(stored.calls.length, 1);
    });

    it('ac-3: the generate that would exceed $1 in the UTC day is refused', async () => {
        let calls = 0;
        const transport = async () => {
            calls += 1;
            if (calls === 1) {
                return { status: 502, body: { error: { message: 'after send' } } };
            }
            return okResponse();
        };
        const store = stubStore();
        const start = Date.UTC(2026, 9, 9, 0, 0, 0);
        await rejects(
            generate({ id: 'day-0' }, { transport, ingest: store.ingestImage, now: start }),
            'provider_error'
        );
        for (let i = 1; i < 90; i += 1) {
            const hour = Math.floor(i / 15);
            await generate(
                { id: 'day-' + i },
                { transport, ingest: store.ingestImage, now: start + hour * HOUR }
            );
        }
        assert.equal(calls, 90);
        assert.equal(images._generationCounters().dayMills, 990);
        await rejects(
            generate(
                { id: 'day-90' },
                { transport, ingest: store.ingestImage, now: start + 6 * HOUR }
            ),
            'daily_cap'
        );
        assert.equal(calls, 90);
        assert.equal(store.calls.length, 89);
        const nextDay = await generate(
            { id: 'next-day' },
            {
                transport,
                ingest: store.ingestImage,
                now: Date.UTC(2026, 9, 10, 0, 0, 0)
            }
        );
        assert.equal(nextDay.status, 'pending');
        assert.equal(calls, 91);
    });

    it('ac-4: the outbound body uses gpt-image-1 and no other model', async () => {
        const fake = recordingTransport(okResponse());
        const store = stubStore();
        await generate(
            { model: 'dall-e-3', learnerText: 'learner-typed-should-not-leak-9f3a' },
            { transport: fake.transport, ingest: store.ingestImage, now: T0 }
        );
        assert.equal(fake.calls.length, 1);
        const outbound = fake.calls[0];
        assert.equal(outbound.body.model, 'gpt-image-1');
        assert.equal(outbound.body.n, 1);
        assert.equal(outbound.body.size, '1024x1024');
        assert.equal(outbound.body.quality, 'low');
        assert.equal(outbound.body.prompt, 'a red apple');
        assert.deepEqual(Object.keys(outbound.body).sort(), ['model', 'n', 'prompt', 'quality', 'size']);
        assert.equal(outbound.headers.Authorization, 'Bearer test-key');
        assert.equal(JSON.stringify(outbound.body).includes('test-key'), false);
        assert.equal(JSON.stringify(outbound.body).includes('learner-typed'), false);
        assert.equal(store.calls[0].provider, 'openai');
        assert.equal(store.calls[0].modelId, 'gpt-image-1');
        assert.equal(store.calls[0].promptText, 'a red apple');
        assert.equal(store.calls[0].bytes.equals(pngBytes), true);

        const src = fs.readFileSync(path.join(root, 'server', 'media', 'openai-images.js'), 'utf8');
        const start = src.indexOf('function buildGenerationBody');
        const end = src.indexOf('\nfunction ', start + 1);
        const fn = src.slice(start, end);
        assert.deepEqual(fn.match(/['"][^'"]+['"]/g), ["'gpt-image-1'", "'1024x1024'", "'low'"]);
        for (const other of ['dall-e', 'imagen', 'stability', 'gpt-image-2', 'gpt-image-1.5']) {
            assert.equal(src.includes(other), false);
        }
    });

    it('refuses a sexual act, a minor, or a likeness before any provider call', async () => {
        const fake = recordingTransport(okResponse());
        const refused = [
            'a sexual act',
            'have sex',
            'a minor',
            'a child',
            'a 12 year old',
            'someone under 18',
            'a real person',
            "a real person's likeness",
            'min\u200Bor on a chair'
        ];
        for (const prompt of refused) {
            await rejects(
                images.planImage({ prompt }, { transport: fake.transport }),
                'prompt_refused'
            );
            await rejects(
                generate({ prompt }, { transport: fake.transport, ingest: stubStore().ingestImage }),
                'prompt_refused'
            );
        }
        assert.equal(fake.calls.length, 0);
        const allowed = await images.planImage({ prompt: 'mineral water in Essex' });
        assert.equal(allowed.estimatedDollars, 0.011);
        const adult = await images.planImage({ prompt: 'an 18 year old oak tree' });
        assert.equal(adult.count, 1);
    });

    it('does not send learner-typed text when it is the only text', async () => {
        const fake = recordingTransport(okResponse());
        const marker = 'learner-typed-should-not-leak-9f3a';
        await rejects(
            images.generateImage(
                { learnerText: marker, confirm: true, estimate: 0.011, actor: 'mcp', id: 'x' },
                { transport: fake.transport }
            ),
            'invalid_input'
        );
        assert.equal(fake.calls.length, 0);
    });

    it('refuses a prompt that contains the key and does not call out', async () => {
        const fake = recordingTransport(okResponse());
        await rejects(
            generate(
                { prompt: 'a red apple and test-key' },
                { transport: fake.transport, ingest: stubStore().ingestImage }
            ),
            'prompt_refused'
        );
        assert.equal(fake.calls.length, 0);
    });

    it('refuses a mismatched estimate with zero provider calls', async () => {
        const fake = recordingTransport(okResponse());
        await rejects(
            generate({ estimate: 0.022 }, { transport: fake.transport }),
            'estimate_mismatch'
        );
        await rejects(
            generate({ estimate: '0.011' }, { transport: fake.transport }),
            'estimate_mismatch'
        );
        assert.equal(fake.calls.length, 0);
        assert.equal(images._generationCounters().hourCount, 0);
    });

    it('a rejected model id disables generation and is not replaced', async () => {
        let calls = 0;
        const transport = async (outbound) => {
            calls += 1;
            assert.equal(outbound.body.model, 'gpt-image-1');
            return {
                status: 400,
                body: {
                    error: {
                        param: 'model',
                        code: 'model_not_found',
                        message: 'use dall-e-3 instead'
                    }
                }
            };
        };
        await rejects(
            generate({}, { transport, ingest: stubStore().ingestImage, now: T0 }),
            'generation_disabled'
        );
        assert.equal(calls, 1);
        assert.equal(images._generationCounters().disabled, true);
        assert.equal(images._generationCounters().hourCount, 1);
        await rejects(
            generate({ id: 'again' }, { transport, ingest: stubStore().ingestImage, now: T0 }),
            'generation_disabled'
        );
        assert.equal(calls, 1);
        const plan = await images.planImage({ prompt: 'a red apple' });
        assert.equal(plan.estimatedDollars, 0.011);
    });

    it('does not follow a provider url', async () => {
        const fake = recordingTransport({
            status: 200,
            body: { data: [{ url: 'https://evil.example/secret.png' }] }
        });
        await rejects(
            generate({}, { transport: fake.transport, ingest: stubStore().ingestImage, now: T0 }),
            'provider_error'
        );
        assert.equal(fake.calls.length, 1);
        assert.equal(images._generationCounters().hourCount, 1);
        assert.equal(networkCalls, 0);
    });

    it('the real transport posts only to api.openai.com and puts the key in the header', async () => {
        const captured = {};
        function requestFn(options, callback) {
            captured.options = options;
            const chunks = [];
            return {
                on() { return this; },
                setTimeout() { return this; },
                destroy() { return this; },
                write(chunk) {
                    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
                    return true;
                },
                end(chunk) {
                    if (chunk) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
                    captured.body = Buffer.concat(chunks).toString('utf8');
                    const res = new PassThrough();
                    res.statusCode = 200;
                    callback(res);
                    res.end(JSON.stringify({ data: [{ b64_json: pngB64 }] }));
                    return this;
                }
            };
        }
        const store = stubStore();
        const transport = images.createHttpsTransport(requestFn);
        await generate(
            {},
            {
                transport,
                ingest: store.ingestImage,
                now: T0
            }
        );
        assert.equal(captured.options.protocol, 'https:');
        assert.equal(captured.options.hostname, 'api.openai.com');
        assert.equal(captured.options.path, '/v1/images/generations');
        assert.equal(captured.options.method, 'POST');
        assert.equal(captured.options.headers.Authorization, 'Bearer test-key');
        const sent = JSON.parse(captured.body);
        assert.equal(sent.model, 'gpt-image-1');
        assert.equal(JSON.stringify(sent).includes('test-key'), false);
        assert.equal(store.calls[0].bytes.equals(pngBytes), true);
        assert.equal(networkCalls, 0);
    });

    it('passes generated bytes through ingest and stores a pending openai row', async () => {
        const fake = recordingTransport(okResponse());
        const result = await generate(
            { id: 'stored-apple', actor: 'mcp', prompt: '  a   red   apple  ' },
            { transport: fake.transport, now: T0 }
        );
        assert.equal(result.status, 'pending');
        assert.match(result.storedName, /^[0-9a-f]{64}\.png$/);
        const row = media.getById('stored-apple');
        assert.equal(row.status, 'pending');
        assert.equal(row.provider, 'openai');
        assert.equal(row.modelId, 'gpt-image-1');
        assert.equal(row.actor, 'mcp');
        assert.equal(row.promptText, 'a red apple');
        assert.equal(typeof row.createdAt, 'string');
        assert.equal(row.createdAt.length > 0, true);
        const blob = JSON.stringify(row);
        if (blob.includes('test-key') || blob.includes('Bearer ')) {
            throw new Error('media row exposed a key-shaped string');
        }
        const file = fs.readFileSync(path.join(tmp, 'media', row.storedName));
        assert.equal(file.subarray(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])), true);
        assert.equal(fake.calls.length, 1);
    });
});
