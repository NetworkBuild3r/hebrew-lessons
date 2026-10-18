'use strict';

// Seeds a fixture library with the Node implementation and writes jsonl
// that the Go store must match. List cases run before any suspend write.
// The snapshot database is the pre-suspend file.

const fs = require('fs');
const path = require('path');

const dataDir = path.resolve(process.argv[2]);
const lessonsDir = path.resolve(process.argv[3]);
const outPath = path.resolve(process.argv[4]);
if (!dataDir || !lessonsDir || !outPath) {
    console.error('usage: golden.js <data-dir> <lessons-dir> <out.jsonl>');
    process.exit(2);
}

const repo = path.resolve(__dirname, '../../../../..');
process.env.HEBREW_ROOT = repo;
process.env.DATA_DIR = dataDir;
process.env.LESSONS_DIR = lessonsDir;
process.env.TZ = 'UTC';
process.env.SRS_SCHEDULER = 'ladder';
fs.mkdirSync(dataDir, { recursive: true });
fs.mkdirSync(path.join(lessonsDir, 'hear'), { recursive: true });
fs.mkdirSync(path.join(lessonsDir, '_audio', 'words'), { recursive: true });

fs.writeFileSync(path.join(lessonsDir, 'hear', 'worksheet.js'),
    'window.WORKSHEET = {"vocab":{"items":['
    + '{"word":"שמע","meaning":"hear","sound":"shema","audio":"hear.mp3"}'
    + ']}};\n');
fs.writeFileSync(path.join(lessonsDir, '_audio', 'words', 'catalog.js'),
    'window.HEBREW_WORD_AUDIO = Object.assign({}, window.HEBREW_WORD_AUDIO || {}, '
    + '{"שמע":"cat.mp3"});\n');

const db = require(path.join(repo, 'server/db'));
const library = require(path.join(repo, 'server/learner/library'));
const lexicon = require(path.join(repo, 'server/db/repos/lexicon'));
db.migrate();

const now = '2026-01-15T12:00:00.000Z';
const day = (offset) => {
    const t = new Date(Date.parse(now) + offset * 24 * 60 * 60 * 1000);
    return t.toISOString();
};

function insertUser(username) {
    const created = db.run(
        `INSERT INTO users (
            username, password_hash, display_name, role, status,
            auth_provider, must_change_password, created_at, updated_at
        ) VALUES (?, 'x', NULL, 'user', 'active', 'local', 0, ?, ?)`,
        username, now, now
    );
    return Number(created.lastInsertRowid);
}

const learner = insertUser('learner');
const other = insertUser('other');

function lex(row) {
    lexicon.upsert(Object.assign({
        accept_json: '[]',
        verified: 0,
        source: 'seed',
        updated_at: now
    }, row));
}

lex({ key: 'כתב', kind: 'lemma', gloss_en: 'wrote', pos: 'verb', root: 'כ.ת.ב', binyan: 'paal', root_class: 'regular', translit: 'katav' });
lex({ key: 'כתבתי', kind: 'form', lemma_key: 'כתב', gloss_en: 'I wrote', tense: 'past', cell: '1s' });
lex({ key: 'שיר', kind: 'lemma', gloss_en: 'song', pos: 'noun', translit: 'shir' });

function personal(word, source, gloss) {
    db.run(
        `INSERT INTO user_words (user_id, word, source, added_at, gloss_en)
         VALUES (?, ?, ?, ?, ?)`,
        learner, word, source, now, gloss
    );
}

personal('שמע', 'self', null);
personal('מילה', 'self', 'a word');
personal('שיר', 'self', 'my note');
personal('כתב', 'ai', null);

