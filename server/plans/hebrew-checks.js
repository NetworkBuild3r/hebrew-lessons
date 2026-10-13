/**
 * Five Hebrew and consistency checks for a lesson plan, reported as warnings
 * with stable codes (INIT-008 D3, ADR-0011 Decision 5). They are advice for the
 * AI and the admin; none of them blocks a write or a publish.
 *
 *   pointing-form        malformed vowel pointing, or text that is not NFC
 *   field-language       Hebrew in an English or Latin field; none in a Hebrew field
 *   lexicon-form         a word pointed or glossed differently from its lexicon entry
 *   pointing-consistency one word pointed two ways in one lesson
 *   audio-stale          a text field changed since publish while its clip path stayed
 *
 * What a program can and cannot judge (RSCH-009 §1.4): form, language, and
 * consistency are mechanical. Whether a form is correct in context is not; the
 * lexicon `verified` flag, set by a person, is the only authority here.
 *
 * A pure function of a plan, an optional lexicon, and an optional snapshot.
 * Its only I/O is reading the plan schema once. No network, no model.
 * Provenance: INIT-008/SPEC-008
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { actionFor, toIdPath } = require('./id-path');

const SCHEMA_PATH = path.join(
    __dirname, '..', '..', 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json'
);
const DEFINITIONS = '#/definitions/';

// --- Hebrew text helpers (the same definitions as validate.js; a test pins them) ---

/** ADR-0001 strip range, the one validate.js uses. */
const POINTS_AND_ACCENTS = /[\u0591-\u05C7]/g;
/** validate.js HEBREW_POINT: any vowel point, dagesh, meteg, rafe, or shin and sin dot. */
const HEBREW_POINT = /[\u05B0-\u05BD\u05BF\u05C1\u05C2\u05C7]/;
const HEBREW_LETTER = /[\u05D0-\u05EA]/;

/**
 * Allowed pointing on one letter (ledger aud-2). Written from the Unicode
 * ranges above; a person who reads Hebrew confirms it. Advisory only.
 *   vowel                 U+05B0-U+05BB and U+05C7 (sheva through qubuts, qamats qatan).
 *                         At most one per letter.
 *   dagesh or mapiq       U+05BC      allowed, never counted as the vowel
 *   shin or sin dot       U+05C1, U+05C2
 *   rafe                  U+05BF
 *   meteg                 U+05BD
 *   cantillation          U+0591-U+05AF
 *   upper and lower dots  U+05C4, U+05C5
 * Maqaf, paseq, sof pasuq, and nun hafukha (U+05BE, U+05C0, U+05C3, U+05C6)
 * are punctuation: they end a letter's marks like a space does.
 */
const POINT_CLASSES = Object.freeze({
    vowel: /[\u05B0-\u05BB\u05C7]/,
    combining: /[\u0591-\u05BD\u05BF\u05C1\u05C2\u05C4\u05C5\u05C7]/
});

function exactKey(value) {
    return String(value).normalize('NFC').replace(/\s+/g, ' ').trim();
}

function strippedKey(value) {
    return exactKey(exactKey(value).replace(POINTS_AND_ACCENTS, ''));
}

const EDGE_MARK = /^[\p{P}\p{S}\p{Cf}]$/u;

/** Trim punctuation, symbols, and format marks from both ends of a token. */
function trimEdges(token) {
    const chars = Array.from(token);
    let start = 0;
    let end = chars.length;
    while (start < end && EDGE_MARK.test(chars[start])) start += 1;
    while (end > start && EDGE_MARK.test(chars[end - 1])) end -= 1;
    return start === 0 && end === chars.length ? token : chars.slice(start, end).join('');
}

/** A field value as whitespace-separated tokens without edge punctuation, NFC. */
function formOf(value) {
    return exactKey(value).split(' ').map(trimEdges).filter(Boolean).join(' ');
}

function isObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function list(value) {
    return Array.isArray(value) ? value : [];
}

