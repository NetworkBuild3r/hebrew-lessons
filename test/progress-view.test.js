/**
 * Progress view — DOM render from metrics fixtures.
 * Provenance: INIT-002/SPEC-015
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

const GAMIFICATION_RE = /\b(streak|points|badge|leaderboard)s?\b/i;
const GAMIFICATION_CLASS_RE = /\b(streak|points|badge|leaderboard)[-_]?\w*/i;

function loadView() {
    const sandbox = {
        console,
        document: { readyState: 'complete', getElementById() { return null; } },
        window: {},
        globalThis: {}
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'progress-page.js'), 'utf8'),
        sandbox
    );
    return sandbox;
}

function el() {
    return { innerHTML: '' };
}

function fixtureReady() {
    return {
        stages: {
            he_en: { new: 2, learning: 3, longTerm: 1 },
            en_he: { new: 1, learning: 4, longTerm: 2 }
        },
        accuracy: {
            he_en: { pass: 8, miss: 2, pct: 0.8 },
            en_he: { pass: 7, miss: 3, pct: 0.7 }
        }
    };
}

function fixtureLowData() {
    return {
        stages: {
            he_en: { new: 2, learning: 1, longTerm: 0 },
            en_he: { new: 1, learning: 0, longTerm: 0 }
        },
        accuracy: {
            he_en: { pass: 3, miss: 2, pct: null },
            en_he: { pass: 1, miss: 0, pct: null }
        }
    };
}

function fixtureEmpty() {
    return {
        stages: {
            he_en: { new: 0, learning: 0, longTerm: 0 },
            en_he: { new: 0, learning: 0, longTerm: 0 }
        },
        accuracy: {
            he_en: { pass: 0, miss: 0, pct: null },
            en_he: { pass: 0, miss: 0, pct: null }
        }
    };
}

function assertNoGamification(html) {
    assert.doesNotMatch(html, GAMIFICATION_RE);
    assert.doesNotMatch(html, GAMIFICATION_CLASS_RE);
}

