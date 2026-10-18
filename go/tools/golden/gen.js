#!/usr/bin/env node
'use strict';

/**
 * Go-1b golden generator. Runs the Node plan validators at this commit and
 * writes go/testdata/golden. Regenerate with Node 22.14.0 from the repo root:
 *
 *   node go/tools/golden/gen.js
 *
 * There is no Go -update flag. Hand-editing the goldens will fail CI.
 */

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const ROOT = path.resolve(__dirname, '../../..');
const OUT = path.join(ROOT, 'go', 'testdata', 'golden');
const validate = require(path.join(ROOT, 'server', 'plans', 'validate'));
const hebrew = require(path.join(ROOT, 'server', 'plans', 'hebrew-checks'));
const { basePlan } = require(path.join(ROOT, 'test', 'helpers', 'plan-fixtures'));
const { validatePlan, exactKey, strippedKey } = validate;
const { checkHebrew } = hebrew;

const PLANS_LEX = JSON.parse(fs.readFileSync(path.join(ROOT, 'test', 'fixtures', 'plans-1.1', 'lexicon.json'), 'utf8')).entries;
const LEDABER_LEX = JSON.parse(fs.readFileSync(path.join(ROOT, 'test', 'fixtures', 'lesson-types', 'lexicon-ledaber.json'), 'utf8')).entries;

function blob(rel) {
    return execFileSync('git', ['hash-object', path.join(ROOT, rel)], { encoding: 'utf8' }).trim();
}

function readRaw(rel) {
    return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function plan11(lesson) {
    return {
        schemaVersion: '1.1.0',
        id: 'mut-plan',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Mutation',
        language: { target: 'he', gloss: 'en' },
        units: [{ id: 'unit-1', title: 'Unit', theme: 'home', lessons: [lesson] }]
    };
}

function plan12(lessons) {
    const plan = plan11(lessons[0]);
    plan.schemaVersion = '1.2.0';
    plan.units[0].lessons = lessons;
    return plan;
}

function lessonOf(plan) {
    return plan.units[0].lessons[0];
}

function lexiconRuntime(spec) {
    if (!spec) return undefined;
    const rows = spec.rows || [];
    const lex = {};
    if (spec.find) {
        lex.find = (key) => {
            const want = exactKey(key);
            return rows.find((row) => exactKey(row.key) === want) || null;
        };
    }
    if (spec.listByStrippedKeys === true) {
        lex.listByStrippedKeys = (keys) => rows.filter((row) => keys.some((key) => strippedKey(row.key) === strippedKey(key)));
    } else if (spec.listByStrippedKeys === 'throw') {
        lex.listByStrippedKeys = () => {
            throw new Error('no such table');
        };
    }
    return lex;
}

const validateCases = [];

// A run of 64 or more identical code points is stored as @@REPEAT:hex:count@@.
// The Go test expands that marker before it calls the validators. The marker
// is only a file-size device; validatePlan still sees the full string.
function packRuns(s) {
    if (s.includes('@@REPEAT:')) throw new Error('input already contains a repeat marker');
    const chars = Array.from(s);
    let out = '';
    for (let i = 0; i < chars.length;) {
        let j = i + 1;
        while (j < chars.length && chars[j] === chars[i]) j++;
        const n = j - i;
        if (n >= 64) out += '@@REPEAT:' + chars[i].codePointAt(0).toString(16) + ':' + n + '@@';
        else out += chars[i].repeat(n);
        i = j;
    }
    return out;
}

function unpackRuns(s) {
    return s.replace(/@@REPEAT:([0-9a-f]+):(\d+)@@/g, (_, hex, n) => String.fromCodePoint(parseInt(hex, 16)).repeat(Number(n)));
}

function writeCase(id, input, options) {
    const packed = packRuns(input);
    if (unpackRuns(packed) !== input) throw new Error('pack/unpack mismatch for ' + id);
    const opt = {};
    const lex = lexiconRuntime(options.lexicon);
    if (lex) opt.lexicon = lex;
    if (options.snapshot !== undefined && options.snapshot !== null) opt.snapshot = options.snapshot;
    const doc = options.inputMode === 'object' ? JSON.parse(input) : input;
    const result = validatePlan(doc, opt);
    let hebrewOut = [];
    try {
        const parsed = JSON.parse(input);
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
            hebrewOut = checkHebrew(parsed, opt);
        }
    } catch (err) {
        hebrewOut = [];
    }
    validateCases.push({ id, input: packed, options, want: result, hebrew: hebrewOut });
    return { id, result, hebrew: hebrewOut };
}

