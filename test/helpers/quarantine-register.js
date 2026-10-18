/**
 * Preload for `node --test` child processes (`--require`).
 *
 * Returns a wrapped node:test from require() before the test file loads, so a
 * destructured `it` / `test` still goes through the quarantine wrapper.
 * The parent runner is left alone: only children set NODE_TEST_CONTEXT.
 */
'use strict';

const testContext = process.env.NODE_TEST_CONTEXT || '';
if (testContext === 'child' || testContext.startsWith('child-')) {
    const fs = require('fs');
    const path = require('path');
    const quarantine = require('./quarantine');

    const Module = require('module');
    const origRequire = Module.prototype.require;
    const entries = quarantine.loadList();
    const seenDir = process.env.HEBREW_QUARANTINE_SEEN;

    function record(file, name) {
        if (!seenDir) return;
        fs.mkdirSync(seenDir, { recursive: true });
        fs.appendFileSync(
            path.join(seenDir, process.pid + '.jsonl'),
            JSON.stringify({ file, name }) + '\n'
        );
    }

    // node:test is a builtin and is not stored in require.cache in test
    // children, so the wrapper has to be returned from require() itself.
    const wrapped = quarantine.install(origRequire.call(module, 'node:test'), entries, record);
    Module.prototype.require = function (request) {
        if (request === 'node:test') return wrapped;
        return origRequire.apply(this, arguments);
    };
}
