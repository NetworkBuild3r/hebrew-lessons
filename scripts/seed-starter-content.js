/**
 * Draft starter content: a "Decoding Hebrew Verbs" guide and one verb workshop.
 * Lexicon rows are written with verified=0. A second run sets verified only on
 * forms whose checklist line is ticked. Pointing is a draft until a person
 * checks it. Prose is original; it is not copied from a class sheet.
 * Idempotent. Refuses when a target key already exists with source other than admin.
 * Writes go through lexicon.upsert, lexicon.setVerified, and plan ops.
 * Provenance: INIT-006/SPEC-018
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { stripPoints } = require('../server/lexicon/validate');
const { rootDir } = require('../server/paths');

const ACTOR = 'admin:starter-content';
const PRESENT_CELLS = ['ms', 'fs', 'mp', 'fp'];
const PAST_CELLS = ['1s', '2ms', '2fs', '3ms', '3fs', '1p', '2mp', '2fp', '3p'];
const PLAN_IDS = { guides: 'guides', verbs: 'verbs' };
const LESSON_IDS = {
    guide: 'decoding-hebrew-verbs',
    workshop: 'verbs-think-feel-ask-answer'
};

const P = {
    sheva: '\u05B0',
    hatafPatah: '\u05B2',
    hiriq: '\u05B4',
    tsere: '\u05B5',
    segol: '\u05B6',
    patah: '\u05B7',
    qamats: '\u05B8',
    holam: '\u05B9',
    dagesh: '\u05BC',
    shin: '\u05C1'
};

/**
 * @param {string} base
 * @param {...string} marks
 * @returns {string}
 */
function L(base, ...marks) {
    return base + marks.join('');
}

/**
 * @param {...string} parts
 * @returns {string}
 */
function W(...parts) {
    return parts.join('').normalize('NFC');
}

/**
 * @param {string} pointed
 * @param {string[]} [extras] Unpointed plene spellings.
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
 * @param {object} spec
 * @param {string} tense
 * @param {string} cell
 * @param {string} key
 * @param {string} gloss
 * @returns {object}
 */
function formEntry(spec, tense, cell, key, gloss) {
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
        accept: accepts(key)
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
    const order = tense === 'present' ? PRESENT_CELLS : PAST_CELLS;
    return order.map((cell) => formEntry(spec, tense, cell, pointed[cell], gloss[cell]));
}

const THINK = {
    key: W(L('\u05DC', P.patah), L('\u05D7', P.sheva), L('\u05E9', P.holam, P.shin), L('\u05D1')),
    gloss_en: 'to think',
    pos: 'verb',
    root: '\u05D7.\u05E9.\u05D1',
    binyan: 'paal',
    root_class: 'regular',
    acceptExtra: ['\u05DC\u05D7\u05E9\u05D5\u05D1'],
    slug: 'think'
};

const FEEL = {
    key: W(L('\u05DC', P.sheva), L('\u05D4', P.patah), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.shin)),
    gloss_en: 'to feel',
    pos: 'verb',
    root: '\u05E8.\u05D2.\u05E9',
    binyan: 'hifil',
    root_class: 'regular',
    acceptExtra: [],
    slug: 'feel'
};

const ASK = {
    key: W(L('\u05DC', P.hiriq), L('\u05E9', P.sheva, P.shin), L('\u05D0', P.holam), L('\u05DC')),
    gloss_en: 'to ask',
    pos: 'verb',
    root: '\u05E9.\u05D0.\u05DC',
    binyan: 'paal',
    root_class: 'regular',
    acceptExtra: ['\u05DC\u05E9\u05D0\u05D5\u05DC'],
    slug: 'ask'
};

const ANSWER = {
    key: W(L('\u05DC', P.patah), L('\u05E2', P.hatafPatah), L('\u05E0'), L('\u05D5', P.holam), L('\u05EA')),
    gloss_en: 'to answer',
    pos: 'verb',
    root: '\u05E2.\u05E0.\u05D4',
    binyan: 'paal',
    root_class: 'lamed-he',
    acceptExtra: [],
    slug: 'answer'
};

