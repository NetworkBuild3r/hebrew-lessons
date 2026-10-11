'use strict';

/**
 * Focused JSON Schema draft-07 checker for the ADR-0003 lesson plan.
 * Walks the keywords this schema actually uses. No npm dependency.
 * Provenance: INIT-003/SPEC-009
 */

const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '..', '..');
const SCHEMA_PATH = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'lesson-plan.schema.json');
const SAMPLES_DIR = path.join(ROOT, 'docs', 'decisions', 'ADR-0003-assets', 'samples');

/** Spike lookup. A plan may only name these picture ids. */
const MEDIA_IDS = new Set(['coffee-cup', 'water-glass', 'milk', 'sugar']);

const KNOWN_KEYWORDS = new Set([
    '$schema', '$id', 'title', 'description', 'definitions', '$ref',
    'type', 'additionalProperties', 'required', 'properties', 'allOf', 'if', 'then',
    'const', 'enum', 'items', 'minItems', 'maxItems', 'minLength', 'maxLength',
    'pattern', 'not', 'minimum', 'maximum', 'oneOf', 'maxProperties'
]);

function loadSchema() {
    return JSON.parse(fs.readFileSync(SCHEMA_PATH, 'utf8'));
}

function sampleFiles() {
    return fs.readdirSync(SAMPLES_DIR)
        .filter((name) => name.endsWith('.plan.json'))
        .sort()
        .map((name) => path.join(SAMPLES_DIR, name));
}

function jsonType(instance) {
    if (instance === null) return 'null';
    if (Array.isArray(instance)) return 'array';
    if (typeof instance === 'number') return Number.isInteger(instance) ? 'integer' : 'number';
    return typeof instance;
}

function typeOk(instance, type) {
    const actual = jsonType(instance);
    const list = Array.isArray(type) ? type : [type];
    return list.indexOf(actual) !== -1;
}

function deref(schema, root) {
    if (schema && typeof schema.$ref === 'string') {
        const prefix = '#/definitions/';
        if (schema.$ref.indexOf(prefix) !== 0) {
            throw new Error('unsupported $ref ' + schema.$ref);
        }
        const name = schema.$ref.slice(prefix.length);
        if (!root.definitions || !root.definitions[name]) {
            throw new Error('missing definition ' + name);
        }
        return root.definitions[name];
    }
    return schema;
}

function push(errors, instancePath, keyword, message) {
    errors.push({
        path: instancePath || '/',
        keyword: keyword,
        message: message
    });
}

function check(instance, schema, root, instancePath, errors) {
    schema = deref(schema, root);
    if (!schema || typeof schema !== 'object') {
        push(errors, instancePath, 'schema', 'invalid schema');
        return;
    }

    const unknown = Object.keys(schema).filter((key) => !KNOWN_KEYWORDS.has(key));
    if (unknown.length) {
        throw new Error('unhandled schema keyword: ' + unknown.join(', '));
    }

    if (Object.prototype.hasOwnProperty.call(schema, 'const') && instance !== schema.const) {
        push(errors, instancePath, 'const', 'expected ' + JSON.stringify(schema.const));
        return;
    }

    if (schema.enum && schema.enum.indexOf(instance) === -1) {
        push(errors, instancePath, 'enum', 'value is not allowed');
        return;
    }

    if (schema.type !== undefined && !typeOk(instance, schema.type)) {
        push(errors, instancePath, 'type', 'expected ' + JSON.stringify(schema.type));
        return;
    }

    if (schema.oneOf) {
        let matches = 0;
        let preferred = null;
        let fewest = null;
        schema.oneOf.forEach((branch) => {
            const branchErrors = [];
            check(instance, branch, root, instancePath, branchErrors);
            if (branchErrors.length === 0) matches += 1;
            const resolved = deref(branch, root);
            const declared = resolved.properties && resolved.properties.type && resolved.properties.type.const;
            if (instance && declared !== undefined && declared === instance.type) preferred = branchErrors;
            if (!fewest || branchErrors.length < fewest.length) fewest = branchErrors;
        });
        if (matches !== 1) {
            push(errors, instancePath, 'oneOf', matches === 0
                ? 'does not match any allowed block type'
                : 'matches more than one block type');
            (preferred || fewest || []).forEach((error) => errors.push(error));
        }
    }

    if (schema.not) {
        const notErrors = [];
        check(instance, schema.not, root, instancePath, notErrors);
        if (notErrors.length === 0) {
            push(errors, instancePath, 'not', 'markup rejected');
        }
    }

    if (typeof instance === 'string') {
        if (schema.minLength !== undefined && instance.length < schema.minLength) {
            push(errors, instancePath, 'minLength', 'shorter than ' + schema.minLength);
        }
        if (schema.maxLength !== undefined && instance.length > schema.maxLength) {
            const oversized = schema.maxLength >= 4000 ? 'oversized text' : 'longer than ' + schema.maxLength;
            push(errors, instancePath, 'maxLength', oversized);
        }
        if (schema.pattern !== undefined && !new RegExp(schema.pattern).test(instance)) {
            const media = instancePath.endsWith('/mediaId');
            push(errors, instancePath, 'pattern', media ? 'bad media reference' : 'does not match pattern');
        }
    }

    if (typeof instance === 'number') {
        if (schema.minimum !== undefined && instance < schema.minimum) {
            push(errors, instancePath, 'minimum', 'below minimum');
        }
        if (schema.maximum !== undefined && instance > schema.maximum) {
            push(errors, instancePath, 'maximum', 'above maximum');
        }
    }

    if (Array.isArray(instance)) {
        if (schema.minItems !== undefined && instance.length < schema.minItems) {
            push(errors, instancePath, 'minItems', 'too few items');
        }
        if (schema.maxItems !== undefined && instance.length > schema.maxItems) {
            push(errors, instancePath, 'maxItems', 'too many items');
        }
        if (schema.items) {
            instance.forEach((item, index) => {
                check(item, schema.items, root, instancePath + '/' + index, errors);
            });
        }
    }

    if (instance && jsonType(instance) === 'object') {
        const keys = Object.keys(instance);
        if (schema.maxProperties !== undefined && keys.length > schema.maxProperties) {
            push(errors, instancePath, 'maxProperties', 'too many properties');
        }
        const props = schema.properties || {};
        if (schema.required) {
            schema.required.forEach((key) => {
                if (!Object.prototype.hasOwnProperty.call(instance, key)) {
                    const message = key === 'alt' ? 'missing alt text' : 'missing required field: ' + key;
                    push(errors, instancePath + '/' + key, 'required', message);
                }
            });
        }
        keys.forEach((key) => {
            const childPath = instancePath + '/' + key;
            if (Object.prototype.hasOwnProperty.call(props, key)) {
                check(instance[key], props[key], root, childPath, errors);
                return;
            }
            if (schema.additionalProperties === false) {
                push(errors, childPath, 'additionalProperties', 'unknown property');
                return;
            }
            if (schema.additionalProperties && typeof schema.additionalProperties === 'object') {
                check(instance[key], schema.additionalProperties, root, childPath, errors);
            }
        });
    }

    if (schema.allOf) {
        schema.allOf.forEach((part) => {
            check(instance, part, root, instancePath, errors);
        });
    }

    if (schema.if) {
        const ifErrors = [];
        check(instance, schema.if, root, instancePath, ifErrors);
        if (ifErrors.length === 0 && schema.then) {
            check(instance, schema.then, root, instancePath, errors);
        }
    }
}

