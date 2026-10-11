'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { brokenPlans, sampleFiles, validatePlan } = require('../validate');

test('four ADR-0003 samples validate', () => {
    const files = sampleFiles();
    assert.equal(files.length, 4);
    files.forEach((file) => {
        const data = JSON.parse(fs.readFileSync(file, 'utf8'));
        const result = validatePlan(data);
        assert.equal(result.ok, true, path.basename(file) + ' ' + JSON.stringify(result.errors.slice(0, 3)));
    });
});

test('broken plans are rejected for the stated reason', () => {
    const base = JSON.parse(fs.readFileSync(
        path.join(path.dirname(sampleFiles()[0]), 'beginner-unit-1.plan.json'),
        'utf8'
    ));
    brokenPlans(base).forEach((item) => {
        const result = validatePlan(item.plan);
        assert.equal(result.ok, false, item.name);
        assert.ok(
            result.errors.some((error) => error.message === item.reason),
            item.name + ' ' + JSON.stringify(result.errors.map((error) => error.message))
        );
    });
});
