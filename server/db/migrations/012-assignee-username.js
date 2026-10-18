/**
 * Relax plans.assignee_id to the users-table username rule.
 *
 * Migration 002 CHECK required 8–64 characters from [a-z0-9-]. The users
 * table (001-initial.js) only requires username = lower(username). Account
 * lookup already caps assigneeId at 64 characters. This migration keeps that
 * cap, drops the 8-character minimum and the [a-z0-9-] class, and requires
 * assignee_id = lower(assignee_id), the same CHECK the users table uses.
 *
 * SQLite cannot ALTER a CHECK. The table is rebuilt. plan_revisions and
 * plan_field_marks reference plans(id) ON DELETE CASCADE, and migrate()
 * runs up() inside a transaction, where PRAGMA foreign_keys cannot be
 * turned off. Child rows are parked, the children are dropped, plans is
 * rebuilt, and the children are recreated from the SQL sqlite_master
 * already stored. No plan, revision, or field-mark row is deleted.
 *
 * schemaVersion is unchanged. The pattern relaxation is additive: every
 * assignee id 002 accepted still passes.
 * Provenance: assigneeId plan-document schema relaxation.
 */
'use strict';

const VERSION = 12;

const CREATE_PLANS = `
    CREATE TABLE plans_new (
        id TEXT PRIMARY KEY,
        revision INTEGER NOT NULL DEFAULT 1
            CHECK (revision >= 1 AND revision <= 1000000),
        status TEXT NOT NULL DEFAULT 'draft'
            CHECK (status IN ('draft', 'published')),
        audience TEXT NOT NULL
            CHECK (audience IN ('shared', 'personal')),
        assignee_id TEXT,
        working_copy TEXT NOT NULL,
        snapshot TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        CHECK (length(id) BETWEEN 1 AND 64),
        CHECK (id NOT GLOB '*[^a-z0-9-]*'),
        CHECK (id NOT GLOB '-*'),
        CHECK (id NOT GLOB '*-'),
        CHECK (id NOT GLOB '*--*'),
        CHECK (length(working_copy) > 0),
        CHECK (snapshot IS NULL OR length(snapshot) > 0),
        CHECK (
            assignee_id IS NULL
            OR (
                length(assignee_id) BETWEEN 1 AND 64
                AND assignee_id = lower(assignee_id)
            )
        ),
        CHECK (
            (audience = 'shared' AND assignee_id IS NULL)
            OR (audience = 'personal' AND assignee_id IS NOT NULL)
        )
    );
`;

const CREATE_PLANS_OLD = `
    CREATE TABLE plans_new (
        id TEXT PRIMARY KEY,
        revision INTEGER NOT NULL DEFAULT 1
            CHECK (revision >= 1 AND revision <= 1000000),
        status TEXT NOT NULL DEFAULT 'draft'
            CHECK (status IN ('draft', 'published')),
        audience TEXT NOT NULL
            CHECK (audience IN ('shared', 'personal')),
        assignee_id TEXT,
        working_copy TEXT NOT NULL,
        snapshot TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        CHECK (length(id) BETWEEN 1 AND 64),
        CHECK (id NOT GLOB '*[^a-z0-9-]*'),
        CHECK (id NOT GLOB '-*'),
        CHECK (id NOT GLOB '*-'),
        CHECK (id NOT GLOB '*--*'),
        CHECK (length(working_copy) > 0),
        CHECK (snapshot IS NULL OR length(snapshot) > 0),
        CHECK (
            assignee_id IS NULL
            OR (
                length(assignee_id) BETWEEN 8 AND 64
                AND assignee_id GLOB '[a-z0-9]*'
                AND assignee_id NOT GLOB '*[^a-z0-9-]*'
            )
        ),
        CHECK (
            (audience = 'shared' AND assignee_id IS NULL)
            OR (audience = 'personal' AND assignee_id IS NOT NULL)
        )
    );
`;

