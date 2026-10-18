/**
 * Brand icons, logo and favicon wired into the shell and views (INIT-005/SPEC-010).
 * Static and jsdom-free: loads the browser scripts in a vm and reads the files.
 * Never touches the network. Provenance: INIT-005/SPEC-010
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');
const read = (...p) => fs.readFileSync(path.join(root, ...p), 'utf8');

function walk(dir, out) {
    for (const name of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, name.name);
        if (name.isDirectory()) walk(full, out);
        else if (name.name.endsWith('.js')) out.push(full);
    }
    return out;
}

function pngSize(file) {
    const b = fs.readFileSync(file);
    return { w: b.readUInt32BE(16), h: b.readUInt32BE(20) };
}

const spriteText = read('shared', 'brand', 'sprite.svg');
const spriteIds = new Set([...spriteText.matchAll(/<symbol id="icon-([a-z-]+)"/g)].map((m) => m[1]));

function loadScripts() {
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        fetch: () => Promise.reject(new Error('no network in tests')),
        location: { pathname: '/', search: '', href: 'http://127.0.0.1/' },
        document: {
            readyState: 'complete',
            documentElement: { classList: { add() {}, remove() {} } },
            body: { classList: { add() {}, remove() {} } },
            addEventListener() {},
            querySelector() { return null; }
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    for (const f of ['player/utils.js', 'api.js', 'app-shell.js', 'layout.js']) {
        vm.runInContext(read('shared', f), sandbox);
    }
    return sandbox;
}

const ME = { id: 1, username: 'a', display_name: 'A', role: 'admin', must_change_password: false };

describe('shell header and nav (ac-1, ac-2)', () => {
    let win;
    let html;
    before(() => {
        win = loadScripts();
        html = win.HebrewShell.renderHtml(ME, '/');
    });

    it('shows the chosen mark from the local brand folder, as a decorative image', () => {
        assert.match(html, /<img class="hebrew-logo-mark" src="\/shared\/brand\/mark-a\.svg"/);
        assert.match(html, /<img class="hebrew-logo-mark"[^>]*alt=""/);
        assert.match(html, /<span class="hebrew-brand-he ui-he" lang="he">דֶּרֶךְ עִבְרִית<\/span>/);
        assert.match(html, /<span class="hebrew-brand-en ui-en" lang="en" dir="ltr">Hebrew Way<\/span>/);
        assert.doesNotMatch(html, /<text|ע<\/text>/, 'the old text logo is gone');
    });

    it('draws nav icons from the sprite and every nav icon name exists in it', () => {
        const used = [...html.matchAll(/<use href="\/shared\/brand\/sprite\.svg#icon-([a-z-]+)">/g)].map((m) => m[1]);
        assert.deepEqual(used.sort(), ['admin', 'lessons', 'progress', 'review', 'theme', 'today', 'words']);
        for (const name of used) assert.ok(spriteIds.has(name), 'sprite has icon-' + name);
    });

    it('keeps the visible Hebrew word and English gloss on every nav item; icons are aria-hidden', () => {
        for (const item of win.HebrewShell.NAV) {
            const re = new RegExp('data-nav="' + item.id + '">(.*?)</a>');
            const link = html.match(re);
            assert.ok(link, item.id + ' link');
            assert.ok(link[1].includes(item.he), item.id + ' Hebrew word');
            assert.ok(link[1].includes('>' + item.en + '<'), item.id + ' English word');
            assert.ok(!link[1].includes('(' + item.en + ')'), item.id + ' is not a parenthetical gloss');
            // The link has no aria-label of its own: its visible words are its name (label in name).
            assert.doesNotMatch(link[0], /aria-label=/);
        }
        const svgs = html.match(/<svg\b[^>]*>/g) || [];
        assert.ok(svgs.length >= 6);
        for (const tag of svgs) {
            assert.match(tag, /aria-hidden="true"/);
            assert.match(tag, /focusable="false"/);
        }
        assert.match(html, /data-nav="guides">[^]*?<span class="ui-he" lang="he">מדריכים<\/span>/, 'Guides stays text only');
    });

    it('no label in the shell mixes two languages in one aria-label (REQ-016)', () => {
        for (const m of html.matchAll(/aria-label="([^"]*)"/g)) {
            const hasHe = /[\u0590-\u05FF]/.test(m[1]);
            const hasLatin = /[A-Za-z]/.test(m[1]);
            assert.ok(!(hasHe && hasLatin), 'mixed-language aria-label: ' + m[1]);
        }
    });

    it('page headings use the sprite too (layout.js)', () => {
        const page = win.HebrewLayout.page({
            id: 'review', rootId: 'review-app', heading: { he: 'חזרה', en: 'Review' }
        });
        assert.match(page, /<use href="\/shared\/brand\/sprite\.svg#icon-review">/);
        assert.match(page, /page-heading-icon-wrap" aria-hidden="true"/);
        const guides = win.HebrewLayout.page({ id: 'guides', rootId: 'guides-app', heading: { he: 'מדריכים', en: 'Guides' } });
        assert.doesNotMatch(guides, /<use /);
    });
});

describe('HebrewUtils sprite helper', () => {
    let U;
    before(() => { U = loadScripts().HebrewUtils; });

    it('sprite() is decorative, same-origin, and sanitises the name', () => {
        const svg = U.sprite('play');
        assert.match(svg, /^<svg class="icon" aria-hidden="true" focusable="false">/);
        assert.match(svg, /<use href="\/shared\/brand\/sprite\.svg#icon-play"><\/use>/);
        const hostile = U.sprite('x"><script>alert(1)</script>');
        assert.doesNotMatch(hostile, /<script/);
        assert.equal((hostile.match(/"/g) || []).length, 8, 'no extra quote got through');
        assert.match(U.sprite('next', '', { directional: true }), /class="icon icon-directional"/);
    });

    it('icon() maps existing names onto the sprite and keeps inline paths for the rest', () => {
        for (const [old, name] of [['volume-high', 'sound'], ['play', 'play'], ['check', 'check'],
            ['house', 'home'], ['rotate-right', 'retry'], ['list', 'lessons']]) {
            assert.ok(spriteIds.has(name));
            assert.match(U.icon(old), new RegExp('icon-' + name + '"'));
            assert.match(U.icon('fa-' + old, 'tone-icon'), /class="icon tone-icon"/);
        }
        assert.match(U.icon('microphone'), /<path d=/, 'no sprite icon for the microphone yet');
        assert.doesNotMatch(U.icon('microphone'), /<use/);
    });
});

describe('every sprite reference points at a real icon (ac-1)', () => {
    it('all #icon-NAME references in shared/, login/ and docs-linked HTML exist in sprite.svg', () => {
        const files = walk(path.join(root, 'shared'), []);
        files.push(path.join(root, 'login', 'index.html'));
        let count = 0;
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            for (const m of text.matchAll(/sprite\.svg#icon-([a-z-]+)/g)) {
                count += 1;
                assert.ok(spriteIds.has(m[1]), path.relative(root, file) + ' uses missing icon-' + m[1]);
            }
        }
        assert.ok(count >= 1);
    });

    it('icon names built in code (not literal references) exist in sprite.svg', () => {
        const layout = read('shared', 'layout.js');
        const routeNames = Object.keys(JSON.parse(
            layout.match(/var ROUTE_ICONS = (\{[^}]*\});/)[1].replace(/(\w+):/g, '"$1":')
        ));
        const shell = read('shared', 'app-shell.js');
        const shellNames = JSON.parse(shell.match(/var ICON_NAMES = (\[[^\]]*\]);/)[1].replace(/'/g, '"'));
        const utils = read('shared', 'player', 'utils.js');
        const mapped = [...utils.match(/const SPRITE_NAMES = \{([^}]*)\}/)[1].matchAll(/:\s*'([a-z-]+)'/g)].map((m) => m[1]);
        const direct = [
            ...read('shared', 'player', 'feedback.js').matchAll(/spriteIcon\('([a-z-]+)'\)/g),
            ...read('shared', 'player', 'core.js').matchAll(/U\.sprite\('([a-z-]+)'\)/g),
            ...read('shared', 'today-page.js').match(/var name = \(([^)]*)\)/)[1].matchAll(/kind === '([a-z]+)'/g)
        ].map((m) => m[1]);
        const all = new Set([...routeNames, ...shellNames, ...mapped, ...direct]);
        assert.ok(all.size >= 10, [...all].join(','));
        for (const name of all) assert.ok(spriteIds.has(name), 'sprite has icon-' + name);
        // Every nav id with an icon in NAV is in the shell list.
        for (const m of shell.matchAll(/icon: '([a-z-]+)'/g)) assert.ok(shellNames.includes(m[1]), m[1]);
    });

    it('no external icon host, CDN or icon font is referenced by the app (und-1)', () => {
        const files = walk(path.join(root, 'shared'), []);
        files.push(path.join(root, 'login', 'index.html'), path.join(root, 'shared', 'app.html'));
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            assert.doesNotMatch(text, /(?:href|src)="https?:\/\/[^"]*(?:icon|font|cdn)/i, path.relative(root, file));
            assert.doesNotMatch(text, /fontawesome|fonts\.googleapis|cdnjs|unpkg|jsdelivr/i, path.relative(root, file));
        }
    });
});

describe('icon-only controls have a Hebrew accessible name (ac-2)', () => {
    it('every <button> whose only child is an icon carries an aria-label, in one language', () => {
        const files = walk(path.join(root, 'shared'), []);
        let seen = 0;
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            const iconOnly = /<button\b[^>]*>\s*\$\{U\.icon\([^)]*\)\}\s*<\/button>/g;
            for (const m of text.matchAll(iconOnly)) {
                seen += 1;
                const open = m[0].slice(0, m[0].indexOf('>') + 1);
                assert.match(open, /aria-label="/, path.relative(root, file) + ': ' + open.slice(0, 90));
                const label = open.match(/aria-label="([^"]*)"/)[1];
                assert.doesNotMatch(label, /\$\{[^}]*\bbi\(/, 'bi() makes markup, not a label: ' + label);
                if (!/\$\{/.test(label)) {
                    assert.ok(!(/[\u0590-\u05FF]/.test(label) && /[A-Za-z]/.test(label)), 'mixed label: ' + label);
                }
            }
        }
        assert.ok(seen >= 3, 'found the icon-only buttons (speak, record, play recording, gloss, speak-built): ' + seen);
        assert.match(read('shared', 'admin', 'users.js'), /data-admin-speak="[\s\S]{0,200}aria-label="' \+ esc\('השמעה'\)/);
    });

    it('title and aria-label attributes never hold bilingual markup (it broke the attribute)', () => {
        for (const file of walk(path.join(root, 'shared'), [])) {
            const text = fs.readFileSync(file, 'utf8');
            assert.doesNotMatch(text, /(?:title|aria-label)="\$\{(?:ctx|U)\.bi\(/, path.relative(root, file));
            assert.doesNotMatch(text, /(?:title|aria-label)="\$\{ctx\.esc\((?:ctx|U)\.bi\(/, path.relative(root, file));
        }
    });

    it('bilingual markup is never assigned through textContent (it showed raw <span> text in the player)', () => {
        for (const file of walk(path.join(root, 'shared'), [])) {
            assert.doesNotMatch(fs.readFileSync(file, 'utf8'), /textContent\s*=\s*(?:U|ctx|utils)\.bi\(/, path.relative(root, file));
        }
    });

    it('the sound toggle shows an icon beside its visible word and keeps every hook', () => {
        const core = read('shared', 'player', 'core.js');
        const btn = core.match(/<button type="button" class="btn btn-sfx"[\s\S]*?<\/button>/)[0];
        assert.match(btn, /id="btn-sfx"/);
        assert.match(btn, /data-sfx-toggle/);
        assert.match(btn, /aria-pressed="false"/);
        assert.match(btn, /U\.sprite\('sound'\)/);
        assert.match(btn, /id="btn-sfx-label"[^>]*>\$\{U\.bi\('צלילים', 'Sounds'\)\}/);
    });

    it('speak buttons say השמעה, record says הקלטה (docs/design/icon-usage.md names)', () => {
        const core = read('shared', 'player', 'core.js');
        assert.match(core, /data-speak="[^"]*"[^`]*aria-label="השמעה"/);
        assert.match(core, /data-rec-toggle=[^`]*aria-label="הקלטה"/);
        assert.match(core, /data-rec-play=[^`]*aria-label="השמעת ההקלטה"/);
    });
});

describe('favicon, touch icon and manifest (ac-1)', () => {
    const PAGES = ['shared/app.html', 'login/index.html', 'shared/plan/index.html'];

    it('each full-page document links the svg and png favicon and the touch icon, all local and real', () => {
        for (const page of PAGES) {
            const text = read(...page.split('/'));
            const links = [...text.matchAll(/<link\b[^>]*>/g)].map((m) => m[0]);
            const hrefOf = (re) => {
                const tag = links.find((l) => re.test(l));
                assert.ok(tag, page + ' is missing ' + re);
                return tag.match(/href="([^"]+)"/)[1];
            };
            const hrefs = [
                hrefOf(/rel="icon"[^>]*image\/svg\+xml/),
                hrefOf(/rel="icon"[^>]*image\/png/),
                hrefOf(/rel="apple-touch-icon"/)
            ];
            for (const href of hrefs) {
                assert.match(href, /^\/shared\/brand\/[a-z0-9-]+\.(svg|png)$/, page + ' ' + href);
                assert.ok(fs.existsSync(path.join(root, href)), page + ' links a missing file ' + href);
            }
            assert.match(hrefOf(/rel="manifest"/), /^\/manifest\.webmanifest$/);
        }
        assert.doesNotMatch(read('shared', 'app.html'), /href="\/icon\.svg"/, 'app.html uses the brand set');
    });

    it('lesson pages are not edited by this spec (they get the links from the shell script)', () => {
        const dir = path.join(root, 'worksheets');
        for (const name of fs.readdirSync(dir, { withFileTypes: true })) {
            const index = path.join(dir, name.name, 'index.html');
            if (!name.isDirectory() || !fs.existsSync(index)) continue;
            assert.doesNotMatch(fs.readFileSync(index, 'utf8'), /shared\/brand/, name.name);
        }
        const shell = read('shared', 'app-shell.js');
        assert.match(shell, /function ensureBrandHead/);
        assert.match(shell, /favicon\.svg/);
        assert.match(shell, /apple-touch-icon\.png/);
    });

    it('ensureBrandHead adds missing links once and leaves existing ones alone', () => {
        const added = [];
        const have = new Set(['link[rel="icon"][type="image/png"]']);
        const head = {
            appendChild(el) { added.push(el); },
            querySelector(sel) { return have.has(sel) ? {} : null; }
        };
        const sandbox = {
            console, setTimeout, clearTimeout,
            location: { pathname: '/worksheets/x/', search: '', href: 'http://127.0.0.1/worksheets/x/' },
            document: {
                readyState: 'complete', head,
                documentElement: { classList: { add() {} } },
                body: { classList: { add() {} } },
                addEventListener() {}, querySelector() { return null; },
                createElement() {
                    const attrs = {};
                    return { attrs, setAttribute(k, v) { attrs[k] = v; } };
                }
            }
        };
        sandbox.window = sandbox;
        sandbox.globalThis = sandbox;
        vm.createContext(sandbox);
        vm.runInContext(read('shared', 'player', 'utils.js'), sandbox);
        vm.runInContext(read('shared', 'app-shell.js'), sandbox);
        const host = { innerHTML: '', classList: { add() {} }, querySelector() { return null; },
            querySelectorAll() { return []; }, addEventListener() {}, contains() { return false; } };
        return sandbox.HebrewShell.mount(host, { skipFetch: true, me: ME, pathname: '/worksheets/x/' }).then(() => {
            const rels = added.map((el) => el.attrs.rel + ':' + (el.attrs.type || ''));
            assert.deepEqual(rels.sort(), ['apple-touch-icon:', 'icon:image/svg+xml', 'manifest:']);
            for (const el of added) {
                if (el.attrs.href.startsWith('/shared/brand/')) {
                    assert.ok(fs.existsSync(path.join(root, el.attrs.href)), el.attrs.href);
                }
            }
        });
    });

    it('manifest icons are real local files of the declared size, with 192, 512 and a maskable', () => {
        const manifest = JSON.parse(read('manifest.webmanifest'));
        const purposes = new Set();
        const sizes = new Set();
        for (const icon of manifest.icons) {
            assert.match(icon.src, /^\/(shared\/brand\/[a-z0-9-]+\.(png|svg)|icon\.svg)$/, icon.src);
            const file = path.join(root, icon.src);
            assert.ok(fs.existsSync(file), icon.src);
            if (icon.type === 'image/png') {
                const { w, h } = pngSize(file);
                assert.equal(icon.sizes, w + 'x' + h, icon.src);
                sizes.add(icon.sizes);
            }
            icon.purpose.split(' ').forEach((p) => purposes.add(p));
        }
        assert.ok(sizes.has('192x192') && sizes.has('512x512'));
        assert.ok(purposes.has('maskable') && purposes.has('any'));
        assert.equal(manifest.theme_color, '#1d4ed8');
    });

    it('/icon.svg is the chosen mark and carries no script, handler, text or external reference (ac-3)', () => {
        const svg = read('icon.svg');
        assert.equal(svg, read('shared', 'brand', 'favicon.svg'));
        assert.doesNotMatch(svg, /<script|\son[a-z]+=|<foreignObject|<image|<text|<style/i);
        assert.deepEqual(svg.match(/https?:\/\/[^"'\s<)]+/g), ['http://www.w3.org/2000/svg']);
    });
});

describe('icon CSS block', () => {
    const css = read('shared', 'worksheet.css');

    it('lives in one delimited block at the end of worksheet.css and mirrors only next/back in RTL', () => {
        const start = css.indexOf('/* === INIT-005/SPEC-010 icons === */');
        const end = css.indexOf('/* === end INIT-005/SPEC-010 icons === */');
        assert.ok(start > 0 && end > start);
        assert.equal(css.indexOf('/* === INIT-005/SPEC-010 icons === */', start + 1), -1, 'one block');
        const block = css.slice(start, end);
        assert.match(block, /\[dir="rtl"\] \.icon-directional \{ transform: scaleX\(-1\); \}/);
        assert.match(block, /^\.icon \{/m);
        // Nothing in the block changes the page theme.
        assert.doesNotMatch(block, /prefers-color-scheme|data-theme|:root/);
    });

    it('stays light by default: the light-by-default block is still there', () => {
        assert.match(css, /\/\* === Light by default/);
        assert.match(css, /\/\* === end light by default/);
    });

    it('the next arrow on Today is mirrored (icon-directional) and play/review are not', () => {
        const today = read('shared', 'today-page.js');
        assert.match(today, /name === 'next' \? ' icon-directional' : ''/);
    });
});

