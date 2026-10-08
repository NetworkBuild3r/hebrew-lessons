/**
 * Per-user catalog / vocab / word-audio content filtering.
 * Provenance: INIT-002/SPEC-006
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

const EVERYONE = 'public-lesson';
const SELECTED = 'selected-lesson';
const HIDDEN = 'hidden-lesson';

const SHARED_WORD = 'מִשְׁתָּף';
const PUBLIC_ONLY = 'פֻּבְּלִי';
const SELECTED_ONLY = 'סֶלֶקְט';
const HIDDEN_ONLY = 'הִידֵן';

function writeWorksheet(dir, slug, title, words) {
    const lesson = path.join(dir, slug);
    fs.mkdirSync(lesson, { recursive: true });
    const items = words.map((word, i) => ({
        id: String(i + 1),
        word,
        meaning: 'meaning-' + word,
        sound: '',
        audio: ''
    }));
    const worksheet = {
        id: slug,
        title,
        level: 'basic',
        vocab: { items },
        reading: { paragraphs: [words.join(' ')] },
        activities: [{ type: 'words' }]
    };
    fs.writeFileSync(
        path.join(lesson, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/repos/visibility',
        '../server/db/repos/audit',
        '../server/db/repos/users',
        '../server/auth/policy',
        '../server/visibility',
        '../server/content-filter',
        '../server/lesson-data'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

function parseScript(body, expectedName) {
    const { parseDataWindow } = require('../server/lesson-data');
    const globals = parseDataWindow(body, '<filtered>');
    assert.ok(Object.prototype.hasOwnProperty.call(globals, expectedName));
    return globals[expectedName];
}

describe('content-filter (SPEC-006 ac-1..ac-5)', () => {
    let tmpData;
    let tmpLessons;
    let tmpRoot;
    let db;
    let filter;
    let visibilityRepo;
    let users;
    let learner;
    let admin;
    let grantedUser;

    before(() => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-cf-data-'));
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-cf-lessons-'));
        tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-cf-root-'));

        writeWorksheet(tmpLessons, EVERYONE, 'Public Title', [SHARED_WORD, PUBLIC_ONLY]);
        writeWorksheet(tmpLessons, SELECTED, 'Selected Secret', [SHARED_WORD, SELECTED_ONLY]);
        writeWorksheet(tmpLessons, HIDDEN, 'Hidden Secret', [HIDDEN_ONLY]);

        const catalog = [
            {
                slug: EVERYONE,
                title: 'Public Title with </script> and "quotes"',
                subtitle: '',
                blurb: 'blurb with </script> and \'quotes\'',
                level: 'basic'
            },
            { slug: SELECTED, title: 'Selected Secret', subtitle: '', blurb: 'selected blurb', level: 'basic' },
            { slug: HIDDEN, title: 'Hidden Secret', subtitle: '', blurb: 'hidden blurb', level: 'basic' }
        ];

        fs.writeFileSync(
            path.join(tmpLessons, 'catalog.js'),
            'window.HEBREW_CATALOG = ' + JSON.stringify(catalog, null, 4) + ';\n',
            'utf8'
        );

        const vocabIndex = [
            {
                key: SHARED_WORD,
                word: SHARED_WORD,
                meaning: 'shared',
                sound: '',
                audio: '',
                sentence: '',
                sentenceAudio: '',
                lessons: [EVERYONE, SELECTED]
            },
            {
                key: PUBLIC_ONLY,
                word: PUBLIC_ONLY,
                meaning: 'public-only',
                sound: '',
                audio: '',
                sentence: '',
                sentenceAudio: '',
                lessons: [EVERYONE]
            },
            {
                key: SELECTED_ONLY,
                word: SELECTED_ONLY,
                meaning: 'selected-only',
                sound: '',
                audio: '',
                sentence: '',
                sentenceAudio: '',
                lessons: [SELECTED]
            },
            {
                key: HIDDEN_ONLY,
                word: HIDDEN_ONLY,
                meaning: 'hidden-only',
                sound: '',
                audio: '',
                sentence: '',
                sentenceAudio: '',
                lessons: [HIDDEN]
            }
        ];
        fs.mkdirSync(path.join(tmpLessons, '_review'), { recursive: true });
        fs.writeFileSync(
            path.join(tmpLessons, '_review', 'vocab-index.js'),
            'window.HEBREW_VOCAB_INDEX = ' + JSON.stringify(vocabIndex, null, 4) + ';\n',
            'utf8'
        );

        const wordAudio = {
            [SHARED_WORD]: '../_audio/words/shared.mp3',
            [PUBLIC_ONLY]: '../_audio/words/public.mp3',
            [SELECTED_ONLY]: '../_audio/words/selected.mp3',
            [HIDDEN_ONLY]: '../_audio/words/hidden.mp3'
        };
        fs.mkdirSync(path.join(tmpLessons, '_audio', 'words'), { recursive: true });
        fs.writeFileSync(
            path.join(tmpLessons, '_audio', 'words', 'catalog.js'),
            'window.HEBREW_WORD_AUDIO = Object.assign({}, window.HEBREW_WORD_AUDIO || {}, '
                + JSON.stringify(wordAudio) + ');\n',
            'utf8'
        );

        // HEBREW_ROOT only for shared/word-audio-catalog.js; server modules still load from repo.
        fs.mkdirSync(path.join(tmpRoot, 'shared'), { recursive: true });
        fs.writeFileSync(
            path.join(tmpRoot, 'shared', 'word-audio-catalog.js'),
            'window.HEBREW_WORD_AUDIO = Object.assign({}, window.HEBREW_WORD_AUDIO || {}, '
                + JSON.stringify(wordAudio) + ');\n',
            'utf8'
        );

        process.env.HEBREW_ROOT = tmpRoot;
        process.env.DATA_DIR = tmpData;
        process.env.LESSONS_DIR = tmpLessons;
        clearModules();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        visibilityRepo = require('../server/db/repos/visibility');
        users = require('../server/db/repos/users');
        filter = require('../server/content-filter');

        learner = users.findById(Number(users.create({
            username: 'cf-learner',
            password_hash: 'x',
            role: 'user'
        }).lastInsertRowid));
        admin = users.findById(Number(users.create({
            username: 'cf-admin',
            password_hash: 'x',
            role: 'admin'
        }).lastInsertRowid));
        grantedUser = users.findById(Number(users.create({
            username: 'cf-granted',
            password_hash: 'x',
            role: 'user'
        }).lastInsertRowid));

        visibilityRepo.setVisibility(EVERYONE, 'everyone');
        visibilityRepo.setVisibility(SELECTED, 'selected');
        visibilityRepo.setVisibility(HIDDEN, 'hidden');
        visibilityRepo.grant(SELECTED, grantedUser.id);
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmpData, { recursive: true, force: true });
        fs.rmSync(tmpLessons, { recursive: true, force: true });
        if (tmpRoot) fs.rmSync(tmpRoot, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete process.env.LESSONS_DIR;
        delete process.env.HEBREW_ROOT;
        clearModules();
    });

    function assertNoLeak(body, restrictedSlugs, restrictedTitles, restrictedWords) {
        for (const slug of restrictedSlugs) {
            assert.ok(!body.includes(slug), 'leaked slug ' + slug);
        }
        for (const title of restrictedTitles) {
            assert.ok(!body.includes(title), 'leaked title ' + title);
        }
        for (const word of restrictedWords) {
            assert.ok(!body.includes(word), 'leaked word ' + word);
        }
    }

    it('ac-1: non-granted user sees none of selected/hidden slug, title, or unique words in all four scripts', () => {
        filter.clearScriptCache();
        const restrictedSlugs = [SELECTED, HIDDEN];
        const restrictedTitles = ['Selected Secret', 'Hidden Secret'];
        const restrictedWords = [SELECTED_ONLY, HIDDEN_ONLY];

        const catalogBody = filter.catalogScriptFor(learner);
        assertNoLeak(catalogBody, restrictedSlugs, restrictedTitles, restrictedWords);
        assert.ok(catalogBody.includes(EVERYONE));
        assert.ok(catalogBody.includes(PUBLIC_ONLY) === false); // catalog has no vocab words
        const catalog = parseScript(catalogBody, 'HEBREW_CATALOG');
        assert.equal(catalog.length, 1);
        assert.equal(catalog[0].slug, EVERYONE);

        const vocabBody = filter.vocabIndexScriptFor(learner);
        assertNoLeak(vocabBody, restrictedSlugs, restrictedTitles, restrictedWords);
        assert.ok(vocabBody.includes(SHARED_WORD));
        assert.ok(vocabBody.includes(PUBLIC_ONLY));
        const vocab = parseScript(vocabBody, 'HEBREW_VOCAB_INDEX');
        const shared = vocab.find((e) => e.key === SHARED_WORD);
        assert.ok(shared);
        assert.deepEqual(shared.lessons, [EVERYONE]);

        const wordsBody = filter.wordAudioScriptFor(learner, 'words');
        assertNoLeak(wordsBody, restrictedSlugs, restrictedTitles, restrictedWords);
        const words = parseScript(wordsBody, 'HEBREW_WORD_AUDIO');
        assert.ok(words[SHARED_WORD]);
        assert.ok(words[PUBLIC_ONLY]);
        assert.equal(words[SELECTED_ONLY], undefined);
        assert.equal(words[HIDDEN_ONLY], undefined);

        const sharedBody = filter.wordAudioScriptFor(learner, 'shared');
        assertNoLeak(sharedBody, restrictedSlugs, restrictedTitles, restrictedWords);
        const sharedMap = parseScript(sharedBody, 'HEBREW_WORD_AUDIO');
        assert.ok(sharedMap[SHARED_WORD]);
        assert.equal(sharedMap[SELECTED_ONLY], undefined);
        assert.equal(sharedMap[HIDDEN_ONLY], undefined);
    });

    it('ac-2: granted user and admin see the selected lesson in all four scripts', () => {
        filter.clearScriptCache();

        for (const user of [grantedUser, admin]) {
            const catalog = parseScript(filter.catalogScriptFor(user), 'HEBREW_CATALOG');
            assert.ok(catalog.some((e) => e.slug === SELECTED), 'catalog missing selected for ' + user.username);

            const vocab = parseScript(filter.vocabIndexScriptFor(user), 'HEBREW_VOCAB_INDEX');
            const selectedEntry = vocab.find((e) => e.key === SELECTED_ONLY);
            assert.ok(selectedEntry, 'vocab missing selected-only for ' + user.username);
            assert.ok(selectedEntry.lessons.includes(SELECTED));

            const words = parseScript(filter.wordAudioScriptFor(user, 'words'), 'HEBREW_WORD_AUDIO');
            assert.ok(words[SELECTED_ONLY], 'word-audio missing selected-only for ' + user.username);

            const sharedMap = parseScript(filter.wordAudioScriptFor(user, 'shared'), 'HEBREW_WORD_AUDIO');
            assert.ok(sharedMap[SELECTED_ONLY]);
        }

        const adminCatalog = parseScript(filter.catalogScriptFor(admin), 'HEBREW_CATALOG');
        assert.ok(adminCatalog.some((e) => e.slug === HIDDEN));
        const learnerCatalog = parseScript(filter.catalogScriptFor(learner), 'HEBREW_CATALOG');
        assert.ok(!learnerCatalog.some((e) => e.slug === HIDDEN));
    });

    it('ac-3: shared word stays with only the visible lesson in lessons list', () => {
        filter.clearScriptCache();
        const vocab = parseScript(filter.vocabIndexScriptFor(learner), 'HEBREW_VOCAB_INDEX');
        const shared = vocab.find((e) => e.key === SHARED_WORD);
        assert.ok(shared);
        assert.deepEqual(shared.lessons, [EVERYONE]);
        assert.ok(!shared.lessons.includes(SELECTED));

        const words = parseScript(filter.wordAudioScriptFor(learner, 'words'), 'HEBREW_WORD_AUDIO');
        assert.ok(words[SHARED_WORD]);
    });

    it('ac-4: filtered output re-parses via lesson-data; </script> and quotes are JSON-safe', () => {
        filter.clearScriptCache();
        const body = filter.catalogScriptFor(learner);
        assert.ok(body.includes('\\u003c'), 'expected < escaped so </script> cannot break HTML');
        assert.ok(!/<\/script>/i.test(body), 'raw </script> must not appear in emitted script');
        assert.ok(body.includes('\\"'), 'quotes must be JSON-escaped');
        const parsed = parseScript(body, 'HEBREW_CATALOG');
        assert.ok(parsed[0].title.includes('</script>'));
        assert.ok(parsed[0].title.includes('"'));
        assert.ok(body.startsWith('window.HEBREW_CATALOG = '));
        assert.ok(body.trimEnd().endsWith(';'));
    });

    it('ac-5: malformed source yields empty dataset (fail closed)', () => {
        filter.clearScriptCache();
        const catalogPath = path.join(tmpLessons, 'catalog.js');
        const backup = fs.readFileSync(catalogPath, 'utf8');
        fs.writeFileSync(catalogPath, 'window.HEBREW_CATALOG = (function(){ return []; })();\n', 'utf8');
        try {
            const body = filter.catalogScriptFor(learner);
            assert.equal(body, 'window.HEBREW_CATALOG = [];\n');
            const parsed = parseScript(body, 'HEBREW_CATALOG');
            assert.deepEqual(parsed, []);
            assert.ok(!body.includes('SELECTED') && !body.includes(SELECTED));
        } finally {
            fs.writeFileSync(catalogPath, backup, 'utf8');
            filter.clearScriptCache();
        }
    });
});
