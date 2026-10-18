/**
 * Verb practice: model, ladder, rounds, judging, rendering, the draft-form signal rule,
 * the plan-page studio branch, and the /verbs/ route.
 * Provenance: INIT-013 follow-up (verb practice redesign).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const VP = require('../shared/verb-practice.js');
const seed = require('../scripts/seed-verb-tense-workshop.js');

const ROOT = path.resolve(__dirname, '..');
const DAY = VP.DAY;

function fixture(opts) {
    const o = opts || {};
    const forms = {};
    for (const f of seed.WORKSHOP_FORMS) {
        forms[f.key] = Object.assign({}, f, { verified: o.verified ? 1 : 0 });
    }
    const lemmas = {};
    for (const e of seed.ENTRIES.filter((x) => x.kind === 'lemma')) lemmas[e.key] = e;
    const paradigms = {};
    for (const f of seed.WORKSHOP_FORMS) {
        const p = paradigms[f.lemma_key] = paradigms[f.lemma_key] || {};
        (p[f.tense] = p[f.tense] || {})[f.cell] = forms[f.key];
    }
    const blocks = VP.conjugationBlocks(seed.PLANS[0], '');
    return { forms, lemmas, paradigms, blocks };
}

function verbsOf(opts) {
    const fx = fixture(opts);
    return VP.collectVerbs(fx.blocks, fx.forms, fx.lemmas, fx.paradigms);
}

function seededRng(seedValue) {
    let s = seedValue || 1;
    return () => {
        s = (s * 16807) % 2147483647;
        return (s - 1) / 2147483646;
    };
}

function memoryStorage() {
    const data = {};
    return {
        data,
        getItem: (k) => (Object.prototype.hasOwnProperty.call(data, k) ? data[k] : null),
        setItem: (k, v) => { data[k] = String(v); }
    };
}

describe('verb model', () => {
    it('fills past, present and future for every verb from the lexicon paradigm', () => {
        const verbs = verbsOf();
        assert.equal(verbs.length, 4);
        for (const verb of verbs) {
            assert.equal(VP.allForms([verb]).length, 22, verb.lemma);
            for (const tense of VP.TENSES) {
                assert.equal(Object.keys(verb.tenses[tense]).length, VP.CELLS[tense].length, verb.lemma + ' ' + tense);
            }
            assert.equal('cells' in verb, false);
        }
    });

    it('falls back to the block cells when the lexicon has no paradigm, and drops verbs with no forms', () => {
        const fx = fixture();
        const verbs = VP.collectVerbs(fx.blocks, fx.forms, fx.lemmas, {});
        assert.ok(verbs.length >= 1);
        for (const verb of verbs) assert.ok(VP.allForms([verb]).length > 0);
        assert.deepEqual(VP.collectVerbs(fx.blocks, {}, fx.lemmas, {}), []);
    });

    it('never shows the lexicon key tag (e.g. -3fs) as part of a form', () => {
        assert.equal(VP.displayKey('abc-3fs'), 'abc');
        assert.equal(VP.displayKey('abc'), 'abc');
        const verbs = verbsOf();
        for (const form of VP.allForms(verbs)) assert.equal(/-[0-9a-z]+$/i.test(form.he), false, form.key);
        const look = VP.renderLook(verbs, verbs[0].lemma, '');
        assert.equal(/-3fs/.test(look), false);
    });

    it('marks exactly the three root letters, skipping the person suffix', () => {
        const verbs = verbsOf();
        for (const verb of verbs) {
            for (const form of VP.allForms([verb])) {
                const html = VP.markRoot(form, verb.root);
                const marked = (html.match(/class="root-letter"/g) || []).length;
                assert.equal(marked, 3, form.he + ' ' + html);
            }
        }
    });
});

describe('judging', () => {
    const form = { he: '\u05de\u05b7\u05d7\u05b0\u05dc\u05b4\u05d9\u05d8', accept: ['\u05de\u05d7\u05dc\u05d9\u05d8'] };

    it('ignores an empty answer, accepts the form without vowel points, rejects anything else', () => {
        assert.equal(VP.isMatch('', form), null);
        assert.equal(VP.isMatch('   ', form), null);
        assert.equal(VP.isMatch('\u05de\u05d7\u05dc\u05d9\u05d8', form), true);
        assert.equal(VP.isMatch(form.he, form), true);
        assert.equal(VP.isMatch('\u05de\u05d7\u05dc\u05d9\u05d8\u05d4', form), false);
    });
});

describe('ladder', () => {
    it('moves pick -> type -> known and drops back after a miss', () => {
        let ladder = {};
        ladder = VP.applyResult(ladder, 'f', 'recognize', 'first', 0);
        assert.equal(ladder.f.l, 1);
        assert.equal(ladder.f.d, 0);
        assert.equal(VP.modeFor(ladder.f, false), 'type');
        ladder = VP.applyResult(ladder, 'f', 'type', 'first', 0);
        assert.equal(ladder.f.l, 2);
        assert.equal(ladder.f.d, DAY);
        ladder = VP.applyResult(ladder, 'f', 'type', 'first', 0);
        ladder = VP.applyResult(ladder, 'f', 'type', 'first', 0);
        assert.equal(ladder.f.l, 4);
        assert.equal(ladder.f.d, 7 * DAY);
        ladder = VP.applyResult(ladder, 'f', 'type', 'miss', 0);
        assert.equal(ladder.f.l, 1);
        assert.equal(ladder.f.d, 0);
        assert.equal(ladder.f.w, 1);
    });

    it('starts a never-right form as a pick and caps the level', () => {
        assert.equal(VP.modeFor(null, false), 'recognize');
        let ladder = {};
        for (let i = 0; i < 9; i += 1) ladder = VP.applyResult(ladder, 'f', 'type', 'first', 0);
        assert.equal(ladder.f.l, VP.INTERVAL_DAYS.length - 1);
    });

    it('stores per user and survives bad JSON', () => {
        const storage = memoryStorage();
        VP.saveLadder(storage, { username: 'dana' }, { a: { l: 2, d: 5, r: 1, w: 0, s: 1 } });
        assert.ok(storage.data[VP.storageKey({ username: 'dana' })]);
        assert.equal(VP.loadLadder(storage, { username: 'dana' }).a.l, 2);
        assert.deepEqual(Object.keys(VP.loadLadder(storage, { username: 'other' })), []);
        storage.setItem(VP.storageKey({ username: 'dana' }), '{not json');
        assert.deepEqual(Object.keys(VP.loadLadder(storage, { username: 'dana' })), []);
    });

    it('counts due and known forms', () => {
        const ladder = { a: { l: 2, d: 10, r: 1, w: 0, s: 0 }, b: { l: 0, d: 0, r: 0, w: 1, s: 0 } };
        assert.deepEqual(VP.dueSummary(ladder, 5), { due: 1, known: 1, seen: 2 });
    });
});

describe('rounds', () => {
    it('builds 12 cards, due forms first, at most 3 new when enough are due', () => {
        const verbs = verbsOf();
        const pool = VP.poolOf(verbs, {});
        const ladder = {};
        pool.slice(0, 15).forEach((form) => { ladder[form.id] = { l: 2, d: 0, r: 1, w: 0, s: 0 }; });
        const round = VP.buildRound(pool, ladder, DAY, seededRng(7));
        assert.equal(round.length, VP.ROUND);
        const fresh = round.filter((form) => !ladder[form.id]).length;
        assert.ok(fresh <= 3, 'new forms: ' + fresh);
    });

    it('does not put the same verb twice in a row when another verb can go between', () => {
        const verbs = verbsOf();
        const round = VP.buildRound(VP.poolOf(verbs, {}), {}, 0, seededRng(3));
        let repeats = 0;
        for (let i = 1; i < round.length; i += 1) if (round[i].lemma === round[i - 1].lemma) repeats += 1;
        assert.ok(repeats <= 1, 'repeats: ' + repeats);
    });

    it('offers four different choices that include the answer', () => {
        const verbs = verbsOf();
        for (const form of VP.allForms(verbs)) {
            const verb = verbs.find((v) => v.lemma === form.lemma);
            const options = VP.optionsFor(form, verb, seededRng(11));
            assert.equal(options.length, 4, form.he);
            assert.ok(options.includes(form.he));
            assert.equal(new Set(options).size, 4);
        }
    });

    it('respects the verb and tense choices', () => {
        const verbs = verbsOf();
        const s = VP.newSession(verbs, { verbs: [verbs[1].lemma], tenses: ['future'] }, {}, 0, seededRng(5));
        assert.ok(s.items.length > 0);
        for (const item of s.items) {
            assert.equal(item.form.lemma, verbs[1].lemma);
            assert.equal(item.form.tense, 'future');
            assert.equal(item.mode, 'recognize');
        }
    });
});

describe('rendering', () => {
    it('keeps the table hidden until the learner answers', () => {
        const verbs = verbsOf();
        const settings = { verbs: verbs.map((v) => v.lemma), tenses: VP.TENSES.slice() };
        const session = VP.newSession(verbs, settings, {}, 0, seededRng(2));
        const ask = VP.renderPractice({ verbs, settings, session, ladder: {} });
        assert.match(ask, /data-vp-table="hidden"/);
        assert.equal(/data-vp-table="open"/.test(ask), false);
        assert.match(ask, /data-vp-option=/);
        assert.match(ask, /class="vp-draft"/);
        session.phase = 'reveal';
        const shown = VP.renderPractice({ verbs, settings, session, ladder: {} });
        assert.match(shown, /data-vp-table="open"/);
        assert.match(shown, /aria-current="true"/);
    });

    it('shows Use only when the lesson has sentence work', () => {
        const verbs = verbsOf();
        const without = VP.renderStudio({ verbs, lookHtml: '' });
        assert.equal(/data-vs-tab="use"/.test(without), false);
        const withUse = VP.renderStudio({ verbs, lookHtml: '', useHtml: '<p>x</p>', tab: 'practice' });
        assert.match(withUse, /role="tab"[^>]*aria-selected="true"[^>]*data-vs-tab="practice"/);
        assert.match(withUse, /data-vs-panel="look" hidden/);
        assert.match(withUse, /data-vs-tab="use"/);
    });

    it('escapes plan text', () => {
        const verbs = verbsOf();
        const html = VP.renderStudio({ verbs, title: '<img src=x onerror=alert(1)>', lookHtml: '' });
        assert.equal(html.includes('<img'), false);
    });
});

/** Just enough DOM for mount(): delegated clicks, the practice host, and the typing field. */
function fakeStudio() {
    const handlers = {};
    const hostEl = { innerHTML: '', querySelector: () => null, closest: () => null };
    const field = { value: '' };
    const el = {
        addEventListener: (type, fn) => { handlers[type] = fn; },
        removeEventListener: (type) => { delete handlers[type]; },
        querySelector: (sel) => (sel === '[data-vp-host]' ? hostEl : sel === '[data-vp-input]' ? field : null),
        querySelectorAll: () => []
    };
    function click(attr, value) {
        const node = { disabled: false, getAttribute: (name) => (name === attr ? value : null) };
        handlers.click({ target: { closest: (sel) => (sel === '[' + attr + ']' ? node : null) } });
    }
    return { el, hostEl, field, click };
}

