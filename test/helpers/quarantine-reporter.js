/**
 * Second node:test reporter. The spec reporter still prints the run.
 * This one records tests that were skipped with a reason, so `npm test`
 * can fail when a quarantine.json entry was not skipped.
 *
 * node --test --test-reporter spec --test-reporter-destination stdout \
 *   --test-reporter test/helpers/quarantine-reporter.js \
 *   --test-reporter-destination <file>
 */
'use strict';

module.exports = async function* quarantineReporter(source) {
    const lines = [];
    for await (const event of source) {
        const data = event && event.data;
        if (!data || event.type !== 'test:pass') continue;
        if (typeof data.skip !== 'string' || data.skip.length === 0) continue;
        lines.push(JSON.stringify({ file: data.file, name: data.name, skip: data.skip }));
    }
    yield lines.join('\n') + (lines.length ? '\n' : '');
};
