/**
 * Explainer and personal-question renderers. Hostile plan text stays text.
 * Authored-by-spec: INIT-006/SPEC-014
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const explainer = require('../shared/ui/blocks/explainer');
const personal = require('../shared/ui/blocks/personal-question');
const ui = require('../shared/ui/plan-render');

const ROOT = path.resolve(__dirname, '..');
const CSS_PATH = path.join(ROOT, 'shared', 'worksheet.css');
const TOKENS_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0006-assets', 'tokens.json');
const CSS_START = '/* === INIT-006/SPEC-014 explainer and conjugation === */';
const CSS_END = '/* === end INIT-006/SPEC-014 explainer and conjugation === */';
const PAYLOAD = '<script>alert(1)</script>';
const HANDLER = '<img src=x onerror=alert(1)>';

function lin(channel) {
    const s = channel / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
    const h = hex.replace('#', '');
    const r = parseInt(h.slice(0, 2), 16);
    const g = parseInt(h.slice(2, 4), 16);
    const b = parseInt(h.slice(4, 6), 16);
    return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a, b) {
    const hi = Math.max(luminance(a), luminance(b));
    const lo = Math.min(luminance(a), luminance(b));
    return (hi + 0.05) / (lo + 0.05);
}

function cssBlock() {
    const css = fs.readFileSync(CSS_PATH, 'utf8');
    const start = css.indexOf(CSS_START);
    const end = css.indexOf(CSS_END);
    assert.ok(start > 0);
    assert.ok(end > start);
    return css.slice(start, end + CSS_END.length);
}

function count(html, needle) {
    let n = 0;
    let from = 0;
    while (from < html.length) {
        const at = html.indexOf(needle, from);
        if (at < 0) break;
        n += 1;
        from = at + needle.length;
    }
    return n;
}

function hostileExplainer() {
    return {
        id: PAYLOAD,
        type: 'explainer',
        title: PAYLOAD,
        image: { mediaId: 'x', alt: PAYLOAD, src: 'http://evil.example/a.png' },
        sections: [{
            id: PAYLOAD,
            heading: PAYLOAD,
            paragraphs: [PAYLOAD],
            examples: [{
                id: PAYLOAD,
                he: PAYLOAD,
                en: PAYLOAD,
                translit: PAYLOAD,
                root: PAYLOAD
            }],
            table: {
                columns: [PAYLOAD],
                rows: [[PAYLOAD]]
            },
            callouts: [{
                id: PAYLOAD,
                tone: 'green',
                title: PAYLOAD,
                text: PAYLOAD
            }]
        }]
    };
}

describe('explainer renderer', () => {
    it('escapes every string field and does not emit a picture', () => {
        const html = explainer.render(hostileExplainer(), {
            audio: { [PAYLOAD]: 'javascript:alert(1)' }
        });
        const escaped = explainer.escapeHtml(PAYLOAD);
        assert.equal(html.includes(PAYLOAD), false);
        assert.equal(html.includes(HANDLER), false);
        assert.ok(count(html, escaped) >= 12, String(count(html, escaped)));
        assert.equal(/<img\b/i.test(html), false);
        assert.equal(/<script/i.test(html), false);
        assert.equal(html.includes('onerror='), false);
        assert.equal(html.includes('javascript:'), false);
        assert.equal(html.includes('evil.example'), false);
        assert.match(html, /data-tone="green"/);
        assert.match(html, /callout-word/);
        assert.match(html, />Go</);
        assert.match(html, /קדימה/);
    });

    it('highlights root letters in order and ignores points', () => {
        const html = explainer.highlightRoot('חָשַׁב', 'ח.ש.ב');
        assert.match(html, /<span class="root-letter">חָ<\/span>/);
        assert.match(html, /<span class="root-letter">שַׁ<\/span>/);
        assert.match(html, /<span class="root-letter">ב<\/span>/);
        const skipped = explainer.highlightRoot('כּכ', 'כ.ת.ב');
        assert.equal(count(skipped, 'root-letter'), 1);
        assert.equal(explainer.highlightRoot('חָשַׁב', PAYLOAD).includes('<span'), false);
        assert.equal(explainer.highlightRoot(PAYLOAD, 'ח.ש.ב').includes(PAYLOAD), false);
        assert.match(explainer.highlightRoot(PAYLOAD, 'ח.ש.ב'), /&lt;script&gt;/);
    });

    it('plays a clip only when the path is a safe file, and never copies the path', () => {
        const block = hostileExplainer();
        block.sections[0].examples[0].he = 'חָשַׁב';
        block.sections[0].examples[0].root = 'ח.ש.ב';
        const html = explainer.render(block, { audio: { 'חָשַׁב': 'audio/think.mp3' } });
        assert.match(html, /data-plan-audio="ready"/);
        assert.equal(html.includes('audio/think.mp3'), false);
        assert.equal(html.includes('.mp3'), false);
        const refused = explainer.render(block, { audio: { 'חָשַׁב': '../secret.mp3' } });
        assert.equal(refused.includes('data-plan-audio="ready"'), false);
        assert.equal(refused.includes('secret.mp3'), false);
    });

    it('names every callout tone in words', () => {
        const tones = ['green', 'yellow', 'red', 'info'];
        const words = ['Go', 'Check', 'Stop', 'Note'];
        tones.forEach((tone, index) => {
            const block = hostileExplainer();
            block.sections[0].callouts[0].tone = tone;
            const html = explainer.render(block);
            assert.match(html, new RegExp('data-tone="' + tone + '"'));
            assert.match(html, new RegExp('>' + words[index] + '<'));
        });
    });

    it('is registered, and verse and letter blocks render too', () => {
        assert.equal(typeof ui.blocks.explainer.render, 'function');
        assert.match(ui.renderBlock({ type: 'explainer', id: 'e', title: 'כלל', sections: [] }), /data-block-type="explainer"/);
        assert.match(ui.renderBlock({
            type: 'verse-reading',
            id: 'v',
            verse: 'בראשית',
            ref: { book: 'Genesis', chapter: 1, from: 1, to: 1, source: 'WLC' }
        }), /data-block-type="verse-reading"/);
        assert.match(ui.renderBlock({
            type: 'letter-intro',
            id: 'l',
            letter: 'א',
            name: 'אָלֶף',
            sounds: [{ id: 's', vowel: 'קָמָץ', pointed: 'אָ', sound: 'a' }]
        }), /data-block-type="letter-intro"/);
    });
});

