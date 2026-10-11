#!/usr/bin/env node
/**
 * Print per-user new / learning / longTerm review counts per direction.
 * Usernames only — no password hashes, tokens, or other PII.
 *
 *   node scripts/stage-counts.js
 *
 * Reads SRS_SCHEDULER once (ladder default; fsrs when set). Before/after
 * migration comparisons must use the same scheduler. Opens the database
 * read-only and does not migrate.
 *
 * Provenance: INIT-006/SPEC-020
 */
'use strict';

const path = require('path');
const db = require('../server/db');

const LONG_TERM_STABILITY_DAYS = 14;
const DIRECTIONS = ['he_en', 'en_he'];

/**
 * @param {string|undefined} raw
 * @returns {'ladder'|'fsrs'}
 */
function resolveScheduler(raw) {
    if (raw == null || String(raw).trim() === '') return 'ladder';
    const value = String(raw).trim().toLowerCase();
    if (value === 'fsrs' || value === 'ladder') return value;
    console.warn('[stage-counts] SRS_SCHEDULER ' + JSON.stringify(String(raw))
        + ' is not fsrs or ladder; using ladder');
    return 'ladder';
}

/**
 * Same rules as server/progress.js stageFor.
 * @param {{ step?: number, stability?: number|null }} dir
 * @param {boolean} everPassed
 * @param {'ladder'|'fsrs'} scheduler
 */
function stageFor(dir, everPassed, scheduler) {
    if (!everPassed) return 'new';
    if (scheduler === 'fsrs') {
        const stability = dir && Number.isFinite(dir.stability) ? dir.stability : 0;
        return stability >= LONG_TERM_STABILITY_DAYS ? 'longTerm' : 'learning';
    }
    const step = dir && Number.isInteger(dir.step) ? dir.step : 0;
    if (step >= 3) return 'longTerm';
    return 'learning';
}

function emptyStages() {
    return { new: 0, learning: 0, longTerm: 0 };
}

/** Ladder intervals used when fsrs needs a backfilled stability (progress.js). */
function ladderBackfillStability(step) {
    const ladder = [1, 3, 7, 14, 30];
    if (Number.isInteger(step) && step >= 0) {
        return ladder[Math.min(step, ladder.length - 1)];
    }
    return ladder[0];
}

/**
 * @param {import('node:sqlite').DatabaseSync} conn
 * @param {string} table
 * @param {string} column
 * @returns {boolean}
 */
function hasColumn(conn, table, column) {
    const cols = conn.prepare('PRAGMA table_info(' + table + ')').all();
    return cols.some((row) => row.name === column);
}

/**
 * @param {import('node:sqlite').DatabaseSync} conn
 * @param {number} userId
 * @param {'ladder'|'fsrs'} scheduler
 * @param {{ withStability: boolean, withKind: boolean, withLastGrade: boolean }} flags
 */
function stagesForUser(conn, userId, scheduler, flags) {
    const passSql = flags.withKind
        ? `SELECT DISTINCT word, direction FROM practice_events
           WHERE user_id = ? AND result = 'pass' AND kind = 'word'`
        : `SELECT DISTINCT word, direction FROM practice_events
           WHERE user_id = ? AND result = 'pass'`;
    const passedRows = conn.prepare(passSql).all(userId);
    const passed = Object.create(null);
    for (const row of passedRows) {
        passed[row.word + '\0' + row.direction] = true;
    }

    const selectCols = ['word', 'direction', 'step', 'last_result'];
    if (flags.withLastGrade) selectCols.push('last_grade');
    if (flags.withStability) selectCols.push('stability');
    const items = conn.prepare(
        'SELECT ' + selectCols.join(', ') + ' FROM review_items WHERE user_id = ?'
    ).all(userId);

    const stages = {
        he_en: emptyStages(),
        en_he: emptyStages()
    };
    for (const row of items) {
        const dir = row.direction;
        if (!stages[dir]) continue;
        const ever = Boolean(
            passed[row.word + '\0' + dir]
            || row.last_result === 'pass'
            || (Number(row.step) >= 1)
            || (flags.withLastGrade && Number(row.last_grade) >= 2)
        );
        const record = {
            step: row.step == null ? 0 : Number(row.step)
        };
        if (flags.withStability && row.stability != null) {
            record.stability = Number(row.stability);
        } else if (scheduler === 'fsrs' && ever) {
            record.stability = ladderBackfillStability(record.step);
        }
        const bucket = stageFor(record, ever, scheduler);
        stages[dir][bucket] += 1;
    }
    return stages;
}

function formatLine(username, stages) {
    const parts = [username];
    for (const dir of DIRECTIONS) {
        const s = stages[dir];
        parts.push(
            dir + ': new=' + s.new
                + ' learning=' + s.learning
                + ' longTerm=' + s.longTerm
        );
    }
    return parts.join(' | ');
}

function main() {
    const scheduler = resolveScheduler(process.env.SRS_SCHEDULER);
    const file = db.databasePath();
    const conn = db.openReadOnly(file);
    try {
        const flags = {
            withStability: hasColumn(conn, 'review_items', 'stability'),
            withKind: hasColumn(conn, 'practice_events', 'kind'),
            withLastGrade: hasColumn(conn, 'review_items', 'last_grade')
        };
        if (scheduler === 'fsrs' && !flags.withStability) {
            throw new Error(
                'SRS_SCHEDULER=fsrs needs FSRS columns; migrate first or use ladder'
            );
        }
        const users = conn.prepare(
            'SELECT id, username FROM users ORDER BY username ASC'
        ).all();
        console.log('scheduler=' + scheduler);
        console.log('database=' + path.basename(file));
        console.log('users=' + users.length);
        for (const user of users) {
            const stages = stagesForUser(conn, user.id, scheduler, flags);
            console.log(formatLine(user.username, stages));
        }
    } finally {
        conn.close();
    }
}

try {
    main();
} catch (err) {
    console.error(err && err.message ? err.message : err);
    process.exit(1);
}
