/**
 * Re-export of the shared activity map (INIT-010/SPEC-002).
 * Browser scripts load shared/admin/activity-map.js; the service require()s this path.
 * Provenance: INIT-010/SPEC-002, INIT-010/SPEC-003
 */
'use strict';

module.exports = require('../../shared/admin/activity-map.js');
