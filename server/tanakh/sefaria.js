/**
 * Fetch Tanakh text from the Sefaria API, pinned to Public Domain versions:
 * Hebrew "Tanach with Nikkud", English "The Holy Scriptures: A New Translation
 * (JPS 1917)". No attribution is legally required for either; provenance is
 * still recorded in the manifest and tanakh_meta.
 *
 * The fetch refuses any payload whose license metadata is not Public Domain,
 * so a Sefaria-side version change can never silently pull in CC-BY-NC text.
 */
'use strict';

const API_BASE = 'https://www.sefaria.org';
const VERSION_EN = 'The Holy Scriptures: A New Translation (JPS 1917)';
const VERSION_HE = 'Tanach with Nikkud';
const USER_AGENT = 'hebrew-app corpus fetch (public domain texts only)';

/** The 39 books of Tanakh with their Sefaria index titles, in canonical order. */
const BOOKS = [
    { slug: 'genesis', title: 'Genesis' },
    { slug: 'exodus', title: 'Exodus' },
    { slug: 'leviticus', title: 'Leviticus' },
    { slug: 'numbers', title: 'Numbers' },
    { slug: 'deuteronomy', title: 'Deuteronomy' },
    { slug: 'joshua', title: 'Joshua' },
    { slug: 'judges', title: 'Judges' },
    { slug: 'i-samuel', title: 'I Samuel' },
    { slug: 'ii-samuel', title: 'II Samuel' },
    { slug: 'i-kings', title: 'I Kings' },
    { slug: 'ii-kings', title: 'II Kings' },
    { slug: 'isaiah', title: 'Isaiah' },
    { slug: 'jeremiah', title: 'Jeremiah' },
    { slug: 'ezekiel', title: 'Ezekiel' },
    { slug: 'hosea', title: 'Hosea' },
    { slug: 'joel', title: 'Joel' },
    { slug: 'amos', title: 'Amos' },
    { slug: 'obadiah', title: 'Obadiah' },
    { slug: 'jonah', title: 'Jonah' },
    { slug: 'micah', title: 'Micah' },
    { slug: 'nahum', title: 'Nahum' },
    { slug: 'habakkuk', title: 'Habakkuk' },
    { slug: 'zephaniah', title: 'Zephaniah' },
    { slug: 'haggai', title: 'Haggai' },
    { slug: 'zechariah', title: 'Zechariah' },
    { slug: 'malachi', title: 'Malachi' },
    { slug: 'psalms', title: 'Psalms' },
    { slug: 'proverbs', title: 'Proverbs' },
    { slug: 'job', title: 'Job' },
    { slug: 'song-of-songs', title: 'Song of Songs' },
    { slug: 'ruth', title: 'Ruth' },
    { slug: 'lamentations', title: 'Lamentations' },
    { slug: 'ecclesiastes', title: 'Ecclesiastes' },
    { slug: 'esther', title: 'Esther' },
    { slug: 'daniel', title: 'Daniel' },
    { slug: 'ezra', title: 'Ezra' },
    { slug: 'nehemiah', title: 'Nehemiah' },
    { slug: 'i-chronicles', title: 'I Chronicles' },
    { slug: 'ii-chronicles', title: 'II Chronicles' }
];

