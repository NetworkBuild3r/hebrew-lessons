/**
 * Lesson-plan validator: the ADR-0003 JSON Schema (draft-07) and the
 * Decision 6 rule registry, with the schemaVersion 1.1.0 blocks and rules of
 * ADR-0009 §7. A pure function of one document and, for N13 and N14, of the
 * lexicon the caller passes in. Its only I/O is reading the schema file, once.
 * Five more rules, all warnings with stable codes, come from hebrew-checks.js
 * and read an optional published snapshot (INIT-008/SPEC-008).
 * Provenance: INIT-004/SPEC-002; schemaVersion 1.1.0: INIT-006/SPEC-011;
 * picture-on-recall and closed-set: INIT-010/SPEC-002;
 * schemaVersion 1.2.0 and exercise: INIT-012/SPEC-002
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { HEBREW_RULES, checkHebrew } = require('./hebrew-checks');
const { ACTIVITY_TO_BLOCK } = require('./activity-map');

const SCHEMA_PATH = path.join(
    __dirname, '..', '..', 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json'
);
const DRAFT_07 = 'http://json-schema.org/draft-07/schema#';
const DEFINITIONS = '#/definitions/';
const V100 = '1.0.0';
const V110 = '1.1.0';
const V120 = '1.2.0';

/**
 * ADR-0003 Decision 6, one row per rule id. A figure is written here once, or
 * read from the schema (N8, N10, N11). The ADR numbers N1 to N12; its further
 * rules carry no number there, so they are named. N13 to N19, and the changes
 * to N1, N2, N6, and N7, are ADR-0009 §7 and reach only a 1.1.0 document.
 * A row with `checked: false` is never reported.
 */
const ADR_RULES = {
    schema: {
        severity: 'block',
        summary: 'Matches lesson-plan.schema.json. Unknown properties, an unknown schemaVersion, and a 1.1.0 construct in a 1.0.0 document fail.'
    },
    N1: {
        severity: 'block',
        legacySeverity: 'warn',
        maxNewWords: 8,
        summary: 'New words a lesson introduces, verse-reading newWords included. Warn instead of block when migration.legacy is true. A topic-conversation lesson is counted by N18.'
    },
    N2: {
        severity: 'warn',
        minRecycles: 3,
        exemptRecipes: ['guide'],
        summary: 'Recycled words in a lesson whose buildsOn is non-empty. Not for a guide.'
    },
    N3: {
        severity: 'warn',
        minKnownPercent: 95,
        summary: 'Known running words in a graded-story or listening passage.'
    },
    N4: {
        severity: null,
        checked: false,
        summary: 'No second coverage bar for listening. N3 is the only coverage warn.'
    },
    N5: {
        severity: 'block',
        summary: 'One introduction per lesson, so at most one pattern.'
    },
    N6: {
        severity: 'warn',
        minOccurrences: 2,
        summary: 'Occurrences of each new word in its introducing lesson. Also each verse-reading new word, where a verse and its questions count.'
    },
    N7: {
        severity: 'warn',
        blocksPerIntroduction: 4,
        languageFocused: ['introduction', 'explainer', 'conjugation-table', 'letter-intro'],
        excludedRecipes: ['guide', 'letters-and-vowels'],
        summary: 'Share of language-focused blocks among the blocks of a unit. In 1.0.0 only introduction counts. Guide and letters-and-vowels lessons are left out of both sides.'
    },
    N8: {
        severity: 'block',
        summary: 'Image alt length, from the schema altText definition. A missing alt fails.'
    },
    N9: {
        severity: 'block',
        planBytes: 512 * 1024,
        lessonBytes: 128 * 1024,
        blockBytes: 32 * 1024,
        summary: 'UTF-8 size of the compact JSON of a plan, a lesson, and a block.'
    },
    N10: {
        severity: 'block',
        summary: 'Units, blocks, and introduction words, from the schema maxItems. Lessons have no maximum (INIT-010/SPEC-002).'
    },
    N11: {
        severity: 'block',
        summary: 'Plain-text length, from the schema plainText definition. Also applied to map keys.'
    },
    N12: {
        severity: 'block',
        sharedLessonSeverity: 'warn',
        summary: 'A fluency-pass has no word list. Warn when its lesson introduces new words.'
    },
    N13: {
        severity: 'block',
        summary: 'Each conjugation-table formKey names a stored form whose lemma is the table lemmaKey, whose tense is the table tense, and whose cell is the entry cell. A cell appears at most once. Without a lexicon no form can be shown to be stored.'
    },
    N14: {
        severity: 'warn',
        summary: 'A conjugation-table lists a stored form that is not verified.'
    },
    N15: {
        severity: 'warn',
        exemptRecipes: ['guide'],
        firstBlock: 'spaced-review-checkpoint',
        summary: 'A lesson with a recipe other than guide and a non-empty buildsOn starts with a spaced-review-checkpoint.'
    },
    N16: {
        severity: 'inform',
        recipes: ['topic-conversation', 'verb-workshop', 'verse-reading'],
        summary: 'A topic-conversation, verb-workshop, or verse-reading lesson contains a personal-question, or for verse-reading a personalQuestion.'
    },
    N17: {
        severity: 'inform',
        runLength: 3,
        summary: 'Three lessons in a row in one unit have the same recipe.'
    },
    N18: {
        severity: 'warn',
        blockSeverity: 'block',
        legacySeverity: 'warn',
        recipe: 'topic-conversation',
        warnAbove: 8,
        blockAbove: 15,
        maxGroupWords: 5,
        summary: 'A topic-conversation lesson warns above 8 new words or on an introduction group over 5, and blocks above 15. These are N1 for that recipe, so migration.legacy turns the block into a warn.'
    },
    N19: {
        severity: 'block',
        summary: 'A verse-reading carries a non-empty ref.source.'
    },
    N20: {
        severity: 'block',
        summary: 'In a choose item, answer must equal one of that item\'s choices.'
    },
    N21: {
        severity: 'block',
        summary: 'In an unscramble item, words has 3–10 entries and each alt order is a different permutation of the same multiset.'
    },
    N22: {
        severity: 'block',
        summary: 'In a dictation item, audioPath is present and matches the plan audioPath pattern.'
    },
    N23: {
        severity: 'warn',
        maxExerciseBlocks: 2,
        recipe: 'topic-conversation',
        summary: 'A topic-conversation lesson with more than two exercise blocks warns that exercises may crowd out conversation.'
    },
    'later-context': {
        severity: 'warn',
        summary: 'In a multi-lesson plan, each new word appears in a later lesson in a different sentence.'
    },
    'later-checkpoint': {
        severity: 'warn',
        summary: 'In a multi-lesson plan, a later spaced-review-checkpoint lists each new word.'
    },
    'recall-day': {
        severity: 'inform',
        summary: 'Recall on a later calendar day is the scheduler\'s job. The plan has no dates.'
    },
    'semantic-set': {
        severity: 'warn',
        checked: false,
        summary: 'New words are not one semantic category. Judged by the author or reviewer, not counted.'
    },
    'frequency-band': {
        severity: 'inform',
        summary: 'A unit frequencyBand is recorded and not checked. No Hebrew frequency list is validated.'
    },
    'pattern-focus': {
        severity: 'warn',
        summary: 'The pattern focus appears in a story or dialogue in the lesson or later in the unit.'
    },
    'pointed-word': {
        severity: 'warn',
        summary: 'Each new word, verse-reading newWords included, carries a Hebrew vowel point.'
    },
    'romanized-answer': {
        severity: 'block',
        summary: 'A cloze answer and the correct pattern choice contain a Hebrew letter.'
    },
    questions: {
        severity: 'inform',
        summary: 'Comprehension questions are optional and are not the recall event.'
    },
    'queue-order': {
        severity: 'inform',
        summary: 'A personal queue may insert due reviews. It keeps this plan\'s lesson order.'
    },
    meaning: {
        severity: 'block',
        summary: 'Every new word has an introduction entry with a first-language meaning.'
    },
    'image-placement': {
        severity: 'block',
        summary: 'Images appear only on introduction words.'
    },
    'form-recall-image': {
        severity: 'block',
        checked: false,
        summary: 'No image on a written-form task. The conjugation-table drill is the only one and has no image field, so image-placement reports any image there.'
    },
    'picture-on-recall': {
        severity: 'warn',
        summary: 'A type-it block whose cue is a picture. Warn only; does not block publish (ADR-0013).'
    },
    'closed-set': {
        severity: 'warn',
        summary: 'Every new word in a lesson is a kinship term. Warn only; does not block publish (ADR-0013).'
    },
    html: {
        severity: 'block',
        summary: 'No string in the plan starts an HTML tag: the schema plainText markup pattern.'
    },
    'audio-path': {
        severity: 'block',
        summary: 'Every audio path matches the schema relative .mp3 pattern.'
    }
};

/**
 * Kinship new-word set for closed-set (mother, father, brother, sister, son,
 * daughter). Compare after stripping nikud. Provenance: INIT-010/SPEC-002
 */
const KINSHIP_STRIPPED = Object.freeze(new Set([
    'אם', 'אב', 'אח', 'אחות', 'בן', 'בת'
].map((word) => word.normalize('NFC'))));

// Keep the activity map loaded so a missing module fails at require time.
void ACTIVITY_TO_BLOCK;

