/**
 * Lesson data loader tests — INIT-001 security remediation (vm sandbox escape, HIGH-1).
 * Lesson and catalog files must be parsed as literals and never executed.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { parseDataWindow } = require('../server/lesson-data');

const root = path.resolve(__dirname, '..');

describe('lesson data loader: accepted literal data', () => {
    it('parses window.NAME = literal assignments', () => {
        const win = parseDataWindow(
            "window.A = { id: 'x', n: 1, neg: -2.5e1, ok: true, nothing: null, list: [1, 'two', [3]] };\n" +
            'window.B = [ "one", `two`, ];\n',
            'mem'
        );
        assert.deepEqual(win.A, { id: 'x', n: 1, neg: -25, ok: true, nothing: null, list: [1, 'two', [3]] });
        assert.deepEqual(win.B, ['one', 'two']);
    });

    it('accepts comments, trailing commas, and a leading BOM', () => {
        const win = parseDataWindow(
            '\uFEFF// header comment\n/* block */\nwindow.C = {\n  // inside\n  key: "v", /* more */\n};\n',
            'mem'
        );
        assert.deepEqual(win.C, { key: 'v' });
    });

    it('decodes common string escapes and keeps Hebrew text intact', () => {
        const win = parseDataWindow("window.H = { word: 'קָם', esc: 'a\\n\\u05D0\\x41\\'' };", 'mem');
        assert.equal(win.H.word, 'קָם');
        assert.equal(win.H.esc, "a\nאA'");
    });

    it('accepts the generated catalog merge form and accumulates across files', () => {
        const win = parseDataWindow(
            'window.M = Object.assign({}, window.M || {}, {\n    "a": "../_audio/words/1.mp3"\n});\n'
            + 'window.M = Object.assign({}, window.M || {}, {\n    "b": "../_audio/words/2.mp3"\n});\n',
            'mem'
        );
        assert.deepEqual(win.M, { a: '../_audio/words/1.mp3', b: '../_audio/words/2.mp3' });
    });

    it('keeps a __proto__ key as an own property without changing the prototype', () => {
        const win = parseDataWindow('window.P = { "__proto__": { polluted: true } };', 'mem');
        assert.ok(Object.prototype.hasOwnProperty.call(win.P, '__proto__'));
        assert.equal({}.polluted, undefined);
    });
});

describe('lesson data loader: code is rejected, never executed', () => {
    const attacks = {
        'sandbox escape via constructor chain':
            "window.X = this.constructor.constructor('return process')();",
        'require call as a value':
            "window.X = { fs: require('fs') };",
        'process.env as a value':
            'window.X = process.env;',
        'arithmetic expression':
            'window.X = 1 + 1;',
        'extra statement after the data':
            "window.X = {}; require('child_process');",
        'non-window assignment':
            'globalThis.X = {};',
        'template interpolation':
            'window.X = `${process.pid}`;',
        'IIFE':
            'window.X = (function () { return 1; })();',
        'unterminated object':
            'window.X = { a: 1',
        'Object.assign onto process':
            'window.X = Object.assign(process, {});',
        'Object.assign with process.env source':
            'window.X = Object.assign({}, process.env);',
        'merge form naming a different global':
            'window.X = Object.assign({}, window.Y || {}, { a: 1 });',
        'merge form with a non-object source':
            'window.X = Object.assign({}, window.X || {}, [1, 2]);',
    };

    for (const [label, code] of Object.entries(attacks)) {
        it('rejects: ' + label, () => {
            assert.throws(() => parseDataWindow(code, 'attack.js'), SyntaxError);
        });
    }

    it('does not evaluate the escape payload (no process object reached)', () => {
        const payload = "window.X = this.constructor.constructor('return typeof process')();";
        assert.throws(() => parseDataWindow(payload, 'payload.js'), SyntaxError);
    });
});

describe('lesson data loader: real repo data files', () => {
    const files = [];
    const worksheets = path.join(root, 'worksheets');
    if (fs.existsSync(worksheets)) {
        for (const entry of fs.readdirSync(worksheets, { withFileTypes: true })) {
            if (!entry.isDirectory() || entry.name.startsWith('_') || entry.name.startsWith('.')) continue;
            const file = path.join(worksheets, entry.name, 'worksheet.js');
            if (fs.existsSync(file)) files.push(file);
        }
    }
    for (const rel of [
        path.join('worksheets', 'catalog.js'),
        path.join('worksheets', '_audio', 'words', 'catalog.js'),
        path.join('shared', 'word-audio-catalog.js'),
    ]) {
        const file = path.join(root, rel);
        if (fs.existsSync(file)) files.push(file);
    }

    it('parses every lesson worksheet and catalog in the repo as data', () => {
        assert.ok(files.length >= 2, 'expected repo lesson and catalog files');
        for (const file of files) {
            const parsed = parseDataWindow(fs.readFileSync(file, 'utf8'), file);
            assert.ok(Object.keys(parsed).length >= 1, 'no window globals in ' + file);
        }
    });
});
