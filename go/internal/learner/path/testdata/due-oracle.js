'use strict';

// Review-page due count. cards.deck(user, 'due', now) is what
// GET /api/me/cards?deck=due returns, and the Review page shows
// cards.length. instantCount is the uncapped isDue row count.

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
const cards = require(path.join(repoRoot, 'server', 'learner', 'cards'));
const Review = require(path.join(repoRoot, 'shared', 'player', 'review.js'));

const text = fs.readFileSync(casesArg, 'utf8');
const cases = text.split('\n').filter((line) => line.trim() !== '').map((line) => JSON.parse(line));

for (const item of cases) {
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
    const deck = cards.deck(user.id, 'due', nowMs);
    if (!deck.ok) {
        console.error('deck ' + item.id + ' ' + deck.error);
        process.exit(1);
    }
    const rows = db.all(
        `SELECT word, direction, due, suspended
         FROM review_items WHERE user_id = ? ORDER BY id ASC`,
        user.id
    );
    let instantCount = 0;
    for (const row of rows) {
        if (Number(row.suspended) === 1) continue;
        if (row.direction !== 'he_en' && row.direction !== 'en_he') continue;
        if (Review.isDue({ due: row.due }, nowMs)) instantCount += 1;
    }
    process.stdout.write(JSON.stringify({
        id: item.id,
        now: item.now,
        username: item.username,
        node: process.version,
        unicode: process.versions.unicode,
        count: deck.cards.length,
        instantCount: instantCount,
        cards: deck.cards.map((card) => ({ word: card.word, direction: card.direction }))
    }) + '\n');
}

db.close();
