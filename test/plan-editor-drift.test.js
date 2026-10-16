/**
 * The editor drift test: a block type, block property, or lesson or plan property that the
 * plan JSON Schema holds must have a hint or a written exemption, so a new one cannot be
 * silently uneditable (the 1.1.0 gap RSCH-009 found). Run against a mutated copy of the
 * schema too, so the test cannot pass by being blind.
 * Provenance: INIT-008/SPEC-012
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const Form = require('../shared/admin/plan-form.js');
const Hints = require('../shared/admin/plan-hints.js');
const Widgets = require('../shared/admin/plan-widgets.js');

const SCHEMA = JSON.parse(fs.readFileSync(
    path.join(__dirname, '..', 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json'), 'utf8'
));

function registries(over) {
    return Object.assign({
        hints: Hints,
        exemptions: Hints.exemptions,
        widgets: Widgets.list()
    }, over || {});
}

function mutated() {
    const copy = JSON.parse(JSON.stringify(SCHEMA));
    copy.definitions.brandNewBlock = {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'type'],
        properties: { id: { $ref: '#/definitions/entryId' }, type: { const: 'brand-new' } }
    };
    copy.definitions.block.oneOf.push({ $ref: '#/definitions/brandNewBlock' });
    copy.definitions.lesson.properties.extraLessonThing = { type: 'string' };
    copy.definitions.wordEntry.properties.extraWordThing = { type: 'string' };
    return copy;
}

describe('plan editor drift (INIT-008/SPEC-012)', () => {
    it('ac-1: the real schema has no uncovered block type or property', () => {
        assert.deepEqual(Form.findUncoveredSchemaNodes(SCHEMA, registries()), []);
    });

    it('ac-1: a mutated schema is reported exactly: the new block type, lesson property, wordEntry property', () => {
        const found = Form.findUncoveredSchemaNodes(mutated(), registries());
        assert.deepEqual(
            found.map((p) => p.kind + ' ' + p.key).sort(),
            ['block-type brand-new', 'property lesson.extraLessonThing', 'property wordEntry.extraWordThing']
        );
    });

    it('ac-2: hints exist by name for all eleven block types and the named lesson fields', () => {
        const types = [
            'introduction', 'graded-story', 'listening', 'short-dialogue', 'fluency-pass',
            'spaced-review-checkpoint', 'explainer', 'conjugation-table', 'personal-question',
            'verse-reading', 'letter-intro', 'type-it', 'exercise', 'assignment'
        ];
        const tools = Form.createTools(SCHEMA);
        assert.deepEqual(tools.blockTypes().map((b) => b.type), types, 'the schema block list moved; update this list');
        for (const type of types) {
            assert.ok(Hints.blocks[type], 'no hint for block type ' + type);
            assert.equal(Hints.blocks[type].label.length, 2);
        }
        for (const key of ['lesson.recipe', 'lesson.newWords', 'lesson.buildsOn', 'lesson.recycles', 'lesson.visibility',
            'conjugationTableBlock.lemmaKey', 'personalQuestionBlock.prompts', 'verseReadingBlock.ref',
            'letterIntroBlock.sounds', 'explainerBlock.sections']) {
            assert.ok(Hints.fields[key], 'no hint for ' + key);
        }
    });

    it('ac-2: exactly two exemptions, each with a written reason', () => {
        assert.deepEqual(Object.keys(Hints.exemptions).sort(), ['lesson.legacyStorageKey', 'plan.migration']);
        for (const [key, entry] of Object.entries(Hints.exemptions)) {
            assert.ok(typeof entry.reason === 'string' && entry.reason.trim().length > 20, key + ' has no reason');
        }
    });

    it('an exemption with no reason, a hint on a missing path, and an unknown widget name are all reported', () => {
        const over = registries({
            hints: {
                fields: Object.assign({}, Hints.fields, {
                    'lesson.noSuchThing': { label: ['x', 'x'] },
                    'lesson.title': Object.assign({}, Hints.fields['lesson.title'], { widget: 'nonesuch' })
                }),
                blocks: Object.assign({}, Hints.blocks, { 'no-such-block': { label: ['x', 'x'] } }),
                kinds: Hints.kinds
            },
            exemptions: Object.assign({}, Hints.exemptions, { 'lesson.footer': { label: ['x', 'x'], reason: '' } })
        });
        const kinds = Form.findUncoveredSchemaNodes(SCHEMA, over).map((p) => p.kind + ' ' + p.key).sort();
        assert.deepEqual(kinds, [
            'exemption-reason lesson.footer',
            'hint-path block:no-such-block',
            'hint-path lesson.noSuchThing',
            'widget-name lesson.title'
        ]);
    });

    it('a registered widget that no hint names and no default uses is reported', () => {
        Widgets.register({ name: 'orphan', isDefault: false, test: () => 0, render: () => null });
        try {
            const found = Form.findUncoveredSchemaNodes(SCHEMA, registries({ widgets: Widgets.list() }));
            assert.deepEqual(found.map((p) => p.kind + ' ' + p.key), ['widget orphan']);
        } finally {
            Widgets.unregister('orphan');
        }
    });

    it('every default widget name is in the registry, and the JSON widget matches anything', () => {
        const names = Widgets.list().map((w) => w.name).sort();
        assert.deepEqual(names, Widgets.DEFAULT_NAMES.slice().sort());
        assert.equal(Widgets.byName('json').test({ schema: { oneOf: [] } }, {}), 1);
    });
});
