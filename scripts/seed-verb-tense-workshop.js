/**
 * Schema 1.2.0 multi-binyan, multi-tense verb workshop seed.
 * Four lemmas, 88 forms (present 4 + past 9 + future 9 each), draft plan
 * `verb-tense-workshop` / lesson `decide-choose-believe-win`, and a human
 * verification checklist. Lexicon rows default to verified=0. A second run
 * sets verified only on forms whose checklist line is ticked.
 * Idempotent. Refuses when a target key exists with source other than admin.
 * Writes go through lexicon.upsert, lexicon.setVerified, and plan ops.
 * Does not modify scripts/seed-starter-content.js or data/hebrew.sqlite.
 * Provenance: INIT-013/SPEC-007
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { stripPoints } = require('../server/lexicon/validate');
const { rootDir } = require('../server/paths');

const ACTOR = 'admin:verb-tense-workshop';
const PRESENT_CELLS = ['ms', 'fs', 'mp', 'fp'];
const PAST_CELLS = ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'];
const FUTURE_CELLS = PAST_CELLS.slice();
const PLAN_ID = 'verb-tense-workshop';
const LESSON_ID = 'decide-choose-believe-win';

/**
 * @param {string} text
 * @returns {string}
 */
function nfc(text) {
    return String(text).normalize('NFC');
}

/**
 * @param {string} pointed
 * @param {string[]} [extras]
 * @returns {string[]}
 */
function accepts(pointed, extras) {
    const bare = stripPoints(pointed);
    const list = [];
    [bare].concat(extras || []).forEach((item) => {
        const text = stripPoints(item).normalize('NFC');
        if (text && !list.includes(text)) list.push(text);
    });
    return list;
}

/**
 * Future 2ms and 3fs are homographs. Lexicon keys must be unique, so 3fs
 * rows append `-3fs` (same pattern as test/fixtures/lesson-types/lexicon-ledaber.json).
 * @param {string} pointed
 * @param {string} cell
 * @param {string} tense
 * @returns {string}
 */
function formKey(pointed, cell, tense) {
    const base = nfc(pointed);
    if (tense === 'future' && cell === '3fs') return base + '-3fs';
    return base;
}

const DECIDE = {
    key: nfc('לְהַחְלִיט'),
    gloss_en: 'to decide',
    pos: 'verb',
    root: 'ח.ל.ט',
    binyan: 'hifil',
    // guttural is not a registry root_class; regular hifil of ח.ל.ט
    root_class: 'regular',
    acceptExtra: ['להחליט'],
    slug: 'decide'
};

const BELIEVE = {
    key: nfc('לְהַאֲמִין'),
    gloss_en: 'to believe',
    pos: 'verb',
    root: 'א.מ.נ',
    binyan: 'hifil',
    // alef-guttural; registry has no "guttural" — irregular marks the special stem
    root_class: 'irregular',
    acceptExtra: ['להאמין'],
    slug: 'believe'
};

const WIN = {
    key: nfc('לְנַצֵּחַ'),
    gloss_en: 'to win',
    pos: 'verb',
    root: 'נ.צ.ח',
    binyan: 'piel',
    root_class: 'irregular',
    acceptExtra: ['לנצח'],
    slug: 'win'
};

const CHOOSE = {
    key: nfc('לִבְחוֹר'),
    gloss_en: 'to choose',
    pos: 'verb',
    root: 'ב.ח.ר',
    binyan: 'paal',
    root_class: 'irregular',
    acceptExtra: ['לבחור', 'לבחר'],
    slug: 'choose'
};

const WORKSHOP = [DECIDE, CHOOSE, BELIEVE, WIN];

/**
 * Pointed paradigms. Dictionary pointing; class-doc typos are never used.
 * @param {object} spec
 * @returns {{ present: Record<string, string>, past: Record<string, string>, future: Record<string, string> }}
 */
