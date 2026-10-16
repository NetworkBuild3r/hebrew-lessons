/**
 * Fixtures for the plan editor tests (INIT-008/SPEC-011, SPEC-012): a sample plan with one block of
 * each of the eleven types, an editor on the fake DOM with a recording transport. Hand-written text only.
 */
'use strict';

const fs = require('fs');
const path = require('path');
const Form = require('../../shared/admin/plan-form.js');
const dom = require('./fake-dom');

const SCHEMA_PATH = path.join(__dirname, '..', '..', 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json');
const SCHEMA = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
const TOOLS = Form.createTools(SCHEMA);
const HEBREW = /[\u0590-\u05FF]/;

const LESSON = 'units/u1/lessons/l1';

/** One block of each type with a little hand-written content (not validated here). */
function sampleBlock(type, id) {
    const he = (text) => text;
    const samples = {
        introduction: {
            words: [{ id: '1', word: he('בַּיִת'), meaning: 'house', translit: 'bayit', group: 1 }],
            pattern: { heading: 'A pattern', rules: [{ title: 'r', english: 'e', example: he('בַּיִת') }] },
            cloze: { items: [{ id: '1', textPre: he('זֶה'), textPost: he('שֶׁלִּי'), answer: he('בַּיִת') }] }
        },
        'graded-story': {
            paragraphs: [{ id: '1', text: he('הַבַּיִת גָּדוֹל') }],
            support: [{ id: '1', he: he('בַּיִת'), en: 'house' }],
            glosses: { [he('בַּיִת')]: 'house' },
            questions: [{ id: '1', question: 'What is big?' }]
        },
        listening: { passage: he('שָׁלוֹם'), questions: [{ id: '1', question: 'Who?' }] },
        'short-dialogue': { source: 'authored', lines: [{ id: '1', pointed: he('מַה שְּׁלוֹמְךָ') }] },
        'fluency-pass': { seconds: 30, source: 'vocab', cues: ['go'] },
        'spaced-review-checkpoint': { wordKeys: [he('בַּיִת')], fromLessons: ['earlier'] },
        explainer: {
            title: 'A page',
            sections: [{
                id: '1', heading: 'H', paragraphs: ['p'],
                examples: [{ id: '1', he: he('בַּיִת'), en: 'house' }],
                table: { columns: ['a'], rows: [['b']] },
                callouts: [{ id: '1', tone: 'info', title: 't', text: 'x' }]
            }]
        },
        'conjugation-table': {
            lemmaKey: he('שׁוֹתֶה'), tense: 'present', mode: 'study',
            cells: [{ cell: 'ms', formKey: he('שׁוֹתֶה') }],
            sentences: [{ id: '1', he: he('אֲנִי שׁוֹתֶה') }],
            questions: [{ id: '1', he: he('מָה אַתָּה שׁוֹתֶה') }]
        },
        'personal-question': { prompts: [{ id: '1', he: he('מָה שְׁמֶךָ') }], answer: 'either' },
        'verse-reading': {
            ref: { book: 'Genesis', chapter: 1, from: 1, to: 1, source: 'an edition' },
            newWords: [he('בְּרֵאשִׁית')], verse: he('בְּרֵאשִׁית'),
            questions: [{ id: '1', he: he('מָה') }], personalQuestion: { he: he('מָה') }
        },
        'letter-intro': {
            letter: he('בּ'), name: 'Bet',
            sounds: [{ id: '1', vowel: 'a', pointed: he('בַּ'), sound: 'ba' }],
            findIn: [{ id: '1', word: he('בַּיִת'), contains: true }]
        },
        exercise: {
            kind: 'choose',
            items: [{
                id: 'item-1',
                prompt: 'Pick one',
                choices: ['a', 'b'],
                answer: 'a'
            }]
        },
        assignment: {
            prompt: 'Write about your week.',
            minSentences: 2,
            useWords: [he('בַּיִת')],
            homework: ['Read once more.']
        },
        'type-it': {
            prompt: 'Type the word',
            answer: he('בַּיִת')
        }
    };
    return Object.assign({ id, type }, samples[type]);
}

function samplePlan() {
    const blocks = TOOLS.blockTypes().map((b) => sampleBlock(b.type, b.type));
    return {
        schemaVersion: '1.1.0', id: 'sample', revision: 4, status: 'draft', audience: 'shared', assigneeId: null,
        title: 'Sample', language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'u1', title: 'Unit', theme: 'home',
            lessons: [{
                id: 'l1', title: 'Lesson', level: 'basic', visibility: 'hidden', recipe: 'guide',
                buildsOn: ['earlier'], newWords: [HEBREW.source ? 'בַּיִת' : ''], recycles: [{ word: 'בַּיִת', fromLessonId: 'earlier' }],
                blocks
            }]
        }]
    };
}

function makeEditor(extra) {
    const doc = dom.makeDocument();
    const host = doc.createElement('div');
    const calls = [];
    const transport = {
        setField(p, f, v, hooks) { calls.push({ op: 'set', path: p, field: f, value: v }); if (hooks && hooks.onSaved) hooks.onSaved(); return Promise.resolve({ ok: true, status: 200, data: {} }); },
        putPart(p, v, hooks) { calls.push({ op: 'put', path: p, value: v }); if (hooks && hooks.onSaved) hooks.onSaved(); return Promise.resolve({ ok: true, status: 200, data: {} }); }
    };
    const editor = Form.createEditor(Object.assign({ doc, host, schema: SCHEMA, transport }, extra || {}));
    return { doc, host, editor, calls };
}

/** The `[data-plan-field]` elements that belong to this node form and not to a nested one. */
function ownFields(formEl) {
    return dom.collect(formEl, (n) => n.hasAttribute && n.hasAttribute('data-plan-field')).filter((n) => {
        for (let cur = n.parentNode; cur; cur = cur.parentNode) {
            if (cur.hasAttribute && cur.hasAttribute('data-plan-node')) return cur === formEl;
        }
        return false;
    });
}


module.exports = { LESSON, SCHEMA, SCHEMA_PATH, TOOLS, HEBREW, sampleBlock, samplePlan, makeEditor, ownFields };