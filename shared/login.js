/**
 * Sign-in page controller (canonical module).
 * The live /login/ page inlines this logic because /shared/* is gated when
 * signed out (SPEC-008). Keep behavior in sync with login/index.html.
 * Provenance: INIT-002/SPEC-012
 */
(function (root) {
    'use strict';

    var PRIVACY_NOTE =
        'מנהלים יכולים לראות את כל הפעילות שלך, כולל כל תשובה. (Admins can see all your activity, including every answer.)';

    var MSG_BAD = 'שם משתמש או סיסמה שגויים. (Wrong username or password.)';
    var MSG_THROTTLE = 'יותר מדי ניסיונות. נסו שוב מאוחר יותר. (Too many attempts. Try again later.)';
    var MSG_NETWORK = 'שגיאת רשת. בדקו את החיבור ונסו שוב. (Network error. Check your connection and try again.)';

    function qs(sel, rootEl) {
        return (rootEl || document).querySelector(sel);
    }

    function nextFromQuery() {
        var Api = root.HebrewApi;
        var safe = Api && Api.safeNextPath ? Api.safeNextPath : function () { return '/'; };
        try {
            var params = new URLSearchParams(root.location.search || '');
            return safe(params.get('next'));
        } catch (err) {
            return '/';
        }
    }

    function setError(el, text) {
        if (!el) return;
        if (!text) {
            el.hidden = true;
            el.textContent = '';
            return;
        }
        el.hidden = false;
        el.textContent = text;
    }

    function boot() {
        var form = qs('#login-form');
        if (!form) return;
        var userInput = qs('#login-username', form);
        var passInput = qs('#login-password', form);
        var errEl = qs('#login-error');
        var submitBtn = qs('#login-submit', form);
        var privacy = qs('#login-privacy');
        if (privacy) privacy.textContent = PRIVACY_NOTE;

        form.addEventListener('submit', function (event) {
            event.preventDefault();
            setError(errEl, '');
            if (submitBtn) submitBtn.disabled = true;

            var Api = root.HebrewApi;
            if (!Api || typeof Api.signIn !== 'function') {
                setError(errEl, MSG_NETWORK);
                if (submitBtn) submitBtn.disabled = false;
                return;
            }

            var username = userInput ? String(userInput.value || '') : '';
            var password = passInput ? String(passInput.value || '') : '';
            var next = nextFromQuery();

            Api.signIn(username, password, next).then(function (res) {
                if (res.ok && res.data) {
                    var dest = Api.safeNextPath(res.data.redirect != null ? res.data.redirect : next);
                    root.location.href = dest;
                    return;
                }
                if (res.status === 429) {
                    setError(errEl, MSG_THROTTLE);
                } else {
                    setError(errEl, MSG_BAD);
                }
                if (submitBtn) submitBtn.disabled = false;
            }).catch(function () {
                setError(errEl, MSG_NETWORK);
                if (submitBtn) submitBtn.disabled = false;
            });
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', boot);
    } else {
        boot();
    }
})(typeof window !== 'undefined' ? window : globalThis);
