/**
 * Plan editor page at /editor/: plan list and one open plan.
 * Admin at /admin/ stays users and worksheet lessons.
 * Provenance: INIT-010/SPEC-003
 */
(function (root) {
    'use strict';

    var PLAN_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

    /**
     * @param {string} hash
     * @returns {{ view: 'plans'|'plan', planId: string|null }}
     */
    function parseHash(hash) {
        var raw = String(hash || '').replace(/^#/, '').trim();
        if (!raw || raw === 'plans') {
            return { view: 'plans', planId: null };
        }
        var planMatch = /^plan\/([^/]+)$/.exec(raw);
        if (planMatch) {
            var planId = '';
            try {
                planId = decodeURIComponent(planMatch[1]);
            } catch (err) {
                planId = '';
            }
            if (PLAN_SLUG.test(planId)) {
                return { view: 'plan', planId: planId };
            }
        }
        return { view: 'plans', planId: null };
    }

    function setHash(route) {
        try {
            if (root.location) {
                root.location.hash = route;
            }
        } catch (err) { /* ignore */ }
    }

    var activeBuilder = null;

    /**
     * @param {HTMLElement} target
     * @param {{ PlanBuilder?: object, signal?: AbortSignal }} [opts]
     */
    function mount(target, opts) {
        if (!target) return;
        var options = opts || {};
        var PlanBuilder = options.PlanBuilder || root.HebrewPlanBuilder;
        var state = { route: parseHash(root.location && root.location.hash) };

        function paint() {
            var route = state.route;
            target.innerHTML = '<div class="editor-shell" dir="rtl" lang="he">'
                + '<div class="editor-panel" id="editor-panel" data-editor-panel></div>'
                + '</div>';
            var panel = target.querySelector('[data-editor-panel]');
            if (!panel) return;

            activeBuilder = PlanBuilder || null;
            if (route.view === 'plan' && route.planId && PlanBuilder && typeof PlanBuilder.mount === 'function') {
                PlanBuilder.mount(panel, route.planId);
                return;
            }
            if (PlanBuilder && typeof PlanBuilder.mountPicker === 'function') {
                PlanBuilder.mountPicker(panel, { setHash: setHash });
            } else {
                panel.innerHTML = '<div class="card" data-editor-view="plans"><p>'
                    + 'בוחר התוכנית לא נטען. (The plan picker is not loaded.)'
                    + '</p></div>';
            }
        }

        function onHash() {
            state.route = parseHash(root.location && root.location.hash);
            paint();
        }

        if (root.addEventListener) {
            root.addEventListener('hashchange', onHash, options.signal ? { signal: options.signal } : false);
        }

        if (!root.location || !root.location.hash) {
            try {
                root.history.replaceState(root.history.state, '', '#plans');
            } catch (err) {
                setHash('plans');
            }
        }
        state.route = parseHash(root.location && root.location.hash);
        paint();
    }

    function canLeave() {
        if (activeBuilder && typeof activeBuilder.canLeave === 'function') {
            return activeBuilder.canLeave();
        }
        return true;
    }

    function unmount() {
        if (activeBuilder && typeof activeBuilder.unmount === 'function') {
            activeBuilder.unmount();
        }
        activeBuilder = null;
    }

    root.HebrewEditorShell = {
        parseHash: parseHash,
        mount: mount,
        unmount: unmount,
        canLeave: canLeave,
        setHash: setHash
    };
})(typeof window !== 'undefined' ? window : globalThis);
