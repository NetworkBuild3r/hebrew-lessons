/**
 * Starts the signed-in app (ADR-0007): paint the shell once, load server state
 * once, then hand the main region to the router.
 *
 * Nothing screen-specific belongs here. Screens are described in
 * shared/app-routes.js and implemented as page modules.
 *
 * Provenance: INIT-005/SPEC-002
 */
(function (root) {
    'use strict';

    function boot() {
        var doc = root.document;
        var user = root.__HEBREW_ME__ || null;

        if (root.HebrewShell) {
            root.HebrewShell.mount(doc.getElementById('app-shell'), { me: user });
        }

        // One HebrewSync.init() per page load. Views await ctx.ready instead of calling init() again.
        var Sync = root.HebrewSync;
        var ready = Sync && typeof Sync.init === 'function'
            ? Promise.resolve(Sync.init({})).then(function (u) {
                if (typeof Sync.paintStatus === 'function') Sync.paintStatus();
                return u;
            }).catch(function () { return null; })
            : Promise.resolve(null);

        root.HebrewRouter.start({
            routes: root.HebrewAppRoutes || [],
            main: doc.getElementById('app-main'),
            user: user,
            ready: ready
        });
    }

    if (root.document.readyState === 'loading') {
        root.document.addEventListener('DOMContentLoaded', boot);
    } else {
        boot();
    }
})(window);