const GUIDE_LEMMAS = [
    {
        key: W(L('\u05E8', P.qamats), L('\u05D0', P.qamats), L('\u05D4')),
        gloss_en: 'he saw (citation draft)',
        root: '\u05E8.\u05D0.\u05D4',
        binyan: 'paal',
        root_class: 'lamed-he',
        en: 'he saw'
    },
    {
        key: W(L('\u05E7', P.qamats), L('\u05E0', P.qamats), L('\u05D4')),
        gloss_en: 'he bought (citation draft)',
        root: '\u05E7.\u05E0.\u05D4',
        binyan: 'paal',
        root_class: 'lamed-he',
        en: 'he bought'
    },
    {
        key: W(L('\u05D4', P.qamats), L('\u05DC', P.patah), L('\u05DA', P.sheva)),
        gloss_en: 'he walked (citation draft)',
        root: '\u05D4.\u05DC.\u05DB',
        binyan: 'paal',
        root_class: 'irregular',
        en: 'he walked'
    },
    {
        key: W(L('\u05D9', P.qamats), L('\u05E8', P.patah), L('\u05D3')),
        gloss_en: 'he went down (citation draft)',
        root: '\u05D9.\u05E8.\u05D3',
        binyan: 'paal',
        root_class: 'peh-yod',
        en: 'he went down'
    },
    {
        key: W(L('\u05D9', P.qamats), L('\u05E9', P.patah, P.shin), L('\u05D1')),
        gloss_en: 'he sat (citation draft)',
        root: '\u05D9.\u05E9.\u05D1',
        binyan: 'paal',
        root_class: 'peh-yod',
        en: 'he sat'
    }
];

/**
 * Pointed present and past cells. Draft Modern Hebrew, not a checked paradigm.
 * @param {object} spec
 * @returns {{ present: Record<string, string>, past: Record<string, string> }}
 */