describe('mount: what is sent and saved', () => {
    function start(verified) {
        const verbs = verbsOf({ verified });
        const posted = [];
        const storage = memoryStorage();
        const studio = fakeStudio();
        const ctl = VP.mount(studio.el, {
            verbs, user: { username: 't' }, storage, now: () => 0, rng: seededRng(9), slug: 'lesson-a',
            post: (signal) => posted.push(signal)
        });
        return { verbs, posted, storage, studio, ctl };
    }

    it('sends no form signal for a draft form, even when typed', () => {
        const t = start(false);
        const item = t.ctl.state.session.items[0];
        item.mode = 'type';
        t.studio.field.value = item.form.he;
        t.studio.click('data-vp-action', 'check');
        assert.equal(t.ctl.state.session.phase, 'right');
        assert.deepEqual(t.posted, []);
        assert.ok(t.storage.data[VP.storageKey({ username: 't' })]);
    });

    it('sends one form signal for a checked form typed from memory', () => {
        const t = start(true);
        const item = t.ctl.state.session.items[0];
        item.mode = 'type';
        t.studio.field.value = item.form.he;
        t.studio.click('data-vp-action', 'check');
        assert.equal(t.posted.length, 1);
        assert.equal(t.posted[0].kind, 'form');
        assert.equal(t.posted[0].result, 'pass');
        assert.equal(t.posted[0].slug, 'lesson-a');
    });

    it('sends nothing for a pick (recognition is not recall)', () => {
        const t = start(true);
        const item = t.ctl.state.session.items[0];
        assert.equal(item.mode, 'recognize');
        t.studio.click('data-vp-option', item.form.he);
        assert.equal(t.ctl.state.session.phase, 'right');
        assert.deepEqual(t.posted, []);
    });

    it('gives one retry, then shows the form and brings it back once later in the round', () => {
        const t = start(true);
        const s = t.ctl.state.session;
        const count = s.items.length;
        const item = s.items[0];
        item.mode = 'type';
        t.studio.field.value = 'x';
        t.studio.click('data-vp-action', 'check');
        assert.equal(s.phase, 'ask');
        assert.equal(s.tries, 1);
        assert.match(t.studio.hostEl.innerHTML, /data-result="retry"/);
        t.studio.click('data-vp-action', 'check');
        assert.equal(s.phase, 'reveal');
        assert.equal(s.items.length, count + 1);
        assert.equal(s.items[count].form.id, item.form.id);
        assert.equal(s.items[count].again, true);
        assert.equal(t.posted.map((p) => p.result).join(','), 'miss,miss');
    });
});