/**
 * The five INIT-008 rows (hebrew-checks.js): pointing-form, field-language,
 * lexicon-form, pointing-consistency, audio-stale. All `warn`; none can block.
 * They are own properties of RULES, so `RULES[id]` and `hasOwnProperty` find
 * them, but they are not enumerable: the ADR-0003 Decision 6 table tests
 * (`Object.keys(RULES)`) keep counting that table's rows only.
 */
Object.keys(HEBREW_RULES).forEach((id) => {
    Object.defineProperty(ADR_RULES, id, { value: HEBREW_RULES[id], enumerable: false });
});
const RULES = deepFreeze(ADR_RULES);

/** ADR-0003 §6 closed function list: a denominator for N3, not a claim that these were taught. */
const FUNCTION_WORDS = Object.freeze([
    'אֲנִי', 'אַתָּה', 'אַתְּ', 'הוּא', 'הִיא', 'אֲנַחְנוּ', 'זֶה', 'לֹא', 'כֵּן'
].map((word) => word.normalize('NFC')));

const SCHEMA_KEYWORDS = new Set([
    '$schema', '$id', '$ref', 'title', 'description', 'definitions',
    'type', 'const', 'enum', 'properties', 'required', 'additionalProperties', 'maxProperties',
    'items', 'minItems', 'maxItems', 'minLength', 'maxLength', 'pattern', 'minimum', 'maximum',
    'allOf', 'oneOf', 'not', 'if', 'then', 'else'
]);
const SCHEMA_TYPES = new Set(['null', 'boolean', 'object', 'array', 'number', 'integer', 'string']);

const MESSAGES = Object.freeze({
    notJson: 'the plan is not a JSON document',
    html: 'markup is not allowed in plan text',
    image: 'images are allowed only on introduction words',
    fluencyList: 'a fluency-pass block has no word list',
    romanized: 'the correct answer has no Hebrew letter',
    mediaId: 'media id must be an opaque slug, not a URL, path, or filename',
    v110Field: ' needs schemaVersion 1.1.0',
    v110Block: 'this block type needs schemaVersion 1.1.0',
    v120Field: ' needs schemaVersion 1.2.0',
    v120Block: 'this block type needs schemaVersion 1.2.0',
    lexiconKey: 'a lexicon key needs a Hebrew letter',
    cellCode: 'not a present or past cell code',
    cellTense: 'this cell does not belong to the table tense',
    cellTwice: 'this cell appears more than once in the table',
    noLexicon: 'no lexicon was given, so this form cannot be checked',
    noForm: 'formKey names no stored form',
    otherLemma: 'formKey names a form of another lemma',
    otherTense: 'formKey names a form of another tense',
    otherCell: 'formKey names a form of another cell',
    unverified: 'this form is not verified',
    verseSource: 'a verse-reading needs a non-empty ref.source naming its edition'
});

/** Schema text definitions that carry the plainText markup pattern. */
const MARKUP_DEFS = new Set([
    'plainText', 'altText', 'text80', 'text120', 'text200', 'text300', 'text600', 'text1500',
    'tableCell', 'lexiconKey'
]);

/** ADR-0001 strip range, the same one speech.js and HebrewUtils.stripVowels use. */
const POINTS_AND_ACCENTS = /[\u0591-\u05C7]/g;
const HEBREW_POINT = /[\u05B0-\u05BD\u05BF\u05C1\u05C2\u05C7]/;
const HEBREW_LETTER = /[\u05D0-\u05EA]/;
const EDGE_MARK = /^[\p{P}\p{S}\p{Cf}]$/u;

const N10_LISTS = [
    /^\/units$/,
    /^\/units\/\d+\/lessons$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/words$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/items$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/items\/\d+\/choices$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/items\/\d+\/words$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/items\/\d+\/alt$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/items\/\d+\/accept$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/useWords$/,
    /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/homework$/
];
const ACCEPT_STRING = /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/items\/\d+\/accept\/\d+$/;
const WORD_MEANING = /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/words\/\d+\/meaning$/;
const BLOCK_CHILD = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)\/([^/]+)$/;
const INTRO_WORD = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)\/words\/\d+$/;
/** Keys of a glosses map are story tokens, not property names. */
const GLOSS_MAP = /^\/units\/\d+\/lessons\/\d+\/blocks\/\d+\/glosses$/;
const VERSE_SOURCE = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)\/ref\/source$/;
const CELL_AT = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)\/cells\/(\d+)\/cell$/;

/**
 * @typedef {object} Finding
 * @property {string} rule A key of RULES.
 * @property {'block'|'warn'|'inform'} severity
 * @property {string} path RFC 6901 pointer into the plan; the empty string is the plan itself.
 * @property {string} message
 * @property {string} [field] INIT-008 rules only: the pointer remainder below the deepest id node.
 * @property {object} [fix] INIT-008 rules only: `{ action, path, field, suggested | candidates }`.
 */

/**
 * @typedef {object} ValidationResult
 * @property {boolean} ok True when there is no block finding, so publish may proceed.
 * @property {Finding[]} blocks
 * @property {Finding[]} warnings
 * @property {Finding[]} informs
 */

/**
 * @typedef {object} LexiconSource
 * @property {(key: string) => (object|null|undefined)} find Row for an exactKey,
 *   with `kind`, `lemma_key`, `tense`, `cell`, and `verified`, as server/db/repos/lexicon.js returns it.
 */

/**
 * Validate one plan document against the schema and the Decision 6 registry.
 * Accepts the parsed document or its JSON text and never mutates the input.
 * An unknown schemaVersion is reported alone: the rest is not parsed. A plan
 * over its N9 size is reported by size alone: it is refused either way, and
 * this bounds the work and the report an oversized input can cause.
 * @param {unknown} document
 * @param {{ lexicon?: LexiconSource, snapshot?: object|string|null }} [options] N13 and N14 read stored forms
 *   from `lexicon`. Without it every conjugation-table form is an N13 block. `lexicon-form` also
 *   needs `lexicon.listByStrippedKeys(keys)`. `snapshot` is the published copy, parsed or as
 *   JSON text, for `audio-stale`; without it that rule is silent.
 * @returns {ValidationResult}
 */
function validatePlan(document, options) {
    const findings = createFindings();
    const parsed = canonical(document);
    if (parsed.error) {
        findings.add('schema', '', parsed.error);
        return findings.result();
    }
    const plan = parsed.plan;
    const schema = loadSchema();
    if (!isObject(plan)) {
        findings.add('schema', '', 'a plan must be a JSON object');
        return findings.result();
    }
    if (!schema.versions.includes(plan.schemaVersion)) {
        findings.add('schema', '/schemaVersion',
            'schemaVersion must be ' + schema.versions.join(' or ') + '; other versions are not parsed');
        return findings.result();
    }
    const context = {
        v110: plan.schemaVersion === V110 || plan.schemaVersion === V120,
        v120: plan.schemaVersion === V120,
        recipes: schema.recipes,
        keyLimit: schema.keyLimit,
        lexicon: lexiconOf(options)
    };
    const lessons = lessonsOf(plan, context);
    if (!checkSizes(lessons, parsed.bytes, findings)) return findings.result();
    checkSchema(plan, schema, findings);
    checkStrings(plan, schema, findings);
    checkLessonRules(plan, lessons, context, findings);
    checkEstimates(lessons, context, findings);
    checkHebrewRules(plan, options, findings);
    return findings.result();
}

/**
 * The five INIT-008 warnings. They never block: each is added at its registry
 * severity, `warn`, and `ok` depends on blocks alone.
 */
function checkHebrewRules(plan, options, findings) {
    const given = isObject(options) ? options : {};
    checkHebrew(plan, { lexicon: given.lexicon, snapshot: given.snapshot }).forEach((item) => {
        const extra = { field: item.field };
        if (item.fix) extra.fix = item.fix;
        findings.add(item.rule, item.path, item.message, RULES[item.rule].severity, extra);
    });
}

/**
 * True when a plan holds a construct ADR-0009 §7 allows only in 1.1.0: one of
 * the five block types, a lesson `recipe`, or a `group` on a word. A key that
 * is present counts whatever its value, so the schema judges the value.
 * @param {unknown} plan
 * @returns {boolean}
 */
function usesV110(plan) {
    const types = loadSchema().v110Types;
    for (const unit of list(isObject(plan) ? plan.units : undefined)) {
        for (const lesson of list(isObject(unit) ? unit.lessons : undefined)) {
            if (!isObject(lesson)) continue;
            if (has(lesson, 'recipe')) return true;
            for (const block of list(lesson.blocks)) {
                if (!isObject(block)) continue;
                if (types.has(block.type)) return true;
                if (list(block.words).some((word) => isObject(word) && has(word, 'group'))) return true;
            }
        }
    }
    return false;
}

/**
 * True when a plan holds a construct ADR-0014 allows only in 1.2.0: an exercise
 * or assignment block, or a conjugation-table with tense future (SPEC-003).
 * @param {unknown} plan
 * @returns {boolean}
 */
