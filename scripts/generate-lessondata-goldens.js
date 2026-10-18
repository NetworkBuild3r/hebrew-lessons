#!/usr/bin/env node
/**
 * Writes go/internal/lessondata/testdata/golden from the real
 * server/lesson-data.js parser and JSON.stringify writers.
 *
 *   node scripts/generate-lessondata-goldens.js
 *
 * Covers every worksheets/**\/*.js file, a mutation corpus of rejected
 * inputs, and accepted inputs that exercise JSON.stringify quirks
 * (key order, number format, U+2028/U+2029, lone surrogates).
 *
 * Layout under go/internal/lessondata/testdata/golden/:
 *   meta.json          node version, lesson-data.js sha, counts
 *   mutations.jsonl    one rejected input per line
 *   quirks.jsonl       one accepted synthetic input per line
 *   worksheets.jsonl   one real worksheet per line (emit and assign text)
 *   written/...        indented writer bytes, one file per worksheet
 *
 * errorJSON is JSON.stringify(message): ASCII, so a lone surrogate in the
 * message survives a Go encoding/json unmarshal as the characters \udXXX.
 */
'use strict';

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { parseDataWindow } = require('../server/lesson-data');

const root = path.resolve(__dirname, '..');
const golden = path.join(root, 'go', 'internal', 'lessondata', 'testdata', 'golden');

function walkJS(dir, rel, out) {
    const ents = fs.readdirSync(dir, { withFileTypes: true });
    ents.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
    for (const ent of ents) {
        const childRel = rel ? rel + '/' + ent.name : ent.name;
        const child = path.join(dir, ent.name);
        if (ent.isDirectory()) walkJS(child, childRel, out);
        else if (ent.isFile() && ent.name.endsWith('.js')) out.push(childRel);
    }
}

function canonical(globals) {
    let s = '';
    for (const name of Object.keys(globals)) {
        s += 'window.' + name + ' = ' + JSON.stringify(globals[name], null, 4) + ';\n';
    }
    return s;
}

function emitAll(globals) {
    let s = '';
    for (const name of Object.keys(globals)) {
        s += 'window.' + name + ' = ' + JSON.stringify(globals[name]).replace(/</g, '\\u003c') + ';\n';
    }
    return s;
}

function assignAll(globals) {
    let s = '';
    for (const name of Object.keys(globals)) {
        s += 'window.' + name + ' = Object.assign({}, window.' + name + ' || {}, '
            + JSON.stringify(globals[name], null, 4) + ');\n';
    }
    return s;
}

function assignParses(globals) {
    const names = Object.keys(globals);
    return names.every((name) => {
        const v = globals[name];
        return v !== null && typeof v === 'object' && !Array.isArray(v);
    });
}

function evaluate(rec) {
    const text = Buffer.isBuffer(rec.src) ? rec.src.toString('utf8') : rec.src;
    let globals;
    try {
        globals = parseDataWindow(text, rec.label);
    } catch (err) {
        if (!(err instanceof SyntaxError)) throw err;
        if (rec.kind !== 'mutation') {
            throw new Error(rec.kind + ' ' + rec.id + ' failed: ' + err.message);
        }
        return {
            id: rec.id,
            label: rec.label,
            input: text,
            errorJSON: JSON.stringify(err.message)
        };
    }
    if (rec.kind === 'mutation') {
        throw new Error('mutation parsed successfully: ' + rec.id);
    }
    const written = canonical(globals);
    const emit = emitAll(globals);
    const assign = assignAll(globals);
    const parses = assignParses(globals);
    if (canonical(parseDataWindow(written, rec.label)) !== written) {
        throw new Error('indented writer is not idempotent: ' + rec.id);
    }
    if (canonical(parseDataWindow(emit, rec.label)) !== written) {
        throw new Error('emit script changed the value: ' + rec.id);
    }
    if (parses && canonical(parseDataWindow(assign, rec.label)) !== written) {
        throw new Error('assign script changed the value: ' + rec.id);
    }
    return {
        id: rec.id,
        label: rec.label,
        input: text,
        assignParses: parses,
        written,
        emit,
        assign,
        repo: rec.repo || ''
    };
}

function writeJSONL(name, rows) {
    const body = rows.map((row) => JSON.stringify(row)).join('\n') + (rows.length ? '\n' : '');
    fs.writeFileSync(path.join(golden, name), body);
}

