/**
 * plan_field_marks repository: SQL only, prepared statements only.
 * One row per (plan, id path, field): who wrote it last, and the last time a
 * person wrote it with the value they replaced. Rows are advisory notes beside
 * the plan; nothing here is read to refuse a write (ADR-0011 §4).
 * A path prefix is compared with substr(), never LIKE, so a `_` or `%` in an id
 * is never a wildcard.
 * Provenance: INIT-008/SPEC-006
 */
'use strict';

const db = require('../index');

/**
 * @param {object} row
 * @returns {object}
 */
function mapRow(row) {
    return {
        planId: row.plan_id,
        path: row.path,
        field: row.field,
        lastActorKind: row.last_actor_kind,
        lastActorName: row.last_actor_name,
        lastRevision: row.last_revision,
        lastAt: row.last_at,
        valueHash: row.value_hash,
        humanRevision: row.human_revision == null ? null : row.human_revision,
        humanAt: row.human_at == null ? null : row.human_at,
        humanActorName: row.human_actor_name == null ? null : row.human_actor_name,
        previousValueJson: row.previous_value_json == null ? null : row.previous_value_json,
        previousWriterKind: row.previous_writer_kind == null ? null : row.previous_writer_kind
    };
}

/**
 * Insert or replace the mark for (planId, path, field). The caller computes the
 * whole row, so this is a plain write of every column.
 * @param {object} mark Same keys as the object forPlan returns.
 */
function upsert(mark) {
    db.run(
        `INSERT INTO plan_field_marks (
            plan_id, path, field, last_actor_kind, last_actor_name, last_revision, last_at,
            value_hash, human_revision, human_at, human_actor_name,
            previous_value_json, previous_writer_kind
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT (plan_id, path, field) DO UPDATE SET
            last_actor_kind = excluded.last_actor_kind,
            last_actor_name = excluded.last_actor_name,
            last_revision = excluded.last_revision,
            last_at = excluded.last_at,
            value_hash = excluded.value_hash,
            human_revision = excluded.human_revision,
            human_at = excluded.human_at,
            human_actor_name = excluded.human_actor_name,
            previous_value_json = excluded.previous_value_json,
            previous_writer_kind = excluded.previous_writer_kind`,
        mark.planId,
        mark.path,
        mark.field,
        mark.lastActorKind,
        mark.lastActorName,
        mark.lastRevision,
        mark.lastAt,
        mark.valueHash,
        mark.humanRevision == null ? null : mark.humanRevision,
        mark.humanAt == null ? null : mark.humanAt,
        mark.humanActorName == null ? null : mark.humanActorName,
        mark.previousValueJson == null ? null : mark.previousValueJson,
        mark.previousWriterKind == null ? null : mark.previousWriterKind
    );
}

/**
 * @param {string} planId
 * @returns {object[]}
 */
function forPlan(planId) {
    return db.all('SELECT * FROM plan_field_marks WHERE plan_id = ?', planId).map(mapRow);
}

/**
 * Marks at `prefix` and below it. An empty prefix is the whole plan.
 * @param {string} planId
 * @param {string} prefix
 * @returns {object[]}
 */
function forPathPrefix(planId, prefix) {
    if (prefix === '') return forPlan(planId);
    return db.all(
        `SELECT * FROM plan_field_marks
         WHERE plan_id = ?
           AND (path = ? OR substr(path, 1, length(?) + 1) = ? || '/')`,
        planId,
        prefix,
        prefix,
        prefix
    ).map(mapRow);
}

/**
 * Delete the marks at `prefix` and below it. An empty prefix is the whole plan.
 * @param {string} planId
 * @param {string} prefix
 */
function deleteUnder(planId, prefix) {
    if (prefix === '') {
        db.run('DELETE FROM plan_field_marks WHERE plan_id = ?', planId);
        return;
    }
    db.run(
        `DELETE FROM plan_field_marks
         WHERE plan_id = ?
           AND (path = ? OR substr(path, 1, length(?) + 1) = ? || '/')`,
        planId,
        prefix,
        prefix,
        prefix
    );
}

/**
 * Count of marks whose current last writer is a person, for every plan, in one
 * grouped query.
 * @returns {Map<string, number>}
 */
function humanCounts() {
    const counts = new Map();
    const rows = db.all(
        `SELECT plan_id, COUNT(*) AS n
         FROM plan_field_marks
         WHERE last_actor_kind = 'human'
         GROUP BY plan_id`
    );
    for (const row of rows) counts.set(row.plan_id, row.n);
    return counts;
}

/**
 * The newest person edit anywhere in the plan.
 * @param {string} planId
 * @returns {{ revision: number, at: string }|null}
 */
function lastHuman(planId) {
    const row = db.get(
        `SELECT human_revision, human_at
         FROM plan_field_marks
         WHERE plan_id = ? AND human_revision IS NOT NULL
         ORDER BY human_revision DESC, human_at DESC
         LIMIT 1`,
        planId
    );
    return row ? { revision: row.human_revision, at: row.human_at } : null;
}

module.exports = { upsert, forPlan, forPathPrefix, deleteUnder, humanCounts, lastHuman };