function paradigmOf(spec) {
    if (spec.slug === 'decide') {
        return {
            present: {
                ms: nfc('מַחְלִיט'),
                fs: nfc('מַחְלִיטָה'),
                mp: nfc('מַחְלִיטִים'),
                fp: nfc('מַחְלִיטוֹת')
            },
            past: {
                '1s': nfc('הֶחְלַטְתִּי'),
                '2ms': nfc('הֶחְלַטְתָּ'),
                '2fs': nfc('הֶחְלַטְתְּ'),
                '3ms': nfc('הֶחְלִיט'),
                '3fs': nfc('הֶחְלִיטָה'),
                '1p': nfc('הֶחְלַטְנוּ'),
                '2mp': nfc('הֶחְלַטְתֶּם'),
                '2fp': nfc('הֶחְלַטְתֶּן'),
                '3p': nfc('הֶחְלִיטוּ')
            },
            future: {
                '1s': nfc('אַחְלִיט'),
                '2ms': nfc('תַּחְלִיט'),
                '2fs': nfc('תַּחְלִיטִי'),
                '3ms': nfc('יַחְלִיט'),
                '3fs': nfc('תַּחְלִיט'),
                '1p': nfc('נַחְלִיט'),
                '2mp': nfc('תַּחְלִיטוּ'),
                '2fp': nfc('תַּחְלֵטְנָה'),
                '3p': nfc('יַחְלִיטוּ')
            }
        };
    }
    if (spec.slug === 'believe') {
        return {
            present: {
                ms: nfc('מַאֲמִין'),
                fs: nfc('מַאֲמִינָה'),
                mp: nfc('מַאֲמִינִים'),
                fp: nfc('מַאֲמִינוֹת')
            },
            past: {
                '1s': nfc('הֶאֱמַנְתִּי'),
                '2ms': nfc('הֶאֱמַנְתָּ'),
                '2fs': nfc('הֶאֱמַנְתְּ'),
                '3ms': nfc('הֶאֱמִין'),
                '3fs': nfc('הֶאֱמִינָה'),
                '1p': nfc('הֶאֱמַנּוּ'),
                '2mp': nfc('הֶאֱמַנְתֶּם'),
                '2fp': nfc('הֶאֱמַנְתֶּן'),
                '3p': nfc('הֶאֱמִינוּ')
            },
            future: {
                '1s': nfc('אֶאֱמִין'),
                '2ms': nfc('תַּאֲמִין'),
                '2fs': nfc('תַּאֲמִינִי'),
                '3ms': nfc('יַאֲמִין'),
                '3fs': nfc('תַּאֲמִין'),
                '1p': nfc('נַאֲמִין'),
                '2mp': nfc('תַּאֲמִינוּ'),
                '2fp': nfc('תַּאֲמֵנָּה'),
                '3p': nfc('יַאֲמִינוּ')
            }
        };
    }
    if (spec.slug === 'win') {
        return {
            present: {
                ms: nfc('מְנַצֵּחַ'),
                fs: nfc('מְנַצַּחַת'),
                mp: nfc('מְנַצְּחִים'),
                fp: nfc('מְנַצְּחוֹת')
            },
            past: {
                '1s': nfc('נִצַּחְתִּי'),
                '2ms': nfc('נִצַּחְתָּ'),
                '2fs': nfc('נִצַּחְתְּ'),
                '3ms': nfc('נִצֵּחַ'),
                '3fs': nfc('נִצְּחָה'),
                '1p': nfc('נִצַּחְנוּ'),
                '2mp': nfc('נִצַּחְתֶּם'),
                '2fp': nfc('נִצַּחְתֶּן'),
                '3p': nfc('נִצְּחוּ')
            },
            future: {
                '1s': nfc('אֲנַצֵּחַ'),
                '2ms': nfc('תְּנַצֵּחַ'),
                '2fs': nfc('תְּנַצְּחִי'),
                '3ms': nfc('יְנַצֵּחַ'),
                '3fs': nfc('תְּנַצֵּחַ'),
                '1p': nfc('נְנַצֵּחַ'),
                '2mp': nfc('תְּנַצְּחוּ'),
                '2fp': nfc('תְּנַצַּחְנָה'),
                '3p': nfc('יְנַצְּחוּ')
            }
        };
    }
    return {
        present: {
            ms: nfc('בּוֹחֵר'),
            fs: nfc('בּוֹחֶרֶת'),
            mp: nfc('בּוֹחֲרִים'),
            fp: nfc('בּוֹחֲרוֹת')
        },
        past: {
            '1s': nfc('בָּחַרְתִּי'),
            '2ms': nfc('בָּחַרְתָּ'),
            '2fs': nfc('בָּחַרְתְּ'),
            '3ms': nfc('בָּחַר'),
            '3fs': nfc('בָּחֲרָה'),
            '1p': nfc('בָּחַרְנוּ'),
            '2mp': nfc('בְּחַרְתֶּם'),
            '2fp': nfc('בְּחַרְתֶּן'),
            '3p': nfc('בָּחֲרוּ')
        },
        future: {
            '1s': nfc('אֶבְחַר'),
            '2ms': nfc('תִּבְחַר'),
            '2fs': nfc('תִּבְחֲרִי'),
            '3ms': nfc('יִבְחַר'),
            '3fs': nfc('תִּבְחַר'),
            '1p': nfc('נִבְחַר'),
            '2mp': nfc('תִּבְחֲרוּ'),
            '2fp': nfc('תִּבְחַרְנָה'),
            '3p': nfc('יִבְחֲרוּ')
        }
    };
}

