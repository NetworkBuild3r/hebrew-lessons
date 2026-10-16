'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { apply, loadPlan } = require('../ops');

const WORD = 'units/unit-1/lessons/cafe-order/blocks/introduction/words/2';
const LESSON = 'units/unit-1/lessons/cafe-order';

function coffee(plan) {
    return plan.units[0].lessons[0].blocks[0].words[1];
}

test('set_field replaces a picture and bumps the revision', () => {
    const plan = loadPlan();
    const result = apply(plan, {
        op: 'set_field',
        path: WORD,
        field: 'image',
        value: { mediaId: 'milk', alt: 'coffee' }
    }, 1);
    assert.equal(result.conflict, undefined);
    assert.equal(result.plan.revision, 2);
    assert.equal(result.plan.status, 'draft');
    assert.equal(coffee(result.plan).image.mediaId, 'milk');
    assert.equal(coffee(plan).image.mediaId, 'coffee-cup');
});

test('set_field can set alt on the picture', () => {
    const plan = loadPlan();
    const result = apply(plan, {
        op: 'set_field',
        path: WORD,
        field: 'alt',
        value: 'a cup of coffee'
    }, 1);
    assert.equal(coffee(result.plan).image.alt, 'a cup of coffee');
    assert.equal(coffee(result.plan).alt, undefined);
});

test('stale revision is a conflict and the plan is unchanged', () => {
    const plan = loadPlan();
    const before = JSON.stringify(plan);
    const result = apply(plan, {
        op: 'set_field',
        path: WORD,
        field: 'meaning',
        value: 'changed'
    }, 99);
    assert.deepEqual(result, { conflict: { error: 'revision_conflict', revision: 1 } });
    assert.equal(JSON.stringify(plan), before);
});

test('a later edit returns a published plan to draft', () => {
    const plan = loadPlan();
    plan.status = 'published';
    plan.revision = 3;
    const result = apply(plan, {
        op: 'set_field',
        path: 'units/unit-1',
        field: 'theme',
        value: 'ordering at a café'
    }, 3);
    assert.equal(result.plan.status, 'draft');
    assert.equal(result.plan.revision, 4);
    assert.equal(result.plan.units[0].theme, 'ordering at a café');
    assert.equal(plan.revision, 3);
});

test('insert_block, move_block, and remove_block', () => {
    const plan = loadPlan();
    const inserted = apply(plan, {
        op: 'insert_block',
        path: LESSON,
        index: 1,
        block: { id: 'extra', type: 'listening', passage: 'שלום' }
    }, 1);
    assert.equal(inserted.plan.units[0].lessons[0].blocks[1].id, 'extra');
    assert.equal(inserted.plan.units[0].lessons[0].blocks.length, 5);

    const moved = apply(inserted.plan, {
        op: 'move_block',
        path: LESSON + '/blocks/extra',
        index: 3
    }, 2);
    const ids = moved.plan.units[0].lessons[0].blocks.map((block) => block.id);
    assert.deepEqual(ids, ['introduction', 'story', 'listen', 'extra', 'dialogue']);

    const removed = apply(moved.plan, {
        op: 'remove_block',
        path: LESSON + '/blocks/extra'
    }, 3);
    assert.equal(removed.plan.units[0].lessons[0].blocks.some((block) => block.id === 'extra'), false);
    assert.equal(removed.plan.revision, 4);
});

test('replace_part swaps one block', () => {
    const plan = loadPlan();
    const block = {
        id: 'listen',
        type: 'listening',
        passage: 'תּוֹדָה.'
    };
    const result = apply(plan, {
        op: 'replace_part',
        path: LESSON + '/blocks/listen',
        value: block
    }, 1);
    assert.equal(result.plan.units[0].lessons[0].blocks[2].passage, 'תּוֹדָה.');
    assert.equal(result.plan.revision, 2);
});

test('read_part does not change the revision', () => {
    const plan = loadPlan();
    const result = apply(plan, { op: 'read_part', path: LESSON }, 50);
    assert.equal(result.part.id, 'cafe-order');
    assert.equal(result.plan.revision, 1);
    assert.equal(plan.revision, 1);
});

test('a prototype field name is rejected', () => {
    const plan = loadPlan();
    const result = apply(plan, {
        op: 'set_field',
        path: 'units/unit-1',
        field: '__proto__',
        value: { polluted: true }
    }, 1);
    assert.equal(result.conflict.error, 'invalid_op');
    assert.equal(plan.revision, 1);
});
