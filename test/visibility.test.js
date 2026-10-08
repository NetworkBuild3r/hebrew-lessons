/**
 * Lesson visibility service tests.
 * Provenance: INIT-002/SPEC-006
 */
'use strict';

const { describe, it, before, after, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const root = path.resolve(__dirname, '..');

const EVERYONE = 'public-lesson';
const SELECTED = 'selected-lesson';
const HIDDEN = 'hidden-lesson';

function writeWorksheet(dir, slug, title, words) {
    const lesson = path.join(dir, slug);
    fs.mkdirSync(lesson, { recursive: true });
    const items = words.map((word, i) => ({
        id: String(i + 1),
        word,
        meaning: 'm-' + word,
        sound: '',
        audio: ''
    }));
    const worksheet = {
        id: slug,
        title,
        level: 'basic',
        vocab: { items },
        reading: { paragraphs: [words.join(' ')] },
        activities: [{ type: 'words' }]
    };
    fs.writeFileSync(
        path.join(lesson, 'worksheet.js'),
        'window.WORKSHEET = ' + JSON.stringify(worksheet, null, 4) + ';\n',
        'utf8'
    );
}

function clearModules() {
    const names = [
        '../server/paths',
        '../server/db',
        '../server/db/migrations/001-initial',
        '../server/db/repos/visibility',
        '../server/db/repos/audit',
        '../server/db/repos/users',
        '../server/auth/policy',
        '../server/visibility'
    ];
    for (const name of names) {
        try {
            delete require.cache[require.resolve(name)];
        } catch (_) { /* not loaded */ }
    }
}

describe('visibility service (SPEC-006)', () => {
    let tmpData;
    let tmpLessons;
    let db;
    let visibility;
    let visibilityRepo;
    let audit;
    let users;
    let learner;
    let admin;
    let grantedUser;

    before(() => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-vis-data-'));
        tmpLessons = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-vis-lessons-'));

        writeWorksheet(tmpLessons, EVERYONE, 'Public Title', ['שָׁלוֹם', 'עוֹלָם']);
        writeWorksheet(tmpLessons, SELECTED, 'Selected Title', ['סוֹד', 'מֻגְבָּל']);
        writeWorksheet(tmpLessons, HIDDEN, 'Hidden Title', ['נֶעֱלָם', 'חָסוּי']);

        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmpData;
        process.env.LESSONS_DIR = tmpLessons;
        clearModules();

        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();

        visibilityRepo = require('../server/db/repos/visibility');
        audit = require('../server/db/repos/audit');
        users = require('../server/db/repos/users');
        visibility = require('../server/visibility');

        const learnerRow = users.create({
            username: 'learner',
            password_hash: 'x',
            display_name: 'Learner',
            role: 'user'
        });
        learner = users.findById(Number(learnerRow.lastInsertRowid));

        const adminRow = users.create({
            username: 'admin',
            password_hash: 'x',
            display_name: 'Admin',
            role: 'admin'
        });
        admin = users.findById(Number(adminRow.lastInsertRowid));

        const grantedRow = users.create({
            username: 'granted',
            password_hash: 'x',
            display_name: 'Granted',
            role: 'user'
        });
        grantedUser = users.findById(Number(grantedRow.lastInsertRowid));

        visibilityRepo.setVisibility(EVERYONE, 'everyone');
        visibilityRepo.setVisibility(SELECTED, 'selected');
        visibilityRepo.setVisibility(HIDDEN, 'hidden');
        visibilityRepo.grant(SELECTED, grantedUser.id);
    });

    after(() => {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
        }
        try {
            fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        try {
            fs.rmSync(tmpLessons, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
        } catch (_) { /* ignore */ }
        delete process.env.DATA_DIR;
        delete process.env.LESSONS_DIR;
        clearModules();
    });

    it('ac-2/modes: everyone visible; selected only with grant; hidden never; admin sees all', () => {
        const plain = visibility.visibleSlugs(learner);
        assert.deepEqual(plain, [EVERYONE]);
        assert.equal(visibility.canSeeLesson(learner, EVERYONE), true);
        assert.equal(visibility.canSeeLesson(learner, SELECTED), false);
        assert.equal(visibility.canSeeLesson(learner, HIDDEN), false);

        const withGrant = visibility.visibleSlugs(grantedUser);
        assert.ok(withGrant.includes(EVERYONE));
        assert.ok(withGrant.includes(SELECTED));
        assert.ok(!withGrant.includes(HIDDEN));

        const adminSlugs = visibility.visibleSlugs(admin);
        assert.ok(adminSlugs.includes(EVERYONE));
        assert.ok(adminSlugs.includes(SELECTED));
        assert.ok(adminSlugs.includes(HIDDEN));
        assert.equal(visibility.canSeeLesson(admin, HIDDEN), true);
    });

    it('no visibility row defaults to hidden (ADR-0002)', () => {
        writeWorksheet(tmpLessons, 'unset-lesson', 'Unset', ['חָדָשׁ']);
        assert.equal(visibility.canSeeLesson(learner, 'unset-lesson'), false);
        assert.ok(!visibility.visibleSlugs(learner).includes('unset-lesson'));
        assert.equal(visibility.canSeeLesson(admin, 'unset-lesson'), true);
        fs.rmSync(path.join(tmpLessons, 'unset-lesson'), { recursive: true, force: true });
    });

    it('ac-6: setVisibility / setGrants refuse non-admin, reject bad slugs, audit; mcp ok', () => {
        assert.throws(
            () => visibility.setVisibility(learner, EVERYONE, 'hidden'),
            /admin|mcp/i
        );
        assert.throws(
            () => visibility.setGrants(learner, SELECTED, [learner.id]),
            /admin|mcp/i
        );
        assert.throws(
            () => visibility.setVisibility(admin, '../evil', 'hidden'),
            /slug/i
        );
        assert.throws(
            () => visibility.setGrants(admin, 'Not A Slug', [1]),
            /slug/i
        );

        const before = audit.listRecent(50).length;
        visibility.setVisibility(admin, SELECTED, 'selected');
        visibility.setGrants(admin, SELECTED, [grantedUser.id, learner.id]);
        const afterAdmin = audit.listRecent(50);
        assert.ok(afterAdmin.length >= before + 2);
        const visRow = afterAdmin.find((r) => r.action === 'set_visibility' && r.target === SELECTED);
        assert.ok(visRow);
        assert.equal(visRow.actor, 'admin');
        const grantRow = afterAdmin.find((r) => r.action === 'set_grants' && r.target === SELECTED);
        assert.ok(grantRow);
        assert.equal(grantRow.actor, 'admin');

        visibility.setVisibility('mcp', HIDDEN, 'everyone');
        visibility.setGrants('mcp', HIDDEN, []);
        const mcpRows = audit.listByActor('mcp', 10);
        assert.ok(mcpRows.some((r) => r.action === 'set_visibility' && r.target === HIDDEN));
        assert.ok(mcpRows.some((r) => r.action === 'set_grants' && r.target === HIDDEN));
        // restore fixture modes for other tests
        visibilityRepo.setVisibility(HIDDEN, 'hidden');
        visibilityRepo.setVisibility(SELECTED, 'selected');
        visibility.setGrants(admin, SELECTED, [grantedUser.id]);
    });
});

describe('visibility seed (ac-7)', () => {
    let tmpData;
    let db;
    let visibility;
    let visibilityRepo;

    beforeEach(() => {
        tmpData = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-vis-seed-'));
        process.env.HEBREW_ROOT = root;
        process.env.DATA_DIR = tmpData;
        delete process.env.LESSONS_DIR; // real worksheets/ with three lessons
        clearModules();
        db = require('../server/db');
        db.close();
        db.open({ force: true });
        db.migrate();
        visibilityRepo = require('../server/db/repos/visibility');
        visibility = require('../server/visibility');
    });

    afterEach(() => {
        if (db) {
            try { db.close(); } catch (_) { /* ignore */ }
            db = null;
        }
        if (tmpData) {
            try {
                fs.rmSync(tmpData, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
            } catch (_) { /* Windows may briefly lock WAL files */ }
            tmpData = null;
        }
        delete process.env.DATA_DIR;
        delete process.env.LESSONS_DIR;
        clearModules();
    });

    it('first startup seeds three existing lessons as everyone; second run no-ops; later lesson stays hidden', () => {
        const first = visibility.seedVisibility();
        assert.equal(first.seeded, true);
        for (const slug of ['my-morning', 'shai-date-indirect-speech', 'anal-sex-vocab']) {
            const row = visibilityRepo.getVisibility(slug);
            assert.ok(row, 'expected visibility row for ' + slug);
            assert.equal(row.mode, 'everyone');
        }

        const second = visibility.seedVisibility();
        assert.equal(second.seeded, false);
        assert.deepEqual(second.inserted, []);

        // Lesson added after the seed: no row → ADR default hidden; second seed no-ops.
        const overlay = fs.mkdtempSync(path.join(os.tmpdir(), 'hebrew-vis-overlay-'));
        try {
            for (const slug of ['my-morning', 'shai-date-indirect-speech', 'anal-sex-vocab']) {
                fs.cpSync(
                    path.join(root, 'worksheets', slug),
                    path.join(overlay, slug),
                    { recursive: true }
                );
            }
            fs.mkdirSync(path.join(overlay, 'brand-new-lesson'), { recursive: true });
            fs.writeFileSync(
                path.join(overlay, 'brand-new-lesson', 'worksheet.js'),
                'window.WORKSHEET = ' + JSON.stringify({
                    id: 'brand-new-lesson',
                    title: 'New',
                    vocab: { items: [{ word: 'חָדָשׁ', meaning: 'new' }] },
                    reading: { paragraphs: ['חָדָשׁ'] }
                }) + ';\n'
            );
            fs.copyFileSync(
                path.join(root, 'worksheets', 'catalog.js'),
                path.join(overlay, 'catalog.js')
            );

            db.close();
            clearModules();
            process.env.LESSONS_DIR = overlay;
            process.env.DATA_DIR = tmpData;
            db = require('../server/db');
            db.open({ force: true });
            visibilityRepo = require('../server/db/repos/visibility');
            visibility = require('../server/visibility');

            const again = visibility.seedVisibility();
            assert.equal(again.seeded, false);
            assert.equal(visibilityRepo.getVisibility('brand-new-lesson'), undefined);
            const learner = { id: 1, role: 'user', status: 'active' };
            assert.equal(visibility.canSeeLesson(learner, 'brand-new-lesson'), false);
            assert.equal(visibility.canSeeLesson(learner, 'anal-sex-vocab'), true);
        } finally {
            try { db.close(); } catch (_) { /* ignore */ }
            try {
                fs.rmSync(overlay, { recursive: true, force: true, maxRetries: 10, retryDelay: 25 });
            } catch (_) { /* ignore */ }
        }
    });
});
