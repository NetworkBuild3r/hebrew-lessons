/**
 * Plan HTTP routes. Session, role, and body checks, then one function
 * in server/plans/ops.js or publish.js. These handlers do not edit JSON.
 * Mutating CSRF and Origin checks run in server/web.js before dispatch,
 * the same gate as every other /api/ write.
 * Provenance: INIT-004/SPEC-004
 */
'use strict';

const crypto = require('crypto');
const body = require('../http/body');
const security = require('../http/security');
const ops = require('../plans/ops');
const publish = require('../plans/publish');
const create = require('../plans/create');
const revisions = require('../plans/revisions');
const issues = require('../plans/issues');
const personal = require('../plans/personal');
const validate = require('../plans/validate');
const store = require('../plans/store');
const { makeContext } = require('../plans/context');
const visibilityRepo = require('../db/repos/visibility');

const BODY_LIMIT = 1024 * 1024;

/**
 * Write routes. Tests iterate this list so a new write cannot skip the admin check.
 * @type {ReadonlyArray<{ method: string, path: string, fn: string }>}
 */
const WRITE_ROUTES = Object.freeze([
    { method: 'PUT', path: '/api/plans/:id/parts', fn: 'replacePart' },
    { method: 'POST', path: '/api/plans/:id/blocks', fn: 'insertBlock' },
    { method: 'POST', path: '/api/plans/:id/blocks/move', fn: 'moveBlock' },
    { method: 'DELETE', path: '/api/plans/:id/parts', fn: 'removeBlock' },
    { method: 'PATCH', path: '/api/plans/:id/fields', fn: 'setField' },
    { method: 'POST', path: '/api/plans/:id/publish', fn: 'publishPlan' },
    // INIT-008/SPEC-010: the three new writes. Every other new route is a read.
    { method: 'POST', path: '/api/plans', fn: 'createPlan' },
    { method: 'POST', path: '/api/plans/:id/assign', fn: 'assign' },
    { method: 'POST', path: '/api/plans/:id/restore', fn: 'restore' }
]);

/**
 * @param {unknown} err
 * @param {import('http').ServerResponse} res
 * @returns {boolean}
 */
function sendBodyError(err, res) {
    if (err && err.name === 'BodyError') {
        security.sendJsonError(res, err.status, err.message, err.code);
        return true;
    }
    return false;
}

/**
 * Unknown keys are rejected. Required keys must be present. Optional keys may be omitted.
 * @param {object} payload
 * @param {readonly string[]} required
 * @param {readonly string[]} [optional]
 * @returns {boolean}
 */
function allowedKeys(payload, required, optional) {
    const extra = optional || [];
    const keys = Object.keys(payload);
    for (const key of keys) {
        if (required.indexOf(key) === -1 && extra.indexOf(key) === -1) return false;
    }
    for (const key of required) {
        if (!Object.prototype.hasOwnProperty.call(payload, key)) return false;
    }
    return true;
}

/**
 * @param {import('http').IncomingMessage} req
 * @param {import('http').ServerResponse} res
 * @param {readonly string[]} required
 * @param {readonly string[]} [optional]
 * @returns {Promise<object|undefined>} undefined when a response was sent
 */
async function readOperation(req, res, required, optional) {
    const payload = await readBodyKeys(req, res, required, optional);
    if (!payload) return undefined;
    if (!Number.isInteger(payload.expectedRevision)) {
        security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
        return undefined;
    }
    return payload;
}

/**
 * A JSON object body with exactly the allowed keys. Unknown keys, including
 * `actor`, `by`, and `kind`, are refused. For a body with no expectedRevision.
 * @param {import('http').IncomingMessage} req
 * @param {import('http').ServerResponse} res
 * @param {readonly string[]} required
 * @param {readonly string[]} [optional]
 * @returns {Promise<object|undefined>} undefined when a response was sent
 */
async function readBodyKeys(req, res, required, optional) {
    let payload;
    try {
        payload = await body.readJson(req, { limit: BODY_LIMIT });
    } catch (err) {
        if (sendBodyError(err, res)) return undefined;
        security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
        return undefined;
    }
    if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
        security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
        return undefined;
    }
    if (!allowedKeys(payload, required, optional)) {
        security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
        return undefined;
    }
    return payload;
}

