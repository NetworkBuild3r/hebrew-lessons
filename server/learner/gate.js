/**
 * Who may read a learner profile or a coverage result, and what each read
 * leaves behind (ADR-0009 §1).
 *
 * `self` is the signed-in learner reading their own data: no switch, no rate
 * count, no audit row. `mcp` is the MCP credential: every call takes one of the
 * 60 reads per UTC clock hour that the profile and coverage share, needs the
 * learner's AI-planning switch, and writes one audit row, refusals included.
 * The count is in memory for the one MCP credential (one replica, as ADR-0002).
 * The hour and the audit time come from the server clock, never from a caller.
 * Provenance: INIT-006/SPEC-007
 */
'use strict';

const usersRepo = require('../db/repos/users');
const auditRepo = require('../db/repos/audit');
const contract = require('./profile-contract');

const HOUR_MS = 60 * 60 * 1000;
const AUDIENCES = new Set(['mcp', 'self']);

/** @type {{ hour: number, count: number }|null} */
let mcpReads = null;

/** @type {() => number} */
let clock = Date.now;

/**
 * @param {unknown} audience
 * @returns {'mcp'|'self'}
 */
function audienceOf(audience) {
    if (!AUDIENCES.has(audience)) {
        throw new TypeError("audience must be 'mcp' or 'self'");
    }
    return /** @type {'mcp'|'self'} */ (audience);
}

/**
 * Take one MCP read in the current UTC clock hour. The count only moves
 * forward: an earlier clock value counts against the latest hour seen. Once the
 * hour is full, refusals do not add to it.
 * @param {number} nowMs server clock
 * @returns {boolean}
 */
function takeMcpRead(nowMs) {
    const hour = Math.floor(nowMs / HOUR_MS);
    if (!mcpReads || hour > mcpReads.hour) {
        mcpReads = { hour, count: 1 };
        return true;
    }
    if (mcpReads.count >= contract.LIMITS.mcpReadsPerHour) return false;
    mcpReads.count += 1;
    return true;
}

/**
 * A `self` read names the signed-in user by id, never by username, so an
 * adapter cannot pass a name from a request body and read another account.
 * @param {unknown} learner
 * @param {'mcp'|'self'} audience
 * @returns {{ id: number, username: string, status: string, ai_planning: number }|undefined}
 */
function lookup(learner, audience) {
    if (Number.isInteger(learner) && learner > 0) return usersRepo.findLearnerById(learner);
    if (audience === 'self') {
        throw new TypeError('a self read takes the signed-in user id');
    }
    if (typeof learner === 'string') return usersRepo.findLearnerByUsername(learner);
    return undefined;
}

/**
 * @param {{ id: number, username: string, ai_planning: number }} row
 * @returns {{ id: number, username: string, aiPlanning: boolean }}
 */
function subject(row) {
    return {
        id: Number(row.id),
        username: String(row.username),
        aiPlanning: Number(row.ai_planning) === 1
    };
}

/**
 * Append one `mcp` audit row at the server clock's time. `detail` carries the
 * outcome, field names, and counts only: no word key and no text.
 * @param {string} action
 * @param {string|null} username
 * @param {object} detail
 */
function record(action, username, detail) {
    auditRepo.append({
        actor: contract.AUDIT.actor,
        action,
        target: username == null ? null : String(username),
        at: new Date(clock()).toISOString(),
        detail: JSON.stringify(detail)
    });
}

/**
 * Admit one read, or refuse it with `{ error }` and no other key.
 * For `mcp` the checks run in this order: rate, input, account, switch, and a
 * refusal is audited with `fields: 'none'`. A disabled or missing account is
 * `not_found`. `self` skips the rate and the switch and writes nothing.
 * @param {unknown} learner a user id, or for `mcp` a username
 * @param {{ audience: 'mcp'|'self', action: string, invalid?: string|null }} opts
 *   `invalid` is the caller's input error code, or null when the input is valid
 * @returns {{ learner: { id: number, username: string, aiPlanning: boolean } }
 *   | { refusal: { error: string } }}
 */
function admit(learner, opts) {
    const row = lookup(learner, opts.audience);
    const active = Boolean(row) && row.status === 'active';
    if (opts.audience === 'self') {
        if (opts.invalid) return { refusal: { error: opts.invalid } };
        if (!active) return { refusal: { error: 'not_found' } };
        return { learner: subject(row) };
    }
    let outcome = null;
    if (!takeMcpRead(clock())) outcome = 'rate_limited';
    else if (opts.invalid) outcome = opts.invalid;
    else if (!active) outcome = 'not_found';
    else if (Number(row.ai_planning) !== 1) outcome = 'planning_off';
    if (outcome) {
        record(opts.action, row ? row.username : null, { outcome, fields: 'none' });
        return { refusal: { error: outcome } };
    }
    return { learner: subject(row) };
}

/**
 * Run an admitted `mcp` read and audit it. The result is returned only after
 * its audit row is written, so a failed audit write returns nothing. A read
 * that throws is audited as `error` and the error is rethrown.
 * @template T
 * @param {string} action
 * @param {{ username: string }} learner
 * @param {() => T} read
 * @param {(result: T) => object} describe audit fields and counts for a result
 * @returns {T}
 */
function auditedRead(action, learner, read, describe) {
    let result;
    try {
        result = read();
    } catch (err) {
        try {
            record(action, learner.username, { outcome: 'error', fields: 'none' });
        } catch (_) {
            // The read's own error is the one the caller sees.
        }
        throw err;
    }
    record(action, learner.username, Object.assign({ outcome: 'ok' }, describe(result)));
    return result;
}

/** Test helper — clear the MCP read count. */
function _resetMcpReads() {
    mcpReads = null;
}

/**
 * Test helper — replace the server clock. A non-function restores Date.now.
 * @param {(() => number)|null} fn
 */
function _setClock(fn) {
    clock = typeof fn === 'function' ? fn : Date.now;
}

module.exports = {
    audienceOf,
    admit,
    auditedRead,
    _resetMcpReads,
    _setClock
};
