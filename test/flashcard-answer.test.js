/**
 * Local typed-answer check for flashcards.
 * Authored-by-spec: INIT-006/SPEC-012
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const Answers = require('../shared/player/flashcard-answer.js');

const POINTED = 'בֹּקֶר';
const UNPOINTED = 'בקר';

describe('flashcard typed answers (INIT-006/SPEC-012)', () => {
    it('matches pointed and unpointed forms of the same key', () => {
        assert.equal(Answers.matches(UNPOINTED, POINTED, []), true);
        assert.equal(Answers.matches(POINTED, POINTED, []), true);
        assert.equal(Answers.matches(POINTED, UNPOINTED, []), true);
    });

    it('matches an accept[] variant that is not the stripped key', () => {
        assert.equal(Answers.matches('בוקר', POINTED, ['בוקר']), true);
        assert.equal(Answers.matches('בוקר', POINTED, []), false);
    });

    it('rejects a wrong answer and empty typing', () => {
        assert.equal(Answers.matches('קם', POINTED, ['בוקר']), false);
        assert.equal(Answers.matches('   ', POINTED, [UNPOINTED]), false);
        assert.equal(Answers.matches('', POINTED, []), false);
    });

    it('suggests Good for a match, Again for a miss, and nothing when blank', () => {
        assert.equal(Answers.suggestGrade(UNPOINTED, POINTED, []), 3);
        assert.equal(Answers.suggestGrade('בוקר', POINTED, ['בוקר']), 3);
        assert.equal(Answers.suggestGrade('קם', POINTED, ['בוקר']), 1);
        assert.equal(Answers.suggestGrade('', POINTED, [UNPOINTED]), null);
    });

    it('maps keys 1–4 and leaves the typed text off the signal', () => {
        assert.deepEqual([1, 2, 3, 4].map((n) => Answers.keyToGrade(String(n))), [1, 2, 3, 4]);
        assert.equal(Answers.keyToGrade('5'), null);
        assert.equal(Answers.keyToGrade(' '), null);
        const typed = 'בוקר';
        const signal = Answers.gradeSignal({
            word: POINTED,
            direction: 'en_he',
            prompt: 'morning',
            answer: POINTED,
            typed
        }, 3, 'my-morning');
        assert.equal(signal.grade, 3);
        assert.equal(signal.result, 'pass');
        assert.equal(signal.word, POINTED);
        assert.equal(signal.direction, 'en_he');
        assert.equal(signal.slug, 'my-morning');
        assert.equal(signal.activity, 'flashcard');
        assert.equal(Object.hasOwn(signal, 'typed'), false);
        assert.equal(Object.hasOwn(signal, 'answer'), false);
        assert.equal(Object.hasOwn(signal, 'prompt'), false);
        assert.equal(JSON.stringify(signal).includes(typed), false);
        assert.equal(Answers.gradeSignal({ word: POINTED, direction: 'he_en' }, 1, 'my-morning').result, 'miss');
        assert.equal(Answers.gradeSignal({ word: POINTED }, 2).result, 'pass');
        assert.equal(Answers.gradeSignal({ word: POINTED }, 4).result, 'pass');
    });
});