const GLOSS = {
    decide: {
        present: {
            ms: 'decides (m. sg.)', fs: 'decides (f. sg.)',
            mp: 'decide (m. pl.)', fp: 'decide (f. pl.)'
        },
        past: {
            '1s': 'I decided', '2ms': 'you decided (m. sg.)', '2fs': 'you decided (f. sg.)',
            '3ms': 'he decided', '3fs': 'she decided', '1p': 'we decided',
            '2mp': 'you decided (m. pl.)', '2fp': 'you decided (f. pl.)', '3p': 'they decided'
        },
        future: {
            '1s': 'I will decide', '2ms': 'you will decide (m. sg.)', '2fs': 'you will decide (f. sg.)',
            '3ms': 'he will decide', '3fs': 'she will decide', '1p': 'we will decide',
            '2mp': 'you will decide (m. pl.)', '2fp': 'you will decide (f. pl.)', '3p': 'they will decide'
        }
    },
    choose: {
        present: {
            ms: 'chooses (m. sg.)', fs: 'chooses (f. sg.)',
            mp: 'choose (m. pl.)', fp: 'choose (f. pl.)'
        },
        past: {
            '1s': 'I chose', '2ms': 'you chose (m. sg.)', '2fs': 'you chose (f. sg.)',
            '3ms': 'he chose', '3fs': 'she chose', '1p': 'we chose',
            '2mp': 'you chose (m. pl.)', '2fp': 'you chose (f. pl.)', '3p': 'they chose'
        },
        future: {
            '1s': 'I will choose', '2ms': 'you will choose (m. sg.)', '2fs': 'you will choose (f. sg.)',
            '3ms': 'he will choose', '3fs': 'she will choose', '1p': 'we will choose',
            '2mp': 'you will choose (m. pl.)', '2fp': 'you will choose (f. pl.)', '3p': 'they will choose'
        }
    },
    believe: {
        present: {
            ms: 'believes (m. sg.)', fs: 'believes (f. sg.)',
            mp: 'believe (m. pl.)', fp: 'believe (f. pl.)'
        },
        past: {
            '1s': 'I believed', '2ms': 'you believed (m. sg.)', '2fs': 'you believed (f. sg.)',
            '3ms': 'he believed', '3fs': 'she believed', '1p': 'we believed',
            '2mp': 'you believed (m. pl.)', '2fp': 'you believed (f. pl.)', '3p': 'they believed'
        },
        future: {
            '1s': 'I will believe', '2ms': 'you will believe (m. sg.)', '2fs': 'you will believe (f. sg.)',
            '3ms': 'he will believe', '3fs': 'she will believe', '1p': 'we will believe',
            '2mp': 'you will believe (m. pl.)', '2fp': 'you will believe (f. pl.)', '3p': 'they will believe'
        }
    },
    win: {
        present: {
            ms: 'wins (m. sg.)', fs: 'wins (f. sg.)',
            mp: 'win (m. pl.)', fp: 'win (f. pl.)'
        },
        past: {
            '1s': 'I won', '2ms': 'you won (m. sg.)', '2fs': 'you won (f. sg.)',
            '3ms': 'he won', '3fs': 'she won', '1p': 'we won',
            '2mp': 'you won (m. pl.)', '2fp': 'you won (f. pl.)', '3p': 'they won'
        },
        future: {
            '1s': 'I will win', '2ms': 'you will win (m. sg.)', '2fs': 'you will win (f. sg.)',
            '3ms': 'he will win', '3fs': 'she will win', '1p': 'we will win',
            '2mp': 'you will win (m. pl.)', '2fp': 'you will win (f. pl.)', '3p': 'they will win'
        }
    }
};

/**
 * @param {object} spec
 * @returns {object}
 */
function lemmaEntry(spec) {
    return {
        key: spec.key,
        kind: 'lemma',
        gloss_en: spec.gloss_en,
        pos: 'verb',
        root: spec.root,
        binyan: spec.binyan,
        root_class: spec.root_class,
        accept: accepts(spec.key, spec.acceptExtra || [])
    };
}

/**
 * @param {object} spec
 * @param {string} tense
 * @param {string} cell
 * @param {string} pointed
 * @param {string} gloss
 * @returns {object}
 */
function formEntry(spec, tense, cell, pointed, gloss) {
    const key = formKey(pointed, cell, tense);
    const extras = [];
    if (tense === 'future' && cell === '3fs') {
        extras.push(stripPoints(pointed));
    }
    return {
        key,
        kind: 'form',
        lemma_key: spec.key,
        gloss_en: gloss,
        pos: 'verb',
        root: spec.root,
        binyan: spec.binyan,
        root_class: spec.root_class,
        tense,
        cell,
        accept: accepts(pointed, extras)
    };
}

/**
 * @param {object} spec
 * @param {Record<string, string>} pointed
 * @param {Record<string, string>} gloss
 * @param {string} tense
 * @returns {object[]}
 */
