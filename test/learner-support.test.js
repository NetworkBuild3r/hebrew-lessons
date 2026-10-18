/**
 * Learner-support changes (2026-10-08): match pairs, Type it hints, story questions.
 * Rendering and state logic only. The click-through lives in test/chrome-learner-support-smoke.js.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const path = require('path');
const fs = require('fs');
const vm = require('vm');

const bundle = require('./helpers/player-bundle');
const speechBundle = bundle.ensurePlayerBundle();

function loadActivity(file, extra) {
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        module: { exports: {} },
        exports: {},
        localStorage: {
            _d: {},
            getItem(k) { return this._d[k] || null; },
            setItem(k, v) { this._d[k] = String(v); }
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    let def = null;
    sandbox.HebrewPlayer = { register(d) { def = d; } };
    const files = [
        path.join(__dirname, '../shared/player/utils.js'),
        ...(extra || []),
        path.join(__dirname, '../shared/activities/' + file)
    ];
    for (const f of files) {
        vm.runInNewContext(fs.readFileSync(f, 'utf8'), sandbox, { filename: f });
    }
    return { def, sandbox };
}

function makeCtx(sandbox, ws) {
    const U = sandbox.HebrewUtils;
    return {
        ws,
        esc: U.escapeHtml,
        bi: U.bi,
        sid: U.sid,
        speakButton: (text) => '<button data-speak="' + U.escapeHtml(text) + '"></button>',
        Speech: { play() {} },
        stepIndex: 0,
        markComplete() {},
        save() {}
    };
}

const WS = {
    id: 'x',
    level: 'basic',
    vocab: {
        items: [
            { id: 1, word: 'קָם', meaning: 'gets up' },
            { id: 2, word: 'בֹּקֶר', meaning: 'morning' },
            { id: 3, word: 'מַיִם', meaning: 'water' },
            { id: 4, word: 'לֶחֶם', meaning: 'bread' }
        ]
    },
    reading: {
        paragraphs: ['קם בבוקר'],
        questions: [{ id: 1, question: 'מי קם?' }]
    }
};

describe('match pairs', () => {
    it('starts with every word in the columns and no pairs list', () => {
        const { def, sandbox } = loadActivity('match.js');
        const html = def.render(makeCtx(sandbox, WS), {}, {});
        assert.equal((html.match(/data-match="en"/g) || []).length, 4);
        assert.equal((html.match(/data-match="he"/g) || []).length, 4);
        assert.match(html, /id="match-pairs"/);
        assert.match(html, /class="match-pairs hidden"/);
        assert.equal((html.match(/class="match-pair /g) || []).length, 0);
    });

    it('moves saved pairs to the side-by-side list in match order, one colour each', () => {
        const { def, sandbox } = loadActivity('match.js');
        const html = def.render(makeCtx(sandbox, WS), {}, { matchedPairs: ['3', '1'] });
        assert.equal((html.match(/data-match="en"/g) || []).length, 2, 'matched words leave the columns');
        assert.ok(!/data-match="he" data-id="3"/.test(html));
        assert.ok(!/data-match="he" data-id="1"/.test(html));
        const rows = html.match(/<li class="match-pair [^"]*"/g);
        assert.deepEqual(rows, [
            '<li class="match-pair pair-c0"',
            '<li class="match-pair pair-c1"'
        ]);
        assert.ok(html.indexOf('water') < html.indexOf('gets up'), 'first matched pair is listed first');
        assert.match(html, /class="match-pairs"/);
        assert.match(html, /pair-en/);
        assert.match(html, /pair-he/);
        assert.match(html, /data-speak=/);
    });

    it('hides the grid and says all matched when nothing is left', () => {
        const { def, sandbox } = loadActivity('match.js');
        const html = def.render(makeCtx(sandbox, WS), {}, { matchedPairs: ['1', '2', '3', '4'] });
        assert.match(html, /class="match-grid hidden"/);
        assert.match(html, /All matched/);
        assert.equal((html.match(/data-match="en"/g) || []).length, 0);
    });

    it('ignores saved ids that are not in the lesson', () => {
        const { def, sandbox } = loadActivity('match.js');
        const html = def.render(makeCtx(sandbox, WS), {}, { matchedPairs: ['99', '2'] });
        assert.equal((html.match(/<li class="match-pair /g) || []).length, 1);
        assert.equal((html.match(/data-match="en"/g) || []).length, 3);
    });

    it('collect keeps the order the pairs were matched in', () => {
        const { def, sandbox } = loadActivity('match.js');
        const ctx = makeCtx(sandbox, WS);
        const root = { _match: { matched: new Set(['4', '2', '1']) } };
        // Arrays built inside the VM context have a different prototype; compare as plain data.
        const out = JSON.parse(JSON.stringify(def.collect(ctx, root)));
        assert.deepEqual(out, { matchedPairs: ['4', '2', '1'] });
    });

    it('defines a distinct colour for each of the eight pair classes', () => {
        const css = fs.readFileSync(path.join(__dirname, '../shared/worksheet.css'), 'utf8');
        const edges = new Set();
        for (let i = 0; i < 8; i += 1) {
            const m = css.match(new RegExp('\\.match-pair\\.pair-c' + i + ' \\{ --pair-edge: (#[0-9a-f]{6});'));
            assert.ok(m, 'pair-c' + i + ' is defined');
            edges.add(m[1]);
        }
        assert.equal(edges.size, 8);
    });
});

describe('story comprehension questions', () => {
    it('collapses the questions by default but keeps the answer boxes in the page', () => {
        const { def, sandbox } = loadActivity('story.js', [
            speechBundle
        ]);
        const ctx = makeCtx(sandbox, WS);
        ctx.vowelsOn = true;
        const html = def.render(ctx, {}, {});
        assert.match(html, /id="story-questions-card"/);
        assert.match(html, /class="card stack hidden" id="story-questions-card"/);
        assert.match(html, /<textarea id="reading-ans-1"/);
        assert.match(html, /Show comprehension questions/);
        assert.match(html, /aria-expanded="false"/);
    });

    it('opens by default once the learner chose to show them', () => {
        const { def, sandbox } = loadActivity('story.js', [
            speechBundle
        ]);
        sandbox.localStorage.setItem('hebrew-worksheet-prefs', JSON.stringify({ questionsOpen: true }));
        const ctx = makeCtx(sandbox, WS);
        ctx.vowelsOn = true;
        const html = def.render(ctx, {}, {});
        assert.match(html, /class="card stack" id="story-questions-card"/);
        assert.match(html, /Hide comprehension questions/);
        assert.match(html, /aria-expanded="true"/);
    });

    it('shows no toggle when a lesson has no questions', () => {
        const { def, sandbox } = loadActivity('story.js', [
            speechBundle
        ]);
        const ctx = makeCtx(sandbox, { ...WS, reading: { paragraphs: ['קם'] } });
        ctx.vowelsOn = true;
        const html = def.render(ctx, {}, {});
        assert.ok(!/story-questions-card/.test(html));
    });
});
