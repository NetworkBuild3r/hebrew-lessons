/**
 * Client router for the signed-in app (ADR-0007). Hand-built, no dependency.
 *
 * What it guarantees on every in-app navigation:
 *   - the shell (top nav, account menu) is never re-rendered;
 *   - the new heading appears at once, before any data loads;
 *   - the old view is unmounted and every listener it added with ctx.signal is removed;
 *   - title, aria-current, focus, a screen-reader announcement, and scroll are updated;
 *   - back/forward restore the previous scroll position.
 *
 * VIEW CONTRACT (every page module must follow this):
 *   mount(el, ctx)   Fill `el` (the route's view root). May return a Promise.
 *                    ctx = { route, user, signal, actions, ready, navigate }
 *                      signal   AbortSignal. Pass it to every listener you add
 *                               OUTSIDE `el`: document/window.addEventListener(type, fn, { signal: ctx.signal }).
 *                               It is aborted when the route closes.
 *                      actions  the heading's actions slot (buttons next to the h1), or null.
 *                      ready    Promise that resolves once HebrewSync has loaded server state.
 *                               Await it instead of calling HebrewSync.init() yourself.
 *                      navigate (path) → in-app navigation.
 *   unmount()        Optional. Stop timers and audio. Listeners on ctx.signal are already gone.
 *   canLeave()       Optional. Return false to block navigation (e.g. unsaved edits).
 *                    Ask the user yourself (confirm) and return their answer.
 *
 * Ready signal: once mount() settles, #app-main gets data-route-ready="<route id>".
 * Tests and Chrome smokes wait for that attribute, not for a global to appear.
 *
 * Links: use a plain <a href="/review/">. The router intercepts same-origin
 * left-clicks on known routes, so open-in-new-tab and copy-link keep working.
 * Do not use onclick="location=..." or buttons for navigation.
 *
 * Provenance: INIT-005/SPEC-002
 */