function cellsOf(spec, pointed, gloss, tense) {
    const order = tense === 'present' ? PRESENT_CELLS
        : tense === 'past' ? PAST_CELLS
            : FUTURE_CELLS;
    return order.map((cell) => formEntry(spec, tense, cell, pointed[cell], gloss[cell]));
}

const WORKSHOP_FORMS = WORKSHOP.flatMap((spec) => {
    const pointed = paradigmOf(spec);
    const gloss = GLOSS[spec.slug];
    return cellsOf(spec, pointed.present, gloss.present, 'present')
        .concat(cellsOf(spec, pointed.past, gloss.past, 'past'))
        .concat(cellsOf(spec, pointed.future, gloss.future, 'future'));
});

const ENTRIES = WORKSHOP.map(lemmaEntry).concat(WORKSHOP_FORMS);

const seenKeys = new Set();
ENTRIES.forEach((entry) => {
    if (seenKeys.has(entry.key)) {
        throw new Error('verb-tense workshop has a duplicate lexicon key: ' + entry.key);
    }
    seenKeys.add(entry.key);
    if (entry.key !== entry.key.normalize('NFC')) {
        throw new Error('verb-tense workshop key is not NFC');
    }
});

if (WORKSHOP_FORMS.length !== 88) {
    throw new Error('verb-tense workshop expected 88 forms, got ' + WORKSHOP_FORMS.length);
}

/**
 * @param {string} lemmaKey
 * @param {string} tense
 * @param {string} cell
 * @returns {object}
 */
function formAt(lemmaKey, tense, cell) {
    const found = WORKSHOP_FORMS.find((entry) => {
        return entry.lemma_key === lemmaKey && entry.tense === tense && entry.cell === cell;
    });
    if (!found) throw new Error('missing form ' + lemmaKey + ' ' + tense + ' ' + cell);
    return found;
}

/**
 * @param {string} id
 * @param {string} he
 * @param {string} [en]
 * @returns {object}
 */
function line(id, he, en) {
    const row = { id, he: nfc(he) };
    if (en) row.en = en;
    return row;
}

/**
 * @param {string} id
 * @param {string} he
 * @param {string} en
 * @param {string} [root]
 * @returns {object}
 */
function example(id, he, en, root) {
    const row = { id, he: nfc(he), en };
    if (root) row.root = root;
    return row;
}

/**
 * @param {object} spec
 * @param {string} tense
 * @param {string} mode
 * @param {boolean} withPractice
 * @returns {object}
 */
function tableBlock(spec, tense, mode, withPractice) {
    const order = tense === 'present' ? PRESENT_CELLS
        : tense === 'past' ? PAST_CELLS
            : FUTURE_CELLS;
    const block = {
        id: mode + '-' + tense + '-' + spec.slug,
        type: 'conjugation-table',
        lemmaKey: spec.key,
        tense,
        mode,
        cells: order.map((cell) => ({
            cell,
            formKey: formAt(spec.key, tense, cell).key
        }))
    };
    if (withPractice) {
        const ms = formAt(spec.key, 'present', 'ms').key;
        const fs = formAt(spec.key, 'present', 'fs').key;
        if (spec.slug === 'decide') {
            block.sentences = [
                line('s1', 'הַיּוֹם אֲנִי ' + ms + ' מָה לַעֲשׂוֹת.', 'Today I decide what to do.')
            ];
            block.questions = [
                line('q1', 'מָה אַתָּה ' + ms + '?', 'What do you decide?'),
                line('q2', 'מָה הִיא ' + fs + '?', 'What does she decide?')
            ];
        } else if (spec.slug === 'choose') {
            block.sentences = [
                line('s1', 'אֲנִי ' + ms + ' סֵפֶר חָדָשׁ.', 'I choose a new book.')
            ];
            block.questions = [
                line('q1', 'מָה אַתָּה ' + ms + '?', 'What do you choose?')
            ];
        } else if (spec.slug === 'believe') {
            block.sentences = [
                line('s1', 'הִיא ' + fs + ' בַּסִּפּוּר.', 'She believes the story.')
            ];
            block.questions = [
                line('q1', 'לָמָּה הִיא ' + fs + '?', 'Why does she believe?')
            ];
        } else {
            block.sentences = [
                line('s1', 'אֲנִי ' + ms + ' הַיּוֹם.', 'I win today.')
            ];
        }
    }
    return block;
}

/**
 * @returns {object}
 */
