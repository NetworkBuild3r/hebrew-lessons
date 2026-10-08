const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');

describe('shared word audio catalog', () => {
    it('maps קָם to the permanent shared clip', () => {
        const { rebuildWordCatalog } = require('../server/audio');
        rebuildWordCatalog();
        const file = path.join(root, 'worksheets', '_audio', 'words', 'catalog.js');
        assert.ok(fs.existsSync(file));
        const code = fs.readFileSync(file, 'utf8');
        assert.match(code, /window\.HEBREW_WORD_AUDIO/);
        assert.match(code, /קָם/);
        assert.match(code, /\.\.\/_audio\/words\/806bb3fc99c984531106aeb5\.mp3/);
    });
});
