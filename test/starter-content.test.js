/**
 * Draft verb guide and verb workshop on a temporary database.
 * Does not open data/hebrew.sqlite and does not publish.
 * Provenance: INIT-006/SPEC-018
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const BANNED = ['Brian', 'Nelson', 'Shai', 'Shay', '\u05E8\u05B7\u05E2\u05B0\u05D9\u05D5\u05B9\u05DF'];

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/audit',
        '../server/db/repos/lexicon',
        '../server/vocab-index',
        '../server/lexicon/validate',
        '../server/lexicon/service',
        '../server/plans/store',
        '../server/plans/validate',
        '../server/plans/ops',
        '../scripts/seed-starter-content'
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
        service: require('../server/lexicon/service'),
        repo: require('../server/db/repos/lexicon'),
        store: require('../server/plans/store'),
        validatePlan: require('../server/plans/validate').validatePlan,
        starter: require('../scripts/seed-starter-content')
    };
}

describe('starter verb content', { concurrency: false }, () => {
    let tmp;
    let db;
    let service;
    let repo;
    let store;
    let validatePlan;
    let starter;
    let checklist;
    let prevData;
    let prevRoot;

    before(() => {
        prevData = process.env.DATA_DIR;
        prevRoot = process.env.HEBREW_ROOT;
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-starter-'));
        checklist = path.join(tmp, 'SPEC-018-forms-checklist.md');
        const mods = freshModules(path.join(tmp, 'data'));
        db = mods.db;
        service = mods.service;
        repo = mods.repo;
        store = mods.store;
        validatePlan = mods.validatePlan;
        starter = mods.starter;
        db.open();
        db.migrate();
    });

    after(() => {
        if (db) db.close();
        if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
        if (prevData === undefined) delete process.env.DATA_DIR;
        else process.env.DATA_DIR = prevData;
        if (prevRoot === undefined) delete process.env.HEBREW_ROOT;
        else process.env.HEBREW_ROOT = prevRoot;
    });

    function seed() {
        return starter.run({ checklistPath: checklist });
    }

    function lesson(planId) {
        const row = store.loadWorkingCopy(planId);
        const plan = JSON.parse(row.document);
        return { row, plan, lesson: plan.units[0].lessons[0] };
    }

    it('ac-1 and ac-2: drafts validate and 52 forms stay unverified', () => {
        const result = seed();
        assert.equal(result.ok, true, JSON.stringify(result));
        assert.equal(result.forms, 52);
        assert.equal(result.verified, 0);

        const forms = starter.WORKSHOP_FORMS;
        assert.equal(forms.length, 52);
        const lemmas = new Set(forms.map((entry) => entry.lemma_key));
        assert.equal(lemmas.size, 4);
        forms.forEach((entry) => {
            const row = repo.find(entry.key);
            assert.ok(row, entry.cell);
            assert.equal(row.kind, 'form');
            assert.equal(row.tense, entry.tense);
            assert.equal(row.cell, entry.cell);
            assert.ok(row.binyan);
            assert.ok(row.root_class);
            assert.equal(row.verified, 0);
            assert.equal(row.source, 'admin');
            assert.equal(row.key, row.key.normalize('NFC'));
            assert.equal(row.lemma_key, row.lemma_key.normalize('NFC'));
        });

        const answer = starter.WORKSHOP.find((item) => item.slug === 'answer');
        const feel = starter.WORKSHOP.find((item) => item.slug === 'feel');
        assert.equal(repo.find(answer.key).root_class, 'lamed-he');
        assert.equal(repo.find(answer.key).binyan, 'paal');
        assert.equal(repo.find(feel.key).binyan, 'hifil');
        starter.GUIDE_LEMMAS.forEach((lemma) => {
            const row = repo.find(lemma.key);
            assert.equal(row.kind, 'lemma');
            assert.equal(row.verified, 0);
            assert.equal(repo.listByLemma(lemma.key).filter((item) => item.kind === 'form').length, 0);
        });

        ['guides', 'verbs'].forEach((id) => {
            const loaded = lesson(id);
            assert.equal(loaded.row.status, 'draft');
            assert.equal(loaded.row.snapshot, null);
            assert.equal(loaded.plan.schemaVersion, '1.1.0');
            assert.equal(loaded.plan.audience, 'shared');
            assert.equal(loaded.lesson.visibility, 'hidden');
            assert.match(loaded.plan.subtitle, /Draft/);
            assert.match(loaded.lesson.subtitle, /not verified/);
            const validation = validatePlan(loaded.plan, { lexicon: repo });
            assert.deepEqual(validation.blocks, []);
        });

        const guide = lesson('guides');
        const verbs = lesson('verbs');
        assert.equal(guide.lesson.recipe, 'guide');
        assert.equal(guide.lesson.id, 'decoding-hebrew-verbs');
        assert.ok(guide.lesson.blocks.every((block) => block.type === 'explainer'));
        assert.equal(verbs.lesson.recipe, 'verb-workshop');
        assert.equal(verbs.lesson.id, 'verbs-think-feel-ask-answer');
        const study = verbs.lesson.blocks.filter((block) => {
            return block.type === 'conjugation-table' && block.mode === 'study';
        });
        const drill = verbs.lesson.blocks.filter((block) => {
            return block.type === 'conjugation-table' && block.mode === 'drill';
        });
        assert.equal(study.length, 8);
        assert.equal(drill.length, 1);
        assert.equal(verbs.lesson.blocks.filter((block) => block.type === 'personal-question').length, 1);
        starter.WORKSHOP.forEach((spec) => {
            const present = study.find((block) => block.lemmaKey === spec.key && block.tense === 'present');
            assert.equal(present.sentences.length, 1);
            assert.equal(present.questions.length, 2);
        });
        assert.ok(validatePlan(verbs.plan, { lexicon: repo }).warnings.some((item) => item.rule === 'N14'));

        const text = fs.readFileSync(checklist, 'utf8');
        const boxes = text.match(/^- \[[ x]\] \| /gm);
        assert.equal(boxes.length, 52);
        assert.match(text, /Draft/);
        assert.match(text, /not verified/);
    });

    it('a second run with no ticks leaves revisions and verified flags alone', () => {
        const before = repo.find(starter.WORKSHOP_FORMS[0].key);
        const planBefore = store.loadWorkingCopy('verbs').revision;
        const again = seed();
        assert.equal(again.ok, true, JSON.stringify(again));
        assert.equal(again.verified, 0);
        const after = repo.find(starter.WORKSHOP_FORMS[0].key);
        assert.equal(after.rev, before.rev);
        assert.equal(after.verified, 0);
        assert.equal(store.loadWorkingCopy('verbs').revision, planBefore);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n, starter.ENTRIES.length);
    });

    it('ac-3: a second run verifies only ticked forms', () => {
        const picked = starter.WORKSHOP_FORMS.filter((entry) => entry.cell === 'ms' && entry.tense === 'present');
        assert.equal(picked.length, 4);
        const keep = picked.slice(0, 2).map((entry) => entry.key);
        let text = fs.readFileSync(checklist, 'utf8');
        keep.forEach((key) => {
            const line = text.split('\n').find((item) => item.endsWith('| ' + key));
            assert.ok(line, key);
            text = text.replace(line, line.replace('- [ ]', '- [x]'));
        });
        fs.writeFileSync(checklist, text);
        const ticked = seed();
        assert.equal(ticked.ok, true, JSON.stringify(ticked));
        assert.equal(ticked.verified, 2);
        starter.WORKSHOP_FORMS.forEach((entry) => {
            const row = repo.find(entry.key);
            assert.equal(row.verified, keep.includes(entry.key) ? 1 : 0);
        });
        starter.GUIDE_LEMMAS.forEach((lemma) => {
            assert.equal(repo.find(lemma.key).verified, 0);
        });

        const drop = keep[0];
        text = fs.readFileSync(checklist, 'utf8');
        const line = text.split('\n').find((item) => item.endsWith('| ' + drop));
        text = text.replace(line, line.replace('- [x]', '- [ ]'));
        fs.writeFileSync(checklist, text);
        const cleared = seed();
        assert.equal(cleared.ok, true, JSON.stringify(cleared));
        assert.equal(cleared.verified, 1);
        assert.equal(repo.find(drop).verified, 0);
        assert.equal(repo.find(keep[1]).verified, 1);
    });

    it('refuses when a target key exists with a source other than admin', () => {
        const victim = starter.WORKSHOP_FORMS[3];
        const before = repo.find(victim.key);
        db.run('UPDATE lexicon_entries SET source = ? WHERE key = ?', 'seed', victim.key);
        const other = starter.WORKSHOP_FORMS[4].key;
        const otherBefore = repo.find(other);
        const refused = seed();
        assert.equal(refused.ok, false);
        assert.equal(refused.error, 'foreign_source');
        assert.ok(refused.keys.includes(victim.key));
        const after = repo.find(victim.key);
        assert.equal(after.source, 'seed');
        assert.equal(after.gloss_en, before.gloss_en);
        assert.equal(after.rev, before.rev);
        const otherAfter = repo.find(other);
        assert.equal(otherAfter.rev, otherBefore.rev);
        assert.equal(otherAfter.verified, otherBefore.verified);
    });

    it('ac-4: stored content has no name and no copied class line', () => {
        const blobs = [
            fs.readFileSync(path.join(root, 'scripts', 'seed-starter-content.js'), 'utf8'),
            fs.readFileSync(checklist, 'utf8')
        ];
        ['guides', 'verbs'].forEach((id) => {
            blobs.push(store.loadWorkingCopy(id).document);
        });
        starter.ENTRIES.forEach((entry) => {
            const row = repo.find(entry.key);
            blobs.push(row.gloss_en || '');
            blobs.push(row.translit || '');
        });
        const haystack = blobs.join('\n');
        BANNED.forEach((needle) => {
            assert.equal(haystack.includes(needle), false, needle);
        });
        assert.equal(/(^|[^\u05D0-\u05EA])\u05E9\u05D9([^\u05D0-\u05EA]|$)/.test(haystack), false);
    });
});
