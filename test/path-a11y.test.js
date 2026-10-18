/**
 * Path-map accessible names. The rendered count (exactly one aria-current="step")
 * is checked in go/internal/web/pages/learn_test.go against the fixture HTML.
 * This file guards the template and the label builder the npm suite can read.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');

describe('learning path accessible names', () => {
    const node = fs.readFileSync(path.join(ROOT, 'go/internal/web/views/components/path_node.templ'), 'utf8');
    const present = fs.readFileSync(path.join(ROOT, 'go/internal/web/path/present.go'), 'utf8');

    it('emits one current step and no disabled not-yet control', () => {
        assert.doesNotMatch(node, /aria-disabled/);
        assert.equal((node.match(/aria-current="step"/g) || []).length, 1);
        assert.match(node, /data-gate="warn"/);
        assert.match(node, /data-gate="open"/);
        assert.match(present, /not yet started/);
    });
});
