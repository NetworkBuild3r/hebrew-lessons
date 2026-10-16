/**
 * A browser-like sandbox for the plan screens (INIT-008/SPEC-013 to SPEC-015): every admin script
 * loaded into a `vm` context over the fake DOM, and a small stand-in for the plan routes that records
 * each request. Hand-written text only. A browser run is test/chrome-plan-editor-smoke.js.
 *
 *   const env = loadPanels({ plan, revision, issues, revisions, diff, editState, users, plans });
 *   env.calls              every request: { method, url, body }
 *   env.server             the mutable state the routes answer from
 *   env.on(method, re, fn) override one route: fn({ url, body, match }) -> { status, data }
 *   env.panel()            a fresh fake element to mount into
 *   env.W                  the sandbox (W.HebrewPlanBuilder, W.HebrewPlanList, ...)
 */
'use strict';

const fs = require('fs');
const path = require('path');
const vm = require('vm');
const dom = require('./fake-dom');
const { SCHEMA, samplePlan } = require('./plan-editor-fixtures');

const ROOT = path.resolve(__dirname, '..', '..');
const SCRIPTS = [
    'shared/player/utils.js',
    'shared/api.js',
    'shared/ui/blocks/explainer.js',
    'shared/ui/blocks/conjugation-table.js',
    'shared/ui/blocks/personal-question.js',
    'shared/ui/blocks/verse-reading.js',
    'shared/ui/blocks/letter-intro.js',
    'shared/ui/plan-render.js',
    'shared/admin/activity-map.js',
    'shared/admin/plan-ui.js',
    'shared/admin/plan-hints.js',
    'shared/admin/plan-widgets.js',
    'shared/admin/plan-form.js',
    'shared/admin/plan-list.js',
    'shared/admin/plan-issues.js',
    'shared/admin/plan-history.js',
    'shared/admin/plan-preview.js',
    'shared/admin/plan-builder.js',
    'shared/admin/editor-shell.js'
];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function clone(value) {
    return JSON.parse(JSON.stringify(value));
}

function nodeAt(plan, idPath) {
    const segs = String(idPath || '').split('/').filter(Boolean);
    let node = plan;
    for (let i = 0; i < segs.length; i += 2) {
        const list = node[segs[i]];
        node = Array.isArray(list) ? list.find((item) => item.id === segs[i + 1]) : null;
        if (!node) return null;
    }
    return node;
}