function textCase(id, value, options) {
    const input = typeof value === 'string' ? value : JSON.stringify(value);
    return writeCase(id, input, options || { inputMode: 'string', lexicon: null, snapshot: null });
}

function rulesOf(found) {
    const set = new Set();
    for (const item of found.result.blocks.concat(found.result.warnings, found.result.informs, found.hebrew)) {
        set.add(item.rule);
    }
    return set;
}

// props.jsonl is one row per run of code points that share trim, UTF-16
// length, and \s, and whose toLowerCase and NFC are the identity. A code
// point whose toLowerCase or NFC differs gets its own row, with that output
// as a JSON string. The Go test builds the four positions around שלום.
// named-trim.jsonl is the explicit whitespace exhibit and is not derived
// from the table. phrases.jsonl holds multi-character strings, because NFC
// and case conversion do not always apply one code point at a time.
function jsstrGoldens() {
    const dir = path.join(OUT, 'jsstr');
    fs.mkdirSync(dir, { recursive: true });
    const word = 'שָׁלוֹם';
    const wordChars = Array.from(word);
    const named = [
        ['U+FEFF', 0xFEFF],
        ['U+0085', 0x0085],
        ['U+180E', 0x180E],
        ['U+200B', 0x200B],
        ['U+2028', 0x2028],
        ['U+2029', 0x2029],
        ['U+3000', 0x3000],
        ['U+00A0', 0x00A0]
    ];
    const astral = [0x10000, 0x1F600, 0x1F4A9, 0x1D11E, 0x20000, 0x2F800, 0xE0100, 0x10FFFF];

    function q(s) {
        return JSON.stringify(s);
    }
    function hex(cp) {
        const h = cp.toString(16);
        return h.length < 4 ? h.padStart(4, '0') : h;
    }
    function props(cp) {
        const ch = String.fromCodePoint(cp);
        const lower = ch.toLowerCase();
        const nfc = ch.normalize('NFC');
        let flags = '';
        if (ch.trim() === '') flags += 't';
        if (/\s/.test(ch)) flags += 's';
        if (!flags) flags = '-';
        return {
            utf16: ch.length,
            flags,
            lower: lower === ch ? '' : q(lower),
            nfc: nfc === ch ? '' : q(nfc)
        };
    }
    function format(start, end, row) {
        const span = start === end ? hex(start) : hex(start) + '-' + hex(end);
        return [span, String(row.utf16), row.flags, row.lower, row.nfc].join('\t');
    }
    const lines = [
        '# start[-end]\tutf16\tflags\tlower\tnfc',
        '# flags: t = trim removes the character, s = /\\s/ matches it, - = neither',
        '# lower and nfc are JSON strings only when they differ from the character'
    ];
    for (let i = 0; i <= 0xFFFF;) {
        const cur = props(i);
        if (cur.lower || cur.nfc) {
            lines.push(format(i, i, cur));
            i++;
            continue;
        }
        let j = i;
        while (j + 1 <= 0xFFFF) {
            const next = props(j + 1);
            if (next.lower || next.nfc || next.utf16 !== cur.utf16 || next.flags !== cur.flags) break;
            j++;
        }
        lines.push(format(i, j, cur));
        i = j + 1;
    }
    astral.forEach((cp) => lines.push(format(cp, cp, props(cp))));
    fs.writeFileSync(path.join(dir, 'props.jsonl'), lines.join('\n') + '\n');

    const phrases = [
        ['word', word],
        ['alef-dagesh-qamats', 'א\u05BC\u05B8'],
        ['shin-qamats-shin-dot', 'ש\u05B8\u05C1'],
        ['surrogate-then-nfd', '\uD800א\u05BC\u05B8'],
        ['cafe-combining', 'cafe\u0301'],
        ['dotted-i', 'İ'],
        ['final-sigma', 'ΣΣ'],
        ['sharp-s', 'ẞ'],
        ['ligature-fi', 'ﬁ'],
        ['emoji', '😀'],
        ['lone-surrogate', '\uD800'],
        ['two-high-surrogates', '\uD800\uD800'],
        ['empty', '']
    ];
    const phraseLines = ['# name\tinput\tcodePoints\tutf16\tlower\tnfc\ttrim\tcollapse'];
    phrases.forEach(([label, input]) => {
        phraseLines.push([
            label,
            q(input),
            String(Array.from(input).length),
            String(input.length),
            q(input.toLowerCase()),
            q(input.normalize('NFC')),
            q(input.trim()),
            q(input.replace(/\s+/g, ' '))
        ].join('\t'));
    });
    fs.writeFileSync(path.join(dir, 'phrases.jsonl'), phraseLines.join('\n') + '\n');

    function positions(ch) {
        return [
            ['lead', ch + word],
            ['trail', word + ch],
            ['interior', wordChars[0] + ch + wordChars.slice(1).join('')],
            ['repeated', ch + ch + word + ch + ch]
        ];
    }
    const namedLines = [];
    named.forEach(([label, cp]) => {
        const ch = String.fromCodePoint(cp);
        namedLines.push(label + '-only\t' + q(ch) + '\t' + q(ch.trim()));
        for (const [pos, input] of positions(ch)) {
            namedLines.push(label + '-' + pos + '\t' + q(input) + '\t' + q(input.trim()));
        }
    });
    fs.writeFileSync(path.join(dir, 'named-trim.jsonl'), namedLines.join('\n') + '\n');
}

