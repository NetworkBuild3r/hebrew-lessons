/**
 * Authorization matrix — ADR-0002 §6.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

/**
 * Action names covering the ADR-0002 role × route matrix (admin surface).
 * Learner-scoped actions are allowed for any active signed-in role.
 */
const ADMIN_ACTIONS = Object.freeze([
    'admin.users.list',
    'admin.users.create',
    'admin.users.update',
    'admin.users.resetPassword',
    'admin.progress.read',
    'admin.events.read',
    'admin.lessons.read',
    'admin.lessons.write',
    'admin.lessons.visibility',
    'admin.audio.plan',
    'admin.audio.generate',
    'admin.page'
]);

const USER_ACTIONS = Object.freeze([
    'session.create',
    'session.delete',
    'me.read',
    'me.password.change',
    'review.state.read',
    'review.signals.write',
    'lessons.progress.read',
    'lessons.progress.write',
    'me.metrics.read'
]);

/** @type {Readonly<Record<string, ReadonlySet<string>>>} */
const MATRIX = Object.freeze({
    user: new Set(USER_ACTIONS),
    admin: new Set(USER_ACTIONS.concat(ADMIN_ACTIONS))
});

/**
 * @param {{ role?: string, status?: string } | null | undefined} user
 * @param {string} action
 * @param {unknown} [_resource]
 * @returns {boolean}
 */
function can(user, action, _resource) {
    if (!user || typeof action !== 'string') return false;
    if (user.status && user.status !== 'active') return false;
    const role = user.role;
    const allowed = MATRIX[role];
    if (!allowed) return false;
    return allowed.has(action);
}

/**
 * @param {{ role?: string, status?: string } | null | undefined} user
 * @returns {boolean}
 */
function requireAdmin(user) {
    return Boolean(user && user.status === 'active' && user.role === 'admin');
}

module.exports = {
    can,
    requireAdmin,
    ADMIN_ACTIONS,
    USER_ACTIONS,
    MATRIX
};
