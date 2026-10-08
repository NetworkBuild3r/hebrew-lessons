/**
 * In-memory sign-in throttle — ADR-0002 §3.
 * One replica only (documented operational limit).
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const MAX_USERNAME_FAILURES = 5;
const INITIAL_BACKOFF_MS = 30 * 1000;
const MAX_BACKOFF_MS = 15 * 60 * 1000;
const IP_CAP = 20;
const IP_WINDOW_MS = 15 * 60 * 1000;

/** @type {Map<string, { failures: number, lockedUntil: number, nextBackoffMs: number }>} */
const byUsername = new Map();
/** @type {Map<string, number[]>} */
const byIp = new Map();

function toMs(now) {
    if (now == null) return Date.now();
    if (now instanceof Date) return now.getTime();
    if (typeof now === 'number') return now;
    return new Date(now).getTime();
}

function normalizeUsername(username) {
    return String(username || '').toLowerCase();
}

function normalizeIp(ip) {
    return String(ip || '');
}

/**
 * @param {string} username
 * @param {string} ip
 * @param {Date|number|string} [now]
 * @returns {{ ok: true } | { ok: false, reason: string, retryAfterMs: number }}
 */
function check(username, ip, now) {
    const t = toMs(now);
    const key = normalizeUsername(username);
    const entry = byUsername.get(key);
    if (entry && entry.lockedUntil > t) {
        return {
            ok: false,
            reason: 'try again later',
            retryAfterMs: entry.lockedUntil - t
        };
    }

    const ipKey = normalizeIp(ip);
    const stamps = (byIp.get(ipKey) || []).filter((ts) => t - ts < IP_WINDOW_MS);
    byIp.set(ipKey, stamps);
    if (stamps.length >= IP_CAP) {
        const oldest = stamps[0];
        return {
            ok: false,
            reason: 'try again later',
            retryAfterMs: Math.max(0, IP_WINDOW_MS - (t - oldest))
        };
    }

    return { ok: true };
}

/**
 * Record a failed sign-in for username and client IP.
 * @param {string} username
 * @param {string} ip
 * @param {Date|number|string} [now]
 */
function recordFailure(username, ip, now) {
    const t = toMs(now);
    const key = normalizeUsername(username);
    let entry = byUsername.get(key);
    if (!entry) {
        entry = { failures: 0, lockedUntil: 0, nextBackoffMs: INITIAL_BACKOFF_MS };
        byUsername.set(key, entry);
    }
    entry.failures += 1;
    if (entry.failures >= MAX_USERNAME_FAILURES) {
        entry.lockedUntil = t + entry.nextBackoffMs;
        entry.nextBackoffMs = Math.min(entry.nextBackoffMs * 2, MAX_BACKOFF_MS);
    }

    const ipKey = normalizeIp(ip);
    const stamps = (byIp.get(ipKey) || []).filter((ts) => t - ts < IP_WINDOW_MS);
    stamps.push(t);
    byIp.set(ipKey, stamps);
}

/**
 * Clear username failure state after a successful sign-in.
 * @param {string} username
 */
function clear(username) {
    byUsername.delete(normalizeUsername(username));
}

/** Test helper — wipe all in-memory counters. */
function reset() {
    byUsername.clear();
    byIp.clear();
}

/** Test helper — inspect username entry. */
function _getUsernameState(username) {
    return byUsername.get(normalizeUsername(username)) || null;
}

module.exports = {
    check,
    recordFailure,
    clear,
    reset,
    _getUsernameState,
    MAX_USERNAME_FAILURES,
    INITIAL_BACKOFF_MS,
    MAX_BACKOFF_MS,
    IP_CAP,
    IP_WINDOW_MS
};
