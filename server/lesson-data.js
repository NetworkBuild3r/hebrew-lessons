/**
 * Data-only loader for lesson worksheets and generated catalogs.
 *
 * Lesson worksheet.js files and the catalog files are data: each is one or more
 * `window.NAME = <literal>;` assignments. This module parses that literal grammar
 * directly and never executes the file.
 *
 * Why not vm: Node's `vm` module is not a security boundary. Code running in a vm
 * context can reach the host `process` object (for example through
 * `this.constructor.constructor`), including `process.env`. Lesson text can come
 * from transcripts or notes, so it must never be evaluated as code.
 *
 * Accepted grammar:
 *   - object and array literals, with trailing commas allowed
 *   - string literals in single, double, or backtick quotes (no `${` interpolation)
 *   - finite number literals, true, false, null
 *   - object keys that are bare identifiers or string literals
 *   - // and block comments
 * Anything else (calls, identifiers as values, operators, other statements) throws.
 *
 * Trusted repo code (for example shared/player/utils.js, which is an IIFE) is NOT
 * loaded here; callers that need it keep their own explicit vm loader.
 */
const fs = require('fs');

const PUNCT = new Set(['{', '}', '[', ']', ':', ',', ';', '.', '=', '(', ')', '|']);
const NUMBER_RE = /-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?/y;
const IDENT_RE = /[A-Za-z_$][\w$]*/y;
const SIMPLE_ESCAPES = { n: '\n', t: '\t', r: '\r', b: '\b', f: '\f', v: '\v', '0': '\0' };

function define(target, key, value) {
    // defineProperty so a key such as "__proto__" stays an own property and cannot
    // change the prototype of the object being built.
    Object.defineProperty(target, key, { value, enumerable: true, writable: true, configurable: true });
    return target;
}

function tokenize(src, file) {
    const tokens = [];
    let i = 0;
    const fail = (message) => {
        throw new SyntaxError(file + ': ' + message + ' at offset ' + i);
    };

    while (i < src.length) {
        const ch = src[i];
        if (/\s/.test(ch)) { i += 1; continue; }
        if (src.startsWith('//', i)) {
            const nl = src.indexOf('\n', i);
            i = nl === -1 ? src.length : nl + 1;
            continue;
        }
        if (src.startsWith('/*', i)) {
            const end = src.indexOf('*/', i + 2);
            if (end === -1) fail('unterminated comment');
            i = end + 2;
            continue;
        }
        if (PUNCT.has(ch)) {
            tokens.push({ type: 'punct', value: ch });
            i += 1;
            continue;
        }
        if (ch === '"' || ch === "'" || ch === '`') {
            const read = readString(src, i, fail);
            tokens.push({ type: 'string', value: read.value });
            i = read.next;
            continue;
        }
        NUMBER_RE.lastIndex = i;
        const num = NUMBER_RE.exec(src);
        if (num) {
            tokens.push({ type: 'number', value: num[0] });
            i += num[0].length;
            continue;
        }
        IDENT_RE.lastIndex = i;
        const ident = IDENT_RE.exec(src);
        if (ident) {
            tokens.push({ type: 'ident', value: ident[0] });
            i += ident[0].length;
            continue;
        }
        fail('unexpected character "' + ch + '"');
    }
    return tokens;
}

function readString(src, start, fail) {
    const quote = src[start];
    let out = '';
    let i = start + 1;
    while (i < src.length) {
        const ch = src[i];
        if (ch === quote) return { value: out, next: i + 1 };
        if (ch === '\n' && quote !== '`') fail('newline inside string literal');
        if (quote === '`' && src.startsWith('${', i)) fail('template interpolation is not allowed');
        if (ch !== '\\') {
            out += ch;
            i += 1;
            continue;
        }
        const next = src[i + 1];
        if (next === undefined) fail('unterminated string literal');
        if (next === 'u') {
            const hex = src.substr(i + 2, 4);
            if (!/^[0-9a-fA-F]{4}$/.test(hex)) fail('malformed \\u escape');
            out += String.fromCharCode(parseInt(hex, 16));
            i += 6;
            continue;
        }
        if (next === 'x') {
            const hex = src.substr(i + 2, 2);
            if (!/^[0-9a-fA-F]{2}$/.test(hex)) fail('malformed \\x escape');
            out += String.fromCharCode(parseInt(hex, 16));
            i += 4;
            continue;
        }
        if (next === '\n' || next === '\r') fail('line continuation is not allowed');
        out += Object.prototype.hasOwnProperty.call(SIMPLE_ESCAPES, next) ? SIMPLE_ESCAPES[next] : next;
        i += 2;
    }
    return fail('unterminated string literal');
}

