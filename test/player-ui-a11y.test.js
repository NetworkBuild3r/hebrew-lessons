/**
 * INIT-007/SPEC-007 — player touch targets, contrast, reduced-motion, ADR-0006 hooks.
 * Contrast math matches spikes/look/check.js (relative luminance). No new deps.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const CSS_PATH = path.join(ROOT, 'shared', 'worksheet.css');
const TOKENS_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0006-assets', 'tokens.json');
const ADR_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0006-visual-language-and-interface-rules.md');
const SHARED = path.join(ROOT, 'shared');

const TEXT_RATIO = 4.5;
const UI_RATIO = 3;
const PRIMARY_PX = 44;

function read(file) {
    return fs.readFileSync(file, 'utf8');
}

function lin(channel) {
    const s = channel / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
    const h = hex.replace('#', '');
    const full = h.length === 3 ? h.split('').map((c) => c + c).join('') : h;
    const r = parseInt(full.slice(0, 2), 16);
    const g = parseInt(full.slice(2, 4), 16);
    const b = parseInt(full.slice(4, 6), 16);
    return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a, b) {
    const L1 = luminance(a);
    const L2 = luminance(b);
    const hi = Math.max(L1, L2);
    const lo = Math.min(L1, L2);
    return (hi + 0.05) / (lo + 0.05);
}

function balancedBlock(css, startBrace) {
    let depth = 0;
    for (let i = startBrace; i < css.length; i++) {
        if (css[i] === '{') depth += 1;
        else if (css[i] === '}') {
            depth -= 1;
            if (depth === 0) return css.slice(startBrace, i + 1);
        }
    }
    return '';
}

function stripComments(css) {
    return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

function walkSharedFiles(dir, out) {
    for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name);
        const st = fs.statSync(full);
        if (st.isDirectory()) walkSharedFiles(full, out);
        else if (/\.(js|css|html)$/.test(name)) out.push(full);
    }
    return out;
}

function parseHookContract(adrMarkdown) {
    const attrs = [];
    const classes = [];
    const attrSection = adrMarkdown.match(
        /#### `data-\*` attributes \(70, all kept\)([\s\S]*?)#### Classes \(50, all kept\)/
    );
    const classSection = adrMarkdown.match(
        /#### Classes \(50, all kept\)([\s\S]*?)#### Retired/
    );
    assert.ok(attrSection, 'ADR-0006 data-* section');
    assert.ok(classSection, 'ADR-0006 classes section');
    for (const m of attrSection[1].matchAll(/\| `([a-z0-9-]+)` \|/g)) {
        if (m[1].startsWith('data-')) attrs.push(m[1]);
    }
    for (const m of classSection[1].matchAll(/\| `([A-Za-z][\w-]*)` \|/g)) {
        classes.push(m[1]);
    }
    return { attrs: [...new Set(attrs)], classes: [...new Set(classes)] };
}

function cssDeclBlock(css, selectorNeedle) {
    const idx = css.indexOf(selectorNeedle);
    if (idx === -1) return '';
    const brace = css.indexOf('{', idx);
    if (brace === -1) return '';
    return balancedBlock(css, brace);
}

/** Prefer the rule block for `selectorNeedle` that also matches `mustContain`. */
function cssDeclBlockMatching(css, selectorNeedle, mustContain) {
    let from = 0;
    while (from < css.length) {
        const idx = css.indexOf(selectorNeedle, from);
        if (idx === -1) return '';
        const brace = css.indexOf('{', idx);
        const block = balancedBlock(css, brace);
        if (!mustContain || block.includes(mustContain)) return block;
        from = brace + block.length;
    }
    return '';
}