function usesV120(plan) {
    const types = loadSchema().v120Types;
    for (const unit of list(isObject(plan) ? plan.units : undefined)) {
        for (const lesson of list(isObject(unit) ? unit.lessons : undefined)) {
            if (!isObject(lesson)) continue;
            for (const block of list(lesson.blocks)) {
                if (!isObject(block)) continue;
                if (types.has(block.type)) return true;
                if (block.type === 'conjugation-table' && block.tense === 'future') return true;
            }
        }
    }
    return false;
}

/**
 * The caller's lexicon behind a per-call memo, so each key is looked up once.
 * @param {unknown} options
 * @returns {{ available: boolean, find: (key: string) => object|null }}
 */
function lexiconOf(options) {
    const source = isObject(options) && isObject(options.lexicon) && typeof options.lexicon.find === 'function'
        ? options.lexicon
        : null;
    const rows = new Map();
    return {
        available: source !== null,
        find(key) {
            const normalized = exactKey(key);
            if (!rows.has(normalized)) {
                const row = source.find(normalized);
                rows.set(normalized, isObject(row) ? row : null);
            }
            return rows.get(normalized);
        }
    };
}

/**
 * Round-trip through JSON so the rules see exactly the document a store would
 * save, and measure its compact UTF-8 size for N9.
 * @param {unknown} document
 * @returns {{ plan?: unknown, bytes?: number, error?: string }}
 */
function canonical(document) {
    try {
        const source = typeof document === 'string' ? document : JSON.stringify(document);
        if (typeof source !== 'string') return { error: MESSAGES.notJson };
        const plan = JSON.parse(source);
        const compact = typeof document === 'string' ? JSON.stringify(plan) : source;
        return { plan, bytes: Buffer.byteLength(compact, 'utf8') };
    } catch (err) {
        return { error: MESSAGES.notJson };
    }
}

function createFindings() {
    const seen = new Set();
    const items = [];
    return {
        /**
         * `extra` carries the `field` and `fix` keys of the INIT-008 rules. A finding with
         * `extra` is kept once per message, so one field can carry two pointing-form clauses.
         * Findings of the older rules are given no `extra` and gain no keys.
         */
        add(rule, at, message, severity, extra) {
            const key = rule + '\u0000' + at + (extra ? '\u0000' + message : '');
            if (seen.has(key)) return;
            seen.add(key);
            const item = { rule, severity: severity || RULES[rule].severity, path: at, message };
            if (extra) Object.assign(item, extra);
            items.push(item);
        },
        result() {
            const pick = (severity) => items.filter((item) => item.severity === severity);
            const blocks = pick('block');
            return { ok: blocks.length === 0, blocks, warnings: pick('warn'), informs: pick('inform') };
        }
    };
}

// --- schema --------------------------------------------------------------

let loaded = null;
let schemaSource = null;

/**
 * The plan JSON Schema file as text, exactly the bytes the validator reads
 * from SCHEMA_PATH, read once per process. The admin editor gets it from
 * GET /api/plan-schema and builds its forms from it (INIT-008/SPEC-010).
 * @returns {string}
 */
function schemaText() {
    if (schemaSource === null) schemaSource = fs.readFileSync(SCHEMA_PATH, 'utf8');
    return schemaSource;
}

/**
 * Read the schema once. Throws on a keyword or type this walker does not
 * implement, so a schema change cannot be silently ignored.
 * @returns {{ root: object, patterns: Map<string, RegExp>, versions: unknown[], markup: RegExp, textLimit: number,
 *   recipes: Set<string>, v110Types: Set<string>, v120Types: Set<string>, cellCodes: Set<string>,
 *   keyLimit: number, audioPath: RegExp }}
 */
function loadSchema() {
    if (loaded) return loaded;
    const root = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
    if (!isObject(root) || root.$schema !== DRAFT_07) {
        throw new Error('lesson-plan schema is not JSON Schema draft-07');
    }
    const patterns = new Map();
    assertSupported(root, root, '#', patterns);
    const version = isObject(root.properties) ? root.properties.schemaVersion : undefined;
    const versions = isObject(version) && has(version, 'const') ? [version.const] : list(version && version.enum);
    const defs = isObject(root.definitions) ? root.definitions : {};
    const text = defs.plainText;
    const markup = isObject(text) && isObject(text.not) ? patterns.get(text.not.pattern) : undefined;
    if (!versions.length || !markup || !Number.isInteger(text.maxLength)) {
        throw new Error('lesson-plan schema lacks schemaVersion or the plainText definition');
    }
    const recipes = isObject(defs.recipe) ? list(defs.recipe.enum) : [];
    const v110Types = isObject(defs.v110BlockType) && isObject(defs.v110BlockType.not)
        ? list(defs.v110BlockType.not.enum)
        : [];
    const v120Types = isObject(defs.v120BlockType) && isObject(defs.v120BlockType.not)
        ? list(defs.v120BlockType.not.enum)
        : [];
    const key = defs.lexiconKey;
    const cellCodes = isObject(defs.cellCode) ? list(defs.cellCode.enum) : [];
    const audio = defs.audioPath;
    const audioPath = isObject(audio) && typeof audio.pattern === 'string'
        ? patterns.get(audio.pattern)
        : undefined;
    if (!versions.includes(V100) || !versions.includes(V110) || !versions.includes(V120) || !recipes.length
        || !v110Types.length || !v120Types.length || !cellCodes.length || !isObject(key)
        || !Number.isInteger(key.maxLength) || !audioPath) {
        throw new Error('lesson-plan schema lacks version, recipe, block-type, cell, lexicon, or audioPath definitions');
    }
    loaded = Object.freeze({
        root,
        patterns,
        versions,
        markup,
        textLimit: text.maxLength,
        recipes: new Set(recipes),
        v110Types: new Set(v110Types),
        v120Types: new Set(v120Types),
        cellCodes: new Set(cellCodes),
        keyLimit: key.maxLength,
        audioPath
    });
    return loaded;
}

function assertSupported(node, root, where, patterns) {
    if (!isObject(node)) throw new Error('schema node at ' + where + ' is not an object');
    Object.keys(node).forEach((key) => {
        if (!SCHEMA_KEYWORDS.has(key)) throw new Error('schema keyword ' + key + ' at ' + where + ' is not implemented');
    });
    if (node.$ref !== undefined) assertLocalRef(node, root, where);
    [].concat(node.type === undefined ? [] : node.type).forEach((type) => {
        if (!SCHEMA_TYPES.has(type)) throw new Error('schema type ' + type + ' at ' + where + ' is not implemented');
    });
    if (node.pattern !== undefined) patterns.set(node.pattern, new RegExp(node.pattern, 'u'));
    if (Array.isArray(node.items)) throw new Error('tuple items at ' + where + ' are not implemented');
    ['items', 'not', 'if', 'then', 'else'].forEach((key) => {
        if (node[key] !== undefined) assertSupported(node[key], root, where + '/' + key, patterns);
    });
    ['allOf', 'oneOf'].forEach((key) => {
        list(node[key]).forEach((child, i) => assertSupported(child, root, where + '/' + key + '/' + i, patterns));
    });
    ['properties', 'definitions'].forEach((key) => {
        if (!isObject(node[key])) return;
        Object.keys(node[key]).forEach((name) => {
            assertSupported(node[key][name], root, where + '/' + key + '/' + name, patterns);
        });
    });
    if (isObject(node.additionalProperties)) {
        assertSupported(node.additionalProperties, root, where + '/additionalProperties', patterns);
    }
}

function assertLocalRef(node, root, where) {
    const seen = new Set();
    let target = node;
    while (isObject(target) && target.$ref !== undefined) {
        const ref = target.$ref;
        const name = typeof ref === 'string' && ref.startsWith(DEFINITIONS) ? ref.slice(DEFINITIONS.length) : '';
        if (!name || !isObject(root.definitions) || !has(root.definitions, name) || seen.has(name)) {
            throw new Error('schema $ref ' + ref + ' at ' + where + ' is not a local, acyclic definition');
        }
        seen.add(name);
        target = root.definitions[name];
    }
}

function checkSchema(plan, schema, findings) {
    const errors = [];
    check(plan, schema.root, { root: schema.root, patterns: schema.patterns, def: null }, '', errors);
    errors.forEach((error) => {
        const [rule, message] = ruleForSchemaError(error, plan, schema);
        findings.add(rule, error.path, message);
    });
}

/**
 * Name the registry rule a schema failure belongs to. The schema carries the
 * N8, N10, and N11 figures, the markup, audio-path, and meaning checks, the
 * 1.0.0 restriction, and in 1.1.0 the N13 tense of a cell and the N19 source.
 * A value of the wrong shape stays `schema`, so ops.js refuses to store it.
 * @returns {[string, string]} rule id and message
 */
