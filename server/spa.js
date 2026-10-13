/**
 * Serves the single shell document for every signed-in app screen (ADR-0007).
 *
 * When you add a screen in shared/app-routes.js, add its path to APP_PATHS
 * here too, or a refresh on that screen returns 404.
 * Gating (sign-in, admin role) happens in server/web.js BEFORE this is called.
 *
 * Provenance: INIT-005/SPEC-002
 */
'use strict';

const fs = require('fs');
const path = require('path');
const { rootDir } = require('./paths');

const TEMPLATE_PATH = path.join(rootDir(), 'shared', 'app.html');

/** Canonical app paths. Must match `path` in shared/app-routes.js. */
const APP_PATHS = ['/', '/lessons/', '/guides/', '/review/', '/words/', '/progress/', '/admin/'];

let cachedTemplate = null;
let cachedMtimeMs = 0;

/**
 * "/review", "/review/", "/review/index.html" → "/review/". "/index.html" → "/".
 * Same rule as HebrewRouter.normalize on the client.
 * @param {string} rel decoded request path
 * @returns {string}
 */
function normalizeAppPath(rel) {
    let p = String(rel || '/');
    if (p === '' || p === '/index.html') return '/';
    if (p.endsWith('/index.html')) p = p.slice(0, -'index.html'.length);
    if (!p.endsWith('/')) p += '/';
    return p;
}

/**
 * @param {string} rel decoded request path
 * @returns {boolean}
 */
function isAppPath(rel) {
    return APP_PATHS.includes(normalizeAppPath(rel));
}

/** Trees that are real files or APIs. A missing file there stays a plain 404. */
const FILE_TREES = ['/shared/', '/worksheets/', '/login/', '/api/'];

/**
 * A signed-in browser navigation to a path that is not an app screen and not a file
 * tree ("/nope/", "/review/typo"). The server answers with the shell and HTTP 404;
 * the client router then shows a not-found view inside the shell (REQ-006).
 * Missing files (anything with an extension, or under /shared/, /worksheets/, /login/)
 * are NOT matched, so they keep returning the plain-text 404.
 * @param {string} rel decoded request path
 * @returns {boolean}
 */
function isUnknownAppPath(rel) {
    const p = String(rel || '/');
    if (isAppPath(p)) return false;
    if (p === '/login' || p === '/shared' || p === '/worksheets' || p === '/api') return false;
    if (FILE_TREES.some((tree) => p.startsWith(tree))) return false;
    const last = p.split('/').filter(Boolean).pop() || '';
    if (last.includes('.')) return false;
    return true;
}

/**
 * Same public shape as GET /api/me (server/api/auth-routes.js mePayload).
 * Never add password hashes, session tokens, or other private fields.
 * @param {object} user
 * @returns {object}
 */
function mePayload(user) {
    return {
        id: user.id,
        username: user.username,
        display_name: user.display_name,
        role: user.role,
        must_change_password: Boolean(user.must_change_password)
    };
}

const BOOT_PLACEHOLDER = '%%HEBREW_BOOT_JSON%%';

function readTemplate() {
    const stat = fs.statSync(TEMPLATE_PATH);
    if (!cachedTemplate || stat.mtimeMs !== cachedMtimeMs) {
        cachedTemplate = fs.readFileSync(TEMPLATE_PATH, 'utf8');
        cachedMtimeMs = stat.mtimeMs;
    }
    return cachedTemplate;
}

/**
 * Shell HTML with the user embedded as JSON. `<` is escaped so a display name
 * cannot close the <script> element.
 * @param {object} user
 * @returns {string}
 */
function renderShell(user) {
    const json = JSON.stringify(mePayload(user)).replace(/</g, '\\u003c');
    const parts = readTemplate().split(BOOT_PLACEHOLDER);
    if (parts.length !== 2) {
        throw new Error('shared/app.html must contain ' + BOOT_PLACEHOLDER + ' exactly once');
    }
    // join, not String.replace: a display name containing "$&" must stay literal.
    return parts.join(json);
}

module.exports = {
    APP_PATHS,
    normalizeAppPath,
    isAppPath,
    isUnknownAppPath,
    mePayload,
    renderShell,
    TEMPLATE_PATH
};