function walkMedia(node, instancePath, errors) {
    if (!node || typeof node !== 'object') return;
    if (Array.isArray(node)) {
        node.forEach((item, index) => walkMedia(item, instancePath + '/' + index, errors));
        return;
    }
    if (typeof node.mediaId === 'string' && !MEDIA_IDS.has(node.mediaId)) {
        push(errors, instancePath + '/mediaId', 'media', 'broken media reference');
    }
    Object.keys(node).forEach((key) => {
        walkMedia(node[key], instancePath + '/' + key, errors);
    });
}

function validatePlan(data, schema) {
    const root = schema || loadSchema();
    const errors = [];
    check(data, root, root, '', errors);
    walkMedia(data, '', errors);
    return { ok: errors.length === 0, errors: errors };
}

function brokenPlans(base) {
    function clone() {
        return JSON.parse(JSON.stringify(base));
    }

    const script = clone();
    script.title = '<script>alert(1)</script>';

    const missingAlt = clone();
    delete missingAlt.units[0].lessons[0].blocks[0].words[1].image.alt;

    const oversized = clone();
    oversized.subtitle = 'x'.repeat(4001);

    const badMedia = clone();
    badMedia.units[0].lessons[0].blocks[0].words[1].image.mediaId = 'http://evil.example/x.png';

    const unknownMedia = clone();
    unknownMedia.units[0].lessons[0].blocks[0].words[1].image.mediaId = 'not-a-real-picture';

    const missingTitle = clone();
    delete missingTitle.title;

    return [
        { name: 'script-in-title', plan: script, reason: 'markup rejected' },
        { name: 'missing-alt', plan: missingAlt, reason: 'missing alt text' },
        { name: 'oversized-text', plan: oversized, reason: 'oversized text' },
        { name: 'bad-media', plan: badMedia, reason: 'bad media reference' },
        { name: 'unknown-media', plan: unknownMedia, reason: 'broken media reference' },
        { name: 'missing-title', plan: missingTitle, reason: 'missing required field: title' }
    ];
}

function main() {
    const schema = loadSchema();
    const files = sampleFiles();
    let failed = 0;

    if (files.length !== 4) {
        process.stderr.write('expected 4 samples, found ' + files.length + '\n');
        failed += 1;
    }

    files.forEach((file) => {
        const data = JSON.parse(fs.readFileSync(file, 'utf8'));
        const result = validatePlan(data, schema);
        if (result.ok) {
            process.stdout.write('ok ' + path.basename(file) + '\n');
            return;
        }
        failed += 1;
        process.stderr.write('FAIL ' + path.basename(file) + '\n');
        result.errors.slice(0, 12).forEach((error) => {
            process.stderr.write('  ' + error.path + ' ' + error.message + '\n');
        });
    });

    const base = JSON.parse(fs.readFileSync(
        path.join(SAMPLES_DIR, 'beginner-unit-1.plan.json'),
        'utf8'
    ));
    brokenPlans(base).forEach((item) => {
        const result = validatePlan(item.plan, schema);
        const hit = !result.ok && result.errors.some((error) => error.message === item.reason);
        if (hit) {
            process.stdout.write('rejected ' + item.name + ': ' + item.reason + '\n');
            return;
        }
        failed += 1;
        process.stderr.write('FAIL broken ' + item.name + ' was not rejected for ' + item.reason + '\n');
        result.errors.slice(0, 8).forEach((error) => {
            process.stderr.write('  ' + error.path + ' ' + error.message + '\n');
        });
    });

    if (failed) {
        process.stderr.write('validate: failed\n');
        return 1;
    }
    process.stdout.write('validate: ok\n');
    return 0;
}

if (require.main === module) {
    process.exit(main());
}

module.exports = {
    MEDIA_IDS,
    SCHEMA_PATH,
    SAMPLES_DIR,
    brokenPlans,
    loadSchema,
    sampleFiles,
    validatePlan
};