/**
 * @param {import('http').ServerResponse} res
 * @param {object|undefined} result
 */
function sendMapped(res, result) {
    if (sendFailure(res, result)) return;
    const bodyOut = {};
    if (result.part !== undefined) bodyOut.part = result.part;
    if (result.plan !== undefined) bodyOut.plan = result.plan;
    if (result.revision !== undefined) bodyOut.revision = result.revision;
    if (result.status !== undefined) bodyOut.status = result.status;
    if (result.validation !== undefined) bodyOut.validation = result.validation;
    // Advisory notes (ADR-0011 §4): what a person last changed, and what this write replaced.
    if (result.editState !== undefined) bodyOut.editState = result.editState;
    if (result.overwrote !== undefined) bodyOut.overwrote = result.overwrote;
    // A restore says which revision it came from (INIT-008/SPEC-010).
    if (result.restoredFrom !== undefined) bodyOut.restoredFrom = result.restoredFrom;
    security.sendJson(res, 200, bodyOut);
}

/**
 * Map a read function's result: a failure the same way as a write, otherwise
 * the result itself as the 200 body. The reads return only plain data.
 * @param {import('http').ServerResponse} res
 * @param {object|undefined} result
 */
function sendRead(res, result) {
    if (sendFailure(res, result)) return;
    security.sendJson(res, 200, result);
}

/**
 * A fixed identifier from a service, never caller text.
 * @param {unknown} value
 * @returns {boolean}
 */
function isErrorWord(value) {
    return typeof value === 'string' && /^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(value);
}

/**
 * Send a coded error: `{ error, code }`, plus `field` or `reason` when the
 * service named one (which input was refused, why a part could not be placed).
 * @param {import('http').ServerResponse} res
 * @param {number} status
 * @param {string} message
 * @param {string} code
 * @param {object} result
 */
function sendCoded(res, status, message, code, result) {
    const out = { error: message, code: code };
    if (isErrorWord(result.field)) out.field = result.field;
    if (isErrorWord(result.reason)) out.reason = result.reason;
    security.sendJson(res, status, out);
}

/**
 * The status mapping shared by every plan route: 409 for `revision_conflict`
 * (stored revision only), 404 for `not_found`, 400 with the rule lists for
 * `validation`, 400 with the code for any other error. No request text is echoed.
 * @param {import('http').ServerResponse} res
 * @param {object|undefined} result
 * @returns {boolean} true when a response was sent
 */
function sendFailure(res, result) {
    if (!result || typeof result !== 'object') {
        security.sendJsonError(res, 500, 'Internal error', 'internal');
        return true;
    }
    if (result.error === 'revision_conflict') {
        security.sendJson(res, 409, {
            error: 'revision_conflict',
            revision: result.revision
        });
        return true;
    }
    if (result.error === 'not_found') {
        sendCoded(res, 404, 'Not found', 'not_found', result);
        return true;
    }
    if (result.error === 'validation') {
        security.sendJson(res, 400, {
            error: 'validation',
            revision: result.revision,
            blocks: result.blocks,
            warnings: result.warnings,
            informs: result.informs
        });
        return true;
    }
    if (result.error) {
        sendCoded(res, 400, 'Invalid request', result.error, result);
        return true;
    }
    return false;
}

/**
 * @param {import('http').IncomingMessage} req
 * @returns {{ path: string }|{ error: true }}
 */
function readPartQuery(req) {
    let url;
    try {
        url = new URL(req.url || '/', 'http://127.0.0.1');
    } catch (_) {
        return { error: true };
    }
    const pairs = [];
    url.searchParams.forEach((value, key) => {
        pairs.push({ key: key, value: value });
    });
    if (pairs.length > 1) return { error: true };
    if (pairs.length === 1 && pairs[0].key !== 'path') return { error: true };
    if (pairs.length === 0) return { path: '' };
    return { path: pairs[0].value };
}

/**
 * The account username is the caller identity this database can store in
 * assigneeId (ADR-0003: an opaque account id, never a display name).
 * @param {object} user
 * @param {unknown} assigneeId
 * @returns {boolean}
 */
function isAssignee(user, assigneeId) {
    if (!user || typeof assigneeId !== 'string' || assigneeId.length === 0) return false;
    return assigneeId === user.username;
}

/**
 * @param {string} slug
 * @param {number} userId
 * @returns {boolean}
 */
