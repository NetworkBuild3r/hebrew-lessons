'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const fs = require('fs');
const path = require('path');
const { PLAN_PATH } = require('../ops');
const { applyPosted } = require('../server');

const SKELETON = path.join(__dirname, '..');

function runCli(fixture) {
    return spawnSync(process.execPath, [path.join(SKELETON, 'cli.js'), fixture], {
        cwd: SKELETON,
        encoding: 'utf8'
    });
}

test('the command and the form produce byte-identical plans', () => {
    const fixture = path.join(__dirname, 'fixtures', 'set-image.json');
    const body = JSON.parse(fs.readFileSync(fixture, 'utf8'));
    const before = fs.readFileSync(PLAN_PATH);
    const cli = runCli(fixture);
    const form = applyPosted(body);
    assert.equal(cli.status, 0, cli.stderr);
    assert.equal(form.conflict, undefined);
    assert.equal(cli.stdout, JSON.stringify(form.plan) + '\n');
    assert.equal(form.plan.revision, 2);
    assert.equal(
        form.plan.units[0].lessons[0].blocks[0].words[1].image.mediaId,
        'milk'
    );
    assert.equal(fs.readFileSync(PLAN_PATH).equals(before), true);
});

test('a stale revision conflicts on both paths', () => {
    const fixture = path.join(__dirname, 'fixtures', 'stale-revision.json');
    const body = JSON.parse(fs.readFileSync(fixture, 'utf8'));
    const cli = runCli(fixture);
    const form = applyPosted(body);
    assert.equal(cli.status, 1);
    assert.equal(cli.stdout, JSON.stringify({ conflict: form.conflict }) + '\n');
    assert.equal(form.conflict.error, 'revision_conflict');
    assert.equal(form.conflict.revision, 1);
    assert.equal(form.plan, undefined);
});
