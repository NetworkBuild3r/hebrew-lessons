/**
 * Exercise block schema and validator rules N20–N23 (ADR-0014, INIT-012/SPEC-002).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { validatePlan, RULES } = require('../server/plans/validate');

const ROOT = path.resolve(__dirname, '..');
const EXAMPLES = JSON.parse(fs.readFileSync(
    path.join(ROOT, 'docs', 'decisions', 'ADR-0014-assets', 'exercise-examples.json'), 'utf8'
));
const EXERCISE_BLOCKS = EXAMPLES.blocks.filter((block) => block.type === 'exercise');

function shell(blocks, recipe) {
    return {
        schemaVersion: '1.2.0',
        id: 'test-exercise-plan',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Exercise test',
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
                recipe: recipe || 'review-worksheet',
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

function blocks(result) {
    return result.blocks.map((item) => item.rule + ' ' + item.path);
}

describe('INIT-012/SPEC-002: exercise block', () => {
    it('ac-1: each ADR-0014 exercise kind validates in a 1.2.0 document', () => {
        assert.equal(EXERCISE_BLOCKS.length, 5);
        EXERCISE_BLOCKS.forEach((block) => {
            const plan = shell([clone(block)]);
            const result = validatePlan(plan);
            assert.equal(result.ok, true, block.kind + ': ' + JSON.stringify(result.blocks));
        });
    });

    it('ac-2: the same exercise block in a 1.1.0 document is refused', () => {
        const block = clone(EXERCISE_BLOCKS[0]);
        const plan = shell([block]);
        plan.schemaVersion = '1.1.0';
        const result = validatePlan(plan);
        assert.equal(result.ok, false);
        const found = result.blocks.find((item) => item.message === 'this block type needs schemaVersion 1.2.0');
        assert.ok(found, blocks(result).join('; '));
    });

    it('ac-4: N20 refuses a choose answer that is not in choices', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'choose'));
        block.items[0].answer = 'not a choice';
        const result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((item) => item.rule === 'N20'));
    });

    it('ac-5: N21 refuses unscramble with two words or a bad alt order', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'unscramble'));
        block.items[0].words = block.items[0].words.slice(0, 2);
        let result = validatePlan(shell([block]));
        assert.ok(result.blocks.length > 0, 'two words fails schema or N21');

        const good = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'unscramble'));
        good.items[0].alt = [['wrong', 'token', 'list', 'here', 'extra']];
        result = validatePlan(shell([good]));
        assert.ok(result.blocks.some((item) => item.rule === 'N21'));
    });

    it('ac-6: N22 and audio-path refuse dictation without audioPath or with ../ escape', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'dictation'));
        delete block.items[0].audioPath;
        let result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((item) => item.rule === 'N22' || item.rule === 'schema'));

        const escaped = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'dictation'));
        escaped.items[0].audioPath = 'audio/../secret.mp3';
        result = validatePlan(shell([escaped]));
        assert.ok(result.blocks.some((item) => item.rule === 'N22' || item.rule === 'audio-path'));
    });

    it('ac-7: pointed accept strings and markup in text fields are refused', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'translate'));
        block.items[0].accept = ['שָׁלוֹם'];
        let result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((item) => item.path.includes('/accept/')));

        block.items[0].accept = ['שלום'];
        block.items[0].english = 'bad <script>';
        result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((item) => item.rule === 'html'));
    });

    it('ac-8: twelve exercise items pass; a thirteenth is refused by N10', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'true-false'));
        const item = block.items[0];
        block.items = Array.from({ length: 12 }, (_, i) => Object.assign(clone(item), { id: 'tf-' + i }));
        let result = validatePlan(shell([block]));
        assert.equal(result.blocks.some((finding) => finding.rule === 'N10'), false);

        block.items.push(Object.assign(clone(item), { id: 'tf-12' }));
        result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((finding) => finding.rule === 'N10'));
    });

    it('ac-8: an oversized exercise block or plan is refused by N9', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'true-false'));
        block.items[0].why = 'x'.repeat(40000);
        let result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((finding) => finding.rule === 'N9'));

        const plan = shell([clone(EXERCISE_BLOCKS.find((b) => b.kind === 'choose'))]);
        plan.title = 'x'.repeat(600000);
        result = validatePlan(plan);
        assert.ok(result.blocks.some((finding) => finding.rule === 'N9' && finding.path === ''));
    });

    it('N21 refuses unscramble when an alt order matches the main words order', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'unscramble'));
        block.items[0].alt = [block.items[0].words.slice()];
        const result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((finding) => finding.rule === 'N21'));
    });

    it('N20 refuses choose when answer matches a choice only by vowel points', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'choose'));
        block.items[0].answer = 'במטבח יש שולחן.';
        const result = validatePlan(shell([block]));
        assert.ok(result.blocks.some((finding) => finding.rule === 'N20'));
    });

    it('translate accept may list both plene and defective unpointed spellings', () => {
        const block = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'translate'));
        block.items[0].answer = 'שלוש';
        block.items[0].accept = ['שלוש', 'שלש'];
        const result = validatePlan(shell([block]));
        assert.equal(result.ok, true, JSON.stringify(result.blocks));
        assert.equal(result.blocks.some((finding) => finding.path.includes('/accept/')), false);
    });

    it('N23 warns when a topic-conversation lesson has more than two exercise blocks', () => {
        const choose = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'choose'));
        const tf = clone(EXERCISE_BLOCKS.find((b) => b.kind === 'true-false'));
        tf.id = 'ex-tf-2';
        const third = clone(choose);
        third.id = 'ex-choose-3';
        const plan = shell([choose, tf, third], 'topic-conversation');
        const result = validatePlan(plan);
        assert.ok(result.warnings.some((item) => item.rule === 'N23'));
        assert.equal(RULES.N23.severity, 'warn');
    });
});
