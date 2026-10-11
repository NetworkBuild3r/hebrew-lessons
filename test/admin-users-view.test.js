/**
 * Admin users view — DOM helpers with fake HebrewApi (SPEC-016).
 * Provenance: INIT-002/SPEC-016
 */
'use strict';

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function loadAdminUsers() {
    const sandbox = {
        console,
        document: {
            readyState: 'complete',
            getElementById() { return null; },
            activeElement: null,
            addEventListener() {}
        },
        location: { href: 'http://127.0.0.1/admin/', hash: '#users', pathname: '/admin/' },
        window: {},
        globalThis: {},
        HEBREW_VOCAB_INDEX: [
            { key: 'שָׁלוֹם', word: 'שָׁלוֹם', meaning: 'hello', audio: '' },
            {
                key: 'evil',
                word: '<img src=x onerror=alert(1)>',
                meaning: '<b>bad</b>',
                audio: ''
            }
        ],
        HEBREW_WORD_AUDIO: { 'שָׁלוֹם': '../_audio/words/a.mp3' }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    vm.createContext(sandbox);
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'player', 'utils.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'admin', 'users.js'), 'utf8'),
        sandbox
    );
    vm.runInContext(
        fs.readFileSync(path.join(root, 'shared', 'admin', 'admin-shell.js'), 'utf8'),
        sandbox
    );
    return sandbox;
}

function makeBody(initialHtml) {
    let html = initialHtml || '';
    const body = {
        get innerHTML() { return html; },
        set innerHTML(v) { html = String(v); },
        insertAdjacentHTML(pos, chunk) {
            if (pos === 'beforeend') html += String(chunk);
            else html = String(chunk) + html;
        },
        querySelector() { return null; }
    };
    return body;
}

function makePanel(tableHtml) {
    const body = makeBody(tableHtml);
    let guardHidden = true;
    let guardText = '';
    const guard = {
        get hidden() { return guardHidden; },
        set hidden(v) { guardHidden = !!v; },
        get textContent() { return guardText; },
        set textContent(v) { guardText = String(v == null ? '' : v); },
        setAttribute(name, value) {
            if (name === 'hidden') guardHidden = true;
        },
        removeAttribute(name) {
            if (name === 'hidden') guardHidden = false;
        }
    };
    return {
        querySelector(sel) {
            if (sel === '[data-admin-users-body]') return body;
            if (sel === '[data-admin-guard-error]') return guard;
            return null;
        },
        _body: body,
        _guard: guard
    };
}

function makeHistoryHost(initialRows) {
    const body = makeBody(initialRows || '');
    let btnHidden = false;
    const btn = {
        get hidden() { return btnHidden; },
        set hidden(v) { btnHidden = !!v; },
        setAttribute(name) {
            if (name === 'hidden') btnHidden = true;
        },
        removeAttribute(name) {
            if (name === 'hidden') btnHidden = false;
        }
    };
    return {
        querySelector(sel) {
            if (sel === '[data-admin-history-body]') return body;
            if (sel === '[data-admin-load-older]') return btn;
            return null;
        },
        _body: body,
        _btn: btn
    };
}

