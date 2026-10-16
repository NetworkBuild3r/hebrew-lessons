/**
 * Menu names must map to a block.oneOf type; every *Block definition must be
 * referenced from oneOf (or listed as deliberately unused).
 * Provenance: INIT-011/SPEC-002
 *
 * Pre-change (red) facts this test caught before the SPEC-002 edits:
 * - ACTIVITY_NAMES included Match → type "match", which no oneOf branch accepts
 * - verseReadingBlock was defined but missing from block.oneOf
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..');
const SCHEMA_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json');
const activityMap = require('../shared/admin/activity-map.js');

/** Block definitions kept out of oneOf on purpose (schema version helpers). */
const DELIBERATELY_UNUSED_BLOCKS = Object.freeze(['v100Block', 'v110Block']);

function oneOfTypes(schema) {
    const defs = schema.definitions;
    return (defs.block.oneOf || []).map((branch) => {
        const name = branch.$ref.slice('#/definitions/'.length);
        return defs[name].properties.type.const;
    });
}

function oneOfDefNames(schema) {
    return (schema.definitions.block.oneOf || []).map((branch) =>
        branch.$ref.slice('#/definitions/'.length)
    );
}

describe('INIT-011/SPEC-002: activity menu ↔ block.oneOf', () => {
    const schema = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
    const accepted = new Set(oneOfTypes(schema));
    const unused = new Set(DELIBERATELY_UNUSED_BLOCKS);

    it('every ACTIVITY_NAMES entry maps to a type some oneOf branch accepts', () => {
        const names = activityMap.ACTIVITY_NAMES;
        assert.ok(Array.isArray(names) && names.length > 0, 'ACTIVITY_NAMES present');
        names.forEach((name) => {
            const type = activityMap.ACTIVITY_TO_BLOCK[name];
            assert.ok(type, name + ' has a mapped block type');
            assert.ok(
                accepted.has(type),
                name + ' → ' + type + ' is not accepted by block.oneOf'
            );
        });
    });

    it('every *Block definition is in block.oneOf unless deliberately unused', () => {
        const defNames = Object.keys(schema.definitions).filter((n) => /Block$/.test(n));
        const inOneOf = new Set(oneOfDefNames(schema));
        defNames.forEach((name) => {
            if (unused.has(name)) {
                assert.equal(
                    inOneOf.has(name),
                    false,
                    name + ' is listed as deliberately unused but still in oneOf'
                );
                return;
            }
            assert.ok(inOneOf.has(name), name + ' is missing from block.oneOf');
        });
    });
});
