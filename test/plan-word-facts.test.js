/**
 * What the open plan says about a tapped word: the meaning it saved, the
 * pointed spelling, and how many lessons use it.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

require('../shared/admin/plan-builder.js');

const { factsFromPlan } = globalThis.HebrewPlanBuilder;

function plan() {
    return {
        units: [{
            id: 'u1',
            lessons: [
                {
                    id: 'l1',
                    newWords: ['בית'],
                    blocks: [{
                        id: 'intro',
                        type: 'introduction',
                        words: [{ id: '1', word: 'בַּיִת', meaning: 'house', translit: 'bayit' }]
                    }]
                },
                {
                    id: 'l2',
                    blocks: [{
                        id: 'story',
                        type: 'graded-story',
                        paragraphs: [{ id: 'p1', text: 'הַבַּיִת גָּדוֹל וְיָפֶה' }],
                        glosses: { 'גָּדוֹל': 'big', 'יָפֶה': 'beautiful' }
                    }]
                },
                {
                    id: 'l3',
                    blocks: [{ id: 'talk', type: 'short-dialogue', lines: [{ id: '1', pointed: 'מַה זֶּה? בַּיִת.' }] }]
                }
            ]
        }]
    };
}

describe('facts the plan saved for a word', () => {
    it('gives the introduction meaning, the sound, the pointed spelling, and the lesson count', () => {
        const facts = factsFromPlan(plan(), 'בית');
        assert.equal(facts.meaning, 'house');
        assert.equal(facts.translit, 'bayit');
        assert.equal(facts.pointed, 'בַּיִת');
        assert.equal(facts.lessons, 2, 'the first lesson and the dialogue; the story has הבית, a different word');
    });

    it('falls back to the story gloss and its pointed spelling', () => {
        const facts = factsFromPlan(plan(), 'גדול');
        assert.equal(facts.meaning, 'big');
        assert.equal(facts.pointed, 'גָּדוֹל');
        assert.equal(facts.lessons, 1);
    });

    it('matches with or without vowel points', () => {
        assert.deepEqual(factsFromPlan(plan(), 'בַּיִת'), factsFromPlan(plan(), 'בית'));
    });

    it('is null for a word the plan never uses, and for a bad plan', () => {
        assert.equal(factsFromPlan(plan(), 'כלב'), null);
        assert.equal(factsFromPlan(null, 'בית'), null);
        assert.equal(factsFromPlan({ units: [] }, ''), null);
    });

    it('a word used but never glossed has a count and no meaning', () => {
        const facts = factsFromPlan(plan(), 'יפה');
        assert.equal(facts.meaning, 'beautiful');
        const used = factsFromPlan(plan(), 'זה');
        assert.equal(used.meaning, '');
        assert.equal(used.lessons, 1);
    });
});