function jsonGoldens() {
    const texts = [
        '{"b":1,"0":2,"a":3,"10":4,"2":5}',
        '{"a":1,"b":2,"a":3}',
        '[1e21,1e20,1e-7,0.000001,-0,0,-0.0]',
        '"\\ud800"',
        '"\\ud800\\ud800"',
        '"\\ud800\\ude00"',
        '"\u2028\u2029"',
        '"\\u0001\\u001f\\b\\u007f"',
        '"<" ',
        'null',
        '[]',
        '{}'
    ];
    const cases = texts.map((input) => {
        const value = JSON.parse(input);
        return { input, canonical: JSON.stringify(value) };
    });
    fs.writeFileSync(path.join(OUT, 'json.json'), JSON.stringify(cases, null, 2) + '\n');
}

function mutations() {
    const cases = [];
    const add = (id, value, options) => {
        const input = typeof value === 'string' ? value : JSON.stringify(value);
        cases.push(writeCase(id, input, Object.assign({ inputMode: 'string', lexicon: null, snapshot: null }, options)));
    };

    add('not-json', 'not json');
    add('non-object-array', '[]');
    add('non-object-number', '42');
    add('unknown-schema-version', { schemaVersion: '9.9.9', id: 'x' });
    add('duplicate-schema-version', '{"schemaVersion":"1.1.0","id":"dup","schemaVersion":"9.9.9"}');

    const unknownOrderA = plan11({ id: 'l', title: 'L', level: 'basic', visibility: 'hidden', buildsOn: [], newWords: ['שָׁלוֹם'], recycles: [], blocks: [] });
    unknownOrderA.zzz = 1;
    unknownOrderA.aaa = 2;
    add('key-order-zzz-aaa', unknownOrderA);
    const unknownOrderB = {};
    unknownOrderB.schemaVersion = unknownOrderA.schemaVersion;
    unknownOrderB.id = unknownOrderA.id;
    unknownOrderB.revision = unknownOrderA.revision;
    unknownOrderB.status = unknownOrderA.status;
    unknownOrderB.audience = unknownOrderA.audience;
    unknownOrderB.assigneeId = unknownOrderA.assigneeId;
    unknownOrderB.title = unknownOrderA.title;
    unknownOrderB.language = unknownOrderA.language;
    unknownOrderB.units = unknownOrderA.units;
    unknownOrderB.aaa = 2;
    unknownOrderB.zzz = 1;
    add('key-order-aaa-zzz', unknownOrderB);

    const pretty = JSON.stringify(basePlan(), null, 2) + '\n\n';
    add('object-mode-base', JSON.stringify(basePlan()), { inputMode: 'object' });
    add('pretty-whitespace-base', pretty);

    const n9plan = basePlan();
    n9plan.title = 'א'.repeat(300000);
    add('n9-plan-oversize', n9plan);
    const n9lesson = basePlan();
    lessonOf(n9lesson).title = 'א'.repeat(80000);
    add('n9-lesson-oversize', n9lesson);
    const n9block = basePlan();
    lessonOf(n9block).blocks[2].passage = 'א'.repeat(20000);
    add('n9-block-oversize', n9block);

    const emoji = '😀';
    const longOk = basePlan();
    lessonOf(longOk).blocks[2].passage = emoji.repeat(4000);
    add('maxlength-4000-emoji', longOk);
    const longBad = basePlan();
    lessonOf(longBad).blocks[2].passage = emoji.repeat(4001);
    add('maxlength-4001-emoji', longBad);

    const nbsp = basePlan();
    lessonOf(nbsp).blocks[2].passage = '<\u00A0b';
    add('markup-nbsp', nbsp);
    const nel = basePlan();
    lessonOf(nel).blocks[2].passage = '<\u0085b';
    add('markup-0085', nel);
    const html = basePlan();
    lessonOf(html).blocks[2].passage = '<b>no</b>';
    add('html-tag', html);

    const fef = basePlan();
    lessonOf(fef).blocks[2].passage = '\uFEFF';
    add('trim-fef-only-passage', fef);

    // Unicode 16 scalars Go 1.26's unicode package still treats as unassigned.
    // Edge symbols are stripped by trimEdges; a new letter trips field-language.
    const edgeSymbol = (id, cp) => {
        const ch = String.fromCodePoint(cp);
        const plan = basePlan();
        const lesson = lessonOf(plan);
        lesson.newWords = ['שלום' + ch];
        lesson.blocks[0].words.push({ id: 'w-edge', word: 'שלום' + ch, meaning: 'hello', sentence: 'שלום.' });
        add(id, plan);
    };
    edgeSymbol('ucd-edge-1fae9', 0x1FAE9);
    edgeSymbol('ucd-edge-1fa89', 0x1FA89);
    edgeSymbol('ucd-edge-1fac6', 0x1FAC6);
    edgeSymbol('ucd-edge-2427', 0x2427);
    edgeSymbol('ucd-edge-2428', 0x2428);
    edgeSymbol('ucd-edge-2429', 0x2429);
    const letterOnly = (id, cp) => {
        const plan = basePlan();
        lessonOf(plan).blocks[0].words[0].word = String.fromCodePoint(cp);
        add(id, plan);
    };
    letterOnly('ucd-letter-1c89', 0x1C89);
    letterOnly('ucd-letter-a7cb', 0xA7CB);
    edgeSymbol('ucd-mark-0897', 0x0897);
    edgeSymbol('ucd-number-10d40', 0x10D40);
    edgeSymbol('ucd-punct-1b4e', 0x1B4E);
    const nelField = basePlan();
    lessonOf(nelField).blocks[2].passage = '\u0085hello';
    add('trim-0085-not-skipped', nelField);

    const nfd = basePlan();
    lessonOf(nfd).blocks[0].words[0].word = 'א\u05BC\u05B8';
    lessonOf(nfd).newWords = ['א\u05BC\u05B8', 'בַּיִת'];
    add('pointing-nfd', nfd);
    const stray = basePlan();
    lessonOf(stray).blocks[0].words[0].word = '\u05B7בית';
    add('pointing-stray-vowel', stray);
    const doubled = basePlan();
    lessonOf(doubled).blocks[0].words[0].word = 'ב\u05B7\u05B8ית';
    add('pointing-doubled-vowel', doubled);
    const sur = basePlan();
    lessonOf(sur).title = 'title\uD800';
    add('lone-surrogate-title', sur);

    const lang = basePlan();
    lessonOf(lang).blocks[0].words[0].meaning = 'שָׁלוֹם בַּיִת';
    add('field-language-meaning', lang);

    const audio = basePlan();
    lessonOf(audio).blocks[0].words[0].audioPath = 'http://cdn.example/a.mp3';
    add('audio-path-absolute', audio);

    const n1 = basePlan();
    const words = ['אֶחָד', 'שְׁנַיִם', 'שָׁלוֹשׁ', 'אַרְבַּע', 'חָמֵשׁ', 'שֵׁשׁ', 'שֶׁבַע', 'שְׁמוֹנֶה', 'תֵּשַׁע'];
    lessonOf(n1).newWords = words;
    add('n1-nine-new-words', n1);

    const n2 = basePlan();
    lessonOf(n2).buildsOn = ['earlier'];
    lessonOf(n2).recycles = [{ word: 'שָׁלוֹם', fromLessonId: 'earlier' }];
    add('n2-few-recycles', n2);

    const n3 = basePlan();
    lessonOf(n3).blocks[1].paragraphs[0].text = 'דָּנִי גָּר בְּעִיר גְּדוֹלָה עִם כֶּלֶב וְחָתוּל וְעוֹף וְסוּס.';
    add('n3-unknown-story', n3);
    const n4 = basePlan();
    lessonOf(n4).blocks[2].passage = 'דָּנִי גָּר בְּעִיר גְּדוֹלָה עִם כֶּלֶב וְחָתוּל.';
    add('n4-listening-only', n4);

    const n5 = basePlan();
    lessonOf(n5).blocks.push({ id: 'intro2', type: 'introduction', words: [{ id: 'w3', word: 'כֶּלֶב', meaning: 'dog' }] });
    add('n5-two-introductions', n5);

    const n6 = basePlan();
    lessonOf(n6).newWords = ['כֶּלֶב'];
    lessonOf(n6).blocks[0].words.push({ id: 'w3', word: 'כֶּלֶב', meaning: 'dog', sentence: 'זֶה כֶּלֶב.' });
    add('n6-once', n6);

    const n7 = basePlan();
    add('n7-too-many-introductions', n7);

    const n8 = basePlan();
    lessonOf(n8).blocks[0].words[0].image = { mediaId: 'pic-1' };
    add('n8-missing-alt', n8);

    const words25 = [];
    for (let i = 0; i < 25; i++) words25.push({ id: 'w' + i, word: 'שָׁלוֹם', meaning: 'hello ' + i });
    const n10 = basePlan();
    lessonOf(n10).blocks[0].words = words25;
    add('n10-too-many-words', n10);

    const n12 = basePlan();
    lessonOf(n12).blocks.push({ id: 'flu', type: 'fluency-pass', seconds: 30, source: 'vocab', words: ['שָׁלוֹם'] });
    add('n12-fluency-list', n12);

    const kin = ['אֵם', 'אָב', 'אָח', 'אָחוֹת', 'בֵּן', 'בַּת'];
    const closed = basePlan();
    lessonOf(closed).newWords = kin;
    lessonOf(closed).blocks[0].words = kin.map((word, i) => ({ id: 'k' + i, word, meaning: 'kin ' + i }));
    add('semantic-set-kinship', closed);

    const freq = basePlan();
    freq.units[0].frequencyBand = 'top-1000';
    add('frequency-band', freq);

    const focus = basePlan();
    lessonOf(focus).blocks[0].pattern = {
        heading: 'Frames',
        rules: [{ title: 'One', english: 'A frame.', example: 'שָׁלוֹם', focus: ['מִלָּה'] }]
    };
    add('pattern-focus-missing', focus);

    const unpointed = basePlan();
    lessonOf(unpointed).newWords = ['שלום', 'בית'];
    lessonOf(unpointed).blocks[0].words[0].word = 'שלום';
    lessonOf(unpointed).blocks[0].words[1].word = 'בית';
    add('pointed-word-missing', unpointed);

    const roman = basePlan();
    lessonOf(roman).blocks[0].cloze.items[0].answer = 'house';
    add('romanized-answer', roman);

    const questions = basePlan();
    lessonOf(questions).blocks[1].questions = [{ id: 'q1', question: 'Who?', answer: 'דָּנִי' }];
    add('questions-present', questions);

    const personal = basePlan();
    personal.audience = 'personal';
    personal.assigneeId = 'learner-1';
    add('queue-order-personal', personal);

    const meaning = basePlan();
    lessonOf(meaning).newWords = ['כֶּלֶב', 'בַּיִת'];
    lessonOf(meaning).blocks[0].words[0].word = 'כֶּלֶב';
    lessonOf(meaning).blocks[0].words[0].meaning = '   ';
    add('meaning-blank', meaning);

    const placed = basePlan();
    lessonOf(placed).blocks[1].image = { mediaId: 'pic-1', alt: 'a dog' };
    add('image-placement-story', placed);

    const recall = basePlan();
    lessonOf(recall).blocks.push({ id: 'type', type: 'type-it', mediaId: 'pic-1', prompt: 'שָׁלוֹם', answer: 'hello' });
    add('picture-on-recall', recall);

    const tableImage = basePlan();
    lessonOf(tableImage).blocks.push({
        id: 'table',
        type: 'conjugation-table',
        lemmaKey: 'לַחְשֹׁב',
        tense: 'present',
        image: { mediaId: 'pic-1', alt: 'chart' },
        cells: [{ id: 'c1', cell: 'ms', formKey: 'חוֹשֵׁב', gloss: 'thinks' }]
    });
    add('form-recall-image-on-table', tableImage);

    const guide = basePlan();
    lessonOf(guide).recipe = 'guide';
    lessonOf(guide).buildsOn = ['earlier'];
    add('n2-guide-exempt', guide);

    const n15 = basePlan();
    lessonOf(n15).recipe = 'review-worksheet';
    lessonOf(n15).buildsOn = ['earlier'];
    lessonOf(n15).recycles = [
        { word: 'שָׁלוֹם', fromLessonId: 'earlier' },
        { word: 'בַּיִת', fromLessonId: 'earlier' },
        { word: 'כֶּלֶב', fromLessonId: 'earlier' }
    ];
    add('n15-review-not-first', n15);

    const n16 = plan12([{
        id: 'talk',
        title: 'Talk',
        level: 'basic',
        visibility: 'hidden',
        recipe: 'topic-conversation',
        buildsOn: [],
        newWords: ['שָׁלוֹם'],
        recycles: [],
        blocks: [
            { id: 'intro', type: 'introduction', words: [{ id: 'w1', word: 'שָׁלוֹם', meaning: 'hello', sentence: 'שָׁלוֹם.' }] },
            { id: 'listen', type: 'listening', passage: 'שָׁלוֹם שָׁלוֹם.', audioPath: 'listen/one.mp3' }
        ]
    }]);
    add('n16-no-personal-question', n16);

    const same = {
        id: 'same',
        title: 'Same',
        level: 'basic',
        visibility: 'hidden',
        recipe: 'review-worksheet',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{ id: 'only', type: 'listening', passage: 'שָׁלוֹם.', audioPath: 'listen/one.mp3' }]
    };
    add('n17-three-recipe-run', plan12([clone(same), clone(same), Object.assign(clone(same), { id: 'same-3' })]));

    const grouped = plan12([Object.assign(clone(same), {
        id: 'groups',
        recipe: 'topic-conversation',
        newWords: ['אֶחָד', 'שְׁנַיִם', 'שָׁלוֹשׁ', 'אַרְבַּע', 'חָמֵשׁ', 'שֵׁשׁ', 'שֶׁבַע', 'שְׁמוֹנֶה', 'תֵּשַׁע'],
        blocks: [{
            id: 'intro',
            type: 'introduction',
            words: ['אֶחָד', 'שְׁנַיִם', 'שָׁלוֹשׁ', 'אַרְבַּע', 'חָמֵשׁ', 'שֵׁשׁ'].map((word, i) => ({
                id: 'g' + i, word, meaning: 'n' + i, group: 1, sentence: word + '.'
            }))
        }]
    })]);
    add('n18-group-and-count', grouped);

    const verse = plan11({
        id: 'verse',
        title: 'Verse',
        level: 'basic',
        visibility: 'hidden',
        recipe: 'verse-reading',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{
            id: 'v',
            type: 'verse-reading',
            ref: { book: 'Gen', chapter: 1, verse: 1, source: '' },
            lines: [{ id: 'ln', pointed: 'בְּרֵאשִׁית' }]
        }]
    });
    add('n19-empty-source', verse);

    function exercise(kind, items) {
        return plan12([{
            id: 'ex',
            title: 'Exercise',
            level: 'basic',
            visibility: 'hidden',
            recipe: 'review-worksheet',
            buildsOn: [],
            newWords: [],
            recycles: [],
            blocks: [{ id: 'block', type: 'exercise', kind, items }]
        }]);
    }
    add('n20-answer-not-choice', exercise('choose', [{
        id: 'i1', prompt: 'Pick', choices: ['שָׁלוֹם', 'בַּיִת'], answer: 'כֶּלֶב'
    }]));
    add('n21-alt-not-permutation', exercise('unscramble', [{
        id: 'i1', words: ['אֶחָד', 'שְׁנַיִם', 'שָׁלוֹשׁ'], alt: [['אֶחָד', 'שְׁנַיִם', 'אַרְבַּע']], english: 'one two three'
    }]));
    add('n22-bad-audio', exercise('dictation', [{
        id: 'i1', audioPath: '/abs/no.mp3', answer: 'שָׁלוֹם'
    }]));

    const n23 = plan12([{
        id: 'crowd',
        title: 'Crowd',
        level: 'basic',
        visibility: 'hidden',
        recipe: 'topic-conversation',
        buildsOn: [],
        newWords: ['שָׁלוֹם'],
        recycles: [],
        blocks: [0, 1, 2].map((i) => ({
            id: 'ex' + i,
            type: 'exercise',
            kind: 'true-false',
            items: [{ id: 'i' + i, statement: 'שָׁלוֹם is a greeting.', answer: true }]
        }))
    }]);
    add('n23-three-exercises', n23);

    const later = plan11(lessonOf(basePlan()));
    later.units[0].lessons.push({
        id: 'lesson-2',
        title: 'Lesson two',
        level: 'basic',
        visibility: 'hidden',
        buildsOn: ['lesson-1'],
        newWords: [],
        recycles: [
            { word: 'שָׁלוֹם', fromLessonId: 'lesson-1' },
            { word: 'בַּיִת', fromLessonId: 'lesson-1' },
            { word: 'כֶּלֶב', fromLessonId: 'lesson-1' }
        ],
        blocks: [{ id: 'listen2', type: 'listening', passage: 'רַק בַּיִת.', audioPath: 'listen/two.mp3' }]
    });
    add('later-context-and-checkpoint', later);
    const recallDay = basePlan();
    lessonOf(recallDay).blocks.push({
        id: 'rev',
        type: 'spaced-review-checkpoint',
        wordKeys: ['שָׁלוֹם'],
        fromLessons: ['earlier']
    });
    add('recall-day-checkpoint', recallDay);

    const v100 = basePlan();
    v100.schemaVersion = '1.0.0';
    add('v100-rejects-explainer', v100);

    const legacy = basePlan();
    legacy.migration = { legacy: true };
    lessonOf(legacy).newWords = ['אֶחָד', 'שְׁנַיִם', 'שָׁלוֹשׁ', 'אַרְבַּע', 'חָמֵשׁ', 'שֵׁשׁ', 'שֶׁבַע', 'שְׁמוֹנֶה', 'תֵּשַׁע'];
    add('n1-legacy-warn', legacy);

    const lexRows = [
        { key: 'שָלוֹם', gloss_en: 'hello', translit: 'shalom', verified: 1 },
        { key: 'בַּיִת', gloss_en: 'house', translit: 'bayit', verified: true }
    ];
    const lexSpec = { find: true, listByStrippedKeys: true, rows: lexRows };
    add('lexicon-form-verified', basePlan(), { lexicon: lexSpec });
    add('lexicon-list-throws', basePlan(), { lexicon: { find: true, listByStrippedKeys: 'throw', rows: lexRows } });
    add('lexicon-find-only', basePlan(), { lexicon: { find: true, listByStrippedKeys: false, rows: lexRows } });

    const consistent = basePlan();
    lessonOf(consistent).blocks[4].findIn[0].word = 'בָּיִת';
    add('pointing-consistency', consistent, { lexicon: lexSpec });

    const stalePlan = basePlan();
    const snap = basePlan();
    lessonOf(stalePlan).blocks[0].words[0].word = 'שָׁלוֹם!';
    add('audio-stale-object', stalePlan, { snapshot: snap });
    add('audio-stale-string', stalePlan, { snapshot: JSON.stringify(snap) });
    add('audio-stale-unchanged', basePlan(), { snapshot: JSON.stringify(basePlan()) });

    const noForm = plan11({
        id: 'verbs',
        title: 'Verbs',
        level: 'basic',
        visibility: 'hidden',
        recipe: 'verb-workshop',
        buildsOn: [],
        newWords: [],
        recycles: [],
        blocks: [{
            id: 'table',
            type: 'conjugation-table',
            lemmaKey: 'לַחְשֹׁב',
            tense: 'present',
            cells: [
                { id: 'c1', cell: 'ms', formKey: 'חוֹשֵׁב', gloss: 'thinks' },
                { id: 'c2', cell: 'ms', formKey: 'חוֹשֵׁב', gloss: 'thinks again' }
            ]
        }]
    });
    add('n13-no-lexicon', noForm);
    add('n13-n14-lexicon', noForm, {
        lexicon: {
            find: true,
            listByStrippedKeys: true,
            rows: [
                { key: 'חוֹשֵׁב', kind: 'form', lemma_key: 'לַחְשֹׁב', tense: 'present', cell: 'ms', verified: false },
                { key: 'לַחְשֹׁב', kind: 'lemma', lemma_key: null, tense: null, cell: null, verified: true }
            ]
        }
    });

    return cases;
}