function workshopPlan() {
    const decideMs = formAt(DECIDE.key, 'present', 'ms').key;
    const chooseMs = formAt(CHOOSE.key, 'present', 'ms').key;
    const believeMs = formAt(BELIEVE.key, 'present', 'ms').key;
    const winMs = formAt(WIN.key, 'present', 'ms').key;
    const decide1s = formAt(DECIDE.key, 'past', '1s').key;
    const decide3ms = formAt(DECIDE.key, 'past', '3ms').key;

    const blocks = [
        {
            id: 'binyan-map',
            type: 'explainer',
            title: 'Draft: three patterns, one lesson',
            sections: [
                {
                    id: 'compare',
                    heading: 'Paal, piel, and hifil',
                    paragraphs: [
                        'Draft. Four verbs share this workshop: decide (hifil), choose (paal), believe (hifil), and win (piel). The root stays; the pattern around it changes. Present in piel and hifil often starts with mem. Past endings mark person. Future prefixes are alef, tav, yod, and nun.',
                        'Study the tables first. Then retrieve forms with the stem hidden. Finish with sentences you choose and write yourself. No form here is verified until a person ticks the checklist.'
                    ],
                    table: {
                        columns: ['Pattern', 'Root', 'Infinitive', 'Present cue', 'Past stem', 'Future prefix'],
                        rows: [
                            ['פָּעַל', CHOOSE.root, CHOOSE.key, 'no mem', 'stem vowels', 'א/ת/י/נ'],
                            ['פִּיעֵל', WIN.root, WIN.key, 'מ-', 'dagesh stem', 'א/ת/י/נ'],
                            ['הִפְעִיל', DECIDE.root, DECIDE.key, 'מ-', decide1s + ' / ' + decide3ms, 'א/ת/י/נ']
                        ]
                    },
                    examples: [
                        example('ex-decide', DECIDE.key, 'to decide (hifil)', DECIDE.root),
                        example('ex-choose', CHOOSE.key, 'to choose (paal)', CHOOSE.root),
                        example('ex-believe', BELIEVE.key, 'to believe (hifil)', BELIEVE.root),
                        example('ex-win', WIN.key, 'to win (piel, furtive patach)', WIN.root),
                        example('ex-decide-ms', decideMs, 'decides (m. sg.)', DECIDE.root),
                        example('ex-win-ms', winMs, 'wins (m. sg.)', WIN.root)
                    ],
                    callouts: [
                        {
                            id: 'green',
                            tone: 'green',
                            title: 'Green — regular hifil',
                            text: 'Decide follows a regular hifil shape. Start with its present and past before the future.'
                        },
                        {
                            id: 'yellow',
                            tone: 'yellow',
                            title: 'Yellow — guttural / furtive patach',
                            text: 'Believe and win keep guttural letters in the root. Win ends with a furtive patach on the chet.'
                        },
                        {
                            id: 'info',
                            tone: 'info',
                            title: 'Info — not verified',
                            text: 'Pointing is a draft. Tick a form on the checklist only after a person checks it.'
                        }
                    ]
                }
            ]
        },
        tableBlock(DECIDE, 'present', 'study', true),
        tableBlock(CHOOSE, 'past', 'study', false),
        tableBlock(BELIEVE, 'future', 'study', false),
        tableBlock(DECIDE, 'present', 'drill', false),
        tableBlock(CHOOSE, 'past', 'drill', false),
        tableBlock(BELIEVE, 'future', 'drill', false),
        tableBlock(WIN, 'present', 'drill', false),
        {
            id: 'ex-choose',
            type: 'exercise',
            kind: 'choose',
            heading: 'Pick the form that fits',
            items: [
                {
                    id: 'ch-1',
                    prompt: 'Which sentence means "I decided yesterday"?',
                    choices: [
                        nfc('אֶתְמוֹל הֶחְלַטְתִּי.'),
                        nfc('מָחָר אַחְלִיט.'),
                        nfc('הַיּוֹם אֲנִי מַחְלִיט.')
                    ],
                    answer: nfc('אֶתְמוֹל הֶחְלַטְתִּי.'),
                    why: 'Past first-person uses the תִּי ending on the hifil stem.'
                },
                {
                    id: 'ch-2',
                    prompt: 'Which sentence means "She will choose a book"?',
                    choices: [
                        nfc('הִיא תִּבְחַר סֵפֶר.'),
                        nfc('הִיא בָּחֲרָה סֵפֶר.'),
                        nfc('הִיא בּוֹחֶרֶת סֵפֶר.')
                    ],
                    answer: nfc('הִיא תִּבְחַר סֵפֶר.'),
                    why: 'Future third-person feminine shares the תִּבְחַר shape with second-person masculine; the pronoun marks the person.'
                },
                {
                    id: 'ch-3',
                    prompt: 'Which sentence means "We believe the story"?',
                    choices: [
                        nfc('אֲנַחְנוּ מַאֲמִינִים בַּסִּפּוּר.'),
                        nfc('אֲנַחְנוּ הֶאֱמַנּוּ בַּסִּפּוּר.'),
                        nfc('אֲנַחְנוּ נַאֲמִין בַּסִּפּוּר.')
                    ],
                    answer: nfc('אֲנַחְנוּ מַאֲמִינִים בַּסִּפּוּר.'),
                    why: 'Present masculine plural of hifil starts with mem and ends with ים.'
                }
            ]
        },
        {
            id: 'ex-translate',
            type: 'exercise',
            kind: 'translate',
            heading: 'Type the Hebrew sentence',
            items: [
                {
                    id: 'tr-1',
                    english: 'I decide today.',
                    answer: stripPoints(nfc('אני מחליט היום')),
                    accept: ['אני מחליטה היום', 'אֲנִי מַחְלִיט הַיּוֹם'].map((s) => stripPoints(nfc(s)))
                },
                {
                    id: 'tr-2',
                    english: 'They will win tomorrow.',
                    answer: stripPoints(nfc('הם ינצחו מחר')),
                    accept: ['הן ינצחו מחר', 'הם יְנַצְּחוּ מָחָר'].map((s) => stripPoints(nfc(s)))
                },
                {
                    id: 'tr-3',
                    english: 'You (m. sg.) chose well.',
                    answer: stripPoints(nfc('אתה בחרת היטב')),
                    accept: ['אתה בָּחַרְתָּ הֵיטֵב'].map((s) => stripPoints(nfc(s)))
                }
            ]
        },
        {
            id: 'write',
            type: 'assignment',
            prompt: 'Write at least six Hebrew sentences. Use decide, choose, believe, and win in present, past, and future. Then write two questions for a partner.',
            minSentences: 6,
            useWords: [DECIDE.key, CHOOSE.key, BELIEVE.key, WIN.key],
            homework: [
                'Write two present sentences with two of the four verbs.',
                'Write two past sentences with two of the four verbs.',
                'Write two future sentences with two of the four verbs.',
                'Write two questions that use at least two of the four verbs.'
            ]
        },
        {
            id: 'closing',
            type: 'personal-question',
            prompts: [
                line(
                    'p1',
                    'מָה אַתָּה ' + decideMs + ', וּמָה אַתָּה ' + chooseMs + '?',
                    'What do you decide, and what do you choose?'
                ),
                line(
                    'p2',
                    'בַּמֶּה אַתָּה ' + believeMs + ', וּמָתַי אַתָּה ' + winMs + '?',
                    'What do you believe, and when do you win?'
                )
            ],
            answer: 'either'
        }
    ];

    return {
        schemaVersion: '1.2.0',
        id: PLAN_ID,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Verb tense workshop',
        subtitle: 'Draft. Nothing in this workshop is verified.',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'multi-tense-verbs',
            title: 'Decide, choose, believe, win',
            theme: 'multi-binyan present, past, and future',
            lessons: [{
                id: LESSON_ID,
                title: 'Decide, choose, believe, win',
                subtitle: 'Draft. Pointing is not verified.',
                level: 'basic',
                visibility: 'hidden',
                recipe: 'verb-workshop',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks
            }]
        }]
    };
}