function hasGrant(slug, userId) {
    const rows = visibilityRepo.listGrants(slug);
    const id = Number(userId);
    return rows.some((row) => Number(row.user_id) === id);
}

/**
 * Who may receive this snapshot. Draft, hidden, ungranted selected, and a
 * personal plan for anyone but assigneeId are refused. The working copy is
 * not consulted: learners keep the last published snapshot.
 * @param {object} user
 * @param {object} snapshot
 * @returns {boolean}
 */
function mayReadPublished(user, snapshot) {
    if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) return false;
    if (snapshot.status !== 'published') return false;
    if (snapshot.audience === 'personal') {
        return isAssignee(user, snapshot.assigneeId);
    }
    if (snapshot.audience !== 'shared') return false;
    const units = Array.isArray(snapshot.units) ? snapshot.units : [];
    let lessons = 0;
    for (const unit of units) {
        const list = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
        for (const lesson of list) {
            lessons += 1;
            if (!lesson || typeof lesson !== 'object' || typeof lesson.id !== 'string' || !lesson.id) {
                return false;
            }
            if (lesson.visibility === 'everyone') continue;
            if (lesson.visibility === 'selected') {
                if (!hasGrant(lesson.id, user.id)) return false;
                continue;
            }
            return false;
        }
    }
    return lessons > 0;
}

/**
 * @param {import('http').ServerResponse} res
 */
function sendNotFound(res) {
    security.sendJsonError(res, 404, 'Not found', 'not_found');
}

const CATALOG_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const CATALOG_RECIPES = new Set([
    'topic-conversation',
    'review-worksheet',
    'verse-reading',
    'verb-workshop',
    'letters-and-vowels',
    'guide'
]);
const CATALOG_LIMIT = 200;
const CATALOG_TEXT = 120;

/**
 * @param {unknown} value
 * @returns {string}
 */
function clipCatalogText(value) {
    if (typeof value !== 'string') return '';
    return value.length > CATALOG_TEXT ? value.slice(0, CATALOG_TEXT) : value;
}

/**
 * Published lessons the session may already read. Titles and topics only.
 * A snapshot the session cannot read is omitted, including a hidden lesson.
 * Provenance: INIT-006/SPEC-016
 * @param {object} user
 * @param {Array<{ id: string, snapshot: string }>|null|undefined} rows
 * @returns {{ lessons: object[] }}
 */
function catalogFromSnapshots(user, rows) {
    const lessons = [];
    const list = Array.isArray(rows) ? rows : [];
    for (const row of list) {
        if (lessons.length >= CATALOG_LIMIT) break;
        if (!row || typeof row.id !== 'string' || !CATALOG_SLUG.test(row.id)) continue;
        if (typeof row.snapshot !== 'string' || !row.snapshot) continue;
        let snapshot;
        try {
            snapshot = JSON.parse(row.snapshot);
        } catch (_) {
            continue;
        }
        if (!mayReadPublished(user, snapshot)) continue;
        if (!snapshot || snapshot.id !== row.id) continue;
        const units = Array.isArray(snapshot.units) ? snapshot.units : [];
        for (const unit of units) {
            if (lessons.length >= CATALOG_LIMIT) break;
            const unitId = unit && typeof unit.id === 'string' && CATALOG_SLUG.test(unit.id) ? unit.id : '';
            const topic = clipCatalogText(unit && unit.title);
            const unitLessons = unit && Array.isArray(unit.lessons) ? unit.lessons : [];
            for (const lesson of unitLessons) {
                if (lessons.length >= CATALOG_LIMIT) break;
                if (!lesson || typeof lesson.id !== 'string' || !CATALOG_SLUG.test(lesson.id)) continue;
                const recipe = typeof lesson.recipe === 'string' && CATALOG_RECIPES.has(lesson.recipe)
                    ? lesson.recipe
                    : '';
                lessons.push({
                    planId: row.id,
                    unitId: unitId,
                    topic: topic,
                    lessonId: lesson.id,
                    title: clipCatalogText(lesson.title),
                    recipe: recipe
                });
            }
        }
    }
    return { lessons: lessons };
}