describe('plan page: verb workshop opens as a studio', () => {
    const ui = require('../shared/ui/plan-render');

    function lexiconBody(lemma) {
        const fx = fixture();
        return { entry: fx.lemmas[lemma] || null, lemma: fx.lemmas[lemma] || null, paradigm: fx.paradigms[lemma] || {} };
    }

    function fakeDoc(search) {
        const mount = {
            innerHTML: '', textContent: '', attrs: {},
            setAttribute(k, v) { this.attrs[k] = v; },
            addEventListener() {}, removeEventListener() {},
            querySelector: () => null, querySelectorAll: () => []
        };
        const doc = {
            location: { search, hash: '' },
            body: { classList: { list: [], add(c) { this.list.push(c); } } },
            getElementById: (id) => (id === 'plan-root' ? mount : null),
            querySelectorAll: () => []
        };
        return { doc, mount };
    }

    function fakeFetch(plan) {
        return (url) => {
            let body = null;
            if (url === '/api/plans/' + plan.id + '/published') body = plan;
            else if (url === '/api/me') body = { username: 't' };
            else if (url.indexOf('/api/lexicon/') === 0) body = lexiconBody(decodeURIComponent(url.slice(13)));
            return Promise.resolve({ ok: !!body, json: () => Promise.resolve(body || {}) });
        };
    }

    it('paints Look / Practise / Use for a verb-workshop lesson', async () => {
        globalThis.HebrewVerbPractice = VP;
        try {
            const plan = seed.PLANS[0];
            const lesson = plan.units[0].lessons[0];
            const { doc, mount } = fakeDoc('?id=' + plan.id + '&lesson=' + lesson.id);
            await ui.start(doc, fakeFetch(plan));
            assert.match(mount.innerHTML, /data-verb-studio/);
            for (const tab of ['look', 'practice', 'use']) assert.match(mount.innerHTML, new RegExp('data-vs-tab="' + tab + '"'));
            assert.match(mount.innerHTML, /class="vl-explainer"/);
            assert.equal(/-3fs/.test(mount.innerHTML), false);
            assert.equal(mount.attrs['data-plan-ready'], '1');
            assert.deepEqual(doc.body.classList.list, ['verb-studio-page']);
        } finally {
            delete globalThis.HebrewVerbPractice;
        }
    });

    it('keeps the screens for any other recipe', async () => {
        globalThis.HebrewVerbPractice = VP;
        try {
            const plan = JSON.parse(JSON.stringify(seed.PLANS[0]));
            plan.units[0].lessons[0].recipe = 'reading';
            const { doc, mount } = fakeDoc('?id=' + plan.id + '&lesson=' + plan.units[0].lessons[0].id);
            await ui.start(doc, fakeFetch(plan));
            assert.equal(/data-verb-studio/.test(mount.innerHTML), false);
            assert.match(mount.innerHTML, /data-screen=/);
        } finally {
            delete globalThis.HebrewVerbPractice;
        }
    });
});

