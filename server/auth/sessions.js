/**
 * Opaque server-side sessions — ADR-0002 §4.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const crypto = require('crypto');
const db = require('../db');
const sessionsRepo = require('../db/repos/sessions');
const usersRepo = require('../db/repos/users');

const TOKEN_BYTES = 32;
const LIFETIME_MS = 30 * 24 * 60 * 60 * 1000; // 30 days from sign-in
const LAST_SEEN_MIN_INTERVAL_MS = 60 * 1000;

/**
 * @param {Date|number|string|null|undefined} now
 * @returns {Date}
 */
function toDate(now) {
    if (now == null) return new Date();
    if (now instanceof Date) return now;
    if (typeof now === 'number') return new Date(now);
    return new Date(now);
}

function hashToken(token) {
    return crypto.createHash('sha256').update(String(token), 'utf8').digest('hex');
}

/**
 * Create a session for userId. Returns the raw token once; only SHA-256 is stored.
 * @param {number} userId
 * @param {{ now?: Date|number|string }} [meta]
 * @returns {{ token: string, tokenHash: string, expiresAt: string, createdAt: string }}
 */
function create(userId, meta) {
    const opts = meta || {};
    const now = toDate(opts.now);
    const createdAt = now.toISOString();
    const expiresAt = new Date(now.getTime() + LIFETIME_MS).toISOString();
    const token = crypto.randomBytes(TOKEN_BYTES).toString('base64url');
    const tokenHash = hashToken(token);
    sessionsRepo.create({
        user_id: userId,
        token_hash: tokenHash,
        created_at: createdAt,
        last_seen_at: createdAt,
        expires_at: expiresAt
    });
    return { token, tokenHash, expiresAt, createdAt };
}

/**
 * Resolve a raw session token. Lifetime is fixed from sign-in (no idle timeout).
 * Updates last_seen_at at most once per minute.
 * @param {string} token
 * @param {Date|number|string} [now]
 * @returns {{ user: object, session: object } | null}
 */
function resolve(token, now) {
    if (typeof token !== 'string' || !token) return null;
    const at = toDate(now);
    const tokenHash = hashToken(token);
    const session = sessionsRepo.findByTokenHash(tokenHash);
    if (!session) return null;
    const expiresMs = Date.parse(session.expires_at);
    if (!(expiresMs > at.getTime())) return null;
    const user = usersRepo.findById(session.user_id);
    if (!user || user.status !== 'active') return null;

    const lastSeenMs = Date.parse(session.last_seen_at);
    if (!(at.getTime() - lastSeenMs < LAST_SEEN_MIN_INTERVAL_MS)) {
        const iso = at.toISOString();
        sessionsRepo.touch(session.id, iso);
        session.last_seen_at = iso;
    }

    return { user, session };
}

/**
 * @param {string} token raw session token
 * @returns {boolean} true if a row was deleted
 */
function revoke(token) {
    if (typeof token !== 'string' || !token) return false;
    const tokenHash = hashToken(token);
    const session = sessionsRepo.findByTokenHash(tokenHash);
    if (!session) return false;
    sessionsRepo.deleteById(session.id);
    return true;
}

/**
 * Revoke all sessions for a user. Optionally keep one token hash (e.g. current session).
 * @param {number} userId
 * @param {string} [exceptHash]
 */
function revokeAllForUser(userId, exceptHash) {
    if (exceptHash) {
        db.run(
            'DELETE FROM sessions WHERE user_id = ? AND token_hash != ?',
            userId,
            exceptHash
        );
        return;
    }
    sessionsRepo.deleteByUser(userId);
}

module.exports = {
    create,
    resolve,
    revoke,
    revokeAllForUser,
    hashToken,
    toDate,
    TOKEN_BYTES,
    LIFETIME_MS,
    LAST_SEEN_MIN_INTERVAL_MS
};
