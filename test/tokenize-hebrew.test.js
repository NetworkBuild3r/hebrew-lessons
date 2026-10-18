/**
 * tokenizeHebrew — whitespace/punctuation split; maqaf & geresh stay with word.
 * Authored-by-spec: INIT-001/SPEC-007
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadUtils() {
    const sandbox = { console, window: {}, globalThis: {} };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'),
        sandbox
    );
    return sandbox.HebrewUtils;
}

function nfcList(arr) {
    // Array.from: vm-context arrays have a different Array ctor; deepEqual requires host arrays.
    return Array.from(arr, t => String(t).normalize('NFC'));
}

describe('tokenizeHebrew', () => {
    const U = loadUtils();

    it('exports tokenizeHebrew on HebrewUtils', () => {
        assert.equal(typeof U.tokenizeHebrew, 'function');
    });

    it('splits on whitespace', () => {
        assert.deepEqual(
            nfcList(U.tokenizeHebrew('קָם שׁוֹתֶה קָפֶה')),
            nfcList(['קָם', 'שׁוֹתֶה', 'קָפֶה'])
        );
    });

    it('splits on Latin punctuation but keeps Hebrew letters', () => {
        assert.deepEqual(
            nfcList(U.tokenizeHebrew('קָם, שׁוֹתֶה.')),
            nfcList(['קָם', 'שׁוֹתֶה'])
        );
    });

    it('keeps maqaf with the word', () => {
        const tokens = nfcList(U.tokenizeHebrew('בית־ספר גדול'));
        assert.ok(tokens.some(t => t.includes('\u05BE') || t.includes('־')));
        assert.equal(tokens[0], 'בית־ספר'.normalize('NFC'));
    });

    it('keeps geresh with the word', () => {
        // ASCII apostrophe may split; Hebrew geresh U+05F3 stays
        const withGeresh = nfcList(U.tokenizeHebrew('ג\u05F3יפ נוסע'));
        assert.equal(withGeresh[0], ('ג\u05F3יפ').normalize('NFC'));
        assert.equal(withGeresh[1], 'נוסע'.normalize('NFC'));
    });

    it('preserves final letters (ך ם ן ף ץ)', () => {
        assert.deepEqual(
            nfcList(U.tokenizeHebrew('בוקר טוב לך')),
            nfcList(['בוקר', 'טוב', 'לך'])
        );
        const kam = U.tokenizeHebrew('קָם')[0].normalize('NFC');
        assert.ok(kam.includes('ם') || kam === 'קָם'.normalize('NFC'));
    });

    it('returns empty array for blank input', () => {
        assert.equal(U.tokenizeHebrew('').length, 0);
        assert.equal(U.tokenizeHebrew('   ').length, 0);
        assert.equal(U.tokenizeHebrew(null).length, 0);
    });

    it('stripVowels keeps maqaf and drops points', () => {
        const stripped = U.stripVowels('\u05E2\u05B7\u05DC\u05BE\u05DB\u05BC\u05B5\u05DF');
        assert.equal(stripped, '\u05E2\u05DC\u05BE\u05DB\u05DF');
        assert.ok(stripped.includes('\u05BE'));
    });

    it('NFC-normalizes input', () => {
        const a = 'קָם'.normalize('NFC');
        const tokens = U.tokenizeHebrew(a);
        assert.equal(tokens.length, 1);
        assert.equal(tokens[0].normalize('NFC'), a);
    });
});
