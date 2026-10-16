/**
 * Read a StarDict Hebrew–English dictionary (.idx, .dict.dz, .syn.dz) into rows.
 * Each article is HTML: a part-of-speech label, then the English senses.
 * A synonym file maps spelling variants to an article, and each becomes a row too.
 */
'use strict';

const zlib = require('zlib');

const { foldHebrew } = require('./parse-dsl');

const ENTITIES = { '&gt;': '>', '&lt;': '<', '&amp;': '&', '&quot;': '"', '&#39;': "'", '&nbsp;': ' ' };
const SENSES_MAX = 600;
const HE_MAX = 80;

/**
 * @param {Buffer} idx
 * @returns {{ word: string, offset: number, size: number }[]}
 */
function readIndex(idx) {
    const out = [];
    let at = 0;
    while (at < idx.length) {
        const end = idx.indexOf(0, at);
        if (end < 0 || end + 9 > idx.length) break;
        out.push({
            word: idx.toString('utf8', at, end),
            offset: idx.readUInt32BE(end + 1),
            size: idx.readUInt32BE(end + 5)
        });
        at = end + 9;
    }
    return out;
}

/**
 * @param {Buffer} syn
 * @returns {{ word: string, index: number }[]}
 */
function readSynonyms(syn) {
    const out = [];
    let at = 0;
    while (at < syn.length) {
        const end = syn.indexOf(0, at);
        if (end < 0 || end + 5 > syn.length) break;
        out.push({ word: syn.toString('utf8', at, end), index: syn.readUInt32BE(end + 1) });
        at = end + 5;
    }
    return out;
}

/**
 * @param {string} text
 * @returns {string}
 */
function plain(text) {
    return String(text)
        .replace(/<[^>]*>/g, ' ')
        .replace(/&(?:gt|lt|amp|quot|nbsp|#39);/g, (m) => ENTITIES[m] || ' ')
        .replace(/\(>>[^)]*\)/g, ' ')
        .replace(/\s+/g, ' ')
        .replace(/\s+([,;.])/g, '$1')
        .trim();
}

/**
 * One article becomes a part of speech and the senses text.
 * @param {string} html
 * @returns {{ pos: string, en: string }|null}
 */
function readArticle(html) {
    const parts = String(html).split(/<br\s*\/?>/i);
    const head = parts.shift() || '';
    const font = /<font[^>]*>([^<]*)<\/font>/i.exec(head);
    const pos = font ? plain(font[1]) : '';
    // After the label come the other forms: a verb's infinitive, a noun's plural.
    const rest = plain(head.replace(/<font[^>]*>[^<]*<\/font>/i, ''));
    const forms = rest.split(/[,;]/).map((s) => s.trim()).filter((s) => /[\u05D0-\u05EA]/.test(s));
    let en = plain(parts.join(' '));
    if (!en) en = rest;
    if (!en) return null;
    if (en.length > SENSES_MAX) en = en.slice(0, SENSES_MAX).replace(/[,;]\s*[^,;]*$/, '');
    return { pos, en, forms };
}

/**
 * @param {{ idx: Buffer, dict: Buffer, syn?: Buffer|null }} files dict is already unzipped.
 * @returns {{ he: string, heFold: string, pos: string, en: string }[]}
 */
function parseStarDict(files) {
    const index = readIndex(files.idx);
    const articles = index.map((entry) => {
        const html = files.dict.toString('utf8', entry.offset, entry.offset + entry.size);
        return readArticle(html);
    });
    const rows = [];
    const add = (word, article) => {
        if (!article) return;
        const he = word.normalize('NFC').replace(/\s+/g, ' ').trim();
        if (!he || [...he].length > HE_MAX) return;
        if (!/[\u05D0-\u05EA]/.test(he)) return;
        rows.push({ he, heFold: foldHebrew(he), pos: article.pos, en: article.en });
    };
    index.forEach((entry, i) => {
        add(entry.word, articles[i]);
        if (articles[i]) articles[i].forms.forEach((form) => add(form, articles[i]));
    });
    if (files.syn && files.syn.length) {
        readSynonyms(files.syn).forEach((syn) => add(syn.word, articles[syn.index]));
    }
    return rows;
}

/**
 * @param {{ idx: Buffer, dictDz: Buffer, synDz?: Buffer|null }} files
 * @returns {{ he: string, heFold: string, pos: string, en: string }[]}
 */
function parseStarDictZipped(files) {
    return parseStarDict({
        idx: files.idx,
        dict: zlib.gunzipSync(files.dictDz),
        syn: files.synDz ? zlib.gunzipSync(files.synDz) : null
    });
}

module.exports = { parseStarDict, parseStarDictZipped, readArticle, readIndex };