describe('progress view (SPEC-015)', () => {
    let View;

    before(() => {
        const win = loadView();
        View = win.HebrewProgressView;
        assert.ok(View && typeof View.render === 'function');
    });

    it('renders loading state', () => {
        const host = el();
        View.render(host, { state: 'loading' });
        assert.match(host.innerHTML, /data-progress-state="loading"/);
        assert.match(host.innerHTML, /טוען/);
        assert.match(host.innerHTML, /Loading/);
        assertNoGamification(host.innerHTML);
    });

    it('renders error state', () => {
        const host = el();
        View.render(host, { state: 'error', message: 'boom <x>' });
        assert.match(host.innerHTML, /data-progress-state="error"/);
        assert.match(host.innerHTML, /שגיאה/);
        assert.match(host.innerHTML, /Could not load progress/);
        assert.match(host.innerHTML, /boom &lt;x&gt;/);
        assert.doesNotMatch(host.innerHTML, /boom <x>/);
        assertNoGamification(host.innerHTML);
    });

    it('ac-3: empty state points to Today and Lessons', () => {
        const host = el();
        View.render(host, fixtureEmpty());
        assert.match(host.innerHTML, /data-progress-state="empty"/);
        assert.match(host.innerHTML, /עדיין אין חזרות/);
        assert.match(host.innerHTML, /No reviews yet/);
        assert.match(host.innerHTML, /href="\/"/);
        assert.match(host.innerHTML, /היום/);
        assert.match(host.innerHTML, />Today</);
        assert.match(host.innerHTML, /href="\/lessons\/"/);
        assert.match(host.innerHTML, /שיעורים/);
        assert.match(host.innerHTML, />Lessons</);
        assertNoGamification(host.innerHTML);
    });

    it('ac-1: stage bars show exact counts with text equivalents', () => {
        const host = el();
        const m = fixtureReady();
        View.render(host, m);
        assert.match(host.innerHTML, /data-progress-state="ready"/);

        const he = host.innerHTML.match(
            /data-direction="he_en"[\s\S]*?data-direction="en_he"|data-direction="he_en"[\s\S]*?<\/section>/
        );
        assert.ok(he, 'he_en section present');

        function countsFor(dir) {
            const re = new RegExp(
                'data-direction="' + dir + '"[\\s\\S]*?<\\/section>',
                'm'
            );
            const block = host.innerHTML.match(re);
            assert.ok(block, dir + ' block');
            const text = block[0];
            const stages = m.stages[dir];
            assert.match(text, new RegExp('data-stage="new"[^>]*data-count="' + stages.new + '"'));
            assert.match(text, new RegExp('data-stage="learning"[^>]*data-count="' + stages.learning + '"'));
            assert.match(text, new RegExp('data-stage="longTerm"[^>]*data-count="' + stages.longTerm + '"'));
            assert.match(text, new RegExp(
                'data-stage="new"[\\s\\S]*?<span class="progress-count-n">' + stages.new + '</span>'
            ));
            assert.match(text, new RegExp(
                'data-stage="learning"[\\s\\S]*?<span class="progress-count-n">' + stages.learning + '</span>'
            ));
            assert.match(text, new RegExp(
                'data-stage="longTerm"[\\s\\S]*?<span class="progress-count-n">' + stages.longTerm + '</span>'
            ));
        }
        countsFor('he_en');
        countsFor('en_he');

        assert.match(host.innerHTML, /חדש/);
        assert.match(host.innerHTML, />New</);
        assert.match(host.innerHTML, /בלמידה/);
        assert.match(host.innerHTML, />Learning</);
        assert.match(host.innerHTML, /לטווח ארוך/);
        assert.match(host.innerHTML, />Long-term</);
        assert.match(host.innerHTML, /החזרה הבאה בעוד שבועיים או יותר/);
        assert.match(host.innerHTML, /next review 2\+ weeks away/);
        assertNoGamification(host.innerHTML);
    });

    it('ac-2: accuracy shows pct when ready and low-data sentence when pct null', () => {
        const readyHost = el();
        View.render(readyHost, fixtureReady());
        assert.match(readyHost.innerHTML, /data-pct="0\.8"/);
        assert.match(readyHost.innerHTML, />80%</);
        assert.match(readyHost.innerHTML, /data-attempts="10"/);
        assert.match(readyHost.innerHTML, /data-pct="0\.7"/);
        assert.match(readyHost.innerHTML, />70%/);
        assert.doesNotMatch(readyHost.innerHTML, /עדיין אין מספיק תשובות/);

        const lowHost = el();
        View.render(lowHost, fixtureLowData());
        assert.match(lowHost.innerHTML, /data-progress-state="low-data"/);
        assert.match(lowHost.innerHTML, /עדיין אין מספיק תשובות/);
        assert.match(lowHost.innerHTML, /Not enough answers yet/);
        assert.match(lowHost.innerHTML, /data-accuracy-state="low-data"/);
        assert.doesNotMatch(lowHost.innerHTML, /data-pct=/);
        assert.match(lowHost.innerHTML, /data-attempts="5"/);
        assert.match(lowHost.innerHTML, /data-attempts="1"/);
        assertNoGamification(lowHost.innerHTML);
    });

    it('ac-5: Hebrew with English labels for directions and why lines', () => {
        const host = el();
        View.render(host, fixtureReady());
        assert.match(host.innerHTML, /עברית → אנגלית/);
        assert.match(host.innerHTML, /Hebrew → English/);
        assert.match(host.innerHTML, /אנגלית → עברית/);
        assert.match(host.innerHTML, /English → Hebrew/);
        assert.match(host.innerHTML, /מילים עוברות לטווח ארוך אחרי שנזכרים בהן בימים נפרדים/);
        assert.match(host.innerHTML, /Words move to long-term after you recall them on separate days/);
        assert.match(host.innerHTML, /עברית לאנגלית ואנגלית לעברית בונות ידע שונה/);
        assert.match(host.innerHTML, /Hebrew to English and English to Hebrew build different knowledge/);
    });

    it('ac-4: no streak, points, badge, or leaderboard in rendered HTML or source', () => {
        const host = el();
        View.render(host, fixtureReady());
        assertNoGamification(host.innerHTML);
        View.render(host, fixtureLowData());
        assertNoGamification(host.innerHTML);
        View.render(host, fixtureEmpty());
        assertNoGamification(host.innerHTML);
        View.render(host, { state: 'loading' });
        assertNoGamification(host.innerHTML);
        View.render(host, { state: 'error' });
        assertNoGamification(host.innerHTML);

        const src = fs.readFileSync(path.join(root, 'shared', 'progress-page.js'), 'utf8');
        const html = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8')
            + fs.readFileSync(path.join(root, 'shared', 'app-routes.js'), 'utf8');
        assert.doesNotMatch(src, GAMIFICATION_RE);
        assert.doesNotMatch(html, GAMIFICATION_RE);
        assert.doesNotMatch(src, /class=["'][^"']*\b(streak|points|badge|leaderboard)/i);
        assert.doesNotMatch(html, /class=["'][^"']*\b(streak|points|badge|leaderboard)/i);
    });

    it('page mounts in the app shell and does not recompute metrics', () => {
        const html = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8');
        assert.match(html, /shared\/app-shell\.js/);
        assert.match(html, /shared\/api\.js/);
        const progress = require('../shared/app-routes.js').find((r) => r.id === 'progress');
        assert.deepEqual(progress.scripts, ['/shared/progress-page.js']);
        assert.equal(progress.view, 'HebrewProgressView');
        assert.equal(typeof View.mount, 'function');
        assert.equal(typeof View.unmount, 'function');
        const src = fs.readFileSync(path.join(root, 'shared', 'progress-page.js'), 'utf8');
        assert.match(src, /\/api\/me\/metrics/);
        assert.doesNotMatch(src, /longTerm\s*\+/);
        assert.doesNotMatch(src, /pass\s*\/\s*attempts/);
    });
});
