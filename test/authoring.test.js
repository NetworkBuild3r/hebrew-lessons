/**
 * Lesson authoring service tests (INIT-002/SPEC-007).
 * Temp DATA_DIR + LESSONS_DIR, fake TTS, no network.
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('crypto');
const fs = require('fs');
const https = require('https');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

function sha256(bufOrPath) {
    if (Buffer.isBuffer(bufOrPath) || typeof bufOrPath === 'string' && !fs.existsSync(bufOrPath)) {
        return crypto.createHash('sha256').update(bufOrPath).digest('hex');
    }
    return crypto.createHash('sha256').update(fs.readFileSync(bufOrPath)).digest('hex');
}

function sampleWorksheet(slug, overrides) {
    const base = {
        id: slug,
        title: 'בדיקה — ' + slug,
        subtitle: 'test',
        level: 'basic',
        vocab: {
            items: [{
                id: '1',
                word: 'שָׁלוֹם',
                meaning: 'hello',
                sound: 'sha-LOM',
                trap: 'Stress the last syllable.',
                sentence: 'שָׁלוֹם עוֹלָם.'
            }]
        },
        reading: {
            paragraphs: ['שָׁלוֹם עוֹלָם.']
        },
        activities: [{ type: 'words' }, { type: 'story' }]
    };
    return Object.assign(base, overrides || {});
}

function stubHttps(payloadFactory) {
    const orig = https.request;
    let calls = 0;
    https.request = function (url, options, cb) {
        calls += 1;
        let body = '';
        const listeners = { data: [], end: [] };
        const res = {
            statusCode: 200,
            on(ev, fn) {
                if (ev === 'data') listeners.data.push(fn);
                if (ev === 'end') listeners.end.push(fn);
                return res;
            }
        };
        if (typeof options === 'function') cb = options;
        if (typeof cb === 'function') cb(res);
        return {
            on() { return this; },
            write(chunk) { body += chunk; },
            end() {
                let text = '';
                try { text = JSON.parse(body).text; } catch (e) { /* ignore */ }
                const buf = Buffer.from(
                    typeof payloadFactory === 'function' ? payloadFactory(text, calls) : payloadFactory
                );
                process.nextTick(() => {
                    listeners.data.forEach((fn) => fn(buf));
                    listeners.end.forEach((fn) => fn());
                });
            }
        };
    };
    return {
        restore() { https.request = orig; },
        get calls() { return calls; }
    };
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/repos/visibility',
        '../server/db/repos/audit',
        '../server/db/repos/users',
        '../server/auth/policy',
        '../server/visibility',
        '../server/accounts',
        '../server/audio',
        '../server/vocab-index',
        '../server/authoring',
        '../mcp/store'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

function listFilesRecursive(dir, base) {
    const out = [];
    if (!fs.existsSync(dir)) return out;
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        const rel = path.relative(base || dir, full);
        if (entry.isDirectory()) out.push(...listFilesRecursive(full, base || dir));
        else out.push(rel.replace(/\\/g, '/'));
    }
    return out.sort();
}

