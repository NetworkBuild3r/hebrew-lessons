/**
 * The issue list: every finding the validator produces for one plan, with a
 * stable code, an id path the part operations accept, the field below that
 * node, and a fix hint where one is known. Either the AI or an admin can act
 * on an issue with one `set_field` or `replace_part` call (ADR-0011 Decision 5).
 * Read only: it opens the working copy and validates it; it writes nothing.
 * The route and the MCP tool (SPEC-009, SPEC-010) are thin callers of list().
 * Provenance: INIT-008/SPEC-008
 */
'use strict';

const lexicon = require('../db/repos/lexicon');
const { validatePlan } = require('./validate');
const { openWorkingCopy } = require('./ops');
const { toIdPath } = require('./id-path');

const SEVERITIES = Object.freeze(['block', 'warn', 'inform']);
const DEFAULT_SEVERITIES = Object.freeze(['block', 'warn']);
const DEFAULT_LIMIT = 100;
const MAX_LIMIT = 200;

function invalid(field) {
    return { error: 'invalid_op', field };
}

/**
 * @param {unknown} value A severity, an array of them, or 'all'. Absent means block and warn.
 * @returns {string[]|null} null when the value names an unknown severity.
 */
function severitiesOf(value) {
    if (value === undefined || value === null) return DEFAULT_SEVERITIES.slice();
    if (value === 'all') return SEVERITIES.slice();
    const wanted = Array.isArray(value) ? value : [value];
    if (!wanted.length || !wanted.every((item) => SEVERITIES.includes(item))) return null;
    return SEVERITIES.filter((item) => wanted.includes(item));
}

/**
 * True when `path` is `prefix` or under it, on a segment boundary, so `units/a`
 * does not match `units/ab`. A leading slash on the prefix is tolerated.
 */
function underPrefix(path, prefix) {
    const base = prefix.replace(/^\/+/, '').replace(/\/+$/, '');
    return base === '' || path === base || path.startsWith(base + '/');
}

/**
 * Turn validator findings into issues, sorted block, warn, inform, then path.
 * @param {object} plan The document the findings were reported against.
 * @param {object[]} findings
 * @returns {object[]}
 */
function toIssues(plan, findings) {
    const issues = findings.map((finding) => {
        const target = toIdPath(plan, finding.path);
        const issue = {
            code: finding.rule,
            severity: finding.severity,
            hard: finding.severity === 'block',
            path: target.path,
            field: target.field,
            pointer: finding.path,
            message: finding.message
        };
        if (finding.fix) issue.fix = finding.fix;
        return issue;
    });
    const rank = (issue) => SEVERITIES.indexOf(issue.severity);
    const order = (a, b) => (a < b ? -1 : (a > b ? 1 : 0));
    return issues.sort((a, b) => rank(a) - rank(b)
        || order(a.path, b.path) || order(a.pointer, b.pointer) || order(a.code, b.code));
}

/**
 * List the issues of one plan's working copy.
 * @param {object} store Plan store from server/plans/store.js.
 * @param {string} planId
 * @param {{ severity?: string|string[], code?: string, pathPrefix?: string, limit?: number }} [filters]
 *   `severity` defaults to block and warn; `inform` is listed only when asked, or with 'all'.
 *   `limit` defaults to 100 and is capped at 200.
 * @param {object} [ctx] Accepted for the adapters' call shape; a read needs no context.
 * @returns {{ revision: number, status: string,
 *   counts: { block: number, warn: number, inform: number },
 *   issues: { code: string, severity: string, hard: boolean, path: string, field: string,
 *     pointer: string, message: string, fix?: object }[],
 *   truncated: boolean }|{ error: string, revision?: number, field?: string }}
 *   `counts` always covers every severity of the whole plan, whatever the filters.
 */
function list(store, planId, filters, ctx) { // eslint-disable-line no-unused-vars
    const options = filters && typeof filters === 'object' ? filters : {};
    const severities = severitiesOf(options.severity);
    if (!severities) return invalid('severity');
    if (options.code !== undefined && options.code !== null && typeof options.code !== 'string') return invalid('code');
    if (options.pathPrefix !== undefined && options.pathPrefix !== null && typeof options.pathPrefix !== 'string') {
        return invalid('pathPrefix');
    }
    let limit = DEFAULT_LIMIT;
    if (options.limit !== undefined && options.limit !== null) {
        if (!Number.isInteger(options.limit) || options.limit < 1) return invalid('limit');
        limit = Math.min(options.limit, MAX_LIMIT);
    }

    const opened = openWorkingCopy(store, planId);
    if (opened.error) {
        const out = { error: opened.error };
        if (opened.revision != null) out.revision = opened.revision;
        return out;
    }
    const validation = validatePlan(opened.plan, { lexicon, snapshot: opened.row.snapshot });
    const all = toIssues(opened.plan, validation.blocks.concat(validation.warnings, validation.informs));
    const counts = { block: 0, warn: 0, inform: 0 };
    all.forEach((issue) => { counts[issue.severity] += 1; });

    const matching = all.filter((issue) => severities.includes(issue.severity)
        && (!options.code || issue.code === options.code)
        && (!options.pathPrefix || underPrefix(issue.path, options.pathPrefix)));
    return {
        revision: opened.row.revision,
        status: opened.row.status,
        counts,
        issues: matching.slice(0, limit),
        truncated: matching.length > limit
    };
}

module.exports = { DEFAULT_LIMIT, MAX_LIMIT, list, toIssues };
