/**
 * Server-backed progress client: pending signal queue + lesson progress PUT.
 * Provenance: INIT-002/SPEC-013
 */
(function (root) {
    'use strict';

    var PENDING_TEXT = 'עדיין לא נשמר — ננסה שוב. (Not saved yet — will retry.)';
    var QUEUE_PREFIX = 'hebrew-sync:v1:';
    var BATCH_SIZE = 200;
    var QUEUE_CAP = 1000;
    var LESSON_DEBOUNCE_MS = 400;
    var BACKOFF_BASE_MS = 1000;
    var BACKOFF_MAX_MS = 30000;

    /** @type {object|null} */
    var user = null;
    /** @type {{ v:number, words:object, migratedLessons:object }|null} */
    var reviewState = null;
    /** @type {string|null} */
    var lessonSlug = null;
    /** @type {object} */
    var lessonState = {};
    /** @type {number} */
    var lessonRev = 0;
    /** @type {'loading'|'synced'|'pending'|'error'} */
    var status = 'loading';
    /** @type {boolean} */
    var flushing = false;
    /** @type {number} */
    var backoffMs = BACKOFF_BASE_MS;
    /** @type {number|null} */
    var flushTimerId = null;
    /** @type {number|null} */
    var lessonTimerId = null;
    /** @type {Promise<object|null>|null} */
    var putInFlight = null;
    /** @type {boolean} */
    var putAgain = false;
    /** @type {function(object): void|null} */
    var onLessonConflict = null;
    /** @type {boolean} */
    var listenersBound = false;
    /** @type {boolean} */
    var initStarted = false;

    // Injectable for unit tests (no real timers there).
    var nowFn = function () { return Date.now(); };
    var scheduleFn = function (fn, ms) {
        return root.setTimeout(fn, ms);
    };
    var clearScheduleFn = function (id) {
        if (id != null) root.clearTimeout(id);
    };
    var fetchFn = typeof root.fetch === 'function'
        ? function (url, init) { return root.fetch(url, init); }
        : null;
    var uuidFn = function () {
        if (root.crypto && typeof root.crypto.randomUUID === 'function') {
            return root.crypto.randomUUID();
        }
        return 'sig-' + nowFn().toString(36) + '-' + Math.random().toString(36).slice(2, 10);
    };

    function api() {
        return root.HebrewApi;
    }

    function storage() {
        try {
            return root.localStorage || null;
        } catch (err) {
            return null;
        }
    }

    function queueKey(userId) {
        return QUEUE_PREFIX + String(userId);
    }

    function readQueue(userId) {
        var store = storage();
        if (!store || userId == null) return [];
        try {
            var raw = store.getItem(queueKey(userId));
            if (!raw) return [];
            var parsed = JSON.parse(raw);
            return Array.isArray(parsed) ? parsed : [];
        } catch (err) {
            return [];
        }
    }

    function writeQueue(userId, queue) {
        var store = storage();
        if (!store || userId == null) return;
        try {
            store.setItem(queueKey(userId), JSON.stringify(queue));
        } catch (err) { /* fail-soft */ }
    }

    function isActive() {
        return !!(user && user.id != null);
    }

    function getReviewState() {
        return reviewState || { v: 1, words: {}, migratedLessons: {} };
    }

    function getLessonProgress() {
        return { state: lessonState || {}, rev: lessonRev || 0 };
    }

    function getStatus() {
        return status;
    }

    function getUser() {
        return user;
    }

    function setStatus(next) {
        status = next;
        paintStatus();
    }

    function paintStatus() {
        var el = null;
        try {
            el = root.document && root.document.querySelector
                ? root.document.querySelector('[data-shell-sync]')
                : null;
        } catch (err) {
            el = null;
        }
        if (!el) return;
        el.setAttribute('data-sync-status', status);
        if (status === 'pending') {
            el.textContent = PENDING_TEXT;
        } else if (status === 'error') {
            el.textContent = 'שגיאת שמירה — ננסה שוב. (Save error — will retry.)';
        } else if (status === 'loading') {
            el.textContent = '';
        } else {
            el.textContent = '';
        }
    }

    function adoptReviewState(state) {
        reviewState = state && typeof state === 'object'
            ? {
                v: state.v != null ? state.v : 1,
                words: state.words && typeof state.words === 'object' ? state.words : {},
                migratedLessons: state.migratedLessons && typeof state.migratedLessons === 'object'
                    ? state.migratedLessons
                    : {}
            }
            : { v: 1, words: {}, migratedLessons: {} };
        var Review = root.HebrewReview;
        if (Review && typeof Review._resetMemory === 'function') {
            Review._resetMemory(reviewState);
        }
    }

    function adoptLessonProgress(state, rev) {
        lessonState = state && typeof state === 'object' ? state : {};
        lessonRev = Number.isFinite(Number(rev)) ? Number(rev) : 0;
    }

    function capQueue(queue) {
        var next = queue.slice();
        while (next.length > QUEUE_CAP) {
            next.shift();
            try {
                console.warn('hebrew-sync: dropped oldest pending signal (queue cap ' + QUEUE_CAP + ')');
            } catch (err) { /* ignore */ }
        }
        return next;
    }

    function queueLength() {
        if (!isActive()) return 0;
        return readQueue(user.id).length;
    }

    function refreshStatusFromQueue() {
        if (!isActive()) {
            setStatus('synced');
            return;
        }
        if (queueLength() > 0) setStatus('pending');
        else if (status === 'error') setStatus('error');
        else setStatus('synced');
    }

    /**
     * @param {object} signal
     * @returns {object}
     */
    function queueSignal(signal) {
        if (!isActive()) return signal || null;
        var item = signal && typeof signal === 'object' ? Object.assign({}, signal) : {};
        if (!item.id) item.id = uuidFn();
        if (!item.at) item.at = new Date(nowFn()).toISOString();
        var q = readQueue(user.id);
        q.push(item);
        q = capQueue(q);
        writeQueue(user.id, q);
        setStatus('pending');
        scheduleFlush(0);
        return item;
    }

    function clearFlushTimer() {
        if (flushTimerId != null) {
            clearScheduleFn(flushTimerId);
            flushTimerId = null;
        }
    }

    function scheduleFlush(delayMs) {
        clearFlushTimer();
        var wait = delayMs == null ? 0 : Math.max(0, Number(delayMs) || 0);
        flushTimerId = scheduleFn(function () {
            flushTimerId = null;
            flush({ keepalive: false });
        }, wait);
    }

    /**
     * @param {{ keepalive?: boolean }} [opts]
     * @returns {Promise<boolean>}
     */
    function flush(opts) {
        var options = opts || {};
        if (!isActive()) return Promise.resolve(true);
        if (flushing) return Promise.resolve(false);
        var q = readQueue(user.id);
        if (!q.length) {
            setStatus('synced');
            backoffMs = BACKOFF_BASE_MS;
            return Promise.resolve(true);
        }

        flushing = true;
        var batch = q.slice(0, BATCH_SIZE);
        var body = { signals: batch };

        var send = options.keepalive && fetchFn
            ? fetchFn('/api/review/signals', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
                keepalive: true
            }).then(function (res) {
                return res.json().then(function (data) {
                    return { ok: res.ok, status: res.status, data: data };
                }).catch(function () {
                    return { ok: res.ok, status: res.status, data: null };
                });
            })
            : (function () {
                var Api = api();
                if (!Api || typeof Api.post !== 'function') {
                    return Promise.reject(new Error('HebrewApi missing'));
                }
                return Api.post('/api/review/signals', body, { redirectOn401: false });
            })();

        return send.then(function (res) {
            flushing = false;
            if (!res || !res.ok) {
                setStatus(queueLength() > 0 ? 'pending' : 'error');
                scheduleFlush(backoffMs);
                backoffMs = Math.min(BACKOFF_MAX_MS, backoffMs * 2);
                return false;
            }
            var data = res.data || {};
            if (data.state) adoptReviewState(data.state);

            // Drop flushed ids (idempotent re-flush of leftovers is fine).
            var flushed = Object.create(null);
            batch.forEach(function (s) {
                if (s && s.id != null) flushed[String(s.id)] = true;
            });
            var remaining = readQueue(user.id).filter(function (s) {
                return !(s && s.id != null && flushed[String(s.id)]);
            });
            writeQueue(user.id, remaining);
            backoffMs = BACKOFF_BASE_MS;

            if (remaining.length) {
                setStatus('pending');
                scheduleFlush(0);
            } else {
                setStatus('synced');
            }
            return true;
        }).catch(function () {
            flushing = false;
            setStatus('pending');
            scheduleFlush(backoffMs);
            backoffMs = Math.min(BACKOFF_MAX_MS, backoffMs * 2);
            return false;
        });
    }

    function clearLessonTimer() {
        if (lessonTimerId != null) {
            clearScheduleFn(lessonTimerId);
            lessonTimerId = null;
        }
    }

    /**
     * Debounced PUT /api/lessons/:slug/progress. On 409, adopt server copy.
     * @param {object} state
     * @returns {void}
     */
    function saveLessonProgress(state) {
        if (!isActive() || !lessonSlug) return;
        lessonState = state && typeof state === 'object' ? state : {};
        clearLessonTimer();
        lessonTimerId = scheduleFn(function () {
            lessonTimerId = null;
            putLessonNow();
        }, LESSON_DEBOUNCE_MS);
    }

    /**
     * One lesson save at a time. A second PUT sent before the first answers would carry the
     * old revision, and the server would reject it as a conflict (409). On a slower link that
     * threw away the learner's newest answers and redrew the page. While one is in flight,
     * later saves are folded into a single follow-up that uses the revision the server returned.
     */
    function putLessonNow() {
        if (!isActive() || !lessonSlug) return Promise.resolve(null);
        var Api = api();
        if (!Api || typeof Api.request !== 'function') return Promise.resolve(null);
        if (putInFlight) {
            putAgain = true;
            return putInFlight;
        }
        var sentState = lessonState;
        var payload = { state: sentState, rev: lessonRev };
        putInFlight = Api.request('/api/lessons/' + encodeURIComponent(lessonSlug) + '/progress', {
            method: 'PUT',
            body: payload,
            redirectOn401: false
        }).then(function (res) {
            if (res && res.ok && res.data) {
                var newer = putAgain || lessonTimerId != null || lessonState !== sentState;
                if (newer) {
                    // Keep the newer local answers; only take the new revision.
                    var nextRev = Number(res.data.rev);
                    if (Number.isFinite(nextRev)) lessonRev = nextRev;
                } else {
                    adoptLessonProgress(res.data.state, res.data.rev);
                }
                return res.data;
            }
            if (res && res.status === 409 && res.data) {
                // A real conflict (another tab or device saved first): the server copy wins.
                putAgain = false;
                adoptLessonProgress(res.data.state, res.data.rev);
                if (typeof onLessonConflict === 'function') {
                    try { onLessonConflict(getLessonProgress()); } catch (err) { /* fail-soft */ }
                }
                return res.data;
            }
            return null;
        }).catch(function () {
            return null;
        }).then(function (out) {
            putInFlight = null;
            if (putAgain) {
                putAgain = false;
                putLessonNow();
            }
            return out;
        });
        return putInFlight;
    }

    function bindLifecycle() {
        if (listenersBound || typeof root.document === 'undefined') return;
        listenersBound = true;
        root.document.addEventListener('visibilitychange', function () {
            if (root.document.visibilityState === 'hidden') {
                flush({ keepalive: true });
            }
        });
        root.addEventListener('pagehide', function () {
            flush({ keepalive: true });
        });
    }

    /**
     * @param {{ lessonSlug?: string|null, onLessonConflict?: function }} [opts]
     * @returns {Promise<object|null>}
     */
    function init(opts) {
        var options = opts || {};
        if (typeof options.onLessonConflict === 'function') {
            onLessonConflict = options.onLessonConflict;
        }
        if (options.lessonSlug != null) {
            lessonSlug = String(options.lessonSlug);
        } else if (root.WORKSHEET && root.WORKSHEET.id) {
            lessonSlug = String(root.WORKSHEET.id);
        }

        initStarted = true;
        setStatus('loading');
        bindLifecycle();

        var Api = api();
        if (!Api || typeof Api.me !== 'function') {
            user = null;
            setStatus('error');
            return Promise.resolve(null);
        }

        return Api.me({ redirectOn401: true }).then(function (meRes) {
            if (!meRes || !meRes.ok || !meRes.data || meRes.data.id == null) {
                user = null;
                setStatus('error');
                return null;
            }
            user = meRes.data;
            return Api.get('/api/review/state', { redirectOn401: false }).then(function (stateRes) {
                if (stateRes && stateRes.ok && stateRes.data) {
                    adoptReviewState(stateRes.data);
                } else {
                    adoptReviewState({ v: 1, words: {}, migratedLessons: {} });
                }

                var afterLesson = Promise.resolve(null);
                if (lessonSlug) {
                    afterLesson = Api.get(
                        '/api/lessons/' + encodeURIComponent(lessonSlug) + '/progress',
                        { redirectOn401: false }
                    ).then(function (progRes) {
                        if (progRes && progRes.ok && progRes.data) {
                            adoptLessonProgress(progRes.data.state, progRes.data.rev);
                        } else {
                            adoptLessonProgress({}, 0);
                        }
                        return getLessonProgress();
                    }).catch(function () {
                        adoptLessonProgress({}, 0);
                        return getLessonProgress();
                    });
                }

                return afterLesson.then(function () {
                    if (queueLength() > 0) {
                        setStatus('pending');
                        scheduleFlush(0);
                    } else {
                        setStatus('synced');
                    }
                    paintStatus();
                    return user;
                });
            });
        }).catch(function () {
            user = null;
            setStatus('error');
            return null;
        });
    }

    function setOnLessonConflict(fn) {
        onLessonConflict = typeof fn === 'function' ? fn : null;
    }

    function setLessonSlug(slug) {
        lessonSlug = slug != null ? String(slug) : null;
    }

    /** Test / reset helpers — not for production UI. */
    function _resetForTests() {
        clearFlushTimer();
        clearLessonTimer();
        putInFlight = null;
        putAgain = false;
        user = null;
        reviewState = null;
        lessonSlug = null;
        lessonState = {};
        lessonRev = 0;
        status = 'loading';
        flushing = false;
        backoffMs = BACKOFF_BASE_MS;
        onLessonConflict = null;
        initStarted = false;
        nowFn = function () { return Date.now(); };
        scheduleFn = function (fn, ms) { return root.setTimeout(fn, ms); };
        clearScheduleFn = function (id) { if (id != null) root.clearTimeout(id); };
        fetchFn = typeof root.fetch === 'function'
            ? function (url, init) { return root.fetch(url, init); }
            : null;
        uuidFn = function () {
            if (root.crypto && typeof root.crypto.randomUUID === 'function') {
                return root.crypto.randomUUID();
            }
            return 'sig-' + nowFn().toString(36) + '-' + Math.random().toString(36).slice(2, 10);
        };
    }

    var Sync = {
        PENDING_TEXT: PENDING_TEXT,
        QUEUE_PREFIX: QUEUE_PREFIX,
        BATCH_SIZE: BATCH_SIZE,
        QUEUE_CAP: QUEUE_CAP,
        init: init,
        queueSignal: queueSignal,
        flush: flush,
        isActive: isActive,
        getUser: getUser,
        getStatus: getStatus,
        getReviewState: getReviewState,
        getLessonProgress: getLessonProgress,
        saveLessonProgress: saveLessonProgress,
        adoptLessonProgress: adoptLessonProgress,
        adoptReviewState: adoptReviewState,
        setOnLessonConflict: setOnLessonConflict,
        setLessonSlug: setLessonSlug,
        paintStatus: paintStatus,
        queueKey: queueKey,
        _resetForTests: _resetForTests,
        _test: {
            setNow: function (fn) { nowFn = fn; },
            setSchedule: function (sched, clear) {
                scheduleFn = sched;
                if (typeof clear === 'function') clearScheduleFn = clear;
            },
            setFetch: function (fn) { fetchFn = fn; },
            setUuid: function (fn) { uuidFn = fn; },
            readQueue: readQueue,
            writeQueue: writeQueue,
            putLessonNow: putLessonNow,
            getBackoffMs: function () { return backoffMs; },
            setBackoffMs: function (ms) { backoffMs = ms; },
            setUser: function (u) { user = u; },
            setStatus: setStatus,
            LESSON_DEBOUNCE_MS: LESSON_DEBOUNCE_MS
        }
    };

    root.HebrewSync = Sync;
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = Sync;
    }
})(typeof window !== 'undefined' ? window : globalThis);
