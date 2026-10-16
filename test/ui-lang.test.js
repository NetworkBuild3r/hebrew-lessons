/**
 * Interface language: one chrome language at a time, remembered in this browser.
 * Lesson text is not part of the switch.
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadUtils(storage, doc) {
    const sandbox = {
        console,
        localStorage: storage,
        document: doc,
        window: null
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'), sandbox);
    return sandbox.HebrewUtils;
}

describe('interface language', () => {
    it('prints both languages without parentheses, and leaves a one-language string alone', () => {
        const U = loadUtils({ getItem() { return null; }, setItem() {} }, { documentElement: {} });
        assert.equal(
            U.bi('המשך', 'Continue'),
            '<span class="ui-pair"><span class="ui-he" lang="he">המשך</span>'
                + '<span class="ui-en" lang="en" dir="ltr">Continue</span></span>'
        );
        assert.equal(U.bi('רק עברית', ''), 'רק עברית');
        assert.equal(U.bi('', 'Only English'), '<span class="ui-en" lang="en" dir="ltr">Only English</span>');
        assert.equal(U.bi('<b>', 'a&b'), '<span class="ui-pair"><span class="ui-he" lang="he">&lt;b&gt;</span>'
            + '<span class="ui-en" lang="en" dir="ltr">a&amp;b</span></span>');
    });

    it('defaults to English and remembers a choice', () => {
        const store = {
            value: null,
            getItem() { return this.value; },
            setItem(key, value) { this.value = value; this.key = key; }
        };
        const html = {
            attrs: {},
            setAttribute(name, value) { this.attrs[name] = value; },
            getAttribute(name) { return this.attrs[name]; }
        };
        const chrome = { dir: '', setAttribute(name, value) { if (name === 'dir') this.dir = value; } };
        const doc = {
            documentElement: html,
            title: '',
            querySelectorAll(sel) {
                if (sel === '.ui-chrome') return [chrome];
                return [];
            }
        };
        const U = loadUtils(store, doc);
        assert.equal(U.readUiLang(), 'en');
        assert.equal(html.attrs['data-ui-lang'], 'en');
        assert.equal(chrome.dir, 'ltr');
        U.setUiLang('he');
        assert.equal(store.value, 'he');
        assert.equal(store.key, 'hebrew-ui-lang');
        assert.equal(html.attrs['data-ui-lang'], 'he');
        assert.equal(chrome.dir, 'rtl');
        html.attrs['data-page-he'] = 'היום';
        html.attrs['data-page-en'] = 'Today';
        U.applyUiLang('en');
        assert.equal(doc.title, 'Today · Hebrew Way');
        U.applyUiLang('he');
        assert.equal(doc.title, 'היום · דרך עברית');
    });

    it('the stylesheet hides one language and keeps the lesson text', () => {
        const css = fs.readFileSync(path.join(root, 'shared', 'worksheet.css'), 'utf8');
        assert.match(css, /html:not\(\[data-ui-lang="he"\]\) \.ui-pair > \.ui-he \{ display: none; \}/);
        assert.match(css, /html\[data-ui-lang="he"\] \.ui-pair > \.ui-en \{ display: none; \}/);
        assert.doesNotMatch(css, /\.vocab-card \.meaning[^{]*display:\s*none/);
    });
});