function loadPanels(options) {
    const opts = options || {};
    const calls = [];
    const overrides = [];
    const hashes = [];
    const server = {
        plan: opts.plan || (() => { const p = samplePlan(); p.id = 'sample'; p.revision = 4; return p; })(),
        revision: opts.revision || 4,
        status: 'draft',
        editState: opts.editState || { advisory: true, lastHumanEdit: null, humanEdited: [], more: 0 },
        issues: opts.issues || { revision: 4, status: 'draft', counts: { block: 0, warn: 0, inform: 0 }, issues: [], truncated: false },
        revisions: opts.revisions || [],
        revisionsTruncated: opts.revisionsTruncated === true,
        diff: opts.diff || { from: { ref: 1, revision: 1 }, to: { ref: 'working', revision: 4 }, entries: [], truncated: false },
        users: opts.users || [],
        plans: opts.plans || [],
        plansById: opts.plansById || {},
        plansTruncated: opts.plansTruncated === true
    };
    if (!server.plansById[server.plan.id]) server.plansById[server.plan.id] = server.plan;
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        Promise,
        URL,
        confirm() { return true; },
        location: { href: 'http://127.0.0.1/editor/', hash: '#plans', pathname: '/editor/' },
        document: dom.makeDocument(),
        fetch(url, init) {
            const method = String((init && init.method) || 'GET').toUpperCase();
            const body = init && init.body ? JSON.parse(init.body) : null;
            calls.push({ method, url: String(url), body, signal: !!(init && init.signal) });
            // a route may answer with a promise, to hold one answer back
            return Promise.resolve(route(method, String(url), body)).then((reply) => ({
                ok: reply.status >= 200 && reply.status < 300,
                status: reply.status,
                headers: { get: (name) => (String(name).toLowerCase() === 'content-type' ? 'application/json' : null) },
                json() { return Promise.resolve(reply.data); },
                text() { return Promise.resolve(JSON.stringify(reply.data)); }
            }));
        }
    };
    sandbox.window = sandbox;
    sandbox.globalThis = sandbox;
    sandbox.HebrewAdminShell = { setHash(route) { hashes.push(route); } };
    vm.createContext(sandbox);
    SCRIPTS.forEach((file) => vm.runInContext(fs.readFileSync(path.join(ROOT, file), 'utf8'), sandbox, { filename: file }));
    if (sandbox.HebrewEditorShell && typeof sandbox.HebrewEditorShell.setHash === 'function') {
        const editorSetHash = sandbox.HebrewEditorShell.setHash;
        sandbox.HebrewEditorShell.setHash = function (route) {
            hashes.push(route);
            return editorSetHash.call(this, route);
        };
    } else {
        sandbox.HebrewEditorShell = { setHash(route) { hashes.push(route); } };
    }

    const json = (status, data) => ({ status, data });

    function route(method, url, body) {
        for (const o of overrides) {
            if (o.method === method) {
                const match = o.re.exec(url);
                if (match) return o.fn({ url, body, match });
            }
        }
        const base = '/api/plans/sample';
        if (method === 'GET' && url === '/api/plan-schema') return json(200, SCHEMA);
        if (method === 'GET' && url === '/api/plans') return json(200, { plans: server.plans, truncated: server.plansTruncated });
        if (method === 'GET' && url === '/api/admin/users') return json(200, { users: server.users });
        const partsMatch = method === 'GET' && /^\/api\/plans\/([^/]+)\/parts/.exec(url);
        if (partsMatch) {
            const planId = decodeURIComponent(partsMatch[1]);
            const planDoc = planId === 'sample' || planId === server.plan.id
                ? server.plan
                : (server.plansById[planId] || null);
            if (!planDoc) return json(404, { error: 'Not found', code: 'not_found' });
            const q = /path=([^&]*)/.exec(url);
            const part = q ? nodeAt(planDoc, decodeURIComponent(q[1])) : planDoc;
            return json(200, { part: clone(part), revision: server.revision, editState: server.editState });
        }
        if (method === 'GET' && url.indexOf(base + '/issues') === 0) return json(200, server.issues);
        if (method === 'GET' && /\/revisions\/\d+$/.test(url)) {
            const n = Number(/(\d+)$/.exec(url)[1]);
            const row = server.revisions.find((r) => r.revision === n);
            return row ? json(200, Object.assign({}, row, { document: clone(server.plan) })) : json(404, { error: 'Not found', code: 'not_found' });
        }
        if (method === 'GET' && url.indexOf(base + '/revisions') === 0) {
            const before = /beforeRevision=(\d+)/.exec(url);
            const kind = /actorKind=(\w+)/.exec(url);
            let rows = server.revisions;
            if (kind) rows = rows.filter((r) => r.actorKind === kind[1]);
            if (before) rows = rows.filter((r) => r.revision < Number(before[1]));
            return json(200, { revisions: rows, truncated: !before && server.revisionsTruncated });
        }
        if (method === 'GET' && url.indexOf(base + '/diff') === 0) return json(200, server.diff);
        if (method === 'POST' && url === base + '/lessons') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const unit = nodeAt(server.plan, body.path);
            if (!unit || !Array.isArray(unit.lessons)) return json(404, { error: 'Not found', code: 'not_found' });
            const at = body.index == null ? unit.lessons.length : body.index;
            unit.lessons.splice(at, 0, clone(body.lesson));
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'POST' && url === base + '/lessons/move') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const found = (() => {
                const parts = String(body.path || '').split('/');
                if (parts.length !== 4 || parts[0] !== 'units' || parts[2] !== 'lessons') return null;
                const unit = (server.plan.units || []).find((u) => u && u.id === parts[1]);
                if (!unit || !Array.isArray(unit.lessons)) return null;
                const index = unit.lessons.findIndex((l) => l && l.id === parts[3]);
                if (index < 0) return null;
                return { list: unit.lessons, index: index };
            })();
            if (!found) return json(404, { error: 'Not found', code: 'not_found' });
            const item = found.list.splice(found.index, 1)[0];
            const at = Math.max(0, Math.min(body.index, found.list.length));
            found.list.splice(at, 0, item);
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'DELETE' && url === base + '/lessons') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const parts = String(body.path || '').split('/');
            const unit = (server.plan.units || []).find((u) => u && u.id === parts[1]);
            if (!unit || !Array.isArray(unit.lessons)) return json(404, { error: 'Not found', code: 'not_found' });
            if (unit.lessons.length <= 1) return json(400, { error: 'invalid_op' });
            const index = unit.lessons.findIndex((l) => l && l.id === parts[3]);
            if (index < 0) return json(404, { error: 'Not found', code: 'not_found' });
            unit.lessons.splice(index, 1);
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'PATCH' && url === base + '/fields') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const node = nodeAt(server.plan, body.path);
            if (!node) return json(404, { error: 'Not found', code: 'not_found' });
            node[body.field] = body.value;
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'POST' && url === base + '/blocks') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const lesson = nodeAt(server.plan, body.path);
            if (!lesson || !Array.isArray(lesson.blocks)) return json(404, { error: 'Not found', code: 'not_found' });
            const at = body.index == null ? lesson.blocks.length : body.index;
            lesson.blocks.splice(at, 0, clone(body.block));
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'POST' && /\/blocks\/move$/.test(url)) {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const parts = String(body.path || '').split('/');
            if (parts.length !== 6 || parts[0] !== 'units' || parts[2] !== 'lessons' || parts[4] !== 'blocks') {
                return json(404, { error: 'Not found', code: 'not_found' });
            }
            const lesson = nodeAt(server.plan, parts.slice(0, 4).join('/'));
            if (!lesson || !Array.isArray(lesson.blocks)) return json(404, { error: 'Not found', code: 'not_found' });
            const from = lesson.blocks.findIndex((b) => b && b.id === parts[5]);
            if (from < 0) return json(404, { error: 'Not found', code: 'not_found' });
            const item = lesson.blocks.splice(from, 1)[0];
            const at = Math.max(0, Math.min(body.index, lesson.blocks.length));
            lesson.blocks.splice(at, 0, item);
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'DELETE' && url === base + '/parts') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const parts = String(body.path || '').split('/');
            if (parts.length !== 6 || parts[4] !== 'blocks') return json(404, { error: 'Not found', code: 'not_found' });
            const lesson = nodeAt(server.plan, parts.slice(0, 4).join('/'));
            if (!lesson || !Array.isArray(lesson.blocks)) return json(404, { error: 'Not found', code: 'not_found' });
            const index = lesson.blocks.findIndex((b) => b && b.id === parts[5]);
            if (index < 0) return json(404, { error: 'Not found', code: 'not_found' });
            lesson.blocks.splice(index, 1);
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'POST' && url === base + '/restore') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] }, restoredFrom: body.fromRevision });
        }
        if (method === 'POST' && url === base + '/publish') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            server.revision += 1;
            server.plan.revision = server.revision;
            server.plan.status = 'published';
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'published', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        const assignMatch = method === 'POST' && /^\/api\/plans\/([^/]+)\/assign$/.exec(url);
        if (assignMatch) {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            server.plan.audience = body.audience;
            server.plan.assigneeId = body.assigneeId || null;
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'POST' && url === '/api/plans') {
            const id = body && typeof body.id === 'string' && body.id
                ? body.id
                : '11111111-1111-4111-8111-111111111111';
            // New plan starts at revision 1 so a follow-up assign can use that revision.
            server.revision = 1;
            server.plan.audience = 'shared';
            server.plan.assigneeId = null;
            if (body && body.title) server.plan.title = body.title;
            return json(200, {
                plan: { id: id, title: body && body.title, audience: 'shared', assigneeId: null },
                revision: 1,
                status: 'draft',
                validation: { ok: true }
            });
        }
        return json(404, { error: 'Not found', code: 'not_found' });
    }

    return {
        W: sandbox,
        calls,
        hashes,
        server,
        on(method, re, fn) { overrides.push({ method, re, fn }); },
        panel() { return sandbox.document.createElement('div'); },
        callsOf(method, re) { return calls.filter((c) => c.method === method && re.test(c.url)); },
        sleep,
        json
    };
}

/** Every descendant (and the node itself) matching `fn`. */
const all = (node, fn) => dom.collect(node, fn);
const byAttr = (node, name, value) => all(node, (n) => n.hasAttribute && n.hasAttribute(name)
    && (value === undefined || n.getAttribute(name) === value));

module.exports = { loadPanels, all, byAttr, sleep, clone, nodeAt };
