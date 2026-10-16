/**
 * Future tense on conjugation-table and lexicon cells (ADR-0014, INIT-012/SPEC-003).
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { validatePlan, usesV120 } = require('../server/plans/validate');
const { withCtx, testContext } = require('./helpers/plan-ctx');

const ROOT = path.resolve(__dirname, '..');
const EXAMPLES = JSON.parse(fs.readFileSync(
    path.join(ROOT, 'docs', 'decisions', 'ADR-0014-assets', 'exercise-examples.json'), 'utf8'
));
const FUTURE_TABLE = EXAMPLES.blocks.find((block) => block.type === 'conjugation-table' && block.tense === 'future');

function shell(blocks, version) {
    return {
        schemaVersion: version || '1.2.0',
        id: 'test-future-plan',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Future test',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-1',
            title: 'Unit',
            theme: 'test',
            lessons: [{
                id: 'lesson-1',
                title: 'Lesson',
                level: 'basic',
                visibility: 'hidden',
                recipe: 'verb-workshop',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks: blocks
            }]
        }]
    };
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function futureTableForLexicon() {
    const table = clone(FUTURE_TABLE);
    const seen = new Set();
    table.cells = table.cells.map((cell) => {
        const row = Object.assign({}, cell);
        if (seen.has(row.formKey)) row.formKey = row.formKey + '.' + row.cell;
        seen.add(row.formKey);
        return row;
    });
    return table;
}

function futureLexicon(table) {
    const source = table || futureTableForLexicon();
    const rows = source.cells.map((cell) => ({
        key: cell.formKey,
        kind: 'form',
        lemma_key: source.lemmaKey,
        tense: 'future',
        cell: cell.cell,
        verified: 1
    }));
    rows.push({
        key: 'דיבר',
        kind: 'form',
        lemma_key: source.lemmaKey,
        tense: 'past',
        cell: '3ms',
        verified: 1
    });
    const map = new Map(rows.map((row) => [row.key, row]));
    return { find: (key) => map.get(key) || null };
}

describe('INIT-012/SPEC-003: future conjugation-table', () => {
    it('ac-3: future table with nine lexicon forms validates', () => {
        const table = futureTableForLexicon();
        const plan = shell([table]);
        const result = validatePlan(plan, { lexicon: futureLexicon(table) });
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
    });

    it('N13: formKey names no stored form when a cell lacks a future lexicon row', () => {
        const table = futureTableForLexicon();
        const lexicon = futureLexicon(table);
        const missingKey = 'לאקיים';
        table.cells[0] = { cell: '1s', formKey: missingKey };
        assert.equal(lexicon.find(missingKey), null);
        const result = validatePlan(shell([table]), { lexicon });
        assert.equal(result.ok, false);
        assert.ok(result.blocks.some((item) => item.rule === 'N13'
            && item.message === 'formKey names no stored form'));
    });

    it('N13 passes when two future forms exist for the same cell and the table uses either', () => {
        const table = futureTableForLexicon();
        const primary1s = table.cells[0].formKey;
        const alternate1s = primary1s + '.alt';
        const rows = table.cells.map((cell) => ({
            key: cell.formKey,
            kind: 'form',
            lemma_key: table.lemmaKey,
            tense: 'future',
            cell: cell.cell,
            verified: 1
        }));
        rows.push({
            key: alternate1s,
            kind: 'form',
            lemma_key: table.lemmaKey,
            tense: 'future',
            cell: '1s',
            verified: 1
        });
        rows.push({
            key: 'דיבר',
            kind: 'form',
            lemma_key: table.lemmaKey,
            tense: 'past',
            cell: '3ms',
            verified: 1
        });
        const map = new Map(rows.map((row) => [row.key, row]));
        const lexicon = { find: (key) => map.get(key) || null };

        for (const formKey of [primary1s, alternate1s]) {
            const planTable = clone(table);
            planTable.cells[0] = { cell: '1s', formKey };
            const result = validatePlan(shell([planTable]), { lexicon });
            assert.equal(result.ok, true, formKey + ': ' + JSON.stringify(result.blocks));
            assert.ok(!result.blocks.some((item) => item.rule === 'N13'));
        }
    });

    it('ac-4: a past form in a future cell is refused by N13', () => {
        const table = futureTableForLexicon();
        const plan = shell([table]);
        plan.units[0].lessons[0].blocks[0].cells[3] = { cell: '3ms', formKey: 'דיבר' };
        const result = validatePlan(plan, { lexicon: futureLexicon(table) });
        assert.equal(result.ok, false);
        assert.ok(result.blocks.some((item) => item.rule === 'N13'
            && item.message === 'formKey names a form of another tense'));
    });

    it('ac-5: unverified future form validates with N14 warning', () => {
        const table = futureTableForLexicon();
        const lexicon = futureLexicon(table);
        const firstKey = table.cells[0].formKey;
        const base = lexicon.find(firstKey);
        lexicon.find = (key) => {
            const row = futureLexicon(table).find(key);
            if (!row) return null;
            if (key === firstKey) return Object.assign({}, base, { verified: 0 });
            return row;
        };
        const result = validatePlan(shell([table]), { lexicon });
        assert.equal(result.ok, true);
        assert.ok(result.warnings.some((item) => item.rule === 'N14'));
    });

    it('ac-6: imperative tense is refused on conjugation-table', () => {
        const plan = shell([clone(FUTURE_TABLE)]);
        plan.units[0].lessons[0].blocks[0].tense = 'imperative';
        assert.equal(validatePlan(plan).ok, false);
    });

    it('future cell ms maps to N13 cellTense in 1.2.0', () => {
        const table = futureTableForLexicon();
        const plan = shell([table]);
        plan.units[0].lessons[0].blocks[0].cells[0] = { cell: 'ms', formKey: table.cells[0].formKey };
        const result = validatePlan(plan, { lexicon: futureLexicon(table) });
        assert.ok(result.blocks.some((item) => item.rule === 'N13'
            && item.message === 'this cell does not belong to the table tense'));
    });

    it('ac-1 guard: future tense in 1.1.0 is refused', () => {
        const plan = shell([clone(FUTURE_TABLE)], '1.1.0');
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        assert.ok(result.blocks.some((item) => item.message === 'future tense needs schemaVersion 1.2.0'));
    });

    it('usesV120 is true for future conjugation-table', () => {
        assert.equal(usesV120(shell([clone(FUTURE_TABLE)])), true);
    });
});

describe('INIT-012/SPEC-003: ops raises on future table', { concurrency: false }, () => {
    let tmp;
    let db;
    let store;
    let ops;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-plan-future-'));
        process.env.LESSONS_DIR = path.join(tmp, 'lessons');
        fs.mkdirSync(process.env.LESSONS_DIR);
        process.env.DATA_DIR = path.join(tmp, 'data');
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/plans/store')];
        delete require.cache[require.resolve('../server/plans/ops')];
        db = require('../server/db');
        store = require('../server/plans/store');
        ops = withCtx(require('../server/plans/ops'));
        db.open();
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete process.env.LESSONS_DIR;
    });

    it('ac-9: insertBlock of future table into 1.1.0 plan sets schemaVersion 1.2.0', () => {
        const before = JSON.parse(fs.readFileSync(
            path.join(__dirname, 'fixtures', 'plans-1.1', 'personal-question.plan.json'), 'utf8'
        ));
        assert.equal(before.schemaVersion, '1.1.0');
        store.saveWorkingCopy({
            id: before.id,
            document: JSON.stringify(before),
            revision: before.revision,
            status: before.status,
            audience: before.audience,
            assigneeId: before.assigneeId
        });
        const block = clone(FUTURE_TABLE);
        const result = ops.insertBlock(store, before.id, {
            path: 'units/family/lessons/my-family',
            block,
            expectedRevision: 1
        });
        assert.equal(result.error, undefined, JSON.stringify(result));
        assert.equal(result.plan.schemaVersion, '1.2.0');
    });
});

describe('INIT-012/SPEC-003: lexicon future cells', () => {
    let tmp;
    let db;
    let service;
    let validate;

    function freshModules(dataDirPath) {
        process.env.HEBREW_ROOT = ROOT;
        process.env.DATA_DIR = dataDirPath;
        const names = [
            '../server/paths',
            '../server/db',
            '../server/db/repos/concepts',
            '../server/db/repos/audit',
            '../server/db/repos/lexicon',
            '../server/lexicon/validate',
            '../server/lexicon/service'
        ];
        const migrationsDir = path.join(ROOT, 'server', 'db', 'migrations');
        for (const file of fs.readdirSync(migrationsDir)) {
            if (/^\d{3,}-.+\.js$/.test(file)) names.push('../server/db/migrations/' + file);
        }
        for (const mod of names) {
            try { delete require.cache[require.resolve(mod)]; } catch (_) { /* skip */ }
        }
        db = require('../server/db');
        db.close();
        return {
            db,
            service: require('../server/lexicon/service'),
            validate: require('../server/lexicon/validate')
        };
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-future-lex-'));
        ({ db, service, validate } = freshModules(path.join(tmp, 'data')));
        db.open();
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
    });

    it('ac-7: listGrammarConcepts returns future cells; upsert accepts 3ms and refuses ms', () => {
        const keys = service.listGrammarConcepts().map((row) => row.key);
        for (const cell of ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p']) {
            assert.ok(keys.includes('cell:future.' + cell), cell);
        }

        assert.equal(service.upsert([
            { key: 'לדבר', kind: 'lemma', gloss_en: 'to speak', pos: 'verb' }
        ], { actor: 'mcp', expectedRev: 0 }).ok, true);

        const ok = service.upsert([
            {
                key: 'ידבר',
                kind: 'form',
                lemma_key: 'לדבר',
                gloss_en: 'he will speak',
                tense: 'future',
                cell: '3ms'
            }
        ], { actor: 'mcp', expectedRev: 0 });
        assert.equal(ok.ok, true);

        const bad = validate.validateEntries([{
            key: 'x',
            kind: 'form',
            lemma_key: 'לדבר',
            gloss_en: 'bad',
            tense: 'future',
            cell: 'ms'
        }], { tableLemmas: new Set(['לדבר']) });
        assert.equal(bad.ok, false);
        assert.ok(bad.errors.some((err) => err.field === 'cell'));
    });

    it('ac-8: MCP upsert stores verified 0 when verified true is sent', () => {
        const result = service.upsert([
            {
                key: 'תדבר',
                kind: 'form',
                lemma_key: 'לדבר',
                gloss_en: 'you will speak',
                tense: 'future',
                cell: '2ms',
                verified: true
            }
        ], { actor: 'mcp', expectedRev: 0 });
        assert.equal(result.ok, true);
        assert.equal(result.entries[0].verified, 0);
        assert.equal(db.get('SELECT verified FROM lexicon_entries WHERE key = ?', 'תדבר').verified, 0);
    });
});

describe('INIT-012/SPEC-003: ac-10 present and past fixtures unchanged', () => {
    const LEXICON_ENTRIES = JSON.parse(fs.readFileSync(
        path.join(__dirname, 'fixtures', 'plans-1.1', 'lexicon.json'), 'utf8'
    )).entries;

    function memoryLexicon(entries) {
        const rows = new Map(entries.map((entry) => [entry.key, Object.assign({}, entry, {
            verified: entry.verified ? 1 : 0
        })]));
        return { find: (key) => rows.get(key) || null };
    }

    it('conjugation-table fixture still validates with the 1.1 lexicon fixture', () => {
        const fixture = JSON.parse(fs.readFileSync(
            path.join(__dirname, 'fixtures', 'plans-1.1', 'conjugation-table.plan.json'), 'utf8'
        ));
        const result = validatePlan(fixture, { lexicon: memoryLexicon(LEXICON_ENTRIES) });
        assert.equal(fixture.schemaVersion, '1.1.0');
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
    });
});