function paradigmOf(spec) {
    if (spec.slug === 'think') {
        return {
            present: {
                ms: W(L('\u05D7'), L('\u05D5', P.holam), L('\u05E9', P.tsere, P.shin), L('\u05D1')),
                fs: W(L('\u05D7'), L('\u05D5', P.holam), L('\u05E9', P.segol, P.shin), L('\u05D1', P.segol), L('\u05EA')),
                mp: W(L('\u05D7'), L('\u05D5', P.holam), L('\u05E9', P.sheva, P.shin), L('\u05D1', P.hiriq), L('\u05D9'), L('\u05DD')),
                fp: W(L('\u05D7'), L('\u05D5', P.holam), L('\u05E9', P.sheva, P.shin), L('\u05D1'), L('\u05D5', P.holam), L('\u05EA'))
            },
            past: {
                '1s': W(L('\u05D7', P.qamats), L('\u05E9', P.patah, P.shin), L('\u05D1', P.sheva), L('\u05EA', P.hiriq, P.dagesh), L('\u05D9')),
                '2ms': W(L('\u05D7', P.qamats), L('\u05E9', P.patah, P.shin), L('\u05D1', P.sheva), L('\u05EA', P.qamats, P.dagesh)),
                '2fs': W(L('\u05D7', P.qamats), L('\u05E9', P.patah, P.shin), L('\u05D1', P.sheva), L('\u05EA', P.sheva, P.dagesh)),
                '3ms': W(L('\u05D7', P.qamats), L('\u05E9', P.patah, P.shin), L('\u05D1')),
                '3fs': W(L('\u05D7', P.qamats), L('\u05E9', P.sheva, P.shin), L('\u05D1', P.qamats), L('\u05D4')),
                '1p': W(L('\u05D7', P.qamats), L('\u05E9', P.patah, P.shin), L('\u05D1', P.sheva), L('\u05E0'), L('\u05D5', P.dagesh)),
                '2mp': W(L('\u05D7', P.hatafPatah), L('\u05E9', P.patah, P.shin), L('\u05D1', P.sheva), L('\u05EA', P.segol, P.dagesh), L('\u05DD')),
                '2fp': W(L('\u05D7', P.hatafPatah), L('\u05E9', P.patah, P.shin), L('\u05D1', P.sheva), L('\u05EA', P.segol, P.dagesh), L('\u05DF')),
                '3p': W(L('\u05D7', P.qamats), L('\u05E9', P.sheva, P.shin), L('\u05D1'), L('\u05D5', P.dagesh))
            }
        };
    }
    if (spec.slug === 'feel') {
        return {
            present: {
                ms: W(L('\u05DE', P.patah), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.shin)),
                fs: W(L('\u05DE', P.patah), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.qamats, P.shin), L('\u05D4')),
                mp: W(L('\u05DE', P.patah), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.hiriq, P.shin), L('\u05D9'), L('\u05DD')),
                fp: W(L('\u05DE', P.patah), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.shin), L('\u05D5', P.holam), L('\u05EA'))
            },
            past: {
                '1s': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.patah), L('\u05E9', P.sheva, P.shin), L('\u05EA', P.hiriq, P.dagesh), L('\u05D9')),
                '2ms': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.patah), L('\u05E9', P.sheva, P.shin), L('\u05EA', P.qamats, P.dagesh)),
                '2fs': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.patah), L('\u05E9', P.sheva, P.shin), L('\u05EA', P.sheva, P.dagesh)),
                '3ms': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.shin)),
                '3fs': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.qamats, P.shin), L('\u05D4')),
                '1p': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.patah), L('\u05E9', P.sheva, P.shin), L('\u05E0'), L('\u05D5', P.dagesh)),
                '2mp': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.patah), L('\u05E9', P.sheva, P.shin), L('\u05EA', P.segol, P.dagesh), L('\u05DD')),
                '2fp': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.patah), L('\u05E9', P.sheva, P.shin), L('\u05EA', P.segol, P.dagesh), L('\u05DF')),
                '3p': W(L('\u05D4', P.hiriq), L('\u05E8', P.sheva), L('\u05D2', P.hiriq), L('\u05D9'), L('\u05E9', P.shin), L('\u05D5', P.dagesh))
            }
        };
    }
    if (spec.slug === 'ask') {
        return {
            present: {
                ms: W(L('\u05E9', P.shin), L('\u05D5', P.holam), L('\u05D0', P.tsere), L('\u05DC')),
                fs: W(L('\u05E9', P.shin), L('\u05D5', P.holam), L('\u05D0', P.segol), L('\u05DC', P.segol), L('\u05EA')),
                mp: W(L('\u05E9', P.shin), L('\u05D5', P.holam), L('\u05D0', P.hatafPatah), L('\u05DC', P.hiriq), L('\u05D9'), L('\u05DD')),
                fp: W(L('\u05E9', P.shin), L('\u05D5', P.holam), L('\u05D0', P.hatafPatah), L('\u05DC'), L('\u05D5', P.holam), L('\u05EA'))
            },
            past: {
                '1s': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.patah), L('\u05DC', P.sheva), L('\u05EA', P.hiriq, P.dagesh), L('\u05D9')),
                '2ms': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.patah), L('\u05DC', P.sheva), L('\u05EA', P.qamats, P.dagesh)),
                '2fs': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.patah), L('\u05DC', P.sheva), L('\u05EA', P.sheva, P.dagesh)),
                '3ms': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.patah), L('\u05DC')),
                '3fs': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.hatafPatah), L('\u05DC', P.qamats), L('\u05D4')),
                '1p': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.patah), L('\u05DC', P.sheva), L('\u05E0'), L('\u05D5', P.dagesh)),
                '2mp': W(L('\u05E9', P.sheva, P.shin), L('\u05D0', P.patah), L('\u05DC', P.sheva), L('\u05EA', P.segol, P.dagesh), L('\u05DD')),
                '2fp': W(L('\u05E9', P.sheva, P.shin), L('\u05D0', P.patah), L('\u05DC', P.sheva), L('\u05EA', P.segol, P.dagesh), L('\u05DF')),
                '3p': W(L('\u05E9', P.qamats, P.shin), L('\u05D0', P.hatafPatah), L('\u05DC'), L('\u05D5', P.dagesh))
            }
        };
    }
    return {
        present: {
            ms: W(L('\u05E2'), L('\u05D5', P.holam), L('\u05E0', P.tsere), L('\u05D4')),
            fs: W(L('\u05E2'), L('\u05D5', P.holam), L('\u05E0', P.qamats), L('\u05D4')),
            mp: W(L('\u05E2'), L('\u05D5', P.holam), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05DD')),
            fp: W(L('\u05E2'), L('\u05D5', P.holam), L('\u05E0'), L('\u05D5', P.holam), L('\u05EA'))
        },
        past: {
            '1s': W(L('\u05E2', P.qamats), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05EA', P.hiriq), L('\u05D9')),
            '2ms': W(L('\u05E2', P.qamats), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05EA', P.qamats)),
            '2fs': W(L('\u05E2', P.qamats), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05EA')),
            '3ms': W(L('\u05E2', P.qamats), L('\u05E0', P.qamats), L('\u05D4')),
            '3fs': W(L('\u05E2', P.qamats), L('\u05E0', P.sheva), L('\u05EA', P.qamats), L('\u05D4')),
            '1p': W(L('\u05E2', P.qamats), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05E0'), L('\u05D5', P.dagesh)),
            '2mp': W(L('\u05E2', P.hatafPatah), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05EA', P.segol, P.dagesh), L('\u05DD')),
            '2fp': W(L('\u05E2', P.hatafPatah), L('\u05E0', P.hiriq), L('\u05D9'), L('\u05EA', P.segol, P.dagesh), L('\u05DF')),
            '3p': W(L('\u05E2', P.qamats), L('\u05E0'), L('\u05D5', P.dagesh))
        }
    };
}

const GLOSS = {
    think: {
        present: { ms: 'thinks (m. sg.)', fs: 'thinks (f. sg.)', mp: 'think (m. pl.)', fp: 'think (f. pl.)' },
        past: {
            '1s': 'I thought', '2ms': 'you thought (m. sg.)', '2fs': 'you thought (f. sg.)',
            '3ms': 'he thought', '3fs': 'she thought', '1p': 'we thought',
            '2mp': 'you thought (m. pl.)', '2fp': 'you thought (f. pl.)', '3p': 'they thought'
        }
    },
    feel: {
        present: { ms: 'feels (m. sg.)', fs: 'feels (f. sg.)', mp: 'feel (m. pl.)', fp: 'feel (f. pl.)' },
        past: {
            '1s': 'I felt', '2ms': 'you felt (m. sg.)', '2fs': 'you felt (f. sg.)',
            '3ms': 'he felt', '3fs': 'she felt', '1p': 'we felt',
            '2mp': 'you felt (m. pl.)', '2fp': 'you felt (f. pl.)', '3p': 'they felt'
        }
    },
    ask: {
        present: { ms: 'asks (m. sg.)', fs: 'asks (f. sg.)', mp: 'ask (m. pl.)', fp: 'ask (f. pl.)' },
        past: {
            '1s': 'I asked', '2ms': 'you asked (m. sg.)', '2fs': 'you asked (f. sg.)',
            '3ms': 'he asked', '3fs': 'she asked', '1p': 'we asked',
            '2mp': 'you asked (m. pl.)', '2fp': 'you asked (f. pl.)', '3p': 'they asked'
        }
    },
    answer: {
        present: { ms: 'answers (m. sg.)', fs: 'answers (f. sg.)', mp: 'answer (m. pl.)', fp: 'answer (f. pl.)' },
        past: {
            '1s': 'I answered', '2ms': 'you answered (m. sg.)', '2fs': 'you answered (f. sg.)',
            '3ms': 'he answered', '3fs': 'she answered', '1p': 'we answered',
            '2mp': 'you answered (m. pl.)', '2fp': 'you answered (f. pl.)', '3p': 'they answered'
        }
    }
};

const WORKSHOP = [THINK, FEEL, ASK, ANSWER];

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

const WORKSHOP_FORMS = WORKSHOP.flatMap((spec) => {
    const pointed = paradigmOf(spec);
    return cellsOf(spec, pointed.present, GLOSS[spec.slug].present, 'present')
        .concat(cellsOf(spec, pointed.past, GLOSS[spec.slug].past, 'past'));
});

const ENTRIES = WORKSHOP.map(lemmaEntry)
    .concat(GUIDE_LEMMAS.map(lemmaEntry))
    .concat(WORKSHOP_FORMS);

const seenKeys = new Set();
ENTRIES.forEach((entry) => {
    if (seenKeys.has(entry.key)) {
        throw new Error('starter content has a duplicate lexicon key');
    }
    seenKeys.add(entry.key);
    if (entry.key !== entry.key.normalize('NFC')) {
        throw new Error('starter content key is not NFC');
    }
});

if (WORKSHOP_FORMS.length !== 52) {
    throw new Error('starter content expected 52 forms');
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
    if (!found) throw new Error('missing form');
    return found;
}

/**
 * @param {string} id
 * @param {string} he
 * @param {string} [en]
 * @returns {object}
 */
function line(id, he, en) {
    const row = { id, he };
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
    const row = { id, he, en };
    if (root) row.root = root;
    return row;
}

const HU = W(L('\u05D4'), L('\u05D5', P.dagesh), L('\u05D0'));
const HI = W(L('\u05D4', P.hiriq), L('\u05D9'), L('\u05D0'));
const ANI = W(L('\u05D0', P.hatafPatah), L('\u05E0', P.hiriq), L('\u05D9'));
const ATA = W(L('\u05D0', P.patah), L('\u05EA', P.qamats, P.dagesh), L('\u05D4'));
const AT = W(L('\u05D0', P.patah), L('\u05EA', P.sheva, P.dagesh));
const AL = W(L('\u05E2', P.patah), L('\u05DC'));
const MA = W(L('\u05DE', P.qamats), L('\u05D4'));
const SEFER = W(L('\u05E1', P.tsere), L('\u05E4', P.segol), L('\u05E8'));
const TOV = W(L('\u05D8'), L('\u05D5', P.holam), L('\u05D1'));
const HAYOM = W(L('\u05D4', P.patah), L('\u05D9', P.dagesh), L('\u05D5', P.holam), L('\u05DD'));
const SHEELA = W(L('\u05E9', P.sheva, P.shin), L('\u05D0', P.tsere), L('\u05DC', P.qamats), L('\u05D4'));
const IVRIT = W(L('\u05D1', P.sheva), L('\u05E2', P.hiriq), L('\u05D1', P.sheva), L('\u05E8', P.hiriq), L('\u05D9'), L('\u05EA'));

/**
 * @param {object} spec
 * @returns {{ he: string, en: string, questions: { he: string, en: string }[] }}
 */
function practiceOf(spec) {
    const ms = formAt(spec.key, 'present', 'ms').key;
    const fs = formAt(spec.key, 'present', 'fs').key;
    if (spec.slug === 'think') {
        return {
            he: HU + ' ' + ms + ' ' + AL + ' ' + SEFER + '.',
            en: 'He is thinking about a book.',
            questions: [
                { he: MA + ' ' + HU + ' ' + ms + '?', en: 'What is he thinking?' },
                { he: MA + ' ' + HI + ' ' + fs + '?', en: 'What is she thinking?' }
            ]
        };
    }
    if (spec.slug === 'feel') {
        return {
            he: ANI + ' ' + ms + ' ' + TOV + ' ' + HAYOM + '.',
            en: 'I feel good today.',
            questions: [
                { he: MA + ' ' + ATA + ' ' + ms + '?', en: 'What do you feel?' },
                { he: MA + ' ' + AT + ' ' + fs + '?', en: 'What do you feel?' }
            ]
        };
    }
    if (spec.slug === 'ask') {
        return {
            he: HI + ' ' + fs + ' ' + SHEELA + '.',
            en: 'She asks a question.',
            questions: [
                { he: MA + ' ' + HI + ' ' + fs + '?', en: 'What does she ask?' },
                { he: MA + ' ' + HU + ' ' + ms + '?', en: 'What does he ask?' }
            ]
        };
    }
    return {
        he: ANI + ' ' + ms + ' ' + IVRIT + '.',
        en: 'I answer in Hebrew.',
        questions: [
            { he: MA + ' ' + ATA + ' ' + ms + '?', en: 'What do you answer?' },
            { he: MA + ' ' + HI + ' ' + fs + '?', en: 'What does she answer?' }
        ]
    };
}

/**
 * @param {object} spec
 * @param {string} tense
 * @param {string} mode
 * @param {boolean} withPractice
 * @returns {object}
 */
function tableBlock(spec, tense, mode, withPractice) {
    const order = tense === 'present' ? PRESENT_CELLS : PAST_CELLS;
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
        const practice = practiceOf(spec);
        block.sentences = [line('s1', practice.he, practice.en)];
        block.questions = practice.questions.map((item, index) => {
            return line('q' + (index + 1), item.he, item.en);
        });
    }
    return block;
}

/**
 * @returns {object}
 */
function guidePlan() {
    const saw = GUIDE_LEMMAS[0];
    const bought = GUIDE_LEMMAS[1];
    const walked = GUIDE_LEMMAS[2];
    const down = GUIDE_LEMMAS[3];
    const sat = GUIDE_LEMMAS[4];
    const thinkMs = formAt(THINK.key, 'present', 'ms').key;
    return {
        schemaVersion: '1.1.0',
        id: PLAN_IDS.guides,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Guides',
        subtitle: 'Draft. Nothing on this page is verified.',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'verb-guides',
            title: 'Verb guides',
            theme: 'how a Hebrew verb is built',
            lessons: [{
                id: LESSON_IDS.guide,
                title: 'Decoding Hebrew Verbs',
                subtitle: 'Draft. Pointing is not verified.',
                level: 'basic',
                visibility: 'hidden',
                recipe: 'guide',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks: [{
                    id: 'decoding',
                    type: 'explainer',
                    title: 'Draft: decoding a Hebrew verb',
                    sections: [
                        {
                            id: 'root',
                            heading: 'The root',
                            paragraphs: [
                                'Draft. A Hebrew verb starts from a root of three letters, written with dots between them. The root is a meaning, not a word you say on its own.'
                            ],
                            examples: [
                                example('root-think', THINK.key, 'to think', THINK.root)
                            ]
                        },
                        {
                            id: 'binyan',
                            heading: 'The pattern',
                            paragraphs: [
                                'The pattern, or binyan, is the shape wrapped around the root. Paal is the usual shape in this draft. Hifil is a different shape: its present starts with mem, and its infinitive starts with le-ha.'
                            ],
                            examples: [
                                example('binyan-think', THINK.key, 'paal: to think', THINK.root),
                                example('binyan-feel', FEEL.key, 'hifil: to feel', FEEL.root)
                            ]
                        },
                        {
                            id: 'regular',
                            heading: 'Regular and special',
                            paragraphs: [
                                'A regular paal verb keeps one pattern in every cell. A special verb changes a letter or drops one. This page treats other patterns, and a few roots, as special. The labels are a study aid.'
                            ]
                        },
                        {
                            id: 'infinitive',
                            heading: 'The infinitive in paal',
                            paragraphs: [
                                'In paal the infinitive starts with lamed. A holam-vav stands before the last letter. A guttural right after the lamed takes patah, so think and answer start with la, and ask starts with li. Hifil does not use that holam. A lamed-he infinitive ends in vav-tav.'
                            ],
                            examples: [
                                example('inf-think', THINK.key, 'to think', THINK.root),
                                example('inf-ask', ASK.key, 'to ask', ASK.root),
                                example('inf-feel', FEEL.key, 'to feel (hifil)', FEEL.root),
                                example('inf-answer', ANSWER.key, 'to answer', ANSWER.root)
                            ]
                        },
                        {
                            id: 'present',
                            heading: 'The present',
                            paragraphs: [
                                'The present has four forms: masculine singular, feminine singular, masculine plural, and feminine plural. There is no separate I or you. The person comes from the pronoun beside the form.'
                            ],
                            table: {
                                columns: ['Cell', 'Hebrew', 'English'],
                                rows: PRESENT_CELLS.map((cell) => [
                                    cell,
                                    formAt(THINK.key, 'present', cell).key,
                                    GLOSS.think.present[cell]
                                ])
                            }
                        },
                        {
                            id: 'past',
                            heading: 'The past',
                            paragraphs: [
                                'The past has nine forms. The ending tells the person, the gender, and the number. The table uses think as the example. Other verbs in the workshop use the same cells.'
                            ],
                            table: {
                                columns: ['Cell', 'Hebrew', 'English'],
                                rows: PAST_CELLS.map((cell) => [
                                    cell,
                                    formAt(THINK.key, 'past', cell).key,
                                    GLOSS.think.past[cell]
                                ])
                            }
                        },
                        {
                            id: 'lights',
                            heading: 'Traffic lights',
                            paragraphs: [
                                'Green means the usual paal pattern. Yellow means a lamed-he root: the he drops or becomes a yod in many cells. Red means the verb does not follow the green pattern, so learn it on its own. The color name is also written in the title.'
                            ],
                            examples: [
                                example('ex-saw', saw.key, saw.en, saw.root),
                                example('ex-bought', bought.key, bought.en, bought.root),
                                example('ex-walk', walked.key, walked.en, walked.root),
                                example('ex-down', down.key, down.en, down.root),
                                example('ex-sat', sat.key, sat.en, sat.root),
                                example('ex-ms', thinkMs, 'thinks (m. sg.)', THINK.root)
                            ],
                            callouts: [
                                {
                                    id: 'green',
                                    tone: 'green',
                                    title: 'Green — regular',
                                    text: 'Think and ask follow the usual paal pattern. Start with those two.'
                                },
                                {
                                    id: 'yellow',
                                    tone: 'yellow',
                                    title: 'Yellow — lamed he',
                                    text: 'Saw, bought, and answer end in he. Expect the he to change.'
                                },
                                {
                                    id: 'red',
                                    tone: 'red',
                                    title: 'Red — irregular',
                                    text: 'Walk, go down, and sit do not follow the green pattern.'
                                }
                            ]
                        }
                    ]
                }]
            }]
        }]
    };
}

/**
 * @returns {object}
 */
function verbPlan() {
    const blocks = [{
        id: 'rule',
        type: 'explainer',
        title: 'Draft: one rule',
        sections: [{
            id: 'rule',
            heading: 'Root, pattern, ending',
            paragraphs: [
                'Draft. A Hebrew verb is a root in a pattern, plus an ending for the present or the past. These four verbs are think, feel, ask, and answer. Present has four forms. Past has nine. No form in this lesson is verified.'
            ],
            callouts: [{
                id: 'draft',
                tone: 'info',
                title: 'Info — not verified',
                text: 'The pointing is a draft. A person checks each form on the checklist before it counts as verified.'
            }]
        }]
    }];
    WORKSHOP.forEach((spec) => {
        blocks.push(tableBlock(spec, 'present', 'study', true));
    });
    WORKSHOP.forEach((spec) => {
        blocks.push(tableBlock(spec, 'past', 'study', false));
    });
    blocks.push(tableBlock(THINK, 'present', 'drill', false));
    const thinkMs = formAt(THINK.key, 'present', 'ms').key;
    const feelMs = formAt(FEEL.key, 'present', 'ms').key;
    blocks.push({
        id: 'closing',
        type: 'personal-question',
        prompts: [
            line(
                'p1',
                MA + ' ' + ATA + ' ' + thinkMs + ', ' + W(L('\u05D5', P.dagesh)) + MA + ' ' + ATA + ' ' + feelMs + '?',
                'What are you thinking, and what do you feel?'
            )
        ],
        answer: 'either'
    });
    return {
        schemaVersion: '1.1.0',
        id: PLAN_IDS.verbs,
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Verbs',
        subtitle: 'Draft. Nothing in this workshop is verified.',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'verb-workshop',
            title: 'Verb workshop',
            theme: 'think, feel, ask, answer',
            lessons: [{
                id: LESSON_IDS.workshop,
                title: 'Think, feel, ask, answer',
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

const PLANS = [guidePlan(), verbPlan()];

function defaultChecklistPath() {
    return path.join(
        rootDir(),
        'sdd',
        'initiatives',
        'INIT-006-language-program',
        'specs',
        'completion-summaries',
        'SPEC-018-forms-checklist.md'
    );
}

/**
 * @param {Set<string>} [ticked]
 * @returns {string}
 */
function renderChecklist(ticked) {
    const ticks = ticked || new Set();
    const lines = [
        '# SPEC-018 forms checklist',
        '',
        'Draft. These pointed forms are not verified.',
        'A person ticks a line only after checking that form.',
        'Write `[x]` in the box. The next run of `scripts/seed-starter-content.js` marks those forms verified and leaves every other form unverified.',
        'Untick a line and run again to clear it. Guide citation lemmas are stored unverified and are not in this list.',
        'This file is not a class transcript. The sentences in the lesson were written fresh for this draft.',
        ''
    ];
    WORKSHOP.forEach((spec) => {
        lines.push('## ' + spec.key + ' — ' + spec.gloss_en + ' — ' + spec.binyan + ' — ' + spec.root + ' — ' + spec.root_class);
        lines.push('');
        WORKSHOP_FORMS.filter((entry) => entry.lemma_key === spec.key).forEach((entry) => {
            const box = ticks.has(entry.key) ? 'x' : ' ';
            lines.push('- [' + box + '] | ' + entry.lemma_key + ' | ' + entry.tense + ' | ' + entry.cell + ' | ' + entry.key);
        });
        lines.push('');
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
        const match = lineText.match(/^- \[([ xX])\] \| (.+) \| (present|past) \| ([a-z0-9]+) \| (.+)$/);
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
 * @param {object} existing Public lexicon row.
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
 * Ops has no create-plan operation. A placeholder row exists only so replacePart
 * can replace the document. It is written after validation passes.
 * @param {object} store
 * @param {object} plan
 */
function ensureRow(store, plan) {
    if (store.loadWorkingCopy(plan.id)) return;
    store.saveWorkingCopy({
        id: plan.id,
        document: JSON.stringify({
            schemaVersion: '1.1.0',
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
 * Write context for this script (ADR-0011 §1): actor kind system, name
 * seed-starter-content. Loaded on use so requiring this file loads no plan module.
 * Provenance: INIT-008/SPEC-002
 * @returns {object}
 */
function seedContext() {
    const { makeContext } = require('../server/plans/context');
    return makeContext({ channel: 'system', name: 'seed-starter-content', tool: 'replace_part' });
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
        console.error('starter content refused: ' + result.error);
        process.exitCode = 1;
        return;
    }
    console.log('starter content ok forms=' + result.forms + ' verified=' + result.verified);
}

module.exports = {
    ACTOR,
    PLAN_IDS,
    LESSON_IDS,
    ENTRIES,
    WORKSHOP,
    WORKSHOP_FORMS,
    GUIDE_LEMMAS,
    PLANS,
    renderChecklist,
    parseTicks,
    run,
    defaultChecklistPath
};

if (require.main === module) main();
