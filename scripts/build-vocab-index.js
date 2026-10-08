#!/usr/bin/env node
/**
 * Build worksheets/_review/vocab-index.js (or LESSONS_DIR/_review/...).
 * Usage: npm run vocab-index
 * Authored-by-spec: INIT-001/SPEC-002
 */
const { rebuildVocabIndex } = require('../server/vocab-index');

function main() {
    const result = rebuildVocabIndex();
    console.log(
        'Wrote ' + result.path
        + ' (' + result.entries.length + ' words from '
        + result.lessonsUsed + '/' + result.lessonsScanned + ' lessons)'
    );
    if (result.warnings.length) {
        console.warn('Warnings:');
        result.warnings.forEach(w => console.warn('- ' + w));
    }
}

main();