function minPxFromDecl(block, prop) {
    const re = new RegExp(prop + '\\s*:\\s*([^;]+);');
    const m = block.match(re);
    if (!m) return null;
    const raw = m[1].trim();
    const px = raw.match(/^(\d+(?:\.\d+)?)px$/);
    if (px) return Number(px[1]);
    if (/var\(--target-primary/.test(raw) || /var\(--size-target-primary/.test(raw)) return 44;
    if (/var\(--size-target\)/.test(raw) || /var\(--target-quiet/.test(raw)) return 24;
    return null;
}

describe('INIT-007/SPEC-007 player UI a11y', () => {
    const css = read(CSS_PATH);
    const cssBare = stripComments(css);
    const tokensDoc = JSON.parse(read(TOKENS_PATH));
    const adr = read(ADR_PATH);

    it('ac-1: primary interactive targets declare ≥44×44 CSS px', () => {
        const primarySelectors = [
            { sel: '.btn {', props: ['min-height'], note: 'Play/Next/Check/Continue family', must: 'min-height' },
            { sel: '.icon-btn {', props: ['width', 'height'], note: 'audio Play icon', must: 'width: 44px' },
            { sel: '.btn-vowels {', props: ['min-height', 'min-width'], note: 'vowel toggle' },
            { sel: '.player-exit {', props: ['min-height'], note: 'Back/exit' },
            { sel: '.player-top .back-link {', props: ['min-height'], note: 'Back link' },
            { sel: '.action-bar-inner .btn {', props: ['min-height'], note: 'Continue dock' },
            { sel: '.path-btn {', props: ['min-height'], note: 'path step buttons' },
            { sel: '.word-match-card, .choice-card, .bank-word {', props: ['min-height'], note: 'drill cards' },
            { sel: '.review-grade {', props: ['min-height', 'min-width'], note: 'Knew/Missed grades', must: 'min-width: 48px' },
            { sel: '.review-audio {', props: ['min-height'], note: 'review Play', must: 'min-height: 48px' },
            { sel: '.feedback-retry {', props: ['min-height'], note: 'retry after miss' },
            {
                sel: '.review-summary-actions .btn,',
                props: ['min-height'],
                note: 'Back to Today',
                must: 'min-height: 48px'
            },
            { sel: '.story-gloss-actions .icon-btn {', props: ['min-width', 'min-height'], note: 'gloss Play' }
        ];

        for (const row of primarySelectors) {
            const block = row.must
                ? cssDeclBlockMatching(cssBare, row.sel, row.must)
                : cssDeclBlock(cssBare, row.sel);
            assert.ok(block, 'missing selector block for ' + row.note + ' (' + row.sel.trim() + ')');
            for (const prop of row.props) {
                const px = minPxFromDecl(block, prop);
                assert.ok(px != null, row.note + ' missing ' + prop);
                assert.ok(
                    px + 1e-9 >= PRIMARY_PX,
                    row.note + ' ' + prop + '=' + px + 'px (need ≥' + PRIMARY_PX + ')'
                );
            }
        }

        assert.match(css, /--target-primary:\s*48px/);
        assert.match(css, /--size-target-primary:\s*44px/);
        assert.match(css, /--size-target:\s*24px/);

        // Preserve path-btn ink (operator constraint)
        const pathBtn = cssDeclBlock(cssBare, '.path-btn {');
        assert.match(pathBtn, /color:\s*var\(--ink\)/);
        assert.match(cssBare, /\.path-btn \.path-label \{[^}]*color:\s*var\(--ink\)/);
    });

    it('ac-2: pointed Hebrew / ink contrast ≥4.5:1 (light + dark); UI chrome ≥3:1', () => {
        const byId = new Map(tokensDoc.tokens.map((t) => [t.id, t]));
        for (const row of tokensDoc.contrast) {
            const [leftId, rightId] = row.pair;
            const left = byId.get(leftId);
            const right = byId.get(rightId);
            assert.ok(left && right, row.pair.join('/'));
            const minimum = Number(row.minimum);
            const lightRatio = contrast(left.light, right.light);
            const darkRatio = contrast(left.dark, right.dark);
            assert.ok(
                lightRatio + 1e-9 >= minimum,
                row.pair.join('/') + ' light ' + lightRatio.toFixed(2)
            );
            assert.ok(
                darkRatio + 1e-9 >= minimum,
                row.pair.join('/') + ' dark ' + darkRatio.toFixed(2)
            );
        }

        // Player surfaces: legacy ink + ADR-0010 pointed text on card/paper/story
        const playerPairs = [
            ['#1e293b', '#ffffff', TEXT_RATIO, 'ink/card'],
            ['#1e293b', '#faf8f4', TEXT_RATIO, 'ink/paper'],
            ['#1e293b', '#f8fafc', TEXT_RATIO, 'ink/bg'],
            ['#1c1917', '#ffffff', TEXT_RATIO, 'color-text/surface light'],
            ['#1c1917', '#f7f6f3', TEXT_RATIO, 'color-text/bg light'],
            ['#1c1917', '#fffbeb', TEXT_RATIO, 'color-text/story light'],
            ['#f6f3ec', '#1e1e1c', TEXT_RATIO, 'color-text/surface dark'],
            ['#f6f3ec', '#141413', TEXT_RATIO, 'color-text/bg dark'],
            ['#f6f3ec', '#23211c', TEXT_RATIO, 'color-text/story dark'],
            ['#8a847c', '#ffffff', UI_RATIO, 'line/surface light'],
            ['#8a847c', '#f7f6f3', UI_RATIO, 'line/bg light'],
            ['#6e6a64', '#1e1e1c', UI_RATIO, 'line/surface dark'],
            ['#6e6a64', '#141413', UI_RATIO, 'line/bg dark'],
            ['#1e3a2f', '#ffffff', UI_RATIO, 'focus/surface light'],
            ['#e4f2e9', '#1e1e1c', UI_RATIO, 'focus/surface dark']
        ];
        for (const [fg, bg, minimum, label] of playerPairs) {
            const ratio = contrast(fg, bg);
            assert.ok(ratio + 1e-9 >= minimum, label + ' ' + ratio.toFixed(2) + ':1');
        }

        assert.match(cssBare, /\.he-word \{[\s\S]*?color:\s*var\(--color-text/);
        assert.match(cssBare, /\.correction-word \{[\s\S]*?font-size:\s*var\(--size-word/);
    });

    it('ac-3: prefers-reduced-motion: reduce disables animations and transitions', () => {
        const reduceNeedle = '@media (prefers-reduced-motion: reduce)';
        assert.ok(cssBare.includes(reduceNeedle), 'missing reduce media query');

        // Global token block zeros all motion
        const tokenReduceAt = css.indexOf('/* === INIT-004/SPEC-013 ADR-0006 tokens === */');
        const tokenReduceEnd = css.indexOf('/* === end INIT-004/SPEC-013 ADR-0006 tokens === */');
        const tokenBlock = css.slice(tokenReduceAt, tokenReduceEnd);
        const reduceAt = tokenBlock.indexOf(reduceNeedle);
        assert.ok(reduceAt >= 0, 'SPEC-013 reduce block');
        const reduceBlock = balancedBlock(tokenBlock, tokenBlock.indexOf('{', reduceAt));
        assert.match(reduceBlock, /animation:\s*none/);
        assert.match(reduceBlock, /transition:\s*none/);

        // INIT-007 player sections also zero their own motion under reduce
        const playerReduceMarkers = [
            'INIT-007/SPEC-002',
            'INIT-007/SPEC-003',
            'INIT-007/SPEC-004',
            'INIT-007/SPEC-005',
            'INIT-007/SPEC-006'
        ];
        for (const marker of playerReduceMarkers) {
            // SPEC-003 uses early player shell reduce near step-enter
            if (marker === 'INIT-007/SPEC-003') {
                assert.match(css, /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.step-screen\.step-enter[\s\S]*?animation:\s*none/);
                assert.match(css, /--motion-step:\s*0s/);
                continue;
            }
            const start = css.indexOf(marker);
            assert.ok(start >= 0, marker + ' ownership block');
            const slice = css.slice(start, start + 12000);
            assert.ok(slice.includes(reduceNeedle), marker + ' missing reduce query');
            const at = slice.indexOf(reduceNeedle);
            const block = balancedBlock(slice, slice.indexOf('{', at));
            assert.ok(
                /animation:\s*none/.test(block) || /transition:\s*none/.test(block),
                marker + ' reduce must clear animation or transition'
            );
        }

        assert.match(css, /--motion-fast:\s*0s none/);
    });

    it('ac-5: ADR-0006 hook contract (70 data-* + 50 classes) still present in shared/', () => {
        const { attrs, classes } = parseHookContract(adr);
        assert.equal(attrs.length, 70, 'documented data-* count');
        assert.equal(classes.length, 50, 'documented class count');
        assert.equal(attrs.length + classes.length, 120, 'kept hooks (retired set-dir is separate)');

        const files = walkSharedFiles(SHARED, []);
        // Also scan chrome/unit tests that assert hooks, so admin-only hooks still count as present
        const testDir = path.join(ROOT, 'test');
        for (const name of fs.readdirSync(testDir)) {
            if (name.endsWith('.js')) files.push(path.join(testDir, name));
        }
        const corpus = files.map(read).join('\n');

        const missingAttrs = attrs.filter((name) => !corpus.includes(name));
        const missingClasses = classes.filter((name) => {
            // Class hooks appear as .name in CSS/tests or as string class tokens in JS
            const asClass = new RegExp(
                '(?:[\'"`\\s.]' + name.replace(/-/g, '\\-') + '(?:[\'"`\\s.])|class(?:Name|List)?[^\\n]{0,80}' + name.replace(/-/g, '\\-') + ')'
            );
            return !asClass.test(corpus) && !corpus.includes('.' + name) && !corpus.includes("'" + name + "'")
                && !corpus.includes('"' + name + '"') && !corpus.includes('`' + name + '`');
        });

        assert.deepEqual(missingAttrs, [], 'missing data-* hooks: ' + missingAttrs.join(', '));
        assert.deepEqual(missingClasses, [], 'missing class hooks: ' + missingClasses.join(', '));

        // ADR documents set-dir as the one retired value (not a kept hook).
        assert.match(adr, /`data-action="set-dir"`[\s\S]*?\| retire \|/);
        // Existing production-recall contract still forbids the value in rendered markup.
        const recallTest = read(path.join(ROOT, 'test', 'production-recall.test.js'));
        assert.match(recallTest, /doesNotMatch\(html, \/data-action="set-dir"\/\)/);

        // build-sentence playTarget still present
        const buildSentence = read(path.join(SHARED, 'activities', 'build-sentence.js'));
        assert.match(buildSentence, /function playTarget\b/);
        assert.match(buildSentence, /playTarget\(ctx\.Speech/);
    });
});
