#!/usr/bin/env node
/**
 * User CLI — create / reset / disable / enable / role / list.
 * Password is never taken from argv or env (ADR-0002 / REQ-001).
 * Provenance: INIT-002/SPEC-004
 */
'use strict';

const fs = require('fs');
const path = require('path');
const readline = require('readline');

const db = require('../server/db');
const accounts = require('../server/accounts');
const usersRepo = require('../server/db/repos/users');

const ACTOR = 'cli';

function usage() {
    console.error([
        'Usage: npm run user -- <command> [args]',
        '',
        'Commands:',
        '  create <username> [--admin] [--display-name <name>] [--password-file <path>]',
        '  reset <username> [--password-file <path>]',
        '  disable <username>',
        '  enable <username>',
        '  role <username> user|admin',
        '  list',
        '',
        'Password is read from a no-echo prompt, or from --password-file.',
        '--password= on argv is rejected.'
    ].join('\n'));
}

/**
 * Reject any argv form that places the password on the command line.
 * @param {string[]} argv
 */
function assertNoPasswordArgv(argv) {
    for (const arg of argv) {
        if (arg === '--password' || arg.startsWith('--password=')) {
            throw new Error(
                'Refusing --password on argv. Use a no-echo prompt or --password-file.'
            );
        }
    }
}

/**
 * @param {string[]} argv
 * @returns {Record<string, string|boolean>}
 */
function parseFlags(argv) {
    const flags = {};
    for (let i = 0; i < argv.length; i++) {
        const arg = argv[i];
        if (arg === '--admin') {
            flags.admin = true;
            continue;
        }
        if (arg === '--password-file' || arg.startsWith('--password-file=')) {
            const value = arg.startsWith('--password-file=')
                ? arg.slice('--password-file='.length)
                : argv[++i];
            if (!value) throw new Error('--password-file requires a path');
            flags.passwordFile = value;
            continue;
        }
        if (arg === '--display-name' || arg.startsWith('--display-name=')) {
            const value = arg.startsWith('--display-name=')
                ? arg.slice('--display-name='.length)
                : argv[++i];
            if (value == null) throw new Error('--display-name requires a value');
            flags.displayName = value;
            continue;
        }
        if (arg.startsWith('-')) {
            throw new Error('Unknown flag: ' + arg);
        }
    }
    return flags;
}

/**
 * Positional args after the command (excluding flags and their values).
 * @param {string[]} argv
 * @returns {string[]}
 */
function positionals(argv) {
    const out = [];
    for (let i = 0; i < argv.length; i++) {
        const arg = argv[i];
        if (arg === '--admin') continue;
        if (arg === '--password-file' || arg === '--display-name') {
            i++;
            continue;
        }
        if (arg.startsWith('--password-file=') || arg.startsWith('--display-name=')) {
            continue;
        }
        if (arg.startsWith('-')) continue;
        out.push(arg);
    }
    return out;
}

/**
 * Read password from file or no-echo TTY prompt.
 * @param {{ passwordFile?: string, prompt?: string }} opts
 * @returns {Promise<string>}
 */
async function readPassword(opts) {
    const options = opts || {};
    if (options.passwordFile) {
        const filePath = path.resolve(options.passwordFile);
        const raw = fs.readFileSync(filePath, 'utf8');
        const password = raw.replace(/\r?\n$/, '');
        if (!password) throw new Error('Password file is empty');
        return password;
    }
    return promptNoEcho(options.prompt || 'Password: ');
}

/**
 * @param {string} promptText
 * @returns {Promise<string>}
 */
