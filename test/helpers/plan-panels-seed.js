/**
 * Seeds for the plan list, history, issues, and preview checks (INIT-008/SPEC-013 to SPEC-015):
 * plans made through the same service functions the MCP and the admin routes call, a disabled
 * account, and a verified lexicon entry. Call it after harness.start(): the harness clears the
 * server module cache when it starts, so the modules are required here, afterwards.
 * Hand-written text only; no learner text.
 */
'use strict';

const { samplePlan } = require('./plan-editor-fixtures');
const { basePlan } = require('./plan-fixtures');

const NFC_BAD = 'ת\u05BC\u05B8'; // a dagesh before a qamats: not in canonical order

/**
 * @returns {object} the contexts and ids the checks use
 */
function seedPanels() {
    const store = require('../../server/plans/store');
    const ops = require('../../server/plans/ops');
    const create = require('../../server/plans/create');
    const publish = require('../../server/plans/publish');
    const lexicon = require('../../server/db/repos/lexicon');
    const accounts = require('../../server/accounts');
    const { makeContext } = require('../../server/plans/context');
    const system = makeContext({ channel: 'system', name: 'panels-seed', tool: 'seed' });
    const ai = makeContext({ channel: 'mcp', tool: 'seed-tool' });

    const out = { system, ai, ids: {} };

    // a published plan with nothing after it, and one published and then edited
    const done = create.createPlan(store, { id: 'published-plan', title: 'Published plan' }, system);
    if (done.error) throw new Error('seed published-plan: ' + JSON.stringify(done));
    const pub = publish.publishPlan(store, 'published-plan', { expectedRevision: 1 }, system);
    if (pub.error) throw new Error('publish published-plan: ' + JSON.stringify(pub));

    const edited = create.createPlan(store, { id: 'edited-plan', title: 'Edited plan', recipe: 'guide' }, system);
    if (edited.error) throw new Error('seed edited-plan: ' + JSON.stringify(edited));
    const pub2 = publish.publishPlan(store, 'edited-plan', { expectedRevision: 1 }, system);
    if (pub2.error) throw new Error('publish edited-plan: ' + JSON.stringify(pub2));
    const after = ops.setField(store, 'edited-plan', {
        path: 'units/unit-1/lessons/lesson-1', field: 'footer', value: 'after the publish', expectedRevision: pub2.revision
    }, ai);
    if (after.error) throw new Error('edit edited-plan: ' + JSON.stringify(after));

    // a plan with a non-NFC word, a word that differs from a verified lexicon entry, and an edited
    // sentence that kept its audio clip after a publish
    lexicon.upsert({ key: 'בַּיִת'.normalize('NFC'), kind: 'lemma', gloss_en: 'house', translit: 'bayit', verified: 1, source: 'seed' });
    const bad = basePlan();
    bad.id = 'issues-plan';
    bad.title = 'Issues plan';
    store.saveWorkingCopy({
        id: bad.id, document: JSON.stringify(bad), revision: 1, status: 'draft', audience: 'shared', assigneeId: null
    });
    const first = publish.publishPlan(store, bad.id, { expectedRevision: 1 }, system);
    if (first.error) throw new Error('publish issues-plan: ' + JSON.stringify(first));
    const lesson = 'units/unit-1/lessons/lesson-1';
    let revision = first.revision;
    const steps = [
        { path: lesson + '/blocks/intro/words/w1', field: 'sentence', value: 'שָׁלוֹם לְכֻלָּם.' }, // audio-stale
        { path: lesson + '/blocks/intro/words/w2', field: 'word', value: 'בַיִת' }, // lexicon-form
        { path: lesson + '/blocks/listen', field: 'passage', value: NFC_BAD + ' דָּנִי.' } // pointing-form
    ];
    steps.forEach((step) => {
        const result = ops.setField(store, bad.id, Object.assign({ expectedRevision: revision }, step), ai);
        if (result.error) throw new Error('issues-plan step: ' + JSON.stringify(result));
        revision = result.revision;
    });
    out.ids.issues = bad.id;
    out.issuesLesson = lesson;

    // An account a personal plan can be assigned to (the schema wants a username of 8 characters or more,
    // so the harness's `learner` cannot be), and a disabled one to be refused as an assignee.
    accounts.createUser({ username: 'example-learner', password: 'a-strong-pass1', role: 'user', displayName: 'Example Learner', actor: 'test' });
    const retired = accounts.createUser({ username: 'example-retired', password: 'a-strong-pass1', role: 'user', displayName: 'Retired', actor: 'test' });
    accounts.setStatus(retired.id, 'disabled', 'test');

    out.sample = () => samplePlan();
    return out;
}

module.exports = { seedPanels, NFC_BAD };