const PLAN_COLUMNS = [
    'id', 'revision', 'status', 'audience', 'assignee_id',
    'working_copy', 'snapshot', 'created_at', 'updated_at'
];

/**
 * @param {import('node:sqlite').DatabaseSync} db
 * @param {string} name
 * @returns {string}
 */
function tableSql(db, name) {
    const row = db.prepare(
        "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?"
    ).get(name);
    return row && row.sql ? String(row.sql) : '';
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 * @param {string} table
 * @returns {string[]}
 */
function indexSqls(db, table) {
    return db.prepare(
        `SELECT sql FROM sqlite_master
         WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL
         ORDER BY name`
    ).all(table).map((row) => row.sql);
}

/**
 * Copy child tables aside, rebuild plans, and put the children back.
 * Foreign keys stay on. Dropping plans while children exist would cascade.
 * @param {import('node:sqlite').DatabaseSync} db
 * @param {string} createPlans
 */
function rebuildPlans(db, createPlans) {
    const children = ['plan_revisions', 'plan_field_marks'];
    const saved = children.map((name) => ({
        name,
        create: tableSql(db, name),
        indexes: indexSqls(db, name)
    }));
    for (const child of saved) {
        if (!child.create) {
            throw new Error('Cannot rebuild plans without table ' + child.name);
        }
    }

    db.exec(`
        CREATE TABLE plan_revisions_keep AS SELECT * FROM plan_revisions;
        CREATE TABLE plan_field_marks_keep AS SELECT * FROM plan_field_marks;
        DROP TABLE plan_field_marks;
        DROP TABLE plan_revisions;
    `);
    db.exec(createPlans);
    db.exec(
        'INSERT INTO plans_new (' + PLAN_COLUMNS.join(', ') + ') '
        + 'SELECT ' + PLAN_COLUMNS.join(', ') + ' FROM plans'
    );
    db.exec(`
        DROP TABLE plans;
        ALTER TABLE plans_new RENAME TO plans;
    `);
    for (const child of saved) {
        db.exec(child.create);
        db.exec(
            'INSERT INTO ' + child.name + ' SELECT * FROM ' + child.name + '_keep'
        );
        for (const sql of child.indexes) db.exec(sql);
        db.exec('DROP TABLE ' + child.name + '_keep');
    }
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function up(db) {
    const sql = tableSql(db, 'plans');
    if (!sql) return;
    if (sql.includes('assignee_id = lower(assignee_id)')) return;
    rebuildPlans(db, CREATE_PLANS);
}

/**
 * Restore the 002 CHECK when every stored assignee still satisfies it.
 * @param {import('node:sqlite').DatabaseSync} db
 */
function down(db) {
    const sql = tableSql(db, 'plans');
    if (!sql || !sql.includes('assignee_id = lower(assignee_id)')) {
        forget(db);
        return;
    }
    const blocked = db.prepare(
        `SELECT id FROM plans
         WHERE assignee_id IS NOT NULL
           AND NOT (
                length(assignee_id) BETWEEN 8 AND 64
                AND assignee_id GLOB '[a-z0-9]*'
                AND assignee_id NOT GLOB '*[^a-z0-9-]*'
           )
         LIMIT 1`
    ).get();
    if (blocked) {
        throw new Error(
            'Cannot reverse migration 012 while an assignee id is outside the previous 8–64 [a-z0-9-] check.'
        );
    }
    rebuildPlans(db, CREATE_PLANS_OLD);
    forget(db);
}

/**
 * @param {import('node:sqlite').DatabaseSync} db
 */
function forget(db) {
    const migrations = db.prepare(
        `SELECT 1 AS ok FROM sqlite_master
         WHERE type = 'table' AND name = 'schema_migrations'`
    ).get();
    if (migrations) {
        db.prepare('DELETE FROM schema_migrations WHERE version = ?').run(VERSION);
    }
}

module.exports = { up, down, VERSION };
