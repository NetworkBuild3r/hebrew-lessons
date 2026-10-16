/**
 * Assignment block schema and version guards (ADR-0014, INIT-012/SPEC-003).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { validatePlan } = require('../server/plans/validate');

const ROOT = path.resolve(__dirname, '..');
const EXAMPLES = JSON.parse(fs.readFileSync(
    path.join(ROOT, 'docs', 'decisions', 'ADR-0014-assets', 'exercise-examples.json'), 'utf8'
));
const ASSIGNMENT = EXAMPLES.blocks.find((block) => block.type === 'assignment');

function shell(blocks, recipe) {
    return {
        schemaVersion: '1.2.0',
        id: 'test-assignment-plan',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Assignment test',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-1',
            title: 'Unit',
            theme: 'test',
            lessons: [{
                id: 'lesson-1',
                title: 'Lesson',
                level: 'basic',
                visibility: 'hidden',
                recipe: recipe || 'topic-conversation',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks: blocks
            }]
        }]
    };
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

describe('INIT-012/SPEC-003: assignment block', () => {
    it('ac-1: ADR-0014 assignment validates in 1.2.0 and is refused in 1.1.0', () => {
        assert.ok(ASSIGNMENT);
        const ok = validatePlan(shell([clone(ASSIGNMENT)]));
        assert.equal(ok.ok, true, JSON.stringify(ok.blocks));

        const old = shell([clone(ASSIGNMENT)]);
        old.schemaVersion = '1.1.0';
        const bad = validatePlan(old);
        assert.equal(bad.ok, false);
        assert.ok(
            bad.blocks.some((item) => item.message === 'this block type needs schemaVersion 1.2.0'),
            bad.blocks.map((item) => item.message).join('; ')
        );
    });

    it('ac-2: minSentences 0 or 13, seven homework lines, and markup are refused', () => {
        const low = clone(ASSIGNMENT);
        low.minSentences = 0;
        assert.equal(validatePlan(shell([low])).ok, false);

        const high = clone(ASSIGNMENT);
        high.minSentences = 13;
        assert.equal(validatePlan(shell([high])).ok, false);

        const homework = clone(ASSIGNMENT);
        homework.homework = homework.homework.concat(['a', 'b', 'c', 'd', 'e']);
        assert.equal(validatePlan(shell([homework])).ok, false);

        const markup = clone(ASSIGNMENT);
        markup.prompt = 'Write <b>here</b>';
        const markupResult = validatePlan(shell([markup]));
        assert.equal(markupResult.ok, false);
        assert.ok(markupResult.blocks.some((item) => item.rule === 'html' || item.rule === 'schema'));
    });

    it('ac-2: markup in useWords and homework is refused with html', () => {
        const words = clone(ASSIGNMENT);
        words.useWords = ['סוף', '<img src=x>'];
        const wordsResult = validatePlan(shell([words]));
        assert.equal(wordsResult.ok, false);
        assert.ok(wordsResult.blocks.some((item) => item.rule === 'html'));

        const hw = clone(ASSIGNMENT);
        hw.homework = ['Read aloud.', '<img src=x>'];
        const hwResult = validatePlan(shell([hw]));
        assert.equal(hwResult.ok, false);
        assert.ok(hwResult.blocks.some((item) => item.rule === 'html'));
    });

    it('assignment without useWords or homework validates in 1.2.0', () => {
        const minimal = clone(ASSIGNMENT);
        delete minimal.useWords;
        delete minimal.homework;
        const result = validatePlan(shell([minimal]));
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
    });

    it('ac-2: N10 reports list-cap violations on useWords and homework', () => {
        const words = clone(ASSIGNMENT);
        words.useWords = Array.from({ length: 13 }, (_, i) => 'word' + i);
        const wordsResult = validatePlan(shell([words]));
        assert.ok(wordsResult.blocks.some((item) => item.rule === 'N10' && item.path.endsWith('/useWords')));

        const lines = clone(ASSIGNMENT);
        lines.homework = Array.from({ length: 7 }, (_, i) => 'line ' + i);
        const linesResult = validatePlan(shell([lines]));
        assert.ok(linesResult.blocks.some((item) => item.rule === 'N10' && item.path.endsWith('/homework')));
    });

    it('assignment satisfies N16 on topic-conversation when no personal-question is present', () => {
        const plan = shell([clone(ASSIGNMENT)], 'topic-conversation');
        const result = validatePlan(plan);
        assert.equal(result.informs.filter((item) => item.rule === 'N16').length, 0);
    });
});
