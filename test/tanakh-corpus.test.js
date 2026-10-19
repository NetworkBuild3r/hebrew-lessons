/**
 * Tanakh corpus: Sefaria normalization, license guard, loader, repo.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

describe('sefaria normalization', () => {
    const sefaria = require('../server/tanakh/sefaria');

    it('book list covers the 39 books of Tanakh in order', () => {
        assert.equal(sefaria.BOOKS.length, 39);
        assert.equal(sefaria.BOOKS[0].slug, 'genesis');
        assert.equal(sefaria.BOOKS[38].slug, 'ii-chronicles');
        const slugs = new Set(sefaria.BOOKS.map((book) => book.slug));
        ['deuteronomy', 'psalms', 'song-of-songs', 'obadiah', 'i-samuel'].forEach((slug) => {
            assert.ok(slugs.has(slug), 'missing ' + slug);
        });
        assert.equal(slugs.size, 39, 'slugs must be unique');
    });

    it('stripTags dissolves inline tags without splitting Hebrew words', () => {
        assert.equal(sefaria.stripTags('<big>בְּ</big>רֵאשִׁ֖ית בָּרָ֣א'), 'בְּרֵאשִׁ֖ית בָּרָ֣א');
        assert.equal(sefaria.stripTags('word <b>bold</b> word'), 'word bold word');
    });

    it('stripTags drops footnotes and their markers whole', () => {
        const html = 'In the beginning<sup class="footnote-marker">a</sup>'
            + '<i class="footnote"><b>In the beginning </b>Others “When God began.”</i> God created';
        assert.equal(sefaria.stripTags(html), 'In the beginning God created');
    });

    it('stripTags decodes entities, turns breaks into spaces, collapses whitespace', () => {
        assert.equal(sefaria.stripTags('a&nbsp;b &amp; c'), 'a b & c');
        assert.equal(sefaria.stripTags('line<br>break'), 'line break');
        assert.equal(sefaria.stripTags('  many   spaces  '), 'many spaces');
        assert.equal(sefaria.stripTags(null), '');
        assert.equal(sefaria.stripTags(undefined), '');
    });

    it('toChapters treats a flat verse array as one chapter (single-chapter books)', () => {
        assert.deepEqual(sefaria.toChapters(['א', 'ב']), [['א', 'ב']]);
        assert.deepEqual(sefaria.toChapters([['א'], ['ב']], ), [['א'], ['ב']]);
        assert.deepEqual(sefaria.toChapters(null), []);
        assert.deepEqual(sefaria.toChapters('nope'), []);
    });

    it('toRows flattens chapters and pairs he/en by index', () => {
        const rows = sefaria.toRows('genesis', {
            he: [['א', 'ב'], ['ג']],
            text: [['one', 'two'], ['three']]
        });
        assert.deepEqual(rows, [
            { book: 'genesis', chapter: 1, verse: 1, he: 'א', en: 'one' },
            { book: 'genesis', chapter: 1, verse: 2, he: 'ב', en: 'two' },
            { book: 'genesis', chapter: 2, verse: 1, he: 'ג', en: 'three' }
        ]);
    });

    it('toRows keeps slots when one side is missing a verse', () => {
        const rows = sefaria.toRows('psalms', { he: [['א', 'ב']], text: [['one']] });
        assert.deepEqual(
            rows.map((row) => [row.verse, row.he, row.en]),
            [[1, 'א', 'one'], [2, 'ב', '']]
        );
    });

    it('toRows skips verses empty on both sides', () => {
        const rows = sefaria.toRows('job', { he: [['', 'א']], text: [['', '']] });
        assert.equal(rows.length, 1);
        assert.equal(rows[0].verse, 2);
    });

    it('chapterCountFromIndex reads schema.lengths and rejects junk', () => {
        assert.equal(sefaria.chapterCountFromIndex({ schema: { lengths: [50, 1533] } }, 'Genesis'), 50);
        assert.equal(sefaria.chapterCountFromIndex({ schema: { lengths: [1, 21] } }, 'Obadiah'), 1);
        assert.throws(() => sefaria.chapterCountFromIndex({ schema: {} }, 'Genesis'), /chapter count/);
        assert.throws(() => sefaria.chapterCountFromIndex(null, 'Genesis'), /chapter count/);
        assert.throws(() => sefaria.chapterCountFromIndex({ schema: { lengths: [0] } }, 'Genesis'), /chapter count/);
    });

    it('toChapterRows pairs flat chapter arrays under the chapter number', () => {
        const rows = sefaria.toChapterRows('genesis', 12, {
            he: ['א', 'ב', 'ג'],
            text: ['one', 'two']
        });
        assert.deepEqual(rows, [
            { book: 'genesis', chapter: 12, verse: 1, he: 'א', en: 'one' },
            { book: 'genesis', chapter: 12, verse: 2, he: 'ב', en: 'two' },
            { book: 'genesis', chapter: 12, verse: 3, he: 'ג', en: '' }
        ]);
    });

    it('toChapterRows handles an empty chapter payload', () => {
        assert.deepEqual(sefaria.toChapterRows('genesis', 1, {}), []);
        assert.deepEqual(sefaria.toChapterRows('genesis', 1, null), []);
    });

    it('license guard accepts Public Domain and refuses everything else', () => {
        sefaria.assertPublicDomain({ license: 'Public Domain', heLicense: 'Public Domain' }, 'Genesis');
        assert.throws(
            () => sefaria.assertPublicDomain({ license: 'CC-BY-NC', heLicense: 'Public Domain' }, 'Genesis'),
            /Refusing Genesis/
        );
        assert.throws(
            () => sefaria.assertPublicDomain({ license: 'Public Domain' }, 'Genesis'),
            /Refusing Genesis/
        );
        assert.throws(() => sefaria.assertPublicDomain(null, 'Genesis'), /Refusing Genesis/);
    });
});

describe('tanakh loader + repo', () => {
    let tmp;
    let db;
    let load;
    let repo;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-tanakh-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/repos/tanakh')];
        delete require.cache[require.resolve('../server/tanakh/load')];

        const dir = path.join(tmp, 'tanakh');
        fs.mkdirSync(dir, { recursive: true });
        fs.writeFileSync(path.join(dir, 'genesis.json'), JSON.stringify({
            book: 'genesis',
            title: 'Genesis',
            rows: [
                { book: 'genesis', chapter: 1, verse: 1, he: 'בְּרֵאשִׁית בָּרָא אֱלֹהִים', en: 'In the beginning God created' },
                { book: 'genesis', chapter: 1, verse: 2, he: 'וְהָאָרֶץ הָיְתָה תֹהוּ וָבֹהוּ', en: 'the earth being unformed and void' }
            ]
        }));
        fs.writeFileSync(path.join(dir, 'psalms.json'), JSON.stringify({
            book: 'psalms',
            title: 'Psalms',
            rows: [
                { book: 'psalms', chapter: 23, verse: 1, he: 'מִזְמוֹר לְדָוִד', en: 'A psalm of David' }
            ]
        }));
        fs.writeFileSync(path.join(dir, 'manifest.json'), JSON.stringify({
            source: 'sefaria-api',
            fetchedAt: '2026-10-18T00:00:00.000Z',
            versionEn: 'The Holy Scriptures: A New Translation (JPS 1917)',
            versionHe: 'Tanach with Nikkud',
            licenseEn: 'Public Domain',
            licenseHe: 'Public Domain',
            books: [
                { slug: 'genesis', title: 'Genesis', rows: 2, file: 'genesis.json' },
                { slug: 'psalms', title: 'Psalms', rows: 1, file: 'psalms.json' }
            ]
        }));

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        repo = require('../server/db/repos/tanakh');
        load = require('../server/tanakh/load');
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
        delete require.cache[require.resolve('../server/paths')];
        delete require.cache[require.resolve('../server/db')];
        delete require.cache[require.resolve('../server/db/repos/tanakh')];
        delete require.cache[require.resolve('../server/tanakh/load')];
    });

    it('filesPresent needs the manifest', () => {
        assert.ok(load.filesPresent());
    });

    it('loadIfEmpty loads verses and provenance once; second run skips', () => {
        const first = load.loadIfEmpty();
        assert.equal(first.loaded, 3);
        assert.equal(first.skipped, '');
        assert.equal(repo.countVerses(), 3);

        const second = load.loadIfEmpty();
        assert.equal(second.loaded, 0);
        assert.equal(second.skipped, 'already loaded');
        assert.equal(repo.countVerses(), 3);
    });

    it('stores provenance in tanakh_meta', () => {
        const meta = repo.allMeta();
        assert.equal(meta.source, 'sefaria-api');
        assert.equal(meta.version_en, 'The Holy Scriptures: A New Translation (JPS 1917)');
        assert.equal(meta.version_he, 'Tanach with Nikkud');
        assert.equal(meta.license_en, 'Public Domain');
        assert.equal(meta.license_he, 'Public Domain');
        assert.equal(meta.book_count, '2');
    });

    it('looks up verses and chapters', () => {
        const verse = repo.verse('genesis', 1, 1);
        assert.equal(verse.en, 'In the beginning God created');
        assert.equal(repo.chapter('genesis', 1).length, 2);
        assert.deepEqual(
            repo.chapter('genesis', 1).map((row) => row.verse),
            [1, 2]
        );
        assert.equal(repo.verse('genesis', 9, 9), null);
        assert.equal(repo.chapter('genesis', 99).length, 0);
        assert.equal(repo.verse('genesis', 'one', 1), null);
        assert.equal(repo.chapter('genesis', 0).length, 0);
    });

    it('lists books with counts in corpus order', () => {
        const books = repo.books();
        assert.equal(books.length, 2);
        assert.equal(books[0].book, 'genesis');
        assert.equal(books[0].verses, 2);
        assert.equal(books[0].chapters, 1);
        assert.equal(books[1].book, 'psalms');
    });

    it('book + chapter + verse is unique', () => {
        assert.throws(() => {
            db.run(
                'INSERT INTO tanakh_verses (book, chapter, verse, he, en) VALUES (?, ?, ?, ?, ?)',
                'genesis', 1, 1, 'x', 'y'
            );
        }, /UNIQUE|constraint/i);
    });

    it('replace swaps the corpus wholesale', () => {
        const written = repo.replaceAll(
            [{ book: 'ruth', chapter: 1, verse: 1, he: 'וַיְהִי', en: 'And it came to pass' }],
            { source: 'test' }
        );
        assert.equal(written, 1);
        assert.equal(repo.countVerses(), 1);
        assert.equal(repo.verse('genesis', 1, 1), null);
        assert.equal(repo.allMeta().source, 'test');
    });
});
