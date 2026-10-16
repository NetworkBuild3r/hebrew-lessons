/**
 * Test helper: a fresh temporary database and freshly loaded plan modules
 * (INIT-008/SPEC-004 to SPEC-007). Nothing here opens data/hebrew.sqlite.
 *
 *   const env = planEnv('my-test-');   // migrates a temp DATA_DIR
 *   env.ai(), env.human(), env.system()  // write contexts for the three actor kinds
 *   env.seed(plan)                       // store a plan row with no log rows (a pre-log plan)
 *   env.close()                          // remove the temp dir, restore the environment
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..', '..');
const SAMPLE = fs.readFileSync(path.join(
    root, 'docs', 'decisions', 'ADR-0003-assets', 'samples', 'beginner-unit-1.plan.json'
), 'utf8');

const PLAN_ID = 'beginner-unit-1';
const LESSON = 'units/unit-1/lessons/cafe-order';
const WORD = LESSON + '/blocks/introduction/words/2';
const LISTEN = LESSON + '/blocks/listen';

/** A fresh copy of the beginner-unit-1 sample plan, optionally edited. */
function samplePlan(edit) {
    const plan = JSON.parse(SAMPLE);
    if (edit) edit(plan);
    return plan;
}

/**
 * Drop every cached module that holds the database or a repo bound to it, so
 * the next require sees the new DATA_DIR.
 */
function clearModules() {
    const keep = [
        path.join(root, 'server', 'plans') + path.sep,
        path.join(root, 'server', 'db') + path.sep,
        path.join(root, 'server', 'paths')
    ];
    for (const key of Object.keys(require.cache)) {
        if (keep.some((prefix) => key.indexOf(prefix) === 0)) delete require.cache[key];
    }
}

/**
 * @param {string} [prefix] Temp directory name prefix.
 * @returns {object}
 */
function planEnv(prefix) {
    const saved = {
        HEBREW_ROOT: process.env.HEBREW_ROOT,
        DATA_DIR: process.env.DATA_DIR,
        LESSONS_DIR: process.env.LESSONS_DIR
    };
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), prefix || 'hebrew-plan-'));
    const lessons = fs.mkdtempSync(path.join(os.tmpdir(), (prefix || 'hebrew-plan-') + 'lessons-'));
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = tmp;
    process.env.LESSONS_DIR = lessons;
    clearModules();

    const db = require('../../server/db');
    db.close();
    db.migrate();
    const store = require('../../server/plans/store');
    const context = require('../../server/plans/context');
    const env = {
        root,
        tmp,
        db,
        store,
        context,
        ops: require('../../server/plans/ops'),
        publish: require('../../server/plans/publish'),
        personal: require('../../server/plans/personal'),
        revisions: require('../../server/plans/revisions'),
        revisionsRepo: require('../../server/db/repos/plan-revisions'),
        marksRepo: require('../../server/db/repos/plan-field-marks'),
        provenance: require('../../server/plans/provenance'),
        create: require('../../server/plans/create'),
        diff: require('../../server/plans/diff'),
        auditRepo: require('../../server/db/repos/audit'),
        PLAN_ID, LESSON, WORD, LISTEN,
        samplePlan,
        ai: (tool) => context.makeContext({ channel: 'mcp', tool: tool || 'test' }),
        human: (tool, username) => context.makeContext({
            channel: 'admin', username: username || 'example-admin', tool: tool || 'test'
        }),
        system: (name) => context.makeContext({ channel: 'system', name: name || 'test-script', tool: 'test' }),
        seed(plan) {
            const doc = plan || samplePlan();
            store.saveWorkingCopy({
                id: doc.id,
                document: JSON.stringify(doc),
                revision: doc.revision,
                status: doc.status,
                audience: doc.audience,
                assigneeId: doc.assigneeId
            });
            return store.loadWorkingCopy(doc.id);
        },
        count(table, planId) {
            const where = planId ? ' WHERE ' + (table === 'audit_log' ? 'target' : 'plan_id') + ' = ?' : '';
            const rows = planId ? db.get('SELECT COUNT(*) AS n FROM ' + table + where, planId)
                : db.get('SELECT COUNT(*) AS n FROM ' + table);
            return rows.n;
        },
        close() {
            db.close();
            fs.rmSync(tmp, { recursive: true, force: true });
            fs.rmSync(lessons, { recursive: true, force: true });
            for (const key of Object.keys(saved)) {
                if (saved[key] === undefined) delete process.env[key];
                else process.env[key] = saved[key];
            }
            clearModules();
        }
    };
    return env;
}

module.exports = { planEnv, samplePlan, PLAN_ID, LESSON, WORD, LISTEN };
