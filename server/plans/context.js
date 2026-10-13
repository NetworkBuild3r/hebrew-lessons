/**
 * Write context for plan operations (ADR-0011 §1).
 * The actor is a fact about the channel the request arrived on, never a claim
 * by the caller. An adapter builds the context from its own channel:
 *   mcp    -> kind `ai`,     name `mcp`
 *   admin  -> kind `human`,  name = the signed-in session's username
 *   system -> kind `system`, name = the script or subsystem name
 * No request body key, no MCP argument, and no JSON value can set the actor:
 * a context made by makeContext carries a Symbol brand that JSON cannot hold,
 * and every mutating function calls requireContext before it reads anything.
 * Contexts hold no plan text and no field value.
 * Provenance: INIT-008/SPEC-002
 */
'use strict';

/** Brand shared across module reloads (tests clear the require cache). */
const BRAND = Symbol.for('hebrew.plans.context');

const KINDS = Object.freeze(['ai', 'human', 'system']);
const CHANNEL_KIND = Object.freeze({ mcp: 'ai', admin: 'human', system: 'system' });

const NAME_MAX = 64;
const LABEL_MAX = 64;

/**
 * @param {unknown} value
 * @param {number} max
 * @returns {string|undefined}
 */
function label(value, max) {
    if (value == null) return undefined;
    if (typeof value !== 'string' && typeof value !== 'number') return undefined;
    const text = String(value).replace(/[\u0000-\u001f\u007f]/g, ' ').trim();
    if (!text) return undefined;
    return text.length > max ? text.slice(0, max) : text;
}

/**
 * Build a context from a channel. Throws TypeError for an unknown channel or
 * a channel that needs a name and has none.
 * @param {{
 *   channel: 'mcp'|'admin'|'system',
 *   tool?: string,
 *   username?: string,
 *   name?: string,
 *   requestId?: string|number,
 *   client?: string
 * }} input
 * @returns {{ actor: { kind: string, name: string, tool?: string, client?: string }, requestId?: string }}
 */
function makeContext(input) {
    const src = input && typeof input === 'object' ? input : {};
    const kind = Object.prototype.hasOwnProperty.call(CHANNEL_KIND, src.channel)
        ? CHANNEL_KIND[src.channel]
        : null;
    if (!kind) throw new TypeError('context channel must be mcp, admin, or system');

    let name;
    if (src.channel === 'mcp') {
        name = 'mcp';
    } else if (src.channel === 'admin') {
        name = label(src.username, NAME_MAX);
        if (!name) throw new TypeError('an admin context needs the session username');
    } else {
        name = label(src.name, NAME_MAX);
        if (!name) throw new TypeError('a system context needs a script or subsystem name');
    }

    const actor = { kind: kind, name: name };
    const tool = label(src.tool, LABEL_MAX);
    const client = label(src.client, LABEL_MAX);
    if (tool) actor.tool = tool;
    if (client) actor.client = client;

    const ctx = { actor: Object.freeze(actor) };
    const requestId = label(src.requestId, LABEL_MAX);
    if (requestId) ctx.requestId = requestId;
    Object.defineProperty(ctx, BRAND, { value: true, enumerable: false });
    return Object.freeze(ctx);
}

/**
 * @param {unknown} ctx
 * @returns {boolean}
 */
function isContext(ctx) {
    if (!ctx || typeof ctx !== 'object' || ctx[BRAND] !== true) return false;
    const actor = ctx.actor;
    if (!actor || typeof actor !== 'object') return false;
    if (KINDS.indexOf(actor.kind) === -1) return false;
    return typeof actor.name === 'string' && actor.name.length > 0;
}

/**
 * Throw TypeError unless ctx came from makeContext. A write with no context
 * must fail before it reads or writes anything.
 * @param {unknown} ctx
 * @returns {object} The same context.
 */
function requireContext(ctx) {
    if (!isContext(ctx)) {
        throw new TypeError('a plan write needs a context from makeContext');
    }
    return ctx;
}

/**
 * The audit_log actor string (ADR-0004 §10): `admin:<username>`, `mcp`,
 * or `system:<name>`.
 * @param {object} ctx
 * @returns {string}
 */
function actorString(ctx) {
    requireContext(ctx);
    const actor = ctx.actor;
    if (actor.kind === 'human') return 'admin:' + actor.name;
    if (actor.kind === 'ai') return 'mcp';
    return 'system:' + actor.name;
}

module.exports = { makeContext, requireContext, isContext, actorString, KINDS };