function pointer(base, key) {
    return base + '/' + String(key).replace(/~/g, '~0').replace(/\//g, '~1');
}

// --- registries -------------------------------------------------------------------

/**
 * Language of every free-text string property of the plan schema, keyed
 * `definition.property` (`plan` is the schema root). `he` Hebrew, `en` English,
 * `latin` a transliteration or sound spelling, `mixed` either or both,
 * `exempt` not language text (ids, titles, paths, patterns). A `*.property` key
 * covers that property on every definition. A string property that is an enum
 * or const is a closed vocabulary and needs no row.
 *
 * A property is `mixed` where the existing plans put English notes beside
 * Hebrew words (a pattern `english` field quotes the Hebrew it explains), so
 * calling it English would warn on valid work.
 * Label for the rule rows: product choice (RSCH-009 D8); the field list is adapted
 * from ADR-0003 and the three imported plans.
 * A test fails when the schema holds a string property in neither this table nor
 * the enum rule, so a new field cannot skip the checks.
 */
const FIELD_LANGUAGE = Object.freeze({
    // identifiers, titles, and paths
    '*.id': 'exempt',
    'plan.assigneeId': 'exempt',
    'plan.title': 'exempt',
    'plan.subtitle': 'exempt',
    'migration.sourceWorksheet': 'exempt',
    'migration.activityOrder': 'exempt',
    'migration.retiredActivityTypes': 'exempt',
    'migration.retiredFields': 'exempt',
    'unit.title': 'exempt',
    'unit.theme': 'exempt',
    'unit.frequencyBand': 'exempt',
    'lesson.title': 'exempt',
    'lesson.subtitle': 'exempt',
    'lesson.footer': 'exempt',
    'lesson.legacyStorageKey': 'exempt',
    'lesson.buildsOn': 'exempt',
    'recycle.fromLessonId': 'exempt',
    'wordEntry.fromLessonId': 'exempt',
    'imageRef.mediaId': 'exempt',
    'wordEntry.audioPath': 'exempt',
    'wordEntry.sentenceAudioPath': 'exempt',
    'patternItem.audioPath': 'exempt',
    'paragraph.audioPath': 'exempt',
    'question.audioPath': 'exempt',
    'line.audioPath': 'exempt',
    'listeningBlock.audioPath': 'exempt',
    'checkpointBlock.fromLessons': 'exempt',
    'patternRule.title': 'exempt',
    'explainerExample.root': 'exempt',
    // Hebrew
    'lesson.newWords': 'he',
    'recycle.word': 'he',
    'wordEntry.word': 'he',
    'wordEntry.sentence': 'he',
    'patternRule.example': 'he',
    'patternRule.focus': 'he',
    'patternItem.choices': 'he',
    'patternItem.pointed': 'he',
    'glossPair.he': 'he',
    'cloze.bank': 'he',
    'clozeItem.textPre': 'he',
    'clozeItem.textPost': 'he',
    'clozeItem.prefix': 'he',
    'clozeItem.answer': 'he',
    'span.text': 'he',
    'paragraph.text': 'he',
    'paragraph.highlights': 'he',
    'line.pointed': 'he',
    'line.youFeminine': 'he',
    'line.speak': 'he',
    'line.followUp': 'he',
    'listeningBlock.passage': 'he',
    'shortDialogueBlock.intro': 'he',
    'checkpointBlock.wordKeys': 'he',
    'hebrewLine.he': 'he',
    'explainerExample.he': 'he',
    'conjugationTableBlock.lemmaKey': 'he',
    'conjugationCell.formKey': 'he',
    'verseReadingBlock.newWords': 'he',
    'verseReadingBlock.verse': 'he',
    'versePersonalQuestion.he': 'he',
    'letterIntroBlock.letter': 'he',
    'letterSound.pointed': 'he',
    'letterFind.word': 'he',
    // English
    'wordEntry.meaning': 'en',
    'wordEntry.pos': 'en',
    'glossPair.en': 'en',
    'clozeItem.english': 'en',
    'hebrewLine.en': 'en',
    'explainerExample.en': 'en',
    'versePersonalQuestion.en': 'en',
    // Latin script: a sound spelling or a transliteration
    'wordEntry.sound': 'latin',
    'wordEntry.translit': 'latin',
    'explainerExample.translit': 'latin',
    'letterSound.sound': 'latin',
    // either: notes and prompts that mix English with the Hebrew they explain
    'imageRef.alt': 'mixed',
    'wordEntry.trap': 'mixed',
    'wordEntry.infinitive': 'mixed',
    'pattern.tabLabel': 'mixed',
    'pattern.heading': 'mixed',
    'pattern.lede': 'mixed',
    'pattern.prompt': 'mixed',
    'pattern.reminders': 'mixed',
    'patternRule.english': 'mixed',
    'patternItem.direct': 'mixed',
    'patternItem.why': 'mixed',
    'question.question': 'mixed',
    'question.sampleAnswer': 'mixed',
    'line.question': 'mixed',
    'line.gloss': 'mixed',
    'introductionBlock.heading': 'mixed',
    'introductionBlock.note': 'mixed',
    'gradedStoryBlock.notice': 'mixed',
    'gradedStoryBlock.glosses': 'mixed',
    'shortDialogueBlock.introEn': 'mixed',
    'fluencyPassBlock.cues': 'mixed',
    'explainerBlock.title': 'mixed',
    'explainerSection.heading': 'mixed',
    'explainerSection.paragraphs': 'mixed',
    'explainerTable.columns': 'mixed',
    'explainerTable.rows': 'mixed',
    'explainerCallout.title': 'mixed',
    'explainerCallout.text': 'mixed',
    'verseRef.book': 'mixed',
    'verseRef.source': 'mixed',
    'letterIntroBlock.name': 'mixed',
    'letterSound.vowel': 'mixed'
});

/**
 * Each text field and the clip it plays. Label: product choice. `RSCH-009`
 * found nothing in `server/` links a plan text field to its `audioPath` except
 * the path pattern, so this table is that link. A test fails when the schema
 * holds a property named `audioPath` or `sentenceAudioPath` with no row here.
 */
const AUDIO_PAIRS = Object.freeze([
    { owner: 'wordEntry', text: 'word', clip: 'audioPath' },
    { owner: 'wordEntry', text: 'sentence', clip: 'sentenceAudioPath' },
    { owner: 'patternItem', text: 'pointed', clip: 'audioPath' },
    { owner: 'paragraph', text: 'text', clip: 'audioPath' },
    { owner: 'question', text: 'question', clip: 'audioPath' },
    { owner: 'line', text: 'pointed', clip: 'audioPath' },
    { owner: 'listeningBlock', text: 'passage', clip: 'audioPath' }
].map((row) => Object.freeze(row)));

/** Property names that hold a clip path. */
const CLIP_PROPERTIES = Object.freeze(['audioPath', 'sentenceAudioPath']);

/** Word-level Hebrew fields compared by pointing-consistency. */
const WORD_LEVEL_FIELDS = Object.freeze([
    'wordEntry.word', 'letterFind.word', 'explainerExample.he', 'conjugationCell.formKey'
]);

/** The regenerate path, as read from the code on 2026-10-12 (ledger aud-4). */
const REGENERATE_NOTE = 'No tool generates audio for a plan field: generate_lesson_audio and '
    + 'server/audio.js work on legacy worksheet.js lessons only. Remove the stale clip path '
    + '(replace_part without it) so the old clip stops playing, or put a new .mp3 under the '
    + 'audio folder and set its path.';

function languageOf(owner, prop) {
    const own = owner + '.' + prop;
    if (Object.prototype.hasOwnProperty.call(FIELD_LANGUAGE, own)) return FIELD_LANGUAGE[own];
    const any = '*.' + prop;
    return Object.prototype.hasOwnProperty.call(FIELD_LANGUAGE, any) ? FIELD_LANGUAGE[any] : undefined;
}

// --- schema index -------------------------------------------------------------------

let schemaRoot = null;

function loadSchemaRoot() {
    if (!schemaRoot) schemaRoot = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
    return schemaRoot;
}

/**
 * Index a schema by definition: for each object definition, its properties
 * with a shape (`string`, `enum`, `array`, `map`, `object`, `oneOf`, `other`).
 * The root is the definition `plan`. Built from the schema, never from a list
 * of block names, so a new block type is walked as soon as the schema has it.
 * @param {object} root
 */
function buildIndex(root) {
    const defs = isObject(root.definitions) ? root.definitions : {};
    const objects = new Map();

    function resolve(node) {
        let name = null;
        let target = node;
        for (let hops = 0; isObject(target) && typeof target.$ref === 'string' && hops < 16; hops += 1) {
            name = target.$ref.startsWith(DEFINITIONS) ? target.$ref.slice(DEFINITIONS.length) : null;
            target = name ? defs[name] : undefined;
        }
        return { name, node: target };
    }

    function objectShape(name, node) {
        if (objects.has(name)) return objects.get(name);
        const shape = { kind: 'object', def: name, props: new Map() };
        objects.set(name, shape);
        Object.keys(node.properties).forEach((prop) => {
            shape.props.set(prop, shapeOf(node.properties[prop], name + '.' + prop));
        });
        return shape;
    }

    function shapeOf(node, fallback) {
        const { name, node: target } = resolve(node);
        if (!isObject(target)) return { kind: 'other' };
        if (isObject(target.properties)) return objectShape(name || fallback, target);
        if (Array.isArray(target.oneOf)) {
            const branches = new Map();
            target.oneOf.forEach((branch) => {
                const resolved = resolve(branch);
                const type = isObject(resolved.node) && isObject(resolved.node.properties)
                    && isObject(resolved.node.properties.type) ? resolved.node.properties.type.const : undefined;
                if (typeof type === 'string' && resolved.name) {
                    branches.set(type, objectShape(resolved.name, resolved.node));
                }
            });
            return { kind: 'oneOf', branches };
        }
        if (target.enum !== undefined || target.const !== undefined) return { kind: 'enum' };
        const types = [].concat(target.type || []);
        if (types.includes('string')) return { kind: 'string' };
        if (types.includes('array') && target.items) return { kind: 'array', items: shapeOf(target.items, fallback) };
        if (types.includes('object') && isObject(target.additionalProperties)) {
            return { kind: 'map', values: shapeOf(target.additionalProperties, fallback) };
        }
        return { kind: 'other' };
    }

    const rootShape = objectShape('plan', root);
    // Definitions the root never reaches by a property (a drift test still wants them).
    Object.keys(defs).forEach((name) => {
        if (isObject(defs[name]) && isObject(defs[name].properties)) objectShape(name, defs[name]);
    });
    return { root: rootShape, objects };
}

let cachedIndex = null;

function schemaIndex() {
    if (!cachedIndex) cachedIndex = buildIndex(loadSchemaRoot());
    return cachedIndex;
}

/** True when a shape ends in free text: a string, or an array or map of one. */
function holdsText(shape) {
    if (!shape) return false;
    if (shape.kind === 'string') return true;
    if (shape.kind === 'array') return holdsText(shape.items);
    if (shape.kind === 'map') return holdsText(shape.values);
    return false;
}

/**
 * Every free-text string property of a schema, as `definition.property`.
 * @param {object} [root] A schema document; the real schema when omitted.
 * @returns {string[]}
 */
function stringFields(root) {
    const index = root ? buildIndex(root) : schemaIndex();
    const found = [];
    index.objects.forEach((shape, name) => {
        shape.props.forEach((child, prop) => {
            if (holdsText(child)) found.push(name + '.' + prop);
        });
    });
    return found.sort();
}

/**
 * Free-text properties of a schema that no row of FIELD_LANGUAGE covers.
 * Empty on the real schema; a field added without a row appears here.
 * @param {object} [root]
 * @returns {string[]}
 */
function unregisteredFields(root) {
    return stringFields(root).filter((key) => {
        const dot = key.indexOf('.');
        return languageOf(key.slice(0, dot), key.slice(dot + 1)) === undefined;
    });
}

/**
 * Properties named like a clip path that AUDIO_PAIRS has no row for.
 * @param {object} [root]
 * @returns {string[]}
 */
function unpairedAudioFields(root) {
    const index = root ? buildIndex(root) : schemaIndex();
    const found = [];
    index.objects.forEach((shape, name) => {
        shape.props.forEach((child, prop) => {
            if (CLIP_PROPERTIES.includes(prop)
                && !AUDIO_PAIRS.some((row) => row.owner === name && row.clip === prop)) {
                found.push(name + '.' + prop);
            }
        });
    });
    return found.sort();
}

// --- the walk ------------------------------------------------------------------------

/**
 * Walk a plan along the schema. Unknown properties are not followed (the schema
 * rule reports them), so the walk is bounded by the schema's depth.
 * `onString({ owner, prop, value, pointer, identity })` for each free-text value;
 * `onObject({ owner, node, pointer, identity })` for each object.
 * `identity` names a node by ids, so a working copy and a snapshot line up.
 */
function walk(plan, onString, onObject) {
    const { root } = schemaIndex();

    function child(identity, prop, node, index) {
        const id = isObject(node) && typeof node.id === 'string' && node.id !== '' ? node.id : null;
        return identity + '/' + prop + (id === null ? (index === undefined ? '' : '/#' + index) : '/' + id);
    }

    function visit(value, shape, at, identity, owner, prop, index) {
        if (!shape) return;
        if (shape.kind === 'string') {
            if (typeof value === 'string') onString({ owner, prop, value, pointer: at, identity });
        } else if (shape.kind === 'array') {
            if (!Array.isArray(value)) return;
            value.forEach((item, i) => {
                const itemIdentity = isObject(item) ? child(identity, prop, item, i) : identity;
                visit(item, shape.items, pointer(at, i), itemIdentity, owner, prop, i);
            });
        } else if (shape.kind === 'map') {
            // map values are story tokens' glosses; registered as mixed, never checked
        } else if (shape.kind === 'oneOf') {
            const branch = isObject(value) ? shape.branches.get(value.type) : undefined;
            if (branch) visit(value, branch, at, identity, owner, prop, index);
        } else if (shape.kind === 'object') {
            if (!isObject(value)) return;
            onObject({ owner: shape.def, node: value, pointer: at, identity });
            Object.keys(value).forEach((key) => {
                const next = shape.props.get(key);
                if (!next) return;
                const nextIdentity = next.kind === 'object' || next.kind === 'oneOf'
                    ? child(identity, key, value[key])
                    : identity;
                visit(value[key], next, pointer(at, key), nextIdentity, shape.def, key);
            });
        }
    }

    visit(plan, root, '', '', 'plan', '');
}

// --- findings ------------------------------------------------------------------------

function makeFinding(plan, rule, at, message, fix) {
    const target = toIdPath(plan, at);
    const finding = { rule, severity: 'warn', path: at, message, field: target.field };
    if (fix) {
        finding.fix = Object.assign({ action: actionFor(target), path: target.path, field: target.field }, fix);
    }
    return finding;
}

/**
 * pointing-form clauses for one Hebrew string.
 * @returns {{ clause: 'i'|'ii'|'iii', message: string, suggested?: string }[]}
 */
function pointingClauses(value) {
    const out = [];
    const nfc = value.normalize('NFC');
    let stray = 0;
    let doubled = 0;
    let letter = false;
    let vowels = 0;
    let position = 0;
    for (const ch of nfc) {
        position += 1;
        if (HEBREW_LETTER.test(ch)) {
            letter = true;
            vowels = 0;
        } else if (POINT_CLASSES.vowel.test(ch)) {
            if (!letter) {
                if (!stray) stray = position;
            } else {
                vowels += 1;
                if (vowels === 2 && !doubled) doubled = position;
            }
        } else if (!POINT_CLASSES.combining.test(ch)) {
            letter = false;
            vowels = 0;
        }
    }
    if (stray) {
        out.push({
            clause: 'i',
            message: 'a vowel point at character ' + stray + ' has no Hebrew letter before it'
        });
    }
    if (doubled) {
        out.push({
            clause: 'ii',
            message: 'two vowel points sit on one letter, ending at character ' + doubled
        });
    }
    if (value !== nfc) {
        out.push({
            clause: 'iii',
            message: 'the text is not in NFC form (letters, points, and marks are in a different order than the stored form)',
            suggested: nfc
        });
    }
    return out;
}

/**
 * True when Hebrew letters outnumber Latin letters. An English field that quotes a
 * Hebrew word to explain it ("לא with א means no") is not flagged; one that is
 * Hebrew, or has no Latin letter at all, is. Tuned on the three imported plans.
 */
function mostlyHebrew(value) {
    const hebrew = (value.match(/[\u05D0-\u05EA]/g) || []).length;
    const latin = (value.match(/[A-Za-z]/g) || []).length;
    return hebrew > latin;
}

/** Hebrew fields: language and pointing form. English and Latin fields: language. */
function checkText(plan, strings, findings) {
    strings.forEach((item) => {
        const language = languageOf(item.owner, item.prop);
        if (!language || !item.value.trim()) return;
        const hasHebrew = HEBREW_LETTER.test(item.value);
        if (language === 'he') {
            if (!hasHebrew && /\p{L}/u.test(item.value)) {
                findings.push(makeFinding(plan, 'field-language', item.pointer,
                    'this field is marked Hebrew and holds no Hebrew letter; it may hold English or a transliteration'));
            }
            pointingClauses(item.value).forEach((clause) => {
                const fix = clause.suggested === undefined ? undefined : { suggested: clause.suggested };
                findings.push(makeFinding(plan, 'pointing-form', item.pointer, clause.message, fix));
            });
        } else if ((language === 'en' || language === 'latin') && hasHebrew
            && (language === 'latin' || mostlyHebrew(item.value))) {
            findings.push(makeFinding(plan, 'field-language', item.pointer,
                'this field is marked ' + (language === 'en' ? 'English' : 'Latin-script')
                + ' and holds ' + (language === 'en' ? 'mostly ' : 'a ') + 'Hebrew letter'
                + (language === 'en' ? 's' : '') + '; the Hebrew may belong in a Hebrew field'));
        }
    });
}

/**
 * lexicon-form. The lexicon is read through `listByStrippedKeys(keys)`, which
 * returns every row whose stripped key is in `keys`. A lexicon without it, or
 * one that throws (no table yet), produces no finding: this rule is advice.
 */
function checkLexicon(plan, entries, lexicon, findings) {
    if (!isObject(lexicon) || typeof lexicon.listByStrippedKeys !== 'function') return;
    const keys = new Set();
    entries.forEach((entry) => {
        if (typeof entry.node.word === 'string' && HEBREW_LETTER.test(entry.node.word)) {
            keys.add(strippedKey(entry.node.word));
        }
    });
    if (!keys.size) return;
    let rows;
    try {
        rows = list(lexicon.listByStrippedKeys(Array.from(keys)));
    } catch (err) {
        return;
    }
    const byStripped = new Map();
    rows.forEach((row) => {
        if (!isObject(row) || typeof row.key !== 'string') return;
        const stripped = strippedKey(row.key);
        if (!byStripped.has(stripped)) byStripped.set(stripped, []);
        byStripped.get(stripped).push(row);
    });
    entries.forEach((entry) => {
        const word = entry.node.word;
        if (typeof word !== 'string' || !HEBREW_LETTER.test(word)) return;
        const matches = byStripped.get(strippedKey(word)) || [];
        if (matches.length !== 1) return;
        const row = matches[0];
        const verified = row.verified === 1 || row.verified === true;
        if (exactKey(row.key) !== exactKey(word)) {
            findings.push(makeFinding(plan, 'lexicon-form', pointer(entry.pointer, 'word'),
                'the lexicon has one entry with these letters and it is pointed differently'
                + (verified
                    ? '; it is verified, so the lexicon form is the authority'
                    : '; it is not verified, so treat it as a lead and not as the authority'),
                { suggested: exactKey(row.key) }));
        }
        if (!verified) return;
        [['meaning', 'gloss_en'], ['translit', 'translit']].forEach(([field, column]) => {
            const have = entry.node[field];
            const stored = row[column];
            if (typeof have !== 'string' || typeof stored !== 'string' || !have.trim() || !stored.trim()) return;
            if (have.trim().toLowerCase() === stored.trim().toLowerCase()) return;
            findings.push(makeFinding(plan, 'lexicon-form', pointer(entry.pointer, field),
                'this ' + field + ' differs from the verified lexicon entry, which is the authority',
                { suggested: stored.trim() }));
        });
    });
}

/**
 * pointing-consistency: one word pointed two ways inside one lesson, across the
 * word-level Hebrew fields. Only forms that carry a point are compared, so an
 * unpointed letters-lesson word does not clash with its pointed twin. The most
 * frequent form (the earliest on a tie) is the reference; each occurrence that
 * differs from it is reported with every form as `fix.candidates`.
 */
function checkConsistency(plan, strings, findings) {
    // The vocabulary entries of a lesson are one set, and the cells of one conjugation table
    // are another: masculine and feminine forms, or two cells, share their letters on purpose
    // (tuned on the imported my-morning plan: שׁוֹתֶה and שׁוֹתָה are two entries).
    const setOf = (item) => (item.owner === 'wordEntry' ? 'words'
        : (item.owner === 'conjugationCell' ? item.pointer.replace(/\/cells\/\d+\/formKey$/, '') : null));
    const lessons = new Map();
    strings.forEach((item) => {
        if (!WORD_LEVEL_FIELDS.includes(item.owner + '.' + item.prop)) return;
        const lesson = /^\/units\/\d+\/lessons\/\d+/.exec(item.pointer);
        if (!lesson || !HEBREW_POINT.test(item.value)) return;
        const form = formOf(item.value);
        const stripped = strippedKey(form);
        if (!stripped) return;
        if (!lessons.has(lesson[0])) lessons.set(lesson[0], new Map());
        const groups = lessons.get(lesson[0]);
        if (!groups.has(stripped)) groups.set(stripped, []);
        groups.get(stripped).push({ form, pointer: item.pointer, set: setOf(item) });
    });
    lessons.forEach((groups) => {
        groups.forEach((occurrences) => {
            const counts = new Map();
            occurrences.forEach((o) => counts.set(o.form, (counts.get(o.form) || 0) + 1));
            if (counts.size < 2) return;
            let reference = null;
            counts.forEach((count, form) => {
                if (reference === null || count > counts.get(reference)) reference = form;
            });
            const candidates = Array.from(counts.keys());
            occurrences.forEach((o) => {
                if (o.form === reference) return;
                // Entries of one set are listed side by side on purpose, so they are not
                // compared with each other.
                if (!occurrences.some((p) => p.form !== o.form && !(p.set && p.set === o.set))) return;
                const other = occurrences.find((p) => p.form === reference);
                findings.push(makeFinding(plan, 'pointing-consistency', o.pointer,
                    'this lesson points the same word two ways; this field has ' + o.form
                    + ' and ' + toIdPath(plan, other.pointer).path + ' (' + toIdPath(plan, other.pointer).field
                    + ') has ' + reference + '. Both may be right (a homograph); pick one if they are not',
                    { candidates }));
            });
        });
    });
}

/**
 * audio-stale: a node in the working copy and the snapshot whose text differs
 * while its clip path is the same non-empty path. Silent without a snapshot.
 */
function checkAudio(plan, objects, snapshot, findings) {
    if (!isObject(snapshot)) return;
    const before = new Map();
    walk(snapshot, () => {}, (item) => {
        AUDIO_PAIRS.forEach((pair) => {
            if (item.owner !== pair.owner) return;
            before.set(item.identity + '\u0000' + pair.text + '\u0000' + pair.clip, {
                text: item.node[pair.text],
                clip: item.node[pair.clip]
            });
        });
    });
    objects.forEach((item) => {
        AUDIO_PAIRS.forEach((pair) => {
            if (item.owner !== pair.owner) return;
            const old = before.get(item.identity + '\u0000' + pair.text + '\u0000' + pair.clip);
            const text = item.node[pair.text];
            const clip = item.node[pair.clip];
            if (!old || typeof text !== 'string' || typeof old.text !== 'string') return;
            if (typeof clip !== 'string' || clip === '' || clip !== old.clip) return;
            if (text.normalize('NFC').trim() === old.text.normalize('NFC').trim()) return;
            const target = toIdPath(plan, pointer(item.pointer, pair.clip));
            findings.push(makeFinding(plan, 'audio-stale', pointer(item.pointer, pair.text),
                'this text changed since the last publish and its clip ' + pair.clip + ' did not, so the clip may no '
                + 'longer match the text. This check compares with the published copy; it cannot tell a clip that '
                + 'was already stale at publish.',
                { action: 'replace_part', path: target.path, field: target.field, text: pair.text, clip: pair.clip,
                    note: REGENERATE_NOTE }));
        });
    });
}

function parseSnapshot(snapshot) {
    if (isObject(snapshot)) return snapshot;
    if (typeof snapshot !== 'string' || !snapshot) return null;
    try {
        const parsed = JSON.parse(snapshot);
        return isObject(parsed) ? parsed : null;
    } catch (err) {
        return null;
    }
}

/**
 * Run the five checks over one plan document.
 * @param {unknown} plan A parsed plan.
 * @param {{ lexicon?: object, snapshot?: object|string|null }} [options]
 *   `lexicon.listByStrippedKeys(keys)` for lexicon-form; `snapshot` (the published
 *   copy, parsed or as JSON text) for audio-stale.
 * @returns {{ rule: string, severity: 'warn', path: string, message: string, field: string, fix?: object }[]}
 *   `path` is an RFC 6901 pointer, as every validator finding; `field` is the pointer
 *   remainder below the deepest id-addressable node; `fix` names one `set_field` or
 *   `replace_part` call and the value (`suggested`) or values (`candidates`).
 */
function checkHebrew(plan, options) {
    if (!isObject(plan)) return [];
    const opts = isObject(options) ? options : {};
    const strings = [];
    const objects = [];
    walk(plan, (item) => strings.push(item), (item) => objects.push(item));
    const findings = [];
    checkText(plan, strings, findings);
    checkLexicon(plan, objects.filter((item) => item.owner === 'wordEntry'), opts.lexicon, findings);
    checkConsistency(plan, strings, findings);
    checkAudio(plan, objects, parseSnapshot(opts.snapshot), findings);
    return findings;
}

/**
 * The rule rows validate.js adds to its registry. All warn, none blocks (D3).
 * Labels: pointing-form, field-language, pointing-consistency, audio-stale are
 * product choices (RSCH-009 §1.4, D8); lexicon-form is adapted from ADR-0009 §6
 * (`verified` is the authority).
 */
const HEBREW_RULES = Object.freeze({
    'pointing-form': Object.freeze({
        severity: 'warn',
        summary: 'Hebrew text has no stray or doubled vowel point and is in NFC form. Product choice; the allowed marks are in POINT_CLASSES.'
    }),
    'field-language': Object.freeze({
        severity: 'warn',
        summary: 'A field marked Hebrew holds a Hebrew letter; a field marked English or Latin holds none. Product choice.'
    }),
    'lexicon-form': Object.freeze({
        severity: 'warn',
        summary: 'A new word matches its one lexicon entry by stripped key in pointing, and, when that entry is verified, in meaning and translit. Adapted from ADR-0009 §6.'
    }),
    'pointing-consistency': Object.freeze({
        severity: 'warn',
        summary: 'One word is pointed one way across the word-level Hebrew fields of a lesson. Product choice.'
    }),
    'audio-stale': Object.freeze({
        severity: 'warn',
        summary: 'A text field changed since the last publish while its clip path stayed. Product choice; needs a published snapshot.'
    })
});

module.exports = {
    AUDIO_PAIRS,
    FIELD_LANGUAGE,
    HEBREW_RULES,
    POINT_CLASSES,
    checkHebrew,
    exactKey,
    formOf,
    languageOf,
    pointingClauses,
    stringFields,
    strippedKey,
    unpairedAudioFields,
    unregisteredFields
};
