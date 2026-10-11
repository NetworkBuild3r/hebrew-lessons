/**
 * Same-origin /api/ client with JSON + credentials.
 * A FormData body is sent as multipart without a JSON content type, so the
 * browser keeps the boundary. Upload uses this path (INIT-004/SPEC-016).
 * Provenance: INIT-002/SPEC-012
 */
(function (root) {
    'use strict';

    /**
     * Same-site absolute path only. Hostile values become `/`.
     * @param {unknown} value
     * @returns {string}
     */
    function safeNextPath(value) {
        if (typeof value !== 'string' || !value) return '/';
        var next = value.trim();
        if (!next) return '/';
        if (!next.startsWith('/')) return '/';
        if (next.startsWith('//')) return '/';
        if (next.indexOf('\\') !== -1) return '/';
        if (next.indexOf('\0') !== -1) return '/';
        if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(next)) return '/';
        return next;
    }

    function currentPathForNext() {
        try {
            var loc = root.location;
            var path = (loc && loc.pathname) || '/';
            return safeNextPath(path);
        } catch (err) {
            return '/';
        }
    }

    function redirectToLogin() {
        var next = currentPathForNext();
        var target = '/login/?next=' + encodeURIComponent(next);
        try {
            root.location.href = target;
        } catch (err) { /* fail-soft */ }
    }

    /**
     * Browser FormData, including a test double whose constructor is named FormData.
     * @param {unknown} body
     * @returns {boolean}
     */
    function isMultipart(body) {
        if (!body || typeof body.append !== 'function') return false;
        var name = body.constructor && body.constructor.name;
        return name === 'FormData';
    }

    /**
     * @param {string} path must start with /api/
     * @param {{ method?: string, body?: object|string, headers?: object, redirectOn401?: boolean, raw?: boolean }} [opts]
     *   `raw: true` keeps the response body string on `rawText` (INIT-006/SPEC-017).
     * @returns {Promise<{ ok: boolean, status: number, data: object|null, error: string|null, code: string|null, response: Response }>}
     */
    function request(path, opts) {
        var options = opts || {};
        if (typeof path !== 'string' || path.indexOf('/api/') !== 0) {
            return Promise.reject(new Error('HebrewApi only calls /api/ paths'));
        }
        var method = String(options.method || 'GET').toUpperCase();
        var headers = Object.assign({}, options.headers || {});
        var body = options.body;
        var init = {
            method: method,
            credentials: 'same-origin',
            headers: headers
        };
        if (body != null && method !== 'GET' && method !== 'HEAD') {
            if (isMultipart(body)) {
                init.body = body;
            } else {
                if (!headers['Content-Type'] && !headers['content-type']) {
                    headers['Content-Type'] = 'application/json';
                }
                init.body = typeof body === 'string' ? body : JSON.stringify(body);
            }
        }
        if (options.signal) init.signal = options.signal;

        return fetch(path, init).then(function (res) {
            if (res.status === 401 && options.redirectOn401 !== false) {
                redirectToLogin();
            }
            var ctype = res.headers.get('content-type') || '';
            var parse;
            if (options.raw) {
                parse = (typeof res.text === 'function' ? res.text() : Promise.resolve(''))
                    .then(function (text) {
                        var rawText = text == null ? '' : String(text);
                        var data = null;
                        if (ctype.indexOf('application/json') !== -1 && rawText) {
                            try { data = JSON.parse(rawText); } catch (err) { data = null; }
                        } else if (rawText) {
                            data = { raw: rawText };
                        }
                        return { data: data, rawText: rawText };
                    })
                    .catch(function () { return { data: null, rawText: '' }; });
            } else {
                parse = (ctype.indexOf('application/json') !== -1
                    ? res.json().catch(function () { return null; })
                    : res.text().then(function (t) {
                        return t ? { raw: t } : null;
                    }).catch(function () { return null; }))
                    .then(function (data) { return { data: data, rawText: null }; });
            }
            return parse.then(function (parsed) {
                var data = parsed.data;
                var errMsg = null;
                var code = null;
                if (data && typeof data === 'object') {
                    if (data.error) errMsg = String(data.error);
                    if (data.code) code = String(data.code);
                }
                var result = {
                    ok: res.ok,
                    status: res.status,
                    data: data,
                    error: errMsg,
                    code: code,
                    response: res
                };
                if (options.raw) result.rawText = parsed.rawText;
                return result;
            });
        });
    }

    function get(path, opts) {
        return request(path, Object.assign({}, opts || {}, { method: 'GET' }));
    }

    function post(path, body, opts) {
        return request(path, Object.assign({}, opts || {}, { method: 'POST', body: body }));
    }

    function del(path, body, opts) {
        return request(path, Object.assign({}, opts || {}, {
            method: 'DELETE',
            body: body != null ? body : {}
        }));
    }

    function me(opts) {
        return get('/api/me', opts);
    }

    function signIn(username, password, next) {
        var body = { username: username, password: password };
        if (next != null) body.next = next;
        return post('/api/session', body, { redirectOn401: false });
    }

    function signOut() {
        return del('/api/session', {}, { redirectOn401: false });
    }

    function changePassword(currentPassword, newPassword) {
        return post('/api/me/password', {
            current_password: currentPassword,
            new_password: newPassword
        });
    }

    root.HebrewApi = {
        safeNextPath: safeNextPath,
        currentPathForNext: currentPathForNext,
        request: request,
        get: get,
        post: post,
        del: del,
        me: me,
        signIn: signIn,
        signOut: signOut,
        changePassword: changePassword
    };
})(typeof window !== 'undefined' ? window : globalThis);
