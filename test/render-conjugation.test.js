/**
 * Conjugation tables: study layout, drill scoring, form signals.
 * Authored-by-spec: INIT-006/SPEC-014
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

require('../shared/ui/blocks/explainer');
const table = require('../shared/ui/blocks/conjugation-table');

const SRC = fs.readFileSync(path.join(__dirname, '..', 'shared', 'ui', 'blocks', 'conjugation-table.js'), 'utf8');
const PAYLOAD = '<script>alert(1)</script>';
const LEMMA = 'לַחְשֹׁב';
const ROOT = 'ח.ש.ב';

const PRESENT_CELLS = [
    { cell: 'fp', formKey: 'חוֹשְׁבוֹת' },
    { cell: 'mp', formKey: 'חוֹשְׁבִים' },
    { cell: 'fs', formKey: 'חוֹשֶׁבֶת' },
    { cell: 'ms', formKey: 'חוֹשֵׁב' }
];

const PAST_CELLS = [
    { cell: '3p', formKey: 'חָשְׁבוּ' },
    { cell: '2fp', formKey: 'חֲשַׁבְתֶּן' },
    { cell: '2mp', formKey: 'חֲשַׁבְתֶּם' },
    { cell: '1p', formKey: 'חָשַׁבְנוּ' },
    { cell: '3fs', formKey: 'חָשְׁבָה' },
    { cell: '3ms', formKey: 'חָשַׁב' },
    { cell: '2fs', formKey: 'חָשַׁבְתְּ' },
    { cell: '2ms', formKey: 'חָשַׁבְתָּ' },
    { cell: '1s', formKey: 'חָשַׁבְתִּי' }
];

function formsFor(cells, unverified) {
    const forms = Object.create(null);
    cells.forEach((entry) => {
        forms[entry.formKey] = {
            key: entry.formKey,
            verified: !unverified.has(entry.formKey),
            root: ROOT,
            accept: []
        };
    });
    return forms;
}

function orderOf(html, labels) {
    let from = 0;
    labels.forEach((label) => {
        const at = html.indexOf(label, from);
        assert.ok(at >= from, label + ' missing after ' + from);
        from = at + label.length;
    });
}

describe('conjugation source', () => {
    it('says form signals never create cards and never drill an unverified form, and has no timer', () => {
        assert.match(SRC, /Form signals never create cards/);
        assert.match(SRC, /Never drill an unverified form/);
        assert.equal(/setTimeout|setInterval/.test(SRC), false);
    });
});

describe('study tables', () => {
    it('lays out present columns SM SF PM PF and past rows in person order', () => {
        const present = table.render({
            id: 'present',
            type: 'conjugation-table',
            lemmaKey: LEMMA,
            tense: 'present',
            mode: 'study',
            cells: PRESENT_CELLS
        }, { forms: formsFor(PRESENT_CELLS, new Set(['חוֹשְׁבוֹת'])) });
        orderOf(present, ['>SM<', '>SF<', '>PM<', '>PF<']);
        assert.match(present, /not yet checked/);
        assert.match(present, /class="root-letter"/);
        assert.equal(/<img\b/i.test(present), false);
        const past = table.render({
            id: 'past',
            type: 'conjugation-table',
            lemmaKey: LEMMA,
            tense: 'past',
            mode: 'study',
            cells: PAST_CELLS
        }, { forms: formsFor(PAST_CELLS, new Set()) });
        orderOf(past, ['אני', 'אתה', 'את', 'הוא', 'היא', 'אנחנו', 'אתם', 'אתן', 'הם/הן']);
        assert.equal((past.match(/<tr>/g) || []).length, 9);
    });

    it('shows a play control when a clip exists and does not copy the path', () => {
        const html = table.render({
            id: 'present',
            type: 'conjugation-table',
            lemmaKey: LEMMA,
            tense: 'present',
            mode: 'study',
            cells: [{ cell: 'ms', formKey: 'חוֹשֵׁב' }],
            image: { mediaId: 'pic', alt: 'a verb' }
        }, {
            forms: formsFor([{ formKey: 'חוֹשֵׁב' }], new Set()),
            audio: { 'חוֹשֵׁב': 'audio/think.mp3', 'לַחְשֹׁב': 'http://evil.example/a.mp3' }
        });
        assert.match(html, /data-plan-audio="ready"/);
        assert.equal(html.includes('think.mp3'), false);
        assert.equal(html.includes('evil.example'), false);
        assert.equal(/<img\b/i.test(html), false);
    });

    it('escapes every string field', () => {
        const html = table.render({
            id: PAYLOAD,
            type: 'conjugation-table',
            lemmaKey: PAYLOAD,
            tense: 'present',
            mode: 'study',
            cells: [{ cell: PAYLOAD, formKey: PAYLOAD }],
            sentences: [{ id: PAYLOAD, he: PAYLOAD, en: PAYLOAD }],
            questions: [{ id: PAYLOAD, he: PAYLOAD, en: PAYLOAD }]
        });
        const escaped = table.escapeHtml(PAYLOAD);
        assert.equal(html.includes(PAYLOAD), false);
        assert.ok(html.includes(escaped));
        assert.equal(/<script/i.test(html), false);
        assert.equal(/<img\b/i.test(html), false);
        assert.match(html, /not yet checked/);
    });
});

describe('drill mode', () => {
    const verified = {
        'חָשַׁבְתִּי': { key: 'חָשַׁבְתִּי', verified: true, accept: ['חשבתי', 'חשבתיי'], root: ROOT },
        'חָשַׁבְתָּ': { key: 'חָשַׁבְתָּ', verified: 1, accept: ['חשבת'], root: ROOT },
        'חָשַׁב': { key: 'חָשַׁב', verified: false, accept: ['חשב'], root: ROOT }
    };
    const past = {
        id: 'past',
        type: 'conjugation-table',
        lemmaKey: LEMMA,
        tense: 'past',
        mode: 'drill',
        cells: [
            { cell: '1s', formKey: 'חָשַׁבְתִּי' },
            { cell: '2ms', formKey: 'חָשַׁבְתָּ' },
            { cell: '3ms', formKey: 'חָשַׁב' }
        ],
        sentences: [{ id: 's1', he: 'אֲנִי חושב על זה.', en: 'I am thinking about it.' }],
        questions: [{ id: 'q1', he: 'מָה עָשִׂיתָ?', en: 'What did you do?' }]
    };
    const other = {
        id: 'other',
        type: 'conjugation-table',
        lemmaKey: 'לִכְתֹּב',
        tense: 'past',
        mode: 'drill',
        cells: [{ cell: '1s', formKey: 'כָּתַבְתִּי' }]
    };

    it('queues only verified cells and interleaves lemmas', () => {
        const html = table.render(past, { forms: verified, lesson: { id: 'public-lesson', blocks: [past] } });
        assert.match(html, /data-drill-owner="1"/);
        assert.match(html, /data-letter="א"/);
        assert.equal(html.includes('חָשַׁבְתִּי'), false);
        assert.equal(html.includes('חָשַׁבְתָּ'), false);
        assert.equal(html.includes('>חָשַׁב<'), false);
        assert.match(html, /אֲנִי חושב על זה/);
        assert.equal(/<img\b/i.test(html), false);

        const queue = table.drillQueue([past, other], Object.assign({
            'כָּתַבְתִּי': { key: 'כָּתַבְתִּי', verified: true, accept: [] }
        }, verified), function () { return 0.999999; });
        assert.deepEqual(queue.map((item) => item.formKey), ['חָשַׁבְתִּי', 'כָּתַבְתִּי', 'חָשַׁבְתָּ']);
        assert.equal(queue.some((item) => item.formKey === 'חָשַׁב'), false);
    });

    it('passes an unpointed answer, asks once, then reveals', () => {
        const form = { formKey: 'חָשַׁבְתִּי', accept: ['חשבתי', 'חשבתיי'] };
        assert.equal(table.decideAttempt('   ', form, 0).attempted, false);
        const pointed = table.decideAttempt('חָשַׁבְתִּי', form, 0);
        assert.equal(pointed.correct, true);
        assert.equal(pointed.result, 'pass');
        const unpointed = table.decideAttempt('חשבתי', form, 0);
        assert.equal(unpointed.correct, true);
        const alternate = table.decideAttempt('חשבתיי', form, 0);
        assert.equal(alternate.correct, true);
        const first = table.decideAttempt('WRONG-XYZ', form, 0);
        assert.equal(first.correct, false);
        assert.equal(first.reveal, false);
        assert.equal(first.result, 'miss');
        const second = table.decideAttempt('WRONG-XYZ', form, 1);
        assert.equal(second.reveal, true);
        assert.equal(second.result, 'miss');
    });

    it('builds one form signal and leaves the typed text off it', () => {
        const typed = 'WRONG-XYZ';
        const signal = table.formSignal({
            formKey: 'חָשַׁבְתִּי',
            lemma: LEMMA
        }, 'miss', 'conjugation-drill', 'public-lesson');
        assert.equal(signal.kind, 'form');
        assert.equal(signal.word, 'חָשַׁבְתִּי');
        assert.equal(signal.lemma, LEMMA);
        assert.equal(signal.result, 'miss');
        assert.equal(signal.activity, 'conjugation-drill');
        assert.equal(signal.direction, 'en_he');
        assert.equal(JSON.stringify(signal).includes(typed), false);
        assert.equal(Object.prototype.hasOwnProperty.call(signal, 'typed'), false);
        const override = table.formSignal({
            formKey: 'חָשַׁבְתִּי',
            lemma: LEMMA
        }, 'pass', 'conjugation-override', 'public-lesson');
        assert.equal(override.activity, 'conjugation-override');
        assert.equal(override.result, 'pass');
    });
});
