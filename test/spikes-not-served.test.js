/**
 * INIT-003/SPEC-006 — spikes/ is a local mockup folder.
 * The web server must not resolve it, and the Docker image must not copy it.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { publicFile, resolveFile } = require('../server/web.js');
const { servedRoots, rootDir } = require('../server/paths.js');

describe('spikes are not served', () => {
    it('resolveFile and publicFile do not map /spikes/', () => {
        assert.equal(publicFile('/spikes/look/a-calm-type/today.html'), null);
        assert.equal(publicFile('/spikes/look/check.js'), null);
        assert.equal(publicFile('/spikes/'), null);
        assert.equal(resolveFile('/spikes/look/b-warm-illustrated/lesson-step.html'), null);
        assert.equal(resolveFile('/spikes/look/c-dense-tool/look.css'), null);
    });

    it('served roots do not contain the spikes directory', () => {
        const spikes = path.resolve(rootDir(), 'spikes');
        for (const root of servedRoots()) {
            const base = path.resolve(root);
            const prefix = base.endsWith(path.sep) ? base : base + path.sep;
            assert.notEqual(spikes, base);
            assert.equal(spikes.startsWith(prefix), false, spikes + ' must not sit inside ' + base);
        }
    });

    it('.dockerignore excludes spikes/', () => {
        const text = fs.readFileSync(path.join(rootDir(), '.dockerignore'), 'utf8');
        const lines = text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line && !line.startsWith('#'));
        assert.ok(
            lines.some((line) => line === 'spikes' || line === 'spikes/' || line === 'spikes/**'),
            'dockerignore lines: ' + lines.join(', ')
        );
    });
});