function fixtures() {
    const found = [];
    const specs = [
        ['docs/decisions/ADR-0003-assets/samples/anal-sex-vocab.plan.json', 'plans'],
        ['docs/decisions/ADR-0003-assets/samples/beginner-unit-1.plan.json', 'plans'],
        ['docs/decisions/ADR-0003-assets/samples/my-morning.plan.json', 'plans'],
        ['docs/decisions/ADR-0003-assets/samples/shai-date-indirect-speech.plan.json', 'plans']
    ];
    for (const name of fs.readdirSync(path.join(ROOT, 'test', 'fixtures', 'plans-1.1'))) {
        if (!name.endsWith('.plan.json')) continue;
        specs.push(['test/fixtures/plans-1.1/' + name, 'plans']);
    }
    for (const name of fs.readdirSync(path.join(ROOT, 'test', 'fixtures', 'lesson-types'))) {
        if (!name.endsWith('.json') || name === 'lexicon-ledaber.json') continue;
        specs.push(['test/fixtures/lesson-types/' + name, 'ledaber']);
    }
    for (const [rel, which] of specs) {
        const id = 'fixture-' + rel.split('/').pop().replace(/\.json$/, '');
        const rows = which === 'plans' ? PLANS_LEX : LEDABER_LEX;
        found.push(writeCase(id, readRaw(rel), {
            inputMode: 'string',
            lexicon: { find: true, listByStrippedKeys: true, rows },
            snapshot: null
        }));
    }
    return found;
}

