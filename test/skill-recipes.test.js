/**
 * Hebrew-worksheet skill docs match schema, activity map, and validator (INIT-012/SPEC-008).
 */
'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { validatePlan, RULES } = require('../server/plans/validate');
const activityMap = require('../shared/admin/activity-map.js');

const ROOT = path.resolve(__dirname, '..');
const SCHEMA_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json');
const RECIPES_PATH = path.join(ROOT, '.cursor', 'skills', 'hebrew-worksheet', 'recipes.md');
const SKILL_PATH = path.join(ROOT, '.cursor', 'skills', 'hebrew-worksheet', 'SKILL.md');
const SCHEMA_DOC_PATH = path.join(ROOT, '.cursor', 'skills', 'hebrew-worksheet', 'schema.md');

const recipesMd = fs.readFileSync(RECIPES_PATH, 'utf8');
const skillMd = fs.readFileSync(SKILL_PATH, 'utf8');
const schemaDocMd = fs.readFileSync(SCHEMA_DOC_PATH, 'utf8');
const schema = JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));

function oneOfBlockTypes() {
    const defs = schema.definitions;
    return (defs.block.oneOf || []).map((branch) => {
        const name = branch.$ref.slice('#/definitions/'.length);
        return defs[name].properties.type.const;
    });
}