describe('admin users view (SPEC-016)', () => {
    let Users;
    let Shell;

    before(() => {
        const win = loadAdminUsers();
        Users = win.HebrewAdminUsers;
        Shell = win.HebrewAdminShell;
        assert.ok(Users && typeof Users.renderUsersTable === 'function');
        assert.ok(Shell && typeof Shell.parseHash === 'function');
    });

    it('escapes usernames, display names, and trouble-word text (ac-5)', () => {
        const dirty = '<img src=x onerror=alert(1)>';
        const html = Users.renderUsersTable([
            {
                id: 9,
                username: dirty,
                display_name: dirty,
                role: 'user',
                status: 'active',
                last_sign_in: null
            }
        ]);
        assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.doesNotMatch(html, /<img src=x onerror=alert\(1\)>/);
        assert.equal(Users.escapeHtml(dirty), '&lt;img src=x onerror=alert(1)&gt;');

        const trouble = Users.renderTroubleWords([
            { word: dirty, direction: 'he_en', misses: 3, lapses: 1 }
        ]);
        assert.match(trouble, /&lt;img src=x onerror=alert\(1\)&gt;/);
        assert.doesNotMatch(trouble, /<img src=x onerror=alert\(1\)>/);
        assert.match(trouble, /&lt;b&gt;bad&lt;\/b&gt;/);
    });

    it('password fields use autocomplete="new-password" (ac-6)', () => {
        const createHtml = Users.createDialogHtml();
        const resetHtml = Users.resetDialogHtml();
        assert.match(createHtml, /id="admin-create-password"[^>]*autocomplete="new-password"/);
        assert.match(resetHtml, /id="admin-reset-password"[^>]*autocomplete="new-password"/);
        assert.match(createHtml, /לפחות 10 תווים/);
        assert.match(createHtml, /At least 10 characters/);
    });

    it('last-admin 409 leaves the table unchanged and shows the server message (ac-2)', () => {
        const users = [
            {
                id: 1,
                username: 'admin',
                display_name: 'Admin',
                role: 'admin',
                status: 'active',
                last_sign_in: null
            }
        ];
        const rowHtml = Users.renderUserRow(users[0]);
        const panel = makePanel(rowHtml);
        const state = { users: Users.usersSnapshot(users) };
        const beforeHtml = panel._body.innerHTML;

        const ok = Users.applyPatchResult(panel, state, Users.usersSnapshot(users), beforeHtml, {
            ok: false,
            status: 409,
            error: 'Cannot demote or disable the last active admin',
            code: 'last_admin'
        });

        assert.equal(ok, false);
        assert.equal(panel._body.innerHTML, beforeHtml);
        assert.match(panel._body.innerHTML, /data-role="admin"/);
        assert.equal(panel._guard.hidden, false);
        assert.match(panel._guard.textContent, /Cannot demote or disable the last active admin/);
        assert.equal(state.users[0].role, 'admin');
    });

    it('load older appends without duplicates and hides at the end (ac-3)', () => {
        const host = makeHistoryHost('');
        const state = { seenIds: Object.create(null), nextBefore: 'cursor-1' };
        const limit = 2;

        const page1 = {
            events: [
                {
                    id: 'e1', word: 'א', direction: 'he_en', result: 'pass',
                    at: '2024-01-02T00:00:00.000Z', slug: 'l', activity: 'words', lessonTitle: 'L'
                },
                {
                    id: 'e2', word: 'ב', direction: 'he_en', result: 'miss',
                    at: '2024-01-01T00:00:00.000Z', slug: 'l', activity: 'words', lessonTitle: 'L'
                }
            ],
            nextBefore: 'cursor-2'
        };
        const r1 = Users.applyHistoryPage(state, host, page1, limit);
        assert.equal(r1.appended, 2);
        assert.equal(r1.more, true);
        assert.equal(host._btn.hidden, false);
        assert.match(host._body.innerHTML, /data-event-id="e1"/);
        assert.match(host._body.innerHTML, /data-event-id="e2"/);

        const page2dup = {
            events: [
                page1.events[1],
                {
                    id: 'e3', word: 'ג', direction: 'en_he', result: 'pass',
                    at: '2023-12-31T00:00:00.000Z', slug: 'l', activity: 'match', lessonTitle: 'L'
                }
            ],
            nextBefore: 'cursor-3'
        };
        const r2 = Users.applyHistoryPage(state, host, page2dup, limit);
        assert.equal(r2.appended, 1);
        assert.equal(r2.more, true);
        const e2count = (host._body.innerHTML.match(/data-event-id="e2"/g) || []).length;
        assert.equal(e2count, 1);
        assert.match(host._body.innerHTML, /data-event-id="e3"/);

        const page3 = {
            events: [
                {
                    id: 'e4', word: 'ד', direction: 'he_en', result: 'pass',
                    at: '2023-12-30T00:00:00.000Z', slug: 'l', activity: 'words', lessonTitle: 'L'
                }
            ],
            nextBefore: 'cursor-4'
        };
        const r3 = Users.applyHistoryPage(state, host, page3, limit);
        assert.equal(r3.appended, 1);
        assert.equal(r3.more, false);
        assert.equal(host._btn.hidden, true);

        const pageEmpty = { events: [], nextBefore: null };
        const r4 = Users.applyHistoryPage(state, host, pageEmpty, limit);
        assert.equal(r4.appended, 0);
        assert.equal(r4.more, false);
        assert.equal(host._btn.hidden, true);
    });

    it('admin shell hash routes and lessons placeholder', () => {
        const users = Shell.parseHash('#users');
        assert.equal(users.view, 'users');
        assert.equal(users.userId, null);
        const lessons = Shell.parseHash('#lessons');
        assert.equal(lessons.view, 'lessons');
        assert.equal(lessons.userId, null);
        const user = Shell.parseHash('#user/42');
        assert.equal(user.view, 'user');
        assert.equal(user.userId, '42');
        const placeholder = Shell.renderLessonsPlaceholder();
        assert.match(placeholder, /עורך השיעורים עדיין לא בעמוד הזה/);
        assert.match(placeholder, /lesson editor is not on this page yet/i);
        const tabs = Shell.renderTabs('users');
        assert.match(tabs, /משתמשים/);
        assert.match(tabs, /Users/);
        assert.match(tabs, /שיעורים/);
        assert.match(tabs, /Lessons/);
    });

    it('admin page does not fetch catalog files (script tags only)', () => {
        const admin = require('../shared/app-routes.js').find((r) => r.id === 'admin');
        assert.equal(admin.adminOnly, true);
        const scripts = admin.scripts.join('\n');
        assert.match(scripts, /shared\/admin\/users\.js/);
        assert.match(scripts, /shared\/admin\/admin-shell\.js/);
        assert.match(scripts, /vocab-index\.js/);
        assert.match(scripts, /word-audio-catalog\.js/);
        const shell = fs.readFileSync(path.join(root, 'shared', 'app.html'), 'utf8');
        assert.doesNotMatch(shell, /\bfetch\s*\(/);
        const usersJs = fs.readFileSync(path.join(root, 'shared', 'admin', 'users.js'), 'utf8');
        assert.doesNotMatch(usersJs, /\bfetch\s*\(/);
        assert.match(usersJs, /HebrewApi/);
    });

    it('AI planning confirm quotes the disclosure and does not change state before a 200 (SPEC-017)', () => {
        const disclosure = 'If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator\'s AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.';
        const adminSentence = 'Admins can see all your activity, including every answer';
        const dialog = Users.aiPlanningDialogHtml();
        assert.match(dialog, /data-ai-disclosure/);
        assert.ok(dialog.includes(disclosure));
        assert.equal(Users.AI_DISCLOSURE.includes(disclosure), true);

        const off = Users.renderUserRow({
            id: 4,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            status: 'active',
            last_sign_in: null,
            ai_planning: false
        });
        assert.match(off, /data-admin-action="ai-planning"/);
        assert.match(off, /aria-pressed="false"/);
        assert.doesNotMatch(off, /aria-pressed="true"/);

        const unknown = Users.renderUserRow({
            id: 5,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            status: 'active',
            last_sign_in: null
        });
        assert.match(unknown, /disabled/);
        assert.doesNotMatch(unknown, /data-admin-action="ai-planning"/);

        const users = [{
            id: 4,
            username: 'learner',
            display_name: 'Learner',
            role: 'user',
            status: 'active',
            last_sign_in: null,
            ai_planning: false
        }];
        const panel = makePanel(Users.renderUserRow(users[0]));
        const state = { users: Users.usersSnapshot(users) };
        const before = panel._body.innerHTML;

        const failed = Users.applyAiPlanningResult(panel, state, 4, {
            ok: false,
            status: 403,
            error: 'Forbidden'
        });
        assert.equal(failed, false);
        assert.equal(state.users[0].ai_planning, false);
        assert.equal(panel._body.innerHTML, before);
        assert.match(panel._body.innerHTML, /aria-pressed="false"/);
        assert.equal(panel._guard.hidden, false);
        assert.match(panel._guard.textContent, /Forbidden/);

        const succeeded = Users.applyAiPlanningResult(panel, state, 4, {
            ok: true,
            status: 200,
            data: { ai_planning: true, previous: 0, next: 1 }
        });
        assert.equal(succeeded, true);
        assert.equal(state.users[0].ai_planning, true);
        assert.match(panel._body.innerHTML, /aria-pressed="true"/);
        assert.equal(panel._guard.hidden, true);

        const ignored = Users.applyAiPlanningResult(panel, state, 4, {
            ok: true,
            status: 200,
            data: { enabled: false }
        });
        assert.equal(ignored, false);
        assert.equal(state.users[0].ai_planning, true);
        assert.match(panel._body.innerHTML, /aria-pressed="true"/);

        const shell = fs.readFileSync(path.join(root, 'shared', 'app-shell.js'), 'utf8');
        const login = fs.readFileSync(path.join(root, 'login', 'index.html'), 'utf8');
        const loginJs = fs.readFileSync(path.join(root, 'shared', 'login.js'), 'utf8');
        const progress = fs.readFileSync(path.join(root, 'shared', 'progress-page.js'), 'utf8');
        const marker = 'a summary of your progress and your username are sent to the operator';
        for (const file of [shell, login, loginJs]) {
            assert.ok(file.includes(marker));
        }
        assert.match(progress, /HebrewShell/);
        assert.match(progress, /AI_DISCLOSURE/);
        assert.ok(dialog.includes(disclosure));
        for (const file of [shell, login, loginJs]) {
            assert.ok(file.includes(adminSentence));
        }
    });
});