const PLANS = [workshopPlan()];

function defaultChecklistPath() {
    return path.join(
        rootDir(),
        'sdd',
        'initiatives',
        'INIT-013-responsive-desktop-mobile-redesign',
        'specs',
        'completion-summaries',
        'SPEC-007-forms-checklist.md'
    );
}

/**
 * @param {Set<string>} [ticked]
 * @returns {string}
 */
function renderChecklist(ticked) {
    const ticks = ticked || new Set();
    const lines = [
        '# SPEC-007 forms checklist',
        '',
        'Draft. These pointed forms are not verified.',
        'A person ticks a line only after checking that form.',
        'Write `[x]` in the box. The next run of `scripts/seed-verb-tense-workshop.js` marks those forms verified and leaves every other form unverified.',
        'Untick a line and run again to clear it.',
        'Future `2ms` and `3fs` share the same pointed letters; the `3fs` lexicon key ends with `-3fs` so both rows stay unique.',
        'This file is not a class transcript. Lesson sentences were written fresh for this draft.',
        'Provenance: INIT-013/SPEC-007',
        ''
    ];
    WORKSHOP.forEach((spec) => {
        lines.push('## ' + spec.key + ' — ' + spec.gloss_en + ' — ' + spec.binyan + ' — ' + spec.root + ' — ' + spec.root_class);
        lines.push('');
        ['present', 'past', 'future'].forEach((tense) => {
            lines.push('### ' + tense);
            lines.push('');
            WORKSHOP_FORMS.filter((entry) => entry.lemma_key === spec.key && entry.tense === tense).forEach((entry) => {
                const box = ticks.has(entry.key) ? 'x' : ' ';
                lines.push('- [' + box + '] | ' + entry.lemma_key + ' | ' + entry.tense + ' | ' + entry.cell + ' | ' + entry.key);
            });
            lines.push('');
        });
    });
    return lines.join('\n');
}