function ruleForSchemaError(error, plan, schema) {
    const { keyword, def, property } = error;
    const at = error.path;
    const v110 = plan.schemaVersion === V110 || plan.schemaVersion === V120;
    if (def === 'v110Only') return ['schema', at.slice(at.lastIndexOf('/') + 1) + MESSAGES.v110Field];
    if (def === 'v110BlockType') return ['schema', MESSAGES.v110Block];
    if (def === 'v120Only') {
        const leaf = at.slice(at.lastIndexOf('/') + 1);
        if (leaf === 'tense') {
            const match = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)\/tense$/.exec(at);
            if (match) {
                const block = blockAt(plan, Number(match[1]), Number(match[2]), Number(match[3]));
                if (block && block.tense === 'future') {
                    return ['schema', 'future tense needs schemaVersion 1.2.0'];
                }
            }
        }
        return ['schema', leaf + MESSAGES.v120Field];
    }
    if (def === 'v120BlockType') return ['schema', MESSAGES.v120Block];
    if (def === 'acceptString' && keyword === 'not') {
        return ['schema', 'accept must not include vowel points or cantillation'];
    }
    if (def === 'acceptString' && keyword === 'pattern') {
        return ['schema', 'accept must include at least one Hebrew letter'];
    }
    if (keyword === 'not' && MARKUP_DEFS.has(def)) return ['html', MESSAGES.html];
    if (def === 'cellCode' || def === 'presentCell' || def === 'pastCell' || def === 'futureCell') {
        if (def === 'cellCode' || !schema.cellCodes.has(cellAt(plan, at))) return ['schema', MESSAGES.cellCode];
        return [v110 ? 'N13' : 'schema', MESSAGES.cellTense];
    }
    if (v110 && (keyword === 'required' || keyword === 'minLength') && verseSourceAt(plan, at)) {
        return ['N19', MESSAGES.verseSource];
    }
    if (def === 'lexiconKey' && keyword === 'pattern') return ['schema', MESSAGES.lexiconKey];
    if (keyword === 'required' && property === 'alt') return ['N8', 'an image needs alt text'];
    if (def === 'altText') return ['N8', 'alt ' + error.message];
    if (def === 'audioPath') {
        return ['audio-path', keyword === 'pattern'
            ? 'audio path must be a relative .mp3 path with no scheme or leading slash'
            : 'audio path ' + error.message];
    }
    if (def === 'plainText' && keyword === 'maxLength') return ['N11', error.message];
    if (WORD_MEANING.test(at)) {
        return ['meaning', keyword === 'required' ? 'every word needs a first-language meaning' : 'meaning ' + error.message];
    }
    if (keyword === 'maxItems' && N10_LISTS.some((pattern) => pattern.test(at))) return ['N10', error.message];
    if (keyword === 'additionalProperties') {
        if (property === 'image' || property === 'mediaId') return ['image-placement', MESSAGES.image];
        const child = BLOCK_CHILD.exec(at);
        if (child && (child[4] === 'words' || child[4] === 'newWords')) {
            const block = blockAt(plan, Number(child[1]), Number(child[2]), Number(child[3]));
            if (block && block.type === 'fluency-pass') return ['N12', MESSAGES.fluencyList];
        }
    }
    if (keyword === 'pattern' && at.endsWith('/mediaId')) return ['schema', MESSAGES.mediaId];
    if (keyword === 'pattern' && at === '/assigneeId') {
        return ['schema', "This username format can't be assigned to a personal plan yet."];
    }
    return ['schema', error.message];
}

function check(instance, node, ctx, at, errors) {
    const { schema, scope } = resolve(node, ctx);
    if (has(schema, 'const') && !sameJson(instance, schema.const)) {
        push(errors, at, 'const', scope.def, 'must be ' + JSON.stringify(schema.const));
        return;
    }
    if (Array.isArray(schema.enum) && !schema.enum.some((value) => sameJson(instance, value))) {
        push(errors, at, 'enum', scope.def,
            'must be one of ' + schema.enum.map((value) => JSON.stringify(value)).join(', '));
        return;
    }
    if (schema.type !== undefined && !typeMatches(instance, schema.type)) {
        push(errors, at, 'type', scope.def, 'must be ' + [].concat(schema.type).join(' or '));
        return;
    }
    let overLength = false;
    if (typeof instance === 'string') overLength = checkString(instance, schema, scope, at, errors);
    else if (typeof instance === 'number') checkNumber(instance, schema, scope, at, errors);
    else if (Array.isArray(instance)) checkArray(instance, schema, scope, at, errors);
    else if (isObject(instance)) checkObject(instance, schema, scope, at, errors);

    if (schema.not !== undefined && !overLength && matches(instance, schema.not, scope)) {
        push(errors, at, 'not', scope.def, 'matches a forbidden pattern');
    }
    list(schema.allOf).forEach((part) => check(instance, part, scope, at, errors));
    if (schema.if !== undefined) {
        const branch = matches(instance, schema.if, scope) ? schema.then : schema.else;
        if (branch !== undefined) check(instance, branch, scope, at, errors);
    }
    if (Array.isArray(schema.oneOf)) checkOneOf(instance, schema.oneOf, scope, at, errors);
}

function resolve(node, ctx) {
    let schema = node;
    let def = ctx.def;
    while (isObject(schema) && schema.$ref !== undefined) {
        def = schema.$ref.slice(DEFINITIONS.length);
        schema = ctx.root.definitions[def];
    }
    return { schema, scope: def === ctx.def ? ctx : { root: ctx.root, patterns: ctx.patterns, def } };
}

function matches(instance, node, ctx) {
    const errors = [];
    check(instance, node, ctx, '', errors);
    return errors.length === 0;
}

/**
 * Lengths count code points, as JSON Schema does. A string over maxLength
 * skips `pattern` and `not`: it already fails, and this bounds regex input.
 * @returns {boolean} true when the string is over maxLength
 */
function checkString(value, schema, ctx, at, errors) {
    const length = codePoints(value);
    if (schema.minLength !== undefined && length < schema.minLength) {
        push(errors, at, 'minLength', ctx.def, schema.minLength === 1
            ? 'must not be empty'
            : 'must be at least ' + schema.minLength + ' characters');
    }
    if (schema.maxLength !== undefined && length > schema.maxLength) {
        push(errors, at, 'maxLength', ctx.def, 'must be at most ' + schema.maxLength + ' characters');
        return true;
    }
    if (schema.pattern !== undefined && !ctx.patterns.get(schema.pattern).test(value)) {
        push(errors, at, 'pattern', ctx.def, 'does not match the required format');
    }
    return false;
}

function checkNumber(value, schema, ctx, at, errors) {
    if (schema.minimum !== undefined && value < schema.minimum) {
        push(errors, at, 'minimum', ctx.def, 'must be at least ' + schema.minimum);
    }
    if (schema.maximum !== undefined && value > schema.maximum) {
        push(errors, at, 'maximum', ctx.def, 'must be at most ' + schema.maximum);
    }
}

function checkArray(value, schema, ctx, at, errors) {
    if (schema.minItems !== undefined && value.length < schema.minItems) {
        push(errors, at, 'minItems', ctx.def, 'must have at least ' + schema.minItems + ' item'
            + (schema.minItems === 1 ? '' : 's'));
    }
    if (schema.maxItems !== undefined && value.length > schema.maxItems) {
        push(errors, at, 'maxItems', ctx.def, 'must have at most ' + schema.maxItems + ' items');
    }
    if (schema.items !== undefined) {
        value.forEach((item, index) => check(item, schema.items, ctx, pointer(at, index), errors));
    }
}

function checkObject(value, schema, ctx, at, errors) {
    const keys = Object.keys(value);
    if (schema.maxProperties !== undefined && keys.length > schema.maxProperties) {
        push(errors, at, 'maxProperties', ctx.def, 'must have at most ' + schema.maxProperties + ' entries');
    }
    list(schema.required).forEach((key) => {
        if (!has(value, key)) push(errors, pointer(at, key), 'required', ctx.def, 'missing required field: ' + key, key);
    });
    const properties = isObject(schema.properties) ? schema.properties : {};
    keys.forEach((key) => {
        const child = pointer(at, key);
        if (has(properties, key)) {
            check(value[key], properties[key], ctx, child, errors);
        } else if (schema.additionalProperties === false) {
            push(errors, child, 'additionalProperties', ctx.def, 'unknown property', key);
        } else if (isObject(schema.additionalProperties)) {
            check(value[key], schema.additionalProperties, ctx, child, errors);
        }
    });
}

/**
 * When no branch matches, report the errors of the branch whose `type` const
 * names the instance's type, so a bad block explains itself.
 */
function checkOneOf(value, branches, ctx, at, errors) {
    let matched = 0;
    let declared = null;
    branches.forEach((branch) => {
        const branchErrors = [];
        check(value, branch, ctx, at, branchErrors);
        if (!branchErrors.length) matched += 1;
        if (isObject(value) && value.type !== undefined && discriminator(branch, ctx) === value.type) {
            declared = branchErrors;
        }
    });
    if (matched === 1) return;
    if (matched > 1) push(errors, at, 'oneOf', ctx.def, 'matches more than one allowed shape');
    else if (declared) declared.forEach((error) => errors.push(error));
    else push(errors, at, 'oneOf', ctx.def, isObject(value) ? 'unknown block type' : 'must be an object of a known type');
}

function discriminator(branch, ctx) {
    const { schema } = resolve(branch, ctx);
    const type = isObject(schema.properties) ? schema.properties.type : undefined;
    return isObject(type) && has(type, 'const') ? type.const : undefined;
}

function typeMatches(instance, type) {
    return [].concat(type).some((name) => {
        switch (name) {
            case 'null': return instance === null;
            case 'boolean': return typeof instance === 'boolean';
            case 'object': return isObject(instance);
            case 'array': return Array.isArray(instance);
            case 'number': return typeof instance === 'number' && Number.isFinite(instance);
            case 'integer': return Number.isInteger(instance);
            case 'string': return typeof instance === 'string';
            default: return false;
        }
    });
}