function parseTokens(tokens, file) {
    let p = 0;
    const fail = (message) => {
        throw new SyntaxError(file + ': ' + message + ' (token ' + p + ')');
    };
    const isPunct = (value) => {
        const t = tokens[p];
        return !!t && t.type === 'punct' && t.value === value;
    };
    const expectPunct = (value) => {
        if (!isPunct(value)) fail('expected "' + value + '"');
        p += 1;
    };

    function parseValue() {
        const t = tokens[p];
        if (!t) fail('unexpected end of file');
        if (t.type === 'string') { p += 1; return t.value; }
        if (t.type === 'number') {
            p += 1;
            const n = Number(t.value);
            if (!Number.isFinite(n)) fail('non-finite number');
            return n;
        }
        if (t.type === 'ident') {
            p += 1;
            if (t.value === 'true') return true;
            if (t.value === 'false') return false;
            if (t.value === 'null') return null;
            fail('identifier "' + t.value + '" is not a literal value');
        }
        if (isPunct('{')) return parseObject();
        if (isPunct('[')) return parseArray();
        return fail('unexpected token "' + t.value + '"');
    }

    function parseObject() {
        expectPunct('{');
        const obj = {};
        while (!isPunct('}')) {
            const key = tokens[p];
            if (!key || (key.type !== 'ident' && key.type !== 'string')) {
                fail('object key must be an identifier or string');
            }
            p += 1;
            expectPunct(':');
            define(obj, key.value, parseValue());
            if (!isPunct(',')) break;
            p += 1;
        }
        expectPunct('}');
        return obj;
    }

    function parseArray() {
        expectPunct('[');
        const arr = [];
        while (!isPunct(']')) {
            arr.push(parseValue());
            if (!isPunct(',')) break;
            p += 1;
        }
        expectPunct(']');
        return arr;
    }

    const globals = {};

    function expectIdent(value) {
        const t = tokens[p];
        if (!t || t.type !== 'ident' || t.value !== value) fail('expected "' + value + '"');
        p += 1;
    }

    /**
     * The one non-literal form the generated word-audio catalogs use, so several
     * catalog files can add to the same global in a browser:
     *   Object.assign({}, window.NAME || {}, { ...literal... })
     * Only this exact shape is accepted. Any other Object.assign call is rejected.
     */
    function parseMergeAssign(name) {
        expectIdent('Object');
        expectPunct('.');
        expectIdent('assign');
        expectPunct('(');
        expectPunct('{');
        expectPunct('}');
        expectPunct(',');
        expectIdent('window');
        expectPunct('.');
        expectIdent(name);
        expectPunct('|');
        expectPunct('|');
        expectPunct('{');
        expectPunct('}');
        expectPunct(',');
        if (!isPunct('{')) fail('merge source must be an object literal');
        const extra = parseObject();
        expectPunct(')');
        const prior = globals[name];
        const merged = {};
        if (prior && typeof prior === 'object' && !Array.isArray(prior)) {
            Object.keys(prior).forEach(key => define(merged, key, prior[key]));
        }
        Object.keys(extra).forEach(key => define(merged, key, extra[key]));
        return merged;
    }

    while (p < tokens.length) {
        const head = tokens[p];
        if (!head || head.type !== 'ident' || head.value !== 'window') fail('only window.NAME = <literal> statements are allowed');
        p += 1;
        expectPunct('.');
        const name = tokens[p];
        if (!name || name.type !== 'ident') fail('expected a global name after window.');
        p += 1;
        expectPunct('=');
        if (tokens[p] && tokens[p].type === 'ident' && tokens[p].value === 'Object') {
            define(globals, name.value, parseMergeAssign(name.value));
        } else {
            define(globals, name.value, parseValue());
        }
        if (isPunct(';')) p += 1;
    }
    return globals;
}

/**
 * Parse a data source string. Returns an object of { NAME: value } for every
 * `window.NAME = <literal>` statement. Throws SyntaxError on anything else.
 */
function parseDataWindow(src, file) {
    const text = String(src).replace(/^\uFEFF/, '');
    return parseTokens(tokenize(text, file || '<data>'), file || '<data>');
}

/** Read and parse a lesson or catalog data file. Never executes the file. */
function loadDataWindow(file) {
    return parseDataWindow(fs.readFileSync(file, 'utf8'), file);
}

module.exports = { loadDataWindow, parseDataWindow };
