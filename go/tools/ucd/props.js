#!/usr/bin/env node
'use strict';

// Emits the Unicode property golden Node actually evaluates. The Go ucd
// package must match it. Run from the repo root, after go/internal/ucd/gen:
//
//   node go/tools/ucd/props.js

const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '../../..');
const OUT = path.join(ROOT, 'go', 'testdata', 'golden', 'ucd');
const CPS = path.join(ROOT, 'go', 'internal', 'ucd', 'delta-cps.txt');
const { exactKey, strippedKey, tokenize } = require(path.join(ROOT, 'server', 'plans', 'validate'));

const BITS = [
    ['L', /^\p{L}$/u],
    ['M', /^\p{M}$/u],
    ['N', /^\p{N}$/u],
    ['P', /^\p{P}$/u],
    ['S', /^\p{S}$/u],
    ['Cf', /^\p{Cf}$/u],
    ['Z', /^\p{Z}$/u],
    ['C', /^\p{C}$/u],
    ['js', /^\s$/u],
    ['ws', /^\p{White_Space}$/u]
];

function mask(cp) {
    const s = String.fromCodePoint(cp);
    let m = 0;
    for (let i = 0; i < BITS.length; i++) {
        if (BITS[i][1].test(s)) m |= 1 << i;
    }
    return m;
}

function readDeltas() {
    const text = fs.readFileSync(CPS, 'utf8');
    const cps = [];
    for (const line of text.split('\n')) {
        const raw = line.split('#')[0].trim();
        if (!raw) continue;
        const [a, b] = raw.split('..');
        const lo = parseInt(a, 16);
        const hi = b ? parseInt(b, 16) : lo;
        for (let cp = lo; cp <= hi; cp++) cps.push(cp);
    }
    return cps;
}

function main() {
    const unicode = process.versions.unicode;
    if (unicode !== '16.0') {
        console.error('process.versions.unicode is ' + unicode + ', props golden is pinned to 16.0');
        process.exit(1);
    }
    fs.mkdirSync(OUT, { recursive: true });
    const lines = ['# unicode 16.0'];
    BITS.forEach((bit, i) => lines.push('# bit ' + bit[0] + ' ' + (1 << i)));
    let start = 0;
    let prevMask = mask(0);
    for (let cp = 1; cp <= 0x10FFFF; cp++) {
        const m = mask(cp);
        if (m === prevMask) continue;
        lines.push(hex(start) + (start === cp - 1 ? '' : '..' + hex(cp - 1)) + '\t' + prevMask.toString(16));
        start = cp;
        prevMask = m;
    }
    lines.push(hex(start) + (start === 0x10FFFF ? '' : '..' + hex(0x10FFFF)) + '\t' + prevMask.toString(16));
    fs.writeFileSync(path.join(OUT, 'props.rle'), lines.join('\n') + '\n');

    const word = 'שלום';
    const deltas = ['# unicode 16.0', '# word ' + word, '# k kept  e edge-stripped  x exception'];
    for (const cp of readDeltas()) {
        const ch = String.fromCodePoint(cp);
        const input = word + ch;
        const exact = exactKey(input);
        const stripped = strippedKey(input);
        const tokens = tokenize(input);
        const kept = exact === input && stripped === input && tokens.length === 1 && tokens[0] === input;
        const edge = exact === input && stripped === input && tokens.length === 1 && tokens[0] === word;
        if (kept) deltas.push(hex(cp) + '\tk');
        else if (edge) deltas.push(hex(cp) + '\te');
        else deltas.push(hex(cp) + '\tx\t' + JSON.stringify(exact) + '\t' + JSON.stringify(stripped) + '\t' + JSON.stringify(tokens));
    }
    fs.writeFileSync(path.join(OUT, 'deltas.tsv'), deltas.join('\n') + '\n');
    fs.writeFileSync(path.join(OUT, 'meta.json'), JSON.stringify({
        unicode: process.versions.unicode,
        node: process.version
    }, null, 2) + '\n');
    console.log('ucd props', lines.length, 'runs;', deltas.length, 'delta rows');
}

function hex(cp) {
    return cp.toString(16).toUpperCase().padStart(4, '0');
}

main();
