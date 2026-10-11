/**
 * Lexicon validator, write service, and vocab seed.
 * Temporary database only. Does not open data/hebrew.sqlite.
 * Provenance: INIT-006/SPEC-005
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const LESSON_SLUGS = ['my-morning', 'shai-date-indirect-speech', 'anal-sex-vocab'];

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/audit',
        '../server/db/repos/lexicon',
        '../server/vocab-index',
        '../server/lesson-data',
        '../server/lexicon/validate',
        '../server/lexicon/service',
        '../server/lexicon/seed'
    ];
    const migrationsDir = path.join(root, 'server', 'db', 'migrations');
    for (const file of fs.readdirSync(migrationsDir)) {
        if (/^\d{3,}-.+\.js$/.test(file)) names.push('../server/db/migrations/' + file);
    }
    for (const mod of names) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    return {
        db,
        audit: require('../server/db/repos/audit'),
        validate: require('../server/lexicon/validate'),
        service: require('../server/lexicon/service'),
        seed: require('../server/lexicon/seed')
    };
}

function copyLessons(dest) {
    for (const slug of LESSON_SLUGS) {
        const dir = path.join(dest, slug);
        fs.mkdirSync(dir, { recursive: true });
        fs.copyFileSync(
            path.join(root, 'worksheets', slug, 'worksheet.js'),
            path.join(dir, 'worksheet.js')
        );
    }
}

function countEntries(db) {
    return db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n;
}

describe('lexicon service', () => {
    let tmp;
    let lessons;
    let db;
    let audit;
    let service;
    let seed;
    let prevData;
    let prevLessons;
    let prevRoot;

    before(() => {
        prevData = process.env.DATA_DIR;
        prevLessons = process.env.LESSONS_DIR;
        prevRoot = process.env.HEBREW_ROOT;
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-lexicon-svc-'));
        lessons = path.join(tmp, 'lessons');
        fs.mkdirSync(lessons, { recursive: true });
        copyLessons(lessons);
        process.env.LESSONS_DIR = lessons;
        const mods = freshModules(path.join(tmp, 'data'));
        db = mods.db;
        audit = mods.audit;
        service = mods.service;
        seed = mods.seed;
        db.open();
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        if (prevData === undefined) delete process.env.DATA_DIR;
        else process.env.DATA_DIR = prevData;
        if (prevLessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = prevLessons;
        if (prevRoot === undefined) delete process.env.HEBREW_ROOT;
        else process.env.HEBREW_ROOT = prevRoot;
    });

    it('ac-1: a batch with one invalid entry writes nothing and returns every error', () => {
        const beforeCount = countEntries(db);
        const result = service.upsert([
            {
                key: 'סֵפֶר',
                kind: 'lemma',
                gloss_en: 'book',
                pos: 'noun'
            },
            {
                key: 'חָשַׁב',
                kind: 'lemma',
                gloss_en: 'thought <script>alert(1)</script>',
                pos: 'not-a-pos',
                root: 'not-a-root'
            }
        ], { actor: 'admin:ada', expectedRev: 0 });

        assert.equal(result.ok, false);
        assert.ok(Array.isArray(result.errors));
        assert.ok(result.errors.length >= 3);
        for (const error of result.errors) {
            assert.equal(typeof error.key, 'string');
            assert.equal(typeof error.field, 'string');
            assert.equal(typeof error.code, 'string');
            assert.ok(error.key);
            assert.ok(error.field);
            assert.ok(error.code);
        }
        const fields = result.errors.map((error) => error.field + ':' + error.code);
        assert.ok(fields.includes('gloss_en:markup'));
        assert.ok(fields.includes('pos:enum'));
        assert.ok(fields.includes('root:root_shape'));
        assert.equal(countEntries(db), beforeCount);
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', 'סֵפֶר'), undefined);
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', 'חָשַׁב'), undefined);
        assert.equal(audit.listRecent(20).some((row) => row.action === 'lexicon_upsert'), false);
    });

    it('ac-2: markup is rejected and NFD input is stored NFC', () => {
        const rejected = service.upsert([
            { key: 'דֶּלֶת', kind: 'lemma', gloss_en: '<script>' }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(rejected.ok, false);
        assert.ok(rejected.errors.some((error) => error.field === 'gloss_en' && error.code === 'markup'));
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', 'דֶּלֶת'), undefined);

        const nfdKey = 'שלום' + 'e\u0301';
        const nfdGloss = 'cafe\u0301';
        assert.notEqual(nfdKey, nfdKey.normalize('NFC'));
        assert.notEqual(nfdGloss, nfdGloss.normalize('NFC'));
        const stored = service.upsert([
            { key: nfdKey, kind: 'lemma', gloss_en: nfdGloss }
        ], { actor: 'mcp', expectedRev: 0 });
        assert.equal(stored.ok, true);
        assert.equal(stored.entries[0].key, nfdKey.normalize('NFC'));
        assert.equal(stored.entries[0].gloss_en, nfdGloss.normalize('NFC'));
        assert.equal(stored.entries[0].source, 'mcp');
        assert.equal(stored.entries[0].verified, 0);
        const row = db.get('SELECT key, gloss_en FROM lexicon_entries WHERE key = ?', nfdKey.normalize('NFC'));
        assert.equal(row.key, nfdKey.normalize('NFC'));
        assert.equal(row.gloss_en, 'café');
    });

    it('ac-3: a stale expectedRev returns revision_conflict and leaves the row', () => {
        const created = service.upsert([
            { key: 'כֶּלֶב', kind: 'lemma', gloss_en: 'dog', pos: 'noun' }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(created.ok, true);
        assert.equal(created.entries[0].rev, 1);
        const before = db.get('SELECT gloss_en, rev, source FROM lexicon_entries WHERE key = ?', 'כֶּלֶב');

        const stale = service.upsert([
            { key: 'כֶּלֶב', kind: 'lemma', gloss_en: 'hound', pos: 'noun' }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(stale.ok, false);
        assert.equal(stale.error, 'revision_conflict');
        assert.equal(stale.conflicts[0].key, 'כֶּלֶב');
        assert.equal(stale.conflicts[0].rev, 1);

        const after = db.get('SELECT gloss_en, rev, source FROM lexicon_entries WHERE key = ?', 'כֶּלֶב');
        assert.deepEqual(after, before);

        const mixed = service.upsert([
            { key: 'כֶּלֶב', kind: 'lemma', gloss_en: 'hound', pos: 'noun' },
            { key: 'חָתוּל', kind: 'lemma', gloss_en: 'cat', pos: 'noun' }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(mixed.error, 'revision_conflict');
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', 'חָתוּל'), undefined);
    });

    it('ac-4: editing a verified form clears verified; setVerified audits and refuses mcp', () => {
        const created = service.upsert([
            { key: 'לָמַד', kind: 'lemma', gloss_en: 'he studied', pos: 'verb', root: 'ל.מ.ד', binyan: 'paal' },
            {
                key: 'לָמַדְתִּי',
                kind: 'form',
                lemma_key: 'לָמַד',
                gloss_en: 'I studied',
                tense: 'past',
                cell: '1s',
                verified: 1
            }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(created.ok, true);
        const form = created.entries.find((entry) => entry.kind === 'form');
        assert.equal(form.verified, 0);

        const refused = service.setVerified(['לָמַדְתִּי'], 1, { actor: 'mcp' });
        assert.equal(refused.ok, false);
        assert.equal(refused.error, 'forbidden');
        assert.equal(
            audit.listRecent(20).some((row) => row.action === 'lexicon_verify'),
            false
        );
        assert.equal(db.get('SELECT verified FROM lexicon_entries WHERE key = ?', 'לָמַדְתִּי').verified, 0);

        const verified = service.setVerified(['לָמַדְתִּי'], 1, { actor: 'admin:ada' });
        assert.equal(verified.ok, true);
        assert.equal(verified.entries[0].verified, 1);
        assert.equal(verified.entries[0].verified_by, 'admin:ada');
        const verifyAudit = audit.listRecent(20).find((row) => row.action === 'lexicon_verify');
        assert.ok(verifyAudit);
        assert.equal(verifyAudit.actor, 'admin:ada');
        const detail = JSON.parse(verifyAudit.detail);
        assert.equal(detail.count, 1);
        assert.equal(detail.value, 1);
        assert.equal(verifyAudit.detail.includes('<'), false);
        assert.equal(verifyAudit.detail.includes('לָמַד'), false);

        const edited = service.upsert([
            {
                key: 'לָמַדְתִּי',
                kind: 'form',
                lemma_key: 'לָמַד',
                gloss_en: 'I learned',
                tense: 'past',
                cell: '1s'
            }
        ], { actor: 'mcp', expectedRev: form.rev });
        assert.equal(edited.ok, true);
        assert.equal(edited.entries[0].verified, 0);
        assert.equal(edited.entries[0].verified_by, null);
        assert.equal(edited.entries[0].gloss_en, 'I learned');
        const upsertAudit = audit.listRecent(20).find((row) => row.action === 'lexicon_upsert');
        assert.ok(upsertAudit);
        const upsertDetail = JSON.parse(upsertAudit.detail);
        assert.equal(upsertDetail.cleared, 1);
        assert.equal(upsertAudit.detail.includes('I learned'), false);

        const stillRefused = service.setVerified(['לָמַדְתִּי'], 1, { actor: 'mcp' });
        assert.equal(stillRefused.error, 'forbidden');
        assert.equal(db.get('SELECT verified FROM lexicon_entries WHERE key = ?', 'לָמַדְתִּי').verified, 0);
    });

    it('ac-5: seeding the three lessons twice keeps the row count and an mcp gloss', () => {
        const first = seed.run({ lessonsDir: lessons });
        assert.ok(first.inserted > 0);
        assert.equal(first.skipped, 0);
        const afterFirst = countEntries(db);

        const second = seed.run({ lessonsDir: lessons });
        assert.equal(second.inserted, 0);
        assert.equal(second.updated, 0);
        assert.equal(countEntries(db), afterFirst);

        const sample = db.get(
            "SELECT key, kind, gloss_en, translit, rev FROM lexicon_entries WHERE source = 'seed' ORDER BY key LIMIT 1"
        );
        assert.ok(sample);
        assert.equal(sample.gloss_en == null || sample.gloss_en.includes('<'), false);
        const changed = service.upsert([
            {
                key: sample.key,
                kind: sample.kind,
                gloss_en: 'kept-by-mcp',
                translit: sample.translit
            }
        ], { actor: 'mcp', expectedRev: sample.rev });
        assert.equal(changed.ok, true);
        assert.equal(changed.entries[0].source, 'mcp');

        const third = seed.run({ lessonsDir: lessons });
        assert.equal(third.inserted, 0);
        assert.equal(countEntries(db), afterFirst);
        const kept = db.get('SELECT gloss_en, source FROM lexicon_entries WHERE key = ?', sample.key);
        assert.equal(kept.source, 'mcp');
        assert.equal(kept.gloss_en, 'kept-by-mcp');

        const tagged = db.get(
            "SELECT COUNT(*) AS n FROM lexicon_entries WHERE source = 'seed' AND (pos IS NOT NULL OR root IS NOT NULL OR binyan IS NOT NULL)"
        );
        assert.equal(tagged.n, 0);
    });

    it('rejects pointed accept spellings, a cell outside its tense, and a batch over 200', () => {
        const pointed = service.upsert([
            { key: 'יֶלֶד', kind: 'lemma', gloss_en: 'boy', accept: ['יֶלֶד'] }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(pointed.ok, false);
        assert.ok(pointed.errors.some((error) => error.code === 'accept_pointed'));

        const cell = service.upsert([
            { key: 'יֶלֶד', kind: 'lemma', gloss_en: 'boy', tense: 'past', cell: 'ms' }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(cell.ok, false);
        assert.ok(cell.errors.some((error) => error.field === 'cell' && error.code === 'cell_tense'));

        const tooMany = service.upsert(
            Array.from({ length: 201 }, (_, index) => ({
                key: 'מילה',
                kind: 'lemma',
                gloss_en: 'word ' + index
            })),
            { actor: 'admin:ada', expectedRev: 0 }
        );
        assert.equal(tooMany.ok, false);
        assert.equal(tooMany.errors[0].code, 'batch_limit');
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', 'יֶלֶד'), undefined);
    });

    it('lists filters and groups a paradigm by tense and cell', () => {
        const page = service.list({ q: 'dog', pos: 'noun', limit: 10 });
        assert.equal(page.ok, true);
        assert.ok(page.entries.some((entry) => entry.key === 'כֶּלֶב'));

        const loaded = service.get('לָמַדְתִּי');
        assert.equal(loaded.entry.kind, 'form');
        assert.equal(loaded.lemma.key, 'לָמַד');
        assert.equal(loaded.paradigm.past['1s'].key, 'לָמַדְתִּי');

        const grouped = service.paradigm('לָמַד');
        assert.equal(grouped.paradigm.past['1s'].cell, '1s');
        assert.equal(service.paradigm('missing-lemma'), null);
        assert.equal(service.get('missing'), null);

        const orphan = service.upsert([
            { key: 'לָמַדְתָּ', kind: 'form', lemma_key: 'אֵין', gloss_en: 'you studied', tense: 'past', cell: '2ms' }
        ], { actor: 'admin:ada', expectedRev: 0 });
        assert.equal(orphan.ok, false);
        assert.ok(orphan.errors.some((error) => error.field === 'lemma_key' && error.code === 'lemma_missing'));
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', 'לָמַדְתָּ'), undefined);
    });
});
