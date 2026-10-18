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

    it('maqaf and an ASCII hyphen match in both directions', () => {
        const maqaf = 'על\u05BEכן';
        const hyphen = 'על-כן';
        const pointedMaqaf = '\u05E2\u05B7\u05DC\u05BE\u05DB\u05BC\u05B5\u05DF';
        const pointedHyphen = '\u05E2\u05B7\u05DC-\u05DB\u05BC\u05B5\u05DF';
        assert.equal(U.stripVowels(maqaf), 'על\u05BEכן');
        assert.equal(U.stripVowels(hyphen), 'על\u05BEכן');
        assert.equal(U.stripVowels(maqaf), U.stripVowels(hyphen));
        assert.equal(U.stripVowels(hyphen), U.stripVowels(maqaf));
        assert.equal(U.stripVowels(pointedMaqaf), U.stripVowels(pointedHyphen));
        assert.equal(U.stripVowels(pointedHyphen), U.stripVowels(maqaf));
        assert.equal(U.normalizeHebrew(maqaf), 'על\u05BEכן');
        assert.equal(U.normalizeHebrew(hyphen), 'על\u05BEכן');
        assert.equal(U.normalizeHebrew(maqaf), U.normalizeHebrew(hyphen));
        assert.equal(U.normalizeHebrew(hyphen), U.normalizeHebrew(maqaf));
        assert.equal(U.normalizeHebrew(pointedMaqaf), U.normalizeHebrew(hyphen));
        assert.equal(U.normalizeHebrew(hyphen), U.normalizeHebrew(pointedMaqaf));
    });

    it('a word with no joiner stays distinct from the maqaf spelling', () => {
        const maqaf = 'על\u05BEכן';
        const plain = 'עלכן';
        assert.equal(U.normalizeHebrew(plain), 'עלכן');
        assert.equal(U.stripVowels(plain), 'עלכן');
        assert.notEqual(U.normalizeHebrew(plain), U.normalizeHebrew(maqaf));
        assert.notEqual(U.normalizeHebrew(plain), U.normalizeHebrew('על-כן'));
        assert.notEqual(U.stripVowels(plain), U.stripVowels(maqaf));
    });

    it('NFC-normalizes input', () => {
        const a = 'קָם'.normalize('NFC');
        const tokens = U.tokenizeHebrew(a);
        assert.equal(tokens.length, 1);
        assert.equal(tokens[0].normalize('NFC'), a);
    });
});