/**
 * Write context for an admin session (ADR-0011 §1). The actor is the signed-in
 * account, derived from the session. No body key or query value can set it.
 * @param {{ user: { username: string } }} routeCtx Router context from requireSession.
 * @param {string} tool Operation name recorded as the source label.
 * @returns {object}
 */
function adminContext(routeCtx, tool) {
    return makeContext({ channel: 'admin', username: routeCtx.user.username, tool: tool });
}

/**
 * @param {ReturnType<import('./router').createRouter>} router
 * @param {string} method
 * @param {string} path
 * @param {Function} handler
 */
function addAdmin(router, method, path, handler) {
    router.add(method, path, router.requireSession(router.requireAdmin(handler)));
}

/**
 * @param {import('http').ServerResponse} res
 */
function sendBadRequest(res) {
    security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
}

/**
 * Read a query string against an allowed key list. A repeated or unknown key
 * is an error, as `GET .../parts` does for its one key.
 * @param {import('http').IncomingMessage} req
 * @param {readonly string[]} allowed
 * @returns {{ values: Record<string, string> }|{ error: true }}
 */
function readQuery(req, allowed) {
    let url;
    try {
        url = new URL(req.url || '/', 'http://127.0.0.1');
    } catch (_) {
        return { error: true };
    }
    const values = {};
    let bad = false;
    url.searchParams.forEach((value, key) => {
        if (allowed.indexOf(key) === -1 || Object.prototype.hasOwnProperty.call(values, key)) bad = true;
        else values[key] = value;
    });
    return bad ? { error: true } : { values: values };
}

/**
 * @param {unknown} text
 * @returns {number|null} A whole number from digits only, else null.
 */
function queryInt(text) {
    if (typeof text !== 'string' || !/^\d{1,10}$/.test(text)) return null;
    return Number(text);
}

/**
 * A diff side from the query: digits, `working`, or `published`.
 * @param {unknown} text
 * @returns {number|string|null}
 */
function queryRef(text) {
    if (text === 'working' || text === 'published') return text;
    return queryInt(text);
}

/**
 * Each value is a string, or absent.
 * @param {object} payload
 * @param {readonly string[]} keys
 * @returns {boolean}
 */
function stringsOrAbsent(payload, keys) {
    return keys.every((key) => payload[key] === undefined || typeof payload[key] === 'string');
}

/**
 * The HTTP side of the plan-builder capability table (INIT-008/SPEC-010):
 * list, create, assign, revisions, diff, restore, issues, and the schema. Each
 * route is session, admin, parse, one call, map. Each function is looked up on
 * its module at call time, and each context is built here from the session, so
 * no body or query value can name an actor. No handler edits plan JSON.
 * @param {ReturnType<import('./router').createRouter>} router
 */
