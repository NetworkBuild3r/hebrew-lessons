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
    /* Hebrew line is a draft until the operator approves it (ADR-0009). Provenance: INIT-006/SPEC-017 */
    var AI_DISCLOSURE_EN = 'If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator\'s AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.';
    var AI_DISCLOSURE_HE = 'אם מנהל מפעיל תכנון בינה מלאכותית לחשבון שלך, סיכום של ההתקדמות ושם המשתמש נשלחים לספק הבינה המלאכותית של המפעיל כדי לנסח את השיעור הבא. הסיכום כולל את המילים שתורגלו ומידת הידיעה שלהן, את הדקדוק שתורגל ומידת הדיוק, את השיעורים שהושלמו, ובאיזו תדירות הקריאה היא בלי ניקוד. הסיכום אינו כולל תשובות בודדות, טקסט שהוקלד, או מועדי תרגול.';
    var AI_DISCLOSURE = AI_DISCLOSURE_HE + ' (' + AI_DISCLOSURE_EN + ')';

    var MSG_BAD = 'שם משתמש או סיסמה שגויים. (Wrong username or password.)';
    var MSG_BLOCKED = 'הכניסה נחסמה. רעננו את הדף ונסו שוב. (Sign-in was blocked. Refresh the page and try again.)';
    var MSG_THROTTLE = 'יותר מדי ניסיונות. נסו שוב מאוחר יותר. (Too many attempts. Try again later.)';
    var MSG_NETWORK = 'שגיאת רשת. בדקו את החיבור ונסו שוב. (Network error. Check your connection and try again.)';
    var GOOGLE_ERRORS = {
        google_unlinked: 'החשבון הזה ב-Google לא מקושר למשתמש. (This Google account is not linked to a user.)',
        google_denied: 'הכניסה עם Google בוטלה. (Google sign-in was cancelled.)',
        google_failed: 'הכניסה עם Google נכשלה. נסו שוב. (Google sign-in failed. Try again.)',
        google_expired: 'הכניסה פגה. נסו שוב. (Sign-in expired. Try again.)',
        google_throttled: MSG_THROTTLE,
        google_off: 'הכניסה עם Google אינה מופעלת. (Google sign-in is not turned on.)',
        google_unverified: 'Google לא אימת את כתובת הדוא״ל. (Google did not verify that email.)',
        google_disabled: 'החשבון מושבת. (This account is disabled.)'
    };

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
        try {
            var errCode = new URLSearchParams(root.location.search || '').get('error');
            if (errCode && GOOGLE_ERRORS[errCode]) setError(errEl, GOOGLE_ERRORS[errCode]);
        } catch (err) { /* ignore a bad query */ }
        var googleWrap = qs('#login-google-wrap');
        var googleLink = qs('#login-google');
        var fetcher = root.fetch;
        if (typeof fetcher === 'function') {
            fetcher('/api/auth/google', { credentials: 'same-origin' }).then(function (res) {
                return res && res.ok ? res.json() : null;
            }).then(function (data) {
                if (!data || !data.enabled || !googleWrap || !googleLink) return;
                googleLink.href = '/api/auth/google/start?next=' + encodeURIComponent(nextFromQuery());
                googleWrap.hidden = false;
            }).catch(function () { /* password sign-in still works */ });
        }
        var doc = root.document || document;
        if (doc && typeof doc.querySelectorAll === 'function') {
            var disclosures = doc.querySelectorAll('[data-ai-disclosure]');
            for (var i = 0; i < disclosures.length; i += 1) disclosures[i].textContent = AI_DISCLOSURE;
        }

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
                } else if (res.status === 403) {
                    setError(errEl, MSG_BLOCKED);
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
