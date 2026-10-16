/**
 * Accounts service — sign-in, user lifecycle, last-admin guards, audit.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const crypto = require('crypto');
const db = require('./db');
const usersRepo = require('./db/repos/users');
const auditRepo = require('./db/repos/audit');
const passwords = require('./auth/passwords');
const sessions = require('./auth/sessions');
const throttle = require('./auth/throttle');
const policy = require('./auth/policy');

const GENERIC_AUTH_ERROR = 'Invalid username or password';
const THROTTLE_ERROR = 'try again later';

/** @type {boolean} */
let lastSignInUsedDummy = false;

class AccountsError extends Error {
    /**
     * @param {string} message
     * @param {string} code
     */
    constructor(message, code) {
        super(message);
        this.name = 'AccountsError';
        this.code = code;
    }
}

/**
 * @param {object} row
 * @returns {object}
 */
function publicUser(row) {
    if (!row) return null;
    return {
        id: row.id,
        username: row.username,
        display_name: row.display_name,
        role: row.role,
        status: row.status,
        auth_provider: row.auth_provider,
        google_email: row.google_email || null,
        must_change_password: Boolean(row.must_change_password),
        created_at: row.created_at,
        updated_at: row.updated_at
    };
}

function nowIso(now) {
    return sessions.toDate(now).toISOString();
}

function actorLabel(actor) {
    if (actor == null) return 'system';
    if (typeof actor === 'string') return actor;
    if (actor.username) return String(actor.username);
    if (actor.id != null) return 'user:' + actor.id;
    return 'system';
}

function countActiveAdmins() {
    const row = db.get(
        "SELECT COUNT(*) AS n FROM users WHERE role = 'admin' AND status = 'active'"
    );
    return row ? Number(row.n) : 0;
}

function updateUserFields(id, fields, now) {
    const sets = [];
    const params = [];
    for (const [key, value] of Object.entries(fields)) {
        sets.push(key + ' = ?');
        params.push(value);
    }
    sets.push('updated_at = ?');
    params.push(nowIso(now));
    params.push(id);
    return db.run(
        'UPDATE users SET ' + sets.join(', ') + ' WHERE id = ?',
        ...params
    );
}

function writeAudit(actor, action, target, detail, now) {
    auditRepo.append({
        actor: actorLabel(actor),
        action,
        target: target == null ? null : String(target),
        at: nowIso(now),
        detail: detail == null ? null : detail
    });
}

/**
 * Assert the target is not the last active admin being demoted or disabled.
 * @param {object} target
 * @param {{ role?: string, status?: string }} next
 */
function assertLastAdminSafe(target, next) {
    if (!target || target.role !== 'admin' || target.status !== 'active') return;
    const demoting = next.role != null && next.role !== 'admin';
    const disabling = next.status != null && next.status !== 'active';
    if (!demoting && !disabling) return;
    if (countActiveAdmins() <= 1) {
        throw new AccountsError(
            'Cannot demote or disable the last active admin',
            'last_admin'
        );
    }
}

/**
 * Sign in. Failures are one generic error; lockout is a distinct message.
 * Unknown usernames run verify against a dummy hash (timing).
 *
 * @param {string} username
 * @param {string} password
 * @param {{ ip?: string, now?: Date|number|string, userAgent?: string }} [meta]
 * @returns {{ user: object, token: string, expiresAt: string }}
 */
/**
 * Empty becomes null (clear). A bad address is false.
 * @param {unknown} value
 * @returns {string|null|false}
 */
function normalizeEmail(value) {
    if (value == null) return null;
    const email = String(value).trim().toLowerCase();
    if (!email) return null;
    if (email.length > 254) return false;
    if (!/^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$/.test(email)) return false;
    return email;
}

function signIn(username, password, meta) {
    lastSignInUsedDummy = false;
    const opts = meta || {};
    const now = opts.now;
    const ip = opts.ip || '';

    const gate = throttle.check(username, ip, now);
    if (!gate.ok) {
        throw new AccountsError(THROTTLE_ERROR, 'throttled');
    }

    const user = usersRepo.findByUsername(username);
    let ok = false;
    if (!user) {
        lastSignInUsedDummy = true;
        passwords.verify(password, passwords.dummyHash());
        ok = false;
    } else {
        ok = passwords.verify(password, user.password_hash);
    }

    if (!user || !ok || user.status !== 'active') {
        throttle.recordFailure(username, ip, now);
        throw new AccountsError(GENERIC_AUTH_ERROR, 'auth_failed');
    }

    throttle.clear(username);
    const session = sessions.create(user.id, { now });
    return {
        user: publicUser(user),
        token: session.token,
        expiresAt: session.expiresAt
    };
}