function registerBuilderRoutes(router) {
    addAdmin(router, 'GET', '/api/plans', async (req, res, ctx) => {
        const query = readQuery(req, ['limit']);
        if (query.error) return sendBadRequest(res);
        const opts = {};
        if (query.values.limit !== undefined) {
            opts.limit = queryInt(query.values.limit);
            if (opts.limit === null) return sendBadRequest(res);
        }
        sendRead(res, create.listPlans(store, opts, adminContext(ctx, 'list_plans')));
    });

    addAdmin(router, 'POST', '/api/plans', async (req, res, ctx) => {
        const payload = await readBodyKeys(req, res, ['id', 'title'], ['recipe', 'unitId', 'lessonId']);
        if (!payload) return;
        if (!stringsOrAbsent(payload, ['id', 'title', 'recipe', 'unitId', 'lessonId'])) return sendBadRequest(res);
        const input = { id: payload.id, title: payload.title };
        for (const key of ['recipe', 'unitId', 'lessonId']) {
            if (payload[key] !== undefined) input[key] = payload[key];
        }
        sendMapped(res, create.createPlan(store, input, adminContext(ctx, 'create_plan')));
    });

    addAdmin(router, 'POST', '/api/plans/:id/assign', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['audience', 'expectedRevision'], ['assigneeId']);
        if (!payload) return;
        if (!stringsOrAbsent(payload, ['audience', 'assigneeId'])) return sendBadRequest(res);
        const input = { audience: payload.audience, expectedRevision: payload.expectedRevision };
        if (payload.assigneeId !== undefined) input.assigneeId = payload.assigneeId;
        sendMapped(res, personal.assign(store, ctx.params.id, input, adminContext(ctx, 'assign_plan')));
    });

    addAdmin(router, 'GET', '/api/plans/:id/revisions', async (req, res, ctx) => {
        const query = readQuery(req, ['sinceRevision', 'beforeRevision', 'actorKind', 'pathPrefix', 'limit']);
        if (query.error) return sendBadRequest(res);
        const opts = {};
        for (const key of ['sinceRevision', 'beforeRevision', 'limit']) {
            if (query.values[key] === undefined) continue;
            opts[key] = queryInt(query.values[key]);
            if (opts[key] === null) return sendBadRequest(res);
        }
        for (const key of ['actorKind', 'pathPrefix']) {
            if (query.values[key] !== undefined) opts[key] = query.values[key];
        }
        sendRead(res, revisions.list(ctx.params.id, opts, adminContext(ctx, 'list_plan_revisions')));
    });

    addAdmin(router, 'GET', '/api/plans/:id/revisions/:revision', async (req, res, ctx) => {
        const query = readQuery(req, []);
        if (query.error) return sendBadRequest(res);
        sendRead(res, revisions.get(
            ctx.params.id,
            queryInt(ctx.params.revision),
            adminContext(ctx, 'get_plan_revision')
        ));
    });

    addAdmin(router, 'GET', '/api/plans/:id/diff', async (req, res, ctx) => {
        const query = readQuery(req, ['from', 'to', 'path']);
        if (query.error) return sendBadRequest(res);
        const from = queryRef(query.values.from);
        const to = queryRef(query.values.to);
        if (from === null || to === null) return sendBadRequest(res);
        const opts = {};
        if (query.values.path !== undefined) opts.path = query.values.path;
        sendRead(res, revisions.diff(ctx.params.id, from, to, opts, adminContext(ctx, 'diff_plan')));
    });

    addAdmin(router, 'POST', '/api/plans/:id/restore', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['fromRevision', 'expectedRevision'], ['path', 'field']);
        if (!payload) return;
        if (!Number.isInteger(payload.fromRevision) || !stringsOrAbsent(payload, ['path', 'field'])) {
            return sendBadRequest(res);
        }
        const input = { fromRevision: payload.fromRevision, expectedRevision: payload.expectedRevision };
        for (const key of ['path', 'field']) {
            if (payload[key] !== undefined) input[key] = payload[key];
        }
        sendMapped(res, revisions.restore(store, ctx.params.id, input, adminContext(ctx, 'restore_plan_revision')));
    });

    addAdmin(router, 'GET', '/api/plans/:id/issues', async (req, res, ctx) => {
        const query = readQuery(req, ['severity', 'code', 'pathPrefix', 'limit']);
        if (query.error) return sendBadRequest(res);
        const filters = {};
        if (query.values.severity !== undefined) {
            const parts = query.values.severity.split(',');
            filters.severity = parts.length === 1 ? parts[0] : parts;
        }
        for (const key of ['code', 'pathPrefix']) {
            if (query.values[key] !== undefined) filters[key] = query.values[key];
        }
        if (query.values.limit !== undefined) {
            filters.limit = queryInt(query.values.limit);
            if (filters.limit === null) return sendBadRequest(res);
        }
        sendRead(res, issues.list(store, ctx.params.id, filters, adminContext(ctx, 'list_plan_issues')));
    });

    // UI plumbing, with no MCP tool: the AI reads the schema from the repository.
    addAdmin(router, 'GET', '/api/plan-schema', async (req, res) => {
        const text = validate.schemaText();
        const etag = '"' + crypto.createHash('sha256').update(text).digest('hex').slice(0, 32) + '"';
        security.applySecurityHeaders(res);
        if (req.headers['if-none-match'] === etag) {
            res.writeHead(304, { ETag: etag, 'Cache-Control': 'private, no-cache' });
            res.end();
            return;
        }
        res.writeHead(200, {
            'Content-Type': 'application/json; charset=utf-8',
            'Cache-Control': 'private, no-cache',
            ETag: etag,
            'Content-Length': Buffer.byteLength(text)
        });
        res.end(text);
    });
}

/**
 * Register plan routes on a router from createRouter().
 * @param {ReturnType<import('./router').createRouter>} router
 */