function sameJson(a, b) {
    if (a === b) return true;
    if (Array.isArray(a)) {
        return Array.isArray(b) && a.length === b.length && a.every((value, i) => sameJson(value, b[i]));
    }
    if (isObject(a) && isObject(b)) {
        const keys = Object.keys(a);
        return keys.length === Object.keys(b).length && keys.every((key) => has(b, key) && sameJson(a[key], b[key]));
    }
    return false;
}

function push(errors, at, keyword, def, message, property) {
    errors.push({ path: at, keyword, def: def || null, property: property === undefined ? null : property, message });
}

// --- whole-document scans --------------------------------------------------

/**
 * Every string and every key, including keys of free-form maps the schema does
 * not limit (glosses) and values under unknown properties the schema never
 * descends into. Iterative, so nesting depth cannot exhaust the stack.
 */
function checkStrings(plan, schema, findings) {
    const stack = [{ value: plan, at: '', key: null, parent: '' }];
    while (stack.length) {
        const { value, at, key, parent } = stack.pop();
        if (key !== null) checkKey(plan, schema, key, parent, at, findings);
        if (typeof value === 'string') {
            if (schema.markup.test(value)) findings.add('html', at, MESSAGES.html);
        } else if (Array.isArray(value)) {
            for (let i = value.length - 1; i >= 0; i -= 1) {
                stack.push({ value: value[i], at: pointer(at, i), key: null, parent: at });
            }
        } else if (isObject(value)) {
            const keys = Object.keys(value);
            for (let i = keys.length - 1; i >= 0; i -= 1) {
                stack.push({ value: value[keys[i]], at: pointer(at, keys[i]), key: keys[i], parent: at });
            }
        }
    }
}

function checkKey(plan, schema, key, parent, at, findings) {
    if (schema.markup.test(key)) findings.add('html', at, MESSAGES.html);
    if (codePoints(key) > schema.textLimit) {
        findings.add('N11', at, 'key must be at most ' + schema.textLimit + ' characters');
    }
    if (GLOSS_MAP.test(parent)) return;
    // type-it picture cues are picture-on-recall (warn), not image-placement (block).
    if (typeItAt(plan, parent) || typeItAt(plan, at)) return;
    if (key === 'image' && !introWordAt(plan, parent)) findings.add('image-placement', at, MESSAGES.image);
    if (key === 'mediaId') {
        const imageAt = parent.endsWith('/image')
            ? parent.slice(0, -'/image'.length)
            : (parent.endsWith('/picture') ? parent.slice(0, -'/picture'.length) : null);
        if (imageAt === null || !introWordAt(plan, imageAt)) findings.add('image-placement', at, MESSAGES.image);
    }
}

/**
 * True when `at` is under a type-it block. Provenance: INIT-010/SPEC-002
 * @param {object} plan
 * @param {string} at
 * @returns {boolean}
 */
function typeItAt(plan, at) {
    const match = /^\/units\/(\d+)\/lessons\/(\d+)\/blocks\/(\d+)/.exec(at);
    if (!match) return false;
    const block = blockAt(plan, Number(match[1]), Number(match[2]), Number(match[3]));
    return Boolean(block) && block.type === 'type-it';
}

function introWordAt(plan, at) {
    const match = INTRO_WORD.exec(at);
    if (!match) return false;
    const block = blockAt(plan, Number(match[1]), Number(match[2]), Number(match[3]));
    return Boolean(block) && block.type === 'introduction';
}

function verseSourceAt(plan, at) {
    const match = VERSE_SOURCE.exec(at);
    if (!match) return false;
    const block = blockAt(plan, Number(match[1]), Number(match[2]), Number(match[3]));
    return Boolean(block) && block.type === 'verse-reading';
}

/** The `cell` value at a conjugation-table cell pointer, or undefined. */
function cellAt(plan, at) {
    const match = CELL_AT.exec(at);
    if (!match) return undefined;
    const block = blockAt(plan, Number(match[1]), Number(match[2]), Number(match[3]));
    const row = block ? list(block.cells)[Number(match[4])] : undefined;
    return isObject(row) ? row.cell : undefined;
}

// --- lesson rules ------------------------------------------------------------

/**
 * @returns {boolean} true when the plan is within the N9 plan size
 */
function checkSizes(lessons, planBytes, findings) {
    const rule = RULES.N9;
    if (planBytes > rule.planBytes) {
        findings.add('N9', '', 'the plan is ' + planBytes + ' bytes of JSON; N9 allows ' + rule.planBytes);
    }
    lessons.forEach((entry) => {
        const lessonBytes = jsonBytes(entry.lesson);
        if (lessonBytes > rule.lessonBytes) {
            findings.add('N9', entry.at, 'the lesson is ' + lessonBytes + ' bytes of JSON; N9 allows ' + rule.lessonBytes);
        }
        entry.blocks.forEach((item) => {
            const blockBytes = jsonBytes(item.raw);
            if (blockBytes > rule.blockBytes) {
                findings.add('N9', item.at, 'the block is ' + blockBytes + ' bytes of JSON; N9 allows ' + rule.blockBytes);
            }
        });
    });
    return planBytes <= rule.planBytes;
}

function checkLessonRules(plan, lessons, context, findings) {
    const legacy = isObject(plan.migration) && plan.migration.legacy === true;
    lessons.forEach((entry) => {
        checkNewWordCap(entry, legacy, findings);
        checkRecycles(entry, findings);
        checkIntroductions(entry, findings);
        checkFluency(entry, findings);
        checkMeanings(entry, findings);
        checkPoints(entry, findings);
        checkAnswers(entry, findings);
        checkBlockInforms(entry, findings);
        checkPictureOnRecall(entry, findings);
        checkClosedSet(entry, findings);
        if (context.v120) checkExerciseBlocks(entry, findings);
        if (!context.v110) return;
        checkConjugationTables(entry, context, findings);
        checkExerciseCount(entry, findings);
        checkReviewFirst(entry, findings);
        checkPersonalQuestion(entry, findings);
        checkVerseSources(entry, findings);
    });
    checkUnits(plan, context, findings);
    checkCheckpoints(lessons, findings);
    if (context.v110) checkRecipeRuns(lessons, findings);
    if (plan.audience === 'personal') {
        findings.add('queue-order', '', 'a personal queue may insert due reviews; it keeps this plan\'s lesson order');
    }
}

/**
 * Warn when a type-it block uses a picture as the cue (ADR-0013, RSCH-005).
 * Provenance: INIT-010/SPEC-002
 * @param {object} entry
 * @param {object} findings
 */
function checkPictureOnRecall(entry, findings) {
    entry.blocks.forEach((item) => {
        if (item.type !== 'type-it') return;
        if (!typeItHasPictureCue(item.block)) return;
        findings.add('picture-on-recall', item.at,
            'a Type it cue is a picture; hide the picture on recall');
    });
}

/**
 * @param {object} block
 * @returns {boolean}
 */
function typeItHasPictureCue(block) {
    if (!isObject(block)) return false;
    if (has(block, 'image') && block.image) return true;
    if (has(block, 'picture') && block.picture) return true;
    if (typeof block.mediaId === 'string' && block.mediaId.length > 0) return true;
    return false;
}

/**
 * Warn when every new word in a lesson is in the kinship set.
 * Provenance: INIT-010/SPEC-002
 * @param {object} entry
 * @param {object} findings
 */
function checkClosedSet(entry, findings) {
    if (!entry.newWords.length) return;
    const allKinship = entry.newWords.every((word) => KINSHIP_STRIPPED.has(strippedKey(word.raw)));
    if (!allKinship) return;
    findings.add('closed-set', entry.at + '/newWords',
        'every new word is a kinship term; mix people, places, and actions');
}

function checkNewWordCap(entry, legacy, findings) {
    const count = entry.newWords.length + entry.verseWords.length;
    if (entry.recipe === RULES.N18.recipe) {
        checkTopicConversation(entry, count, legacy, findings);
        return;
    }
    const rule = RULES.N1;
    if (count <= rule.maxNewWords) return;
    findings.add('N1', entry.at + '/newWords',
        count + ' new words; N1 allows ' + rule.maxNewWords + (legacy ? ' (migration.legacy: warn only)' : ''),
        legacy ? rule.legacySeverity : rule.severity);
}

/** N18 replaces N1's figures for this recipe, and keeps N1's migration.legacy warn. */
function checkTopicConversation(entry, count, legacy, findings) {
    const rule = RULES.N18;
    if (count > rule.blockAbove) {
        findings.add('N18', entry.at + '/newWords', count + ' new words; a ' + rule.recipe + ' lesson blocks above '
            + rule.blockAbove + (legacy ? ' (migration.legacy: warn only)' : ''),
            legacy ? rule.legacySeverity : rule.blockSeverity);
    } else if (count > rule.warnAbove) {
        findings.add('N18', entry.at + '/newWords',
            count + ' new words; a ' + rule.recipe + ' lesson warns above ' + rule.warnAbove);
    }
    entry.blocks.forEach((item) => {
        if (item.type !== 'introduction') return;
        const sizes = new Map();
        list(item.block.words).forEach((word, w) => {
            if (!isObject(word) || !Number.isInteger(word.group)) return;
            const size = (sizes.get(word.group) || 0) + 1;
            sizes.set(word.group, size);
            if (size === rule.maxGroupWords + 1) {
                findings.add('N18', item.at + '/words/' + w, 'introduction group ' + word.group
                    + ' has more than ' + rule.maxGroupWords + ' words');
            }
        });
    });
}

