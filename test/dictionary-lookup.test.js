'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { readArticle } = require('../server/dictionary/stardict');
const { candidates, shortGloss } = require('../server/dictionary/lookup');

test('a verb article gives its infinitive as another form', () => {
    const article = readArticle('<font color="#007000">verb</font> לאכול<br> to eat, consume; to use up');
    assert.equal(article.pos, 'verb');
    assert.equal(article.en, 'to eat, consume; to use up');
    assert.deepEqual(article.forms, ['לאכול']);
});

test('a cross-reference is not a form or a sense', () => {
    const article = readArticle('<font color="#007000">noun</font> אמא (&gt;&gt;אימא)<br> mother');
    assert.equal(article.en, 'mother');
    assert.ok(!article.en.includes('>>'));
});

test('candidates find the stored spelling of a present-tense verb', () => {
    const found = candidates('רוצה');
    assert.ok(found.includes('רצה'), 'drops the inner vav');
    assert.ok(found.includes('רץ'), 'strips the ending and takes the final letter shape');
});

test('candidates strip a one-letter prefix', () => {
    assert.ok(candidates('הבית').includes('בית'));
});

test('the short meaning keeps the first three senses', () => {
    assert.equal(shortGloss('book, volume; notebook; ledger'), 'book, volume, notebook');
});
