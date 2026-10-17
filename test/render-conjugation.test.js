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

describe('future tense (INIT-012/SPEC-006)', () => {
    const FUTURE_CELLS = [
        { cell: '1s', formKey: 'אדבר' },
        { cell: '2ms', formKey: 'תדבר' },
        { cell: '2fs', formKey: 'תדברי' },
        { cell: '3ms', formKey: 'ידבר' },
        { cell: '3fs', formKey: 'תדבר' },
        { cell: '1p', formKey: 'נדבר' },
        { cell: '2mp', formKey: 'תדברו' },
        { cell: '2fp', formKey: 'תדברנה' },
        { cell: '3p', formKey: 'ידברו' }
    ];

    it('study mode lists nine person rows like past', () => {
        const forms = formsFor(FUTURE_CELLS, new Set(['תדברנה']));
        const html = table.render({
            id: 'future',
            type: 'conjugation-table',
            lemmaKey: 'לדבר',
            tense: 'future',
            mode: 'study',
            cells: FUTURE_CELLS
        }, { forms });
        orderOf(html, ['אני', 'אתה', 'את', 'הוא', 'היא', 'אנחנו', 'אתם', 'אתן', 'הם/הן']);
        assert.match(html, /data-tense="future"/);
        assert.match(html, /עתיד/);
        assert.match(html, /not yet checked/);
        assert.match(html, /data-verified="0"/);
    });

    it('drill mode skips unverified future cells', () => {
        const block = {
            id: 'f-drill',
            type: 'conjugation-table',
            lemmaKey: 'לדבר',
            tense: 'future',
            mode: 'drill',
            cells: FUTURE_CELLS.slice(0, 3)
        };
        const forms = formsFor(FUTURE_CELLS.slice(0, 3), new Set([FUTURE_CELLS[2].formKey]));
        const queue = table.drillQueue([block], forms, () => 0.5);
        assert.equal(queue.some((item) => item.formKey === FUTURE_CELLS[2].formKey), false);
        assert.ok(queue.length >= 2);
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

describe('SPEC-005 multi-verb multi-tense drill cues (ac-4)', () => {
    const { createElement, makeDocument, walk, matches } = require('./helpers/fake-dom');

    function patchClosest(node) {
        if (!node || node.closest) return;
        node.closest = function (selector) {
            for (let cur = this; cur; cur = cur.parentNode) {
                if (cur.tagName && matches(cur, selector)) return cur;
            }
            return null;
        };
    }

    function wireTree(node) {
        patchClosest(node);
        walk(node, (child) => patchClosest(child));
    }

    function makePlanDoc() {
        const doc = makeDocument();
        const root = createElement('div');
        doc._mount = root;
        doc.appendChild = function (child) {
            child.parentNode = root;
            root.children.push(child);
            wireTree(child);
            return child;
        };
        doc.querySelectorAll = function (sel) {
            const out = [];
            walk(root, (child) => { if (matches(child, sel)) out.push(child); });
            return out;
        };
        doc.querySelector = function (sel) { return doc.querySelectorAll(sel)[0] || null; };
        return doc;
    }

    function mountDrillOwner(doc, blockId) {
        const section = createElement('section');
        section.setAttribute('data-block-type', 'conjugation-table');
        section.setAttribute('data-block-id', blockId);
        section.setAttribute('data-mode', 'drill');
        section.setAttribute('data-drill-owner', '1');
        section.setAttribute('data-lesson-slug', 'verb-workshop');
        wireTree(section);

        const prompt = createElement('p');
        prompt.setAttribute('data-drill-prompt', '');
        section.appendChild(prompt);

        const feedback = createElement('p');
        feedback.setAttribute('data-drill-feedback', '');
        feedback.hidden = true;
        section.appendChild(feedback);

        const reveal = createElement('p');
        reveal.setAttribute('data-drill-reveal', '');
        reveal.hidden = true;
        section.appendChild(reveal);

        const input = createElement('input');
        input.setAttribute('data-drill-input', '');
        section.appendChild(input);

        const check = createElement('button');
        check.setAttribute('type', 'button');
        check.setAttribute('data-drill-check', '');
        check.textContent = 'Check';
        section.appendChild(check);

        const next = createElement('button');
        next.setAttribute('type', 'button');
        next.setAttribute('data-drill-next', '');
        next.hidden = true;
        next.textContent = 'Next';
        section.appendChild(next);

        doc.appendChild(section);
        return section;
    }

    const lehachlit = {
        id: 'drill-lehachlit',
        type: 'conjugation-table',
        lemmaKey: 'lehachlit',
        tense: 'present',
        mode: 'drill',
        cells: [{ cell: 'ms', formKey: 'מַחְלִיט' }]
    };
    const livchorPresent = {
        id: 'drill-livchor-present',
        type: 'conjugation-table',
        lemmaKey: 'livchor',
        tense: 'present',
        mode: 'drill',
        cells: [{ cell: 'fs', formKey: 'בּוֹחֶרֶת' }]
    };
    const livchorPast = {
        id: 'drill-livchor-past',
        type: 'conjugation-table',
        lemmaKey: 'livchor',
        tense: 'past',
        mode: 'drill',
        cells: [{ cell: '1s', formKey: 'בָּחַרְתִּי' }]
    };
    const forms = {
        'מַחְלִיט': { key: 'מַחְלִיט', verified: true, accept: [], root: 'ח.ל.ט' },
        'בּוֹחֶרֶת': { key: 'בּוֹחֶרֶת', verified: true, accept: [], root: 'ב.ח.ר' },
        'בָּחַרְתִּי': { key: 'בָּחַרְתִּי', verified: true, accept: [], root: 'ב.ח.ר' }
    };

    it('queues lemmaKey + lemma + tense on interleaved multi-verb items', () => {
        const queue = table.drillQueue(
            [lehachlit, livchorPresent, livchorPast],
            forms,
            () => 0.999999
        );
        assert.equal(queue.length, 3);
        queue.forEach((item) => {
            assert.ok(item.lemmaKey === 'lehachlit' || item.lemmaKey === 'livchor', item.lemmaKey);
            assert.equal(item.lemma, item.lemmaKey);
            assert.ok(['present', 'past'].includes(item.tense), item.tense);
            assert.ok(item.cell);
            assert.ok(item.label);
        });
        assert.ok(queue.some((item) => item.lemmaKey === 'lehachlit' && item.tense === 'present'));
        assert.ok(queue.some((item) => item.lemmaKey === 'livchor' && item.tense === 'past'));
    });

    it('exposes data-drill-lemma/tense/cell and lemma·tense·label text on the prompt', () => {
        const doc = makePlanDoc();
        mountDrillOwner(doc, lehachlit.id);
        const plan = {
            units: [{
                lessons: [{
                    id: 'verb-workshop',
                    blocks: [lehachlit, livchorPresent, livchorPast]
                }]
            }]
        };
        table.bind(doc, { plan: plan, forms: forms, rng: () => 0.999999 });
        const prompt = doc.querySelector('[data-drill-prompt]');
        assert.ok(prompt);
        assert.ok(prompt.getAttribute('data-drill-cell'));
        assert.ok(prompt.getAttribute('data-drill-lemma'));
        assert.ok(prompt.getAttribute('data-drill-tense'));
        const lemma = prompt.getAttribute('data-drill-lemma');
        const tense = prompt.getAttribute('data-drill-tense');
        assert.ok(lemma === 'lehachlit' || lemma === 'livchor', lemma);
        assert.ok(tense === 'present' || tense === 'past', tense);
        assert.ok(prompt.textContent.indexOf(lemma) >= 0, prompt.textContent);
        if (tense === 'present') {
            assert.ok(prompt.textContent.indexOf('הווה / present') >= 0, prompt.textContent);
        } else {
            assert.ok(prompt.textContent.indexOf('עבר / past') >= 0, prompt.textContent);
        }
        assert.ok(prompt.textContent.indexOf(' · ') >= 0, prompt.textContent);
    });

    it('shows Done and clears drill attrs when the interleaved queue finishes', () => {
        const single = {
            id: 'drill-one',
            type: 'conjugation-table',
            lemmaKey: 'lehachlit',
            tense: 'future',
            mode: 'drill',
            cells: [{ cell: '1s', formKey: 'אַחְלִיט' }]
        };
        const singleForms = {
            'אַחְלִיט': { key: 'אַחְלִיט', verified: true, accept: ['אחליט'], root: 'ח.ל.ט' }
        };
        const doc = makePlanDoc();
        mountDrillOwner(doc, single.id);
        const plan = {
            units: [{ lessons: [{ id: 'verb-workshop', blocks: [single] }] }]
        };
        table.bind(doc, { plan: plan, forms: singleForms, rng: () => 0.5 });
        const prompt = doc.querySelector('[data-drill-prompt]');
        assert.equal(prompt.getAttribute('data-drill-lemma'), 'lehachlit');
        assert.equal(prompt.getAttribute('data-drill-tense'), 'future');
        assert.ok(prompt.textContent.indexOf('עתיד / future') >= 0, prompt.textContent);
        assert.ok(prompt.textContent.indexOf('lehachlit') >= 0, prompt.textContent);

        const input = doc.querySelector('[data-drill-input]');
        input.value = 'אַחְלִיט';
        doc.querySelector('[data-drill-check]').click();
        doc.querySelector('[data-drill-next]').click();

        assert.ok(prompt.textContent.indexOf('Done') >= 0, prompt.textContent);
        assert.equal(prompt.textContent, 'סיימתם. / Done.');
        assert.equal(prompt.getAttribute('data-drill-cell'), null);
        assert.equal(prompt.getAttribute('data-drill-lemma'), null);
        assert.equal(prompt.getAttribute('data-drill-tense'), null);
    });

    it('keeps empty non-owner drill sections in the DOM for CSS collapse', () => {
        const owner = Object.assign({}, lehachlit, { id: 'owner-drill' });
        const follower = Object.assign({}, livchorPast, { id: 'follower-drill' });
        const ownerHtml = table.render(owner, {
            forms: forms,
            lesson: { id: 'verb-workshop', blocks: [owner, follower] }
        });
        const followerHtml = table.render(follower, {
            forms: forms,
            lesson: { id: 'verb-workshop', blocks: [owner, follower] }
        });
        assert.match(ownerHtml, /data-drill-owner="1"/);
        assert.match(ownerHtml, /data-drill-station/);
        assert.match(followerHtml, /data-drill-owner="0"/);
        assert.match(followerHtml, /plan-block-conjugation-table/);
        assert.equal(followerHtml.includes('data-drill-station'), false);
        assert.match(followerHtml, /conj-header/);
        assert.equal(/conj-sentences|conj-questions|conj-extra-cells|conj-table/.test(followerHtml), false);
    });
});
