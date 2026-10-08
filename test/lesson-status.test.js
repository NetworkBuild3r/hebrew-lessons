/**
 * HebrewUtils.lessonStatus — round status helper (SPEC-014 ac-6).
 * Provenance: INIT-002/SPEC-014
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const DAY_MS = 24 * 60 * 60 * 1000;

function loadUtils() {
    const sandbox = { console, window: {}, globalThis: {} };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'),
        sandbox
    );
    return sandbox.HebrewUtils;
}

describe('HebrewUtils.lessonStatus (SPEC-014 ac-6)', () => {
    const U = loadUtils();
    const now = Date.UTC(2026, 9, 8, 15, 0, 0);

    it('exports lessonStatus', () => {
        assert.equal(typeof U.lessonStatus, 'function');
    });

    it('new: missing or empty state', () => {
        const a = U.lessonStatus(null, now);
        const b = U.lessonStatus({}, now);
        assert.equal(a.kind, 'new');
        assert.equal(b.kind, 'new');
        assert.equal(a.chipEn, 'new');
        assert.match(a.he, /חדש/);
    });

    it('in progress: started round with incomplete activities', () => {
        const status = U.lessonStatus({
            lesson: 'demo',
            round: 1,
            roundStartedAt: now - 60 * 60 * 1000,
            stepIndex: 0,
            completedSteps: {}
        }, now, 3);
        assert.equal(status.kind, 'in_progress');
        assert.equal(status.chipEn, 'in progress');
        assert.match(status.chipHe, /בתהליך/);
        assert.equal(status.round, 1);
    });

    it('due again: 24 hours since roundStartedAt', () => {
        const status = U.lessonStatus({
            lesson: 'demo',
            round: 2,
            roundStartedAt: now - DAY_MS - 1000,
            stepIndex: -1,
            completedSteps: { 0: true, 1: true, 2: true }
        }, now, 3);
        assert.equal(status.kind, 'due');
        assert.equal(status.rank, 0);
        assert.equal(status.tone, 'ready');
        assert.match(status.chipEn, /round 2 due/i);
        assert.match(status.he, /מוכן לסיבוב נוסף/);
    });

    it('not yet due (done for now): all activities complete and waiting', () => {
        const status = U.lessonStatus({
            lesson: 'demo',
            round: 1,
            roundStartedAt: now - 2 * 60 * 60 * 1000,
            stepIndex: -1,
            completedSteps: { 0: true, 1: true, 2: true }
        }, now, 3);
        assert.equal(status.kind, 'done');
        assert.equal(status.chipEn, 'done for now');
        assert.match(status.chipHe, /מספיק לעכשיו/);
        assert.ok(status.dueAt > now);
        assert.equal(status.dueAt, now - 2 * 60 * 60 * 1000 + DAY_MS);
    });

    it('does not invent a second 24-hour rule (due boundary)', () => {
        const almost = U.lessonStatus({
            round: 1,
            roundStartedAt: now - DAY_MS + 1,
            stepIndex: -1,
            completedSteps: { 0: true }
        }, now, 1);
        const exact = U.lessonStatus({
            round: 1,
            roundStartedAt: now - DAY_MS,
            stepIndex: -1,
            completedSteps: { 0: true }
        }, now, 1);
        assert.equal(almost.kind, 'done');
        assert.equal(exact.kind, 'due');
    });
});