(function (root) {
    'use strict';

    var routes = [];
    var current = null;      // { route, view, controller }
    var mainEl = null;
    var started = false;
    var navSeq = 0;
    var user = null;
    var readyPromise = Promise.resolve(null);
    var scriptCache = Object.create(null);  // url → Promise

    // ---- paths ---------------------------------------------------------

    /** "/review", "/review/index.html", "/review/" → "/review/". "/" and "/index.html" → "/". */
    function normalize(pathname) {
        var p = String(pathname || '/').split('?')[0].split('#')[0];
        if (p === '' || p === '/index.html') return '/';
        if (/\/index\.html$/.test(p)) p = p.slice(0, -'index.html'.length);
        if (p.charAt(p.length - 1) !== '/') p += '/';
        return p;
    }

    function find(pathname) {
        var p = normalize(pathname);
        for (var i = 0; i < routes.length; i += 1) {
            if (routes[i].path === p) return routes[i];
        }
        return null;
    }

    function canOpen(route) {
        return !route.adminOnly || (user && user.role === 'admin');
    }

    // ---- scripts -------------------------------------------------------

    function alreadyInPage(url) {
        var doc = root.document;
        if (!doc || !doc.querySelectorAll) return false;
        var tags = doc.querySelectorAll('script[src]');
        for (var i = 0; i < tags.length; i += 1) {
            var src = tags[i].getAttribute('src') || '';
            if (src === url) return true;
        }
        return false;
    }

    /**
     * Load script URLs once each. They download in parallel and run in list order
     * (async=false on injected scripts keeps insertion order).
     * @param {string[]} urls
     * @returns {Promise<void>}
     */
    function loadScripts(urls) {
        var doc = root.document;
        var pending = (urls || []).map(function (url) {
            if (scriptCache[url]) return scriptCache[url];
            if (alreadyInPage(url)) {
                scriptCache[url] = Promise.resolve();
                return scriptCache[url];
            }
            scriptCache[url] = new Promise(function (resolve, reject) {
                var el = doc.createElement('script');
                el.src = url;
                el.async = false;
                el.onload = function () { resolve(); };
                el.onerror = function () {
                    delete scriptCache[url];
                    reject(new Error('Could not load ' + url));
                };
                (doc.head || doc.documentElement).appendChild(el);
            });
            return scriptCache[url];
        });
        return Promise.all(pending).then(function () {});
    }

    /** After the first screen is up, warm the scripts for every other screen this user can open. */
    function prefetchOthers() {
        var run = function () {
            routes.forEach(function (route) {
                if (canOpen(route)) loadScripts(route.scripts).catch(function () { /* retried on visit */ });
            });
        };
        if (typeof root.requestIdleCallback === 'function') root.requestIdleCallback(run, { timeout: 2000 });
        else setTimeout(run, 300);
    }

    // ---- page chrome ---------------------------------------------------

    function setBodyClass(prev, next) {
        var body = root.document && root.document.body;
        if (!body || !body.classList) return;
        if (prev && prev.bodyClass) body.classList.remove(prev.bodyClass);
        if (next && next.bodyClass) body.classList.add(next.bodyClass);
    }

    function announce(text) {
        var el = root.document && root.document.getElementById('app-route-announce');
        if (!el) return;
        el.textContent = '';
        setTimeout(function () { el.textContent = String(text || ''); }, 30);
    }

    function focusHeading() {
        if (!mainEl) return;
        var h1 = mainEl.querySelector && mainEl.querySelector('h1');
        var target = h1 || mainEl;
        try { target.focus({ preventScroll: true }); } catch (err) {
            try { target.focus(); } catch (e2) { /* ignore */ }
        }
    }

    function reducedMotion() {
        try {
            return !!(root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)').matches);
        } catch (err) {
            return false;
        }
    }

    /**
     * Replace the main region synchronously, then play a short fade (CSS .route-enter).
     * Keep this synchronous: navigate() reads the new DOM on the next line.
     */
    function swap(html) {
        mainEl.innerHTML = html;
        if (!mainEl.classList) return;
        mainEl.classList.remove('route-enter');
        if (reducedMotion()) return;   // no class, so no animation at all (CSS also forces animation: none)
        void mainEl.offsetWidth;  // restart the animation
        mainEl.classList.add('route-enter');
    }

    function saveScroll() {
        try {
            var state = Object.assign({}, root.history.state || {}, { scrollY: root.scrollY || 0 });
            root.history.replaceState(state, '', root.location.href);
        } catch (err) { /* ignore */ }
    }

    // ---- lifecycle -----------------------------------------------------

    function leaveCurrent() {
        if (!current) return;
        try { if (current.controller) current.controller.abort(); } catch (err) { /* ignore */ }
        try {
            if (current.view && typeof current.view.unmount === 'function') current.view.unmount();
        } catch (err2) {
            if (root.console) root.console.error('[router] unmount failed', err2);
        }
        var Speech = root.HebrewSpeech;
        if (Speech && typeof Speech.stopAll === 'function') Speech.stopAll();
    }

    function showMessage(he, en, bodyHe, bodyEn) {
        var Layout = root.HebrewLayout;
        swap(Layout.message(he, en, bodyHe, bodyEn));
        if (root.document) root.document.title = he + ' — ' + en + ' · דרך עברית';
        announce(he);
        focusHeading();
    }

    /** No nav link is "current" on a screen that does not exist. */
    function clearActiveNav() {
        var doc = root.document;
        if (!doc || !doc.querySelectorAll) return;
        var links = doc.querySelectorAll('.hebrew-shell [data-nav][aria-current]');
        for (var i = 0; i < links.length; i += 1) links[i].removeAttribute('aria-current');
    }

    /**
     * The server sends the shell with HTTP 404 for an unknown screen URL (server/spa.js
     * isUnknownAppPath). Show "not found" inside the shell: heading focused, title and
     * announcement set, nav painted once and not marked current. The URL is left as typed.
     * Provenance: INIT-005/SPEC-002 ac-2 (REQ-006)
     */
    function showNotFound() {
        var prevRoute = current && current.route;
        leaveCurrent();
        setBodyClass(prevRoute, null);
        current = { route: null, url: root.location ? root.location.pathname : '', view: null, controller: null };
        navSeq += 1;
        if (mainEl.removeAttribute) mainEl.removeAttribute('data-route-ready');
        clearActiveNav();
        showMessage('העמוד לא נמצא', 'Page not found',
            'לא מצאנו את העמוד הזה. אפשר לחזור להיום ולהמשיך משם.',
            'We could not find that page. Go back to Today and carry on from there.');
        try { root.scrollTo(0, 0); } catch (err) { /* ignore */ }
        if (mainEl.setAttribute) mainEl.setAttribute('data-route-ready', 'not-found');
    }

    /**
     * Split a path that may include a query and a hash.
     * The query stays on the history URL so a screen can restore filters on Back.
     * Provenance: INIT-006/SPEC-013
     * @param {string} raw
     * @returns {{ path: string, search: string, hash: string }}
     */
    function splitTarget(raw) {
        var s = String(raw || '');
        var hash = '';
        var hashAt = s.indexOf('#');
        if (hashAt >= 0) {
            hash = s.slice(hashAt);
            s = s.slice(0, hashAt);
        }
        var search = '';
        var queryAt = s.indexOf('?');
        if (queryAt >= 0) {
            search = s.slice(queryAt);
            s = s.slice(0, queryAt);
        }
        return { path: s, search: search, hash: hash };
    }

    /**
     * Open a route. Returns false when the path is not an app route, so the caller
     * can let the browser do a normal page load (lesson pages, login, files).
     * A query string on `pathname` is kept (`/words/?binyan=hifil`).
     * @param {string} pathname
     * @param {{ replace?: boolean, popstate?: boolean, scrollY?: number, keepHash?: boolean }} [opts]
     */
    function navigate(pathname, opts) {
        var options = opts || {};
        var parts = splitTarget(pathname);
        var route = find(parts.path);
        if (!route) return false;

        // popstate already asked in onPopState.
        if (current && current.view && typeof current.view.canLeave === 'function' && !options.popstate) {
            if (current.view.canLeave() === false) return true;
        }

        var seq = ++navSeq;
        var hash = parts.hash
            ? parts.hash
            : (options.keepHash && root.location ? (root.location.hash || '') : '');
        var url = route.path + parts.search + hash;

        if (!options.popstate) {
            saveScroll();
            try {
                if (options.replace) root.history.replaceState({ scrollY: 0 }, '', url);
                else root.history.pushState({ scrollY: 0 }, '', url);
            } catch (err) { /* ignore */ }
        }

        var prevRoute = current && current.route;
        leaveCurrent();
        setBodyClass(prevRoute, route);

        if (root.HebrewShell && typeof root.HebrewShell.setActiveNav === 'function') {
            root.HebrewShell.setActiveNav(route.path);
        }

        if (!canOpen(route)) {
            current = { route: route, url: url, view: null, controller: null };
            showMessage('אין גישה', 'No access', 'העמוד הזה למנהלים בלבד.', 'This page is for admins only.');
            return true;
        }

        var controller = typeof root.AbortController === 'function' ? new root.AbortController() : null;
        current = { route: route, url: url, view: null, controller: controller };

        if (mainEl.removeAttribute) mainEl.removeAttribute('data-route-ready');
        swap(root.HebrewLayout.page(route));
        if (root.document) root.document.title = route.title;
        announce(route.announce);
        focusHeading();
        if (options.popstate) {
            try { root.scrollTo(0, options.scrollY || 0); } catch (err) { /* ignore */ }
        } else {
            try { root.scrollTo(0, 0); } catch (err) { /* ignore */ }
        }

        loadScripts(route.scripts).then(function () {
            if (seq !== navSeq) return null;   // a newer navigation won
            var view = root[route.view];
            if (!view || typeof view.mount !== 'function') {
                throw new Error('View ' + route.view + ' has no mount()');
            }
            current.view = view;
            var el = root.document.getElementById(route.rootId);
            var actions = mainEl.querySelector('[data-page-actions]');
            return view.mount(el, {
                route: route,
                user: user,
                signal: controller ? controller.signal : undefined,
                actions: actions,
                ready: readyPromise,
                navigate: function (p) { return navigate(p); }
            });
        }).then(function () {
            if (seq !== navSeq) return;
            if (mainEl.setAttribute) mainEl.setAttribute('data-route-ready', route.id);
            if (options.popstate && options.scrollY) {
                try { root.scrollTo(0, options.scrollY); } catch (err) { /* ignore */ }
            }
        }).catch(function (err) {
            if (seq !== navSeq) return;
            if (root.console) root.console.error('[router]', err);
            showMessage('שגיאה', 'Something went wrong',
                'לא הצלחנו לפתוח את העמוד. רעננו ונסו שוב.', 'Could not open this page. Refresh and try again.');
        });
        return true;
    }

    // ---- events --------------------------------------------------------

    function onClick(ev) {
        if (ev.defaultPrevented || ev.button !== 0) return;
        if (ev.metaKey || ev.ctrlKey || ev.shiftKey || ev.altKey) return;
        var a = ev.target && ev.target.closest ? ev.target.closest('a[href]') : null;
        if (!a) return;
        if (a.target && a.target !== '_self') return;
        if (a.hasAttribute('download')) return;
        var href = a.getAttribute('href') || '';
        if (href.charAt(0) === '#') return;          // in-page anchors and admin tabs
        var url;
        try { url = new root.URL(a.href, root.location.href); } catch (err) { return; }
        if (url.origin !== root.location.origin) return;
        if (!find(url.pathname)) return;             // not an app route → normal page load
        // Same route with only a hash change (e.g. /admin/#lessons) → let the browser fire hashchange.
        if (current && current.route && normalize(url.pathname) === current.route.path && url.hash) return;
        ev.preventDefault();
        navigate(url.pathname + url.search + url.hash);
    }

    function onPopState(ev) {
        var path = root.location.pathname;
        // A hash-only change fires popstate too. The open view handles its own hash.
        if (current && current.route && normalize(path) === current.route.path) return;
        // Back/forward already changed the URL. If the view refuses to leave, put its URL back.
        if (current && current.view && typeof current.view.canLeave === 'function'
            && current.view.canLeave() === false) {
            try { root.history.pushState({ scrollY: root.scrollY || 0 }, '', current.url); } catch (err) { /* ignore */ }
            return;
        }
        var loc = root.location || {};
        var handled = navigate(String(loc.pathname || path) + String(loc.search || ''), {
            popstate: true,
            keepHash: true,
            scrollY: ev && ev.state && ev.state.scrollY
        });
        if (!handled) root.location.reload();
    }

    /**
     * @param {{ routes: object[], main: Element, user?: object, ready?: Promise }} opts
     */
    function start(opts) {
        if (started) return;
        started = true;
        var o = opts || {};
        routes = (o.routes || []).slice();
        mainEl = o.main;
        user = o.user || null;
        readyPromise = o.ready || Promise.resolve(null);
        try { root.history.scrollRestoration = 'manual'; } catch (err) { /* ignore */ }
        root.document.addEventListener('click', onClick);
        root.addEventListener('popstate', onPopState);
        root.addEventListener('hashchange', function () {
            if (current) current.url = root.location.pathname + root.location.hash;
        });
        var loc = root.location || {};
        var initial = String(loc.pathname || '/') + String(loc.search || '');
        var ok = navigate(initial, { replace: true, keepHash: true });
        if (!ok) showNotFound();
        prefetchOthers();
    }

    root.HebrewRouter = {
        start: start,
        navigate: navigate,
        showNotFound: showNotFound,
        normalize: normalize,
        find: find,
        loadScripts: loadScripts
    };
})(typeof window !== 'undefined' ? window : globalThis);
