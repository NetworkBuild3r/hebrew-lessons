/**
 * The plan-builder capability table as data (INIT-008 initiative section 12,
 * ADR-0011 Decision 7), and a pure checker over it. One function, two adapters:
 * every user action has an MCP tool and an admin route that call the same
 * function. test/plan-builder-parity.test.js reads this table, and runs the
 * checker on the real registries and on mutated copies, so it cannot pass by
 * being blind. SPEC-016 reuses it.
 *
 * A row is { operation, module, fn, tool, route, write, exemption }.
 *   module + fn   the service function both adapters call (looked up on the
 *                 module object at call time, so a spy sees it)
 *   tool          the MCP tool name, or null with an exemption
 *   route         { method, path }, or null with an exemption
 *   write         true when the route mutates a plan (it must be in WRITE_ROUTES)
 *   exemption     why a side is missing; null when both sides exist
 *   note          optional remark on a row that has both sides
 * Provenance: INIT-008/SPEC-010
 */
'use strict';

/** @type {ReadonlyArray<object>} */
const ROWS = Object.freeze([
    { operation: 'read a part, with editState', module: 'ops', fn: 'readPart', tool: 'read_part', route: { method: 'GET', path: '/api/plans/:id/parts' }, write: false, exemption: null },
    { operation: 'replace a part', module: 'ops', fn: 'replacePart', tool: 'replace_part', route: { method: 'PUT', path: '/api/plans/:id/parts' }, write: true, exemption: null },
    { operation: 'insert a block', module: 'ops', fn: 'insertBlock', tool: 'insert_block', route: { method: 'POST', path: '/api/plans/:id/blocks' }, write: true, exemption: null },
    { operation: 'move a block', module: 'ops', fn: 'moveBlock', tool: 'move_block', route: { method: 'POST', path: '/api/plans/:id/blocks/move' }, write: true, exemption: null },
    { operation: 'remove a block', module: 'ops', fn: 'removeBlock', tool: 'remove_block', route: { method: 'DELETE', path: '/api/plans/:id/parts' }, write: true, exemption: null },
    { operation: 'set a field', module: 'ops', fn: 'setField', tool: 'set_field', route: { method: 'PATCH', path: '/api/plans/:id/fields' }, write: true, exemption: null },
    { operation: 'publish', module: 'publish', fn: 'publishPlan', tool: 'publish_plan', route: { method: 'POST', path: '/api/plans/:id/publish' }, write: true, exemption: null },
    { operation: 'list plans', module: 'create', fn: 'listPlans', tool: 'list_plans', route: { method: 'GET', path: '/api/plans' }, write: false, exemption: null },
    { operation: 'create a plan', module: 'create', fn: 'createPlan', tool: 'create_plan', route: { method: 'POST', path: '/api/plans' }, write: true, exemption: null },
    { operation: 'assign a plan', module: 'personal', fn: 'assign', tool: 'assign_plan', route: { method: 'POST', path: '/api/plans/:id/assign' }, write: true, exemption: null },
    { operation: 'list revisions', module: 'revisions', fn: 'list', tool: 'list_plan_revisions', route: { method: 'GET', path: '/api/plans/:id/revisions' }, write: false, exemption: null },
    { operation: 'read one revision', module: 'revisions', fn: 'get', tool: 'get_plan_revision', route: { method: 'GET', path: '/api/plans/:id/revisions/:revision' }, write: false, exemption: null },
    { operation: 'diff', module: 'revisions', fn: 'diff', tool: 'diff_plan', route: { method: 'GET', path: '/api/plans/:id/diff' }, write: false, exemption: null },
    { operation: 'restore (whole, part, or field)', module: 'revisions', fn: 'restore', tool: 'restore_plan_revision', route: { method: 'POST', path: '/api/plans/:id/restore' }, write: true, exemption: null },
    { operation: 'list issues', module: 'issues', fn: 'list', tool: 'list_plan_issues', route: { method: 'GET', path: '/api/plans/:id/issues' }, write: false, exemption: null },
    {
        operation: 'preview as the learner (saved working copy)',
        module: 'ops',
        fn: 'readPart',
        tool: 'read_part',
        route: { method: 'GET', path: '/api/plans/:id/parts' },
        write: false,
        exemption: null,
        note: 'no new route and no new tool: read_part with an empty path; unsaved form state stays in the browser'
    },
    {
        operation: 'schema for the editor',
        module: 'validate',
        fn: 'schemaText',
        tool: null,
        route: { method: 'GET', path: '/api/plan-schema' },
        write: false,
        exemption: 'UI plumbing: a read helper for the form renderer; the AI reads the schema file in the repository'
    }
]);

/**
 * Acts a person does and the MCP can never do (ADR-0011 Decision 6). The tool
 * is registered only to answer `forbidden`, or does not exist at all.
 * @type {ReadonlyArray<object>}
 */
const HUMAN_ONLY = Object.freeze([
    { act: 'mark a lexicon form verified', tool: 'set_lexicon_verified', route: { method: 'PATCH', path: '/api/admin/lexicon/:key/verified' } },
    { act: 'turn the AI-planning switch on', tool: 'set_ai_planning', route: { method: 'PATCH', path: '/api/admin/users/:id/ai-planning' } },
    { act: 'approve a picture', tool: null, route: { method: 'POST', path: '/api/admin/media/:id/approval' } }
]);

/**
 * @param {{ method: string, path: string }} route
 * @returns {string}
 */
function routeKey(route) {
    return route.method + ' ' + route.path;
}

/**
 * Compare a table against what is registered. Returns one string per problem;
 * an empty list means the table holds.
 * @param {ReadonlyArray<object>} rows
 * @param {{ tools: Iterable<string>, routes: Iterable<string>, writeRoutes: Iterable<string>,
 *   functions?: (module: string, fn: string) => boolean }} registry
 *   `routes` and `writeRoutes` hold `METHOD /path` keys; `tools` holds tool names.
 * @returns {string[]}
 */
function findParityProblems(rows, registry) {
    const tools = new Set(registry.tools);
    const routes = new Set(registry.routes);
    const writes = new Set(registry.writeRoutes);
    const problems = [];
    const seen = new Set();
    for (const row of rows) {
        const label = row.operation;
        if (seen.has(label)) problems.push('duplicate row: ' + label);
        seen.add(label);
        if (row.tool && !tools.has(row.tool)) problems.push('tool not registered: ' + row.tool + ' (' + label + ')');
        if (row.route && !routes.has(routeKey(row.route))) {
            problems.push('route not registered: ' + routeKey(row.route) + ' (' + label + ')');
        }
        if (!row.tool && !row.exemption) problems.push('no MCP tool and no exemption: ' + label);
        if (!row.route && !row.exemption) problems.push('no route and no exemption: ' + label);
        if (!row.tool && !row.route) problems.push('neither side exists: ' + label);
        if (row.exemption && row.tool && row.route) {
            problems.push('exemption on a row that has both sides: ' + label);
        }
        if (row.write && row.route && !writes.has(routeKey(row.route))) {
            problems.push('write route missing from WRITE_ROUTES: ' + routeKey(row.route));
        }
        if (row.write && !row.route) problems.push('write row without a route: ' + label);
        if (registry.functions && !registry.functions(row.module, row.fn)) {
            problems.push('function not found: ' + row.module + '.' + row.fn + ' (' + label + ')');
        }
    }
    return problems;
}

module.exports = { ROWS, HUMAN_ONLY, findParityProblems, routeKey };
