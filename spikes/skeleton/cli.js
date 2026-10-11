'use strict';

/**
 * node cli.js <op-json>
 * Loads the beginner-unit sample and calls ops.applyBody (the same function the form uses).
 * Provenance: INIT-003/SPEC-009
 */

const fs = require('fs');
const { applyBody, loadPlan } = require('./ops');

function run(opPath) {
    const body = JSON.parse(fs.readFileSync(opPath, 'utf8'));
    return applyBody(loadPlan(), body);
}

function main() {
    const opPath = process.argv[2];
    if (!opPath) {
        process.stderr.write('usage: node cli.js <op-json>\n');
        return 2;
    }
    const result = run(opPath);
    if (result.conflict) {
        process.stdout.write(JSON.stringify({ conflict: result.conflict }) + '\n');
        return 1;
    }
    process.stdout.write(JSON.stringify(result.plan) + '\n');
    return 0;
}

if (require.main === module) {
    process.exit(main());
}

module.exports = { main, run };
