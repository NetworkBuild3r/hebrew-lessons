/**
 * The story-meanings list in the plan editor: a word and its meaning per row,
 * and a draft from the dictionary that a person checks before saving.
 * Fake DOM and a fake API; Hebrew here is hand-written test text.
 */
'use strict';

const { describe, it, afterEach } = require('node:test');
const assert = require('node:assert/strict');

const Form = require('../shared/admin/plan-form.js');
const dom = require('./helpers/fake-dom');
const { LESSON, samplePlan, makeEditor } = require('./helpers/plan-editor-fixtures');

const WORD = 'אֲנִי';
const MEANING = 'I, me';

function setup(glosses) {
    const plan = samplePlan();
    const block = plan.units[0].lessons[0].blocks.find((b) => b.type === 'graded-story');
    if (glosses) block.glosses = glosses;
    else delete block.glosses;
    const env = makeEditor({ getPlan: () => plan });
    const form = env.editor.renderNode({ path: LESSON + '/blocks/' + block.id, node: block, kind: 'block' });
    env.host.appendChild(form.el);
    const box = dom.collect(form.el, (n) => n.hasAttribute && n.hasAttribute('data-plan-gloss-map'))[0];
    return Object.assign({ plan, block, form, box }, env);
}

const attr = (root, name) => dom.collect(root, (n) => n.hasAttribute && n.hasAttribute(name));

describe('story meanings (word and meaning rows)', () => {
    afterEach(() => {
        delete globalThis.HebrewApi;
        delete globalThis.HebrewPlanForm;
    });

    it('is a list of rows, not a JSON box, and shows what is saved', () => {
        const env = setup({ [WORD]: MEANING });
        assert.ok(env.box, 'the gloss list is drawn');
        const field = attr(env.form.el, 'data-plan-field').find((n) => n.getAttribute('data-plan-field') === 'glosses');
        assert.equal(field.getAttribute('data-plan-widget'), 'gloss-map');
        const rows = attr(env.box, 'data-plan-item-op');
        assert.equal(rows.length, 1);
        const inputs = dom.collect(env.box, (n) => n.tagName === 'INPUT');
        assert.equal(inputs[0].value, WORD);
        assert.equal(inputs[1].value, MEANING);
        assert.equal(inputs[0].getAttribute('dir'), 'rtl');
        assert.equal(inputs[1].getAttribute('dir'), 'ltr');
    });

    it('a story with no meanings draws an empty list and keeps the field absent', () => {
        const env = setup(null);
        assert.equal(dom.collect(env.box, (n) => n.tagName === 'INPUT').length, 0);
        assert.ok(attr(env.box, 'data-plan-gloss-suggest')[0], 'the suggest button is there');
    });

    it('Suggest meanings adds draft rows for new words and never overwrites a typed one', async () => {
        globalThis.HebrewPlanForm = Form;
        const sent = [];
        globalThis.HebrewApi = {
            post(url, body) {
                sent.push({ url, body });
                return Promise.resolve({
                    ok: true,
                    status: 200,
                    data: {
                        suggestions: [
                            { word: 'בית', meaning: 'house, home', form: null, senses: [] },
                            { word: 'רוצים', meaning: 'to want', form: 'רץ', senses: [] }
                        ],
                        unmatched: ['צצצ'],
                        skipped: 1
                    }
                });
            }
        };
        const env = setup({ [WORD]: MEANING });
        const suggest = attr(env.box, 'data-plan-gloss-suggest')[0];
        suggest.click();
        await new Promise((resolve) => setTimeout(resolve, 20));
        assert.equal(sent.length, 1);
        assert.equal(sent[0].url, '/api/dictionary/suggest-glosses');
        assert.ok(sent[0].body.skip.includes(WORD), 'a word already in the list is not asked for again');
        assert.ok(sent[0].body.text.length > 0, 'the story text is sent');
        const inputs = dom.collect(env.box, (n) => n.tagName === 'INPUT');
        assert.deepEqual(inputs.map((n) => n.value), [WORD, MEANING, 'בית', 'house, home', 'רוצים', 'to want']);
        assert.equal(attr(env.box, 'data-plan-gloss-draft').length, 2, 'only the new rows are marked as drafts');
        const note = attr(env.box, 'data-plan-gloss-note')[0];
        assert.equal(note.hidden, false);
        assert.match(note.textContent, /Check each one against the story|קראו כל אחד/);
    });

    it('says so when the dictionary call fails, and adds nothing', async () => {
        globalThis.HebrewPlanForm = Form;
        globalThis.HebrewApi = { post: () => Promise.resolve({ ok: false, status: 500, data: null }) };
        const env = setup(null);
        attr(env.box, 'data-plan-gloss-suggest')[0].click();
        await new Promise((resolve) => setTimeout(resolve, 20));
        assert.equal(dom.collect(env.box, (n) => n.tagName === 'INPUT').length, 0);
        assert.match(attr(env.box, 'data-plan-gloss-note')[0].textContent, /could not be suggested|לא ניתן/);
    });
});