function review(word, direction, fields) {
    const row = Object.assign({
        step: 0,
        streak: 0,
        lapses: 0,
        last_result: null,
        last_at: null,
        sources: '[]',
        stability: null,
        difficulty: null,
        reps: 0,
        fsrs_state: null,
        last_grade: null,
        suspended: 0,
        due: day(2)
    }, fields);
    db.run(
        `INSERT INTO review_items (
            user_id, word, direction, due, step, streak, lapses,
            last_result, last_at, sources, stability, difficulty, reps,
            fsrs_state, last_grade, suspended
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        learner, word, direction, row.due, row.step, row.streak, row.lapses,
        row.last_result, row.last_at, row.sources, row.stability, row.difficulty,
        row.reps, row.fsrs_state, row.last_grade, row.suspended
    );
}

review('אבא', 'he_en', { sources: '["intro"]', due: day(1) });
review('שמע', 'he_en', { step: 1, last_result: 'pass', last_grade: 3, last_at: day(-2), due: day(3) });
review('שמע', 'en_he', { step: 1, last_result: 'pass', last_grade: 3, last_at: day(-2), due: day(4) });
review('כתב', 'he_en', { step: 4, last_result: 'pass', last_grade: 4, last_at: day(-5), sources: '["my-morning"]', due: day(10) });
review('כתב', 'en_he', { step: 4, last_result: 'pass', last_grade: 4, last_at: day(-5), sources: '["my-morning"]', due: day(10) });
review('כתבתי', 'he_en', { step: 1, lapses: 3, last_result: 'miss', last_grade: 1, last_at: day(-1), sources: '["drill"]', due: day(-1) });
review('נח', 'he_en', { suspended: 1, due: day(0) });
review('נח', 'en_he', { suspended: 1, due: day(0) });
review('שיר', 'he_en', {
    step: 2, lapses: 2, last_result: 'pass', last_grade: 2, last_at: day(-3),
    sources: '["my-morning"]', stability: 30, difficulty: 5, reps: 1, due: day(6)
});
review('שיר', 'en_he', {
    step: 2, lapses: 2, last_result: 'pass', last_grade: 2, last_at: day(-3),
    sources: '["my-morning"]', stability: 30, difficulty: 5, reps: 1, due: day(6)
});

// Pointed Hebrew (qamats, shin-dot, patah, etnachta), emoji, a non-emoji
// astral (𝄞), and a high BMP character. UTF-16 order is astral, emoji, then
// U+F8FF; UTF-8 byte order is the reverse of that chain.
const pointed = '\u05E9\u05B8\u05C1\u05DE\u05B7\u0597\u05E2'.normalize('NFC');
const emojiWord = '\u05E9\u{1F600}';
const astralWord = '\u05E9\u{1D11E}';
const highBmp = '\u05E9\uF8FF';
review(pointed, 'he_en', { step: 1, last_result: 'pass', last_grade: 3, last_at: day(-1), sources: '["intro"]', due: day(2) });
review(emojiWord, 'he_en', { sources: '["intro"]', due: day(1) });
review(astralWord, 'he_en', { sources: '["intro"]', due: day(1) });
review(highBmp, 'he_en', { sources: '["intro"]', due: day(1) });

const lines = [];
function emit(row) {
    lines.push(JSON.stringify(row));
}

function columns(table) {
    return db.all("SELECT name FROM pragma_table_info('" + table + "') ORDER BY cid").map((row) => row.name);
}

const snap = path.join(dataDir, 'library.sqlite');
emit({
    kind: 'meta',
    db: snap,
    lessons: lessonsDir,
    now,
    learner,
    other,
    schema: {
        user_words: columns('user_words'),
        review_items: columns('review_items'),
        lexicon_entries: columns('lexicon_entries')
    }
});

const queries = [
    { id: 'all', query: { limit: '200' }, scheduler: 'ladder' },
    { id: 'fsrs', query: { limit: '200' }, scheduler: 'fsrs' },
    { id: 'q-hear', query: { q: 'hear', limit: '50' }, scheduler: 'ladder' },
    { id: 'q-note', query: { q: 'my note', limit: '50' }, scheduler: 'ladder' },
    { id: 'q-katav', query: { q: 'katav', limit: '50' }, scheduler: 'ladder' },
    { id: 'q-pointed', query: { q: ' \uFEFF ' + pointed + ' \t ', limit: '50' }, scheduler: 'ladder' },
    { id: 'q-emoji', query: { q: '\u{1F600}', limit: '50' }, scheduler: 'ladder' },
    { id: 'q-astral', query: { q: '\u{1D11E}', limit: '50' }, scheduler: 'ladder' },
    { id: 'q-emoji-run', query: { q: '\u{1F600}'.repeat(41), limit: '50' }, scheduler: 'ladder' },
    { id: 'bad-emoji-q', query: { q: '\u{1F600}'.repeat(81) }, scheduler: 'ladder' },
    { id: 'status-new', query: { status: 'new', limit: '50' }, scheduler: 'ladder' },
    { id: 'status-learning', query: { status: 'learning', limit: '50' }, scheduler: 'ladder' },
    { id: 'status-long', query: { status: 'longTerm', limit: '50' }, scheduler: 'ladder' },
    { id: 'weak-any', query: { weak: '1', limit: '50' }, scheduler: 'ladder' },
    { id: 'weak-none', query: { weak: '0', limit: '50' }, scheduler: 'ladder' },
    { id: 'weak-weak', query: { weak: 'weak', limit: '50' }, scheduler: 'ladder' },
    { id: 'weak-very', query: { weak: 'very_weak', limit: '50' }, scheduler: 'ladder' },
    { id: 'pos-verb', query: { pos: 'verb', limit: '50' }, scheduler: 'ladder' },
    { id: 'pos-noun', query: { pos: 'noun', limit: '50' }, scheduler: 'ladder' },
    { id: 'binyan-paal', query: { binyan: 'paal', limit: '50' }, scheduler: 'ladder' },
    { id: 'tense-past', query: { tense: 'past', limit: '50' }, scheduler: 'ladder' },
    { id: 'source-self', query: { source: 'self', limit: '50' }, scheduler: 'ladder' },
    { id: 'source-ai', query: { source: 'ai', limit: '50' }, scheduler: 'ladder' },
    { id: 'source-lesson', query: { source: 'lesson', limit: '50' }, scheduler: 'ladder' },
    { id: 'suspended', query: { suspended: '1', limit: '50' }, scheduler: 'ladder' },
    { id: 'not-suspended', query: { suspended: 'false', limit: '50' }, scheduler: 'ladder' },
    { id: 'other-user', userId: other, query: { limit: '50' }, scheduler: 'ladder' },
    { id: 'bad-status', query: { status: 'nope' }, scheduler: 'ladder' },
    { id: 'bad-limit', query: { limit: '0' }, scheduler: 'ladder' },
    { id: 'bad-weak', query: { weak: 'maybe' }, scheduler: 'ladder' },
    { id: 'bad-q', query: { q: 'a<b' }, scheduler: 'ladder' }
];

for (const item of queries) {
    process.env.SRS_SCHEDULER = item.scheduler;
    const result = library.list(item.userId || learner, item.query, now);
    emit({
        kind: 'list',
        id: item.id,
        userId: item.userId || learner,
        scheduler: item.scheduler,
        query: item.query,
        ok: result.ok === true,
        error: result.ok ? '' : result.error,
        status: result.ok ? 200 : result.status,
        words: result.ok ? result.words : []
    });
}

db.get('PRAGMA wal_checkpoint(TRUNCATE)');
fs.copyFileSync(path.join(dataDir, 'hebrew.sqlite'), snap);

function flags(userId) {
    return db.all(
        `SELECT word, direction, suspended FROM review_items
         WHERE user_id = ? ORDER BY word, direction`,
        userId
    ).map((row) => ({ word: row.word, direction: row.direction, suspended: Number(row.suspended) }));
}

function auditCount() {
    return Number(db.get('SELECT count(*) AS n FROM audit_log').n);
}

const suspends = [
    { id: 'resume', userId: learner, key: 'נח', suspended: false },
    { id: 'suspend-again', userId: learner, key: 'נח', suspended: true },
    { id: 'personal-only', userId: learner, key: 'מילה', suspended: true },
    { id: 'missing', userId: learner, key: 'אין', suspended: true },
    { id: 'bad-key', userId: learner, key: 'abc', suspended: true },
    { id: 'bad-user', userId: 0, key: 'נח', suspended: true },
    { id: 'trim-key', userId: learner, key: '  כתב  ', suspended: false },
    { id: 'pointed', userId: learner, key: ' \uFEFF ' + pointed + ' ', suspended: true }
];

for (const item of suspends) {
    const before = auditCount();
    const result = library.setSuspended(item.userId, item.key, item.suspended);
    emit({
        kind: 'suspend',
        id: item.id,
        userId: item.userId,
        key: item.key,
        suspended: item.suspended,
        ok: result.ok === true,
        error: result.ok ? '' : result.error,
        status: result.ok ? 200 : result.status,
        word: result.ok ? result.word : '',
        audit: auditCount(),
        auditBefore: before,
        flags: flags(learner)
    });
}

const added = library.add(learner, [{ key: 'חדש', gloss_en: 'new', translit: 'hadash' }], { source: 'self' });
const stored = db.get('SELECT * FROM user_words WHERE user_id = ? AND word = ?', learner, 'חדש');
emit({
    kind: 'add',
    ok: added.ok === true,
    error: added.ok ? '' : added.error,
    gloss_en: stored ? stored.gloss_en : null,
    columns: stored ? Object.keys(stored) : [],
    passedTranslit: 'hadash'
});

db.get('PRAGMA wal_checkpoint(TRUNCATE)');
db.close();
fs.writeFileSync(outPath, lines.join('\n') + '\n');
