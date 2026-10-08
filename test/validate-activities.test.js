const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('child_process');
const path = require('path');

const root = path.resolve(__dirname, '..');
const validate = path.join(root, '.cursor', 'skills', 'hebrew-worksheet', 'scripts', 'validate.js');

function run(slug) {
    return spawnSync(process.execPath, [validate, slug], {
        cwd: root,
        encoding: 'utf8',
        env: Object.assign({}, process.env, { HEBREW_ROOT: root })
    });
}

describe('validator', () => {
    it('accepts my-morning with activities', () => {
        const result = run('my-morning');
        assert.equal(result.status, 0, result.stderr || result.stdout);
        assert.match(result.stdout, /OK my-morning/);
    });

    it('accepts shai without activities (legacy fallback)', () => {
        const result = run('shai-date-indirect-speech');
        assert.equal(result.status, 0, result.stderr || result.stdout);
        assert.match(result.stdout, /OK shai-date-indirect-speech/);
    });

    it('exports activity catalog for MCP', () => {
        const { CATALOG, LEVEL_DEFAULTS } = require('../shared/activities/catalog');
        assert.ok(CATALOG.some(item => item.type === 'speed-quiz'));
        assert.ok(CATALOG.some(item => item.type === 'build-sentence'));
        assert.ok(CATALOG.some(item => item.type === 'listen-pick'));
        assert.ok(CATALOG.some(item => item.type === 'type-it'));
        assert.ok(LEVEL_DEFAULTS.basic);
        assert.ok(LEVEL_DEFAULTS.advanced);
    });
});
