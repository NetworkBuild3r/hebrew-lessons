/**
 * Audit log repository — prepared statements only.
 * Provenance: INIT-002/SPEC-003
 */
'use strict';

const db = require('../index');

function append(row) {
    return db.run(
        `INSERT INTO audit_log (actor, action, target, at, detail)
         VALUES (?, ?, ?, ?, ?)`,
        row.actor,
        row.action,
        row.target == null ? null : row.target,
        row.at || new Date().toISOString(),
        row.detail == null ? null : row.detail
    );
}

function listRecent(limit) {
    const cap = limit == null ? 100 : limit;
    return db.all(
        'SELECT * FROM audit_log ORDER BY id DESC LIMIT ?',
        cap
    );
}

function listByActor(actor, limit) {
    const cap = limit == null ? 100 : limit;
    return db.all(
        'SELECT * FROM audit_log WHERE actor = ? ORDER BY id DESC LIMIT ?',
        actor,
        cap
    );
}

module.exports = {
    append,
    listRecent,
    listByActor
};
