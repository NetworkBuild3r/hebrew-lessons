/**
 * deletePlan removes the working copy, the snapshot, and the history rows.
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');

const { planEnv } = require('./helpers/plan-db');

describe('deletePlan', { concurrency: false }, () => {
    let env;
    let deletePlan;

    before(() => {
        env = planEnv('hebrew-plan-delete-');
        deletePlan = require('../server/plans/delete').deletePlan;
    });

    after(() => {
        env.close();
    });

    beforeEach(() => {
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM audit_log');
    });

    it('deletes the plan, its snapshot, and its history, and writes an audit row', () => {
        const created = env.create.createPlan(env.store, {
            id: 'shelf-plan',
            title: 'Shelf plan'
        }, env.ai('create_plan'));
        assert.equal(created.error, undefined);
        const published = env.publish.publishPlan(env.store, 'shelf-plan', {
            expectedRevision: 1
        }, env.ai('publish_plan'));
        assert.equal(published.error, undefined);
        assert.equal(env.db.get('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?', 'shelf-plan').n > 0, true);

        const gone = deletePlan(env.store, 'shelf-plan', { expectedRevision: published.revision }, env.ai('delete_plan'));
        assert.deepEqual(gone, { id: 'shelf-plan', deleted: true });
        assert.equal(env.store.loadWorkingCopy('shelf-plan'), null);
        assert.equal(env.store.loadSnapshot('shelf-plan'), null);
        assert.equal(env.db.get('SELECT COUNT(*) AS n FROM plan_revisions WHERE plan_id = ?', 'shelf-plan').n, 0);
        assert.equal(env.db.get('SELECT COUNT(*) AS n FROM plan_field_marks WHERE plan_id = ?', 'shelf-plan').n, 0);
        const audit = env.db.get(
            "SELECT actor, action, target FROM audit_log WHERE action = 'plan.delete'"
        );
        assert.equal(audit.action, 'plan.delete');
        assert.equal(audit.target, 'shelf-plan');
        assert.match(audit.actor, /mcp/);
    });

    it('a stale revision deletes nothing', () => {
        env.create.createPlan(env.store, { id: 'keep-plan', title: 'Keep' }, env.human('create_plan'));
        const stale = deletePlan(env.store, 'keep-plan', { expectedRevision: 9 }, env.human('delete_plan'));
        assert.deepEqual(stale, { error: 'revision_conflict', revision: 1 });
        assert.equal(env.store.loadWorkingCopy('keep-plan').revision, 1);
        assert.equal(env.db.get("SELECT COUNT(*) AS n FROM audit_log WHERE action = 'plan.delete'").n, 0);
    });

    it('an unknown plan is not_found', () => {
        const missing = deletePlan(env.store, 'missing-plan', { expectedRevision: 1 }, env.human('delete_plan'));
        assert.equal(missing.error, 'not_found');
    });
});
