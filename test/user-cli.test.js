/**
 * User CLI tests — password-file create, reject --password=.
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const root = path.resolve(__dirname, '..');

describe('user CLI (ac-6)', () => {
    let tmp;
    let passwordFile;
    let db;
    let accounts;
    let userCli;

    before(() => {
        tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-user-cli-'));
        passwordFile = path.join(tmp, 'pw.txt');
        fs.writeFileSync(passwordFile, 'cli-admin-pass1\n', 'utf8');
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmp;
        for (const mod of [
            '../server/paths',
            '../server/db',
            '../server/db/repos/users',
            '../server/db/repos/sessions',
            '../server/db/repos/audit',
            '../server/auth/passwords',
            '../server/auth/sessions',
            '../server/auth/throttle',
            '../server/auth/policy',
            '../server/accounts',
            '../scripts/user'
        ]) {
            delete require.cache[require.resolve(mod)];
        }
        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        accounts = require('../server/accounts');
        userCli = require('../scripts/user');
    });

    after(() => {
        if (db) db.close();
        fs.rmSync(tmp, { recursive: true, force: true });
        delete process.env.DATA_DIR;
    });

    beforeEach(() => {
        db.run('DELETE FROM sessions');
        db.run('DELETE FROM audit_log');
        db.run('DELETE FROM users');
    });

    it('rejects --password= on argv', () => {
        assert.throws(
            () => userCli.assertNoPasswordArgv(['create', 'alice', '--password=secret']),
            /Refusing --password/
        );
        assert.throws(
            () => userCli.assertNoPasswordArgv(['create', 'alice', '--password', 'secret']),
            /Refusing --password/
        );
    });

    it('create alice --admin --password-file creates active admin', async () => {
        const code = await userCli.main([
            'create',
            'alice',
            '--admin',
            '--password-file',
            passwordFile
        ]);
        assert.equal(code, 0);
        // main closes db; reopen for assertions
        db.open({ force: true });
        const users = accounts.listUsers();
        assert.equal(users.length, 1);
        assert.equal(users[0].username, 'alice');
        assert.equal(users[0].role, 'admin');
        assert.equal(users[0].status, 'active');
        const signed = accounts.signIn('alice', 'cli-admin-pass1', {
            ip: '127.0.0.1',
            now: new Date('2026-06-01T00:00:00.000Z')
        });
        assert.equal(signed.user.role, 'admin');
    });

    it('npm run user -- create rejects --password via child process', () => {
        const result = spawnSync(
            process.execPath,
            [path.join(root, 'scripts/user.js'), 'create', 'eve', '--password=leaked'],
            {
                cwd: root,
                env: Object.assign({}, process.env, {
                    HEBREW_ROOT: root,
                    DATA_DIR: tmp
                }),
                encoding: 'utf8'
            }
        );
        assert.notEqual(result.status, 0);
        assert.match(result.stderr, /Refusing --password/);
    });

    it('role / disable / enable / list / reset via CLI', async () => {
        await userCli.main([
            'create', 'alice', '--admin', '--password-file', passwordFile
        ]);
        db.open({ force: true });
        await userCli.main([
            'create', 'bob', '--password-file', passwordFile
        ]);
        db.open({ force: true });
        let code = await userCli.main(['role', 'bob', 'admin']);
        assert.equal(code, 0);
        db.open({ force: true });
        assert.equal(
            accounts.listUsers().find((u) => u.username === 'bob').role,
            'admin'
        );

        const resetFile = path.join(tmp, 'reset-pw.txt');
        fs.writeFileSync(resetFile, 'reset-pass-999\n', 'utf8');
        code = await userCli.main(['reset', 'bob', '--password-file', resetFile]);
        assert.equal(code, 0);
        db.open({ force: true });
        assert.equal(
            accounts.listUsers().find((u) => u.username === 'bob').must_change_password,
            true
        );

        code = await userCli.main(['disable', 'bob']);
        assert.equal(code, 0);
        db.open({ force: true });
        assert.equal(
            accounts.listUsers().find((u) => u.username === 'bob').status,
            'disabled'
        );

        code = await userCli.main(['enable', 'bob']);
        assert.equal(code, 0);
        db.open({ force: true });
        assert.equal(
            accounts.listUsers().find((u) => u.username === 'bob').status,
            'active'
        );

        code = await userCli.main(['list']);
        assert.equal(code, 0);
    });
});
