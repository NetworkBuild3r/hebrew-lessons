/**
 * INIT-009/SPEC-002 — resolveSteps honors stored activities (including []).
 * Legacy worksheets with no activities key still use fallbackActivities.
 */
const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const { parseDataWindow } = require('../server/lesson-data');

const root = path.resolve(__dirname, '..');

function loadPlayer() {
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        localStorage: {
            _d: {},
            getItem(k) { return Object.prototype.hasOwnProperty.call(this._d, k) ? this._d[k] : null; },
            setItem(k, v) { this._d[k] = String(v); },
            removeItem(k) { delete this._d[k]; }
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    sandbox.module = { exports: {} };
    sandbox.exports = sandbox.module.exports;

    const files = [
        path.join(root, 'shared', 'player', 'utils.js'),
        path.join(root, 'shared', 'activities', 'catalog.js'),
        path.join(root, 'shared', 'player', 'core.js'),
        path.join(root, 'shared', 'activities', 'words.js'),
        path.join(root, 'shared', 'activities', 'match.js'),
        path.join(root, 'shared', 'activities', 'story.js'),
        path.join(root, 'shared', 'activities', 'fill.js'),
        path.join(root, 'shared', 'activities', 'pattern.js'),
        path.join(root, 'shared', 'activities', 'retell.js'),
        path.join(root, 'shared', 'activities', 'ask-friend.js')
    ];
    for (const file of files) {
        vm.runInNewContext(fs.readFileSync(file, 'utf8'), sandbox, { filename: file });
    }
    return sandbox;
}

function stepTypes(resolved) {
    return resolved.steps.map((s) => s.type);
}

describe('player resolveSteps (INIT-009/SPEC-002)', () => {
    it('activities: [] yields zero steps and no fill/grammar/reading/ask-friend ladder', () => {
        const sandbox = loadPlayer();
        const ws = {
            id: 'empty-acts',
            level: 'basic',
            activities: [],
            vocab: { items: [{ id: '1', word: 'קָם', meaning: 'gets up' }] },
            fill: { sentences: [{ id: '1', text: '___' }] },
            grammar: { items: [{ id: '1', prompt: 'x' }] },
            reading: { paragraphs: ['שלום'], retell: 'retell' },
            askFriend: { questions: [{ id: '1', prompt: 'q' }] }
        };
        const resolved = sandbox.HebrewPlayer.resolveSteps(ws);
        assert.equal(resolved.steps.length, 0);
        const types = stepTypes(resolved);
        assert.equal(types.includes('fill'), false);
        assert.equal(types.includes('pattern'), false);
        assert.equal(types.includes('story'), false);
        assert.equal(types.includes('retell'), false);
        assert.equal(types.includes('ask-friend'), false);
        assert.equal(types.includes('words'), false);
    });

    it('activities: [{ type: "story" }] yields only that step', () => {
        const sandbox = loadPlayer();
        const ws = {
            id: 'story-only',
            level: 'basic',
            activities: [{ type: 'story' }],
            vocab: { items: [{ id: '1', word: 'קָם', meaning: 'gets up' }] },
            fill: { sentences: [{ id: '1', text: '___' }] },
            grammar: { items: [{ id: '1', prompt: 'x' }] },
            reading: { paragraphs: ['שלום'], retell: 'retell' },
            askFriend: { questions: [{ id: '1', prompt: 'q' }] }
        };
        const resolved = sandbox.HebrewPlayer.resolveSteps(ws);
        assert.equal(resolved.steps.length, 1);
        assert.equal(resolved.steps[0].type, 'story');
    });

    it('no activities key with vocab.items still includes a words step (legacy fallback)', () => {
        const sandbox = loadPlayer();
        const ws = {
            id: 'legacy-vocab',
            level: 'basic',
            vocab: { items: [{ id: '1', word: 'קָם', meaning: 'gets up' }] }
        };
        const resolved = sandbox.HebrewPlayer.resolveSteps(ws);
        assert.ok(stepTypes(resolved).includes('words'));
    });

    it('three published worksheets still resolve as expected', () => {
        const sandbox = loadPlayer();
        const published = ['my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech'];
        for (const slug of published) {
            const file = path.join(root, 'worksheets', slug, 'worksheet.js');
            const parsed = parseDataWindow(fs.readFileSync(file, 'utf8'), file);
            const ws = parsed.WORKSHEET;
            assert.ok(ws && ws.id, slug);
            const types = stepTypes(sandbox.HebrewPlayer.resolveSteps(ws));
            if (slug === 'shai-date-indirect-speech') {
                assert.equal(Array.isArray(ws.activities), false, 'shai has no activities key');
                assert.ok(types.includes('words'), 'shai legacy words');
                assert.ok(types.includes('fill') || types.includes('pattern') || types.includes('story')
                    || types.includes('ask-friend'), 'shai legacy ladder from sections');
            } else {
                assert.ok(Array.isArray(ws.activities) && ws.activities.length > 0, slug + ' has activities');
                assert.equal(types.length > 0, true, slug);
                assert.ok(types.includes('words') || types.includes(ws.activities[0].type), slug);
            }
        }
    });
});
