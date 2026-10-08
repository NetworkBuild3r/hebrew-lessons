const fs = require('fs');
const path = require('path');
const root = process.env.HEBREW_ROOT
    ? path.resolve(process.env.HEBREW_ROOT)
    : path.resolve(__dirname, '..', '..', '..', '..');
const lessonsRoot = process.env.LESSONS_DIR
    ? path.resolve(process.env.LESSONS_DIR)
    : path.join(root, 'worksheets');
const slug = process.argv[2];

if (!slug || slug.startsWith('-')) {
    console.error('Usage: node .cursor/skills/hebrew-worksheet/scripts/validate.js <slug>');
    process.exit(1);
}

const errors = [];
function fail(message) { errors.push(message); }

// Lesson and catalog files are data. They are parsed as literals and never executed
// (vm is not a security boundary; see server/lesson-data.js).
const { loadDataWindow } = require(path.join(root, 'server', 'lesson-data.js'));

function loadWindow(file) {
    return loadDataWindow(file);
}

function stripNikud(value) {
    return String(value || '').replace(/[\u0591-\u05C7]/g, '');
}

function hasNikud(value) {
    return /[\u0591-\u05C7]/.test(String(value || ''));
}

/** NFC + whitespace collapse — ADR-0001 exactKey (INIT-001/SPEC-008). */
function exactKey(text) {
    return String(text || '').normalize('NFC').replace(/\s+/g, ' ').trim();
}

/**
 * Split Hebrew text into tokens (letters + nikud + maqaf).
 * TODO(INIT-001/SPEC-007): consolidate with shared/player/utils.js tokenizeHebrew when it lands.
 */
function tokenizeHebrew(text) {
    const s = exactKey(text);
    if (!s) return [];
    return s.split(/[^\u05D0-\u05EA\u0591-\u05C7\u05BE]+/u).filter(token => /[\u05D0-\u05EA]/.test(token));
}

function countEncounters(corpus, word) {
    const target = stripNikud(word);
    if (!target) return 0;
    const plain = stripNikud(corpus);
    let count = 0;
    let idx = 0;
    while (idx < plain.length) {
        const found = plain.indexOf(target, idx);
        if (found < 0) break;
        count += 1;
        idx = found + Math.max(target.length, 1);
    }
    return count;
}

const catalogModule = path.join(root, 'shared', 'activities', 'catalog.js');
let ACTIVITY_TYPES = new Set();
let catalogMeta = [];
try {
    const mod = require(catalogModule);
    catalogMeta = mod.CATALOG || [];
    ACTIVITY_TYPES = new Set(catalogMeta.map(item => item.type));
} catch (err) {
    fail('Could not load shared/activities/catalog.js: ' + err.message);
}

const lessonDir = path.join(lessonsRoot, slug);
const dataFile = path.join(lessonDir, 'worksheet.js');
const shellFile = path.join(lessonDir, 'index.html');
const catalogFile = path.join(lessonsRoot, 'catalog.js');

if (!fs.existsSync(dataFile)) fail('Missing ' + path.relative(root, dataFile));
if (!fs.existsSync(shellFile)) fail('Missing lesson index.html. Copy worksheets/_template whole.');
if (errors.length) {
    errors.forEach(message => console.error(message));
    process.exit(1);
}

let ws;
let catalog;
try {
    ws = loadWindow(dataFile).WORKSHEET;
} catch (err) {
    fail('worksheet.js did not run: ' + err.message);
}
try {
    catalog = loadWindow(catalogFile).HEBREW_CATALOG;
} catch (err) {
    fail('catalog.js did not run: ' + err.message);
}

if (!ws || !ws.id) fail('window.WORKSHEET.id is missing.');
if (ws && ws.id === 'template') fail('id is still "template". Set it to the folder slug.');
if (ws && ws.id && ws.id !== slug) fail('id "' + ws.id + '" does not match the folder "' + slug + '".');
if (!Array.isArray(catalog)) fail('window.HEBREW_CATALOG is not an array.');
else if (!catalog.some(entry => entry && entry.slug === slug)) fail('catalog.js has no entry with slug "' + slug + '".');

const vocab = ws?.vocab?.items || [];
const sentences = ws?.fill?.sentences || [];
const bank = ws?.fill?.bank || [];
const grammar = ws?.grammar?.items || [];
const paragraphs = ws?.reading?.paragraphs || [];
const friends = ws?.askFriend?.questions || [];
const activities = Array.isArray(ws?.activities) ? ws.activities : null;

