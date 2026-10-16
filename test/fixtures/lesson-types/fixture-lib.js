/**
 * Shared loaders for lesson-type fixtures (INIT-012/SPEC-009).
 */
'use strict';

const fs = require('fs');
const path = require('path');

const DIR = __dirname;
const XSS = '<img src=x onerror=alert(1)>';

const TYPE_FILES = {
    A: 'type-a-topic-conversation.json',
    B: 'type-b-review-worksheet.json',
    C: 'type-c-verse-reading.json',
    D: 'type-d-verb-workshop.json',
    E: 'type-e-short-story.json',
    dictation: 'type-e-short-story.json',
    'one-block': 'edge-one-block.json',
    'twelve-items': 'edge-twelve-exercise.json'
};

function readJson(name) {
    return JSON.parse(fs.readFileSync(path.join(DIR, name), 'utf8'));
}

function loadLesson(typeKey) {
    const file = TYPE_FILES[typeKey];
    if (!file) throw new Error('unknown lesson type key: ' + typeKey);
    return readJson(file);
}

function memoryLexicon(entries) {
    const rows = new Map(entries.map((entry) => {
        const row = Object.assign({}, entry, {
            verified: entry.verified ? 1 : 0
        });
        return [row.key, row];
    }));
    return {
        find: (key) => rows.get(key) || null,
        listByStrippedKeys: (keys) => {
            const { strippedKey } = require('../../../server/plans/validate');
            return entries.filter((row) => keys.some((k) => strippedKey(row.key) === strippedKey(k)));
        }
    };
}

function lexiconRepo() {
    const data = readJson('lexicon-ledaber.json');
    return memoryLexicon(data.entries);
}

function planFromLessons(id, lessons) {
    return {
        schemaVersion: '1.2.0',
        id,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Five lesson types sample',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-types',
            title: 'Lesson type coverage',
            theme: 'samples',
            lessons
        }]
    };
}

function combinedFiveLessonPlan() {
    const raw = readJson('five-lesson-types.plan.json');
    return raw;
}

module.exports = {
    DIR,
    XSS,
    TYPE_FILES,
    readJson,
    loadLesson,
    memoryLexicon,
    lexiconRepo,
    planFromLessons,
    combinedFiveLessonPlan
};
