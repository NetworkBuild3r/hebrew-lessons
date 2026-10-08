/**
 * Password hash / verify / policy tests.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const passwords = require('../server/auth/passwords');

describe('auth/passwords (ac-1)', () => {
    it('stored hash never contains the plaintext password', () => {
        const password = 'unique-secret-99';
        const stored = passwords.hash(password);
        assert.equal(stored.includes(password), false);
        assert.match(stored, /^scrypt\$N=32768\$r=8\$p=3\$/);
    });

    it('same password yields different hashes (unique salts)', () => {
        const password = 'same-password-aa';
        const a = passwords.hash(password);
        const b = passwords.hash(password);
        assert.notEqual(a, b);
        assert.equal(passwords.verify(password, a), true);
        assert.equal(passwords.verify(password, b), true);
    });

    it('verify rejects a one-character change', () => {
        const stored = passwords.hash('correct-horse');
        assert.equal(passwords.verify('correct-horsf', stored), false);
        assert.equal(passwords.verify('correct-horse', stored), true);
    });

    it('stored format records scrypt parameters', () => {
        const stored = passwords.hash('parameter-check1');
        const parsed = passwords.parseStored(stored);
        assert.ok(parsed);
        assert.equal(parsed.N, passwords.N);
        assert.equal(parsed.r, passwords.R);
        assert.equal(parsed.p, passwords.P);
        assert.equal(parsed.salt.length, passwords.SALT_BYTES);
        assert.equal(parsed.key.length, passwords.KEY_BYTES);
    });

    it('policyCheck enforces min/max and deny list', () => {
        assert.equal(passwords.policyCheck('short').ok, false);
        assert.equal(passwords.policyCheck('a'.repeat(201)).ok, false);
        assert.equal(passwords.policyCheck('password123').ok, false);
        assert.equal(passwords.policyCheck('a-good-long-pass').ok, true);
    });
});
