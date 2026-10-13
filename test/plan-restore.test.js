/**
 * One-step restore of a plan revision, part, or field (INIT-008/SPEC-005).
 * Temporary database only. No network, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-005
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');

const { planEnv, samplePlan, PLAN_ID, LESSON, WORD, LISTEN } = require('./helpers/plan-db');

describe('plan restore (INIT-008/SPEC-005)', { concurrency: false }, () => {
    let env;

    before(() => {
        env = planEnv('hebrew-plan-restore-');
    });

    after(() => {
        env.close();
    });

    beforeEach(() => {
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM audit_log');
    });

    function working(id) {
        return JSON.parse(env.store.loadWorkingCopy(id || PLAN_ID).document);
    }

    function logRows(id) {
        return env.db.all(
            'SELECT revision, op, actor_kind, actor_name, summary, document_sha256 FROM plan_revisions WHERE plan_id = ? ORDER BY revision',
            id || PLAN_ID
        );
    }

    function wordOf(plan) {
        return plan.units[0].lessons[0].blocks[0].words.find((w) => w.id === '2');
    }

    /** AI sets the meaning (rev 2), a person changes it (rev 3). */
    function aiThenHuman(id) {
        const plan = id || PLAN_ID;
        env.seed(samplePlan((p) => { p.id = plan; }));
        const ai = env.ops.setField(env.store, plan, {
            path: WORD, field: 'meaning', value: 'ai-value', expectedRevision: 1
        }, env.ai('set_field'));
        const human = env.ops.setField(env.store, plan, {
            path: WORD, field: 'meaning', value: 'human-value', expectedRevision: ai.revision
        }, env.human('set_field'));
        assert.deepEqual([ai.revision, human.revision], [2, 3]);
        return plan;
    }

    for (const kind of ['ai', 'human']) {
        it('ac-4: a field restore from the AI revision works for the ' + kind + ' actor: one new revision, nothing else changes', () => {
            const id = aiThenHuman('restore-by-' + kind);
            const before = working(id);
            const ctx = kind === 'ai' ? env.ai('restore_plan_revision') : env.human('restore_plan_revision');
            const logBefore = logRows(id);

            const result = env.revisions.restore(env.store, id, {
                fromRevision: 2, path: WORD, field: 'meaning', expectedRevision: 3
            }, ctx);
            assert.equal(result.error, undefined, JSON.stringify(result));
            assert.equal(result.revision, 4);
            assert.equal(result.restoredFrom, 2);
            assert.equal(result.status, 'draft');

            const now = working(id);
            assert.equal(wordOf(now).meaning, 'ai-value');
            const expected = JSON.parse(JSON.stringify(before));
            wordOf(expected).meaning = 'ai-value';
            expected.revision = 4;
            assert.deepEqual(now, expected);

            const log = logRows(id);
            assert.equal(log.length, logBefore.length + 1);
            assert.deepEqual(log.slice(0, logBefore.length), logBefore, 'history rows are untouched');
            const last = log[log.length - 1];
            assert.equal(last.op, 'restore');
            assert.equal(last.revision, 4);
            assert.equal(last.actor_kind, kind);
            assert.equal(last.summary, 'restore ' + WORD + '#meaning from r2');
            assert.deepEqual(env.revisions.list(id, {}, env.human()).revisions.map((r) => r.op),
                ['restore', 'set_field', 'set_field', 'baseline']);
            assert.equal(env.db.get(
                "SELECT COUNT(*) AS n FROM audit_log WHERE action = 'plan.restore' AND target = ?", id
            ).n, 1);
        });
    }

    it('und-1: a restore is itself in history and can be restored', () => {
        const id = aiThenHuman();
        const first = env.revisions.restore(env.store, id, { fromRevision: 2, path: WORD, field: 'meaning', expectedRevision: 3 }, env.ai());
        assert.equal(first.revision, 4);
        const back = env.revisions.restore(env.store, id, { fromRevision: 3, path: WORD, field: 'meaning', expectedRevision: 4 }, env.human());
        assert.equal(back.revision, 5);
        assert.equal(wordOf(working()).meaning, 'human-value');
        assert.equal(logRows().length, 5);
    });

    it('a field that did not exist then is removed now', () => {
        env.seed(samplePlan((p) => { delete p.subtitle; }));
        const set = env.ops.setField(env.store, PLAN_ID, { path: '', field: 'subtitle', value: 'Added later', expectedRevision: 1 }, env.ai());
        assert.equal(working().subtitle, 'Added later');
        const result = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: '', field: 'subtitle', expectedRevision: set.revision
        }, env.human());
        assert.equal(result.error, undefined, JSON.stringify(result));
        assert.equal(Object.prototype.hasOwnProperty.call(working(), 'subtitle'), false);
        assert.equal(logRows().pop().summary, 'restore (plan)#subtitle from r1');
    });

    it('a part restore puts the node back in place', () => {
        env.seed();
        const a = env.ops.setField(env.store, PLAN_ID, { path: WORD, field: 'meaning', value: 'one', expectedRevision: 1 }, env.ai());
        const b = env.ops.setField(env.store, PLAN_ID, { path: WORD, field: 'sentence', value: 'something else', expectedRevision: a.revision }, env.human());
        const result = env.revisions.restore(env.store, PLAN_ID, { fromRevision: 1, path: WORD, expectedRevision: b.revision }, env.ai());
        assert.equal(result.error, undefined, JSON.stringify(result));
        const original = wordOf(samplePlan());
        assert.deepEqual(wordOf(working()), original);
        assert.equal(logRows().pop().summary, 'restore ' + WORD + ' from r1');
    });

    it('ac-5: a whole restore keeps audience and assigneeId, sets status to draft, and never lowers schemaVersion', () => {
        env.seed();
        const ctx = env.human('restore_plan_revision');
        const title = env.ops.setField(env.store, PLAN_ID, { path: '', field: 'title', value: 'Title two', expectedRevision: 1 }, ctx);
        const assigned = env.ops.assignPlan(env.store, PLAN_ID, {
            audience: 'personal', assigneeId: 'example-learner', expectedRevision: title.revision
        }, ctx);
        const raised = env.ops.insertBlock(env.store, PLAN_ID, {
            path: LESSON, block: {
                id: 'explain', type: 'explainer', title: 'A note',
                sections: [{ id: 'sec1', heading: 'Note', paragraphs: ['A short note.'] }]
            }, expectedRevision: assigned.revision
        }, ctx);
        assert.equal(raised.error, undefined, JSON.stringify(raised));
        assert.equal(working().schemaVersion, '1.1.0');
        const published = env.publish.publishPlan(env.store, PLAN_ID, { expectedRevision: raised.revision }, ctx);
        assert.equal(published.error, undefined, JSON.stringify(published.blocks));
        assert.equal(published.status, 'published');
        const snapshotBefore = env.store.loadSnapshot(PLAN_ID);

        const result = env.revisions.restore(env.store, PLAN_ID, { fromRevision: 1, expectedRevision: published.revision }, env.ai());
        assert.equal(result.error, undefined, JSON.stringify(result));
        assert.equal(result.revision, published.revision + 1);
        assert.equal(result.restoredFrom, 1);
        const now = working();
        assert.equal(now.title, samplePlan().title);
        assert.equal(now.audience, 'personal');
        assert.equal(now.assigneeId, 'example-learner');
        assert.equal(now.status, 'draft');
        assert.equal(now.schemaVersion, '1.1.0');
        assert.equal(now.revision, published.revision + 1);
        assert.equal(now.id, PLAN_ID);
        assert.equal(now.units[0].lessons[0].blocks.some((b) => b.id === 'explain'), false);
        const row = env.store.loadWorkingCopy(PLAN_ID);
        assert.equal(row.audience, 'personal');
        assert.equal(row.assigneeId, 'example-learner');
        assert.equal(row.status, 'draft');
        assert.equal(row.snapshot, snapshotBefore, 'learners keep the last published snapshot');
    });

    it('ac-5: restoring a 1.0.0 revision onto a plan that is already 1.1.0 keeps 1.1.0', () => {
        env.seed();
        const added = env.ops.insertBlock(env.store, PLAN_ID, {
            path: LESSON, block: {
                id: 'explain', type: 'explainer', title: 'A note',
                sections: [{ id: 'sec1', heading: 'Note', paragraphs: ['A short note.'] }]
            }, expectedRevision: 1
        }, env.human());
        assert.equal(working().schemaVersion, '1.1.0');
        assert.equal(env.revisions.get(PLAN_ID, 1, env.human()).document.schemaVersion, '1.0.0');
        const result = env.revisions.restore(env.store, PLAN_ID, { fromRevision: 1, expectedRevision: added.revision }, env.human());
        assert.equal(result.error, undefined);
        assert.equal(working().schemaVersion, '1.1.0');
    });

    it('ac-6: a removed block returns at its former index; the index is clamped to the list', () => {
        env.seed();
        const ids = () => working().units[0].lessons[0].blocks.map((b) => b.id);
        assert.deepEqual(ids(), ['introduction', 'story', 'listen', 'dialogue']);

        const removed = env.ops.removeBlock(env.store, PLAN_ID, { path: LISTEN, expectedRevision: 1 }, env.human());
        assert.deepEqual(ids(), ['introduction', 'story', 'dialogue']);
        const back = env.revisions.restore(env.store, PLAN_ID, { fromRevision: 1, path: LISTEN, expectedRevision: removed.revision }, env.ai());
        assert.equal(back.error, undefined, JSON.stringify(back));
        assert.deepEqual(ids(), ['introduction', 'story', 'listen', 'dialogue']);

        let revision = back.revision;
        for (const block of ['dialogue', 'listen', 'story']) {
            const r = env.ops.removeBlock(env.store, PLAN_ID, { path: LESSON + '/blocks/' + block, expectedRevision: revision }, env.human());
            assert.equal(r.error, undefined, JSON.stringify(r));
            revision = r.revision;
        }
        assert.deepEqual(ids(), ['introduction']);
        const clamped = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: LESSON + '/blocks/dialogue', expectedRevision: revision
        }, env.ai());
        assert.equal(clamped.error, undefined, JSON.stringify(clamped));
        assert.deepEqual(ids(), ['introduction', 'dialogue']);
    });

    it('ac-6: a lesson whose parent unit exists comes back; a block whose lesson is gone is parent_missing', () => {
        env.seed();
        const unit = working().units[0];
        const trimmed = JSON.parse(JSON.stringify(unit));
        trimmed.lessons = trimmed.lessons.filter((l) => l.id !== 'cafe-recall');
        const replaced = env.ops.replacePart(env.store, PLAN_ID, { path: 'units/unit-1', value: trimmed, expectedRevision: 1 }, env.human());
        assert.equal(replaced.error, undefined, JSON.stringify(replaced));
        assert.deepEqual(working().units[0].lessons.map((l) => l.id), ['cafe-order', 'cafe-again']);

        const before = env.db.get('SELECT COUNT(*) AS n FROM plan_revisions').n;
        const orphan = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: 'units/unit-1/lessons/cafe-recall/blocks/checkpoint', expectedRevision: replaced.revision
        }, env.ai());
        assert.deepEqual(orphan, {
            error: 'not_found', revision: replaced.revision,
            path: 'units/unit-1/lessons/cafe-recall/blocks/checkpoint', reason: 'parent_missing'
        });
        assert.equal(env.db.get('SELECT COUNT(*) AS n FROM plan_revisions').n, before);

        const lesson = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: 'units/unit-1/lessons/cafe-recall', expectedRevision: replaced.revision
        }, env.ai());
        assert.equal(lesson.error, undefined, JSON.stringify(lesson));
        assert.deepEqual(working().units[0].lessons.map((l) => l.id), ['cafe-order', 'cafe-again', 'cafe-recall']);
    });

    it('ac-6: a node that did not exist in the revision is absent_in_revision', () => {
        env.seed();
        const added = env.ops.insertBlock(env.store, PLAN_ID, {
            path: LESSON, block: { id: 'extra', type: 'listening', passage: 'תּוֹדָה.' }, expectedRevision: 1
        }, env.human());
        const before = env.db.get('SELECT COUNT(*) AS n FROM plan_revisions').n;
        const whole = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: LESSON + '/blocks/extra', expectedRevision: added.revision
        }, env.ai());
        assert.equal(whole.error, 'not_found');
        assert.equal(whole.reason, 'absent_in_revision');
        const field = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: LESSON + '/blocks/extra', field: 'passage', expectedRevision: added.revision
        }, env.ai());
        assert.equal(field.error, 'not_found');
        assert.equal(field.reason, 'absent_in_revision');
        const missing = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: LESSON + '/blocks/never', field: 'passage', expectedRevision: added.revision
        }, env.ai());
        assert.equal(missing.error, 'not_found');
        assert.equal(env.db.get('SELECT COUNT(*) AS n FROM plan_revisions').n, before);
    });

    it('ac-7: audience and assigneeId cannot be restored as fields', () => {
        env.seed();
        const ctx = env.human();
        const assigned = env.ops.assignPlan(env.store, PLAN_ID, { audience: 'personal', assigneeId: 'example-learner', expectedRevision: 1 }, ctx);
        const shared = env.ops.assignPlan(env.store, PLAN_ID, { audience: 'shared', expectedRevision: assigned.revision }, ctx);
        const before = env.store.loadWorkingCopy(PLAN_ID).document;
        const rows = logRows().length;
        for (const field of ['audience', 'assigneeId']) {
            const result = env.revisions.restore(env.store, PLAN_ID, {
                fromRevision: assigned.revision, path: '', field: field, expectedRevision: shared.revision
            }, env.ai());
            assert.equal(result.error, 'assign_required', field);
        }
        assert.equal(env.store.loadWorkingCopy(PLAN_ID).document, before);
        assert.equal(logRows().length, rows);
        for (const field of ['id', 'revision', 'status']) {
            assert.equal(env.revisions.restore(env.store, PLAN_ID, {
                fromRevision: 1, path: '', field: field, expectedRevision: shared.revision
            }, env.ai()).error, 'invalid_op', field);
        }
    });

    it('ac-7: a stale expectedRevision is revision_conflict and nothing is written', () => {
        const id = aiThenHuman();
        const before = env.store.loadWorkingCopy(id).document;
        const rows = logRows(id).length;
        const audit = env.count('audit_log');
        const marks = env.count('plan_field_marks');
        const result = env.revisions.restore(env.store, id, {
            fromRevision: 2, path: WORD, field: 'meaning', expectedRevision: 2
        }, env.ai());
        assert.deepEqual(result, { error: 'revision_conflict', revision: 3 });
        const missing = env.revisions.restore(env.store, id, { fromRevision: 2, path: WORD, field: 'meaning' }, env.ai());
        assert.equal(missing.error, 'revision_conflict');
        assert.equal(env.store.loadWorkingCopy(id).document, before);
        assert.equal(logRows(id).length, rows);
        assert.equal(env.count('audit_log'), audit);
        assert.equal(env.count('plan_field_marks'), marks);
    });

    it('ac-7: a restore whose result fails a write-time block rule is refused and writes nothing', () => {
        // A plan that was stored (by a script) with markup in its title, then fixed by a person.
        env.seed(samplePlan((p) => { p.title = '<b>old markup</b>'; }));
        const fixed = env.ops.setField(env.store, PLAN_ID, { path: '', field: 'title', value: 'Clean title', expectedRevision: 1 }, env.human());
        assert.equal(fixed.error, undefined);
        const before = env.store.loadWorkingCopy(PLAN_ID).document;
        const rows = logRows().length;
        const audit = env.count('audit_log');
        const marks = env.count('plan_field_marks');

        const field = env.revisions.restore(env.store, PLAN_ID, {
            fromRevision: 1, path: '', field: 'title', expectedRevision: fixed.revision
        }, env.ai());
        assert.equal(field.error, 'validation');
        assert.equal(field.blocks.some((b) => b.rule === 'html'), true);
        const whole = env.revisions.restore(env.store, PLAN_ID, { fromRevision: 1, expectedRevision: fixed.revision }, env.human());
        assert.equal(whole.error, 'validation');

        assert.equal(env.store.loadWorkingCopy(PLAN_ID).document, before);
        assert.equal(logRows().length, rows);
        assert.equal(env.count('audit_log'), audit);
        assert.equal(env.count('plan_field_marks'), marks);
    });

    it('a missing, unreadable, or malformed request is refused without a write', () => {
        const id = aiThenHuman();
        const before = env.store.loadWorkingCopy(id).document;
        const ctx = env.ai();
        assert.equal(env.revisions.restore(env.store, id, { fromRevision: 99, expectedRevision: 3 }, ctx).error, 'not_found');
        assert.equal(env.revisions.restore(env.store, 'no-such-plan', { fromRevision: 1, expectedRevision: 1 }, ctx).error, 'not_found');
        for (const bad of [undefined, 0, -1, 1.5, '2', null]) {
            assert.equal(env.revisions.restore(env.store, id, { fromRevision: bad, expectedRevision: 3 }, ctx).error, 'invalid_op');
        }
        assert.equal(env.revisions.restore(env.store, id, { fromRevision: 2, path: 5, expectedRevision: 3 }, ctx).error, 'invalid_op');
        assert.equal(env.revisions.restore(env.store, id, { fromRevision: 2, path: WORD, field: 'bad field!', expectedRevision: 3 }, ctx).error, 'invalid_op');
        env.db.run('UPDATE plan_revisions SET document_sha256 = ? WHERE plan_id = ? AND revision = 2', 'b'.repeat(64), id);
        assert.equal(env.revisions.restore(env.store, id, { fromRevision: 2, path: WORD, field: 'meaning', expectedRevision: 3 }, ctx).error, 'revision_corrupt');
        assert.throws(() => env.revisions.restore(env.store, id, { fromRevision: 2, expectedRevision: 3 }, undefined), TypeError);
        assert.equal(env.store.loadWorkingCopy(id).document, before);
    });

    it('ac-4: restore writes the marks of the restoring actor', () => {
        const id = aiThenHuman();
        env.revisions.restore(env.store, id, { fromRevision: 2, path: WORD, field: 'meaning', expectedRevision: 3 }, env.human('restore_plan_revision'));
        const mark = env.marksRepo.forPlan(id).find((m) => m.path === WORD && m.field === 'meaning');
        assert.equal(mark.lastActorKind, 'human');
        assert.equal(mark.lastRevision, 4);
        assert.equal(mark.previousValueJson, JSON.stringify('human-value'));
    });
});
