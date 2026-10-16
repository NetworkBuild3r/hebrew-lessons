/**
 * Import the four ADR-0003 sample plans on a temporary database.
 * Audio inventory and verify run against a fixture tree, not the operator backup.
 * Provenance: INIT-004/SPEC-007
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const LIVE_LESSONS = path.join(root, 'worksheets');
const HTML = [
    'worksheets/my-morning/index.html',
    'worksheets/anal-sex-vocab/index.html',
    'worksheets/shai-date-indirect-speech/index.html'
];

function git(args) {
    const result = spawnSync('git', args, { cwd: root, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout;
}

function sha256(file) {
    return crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
}

function stampTree(absDir) {
    const rows = [];
    function walk(dir) {
        fs.readdirSync(dir).sort().forEach((name) => {
            const full = path.join(dir, name);
            const stat = fs.statSync(full);
            if (stat.isDirectory()) {
                walk(full);
                return;
            }
            rows.push(path.relative(root, full).replace(/\\/g, '/') + ' ' + sha256(full));
        });
    }
    walk(absDir);
    return rows.join('\n');
}

function inside(child, parent) {
    const rel = path.relative(path.resolve(parent), path.resolve(child));
    return rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel));
}

function collectAudio(value, found) {
    if (Array.isArray(value)) {
        value.forEach((item) => collectAudio(item, found));
        return;
    }
    if (!value || typeof value !== 'object') return;
    if (Object.prototype.hasOwnProperty.call(value, 'audioPath')) {
        found.push(['audioPath', value.audioPath]);
    }
    if (Object.prototype.hasOwnProperty.call(value, 'sentenceAudioPath')) {
        found.push(['sentenceAudioPath', value.sentenceAudioPath]);
    }
    Object.keys(value).forEach((key) => {
        if (key === 'audioPath' || key === 'sentenceAudioPath') return;
        collectAudio(value[key], found);
    });
}

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    process.env.IMPORT_AUDIO_FIXTURE = '1';
    const modules = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/migrations/002-plan-store',
        '../server/db/migrations/003-media-and-ai-planning',
        '../server/plans/store',
        '../server/plans/validate',
        '../scripts/import-lesson-plans'
    ];
    for (const mod of modules) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    const importer = require('../scripts/import-lesson-plans');
    const store = require('../server/plans/store');
    return { db, importer, store };
}

function writeFixture(dir) {
    fs.mkdirSync(path.join(dir, 'sentences'), { recursive: true });
    fs.writeFileSync(path.join(dir, 'clip.mp3'), Buffer.from('fixture-clip-bytes'));
    fs.writeFileSync(path.join(dir, 'sentences', 'line.mp3'), Buffer.from('fixture-line-bytes'));
}

describe('import lesson plans', () => {
    let tmp;
    let fixture;
    let db;
    let importer;
    let store;
    let worksheetsBefore;
    let htmlBefore;
    let diffBefore;
    let inventoryFile;
    const saved = {};

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-import-plans-'));
        fixture = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-import-lessons-'));
        writeFixture(fixture);
        saved.lessons = process.env.LESSONS_DIR;
        saved.data = process.env.DATA_DIR;
        saved.root = process.env.HEBREW_ROOT;
        saved.fixture = process.env.IMPORT_AUDIO_FIXTURE;
        saved.fixtureId = process.env.IMPORT_FIXTURE_ID;
        process.env.LESSONS_DIR = fixture;
        process.env.IMPORT_FIXTURE_ID = 'plans-' + crypto.randomBytes(4).toString('hex');
        worksheetsBefore = stampTree(LIVE_LESSONS);
        htmlBefore = HTML.map((rel) => sha256(path.join(root, rel)));
        diffBefore = git(['diff', '--', 'worksheets']);
        ({ db, importer, store } = freshModules(tmp));
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        fs.rmSync(fixture, { recursive: true, force: true });
        if (process.env.IMPORT_FIXTURE_ID) {
            fs.rmSync(path.join(root, 'data', 'audio-inventory', 'import-fixtures', process.env.IMPORT_FIXTURE_ID), {
                recursive: true,
                force: true
            });
        }
        if (saved.lessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = saved.lessons;
        if (saved.data === undefined) delete process.env.DATA_DIR;
        else process.env.DATA_DIR = saved.data;
        if (saved.root === undefined) delete process.env.HEBREW_ROOT;
        else process.env.HEBREW_ROOT = saved.root;
        if (saved.fixture === undefined) delete process.env.IMPORT_AUDIO_FIXTURE;
        else process.env.IMPORT_AUDIO_FIXTURE = saved.fixture;
        if (saved.fixtureId === undefined) delete process.env.IMPORT_FIXTURE_ID;
        else process.env.IMPORT_FIXTURE_ID = saved.fixtureId;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    it('ac-1: stores the three legacy plans published and the café unit as draft', () => {
        const result = importer.importLessonPlans();
        assert.equal(result.ok, true, JSON.stringify({
            error: result.error,
            blocks: result.blocks,
            verify: result.verify,
            inventory: result.inventory
        }));
        inventoryFile = result.inventory.outPath;
        assert.equal(typeof inventoryFile, 'string');
        assert.equal(fs.existsSync(inventoryFile), true);
        assert.equal(result.plans.length, 4);

        const expected = {
            'my-morning': 'published',
            'anal-sex-vocab': 'published',
            'shai-date-indirect-speech': 'published',
            'beginner-unit-1': 'draft'
        };
        importer.loadSampleTexts().forEach((sample) => {
            const plan = JSON.parse(sample.text);
            const row = store.loadWorkingCopy(plan.id);
            assert.equal(row.status, expected[plan.id]);
            assert.equal(row.document, sample.text);
            assert.equal(row.revision, plan.revision);
            if (expected[plan.id] === 'published') {
                assert.equal(store.loadSnapshot(plan.id), sample.text);
                assert.equal(plan.migration.legacy, true);
            } else {
                assert.equal(row.snapshot, null);
                assert.equal(store.loadSnapshot(plan.id), null);
                assert.equal(plan.migration, undefined);
            }
            plan.units.forEach((unit) => {
                unit.lessons.forEach((lesson) => {
                    const storedLesson = JSON.parse(row.document).units
                        .find((item) => item.id === unit.id).lessons
                        .find((item) => item.id === lesson.id);
                    assert.equal(storedLesson.visibility, lesson.visibility);
                    if (expected[plan.id] === 'published') assert.equal(lesson.visibility, 'everyone');
                    else assert.equal(lesson.visibility, 'hidden');
                });
            });
        });

        const n1 = result.warnings.filter((finding) => finding.rule === 'N1');
        assert.deepEqual(
            n1.map((finding) => finding.planId).sort(),
            ['anal-sex-vocab', 'shai-date-indirect-speech']
        );
        n1.forEach((finding) => assert.equal(finding.severity, 'warn'));
        assert.equal(result.warnings.some((finding) => finding.planId === 'my-morning' && finding.rule === 'N1'), false);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans').n, 4);
    });

    it('ac-2: stored audio strings match the samples and worksheets are untouched', () => {
        importer.loadSampleTexts().forEach((sample) => {
            const fromSample = [];
            const fromStore = [];
            collectAudio(JSON.parse(sample.text), fromSample);
            collectAudio(JSON.parse(store.loadWorkingCopy(JSON.parse(sample.text).id).document), fromStore);
            assert.deepEqual(fromStore, fromSample);
            if (sample.name === 'shai-date-indirect-speech.plan.json'
                || sample.name === 'beginner-unit-1.plan.json') {
                assert.equal(fromSample.length, 0);
                assert.equal(fromStore.length, 0);
            } else {
                assert.ok(fromSample.length > 0);
            }
        });

        const cafe = JSON.parse(importer.loadSampleTexts()
            .find((sample) => sample.name === 'beginner-unit-1.plan.json').text);
        const firstWord = cafe.units[0].lessons[0].blocks[0].words[0];
        assert.equal(Object.prototype.hasOwnProperty.call(firstWord, 'audioPath'), false);
        assert.equal(Object.prototype.hasOwnProperty.call(firstWord, 'sentenceAudioPath'), false);
        const storedCafe = JSON.parse(store.loadWorkingCopy('beginner-unit-1').document);
        const storedWord = storedCafe.units[0].lessons[0].blocks[0].words[0];
        assert.equal(Object.prototype.hasOwnProperty.call(storedWord, 'audioPath'), false);
        assert.equal(JSON.stringify(storedCafe), JSON.stringify(cafe));

        assert.equal(stampTree(LIVE_LESSONS), worksheetsBefore);
        HTML.forEach((rel, index) => {
            assert.equal(sha256(path.join(root, rel)), htmlBefore[index]);
        });
        assert.equal(git(['diff', '--', ...HTML]), '');
        assert.equal(diffBefore.includes('index.html'), false);
        assert.equal(git(['diff', '--', 'worksheets']), diffBefore);
        assert.equal(git(['diff', '--',
            'worksheets/my-morning',
            'worksheets/anal-sex-vocab',
            'worksheets/shai-date-indirect-speech'
        ]), '');
    });

    it('ac-3: audio:verify exits 0 against the fixture inventory, not the live tree', () => {
        const row = store.loadWorkingCopy('my-morning');
        assert.ok(row);
        assert.equal(fs.existsSync(inventoryFile), true, inventoryFile);
        const latest = inventoryFile;
        const payload = JSON.parse(fs.readFileSync(latest, 'utf8'));
        assert.equal(path.resolve(payload.lessonsDir), path.resolve(fixture));
        assert.notEqual(path.resolve(payload.lessonsDir), path.resolve(LIVE_LESSONS));
        assert.ok(payload.files.some((entry) => inside(path.resolve(root, entry.path), fixture)));

        const verify = spawnSync(process.execPath, [
            path.join(root, 'scripts', 'audio-inventory.js'),
            'verify',
            '--against',
            latest
        ], {
            cwd: root,
            encoding: 'utf8',
            env: Object.assign({}, process.env, {
                HEBREW_ROOT: root,
                LESSONS_DIR: fixture,
                DATA_DIR: tmp
            })
        });
        assert.equal(verify.status, 0, verify.stderr || verify.stdout);
        assert.match(verify.stdout, /audio:verify OK/);
        assert.equal(fs.readFileSync(path.join(fixture, 'clip.mp3')).equals(Buffer.from('fixture-clip-bytes')), true);
        assert.equal(fs.readFileSync(path.join(fixture, 'sentences', 'line.mp3')).equals(Buffer.from('fixture-line-bytes')), true);
    });

    it('und-2: a second import fails without a second copy and without writing worksheets', () => {
        const before = db.get('SELECT updated_at FROM plans WHERE id = ?', 'my-morning').updated_at;
        const stamp = stampTree(LIVE_LESSONS);
        const second = importer.importLessonPlans();
        assert.equal(second.ok, false);
        assert.equal(second.error, 'already_imported');
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans').n, 4);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM plans WHERE id = ?', 'my-morning').n, 1);
        assert.equal(db.get('SELECT updated_at FROM plans WHERE id = ?', 'my-morning').updated_at, before);
        assert.equal(stampTree(LIVE_LESSONS), stamp);
    });

    it('refuses a block failure and an inventory failure without inserting rows', () => {
        const blockedDir = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-import-block-'));
        const savedData = process.env.DATA_DIR;
        process.env.DATA_DIR = blockedDir;
        db.close();
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/plans/store')];
        delete require.cache[require.resolve('../scripts/import-lesson-plans')];
        const blockedDb = require('../server/db');
        const blockedImporter = require('../scripts/import-lesson-plans');
        blockedDb.migrate();

        const texts = blockedImporter.loadSampleTexts().map((sample) => {
            if (sample.name !== 'my-morning.plan.json') return sample;
            const plan = JSON.parse(sample.text);
            delete plan.migration.legacy;
            plan.units[0].lessons[0].newWords.push('extra');
            return { name: sample.name, text: JSON.stringify(plan) };
        });
        let verified = false;
        const blocked = blockedImporter.importLessonPlans({
            texts,
            inventory: () => ({ ok: true, outPath: null, count: 0, status: 0 }),
            verify: () => {
                verified = true;
                return { ok: true, status: 0 };
            }
        });
        assert.equal(blocked.ok, false);
        assert.equal(blocked.error, 'validation');
        assert.ok(blocked.blocks.some((finding) => finding.rule === 'N1' && finding.severity === 'block'));
        assert.equal(verified, false);
        assert.equal(blockedDb.get('SELECT COUNT(*) AS n FROM plans').n, 0);

        const missed = blockedImporter.importLessonPlans({
            inventory: () => ({ ok: false, outPath: null, count: 0, status: 1 }),
            verify: () => {
                verified = true;
                return { ok: true, status: 0 };
            }
        });
        assert.equal(missed.ok, false);
        assert.equal(missed.error, 'audio_inventory');
        assert.equal(verified, false);
        assert.equal(blockedDb.get('SELECT COUNT(*) AS n FROM plans').n, 0);

        blockedDb.close();
        process.env.DATA_DIR = savedData;
        fs.rmSync(blockedDir, { recursive: true, force: true });
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/plans/store')];
        delete require.cache[require.resolve('../scripts/import-lesson-plans')];
        ({ db, importer, store } = freshModules(tmp));
    });
});

describe('import cli', () => {
    let tmp;
    let fixture;
    let cliFixtureId;
    const saved = {};

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-import-cli-'));
        fixture = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-import-cli-lessons-'));
        fs.mkdirSync(fixture, { recursive: true });
        fs.writeFileSync(path.join(fixture, 'clip.mp3'), Buffer.from('cli-fixture'));
        saved.lessons = process.env.LESSONS_DIR;
        saved.data = process.env.DATA_DIR;
        saved.root = process.env.HEBREW_ROOT;
        saved.fixture = process.env.IMPORT_AUDIO_FIXTURE;
        saved.fixtureId = process.env.IMPORT_FIXTURE_ID;
        cliFixtureId = 'cli-' + crypto.randomBytes(4).toString('hex');
    });

    after(() => {
        fs.rmSync(tmp, { recursive: true, force: true });
        fs.rmSync(fixture, { recursive: true, force: true });
        if (cliFixtureId) {
            fs.rmSync(path.join(root, 'data', 'audio-inventory', 'import-fixtures', cliFixtureId), {
                recursive: true,
                force: true
            });
        }
        if (saved.lessons === undefined) delete process.env.LESSONS_DIR;
        else process.env.LESSONS_DIR = saved.lessons;
        if (saved.data === undefined) delete process.env.DATA_DIR;
        else process.env.DATA_DIR = saved.data;
        if (saved.root === undefined) delete process.env.HEBREW_ROOT;
        else process.env.HEBREW_ROOT = saved.root;
        if (saved.fixture === undefined) delete process.env.IMPORT_AUDIO_FIXTURE;
        else process.env.IMPORT_AUDIO_FIXTURE = saved.fixture;
        if (saved.fixtureId === undefined) delete process.env.IMPORT_FIXTURE_ID;
        else process.env.IMPORT_FIXTURE_ID = saved.fixtureId;
    });

    it('prints N1 warnings and no plan body, and rejects a path argument', () => {
        const env = Object.assign({}, process.env, {
            HEBREW_ROOT: root,
            DATA_DIR: tmp,
            LESSONS_DIR: fixture,
            IMPORT_AUDIO_FIXTURE: '1',
            IMPORT_FIXTURE_ID: cliFixtureId
        });
        const refused = spawnSync(process.execPath, [
            path.join(root, 'scripts', 'import-lesson-plans.js'),
            '../worksheets/my-morning/index.html'
        ], { cwd: root, encoding: 'utf8', env });
        assert.equal(refused.status, 1);
        assert.match(refused.stderr, /does not take paths/);
        assert.equal(fs.existsSync(path.join(tmp, 'hebrew.sqlite')), false);

        const run = spawnSync(process.execPath, [
            path.join(root, 'scripts', 'import-lesson-plans.js')
        ], { cwd: root, encoding: 'utf8', env });
        assert.equal(run.status, 0, run.stderr || run.stdout);
        assert.match(run.stdout, /warn plan=anal-sex-vocab rule=N1 severity=warn/);
        assert.match(run.stdout, /warn plan=shai-date-indirect-speech rule=N1 severity=warn/);
        assert.match(run.stdout, /stored plan=beginner-unit-1 status=draft/);
        assert.match(run.stdout, /audio:verify ok status=0/);
        assert.equal(run.stdout.includes('אֲנִי קָם בַּבֹּקֶר'), false);
        assert.equal(run.stdout.includes('gets up (m. sg.)'), false);
        assert.equal(run.stderr.includes('schemaVersion'), false);
        assert.equal(path.resolve(env.LESSONS_DIR), path.resolve(fixture));
        assert.notEqual(path.resolve(env.LESSONS_DIR), path.resolve(LIVE_LESSONS));
    });
});