function checkRecycles(entry, findings) {
    const rule = RULES.N2;
    if (rule.exemptRecipes.includes(entry.recipe)) return;
    if (!list(entry.lesson.buildsOn).length || entry.recycled.length >= rule.minRecycles) return;
    findings.add('N2', entry.at + '/recycles',
        'buildsOn is set and ' + entry.recycled.length + ' words are recycled; N2 asks for ' + rule.minRecycles);
}

function checkIntroductions(entry, findings) {
    entry.blocks.filter((item) => item.type === 'introduction').slice(1).forEach((item) => {
        findings.add('N5', item.at, isObject(item.block.pattern)
            ? 'a second introduction, with a second pattern; N5 allows one of each'
            : 'a second introduction; N5 allows one per lesson');
    });
}

function checkFluency(entry, findings) {
    entry.blocks.forEach((item) => {
        if (item.type !== 'fluency-pass') return;
        ['words', 'newWords'].forEach((key) => {
            if (has(item.block, key)) findings.add('N12', item.at + '/' + key, MESSAGES.fluencyList);
        });
        if (entry.newWords.length) {
            findings.add('N12', item.at, 'a fluency-pass in a lesson that introduces new words',
                RULES.N12.sharedLessonSeverity);
        }
    });
}

function checkMeanings(entry, findings) {
    const glossed = new Set();
    entry.blocks.forEach((item) => {
        if (item.type !== 'introduction') return;
        list(item.block.words).forEach((word) => {
            if (isObject(word) && typeof word.word === 'string'
                && typeof word.meaning === 'string' && word.meaning.trim()) {
                glossed.add(exactKey(word.word));
            }
        });
    });
    entry.newWords.forEach((word) => {
        if (!glossed.has(word.key)) {
            findings.add('meaning', word.at, 'no introduction word gives this new word a first-language meaning');
        }
    });
}

function checkPoints(entry, findings) {
    entry.newWords.concat(entry.verseWords).forEach((word) => {
        if (!HEBREW_POINT.test(word.raw)) findings.add('pointed-word', word.at, 'the new word has no Hebrew vowel point');
    });
}

/** N20–N22 on exercise blocks (INIT-012/SPEC-002). */
function checkExerciseBlocks(entry, findings) {
    const audioPath = loadSchema().audioPath;
    entry.blocks.forEach((item) => {
        if (item.type !== 'exercise') return;
        const block = item.block;
        const kind = block.kind;
        list(block.items).forEach((row, i) => {
            if (!isObject(row)) return;
            const at = item.at + '/items/' + i;
            if (kind === 'choose') {
                if (typeof row.answer === 'string' && Array.isArray(row.choices)
                    && !row.choices.some((choice) => choice === row.answer)) {
                    findings.add('N20', at + '/answer', 'the answer is not one of this item\'s choices');
                }
            } else if (kind === 'unscramble') {
                const words = list(row.words).filter((token) => typeof token === 'string');
                if (words.length < 3 || words.length > 10) {
                    findings.add('N21', at + '/words', 'unscramble needs 3 to 10 words');
                }
                list(row.alt).forEach((order, a) => {
                    if (!Array.isArray(order)) return;
                    const altAt = at + '/alt/' + a;
                    const altWords = order.filter((token) => typeof token === 'string');
                    if (altWords.length < 3 || altWords.length > 10) {
                        findings.add('N21', altAt, 'each alternate order needs 3 to 10 words');
                        return;
                    }
                    if (!sameWordMultiset(words, altWords)) {
                        findings.add('N21', altAt, 'this order is not a rearrangement of the same words');
                    } else if (sameWordOrder(words, altWords)) {
                        findings.add('N21', altAt, 'an alternate order must not match the main word order');
                    }
                });
            } else if (kind === 'dictation') {
                if (typeof row.audioPath !== 'string' || !row.audioPath.length) {
                    findings.add('N22', at + '/audioPath', 'dictation needs an audio path');
                } else if (!audioPath.test(row.audioPath)) {
                    findings.add('N22', at + '/audioPath',
                        'audio path must be a relative .mp3 path with no scheme or leading slash');
                }
            }
            list(row.accept).forEach((alt, a) => {
                if (typeof alt === 'string' && HEBREW_POINT.test(alt)) {
                    findings.add('schema', at + '/accept/' + a, 'accept must not include vowel points or cantillation');
                }
            });
        });
    });
}

/** N23: topic-conversation with more than two exercise blocks warns. */
function checkExerciseCount(entry, findings) {
    const rule = RULES.N23;
    if (entry.recipe !== rule.recipe) return;
    const count = entry.blocks.filter((item) => item.type === 'exercise').length;
    if (count > rule.maxExerciseBlocks) {
        findings.add('N23', entry.at + '/blocks',
            count + ' exercise blocks; a ' + rule.recipe + ' lesson warns above ' + rule.maxExerciseBlocks);
    }
}

function sameWordOrder(left, right) {
    return left.length === right.length && left.every((word, index) => word === right[index]);
}

function sameWordMultiset(left, right) {
    if (left.length !== right.length) return false;
    const tally = (words) => {
        const map = new Map();
        words.forEach((word) => map.set(word, (map.get(word) || 0) + 1));
        return map;
    };
    const a = tally(left);
    const b = tally(right);
    if (a.size !== b.size) return false;
    for (const [word, count] of a) {
        if (b.get(word) !== count) return false;
    }
    return true;
}

function checkAnswers(entry, findings) {
    entry.blocks.forEach((item) => {
        if (item.type !== 'introduction') return;
        const cloze = isObject(item.block.cloze) ? item.block.cloze : {};
        list(cloze.items).forEach((clozeItem, i) => {
            if (isObject(clozeItem) && typeof clozeItem.answer === 'string' && !HEBREW_LETTER.test(clozeItem.answer)) {
                findings.add('romanized-answer', item.at + '/cloze/items/' + i + '/answer', MESSAGES.romanized);
            }
        });
        const pattern = isObject(item.block.pattern) ? item.block.pattern : {};
        list(pattern.items).forEach((patternItem, i) => {
            if (!isObject(patternItem) || !Number.isInteger(patternItem.answer) || !Array.isArray(patternItem.choices)) {
                return;
            }
            const at = item.at + '/pattern/items/' + i;
            const answer = patternItem.answer;
            if (answer < 0 || answer >= patternItem.choices.length) {
                findings.add('romanized-answer', at + '/answer', 'the answer index names no choice, so there is no Hebrew answer');
            } else if (typeof patternItem.choices[answer] === 'string' && !HEBREW_LETTER.test(patternItem.choices[answer])) {
                findings.add('romanized-answer', at + '/choices/' + answer, MESSAGES.romanized);
            }
        });
    });
}

function checkBlockInforms(entry, findings) {
    entry.blocks.forEach((item) => {
        if ((item.type === 'graded-story' || item.type === 'listening') && list(item.block.questions).length) {
            findings.add('questions', item.at + '/questions', 'comprehension questions are optional and are not the recall event');
        }
        if (item.type === 'spaced-review-checkpoint') {
            findings.add('recall-day', item.at, 'the scheduler places this recall on a later day; the plan holds no dates');
        }
    });
}

/**
 * N13 and N14. The schema has already checked each cell code against the
 * table's tense; this checks repeats and asks the lexicon about each form.
 */
function checkConjugationTables(entry, context, findings) {
    entry.blocks.forEach((item) => {
        if (item.type !== 'conjugation-table') return;
        const table = item.block;
        const lemma = typeof table.lemmaKey === 'string' ? exactKey(table.lemmaKey) : null;
        const cells = new Set();
        list(table.cells).forEach((row, i) => {
            if (!isObject(row)) return;
            const at = item.at + '/cells/' + i;
            if (typeof row.cell === 'string') {
                if (cells.has(row.cell)) findings.add('N13', at + '/cell', MESSAGES.cellTwice);
                cells.add(row.cell);
            }
            checkFormKey(row, table, lemma, at + '/formKey', context, findings);
        });
    });
}

/** A key the schema already refuses is not looked up. */
function checkFormKey(row, table, lemma, at, context, findings) {
    const key = row.formKey;
    if (typeof key !== 'string' || codePoints(key) > context.keyLimit || !HEBREW_LETTER.test(key)) return;
    if (!context.lexicon.available) {
        findings.add('N13', at, MESSAGES.noLexicon);
        return;
    }
    const form = context.lexicon.find(key);
    if (!form || form.kind !== 'form') {
        findings.add('N13', at, MESSAGES.noForm);
        return;
    }
    if (form.verified !== 1 && form.verified !== true) findings.add('N14', at, MESSAGES.unverified);
    if (form.lemma_key !== lemma) findings.add('N13', at, MESSAGES.otherLemma);
    else if (form.tense !== table.tense) findings.add('N13', at, MESSAGES.otherTense);
    else if (form.cell !== row.cell) findings.add('N13', at, MESSAGES.otherCell);
}

