'use strict';

// exactKey oracle. Prints one JSON object per line.
// The first line is the Node 22.14 Unicode 16.0 /\s/ set.
// Later lines are exactKey results. Library exports and the plan
// checkers' string formula must agree on every input.

const path = require('path');

const repoRoot = path.resolve(__dirname, '../../../..');
process.env.HEBREW_ROOT = repoRoot;
process.env.DATA_DIR = process.env.DATA_DIR || path.join(repoRoot, 'data');
process.env.LESSONS_DIR = path.join(repoRoot, 'worksheets');

const vocab = require(path.join(repoRoot, 'server', 'vocab-index')).exactKey;
const userWords = require(path.join(repoRoot, 'server', 'db', 'repos', 'user-words')).exactKey;
const lexicon = require(path.join(repoRoot, 'server', 'db', 'repos', 'lexicon')).exactKey;

function plan(value) {
    return String(value).normalize('NFC').replace(/\s+/g, ' ').trim();
}

const space = [];
for (let cp = 0; cp <= 0x10FFFF; cp++) {
    if (cp >= 0xD800 && cp <= 0xDFFF) continue;
    const ch = String.fromCodePoint(cp);
    const matched = /\s/.test(ch);
    if (matched !== /\s/u.test(ch) || matched !== (ch.trim() === '')) {
        console.error('trim and \\s disagree at U+' + cp.toString(16));
        process.exit(1);
    }
    if (matched) space.push(cp);
}

const inputs = [
    ['reorder', 'ש\u05C1\u05B8'],
    ['ordered', 'ש\u05B8\u05C1'],
    ['double-space', 'a  b'],
    ['em-space', 'a\u2003b'],
    ['nbsp', 'a\u00A0b'],
    ['fef', '\uFEFFa\uFEFF'],
    ['nel-inside', 'a\u0085b'],
    ['nel-edge', '\u0085a\u0085'],
    ['u1c89', 'a\u1C89b'],
    ['u1fae9', 'a\u{1FAE9}b'],
    ['case', 'Shalom'],
    ['edges', ' \t\n a \u1680 '],
    ['hebrew-gap', 'שָׁלוֹם  בַּיִת'],
    ['only-space', ' \u3000\uFEFF '],
    ['empty', ''],
    ['mongolian', 'a\u180Eb']
];

process.stdout.write(JSON.stringify({
    kind: 'space',
    node: process.version,
    unicode: process.versions.unicode,
    cps: space
}) + '\n');

for (const [id, value] of inputs) {
    const got = vocab(value);
    if (got !== userWords(value) || got !== lexicon(value) || got !== plan(value)) {
        console.error('exactKey mismatch ' + id
            + ' vocab=' + JSON.stringify(got)
            + ' userWords=' + JSON.stringify(userWords(value))
            + ' lexicon=' + JSON.stringify(lexicon(value))
            + ' plan=' + JSON.stringify(plan(value)));
        process.exit(1);
    }
    process.stdout.write(JSON.stringify({ kind: 'key', id: id, in: value, out: got }) + '\n');
}