function usernameFromEmail(email) {
    const local = String(email).split('@')[0] || '';
    let base = local.toLowerCase().replace(/[^a-z0-9._-]/g, '');
    base = base.replace(/^[._-]+/, '').replace(/[._-]+$/, '');
    if (!base) base = 'learner';
    if (base.length > 48) base = base.slice(0, 48);
    let candidate = base;
    let n = 2;
    while (usersRepo.findByUsername(candidate)) {
        const suffix = String(n);
        candidate = base.slice(0, 64 - suffix.length) + suffix;
        n += 1;
        if (n > 1000) throw new AccountsError('Could not create account', 'signup_failed');
    }
    return candidate;
}

/**
 * Create a learner from a verified Google profile. Role is always user.
 * The password is random; they sign in with Google.
 * @param {{ email: string, sub?: string, name?: string }} identity
 * @param {Date|number|string} [now]
 */
function provisionGoogleUser(identity, now) {
    const email = normalizeEmail(identity.email);
    const sub = identity.sub ? String(identity.sub) : '';
    const username = usernameFromEmail(email);
    const display = String(identity.name || '').trim() || username;
    let created;
    try {
        created = usersRepo.create({
            username: username,
            password_hash: passwords.hash(crypto.randomBytes(32).toString('base64url')),
            display_name: display,
            role: 'user',
            status: 'active',
            auth_provider: 'google',
            provider_subject: sub || null,
            must_change_password: false,
            created_at: nowIso(now),
            updated_at: nowIso(now)
        });
    } catch (err) {
        const existing = usersRepo.findByGoogleEmail(email) || (sub ? usersRepo.findByProviderSubject(sub) : null);
        if (existing) return existing;
        throw err;
    }
    const id = Number(created.lastInsertRowid);
    updateUserFields(id, { google_email: email }, now);
    writeAudit('google', 'user.create', username, JSON.stringify({ role: 'user', provider: 'google' }), now);
    return usersRepo.findById(id);
}

/**
 * Sign in a verified Google account. An unknown account is created from the profile.
 * Password sign-in is unchanged. A disabled account is not replaced with a new one.
 * @param {{ email: string, sub?: string, name?: string }} identity
 * @param {{ ip?: string, now?: Date|number|string }} [meta]
 * @returns {{ user: object, token: string, expiresAt: string }}
 */
function signInWithGoogle(identity, meta) {
    const opts = meta || {};
    const now = opts.now;
    const ip = opts.ip || '';
    const email = normalizeEmail(identity && identity.email);
    const sub = identity && identity.sub ? String(identity.sub) : '';
    if (!email) throw new AccountsError(GENERIC_AUTH_ERROR, 'not_linked');

    const gate = throttle.check(email, ip, now);
    if (!gate.ok) throw new AccountsError(THROTTLE_ERROR, 'throttled');

    const bySub = sub ? usersRepo.findByProviderSubject(sub) : null;
    const byEmail = usersRepo.findByGoogleEmail(email);
    if (bySub && byEmail && bySub.id !== byEmail.id) {
        throttle.recordFailure(email, ip, now);
        throw new AccountsError(GENERIC_AUTH_ERROR, 'not_linked');
    }
    let user = bySub || byEmail;
    if (!user) {
        user = provisionGoogleUser({ email: email, sub: sub, name: identity.name }, now);
    }
    if (user.status !== 'active') {
        throttle.recordFailure(email, ip, now);
        throw new AccountsError(GENERIC_AUTH_ERROR, 'disabled');
    }
    if (user.google_email && user.google_email !== email) {
        throttle.recordFailure(email, ip, now);
        throw new AccountsError(GENERIC_AUTH_ERROR, 'not_linked');
    }
    if (user.provider_subject && sub && user.provider_subject !== sub) {
        throttle.recordFailure(email, ip, now);
        throw new AccountsError(GENERIC_AUTH_ERROR, 'not_linked');
    }
    if (sub && !user.provider_subject) {
        updateUserFields(user.id, { provider_subject: sub }, now);
    }
    throttle.clear(email);
    const session = sessions.create(user.id, { now });
    return {
        user: publicUser(usersRepo.findById(user.id)),
        token: session.token,
        expiresAt: session.expiresAt
    };
}

