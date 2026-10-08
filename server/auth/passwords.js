/**
 * Password hashing (scrypt) and policy checks — ADR-0002 §3.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const crypto = require('crypto');

const N = 1 << 15; // 2^15
const R = 8;
const P = 3;
const SALT_BYTES = 16;
const KEY_BYTES = 32;
const MAXMEM = 64 * 1024 * 1024;

const MIN_LENGTH = 10;
const MAX_LENGTH = 200;

/** Short common-password deny list (exact match, case-insensitive). */
const COMMON_PASSWORDS = new Set([
    'password',
    'password1',
    'password12',
    'password123',
    '1234567890',
    '123456789',
    'qwertyuiop',
    'abcdefghij',
    'letmein123',
    'welcome123',
    'adminadmin',
    'iloveyou12',
    'monkey1234',
    'dragon1234',
    'master1234',
    'loginlogin',
    'abc1234567',
    'passw0rd12',
    'changeme12',
    'hebrew1234'
]);

const DUMMY_PASSWORD = 'dummy-password-for-timing-only-never-used';
let cachedDummyHash = null;

/**
 * Self-describing stored format:
 * scrypt$N=<n>$r=<r>$p=<p>$<salt_b64url>$<key_b64url>
 */
function formatStored(n, r, p, salt, key) {
    return [
        'scrypt',
        'N=' + n,
        'r=' + r,
        'p=' + p,
        salt.toString('base64url'),
        key.toString('base64url')
    ].join('$');
}

function parseStored(stored) {
    if (typeof stored !== 'string') return null;
    const parts = stored.split('$');
    if (parts.length !== 6 || parts[0] !== 'scrypt') return null;
    const nMatch = /^N=(\d+)$/.exec(parts[1]);
    const rMatch = /^r=(\d+)$/.exec(parts[2]);
    const pMatch = /^p=(\d+)$/.exec(parts[3]);
    if (!nMatch || !rMatch || !pMatch) return null;
    let salt;
    let key;
    try {
        salt = Buffer.from(parts[4], 'base64url');
        key = Buffer.from(parts[5], 'base64url');
    } catch (_) {
        return null;
    }
    if (salt.length === 0 || key.length === 0) return null;
    return {
        N: Number(nMatch[1]),
        r: Number(rMatch[1]),
        p: Number(pMatch[1]),
        salt,
        key
    };
}

function derive(password, salt, n, r, p, keyLen) {
    return crypto.scryptSync(password, salt, keyLen, {
        N: n,
        r,
        p,
        maxmem: MAXMEM
    });
}

/**
 * @param {string} password
 * @returns {string} self-describing scrypt hash
 */
function hash(password) {
    if (typeof password !== 'string') {
        throw new Error('Password must be a string');
    }
    const salt = crypto.randomBytes(SALT_BYTES);
    const key = derive(password, salt, N, R, P, KEY_BYTES);
    return formatStored(N, R, P, salt, key);
}

/**
 * Timing-safe verify against a stored self-describing hash.
 * @param {string} password
 * @param {string} stored
 * @returns {boolean}
 */
function verify(password, stored) {
    const parsed = parseStored(stored);
    if (!parsed || typeof password !== 'string') {
        // Burn comparable work against the dummy so callers stay constant-time-ish.
        try {
            derive(String(password || ''), crypto.randomBytes(SALT_BYTES), N, R, P, KEY_BYTES);
        } catch (_) { /* ignore */ }
        return false;
    }
    const derived = derive(password, parsed.salt, parsed.N, parsed.r, parsed.p, parsed.key.length);
    if (derived.length !== parsed.key.length) return false;
    return crypto.timingSafeEqual(derived, parsed.key);
}

/**
 * @param {string} password
 * @returns {{ ok: true } | { ok: false, reason: string }}
 */
function policyCheck(password) {
    if (typeof password !== 'string') {
        return { ok: false, reason: 'Password must be a string' };
    }
    if (password.length < MIN_LENGTH) {
        return { ok: false, reason: 'Password must be at least ' + MIN_LENGTH + ' characters' };
    }
    if (password.length > MAX_LENGTH) {
        return { ok: false, reason: 'Password must be at most ' + MAX_LENGTH + ' characters' };
    }
    if (COMMON_PASSWORDS.has(password.toLowerCase())) {
        return { ok: false, reason: 'Password is too common' };
    }
    return { ok: true };
}

/** Precomputed (lazily) dummy hash for unknown-user timing. */
function dummyHash() {
    if (!cachedDummyHash) {
        cachedDummyHash = hash(DUMMY_PASSWORD);
    }
    return cachedDummyHash;
}

module.exports = {
    hash,
    verify,
    policyCheck,
    dummyHash,
    parseStored,
    N,
    R,
    P,
    SALT_BYTES,
    KEY_BYTES,
    MIN_LENGTH,
    MAX_LENGTH,
    COMMON_PASSWORDS
};
