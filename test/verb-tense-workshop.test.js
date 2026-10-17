/**
 * Multi-tense verb workshop on a temporary database.
 * Does not open data/hebrew.sqlite and does not publish.
 * Provenance: INIT-013/SPEC-007
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const BANNED = ['Brian', 'Nelson', 'Aaron', 'Shai', 'Shay'];
const BANNED_POINTING = ['הֶחְלַטֶתִי', 'בַּחָרתִי', 'מְנָּצֵחַ'];

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
        '../scripts/seed-verb-tense-workshop'
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
        seed: require('../scripts/seed-verb-tense-workshop'),
        conjugation: require('../shared/ui/blocks/conjugation-table')
    };
}

describe('verb-tense workshop', { concurrency: false }, () => {
    let tmp;
    let db;
    let service;
    let repo;
    let store;
    let validatePlan;
    let seed;
    let conjugation;
    let checklist;
    let prevData;
    let prevRoot;

    before(() => {
        prevData = process.env.DATA_DIR;
        prevRoot = process.env.HEBREW_ROOT;
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'heb-verb-tense-'));
        checklist = path.join(tmp, 'SPEC-007-forms-checklist.md');
        const mods = freshModules(path.join(tmp, 'data'));
        db = mods.db;
        service = mods.service;
        repo = mods.repo;
        store = mods.store;
        validatePlan = mods.validatePlan;
        seed = mods.seed;
        conjugation = mods.conjugation;
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

    function runSeed() {
        return seed.run({ checklistPath: checklist });
    }

    function lesson() {
        const row = store.loadWorkingCopy(seed.PLAN_ID);
        const plan = JSON.parse(row.document);
        return { row, plan, lesson: plan.units[0].lessons[0] };
    }

    it('ac-1 and ac-2: 1.2.0 plan validates, 88 forms stay unverified', () => {
        const result = runSeed();
        assert.equal(result.ok, true, JSON.stringify(result));
        assert.equal(result.forms, 88);
        assert.equal(result.verified, 0);

        const forms = seed.WORKSHOP_FORMS;
        assert.equal(forms.length, 88);
        const lemmas = new Set(forms.map((entry) => entry.lemma_key));
        assert.equal(lemmas.size, 4);
        const expectedLemmas = seed.WORKSHOP.map((item) => item.key);
        expectedLemmas.forEach((key) => assert.ok(lemmas.has(key), key));

        const byTense = { present: 0, past: 0, future: 0 };
        forms.forEach((entry) => {
            const row = repo.find(entry.key);
            assert.ok(row, entry.key);
            assert.equal(row.kind, 'form');
            assert.equal(row.tense, entry.tense);
            assert.equal(row.cell, entry.cell);
            assert.ok(row.binyan);
            assert.ok(row.root);
            assert.ok(row.root_class);
            assert.ok(Array.isArray(JSON.parse(row.accept_json || '[]')) || Array.isArray(row.accept));
            const accept = row.accept || JSON.parse(row.accept_json || '[]');
            assert.ok(accept.length >= 1, entry.key);
            assert.equal(row.verified, 0);
            assert.equal(row.source, 'admin');
            assert.equal(row.key, row.key.normalize('NFC'));
            byTense[entry.tense] += 1;
        });
        assert.equal(byTense.present, 16);
        assert.equal(byTense.past, 36);
        assert.equal(byTense.future, 36);

        const decide1s = forms.find((e) => e.lemma_key === expectedLemmas[0] && e.tense === 'past' && e.cell === '1s');
        assert.equal(decide1s.key, 'הֶחְלַטְתִּי'.normalize('NFC'));
        const choose1s = forms.find((e) => {
            return e.tense === 'past' && e.cell === '1s' && e.gloss_en === 'I chose';
        });
        assert.equal(choose1s.key, 'בָּחַרְתִּי'.normalize('NFC'));
        const winMs = forms.find((e) => e.tense === 'present' && e.cell === 'ms' && e.gloss_en.startsWith('wins'));
        assert.equal(winMs.key, 'מְנַצֵּחַ'.normalize('NFC'));

        const futureHomographs = forms.filter((e) => e.tense === 'future' && (e.cell === '2ms' || e.cell === '3fs'));
        assert.equal(futureHomographs.length, 8);
        futureHomographs.filter((e) => e.cell === '3fs').forEach((entry) => {
            assert.match(entry.key, /-3fs$/);
        });

        seed.WORKSHOP.forEach((spec) => {
            const row = repo.find(spec.key);
            assert.equal(row.kind, 'lemma');
            assert.equal(row.verified, 0);
            assert.equal(row.binyan, spec.binyan);
            assert.equal(row.root, spec.root);
        });

        const loaded = lesson();
        assert.equal(loaded.row.status, 'draft');
        assert.equal(loaded.row.snapshot, null);
        assert.equal(loaded.plan.schemaVersion, '1.2.0');
        assert.equal(loaded.plan.audience, 'shared');
        assert.equal(loaded.lesson.id, seed.LESSON_ID);
        assert.equal(loaded.lesson.visibility, 'hidden');
        assert.equal(loaded.lesson.recipe, 'verb-workshop');
        const validation = validatePlan(loaded.plan, { lexicon: repo });
        assert.deepEqual(validation.blocks, []);
        assert.ok(validation.warnings.some((item) => item.rule === 'N14'));

        const text = fs.readFileSync(checklist, 'utf8');
        const boxes = text.match(/^- \[[ x]\] \| /gm);
        assert.equal(boxes.length, 88);
        assert.match(text, /present/);
        assert.match(text, /past/);
        assert.match(text, /future/);
        assert.equal((text.match(/^- \[ \]/gm) || []).length, 88);
    });

    it('a second run with no ticks leaves revisions and verified flags alone', () => {
        const before = repo.find(seed.WORKSHOP_FORMS[0].key);
        const planBefore = store.loadWorkingCopy(seed.PLAN_ID).revision;
        const again = runSeed();
        assert.equal(again.ok, true, JSON.stringify(again));
        assert.equal(again.verified, 0);
        const after = repo.find(seed.WORKSHOP_FORMS[0].key);
        assert.equal(after.rev, before.rev);
        assert.equal(after.verified, 0);
        assert.equal(store.loadWorkingCopy(seed.PLAN_ID).revision, planBefore);
    });

    it('ac-3: ticking present, past, and future verifies only those forms', () => {
        const picked = [
            seed.WORKSHOP_FORMS.find((e) => e.tense === 'present' && e.cell === 'ms'),
            seed.WORKSHOP_FORMS.find((e) => e.tense === 'past' && e.cell === '1s'),
            seed.WORKSHOP_FORMS.find((e) => e.tense === 'future' && e.cell === '2ms')
        ];
        assert.equal(picked.length, 3);
        const keep = picked.map((entry) => entry.key);
        let text = fs.readFileSync(checklist, 'utf8');
        keep.forEach((key) => {
            const line = text.split('\n').find((item) => item.endsWith('| ' + key));
            assert.ok(line, key);
            text = text.replace(line, line.replace('- [ ]', '- [x]'));
        });
        fs.writeFileSync(checklist, text);
        const ticked = runSeed();
        assert.equal(ticked.ok, true, JSON.stringify(ticked));
        assert.equal(ticked.verified, 3);
        seed.WORKSHOP_FORMS.forEach((entry) => {
            const row = repo.find(entry.key);
            assert.equal(row.verified, keep.includes(entry.key) ? 1 : 0);
        });

        const drop = keep[0];
        text = fs.readFileSync(checklist, 'utf8');
        const line = text.split('\n').find((item) => item.endsWith('| ' + drop));
        text = text.replace(line, line.replace('- [x]', '- [ ]'));
        fs.writeFileSync(checklist, text);
        const cleared = runSeed();
        assert.equal(cleared.ok, true, JSON.stringify(cleared));
        assert.equal(cleared.verified, 2);
        assert.equal(repo.find(drop).verified, 0);
        assert.equal(repo.find(keep[1]).verified, 1);
        assert.equal(repo.find(keep[2]).verified, 1);
    });

    it('refuses when a target key exists with a source other than admin', () => {
        const victim = seed.WORKSHOP_FORMS[3];
        const before = repo.find(victim.key);
        db.run('UPDATE lexicon_entries SET source = ? WHERE key = ?', 'seed', victim.key);
        const other = seed.WORKSHOP_FORMS[4].key;
        const otherBefore = repo.find(other);
        const refused = runSeed();
        assert.equal(refused.ok, false);
        assert.equal(refused.error, 'foreign_source');
        assert.ok(refused.keys.includes(victim.key));
        const after = repo.find(victim.key);
        assert.equal(after.source, 'seed');
        assert.equal(after.gloss_en, before.gloss_en);
        assert.equal(after.rev, before.rev);
        const otherAfter = repo.find(other);
        assert.equal(otherAfter.rev, otherBefore.rev);
        db.run('UPDATE lexicon_entries SET source = ? WHERE key = ?', 'admin', victim.key);
    });

    it('ac-4: lesson has five stages and drillQueue interleaves verbs and tenses', () => {
        const loaded = lesson();
        const blocks = loaded.lesson.blocks;
        assert.ok(blocks.some((b) => b.type === 'explainer' && b.sections && b.sections[0].table));
        assert.ok(blocks.some((b) => b.type === 'explainer' && (b.sections[0].examples || []).some((ex) => ex.root)));
        assert.ok(blocks.some((b) => b.type === 'explainer' && (b.sections[0].callouts || []).length >= 3));

        const study = blocks.filter((b) => b.type === 'conjugation-table' && b.mode === 'study');
        const drill = blocks.filter((b) => b.type === 'conjugation-table' && b.mode === 'drill');
        assert.ok(study.some((b) => b.tense === 'present'));
        assert.ok(study.some((b) => b.tense === 'past'));
        assert.ok(study.some((b) => b.tense === 'future'));
        assert.ok(drill.length >= 2);
        const drillLemmas = new Set(drill.map((b) => b.lemmaKey));
        const drillTenses = new Set(drill.map((b) => b.tense));
        assert.ok(drillLemmas.size >= 2);
        assert.ok(drillTenses.size >= 2);

        assert.ok(blocks.some((b) => b.type === 'exercise' && b.kind === 'choose'));
        assert.ok(blocks.some((b) => b.type === 'exercise' && b.kind === 'translate'));
        assert.ok(blocks.some((b) => b.type === 'assignment' && b.minSentences === 6));
        assert.ok(blocks.some((b) => b.type === 'personal-question'));

        // Verify enough drill cells so interleaving is observable.
        const verifyKeys = [];
        drill.forEach((block) => {
            (block.cells || []).slice(0, 3).forEach((cell) => {
                verifyKeys.push(cell.formKey);
            });
        });
        assert.ok(verifyKeys.length >= 4);
        const set = service.setVerified(verifyKeys, true, { actor: seed.ACTOR });
        assert.equal(set.ok, true, JSON.stringify(set));

        const forms = Object.create(null);
        verifyKeys.forEach((key) => {
            const row = repo.find(key);
            forms[key] = {
                key: row.key,
                verified: 1,
                root: row.root,
                accept: row.accept || JSON.parse(row.accept_json || '[]')
            };
        });

        let n = 0;
        const rng = () => {
            n += 1;
            return (n % 10) / 10;
        };
        const queue = conjugation.drillQueue(drill, forms, rng);
        assert.ok(queue.length >= 4, 'queue length ' + queue.length);
        const qLemmas = new Set(queue.map((item) => item.lemma));
        const qTenses = new Set(queue.map((item) => item.tense));
        assert.ok(qLemmas.size >= 2, 'lemmas ' + [...qLemmas].join(','));
        assert.ok(qTenses.size >= 2, 'tenses ' + [...qTenses].join(','));

        // Clear verification used only for this drill check; checklist remains source of truth.
        const clear = service.setVerified(verifyKeys, false, { actor: seed.ACTOR });
        assert.equal(clear.ok, true);
        // Restore checklist-driven ticks from earlier ac-3 (2 forms still ticked on disk).
        const restored = runSeed();
        assert.equal(restored.ok, true, JSON.stringify(restored));
        assert.equal(restored.verified, 2);
    });

    it('ac-5: no personal names and no class-doc pointing typos', () => {
        const seedSrc = fs.readFileSync(path.join(root, 'scripts', 'seed-verb-tense-workshop.js'), 'utf8');
        const checklistSrc = fs.readFileSync(checklist, 'utf8');
        const committedChecklist = fs.readFileSync(
            path.join(
                root,
                'sdd',
                'initiatives',
                'INIT-013-responsive-desktop-mobile-redesign',
                'specs',
                'completion-summaries',
                'SPEC-007-forms-checklist.md'
            ),
            'utf8'
        );
        const blobs = [
            seedSrc,
            checklistSrc,
            committedChecklist,
            store.loadWorkingCopy(seed.PLAN_ID).document
        ];
        seed.ENTRIES.forEach((entry) => {
            const row = repo.find(entry.key);
            blobs.push(row.gloss_en || '');
            blobs.push(row.key);
        });
        const haystack = blobs.join('\n');
        BANNED.forEach((needle) => {
            assert.equal(haystack.includes(needle), false, needle);
        });
        // Pointing typos must not appear in seed or checklist sources (or stored keys).
        BANNED_POINTING.forEach((needle) => {
            assert.equal(seedSrc.includes(needle), false, 'seed:' + needle);
            assert.equal(checklistSrc.includes(needle), false, 'checklist:' + needle);
            assert.equal(committedChecklist.includes(needle), false, 'committed:' + needle);
            seed.WORKSHOP_FORMS.forEach((entry) => {
                assert.equal(entry.key.replace(/-3fs$/, '') === needle.normalize('NFC'), false, entry.key);
            });
        });
        assert.ok(seedSrc.includes('הֶחְלַטְתִּי'));
        assert.ok(seedSrc.includes('בָּחַרְתִּי'));
        assert.ok(seedSrc.includes('מְנַצֵּחַ'));
    });
});
