/**
 * Lesson visibility service — modes, grants, one-time seed, admin/MCP mutations.
 * Provenance: INIT-002/SPEC-006
 */
'use strict';

const fs = require('fs');
const path = require('path');
const db = require('./db');
const visibilityRepo = require('./db/repos/visibility');
const audit = require('./db/repos/audit');
const { lessonsDir } = require('./paths');
const { requireAdmin } = require('./auth/policy');

/** Same rule as mcp/store.js assertSlug (not exported from the store). */
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const META_SEED_KEY = 'visibility_seeded';
const MODES = new Set(['hidden', 'everyone', 'selected']);

function assertSlug(slug) {
    if (typeof slug !== 'string' || !SLUG.test(slug) || slug === 'template') {
        throw new Error(
            'slug must be lowercase words separated by hyphens, such as shai-date-indirect-speech.'
        );
    }
}

/**
 * @param {unknown} actor
 * @returns {boolean}
 */
function isPrivilegedActor(actor) {
    if (actor === 'mcp') return true;
    return requireAdmin(actor);
}

/**
 * @param {unknown} actor
 * @returns {string}
 */
function actorLabel(actor) {
    if (actor === 'mcp') return 'mcp';
    if (actor && typeof actor === 'object' && actor.username) {
        return String(actor.username);
    }
    if (actor && typeof actor === 'object' && actor.id != null) {
        return String(actor.id);
    }
    return String(actor);
}

/**
 * Real lesson folders under LESSONS_DIR (worksheet.js present; skip _* / .*).
 * @returns {string[]}
 */
function listRealLessonSlugs() {
    const base = lessonsDir();
    if (!fs.existsSync(base)) return [];
    return fs.readdirSync(base, { withFileTypes: true })
        .filter((entry) => entry.isDirectory()
            && !entry.name.startsWith('_')
            && !entry.name.startsWith('.'))
        .filter((entry) => fs.existsSync(path.join(base, entry.name, 'worksheet.js')))
        .map((entry) => entry.name)
        .filter((slug) => {
            try {
                assertSlug(slug);
                return true;
            } catch (_) {
                return false;
            }
        })
        .sort();
}

/**
 * One-time seed: insert `everyone` for every on-disk lesson with no row.
 * Later startups are no-ops (meta flag). New lessons after the first run stay
 * without a row → ADR-0002 default `hidden`.
 * @returns {{ seeded: boolean, inserted: string[] }}
 */
function seedVisibility() {
    const existing = db.get('SELECT value FROM meta WHERE key = ?', META_SEED_KEY);
    if (existing) {
        return { seeded: false, inserted: [] };
    }

    const inserted = [];
    db.transaction(() => {
        for (const slug of listRealLessonSlugs()) {
            if (visibilityRepo.getVisibility(slug)) continue;
            visibilityRepo.setVisibility(slug, 'everyone');
            inserted.push(slug);
        }
        db.run(
            'INSERT INTO meta (key, value) VALUES (?, ?)',
            META_SEED_KEY,
            '1'
        );
    });
    return { seeded: true, inserted };
}

/**
 * @param {{ role?: string, status?: string, id?: number } | null | undefined} user
 * @returns {boolean}
 */
function isAdminUser(user) {
    return Boolean(user && user.role === 'admin');
}

/**
 * @param {{ role?: string, id?: number } | null | undefined} user
 * @returns {string[]}
 */
function visibleSlugs(user) {
    const onDisk = listRealLessonSlugs();
    if (isAdminUser(user)) return onDisk.slice();

    const rows = visibilityRepo.listVisibility();
    const modeBySlug = new Map(rows.map((row) => [row.slug, row.mode]));
    const grantSlugs = new Set();
    if (user && user.id != null) {
        for (const grant of visibilityRepo.listGrantsForUser(user.id)) {
            grantSlugs.add(grant.slug);
        }
    }

    const visible = [];
    for (const slug of onDisk) {
        const mode = modeBySlug.get(slug);
        // No row → ADR-0002 default hidden (after the one-time seed).
        if (!mode || mode === 'hidden') continue;
        if (mode === 'everyone') {
            visible.push(slug);
            continue;
        }
        if (mode === 'selected' && grantSlugs.has(slug)) {
            visible.push(slug);
        }
    }
    return visible;
}

/**
 * @param {{ role?: string, id?: number } | null | undefined} user
 * @param {string} slug
 * @returns {boolean}
 */
function canSeeLesson(user, slug) {
    if (isAdminUser(user)) return true;
    if (typeof slug !== 'string' || !slug) return false;
    return visibleSlugs(user).includes(slug);
}

/**
 * @param {unknown} actor
 * @param {string} slug
 * @param {string} mode
 */
function setVisibility(actor, slug, mode) {
    if (!isPrivilegedActor(actor)) {
        throw new Error('setVisibility requires an admin user or the mcp actor');
    }
    assertSlug(slug);
    if (!MODES.has(mode)) {
        throw new Error('mode must be hidden, everyone, or selected');
    }
    visibilityRepo.setVisibility(slug, mode);
    audit.append({
        actor: actorLabel(actor),
        action: 'set_visibility',
        target: slug,
        detail: JSON.stringify({ mode })
    });
}

/**
 * Replace the grant set for a selected lesson (admin / mcp only).
 * @param {unknown} actor
 * @param {string} slug
 * @param {Array<number|string>} userIds
 */
function setGrants(actor, slug, userIds) {
    if (!isPrivilegedActor(actor)) {
        throw new Error('setGrants requires an admin user or the mcp actor');
    }
    assertSlug(slug);
    if (!Array.isArray(userIds)) {
        throw new Error('userIds must be an array');
    }
    const nextIds = userIds.map((id) => Number(id)).filter((id) => Number.isInteger(id) && id > 0);

    db.transaction(() => {
        const prior = visibilityRepo.listGrants(slug);
        for (const row of prior) {
            visibilityRepo.revoke(slug, row.user_id);
        }
        for (const id of nextIds) {
            visibilityRepo.grant(slug, id);
        }
    });

    audit.append({
        actor: actorLabel(actor),
        action: 'set_grants',
        target: slug,
        detail: JSON.stringify({ userIds: nextIds })
    });
}

module.exports = {
    assertSlug,
    listRealLessonSlugs,
    seedVisibility,
    visibleSlugs,
    canSeeLesson,
    setVisibility,
    setGrants,
    META_SEED_KEY
};