describe('/verbs/ route', () => {
    it('is a registered screen with a nav link and a server path', () => {
        const routes = require('../shared/app-routes.js');
        const route = routes.find((r) => r.id === 'verbs');
        assert.ok(route);
        assert.equal(route.path, '/verbs/');
        assert.equal(route.view, 'HebrewVerbsPage');
        assert.ok(route.scripts.includes('/shared/verb-practice.js'));
        for (const src of route.scripts) assert.ok(fs.existsSync(path.join(ROOT, src)), src);
        assert.ok(require('../server/spa.js').APP_PATHS.includes('/verbs/'));
        const shell = fs.readFileSync(path.join(ROOT, 'shared', 'app-shell.js'), 'utf8');
        assert.match(shell, /id: 'verbs', href: '\/verbs\/'/);
    });

    it('lists only verb-workshop lessons with slug ids', () => {
        const page = require('../shared/verbs-page.js');
        const rows = page.verbLessons([
            { planId: 'p', lessonId: 'a', recipe: 'verb-workshop', title: 'A' },
            { planId: 'p', lessonId: 'b', recipe: 'guide', title: 'B' },
            { planId: '../x', lessonId: 'c', recipe: 'verb-workshop', title: 'C' }
        ]);
        assert.deepEqual(rows.map((r) => r.lessonId), ['a']);
        assert.equal(page.lessonHref(rows[0]), '/shared/plan/?id=p&lesson=a');
    });
});