describe('callout contrast', () => {
    const tokens = JSON.parse(fs.readFileSync(TOKENS_PATH, 'utf8'));
    const byId = Object.create(null);
    tokens.tokens.forEach((token) => { byId[token.id] = token; });

    function pair(fg, bg) {
        return {
            light: contrast(byId[fg].light, byId[bg].light),
            dark: contrast(byId[fg].dark, byId[bg].dark)
        };
    }

    it('uses token pairs that clear 4.5:1 in light and dark, and no raw hex', () => {
        const css = cssBlock();
        assert.equal(/#[0-9a-fA-F]{3,8}/.test(css), false);
        assert.equal(/box-shadow/.test(css), false);
        ['--color-control', '--color-control-text', '--color-surface', '--color-text', '--color-bg', '--color-muted', '--color-line'].forEach((name) => {
            assert.ok(css.includes(name), name);
        });
        const pairs = [
            pair('color.controlText', 'color.control'),
            pair('color.text', 'color.surface'),
            pair('color.bg', 'color.text'),
            pair('color.muted', 'color.surface'),
            pair('color.text', 'color.bg')
        ];
        pairs.forEach((ratio) => {
            assert.ok(ratio.light >= 4.5, String(ratio.light));
            assert.ok(ratio.dark >= 4.5, String(ratio.dark));
        });
    });
});

describe('personal question', () => {
    it('escapes prompts, offers say-aloud, and keeps a text box only when typing is allowed', () => {
        const speak = personal.render({
            id: PAYLOAD,
            type: 'personal-question',
            answer: 'speak',
            prompts: [{ id: PAYLOAD, he: PAYLOAD, en: PAYLOAD }],
            image: { mediaId: 'x', alt: 'hidden' }
        });
        assert.equal(speak.includes(PAYLOAD), false);
        assert.ok(speak.includes(personal.escapeHtml(PAYLOAD)));
        assert.match(speak, /data-say-aloud/);
        assert.equal(speak.includes('data-personal-input'), false);
        assert.equal(/<img\b/i.test(speak), false);
        assert.match(speak, /personal-en[^>]*hidden/);

        const typed = personal.render({
            id: 'you',
            type: 'personal-question',
            answer: 'either',
            prompts: [{ id: 'p1', he: 'מָה שְׁמְךָ?', en: 'What is your name?' }]
        }, { lesson: { id: 'public-lesson' } });
        assert.match(typed, /data-personal-input/);
        assert.match(typed, /data-lesson-slug="public-lesson"/);
        assert.match(typed, /data-personal-done/);
    });

    it('stores a done boolean and drops typed text', () => {
        const secret = 'pq-secret-typed';
        const patch = personal.donePatch({
            lesson: 'public-lesson',
            activityState: {
                'personal-question': { leaked: secret }
            }
        }, 'you');
        const json = JSON.stringify(patch);
        assert.equal(json.includes(secret), false);
        assert.equal(patch.activityState['personal-question'].you, true);
        assert.equal(patch.activityState['personal-question'].leaked, undefined);
        assert.equal(patch.lesson, 'public-lesson');
    });
});