const FOOTNOTE = /<i\b[^>]*class="footnote"[^>]*>[\s\S]*?<\/i>/gi;
const FOOTNOTE_MARKER = /<sup\b[^>]*>[\s\S]*?<\/sup>/gi;
const LINE_BREAK = /<br\b[^>]*>/gi;
const TAG = /<[^>]+>/g;
const ENTITY = /&[a-zA-Z#0-9]+;/g;
const ENTITIES = {
    '&thinsp;': '',
    '&nbsp;': ' ',
    '&amp;': '&',
    '&lt;': '<',
    '&gt;': '>',
    '&quot;': '"',
    '&#39;': "'",
    '&rsquo;': '\u2019',
    '&lsquo;': '\u2018',
    '&ldquo;': '\u201C',
    '&rdquo;': '\u201D',
    '&mdash;': '\u2014',
    '&ndash;': '\u2013',
    '&hellip;': '\u2026'
};

/**
 * Sefaria text arrives with HTML: footnotes, markers, and inline formatting.
 * Footnotes and markers are dropped whole; other tags dissolve without adding
 * spaces, because Hebrew words can be split across tags (<big>בְּ</big>רֵאשִׁית).
 * @param {unknown} value
 * @returns {string}
 */
function stripTags(value) {
    if (value == null) return '';
    let text = String(value);
    text = text.replace(FOOTNOTE, '').replace(FOOTNOTE_MARKER, '');
    text = text.replace(LINE_BREAK, ' ');
    text = text.replace(TAG, '');
    text = text.replace(ENTITY, (match) => (
        Object.prototype.hasOwnProperty.call(ENTITIES, match) ? ENTITIES[match] : ' '
    ));
    return text.replace(/\s+/g, ' ').trim();
}

/**
 * Fail loud when a payload is not the public-domain text we asked for.
 * @param {{ license?: unknown, heLicense?: unknown }} payload
 * @param {string} title
 */
function assertPublicDomain(payload, title) {
    const en = payload && payload.license;
    const he = payload && payload.heLicense;
    if (en !== 'Public Domain' || he !== 'Public Domain') {
        throw new Error(
            'Refusing ' + title + ': license en=' + JSON.stringify(en)
            + ' he=' + JSON.stringify(he) + ' (expected Public Domain for both)'
        );
    }
}

/**
 * Chapter count from an /api/index/<title> payload. schema.lengths is
 * [chapters, verses] for the simple book structures of Tanakh.
 * @param {{ schema?: { lengths?: unknown } }} payload
 * @param {string} title
 * @returns {number}
 */
function chapterCountFromIndex(payload, title) {
    const lengths = payload && payload.schema && payload.schema.lengths;
    const count = Array.isArray(lengths) ? Number(lengths[0]) : NaN;
    if (!Number.isInteger(count) || count < 1) {
        throw new Error('No chapter count in the index for ' + title);
    }
    return count;
}

/**
 * Fetch the index metadata for one book (chapter count lives there).
 * @param {string} title
 * @param {{ baseUrl?: string }} [options]
 * @returns {Promise<object>}
 */
async function fetchIndex(title, options) {
    const opts = options || {};
    const base = opts.baseUrl || API_BASE;
    const res = await fetch(base + '/api/index/' + encodeURIComponent(title), {
        headers: { 'User-Agent': USER_AGENT }
    });
    if (!res.ok) throw new Error('Sefaria HTTP ' + res.status + ' for index ' + title);
    return res.json();
}

/**
 * Fetch one chapter with versions pinned. Book-level refs with pinned
 * versions silently collapse to the first chapter, so everything goes
 * through chapter refs. Throws on HTTP errors and non-public-domain payloads.
 * @param {string} title Sefaria index title, e.g. "Genesis".
 * @param {number} chapterNum 1-based.
 * @param {{ baseUrl?: string }} [options]
 * @returns {Promise<object>}
 */
async function fetchChapter(title, chapterNum, options) {
    const opts = options || {};
    const base = opts.baseUrl || API_BASE;
    const ref = title + '.' + chapterNum;
    const url = base + '/api/texts/' + encodeURIComponent(ref)
        + '?context=0&commentary=0&pad=1&wrapLinks=0'
        + '&ven=' + encodeURIComponent(VERSION_EN)
        + '&vhe=' + encodeURIComponent(VERSION_HE);
    const res = await fetch(url, { headers: { 'User-Agent': USER_AGENT } });
    if (!res.ok) throw new Error('Sefaria HTTP ' + res.status + ' for ' + ref);
    const payload = await res.json();
    assertPublicDomain(payload, ref);
    return payload;
}

/**
 * Normalize one text section (he or text) to chapter arrays. Multi-chapter
 * books arrive as arrays of arrays; single-chapter books (Obadiah) arrive as
 * a flat array of verse strings.
 * @param {unknown} section
 * @returns {string[][]}
 */
function toChapters(section) {
    if (!Array.isArray(section)) return [];
    if (section.every((entry) => typeof entry === 'string')) return [section];
    return section.map((chapter) => (Array.isArray(chapter) ? chapter : []));
}

/**
 * Flatten a book payload to verse rows. Hebrew and English are paired by
 * index (pad=1 keeps them aligned); a verse missing on one side keeps its
 * slot with an empty string. Rows empty on both sides are skipped.
 * @param {string} slug
 * @param {{ he?: unknown, text?: unknown }} payload
 * @returns {{ book: string, chapter: number, verse: number, he: string, en: string }[]}
 */
function toRows(slug, payload) {
    const heChapters = toChapters(payload && payload.he);
    const enChapters = toChapters(payload && payload.text);
    const rows = [];
    const chapterCount = Math.max(heChapters.length, enChapters.length);
    for (let c = 0; c < chapterCount; c += 1) {
        const he = heChapters[c] || [];
        const en = enChapters[c] || [];
        const verseCount = Math.max(he.length, en.length);
        for (let v = 0; v < verseCount; v += 1) {
            const heText = stripTags(he[v]);
            const enText = stripTags(en[v]);
            if (!heText && !enText) continue;
            rows.push({ book: slug, chapter: c + 1, verse: v + 1, he: heText, en: enText });
        }
    }
    return rows;
}

/**
 * Flatten a chapter payload to verse rows under chapterNum. Chapter-level
 * refs return flat verse arrays; pad=1 keeps he/en index-aligned, a verse
 * missing on one side keeps its slot with an empty string. Rows empty on
 * both sides are skipped.
 * @param {string} slug
 * @param {number} chapterNum 1-based.
 * @param {{ he?: unknown, text?: unknown }} payload
 * @returns {{ book: string, chapter: number, verse: number, he: string, en: string }[]}
 */
function toChapterRows(slug, chapterNum, payload) {
    const he = toChapters(payload && payload.he)[0] || [];
    const en = toChapters(payload && payload.text)[0] || [];
    const rows = [];
    const verseCount = Math.max(he.length, en.length);
    for (let v = 0; v < verseCount; v += 1) {
        const heText = stripTags(he[v]);
        const enText = stripTags(en[v]);
        if (!heText && !enText) continue;
        rows.push({ book: slug, chapter: chapterNum, verse: v + 1, he: heText, en: enText });
    }
    return rows;
}

module.exports = {
    API_BASE,
    VERSION_EN,
    VERSION_HE,
    BOOKS,
    stripTags,
    assertPublicDomain,
    chapterCountFromIndex,
    fetchIndex,
    fetchChapter,
    toChapters,
    toRows,
    toChapterRows
};
