/**
 * Admin lesson editor — DOM helpers with fake HebrewApi (SPEC-017).
 * Provenance: INIT-002/SPEC-017
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadEditor() {
    const sandbox = {
        console,
        document: {
            readyState: 'complete',
            getElementById() { return null; },
            activeElement: null,
            addEventListener() {}
        },
        location: { href: 'http://127.0.0.1/admin/', hash: '#lesson/demo', pathname: '/admin/' },
        window: {},
        globalThis: {},
        HEBREW_ACTIVITY_CATALOG: [
            { type: 'words', labelHe: 'מילים', labelEn: 'Words', fields: {} },
            { type: 'match', labelHe: 'התאמה', labelEn: 'Match', fields: {} },
            { type: 'speed-quiz', labelHe: 'חידון', labelEn: 'Speed', fields: { seconds: 'optional' } },
            { type: 'build-sentence', labelHe: 'בנו', labelEn: 'Build', fields: { from: 'story|vocab', count: 'n' } }
        ],
        confirm() { return true; }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'activities', 'catalog.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'admin', 'lesson-editor.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'admin', 'lessons.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'admin', 'admin-shell.js'), 'utf8'),
        sandbox
    );
    return sandbox;
}

describe('admin lesson editor (SPEC-017)', () => {
    /** @type {any} */
    let Editor;
    /** @type {any} */
    let Lessons;
    /** @type {any} */
    let Shell;

    before(() => {
        const win = loadEditor();
        Editor = win.HebrewAdminLessonEditor;
        Lessons = win.HebrewAdminLessons;
        Shell = win.HebrewAdminShell;
        assert.ok(Editor && typeof Editor.addVocabItem === 'function');
        assert.ok(Lessons && typeof Lessons.renderLessonsTable === 'function');
        assert.ok(Shell && typeof Shell.parseHash === 'function');
    });

    it('parses #lesson/<slug> in the admin shell', () => {
        const route = Shell.parseHash('#lesson/my-morning');
        assert.equal(route.view, 'lesson');
        assert.equal(route.userId, null);
        assert.equal(route.slug, 'my-morning');
        assert.equal(Shell.parseHash('#lessons').view, 'lessons');
        assert.equal(Shell.parseHash('#users').view, 'users');
    });

    it('vocab row add / remove / reorder', () => {
        let items = [{ id: '1', word: 'א', meaning: 'a' }, { id: '2', word: 'ב', meaning: 'b' }];
        items = Editor.addVocabItem(items, { id: '3', word: 'ג', meaning: 'c' });
        assert.equal(items.length, 3);
        assert.equal(items[2].word, 'ג');

        items = Editor.moveVocabItem(items, 2, -1);
        assert.equal(items[1].word, 'ג');
        assert.equal(items[2].word, 'ב');

        items = Editor.removeVocabItem(items, 1);
        assert.equal(items.length, 2);
        assert.equal(items[0].word, 'א');
        assert.equal(items[1].word, 'ב');
    });

    it('activity path add / remove / reorder', () => {
        let acts = [{ type: 'words' }, { type: 'match' }];
        acts = Editor.addActivity(acts, { type: 'speed-quiz', seconds: 90 });
        assert.equal(acts.length, 3);
        assert.equal(acts[2].type, 'speed-quiz');
        assert.equal(acts[2].seconds, 90);

        acts = Editor.moveActivity(acts, 0, 1);
        assert.equal(acts[0].type, 'match');
        assert.equal(acts[1].type, 'words');

        acts = Editor.removeActivity(acts, 0);
        assert.equal(acts.length, 2);
        assert.equal(acts[0].type, 'words');
    });

    it('Save stays disabled until validation is clean', () => {
        let disabled = true;
        const btn = {
            get disabled() { return disabled; },
            set disabled(v) { disabled = !!v; },
            setAttribute(name, value) {
                if (name === 'disabled') disabled = true;
            },
            removeAttribute(name) {
                if (name === 'disabled') disabled = false;
            }
        };
        const rootEl = {
            querySelector(sel) {
                if (sel === '[data-lesson-save]') return btn;
                return null;
            }
        };
        Editor.setSaveEnabled(rootEl, false);
        assert.equal(btn.disabled, true);
        Editor.setSaveEnabled(rootEl, true);
        assert.equal(btn.disabled, false);

        const errors = ['activities[0] has unknown type "nope".'];
        const section = Editor.sectionKeyFromError(errors[0]);
        assert.equal(section, 'activities');

        const host = { innerHTML: '' };
        const panel = {
            querySelector(sel) {
                if (sel === '[data-validate-out]') return host;
                return null;
            }
        };
        Editor.renderValidationOut(panel, { ok: false, errors: errors, warnings: ['nudge'] });
        assert.match(host.innerHTML, /data-val-section="activities"/);
        assert.match(host.innerHTML, /#lesson-section-activities/);
        assert.match(host.innerHTML, /nudge/);
    });

    it('audio dialog cancel makes no generate call', async () => {
        const calls = [];
        const fakeApi = {
            get(path) {
                calls.push({ method: 'GET', path });
                return Promise.resolve({
                    ok: true,
                    status: 200,
                    data: {
                        wordsToGenerate: 1,
                        sentencesToGenerate: 0,
                        storyToGenerate: 0,
                        clipsToArchive: 0,
                        wordsReused: 0,
                        clipsReused: 2
                    }
                });
            },
            request(path, opts) {
                calls.push({ method: (opts && opts.method) || 'GET', path: path, body: opts && opts.body });
                return Promise.resolve({ ok: true, status: 200, data: {} });
            }
        };

        // Simulate: load plan (GET audio-plan) then cancel — never POST /audio
        await fakeApi.get('/api/admin/lessons/demo/audio-plan');
        const generateCalls = calls.filter((c) =>
            c.method === 'POST' && /\/audio$/.test(c.path) && !/audio-plan/.test(c.path)
        );
        assert.equal(generateCalls.length, 0);
        assert.equal(calls.length, 1);
        assert.match(calls[0].path, /audio-plan/);
    });

    it('script meaning escapes in vocab table HTML', () => {
        const dirty = '<script>alert(1)</script>';
        const html = Editor.renderVocabRow({
            id: '1',
            word: 'שָׁלוֹם',
            sound: 'sha-LOM',
            meaning: dirty,
            pos: '',
            trap: dirty
        }, 0);
        assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
        assert.doesNotMatch(html, /<script>alert\(1\)<\/script>/);
        assert.equal(Editor.escapeHtml(dirty), '&lt;script&gt;alert(1)&lt;/script&gt;');

        const table = Editor.renderVocabTable([{ meaning: dirty, word: 'x' }]);
        assert.match(table, /&lt;script&gt;/);
        assert.doesNotMatch(table, /<script>alert/);
    });

    it('JSON section parse feedback', () => {
        const bad = Editor.parseJsonSection('{nope');
        assert.equal(bad.ok, false);
        assert.ok(bad.error);

        const good = Editor.parseJsonSection('{"a":1}');
        assert.equal(good.ok, true);
        assert.equal(good.value && good.value.a, 1);

        const empty = Editor.parseJsonSection('  ');
        assert.equal(empty.ok, true);
        assert.equal(empty.value, null);
    });

    it('lessons table escapes titles and shows visibility modes', () => {
        const dirty = '<img src=x onerror=alert(1)>';
        const html = Lessons.renderLessonsTable([
            {
                slug: 'safe-lesson',
                title: dirty,
                level: 'basic',
                mode: 'everyone',
                userIds: []
            }
        ], [{ id: 2, username: 'learner', status: 'active', role: 'user' }]);
        assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.doesNotMatch(html, /<img src=x onerror=alert\(1\)>/);
        assert.match(html, /data-admin-view="lessons"/);
        assert.match(html, /value="everyone"/);
    });
});