if (!vocab.length && !sentences.length && !grammar.length && !paragraphs.length && !friends.length) {
    fail('The lesson has no vocab, fill, grammar, reading, or speaking section.');
}

if (ws.level && !['basic', 'intermediate', 'advanced'].includes(String(ws.level).toLowerCase())) {
    fail('level must be basic, intermediate, or advanced (got "' + ws.level + '").');
}

const meanings = new Map();
vocab.forEach(item => {
    if (!item || !item.word) fail('A vocab item is missing word.');
    if (!item.meaning) fail('Vocab "' + (item.word || item.id) + '" is missing meaning.');
    if (!item.sound) fail('Vocab "' + (item.word || item.id) + '" is missing sound.');
    if (!item.trap) fail('Vocab "' + (item.word || item.id) + '" is missing trap.');
    if (item.meaning) meanings.set(stripNikud(item.word), String(item.meaning).toLowerCase());
});

function norm(value) {
    return stripNikud(value).replace(/[.,!?;:"'״׳\-–—()]/g, '').replace(/\s+/g, ' ').trim();
}

if (sentences.length) {
    const answers = sentences.map(sentence => norm(sentence.answer)).filter(Boolean);
    const unique = new Set(answers);
    if (unique.size !== answers.length) fail('Two fill sentences share an answer. Each answer is used once.');
    bank.forEach(word => {
        const count = bank.filter(entry => norm(entry) === norm(word)).length;
        if (count > 1) fail('Word bank repeats "' + word + '".');
    });
    const bankNorms = new Set(bank.map(norm));
    answers.forEach(answer => {
        if (!bankNorms.has(answer)) fail('Fill answer "' + answer + '" is not in the word bank.');
    });
    const extras = [...bankNorms].filter(word => !unique.has(word));
    if (extras.length < 1) fail('The word bank needs one extra word that is not an answer.');
    sentences.forEach(sentence => {
        const meaning = meanings.get(norm(sentence.answer));
        if (!sentence.english || !meaning) return;
        const leak = meaning.split('/').map(part => part.trim()).filter(part => part.length > 2);
        leak.forEach(part => {
            const pattern = new RegExp('(^|[^a-z])' + part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '([^a-z]|$)', 'i');
            if (pattern.test(sentence.english)) {
                fail('Fill english for "' + sentence.answer + '" gives away "' + part + '". Use ____.');
            }
        });
    });
}

const warnings = [];
function warn(message) { warnings.push(message); }

const rules = ws?.grammar?.rules || [];
if (rules.length > 1) {
    fail('grammar.rules has ' + rules.length + ' rules. State one transparent pattern. Move the rest into grammar.reminders as one-liners.');
}

grammar.forEach((item, index) => {
    const choices = item?.choices || [];
    const label = 'Grammar item ' + (item?.id ?? index);
    if (!Number.isInteger(item?.answer) || item.answer < 0 || item.answer >= choices.length) {
        fail(label + ' answer index is outside its choices.');
        return;
    }
    if (item.pointed && norm(item.pointed) !== norm(choices[item.answer])) {
        fail(label + ' pointed does not match the correct choice once the vowel points are removed.');
    }
});
if (grammar.length > 1) {
    const spread = new Set(grammar.map(item => item?.answer));
    if (spread.size < 2) warn('Every grammar answer sits at index ' + [...spread][0] + '. Vary the position of the correct choice.');
}

friends.forEach((question, index) => {
    if (!question?.question) fail('Speaking question ' + (index + 1) + ' is missing question.');
    if (!question?.pointed || !hasNikud(question.pointed)) {
        fail('Speaking question ' + (index + 1) + ' needs pointed Hebrew in pointed.');
    }
});

function partText(part) {
    if (typeof part === 'string') return part;
    if (!part || typeof part !== 'object') return '';
    const value = part.reported || part.direct || part.strong || part.word || '';
    return Array.isArray(value) ? value.map(partText).join('') : value;
}

function collectWords(part, into) {
    if (Array.isArray(part)) { part.forEach(entry => collectWords(entry, into)); return; }
    if (!part || typeof part !== 'object') return;
    if (part.word) into.push(String(part.word));
    if (Array.isArray(part.reported)) collectWords(part.reported, into);
    if (Array.isArray(part.direct)) collectWords(part.direct, into);
}

let storyText = '';
if (paragraphs.length) {
    storyText = paragraphs.map(paragraph => (Array.isArray(paragraph) ? paragraph : [paragraph]).map(partText).join('')).join(' ');
    if (!hasNikud(storyText)) warn('The story has no vowel points. Show vowels on a story with new words.');
    const marked = [];
    paragraphs.forEach(paragraph => collectWords(paragraph, marked));
    const vocabForms = vocab.map(item => stripNikud(item.word));
    marked.forEach(word => {
        if (!vocabForms.includes(stripNikud(word))) warn('Story marks "' + word + '" but it is not a vocab word.');
    });
    const plainStory = stripNikud(storyText);
    vocab.forEach(item => {
        if (item?.word && !plainStory.includes(stripNikud(item.word))) warn('Vocab word "' + stripNikud(item.word) + '" never appears in the story.');
    });
    if (vocab.length && !marked.length) warn('No new words are marked in the story. Wrap them in { word: "..." }.');
}

// reading.glosses — ADR-0001 §8 / INIT-001/SPEC-008
const glosses = ws?.reading?.glosses;
if (glosses !== undefined && glosses !== null) {
    if (typeof glosses !== 'object' || Array.isArray(glosses)) {
        fail('reading.glosses must be an object map of story-token → English gloss (ADR-0001).');
    } else {
        const storyNfc = exactKey(storyText);
        Object.keys(glosses).forEach(key => {
            const value = glosses[key];
            if (typeof key !== 'string' || typeof value !== 'string') {
                fail('reading.glosses entries must be string → string.');
                return;
            }
            const keyNfc = exactKey(key);
            if (!keyNfc) return;
            const inStory = storyNfc.includes(keyNfc)
                || tokenizeHebrew(storyText).some(token => exactKey(token) === keyNfc || stripNikud(token) === stripNikud(keyNfc));
            if (!inStory) {
                warn('reading.glosses key "' + key + '" was not found in the story text.');
            }
        });
    }
}

function audioCheck(value, label) {
    if (!value) return;
    if (typeof value !== 'string' || value.includes('\\')) {
        fail(label + ' audio path must be relative (audio/etza.mp3), ../_audio/words/..., or /shared/audio-cache/words/...');
        return;
    }
    if (value.startsWith('../_audio/')) {
        if (value.includes('..', 3) || !/^(\.\.\/_audio\/words\/)[^/]+\.mp3$/.test(value)) {
            fail(label + ' shared word audio must be ../_audio/words/<file>.mp3');
            return;
        }
        const full = path.resolve(lessonDir, value);
        const base = path.resolve(lessonsRoot, '_audio') + path.sep;
        if (!full.startsWith(base)) {
            fail(label + ' shared audio path escaped LESSONS_DIR/_audio/.');
            return;
        }
        if (!fs.existsSync(full)) fail(label + ' shared audio file is missing: ' + value);
        return;
    }
    if (value.includes('..')) {
        fail(label + ' audio path must be relative (audio/etza.mp3), ../_audio/words/..., or /shared/audio-cache/words/...');
        return;
    }
    if (value.startsWith('/shared/audio-cache/')) {
        const full = path.resolve(root, value.slice(1));
        const base = path.resolve(root, 'shared', 'audio-cache') + path.sep;
        if (!full.startsWith(base)) {
            fail(label + ' shared audio path escaped shared/audio-cache/.');
            return;
        }
        if (!fs.existsSync(full)) fail(label + ' shared audio file is missing: ' + value);
        return;
    }
    if (value.startsWith('/') || path.isAbsolute(value)) {
        fail(label + ' audio path must be relative to the lesson folder, ../_audio/words/, or under /shared/audio-cache/.');
        return;
    }
    if (!fs.existsSync(path.join(lessonDir, value))) fail(label + ' audio file is missing: ' + value);
}
vocab.forEach(item => {
    audioCheck(item?.audio, 'Vocab "' + (item?.word || item?.id) + '"');
    audioCheck(item?.sentenceAudio, 'Vocab sentence "' + (item?.word || item?.id) + '"');
});
friends.forEach((question, index) => audioCheck(question?.audio, 'Speaking question ' + (index + 1)));
grammar.forEach((item, index) => audioCheck(item?.audio, 'Grammar item ' + (index + 1)));
(ws?.reading?.questions || []).forEach((question, index) => audioCheck(question?.audio, 'Reading question ' + (index + 1)));
if (ws?.reading?.audio != null) {
    if (!Array.isArray(ws.reading.audio)) {
        fail('reading.audio must be an array of story line clip paths.');
    } else {
        if (ws.reading.audio.length > paragraphs.length) {
            fail('reading.audio has more entries than reading.paragraphs.');
        }
        ws.reading.audio.forEach((clip, index) => audioCheck(clip, 'Story line audio ' + (index + 1)));
    }
}

const others = Array.isArray(catalog) ? catalog.filter(entry => entry && entry.slug && entry.slug !== slug) : [];
const earlier = new Map();
others.forEach(entry => {
    const file = path.join(lessonsRoot, entry.slug, 'worksheet.js');
    if (!fs.existsSync(file)) return;
    try {
        const items = loadWindow(file).WORKSHEET?.vocab?.items || [];
        earlier.set(entry.slug, new Set(items.map(item => stripNikud(item.word))));
    } catch (err) {
        warn('Could not read lesson "' + entry.slug + '" to check recycled words.');
    }
});

// Earlier-corpus keys: prefer vocab-index, else lesson walk; word-audio catalogs keys seen before (INIT-001/SPEC-008).
const earlierKeys = new Set();
let usedVocabIndex = false;
const vocabIndexFile = path.join(lessonsRoot, '_review', 'vocab-index.js');
if (fs.existsSync(vocabIndexFile)) {
    try {
        const index = loadWindow(vocabIndexFile).HEBREW_VOCAB_INDEX;
        if (Array.isArray(index)) {
            usedVocabIndex = true;
            index.forEach(entry => {
                if (!entry) return;
                const lessons = Array.isArray(entry.lessons) ? entry.lessons : [];
                if (!lessons.some(lessonSlug => lessonSlug && lessonSlug !== slug)) return;
                const key = stripNikud(entry.word || entry.key || '');
                if (key) earlierKeys.add(key);
            });
        }
    } catch (err) {
        warn('Could not read vocab-index to check recycled words.');
    }
}
if (!usedVocabIndex) {
    earlier.forEach(words => {
        words.forEach(word => earlierKeys.add(word));
    });
}
const wordAudioFile = path.join(lessonsRoot, '_audio', 'words', 'catalog.js');
const wordAudioKeys = new Set();
if (fs.existsSync(wordAudioFile)) {
    try {
        const map = loadWindow(wordAudioFile).HEBREW_WORD_AUDIO;
        if (map && typeof map === 'object') {
            Object.keys(map).forEach(key => {
                const stripped = stripNikud(key);
                if (stripped) wordAudioKeys.add(stripped);
            });
        }
    } catch (err) {
        warn('Could not read word-audio catalog to check recycled words.');
    }
}

vocab.forEach(item => {
    if (!item?.from) return;
    const words = earlier.get(item.from);
    if (!words) fail('Vocab "' + item.word + '" says from "' + item.from + '", which is not a published lesson with vocab.');
    else if (!words.has(stripNikud(item.word))) fail('Vocab "' + item.word + '" is not a word in lesson "' + item.from + '".');
});

const recycledWords = new Set();
vocab.forEach(item => {
    if (!item?.word) return;
    const key = stripNikud(item.word);
    const fromEarlier = item.from || earlierKeys.has(key);
    // word-audio alone is not enough; require earlier-lesson evidence (index or from:)
    if (fromEarlier) recycledWords.add(key);
    else if (wordAudioKeys.has(key) && earlierKeys.has(key)) recycledWords.add(key);
});
const recycled = recycledWords.size;
const hasEarlierWords = earlierKeys.size > 0 || [...earlier.values()].some(words => words.size > 0);
if (vocab.length && hasEarlierWords) {
    if (recycled < 3) {
        warn('Bring back at least 3 words from earlier lessons (set from: "<slug>" or reuse catalog/index words). This lesson recycles ' + recycled + '. Use list_words.');
    }
    if (recycled > 5) warn('This lesson recycles ' + recycled + ' earlier words. Keep it to 5 so new words stay the focus.');
}

// Encounters nudge: new (non-recycled) vocab should appear ≥2 times in story/sentences/activities.
const encounterChunks = [];
if (storyText) encounterChunks.push(storyText);
sentences.forEach(sentence => {
    if (sentence?.hebrew) encounterChunks.push(String(sentence.hebrew));
    if (sentence?.text) encounterChunks.push(String(sentence.text));
    if (sentence?.answer) encounterChunks.push(String(sentence.answer));
    if (sentence?.english) encounterChunks.push(String(sentence.english));
});
(bank || []).forEach(word => encounterChunks.push(String(word)));
vocab.forEach(item => {
    if (item?.sentence) encounterChunks.push(String(item.sentence));
});
grammar.forEach(item => {
    if (item?.pointed) encounterChunks.push(String(item.pointed));
    (item?.choices || []).forEach(choice => encounterChunks.push(String(choice)));
});
friends.forEach(question => {
    if (question?.pointed) encounterChunks.push(String(question.pointed));
    if (question?.question) encounterChunks.push(String(question.question));
});
if (Array.isArray(activities)) {
    activities.forEach(entry => {
        if (!entry || typeof entry !== 'object') return;
        Object.keys(entry).forEach(key => {
            if (key === 'type') return;
            const value = entry[key];
            if (typeof value === 'string') encounterChunks.push(value);
        });
    });
}
const encounterCorpus = encounterChunks.join(' ');
vocab.forEach(item => {
    if (!item?.word) return;
    const key = stripNikud(item.word);
    if (recycledWords.has(key)) return;
    const hits = countEncounters(encounterCorpus, item.word);
    if (hits < 2) {
        warn('New vocab "' + item.word + '" appears only ' + hits + ' time(s) across story/sentences/activities (want ≥2).');
    }
});

// Coverage nudge: estimated share of story tokens known (this lesson + earlier) ≥90%.
if (storyText && (vocab.length || earlierKeys.size)) {
    const knownKeys = new Set(earlierKeys);
    vocab.forEach(item => {
        if (item?.word) knownKeys.add(stripNikud(item.word));
    });
    const tokens = tokenizeHebrew(storyText);
    if (tokens.length) {
        let known = 0;
        tokens.forEach(token => {
            if (knownKeys.has(stripNikud(token))) known += 1;
        });
        const pct = Math.round((known / tokens.length) * 100);
        if (pct < 90) {
            warn('Estimated known-word coverage of the story is ' + pct + '% (below 90%). Estimate only; prefixes are not stripped.');
        }
    }
}

if (activities) {
    if (!activities.length) fail('activities is an empty array. Omit it to use the default path, or list at least one step.');
    let meaning = 0;
    let drill = 0;
    activities.forEach((entry, index) => {
        const type = typeof entry === 'string' ? entry : entry && entry.type;
        const label = 'activities[' + index + ']';
        if (!type) {
            fail(label + ' needs a type.');
            return;
        }
        if (!ACTIVITY_TYPES.has(type)) {
            fail(label + ' has unknown type "' + type + '". Call list_activity_types.');
            return;
        }
        const meta = catalogMeta.find(item => item.type === type);
        if (meta && meta.meaning) meaning += 1;
        else drill += 1;
        if (type === 'fill' && !sentences.length) fail(label + ' type fill needs fill.sentences.');
        if (type === 'pattern' && !grammar.length) fail(label + ' type pattern needs grammar.items.');
        if (type === 'story' && !paragraphs.length) fail(label + ' type story needs reading.paragraphs.');
        if (type === 'retell' && !(ws.reading && ws.reading.retell)) fail(label + ' type retell needs reading.retell.');
        if (type === 'ask-friend' && !friends.length) fail(label + ' type ask-friend needs askFriend.questions.');
        if ((type === 'words' || type === 'match' || type === 'speed-quiz' || type === 'listen-pick' || type === 'type-it') && !vocab.length) {
            fail(label + ' type ' + type + ' needs vocab.items.');
        }
        if (type === 'build-sentence') {
            const from = (entry && entry.from) || 'story';
            if (from === 'story' && !paragraphs.length) fail(label + ' build-sentence from story needs reading.paragraphs.');
            if (from === 'vocab' && !vocab.some(item => item.sentence)) fail(label + ' build-sentence from vocab needs vocab sentences.');
        }
        if (type === 'type-it' && String(ws.level || '').toLowerCase() !== 'advanced') {
            warn(label + ' type-it is meant for level advanced.');
        }
    });
    if (!activities.some(entry => (typeof entry === 'string' ? entry : entry && entry.type) === 'story') && paragraphs.length) {
        warn('The lesson has a story but no story activity in the path.');
    }
    if (!activities.some(entry => (typeof entry === 'string' ? entry : entry && entry.type) === 'ask-friend') && friends.length) {
        warn('The lesson has speaking questions but no ask-friend activity.');
    }
    if (!activities.some(entry => (typeof entry === 'string' ? entry : entry && entry.type) === 'story') && !friends.length) {
        warn('No story and no speaking step. Keep most of the lesson on meaning (story / speaking).');
    }
    if (drill > meaning && meaning > 0) {
        warn('Drill steps (' + drill + ') outnumber meaning steps (' + meaning + '). Keep about three quarters on story and speaking.');
    }
}

if (errors.length) {
    errors.forEach(message => console.error(message));
    process.exit(1);
}

console.log('OK ' + slug);
warnings.forEach(message => console.log('WARN ' + message));
