/**
 * Learner library, flashcard, lexicon-read, and graded-signal routes.
 * Provenance: INIT-006/SPEC-009
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');

const harness = require('./helpers/signed-in-server');

const REVIEWED = ['אָב', 'אֵם', 'בַּיִת', 'יֶלֶד', 'לִכְתֹּב'];
const ADDED = ['שָׁלוֹם', 'עוֹלָם'];
const VERB = 'לִכְתֹּב';

describe('library and card routes (INIT-006/SPEC-009)', () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(u: string) => string} */
    let cookieFor;
    /** @type {Record<string, { id: number, username: string, role: string, token: string }>} */
    let users;
    /** @type {typeof import('../server/db')} */
    let db;
    /** @type {typeof import('../server/db/repos/review')} */
    let reviewRepo;
    /** @type {typeof import('../server/db/repos/user-words')} */
    let userWords;
    /** @type {typeof import('../server/lexicon/service')} */
    let lexicon;
    /** @type {typeof import('../server/vocab-index').exactKey} */
    let exactKey;

    before(async () => {
        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        users = started.users;
        db = require('../server/db');
        reviewRepo = require('../server/db/repos/review');
        userWords = require('../server/db/repos/user-words');
        lexicon = require('../server/lexicon/service');
        exactKey = require('../server/vocab-index').exactKey;
    });

    after(async () => {
        await harness.stop();
    });

    function jsonHeaders(username, extra) {
        const headers = {
            'Content-Type': 'application/json',
            Origin: baseUrl
        };
        if (username) headers.Cookie = cookieFor(username);
        return Object.assign(headers, extra || {});
    }

    async function api(method, path, username, body, opts) {
        const options = opts || {};
        const headers = jsonHeaders(username);
        if (options.origin === false) delete headers.Origin;
        const init = { method, headers };
        if (body !== undefined) {
            init.body = typeof body === 'string' ? body : JSON.stringify(body);
        }
        const res = await fetch(baseUrl + path, init);
        const text = await res.text();
        let json = null;
        if (text) {
            try { json = JSON.parse(text); } catch (_) { json = null; }
        }
        return { status: res.status, json, text };
    }

    function signal(partial) {
        return Object.assign({
            id: 'sig-' + Math.random().toString(36).slice(2),
            word: 'אָב',
            direction: 'he_en',
            result: 'pass',
            grade: 3,
            kind: 'word',
            at: new Date().toISOString(),
            slug: 'public-lesson',
            activity: 'words'
        }, partial);
    }

    /**
     * @param {string} word
     * @param {object} [extra]
     */
    function lemma(word, extra) {
        return Object.assign({
            key: exactKey(word),
            kind: 'lemma',
            gloss_en: 'gloss-' + word,
            pos: 'noun'
        }, extra || {});
    }

    it('ac-1: five reviewed words and two added words are seven tagged rows', async () => {
        const entries = REVIEWED.map((word) => {
            if (word === VERB) {
                return lemma(word, {
                    gloss_en: 'to write',
                    pos: 'verb',
                    binyan: 'paal',
                    root: 'כ.ת.ב'
                });
            }
            return lemma(word);
        });
        entries.push(lemma('שָׁלוֹם', { gloss_en: 'peace', pos: 'noun' }));
        entries.push(lemma('עוֹלָם', { gloss_en: 'world', pos: 'noun' }));
        const wrote = lexicon.upsert(entries, { actor: 'admin:test', expectedRev: 0 });
        assert.equal(wrote.ok, true, JSON.stringify(wrote));

        const signals = REVIEWED.map((word, index) => signal({
            id: 'ac1-' + index,
            word,
            result: 'pass',
            grade: 3
        }));
        const graded = await api('POST', '/api/review/signals', 'learner', { signals });
        assert.equal(graded.status, 200);
        assert.equal(graded.json.applied, 5);

        const added = await api('POST', '/api/me/library', 'learner', {
            words: [
                { key: 'שָׁלוֹם', gloss_en: 'should-be-ignored' },
                { key: 'עוֹלָם', gloss_en: 'should-be-ignored' }
            ]
        });
        assert.equal(added.status, 200);
        assert.equal(added.json.created, 2);

        const listed = await api('GET', '/api/me/library', 'learner');
        assert.equal(listed.status, 200);
        assert.equal(listed.json.words.length, 7);

        const byWord = new Map(listed.json.words.map((row) => [row.word, row]));
        for (const word of REVIEWED) {
            const row = byWord.get(exactKey(word));
            assert.ok(row, word);
            assert.equal(row.he_en && row.he_en.stage, 'learning', word);
            assert.equal(row.en_he, null, word);
            assert.equal(row.pos, word === VERB ? 'verb' : 'noun');
            assert.equal(row.suspended, false);
        }
        const verb = byWord.get(exactKey(VERB));
        assert.equal(verb.binyan, 'paal');
        assert.equal(verb.root, 'כ.ת.ב');
        assert.equal(verb.gloss_en, 'to write');

        for (const word of ADDED) {
            const row = byWord.get(exactKey(word));
            assert.ok(row, word);
            assert.equal(row.he_en.stage, 'new');
            assert.equal(row.en_he.stage, 'new');
            assert.equal(row.sources.includes('self'), true);
            assert.equal(userWords.find(users.learner.id, word).gloss_en, null);
        }
        const shalom = byWord.get(exactKey('שָׁלוֹם'));
        assert.equal(shalom.gloss_en, 'peace');
        assert.equal(shalom.note, null);
        assert.equal(shalom.audio, 'a.mp3');
        assert.equal(shalom.pos, 'noun');
    });

    it('ac-2: origin, idempotent add, personal gloss, then lexicon gloss plus note', async () => {
        const blocked = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: 'חָבֵר', gloss_en: 'friend note' }]
        }, { origin: false });
        assert.equal(blocked.status, 403);
        assert.equal(blocked.json.code, 'csrf');
        assert.equal(userWords.find(users.learner.id, 'חָבֵר'), undefined);

        const missing = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: 'כֶּלֶב' }]
        });
        assert.equal(missing.status, 400);
        assert.equal(missing.json.error, 'gloss_required');
        assert.equal(userWords.find(users.learner.id, 'כֶּלֶב'), undefined);

        const marked = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: 'כֶּלֶב', gloss_en: 'a <dog>' }]
        });
        assert.equal(marked.status, 400);
        assert.equal(marked.json.error, 'invalid_gloss');

        const bare = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: 'hello', gloss_en: 'no' }]
        });
        assert.equal(bare.status, 400);
        assert.equal(bare.json.error, 'invalid_key');

        const before = db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n;
        const created = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: 'חָבֵר', gloss_en: 'friend note' }]
        });
        assert.equal(created.status, 200);
        assert.equal(created.json.created, 1);
        const after = db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n;
        assert.equal(after, before);

        const key = exactKey('חָבֵר');
        const directions = reviewRepo.listByUser(users.learner.id).filter((row) => row.word === key);
        assert.equal(directions.length, 2);
        assert.deepEqual(directions.map((row) => row.direction).sort(), ['en_he', 'he_en']);
        assert.equal(directions.every((row) => row.fsrs_state === 'new'), true);
        assert.equal(userWords.find(users.learner.id, key).gloss_en, 'friend note');

        const again = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: 'חָבֵר', gloss_en: 'friend note' }]
        });
        assert.equal(again.status, 200);
        assert.equal(again.json.created, 0);
        const still = reviewRepo.listByUser(users.learner.id).filter((row) => row.word === key);
        assert.equal(still.length, 2);

        const promoted = lexicon.upsert(
            [lemma('חָבֵר', { gloss_en: 'friend', pos: 'noun' })],
            { actor: 'admin:test', expectedRev: 0 }
        );
        assert.equal(promoted.ok, true, JSON.stringify(promoted));
        const listed = await api('GET', '/api/me/library', 'learner');
        const row = listed.json.words.find((item) => item.word === key);
        assert.ok(row);
        assert.equal(row.gloss_en, 'friend');
        assert.equal(row.note, 'friend note');
        assert.equal(db.get('SELECT COUNT(*) AS n FROM lexicon_entries').n, before + 1);
    });

    it('ac-3: weak deck is only weak or very weak words, each with four intervals', async () => {
        const weakWord = exactKey('חָתוּל');
        const veryWeakWord = exactKey('דָּג');
        const steadyWord = exactKey('עֵץ');
        const planted = lexicon.upsert([
            lemma(weakWord, { gloss_en: 'cat' }),
            lemma(veryWeakWord, { gloss_en: 'fish' }),
            lemma(steadyWord, { gloss_en: 'tree' })
        ], { actor: 'admin:test', expectedRev: 0 });
        assert.equal(planted.ok, true, JSON.stringify(planted));

        const misses = [];
        for (let i = 0; i < 1; i += 1) {
            for (const direction of ['he_en', 'en_he']) {
                misses.push(signal({
                    id: 'weak-' + direction,
                    word: weakWord,
                    direction,
                    result: 'miss',
                    grade: 1
                }));
            }
        }
        for (let i = 0; i < 3; i += 1) {
            for (const direction of ['he_en', 'en_he']) {
                misses.push(signal({
                    id: 'very-' + i + '-' + direction,
                    word: veryWeakWord,
                    direction,
                    result: 'miss',
                    grade: 1
                }));
            }
        }
        misses.push(signal({
            id: 'steady-he',
            word: steadyWord,
            direction: 'he_en',
            result: 'pass',
            grade: 3
        }));
        const graded = await api('POST', '/api/review/signals', 'learner', { signals: misses });
        assert.equal(graded.status, 200);
        assert.equal(graded.json.rejected, 0);

        const deck = await api('GET', '/api/me/cards?deck=weak', 'learner');
        assert.equal(deck.status, 200);
        assert.equal(deck.json.deck, 'weak');
        const words = [...new Set(deck.json.cards.map((card) => card.word))].sort();
        assert.deepEqual(words, [veryWeakWord, weakWord].sort());
        assert.ok(deck.json.cards.length >= 2);
        for (const card of deck.json.cards) {
            assert.equal(card.word === steadyWord, false);
            assert.deepEqual(Object.keys(card.intervals).sort(), ['1', '2', '3', '4']);
            for (const grade of ['1', '2', '3', '4']) {
                assert.equal(typeof card.intervals[grade], 'number');
                assert.ok(card.intervals[grade] >= 1);
            }
        }
    });

    it('ac-4: a suspended word stays in the library and leaves every deck', async () => {
        const word = exactKey('מַפְתֵּחַ');
        const added = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: word, gloss_en: 'key' }]
        });
        assert.equal(added.status, 200);

        async function decksContaining() {
            const names = ['due', 'weak', 'new', 'all', 'lesson:public-lesson'];
            const found = [];
            for (const name of names) {
                const res = await api('GET', '/api/me/cards?deck=' + encodeURIComponent(name), 'learner');
                assert.equal(res.status, 200, name);
                if (res.json.cards.some((card) => card.word === word)) found.push(name);
            }
            return found;
        }

        const before = await decksContaining();
        assert.ok(before.includes('due'));
        assert.ok(before.includes('new'));
        assert.ok(before.includes('all'));

        const blocked = await api(
            'PATCH',
            '/api/me/library/' + encodeURIComponent(word),
            'learner',
            { suspended: true },
            { origin: false }
        );
        assert.equal(blocked.status, 403);
        assert.equal(blocked.json.code, 'csrf');
        assert.equal(reviewRepo.find(users.learner.id, word, 'he_en').suspended, 0);

        const patched = await api(
            'PATCH',
            '/api/me/library/' + encodeURIComponent(word),
            'learner',
            { suspended: true }
        );
        assert.equal(patched.status, 200);
        assert.equal(patched.json.suspended, true);
        assert.equal(patched.json.word, word);

        const listed = await api('GET', '/api/me/library?suspended=1', 'learner');
        const row = listed.json.words.find((item) => item.word === word);
        assert.ok(row);
        assert.equal(row.suspended, true);

        const after = await decksContaining();
        assert.deepEqual(after, []);
    });

    it('ac-5: signed-out requests are 401 and learners cannot see each other', async () => {
        const routes = [
            ['GET', '/api/me/library'],
            ['GET', '/api/me/cards'],
            ['GET', '/api/me/cards?deck=weak'],
            ['GET', '/api/lexicon'],
            ['GET', '/api/lexicon/' + encodeURIComponent(exactKey('אָב'))],
            ['GET', '/api/grammar/concepts']
        ];
        for (const [method, path] of routes) {
            const res = await api(method, path, null);
            assert.equal(res.status, 401, path);
            assert.equal(res.json.code, 'no_session');
        }
        const post = await api('POST', '/api/me/library', null, {
            words: [{ key: 'סִפְרִיָּה', gloss_en: 'library' }]
        });
        assert.equal(post.status, 401);
        assert.equal(post.json.code, 'no_session');
        const patch = await api(
            'PATCH',
            '/api/me/library/' + encodeURIComponent(exactKey('אָב')),
            null,
            { suspended: true }
        );
        assert.equal(patch.status, 401);

        const learnerWord = exactKey('נַעַל');
        const adminWord = exactKey('כּוֹבַע');
        const learnerAdd = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: learnerWord, gloss_en: 'shoe' }],
            userId: users.admin.id
        });
        assert.equal(learnerAdd.status, 400);
        assert.equal(userWords.find(users.admin.id, learnerWord), undefined);

        const learnerOk = await api('POST', '/api/me/library', 'learner', {
            words: [{ key: learnerWord, gloss_en: 'shoe' }]
        });
        assert.equal(learnerOk.status, 200);
        const adminOk = await api('POST', '/api/me/library', 'admin', {
            words: [{ key: adminWord, gloss_en: 'hat' }]
        });
        assert.equal(adminOk.status, 200);

        const asLearner = await api(
            'GET',
            '/api/me/library?userId=' + users.admin.id,
            'learner'
        );
        const learnerWords = asLearner.json.words.map((row) => row.word);
        assert.equal(learnerWords.includes(learnerWord), true);
        assert.equal(learnerWords.includes(adminWord), false);

        const asAdmin = await api(
            'GET',
            '/api/me/library?userId=' + users.learner.id,
            'admin'
        );
        const adminWords = asAdmin.json.words.map((row) => row.word);
        assert.equal(adminWords.includes(adminWord), true);
        assert.equal(adminWords.includes(learnerWord), false);

        const learnerCards = await api('GET', '/api/me/cards?deck=all', 'learner');
        assert.equal(learnerCards.json.cards.some((card) => card.word === adminWord), false);
    });

    it('graded signals: grade is stored and a form signal writes no review row', async () => {
        const word = exactKey('דֶּלֶת');
        const graded = await api('POST', '/api/review/signals', 'learner', {
            signals: [signal({
                id: 'grade-door',
                word,
                result: 'pass',
                grade: 4,
                kind: 'word'
            })]
        });
        assert.equal(graded.status, 200);
        assert.equal(graded.json.applied, 1);
        const row = reviewRepo.find(users.learner.id, word, 'he_en');
        assert.equal(row.last_grade, 4);

        const formWord = exactKey('כָּתַבְתִּי');
        const form = await api('POST', '/api/review/signals', 'learner', {
            signals: [signal({
                id: 'grade-form',
                word: formWord,
                result: 'pass',
                grade: 3,
                kind: 'form',
                lemma: exactKey('לִכְתֹּב')
            })]
        });
        assert.equal(form.status, 200);
        assert.equal(form.json.applied, 1);
        assert.equal(reviewRepo.find(users.learner.id, formWord, 'he_en'), undefined);

        const bad = await api('POST', '/api/review/signals', 'learner', {
            signals: [signal({
                id: 'grade-bad',
                word: exactKey('חַלּוֹן'),
                result: 'miss',
                grade: 4
            })]
        });
        assert.equal(bad.status, 200);
        assert.equal(bad.json.applied, 0);
        assert.ok(bad.json.rejected >= 1);
        assert.equal(reviewRepo.find(users.learner.id, 'חַלּוֹן', 'he_en'), undefined);
    });

    it('batch and body limits refuse oversized library writes', async () => {
        const letters = ['א', 'ב', 'ג', 'ד', 'ה', 'ו', 'ז', 'ח', 'ט', 'י', 'כ', 'ל', 'מ', 'נ', 'ס', 'ע', 'פ', 'צ', 'ק', 'ר', 'ש', 'ת'];
        const words = [];
        for (let i = 0; i < 51; i += 1) {
            const a = letters[i % letters.length];
            const b = letters[Math.floor(i / letters.length) % letters.length];
            const c = letters[(i * 3) % letters.length];
            words.push({ key: a + b + c, gloss_en: 'n' + i });
        }
        const tooMany = await api('POST', '/api/me/library', 'learner', { words });
        assert.equal(tooMany.status, 413);
        assert.equal(tooMany.json.error, 'words_too_many');

        const huge = JSON.stringify({
            words: [{ key: 'א', gloss_en: 'x'.repeat(20000) }]
        });
        assert.ok(Buffer.byteLength(huge) > 16 * 1024);
        let hugeStatus = null;
        try {
            const tooBig = await api('POST', '/api/me/library', 'learner', huge);
            hugeStatus = tooBig.status;
            assert.equal(tooBig.json.code, 'body_too_large');
        } catch (err) {
            // body.js destroys the socket once the limit is crossed.
            assert.ok(err && (err.cause || err.message));
            hugeStatus = 'aborted';
        }
        assert.ok(hugeStatus === 413 || hugeStatus === 'aborted');
        assert.equal(userWords.find(users.learner.id, exactKey('א')), undefined);
    });

    it('due deck keeps the new-card cap', async () => {
        const letters = ['א', 'ב', 'ג', 'ד', 'ה', 'ו', 'ז', 'ח', 'ט', 'י', 'כ'];
        const words = letters.map((letter, index) => ({
            key: 'ק' + letter + 'ק',
            gloss_en: 'cap-' + index
        }));
        const added = await api('POST', '/api/me/library', 'learner', { words });
        assert.equal(added.status, 200, JSON.stringify(added.json));
        const deck = await api('GET', '/api/me/cards?deck=due', 'learner');
        assert.equal(deck.status, 200);
        const unique = new Set(deck.json.cards.map((card) => card.word));
        assert.equal(unique.size, 10);
        const capped = words.map((item) => exactKey(item.key));
        const present = capped.filter((word) => unique.has(word));
        assert.ok(present.length > 0);
        assert.ok(present.length < capped.length);
    });

    it('lexicon reads return unverified entries and grammar concepts to a signed-in learner', async () => {
        const list = await api('GET', '/api/lexicon?q=' + encodeURIComponent('לִכְתֹּב'), 'learner');
        assert.equal(list.status, 200);
        const entry = list.json.entries.find((row) => row.key === exactKey(VERB));
        assert.ok(entry);
        assert.equal(entry.verified, false);

        const one = await api('GET', '/api/lexicon/' + encodeURIComponent(exactKey(VERB)), 'learner');
        assert.equal(one.status, 200);
        assert.equal(one.json.entry.key, exactKey(VERB));
        assert.equal(one.json.entry.verified, false);
        assert.equal(one.json.entry.binyan, 'paal');

        const missing = await api('GET', '/api/lexicon/' + encodeURIComponent(exactKey('צִפּוֹר')), 'learner');
        assert.equal(missing.status, 404);
        assert.equal(missing.json.error, 'not_found');

        const concepts = await api('GET', '/api/grammar/concepts', 'learner');
        assert.equal(concepts.status, 200);
        const paal = concepts.json.concepts.find((row) => row.key === 'binyan:paal');
        assert.ok(paal);
        assert.equal(paal.family, 'binyan');
        assert.equal(typeof paal.label_en, 'string');
    });

    it('library pages with an opaque cursor and rejects a bad cursor', async () => {
        const first = await api('GET', '/api/me/library?limit=2', 'learner');
        assert.equal(first.status, 200);
        assert.equal(first.json.words.length, 2);
        assert.equal(typeof first.json.nextCursor, 'string');
        const second = await api(
            'GET',
            '/api/me/library?limit=2&cursor=' + encodeURIComponent(first.json.nextCursor),
            'learner'
        );
        assert.equal(second.status, 200);
        const firstWords = first.json.words.map((row) => row.word);
        for (const row of second.json.words) {
            assert.equal(firstWords.includes(row.word), false);
        }
        const bad = await api('GET', '/api/me/library?cursor=not-a-cursor', 'learner');
        assert.equal(bad.status, 400);
        assert.equal(bad.json.error, 'invalid_cursor');
        const high = await api('GET', '/api/me/library?limit=500', 'learner');
        assert.equal(high.status, 200);
        assert.ok(high.json.words.length <= 200);
    });
});