function register(router) {
    addAdmin(router, 'GET', '/api/plans/:id/parts', async (req, res, ctx) => {
        const query = readPartQuery(req);
        if (query.error) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        sendMapped(res, ops.readPart(store, ctx.params.id, { path: query.path }, adminContext(ctx, 'read_part')));
    });

    addAdmin(router, 'PUT', '/api/plans/:id/parts', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['path', 'value', 'expectedRevision']);
        if (!payload) return;
        if (typeof payload.path !== 'string') {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!payload.value || typeof payload.value !== 'object' || Array.isArray(payload.value)) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        sendMapped(res, ops.replacePart(store, ctx.params.id, {
            path: payload.path,
            value: payload.value,
            expectedRevision: payload.expectedRevision
        }, adminContext(ctx, 'replace_part')));
    });

    addAdmin(router, 'POST', '/api/plans/:id/blocks', async (req, res, ctx) => {
        const payload = await readOperation(
            req,
            res,
            ['path', 'block', 'expectedRevision'],
            ['index']
        );
        if (!payload) return;
        if (typeof payload.path !== 'string') {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!payload.block || typeof payload.block !== 'object' || Array.isArray(payload.block)) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (payload.index !== undefined && !Number.isInteger(payload.index)) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        const input = {
            path: payload.path,
            block: payload.block,
            expectedRevision: payload.expectedRevision
        };
        if (payload.index !== undefined) input.index = payload.index;
        sendMapped(res, ops.insertBlock(store, ctx.params.id, input, adminContext(ctx, 'insert_block')));
    });

    addAdmin(router, 'POST', '/api/plans/:id/blocks/move', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['path', 'index', 'expectedRevision']);
        if (!payload) return;
        if (typeof payload.path !== 'string' || !Number.isInteger(payload.index)) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        sendMapped(res, ops.moveBlock(store, ctx.params.id, {
            path: payload.path,
            index: payload.index,
            expectedRevision: payload.expectedRevision
        }, adminContext(ctx, 'move_block')));
    });

    addAdmin(router, 'DELETE', '/api/plans/:id/parts', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['path', 'expectedRevision']);
        if (!payload) return;
        if (typeof payload.path !== 'string') {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        sendMapped(res, ops.removeBlock(store, ctx.params.id, {
            path: payload.path,
            expectedRevision: payload.expectedRevision
        }, adminContext(ctx, 'remove_block')));
    });

    addAdmin(router, 'PATCH', '/api/plans/:id/fields', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['path', 'field', 'value', 'expectedRevision']);
        if (!payload) return;
        if (typeof payload.path !== 'string' || typeof payload.field !== 'string') {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        if (!Object.prototype.hasOwnProperty.call(payload, 'value')) {
            security.sendJsonError(res, 400, 'Invalid request', 'bad_request');
            return;
        }
        sendMapped(res, ops.setField(store, ctx.params.id, {
            path: payload.path,
            field: payload.field,
            value: payload.value,
            expectedRevision: payload.expectedRevision
        }, adminContext(ctx, 'set_field')));
    });

    addAdmin(router, 'POST', '/api/plans/:id/publish', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['expectedRevision']);
        if (!payload) return;
        sendMapped(res, publish.publishPlan(store, ctx.params.id, {
            expectedRevision: payload.expectedRevision
        }, adminContext(ctx, 'publish_plan')));
    });

    registerBuilderRoutes(router);

    router.add('GET', '/api/plans/catalog', router.requireSession(async (req, res, ctx) => {
        security.sendJson(res, 200, catalogFromSnapshots(ctx.user, store.listSnapshots()));
    }));

    router.add('GET', '/api/plans/:id/published', router.requireSession(async (req, res, ctx) => {
        let text;
        try {
            text = store.loadSnapshot(ctx.params.id);
        } catch (_) {
            sendNotFound(res);
            return;
        }
        if (typeof text !== 'string' || text.length === 0) {
            sendNotFound(res);
            return;
        }
        let snapshot;
        try {
            snapshot = JSON.parse(text);
        } catch (_) {
            sendNotFound(res);
            return;
        }
        if (!mayReadPublished(ctx.user, snapshot)) {
            sendNotFound(res);
            return;
        }
        security.sendJson(res, 200, snapshot);
    }));
}

module.exports = {
    register,
    WRITE_ROUTES,
    mayReadPublished,
    isAssignee,
    catalogFromSnapshots
};
