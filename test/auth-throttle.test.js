/**
 * Sign-in throttle tests.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const throttle = require('../server/auth/throttle');

describe('auth/throttle (ac-3)', () => {
    const t0 = Date.parse('2026-04-01T10:00:00.000Z');

    beforeEach(() => {
        throttle.reset();
    });

    it('allows attempts until configured failures, then locks even with right password path', () => {
        const username = 'alice';
        const ip = '10.0.0.1';
        for (let i = 0; i < throttle.MAX_USERNAME_FAILURES; i++) {
            assert.equal(throttle.check(username, ip, t0 + i).ok, true);
            throttle.recordFailure(username, ip, t0 + i);
        }
        const locked = throttle.check(username, ip, t0 + throttle.MAX_USERNAME_FAILURES);
        assert.equal(locked.ok, false);
        assert.match(locked.reason, /try again later/i);
        assert.ok(locked.retryAfterMs > 0);
        assert.ok(locked.retryAfterMs <= throttle.INITIAL_BACKOFF_MS);

        // Still locked from a different IP — username lock applies globally
        const otherIp = throttle.check(username, '10.0.0.99', t0 + 1000);
        assert.equal(otherIp.ok, false);
    });

    it('different IP failures still count against the username', () => {
        const username = 'bob';
        for (let i = 0; i < throttle.MAX_USERNAME_FAILURES; i++) {
            throttle.recordFailure(username, '1.1.1.' + i, t0 + i * 1000);
        }
        const locked = throttle.check(username, '9.9.9.9', t0 + 10_000);
        assert.equal(locked.ok, false);
    });

    it('backoff doubles up to 15 minute lockout', () => {
        const username = 'carol';
        const ip = '10.0.0.2';
        let now = t0;

        // First lockout at 30s
        for (let i = 0; i < throttle.MAX_USERNAME_FAILURES; i++) {
            throttle.recordFailure(username, ip, now);
            now += 1;
        }
        let state = throttle._getUsernameState(username);
        assert.equal(state.lockedUntil, t0 + throttle.MAX_USERNAME_FAILURES - 1 + throttle.INITIAL_BACKOFF_MS);
        assert.equal(state.nextBackoffMs, throttle.INITIAL_BACKOFF_MS * 2);

        // Unlock window ends; another failure wave → 60s
        now = state.lockedUntil + 1;
        for (let i = 0; i < throttle.MAX_USERNAME_FAILURES; i++) {
            // After unlock, failures continue from prior count (>=5), each failure re-locks
            // Our implementation locks whenever failures >= 5, so one failure after unlock re-locks.
            throttle.recordFailure(username, ip, now);
            now += 1;
            break;
        }
        state = throttle._getUsernameState(username);
        assert.equal(state.nextBackoffMs, throttle.INITIAL_BACKOFF_MS * 4);

        // Drive nextBackoffMs to the 15 min cap
        while (state.nextBackoffMs < throttle.MAX_BACKOFF_MS) {
            now = state.lockedUntil + 1;
            throttle.recordFailure(username, ip, now);
            state = throttle._getUsernameState(username);
        }
        assert.equal(state.nextBackoffMs, throttle.MAX_BACKOFF_MS);
    });

    it('per-IP cap of 20 failures / 15 min', () => {
        const ip = '203.0.113.10';
        for (let i = 0; i < throttle.IP_CAP; i++) {
            throttle.recordFailure('user' + i, ip, t0 + i);
        }
        const blocked = throttle.check('freshuser', ip, t0 + throttle.IP_CAP);
        assert.equal(blocked.ok, false);
        assert.match(blocked.reason, /try again later/i);
    });

    it('clear removes username lock after success', () => {
        const username = 'dave';
        for (let i = 0; i < throttle.MAX_USERNAME_FAILURES; i++) {
            throttle.recordFailure(username, '10.0.0.3', t0 + i);
        }
        assert.equal(throttle.check(username, '10.0.0.3', t0 + 100).ok, false);
        throttle.clear(username);
        assert.equal(throttle.check(username, '10.0.0.3', t0 + 100).ok, true);
    });
});
