/**
 * Plan revision log (INIT-008/SPEC-004): one row per successful write, written in
 * the same transaction as the write; list and get; names hidden from an AI caller.
 * Temporary database only. No network, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-004
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const zlib = require('node:zlib');

const { planEnv } = require('./helpers/plan-db');

const NEW_VALUE = 'MUST-NOT-APPEAR-IN-SUMMARY-espresso';
const EXTRA = { id: 'extra', type: 'listening', passage: 'תּוֹדָה.' };

describe('plan revision log (INIT-008/SPEC-004)', { concurrency: false }, () => {
    let env;

    before(() => {
        env = planEnv('hebrew-plan-revisions-');
    });

    after(() => {
        env.close();
    });

    beforeEach(() => {
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM audit_log');
    });

    function rows(planId) {
        return env.db.all(
            'SELECT revision, op, actor_kind, actor_name, tool, path, field, summary FROM plan_revisions WHERE plan_id = ? ORDER BY revision',
            planId
        );
    }

    it('ac-1: each of the seven write operations leaves exactly one row, matching the audit rows', () => {
        const { ops, publish, store, PLAN_ID, WORD, LISTEN, LESSON } = env;
        env.seed();
        const ctx = env.human('replace_part');
        const steps = [
            ['set_field', (rev) => ops.setField(store, PLAN_ID, {
                path: WORD, field: 'meaning', value: NEW_VALUE, expectedRevision: rev
            }, env.human('set_field'))],
            ['replace_part', (rev) => ops.replacePart(store, PLAN_ID, {
                path: LISTEN, value: { id: 'listen', type: 'listening', passage: 'תּוֹדָה.' }, expectedRevision: rev
            }, ctx)],
            ['insert_block', (rev) => ops.insertBlock(store, PLAN_ID, {
                path: LESSON, index: 1, block: EXTRA, expectedRevision: rev
            }, env.human('insert_block'))],
            ['move_block', (rev) => ops.moveBlock(store, PLAN_ID, {
                path: LESSON + '/blocks/extra', index: 0, expectedRevision: rev
            }, env.human('move_block'))],
            ['remove_block', (rev) => ops.removeBlock(store, PLAN_ID, {
                path: LESSON + '/blocks/extra', expectedRevision: rev
            }, env.human('remove_block'))],
            ['assign', (rev) => ops.assignPlan(store, PLAN_ID, {
                audience: 'personal', assigneeId: 'example-learner', expectedRevision: rev
            }, env.human('assign'))],
            ['publish', (rev) => publish.publishPlan(store, PLAN_ID, { expectedRevision: rev }, env.human('publish_plan'))]
        ];
        let revision = 1;
        for (const [op, run] of steps) {
            const before = env.count('plan_revisions', PLAN_ID);
            const result = run(revision);
            assert.equal(result.error, undefined, op + ' ' + JSON.stringify(result));
            const added = env.count('plan_revisions', PLAN_ID) - before;
            // The very first write also adds the baseline row.
            assert.equal(added, before === 0 ? 2 : 1, op);
            revision = result.revision;
            assert.equal(revision, store.loadWorkingCopy(PLAN_ID).revision);

            const row = env.revisionsRepo.get(PLAN_ID, revision);
            assert.equal(row.op, op);
            assert.equal(row.actorKind, 'human');
            assert.equal(row.actorName, 'example-admin');
            assert.equal(row.tool, op === 'replace_part' ? 'replace_part' : (op === 'publish' ? 'publish_plan' : op));
            assert.equal(row.document, store.loadWorkingCopy(PLAN_ID).document, op);
        }
        assert.equal(rows(PLAN_ID).filter((r) => r.op !== 'baseline').length, steps.length);
        assert.equal(env.count('audit_log'), steps.length);
        assert.equal(env.count('plan_revisions', PLAN_ID), steps.length + 1);
    });

    it('ac-1: the actor kind and name come from the context, for the AI and for a script', () => {
        const { ops, store, PLAN_ID, WORD } = env;
        env.seed();
        const first = ops.setField(store, PLAN_ID, {
            path: WORD, field: 'meaning', value: 'a', expectedRevision: 1
        }, env.ai('set_field'));
        const second = ops.setField(store, PLAN_ID, {
            path: WORD, field: 'meaning', value: 'b', expectedRevision: first.revision
        }, env.system('seed-script'));
        const all = rows(PLAN_ID);
        assert.deepEqual(all.map((r) => [r.revision, r.op, r.actor_kind, r.actor_name]), [
            [1, 'baseline', 'system', 'baseline'],
            [2, 'set_field', 'ai', 'mcp'],
            [3, 'set_field', 'system', 'seed-script']
        ]);
        assert.equal(second.revision, 3);
    });

    it('ac-2: a plan with no log gets a baseline at its old revision, then one row per edit', () => {
        const { ops, store, PLAN_ID, WORD } = env;
        const seeded = env.seed();
        assert.equal(env.count('plan_revisions', PLAN_ID), 0);
        const one = ops.setField(store, PLAN_ID, {
            path: WORD, field: 'meaning', value: 'first', expectedRevision: 1
        }, env.human('set_field'));
        assert.equal(one.error, undefined);
        let all = rows(PLAN_ID);
        assert.equal(all.length, 2);
        assert.equal(all[0].op, 'baseline');
        assert.equal(all[0].revision, 1);
        assert.equal(all[1].op, 'set_field');
        assert.equal(all[1].revision, 2);
        assert.deepEqual(
            JSON.parse(env.revisionsRepo.get(PLAN_ID, 1).document),
            JSON.parse(seeded.document)
        );

        const two = ops.setField(store, PLAN_ID, {
            path: WORD, field: 'meaning', value: 'second', expectedRevision: 2
        }, env.human('set_field'));
        assert.equal(two.error, undefined);
        all = rows(PLAN_ID);
        assert.equal(all.length, 3);
        assert.equal(all.filter((r) => r.op === 'baseline').length, 1);
    });

    it('ac-3: a refused write adds no revision, audit, or mark row', () => {
        const { ops, store, PLAN_ID, WORD, LISTEN } = env;
        const before = env.seed();
        const attempts = [
            () => ops.setField(store, PLAN_ID, { path: WORD, field: 'meaning', value: 'x', expectedRevision: 99 }, env.human()),
            () => ops.setField(store, PLAN_ID, { path: '', field: 'title', value: '<b>no</b>', expectedRevision: 1 }, env.human()),
            () => ops.setField(store, PLAN_ID, { path: LISTEN, field: 'id', value: 'renamed', expectedRevision: 1 }, env.human()),
            () => ops.setField(store, PLAN_ID, { path: '', field: 'assigneeId', value: 'learner01', expectedRevision: 1 }, env.human()),
            () => ops.replacePart(store, PLAN_ID, {
                path: LISTEN, value: { id: 'other', type: 'listening', passage: 'תּוֹדָה.' }, expectedRevision: 1
            }, env.human())
        ];
        const errors = attempts.map((run) => run().error);
        assert.deepEqual(errors, ['revision_conflict', 'validation', 'id_mismatch', 'assign_required', 'id_mismatch']);
        assert.equal(env.count('plan_revisions'), 0);
        assert.equal(env.count('audit_log'), 0);
        assert.equal(env.count('plan_field_marks'), 0);
        assert.equal(store.loadWorkingCopy(PLAN_ID).document, before.document);
    });

    it('ac-3: a failing revision insert rolls back the working copy, the audit row, and the marks', () => {
        const { ops, publish, store, PLAN_ID, WORD } = env;
        const before = env.seed();
        const original = env.revisionsRepo.append;
        env.revisionsRepo.append = function () { throw new Error('revision insert failed'); };
        try {
            assert.throws(() => ops.setField(store, PLAN_ID, {
                path: WORD, field: 'meaning', value: 'x', expectedRevision: 1
            }, env.human()), /revision insert failed/);
            assert.throws(() => publish.publishPlan(store, PLAN_ID, { expectedRevision: 1 }, env.human()),
                /revision insert failed/);
        } finally {
            env.revisionsRepo.append = original;
        }
        const row = store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.document, before.document);
        assert.equal(row.revision, 1);
        assert.equal(row.snapshot, null);
        assert.equal(env.count('audit_log'), 0);
        assert.equal(env.count('plan_revisions'), 0);
        assert.equal(env.count('plan_field_marks'), 0);

        const ok = ops.setField(store, PLAN_ID, { path: WORD, field: 'meaning', value: 'x', expectedRevision: 1 }, env.human());
        assert.equal(ok.error, undefined);
        assert.equal(env.count('plan_revisions', PLAN_ID), 2);
    });

    it('ac-3: a failing audit insert leaves no revision row behind', () => {
        const { ops, store, PLAN_ID, WORD } = env;
        env.seed();
        const original = env.auditRepo.append;
        env.auditRepo.append = function () { throw new Error('audit insert failed'); };
        try {
            assert.throws(() => ops.setField(store, PLAN_ID, {
                path: WORD, field: 'meaning', value: 'x', expectedRevision: 1
            }, env.human()), /audit insert failed/);
        } finally {
            env.auditRepo.append = original;
        }
        assert.equal(env.count('plan_revisions'), 0);
        assert.equal(store.loadWorkingCopy(PLAN_ID).revision, 1);
    });

    it('ac-4: the log has no UPDATE or DELETE, and a summary never holds the new value', () => {
        const sources = [
            path.join(env.root, 'server', 'db', 'repos', 'plan-revisions.js'),
            path.join(env.root, 'server', 'plans', 'revisions.js')
        ];
        for (const file of sources) {
            const text = fs.readFileSync(file, 'utf8');
            assert.equal(/\bUPDATE\s+plan_revisions\b/i.test(text), false, file);
            assert.equal(/\bDELETE\s+FROM\s+plan_revisions\b/i.test(text), false, file);
            assert.equal(/\bINSERT\s+OR\s+REPLACE\b|\bREPLACE\s+INTO\b/i.test(text), false, file);
            assert.equal(/\bUPDATE\b|\bDELETE\b/.test(text), false, file);
        }

        const { ops, store, PLAN_ID, WORD, LESSON } = env;
        env.seed();
        let result = ops.setField(store, PLAN_ID, { path: WORD, field: 'meaning', value: NEW_VALUE, expectedRevision: 1 }, env.human());
        result = ops.replacePart(store, PLAN_ID, {
            path: WORD, value: Object.assign(env.ops.readPart(store, PLAN_ID, { path: WORD }).part, { meaning: NEW_VALUE + '2' }),
            expectedRevision: result.revision
        }, env.human());
        result = ops.insertBlock(store, PLAN_ID, {
            path: LESSON, block: { id: 'extra', type: 'listening', passage: NEW_VALUE }, expectedRevision: result.revision
        }, env.human());
        for (const row of rows(PLAN_ID)) {
            assert.equal(row.summary.includes('MUST-NOT-APPEAR'), false, row.summary);
            assert.equal(row.summary.length <= 200, true);
        }
        assert.equal(rows(PLAN_ID)[1].summary, 'set_field ' + WORD + '#meaning');
    });

    describe('list and get', () => {
        const PLAN = 'history-plan';

        function build(edits) {
            const created = env.create.createPlan(env.store, { id: PLAN, title: 'History plan' }, env.ai('create_plan'));
            assert.equal(created.error, undefined, JSON.stringify(created));
            let revision = created.revision;
            for (let i = 0; i < edits; i += 1) {
                const ctx = i % 3 === 0 ? env.human('set_field') : env.ai('set_field');
                const path = i % 2 === 0 ? '' : 'units/unit-1';
                const result = env.ops.setField(env.store, PLAN, {
                    path: path, field: 'title', value: 'Title ' + i, expectedRevision: revision
                }, ctx);
                assert.equal(result.error, undefined, JSON.stringify(result));
                revision = result.revision;
            }
            return revision;
        }

        it('ac-5: newest first, with sinceRevision, actorKind, pathPrefix, and limit', () => {
            build(8);
            const all = env.revisions.list(PLAN, {}, env.human());
            assert.equal(all.truncated, false);
            assert.deepEqual(all.revisions.map((r) => r.revision), [9, 8, 7, 6, 5, 4, 3, 2, 1]);
            assert.equal(all.revisions[0].op, 'set_field');
            assert.equal(all.revisions[all.revisions.length - 1].op, 'create');

            const since = env.revisions.list(PLAN, { sinceRevision: 6 }, env.human());
            assert.deepEqual(since.revisions.map((r) => r.revision), [9, 8, 7]);

            const humans = env.revisions.list(PLAN, { actorKind: 'human' }, env.human());
            assert.deepEqual(humans.revisions.map((r) => r.revision), [8, 5, 2]);

            const unit = env.revisions.list(PLAN, { pathPrefix: 'units/unit-1' }, env.human());
            assert.deepEqual(unit.revisions.map((r) => r.revision), [9, 7, 5, 3]);

            const before = env.revisions.list(PLAN, { beforeRevision: 4, limit: 2 }, env.human());
            assert.deepEqual(before.revisions.map((r) => r.revision), [3, 2]);
            assert.equal(before.truncated, true);

            const two = env.revisions.list(PLAN, { limit: 2 }, env.human());
            assert.equal(two.revisions.length, 2);
            assert.equal(two.truncated, true);
        });

        it('ac-5: a _ or % in a prefix is never a wildcard', () => {
            build(4);
            for (const prefix of ['units/unit-_', 'units/%', '%', '_']) {
                const result = env.revisions.list(PLAN, { pathPrefix: prefix }, env.human());
                assert.deepEqual(result.revisions, [], prefix);
            }
        });

        it('ac-5: the default limit is 50 and the cap is 200', () => {
            build(205);
            const fifty = env.revisions.list(PLAN, {}, env.human());
            assert.equal(fifty.revisions.length, 50);
            assert.equal(fifty.truncated, true);
            const capped = env.revisions.list(PLAN, { limit: 5000 }, env.human());
            assert.equal(capped.revisions.length, 200);
            assert.equal(capped.truncated, true);
            assert.equal(capped.revisions[0].revision, 206);
        });

        it('ac-5: a bad filter, and an unknown plan, are errors and not an empty list', () => {
            build(1);
            assert.equal(env.revisions.list(PLAN, { actorKind: 'robot' }, env.human()).error, 'invalid_op');
            assert.equal(env.revisions.list(PLAN, { limit: 0 }, env.human()).error, 'invalid_op');
            assert.equal(env.revisions.list('no-such-plan', {}, env.human()).error, 'not_found');
            assert.throws(() => env.revisions.list(PLAN, {}, undefined), TypeError);
        });

        it('ac-5: get returns the document as saved, not_found for an unknown revision', () => {
            build(3);
            for (let revision = 1; revision <= 4; revision += 1) {
                const got = env.revisions.get(PLAN, revision, env.human());
                assert.equal(got.revision, revision);
                const stored = env.revisionsRepo.get(PLAN, revision);
                assert.deepEqual(got.document, JSON.parse(stored.document));
                assert.equal(got.document.revision, revision);
            }
            assert.equal(env.revisions.get(PLAN, 99, env.human()).error, 'not_found');
            assert.equal(env.revisions.get(PLAN, 0, env.human()).error, 'not_found');
            assert.equal(env.revisions.get('no-such-plan', 1, env.human()).error, 'not_found');
        });

        it('ac-5: a row whose stored hash or bytes no longer match is revision_corrupt', () => {
            build(2);
            env.db.run('UPDATE plan_revisions SET document_sha256 = ? WHERE plan_id = ? AND revision = 2', 'a'.repeat(64), PLAN);
            assert.equal(env.revisions.get(PLAN, 2, env.human()).error, 'revision_corrupt');
            assert.equal(env.revisionsRepo.get(PLAN, 2).error, 'revision_corrupt');
            assert.notEqual(env.revisions.get(PLAN, 3, env.human()).error, 'revision_corrupt');

            const tampered = zlib.gzipSync(Buffer.from('{"changed":true}', 'utf8'));
            env.db.run('UPDATE plan_revisions SET document_gz = ? WHERE plan_id = ? AND revision = 3', tampered, PLAN);
            assert.equal(env.revisions.get(PLAN, 3, env.human()).error, 'revision_corrupt');
            env.db.run('UPDATE plan_revisions SET document_gz = ? WHERE plan_id = ? AND revision = 1', Buffer.from('not gzip'), PLAN);
            assert.equal(env.revisions.get(PLAN, 1, env.human()).error, 'revision_corrupt');
        });

        it('ac-6: an AI caller sees human and no account name; an admin and a script see the name', () => {
            build(4);
            const ai = env.revisions.list(PLAN, {}, env.ai('list_plan_revisions'));
            const aiGot = env.revisions.get(PLAN, 2, env.ai('get_plan_revision'));
            assert.equal(JSON.stringify(ai).includes('example-admin'), false);
            assert.equal(JSON.stringify(aiGot).includes('example-admin'), false);
            const humanRow = ai.revisions.find((r) => r.actorKind === 'human');
            assert.equal(humanRow.actorName, 'human');
            assert.equal(ai.revisions.find((r) => r.actorKind === 'ai').actorName, 'mcp');
            assert.equal(aiGot.actorName, 'human');

            const admin = env.revisions.list(PLAN, {}, env.human());
            assert.equal(admin.revisions.find((r) => r.actorKind === 'human').actorName, 'example-admin');
            assert.equal(env.revisions.get(PLAN, 2, env.human()).actorName, 'example-admin');
            const script = env.revisions.list(PLAN, {}, env.system('report'));
            assert.equal(script.revisions.find((r) => r.actorKind === 'human').actorName, 'example-admin');
        });
    });

    it('ac-7: growth, measured from 100 scripted edits to two fixture plans', () => {
        const samples = path.join(env.root, 'docs', 'decisions', 'ADR-0003-assets', 'samples');
        const report = [];
        for (const name of ['beginner-unit-1.plan.json', 'shai-date-indirect-speech.plan.json']) {
            const plan = JSON.parse(fs.readFileSync(path.join(samples, name), 'utf8'));
            env.seed(plan);
            let revision = plan.revision;
            for (let i = 0; i < 100; i += 1) {
                const result = env.ops.setField(env.store, plan.id, {
                    path: '', field: 'title', value: 'Edit number ' + i, expectedRevision: revision
                }, i % 2 ? env.human('set_field') : env.ai('set_field'));
                assert.equal(result.error, undefined, name + ' ' + JSON.stringify(result));
                revision = result.revision;
            }
            const stats = env.db.get(
                'SELECT COUNT(*) AS n, SUM(length(document_gz)) AS bytes FROM plan_revisions WHERE plan_id = ?',
                plan.id
            );
            assert.equal(stats.n, 101);
            const perRevision = Math.round(stats.bytes / stats.n);
            const compact = JSON.stringify(plan).length;
            report.push(name + ': compact ' + compact + ' bytes, 101 rows (100 edits + baseline) = '
                + stats.bytes + ' bytes gzipped, about ' + perRevision + ' per revision, 1000 revisions about '
                + Math.round(perRevision * 1000 / 1024) + ' KiB');
            assert.equal(perRevision > 0 && perRevision < compact, true);
        }
        console.log('SPEC-004 growth record\n  ' + report.join('\n  '));
    });
});
