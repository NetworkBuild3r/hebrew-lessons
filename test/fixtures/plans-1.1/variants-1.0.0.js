/**
 * 1.0.0 documents that trip N1, N2, N6, N7, and the further rules, built from
 * the ADR-0003 samples. samples-1.0.0-results.json holds the INIT-004/SPEC-002
 * validator's result for each sample and each variant, recorded before
 * INIT-006/SPEC-011 changed the validator.
 * Provenance: INIT-006/SPEC-011
 */
'use strict';

const fs = require('fs');
const path = require('path');

const SAMPLES = path.join(__dirname, '..', '..', '..', 'docs', 'decisions', 'ADR-0003-assets', 'samples');
const SAMPLE_NAMES = ['anal-sex-vocab', 'beginner-unit-1', 'my-morning', 'shai-date-indirect-speech'];

function sample(name) {
    return JSON.parse(fs.readFileSync(path.join(SAMPLES, name + '.plan.json'), 'utf8'));
}

function lesson(plan, index) {
    return plan.units[0].lessons[index];
}

function addWord(target, word, meaning, sentence) {
    target.newWords.push(word);
    const words = target.blocks[0].words;
    words.push({ id: String(words.length + 1), word, meaning, sentence: sentence || word + '.' });
}

const VARIANTS = Object.freeze({
    'my-morning-ninth-word-not-legacy': () => {
        const plan = sample('my-morning');
        addWord(lesson(plan, 0), 'עֲבוֹדָה', 'work', 'אֲנִי יוֹצֵא לָעֲבוֹדָה.');
        delete plan.migration.legacy;
        return plan;
    },
    'my-morning-ninth-word-legacy': () => {
        const plan = sample('my-morning');
        addWord(lesson(plan, 0), 'עֲבוֹדָה', 'work', 'אֲנִי יוֹצֵא לָעֲבוֹדָה.');
        return plan;
    },
    'beginner-unit-1-two-recycles': () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 1).recycles = lesson(plan, 1).recycles.slice(0, 2);
        return plan;
    },
    'beginner-unit-1-introductions-only': () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks = lesson(plan, 0).blocks.slice(0, 1);
        lesson(plan, 1).blocks = lesson(plan, 1).blocks.slice(0, 1);
        return plan;
    },
    'beginner-unit-1-word-used-once': () => {
        const plan = sample('beginner-unit-1');
        addWord(lesson(plan, 1), 'שָׁלוֹם', 'hello');
        return plan;
    },
    'beginner-unit-1-unpointed-word': () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).newWords[1] = 'קפה';
        lesson(plan, 0).blocks[0].words[1].word = 'קפה';
        return plan;
    },
    'beginner-unit-1-unknown-block-type': () => {
        const plan = sample('beginner-unit-1');
        lesson(plan, 0).blocks[1].type = 'speed-quiz';
        return plan;
    }
});

module.exports = { SAMPLE_NAMES, VARIANTS, sample };
