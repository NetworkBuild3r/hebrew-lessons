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
    'shared/admin/plan-ui.js',
    'shared/admin/plan-hints.js',
    'shared/admin/plan-widgets.js',
    'shared/admin/plan-form.js',
    'shared/admin/plan-list.js',
    'shared/admin/plan-issues.js',
    'shared/admin/plan-history.js',
    'shared/admin/plan-preview.js',
    'shared/admin/plan-builder.js'
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
        plansTruncated: opts.plansTruncated === true
    };
    const sandbox = {
        console,
        setTimeout,
        clearTimeout,
        Promise,
        URL,
        confirm() { return true; },
        location: { href: 'http://127.0.0.1/admin/', hash: '#plans', pathname: '/admin/' },
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
        if (method === 'GET' && url.indexOf(base + '/parts') === 0) {
            const q = /path=([^&]*)/.exec(url);
            const part = q ? nodeAt(server.plan, decodeURIComponent(q[1])) : server.plan;
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
        if (method === 'PATCH' && url === base + '/fields') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            const node = nodeAt(server.plan, body.path);
            if (!node) return json(404, { error: 'Not found', code: 'not_found' });
            node[body.field] = body.value;
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
        if (method === 'POST' && url === base + '/assign') {
            if (body.expectedRevision !== server.revision) return json(409, { error: 'revision_conflict', revision: server.revision });
            server.plan.audience = body.audience;
            server.plan.assigneeId = body.assigneeId || null;
            server.revision += 1;
            server.plan.revision = server.revision;
            return json(200, { plan: clone(server.plan), revision: server.revision, status: 'draft', validation: { ok: true, blocks: [], warnings: [], informs: [] } });
        }
        if (method === 'POST' && url === '/api/plans') {
            return json(200, { plan: { id: body.id }, revision: 1, status: 'draft', validation: { ok: true } });
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
