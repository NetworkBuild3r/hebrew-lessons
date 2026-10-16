/**
 * Draft word meanings for a story. The teacher (or the AI) reads the draft,
 * fixes what is wrong for this text, and saves it as the block's glosses. A
 * draft never writes to a plan.
 */
'use strict';

const wordLookup = require('./lookup');
const { foldHebrew } = require('./parse-dsl');

const MAX_WORDS = 80;
const TOKEN = /[\u05D0-\u05EA][\u05D0-\u05EA\u0591-\u05C7\u05BE'\u05F3\u05F4"]*/g;

/**
 * Hebrew words in a text, once each, in reading order, as written.
 * @param {unknown} text
 * @returns {string[]}
 */
function tokensOf(text) {
    const seen = new Set();
    const out = [];
    const raw = typeof text === 'string' ? text.normalize('NFC') : '';
    (raw.match(TOKEN) || []).forEach((token) => {
        const word = token.replace(/[\u05BE'\u05F3\u05F4"]+$/, '');
        const fold = foldHebrew(word);
        if ([...fold].length < 2 || seen.has(fold)) return;
        seen.add(fold);
        out.push(word);
    });
    return out;
}

/**
 * @param {string} text
 * @param {{ skip?: string[], max?: number, lookup?: Function }} [options]
 *   skip: words that already have a meaning (any spelling, pointed or not).
 * @returns {{
 *   suggestions: { word: string, meaning: string, form: string|null, senses: { pos: string, en: string }[] }[],
 *   unmatched: string[],
 *   skipped: number
 * }}
 */
function suggestGlosses(text, options) {
    const opts = options || {};
    const find = typeof opts.lookup === 'function' ? opts.lookup : wordLookup.lookup;
    const max = Math.min(Number.isInteger(opts.max) ? opts.max : MAX_WORDS, MAX_WORDS);
    const have = new Set((opts.skip || []).map((word) => foldHebrew(String(word))));
    const result = { suggestions: [], unmatched: [], skipped: 0 };
    tokensOf(text).forEach((word) => {
        if (have.has(foldHebrew(word))) {
            result.skipped += 1;
            return;
        }
        if (result.suggestions.length + result.unmatched.length >= max) return;
        const found = find(word);
        if (found && found.short) {
            result.suggestions.push({
                word,
                meaning: found.short,
                form: found.form || null,
                senses: (found.meanings || []).slice(0, 4).map((m) => ({ pos: m.pos || '', en: m.en }))
            });
        } else {
            result.unmatched.push(word);
        }
    });
    return result;
}

/**
 * Draft meanings for one graded-story block: every word in its paragraphs that
 * has no gloss yet and is not in its support glossary.
 * @param {object} block
 * @param {{ lookup?: Function }} [options]
 */
function suggestForBlock(block, options) {
    const story = block && typeof block === 'object' ? block : {};
    const paragraphs = Array.isArray(story.paragraphs) ? story.paragraphs : [];
    const text = paragraphs.map((p) => (p && typeof p.text === 'string' ? p.text : '')).join(' ');
    const glossed = story.glosses && typeof story.glosses === 'object' && !Array.isArray(story.glosses)
        ? Object.keys(story.glosses) : [];
    const support = (Array.isArray(story.support) ? story.support : [])
        .map((pair) => (pair && typeof pair.he === 'string' ? pair.he : ''))
        .filter(Boolean);
    const room = Math.max(0, MAX_WORDS - glossed.length);
    return suggestGlosses(text, {
        skip: glossed.concat(support),
        max: room,
        lookup: options && options.lookup
    });
}

module.exports = { suggestForBlock, suggestGlosses, tokensOf, MAX_WORDS };