function main() {
    fs.rmSync(OUT, { recursive: true, force: true });
    fs.mkdirSync(OUT, { recursive: true });
    jsstrGoldens();
    jsonGoldens();
    const found = fixtures().concat(mutations());
    fs.writeFileSync(path.join(OUT, 'validate.jsonl'), validateCases.map((item) => JSON.stringify(item)).join('\n') + '\n');
    const counts = {};
    const byRule = {};
    const byCase = {};
    for (const item of found) {
        const rules = Array.from(rulesOf(item)).sort();
        byCase[item.id] = rules;
        for (const rule of rules) {
            counts[rule] = (counts[rule] || 0) + 1;
            if (!byRule[rule]) byRule[rule] = [];
            byRule[rule].push(item.id);
        }
    }
    for (const rule of Object.keys(byRule)) byRule[rule].sort();
    const silent = ['N4', 'semantic-set', 'form-recall-image'];
    for (const rule of silent) {
        if (counts[rule]) throw new Error(rule + ' fired and must stay silent');
        counts[rule] = 0;
        byRule[rule] = [];
    }
    const expected = [
        'schema', 'N1', 'N2', 'N3', 'N5', 'N6', 'N7', 'N8', 'N9', 'N10', 'N11', 'N12', 'N13', 'N14',
        'N15', 'N16', 'N17', 'N18', 'N19', 'N20', 'N21', 'N22', 'N23',
        'later-context', 'later-checkpoint', 'recall-day', 'frequency-band', 'pattern-focus',
        'pointed-word', 'romanized-answer', 'questions', 'queue-order', 'meaning', 'image-placement',
        'picture-on-recall', 'closed-set', 'html', 'audio-path',
        'pointing-form', 'field-language', 'lexicon-form', 'pointing-consistency', 'audio-stale'
    ];
    const missing = expected.filter((rule) => !counts[rule]);
    if (missing.length) {
        console.error('missing rules:', missing.join(', '));
        console.error('seen:', Object.keys(counts).sort().join(', '));
        process.exit(1);
    }
    const manifest = {
        node: process.version,
        gitSha: {
            'server/plans/validate.js': blob('server/plans/validate.js'),
            'server/plans/hebrew-checks.js': blob('server/plans/hebrew-checks.js'),
            'server/plans/id-path.js': blob('server/plans/id-path.js')
        }
    };
    fs.writeFileSync(path.join(OUT, 'MANIFEST.json'), JSON.stringify(manifest, null, 2) + '\n');
    const sortObj = (obj) => {
        const out = {};
        Object.keys(obj).sort().forEach((key) => { out[key] = obj[key]; });
        return out;
    };
    const coverage = {
        cases: found.length,
        rules: sortObj(counts),
        byRule: sortObj(byRule),
        byCase: sortObj(byCase),
        silent: {
            N4: 'n4-listening-only trips listening coverage; N3 reports it and N4 is never emitted',
            'semantic-set': 'semantic-set-kinship is one category; closed-set reports it and semantic-set is never emitted',
            'form-recall-image': 'form-recall-image-on-table puts an image on a conjugation table; image-placement reports it and form-recall-image is never emitted'
        }
    };
    fs.writeFileSync(path.join(OUT, 'coverage.json'), JSON.stringify(coverage, null, 2) + '\n');
    execFileSync(process.execPath, [path.join(__dirname, '..', 'ucd', 'props.js')], { stdio: 'inherit' });
    console.log('wrote', found.length, 'cases');
    console.log(JSON.stringify(counts, null, 2));
}

main();
