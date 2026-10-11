/**
 * Cross-checks fsrs6-golden.json against a second reference implementation, ts-fsrs.
 *
 * Run OUTSIDE the repository, with ts-fsrs installed in a throwaway folder, so it
 * never becomes a dependency of this project:
 *
 *     npm install --prefix <tmp> ts-fsrs@5.4.2
 *     NODE_PATH=<tmp>/node_modules node test/fixtures/fsrs/crosscheck-ts-fsrs.cjs test/fixtures/fsrs/fsrs6-golden.json
 *
 * For every step it feeds the fixture's previous stability and difficulty to
 * ts-fsrs's FSRSAlgorithm.next_state, compares the result with the fixture, and
 * writes the largest differences into the fixture's `crossCheck` block. It exits 1
 * if any relative difference is above 1e-6. npm test never runs this file.
 *
 * Authored-by-spec: INIT-006/SPEC-004
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { FSRSAlgorithm, generatorParameters } = require('ts-fsrs');

const EXPECTED_VERSION = '5.4.2';
const MAX_RELATIVE_DIFF = 1e-6;

function packageVersion() {
    let dir = path.dirname(require.resolve('ts-fsrs'));
    for (;;) {
        const file = path.join(dir, 'package.json');
        if (fs.existsSync(file)) {
            const pkg = JSON.parse(fs.readFileSync(file, 'utf8'));
            if (pkg.name === 'ts-fsrs') return pkg.version;
        }
        const parent = path.dirname(dir);
        if (parent === dir) throw new Error('ts-fsrs package.json not found');
        dir = parent;
    }
}

function relative(a, b) {
    return Math.abs(a - b) / Math.max(1, Math.abs(b));
}

function main() {
    const file = process.argv[2];
    if (!file) throw new Error('usage: crosscheck-ts-fsrs.cjs <fsrs6-golden.json>');
    const version = packageVersion();
    if (version !== EXPECTED_VERSION) {
        throw new Error('expected ts-fsrs ' + EXPECTED_VERSION + ', found ' + version);
    }
    const fixture = JSON.parse(fs.readFileSync(file, 'utf8'));
    const algorithm = new FSRSAlgorithm(generatorParameters({
        w: fixture.parameters,
        request_retention: fixture.desiredRetention,
        maximum_interval: fixture.intervalDays.max,
        enable_fuzz: false,
        enable_short_term: true,
        learning_steps: [],
        relearning_steps: []
    }));
    const used = Array.from(algorithm.parameters.w);
    if (used.length !== fixture.parameters.length
        || used.some((value, i) => value !== fixture.parameters[i])) {
        throw new Error('ts-fsrs clipped or changed the parameters: ' + JSON.stringify(used));
    }

    const worst = { stability: 0, difficulty: 0, retrievability: 0 };
    let steps = 0;
    for (const testCase of fixture.cases) {
        let memory = null;
        for (const step of testCase.steps) {
            const t = step.elapsedDays == null ? 0 : step.elapsedDays;
            const next = algorithm.next_state(memory, t, step.grade);
            worst.stability = Math.max(worst.stability, relative(next.stability, step.stability));
            worst.difficulty = Math.max(worst.difficulty, relative(next.difficulty, step.difficulty));
            if (memory && step.retrievability != null) {
                const r = algorithm.forgetting_curve(t, memory.stability);
                worst.retrievability = Math.max(worst.retrievability, relative(r, step.retrievability));
            }
            memory = { stability: step.stability, difficulty: step.difficulty };
            steps += 1;
        }
    }

    fixture.crossCheck = {
        name: 'ts-fsrs',
        version,
        url: 'https://www.npmjs.com/package/ts-fsrs/v/' + version,
        function: 'FSRSAlgorithm.next_state, enable_short_term true, no learning steps',
        steps,
        maxRelativeDiff: worst,
        tolerance: MAX_RELATIVE_DIFF,
        note: 'ts-fsrs rounds intermediate values to 8 decimals and py-fsrs does not. '
            + 'Intervals are not compared: ts-fsrs orders its button intervals by a different rule.',
        ranOutsideRepository: true,
        script: 'test/fixtures/fsrs/crosscheck-ts-fsrs.cjs'
    };
    fs.writeFileSync(file, JSON.stringify(fixture, null, 2) + '\n', 'utf8');
    console.log('ts-fsrs ' + version + ': ' + steps + ' steps, max relative diff ' + JSON.stringify(worst));
    const over = Object.keys(worst).filter((key) => worst[key] > MAX_RELATIVE_DIFF);
    if (over.length) {
        console.error('Cross-check failed for: ' + over.join(', '));
        process.exitCode = 1;
    }
}

main();
