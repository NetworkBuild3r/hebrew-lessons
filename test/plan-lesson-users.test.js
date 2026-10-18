/**
 * Optional users on a selected plan lesson. Users are checked with the
 * set_lesson_visibility messages and stored as lesson_grants on the lesson id.
 */
'use strict';

const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');

const { planEnv } = require('./helpers/plan-db');

describe('plan lesson users', { concurrency: false }, () => {
    let env;
    let users;
    let visibilityRepo;
    let mayReadPublished;
    let toolHandlers;
    let registerLessonTools;

    before(() => {
        env = planEnv('plan-lesson-users-');
        users = require('../server/db/repos/users');
        visibilityRepo = require('../server/db/repos/visibility');
        ({ mayReadPublished } = require('../server/api/plan-routes'));
        ({ toolHandlers, registerLessonTools } = require('../mcp/tools'));
        registerLessonTools({ registerTool() {} });
        process.env.MCP_AUTH_TOKEN = 'test-token';
    });

    after(() => {
        delete process.env.MCP_AUTH_TOKEN;
        if (env) env.close();
    });

    function reset() {
        env.db.run('DELETE FROM lesson_grants');
        env.db.run('DELETE FROM plans');
        env.db.run('DELETE FROM users');
    }

    function account(username, status) {
        users.create({
            username,
            password_hash: 'not-used',
            role: 'user',
            status: status || 'active'
        });
        return users.findByUsername(username);
    }

    function lessonPaths(plan) {
        const paths = [];
        for (const unit of plan.units) {
            for (const lesson of unit.lessons) {
                paths.push({
                    id: lesson.id,
                    path: 'units/' + unit.id + '/lessons/' + lesson.id
                });
            }
        }
        return paths;
    }

    function setField(path, field, value, revision) {
        return env.ops.setField(env.store, env.PLAN_ID, {
            path,
            field,
            value,
            expectedRevision: revision
        }, env.human('set_field'));
    }

    it('selected users are visible to a listed account and not to an unlisted one', () => {
        reset();
        const seeded = env.seed();
        const listed = account('bnelson');
        const unlisted = account('otherone');
        let revision = seeded.revision;
        const lessons = lessonPaths(env.samplePlan());
        assert.ok(lessons.length > 1);

        for (const lesson of lessons) {
            const mode = setField(lesson.path, 'visibility', 'selected', revision);
            assert.equal(mode.error, undefined, JSON.stringify(mode));
            revision = mode.revision;
            const named = setField(lesson.path, 'users', ['Bnelson'], revision);
            assert.equal(named.error, undefined, JSON.stringify(named));
            revision = named.revision;
            const storedLesson = named.plan.units
                .flatMap((unit) => unit.lessons)
                .find((item) => item.id === lesson.id);
            assert.deepEqual(storedLesson.users, ['bnelson']);
            const grants = visibilityRepo.listGrants(lesson.id);
            assert.deepEqual(grants.map((row) => row.user_id), [listed.id]);
        }

        const published = env.publish.publishPlan(env.store, env.PLAN_ID, {
            expectedRevision: revision
        }, env.human('publish_plan'));
        assert.equal(published.error, undefined, JSON.stringify(published.blocks));
        const snapshot = JSON.parse(env.store.loadSnapshot(env.PLAN_ID));
        assert.equal(mayReadPublished({ id: listed.id, username: listed.username }, snapshot), true);
        assert.equal(mayReadPublished({ id: unlisted.id, username: unlisted.username }, snapshot), false);

        const beforeTitle = env.store.loadWorkingCopy(env.PLAN_ID);
        users.create({
            username: 'disabled1',
            password_hash: 'not-used',
            role: 'user',
            status: 'disabled'
        });
        const title = setField(lessons[0].path, 'title', 'Still listed', beforeTitle.revision);
        assert.equal(title.error, undefined, JSON.stringify(title));
        assert.deepEqual(visibilityRepo.listGrants(lessons[0].id).map((row) => row.user_id), [listed.id]);
    });

    it('rejects an unknown, empty, or inactive username and writes nothing', async () => {
        reset();
        const seeded = env.seed();
        account('bnelson');
        const path = lessonPaths(env.samplePlan())[0].path;
        const cases = [
            [['no-such-user'], 'Unknown or inactive username: "no-such-user".'],
            [[''], 'visibility.users contains an empty username.'],
            ['nope', 'visibility.users must be an array of usernames.']
        ];
        for (const [value, message] of cases) {
            assert.throws(
                () => setField(path, 'users', value, seeded.revision),
                (err) => err && err.code === 'username_list' && err.message === message
            );
            const stored = env.store.loadWorkingCopy(env.PLAN_ID);
            assert.equal(stored.revision, seeded.revision);
            assert.equal(stored.document.includes('no-such-user'), false);
            assert.equal(env.db.get('SELECT COUNT(*) AS n FROM lesson_grants').n, 0);
        }

        const inactive = account('quietone', 'disabled');
        assert.throws(
            () => setField(path, 'users', [inactive.username], seeded.revision),
            (err) => err && err.message === 'Unknown or inactive username: "quietone".'
        );
        assert.equal(env.store.loadWorkingCopy(env.PLAN_ID).revision, seeded.revision);

        const reply = await toolHandlers.set_field({
            id: env.PLAN_ID,
            path,
            field: 'users',
            value: ['no-such-user'],
            expectedRevision: seeded.revision
        });
        assert.equal(reply.isError, true);
        assert.equal(reply.content[0].text, 'Unknown or inactive username: "no-such-user".');
        assert.equal(env.store.loadWorkingCopy(env.PLAN_ID).revision, seeded.revision);
    });

    it('omitted users leaves grants alone', () => {
        reset();
        const seeded = env.seed();
        const listed = account('bnelson');
        let revision = seeded.revision;
        const lessons = lessonPaths(env.samplePlan());
        for (const lesson of lessons) {
            const mode = setField(lesson.path, 'visibility', 'selected', revision);
            assert.equal(mode.error, undefined, JSON.stringify(mode));
            revision = mode.revision;
            assert.deepEqual(visibilityRepo.listGrants(lesson.id), []);
            const storedLesson = mode.plan.units
                .flatMap((unit) => unit.lessons)
                .find((item) => item.id === lesson.id);
            assert.equal(Object.prototype.hasOwnProperty.call(storedLesson, 'users'), false);
        }
        const published = env.publish.publishPlan(env.store, env.PLAN_ID, {
            expectedRevision: revision
        }, env.human('publish_plan'));
        assert.equal(published.error, undefined, JSON.stringify(published.blocks));
        const snapshot = JSON.parse(env.store.loadSnapshot(env.PLAN_ID));
        assert.equal(mayReadPublished({ id: listed.id, username: listed.username }, snapshot), false);
    });
});
