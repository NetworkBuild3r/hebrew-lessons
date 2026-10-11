/**
 * First admin comes from the environment only when the database has none.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');
const PASS = 'bootstrap-test-pass-1';
const OTHER = 'bootstrap-test-pass-2';

function clearServerModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/repos/users',
        '../server/db/repos/audit',
        '../server/auth/passwords',
        '../server/auth/sessions',
        '../server/auth/throttle',
        '../server/auth/policy',
        '../server/accounts',
        '../server/bootstrap-admin'
    ];
    for (const name of names) {
        try { delete require.cache[require.resolve(name)]; } catch (_) { /* not loaded */ }
    }
}

describe('bootstrap admin from env', () => {
    let tmp;
    let priorData;
    let priorRoot;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-bootstrap-'));
        priorData = process.env.DATA_DIR;
        priorRoot = process.env.HEBREW_ROOT;
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        clearServerModules();
        const db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
    });

    after(() => {
        try { require('../server/db').close(); } catch (_) { /* ignore */ }
        clearServerModules();
        if (priorData === undefined) delete process.env.DATA_DIR;
        else process.env.DATA_DIR = priorData;
        if (priorRoot === undefined) delete process.env.HEBREW_ROOT;
        else process.env.HEBREW_ROOT = priorRoot;
        fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 10, retryDelay: 50 });
    });

    function fresh() {
        // Close the open handle before dropping the module, or Windows keeps the file locked.
        try { require('../server/db').close(); } catch (_) { /* not loaded yet */ }
        clearServerModules();
        const db = require('../server/db');
        db.open({ force: true });
        return {
            bootstrap: require('../server/bootstrap-admin'),
            passwords: require('../server/auth/passwords'),
            users: require('../server/db/repos/users'),
            accounts: require('../server/accounts')
        };
    }

    it('creates an admin from env when the database has none', () => {
        const { bootstrap, passwords, users } = fresh();
        const result = bootstrap.ensureBootstrapAdmin({
            ADMIN_USERNAME: 'Brian',
            ADMIN_PASSWORD: PASS,
            ADMIN_DISPLAY_NAME: 'Brian'
        });
        assert.deepEqual(result, { created: true, username: 'brian' });
        assert.equal(Object.hasOwn(result, 'password'), false);
        const row = users.findByUsername('brian');
        assert.equal(row.role, 'admin');
        assert.equal(row.display_name, 'Brian');
        assert.equal(row.must_change_password, 0);
        assert.equal(passwords.verify(PASS, row.password_hash), true);
    });

    it('ignores env once an admin exists, even if the password differs', () => {
        const { bootstrap, passwords, users } = fresh();
        const result = bootstrap.ensureBootstrapAdmin({
            ADMIN_USERNAME: 'someoneelse',
            ADMIN_PASSWORD: OTHER,
            ADMIN_DISPLAY_NAME: 'Other'
        });
        assert.deepEqual(result, { created: false });
        assert.equal(users.listAll().length, 1);
        const row = users.findByUsername('brian');
        assert.equal(passwords.verify(PASS, row.password_hash), true);
        assert.equal(passwords.verify(OTHER, row.password_hash), false);
        assert.equal(users.findByUsername('someoneelse'), undefined);
    });

    it('refuses to start a database that has no admin and no env', () => {
        const { bootstrap, users } = fresh();
        users.deleteById(users.findByUsername('brian').id);
        assert.throws(
            () => bootstrap.ensureBootstrapAdmin({}),
            (err) => err.code === 'bootstrap_admin_missing'
        );
        assert.equal(users.listAll().length, 0);
    });

    it('does not create a user when the password fails policy', () => {
        const { bootstrap, users } = fresh();
        assert.throws(
            () => bootstrap.ensureBootstrapAdmin({
                ADMIN_USERNAME: 'ada',
                ADMIN_PASSWORD: 'short'
            }),
            (err) => err.code === 'password_policy'
        );
        assert.equal(users.findByUsername('ada'), undefined);
    });

    it('startup logs the username and not the password value', () => {
        const start = fs.readFileSync(path.join(root, 'server', 'start.js'), 'utf8');
        assert.match(start, /ensureBootstrapAdmin/);
        assert.match(start, /boot\.username/);
        assert.doesNotMatch(start, /ADMIN_PASSWORD|boot\.password|process\.env\.ADMIN/);
    });
});