/**
 * Link, replace, or clear the Google email on a user. Clearing also drops the stored Google subject.
 * @param {number} userId
 * @param {string|null} email
 * @param {string|object} actor
 * @param {Date|number|string} [now]
 */
function setGoogleEmail(userId, email, actor, now) {
    const target = usersRepo.findById(userId);
    if (!target) throw new AccountsError('User not found', 'not_found');
    const normalized = normalizeEmail(email);
    if (normalized === false) throw new AccountsError('Invalid email', 'invalid_email');
    if (normalized) {
        const taken = usersRepo.findByGoogleEmail(normalized);
        if (taken && taken.id !== target.id) {
            throw new AccountsError('That Google email is already linked', 'google_email_taken');
        }
    }
    const same = (target.google_email || null) === normalized;
    const fields = { google_email: normalized };
    if (!same) fields.provider_subject = null;
    updateUserFields(userId, fields, now);
    writeAudit(actor, 'user.setGoogleEmail', target.username, JSON.stringify({
        linked: Boolean(normalized)
    }), now);
    return publicUser(usersRepo.findById(userId));
}

/**
 * @param {{
 *   username: string,
 *   password: string,
 *   role?: 'user'|'admin',
 *   displayName?: string|null,
 *   googleEmail?: string|null,
 *   actor?: string|object,
 *   now?: Date|number|string
 * }} input
 */
function createUser(input) {
    const policyResult = passwords.policyCheck(input.password);
    if (!policyResult.ok) {
        throw new AccountsError(policyResult.reason, 'password_policy');
    }
    const role = input.role || 'user';
    if (role !== 'user' && role !== 'admin') {
        throw new AccountsError('Invalid role', 'invalid_role');
    }
    const existing = usersRepo.findByUsername(input.username);
    if (existing) {
        throw new AccountsError('Username already exists', 'username_taken');
    }
    let googleEmail = null;
    if (input.googleEmail != null && String(input.googleEmail).trim()) {
        googleEmail = normalizeEmail(input.googleEmail);
        if (googleEmail === false) throw new AccountsError('Invalid email', 'invalid_email');
        if (usersRepo.findByGoogleEmail(googleEmail)) {
            throw new AccountsError('That Google email is already linked', 'google_email_taken');
        }
    }
    const passwordHash = passwords.hash(input.password);
    const created = usersRepo.create({
        username: input.username,
        password_hash: passwordHash,
        display_name: input.displayName == null ? null : input.displayName,
        role,
        status: 'active',
        must_change_password: false,
        created_at: nowIso(input.now),
        updated_at: nowIso(input.now)
    });
    const id = Number(created.lastInsertRowid);
    if (googleEmail) updateUserFields(id, { google_email: googleEmail }, input.now);
    writeAudit(input.actor, 'user.create', input.username, JSON.stringify({ role }), input.now);
    return publicUser(usersRepo.findById(id));
}

/**
 * @param {number} userId
 * @param {'user'|'admin'} role
 * @param {string|object} actor
 * @param {Date|number|string} [now]
 */
function setRole(userId, role, actor, now) {
    if (role !== 'user' && role !== 'admin') {
        throw new AccountsError('Invalid role', 'invalid_role');
    }
    const target = usersRepo.findById(userId);
    if (!target) throw new AccountsError('User not found', 'not_found');
    assertLastAdminSafe(target, { role });
    updateUserFields(userId, { role }, now);
    sessions.revokeAllForUser(userId);
    writeAudit(actor, 'user.setRole', target.username, JSON.stringify({ role }), now);
    return publicUser(usersRepo.findById(userId));
}

/**
 * @param {number} userId
 * @param {'active'|'disabled'} status
 * @param {string|object} actor
 * @param {Date|number|string} [now]
 */
function setStatus(userId, status, actor, now) {
    if (status !== 'active' && status !== 'disabled') {
        throw new AccountsError('Invalid status', 'invalid_status');
    }
    const target = usersRepo.findById(userId);
    if (!target) throw new AccountsError('User not found', 'not_found');

    if (status === 'disabled') {
        const actorId = typeof actor === 'object' && actor ? actor.id : null;
        if (actorId != null && Number(actorId) === Number(userId)) {
            throw new AccountsError('Cannot disable your own account', 'self_disable');
        }
        assertLastAdminSafe(target, { status });
    }

    updateUserFields(userId, { status }, now);
    if (status === 'disabled') {
        sessions.revokeAllForUser(userId);
    }
    writeAudit(actor, 'user.setStatus', target.username, JSON.stringify({ status }), now);
    return publicUser(usersRepo.findById(userId));
}