function promptNoEcho(promptText) {
    return new Promise((resolve, reject) => {
        const stdin = process.stdin;
        const stdout = process.stdout;
        if (!stdin.isTTY || typeof stdin.setRawMode !== 'function') {
            // Non-TTY fallback (tests should use --password-file).
            const rl = readline.createInterface({ input: stdin, output: stdout });
            rl.question(promptText, (answer) => {
                rl.close();
                resolve(answer);
            });
            rl.on('error', reject);
            return;
        }

        stdout.write(promptText);
        stdin.resume();
        stdin.setRawMode(true);
        stdin.setEncoding('utf8');
        let password = '';
        function onData(chunk) {
            const s = String(chunk);
            if (s === '\n' || s === '\r' || s === '\u0004') {
                stdin.setRawMode(false);
                stdin.pause();
                stdin.removeListener('data', onData);
                stdout.write('\n');
                resolve(password);
                return;
            }
            if (s === '\u0003') {
                stdin.setRawMode(false);
                stdin.pause();
                stdin.removeListener('data', onData);
                stdout.write('\n');
                reject(new Error('Interrupted'));
                return;
            }
            if (s === '\u007f' || s === '\b') {
                password = password.slice(0, -1);
                return;
            }
            password += s;
        }
        stdin.on('data', onData);
    });
}

function requireUser(username) {
    const row = usersRepo.findByUsername(username);
    if (!row) throw new Error('User not found: ' + username);
    return row;
}

/**
 * @param {string[]} argv process.argv.slice(2)
 * @returns {Promise<number>} exit code
 */
async function main(argv) {
    assertNoPasswordArgv(argv);
    const cmd = argv[0];
    if (!cmd) {
        usage();
        return 1;
    }
    const rest = argv.slice(1);
    const flags = parseFlags(rest);
    const args = positionals(rest);

    db.open();
    db.migrate();

    try {
        if (cmd === 'create') {
            const username = args[0];
            if (!username) throw new Error('create requires <username>');
            const password = await readPassword({ passwordFile: flags.passwordFile });
            const user = accounts.createUser({
                username,
                password,
                role: flags.admin ? 'admin' : 'user',
                displayName: flags.displayName || null,
                actor: ACTOR
            });
            console.log('Created ' + user.username + ' (' + user.role + ', ' + user.status + ')');
            return 0;
        }

        if (cmd === 'reset') {
            const username = args[0];
            if (!username) throw new Error('reset requires <username>');
            const row = requireUser(username);
            const password = await readPassword({ passwordFile: flags.passwordFile });
            accounts.resetPassword(row.id, password, ACTOR);
            console.log('Password reset for ' + row.username + ' (must change on next sign-in)');
            return 0;
        }

        if (cmd === 'disable') {
            const username = args[0];
            if (!username) throw new Error('disable requires <username>');
            const row = requireUser(username);
            accounts.setStatus(row.id, 'disabled', ACTOR);
            console.log('Disabled ' + row.username);
            return 0;
        }

        if (cmd === 'enable') {
            const username = args[0];
            if (!username) throw new Error('enable requires <username>');
            const row = requireUser(username);
            accounts.setStatus(row.id, 'active', ACTOR);
            console.log('Enabled ' + row.username);
            return 0;
        }

        if (cmd === 'role') {
            const username = args[0];
            const role = args[1];
            if (!username || (role !== 'user' && role !== 'admin')) {
                throw new Error('role requires <username> user|admin');
            }
            const row = requireUser(username);
            accounts.setRole(row.id, role, ACTOR);
            console.log('Set role of ' + row.username + ' to ' + role);
            return 0;
        }

        if (cmd === 'list') {
            const rows = accounts.listUsers();
            for (const u of rows) {
                console.log([
                    u.username,
                    u.role,
                    u.status,
                    u.display_name || '',
                    u.must_change_password ? 'must_change_password' : ''
                ].join('\t'));
            }
            return 0;
        }

        usage();
        return 1;
    } finally {
        try { db.close(); } catch (_) { /* ignore */ }
    }
}

module.exports = {
    main,
    assertNoPasswordArgv,
    parseFlags,
    positionals,
    readPassword,
    usage
};

if (require.main === module) {
    main(process.argv.slice(2)).then((code) => {
        process.exit(code);
    }).catch((err) => {
        console.error(err && err.message ? err.message : err);
        try { db.close(); } catch (_) { /* ignore */ }
        process.exit(1);
    });
}
