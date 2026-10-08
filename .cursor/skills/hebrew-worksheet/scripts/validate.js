const fs = require('fs');
const path = require('path');
const vm = require('vm');

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

function loadWindow(file) {
    const code = fs.readFileSync(file, 'utf8');
    const sandbox = { window: {}, console };
    vm.createContext(sandbox);
    vm.runInContext(code, sandbox, { filename: file });
    return sandbox.window;
}

function stripNikud(value) {
    return String(value || '').replace(/[\u0591-\u05C7]/g, '');
}

function hasNikud(value) {
    return /[\u0591-\u05C7]/.test(String(value || ''));
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

if (paragraphs.length) {
    const story = paragraphs.map(paragraph => (Array.isArray(paragraph) ? paragraph : [paragraph]).map(partText).join('')).join(' ');
    if (!hasNikud(story)) warn('The story has no vowel points. Show vowels on a story with new words.');
    const marked = [];
    paragraphs.forEach(paragraph => collectWords(paragraph, marked));
    const vocabForms = vocab.map(item => stripNikud(item.word));
    marked.forEach(word => {
        if (!vocabForms.includes(stripNikud(word))) warn('Story marks "' + word + '" but it is not a vocab word.');
    });
    const plainStory = stripNikud(story);
    vocab.forEach(item => {
        if (item?.word && !plainStory.includes(stripNikud(item.word))) warn('Vocab word "' + stripNikud(item.word) + '" never appears in the story.');
    });
    if (vocab.length && !marked.length) warn('No new words are marked in the story. Wrap them in { word: "..." }.');
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

let recycled = 0;
vocab.forEach(item => {
    if (!item?.from) return;
    recycled += 1;
    const words = earlier.get(item.from);
    if (!words) fail('Vocab "' + item.word + '" says from "' + item.from + '", which is not a published lesson with vocab.');
    else if (!words.has(stripNikud(item.word))) fail('Vocab "' + item.word + '" is not a word in lesson "' + item.from + '".');
});
const hasEarlierWords = [...earlier.values()].some(words => words.size > 0);
if (vocab.length && hasEarlierWords) {
    if (recycled < 3) warn('Bring back 3 to 5 words from earlier lessons (set from: "<slug>" on each). This lesson recycles ' + recycled + '. Use list_words.');
    if (recycled > 5) warn('This lesson recycles ' + recycled + ' earlier words. Keep it to 5 so new words stay the focus.');
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
