/**
 * Plan HTTP routes. Session, role, and body checks, then one function
 * in server/plans/ops.js or publish.js. These handlers do not edit JSON.
 * Mutating CSRF and Origin checks run in server/web.js before dispatch,
 * the same gate as every other /api/ write.
 * Provenance: INIT-004/SPEC-004
 */
'use strict';

const body = require('../http/body');
const security = require('../http/security');
const ops = require('../plans/ops');
const publish = require('../plans/publish');
const store = require('../plans/store');
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
    { method: 'POST', path: '/api/plans/:id/publish', fn: 'publishPlan' }
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
    if (!Number.isInteger(payload.expectedRevision)) {
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
    if (!result || typeof result !== 'object') {
        security.sendJsonError(res, 500, 'Internal error', 'internal');
        return;
    }
    if (result.error === 'revision_conflict') {
        security.sendJson(res, 409, {
            error: 'revision_conflict',
            revision: result.revision
        });
        return;
    }
    if (result.error === 'not_found') {
        security.sendJsonError(res, 404, 'Not found', 'not_found');
        return;
    }
    if (result.error === 'validation') {
        security.sendJson(res, 400, {
            error: 'validation',
            revision: result.revision,
            blocks: result.blocks,
            warnings: result.warnings,
            informs: result.informs
        });
        return;
    }
    if (result.error) {
        security.sendJsonError(res, 400, 'Invalid request', result.error);
        return;
    }
    const bodyOut = {};
    if (result.part !== undefined) bodyOut.part = result.part;
    if (result.plan !== undefined) bodyOut.plan = result.plan;
    if (result.revision !== undefined) bodyOut.revision = result.revision;
    if (result.status !== undefined) bodyOut.status = result.status;
    if (result.validation !== undefined) bodyOut.validation = result.validation;
    security.sendJson(res, 200, bodyOut);
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
 * @param {ReturnType<import('./router').createRouter>} router
 * @param {string} method
 * @param {string} path
 * @param {Function} handler
 */
function addAdmin(router, method, path, handler) {
    router.add(method, path, router.requireSession(router.requireAdmin(handler)));
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
        sendMapped(res, ops.readPart(store, ctx.params.id, { path: query.path }));
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
        }));
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
        sendMapped(res, ops.insertBlock(store, ctx.params.id, input));
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
        }));
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
        }));
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
        }));
    });

    addAdmin(router, 'POST', '/api/plans/:id/publish', async (req, res, ctx) => {
        const payload = await readOperation(req, res, ['expectedRevision']);
        if (!payload) return;
        sendMapped(res, publish.publishPlan(store, ctx.params.id, {
            expectedRevision: payload.expectedRevision
        }));
    });

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