/**
 * @param {string} text
 * @returns {Set<string>}
 */
function parseTicks(text) {
    const byKey = new Map(WORKSHOP_FORMS.map((entry) => [entry.key, entry]));
    const ticked = new Set();
    String(text || '').split('\n').forEach((lineText) => {
        const match = lineText.match(/^- \[([ xX])\] \| (.+) \| (present|past|future) \| ([a-z0-9]+) \| (.+)$/);
        if (!match || match[1] === ' ') return;
        const key = match[5].normalize('NFC').trim();
        const entry = byKey.get(key);
        if (!entry) return;
        if (entry.lemma_key !== match[2].normalize('NFC').trim()) return;
        if (entry.tense !== match[3] || entry.cell !== match[4]) return;
        ticked.add(key);
    });
    return ticked;
}

/**
 * @param {object} existing
 * @param {object} entry
 * @returns {boolean}
 */
function samePayload(existing, entry) {
    const accept = (existing.accept || []).slice().sort();
    const wanted = entry.accept.slice().sort();
    return existing.kind === entry.kind
        && (existing.lemma_key || null) === (entry.lemma_key || null)
        && (existing.gloss_en || null) === (entry.gloss_en || null)
        && (existing.pos || null) === (entry.pos || null)
        && (existing.root || null) === (entry.root || null)
        && (existing.binyan || null) === (entry.binyan || null)
        && (existing.root_class || null) === (entry.root_class || null)
        && (existing.tense || null) === (entry.tense || null)
        && (existing.cell || null) === (entry.cell || null)
        && JSON.stringify(accept) === JSON.stringify(wanted);
}

/**
 * @param {object} service
 * @param {object[]} entries
 * @returns {{ ok: true }|{ ok: false, error: string, errors?: object[], conflicts?: object[] }}
 */
function writeEntries(service, entries) {
    const groups = new Map();
    for (const entry of entries) {
        const got = service.get(entry.key);
        const existing = got ? got.entry : null;
        if (existing && samePayload(existing, entry)) continue;
        const rev = existing ? existing.rev : 0;
        if (!groups.has(rev)) groups.set(rev, []);
        groups.get(rev).push(entry);
    }
    for (const [rev, batch] of groups) {
        const written = service.upsert(batch, { actor: ACTOR, expectedRev: rev });
        if (!written.ok) {
            return {
                ok: false,
                error: written.error || 'lexicon',
                errors: written.errors,
                conflicts: written.conflicts
            };
        }
    }
    return { ok: true };
}

/**
 * @param {object} service
 * @param {Set<string>} ticked
 * @returns {{ ok: true, verified: number }|{ ok: false, error: string }}
 */
function applyTicks(service, ticked) {
    const on = [];
    const off = [];
    WORKSHOP_FORMS.forEach((entry) => {
        const got = service.get(entry.key);
        const verified = got && got.entry ? got.entry.verified : 0;
        if (ticked.has(entry.key)) {
            if (verified !== 1) on.push(entry.key);
        } else if (verified === 1) {
            off.push(entry.key);
        }
    });
    if (on.length) {
        const up = service.setVerified(on, true, { actor: ACTOR });
        if (!up.ok) return { ok: false, error: up.error || 'set_verified' };
    }
    if (off.length) {
        const down = service.setVerified(off, false, { actor: ACTOR });
        if (!down.ok) return { ok: false, error: down.error || 'set_verified' };
    }
    return { ok: true, verified: ticked.size };
}

/**
 * @param {object} plan
 * @returns {string}
 */
function fingerprint(plan) {
    const copy = JSON.parse(JSON.stringify(plan));
    copy.revision = 0;
    return JSON.stringify(copy);
}

/**
 * @param {object} store
 * @param {object} plan
 */
function ensureRow(store, plan) {
    if (store.loadWorkingCopy(plan.id)) return;
    store.saveWorkingCopy({
        id: plan.id,
        document: JSON.stringify({
            schemaVersion: '1.2.0',
            id: plan.id,
            revision: 1,
            status: 'draft',
            audience: 'shared',
            assigneeId: null,
            title: 'draft',
            language: { target: 'he', gloss: 'en' },
            units: [{
                id: 'placeholder',
                title: 'draft',
                theme: 'draft',
                lessons: [{
                    id: 'placeholder',
                    title: 'draft',
                    level: 'basic',
                    visibility: 'hidden',
                    buildsOn: [],
                    newWords: [],
                    recycles: [],
                    blocks: [{
                        id: 'placeholder',
                        type: 'explainer',
                        title: 'draft',
                        sections: [{ id: 'placeholder', heading: 'draft' }]
                    }]
                }]
            }]
        }),
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null
    });
}

