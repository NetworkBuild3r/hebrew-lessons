/**
 * No write is ever refused because of who wrote a field last (INIT-008/SPEC-006,
 * initiative §7 "AI-first honesty"). A property over every actor and last-writer
 * combination, a scan of every error string the plan modules can return, and a
 * search for the words the operator ruled out.
 * Temporary database only. No network, no ElevenLabs, no OpenAI.
 * Provenance: INIT-008/SPEC-006
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');

const { planEnv, samplePlan, LESSON, WORD, LISTEN } = require('./helpers/plan-db');

const ACTORS = ['ai', 'human', 'system'];
const LAST_WRITERS = ['ai', 'human', 'system', 'none'];
const ALLOWED_ERRORS = [
    'revision_conflict', 'validation', 'id_mismatch', 'assign_required', 'not_found', 'invalid_op'
];

describe('no write is refused because of a mark (INIT-008/SPEC-006)', { concurrency: false }, () => {
    let env;

    before(() => {
        env = planEnv('hebrew-plan-norefusal-');
    });

    after(() => {
        env.close();
    });

    function ctxFor(kind) {
        if (kind === 'ai') return env.ai('test');
        if (kind === 'human') return env.human('test');
        return env.system('writer-script');
    }

    /** Normalize what may legitimately differ between runs: the plan id and its revision. */
    function normalized(id) {
        const plan = JSON.parse(env.store.loadWorkingCopy(id).document);
        plan.id = 'normalized';
        plan.revision = 0;
        return plan;
    }

    const WRITES = {
        set_field: (id, revision, ctx) => env.ops.setField(env.store, id, {
            path: WORD, field: 'meaning', value: 'final-value', expectedRevision: revision
        }, ctx),
        replace_part: (id, revision, ctx) => {
            const word = env.ops.readPart(env.store, id, { path: WORD }).part;
            word.meaning = 'final-value';
            return env.ops.replacePart(env.store, id, { path: WORD, value: word, expectedRevision: revision }, ctx);
        },
        replace_lesson: (id, revision, ctx) => {
            const lesson = env.ops.readPart(env.store, id, { path: LESSON }).part;
            lesson.blocks[0].words[1].meaning = 'final-value';
            return env.ops.replacePart(env.store, id, { path: LESSON, value: lesson, expectedRevision: revision }, ctx);
        },
        restore: (id, revision, ctx) => env.revisions.restore(env.store, id, {
            fromRevision: 1, path: WORD, field: 'meaning', expectedRevision: revision
        }, ctx)
    };

    for (const [name, write] of Object.entries(WRITES)) {
        it('ac-4: ' + name + ' is accepted and produces the same document for all 12 actor and last-writer combinations', () => {
            const documents = [];
            let n = 0;
            for (const actor of ACTORS) {
                for (const last of LAST_WRITERS) {
                    n += 1;
                    const id = 'matrix-' + name.replace('_', '-') + '-' + n;
                    env.seed(samplePlan((p) => { p.id = id; }));
                    let revision = 1;
                    if (name === 'restore') {
                        // A restore needs a logged revision to come from: one script edit of another field makes the baseline.
                        const prep = env.ops.setField(env.store, id, {
                            path: '', field: 'title', value: 'Prepared', expectedRevision: 1
                        }, env.system('prep-script'));
                        revision = prep.revision;
                    }
                    if (last !== 'none') {
                        const first = env.ops.setField(env.store, id, {
                            path: WORD, field: 'meaning', value: 'first-from-' + last, expectedRevision: revision
                        }, ctxFor(last));
                        assert.equal(first.error, undefined, JSON.stringify(first));
                        revision = first.revision;
                    }
                    const result = write(id, revision, ctxFor(actor));
                    assert.equal(result.error, undefined, name + ' ' + actor + '/' + last + ' ' + JSON.stringify(result).slice(0, 300));
                    assert.equal(result.revision, revision + 1);
                    documents.push(normalized(id));
                }
            }
            assert.equal(documents.length, 12);
            for (const doc of documents.slice(1)) assert.deepEqual(doc, documents[0], name);
        });
    }

    it('ac-4: a write that fails fails only for the old reasons, whoever wrote the field last', () => {
        const seen = new Set();
        let n = 0;
        for (const actor of ACTORS) {
            for (const last of LAST_WRITERS) {
                n += 1;
                const id = 'fails-' + n;
                env.seed(samplePlan((p) => { p.id = id; }));
                let revision = 1;
                if (last !== 'none') {
                    revision = env.ops.setField(env.store, id, {
                        path: WORD, field: 'meaning', value: 'person-or-ai', expectedRevision: 1
                    }, ctxFor(last)).revision;
                }
                const ctx = ctxFor(actor);
                const attempts = [
                    env.ops.setField(env.store, id, { path: WORD, field: 'meaning', value: 'x', expectedRevision: revision + 5 }, ctx),
                    env.ops.setField(env.store, id, { path: WORD, field: 'meaning', value: '<b>x</b>', expectedRevision: revision }, ctx),
                    env.ops.setField(env.store, id, { path: LISTEN, field: 'id', value: 'renamed', expectedRevision: revision }, ctx),
                    env.ops.setField(env.store, id, { path: '', field: 'audience', value: 'personal', expectedRevision: revision }, ctx),
                    env.ops.setField(env.store, id, { path: LESSON + '/blocks/missing', field: 'x', value: 1, expectedRevision: revision }, ctx),
                    env.ops.setField(env.store, id, { path: WORD, field: 'bad field', value: 1, expectedRevision: revision }, ctx),
                    env.ops.replacePart(env.store, id, { path: LISTEN, value: { id: 'other', type: 'listening', passage: 'x' }, expectedRevision: revision }, ctx),
                    env.revisions.restore(env.store, id, { fromRevision: 99, expectedRevision: revision }, ctx)
                ];
                for (const result of attempts) {
                    assert.equal(typeof result.error, 'string', JSON.stringify(result).slice(0, 200));
                    seen.add(result.error);
                }
            }
        }
        assert.deepEqual(Array.from(seen).sort(), ALLOWED_ERRORS.slice().sort());
    });

    it('ac-4: no error string any plan module can return mentions a mark, a person, a pin, or a lock', () => {
        const dir = path.join(env.root, 'server', 'plans');
        const skip = new Set(['validate.js', 'hebrew-checks.js', 'issues.js']);
        const files = fs.readdirSync(dir).filter((f) => f.endsWith('.js') && !skip.has(f)).map((f) => path.join(dir, f));
        files.push(path.join(env.root, 'server', 'api', 'plan-routes.js'));
        const found = new Set();
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            for (const re of [/\berror:\s*'([A-Za-z_]+)'/g, /\bconflict\('([A-Za-z_]+)'/g, /\bfailure\('([A-Za-z_]+)'/g,
                /sendJsonError\([^)]*?,\s*'([A-Za-z_]+)'\s*\)/g]) {
                for (const m of text.matchAll(re)) found.add(m[1]);
            }
        }
        for (const known of ALLOWED_ERRORS.concat(['already_exists', 'invalid_id', 'invalid_recipe', 'no_snapshot', 'revision_corrupt'])) {
            assert.equal(found.has(known), true, known + ' should be found by the scan');
        }
        assert.equal(found.size >= 12, true);
        for (const code of found) {
            assert.equal(/mark|human|person|\bpin|pinned|lock|edited|overwr|writer/i.test(code), false, code);
        }
    });

    it('ac-5: no pin or lock identifier, column, key, or tool text exists in the plan code', () => {
        const root = env.root;
        const files = [];
        const plans = path.join(root, 'server', 'plans');
        for (const f of fs.readdirSync(plans)) if (f.endsWith('.js')) files.push(path.join(plans, f));
        const repos = path.join(root, 'server', 'db', 'repos');
        for (const f of fs.readdirSync(repos)) if (/^plan-.*\.js$/.test(f)) files.push(path.join(repos, f));
        const migrations = path.join(root, 'server', 'db', 'migrations');
        for (const f of fs.readdirSync(migrations)) if (f.startsWith('006-')) files.push(path.join(migrations, f));
        files.push(path.join(root, 'server', 'api', 'plan-routes.js'));
        assert.equal(files.length >= 14, true);

        const banned = [
            /field_pinned|keepPinned|pinned_by|pinned_at/,
            /\b(?:pinned|locked)\s*:/,
            /['"`](?:pinned|locked)['"`]/,
            /\.(?:pinned|locked)\b/,
            /\b(?:pinned|locked)\s+(?:INTEGER|TEXT|BOOLEAN|BLOB|NUMERIC)\b/i,
            /\b(?:pinned|locked)_[a-z]+\b/
        ];
        for (const file of files) {
            const text = fs.readFileSync(file, 'utf8');
            for (const re of banned) {
                assert.equal(re.test(text), false, path.basename(file) + ' ' + re);
            }
        }

        for (const table of ['plans', 'plan_revisions', 'plan_field_marks']) {
            for (const col of env.db.all('PRAGMA table_info(' + table + ')')) {
                assert.equal(/^(?:pinned|locked|pin|lock)/i.test(col.name), false, table + '.' + col.name);
            }
        }
    });

    it('ac-5: no registered plan tool name or description holds a pin or a lock', () => {
        const prev = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'test-no-refusal-token';
        const metas = Object.create(null);
        try {
            const tools = require('../mcp/tools');
            tools.registerLessonTools({ registerTool(name, meta) { metas[name] = meta; } });
        } finally {
            if (prev === undefined) delete process.env.MCP_AUTH_TOKEN;
            else process.env.MCP_AUTH_TOKEN = prev;
        }
        const planTools = ['read_part', 'replace_part', 'insert_block', 'move_block', 'remove_block', 'set_field', 'publish_plan'];
        for (const name of planTools) {
            assert.equal(Boolean(metas[name]), true, name);
            const text = [name, metas[name].title, metas[name].description, Object.keys(metas[name].inputSchema || {}).join(' ')].join(' ');
            assert.equal(/field_pinned|keepPinned|pinned_by|pinned_at|\bpinned\b|\blocked\b|\bpin\b|\block\b/i.test(text), false, name);
        }
        assert.equal(/never blocks a write/i.test(metas.read_part.description), true);
    });

    it('the advisory words are present: editState is a note, and the note says so', () => {
        env.seed(samplePlan((p) => { p.id = 'advisory-plan'; }));
        const state = env.ops.readPart(env.store, 'advisory-plan', { path: '' }, env.ai()).editState;
        assert.equal(state.advisory, true);
    });
});