const mutations = [
    ['unterminated-comment', 'window.X = 1; /* hi'],
    ['unterminated-comment-start', '/*'],
    ['slash-star-slash', 'window.X = 1; /*/'],
    ['newline-double', 'window.X = "ab\ncd";'],
    ['newline-single', "window.X = 'ab\ncd';"],
    ['template-interp', 'window.X = `${1}`;'],
    ['template-interp-later', 'window.X = `a${b}`;'],
    ['unterminated-double', 'window.X = "abc'],
    ['unterminated-single', "window.X = 'abc"],
    ['unterminated-backtick', 'window.X = `abc'],
    ['unterminated-escape', 'window.X = "abc\\'],
    ['bad-u-short', 'window.X = "\\u12";'],
    ['bad-u-hex', 'window.X = "\\uZZZZ";'],
    ['bad-u-end', 'window.X = "\\u";'],
    ['bad-u-brace', 'window.X = "\\u{41}";'],
    ['bad-x-short', 'window.X = "\\x1";'],
    ['bad-x-hex', 'window.X = "\\xZZ";'],
    ['bad-x-end', 'window.X = "\\x";'],
    ['line-cont-lf', 'window.X = "ab\\\ncd";'],
    ['line-cont-cr', 'window.X = "ab\\\rcd";'],
    ['unexpected-plus', 'window.X = 1 + 2;'],
    ['unexpected-amp', 'window.X = a && b;'],
    ['unexpected-slash', 'window.X = 1/2;'],
    ['unexpected-hebrew', 'window.X = {ש: 1};'],
    ['unexpected-at', 'window.X = @;'],
    ['unexpected-backslash', 'window.X = \\;'],
    ['unexpected-nul', 'window.X = \u0000;'],
    ['unexpected-nel', 'window.X =\u00851;'],
    ['unexpected-emoji', 'window.X = 😀;'],
    ['nonfinite', 'window.X = 1e309;'],
    ['nonfinite-neg', 'window.X = -1e309;'],
    ['nonfinite-huge', 'window.X = 1e999;'],
    ['ident-undefined', 'window.X = undefined;'],
    ['ident-require', "window.X = { fs: require('fs') };"],
    ['ident-process', 'window.X = process.env;'],
    ['ident-truee', 'window.X = truee;'],
    ['ident-True', 'window.X = True;'],
    ['eof-value', 'window.X ='],
    ['unexpected-semi', 'window.X = ;'],
    ['unexpected-paren', 'window.X = (function () { return 1; })();'],
    ['unexpected-dot', 'window.X = .5;'],
    ['key-number', 'window.X = {1:2};'],
    ['key-punct', 'window.X = {:1};'],
    ['missing-colon', 'window.X = {a 1};'],
    ['missing-brace', 'window.X = {a:1'],
    ['missing-bracket', 'window.X = [1, 2'],
    ['missing-value-key', 'window.X = {a:1, b}'],
    ['empty-comma-array', 'window.X = [,];'],
    ['double-comma-object', 'window.X = {a:1,,};'],
    ['hole-array', 'window.X = [1,,2];'],
    ['not-window', 'foo = 1;'],
    ['global-this', 'globalThis.X = {};'],
    ['window-capital', 'Window.X = 1;'],
    ['missing-name', 'window. = 1;'],
    ['name-number', 'window.1 = 2;'],
    ['name-string', 'window."A" = 1;'],
    ['name-eof', 'window.'],
    ['missing-dot', 'window = 1;'],
    ['missing-eq', 'window.X 1;'],
    ['merge-array', 'window.X = Object.assign({}, window.X || {}, [1, 2]);'],
    ['merge-wrong-name', 'window.X = Object.assign({}, window.Y || {}, { a: 1 });'],
    ['merge-process', 'window.X = Object.assign(process, {});'],
    ['merge-process-env', 'window.X = Object.assign({}, process.env);'],
    ['merge-extra-arg', 'window.X = Object.assign({}, window.X || {}, {a:1}, {b:2});'],
    ['merge-unclosed', 'window.X = Object.assign({}, window.X || {}, {a:1}'],
    ['merge-one-pipe', 'window.X = Object.assign({}, window.X | {}, {a:1});'],
    ['merge-missing-source', 'window.X = Object.assign({}, window.X || {}, );'],
    ['arithmetic', 'window.X = 1 + 1;'],
    ['extra-statement', "window.X = {}; require('child_process');"],
    ['call', 'window.X = foo();'],
    ['unary-plus', 'window.X = +1;'],
    ['trailing-minus', 'window.X = -'],
    ['minus-dot', 'window.X = -.;'],
    ['hex', 'window.X = 0x10;'],
    ['num-then-ident', 'window.X = 1a;'],
    ['sandbox-escape', "window.X = this.constructor.constructor('return process')();"],
    ['sandbox-typeof', "window.X = this.constructor.constructor('return typeof process')();"],
    ['trailing-junk', 'window.X = 1; junk'],
    ['regex-like', 'window.X = /a/;'],
    ['nested-unclosed', 'window.X = {a: {b: 1}'],
    ['array-unclosed-nested', 'window.X = [1, {a: 2]']
];

