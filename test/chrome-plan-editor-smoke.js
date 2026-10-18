/**
 * Chrome smoke: the schema-driven plan editor in Admin > Plans (INIT-008/SPEC-011, SPEC-012).
 *
 * Drives headless Chrome over CDP against the signed-in test server (temp database; the real
 * one is never opened). A fixture plan holds one block of each of the eleven types. It checks:
 *   - the lesson form renders with a widget per schema property, recipe and visibility are selects
 *   - all eleven block types render a form with bilingual labels, and every Hebrew control is
 *     lang="he" dir="rtl", every English control lang="en" dir="ltr"
 *   - an edit in a text field of an introduction word, and in each of the five 1.1.0 blocks
 *     (explainer, conjugation-table, personal-question, verse-reading, letter-intro), is saved
 *     (the revision goes up by one) and is still there after a reload; the stored plan has it
 *   - the lesson recipe and visibility save
 *   - a new block is added through the new-block form (required fields filled in)
 *   - a stale revision (the AI wrote in between) is a 409: the typed text is kept, the code and the
 *     stored revision are shown, and Reload takes the server's copy
 *   - a validation error shows its rule code at the field and keeps the text
 *   - SPEC-013: the Plans tab lists plans with their status; a row opens its plan; New plan (bad id,
 *     an id that exists, a recipe from the schema) opens straight into the editor; the assign control
 *     offers active accounts only, saves personal and shared, and shows assignee_refused
 *   - SPEC-014: a plan edited by an AI (service layer) and by the admin (editor) shows both in History
 *     with the right actor marks; compare shows before and after; restore from a diff, from a row, and
 *     from the "edited by a person" mark each ask once and add one revision; a stale restore is a 409;
 *     nothing is disabled because of a mark
 *   - SPEC-015: a plan with a non-NFC word, a word that differs from a verified lexicon entry, and an
 *     edited sentence that kept its clip lists all three as warnings; a link jumps to the field; "Use
 *     this value" is one write; the preview changes before any save with no request and shows typed
 *     markup as text; Publish shows the counts; the learner route is still 404 for a draft
 *   - SPEC-016: the AI-first loop in the browser. A plan the AI made with create_plan and the MCP tool
 *     handlers and published shows in the list; a person fixes two fields in the editor; read_part
 *     shows both as editState; History lists the AI and the person; the Issues panel lists the left-over
 *     warning and the stale clip; a mark shows on each person-written field; restoring the published
 *     meaning from the published-to-working diff makes one new revision; the next write by the AI lists
 *     overwrote and is not refused; the learner route still serves the published revision
 *   - a learner gets none of the plan markup and 403 from the plan routes
 *   - at 360 and 1280 px: no horizontal overflow, every target >= 24 px, primary buttons >= 44 px,
 *     every control has an accessible name, text contrast >= 4.5:1 (editor, plan list, history, issues)
 *   - every request the browser makes is same-origin, and the page throws nothing
 *
 * Run: node test/chrome-plan-editor-smoke.js     (exit 0 = pass, 1 = fail)
 * Needs Chrome at the default Windows path (or CHROME_PATH).
 * Provenance: INIT-008/SPEC-011, INIT-008/SPEC-012, INIT-008/SPEC-013, INIT-008/SPEC-014, INIT-008/SPEC-015, INIT-008/SPEC-016
 */
'use strict';

const harness = require('./helpers/signed-in-server');
const chromeCdp = require('./helpers/chrome-cdp');
const { samplePlan } = require('./helpers/plan-editor-fixtures');
const { seedPanels } = require('./helpers/plan-panels-seed');

const { sleep, waitFor } = chromeCdp;

const PLAN_ID = 'editor-smoke';
const LESSON = 'units/u1/lessons/l1';

const failures = [];
const report = { edits: {}, audit: {}, requests: 0 };
function fail(where, msg) { failures.push(where + ': ' + msg); }

/** Runs inside the page. Plain data only. */
function pageAudit() {
    const out = { overflow: null, small: [], big: [], unnamed: [], contrast: [], texts: 0 };
    const panel = document.getElementById('admin-panel') || document.body;
    const isVisible = (el) => {
        const r = el.getBoundingClientRect();
        if (r.width === 0 || r.height === 0) return false;
        const cs = getComputedStyle(el);
        return cs.visibility !== 'hidden' && cs.display !== 'none' && !el.closest('[hidden]');
    };
    const label = (el) => el.tagName.toLowerCase()
        + (el.getAttribute('data-plan-field') ? '[field=' + el.getAttribute('data-plan-field') + ']' : '')
        + (el.className && typeof el.className === 'string' ? '.' + el.className.trim().split(/\s+/).slice(0, 2).join('.') : '');
    const textOf = (node) => {
        if (node.nodeType === 3) return node.nodeValue;
        if (node.nodeType !== 1 || node.getAttribute('aria-hidden') === 'true') return '';
        if (getComputedStyle(node).display === 'none') return '';
        return Array.from(node.childNodes).map(textOf).join(' ');
    };
    const nameOf = (el) => {
        const by = el.getAttribute('aria-labelledby');
        if (by) {
            const n = by.split(/\s+/).map((id) => { const e = document.getElementById(id); return e ? textOf(e) : ''; }).join(' ').trim();
            if (n) return n;
        }
        const al = (el.getAttribute('aria-label') || '').trim();
        if (al) return al;
        if (el.labels && el.labels.length) {
            const n = Array.from(el.labels).map(textOf).join(' ').trim();
            if (n) return n;
        }
        const tag = el.tagName.toLowerCase();
        if (tag === 'input' || tag === 'select' || tag === 'textarea') return (el.getAttribute('title') || '').trim();
        return Array.from(el.childNodes).map(textOf).join(' ').replace(/\s+/g, ' ').trim();
    };
    const INTERACTIVE = 'a[href], button, input:not([type=hidden]), select, textarea, summary';
    const PRIMARY = '.btn:not(.btn-ghost):not(.btn-small):not(.admin-row-btn)';
    const targets = [];
    for (const el of panel.querySelectorAll(INTERACTIVE)) {
        if (!isVisible(el) || el.disabled) continue;
        const type = (el.getAttribute('type') || '').toLowerCase();
        let r = el.getBoundingClientRect();
        if ((type === 'checkbox' || type === 'radio') && el.labels && el.labels[0]) {
            const lr = el.labels[0].getBoundingClientRect();
            if (lr.width * lr.height > r.width * r.height) r = lr;
        }
        const inline = el.tagName === 'A' && getComputedStyle(el).display === 'inline'
            && el.parentElement && el.parentElement.textContent.trim().length > el.textContent.trim().length + 8;
        targets.push({ el, r, inline });
        if (el.matches(PRIMARY) && (r.height < 43.5 || r.width < 43.5)) out.big.push(label(el) + ' ' + Math.round(r.width) + 'x' + Math.round(r.height));
        if (!nameOf(el)) out.unnamed.push(label(el));
    }
    for (const t of targets) {
        if (t.inline || (t.r.width >= 23.5 && t.r.height >= 23.5)) continue;
        out.small.push(label(t.el) + ' ' + Math.round(t.r.width) + 'x' + Math.round(t.r.height));
    }
    out.overflow = { scrollWidth: document.documentElement.scrollWidth, innerWidth: window.innerWidth };

    const parse = (c) => {
        const m = String(c).match(/rgba?\(([^)]+)\)/);
        if (!m) return null;
        const p = m[1].split(/[ ,\/]+/).filter(Boolean).map(Number);
        return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 };
    };
    const over = (top, under) => ({
        r: top.r * top.a + under.r * (1 - top.a), g: top.g * top.a + under.g * (1 - top.a), b: top.b * top.a + under.b * (1 - top.a), a: 1
    });
    const lum = (c) => {
        const f = (v) => { const s = v / 255; return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4); };
        return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b);
    };
    const ratio = (a, b) => { const hi = Math.max(lum(a), lum(b)); const lo = Math.min(lum(a), lum(b)); return (hi + 0.05) / (lo + 0.05); };
    const backdrop = (el) => {
        const layers = [];
        let gradient = false;
        for (let n = el; n; n = n.parentElement) {
            const cs = getComputedStyle(n);
            if (cs.backgroundImage && cs.backgroundImage !== 'none') gradient = true;
            const bg = parse(cs.backgroundColor);
            if (bg && bg.a > 0) { layers.push(bg); if (bg.a === 1) break; }
        }
        let base = { r: 255, g: 255, b: 255, a: 1 };
        for (let i = layers.length - 1; i >= 0; i -= 1) base = over(layers[i], base);
        return { color: base, gradient };
    };
    const seen = new Set();
    const walker = document.createTreeWalker(panel, NodeFilter.SHOW_TEXT);
    for (let t = walker.nextNode(); t; t = walker.nextNode()) {
        if (!t.nodeValue.trim()) continue;
        const el = t.parentElement;
        if (!el || seen.has(el) || !isVisible(el) || el.closest('option, script, style, :disabled')) continue;
        seen.add(el);
        out.texts += 1;
        const b = backdrop(el);
        if (b.gradient) continue;
        const cs = getComputedStyle(el);
        const fg0 = parse(cs.color);
        if (!fg0) continue;
        const fg = over(fg0, b.color);
        const size = parseFloat(cs.fontSize);
        const large = size >= 24 || (size >= 18.66 && Number(cs.fontWeight) >= 700);
        const need = large ? 3 : 4.5;
        const got = ratio(fg, b.color);
        if (got + 0.005 < need) out.contrast.push(label(el) + ' "' + t.nodeValue.trim().slice(0, 24) + '" ' + got.toFixed(2) + ' < ' + need);
    }
    return out;
}