/**
 * Admin password reset — sets must_change_password and revokes all sessions.
 * @param {number} userId
 * @param {string} newPassword
 * @param {string|object} actor
 * @param {Date|number|string} [now]
 */
function resetPassword(userId, newPassword, actor, now) {
    const policyResult = passwords.policyCheck(newPassword);
    if (!policyResult.ok) {
        throw new AccountsError(policyResult.reason, 'password_policy');
    }
    const target = usersRepo.findById(userId);
    if (!target) throw new AccountsError('User not found', 'not_found');
    const passwordHash = passwords.hash(newPassword);
    updateUserFields(userId, {
        password_hash: passwordHash,
        must_change_password: 1
    }, now);
    sessions.revokeAllForUser(userId);
    writeAudit(actor, 'user.resetPassword', target.username, null, now);
    return publicUser(usersRepo.findById(userId));
}

/**
 * Self password change — verifies current password; revokes other sessions.
 * @param {number} userId
 * @param {string} currentPassword
 * @param {string} newPassword
 * @param {{ now?: Date|number|string, keepToken?: string }} [opts]
 */
function changePassword(userId, currentPassword, newPassword, opts) {
    const options = opts || {};
    const policyResult = passwords.policyCheck(newPassword);
    if (!policyResult.ok) {
        throw new AccountsError(policyResult.reason, 'password_policy');
    }
    const target = usersRepo.findById(userId);
    if (!target) throw new AccountsError('User not found', 'not_found');
    if (!passwords.verify(currentPassword, target.password_hash)) {
        throw new AccountsError(GENERIC_AUTH_ERROR, 'auth_failed');
    }
    const passwordHash = passwords.hash(newPassword);
    updateUserFields(userId, {
        password_hash: passwordHash,
        must_change_password: 0
    }, options.now);
    const exceptHash = options.keepToken
        ? sessions.hashToken(options.keepToken)
        : undefined;
    sessions.revokeAllForUser(userId, exceptHash);
    writeAudit(
        target.username,
        'user.changePassword',
        target.username,
        null,
        options.now
    );
    return publicUser(usersRepo.findById(userId));
}

/**
 * @returns {object[]} public user rows (no password hashes)
 */
/**
 * The audit actor ADR-0004 §10 names for an admin action.
 * @param {string|object|null|undefined} actor
 * @returns {string}
 */
function adminActor(actor) {
    const label = actorLabel(actor);
    if (label.startsWith('admin:')) return label;
    return 'admin:' + label;
}

/**
 * Turn AI planning on or off for one account and write the audit row.
 * `users.setAiPlanning` stores the flag and does not audit; this is the
 * caller that does (INIT-006/SPEC-010). `enabled` is a boolean or 0 or 1.
 * @param {number} userId
 * @param {boolean|number} enabled
 * @param {string|object} actor signed-in admin
 * @param {Date|number|string} [now]
 * @returns {{ previous: 0|1, next: 0|1, username: string }}
 */
function setAiPlanning(userId, enabled, actor, now) {
    const id = Number(userId);
    if (!Number.isInteger(id) || id <= 0) {
        throw new AccountsError('User not found', 'not_found');
    }
    const target = usersRepo.findById(id);
    if (!target) throw new AccountsError('User not found', 'not_found');
    const result = usersRepo.setAiPlanning(id, enabled);
    if (!result) throw new AccountsError('User not found', 'not_found');
    writeAudit(
        adminActor(actor),
        'ai_planning_switch',
        target.username,
        JSON.stringify({ previous: result.previous, next: result.next }),
        now
    );
    return {
        previous: result.previous,
        next: result.next,
        username: target.username
    };
}

function listUsers() {
    return usersRepo.listAll().map(publicUser);
}

function didLastSignInUseDummy() {
    return lastSignInUsedDummy;
}

module.exports = {
    signIn,
    signInWithGoogle,
    setGoogleEmail,
    createUser,
    setRole,
    setStatus,
    resetPassword,
    changePassword,
    listUsers,
    setAiPlanning,
    publicUser,
    AccountsError,
    GENERIC_AUTH_ERROR,
    THROTTLE_ERROR,
    didLastSignInUseDummy,
    policy,
    countActiveAdmins
};
