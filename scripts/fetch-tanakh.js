/**
 * Download the Tanakh from the Sefaria API into data/tanakh, pinned to
 * Public Domain versions (Hebrew: Tanach with Nikkud; English: JPS 1917).
 * Every book is checked at fetch time; anything not Public Domain is refused.
 *
 *   node scripts/fetch-tanakh.js                          # all 39 books, skip files already present
 *   node scripts/fetch-tanakh.js --force                  # re-download everything
 *   node scripts/fetch-tanakh.js --book Genesis --book Psalms
 */
'use strict';

const fs = require('fs');
const path = require('path');

const { dataDir } = require('../server/paths');
const {
    BOOKS,
    fetchIndex,
    fetchChapter,
    chapterCountFromIndex,
    toChapterRows,
    VERSION_EN,
    VERSION_HE
} = require('../server/tanakh/sefaria');

const DELAY_MS = 120;

/**
 * @param {string[]} argv
 * @returns {{ force: boolean, titles: Set<string>|null }}
 */
function parseArgs(argv) {
    let force = false;
    const titles = [];
    for (let i = 0; i < argv.length; i += 1) {
        if (argv[i] === '--force') {
            force = true;
        } else if (argv[i] === '--book' && argv[i + 1]) {
            titles.push(argv[i + 1]);
            i += 1;
        } else {
            throw new Error('Unknown argument: ' + argv[i] + ' (expected --force or --book <Title>)');
        }
    }
    return { force, titles: titles.length ? new Set(titles) : null };
}

/**
 * @param {number} ms
 * @returns {Promise<void>}
 */
function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

async function main() {
    const { force, titles } = parseArgs(process.argv.slice(2));
    const dir = path.join(dataDir(), 'tanakh');
    fs.mkdirSync(dir, { recursive: true });

    const wanted = BOOKS.filter((book) => !titles || titles.has(book.title));
    if (titles) {
        const known = new Set(BOOKS.map((book) => book.title));
        titles.forEach((title) => {
            if (!known.has(title)) throw new Error('Unknown book title: ' + title);
        });
    }

    const manifestBooks = [];
    let fetched = 0;
    let kept = 0;
    for (const book of wanted) {
        const file = path.join(dir, book.slug + '.json');
        if (!force && fs.existsSync(file)) {
            const existing = JSON.parse(fs.readFileSync(file, 'utf8'));
            const count = Array.isArray(existing.rows) ? existing.rows.length : 0;
            manifestBooks.push({ slug: book.slug, title: book.title, rows: count, file: book.slug + '.json' });
            kept += 1;
            process.stdout.write(book.title + ': kept ' + count + ' verses (already downloaded)\n');
            continue;
        }
        await sleep(DELAY_MS);
        const index = await fetchIndex(book.title);
        const chapters = chapterCountFromIndex(index, book.title);
        const rows = [];
        for (let chapterNum = 1; chapterNum <= chapters; chapterNum += 1) {
            await sleep(DELAY_MS);
            const payload = await fetchChapter(book.title, chapterNum);
            rows.push(...toChapterRows(book.slug, chapterNum, payload));
        }
        fs.writeFileSync(
            file,
            JSON.stringify({ book: book.slug, title: book.title, rows }) + '\n'
        );
        manifestBooks.push({ slug: book.slug, title: book.title, rows: rows.length, file: book.slug + '.json' });
        fetched += 1;
        process.stdout.write(book.title + ': ' + rows.length + ' verses (' + chapters + ' chapters)\n');
    }

    const manifest = {
        source: 'sefaria-api',
        fetchedAt: new Date().toISOString(),
        versionEn: VERSION_EN,
        versionHe: VERSION_HE,
        licenseEn: 'Public Domain',
        licenseHe: 'Public Domain',
        books: manifestBooks
    };
    fs.writeFileSync(path.join(dir, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
    process.stdout.write(
        'Done: ' + fetched + ' fetched, ' + kept + ' kept → ' + path.join(dir, 'manifest.json') + '\n'
    );
}

main().catch((err) => {
    process.stderr.write('fetch-tanakh failed: ' + (err && err.message ? err.message : err) + '\n');
    process.exit(1);
});