function checkReviewFirst(entry, findings) {
    const rule = RULES.N15;
    if (!entry.recipe || rule.exemptRecipes.includes(entry.recipe) || !list(entry.lesson.buildsOn).length) return;
    const first = entry.blocks[0];
    if (first && first.type === rule.firstBlock) return;
    findings.add('N15', first ? first.at : entry.at + '/blocks',
        'a lesson that builds on earlier lessons starts with a ' + rule.firstBlock);
}

function checkPersonalQuestion(entry, findings) {
    if (!RULES.N16.recipes.includes(entry.recipe)) return;
    const asked = entry.blocks.some((item) => item.type === 'personal-question'
        || item.type === 'assignment'
        || (entry.recipe === 'verse-reading' && item.type === 'verse-reading' && isObject(item.block.personalQuestion)));
    if (!asked) findings.add('N16', entry.at + '/blocks', 'a ' + entry.recipe + ' lesson has no personal question');
}

function checkVerseSources(entry, findings) {
    entry.blocks.forEach((item) => {
        if (item.type !== 'verse-reading') return;
        const ref = item.block.ref;
        const source = isObject(ref) ? ref.source : undefined;
        if (typeof source !== 'string' || !source.trim()) {
            findings.add('N19', item.at + '/ref/source', MESSAGES.verseSource);
        }
    });
}

/** N17, once per run, at the lesson that makes the run long enough. */
function checkRecipeRuns(lessons, findings) {
    const rule = RULES.N17;
    let unit = null;
    let recipe;
    let run = 0;
    lessons.forEach((entry) => {
        if (entry.u !== unit) {
            unit = entry.u;
            recipe = undefined;
            run = 0;
        }
        run = entry.recipe && entry.recipe === recipe ? run + 1 : 1;
        recipe = entry.recipe;
        if (recipe && run === rule.runLength) {
            findings.add('N17', entry.at + '/recipe',
                rule.runLength + ' lessons in a row in this unit have the recipe ' + recipe);
        }
    });
}

function checkUnits(plan, context, findings) {
    const rule = RULES.N7;
    const focused = context.v110 ? rule.languageFocused : ['introduction'];
    list(plan.units).forEach((unit, u) => {
        if (!isObject(unit)) return;
        let introductions = 0;
        let blocks = 0;
        list(unit.lessons).forEach((lesson) => {
            if (!isObject(lesson) || rule.excludedRecipes.includes(recipeOf(lesson, context))) return;
            list(lesson.blocks).forEach((block) => {
                blocks += 1;
                if (isObject(block) && focused.includes(block.type)) introductions += 1;
            });
        });
        if (introductions * rule.blocksPerIntroduction > blocks) {
            findings.add('N7', '/units/' + u, introductions + ' of ' + blocks + ' blocks are '
                + (context.v110 ? 'language-focused' : 'introductions')
                + '; N7 allows one in ' + rule.blocksPerIntroduction);
        }
        if (typeof unit.frequencyBand === 'string' && unit.frequencyBand.trim()) {
            findings.add('frequency-band', '/units/' + u + '/frequencyBand', 'frequencyBand is recorded and not checked');
        }
    });
}

/** Checkpoint word keys are review keys, so they match by ADR-0001 exactKey only. */
function checkCheckpoints(lessons, findings) {
    if (lessons.length < 2) return;
    const lastListed = new Map();
    lessons.forEach((entry) => {
        entry.blocks.forEach((item) => {
            if (item.type !== 'spaced-review-checkpoint') return;
            list(item.block.wordKeys).forEach((key) => {
                if (typeof key === 'string') lastListed.set(exactKey(key), entry.index);
            });
        });
    });
    lessons.forEach((entry) => {
        entry.newWords.forEach((word) => {
            if (!(lastListed.get(word.key) > entry.index)) {
                findings.add('later-checkpoint', word.at, 'no later spaced-review-checkpoint lists this word');
            }
        });
    });
}

// --- text estimates ------------------------------------------------------------

/**
 * N3, N6, later-context, and pattern-focus read running text. Words match as
 * token runs: by exact key, or by stripped key when exactly one word in the
 * plan has it (ADR-0001). Prefixes are never stripped.
 */
function checkEstimates(lessons, context, findings) {
    lessons.forEach((entry) => checkCoverage(entry, findings));
    const words = [];
    lessons.forEach((entry) => {
        entry.newWords.forEach((word) => words.push(word.raw));
        entry.verseWords.forEach((word) => words.push(word.raw));
        entry.recycled.forEach((word) => words.push(word.raw));
        entry.focus.forEach((focus) => words.push(focus.raw));
    });
    const find = occurrences(textUnits(lessons, context), catalog(words));
    const multiLesson = lessons.length > 1;
    lessons.forEach((entry) => {
        entry.newWords.forEach((word) => {
            const hits = find(word.raw);
            const here = countOccurrences(hits, word, entry, findings);
            if (!multiLesson) return;
            const used = new Set(here.map(({ unit }) => unit.sentence));
            if (!hits.some(({ unit }) => unit.lesson > entry.index && !used.has(unit.sentence))) {
                findings.add('later-context', word.at, 'no later lesson uses this word in a different sentence');
            }
        });
        entry.verseWords.forEach((word) => countOccurrences(find(word.raw), word, entry, findings));
        entry.focus.forEach((focus) => {
            const seen = find(focus.raw).some(({ unit }) => (unit.kind === 'story' || unit.kind === 'dialogue')
                && (unit.lesson === entry.index || (unit.lesson > entry.index && unit.unit === entry.u)));
            if (!seen) {
                findings.add('pattern-focus', focus.at,
                    'the pattern focus is in no story or dialogue in this lesson or later in the unit');
            }
        });
    });
}

/**
 * N6 for one new word.
 * @returns {{ unit: object, count: number }[]} the hits in the word's own lesson
 */
function countOccurrences(hits, word, entry, findings) {
    const here = hits.filter(({ unit }) => unit.lesson === entry.index);
    const count = here.reduce((sum, hit) => sum + hit.count, 0);
    if (count < RULES.N6.minOccurrences) {
        findings.add('N6', word.at, 'appears ' + count + ' time(s) in this lesson; N6 asks for '
            + RULES.N6.minOccurrences);
    }
    return here;
}

/**
 * N3 per passage. The lexicon is the lesson's newWords, recycles, and the
 * closed function list; a token listed in glosses, support, or the
 * paragraph's spans is treated as known for this estimate only.
 */
function checkCoverage(entry, findings) {
    const rule = RULES.N3;
    const words = catalog(entry.newWords.map((word) => word.raw)
        .concat(entry.recycled.map((word) => word.raw), FUNCTION_WORDS));
    entry.blocks.forEach(({ block, type, at }) => {
        let known = 0;
        let total = 0;
        if (type === 'graded-story') {
            const shared = treatedTokens((isObject(block.glosses) ? Object.keys(block.glosses) : [])
                .concat(list(block.support).map((pair) => (isObject(pair) ? pair.he : undefined))));
            list(block.paragraphs).forEach((paragraph) => {
                if (!isObject(paragraph)) return;
                const own = treatedTokens(list(paragraph.spans).map((span) => (isObject(span) ? span.text : undefined)));
                const counted = coverage(paragraph.text, words, (token) => shared.has(token) || own.has(token));
                known += counted.known;
                total += counted.total;
            });
        } else if (type === 'listening') {
            const counted = coverage(block.passage, words, () => false);
            known = counted.known;
            total = counted.total;
        } else {
            return;
        }
        if (total && known * 100 < rule.minKnownPercent * total) {
            findings.add('N3', at, 'about ' + Math.floor((known * 100) / total)
                + '% of running words are known; N3 asks for ' + rule.minKnownPercent + '%');
        }
    });
}

/** Stripped keys of every token in `values`, for the treated-as-known test. */
function treatedTokens(values) {
    const tokens = new Set();
    values.forEach((value) => {
        if (typeof value === 'string') tokenize(value).forEach((token) => tokens.add(strippedKey(token)));
    });
    return tokens;
}

function coverage(value, words, treated) {
    const exact = typeof value === 'string' ? tokenize(value) : [];
    const unit = { exact, stripped: exact.map(strippedKey) };
    let known = 0;
    let i = 0;
    while (i < exact.length) {
        let longest = 0;
        walk(words, unit, i, (entry, length) => {
            longest = length;
        });
        if (longest) {
            known += longest;
            i += longest;
        } else {
            if (treated(unit.stripped[i])) known += 1;
            i += 1;
        }
    }
    return { known, total: exact.length };
}

/**
 * Occurrence contexts N6 names: example sentence, story, listening, dialogue,
 * cloze, and in 1.1.0 a verse and its questions.
 */