describe('RSCH-008 feedback-kit follow-ups', () => {
    const css = read('shared', 'worksheet.css');

    it('the wrong-answer shake is gone but the class hook is kept', () => {
        assert.doesNotMatch(css, /match-wrong-shake/);
        assert.match(css, /\.word-match-card\.wrong \{[^}]*border-color: #e11d48;[^}]*outline: 2px solid #e11d48/);
    });

    it('the loading pulse is bounded to five seconds (WCAG 2.2.2)', () => {
        const rule = css.match(/\.icon-btn\.is-loading,\s*\[data-speak\]\.is-loading,\s*\[data-audio\]\.is-loading \{([^}]*)\}/);
        assert.ok(rule, 'loading rule');
        const m = rule[1].match(/animation:\s*hebrew-audio-pulse\s+([\d.]+)s\s+[a-z-]+\s+(\d+|infinite)\s*;/);
        assert.ok(m, 'animation shorthand: ' + rule[1]);
        assert.notEqual(m[2], 'infinite');
        assert.ok(Number(m[1]) * Number(m[2]) <= 5, 'at most 5 s in total');
    });

    it('feedback.js cites RSCH-008 and says suppress, not duck', () => {
        const src = read('shared', 'player', 'feedback.js');
        assert.match(src, /RSCH-008/);
        assert.doesNotMatch(src, /RSCH-007/);
        assert.doesNotMatch(src, /ducks? and mutes?/i);
        assert.match(src, /SUPPRESSING/);
        assert.doesNotMatch(src, /frequency\.value\s*=[^;]*\b220\b/, 'the 220 Hz miss tone is gone');
    });
});
