/**
 * Hints, new blocks, and a form for every node kind and block type (INIT-008/SPEC-012).
 * Fake DOM (test/helpers/fake-dom.js). The browser run is test/chrome-plan-editor-smoke.js.
 * Hebrew in these fixtures is hand-written; there is no teacher or learner text.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const Form = require('../shared/admin/plan-form.js');
const Hints = require('../shared/admin/plan-hints.js');
const Widgets = require('../shared/admin/plan-widgets.js');
const { FIELD_LANGUAGE, languageOf } = require('../server/plans/hebrew-checks');
const dom = require('./helpers/fake-dom');

const { LESSON, SCHEMA, TOOLS, HEBREW, samplePlan, makeEditor, ownFields } = require('./helpers/plan-editor-fixtures');

describe('hints agree with the server language registry (INIT-008/SPEC-012 ac-4)', () => {
    it('every hint language equals FIELD_LANGUAGE for the same schema path, and the reverse', () => {
        const reachable = new Set(Object.keys(Hints.fields));
        const bad = [];
        for (const [key, hint] of Object.entries(Hints.fields).concat(Object.entries(Hints.exemptions))) {
            const dot = key.indexOf('.');
            const expected = languageOf(key.slice(0, dot), key.slice(dot + 1));
            if (expected === '*.id') continue;
            if (hint.lang !== expected) bad.push(key + ': hint ' + hint.lang + ', server ' + expected);
        }
        assert.deepEqual(bad.filter((line) => !/\.id:|\.type:/.test(line)), []);
        // Every non-wildcard registry row whose property the editor shows has a hint with a language.
        const missing = Object.keys(FIELD_LANGUAGE)
            .filter((key) => key.indexOf('*.') !== 0 && !key.startsWith('migration.'))
            .filter((key) => !reachable.has(key) && !Hints.exemptions[key]);
        assert.deepEqual(missing, []);
    });

    it('langAttrs gives rtl/he for Hebrew, ltr/en for English, and no fixed direction for a mixed field', () => {
        assert.deepEqual(Hints.langAttrs('he'), { dir: 'rtl', lang: 'he' });
        assert.deepEqual(Hints.langAttrs('en'), { dir: 'ltr', lang: 'en' });
        assert.equal(Hints.langAttrs('latin').dir, 'ltr');
        assert.equal(Hints.langAttrs('mixed').dir, 'auto');
        assert.equal(Hints.langAttrs(undefined).dir, 'auto');
    });
});

describe('newBlock (INIT-008/SPEC-012 ac-3)', () => {
    it('for each of the eleven types: every required property, the type, the given id, and no Hebrew', () => {
        const types = TOOLS.blockTypes();
        assert.equal(types.length, 14);
        const ids = new Set();
        for (const { type, def } of types) {
            const id = 'new-' + type;
            const block = Form.newBlock(type, id, SCHEMA);
            assert.equal(block.id, id);
            assert.equal(block.type, type);
            for (const key of SCHEMA.definitions[def].required) {
                assert.ok(Object.prototype.hasOwnProperty.call(block, key), type + ' lacks required ' + key);
            }
            assert.doesNotMatch(JSON.stringify(block), HEBREW, type + ' holds Hebrew');
            assert.ok(!ids.has(block.id));
            ids.add(block.id);
        }
        assert.equal(Form.newBlock('no-such-type', 'x', SCHEMA), null);
    });

    it('uses a hint default over the schema minimum, and the minimum for a required integer', () => {
        assert.equal(Form.newBlock('fluency-pass', 'f', SCHEMA).seconds, 30);
        assert.deepEqual(Form.newBlock('verse-reading', 'v', SCHEMA).ref, { book: '', chapter: 1, from: 1, to: 1, source: '' });
        assert.equal(Form.newBlock('personal-question', 'p', SCHEMA).answer, 'either');
    });

    it('the block-type list comes from the schema, in its order, with bilingual labels', () => {
        const list = Form.blockTypeList(SCHEMA);
        assert.deepEqual(list.map((b) => b.type), TOOLS.blockTypes().map((b) => b.type));
        for (const b of list) assert.ok(HEBREW.test(b.he) && /[A-Za-z]/.test(b.en), b.type);
        assert.deepEqual(Form.enumOf('lesson', 'recipe', SCHEMA), SCHEMA.definitions.recipe.enum);
    });
});

describe('a form for every node kind (INIT-008/SPEC-012 ac-4)', () => {
    const plan = samplePlan();

    function render(spec) {
        const env = makeEditor();
        const form = env.editor.renderNode(spec);
        env.host.appendChild(form.el);
        return Object.assign({ form }, env);
    }

    it('plan, unit, and lesson: one control per property except the child list, each with a widget', () => {
        const lesson = plan.units[0].lessons[0];
        const cases = [
            { spec: { path: '', node: plan, kind: 'plan' }, def: 'plan', child: 'units' },
            { spec: { path: 'units/u1', node: plan.units[0], kind: 'unit' }, def: 'unit', child: 'lessons' },
            { spec: { path: LESSON, node: lesson, kind: 'lesson' }, def: 'lesson', child: 'blocks' }
        ];
        for (const c of cases) {
            const { form } = render(c.spec);
            const want = Object.keys(SCHEMA.definitions[c.def] ? SCHEMA.definitions[c.def].properties : SCHEMA.properties)
                .filter((p) => p !== c.child).sort();
            const have = ownFields(form.el).map((n) => n.getAttribute('data-plan-field')).sort();
            assert.deepEqual(have, want, c.def);
            ownFields(form.el).forEach((n) => assert.ok(Widgets.byName(n.getAttribute('data-plan-widget')), c.def));
        }
    });

    it('each block type: a control per property and a widget on each', () => {
        const lesson = plan.units[0].lessons[0];
        const jsonFallbacks = [];
        for (const { type, def } of TOOLS.blockTypes()) {
            const block = lesson.blocks.find((b) => b.type === type);
            const { form } = render({ path: LESSON + '/blocks/' + block.id, node: block, kind: 'block' });
            assert.equal(form.def, def);
            const want = Object.keys(SCHEMA.definitions[def].properties)
                .filter((p) => p !== 'id')
                .filter((p) => !(p === 'type' && (type === 'exercise' || type === 'assignment')))
                .sort();
            const have = ownFields(form.el).map((n) => n.getAttribute('data-plan-field')).sort();
            assert.deepEqual(have, want, type);
            ownFields(form.el).forEach((n) => {
                const name = n.getAttribute('data-plan-widget');
                assert.ok(Widgets.byName(name), type + ' ' + n.getAttribute('data-plan-field'));
                if (name === 'json') jsonFallbacks.push(def + '.' + n.getAttribute('data-plan-field'));
            });
        }
        // No block property needs the raw JSON box: story glosses have their own word-and-meaning list.
        assert.deepEqual(jsonFallbacks, []);
    });

    it('a bucket item form for a word, paragraph, line, and question has every property', () => {
        const lesson = plan.units[0].lessons[0];
        const cases = [
            ['introduction', 'words', 'wordEntry'], ['graded-story', 'paragraphs', 'paragraph'],
            ['short-dialogue', 'lines', 'line'], ['listening', 'questions', 'question']
        ];
        for (const [type, bucket, def] of cases) {
            const block = lesson.blocks.find((b) => b.type === type);
            const item = block[bucket][0];
            const { form } = render({ path: LESSON + '/blocks/' + block.id + '/' + bucket + '/' + item.id, node: item, def, kind: 'item' });
            assert.deepEqual(
                ownFields(form.el).map((n) => n.getAttribute('data-plan-field')).sort(),
                Object.keys(SCHEMA.definitions[def].properties).filter((p) => p !== 'id').sort(), def
            );
        }
    });

    it('a block shows its bucket items as forms at their own id paths', () => {
        const block = plan.units[0].lessons[0].blocks.find((b) => b.type === 'introduction');
        const { form, editor } = render({ path: LESSON + '/blocks/' + block.id, node: block, kind: 'block' });
        const forms = editor.forms();
        assert.ok(forms[LESSON + '/blocks/introduction/words/1'], Object.keys(forms).join());
        assert.equal(form.fields.words.widgetName, 'bucket-list');
    });

    it('nested structures use object, object-list, and string-list widgets, not JSON', () => {
        const block = plan.units[0].lessons[0].blocks.find((b) => b.type === 'explainer');
        const { form } = render({ path: LESSON + '/blocks/explainer', node: block, kind: 'block' });
        assert.equal(form.fields.sections.widgetName, 'object-list');
        const cj = plan.units[0].lessons[0].blocks.find((b) => b.type === 'conjugation-table');
        const r2 = render({ path: LESSON + '/blocks/conjugation-table', node: cj, kind: 'block' });
        assert.equal(r2.form.fields.cells.widgetName, 'table-cells');
        assert.equal(r2.form.fields.tense.widgetName, 'select');
        const verse = plan.units[0].lessons[0].blocks.find((b) => b.type === 'verse-reading');
        const r3 = render({ path: LESSON + '/blocks/verse-reading', node: verse, kind: 'block' });
        assert.equal(r3.form.fields.ref.widgetName, 'object');
        assert.equal(r3.form.fields.newWords.widgetName, 'string-list');
        const lesson = plan.units[0].lessons[0];
        const r4 = render({ path: LESSON, node: lesson, kind: 'lesson' });
        assert.equal(r4.form.fields.recipe.widgetName, 'select');
        assert.equal(r4.form.fields.visibility.widgetName, 'select');
        assert.equal(r4.form.fields.recycles.widgetName, 'object-list');
        assert.equal(r4.form.fields.buildsOn.widgetName, 'string-list');
        assert.equal(r4.form.fields.newWords.widgetName, 'string-list');
    });

    it('every Hebrew control is lang="he" dir="rtl" and every English control lang="en" dir="ltr"', () => {
        const lesson = plan.units[0].lessons[0];
        let checked = 0;
        const visit = (spec, def) => {
            const { form } = render(spec);
            Object.keys(form.fields).forEach((prop) => {
                const handle = form.fields[prop];
                const lang = languageOf(def, prop);
                if (lang !== 'he' && lang !== 'en') return;
                const ctl = handle.widgetName === 'string-list'
                    ? dom.collect(handle.el, (n) => n.tagName === 'INPUT' || n.tagName === 'TEXTAREA')[0]
                    : handle.el;
                if (!ctl) return;
                assert.equal(ctl.getAttribute('lang'), lang, def + '.' + prop);
                assert.equal(ctl.getAttribute('dir'), lang === 'he' ? 'rtl' : 'ltr', def + '.' + prop);
                checked += 1;
            });
        };
        visit({ path: LESSON, node: lesson, kind: 'lesson' }, 'lesson');
        for (const { type, def } of TOOLS.blockTypes()) {
            const block = lesson.blocks.find((b) => b.type === type);
            visit({ path: LESSON + '/blocks/' + block.id, node: block, kind: 'block' }, def);
        }
        assert.ok(checked > 5, 'only ' + checked + ' language controls were checked');
        // Every text control of every form, nested ones included: a lang of he is rtl, a lang of en is ltr.
        let heControls = 0;
        let enControls = 0;
        const all = makeEditor();
        for (const block of lesson.blocks) {
            const f = all.editor.renderNode({ path: LESSON + '/blocks/' + block.id, node: block, kind: 'block' });
            dom.collect(f.el, (n) => n.tagName === 'INPUT' || n.tagName === 'TEXTAREA').forEach((n) => {
                if (n.getAttribute('lang') === 'he') {
                    heControls += 1;
                    assert.equal(n.getAttribute('dir'), 'rtl');
                }
                if (n.getAttribute('lang') === 'en') {
                    enControls += 1;
                    assert.equal(n.getAttribute('dir'), 'ltr');
                }
            });
        }
        assert.ok(heControls > 25 && enControls > 8, 'he ' + heControls + ' en ' + enControls);
    });

    it('a Hebrew field shows its character count against the schema limit and the pointing note', () => {
        const block = plan.units[0].lessons[0].blocks.find((b) => b.type === 'listening');
        const { form } = render({ path: LESSON + '/blocks/listening', node: block, kind: 'block' });
        const note = dom.collect(form.fields.passage.row, (n) => n.hasAttribute && n.hasAttribute('data-plan-count'))[0];
        assert.equal(note.textContent, block.passage.length + ' / 4000');
        assert.match(form.fields.passage.row.textContent, /Full pointing/);
    });

    it('labels are Hebrew first, English second, and use no points, streak, or joke wording', () => {
        const strings = [];
        for (const hint of Object.values(Hints.fields).concat(Object.values(Hints.blocks), Object.values(Hints.exemptions))) {
            strings.push(...hint.label);
            if (hint.help) strings.push(...hint.help);
        }
        for (const [he, en] of Object.values(Hints.fields).concat(Object.values(Hints.blocks)).map((h) => h.label)) {
            assert.ok(HEBREW.test(he), 'Hebrew label is not Hebrew: ' + he + ' / ' + en);
            assert.ok(/[A-Za-z]/.test(en), 'English gloss missing: ' + he);
        }
        assert.doesNotMatch(strings.join(' '), /\b(points?|streak|score|joke|fun)\b/i);
    });
});

describe('every object definition gets a widget for every property (INIT-008/SPEC-012 ac-4)', () => {
    it('renders each schema definition that has properties as a draft form, one row per property', () => {
        const env = makeEditor();
        const names = Object.keys(SCHEMA.definitions).filter((n) => SCHEMA.definitions[n].properties && !/^v1\d\d/.test(n));
        // The item types the spec names, among the rest.
        for (const must of ['patternItem', 'clozeItem', 'glossPair', 'hebrewLine', 'letterSound', 'letterFind',
            'explainerExample', 'explainerCallout', 'patternRule', 'span', 'recycle', 'verseRef', 'conjugationCell']) {
            assert.ok(names.includes(must), must);
        }
        let rows = 0;
        for (const def of names) {
            const node = TOOLS.defSchema(def);
            const seed = env.editor.ctx.skeleton(node, def);
            const handle = env.editor.renderDraft(def, seed, null);
            // The object widget's body is the last child of its wrapper; count the direct rows of it.
            const body = dom.collect(handle.el, (n) => n.className === 'plan-object-body')[0];
            const direct = body.children.filter((c) => c.hasAttribute('data-plan-row'));
            assert.equal(direct.length, Object.keys(node.properties).length, def);
            direct.forEach((row) => {
                const control = row.children.find((c) => c.hasAttribute('data-plan-sub'));
                assert.ok(control, def + ' ' + row.getAttribute('data-plan-row') + ' has no control');
                assert.ok(Widgets.byName(control.getAttribute('data-plan-widget')), def);
            });
            rows += direct.length;
        }
        assert.ok(rows > 120, 'only ' + rows + ' rows');
    });
});
describe('the hand-written lists are gone (INIT-008/SPEC-012 ac-5)', () => {
    it('BLOCK_TYPES, BLOCK_FIELDS, ITEM_FIELDS, and FIELD_OK no longer appear in shared/admin', () => {
        const dir = path.join(__dirname, '..', 'shared', 'admin');
        for (const name of fs.readdirSync(dir).filter((n) => /\.js$/.test(n))) {
            assert.doesNotMatch(fs.readFileSync(path.join(dir, name), 'utf8'), /BLOCK_TYPES|BLOCK_FIELDS|ITEM_FIELDS|FIELD_OK/, name);
        }
    });

    it('the editor files use no innerHTML and no eval', () => {
        const dir = path.join(__dirname, '..', 'shared', 'admin');
        for (const name of ['plan-form.js', 'plan-widgets.js', 'plan-hints.js']) {
            const text = fs.readFileSync(path.join(dir, name), 'utf8');
            assert.doesNotMatch(text, /innerHTML|outerHTML|insertAdjacentHTML|document\.write|eval\s*\(|new Function/, name);
        }
    });
});
