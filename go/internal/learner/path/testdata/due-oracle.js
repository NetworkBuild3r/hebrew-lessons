'use strict';

// Due-count oracle. Go tests compare DueCount to this output.
// localDayNumber and dueInDays are the functions in server/learner/model.js.
// The script checks them against model.wordStates before it counts.
// A suspended row is excluded by the row, not by the word-level flag.
// timestampCount uses shared/player/review.js isDue (due <= now).
//
// usage: TZ=<zone> node due-oracle.js <data-dir> <cases.jsonl>
// stdout: one compact JSON object per case whose tz equals TZ.

const fs = require('fs');
const path = require('path');

const dataDirArg = process.argv[2];
const casesArg = process.argv[3];
if (!dataDirArg || !casesArg) {
    console.error('usage: due-oracle.js <data-dir> <cases.jsonl>');
    process.exit(2);
}

const repoRoot = path.resolve(__dirname, '../../../../..');
const dataDir = path.resolve(dataDirArg);
process.env.HEBREW_ROOT = repoRoot;
process.env.DATA_DIR = dataDir;
process.env.LESSONS_DIR = path.join(repoRoot, 'worksheets');

const db = require(path.join(repoRoot, 'server', 'db'));
const model = require(path.join(repoRoot, 'server', 'learner', 'model'));
const Review = require(path.join(repoRoot, 'shared', 'player', 'review.js'));

const MS_PER_DAY = 24 * 60 * 60 * 1000;
const zone = process.env.TZ || 'UTC';

function localDayNumber(t) {
    const d = new Date(t);
    return Math.round(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / MS_PER_DAY);
}

function dueInDays(due, nowMs) {
    if (due == null || due === '') return null;
    const dueMs = Date.parse(String(due));
    if (!Number.isFinite(dueMs) || !Number.isFinite(nowMs)) return null;
    return localDayNumber(dueMs) - localDayNumber(nowMs);
}

const text = fs.readFileSync(casesArg, 'utf8');
const cases = text.split('\n').filter((line) => line.trim() !== '').map((line) => JSON.parse(line));

for (const item of cases) {
    if (item.tz !== zone) continue;
    const user = db.get('SELECT id FROM users WHERE username = ?', item.username);
    if (!user) {
        console.error('missing user ' + item.username);
        process.exit(1);
    }
    const nowMs = Date.parse(item.now);
    if (!Number.isFinite(nowMs)) {
        console.error('bad now ' + item.now);
        process.exit(1);
    }
    const states = model.wordStates(user.id, nowMs);
    const byWord = new Map(states.map((entry) => [entry.word, entry]));
    const rows = db.all(
        `SELECT word, direction, due, suspended
         FROM review_items WHERE user_id = ? ORDER BY id ASC`,
        user.id
    );
    let count = 0;
    let timestampCount = 0;
    let wordLevelCount = 0;
    for (const entry of states) {
        if (entry.suspended) continue;
        for (const direction of ['he_en', 'en_he']) {
            const part = entry[direction];
            if (part && part.due_in_days != null && part.due_in_days <= 0) wordLevelCount += 1;
        }
    }
    for (const row of rows) {
        const days = dueInDays(row.due, nowMs);
        const state = byWord.get(String(row.word));
        const part = state && state[row.direction];
        const modelDays = part ? part.due_in_days : undefined;
        if (!state || modelDays !== days) {
            console.error('due_in_days mismatch for ' + item.id + ' ' + row.word + ' ' + row.direction
                + ' copy=' + days + ' model=' + modelDays);
            process.exit(1);
        }
        const suspended = Number(row.suspended) === 1;
        if (!suspended && days != null && days <= 0) count += 1;
        if (!suspended && Review.isDue({ due: row.due }, nowMs)) timestampCount += 1;
    }
    process.stdout.write(JSON.stringify({
        id: item.id,
        tz: zone,
        node: process.version,
        count: count,
        timestampCount: timestampCount,
        wordLevelCount: wordLevelCount,
        nowMs: nowMs,
        nowDay: localDayNumber(nowMs)
    }) + '\n');
}

db.close();