function textUnits(lessons, context) {
    const units = [];
    const add = (entry, kind, value) => {
        if (typeof value !== 'string') return;
        const exact = tokenize(value);
        if (!exact.length) return;
        const stripped = exact.map(strippedKey);
        units.push({ lesson: entry.index, unit: entry.u, kind, exact, stripped, sentence: stripped.join(' ') });
    };
    lessons.forEach((entry) => {
        entry.blocks.forEach(({ block, type }) => {
            if (type === 'introduction') {
                list(block.words).forEach((word) => {
                    if (isObject(word)) add(entry, 'example', word.sentence);
                });
                const cloze = isObject(block.cloze) ? block.cloze : {};
                list(cloze.items).forEach((item) => {
                    if (!isObject(item)) return;
                    add(entry, 'cloze', [item.textPre, item.answer, item.textPost]
                        .filter((part) => typeof part === 'string').join(' '));
                });
            } else if (type === 'graded-story') {
                list(block.paragraphs).forEach((paragraph) => {
                    if (isObject(paragraph)) add(entry, 'story', paragraph.text);
                });
            } else if (type === 'listening') {
                add(entry, 'listening', block.passage);
            } else if (type === 'short-dialogue') {
                list(block.lines).forEach((line) => {
                    if (!isObject(line)) return;
                    add(entry, 'dialogue', line.pointed);
                    add(entry, 'dialogue', line.followUp);
                });
            } else if (context.v110 && type === 'verse-reading') {
                add(entry, 'verse', block.verse);
                list(block.questions).forEach((question) => {
                    if (isObject(question)) add(entry, 'verse', question.he);
                });
            }
        });
    });
    return units;
}

/**
 * A word list keyed for ADR-0001 identity, as two token tries: stripped and
 * exact. A stripped path that ends exactly one entry is unique, so a stripped
 * match can only mean that entry; otherwise only the exact path matches.
 * Matching walks the tries once per text position, so its cost does not grow
 * with the number of entries.
 */
function catalog(words) {
    const byExact = new Map();
    const loose = trieNode();
    const strict = trieNode();
    words.forEach((raw) => {
        if (typeof raw !== 'string') return;
        const exact = tokenize(raw);
        const key = exact.join(' ');
        if (!exact.length || byExact.has(key)) return;
        const entry = { exact };
        byExact.set(key, entry);
        const stripped = exact.map(strippedKey);
        const end = descend(loose, stripped);
        end.entries.push(entry);
        end.blank = stripped.every((token) => token === '');
        descend(strict, exact).entry = entry;
    });
    return { byExact, loose, strict };
}

function trieNode() {
    return { next: new Map(), entries: [], blank: false, entry: null };
}

function descend(node, tokens) {
    let at = node;
    tokens.forEach((token) => {
        if (!at.next.has(token)) at.next.set(token, trieNode());
        at = at.next.get(token);
    });
    return at;
}

/** Calls `onHit(entry, tokenCount)` for every entry that starts at `start`, shortest first. */
function walk(words, unit, start, onHit) {
    let loose = words.loose;
    let strict = words.strict;
    for (let i = start; i < unit.exact.length; i += 1) {
        loose = loose.next.get(unit.stripped[i]);
        if (!loose) return;
        strict = strict ? strict.next.get(unit.exact[i]) : undefined;
        const entry = loose.entries.length === 1 && !loose.blank ? loose.entries[0] : strict && strict.entry;
        if (entry) onHit(entry, i - start + 1);
    }
}

/** One pass over the text: for each entry, the units it occurs in and how often. */
function occurrences(units, words) {
    const found = new Map();
    units.forEach((unit, u) => {
        for (let start = 0; start < unit.exact.length; start += 1) {
            walk(words, unit, start, (entry) => {
                if (!found.has(entry)) found.set(entry, new Map());
                const perUnit = found.get(entry);
                perUnit.set(u, (perUnit.get(u) || 0) + 1);
            });
        }
    });
    return (raw) => {
        const entry = words.byExact.get(tokenize(raw).join(' '));
        const perUnit = entry ? found.get(entry) : undefined;
        return perUnit ? Array.from(perUnit, ([u, count]) => ({ unit: units[u], count })) : [];
    };
}

// --- shared helpers ------------------------------------------------------------

function lessonsOf(plan, context) {
    const lessons = [];
    list(plan.units).forEach((unit, u) => {
        if (!isObject(unit)) return;
        list(unit.lessons).forEach((lesson, l) => {
            if (!isObject(lesson)) return;
            const at = '/units/' + u + '/lessons/' + l;
            const blocks = list(lesson.blocks).map((block, b) => ({
                raw: block,
                block: isObject(block) ? block : {},
                type: isObject(block) ? block.type : undefined,
                at: at + '/blocks/' + b
            }));
            const newWords = declared(list(lesson.newWords), at + '/newWords');
            lessons.push({
                lesson,
                u,
                at,
                index: lessons.length,
                blocks,
                recipe: recipeOf(lesson, context),
                newWords,
                verseWords: context.v110 ? verseWordsOf(blocks, newWords) : [],
                recycled: declared(list(lesson.recycles).map((item) => (isObject(item) ? item.word : undefined)),
                    at + '/recycles'),
                focus: focusOf(blocks)
            });
        });
    });
    return lessons;
}

/** A known recipe in a 1.1.0 document; otherwise none, and the schema reports the field. */
function recipeOf(lesson, context) {
    return context.v110 && context.recipes.has(lesson.recipe) ? lesson.recipe : undefined;
}

/**
 * verse-reading newWords that the lesson's newWords do not already list.
 * N1, N6, and pointed-word count them (ADR-0009 §7); meaning and the
 * later-lesson rules do not.
 */
function verseWordsOf(blocks, newWords) {
    const seen = new Set(newWords.map((word) => word.key));
    const words = [];
    blocks.forEach((item) => {
        if (item.type !== 'verse-reading') return;
        list(item.block.newWords).forEach((value, i) => {
            if (typeof value !== 'string') return;
            const key = exactKey(value);
            if (!key || seen.has(key)) return;
            seen.add(key);
            words.push({ key, raw: value, at: item.at + '/newWords/' + i });
        });
    });
    return words;
}

/** Distinct strings by ADR-0001 exactKey, each with the pointer of its first entry. */
function declared(values, at) {
    const seen = new Map();
    values.forEach((value, i) => {
        if (typeof value !== 'string') return;
        const key = exactKey(value);
        if (key && !seen.has(key)) seen.set(key, { key, raw: value, at: pointer(at, i) });
    });
    return Array.from(seen.values());
}

function focusOf(blocks) {
    const focus = [];
    blocks.forEach((item) => {
        if (item.type !== 'introduction' || !isObject(item.block.pattern)) return;
        list(item.block.pattern.rules).forEach((rule, r) => {
            if (!isObject(rule)) return;
            list(rule.focus).forEach((value, f) => {
                if (typeof value === 'string') focus.push({ raw: value, at: item.at + '/pattern/rules/' + r + '/focus/' + f });
            });
        });
    });
    return focus;
}

function blockAt(plan, u, l, b) {
    const unit = list(plan.units)[u];
    const lesson = isObject(unit) ? list(unit.lessons)[l] : undefined;
    const block = isObject(lesson) ? list(lesson.blocks)[b] : undefined;
    return isObject(block) ? block : null;
}

/** ADR-0001 exactKey: NFC, whitespace collapsed, trimmed. */
function exactKey(value) {
    return String(value).normalize('NFC').replace(/\s+/g, ' ').trim();
}

/** ADR-0001 strippedKey: exactKey without vowel points and cantillation. */
function strippedKey(value) {
    return exactKey(exactKey(value).replace(POINTS_AND_ACCENTS, ''));
}

/**
 * ADR-0003 §6 tokenizer: split on whitespace, then strip leading and trailing
 * punctuation, symbols, and invisible format marks. Prefixes stay on.
 * @param {string} value
 * @returns {string[]}
 */
function tokenize(value) {
    const key = exactKey(value);
    if (!key) return [];
    return key.split(' ').map(trimEdges).filter(Boolean);
}

/** Scans code points from both ends: a trailing-anchored regex here backtracks quadratically. */
function trimEdges(token) {
    const chars = Array.from(token);
    let start = 0;
    let end = chars.length;
    while (start < end && EDGE_MARK.test(chars[start])) start += 1;
    while (end > start && EDGE_MARK.test(chars[end - 1])) end -= 1;
    return start === 0 && end === chars.length ? token : chars.slice(start, end).join('');
}

/** JSON Schema string length: Unicode code points, not UTF-16 units. */
function codePoints(value) {
    let count = 0;
    for (let i = 0; i < value.length; i += 1) {
        const unit = value.charCodeAt(i);
        if (unit >= 0xd800 && unit <= 0xdbff && i + 1 < value.length) {
            const next = value.charCodeAt(i + 1);
            if (next >= 0xdc00 && next <= 0xdfff) i += 1;
        }
        count += 1;
    }
    return count;
}

function jsonBytes(value) {
    return Buffer.byteLength(JSON.stringify(value === undefined ? null : value), 'utf8');
}

/** RFC 6901 pointer to `key` under `base`. */
function pointer(base, key) {
    return base + '/' + String(key).replace(/~/g, '~0').replace(/\//g, '~1');
}

function isObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function list(value) {
    return Array.isArray(value) ? value : [];
}

function has(object, key) {
    return Object.prototype.hasOwnProperty.call(object, key);
}

function deepFreeze(value) {
    Object.values(value).forEach((child) => {
        if (child && typeof child === 'object') deepFreeze(child);
    });
    return Object.freeze(value);
}

module.exports = {
    FUNCTION_WORDS,
    RULES,
    SCHEMA_PATH,
    V100,
    V110,
    V120,
    exactKey,
    schemaText,
    strippedKey,
    tokenize,
    usesV110,
    usesV120,
    validate: validatePlan,
    validatePlan
};
