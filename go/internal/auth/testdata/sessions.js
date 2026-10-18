'use strict';

// Builds fixture sessions with the Node session code and writes a jsonl file
// Go resolves against. This process does not call sessions.resolve, so
// last_seen_at stays at the inserted value.

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const dataDir = path.resolve(process.argv[2]);
const outPath = path.resolve(process.argv[3]);
if (!process.argv[2] || !process.argv[3]) {
    console.error('usage: sessions.js <data-dir> <out.jsonl>');
    process.exit(2);
}

const repo = path.resolve(__dirname, '../../../..');
process.env.HEBREW_ROOT = repo;
process.env.DATA_DIR = dataDir;
process.env.LESSONS_DIR = path.join(repo, 'worksheets');
process.env.TZ = 'UTC';
fs.mkdirSync(dataDir, { recursive: true });

const db = require(path.join(repo, 'server/db'));
const sessions = require(path.join(repo, 'server/auth/sessions'));
db.migrate();

const now = '2026-06-01T00:00:00.000Z';
const far = '2099-01-01T00:00:00.000Z';

function insertUser(username, role, displayName) {
    const created = db.run(
        `INSERT INTO users (
            username, password_hash, display_name, role, status,
            auth_provider, must_change_password, created_at, updated_at
        ) VALUES (?, 'x', ?, ?, 'active', 'local', 0, ?, ?)`,
        username, displayName, role, now, now
    );
    return Number(created.lastInsertRowid);
}

function publicUser(id) {
    const row = db.get(
        `SELECT id, username, display_name, role, status, auth_provider,
                google_email, must_change_password, created_at, updated_at
         FROM users WHERE id = ?`,
        id
    );
    return {
        id: Number(row.id),
        username: row.username,
        display_name: row.display_name == null ? null : row.display_name,
        role: row.role,
        status: row.status,
        auth_provider: row.auth_provider,
        google_email: row.google_email || null,
        must_change_password: Boolean(row.must_change_password),
        created_at: row.created_at,
        updated_at: row.updated_at
    };
}

function hashToken(token) {
    return crypto.createHash('sha256').update(String(token), 'utf8').digest('hex');
}

function insertToken(userId, token, expiresAt) {
    const created = sessions.create(userId, { now });
    db.run('UPDATE sessions SET expires_at = ? WHERE token_hash = ?', expiresAt, created.tokenHash);
    if (token == null) {
        return created.token;
    }
    const hash = hashToken(token);
    db.run('UPDATE sessions SET token_hash = ? WHERE token_hash = ?', hash, created.tokenHash);
    return token;
}

const learner = insertUser('learner', 'user', 'Dana');
const admin = insertUser('admin', 'admin', null);
const disabled = insertUser('disabled', 'user', 'Off');
db.run("UPDATE users SET status = 'disabled' WHERE id = ?", disabled);

const lines = [];
function emit(row) {
    lines.push(JSON.stringify(row));
}

emit({ kind: 'meta', db: path.join(dataDir, 'hebrew.sqlite'), now });

const cases = [
    { name: 'learner', userId: learner, token: null, expires: far, user: learner },
    { name: 'admin', userId: admin, token: null, expires: far, user: admin },
    { name: 'expired', userId: learner, token: null, expires: '2020-01-01T00:00:00.000Z', user: null },
    { name: 'boundary-eq', userId: learner, token: null, expires: now, user: null },
    { name: 'boundary-after', userId: learner, token: null, expires: '2026-06-01T00:00:00.001Z', user: learner },
    { name: 'disabled', userId: disabled, token: null, expires: far, user: null },
    { name: 'unicode', userId: learner, token: 'tok-\u05e9\u05dc\u05d5\u05dd-\u{1F600}', expires: far, user: learner },
    // Nikud (qamats, shin-dot, patah), cantillation (etnachta), emoji, and a non-emoji astral (𝄞).
    { name: 'astral', userId: learner, token: 'tok-\u05E9\u05B8\u05C1\u05DE\u05B7\u0597\u05E2-\u{1F600}-\u{1D11E}', expires: far, user: learner }
];

for (const item of cases) {
    const token = insertToken(item.userId, item.token, item.expires);
    const tokenHash = hashToken(token);
    const seen = db.get('SELECT last_seen_at FROM sessions WHERE token_hash = ?', tokenHash);
    emit({
        kind: 'case',
        name: item.name,
        token,
        tokenHash,
        lastSeenAt: seen.last_seen_at,
        user: item.user == null ? null : publicUser(item.user)
    });
}

emit({
    kind: 'case',
    name: 'garbage',
    token: 'not-a-real-session-token',
    tokenHash: hashToken('not-a-real-session-token'),
    lastSeenAt: null,
    user: null
});
emit({
    kind: 'case',
    name: 'empty',
    token: '',
    tokenHash: '',
    lastSeenAt: null,
    user: null
});

db.get('PRAGMA wal_checkpoint(TRUNCATE)');
db.close();
fs.writeFileSync(outPath, lines.join('\n') + '\n');