function parseLessonTypesTable(md) {
    const section = md.split('## Lesson types')[1].split(/^## /m)[0];
    const rows = [];
    section.split('\n').forEach((line) => {
        if (!line.startsWith('|')) return;
        const cells = line.split('|').map((c) => c.trim()).filter(Boolean);
        if (cells.length < 3 || cells[0] === 'Teacher type' || cells[0].startsWith('---')) return;
        const recipeMatch = cells[1].match(/`([^`]+)`/);
        rows.push({
            label: cells[0],
            recipe: recipeMatch ? recipeMatch[1] : cells[1]
        });
    });
    return rows;
}

function parseActivityTable(md) {
    const section = md.split('## Activity names')[1].split(/^## /m)[0];
    const rows = [];
    section.split('\n').forEach((line) => {
        if (!line.startsWith('|')) return;
        const cells = line.split('|').map((c) => c.trim()).filter(Boolean);
        if (cells.length < 2 || cells[0] === 'Activity' || cells[0].startsWith('---')) return;
        const block = cells[1].replace(/^`|`$/g, '');
        rows.push({ activity: cells[0], block });
    });
    return rows;
}

function extractExerciseJsonExamples(md) {
    const section = md.split('## How to write each exercise')[1].split(/^## /m)[0];
    const blocks = [];
    const re = /```json\r?\n([\s\S]*?)```/g;
    let m;
    while ((m = re.exec(section)) !== null) {
        blocks.push(JSON.parse(m[1]));
    }
    return blocks;
}

function minimalPlan(blocks, recipe) {
    return {
        schemaVersion: '1.2.0',
        id: 'skill-recipes-test-plan',
        revision: 1,
        status: 'draft',
        audience: 'shared',
        assigneeId: null,
        title: 'Skill recipes test',
        language: { target: 'he', gloss: 'en' },
        units: [{
            id: 'unit-1',
            title: 'Unit',
            theme: 'test',
            lessons: [{
                id: 'lesson-1',
                title: 'Lesson',
                level: 'basic',
                visibility: 'hidden',
                recipe: recipe || 'review-worksheet',
                buildsOn: [],
                newWords: [],
                recycles: [],
                blocks
            }]
        }]
    };
}

const RECIPE_ENUM = new Set(schema.definitions.recipe.enum);
const BLOCK_TYPES = new Set(oneOfBlockTypes());
const MATCH_MARKER = /INIT-010,\s*not held yet|not held until INIT-010 SPEC-004/i;

describe('INIT-012/SPEC-008: hebrew-worksheet skill docs', () => {
    it('ac-1: lesson-types table lists A–E with schema recipe values', () => {
        const rows = parseLessonTypesTable(recipesMd);
        const letterRows = rows.filter((r) => /^[A-E]\./.test(r.label));
        assert.equal(letterRows.length, 5, 'expected five lettered lesson types');
        letterRows.forEach((row) => {
            assert.ok(RECIPE_ENUM.has(row.recipe), row.label + ' recipe ' + row.recipe);
        });
    });

    it('ac-2: block types in recipes.md are schema-backed or match is INIT-010 gated', () => {
        const tokens = new Set([...recipesMd.matchAll(/`([a-z][a-z0-9-]*)`/g)].map((m) => m[1]));
        const blockLike = new Set([...BLOCK_TYPES, 'match']);
        tokens.forEach((token) => {
            if (!blockLike.has(token)) return;
            if (token === 'match') return;
            assert.ok(BLOCK_TYPES.has(token), 'block type `' + token + '` not in block.oneOf');
        });
        recipesMd.split('\n').forEach((line) => {
            if (!line.includes('`match`')) return;
            assert.ok(
                MATCH_MARKER.test(line) || /Leave for the teacher/i.test(line),
                'match must be INIT-010 gated: ' + line.trim()
            );
        });
    });

    it('ac-3: skill docs do not say choose, unscramble, true/false, or future is unheld', () => {
        const files = [
            ['SKILL.md', skillMd],
            ['recipes.md', recipesMd],
            ['schema.md', schemaDocMd]
        ];
        const feature = /choose|unscramble|true[- ]?false|future tense|future-tense|kind:\s*choose|kind:\s*dictation/i;
        const denial = /not held|not in the registry|no block in schema|Do not invent a choice block/i;
        files.forEach(([name, text]) => {
            text.split('\n').forEach((line, index) => {
                if (!feature.test(line) || !denial.test(line)) return;
                if (/`match`|Match column|drawing block|imperative/i.test(line)) return;
                assert.fail(name + ':' + (index + 1) + ' still denies a held feature: ' + line.trim());
            });
        });
    });

    it('ac-4: exercise JSON examples in recipes.md validate in a 1.2.0 lesson', () => {
        const examples = extractExerciseJsonExamples(recipesMd);
        assert.equal(examples.length, 5, 'expected five exercise JSON examples');
        examples.forEach((block) => {
            const result = validatePlan(minimalPlan([block]));
            assert.equal(result.ok, true, block.kind + ': ' + JSON.stringify(result.blocks));
        });
    });

    it('ac-5: SKILL.md activity table matches activity-map.js', () => {
        const rows = parseActivityTable(skillMd);
        const fromSkill = Object.fromEntries(rows.map((r) => [r.activity, r.block]));
        assert.deepEqual(Object.keys(fromSkill).sort(), [...activityMap.ACTIVITY_NAMES].sort());
        activityMap.ACTIVITY_NAMES.forEach((name) => {
            assert.equal(fromSkill[name], activityMap.ACTIVITY_TO_BLOCK[name], name);
        });
    });

    it('ac-6: class-docs section requires original lines and dictionary pointing', () => {
        assert.match(recipesMd, /## Using the teacher's class docs/);
        const section = recipesMd.split("## Using the teacher's class docs")[1].split(/^## /m)[0];
        assert.match(section, /original/i);
        assert.match(section, /dictionary|lexicon|lookup_lexicon/i);
        assert.match(section, /never copy.*learner|class-doc text/i);
    });

    it('edge: every N1–N23 id cited in recipes.md exists in RULES', () => {
        const cited = new Set([...recipesMd.matchAll(/\bN(\d+)\b/g)].map((m) => 'N' + m[1]));
        cited.forEach((id) => {
            const num = Number(id.slice(1));
            if (num < 1 || num > 23) return;
            assert.ok(Object.prototype.hasOwnProperty.call(RULES, id), 'missing RULES.' + id);
        });
    });

    it('edge: JSON examples in recipes.md contain no angle brackets', () => {
        extractExerciseJsonExamples(recipesMd).forEach((block, index) => {
            const raw = JSON.stringify(block);
            assert.ok(!raw.includes('<') && !raw.includes('>'), 'example ' + index);
        });
    });
});
