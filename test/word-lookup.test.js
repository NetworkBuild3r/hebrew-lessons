/**
 * Hebrew words in a sentence become tappable pieces; punctuation stays put.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const { splitLookup, fetchWord, meanings } = require('../shared/word-lookup.js');

describe('word lookup split', () => {
    it('keeps the Hebrew words and the spaces between them', () => {
        assert.deepEqual(splitLookup('אמא ו אבא.'), [
            { type: 'word', value: 'אמא' },
            { type: 'text', value: ' ' },
            { type: 'word', value: 'ו' },
            { type: 'text', value: ' ' },
            { type: 'word', value: 'אבא' },
            { type: 'text', value: '.' }
        ]);
    });

    it('leaves an English sentence alone', () => {
        assert.deepEqual(splitLookup('mother and father'), [
            { type: 'text', value: 'mother and father' }
        ]);
    });
});

describe('word lookup request', () => {
    it('asks the app for the word and returns the short meaning and the senses', async () => {
        const asked = [];
        globalThis.HebrewApi = {
            get(url) {
                asked.push(url);
                return Promise.resolve({
                    ok: true,
                    data: { short: 'house, home', form: null, meanings: [{ pos: 'noun', en: 'house, home, residence' }] }
                });
            }
        };
        try {
            const found = await fetchWord('בַּיִת');
            assert.equal(found.ok, true);
            assert.equal(found.short, 'house, home');
            assert.equal(found.meanings.length, 1);
            assert.match(asked[0], /^\/api\/dictionary\/lookup\?word=/);
            assert.equal(await meanings('בית'), 'house, home');
        } finally {
            delete globalThis.HebrewApi;
        }
    });

    it('a failed or refused request is an answer, never a hang', async () => {
        globalThis.HebrewApi = { get: () => Promise.reject(new Error('offline')) };
        try {
            assert.equal((await fetchWord('בית')).ok, false);
            globalThis.HebrewApi = { get: () => Promise.resolve({ ok: false, status: 500, data: null }) };
            assert.equal((await fetchWord('בית')).ok, false);
            globalThis.HebrewApi = { get() { throw new Error('boom'); } };
            assert.equal((await fetchWord('בית')).ok, false);
        } finally {
            delete globalThis.HebrewApi;
        }
    });
});