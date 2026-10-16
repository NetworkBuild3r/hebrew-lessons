/**
 * Test helper: a write context for direct calls to the plan operations
 * (INIT-008/SPEC-002). Every mutating function now takes a trailing context
 * built by an adapter. Tests that call the functions directly use this one,
 * so each existing call changes by the added argument only.
 */
'use strict';

const { makeContext } = require('../../server/plans/context');

/** A system-channel context, named for the test files. */
function testContext(name) {
    return makeContext({ channel: 'system', name: name || 'test', tool: 'test' });
}

const WRITES = [
    'replacePart',
    'insertBlock',
    'moveBlock',
    'removeBlock',
    'insertLesson',
    'moveLesson',
    'removeLesson',
    'setField',
    'assignPlan'
];

/**
 * A copy of the plan operations whose writes get ctx as the last argument.
 * Reads and apply() pass through. Spies that replace a function on the real
 * module are seen, because each call looks the function up at call time.
 * @param {object} ops The server/plans/ops module.
 * @param {object} [ctx]
 * @returns {object}
 */
function withCtx(ops, ctx) {
    const context = ctx || testContext('ops-test');
    const out = {};
    for (const name of Object.keys(ops)) {
        if (WRITES.indexOf(name) !== -1) {
            out[name] = function (store, id, input) { return ops[name](store, id, input, context); };
        } else {
            out[name] = function () { return ops[name].apply(ops, arguments); };
        }
    }
    return out;
}

/**
 * publishPlan with ctx added.
 * @param {Function} publishPlan
 * @param {object} [ctx]
 * @returns {Function}
 */
function publishWithCtx(publishPlan, ctx) {
    const context = ctx || testContext('publish-test');
    return function (store, id, input) { return publishPlan(store, id, input, context); };
}

module.exports = { testContext, withCtx, publishWithCtx };