const quirks = [
    ['negative-zero', 'window.X = -0;'],
    ['negative-zero-dot', 'window.X = -0.0;'],
    ['numbers', 'window.X = [1., 1.e2, 1.E2, -.5, 00.10, 1e2, 1e+2, 1E-2, 1.50, 100.0, 0.0, 01, 1e21, 1e20, 1e-6, 1e-7, 1e-10, 9007199254740993, 1.0000000000000002, 5.e1, 1.2300, 2.2250738585072014e-308, 1.7976931348623157e+308, 5e-324, 1234567890123456789, 1e308, 1e-323, 1e-324, -1e-324, 9999999999999999, 0.30000000000000004];'],
    ['line-separators', 'window.X = "a\u2028b\u2029c";'],
    ['lone-high', 'window.X = "\\uD800";'],
    ['lone-low', 'window.X = "\\uDEAD";'],
    ['lone-dfff', 'window.X = "\\uDFFF";'],
    ['two-high', 'window.X = "\\uD800\\uD800";'],
    ['surrogate-pair-esc', 'window.X = "\\uD83D\\uDE00";'],
    ['surrogate-pair-raw', 'window.X = "😀";'],
    ['script-and-html', 'window.X = "</script><&>";'],
    ['controls', 'window.X = "\\u0000\\u0001\\b\\f\\n\\r\\t\\v\\0";'],
    ['raw-controls', 'window.X = "\u0001\u0007";'],
    ['del-and-nel', 'window.X = "\u007f\u0085";'],
    ['proto', 'window.P = { "__proto__": { polluted: true } };'],
    ['proto-ident', 'window.P = { __proto__: { polluted: true }, z: 1 };'],
    ['index-keys', 'window.X = {b:1, "10":2, a:3, "2":4, "4294967295":5, "4294967294":6, "01":7};'],
    ['index-key-edges', 'window.X = {"": 1, "1a": 2, "0": 3, "123456789012": 4};'],
    ['dup-keys', 'window.X = {a:1, b:2, a:3};'],
    ['escapes', "window.H = { word: 'קָם', esc: 'a\\n\\u05D0\\x41\\'' };"],
    ['backtick-nl', 'window.X = `a\nb`;'],
    ['escaped-interp', 'window.X = `\\${1}`;'],
    ['single-dollar', "window.X = '${no}';"],
    ['comments-bom', '\uFEFF// header\n/* block */\nwindow.C = {\n  // inside\n  key: "v", /* more */\n};\n'],
    ['double-bom', '\uFEFF\uFEFFwindow.X = 1;'],
    ['nbsp', 'window.X\u00A0=\u00A01;'],
    ['formfeed', 'window.X\f=\f1;'],
    ['ideographic-space', 'window.X\u3000=\u30001;'],
    ['optional-semi', 'window.A = 1 window.B = 2'],
    ['reassign-order', 'window.B = 1; window.A = 2; window.B = {x:3};'],
    ['merge-accum', 'window.M = Object.assign({}, window.M || {}, {\n    "a": "../_audio/words/1.mp3"\n});\nwindow.M = Object.assign({}, window.M || {}, {\n    "b": "../_audio/words/2.mp3"\n});\n'],
    ['merge-over-array', 'window.X = [1,2]; window.X = Object.assign({}, window.X || {}, {a:1});'],
    ['merge-over-string', 'window.X = "hi"; window.X = Object.assign({}, window.X || {}, {a:1});'],
    ['merge-over-null', 'window.X = null; window.X = Object.assign({}, window.X || {}, {a:1});'],
    ['merge-over-number', 'window.X = 1; window.X = Object.assign({}, window.X || {}, {a:1});'],
    ['merge-comments', 'window.M = Object /*c*/ . assign ( { } , window . M || { } , { a : 1 } ) ;'],
    ['merge-then-plain', 'window.M = {a:1}; window.M = Object.assign({}, window.M || {}, {b:2});'],
    ['dollar-ident', 'window.$ = { _a: 1, $b: 2 }; window._ = null;'],
    ['bool-null-keys', 'window.X = {true: false, null: 1, false: true};'],
    ['string-keys', 'window.X = {"a-b": 1, "c d": 2, "קָם": 3};'],
    ['empty-containers', 'window.X = {a:{}, b:[], c:[[], {}]};'],
    ['nested', 'window.X = {a:[1,{b:null,c:[true,false]}]};'],
    ['cr-in-string', 'window.X = "a\rb";'],
    ['cr-does-not-end-comment', '// c\rwindow.X = 1;'],
    ['comment-swallows-ls', '// hello\u2028window.X = 1;\nwindow.Y = 2;'],
    ['empty', ''],
    ['comments-only', '// hi\n/* x */'],
    ['whitespace-only', '  \n\t\r '],
    ['trailing-commas', 'window.X = {a:1,}; window.Y = [1,];'],
    ['block-between', 'window.X = /**/ { /**/ a /**/ : /**/ 1 /**/ , /**/ };'],
    ['url', 'window.X = "http://x";'],
    ['quote-key', 'window.X = {"a\\"b": 1, "c\\\\d": 2};'],
    ['danger-keys', 'window.X = {constructor: 1, prototype: 2, toString: 3};'],
    ['mixed-quotes', "window.A = { id: 'x', n: 1, neg: -2.5e1, ok: true, nothing: null, list: [1, 'two', [3]] };\nwindow.B = [ \"one\", `two`, ];\n"],
    ['identity-escape', 'window.X = "\\q\\/\\\'";'],
    ['deep', 'window.X = {a:{b:{c:{d:[1,2,3]}}}};'],
    ['two-globals-assign', 'window.A = {a:1}; window.B = Object.assign({}, window.B || {}, {b:2});'],
    ['whitespace-vt', 'window.X\v=\v[1,\v2];'],
    ['ogham-space', 'window.X\u1680=\u16801;'],
    ['slash-in-string', 'window.X = "a/b//c/*d*/";'],
    ['empty-string', 'window.X = "";'],
    ['true-false-null', 'window.X = [true, false, null];'],
    ['max-safe-neighbors', 'window.X = [9007199254740991, 9007199254740992, 9007199254740993];'],
    ['ls-between-tokens', 'window.X\u2028=\u20281;']
];