describe('authoring service (SPEC-007)', () => {
    let tmpData;
    let tmpLessons;
    let db;
    let authoring;
    let store;
    let visibility;
    let audio;
    let prevEnv;

    before(() => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-auth-data-'));
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-auth-lessons-'));
        fs.mkdirSync(path.join(tmpLessons, '_audio', 'words'), { recursive: true });
        fs.writeFileSync(
            path.join(tmpLessons, 'catalog.js'),
            'window.HEBREW_CATALOG = [];\n',
            'utf8'
        );

        prevEnv = {
            HEBREW_ROOT: process.env.HEBREW_ROOT,
            DATA_DIR: process.env.DATA_DIR,
            LESSONS_DIR: process.env.LESSONS_DIR,
            ELEVENLABS_API_KEY: process.env.ELEVENLABS_API_KEY,
            ELEVENLABS_VOICE_ID: process.env.ELEVENLABS_VOICE_ID,
            ELEVENLABS_MODEL: process.env.ELEVENLABS_MODEL
        };
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmpData;
        process.env.LESSONS_DIR = tmpLessons;
        process.env.ELEVENLABS_API_KEY = 'test-key-authoring';
        process.env.ELEVENLABS_VOICE_ID = 'voice';
        process.env.ELEVENLABS_MODEL = 'eleven_v3';
        clearModules();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        authoring = require('../server/authoring');
        store = require('../mcp/store');
        visibility = require('../server/visibility');
        audio = require('../server/audio');
    });

    after(() => {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        try {
            fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(tmpLessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        for (const [k, v] of Object.entries(prevEnv)) {
            if (v === undefined) delete process.env[k];
            else process.env[k] = v;
        }
        clearModules();
    });

    beforeEach(() => {
        // Keep catalog; remove lesson folders except meta.
        for (const name of fs.readdirSync(tmpLessons)) {
            if (name === 'catalog.js' || name.startsWith('_')) continue;
            fs.rmSync(path.join(tmpLessons, name), { recursive: true, force: true });
        }
        fs.writeFileSync(
            path.join(tmpLessons, 'catalog.js'),
            'window.HEBREW_CATALOG = [];\n',
            'utf8'
        );
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM lesson_grants');
        db.run('DELETE FROM lesson_visibility');
    });

    it('ac-1: save matches upsertLesson worksheet.js and catalog bytes', () => {
        const slug = 'ac1-match';
        const worksheet = sampleWorksheet(slug);
        const catalogEntry = {
            slug,
            title: worksheet.title,
            subtitle: 'sub',
            blurb: 'blurb one',
            level: 'basic'
        };

        // Path A: upsertLesson
        store.upsertLesson({
            slug,
            worksheet: JSON.parse(JSON.stringify(worksheet)),
            title: catalogEntry.title,
            subtitle: catalogEntry.subtitle,
            blurb: catalogEntry.blurb,
            level: catalogEntry.level,
            audio: false
        });
        const wsA = fs.readFileSync(path.join(tmpLessons, slug, 'worksheet.js'));
        const catA = fs.readFileSync(path.join(tmpLessons, 'catalog.js'));

        // Reset and path B: authoring.save
        fs.rmSync(path.join(tmpLessons, slug), { recursive: true, force: true });
        fs.writeFileSync(path.join(tmpLessons, 'catalog.js'), 'window.HEBREW_CATALOG = [];\n', 'utf8');

        authoring.save('admin', slug, JSON.parse(JSON.stringify(worksheet)), catalogEntry, { create: true });
        const wsB = fs.readFileSync(path.join(tmpLessons, slug, 'worksheet.js'));
        const catB = fs.readFileSync(path.join(tmpLessons, 'catalog.js'));

        assert.equal(Buffer.compare(wsA, wsB), 0, 'worksheet.js bytes must match');
        assert.equal(Buffer.compare(catA, catB), 0, 'catalog.js bytes must match');
    });

    it('ac-2: validator error writes nothing; prior bytes unchanged', () => {
        const slug = 'ac2-block';
        const good = sampleWorksheet(slug);
        authoring.save('admin', slug, good, { title: good.title }, { create: true });
        const wsBefore = fs.readFileSync(path.join(tmpLessons, slug, 'worksheet.js'));
        const catBefore = fs.readFileSync(path.join(tmpLessons, 'catalog.js'));
        const indexBefore = fs.readFileSync(path.join(tmpLessons, slug, 'index.html'));

        const bad = sampleWorksheet(slug, {
            vocab: { items: [{ id: '1', word: 'שָׁלוֹם' }] } // missing meaning/sound/trap
        });
        assert.throws(() => {
            authoring.save('admin', slug, bad, { title: bad.title });
        }, /missing/i);

        assert.equal(
            Buffer.compare(wsBefore, fs.readFileSync(path.join(tmpLessons, slug, 'worksheet.js'))),
            0
        );
        assert.equal(
            Buffer.compare(catBefore, fs.readFileSync(path.join(tmpLessons, 'catalog.js'))),
            0
        );
        assert.equal(
            Buffer.compare(indexBefore, fs.readFileSync(path.join(tmpLessons, slug, 'index.html'))),
            0
        );
    });

    it('ac-3: generateAudio archives changed sentence; verify passes; forced verify fails', async () => {
        const slug = 'ac3-archive';
        const worksheet = sampleWorksheet(slug);
        authoring.save('admin', slug, worksheet, { title: worksheet.title }, { create: true });

        const clipRel = path.join('audio', 'sentences', 'vocab-1.mp3');
        const clipFull = path.join(tmpLessons, slug, clipRel);
        fs.mkdirSync(path.dirname(clipFull), { recursive: true });
        const oldBytes = Buffer.from('OLD-SENTENCE-CLIP');
        fs.writeFileSync(clipFull, oldBytes);
        const oldHash = sha256(oldBytes);
        const oldTextHash = audio.textHash('voice', 'eleven_v3', 'שָׁלוֹם עוֹלָם.');
        fs.writeFileSync(
            path.join(tmpLessons, slug, 'audio', 'manifest.json'),
            JSON.stringify({ version: 1, clips: { 'audio/sentences/vocab-1.mp3': oldTextHash } })
        );

        // Change sentence text.
        const updated = sampleWorksheet(slug, {
            vocab: {
                items: [{
                    id: '1',
                    word: 'שָׁלוֹם',
                    meaning: 'hello',
                    sound: 'sha-LOM',
                    trap: 'Stress the last syllable.',
                    sentence: 'שָׁלוֹם חָדָשׁ.'
                }]
            }
        });
        authoring.save('admin', slug, updated, { title: updated.title });
        // Restore old clip + old hash so regenerate archives.
        fs.writeFileSync(clipFull, oldBytes);
        fs.writeFileSync(
            path.join(tmpLessons, slug, 'audio', 'manifest.json'),
            JSON.stringify({ version: 1, clips: { 'audio/sentences/vocab-1.mp3': oldTextHash } })
        );

        const httpsStub = stubHttps('NEW-SENTENCE-CLIP');
        try {
            const result = await authoring.generateAudio('admin', slug, {});
            assert.ok(result.clipsCreated >= 1);
            const archiveName = 'vocab-1.' + oldHash.slice(0, 12) + '.mp3';
            const archivePath = path.join(tmpLessons, slug, 'audio', '_archive', archiveName);
            assert.ok(fs.existsSync(archivePath), 'expected SPEC-002 archive at ' + archivePath);
            assert.equal(sha256(archivePath), oldHash);
            assert.equal(fs.readFileSync(clipFull).toString(), 'NEW-SENTENCE-CLIP');

            await assert.rejects(
                () => authoring.generateAudio('admin', slug, { forceVerifyFailure: true }),
                /audio:verify failed/
            );
        } finally {
            httpsStub.restore();
        }
    });

    it('ac-4: audioPlan is dry (no network/files) and counts match generateAudio', async () => {
        const slug = 'ac4-plan';
        const worksheet = sampleWorksheet(slug, {
            reading: { paragraphs: ['שָׁלוֹם עוֹלָם.', 'עוֹד שׁוּרָה.'] }
        });
        authoring.save('admin', slug, worksheet, { title: worksheet.title }, { create: true });

        const clipFull = path.join(tmpLessons, slug, 'audio', 'sentences', 'vocab-1.mp3');
        fs.mkdirSync(path.dirname(clipFull), { recursive: true });
        const oldBytes = Buffer.from('PLAN-OLD');
        fs.writeFileSync(clipFull, oldBytes);
        const oldTextHash = audio.textHash('voice', 'eleven_v3', 'שָׁלוֹם עוֹלָם.');
        fs.writeFileSync(
            path.join(tmpLessons, slug, 'audio', 'manifest.json'),
            JSON.stringify({ version: 1, clips: { 'audio/sentences/vocab-1.mp3': oldTextHash } })
        );
        // Change sentence so plan expects regenerate + archive.
        const updated = sampleWorksheet(slug, {
            vocab: {
                items: [{
                    id: '1',
                    word: 'שָׁלוֹם',
                    meaning: 'hello',
                    sound: 'sha-LOM',
                    trap: 'Stress.',
                    sentence: 'שָׁלוֹם חָדָשׁ.'
                }]
            },
            reading: { paragraphs: ['שָׁלוֹם עוֹלָם.', 'עוֹד שׁוּרָה.'] }
        });
        authoring.save('admin', slug, updated, { title: updated.title });
        fs.writeFileSync(clipFull, oldBytes);
        fs.writeFileSync(
            path.join(tmpLessons, slug, 'audio', 'manifest.json'),
            JSON.stringify({ version: 1, clips: { 'audio/sentences/vocab-1.mp3': oldTextHash } })
        );

        const beforeFiles = new Map();
        for (const rel of listFilesRecursive(tmpLessons)) {
            const full = path.join(tmpLessons, rel);
            beforeFiles.set(rel, sha256(full));
        }

        const httpsStub = stubHttps((text) => 'TTS:' + text);
        try {
            const plan = await authoring.audioPlan(slug);
            assert.equal(httpsStub.calls, 0, 'audioPlan must not call TTS');
            for (const [rel, hash] of beforeFiles) {
                const full = path.join(tmpLessons, rel);
                assert.ok(fs.existsSync(full), 'audioPlan must not delete ' + rel);
                assert.equal(sha256(full), hash, 'audioPlan must not change ' + rel);
            }
            assert.ok(plan.clipsToArchive >= 1, 'expected at least one archive in plan');
            assert.ok(
                plan.sentencesToGenerate + plan.storyToGenerate >= 1,
                'expected clips to generate'
            );

            const result = await authoring.generateAudio('admin', slug, {});
            assert.ok(httpsStub.calls > 0, 'generateAudio should call fake TTS');
            assert.equal(
                result.wordsCreated,
                plan.wordsToGenerate,
                'wordsToGenerate should match wordsCreated'
            );
            assert.equal(
                result.clipsCreated,
                plan.sentencesToGenerate + plan.storyToGenerate,
                'clip generate counts should match'
            );
            const archives = fs.existsSync(path.join(tmpLessons, slug, 'audio', '_archive'))
                ? fs.readdirSync(path.join(tmpLessons, slug, 'audio', '_archive'))
                    .filter((n) => n.endsWith('.mp3'))
                : [];
            assert.ok(archives.length >= plan.clipsToArchive);
        } finally {
            httpsStub.restore();
        }
    });

    it('ac-5: bad/reserved slugs refused; dangerous strings round-trip as data', () => {
        assert.throws(() => authoring.loadForEdit('Bad_Slug'), /slug/i);
        assert.throws(() => authoring.loadForEdit('_template'), /reserved|slug/i);
        assert.throws(() => authoring.loadForEdit('_audio'), /reserved|slug/i);
        assert.throws(() => authoring.loadForEdit('_review'), /reserved|slug/i);
        assert.throws(() => authoring.save('admin', 'Not Valid', sampleWorksheet('x')), /slug/i);

        const slug = 'ac5-safe';
        const payload = '</script>`${process.env.SECRET}';
        const worksheet = sampleWorksheet(slug, {
            title: 'Safe ' + payload,
            vocab: {
                items: [{
                    id: '1',
                    word: 'שָׁלוֹם',
                    meaning: 'hello ' + payload,
                    sound: 'sha-LOM',
                    trap: 'trap ' + payload,
                    sentence: 'שָׁלוֹם.'
                }]
            }
        });
        authoring.save('admin', slug, worksheet, { title: worksheet.title }, { create: true });
        const loaded = authoring.loadForEdit(slug);
        assert.equal(loaded.worksheet.title, worksheet.title);
        assert.equal(loaded.worksheet.vocab.items[0].meaning, worksheet.vocab.items[0].meaning);
        assert.equal(loaded.worksheet.vocab.items[0].trap, worksheet.vocab.items[0].trap);
        // Literal parse — never executed.
        const raw = fs.readFileSync(path.join(tmpLessons, slug, 'worksheet.js'), 'utf8');
        assert.match(raw, /<\/script>/);
        assert.match(raw, /`/);
        assert.match(raw, /\$\{/);
        const { loadDataWindow } = require('../server/lesson-data');
        const again = loadDataWindow(path.join(tmpLessons, slug, 'worksheet.js')).WORKSHEET;
        assert.equal(again.vocab.items[0].meaning, worksheet.vocab.items[0].meaning);
    });

    it('ac-6: newly created lesson is hidden', () => {
        const slug = 'ac6-hidden';
        const worksheet = sampleWorksheet(slug);
        const saved = authoring.save('admin', slug, worksheet, { title: worksheet.title }, { create: true });
        assert.equal(saved.visibility.mode, 'hidden');
        assert.equal(visibility.canSeeLesson({ id: 1, role: 'user' }, slug), false);
        const loaded = authoring.loadForEdit(slug);
        assert.equal(loaded.visibility.mode, 'hidden');
    });

    it('ac-7: existing index.html hash unchanged after save', () => {
        const slug = 'ac7-index';
        const worksheet = sampleWorksheet(slug);
        authoring.save('admin', slug, worksheet, { title: worksheet.title }, { create: true });
        const indexPath = path.join(tmpLessons, slug, 'index.html');
        const before = sha256(indexPath);
        const marker = '<!-- ac7-marker -->';
        fs.writeFileSync(indexPath, fs.readFileSync(indexPath, 'utf8') + marker, 'utf8');
        const marked = sha256(indexPath);

        const next = sampleWorksheet(slug, { subtitle: 'updated subtitle' });
        authoring.save('admin', slug, next, { title: next.title, subtitle: 'updated subtitle' });
        assert.equal(sha256(indexPath), marked);
        assert.notEqual(marked, before);
        assert.match(fs.readFileSync(indexPath, 'utf8'), /ac7-marker/);
    });
});