const q = (sel) => 'document.querySelector(' + JSON.stringify(sel) + ')';

async function main() {
    const started = await harness.start({ lessons: 'fixture' });
    const BASE = started.baseUrl;
    const store = require('../server/plans/store');
    const ops = require('../server/plans/ops');
    const { makeContext } = require('../server/plans/context');
    const aiCtx = makeContext({ channel: 'mcp', tool: 'smoke' });
    // published and edited plans, a plan with three kinds of warning, and a disabled account (SPEC-013 to SPEC-015)
    seedPanels();

    const plan = samplePlan();
    plan.id = PLAN_ID;
    plan.title = 'Editor smoke';
    plan.revision = 1;
    store.saveWorkingCopy({
        id: plan.id, document: JSON.stringify(plan), revision: 1, status: 'draft', audience: 'shared', assigneeId: null
    });
    const stored = () => JSON.parse(store.loadWorkingCopy(PLAN_ID).document);
    const storedRevision = () => store.loadWorkingCopy(PLAN_ID).revision;
    const blockOf = (doc, id) => doc.units[0].lessons[0].blocks.find((b) => b.id === id);

    const browser = await chromeCdp.launch({ tag: 'plan-editor', portBase: 9900, width: 1280, height: 900 });
    const cdp = browser.cdp;
    const sameOrigin = (u) => !/^(https?|wss?|ftp):/i.test(u) || u.startsWith(BASE + '/') || u === BASE;

    const open = async () => {
        await cdp.call('Page.navigate', { url: BASE + '/admin/#plan/' + PLAN_ID });
        await waitFor(cdp, `!!${q('[data-plan-section="lesson"] [data-plan-field="title"]')}`, 'the lesson form', 20000);
        await sleep(300);
    };
    const reload = async () => {
        await cdp.call('Page.reload');
        await waitFor(cdp, `!!${q('[data-plan-section="lesson"] [data-plan-field="title"]')}`, 'the lesson form after reload', 20000);
        await sleep(300);
    };
    const nodeSel = (path) => '[data-plan-node="' + path + '"]';
    const blockPath = (id) => LESSON + '/blocks/' + id;
    const edit = async (id) => {
        await cdp.evaluate(`${q('[data-plan-block="' + id + '"] [data-plan-edit]')}.click(); true`);
        await waitFor(cdp, `!!${q('[data-plan-fields] ' + nodeSel(blockPath(id)))}`, 'form of ' + id);
    };
    /** Select a control's text and type over it, as a person would. */
    const typeInto = async (selector, text) => {
        await cdp.evaluate(`(() => { const el = ${q(selector)}; el.scrollIntoView({ block: 'center' }); el.focus(); el.select(); return true; })()`);
        await cdp.call('Input.insertText', { text });
        await sleep(60);
    };
    const click = async (selector) => cdp.evaluate(`(() => { const el = ${q(selector)}; el.scrollIntoView({ block: 'center' }); el.click(); return true; })()`);
    const revisionText = () => cdp.evaluate(`${q('[data-plan-revision]')}.textContent`);
    const waitRevision = async (n, label) => waitFor(cdp, `${q('[data-plan-revision]')} && ${q('[data-plan-revision]')}.textContent === '${n}'`, label || ('revision ' + n));

    try {
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');
        await chromeCdp.setViewport(cdp, 1280, 900);
        await open();

        // ---- the lesson form: a widget per schema property; recipe and visibility are selects
        const lessonForm = await cdp.evaluate(`(() => {
            const form = ${q('[data-plan-section="lesson"] ' + nodeSel(LESSON))};
            const own = Array.from(form.querySelectorAll('[data-plan-field]')).filter((n) => n.closest('[data-plan-node]') === form);
            return own.map((n) => n.getAttribute('data-plan-field') + ':' + n.getAttribute('data-plan-widget'));
        })()`);
        report.lessonFields = lessonForm;
        const want = ['id', 'title', 'subtitle', 'footer', 'level', 'visibility', 'recipe', 'legacyStorageKey', 'buildsOn', 'newWords', 'recycles'];
        for (const f of want) if (!lessonForm.some((s) => s.indexOf(f + ':') === 0)) fail('lesson form', 'no control for ' + f);
        if (lessonForm.indexOf('recipe:select') === -1 || lessonForm.indexOf('visibility:select') === -1) fail('lesson form', 'recipe or visibility is not a select');
        if (lessonForm.some((s) => /:undefined|:null$/.test(s))) fail('lesson form', 'a control without a widget');

        // ---- every block type renders a form with bilingual labels and language attributes
        const types = ['introduction', 'graded-story', 'listening', 'short-dialogue', 'fluency-pass', 'spaced-review-checkpoint',
            'explainer', 'conjugation-table', 'personal-question', 'verse-reading', 'letter-intro'];
        report.blocks = {};
        for (const id of types) {
            await edit(id);
            const r = await cdp.evaluate(`(() => {
                const form = ${q('[data-plan-fields] ' + nodeSel(blockPath(id)))};
                const own = Array.from(form.querySelectorAll('[data-plan-field]')).filter((n) => n.closest('[data-plan-node]') === form);
                const labels = Array.from(form.querySelectorAll('[data-plan-row] > label, [data-plan-row] > span[id]')).filter((l) => l.closest('[data-plan-node]') === form);
                const he = Array.from(form.querySelectorAll('[lang="he"]:is(input, textarea)'));
                const en = Array.from(form.querySelectorAll('[lang="en"]:is(input, textarea)'));
                return {
                    controls: own.length,
                    noWidget: own.filter((n) => !n.getAttribute('data-plan-widget')).length,
                    bilingual: labels.filter((l) => /[\\u0590-\\u05FF]/.test(l.querySelector('[lang=he]') ? l.querySelector('[lang=he]').textContent : '') && l.querySelector('[lang=en]')).length,
                    labels: labels.length,
                    heWrong: he.filter((n) => n.getAttribute('dir') !== 'rtl').length,
                    enWrong: en.filter((n) => n.getAttribute('dir') !== 'ltr').length,
                    he: he.length, en: en.length
                };
            })()`);
            report.blocks[id] = r;
            if (!r.controls || r.noWidget) fail('block ' + id, JSON.stringify(r));
            if (r.bilingual !== r.labels) fail('block ' + id, 'a label is not Hebrew over English: ' + JSON.stringify(r));
            if (r.heWrong || r.enWrong) fail('block ' + id, 'lang/dir mismatch ' + JSON.stringify(r));
        }

        // ---- edit one text field in a word and in each 1.1.0 block; each save bumps the revision once
        const edits = [
            { id: 'introduction', sel: nodeSel(blockPath('introduction') + '/words/1') + ' > [data-plan-row="meaning"] [data-plan-field="meaning"]',
                save: nodeSel(blockPath('introduction') + '/words/1') + ' > [data-plan-row="meaning"] [data-plan-save]',
                text: 'a home, edited', check: (d) => blockOf(d, 'introduction').words[0].meaning === 'a home, edited' },
            { id: 'explainer', sel: nodeSel(blockPath('explainer')) + ' > [data-plan-row="title"] [data-plan-field="title"]',
                save: nodeSel(blockPath('explainer')) + ' > [data-plan-row="title"] [data-plan-save]',
                text: 'A page, edited', check: (d) => blockOf(d, 'explainer').title === 'A page, edited' },
            { id: 'conjugation-table', sel: nodeSel(blockPath('conjugation-table')) + ' > [data-plan-row="mode"] [data-plan-field="mode"]',
                save: nodeSel(blockPath('conjugation-table')) + ' > [data-plan-row="mode"] [data-plan-save]',
                select: 'drill', check: (d) => blockOf(d, 'conjugation-table').mode === 'drill' },
            { id: 'personal-question', sel: nodeSel(blockPath('personal-question')) + ' > [data-plan-row="prompts"] [data-plan-sub="en"]',
                save: nodeSel(blockPath('personal-question')) + ' > [data-plan-row="prompts"] [data-plan-save]',
                text: 'What is your name, edited?', check: (d) => blockOf(d, 'personal-question').prompts[0].en === 'What is your name, edited?' },
            { id: 'verse-reading', sel: nodeSel(blockPath('verse-reading')) + ' > [data-plan-row="ref"] [data-plan-sub="book"]',
                save: nodeSel(blockPath('verse-reading')) + ' > [data-plan-row="ref"] [data-plan-save]',
                text: 'Genesis, edited', check: (d) => blockOf(d, 'verse-reading').ref.book === 'Genesis, edited' && blockOf(d, 'verse-reading').ref.chapter === 1 },
            { id: 'letter-intro', sel: nodeSel(blockPath('letter-intro')) + ' > [data-plan-row="name"] [data-plan-field="name"]',
                save: nodeSel(blockPath('letter-intro')) + ' > [data-plan-row="name"] [data-plan-save]',
                text: 'Bet, edited', check: (d) => blockOf(d, 'letter-intro').name === 'Bet, edited' }
        ];
        let rev = storedRevision();
        for (const e of edits) {
            await edit(e.id);
            if (e.select) {
                await cdp.evaluate(`(() => { const el = ${q(e.sel)}; el.value = ${JSON.stringify(e.select)}; el.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`);
            } else {
                await typeInto(e.sel, e.text);
            }
            const dirty = await cdp.evaluate(`!!${q(e.save)}.closest('[data-plan-row]').getAttribute('data-plan-dirty')`);
            if (!dirty) fail('edit ' + e.id, 'the field row is not marked unsaved after typing');
            await click(e.save);
            rev += 1;
            await waitRevision(rev, 'revision after saving ' + e.id);
            const doc = stored();
            if (!e.check(doc)) fail('edit ' + e.id, 'the stored plan does not hold the edit');
            if (storedRevision() !== rev) fail('edit ' + e.id, 'revision is ' + storedRevision() + ', wanted ' + rev);
            report.edits[e.id] = 'saved, revision ' + rev;
        }
        // the edits survive a reload of the page
        await reload();
        for (const e of edits) {
            await edit(e.id);
            const shown = await cdp.evaluate(`${q(e.sel)}.value`);
            const wanted = e.select || e.text;
            if (shown !== wanted) fail('reload ' + e.id, 'form shows "' + shown + '", wanted "' + wanted + '"');
        }

        // ---- the lesson recipe and visibility
        const lessonSel = (prop) => '[data-plan-section="lesson"] ' + nodeSel(LESSON) + ' > [data-plan-row="' + prop + '"]';
        for (const [prop, value] of [['recipe', 'verb-workshop'], ['visibility', 'selected']]) {
            await cdp.evaluate(`(() => { const el = ${q(lessonSel(prop) + ' [data-plan-field]')}; el.value = ${JSON.stringify(value)}; el.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`);
            await click(lessonSel(prop) + ' [data-plan-save]');
            rev += 1;
            await waitRevision(rev, 'revision after ' + prop);
            const lesson = stored().units[0].lessons[0];
            if (lesson[prop] !== value) fail('lesson ' + prop, 'stored ' + lesson[prop]);
        }
        report.edits.lesson = 'recipe + visibility saved';

        // ---- the new-block form: a listening block with its required passage
        await cdp.evaluate(`(() => {
            const sel = ${q('[data-plan-new-type]')};
            sel.value = 'listening'; sel.dispatchEvent(new Event('change', { bubbles: true }));
            return true;
        })()`);
        await typeInto('[data-plan-new-draft] [data-plan-sub="passage"]', 'משפט חדש');
        await click('[data-plan-insert]');
        rev += 1;
        await waitRevision(rev, 'revision after inserting a block');
        const added = stored().units[0].lessons[0].blocks.find((b) => b.type === 'listening' && b.passage === 'משפט חדש');
        if (!added) fail('new block', JSON.stringify(stored().units[0].lessons[0].blocks));
        const listenId = added.id;
        report.edits.newBlock = 'listening block inserted';

        // ---- a stale revision: the AI writes in between; the page's save is a 409
        await reload();
        await edit('listening');
        const staleSel = nodeSel(blockPath('listening')) + ' > [data-plan-row="passage"] [data-plan-field="passage"]';
        const staleSave = nodeSel(blockPath('listening')) + ' > [data-plan-row="passage"] [data-plan-save]';
        const before = storedRevision();
        const bumped = ops.setField(store, PLAN_ID, { path: LESSON, field: 'footer', value: 'the AI was here', expectedRevision: before }, aiCtx);
        if (bumped.error) throw new Error('could not bump the revision: ' + JSON.stringify(bumped));
        await typeInto(staleSel, 'my edit that is now stale');
        await click(staleSave);
        await waitFor(cdp, `!${q('[data-plan-conflict]')}.hidden`, 'the conflict message');
        const conflict = await cdp.evaluate(`({
            box: ${q('[data-plan-conflict]')}.textContent,
            inline: ${q(staleSel)}.closest('[data-plan-row]').querySelector('[data-plan-issues]').textContent,
            kept: ${q(staleSel)}.value,
            shownRevision: ${q('[data-plan-revision]')}.textContent
        })`);
        report.conflict = conflict;
        if (!/revision_conflict/.test(conflict.box) || conflict.box.indexOf(String(before + 1)) === -1) fail('409', 'conflict box: ' + conflict.box);
        if (!/revision_conflict/.test(conflict.inline)) fail('409', 'no code at the field: ' + conflict.inline);
        if (conflict.kept !== 'my edit that is now stale') fail('409', 'typed text was lost: ' + conflict.kept);
        if (blockOf(stored(), 'listening').passage === 'my edit that is now stale') fail('409', 'a stale write was stored');
        // Reload takes the server's copy
        await click('[data-plan-reload]');
        await waitFor(cdp, `${q('[data-plan-revision]')}.textContent === '${before + 1}'`, 'revision after Reload');
        await edit('listening');
        const afterReload = await cdp.evaluate(`${q(staleSel)}.value`);
        if (afterReload === 'my edit that is now stale') fail('409', 'Reload kept the stale text');
        // and now the save goes through
        await typeInto(staleSel, 'my edit after reload');
        await click(staleSave);
        await waitRevision(before + 2, 'revision after the retried save');
        if (blockOf(stored(), 'listening').passage !== 'my edit after reload') fail('409', 'the retried save was not stored');
        report.edits.conflict = '409 kept the text; reload; retry saved';

        // ---- a validation error shows its rule at the field and keeps the text
        await edit('listening');
        await typeInto(staleSel, '');
        await click(staleSave);
        await waitFor(cdp, `${q(staleSel)}.closest('[data-plan-row]').querySelectorAll('[data-plan-issue]').length > 0`, 'inline validation');
        const invalid = await cdp.evaluate(`({
            rules: Array.from(${q(staleSel)}.closest('[data-plan-row]').querySelectorAll('[data-plan-issue]')).map((n) => n.getAttribute('data-plan-issue')),
            invalid: ${q(staleSel)}.getAttribute('aria-invalid'),
            value: ${q(staleSel)}.value
        })`);
        report.validation = invalid;
        if (invalid.rules.indexOf('schema') === -1) fail('validation', 'no schema rule at the field: ' + JSON.stringify(invalid));
        if (invalid.value !== '') fail('validation', 'text changed');
        if (storedRevision() !== before + 2) fail('validation', 'a refused write bumped the revision');
        await typeInto(staleSel, 'restored text');
        await click(staleSave);
        await waitRevision(before + 3, 'revision after fixing the field');

        // ====================================================================================
        // INIT-008/SPEC-013 to SPEC-015: the Plans tab, history, issues, and the live preview
        // ====================================================================================
        const nav = async (hash, readySel, label) => {
            await cdp.call('Page.navigate', { url: 'about:blank' });
            await cdp.call('Page.navigate', { url: BASE + '/admin/' + hash });
            await waitFor(cdp, `!!${q(readySel)}`, label, 20000);
            await sleep(300);
        };
        const textOf = (sel) => cdp.evaluate(`(${q(sel)} || {}).textContent || ''`);
        const countOf = (sel) => cdp.evaluate(`document.querySelectorAll(${JSON.stringify(sel)}).length`);
        const setValue = (sel, value) => cdp.evaluate(`(() => { const el = ${q(sel)}; el.value = ${JSON.stringify(value)}; el.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`);
        const LESSON_READY = '[data-plan-section="lesson"] [data-plan-field="title"]';
        const planOf = (id) => JSON.parse(store.loadWorkingCopy(id).document);
        const revisionOf = (id) => store.loadWorkingCopy(id).revision;
        const requestsBefore = () => cdp.requests.length;

        // ---- SPEC-013: the list shows every plan with its status
        await nav('#plans', '[data-plan-list] [data-plan-row]', 'the plan list');
        const listed = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-list] [data-plan-row]')).map((r) => r.getAttribute('data-plan-row') + ':' + r.getAttribute('data-plan-row-status') + ':' + !!r.querySelector('[data-plan-unpublished]'))`);
        report.list = listed;
        for (const want of ['editor-smoke:draft:false', 'published-plan:published:false', 'edited-plan:draft:true']) {
            if (listed.indexOf(want) === -1) fail('plan list', 'no row ' + want + ' in ' + listed.join(', '));
        }
        const links = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-open-link]')).map((a) => a.getAttribute('href'))`);
        if (links.indexOf('#plan/editor-smoke') === -1) fail('plan list', 'no link to a plan: ' + links.join(', '));
        const humanCount = await cdp.evaluate(`(${q('[data-plan-row="editor-smoke"] [data-plan-human-fields]')} || {}).textContent`);
        if (!/^\d+$/.test(String(humanCount))) fail('plan list', 'no edited-by-a-person count: ' + humanCount);
        // a row opens its plan
        await click('[data-plan-row="published-plan"] [data-plan-open-link]');
        await waitFor(cdp, `!!${q(LESSON_READY)}`, 'the editor of a listed plan', 20000);
        if (await cdp.evaluate('location.hash') !== '#plan/published-plan') fail('plan list', 'the link did not open the plan');

        // ---- SPEC-013: New plan
        await nav('#plans', '[data-plan-new-open]', 'the plans tab');
        await click('[data-plan-new-open]');
        await waitFor(cdp, `!!${q('[data-plan-new]')}`, 'the New plan form');
        const newPlanText = await cdp.evaluate(`(${q('[data-plan-new]')} || {}).innerText || ''`);
        if (/\bId\b|מזהה/.test(newPlanText)) fail('new plan', 'the form still asks for an id');
        // an empty title is refused in the browser and stores nothing
        const idsBefore = store.listPlans(50).map((row) => row.id);
        await click('[data-plan-create]');
        await waitFor(cdp, `!${q('[data-plan-new-title-message]')}.hidden`, 'the title message');
        if (!/empty_title/.test(await textOf('[data-plan-new-title-message]'))) fail('new plan', 'no empty_title message');
        const idsAfterEmpty = store.listPlans(50).map((row) => row.id);
        if (idsAfterEmpty.length !== idsBefore.length) fail('new plan', 'an empty title was stored');
        // create for real, with a recipe: the server assigns a UUID and the editor opens on it
        await typeInto('[data-plan-new-title]', 'From the UI');
        await click('[data-plan-create]');
        await waitFor(cdp, `!!${q(LESSON_READY)}`, 'the editor of the new plan', 20000);
        const bornHash = await cdp.evaluate('location.hash');
        const bornId = decodeURIComponent(String(bornHash).replace(/^#plan\//, ''));
        if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(bornId)) fail('new plan', 'the new plan id is not a uuid: ' + bornHash);
        await setValue('[data-plan-section="lesson"] [data-plan-field="recipe"]', 'verb-workshop');
        await click('[data-plan-section="lesson"] [data-plan-row="recipe"] [data-plan-save]');
        await waitRevision(2, 'revision after setting recipe on the new plan');
        const born = planOf(bornId);
        if (born.units[0].lessons[0].recipe !== 'verb-workshop' || born.title !== 'From the UI' || born.id !== bornId || revisionOf(bornId) !== 2) fail('new plan', 'stored plan: ' + JSON.stringify(born).slice(0, 200));
        // edit it through the editor
        await typeInto('[data-plan-section="lesson"] [data-plan-row="title"] [data-plan-field="title"]', 'From the UI, edited');
        await click('[data-plan-section="lesson"] [data-plan-row="title"] [data-plan-save]');
        await waitRevision(3, 'revision after editing the new plan');
        if (planOf(bornId).units[0].lessons[0].title !== 'From the UI, edited') fail('new plan', 'the edit was not stored');
        await click('[data-plan-history-toggle]');
        await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length >= 2`, 'history of the new plan');
        const bornRows = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-history] [data-plan-revision]')).map((r) => r.getAttribute('data-plan-actor') + ':' + r.querySelector('[data-plan-op]').textContent)`);
        report.newPlanHistory = bornRows;
        if (bornRows.indexOf('human:create') === -1 || bornRows.indexOf('human:set_field') === -1) fail('new plan', 'history rows: ' + bornRows.join(', '));

        // ---- SPEC-013: assign
        await nav('#plan/published-plan', '[data-plan-assign-user] option[value="example-learner"]', 'the assign control');
        const offered = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-assign-user] option')).map((o) => o.value)`);
        report.assignOffered = offered;
        if (offered.indexOf('example-learner') === -1 || offered.indexOf('example-retired') !== -1) fail('assign', 'accounts offered: ' + offered.join(','));
        if (await countOf('[data-plan-assign] input, [data-plan-assign] textarea') !== 0) fail('assign', 'the assign control has a free-text field');
        const beforeAssign = revisionOf('published-plan');
        await setValue('[data-plan-assign-audience]', 'personal');
        await setValue('[data-plan-assign-user]', 'example-learner');
        await click('[data-plan-assign-save]');
        await waitRevision(beforeAssign + 1, 'revision after assigning');
        const personal = store.loadWorkingCopy('published-plan');
        if (personal.audience !== 'personal' || personal.assigneeId !== 'example-learner') fail('assign', 'stored ' + personal.audience + ' ' + personal.assigneeId);
        if (!/example-learner/.test(await textOf('[data-plan-assign-summary]'))) fail('assign', 'the summary does not name the account');
        await setValue('[data-plan-assign-audience]', 'shared');
        await click('[data-plan-assign-save]');
        await waitRevision(beforeAssign + 2, 'revision after sharing again');
        const shared = store.loadWorkingCopy('published-plan');
        if (shared.audience !== 'shared' || shared.assigneeId !== null) fail('assign', 'not shared again: ' + shared.audience + ' ' + shared.assigneeId);
        // a disabled account is not offered; if one is sent anyway the server refuses it, in plain words
        await setValue('[data-plan-assign-audience]', 'personal');
        const inject = (name) => cdp.evaluate(`(() => { const s = ${q('[data-plan-assign-user]')}; const o = document.createElement('option'); o.value = ${JSON.stringify(name)}; o.textContent = ${JSON.stringify(name)}; s.appendChild(o); s.value = ${JSON.stringify(name)}; return true; })()`);
        await inject('example-retired');
        await click('[data-plan-assign-save]');
        await waitFor(cdp, `!${q('[data-plan-assign-message]')}.hidden`, 'the assignee_refused message');
        const refused = await textOf('[data-plan-assign-message]');
        if (!/assignee_refused/.test(refused) || !/not active/.test(refused)) fail('assign', 'message: ' + refused);
        if (store.loadWorkingCopy('published-plan').audience !== 'shared') fail('assign', 'a refused assignment changed the plan');
        // an active account whose username the schema's assignee pattern refuses (under 8 characters) is told why
        await inject('learner');
        await click('[data-plan-assign-save]');
        await waitFor(cdp, `/schema/.test(${q('[data-plan-assign-message]')}.textContent)`, 'the schema refusal for a short username');
        const short = await textOf('[data-plan-assign-message]');
        report.shortUsername = short;
        if (!/assigneeId/.test(short)) fail('assign', 'the refusal does not say which field: ' + short);
        if (!/can't be assigned to a personal plan yet/.test(short)) fail('assign', 'the refusal is not the plain username message: ' + short);
        if (store.loadWorkingCopy('published-plan').audience !== 'shared') fail('assign', 'a refused assignment changed the plan');

        // ---- SPEC-014: history, compare, restore
        const HP = 'history-plan';
        const hp = samplePlan();
        hp.id = HP;
        hp.title = 'History plan';
        hp.revision = 1;
        store.saveWorkingCopy({ id: HP, document: JSON.stringify(hp), revision: 1, status: 'draft', audience: 'shared', assigneeId: null });
        const AI_TEXT = 'שָׁלוֹם מֵהָ-AI';
        const aiEdit = ops.setField(store, HP, { path: blockPath('listening'), field: 'passage', value: AI_TEXT, expectedRevision: 1 }, aiCtx);
        if (aiEdit.error) throw new Error('AI edit: ' + JSON.stringify(aiEdit));
        const passageSel = '[data-plan-fields] [data-plan-row="passage"] [data-plan-field="passage"]';
        const passageSave = '[data-plan-fields] [data-plan-row="passage"] [data-plan-save]';
        await nav('#plan/' + HP, LESSON_READY, 'the history plan');
        await edit('listening');
        const HUMAN_TEXT = 'שלום מהאדם';
        await typeInto(passageSel, HUMAN_TEXT);
        await click(passageSave);
        await waitRevision(3, 'revision after the person\'s edit');
        await click('[data-plan-history-toggle]');
        await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length >= 3`, 'three history rows');
        const hist = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-history] [data-plan-revision]')).map((r) => ({ rev: r.getAttribute('data-plan-revision'), actor: r.getAttribute('data-plan-actor'), mark: r.querySelector('[data-plan-actor-mark]').textContent, icon: !!r.querySelector('[data-plan-actor-mark] svg use') }))`);
        report.history = hist.map((h) => h.rev + ':' + h.actor);
        const byRev = (rev) => hist.find((h) => h.rev === String(rev));
        if (!byRev(3) || byRev(3).actor !== 'human' || !/admin/.test(byRev(3).mark)) fail('history', 'the person\'s row: ' + JSON.stringify(byRev(3)));
        if (!byRev(2) || byRev(2).actor !== 'ai' || !/set_field|smoke/.test(byRev(2).mark)) fail('history', 'the AI row: ' + JSON.stringify(byRev(2)));
        if (!byRev(1) || byRev(1).actor !== 'system') fail('history', 'the baseline row: ' + JSON.stringify(byRev(1)));
        if (hist.some((h) => !h.icon)) fail('history', 'an actor mark has no icon');
        if (hist.some((h) => !/[\u0590-\u05FF]/.test(h.mark) || !/[A-Za-z]/.test(h.mark))) fail('history', 'an actor mark is not Hebrew with an English gloss');
        // the filter narrows the list
        await setValue('[data-plan-history-filter]', 'ai');
        await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length === 1`, 'the AI-only list');
        await setValue('[data-plan-history-filter]', '');
        await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length >= 3`, 'the full list again');
        // compare the AI revision and the person's
        await setValue('[data-plan-diff-from]', '2');
        await setValue('[data-plan-diff-to]', '3');
        await click('[data-plan-diff-run]');
        await waitFor(cdp, `!!${q('[data-plan-diff] [data-plan-diff-entry="passage"]')}`, 'the passage in the diff');
        const diff = await cdp.evaluate(`({
            before: ${q('[data-plan-diff-entry="passage"] [data-plan-diff-before]')}.textContent,
            after: ${q('[data-plan-diff-entry="passage"] [data-plan-diff-after]')}.textContent,
            dir: ${q('[data-plan-diff-entry="passage"] [data-plan-diff-before] bdi')}.getAttribute('dir'),
            lang: ${q('[data-plan-diff-entry="passage"] [data-plan-diff-before] bdi')}.getAttribute('lang'),
            head: ${q('[data-plan-diff-head]')}.textContent
        })`);
        report.diff = diff;
        if (diff.before.indexOf(AI_TEXT) === -1 || diff.after.indexOf(HUMAN_TEXT) === -1) fail('diff', 'before/after: ' + JSON.stringify(diff));
        if (diff.dir !== 'rtl' || diff.lang !== 'he') fail('diff', 'a Hebrew value is not rtl/he');
        // restore the earlier value from the diff: one confirmation, then one new revision
        await click('[data-plan-diff-entry="passage"] [data-plan-restore="field"]');
        await waitFor(cdp, `!!${q('[data-plan-restore-box]')}`, 'the restore confirmation');
        if (revisionOf(HP) !== 3) fail('restore', 'the restore was sent before the confirmation');
        if (!/history is kept/.test(await textOf('[data-plan-restore-box]'))) fail('restore', 'the confirmation does not say history is kept');
        await click('[data-plan-restore-cancel]');
        if (await countOf('[data-plan-restore-box]') !== 0 || revisionOf(HP) !== 3) fail('restore', 'cancel sent or kept something');
        await click('[data-plan-diff-entry="passage"] [data-plan-restore="field"]');
        await click('[data-plan-restore-confirm]');
        await waitRevision(4, 'revision after restoring the field');
        if (blockOf(planOf(HP), 'listening').passage !== AI_TEXT) fail('restore', 'the AI value did not come back');
        await waitFor(cdp, `!!${q('[data-plan-history] [data-plan-revision="4"]')}`, 'the restore row');
        const restoreRow = await cdp.evaluate(`({ op: ${q('[data-plan-history] [data-plan-revision="4"] [data-plan-op]')}.textContent, actor: ${q('[data-plan-history] [data-plan-revision="4"]')}.getAttribute('data-plan-actor') })`);
        if (restoreRow.op !== 'restore' || restoreRow.actor !== 'human') fail('restore', 'the new row: ' + JSON.stringify(restoreRow));
        // restore a whole revision (the person's), and the plan view is at the new revision
        await click('[data-plan-history] [data-plan-revision="3"] [data-plan-restore="whole"]');
        await click('[data-plan-restore-confirm]');
        await waitRevision(5, 'revision after the whole restore');
        if (blockOf(planOf(HP), 'listening').passage !== HUMAN_TEXT) fail('restore', 'the whole restore did not bring back revision 3');
        await edit('listening');
        if (await cdp.evaluate(`${q(passageSel)}.value`) !== HUMAN_TEXT) fail('restore', 'the editor does not show the restored text');
        // a stale restore (the AI wrote in between) is a 409 with a way back
        const bump = ops.setField(store, HP, { path: LESSON, field: 'footer', value: 'the AI footer', expectedRevision: 5 }, aiCtx);
        if (bump.error) throw new Error('bump: ' + JSON.stringify(bump));
        await click('[data-plan-history] [data-plan-revision="2"] [data-plan-restore="whole"]');
        await click('[data-plan-restore-confirm]');
        await waitFor(cdp, `!${q('[data-plan-restore-error]')}.hidden`, 'the restore conflict message');
        const conflictText = await textOf('[data-plan-restore-error]');
        if (!/revision_conflict 6/.test(conflictText)) fail('restore 409', 'message: ' + conflictText);
        if (revisionOf(HP) !== 6) fail('restore 409', 'a refused restore wrote');
        await click('[data-plan-history-reload]');
        await waitRevision(6, 'revision after Reload');
        // the mark: a person wrote this field last; it opens to the earlier value and never blocks typing
        await edit('listening');
        await waitFor(cdp, `!!${q('[data-plan-edited-mark]')}`, 'the edited-by-a-person mark');
        await click('[data-plan-edited-mark]');
        if (await cdp.evaluate(`${q('[data-plan-edited-detail]')}.hidden`)) fail('mark', 'the detail did not open');
        const prev = await textOf('[data-plan-prev-value]');
        if (!prev.trim()) fail('mark', 'no earlier value shown');
        await click('[data-plan-edited-detail] [data-plan-restore="value"]');
        await waitFor(cdp, `!!${q('[data-plan-edited-detail] [data-plan-restore-box]')}`, 'the confirmation under the mark');
        await click('[data-plan-edited-detail] [data-plan-restore-cancel]');
        if (revisionOf(HP) !== 6) fail('mark', 'cancel wrote');
        await click('[data-plan-edited-detail] [data-plan-restore="value"]');
        await click('[data-plan-edited-detail] [data-plan-restore-confirm]');
        await waitRevision(7, 'revision after restoring from the mark');
        // nothing is disabled because of a mark: only list-end move buttons and the media panel's own gates
        await edit('listening');
        const disabled = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-fields] :is(input, textarea, select, button)')).filter((el) => el.disabled || el.hasAttribute('readonly')).filter((el) => !el.matches('[data-plan-move], .plan-item-ops *, [data-plan-insert], [data-plan-generate], [data-plan-approve]')).map((el) => el.tagName + ' ' + (el.getAttribute('data-plan-field') || el.className))`);
        if (disabled.length) fail('mark', 'a control is disabled for another reason: ' + disabled.join('; '));
        // and the person's own edit in a marked field still saves
        await typeInto(passageSel, 'שלום אחרי הכול');
        await click(passageSave);
        await waitRevision(8, 'revision after editing a marked field');

        // ---- SPEC-015: issues, jump, fix, preview, publish
        const IP = 'issues-plan';
        await nav('#plan/' + IP, '[data-plan-issues="panel"] [data-plan-issue]', 'the issues panel');
        const issues = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-issues="panel"] [data-plan-issue]')).map((i) => i.getAttribute('data-plan-issue') + ':' + i.getAttribute('data-plan-issue-severity'))`);
        report.issues = issues;
        for (const want of ['pointing-form:warn', 'lexicon-form:warn', 'audio-stale:warn']) {
            if (issues.indexOf(want) === -1) fail('issues', 'no ' + want + ' in ' + issues.join(', '));
        }
        const counts = await textOf('[data-plan-issues-count]');
        if (!/אזהרות/.test(counts) || !/warnings/.test(counts)) fail('issues', 'counts: ' + counts);
        const groupsText = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-issues="panel"] [data-plan-issue-group="warn"]')).map((g) => g.textContent).join(' ')`);
        if (/blocks? publish/i.test(groupsText.replace(/does not block publish/g, ''))) fail('issues', 'a warning is called a blocker');
        const note = await textOf('[data-plan-publish-note]');
        if (!/Warnings do not stop publishing/.test(note) || /approv/i.test(note)) fail('issues', 'publish note: ' + note);
        // a link jumps to the field
        await click('[data-plan-issue="pointing-form"] [data-plan-issue-link]');
        await waitFor(cdp, `!!${q('[data-plan-jump]')}`, 'the jumped field');
        const jumped = await cdp.evaluate(`(() => {
            const a = document.activeElement;
            const r = ${q('[data-plan-jump]')}.getBoundingClientRect();
            return { field: a && a.getAttribute('data-plan-field'), inView: r.top >= 0 && r.bottom <= window.innerHeight };
        })()`);
        if (jumped.field !== 'passage' || !jumped.inView) fail('issues jump', JSON.stringify(jumped));
        // the one-click fix on a lexicon-form item
        const beforeFix = revisionOf(IP);
        await click('[data-plan-issue="lexicon-form"] [data-plan-fix]');
        await waitFor(cdp, `!${q('[data-plan-issues="panel"] [data-plan-issue="lexicon-form"]')}`, 'the lexicon-form item to clear');
        if (revisionOf(IP) !== beforeFix + 1) fail('issues fix', 'the fix was not exactly one write');
        if (blockOf(planOf(IP), 'intro').words[1].word !== 'בַּיִת'.normalize('NFC')) fail('issues fix', 'the lexicon form was not stored');
        // the live preview: typing changes it before any save, with no request
        await waitFor(cdp, `!!${q('[data-plan-fields] [data-plan-field="passage"]')}`, 'the passage field');
        const stateOf = () => cdp.evaluate(`${q('[data-plan-preview]')}.getAttribute('data-plan-preview-state')`);
        if (await stateOf() !== 'saved') fail('preview', 'state before typing is ' + await stateOf());
        const reqs = requestsBefore();
        await typeInto('[data-plan-fields] [data-plan-field="passage"]', 'שלום חדש מאוד');
        await waitFor(cdp, `${q('[data-plan-preview]')}.textContent.indexOf('חדש מאוד') !== -1`, 'the preview to change', 3000);
        if (await stateOf() !== 'unsaved') fail('preview', 'state after typing is ' + await stateOf());
        if (!/not saved/.test(await textOf('[data-plan-preview-label]'))) fail('preview', 'no "not saved" label');
        // No API request. The learner's sound icon is a <use> of the brand sprite, and the server marks static files
        // no-cache, so redrawing the preview makes Chrome revalidate that one file: a same-origin GET of an icon file.
        const typingRequests = cdp.requests.slice(reqs).filter((u) => !/\/shared\/brand\/sprite\.svg$/.test(u));
        if (typingRequests.length) fail('preview', 'typing made request(s): ' + typingRequests.join(', '));
        report.previewTypingRequests = cdp.requests.slice(reqs).map((u) => u.replace(/^https?:\/\/[^/]+/, ''));
        if (blockOf(planOf(IP), 'listen').passage === 'שלום חדש מאוד') fail('preview', 'typing saved something');
        await click('[data-plan-fields] [data-plan-row="passage"] [data-plan-save]');
        await waitFor(cdp, `${q('[data-plan-preview]')}.getAttribute('data-plan-preview-state') === 'saved'`, 'the saved state');
        // hostile text shows as text
        await typeInto('[data-plan-fields] [data-plan-field="passage"]', '<img src=x onerror=alert(1)>');
        await waitFor(cdp, `${q('[data-plan-preview]')}.textContent.indexOf('<img src=x') !== -1`, 'the hostile text in the preview', 3000);
        if (await countOf('[data-plan-preview] img, [data-plan-preview] [onerror]') !== 0) fail('preview', 'typed markup became an element');
        // the learner still cannot read a draft, and nothing went to a preview route
        const learnerRead = await cdp.evaluate(`fetch('/api/plans/${HP}/published', { credentials: 'same-origin' }).then((r) => r.status)`);
        if (learnerRead !== 404) fail('preview', 'GET /api/plans/:id/published answered ' + learnerRead + ' for a draft');
        if (cdp.requests.some((u) => /^\/api\/.*preview/i.test(u.replace(/^https?:\/\/[^/]+/, '')))) fail('preview', 'a request went to a preview route');
        // publish: counts first, no approval, the refusal or the result after
        await nav('#plan/' + IP, '[data-plan-publish-note]', 'the issues plan again');
        await waitFor(cdp, `${q('[data-plan-publish-note]')}.textContent.length > 0`, 'the publish note');
        await click('[data-plan-publish]');
        await waitFor(cdp, `/Published/.test(${q('[data-plan-notice]')}.textContent)`, 'the publish result', 15000);
        const published = await textOf('[data-plan-notice]');
        report.published = published;
        if (!/warnings/.test(published)) fail('publish', 'the result does not show the issue counts: ' + published);
        if (store.loadWorkingCopy(IP).status !== 'published') fail('publish', 'not published');

        // ---- SPEC-016: the AI-first loop, end to end. The steps by the AI are the registered MCP tool handlers
        // (create_plan, insert_block, set_field, publish_plan); the steps by the person are made in the browser.
        const EP = 'e2e-ai-plan';
        const E_LESSON = 'units/unit-1/lessons/lesson-1';
        const E_W2 = E_LESSON + '/blocks/intro/words/w2';
        const E_CLIP = 'sentences/example-house.mp3';
        const E_SENTENCE_AI = 'זֶה בַּיִת.';
        const E_MEANING_AI = 'house';
        const priorToken = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'smoke-ai-first-token';
        const toolsModule = require('../mcp/tools');
        toolsModule.registerLessonTools({ registerTool() { /* only the handlers are wanted */ } });
        const mcpTools = toolsModule.toolHandlers;
        const callTool = async (name, args) => {
            const r = await mcpTools[name](args);
            const text = r.content[0].text;
            if (r.isError) throw new Error('MCP ' + name + ' failed: ' + text.slice(0, 300));
            try { return JSON.parse(text); } catch (_) { return null; }
        };
        try {
            await callTool('create_plan', { id: EP, title: 'Example plan', recipe: 'topic-conversation' });
            const eRev = () => revisionOf(EP);
            await callTool('insert_block', {
                id: EP, path: E_LESSON, index: 0, expectedRevision: eRev(),
                block: { id: 'intro', type: 'introduction', words: [
                    { id: 'w1', word: 'אַתָּה', meaning: 'you', translit: 'ata' },
                    { id: 'w2', word: 'בַּיִת', meaning: 'a house', translit: 'bayit' }
                ] }
            });
            await callTool('remove_block', { id: EP, path: E_LESSON + '/blocks/placeholder', expectedRevision: eRev() });
            for (const [path, field, value] of [
                [E_W2, 'meaning', E_MEANING_AI], [E_W2, 'sentence', E_SENTENCE_AI], [E_W2, 'sentenceAudioPath', E_CLIP],
                [E_LESSON, 'newWords', ['אַתָּה', 'בַּיִת']], [E_LESSON, 'visibility', 'everyone'],
                [E_LESSON + '/blocks/intro/words/w1', 'translit', 'אתה'] // a warning the AI leaves on purpose
            ]) {
                await callTool('set_field', { id: EP, path, field, value, expectedRevision: eRev() });
            }
            const issuesBefore = await callTool('list_plan_issues', { id: EP });
            if (issuesBefore.counts['block'] !== 0 || !issuesBefore.issues.some((i) => i.code === 'field-language')) fail('e2e', 'the plan the AI made should have only warnings: ' + JSON.stringify(issuesBefore.counts));
            const publishedBody = await callTool('publish_plan', { id: EP, expectedRevision: eRev() });
            if (publishedBody.status !== 'published') fail('e2e', 'the AI could not publish');
            const publishedRevision = eRev();

            // the list shows the plan as published, made by the AI
            await nav('#plans', '[data-plan-list] [data-plan-row]', 'the plan list for the e2e plan');
            const eRow = await cdp.evaluate(`(() => { const r = ${q('[data-plan-row="' + EP + '"]')}; return r ? r.getAttribute('data-plan-row-status') : null; })()`);
            if (eRow !== 'published') fail('e2e', 'the list shows ' + EP + ' as ' + eRow);

            // the person opens it and fixes two fields in the editor
            await nav('#plan/' + EP, LESSON_READY, 'the e2e plan');
            await cdp.evaluate(`${q('[data-plan-block="intro"] [data-plan-edit]')}.click(); true`);
            await waitFor(cdp, `!!${q('[data-plan-fields] ' + nodeSel(E_W2))}`, 'the form of the second word');
            const field = (name, what) => nodeSel(E_W2) + ' > [data-plan-row="' + name + '"] ' + what;
            await typeInto(field('meaning', '[data-plan-field="meaning"]'), 'a home');
            await click(field('meaning', '[data-plan-save]'));
            await waitRevision(publishedRevision + 1, 'revision after the person fixed the meaning');
            await typeInto(field('sentence', '[data-plan-field="sentence"]'), 'זֶה הַבַּיִת שֶׁלִּי.');
            await click(field('sentence', '[data-plan-save]'));
            await waitRevision(publishedRevision + 2, 'revision after the person fixed the sentence');
            const eDoc = planOf(EP);
            const eWord = eDoc.units[0].lessons[0].blocks[0].words[1];
            if (eWord.meaning !== 'a home' || eWord.sentenceAudioPath !== E_CLIP) fail('e2e', 'stored word: ' + JSON.stringify(eWord));
            if (store.loadWorkingCopy(EP).status !== 'draft') fail('e2e', 'the plan is not a draft after an edit by a person');

            // the AI re-reads the part and sees that a person changed both fields
            const read = await callTool('read_part', { id: EP, path: E_W2 });
            const seen = read.editState.humanEdited.map((e) => e['field'] + ':' + e.previousWriter + ':' + e.previousValue);
            report.e2eEditState = seen;
            if (seen.indexOf('meaning:ai:' + E_MEANING_AI) === -1 || seen.indexOf('sentence:ai:' + E_SENTENCE_AI) === -1) fail('e2e', 'editState: ' + seen.join(' | '));
            if (JSON.stringify(read).indexOf('"by"') !== -1 || JSON.stringify(read).indexOf('"admin"') !== -1) fail('e2e', 'the MCP result names the person');

            // History shows the AI and the person; the mark shows on the fields a person wrote; the Issues panel lists the left-over warnings
            await click('[data-plan-history-toggle]');
            await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length >= ${publishedRevision + 2}`, 'the e2e history rows');
            const eHist = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-history] [data-plan-revision]')).map((r) => r.getAttribute('data-plan-actor') + ':' + r.querySelector('[data-plan-op]').textContent)`);
            report.e2eHistory = eHist;
            if (eHist.filter((h) => h === 'human:set_field').length !== 2) fail('e2e', 'history shows ' + eHist.join(', '));
            if (eHist.indexOf('ai:create') === -1 || eHist.indexOf('ai:publish') === -1) fail('e2e', 'history lacks the AI rows: ' + eHist.join(', '));
            if (eHist.some((h) => /^(?!ai|human|system)/.test(h))) fail('e2e', 'an unknown actor mark: ' + eHist.join(', '));
            await waitFor(cdp, `!!${q('[data-plan-issues="panel"] [data-plan-issue]')}`, 'the e2e issues panel');
            const eIssues = await cdp.evaluate(`Array.from(document.querySelectorAll('[data-plan-issues="panel"] [data-plan-issue]')).map((i) => i.getAttribute('data-plan-issue') + ':' + i.getAttribute('data-plan-issue-severity'))`);
            report.e2eIssues = eIssues;
            for (const want of ['field-language:warn', 'audio-stale:warn']) if (eIssues.indexOf(want) === -1) fail('e2e', 'issues lack ' + want + ': ' + eIssues.join(', '));
            if (eIssues.some((s) => /:block$/.test(s))) fail('e2e', 'a hard error in the plan the AI published: ' + eIssues.join(', '));
            await waitFor(cdp, `!!${q('[data-plan-edited-mark]')}`, 'the edited-by-a-person mark on the e2e plan');
            const marks = await countOf('[data-plan-edited-mark]');
            if (marks < 2) fail('e2e', 'expected a mark on each of the two fields, saw ' + marks);

            // restore the published meaning (written by the AI) from a comparison with the published copy: one confirmation, one new revision
            const beforeRestore = eRev();
            await click('[data-plan-diff-quick="working-published"]');
            await waitFor(cdp, `!!${q('[data-plan-diff-entry="meaning"] [data-plan-restore="field"]')}`, 'the meaning in the published-to-working diff');
            await click('[data-plan-diff-entry="meaning"] [data-plan-restore="field"]');
            await waitFor(cdp, `!!${q('[data-plan-restore-box]')}`, 'the e2e restore confirmation');
            if (eRev() !== beforeRestore) fail('e2e', 'the restore was sent before it was confirmed');
            await click('[data-plan-restore-confirm]');
            await waitRevision(beforeRestore + 1, 'revision after the e2e restore');
            const restoredWord = planOf(EP).units[0].lessons[0].blocks[0].words[1];
            if (restoredWord.meaning !== E_MEANING_AI) fail('e2e', 'the restore did not bring back the AI value: ' + restoredWord.meaning);
            if (restoredWord.sentence !== 'זֶה הַבַּיִת שֶׁלִּי.') fail('e2e', 'the restore changed another field');
            const lastRow = await cdp.evaluate(`(() => { const r = document.querySelector('[data-plan-history] [data-plan-revision="${beforeRestore + 1}"]'); return r ? r.getAttribute('data-plan-actor') + ':' + r.querySelector('[data-plan-op]').textContent : null; })()`);
            if (lastRow !== 'human:restore') fail('e2e', 'the restore row is ' + lastRow);

            // the AI writes again and is told what it overwrote; nothing is refused
            const again = await callTool('set_field', { id: EP, path: E_W2, field: 'sentence', value: E_SENTENCE_AI, expectedRevision: eRev() });
            if (!again.overwrote || again.overwrote[0]['field'] !== 'sentence') fail('e2e', 'the write by the AI did not list overwrote: ' + JSON.stringify(again.overwrote));

            // the learner route: the snapshot, never the draft
            const snap = await cdp.evaluate(`fetch('/api/plans/${EP}/published', { credentials: 'same-origin' }).then((r) => r.status === 200 ? r.json().then((j) => j.revision) : r.status)`);
            if (snap !== publishedRevision) fail('e2e', 'the published read answered ' + snap + ', wanted revision ' + publishedRevision);
        } finally {
            if (priorToken === undefined) delete process.env.MCP_AUTH_TOKEN;
            else process.env.MCP_AUTH_TOKEN = priorToken;
        }

        // ---- a learner gets none of it
        await chromeCdp.signInAs(cdp, harness, BASE, 'learner');
        for (const hash of ['#plans', '#plan/' + HP]) {
            await cdp.call('Page.navigate', { url: 'about:blank' });
            await cdp.call('Page.navigate', { url: BASE + '/admin/' + hash });
            await sleep(2500);
            const present = await countOf('[data-plan-list], [data-plan-new], [data-plan-history], [data-plan-issues="panel"], [data-plan-assign]');
            if (present !== 0) fail('learner', hash + ' shows plan markup to a learner');
        }
        const learnerCalls = await cdp.evaluate(`Promise.all(['/api/plans', '/api/plans/${HP}/revisions', '/api/plans/${HP}/issues'].map((u) => fetch(u, { credentials: 'same-origin' }).then((r) => r.status)))`);
        report.learnerStatuses = learnerCalls;
        if (learnerCalls.some((s) => s !== 403)) fail('learner', 'plan routes answered ' + learnerCalls.join(','));
        await chromeCdp.signInAs(cdp, harness, BASE, 'admin');

        // ---- layout, targets, names, contrast at 360 and 1280 px
        for (const width of [360, 1280]) {
            // mobile: false, so the layout viewport stays at ${width} and a too-wide control shows as overflow
            // (with mobile emulation Chrome widens the layout viewport to fit the content and hides it).
            await cdp.call('Emulation.setDeviceMetricsOverride', { width, height: width === 360 ? 800 : 900, deviceScaleFactor: 1, mobile: false });
            // the block audits below use the sample plan's blocks; the last pass ended on another plan
            await nav('#plan/' + HP, LESSON_READY, 'the sample plan at ' + width);
            for (const blockId of ['introduction', 'listening', 'explainer']) {
            await edit(blockId);
            const a = await cdp.evaluate('(' + pageAudit + ')()');
            report.audit[width + ' ' + blockId] = { texts: a.texts, scrollWidth: a.overflow.scrollWidth };
            if (a.overflow.scrollWidth > a.overflow.innerWidth) fail('layout@' + width + ' ' + blockId, 'horizontal overflow ' + a.overflow.scrollWidth + ' > ' + a.overflow.innerWidth);
            if (a.small.length) fail('targets@' + width, a.small.slice(0, 6).join('; '));
            if (a.big.length) fail('primary@' + width, a.big.slice(0, 6).join('; '));
            if (a.unnamed.length) fail('names@' + width, a.unnamed.slice(0, 6).join('; '));
            if (a.contrast.length) fail('contrast@' + width, a.contrast.slice(0, 6).join('; '));
            }

            // the Plans tab with the New plan form open, the issues panel, and History with a comparison,
            // a restore confirmation, and the "edited by a person" detail open
            const auditPage = async (label) => {
                const a = await cdp.evaluate('(' + pageAudit + ')()');
                report.audit[width + ' ' + label] = { texts: a.texts, scrollWidth: a.overflow.scrollWidth };
                if (a.overflow.scrollWidth > a.overflow.innerWidth) fail('layout@' + width + ' ' + label, 'horizontal overflow ' + a.overflow.scrollWidth + ' > ' + a.overflow.innerWidth);
                if (a.small.length) fail('targets@' + width + ' ' + label, a.small.slice(0, 6).join('; '));
                if (a.big.length) fail('primary@' + width + ' ' + label, a.big.slice(0, 6).join('; '));
                if (a.unnamed.length) fail('names@' + width + ' ' + label, a.unnamed.slice(0, 6).join('; '));
                if (a.contrast.length) fail('contrast@' + width + ' ' + label, a.contrast.slice(0, 6).join('; '));
            };
            await nav('#plans', '[data-plan-list] [data-plan-row]', 'the plan list at ' + width);
            await click('[data-plan-new-open]');
            await waitFor(cdp, `!!${q('[data-plan-new]')}`, 'the New plan form at ' + width);
            await auditPage('plans');
            await nav('#plan/' + IP, '[data-plan-issues="panel"] [data-plan-issue]', 'issues at ' + width);
            await auditPage('issues');
            await nav('#plan/' + HP, LESSON_READY, 'the history plan at ' + width);
            await click('[data-plan-history-toggle]');
            await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length >= 3`, 'history rows at ' + width);
            await setValue('[data-plan-diff-from]', '2');
            await setValue('[data-plan-diff-to]', '3');
            await click('[data-plan-diff-run]');
            await waitFor(cdp, `!!${q('[data-plan-diff] [data-plan-diff-entry]')}`, 'a diff at ' + width);
            await click('[data-plan-history] [data-plan-revision="3"] [data-plan-restore="whole"]');
            await waitFor(cdp, `!!${q('[data-plan-restore-box]')}`, 'a confirmation at ' + width);
            await edit('listening');
            await waitFor(cdp, `!!${q('[data-plan-edited-mark]')}`, 'a mark at ' + width);
            await click('[data-plan-edited-mark]');
            await auditPage('history');
            // the plan the AI built and published, after edits by a person and a restore (SPEC-016)
            await nav('#plan/' + EP, LESSON_READY, 'the e2e plan at ' + width);
            await click('[data-plan-history-toggle]');
            await waitFor(cdp, `document.querySelectorAll('[data-plan-history] [data-plan-revision]').length >= 5`, 'the e2e history at ' + width);
            await waitFor(cdp, `!!${q('[data-plan-issues="panel"] [data-plan-issue]')}`, 'the e2e issues at ' + width);
            await auditPage('e2e');
        }

        report.requests = cdp.requests.length;
        const foreign = cdp.requests.filter((u) => !sameOrigin(u));
        if (foreign.length) fail('network', 'requests that left the origin: ' + Array.from(new Set(foreign)).slice(0, 6).join(', '));
        if (cdp.requests.length < 10) fail('network', 'only ' + cdp.requests.length + ' requests were seen');
        if (cdp.errors.length) fail('page errors', cdp.errors.slice(0, 3).join(' | '));
    } finally {
        await browser.close();
        await harness.stop();
    }
}

require('./helpers/chrome-cdp').skipIfMissing("chrome-plan-editor-smoke") || main().then(() => {
    console.log(JSON.stringify(report, null, 2));
    if (failures.length) {
        console.error('\nFAILURES (' + failures.length + '):');
        for (const f of failures) console.error(' - ' + f);
        console.error('chrome-plan-editor-smoke: FAIL');
        process.exit(1);
    }
    console.log('chrome-plan-editor-smoke: PASS');
}).catch((err) => {
    console.error(err && err.stack ? err.stack : err);
    process.exit(1);
});