/**
 * @returns {object}
 */
function seedContext() {
    const { makeContext } = require('../server/plans/context');
    return makeContext({ channel: 'system', name: 'seed-verb-tense-workshop', tool: 'replace_part' });
}

/**
 * @param {object} store
 * @param {object} ops
 * @param {Function} validatePlan
 * @param {object} lexiconRepo
 * @param {object} plan
 * @returns {{ ok: true, revision: number }|{ ok: false, error: string, blocks?: object[] }}
 */
function writePlan(store, ops, validatePlan, lexiconRepo, plan) {
    const desired = JSON.parse(JSON.stringify(plan));
    desired.revision = 1;
    const validation = validatePlan(desired, { lexicon: lexiconRepo });
    if (validation.blocks.length) {
        return { ok: false, error: 'validation', blocks: validation.blocks };
    }
    const existing = store.loadWorkingCopy(plan.id);
    if (existing) {
        const stored = JSON.parse(existing.document);
        if (fingerprint(stored) === fingerprint(desired)) {
            return { ok: true, revision: existing.revision };
        }
        const replaced = ops.replacePart(store, plan.id, {
            path: '',
            value: desired,
            expectedRevision: existing.revision
        }, seedContext());
        if (replaced.error) return { ok: false, error: replaced.error, blocks: replaced.blocks };
        return { ok: true, revision: replaced.revision };
    }
    ensureRow(store, plan);
    const replaced = ops.replacePart(store, plan.id, {
        path: '',
        value: desired,
        expectedRevision: 1
    }, seedContext());
    if (replaced.error) return { ok: false, error: replaced.error, blocks: replaced.blocks };
    return { ok: true, revision: replaced.revision };
}

/**
 * @param {{ checklistPath?: string }} [options]
 * @returns {object}
 */
function run(options) {
    const service = require('../server/lexicon/service');
    const store = require('../server/plans/store');
    const ops = require('../server/plans/ops');
    const { validatePlan } = require('../server/plans/validate');
    const lexiconRepo = require('../server/db/repos/lexicon');
    const opts = options || {};
    const checklistPath = opts.checklistPath
        ? path.resolve(opts.checklistPath)
        : defaultChecklistPath();

    const foreign = [];
    ENTRIES.forEach((entry) => {
        const got = service.get(entry.key);
        if (got && got.entry && got.entry.source !== 'admin') foreign.push(entry.key);
    });
    if (foreign.length) {
        return { ok: false, error: 'foreign_source', keys: foreign };
    }

    const lemmas = ENTRIES.filter((entry) => entry.kind === 'lemma');
    const forms = ENTRIES.filter((entry) => entry.kind === 'form');
    const wroteLemmas = writeEntries(service, lemmas);
    if (!wroteLemmas.ok) return wroteLemmas;
    const wroteForms = writeEntries(service, forms);
    if (!wroteForms.ok) return wroteForms;

    let ticks = new Set();
    if (fs.existsSync(checklistPath)) ticks = parseTicks(fs.readFileSync(checklistPath, 'utf8'));
    const verified = applyTicks(service, ticks);
    if (!verified.ok) return verified;

    const planResults = [];
    for (const plan of PLANS) {
        const wrote = writePlan(store, ops, validatePlan, lexiconRepo, plan);
        if (!wrote.ok) return wrote;
        planResults.push({ id: plan.id, revision: wrote.revision });
    }

    const text = renderChecklist(ticks);
    fs.mkdirSync(path.dirname(checklistPath), { recursive: true });
    const previous = fs.existsSync(checklistPath) ? fs.readFileSync(checklistPath, 'utf8') : null;
    if (previous !== text) fs.writeFileSync(checklistPath, text, 'utf8');

    return {
        ok: true,
        forms: WORKSHOP_FORMS.length,
        lemmas: lemmas.length,
        verified: verified.verified,
        plans: planResults,
        checklistPath
    };
}

/**
 * Open the database from DATA_DIR, migrate, and write the drafts.
 * Does not publish and does not generate audio.
 */
function main() {
    const db = require('../server/db');
    db.open();
    db.migrate();
    const result = run();
    if (!result.ok) {
        console.error('verb-tense workshop refused: ' + result.error);
        if (result.blocks) console.error(JSON.stringify(result.blocks, null, 2));
        process.exitCode = 1;
        return;
    }
    console.log('verb-tense workshop ok forms=' + result.forms + ' verified=' + result.verified);
}

module.exports = {
    ACTOR,
    PLAN_ID,
    LESSON_ID,
    ENTRIES,
    WORKSHOP,
    WORKSHOP_FORMS,
    PLANS,
    renderChecklist,
    parseTicks,
    run,
    defaultChecklistPath,
    formAt,
    formKey
};

if (require.main === module) main();
