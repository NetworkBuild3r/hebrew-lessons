'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { suggestGlosses, suggestForBlock, tokensOf } = require('../server/dictionary/suggest-glosses');

const MEANINGS = {
    בית: { short: 'house, home', form: null, meanings: [{ pos: 'noun', en: 'house, home, residence' }] },
    הילד: { short: 'child, boy', form: 'ילד', meanings: [{ pos: 'noun', en: 'child, boy' }] }
};
const fake = (word) => {
    const bare = word.replace(/[\u0591-\u05C7]/g, '');
    return MEANINGS[bare] || { short: '', form: null, meanings: [] };
};

test('tokens are Hebrew words, once each, as written', () => {
    assert.deepEqual(tokensOf('הַבַּיִת, בית. הבית!'), ['הַבַּיִת', 'בית'], 'pointed and plain spellings of one word count once');
    assert.deepEqual(tokensOf('Hello world'), []);
    assert.deepEqual(tokensOf('א בית'), ['בית'], 'a single letter is not a word');
});

test('a draft has a meaning per known word and lists the words with no entry', () => {
    const out = suggestGlosses('בית ילד צצצ', { lookup: fake });
    assert.deepEqual(out.suggestions.map((s) => [s.word, s.meaning]), [['בית', 'house, home']]);
    assert.deepEqual(out.unmatched, ['ילד', 'צצצ']);
});

test('a word that is the form of another says which', () => {
    const out = suggestGlosses('הילד', { lookup: fake });
    assert.equal(out.suggestions[0].form, 'ילד');
});

test('words that already have a meaning are skipped, pointed or not', () => {
    const out = suggestGlosses('הַבַּיִת בית', { skip: ['בית'], lookup: fake });
    assert.equal(out.suggestions.length, 0);
    assert.equal(out.skipped, 1);
});

test('the draft stops at the room left in the story map', () => {
    const many = Array.from({ length: 10 }, (_, i) => 'בית' + 'אבגדהוזחטי'[i]).join(' ');
    const out = suggestGlosses(many, { max: 4, lookup: () => ({ short: 'x', form: null, meanings: [] }) });
    assert.equal(out.suggestions.length, 4);
});

test('a story block is read from its paragraphs, glosses, and support glossary', () => {
    const block = {
        type: 'graded-story',
        paragraphs: [{ id: 'p1', text: 'בית ילד' }, { id: 'p2', text: 'הילד' }],
        glosses: { בית: 'house' },
        support: [{ id: 's1', he: 'ילד', en: 'child' }]
    };
    const out = suggestForBlock(block, { lookup: fake });
    assert.deepEqual(out.suggestions.map((s) => s.word), ['הילד']);
    assert.equal(out.skipped, 2);
});

test('a block with 80 meanings already has no room for a draft', () => {
    const glosses = {};
    for (let i = 0; i < 80; i += 1) glosses['מילה' + String.fromCharCode(0x5d0 + (i % 27)) + String.fromCharCode(0x5d0 + Math.floor(i / 27))] = 'x';
    const out = suggestForBlock({ paragraphs: [{ id: 'p', text: 'בית' }], glosses }, { lookup: fake });
    assert.equal(out.suggestions.length + out.unmatched.length, 0);
});
