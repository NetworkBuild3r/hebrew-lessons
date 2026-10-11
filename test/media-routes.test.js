/**
 * Media, generation, and approval HTTP routes.
 * Temporary DATA_DIR. The image transport is injected. https.request is tripwired.
 * Provenance: INIT-004/SPEC-011
 */
'use strict';

const { describe, it, before, after, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const https = require('https');
const path = require('path');

const harness = require('./helpers/signed-in-server');

const SAMPLE = path.join(
    harness.REPO_ROOT,
    'docs',
    'decisions',
    'ADR-0003-assets',
    'samples',
    'beginner-unit-1.plan.json'
);
const PRETTY = fs.readFileSync(SAMPLE, 'utf8');
const PNG = fs.readFileSync(path.join(__dirname, 'fixtures', 'media', 'tiny.png'));
const KEY = 'sk-spec011-do-not-print';
const PROMPT = 'a plain wooden chair';
const BOUNDARY = 'spec011boundary';

describe('media HTTP routes (INIT-004/SPEC-011)', { concurrency: false }, () => {
    /** @type {string} */
    let baseUrl;
    /** @type {(username: string) => string} */
    let cookieFor;
    /** @type {string} */
    let dataDir;
    /** @type {string} */
    let assigneeCookie;
    /** @type {string|undefined} */
    let priorKey;
    /** @type {typeof https.request} */
    let originalRequest;
    /** @type {number} */
    let httpsCalls;
    /** @type {Array<object>} */
    let transportCalls;

    before(async () => {
        priorKey = process.env.OPENAI_API_KEY;
        process.env.OPENAI_API_KEY = KEY;
        httpsCalls = 0;
        originalRequest = https.request;
        https.request = function tripped() {
            httpsCalls += 1;
            throw new Error('https.request attempted');
        };

        const started = await harness.start({ lessons: 'fixture' });
        baseUrl = started.baseUrl;
        cookieFor = started.cookieFor;
        dataDir = started.dataDir;
        const accounts = require('../server/accounts');
        const cookies = require('../server/http/cookies');
        accounts.createUser({
            username: 'assignee1',
            password: harness.GOOD_PASSWORD,
            role: 'user',
            displayName: 'Assignee',
            actor: 'test'
        });
        const assigneeSession = accounts.signIn('assignee1', harness.GOOD_PASSWORD, {
            ip: '127.0.0.1'
        });
        assigneeCookie = cookies.cookieName() + '=' + encodeURIComponent(assigneeSession.token);
        assert.equal(PNG.includes(Buffer.from(BOUNDARY)), false);
        assert.equal(PNG.includes(Buffer.from(KEY)), false);
    });

    after(async () => {
        https.request = originalRequest;
        if (priorKey === undefined) delete process.env.OPENAI_API_KEY;
        else process.env.OPENAI_API_KEY = priorKey;
        const routes = require('../server/api/media-routes');
        routes._setTransportForTests(null);
        await harness.stop();
    });

    beforeEach(() => {
        httpsCalls = 0;
        transportCalls = [];
        const routes = require('../server/api/media-routes');
        const ingest = require('../server/media/ingest');
        const images = require('../server/media/openai-images');
        routes._setTransportForTests(async (outbound) => {
            transportCalls.push({
                url: outbound && outbound.url,
                method: outbound && outbound.method,
                model: outbound && outbound.body && outbound.body.model
            });
            return {
                status: 200,
                body: { data: [{ b64_json: PNG.toString('base64') }] }
            };
        });
        ingest._resetUploadCounts();
        images._resetGenerationState();
        const db = require('../server/db');
        db.run('DELETE FROM media');
        db.run('DELETE FROM plans');
        db.run('DELETE FROM lesson_grants');
        db.run("DELETE FROM audit_log WHERE action LIKE 'media\\_%' ESCAPE '\\'");
    });

    function samplePlan(id, edit) {
        const plan = JSON.parse(PRETTY);
        plan.id = id;
        if (edit) edit(plan);
        return plan;
    }

    function setLessonVisibility(plan, lessonId, visibility) {
        for (const unit of plan.units) {
            for (const lesson of unit.lessons) {
                if (lesson.id === lessonId) lesson.visibility = visibility;
            }
        }
    }

    function seed(plan) {
        const store = require('../server/plans/store');
        store.saveWorkingCopy({
            id: plan.id,
            document: JSON.stringify(plan),
            revision: plan.revision,
            status: plan.status,
            audience: plan.audience,
            assigneeId: plan.assigneeId
        });
    }

    function insertPending(id) {
        const mediaRepo = require('../server/db/repos/media');
        mediaRepo.insert({
            id: id,
            storedName: 'ab'.repeat(32) + '.png',
            status: 'pending',
            altText: 'a cup of coffee',
            actor: 'admin:admin',
            promptText: 'do-not-copy-into-the-log'
        });
    }

    function mediaAudit() {
        const db = require('../server/db');
        return db.all(
            `SELECT actor, action, target, detail FROM audit_log
             WHERE action LIKE 'media\\_%' ESCAPE '\\'
             ORDER BY id ASC`
        );
    }

    function encodeForm(fields, file, filename) {
        const chunks = [];
        const names = Object.keys(fields);
        for (const name of names) {
            chunks.push(Buffer.from(
                '--' + BOUNDARY + '\r\n'
                + 'Content-Disposition: form-data; name="' + name + '"\r\n\r\n'
                + fields[name] + '\r\n'
            ));
        }
        const shown = filename == null ? 'x.png' : filename;
        chunks.push(Buffer.from(
            '--' + BOUNDARY + '\r\n'
            + 'Content-Disposition: form-data; name="file"; filename="' + shown + '"\r\n'
            + 'Content-Type: image/png\r\n\r\n'
        ));
        chunks.push(file);
        chunks.push(Buffer.from('\r\n--' + BOUNDARY + '--\r\n'));
        return {
            body: Buffer.concat(chunks),
            contentType: 'multipart/form-data; boundary=' + BOUNDARY
        };
    }

    /**
     * @param {string} method
     * @param {string} urlPath
     * @param {{ cookie?: string, body?: object, form?: { body: Buffer, contentType: string }, origin?: boolean, contentType?: boolean, headers?: Record<string, string> }} [opts]
     */
    async function api(method, urlPath, opts) {
        const options = opts || {};
        /** @type {Record<string, string>} */
        const headers = {};
        if (options.cookie) headers.Cookie = options.cookie;
        if (options.form) headers['Content-Type'] = options.form.contentType;
        else if (options.body !== undefined && options.contentType !== false) {
            headers['Content-Type'] = 'application/json';
        }
        if (options.origin !== false && method !== 'GET') headers.Origin = baseUrl;
        if (options.headers) Object.assign(headers, options.headers);
        const res = await fetch(baseUrl + urlPath, {
            method: method,
            headers: headers,
            body: options.form
                ? options.form.body
                : (options.body !== undefined ? JSON.stringify(options.body) : undefined)
        });
        const buf = Buffer.from(await res.arrayBuffer());
        const text = buf.toString('utf8');
        let json = null;
        if (text) {
            try {
                json = JSON.parse(text);
            } catch (_) {
                json = null;
            }
        }
        return { status: res.status, text: text, json: json, buf: buf, headers: res.headers };
    }

    function assertNoSecret(res) {
        assert.equal(res.buf.includes(Buffer.from(KEY)), false);
        assert.equal(res.text.includes('OPENAI_API_KEY'), false);
    }

    function uploadForm(id, extra) {
        const fields = { id: id };
        if (extra) Object.assign(fields, extra);
        return encodeForm(fields, PNG, '../worksheets/index.html');
    }

    async function publish(id, expectedRevision) {
        return api('POST', '/api/plans/' + id + '/publish', {
            cookie: cookieFor('admin'),
            body: { expectedRevision: expectedRevision }
        });
    }

    it('ac-1: signed-out is 401 and a user is 403, except user fetch of an unreferenced id is 404', async () => {
        const routes = require('../server/api/media-routes');
        const form = uploadForm('cup-pic');
        const cells = [
            { method: 'POST', path: '/api/media', form: form, user: 403 },
            { method: 'GET', path: '/api/media/cup-pic', user: 404 },
            { method: 'POST', path: '/api/admin/media/cup-pic/approval', body: {}, user: 403 },
            {
                method: 'GET',
                path: '/api/admin/images/image-plan?prompt=' + encodeURIComponent(PROMPT),
                user: 403
            },
            {
                method: 'POST',
                path: '/api/admin/images',
                body: { prompt: PROMPT, id: 'pic-1', confirm: false, estimate: 0.011 },
                user: 403
            },
            { method: 'POST', path: '/api/admin/media/cup-pic/delete', body: {}, user: 403 }
        ];
        assert.equal(routes.MUTATIONS.length, 4);
        for (const cell of cells) {
            const signedOut = await api(cell.method, cell.path, {
                form: cell.form,
                body: cell.body
            });
            assert.equal(signedOut.status, 401, cell.path);
            assert.equal(signedOut.json.code, 'no_session');
            assertNoSecret(signedOut);

            const user = await api(cell.method, cell.path, {
                cookie: cookieFor('learner'),
                form: cell.form,
                body: cell.body
            });
            assert.equal(user.status, cell.user, cell.path);
            if (cell.user === 403) assert.equal(user.json.code, 'forbidden');
            if (cell.user === 404) assert.equal(user.json.code, 'not_found');
            assertNoSecret(user);
        }
        const missing = await api('GET', '/api/media/does-not-exist', {
            cookie: cookieFor('admin')
        });
        assert.equal(missing.status, 404);
        assert.equal(missing.json.code, 'not_found');
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, 0);
        assert.equal(mediaAudit().length, 0);
        const mediaRepo = require('../server/db/repos/media');
        assert.equal(mediaRepo.getById('cup-pic'), null);

        const priorMcp = process.env.MCP_AUTH_TOKEN;
        process.env.MCP_AUTH_TOKEN = 'mcp-spec011-token';
        try {
            const mcp = await api('POST', '/api/admin/media/cup-pic/approval', {
                body: { altText: 'a cup of coffee' },
                headers: { Authorization: 'Bearer ' + process.env.MCP_AUTH_TOKEN }
            });
            assert.equal(mcp.status, 403);
            assert.equal(mcp.json.code, 'forbidden');
            assert.equal(mcp.text.includes(process.env.MCP_AUTH_TOKEN), false);
            assert.equal(mediaAudit().length, 0);
        } finally {
            if (priorMcp === undefined) delete process.env.MCP_AUTH_TOKEN;
            else process.env.MCP_AUTH_TOKEN = priorMcp;
        }
    });

    it('ac-1: admin upload is pending, self-approval is a second audit row, and delete is admin-only', async () => {
        const ingest = require('../server/media/ingest');
        const original = ingest.ingestImage;
        /** @type {Array<object|undefined>} */
        const optionLog = [];
        ingest.ingestImage = async function spied(input, options) {
            optionLog.push(options);
            assert.equal(Object.prototype.hasOwnProperty.call(input, 'filename'), false);
            assert.equal(input.actor, 'admin:admin');
            return original(input, options);
        };
        const approve = ingest.approveMedia;
        /** @type {string[]} */
        const kinds = [];
        ingest.approveMedia = function spiedApprove(request, deps) {
            kinds.push(request && request.actorKind);
            assert.equal(deps, undefined);
            return approve(request);
        };
        const remove = ingest.deleteMedia;
        ingest.deleteMedia = function spiedDelete(request, deps) {
            kinds.push(request && request.actorKind);
            assert.equal(deps, undefined);
            return remove(request);
        };

        try {
            const uploaded = await api('POST', '/api/media', {
                cookie: cookieFor('admin'),
                form: uploadForm('cup-pic', { altText: '' })
            });
            assert.equal(uploaded.status, 201, uploaded.text);
            assert.equal(uploaded.json.status, 'pending');
            assert.equal(uploaded.json.id, 'cup-pic');
            assert.equal(uploaded.json.altText, '');
            assert.equal(uploaded.text.includes('worksheets'), false);
            assert.equal(uploaded.text.includes('storedName'), false);
            assertNoSecret(uploaded);
            assert.equal(optionLog.length, 1);
            assert.equal(optionLog[0], undefined);

            const names = fs.readdirSync(path.join(dataDir, 'media'));
            assert.equal(names.length, 1);
            assert.match(names[0], /^[0-9a-f]{64}\.png$/);

            const adminFetch = await api('GET', '/api/media/cup-pic', {
                cookie: cookieFor('admin')
            });
            assert.equal(adminFetch.status, 200);
            assert.equal(adminFetch.headers.get('content-type'), 'image/png');
            assert.equal(adminFetch.headers.get('x-content-type-options'), 'nosniff');
            assert.equal(adminFetch.buf[0], 0x89);
            assert.equal(adminFetch.buf[1], 0x50);
            assert.equal(adminFetch.buf[2], 0x4e);
            assert.equal(adminFetch.buf[3], 0x47);

            const learnerFetch = await api('GET', '/api/media/cup-pic', {
                cookie: cookieFor('learner')
            });
            assert.equal(learnerFetch.status, 404);
            assert.equal(learnerFetch.json.code, 'not_found');

            const emptyAlt = await api('POST', '/api/admin/media/cup-pic/approval', {
                cookie: cookieFor('admin'),
                body: {}
            });
            assert.equal(emptyAlt.status, 400);
            assert.equal(emptyAlt.json.code, 'alt_text_required');
            assertNoSecret(emptyAlt);
            const mediaRepo = require('../server/db/repos/media');
            assert.equal(mediaRepo.getById('cup-pic').status, 'pending');

            const approved = await api('POST', '/api/admin/media/cup-pic/approval', {
                cookie: cookieFor('admin'),
                body: { altText: 'a cup of coffee' }
            });
            assert.equal(approved.status, 200, approved.text);
            assert.equal(approved.json.status, 'approved');
            assert.equal(approved.json.altText, 'a cup of coffee');
            assert.equal(approved.text.includes('do-not'), false);

            const rows = mediaAudit();
            assert.deepEqual(rows.map((row) => row.action), ['media_upload', 'media_approve']);
            assert.deepEqual(rows.map((row) => row.actor), ['admin:admin', 'admin:admin']);
            assert.deepEqual(rows.map((row) => row.target), ['cup-pic', 'cup-pic']);
            assert.deepEqual(rows.map((row) => row.detail), [null, null]);
            assert.equal(kinds[0], 'admin');

            const removed = await api('POST', '/api/admin/media/cup-pic/delete', {
                cookie: cookieFor('admin'),
                body: {}
            });
            assert.equal(removed.status, 200, removed.text);
            assert.deepEqual(removed.json, { id: 'cup-pic' });
            assert.equal(kinds[1], 'admin');
            const after = mediaAudit();
            assert.equal(after[after.length - 1].action, 'media_delete');
            assert.equal(after[after.length - 1].detail, null);
            const gone = await api('GET', '/api/media/cup-pic', { cookie: cookieFor('admin') });
            assert.equal(gone.status, 404);
            assert.equal(httpsCalls, 0);
            assert.equal(transportCalls.length, 0);
        } finally {
            ingest.ingestImage = original;
            ingest.approveMedia = approve;
            ingest.deleteMedia = remove;
        }
    });

    async function uploadApproved(id, alt) {
        const uploaded = await api('POST', '/api/media', {
            cookie: cookieFor('admin'),
            form: uploadForm(id, { altText: alt })
        });
        assert.equal(uploaded.status, 201, uploaded.text);
        const approved = await api('POST', '/api/admin/media/' + id + '/approval', {
            cookie: cookieFor('admin'),
            body: { altText: alt }
        });
        assert.equal(approved.status, 200, approved.text);
    }

    it('ac-1: a user fetch is 200 only when a published lesson they may see references the id', async () => {
        await uploadApproved('coffee-cup', 'a cup of coffee');

        const draft = samplePlan('draft-cup', (plan) => {
            setLessonVisibility(plan, 'cafe-order', 'everyone');
        });
        seed(draft);
        const draftFetch = await api('GET', '/api/media/coffee-cup', {
            cookie: cookieFor('learner')
        });
        assert.equal(draftFetch.status, 404);
        assert.equal(draftFetch.json.code, 'not_found');

        const hidden = samplePlan('hidden-cup');
        seed(hidden);
        const hiddenPub = await publish('hidden-cup', 1);
        assert.equal(hiddenPub.status, 200, hiddenPub.text);
        const hiddenFetch = await api('GET', '/api/media/coffee-cup', {
            cookie: cookieFor('learner')
        });
        assert.equal(hiddenFetch.status, 404);
        assert.equal(hiddenFetch.json.code, 'not_found');
        const adminHidden = await api('GET', '/api/media/coffee-cup', {
            cookie: cookieFor('admin')
        });
        assert.equal(adminHidden.status, 200);
        assert.equal(adminHidden.headers.get('content-type'), 'image/png');
        assert.equal(adminHidden.headers.get('x-content-type-options'), 'nosniff');

        const sharedPub = await publish('draft-cup', 1);
        assert.equal(sharedPub.status, 200, sharedPub.text);
        const seen = await api('GET', '/api/media/coffee-cup', { cookie: cookieFor('learner') });
        assert.equal(seen.status, 200);
        assert.equal(seen.headers.get('content-type'), 'image/png');
        assert.equal(seen.headers.get('x-content-type-options'), 'nosniff');
        assert.equal(seen.buf[0], 0x89);
        assert.equal(seen.buf[1], 0x50);

        await uploadApproved('sugar', 'sugar');
        const selected = samplePlan('selected-sugar', (plan) => {
            setLessonVisibility(plan, 'cafe-again', 'selected');
        });
        seed(selected);
        const selectedPub = await publish('selected-sugar', 1);
        assert.equal(selectedPub.status, 200, selectedPub.text);
        const ungranted = await api('GET', '/api/media/sugar', {
            cookie: cookieFor('learner')
        });
        assert.equal(ungranted.status, 404);
        assert.equal(ungranted.json.code, 'not_found');

        const visibilityRepo = require('../server/db/repos/visibility');
        const db = require('../server/db');
        const learner = db.get('SELECT id FROM users WHERE username = ?', 'learner');
        visibilityRepo.grant('cafe-again', learner.id);
        const granted = await api('GET', '/api/media/sugar', { cookie: cookieFor('learner') });
        assert.equal(granted.status, 200);
        assert.equal(granted.headers.get('content-type'), 'image/png');
        assert.equal(httpsCalls, 0);
    });

    it('ac-1: a personal plan image is visible to its assignee only', async () => {
        const uploaded = await api('POST', '/api/media', {
            cookie: cookieFor('admin'),
            form: uploadForm('milk', { altText: 'milk' })
        });
        assert.equal(uploaded.status, 201, uploaded.text);
        const personal = samplePlan('personal-milk', (plan) => {
            plan.audience = 'personal';
            plan.assigneeId = 'assignee1';
        });
        seed(personal);
        const personalPub = await publish('personal-milk', 1);
        assert.equal(personalPub.status, 200, personalPub.text);
        const stranger = await api('GET', '/api/media/milk', { cookie: cookieFor('learner') });
        assert.equal(stranger.status, 404);
        assert.equal(stranger.json.code, 'not_found');
        const assignee = await api('GET', '/api/media/milk', { cookie: assigneeCookie });
        assert.equal(assignee.status, 200);
        assert.equal(assignee.headers.get('content-type'), 'image/png');
        const admin = await api('GET', '/api/media/milk', { cookie: cookieFor('admin') });
        assert.equal(admin.status, 200);
    });

    it('ac-2: image-plan returns the estimate and generate without confirm records zero calls', async () => {
        const plan = await api(
            'GET',
            '/api/admin/images/image-plan?prompt=' + encodeURIComponent(PROMPT),
            { cookie: cookieFor('admin') }
        );
        assert.equal(plan.status, 200, plan.text);
        assert.equal(plan.json.count, 1);
        assert.equal(plan.json.estimatedDollars, 0.011);
        assertNoSecret(plan);
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, 0);

        const counted = await api(
            'GET',
            '/api/admin/images/image-plan?prompt=' + encodeURIComponent(PROMPT) + '&count=2',
            { cookie: cookieFor('admin') }
        );
        assert.equal(counted.status, 200, counted.text);
        assert.equal(counted.json.count, 2);
        assert.equal(counted.json.estimatedDollars, 0.022);
        assert.equal(transportCalls.length, 0);

        const missingConfirm = await api('POST', '/api/admin/images', {
            cookie: cookieFor('admin'),
            body: { prompt: PROMPT, id: 'pic-1', estimate: 0.011 }
        });
        assert.equal(missingConfirm.status, 400);
        assert.equal(missingConfirm.json.code, 'confirm_required');
        assertNoSecret(missingConfirm);
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, 0);

        const falseConfirm = await api('POST', '/api/admin/images', {
            cookie: cookieFor('admin'),
            body: { prompt: PROMPT, id: 'pic-1', confirm: false, estimate: 0.011 }
        });
        assert.equal(falseConfirm.status, 400);
        assert.equal(falseConfirm.json.code, 'confirm_required');
        assert.equal(transportCalls.length, 0);

        const generated = await api('POST', '/api/admin/images', {
            cookie: cookieFor('admin'),
            body: {
                prompt: PROMPT,
                id: 'pic-1',
                confirm: true,
                estimate: 0.011,
                altText: 'a chair'
            }
        });
        assert.equal(generated.status, 201, generated.text);
        assert.equal(generated.json.status, 'pending');
        assert.equal(generated.json.id, 'pic-1');
        assert.equal(generated.text.includes(PROMPT), false);
        assert.equal(generated.text.includes('gpt-image'), false);
        assertNoSecret(generated);
        assert.equal(transportCalls.length, 1);
        assert.equal(transportCalls[0].model, 'gpt-image-1');
        assert.equal(httpsCalls, 0);
        const mediaRepo = require('../server/db/repos/media');
        assert.equal(mediaRepo.getById('pic-1').promptText, PROMPT);
        const rows = mediaAudit();
        const blob = JSON.stringify(rows);
        assert.equal(blob.includes('chair'), false);
        assert.equal(blob.includes(KEY), false);
        assert.equal(rows[rows.length - 1].action, 'media_generate');
        assert.equal(rows[rows.length - 1].detail, null);
        assert.equal(rows[rows.length - 1].actor, 'admin:admin');
    });

    it('ac-3: everyone and selected refuse a pending image; hidden allows it', async () => {
        insertPending('coffee-cup');
        const everyone = samplePlan('everyone-cup', (plan) => {
            setLessonVisibility(plan, 'cafe-order', 'everyone');
        });
        seed(everyone);
        const refused = await publish('everyone-cup', 1);
        assert.equal(refused.status, 400);
        assert.equal(refused.json.code, 'pending_media');
        assert.equal(refused.text.includes('do-not-copy-into-the-log'), false);
        assertNoSecret(refused);
        const store = require('../server/plans/store');
        assert.equal(store.loadSnapshot('everyone-cup'), null);
        assert.equal(store.loadWorkingCopy('everyone-cup').revision, 1);

        const selected = samplePlan('selected-pending', (plan) => {
            setLessonVisibility(plan, 'cafe-order', 'selected');
        });
        seed(selected);
        const selectedRefused = await publish('selected-pending', 1);
        assert.equal(selectedRefused.status, 400);
        assert.equal(selectedRefused.json.code, 'pending_media');
        assert.equal(store.loadSnapshot('selected-pending'), null);

        const hidden = samplePlan('hidden-pending');
        seed(hidden);
        const allowed = await publish('hidden-pending', 1);
        assert.equal(allowed.status, 200, allowed.text);
        assert.equal(store.loadWorkingCopy('hidden-pending').status, 'published');
        assert.equal(typeof store.loadSnapshot('hidden-pending'), 'string');

        const mediaRepo = require('../server/db/repos/media');
        mediaRepo.update('coffee-cup', { status: 'approved' });
        const again = await publish('everyone-cup', 1);
        assert.equal(again.status, 200, again.text);
        assert.equal(store.loadWorkingCopy('everyone-cup').status, 'published');
    });

    it('security: CSRF, oversize, unknown fields, and a missing key do not call the provider or ingest', async () => {
        const form = uploadForm('cup-pic', { altText: 'a cup of coffee' });
        const noOrigin = await api('POST', '/api/media', {
            cookie: cookieFor('admin'),
            form: form,
            origin: false
        });
        assert.equal(noOrigin.status, 403);
        assert.equal(noOrigin.json.code, 'csrf');
        const jsonUpload = await api('POST', '/api/media', {
            cookie: cookieFor('admin'),
            body: { id: 'cup-pic' }
        });
        assert.equal(jsonUpload.status, 403);
        assert.equal(jsonUpload.json.code, 'csrf');
        const noOriginPlan = await api('POST', '/api/admin/images', {
            cookie: cookieFor('admin'),
            body: { prompt: PROMPT, id: 'pic-1', confirm: true, estimate: 0.011 },
            origin: false
        });
        assert.equal(noOriginPlan.status, 403);
        assert.equal(noOriginPlan.json.code, 'csrf');
        const noOriginApproval = await api('POST', '/api/admin/media/cup-pic/approval', {
            cookie: cookieFor('admin'),
            body: { altText: 'a cup of coffee' },
            origin: false
        });
        assert.equal(noOriginApproval.status, 403);
        assert.equal(noOriginApproval.json.code, 'csrf');
        const noOriginDelete = await api('POST', '/api/admin/media/cup-pic/delete', {
            cookie: cookieFor('admin'),
            body: {},
            origin: false
        });
        assert.equal(noOriginDelete.status, 403);
        assert.equal(noOriginDelete.json.code, 'csrf');

        const extra = encodeForm({ id: 'cup-pic', now: '1', altText: 'a cup' }, PNG, 'x.png');
        const rejected = await api('POST', '/api/media', {
            cookie: cookieFor('admin'),
            form: extra
        });
        assert.equal(rejected.status, 400);
        assert.equal(rejected.json.code, 'bad_request');
        const mediaRepo = require('../server/db/repos/media');
        assert.equal(mediaRepo.getById('cup-pic'), null);

        const smuggled = await api('POST', '/api/admin/images', {
            cookie: cookieFor('admin'),
            body: {
                prompt: PROMPT,
                id: 'pic-1',
                confirm: true,
                estimate: 0.011,
                now: 1,
                codec: 'sharp',
                transport: 'https',
                actor: 'mcp'
            }
        });
        assert.equal(smuggled.status, 400);
        assert.equal(smuggled.json.code, 'bad_request');
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, 0);

        const query = await api(
            'GET',
            '/api/admin/images/image-plan?prompt=' + encodeURIComponent(PROMPT) + '&now=1',
            { cookie: cookieFor('admin') }
        );
        assert.equal(query.status, 400);
        assert.equal(query.json.code, 'bad_request');
        assert.equal(transportCalls.length, 0);

        delete process.env.OPENAI_API_KEY;
        try {
            const dry = await api(
                'GET',
                '/api/admin/images/image-plan?prompt=' + encodeURIComponent(PROMPT),
                { cookie: cookieFor('admin') }
            );
            assert.equal(dry.status, 503);
            assert.equal(dry.json.code, 'missing_key');
            assert.equal(dry.text.includes('sk-'), false);
            const gen = await api('POST', '/api/admin/images', {
                cookie: cookieFor('admin'),
                body: { prompt: PROMPT, id: 'pic-1', confirm: true, estimate: 0.011 }
            });
            assert.equal(gen.status, 503);
            assert.equal(gen.json.code, 'missing_key');
            assert.equal(gen.text.includes('sk-'), false);
            assert.equal(transportCalls.length, 0);
            assert.equal(httpsCalls, 0);
        } finally {
            process.env.OPENAI_API_KEY = KEY;
        }

        const echoed = await api(
            'GET',
            '/api/admin/images/image-plan?prompt=' + encodeURIComponent('see ' + KEY),
            { cookie: cookieFor('admin') }
        );
        assert.equal(echoed.status, 400);
        assert.equal(echoed.json.code, 'prompt_refused');
        assertNoSecret(echoed);
        assert.equal(transportCalls.length, 0);

        const huge = Buffer.alloc(ingestLimit() + 1, 0x61);
        let hugeStatus = 'aborted';
        try {
            const over = await api('POST', '/api/media', {
                cookie: cookieFor('admin'),
                form: {
                    body: huge,
                    contentType: 'multipart/form-data; boundary=' + BOUNDARY
                }
            });
            hugeStatus = over.status;
            assertNoSecret(over);
        } catch (_) {
            hugeStatus = 'aborted';
        }
        assert.ok(hugeStatus === 413 || hugeStatus === 'aborted', String(hugeStatus));
        assert.equal(mediaRepo.getById('cup-pic'), null);
        assert.equal(transportCalls.length, 0);
        assert.equal(httpsCalls, 0);
        assert.equal(mediaAudit().filter((row) => row.action === 'media_upload').length, 0);
    });
});

function ingestLimit() {
    return require('../server/media/ingest').MAX_INPUT_BYTES;
}