fs.rmSync(golden, { recursive: true, force: true });
fs.mkdirSync(golden, { recursive: true });

const files = [];
walkJS(path.join(root, 'worksheets'), 'worksheets', files);
const worksheetRows = [];
for (const rel of files) {
    const got = evaluate({
        id: rel,
        kind: 'file',
        label: rel,
        repo: rel,
        src: fs.readFileSync(path.join(root, rel))
    });
    if (got.errorJSON) throw new Error('file case failed: ' + rel);
    const writtenRel = 'written/' + rel.replace(/\.js$/, '.written.js');
    fs.mkdirSync(path.dirname(path.join(golden, writtenRel)), { recursive: true });
    fs.writeFileSync(path.join(golden, writtenRel), got.written);
    worksheetRows.push({
        repo: rel,
        label: rel,
        assignParses: got.assignParses,
        written: writtenRel,
        emit: got.emit,
        assign: got.assign
    });
}

const mutationRows = [];
for (const [id, src] of mutations) {
    const got = evaluate({
        id,
        kind: 'mutation',
        label: 'mutations/' + id + '.js',
        src
    });
    if (!got.errorJSON) throw new Error('mutation was not an error: ' + id);
    mutationRows.push({ id, label: got.label, input: got.input, errorJSON: got.errorJSON });
}

const quirkRows = [];
for (const [id, src] of quirks) {
    const got = evaluate({
        id,
        kind: 'quirk',
        label: 'quirks/' + id + '.js',
        src
    });
    if (got.errorJSON) throw new Error('quirk failed: ' + id);
    quirkRows.push({
        id,
        label: got.label,
        input: got.input,
        assignParses: got.assignParses,
        written: got.written,
        emit: got.emit,
        assign: got.assign
    });
}

writeJSONL('worksheets.jsonl', worksheetRows);
writeJSONL('mutations.jsonl', mutationRows);
writeJSONL('quirks.jsonl', quirkRows);

const lessonPath = path.join(root, 'server', 'lesson-data.js');
const lessonSha = crypto.createHash('sha256').update(fs.readFileSync(lessonPath)).digest('hex');
const meta = {
    node: process.version,
    lessonDataSha256: lessonSha,
    generator: 'scripts/generate-lessondata-goldens.js',
    files: worksheetRows.length,
    mutations: mutationRows.length,
    quirks: quirkRows.length
};
fs.writeFileSync(path.join(golden, 'meta.json'), JSON.stringify(meta, null, 2) + '\n');
console.log(JSON.stringify(meta));
