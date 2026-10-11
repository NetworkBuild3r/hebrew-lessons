/**
 * Lexicon, concept registry, personal library, and concept mastery.
 * Temporary database only. Does not open data/hebrew.sqlite.
 * Provenance: INIT-006/SPEC-003
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

/**
 * ADR-0009 §4 key list, copied once. family:name, table order, 32 keys.
 * Future and imperative have tense keys and no cell keys.
 */
const ADR_0009_SECTION_4_KEYS = [
    'binyan:paal',
    'binyan:nifal',
    'binyan:piel',
    'binyan:pual',
    'binyan:hifil',
    'binyan:hufal',
    'binyan:hitpael',
    'tense:infinitive',
    'tense:present',
    'tense:past',
    'tense:future',
    'tense:imperative',
    'root-class:regular',
    'root-class:lamed-he',
    'root-class:peh-yod',
    'root-class:peh-nun',
    'root-class:ayin-vav',
    'root-class:irregular',
    'cell:present.ms',
    'cell:present.fs',
    'cell:present.mp',
    'cell:present.fp',
    'cell:past.1s',
    'cell:past.2ms',
    'cell:past.2fs',
    'cell:past.3ms',
    'cell:past.3fs',
    'cell:past.1p',
    'cell:past.2mp',
    'cell:past.2fp',
    'cell:past.3p',
    'cell:infinitive.inf'
];

/** 8 spellings × 60 characters, plus JSON quotes, commas, and brackets. */
const ACCEPT_JSON_MAX = 505;

const LEXICON_COLUMNS = [
    'key', 'kind', 'lemma_key', 'gloss_en', 'pos', 'root', 'binyan', 'root_class',
    'tense', 'cell', 'translit', 'accept_json', 'verified', 'verified_by',
    'source', 'rev', 'updated_at'
];

const NOW = '2026-10-10T00:00:00.000Z';

function freshModules(dataDirPath) {
    process.env.HEBREW_ROOT = root;
    process.env.DATA_DIR = dataDirPath;
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/repos/users',
        '../server/db/repos/lexicon',
        '../server/db/repos/concepts',
        '../server/db/repos/user-words',
        '../server/db/repos/concept-mastery'
    ];
    const migrationsDir = path.join(root, 'server', 'db', 'migrations');
    for (const file of fs.readdirSync(migrationsDir)) {
        if (/^\d{3,}-.+\.js$/.test(file)) {
            names.push('../server/db/migrations/' + file);
        }
    }
    for (const mod of names) {
        try {
            delete require.cache[require.resolve(mod)];
        } catch (_) { /* not loaded yet */ }
    }
    const db = require('../server/db');
    db.close();
    return {
        db,
        users: require('../server/db/repos/users'),
        lexicon: require('../server/db/repos/lexicon'),
        concepts: require('../server/db/repos/concepts'),
        userWords: require('../server/db/repos/user-words'),
        mastery: require('../server/db/repos/concept-mastery'),
        migration: require('../server/db/migrations/005-lexicon-library-mastery')
    };
}

function lexiconRow(overrides) {
    return Object.assign({
        key: 'חָשַׁב',
        kind: 'lemma',
        lemma_key: null,
        gloss_en: 'he thought',
        pos: 'verb',
        root: 'ח.ש.ב',
        binyan: 'paal',
        root_class: 'regular',
        tense: null,
        cell: null,
        translit: 'khashav',
        accept_json: '[]',
        verified: 0,
        verified_by: null,
        source: 'admin',
        rev: 0,
        updated_at: NOW
    }, overrides);
}

function expectCheck(fn) {
    assert.throws(fn, (err) => {
        assert.match(String(err && err.message), /CHECK constraint failed/i);
        return true;
    });
}

describe('lexicon, library, and concept mastery', () => {
    let tmp;
    let db;
    let users;
    let lexicon;
    let concepts;
    let userWords;
    let mastery;
    let migration;
    let userId;

    function insertLexicon(overrides) {
        const row = lexiconRow(overrides);
        db.run(
            'INSERT INTO lexicon_entries (' + LEXICON_COLUMNS.join(', ')
                + ') VALUES (' + LEXICON_COLUMNS.map(() => '?').join(', ') + ')',
            ...LEXICON_COLUMNS.map((name) => row[name])
        );
    }

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-lexicon-'));
        ({ db, users, lexicon, concepts, userWords, mastery, migration } = freshModules(tmp));
        db.migrate();
        assert.equal(
            path.resolve(db.databasePath()),
            path.resolve(tmp, 'hebrew.sqlite')
        );
        const created = users.create({
            username: 'lexicon-learner',
            password_hash: 'hash',
            display_name: 'Learner'
        });
        userId = Number(created.lastInsertRowid);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
    });

    it('ac-3: seeded grammar_concepts keys equal ADR-0009 §4', () => {
        const rows = concepts.list();
        assert.equal(rows.length, 32);
        assert.deepEqual(rows.map((row) => row.key), ADR_0009_SECTION_4_KEYS);
        assert.equal(new Set(rows.map((row) => row.key)).size, 32);

        const byKey = new Map(rows.map((row) => [row.key, row]));
        assert.equal(byKey.get('binyan:paal').family, 'binyan');
        assert.equal(byKey.get('binyan:paal').label_he, 'פָּעַל');
        assert.equal(byKey.get('binyan:paal').colour, null);
        assert.equal(byKey.get('tense:infinitive').label_he, 'שם פועל');
        assert.equal(byKey.get('root-class:regular').colour, 'green');
        assert.equal(byKey.get('root-class:regular').label_he, 'שלם');
        assert.equal(byKey.get('root-class:lamed-he').colour, 'yellow');
        assert.equal(byKey.get('root-class:lamed-he').label_he, 'ל״ה');
        assert.equal(byKey.get('root-class:peh-yod').colour, 'red');
        assert.equal(byKey.get('root-class:peh-yod').label_he, 'פ״י');
        assert.equal(byKey.get('root-class:peh-nun').label_he, 'פ״נ');
        assert.equal(byKey.get('root-class:ayin-vav').label_he, 'ע״ו');
        assert.equal(byKey.get('root-class:irregular').colour, 'red');
        assert.equal(byKey.get('root-class:irregular').label_he, 'חריג');
        assert.equal(byKey.get('cell:past.2ms').label_he, 'אתה');
        assert.equal(byKey.get('cell:past.3p').label_he, 'הם, הן');
        assert.equal(byKey.get('cell:present.ms').label_he, null);
        assert.equal(byKey.get('cell:infinitive.inf').label_he, null);
        assert.equal(byKey.get('cell:present.ms').colour, null);
        assert.equal(byKey.get('tense:future').family, 'tense');
        assert.equal(byKey.has('cell:future.1s'), false);
        assert.equal(byKey.has('cell:imperative.ms'), false);

        for (const row of rows) {
            if (row.family === 'root-class') {
                assert.ok(row.colour === 'green' || row.colour === 'yellow' || row.colour === 'red');
            } else {
                assert.equal(row.colour, null);
            }
            assert.equal(row.key.startsWith(row.family + ':'), true);
        }
    });

    it('ac-1: enum columns and length limits reject out-of-registry values', () => {
        insertLexicon({ key: 'חָשַׁב' });
        const stored = db.get(
            'SELECT kind, pos, binyan, gloss_en, accept_json FROM lexicon_entries WHERE key = ?',
            'חָשַׁב'
        );
        assert.equal(stored.kind, 'lemma');
        assert.equal(stored.pos, 'verb');
        assert.equal(stored.binyan, 'paal');
        assert.equal(stored.gloss_en, 'he thought');
        assert.equal(stored.accept_json, '[]');

        const badEnums = [
            { kind: 'word' },
            { pos: 'interjection' },
            { binyan: 'qal' },
            { root_class: 'hollow' },
            { tense: 'gerund' },
            { cell: 'ms' },
            { tense: 'present', cell: '1s' },
            { tense: 'future', cell: 'ms' },
            { tense: 'imperative', cell: 'ms' },
            { source: 'user' },
            { verified: 2 }
        ];
        badEnums.forEach((overrides, index) => {
            expectCheck(() => insertLexicon(Object.assign({ key: 'ק' + index }, overrides)));
        });

        insertLexicon({
            key: 'יָשַׁב',
            kind: 'form',
            lemma_key: 'חָשַׁב',
            tense: 'past',
            cell: '3ms'
        });
        expectCheck(() => insertLexicon({
            key: 'יָשְׁבָה',
            kind: 'form',
            lemma_key: null
        }));
        expectCheck(() => insertLexicon({
            key: 'בֹּקֶר טוֹב',
            kind: 'lemma'
        }));
        insertLexicon({
            key: 'בֹּקֶר טוֹב',
            kind: 'phrase',
            pos: null,
            root: null,
            binyan: null,
            root_class: null
        });
        expectCheck(() => insertLexicon({
            key: 'שלום',
            kind: 'phrase'
        }));

        insertLexicon({ key: 'אוֹר', gloss_en: 'א'.repeat(120) });
        expectCheck(() => insertLexicon({ key: 'אוֹרָה', gloss_en: 'א'.repeat(121) }));
        expectCheck(() => insertLexicon({ key: 'כֶּלֶב', gloss_en: 'a <b> dog', pos: 'noun', root: null, binyan: null, root_class: null }));
        expectCheck(() => insertLexicon({ key: 'חָתוּל', gloss_en: 'a\u0001b', pos: 'noun', root: null, binyan: null, root_class: null }));

        const spelling = 'א'.repeat(60);
        const packed = JSON.stringify([
            spelling, spelling, spelling, spelling,
            spelling, spelling, spelling, spelling
        ]);
        assert.equal(packed.length, ACCEPT_JSON_MAX);
        insertLexicon({ key: 'מַיִם', accept_json: packed, pos: 'noun', root: null, binyan: null, root_class: null });
        expectCheck(() => insertLexicon({
            key: 'נַחַל',
            accept_json: packed + 'x',
            pos: 'noun',
            root: null,
            binyan: null,
            root_class: null
        }));
        expectCheck(() => insertLexicon({
            key: 'נָהָר',
            accept_json: JSON.stringify(['א'.repeat(61)]),
            pos: 'noun',
            root: null,
            binyan: null,
            root_class: null
        }));
        expectCheck(() => insertLexicon({
            key: 'יָם',
            accept_json: JSON.stringify(Array(9).fill('א')),
            pos: 'noun',
            root: null,
            binyan: null,
            root_class: null
        }));
        expectCheck(() => insertLexicon({
            key: 'אַיִל',
            accept_json: JSON.stringify(['אַ']),
            pos: 'noun',
            root: null,
            binyan: null,
            root_class: null
        }));
        expectCheck(() => insertLexicon({ key: 'עֵץ', translit: 'x'.repeat(81), pos: 'noun', root: null, binyan: null, root_class: null }));
        expectCheck(() => insertLexicon({ key: 'x'.repeat(61) }));

        userWords.add({
            user_id: userId,
            word: 'סֵפֶר',
            source: 'self',
            added_at: NOW,
            gloss_en: 'a'.repeat(100)
        });
        expectCheck(() => userWords.add({
            user_id: userId,
            word: 'סְפָרִים',
            source: 'self',
            added_at: NOW,
            gloss_en: 'a'.repeat(101)
        }));
        expectCheck(() => userWords.add({
            user_id: userId,
            word: 'עֵט',
            source: 'mcp',
            added_at: NOW
        }));
        assert.throws(() => mastery.upsert({
            user_id: userId,
            concept_key: 'binyan:nope',
            attempts: 1,
            correct: 0,
            ewma: 0,
            updated_at: NOW
        }), /FOREIGN KEY constraint failed/i);

        expectCheck(() => db.run(
            `INSERT INTO grammar_concepts (key, family, label_en, label_he, colour, sort)
             VALUES (?, ?, ?, ?, ?, ?)`,
            'mood:indicative',
            'mood',
            'indicative',
            null,
            null,
            100
        ));
        expectCheck(() => db.run(
            `INSERT INTO grammar_concepts (key, family, label_en, label_he, colour, sort)
             VALUES (?, ?, ?, ?, ?, ?)`,
            'root-class:extra',
            'root-class',
            'extra',
            'נוסף',
            'blue',
            101
        ));
        expectCheck(() => db.run(
            `INSERT INTO grammar_concepts (key, family, label_en, label_he, colour, sort)
             VALUES (?, ?, ?, ?, ?, ?)`,
            'binyan:extra',
            'binyan',
            'extra',
            null,
            'green',
            102
        ));
    });

    it('stores an NFD key as NFC through the repository', () => {
        const nfd = '\u05E9\u0065\u0301';
        const nfc = nfd.normalize('NFC');
        assert.notEqual(nfd, nfc);

        lexicon.upsert({
            key: nfd,
            kind: 'lemma',
            lemma_key: null,
            gloss_en: 'nfc lemma',
            pos: 'noun',
            root: null,
            binyan: null,
            root_class: null,
            source: 'admin',
            updated_at: NOW
        });
        const found = lexicon.find(nfd);
        assert.equal(found.key, nfc);
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', nfc).key, nfc);
        assert.equal(db.get('SELECT key FROM lexicon_entries WHERE key = ?', nfd), undefined);

        const formNfd = '\u05D1\u0065\u0301';
        const formNfc = formNfd.normalize('NFC');
        lexicon.upsert({
            key: formNfd,
            kind: 'form',
            lemma_key: nfd,
            gloss_en: 'nfc form',
            pos: 'noun',
            source: 'mcp',
            updated_at: NOW
        });
        const form = lexicon.find(formNfc);
        assert.equal(form.key, formNfc);
        assert.equal(form.lemma_key, nfc);

        userWords.add({
            user_id: userId,
            word: nfd,
            source: 'lesson',
            added_at: NOW,
            gloss_en: 'my note'
        });
        assert.equal(userWords.find(userId, nfd).word, nfc);
        assert.equal(
            db.get('SELECT word FROM user_words WHERE user_id = ? AND word = ?', userId, nfc).word,
            nfc
        );
    });

    it('ac-2: deleting a user removes library and mastery rows and no lexicon row', () => {
        const learner = users.create({
            username: 'cascade-learner',
            password_hash: 'hash'
        });
        const other = users.create({
            username: 'cascade-other',
            password_hash: 'hash'
        });
        const learnerId = Number(learner.lastInsertRowid);
        const otherId = Number(other.lastInsertRowid);

        const lexiconBefore = db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n;
        insertLexicon({
            key: 'דֶּלֶת',
            pos: 'noun',
            root: null,
            binyan: null,
            root_class: null,
            gloss_en: 'door'
        });
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n, lexiconBefore + 1);

        userWords.add({
            user_id: learnerId,
            word: 'דֶּלֶת',
            source: 'ai',
            added_at: NOW,
            gloss_en: 'a door'
        });
        userWords.add({
            user_id: otherId,
            word: 'דֶּלֶת',
            source: 'self',
            added_at: NOW
        });
        mastery.upsert({
            user_id: learnerId,
            concept_key: 'binyan:paal',
            attempts: 2,
            correct: 1,
            ewma: 0.5,
            updated_at: NOW
        });
        mastery.upsert({
            user_id: otherId,
            concept_key: 'tense:past',
            attempts: 1,
            correct: 1,
            ewma: 1,
            updated_at: NOW
        });

        users.deleteById(learnerId);

        assert.equal(userWords.listByUser(learnerId).length, 0);
        assert.equal(mastery.listByUser(learnerId).length, 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM user_words WHERE user_id = ?', learnerId).n, 0);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM concept_mastery WHERE user_id = ?', learnerId).n, 0);
        assert.equal(userWords.find(otherId, 'דֶּלֶת').source, 'self');
        assert.equal(mastery.find(otherId, 'tense:past').attempts, 1);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n, lexiconBefore + 1);
        assert.equal(db.get('SELECT gloss_en FROM lexicon_entries WHERE key = ?', 'דֶּלֶת').gloss_en, 'door');
        assert.equal(concepts.list().length, 32);
    });

    it('ac-4: down drops the four tables and leaves earlier row counts unchanged', () => {
        db.run(
            `INSERT INTO review_items (user_id, word, direction, due)
             VALUES (?, ?, ?, ?)`,
            userId,
            'שלום',
            'he_en',
            NOW
        );
        db.run(
            `INSERT INTO practice_events (id, user_id, word, direction, result, at)
             VALUES (?, ?, ?, ?, ?, ?)`,
            'evt-lexicon',
            userId,
            'שלום',
            'he_en',
            'pass',
            NOW
        );
        db.run(
            `INSERT INTO lesson_progress (user_id, slug, state_json, rev, updated_at)
             VALUES (?, ?, ?, ?, ?)`,
            userId,
            'lexicon-unit',
            '{}',
            0,
            NOW
        );

        const counts = {
            users: db.get('SELECT COUNT(*) AS n FROM users').n,
            review_items: db.get('SELECT COUNT(*) AS n FROM review_items').n,
            practice_events: db.get('SELECT COUNT(*) AS n FROM practice_events').n,
            lesson_progress: db.get('SELECT COUNT(*) AS n FROM lesson_progress').n
        };
        assert.ok(counts.users >= 1);
        assert.ok(counts.review_items >= 1);
        assert.ok(counts.practice_events >= 1);
        assert.ok(counts.lesson_progress >= 1);
        assert.ok(db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n >= 1);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM grammar_concepts').n, 32);

        const tablesBefore = db.all(
            `SELECT name FROM sqlite_master
             WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
        ).map((row) => row.name);
        for (const name of ['lexicon_entries', 'grammar_concepts', 'user_words', 'concept_mastery']) {
            assert.ok(tablesBefore.includes(name), name);
        }

        migration.down(db.open());

        const tablesAfter = db.all(
            `SELECT name FROM sqlite_master
             WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
        ).map((row) => row.name);
        for (const name of ['lexicon_entries', 'grammar_concepts', 'user_words', 'concept_mastery']) {
            assert.equal(tablesAfter.includes(name), false, name);
        }
        assert.equal(db.get('SELECT COUNT(*) AS n FROM users').n, counts.users);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM review_items').n, counts.review_items);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM practice_events').n, counts.practice_events);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_progress').n, counts.lesson_progress);
        assert.equal(
            db.get('SELECT COUNT(*) AS n FROM schema_migrations WHERE version = 5').n,
            0
        );
        assert.equal(
            db.get('SELECT COUNT(*) AS n FROM schema_migrations WHERE version = 1').n,
            1
        );

        db.migrate();
        assert.equal(db.get('SELECT COUNT(*) AS n FROM grammar_concepts').n, 32);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM users').n, counts.users);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM review_items').n, counts.review_items);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM practice_events').n, counts.practice_events);
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lesson_progress').n, counts.lesson_progress);
        const restored = db.get(
            'SELECT name FROM schema_migrations WHERE version = 5'
        );
        assert.match(restored.name, /^005-lexicon-library-mastery\.js$/);
    });

    it('repositories use bound parameters and do not read the filesystem', () => {
        const hostile = "ש';DROPTABLEusers;--";
        lexicon.upsert({
            key: hostile,
            kind: 'lemma',
            gloss_en: hostile,
            pos: 'other',
            root: null,
            binyan: null,
            root_class: null,
            source: 'seed',
            updated_at: NOW
        });
        assert.equal(lexicon.find(hostile).gloss_en, hostile);
        assert.ok(db.get('SELECT COUNT(*) AS n FROM users').n >= 1);

        const files = [
            'server/db/migrations/005-lexicon-library-mastery.js',
            'server/db/repos/lexicon.js',
            'server/db/repos/concepts.js',
            'server/db/repos/user-words.js',
            'server/db/repos/concept-mastery.js'
        ];
        for (const rel of files) {
            const source = fs.readFileSync(path.join(root, rel), 'utf8');
            assert.equal(/\brequire\(/.test(source), rel.startsWith('server/db/repos/'), rel);
            assert.equal(/node:fs|require\('fs'\)|require\("fs"\)/.test(source), false, rel);
            assert.equal(/LESSONS_DIR|worksheets/.test(source), false, rel);
        }
    });
});
